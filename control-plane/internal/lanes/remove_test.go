package lanes

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/gitx/gitxtest"
)

// TestRemoveTreeDeletesReadOnlyObjects is a regression pin (carried t01-D6): RemoveTree has no
// read-only walk, and os.RemoveAll deletes git's read-only packs on both toolchains.
func TestRemoveTreeDeletesReadOnlyObjects(t *testing.T) {
	t.Parallel()
	p := gitxtest.NewRepo(t)
	p.Git("gc", "-q")
	clone := gitxtest.CloneOf(t, p, "main")
	forced := 0
	err := filepath.WalkDir(filepath.Join(clone.Dir, ".git", "objects"), func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		forced++
		return os.Chmod(path, 0o444)
	})
	if err != nil || forced == 0 {
		t.Fatalf("forcing read-only: %d files, %v", forced, err)
	}
	if err := RemoveTree(filepath.Dir(clone.Dir), clone.Dir); err != nil {
		t.Fatalf("RemoveTree: %v", err)
	}
	if _, err := os.Lstat(clone.Dir); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("%s still exists: %v", clone.Dir, err)
	}
}

func TestRemoveTreeRefusesOutsideRoot(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "worktrees")
	keep := filepath.Join(root, "p", "x-aaaaaa", "f")
	writeFile(t, keep, "keep\n")
	sibling := filepath.Join(filepath.Dir(root), "sibling")
	writeFile(t, filepath.Join(sibling, "f"), "keep\n")
	for _, p := range []string{filepath.Dir(root), root, sibling, filepath.Join(root, "..", "sibling"), ""} {
		if err := RemoveTree(root, p); err == nil {
			t.Errorf("RemoveTree(root, %q) = nil, want a refusal", p)
		}
	}
	// A link between root and the path would carry the delete outside root.
	outside := t.TempDir()
	writeFile(t, filepath.Join(outside, "lane", "f"), "outside\n")
	dirLink(t, outside, filepath.Join(root, "linked"))
	if err := RemoveTree(root, filepath.Join(root, "linked", "lane")); err == nil {
		t.Error("RemoveTree through a directory link = nil, want a refusal")
	}
	for _, f := range []string{keep, filepath.Join(sibling, "f"), filepath.Join(outside, "lane", "f")} {
		if _, err := os.Stat(f); err != nil {
			t.Errorf("%s was deleted: %v", f, err)
		}
	}
}
