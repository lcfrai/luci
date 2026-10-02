package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/project"
	"github.com/lcfrai/luci/control-plane/internal/worktree"
)

// listingGit is a project.Runner double: `git worktree list --porcelain` in
// dir answers listing, and every other invocation fails, so no git runs.
type listingGit struct{ dir, listing string }

func (g listingGit) Run(ctx context.Context, dir string, args ...string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if dir == g.dir && strings.Join(args, " ") == "worktree list --porcelain" {
		return []byte(g.listing), nil
	}
	return nil, fmt.Errorf("listingGit: no scripted reply for %q in %q", strings.Join(args, " "), dir)
}

var _ project.Runner = listingGit{}

// makeJunction replaces link with a Windows junction to target (mklink /J, no
// privilege) or skips: a security claim this host cannot exercise must not
// read as proven.
func makeJunction(t *testing.T, target, link string) {
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

// registerLinkedWorktree makes wt a linked worktree of the repository at dir
// the way git records one: <dir>/.git/worktrees/<name>/gitdir names wt/.git.
func registerLinkedWorktree(t *testing.T, dir, wt string) {
	t.Helper()
	entry := filepath.Join(dir, ".git", "worktrees", filepath.Base(wt))
	for _, d := range []string{entry, wt} {
		if err := os.MkdirAll(d, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	body := filepath.ToSlash(filepath.Join(wt, ".git")) + "\n"
	if err := os.WriteFile(filepath.Join(entry, "gitdir"), []byte(body), 0o640); err != nil {
		t.Fatal(err)
	}
	for path, value := range map[string]string{
		filepath.Join(wt, ".git"):         "gitdir: " + filepath.ToSlash(entry) + "\n",
		filepath.Join(entry, "commondir"): "../..\n",
	} {
		if err := os.WriteFile(path, []byte(value), 0o640); err != nil {
			t.Fatal(err)
		}
	}
}

// porcelainOf is `git worktree list --porcelain` naming dirs, the primary
// first, in git's forward slashes.
func porcelainOf(dirs ...string) string {
	var b strings.Builder
	for i, d := range dirs {
		fmt.Fprintf(&b, "worktree %s\nHEAD %040d\nbranch refs/heads/wt-%d\n\n", filepath.ToSlash(d), i, i)
	}
	return b.String()
}

// X-3 review 5 C-1. Every sandbox mounts its worktree's conversation store,
// <ConversationsDir>/<owner>/<worktree id>, read-WRITE, so a worktree location
// registered inside the stores' root is the agent's to swap for a junction
// (X-3 review 4 I-D). Only newScanner names that root to the scanner, and with
// the call dropped every suite stayed green (MUT-WIRE). The config is
// loadConfig's, so the root is the one serve hands api.Deps too. Each case is
// resolved once as a real directory, the control that the scanner finds it,
// and then refused with the junction at the location or on the way to it.
func TestNewScannerRefusesAWorktreeSwappedInsideAConversationStore(t *testing.T) {
	for _, tc := range []struct{ name, swap string }{
		{"the location itself", "wt-app"},
		{"a directory on the way", "local"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			projects := t.TempDir()
			cfg, err := loadConfig(env(map[string]string{envStateDir: t.TempDir(), envProjectsDir: projects}))
			if err != nil {
				t.Fatalf("loadConfig: %v", err)
			}
			// Deliberately overlap the fixture store and project boundary so the
			// junction guard remains exercised after outside worktrees are refused.
			cfg.ConversationsDir = filepath.Join(projects, "fixture-conversations")
			app := filepath.Join(projects, "app")
			loc := filepath.Join(cfg.ConversationsDir, "local", "wt-app")
			registerLinkedWorktree(t, app, loc)
			s, err := newScanner(cfg, listingGit{dir: app, listing: porcelainOf(app, loc)})
			if err != nil {
				t.Fatalf("newScanner: %v", err)
			}
			id := project.WorktreeID(loc)
			if got, err := s.Path(context.Background(), id); err != nil || got != loc {
				t.Fatalf("control: Path(%q) of the real directory = %q, %v; want it and no error", loc, got, err)
			}

			swapped := loc
			if tc.swap == "local" {
				swapped = filepath.Dir(loc)
			}
			outside := filepath.Join(t.TempDir(), "elsewhere")
			if err := os.MkdirAll(filepath.Join(outside, "wt-app"), 0o750); err != nil {
				t.Fatal(err)
			}
			target := outside
			if tc.swap == "wt-app" {
				target = filepath.Join(outside, "wt-app")
			}
			makeJunction(t, target, swapped)

			got, err := s.Path(context.Background(), id)
			// Strict enumeration filters unsafe paths before they can become
			// a mount candidate. The result is now an unknown worktree.
			if !errors.Is(err, worktree.ErrUnknownID) || got != "" {
				t.Fatalf("Path(%q) with %s a junction out of the store = %q, %v; want no mount and an unknown worktree", loc, swapped, got, err)
			}
		})
	}
}

// FAIL CLOSED: a root that is not absolute leaves unknown which directories
// sandboxes write, so newScanner builds nothing rather than a scanner whose
// boundary has the stores missing from it.
func TestNewScannerRefusesAConversationStoreRootThatIsNotAbsolute(t *testing.T) {
	for _, root := range []string{"", "conversations"} {
		s, err := newScanner(Config{ProjectsDir: t.TempDir(), ConversationsDir: root}, listingGit{})
		if err == nil || s != nil {
			t.Errorf("newScanner with the stores' root %q = %v, %v; want nil and an error", root, s, err)
		}
	}
}
