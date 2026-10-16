package api

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
)

// Unregistered and explicit-executor only. No Deps/New/Product/route/autostart
// wiring, engine selection, package acquisition or image registration exists.
func (a *API) executeDocumentImageAssembly(ctx context.Context, executor backend.DocumentImageAssemblyExecutor, req documentImageAssemblyRequest) (documentImageAssemblyOutcome, error) {
	if !validAssemblyRequest(req) {
		return documentImageAssemblyOutcome{State: "not_started", Stage: "admission", ReaderSettled: true, CleanupConfirmed: true, WatchdogSettled: true}, backend.ErrUnsupported
	}
	h, err := a.documentImageAssemblySharedHooks(executor, req)
	if err != nil {
		return documentImageAssemblyOutcome{State: "not_started", Stage: "admission", ReaderSettled: true, CleanupConfirmed: true, WatchdogSettled: true}, err
	}
	if err = h.Authorize(ctx, req.Actor, req.Spec.Identity); err != nil {
		return documentImageAssemblyOutcome{State: "not_started", Stage: "admission", ReaderSettled: true, CleanupConfirmed: true, WatchdogSettled: true}, err
	}
	// Initialize only the existing complete ledger for this explicit mutating
	// entrypoint; read projections never create a directory/lock or assume zero.
	if err = a.execution.mutate(ctx, func(*executionLeaseLedger) error { return nil }); err != nil {
		return documentImageAssemblyOutcome{State: "not_started", Stage: "admission", ReaderSettled: true, CleanupConfirmed: true, WatchdogSettled: true}, err
	}
	return runDocumentImageAssembly(ctx, executor, h, req)
}

func (a *API) documentImageAssemblySharedHooks(executor backend.DocumentImageAssemblyExecutor, req documentImageAssemblyRequest) (documentImageAssemblyHooks, error) {
	var h documentImageAssemblyHooks
	if a == nil || a.execution == nil || a.deps.Product == nil || executor == nil || req.ValidateCurrent == nil || !backend.ValidateDocumentImageAssemblySpec(req.Spec) || req.Spec.Identity.ScopeDigest != a.execution.scope || req.Spec.Identity.ProjectID != "" || req.Actor.ProjectID != "" || req.Actor.TeamID != "" || req.Spec.Identity.ActorDigest != backend.DocumentExtractionActorDigest(req.Actor.UserID, req.Actor.TeamID, req.Actor.ProjectID, req.Actor.Local) {
		return h, backend.ErrUnsupported
	}
	g := a.execution
	bound := func(identity backend.DocumentImageAssemblyIdentity) bool {
		copy := identity
		copy.EngineID, copy.InstanceID = "", ""
		return copy == req.Spec.Identity && backend.ValidateDocumentImageAssemblyIdentity(identity, true)
	}
	store := func(identity backend.DocumentImageAssemblyIdentity) (*assemblyPrivateCapture, error) {
		if !bound(identity) {
			return nil, backend.ErrLaunchFenceOwnership
		}
		return g.assemblyPrivateCapture(identity)
	}
	h.Authorize = func(ctx context.Context, actor management.ExecutionActorRef, identity backend.DocumentImageAssemblyIdentity) error {
		copy := identity
		copy.EngineID, copy.InstanceID = "", ""
		if actor != req.Actor || copy != req.Spec.Identity {
			return backend.ErrLaunchFenceOwnership
		}
		if _, err := a.authorizeRuntimeInstaller(ctx, actor); err != nil {
			return err
		}
		return req.ValidateCurrent(ctx)
	}
	h.Admit = func(ctx context.Context, p backend.PreparedDocumentImageAssembly, r documentImageAssemblyRequest) (documentImageAssemblyLease, error) {
		if r.Spec != req.Spec || r.Actor != req.Actor || r.CreatedAt != req.CreatedAt || r.ComputeDeadline != req.ComputeDeadline || r.WholeDeadline != req.WholeDeadline {
			return documentImageAssemblyLease{}, backend.ErrLaunchFenceOwnership
		}
		authority, err := a.authorizeRuntimeInstaller(ctx, req.Actor)
		if err != nil {
			return documentImageAssemblyLease{}, err
		}
		policy, err := a.deps.Product.SwarmSchedulerPolicy()
		if err != nil {
			return documentImageAssemblyLease{}, err
		}
		return g.reserveDocumentImageAssembly(ctx, p, req, authority, policy)
	}
	h.Lock = func(ctx context.Context, identity backend.DocumentImageAssemblyIdentity) (func(), error) {
		if !bound(identity) {
			return nil, backend.ErrLaunchFenceOwnership
		}
		release, err := g.lockChildLifecycle(ctx, "assembly-"+assemblyIdentityDigest(identity))
		if err != nil {
			return nil, err
		}
		l, err := g.readReservationLedger(ctx)
		if err == nil {
			var i int
			i, err = assemblyLeaseIndex(&l, identity)
			// The lifecycle lock also serves explicit retire/fence-only recovery.
			// Execute's Admit independently refuses every allocated replay.
			if err == nil && (l.ImageAssemblies[i].Actor != req.Actor || l.ImageAssemblies[i].State == "finished") {
				err = backend.ErrLaunchFenceOwnership
			}
		}
		if err != nil {
			release()
			return nil, err
		}
		return release, nil
	}
	h.Transition = func(ctx context.Context, identity backend.DocumentImageAssemblyIdentity, from, to, ref string) error {
		if !bound(identity) {
			return backend.ErrLaunchFenceOwnership
		}
		return g.mutate(ctx, func(l *executionLeaseLedger) error {
			i, err := assemblyLeaseIndex(l, identity)
			if err != nil {
				return err
			}
			row := &l.ImageAssemblies[i]
			allowed := from == "reserved" && to == "creating" || from == "creating" && to == "created" || from == "created" && to == "starting" || from == "starting" && to == "running"
			if !allowed || row.Actor != req.Actor || row.State != from || row.Ref != "" && row.Ref != ref {
				return backend.ErrLaunchFenceOwnership
			}
			row.State, row.Ref, row.UpdatedAt = to, ref, time.Now().UTC()
			if !validDocumentImageAssemblyLease(*row) {
				return backend.ErrLaunchFenceOwnership
			}
			return nil
		})
	}
	h.Headroom = func(ctx context.Context, engine string, minimum uint64) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if _, err := a.authorizeRuntimeInstaller(ctx, req.Actor); err != nil {
			return err
		}
		if err := a.documentExtractionHeadroom(engine, minimum); err != nil {
			return err
		}
		l, err := g.readReservationLedger(ctx)
		if err != nil { // Before first admission the existing ledger may be absent.
			return err
		}
		active := false
		for _, row := range l.ImageAssemblies {
			if row.State != "finished" {
				active = true
			}
		}
		if !active {
			return nil
		}
		policy, err := a.deps.Product.SwarmSchedulerPolicy()
		if err != nil {
			return err
		}
		var demand executionCapacityDemand
		return g.addAssemblyAdmissionCapacity(ctx, a.deps.ResourceSnapshot(), l.ImageAssemblies, g.scope, req.Spec.Identity.AllocationID, &demand, policy.DiskReserveMiB)
	}
	h.CaptureSink = func(ctx context.Context, identity backend.DocumentImageAssemblyIdentity) (io.WriteCloser, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		s, err := store(identity)
		if err != nil {
			return nil, err
		}
		return s.open()
	}
	h.HoldFence = func(ctx context.Context, fence backend.DocumentImageAssemblyFenceRequest) error {
		if !bound(fence.Identity) || !checkHex(fence.IntentID, 16) || fence.ExpectedRef != "" && !checkHex(fence.ExpectedRef, 32) {
			return backend.ErrLaunchFenceOwnership
		}
		return g.mutate(ctx, func(l *executionLeaseLedger) error {
			i, err := assemblyLeaseIndex(l, fence.Identity)
			if err != nil {
				return err
			}
			row := &l.ImageAssemblies[i]
			if row.Actor != req.Actor || row.State == "finished" || row.State == "reserved" || row.Ref != "" && row.Ref != fence.ExpectedRef {
				return backend.ErrLaunchFenceOwnership
			}
			for _, old := range l.ImageAssemblyFences {
				if old.Scope == g.scope && old.Identity.AllocationID == row.ID {
					if old.Identity == fence.Identity && old.IntentID == fence.IntentID && old.ExpectedRef == fence.ExpectedRef && old.State != "retired" {
						return nil
					}
					return backend.ErrLaunchFenceOwnership
				}
			}
			if executionFenceCount(*l) >= backend.MaxLaunchFences {
				return errors.New("permanent launch fence history is full")
			}
			now := time.Now().UTC()
			row.State, row.Ref, row.UpdatedAt = "fencing", fence.ExpectedRef, now
			l.ImageAssemblyFences = append(l.ImageAssemblyFences, documentImageAssemblyFenceRecord{Scope: g.scope, Identity: fence.Identity, Actor: req.Actor, IntentID: fence.IntentID, ExpectedRef: fence.ExpectedRef, State: "intent", CreatedAt: now, UpdatedAt: now})
			return nil
		})
	}
	h.RecordFence = func(ctx context.Context, fence backend.DocumentImageAssemblyFenceRequest, proof backend.DocumentImageAssemblyFenceProof) error {
		if !bound(fence.Identity) {
			return backend.ErrLaunchFenceOwnership
		}
		if err := executor.VerifyDocumentImageAssemblyFence(ctx, fence, proof); err != nil {
			return err
		}
		return g.mutate(ctx, func(l *executionLeaseLedger) error {
			for i := range l.ImageAssemblyFences {
				row := &l.ImageAssemblyFences[i]
				if row.Scope != g.scope || row.Identity != fence.Identity {
					continue
				}
				if row.Actor != req.Actor || row.IntentID != fence.IntentID || row.ExpectedRef != fence.ExpectedRef || row.State != "intent" && row.State != "proved" || row.State == "proved" && row.Proof != proof {
					return backend.ErrLaunchFenceOwnership
				}
				row.State, row.Proof, row.UpdatedAt = "proved", proof, time.Now().UTC()
				return nil
			}
			return backend.ErrLaunchFenceOwnership
		})
	}
	h.Publish = func(ctx context.Context, out documentImageAssemblyOutcome) error {
		s, err := store(out.Identity)
		if err != nil {
			return err
		}
		before, err := g.readReservationLedger(ctx)
		if err != nil {
			return err
		}
		index, err := assemblyLeaseIndex(&before, out.Identity)
		if err != nil {
			return err
		}
		prior := before.ImageAssemblies[index]
		if prior.Actor != req.Actor || prior.State == "finished" || prior.Ref != "" && prior.Ref != out.Ref || prior.Capture != (backend.DocumentImageAssemblyCapture{}) && prior.Capture != out.Capture || out.Ref != "" && prior.State == "reserved" {
			return backend.ErrLaunchFenceOwnership
		}
		exitVerified := false
		if out.Capture != (backend.DocumentImageAssemblyCapture{}) {
			previous, e := s.read()
			if e == nil && previous.CaptureExitVerified && previous.Outcome.Identity == out.Identity && previous.Outcome.Ref == out.Ref && previous.Outcome.Capture == out.Capture {
				exitVerified = true
			}
			if !exitVerified {
				observed, e := executor.InspectDocumentImageAssembly(ctx, out.Identity, out.Ref)
				if e != nil || observed.Ref != out.Ref || observed.State != "exited" || observed.ExitCode == nil || *observed.ExitCode != 0 || observed.OOMKilled {
					return backend.ErrLaunchFenceOwnership
				}
				exitVerified = true
			}
		}
		if err = s.publish(ctx, out, exitVerified); err != nil {
			return err
		}
		return g.mutate(ctx, func(l *executionLeaseLedger) error {
			i, e := assemblyLeaseIndex(l, out.Identity)
			if e != nil {
				return e
			}
			row := &l.ImageAssemblies[i]
			if *row != prior {
				return backend.ErrLaunchFenceOwnership
			}
			if out.Ref != "" {
				if !checkHex(out.Ref, 32) || row.State == "reserved" {
					return backend.ErrLaunchFenceOwnership
				}
				row.Ref = out.Ref
				if row.State != "fencing" {
					row.State = "cleaning"
				}
			}
			row.Capture, row.UpdatedAt = out.Capture, time.Now().UTC()
			return nil
		})
	}
	h.Finish = func(ctx context.Context, identity backend.DocumentImageAssemblyIdentity, out documentImageAssemblyOutcome) error {
		if !bound(identity) || out.Identity != identity || out.ResourcesHeld || !out.CleanupConfirmed || !out.ReaderSettled || !out.WatchdogSettled {
			return backend.ErrLaunchFenceOwnership
		}
		if out.State != "failed_owned_resources_retired" && out.State != "assembled_owned_resources_retired_not_registered" && out.State != "interrupted_capture_unavailable_owned_resources_retired" {
			return backend.ErrLaunchFenceOwnership
		}
		s, err := store(identity)
		if err != nil {
			return err
		}
		receipt, err := s.read()
		if err != nil || assemblyOutcomeDigest(receipt.Outcome) != assemblyOutcomeDigest(out) {
			return backend.ErrLaunchFenceOwnership
		}
		if out.State == "assembled_owned_resources_retired_not_registered" && (out.Capture == (backend.DocumentImageAssemblyCapture{}) || !receipt.CaptureExitVerified) {
			return backend.ErrLaunchFenceOwnership
		}
		if out.Capture != (backend.DocumentImageAssemblyCapture{}) {
			if err = s.verify(ctx, out.Capture); err != nil {
				return err
			}
		}
		l, err := g.readReservationLedger(ctx)
		if err != nil {
			return err
		}
		i, err := assemblyLeaseIndex(&l, identity)
		if err != nil {
			return err
		}
		before := l.ImageAssemblies[i]
		if before.Actor != req.Actor || before.Ref != out.Ref || before.State == "finished" || before.Capture != out.Capture {
			return backend.ErrLaunchFenceOwnership
		}
		fenceIndex := -1
		if before.State == "fencing" {
			for j, fence := range l.ImageAssemblyFences {
				if fence.Identity == identity && fence.Actor == req.Actor && fence.ExpectedRef == out.Ref && fence.State == "proved" {
					request := backend.DocumentImageAssemblyFenceRequest{Identity: identity, IntentID: fence.IntentID, ExpectedRef: fence.ExpectedRef}
					if err = executor.VerifyDocumentImageAssemblyFence(ctx, request, fence.Proof); err != nil {
						return err
					}
					fenceIndex = j
					break
				}
			}
			if fenceIndex < 0 {
				return backend.ErrLaunchFencePending
			}
		} else if out.Ref != "" {
			observed, e := executor.InspectDocumentImageAssembly(ctx, identity, out.Ref)
			if e != nil || observed.Ref != out.Ref || observed.State != "absent" {
				return backend.ErrLaunchFencePending
			}
		} else if before.State != "reserved" {
			return backend.ErrLaunchFencePending
		}
		return g.mutate(ctx, func(current *executionLeaseLedger) error {
			index, e := assemblyLeaseIndex(current, identity)
			if e != nil {
				return e
			}
			if current.ImageAssemblies[index] != before {
				return backend.ErrLaunchFenceOwnership
			}
			if fenceIndex >= 0 {
				if fenceIndex >= len(current.ImageAssemblyFences) || current.ImageAssemblyFences[fenceIndex] != l.ImageAssemblyFences[fenceIndex] {
					return backend.ErrLaunchFenceOwnership
				}
				current.ImageAssemblyFences[fenceIndex].State = "retired"
				current.ImageAssemblyFences[fenceIndex].UpdatedAt = time.Now().UTC()
			}
			row := &current.ImageAssemblies[index]
			row.State, row.OutcomeDigest, row.UpdatedAt = "finished", assemblyOutcomeDigest(out), time.Now().UTC()
			return nil
		})
	}
	return h, nil
}
