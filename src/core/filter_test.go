package core

import (
	"testing"
)

func TestGetReleases(t *testing.T) {
	t.Parallel()

	type args struct {
		action      string
		gitHubToken string
		delay       *uint
	}

	var delay uint = 14
	var zero uint = 0
	var empty map[string]interface{}
	want := map[string]interface{}{
		"tarball_url":  "https: //api.github.com/repos/JamesWoolfenden/test-data-action/tarball/v0.0.1",
		"zipball_url":  "https: //api.github.com/repos/JamesWoolfenden/test-data-action/zipball/v0.0.1",
		"assets_url":   "https:  //api.github.com/repos/JamesWoolfenden/test-data-action/releases/109328421/assets",
		"id":           109328421,
		"draft":        false,
		"created_at":   "2023-06-21T06:59:22Z",
		"published_at": "2023-06-21T06:59:51Z",
		"assets":       []interface{}{},
		"html_url":     "https: //github.com/JamesWoolfenden/test-data-action/releases/tag/v0.0.1",
		"author": map[string]interface{}{
			"avatar_url":          "https://avatars.githubusercontent.com/u/1456880?v=4",
			"url":                 "https://api.github.com/users/JamesWoolfenden",
			"type":                "User",
			"followers_url":       "https://api.github.com/users/JamesWoolfenden/followers",
			"organizations_url":   "https://api.github.com/users/JamesWoolfenden/orgs",
			"starred_url":         "https://api.github.com/users/JamesWoolfenden/starred{/owner}{/repo}",
			"events_url":          "https://api.github.com/users/JamesWoolfenden/events{/privacy}",
			"login":               "JamesWoolfenden",
			"id":                  1456880,
			"node_id":             "MDQ6VXNlcjE0NTY4ODA=",
			"gravatar_id":         "",
			"html_url":            "https://github.com/JamesWoolfenden",
			"following_url":       "https://api.github.com/users/JamesWoolfenden/following{/other_user}",
			"gists_url":           "https://api.github.com/users/JamesWoolfenden/gists{/gist_id}",
			"subscriptions_url":   "https://api.github.com/users/JamesWoolfenden/subscriptions",
			"repos_url":           "https://api.github.com/users/JamesWoolfenden/repos",
			"received_events_url": "https://api.github.com/users/JamesWoolfenden/received_events",
			"site_admin":          false,
		},
		"node_id":          "RE_kwDOJyIXLs4GhDgl",
		"tag_name":         "v0.0.1",
		"name":             "Test",
		"prerelease":       false,
		"body":             "",
		"url":              "https://api.github.com/repos/JamesWoolfenden/test-data-action/releases/109328421",
		"upload_url":       "https: //uploads.github.com/repos/JamesWoolfenden/test-data-action/releases/109328421/assets{?name,label}",
		"target_commitish": "main",
	}

	result := map[string]interface{}{
		"tarball_url":  "https: //api.github.com/repos/JamesWoolfenden/test-data-action/tarball/v0.0.1",
		"zipball_url":  "https: //api.github.com/repos/JamesWoolfenden/test-data-action/zipball/v0.0.1",
		"assets_url":   "https:  //api.github.com/repos/JamesWoolfenden/test-data-action/releases/109328421/assets",
		"id":           109328421,
		"draft":        false,
		"created_at":   "2023-06-21T06:59:22Z",
		"published_at": "2023-06-21T06:59:51Z",
		"assets":       []interface{}{},
		"html_url":     "https: //github.com/JamesWoolfenden/test-data-action/releases/tag/v0.0.1",
		"author": map[string]interface{}{
			"avatar_url":          "https://avatars.githubusercontent.com/u/1456880?v=4",
			"url":                 "https://api.github.com/users/JamesWoolfenden",
			"type":                "User",
			"followers_url":       "https://api.github.com/users/JamesWoolfenden/followers",
			"organizations_url":   "https://api.github.com/users/JamesWoolfenden/orgs",
			"starred_url":         "https://api.github.com/users/JamesWoolfenden/starred{/owner}{/repo}",
			"events_url":          "https://api.github.com/users/JamesWoolfenden/events{/privacy}",
			"login":               "JamesWoolfenden",
			"id":                  1456880,
			"node_id":             "MDQ6VXNlcjE0NTY4ODA=",
			"gravatar_id":         "",
			"html_url":            "https://github.com/JamesWoolfenden",
			"following_url":       "https://api.github.com/users/JamesWoolfenden/following{/other_user}",
			"gists_url":           "https://api.github.com/users/JamesWoolfenden/gists{/gist_id}",
			"subscriptions_url":   "https://api.github.com/users/JamesWoolfenden/subscriptions",
			"repos_url":           "https://api.github.com/users/JamesWoolfenden/repos",
			"received_events_url": "https://api.github.com/users/JamesWoolfenden/received_events",
			"site_admin":          false,
		},
		"node_id":          "RE_kwDOJyIXLs4GhDgl",
		"tag_name":         "v0.0.1",
		"name":             "Test",
		"prerelease":       false,
		"body":             "",
		"url":              "https://api.github.com/repos/JamesWoolfenden/test-data-action/releases/109328421",
		"upload_url":       "https: //uploads.github.com/repos/JamesWoolfenden/test-data-action/releases/109328421/assets{?name,label}",
		"target_commitish": "main",
	}

	tests := []struct {
		name    string
		args    args
		want    map[string]interface{}
		wantErr bool
	}{
		{"Pass", args{"jameswoolfenden/empty", gitHubToken, &delay}, empty, false},
		{"Has release", args{"jameswoolfenden/test-data-action", gitHubToken, &delay}, want, false},
		{"Has released", args{"jameswoolfenden/test-data-action", gitHubToken, &zero}, result, false},
		{"Fake", args{"jameswoolfenden/god", gitHubToken, &zero}, nil, true},
		{"no token", args{"actions/checkout", "", &zero}, nil, true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := GetReleases(tt.args.action, tt.args.gitHubToken, tt.args.delay)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetReleases() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !(got["tag_name"] == tt.want["tag_name"]) {
				t.Errorf("GetReleases() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetReleasesEdgeCases(t *testing.T) {
	t.Parallel()

	var days uint = 14
	var zero uint = 0

	tests := []struct {
		name        string
		action      string
		gitHubToken string
		days        *uint
		wantErr     bool
		errMsg      string
	}{
		{
			name:        "Empty GitHub token",
			action:      "JamesWoolfenden/test-data-action",
			gitHubToken: "",
			days:        &days,
			wantErr:     true,
			errMsg:      "github token is empty",
		},
		{
			name:        "Empty action name",
			action:      "",
			gitHubToken: "dummy-token",
			days:        &days,
			wantErr:     true,
			errMsg:      "action is empty",
		},
		{
			name:        "Zero days filter",
			action:      "JamesWoolfenden/test-data-action",
			gitHubToken: "dummy-token",
			days:        &zero,
			wantErr:     true,
			errMsg:      "failed to request list of releases api failed with 401",
		},
		{
			name:        "Valid days filter",
			action:      "JamesWoolfenden/test-data-action",
			gitHubToken: "dummy-token",
			days:        &days,
			wantErr:     true,
			errMsg:      "failed to request list of releases api failed with 401",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := GetReleases(tt.action, tt.gitHubToken, tt.days)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetReleases() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && err.Error() != tt.errMsg {
				t.Errorf("GetReleases() error message = %v, want %v", err.Error(), tt.errMsg)
			}
			if !tt.wantErr && got == nil {
				t.Error("GetReleases() returned nil result when error not expected")
			}
		})
	}
}
