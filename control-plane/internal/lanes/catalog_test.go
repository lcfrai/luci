package lanes

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/project"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/worktree"
)

// fakeSource is phase 2's enumeration with no git: p has a primary and a linked worktree.
type fakeSource struct {
	t        *testing.T
	wts      map[string][]project.Worktree
	paths    map[string]string
	pathCall int
}

func (f *fakeSource) Projects(context.Context) ([]project.Project, error) { return nil, nil }
func (f *fakeSource) Branches(context.Context, string) ([]project.Branch, error) {
	return nil, nil
}
func (f *fakeSource) Worktrees(_ context.Context, projectID string) ([]project.Worktree, error) {
	return append([]project.Worktree(nil), f.wts[projectID]...), nil
}
func (f *fakeSource) Path(_ context.Context, id string) (string, error) {
	f.pathCall++
	if p, ok := f.paths[id]; ok {
		return p, nil
	}
	return "", worktree.ErrUnknownID
}

func catalogFixture(t *testing.T) (*Catalog, *fakeSource, []Lane) {
	t.Helper()
	src := &fakeSource{t: t, wts: map[string][]project.Worktree{
		"p-1": {
			{ID: "w-primary", ProjectID: "p-1", Name: "repo", Path: "C:\\repo", IsPrimary: true, Kind: "primary"},
			{ID: "w-linked", ProjectID: "p-1", Name: "wt", Path: "C:\\wt", Kind: "linked"},
		},
	}, paths: map[string]string{"w-primary": "C:\\repo"}}
	store := openStore(t, t.TempDir())
	at := time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)
	var ls []Lane
	for i, spec := range []struct{ p, b, n string }{{"p-1", "feat/a", "ui"}, {"p-1", "main", "api"}, {"p-2", "main", "other"}} {
		dir := filepath.Join(t.TempDir(), spec.n+"-abc123")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		l := Lane{ID: project.WorktreeID(dir), ProjectID: spec.p, Branch: spec.b, Name: spec.n, Path: dir,
			MachineID: "local", CreatedAt: at.Add(time.Duration(i) * time.Second)}
		if err := store.Add(l); err != nil {
			t.Fatal(err)
		}
		ls = append(ls, l)
	}
	return &Catalog{Source: src, Store: store}, src, ls
}

func TestCatalogListsClonesAfterTheScannersWorktrees(t *testing.T) {
	c, _, ls := catalogFixture(t)
	got, err := c.Worktrees(context.Background(), "p-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 || got[0].Kind != "primary" || got[1].Kind != "linked" {
		t.Fatalf("Worktrees(p-1) = %+v, want primary, linked, then 2 clones", got)
	}
	for i, l := range ls[:2] {
		w := got[2+i]
		want := project.Worktree{ID: l.ID, ProjectID: "p-1", BranchID: project.BranchID("p-1", l.Branch),
			Name: l.Name, Path: l.Path, IsPrimary: false, Kind: "clone"}
		if w != want {
			t.Errorf("clone %d = %+v, want %+v", i, w, want)
		}
		if w.Name == filepath.Base(l.Path) {
			t.Errorf("clone %d is named by its directory %q", i, w.Name)
		}
	}
}

func TestCatalogPathResolvesALaneFirst(t *testing.T) {
	c, src, ls := catalogFixture(t)
	got, err := c.Path(context.Background(), ls[0].ID)
	if err != nil || got != ls[0].Path {
		t.Errorf("Path(lane) = %q, %v; want %q", got, err, ls[0].Path)
	}
	if src.pathCall != 0 {
		t.Errorf("Source.Path ran %d times for a lane id", src.pathCall)
	}
	if got, err := c.Path(context.Background(), "w-primary"); err != nil || got != "C:\\repo" || src.pathCall != 1 {
		t.Errorf("Path(primary) = %q, %v (Source.Path calls %d)", got, err, src.pathCall)
	}
}

func TestCatalogPathOfAVanishedClone(t *testing.T) {
	c, _, ls := catalogFixture(t)
	if err := os.RemoveAll(ls[1].Path); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Path(context.Background(), ls[1].ID); !errors.Is(err, worktree.ErrNotADir) {
		t.Errorf("Path of a deleted clone: err = %v, want worktree.ErrNotADir", err)
	}
}

func TestCatalogPathUsesItsFS(t *testing.T) {
	c, _, ls := catalogFixture(t)
	gone := ls[1]
	if err := os.RemoveAll(gone.Path); err != nil {
		t.Fatal(err)
	}
	c.FS = answerFS{answers: map[string][2]bool{ls[0].Path: {true, false}, gone.Path: {true, true}}}
	if _, err := c.Path(context.Background(), ls[0].ID); !errors.Is(err, worktree.ErrNotADir) {
		t.Errorf("FS says not a dir: err = %v, want ErrNotADir", err)
	}
	if got, err := c.Path(context.Background(), gone.ID); err != nil || got != gone.Path {
		t.Errorf("FS says a dir (not on this disk): %q, %v; want its Path", got, err)
	}
	c.FS = nil
	if _, err := c.Path(context.Background(), gone.ID); !errors.Is(err, worktree.ErrNotADir) {
		t.Errorf("FS nil: err = %v, want the vanished-clone answer", err)
	}
}

// answerFS answers Exists from a table: [exists, dir].
type answerFS struct {
	nopFS
	answers map[string][2]bool
}

func (a answerFS) Exists(_ context.Context, p string) (bool, bool, error) {
	v := a.answers[p]
	return v[0], v[1], nil
}
