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
	defaultTimeout  = 30 * time.Second
)

func (myRegistry *Registry) IsRegistryModule(module string) (bool, error) {
	module = url.PathEscape(module)
	urlBuilt := registryBaseURL + module + "/versions"
	result, err := IsOK(urlBuilt)

	myRegistry.Registry = result

	return result, err
}

type URLFormatError struct {
	err error
}

func (e URLFormatError) Error() string {
	return fmt.Sprintf("failed to format url: %v", e.err)
}

func IsOK(rawURL string) (bool, error) {

	if rawURL == "" {
		return false, &emptyURL{}
	}

	// Add URL format validation
	if _, err := url.Parse(rawURL); err != nil {
		return false, &URLFormatError{err: err}
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)

	if err != nil {
		return false, &requestFailedError{err: err}
	}

	resp, err := http.DefaultClient.Do(req)

	if err != nil {
		return false, &httpClientError{err: err}
	}

	// Add resp.Body.Close() to prevent resource leaks
	defer resp.Body.Close()

	if resp.StatusCode == successStatus {
		return true, nil
	}

	return false, fmt.Errorf("received %s for %s", resp.Status, rawURL)
}

type urlJoinError struct {
	err error
}

func (m *urlJoinError) Error() string {
	return fmt.Sprintf("failed to join url: %v", m.err)
}

func (myRegistry *Registry) GetLatest(module string) (*string, error) {
	// Add module name validation
	if module == "" {
		return nil, &moduleEmptyError{}
	}

	found, err := myRegistry.IsRegistryModule(module)

	if err != nil {
		return nil, &registryModuleError{module, err}
	}

	if found {
		// Add URL sanitization
		urlBuilt, err := url.JoinPath(registryBaseURL, url.PathEscape(module))

		if err != nil {
			return nil, &urlJoinError{err: err}
		}

		// Add timeout to prevent hanging requests
		client := &http.Client{
			Timeout: defaultTimeout,
		}

		resp, err := client.Get(urlBuilt)

		if err != nil {
			return nil, &httpGetError{err: err}
		}

		if resp == nil {
			return nil, &responseNilError{}
		}

		if resp.StatusCode != successStatus {
			return nil, fmt.Errorf("api failed with %d", resp.StatusCode)
		}

		defer func(Body io.ReadCloser) {
			_ = Body.Close()
		}(resp.Body)

		body, err := io.ReadAll(resp.Body)

		if err != nil {
			return nil, &responseReadError{err: err}
		}

		var msg map[string]interface{}

		err = json.Unmarshal(body, &msg)

		if err != nil {
			return nil, &unmarshalJSONError{err: err}
		}

		var ok bool

		myRegistry.LatestVersion, ok = msg["version"].(string)

		if !ok {
			return nil, &castToStringError{"version"}
		}
	}

	return &myRegistry.LatestVersion, nil
}
