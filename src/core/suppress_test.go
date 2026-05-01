package core

import "testing"

func TestParseSuppression(t *testing.T) {
	tests := []struct {
		line       string
		suppressed bool
		reason     string
	}{
		{
			line:       "- uses: actions/checkout@v4  # ghat:suppress",
			suppressed: true,
			reason:     "",
		},
		{
			line:       "- uses: actions/checkout@v4  # ghat:suppress:reason=tag pinning is our design choice",
			suppressed: true,
			reason:     "tag pinning is our design choice",
		},
		{
			line:       "  image: nginx:1.25  # ghat:suppress:reason=version locked by compliance team",
			suppressed: true,
			reason:     "version locked by compliance team",
		},
		{
			line:       "FROM ubuntu:22.04  # ghat:suppress:reason=base image frozen",
			suppressed: true,
			reason:     "base image frozen",
		},
		{
			line:       "- uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v4",
			suppressed: false,
			reason:     "",
		},
		{
			line:       "- uses: actions/checkout@v4",
			suppressed: false,
			reason:     "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			got, reason := parseSuppression(tt.line)
			if got != tt.suppressed {
				t.Errorf("suppressed = %v, want %v", got, tt.suppressed)
			}
			if reason != tt.reason {
				t.Errorf("reason = %q, want %q", reason, tt.reason)
			}
		})
	}
}

func TestIsImageLineSuppressed(t *testing.T) {
	content := `
jobs:
  test:
    container:
      image: nginx:1.25  # ghat:suppress:reason=version locked by compliance team
    services:
      db:
        image: postgres:16
`
	ok, reason := imageLineSuppression(content, "nginx:1.25")
	if !ok {
		t.Error("nginx:1.25 should be suppressed")
	}
	if reason != "version locked by compliance team" {
		t.Errorf("reason = %q, want %q", reason, "version locked by compliance team")
	}

	ok, _ = imageLineSuppression(content, "postgres:16")
	if ok {
		t.Error("postgres:16 should not be suppressed")
	}
}

func TestFindUnpinnedSuppressed(t *testing.T) {
	body := `
      - uses: actions/checkout@v4  # ghat:suppress:reason=tag pinning is our design choice
      - uses: actions/setup-go@v5
      - uses: actions/cache@de0fac2e4500dabe0009e67214ff5f5447ce83dd
`
	rs := findUnpinned([]byte(body))
	if rs.suppressed != 1 {
		t.Errorf("suppressed = %d, want 1", rs.suppressed)
	}
	if len(rs.unpinned) != 1 || rs.unpinned[0] != "actions/setup-go@v5" {
		t.Errorf("unpinned = %v, want [actions/setup-go@v5]", rs.unpinned)
	}
	if rs.total != 2 {
		t.Errorf("total = %d, want 2", rs.total)
	}
}
