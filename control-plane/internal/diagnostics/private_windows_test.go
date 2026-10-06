//go:build windows

package diagnostics_test

import (
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestRecorderWindowsUsesProtectedCurrentUserOnlyACL(t *testing.T) {
	state := t.TempDir()
	r := openRecorder(t, state)
	for i := 0; i < 2; i++ {
		record(t, r, testCause{"native_state_link"})
		for _, path := range []string{filepath.Dir(recorderPath(state)), recorderPath(state)} {
			sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
			if err != nil {
				t.Fatal(err)
			}
			user, err := windows.GetCurrentProcessToken().GetTokenUser()
			if err != nil {
				t.Fatal(err)
			}
			got := strings.Replace(sd.String(), "D:PAI", "D:P", 1)
			fileACL := "D:P(A;;FA;;;" + user.User.Sid.String() + ")"
			inheritedACL := "D:P(A;OICI;FA;;;" + user.User.Sid.String() + ")"
			if got != fileACL && got != inheritedACL {
				t.Fatalf("diagnostic ACL is not protected current-user-only: %q", sd.String())
			}
		}
	}
}
