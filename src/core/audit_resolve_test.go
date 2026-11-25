package core

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGopkgInRepo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path        string
		owner, repo string
		ok          bool
	}{
		{"gopkg.in/yaml.v3", "go-yaml", "yaml", true},
		{"gopkg.in/natefinch/lumberjack.v2", "natefinch", "lumberjack", true},
		{"gopkg.in/noversion", "", "", false},
		{"example.com/yaml.v3", "", "", false},
	}

	for _, tt := range tests {
		o, r, ok := gopkgInRepo(tt.path)
		if o != tt.owner || r != tt.repo || ok != tt.ok {
			t.Errorf("gopkgInRepo(%q) = %q, %q, %v; want %q, %q, %v", tt.path, o, r, ok, tt.owner, tt.repo, tt.ok)
		}
	}
}

// TestResolveRepoUsesProxy checks that vanity paths are resolved through the
// module proxy, and that the path is case-escaped the way the proxy expects.
func TestResolveRepoUsesProxy(t *testing.T) {
	var gotPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path

		switch {
		case strings.HasPrefix(r.URL.Path, "/go.uber.org/zap/"):
			_, _ = w.Write([]byte(`{"Version":"v1.0.0","Origin":{"VCS":"git","URL":"https://github.com/uber-go/zap.git"}}`))
		case strings.HasPrefix(r.URL.Path, "/go.googlesource.example/"):
			_, _ = w.Write([]byte(`{"Version":"v1.0.0","Origin":{"VCS":"git","URL":"https://go.googlesource.com/mod"}}`))
		case strings.HasPrefix(r.URL.Path, "/old.example/"):
			_, _ = w.Write([]byte(`{"Version":"v1.0.0"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	old := goProxyURL
	goProxyURL = srv.URL

	defer func() { goProxyURL = old }()

	o, r, err := resolveRepo("go.uber.org/zap")
	if err != nil || o != "uber-go" || r != "zap" {
		t.Errorf("go.uber.org/zap → %s/%s, %v", o, r, err)
	}

	if gotPath != "/go.uber.org/zap/@latest" {
		t.Errorf("proxy path = %q", gotPath)
	}

	if _, _, err := resolveRepo("go.googlesource.example/mod"); err == nil || !strings.Contains(err.Error(), "not on GitHub") {
		t.Errorf("non-GitHub origin: err = %v, want 'not on GitHub'", err)
	}

	if _, _, err := resolveRepo("old.example/pkg"); err == nil || !strings.Contains(err.Error(), "no origin") {
		t.Errorf("missing origin: err = %v, want 'no origin'", err)
	}

	if _, _, err := resolveRepo("missing.example/pkg"); err == nil {
		t.Error("404 from proxy: expected an error")
	}

	if _, _, err := resolveRepo("github.com/Azure/azure-sdk"); err != nil {
		t.Errorf("github.com paths must not need the proxy: %v", err)
	}

	// The stub 404s this; only the escaped request path matters.
	_, _, _ = resolveRepo("example.com/Pkg")

	if gotPath != "/example.com/!pkg/@latest" {
		t.Errorf("uppercase not escaped: proxy path = %q", gotPath)
	}
}

// TestResolveRepoNeverContactsModuleHost is the regression test for the
// vanity lookup: the module's own host must not be contacted.
func TestResolveRepoNeverContactsModuleHost(t *testing.T) {
	hit := false

	host := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hit = true }))
	defer host.Close()

	proxy := httptest.NewServer(http.NotFoundHandler())
	defer proxy.Close()

	old := goProxyURL
	goProxyURL = proxy.URL

	defer func() { goProxyURL = old }()

	_, _, _ = resolveRepo(strings.TrimPrefix(host.URL, "https://") + "/mod")

	if hit {
		t.Fatal("resolveRepo contacted the module host directly")
	}
}
