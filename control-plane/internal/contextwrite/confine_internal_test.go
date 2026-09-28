package contextwrite

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Review I-2's read half, at the one moment no external test reaches: a link
// swapped in between readChecked's check and its open. The restart path
// reads beside the live agent (api/sessions.go:1724), so the moment is real.
// openForRead is the seam: the swap runs inside it, just before the open.
func TestAReadRefusesALinkSwappedInAfterItsCheck(t *testing.T) {
	probe := filepath.Join(t.TempDir(), "probe")
	if err := os.Symlink(t.TempDir(), probe); err != nil {
		t.Skipf("os.Symlink refused here: %v", err)
	}
	const secret = "SECRET-host-private-key\n"
	for _, row := range []struct {
		name string
		swap func(dir, outside string) error
	}{
		// Killed by os.SameFile, or on Linux and macOS by O_NOFOLLOW first.
		{"the file itself", func(dir, outside string) error {
			p := filepath.Join(dir, ".claude", "CLAUDE.md")
			if err := os.Remove(p); err != nil {
				return err
			}
			return os.Symlink(filepath.Join(outside, "CLAUDE.md"), p)
		}},
		// O_NOFOLLOW covers only the last component. On Windows the check's
		// file ID is pinned when checked, else it would be read through the
		// new link, agree with the open, and let the read through.
		{"the directory above it", func(dir, outside string) error {
			if err := os.RemoveAll(filepath.Join(dir, ".claude")); err != nil {
				return err
			}
			return os.Symlink(outside, filepath.Join(dir, ".claude"))
		}},
	} {
		t.Run(row.name, func(t *testing.T) {
			dir, outside := t.TempDir(), t.TempDir()
			if err := os.WriteFile(filepath.Join(outside, "CLAUDE.md"), []byte(secret), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Join(dir, ".claude"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, ".claude", "CLAUDE.md"), []byte("mine\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			orig := openForRead
			t.Cleanup(func() { openForRead = orig })
			var swapErr error
			openForRead = func(p string) (*os.File, error) {
				swapErr = row.swap(dir, outside)
				return orig(p)
			}
			s, ok, err := readChecked(dir, filepath.Join(dir, ".claude", "CLAUDE.md"))
			if swapErr != nil {
				t.Fatalf("the swap: %v", swapErr)
			}
			if err == nil {
				t.Fatalf("readChecked = %q, %v, nil through a link swapped in after the check", s, ok)
			}
			if strings.Contains(s+err.Error(), "SECRET") {
				t.Fatalf("the outside file's text came back: %q, %v", s, err)
			}
		})
	}
}
