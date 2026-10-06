package product

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// projectImportRenameNoReplace uses the filesystem's exclusive rename. There
// is deliberately no ordinary-rename or copy fallback on unsupported volumes.
// The caller owns ancestor validation and directory syncing.
func projectImportRenameNoReplace(from, to string) error {
	if err := unix.RenamexNp(from, to, unix.RENAME_EXCL); err != nil {
		return &os.LinkError{Op: "rename-noreplace", Old: from, New: to, Err: err}
	}
	return nil
}

// projectImportDirectoryIdentity does not follow the final path component. The
// caller must guard its ancestors; an identity is not an authorization check.
func projectImportDirectoryIdentity(path string) (string, error) {
	if path == "" {
		return "", &os.PathError{Op: "directory-identity", Path: path, Err: os.ErrInvalid}
	}
	var info unix.Stat_t
	if err := unix.Lstat(filepath.Clean(path), &info); err != nil {
		return "", &os.PathError{Op: "directory-identity", Path: path, Err: err}
	}
	if info.Mode&unix.S_IFMT != unix.S_IFDIR {
		return "", &os.PathError{Op: "directory-identity", Path: path, Err: os.ErrInvalid}
	}
	return fmt.Sprintf("unix:%x:%x", uint64(info.Dev), uint64(info.Ino)), nil
}
