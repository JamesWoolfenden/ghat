package core

// Tests for Wave 1 additions to AnalyzeWorkflow and AnalyzeGitlabCI:
//   - WorkflowAnalysis.HasConcurrency
//   - StepAnalysis.ExposesSecretInEnv
//   - JobAnalysis.RunsOn
//   - JobAnalysis.HasPermissions / Permissions
//   - AnalyzeGitlabCI

import (
	"testing"
)

// ---------------------------------------------------------------------------
// HasConcurrency
// ---------------------------------------------------------------------------

func TestAnalyzeWorkflow_HasConcurrency_Present(t *testing.T) {
	content := []byte(`
on: [push]
permissions:
  contents: read
concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: true
jobs:
  build:
    runs-on: ubuntu-latest
    timeout-minutes: 10
    steps:
      - uses: actions/checkout@v4
`)
	a := AnalyzeWorkflow("ci.yml", content)
	if !a.HasConcurrency {
		t.Error("expected HasConcurrency = true when concurrency: block is present")
	}
}

func TestAnalyzeWorkflow_HasConcurrency_Absent(t *testing.T) {
	content := []byte(`
on: [push]
permissions:
  contents: read
jobs:
  build:
    runs-on: ubuntu-latest
    timeout-minutes: 10
    steps:
      - uses: actions/checkout@v4
`)
	a := AnalyzeWorkflow("ci.yml", content)
	if a.HasConcurrency {
		t.Error("expected HasConcurrency = false when no concurrency: block")
	}
}

func TestAnalyzeWorkflow_HasConcurrency_ScalarForm(t *testing.T) {
	// concurrency: as a plain string (not a mapping)
	content := []byte(`
on: [push]
concurrency: my-group
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
`)
	a := AnalyzeWorkflow("ci.yml", content)
	if !a.HasConcurrency {
		t.Error("expected HasConcurrency = true for scalar concurrency: form")
	}
}

// ---------------------------------------------------------------------------
// StepAnalysis.ExposesSecretInEnv
// ---------------------------------------------------------------------------

func TestAnalyzeWorkflow_StepExposesSecretInEnv_True(t *testing.T) {
	content := []byte(`
on: [push]
permissions:
  contents: read
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: some-org/some-action@v1
        env:
          API_TOKEN: ${{ secrets.API_TOKEN }}
        run: echo done
`)
	a := AnalyzeWorkflow("ci.yml", content)

	var found *StepAnalysis
	for i := range a.Steps {
		if a.Steps[i].Action == "some-org/some-action" {
			found = &a.Steps[i]
			break
		}
	}
	if found == nil {
		t.Fatal("some-org/some-action not found in steps")
	}
	if !found.ExposesSecretInEnv {
		t.Error("ExposesSecretInEnv should be true for step with ${{ secrets.* }} in env:")
	}
}

func TestAnalyzeWorkflow_StepExposesSecretInEnv_False(t *testing.T) {
	content := []byte(`
on: [push]
permissions:
  contents: read
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        env:
          SAFE_VAR: not-a-secret
`)
	a := AnalyzeWorkflow("ci.yml", content)
	for _, step := range a.Steps {
		if step.Action == "actions/checkout" && step.ExposesSecretInEnv {
			t.Error("ExposesSecretInEnv should be false when env: has no secrets")
		}
	}
}

func TestAnalyzeWorkflow_StepExposesSecretInEnv_NoEnv(t *testing.T) {
	content := []byte(`
on: [push]
permissions:
  contents: read
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.23"
`)
	a := AnalyzeWorkflow("ci.yml", content)
	for _, step := range a.Steps {
		if step.ExposesSecretInEnv {
			t.Errorf("step %q: ExposesSecretInEnv should be false when there is no env: block", step.Action)
		}
	}
}

// ---------------------------------------------------------------------------
// JobAnalysis.RunsOn
// ---------------------------------------------------------------------------

func TestAnalyzeWorkflow_JobRunsOn_String(t *testing.T) {
	content := []byte(`
on: [push]
permissions:
  contents: read
jobs:
  build:
    runs-on: ubuntu-latest
    timeout-minutes: 10
    steps:
      - uses: actions/checkout@v4
`)
	a := AnalyzeWorkflow("ci.yml", content)
	if len(a.Jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(a.Jobs))
	}
	if a.Jobs[0].RunsOn != "ubuntu-latest" {
		t.Errorf("RunsOn = %q, want ubuntu-latest", a.Jobs[0].RunsOn)
	}
}

func TestAnalyzeWorkflow_JobRunsOn_List(t *testing.T) {
	content := []byte(`
on: [push]
permissions:
  contents: read
jobs:
  build:
    runs-on: [self-hosted, linux, x64]
    timeout-minutes: 10
    steps:
      - uses: actions/checkout@v4
`)
	a := AnalyzeWorkflow("ci.yml", content)
	if len(a.Jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(a.Jobs))
	}
	if a.Jobs[0].RunsOn != "self-hosted,linux,x64" {
		t.Errorf("RunsOn = %q, want self-hosted,linux,x64", a.Jobs[0].RunsOn)
	}
}

func TestAnalyzeWorkflow_JobRunsOn_SelfHosted(t *testing.T) {
	content := []byte(`
on: [push]
permissions:
  contents: read
jobs:
  deploy:
    runs-on: self-hosted
    steps:
      - uses: actions/checkout@v4
`)
	a := AnalyzeWorkflow("ci.yml", content)
	if len(a.Jobs) == 0 {
		t.Fatal("no jobs found")
	}
	if a.Jobs[0].RunsOn != "self-hosted" {
		t.Errorf("RunsOn = %q, want self-hosted", a.Jobs[0].RunsOn)
	}
}

// ---------------------------------------------------------------------------
// JobAnalysis.HasPermissions / Permissions
// ---------------------------------------------------------------------------

func TestAnalyzeWorkflow_JobPermissions_Mapping(t *testing.T) {
	content := []byte(`
on: [push]
jobs:
  deploy:
    runs-on: ubuntu-latest
    permissions:
      id-token: write
      contents: read
    steps:
      - uses: actions/checkout@v4
`)
	a := AnalyzeWorkflow("ci.yml", content)
	if len(a.Jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(a.Jobs))
	}
	j := a.Jobs[0]
	if !j.HasPermissions {
		t.Error("HasPermissions should be true")
	}
	if j.Permissions["id-token"] != "write" {
		t.Errorf("Permissions[id-token] = %q, want write", j.Permissions["id-token"])
	}
	if j.Permissions["contents"] != "read" {
		t.Errorf("Permissions[contents] = %q, want read", j.Permissions["contents"])
	}
}

func TestAnalyzeWorkflow_JobPermissions_WriteAll(t *testing.T) {
	content := []byte(`
on: [push]
jobs:
  build:
    runs-on: ubuntu-latest
    permissions: write-all
    steps:
      - uses: actions/checkout@v4
`)
	a := AnalyzeWorkflow("ci.yml", content)
	if len(a.Jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(a.Jobs))
	}
	j := a.Jobs[0]
	if !j.HasPermissions {
		t.Error("HasPermissions should be true when permissions: write-all")
	}
	if j.Permissions["_all"] != "write-all" {
		t.Errorf("Permissions[_all] = %q, want write-all", j.Permissions["_all"])
	}
}

func TestAnalyzeWorkflow_JobPermissions_Absent(t *testing.T) {
	content := []byte(`
on: [push]
permissions:
  contents: read
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
`)
	a := AnalyzeWorkflow("ci.yml", content)
	if len(a.Jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(a.Jobs))
	}
	if a.Jobs[0].HasPermissions {
		t.Error("HasPermissions should be false when no per-job permissions block")
	}
}

// ---------------------------------------------------------------------------
// AnalyzeGitlabCI
// ---------------------------------------------------------------------------

func TestAnalyzeGitlabCI_BasicJobs(t *testing.T) {
	content := []byte(`
stages:
  - build
  - test

variables:
  DOCKER_IMAGE: golang:1.21

build-job:
  image: golang:1.21
  timeout: 30 minutes
  allow_failure: false
  script:
    - go build

test-job:
  image: node:18
  allow_failure: true
  script:
    - npm test
`)
	a := AnalyzeGitlabCI(content)

	if len(a.Jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d: %v", len(a.Jobs), jobNames(a))
	}

	// Jobs sorted alphabetically: build-job, test-job
	build := a.Jobs[0]
	if build.Name != "build-job" {
		t.Errorf("jobs[0].Name = %q, want build-job", build.Name)
	}
	if !build.HasTimeout {
		t.Error("build-job should have HasTimeout = true")
	}
	if build.AllowFailure {
		t.Error("build-job should have AllowFailure = false")
	}
	if len(build.Images) != 1 || build.Images[0].Name != "golang:1.21" {
		t.Errorf("build-job images = %v, want [golang:1.21]", build.Images)
	}

	test := a.Jobs[1]
	if test.Name != "test-job" {
		t.Errorf("jobs[1].Name = %q, want test-job", test.Name)
	}
	if test.HasTimeout {
		t.Error("test-job should have HasTimeout = false")
	}
	if !test.AllowFailure {
		t.Error("test-job should have AllowFailure = true")
	}
}

func TestAnalyzeGitlabCI_DigestPinning(t *testing.T) {
	content := []byte(`
build-job:
  image: golang:1.21
  script:
    - go build

pinned-job:
  image: golang@sha256:abc1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcd
  script:
    - go test
`)
	a := AnalyzeGitlabCI(content)

	if len(a.Jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(a.Jobs))
	}

	// alphabetical: build-job, pinned-job
	build := a.Jobs[0]
	if len(build.Images) != 1 || build.Images[0].IsDigestPinned {
		t.Errorf("build-job image should NOT be digest-pinned: %v", build.Images)
	}

	pinned := a.Jobs[1]
	if len(pinned.Images) != 1 || !pinned.Images[0].IsDigestPinned {
		t.Errorf("pinned-job image should be digest-pinned: %v", pinned.Images)
	}
}

func TestAnalyzeGitlabCI_ImageSuppression(t *testing.T) {
	content := []byte(`
build-job:
  image: golang:1.21 # ghat:suppress:reason=version locked by compliance team
  script:
    - go build

test-job:
  image: node:18
  script:
    - npm test
`)
	a := AnalyzeGitlabCI(content)

	if len(a.Jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(a.Jobs))
	}

	// alphabetical: build-job, test-job
	build := a.Jobs[0]
	if len(build.Images) != 1 || !build.Images[0].IsSuppressed {
		t.Errorf("build-job image should be suppressed: %v", build.Images)
	}

	test := a.Jobs[1]
	if len(test.Images) != 1 || test.Images[0].IsSuppressed {
		t.Errorf("test-job image should NOT be suppressed: %v", test.Images)
	}
}

func TestAnalyzeGitlabCI_ImageObjectFormat(t *testing.T) {
	content := []byte(`
build-job:
  image:
    name: node:18-alpine
    entrypoint: [""]
  script:
    - npm test
`)
	a := AnalyzeGitlabCI(content)

	if len(a.Jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(a.Jobs))
	}
	if len(a.Jobs[0].Images) != 1 || a.Jobs[0].Images[0].Name != "node:18-alpine" {
		t.Errorf("expected image node:18-alpine, got %v", a.Jobs[0].Images)
	}
}

func TestAnalyzeGitlabCI_SkipsNonJobKeys(t *testing.T) {
	content := []byte(`
stages:
  - build
variables:
  FOO: bar
default:
  image: golang:1.21
include:
  - project: myorg/templates
    file: common.yml
workflow:
  rules:
    - if: $CI_COMMIT_BRANCH
real-job:
  image: golang:1.21
  script:
    - go build
`)
	a := AnalyzeGitlabCI(content)

	for _, j := range a.Jobs {
		switch j.Name {
		case "stages", "variables", "default", "include", "workflow":
			t.Errorf("non-job key %q should not appear as a job", j.Name)
		}
	}

	if len(a.Jobs) != 1 || a.Jobs[0].Name != "real-job" {
		t.Errorf("expected only real-job, got %v", jobNames(a))
	}
}

func TestAnalyzeGitlabCI_AllowFailureObjectForm(t *testing.T) {
	content := []byte(`
flaky-job:
  image: golang:1.21
  allow_failure:
    exit_codes:
      - 137
  script:
    - go test
`)
	a := AnalyzeGitlabCI(content)

	if len(a.Jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(a.Jobs))
	}
	if !a.Jobs[0].AllowFailure {
		t.Error("allow_failure with exit_codes object should be AllowFailure = true")
	}
}

func TestAnalyzeGitlabCI_Empty(t *testing.T) {
	a := AnalyzeGitlabCI([]byte{})
	if len(a.Jobs) != 0 {
		t.Errorf("empty content should produce 0 jobs, got %d", len(a.Jobs))
	}
}

func TestAnalyzeGitlabCI_InvalidYAML(t *testing.T) {
	a := AnalyzeGitlabCI([]byte(`{{{ not yaml`))
	if len(a.Jobs) != 0 {
		t.Errorf("invalid YAML should produce 0 jobs, got %d", len(a.Jobs))
	}
}

func TestAnalyzeGitlabCI_VariableImagesSkipped(t *testing.T) {
	content := []byte(`
build-job:
  image: $CI_REGISTRY_IMAGE
  script:
    - go build
`)
	a := AnalyzeGitlabCI(content)

	if len(a.Jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(a.Jobs))
	}
	if len(a.Jobs[0].Images) != 0 {
		t.Errorf("variable images should be skipped, got %v", a.Jobs[0].Images)
	}
}

func TestAnalyzeGitlabCI_Sorted(t *testing.T) {
	content := []byte(`
zebra-job:
  image: golang:1.21
  script: [go build]
alpha-job:
  image: node:18
  script: [npm test]
mango-job:
  image: python:3.12
  script: [python -m pytest]
`)
	a := AnalyzeGitlabCI(content)

	if len(a.Jobs) != 3 {
		t.Fatalf("expected 3 jobs, got %d", len(a.Jobs))
	}
	want := []string{"alpha-job", "mango-job", "zebra-job"}
	for i, j := range a.Jobs {
		if j.Name != want[i] {
			t.Errorf("jobs[%d].Name = %q, want %q (not sorted)", i, j.Name, want[i])
		}
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func jobNames(a GitlabCIAnalysis) []string {
	names := make([]string, len(a.Jobs))
	for i, j := range a.Jobs {
		names[i] = j.Name
	}
	return names
}
