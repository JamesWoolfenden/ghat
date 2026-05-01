package core

import (
	"os"
	"strings"
	"testing"
)

func TestParseCpanfile(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/cpan/cpanfile")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	deps := parseCpanfile(string(data))

	want := []string{"Text::Template", "JSON", "Test::More", "Capture::Tiny", "JSON::XS", "Already::Pinned", "Dist::From::Git"}
	if len(deps) != len(want) {
		t.Fatalf("parseCpanfile() got %d deps, want %d: %#v", len(deps), len(want), deps)
	}
	for i, w := range want {
		if deps[i].module != w {
			t.Errorf("dep[%d].module = %q, want %q", i, deps[i].module, w)
		}
	}
	if deps[0].quote != "'" || deps[1].quote != `"` {
		t.Errorf("quote capture wrong: %q %q", deps[0].quote, deps[1].quote)
	}
}

func TestRewriteCpanfile(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/cpan/cpanfile")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	pins := map[string]string{
		"Text::Template":  "1.61",
		"JSON":            "4.10",
		"Test::More":      "1.302199",
		"Capture::Tiny":   "9.99",
		"JSON::XS":        "4.03",
		"Already::Pinned": "9.99",
		"Dist::From::Git": "9.99",
	}
	got := rewriteCpanfile(string(data), pins)

	for _, want := range []string{
		"requires 'Text::Template', '== 1.61';",
		`requires "JSON", '== 4.10';`,
		"    requires 'Test::More', '== 1.302199';",
		"recommends 'JSON::XS', '== 4.03';",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rewriteCpanfile() missing %q in:\n%s", want, got)
		}
	}
	for _, keep := range []string{
		"requires 'Capture::Tiny'; # ghat:suppress",
		"requires 'Already::Pinned', '== 1.23';",
		"requires 'Dist::From::Git', git => 'https://example.com/foo.git';",
		"on 'test' => sub {",
	} {
		if !strings.Contains(got, keep) {
			t.Errorf("rewriteCpanfile() should preserve %q in:\n%s", keep, got)
		}
	}
	if strings.Contains(got, "Capture::Tiny', '== ") {
		t.Error("rewriteCpanfile() must not pin a suppressed entry")
	}
}

func TestUpdateCpanfile_NoFile(t *testing.T) {
	t.Parallel()

	flags := &Flags{Directory: t.TempDir(), DryRun: true}
	if err := flags.UpdateCpanfile(); err != nil {
		t.Errorf("UpdateCpanfile() with no cpanfile should be a no-op, got %v", err)
	}
}
