package lanes

import (
	"context"
	"errors"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/gitx"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/gitx/gitxtest"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/project"
)

// recFS wraps a MachineFS (LocalFS by default) and logs every call with the recorder's call
// count, so a test can place each FS call among the git calls. exists overrides Exists for a path.
type recFS struct {
	inner   gitx.MachineFS
	rec     *recorder
	mu      sync.Mutex
	log     []fsCall
	exists  map[string]bool
	sealErr error
}

type fsCall struct {
	op    string
	args  []string
	calls int // len(rec.calls) when it ran
}

func (f *recFS) note(op string, args ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	if f.rec != nil {
		f.rec.mu.Lock()
		n = len(f.rec.calls)
		f.rec.mu.Unlock()
	}
	f.log = append(f.log, fsCall{op: op, args: args, calls: n})
}

func (f *recFS) Exists(ctx context.Context, p string) (bool, bool, error) {
	f.note("Exists", p)
	if e, ok := f.exists[p]; ok {
		return e, e, nil
	}
	return f.inner.Exists(ctx, p)
}

func (f *recFS) RemoveTree(ctx context.Context, root, p string) error {
	f.note("RemoveTree", root, p)
	return f.inner.RemoveTree(ctx, root, p)
}

func (f *recFS) CopyGeneratedHooks(ctx context.Context, projectTop, cloneTop, rel string) (int, error) {
	f.note("CopyGeneratedHooks", projectTop, cloneTop, rel)
	return f.inner.CopyGeneratedHooks(ctx, projectTop, cloneTop, rel)
}

func (f *recFS) Seal(ctx context.Context, p string) error {
	f.note("Seal", p)
	if f.sealErr != nil {
		return f.sealErr
	}
	return f.inner.Seal(ctx, p)
}

func (f *recFS) ops() []fsCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]fsCall(nil), f.log...)
}

// recRig is a rig whose machine runs through a recorder and a recording FS over LocalFS.
func recRig(t *testing.T, p *gitxtest.Repo) (*rig, *recorder, *recFS) {
	r := newRig(t, p)
	rec := &recorder{inner: r.c.Machine.Git()}
	fs := &recFS{inner: LocalFS{}, rec: rec}
	r.withRunner(rec, fs)
	return r, rec, fs
}

func TestLocalFSExistsIsLstat(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	file := filepath.Join(dir, "f")
	writeFile(t, file, "x")
	link := filepath.Join(t.TempDir(), "link")
	dirLink(t, dir, link)
	for _, tc := range []struct {
		name, path  string
		exists, dir bool
	}{
		{"absent", filepath.Join(dir, "nope"), false, false},
		{"a directory", dir, true, true},
		{"a file", file, true, false},
		{"a link to a directory", link, true, false},
	} {
		e, d, err := LocalFS{}.Exists(ctx, tc.path)
		if err != nil || e != tc.exists || d != tc.dir {
			t.Errorf("%s: Exists = (%v, %v, %v), want (%v, %v, nil)", tc.name, e, d, err, tc.exists, tc.dir)
		}
	}
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, err := (LocalFS{}).Exists(cctx, dir); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled ctx: err = %v, want context.Canceled", err)
	}
	if err := (LocalFS{}).RemoveTree(cctx, dir, file); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled RemoveTree: err = %v", err)
	}
	if _, err := (LocalFS{}).CopyGeneratedHooks(cctx, dir, dir, "x"); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled CopyGeneratedHooks: err = %v", err)
	}
	if _, err := os.Stat(file); err != nil {
		t.Errorf("a cancelled RemoveTree removed %s", file)
	}
}

func TestCreateDrawsThroughTheMachineFS(t *testing.T) {
	t.Parallel()
	r, _, fs := recRig(t, gitxtest.NewRepo(t))
	first := filepath.Join(r.root, project.ProjectID(r.proj.Dir), "ui-aaaaaa")
	fs.exists = map[string]bool{first: true} // nothing is on disk there
	seq := []string{"aaaaaa", "bbbbbb"}
	r.c.Nonce = func() string { n := seq[0]; seq = seq[1:]; return n }
	l := r.mustCreate("main", "ui")
	if filepath.Base(l.Path) != "ui-bbbbbb" {
		t.Errorf("Path = %s, want the second draw ui-bbbbbb", l.Path)
	}
}

// addMark is a Store whose Add records the recorder's call count.
type addMark struct {
	*FileStore
	rec  *recorder
	mark int
}

func (s *addMark) Add(l Lane) error {
	s.rec.mu.Lock()
	s.mark = len(s.rec.calls)
	s.rec.mu.Unlock()
	return s.FileStore.Add(l)
}

func TestCreateSealsAfterTheLastLaneWrite(t *testing.T) {
	t.Parallel()
	r, rec, fs := recRig(t, huskyRepo(t))
	store := &addMark{FileStore: r.store, rec: rec, mark: -1}
	r.c.Store = store
	l := r.mustCreate("main", "seal")
	last := -1
	for i, c := range rec.calls {
		if c.call.NewLane == l.Path {
			last = i
		}
	}
	var seals []fsCall
	for _, op := range fs.ops() {
		if op.op == "Seal" {
			seals = append(seals, op)
		}
	}
	if len(seals) != 1 || seals[0].args[0] != l.Path {
		t.Fatalf("Seal calls = %+v, want one, with %s", seals, l.Path)
	}
	if last < 0 || seals[0].calls <= last || seals[0].calls > store.mark {
		t.Errorf("Seal ran after call %d; the last NewLane call is %d and Add came after call %d", seals[0].calls, last, store.mark)
	}

	r2, _, fs2 := recRig(t, huskyRepo(t))
	fs2.sealErr = errors.New("machined could not seal")
	r2.c.Nonce = func() string { return "dddddd" }
	if _, err := r2.create("main", "seal"); !errors.Is(err, fs2.sealErr) {
		t.Fatalf("Create with a failing Seal: err = %v", err)
	}
	ops := fs2.ops()
	path2 := filepath.Join(r2.root, project.ProjectID(r2.proj.Dir), "seal-dddddd")
	sealed := false
	for i, op := range ops {
		if op.op == "Seal" {
			sealed = true
			if i+1 >= len(ops) || ops[i+1].op != "RemoveTree" || ops[i+1].args[0] != r2.root || ops[i+1].args[1] != path2 {
				t.Errorf("after the failed Seal the FS saw %+v, want RemoveTree(%s, %s)", ops[i+1:], r2.root, path2)
			}
		}
	}
	if !sealed {
		t.Errorf("no Seal in %+v", ops)
	}
	if _, err := os.Lstat(path2); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("%s survived a failed Seal", path2)
	}
	if n := len(r2.store.List()); n != 0 {
		t.Errorf("the store holds %d lanes after a failed Seal", n)
	}
}

func TestCreateCleansUpThroughTheMachineFS(t *testing.T) {
	t.Parallel()
	r, rec, fs := recRig(t, gitxtest.NewRepo(t))
	boom := errors.New("clone failed")
	rec.fail = func(c gitx.Call) error {
		if c.Args[0] == "clone" {
			return boom
		}
		return nil
	}
	r.c.Nonce = func() string { return "eeeeee" }
	if _, err := r.create("main", "ui"); !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	want := filepath.Join(r.root, project.ProjectID(r.proj.Dir), "ui-eeeeee")
	ops := fs.ops()
	if n := len(ops); n == 0 || ops[n-1].op != "RemoveTree" || ops[n-1].args[0] != r.root || ops[n-1].args[1] != want {
		t.Errorf("FS log = %+v, want RemoveTree(%s, %s) last", ops, r.root, want)
	}
}

// cloudMachine is a machine that is not "local", with path.Join and its own FS.
type cloudMachine struct {
	git  gitx.Runner
	root string
	fs   gitx.MachineFS
}

func (m cloudMachine) ID() string                 { return "m-0123abcd" }
func (m cloudMachine) Git() gitx.Runner           { return m.git }
func (m cloudMachine) CloneRoot() string          { return m.root }
func (m cloudMachine) Join(elem ...string) string { return path.Join(elem...) }
func (m cloudMachine) FS() gitx.MachineFS         { return m.fs }

// nopFS is a remote disk where nothing exists and every write succeeds.
type nopFS struct{}

func (nopFS) Exists(context.Context, string) (bool, bool, error)                      { return false, false, nil }
func (nopFS) RemoveTree(context.Context, string, string) error                        { return nil }
func (nopFS) CopyGeneratedHooks(context.Context, string, string, string) (int, error) { return 0, nil }
func (nopFS) Seal(context.Context, string) error                                      { return nil }

func TestFSOfRefusesANilCloudFS(t *testing.T) {
	t.Parallel()
	script := gitxtest.NewScript(t)
	root := t.TempDir()
	m := cloudMachine{git: script, root: root}
	if _, err := FSOf(m); !errors.Is(err, ErrNoMachineFS) {
		t.Errorf("FSOf(cloud, nil FS) = %v, want ErrNoMachineFS", err)
	}
	c := &Creator{Machine: m, Store: openStore(t, t.TempDir())}
	p := project.Project{ID: "p-12345678", Name: "proj", Path: t.TempDir()}
	if _, err := c.Create(context.Background(), CreateRequest{Project: p, Branch: "main", Name: "ui"}); !errors.Is(err, ErrNoMachineFS) {
		t.Errorf("Create on a cloud machine with no FS: err = %v, want ErrNoMachineFS", err)
	}
	if len(script.Calls) != 0 {
		t.Errorf("git ran %d times", len(script.Calls))
	}
	if ents, _ := os.ReadDir(root); len(ents) != 0 {
		t.Errorf("CloneRoot holds %d entries", len(ents))
	}
	if fs, err := FSOf(gitx.Local{Root: root}); err != nil || fs != (LocalFS{}) {
		t.Errorf("FSOf(local, nil FS) = %v, %v; want LocalFS{}", fs, err)
	}
}

func TestCreateBuildsPathThroughMachineJoin(t *testing.T) {
	t.Parallel()
	p := project.Project{ID: "p-12345678", Name: "proj", Path: t.TempDir()}
	want := "/machine/lanes/p-12345678/ui-aaaaaa"
	script := gitxtest.NewScript(t).
		On("rev-parse --verify --quiet refs/heads/main^{commit}", gitx.Result{}, nil).
		On(argv(gitx.Call{Args: cloneArgv("main", p.Path, want)}), gitx.Result{}, nil).
		On("config remote.origin.url "+NoFetchURL, gitx.Result{}, nil).
		On("config remote.origin.pushurl "+NoPushURL, gitx.Result{}, nil).
		On("rev-parse --verify -q --symbolic-full-name main@{upstream}", gitx.Result{ExitCode: 1}, nil).
		On("var GIT_AUTHOR_IDENT", gitx.Result{ExitCode: 128}, nil).
		On("config --local --get core.hooksPath", gitx.Result{ExitCode: 1}, nil)
	fs := &recFS{inner: nopFS{}}
	c := &Creator{Machine: cloudMachine{git: script, root: "/machine/lanes", fs: fs}, Store: openStore(t, t.TempDir()),
		Nonce: func() string { return "aaaaaa" }, Now: func() time.Time { return time.Unix(0, 0) }}
	l, err := c.Create(context.Background(), CreateRequest{Project: p, Branch: "main", Name: "ui"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	ops := fs.ops()
	if len(ops) == 0 || ops[0].op != "Exists" || ops[0].args[0] != want {
		t.Errorf("first FS call = %+v, want Exists(%s) on %s", ops, want, runtime.GOOS)
	}
	if l.Path != want || l.MachineID != "m-0123abcd" {
		t.Errorf("Lane = %+v, want Path %s", l, want)
	}
}

func TestCopyHooksGoesThroughTheMachineFS(t *testing.T) {
	t.Parallel()
	find := func(ops []fsCall, op string) []fsCall {
		var out []fsCall
		for _, o := range ops {
			if o.op == op {
				out = append(out, o)
			}
		}
		return out
	}
	r, _, fs := recRig(t, huskyRepo(t))
	l := r.mustCreate("main", "husky")
	if got := find(fs.ops(), "CopyGeneratedHooks"); len(got) != 1 || got[0].args[0] != r.proj.Dir || got[0].args[1] != l.Path || got[0].args[2] != ".husky/_" {
		t.Errorf("CopyGeneratedHooks calls = %+v", got)
	}

	p := gitxtest.NewRepo(t)
	p.Git("config", "core.hooksPath", ".githooks")
	huskyFiles(t, filepath.Join(p.Dir, ".githooks"), 1, 4)
	r2, _, fs2 := recRig(t, p)
	l2 := r2.mustCreate("main", "gh")
	rm := find(fs2.ops(), "RemoveTree")
	if len(rm) != 1 || rm[0].args[0] != l2.Path || rm[0].args[1] != filepath.Join(l2.Path, ".githooks") {
		t.Errorf("take-back = %+v, want RemoveTree(%s, %s)", rm, l2.Path, filepath.Join(l2.Path, ".githooks"))
	}

	before := len(fs.ops())
	n, err := CopyGeneratedHooks(context.Background(), r.c.Machine, r.proj.Dir, l.Path, ".git/hooks")
	if n != 0 || err != nil {
		t.Errorf(".git/hooks = %d, %v", n, err)
	}
	if got := fs.ops()[before:]; len(got) != 0 {
		t.Errorf("a .git rel reached the FS: %+v", got)
	}
}

// existsFails is a MachineFS whose Exists cannot answer for one path, as a cloud machine's may
// not. The copy then cannot know which parents it would make, so which its take-back may remove.
type existsFails struct {
	gitx.MachineFS
	path string
}

func (e existsFails) Exists(ctx context.Context, p string) (bool, bool, error) {
	if p == e.path {
		return false, false, errors.New("existsFails: no answer")
	}
	return e.MachineFS.Exists(ctx, p)
}

// TestCopyHooksStopsWhenExistsCannotAnswer: Exists fails on a tracked parent of rel, so the copy
// stops with that error. Read as "absent", the failure names the parent as made, and the
// take-back of an unignored copy deletes the lane's tracked tools/ (fix round 1, mutant K13).
func TestCopyHooksStopsWhenExistsCannotAnswer(t *testing.T) {
	t.Parallel()
	p := gitxtest.NewRepo(t)
	p.Commit("tools", map[string]string{"tools/README": "tracked\n"})
	r := newRig(t, p)
	l := r.mustCreate("main", "tools")
	huskyFiles(t, filepath.Join(p.Dir, "tools", "hooks"), 1, 4)
	m := gitx.Local{Runner: r.c.Machine.Git(), Root: r.root,
		Files: existsFails{MachineFS: LocalFS{}, path: filepath.Join(l.Path, "tools")}}
	n, err := CopyGeneratedHooks(context.Background(), m, p.Dir, l.Path, "tools/hooks")
	if n != 0 || err == nil {
		t.Errorf("= %d, %v; want 0 and Exists' error", n, err)
	}
	if b, rerr := os.ReadFile(filepath.Join(l.Path, "tools", "README")); rerr != nil || string(b) != "tracked\n" {
		t.Errorf("tools/README = %q, %v; want the tracked file intact", b, rerr)
	}
	if _, lerr := os.Lstat(filepath.Join(l.Path, "tools", "hooks")); !errors.Is(lerr, os.ErrNotExist) {
		t.Errorf("tools/hooks after the refused copy: %v, want absent", lerr)
	}
}
