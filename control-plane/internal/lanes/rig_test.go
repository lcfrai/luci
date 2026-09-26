package lanes

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/gitx"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/gitx/gitxtest"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/project"
)

// rig is one project, a CloneRoot, a real gitx.Exec whose Hardening.LaneRoot is that root (so
// every lane call is checked the way production checks it), and a Creator over a FileStore.
type rig struct {
	t     *testing.T
	proj  *gitxtest.Repo
	root  string
	exec  *gitx.Exec
	store *FileStore
	c     *Creator
}

func newRig(t *testing.T, proj *gitxtest.Repo) *rig {
	t.Helper()
	root := filepath.Join(t.TempDir(), "worktrees")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	x := gitxtest.NewExec(t)
	x.Hardening.LaneRoot = root
	r := &rig{t: t, proj: proj, root: root, exec: x, store: openStore(t, t.TempDir())}
	r.c = &Creator{Machine: gitx.Local{Runner: proj.RunnerOf(x), Root: root}, Store: r.store}
	return r
}

// withRunner swaps the machine's Runner (a wrapper around the rig's own) and its FS.
func (r *rig) withRunner(run gitx.Runner, fs gitx.MachineFS) {
	r.c.Machine = gitx.Local{Runner: run, Root: r.root, Files: fs}
}

func projectOf(repo *gitxtest.Repo) project.Project {
	return project.Project{ID: project.ProjectID(repo.Dir), Name: filepath.Base(repo.Dir), Path: repo.Dir}
}

func (r *rig) create(branch, name string) (Lane, error) {
	r.t.Helper()
	return r.c.Create(context.Background(), CreateRequest{Project: projectOf(r.proj), Branch: branch, Name: name})
}

func (r *rig) mustCreate(branch, name string) Lane {
	r.t.Helper()
	l, err := r.create(branch, name)
	if err != nil {
		r.t.Fatalf("Create(%s, %q): %v", branch, name, err)
	}
	return l
}

// lane is the clone as a raw-git Repo under the project's environment, for assertions only.
func (r *rig) lane(l Lane) *gitxtest.Repo {
	return &gitxtest.Repo{T: r.t, Dir: l.Path, Env: r.proj.Env}
}

// noIdentityEnv drops the fixed identity gitxtest sets, so `git var` answers from config.
func noIdentityEnv(repo *gitxtest.Repo) {
	for _, k := range []string{"GIT_AUTHOR_NAME", "GIT_AUTHOR_EMAIL", "GIT_COMMITTER_NAME", "GIT_COMMITTER_EMAIL"} {
		delete(repo.Env, k)
	}
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	return string(b)
}

func writeFile(t *testing.T, p, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// dirLink makes link a directory link to target: a junction on Windows (no privilege needed),
// a symlink elsewhere.
func dirLink(t *testing.T, target, link string) {
	t.Helper()
	if runtime.GOOS != "windows" {
		if err := os.Symlink(target, link); err != nil {
			t.Fatalf("Symlink: %v", err)
		}
		return
	}
	out, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput()
	if err != nil {
		t.Fatalf("mklink /J %s %s: %v: %s", link, target, err, out)
	}
}

// recorder wraps a Runner and records each Call with the Spy's length before it ran.
type recorder struct {
	inner gitx.Runner
	spy   *[]gitxtest.Spawn
	mu    sync.Mutex
	calls []recorded
	// fail, when set, is asked after the call ran; a non-nil answer replaces the call's error.
	fail func(c gitx.Call) error
}

type recorded struct {
	call   gitx.Call
	spyLen int
}

func (w *recorder) Run(ctx context.Context, c gitx.Call) (gitx.Result, error) {
	w.mu.Lock()
	n := 0
	if w.spy != nil {
		n = len(*w.spy)
	}
	w.calls = append(w.calls, recorded{call: c, spyLen: n})
	w.mu.Unlock()
	res, err := w.inner.Run(ctx, c)
	if w.fail != nil {
		if ferr := w.fail(c); ferr != nil {
			return gitx.Result{}, ferr
		}
	}
	return res, err
}

func argv(c gitx.Call) string { return strings.Join(c.Args, " ") }
