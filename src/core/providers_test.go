package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rs/zerolog/log"
)

func TestGetLatestProviderVersion(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		provider  string
		wantErr   bool
	}{
		{
			name:      "hashicorp aws",
			namespace: "hashicorp",
			provider:  "aws",
			wantErr:   false,
		},
		{
			name:      "hashicorp random",
			namespace: "hashicorp",
			provider:  "random",
			wantErr:   false,
		},
		{
			name:      "invalid provider",
			namespace: "nonexistent",
			provider:  "fake",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version, err := getLatestProviderVersion(tt.namespace, tt.provider)
			if (err != nil) != tt.wantErr {
				t.Errorf("getLatestProviderVersion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if version == "" {
					t.Errorf("getLatestProviderVersion() returned empty version")
				}
				t.Logf("Latest version of %s/%s: %s", tt.namespace, tt.provider, version)
			}
		})
	}
}

func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		name     string
		version  string
		expected string
	}{
		{"simple version", "1.2.3", "v1.2.3"},
		{"with v prefix", "v1.2.3", "v1.2.3"},
		{"with ~> constraint", "~> 5.0", "v5.0"},
		{"with >= constraint", ">= 3.0.0", "v3.0.0"},
		{"with = constraint", "= 2.1.0", "v2.1.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeVersion(tt.version)
			if got != tt.expected {
				t.Errorf("normalizeVersion(%q) = %q, want %q", tt.version, got, tt.expected)
			}
		})
	}
}

func TestHasVersionConstraint(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    bool
	}{
		{"no constraint", "1.2.3", false},
		{"tilde arrow", "~> 5.0", true},
		{"greater than equal", ">= 3.0", true},
		{"less than", "< 2.0", true},
		{"not equal", "!= 1.0", true},
		{"exact version", "= 1.2.3", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasVersionConstraint(tt.version)
			if got != tt.want {
				t.Errorf("hasVersionConstraint(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

func TestShouldUpdateProvider(t *testing.T) {
	tests := []struct {
		name     string
		provider *ProviderInfo
		want     bool
	}{
		{
			name: "no current version",
			provider: &ProviderInfo{
				CurrentVersion: "",
				LatestVersion:  "5.0.0",
			},
			want: true,
		},
		{
			name: "with constraint",
			provider: &ProviderInfo{
				CurrentVersion: "~> 4.0",
				LatestVersion:  "5.0.0",
			},
			want: true,
		},
		{
			name: "outdated version",
			provider: &ProviderInfo{
				CurrentVersion: "4.0.0",
				LatestVersion:  "5.0.0",
			},
			want: true,
		},
		{
			name: "up to date",
			provider: &ProviderInfo{
				CurrentVersion: "5.0.0",
				LatestVersion:  "5.0.0",
			},
			want: false,
		},
		{
			name: "newer than latest",
			provider: &ProviderInfo{
				CurrentVersion: "5.1.0",
				LatestVersion:  "5.0.0",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldUpdateProvider(tt.provider)
			if got != tt.want {
				t.Errorf("shouldUpdateProvider() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseProviderBlock(t *testing.T) {
	// This is harder to test without real HCL, so we'll test the extract pattern helper
	tests := []struct {
		name    string
		content string
		pattern string
		want    string
	}{
		{
			name:    "extract source",
			content: `source = "hashicorp/aws"`,
			pattern: "source",
			want:    "hashicorp/aws",
		},
		{
			name:    "extract version",
			content: `version = "~> 5.0"`,
			pattern: "version",
			want:    "~> 5.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractPattern(tt.content, tt.pattern)
			if len(result) < 2 {
				t.Errorf("extractPattern() returned %v, expected at least 2 elements", result)
				return
			}
			if result[1] != tt.want {
				t.Errorf("extractPattern() = %q, want %q", result[1], tt.want)
			}
		})
	}
}

func TestUpdateProvider(t *testing.T) {
	// Create a temporary test file
	tmpDir, err := os.MkdirTemp("", "provider-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func(path string) {
		err := os.RemoveAll(path)
		if err != nil {
			t.Fatal(err)
		}
	}(tmpDir)

	testFile := filepath.Join(tmpDir, "test.tf")
	testContent := `terraform {
  required_version = ">= 1.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 4.0"
    }
  }
}
`
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		dryRun  bool
		wantErr bool
	}{
		{
			name:    "dry run",
			dryRun:  true,
			wantErr: false,
		},
		{
			name:    "actual update",
			dryRun:  false,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags := &Flags{
				DryRun: tt.dryRun,
			}

			err := flags.UpdateProvider(testFile)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateProvider() error = %v, wantErr %v", err, tt.wantErr)
			}

			// For actual update, verify the file was modified
			if !tt.dryRun && !tt.wantErr {
				content, err := os.ReadFile(testFile)
				if err != nil {
					t.Fatal(err)
				}

				// Should no longer have the ~> constraint
				if strings.Contains(string(content), "~> 4.0") {
					t.Errorf("File still contains old version constraint")
				}

				t.Logf("Updated content:\n%s", string(content))
			}
		})
	}
}

func TestGetProviderFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "provider-files-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func(path string) {
		err := os.RemoveAll(path)
		if err != nil {
			log.Warn().Msgf("Error removing temporary files: %v", err)
		}
	}(tmpDir)

	// Create test files
	providerFile := filepath.Join(tmpDir, "providers.tf")
	regularFile := filepath.Join(tmpDir, "main.tf")

	providerContent := `terraform {
  required_providers {
    aws = {
      source = "hashicorp/aws"
    }
  }
}
`
	regularContent := `resource "null_resource" "test" {}
`

	if err := os.WriteFile(providerFile, []byte(providerContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(regularFile, []byte(regularContent), 0644); err != nil {
		t.Fatal(err)
	}

	flags := &Flags{
		Directory: tmpDir,
		Entries:   []string{providerFile, regularFile},
	}

	files, err := flags.GetProviderFiles()
	if err != nil {
		t.Fatal(err)
	}

	if len(files) != 1 {
		t.Errorf("GetProviderFiles() returned %d files, expected 1", len(files))
	}

	if len(files) > 0 && !strings.Contains(files[0], "providers.tf") {
		t.Errorf("GetProviderFiles() didn't return providers.tf, got %v", files)
	}
}

func TestExtractProvidersFromFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "extract-providers-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func(path string) {
		err := os.RemoveAll(path)
		if err != nil {
			log.Warn().Msgf("Error removing temp dir %s: %v", path, err)
		}
	}(tmpDir)

	testFile := filepath.Join(tmpDir, "providers.tf")
	testContent := `terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "5.0.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "3.5.1"
    }
  }
}
`
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatal(err)
	}

	providers, err := extractProvidersFromFile(testFile)
	if err != nil {
		t.Fatal(err)
	}

	if len(providers) != 2 {
		t.Errorf("extractProvidersFromFile() returned %d providers, expected 2", len(providers))
	}

	// Check that we found both providers
	foundAws := false
	foundRandom := false

	for _, p := range providers {
		if p.Name == "aws" {
			foundAws = true
			if p.Source != "hashicorp/aws" {
				t.Errorf("AWS provider has wrong source: %s", p.Source)
			}
		}
		if p.Name == "random" {
			foundRandom = true
			if p.Source != "hashicorp/random" {
				t.Errorf("Random provider has wrong source: %s", p.Source)
			}
		}
		t.Logf("Found provider: %s (%s) version %s", p.Name, p.Source, p.CurrentVersion)
	}

	if !foundAws {
		t.Error("Did not find aws provider")
	}
	if !foundRandom {
		t.Error("Did not find random provider")
	}
}

func TestGetTerraformFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "terraform-files-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func(path string) {
		err := os.RemoveAll(path)
		if err != nil {
			t.Fatalf("Failed to remove temporary directory: %v", err)
		}
	}(tmpDir)

	// Create nested directory structure
	subDir := filepath.Join(tmpDir, "modules", "vpc")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create .tf files
	files := []string{
		filepath.Join(tmpDir, "main.tf"),
		filepath.Join(tmpDir, "variables.tf"),
		filepath.Join(subDir, "vpc.tf"),
	}

	for _, f := range files {
		if err := os.WriteFile(f, []byte("# test"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Create a non-.tf file
	if err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# test"), 0644); err != nil {
		t.Fatal(err)
	}

	tfFiles, err := GetTerraformFiles(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	if len(tfFiles) != 3 {
		t.Errorf("GetTerraformFiles() returned %d files, expected 3. Got: %v", len(tfFiles), tfFiles)
	}

	// Verify all are .tf files
	for _, f := range tfFiles {
		if filepath.Ext(f) != ".tf" {
			t.Errorf("GetTerraformFiles() returned non-.tf file: %s", f)
		}
	}
}

func TestUpdateProviders_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	// Use the test data file
	testFile := "testdata/providers/simple/providers.tf"
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Skip("Test data file not found")
	}

	// Create a temp copy
	tmpDir, err := os.MkdirTemp("", "providers-integration-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func(path string) {
		err := os.RemoveAll(path)
		if err != nil {
			t.Fatalf("Error removing temporary directory: %v", err)
		}
	}(tmpDir)

	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatal(err)
	}

	tmpFile := filepath.Join(tmpDir, "providers.tf")
	if err := os.WriteFile(tmpFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	flags := &Flags{
		Directory: tmpDir,
		Entries:   []string{tmpFile},
		DryRun:    true, // Don't actually modify in test
	}

	err = flags.UpdateProviders()
	if err != nil {
		t.Errorf("UpdateProviders() error = %v", err)
	}
}
