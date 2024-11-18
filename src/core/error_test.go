package core

import (
	"testing"
)

func TestActionIsEmptyError(t *testing.T) {
	t.Parallel()
	err := &actionIsEmptyError{}
	expected := "action is empty"
	if err.Error() != expected {
		t.Errorf("Expected error message '%s', got '%s'", expected, err.Error())
	}
}

func TestDirectoryReadError(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name      string
		directory string
		expected  string
	}{
		{"Empty directory", "", "action failed to read "},
		{"Valid directory", "/test/dir", "action failed to read /test/dir"},
		{"Relative path", "./relative", "action failed to read ./relative"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := &directoryReadError{directory: tc.directory}
			if err.Error() != tc.expected {
				t.Errorf("Expected error message '%s', got '%s'", tc.expected, err.Error())
			}
		})
	}
}

func TestWorkingDirectoryError(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name      string
		directory string
		expected  string
	}{
		{"Empty directory", "", "failed to get working directory: "},
		{"Valid directory", "/home/user", "failed to get working directory: /home/user"},
		{"Windows path", "C:\\Users\\test", "failed to get working directory: C:\\Users\\test"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := &workingDirectoryError{directory: tc.directory}
			if err.Error() != tc.expected {
				t.Errorf("Expected error message '%s', got '%s'", tc.expected, err.Error())
			}
		})
	}
}

func TestExecuteActionError(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name     string
		action   string
		expected string
	}{
		{"Empty action", "", "failed to execute action: "},
		{"Simple action", "build", "failed to execute action: build"},
		{"Complex action", "deploy --force --env=prod", "failed to execute action: deploy --force --env=prod"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := &executeActionError{action: tc.action}
			if err.Error() != tc.expected {
				t.Errorf("Expected error message '%s', got '%s'", tc.expected, err.Error())
			}
		})
	}
}

func TestDirAndFileEmptyError(t *testing.T) {
	t.Parallel()
	err := &dirAndFileEmptyError{}
	expected := "file and directory are empty"
	if err.Error() != expected {
		t.Errorf("Expected error message '%s', got '%s'", expected, err.Error())
	}
}

func TestGHAUpdateError(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name     string
		gha      string
		expected string
	}{
		{"Empty GHA", "", "GHA update error "},
		{"Valid GHA", "workflow.yml", "GHA update error workflow.yml"},
		{"Path GHA", ".github/workflows/test.yml", "GHA update error .github/workflows/test.yml"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := &ghaUpdateError{gha: tc.gha}
			if err.Error() != tc.expected {
				t.Errorf("Expected error message '%s', got '%s'", tc.expected, err.Error())
			}
		})
	}
}

func TestGHAFileError(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name     string
		file     string
		expected string
	}{
		{"Empty file", "", "GHA file error "},
		{"Simple file", "main.yml", "GHA file error main.yml"},
		{"Nested file", "workflows/deploy.yml", "GHA file error workflows/deploy.yml"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := &ghaFileError{file: tc.file}
			if err.Error() != tc.expected {
				t.Errorf("Expected error message '%s', got '%s'", tc.expected, err.Error())
			}
		})
	}
}

func TestCastToMapError(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name     string
		object   string
		expected string
	}{
		{"Empty object", "", "failed to cast  to map[string]interface{}"},
		{"Simple object", "config", "failed to cast config to map[string]interface{}"},
		{"Complex object", "workflow.settings", "failed to cast workflow.settings to map[string]interface{}"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := &castToMapError{object: tc.object}
			if err.Error() != tc.expected {
				t.Errorf("Expected error message '%s', got '%s'", tc.expected, err.Error())
			}
		})
	}
}

func TestWriteGHAError(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name     string
		gha      string
		expected string
	}{
		{"Empty GHA", "", "failed to write GHA "},
		{"Simple GHA", "ci.yml", "failed to write GHA ci.yml"},
		{"Full path GHA", ".github/workflows/release.yml", "failed to write GHA .github/workflows/release.yml"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := &writeGHAError{gha: tc.gha}
			if err.Error() != tc.expected {
				t.Errorf("Expected error message '%s', got '%s'", tc.expected, err.Error())
			}
		})
	}
}
