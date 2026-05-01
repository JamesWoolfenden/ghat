package core

import (
	"os"
	"strings"
	"testing"

	"github.com/rs/zerolog/log"
)

func Test_isDockerfile(t *testing.T) {
	tests := []struct {
		file string
		want bool
	}{
		{"Dockerfile", true},
		{"Dockerfile.prod", true},
		{"Dockerfile.dev", true},
		{"app.dockerfile", true},
		{"main.go", false},
		{"docker-compose.yml", false},
		{"Makefile", false},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			if got := isDockerfile(tt.file); got != tt.want {
				t.Errorf("isDockerfile(%q) = %v, want %v", tt.file, got, tt.want)
			}
		})
	}
}

func Test_formatDockerImage(t *testing.T) {
	tests := []struct {
		name   string
		ref    ImageReference
		digest string
		want   string
	}{
		{
			name: "official image with tag",
			ref: ImageReference{
				Registry:   "docker.io",
				Repository: "library/golang",
				Tag:        "1.22-alpine",
			},
			digest: "sha256:abc123",
			want:   "golang:1.22-alpine@sha256:abc123",
		},
		{
			name: "official image latest tag omitted",
			ref: ImageReference{
				Registry:   "docker.io",
				Repository: "library/nginx",
				Tag:        "latest",
			},
			digest: "sha256:def456",
			want:   "nginx@sha256:def456",
		},
		{
			name: "user image",
			ref: ImageReference{
				Registry:   "docker.io",
				Repository: "jameswoolfenden/ghat",
				Tag:        "0.1.0",
			},
			digest: "sha256:xyz789",
			want:   "jameswoolfenden/ghat:0.1.0@sha256:xyz789",
		},
		{
			name: "custom registry",
			ref: ImageReference{
				Registry:   "gcr.io",
				Repository: "project/image",
				Tag:        "v1.0",
			},
			digest: "sha256:aaa111",
			want:   "gcr.io/project/image:v1.0@sha256:aaa111",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDockerImage(tt.ref, tt.digest)
			if got != tt.want {
				t.Errorf("formatDockerImage() = %q, want %q", got, tt.want)
			}
		})
	}
}

func Test_parsePinnedFromLines(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    map[string]string
	}{
		{
			name:    "no pinned lines",
			content: "FROM golang:1.22-alpine\n",
			want:    map[string]string{},
		},
		{
			name:    "single pinned line",
			content: "FROM golang:1.22-alpine@sha256:abcdef1234567890abcdef1234567890abcdef1234 AS builder\n",
			want:    map[string]string{"1.22-alpine": "sha256:abcdef1234567890abcdef1234567890abcdef1234"},
		},
		{
			name:    "multiple pinned lines",
			content: "FROM golang:1.22@sha256:aaaa AS build\nFROM nginx:1.25@sha256:bbbb\n",
			want:    map[string]string{"1.22": "sha256:aaaa", "1.25": "sha256:bbbb"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parsePinnedFromLines(tt.content)
			if len(got) != len(tt.want) {
				t.Errorf("parsePinnedFromLines() len=%d, want %d. got=%v", len(got), len(tt.want), got)
				return
			}
			for tag, digest := range tt.want {
				if got[tag] != digest {
					t.Errorf("parsePinnedFromLines()[%q] = %q, want %q", tag, got[tag], digest)
				}
			}
		})
	}
}

func TestUpdateDockerfile_DryRun(t *testing.T) {
	content := "FROM golang:1.22-alpine AS builder\nFROM nginx:1.25\n"
	tmp, err := os.CreateTemp("", "Dockerfile.*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()

	if _, err := tmp.WriteString(content); err != nil {
		t.Fatal(err)
	}
	_ = tmp.Close()

	myFlags := &Flags{DryRun: true}
	// Without a real registry, getImageDigest will fail, but we just verify no write happens.
	_ = myFlags.UpdateDockerfile(tmp.Name())

	got, err := os.ReadFile(tmp.Name())
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != content {
		t.Error("DryRun should not modify the file")
	}
}

func TestUpdateDockerfile_SkipsScratchAndVars(t *testing.T) {
	content := "FROM scratch\nFROM $BASE_IMAGE\n"
	tmp, err := os.CreateTemp("", "Dockerfile.*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()

	if _, err := tmp.WriteString(content); err != nil {
		t.Fatal(err)
	}
	_ = tmp.Close()

	myFlags := &Flags{DryRun: false}
	if err := myFlags.UpdateDockerfile(tmp.Name()); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(tmp.Name())
	if err != nil {
		t.Fatal(err)
	}
	// scratch and vars must be untouched
	if !strings.Contains(string(got), "FROM scratch") {
		t.Error("FROM scratch should be preserved")
	}
	if !strings.Contains(string(got), "FROM $BASE_IMAGE") {
		t.Error("FROM $BASE_IMAGE should be preserved")
	}
}

func TestUpdateDockerfile_DynamicImageWarning(t *testing.T) {
	var buf strings.Builder
	original := log.Logger
	log.Logger = log.Output(&buf)
	t.Cleanup(func() { log.Logger = original })

	// Both start with $ so the image name itself is dynamic — SUPPLY CHAIN RISK in both cases.
	// The "unexpanded variable tag" info path applies to golang:${TAG} style where
	// the image name is known but the tag is a variable.
	content := "FROM $BASE_IMAGE\nFROM ${VERSION}-alpine\n"
	tmp, err := os.CreateTemp("", "Dockerfile.*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()

	if _, err := tmp.WriteString(content); err != nil {
		t.Fatal(err)
	}
	_ = tmp.Close()

	if err := (&Flags{DryRun: true}).UpdateDockerfile(tmp.Name()); err != nil {
		t.Fatalf("UpdateDockerfile() unexpected error: %v", err)
	}

	output := buf.String()
	for _, ref := range []string{"$BASE_IMAGE", "${VERSION}-alpine"} {
		if !strings.Contains(output, "SUPPLY CHAIN RISK") || !strings.Contains(output, ref) {
			t.Errorf("expected SUPPLY CHAIN RISK warning for %s, got: %s", ref, output)
		}
	}

	got, err := os.ReadFile(tmp.Name())
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != content {
		t.Error("dynamic FROM lines should not be modified")
	}
}

func TestUpdateDockerfile_UnexpandedVariableTag(t *testing.T) {
	var buf strings.Builder
	original := log.Logger
	log.Logger = log.Output(&buf)
	t.Cleanup(func() { log.Logger = original })

	// Image name is known; only the tag is a variable with no ARG default → info, not SUPPLY CHAIN RISK.
	content := "FROM golang:${GOLANG_VER}\n"
	tmp, err := os.CreateTemp("", "Dockerfile.*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.WriteString(content); err != nil {
		t.Fatal(err)
	}
	_ = tmp.Close()

	if err := (&Flags{DryRun: true}).UpdateDockerfile(tmp.Name()); err != nil {
		t.Fatalf("UpdateDockerfile() unexpected error: %v", err)
	}

	output := buf.String()
	if strings.Contains(output, "SUPPLY CHAIN RISK") {
		t.Errorf("golang:${TAG} should not trigger SUPPLY CHAIN RISK, got: %s", output)
	}
	if !strings.Contains(output, "unexpanded variable tag") {
		t.Errorf("expected unexpanded variable tag info, got: %s", output)
	}
}

func Test_parseArgDefaults(t *testing.T) {
	lines := []string{
		"ARG GOLANG_VER=1.22-alpine",
		"ARG ALPINE_VER=3.19",
		"ARG NO_DEFAULT",
		"ARG EMPTY=",
		"FROM golang:${GOLANG_VER}",
	}
	got := parseArgDefaults(lines)
	want := map[string]string{
		"GOLANG_VER": "1.22-alpine",
		"ALPINE_VER": "3.19",
	}
	if len(got) != len(want) {
		t.Fatalf("parseArgDefaults() = %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("parseArgDefaults()[%q] = %q, want %q", k, got[k], v)
		}
	}
	if _, ok := got["NO_DEFAULT"]; ok {
		t.Error("ARG without default should not appear in map")
	}
}

func Test_expandDockerVars(t *testing.T) {
	vars := map[string]string{"TAG": "1.22", "REG": "gcr.io"}
	tests := []struct {
		in   string
		want string
	}{
		{"golang:${TAG}", "golang:1.22"},
		{"${REG}/app:latest", "gcr.io/app:latest"},
		{"golang:${MISSING}", "golang:${MISSING}"},
		{"golang:${MISSING:-3.19}", "golang:3.19"},
		{"no-vars:latest", "no-vars:latest"},
	}
	for _, tt := range tests {
		got := expandDockerVars(tt.in, vars)
		if got != tt.want {
			t.Errorf("expandDockerVars(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestGetDockerfiles(t *testing.T) {
	myFlags := &Flags{
		Entries: []string{
			"testdata/docker/Dockerfile",
			"testdata/docker/Dockerfile.prod",
			"testdata/compose/docker-compose.yaml",
			"src/core/gitlab.go",
		},
	}
	got := myFlags.GetDockerfiles()
	if len(got) != 2 {
		t.Errorf("GetDockerfiles() = %d files, want 2: %v", len(got), got)
	}
}
