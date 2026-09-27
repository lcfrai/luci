package lanes

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/gitx/gitxtest"
)

// refuseRow sets up the project (and, after the lane exists, the clone), names the rel to copy,
// and checks afterwards that nothing was written.
type refuseRow struct {
	name   string
	branch string
	before func(t *testing.T, p *gitxtest.Repo)         // before the lane is created
	after  func(t *testing.T, p *gitxtest.Repo, l Lane) // after it, before the copy
	rel    string
	check  func(t *testing.T, p *gitxtest.Repo, l Lane) // nothing written
}

func unchanged(rel, want string) func(t *testing.T, p *gitxtest.Repo, l Lane) {
	return func(t *testing.T, p *gitxtest.Repo, l Lane) {
		if got := readFile(t, filepath.Join(l.Path, filepath.FromSlash(rel))); got != want {
			t.Errorf("clone %s = %q, want %q unchanged", rel, got, want)
		}
	}
}

func absent(rel string) func(t *testing.T, p *gitxtest.Repo, l Lane) {
	return func(t *testing.T, p *gitxtest.Repo, l Lane) {
		if _, err := os.Lstat(filepath.Join(l.Path, filepath.FromSlash(rel))); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("clone %s exists after a refused copy (%v)", rel, err)
		}
	}
}

func huskyFiles(t *testing.T, dir string, n, size int) {
	t.Helper()
	for i := 0; i < n; i++ {
		writeFile(t, filepath.Join(dir, "h"+strconv.Itoa(i)), strings.Repeat("x", size))
	}
}

func refuseRows() []refuseRow {
	gitHooks := func(rel string) refuseRow {
		var before map[string]string
		return refuseRow{name: rel, rel: rel,
			after: func(t *testing.T, p *gitxtest.Repo, l Lane) {
				before = listDir(t, filepath.Join(l.Path, ".git", "hooks"))
			},
			check: func(t *testing.T, p *gitxtest.Repo, l Lane) {
				if got := listDir(t, filepath.Join(l.Path, ".git", "hooks")); !reflect.DeepEqual(got, before) {
					t.Errorf("clone .git/hooks changed: %v, was %v", keys(got), keys(before))
				}
			}}
	}
	return []refuseRow{
		{name: "tracked", rel: ".githooks",
			before: func(t *testing.T, p *gitxtest.Repo) { p.Commit("hooks", map[string]string{".githooks/x": "v1\n"}) },
			after: func(t *testing.T, p *gitxtest.Repo, l Lane) {
				writeFile(t, filepath.Join(p.Dir, ".githooks", "x"), "v2\n")
			},
			check: unchanged(".githooks/x", "v1\n")},
		{name: "tracked on the lane's branch only", branch: "feat", rel: ".githooks",
			before: func(t *testing.T, p *gitxtest.Repo) {
				p.Git("checkout", "-q", "-b", "feat")
				p.Commit("hooks", map[string]string{".githooks/x": "feat\n"})
				p.Git("checkout", "-q", "main")
				writeFile(t, filepath.Join(p.Dir, ".githooks", "x"), "untracked\n")
			},
			check: unchanged(".githooks/x", "feat\n")},
		gitHooks(".git/hooks"), gitHooks(".GIT/hooks"), gitHooks(".git./hooks"),
		// The clone has .git/hooks, so its not-exist guard also holds the rows above. Here the
		// clone lacks .git/custom. Task 03's M22x (no trailing-dot trim, no SameFile walk) still
		// passes this row: Win32 appears to strip trailing dots from a path's last component only,
		// so an inner `.git.` names no directory (the likely mechanism, not probed directly).
		{name: ".git./custom, which the clone lacks", rel: ".git./custom",
			before: func(t *testing.T, p *gitxtest.Repo) { huskyFiles(t, filepath.Join(p.Dir, ".git", "custom"), 1, 4) },
			check:  absent(".git/custom")},
		{name: "husky a directory link", rel: ".husky/_",
			before: func(t *testing.T, p *gitxtest.Repo) {
				outside := t.TempDir()
				huskyFiles(t, filepath.Join(outside, "_"), 1, 4)
				dirLink(t, outside, filepath.Join(p.Dir, ".husky"))
			},
			check: absent(".husky/_")},
		{name: "a symlink file inside", rel: ".husky/_",
			before: func(t *testing.T, p *gitxtest.Repo) {
				huskyFiles(t, filepath.Join(p.Dir, ".husky", "_"), 1, 4)
				target := filepath.Join(t.TempDir(), "secret")
				writeFile(t, target, "outside\n")
				if err := os.Symlink(target, filepath.Join(p.Dir, ".husky", "_", "l")); err != nil {
					t.Skipf("this user cannot make a file symlink: %v", err)
				}
			},
			check: absent(".husky/_")},
		{name: "a subdirectory inside", rel: ".husky/_",
			before: func(t *testing.T, p *gitxtest.Repo) {
				huskyFiles(t, filepath.Join(p.Dir, ".husky", "_"), 1, 4)
				huskyFiles(t, filepath.Join(p.Dir, ".husky", "_", "sub"), 1, 4)
			},
			check: absent(".husky/_")},
		{name: "65 files", rel: ".husky/_",
			before: func(t *testing.T, p *gitxtest.Repo) { huskyFiles(t, filepath.Join(p.Dir, ".husky", "_"), 65, 1) },
			check:  absent(".husky/_")},
		{name: "1 MiB and 1 byte", rel: ".husky/_",
			before: func(t *testing.T, p *gitxtest.Repo) {
				huskyFiles(t, filepath.Join(p.Dir, ".husky", "_"), 1, maxHookBytes+1)
			},
			check: absent(".husky/_")},
		{name: "untracked and not ignored", rel: ".githooks",
			before: func(t *testing.T, p *gitxtest.Repo) { huskyFiles(t, filepath.Join(p.Dir, ".githooks"), 2, 4) },
			check:  absent(".githooks")},
		// review m-4: the take-back removes the parent the copy created, not only <rel>.
		{name: "untracked and not ignored, under a parent the clone lacks", rel: "hooks/x",
			before: func(t *testing.T, p *gitxtest.Repo) { huskyFiles(t, filepath.Join(p.Dir, "hooks", "x"), 2, 4) },
			check:  absent("hooks")},
		// carried b03-2 Minor 3: the clone side's parent is a link (a host that makes links).
		{name: "the clone's parent a directory link", rel: ".husky/_",
			before: func(t *testing.T, p *gitxtest.Repo) {
				huskyFiles(t, filepath.Join(p.Dir, ".husky", "_"), 1, 4)
				writeFile(t, filepath.Join(p.Dir, ".husky", "_", ".gitignore"), "*\n")
			},
			after: func(t *testing.T, p *gitxtest.Repo, l Lane) {
				outside := t.TempDir()
				dirLink(t, outside, filepath.Join(l.Path, ".husky"))
				t.Cleanup(func() {
					if ents, _ := os.ReadDir(outside); len(ents) != 0 {
						t.Errorf("the copy wrote %d entries outside the clone", len(ents))
					}
				})
			}},
	}
}

func TestCopyGeneratedHooksRefuses(t *testing.T) {
	t.Parallel()
	for _, row := range refuseRows() {
		row := row
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			p := gitxtest.NewRepo(t)
			if row.before != nil {
				row.before(t, p)
			}
			r := newRig(t, p)
			branch := row.branch
			if branch == "" {
				branch = "main"
			}
			l := r.mustCreate(branch, "r")
			if row.after != nil {
				row.after(t, p, l)
			}
			n, err := CopyGeneratedHooks(context.Background(), r.c.Machine, p.Dir, l.Path, row.rel)
			if n != 0 || err != nil {
				t.Errorf("CopyGeneratedHooks(%q) = %d, %v; want 0, nil", row.rel, n, err)
			}
			if row.check != nil {
				row.check(t, p, l)
			}
		})
	}
}

// TestLocalFSRefusesAGitSegment reaches LocalFS's own `.git`-segment refusal, which the package
// CopyGeneratedHooks' refusal otherwise stands in front of (env task 21's route calls LocalFS
// directly). A `.git` below the top is no project `.git` by os.SameFile, so only this refusal
// holds it: a nested repository's, or a vendored one's (task 03 review m-7).
func TestLocalFSRefusesAGitSegment(t *testing.T) {
	t.Parallel()
	for _, rel := range []string{"sub/.git/custom", "sub/.GIT/custom"} {
		projectTop, cloneTop := t.TempDir(), t.TempDir()
		huskyFiles(t, filepath.Join(projectTop, "sub", ".git", "custom"), 1, 4)
		n, err := LocalFS{}.CopyGeneratedHooks(context.Background(), projectTop, cloneTop, rel)
		if n != 0 || err != nil {
			t.Errorf("LocalFS.CopyGeneratedHooks(%q) = %d, %v; want 0, nil", rel, n, err)
		}
		if ents, _ := os.ReadDir(cloneTop); len(ents) != 0 {
			t.Errorf("%q: the clone holds %d entries after a refused copy", rel, len(ents))
		}
	}
}
