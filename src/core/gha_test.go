package core

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/rs/zerolog/log"
)

var gitHubToken = os.Getenv("GITHUB_TOKEN")

func TestGetBody(t *testing.T) {
	t.Parallel()

	garbage := "guff-inhere"
	failUrl := "https://api.github.com/users/JamesWoolfenden2/orgs"
	url := "https://api.github.com/users/JamesWoolfenden/orgs"

	result := map[string]interface{}{
		"login":              "teamvulkan",
		"id":                 46164047,
		"node_id":            "MDEyOk9yZ2FuaXphdGlvbjQ2MTY0MDQ3",
		"url":                "https://api.github.com/orgs/teamvulkan",
		"repos_url":          "https://api.github.com/orgs/teamvulkan/repos",
		"events_url":         "https://api.github.com/orgs/teamvulkan/events",
		"hooks_url":          "https://api.github.com/orgs/teamvulkan/hooks",
		"issues_url":         "https://api.github.com/orgs/teamvulkan/issues",
		"members_url":        "https://api.github.com/orgs/teamvulkan/members{/member}",
		"public_members_url": "https://api.github.com/orgs/teamvulkan/public_members{/member}",
		"avatar_url":         "https://avatars.githubusercontent.com/u/46164047?v=4",
		"description":        "",
	}

	type args struct {
		gitHubToken string
		url         string
	}

	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		{"Pass", args{gitHubToken: gitHubToken, url: url}, result, false},
		{"Pass no token", args{url: url}, result, false},
		{"Fail 404", args{gitHubToken: gitHubToken, url: failUrl}, nil, true},
		{"Garbage", args{gitHubToken: gitHubToken, url: garbage}, nil, true},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := GetGithubBody(tt.args.gitHubToken, tt.args.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetGithubBody() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.want != nil {
				_, ok := got.([]interface{})
				if !ok {
					log.Info().Msgf("assertion error %s", err)
					return
				}

				gotMap := got.([]interface{})[0].(map[string]interface{})
				wanted := tt.want.(map[string]interface{})

				if !reflect.DeepEqual(gotMap["node_id"], wanted["node_id"]) {
					t.Errorf("GetGithubBody() got = %v, want %v", got, tt.want)
				}
				return
			}
			if got != nil {
				t.Errorf("GetGithubBody() nillness got = %v, want %v", got, tt.want)
			}

		})
	}
}

func Test_getHash(t *testing.T) {
	t.Parallel()

	type args struct {
		action      string
		tag         string
		gitHubToken string
	}

	want := map[string]interface{}{
		"node_id": "MDM6UmVmMTk3ODE0NjI5OnJlZnMvdGFncy92NC4wLjA=",
		"object": map[string]interface{}{
			"sha":  "1e31de5234b9f8995739874a8ce0492dc87873e2",
			"type": "commit",
			"url":  "https://api.github.com/repos/actions/checkout/git/commits/1e31de5234b9f8995739874a8ce0492dc87873e2",
		},
		"ref": "refs/tags/v4.0.0",
		"url": "https://api.github.com/repos/actions/checkout/git/refs/tags/v4.0.0",
	}

	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		{"pass", args{"actions/checkout", "v4.0.0", gitHubToken}, want, false},
		{"pass", args{"actions/checkout", "v4.0.999", gitHubToken}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getHash(tt.args.action, tt.args.tag, tt.args.gitHubToken)
			if (err != nil) != tt.wantErr {
				t.Errorf("getHash() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getHash() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getPayload(t *testing.T) {
	t.Parallel()

	type args struct {
		action      string
		gitHubToken string
		days        *uint
	}

	var days uint = 0
	var ninety uint = 90

	daysMap := map[string]interface{}{
		"html_url":         "https://github.com/JamesWoolfenden/action-pike/releases/tag/v0.1.3",
		"id":               81460196,
		"created_at":       "2022-10-29T11:25:25Z",
		"url":              "https://api.github.com/repos/JamesWoolfenden/action-pike/releases/81460196",
		"node_id":          "RE_kwDOIVF07c4E2vvk",
		"prerelease":       "false",
		"tarball_url":      "https://api.github.com/repos/JamesWoolfenden/action-pike/tarball/v0.1.3",
		"target_commitish": "master",
		"name":             "Initial Release",
		"zipball_url":      "https://api.github.com/repos/JamesWoolfenden/action-pike/zipball/v0.1.3",
		"assets_url":       "https://api.github.com/repos/JamesWoolfenden/action-pike/releases/81460196/assets",
		"upload_url":       "https://uploads.github.com/repos/JamesWoolfenden/action-pike/releases/81460196/assets{?name,label}",
		"tag_name":         "v0.1.3",
		"draft":            "false",
		"published_at":     "2022-10-29T15:17:57Z",
	}

	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		{"pass", args{"JamesWoolfenden/action-pike", gitHubToken, &days}, daysMap, false},
		{"pass", args{"JamesWoolfenden/action-pike", gitHubToken, &ninety}, daysMap, false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := getPayload(tt.args.action, tt.args.gitHubToken, tt.args.days)
			if (err != nil) != tt.wantErr {
				t.Errorf("getPayload() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			gotMap := got.(map[string]interface{})
			wantMap := tt.want.(map[string]interface{})

			if !reflect.DeepEqual(gotMap["created_at"], wantMap["created_at"]) {
				t.Errorf("getPayload() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFlags_GetGHA(t *testing.T) {
	type fields struct {
		File        string
		Directory   string
		GitHubToken string
		Days        uint
		DryRun      bool
	}

	type args struct {
		matches []os.DirEntry
		ghat    []os.DirEntry
	}

	duffDir := fields{"", "nothere", gitHubToken, 0, false}
	noMatches, _ := os.ReadDir(duffDir.Directory)

	noWorkflowsDir := fields{"", "./testdata/noworkflows", gitHubToken, 0, false}
	noWorkflows, _ := os.ReadDir(noWorkflowsDir.Directory)

	noWorkflowsWithDir := fields{"", "./testdata/noworkflowswithdir", gitHubToken, 0, false}
	noWorkflowsWithDirContents, _ := os.ReadDir(noWorkflowsWithDir.Directory)

	var nothing []string
	tests := []struct {
		name   string
		fields fields
		args   args
		want   []string
	}{
		{"no matches", duffDir, args{noMatches, nil}, nothing},
		{"no workflows", noWorkflowsDir, args{noWorkflows, nil}, nil},
		{"no workflows with dir", noWorkflowsWithDir, args{noWorkflowsWithDirContents, nil}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			myFlags := &Flags{
				File:        tt.fields.File,
				Directory:   tt.fields.Directory,
				GitHubToken: tt.fields.GitHubToken,
				Days:        tt.fields.Days,
				DryRun:      tt.fields.DryRun,
			}
			got := myFlags.GetGHA()

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetGHA() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetLatestTag(t *testing.T) {
	t.Parallel()
	type args struct {
		action      string
		gitHubToken string
	}

	latest := "34bf44973c4f415bd3e791728b630e5d110a2244"

	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{"Pass", args{"jameswoolfenden/terraform-azurerm-diskencryptionset", gitHubToken}, latest, false},
		{"Fail", args{"jameswoolfenden/terraform-azurerm-guff", gitHubToken}, "", true},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := GetLatestTag(tt.args.action, tt.args.gitHubToken)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetLatestTag() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got == nil && tt.want != "" {
				t.Errorf("GetLatestTag() got = nil, want %v", tt.want)
				return
			}

			if (got == nil) == (tt.want == "") {
				return
			}

			returned := got.(map[string]interface{})
			commit := returned["commit"].(map[string]interface{})
			hash := commit["sha"].(string)
			if hash != tt.want {
				t.Errorf("GetLatestTag() got = %v, want %v", hash, tt.want)
			}
		})
	}
}

func TestFlags_UpdateGHAS(t *testing.T) {
	t.Parallel()

	type fields struct {
		File        string
		Directory   string
		GitHubToken string
		Days        uint
		DryRun      bool
		Entries     []string
		Update      bool
	}

	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{"Pass file",
			fields{"./testdata/gha/.github/workflows/test.yml", "", gitHubToken, 0, true, []string{"./testdata/gha/.github/workflows/test.yml"}, true}, false},
		{"Pass file not dry",
			fields{"./testdata/gha/.github/workflows/test.yml", "", gitHubToken, 0, false, []string{"./testdata/gha/.github/workflows/test.yml"}, true}, false},
		{"Pass dir",
			fields{"", "./testdata/gha/.github/workflows", gitHubToken, 0, true, []string{"./testdata/gha/.github/workflows/test.yml"}, true}, false},
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
			if err := myFlags.UpdateGHAS(); (err != nil) != tt.wantErr {
				t.Errorf("UpdateGHAS() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFlags_UpdateGHA(t *testing.T) {
	t.Parallel()
	type fields struct {
		File            string
		Directory       string
		GitHubToken     string
		Days            uint
		DryRun          bool
		Entries         []string
		Update          bool
		ContinueOnError bool
	}

	type args struct {
		file string
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{name: "Pass file",
			fields: fields{File: "./testdata/gha/.github/workflows/test.yml", GitHubToken: gitHubToken, DryRun: true, Entries: []string{"./testdata/gha/.github/workflows/test.yml"}, Update: true},
			args:   args{"./testdata/gha/.github/workflows/test.yml"}},
		{name: "No such file",
			fields:  fields{File: "./testdata/gha/.github/workflows/guff.yml", GitHubToken: gitHubToken, DryRun: true, Entries: []string{"./testdata/gha/.github/workflows/test.yml"}, Update: true},
			args:    args{"./testdata/gha/.github/workflows/guff.yml"},
			wantErr: true},
		{name: "Faulty GHA",
			fields:  fields{File: "./testdata/faulty/.github/workflows/test.yml", GitHubToken: gitHubToken, DryRun: true, Entries: []string{"./testdata/faulty/.github/workflows/test.yml"}, Update: true},
			args:    args{file: "./testdata/faulty/.github/workflows/test.yml"},
			wantErr: true},
		{name: "Faulty GHA continue",
			fields: fields{File: "./testdata/faulty/.github/workflows/test.yml", GitHubToken: gitHubToken, DryRun: true, Entries: []string{"./testdata/faulty/.github/workflows/test.yml"}, Update: true, ContinueOnError: true},
			args:   args{file: "./testdata/faulty/.github/workflows/test.yml"}},
		{
			name: "Empty entries",
			fields: fields{
				Entries:     []string{},
				GitHubToken: gitHubToken,
			},
			wantErr: true,
		},
		{
			name: "Invalid file path",
			fields: fields{
				Entries:     []string{"./testdata/nonexistent/workflow.yml"},
				GitHubToken: gitHubToken,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			myFlags := &Flags{
				File:            tt.fields.File,
				Directory:       tt.fields.Directory,
				GitHubToken:     tt.fields.GitHubToken,
				Days:            tt.fields.Days,
				DryRun:          tt.fields.DryRun,
				Entries:         tt.fields.Entries,
				Update:          tt.fields.Update,
				ContinueOnError: tt.fields.ContinueOnError,
			}
			if err := myFlags.UpdateGHA(tt.args.file); (err != nil) != tt.wantErr {
				t.Errorf("UpdateGHA() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func setupSuite(tb testing.TB) func(tb testing.TB) {
	log.Info().Msgf("setup suite %s", tb.Name())
	testPath, _ := filepath.Abs("./testdata/empty")
	_ = os.Mkdir(testPath, os.ModePerm)
	_ = os.Mkdir("./testdata/.terraform/", os.ModePerm)
	_ = os.Mkdir("./testdata/.git/", os.ModePerm)

	return func(tb testing.TB) {
		log.Info().Msg("teardown suite")
		_ = os.RemoveAll(testPath)
		_ = os.RemoveAll("./testdata/.terraform/")
		_ = os.RemoveAll("./testdata/.git/")

	}
}

func TestGetFiles(t *testing.T) {
	t.Parallel()

	//teardownSuite := setupSuite(t)
	//defer teardownSuite(t)

	tests := []struct {
		name    string
		dir     string
		want    int
		wantErr bool
	}{
		{"Valid directory", "./testdata/gha", 1, false},
		{"Empty directory", "./testdata/empty", 0, false},
		{"Non-existent directory", "./testdata/nonexistent", 0, true},
		{"Directory with .terraform", "./testdata/.terraform", 0, false},
		{"Directory with .git", "./testdata/.git", 0, false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			teardownSuite := setupSuite(t)
			defer teardownSuite(t)
			got, err := GetFiles(tt.dir)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetFiles() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(got) != tt.want {
				t.Errorf("GetFiles() got = %v files, want %v", len(got), tt.want)
			}
		})
	}
}

func TestReadFilesError(t *testing.T) {
	t.Parallel()

	testErr := fmt.Errorf("test error")
	err := &readFilesError{err: testErr}
	expected := "failed to read files: test error"

	if err.Error() != expected {
		t.Errorf("readFilesError.Error() = %v, want %v", err.Error(), expected)
	}
}

func TestAbsolutePathError(t *testing.T) {
	t.Parallel()

	testErr := fmt.Errorf("test error")
	testDir := "/test/dir"
	err := &absolutePathError{directory: testDir, err: testErr}
	expected := "failed to get absolute path: test error /test/dir "

	if err.Error() != expected {
		t.Errorf("absolutePathError.Error() = %v, want %v", err.Error(), expected)
	}
}

func TestGetGithubBody_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		gitHubToken string
		url         string
		wantErr     bool
	}{
		{
			name:        "Invalid URL format",
			gitHubToken: gitHubToken,
			url:         "not-a-url",
			wantErr:     true,
		},
		{
			name:        "Empty URL",
			gitHubToken: gitHubToken,
			url:         "",
			wantErr:     true,
		},
		{
			name:        "Invalid JSON response",
			gitHubToken: gitHubToken,
			url:         "https://api.github.com/invalid-endpoint",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := GetGithubBody(tt.gitHubToken, tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetGithubBody() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetPayload_ErrorCases(t *testing.T) {
	t.Parallel()

	var days uint = 30
	tests := []struct {
		name        string
		action      string
		gitHubToken string
		days        *uint
		wantErr     bool
	}{
		{
			name:        "Empty action",
			action:      "",
			gitHubToken: gitHubToken,
			days:        &days,
			wantErr:     true,
		},
		{
			name:        "Invalid action format",
			action:      "invalid-format",
			gitHubToken: gitHubToken,
			days:        &days,
			wantErr:     true,
		},
		{
			name:        "Nil days pointer",
			action:      "actions/checkout",
			gitHubToken: gitHubToken,
			days:        nil,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := getPayload(tt.action, tt.gitHubToken, tt.days)
			if (err != nil) != tt.wantErr {
				t.Errorf("getPayload() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
