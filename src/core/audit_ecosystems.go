package core

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	SourceNpm   = "npm"
	SourcePypi  = "pypi"
	SourceCargo = "cargo"
	SourceGem   = "gem"
)

var (
	pypiNameRe  = regexp.MustCompile(`^\s*([A-Za-z0-9][\w.-]*)`)
	cargoDepRe  = regexp.MustCompile(`^\s*([\w-]+)\s*=`)
	gemRe       = regexp.MustCompile(`^\s*gem\s+['"]([\w-]+)['"]`)
	tomlArrayRe = regexp.MustCompile(`"([^"]+)"`)
)

func getJSON(u string, out any) error {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "ghat (https://github.com/JamesWoolfenden/ghat)")
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// resolvePackageRepo asks the ecosystem registry for pkg's source repo and
// returns the GitHub owner/repo if it lives there.
func resolvePackageRepo(eco, pkg string) (string, string, error) {
	var candidates []string
	switch eco {
	case SourceNpm:
		var p struct {
			Repository json.RawMessage `json:"repository"`
			Homepage   string          `json:"homepage"`
		}
		if err := getJSON("https://registry.npmjs.org/"+url.PathEscape(pkg)+"/latest", &p); err != nil {
			return "", "", err
		}
		var s string
		var o struct{ URL string }
		if json.Unmarshal(p.Repository, &s) == nil {
			candidates = append(candidates, s)
		} else if json.Unmarshal(p.Repository, &o) == nil {
			candidates = append(candidates, o.URL)
		}
		candidates = append(candidates, p.Homepage)

	case SourcePypi:
		var p struct {
			Info struct {
				ProjectURLs map[string]string `json:"project_urls"`
				HomePage    string            `json:"home_page"`
			} `json:"info"`
		}
		if err := getJSON("https://pypi.org/pypi/"+url.PathEscape(pkg)+"/json", &p); err != nil {
			return "", "", err
		}
		for _, v := range p.Info.ProjectURLs {
			candidates = append(candidates, v)
		}
		candidates = append(candidates, p.Info.HomePage)

	case SourceCargo:
		var p struct {
			Crate struct {
				Repository string `json:"repository"`
				Homepage   string `json:"homepage"`
			} `json:"crate"`
		}
		if err := getJSON("https://crates.io/api/v1/crates/"+url.PathEscape(pkg), &p); err != nil {
			return "", "", err
		}
		candidates = append(candidates, p.Crate.Repository, p.Crate.Homepage)

	case SourceGem:
		var p struct {
			SourceCodeURI string `json:"source_code_uri"`
			HomepageURI   string `json:"homepage_uri"`
		}
		if err := getJSON("https://rubygems.org/api/v1/gems/"+url.PathEscape(pkg)+".json", &p); err != nil {
			return "", "", err
		}
		candidates = append(candidates, p.SourceCodeURI, p.HomepageURI)
	}

	for _, c := range candidates {
		if owner, repo, ok := githubOwnerRepo(c); ok {
			return owner, repo, nil
		}
	}
	return "", "", fmt.Errorf("not on GitHub")
}

func registryDeps(source string, names []string) []dep {
	sort.Strings(names)
	deps := make([]dep, 0, len(names))
	for _, n := range names {
		owner, repo, err := resolvePackageRepo(source, n)
		if err != nil {
			deps = append(deps, dep{source: source, label: n, skip: err.Error()})
			continue
		}
		deps = append(deps, dep{source: source, label: n, owner: owner, repo: repo})
	}
	return deps
}

func (f *Flags) collectNpmDeps() []dep {
	data, err := os.ReadFile(filepath.Join(f.Directory, "package.json"))
	if err != nil {
		return nil
	}
	var p struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if json.Unmarshal(data, &p) != nil {
		return nil
	}
	seen := map[string]bool{}
	var names []string
	for _, m := range []map[string]string{p.Dependencies, p.DevDependencies} {
		for k := range m {
			if !seen[k] {
				seen[k] = true
				names = append(names, k)
			}
		}
	}
	return registryDeps(SourceNpm, names)
}

func (f *Flags) collectPypiDeps() []dep {
	seen := map[string]bool{}
	var names []string
	add := func(spec string) {
		spec = strings.TrimSpace(spec)
		if spec == "" || strings.HasPrefix(spec, "#") || strings.HasPrefix(spec, "-") {
			return
		}
		if m := pypiNameRe.FindStringSubmatch(spec); m != nil && !seen[m[1]] {
			seen[m[1]] = true
			names = append(names, m[1])
		}
	}
	for _, g := range []string{"requirements*.txt", "pyproject.toml"} {
		matches, _ := filepath.Glob(filepath.Join(f.Directory, g))
		for _, file := range matches {
			data, err := os.ReadFile(file) // #nosec G304
			if err != nil {
				continue
			}
			if strings.HasSuffix(file, ".toml") {
				for _, spec := range pyprojectDeps(string(data)) {
					add(spec)
				}
				continue
			}
			for _, line := range strings.Split(string(data), "\n") {
				add(line)
			}
		}
	}
	return registryDeps(SourcePypi, names)
}

// pyprojectDeps pulls quoted entries out of the [project] dependencies array.
func pyprojectDeps(body string) []string {
	i := strings.Index(body, "\n[project]")
	if i < 0 && !strings.HasPrefix(body, "[project]") {
		return nil
	}
	if i >= 0 {
		body = body[i:]
	}
	j := strings.Index(body, "dependencies")
	if j < 0 {
		return nil
	}
	body = body[j:]
	open := strings.Index(body, "[")
	closer := strings.Index(body, "]")
	if open < 0 || closer < open {
		return nil
	}
	var out []string
	for _, m := range tomlArrayRe.FindAllStringSubmatch(body[open:closer], -1) {
		out = append(out, m[1])
	}
	return out
}

func (f *Flags) collectCargoDeps() []dep {
	data, err := os.ReadFile(filepath.Join(f.Directory, "Cargo.toml"))
	if err != nil {
		return nil
	}
	var names []string
	in := false
	for _, line := range strings.Split(string(data), "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "[") {
			in = t == "[dependencies]" || strings.HasSuffix(t, "dependencies]")
			continue
		}
		if !in {
			continue
		}
		if m := cargoDepRe.FindStringSubmatch(line); m != nil {
			names = append(names, m[1])
		}
	}
	return registryDeps(SourceCargo, names)
}

func (f *Flags) collectGemDeps() []dep {
	data, err := os.ReadFile(filepath.Join(f.Directory, "Gemfile"))
	if err != nil {
		return nil
	}
	var names []string
	for _, line := range strings.Split(string(data), "\n") {
		if m := gemRe.FindStringSubmatch(line); m != nil {
			names = append(names, m[1])
		}
	}
	return registryDeps(SourceGem, names)
}

// GetLatestPackageVersion fetches the newest published version of pkg from its
// ecosystem's public registry. Returns the raw version string as the registry
// reports it (e.g. "4.18.2" for npm, "v0.22.0" for Go modules).
func GetLatestPackageVersion(eco, pkg string) (string, error) {
	switch eco {
	case SourceNpm:
		var p struct {
			Version string `json:"version"`
		}
		if err := getJSON("https://registry.npmjs.org/"+url.PathEscape(pkg)+"/latest", &p); err != nil {
			return "", err
		}
		if p.Version == "" {
			return "", fmt.Errorf("no version in npm registry response for %s", pkg)
		}
		return p.Version, nil

	case SourcePypi:
		var p struct {
			Info struct {
				Version string `json:"version"`
			} `json:"info"`
		}
		if err := getJSON("https://pypi.org/pypi/"+url.PathEscape(pkg)+"/json", &p); err != nil {
			return "", err
		}
		if p.Info.Version == "" {
			return "", fmt.Errorf("no version in PyPI response for %s", pkg)
		}
		return p.Info.Version, nil

	case SourceCargo:
		var p struct {
			Crate struct {
				NewestVersion string `json:"newest_version"`
			} `json:"crate"`
		}
		if err := getJSON("https://crates.io/api/v1/crates/"+url.PathEscape(pkg), &p); err != nil {
			return "", err
		}
		if p.Crate.NewestVersion == "" {
			return "", fmt.Errorf("no newest_version in crates.io response for %s", pkg)
		}
		return p.Crate.NewestVersion, nil

	case SourceGem:
		var p struct {
			Version string `json:"version"`
		}
		if err := getJSON("https://rubygems.org/api/v1/versions/"+url.PathEscape(pkg)+"/latest.json", &p); err != nil {
			return "", err
		}
		if p.Version == "" {
			return "", fmt.Errorf("no version in RubyGems response for %s", pkg)
		}
		return p.Version, nil

	case SourceGo:
		// Module paths contain slashes — don't escape them.
		var p struct {
			Version string `json:"Version"`
		}
		if err := getJSON("https://proxy.golang.org/"+pkg+"/@latest", &p); err != nil {
			return "", err
		}
		if p.Version == "" {
			return "", fmt.Errorf("no version in Go module proxy response for %s", pkg)
		}
		return p.Version, nil

	case SourceCpanfile:
		return GetMetaCPANVersion(pkg)
	}
	return "", fmt.Errorf("unsupported ecosystem %q", eco)
}
