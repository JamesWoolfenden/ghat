package core

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/sergi/go-diff/diffmatchpatch"
)

// fromRe matches a Dockerfile FROM line, capturing optional platform flag,
// the image reference, and optional AS alias.
var fromRe = regexp.MustCompile(`(?i)^(FROM\s+(?:--platform=\S+\s+)?)(\S+?)(\s+AS\s+\S+)?\s*$`)

// pinnedFromRe matches an already-pinned FROM line: image:tag@sha256:digest
var pinnedFromRe = regexp.MustCompile(`\S+:(\S+?)@(sha256:[0-9a-f]+)`)

// UpdateDockerfiles pins FROM image references in all Dockerfiles found in the entries.
func (myFlags *Flags) UpdateDockerfiles() error {
	for _, f := range myFlags.GetDockerfiles() {
		if err := myFlags.UpdateDockerfile(f); err != nil {
			if myFlags.ContinueOnError {
				log.Warn().Err(err).Str("file", f).Msg("skipping file")
				continue
			}
			return err
		}
	}
	return nil
}

// UpdateDockerfile pins FROM image references in a single Dockerfile to SHA digests.
// Output format: FROM image:tag@sha256:digest  (valid Docker syntax, tag preserved inline).
func (myFlags *Flags) UpdateDockerfile(file string) error {
	content, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", file, err)
	}

	pinned := parsePinnedFromLines(string(content))

	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		m := fromRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		prefix, imageStr, alias := m[1], m[2], m[3]

		if imageStr == "scratch" || strings.HasPrefix(imageStr, "$") {
			continue
		}

		// Strip existing digest so we resolve a fresh one for the tag.
		bareImage := imageStr
		if idx := strings.Index(imageStr, "@sha256:"); idx != -1 {
			bareImage = imageStr[:idx]
		}

		imgRef := parseImageReference(bareImage)
		digest, err := myFlags.getImageDigest(imgRef)
		if err != nil {
			log.Warn().Err(err).Str("image", bareImage).Msg("failed to get digest, skipping")
			continue
		}

		if cur, ok := pinned[imgRef.Tag]; ok && isTagMutation(cur, imgRef.Tag, digest, imgRef.Tag) {
			log.Warn().Msgf("SUSPICIOUS: %s — digest changed from %s to %s with the same tag. "+
				"The image tag may have been repointed. Verify before accepting.", bareImage, cur, digest)
		}

		lines[i] = prefix + formatDockerImage(imgRef, digest) + alias
	}

	replacement := strings.Join(lines, "\n")

	dmp := diffmatchpatch.New()
	fmt.Println(dmp.DiffPrettyText(dmp.DiffMain(string(content), replacement, false)))

	if !myFlags.DryRun && string(content) != replacement {
		if err := os.WriteFile(file, []byte(replacement), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", file, err)
		}
	}
	return nil
}

// GetDockerfiles returns all Dockerfile paths from the scanned entries.
func (myFlags *Flags) GetDockerfiles() []string {
	var files []string
	for _, entry := range myFlags.Entries {
		if isDockerfile(entry) {
			files = append(files, entry)
		}
	}
	return files
}

// isDockerfile returns true for files named Dockerfile, Dockerfile.*, or *.dockerfile.
func isDockerfile(file string) bool {
	base := filepath.Base(file)
	lower := strings.ToLower(base)
	return base == "Dockerfile" ||
		strings.HasPrefix(base, "Dockerfile.") ||
		strings.HasSuffix(lower, ".dockerfile")
}

// formatDockerImage produces "image:tag@sha256:digest" — valid Docker pull syntax
// that keeps the original tag visible without needing a comment.
func formatDockerImage(ref ImageReference, digest string) string {
	var b strings.Builder
	if ref.Registry == "docker.io" {
		repo := strings.TrimPrefix(ref.Repository, "library/")
		b.WriteString(repo)
	} else {
		b.WriteString(ref.Registry)
		b.WriteString("/")
		b.WriteString(ref.Repository)
	}
	if ref.Tag != "latest" {
		b.WriteString(":")
		b.WriteString(ref.Tag)
	}
	b.WriteString("@")
	b.WriteString(digest)
	return b.String()
}

// parsePinnedFromLines scans Dockerfile content for already-pinned FROM lines
// in the form image:tag@sha256:digest, returning a map of tag → digest.
func parsePinnedFromLines(content string) map[string]string {
	pinned := make(map[string]string)
	for _, m := range pinnedFromRe.FindAllStringSubmatch(content, -1) {
		tag, digest := m[1], m[2]
		pinned[tag] = digest
	}
	return pinned
}
