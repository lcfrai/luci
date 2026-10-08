package api

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/lcfrai/luci/control-plane/internal/project"
	"github.com/lcfrai/luci/control-plane/internal/worktree"
)

// Task checkouts are independent repositories owned by the private run ledger,
// not additional projects or Git-linked worktrees. One decorator supplies both
// catalogue ownership and session path resolution; the management gateway uses
// this same catalogue when applying the original project's grants.
type swarmWorktrees struct {
	base     Enumerator
	resolver worktree.Provider
	owned    func(context.Context, string) ([]project.Worktree, error)
	private  func(string) bool
}

func (p *swarmWorktrees) Projects(ctx context.Context) ([]project.Project, error) {
	return p.base.Projects(ctx)
}

func (p *swarmWorktrees) ownedWorktrees(ctx context.Context, projectID string) ([]project.Worktree, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	rows, err := p.owned(ctx, projectID)
	if err != nil {
		return nil, err
	}
	result := make([]project.Worktree, 0, len(rows))
	seen := map[string]bool{}
	for _, row := range rows {
		if row.ProjectID != projectID || row.IsPrimary || !filepath.IsAbs(row.Path) || filepath.Clean(row.Path) != row.Path || row.ID != project.WorktreeID(row.Path) || seen[row.ID] {
			return nil, errors.New("The task checkout does not have a unique owned project identity.")
		}
		seen[row.ID] = true
		// A detached task checkout still needs a stable selection group in the
		// rail. This identifies the task, never a movable primary Git branch.
		if row.BranchID == "" {
			row.BranchID = project.BranchID(projectID, "luci-task/"+row.ID)
		}
		result = append(result, row)
	}
	return result, nil
}

func (p *swarmWorktrees) Worktrees(ctx context.Context, projectID string) ([]project.Worktree, error) {
	base, err := p.base.Worktrees(ctx, projectID)
	if err != nil {
		return nil, err
	}
	owned, err := p.ownedWorktrees(ctx, projectID)
	if err != nil {
		return nil, err
	}
	result := make([]project.Worktree, 0, len(base)+len(owned))
	seen := map[string]bool{}
	for _, row := range base {
		if p.private != nil && p.private(row.Path) {
			continue
		}
		seen[row.ID] = true
		result = append(result, row)
	}
	for _, row := range owned {
		if seen[row.ID] {
			return nil, errors.New("A task checkout conflicts with an existing worktree identity.")
		}
		seen[row.ID] = true
		result = append(result, row)
	}
	return result, nil
}

func (p *swarmWorktrees) Branches(ctx context.Context, projectID string) ([]project.Branch, error) {
	base, err := p.base.Branches(ctx, projectID)
	if err != nil {
		return nil, err
	}
	owned, err := p.ownedWorktrees(ctx, projectID)
	if err != nil {
		return nil, err
	}
	result := append([]project.Branch{}, base...)
	seen := map[string]bool{}
	for _, row := range base {
		seen[row.ID] = true
	}
	for _, row := range owned {
		if seen[row.BranchID] {
			return nil, errors.New("A task checkout conflicts with an existing branch identity.")
		}
		seen[row.BranchID] = true
		result = append(result, project.Branch{ID: row.BranchID, ProjectID: projectID, Name: row.Name})
	}
	return result, nil
}

func (p *swarmWorktrees) Path(ctx context.Context, id string) (string, error) {
	path, err := p.resolver.Path(ctx, id)
	if err == nil {
		if p.private == nil || !p.private(path) {
			return path, nil
		}
		// Do not trust a Git-linked worktree merely because the base scanner
		// knows it. Private controller paths require explicit ledger ownership.
	} else if !errors.Is(err, worktree.ErrUnknownID) {
		return path, err
	}
	projects, err := p.base.Projects(ctx)
	if err != nil {
		return "", err
	}
	var found string
	for _, owner := range projects {
		rows, err := p.ownedWorktrees(ctx, owner.ID)
		if err != nil {
			return "", err
		}
		for _, row := range rows {
			if row.ID != id {
				continue
			}
			if found != "" {
				return "", errors.New("The task checkout has conflicting project owners.")
			}
			found = row.Path
		}
	}
	if found == "" {
		return "", fmt.Errorf("%w: %q", worktree.ErrUnknownID, id)
	}
	return worktree.Verify(found)
}
