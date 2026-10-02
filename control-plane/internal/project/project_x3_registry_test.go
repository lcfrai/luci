package project_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

// X-3 review 3 I-A. The two O-1 attack tests in project_x3_test.go use a
// project whose .git/worktrees is EMPTY, so they cannot tell "compares the
// listed directory with each registered one" from "accepts anything once one
// entry exists": MUT-P1 (the per-entry compare reduced to os.Stat(wt) == nil)
// passed every package. Most real projects have a linked worktree, so this is
// the shape that matters: a registry with an entry in it, and a steered
// listing that names that entry AND a directory the registry does not name.
func TestPathRefusesAnUnregisteredWorktreeOfAProjectThatRegistersOthers(t *testing.T) {
	root := t.TempDir()
	dir := makeRepo(t, root, "luci")
	feat := filepath.Join(t.TempDir(), "worktrees", "luci-feat")
	x3Register(t, dir, feat)
	elsewhere := filepath.Join(t.TempDir(), "dotfiles")
	if err := os.MkdirAll(elsewhere, 0o750); err != nil {
		t.Fatal(err)
	}
	g := newFakeGit()
	g.reply(dir, worktreePorcelain(dir, feat, elsewhere), "worktree", "list", "--porcelain")
	s := project.NewScanner(root, g)
	ctx := context.Background()

	// The registry is read and its one entry resolves, so the refusal below
	// is the per-entry compare and not an unreadable registry.
	if got, err := s.Path(ctx, project.WorktreeID(feat)); err != nil || got != feat {
		t.Fatalf("Path of the registered worktree = %q, %v; want %q", got, err, feat)
	}
	got, err := s.Path(ctx, project.WorktreeID(elsewhere))
	if !errors.Is(err, project.ErrNotRegistered) {
		t.Fatalf("Path of a directory the registry does not name = %q, %v; want project.ErrNotRegistered", got, err)
	}
	if got != "" {
		t.Fatalf("a refusal must hand back no path to mount, got %q", got)
	}
}

// The same attack end to end with real git, the reviewer's reproduction: a
// project WITH a real linked worktree, then the one agent-written line in
// <project>/.git/commondir that makes `git worktree list` name another
// repository and ITS linked worktree. Every steered id must be refused.
func TestPathRefusesACommondirSteeredWorktreeOfAProjectWithARealLinkedWorktree(t *testing.T) {
	requireGit(t)
	root := longTemp(t)
	app := filepath.Join(root, "app")
	makeGitRepo(t, app)
	appFeat := filepath.Join(longTemp(t), "app-feat")
	gitIn(t, app, "worktree", "add", appFeat, "evidence-upload")
	victim := filepath.Join(longTemp(t), "victim")
	makeGitRepo(t, victim)
	victimFeat := filepath.Join(longTemp(t), "victim-feat")
	gitIn(t, victim, "worktree", "add", victimFeat, "evidence-upload")

	s := project.NewScanner(root, project.NewExecRunner())
	ctx := context.Background()
	if got, err := s.Path(ctx, project.WorktreeID(appFeat)); err != nil || got != appFeat {
		t.Fatalf("before the steer, Path of app's own linked worktree = %q, %v; want %q", got, err, appFeat)
	}

	steer := filepath.ToSlash(filepath.Join(victim, ".git")) + "\n"
	if err := os.WriteFile(filepath.Join(app, ".git", "commondir"), []byte(steer), 0o600); err != nil {
		t.Fatal(err)
	}
	listed, err := s.Worktrees(ctx, project.ProjectID(app))
	if err != nil {
		t.Fatalf("Worktrees after the steer: %v", err)
	}
	want := map[string]bool{project.WorktreeID(victim): true, project.WorktreeID(victimFeat): true}
	for _, w := range listed {
		delete(want, w.ID)
	}
	if len(want) != 0 {
		t.Fatalf("the steering premise did not reproduce: after the commondir line, git listed %#v, not victim and victim-feat", listed)
	}
	for _, w := range listed {
		got, err := s.Path(ctx, w.ID)
		if !errors.Is(err, project.ErrNotRegistered) || got != "" {
			t.Errorf("Path of steered %s = %q, %v; want \"\" and project.ErrNotRegistered", w.Path, got, err)
		}
	}
}

// gitIn runs real git in dir with the developer's config neutralised, as
// makeGitRepo does.
func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cfg := t.TempDir()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL="+filepath.Join(cfg, "gitconfig"),
		"GIT_CONFIG_SYSTEM="+filepath.Join(cfg, "gitconfig-system"),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

// longTemp is t.TempDir() in the long form git prints (see
// TestScannerAgainstARealRepositoryAndARealLinkedWorktree for why).
func longTemp(t *testing.T) string {
	t.Helper()
	d, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return d
}
