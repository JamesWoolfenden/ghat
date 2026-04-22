package core

import (
	"strings"
	"testing"
)

func TestRewritePreCommitRevs(t *testing.T) {
	t.Parallel()

	pins := map[string]revPin{
		"https://github.com/pre-commit/pre-commit-hooks": {sha: "3e8a8703264a2f4a69428a0aa4dcb512790b2c8c", tag: "v6.0.0"},
		"https://github.com/gitleaks/gitleaks":           {sha: "2ca41cc1372d1e939a6a879f18cdc19fc1cac1ce", tag: "v8.30.0"},
	}

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "tag to sha with comment, preserves indent and surrounding comments",
			in: "repos:\n" +
				"  # keep me\n" +
				"  - repo: https://github.com/pre-commit/pre-commit-hooks\n" +
				"    rev: v5.0.0\n" +
				"    hooks:\n" +
				"      - id: trailing-whitespace\n",
			want: "repos:\n" +
				"  # keep me\n" +
				"  - repo: https://github.com/pre-commit/pre-commit-hooks\n" +
				"    rev: 3e8a8703264a2f4a69428a0aa4dcb512790b2c8c # v6.0.0\n" +
				"    hooks:\n" +
				"      - id: trailing-whitespace\n",
		},
		{
			name: "existing sha and stale comment are replaced",
			in: "repos:\n" +
				"  - repo: https://github.com/gitleaks/gitleaks\n" +
				"    rev: deadbeefdeadbeefdeadbeefdeadbeefdeadbeef  # v1.0.0\n",
			want: "repos:\n" +
				"  - repo: https://github.com/gitleaks/gitleaks\n" +
				"    rev: 2ca41cc1372d1e939a6a879f18cdc19fc1cac1ce # v8.30.0\n",
		},
		{
			name: "local and unknown repos are untouched",
			in: "repos:\n" +
				"  - repo: local\n" +
				"    hooks:\n" +
				"      - id: noop\n" +
				"  - repo: https://github.com/unknown/thing\n" +
				"    rev: v1.0.0\n",
			want: "repos:\n" +
				"  - repo: local\n" +
				"    hooks:\n" +
				"      - id: noop\n" +
				"  - repo: https://github.com/unknown/thing\n" +
				"    rev: v1.0.0\n",
		},
		{
			name: "rev before hooks key with trailing repo comment",
			in: "repos:\n" +
				"  - repo: https://github.com/pre-commit/pre-commit-hooks  # upstream\n" +
				"    rev: v5.0.0\n",
			want: "repos:\n" +
				"  - repo: https://github.com/pre-commit/pre-commit-hooks  # upstream\n" +
				"    rev: 3e8a8703264a2f4a69428a0aa4dcb512790b2c8c # v6.0.0\n",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := rewritePreCommitRevs(tt.in, pins)
			if got != tt.want {
				t.Errorf("rewritePreCommitRevs() mismatch\n--- want ---\n%s\n--- got ---\n%s",
					strings.ReplaceAll(tt.want, " ", "·"),
					strings.ReplaceAll(got, " ", "·"))
			}
		})
	}
}

func TestParseLsRemoteTags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		in      string
		wantSHA string
		wantTag string
		wantErr bool
	}{
		{
			// --sort=-version:refname puts the highest first; the ^{} peel
			// for an annotated tag may land on either side of its ref line
			// depending on git version — parser must not care.
			name: "annotated highest, peeled sha wins",
			in: "27652d0e1c3d7d571685d4ce1f9573af28026d70\trefs/tags/v1.1.0\n" +
				"e0d38715ff9d90334235cbeccb2cdc12b0297255\trefs/tags/v1.1.0^{}\n" +
				"ed2ed26d6c29d12e170de8b667dc6511bb864e60\trefs/tags/v1.0.1\n" +
				"6e60a820cd7492b5b53a883de6daa9ddfa36a992\trefs/tags/v1.0.1^{}\n" +
				"6e60a820cd7492b5b53a883de6daa9ddfa36a992\trefs/tags/v1.0.0\n",
			wantSHA: "e0d38715ff9d90334235cbeccb2cdc12b0297255",
			wantTag: "v1.1.0",
		},
		{
			name: "lightweight tag (no peel line) returns ref sha",
			in: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\trefs/tags/v2.0.0\n" +
				"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\trefs/tags/v1.0.0\n",
			wantSHA: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			wantTag: "v2.0.0",
		},
		{
			name: "peel line sorted before its ref line",
			in: "cccccccccccccccccccccccccccccccccccccccc\trefs/tags/v3.0.0^{}\n" +
				"dddddddddddddddddddddddddddddddddddddddd\trefs/tags/v3.0.0\n",
			wantSHA: "cccccccccccccccccccccccccccccccccccccccc",
			wantTag: "v3.0.0",
		},
		{
			name:    "no tags",
			in:      "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sha, tag, err := parseLsRemoteTags(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if sha != tt.wantSHA || tag != tt.wantTag {
				t.Errorf("got (%q, %q), want (%q, %q)", sha, tag, tt.wantSHA, tt.wantTag)
			}
		})
	}
}

func TestFlags_UpdateHooks(t *testing.T) {
	t.Parallel()
	type fields struct {
		File            string
		Directory       string
		GitHubToken     string
		Days            *uint
		DryRun          bool
		Entries         []string
		Update          bool
		ContinueOnError bool
	}

	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{name: "Empty", fields: fields{GitHubToken: gitHubToken}, wantErr: true},
		{name: "guff", fields: fields{Directory: "guff", GitHubToken: gitHubToken}, wantErr: true},
		{name: "Pass relative", fields: fields{Directory: "../../", GitHubToken: gitHubToken}, wantErr: false},
		//{name: "Pass absolute", fields: fields{Directory: "E:/Code/pike", GitHubToken: gitHubToken}, wantErr: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			myFlags := &Flags{
				File:            tt.fields.File,
				Directory:       tt.fields.Directory,
				GitHubToken:     tt.fields.GitHubToken,
				Days:            tt.fields.Days,
				DryRun:          tt.fields.DryRun,
				Entries:         tt.fields.Entries,
				Update:          tt.fields.Update,
				ContinueOnError: tt.fields.ContinueOnError,
			}
			if err := myFlags.UpdateHooks(); (err != nil) != tt.wantErr {
				t.Errorf("UpdateHooks() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
