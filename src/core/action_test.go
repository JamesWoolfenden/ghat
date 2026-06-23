package core

import (
	"os"
	"testing"
)

func TestFlags_Action(t *testing.T) {
	t.Parallel()
	type fields struct {
		File        string
		Directory   string
		GitHubToken string
		Days        *uint
		DryRun      bool
		Entries     []string
		Update      bool
		Exclude     string
	}

	type args struct {
		Action string
	}

	var days uint = 0

	dir := fields{"", "testdata/files/", gitHubToken, &days, false, nil, true, ""}
	bogus := fields{"", "testdata/bogus/", gitHubToken, &days, false, nil, true, ""}
	empty := fields{"", "testdata/empty", gitHubToken, &days, false, nil, true, ""}
	dirDry := fields{"", "testdata/files/", gitHubToken, &days, true, nil, true, ""}
	fileGHA := fields{"testdata/files/ci.yml", "testdata/files/", gitHubToken, &days, true, nil, true, ""}
	file := fields{"testdata/files/module.tf", "testdata/files/", gitHubToken, &days, true, nil, true, ""}
	noFile := fields{"testdata/files/guff.tf", "testdata/files/", gitHubToken, &days, true, nil, true, ""}
	excludeAll := fields{"", "testdata/files/", gitHubToken, &days, true, nil, true, ".*"}

	_ = os.Remove("testdata/empty")

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{"Pass", dir, args{}, true},
		{"Bogus", bogus, args{}, true},
		{"Empty swot", empty, args{"swot"}, true},
		{"Empty swipe", empty, args{"swipe"}, true},
		{"dirDry", dirDry, args{}, true},
		{"file swipe", file, args{"swipe"}, false},
		{"file swot", fileGHA, args{"swot"}, false},
		{"file swot empty", dirDry, args{"swot"}, false},
		{"file swipe empty", dirDry, args{"swipe"}, false},
		{"no file", noFile, args{"swipe"}, true},
		{"sift", fields{Directory: "../../"}, args{"sift"}, false},
		{"exclude everything still scans ok", excludeAll, args{"swot"}, false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			myFlags := &Flags{
				File:        tt.fields.File,
				Directory:   tt.fields.Directory,
				GitHubToken: tt.fields.GitHubToken,
				Days:        tt.fields.Days,
				DryRun:      tt.fields.DryRun,
				Entries:     tt.fields.Entries,
				Update:      tt.fields.Update,
				Exclude:     tt.fields.Exclude,
			}
			if err := myFlags.Action(tt.args.Action); (err != nil) != tt.wantErr {
				t.Errorf("Action() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFilterExcluded(t *testing.T) {
	t.Parallel()

	entries := []string{
		"tests/fixtures/ci.yml",
		".github/workflows/ci.yml",
		"module.tf",
	}

	got, err := filterExcluded(entries, "^tests/fixtures/")
	if err != nil {
		t.Fatalf("filterExcluded() error = %v", err)
	}

	want := []string{".github/workflows/ci.yml", "module.tf"}
	if len(got) != len(want) {
		t.Fatalf("filterExcluded() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("filterExcluded()[%d] = %q, want %q", i, got[i], want[i])
		}
	}

	if _, err := filterExcluded(entries, "(("); err == nil {
		t.Error("filterExcluded() with invalid regex: want error, got nil")
	}
}

func TestExecuteAction(t *testing.T) {
	t.Parallel()
	type args struct {
		action  string
		myFlags *Flags
	}

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Unknown action type",
			args: args{
				action: "unknown",
				myFlags: &Flags{
					File:        "",
					Directory:   "testdata/files/",
					GitHubToken: gitHubToken,
				},
			},
			wantErr: false,
		},
		{
			name: "Swipe with nil flags",
			args: args{
				action:  ActionSwipe,
				myFlags: nil,
			},
			wantErr: true,
		},
		{
			name: "Swot with empty file and directory",
			args: args{
				action: ActionSwot,
				myFlags: &Flags{
					File:        "",
					Directory:   "",
					GitHubToken: "",
				},
			},
			wantErr: true,
		},
		{
			name: "Sift with no pre-commit config",
			args: args{
				action: ActionSift,
				myFlags: &Flags{
					File:        "",
					Directory:   "testdata/files/",
					GitHubToken: "",
				},
			},
			wantErr: false,
		},
		{
			name: "Swipe with invalid file path format",
			args: args{
				action: ActionSwipe,
				myFlags: &Flags{
					File:        "///",
					Directory:   "testdata/files/",
					GitHubToken: gitHubToken,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := executeAction(tt.args.action, tt.args.myFlags); (err != nil) != tt.wantErr {
				t.Errorf("executeAction() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
