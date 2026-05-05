package core

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"golang.org/x/mod/modfile"
	"gopkg.in/yaml.v3"
)

var (
	usesRe     = regexp.MustCompile(`uses:\s*["']?([^\s"'#]+)`)
	shaRe      = regexp.MustCompile(`^[0-9a-f]{40}$`)
	goImportRe = regexp.MustCompile(`<meta\s+name="go-import"\s+content="[^ ]+\s+git\s+(https://[^"]+)"`)
	ghRepoRe   = regexp.MustCompile(`github\.com[:/]([\w.-]+)/([\w.-]+)`)
	tfSourceRe = regexp.MustCompile(`source\s*=\s*"([^"]+)"`)
)

const (
	SourceGo        = "go"
	SourceGHA       = "gha"
	SourcePreCommit = "pre-commit"
	SourceTerraform = "terraform"
)

var allSources = []string{SourceGo, SourceGHA, SourcePreCommit, SourceTerraform}

type dep struct {
	source    string
	label     string
	owner     string
	repo      string
	pinnedSHA string // the SHA *we* have pinned this dep to, if any
	skip      string
}

type auditResult struct {
	source     string
	label      string
	repo       string
	skipped    string
	scanned    int
	total      int
	unpinned   []string
	suppressed int // refs skipped via # ghat:suppress in the dep's own workflows
	checks     []checkResult
	bucket     string
}

func (f *Flags) Audit() error {
	sources := f.Sources
	if len(sources) == 0 {
		sources = allSources
	}

	var deps []dep
	for _, s := range sources {
		switch s {
		case SourceGo:
			deps = append(deps, f.collectGoDeps()...)
		case SourceGHA:
			deps = append(deps, f.collectGHADeps()...)
		case SourcePreCommit:
			deps = append(deps, f.collectPreCommitDeps()...)
		case SourceTerraform:
			deps = append(deps, f.collectTerraformDeps()...)
		default:
			return fmt.Errorf("unknown audit source %q (valid: %s)", s, strings.Join(allSources, ", "))
		}
	}

	var results []auditResult
	seen := map[string]bool{}

	for _, d := range deps {
		if d.skip != "" {
			results = append(results, auditResult{source: d.source, label: d.label, skipped: d.skip})
			continue
		}
		key := d.owner + "/" + d.repo
		if seen[key] {
			continue
		}
		seen[key] = true

		files, err := fetchWorkflows(d.owner, d.repo, f.GitHubToken)
		if err != nil {
			log.Warn().Str("dep", d.label).Err(err).Msg("failed to fetch workflows")
			results = append(results, auditResult{source: d.source, label: d.label, repo: key, skipped: "fetch failed"})
			continue
		}

		res := auditResult{source: d.source, label: d.label, repo: key, scanned: len(files)}
		var agg refScan
		for name, body := range files {
			refs := findUnpinned(body)
			agg.total += refs.total
			agg.suppressed += refs.suppressed
			for _, u := range refs.unpinned {
				agg.unpinned = append(agg.unpinned, name+": "+u)
			}
		}
		res.total = agg.total
		res.suppressed = agg.suppressed
		res.unpinned = agg.unpinned
		res.checks = runChecks(d, files, agg, f.GitHubToken)
		res.bucket = bucket(res.checks)
		results = append(results, res)
	}

	return reportAudit(results)
}

func (f *Flags) listModules() ([]string, error) {
	if f.Deep {
		cmd := exec.Command("go", "list", "-m", "-json", "all")
		cmd.Dir = f.Directory
		out, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("go list -m all: %w", err)
		}
		dec := json.NewDecoder(strings.NewReader(string(out)))
		var mods []string
		for {
			var m struct {
				Path string
				Main bool
			}
			if err := dec.Decode(&m); err != nil {
				if err == io.EOF {
					break
				}
				return nil, err
			}
			if !m.Main {
				mods = append(mods, m.Path)
			}
		}
		return mods, nil
	}

	data, err := os.ReadFile(filepath.Join(f.Directory, "go.mod"))
	if err != nil {
		return nil, err
	}
	mf, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		return nil, err
	}
	var mods []string
	for _, r := range mf.Require {
		if !r.Indirect {
			mods = append(mods, r.Mod.Path)
		}
	}
	return mods, nil
}

func resolveRepo(modulePath string) (owner, repo string, err error) {
	if rest, ok := strings.CutPrefix(modulePath, "github.com/"); ok {
		return splitGithubPath(rest)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get("https://" + modulePath + "?go-get=1")
	if err != nil {
		return "", "", fmt.Errorf("vanity lookup failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	body, _ := io.ReadAll(resp.Body)

	m := goImportRe.FindStringSubmatch(string(body))
	if m == nil {
		return "", "", fmt.Errorf("no go-import meta tag")
	}
	repoURL := strings.TrimSuffix(m[1], ".git")
	rest, ok := strings.CutPrefix(repoURL, "https://github.com/")
	if !ok {
		return "", "", fmt.Errorf("not on GitHub (%s)", repoURL)
	}
	return splitGithubPath(rest)
}

func splitGithubPath(p string) (string, string, error) {
	parts := strings.Split(p, "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("malformed github path %q", p)
	}
	return parts[0], parts[1], nil
}

func fetchWorkflows(owner, repo, token string) (map[string][]byte, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/.github/workflows", owner, repo)
	listing, err := GetGithubBody(token, url)
	if err != nil {
		if strings.Contains(err.Error(), "status 404") {
			return map[string][]byte{}, nil
		}
		return nil, err
	}
	items, ok := listing.([]interface{})
	if !ok {
		return map[string][]byte{}, nil
	}

	out := map[string][]byte{}
	for _, it := range items {
		m, ok := it.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := m["name"].(string)
		typ, _ := m["type"].(string)
		itemURL, _ := m["url"].(string)
		if typ != "file" || (!strings.HasSuffix(name, ".yml") && !strings.HasSuffix(name, ".yaml")) {
			continue
		}
		file, err := GetGithubBody(token, itemURL)
		if err != nil {
			log.Warn().Str("file", name).Err(err).Msg("failed to fetch workflow file")
			continue
		}
		fm, ok := file.(map[string]interface{})
		if !ok {
			continue
		}
		enc, _ := fm["content"].(string)
		dec, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(enc, "\n", ""))
		if err != nil {
			continue
		}
		out[name] = dec
	}
	return out, nil
}

type refScan struct {
	total      int
	suppressed int
	unpinned   []string
}

func findUnpinned(body []byte) refScan {
	var rs refScan
	for _, line := range strings.Split(string(body), "\n") {
		m := usesRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		ref := strings.TrimSpace(m[1])
		if strings.HasPrefix(ref, "./") || strings.HasPrefix(ref, "docker://") {
			continue
		}
		at := strings.LastIndex(ref, "@")
		if at < 0 {
			continue
		}
		if isSuppressed(line) {
			rs.suppressed++
			continue
		}
		rs.total++
		if !shaRe.MatchString(ref[at+1:]) {
			rs.unpinned = append(rs.unpinned, ref)
		}
	}
	return rs
}

func reportAudit(results []auditResult) error {
	type tally struct{ ok, risk, stale, total int }
	bySource := map[string]*tally{}
	var risk, stale int

	for _, r := range results {
		if _, ok := bySource[r.source]; !ok {
			bySource[r.source] = &tally{}
		}
		if r.skipped != "" {
			fmt.Printf("[  ?? ] %-10s %-45s skipped: %s\n", r.source, r.label, r.skipped)
			continue
		}
		t := bySource[r.source]
		t.total++
		switch r.bucket {
		case "RISK":
			t.risk++
			risk++
		case "STALE":
			t.stale++
			stale++
		default:
			t.ok++
		}
		pass, total := score(r.checks)
		fmt.Printf("[%-5s] %-10s %-45s %-35s %d/%d\n", r.bucket, r.source, r.label, r.repo, pass, total)
		if r.bucket != "ok" {
			fmt.Printf("        %s\n", formatChecks(r.checks))
			for i, u := range r.unpinned {
				if i == 5 {
					fmt.Printf("          ... and %d more\n", len(r.unpinned)-5)
					break
				}
				fmt.Printf("          %s\n", u)
			}
		}
	}

	fmt.Printf("\n  %-10s %5s %5s %5s %5s\n", "", "total", "ok", "risk", "stale")
	for _, s := range allSources {
		if t, ok := bySource[s]; ok {
			fmt.Printf("  %-10s %5d %5d %5d %5d\n", s, t.total, t.ok, t.risk, t.stale)
		}
	}

	if risk > 0 {
		return fmt.Errorf("%d RISK, %d STALE", risk, stale)
	}
	if stale > 0 {
		fmt.Printf("\n%d STALE (no RISK findings)\n", stale)
	}
	return nil
}

func formatChecks(checks []checkResult) string {
	var b strings.Builder
	for i, c := range checks {
		if i > 0 {
			b.WriteString("  ")
		}
		switch c.outcome {
		case checkPass:
			b.WriteString("✓ ")
		case checkFail:
			b.WriteString("✗ ")
		case checkSkip:
			b.WriteString("- ")
		}
		b.WriteString(c.name)
		if c.detail != "" {
			b.WriteString(" (" + c.detail + ")")
		}
	}
	return b.String()
}

func (f *Flags) collectGoDeps() []dep {
	mods, err := f.listModules()
	if err != nil {
		log.Warn().Err(err).Msg("audit: no go.mod found, skipping go source")
		return nil
	}
	var deps []dep
	for _, mod := range mods {
		owner, repo, err := resolveRepo(mod)
		if err != nil {
			deps = append(deps, dep{source: SourceGo, label: mod, skip: err.Error()})
			continue
		}
		deps = append(deps, dep{source: SourceGo, label: mod, owner: owner, repo: repo})
	}
	return deps
}

func (f *Flags) collectGHADeps() []dep {
	seen := map[string]bool{}
	var deps []dep
	for _, file := range f.Entries {
		abs, _ := filepath.Abs(file)
		if !strings.Contains(abs, githubWorkflowPath) {
			continue
		}
		if ext := filepath.Ext(file); ext != yamlExtension && ext != yamlAltExtension {
			continue
		}
		body, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(body), "\n") {
			m := usesRe.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			if isSuppressed(line) {
				continue
			}
			ref := strings.TrimSpace(m[1])
			if strings.HasPrefix(ref, "./") || strings.HasPrefix(ref, "docker://") {
				continue
			}
			path, ver, _ := strings.Cut(ref, "@")
			parts := strings.SplitN(path, "/", 3)
			if len(parts) < 2 {
				continue
			}
			key := parts[0] + "/" + parts[1]
			if seen[key] {
				continue
			}
			seen[key] = true
			d := dep{source: SourceGHA, label: key, owner: parts[0], repo: parts[1]}
			if shaRe.MatchString(ver) {
				d.pinnedSHA = ver
			}
			deps = append(deps, d)
		}
	}
	return deps
}

func (f *Flags) collectPreCommitDeps() []dep {
	data, err := os.ReadFile(filepath.Join(f.Directory, PreCommitConfigFile))
	if err != nil {
		return nil
	}
	var cfg ConfigFile
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		log.Warn().Err(err).Msg("audit: failed to parse .pre-commit-config.yaml")
		return nil
	}
	var deps []dep
	for _, r := range cfg.Repos {
		if !strings.Contains(r.Repo, "://") {
			continue
		}
		owner, repo, ok := githubOwnerRepo(r.Repo)
		if !ok {
			deps = append(deps, dep{source: SourcePreCommit, label: r.Repo, skip: "not on GitHub"})
			continue
		}
		d := dep{source: SourcePreCommit, label: r.Repo, owner: owner, repo: repo}
		if shaRe.MatchString(r.Rev) {
			d.pinnedSHA = r.Rev
		}
		deps = append(deps, d)
	}
	return deps
}

func (f *Flags) collectTerraformDeps() []dep {
	seen := map[string]bool{}
	var deps []dep
	for _, file := range f.Entries {
		if filepath.Ext(file) != ".tf" {
			continue
		}
		body, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		for _, m := range tfSourceRe.FindAllStringSubmatch(string(body), -1) {
			src := m[1]
			owner, repo, ok := githubOwnerRepo(src)
			if !ok {
				continue
			}
			key := owner + "/" + repo
			if seen[key] {
				continue
			}
			seen[key] = true
			deps = append(deps, dep{source: SourceTerraform, label: src, owner: owner, repo: repo})
		}
	}
	return deps
}

func githubOwnerRepo(s string) (string, string, bool) {
	m := ghRepoRe.FindStringSubmatch(s)
	if m == nil {
		return "", "", false
	}
	return m[1], strings.TrimSuffix(m[2], ".git"), true
}
