package api

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// HostPressureObservation is a private control-plane read. The candidate is
// backed by both the exact local session and its durable, settled execution
// lease. The controller compares observations from every active owned runtime
// before asking one worker to stop its own child.
func (a *API) HostPressureObservation(ctx context.Context, hostTotal uint64) (management.HostPressureObservation, error) {
	var out management.HostPressureObservation
	if a.deps.Backend.Kind() != backend.KindDocker || a.deps.Product == nil || hostTotal == 0 {
		return out, nil
	}
	policy, err := a.deps.Product.SwarmSchedulerPolicy()
	if err != nil {
		return out, err
	}
	out.ReserveBytes = policy.MemoryHeadroomBytes(hostTotal)
	rows, err := a.deps.Store.List()
	if err != nil {
		return out, err
	}
	for i := len(rows) - 1; i >= 0; i-- {
		row := rows[i]
		if row.Backend != string(backend.KindDocker) || row.MachineID != LocalMachineID || row.SandboxID == "" || row.SandboxRef == "" ||
			(row.Status != registry.StatusStarting && row.Status != registry.StatusRunning && row.Status != registry.StatusIdle) {
			continue
		}
		lease, err := a.pressureLease(ctx, row)
		if err != nil {
			return out, err
		}
		if lease != "" {
			out.Candidate = &management.HostPressureCandidate{SessionID: row.ID, SandboxID: row.SandboxID, SandboxRef: row.SandboxRef, LeaseID: lease, CreatedAt: row.CreatedAt}
			return out, nil
		}
	}
	return out, nil
}

func (a *API) pressureLease(ctx context.Context, row registry.Session) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	ledger, err := a.execution.readFenceLedger()
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	for _, lease := range ledger.Leases {
		if lease.Scope == a.execution.scope && lease.SessionID == row.ID && lease.SandboxID == row.SandboxID && lease.ProjectID == row.ProjectID && lease.State == "backend-settled" && lease.LaunchGuard == 1 {
			return lease.ID, nil
		}
	}
	return "", nil
}

// StopHostPressureCandidate refuses a changed row or lease. A lost backend
// acknowledgment is an attempted stop and retains the original live record
// and allocation for explicit reconciliation; it is never retried this episode.
func (a *API) StopHostPressureCandidate(parent context.Context, candidate management.HostPressureCandidate) (bool, error) {
	if candidate.SessionID == "" || candidate.SandboxID == "" || candidate.SandboxRef == "" || candidate.LeaseID == "" || !sessionRestarts.acquire(candidate.SessionID) {
		return false, nil
	}
	defer sessionRestarts.release(candidate.SessionID)
	chosen, err := a.deps.Store.Get(candidate.SessionID)
	if err != nil {
		return false, err
	}
	if chosen.SandboxID != candidate.SandboxID || chosen.SandboxRef != candidate.SandboxRef || !chosen.CreatedAt.Equal(candidate.CreatedAt) || chosen.Backend != string(backend.KindDocker) || chosen.MachineID != LocalMachineID || (chosen.Status != registry.StatusStarting && chosen.Status != registry.StatusRunning && chosen.Status != registry.StatusIdle) {
		return false, nil
	}
	lease, err := a.pressureLease(parent, chosen)
	if err != nil || lease != candidate.LeaseID {
		return false, err
	}
	if chosen.Harness == "codex" {
		a.codexAccountMu.Lock()
		defer a.codexAccountMu.Unlock()
	}
	current, err := a.deps.Store.Get(chosen.ID)
	if err != nil {
		return false, err
	}
	if current.SandboxID != chosen.SandboxID || current.SandboxRef != chosen.SandboxRef ||
		(current.Status != registry.StatusStarting && current.Status != registry.StatusRunning && current.Status != registry.StatusIdle) {
		return false, nil
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	if err := a.deps.Backend.Destroy(ctx, current.SandboxID); err != nil && !errors.Is(err, backend.ErrNotFound) {
		return true, err // Keep the live record and held lease for recovery.
	}
	_, err = a.deps.Store.Update(current.ID, func(row *registry.Session) error {
		if row.SandboxID != current.SandboxID || row.SandboxRef != current.SandboxRef || row.Backend != current.Backend || row.MachineID != current.MachineID ||
			(row.Status != registry.StatusStarting && row.Status != registry.StatusRunning && row.Status != registry.StatusIdle) {
			return sessionErrMoved
		}
		row.Status = registry.StatusExited
		if row.ExitReason == "" {
			row.ExitReason = "Luci stopped this session to protect your computer's memory. Your project context is saved; retry when capacity recovers."
		}
		row.Endpoint, row.AttachToken = "", ""
		return nil
	})
	if err != nil {
		return true, err // The exact sandbox was stopped; never fabricate a settled lease.
	}
	releaseErr := a.execution.releaseSession(ctx, current.ID)
	if current.Harness == "codex" && current.CodexConnectionID != "" {
		account, err := a.readCodexConnection()
		if err != nil {
			return true, errors.Join(releaseErr, err)
		}
		if account != nil && account.ID == current.CodexConnectionID && account.ActiveSessionID == current.ID {
			if err := a.reconcileCodexConnection(account); err != nil {
				return true, errors.Join(releaseErr, err)
			}
			account.ActiveSessionID = ""
			if err := a.writeCodexConnection(account); err != nil {
				return true, errors.Join(releaseErr, err)
			}
		}
	}
	a.sessionLog().Warn("stopped exact local session after measured host reserve breach", "cause", "host_memory_pressure", "sessionId", current.ID, "sandboxId", current.SandboxID)
	return true, releaseErr // A held lease remains visible even after a proven stop.
}
