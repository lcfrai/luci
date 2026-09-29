package project_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hostpath"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/project"
)

// wantPathRefused asserts that Path of wt fails as a reparse point and hands
// back nothing to mount.
func wantPathRefused(t *testing.T, s *project.Scanner, wt, why string) {
	t.Helper()
	got, err := s.Path(context.Background(), project.WorktreeID(wt))
	if !errors.Is(err, hostpath.ErrReparsePoint) || got != "" {
		t.Fatalf("%s: Path(%q) = %q, %v; want \"\" and hostpath.ErrReparsePoint", why, wt, got, err)
	}
}

// outsideDir is a directory outside every mount, a junction's target, made
// with the subdirectories sub below it.
func outsideDir(t *testing.T, sub ...string) string {
	t.Helper()
	d := filepath.Join(t.TempDir(), "elsewhere")
	if err := os.MkdirAll(filepath.Join(append([]string{d}, sub...)...), 0o750); err != nil {
		t.Fatal(err)
	}
	return d
}

// X-3 review 4 I-E, lifted: a COMPONENT between the project and the nested
// location (.worktrees) is the junction, not the leaf. Every earlier I-B attack
// swapped the leaf, so MUT-LEAF (refuse only the leaf) passed internal/hostpath
// and internal/project whole; this is the test that kills it.
func TestPathRefusesANestedWorktreeReachedThroughAJunctionedComponent(t *testing.T) {
	root := t.TempDir()
	dir := makeRepo(t, root, "app")
	nested := filepath.Join(dir, ".worktrees", "feat")
	x3Register(t, dir, nested)
	x3ReparseJunction(t, outsideDir(t, "feat"), filepath.Join(dir, ".worktrees"))
	g := newFakeGit()
	g.reply(dir, worktreePorcelain(dir, nested), "worktree", "list", "--porcelain")

	wantPathRefused(t, project.NewScanner(root, g), nested, "a junctioned component")
}

// X-3 review 4 I-D, lifted: project app's linked worktree is registered INSIDE
// project other's tree, which other's own sessions mount read-write. An agent
// there swaps the location for a junction. With only app's primary as the
// boundary the walk never entered a mount, and Path returned the junction.
func TestPathRefusesALocationInsideAnotherProjectsTree(t *testing.T) {
	root := t.TempDir()
	app := makeRepo(t, root, "app")
	other := makeRepo(t, root, "other")
	loc := filepath.Join(other, "wt-app")
	x3Register(t, app, loc)
	x3ReparseJunction(t, outsideDir(t), loc)
	g := newFakeGit()
	g.reply(app, worktreePorcelain(app, loc), "worktree", "list", "--porcelain")
	g.reply(other, worktreePorcelain(other), "worktree", "list", "--porcelain")

	wantPathRefused(t, project.NewScanner(root, g), loc, "inside another project's read-write tree")
}
