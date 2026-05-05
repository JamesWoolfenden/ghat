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

func TestCheckPermissions(t *testing.T) {
	tests := []struct {
		name string
		wf   map[string][]byte
		want checkOutcome
	}{
		{"none", nil, checkSkip},
		{"set", map[string][]byte{"a.yml": []byte("permissions:\n  contents: read\n")}, checkPass},
		{"missing", map[string][]byte{"a.yml": []byte("on: push\njobs:\n")}, checkFail},
		{"mixed", map[string][]byte{
			"a.yml": []byte("permissions: {}\n"),
			"b.yml": []byte("on: push\n"),
		}, checkFail},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checkPermissions(tt.wf); got.outcome != tt.want {
				t.Errorf("outcome = %v, want %v (%s)", got.outcome, tt.want, got.detail)
			}
		})
	}
}

func TestCheckDangerousTrigger(t *testing.T) {
	prtCheckout := `on:
  pull_request_target:
jobs:
  x:
    steps:
      - uses: actions/checkout@v4
        with:
          ref: ${{ github.event.pull_request.head.sha }}
`
	tests := []struct {
		name string
		wf   map[string][]byte
		want checkOutcome
	}{
		{"none", nil, checkSkip},
		{"clean", map[string][]byte{"a.yml": []byte("on: push\njobs:\n")}, checkPass},
		{"prt-alone", map[string][]byte{"a.yml": []byte("on:\n  pull_request_target:\njobs:\n")}, checkPass},
		{"prt+checkout", map[string][]byte{"a.yml": []byte(prtCheckout)}, checkFail},
		{"run-inject", map[string][]byte{"a.yml": []byte("    run: echo ${{ github.event.issue.title }}\n")}, checkFail},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checkDangerousTrigger(tt.wf); got.outcome != tt.want {
				t.Errorf("outcome = %v, want %v (%s)", got.outcome, tt.want, got.detail)
			}
		})
	}
}

func TestBucketAndScore(t *testing.T) {
	cs := []checkResult{
		{"signed-pin", checkSkip, ""},
		{"ci-pinned", checkPass, ""},
		{"maintained", checkFail, ""},
	}
	if b := bucket(cs); b != "STALE" {
		t.Errorf("bucket = %s, want STALE", b)
	}
	cs = append(cs, checkResult{"permissions", checkFail, ""})
	if b := bucket(cs); b != "RISK" {
		t.Errorf("bucket = %s, want RISK", b)
	}
	pass, total := score(cs)
	if pass != 1 || total != 3 {
		t.Errorf("score = %d/%d, want 1/3", pass, total)
	}
	if b := bucket([]checkResult{{"alive", checkPass, ""}}); b != "ok" {
		t.Errorf("bucket = %s, want ok", b)
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
