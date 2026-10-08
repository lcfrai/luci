package api

import (
	"context"
	"errors"

	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

var errTaskHandoffBeforePurge = errors.New("Before forgetting this task, use Check team status or open Task result and capture its report. Its session and reviewed context have been kept. A recorded missing or invalid report is also retained explicitly.")

// Forget removes the session's captured context and ownership record. Keep
// those prerequisites until the exact task has a durable canonical handoff,
// including an explicit missing/invalid/unavailable receipt. This is a bounded
// stored read only: collection takes the same child guard and must never run
// under the delete handler's lifecycle lock.
func (a *API) requireTaskHandoffBeforePurge(ctx context.Context, row registry.Session) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if row.ProjectRunID == "" && row.ProjectRunOperationID == "" {
		return nil
	}
	if a.deps.Product == nil || row.ProjectRunID == "" || row.ProjectRunOperationID == "" {
		return errTaskHandoffBeforePurge
	}
	run, err := a.deps.Product.GetProjectSwarmRun(row.ProjectID, row.ProjectRunID)
	if err != nil || run.ProjectID != row.ProjectID || run.ID != row.ProjectRunID {
		return errTaskHandoffBeforePurge
	}
	for _, op := range run.Operations {
		if op.ID != row.ProjectRunOperationID {
			continue
		}
		if op.Kind != "session" || op.ChildID != row.ID || op.RequestDigest != product.ProjectRunSessionRequestDigest(row) {
			return errTaskHandoffBeforePurge
		}
		if op.TaskID == "" {
			if run.WorktreeID != row.WorktreeID {
				return errTaskHandoffBeforePurge
			}
			return nil // Positively identified serial runs keep ordinary Forget.
		}
		if op.WorktreeID != row.WorktreeID || op.Handoff == nil {
			return errTaskHandoffBeforePurge
		}
		view, err := a.deps.Product.TaskHandoffForOperation(row.ProjectID, row.ProjectRunID, op.ID)
		if err != nil || view.Receipt == nil || view.Digest != op.Handoff.Digest || view.Receipt.ChildID != row.ID {
			return errTaskHandoffBeforePurge
		}
		switch view.Status {
		case "captured", "missing", "invalid", "unavailable":
			return ctx.Err()
		default:
			return errTaskHandoffBeforePurge
		}
	}
	return errTaskHandoffBeforePurge
}
