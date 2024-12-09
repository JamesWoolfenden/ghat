package core

import (
	"fmt"
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

func TestReadConfigError(t *testing.T) {
	t.Parallel()
	config := "config.yaml"
	testErr := fmt.Errorf("test error")
	err := &readConfigError{config: &config, err: testErr}
	expected := "failed to read config.yaml: test error"
	if err.Error() != expected {
		t.Errorf("Expected error message '%s', got '%s'", expected, err.Error())
	}
}

func TestMarshalJSONError(t *testing.T) {
	t.Parallel()
	testErr := fmt.Errorf("marshal error")
	err := &marshalJSONError{err: testErr}
	expected := "failed to marshal JSON: marshal error"
	if err.Error() != expected {
		t.Errorf("Expected error message '%s', got '%s'", expected, err.Error())
	}
}

func TestGetHookError(t *testing.T) {
	t.Parallel()
	testErr := fmt.Errorf("hook error")
	err := &getHookError{err: testErr}
	expected := "failed to get hook: hook error"
	if err.Error() != expected {
		t.Errorf("Expected error message '%s', got '%s'", expected, err.Error())
	}
}

func TestCastToStringError(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name     string
		object   string
		expected string
	}{
		{"Empty object", "", "failed to cast  to string"},
		{"Valid object", "testObject", "failed to cast testObject to string"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := &castToStringError{object: tc.object}
			if err.Error() != tc.expected {
				t.Errorf("Expected error message '%s', got '%s'", tc.expected, err.Error())
			}
		})
	}
}

func TestRequestFailedError(t *testing.T) {
	t.Parallel()
	testErr := fmt.Errorf("request error")
	err := &requestFailedError{err: testErr}
	expected := "request failed: request error"
	if err.Error() != expected {
		t.Errorf("Expected error message '%s', got '%s'", expected, err.Error())
	}
}

func TestHTTPClientError(t *testing.T) {
	t.Parallel()
	testErr := fmt.Errorf("client error")
	err := &httpClientError{err: testErr}
	expected := "http client error: client error"
	if err.Error() != expected {
		t.Errorf("Expected error message '%s', got '%s'", expected, err.Error())
	}
}

func TestEmptyURL(t *testing.T) {
	t.Parallel()
	err := &emptyURL{}
	expected := "URL is empty"
	if err.Error() != expected {
		t.Errorf("Expected error message '%s', got '%s'", expected, err.Error())
	}
}

func TestRegistryModuleError(t *testing.T) {
	t.Parallel()
	testErr := fmt.Errorf("module error")
	err := &registryModuleError{module: "test-module", err: testErr}
	expected := "failed to get module test-module: module error"
	if err.Error() != expected {
		t.Errorf("Expected error message '%s', got '%s'", expected, err.Error())
	}
}

func TestHTTPGetError(t *testing.T) {
	t.Parallel()
	testErr := fmt.Errorf("get error")
	err := &httpGetError{err: testErr}
	expected := "http get error: get error"
	if err.Error() != expected {
		t.Errorf("Expected error message '%s', got '%s'", expected, err.Error())
	}
}

func TestUnmarshalJSONError(t *testing.T) {
	t.Parallel()
	testErr := fmt.Errorf("unmarshal error")
	err := &unmarshalJSONError{err: testErr}
	expected := "failed to unmarshal: unmarshal error"
	if err.Error() != expected {
		t.Errorf("Expected error message '%s', got '%s'", expected, err.Error())
	}
}

func TestModuleEmptyError(t *testing.T) {
	t.Parallel()
	err := &moduleEmptyError{}
	expected := "module name cannot be empty"
	if err.Error() != expected {
		t.Errorf("Expected error message '%s', got '%s'", expected, err.Error())
	}
}

func TestResponseReadError(t *testing.T) {
	t.Parallel()
	testErr := fmt.Errorf("read error")
	err := &responseReadError{err: testErr}
	expected := "failed to read response: read error"
	if err.Error() != expected {
		t.Errorf("Expected error message '%s', got '%s'", expected, err.Error())
	}
}

func TestResponseNilError(t *testing.T) {
	t.Parallel()
	err := &responseNilError{}
	expected := "api response is nil"
	if err.Error() != expected {
		t.Errorf("Expected error message '%s', got '%s'", expected, err.Error())
	}
}
