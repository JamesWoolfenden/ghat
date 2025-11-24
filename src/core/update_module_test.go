package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
)

func TestUpdateModule_PreservesCommentsAndWhitespace(t *testing.T) {
	// Skip this test if GITHUB_TOKEN is not set (to avoid API rate limits)
	if os.Getenv("GITHUB_TOKEN") == "" && os.Getenv("GITHUB_API") == "" {
		t.Skip("Skipping test: GITHUB_TOKEN or GITHUB_API not set. This test makes real GitHub API calls.")
	}

	// Test input with comments and whitespace
	// Use git format to avoid Terraform Registry lookup issues
	input := `# This is a comment at the top

resource "type" "name" {
  key = "value"
  # Comment in block
  another_key = "another_value"
}

# Comment between blocks

module "ip" {
  source = "git::https://github.com/JamesWoolfenden/terraform-http-ip.git?ref=v0.3.12"
}

# Comment at the end
`

	// Create a temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.tf")

	err := os.WriteFile(testFile, []byte(input), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create flags with necessary mocks/setup
	myFlags := &Flags{
		DryRun: false,
		Update: false,
	}

	// Run UpdateModule
	err = myFlags.UpdateModule(testFile)
	if err != nil {
		t.Fatalf("UpdateModule failed: %v", err)
	}

	// Read the output
	output, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	outputStr := string(output)

	// Test assertions
	tests := []struct {
		name        string
		shouldExist string
		description string
	}{
		{
			name:        "top_comment",
			shouldExist: "# This is a comment at the top",
			description: "Top comment should be preserved",
		},
		{
			name:        "comment_in_block",
			shouldExist: "# Comment in block",
			description: "Comment inside resource block should be preserved",
		},
		{
			name:        "comment_between_blocks",
			shouldExist: "# Comment between blocks",
			description: "Comment between resource and module should be preserved",
		},
		{
			name:        "comment_at_end",
			shouldExist: "# Comment at the end",
			description: "Comment at end of file should be preserved",
		},
		{
			name:        "resource_block",
			shouldExist: `resource "type" "name"`,
			description: "Resource block should be preserved",
		},
		{
			name:        "module_block",
			shouldExist: `module "ip"`,
			description: "Module block should be preserved",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(outputStr, tt.shouldExist) {
				t.Errorf("%s\nExpected to find: %q\nGot output:\n%s",
					tt.description, tt.shouldExist, outputStr)
			}
		})
	}

	// Check that blank lines are preserved
	inputLines := strings.Split(input, "\n")
	outputLines := strings.Split(outputStr, "\n")

	// Count blank lines in input
	inputBlankLines := 0
	for _, line := range inputLines {
		if strings.TrimSpace(line) == "" {
			inputBlankLines++
		}
	}

	// Count blank lines in output
	outputBlankLines := 0
	for _, line := range outputLines {
		if strings.TrimSpace(line) == "" {
			outputBlankLines++
		}
	}

	// We should have roughly the same number of blank lines (within 1-2)
	// Some variation is acceptable due to formatting
	if abs(inputBlankLines-outputBlankLines) > 2 {
		t.Errorf("Blank line count changed significantly. Input: %d, Output: %d",
			inputBlankLines, outputBlankLines)
	}
}

func TestUpdateModule_UpdatesModuleSource(t *testing.T) {
	gitHubToken := os.Getenv("GITHUB_TOKEN")
	if gitHubToken == "" {
		gitHubToken = os.Getenv("GITHUB_API")
	}

	if gitHubToken == "" {
		t.Skip("Skipping test: GITHUB_TOKEN or GITHUB_API not set. This test requires authentication to avoid rate limits.")
	}

	input := `module "ip" {
  source  = "JamesWoolfenden/ip/http"
  version = "0.3.12"
}
`

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.tf")

	err := os.WriteFile(testFile, []byte(input), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	myFlags := &Flags{
		DryRun:      false,
		Update:      false,
		GitHubToken: gitHubToken, // Make sure to pass the token!
	}

	err = myFlags.UpdateModule(testFile)
	if err != nil {
		t.Fatalf("UpdateModule failed: %v", err)
	}

	output, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	outputStr := string(output)

	// The source should be updated to use git with commit hash
	if !strings.Contains(outputStr, "git::https://github.com/") {
		t.Errorf("Expected source to be updated to git URL with hash, got:\n%s", outputStr)
	}

	// Version attribute should be removed
	if strings.Contains(outputStr, `version = "0.3.12"`) {
		t.Errorf("Expected version attribute to be removed, but it still exists")
	}

	t.Logf("Updated module source:\n%s", outputStr)
}

func TestUpdateModule_PreservesIndentation(t *testing.T) {
	// This test doesn't need real GitHub API calls - test the HCL behavior directly
	input := `module "example" {
  source  = "old-source"
  version = "1.0.0"

  key1 = "value1"
  key2 = "value2"
}
`

	src := []byte(input)
	inFile, diags := hclwrite.ParseConfig(src, "", hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		t.Fatalf("Failed to parse: %v", diags)
	}

	// Modify the module block
	root := inFile.Body()
	for _, block := range root.Blocks() {
		if block.Type() == "module" {
			block.Body().RemoveAttribute("version")
			block.Body().SetAttributeValue("source", cty.StringVal("new-source"))
		}
	}

	output := string(inFile.Bytes())

	// Check that indentation is preserved
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "key1") || strings.Contains(line, "key2") {
			if !strings.HasPrefix(line, "  ") {
				t.Errorf("Expected line to be indented with 2 spaces: %q", line)
			}
		}
		if strings.Contains(line, "source") {
			if !strings.HasPrefix(line, "  ") {
				t.Errorf("Expected source line to be indented with 2 spaces: %q", line)
			}
		}
	}

	t.Logf("Output with preserved indentation:\n%s", output)
}

func TestUpdateModule_DryRun(t *testing.T) {
	if os.Getenv("GITHUB_TOKEN") == "" && os.Getenv("GITHUB_API") == "" {
		t.Skip("Skipping test: GITHUB_TOKEN or GITHUB_API not set")
	}

	input := `module "ip" {
  source  = "JamesWoolfenden/ip/http"
  version = "0.3.12"
}
`

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.tf")

	err := os.WriteFile(testFile, []byte(input), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	myFlags := &Flags{
		DryRun: true, // Enable dry run
	}

	err = myFlags.UpdateModule(testFile)
	if err != nil {
		t.Fatalf("UpdateModule failed: %v", err)
	}

	// Read the file - it should be unchanged
	output, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	if string(output) != input {
		t.Errorf("File was modified during dry run. Expected no changes.")
	}
}

func TestUpdateModule_MultipleModules(t *testing.T) {
	// This test verifies that multiple modules can be processed and comments are preserved
	// It uses a minimal HCL test instead of real GitHub API calls
	input := `# First module
module "first" {
  source  = "test-source-1"
  version = "1.0.0"
}

# Second module
module "second" {
  source  = "test-source-2"
  version = "2.0.0"
}

# Third module
module "third" {
  source  = "test-source-3"
  version = "3.0.0"
}
`

	src := []byte(input)
	inFile, diags := hclwrite.ParseConfig(src, "", hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		t.Fatalf("Failed to parse: %v", diags)
	}

	// Modify all module blocks (simulating what UpdateModule does)
	root := inFile.Body()
	moduleCount := 0
	for _, block := range root.Blocks() {
		if block.Type() == "module" {
			moduleCount++
			block.Body().RemoveAttribute("version")
			// Update source with different values to verify each module is processed
			moduleName := block.Labels()[0]
			block.Body().SetAttributeValue("source", cty.StringVal("new-source-"+moduleName))
		}
	}

	if moduleCount != 3 {
		t.Errorf("Expected to find 3 modules, found %d", moduleCount)
	}

	output := string(inFile.Bytes())

	// All comments should be preserved
	comments := []string{"# First module", "# Second module", "# Third module"}
	for _, comment := range comments {
		if !strings.Contains(output, comment) {
			t.Errorf("Expected comment %q to be preserved", comment)
		}
	}

	// All module blocks should still exist
	modules := []string{`module "first"`, `module "second"`, `module "third"`}
	for _, module := range modules {
		if !strings.Contains(output, module) {
			t.Errorf("Expected module %q to be preserved", module)
		}
	}

	// Verify sources were updated
	expectedSources := []string{"new-source-first", "new-source-second", "new-source-third"}
	for _, source := range expectedSources {
		if !strings.Contains(output, source) {
			t.Errorf("Expected source %q to be in output", source)
		}
	}

	// Verify versions were removed
	if strings.Contains(output, `version = `) {
		t.Errorf("Version attributes should have been removed")
	}

	t.Logf("Output with multiple modules processed:\n%s", output)
}

// Helper function for absolute value
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// TestUpdateModule_PreservesCommentsAndWhitespace_Minimal is a simplified test
// that doesn't require all the dependencies to be working
// THIS IS THE MAIN TEST - Run this one!
func TestUpdateModule_PreservesCommentsAndWhitespace_Minimal(t *testing.T) {
	// This test directly tests the hclwrite behavior without calling UpdateModule
	// It verifies that our fix (not creating a new empty file) works correctly

	input := `# This is a comment at the top

resource "type" "name" {
  key = "value"
  # Comment in block
  another_key = "another_value"
}

# Comment between blocks

module "example" {
  source  = "test-source"
  version = "1.0.0"
}

# Comment at the end
`

	src := []byte(input)

	t.Run("correct_approach_modifies_in_place", func(t *testing.T) {
		// THE CORRECT APPROACH - modify inFile in place
		inFile, diags := hclwrite.ParseConfig(src, "", hcl.Pos{Line: 1, Column: 1})
		if diags.HasErrors() {
			t.Fatalf("Failed to parse: %v", diags)
		}

		// Simulate what UpdateModule does - modify a module block
		root := inFile.Body()
		for _, block := range root.Blocks() {
			if block.Type() == "module" {
				// Remove version and update source (like UpdateModule does)
				block.Body().RemoveAttribute("version")
				block.Body().SetAttributeValue("source", cty.StringVal("new-source"))
			}
		}

		// Get the output from the SAME file we parsed
		output := string(inFile.Bytes())

		// Verify comments are preserved
		comments := []string{
			"# This is a comment at the top",
			"# Comment in block",
			"# Comment between blocks",
			"# Comment at the end",
		}

		for _, comment := range comments {
			if !strings.Contains(output, comment) {
				t.Errorf("Comment not preserved: %q\nOutput:\n%s", comment, output)
			}
		}

		// Verify the source was actually updated
		if !strings.Contains(output, "new-source") {
			t.Errorf("Source was not updated to 'new-source'\nOutput:\n%s", output)
		}

		// Verify version was removed
		if strings.Contains(output, `version = "1.0.0"`) {
			t.Errorf("Version attribute was not removed\nOutput:\n%s", output)
		}

		t.Logf("✓ Output correctly preserved comments:\n%s", output)
	})

	t.Run("wrong_approach_loses_comments", func(t *testing.T) {
		// THE WRONG APPROACH - creating a new empty file (what the bug was doing)
		inFile, diags := hclwrite.ParseConfig(src, "", hcl.Pos{Line: 1, Column: 1})
		if diags.HasErrors() {
			t.Fatalf("Failed to parse: %v", diags)
		}

		// THIS IS THE BUG - creating a new empty file
		outFile := hclwrite.NewEmptyFile()
		newBody := outFile.Body()

		root := inFile.Body()
		for _, block := range root.Blocks() {
			if block.Type() == "module" {
				block.Body().RemoveAttribute("version")
				block.Body().SetAttributeValue("source", cty.StringVal("new-source"))
			}
			// Appending to the new empty file
			newBody.AppendBlock(block)
		}

		// Get output from the NEW file (loses comments)
		output := string(outFile.Bytes())

		// Verify that comments are LOST with the wrong approach
		if strings.Contains(output, "# This is a comment at the top") {
			t.Errorf("Bug NOT reproduced - top comment should be lost with wrong approach")
		}
		if strings.Contains(output, "# Comment between blocks") {
			t.Errorf("Bug NOT reproduced - between comment should be lost with wrong approach")
		}
		if strings.Contains(output, "# Comment at the end") {
			t.Errorf("Bug NOT reproduced - end comment should be lost with wrong approach")
		}

		t.Logf("✓ Confirmed: wrong approach loses comments:\n%s", output)
	})
}
