package core

import (
	"fmt"
	"time"
)

const (
	dayInNanos int64 = 24 * 60 * 60 * 1000 * 1000 * 1000
	apiBaseURL       = "https://api.github.com/repos/"
)

type githubTokenIsEmptyError struct{}

func (e githubTokenIsEmptyError) Error() string {
	return "github token is empty"
}

type timeParsingError struct {
	err error
}

func (e timeParsingError) Error() string {
	return fmt.Sprintf("failed to parse time %v", e.err)
}

type daysParameterError struct{}

func (e daysParameterError) Error() string {
	return "days parameter must be positive"
}

func GetReleases(action string, gitHubToken string, days *uint) (map[string]interface{}, error) {
	if days == nil {
		return nil, &daysParameterError{}
	}

	if gitHubToken == "" {
		return nil, &githubTokenIsEmptyError{}
	}

	if action == "" {
		return nil, &actionIsEmptyError{}
	}

	now := time.Now()
	interval := time.Duration(int64(*days) * dayInNanos)
	limit := now.Add(-interval)

	url := apiBaseURL + action + "/releases"
	temp, err := GetGithubBody(gitHubToken, url)

	if err != nil {
		return nil, fmt.Errorf("failed to request list of releases %w", err)
	}

	bodies, ok := temp.([]interface{})

	if !ok {
		return nil, fmt.Errorf("api query did not return list: %s", bodies)
	}

	for _, body := range bodies {
		release, ok := body.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid release format in response")
		}

		temp, ok := release["published_at"].(string)

		if !ok {
			return nil, &castToStringError{"published_at"}
		}

		released, err := time.Parse(time.RFC3339, temp)

		if err != nil {
			return nil, &timeParsingError{err: err}
		}

		if released.Before(limit) {
			return release, nil
		}
	}

	return nil, nil
}
