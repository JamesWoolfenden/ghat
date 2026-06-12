package core

import "strings"

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
	// Line is the 1-indexed source line of the FROM directive.
	Line int
}

// AnalyzeDockerfile performs static analysis on Dockerfile content. ARG
// defaults declared above each FROM are expanded so that
// `ARG GO=1.21` / `FROM golang:${GO}` is correctly classified.
func AnalyzeDockerfile(content []byte) DockerfileAnalysis {
	var a DockerfileAnalysis
	lines := strings.Split(string(content), "\n")

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
		if raw == "scratch" {
			continue
		}
		suppressed, _ := parseSuppression(line)

		resolved := expandDockerVars(raw, parseArgDefaults(lines[:i]))
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
			Line:           i + 1,
		})
	}
	return a
}
