package lanes

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/gitx/gitxtest"
)

// spyMark is a Store whose Add records the Spy's length.
type spyMark struct {
	*FileStore
	spy  *[]gitxtest.Spawn
	mark int
}

func (s *spyMark) Add(l Lane) error {
	s.mark = len(*s.spy)
	return s.FileStore.Add(l)
}

func namesPath(s gitxtest.Spawn, lane string) bool {
	if strings.EqualFold(filepath.Clean(s.Dir), filepath.Clean(lane)) {
		return true
	}
	for _, a := range s.Argv[1:] {
		if strings.Contains(strings.ToLower(filepath.Clean(a)), strings.ToLower(filepath.Clean(lane))) {
			return true
		}
	}
	return false
}

// TestCreateIsTheOnlyHostGitInTheLane (ruling R4.5): every process host git starts in the lane,
// or naming it, comes from a Call marked NewLane == Path and runs before Store.Add, save step
// 4's clone (Dir the project, Path its destination positional). After Add, none names Path.
func TestCreateIsTheOnlyHostGitInTheLane(t *testing.T) {
	t.Parallel()
	p := huskyRepo(t)
	p.Git("config", "user.name", "Ada")
	p.Git("config", "user.email", "ada@x.io")
	r := newRig(t, p)
	spy := gitxtest.Spy(t, r.exec)
	rec := &recorder{inner: r.c.Machine.Git(), spy: spy}
	r.withRunner(rec, nil)
	store := &spyMark{FileStore: r.store, spy: spy, mark: -1}
	r.c.Store = store
	l := r.mustCreate("main", "only")

	spawns := *spy
	if store.mark < 0 || len(spawns) == 0 {
		t.Fatalf("Add mark %d, %d spawns", store.mark, len(spawns))
	}
	callOf := func(s int) int {
		c := -1
		for i, rc := range rec.calls {
			if rc.spyLen <= s {
				c = i
			}
		}
		return c
	}
	sawLane, sawClone := 0, false
	for s, sp := range spawns {
		if !namesPath(sp, l.Path) {
			continue
		}
		ci := callOf(s)
		if ci < 0 {
			t.Fatalf("spawn %d has no call", s)
		}
		call := rec.calls[ci].call
		if call.Args[0] == "clone" && filepath.Clean(call.Dir) == filepath.Clean(p.Dir) && call.Args[len(call.Args)-1] == l.Path && call.NewLane == "" {
			sawClone = true
			continue
		}
		sawLane++
		if call.NewLane != l.Path {
			t.Errorf("spawn %d (%s) names the lane from a call with NewLane %q", s, strings.Join(call.Args, " "), call.NewLane)
		}
		if s >= store.mark {
			t.Errorf("spawn %d (%s) names the lane after Store.Add (mark %d)", s, strings.Join(call.Args, " "), store.mark)
		}
	}
	if !sawClone || sawLane < 5 {
		t.Errorf("saw the clone %v and %d lane spawns; the fixture should give the clone and at least 5", sawClone, sawLane)
	}
}
