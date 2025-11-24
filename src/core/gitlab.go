package core

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/sergi/go-diff/diffmatchpatch"
	"gopkg.in/yaml.v3"
)

const gitlab = ".gitlab-ci.yml"

type gitlabProjectError struct {
	directory string
}

func (e *gitlabProjectError) Error() string {
	return fmt.Sprintf("gitlab project not found in directory: %s", e.directory)
}

type gitlabProjectEmptyError struct {
	file string
}

func (e *gitlabProjectEmptyError) Error() string {
	return fmt.Sprintf("gitlab project empty: %s", e.file)
}

// ImageReference represents a container image reference
type ImageReference struct {
	Registry   string
	Repository string
	Tag        string
	Digest     string
	Original   string
}

func (myFlags *Flags) UpdateGitlab() error {
	if myFlags.Directory == "" {
		return &directoryReadError{directory: myFlags.Directory}
	}

	projectFile := path.Join(myFlags.Directory, gitlab)

	project, err := os.ReadFile(projectFile)
	if err != nil {
		return &gitlabProjectError{directory: myFlags.Directory}
	}

	if len(project) == 0 {
		return &gitlabProjectEmptyError{file: projectFile}
	}

	fileInfo, err := os.Stat(projectFile)
	if err != nil {
		return &gitlabProjectError{directory: myFlags.Directory}
	}

	if fileInfo.Size() == 0 {
		return &gitlabProjectEmptyError{file: projectFile}
	}

	// Parse YAML to find all images
	images, err := extractImages(string(project))
	if err != nil {
		log.Warn().Err(err).Msg("Failed to extract images from YAML")
		return err
	}

	if len(images) == 0 {
		log.Info().Msg("No container images found in GitLab CI configuration")
		return nil
	}

	// Process each image
	replacement := string(project)
	for _, imageStr := range images {
		imageStr = strings.TrimSpace(imageStr)
		if imageStr == "" {
			continue
		}

		// Parse the image reference
		imgRef := parseImageReference(imageStr)
		log.Info().Str("image", imageStr).Msg("Processing image")

		// Get the digest for the image
		digest, err := myFlags.getImageDigest(imgRef)
		if err != nil {
			log.Warn().Err(err).Str("image", imageStr).Msg("Failed to get digest, skipping")
			continue
		}

		// Create new image reference with digest
		newImageRef := formatImageWithDigest(imgRef, digest)
		log.Info().
			Str("old", imageStr).
			Str("new", newImageRef).
			Msg("Image update")

		// Replace in the content
		replacement = strings.ReplaceAll(replacement, imageStr, newImageRef)
	}

	// Show diff
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(string(project), replacement, false)
	fmt.Println(dmp.DiffPrettyText(diffs))

	// Write file if not dry-run
	if !myFlags.DryRun && string(project) != replacement {
		err = os.WriteFile(projectFile, []byte(replacement), 0644)
		if err != nil {
			return fmt.Errorf("failed to write GitLab CI file: %w", err)
		}
		log.Info().Msg("GitLab CI file updated successfully")
	}

	return nil
}

// extractImages finds all container image references in GitLab CI YAML
func extractImages(content string) ([]string, error) {
	var images []string
	var data interface{}

	err := yaml.Unmarshal([]byte(content), &data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Recursively search for image fields
	findImages(data, &images)

	return images, nil
}

// findImages recursively searches for image fields in YAML structure
func findImages(data interface{}, images *[]string) {
	switch v := data.(type) {
	case map[string]interface{}:
		for key, value := range v {
			if key == "image" {
				// Handle both string and object formats
				switch img := value.(type) {
				case string:
					// Simple format: image: "nginx:latest"
					if img != "" && !strings.HasPrefix(img, "$") {
						*images = append(*images, img)
					}
				case map[string]interface{}:
					// Object format: image: { name: "nginx:latest" }
					if name, ok := img["name"].(string); ok && name != "" && !strings.HasPrefix(name, "$") {
						*images = append(*images, name)
					}
				}
			} else {
				// Recurse into nested structures
				findImages(value, images)
			}
		}
	case []interface{}:
		for _, item := range v {
			findImages(item, images)
		}
	}
}

// parseImageReference parses a container image reference into components
func parseImageReference(image string) ImageReference {
	ref := ImageReference{
		Original: image,
	}

	// Check if already has digest
	if strings.Contains(image, "@sha256:") {
		parts := strings.Split(image, "@")
		ref.Digest = parts[1]
		image = parts[0]
	}

	// Split registry/repository and tag
	var repoTag string
	if strings.Contains(image, "/") {
		// Has registry
		firstSlash := strings.Index(image, "/")
		potentialRegistry := image[:firstSlash]

		// Check if it's actually a registry (contains . or : or is localhost)
		if strings.Contains(potentialRegistry, ".") ||
			strings.Contains(potentialRegistry, ":") ||
			potentialRegistry == "localhost" {
			ref.Registry = potentialRegistry
			repoTag = image[firstSlash+1:]
		} else {
			// No registry, default to Docker Hub
			ref.Registry = "docker.io"
			repoTag = image
		}
	} else {
		// No registry, default to Docker Hub
		ref.Registry = "docker.io"
		repoTag = image
	}

	// Split repository and tag
	if strings.Contains(repoTag, ":") {
		parts := strings.Split(repoTag, ":")
		ref.Repository = parts[0]
		ref.Tag = parts[1]
	} else {
		ref.Repository = repoTag
		ref.Tag = "latest"
	}

	// Docker Hub short names need library/ prefix
	if ref.Registry == "docker.io" && !strings.Contains(ref.Repository, "/") {
		ref.Repository = "library/" + ref.Repository
	}

	return ref
}

// getImageDigest retrieves the SHA256 digest for a container image
func (myFlags *Flags) getImageDigest(ref ImageReference) (string, error) {
	// For Docker Hub and standard registries
	if ref.Registry == "docker.io" || ref.Registry == "registry.hub.docker.com" {
		return getDockerHubDigest(ref.Repository, ref.Tag)
	}

	// For GitHub Container Registry
	if ref.Registry == "ghcr.io" {
		return getGHCRDigest(ref.Repository, ref.Tag, myFlags.GitHubToken)
	}

	// For other registries, try generic OCI registry API
	return getOCIRegistryDigest(ref.Registry, ref.Repository, ref.Tag)
}

// getDockerHubDigest gets the digest from Docker Hub
func getDockerHubDigest(repository, tag string) (string, error) {
	// First, get auth token
	tokenURL := fmt.Sprintf("https://auth.docker.io/token?service=registry.docker.io&scope=repository:%s:pull", repository)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(tokenURL)
	if err != nil {
		return "", fmt.Errorf("failed to get auth token: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Warn().Err(err).Msg("failed to close response body")
		}
	}(resp.Body)

	var tokenResp struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse token response: %w", err)
	}

	// Now get the manifest
	manifestURL := fmt.Sprintf("https://registry-1.docker.io/v2/%s/manifests/%s", repository, tag)
	req, err := http.NewRequest("GET", manifestURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+tokenResp.Token)
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")

	resp, err = client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get manifest: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Warn().Err(err).Msg("failed to close response body")
		}
	}(resp.Body)

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("manifest request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Get digest from Docker-Content-Digest header
	digest := resp.Header.Get("Docker-Content-Digest")
	if digest == "" {
		return "", fmt.Errorf("no digest found in response")
	}

	return digest, nil
}

// getGHCRDigest gets the digest from GitHub Container Registry
func getGHCRDigest(repository, tag, token string) (string, error) {
	manifestURL := fmt.Sprintf("https://ghcr.io/v2/%s/manifests/%s", repository, tag)

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", manifestURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get manifest: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Warn().Err(err).Msg("failed to close response body")
		}
	}(resp.Body)

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("manifest request failed with status %d: %s", resp.StatusCode, string(body))
	}

	digest := resp.Header.Get("Docker-Content-Digest")
	if digest == "" {
		return "", fmt.Errorf("no digest found in response")
	}

	return digest, nil
}

// getOCIRegistryDigest gets digest from generic OCI registry
func getOCIRegistryDigest(registry, repository, tag string) (string, error) {
	manifestURL := fmt.Sprintf("https://%s/v2/%s/manifests/%s", registry, repository, tag)

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", manifestURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get manifest: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Warn().Err(err).Msg("failed to close response body")
		}
	}(resp.Body)

	if resp.StatusCode == 401 {
		return "", fmt.Errorf("authentication required for registry %s", registry)
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("manifest request failed with status %d: %s", resp.StatusCode, string(body))
	}

	digest := resp.Header.Get("Docker-Content-Digest")
	if digest == "" {
		return "", fmt.Errorf("no digest found in response")
	}

	return digest, nil
}

// formatImageWithDigest creates the new image reference with digest
func formatImageWithDigest(ref ImageReference, digest string) string {
	var result strings.Builder

	// Don't include docker.io prefix for Docker Hub images in output (keep them simple)
	if ref.Registry == "docker.io" {
		// Remove library/ prefix for official images
		repo := ref.Repository

		repo = strings.TrimPrefix(repo, "library/")

		result.WriteString(repo)
	} else {
		result.WriteString(ref.Registry)
		result.WriteString("/")
		result.WriteString(ref.Repository)
	}

	result.WriteString("@")
	result.WriteString(digest)
	result.WriteString(" # ")
	result.WriteString(ref.Tag)

	return result.String()
}

// GetGitlabFiles finds GitLab CI files in the entries
func (myFlags *Flags) GetGitlabFiles() []string {
	var gitlabFiles []string

	// Check if there's a .gitlab-ci.yml in the directory
	gitlabFile := path.Join(myFlags.Directory, gitlab)
	if _, err := os.Stat(gitlabFile); err == nil {
		gitlabFiles = append(gitlabFiles, gitlabFile)
		return gitlabFiles
	}

	// Also check in entries
	pattern := regexp.MustCompile(`\.gitlab-ci\.ya?ml$`)
	for _, entry := range myFlags.Entries {
		if pattern.MatchString(entry) {
			gitlabFiles = append(gitlabFiles, entry)
		}
	}

	return gitlabFiles
}
