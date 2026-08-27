package core

import (
	"regexp"
	"strings"
)

// stageAliasRe extracts the alias name from an " AS alias" suffix.
var stageAliasRe = regexp.MustCompile(`(?i)^\s+AS\s+(\S+)`)

// DockerfileAnalysis is the result of static-only analysis of a Dockerfile.
// No registry lookups are made.
type DockerfileAnalysis struct {
	Images []DockerImageAnalysis
}

// DockerImageAnalysis describes a single FROM directive.
type DockerImageAnalysis struct {
	// Raw is the image reference as written, e.g. "golang:1.21" or
	// "golang:1.21@sha256:abc…".
	Raw string
	// Resolved is Raw with ARG defaults expanded, e.g. "golang:1.21" when
	// Raw was "golang:${GO}" and ARG GO=1.21 preceded it.
	Resolved string
	// Image is the repository portion after ARG expansion, without tag/digest.
	Image string
	// Tag is the tag portion after ARG expansion, or "" when none was given.
	Tag string
	// IsDigestPinned is true when the reference contains "@sha256:".
	IsDigestPinned bool
	// Suppressed is true when the FROM line carries # ghat:suppress.
	Suppressed bool
	// Trusted is true when the image is from a trusted registry (configured
	// via trusted_registries in .ghat.yml). Trusted images don't need pinning.
	Trusted bool
	// Line is the 1-indexed source line of the FROM directive.
	Line int
}

// AnalyzeDockerfile performs static analysis on Dockerfile content. ARG
// defaults declared above each FROM are expanded so that
// `ARG GO=1.21` / `FROM golang:${GO}` is correctly classified.
// Multi-stage build stage aliases (FROM ... AS <alias>) are tracked and
// skipped when referenced as an image target (FROM <alias>).
func AnalyzeDockerfile(content []byte) DockerfileAnalysis {
	return AnalyzeDockerfileWithTrusted(content, nil)
}

// AnalyzeDockerfileWithTrusted is like AnalyzeDockerfile but also marks
// images from trusted registries. trustedRegistries is a list of registry
// prefixes (e.g., "gcr.io/myproject/").
func AnalyzeDockerfileWithTrusted(content []byte, trustedRegistries []string) DockerfileAnalysis {
	var a DockerfileAnalysis
	lines := strings.Split(string(content), "\n")
	stageAliases := make(map[string]bool)

	for i, line := range lines {
		code := line
		if h := strings.Index(code, "#"); h >= 0 {
			code = code[:h]
		}
		m := fromRe.FindStringSubmatch(code)
		if m == nil {
			continue
		}
		raw := m[2]

		// Track stage aliases defined with "AS <alias>".
		if m[3] != "" {
			if aliasMatch := stageAliasRe.FindStringSubmatch(m[3]); aliasMatch != nil {
				stageAliases[strings.ToLower(aliasMatch[1])] = true
			}
		}

		if raw == "scratch" {
			continue
		}

		// Skip references to earlier build stages (e.g., FROM base).
		resolved := expandDockerVars(raw, parseArgDefaults(lines[:i]))
		baseName := resolved
		if colon := strings.Index(baseName, ":"); colon >= 0 {
			baseName = baseName[:colon]
		}
		if at := strings.Index(baseName, "@"); at >= 0 {
			baseName = baseName[:at]
		}
		if stageAliases[strings.ToLower(baseName)] {
			continue
		}

		suppressed, _ := parseSuppression(line)

		bare := resolved
		if at := strings.Index(bare, "@"); at >= 0 {
			bare = bare[:at]
		}
		ref := parseImageReference(bare)

		a.Images = append(a.Images, DockerImageAnalysis{
			Raw:            raw,
			Resolved:       resolved,
			Image:          ref.Repository,
			Tag:            ref.Tag,
			IsDigestPinned: strings.Contains(resolved, "@sha256:"),
			Suppressed:     suppressed,
			Trusted:        IsTrustedRegistryFromList(resolved, trustedRegistries),
			Line:           i + 1,
		})
	}
	return a
}
