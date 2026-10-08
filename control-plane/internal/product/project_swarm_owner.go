package product

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

// ProjectSwarmWorktreeOwner reads private ownership independently of mutable
// clone files or task status. Retained, failed and integrated checkouts remain
// reserved to their original attempt; a manual session cannot bypass the run's
// concurrency, reviewed-input or integration controls by selecting that path.
func (s *Server) ProjectSwarmWorktreeOwner(ctx context.Context, projectID, worktreeID string) (runID, operationID string, err error) {
	if err := ctx.Err(); err != nil {
		return "", "", err
	}
	p, err := s.resolveProject(projectID)
	if err != nil {
		return "", "", err
	}
	runs, err := s.projectRuns.List(p.ID, p.Path)
	if err != nil {
		return "", "", err
	}
	for _, run := range runs {
		if run.Swarm == nil {
			continue
		}
		for _, op := range run.Operations {
			if op.TaskID == "" || op.WorktreeID != worktreeID {
				continue
			}
			path, pathErr := s.swarmPath(p.ID, run.ID, op.ID)
			if pathErr != nil || project.WorktreeID(path) != worktreeID || runID != "" {
				return "", "", errors.New("The retained task checkout has conflicting ownership.")
			}
			runID, operationID = run.ID, op.ID
		}
	}
	return runID, operationID, nil
}

// SetProjectSwarmWorktreeObserver installs the API's registry-backed exact
// checkout observer before serving requests. Independent task clones do not
// share primary Git metadata and must not block each other's admission.
func (s *Server) SetProjectSwarmWorktreeObserver(observer func(context.Context, string, string) (bool, error)) {
	s.projectSwarmWorktreeObserver = observer
}

// IsProjectSwarmPath identifies controller-owned storage, including retained
// integration worktrees. Only ledger-enumerated task clones may be exposed as
// session sources; a linked integration checkout never becomes a manual source.
func (s *Server) IsProjectSwarmPath(path string) bool {
	root := filepath.Join(s.projectRoot(), ".luci-swarms")
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
