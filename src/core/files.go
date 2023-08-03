package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/sergi/go-diff/diffmatchpatch"
)

// Files updates all actions in a directory.
func (myFlags *Flags) Files() ([]os.DirEntry, error) {

	matches, err := os.ReadDir(myFlags.Directory)

	if err != nil {
		log.Error().Msgf("failed to read %s", myFlags.Directory)
	}

	var ghat []os.DirEntry

	entries, err2 := myFlags.GetGHA(matches, ghat)
	if err2 != nil {
		return entries, err2
	}

	for _, gha := range entries {
		myFlags.File = filepath.Join(myFlags.Directory, gha.Name())
		err = myFlags.UpdateFile()

		if err != nil {
			log.Warn().Msgf("failed to update %s", gha.Name())
		}
	}

	return nil, nil
}

// GetGHA gets all the actions in a directory
func (myFlags *Flags) GetGHA(matches []os.DirEntry, ghat []os.DirEntry) ([]os.DirEntry, error) {
	for _, match := range matches {
		if match.IsDir() {
			if strings.Contains(match.Name(), ".github") {
				log.Print(match.Name())
				AbsDir, _ := filepath.Abs(myFlags.Directory)
				newDirectory := filepath.Join(AbsDir, match.Name(), "workflows")
				if _, err := os.Stat(newDirectory); err == nil {
					ghat, err = os.ReadDir(newDirectory)
					myFlags.Directory = newDirectory

					if err != nil {
						return nil, fmt.Errorf("no files found %w", err)
					}

					return ghat, nil
				}
			}
		} else {
			if strings.Contains(match.Name(), ".yml") || (strings.Contains(match.Name(), ".yaml")) {
				ghat = append(ghat, match)
			}
		}
	}

	return ghat, nil
}

// UpdateFile updates am action with latest dependencies
func (myFlags *Flags) UpdateFile() error {
	buffer, err := os.ReadFile(myFlags.File)
	replacement := string(buffer)

	var newUrl string

	if err != nil {
		return fmt.Errorf("failed to open file %w", err)
	}

	r := regexp.MustCompile(`uses:(.*)`)
	matches := r.FindAllStringSubmatch(string(buffer), -1)
	for _, match := range matches {

		//is path
		if strings.Contains(match[1], ".github") {
			continue
		}

		action := strings.Split(match[1], "@")

		action[0] = strings.TrimSpace(action[0])
		body, err2 := getPayload(action[0], myFlags.GitHubToken, &myFlags.Days)

		if err2 != nil {
			splitter := strings.SplitN(action[0], "/", 3)
			newUrl = splitter[0] + "/" + splitter[1]
			body, err2 = getPayload(newUrl, myFlags.GitHubToken, &myFlags.Days)
			if err2 != nil {
				log.Warn().Msgf("failed to retrieve back %s", err2)

				continue
			}
		}

		msg, ok := body.(map[string]interface{})

		if !ok {
			return errors.New("failed to assert map[string]interface{}")
		}

		if msg["tag_name"] != nil {
			tag := msg["tag_name"].(string)

			url := action[0]

			if newUrl != "" {
				url = newUrl
			}

			payload, err := getHash(url, tag, myFlags.GitHubToken)
			body := payload.(map[string]interface{})

			if err != nil {
				log.Warn().Msgf("failed to retrieve commit hash %s for %s", err, action[0])
				continue
			}

			object, ok := body["object"].(map[string]interface{})
			if !ok {
				log.Warn().Msgf("failed to assert map of string %s", err)
				continue
			}

			sha := object["sha"].(string)
			if !ok {
				log.Warn().Msgf("failed to assert string %s", err)
				continue
			}

			oldAction := action[0] + "@" + action[1]
			newAction := action[0] + "@" + sha + " # " + tag //GET /repos/{owner}/{repo}/git/ref/tags/{tag_name}

			replacement = strings.ReplaceAll(replacement, oldAction, newAction)
		} else {
			log.Warn().Msgf("tag field empty skipping %s", action[0])
		}
	}

	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(string(buffer), replacement, false)

	fmt.Println(dmp.DiffPrettyText(diffs))

	if !myFlags.DryRun {
		newBuffer := []byte(replacement)

		err = os.WriteFile(myFlags.File, newBuffer, 0644)

		if err != nil {
			return fmt.Errorf("failed to write err %w", err)
		}
	}

	return nil
}

func getPayload(action string, gitHubToken string, days *int) (interface{}, error) {
	if *days == 0 {
		url := "https://api.github.com/repos/" + action + "/releases/latest"
		return GetBody(gitHubToken, url)
	}

	return GetReleases(action, gitHubToken, days)
}

func getHash(action string, tag string, gitHubToken string) (interface{}, error) {
	url := "https://api.github.com/repos/" + action + "/git/ref/tags/" + tag
	return GetBody(gitHubToken, url)
}

// GetBody requests a URL using gitHub PAT for auth
func GetBody(gitHubToken string, url string) (interface{}, error) {
	var body []byte

	if gitHubToken != "" {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("request failed %w", err)
		}

		req.Header.Add("Authorization", "Bearer "+gitHubToken)
		client := &http.Client{}
		resp, err := client.Do(req)

		if resp == nil {
			return nil, fmt.Errorf("api failed to respond")
		}

		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("api failed with %d", resp.StatusCode)
		}

		defer func(Body io.ReadCloser) {
			_ = Body.Close()
		}(resp.Body)

		if err != nil {
			return nil, fmt.Errorf("client failed %w", err)
		}

		body, err = io.ReadAll(resp.Body)

		if err != nil {
			return nil, fmt.Errorf("failed to read body %w", err)
		}

	} else {
		log.Warn().Msgf("failing back to anonymous auth")
		resp, err := http.Get(url)
		if err != nil {
			return nil, fmt.Errorf("failed to get url %w", err)
		}

		body, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read body %w", err)
		}
	}

	var msg interface{}

	err := json.Unmarshal(body, &msg)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal %w", err)
	}

	return msg, nil
}
