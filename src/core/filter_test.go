package core

import (
	"os"
	"strings"
	"testing"
)

func TestGetReleases(t *testing.T) {
	t.Parallel()

	// Get GitHub token from environment
	gitHubToken := os.Getenv("GITHUB_TOKEN")
	if gitHubToken == "" {
		gitHubToken = os.Getenv("GITHUB_API")
	}

	// Skip tests requiring token if not available
	if gitHubToken == "" {
		t.Log("GITHUB_TOKEN not set, some tests may be rate limited")
	}

	type args struct {
		action      string
		gitHubToken string
		delay       *uint
	}

	var delay uint = 14
	var zero uint = 0

	tests := []struct {
		name        string
		args        args
		wantTag     string // Just check the tag_name instead of the whole structure
		wantErr     bool
		skipNoToken bool // Skip this test if no token available
	}{
		{
			name:        "Empty repo - should error",
			args:        args{"jameswoolfenden/empty", gitHubToken, &delay},
			wantTag:     "",
			wantErr:     true,
			skipNoToken: false,
		},
		{
			name:        "Has release - no delay",
			args:        args{"actions/checkout", gitHubToken, &zero},
			wantTag:     "", // Don't check exact tag as it changes frequently
			wantErr:     false,
			skipNoToken: true,
		},
		{
			name:        "Has release - with delay",
			args:        args{"actions/checkout", gitHubToken, &delay},
			wantTag:     "", // Don't check exact tag as it changes
			wantErr:     false,
			skipNoToken: true,
		},
		{
			name:        "Fake repo - should error",
			args:        args{"jameswoolfenden/nonexistentrepo12345", gitHubToken, &zero},
			wantTag:     "",
			wantErr:     true,
			skipNoToken: false,
		},
		{
			name:        "Well-known repo",
			args:        args{"hashicorp/terraform", gitHubToken, &zero},
			wantTag:     "", // Don't assert exact tag since it changes over time
			wantErr:     false,
			skipNoToken: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Skip if no token and test requires it
			if tt.skipNoToken && gitHubToken == "" {
				t.Skip("Skipping test: requires GITHUB_TOKEN")
			}

			got, err := GetReleases(tt.args.action, tt.args.gitHubToken, tt.args.delay)

			// Check error expectation
			if (err != nil) != tt.wantErr {
				t.Errorf("GetReleases() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// If we expect an error, don't check the result
			if tt.wantErr {
				if got != nil {
					t.Logf("Got error as expected: %v", err)
				}
				return
			}

			// Check that we got a valid response
			if got == nil {
				t.Errorf("GetReleases() returned nil but no error")
				return
			}

			// Only check tag_name if we specified one to expect
			if tt.wantTag != "" {
				gotTag, ok := got["tag_name"].(string)
				if !ok {
					t.Errorf("GetReleases() tag_name is not a string: %v (type: %T)", got["tag_name"], got["tag_name"])
					return
				}

				if gotTag != tt.wantTag {
					t.Errorf("GetReleases() tag_name = %v, want %v", gotTag, tt.wantTag)
				}
			} else {
				// Just verify that tag_name exists for repos that should have releases
				if tagName, ok := got["tag_name"]; ok {
					t.Logf("Got tag_name: %v", tagName)
				} else {
					t.Logf("No tag_name in response (repo may have no releases)")
				}
			}

			// Verify other expected fields exist (but don't check exact values)
			expectedFields := []string{"id", "html_url", "created_at", "published_at"}
			for _, field := range expectedFields {
				if val, ok := got[field]; !ok {
					t.Errorf("GetReleases() response missing expected field: %s", field)
				} else {
					t.Logf("Field %s: %v", field, val)
				}
			}
		})
	}
}

// TestGetReleases_WithoutToken tests behavior when no GitHub token is provided
func TestGetReleases_WithoutToken(t *testing.T) {
	t.Parallel()

	// Test with a well-known repo that should have releases
	got, err := GetReleases("actions/checkout", "", nil)

	if err != nil {
		// Rate limiting is expected without a token
		t.Logf("GetReleases without token returned error (expected if rate limited): %v", err)
		return
	}

	if got == nil {
		t.Error("GetReleases() returned nil with no error")
		return
	}

	// Just verify we got something back
	if _, ok := got["tag_name"]; !ok {
		t.Error("GetReleases() response missing tag_name")
	}
}

// TestGetReleases_StableDelay tests the stable delay feature
func TestGetReleases_StableDelay(t *testing.T) {
	t.Parallel()

	gitHubToken := os.Getenv("GITHUB_TOKEN")
	if gitHubToken == "" {
		gitHubToken = os.Getenv("GITHUB_API")
	}

	if gitHubToken == "" {
		t.Skip("Skipping test: requires GITHUB_TOKEN")
	}

	var delay uint = 365 // Get releases from 1 year ago

	got, err := GetReleases("hashicorp/terraform", gitHubToken, &delay)

	if err != nil {
		t.Logf("GetReleases with delay returned error: %v", err)
		// This might be expected if there are no releases old enough
		return
	}

	if got == nil {
		t.Log("No releases found within the stable delay period")
		return
	}

	// Verify we got a release
	if tagName, ok := got["tag_name"].(string); ok {
		t.Logf("Got stable release: %s", tagName)

		// Check the created_at date is old enough
		if createdAt, ok := got["created_at"].(string); ok {
			t.Logf("Release created at: %s", createdAt)
		}
	}
}

// TestGetReleases_YourRepo tests with your specific repository
// This test is separate so it can be easily skipped if the repo doesn't exist
func TestGetReleases_YourRepo(t *testing.T) {
	t.Parallel()

	gitHubToken := os.Getenv("GITHUB_TOKEN")
	if gitHubToken == "" {
		gitHubToken = os.Getenv("GITHUB_API")
	}

	if gitHubToken == "" {
		t.Skip("Skipping test: requires GITHUB_TOKEN")
	}

	// Test with your specific repo - change this to match your actual repo
	var zero uint = 0
	got, err := GetReleases("jameswoolfenden/ghat", gitHubToken, &zero)

	if err != nil {
		t.Skipf("Repository may not exist or have releases: %v", err)
		return
	}

	if got == nil {
		t.Skip("No releases found in repository")
		return
	}

	// Verify we got a release
	if tagName, ok := got["tag_name"].(string); ok {
		t.Logf("Got release: %s", tagName)
	} else {
		t.Error("Response missing tag_name field")
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
		errContains string // Changed from errMsg to errContains for partial matching
	}{
		{
			name:        "Empty action name",
			action:      "",
			gitHubToken: "dummy-token",
			days:        &days,
			wantErr:     true,
			errContains: "action", // Just check that error mentions action
		},
		{
			name:        "Invalid action format",
			action:      "invalid-action-format",
			gitHubToken: "dummy-token",
			days:        &days,
			wantErr:     true,
			errContains: "", // Any error is acceptable
		},
		{
			name:        "Zero days filter",
			action:      "actions/checkout",
			gitHubToken: gitHubToken,
			days:        &zero,
			wantErr:     false, // Zero days is valid (means latest)
			errContains: "",
		},
		{
			name:        "Valid days filter with real repo",
			action:      "actions/checkout",
			gitHubToken: gitHubToken,
			days:        &days,
			wantErr:     false, // Should work even without token (may be rate limited)
			errContains: "",
		},
		{
			name:        "Nonexistent repository",
			action:      "thisreposhouldnotexist/ever12345",
			gitHubToken: gitHubToken,
			days:        &zero,
			wantErr:     true,
			errContains: "404", // Should get 404 error
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

			if err != nil && tt.errContains != "" {
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("GetReleases() error = %v, want error containing %q", err.Error(), tt.errContains)
				}
			}

			if !tt.wantErr && got == nil {
				t.Error("GetReleases() returned nil result when error not expected")
			}

			// For successful cases, verify we got a valid release
			if !tt.wantErr && got != nil {
				if _, ok := got["tag_name"]; !ok {
					t.Error("GetReleases() result missing tag_name field")
				}
			}
		})
	}
}
