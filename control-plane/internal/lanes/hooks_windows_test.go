package lanes

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// TestCopyGeneratedHooksRefusesTheShortNameOfDotGit (carried b03-2 Minor 2): Windows opens a
// directory by its 8.3 name, so `GIT~1/hooks` reads the project's .git/hooks though no segment
// spells `.git`. The project-side walk compares each component with <projectTop>/.git by
// os.SameFile. The row needs 8.3 names on the temp volume, and skips, saying so, without them.
func TestCopyGeneratedHooksRefusesTheShortNameOfDotGit(t *testing.T) {
	projectTop, cloneTop := t.TempDir(), t.TempDir()
	gitDir := filepath.Join(projectTop, ".git")
	huskyFiles(t, filepath.Join(gitDir, "hooks"), 1, 4)
	short := shortName(t, gitDir)
	if strings.EqualFold(short, ".git") {
		t.Skipf("8.3 names are off on this volume (%s has no short name)", gitDir)
	}
	rel := short + "/hooks"
	if fi, err := os.Lstat(filepath.Join(projectTop, short)); err != nil || !fi.IsDir() {
		t.Fatalf("precondition: %s does not open .git: %v", rel, err)
	}
	n, err := LocalFS{}.CopyGeneratedHooks(context.Background(), projectTop, cloneTop, rel)
	if n != 0 || err != nil {
		t.Errorf("CopyGeneratedHooks(%q) = %d, %v; want 0, nil", rel, n, err)
	}
	if ents, _ := os.ReadDir(cloneTop); len(ents) != 0 {
		t.Errorf("the refused copy wrote %d entries into the clone", len(ents))
	}
}

func shortName(t *testing.T, path string) string {
	t.Helper()
	long, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]uint16, 1024)
	n, err := syscall.GetShortPathName(long, &buf[0], uint32(len(buf)))
	if err != nil || n == 0 {
		t.Fatalf("GetShortPathName(%s): %v", path, err)
	}
	return filepath.Base(syscall.UTF16ToString(buf[:n]))
}
