//go:build !linux && !darwin && !windows

package product

import (
	"errors"
	"os"
)

// Other platforms fail closed until an exclusive rename and stable directory
// identity implementation is available. Ordinary rename/copy is not safe.
func projectImportRenameNoReplace(from, to string) error {
	return &os.LinkError{Op: "rename-noreplace", Old: from, New: to, Err: errors.ErrUnsupported}
}

func projectImportDirectoryIdentity(path string) (string, error) {
	return "", &os.PathError{Op: "directory-identity", Path: path, Err: errors.ErrUnsupported}
}
