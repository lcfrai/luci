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

// X-3 / X-1 review O-1, the attack itself. Phase 2 mounts the primary worktree
// read-WRITE with its .git included, so the agent writes one line into
// <primary>/.git/commondir and `git worktree list --porcelain` in that
// directory then names ANOTHER repository's worktrees — measured at the git
// level by the X-1 reviewer. The fake git here plays back exactly that output:
// a directory that exists, that the host owns, and that is not the project's.
// A create on it used to mount it read-write.
func TestPathRefusesAWorktreeGitNamedOutsideTheProjectsDirectory(t *testing.T) {
	root := t.TempDir()
	dir := makeRepo(t, root, "panacea")
	// ~/dotfiles, the reviewer's worst realistic target: a repository whose
	// worktree is the home directory.
	elsewhere := filepath.Join(t.TempDir(), "dotfiles")
	if err := os.MkdirAll(elsewhere, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	g := newFakeGit()
	g.reply(dir, worktreePorcelain(dir, elsewhere), "worktree", "list", "--porcelain")

	got, err := project.NewScanner(root, g).Path(context.Background(), project.WorktreeID(elsewhere))
	if !errors.Is(err, hostpath.ErrOutside) {
		t.Fatalf("Path of a worktree git named outside the projects directory = %q, %v; want an error wrapping hostpath.ErrOutside", got, err)
	}
	if got != "" {
		t.Fatalf("a refusal must hand back no path to mount, got %q", got)
	}
}

// The same answer must not be reachable by name either: a link INSIDE the
// projects directory whose target is outside it. worktree.Verify alone says
// yes — Stat follows the link and sees a directory — so a lexical prefix check
// would pass the mount through.
func TestPathRefusesAWorktreeInsideTheProjectsDirectoryThatLinksOutOfIt(t *testing.T) {
	root := t.TempDir()
	dir := makeRepo(t, root, "panacea")
	elsewhere := filepath.Join(t.TempDir(), "dotfiles")
	if err := os.MkdirAll(elsewhere, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	link := filepath.Join(root, "looks-like-a-worktree")
	if err := os.Symlink(elsewhere, link); err != nil {
		t.Skipf("this platform will not make a directory symlink here: %v", err)
	}
	g := newFakeGit()
	g.reply(dir, worktreePorcelain(dir, link), "worktree", "list", "--porcelain")

	if got, err := project.NewScanner(root, g).Path(context.Background(), project.WorktreeID(link)); !errors.Is(err, hostpath.ErrOutside) {
		t.Fatalf("Path of a link out of the projects directory = %q, %v; want an error wrapping hostpath.ErrOutside", got, err)
	}
}

// The legitimate case the guard must not cost: a real linked worktree inside
// the projects directory, which is where the e2e fixture puts one
// (desktop/e2e/fixture.sh: `git worktree add "$root/worktrees/e2e-demo-feat"`).
func TestPathStillResolvesALinkedWorktreeInsideTheProjectsDirectory(t *testing.T) {
	root := t.TempDir()
	dir := makeRepo(t, root, "panacea")
	linked := filepath.Join(root, "worktrees", "evidence-upload")
	if err := os.MkdirAll(linked, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	g := newFakeGit()
	g.reply(dir, worktreePorcelain(dir, linked), "worktree", "list", "--porcelain")

	got, err := project.NewScanner(root, g).Path(context.Background(), project.WorktreeID(linked))
	if err != nil {
		t.Fatalf("Path of a linked worktree under the projects directory: %v", err)
	}
	if got != linked {
		t.Fatalf("Path = %q, want %q", got, linked)
	}
}
