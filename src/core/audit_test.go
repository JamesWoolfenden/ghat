package core

import (
	"reflect"
	"testing"
)

func TestFindUnpinned(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		total    int
		unpinned []string
	}{
		{
			name:  "pinned sha",
			body:  "      - uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v6.0.2",
			total: 1,
		},
		{
			name:     "tag",
			body:     "      - uses: actions/checkout@v4",
			total:    1,
			unpinned: []string{"actions/checkout@v4"},
		},
		{
			name:     "branch",
			body:     "      - uses: actions/checkout@main",
			total:    1,
			unpinned: []string{"actions/checkout@main"},
		},
		{
			name: "local path",
			body: "      - uses: ./.github/actions/build",
		},
		{
			name: "docker",
			body: "      - uses: docker://alpine:3.19",
		},
		{
			name:     "quoted",
			body:     `      - uses: "actions/setup-go@v5"`,
			total:    1,
			unpinned: []string{"actions/setup-go@v5"},
		},
		{
			name:     "mixed",
			body:     "  - uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd\n  - uses: actions/setup-go@v5\n  - uses: ./local",
			total:    2,
			unpinned: []string{"actions/setup-go@v5"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findUnpinned([]byte(tt.body))
			if got.total != tt.total {
				t.Errorf("total = %d, want %d", got.total, tt.total)
			}
			if !reflect.DeepEqual(got.unpinned, tt.unpinned) {
				t.Errorf("unpinned = %v, want %v", got.unpinned, tt.unpinned)
			}
		})
	}
}

func TestSplitGithubPath(t *testing.T) {
	tests := []struct {
		in    string
		owner string
		repo  string
		fail  bool
	}{
		{in: "rs/zerolog", owner: "rs", repo: "zerolog"},
		{in: "hashicorp/hcl/v2", owner: "hashicorp", repo: "hcl"},
		{in: "urfave/cli/v2", owner: "urfave", repo: "cli"},
		{in: "go-git/go-git/v5", owner: "go-git", repo: "go-git"},
		{in: "stretchr/testify/assert", owner: "stretchr", repo: "testify"},
		{in: "single", fail: true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			o, r, err := splitGithubPath(tt.in)
			if tt.fail {
				if err == nil {
					t.Fatalf("expected error, got %s/%s", o, r)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if o != tt.owner || r != tt.repo {
				t.Errorf("got %s/%s, want %s/%s", o, r, tt.owner, tt.repo)
			}
		})
	}
}

func TestGithubOwnerRepo(t *testing.T) {
	tests := []struct {
		in    string
		owner string
		repo  string
		ok    bool
	}{
		{"https://github.com/pre-commit/pre-commit-hooks", "pre-commit", "pre-commit-hooks", true},
		{"https://github.com/pre-commit/pre-commit-hooks.git", "pre-commit", "pre-commit-hooks", true},
		{"git@github.com:JamesWoolfenden/ghat.git", "JamesWoolfenden", "ghat", true},
		{"git::https://github.com/terraform-aws-modules/terraform-aws-vpc.git?ref=v5.0.0", "terraform-aws-modules", "terraform-aws-vpc", true},
		{"https://gitlab.com/foo/bar", "", "", false},
		{"hashicorp/aws/consul", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			o, r, ok := githubOwnerRepo(tt.in)
			if ok != tt.ok || o != tt.owner || r != tt.repo {
				t.Errorf("got (%s,%s,%v), want (%s,%s,%v)", o, r, ok, tt.owner, tt.repo, tt.ok)
			}
		})
	}
}

func TestResolveRepoGithub(t *testing.T) {
	o, r, err := resolveRepo("github.com/rs/zerolog")
	if err != nil || o != "rs" || r != "zerolog" {
		t.Errorf("github.com/rs/zerolog → %s/%s, %v", o, r, err)
	}
	o, r, err = resolveRepo("github.com/hashicorp/hcl/v2")
	if err != nil || o != "hashicorp" || r != "hcl" {
		t.Errorf("github.com/hashicorp/hcl/v2 → %s/%s, %v", o, r, err)
	}
}
