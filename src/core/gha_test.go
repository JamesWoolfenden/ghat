package core

import (
	"os"
	"reflect"
	"testing"
)

var gitHubToken = os.Getenv("GITHUB_TOKEN")

func TestGetBody(t *testing.T) {
	t.Parallel()

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
		// TODO: Add test cases.
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
			if !reflect.DeepEqual(got, tt.want) {
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

//
//func TestFlags_Files(t *testing.T) {
//
//	type fields struct {
//		File        string
//		Directory   string
//		GitHubToken string
//		Days        int
//		DryRun      bool
//	}
//
//	dir := fields{"", "testdata/files/", gitHubToken, 0, false}
//	bogus := fields{"", "testdata/bogus/", gitHubToken, 0, false}
//	empty := fields{"", "testdata/empty", gitHubToken, 0, false}
//	dirDry := fields{"", "testdata/files/", gitHubToken, 0, true}
//
//	tests := []struct {
//		name    string
//		fields  fields
//		want    []os.DirEntry
//		wantErr bool
//	}{
//		{"Pass", dir, nil, false},
//		{"Bogus", bogus, nil, false},
//		{"Empty", empty, nil, false},
//		{"dirDry", dirDry, nil, false},
//	}
//
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			myFlags := &Flags{
//				File:        tt.fields.File,
//				Directory:   tt.fields.Directory,
//				GitHubToken: tt.fields.GitHubToken,
//				Days:        tt.fields.Days,
//				DryRun:      tt.fields.DryRun,
//			}
//			got, err := myFlags.Action()
//			if (err != nil) != tt.wantErr {
//				t.Errorf("Action() error = %v, wantErr %v", err, tt.wantErr)
//				return
//			}
//			if !reflect.DeepEqual(got, tt.want) {
//				t.Errorf("Action() got = %v, want %v", got, tt.want)
//			}
//		})
//	}
//}
//
//func TestFlags_GetGHA(t *testing.T) {
//	type fields struct {
//		File        string
//		Directory   string
//		GitHubToken string
//		Days        int
//		DryRun      bool
//	}
//
//	type args struct {
//		matches []os.DirEntry
//		ghat    []os.DirEntry
//	}
//
//	duffDir := fields{"", "nothere", gitHubToken, 0, false}
//	noMatches, _ := os.ReadDir(duffDir.Directory)
//
//	noWorkflowsDir := fields{"", "./testdata/noworkflows", gitHubToken, 0, false}
//	noWorkflows, _ := os.ReadDir(noWorkflowsDir.Directory)
//
//	noWorkflowsWithDir := fields{"", "./testdata/noworkflowswithdir", gitHubToken, 0, false}
//	noWorkflowsWithDirContents, _ := os.ReadDir(noWorkflowsWithDir.Directory)
//
//	tests := []struct {
//		name    string
//		fields  fields
//		args    args
//		want    []os.DirEntry
//		wantErr bool
//	}{
//		{"no matches", duffDir, args{noMatches, nil}, nil, false},
//		{"no workflows", noWorkflowsDir, args{noWorkflows, nil}, nil, false},
//		{"no workflows with dir", noWorkflowsWithDir, args{noWorkflowsWithDirContents, nil}, nil, false},
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			myFlags := &Flags{
//				File:        tt.fields.File,
//				Directory:   tt.fields.Directory,
//				GitHubToken: tt.fields.GitHubToken,
//				Days:        tt.fields.Days,
//				DryRun:      tt.fields.DryRun,
//			}
//			got, err := myFlags.GetGHA(tt.args.matches, tt.args.ghat)
//			if (err != nil) != tt.wantErr {
//				t.Errorf("GetGHA() error = %v, wantErr %v", err, tt.wantErr)
//				return
//			}
//			if !reflect.DeepEqual(got, tt.want) {
//				t.Errorf("GetGHA() got = %v, want %v", got, tt.want)
//			}
//		})
//	}
//}
//
//func TestFlags_UpdateFile(t *testing.T) {
//	type fields struct {
//		File        string
//		Directory   string
//		GitHubToken string
//		Days        int
//		DryRun      bool
//	}
//	tests := []struct {
//		name    string
//		fields  fields
//		wantErr bool
//	}{
//		// TODO: Add test cases.
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			myFlags := &Flags{
//				File:        tt.fields.File,
//				Directory:   tt.fields.Directory,
//				GitHubToken: tt.fields.GitHubToken,
//				Days:        tt.fields.Days,
//				DryRun:      tt.fields.DryRun,
//			}
//			if err := myFlags.UpdateGHA(); (err != nil) != tt.wantErr {
//				t.Errorf("UpdateGHA() error = %v, wantErr %v", err, tt.wantErr)
//			}
//		})
//	}
//}

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
			returned := got.(map[string]interface{})
			commit := returned["commit"].(map[string]interface{})
			hash := commit["sha"].(string)
			if hash != tt.want {
				t.Errorf("GetLatestTag() got = %v, want %v", hash, tt.want)
			}
		})
	}
}
