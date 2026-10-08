package reaper

import (
	"context"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

type stoppedDestroyer interface {
	DestroyStopped(context.Context, backend.Sandbox) error
}

// A registry observation cannot terminate a newer restart. UpdatedAt also
// fences changes which retain the same allocation identity (for example a
// fresh attach), without holding the registry lock across backend I/O.
func sameObservedSession(a, b registry.Session) bool {
	return a.ID == b.ID && a.Status == b.Status && a.SandboxID == b.SandboxID &&
		a.SandboxRef == b.SandboxRef && a.UpdatedAt.Equal(b.UpdatedAt) &&
		a.ProjectID == b.ProjectID && a.WorktreeID == b.WorktreeID &&
		a.ProjectRunID == b.ProjectRunID && a.ProjectRunOperationID == b.ProjectRunOperationID
}

func (r *Reaper) markObservedExited(observed registry.Session, reason string) (registry.Session, error) {
	row, err := r.store.Update(observed.ID, func(current *registry.Session) error {
		if current.Status == registry.StatusExited || !sameObservedSession(*current, observed) {
			return errUnchanged
		}
		current.Status, current.ExitReason = registry.StatusExited, reason
		current.Endpoint, current.AttachToken, current.AttachCount = "", "", 0
		return nil
	})
	r.reportUpdate(observed.ID, err)
	return row, err
}

func (r *Reaper) strictTaskCleanup(row registry.Session) bool {
	return r.confirmStoppedTaskCleanup != nil && row.ProjectRunID != "" && row.ProjectRunOperationID != ""
}

func (r *Reaper) stoppedTaskMatches(sb backend.Sandbox, row registry.Session) bool {
	return r.instance != "" && sb.Instance == r.instance && sb.Kind == backend.KindDocker &&
		sb.ID == row.SandboxID && sb.Ref != "" && sb.Ref == row.SandboxRef &&
		sb.State == backend.StateStopped && !sb.CreatedOnly && !sb.CreationInFlight && !sb.NonLoopback &&
		sb.Labels[backend.LabelManaged] == "true" && sb.Labels[backend.LabelInstance] == r.instance &&
		sb.Labels[backend.LabelSandboxID] == row.SandboxID && sb.Labels[backend.LabelSessionID] == row.ID
}

func (r *Reaper) collectStoppedTask(ctx context.Context, observed registry.Session, listed backend.Sandbox) (removed, applicable bool) {
	// Only an exact private run lookup can positively identify a non-task run
	// child for the legacy path. Missing/corrupt state is never that signal.
	proofErr := r.confirmStoppedTaskCleanup(ctx, observed)
	if proofErr == backend.ErrUnsupported {
		return false, false
	}
	// A missing callback/capability, uncertain create or foreign/legacy identity
	// is not positive proof. Never fall through to force-by-name for this task.
	destroyer, ok := r.be.(stoppedDestroyer)
	if !ok || !r.stoppedTaskMatches(listed, observed) {
		if observed.Status != registry.StatusExited {
			r.markObservedExited(observed, reasonProcessExit)
		}
		return false, true
	}
	current, err := r.be.Get(ctx, listed.ID)
	if err != nil || current.Ref != listed.Ref || !r.stoppedTaskMatches(current, observed) {
		return false, true
	}
	// Capture the positive settled receipt before publishing exited: normal
	// capacity reconciliation may retire that receipt once the sandbox is gone.
	committed := observed
	if observed.Status != registry.StatusExited {
		committed, err = r.markObservedExited(observed, reasonProcessExit)
		if err != nil {
			return false, true
		}
	}
	if proofErr != nil {
		return false, true
	}
	latest, err := r.store.Get(observed.ID)
	if err != nil || !sameObservedSession(latest, committed) || ctx.Err() != nil {
		return false, true
	}
	// The backend rechecks the immutable container and refuses a running one.
	// No registry lock spans this call, and no failure enables a force retry.
	if err := destroyer.DestroyStopped(ctx, current); err != nil {
		r.log.Warn("could not remove a confirmed stopped task; retaining its allocation for a later check",
			"session", observed.ID, "sandbox", current.ID, "ref", current.Ref, "err", err)
		return false, true
	}
	r.log.Info("removed a confirmed stopped task", "session", observed.ID, "sandbox", current.ID, "ref", current.Ref)
	return true, true
}
