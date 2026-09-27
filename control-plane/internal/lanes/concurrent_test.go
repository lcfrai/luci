package lanes

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/gitx"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/gitx/gitxtest"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/project"
)

// gate holds the first clone call, before it runs, until open is called, and closes arrived
// when that call reaches it. Every later clone runs at once. clones counts every clone call.
// It makes two creates in one project overlap deterministically: the first is mid-build, its
// name not yet in the index, while the second runs from start to end (task 03 fix round 1, I-1).
type gate struct {
	inner   gitx.Runner
	arrived chan struct{}
	release chan struct{}
	mu      sync.Mutex
	clones  int
}

func newGate(inner gitx.Runner) *gate {
	return &gate{inner: inner, arrived: make(chan struct{}), release: make(chan struct{})}
}

func (g *gate) Run(ctx context.Context, c gitx.Call) (gitx.Result, error) {
	if len(c.Args) > 0 && c.Args[0] == "clone" {
		g.mu.Lock()
		g.clones++
		first := g.clones == 1
		g.mu.Unlock()
		if first {
			close(g.arrived)
			select {
			case <-g.release:
			case <-ctx.Done():
				return gitx.Result{}, ctx.Err()
			case <-time.After(5 * time.Minute):
				return gitx.Result{}, errors.New("gate: never opened")
			}
		}
	}
	return g.inner.Run(ctx, c)
}

func (g *gate) open() { close(g.release) }

func (g *gate) count() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.clones
}

type created struct {
	l   Lane
	err error
}

// holdFirst starts a create of (branch, name) in a goroutine and returns once its clone is held
// at the gate. The caller runs the second create, then opens the gate and reads the first's result.
func holdFirst(t *testing.T, r *rig, g *gate, branch, name string) <-chan created {
	t.Helper()
	done := make(chan created, 1)
	go func() {
		l, err := r.c.Create(context.Background(), CreateRequest{Project: projectOf(r.proj), Branch: branch, Name: name})
		done <- created{l, err}
	}()
	select {
	case <-g.arrived:
	case res := <-done:
		t.Fatalf("the first create ended before its clone: %+v, %v", res.l, res.err)
	case <-time.After(5 * time.Minute):
		t.Fatal("the first create never reached its clone")
	}
	return done
}

func TestCreateConcurrentDerivedNamesAreDistinct(t *testing.T) {
	t.Parallel()
	r := newRig(t, gitxtest.NewRepo(t))
	g := newGate(r.c.Machine.Git())
	r.withRunner(g, nil)
	first := holdFirst(t, r, g, "main", "")
	second, err := r.create("main", "")
	g.open()
	a := <-first
	if err != nil || second.Name != "main-2" {
		t.Errorf("the second create, while main-1 was mid-build: %+v, %v; want main-2, nil", second, err)
	}
	if a.err != nil || a.l.Name != "main-1" {
		t.Errorf("the first create: %+v, %v; want main-1, nil", a.l, a.err)
	}
	if n := len(r.store.List()); n != 2 {
		t.Errorf("the store holds %d lanes, want 2", n)
	}
}

func TestCreateConcurrentSameNameClonesOnce(t *testing.T) {
	t.Parallel()
	r := newRig(t, gitxtest.NewRepo(t))
	g := newGate(r.c.Machine.Git())
	r.withRunner(g, nil)
	first := holdFirst(t, r, g, "main", "ui")
	_, err := r.create("main", "ui")
	clones := g.count()
	g.open()
	a := <-first
	if !errors.Is(err, ErrNameTaken) {
		t.Errorf("the second create of ui, while ui was mid-build: err = %v, want ErrNameTaken", err)
	}
	if clones != 1 {
		t.Errorf("%d clones ran by the time the second create returned, want 1", clones)
	}
	if a.err != nil || a.l.Name != "ui" {
		t.Errorf("the first create: %+v, %v; want ui, nil", a.l, a.err)
	}
	ents, rerr := os.ReadDir(filepath.Join(r.root, project.ProjectID(r.proj.Dir)))
	if rerr != nil || len(ents) != 1 || len(r.store.List()) != 1 {
		t.Errorf("after both: %d directories (%v), %d lanes; want 1 and 1", len(ents), rerr, len(r.store.List()))
	}
}

// otherWriter is a second writer on the same index: its Add first files a lane of the same
// project and name under another ID, as a second Creator over this Store would.
type otherWriter struct{ *FileStore }

func (s otherWriter) Add(l Lane) error {
	o := l
	o.ID, o.Path = "w-00000000", l.Path+"-other"
	if err := s.FileStore.Add(o); err != nil {
		return err
	}
	return s.FileStore.Add(l)
}

func TestCreateMapsAnIndexNameClashToErrNameTaken(t *testing.T) {
	t.Parallel()
	r := newRig(t, gitxtest.NewRepo(t))
	r.c.Store = otherWriter{r.store}
	r.c.Nonce = func() string { return "eeeeee" }
	_, err := r.create("main", "ui")
	if !errors.Is(err, ErrNameTaken) {
		t.Errorf("Store.Add's ProjectID+Name clash: err = %v, want ErrNameTaken", err)
	}
	path := filepath.Join(r.root, project.ProjectID(r.proj.Dir), "ui-eeeeee")
	if _, lerr := os.Lstat(path); !errors.Is(lerr, os.ErrNotExist) {
		t.Errorf("%s survived the refused Add: %v", path, lerr)
	}
}
