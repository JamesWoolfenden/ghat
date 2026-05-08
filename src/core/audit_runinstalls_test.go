package core

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/rs/zerolog/log"
)

func TestFindRunInstalls(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{
			name: "go install latest",
			body: "      run: |\n        go install golang.org/x/vuln/cmd/govulncheck@latest\n        govulncheck ./...",
			want: []string{"go-install: golang.org/x/vuln/cmd/govulncheck@latest"},
		},
		{
			name: "go install pinned",
			body: "  run: go install golang.org/x/vuln/cmd/govulncheck@v1.1.3",
		},
		{
			name: "go install main with flags",
			body: "go install -ldflags='-s -w' github.com/foo/bar@main",
			want: []string{"go-install: github.com/foo/bar@main"},
		},
		{
			name: "go run latest",
			body: "go run gotest.tools/gotestsum@latest --junitfile out.xml",
			want: []string{"go-install: gotest.tools/gotestsum@latest"},
		},
		{
			name: "pip unpinned",
			body: "  run: pip install black ruff",
			want: []string{"pip-install: black ruff"},
		},
		{
			name: "pip pinned",
			body: "pip install black==24.4.2",
		},
		{
			name: "pip requirements file",
			body: "pip3 install -r requirements.txt",
		},
		{
			name: "npx latest",
			body: "npx -y prettier@latest --check .",
			want: []string{"npx: prettier@latest"},
		},
		{
			name: "npm install latest",
			body: "npm install -g typescript@latest",
			want: []string{"npm-install: typescript@latest"},
		},
		{
			name: "cargo unpinned",
			body: "cargo install cargo-audit",
			want: []string{"cargo-install: cargo-audit"},
		},
		{
			name: "cargo pinned",
			body: "cargo install cargo-audit --version 0.21.0 --locked",
		},
		{
			name: "gem unpinned",
			body: "gem install bundler",
			want: []string{"gem-install: bundler"},
		},
		{
			name: "gem pinned",
			body: "gem install bundler -v 2.5.6",
		},
		{
			name: "curl pipe sh",
			body: "curl -sSL https://example.com/install.sh | bash",
			want: []string{"curl-pipe-sh: curl -sSL https://example.com/install.sh | bash"},
		},
		{
			name: "wget pipe sudo sh",
			body: "wget -qO- https://example.com/get | sudo sh",
			want: []string{"curl-pipe-sh: wget -qO- https://example.com/get | sudo sh"},
		},
		{
			name: "gitlab script array",
			body: "lint:\n  script:\n    - go install golang.org/x/vuln/cmd/govulncheck@latest\n    - govulncheck ./...",
			want: []string{"go-install: golang.org/x/vuln/cmd/govulncheck@latest"},
		},
		{
			name: "dockerfile RUN",
			body: "RUN go install github.com/google/ko@latest && ko version",
			want: []string{"go-install: github.com/google/ko@latest"},
		},
		{
			name: "suppressed",
			body: "go install golang.org/x/tools/cmd/stringer@latest # ghat:suppress floats intentionally",
		},
		{
			name: "comment ignored",
			body: "# go install foo@latest is what we used to do",
		},
		{
			name: "multiple on one line",
			body: "go install a@latest && pip install foo",
			want: []string{"go-install: a@latest", "pip-install: foo"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findRunInstalls([]byte(tt.body))
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("findRunInstalls() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWarnRunInstalls(t *testing.T) {
	var buf bytes.Buffer
	original := log.Logger
	log.Logger = log.Output(&buf)
	t.Cleanup(func() { log.Logger = original })

	warnRunInstalls("ci.yml", []byte("run: go install golang.org/x/vuln/cmd/govulncheck@latest"))
	out := buf.String()
	if !strings.Contains(out, "unpinned install in script") ||
		!strings.Contains(out, "go-install: golang.org/x/vuln/cmd/govulncheck@latest") ||
		!strings.Contains(out, "ci.yml") {
		t.Errorf("expected warning for govulncheck@latest, got: %s", out)
	}

	buf.Reset()
	warnRunInstalls("ci.yml", []byte("run: go install golang.org/x/vuln/cmd/govulncheck@v1.1.3"))
	if buf.Len() != 0 {
		t.Errorf("expected no output for pinned install, got: %s", buf.String())
	}
}

func TestIsRunInstallTarget(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{".github/workflows/ci.yml", true},
		{".github/workflows/release.yaml", true},
		{".github/actions/setup/action.yml", true},
		{"action.yaml", true},
		{"Dockerfile", true},
		{"docker/Dockerfile.build", true},
		{".github/scripts/tools.sh", true},
		{".github/Makefile", true},
		{".gitlab-ci.yml", true},
		{"ci/backend.gitlab-ci.yaml", true},
		{".gitlab/ci/lint.yml", true},
		{".gitlab/scripts/setup.sh", true},
		{"Makefile", false},
		{"scripts/tools.sh", false},
		{"src/main.go", false},
		{"README.md", false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := isRunInstallTarget(tt.path); got != tt.want {
				t.Errorf("isRunInstallTarget(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}
