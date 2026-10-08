package api

import (
	"errors"

	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// A stopped task keeps its exact settled receipt until the container is gone:
// the reaper needs that receipt to retry a refused or interrupted removal. A
// positively identified serial run keeps its existing terminal cleanup policy.
// Missing private state must never turn a task into an ordinary session.
func (a *API) executionRequiresBackendAbsence(row registry.Session) (bool, error) {
	if row.ProjectRunID == "" && row.ProjectRunOperationID == "" {
		return false, nil
	}
	if a.deps.Product == nil || row.ProjectRunID == "" || row.ProjectRunOperationID == "" || row.ID == "" || row.ProjectID == "" {
		return true, errors.New("execution run ownership cannot be verified")
	}
	run, err := a.deps.Product.GetProjectSwarmRun(row.ProjectID, row.ProjectRunID)
	if err != nil {
		return true, err
	}
	if run.ProjectID != row.ProjectID || run.ID != row.ProjectRunID {
		return true, errors.New("execution run identity changed")
	}
	for _, op := range run.Operations {
		if op.ID != row.ProjectRunOperationID || op.Kind != "session" || op.ChildID != row.ID || op.RequestDigest != product.ProjectRunSessionRequestDigest(row) {
			continue
		}
		if op.TaskID == "" && row.WorktreeID == run.WorktreeID {
			return false, nil
		}
		if op.TaskID != "" && row.WorktreeID == op.WorktreeID && op.AbsenceRetirement == nil {
			return true, nil
		}
	}
	return true, errors.New("execution does not match its reserved operation")
}
