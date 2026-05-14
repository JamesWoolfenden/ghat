package core

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type OrgFlags struct {
	Provider    string // "github" (default) or "gitlab"
	BaseURL     string // self-hosted API root, e.g. https://gitlab.example.com
	Owner       string
	Repos       []string // explicit list; if set, Owner/Limit are ignored
	Token       string   // PAT for Provider (clone/push/PR)
	GitHubToken string   // separate PAT for api.github.com lookups during the sweep
	Branch      string
	Offset      int
	Limit       int
	DryRun      bool
	OpenPR      bool
	AutoMerge   bool
	Threshold   int // pause when fewer than this many API requests remain
}

type RepoResult struct {
	Repo   string
	Status string // "pinned", "already-pinned", "pr-open", "error"
	PRUrl  string
	Error  error
	Gaps   []string
}

// gapPattern describes a version-pinning pattern ghat does not yet handle.
type gapPattern struct {
	label string
	re    *regexp.Regexp
	globs []string
}

var gapPatterns = []gapPattern{
	{"go install @version", regexp.MustCompile(`go install .+@v[0-9]`), []string{"*.sh", "Makefile", "*.mk", "Dockerfile*", "*.dockerfile"}},
	{"pip install pinned", regexp.MustCompile(`pip install [^-].+==[0-9]`), []string{"*.sh", "Makefile", "*.mk", "Dockerfile*", "*.dockerfile", "requirements*.txt"}},
	{"npm/yarn add pinned", regexp.MustCompile(`(npm install|yarn add) .+@[0-9]`), []string{"*.sh", "Makefile", "*.mk"}},
	{"apk add pinned", regexp.MustCompile(`apk add .+=[0-9]`), []string{"Dockerfile*", "*.dockerfile", "*.sh"}},
	{"apt-get install pinned", regexp.MustCompile(`apt-get install .+=[0-9]`), []string{"Dockerfile*", "*.dockerfile", "*.sh"}},
	{"curl release download", regexp.MustCompile(`curl .+releases/download`), []string{"*.sh", "Makefile", "*.mk", "Dockerfile*", "*.dockerfile"}},
	{"wget release download", regexp.MustCompile(`wget .+releases/download`), []string{"*.sh", "Makefile", "*.mk", "Dockerfile*", "*.dockerfile"}},
	{"gem install versioned", regexp.MustCompile(`gem install .+ -v [0-9]`), []string{"*.sh", "Makefile", "*.mk", "Dockerfile*", "*.dockerfile"}},
}

func (o *OrgFlags) RunBulk() ([]RepoResult, error) {
	host, err := newHost(o.Provider, o.Owner, o.Token, o.BaseURL)
	if err != nil {
		return nil, err
	}

	var repos []hostRepo
	if len(o.Repos) > 0 {
		for _, name := range o.Repos {
			r, err := host.RepoFromName(name)
			if err != nil {
				return nil, fmt.Errorf("resolving --repo %s: %w", name, err)
			}
			repos = append(repos, r)
		}
	} else {
		repos, err = host.ListRepos()
		if err != nil {
			return nil, fmt.Errorf("listing repos: %w", err)
		}
		if o.Offset > 0 {
			if o.Offset >= len(repos) {
				return nil, nil
			}
			repos = repos[o.Offset:]
		}
		if o.Limit > 0 && len(repos) > o.Limit {
			repos = repos[:o.Limit]
		}
	}

	log.Info().Int("total", len(repos)).Msg("processing repos")

	var results []RepoResult
	for i, repo := range repos {
		log.Info().Msgf("[%d/%d] %s", i+1, len(repos), repo.Name)
		result := o.processRepo(host, repo)
		results = append(results, result)
		if result.Error != nil {
			log.Warn().Err(result.Error).Str("repo", repo.Name).Msg("skipping")
		}
		host.WaitForRateLimit(o.Threshold)
	}
	return results, nil
}

func (o *OrgFlags) processRepo(host hostProvider, repo hostRepo) RepoResult {
	result := RepoResult{Repo: repo.Name}

	// If a PR/MR is already open, still re-run and force-push so the branch
	// stays current — the existing PR picks up the new commits automatically.
	// Only skip PR creation at the end. A failed check is non-fatal: CreatePR
	// will surface the real error and its fallback re-checks PRExists.
	prAlreadyOpen, existingPRUrl, existingMergeID, err := host.PRExists(repo, o.Branch)
	if err != nil {
		log.Warn().Err(err).Str("repo", repo.Name).Msg("PR existence check failed; continuing")
	}

	dir, err := os.MkdirTemp("", "ghat-*")
	if err != nil {
		result.Status = "error"
		result.Error = fmt.Errorf("mktemp: %w", err)
		return result
	}
	defer func() { _ = os.RemoveAll(dir) }()

	if out, err := exec.Command("git", "clone", "--depth=1", "--quiet", repo.CloneURL, dir).CombinedOutput(); err != nil {
		result.Status = "error"
		result.Error = fmt.Errorf("clone: %w: %s", err, strings.TrimSpace(string(out)))
		return result
	}

	// Raise log level for the sweep so per-file info chatter is suppressed;
	// only warnings (SUPPLY CHAIN RISK, updated, errors) will show.
	prev := zerolog.GlobalLevel()
	zerolog.SetGlobalLevel(zerolog.WarnLevel)
	defer zerolog.SetGlobalLevel(prev)

	// Always write to the temp clone so git status accurately reflects what
	// changed. o.DryRun only controls whether we push and open a PR.
	ghToken := o.GitHubToken
	if ghToken == "" && !strings.EqualFold(o.Provider, "gitlab") {
		ghToken = o.Token
	}
	myFlags := &Flags{
		Directory:       dir,
		GitHubToken:     ghToken,
		DryRun:          false,
		ContinueOnError: true,
		Silent:          true,
	}
	var days uint
	myFlags.Days = &days
	if err := myFlags.InitializeCache(); err != nil {
		result.Status = "error"
		result.Error = fmt.Errorf("cache init: %w", err)
		return result
	}
	if err := myFlags.Action(ActionSweep); err != nil {
		result.Status = "error"
		result.Error = fmt.Errorf("sweep: %w", err)
		return result
	}

	result.Gaps = scanGaps(dir)

	// check for changes
	out, _ := exec.Command("git", "-C", dir, "status", "--porcelain").Output()
	if len(strings.TrimSpace(string(out))) == 0 {
		result.Status = "already-pinned"
		return result
	}

	// dry-run: report what would change but don't push
	if o.DryRun || !o.OpenPR {
		result.Status = "pinned"
		return result
	}

	defaultBranch, _ := exec.Command("git", "-C", dir, "symbolic-ref", "--short", "HEAD").Output()
	base := strings.TrimSpace(string(defaultBranch))

	if out, err := exec.Command("git", "-C", dir, "checkout", "-b", o.Branch).CombinedOutput(); err != nil {
		result.Status = "error"
		result.Error = fmt.Errorf("checkout: %w: %s", err, strings.TrimSpace(string(out)))
		return result
	}

	exec.Command("git", "-C", dir, "add", "-A").Run()                                                          //nolint:errcheck
	exec.Command("git", "-C", dir, "commit", "-m", "chore: pin dependencies to immutable SHAs via ghat").Run() //nolint:errcheck

	// --force is intentional: this is our automation branch created from a fresh
	// shallow clone. --force-with-lease fails on shallow clones because git has no
	// remote tracking ref for branches it never fetched.
	if out, err := exec.Command("git", "-C", dir, "push", "--force", "origin", o.Branch).CombinedOutput(); err != nil {
		result.Status = "error"
		result.Error = fmt.Errorf("push: %w: %s", err, strings.TrimSpace(string(out)))
		return result
	}

	if prAlreadyOpen {
		result.Status = "pr-open"
		result.PRUrl = existingPRUrl
		log.Warn().Str("repo", repo.Name).Str("pr", existingPRUrl).Msg("branch updated, existing PR refreshed")
		if o.AutoMerge && existingMergeID != "" {
			if err := host.EnableAutoMerge(existingMergeID); err != nil {
				log.Warn().Err(err).Msg("auto-merge not enabled")
			}
		}
		return result
	}

	prURL, mergeID, err := host.CreatePR(repo, o.Branch, base)
	if err != nil {
		if open, prURL, mergeID, _ := host.PRExists(repo, o.Branch); open {
			result.Status = "pr-open"
			result.PRUrl = prURL
			log.Warn().Str("repo", repo.Name).Str("pr", prURL).Msg("PR already existed, branch updated")
			if o.AutoMerge && mergeID != "" {
				if err := host.EnableAutoMerge(mergeID); err != nil {
					log.Warn().Err(err).Msg("auto-merge not enabled")
				}
			}
			return result
		}
		result.Status = "error"
		result.Error = fmt.Errorf("create PR: %w", err)
		return result
	}

	result.Status = "pinned"
	result.PRUrl = prURL
	log.Warn().Str("repo", repo.Name).Str("pr", prURL).Msg("PR opened")
	if o.AutoMerge && mergeID != "" {
		if err := host.EnableAutoMerge(mergeID); err != nil {
			log.Warn().Err(err).Msg("auto-merge not enabled")
		}
	}
	return result
}

// CreateLocalPR checks for git changes in dir, then commits them to a branch and
// opens a PR. Returns (prURL, changed, error). If no changes, changed is false.
// If OpenPR is false, changed is still reported but no branch/PR is created.
func (f *Flags) CreateLocalPR(dir string) (string, bool, error) {
	out, _ := exec.Command("git", "-C", dir, "status", "--porcelain").Output()
	if len(strings.TrimSpace(string(out))) == 0 {
		return "", false, nil
	}
	if !f.OpenPR {
		return "", true, nil
	}

	defaultBranch, _ := exec.Command("git", "-C", dir, "symbolic-ref", "--short", "HEAD").Output()
	base := strings.TrimSpace(string(defaultBranch))

	remoteOut, err := exec.Command("git", "-C", dir, "remote", "get-url", "origin").Output()
	if err != nil {
		return "", true, fmt.Errorf("get remote URL: %w", err)
	}
	provider, repoName, baseURL, err := parseRemoteURL(strings.TrimSpace(string(remoteOut)))
	if err != nil {
		return "", true, fmt.Errorf("parse remote URL: %w", err)
	}

	branch := f.Branch
	if branch == "" {
		branch = "ghat/pin-dependencies"
	}

	host, err := newHost(provider, "", f.PRToken, baseURL)
	if err != nil {
		return "", true, fmt.Errorf("create host: %w", err)
	}
	repo, err := host.RepoFromName(repoName)
	if err != nil {
		return "", true, fmt.Errorf("resolve repo: %w", err)
	}

	prAlreadyOpen, existingPRUrl, existingMergeID, _ := host.PRExists(repo, branch)

	// -B creates the branch if it doesn't exist, or resets it to HEAD if it does.
	if out, err := exec.Command("git", "-C", dir, "checkout", "-B", branch).CombinedOutput(); err != nil {
		return "", true, fmt.Errorf("checkout: %w: %s", err, strings.TrimSpace(string(out)))
	}
	exec.Command("git", "-C", dir, "add", "-A").Run()                                                          //nolint:errcheck
	exec.Command("git", "-C", dir, "commit", "-m", "chore: pin dependencies to immutable SHAs via ghat").Run() //nolint:errcheck

	if out, err := exec.Command("git", "-C", dir, "push", "--force", "origin", branch).CombinedOutput(); err != nil {
		return "", true, fmt.Errorf("push: %w: %s", err, strings.TrimSpace(string(out)))
	}

	if prAlreadyOpen {
		if f.AutoMerge && existingMergeID != "" {
			_ = host.EnableAutoMerge(existingMergeID)
		}
		return existingPRUrl, true, nil
	}

	prURL, mergeID, err := host.CreatePR(repo, branch, base)
	if err != nil {
		if open, u, mid, _ := host.PRExists(repo, branch); open {
			if f.AutoMerge && mid != "" {
				_ = host.EnableAutoMerge(mid)
			}
			return u, true, nil
		}
		return "", true, fmt.Errorf("create PR: %w", err)
	}

	if f.AutoMerge && mergeID != "" {
		_ = host.EnableAutoMerge(mergeID)
	}
	return prURL, true, nil
}

// parseRemoteURL extracts the provider, owner/repo name, and base URL from a
// git remote (HTTPS or SSH format).
func parseRemoteURL(remoteURL string) (provider, repoName, baseURL string, err error) {
	if strings.HasPrefix(remoteURL, "git@") {
		rest := strings.TrimPrefix(remoteURL, "git@")
		host, path, ok := strings.Cut(rest, ":")
		if !ok {
			return "", "", "", fmt.Errorf("cannot parse SSH remote: %s", remoteURL)
		}
		repo := strings.TrimSuffix(path, ".git")
		if strings.Contains(host, "github") {
			return "github", repo, "", nil
		}
		return "gitlab", repo, "https://" + host, nil
	}

	u, err := url.Parse(remoteURL)
	if err != nil {
		return "", "", "", fmt.Errorf("cannot parse remote URL: %w", err)
	}
	repo := strings.TrimSuffix(strings.TrimPrefix(u.Path, "/"), ".git")
	if strings.Contains(u.Hostname(), "github") {
		return "github", repo, "", nil
	}
	return "gitlab", repo, u.Scheme + "://" + u.Host, nil
}

func scanGaps(dir string) []string {
	var found []string
	entries, err := GetFiles(dir)
	if err != nil {
		return nil
	}
	for _, file := range entries {
		base := filepath.Base(file)
		rel, err := filepath.Rel(dir, file)
		if err != nil {
			rel = file
		}
		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		for _, p := range gapPatterns {
			if !matchesGlob(base, p.globs) {
				continue
			}
			for i, line := range strings.Split(string(content), "\n") {
				if p.re.MatchString(line) {
					found = append(found, fmt.Sprintf("%s | %s:%d", p.label, rel, i+1))
				}
			}
		}
	}
	return found
}

func matchesGlob(name string, globs []string) bool {
	for _, g := range globs {
		if ok, _ := filepath.Match(g, name); ok {
			return true
		}
		if strings.HasPrefix(g, "Dockerfile") && strings.HasPrefix(name, "Dockerfile") {
			return true
		}
	}
	return false
}
