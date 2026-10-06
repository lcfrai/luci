package product

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// projectImportRenameNoReplace never opts into replacement or cross-volume
// copying. Windows moves directories only within a volume. The caller owns
// ancestor validation and durable receipt ordering.
func projectImportRenameNoReplace(from, to string) error {
	fromPath, err := projectImportWindowsPath(from)
	if err != nil {
		return &os.LinkError{Op: "rename-noreplace", Old: from, New: to, Err: err}
	}
	toPath, err := projectImportWindowsPath(to)
	if err != nil {
		return &os.LinkError{Op: "rename-noreplace", Old: from, New: to, Err: err}
	}
	if err := windows.MoveFileEx(fromPath, toPath, windows.MOVEFILE_WRITE_THROUGH); err != nil {
		if errors.Is(err, windows.ERROR_FILE_EXISTS) || errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
			err = os.ErrExist
		} else if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
			// Existing directories can report ACCESS_DENIED. Inspect only after
			// the exclusive move failed; this is never a precheck for replacement.
			if _, statErr := os.Lstat(to); statErr == nil {
				err = os.ErrExist
			}
		}
		return &os.LinkError{Op: "rename-noreplace", Old: from, New: to, Err: err}
	}
	return nil
}

// projectImportDirectoryIdentity opens the final component without following a
// reparse point, then obtains identity and attributes from that same handle.
// The caller must still guard ancestors against reparse points or replacement.
func projectImportDirectoryIdentity(path string) (string, error) {
	nativePath, err := projectImportWindowsPath(path)
	if err != nil {
		return "", &os.PathError{Op: "directory-identity", Path: path, Err: err}
	}
	handle, err := windows.CreateFile(nativePath, windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return "", &os.PathError{Op: "directory-identity", Path: path, Err: err}
	}
	defer windows.CloseHandle(handle)
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		return "", &os.PathError{Op: "directory-identity", Path: path, Err: err}
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 || info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 {
		return "", &os.PathError{Op: "directory-identity", Path: path, Err: os.ErrInvalid}
	}
	// FILE_ID_INFO supplies the complete identifier on NTFS and ReFS. The
	// older 64-bit file index is not guaranteed unique on ReFS, so a volume
	// unable to provide the full identity is unsupported rather than guessed.
	var identity struct {
		VolumeSerialNumber uint64
		FileID             [16]byte
	}
	if err := windows.GetFileInformationByHandleEx(handle, windows.FileIdInfo,
		(*byte)(unsafe.Pointer(&identity)), uint32(unsafe.Sizeof(identity))); err != nil {
		return "", &os.PathError{Op: "directory-identity", Path: path, Err: err}
	}
	if identity.FileID == [16]byte{} {
		return "", &os.PathError{Op: "directory-identity", Path: path, Err: errors.ErrUnsupported}
	}
	return fmt.Sprintf("windows:%x:%x", identity.VolumeSerialNumber, identity.FileID), nil
}

// Direct Windows calls need extended-length names even when Go's os helpers
// already support a long configured projects root. This does not change the
// separate process-working-directory limit for Git.
func projectImportWindowsPath(path string) (*uint16, error) {
	if path == "" {
		return nil, os.ErrInvalid
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	abs = filepath.Clean(abs)
	switch {
	case strings.HasPrefix(abs, `\\?\`):
		// Already extended-length, including an extended UNC name.
	case strings.HasPrefix(abs, `\\.\`):
		return nil, os.ErrInvalid
	case strings.HasPrefix(abs, `\\`):
		abs = `\\?\UNC\` + strings.TrimPrefix(abs, `\\`)
	default:
		abs = `\\?\` + abs
	}
	return windows.UTF16PtrFromString(abs)
}
