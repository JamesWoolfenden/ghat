package core

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Registry struct {
	Registry      bool
	LatestVersion string
}

func (myRegistry *Registry) IsRegistryModule(module string) (bool, error) {
	url := "https://registry.terraform.io/v1/modules/" + module + "/versions"
	result, err := IsOK(url)

	myRegistry.Registry = result

	return result, err
}

func IsOK(url string) (bool, error) {
	resp, err := http.Get(url)
	if err != nil {
		return false, fmt.Errorf("failed to get url %w", err)
	}

	if resp.StatusCode == 200 {
		return true, nil
	}

	return false, fmt.Errorf("received %s for %s", resp.Status, url)
}

func (myRegistry *Registry) GetLatest(module string) (*string, error) {
	found, err := myRegistry.IsRegistryModule(module)

	if err != nil {
		return nil, err
	}

	if found {
		url := "https://registry.terraform.io/v1/modules/" + module
		resp, _ := http.Get(url)

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
