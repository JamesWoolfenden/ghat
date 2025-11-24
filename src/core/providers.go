package core

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/rs/zerolog/log"
	"github.com/sergi/go-diff/diffmatchpatch"
	"golang.org/x/mod/semver"
)

// ProviderVersion represents a Terraform provider version from the registry
type ProviderVersion struct {
	Version   string   `json:"version"`
	Protocols []string `json:"protocols"`
	Platforms []struct {
		OS   string `json:"os"`
		Arch string `json:"arch"`
	} `json:"platforms"`
}

// ProviderVersionsResponse represents the API response from Terraform Registry
type ProviderVersionsResponse struct {
	Versions []ProviderVersion `json:"versions"`
}

// ProviderInfo holds information about a provider
type ProviderInfo struct {
	Name           string
	Source         string
	Namespace      string
	Type           string
	CurrentVersion string
	LatestVersion  string
}

// UpdateProviders updates all Terraform providers in the directory
func (myFlags *Flags) UpdateProviders() error {
	terraform, err := myFlags.GetTF()
	if err != nil {
		return err
	}

	for _, file := range terraform {
		err = myFlags.UpdateProvider(file)
		if err != nil {
			if myFlags.ContinueOnError {
				log.Warn().Err(err).Str("file", file).Msg("Failed to update providers, continuing")
				continue
			}
			return err
		}
	}

	return nil
}

// UpdateProvider updates providers in a single Terraform file
func (myFlags *Flags) UpdateProvider(file string) error {
	src, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", file, err)
	}

	inFile, diags := hclwrite.ParseConfig(src, "", hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return fmt.Errorf("failed to parse HCL: %s", diags.Error())
	}

	root := inFile.Body()
	modified := false

	// Find terraform blocks
	for _, block := range root.Blocks() {
		if block.Type() == "terraform" {
			// Find required_providers blocks within terraform block
			for _, innerBlock := range block.Body().Blocks() {
				if innerBlock.Type() == "required_providers" {
					// Process each provider
					attrs := innerBlock.Body().Attributes()
					for name, attr := range attrs {
						provider, err := parseProviderBlock(name, attr)
						if err != nil {
							log.Warn().Err(err).Str("provider", name).Msg("Failed to parse provider")
							continue
						}

						// Get latest version
						latestVersion, err := getLatestProviderVersion(provider.Namespace, provider.Type)
						if err != nil {
							log.Warn().Err(err).
								Str("provider", provider.Source).
								Msg("Failed to get latest version")
							continue
						}

						provider.LatestVersion = latestVersion

						// Check if update is needed
						if shouldUpdateProvider(provider) {
							log.Info().
								Str("provider", provider.Source).
								Str("current", provider.CurrentVersion).
								Str("latest", provider.LatestVersion).
								Msg("Updating provider")

							// Update the version in the HCL
							err = updateProviderVersion(innerBlock.Body(), name, provider)
							if err != nil {
								log.Warn().Err(err).Str("provider", name).Msg("Failed to update version")
								continue
							}
							modified = true
						} else {
							log.Info().
								Str("provider", provider.Source).
								Str("version", provider.CurrentVersion).
								Msg("Provider already at latest version")
						}
					}
				}
			}
		}
	}

	if !modified {
		log.Info().Str("file", file).Msg("No provider updates needed")
		return nil
	}

	// Generate diff
	newContent := string(inFile.Bytes())
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(string(src), newContent, false)
	fmt.Println(dmp.DiffPrettyText(diffs))

	// Write file if not dry-run
	if !myFlags.DryRun {
		err = os.WriteFile(file, []byte(newContent), 0644)
		if err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}
		log.Info().Str("file", file).Msg("Provider versions updated")
	}

	return nil
}

// parseProviderBlock extracts provider information from an HCL attribute
func parseProviderBlock(name string, attr *hclwrite.Attribute) (*ProviderInfo, error) {
	provider := &ProviderInfo{
		Name: name,
	}

	// Parse the provider configuration object
	tokens := attr.Expr().BuildTokens(nil)
	content := strings.TrimSpace(string(hclwrite.Format(tokens.Bytes())))

	// Extract source
	sourcePattern := `source\s*=\s*"([^"]+)"`
	if matches := extractPattern(content, sourcePattern); len(matches) > 1 {
		provider.Source = matches[1]
		parts := strings.Split(provider.Source, "/")
		if len(parts) == 2 {
			provider.Namespace = parts[0]
			provider.Type = parts[1]
		} else if len(parts) == 3 {
			provider.Namespace = parts[1]
			provider.Type = parts[2]
		}
	}

	// Extract version
	versionPattern := `version\s*=\s*"([^"]+)"`
	if matches := extractPattern(content, versionPattern); len(matches) > 1 {
		provider.CurrentVersion = matches[1]
	}

	if provider.Source == "" {
		return nil, fmt.Errorf("no source found for provider %s", name)
	}

	return provider, nil
}

// extractPattern is a simple regex helper
func extractPattern(content, pattern string) []string {
	// Simple pattern matching without importing regexp
	// Look for source = "value"
	if strings.Contains(pattern, "source") {
		start := strings.Index(content, `source`)
		if start == -1 {
			return nil
		}
		rest := content[start:]
		quoteStart := strings.Index(rest, `"`)
		if quoteStart == -1 {
			return nil
		}
		quoteEnd := strings.Index(rest[quoteStart+1:], `"`)
		if quoteEnd == -1 {
			return nil
		}
		value := rest[quoteStart+1 : quoteStart+1+quoteEnd]
		return []string{"", value}
	}

	// Look for version = "value"
	if strings.Contains(pattern, "version") {
		start := strings.Index(content, `version`)
		if start == -1 {
			return nil
		}
		rest := content[start:]
		quoteStart := strings.Index(rest, `"`)
		if quoteStart == -1 {
			return nil
		}
		quoteEnd := strings.Index(rest[quoteStart+1:], `"`)
		if quoteEnd == -1 {
			return nil
		}
		value := rest[quoteStart+1 : quoteStart+1+quoteEnd]
		return []string{"", value}
	}

	return nil
}

// getLatestProviderVersion queries the Terraform Registry API
func getLatestProviderVersion(namespace, providerType string) (string, error) {
	url := fmt.Sprintf("https://registry.terraform.io/v1/providers/%s/%s/versions", namespace, providerType)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to query registry: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Error().Err(err).Msg("Failed to close response body")
		}
	}(resp.Body)

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("registry returned status %d", resp.StatusCode)
	}

	var versionsResp ProviderVersionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&versionsResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(versionsResp.Versions) == 0 {
		return "", fmt.Errorf("no versions found")
	}

	// Find the latest stable version (highest semver, excluding pre-releases)
	var latest string
	for _, v := range versionsResp.Versions {
		version := v.Version
		if !strings.HasPrefix(version, "v") {
			version = "v" + version
		}

		// Skip pre-releases by default
		if strings.Contains(version, "-") {
			continue
		}

		if latest == "" || semver.Compare(version, latest) > 0 {
			latest = version
		}
	}

	// Remove v prefix for Terraform
	return strings.TrimPrefix(latest, "v"), nil
}

// shouldUpdateProvider determines if a provider should be updated
func shouldUpdateProvider(provider *ProviderInfo) bool {
	if provider.CurrentVersion == "" {
		return true
	}

	current := normalizeVersion(provider.CurrentVersion)
	latest := normalizeVersion(provider.LatestVersion)

	// If current has constraints like ~>, >=, etc., update it
	if hasVersionConstraint(provider.CurrentVersion) {
		return true
	}

	// Compare versions
	if semver.IsValid(current) && semver.IsValid(latest) {
		return semver.Compare(current, latest) < 0
	}

	// If we can't compare, suggest update if versions differ
	return provider.CurrentVersion != provider.LatestVersion
}

// normalizeVersion adds 'v' prefix if needed for semver
func normalizeVersion(version string) string {
	// Remove constraint operators
	version = strings.TrimPrefix(version, "~>")
	version = strings.TrimPrefix(version, ">=")
	version = strings.TrimPrefix(version, "<=")
	version = strings.TrimPrefix(version, ">")
	version = strings.TrimPrefix(version, "<")
	version = strings.TrimPrefix(version, "=")
	version = strings.TrimSpace(version)

	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}
	return version
}

// hasVersionConstraint checks if version string has constraint operators
func hasVersionConstraint(version string) bool {
	constraints := []string{"~>", ">=", "<=", ">", "<", "!=", "="}
	for _, c := range constraints {
		if strings.Contains(version, c) {
			return true
		}
	}
	return false
}

// updateProviderVersion updates the version attribute in the provider block
func updateProviderVersion(body *hclwrite.Body, providerName string, provider *ProviderInfo) error {
	// Build new provider configuration as HCL source
	newConfig := fmt.Sprintf(`%s = {
    source  = %q
    version = %q
  }`, providerName, provider.Source, provider.LatestVersion)

	// Parse it as a complete attribute
	parsed, diags := hclwrite.ParseConfig([]byte(newConfig), "", hcl.Pos{})
	if diags.HasErrors() {
		return fmt.Errorf("failed to parse new provider block: %s", diags.Error())
	}

	// Get the attribute from the parsed content
	newAttr := parsed.Body().GetAttribute(providerName)
	if newAttr == nil {
		return fmt.Errorf("failed to extract new provider attribute")
	}

	// Remove old attribute and set new one
	body.RemoveAttribute(providerName)
	body.SetAttributeRaw(providerName, newAttr.Expr().BuildTokens(nil))

	return nil
}

// GetProviderFiles finds Terraform files that likely contain provider definitions
func (myFlags *Flags) GetProviderFiles() ([]string, error) {
	allTerraformFiles, err := myFlags.GetTF()
	if err != nil {
		return nil, err
	}

	var providerFiles []string
	for _, file := range allTerraformFiles {
		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		// Simple check if file contains required_providers
		if strings.Contains(string(content), "required_providers") {
			providerFiles = append(providerFiles, file)
		}
	}

	return providerFiles, nil
}

// ListProvidersInDirectory lists all providers found in Terraform files
func (myFlags *Flags) ListProvidersInDirectory() ([]ProviderInfo, error) {
	var allProviders []ProviderInfo

	files, err := myFlags.GetProviderFiles()
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		providers, err := extractProvidersFromFile(file)
		if err != nil {
			log.Warn().Err(err).Str("file", file).Msg("Failed to extract providers")
			continue
		}
		allProviders = append(allProviders, providers...)
	}

	return allProviders, nil
}

// extractProvidersFromFile extracts all provider definitions from a file
func extractProvidersFromFile(filename string) ([]ProviderInfo, error) {
	var providers []ProviderInfo

	src, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	inFile, diags := hclwrite.ParseConfig(src, filename, hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, fmt.Errorf("failed to parse HCL: %s", diags.Error())
	}

	root := inFile.Body()

	for _, block := range root.Blocks() {
		if block.Type() == "terraform" {
			for _, innerBlock := range block.Body().Blocks() {
				if innerBlock.Type() == "required_providers" {
					attrs := innerBlock.Body().Attributes()
					for name, attr := range attrs {
						provider, err := parseProviderBlock(name, attr)
						if err != nil {
							continue
						}
						provider.Name = name
						providers = append(providers, *provider)
					}
				}
			}
		}
	}

	return providers, nil
}

// GetTerraformFiles returns all .tf files in the entries
func GetTerraformFiles(directory string) ([]string, error) {
	var tfFiles []string

	err := filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && filepath.Ext(path) == ".tf" {
			tfFiles = append(tfFiles, path)
		}

		return nil
	})

	return tfFiles, err
}
