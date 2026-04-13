package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/sergi/go-diff/diffmatchpatch"
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
	sha string
	tag string
}

// rewritePreCommitRevs replaces each `rev:` line with `<sha> # <tag>` for repos
// present in pins. Line-based so comments and formatting are preserved
// (consistent with swot's behaviour in gha.go).
func rewritePreCommitRevs(data string, pins map[string]revPin) string {
	lines := strings.Split(data, "\n")
	var currentRepo string

	for i, line := range lines {
		trimmed := strings.TrimSpace(strings.SplitN(line, "#", 2)[0])

		if after, ok := strings.CutPrefix(trimmed, "- repo:"); ok {
			currentRepo = strings.TrimSpace(after)
			continue
		}

		if !strings.HasPrefix(trimmed, "rev:") {
			continue
		}

		p, ok := pins[currentRepo]
		if !ok {
			continue
		}

		indent := line[:strings.Index(line, "rev:")]
		lines[i] = indent + "rev: " + p.sha + " # " + p.tag
	}

	return strings.Join(lines, "\n")
}

func (myFlags *Flags) UpdateHooks() error {
	var config *string
	var err error

	if config, err = myFlags.GetHook(); err != nil {
		return &getHookError{err: err}
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

	// Resolve latest tag name + commit SHA for each GitHub-hosted repo.
	pins := map[string]revPin{}

	for _, item := range m.Repos {
		if !strings.HasPrefix(item.Repo, GitHubPrefix) {
			continue
		}

		action := strings.TrimPrefix(item.Repo, GitHubPrefix)
		tag, err := GetLatestTag(action, myFlags.GitHubToken)

		if err != nil {
			log.Info().Msgf("failed to find %s", item.Repo)
			continue
		}

		myTag := tag.(map[string]interface{})
		commit := myTag["commit"].(map[string]interface{})
		pins[item.Repo] = revPin{
			sha: commit["sha"].(string),
			tag: myTag["name"].(string),
		}
	}

	replacement := rewritePreCommitRevs(string(data), pins)

	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(string(data), replacement, false)

	fmt.Println(dmp.DiffPrettyText(diffs))

	if !myFlags.DryRun {
		err = os.WriteFile(*config, []byte(replacement), FilePermissions)
		if err != nil {
			log.Info().Msgf("failed to write %s", *config)

			return err
		}
	}

	return nil
}

func (myFlags *Flags) GetHook() (*string, error) {
	var err error
	myFlags.Directory, err = filepath.Abs(myFlags.Directory)

	if err != nil {
		return nil, fmt.Errorf("failed to make sense of directory %s", myFlags.Directory)
	}

	fileInfo, err := os.Stat(myFlags.Directory)
	if err != nil {
		return nil, fmt.Errorf("please specify a valid directory: %s", myFlags.Directory)
	}

	if !fileInfo.IsDir() {
		return nil, fmt.Errorf("please specify a directory")
	}

	config := filepath.Join(myFlags.Directory, PreCommitConfigFile)
	if _, err = os.Stat(config); err != nil {
		return nil, fmt.Errorf("pre-commit config not found %s", config)
	}

	return &config, nil
}
