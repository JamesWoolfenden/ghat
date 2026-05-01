package core

import (
	"reflect"
	"testing"
)

func TestParseGitmodules(t *testing.T) {
	t.Parallel()

	got, err := parseGitmodules("testdata/submodules/.gitmodules")
	if err != nil {
		t.Fatalf("parseGitmodules() error = %v", err)
	}

	want := []Submodule{
		{Name: "pyca.cryptography", Path: "pyca-cryptography", URL: "https://github.com/pyca/cryptography.git"},
		{Name: "krb5", Path: "krb5", URL: "https://github.com/krb5/krb5"},
		{Name: "gost-engine", Path: "gost-engine", URL: "https://github.com/gost-engine/engine"},
		{Name: "fuzz/corpora", Path: "fuzz/corpora", URL: "https://github.com/openssl/fuzz-corpora"},
		{Name: "wycheproof", Path: "wycheproof", URL: "https://github.com/google/wycheproof", Suppressed: true, SuppressReason: "tracks main intentionally"},
		{Name: "tlsfuzzer", Path: "tlsfuzzer", URL: "https://github.com/tlsfuzzer/tlsfuzzer", Suppressed: true},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseGitmodules()\n got  %#v\n want %#v", got, want)
	}
}

func TestParseGitmodules_Missing(t *testing.T) {
	t.Parallel()

	if _, err := parseGitmodules("testdata/submodules/nope"); err == nil {
		t.Error("parseGitmodules() expected error for missing file")
	}
}

func TestCoerceSemver(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"v3.0.3":                             "v3.0.3",
		"0.11.0":                             "v0.11.0",
		"python-ecdsa-0.19.2":                "v0.19.2",
		"krb5-1.22.1-final":                  "", // digit in the prefix defeats the heuristic
		"tokio-quiche-0.18.0":                "v0.18.0",
		"ms-bug-test-20060525":               "",
		"openssl-3.0.0-alpha12-liboqs-0.5.0": "v3.0.0-alpha12-liboqs-0.5.0",
		"release":                            "",
		"":                                   "",
	}
	for in, want := range cases {
		if got := coerceSemver(in); got != want {
			t.Errorf("coerceSemver(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPickLatestTag(t *testing.T) {
	t.Parallel()
	mk := func(names ...string) []any {
		out := make([]any, len(names))
		for i, n := range names {
			out[i] = map[string]any{"name": n}
		}
		return out
	}
	cases := []struct {
		name string
		tags []any
		want int
	}{
		{"stable beats prerelease", mk("v0.9.0-beta1", "v0.8.2"), 1},
		{"highest semver wins", mk("0.9.0", "0.11.0", "0.10.0"), 1},
		{"prefix stripped", mk("foo-1.0.0", "foo-2.0.0"), 1},
		{"junk ignored", mk("ms-bug-test-20060525", "v1.0.0"), 1},
		{"all junk falls back to 0", mk("alpha", "beta"), 0},
	}
	for _, tc := range cases {
		if got := pickLatestTag(tc.tags); got != tc.want {
			t.Errorf("%s: pickLatestTag() = %d, want %d", tc.name, got, tc.want)
		}
	}
}

func TestUpdateSubmodules_NoGitmodules(t *testing.T) {
	t.Parallel()

	flags := &Flags{Directory: t.TempDir(), DryRun: true}
	if err := flags.UpdateSubmodules(); err != nil {
		t.Errorf("UpdateSubmodules() with no .gitmodules should be a no-op, got %v", err)
	}
}
