package core

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/sergi/go-diff/diffmatchpatch"
	"gopkg.in/yaml.v3"
)

var k8sKinds = map[string]bool{
	"Pod":         true,
	"Deployment":  true,
	"StatefulSet": true,
	"DaemonSet":   true,
	"Job":         true,
	"CronJob":     true,
	"ReplicaSet":  true,
}

// UpdateKubes pins all Kubernetes manifests and Docker Compose files found in the scanned entries.
func (myFlags *Flags) UpdateKubes() error {
	for _, f := range myFlags.GetKubeFiles() {
		if err := myFlags.UpdateKube(f); err != nil {
			if myFlags.ContinueOnError {
				log.Warn().Err(err).Str("file", f).Msg("skipping file")
				continue
			}
			return err
		}
	}
	for _, f := range myFlags.GetComposeFiles() {
		if err := myFlags.UpdateCompose(f); err != nil {
			if myFlags.ContinueOnError {
				log.Warn().Err(err).Str("file", f).Msg("skipping file")
				continue
			}
			return err
		}
	}
	return nil
}

// UpdateKube pins container image references in a single Kubernetes manifest file.
func (myFlags *Flags) UpdateKube(file string) error {
	content, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", file, err)
	}

	images, err := extractKubeImages(string(content))
	if err != nil {
		return fmt.Errorf("failed to extract images from %s: %w", file, err)
	}

	// Snapshot existing digest→tag mappings before YAML strips comments.
	pinnedImages := parsePinnedImages(string(content))

	replacement := string(content)
	for _, imageStr := range images {
		imgRef := parseImageReference(imageStr)
		digest, err := myFlags.getImageDigest(imgRef)
		if err != nil {
			log.Warn().Err(err).Str("image", imageStr).Msg("failed to get digest, skipping")
			continue
		}

		// Detect tag mutation: same tag, different digest.
		if cur, ok := pinnedImages[imgRef.Tag]; ok && isTagMutation(cur, imgRef.Tag, digest, imgRef.Tag) {
			log.Warn().Msgf("SUSPICIOUS: %s — digest changed from %s to %s with the same tag. "+
				"The image tag may have been repointed to a different layer. Verify before accepting.", imageStr, cur, digest)
		}

		replacement = strings.ReplaceAll(replacement, imageStr, formatImageWithDigest(imgRef, digest))
	}

	dmp := diffmatchpatch.New()
	fmt.Println(dmp.DiffPrettyText(dmp.DiffMain(string(content), replacement, false)))

	if !myFlags.DryRun && string(content) != replacement {
		if err := os.WriteFile(file, []byte(replacement), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", file, err)
		}
	}

	return nil
}

// GetKubeFiles returns all Kubernetes manifest files from the scanned entries.
func (myFlags *Flags) GetKubeFiles() []string {
	var files []string
	for _, entry := range myFlags.Entries {
		if isKubeManifest(entry) {
			files = append(files, entry)
		}
	}
	return files
}

// isKubeManifest returns true if the file is a YAML file containing at least one
// recognised Kubernetes resource kind.
func isKubeManifest(file string) bool {
	if !strings.HasSuffix(file, ".yml") && !strings.HasSuffix(file, ".yaml") {
		return false
	}
	content, err := os.ReadFile(file)
	if err != nil {
		return false
	}
	return hasKubeResource(string(content))
}

// hasKubeResource reports whether the content contains at least one K8s resource.
func hasKubeResource(content string) bool {
	dec := yaml.NewDecoder(strings.NewReader(content))
	for {
		var doc map[string]interface{}
		if err := dec.Decode(&doc); err != nil {
			break
		}
		kind, _ := doc["kind"].(string)
		if _, hasAPI := doc["apiVersion"]; hasAPI && k8sKinds[kind] {
			return true
		}
	}
	return false
}

// extractKubeImages returns all container image strings found in a Kubernetes
// manifest. The file may contain multiple documents separated by ---.
func extractKubeImages(content string) ([]string, error) {
	var images []string
	dec := yaml.NewDecoder(strings.NewReader(content))
	for {
		var doc interface{}
		if err := dec.Decode(&doc); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("failed to parse YAML: %w", err)
		}
		findKubeImages(doc, &images)
	}
	sort.Strings(images)
	return images, nil
}

// findKubeImages recursively walks the YAML tree and collects image strings from
// containers and initContainers specs.
func findKubeImages(data interface{}, images *[]string) {
	switch v := data.(type) {
	case map[string]interface{}:
		for key, value := range v {
			if key == "containers" || key == "initContainers" {
				if list, ok := value.([]interface{}); ok {
					for _, item := range list {
						if c, ok := item.(map[string]interface{}); ok {
							if img, ok := c["image"].(string); ok && img != "" && !isKubeVarRef(img) {
								*images = append(*images, img)
							}
						}
					}
				}
			} else {
				findKubeImages(value, images)
			}
		}
	case []interface{}:
		for _, item := range v {
			findKubeImages(item, images)
		}
	}
}

// isKubeVarRef returns true for Kubernetes variable substitution references
// like $(VAR_NAME) that should not be pinned.
func isKubeVarRef(s string) bool {
	return strings.HasPrefix(s, "$")
}

// composeFileNames is the set of canonical Docker Compose file names.
var composeFileNames = map[string]bool{
	"docker-compose.yml":  true,
	"docker-compose.yaml": true,
	"compose.yml":         true,
	"compose.yaml":        true,
}

// GetComposeFiles returns all Docker Compose files from the scanned entries.
func (myFlags *Flags) GetComposeFiles() []string {
	var files []string
	for _, entry := range myFlags.Entries {
		if isComposeFile(entry) {
			files = append(files, entry)
		}
	}
	return files
}

// isComposeFile returns true for files with a canonical Docker Compose filename.
func isComposeFile(file string) bool {
	return composeFileNames[strings.ToLower(filepath.Base(file))]
}

// UpdateCompose pins image references in a Docker Compose file to SHA digests.
func (myFlags *Flags) UpdateCompose(file string) error {
	content, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", file, err)
	}

	// extractImages (from gitlab.go) does a generic recursive walk for image: keys.
	images, err := extractImages(string(content))
	if err != nil {
		return fmt.Errorf("failed to extract images from %s: %w", file, err)
	}

	pinnedImgs := parsePinnedImages(string(content))
	replacement := string(content)
	for _, imageStr := range images {
		imgRef := parseImageReference(imageStr)
		digest, err := myFlags.getImageDigest(imgRef)
		if err != nil {
			log.Warn().Err(err).Str("image", imageStr).Msg("failed to get digest, skipping")
			continue
		}
		if cur, ok := pinnedImgs[imgRef.Tag]; ok && isTagMutation(cur, imgRef.Tag, digest, imgRef.Tag) {
			log.Warn().Msgf("SUSPICIOUS: %s — digest changed from %s to %s with the same tag. "+
				"Verify before accepting.", imageStr, cur, digest)
		}
		replacement = strings.ReplaceAll(replacement, imageStr, formatImageWithDigest(imgRef, digest))
	}

	dmp := diffmatchpatch.New()
	fmt.Println(dmp.DiffPrettyText(dmp.DiffMain(string(content), replacement, false)))

	if !myFlags.DryRun && string(content) != replacement {
		if err := os.WriteFile(file, []byte(replacement), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", file, err)
		}
	}
	return nil
}
