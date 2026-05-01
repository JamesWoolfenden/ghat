package core

import "strings"

const (
	SuppressAnnotation = "# ghat:suppress"
	suppressReasonKey  = "reason="
)

// parseSuppression checks whether a line carries the ghat:suppress annotation
// and returns (suppressed, reason). The reason is the value of the
// "reason=" key if present, e.g.:
//
//	# ghat:suppress:reason=tag pinning is our design choice
func parseSuppression(line string) (bool, string) {
	idx := strings.Index(line, SuppressAnnotation)
	if idx < 0 {
		return false, ""
	}
	rest := strings.TrimSpace(line[idx+len(SuppressAnnotation):])
	// strip leading colon separator, if any
	rest = strings.TrimPrefix(rest, ":")
	for _, field := range strings.Split(rest, ":") {
		if val, ok := strings.CutPrefix(field, suppressReasonKey); ok {
			return true, strings.TrimSpace(val)
		}
	}
	return true, ""
}

// isSuppressed reports whether a line carries the ghat:suppress annotation.
func isSuppressed(line string) bool {
	ok, _ := parseSuppression(line)
	return ok
}

// imageLineSuppression returns (suppressed, reason) for the first line in
// content that contains imageStr and carries the ghat:suppress annotation.
// Used for YAML-based processors (stun, kube) where comment stripping happens
// before image extraction.
func imageLineSuppression(content, imageStr string) (bool, string) {
	for _, line := range strings.Split(content, "\n") {
		if strings.Contains(line, imageStr) {
			if ok, reason := parseSuppression(line); ok {
				return true, reason
			}
		}
	}
	return false, ""
}
