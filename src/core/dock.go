package core

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
)

// fromRe matches a Dockerfile FROM line, capturing optional platform flag,
// the image reference, and optional AS alias.
var fromRe = regexp.MustCompile(`(?i)^(FROM\s+(?:--platform=\S+\s+)?)(\S+?)(\s+AS\s+\S+)?\s*$`)

// pinnedFromRe matches an already-pinned FROM line: image:tag@sha256:digest
var pinnedFromRe = regexp.MustCompile(`\S+:(\S+?)@(sha256:[0-9a-f]+)`)

// argDefaultRe matches ARG name=value declarations (value required).
var argDefaultRe = regexp.MustCompile(`(?i)^ARG\s+(\w+)=(\S+)`)

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

		if imageStr == "scratch" {
			continue
		}
		if ok, reason := parseSuppression(line); ok {
			log.Info().Str("image", imageStr).Str("reason", reason).Msg("skipping suppressed FROM line")
			continue
		}

		// Expand any ARG variable references using defaults declared above this FROM.
		resolvedStr := expandDockerVars(imageStr, parseArgDefaults(lines[:i]))

		if strings.Contains(resolvedStr, "$") {
			// Still has unexpanded references — classify by position.
			switch {
			case strings.HasPrefix(imageStr, "$"):
				log.Warn().Msgf("SUPPLY CHAIN RISK: FROM uses a dynamic image reference '%s' which cannot be pinned — resolve to a specific tag and digest", imageStr)
			case strings.Contains(imageStr, "@$"):
				log.Info().Str("image", imageStr).Msg("digest is externally pinned via variable, skipping")
			default:
				log.Info().Str("image", imageStr).Msg("skipping image with unexpanded variable tag, cannot pin")
			}
			continue
		}

		// Strip existing sha256 digest from the resolved string before re-resolving.
		bareResolved := resolvedStr
		if idx := strings.Index(resolvedStr, "@sha256:"); idx != -1 {
			bareResolved = resolvedStr[:idx]
		}

		// For write-back: strip any existing digest from the original (variable or literal)
		// so the fresh digest can be appended while preserving variable syntax.
		bareOriginal := imageStr
		if idx := strings.Index(imageStr, "@"); idx != -1 {
			bareOriginal = imageStr[:idx]
		}

		imgRef := parseImageReference(bareResolved)
		digest, err := myFlags.getImageDigest(&imgRef)
		if err != nil {
			log.Warn().Err(err).Str("image", bareResolved).Msg("failed to get digest, skipping")
			continue
		}

		if cur, ok := pinned[imgRef.Tag]; ok && isTagMutation(cur, imgRef.Tag, digest, imgRef.Tag) {
			log.Warn().Msgf("SUSPICIOUS: %s — digest changed from %s to %s with the same tag. "+
				"The image tag may have been repointed. Verify before accepting.", bareResolved, cur, digest)
		}

		var newImageStr string
		if strings.Contains(imageStr, "$") {
			// Preserve variable syntax and append digest. No inline comment —
			// # mid-line is not valid in standard Dockerfile FROM syntax.
			newImageStr = bareOriginal + "@" + digest
		} else {
			newImageStr = formatDockerImage(imgRef, digest)
		}
		// Place the AS alias before the # tag comment so Docker can parse it.
		if alias != "" {
			if idx := strings.Index(newImageStr, " # "); idx >= 0 {
				newImageStr = newImageStr[:idx] + alias + newImageStr[idx:]
			} else {
				newImageStr = newImageStr + alias
			}
		}
		lines[i] = prefix + newImageStr
	}

	replacement := strings.Join(lines, "\n")

	myFlags.printDiff(file, string(content), replacement)

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

// formatDockerImage produces "image:tag@sha256:digest" — the tag is kept
// inline so it remains human-readable without needing a separate comment.
// Inline # comments on FROM lines are not valid in standard Dockerfile syntax.
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
	if ref.Tag != "" {
		b.WriteString(":")
		b.WriteString(ref.Tag)
	}
	b.WriteString("@")
	b.WriteString(digest)
	return b.String()
}

// parseArgDefaults extracts ARG name=default pairs from Dockerfile lines above
// the current FROM, returning a map used for variable expansion. ARG lines
// without a default value (ARG FOO) are omitted — they have nothing to expand.
func parseArgDefaults(lines []string) map[string]string {
	m := make(map[string]string)
	for _, line := range lines {
		parts := argDefaultRe.FindStringSubmatch(strings.TrimSpace(line))
		if parts != nil {
			m[parts[1]] = parts[2]
		}
	}
	return m
}

// expandDockerVars substitutes ${VAR} and $VAR references using the provided
// map, including Docker's ${VAR:-default} modifier. Unknown variables with no
// modifier are left as-is so the caller can detect incomplete expansion.
func expandDockerVars(s string, vars map[string]string) string {
	return os.Expand(s, func(key string) string {
		name, def, hasDefault := strings.Cut(key, ":-")
		if v, ok := vars[name]; ok {
			return v
		}
		if hasDefault {
			return def
		}
		return "${" + key + "}"
	})
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
