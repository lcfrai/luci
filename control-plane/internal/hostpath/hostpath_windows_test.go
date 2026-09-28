//go:build windows

package hostpath

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// shortPath is the 8.3 alias Windows keeps for p, or a skip when the volume
// does not keep one.
func shortPath(t *testing.T, p string) string {
	t.Helper()
	long, err := syscall.UTF16PtrFromString(p)
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]uint16, 1024)
	n, err := syscall.GetShortPathName(long, &buf[0], uint32(len(buf)))
	if err != nil || n == 0 || int(n) > len(buf) {
		t.Skipf("this volume keeps no 8.3 name for %q: %v", p, err)
	}
	short := syscall.UTF16ToString(buf[:n])
	if strings.EqualFold(short, p) {
		t.Skipf("8.3 names are off on this volume: %q", p)
	}
	return short
}

// The walk compares each step to the projects directory by FILE IDENTITY, and
// this is why. Measured while writing X-3: os.MkdirTemp here answers
// C:\Users\AU-DEV~1\… and filepath.EvalSymlinks answers
// C:\Users\au-dev-dmackie1\… for the same directory, so a prefix comparison of
// names gave two different answers for one place. An agent writes the link
// target, so it writes the 8.3 form, and a name check then let the chain walk
// straight through the projects directory.
func TestAnEightDotThreeNameForAStepInsideTheRootIsStillInsideIt(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "ProjectsDirectory")
	repo := filepath.Join(root, "dotfilesrepository")
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(base, "id_rsa")
	writeFile(t, secret, []byte("PRIVATE KEY"))
	hop := filepath.Join(repo, "gitignore")
	linkOrSkip(t, secret, hop)

	short := shortPath(t, hop)
	if !strings.Contains(short, "~") {
		t.Skipf("no 8.3 component in %q, so this host cannot show the bypass", short)
	}
	named := filepath.Join(base, "config", "git", "ignore")
	linkOrSkip(t, short, named)

	if got, err := ResolveOutside(root, named); !errors.Is(err, ErrInside) {
		t.Fatalf("ResolveOutside through the 8.3 name %q = %q, %v; want ErrInside", short, got, err)
	}
}
