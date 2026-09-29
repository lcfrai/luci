package project_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/project"
)

// X-3 review 4 I-D, the one read-write mount that is no worktree: every
// sandbox gets its worktree's conversation store, <ConversationsDir>/<owner>/
// <worktree id>, read-write. A location registered under that root is an
// agent's to swap like one in a worktree, so main.go names the root to the
// scanner (WithWritableDirs) and Path confines against it too.
func TestPathRefusesALocationInsideADirectorySandboxesWrite(t *testing.T) {
	root := t.TempDir()
	app := makeRepo(t, root, "app")
	store := filepath.Join(t.TempDir(), "conversations")
	loc := filepath.Join(store, "local", "wt-app")
	x3Register(t, app, loc)
	x3ReparseJunction(t, outsideDir(t), loc)
	g := newFakeGit()
	g.reply(app, worktreePorcelain(app, loc), "worktree", "list", "--porcelain")

	wantPathRefused(t, project.NewScanner(root, g).WithWritableDirs(store), loc, "inside a conversation store")
}

// FAIL CLOSED on the projects directory as a whole, not only the projects it
// lists: a directory there that is no project now (its .git gone, or never a
// repository) may be what a running session mounts, and nothing under it is
// proved host-owned. A registered location inside one, swapped for a
// junction, is refused.
func TestPathRefusesALocationAnywhereUnderTheProjectsDirectory(t *testing.T) {
	root := t.TempDir()
	app := makeRepo(t, root, "app")
	loc := filepath.Join(root, "notes", "wt-app")
	x3Register(t, app, loc)
	x3ReparseJunction(t, outsideDir(t), loc)
	g := newFakeGit()
	g.reply(app, worktreePorcelain(app, loc), "worktree", "list", "--porcelain")

	wantPathRefused(t, project.NewScanner(root, g), loc, "under the projects directory, in no project")
}

// FAIL CLOSED: a registry entry that names no worktree leaves unknown a
// directory that project's sessions may write, so no location is confined
// against a boundary known to be incomplete. Path fails, naming the fix.
func TestPathFailsWhenARegistryEntryNamesNoWorktree(t *testing.T) {
	root := t.TempDir()
	app := makeRepo(t, root, "app")
	other := makeRepo(t, root, "other")
	if err := os.MkdirAll(filepath.Join(other, ".git", "worktrees", "half-made"), 0o750); err != nil {
		t.Fatal(err)
	}
	g := newFakeGit()
	g.reply(app, worktreePorcelain(app), "worktree", "list", "--porcelain")
	g.reply(other, worktreePorcelain(other), "worktree", "list", "--porcelain")

	got, err := project.NewScanner(root, g).Path(context.Background(), project.WorktreeID(app))
	if err == nil || got != "" {
		t.Fatalf("Path with another project's registry entry naming no worktree = %q, %v; want \"\" and an error", got, err)
	}
}
