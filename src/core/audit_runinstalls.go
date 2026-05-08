package core

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
)

type looseInstallRule struct {
	name   string
	re     *regexp.Regexp
	unless *regexp.Regexp
}

// looseInstallRules match tool installs in run:/RUN/shell text that float to a
// moving target rather than a pinned version. Anchored to the command, not to
// YAML structure, so the same rules work across workflows, Dockerfiles, action
// manifests and plain shell.
var looseInstallRules = []looseInstallRule{
	{
		name: "go-install",
		re:   regexp.MustCompile(`\bgo\s+(?:install|run)\b[^#\n]*?(\S+@(?:latest|main|master|HEAD|tip))`),
	},
	{
		name:   "pip-install",
		re:     regexp.MustCompile(`\bpip3?\s+install\b\s+([^\n#]+)`),
		unless: regexp.MustCompile(`==|\s-r\b|--requirement|\s-c\b|--constraint|\s-e\b|\./|\.whl|\btar\.gz\b`),
	},
	{
		name: "npx",
		re:   regexp.MustCompile(`\bnpx\s+(?:-y\s+|--yes\s+)?([@\w][\w./@-]*@(?:latest|next))`),
	},
	{
		name: "npm-install",
		re:   regexp.MustCompile(`\b(?:npm|pnpm|yarn)\s+(?:i|install|add|global add)\b[^#\n]*?(\S+@(?:latest|next))`),
	},
	{
		name:   "cargo-install",
		re:     regexp.MustCompile(`\bcargo\s+install\b\s+([^\n#]+)`),
		unless: regexp.MustCompile(`--version\b|--locked\b|--git\b.*--(?:rev|tag)\b`),
	},
	{
		name:   "gem-install",
		re:     regexp.MustCompile(`\bgem\s+install\b\s+([^\n#]+)`),
		unless: regexp.MustCompile(`\s-v\b|--version\b`),
	},
	{
		name: "curl-pipe-sh",
		re:   regexp.MustCompile(`\b(?:curl|wget)\b[^\n#]*\|\s*(?:sudo\s+)?(?:ba)?sh\b`),
	},
}

// findRunInstalls scans a file body for loose tool-install patterns and returns
// one label per hit, e.g. "go-install: golang.org/x/vuln/cmd/govulncheck@latest".
func findRunInstalls(body []byte) []string {
	var out []string
	for _, line := range strings.Split(string(body), "\n") {
		if t := strings.TrimSpace(line); t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		if isSuppressed(line) {
			continue
		}
		for _, r := range looseInstallRules {
			m := r.re.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			if r.unless != nil && r.unless.MatchString(line) {
				continue
			}
			detail := strings.TrimSpace(m[len(m)-1])
			if detail == "" {
				detail = strings.TrimSpace(m[0])
			}
			out = append(out, r.name+": "+detail)
		}
	}
	return out
}

// isRunInstallTarget reports whether a local file is worth scanning for loose
// installs: workflow YAML, composite-action manifests, Dockerfiles, and shell
// or Makefiles living under .github/.
func isRunInstallTarget(path string) bool {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	slashed := filepath.ToSlash(path)
	underCI := strings.Contains(slashed, ".github/") || strings.Contains(slashed, ".gitlab/")
	isYAML := ext == yamlExtension || ext == yamlAltExtension

	switch {
	case strings.HasPrefix(base, "Dockerfile"):
		return true
	case base == "action.yml" || base == "action.yaml":
		return true
	case isYAML && strings.HasSuffix(strings.TrimSuffix(base, ext), ".gitlab-ci"):
		return true
	case underCI && isYAML:
		return true
	case underCI && ext == ".sh":
		return true
	case underCI && (base == "Makefile" || base == "makefile"):
		return true
	}
	return false
}

// warnRunInstalls logs a warning for each loose install in body. Called from
// the rewriters (swot, stun) so users see floats ghat can't yet fix.
func warnRunInstalls(file string, body []byte) {
	for _, hit := range findRunInstalls(body) {
		log.Warn().Str("file", file).Msgf("unpinned install in script: %s", hit)
	}
}

// scanLocalRunInstalls walks the local file set for loose tool installs in
// workflow run: blocks, Dockerfiles, action manifests and .github shell.
func (f *Flags) scanLocalRunInstalls() []string {
	var out []string
	for _, file := range f.Entries {
		if !isRunInstallTarget(file) {
			continue
		}
		body, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(f.Directory, file)
		if err != nil {
			rel = file
		}
		for _, hit := range findRunInstalls(body) {
			out = append(out, rel+": "+hit)
		}
	}
	return out
}
