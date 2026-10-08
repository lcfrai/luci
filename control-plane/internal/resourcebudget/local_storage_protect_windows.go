//go:build windows

package resourcebudget

import (
	"golang.org/x/sys/windows"
	"os"
	"syscall"
)

func protectLocalStorageFile(f *os.File) error {
	user, e := windows.GetCurrentProcessToken().GetTokenUser()
	if e != nil {
		return e
	}
	sd, e := windows.SecurityDescriptorFromString("D:P(A;;FA;;;" + user.User.Sid.String() + ")")
	if e != nil {
		return e
	}
	acl, _, e := sd.DACL()
	if e != nil {
		return e
	}
	// os.OpenFile's write handle lacks WRITE_DAC. Reopen the exact file
	// object with that right instead of resolving a mutable path to set ACLs.
	proc := kernel.NewProc("ReOpenFile")
	raw, _, callErr := proc.Call(f.Fd(), uintptr(windows.WRITE_DAC|windows.READ_CONTROL), uintptr(windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE), 0)
	if raw == ^uintptr(0) {
		if callErr == syscall.Errno(0) {
			return syscall.EACCES
		}
		return callErr
	}
	h := windows.Handle(raw)
	defer windows.CloseHandle(h)
	return windows.SetSecurityInfo(h, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil)
}
