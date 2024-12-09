package core

import "fmt"

type actionIsEmptyError struct {
}

func (m *actionIsEmptyError) Error() string {
	return "action is empty"
}

type directoryReadError struct {
	directory string
}

func (m *directoryReadError) Error() string {
	return fmt.Sprintf("action failed to read %s", m.directory)
}

type workingDirectoryError struct {
	directory string
}

func (m *workingDirectoryError) Error() string {
	return fmt.Sprintf("failed to get working directory: %s", m.directory)
}

type executeActionError struct {
	action string
}

func (m *executeActionError) Error() string {
	return fmt.Sprintf("failed to execute action: %s", m.action)
}

type dirAndFileEmptyError struct {
}

func (m *dirAndFileEmptyError) Error() string {
	return "file and directory are empty"
}

type ghaUpdateError struct {
	gha string
}

func (m *ghaUpdateError) Error() string {
	return fmt.Sprintf("GHA update error %s", m.gha)
}

type ghaFileError struct {
	file string
}

func (m *ghaFileError) Error() string {
	return fmt.Sprintf("GHA file error %s", m.file)
}

type castToMapError struct {
	object string
}

func (m *castToMapError) Error() string {
	return fmt.Sprintf("failed to cast %s to map[string]interface{}", m.object)
}

type writeGHAError struct {
	gha string
}

func (m *writeGHAError) Error() string {
	return fmt.Sprintf("failed to write GHA %s", m.gha)
}

type readConfigError struct {
	config *string
	err    error
}

func (m *readConfigError) Error() string {
	return fmt.Sprintf("failed to read %s: %v", *m.config, m.err)
}

type marshalJSONError struct {
	err error
}

func (m *marshalJSONError) Error() string {
	return fmt.Sprintf("failed to marshal JSON: %v", m.err)
}

type getHookError struct {
	err error
}

func (m *getHookError) Error() string {
	return fmt.Sprintf("failed to get hook: %v", m.err)
}

type castToStringError struct {
	object string
}

func (m *castToStringError) Error() string {
	return fmt.Sprintf("failed to cast %s to string", m.object)
}

type requestFailedError struct {
	err error
}

func (m *requestFailedError) Error() string {
	return fmt.Sprintf("request failed: %v", m.err)
}

type httpClientError struct {
	err error
}

func (m *httpClientError) Error() string {
	return fmt.Sprintf("http client error: %v", m.err)
}

type emptyURL struct {
}

func (m *emptyURL) Error() string {
	return "URL is empty"
}

type registryModuleError struct {
	module string
	err    error
}

func (m *registryModuleError) Error() string {
	return fmt.Sprintf("failed to get module %s: %v", m.module, m.err)
}

type httpGetError struct {
	err error
}

func (m *httpGetError) Error() string {
	return fmt.Sprintf("http get error: %v", m.err)
}

type unmarshalJSONError struct {
	err error
}

func (m *unmarshalJSONError) Error() string {
	return fmt.Sprintf("failed to unmarshal: %v", m.err)
}

type moduleEmptyError struct {
}

func (m *moduleEmptyError) Error() string {
	return "module name cannot be empty"
}

type responseReadError struct {
	err error
}

func (m *responseReadError) Error() string {
	return fmt.Sprintf("failed to read response: %v", m.err)
}

type responseNilError struct {
}

func (m *responseNilError) Error() string {
	return "api response is nil"
}
