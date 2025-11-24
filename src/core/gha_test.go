package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

			// Handle rate limit errors for anonymous requests (no token)
			if err != nil && tt.args.gitHubToken == "" && strings.Contains(err.Error(), "rate limit") {
				t.Skipf("Skipping test due to GitHub API rate limit (expected for anonymous requests): %v", err)
				return
			}

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
		Days        *uint
		DryRun      bool
	}

	var days uint = 0

	type args struct {
		matches []os.DirEntry
		ghat    []os.DirEntry
	}

	duffDir := fields{"", "nothere", gitHubToken, &days, false}
	noMatches, _ := os.ReadDir(duffDir.Directory)

	noWorkflowsDir := fields{"", "./testdata/noworkflows", gitHubToken, &days, false}
	noWorkflows, _ := os.ReadDir(noWorkflowsDir.Directory)

	noWorkflowsWithDir := fields{"", "./testdata/noworkflowswithdir", gitHubToken, &days, false}
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
		Days        *uint
		DryRun      bool
		Entries     []string
		Update      bool
	}

	var days uint = 0

	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{"Pass file",
			fields{"./testdata/gha/.github/workflows/test.yml", "", gitHubToken, &days, true, []string{"./testdata/gha/.github/workflows/test.yml"}, true}, false},
		{"Pass file not dry",
			fields{"./testdata/gha/.github/workflows/test.yml", "", gitHubToken, &days, false, []string{"./testdata/gha/.github/workflows/test.yml"}, true}, false},
		{"Pass dir",
			fields{"", "./testdata/gha/.github/workflows", gitHubToken, &days, true, []string{"./testdata/gha/.github/workflows/test.yml"}, true}, false},
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
		Days            *uint
		DryRun          bool
		Entries         []string
		Update          bool
		ContinueOnError bool
	}

	//var days uint = 0

	type args struct {
		file string
	}

	var days uint = 0

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{name: "Pass file",
			fields: fields{File: "./testdata/gha/.github/workflows/test.yml", GitHubToken: gitHubToken, Days: &days, DryRun: true, Entries: []string{"./testdata/gha/.github/workflows/test.yml"}, Update: true},
			args:   args{"./testdata/gha/.github/workflows/test.yml"}},
		{name: "Zero file",
			fields:  fields{File: "./testdata/gha/.github/workflows/test.yml", GitHubToken: gitHubToken, Days: nil, DryRun: true, Entries: []string{"./testdata/gha/.github/workflows/test.yml"}, Update: true},
			args:    args{"./testdata/gha/.github/workflows/test.yml"},
			wantErr: true},
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
	type args struct {
		directory string
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
		setup   func(t *testing.T) string // Setup function to create test directories
	}{
		{
			name: "Empty directory",
			setup: func(t *testing.T) string {
				// Create a temporary empty directory
				tmpDir := t.TempDir()
				emptyDir := filepath.Join(tmpDir, "empty")
				err := os.MkdirAll(emptyDir, 0755)
				if err != nil {
					t.Fatalf("Failed to create empty directory: %v", err)
				}
				return emptyDir
			},
			want:    []string{},
			wantErr: false,
		},
		{
			name: "Directory with files",
			setup: func(t *testing.T) string {
				// Create a temporary directory with some files
				tmpDir := t.TempDir()
				testDir := filepath.Join(tmpDir, "withfiles")
				err := os.MkdirAll(testDir, 0755)
				if err != nil {
					t.Fatalf("Failed to create test directory: %v", err)
				}

				// Create some test files
				files := []string{"file1.txt", "file2.yaml", "file3.yml"}
				for _, filename := range files {
					filePath := filepath.Join(testDir, filename)
					err := os.WriteFile(filePath, []byte("test content"), 0644)
					if err != nil {
						t.Fatalf("Failed to create test file %s: %v", filename, err)
					}
				}
				return testDir
			},
			want:    nil, // We'll check count instead of exact list
			wantErr: false,
		},
		{
			name: "Nonexistent directory",
			setup: func(t *testing.T) string {
				tmpDir := t.TempDir()
				return filepath.Join(tmpDir, "nonexistent")
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Directory with subdirectories",
			setup: func(t *testing.T) string {
				tmpDir := t.TempDir()
				testDir := filepath.Join(tmpDir, "nested")

				// Create nested structure
				subDir := filepath.Join(testDir, "subdir")
				err := os.MkdirAll(subDir, 0755)
				if err != nil {
					t.Fatalf("Failed to create nested directory: %v", err)
				}

				// Create files in both directories
				err = os.WriteFile(filepath.Join(testDir, "root.txt"), []byte("root"), 0644)
				if err != nil {
					t.Fatalf("Failed to create root file: %v", err)
				}

				err = os.WriteFile(filepath.Join(subDir, "nested.txt"), []byte("nested"), 0644)
				if err != nil {
					t.Fatalf("Failed to create nested file: %v", err)
				}

				return testDir
			},
			want:    nil,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup the test directory
			var directory string
			if tt.setup != nil {
				directory = tt.setup(t)
			} else {
				directory = tt.args.directory
			}

			got, err := GetFiles(directory)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetFiles() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				// If we expect an error, don't check the result
				return
			}

			// For empty directory test
			if tt.name == "Empty directory" {
				if len(got) != 0 {
					t.Errorf("GetFiles() for empty directory returned %d files, want 0", len(got))
				}
				return
			}

			// For directories with files, just verify we got some files back
			if tt.name == "Directory with files" {
				if len(got) == 0 {
					t.Errorf("GetFiles() returned no files, expected some files")
				}
				t.Logf("Found %d files: %v", len(got), got)
			}

			// For nested directories, verify we got files from subdirectories too
			if tt.name == "Directory with subdirectories" {
				if len(got) < 2 {
					t.Errorf("GetFiles() returned %d files, expected at least 2 (root and nested)", len(got))
				}
				t.Logf("Found %d files in nested structure: %v", len(got), got)
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

func TestIsRateLimitError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "Nil error",
			err:  nil,
			want: false,
		},
		{
			name: "Rate limit exceeded message",
			err:  errors.New("GitHub API rate limit exceeded"),
			want: true,
		},
		{
			name: "403 status code",
			err:  errors.New("api failed with 403"),
			want: true,
		},
		{
			name: "429 status code",
			err:  errors.New("api failed with 429: Too Many Requests"),
			want: true,
		},
		{
			name: "Generic rate limit message",
			err:  errors.New("rate limit exceeded"),
			want: true,
		},
		{
			name: "Non-rate-limit error",
			err:  errors.New("network timeout"),
			want: false,
		},
		{
			name: "404 error",
			err:  errors.New("api failed with 404: Not Found"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRateLimitError(tt.err); got != tt.want {
				t.Errorf("isRateLimitError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetReleases_RateLimitHandling(t *testing.T) {
	// This test verifies the structure of rate limit handling
	// Without actually hitting the API

	t.Run("nil days parameter", func(t *testing.T) {
		_, err := GetReleases("actions/checkout", "", nil)
		if err == nil {
			t.Error("Expected error for nil days parameter")
		}
		if !strings.Contains(err.Error(), "days") {
			t.Errorf("Expected error about days parameter, got: %v", err)
		}
	})

	t.Run("empty action", func(t *testing.T) {
		var days uint = 0
		_, err := GetReleases("", "", &days)
		if err == nil {
			t.Error("Expected error for empty action")
		}
		if !strings.Contains(err.Error(), "action") {
			t.Errorf("Expected error about action, got: %v", err)
		}
	})

	t.Run("no token warning", func(t *testing.T) {
		// This should log a warning but not error
		// We can't easily test the log output, but we can verify
		// the function handles empty tokens gracefully
		var days uint = 0

		// This will likely fail due to rate limits or network,
		// but it shouldn't panic or return a token-specific error
		_, err := GetReleases("actions/checkout", "", &days)

		// We expect either success or a rate limit / network error
		// But NOT a token validation error
		if err != nil {
			errStr := err.Error()
			if strings.Contains(strings.ToLower(errStr), "token is empty") ||
				strings.Contains(strings.ToLower(errStr), "invalid token") {
				t.Errorf("Should not error on empty token, got: %v", err)
			}
		}
	})
}
