package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

var (
	errLaunchRecoveryUnproved = errors.New("This exact uncertain launch could not be fenced safely. Its output and execution reservation remain retained; no replacement was started.")
	errLegacyLaunchRecovery   = errors.New("This launch predates the fence-aware execution protocol. An older delayed delete could remove a recovery marker, so Luci cannot safely release its reservation through this action.")
	errLaunchFenceLimit       = errors.New("The host has reached its limit of 64 permanent launch-recovery markers. No worker was stopped and no reservation was released.")
	errLaunchRetirementLimit  = errors.New("The host has reached its limit of 4096 retained execution-retirement records. This recovery cannot release its reservation; retain the saved state for operator inspection.")
)

type activeFenceRecovery struct {
	intent          product.ProjectSwarmRecoveryIntent
	operationDigest string
}

func (a *API) authorizeSwarmFenceRecovery(ctx context.Context, req product.ProjectSwarmFenceRecoveryRequest) error {
	return a.authorizeExecutionRetirement(ctx, req.ProjectID)
}

func (a *API) resolveFenceRecovery(ctx context.Context, req product.ProjectSwarmFenceRecoveryRequest) (*registry.Session, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if a.deps.Product == nil || req.ChildID == "" {
		return nil, errLaunchRecoveryUnproved
	}
	run, err := a.deps.Product.GetProjectSwarmRun(req.ProjectID, req.RunID)
	if err != nil || run.ID != req.RunID || run.ProjectID != req.ProjectID {
		return nil, errLaunchRecoveryUnproved
	}
	var selected *product.ProjectRunOperation
	for i := range run.Operations {
		if run.Operations[i].ID == req.OperationID {
			selected = &run.Operations[i]
			break
		}
	}
	if selected == nil || selected.Kind != "session" || selected.TaskID == "" || selected.ChildID != req.ChildID || selected.Allocation != "ready" || selected.Implementation == nil || selected.RequestDigest != req.RequestDigest {
		return nil, errLaunchRecoveryUnproved
	}
	data, err := json.Marshal(selected.Implementation)
	if err != nil {
		return nil, errLaunchRecoveryUnproved
	}
	digest := sha256.Sum256(data)
	if hex.EncodeToString(digest[:]) != req.ImplementationDigest {
		return nil, errLaunchRecoveryUnproved
	}
	original := selected.UpdatedAt
	if selected.LaunchRecovery != nil {
		original = selected.LaunchRecovery.OriginalUpdatedAt
		if req.Intent != nil && *req.Intent != *selected.LaunchRecovery {
			return nil, errLaunchRecoveryUnproved
		}
	}
	if original != req.UpdatedAt {
		return nil, errLaunchRecoveryUnproved
	}
	queueID, err := a.deps.Product.SwarmQueueIDForOperation(req.ProjectID, req.RunID, req.OperationID)
	if err != nil || queueID != req.QueueID {
		return nil, errLaunchRecoveryUnproved
	}
	row, err := a.deps.Store.Get(req.ChildID)
	if errors.Is(err, registry.ErrNotFound) {
		return nil, nil
	}
	if err != nil || row.ID != req.ChildID || row.ProjectID != req.ProjectID || row.ProjectRunID != req.RunID || row.ProjectRunOperationID != req.OperationID || row.WorktreeID != selected.WorktreeID || row.SandboxID != req.ChildID || row.Backend != string(backend.KindDocker) || product.ProjectRunSessionRequestDigest(row) != req.RequestDigest || row.SandboxRef != "" && !exactHex(row.SandboxRef, 32) {
		return nil, errLaunchRecoveryUnproved
	}
	if req.Intent != nil && req.Intent.FenceRequest.ExpectedOriginalRef != "" && row.SandboxRef != req.Intent.FenceRequest.ExpectedOriginalRef {
		return nil, errLaunchRecoveryUnproved
	}
	return &row, nil
}

func lookupFenceRecord(ledger executionLeaseLedger, scope string, req product.ProjectSwarmFenceRecoveryRequest) (*executionFenceRecord, error) {
	for _, record := range ledger.Fences {
		if record.Scope != scope || record.ChildID != req.ChildID {
			continue
		}
		if !fenceRecordMatches(record, req) || req.Intent != nil && record.Intent != *req.Intent {
			return nil, errLaunchRecoveryUnproved
		}
		copy := record
		return &copy, nil
	}
	if req.Intent != nil {
		return nil, errLaunchRecoveryUnproved
	}
	return nil, nil
}

func lookupFencedLease(ledger executionLeaseLedger, scope string, req product.ProjectSwarmFenceRecoveryRequest) (executionLease, error) {
	var found executionLease
	for _, lease := range ledger.Leases {
		if lease.Scope != scope {
			continue
		}
		if req.QueueID != "" && lease.ID != req.QueueID || req.QueueID == "" && lease.SessionID != req.ChildID {
			continue
		}
		if found.ID != "" || lease.ProjectID != req.ProjectID || lease.SessionID != req.ChildID || lease.SandboxID != req.ChildID || lease.LaunchGuard != 1 {
			return executionLease{}, errLaunchRecoveryUnproved
		}
		found = lease
	}
	if found.ID == "" {
		return found, errLaunchRecoveryUnproved
	}
	if found.FenceProtocol != backend.LaunchFenceProtocol {
		return found, errLegacyLaunchRecovery
	}
	if found.State != "backend-dispatching" && found.State != "fencing" {
		return found, errLaunchRecoveryUnproved
	}
	return found, nil
}

// This is a read-only capability view. It neither probes Docker nor creates
// private files; current authority and engine identity are checked on action.
func (a *API) projectSwarmFenceRecoveryAvailability(ctx context.Context, req product.ProjectSwarmFenceRecoveryRequest) (product.ProjectSwarmRecoveryAvailability, error) {
	view := product.ProjectSwarmRecoveryAvailability{ReasonCode: "recovery_unsupported", Message: "This execution service cannot safely stop this uncertain launch. Output and capacity remain retained."}
	if err := a.authorizeExecutionRetirement(ctx, req.ProjectID); err != nil {
		return view, err
	}
	if _, err := a.resolveFenceRecovery(ctx, req); err != nil {
		view.ReasonCode = "recovery_identity_unconfirmed"
		return view, nil
	}
	if _, ok := a.deps.Backend.(backend.LaunchFencer); !ok || a.deps.Backend.Kind() != backend.KindDocker {
		return view, nil
	}
	ledger, err := a.execution.readFenceLedger()
	if err != nil {
		view.ReasonCode = "recovery_state_unavailable"
		return view, nil
	}
	record, err := lookupFenceRecord(ledger, a.execution.scope, req)
	if err != nil {
		view.ReasonCode = "recovery_identity_unconfirmed"
		return view, nil
	}
	if record == nil {
		if _, err := lookupFencedLease(ledger, a.execution.scope, req); err != nil {
			view.ReasonCode = "recovery_launch_unconfirmed"
			if errors.Is(err, errLegacyLaunchRecovery) {
				view.ReasonCode, view.Message = "recovery_legacy_launch", err.Error()
			}
			return view, nil
		}
		if len(ledger.Fences)+len(ledger.CheckFences) >= backend.MaxLaunchFences {
			view.ReasonCode, view.Message = "recovery_history_full", errLaunchFenceLimit.Error()
			return view, nil
		}
		if len(ledger.Retirements) >= 4096 {
			view.ReasonCode, view.Message = "recovery_retirement_history_full", errLaunchRetirementLimit.Error()
			return view, nil
		}
	}
	view.Supported, view.ReasonCode, view.Message = true, "recovery_supported", "This launch supports explicit stop-and-retain recovery. Luci will verify the original engine and retain a permanent recovery marker before releasing capacity."
	return view, nil
}

func (a *API) acquireSwarmFenceRecovery(ctx context.Context, req product.ProjectSwarmFenceRecoveryRequest) (product.ProjectSwarmRecoveryIntent, func(), error) {
	var intent product.ProjectSwarmRecoveryIntent
	if err := a.authorizeExecutionRetirement(ctx, req.ProjectID); err != nil {
		return intent, nil, err
	}
	if _, err := a.resolveFenceRecovery(ctx, req); err != nil {
		return intent, nil, err
	}
	fencer, ok := a.deps.Backend.(backend.LaunchFencer)
	if !ok || a.deps.Backend.Kind() != backend.KindDocker {
		return intent, nil, backend.ErrUnsupported
	}
	ledger, err := a.execution.readFenceLedger()
	if err != nil {
		return intent, nil, errLaunchRecoveryUnproved
	}
	record, err := lookupFenceRecord(ledger, a.execution.scope, req)
	if err != nil {
		return intent, nil, err
	}
	if record == nil {
		if _, err := lookupFencedLease(ledger, a.execution.scope, req); err != nil {
			return intent, nil, err
		}
	}
	if !sessionRestarts.acquire(req.ChildID) {
		return intent, nil, errExecutionLifecycleBusy
	}
	childRelease, err := a.execution.lockChildLifecycle(ctx, req.ChildID)
	if err != nil {
		sessionRestarts.release(req.ChildID)
		return intent, nil, err
	}
	failed := true
	defer func() {
		if failed {
			childRelease()
			sessionRestarts.release(req.ChildID)
		}
	}()
	row, err := a.resolveFenceRecovery(ctx, req)
	if err != nil {
		return intent, nil, err
	}
	bounded, cancel := context.WithTimeout(ctx, 3*time.Second)
	identity, err := fencer.LaunchFenceIdentity(bounded)
	cancel()
	if err != nil || !validFenceIdentity(identity) {
		return intent, nil, errLaunchRecoveryUnproved
	}
	if err := a.authorizeExecutionRetirement(ctx, req.ProjectID); err != nil {
		return intent, nil, err
	}
	actor, err := management.CaptureExecutionControlActor(ctx, req.ProjectID)
	if err != nil {
		return intent, nil, err
	}
	requestedBy := actor.UserID
	if actor.Local {
		requestedBy = "local-owner"
	}
	err = a.execution.mutate(ctx, func(current *executionLeaseLedger) error {
		existing, err := lookupFenceRecord(*current, a.execution.scope, req)
		if err != nil {
			return err
		}
		if existing != nil {
			if existing.Intent.FenceRequest.EngineID != identity.EngineID || existing.Intent.FenceRequest.InstanceID != identity.InstanceID || row != nil && existing.Intent.FenceRequest.ExpectedOriginalRef != "" && row.SandboxRef != existing.Intent.FenceRequest.ExpectedOriginalRef {
				return errLaunchRecoveryUnproved
			}
			intent = existing.Intent
			return nil
		}
		lease, err := lookupFencedLease(*current, a.execution.scope, req)
		if err != nil {
			return err
		}
		if lease.State != "backend-dispatching" || lease.FenceEngineID != identity.EngineID || lease.FenceInstanceID != identity.InstanceID {
			return errLaunchRecoveryUnproved
		}
		if len(current.Fences)+len(current.CheckFences) >= backend.MaxLaunchFences {
			return errLaunchFenceLimit
		}
		if len(current.Retirements) >= 4096 {
			return errLaunchRetirementLimit
		}
		var nonce [16]byte
		if _, err := rand.Read(nonce[:]); err != nil {
			return err
		}
		now := time.Now().UTC()
		originalRef := ""
		if row != nil {
			originalRef = row.SandboxRef
		}
		intent = product.ProjectSwarmRecoveryIntent{ID: hex.EncodeToString(nonce[:]), RequestedAt: now.Format(time.RFC3339Nano), RequestedBy: requestedBy, OriginalUpdatedAt: req.UpdatedAt, RequestDigest: req.RequestDigest, ImplementationDigest: req.ImplementationDigest, LeaseID: lease.ID, QueueID: req.QueueID, ScopeDigest: a.execution.scope, Protocol: backend.LaunchFenceProtocol}
		intent.FenceRequest = backend.LaunchFenceRequest{EngineID: lease.FenceEngineID, InstanceID: lease.FenceInstanceID, SandboxID: lease.SandboxID, SessionID: lease.SessionID, IntentID: intent.ID, ScopeDigest: a.execution.scope, OperationDigest: product.ProjectSwarmFenceOperationDigest(req), ExpectedOriginalRef: originalRef}
		for i := range current.Leases {
			if current.Leases[i] == lease {
				current.Leases[i].State, current.Leases[i].FenceIntentID, current.Leases[i].UpdatedAt = "fencing", intent.ID, now
				break
			}
		}
		current.Fences = append(current.Fences, executionFenceRecord{Scope: a.execution.scope, ProjectID: req.ProjectID, RunID: req.RunID, OperationID: req.OperationID, ChildID: req.ChildID, QueueID: req.QueueID, Intent: intent, State: "intent", CreatedAt: now, UpdatedAt: now})
		return validateExecutionFenceRecords(*current)
	})
	if err != nil {
		return intent, nil, err
	}
	guard := &activeFenceRecovery{intent: intent, operationDigest: intent.FenceRequest.OperationDigest}
	if _, loaded := a.execution.fenceGuards.LoadOrStore(intent.ID, guard); loaded {
		return intent, nil, errExecutionLifecycleBusy
	}
	failed = false
	var once sync.Once
	release := func() {
		once.Do(func() {
			a.execution.fenceGuards.CompareAndDelete(intent.ID, guard)
			childRelease()
			sessionRestarts.release(req.ChildID)
		})
	}
	return intent, release, nil
}

func (a *API) requireFenceGuard(req product.ProjectSwarmFenceRecoveryRequest, intent product.ProjectSwarmRecoveryIntent) error {
	value, ok := a.execution.fenceGuards.Load(intent.ID)
	if !ok {
		return errExecutionLifecycleBusy
	}
	guard, ok := value.(*activeFenceRecovery)
	if !ok || guard.intent != intent || guard.operationDigest != product.ProjectSwarmFenceOperationDigest(req) {
		return errLaunchRecoveryUnproved
	}
	return nil
}

func (a *API) proveSwarmFencedLaunch(ctx context.Context, req product.ProjectSwarmFenceRecoveryRequest, intent product.ProjectSwarmRecoveryIntent) (product.ProjectSwarmRecoveryProof, error) {
	var proof product.ProjectSwarmRecoveryProof
	if err := a.requireFenceGuard(req, intent); err != nil {
		return proof, err
	}
	if err := a.authorizeExecutionRetirement(ctx, req.ProjectID); err != nil {
		return proof, err
	}
	saved, _, err := a.deps.Product.ProjectSwarmLaunchRecovery(req.ProjectID, req.RunID, req.OperationID, req.ChildID, req.QueueID)
	if err != nil || saved == nil || *saved != intent {
		return proof, errLaunchRecoveryUnproved
	}
	row, err := a.resolveFenceRecovery(ctx, req)
	if err != nil {
		return proof, err
	}
	ledger, err := a.execution.readFenceLedger()
	if err != nil {
		return proof, errLaunchRecoveryUnproved
	}
	record, err := lookupFenceRecord(ledger, a.execution.scope, req)
	if err != nil || record == nil || record.Intent != intent {
		return proof, errLaunchRecoveryUnproved
	}
	fencer, ok := a.deps.Backend.(backend.LaunchFencer)
	if !ok {
		return proof, backend.ErrUnsupported
	}
	bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if record.State == "intent" {
		request := intent.FenceRequest
		if request.ExpectedOriginalRef == "" && row != nil {
			request.ExpectedOriginalRef = row.SandboxRef
		}
		fence, err := fencer.FenceLaunch(bounded, request)
		if err != nil {
			return proof, err
		}
		proof = product.ProjectSwarmRecoveryProof{Method: product.ProjectSwarmFenceRecoveryMethod, FenceProof: fence, ChildID: req.ChildID, QueueID: req.QueueID, LeaseID: intent.LeaseID, Backend: string(backend.KindDocker), ObservedAt: fence.ObservedAt}
	} else {
		if err := fencer.VerifyLaunchFence(bounded, intent.FenceRequest, record.Proof.FenceProof); err != nil {
			return proof, err
		}
		proof = record.Proof
		if record.State != "retired" {
			proof.ObservedAt = time.Now().UTC().Format(time.RFC3339Nano)
			proof.FenceProof.ObservedAt = proof.ObservedAt
		}
	}
	observed, err := time.Parse(time.RFC3339Nano, proof.ObservedAt)
	if err != nil || observed.After(time.Now()) || time.Since(observed) > 5*time.Second || !fenceProofMatches(proof, *record) {
		return product.ProjectSwarmRecoveryProof{}, errLaunchRecoveryUnproved
	}
	if err := a.authorizeExecutionRetirement(ctx, req.ProjectID); err != nil {
		return product.ProjectSwarmRecoveryProof{}, err
	}
	err = a.execution.mutate(ctx, func(current *executionLeaseLedger) error {
		for i := range current.Fences {
			if current.Fences[i] != *record {
				continue
			}
			if record.State == "retired" {
				return errLaunchRecoveryUnproved
			}
			current.Fences[i].Proof, current.Fences[i].State, current.Fences[i].UpdatedAt = proof, "proved", time.Now().UTC()
			return validateExecutionFenceRecords(*current)
		}
		return errLaunchRecoveryUnproved
	})
	if err != nil {
		return product.ProjectSwarmRecoveryProof{}, err
	}
	return proof, nil
}

func (a *API) finalizeSwarmFencedLaunch(ctx context.Context, req product.ProjectSwarmFenceRecoveryRequest, intent product.ProjectSwarmRecoveryIntent, proof product.ProjectSwarmRecoveryProof) error {
	if err := a.requireFenceGuard(req, intent); err != nil {
		return err
	}
	if err := a.authorizeExecutionRetirement(ctx, req.ProjectID); err != nil {
		return err
	}
	saved, savedProof, err := a.deps.Product.ProjectSwarmLaunchRecovery(req.ProjectID, req.RunID, req.OperationID, req.ChildID, req.QueueID)
	if err != nil || saved == nil || *saved != intent || savedProof == nil || *savedProof != proof {
		return errLaunchRecoveryUnproved
	}
	retired, err := a.deps.Product.ProjectSwarmRetirement(req.ProjectID, req.RunID, req.OperationID, req.ChildID, req.QueueID)
	if err != nil || retired == nil || *retired != proof {
		return errLaunchRecoveryUnproved
	}
	ledger, err := a.execution.readFenceLedger()
	if err != nil {
		return errLaunchRecoveryUnproved
	}
	record, err := lookupFenceRecord(ledger, a.execution.scope, req)
	if err != nil || record == nil || record.Intent != intent || record.Proof != proof || record.State != "proved" && record.State != "retired" {
		return errLaunchRecoveryUnproved
	}
	fencer, ok := a.deps.Backend.(backend.LaunchFencer)
	if !ok {
		return backend.ErrUnsupported
	}
	bounded, cancel := context.WithTimeout(ctx, 10*time.Second)
	err = fencer.VerifyLaunchFence(bounded, intent.FenceRequest, proof.FenceProof)
	cancel()
	if err != nil {
		return err
	}
	if err := a.authorizeExecutionRetirement(ctx, req.ProjectID); err != nil {
		return err
	}
	row, err := a.resolveFenceRecovery(ctx, req)
	if err != nil {
		return err
	}
	if row != nil {
		_, err = a.deps.Store.Update(row.ID, func(current *registry.Session) error {
			if current.ID != row.ID || current.ProjectID != req.ProjectID || current.ProjectRunID != req.RunID || current.ProjectRunOperationID != req.OperationID || current.WorktreeID != row.WorktreeID || current.SandboxID != intent.FenceRequest.SandboxID || current.SandboxRef != row.SandboxRef || product.ProjectRunSessionRequestDigest(*current) != req.RequestDigest {
				return errLaunchRecoveryUnproved
			}
			current.Status, current.Endpoint, current.AttachToken = registry.StatusExited, "", ""
			current.ExitReason = "the operator retired this uncertain launch behind a permanent execution fence; retained output requires review"
			return nil
		})
		if err != nil {
			return err
		}
	}
	if err := a.authorizeExecutionRetirement(ctx, req.ProjectID); err != nil {
		return err
	}
	return a.execution.mutate(ctx, func(current *executionLeaseLedger) error {
		at := -1
		for i := range current.Fences {
			if current.Fences[i] == *record {
				at = i
				break
			}
		}
		if at < 0 {
			return errLaunchRecoveryUnproved
		}
		if current.Fences[at].State == "retired" {
			return nil
		}
		lease, err := lookupFencedLease(*current, a.execution.scope, req)
		if err != nil || lease.State != "fencing" || lease.ID != intent.LeaseID || lease.FenceIntentID != intent.ID {
			return errLaunchRecoveryUnproved
		}
		if len(current.Retirements) >= 4096 {
			return errLaunchRetirementLimit
		}
		leaseAt := -1
		for i, value := range current.Leases {
			if value == lease {
				leaseAt = i
				break
			}
		}
		if leaseAt < 0 {
			return errLaunchRecoveryUnproved
		}
		now := time.Now().UTC()
		current.Retirements = append(current.Retirements, executionRetirement{Scope: a.execution.scope, LeaseID: intent.LeaseID, ProjectID: req.ProjectID, RunID: req.RunID, OperationID: req.OperationID, ChildID: req.ChildID, QueueID: req.QueueID, Proof: proof, RetiredAt: now})
		current.Leases = append(current.Leases[:leaseAt], current.Leases[leaseAt+1:]...)
		current.Fences[at].State, current.Fences[at].UpdatedAt = "retired", now
		return validateExecutionFenceRecords(*current)
	})
}
