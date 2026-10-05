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

// A natural new-project journey saves planning files before the first agent.
// Enumeration must expose the primary worktree without committing those files
// or requiring a Git identity first.
func TestInitialProjectBranchAndWorktreeRemainUsableBeforeFirstCommit(t *testing.T) {
	requireGit(t)
	for _, name := range []string{"main", "release/green"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "new-app")
			if err := os.Mkdir(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("git", "-c", "init.templateDir=", "init", "--initial-branch="+name, ".")
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+filepath.Join(t.TempDir(), "gitconfig"), "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0")
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("initialize owned fixture: %v %s", err, output)
			}
			if err := os.WriteFile(filepath.Join(dir, "LUCI.md"), []byte("# My saved definition\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			runner := project.NewExecRunner()
			scanner := project.NewScanner(root, runner).WithProjectBoundary()
			pid := project.ProjectID(dir)
			for attempt := 0; attempt < 2; attempt++ {
				branches, err := scanner.Branches(ctx, pid)
				if err != nil || len(branches) != 1 || branches[0].Name != name || !branches[0].IsHead || branches[0].ID != project.BranchID(pid, name) {
					t.Fatalf("current initial branch: %#v %v", branches, err)
				}
				worktrees, err := scanner.Worktrees(ctx, pid)
				if err != nil || len(worktrees) != 1 || !worktrees[0].IsPrimary || worktrees[0].BranchID != branches[0].ID {
					t.Fatalf("initial primary worktree: %#v %v", worktrees, err)
				}
				path, err := scanner.Path(ctx, worktrees[0].ID)
				if err != nil {
					t.Fatalf("first-session mount resolution: %q %v", path, err)
				}
				actual, err := os.Stat(path)
				if err != nil {
					t.Fatal(err)
				}
				owned, err := os.Stat(dir)
				if err != nil || !os.SameFile(actual, owned) {
					t.Fatal("first-session mount resolved outside its owned primary worktree")
				}
			}
			if _, err := runner.Run(ctx, dir, "rev-parse", "--verify", "--quiet", "HEAD"); err == nil {
				t.Fatal("read-only enumeration fabricated the first commit")
			}
			refs, err := runner.Run(ctx, dir, "for-each-ref", "--format=%(refname)", "refs/heads")
			if err != nil || len(refs) != 0 {
				t.Fatalf("initial branch acquired a commit ref: %q %v", refs, err)
			}
			status, err := runner.Run(ctx, dir, "status", "--porcelain=v1")
			if err != nil || strings.TrimSpace(string(status)) != "?? LUCI.md" {
				t.Fatalf("saved planning work was modified: %q %v", status, err)
			}
		})
	}
}

func TestInitialBranchRequiresPositiveValidSymbolicRef(t *testing.T) {
	for _, scenario := range []string{"missing symbolic ref", "foreign ref", "invalid local ref", "symbolic read cancelled"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			dir := makeRepo(t, root, "new-app")
			g := newFakeGit()
			g.reply(dir, "", "for-each-ref", "--format=%(refname:short)", "refs/heads")
			g.fail(dir, errors.New("HEAD has no commit"), "rev-parse", "--abbrev-ref", "HEAD")
			g.fail(dir, &project.GitError{Err: errors.New("exit status 1"), Exited: true, ExitedNonZero: true}, "rev-parse", "--verify", "--quiet", "HEAD")
			switch scenario {
			case "missing symbolic ref":
				g.fail(dir, errors.New("no symbolic HEAD"), "symbolic-ref", "--quiet", "HEAD")
			case "foreign ref":
				g.reply(dir, "refs/tags/main\n", "symbolic-ref", "--quiet", "HEAD")
			case "invalid local ref":
				g.reply(dir, "refs/heads/bad..name\n", "symbolic-ref", "--quiet", "HEAD")
				g.fail(dir, errors.New("invalid ref"), "check-ref-format", "refs/heads/bad..name")
			case "symbolic read cancelled":
				g.fail(dir, context.Canceled, "symbolic-ref", "--quiet", "HEAD")
			}
			if branches, err := branchesOf(t, root, dir, g); err == nil || len(branches) != 0 {
				t.Fatal("an unverified initial branch became a usable branch")
			}
		})
	}
}
