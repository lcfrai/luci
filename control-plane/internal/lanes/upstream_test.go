package lanes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/gitx/gitxtest"
)

// TestCreateGivesTheLaneTheProjectsUpstream (carried b05-2 issue 1): a bare remote, a teammate's
// push, the project fetched, then Create. `git clone` copies the project's refs/heads/* as
// origin/* and nothing of its refs/remotes/, so without step 4b the new lane has no
// upstream/<b> until the next refresh, and `git merge upstream/main` exits 1.
func TestCreateGivesTheLaneTheProjectsUpstream(t *testing.T) {
	t.Parallel()
	p := gitxtest.NewRepo(t)
	bare := &gitxtest.Repo{T: t, Dir: filepath.Join(t.TempDir(), "remote.git"), Env: p.Env}
	if err := os.MkdirAll(bare.Dir, 0o700); err != nil {
		t.Fatal(err)
	}
	bare.Git("init", "-q", "--bare", "-b", "main")
	p.Git("remote", "add", "origin", bare.Dir)
	p.Git("push", "-q", "-u", "origin", "main")
	mate := &gitxtest.Repo{T: t, Dir: filepath.Join(t.TempDir(), "mate"), Env: p.Env}
	(&gitxtest.Repo{T: t, Dir: filepath.Dir(mate.Dir), Env: p.Env}).Git("clone", "-q", bare.Dir, mate.Dir)
	mate.Commit("teammate", map[string]string{"mate.txt": "pushed\n"})
	mate.Git("push", "-q", "origin", "main")
	p.Git("fetch", "-q", "origin")
	want := p.Git("rev-parse", "refs/remotes/origin/main")
	if want == p.Git("rev-parse", "refs/heads/main") {
		t.Fatal("precondition: origin/main is not ahead of main")
	}

	r := newRig(t, p)
	c := r.lane(r.mustCreate("main", "up"))
	if got, err := c.GitErr("rev-parse", "--verify", "-q", "refs/remotes/upstream/main"); err != nil || got != want {
		t.Fatalf("clone upstream/main = %q, %v; want the project's origin/main %s", got, err, want)
	}
	if _, err := c.GitErr("merge", "--ff-only", "-q", "upstream/main"); err != nil {
		t.Errorf("git merge upstream/main in the new lane: %v", err)
	}
	if out, err := c.GitErr("config", "--get-regexp", `^remote\.`); err != nil || out == "" {
		t.Errorf("clone remotes = %q, %v", out, err)
	} else if strings.Contains(out, bare.Dir) || strings.Contains(out, filepath.ToSlash(bare.Dir)) || strings.Contains(out, "remote.git") {
		t.Errorf("the clone's config names the project's remote: %q", out)
	}
}

// No upstream configured: no upstream/<b>, and no error.
func TestCreateWithoutAnUpstreamHasNone(t *testing.T) {
	t.Parallel()
	r := newRig(t, featRepo(t))
	c := r.lane(r.mustCreate("feat", "noup"))
	if out, err := c.GitErr("for-each-ref", "refs/remotes/upstream/"); err != nil || out != "" {
		t.Errorf("refs/remotes/upstream/ = %q, %v; want none", out, err)
	}
}
