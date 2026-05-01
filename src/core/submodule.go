package core

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
)

const GitmodulesFile = ".gitmodules"

type Submodule struct {
	Name           string
	Path           string
	URL            string
	Suppressed     bool
	SuppressReason string
}

// parseGitmodules reads a .gitmodules file. The format is git-config INI but
// only `path` and `url` matter here, so a tiny scanner is enough and keeps
// the read side exec-free (and unit-testable without a git binary).
func parseGitmodules(file string) ([]Submodule, error) {
	f, err := os.Open(file) // #nosec G304 -- path is <directory>/.gitmodules, directory is user-supplied like every other command
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var subs []Submodule
	var cur *Submodule

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		raw := sc.Text()
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[submodule") {
			if cur != nil {
				subs = append(subs, *cur)
			}
			head := strings.TrimSpace(strings.SplitN(line, "#", 2)[0])
			name := strings.Trim(strings.TrimPrefix(strings.TrimSuffix(head, "]"), "[submodule"), ` "`)
			cur = &Submodule{Name: name}
			cur.Suppressed, cur.SuppressReason = parseSuppression(raw)
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		if cur == nil {
			continue
		}
		if ok, reason := parseSuppression(raw); ok {
			cur.Suppressed, cur.SuppressReason = true, reason
		}
		k, v, ok := strings.Cut(strings.SplitN(line, "#", 2)[0], "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(k) {
		case "path":
			cur.Path = strings.TrimSpace(v)
		case "url":
			cur.URL = strings.TrimSpace(v)
		}
	}
	if cur != nil {
		subs = append(subs, *cur)
	}
	return subs, sc.Err()
}

// currentGitlinkSHA returns the commit a submodule path is pinned to in HEAD.
// `ls-tree` reads the committed tree, so it works whether or not the submodule
// is initialised (a shallow clone of openssl has .gitmodules but no submodule
// checkouts).
func currentGitlinkSHA(repoDir, path string) (string, error) {
	// #nosec G204 -- args are discrete argv elements, never through a shell
	out, err := exec.Command("git", "-C", repoDir, "ls-tree", "HEAD", "--", path).Output()
	if err != nil {
		return "", gitErr("ls-tree", err)
	}
	fields := strings.Fields(string(out))
	if len(fields) < 3 || fields[1] != "commit" {
		return "", fmt.Errorf("%s is not a gitlink in HEAD", path)
	}
	return fields[2], nil
}

func setGitlinkSHA(repoDir, path, sha string) error {
	// #nosec G204
	cmd := exec.Command("git", "-C", repoDir, "update-index", "--add", "--cacheinfo", "160000,"+sha+","+path)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git update-index %s: %w: %s", path, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func gitErr(op string, err error) error {
	if ee, ok := err.(*exec.ExitError); ok {
		return fmt.Errorf("git %s: %w: %s", op, err, strings.TrimSpace(string(ee.Stderr)))
	}
	return fmt.Errorf("git %s: %w", op, err)
}

// latestSubmoduleSHA resolves the newest tag for a submodule URL and returns
// the commit it points at. GitHub goes via the REST API (so --token and the
// cache apply); everything else falls back to `git ls-remote` like sift does.
func (myFlags *Flags) latestSubmoduleSHA(url string) (sha, tag string, err error) {
	if action, ok := strings.CutPrefix(url, GitHubPrefix); ok {
		action = strings.TrimSuffix(action, ".git")
		t, err := GetLatestTag(action, myFlags.GitHubToken)
		if err != nil {
			return "", "", err
		}
		m, ok := t.(map[string]any)
		if !ok {
			return "", "", &castToMapError{object: "tag"}
		}
		commit := m["commit"].(map[string]any)
		return commit["sha"].(string), m["name"].(string), nil
	}
	return getLatestTagViaGit(url)
}

func (myFlags *Flags) UpdateSubmodules() error {
	dir, err := filepath.Abs(myFlags.Directory)
	if err != nil {
		return &absolutePathError{directory: myFlags.Directory, err: err}
	}

	config := filepath.Join(dir, GitmodulesFile)
	subs, err := parseGitmodules(config)
	if err != nil {
		if os.IsNotExist(err) {
			log.Info().Msgf("no %s found in %s, skipping", GitmodulesFile, dir)
			return nil
		}
		return &readConfigError{config: &config, err: err}
	}

	var changed int
	for _, s := range subs {
		if s.Path == "" || s.URL == "" {
			continue
		}

		if s.Suppressed {
			log.Info().Str("submodule", s.Path).Str("reason", s.SuppressReason).Msg("skipping suppressed submodule")
			continue
		}

		current, err := currentGitlinkSHA(dir, s.Path)
		if err != nil {
			log.Info().Err(err).Msgf("skip %s", s.Path)
			continue
		}

		latest, tag, err := myFlags.latestSubmoduleSHA(s.URL)
		if err != nil {
			log.Info().Err(err).Msgf("failed to resolve %s", s.URL)
			continue
		}

		if current == latest {
			fmt.Printf("  %-30s %s (at %s)\n", s.Path, current[:12], tag)
			continue
		}

		fmt.Printf("~ %-30s %s -> %s (%s)\n", s.Path, current[:12], latest[:12], tag)
		changed++

		if myFlags.DryRun {
			continue
		}
		if err := setGitlinkSHA(dir, s.Path, latest); err != nil {
			if myFlags.ContinueOnError {
				log.Warn().Err(err).Msgf("failed to pin %s", s.Path)
				continue
			}
			return err
		}
	}

	if myFlags.DryRun && changed > 0 {
		fmt.Printf("\ndry-run: %d submodule(s) would be re-pinned\n", changed)
	}
	return nil
}
