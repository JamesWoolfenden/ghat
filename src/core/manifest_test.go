package core

import (
	"testing"
)

func TestParseManifestGHA(t *testing.T) {
	content := []byte(`on: push
jobs:
  build:
    steps:
      - uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v4
      - uses: actions/setup-go@v5
      - uses: ./local
      - uses: docker://alpine:3.19
`)
	refs := ParseManifest(ManifestGHA, content)
	if len(refs) != 2 {
		t.Fatalf("got %d refs, want 2: %+v", len(refs), refs)
	}
	if refs[0].Name != "actions/checkout" || refs[0].Ecosystem != SourceGHA {
		t.Errorf("ref[0] = %+v", refs[0])
	}
	if refs[1].Name != "actions/setup-go" || refs[1].Version != "v5" {
		t.Errorf("ref[1] = %+v", refs[1])
	}
	if refs[0].Line == 0 || refs[1].Line == 0 {
		t.Error("line numbers must be non-zero")
	}
}

func TestParseManifestGoMod(t *testing.T) {
	content := []byte(`module example.com/mymod

go 1.21

require (
	github.com/rs/zerolog v1.33.0
	github.com/urfave/cli/v2 v2.27.0
)

require github.com/mattn/go-isatty v0.0.20 // indirect
`)
	refs := ParseManifest(ManifestGoMod, content)
	if len(refs) != 2 {
		t.Fatalf("got %d direct refs, want 2: %+v", len(refs), refs)
	}
	names := map[string]bool{}
	for _, r := range refs {
		names[r.Name] = true
		if r.Ecosystem != SourceGo {
			t.Errorf("ecosystem = %s, want %s", r.Ecosystem, SourceGo)
		}
		if r.Line == 0 {
			t.Errorf("%s has line 0", r.Name)
		}
	}
	if !names["github.com/rs/zerolog"] {
		t.Error("missing github.com/rs/zerolog")
	}
}

func TestParseManifestNPM(t *testing.T) {
	content := []byte(`{
  "name": "myapp",
  "dependencies": {
    "lodash": "^4.17.21",
    "express": "^4.18.0"
  },
  "devDependencies": {
    "jest": "^29.0.0"
  }
}`)
	refs := ParseManifest(ManifestNPM, content)
	if len(refs) != 3 {
		t.Fatalf("got %d refs, want 3: %+v", len(refs), refs)
	}
	for _, r := range refs {
		if r.Ecosystem != SourceNpm {
			t.Errorf("ecosystem = %s, want %s", r.Ecosystem, SourceNpm)
		}
	}
}

func TestParseManifestPyPI(t *testing.T) {
	content := []byte(`# requirements
requests==2.31.0
click>=8.0
# commented out
#nope
-r other.txt
Django>=4.2,<5
`)
	refs := ParseManifest(ManifestPyPI, content)
	if len(refs) != 3 {
		t.Fatalf("got %d refs, want 3: %+v", len(refs), refs)
	}
	names := map[string]bool{}
	for _, r := range refs {
		names[r.Name] = true
		if r.Ecosystem != SourcePypi {
			t.Errorf("ecosystem = %s", r.Ecosystem)
		}
		if r.Line == 0 {
			t.Errorf("%s has line 0", r.Name)
		}
	}
	for _, want := range []string{"requests", "click", "Django"} {
		if !names[want] {
			t.Errorf("missing %s", want)
		}
	}
}

func TestParseManifestCargo(t *testing.T) {
	content := []byte(`[package]
name = "myapp"

[dependencies]
serde = "1.0"
tokio = { version = "1", features = ["full"] }

[dev-dependencies]
mockito = "1"
`)
	refs := ParseManifest(ManifestCargo, content)
	if len(refs) != 3 {
		t.Fatalf("got %d refs, want 3: %+v", len(refs), refs)
	}
	names := map[string]bool{}
	for _, r := range refs {
		names[r.Name] = true
		if r.Ecosystem != SourceCargo {
			t.Errorf("ecosystem = %s", r.Ecosystem)
		}
	}
	for _, want := range []string{"serde", "tokio", "mockito"} {
		if !names[want] {
			t.Errorf("missing %s", want)
		}
	}
}

func TestParseManifestGem(t *testing.T) {
	content := []byte(`source 'https://rubygems.org'
gem 'rails', '~> 7.0'
gem "rspec-core"
gem 'pg', platform: :ruby
# gem 'nope'
`)
	refs := ParseManifest(ManifestGem, content)
	if len(refs) != 3 {
		t.Fatalf("got %d refs, want 3: %+v", len(refs), refs)
	}
	for _, r := range refs {
		if r.Ecosystem != SourceGem {
			t.Errorf("ecosystem = %s", r.Ecosystem)
		}
		if r.Line == 0 {
			t.Errorf("%s has line 0", r.Name)
		}
	}
}

func TestParseManifestPreCommit(t *testing.T) {
	content := []byte(`repos:
  - repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v4.4.0
    hooks:
      - id: trailing-whitespace
  - repo: https://github.com/psf/black
    rev: 23.1.0
    hooks:
      - id: black
  - repo: local
    hooks:
      - id: mycheck
`)
	refs := ParseManifest(ManifestPreCommit, content)
	if len(refs) != 2 {
		t.Fatalf("got %d refs, want 2: %+v", len(refs), refs)
	}
	if refs[0].Name != "https://github.com/pre-commit/pre-commit-hooks" {
		t.Errorf("ref[0].Name = %s", refs[0].Name)
	}
	if refs[0].Version != "v4.4.0" {
		t.Errorf("ref[0].Version = %s", refs[0].Version)
	}
	if refs[0].Line == 0 || refs[1].Line == 0 {
		t.Error("line numbers must be non-zero")
	}
}

func TestParseManifestKube(t *testing.T) {
	content := []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
spec:
  template:
    spec:
      containers:
        - name: app
          image: nginx:1.25
        - name: pinned
          image: alpine:3.18@sha256:c0d488a800e4127b334ad4d82bd32ad5c65f5d81ebae3c59b01d36e0b6d44bf7
`)
	refs := ParseManifest(ManifestKube, content)
	if len(refs) != 2 {
		t.Fatalf("got %d refs, want 2: %+v", len(refs), refs)
	}
	names := map[string]string{}
	for _, r := range refs {
		if r.Ecosystem != SourceKube {
			t.Errorf("%s: ecosystem = %s, want %s", r.Name, r.Ecosystem, SourceKube)
		}
		names[r.Name] = r.Version
	}
	if names["nginx"] != "1.25" {
		t.Errorf("nginx version = %q, want 1.25", names["nginx"])
	}
	// alpine:3.18@sha256:... → name="alpine:3.18", version="sha256:..."
	if names["alpine:3.18"] != "sha256:c0d488a800e4127b334ad4d82bd32ad5c65f5d81ebae3c59b01d36e0b6d44bf7" {
		t.Errorf("alpine:3.18 version = %q", names["alpine:3.18"])
	}
}

func TestParseManifestKubeNonKube(t *testing.T) {
	content := []byte(`key: value
other: stuff
`)
	if refs := ParseManifest(ManifestKube, content); refs != nil {
		t.Errorf("expected nil for non-k8s YAML, got %v", refs)
	}
}

func TestParseManifestCompose(t *testing.T) {
	content := []byte(`services:
  web:
    image: nginx:1.25
  db:
    image: postgres:15
`)
	refs := ParseManifest(ManifestCompose, content)
	if len(refs) != 2 {
		t.Fatalf("got %d refs, want 2: %+v", len(refs), refs)
	}
	for _, r := range refs {
		if r.Ecosystem != SourceCompose {
			t.Errorf("%s: ecosystem = %s, want %s", r.Name, r.Ecosystem, SourceCompose)
		}
		if r.Line == 0 {
			t.Errorf("%s has line 0", r.Name)
		}
	}
}

func TestParseManifestTerraform(t *testing.T) {
	content := []byte(`terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "5.1.2"
}
`)
	refs := ParseManifest(ManifestTerraform, content)
	if len(refs) != 2 {
		t.Fatalf("got %d refs, want 2: %+v", len(refs), refs)
	}
	names := map[string]string{}
	for _, r := range refs {
		if r.Ecosystem != SourceTerraform {
			t.Errorf("%s: ecosystem = %s, want %s", r.Name, r.Ecosystem, SourceTerraform)
		}
		names[r.Name] = r.Version
	}
	if names["hashicorp/aws"] != "~> 5.0" {
		t.Errorf("hashicorp/aws version = %q, want ~> 5.0", names["hashicorp/aws"])
	}
	if names["terraform-aws-modules/vpc/aws"] != "5.1.2" {
		t.Errorf("module version = %q, want 5.1.2", names["terraform-aws-modules/vpc/aws"])
	}
}

func TestParseManifestUnknown(t *testing.T) {
	if refs := ParseManifest(ManifestKind(99), []byte("anything")); refs != nil {
		t.Errorf("expected nil for unknown kind, got %v", refs)
	}
}
