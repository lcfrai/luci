//go:build windows

package statelease

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestWindowsLeaseRetainsNormalPrivateProfileACL(t *testing.T) {
	parent := t.TempDir()
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	// Match the inherited profile ACL of an ordinary Windows desktop user.
	setLeaseTestACL(t, parent, "D:P(A;OICI;FA;;;"+user.User.Sid.String()+")(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)")
	dir := filepath.Join(parent, "existing-state")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	before := leaseTestACL(t, dir)
	lease, err := Acquire(dir, "test")
	if err != nil {
		t.Fatal(err)
	}
	if got := leaseTestACL(t, dir); got != before {
		t.Fatal("existing private profile ACL was rewritten")
	}
	if err := os.Remove(filepath.Join(dir, FileName)); err == nil {
		t.Fatal("live lease can be deleted")
	}
	if err := os.Rename(filepath.Join(dir, FileName), filepath.Join(dir, "moved-lock")); err == nil {
		t.Fatal("live lease can be renamed")
	}
	if second, err := Acquire(strings.ToUpper(dir), "test"); !errors.Is(err, ErrInUse) {
		if second != nil {
			second.Close()
		}
		t.Fatalf("case alias bypassed state ownership: %v", err)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Acquire(dir, "test")
	if err != nil {
		t.Fatal(err)
	}
	reopened.Close()
}

func TestWindowsLeaseRejectsJunctionsLinksAndPublicACLs(t *testing.T) {
	for _, kind := range []string{"junction-root", "junction-parent", "symlink-file", "hardlinked-file", "public-root", "public-file", "directory-file"} {
		t.Run(kind, func(t *testing.T) {
			parent := t.TempDir()
			dir := filepath.Join(parent, "state")
			if err := os.Mkdir(dir, 0700); err != nil {
				t.Fatal(err)
			}
			lock := filepath.Join(dir, FileName)
			target := filepath.Join(parent, "untouched")
			if err := os.WriteFile(target, []byte("retain existing bytes"), 0600); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "junction-root", "junction-parent":
				link := filepath.Join(parent, "junction")
				destination := dir
				if kind == "junction-parent" {
					destination = parent
				}
				if out, err := exec.Command("cmd", "/d", "/c", "mklink", "/J", link, destination).CombinedOutput(); err != nil {
					t.Fatalf("create ordinary-user junction: %v %s", err, out)
				}
				t.Cleanup(func() { os.Remove(link) })
				dir = link
				if kind == "junction-parent" {
					dir = filepath.Join(link, "state")
				}
			case "symlink-file":
				if err := os.Symlink(target, lock); err != nil {
					if errors.Is(err, windows.ERROR_PRIVILEGE_NOT_HELD) {
						t.Skip("Windows file symlinks need Developer Mode or symlink privilege; junction cases run without it")
					}
					t.Fatal(err)
				}
			case "hardlinked-file":
				if err := os.Link(target, lock); err != nil {
					t.Fatal(err)
				}
			case "public-root":
				setLeaseTestACL(t, dir, "D:P(A;OICI;FA;;;WD)")
			case "public-file":
				if err := os.WriteFile(lock, []byte("retain existing lock"), 0600); err != nil {
					t.Fatal(err)
				}
				setLeaseTestACL(t, lock, "D:P(A;;FA;;;WD)")
			case "directory-file":
				if err := os.Mkdir(lock, 0700); err != nil {
					t.Fatal(err)
				}
			}
			if lease, err := Acquire(dir, "must-not-write"); err == nil {
				lease.Close()
				t.Fatal("unsafe state path accepted")
			}
			if data, err := os.ReadFile(target); err != nil || string(data) != "retain existing bytes" {
				t.Fatalf("rejected path changed target bytes: %q %v", data, err)
			}
			if kind == "public-file" {
				if data, err := os.ReadFile(lock); err != nil || string(data) != "retain existing lock" {
					t.Fatal("unsafe lock metadata was overwritten")
				}
			}
		})
	}
}

func TestWindowsLeaseRejectsAliasesAndRetiredRoots(t *testing.T) {
	for _, path := range []string{`C:\`, `relative\state`, `C:\state.`, `C:\state `, `C:\state:stream`, `\\server\share\state`, `\\?\C:\state`} {
		if lease, err := Acquire(path, "test"); err == nil {
			lease.Close()
			t.Fatalf("unsafe root accepted: %q", path)
		}
	}
	for _, marker := range []string{".luci-host-deleted", ".luci-host-restore-incomplete"} {
		t.Run(marker, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "state")
			if err := os.Mkdir(dir, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, marker), []byte("retain"), 0600); err != nil {
				t.Fatal(err)
			}
			if lease, err := Acquire(dir, "test"); err == nil {
				lease.Close()
				t.Fatal("retired/interrupted state accepted")
			}
			if _, err := os.Stat(filepath.Join(dir, FileName)); !os.IsNotExist(err) {
				t.Fatal("lease wrote before refusing retired/interrupted state")
			}
		})
	}
}

func setLeaseTestACL(t *testing.T, path, descriptor string) {
	t.Helper()
	sd, err := windows.SecurityDescriptorFromString(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	acl, _, err := sd.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil); err != nil {
		t.Fatal(err)
	}
}

func leaseTestACL(t *testing.T, path string) string {
	t.Helper()
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	return sd.String()
}

func TestWindowsLeaseRetainsPrivateAdministratorOwnedRoot(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "admin-owned-state")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	admin, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		t.Fatal(err)
	}
	// An ordinary desktop user need not be able to assign Administrators as
	// owner. Exercise elevated/installer ownership only when already allowed.
	if err := windows.SetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION, admin, nil, nil, nil); err != nil {
		if errors.Is(err, windows.ERROR_INVALID_OWNER) || errors.Is(err, windows.ERROR_ACCESS_DENIED) || errors.Is(err, windows.ERROR_PRIVILEGE_NOT_HELD) {
			t.Skip("current Windows token cannot assign an administrator owner")
		}
		t.Fatal(err)
	}
	before := leaseTestACL(t, dir)
	lease, err := Acquire(dir, "test")
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	if leaseTestACL(t, dir) != before {
		t.Fatal("safe administrator-owned root security was rewritten")
	}
}

func TestWindowsLeaseRetainsOwnerRightsPrivateACL(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "owner-rights-state")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	sd, err := windows.GetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	owner, _, err := sd.Owner()
	if err != nil || owner == nil || !owner.Equals(user.User.Sid) {
		t.Fatal("fixture must belong to the current Windows user")
	}
	// Matches a fresh Python mode-0700 directory: OWNER RIGHTS, SYSTEM,
	// Administrators. Do not rewrite it into an explicit per-user grant.
	setLeaseTestACL(t, dir, "D:P(A;OICI;FA;;;OW)(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)")
	beforeRoot := leaseTestACL(t, dir)
	first, err := Acquire(dir, "test")
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	lock := filepath.Join(dir, FileName)
	setLeaseTestACL(t, lock, "D:P(A;;FA;;;OW)(A;;FA;;;SY)(A;;FA;;;BA)")
	beforeLock := leaseTestACL(t, lock)
	sentinel := filepath.Join(dir, "existing-state")
	if err := os.WriteFile(sentinel, []byte("retain existing state"), 0600); err != nil {
		t.Fatal(err)
	}
	lease, err := Acquire(dir, "test")
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	if second, err := Acquire(dir, "test"); !errors.Is(err, ErrInUse) {
		if second != nil {
			second.Close()
		}
		t.Fatalf("OWNER RIGHTS bypassed exclusive ownership: %v", err)
	}
	if leaseTestACL(t, dir) != beforeRoot || leaseTestACL(t, lock) != beforeLock {
		t.Fatal("existing OWNER RIGHTS permissions were rewritten")
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Acquire(dir, "test")
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if leaseTestACL(t, dir) != beforeRoot || leaseTestACL(t, lock) != beforeLock {
		t.Fatal("restart changed existing OWNER RIGHTS permissions")
	}
	if data, err := os.ReadFile(sentinel); err != nil || string(data) != "retain existing state" {
		t.Fatal("contention/restart changed existing state")
	}
}

func TestWindowsLeaseOwnerRightsRequireActualCurrentOwner(t *testing.T) {
	token, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	user := token.User.Sid
	rights, err := windows.CreateWellKnownSid(windows.WinCreatorOwnerRightsSid)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		t.Fatal(err)
	}
	world, err := windows.CreateWellKnownSid(windows.WinWorldSid)
	if err != nil {
		t.Fatal(err)
	}
	other, err := windows.StringToSid("S-1-5-21-1-2-3-4242")
	if err != nil {
		t.Fatal(err)
	}
	if trustedLeaseSID(rights, user) {
		t.Fatal("OWNER RIGHTS must never be accepted as an object's owner")
	}
	for _, test := range []struct {
		name       string
		ace, owner *windows.SID
		allowed    bool
	}{
		{"current-owner", rights, user, true},
		{"different-user-owner", rights, other, false},
		{"administrator-owner", rights, admin, false},
		{"owner-rights-is-not-owner", rights, rights, false},
		{"public-ace", world, user, false},
		{"different-user-ace", other, user, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := trustedLeaseAccessSID(test.ace, test.owner, user); got != test.allowed {
				t.Fatalf("access principal/actual-owner relationship: got %v, want %v", got, test.allowed)
			}
		})
	}
}

func TestWindowsLeaseOwnerRightsDoNotConcealPublicAccess(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "owner-rights-with-public-access")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	setLeaseTestACL(t, dir, "D:P(A;OICI;FA;;;OW)(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;GR;;;WD)")
	before := leaseTestACL(t, dir)
	if lease, err := Acquire(dir, "must-not-write"); err == nil {
		lease.Close()
		t.Fatal("OWNER RIGHTS concealed a grant to Everyone")
	}
	if leaseTestACL(t, dir) != before {
		t.Fatal("unsafe existing permissions were rewritten")
	}
	if _, err := os.Stat(filepath.Join(dir, FileName)); !os.IsNotExist(err) {
		t.Fatal("unsafe root was opened for lease metadata writes")
	}
}
