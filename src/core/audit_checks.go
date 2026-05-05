package core

import (
	"fmt"
	"regexp"
	"time"
)

type checkOutcome int

const (
	checkPass checkOutcome = iota
	checkFail
	checkSkip // not applicable / couldn't determine
)

type checkResult struct {
	name    string
	outcome checkOutcome
	detail  string
}

// severity classifies which bucket a failed check pushes the dep into.
// "risk" → active attack surface; "stale" → maintenance concern.
var checkSeverity = map[string]string{
	"signed-pin":        "risk",
	"ci-pinned":         "risk",
	"permissions":       "risk",
	"dangerous-trigger": "risk",
	"maintained":        "stale",
	"alive":             "stale",
}

var (
	permsRe = regexp.MustCompile(`(?m)^permissions:`)
	// pull_request_target on its own is fine; the danger is checking out PR head.
	prTargetRe   = regexp.MustCompile(`(?m)^\s*pull_request_target\s*:`)
	checkoutPRRe = regexp.MustCompile(`actions/checkout@.*\n(?:.*\n){0,6}?.*ref:\s*\$\{\{\s*github\.event\.pull_request`)
	// ${{ github.event.* }} interpolated into a run: shell block.
	runInjectRe = regexp.MustCompile(`(?m)^\s*run:\s*.*\$\{\{\s*github\.event\.`)
)

func runChecks(d dep, workflows map[string][]byte, rs refScan, token string) []checkResult {
	var out []checkResult
	out = append(out, checkSignedPin(d, token))
	out = append(out, checkCIPinned(rs))
	out = append(out, checkPermissions(workflows))
	out = append(out, checkDangerousTrigger(workflows))
	repoBody, _ := GetGithubBody(token, "https://api.github.com/repos/"+d.owner+"/"+d.repo)
	repo, _ := repoBody.(map[string]interface{})
	out = append(out, checkMaintained(d, repo, token))
	out = append(out, checkAlive(repo))
	return out
}

func checkSignedPin(d dep, token string) checkResult {
	if d.pinnedSHA == "" {
		return checkResult{"signed-pin", checkSkip, "no SHA pin"}
	}
	body, err := GetGithubBody(token, "https://api.github.com/repos/"+d.owner+"/"+d.repo+"/commits/"+d.pinnedSHA)
	if err != nil {
		return checkResult{"signed-pin", checkSkip, "lookup failed"}
	}
	m, _ := body.(map[string]interface{})
	commit, _ := m["commit"].(map[string]interface{})
	v, _ := commit["verification"].(map[string]interface{})
	verified, _ := v["verified"].(bool)
	reason, _ := v["reason"].(string)
	if verified {
		return checkResult{"signed-pin", checkPass, ""}
	}
	if reason == "" {
		reason = "unsigned"
	}
	return checkResult{"signed-pin", checkFail, reason}
}

func checkCIPinned(rs refScan) checkResult {
	if rs.total == 0 {
		return checkResult{"ci-pinned", checkSkip, "no workflows"}
	}
	if len(rs.unpinned) == 0 {
		return checkResult{"ci-pinned", checkPass, fmt.Sprintf("%d/%d", rs.total, rs.total)}
	}
	return checkResult{"ci-pinned", checkFail, fmt.Sprintf("%d/%d", rs.total-len(rs.unpinned), rs.total)}
}

func checkPermissions(workflows map[string][]byte) checkResult {
	if len(workflows) == 0 {
		return checkResult{"permissions", checkSkip, "no workflows"}
	}
	bad := 0
	for _, body := range workflows {
		if !permsRe.Match(body) || writeAllRe.Match(body) {
			bad++
		}
	}
	if bad == 0 {
		return checkResult{"permissions", checkPass, ""}
	}
	return checkResult{"permissions", checkFail, fmt.Sprintf("%d/%d write-all", bad, len(workflows))}
}

func checkDangerousTrigger(workflows map[string][]byte) checkResult {
	if len(workflows) == 0 {
		return checkResult{"dangerous-trigger", checkSkip, "no workflows"}
	}
	for name, body := range workflows {
		if prTargetRe.Match(body) && checkoutPRRe.Match(body) {
			return checkResult{"dangerous-trigger", checkFail, name + ": pull_request_target + PR checkout"}
		}
		if runInjectRe.Match(body) {
			return checkResult{"dangerous-trigger", checkFail, name + ": github.event in run:"}
		}
	}
	return checkResult{"dangerous-trigger", checkPass, ""}
}

func checkMaintained(d dep, repo map[string]interface{}, token string) checkResult {
	body, err := GetGithubBody(token, "https://api.github.com/repos/"+d.owner+"/"+d.repo+"/releases/latest")
	when := ""
	if err == nil {
		if m, _ := body.(map[string]interface{}); m != nil {
			when, _ = m["published_at"].(string)
		}
	}
	if when == "" {
		when, _ = repo["pushed_at"].(string)
	}
	if when == "" {
		return checkResult{"maintained", checkSkip, ""}
	}
	t, err := time.Parse(time.RFC3339, when)
	if err != nil {
		return checkResult{"maintained", checkSkip, ""}
	}
	days := int(time.Since(t).Hours() / 24)
	if days <= 365 {
		return checkResult{"maintained", checkPass, fmt.Sprintf("%dd", days)}
	}
	return checkResult{"maintained", checkFail, fmt.Sprintf("%dd", days)}
}

func checkAlive(repo map[string]interface{}) checkResult {
	if repo == nil {
		return checkResult{"alive", checkFail, "404"}
	}
	if archived, _ := repo["archived"].(bool); archived {
		return checkResult{"alive", checkFail, "archived"}
	}
	if disabled, _ := repo["disabled"].(bool); disabled {
		return checkResult{"alive", checkFail, "disabled"}
	}
	return checkResult{"alive", checkPass, ""}
}

// bucket folds a check set into ok / RISK / STALE.
func bucket(checks []checkResult) string {
	risk, stale := false, false
	for _, c := range checks {
		if c.outcome != checkFail {
			continue
		}
		switch checkSeverity[c.name] {
		case "risk":
			risk = true
		case "stale":
			stale = true
		}
	}
	switch {
	case risk:
		return "RISK"
	case stale:
		return "STALE"
	default:
		return "ok"
	}
}

func score(checks []checkResult) (pass, total int) {
	for _, c := range checks {
		if c.outcome == checkSkip {
			continue
		}
		total++
		if c.outcome == checkPass {
			pass++
		}
	}
	return
}
