package api

import (
	"context"
	"errors"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

// localHostPressureGuard samples the host without a Docker call. One pressure
// episode can stop at most one exact Luci session; persistent pressure cannot
// cascade through the user's other projects or unrelated host processes.
type localHostPressureGuard struct {
	sample         func(context.Context) resourcebudget.HostSnapshot
	policy         func() (product.SwarmSchedulerPolicy, error)
	stop           func(context.Context) (bool, error)
	log            func(error)
	record         func(uint64, uint64)
	below, healthy int
	acted          bool
}

func (g *localHostPressureGuard) step(ctx context.Context) {
	s := g.sample(ctx)
	now := time.Now().UTC()
	if !s.Fresh(now, 2*time.Second) || s.Memory.EffectiveAvailableBytes == nil || s.Memory.EffectiveTotalBytes == nil {
		return // An unavailable reading is never proof that stopping work helps.
	}
	p, err := g.policy()
	if err != nil {
		g.log(err)
		return
	}
	reserve := p.MemoryHeadroomBytes(*s.Memory.EffectiveTotalBytes)
	if *s.Memory.EffectiveAvailableBytes >= reserve {
		g.below = 0
		// Re-arm only after a stable recovery with a margin. A Docker/WSL
		// cache oscillating around the reserve must not stop another project.
		if *s.Memory.EffectiveAvailableBytes >= reserve+(256<<20) {
			g.healthy++
		} else {
			g.healthy = 0
		}
		if g.healthy >= 8 {
			g.acted = false
		}
		return
	}
	g.healthy = 0
	g.below++
	if g.below < 2 || g.acted {
		return
	}
	acted, err := g.stop(ctx)
	if acted {
		g.acted = true // Unknown stop outcomes must not be replayed in this episode.
		if g.record != nil {
			g.record(*s.Memory.EffectiveAvailableBytes, reserve)
		}
	}
	if err != nil {
		g.log(err)
	}
}

// RunLocalHostPressureGuard runs only for the local Docker controller. Project
// context and UI reads remain available while launches wait or one owned worker
// is stopped. The separate host-only probe does no Docker inventory or writes.
func (a *API) RunLocalHostPressureGuard(ctx context.Context) {
	if a.deps.ManagedExecution || a.deps.Product == nil || a.deps.Backend.Kind() != backend.KindDocker || a.deps.ResourceSnapshot == nil {
		return
	}
	probe := resourcebudget.NewHostProbe(resourcebudget.ProbeOptions{})
	g := localHostPressureGuard{
		sample: probe.Sample,
		policy: a.deps.Product.SwarmSchedulerPolicy,
		stop:   a.stopNewestOwnedSessionForPressure,
		log: func(err error) {
			a.sessionLog().Warn("local host pressure response retained its exact outcome", "err", err)
		},
		record: func(available, reserve uint64) {
			a.sessionLog().Warn("local host reserve breached; one exact owned session stop attempted", "availableBytes", available, "reserveBytes", reserve)
		},
	}
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			g.step(ctx)
		}
	}
}

func (a *API) stopNewestOwnedSessionForPressure(parent context.Context) (bool, error) {
	rows, err := a.deps.Store.List()
	if err != nil {
		return false, err
	}
	var chosen registry.Session
	for _, row := range rows {
		if row.Backend != string(backend.KindDocker) || row.MachineID != LocalMachineID || row.SandboxID == "" || row.SandboxRef == "" ||
			(row.Status != registry.StatusStarting && row.Status != registry.StatusRunning) {
			continue
		}
		if chosen.ID == "" || row.CreatedAt.After(chosen.CreatedAt) {
			chosen = row
		}
	}
	if chosen.ID == "" || !sessionRestarts.acquire(chosen.ID) {
		return false, nil
	}
	defer sessionRestarts.release(chosen.ID)
	if chosen.Harness == "codex" {
		a.codexAccountMu.Lock()
		defer a.codexAccountMu.Unlock()
	}
	current, err := a.deps.Store.Get(chosen.ID)
	if err != nil {
		return false, err
	}
	if current.SandboxID != chosen.SandboxID || current.SandboxRef != chosen.SandboxRef ||
		(current.Status != registry.StatusStarting && current.Status != registry.StatusRunning) {
		return false, nil
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	if err := a.deps.Backend.Destroy(ctx, current.SandboxID); err != nil && !errors.Is(err, backend.ErrNotFound) {
		return true, err // Keep the live record and held lease for recovery.
	}
	_, err = a.deps.Store.Update(current.ID, func(row *registry.Session) error {
		if row.SandboxID != current.SandboxID || row.SandboxRef != current.SandboxRef || row.Backend != current.Backend || row.MachineID != current.MachineID ||
			(row.Status != registry.StatusStarting && row.Status != registry.StatusRunning) {
			return sessionErrMoved
		}
		row.Status = registry.StatusExited
		if row.ExitReason == "" {
			row.ExitReason = "host_memory_pressure"
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
	a.sessionLog().Warn("stopped exact local session after measured host reserve breach", "sessionId", current.ID, "sandboxId", current.SandboxID)
	return true, releaseErr // A held lease remains visible even after a proven stop.
}
