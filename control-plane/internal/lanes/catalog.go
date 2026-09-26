package lanes

import (
	"context"
	"fmt"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/gitx"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/project"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/worktree"
)

// Source is the phase 2 enumeration the catalog composes: *project.Scanner satisfies it.
type Source interface {
	Projects(ctx context.Context) ([]project.Project, error)
	Branches(ctx context.Context, projectID string) ([]project.Branch, error)
	Worktrees(ctx context.Context, projectID string) ([]project.Worktree, error)
	Path(ctx context.Context, worktreeID string) (string, error)
}

var _ Source = (*project.Scanner)(nil)

// Catalog is what the API serves as both api.Enumerator and worktree.Provider: the scanner's
// primary and linked worktrees, then this project's lane clones, each with Kind "clone",
// BranchID = project.BranchID(projectID, lane.Branch) and Name = lane.Name (never the
// directory's base name, which carries the creation nonce).
type Catalog struct {
	Source Source
	Store  Store
	// FS is the disk the Store's lanes are on (seam S8-P3.6). nil means this process's own:
	// Path runs worktree.Verify, exactly as before the seam. Phase 8 sets its machine's FS()
	// for a cloud machine's catalog (env task 25).
	FS gitx.MachineFS
}

var (
	_ worktree.Provider = (*Catalog)(nil)
)

func (c *Catalog) Projects(ctx context.Context) ([]project.Project, error) {
	return c.Source.Projects(ctx)
}

func (c *Catalog) Branches(ctx context.Context, projectID string) ([]project.Branch, error) {
	return c.Source.Branches(ctx, projectID)
}

func (c *Catalog) Worktrees(ctx context.Context, projectID string) ([]project.Worktree, error) {
	wts, err := c.Source.Worktrees(ctx, projectID)
	if err != nil {
		return nil, err
	}
	for _, l := range c.Store.ByProject(projectID) {
		wts = append(wts, project.Worktree{
			ID:        l.ID,
			ProjectID: l.ProjectID,
			BranchID:  project.BranchID(projectID, l.Branch),
			Name:      l.Name,
			Path:      l.Path,
			IsPrimary: false,
			Kind:      "clone",
		})
	}
	return wts, nil
}

// Path resolves a lane first (worktree.Verify on its Path, so a clone deleted outside the
// cockpit is worktree.ErrNotADir, the 404 session create already maps), then the Source. With
// FS set, a lane's check is FS.Exists(ctx, Path) instead: anything but (true, true, nil) is
// worktree.ErrNotADir, and an FS error is returned as it came.
func (c *Catalog) Path(ctx context.Context, worktreeID string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	l, ok := c.Store.ByID(worktreeID)
	if !ok {
		return c.Source.Path(ctx, worktreeID)
	}
	if c.FS == nil {
		return worktree.Verify(l.Path)
	}
	exists, dir, err := c.FS.Exists(ctx, l.Path)
	if err != nil {
		return "", err
	}
	if !exists || !dir {
		return "", fmt.Errorf("%w: %q", worktree.ErrNotADir, l.Path)
	}
	return l.Path, nil
}
