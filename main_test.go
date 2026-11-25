package main

import (
	"strings"
	"testing"

	"github.com/urfave/cli/v2"
)

func newGuardedApp(ran *bool) *cli.App {
	app := &cli.App{
		Commands: []*cli.Command{
			{
				Name: "swot",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "continue-on-error"},
					&cli.StringFlag{Name: "exclude"},
				},
				Action: func(*cli.Context) error { *ran = true; return nil },
			},
			{
				Name:   "lsp",
				Action: func(*cli.Context) error { *ran = true; return nil },
			},
			{
				Name: "cache",
				Subcommands: []*cli.Command{
					{Name: "clear", Action: func(*cli.Context) error { *ran = true; return nil }},
				},
			},
		},
	}
	rejectPositionalArgs(app.Commands)
	return app
}

func TestRejectPositionalArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"flags only", []string{"ghat", "swot", "--continue-on-error", "--exclude", "/example/"}, ""},
		{"bool flag given a value", []string{"ghat", "swot", "--continue-on-error", "true", "--exclude", "/example/"}, `unexpected argument(s) ["true" "--exclude" "/example/"]`},
		{"bool flag explicit form", []string{"ghat", "swot", "--continue-on-error=false"}, ""},
		{"subcommand stray arg", []string{"ghat", "cache", "clear", "oops"}, "unexpected argument(s)"},
		{"lsp is exempt", []string{"ghat", "lsp", "extra"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ran := false
			err := newGuardedApp(&ran).Run(tt.args)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if !ran {
					t.Fatal("action did not run")
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want containing %q", err, tt.wantErr)
			}
			if ran {
				t.Fatal("action ran despite stray arguments")
			}
		})
	}
}
