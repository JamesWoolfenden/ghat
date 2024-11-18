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
