package project_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hostpath"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/project"
)

// x3ReparseJunction makes a Windows junction (a reparse point that is NOT a
// symlink, mklink /J, no privilege) or skips: a security claim this host cannot
// exercise must not read as proven.
func x3ReparseJunction(t *testing.T, target, link string) {
	t.Helper()
	if runtime.GOOS != "windows" {
		t.Skip("a junction is a Windows reparse point")
	}
	if err := os.RemoveAll(link); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput(); err != nil {
		t.Skipf("this host will not make a junction at %s: %v: %s", link, err, out)
	}
}

// X-3 review 3 I-B. registers() os.Stats THROUGH whatever sits at the registered
// location, so a nested worktree location swapped for a symlink to an OUTSIDE
// directory still registers (both sides resolve to the outside directory). The
// location sits inside the primary's read-WRITE mount, so the entry is the
// agent's to swap, and Path used to hand the unresolved location to the mount.
// Only ConfineToMount refuses it. Red-first: without the guard Path returns the
// location and the mount follows it out of the worktree.
func TestPathRefusesANestedWorktreeLocationSwappedForASymlink(t *testing.T) {
	root := t.TempDir()
	dir := makeRepo(t, root, "app")
	nested := filepath.Join(dir, ".worktrees", "feat")
	x3Register(t, dir, nested)
	outside := filepath.Join(t.TempDir(), "elsewhere")
	if err := os.MkdirAll(outside, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(nested); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, nested); err != nil {
		t.Skipf("this platform will not make a directory symlink here: %v", err)
	}
	g := newFakeGit()
	g.reply(dir, worktreePorcelain(dir, nested), "worktree", "list", "--porcelain")

	got, err := project.NewScanner(root, g).Path(context.Background(), project.WorktreeID(nested))
	if !errors.Is(err, hostpath.ErrReparsePoint) {
		t.Fatalf("Path of a nested location swapped for a symlink = %q, %v; want hostpath.ErrReparsePoint", got, err)
	}
	if got != "" {
		t.Fatalf("a refusal must hand back no path to mount, got %q", got)
	}
}

// The junction is the weaker attacker's tool (no privilege on Windows) and
// EvalSymlinks does not resolve it, so the mask must refuse os.ModeIrregular too.
func TestPathRefusesANestedWorktreeLocationSwappedForAJunction(t *testing.T) {
	root := t.TempDir()
	dir := makeRepo(t, root, "app")
	nested := filepath.Join(dir, ".worktrees", "feat")
	x3Register(t, dir, nested)
	outside := filepath.Join(t.TempDir(), "elsewhere")
	if err := os.MkdirAll(outside, 0o750); err != nil {
		t.Fatal(err)
	}
	x3ReparseJunction(t, outside, nested)
	g := newFakeGit()
	g.reply(dir, worktreePorcelain(dir, nested), "worktree", "list", "--porcelain")

	got, err := project.NewScanner(root, g).Path(context.Background(), project.WorktreeID(nested))
	if !errors.Is(err, hostpath.ErrReparsePoint) {
		t.Fatalf("Path of a nested location swapped for a junction = %q, %v; want hostpath.ErrReparsePoint", got, err)
	}
	if got != "" {
		t.Fatalf("a refusal must hand back no path to mount, got %q", got)
	}
}

// The guard must not refuse a real nested worktree. A registered linked worktree
// inside the projects directory, every component a real directory, resolves —
// otherwise the guard would break the one layout §3a names as legitimate.
func TestPathResolvesARealNestedRegisteredWorktree(t *testing.T) {
	root := t.TempDir()
	dir := makeRepo(t, root, "app")
	nested := filepath.Join(dir, ".worktrees", "feat")
	x3Register(t, dir, nested)
	g := newFakeGit()
	g.reply(dir, worktreePorcelain(dir, nested), "worktree", "list", "--porcelain")

	got, err := project.NewScanner(root, g).Path(context.Background(), project.WorktreeID(nested))
	if err != nil {
		t.Fatalf("Path of a real nested registered worktree: %v", err)
	}
	if got != nested {
		t.Fatalf("Path = %q, want %q", got, nested)
	}
}
