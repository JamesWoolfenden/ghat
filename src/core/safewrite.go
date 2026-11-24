package core

import (
	"fmt"
	"os"
)

// safeWriteFile writes data to path like os.WriteFile, but refuses to write
// through a symlink or to any non-regular file. A scanned repository could
// otherwise commit a workflow as a symlink to a file outside the repo and have
// ghat rewrite that file.
//
// Only the final path element is checked: a symlinked parent directory is not
// detected, because a checkout legitimately living under a symlinked path
// (for example /tmp on macOS) would then be rejected.
func safeWriteFile(path string, data []byte, perm os.FileMode) error {
	info, err := os.Lstat(path)
	if err == nil && !info.Mode().IsRegular() {
		return fmt.Errorf("refusing to write %s: not a regular file (symlink?)", path)
	}

	return os.WriteFile(path, data, perm)
}
