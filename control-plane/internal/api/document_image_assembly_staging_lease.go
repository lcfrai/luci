package api

import (
	"context"
	"errors"
	"os"
	"sync"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// Pre-staging is the SAME allocation/serial lease, not an uncharged helper.
// It reserves host originals before Open and is atomically converted to the
// normal container reservation only after positive original-set Close.
type documentAssemblyStageBinding struct {
	InputsDigest           string                            `json:"inputsDigest"`
	Demand                 backend.DocumentDomainInputDemand `json:"demand"`
	DirectoryBytes         uint64                            `json:"directoryBytes"`
	State                  string                            `json:"state"` // reserved, opening, open, unknown, closed
	WriterHandles          int                               `json:"writerHandles"`
	WriterBufferBytes      int                               `json:"writerBufferBytes"`
	StageWorkers           int                               `json:"stageWorkers"`
	StageRetirementWorkers int                               `json:"stageRetirementWorkers"`
}

func validDocumentAssemblyStage(s documentAssemblyStageBinding) bool {
	d := s.Demand
	if s.WriterHandles != 1 || s.WriterBufferBytes != 64<<10 || s.StageWorkers != 1 || s.StageRetirementWorkers != 1 || d.DeclaredBytes <= 0 || d.OriginalHandles <= d.Inputs || s.DirectoryBytes < uint64(d.DeclaredBytes) {
		return false
	}
	if !assemblyCatalogHex(s.InputsDigest) || d.Inputs < 1 || d.Inputs > backend.DocumentDomainInputLimit || d.OriginalHandles < 2 || d.OriginalHandles > backend.DocumentDomainInputHandleLimit || d.DeclaredBytes < 0 || d.DeclaredBytes > backend.DocumentDomainInputTotalBytes || d.ReadCalls != backend.DocumentDomainInputReadCalls || d.ReadBytes != backend.DocumentDomainInputReadBytes || d.BlockBytes != backend.DocumentDomainInputBlockBytes || d.ReadBufferBytes != 2*backend.DocumentDomainInputBlockBytes || d.AcquisitionWorkers != 1 || d.RetirementWorkers != 1 || d.CancellationWorkers != 1 || d.CancellationCallbacks != d.Inputs+2 || s.DirectoryBytes < 1 {
		return false
	}
	switch s.State {
	case "reserved", "opening", "open", "unknown", "closed":
		return true
	}
	return false
}

type liveDocumentAssemblyStage struct {
	mu                                                                           sync.Mutex
	owner                                                                        backend.DocumentDomainInputs
	writer                                                                       *os.File
	opening, closing, retiring, writing, writerUnknown, workerRunning, abandoned bool
	workerDone, retirementDone                                                   chan struct{}
	workerErr, retirementErr                                                     error
}

// Exactly one stage worker and one retirement worker may be outstanding. A
// deadline latches retirement and returns to the caller independently of a
// blocked original Open/Write/Sync/Close. Their backing and charge stay in live
// custody until the actual stage return and positive Close, never a timer ACK.
func (g *executionAdmission) runDocumentAssemblyStage(ctx context.Context, row documentImageAssemblyLease, live *liveDocumentAssemblyStage, source, directory string, stage assemblyStageSelection, current func(context.Context) error) error {
	return g.runDocumentAssemblyStageWork(ctx, row, live, func() error { return g.stageDocumentAssembly(ctx, row, live, source, directory, stage, current) })
}

func (g *executionAdmission) runDocumentAssemblyStageWork(ctx context.Context, row documentImageAssemblyLease, live *liveDocumentAssemblyStage, work func() error) error {
	if live == nil || work == nil {
		return backend.ErrUnsupported
	}
	live.mu.Lock()
	if live.workerDone != nil || live.retiring {
		live.mu.Unlock()
		return backend.ErrLaunchFenceOwnership
	}
	live.workerDone = make(chan struct{})
	live.workerRunning = true
	done := live.workerDone
	live.mu.Unlock()
	go func() {
		e := work()
		live.mu.Lock()
		live.workerErr = e
		live.workerRunning = false
		close(done)
		live.mu.Unlock()
	}()
	select {
	case <-done:
		live.mu.Lock()
		e := live.workerErr
		live.mu.Unlock()
		return errors.Join(e, g.waitDocumentAssemblyStageRetirement(ctx, row, live, e != nil, false))
	case <-ctx.Done():
		_, _ = g.beginDocumentAssemblyStageRetirement(ctx, row, live, true, false)
		return ctx.Err()
	}
}

func (g *executionAdmission) beginDocumentAssemblyStageRetirement(ctx context.Context, row documentImageAssemblyLease, live *liveDocumentAssemblyStage, abandon, retry bool) (<-chan struct{}, error) {
	if live == nil {
		return nil, backend.ErrUnavailable
	}
	live.mu.Lock()
	live.retiring = true
	live.abandoned = live.abandoned || abandon
	if live.retirementDone != nil {
		select {
		case <-live.retirementDone:
			if live.retirementErr == nil || !retry {
				done := live.retirementDone
				live.mu.Unlock()
				return done, nil
			}
		default:
			done := live.retirementDone
			live.mu.Unlock()
			return done, nil
		}
	}
	if live.workerDone == nil {
		if live.workerRunning || live.opening || live.writing {
			live.mu.Unlock()
			return nil, backend.ErrUnavailable
		}
		live.workerDone = make(chan struct{})
		close(live.workerDone)
	}
	deadline := row.WholeDeadline
	if retry {
		deadline = time.Now().Add(backend.DocumentImageAssemblyCleanupBudget)
		if caller, ok := ctx.Deadline(); ok && caller.Before(deadline) {
			deadline = caller
		}
	}
	originalDone := live.workerDone
	done := make(chan struct{})
	live.retirementDone = done
	live.mu.Unlock()
	go func() {
		cleanup, stop := context.WithDeadline(context.Background(), deadline)
		defer stop()
		var e error
		select {
		case <-originalDone:
			e = cleanup.Err()
		case <-cleanup.Done():
			e = cleanup.Err()
		}
		if e == nil {
			e = g.closeDocumentAssemblyStage(cleanup, row, live)
		}
		live.mu.Lock()
		abandoned := live.abandoned
		live.mu.Unlock()
		if e == nil && abandoned {
			e = g.finishUnstartedDocumentAssembly(cleanup, row)
		}
		live.mu.Lock()
		live.retirementErr = e
		close(done)
		live.mu.Unlock()
	}()
	return done, nil
}

func (g *executionAdmission) waitDocumentAssemblyStageRetirement(ctx context.Context, row documentImageAssemblyLease, live *liveDocumentAssemblyStage, abandon, retry bool) error {
	done, e := g.beginDocumentAssemblyStageRetirement(ctx, row, live, abandon, retry)
	if e != nil {
		return e
	}
	select {
	case <-done:
		live.mu.Lock()
		e = live.retirementErr
		live.mu.Unlock()
		return e
	case <-ctx.Done():
		live.mu.Lock()
		live.abandoned = true
		live.retiring = true
		live.mu.Unlock()
		return ctx.Err()
	}
}

var liveDocumentAssemblyStages sync.Map

func (g *executionAdmission) assemblyStageKey(id string) string {
	return g.path + "\x00" + g.scope + "\x00" + id
}

func (g *executionAdmission) reserveDocumentAssemblyStage(ctx context.Context, row documentImageAssemblyLease, policy product.SwarmSchedulerPolicy, current func(context.Context) error) error {
	if row.Stage == nil || row.Stage.State != "reserved" || row.State != "staging" || !validDocumentImageAssemblyLease(row) || current == nil || g.api.deps.ResourceSnapshot == nil || g.api.deps.Store == nil {
		return backend.ErrUnsupported
	}
	return g.mutate(ctx, func(l *executionLeaseLedger) error {
		if e := current(ctx); e != nil {
			return e
		}
		authority, e := g.api.authorizeRuntimeInstaller(ctx, row.Actor)
		if e != nil {
			return e
		}
		if !row.Actor.Local && (authority.Policy.CheckMemoryMiB < 2048 || authority.Policy.CheckCPUMillis < 1000) {
			return errCheckResourcePolicy
		}
		if assemblyOtherClaims(*l, g.scope, row.ID) {
			return errDocumentImageAssemblySerial
		}
		for _, old := range l.ImageAssemblies {
			if old.Scope == g.scope && old.ID == row.ID {
				return backend.ErrLaunchFenceOwnership
			}
		}
		snap := g.api.deps.ResourceSnapshot()
		now := time.Now().UTC()
		if !freshExecutionCapacity(snap, now) || snap.Execution.EngineID != row.Identity.EngineID || len(snap.Execution.Containers) != 0 || !now.Before(row.ComputeDeadline) {
			return backend.ErrUnavailable
		}
		sessions, e := g.api.deps.Store.List()
		if e != nil {
			return e
		}
		for _, session := range sessions {
			if session.Status != registry.StatusExited {
				return errDocumentImageAssemblySerial
			}
		}
		var demand executionCapacityDemand
		rows := append(append([]documentImageAssemblyLease(nil), l.ImageAssemblies...), row)
		if e = g.addAssemblyAdmissionCapacity(ctx, snap, rows, g.scope, row.ID, &demand, policy.DiskReserveMiB); e != nil {
			return e
		}
		// The prospective stage is skipped above only to avoid duplicate resource
		// counting. Its actual host staging+capture disk remains explicitly charged.
		demand.PendingDiskBytes += row.Stage.DirectoryBytes + row.CaptureDiskBytes
		demand.HostLaunchBytes = row.HostLaunchBytes
		if code, message := evaluateExecutionCapacity(snap, policy, row.Resources, demand, now); code != "" {
			return errors.New(message)
		}
		if executionLeaseCount(*l) >= 4096 {
			return backend.ErrUnavailable
		}
		l.ImageAssemblies = append(l.ImageAssemblies, row)
		return nil
	})
}
func (g *executionAdmission) advanceDocumentAssemblyStage(ctx context.Context, id backend.DocumentImageAssemblyIdentity, from, to string) error {
	return g.mutate(ctx, func(l *executionLeaseLedger) error {
		i, e := assemblyLeaseIndex(l, id)
		if e != nil {
			return e
		}
		row := &l.ImageAssemblies[i]
		if row.State != "staging" || row.Stage == nil || row.Stage.State != from {
			return backend.ErrLaunchFenceOwnership
		}
		next := *row.Stage
		next.State = to
		if !validDocumentAssemblyStage(next) {
			return backend.ErrUnsupported
		}
		row.Stage = &next
		row.UpdatedAt = time.Now().UTC()
		return nil
	})
}

func (g *executionAdmission) closeDocumentAssemblyStage(ctx context.Context, row documentImageAssemblyLease, live *liveDocumentAssemblyStage) error {
	if live == nil {
		return backend.ErrUnavailable
	}
	live.mu.Lock()
	live.retiring = true
	if live.workerRunning || live.opening || live.closing || live.writing || live.writerUnknown {
		live.mu.Unlock()
		return backend.ErrUnavailable
	}
	owner := live.owner
	live.closing = true
	live.mu.Unlock()
	var e error
	if owner != nil {
		e = owner.Close()
	}
	live.mu.Lock()
	live.closing = false
	if e == nil {
		live.owner = nil
	}
	live.mu.Unlock()
	if e != nil {
		return e
	}
	ledger, e := g.readReservationLedger(ctx)
	if e != nil {
		return e
	}
	i, e := assemblyLeaseIndex(&ledger, row.Identity)
	if e != nil {
		return e
	}
	actual := ledger.ImageAssemblies[i]
	if actual.Stage == nil {
		return backend.ErrLaunchFenceOwnership
	}
	if actual.Stage.State == "closed" {
		return nil
	}
	if e = g.advanceDocumentAssemblyStage(ctx, row.Identity, actual.Stage.State, "closed"); e != nil {
		return e
	}
	liveDocumentAssemblyStages.Delete(g.assemblyStageKey(row.ID))
	return nil
}
