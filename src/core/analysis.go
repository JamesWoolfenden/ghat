package core

import (
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// WorkflowAnalysis is the result of static-only analysis of a single GitHub
// Actions workflow file. No network calls are made; results depend only on the
// file content supplied.
type WorkflowAnalysis struct {
	// HasPermissions is true when the workflow declares a top-level
	// permissions: block (any value, including write-all).
	HasPermissions bool
	// IsWriteAll is true when permissions: write-all is set, granting the
	// GITHUB_TOKEN full repository write access to every job.
	IsWriteAll bool
	// HasDangerousTrigger is true when a dangerous trigger combination is
	// detected:
	//   - pull_request_target with a checkout of the PR head, OR
	//   - github.event.* interpolated directly into a run: shell block.
	HasDangerousTrigger  bool
	DangerousTriggerDesc string
	// Steps is the ordered list of external uses: action references found in
	// the workflow. Local paths, docker:// refs, and reusable workflow calls
	// are excluded.
	Steps []StepAnalysis
	// Jobs is the per-job analysis, sorted by job name.
	Jobs []JobAnalysis
}

// StepAnalysis describes a single external uses: step.
type StepAnalysis struct {
	// Action is the action reference without the @ref part, e.g.
	// "actions/checkout" or "aws-actions/configure-aws-credentials".
	Action string
	// Ref is the raw ref as written in the YAML, e.g. "v4" or the ghat
	// pinned format "abc1234…  # v4".
	Ref string
	// IsSHAPinned is true when Ref is anchored to an immutable 40-char
	// commit SHA (bare or in the "sha # tag" comment format).
	IsSHAPinned bool
	// SHA is the extracted commit SHA when IsSHAPinned is true.
	SHA string
	// Tag is the human-readable tag associated with SHA (from the
	// "sha # tag" comment), or the raw floating tag when not yet pinned.
	Tag string
	// Suppressed is true when the uses: line carries a # ghat:suppress
	// annotation — the step is intentionally exempt from pinning.
	Suppressed bool
}

// JobAnalysis describes a single job in the workflow.
type JobAnalysis struct {
	// Name is the job key in the YAML, e.g. "build" or "deploy".
	Name string
	// HasTimeout is true when timeout-minutes: is declared on the job.
	HasTimeout     bool
	TimeoutMinutes int
	// IsReusable is true when the job delegates entirely to a reusable
	// workflow via a job-level `uses:` key.  GitHub does not support
	// timeout-minutes on such jobs; the timeout lives inside the called
	// workflow.
	IsReusable bool
}

var shaOnlyRe = regexp.MustCompile(`^[0-9a-f]{40}`)

// IsSHAPinnedRef reports whether a raw ref value is pinned to an immutable
// commit SHA. It accepts both a bare 40-char hex SHA and the "sha # tag"
// comment format that ghat writes when pinning.
func IsSHAPinnedRef(ref string) bool {
	return shaOnlyRe.MatchString(strings.TrimSpace(ref))
}

// usesExtractRe matches any uses: line regardless of leading indentation or
// list-item dashes, capturing everything after the colon.
var usesExtractRe = regexp.MustCompile(`(?m)^\s*-?\s*uses:\s*(.+)`)

// AnalyzeWorkflow performs static analysis on the content of a GitHub Actions
// workflow file. filename is used only for descriptive fields in the result;
// no I/O is performed and no network calls are made.
//
// The function reuses the regexes and helpers already present in this package
// (permsRe, writeAllRe, prTargetRe, checkoutPRRe, runInjectRe, parsePinnedRef,
// parseSuppression) so the analysis stays in sync with ghat's own checks.
func AnalyzeWorkflow(filename string, content []byte) WorkflowAnalysis {
	var a WorkflowAnalysis

	// Permissions checks — regex on raw bytes avoids YAML parse overhead and
	// is identical to what ensurePermissions / checkPermissions already do.
	a.HasPermissions = permsRe.Match(content)
	a.IsWriteAll = writeAllRe.Match(content)

	// Dangerous trigger detection — same logic as checkDangerousTrigger.
	if prTargetRe.Match(content) && checkoutPRRe.Match(content) {
		a.HasDangerousTrigger = true
		a.DangerousTriggerDesc = filename + ": pull_request_target with PR head checkout"
	} else if runInjectRe.Match(content) {
		a.HasDangerousTrigger = true
		a.DangerousTriggerDesc = filename + ": github.event.* interpolated into run:"
	}

	a.Steps = analyzeSteps(content)
	a.Jobs = analyzeJobs(content)

	return a
}

// analyzeSteps extracts every external uses: reference from the workflow
// content, classifying each one for SHA pinning status and suppression.
func analyzeSteps(content []byte) []StepAnalysis {
	var steps []StepAnalysis

	for _, match := range usesExtractRe.FindAllSubmatch(content, -1) {
		line := string(match[0])
		rawValue := strings.TrimSpace(string(match[1]))

		suppressed, _ := parseSuppression(line)

		// Strip YAML string quoting ("uses: \"owner/action@tag\"").
		if len(rawValue) > 1 {
			if q := rawValue[0]; q == '"' || q == '\'' {
				rawValue = rawValue[1:]
				if len(rawValue) > 0 && rawValue[len(rawValue)-1] == q {
					rawValue = rawValue[:len(rawValue)-1]
				}
			}
		}

		parts := strings.SplitN(rawValue, "@", 2)
		action := strings.TrimSpace(parts[0])

		// Skip local/composite action paths, docker:// refs, and reusable
		// workflow calls — these have no version registry to pin against.
		if strings.HasPrefix(action, ".") ||
			strings.HasPrefix(action, "/") ||
			strings.HasPrefix(action, "docker://") ||
			strings.Contains(action, "/.github/workflows/") {
			continue
		}

		step := StepAnalysis{Action: action, Suppressed: suppressed}

		if len(parts) > 1 {
			ref := strings.TrimSpace(parts[1])
			// Skip dynamic expressions — they cannot be statically analysed.
			if strings.HasPrefix(ref, "$") {
				continue
			}
			step.Ref = ref
			step.IsSHAPinned = IsSHAPinnedRef(ref)
			if sha, tag := parsePinnedRef(ref); sha != "" {
				// Pinned in "sha # tag" format.
				step.SHA = sha
				step.Tag = tag
			} else if step.IsSHAPinned {
				// Bare SHA with no tag comment.
				step.SHA = strings.Fields(ref)[0]
			} else {
				// Floating tag or branch ref.
				step.Tag = ref
			}
		}

		steps = append(steps, step)
	}

	return steps
}

// workflowJobsOnly is a minimal YAML struct for extracting per-job fields
// that are not captured by the existing ghaWorkflow / ghaJob structs.
type workflowJobsOnly struct {
	Jobs map[string]workflowJobTimeout `yaml:"jobs"`
}

type workflowJobTimeout struct {
	TimeoutMinutes *int   `yaml:"timeout-minutes"`
	Uses           string `yaml:"uses"`
}

// analyzeJobs parses the workflow YAML and returns per-job metadata sorted by
// job name for deterministic output.
func analyzeJobs(content []byte) []JobAnalysis {
	var wf workflowJobsOnly
	if err := yaml.Unmarshal(content, &wf); err != nil {
		return nil
	}

	jobs := make([]JobAnalysis, 0, len(wf.Jobs))
	for name, job := range wf.Jobs {
		j := JobAnalysis{Name: name, IsReusable: job.Uses != ""}
		if job.TimeoutMinutes != nil {
			j.HasTimeout = true
			j.TimeoutMinutes = *job.TimeoutMinutes
		}
		jobs = append(jobs, j)
	}
	sort.Slice(jobs, func(i, k int) bool { return jobs[i].Name < jobs[k].Name })
	return jobs
}
