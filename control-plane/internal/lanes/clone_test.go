package lanes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/gitx/gitxtest"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/project"
)

// featRepo is a project on main with a branch feat one commit ahead.
func featRepo(t *testing.T) *gitxtest.Repo {
	t.Helper()
	p := gitxtest.NewRepo(t)
	p.Git("branch", "feat")
	p.Git("checkout", "-q", "feat")
	p.Commit("on feat", map[string]string{"feat.txt": "feat\n"})
	p.Git("checkout", "-q", "main")
	return p
}

func TestCreateClonesOnTheBranch(t *testing.T) {
	t.Parallel()
	p := featRepo(t)
	r := newRig(t, p)
	l := r.mustCreate("feat", "ui")
	c := r.lane(l)
	if got, want := c.Git("rev-parse", "HEAD"), p.Git("rev-parse", "refs/heads/feat"); got != want {
		t.Errorf("clone HEAD = %s, want the project's feat %s", got, want)
	}
	if got := c.Git("symbolic-ref", "HEAD"); got != "refs/heads/feat" {
		t.Errorf("clone symbolic-ref HEAD = %s, want refs/heads/feat", got)
	}
	if l.ID != project.WorktreeID(l.Path) {
		t.Errorf("Lane.ID = %s, want WorktreeID(Path) %s", l.ID, project.WorktreeID(l.Path))
	}
	if l.ProjectID != project.ProjectID(p.Dir) || l.Branch != "feat" || l.Name != "ui" || l.MachineID != "local" {
		t.Errorf("Lane = %+v", l)
	}
	if got, ok := r.store.ByID(l.ID); !ok || got != l {
		t.Errorf("store ByID = %+v, %v; want %+v", got, ok, l)
	}
}

func TestCreateWritesLineEndingConfig(t *testing.T) {
	t.Parallel()
	p := gitxtest.NewRepo(t)
	p.Commit("files", map[string]string{"a.txt": "x\ny\n", "b.bat": "x\ny\n", ".gitattributes": "*.bat eol=crlf\n"})
	p.UseSystemConfig(gitxtest.WindowsInstallerSystem)
	r := newRig(t, p)
	l := r.mustCreate("main", "le")
	c := r.lane(l)
	for k, want := range map[string]string{"core.autocrlf": "false", "core.eol": "lf", "core.checkStat": "minimal", "core.trustctime": "false"} {
		if got := c.Git("config", "--local", "--get", k); got != want {
			t.Errorf("clone config %s = %q, want %q", k, got, want)
		}
	}
	if got := readFile(t, filepath.Join(l.Path, "a.txt")); got != "x\ny\n" {
		t.Errorf("a.txt on disk = %q, want the blob's LF bytes", got)
	}
	if got := readFile(t, filepath.Join(l.Path, "b.bat")); got != "x\r\ny\r\n" {
		t.Errorf("b.bat on disk = %q, want CRLF (eol=crlf outranks core.autocrlf)", got)
	}
	if got := c.Git("status", "--porcelain"); got != "" {
		t.Errorf("status --porcelain = %q, want empty", got)
	}
}

func TestCreateSetsASelfDescribingOrigin(t *testing.T) {
	t.Parallel()
	p := featRepo(t)
	r := newRig(t, p)
	c := r.lane(r.mustCreate("feat", "o"))
	if got := c.Git("config", "--get", "remote.origin.url"); got != NoFetchURL {
		t.Errorf("remote.origin.url = %q, want NoFetchURL", got)
	}
	if got := c.Git("config", "--get", "remote.origin.pushurl"); got != NoPushURL {
		t.Errorf("remote.origin.pushurl = %q, want NoPushURL", got)
	}
	if got := c.Git("config", "--get", "branch.feat.remote"); got != "origin" {
		t.Errorf("branch.feat.remote = %q, want origin", got)
	}
	_, err := c.GitErr("fetch")
	if err == nil || !strings.Contains(err.Error(), NoFetchURL) {
		t.Errorf("git fetch in the clone: err = %v, want a failure naming NoFetchURL", err)
	}
}

func TestCreateSharesNoObjectFileWithTheProject(t *testing.T) {
	t.Parallel()
	p := featRepo(t)
	p.Git("gc", "-q") // a pack, an .idx and friends, as a real project has
	r := newRig(t, p)
	l := r.mustCreate("feat", "obj")
	objs := filepath.Join(l.Path, ".git", "objects")
	n := 0
	err := filepath.WalkDir(objs, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(objs, path)
		mine, err := os.Stat(path)
		if err != nil {
			return err
		}
		theirs, err := os.Stat(filepath.Join(p.Dir, ".git", "objects", rel))
		if err == nil && os.SameFile(mine, theirs) {
			t.Errorf("%s is the project's own file (a hard link)", rel)
		}
		n++
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if n == 0 {
		t.Fatal("the clone has no object files to compare")
	}
}

func TestCreateCopiesTheProjectsIdentity(t *testing.T) {
	t.Parallel()
	p := featRepo(t)
	p.Git("config", "user.name", "Ada")
	p.Git("config", "user.email", "ada@x.io")
	noIdentityEnv(p)
	r := newRig(t, p)
	c := r.lane(r.mustCreate("feat", "id"))
	if got := c.Git("var", "GIT_AUTHOR_IDENT"); !strings.HasPrefix(got, "Ada <ada@x.io>") {
		t.Errorf("clone GIT_AUTHOR_IDENT = %q, want Ada <ada@x.io> …", got)
	}
}

// The fixture sets user.useConfigOnly to model a machine with no identity to derive: host git
// otherwise invents one from the OS account (task 01 D1), and phase 2 treats that as the user's.
func TestCreateWithoutAnIdentityStillCreates(t *testing.T) {
	t.Parallel()
	p := featRepo(t)
	noIdentityEnv(p)
	p.Git("config", "--file", p.Env["GIT_CONFIG_GLOBAL"], "user.useConfigOnly", "true")
	if _, err := p.GitErr("var", "GIT_AUTHOR_IDENT"); err == nil {
		t.Fatal("precondition: git var found an identity")
	}
	r := newRig(t, p)
	l := r.mustCreate("feat", "noid")
	if out, err := r.lane(l).GitErr("config", "--local", "--get", "user.name"); err == nil {
		t.Errorf("clone has user.name %q, want none", out)
	}
}
