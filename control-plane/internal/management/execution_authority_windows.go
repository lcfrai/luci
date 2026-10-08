//go:build windows

package management

import (
	"golang.org/x/sys/windows"
	"os"
	"unsafe"
)

func executionIdentityFilePrivate(f *os.File, info os.FileInfo) bool {
	if !info.Mode().IsRegular() {
		return false
	}
	handle := windows.Handle(f.Fd())
	var details windows.ByHandleFileInformation
	if windows.GetFileInformationByHandle(handle, &details) != nil || details.NumberOfLinks != 1 {
		return false
	}
	sd, e := windows.GetSecurityInfo(handle, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if e != nil {
		return false
	}
	control, _, e := sd.Control()
	if e != nil || control&windows.SE_DACL_PROTECTED == 0 {
		return false
	}
	acl, _, e := sd.DACL()
	if e != nil || acl == nil || acl.AceCount == 0 {
		return false
	}
	user, e := windows.GetCurrentProcessToken().GetTokenUser()
	if e != nil {
		return false
	}
	for i := uint32(0); i < uint32(acl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if windows.GetAce(acl, i, &ace) != nil || ace == nil || ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE || ace.Header.AceSize < uint16(unsafe.Sizeof(windows.ACCESS_ALLOWED_ACE{})) {
			return false
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		if !sid.Equals(user.User.Sid) {
			return false
		}
	}
	return true
}
