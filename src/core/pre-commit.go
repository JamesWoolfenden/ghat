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

func (myFlags *Flags) UpdateHooks() error {
	var config *string
	var err error

	if config, err = myFlags.GetHook(); err != nil {
		return err
	}

	data, _ := os.ReadFile(*config)

	var m ConfigFile

	err = yaml.Unmarshal(data, &m)

	if err != nil {
		return fmt.Errorf("failed to unmarshall %s", *config)
	}

	var newRepos []Repo

	for _, item := range m.Repos {
		action := strings.Replace(item.Repo, "https://github.com/", "", 1)
		tag, err := GetLatestTag(action, myFlags.GitHubToken)

		if err != nil {
			log.Info().Msgf("failed to find %s", item.Repo)
			// i dont want to delete hook
			newRepos = append(newRepos, item)
			continue
		}

		myTag := tag.(map[string]interface{})

		commit := myTag["commit"].(map[string]interface{})

		item.Rev = commit["sha"].(string) // myTag["name"].(string)

		newRepos = append(newRepos, item)
	}

	newConfigFile := m
	newConfigFile.Repos = newRepos

	newData, err := yaml.Marshal(&newConfigFile)
	if err != nil {
		return fmt.Errorf("failed to marshal mew config")
	}

	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(string(data), string(newData), false)

	fmt.Println(dmp.DiffPrettyText(diffs))

	if !myFlags.DryRun {
		err = os.WriteFile(*config, newData, 0666)
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

	config := filepath.Join(myFlags.Directory, ".pre-commit-config.yaml")
	if _, err = os.Stat(config); err != nil {
		return nil, fmt.Errorf("pre-commit config not found %s", config)
	}

	return &config, nil
}
