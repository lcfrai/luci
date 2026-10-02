package project_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

// x3Register makes wt a linked worktree of dir the way git does: a directory
// under <dir>/.git/worktrees whose gitdir file names the worktree's .git.
func x3Register(t *testing.T, dir, wt string) {
	t.Helper()
	entry := filepath.Join(dir, ".git", "worktrees", filepath.Base(wt))
	if err := os.MkdirAll(entry, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(wt, 0o750); err != nil {
		t.Fatal(err)
	}
	body := filepath.ToSlash(filepath.Join(wt, ".git")) + "\n"
	if err := os.WriteFile(filepath.Join(entry, "gitdir"), []byte(body), 0o640); err != nil {
		t.Fatal(err)
	}
}

// X-3 / X-1 review O-1, the attack itself. Phase 2 mounts the primary worktree
// read-WRITE with its .git included, so the agent writes one line into
// <primary>/.git/commondir and `git worktree list --porcelain` in that
// directory then names ANOTHER repository's worktrees — measured at the git
// level by the X-1 reviewer. The fake git plays back exactly that: a directory
// that exists, that the host owns, and that this project does not register.
// A create on it used to mount it read-write.
func TestPathRefusesAWorktreeTheProjectDoesNotRegister(t *testing.T) {
	root := t.TempDir()
	dir := makeRepo(t, root, "luci")
	// ~/dotfiles, the reviewer's worst realistic target: a repository whose
	// worktree is the home directory.
	elsewhere := filepath.Join(t.TempDir(), "dotfiles")
	if err := os.MkdirAll(elsewhere, 0o750); err != nil {
		t.Fatal(err)
	}
	g := newFakeGit()
	g.reply(dir, worktreePorcelain(dir, elsewhere), "worktree", "list", "--porcelain")

	got, err := project.NewScanner(root, g).Path(context.Background(), project.WorktreeID(elsewhere))
	if !errors.Is(err, project.ErrNotRegistered) {
		t.Fatalf("Path of a worktree the project does not register = %q, %v; want project.ErrNotRegistered", got, err)
	}
	if got != "" {
		t.Fatalf("a refusal must hand back no path to mount, got %q", got)
	}
}

// Inside the projects directory too, which is where this differs from the
// guard RESUME.md names: another repository under the same root is still not
// this project's worktree, and a read-write mount of it is still wrong.
func TestPathRefusesAForeignRepositoryInsideTheProjectsDirectory(t *testing.T) {
	root := t.TempDir()
	dir := makeRepo(t, root, "luci")
	other := makeRepo(t, root, "someone-elses")
	g := newFakeGit()
	g.reply(dir, worktreePorcelain(dir, other), "worktree", "list", "--porcelain")

	if got, err := project.NewScanner(root, g).Path(context.Background(), project.WorktreeID(other)); !errors.Is(err, project.ErrNotRegistered) {
		t.Fatalf("Path of another project's directory = %q, %v; want project.ErrNotRegistered", got, err)
	}
}

// The legitimate case, and it must keep working exactly as it is: the e2e
// fixture puts linked worktrees OUTSIDE the projects root on purpose, because
// a linked worktree's .git is a file and one inside the root would enumerate
// as a second project (desktop/e2e/fixture.sh). Registered, so it resolves.
func TestPathResolvesARegisteredLinkedWorktreeOutsideTheProjectsDirectory(t *testing.T) {
	root := t.TempDir()
	dir := makeRepo(t, root, "e2e-demo")
	linked := filepath.Join(t.TempDir(), "worktrees", "e2e-demo-feat")
	x3Register(t, dir, linked)
	g := newFakeGit()
	g.reply(dir, worktreePorcelain(dir, linked), "worktree", "list", "--porcelain")

	got, err := project.NewScanner(root, g).Path(context.Background(), project.WorktreeID(linked))
	if err != nil {
		t.Fatalf("Path of a registered linked worktree outside the root: %v", err)
	}
	if got != linked {
		t.Fatalf("Path = %q, want %q", got, linked)
	}
}

// The registry and the listing are two names for one place, so they are
// compared as places. Here git names a link and the registry names its target.
func TestPathResolvesARegisteredWorktreeReachedByAnotherName(t *testing.T) {
	root := t.TempDir()
	dir := makeRepo(t, root, "luci")
	real := filepath.Join(t.TempDir(), "evidence-upload")
	x3Register(t, dir, real)
	link := filepath.Join(t.TempDir(), "by-another-name")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("this platform will not make a directory symlink here: %v", err)
	}
	g := newFakeGit()
	g.reply(dir, worktreePorcelain(dir, link), "worktree", "list", "--porcelain")

	if _, err := project.NewScanner(root, g).Path(context.Background(), project.WorktreeID(link)); err != nil {
		t.Fatalf("Path of a registered worktree named through a link: %v", err)
	}
}
