package core

import "testing"

func TestResolveDep(t *testing.T) {
	tests := []struct {
		eco, name, version string
		wantOwner          string
		wantErr            bool
	}{
		{"gha", "actions/checkout", "v4", "actions", false},
		{"gha", "actions/setup-go", "abc1234567890123456789012345678901234567890", "actions", false},
		{"gha", "bad", "", "", true},
		{"pre-commit", "https://github.com/pre-commit/pre-commit-hooks", "v4.4.0", "pre-commit", false},
		{"pre-commit", "https://gitlab.com/foo/bar", "", "", true},
		{"unknown-eco", "foo", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.eco+"/"+tt.name, func(t *testing.T) {
			d, err := resolveDep(tt.eco, tt.name, tt.version)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got dep %+v", d)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if d.owner != tt.wantOwner {
				t.Errorf("owner = %q, want %q", d.owner, tt.wantOwner)
			}
		})
	}
}

func TestResolveDepGHASHAPin(t *testing.T) {
	sha := "de0fac2e4500dabe0009e67214ff5f5447ce83dd"
	d, err := resolveDep("gha", "actions/checkout", sha)
	if err != nil {
		t.Fatal(err)
	}
	if d.pinnedSHA != sha {
		t.Errorf("pinnedSHA = %q, want %q", d.pinnedSHA, sha)
	}
}
