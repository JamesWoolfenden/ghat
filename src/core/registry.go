package core

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type Registry struct {
	Registry      bool
	LatestVersion string
}

const (
	registryBaseURL = "https://registry.terraform.io/v1/modules/"
	successStatus   = 200
)

func (myRegistry *Registry) IsRegistryModule(module string) (bool, error) {
	module = url.PathEscape(module)
	urlBuilt := registryBaseURL + module + "/versions"
	result, err := IsOK(urlBuilt)

	myRegistry.Registry = result

	return result, err
}

func IsOK(url string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)

	if err != nil {
		return false, fmt.Errorf("failed to make request with context: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)

	if err != nil {
		return false, fmt.Errorf("failed to get url %w", err)
	}

	if resp.StatusCode == successStatus {
		return true, nil
	}

	return false, fmt.Errorf("received %s for %s", resp.Status, url)
}

func (myRegistry *Registry) GetLatest(module string) (*string, error) {
	// Add module name validation
	if module == "" {
		return nil, fmt.Errorf("module name cannot be empty")
	}

	found, err := myRegistry.IsRegistryModule(module)

	if err != nil {
		return nil, err
	}

	if found {
		urlBuilt := registryBaseURL + module
		resp, err := http.Get(urlBuilt)
		if err != nil {
			return nil, fmt.Errorf("failed to make HTTP request: %w", err)
		}

		if resp == nil {
			return nil, fmt.Errorf("api failed to respond")
		}

		if resp.StatusCode != successStatus {
			return nil, fmt.Errorf("api failed with %d", resp.StatusCode)
		}

		defer func(Body io.ReadCloser) {
			_ = Body.Close()
		}(resp.Body)

		body, err := io.ReadAll(resp.Body)

		if err != nil {
			return nil, fmt.Errorf("failed to read body %w", err)
		}

		var msg map[string]interface{}

		err = json.Unmarshal(body, &msg)

		if err != nil {
			return nil, fmt.Errorf("failed to read body %w", err)
		}

		var ok bool

		myRegistry.LatestVersion, ok = msg["version"].(string)

		if !ok {
			return nil, fmt.Errorf("failed to find version in payload")
		}
	}

	return &myRegistry.LatestVersion, nil
}
