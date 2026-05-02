package core

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type OrgFlags struct {
	Owner     string
	Repos     []string // explicit list; if set, Owner/Limit are ignored
	Token     string
	Branch    string
	Limit     int
	DryRun    bool
	OpenPR    bool
	Threshold int // pause when fewer than this many API requests remain
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
	var repos []string
	if len(o.Repos) > 0 {
		repos = o.Repos
	} else {
		var err error
		repos, err = o.listRepos()
		if err != nil {
			return nil, fmt.Errorf("listing repos: %w", err)
		}
		if o.Limit > 0 && len(repos) > o.Limit {
			repos = repos[:o.Limit]
		}
	}

	log.Info().Int("total", len(repos)).Msg("processing repos")

	var results []RepoResult
	for i, repo := range repos {
		log.Info().Msgf("[%d/%d] %s", i+1, len(repos), repo)
		result := o.processRepo(repo)
		results = append(results, result)
		if result.Error != nil {
			log.Warn().Err(result.Error).Str("repo", repo).Msg("skipping")
		}
		o.waitForRateLimit()
	}
	return results, nil
}

func (o *OrgFlags) listRepos() ([]string, error) {
	url := "https://api.github.com/user/repos?type=owner&per_page=100"
	var all []string
	for url != "" {
		body, next, err := getPagedGithubBody(o.Token, url)
		if err != nil {
			return nil, err
		}
		items, ok := body.([]interface{})
		if !ok {
			return nil, fmt.Errorf("unexpected repos response type")
		}
		for _, item := range items {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			if fork, _ := m["fork"].(bool); fork {
				continue
			}
			if name, _ := m["full_name"].(string); name != "" {
				all = append(all, name)
			}
		}
		url = next
	}
	return all, nil
}

func (o *OrgFlags) processRepo(repo string) RepoResult {
	result := RepoResult{Repo: repo}

	// skip if PR already open
	if open, _ := o.prExists(repo); open {
		result.Status = "pr-open"
		log.Info().Str("repo", repo).Msg("PR already open, skipping")
		return result
	}

	dir, err := os.MkdirTemp("", "ghat-*")
	if err != nil {
		result.Status = "error"
		result.Error = fmt.Errorf("mktemp: %w", err)
		return result
	}
	defer func() { _ = os.RemoveAll(dir) }()

	cloneURL := "https://github.com/" + repo + ".git"
	if out, err := exec.Command("git", "clone", "--depth=1", "--quiet", cloneURL, dir).CombinedOutput(); err != nil {
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
	myFlags := &Flags{
		Directory:       dir,
		GitHubToken:     o.Token,
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

	prURL, err := o.createPR(repo, o.Branch, base)
	if err != nil {
		// PR may already exist if prExists check had a race or prior partial run.
		if open, _ := o.prExists(repo); open {
			result.Status = "pr-open"
			log.Warn().Str("repo", repo).Msg("PR already existed, branch updated")
			return result
		}
		result.Status = "error"
		result.Error = fmt.Errorf("create PR: %w", err)
		return result
	}

	result.Status = "pinned"
	result.PRUrl = prURL
	log.Warn().Str("repo", repo).Str("pr", prURL).Msg("PR opened")
	return result
}

func (o *OrgFlags) prExists(repo string) (bool, error) {
	// GitHub requires owner:branch format for the head filter.
	owner := strings.SplitN(repo, "/", 2)[0]
	url := fmt.Sprintf("https://api.github.com/repos/%s/pulls?head=%s:%s&state=open", repo, owner, o.Branch)
	body, err := GetGithubBody(o.Token, url)
	if err != nil {
		return false, err
	}
	items, ok := body.([]interface{})
	return ok && len(items) > 0, nil
}

func (o *OrgFlags) createPR(repo, head, base string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/pulls", repo)
	payload := map[string]string{
		"title": "chore: pin dependencies to immutable SHAs via ghat",
		"body":  "Automated dependency pinning by [ghat](https://github.com/JamesWoolfenden/ghat).\n\nPins GitHub Actions, pre-commit hooks, Terraform modules/providers, Dockerfiles, and Kubernetes images to SHA digests.",
		"head":  head,
		"base":  base,
	}
	b, _ := json.Marshal(payload)
	resp, err := postGithubBody(o.Token, url, b)
	if err != nil {
		return "", err
	}
	m, ok := resp.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("unexpected PR response")
	}
	prURL, _ := m["html_url"].(string)
	return prURL, nil
}

func (o *OrgFlags) waitForRateLimit() {
	url := "https://api.github.com/rate_limit"
	body, err := GetGithubBody(o.Token, url)
	if err != nil {
		return
	}
	m, ok := body.(map[string]interface{})
	if !ok {
		return
	}
	resources, ok := m["resources"].(map[string]interface{})
	if !ok {
		return
	}
	core, ok := resources["core"].(map[string]interface{})
	if !ok {
		return
	}
	remaining, _ := core["remaining"].(float64)
	reset, _ := core["reset"].(float64)
	if int(remaining) < o.Threshold {
		wait := time.Duration(int64(reset)-time.Now().Unix()+5) * time.Second
		if wait > 0 {
			log.Info().Int("remaining", int(remaining)).Dur("wait", wait).Msg("rate limit low — pausing")
			time.Sleep(wait)
		}
	}
}

func scanGaps(dir string) []string {
	var found []string
	entries, err := GetFiles(dir)
	if err != nil {
		return nil
	}
	for _, file := range entries {
		base := filepath.Base(file)
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
					found = append(found, fmt.Sprintf("%s | %s | %s:%d", p.label, file, base, i+1))
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
