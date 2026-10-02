package product

import (
	"golang.org/x/sys/windows"
	"strings"
	"testing"
)

func assertPrivateFile(t *testing.T, path string) {
	t.Helper()
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	want := "D:P(A;;FA;;;" + user.User.Sid.String() + ")"
	// Windows may retain the auto-inheritance bookkeeping flag AI while P
	// still disables inheritance; both forms contain exactly our single ACE.
	if sd.String() != want && sd.String() != strings.Replace(want, "D:P", "D:PAI", 1) {
		t.Fatalf("credential ACL is %q, expected protected owner-only access %q", sd.String(), want)
	}
}
