package core

import (
	"fmt"
	"net/http"

	"github.com/rs/zerolog/log"
)

type Registry struct {
	Registry bool
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

	log.Info().Msgf("Received %s for %s", resp.Status, url)

	return false, nil
}
