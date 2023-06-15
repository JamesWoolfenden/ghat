package core

import (
	"encoding/json"
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

func Files(directory *string, gitHubToken string) ([]os.DirEntry, error) {

	matches, err := os.ReadDir(*directory)

	if err != nil {
		log.Error().Msgf("failed to read %s", *directory)
	}

	var ghat []os.DirEntry

	entries, directory, err2 := GetGHA(directory, matches, ghat)
	if err2 != nil {
		return entries, err2
	}

	for _, gha := range entries {
		file := filepath.Join(*directory, gha.Name())
		err = UpdateFile(&file, gitHubToken)

		if err != nil {
			log.Warn().Msgf("failed to update %s", gha.Name())
		}
	}

	return nil, nil
}

func GetGHA(directory *string, matches []os.DirEntry, ghat []os.DirEntry) ([]os.DirEntry, *string, error) {
	for _, match := range matches {
		if match.IsDir() {
			if strings.Contains(match.Name(), ".github") {
				log.Print(match.Name())
				AbsDir, _ := filepath.Abs(*directory)
				newDirectory := filepath.Join(AbsDir, match.Name(), "workflows")
				if _, err := os.Stat(newDirectory); err == nil {
					ghat, err = os.ReadDir(newDirectory)

					if err != nil {
						return nil, &newDirectory, fmt.Errorf("no files found %w", err)
					}

					return ghat, &newDirectory, nil
				}
			}
		} else {
			if strings.Contains(match.Name(), ".yml") || (strings.Contains(match.Name(), ".yaml")) {
				ghat = append(ghat, match)
			}
		}
	}
	return ghat, directory, nil
}

func UpdateFile(file *string, gitHubToken string) error {
	buffer, err := os.ReadFile(*file)
	replacement := string(buffer)

	var newUrl string

	if err != nil {
		return fmt.Errorf("failed to open file %w", err)
	}

	r := regexp.MustCompile(`uses:(.*)`)
	matches := r.FindAllStringSubmatch(string(buffer), -1)
	for _, match := range matches {
		action := strings.Split(match[1], "@")

		action[0] = strings.TrimSpace(action[0])
		msg, err2 := getPayload(action[0], gitHubToken)

		if err2 != nil {
			splitter := strings.SplitN(action[0], "/", 3)
			newUrl = splitter[0] + "/" + splitter[1]
			msg, err2 = getPayload(newUrl, gitHubToken)
			if err2 != nil {
				log.Warn().Msgf("failed to retrieve back %s", err2)

				continue
			}
		}

		if msg["tag_name"] != nil {
			tag := msg["tag_name"].(string)

			url := action[0]

			if newUrl != "" {
				url = newUrl
			}

			body, err := getHash(url, tag, gitHubToken)

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

	newBuffer := []byte(replacement)

	err = os.WriteFile(*file, newBuffer, 0644)

	if err != nil {
		return fmt.Errorf("failed to write err %w", err)
	}

	return nil
}

func getPayload(action string, gitHubToken string) (map[string]interface{}, error) {
	url := "https://api.github.com/repos/" + action + "/releases/latest"
	return GetBody(gitHubToken, url)
}

func getHash(action string, tag string, gitHubToken string) (map[string]interface{}, error) {
	url := "https://api.github.com/repos/" + action + "/git/ref/tags/" + tag
	return GetBody(gitHubToken, url)
}

func GetBody(gitHubToken string, url string) (map[string]interface{}, error) {
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

	var msg map[string]interface{}

	err := json.Unmarshal(body, &msg)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal %w", err)
	}

	return msg, nil
}
