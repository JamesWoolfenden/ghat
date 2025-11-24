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
	"time"

	"github.com/rs/zerolog/log"
	"github.com/sergi/go-diff/diffmatchpatch"
)

const (
	githubWorkflowPath = ".github/workflows"
	terraformDir       = ".terraform"
	yamlExtension      = ".yml"
	yamlAltExtension   = ".yaml"
)

type readFilesError struct {
	err error
}

func (m *readFilesError) Error() string {
	return fmt.Sprintf("failed to read files: %s", m.err)
}

type absolutePathError struct {
	directory string
	err       error
}

func (m *absolutePathError) Error() string {
	return fmt.Sprintf("failed to get absolute path: %v %s ", m.err, m.directory)
}

func GetFiles(dir string) ([]string, error) {
	Entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, &readFilesError{err}
	}

	var ParsedEntries []string

	for _, entry := range Entries {
		AbsDir, err := filepath.Abs(dir)
		if err != nil {
			return nil, &absolutePathError{dir, err}
		}
		gitDir := filepath.Join(AbsDir, ".git")

		if entry.IsDir() {

			newDir := filepath.Join(AbsDir, entry.Name())

			if !(strings.Contains(newDir, terraformDir)) && newDir != gitDir {
				newEntries, err := GetFiles(newDir)

				if err != nil {
					return nil, err
				}

				ParsedEntries = append(ParsedEntries, newEntries...)
			}
		} else {
			myFile := filepath.Join(dir, entry.Name())
			if !(strings.Contains(myFile, terraformDir)) {
				ParsedEntries = append(ParsedEntries, myFile)
			}
		}
	}

	return ParsedEntries, nil
}

func (myFlags *Flags) UpdateGHAS() error {
	var err error
	myFlags.Entries = myFlags.GetGHA()

	for _, gha := range myFlags.Entries {
		err = myFlags.UpdateGHA(gha)

		if err != nil {
			return &ghaUpdateError{gha}
		}
	}

	return nil
}

// GetGHA gets all the actions in a directory
func (myFlags *Flags) GetGHA() []string {
	var ghat []string

	for _, match := range myFlags.Entries {
		match, _ = filepath.Abs(match)
		entry, _ := os.Stat(match)
		if strings.Contains(match, githubWorkflowPath) && !entry.IsDir() {
			if strings.Contains(match, yamlExtension) || (strings.Contains(match, yamlAltExtension)) {
				ghat = append(ghat, match)
			}
		}
	}

	return ghat
}

// UpdateGHA updates am action with latest dependencies
func (myFlags *Flags) UpdateGHA(file string) error {
	buffer, err := os.ReadFile(file)
	if err != nil {
		return &ghaFileError{file}
	}

	replacement := string(buffer)

	var newUrl string

	r := regexp.MustCompile(`uses:(.*)`)
	matches := r.FindAllStringSubmatch(string(buffer), -1)
	for _, match := range matches {

		//is path
		if strings.Contains(match[1], ".github") {
			continue
		}

		action := strings.Split(match[1], "@")

		action[0] = strings.TrimSpace(action[0])
		body, err := getPayload(action[0], myFlags.GitHubToken, myFlags.Days)

		if err != nil {
			splitter := strings.SplitN(action[0], "/", 3)
			newUrl = splitter[0] + "/" + splitter[1]
			body, err = getPayload(newUrl, myFlags.GitHubToken, myFlags.Days)
			if err != nil {
				if myFlags.ContinueOnError {
					log.Info().Err(err)
					continue
				}
				return fmt.Errorf("failed to retrieve data for action %s with %s", action[0], err)
			}
		}

		msg, ok := body.(map[string]interface{})

		if !ok {
			return &castToMapError{"body"}
		}

		if msg["tag_name"] != nil {
			tag := msg["tag_name"].(string)

			url := action[0]

			if newUrl != "" {
				url = newUrl
			}

			payload, err := getHash(url, tag, myFlags.GitHubToken)
			if err != nil {
				log.Warn().Msgf("failed to retrieve commit hash %s for %s", err, action[0])
				continue
			}

			body, ok := payload.(map[string]interface{})
			if !ok {
				log.Warn().Msgf("Payload is not expected map %s", body)
				continue
			}

			object, ok := body["object"].(map[string]interface{})
			if !ok {
				log.Warn().Msgf("failed to assert map of string %s", err)
				continue
			}

			sha, ok := object["sha"].(string)
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

		err = os.WriteFile(file, newBuffer, 0644)

		if err != nil {
			return &writeGHAError{file}
		}
	}

	return nil
}

func getPayload(action string, gitHubToken string, days *uint) (interface{}, error) {

	if days == nil {
		return nil, &daysParameterError{}
	}

	if *days == 0 {
		return GetLatestRelease(action, gitHubToken)
	}

	return GetReleases(action, gitHubToken, days)
}

func GetLatestRelease(action string, gitHubToken string) (interface{}, error) {
	url := "https://api.github.com/repos/" + action + "/releases/latest"
	return GetGithubBody(gitHubToken, url)
}

func GetLatestTag(action string, gitHubToken string) (interface{}, error) {
	url := "https://api.github.com/repos/" + action + "/tags"
	tags, err := GetGithubBody(gitHubToken, url)
	tagged, ok := tags.([]interface{})

	if !ok {
		return nil, fmt.Errorf("failed to assert slice %s", tags)
	}

	return tagged[0].(map[string]interface{}), err
}

func getHash(action string, tag string, gitHubToken string) (interface{}, error) {
	url := "https://api.github.com/repos/" + action + "/git/ref/tags/" + tag
	return GetGithubBody(gitHubToken, url)
}

// GetGithubBodyWithCache fetches data from GitHub API with caching support
func GetGithubBodyWithCache(token, url string, cache *Cache) (interface{}, error) {
	// Try cache first
	if cache != nil && cache.enabled {
		if cached, found := cache.Get(url); found {
			log.Debug().Str("url", url).Msg("Using cached response")
			return cached, nil
		}
	}

	// Fetch from API
	log.Debug().Str("url", url).Msg("Fetching from GitHub API")
	data, err := GetGithubBody(token, url)
	if err != nil {
		return nil, err
	}

	// Store in cache
	if cache != nil && cache.enabled {
		if err := cache.Set(url, data); err != nil {
			log.Warn().Err(err).Msg("Failed to cache response")
		}
	}

	return data, nil
}

// GetGithubBody fetches data from GitHub API (existing function, keep as-is for compatibility)
func GetGithubBody(token, url string) (interface{}, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication if token provided
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	// Set user agent
	req.Header.Set("User-Agent", "ghat")
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check rate limiting
	if resp.StatusCode == 403 {
		if resp.Header.Get("X-RateLimit-Remaining") == "0" {
			resetTime := resp.Header.Get("X-RateLimit-Reset")
			return nil, fmt.Errorf("GitHub API rate limit exceeded, resets at %s", resetTime)
		}
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return result, nil
}
