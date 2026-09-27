package lanes

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/gitx"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/gitx/gitxtest"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/project"
)

func TestCreateUnknownBranchTouchesNothing(t *testing.T) {
	t.Parallel()
	r := newRig(t, gitxtest.NewRepo(t))
	for _, b := range []string{"nope", "main~1", "main^{tree}", "a..b", "x:y"} {
		if _, err := r.create(b, "ui"); !errors.Is(err, ErrUnknownBranch) {
			t.Errorf("Create(%q): err = %v, want ErrUnknownBranch", b, err)
		}
	}
	ents, err := os.ReadDir(filepath.Join(r.root, project.ProjectID(r.proj.Dir)))
	if err == nil && len(ents) != 0 {
		t.Errorf("<root>/<p> holds %d entries after refused creates", len(ents))
	}
}

func TestCreateNeverReusesAnExistingDirectory(t *testing.T) {
	t.Parallel()
	r := newRig(t, gitxtest.NewRepo(t))
	pid := project.ProjectID(r.proj.Dir)
	taken := filepath.Join(r.root, pid, "ui-aaaaaa")
	writeFile(t, taken, "not ours\n")
	draws := 0
	r.c.Nonce = func() string { draws++; return "aaaaaa" }
	if _, err := r.create("main", "ui"); !errors.Is(err, ErrNameTaken) {
		t.Fatalf("Create over a taken directory: err = %v, want ErrNameTaken", err)
	}
	if draws != maxDraws || maxDraws != 4 {
		t.Errorf("draws = %d (maxDraws %d), want 4", draws, maxDraws)
	}
	if got := readFile(t, taken); got != "not ours\n" {
		t.Errorf("the existing file was changed: %q", got)
	}
	seq := []string{"aaaaaa", "bbbbbb"}
	r.c.Nonce = func() string { n := seq[0]; seq = seq[1:]; return n }
	l := r.mustCreate("main", "ui")
	if filepath.Base(l.Path) != "ui-bbbbbb" {
		t.Errorf("Path = %s, want …/ui-bbbbbb", l.Path)
	}
}

func TestCreateAgainAfterRemoveGivesANewID(t *testing.T) {
	t.Parallel()
	r := newRig(t, gitxtest.NewRepo(t))
	first := r.mustCreate("main", "ui")
	if err := r.store.Remove(first.ID); err != nil {
		t.Fatal(err)
	}
	if err := RemoveTree(r.root, first.Path); err != nil {
		t.Fatal(err)
	}
	second := r.mustCreate("main", "ui")
	if second.Name != "ui" || second.Path == first.Path || second.ID == first.ID {
		t.Errorf("second = %+v, first = %+v; want the same Name, a new Path and a new ID", second, first)
	}
	for _, l := range []Lane{first, second} {
		if !regexp.MustCompile(`^ui-[0-9a-f]{6}$`).MatchString(filepath.Base(l.Path)) {
			t.Errorf("directory %s is not ui-<6 hex>", filepath.Base(l.Path))
		}
	}
}

func TestCreateCleansUpAfterAFailedClone(t *testing.T) {
	t.Parallel()
	r := newRig(t, gitxtest.NewRepo(t))
	boom := errors.New("clone failed after it made the directory")
	rec := &recorder{inner: r.c.Machine.Git(), fail: func(c gitx.Call) error {
		if c.Args[0] == "clone" {
			return boom
		}
		return nil
	}}
	r.withRunner(rec, nil)
	r.c.Nonce = func() string { return "cccccc" }
	path := filepath.Join(r.root, project.ProjectID(r.proj.Dir), "ui-cccccc")
	if _, err := r.create("main", "ui"); !errors.Is(err, boom) {
		t.Fatalf("Create: err = %v, want the clone's failure", err)
	}
	if !cloneRan(rec) {
		t.Fatal("the clone never ran, so this proves nothing")
	}
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("%s survived a failed create: %v", path, err)
	}
	if n := len(r.store.List()); n != 0 {
		t.Errorf("the store holds %d lanes after a failed create", n)
	}
}

// TestCreateCleansUpWhenTheCallerCancels: the caller's cancel lands while the clone runs, and the
// clone fails after making Path. LocalFS refuses every call under an ended ctx, so the cleanup
// must run under one the cancel does not end (task 03 review m-8).
func TestCreateCleansUpWhenTheCallerCancels(t *testing.T) {
	t.Parallel()
	r := newRig(t, gitxtest.NewRepo(t))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	boom := errors.New("clone failed as the caller cancelled")
	rec := &recorder{inner: r.c.Machine.Git(), fail: func(c gitx.Call) error {
		if c.Args[0] == "clone" {
			cancel()
			return boom
		}
		return nil
	}}
	r.withRunner(rec, nil)
	r.c.Nonce = func() string { return "dddddd" }
	path := filepath.Join(r.root, project.ProjectID(r.proj.Dir), "ui-dddddd")
	if _, err := r.c.Create(ctx, CreateRequest{Project: projectOf(r.proj), Branch: "main", Name: "ui"}); !errors.Is(err, boom) {
		t.Fatalf("Create: err = %v, want the clone's failure", err)
	}
	if !cloneRan(rec) {
		t.Fatal("the clone never ran, so this proves nothing")
	}
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("%s survived a create its caller cancelled: %v", path, err)
	}
}

// envRunner snapshots each clone call's Env as it arrives, then writes into the map it was
// handed, as a careless Runner could.
type envRunner struct {
	inner gitx.Runner
	mu    sync.Mutex
	seen  []map[string]string
}

func (e *envRunner) Run(ctx context.Context, c gitx.Call) (gitx.Result, error) {
	if c.Args[0] == "clone" {
		snap := map[string]string{}
		for k, v := range c.Env {
			snap[k] = v
		}
		e.mu.Lock()
		e.seen = append(e.seen, snap)
		e.mu.Unlock()
		res, err := e.inner.Run(ctx, c)
		c.Env["GIT_CONFIG_COUNT"] = "0"
		delete(c.Env, "GIT_CONFIG_KEY_1")
		return res, err
	}
	return e.inner.Run(ctx, c)
}

// Not parallel: it asserts the package variable NoFilterEnv (task 03 review m-9).
func TestCreateHandsTheRunnerACopyOfNoFilterEnv(t *testing.T) {
	want := map[string]string{
		"GIT_CONFIG_COUNT": "3",
		"GIT_CONFIG_KEY_0": "filter.lfs.smudge", "GIT_CONFIG_VALUE_0": "",
		"GIT_CONFIG_KEY_1": "filter.lfs.process", "GIT_CONFIG_VALUE_1": "",
		"GIT_CONFIG_KEY_2": "filter.lfs.required", "GIT_CONFIG_VALUE_2": "false",
	}
	r := newRig(t, gitxtest.NewRepo(t))
	e := &envRunner{inner: r.c.Machine.Git()}
	r.withRunner(e, nil)
	r.mustCreate("main", "a")
	r.mustCreate("main", "b")
	if !reflect.DeepEqual(NoFilterEnv, want) {
		t.Errorf("NoFilterEnv after a Runner wrote into Call.Env = %v, want %v", NoFilterEnv, want)
	}
	if len(e.seen) != 2 || !reflect.DeepEqual(e.seen[1], want) {
		t.Errorf("clone Envs = %v; want two, the second %v", e.seen, want)
	}
}

func cloneRan(rec *recorder) bool {
	for _, c := range rec.calls {
		if c.call.Args[0] == "clone" {
			return true
		}
	}
	return false
}

func TestCreateRefusesBadNames(t *testing.T) {
	t.Parallel()
	r := newRig(t, gitxtest.NewRepo(t))
	for _, n := range []string{"../x", "A", "-x", strings.Repeat("a", 41), "a/b", "a\\b", "nul.x", "com1.txt"} {
		if _, err := r.create("main", n); !errors.Is(err, ErrBadName) {
			t.Errorf("Create(name %q): err = %v, want ErrBadName", n, err)
		}
	}
	l := r.mustCreate("main", "")
	if l.Name != "main-1" {
		t.Errorf("derived name = %q, want main-1", l.Name)
	}
	if _, err := r.create("main", strings.Repeat("a", 40)); err != nil {
		t.Errorf("a 40-character name: %v", err)
	}
}

func TestCreateDerivesTheLowestFreeName(t *testing.T) {
	t.Parallel()
	p := gitxtest.NewRepo(t)
	p.Git("branch", "feat/upload")
	r := newRig(t, p)
	pid := project.ProjectID(p.Dir)
	for _, n := range []string{"upload-1", "upload-3"} {
		if err := r.store.Add(Lane{ID: "w-" + n, ProjectID: pid, Branch: "feat/upload", Name: n, CreatedAt: time.Now().UTC()}); err != nil {
			t.Fatal(err)
		}
	}
	if l := r.mustCreate("feat/upload", ""); l.Name != "upload-2" {
		t.Errorf("derived name = %q, want upload-2", l.Name)
	}
	if got := deriveName("Feature/A Very Long Branch Name That Goes On", nil); got != "a-very-long-branch-name-1" {
		t.Errorf("deriveName = %q, want the slug cut at 24 and trimmed of '-'", got)
	}
}
