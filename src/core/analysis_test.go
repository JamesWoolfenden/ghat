package core

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// IsSHAPinnedRef
// ---------------------------------------------------------------------------

func TestIsSHAPinnedRef(t *testing.T) {
	tests := []struct {
		ref  string
		want bool
	}{
		// 40-char SHA — pinned
		{"de0fac2e4500dabe0009e67214ff5f5447ce83dd", true},
		// "sha # tag" comment format — pinned
		{"de0fac2e4500dabe0009e67214ff5f5447ce83dd # v4", true},
		// floating tag — not pinned
		{"v4", false},
		{"main", false},
		// short SHA — not pinned (< 40 chars)
		{"de0fac2e45", false},
		// empty
		{"", false},
		// leading/trailing whitespace around a real SHA
		{"  de0fac2e4500dabe0009e67214ff5f5447ce83dd  ", true},
	}
	for _, tt := range tests {
		t.Run(tt.ref, func(t *testing.T) {
			if got := IsSHAPinnedRef(tt.ref); got != tt.want {
				t.Errorf("IsSHAPinnedRef(%q) = %v, want %v", tt.ref, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AnalyzeWorkflow — permissions
// ---------------------------------------------------------------------------

func TestAnalyzeWorkflow_HasPermissions(t *testing.T) {
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
	a := AnalyzeWorkflow("test.yml", content)
	if !a.HasPermissions {
		t.Error("expected HasPermissions = true")
	}
	if a.IsWriteAll {
		t.Error("expected IsWriteAll = false")
	}
}

func TestAnalyzeWorkflow_WriteAll(t *testing.T) {
	content := []byte(`
on: [push]
permissions: write-all
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
`)
	a := AnalyzeWorkflow("test.yml", content)
	if !a.HasPermissions {
		t.Error("expected HasPermissions = true when permissions: write-all is present")
	}
	if !a.IsWriteAll {
		t.Error("expected IsWriteAll = true")
	}
}

func TestAnalyzeWorkflow_NoPermissions(t *testing.T) {
	content := []byte(`
on: [push]
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
`)
	a := AnalyzeWorkflow("test.yml", content)
	if a.HasPermissions {
		t.Error("expected HasPermissions = false")
	}
}

// ---------------------------------------------------------------------------
// AnalyzeWorkflow — dangerous trigger
// ---------------------------------------------------------------------------

func TestAnalyzeWorkflow_DangerousTrigger_PRT(t *testing.T) {
	// pull_request_target + PR-head checkout → dangerous
	content := []byte(`
on:
  pull_request_target:
    types: [opened]
permissions:
  contents: read
jobs:
  review:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          ref: ${{ github.event.pull_request.head.sha }}
`)
	a := AnalyzeWorkflow("ci.yml", content)
	if !a.HasDangerousTrigger {
		t.Error("expected HasDangerousTrigger = true for pull_request_target + PR checkout")
	}
	if !strings.Contains(a.DangerousTriggerDesc, "pull_request_target") {
		t.Errorf("DangerousTriggerDesc = %q, want to mention pull_request_target", a.DangerousTriggerDesc)
	}
	if !strings.Contains(a.DangerousTriggerDesc, "ci.yml") {
		t.Errorf("DangerousTriggerDesc = %q, want to contain the filename", a.DangerousTriggerDesc)
	}
}

func TestAnalyzeWorkflow_DangerousTrigger_RunInject(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			// Expanded form: run: on its own line under a named step
			name: "expanded form",
			content: `
on: [pull_request]
permissions:
  contents: read
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - name: greet
        run: echo ${{ github.event.pull_request.title }}
`,
		},
		{
			// Compact form: - run: on the same line as the list-item dash
			name: "compact form",
			content: `
on: [pull_request]
permissions:
  contents: read
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - run: echo ${{ github.event.pull_request.title }}
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := AnalyzeWorkflow("inject.yml", []byte(tt.content))
			if !a.HasDangerousTrigger {
				t.Errorf("expected HasDangerousTrigger = true for github.event.* in run: (%s)", tt.name)
			}
			if !strings.Contains(a.DangerousTriggerDesc, "github.event") {
				t.Errorf("DangerousTriggerDesc = %q, want to mention github.event", a.DangerousTriggerDesc)
			}
		})
	}
}

func TestAnalyzeWorkflow_NoDangerousTrigger(t *testing.T) {
	content := []byte(`
on: [push]
permissions:
  contents: read
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: echo "safe"
`)
	a := AnalyzeWorkflow("safe.yml", content)
	if a.HasDangerousTrigger {
		t.Errorf("expected HasDangerousTrigger = false, got desc: %s", a.DangerousTriggerDesc)
	}
}

// ---------------------------------------------------------------------------
// AnalyzeWorkflow — step extraction and SHA pinning
// ---------------------------------------------------------------------------

func TestAnalyzeWorkflow_Steps_SHAPinned(t *testing.T) {
	content := []byte(`
on: [push]
permissions:
  contents: read
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v4
      - uses: actions/setup-go@v5
      - uses: aws-actions/configure-aws-credentials@e3dd6a429d7300a6a4c196c26e071d42e0343502 # v4.0.2
`)
	a := AnalyzeWorkflow("test.yml", content)

	if len(a.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d: %v", len(a.Steps), a.Steps)
	}

	// Step 0: SHA-pinned with tag comment
	s0 := a.Steps[0]
	if s0.Action != "actions/checkout" {
		t.Errorf("steps[0].Action = %q, want actions/checkout", s0.Action)
	}
	if !s0.IsSHAPinned {
		t.Error("steps[0].IsSHAPinned should be true")
	}
	if s0.SHA != "de0fac2e4500dabe0009e67214ff5f5447ce83dd" {
		t.Errorf("steps[0].SHA = %q, want de0fac2e...", s0.SHA)
	}
	if s0.Tag != "v4" {
		t.Errorf("steps[0].Tag = %q, want v4", s0.Tag)
	}

	// Step 1: floating tag
	s1 := a.Steps[1]
	if s1.Action != "actions/setup-go" {
		t.Errorf("steps[1].Action = %q, want actions/setup-go", s1.Action)
	}
	if s1.IsSHAPinned {
		t.Error("steps[1].IsSHAPinned should be false (floating tag v5)")
	}
	if s1.Tag != "v5" {
		t.Errorf("steps[1].Tag = %q, want v5", s1.Tag)
	}

	// Step 2: SHA-pinned with longer tag
	s2 := a.Steps[2]
	if !s2.IsSHAPinned {
		t.Error("steps[2].IsSHAPinned should be true")
	}
	if s2.Tag != "v4.0.2" {
		t.Errorf("steps[2].Tag = %q, want v4.0.2", s2.Tag)
	}
}

func TestAnalyzeWorkflow_Steps_BareSHA(t *testing.T) {
	// Bare 40-char SHA with no tag comment
	content := []byte(`
on: [push]
permissions:
  contents: read
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd
`)
	a := AnalyzeWorkflow("test.yml", content)
	if len(a.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(a.Steps))
	}
	s := a.Steps[0]
	if !s.IsSHAPinned {
		t.Error("bare SHA step should be IsSHAPinned = true")
	}
	if s.SHA != "de0fac2e4500dabe0009e67214ff5f5447ce83dd" {
		t.Errorf("SHA = %q, want de0fac2e...", s.SHA)
	}
	if s.Tag != "" {
		t.Errorf("Tag = %q, want empty for bare SHA", s.Tag)
	}
}

func TestAnalyzeWorkflow_Steps_Suppressed(t *testing.T) {
	content := []byte(`
on: [push]
permissions:
  contents: read
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4  # ghat:suppress
      - uses: actions/setup-go@v5
`)
	a := AnalyzeWorkflow("test.yml", content)
	if len(a.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(a.Steps))
	}
	if !a.Steps[0].Suppressed {
		t.Error("steps[0] should be Suppressed = true")
	}
	if a.Steps[1].Suppressed {
		t.Error("steps[1] should be Suppressed = false")
	}
}

func TestAnalyzeWorkflow_Steps_SkipsLocalAndDocker(t *testing.T) {
	content := []byte(`
on: [push]
permissions:
  contents: read
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: ./.github/actions/local-action
      - uses: docker://alpine:3.18
      - uses: owner/repo/.github/workflows/reusable.yml@main
      - uses: actions/checkout@v4
`)
	a := AnalyzeWorkflow("test.yml", content)
	// Only actions/checkout should appear — local, docker, and reusable workflow refs are excluded
	if len(a.Steps) != 1 {
		t.Errorf("expected 1 step (only external), got %d: %v", len(a.Steps), a.Steps)
	}
	if len(a.Steps) > 0 && a.Steps[0].Action != "actions/checkout" {
		t.Errorf("expected actions/checkout, got %q", a.Steps[0].Action)
	}
}

func TestAnalyzeWorkflow_Steps_SkipsDynamicRef(t *testing.T) {
	content := []byte(`
on: [push]
permissions:
  contents: read
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@${{ env.CHECKOUT_VERSION }}
      - uses: actions/setup-go@v5
`)
	a := AnalyzeWorkflow("test.yml", content)
	// dynamic ref step is skipped; only setup-go counts
	if len(a.Steps) != 1 {
		t.Fatalf("expected 1 step (dynamic ref skipped), got %d: %v", len(a.Steps), a.Steps)
	}
	if a.Steps[0].Action != "actions/setup-go" {
		t.Errorf("expected actions/setup-go, got %q", a.Steps[0].Action)
	}
}

// ---------------------------------------------------------------------------
// AnalyzeWorkflow — job analysis (timeouts)
// ---------------------------------------------------------------------------

func TestAnalyzeWorkflow_Jobs_Timeout(t *testing.T) {
	content := []byte(`
on: [push]
permissions:
  contents: read
jobs:
  build:
    runs-on: ubuntu-latest
    timeout-minutes: 15
    steps:
      - uses: actions/checkout@v4
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
`)
	a := AnalyzeWorkflow("test.yml", content)
	if len(a.Jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(a.Jobs))
	}

	// Jobs are sorted by name: "build" < "deploy"
	build := a.Jobs[0]
	if build.Name != "build" {
		t.Errorf("jobs[0].Name = %q, want build", build.Name)
	}
	if !build.HasTimeout {
		t.Error("build job should have HasTimeout = true")
	}
	if build.TimeoutMinutes != 15 {
		t.Errorf("build.TimeoutMinutes = %d, want 15", build.TimeoutMinutes)
	}

	deploy := a.Jobs[1]
	if deploy.Name != "deploy" {
		t.Errorf("jobs[1].Name = %q, want deploy", deploy.Name)
	}
	if deploy.HasTimeout {
		t.Error("deploy job should have HasTimeout = false")
	}
}

func TestAnalyzeWorkflow_Jobs_Reusable(t *testing.T) {
	content := []byte(`
on: [push]
permissions:
  contents: read
  id-token: write
jobs:
  call-workflow:
    uses: JamesWoolfenden/.github/.github/workflows/terraform-verify-gcp.yml@main
    with:
      service_account: sa@project.iam.gserviceaccount.com
  build:
    runs-on: ubuntu-latest
    timeout-minutes: 30
    steps:
      - uses: actions/checkout@v4
`)
	a := AnalyzeWorkflow("test.yml", content)
	if len(a.Jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(a.Jobs))
	}

	// Jobs are sorted: "build" < "call-workflow"
	build := a.Jobs[0]
	if build.Name != "build" {
		t.Errorf("jobs[0].Name = %q, want build", build.Name)
	}
	if build.IsReusable {
		t.Error("build job should have IsReusable = false")
	}

	call := a.Jobs[1]
	if call.Name != "call-workflow" {
		t.Errorf("jobs[1].Name = %q, want call-workflow", call.Name)
	}
	if !call.IsReusable {
		t.Error("call-workflow job should have IsReusable = true")
	}
	if call.HasTimeout {
		t.Error("reusable workflow call job should have HasTimeout = false (timeout-minutes not supported)")
	}
}

func TestAnalyzeWorkflow_Jobs_Sorted(t *testing.T) {
	content := []byte(`
on: [push]
permissions:
  contents: read
jobs:
  zebra:
    runs-on: ubuntu-latest
    timeout-minutes: 5
    steps:
      - run: echo z
  alpha:
    runs-on: ubuntu-latest
    timeout-minutes: 10
    steps:
      - run: echo a
  mango:
    runs-on: ubuntu-latest
    steps:
      - run: echo m
`)
	a := AnalyzeWorkflow("test.yml", content)
	if len(a.Jobs) != 3 {
		t.Fatalf("expected 3 jobs, got %d", len(a.Jobs))
	}
	names := []string{a.Jobs[0].Name, a.Jobs[1].Name, a.Jobs[2].Name}
	want := []string{"alpha", "mango", "zebra"}
	for i := range names {
		if names[i] != want[i] {
			t.Errorf("jobs[%d].Name = %q, want %q (jobs not sorted)", i, names[i], want[i])
		}
	}
}

// ---------------------------------------------------------------------------
// AnalyzeWorkflow — RunSteps
// ---------------------------------------------------------------------------

func TestAnalyzeWorkflow_RunSteps_Basic(t *testing.T) {
	content := []byte(`
on: [push]
permissions:
  contents: read
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: |
          mkdir "$GITHUB_WORKSPACE/bin"
          wget https://example.com/tool.tar.gz
      - run: echo done
`)
	a := AnalyzeWorkflow("test.yml", content)
	if len(a.RunSteps) != 2 {
		t.Fatalf("expected 2 run steps, got %d", len(a.RunSteps))
	}
	if a.RunSteps[0].Job != "build" {
		t.Errorf("RunSteps[0].Job = %q, want build", a.RunSteps[0].Job)
	}
	if !strings.Contains(a.RunSteps[0].Run, "wget") {
		t.Errorf("RunSteps[0].Run = %q, want it to contain wget", a.RunSteps[0].Run)
	}
	if a.RunSteps[1].Run != "echo done" {
		t.Errorf("RunSteps[1].Run = %q, want %q", a.RunSteps[1].Run, "echo done")
	}
	// The uses: step must not show up in RunSteps.
	for _, rs := range a.RunSteps {
		if strings.Contains(rs.Run, "checkout") {
			t.Error("uses: step leaked into RunSteps")
		}
	}
}

func TestAnalyzeWorkflow_RunSteps_MultipleJobs(t *testing.T) {
	content := []byte(`
on: [push]
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - run: echo building
  deploy:
    runs-on: ubuntu-latest
    steps:
      - run: echo deploying
`)
	a := AnalyzeWorkflow("test.yml", content)
	if len(a.RunSteps) != 2 {
		t.Fatalf("expected 2 run steps, got %d", len(a.RunSteps))
	}
	jobs := map[string]bool{}
	for _, rs := range a.RunSteps {
		jobs[rs.Job] = true
	}
	if !jobs["build"] || !jobs["deploy"] {
		t.Errorf("expected run steps tagged with both build and deploy jobs, got %+v", a.RunSteps)
	}
}

func TestAnalyzeWorkflow_RunSteps_ReusableJobHasNone(t *testing.T) {
	content := []byte(`
on: [push]
jobs:
  call-workflow:
    uses: ./.github/workflows/reusable.yml
`)
	a := AnalyzeWorkflow("test.yml", content)
	if len(a.RunSteps) != 0 {
		t.Errorf("expected 0 run steps for a reusable-workflow job, got %d", len(a.RunSteps))
	}
}

func TestAnalyzeWorkflow_RunSteps_Empty(t *testing.T) {
	a := AnalyzeWorkflow("empty.yml", []byte{})
	if len(a.RunSteps) != 0 {
		t.Errorf("expected 0 run steps, got %d", len(a.RunSteps))
	}
}

// ---------------------------------------------------------------------------
// AnalyzeWorkflow — empty / invalid input
// ---------------------------------------------------------------------------

func TestAnalyzeWorkflow_Empty(t *testing.T) {
	a := AnalyzeWorkflow("empty.yml", []byte{})
	if a.HasPermissions || a.IsWriteAll || a.HasDangerousTrigger {
		t.Error("empty content should produce zero-value WorkflowAnalysis booleans")
	}
	if len(a.Steps) != 0 {
		t.Errorf("expected 0 steps, got %d", len(a.Steps))
	}
}

// ---------------------------------------------------------------------------
// AnalyzeWorkflow — line numbers
// ---------------------------------------------------------------------------

func TestAnalyzeWorkflow_LineNumbers(t *testing.T) {
	content := []byte(`on: [push]
permissions: write-all
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v5
  deploy:
    runs-on: ubuntu-latest
    timeout-minutes: 10
    steps:
      - run: echo hi
`)
	a := AnalyzeWorkflow("ci.yml", content)
	if a.PermissionsLine != 2 {
		t.Errorf("PermissionsLine = %d, want 2", a.PermissionsLine)
	}
	if a.WriteAllLine != 2 {
		t.Errorf("WriteAllLine = %d, want 2", a.WriteAllLine)
	}
	if a.JobsLine != 3 {
		t.Errorf("JobsLine = %d, want 3", a.JobsLine)
	}
	if len(a.Steps) != 2 || a.Steps[0].Line != 7 || a.Steps[1].Line != 8 {
		t.Errorf("step lines = %v, want [7 8]", []int{a.Steps[0].Line, a.Steps[1].Line})
	}
	if len(a.Jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(a.Jobs))
	}
	// sorted: build < deploy
	if a.Jobs[0].Name != "build" || a.Jobs[0].Line != 4 {
		t.Errorf("jobs[0] = %s@%d, want build@4", a.Jobs[0].Name, a.Jobs[0].Line)
	}
	if a.Jobs[1].Name != "deploy" || a.Jobs[1].Line != 9 {
		t.Errorf("jobs[1] = %s@%d, want deploy@9", a.Jobs[1].Name, a.Jobs[1].Line)
	}
}

func TestAnalyzeWorkflow_InvalidYAML(t *testing.T) {
	// Bad YAML — step extraction via regex still works; job YAML parse returns nil
	content := []byte(`{{{ not yaml`)
	a := AnalyzeWorkflow("bad.yml", content)
	if len(a.Jobs) != 0 {
		t.Errorf("invalid YAML should produce 0 jobs, got %d", len(a.Jobs))
	}
}
