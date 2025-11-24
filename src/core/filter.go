package core

import (
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	dayInNanos int64 = 24 * 60 * 60 * 1000 * 1000 * 1000
	apiBaseURL       = "https://api.github.com/repos/"
)

const (
	maxRetries     = 3
	initialBackoff = 2 * time.Second
	maxBackoff     = 30 * time.Second
)

// RateLimitError represents a rate limit error
type RateLimitError struct {
	ResetTime time.Time
	Remaining int
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limit exceeded, resets at %s", e.ResetTime.Format(time.RFC3339))
}

// GetReleases fetches releases from GitHub with rate limit handling
func GetReleases(action string, gitHubToken string, days *uint) (map[string]interface{}, error) {
	if days == nil {
		return nil, &daysParameterError{}
	}

	if gitHubToken == "" {
		log.Warn().Str("action", action).Msg("No GitHub token provided - may encounter rate limits")
	}

	if action == "" {
		return nil, &actionIsEmptyError{}
	}

	now := time.Now()
	interval := time.Duration(int64(*days) * dayInNanos)
	limit := now.Add(-interval)

	url := apiBaseURL + action + "/releases"

	// Retry logic with exponential backoff
	var temp interface{}
	var err error
	backoff := initialBackoff

	for attempt := 0; attempt <= maxRetries; attempt++ {
		temp, err = GetGithubBody(gitHubToken, url)

		if err == nil {
			break // Success!
		}

		// Check if it's a rate limit error
		if isRateLimitError(err) {
			if attempt == maxRetries {
				log.Error().
					Str("action", action).
					Int("attempts", attempt+1).
					Msg("Rate limit exceeded after all retries")
				return nil, fmt.Errorf("rate limit exceeded after %d attempts: %w", maxRetries+1, err)
			}

			// Calculate wait time
			waitTime := backoff
			if waitTime > maxBackoff {
				waitTime = maxBackoff
			}

			log.Warn().
				Str("action", action).
				Int("attempt", attempt+1).
				Dur("wait", waitTime).
				Msg("Rate limited, waiting before retry")

			time.Sleep(waitTime)
			backoff *= 2 // Exponential backoff

			continue
		}

		// For non-rate-limit errors, fail immediately
		return nil, fmt.Errorf("failed to request list of releases: %w", err)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to request list of releases after retries: %w", err)
	}

	bodies, ok := temp.([]interface{})
	if !ok {
		return nil, fmt.Errorf("api query did not return list: %v", temp)
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
			log.Debug().
				Str("action", action).
				Str("tag", fmt.Sprintf("%v", release["tag_name"])).
				Time("published", released).
				Msg("Found release within time window")
			return release, nil
		}
	}

	log.Debug().
		Str("action", action).
		Uint("days", *days).
		Msg("No releases found within time window")

	return nil, nil
}

// isRateLimitError checks if an error is related to rate limiting
func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Check for various rate limit indicators
	return strings.Contains(errStr, "rate limit") ||
		strings.Contains(errStr, "403") ||
		strings.Contains(errStr, "429") || // Too Many Requests
		strings.Contains(errStr, "API rate limit exceeded")
}
