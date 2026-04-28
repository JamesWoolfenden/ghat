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
)

var (
	usesRe     = regexp.MustCompile(`uses:\s*["']?([^\s"'#]+)`)
	shaRe      = regexp.MustCompile(`^[0-9a-f]{40}$`)
	goImportRe = regexp.MustCompile(`<meta\s+name="go-import"\s+content="[^ ]+\s+git\s+(https://[^"]+)"`)
)

type auditResult struct {
	module   string
	repo     string
	skipped  string
	scanned  int
	total    int
	unpinned []string
}

func (f *Flags) Audit() error {
	mods, err := f.listModules()
	if err != nil {
		return err
	}

	var results []auditResult
	seen := map[string]bool{}

	for _, mod := range mods {
		owner, repo, err := resolveRepo(mod)
		if err != nil {
			results = append(results, auditResult{module: mod, skipped: err.Error()})
			continue
		}
		key := owner + "/" + repo
		if seen[key] {
			continue
		}
		seen[key] = true

		files, err := fetchWorkflows(owner, repo, f.GitHubToken)
		if err != nil {
			log.Warn().Str("module", mod).Err(err).Msg("failed to fetch workflows")
			results = append(results, auditResult{module: mod, repo: key, skipped: "fetch failed"})
			continue
		}

		res := auditResult{module: mod, repo: key, scanned: len(files)}
		for name, body := range files {
			refs := findUnpinned(body)
			res.total += refs.total
			for _, u := range refs.unpinned {
				res.unpinned = append(res.unpinned, name+": "+u)
			}
		}
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
	total    int
	unpinned []string
}

func findUnpinned(body []byte) refScan {
	var rs refScan
	for _, m := range usesRe.FindAllStringSubmatch(string(body), -1) {
		ref := strings.TrimSpace(m[1])
		if strings.HasPrefix(ref, "./") || strings.HasPrefix(ref, "docker://") {
			continue
		}
		at := strings.LastIndex(ref, "@")
		if at < 0 {
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
	risky := 0
	for _, r := range results {
		if r.skipped != "" {
			fmt.Printf("  %-45s skipped: %s\n", r.module, r.skipped)
			continue
		}
		status := "ok"
		if len(r.unpinned) > 0 {
			status = "RISK"
			risky++
		}
		fmt.Printf("[%-4s] %-45s %s  workflows=%d  pinned=%d/%d\n",
			status, r.module, r.repo, r.scanned, r.total-len(r.unpinned), r.total)
		for _, u := range r.unpinned {
			fmt.Printf("         %s\n", u)
		}
	}
	fmt.Printf("\n%d of %d audited dependencies have unpinned CI actions\n", risky, len(results))
	if risky > 0 {
		return fmt.Errorf("%d dependencies have unpinned CI actions", risky)
	}
	return nil
}
