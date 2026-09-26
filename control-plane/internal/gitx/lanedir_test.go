package gitx_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/gitx"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/gitx/gitxtest"
)

// laneSetup is a project, a lane clone L of it, and an Exec whose LaneRoot holds L, spied on.
func laneSetup(t *testing.T) (r *gitxtest.Repo, lane string, x *gitx.Exec, run gitx.Runner, spawns *[]gitxtest.Spawn) {
	t.Helper()
	r = gitxtest.NewRepo(t)
	l := gitxtest.CloneOf(t, r, "main")
	x = gitxtest.NewExec(t)
	x.Hardening.LaneRoot = filepath.Dir(l.Dir)
	return r, l.Dir, x, r.RunnerOf(x), gitxtest.Spy(t, x)
}

func TestExecRefusesALaneDir(t *testing.T) {
	r, l, x, run, spawns := laneSetup(t)
	root := x.Hardening.LaneRoot
	rows := map[string]gitx.Call{
		"Dir L":            project(l, gitx.Read, "status"),
		"Dir L/sub":        project(filepath.Join(l, "sub"), gitx.Read, "status"),
		"-C L":             project(r.Dir, gitx.Read, "status", "-C", l),
		"--git-dir=L/.git": project(r.Dir, gitx.Read, "status", "--git-dir="+filepath.Join(l, ".git")),
		"--git-dir L/.git": project(r.Dir, gitx.Read, "status", "--git-dir", filepath.Join(l, ".git")),
		"--work-tree L":    project(r.Dir, gitx.Read, "status", "--work-tree", l),
		"Dir x/../L":       project(root+string(filepath.Separator)+"x"+string(filepath.Separator)+".."+string(filepath.Separator)+filepath.Base(l), gitx.Read, "status"),
	}
	if runtime.GOOS == "windows" {
		rows["Dir L in other case"] = project(strings.ToUpper(l), gitx.Read, "status")
	}
	for _, k := range []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_OBJECT_DIRECTORY"} {
		c := project(r.Dir, gitx.Read, "status")
		c.Env = map[string]string{k: filepath.Join(l, ".git")}
		rows["Env "+k] = c
	}
	c := project(r.Dir, gitx.Read, "status")
	c.Env = map[string]string{"GIT_ALTERNATE_OBJECT_DIRECTORIES": "x" + string(os.PathListSeparator) + filepath.Join(l, ".git", "objects")}
	rows["Env GIT_ALTERNATE_OBJECT_DIRECTORIES"] = c
	for name, c := range rows {
		if _, err := run.Run(context.Background(), c); !errors.Is(err, gitx.ErrLaneDir) {
			t.Errorf("%s: err = %v, want ErrLaneDir", name, err)
		}
	}
	if len(*spawns) != 0 {
		t.Fatalf("git started %d times for lane calls: not even H3's config read may run in L", len(*spawns))
	}
}

func TestExecAdmitsTheNewLaneOnly(t *testing.T) {
	r, l, x, run, spawns := laneSetup(t)
	c := gitx.Call{Dir: l, NewLane: l, Args: []string{"rev-parse", "HEAD"}, Class: gitx.Read, Scope: gitx.None}
	runOK(t, run, c)
	*spawns = (*spawns)[:0]
	other := filepath.Join(x.Hardening.LaneRoot, "other")
	for name, tc := range map[string]struct {
		c    gitx.Call
		want error
	}{
		"NewLane L, Dir L2":                 {gitx.Call{Dir: other, NewLane: l, Args: []string{"status"}, Scope: gitx.None}, gitx.ErrLaneDir},
		"NewLane L, Scope Project, project": {gitx.Call{Dir: r.Dir, NewLane: l, Args: []string{"status"}, Scope: gitx.Project}, gitx.ErrCallShape},
		"NewLane L, Scope Project, Dir L":   {gitx.Call{Dir: l, NewLane: l, Args: []string{"status"}, Scope: gitx.Project}, gitx.ErrLaneDir},
	} {
		if _, err := run.Run(context.Background(), tc.c); !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", name, err, tc.want)
		}
	}
	if len(*spawns) != 0 {
		t.Fatalf("git started %d times for refused calls", len(*spawns))
	}
}

func TestExecRunsTheCreateCloneInTheProject(t *testing.T) {
	r, _, x, run, _ := laneSetup(t)
	l := filepath.Join(x.Hardening.LaneRoot, "new")
	c := project(r.Dir, gitx.LocalWrite, "clone", "-q", "--no-hardlinks", "--", r.Dir, l)
	runOK(t, run, c)
	if fi, err := os.Stat(filepath.Join(l, ".git")); err != nil || !fi.IsDir() {
		t.Fatalf("%s is not a clone: %v", l, err)
	}
	c.Args[len(c.Args)-1] = filepath.Join(x.Hardening.LaneRoot, "new2")
	c.NewLane = c.Args[len(c.Args)-1]
	if _, err := run.Run(context.Background(), c); !errors.Is(err, gitx.ErrCallShape) {
		t.Fatalf("the clone marked NewLane: err = %v, want ErrCallShape", err)
	}
}
