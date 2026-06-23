package core

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

type Hook struct {
	ID                      string   `yaml:"id"`
	Name                    string   `yaml:"name,omitempty"`
	Entry                   string   `yaml:"entry,omitempty"`
	Language                string   `yaml:"language,omitempty"`
	Files                   string   `yaml:"files,omitempty"`
	Exclude                 string   `yaml:"exclude,omitempty"`
	Types                   []string `yaml:"types,omitempty"`
	TypesOr                 []string `yaml:"types_or,omitempty"`
	ExcludeTypes            []string `yaml:"exclude_types,omitempty"`
	AlwaysRun               *bool    `yaml:"always_run,omitempty"`
	FailFast                *bool    `yaml:"fail_fast,omitempty"`
	Verbose                 *bool    `yaml:"verbose,omitempty"`
	PassFilenames           *bool    `yaml:"pass_filenames,omitempty"`
	RequireSerial           *bool    `yaml:"require_serial,omitempty"`
	Description             string   `yaml:"description,omitempty"`
	LanguageVersion         string   `yaml:"language_version,omitempty"`
	MinimumPrecommitVersion string   `yaml:"minimum_pre_commit_version,omitempty"`
	Args                    []string `yaml:"args,omitempty"`
	Stages                  []string `yaml:"stages,omitempty"`
}

type Repo struct {
	Hooks []Hook `yaml:"hooks"`
	Repo  string `yaml:"repo"`
	Rev   string `yaml:"rev,omitempty"`
}

type ConfigFile struct {
	DefaultLanguageVersion struct {
		Python string `yaml:"python"`
	} `yaml:"default_language_version"`
	Repos []Repo `yaml:"repos"`
}

// Add constants for repeated values
const (
	PreCommitConfigFile = ".pre-commit-config.yaml"
	GitHubPrefix        = "https://github.com/"
	FilePermissions     = 0666
)

type revPin struct {
	sha    string
	tag    string
	newURL string // if non-empty, rewrite the repo: line too
}

// rewritePreCommitRevs replaces each `rev:` line with `<sha> # <tag>` for repos
// present in pins. Line-based so comments and formatting are preserved
// (consistent with swot's behaviour in gha.go). Returns the rewritten data and
// the set of repo URLs the line-parser recognised, so the caller can detect a
// mismatch between yaml.Unmarshal and this parser.
func rewritePreCommitRevs(data string, pins map[string]revPin) (string, map[string]bool) {
	lines := strings.Split(data, "\n")
	seen := map[string]bool{}
	var currentRepo string
	var suppressCurrent bool
	var suppressCurrentReason string

	for i, line := range lines {
		trimmed := strings.TrimSpace(strings.SplitN(line, "#", 2)[0])

		// pre-commit autoupdate emits `-   repo:` (4-col indent), not `- repo:`.
		bare := strings.TrimLeft(strings.TrimPrefix(trimmed, "-"), " ")
		if after, ok := strings.CutPrefix(bare, "repo:"); ok {
			currentRepo = strings.Trim(strings.TrimSpace(after), `"'`)
			seen[currentRepo] = true
			suppressCurrent, suppressCurrentReason = parseSuppression(line)
			if p, ok := pins[currentRepo]; ok && p.newURL != "" {
				indent := line[:strings.Index(line, "repo:")]
				lines[i] = indent + "repo: " + p.newURL
			}
			continue
		}

		if !strings.HasPrefix(bare, "rev:") {
			continue
		}

		if suppressCurrent {
			log.Info().Str("repo", currentRepo).Str("reason", suppressCurrentReason).Msg("skipping suppressed pre-commit rev")
			continue
		}
		if ok, reason := parseSuppression(line); ok {
			log.Info().Str("repo", currentRepo).Str("reason", reason).Msg("skipping suppressed pre-commit rev")
			continue
		}

		p, ok := pins[currentRepo]
		if !ok {
			continue
		}

		indent := line[:strings.Index(line, "rev:")]
		lines[i] = indent + "rev: " + p.sha + " # " + p.tag
	}

	return strings.Join(lines, "\n"), seen
}

func (f *Flags) UpdateHooks() error {
	var config *string
	var err error

	if config, err = f.GetHook(); err != nil {
		return &getHookError{err: err}
	}
	if config == nil {
		log.Info().Msgf("no %s found in %s, skipping", PreCommitConfigFile, f.Directory)
		return nil
	}

	data, err := os.ReadFile(*config)
	if err != nil {
		return &readConfigError{config, err}
	}

	var m ConfigFile

	err = yaml.Unmarshal(data, &m)

	if err != nil {
		return &unmarshalJSONError{err}
	}

	// Resolve latest tag name + commit SHA for each remote repo.
	// GitHub goes via the API (rate-limit aware, uses --github-token).
	// Anything else falls back to `git ls-remote`, which inherits the
	// caller's git credential helpers — so self-hosted GitLab / Gitea /
	// Bitbucket just work if `git clone` would.
	pins := map[string]revPin{}

	for _, item := range m.Repos {
		if !strings.Contains(item.Repo, "://") {
			// `local`, `meta`, or a bare path — nothing to resolve.
			continue
		}

		repoURL := item.Repo
		newURL := ""
		if sub, changed := f.applyRepoSubstitution(repoURL); changed {
			log.Warn().Str("from", repoURL).Str("to", sub).Msg("substituting pre-commit repo")
			newURL = sub
			repoURL = sub
		}

		if strings.HasPrefix(repoURL, GitHubPrefix) {
			// pre-commit accepts `https://github.com/org/repo.git` but the
			// REST API does not — /repos/org/repo.git/tags is a 404.
			action := strings.TrimSuffix(strings.TrimPrefix(repoURL, GitHubPrefix), ".git")
			tag, err := GetLatestTag(action, f.GitHubToken)

			if err != nil {
				log.Info().Msgf("failed to find %s", item.Repo)
				continue
			}

			myTag := tag.(map[string]interface{})
			commit := myTag["commit"].(map[string]interface{})
			pins[item.Repo] = revPin{
				sha:    commit["sha"].(string),
				tag:    myTag["name"].(string),
				newURL: newURL,
			}
			continue
		}

		sha, tag, err := getLatestTagViaGit(repoURL)
		if err != nil {
			log.Info().Err(err).Msgf("failed to resolve %s via git ls-remote", item.Repo)
			continue
		}
		pins[item.Repo] = revPin{sha: sha, tag: tag, newURL: newURL}
	}

	replacement, seen := rewritePreCommitRevs(string(data), pins)
	for repo := range pins {
		if !seen[repo] {
			log.Warn().Str("repo", repo).Msg("resolved pin but line-parser found no matching repo: entry — please report this")
		}
	}

	f.printDiff(*config, string(data), replacement)

	if !f.DryRun {
		err = os.WriteFile(*config, []byte(replacement), FilePermissions)
		if err != nil {
			log.Info().Msgf("failed to write %s", *config)

			return err
		}
	}

	return nil
}

// getLatestTagViaGit shells out to `git ls-remote --tags`. Git is a hard
// dependency of pre-commit anyway, and exec means we get the user's
// credential helpers (osxkeychain / GCM / .netrc) for free — go-git would
// need explicit auth plumbing per host. --sort is client-side (git ≥2.18).
func getLatestTagViaGit(repoURL string) (sha, tag string, err error) {
	// #nosec G204 — repoURL comes from a tracked .pre-commit-config.yaml the
	// user is already trusting pre-commit to clone; passed as a discrete
	// argv element, never through a shell.
	cmd := exec.Command("git", "ls-remote", "--tags", "--sort=-version:refname", repoURL)
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return "", "", fmt.Errorf("git ls-remote %s: %w: %s", repoURL, err, strings.TrimSpace(string(ee.Stderr)))
		}
		return "", "", fmt.Errorf("git ls-remote %s: %w", repoURL, err)
	}
	return parseLsRemoteTags(string(out))
}

// parseLsRemoteTags picks the highest tag from `git ls-remote --tags
// --sort=-version:refname` output. Annotated tags appear twice — once as the
// tag object and once peeled (`^{}`) to the commit; the peeled SHA is the one
// pre-commit needs. Lightweight tags appear once and already point at the
// commit. Sort order puts the highest version first regardless of which form
// arrives first, so build both maps and then read order[0].
func parseLsRemoteTags(out string) (sha, tag string, err error) {
	peeled := map[string]string{}
	direct := map[string]string{}
	var order []string

	for _, line := range strings.Split(out, "\n") {
		sha, ref, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		name, ok := strings.CutPrefix(ref, "refs/tags/")
		if !ok {
			continue
		}
		if base, isPeel := strings.CutSuffix(name, "^{}"); isPeel {
			peeled[base] = sha
			continue
		}
		direct[name] = sha
		order = append(order, name)
	}

	if len(order) == 0 {
		return "", "", fmt.Errorf("no tags")
	}

	tag = order[0]
	if s, ok := peeled[tag]; ok {
		return s, tag, nil
	}
	return direct[tag], tag, nil
}

func (f *Flags) GetHook() (*string, error) {
	var err error
	f.Directory, err = filepath.Abs(f.Directory)

	if err != nil {
		return nil, fmt.Errorf("failed to make sense of directory %s", f.Directory)
	}

	fileInfo, err := os.Stat(f.Directory)
	if err != nil {
		return nil, fmt.Errorf("please specify a valid directory: %s", f.Directory)
	}

	if !fileInfo.IsDir() {
		return nil, fmt.Errorf("please specify a directory")
	}

	config := filepath.Join(f.Directory, PreCommitConfigFile)
	if _, err = os.Stat(config); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("pre-commit config not found %s", config)
	}

	return &config, nil
}
