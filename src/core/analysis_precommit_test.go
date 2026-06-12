package core

import "testing"

func TestAnalyzePreCommit(t *testing.T) {
	content := []byte(`repos:
  - repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v4.5.0
    hooks:
      - id: trailing-whitespace
  - repo: https://github.com/JamesWoolfenden/ghat
    rev: de0fac2e4500dabe0009e67214ff5f5447ce83dd # v0.1.32
    hooks:
      - id: ghat
  - repo: local
    hooks:
      - id: noop
  - repo: https://github.com/psf/black  # ghat:suppress
    rev: 24.1.0
`)
	a := AnalyzePreCommit(content)
	if len(a.Repos) != 3 {
		t.Fatalf("expected 3 repos (local skipped), got %d: %+v", len(a.Repos), a.Repos)
	}
	if a.Repos[0].Repo != "https://github.com/pre-commit/pre-commit-hooks" || a.Repos[0].IsSHAPinned || a.Repos[0].Line != 3 {
		t.Errorf("repos[0] = %+v, want unpinned at line 3", a.Repos[0])
	}
	if !a.Repos[1].IsSHAPinned || a.Repos[1].Line != 7 {
		t.Errorf("repos[1] = %+v, want pinned at line 7", a.Repos[1])
	}
	if !a.Repos[2].Suppressed || a.Repos[2].Line != 14 {
		t.Errorf("repos[2] = %+v, want suppressed at line 14", a.Repos[2])
	}
}
