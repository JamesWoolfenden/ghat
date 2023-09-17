package core

import (
	"os"
	"reflect"
	"testing"
)

var gitHubToken = os.Getenv("GITHUB_TOKEN")

func TestGetBody(t *testing.T) {
	t.Parallel()

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
			gotMap := got.([]interface{})[0].(map[string]interface{})
			wanted := tt.want.(map[string]interface{})
			if !reflect.DeepEqual(gotMap["node_id"], wanted["node_id"]) {
				t.Errorf("GetGithubBody() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getHash(t *testing.T) {
	type args struct {
		action      string
		tag         string
		gitHubToken string
	}
	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		// TODO: Add test cases.
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
		days        *int
	}

	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		// TODO: Add test cases.
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
			if !reflect.DeepEqual(got, tt.want) {
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
		Days        int
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
		name    string
		fields  fields
		args    args
		want    []string
		wantErr bool
	}{
		{"no matches", duffDir, args{noMatches, nil}, nothing, false},
		{"no workflows", noWorkflowsDir, args{noWorkflows, nil}, nil, false},
		{"no workflows with dir", noWorkflowsWithDir, args{noWorkflowsWithDirContents, nil}, nil, false},
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
			got, err := myFlags.GetGHA()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetGHA() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
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
