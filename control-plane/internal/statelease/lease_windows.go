//go:build windows

package statelease

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Acquire owns an OS lock until the returned handle closes or the process exits.
// The persistent lock file is never deleted, and opening it never truncates it.
func Acquire(dir, releaseVersion string) (*os.File, error) {
	volume := filepath.VolumeName(dir)
	if !filepath.IsAbs(dir) || filepath.Clean(dir) != dir || len(volume) != 2 || volume[1] != ':' || dir == volume+`\` || releaseVersion == "" {
		return nil, errors.New("state lease requires a private absolute local state directory and release version")
	}
	parts := strings.Split(strings.TrimPrefix(dir[len(volume):], `\`), `\`)
	for _, part := range parts {
		// Reject Win32 aliases and alternate streams before using NT names.
		if part == "" || strings.ContainsAny(part, ":/") || strings.TrimRight(part, ". ") != part || !filepath.IsLocal(part) {
			return nil, errors.New("state directory must use ordinary local directory names")
		}
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, err
	}
	// Match the existing private-path convention for new objects. Existing
	// profile directories may also grant SYSTEM/Administrators access; inspect
	// their actual ACL rather than rewriting the user's directory permissions.
	sd, err := windows.SecurityDescriptorFromString("O:" + user.User.Sid.String() + "D:P(A;OICI;FA;;;" + user.User.Sid.String() + ")")
	if err != nil {
		return nil, err
	}
	volumeName, err := windows.UTF16PtrFromString(volume + `\`)
	if err != nil {
		return nil, err
	}
	root, err := windows.CreateFile(volumeName, windows.FILE_READ_ATTRIBUTES|windows.FILE_TRAVERSE|windows.SYNCHRONIZE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, fmt.Errorf("open state volume: %w", err)
	}
	// Retain every parent without delete sharing while traversing. Each next
	// name is relative to that handle, so a concurrent junction/rename cannot
	// retarget the path between validation and opening the lease.
	directories := []windows.Handle{root}
	defer func() {
		for i := len(directories) - 1; i >= 0; i-- {
			windows.CloseHandle(directories[i])
		}
	}()
	for _, part := range parts {
		next, openErr := openLeaseChild(root, part, windows.FILE_READ_ATTRIBUTES|windows.FILE_TRAVERSE|windows.READ_CONTROL|windows.SYNCHRONIZE, windows.FILE_DIRECTORY_FILE, windows.FILE_OPEN_IF, sd)
		if openErr != nil {
			return nil, fmt.Errorf("state directory must contain ordinary directories without junctions or symbolic links: %w", openErr)
		}
		directories = append(directories, next)
		var info windows.ByHandleFileInformation
		if err := windows.GetFileInformationByHandle(next, &info); err != nil {
			return nil, err
		}
		if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 || info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 {
			return nil, errors.New("state directory must not contain junctions or symbolic links")
		}
		root = next
	}
	if err := privateLeaseHandle(root, user.User.Sid); err != nil {
		return nil, fmt.Errorf("state directory must be private to this Windows user: %w", err)
	}
	for _, marker := range []struct{ name, message string }{
		{".luci-host-deleted", "managed host state was marked for erasure; inspect the operator receipt before commissioning a new empty state directory"},
		{".luci-host-restore-incomplete", "host restore is incomplete; retain the partial state and recover into an empty directory before starting Luci"},
	} {
		h, err := openLeaseChild(root, marker.name, windows.FILE_READ_ATTRIBUTES|windows.SYNCHRONIZE, 0, windows.FILE_OPEN, nil)
		if err == nil {
			windows.CloseHandle(h)
			return nil, errors.New(marker.message)
		}
		if !errors.Is(err, windows.STATUS_OBJECT_NAME_NOT_FOUND) {
			return nil, fmt.Errorf("inspect state recovery marker: %w", err)
		}
	}
	handle, err := openLeaseChild(root, FileName, windows.FILE_GENERIC_READ|windows.FILE_GENERIC_WRITE, windows.FILE_NON_DIRECTORY_FILE, windows.FILE_OPEN_IF, sd)
	if err != nil {
		if errors.Is(err, windows.STATUS_SHARING_VIOLATION) {
			return nil, fmt.Errorf("%w: %v", ErrInUse, err)
		}
		return nil, fmt.Errorf("open state lease: %w", err)
	}
	file := os.NewFile(uintptr(handle), filepath.Join(dir, FileName))
	closeOnError := true
	defer func() {
		if closeOnError {
			file.Close()
		}
	}()
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		return nil, err
	}
	if info.FileAttributes&(windows.FILE_ATTRIBUTE_DIRECTORY|windows.FILE_ATTRIBUTE_REPARSE_POINT) != 0 || info.NumberOfLinks != 1 {
		return nil, errors.New("state lease must be a private regular file with one name")
	}
	if err := privateLeaseHandle(handle, user.User.Sid); err != nil {
		return nil, fmt.Errorf("state lease must be private to this Windows user: %w", err)
	}
	if err := windows.LockFileEx(handle, windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &windows.Overlapped{}); err != nil {
		if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
			return nil, fmt.Errorf("%w: %v", ErrInUse, err)
		}
		return nil, fmt.Errorf("lock state lease: %w", err)
	}
	data, _ := json.Marshal(struct {
		Version        int    `json:"version"`
		ReleaseVersion string `json:"releaseVersion"`
	}{1, releaseVersion})
	if err := file.Truncate(0); err != nil {
		return nil, err
	}
	if _, err := file.Write(data); err != nil {
		return nil, err
	}
	if err := file.Sync(); err != nil {
		return nil, err
	}
	closeOnError = false
	return file, nil
}

func openLeaseChild(parent windows.Handle, name string, access, kind, disposition uint32, sd *windows.SECURITY_DESCRIPTOR) (windows.Handle, error) {
	objectName, err := windows.NewNTUnicodeString(name)
	if err != nil {
		return 0, err
	}
	attributes := windows.OBJECT_ATTRIBUTES{
		RootDirectory: parent, ObjectName: objectName,
		Attributes:         windows.OBJ_CASE_INSENSITIVE | windows.OBJ_DONT_REPARSE,
		SecurityDescriptor: sd,
	}
	attributes.Length = uint32(unsafe.Sizeof(attributes))
	var handle windows.Handle
	var status windows.IO_STATUS_BLOCK
	err = windows.NtCreateFile(&handle, access, &attributes, &status, nil, windows.FILE_ATTRIBUTE_NORMAL,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, disposition,
		kind|windows.FILE_OPEN_REPARSE_POINT|windows.FILE_SYNCHRONOUS_IO_NONALERT, 0, 0)
	return handle, err
}

// Check the opened object, not its pathname. Normal private Windows profile
// ACLs include SYSTEM and Administrators; neither is a second ordinary user.
// Unknown ACE types, NULL/empty DACLs and grants to other users fail closed.
func privateLeaseHandle(handle windows.Handle, user *windows.SID) error {
	sd, err := windows.GetSecurityInfo(handle, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	owner, _, err := sd.Owner()
	if err != nil || owner == nil || !trustedLeaseSID(owner, user) {
		return errors.New("directory/file owner is another Windows user or group")
	}
	acl, _, err := sd.DACL()
	if err != nil || acl == nil || acl.AceCount == 0 {
		return errors.New("private access rules are missing")
	}
	for i := uint32(0); i < uint32(acl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(acl, i, &ace); err != nil || ace == nil || ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE || ace.Header.AceSize < uint16(unsafe.Sizeof(windows.ACCESS_ALLOWED_ACE{})) {
			return errors.New("private access rules could not be verified")
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		if !trustedLeaseSID(sid, user) {
			return errors.New("access is granted to another Windows user or group")
		}
	}
	return nil
}

func trustedLeaseSID(sid, user *windows.SID) bool {
	return sid.Equals(user) || sid.IsWellKnown(windows.WinLocalSystemSid) || sid.IsWellKnown(windows.WinBuiltinAdministratorsSid)
}
