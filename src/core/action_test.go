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
		Days        int
		DryRun      bool
		Entries     []string
		Update      bool
	}

	type args struct {
		Action string
	}

	dir := fields{"", "testdata/files/", gitHubToken, 0, false, nil, true}
	bogus := fields{"", "testdata/bogus/", gitHubToken, 0, false, nil, true}
	empty := fields{"", "testdata/empty", gitHubToken, 0, false, nil, true}
	dirDry := fields{"", "testdata/files/", gitHubToken, 0, true, nil, true}
	fileGHA := fields{"testdata/files/ci.yml", "testdata/files/", gitHubToken, 0, true, nil, true}
	file := fields{"testdata/files/module.tf", "testdata/files/", gitHubToken, 0, true, nil, true}
	noFile := fields{"testdata/files/guff.tf", "testdata/files/", gitHubToken, 0, true, nil, true}

	_ = os.Remove("testdata/empty")

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{"Pass", dir, args{}, false},
		{"Bogus", bogus, args{}, true},
		{"Empty swot", empty, args{"swot"}, true},
		{"Empty swipe", empty, args{"swipe"}, true},
		{"dirDry", dirDry, args{}, false},
		{"file swipe", file, args{"swipe"}, false},
		{"file swot", fileGHA, args{"swot"}, false},
		{"file swot empty", dirDry, args{"swot"}, false},
		{"file swipe empty", dirDry, args{"swipe"}, false},
		{"no file", noFile, args{"swipe"}, true},
		{"sift", fields{Directory: "../../"}, args{"sift"}, false},
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
			}
			if err := myFlags.Action(tt.args.Action); (err != nil) != tt.wantErr {
				t.Errorf("Action() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
