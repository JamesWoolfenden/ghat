package lsp

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func codes(ds []diagnostic) map[string]int {
	m := map[string]int{}
	for _, d := range ds {
		m[d.Code]++
	}
	return m
}

func TestClassify(t *testing.T) {
	cases := map[string]fileKind{
		"/r/.github/workflows/ci.yml": kindGHA,
		"/r/.github/workflows/x.yaml": kindGHA,
		"/r/sub/action.yml":           kindGHA,
		"/r/.gitlab-ci.yml":           kindGitlab,
		"/r/.pre-commit-config.yaml":  kindPreCommit,
		"/r/Dockerfile":               kindDockerfile,
		"/r/Dockerfile.dev":           kindDockerfile,
		"/r/service/app.dockerfile":   kindDockerfile,
		"/r/Containerfile":            kindDockerfile,
		"/r/random.yaml":              kindUnknown,
	}
	for path, want := range cases {
		if got := classify(path); got != want {
			t.Errorf("classify(%q) = %d, want %d", path, got, want)
		}
	}
}

func TestDiagnostics_GHA(t *testing.T) {
	content := []byte(`on: [push]
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v5
`)
	s := New("")
	ds := s.analyze("file:///r/.github/workflows/ci.yml", content)
	got := codes(ds)
	if got["ghat-pin"] != 1 {
		t.Errorf("ghat-pin count = %d, want 1 (only checkout@v4)", got["ghat-pin"])
	}
	if got["ghat-permissions"] != 1 {
		t.Errorf("ghat-permissions count = %d, want 1", got["ghat-permissions"])
	}
	if got["ghat-timeout"] != 1 {
		t.Errorf("ghat-timeout count = %d, want 1", got["ghat-timeout"])
	}
	if got["ghat-concurrency"] != 1 {
		t.Errorf("ghat-concurrency count = %d, want 1", got["ghat-concurrency"])
	}
	// pin diagnostic must land on line 6 (LSP 0-indexed → 5) and carry data.
	for _, d := range ds {
		if d.Code == "ghat-pin" {
			if d.Range.Start.Line != 5 {
				t.Errorf("ghat-pin line = %d, want 5", d.Range.Start.Line)
			}
			var pd pinData
			if json.Unmarshal(d.Data, &pd) != nil || pd.Action != "actions/checkout" || pd.Tag != "v4" {
				t.Errorf("ghat-pin data = %+v, want action=actions/checkout tag=v4", pd)
			}
		}
	}
}

func TestDiagnostics_Unknown(t *testing.T) {
	s := New("")
	if ds := s.analyze("file:///r/values.yaml", []byte("foo: bar")); len(ds) != 0 {
		t.Errorf("unknown file kind should yield 0 diagnostics, got %d", len(ds))
	}
}

func TestDiagnostics_PreCommit(t *testing.T) {
	content := []byte(`repos:
  - repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v4.5.0
`)
	s := New("")
	ds := s.analyze("file:///r/.pre-commit-config.yaml", content)
	if codes(ds)["ghat-pin"] != 1 {
		t.Fatalf("want 1 ghat-pin diagnostic, got %v", codes(ds))
	}
	if ds[0].Range.Start.Line != 2 {
		t.Errorf("rev line = %d, want 2 (0-indexed)", ds[0].Range.Start.Line)
	}
}

func TestDiagnostics_Dockerfile(t *testing.T) {
	content := []byte("FROM golang:1.21\nFROM alpine@sha256:6706c73aae2afaa8201d63cc3dda48753c09bcd6c300762251065c0f7e602b25\n")
	s := New("")
	ds := s.analyze("file:///r/Dockerfile", content)
	if codes(ds)["ghat-image-pin"] != 1 {
		t.Fatalf("want 1 ghat-image-pin, got %v", codes(ds))
	}
}

func TestImageSpan(t *testing.T) {
	df := []byte("FROM --platform=linux/amd64 golang:1.21 AS build\n")
	s, e := imageSpan(df, 1, "golang:1.21")
	if got := string(df[s:e]); got != "golang:1.21" {
		t.Errorf("imageSpan dockerfile = %q [%d:%d], want golang:1.21", got, s, e)
	}
	gl := []byte("  image: gcr.io/distroless/static:nonroot\n")
	s, e = imageSpan(gl, 1, "gcr.io/distroless/static:nonroot")
	if got := string(gl[s:e]); got != "gcr.io/distroless/static:nonroot" {
		t.Errorf("imageSpan gitlab = %q", got)
	}
}

func TestDiagnostics_Dockerfile_CarriesResolvedImage(t *testing.T) {
	content := []byte("ARG GO=1.21\nFROM golang:${GO}\n")
	ds := New("").analyze("file:///r/Dockerfile", content)
	if len(ds) != 1 {
		t.Fatalf("want 1 diagnostic, got %d", len(ds))
	}
	var pd pinData
	_ = json.Unmarshal(ds[0].Data, &pd)
	if pd.Image != "golang:1.21" || !pd.Dockerfile {
		t.Errorf("data = %+v, want resolved image golang:1.21 dockerfile=true", pd)
	}
}

func TestRefSpan(t *testing.T) {
	content := []byte("      - uses: actions/checkout@v4\n")
	s, e := refSpan(content, 1)
	if got := string(content[s:e]); got != "v4" {
		t.Errorf("refSpan uses: = %q [%d:%d], want v4", got, s, e)
	}
	pc := []byte("    rev: v4.5.0\n")
	s, e = refSpan(pc, 1)
	if got := string(pc[s:e]); got != "v4.5.0" {
		t.Errorf("refSpan rev: = %q, want v4.5.0", got)
	}
}

func TestCodeAction_Suppress(t *testing.T) {
	s := New("")
	uri := "file:///r/.github/workflows/ci.yml"
	content := []byte(`on: [push]
permissions: write-all
jobs:
  build:
    runs-on: ubuntu-latest
    timeout-minutes: 5
    steps:
      - uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v4
`)
	s.docs[uri] = content
	d := mk(2, "ghat-write-all", 1, "permissions: write-all", pinData{})
	actions := s.actionsFor(uri, content, d)
	if len(actions) != 1 || !strings.Contains(actions[0].Title, "Suppress") {
		t.Fatalf("expected one Suppress action, got %+v", actions)
	}
	te := actions[0].Edit["changes"].(map[string][]textEdit)[uri][0]
	if te.NewText != "  # ghat:suppress" {
		t.Errorf("suppress NewText = %q", te.NewText)
	}
	if te.Range.Start.Line != 1 || te.Range.Start.Character != uint32(len("permissions: write-all")) {
		t.Errorf("suppress insert pos = %+v, want end of line 1", te.Range.Start)
	}
}

func TestCodeAction_InsertPermissions(t *testing.T) {
	s := New("")
	uri := "file:///r/.github/workflows/ci.yml"
	content := []byte("on: [push]\njobs:\n  build:\n    runs-on: ubuntu-latest\n")
	d := mk(1, "ghat-permissions", 2, "no permissions", pinData{JobsLine: 2})
	actions := s.actionsFor(uri, content, d)
	if len(actions) == 0 {
		t.Fatal("expected at least one action")
	}
	te := actions[0].Edit["changes"].(map[string][]textEdit)[uri][0]
	if te.Range.Start.Line != 1 || !strings.HasPrefix(te.NewText, "permissions:") {
		t.Errorf("permissions edit = %+v", te)
	}
}

func TestInitialize(t *testing.T) {
	var buf bytes.Buffer
	s := New("")
	s.out = &buf
	id := json.RawMessage(`1`)
	s.dispatch(rpcRequest{JSONRPC: "2.0", ID: &id, Method: "initialize"})
	out := buf.String()
	if !strings.Contains(out, `"textDocumentSync":1`) || !strings.Contains(out, `"name":"ghat"`) {
		t.Errorf("initialize response missing capabilities: %s", out)
	}
}
