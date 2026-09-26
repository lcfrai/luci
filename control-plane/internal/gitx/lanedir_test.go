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

// TestMachineHardeningReadsSystemThenGlobal: the part has no row for MachineHardening (task 02
// addition). Helpers come from --system then --global in file order; the global proxy wins;
// with neither file naming one, HTTPS_PROXY is the fallback.
func TestMachineHardeningReadsSystemThenGlobal(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	t.Setenv("GIT_CONFIG_NOSYSTEM", "")
	t.Setenv("GIT_CONFIG_SYSTEM", write("system", "[credential]\n\thelper = sys\n[http]\n\tproxy = http://sys.invalid:1\n"))
	t.Setenv("GIT_CONFIG_GLOBAL", write("global", "[credential \"https://x.invalid\"]\n\thelper = glob\n[http]\n\tproxy = http://glob.invalid:2\n"))
	t.Setenv("HTTPS_PROXY", "http://env.invalid:3")
	root := filepath.Join(t.TempDir(), "git")
	h, err := gitx.MachineHardening(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"credential.helper=sys", "credential.https://x.invalid.helper=glob"}
	if strings.Join(h.Credential, "|") != strings.Join(want, "|") || h.Proxy != "http://glob.invalid:2" {
		t.Fatalf("Hardening = Credential %q Proxy %q, want %q and the global proxy", h.Credential, h.Proxy, want)
	}
	if ents, err := os.ReadDir(h.HooksDir); err != nil || len(ents) != 0 || h.OwnedRoot != root || h.LaneRoot != "" {
		t.Fatalf("HooksDir %q (%v, %d entries), OwnedRoot %q, LaneRoot %q", h.HooksDir, err, len(ents), h.OwnedRoot, h.LaneRoot)
	}
	t.Setenv("GIT_CONFIG_SYSTEM", write("system2", ""))
	t.Setenv("GIT_CONFIG_GLOBAL", write("global2", ""))
	if h, err = gitx.MachineHardening(context.Background(), root); err != nil || h.Proxy != "http://env.invalid:3" || len(h.Credential) != 0 {
		t.Fatalf("no proxy in either file = (%q, %q, %v), want HTTPS_PROXY and no helpers", h.Proxy, h.Credential, err)
	}
	// Fix round 1 (review M-5). (a) A named file does not follow [include] unless asked, and
	// H1's reset would then clear the included helper on every Remote call. (b) An empty
	// http.proxy is git's "no proxy at all": it beats the system proxy and the environment.
	inc := write("inc", "[credential]\n\thelper = fromInclude\n")
	t.Setenv("GIT_CONFIG_SYSTEM", write("system3", "[http]\n\tproxy = http://sys.invalid:1\n"))
	t.Setenv("GIT_CONFIG_GLOBAL", write("global3", "[include]\n\tpath = "+filepath.ToSlash(inc)+"\n[http]\n\tproxy =\n"))
	if h, err = gitx.MachineHardening(context.Background(), root); err != nil || h.Proxy != "" ||
		strings.Join(h.Credential, "|") != "credential.helper=fromInclude" {
		t.Fatalf("include + empty global proxy = (%q, %q, %v), want no proxy and the included helper", h.Proxy, h.Credential, err)
	}
	// (c) lowercase all_proxy. Windows' environment is case-insensitive, so this row cannot go
	// red here; it can on Linux.
	t.Setenv("GIT_CONFIG_SYSTEM", write("system4", ""))
	t.Setenv("GIT_CONFIG_GLOBAL", write("global4", ""))
	for _, k := range []string{"HTTPS_PROXY", "https_proxy", "ALL_PROXY"} {
		t.Setenv(k, "")
	}
	t.Setenv("all_proxy", "http://all.invalid:4")
	if h, err = gitx.MachineHardening(context.Background(), root); err != nil || h.Proxy != "http://all.invalid:4" {
		t.Fatalf("only all_proxy = (%q, %v), want it", h.Proxy, err)
	}
}

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
	// A spelling that does not start with LaneRoot's own string, so only a cleaned compare
	// sees it is in L (a raw, case-folded prefix check passes every row above).
	sep := string(filepath.Separator)
	rows["Dir <parent>/x/../<root>/L"] = project(filepath.Dir(root)+sep+"x"+sep+".."+sep+filepath.Base(root)+sep+filepath.Base(l), gitx.Read, "status")
	if runtime.GOOS == "windows" {
		rows["Dir L in other case"] = project(strings.ToUpper(l), gitx.Read, "status")
		// Fix round 1 (review M-6): Win32 drops a segment's single trailing dot, so <root>.
		// names LaneRoot; only a compare of locations, not spellings, sees it.
		dotted := root + "." + sep + filepath.Base(l)
		rows["Dir <root>./L"] = project(dotted, gitx.Read, "status")
		rows["-C <root>./L"] = project(r.Dir, gitx.Read, "status", "-C", dotted)
	}
	// An 8.3 short name and its long name are one location (this machine's TEMP is AU-DEV~1).
	if long, err := filepath.EvalSymlinks(l); err == nil && !strings.EqualFold(long, l) {
		rows["Dir L in its other spelling"] = project(long, gitx.Read, "status")
		rows["--work-tree L in its other spelling"] = project(r.Dir, gitx.Read, "status", "--work-tree", long)
	} else {
		t.Logf("no second spelling of %s here (EvalSymlinks %q, %v): that row runs where TEMP is an 8.3 name", l, long, err)
	}
	// A link in an ancestor. Windows refuses os.Symlink without the privilege, so the row may
	// not run here; a junction needs a program start, which this package's pin forbids.
	link := filepath.Join(t.TempDir(), "j")
	if err := os.Symlink(root, link); err == nil {
		rows["Dir <link to root>/L"] = project(filepath.Join(link, filepath.Base(l)), gitx.Read, "status")
	} else {
		t.Logf("no symlink here (%v): the ancestor-link row runs where one can be made", err)
	}
	// A link to the lane itself, not to the root: no ancestor of the spelled path is LaneRoot's
	// directory, so only the EvalSymlinks form shows it lies under the root (fix round 1: the
	// M6-noEvalSymlinks mutant survived every row above).
	linkL := filepath.Join(t.TempDir(), "k")
	if err := os.Symlink(l, linkL); err == nil {
		rows["Dir <link to L>"] = project(linkL, gitx.Read, "status")
		rows["--git-dir <link to L>/.git"] = project(r.Dir, gitx.Read, "status", "--git-dir="+filepath.Join(linkL, ".git"))
	} else {
		t.Logf("no symlink here (%v): the lane-link row runs where one can be made", err)
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
	outside := t.TempDir()
	root := x.Hardening.LaneRoot
	for name, tc := range map[string]struct {
		c    gitx.Call
		want error
	}{
		"NewLane L, Dir L2":                 {gitx.Call{Dir: other, NewLane: l, Args: []string{"status"}, Scope: gitx.None}, gitx.ErrLaneDir},
		"NewLane L, Scope Project, project": {gitx.Call{Dir: r.Dir, NewLane: l, Args: []string{"status"}, Scope: gitx.Project}, gitx.ErrCallShape},
		"NewLane L, Scope Project, Dir L":   {gitx.Call{Dir: l, NewLane: l, Args: []string{"status"}, Scope: gitx.Project}, gitx.ErrLaneDir},
		// Fix round 1 (review M-7): NewLane names a clone lanes.Create makes UNDER LaneRoot.
		"NewLane outside LaneRoot": {gitx.Call{Dir: outside, NewLane: outside, Args: []string{"status"}, Scope: gitx.None}, gitx.ErrCallShape},
		"NewLane is LaneRoot":      {gitx.Call{Dir: root, NewLane: root, Args: []string{"status"}, Scope: gitx.None}, gitx.ErrCallShape},
	} {
		if _, err := run.Run(context.Background(), tc.c); !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", name, err, tc.want)
		}
	}
	if len(*spawns) != 0 {
		t.Fatalf("git started %d times for refused calls", len(*spawns))
	}
	// With no LaneRoot there is no lane to create, so NewLane opens nothing.
	bare := gitxtest.NewExec(t)
	if _, err := r.RunnerOf(bare).Run(context.Background(), c); !errors.Is(err, gitx.ErrCallShape) {
		t.Fatalf("NewLane with LaneRoot \"\": err = %v, want ErrCallShape", err)
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
	c.Args = append([]string(nil), c.Args...)
	c.Args[len(c.Args)-1] = filepath.Join(x.Hardening.LaneRoot, "new2")
	c.NewLane = c.Args[len(c.Args)-1]
	if _, err := run.Run(context.Background(), c); !errors.Is(err, gitx.ErrCallShape) {
		t.Fatalf("the clone marked NewLane: err = %v, want ErrCallShape", err)
	}
}
