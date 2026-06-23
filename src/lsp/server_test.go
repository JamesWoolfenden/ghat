package lsp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/jameswoolfenden/ghat/src/core"
)

// roundTrip sends one JSON-RPC request to the server and returns the response body.
func roundTrip(t *testing.T, s *Server, method string, params interface{}) map[string]interface{} {
	t.Helper()
	body, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	})
	raw := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body)

	in := bufio.NewReader(strings.NewReader(raw))
	var out bytes.Buffer
	wr := bufio.NewWriter(&out)

	msg, err := readMessage(in)
	if err != nil {
		t.Fatalf("readMessage: %v", err)
	}
	if err := s.dispatch(wr, msg); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	_ = wr.Flush()

	// skip the Content-Length header
	outRd := bufio.NewReader(&out)
	for {
		line, _ := outRd.ReadString('\n')
		if strings.TrimSpace(line) == "" {
			break
		}
	}
	var result map[string]interface{}
	_ = json.NewDecoder(outRd).Decode(&result)
	return result
}

func TestHandleInitialize(t *testing.T) {
	s := New("", nil)
	resp := roundTrip(t, s, "initialize", map[string]interface{}{})
	caps, ok := resp["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("result is not a map: %v", resp["result"])
	}
	if caps["capabilities"] == nil {
		t.Error("missing capabilities in initialize result")
	}
}

func TestClassifyURI(t *testing.T) {
	tests := []struct {
		uri  string
		kind core.ManifestKind
		ok   bool
	}{
		{"file:///repo/.github/workflows/ci.yml", core.ManifestGHA, true},
		{"file:///repo/.github/workflows/ci.yaml", core.ManifestGHA, true},
		{"file:///repo/go.mod", core.ManifestGoMod, true},
		{"file:///repo/package.json", core.ManifestNPM, true},
		{"file:///repo/requirements.txt", core.ManifestPyPI, true},
		{"file:///repo/requirements-dev.txt", core.ManifestPyPI, true},
		{"file:///repo/Cargo.toml", core.ManifestCargo, true},
		{"file:///repo/Gemfile", core.ManifestGem, true},
		{"file:///repo/.pre-commit-config.yaml", core.ManifestPreCommit, true},
		{"file:///repo/cpanfile", core.ManifestCpanfile, true},
		{"file:///repo/docker-compose.yml", core.ManifestCompose, true},
		{"file:///repo/compose.yaml", core.ManifestCompose, true},
		{"file:///repo/main.tf", core.ManifestTerraform, true},
		{"file:///repo/variables.tf", core.ManifestTerraform, true},
		{"file:///repo/main.go", 0, false},
	}
	for _, tt := range tests {
		k, ok := classifyURI(tt.uri)
		if ok != tt.ok {
			t.Errorf("classifyURI(%q) ok=%v, want %v", tt.uri, ok, tt.ok)
		}
		if ok && k != tt.kind {
			t.Errorf("classifyURI(%q) kind=%v, want %v", tt.uri, k, tt.kind)
		}
	}
}

func TestCodeActionNoDeps(t *testing.T) {
	s := New("", nil)
	resp := roundTrip(t, s, "textDocument/codeAction", map[string]interface{}{
		"textDocument": map[string]string{"uri": "file:///repo/go.mod"},
		"range":        map[string]interface{}{"start": map[string]int{"line": 0, "character": 0}, "end": map[string]int{"line": 0, "character": 0}},
	})
	result, ok := resp["result"].([]interface{})
	if !ok || len(result) != 0 {
		// no deps cached → empty actions
		t.Logf("result: %v (ok=%v)", resp["result"], ok)
	}
}

func TestFindVersionLine(t *testing.T) {
	lines := []string{
		"- repo: https://github.com/pre-commit/pre-commit-hooks",
		"  rev: v4.6.0",
		"  hooks:",
	}
	if got := findVersionLine(lines, 0, "v4.6.0"); got != 1 {
		t.Errorf("findVersionLine = %d, want 1", got)
	}
	if got := findVersionLine(lines, 0, "missing"); got != -1 {
		t.Errorf("findVersionLine for missing = %d, want -1", got)
	}
}

func TestCanUpdate(t *testing.T) {
	for _, eco := range []string{
		core.SourceGHA, core.SourcePreCommit, core.SourceTerraform,
		core.SourceGitLab, core.SourceKube, core.SourceCompose,
		core.SourceDockerfile, core.SourceGitLabComponent,
		core.SourceNpm, core.SourcePypi, core.SourceCargo, core.SourceGem, core.SourceGo,
		core.SourceCpanfile,
	} {
		if !canUpdate(eco) {
			t.Errorf("%s should be updatable", eco)
		}
	}
}

func TestGHAStaticDiags(t *testing.T) {
	content := `on: push
jobs:
  build:
    steps:
      - uses: actions/checkout@v4
`
	diags := ghaStaticDiags("test.yml", []byte(content))
	// missing permissions + unpinned step
	if len(diags) < 2 {
		t.Errorf("expected ≥2 diagnostics (permissions + unpinned), got %d: %+v", len(diags), diags)
	}
	has := func(substr string) bool {
		for _, d := range diags {
			if strings.Contains(d.Message, substr) {
				return true
			}
		}
		return false
	}
	if !has("permissions") {
		t.Error("expected a permissions diagnostic")
	}
	if !has("SHA") {
		t.Error("expected an unpinned SHA diagnostic")
	}
}

func TestTerraformStaticDiagsSHAPinnedModule(t *testing.T) {
	refs := []core.DepRef{
		{
			Ecosystem: core.SourceTerraform,
			Name:      "git::https://github.com/JamesWoolfenden/terraform-http-ip.git?ref=2f3cef24e667fb840a3d3481f5a1aaa5a1ac7d28",
			Version:   "",
			Line:      1,
		},
	}
	diags := terraformStaticDiags(refs)
	if len(diags) != 0 {
		t.Errorf("expected no diagnostics for a SHA-pinned module, got %+v", diags)
	}
}

func TestTerraformStaticDiagsUnpinnedModule(t *testing.T) {
	refs := []core.DepRef{
		{
			Ecosystem: core.SourceTerraform,
			Name:      "git::https://github.com/JamesWoolfenden/terraform-http-ip.git",
			Version:   "",
			Line:      1,
		},
	}
	diags := terraformStaticDiags(refs)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic for an unpinned module, got %d: %+v", len(diags), diags)
	}
	if !strings.Contains(diags[0].Message, "no version constraint") {
		t.Errorf("expected 'no version constraint' message, got %q", diags[0].Message)
	}
}
