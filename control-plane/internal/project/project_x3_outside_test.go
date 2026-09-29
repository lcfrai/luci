package project_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/project"
)

// nestedInOutsideLinked is the e2e fixture's layout (a linked worktree outside
// the projects directory, desktop/e2e/fixture.sh) with a second worktree made
// from inside it where host-side Claude Code makes one by default,
// .claude/worktrees/. Both are registered by app, and git lists both.
func nestedInOutsideLinked(t *testing.T) (root, nested string, g *fakeGit) {
	t.Helper()
	root = t.TempDir()
	app := makeRepo(t, root, "app")
	linked := filepath.Join(t.TempDir(), "worktrees", "app-feat")
	x3Register(t, app, linked)
	nested = filepath.Join(linked, ".claude", "worktrees", "w2")
	x3Register(t, app, nested)
	g = newFakeGit()
	g.reply(app, worktreePorcelain(app, linked, nested), "worktree", "list", "--porcelain")
	return root, nested, g
}

// X-3 review 4 I-D, the likelier layout: the outside linked worktree is mounted
// read-write by its own sessions, so the nested location's entry is the agent's
// to swap. The boundary was app's primary only, which this path never enters.
func TestPathRefusesAWorktreeNestedInAnOutsideLinkedWorktree(t *testing.T) {
	root, nested, g := nestedInOutsideLinked(t)
	x3ReparseJunction(t, outsideDir(t), nested)

	wantPathRefused(t, project.NewScanner(root, g), nested, "nested in an outside linked worktree")
}

// The guard must not cost that layout when nothing was swapped.
func TestPathResolvesAWorktreeNestedInAnOutsideLinkedWorktree(t *testing.T) {
	root, nested, g := nestedInOutsideLinked(t)

	got, err := project.NewScanner(root, g).Path(context.Background(), project.WorktreeID(nested))
	if err != nil || got != nested {
		t.Fatalf("Path of a real worktree nested in an outside linked worktree = %q, %v; want %q", got, err, nested)
	}
}

// A path is walked the way the operating system walks it. Here git's listing
// spells the nested location through a junction the HOST made outside every
// mount (alias -> app/.worktrees), and the location itself is the agent's
// junction. A walk that only Lstats each name as spelled never enters app, so
// it passed; following the host's junction enters app by identity and refuses.
func TestPathRefusesALocationSpelledThroughAHostJunctionIntoAMount(t *testing.T) {
	root := t.TempDir()
	app := makeRepo(t, root, "app")
	nested := filepath.Join(app, ".worktrees", "feat")
	x3Register(t, app, nested)
	x3ReparseJunction(t, outsideDir(t), nested)
	alias := filepath.Join(t.TempDir(), "alias")
	x3ReparseJunction(t, filepath.Join(app, ".worktrees"), alias)
	spelled := filepath.Join(alias, "feat")
	g := newFakeGit()
	g.reply(app, worktreePorcelain(app, spelled), "worktree", "list", "--porcelain")

	wantPathRefused(t, project.NewScanner(root, g), spelled, "spelled through the host's own junction")
}
