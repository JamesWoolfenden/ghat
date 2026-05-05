package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMatchesGlob(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		globs []string
		want  bool
	}{
		{"install.sh", []string{"*.sh"}, true},
		{"Makefile", []string{"Makefile"}, true},
		{"Dockerfile", []string{"Dockerfile*"}, true},
		{"Dockerfile.build", []string{"Dockerfile*"}, true},
		{"app.dockerfile", []string{"*.dockerfile"}, true},
		{"requirements.txt", []string{"requirements*.txt"}, true},
		{"main.go", []string{"*.sh", "Makefile"}, false},
		{"main.go", []string{}, false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := matchesGlob(tt.name, tt.globs); got != tt.want {
				t.Errorf("matchesGlob(%q, %v) = %v, want %v", tt.name, tt.globs, got, tt.want)
			}
		})
	}
}

func TestScanGaps(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	dockerfile := filepath.Join(dir, "Dockerfile")
	if err := os.WriteFile(dockerfile, []byte(`FROM alpine:3.18
RUN apt-get install curl=1.0.0
RUN pip install requests==2.28.0
RUN curl -L https://github.com/cli/cli/releases/download/v2.0.0/gh.tar.gz
`), 0644); err != nil {
		t.Fatal(err)
	}

	script := filepath.Join(dir, "install.sh")
	if err := os.WriteFile(script, []byte(`#!/bin/bash
wget https://github.com/some/tool/releases/download/v1.0/tool.tar.gz
go install github.com/some/tool@v1.2.3
`), 0644); err != nil {
		t.Fatal(err)
	}

	gaps := scanGaps(dir)

	if len(gaps) == 0 {
		t.Fatal("expected gaps to be detected, got none")
	}

	found := map[string]bool{}
	for _, g := range gaps {
		found[g] = true
	}

	checkGap := func(label string) {
		t.Helper()
		for g := range found {
			if len(g) > len(label) && g[:len(label)] == label {
				return
			}
		}
		t.Errorf("expected gap %q not found in: %v", label, gaps)
	}

	checkGap("apt-get install pinned")
	checkGap("pip install pinned")
	checkGap("curl release download")
	checkGap("wget release download")
	checkGap("go install @version")
}

func TestScanGaps_Clean(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM alpine:3.18\nRUN echo hello\n"), 0644); err != nil {
		t.Fatal(err)
	}

	gaps := scanGaps(dir)
	if len(gaps) != 0 {
		t.Errorf("expected no gaps, got %v", gaps)
	}
}
