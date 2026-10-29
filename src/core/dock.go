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

// copyFromRe matches a COPY line that copies from another image or build stage, capturing
// everything up to and including "--from=", the reference itself, and the rest of the line.
// --from can sit anywhere among COPY's flags (e.g. after --chown).
var copyFromRe = regexp.MustCompile(`(?i)^(\s*COPY\s+(?:\S+\s+)*?--from=)(\S+)(\s.*)$`)

// stageIndexRe matches a numeric build-stage reference (COPY --from=0).
var stageIndexRe = regexp.MustCompile(`^\d+$`)

// imageDigestFunc resolves an image reference to its digest. A variable so tests can
// observe exactly which references get resolved without reaching a registry.
var imageDigestFunc = func(f *Flags, ref *ImageReference) (string, error) {
	return f.getImageDigest(ref)
}

// UpdateDockerfiles pins FROM and COPY --from image references in all Dockerfiles found in the entries.
func (f *Flags) UpdateDockerfiles() error {
	for _, file := range f.GetDockerfiles() {
		if err := f.UpdateDockerfile(file); err != nil {
			if f.ContinueOnError {
				log.Warn().Err(err).Str("file", file).Msg("skipping file")
				continue
			}
			return err
		}
	}
	return nil
}

// UpdateDockerfile pins image references in a single Dockerfile to SHA digests: FROM lines,
// and COPY --from=<image> lines, which pull from a registry just like FROM does but read like
// a stage reference and are easy to miss.
// Output format: image:tag@sha256:digest  (valid Docker syntax, tag preserved inline).
// Multi-stage build stage aliases (FROM ... AS <alias>) are tracked and skipped when
// referenced as an image target, as are numeric stage indexes (COPY --from=0).
func (f *Flags) UpdateDockerfile(file string) error {
	content, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", file, err)
	}

	pinned := parsePinnedFromLines(string(content))
	stageAliases := make(map[string]bool)

	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		if m := fromRe.FindStringSubmatch(line); m != nil {
			prefix, imageStr, alias := m[1], m[2], m[3]

			// Track stage aliases defined with "AS <alias>".
			if alias != "" {
				if aliasMatch := stageAliasRe.FindStringSubmatch(alias); aliasMatch != nil {
					stageAliases[strings.ToLower(aliasMatch[1])] = true
				}
			}

			if imageStr == "scratch" {
				continue
			}
			if newImageStr, ok := f.pinDockerImage("FROM", imageStr, line, lines[:i], stageAliases, pinned); ok {
				lines[i] = prefix + newImageStr + alias
			}
			continue
		}

		if m := copyFromRe.FindStringSubmatch(line); m != nil {
			prefix, imageStr, rest := m[1], m[2], m[3]
			if stageIndexRe.MatchString(imageStr) {
				continue
			}
			if newImageStr, ok := f.pinDockerImage("COPY --from", imageStr, line, lines[:i], stageAliases, pinned); ok {
				lines[i] = prefix + newImageStr + rest
			}
		}
	}

	replacement := strings.Join(lines, "\n")

	f.printDiff(file, string(content), replacement)

	if !f.DryRun && string(content) != replacement {
		if err := os.WriteFile(file, []byte(replacement), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", file, err)
		}
	}
	return nil
}

// pinDockerImage resolves one image reference from a FROM or COPY --from line (kind names
// which, for logging) and returns its digest-pinned replacement. ok is false when the
// reference should be left as written: a build stage, suppressed, trusted, unpinnable
// because of variables, already pinned by bare digest, or unresolvable.
func (f *Flags) pinDockerImage(kind, imageStr, line string, priorLines []string, stageAliases map[string]bool, pinned map[string]string) (string, bool) {
	// Skip references to earlier build stages (e.g., FROM base, COPY --from=builder).
	baseName := imageStr
	if colon := strings.Index(baseName, ":"); colon >= 0 {
		baseName = baseName[:colon]
	}
	if at := strings.Index(baseName, "@"); at >= 0 {
		baseName = baseName[:at]
	}
	if stageAliases[strings.ToLower(baseName)] {
		return "", false
	}
	if ok, reason := parseSuppression(line); ok {
		log.Info().Str("image", imageStr).Str("reason", reason).Msgf("skipping suppressed %s line", kind)
		return "", false
	}

	// Expand any ARG variable references using defaults declared above this line.
	resolvedStr := expandDockerVars(imageStr, parseArgDefaults(priorLines))

	// Skip images from trusted registries.
	if f.IsTrustedRegistry(resolvedStr) {
		log.Info().Str("image", resolvedStr).Msg("skipping trusted registry")
		return "", false
	}

	if strings.Contains(resolvedStr, "$") {
		// Still has unexpanded references — classify by position.
		switch {
		case strings.HasPrefix(imageStr, "$"):
			log.Warn().Msgf("SUPPLY CHAIN RISK: %s uses a dynamic image reference '%s' which cannot be pinned — resolve to a specific tag and digest", kind, imageStr)
		case strings.Contains(imageStr, "@$"):
			log.Info().Str("image", imageStr).Msg("digest is externally pinned via variable, skipping")
		default:
			log.Info().Str("image", imageStr).Msg("skipping image with unexpanded variable tag, cannot pin")
		}
		return "", false
	}

	// Already pinned by digest with no tag at all (e.g. "gcr.io/distroless/static@sha256:...").
	// There is no tag to re-resolve or check for mutation against — the digest is already
	// the complete, immutable reference — so leave it untouched. Without this check, the
	// digest strip below leaves no tag, parseImageReference defaults that to an implicit
	// "latest", and the digest gets re-resolved against whatever that registry's current
	// best-semver tag happens to be — silently swapping in an unrelated image.
	if beforeDigest, _, found := strings.Cut(resolvedStr, "@sha256:"); found {
		repoPart := beforeDigest
		if slash := strings.LastIndex(beforeDigest, "/"); slash != -1 {
			repoPart = beforeDigest[slash+1:]
		}
		if !strings.Contains(repoPart, ":") {
			log.Info().Str("image", resolvedStr).Msg("already pinned by digest with no tag, skipping")
			return "", false
		}
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
	digest, err := imageDigestFunc(f, &imgRef)
	if err != nil {
		log.Warn().Err(err).Str("image", bareResolved).Msg("failed to get digest, skipping")
		return "", false
	}

	if cur, ok := pinned[imgRef.Tag]; ok && isTagMutation(cur, imgRef.Tag, digest, imgRef.Tag) {
		log.Warn().Msgf("SUSPICIOUS: %s — digest changed from %s to %s with the same tag. "+
			"The image tag may have been repointed. Verify before accepting.", bareResolved, cur, digest)
	}

	if strings.Contains(imageStr, "$") {
		// Preserve variable syntax and append digest. No inline comment —
		// # mid-line is not valid in standard Dockerfile syntax.
		return bareOriginal + "@" + digest, true
	}
	return formatDockerImage(imgRef, digest), true
}

// GetDockerfiles returns all Dockerfile paths from the scanned entries.
func (f *Flags) GetDockerfiles() []string {
	var files []string
	for _, entry := range f.Entries {
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
