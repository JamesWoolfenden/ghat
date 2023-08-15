package core

import (
	"fmt"
	"time"
)

func GetReleases(action string, gitHubToken string, days *int) (map[string]interface{}, error) {
	now := time.Now()
	interval := time.Duration(*days * 24 * 60 * 60 * 1000 * 1000 * 1000)
	limit := now.Add(-interval)

	url := "https://api.github.com/repos/" + action + "/releases"
	temp, err := GetGithubBody(gitHubToken, url)

	if err != nil {
		return nil, fmt.Errorf("failed to request list of releases %w", err)
	}

	bodies, ok := temp.([]interface{})

	if !ok {
		return nil, fmt.Errorf("api query did not return list: %s", bodies)
	}

	for _, body := range bodies {
		release := body.(map[string]interface{})
		temp := release["published_at"].(string)

		released, _ := time.Parse(time.RFC3339, temp)
		if released.Before(limit) {
			return release, nil
		}
	}

	return nil, err
}
