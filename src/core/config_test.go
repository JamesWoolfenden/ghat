package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplySubstitution(t *testing.T) {
	t.Parallel()

	f := &Flags{
		Substitutions: []Substitution{
			{From: "iamnotaturtle/auto-gofmt", To: "JamesWoolfenden/auto-gofmt"},
			{From: "old-org/tool", To: "new-org/tool"},
		},
	}

	tests := []struct {
		name        string
		input       string
		wantResult  string
		wantChanged bool
	}{
		{"match first", "iamnotaturtle/auto-gofmt", "JamesWoolfenden/auto-gofmt", true},
		{"match second", "old-org/tool", "new-org/tool", true},
		{"no match", "actions/checkout", "actions/checkout", false},
		{"partial match — no substitution", "iamnotaturtle/auto-gofmt/path", "iamnotaturtle/auto-gofmt/path", false},
		{"empty", "", "", false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, changed := f.applySubstitution(tt.input)
			if got != tt.wantResult || changed != tt.wantChanged {
				t.Errorf("applySubstitution(%q) = (%q, %v), want (%q, %v)",
					tt.input, got, changed, tt.wantResult, tt.wantChanged)
			}
		})
	}
}

func TestApplyRepoSubstitution(t *testing.T) {
	t.Parallel()

	f := &Flags{
		Substitutions: []Substitution{
			{From: "iamnotaturtle/auto-gofmt", To: "JamesWoolfenden/auto-gofmt"},
		},
	}

	tests := []struct {
		name        string
		input       string
		wantResult  string
		wantChanged bool
	}{
		{
			"github URL matches",
			"https://github.com/iamnotaturtle/auto-gofmt",
			"https://github.com/JamesWoolfenden/auto-gofmt",
			true,
		},
		{
			"github URL with .git suffix",
			"https://github.com/iamnotaturtle/auto-gofmt.git",
			"https://github.com/JamesWoolfenden/auto-gofmt",
			true,
		},
		{
			"no match",
			"https://github.com/actions/checkout",
			"https://github.com/actions/checkout",
			false,
		},
		{
			"non-github URL unchanged",
			"https://gitlab.com/iamnotaturtle/auto-gofmt",
			"https://gitlab.com/iamnotaturtle/auto-gofmt",
			false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, changed := f.applyRepoSubstitution(tt.input)
			if got != tt.wantResult || changed != tt.wantChanged {
				t.Errorf("applyRepoSubstitution(%q) = (%q, %v), want (%q, %v)",
					tt.input, got, changed, tt.wantResult, tt.wantChanged)
			}
		})
	}
}

func TestLoadConfig_DefaultsAlwaysPresent(t *testing.T) {
	t.Parallel()

	cfg := LoadConfig("")
	if len(cfg.Substitutions) == 0 {
		t.Fatal("expected built-in default substitutions, got none")
	}
	found := false
	for _, s := range cfg.Substitutions {
		if s.From == "iamnotaturtle/auto-gofmt" && s.To == "JamesWoolfenden/auto-gofmt" {
			found = true
			break
		}
	}
	if !found {
		t.Error("built-in iamnotaturtle/auto-gofmt substitution not found in defaults")
	}

	if len(cfg.InputUpgrades) == 0 {
		t.Fatal("expected built-in default input_upgrades, got none")
	}
	foundUpgrade := false
	for _, u := range cfg.InputUpgrades {
		if u.Action == "golangci/golangci-lint-action" && u.Input == "version" {
			foundUpgrade = true
			break
		}
	}
	if !foundUpgrade {
		t.Error("built-in golangci-lint input_upgrade not found in defaults")
	}
}

func TestLoadConfig_UserFilesMerge(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	content := "substitutions:\n  - from: old/thing\n    to: new/thing\n"
	if err := os.WriteFile(filepath.Join(dir, ".ghat.yml"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	cfg := LoadConfig(dir)

	foundDefault, foundUser := false, false
	for _, s := range cfg.Substitutions {
		if s.From == "iamnotaturtle/auto-gofmt" {
			foundDefault = true
		}
		if s.From == "old/thing" && s.To == "new/thing" {
			foundUser = true
		}
	}
	if !foundDefault {
		t.Error("built-in default substitution missing after merge")
	}
	if !foundUser {
		t.Error("user-defined substitution from .ghat.yml not found after merge")
	}
}

func TestApplyInputUpgrades(t *testing.T) {
	t.Parallel()

	f := &Flags{
		InputUpgrades: []InputUpgrade{
			{
				Action:      "golangci/golangci-lint-action",
				Input:       "version",
				FromPattern: `^v1\.`,
				To:          "v2.12.1",
			},
		},
	}

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "upgrades v1.x to configured version",
			in: "      - name: golangci-lint\n" +
				"        uses: golangci/golangci-lint-action@abc123 # v9.2.0\n" +
				"        with:\n" +
				"          version: v1.64\n",
			want: "      - name: golangci-lint\n" +
				"        uses: golangci/golangci-lint-action@abc123 # v9.2.0\n" +
				"        with:\n" +
				"          version: v2.12.1\n",
		},
		{
			name: "leaves v2.x untouched",
			in: "        uses: golangci/golangci-lint-action@abc123 # v9.2.0\n" +
				"        with:\n" +
				"          version: v2.1.0\n",
			want: "        uses: golangci/golangci-lint-action@abc123 # v9.2.0\n" +
				"        with:\n" +
				"          version: v2.1.0\n",
		},
		{
			name: "does not affect other actions",
			in: "        uses: actions/checkout@abc123 # v4\n" +
				"        with:\n" +
				"          version: v1.64\n",
			want: "        uses: actions/checkout@abc123 # v4\n" +
				"        with:\n" +
				"          version: v1.64\n",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := f.applyInputUpgrades(tt.in)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("applyInputUpgrades mismatch\n--- want ---\n%s\n--- got ---\n%s", tt.want, got)
			}
		})
	}
}

func TestRewritePreCommitRevs_WithSubstitution(t *testing.T) {
	t.Parallel()

	pins := map[string]revPin{
		"https://github.com/iamnotaturtle/auto-gofmt": {
			sha:    "de36c00b3eef35c5ed321296a11ca3c772da494b",
			tag:    "v0.3",
			newURL: "https://github.com/JamesWoolfenden/auto-gofmt",
		},
	}

	in := "repos:\n" +
		"  - repo: https://github.com/iamnotaturtle/auto-gofmt\n" +
		"    rev: 3934ab53013ffb44d3db33bbd1c271279b5925d5 # v2.1.0\n" +
		"    hooks:\n" +
		"      - id: auto-gofmt\n"

	want := "repos:\n" +
		"  - repo: https://github.com/JamesWoolfenden/auto-gofmt\n" +
		"    rev: de36c00b3eef35c5ed321296a11ca3c772da494b # v0.3\n" +
		"    hooks:\n" +
		"      - id: auto-gofmt\n"

	got := rewritePreCommitRevs(in, pins)
	if got != want {
		t.Errorf("rewritePreCommitRevs with substitution mismatch\n--- want ---\n%s\n--- got ---\n%s", want, got)
	}
}
