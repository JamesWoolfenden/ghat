package core

import "testing"

func FuzzHasKubeResource(f *testing.F) {
	f.Add("apiVersion: apps/v1\nkind: Deployment\nspec: {}")
	f.Add("apiVersion: v1\nkind: Pod\nmetadata:\n  name: test")
	f.Add("---\napiVersion: v1\nkind: Service\n---\napiVersion: apps/v1\nkind: Deployment")
	f.Add("")
	f.Add("not yaml at all }{][")
	f.Fuzz(func(t *testing.T, data string) {
		hasKubeResource(data)
	})
}

func FuzzExtractKubeImages(f *testing.F) {
	f.Add("apiVersion: apps/v1\nkind: Deployment\nspec:\n  template:\n    spec:\n      containers:\n      - image: nginx:latest\n        name: app")
	f.Add("apiVersion: v1\nkind: Pod\nspec:\n  initContainers:\n  - image: busybox\n  containers:\n  - image: myapp:v1")
	f.Add("")
	f.Add("}{not yaml")
	f.Fuzz(func(t *testing.T, data string) {
		_, _ = extractKubeImages(data)
	})
}

func FuzzExtractGitLabImages(f *testing.F) {
	f.Add("image: golang:1.21\nbuild:\n  image: node:20\n  script:\n    - make")
	f.Add("image:\n  name: alpine:3\n  entrypoint: ['']")
	f.Add("")
	f.Add("image: $CI_REGISTRY_IMAGE:latest")
	f.Fuzz(func(t *testing.T, data string) {
		_, _ = extractImages(data)
	})
}

func FuzzExtractGHAContainerImages(f *testing.F) {
	f.Add("jobs:\n  build:\n    container:\n      image: ubuntu:22.04\n    services:\n      redis:\n        image: redis:7")
	f.Add("jobs:\n  test:\n    container: golang:1.21")
	f.Add("")
	f.Fuzz(func(t *testing.T, data string) {
		_, _ = extractGHAContainerImages(data)
	})
}

func FuzzRewritePreCommitRevs(f *testing.F) {
	f.Add("repos:\n- repo: https://github.com/pre-commit/pre-commit-hooks\n  rev: v4.4.0\n  hooks:\n  - id: trailing-whitespace")
	f.Add("repos:\n- repo: local\n  hooks:\n  - id: my-hook")
	f.Add("")
	f.Add("not yaml }{")
	f.Fuzz(func(t *testing.T, data string) {
		rewritePreCommitRevs(data, map[string]revPin{})
	})
}

func FuzzParseSuppression(f *testing.F) {
	f.Add("  uses: actions/checkout@v3 # ghat:suppress")
	f.Add("  image: nginx:latest # ghat:suppress:reason=intentionally unpinned")
	f.Add("  image: nginx:latest")
	f.Add("")
	f.Add("# ghat:suppress:reason=tag pinning is our design choice")
	f.Fuzz(func(t *testing.T, line string) {
		parseSuppression(line)
	})
}

func FuzzParsePinnedImages(f *testing.F) {
	f.Add("image: nginx@sha256:aaabbbccc  # nginx:1.25")
	f.Add("image: alpine:3.18@sha256:deadbeef")
	f.Add("")
	f.Fuzz(func(t *testing.T, data string) {
		parsePinnedImages(data)
	})
}

func FuzzParsePinnedRef(f *testing.F) {
	f.Add("abc123def456abc123def456abc123def456abc12 # v1.2.3")
	f.Add("v1.2.3")
	f.Add("")
	f.Add("not-a-sha # tag")
	f.Fuzz(func(t *testing.T, ref string) {
		parsePinnedRef(ref)
	})
}

func FuzzParseImageReference(f *testing.F) {
	f.Add("nginx:latest")
	f.Add("ghcr.io/owner/repo:v1.0@sha256:abc123")
	f.Add("ubuntu")
	f.Add("registry.example.com:5000/org/image:tag")
	f.Add("")
	f.Fuzz(func(t *testing.T, image string) {
		parseImageReference(image)
	})
}

func FuzzParseCpanfile(f *testing.F) {
	f.Add("requires 'Moose';\nrequires 'Try::Tiny', '0.30';\n")
	f.Add("requires 'strict';\nrequires 'warnings';")
	f.Add("")
	f.Add("not a cpanfile }{")
	f.Fuzz(func(t *testing.T, data string) {
		parseCpanfile(data)
	})
}

func FuzzHasVersionConstraint(f *testing.F) {
	f.Add("~> 1.0")
	f.Add(">= 1.0, < 2.0")
	f.Add("1.0.0")
	f.Add("= 3.5.0")
	f.Add("")
	f.Fuzz(func(t *testing.T, version string) {
		hasVersionConstraint(version)
	})
}

func FuzzParseLsRemoteTags(f *testing.F) {
	f.Add("abc123def456abc123def456abc123def456abc12\trefs/tags/v1.0.0")
	f.Add("abc123def456abc123def456abc123def456abc12\trefs/tags/v1.0.0^{}")
	f.Add("")
	f.Add("no-tab-here")
	f.Fuzz(func(t *testing.T, out string) {
		_, _, _ = parseLsRemoteTags(out)
	})
}

func FuzzParsePinnedFromLines(f *testing.F) {
	f.Add("FROM ubuntu:22.04@sha256:deadbeef1234 AS builder")
	f.Add("FROM golang:1.21\nFROM alpine:3.18@sha256:abc123")
	f.Add("")
	f.Fuzz(func(t *testing.T, data string) {
		parsePinnedFromLines(data)
	})
}
