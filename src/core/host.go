package core

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// hostRepo is a provider-agnostic handle to a repository for the org sweep.
type hostRepo struct {
	Name     string // human-readable path, e.g. "owner/name" or "group/project"
	CloneURL string
	id       string // provider-internal identifier (GitHub: full_name, GitLab: numeric project ID)
}

// hostProvider abstracts the four API touchpoints the org sweep needs so the
// clone→sweep→push loop in processRepo stays host-agnostic.
type hostProvider interface {
	ListRepos() ([]hostRepo, error)
	RepoFromName(name string) (hostRepo, error)
	PRExists(r hostRepo, branch string) (open bool, prURL, mergeID string, err error)
	CreatePR(r hostRepo, head, base string) (prURL, mergeID string, err error)
	EnableAutoMerge(mergeID string) error
	WaitForRateLimit(threshold int)
}

// withBasicAuth injects user:token@ into an https clone URL so git can fetch
// private repos without a credential helper.
func withBasicAuth(cloneURL, user, token string) string {
	if token == "" {
		return cloneURL
	}
	u, err := url.Parse(cloneURL)
	if err != nil || u.Scheme != "https" {
		return cloneURL
	}
	u.User = url.UserPassword(user, token)
	return u.String()
}

func newHost(provider, owner, token, baseURL string) (hostProvider, error) {
	switch strings.ToLower(provider) {
	case "", "github":
		return &githubHost{owner: owner, token: token}, nil
	case "gitlab":
		if baseURL == "" {
			baseURL = "https://gitlab.com"
		}
		return &gitlabHost{owner: owner, token: token, baseURL: strings.TrimRight(baseURL, "/")}, nil
	default:
		return nil, fmt.Errorf("unknown --provider %q (supported: github, gitlab)", provider)
	}
}

// ---- GitHub ---------------------------------------------------------------

type githubHost struct {
	owner string
	token string
}

func (h *githubHost) ListRepos() ([]hostRepo, error) {
	u := "https://api.github.com/user/repos?type=owner&per_page=100"
	if h.owner != "" {
		u = "https://api.github.com/orgs/" + h.owner + "/repos?per_page=100"
		if _, _, err := getPagedGithubBody(h.token, u); err != nil {
			u = "https://api.github.com/users/" + h.owner + "/repos?per_page=100"
		}
	}
	var all []hostRepo
	for u != "" {
		body, next, err := getPagedGithubBody(h.token, u)
		if err != nil {
			return nil, err
		}
		items, ok := body.([]interface{})
		if !ok {
			return nil, fmt.Errorf("unexpected repos response type")
		}
		for _, item := range items {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			if fork, _ := m["fork"].(bool); fork {
				continue
			}
			name, _ := m["full_name"].(string)
			clone, _ := m["clone_url"].(string)
			if name != "" {
				all = append(all, hostRepo{Name: name, CloneURL: withBasicAuth(clone, "x-access-token", h.token), id: name})
			}
		}
		u = next
	}
	return all, nil
}

func (h *githubHost) RepoFromName(name string) (hostRepo, error) {
	clone := withBasicAuth("https://github.com/"+name+".git", "x-access-token", h.token)
	return hostRepo{Name: name, CloneURL: clone, id: name}, nil
}

func (h *githubHost) PRExists(r hostRepo, branch string) (bool, string, string, error) {
	owner := strings.SplitN(r.id, "/", 2)[0]
	u := fmt.Sprintf("https://api.github.com/repos/%s/pulls?head=%s:%s&state=open", r.id, owner, branch)
	body, err := GetGithubBody(h.token, u)
	if err != nil {
		return false, "", "", err
	}
	items, ok := body.([]interface{})
	if !ok || len(items) == 0 {
		return false, "", "", nil
	}
	if m, ok := items[0].(map[string]interface{}); ok {
		prURL, _ := m["html_url"].(string)
		nodeID, _ := m["node_id"].(string)
		return true, prURL, nodeID, nil
	}
	return true, "", "", nil
}

func (h *githubHost) CreatePR(r hostRepo, head, base string) (string, string, error) {
	u := fmt.Sprintf("https://api.github.com/repos/%s/pulls", r.id)
	payload := map[string]string{
		"title": "chore: pin dependencies to immutable SHAs via ghat",
		"body":  prBody,
		"head":  head,
		"base":  base,
	}
	b, _ := json.Marshal(payload)
	resp, err := postGithubBody(h.token, u, b)
	if err != nil {
		return "", "", err
	}
	m, ok := resp.(map[string]interface{})
	if !ok {
		return "", "", fmt.Errorf("unexpected PR response")
	}
	prURL, _ := m["html_url"].(string)
	nodeID, _ := m["node_id"].(string)
	return prURL, nodeID, nil
}

func (h *githubHost) EnableAutoMerge(nodeID string) error {
	query := `mutation($id:ID!){enablePullRequestAutoMerge(input:{pullRequestId:$id,mergeMethod:SQUASH}){clientMutationId}}`
	payload, _ := json.Marshal(map[string]interface{}{
		"query":     query,
		"variables": map[string]string{"id": nodeID},
	})
	_, err := postGithubBody(h.token, "https://api.github.com/graphql", payload)
	return err
}

func (h *githubHost) WaitForRateLimit(threshold int) {
	body, err := GetGithubBody(h.token, "https://api.github.com/rate_limit")
	if err != nil {
		return
	}
	m, _ := body.(map[string]interface{})
	resources, _ := m["resources"].(map[string]interface{})
	core, _ := resources["core"].(map[string]interface{})
	remaining, _ := core["remaining"].(float64)
	reset, _ := core["reset"].(float64)
	if int(remaining) < threshold {
		wait := time.Duration(int64(reset)-time.Now().Unix()+5) * time.Second
		if wait > 0 {
			log.Info().Int("remaining", int(remaining)).Dur("wait", wait).Msg("rate limit low — pausing")
			time.Sleep(wait)
		}
	}
}

// ---- GitLab ---------------------------------------------------------------

type gitlabHost struct {
	owner   string // group path or username; empty = projects owned by token user
	token   string
	baseURL string // e.g. https://gitlab.com or https://gitlab.example.com
}

func (h *gitlabHost) api(path string) string { return h.baseURL + "/api/v4" + path }

func (h *gitlabHost) ListRepos() ([]hostRepo, error) {
	u := h.api("/projects?owned=true&archived=false&per_page=100")
	if h.owner != "" {
		u = h.api("/groups/" + url.PathEscape(h.owner) + "/projects?include_subgroups=true&archived=false&per_page=100")
		if _, _, err := getPagedGithubBody(h.token, u); err != nil {
			u = h.api("/users/" + url.PathEscape(h.owner) + "/projects?archived=false&per_page=100")
		}
	}
	var all []hostRepo
	for u != "" {
		body, next, err := getPagedGithubBody(h.token, u)
		if err != nil {
			return nil, err
		}
		items, ok := body.([]interface{})
		if !ok {
			return nil, fmt.Errorf("unexpected projects response type")
		}
		for _, item := range items {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			if forked := m["forked_from_project"]; forked != nil {
				continue
			}
			name, _ := m["path_with_namespace"].(string)
			clone, _ := m["http_url_to_repo"].(string)
			id, _ := m["id"].(float64)
			if name != "" {
				all = append(all, hostRepo{Name: name, CloneURL: withBasicAuth(clone, "oauth2", h.token), id: fmt.Sprintf("%d", int64(id))})
			}
		}
		u = next
	}
	return all, nil
}

func (h *gitlabHost) RepoFromName(name string) (hostRepo, error) {
	// GitLab accepts URL-encoded namespace/project as the :id path param.
	id := url.PathEscape(name)
	body, err := GetGithubBody(h.token, h.api("/projects/"+id))
	if err != nil {
		return hostRepo{}, fmt.Errorf("lookup %s: %w", name, err)
	}
	m, _ := body.(map[string]interface{})
	clone, _ := m["http_url_to_repo"].(string)
	pid, _ := m["id"].(float64)
	return hostRepo{Name: name, CloneURL: withBasicAuth(clone, "oauth2", h.token), id: fmt.Sprintf("%d", int64(pid))}, nil
}

func (h *gitlabHost) PRExists(r hostRepo, branch string) (bool, string, string, error) {
	u := h.api("/projects/" + r.id + "/merge_requests?state=opened&source_branch=" + url.QueryEscape(branch))
	body, err := GetGithubBody(h.token, u)
	if err != nil {
		return false, "", "", err
	}
	items, ok := body.([]interface{})
	if !ok || len(items) == 0 {
		return false, "", "", nil
	}
	if m, ok := items[0].(map[string]interface{}); ok {
		mrURL, _ := m["web_url"].(string)
		iid, _ := m["iid"].(float64)
		return true, mrURL, fmt.Sprintf("%s:%d", r.id, int64(iid)), nil
	}
	return true, "", "", nil
}

func (h *gitlabHost) CreatePR(r hostRepo, head, base string) (string, string, error) {
	u := h.api("/projects/" + r.id + "/merge_requests")
	payload := map[string]interface{}{
		"title":                "chore: pin dependencies to immutable SHAs via ghat",
		"description":          prBody,
		"source_branch":        head,
		"target_branch":        base,
		"remove_source_branch": true,
	}
	b, _ := json.Marshal(payload)
	resp, err := postGithubBody(h.token, u, b)
	if err != nil {
		return "", "", err
	}
	m, ok := resp.(map[string]interface{})
	if !ok {
		return "", "", fmt.Errorf("unexpected MR response")
	}
	mrURL, _ := m["web_url"].(string)
	iid, _ := m["iid"].(float64)
	return mrURL, fmt.Sprintf("%s:%d", r.id, int64(iid)), nil
}

func (h *gitlabHost) EnableAutoMerge(mergeID string) error {
	pid, iid, ok := strings.Cut(mergeID, ":")
	if !ok {
		return fmt.Errorf("bad merge id %q", mergeID)
	}
	u := h.api("/projects/" + pid + "/merge_requests/" + iid + "/merge")
	payload, _ := json.Marshal(map[string]interface{}{
		"merge_when_pipeline_succeeds": true,
		"squash":                       true,
	})
	_, err := sendGithubBody(h.token, "PUT", u, payload)
	return err
}

// WaitForRateLimit is a no-op for GitLab: gitlab.com exposes per-endpoint
// RateLimit-* headers rather than a single /rate_limit resource, and
// self-hosted instances typically have no limit at all.
func (h *gitlabHost) WaitForRateLimit(threshold int) {}

const prBody = "Automated dependency pinning by [ghat](https://github.com/JamesWoolfenden/ghat).\n\n" +
	"Pins GitHub Actions, pre-commit hooks, Terraform modules/providers, Dockerfiles, and Kubernetes images to SHA digests."
