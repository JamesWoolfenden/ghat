package core

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"golang.org/x/mod/semver"
	"gopkg.in/yaml.v3"
)

const (
	githubWorkflowPath = ".github/workflows"
	terraformDir       = ".terraform"
	yamlExtension      = ".yml"
	yamlAltExtension   = ".yaml"
)

type readFilesError struct {
	err error
}

func (m *readFilesError) Error() string {
	return fmt.Sprintf("failed to read files: %s", m.err)
}

type absolutePathError struct {
	directory string
	err       error
}

func (m *absolutePathError) Error() string {
	return fmt.Sprintf("failed to get absolute path: %v %s ", m.err, m.directory)
}

func GetFiles(dir string) ([]string, error) {
	Entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, &readFilesError{err}
	}

	var ParsedEntries []string

	for _, entry := range Entries {
		AbsDir, err := filepath.Abs(dir)
		if err != nil {
			return nil, &absolutePathError{dir, err}
		}
		gitDir := filepath.Join(AbsDir, ".git")

		if entry.IsDir() {

			newDir := filepath.Join(AbsDir, entry.Name())

			if !(strings.Contains(newDir, terraformDir)) && newDir != gitDir && entry.Name() != "testdata" {
				newEntries, err := GetFiles(newDir)

				if err != nil {
					return nil, err
				}

				ParsedEntries = append(ParsedEntries, newEntries...)
			}
		} else {
			myFile := filepath.Join(dir, entry.Name())
			if !(strings.Contains(myFile, terraformDir)) {
				ParsedEntries = append(ParsedEntries, myFile)
			}
		}
	}

	return ParsedEntries, nil
}

func (myFlags *Flags) UpdateGHAS() error {
	for _, gha := range myFlags.GetGHA() {
		if err := myFlags.UpdateGHA(gha); err != nil {
			return &ghaUpdateError{gha: gha, err: err}
		}
	}

	return nil
}

// GetGHA gets all the actions in a directory
func (myFlags *Flags) GetGHA() []string {
	var ghat []string

	for _, match := range myFlags.Entries {
		match, _ = filepath.Abs(match)
		entry, _ := os.Stat(match)
		if entry == nil || entry.IsDir() {
			continue
		}
		slashed := filepath.ToSlash(match)
		base := filepath.Base(match)
		isWorkflow := strings.Contains(slashed, githubWorkflowPath) &&
			(strings.HasSuffix(slashed, yamlExtension) || strings.HasSuffix(slashed, yamlAltExtension))
		isActionFile := base == "action.yml" || base == "action.yaml"
		if isWorkflow || isActionFile {
			ghat = append(ghat, match)
		}
	}

	return ghat
}

// UpdateGHA updates am action with latest dependencies
func (myFlags *Flags) UpdateGHA(file string) error {
	buffer, err := os.ReadFile(file)
	if err != nil {
		return &ghaFileError{file}
	}

	replacement := string(buffer)

	r := regexp.MustCompile(`uses:(.*)`)
	matches := r.FindAllStringSubmatch(string(buffer), -1)
	for _, match := range matches {
		if ok, reason := parseSuppression(match[0]); ok {
			log.Info().Str("ref", strings.TrimSpace(match[1])).Str("reason", reason).Msg("skipping suppressed uses: line")
			continue
		}

		// Detect and strip YAML string quoting ("uses: \"owner/action@tag\"")
		// so we can rebuild the exact original string for replacement later.
		rawValue := strings.TrimSpace(match[1])
		leadingQuote, trailingQuote := "", ""
		if len(rawValue) > 0 && (rawValue[0] == '"' || rawValue[0] == '\'') {
			leadingQuote = string(rawValue[0])
			rawValue = rawValue[1:]
		}

		action := strings.Split(rawValue, "@")
		action[0] = strings.TrimSpace(action[0])

		if len(action) > 1 && len(action[1]) > 0 {
			last := action[1][len(action[1])-1]
			if last == '"' || last == '\'' {
				trailingQuote = string(last)
				action[1] = action[1][:len(action[1])-1]
			}
		}

		// Local/composite action path or docker:// ref — nothing to resolve on GitHub.
		if strings.HasPrefix(action[0], ".") || strings.HasPrefix(action[0], "/") || strings.HasPrefix(action[0], "docker://") {
			continue
		}

		// Reusable workflow call (owner/repo/.github/workflows/x.yml) — no releases to resolve.
		if strings.Contains(action[0], "/.github/workflows/") {
			log.Info().Str("ref", action[0]).Msg("skipping reusable workflow")
			continue
		}

		// Apply substitution: swap untrusted/abandoned action for a preferred fork.
		// Keep the original name+ref so the exact source string can be replaced.
		originalAction := action[0]
		originalRef := ""
		if len(action) > 1 {
			originalRef = action[1]
		}
		if sub, changed := myFlags.applySubstitution(action[0]); changed {
			log.Warn().Str("from", action[0]).Str("to", sub).Msg("substituting action")
			action[0] = sub
			// Clear existing pin — must re-resolve against the new target.
			if len(action) > 1 {
				action[1] = ""
			}
		}

		// Warn and skip dynamically constructed tags (e.g. ${{ env.VERSION }}) — unpinned refs are a supply chain risk
		if len(action) > 1 && strings.HasPrefix(strings.TrimSpace(action[1]), "$") {
			log.Warn().Msgf("SUPPLY CHAIN RISK: %s uses a dynamic tag expression '%s' which cannot be pinned — resolve to a specific version", strings.TrimSpace(action[0]), strings.TrimSpace(action[1]))
			continue
		}

		// Extract current SHA and tag if already pinned ("sha # tag" format)
		var currentSHA, currentTag string
		if len(action) > 1 {
			currentSHA, currentTag = parsePinnedRef(action[1])
		}

		// --pin-only: resolve the SHA for the current tag without upgrading.
		if myFlags.PinOnly {
			currentRef := action[1]
			if currentTag != "" {
				currentRef = currentTag
			}
			// already pinned to a SHA — nothing to do
			if currentSHA != "" {
				continue
			}
			if currentRef == "" {
				continue
			}
			sha, err := resolveTagSHA(action[0], currentRef, myFlags.GitHubToken)
			if err != nil {
				log.Warn().Msgf("failed to retrieve commit hash for %s@%s: %s", action[0], currentRef, err)
				continue
			}
			oldAction := leadingQuote + originalAction + "@" + originalRef + trailingQuote
			newAction := action[0] + "@" + sha + " # " + currentRef
			if !strings.Contains(string(buffer), oldAction) {
				log.Warn().Str("uses", oldAction).Msg("resolved pin but reconstructed ref not found in source — please report this")
			}
			replacement = strings.ReplaceAll(replacement, oldAction, newAction)
			continue
		}

		body, err := getPayload(action[0], myFlags.GitHubToken, myFlags.Days)

		if err != nil {
			body, err = getPayload(ownerRepo(action[0]), myFlags.GitHubToken, myFlags.Days)
			if err != nil {
				if myFlags.ContinueOnError {
					log.Info().Err(err).Msgf("skipping action %s", action[0])
					continue
				}
				return fmt.Errorf("failed to retrieve data for action %s with %s", action[0], err)
			}
		}

		msg, ok := body.(map[string]interface{})

		if !ok {
			return &castToMapError{"body"}
		}

		if msg["tag_name"] != nil {
			tag := msg["tag_name"].(string)

			sha, err := resolveTagSHA(action[0], tag, myFlags.GitHubToken)
			if err != nil {
				log.Warn().Msgf("failed to retrieve commit hash for %s@%s: %s", action[0], tag, err)
				continue
			}

			if isTagMutation(currentSHA, currentTag, sha, tag) {
				log.Warn().Msgf("SUSPICIOUS: %s@%s — SHA changed from %s to %s with the same tag%s. "+
					"The tag may have been moved to a different commit. Verify this is intentional before accepting.",
					action[0], tag, currentSHA, sha, commitVerification(ownerRepo(action[0]), sha, myFlags.GitHubToken))
			}

			// Don't downgrade: if the current pin is semver-higher than what the
			// API returned (e.g. a git tag with no GitHub Release), keep the
			// current version as-is. The sha in scope here is for the older tag
			// so we must not write it — just skip.
			if currentTag != "" && tag != currentTag {
				cv, nv := coerceSemver(currentTag), coerceSemver(tag)
				if cv != "" && nv != "" && semver.Compare(nv, cv) < 0 {
					log.Info().Str("action", action[0]).Str("current", currentTag).Str("api", tag).
						Msg("API returned older tag than current pin — keeping current version")
					continue
				}
			}

			oldAction := leadingQuote + originalAction + "@" + originalRef + trailingQuote
			newAction := action[0] + "@" + sha + " # " + tag

			if !strings.Contains(string(buffer), oldAction) {
				log.Warn().Str("uses", oldAction).Msg("resolved pin but reconstructed ref not found in source — please report this")
			}
			replacement = strings.ReplaceAll(replacement, oldAction, newAction)
		} else {
			log.Warn().Msgf("tag field empty skipping %s", action[0])
		}
	}

	if upgraded, upgradeErr := myFlags.applyInputUpgrades(replacement); upgradeErr == nil {
		replacement = upgraded
	} else {
		log.Warn().Err(upgradeErr).Msg("input upgrades failed, skipping")
	}

	// Pin images in container: and services: blocks.
	pinnedImgs := parsePinnedImages(string(buffer))
	containerImages, err := extractGHAContainerImages(string(buffer))
	if err != nil {
		log.Warn().Err(err).Msg("failed to extract container/service images from workflow")
	}
	for _, imageStr := range containerImages {
		imgRef := parseImageReference(imageStr)
		digest, err := myFlags.getImageDigest(&imgRef)
		if err != nil {
			log.Warn().Err(err).Str("image", imageStr).Msg("failed to get digest for container image, skipping")
			continue
		}
		if cur, ok := pinnedImgs[imgRef.Tag]; ok && isTagMutation(cur, imgRef.Tag, digest, imgRef.Tag) {
			log.Warn().Msgf("SUSPICIOUS: %s — digest changed from %s to %s with the same tag. "+
				"The image tag may have been repointed. Verify before accepting.", imageStr, cur, digest)
		}
		if at := strings.Index(imageStr, "@sha256:"); at >= 0 {
			replacement = strings.ReplaceAll(replacement, imageStr, imageStr[:at]+"@"+digest)
		} else {
			replacement = replaceWithComment(replacement, imageStr, formatImageWithDigest(imgRef, digest))
		}
	}

	replacement = ensurePermissions(file, replacement)

	myFlags.printDiff(file, string(buffer), replacement)

	if !myFlags.DryRun {
		newBuffer := []byte(replacement)

		err = os.WriteFile(file, newBuffer, 0644)

		if err != nil {
			return &writeGHAError{file}
		}
	}

	return nil
}

var (
	jobsRe     = regexp.MustCompile(`(?m)^jobs\s*:`)
	writeAllRe = regexp.MustCompile(`(?m)^\s*permissions\s*:\s*write-all\b`)
	// Workflows that clearly push back to the repo. False positives are cheap
	// (write is still narrower than the write-all default they had before).
	needsWriteRe = regexp.MustCompile(`(?m)uses:\s*["']?(?i:` +
		`softprops/action-gh-release|` +
		`goreleaser/goreleaser-action|` +
		`marvinpinto/action-automatic-releases|` +
		`ncipollo/release-action|` +
		`svenstaro/upload-release-action|` +
		`anothrNick/github-tag-action|` +
		`mathieudutour/github-tag-action|` +
		`actions/create-release|` +
		`elgohr/Github-Release-Action|` +
		`ad-m/github-push-action|` +
		`stefanzweifel/git-auto-commit-action|` +
		`EndBug/add-and-commit|` +
		`peter-evans/create-pull-request` +
		`)@|run:\s*git (?:push|tag)\b|^\s+git (?:push|tag)\b`)
)

// ensurePermissions inserts a least-privilege top-level permissions: block
// if the workflow doesn't declare one. Anchors on ^jobs: so composite
// action.yml files (which have runs:, not jobs:) are left alone. If a
// permissions block exists but is write-all, warns without rewriting.
func ensurePermissions(file, body string) string {
	loc := jobsRe.FindStringIndex(body)
	if loc == nil {
		return body
	}
	if writeAllRe.MatchString(body) {
		log.Warn().Str("file", file).Msg("SUPPLY CHAIN RISK: permissions: write-all grants the GITHUB_TOKEN full repo write — narrow to the scopes each job needs")
	}
	if permsRe.MatchString(body) {
		return body
	}
	scope := "read"
	if needsWriteRe.MatchString(body) {
		scope = "write"
	}
	log.Warn().Str("file", file).Msgf("workflow has no top-level permissions: block (default GITHUB_TOKEN is write-all) — adding contents: %s", scope)
	return body[:loc[0]] + "permissions:\n  contents: " + scope + "\n\n" + body[loc[0]:]
}

// extractGHAContainerImages finds image references in container: and services: blocks
// of a GitHub Actions workflow file.
func extractGHAContainerImages(content string) ([]string, error) {
	var doc map[string]interface{}
	if err := yaml.Unmarshal([]byte(content), &doc); err != nil {
		return nil, fmt.Errorf("failed to parse workflow YAML: %w", err)
	}

	var images []string
	jobs, _ := doc["jobs"].(map[string]interface{})
	for _, job := range jobs {
		jobMap, ok := job.(map[string]interface{})
		if !ok {
			continue
		}
		// jobs.<id>.container.image
		if container, ok := jobMap["container"].(map[string]interface{}); ok {
			if img, ok := container["image"].(string); ok && img != "" && !strings.HasPrefix(img, "$") {
				images = append(images, img)
			}
		}
		// jobs.<id>.services.<name>.image
		if services, ok := jobMap["services"].(map[string]interface{}); ok {
			for _, svc := range services {
				if svcMap, ok := svc.(map[string]interface{}); ok {
					if img, ok := svcMap["image"].(string); ok && img != "" && !strings.HasPrefix(img, "$") {
						images = append(images, img)
					}
				}
			}
		}
	}
	sort.Strings(images)
	return images, nil
}

// applyInputUpgrades scans workflow content for action `with:` inputs that need
// upgrading when the action's major version changes (e.g. golangci-lint-action v7+
// requires golangci-lint v2+). Rules are loaded from substitutions.yml / ~/.ghat.yml.
func (myFlags *Flags) applyInputUpgrades(content string) (string, error) {
	if len(myFlags.InputUpgrades) == 0 {
		return content, nil
	}

	// Pre-compile patterns and resolve "latest:owner/repo" values once.
	type resolvedRule struct {
		action  string
		input   string
		pattern *regexp.Regexp
		to      string
	}
	var rules []resolvedRule
	for _, u := range myFlags.InputUpgrades {
		pat, err := regexp.Compile(u.FromPattern)
		if err != nil {
			log.Warn().Err(err).Str("pattern", u.FromPattern).Msg("invalid input_upgrade from_pattern, skipping")
			continue
		}
		to := u.To
		if strings.HasPrefix(to, "latest:") {
			repo := strings.TrimPrefix(to, "latest:")
			body, err := GetLatestRelease(repo, myFlags.GitHubToken)
			if err != nil {
				log.Warn().Err(err).Str("repo", repo).Msg("failed to fetch latest release for input upgrade, skipping")
				continue
			}
			if m, ok := body.(map[string]interface{}); ok {
				if tag, ok := m["tag_name"].(string); ok {
					to = tag
				}
			}
		}
		rules = append(rules, resolvedRule{action: u.Action, input: u.Input, pattern: pat, to: to})
	}
	if len(rules) == 0 {
		return content, nil
	}

	lines := strings.Split(content, "\n")
	var activeRule *resolvedRule

	usesRe := regexp.MustCompile(`uses:\s+(\S+?)@`)
	inputRe := regexp.MustCompile(`^(\s+)(\w[\w-]*):\s+(\S+.*)$`)
	stepStartRe := regexp.MustCompile(`-\s+(uses|name|run|with):\s`)

	for i, line := range lines {
		// New step boundary — clear active rule.
		if activeRule != nil && stepStartRe.MatchString(strings.TrimSpace(line)) {
			if !strings.Contains(line, "uses:") {
				activeRule = nil
			}
		}

		// Check if this line uses an action we have a rule for.
		if m := usesRe.FindStringSubmatch(line); m != nil {
			actionName := m[1]
			for j := range rules {
				if actionName == rules[j].action {
					activeRule = &rules[j]
					break
				}
			}
			continue
		}

		// Check if this is the target input line within an active rule's step.
		if activeRule == nil {
			continue
		}
		m := inputRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		indent, key, value := m[1], m[2], m[3]
		if key != activeRule.input {
			continue
		}
		if !activeRule.pattern.MatchString(value) {
			activeRule = nil
			continue
		}
		log.Warn().Str("action", activeRule.action).Str("input", key).
			Str("from", value).Str("to", activeRule.to).Msg("upgrading action input")
		lines[i] = indent + key + ": " + activeRule.to
		activeRule = nil
	}

	return strings.Join(lines, "\n"), nil
}

// parsePinnedRef extracts the SHA and tag from a ref already pinned in "sha # tag" format.
// Returns empty strings if the ref is not in that format.
func parsePinnedRef(ref string) (sha, tag string) {
	re := regexp.MustCompile(`^([0-9a-f]{40})\s+#\s+(.+)$`)
	if m := re.FindStringSubmatch(strings.TrimSpace(ref)); m != nil {
		return m[1], strings.TrimSpace(m[2])
	}
	return "", ""
}

// isTagMutation returns true when the same version tag now points to a different commit,
// which may indicate a supply chain attack (mutable tag rewrite).
func isTagMutation(currentSHA, currentTag, newSHA, newTag string) bool {
	return currentSHA != "" && currentTag == newTag && newSHA != currentSHA
}

func getPayload(action string, gitHubToken string, days *uint) (interface{}, error) {

	if days == nil {
		return nil, &daysParameterError{}
	}

	if *days == 0 {
		return GetLatestRelease(action, gitHubToken)
	}

	return GetReleases(action, gitHubToken, days)
}

func GetLatestRelease(action string, gitHubToken string) (interface{}, error) {
	url := "https://api.github.com/repos/" + action + "/releases/latest"
	return GetGithubBody(gitHubToken, url)
}

func GetLatestTag(action string, gitHubToken string) (interface{}, error) {
	const maxPages = 5
	url := "https://api.github.com/repos/" + action + "/tags?per_page=100"
	var tagged []interface{}
	for p := 0; url != "" && p < maxPages; p++ {
		page, next, err := getPagedGithubBody(gitHubToken, url)
		if err != nil {
			return nil, err
		}
		items, ok := page.([]interface{})
		if !ok {
			return nil, fmt.Errorf("failed to assert slice %s", page)
		}
		tagged = append(tagged, items...)
		url = next
	}

	if len(tagged) == 0 {
		return nil, fmt.Errorf("repo %s has no tags", action)
	}

	return tagged[pickLatestTag(tagged)], nil
}

var versionRunRe = regexp.MustCompile(`\d+\.\d`)

// coerceSemver turns common tag spellings into something golang.org/x/mod/semver
// can compare: strips a leading textual prefix (release-, foo-v, krb5-), adds
// the v, and rejects anything without a dot so date stamps like 20060525 do not
// parse as a giant major version. Returns "" if unrecognisable.
func coerceSemver(tag string) string {
	loc := versionRunRe.FindStringIndex(tag)
	if loc == nil {
		return ""
	}
	v := "v" + tag[loc[0]:]
	if semver.IsValid(v) {
		return v
	}
	// 2-part versions (v0.1) — pad with .0 so semver can compare them.
	if p := strings.SplitN(v, ".", 3); len(p) == 2 {
		if alt := v + ".0"; semver.IsValid(alt) {
			return alt
		}
	}
	// 4-part versions (shellcheck-py v0.11.0.1) — fold the tail into build
	// metadata so x/mod/semver will at least order the first three parts.
	if p := strings.SplitN(v, ".", 4); len(p) == 4 {
		if alt := strings.Join(p[:3], ".") + "+" + p[3]; semver.IsValid(alt) {
			return alt
		}
	}
	return ""
}

// pickLatestTag returns the index of the highest-version tag in a GitHub /tags
// response. The API orders by ref creation, which for repos that backport or
// keep test tags (krb5, oqs-provider) puts junk first. Stable releases beat
// pre-releases; anything that will not coerce to semver is ignored unless
// nothing coerces, in which case we fall back to the API's own ordering.
func pickLatestTag(tagged []any) int {
	best, bestV := 0, ""
	for i, t := range tagged {
		m, ok := t.(map[string]any)
		if !ok {
			continue
		}
		name, _ := m["name"].(string)
		v := coerceSemver(name)
		if v == "" {
			continue
		}
		if bestV == "" {
			best, bestV = i, v
			continue
		}
		bestPre, vPre := semver.Prerelease(bestV) != "", semver.Prerelease(v) != ""
		if bestPre != vPre {
			if bestPre {
				best, bestV = i, v
			}
			continue
		}
		if semver.Compare(v, bestV) > 0 {
			best, bestV = i, v
		}
	}
	return best
}

func getHash(action string, tag string, gitHubToken string) (interface{}, error) {
	url := "https://api.github.com/repos/" + action + "/git/ref/tags/" + tag
	return GetGithubBody(gitHubToken, url)
}

// ownerRepo strips any sub-path from an action ref so it can be used as a
// /repos/{owner}/{repo} API path (e.g. "github/codeql-action/init" → "github/codeql-action").
func ownerRepo(action string) string {
	parts := strings.SplitN(action, "/", 3)
	if len(parts) < 2 {
		return action
	}
	return parts[0] + "/" + parts[1]
}

// resolveTagSHA returns the commit SHA a tag points at, dereferencing
// annotated tag objects to the underlying commit.
func resolveTagSHA(action, tag, token string) (string, error) {
	payload, err := getHash(ownerRepo(action), tag, token)
	if err != nil {
		return "", err
	}
	body, ok := payload.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("unexpected ref payload for %s@%s", action, tag)
	}
	object, ok := body["object"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("ref %s@%s has no object", action, tag)
	}
	sha, _ := object["sha"].(string)
	if t, _ := object["type"].(string); t == "tag" {
		if tagURL, _ := object["url"].(string); tagURL != "" {
			if tagPayload, err := GetGithubBody(token, tagURL); err == nil {
				if tagMap, ok := tagPayload.(map[string]interface{}); ok {
					if tagObject, ok := tagMap["object"].(map[string]interface{}); ok {
						if commitSha, ok := tagObject["sha"].(string); ok {
							sha = commitSha
						}
					}
				}
			}
		}
	}
	if sha == "" {
		return "", fmt.Errorf("no sha for %s@%s", action, tag)
	}
	return sha, nil
}

// commitVerification returns a short " (new commit: …)" suffix describing the
// GPG/SSH signature status of sha, for triaging tag-repoint warnings. Returns
// "" if the lookup fails so the warning still fires.
func commitVerification(repo, sha, token string) string {
	body, err := GetGithubBody(token, "https://api.github.com/repos/"+repo+"/commits/"+sha)
	if err != nil {
		return ""
	}
	m, _ := body.(map[string]interface{})
	commit, _ := m["commit"].(map[string]interface{})
	v, _ := commit["verification"].(map[string]interface{})
	verified, _ := v["verified"].(bool)
	reason, _ := v["reason"].(string)
	if reason == "unsigned" {
		return " (new commit: UNSIGNED)"
	}
	if verified {
		return " (new commit: signed, verified)"
	}
	return fmt.Sprintf(" (new commit: signed, unverified — %s)", reason)
}

// GetGithubBodyWithCache fetches data from GitHub API with caching support
func GetGithubBodyWithCache(token, url string, cache *Cache) (interface{}, error) {
	// Try cache first
	if cache != nil && cache.enabled {
		if cached, found := cache.Get(url); found {
			log.Debug().Str("url", url).Msg("Using cached response")
			return cached, nil
		}
	}

	// Fetch from API
	log.Debug().Str("url", url).Msg("Fetching from GitHub API")
	data, err := GetGithubBody(token, url)
	if err != nil {
		return nil, err
	}

	// Store in cache
	if cache != nil && cache.enabled {
		if err := cache.Set(url, data); err != nil {
			log.Warn().Err(err).Msg("Failed to cache response")
		}
	}

	return data, nil
}

// GetGithubBody fetches data from GitHub API (existing function, keep as-is for compatibility)
func GetGithubBody(token, url string) (interface{}, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication if token provided
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	// Set user agent
	req.Header.Set("User-Agent", "ghat")
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	// If the token is invalid, retry without it rather than failing hard.
	if resp.StatusCode == 401 && token != "" {
		resp.Body.Close() //nolint:errcheck
		return GetGithubBody("", url)
	}

	// Check rate limiting
	if resp.StatusCode == 403 {
		if resp.Header.Get("X-RateLimit-Remaining") == "0" {
			resetTime := resp.Header.Get("X-RateLimit-Reset")
			return nil, fmt.Errorf("GitHub API rate limit exceeded, resets at %s", resetTime)
		}
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return result, nil
}

// getPagedGithubBody fetches one page and returns the body plus the URL of the next page (empty if last).
func getPagedGithubBody(token, url string) (interface{}, string, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %w", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("User-Agent", "ghat")
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(b))
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read response: %w", err)
	}

	var result interface{}
	if err := json.Unmarshal(b, &result); err != nil {
		return nil, "", fmt.Errorf("failed to parse JSON: %w", err)
	}

	// parse Link header for next page URL
	next := ""
	for _, part := range strings.Split(resp.Header.Get("Link"), ",") {
		part = strings.TrimSpace(part)
		if strings.Contains(part, `rel="next"`) {
			if s := strings.Index(part, "<"); s >= 0 {
				if e := strings.Index(part, ">"); e > s {
					next = part[s+1 : e]
				}
			}
		}
	}

	return result, next, nil
}

// postGithubBody sends a JSON POST to the GitHub API and returns the parsed response.
func postGithubBody(token, url string, payload []byte) (interface{}, error) {
	return sendGithubBody(token, http.MethodPost, url, payload)
}

func sendGithubBody(token, method, url string, payload []byte) (interface{}, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest(method, url, strings.NewReader(string(payload)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("User-Agent", "ghat")
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(b))
	}

	var result interface{}
	if err := json.Unmarshal(b, &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}
	return result, nil
}
