package contextwrite_test

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/api"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/claudemd"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/contextwrite"
)

// Review I-2: the worktree is the sandbox's to write, links included, and the
// host resolves a link against its own filesystem. Nothing below the
// worktree is followed, for a read or for a write.

const secret = "SECRET-host-private-key\n"

// symlink makes link -> target, or skips where this OS refuses links to an
// unprivileged process (Windows without developer mode).
func symlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("os.Symlink refused here: %v", err)
	}
}

// stillALink: path is a symlink, not replaced by a file.
func stillALink(t *testing.T, path string) {
	t.Helper()
	fi, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&fs.ModeSymlink == 0 {
		t.Fatalf("%s is no longer a link (mode %v): something was written over it", path, fi.Mode())
	}
}

// outsideFile is a file outside the worktree, aged, and a check that it is
// byte-identical with its mtime unmoved.
func outsideFile(t *testing.T, path, content string) func() {
	t.Helper()
	writeFile(t, path, content)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	return func() {
		t.Helper()
		b, err := os.ReadFile(path)
		if err != nil || string(b) != content {
			t.Fatalf("the outside file %s = %q, %v; want it byte-identical: %q", path, b, err, content)
		}
		if fi, err := os.Stat(path); err != nil || !fi.ModTime().Equal(old) {
			t.Fatalf("the outside file %s was rewritten", path)
		}
	}
}

// noSecretInside: no regular file under dir holds secret. WalkDir follows no
// link, so only what was written INTO the worktree is read.
func noSecretInside(t *testing.T, dir string) {
	t.Helper()
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return err
		}
		b, err := os.ReadFile(p)
		if err == nil && strings.Contains(string(b), strings.TrimSpace(secret)) {
			t.Errorf("%s holds the outside file's text: %q", p, b)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestALinkedCandidateIsNeverFollowed(t *testing.T) {
	type start int
	const (
		withBrief start = iota
		nothingToInherit
		withoutBrief
	)
	run := func(e *env, s start) error {
		req := e.req()
		switch s {
		case withBrief:
			e.src.set(inh(doc(idA, 6, "the brief")), nil)
		case withoutBrief:
			req.WithoutBrief = true
		}
		_, err := e.materialise(req)
		return err
	}
	names := [...]string{"with a brief", "nothing to inherit", "without the brief"}
	for _, s := range []start{withBrief, nothingToInherit, withoutBrief} {
		t.Run(names[s]+"/CLAUDE.md links to an outside file", func(t *testing.T) {
			e := newEnv(t)
			out := filepath.Join(t.TempDir(), "id_ed25519")
			same := outsideFile(t, out, secret+block(idB, 2, "older"))
			symlink(t, out, e.path(contextwrite.FileName))
			wantReason(t, run(e, s), api.BriefReasonWriteFailed)
			same()
			stillALink(t, e.path(contextwrite.FileName))
			noSecretInside(t, e.dir)
		})
		for _, claudeTracked := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/.claude links to an outside directory, CLAUDE.md tracked=%v", names[s], claudeTracked), func(t *testing.T) {
				e := newEnv(t)
				e.git.tracked[contextwrite.FileName] = claudeTracked
				home := t.TempDir() // the person's ~/.claude, say
				same := outsideFile(t, filepath.Join(home, "CLAUDE.md"), secret+block(idB, 2, "older"))
				symlink(t, home, e.path(".claude"))
				wantReason(t, run(e, s), api.BriefReasonWriteFailed)
				same()
				stillALink(t, e.path(".claude"))
				if got, _ := e.read(contextwrite.FileName); strings.Contains(got, claudemd.BeginPrefix) {
					t.Fatalf("CLAUDE.md holds a block after a refusal: %q", got)
				}
			})
		}
	}
}

func TestALinkedExcludePathIsNeverFollowed(t *testing.T) {
	t.Run(".git links to an outside directory", func(t *testing.T) {
		e := newEnv(t)
		gitDir := t.TempDir()
		same := outsideFile(t, filepath.Join(gitDir, "info", "exclude"), "outside\n")
		symlink(t, gitDir, e.path(".git"))
		e.src.set(inh(doc(idA, 6, "the brief")), nil)
		_, err := e.materialise(e.req())
		wantReason(t, err, api.BriefReasonWriteFailed)
		same()
		if _, ok := e.read(contextwrite.FileName); ok {
			t.Fatal("CLAUDE.md written with no exclude line")
		}
	})
	t.Run("the exclude file links to an outside file", func(t *testing.T) {
		e := newEnv(t)
		out := filepath.Join(t.TempDir(), "exclude")
		same := outsideFile(t, out, "outside\n")
		symlink(t, out, e.git.excludePath)
		e.src.set(inh(doc(idA, 6, "the brief")), nil)
		_, err := e.materialise(e.req())
		wantReason(t, err, api.BriefReasonWriteFailed)
		same()
		stillALink(t, e.git.excludePath)
	})
}

// A linked worktree's exclude file is its common directory's, outside the
// worktree. It is written there, and only the file, its info directory and
// the git directory holding that are checked; a Materialiser never builds a
// tree outside the worktree.
func TestAnExcludePathOutsideTheWorktree(t *testing.T) {
	t.Run("a common directory's exclude file is written", func(t *testing.T) {
		e := newEnv(t)
		common := filepath.Join(t.TempDir(), "main", ".git")
		if err := os.MkdirAll(common, 0o755); err != nil {
			t.Fatal(err)
		}
		e.git.excludePath = filepath.Join(common, "info", "exclude")
		e.src.set(inh(doc(idA, 6, "x")), nil)
		if _, err := e.materialise(e.req()); err != nil {
			t.Fatal(err)
		}
		if n := count(e.excludeLines(), "/CLAUDE.md"); n != 1 {
			t.Fatalf("exclude = %q", e.excludeLines())
		}
	})
	t.Run("no tree is built outside the worktree", func(t *testing.T) {
		e := newEnv(t)
		missing := filepath.Join(t.TempDir(), "no", "such", ".git")
		e.git.excludePath = filepath.Join(missing, "info", "exclude")
		e.src.set(inh(doc(idA, 6, "x")), nil)
		_, err := e.materialise(e.req())
		wantReason(t, err, api.BriefReasonWriteFailed)
		if _, err := os.Stat(filepath.Dir(missing)); !os.IsNotExist(err) {
			t.Fatalf("a directory was made outside the worktree: %v", err)
		}
	})
	t.Run("an info directory that is a link", func(t *testing.T) {
		e := newEnv(t)
		common := filepath.Join(t.TempDir(), ".git")
		elsewhere := t.TempDir()
		same := outsideFile(t, filepath.Join(elsewhere, "exclude"), "elsewhere\n")
		symlink(t, elsewhere, filepath.Join(common, "info"))
		e.git.excludePath = filepath.Join(common, "info", "exclude")
		e.src.set(inh(doc(idA, 6, "x")), nil)
		_, err := e.materialise(e.req())
		wantReason(t, err, api.BriefReasonWriteFailed)
		same()
	})
}

func TestInspectNeverFollowsALink(t *testing.T) {
	t.Run("CLAUDE.md", func(t *testing.T) {
		e := newEnv(t)
		out := filepath.Join(t.TempDir(), "id_ed25519")
		outsideFile(t, out, secret)
		symlink(t, out, e.path(contextwrite.FileName))
		e.src.set(inh(doc(idA, 6, "the brief")), nil)
		ins, err := e.m.Inspect(context.Background(), e.req())
		wantPlain(t, err)
		if strings.Contains(err.Error(), strings.TrimSpace(secret)) || len(ins.Diff) != 0 {
			t.Fatalf("Inspect read through the link: %v, %+v", err, ins)
		}
	})
	t.Run(".claude, the target", func(t *testing.T) {
		e := newEnv(t)
		e.git.tracked[contextwrite.FileName] = true
		home := t.TempDir()
		outsideFile(t, filepath.Join(home, "CLAUDE.md"), secret+block(idA, 6, "the brief"))
		symlink(t, home, e.path(".claude"))
		e.src.set(inh(doc(idA, 6, "the brief")), nil)
		_, err := e.m.Inspect(context.Background(), e.req())
		wantPlain(t, err)
	})
}

// A link swapped in between step 5's plan and step 7's write (the restart
// path materialises while the old agent still runs) is refused at the write:
// every write re-checks its path.
func TestALinkSwappedInAfterPlanningIsRefused(t *testing.T) {
	e := newEnv(t)
	e.write(contextwrite.AltFileName, "mine\n"+block(idB, 2, "older"))
	home := t.TempDir()
	same := outsideFile(t, filepath.Join(home, "CLAUDE.md"), "outside\n"+block(idB, 2, "older"))
	probe := filepath.Join(t.TempDir(), "probe")
	symlink(t, home, probe) // skips here, before the hook, where links are refused
	e.git.onExclude = func() {
		if err := os.RemoveAll(e.path(".claude")); err != nil {
			t.Error(err)
		}
		if err := os.Symlink(home, e.path(".claude")); err != nil {
			t.Error(err)
		}
	}
	e.src.set(inh(doc(idA, 6, "the brief")), nil)
	_, err := e.materialise(e.req())
	wantReason(t, err, api.BriefReasonWriteFailed)
	same()
	if got, _ := e.read(contextwrite.FileName); strings.Contains(got, claudemd.BeginPrefix) {
		t.Fatalf("CLAUDE.md holds a block after a refusal: %q", got)
	}
}

// Review m7 (mutant S1): the other candidate's strip is written before the
// target, so a strip that fails at write time leaves no new block behind: at
// most one managed block across the files, and the start refused. Here
// .claude becomes a file after the plan, so the strip cannot be written.
func TestTheOtherCandidatesStripIsWrittenBeforeTheTarget(t *testing.T) {
	e := newEnv(t)
	e.write(contextwrite.AltFileName, "mine\n"+block(idB, 2, "older"))
	e.git.onExclude = func() {
		if err := os.RemoveAll(e.path(".claude")); err != nil {
			t.Error(err)
		}
		if err := os.WriteFile(e.path(".claude"), []byte("a file now\n"), 0o644); err != nil {
			t.Error(err)
		}
	}
	e.src.set(inh(doc(idA, 6, "the brief")), nil)
	_, err := e.materialise(e.req())
	wantReason(t, err, api.BriefReasonWriteFailed)
	if got, ok := e.read(contextwrite.FileName); ok {
		t.Fatalf("CLAUDE.md written before the other candidate's strip: %q", got)
	}
}
