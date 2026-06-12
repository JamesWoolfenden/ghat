package core

import (
	"sort"
	"strings"

	yaml "gopkg.in/yaml.v3"
)

// GitlabCIAnalysis is the result of static-only analysis of a .gitlab-ci.yml
// file. No network calls are made; results depend only on the content supplied.
type GitlabCIAnalysis struct {
	// Jobs is the ordered list of job definitions found in the file, sorted
	// by job name for deterministic output.
	Jobs []GitlabJobAnalysis
}

// GitlabJobAnalysis describes a single job in .gitlab-ci.yml.
type GitlabJobAnalysis struct {
	// Name is the job key in the YAML.
	Name string
	// HasTimeout is true when the job declares a timeout: field.
	HasTimeout bool
	// AllowFailure is true when allow_failure: true is set, or when
	// allow_failure: is an object (partial failure via exit_codes).
	AllowFailure bool
	// Images is the list of container images declared for this job.
	Images []GitlabImageAnalysis
	// Line is the 1-indexed source line of the job key. 0 when unknown.
	Line int
}

// GitlabImageAnalysis describes a container image used in a GitLab CI job.
type GitlabImageAnalysis struct {
	// Name is the image reference exactly as written in the YAML
	// (before any comment stripping), e.g. "golang:1.21" or
	// "gcr.io/project/app@sha256:abc123 # v1.6.0".
	Name string
	// IsDigestPinned is true when the image reference contains "@sha256:".
	IsDigestPinned bool
	// IsSuppressed is true when the image line carries a # ghat:suppress
	// annotation in the source file.
	IsSuppressed bool
	// Line is the 1-indexed source line of the image: declaration. 0 when unknown.
	Line int
}

// gitlabNonJobKeys are top-level .gitlab-ci.yml keys that are configuration
// directives, not job definitions. Any key NOT in this set is treated as a
// job name.
var gitlabNonJobKeys = map[string]bool{
	"stages":        true,
	"variables":     true,
	"include":       true,
	"workflow":      true,
	"default":       true,
	"image":         true,
	"services":      true,
	"before_script": true,
	"after_script":  true,
	"cache":         true,
	"pages":         true,
}

// gitlabJobYAML is a minimal struct for per-job fields in .gitlab-ci.yml.
type gitlabJobYAML struct {
	Timeout      interface{} `yaml:"timeout"`
	AllowFailure interface{} `yaml:"allow_failure"`
	Image        interface{} `yaml:"image"`
}

// AnalyzeGitlabCI performs static-only analysis of a .gitlab-ci.yml file.
// No network calls are made; all analysis is performed on the supplied content.
//
// The function returns metadata about each job: timeout declaration, allow_failure
// setting, and container image digest-pinning status.
func AnalyzeGitlabCI(content []byte) GitlabCIAnalysis {
	var a GitlabCIAnalysis

	var root yaml.Node
	if err := yaml.Unmarshal(content, &root); err != nil {
		return a
	}
	doc := &root
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		doc = doc.Content[0]
	}
	if doc.Kind != yaml.MappingNode {
		return a
	}

	rawStr := string(content)

	for i := 0; i+1 < len(doc.Content); i += 2 {
		key, val := doc.Content[i], doc.Content[i+1]
		name := key.Value
		if gitlabNonJobKeys[name] {
			continue
		}

		var job gitlabJobYAML
		_ = val.Decode(&job)

		ja := GitlabJobAnalysis{
			Name:         name,
			Line:         key.Line,
			HasTimeout:   job.Timeout != nil,
			AllowFailure: gitlabAllowFailureValue(job.AllowFailure),
		}

		// Extract the job-level image, if any.
		if job.Image != nil {
			imgName := gitlabImageNameStr(job.Image)
			if imgName != "" && !strings.HasPrefix(imgName, "$") {
				suppressed, _ := imageLineSuppression(rawStr, imgName)
				ja.Images = append(ja.Images, GitlabImageAnalysis{
					Name:           imgName,
					IsDigestPinned: strings.Contains(imgName, "@sha256:"),
					IsSuppressed:   suppressed,
					Line:           findImageLine(val, key.Line),
				})
			}
		}

		a.Jobs = append(a.Jobs, ja)
	}

	sort.Slice(a.Jobs, func(i, k int) bool { return a.Jobs[i].Name < a.Jobs[k].Name })
	return a
}

// findImageLine returns the line of the `image:` key inside a job mapping
// node, falling back to the job line.
func findImageLine(jobVal *yaml.Node, fallback int) int {
	if jobVal.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(jobVal.Content); i += 2 {
			if jobVal.Content[i].Value == "image" {
				return jobVal.Content[i].Line
			}
		}
	}
	return fallback
}

// gitlabImageNameStr extracts the image name string from either the short
// format ("image: golang:1.21") or the object format
// ("image:\n  name: golang:1.21").
func gitlabImageNameStr(v interface{}) string {
	switch x := v.(type) {
	case string:
		return x
	case map[string]interface{}:
		if n, ok := x["name"].(string); ok {
			return n
		}
	}
	return ""
}

// gitlabAllowFailureValue extracts the boolean allow_failure value. It handles:
//   - bool literal (allow_failure: true)
//   - object with exit_codes (allow_failure: { exit_codes: [137] }) → true
//   - absent / nil → false
func gitlabAllowFailureValue(v interface{}) bool {
	switch x := v.(type) {
	case bool:
		return x
	case map[string]interface{}:
		// allow_failure: { exit_codes: [...] } means some exits are allowed.
		return true
	default:
		_ = x
	}
	return false
}
