package core

import (
	"fmt"
	"strings"
)

// CheckOutcome is the result of a single audit check.
type CheckOutcome int

const (
	CheckPass CheckOutcome = iota
	CheckFail
	CheckSkip
)

// Check is an exported audit check result for a single supply-chain dimension.
type Check struct {
	Name    string
	Outcome CheckOutcome
	Detail  string
}

// AuditScore summarises the supply-chain score for a single dependency.
type AuditScore struct {
	Bucket   string   // "ok", "RISK", or "STALE"
	Checks   []Check  // per-check results
	Unpinned []string // workflow refs that are not SHA-pinned
}

// AuditOne scores a single dependency identified by its ecosystem, name, and
// (optional) version. It makes GitHub API calls; token may be empty for public
// repos (rate limits apply). c may be nil.
func AuditOne(eco, name, version, token string, c *Cache) (AuditScore, error) {
	d, err := resolveDep(eco, name, version)
	if err != nil {
		return AuditScore{}, err
	}

	files, err := fetchWorkflows(d.owner, d.repo, token)
	if err != nil {
		return AuditScore{}, fmt.Errorf("fetch workflows for %s/%s: %w", d.owner, d.repo, err)
	}

	var agg refScan
	for wfName, body := range files {
		refs := findUnpinned(body)
		agg.total += refs.total
		agg.suppressed += refs.suppressed
		for _, u := range refs.unpinned {
			agg.unpinned = append(agg.unpinned, wfName+": "+u)
		}
		for _, u := range findRunInstalls(body) {
			agg.total++
			agg.unpinned = append(agg.unpinned, wfName+": "+u)
		}
	}

	internal := runChecks(d, files, agg, token)
	checks := make([]Check, len(internal))
	for i, c := range internal {
		checks[i] = Check{
			Name:    c.name,
			Outcome: CheckOutcome(c.outcome),
			Detail:  c.detail,
		}
	}

	return AuditScore{
		Bucket:   bucket(internal),
		Checks:   checks,
		Unpinned: agg.unpinned,
	}, nil
}

// ResolveTagSHA resolves a GHA action owner/repo and tag to the commit SHA.
// action is "owner/repo" (e.g. "actions/checkout"), tag is the ref (e.g. "v4").
func ResolveTagSHA(action, tag, token string) (string, error) {
	return resolveTagSHA(action, tag, token)
}

// ResolveLatestSHA returns the latest tag name and its commit SHA for a GitHub
// repo identified by "owner/repo". Used by the LSP to implement update-to-latest
// for GHA actions and pre-commit repos.
func ResolveLatestSHA(ownerRepo, token string) (sha, tag string, err error) {
	payload, err := GetLatestTag(ownerRepo, token)
	if err != nil {
		return "", "", err
	}
	m, ok := payload.(map[string]interface{})
	if !ok {
		return "", "", fmt.Errorf("unexpected tag payload for %s", ownerRepo)
	}
	tag, _ = m["name"].(string)
	commit, _ := m["commit"].(map[string]interface{})
	sha, _ = commit["sha"].(string)
	if sha == "" || tag == "" {
		return "", "", fmt.Errorf("missing sha or tag in response for %s", ownerRepo)
	}
	return sha, tag, nil
}

// resolveDep maps eco/name/version to an internal dep struct for API calls.
func resolveDep(eco, name, version string) (dep, error) {
	d := dep{source: eco, label: name}
	switch eco {
	case SourceGo:
		owner, repo, err := resolveRepo(name)
		if err != nil {
			return dep{}, fmt.Errorf("resolve go module %s: %w", name, err)
		}
		d.owner, d.repo = owner, repo
		if shaRe.MatchString(version) {
			d.pinnedSHA = version
		}

	case SourceGHA:
		parts := strings.SplitN(name, "/", 2)
		if len(parts) < 2 {
			return dep{}, fmt.Errorf("invalid GHA dep %q", name)
		}
		d.owner, d.repo = parts[0], parts[1]
		if shaRe.MatchString(version) {
			d.pinnedSHA = version
		}

	case SourcePreCommit:
		owner, repo, ok := githubOwnerRepo(name)
		if !ok {
			return dep{}, fmt.Errorf("pre-commit dep %q is not on GitHub", name)
		}
		d.owner, d.repo = owner, repo
		if shaRe.MatchString(version) {
			d.pinnedSHA = version
		}

	case SourceNpm, SourcePypi, SourceCargo, SourceGem:
		owner, repo, err := resolvePackageRepo(eco, name)
		if err != nil {
			return dep{}, fmt.Errorf("resolve %s package %s: %w", eco, name, err)
		}
		d.owner, d.repo = owner, repo

	default:
		return dep{}, fmt.Errorf("unknown ecosystem %q", eco)
	}
	return d, nil
}
