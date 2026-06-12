# ghat — Claude guidance

## What this project is

`ghat` is a CLI tool and Go library for auditing and updating GitHub Actions workflow
files. It pins `uses:` action references to immutable commit SHAs, pins container image
references to digest SHAs, inserts least-privilege `permissions:` blocks, and detects
dangerous trigger patterns.

Module path: `github.com/jameswoolfenden/ghat`
Main package: `github.com/jameswoolfenden/ghat/src/core`

## Exported static-analysis API (`src/core/analysis.go`)

An exported, network-free analysis API was added so external tools (e.g. holden) can
consume ghat's checks as a library without importing the CLI flags or making network calls.

### Types

```go
type WorkflowAnalysis struct {
    HasPermissions       bool   // top-level permissions: block is present
    IsWriteAll           bool   // permissions: write-all is set
    HasDangerousTrigger  bool   // pull_request_target+checkout OR run: injection
    DangerousTriggerDesc string // human-readable description of the trigger pattern
    Steps                []StepAnalysis
    Jobs                 []JobAnalysis
}

type StepAnalysis struct {
    Action      string // "actions/checkout"
    Ref         string // raw ref as written; "" when no @ref at all
    IsSHAPinned bool   // ref is a 40-char SHA (bare or "sha # tag" format)
    SHA         string // extracted SHA when IsSHAPinned is true
    Tag         string // tag from "sha # tag" comment, or floating tag/branch
    Suppressed  bool   // uses: line carries # ghat:suppress
}

type JobAnalysis struct {
    Name           string
    HasTimeout     bool
    TimeoutMinutes int
}
```

### Functions

```go
// AnalyzeWorkflow performs fully static analysis on a workflow file's raw bytes.
// No network calls, no file I/O beyond what the caller provides.
func AnalyzeWorkflow(filename string, content []byte) WorkflowAnalysis

// IsSHAPinnedRef reports whether a raw ref value is pinned to an immutable SHA.
// Accepts both a bare 40-char hex SHA and the "sha # tag" comment format.
func IsSHAPinnedRef(ref string) bool
```

### Tests

Tests live in `src/core/analysis_test.go`. Run with:

```
go test ./src/core/ -run "^TestIsSHAPinnedRef$|^TestAnalyzeWorkflow"
```

## Key internals consumed by the analysis API

All of these are unexported package-level symbols in `src/core/`:

| Symbol | File | Purpose |
|---|---|---|
| `permsRe` | `audit_checks.go` | matches `^permissions:` |
| `writeAllRe` | `gha.go` | matches `permissions: write-all` |
| `prTargetRe` | `audit_checks.go` | matches `pull_request_target:` trigger |
| `checkoutPRRe` | `audit_checks.go` | matches checkout of PR head ref |
| `runInjectRe` | `audit_checks.go` | matches `github.event.*` in `run:` block |
| `parsePinnedRef` | `gha.go` | extracts SHA+tag from "sha # tag" format |
| `parseSuppression` | `suppress.go` | parses `# ghat:suppress` annotation |

## `runInjectRe` — both YAML step forms

`runInjectRe` was fixed (v0.1.32) to match both the compact and expanded `run:` forms:

```
# compact (list-item dash on same line as run:)
- run: echo ${{ github.event.issue.title }}

# expanded (run: on its own line under a named step)
- name: greet
  run: echo ${{ github.event.issue.title }}
```

Pattern: `` `(?m)^\s*(?:-\s+)?run:\s*.*\$\{\{\s*github\.event\.` ``

**Known limitation**: multi-line `run: |` blocks where the injection is on a continuation
line (not the `run:` line itself) are not detected by this regex.

## Suppression annotation

`# ghat:suppress` on a `uses:` line marks a step as intentionally exempt from pinning.
An optional reason can be provided:

```yaml
- uses: actions/checkout@v4  # ghat:suppress:reason=tag pinning is our design choice
```

`parseSuppression(line string) (bool, string)` returns `(suppressed, reason)`.

## Commit message style

Do **not** add `Co-Authored-By:` attribution lines to commits in this repo.

## Release process

Releases are tagged on `master`. The current series is `v0.1.x`. After landing changes,
tag and push:

```
git tag v0.1.<N>
git push origin master --tags
```

External consumers (e.g. holden) pin to a specific tag via `go get`.
