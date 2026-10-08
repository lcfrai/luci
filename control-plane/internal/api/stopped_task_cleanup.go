package api

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/hostpath"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// ConfirmStoppedTaskCleanup proves this exact reserved task's full backend
// Create returned. It neither releases capacity nor approves its result.
func (a *API) ConfirmStoppedTaskCleanup(ctx context.Context, row registry.Session) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if a.deps.Product == nil || a.execution == nil || row.ID == "" || row.ProjectID == "" ||
		row.ProjectRunID == "" || row.ProjectRunOperationID == "" || row.SandboxID == "" {
		return errors.New("stopped task ownership is unconfirmed")
	}
	run, err := a.deps.Product.GetProjectSwarmRun(row.ProjectID, row.ProjectRunID)
	if err != nil {
		return err
	}
	if run.ID != row.ProjectRunID || run.ProjectID != row.ProjectID {
		return errors.New("stopped task run identity changed")
	}
	matched := false
	for _, op := range run.Operations {
		if op.ID == row.ProjectRunOperationID && op.Kind == "session" &&
			op.TaskID == "" && op.ChildID == row.ID && run.WorktreeID == row.WorktreeID &&
			op.RequestDigest == product.ProjectRunSessionRequestDigest(row) {
			// A positively identified serial run retains its existing cleanup
			// policy; this sentinel must never stand for missing private state.
			return backend.ErrUnsupported
		}
		if row.SandboxRef != "" && op.ID == row.ProjectRunOperationID && op.Kind == "session" && op.TaskID != "" &&
			op.ChildID == row.ID && op.WorktreeID == row.WorktreeID && op.AbsenceRetirement == nil &&
			op.RequestDigest == product.ProjectRunSessionRequestDigest(row) {
			matched = true
		}
	}
	if !matched {
		return errors.New("stopped task does not match its reserved operation")
	}
	return a.execution.confirmSettledTaskLease(ctx, row)
}

func (g *executionAdmission) confirmSettledTaskLease(ctx context.Context, row registry.Session) error {
	// The ledger is atomically replaced and bounded. This observer does not take
	// the admission mutex/file lock or create directories: a missing receipt is
	// a refusal, not a legacy success as in confirmNoPendingLaunch.
	if !filepath.IsAbs(g.path) {
		return errors.New("execution state path is not absolute")
	}
	if err := codexRegularPath(filepath.VolumeName(g.path)+string(filepath.Separator), g.path); err != nil {
		return err
	}
	if _, err := hostpath.ResolveOutside(g.api.projectsDir(), g.path); err != nil {
		return err
	}
	data, err := codexRead(filepath.Dir(g.path), g.path, 2<<20)
	if err != nil {
		return err
	}
	ledger, err := decodeExecutionLedger(data)
	if err != nil {
		return err
	}
	matches := 0
	for _, lease := range ledger.Leases {
		if lease.Scope != g.scope || lease.SessionID != row.ID {
			continue
		}
		if lease.ProjectID != row.ProjectID || lease.SandboxID != row.SandboxID || lease.LaunchGuard != 1 || lease.State != "backend-settled" {
			return errors.New("task backend launch has an unresolved or conflicting receipt")
		}
		matches++
	}
	if matches != 1 {
		return errors.New("task backend launch requires one exact settled receipt")
	}
	return ctx.Err()
}
