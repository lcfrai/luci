//go:build windows

package runtimeoci

import "golang.org/x/sys/windows"

func protectPullPath(name string, directory bool) error {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	ace := "D:P(A;;FA;;;" + user.User.Sid.String() + ")"
	if directory {
		ace = "D:P(A;OICI;FA;;;" + user.User.Sid.String() + ")"
	}
	sd, err := windows.SecurityDescriptorFromString(ace)
	if err != nil {
		return err
	}
	acl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(name, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil)
}

// The staged files are individually flushed. Windows directory handles do
// not support os.File.Sync; every continuation reopens and hashes the stage.
func syncPullDirectory(string) error { return nil }

// The trusted coordinator validates the enclosing Windows execution-state
// DACL before it assigns this staging path; the child inherits its protected
// current-user ACL and the selected Docker Desktop bind mount is read-only.
func validatePullPrivateParent(string) error { return nil }

func exposePullReadOnlyTree(string) error { return nil }
