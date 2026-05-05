package core

import (
	_ "embed"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Substitution struct {
	From string `yaml:"from"`
	To   string `yaml:"to"`
}

// InputUpgrade rewrites a `with:` input when an action is pinned to a new
// major version that drops support for the old input value.
// To may be a literal version ("v2.12.1") or "latest:owner/repo" to
// fetch the current latest release from the GitHub API at run time.
type InputUpgrade struct {
	Action      string `yaml:"action"`       // e.g. "golangci/golangci-lint-action"
	Input       string `yaml:"input"`        // e.g. "version"
	FromPattern string `yaml:"from_pattern"` // regex matched against the current value
	To          string `yaml:"to"`           // literal version or "latest:owner/repo"
}

type GhatConfig struct {
	Substitutions []Substitution `yaml:"substitutions"`
	InputUpgrades []InputUpgrade `yaml:"input_upgrades"`
}

//go:embed substitutions.yml
var defaultSubstitutionsData []byte

// LoadConfig merges built-in substitutions.yml, ~/.ghat.yml (global),
// and <dir>/.ghat.yml (local). Later entries win on duplicate From values.
func LoadConfig(dir string) GhatConfig {
	var merged GhatConfig
	_ = yaml.Unmarshal(defaultSubstitutionsData, &merged)

	overlay := func(path string) {
		if cfg, err := loadConfigFile(path); err == nil {
			merged.Substitutions = append(merged.Substitutions, cfg.Substitutions...)
			merged.InputUpgrades = append(merged.InputUpgrades, cfg.InputUpgrades...)
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		overlay(filepath.Join(home, ".ghat.yml"))
	}
	if dir != "" {
		overlay(filepath.Join(dir, ".ghat.yml"))
	}
	return merged
}

func loadConfigFile(path string) (GhatConfig, error) {
	var cfg GhatConfig
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	return cfg, yaml.Unmarshal(data, &cfg)
}

// applySubstitution returns the substituted value for s if a matching rule
// exists, otherwise returns s unchanged.
func (f *Flags) applySubstitution(s string) (result string, changed bool) {
	for _, sub := range f.Substitutions {
		if s == sub.From {
			return sub.To, true
		}
	}
	return s, false
}

// applyRepoSubstitution applies substitutions to a pre-commit repo URL.
// Substitution From/To are owner/repo paths; the full URL is reconstructed.
func (f *Flags) applyRepoSubstitution(repoURL string) (string, bool) {
	path := repoURL
	prefix := ""
	if after, ok := strings.CutPrefix(repoURL, GitHubPrefix); ok {
		path = after
		prefix = GitHubPrefix
	}
	path = strings.TrimSuffix(path, ".git")
	if sub, changed := f.applySubstitution(path); changed {
		return prefix + sub, true
	}
	return repoURL, false
}
