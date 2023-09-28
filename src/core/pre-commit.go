package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

func (myFlags *Flags) UpdateHooks() error {
	var config *string
	var err error

	if config, err = myFlags.GetHook(); err != nil {
		return err
	}

	data, _ := os.ReadFile(*config)
	m := make(map[string]interface{})

	err = yaml.Unmarshal(data, &m)
	var newRepos []interface{}

	for _, item := range m["repos"].([]interface{}) {
		newItem := item.(map[string]interface{})
		action := strings.Replace(newItem["repo"].(string), "https://github.com/", "", 1)
		tag, err := GetLatestTag(action, myFlags.GitHubToken)

		if err != nil {
			log.Info().Msgf("failed to find %s", newItem["repo"].(string))
			//i dont want to delete hook
			newRepos = append(newRepos, item)
			continue
		}

		myTag := tag.(map[string]interface{})
		commit := myTag["commit"].(map[string]interface{})
		newItem["rev"] = commit["sha"].(string) //+ " #" + myTag["name"].(string)

		newRepos = append(newRepos, newItem)
	}

	m["repos"] = newRepos
	data, err = yaml.Marshal(&m)
	err = os.WriteFile(*config, data, 0666)
	if err != nil {
		log.Info().Msgf("failed to write %s", *config)
		return err
	}

	fmt.Printf("updated %s", *config)
	return nil
}

func (myFlags *Flags) GetHook() (*string, error) {
	var err error
	myFlags.Directory, err = filepath.Abs(myFlags.Directory)

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
