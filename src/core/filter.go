package core

import (
	"fmt"
	"time"
)

func GetReleases(action string, gitHubToken string, days int) (string, error) {
	now := time.Now()
	interval := time.Duration(14 * 24 * time.Hour)
	limit := now.Add(-interval)

	url := "https://api.github.com/repos/" + action + "/releases"
	temp, err := GetBody(gitHubToken, url)

	bodies := temp.([]interface{})

	for _, body := range bodies {
		release := body.(map[string]interface{})
		temp := release["published_at"].(string)

		released, _ := time.Parse(time.RFC3339, temp)
		if released.Before(limit) {
			fmt.Println(released)
		}
	}

	if err != nil {
		return "", fmt.Errorf("failed to request list of releases %w", err)
	}

	return "", err
}
