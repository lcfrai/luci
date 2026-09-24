//go:build windows

package api

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// FILE_DISPOSITION_INFO_EX's flags (winbase.h). x/sys names the information
// class, FileDispositionInfoEx, but not its flags.
const (
	sessionDispositionDelete         = 0x00000001
	sessionDispositionIgnoreReadOnly = 0x00000010
)

// sessionRemoveIn deletes name in root's subdirectory dir, and nowhere else:
// what goes is the file at root\dir\name when it is opened, whatever dir has
// become since the caller looked at it (why: sessionForgetTranscripts).
//
// On Windows the file is opened for deletion without following a link at its
// own name — FILE_FLAG_OPEN_REPARSE_POINT, so a link there is itself deleted
// and its target is not — and the filesystem is asked where the file it opened
// really is. The deletion is made on that handle, and only when the answer is
// root\dir\name. A link swapped in for dir is followed by the open, so the file
// opened is somewhere else, and nothing is deleted. root itself is resolved as
// it stands: it is the store, the mount's root, and no sandbox can replace it.
//
// As os.Remove does, an empty directory at the name is removed and a read-only
// file is deleted anyway.
func sessionRemoveIn(root, dir, name string) error {
	p := filepath.Join(root, dir, name)
	base, err := sessionFinalPath(root, 0)
	if err != nil {
		return &os.PathError{Op: "remove", Path: p, Err: err}
	}
	h, err := sessionOpenHandle(p, windows.DELETE|windows.FILE_READ_ATTRIBUTES, windows.FILE_FLAG_OPEN_REPARSE_POINT)
	if err != nil {
		return &os.PathError{Op: "remove", Path: p, Err: err}
	}
	defer windows.CloseHandle(h)
	got, err := sessionHandlePath(h)
	if err != nil {
		return &os.PathError{Op: "remove", Path: p, Err: err}
	}
	if want := filepath.Join(base, dir, name); !strings.EqualFold(got, want) {
		return fmt.Errorf("remove %s: the file that opened is %s, not %s; deleted nothing through it", p, got, want)
	}
	if err := sessionDeleteByHandle(h); err != nil {
		return &os.PathError{Op: "remove", Path: p, Err: err}
	}
	return nil
}

// sessionOpenHandle opens path with access, sharing everything, so an open
// here never stands in another process's way. Directories open too
// (FILE_FLAG_BACKUP_SEMANTICS), which is how an empty one is removed.
func sessionOpenHandle(path string, access, flags uint32) (windows.Handle, error) {
	u, err := windows.UTF16PtrFromString(sessionLongPath(path))
	if err != nil {
		return windows.InvalidHandle, err
	}
	return windows.CreateFile(u, access,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|flags, 0)
}

// sessionLongPath is an absolute path in the \\?\ form, which Win32 takes past
// MAX_PATH whether or not long paths are enabled on the machine. The os
// package does the same for its own calls when they are not; these are raw
// CreateFile, and on such a machine a store deep enough would fail to open as
// "path not found" — which a Forget reads as "already gone", and answers 204
// over a transcript it never reached. (Where long paths are enabled, as on the
// machine this was written on, the prefix changes nothing.)
func sessionLongPath(path string) string {
	if !filepath.IsAbs(path) || strings.HasPrefix(path, `\\?\`) || strings.HasPrefix(path, `\\.\`) {
		return path
	}
	path = filepath.Clean(path)
	if strings.HasPrefix(path, `\\`) {
		return `\\?\UNC\` + path[2:]
	}
	return `\\?\` + path
}

// sessionFinalPath is where path really is, links in it followed: the form
// sessionHandlePath gives an open file, so the two compare.
func sessionFinalPath(path string, flags uint32) (string, error) {
	h, err := sessionOpenHandle(path, windows.FILE_READ_ATTRIBUTES, flags)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(h)
	return sessionHandlePath(h)
}

// sessionHandlePath is the normalized path, drive letter and long names, of
// the file h is open on.
func sessionHandlePath(h windows.Handle) (string, error) {
	buf := make([]uint16, windows.MAX_PATH)
	for {
		// Flags 0: FILE_NAME_NORMALIZED | VOLUME_NAME_DOS.
		n, err := windows.GetFinalPathNameByHandle(h, &buf[0], uint32(len(buf)), 0)
		if err != nil {
			return "", err
		}
		if n < uint32(len(buf)) {
			return windows.UTF16ToString(buf[:n]), nil
		}
		buf = make([]uint16, n)
	}
}

// sessionDeleteByHandle marks the file h is open on for deletion, which
// happens when h closes. The Ex class ignores the read-only attribute, as
// os.Remove does by clearing it; a filesystem or Windows older than 1809
// without it gets the plain class.
func sessionDeleteByHandle(h windows.Handle) error {
	flags := uint32(sessionDispositionDelete | sessionDispositionIgnoreReadOnly)
	err := windows.SetFileInformationByHandle(h, windows.FileDispositionInfoEx, (*byte)(unsafe.Pointer(&flags)), uint32(unsafe.Sizeof(flags)))
	if err == nil {
		return nil
	}
	if !errors.Is(err, windows.ERROR_INVALID_PARAMETER) && !errors.Is(err, windows.ERROR_INVALID_FUNCTION) && !errors.Is(err, windows.ERROR_NOT_SUPPORTED) {
		return err
	}
	del := byte(1) // FILE_DISPOSITION_INFO.DeleteFile
	return windows.SetFileInformationByHandle(h, windows.FileDispositionInfo, &del, 1)
}
