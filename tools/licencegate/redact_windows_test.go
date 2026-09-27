//go:build windows

package main

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// shortName is p's 8.3 spelling, or p itself on a volume that generates no
// short names (8dot3 creation can be switched off). Test-only: canon expands
// a short element, it never makes one, so only a test needs this direction.
func shortName(t *testing.T, p string) string {
	t.Helper()
	u, err := syscall.UTF16PtrFromString(p)
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]uint16, 1024)
	n, err := syscall.GetShortPathName(u, &buf[0], uint32(len(buf)))
	if err != nil {
		t.Fatalf("GetShortPathName(%s): %v", p, err)
	}
	return syscall.UTF16ToString(buf[:n])
}

func mustCanon(t *testing.T, p string) string {
	t.Helper()
	c, err := canon(p)
	if err != nil {
		t.Fatalf("canon(%s): %v", p, err)
	}
	return c
}

// TestNewRestorer_theCanonSpellingOfALinkTarget covers redact.go's second
// place per link target, npmPlace{place, place} (fix round 2, review m2:
// deleting it left the suite green). A package-lock "resolved" is written by
// a person and may hold an 8.3 element; the gate joins it to the project
// directory and gets a short spelling, while npm prints the node's path in
// the long one. Only the canon-spelling place matches what npm printed, and
// the filesystem cannot stand in for it: the printed path's redacted element
// is in the prefix, and two sibling UUID directories redact to the same
// "***", so unredact refuses to guess.
func TestNewRestorer_theCanonSpellingOfALinkTarget(t *testing.T) {
	const uuidA = "11111111-2222-3333-4444-555555555555"
	const uuidB = "66666666-7777-8888-9999-000000000000"
	tmp := t.TempDir()
	root := filepath.Join(tmp, uuidA)
	proj := filepath.Join(root, "proj")
	long := filepath.Join(root, "vendorlongdirectoryname")
	node := filepath.Join(long, "b", "node_modules", "dep")
	for _, d := range []string{filepath.Join(tmp, uuidB), proj, node} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	short := filepath.Base(shortName(t, long))
	if short == filepath.Base(long) {
		t.Skip("this volume generates no 8.3 names: a lockfile cannot hold a short element here")
	}
	t.Logf("the lockfile's short element: %s", short)

	locks := map[string]lockEntry{"node_modules/b": {Resolved: "../" + short + "/b", Link: true}}
	r := newRestorer(proj, mustCanon(t, proj), locks)

	want := mustCanon(t, node)
	printed := npmRedact(want) // what npm ls prints: the long spelling, UUID gone
	got, err := r.restore(printed)
	if err != nil {
		t.Fatalf("restore(%s): %v", printed, err)
	}
	if !sameCanon(got, want) {
		t.Errorf("restore(%s) = %s, want %s", printed, got, want)
	}
}
