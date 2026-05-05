package core

import (
	"reflect"
	"strings"
	"testing"
)

func TestPyprojectDeps(t *testing.T) {
	body := `[build-system]
requires = ["setuptools"]

[project]
name = "x"
dependencies = [
  "requests>=2.0",
  "click",
]

[tool.other]
dependencies = ["ignored"]
`
	got := pyprojectDeps(body)
	if !reflect.DeepEqual(got, []string{"requests>=2.0", "click"}) {
		t.Fatalf("got %v", got)
	}
	if pyprojectDeps("[tool.poetry]\nname='x'") != nil {
		t.Fatal("expected nil for no [project]")
	}
}

func TestPypiNameRe(t *testing.T) {
	tests := map[string]string{
		"requests==2.31.0":     "requests",
		"Django>=4.2,<5":       "Django",
		"my-pkg[extra]>=1":     "my-pkg",
		"  numpy  # comment":   "numpy",
		"foo.bar-baz_qux==1.0": "foo.bar-baz_qux",
	}
	for in, want := range tests {
		m := pypiNameRe.FindStringSubmatch(in)
		if m == nil || m[1] != want {
			t.Errorf("%q → %v, want %q", in, m, want)
		}
	}
}

func TestCargoSectionParse(t *testing.T) {
	body := `[package]
name = "x"

[dependencies]
serde = "1.0"
tokio = { version = "1", features = ["full"] }
my-crate = "0.1"

[dev-dependencies]
mockito = "1"
`
	var got []string
	in := false
	for _, line := range strings.Split(body, "\n") {
		tr := strings.TrimSpace(line)
		if strings.HasPrefix(tr, "[") {
			in = tr == "[dependencies]" || strings.HasSuffix(tr, "dependencies]")
			continue
		}
		if in {
			if m := cargoDepRe.FindStringSubmatch(line); m != nil {
				got = append(got, m[1])
			}
		}
	}
	if !reflect.DeepEqual(got, []string{"serde", "tokio", "my-crate", "mockito"}) {
		t.Fatalf("got %v", got)
	}
}

func TestGemRe(t *testing.T) {
	tests := map[string]string{
		`gem 'rails', '~> 7.0'`:     "rails",
		`  gem "rspec-core"`:        "rspec-core",
		`gem 'pg', platform: :ruby`: "pg",
		`# gem 'nope'`:              "",
		`source 'https://rubygems'`: "",
	}
	for in, want := range tests {
		var got string
		if m := gemRe.FindStringSubmatch(in); m != nil {
			got = m[1]
		}
		if got != want {
			t.Errorf("%q → %q, want %q", in, got, want)
		}
	}
}

func TestResolvePackageRepo(t *testing.T) {
	if testing.Short() {
		t.Skip("network")
	}
	tests := []struct {
		eco, pkg, owner string
	}{
		{SourceNpm, "lodash", "lodash"},
		{SourcePypi, "requests", "psf"},
		{SourceCargo, "serde", "serde-rs"},
		{SourceGem, "rake", "ruby"},
	}
	for _, tt := range tests {
		t.Run(tt.eco+"/"+tt.pkg, func(t *testing.T) {
			owner, repo, err := resolvePackageRepo(tt.eco, tt.pkg)
			if err != nil {
				t.Fatalf("err = %v", err)
			}
			if owner != tt.owner {
				t.Errorf("owner = %s/%s, want %s/*", owner, repo, tt.owner)
			}
		})
	}
}
