package core

import "strings"

// PreCommitAnalysis is the result of static-only analysis of a
// .pre-commit-config.yaml file. No network calls are made.
type PreCommitAnalysis struct {
	Repos []PreCommitRepoAnalysis
}

// PreCommitRepoAnalysis describes a single `- repo:` entry.
type PreCommitRepoAnalysis struct {
	// Repo is the repository URL as written, e.g.
	// "https://github.com/pre-commit/pre-commit-hooks".
	Repo string
	// Rev is the raw rev: value as written.
	Rev string
	// IsSHAPinned is true when Rev is a 40-char hex SHA (bare or "sha # tag").
	IsSHAPinned bool
	// Suppressed is true when either the repo: or rev: line carries
	// # ghat:suppress.
	Suppressed bool
	// Line is the 1-indexed source line of the `rev:` key.
	Line int
}

// AnalyzePreCommit performs static analysis on a .pre-commit-config.yaml file.
// Parsing mirrors rewritePreCommitRevs in pre-commit.go (line-based) so the
// pinned/suppressed verdict is identical to what `ghat sift` would act on.
func AnalyzePreCommit(content []byte) PreCommitAnalysis {
	var a PreCommitAnalysis
	lines := strings.Split(string(content), "\n")

	var currentRepo string
	var repoSuppressed bool

	for i, line := range lines {
		trimmed := strings.TrimSpace(strings.SplitN(line, "#", 2)[0])
		bare := strings.TrimLeft(strings.TrimPrefix(trimmed, "-"), " ")

		if after, ok := strings.CutPrefix(bare, "repo:"); ok {
			currentRepo = strings.TrimSpace(after)
			repoSuppressed, _ = parseSuppression(line)
			continue
		}
		if after, ok := strings.CutPrefix(bare, "rev:"); ok {
			if currentRepo == "" || currentRepo == "local" || currentRepo == "meta" {
				continue
			}
			rev := strings.TrimSpace(after)
			revSuppressed, _ := parseSuppression(line)
			a.Repos = append(a.Repos, PreCommitRepoAnalysis{
				Repo:        currentRepo,
				Rev:         rev,
				IsSHAPinned: IsSHAPinnedRef(rev),
				Suppressed:  repoSuppressed || revSuppressed,
				Line:        i + 1,
			})
		}
	}
	return a
}
