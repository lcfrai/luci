package api

import (
	"context"
	"errors"
	"os"
	"sync"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/product"
)

type checkExecutionObservations struct {
	mu        sync.Mutex
	cursor    int
	pending   []checkExecutionLease
	lastError string
}

// ObserveChecksOnce never allocates or starts a process and never resumes the
// next command. It visits at most eight small exact lease records; unattempted
// peers survive a slow first observation, as with task completion observation.
func (a *API) ObserveChecksOnce(ctx context.Context) (result error) {
	if a.deps.Product == nil {
		return nil
	}
	state := &a.execution.checkObserver
	if !state.mu.TryLock() {
		return nil
	}
	defer state.mu.Unlock()
	defer func() {
		message := ""
		if result != nil {
			message = result.Error()
			if len(message) > 4000 {
				message = message[:4000]
			}
		}
		if message != "" && message != state.lastError {
			a.sessionLog().Warn("check execution observation remains pending; exact ownership will be retried", "reason", message)
		}
		state.lastError = message
	}()
	if len(state.pending) == 0 {
		ledger, err := a.execution.readFenceLedger()
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if state.cursor >= len(ledger.Checks) {
			state.cursor = 0
		}
		visited := 0
		for visited < len(ledger.Checks) && visited < 128 && len(state.pending) < 8 {
			row := ledger.Checks[state.cursor]
			state.cursor = (state.cursor + 1) % len(ledger.Checks)
			visited++
			if row.Scope == a.execution.scope {
				state.pending = append(state.pending, row)
			}
		}
	}
	var failures []error
	for len(state.pending) > 0 {
		if ctx.Err() != nil {
			return errors.Join(append(failures, ctx.Err())...)
		}
		candidate := state.pending[0]
		state.pending[0] = checkExecutionLease{}
		state.pending = state.pending[1:]
		if err := a.observeCheckCandidate(ctx, candidate); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

func (a *API) observeCheckCandidate(ctx context.Context, lease checkExecutionLease) error {
	row, index, out, err := a.deps.Product.ProjectCheckObservation(lease.Identity.ProjectID, lease.Identity.CheckID, lease.ID)
	if err != nil {
		return err
	}
	unlock, err := a.execution.lockChildLifecycle(ctx, lease.ID)
	if err != nil {
		return err
	}
	defer unlock()
	ledger, err := a.execution.readFenceLedger()
	if err != nil {
		return err
	}
	current := false
	for _, v := range ledger.Checks {
		if v == lease {
			current = true
			break
		}
	}
	if !current {
		return nil
	}
	persist := func(value product.ProjectCheckCommandOutcome) error {
		next, e := a.deps.Product.RecordProjectCheckObservation(row, index, value)
		if e == nil {
			row = next
			out = value
		}
		return e
	}
	out.AllocationID = lease.ID
	out.ResourcesHeld = lease.State != "finished"
	if lease.State == "finished" {
		if lease.ResultDigest != checkOutcomeDigest(out) {
			return errors.New("retained check result no longer matches its settled receipt")
		}
		// A later command may already have a durable outcome even after its
		// lease was acknowledged, or may have been blocked before reservation.
		// Earlier receipts cannot replace that outcome with "skipped". Running
		// or queued rows alone are not outcomes: interruption before their first
		// reservation still needs the ordinary no-replay publication below.
		laterOutcome := false
		for _, command := range row.Commands[index+1:] {
			switch command.Status {
			case "passed", "failed", "blocked", "unknown", "cancelled":
				laterOutcome = true
			}
		}
		// A later retained lease also protects partial publication and keeps
		// its run-held flag and recovery work list under its own controller.
		for _, other := range ledger.Checks {
			if other.Scope == lease.Scope && other.Identity.CheckID == lease.Identity.CheckID && other.Identity.ProjectID == lease.Identity.ProjectID && other.Identity.CommandIndex > lease.Identity.CommandIndex {
				laterOutcome = true
			}
		}
		if laterOutcome {
			if !out.CleanupConfirmed {
				return errors.New("earlier check output publication is incomplete")
			}
			return a.execution.acknowledgeCheckCompletion(ctx, lease.Identity)
		}
		out.CleanupConfirmed = true
		out.ResourcesHeld = false
		if err = persist(out); err != nil {
			return err
		}
		return a.execution.acknowledgeCheckCompletion(ctx, lease.Identity)
	}
	if lease.State == "reserved" {
		out.Status, out.ExitCode, out.Stage, out.ReasonCode, out.Message = "blocked", nil, "admission", "check_controller_interrupted", "The service restarted before this command started. No command will be replayed."
		_, err = a.finishUnstartedCheck(ctx, lease.Identity, out, persist, false)
		return err
	}
	if lease.State == "creating" || lease.State == "fencing" {
		// Keep the exact published diagnosis when this retained attempt is
		// already unknown. Copy-only changes would invalidate recovery review
		// without new execution evidence. Use the original held flag; out was
		// updated from the lease above. Persistence still checks the captured
		// revision and repairs interrupted run-level publication when needed.
		if out.Status != "unknown" || out.ExitCode != nil || out.Stage != "create" || out.ReasonCode != "check_create_unconfirmed" || out.CleanupConfirmed || row.ResourcesHeld == nil || !*row.ResourcesHeld {
			out.Status, out.ExitCode, out.Stage, out.ReasonCode, out.Message = "unknown", nil, "create", "check_create_unconfirmed", "Docker creation is unconfirmed. Review this exact retained attempt to stop and recover it."
		}
		return persist(out)
	}
	executor, ok := a.deps.Backend.(backend.CheckExecutor)
	if !ok {
		return backend.ErrUnsupported
	}
	observed, err := executor.InspectCheck(ctx, lease.Identity, lease.Ref)
	if err != nil || observed.Ref != lease.Ref {
		return errors.New("exact check process observation is unavailable")
	}
	switch observed.State {
	case "running":
		out.Status, out.ExitCode, out.Stage, out.ReasonCode, out.Message = "unknown", nil, "observe", "check_controller_interrupted", "The exact command is still running after the service restarted. Its exit and output will be observed; remaining commands will not be replayed."
		return persist(out)
	case "created":
		out.Status, out.ExitCode, out.Stage, out.ReasonCode, out.Message = "blocked", nil, "cleanup", "check_controller_interrupted", "The exact created command was never started by this controller. Its retained object is being removed."
		_, err = a.finishAcknowledgedCheck(ctx, lease.Identity, lease.Ref, out, persist, false, false)
		return err
	case "exited":
		if observed.ExitCode == nil {
			return errors.New("the exact stopped check has no exit result")
		}
		logs, e := executor.CheckLogs(ctx, lease.Identity, lease.Ref, 256<<10)
		if e != nil {
			return e
		}
		out.Status, out.ExitCode, out.Output, out.Truncated = "passed", observed.ExitCode, logs.Text, logs.Truncated
		if *observed.ExitCode != 0 {
			out.Status = "failed"
		}
		out.Stage, out.ReasonCode, out.Message = "cleanup", "", "The exact command exit and output are retained; cleanup is pending."
		_, err = a.finishAcknowledgedCheck(ctx, lease.Identity, lease.Ref, out, persist, false, false)
		return err
	case "absent":
		// A durable immutable Create ACK excludes any future Create, and a
		// delayed Start cannot recreate its removed object. Missing output is
		// still unknown evidence even though resources can now be released.
		if out.ExitCode == nil || (out.Status != "passed" && out.Status != "failed") {
			out.Status, out.ExitCode, out.Stage, out.ReasonCode, out.Message = "unknown", nil, "finished", "check_output_unavailable", "The exact command container is absent, but its exit or output was not retained. Rerun explicitly for new evidence."
		}
		if err = persist(out); err != nil {
			return err
		}
		if err = a.execution.markCheckCleaning(ctx, lease.Identity, lease.Ref); err != nil {
			return err
		}
		out.CleanupConfirmed = true
		setConfirmedCheckCleanupMessage(&out)
		if err = a.execution.completeCheck(ctx, lease.Identity, lease.Ref, out); err != nil {
			return err
		}
		out.ResourcesHeld = false
		if err = persist(out); err != nil {
			return err
		}
		return a.execution.acknowledgeCheckCompletion(ctx, lease.Identity)
	default:
		return errors.New("the exact check process state is unsupported")
	}
}

// Retained checks can still write their original project after controller
// restart. Source integration/review must not mistake an empty live-job map
// for proof that an uncertain check allocation stopped.
func (a *API) projectCheckBusy(ctx context.Context, projectID string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	l, err := a.execution.readFenceLedger()
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	for _, v := range l.Checks {
		if v.Scope == a.execution.scope && v.Identity.ProjectID == projectID && v.State != "finished" {
			return true, nil
		}
	}
	return false, nil
}

// A finished lease no longer charges resources, but stays as a bounded restart
// publication receipt until the complete product outcome is durably visible.
func (a *API) acknowledgeProjectCheckResults(ctx context.Context, projectID, checkID string) error {
	row, err := a.deps.Product.GetProjectCheckRun(projectID, checkID)
	if err != nil {
		return err
	}
	if row.Status == "queued" || row.Status == "running" {
		return errors.New("check result publication remains in progress")
	}
	ledger, err := a.execution.readFenceLedger()
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, lease := range ledger.Checks {
		if lease.Scope != a.execution.scope || lease.Identity.ProjectID != projectID || lease.Identity.CheckID != checkID || lease.State != "finished" {
			continue
		}
		i := lease.Identity.CommandIndex
		if i < 0 || i >= len(row.Commands) {
			return errors.New("finished check result binding changed")
		}
		command := row.Commands[i]
		out := product.ProjectCheckCommandOutcome{AllocationID: command.AllocationID, Status: command.Status, Stage: command.Stage, ReasonCode: command.ReasonCode, Message: command.Message, ExitCode: command.ExitCode, Output: command.Output, Truncated: command.Truncated, CleanupConfirmed: command.CleanupConfirmed}
		if command.AllocationID != lease.ID || !command.CleanupConfirmed || checkOutcomeDigest(out) != lease.ResultDigest {
			return errors.New("finished check result differs from its durable outcome")
		}
		if err = a.execution.acknowledgeCheckCompletion(ctx, lease.Identity); err != nil {
			return err
		}
	}
	return nil
}
