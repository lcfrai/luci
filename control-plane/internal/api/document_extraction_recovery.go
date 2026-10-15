package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
)

func (a *API) recoverDocumentExtraction(ctx context.Context, req product.DiscoveryExtractionRecoveryRequest, persist func(product.DiscoveryExtractionOutcome) error) (product.DiscoveryExtractionOutcome, error) {
	out := req.Outcome
	if req.Action != "retire" || persist == nil || req.Actor.ProjectID != req.ProjectID || !checkLogicalID(req.AllocationID) {
		return out, backend.ErrLaunchFenceOwnership
	}
	controlActor, actorErr := management.CaptureExecutionControlActor(ctx, req.ProjectID)
	if actorErr != nil || controlActor != req.Actor {
		return out, backend.ErrLaunchFenceOwnership
	}
	if err := a.authorizeExecutionRetirement(ctx, req.ProjectID); err != nil {
		return out, err
	}
	// A separate explicit recovery gets at most sixty seconds. It cannot resume
	// compute, attach late, reset the original deadlines or allocate a replacement.
	bounded, cancel := context.WithTimeout(ctx, backend.DocumentCleanupBudget)
	defer cancel()
	release, err := a.execution.lockChildLifecycle(bounded, req.AllocationID)
	if err != nil {
		return out, err
	}
	defer release()
	lease, err := a.execution.documentLease(bounded, req.ProjectID, req.DocumentID, req.AttemptID)
	if err != nil && out.State == "claimed" && out.Identity == (backend.DocumentExtractionIdentity{}) && out.Ref == "" {
		// A durable product cancellation precedes this callback. A complete
		// locked ledger with no matching allocation plus the still pre-admission
		// private claim can settle a crash before reservation; unreadable state
		// or any ambiguous allocation is never treated as absence.
		ledger, readErr := a.execution.readReservationLedger(bounded)
		if readErr != nil {
			return out, readErr
		}
		for _, value := range ledger.Documents {
			if value.Scope == a.execution.scope && (value.ID == req.AllocationID || value.Identity.ProjectID == req.ProjectID && value.Identity.DocumentID == req.DocumentID && value.Identity.AttemptID == req.AttemptID) {
				return out, backend.ErrLaunchFenceOwnership
			}
		}
		out.State, out.Stage, out.ReasonCode, out.Message = "cancelled", "recovery", "document_unallocated_claim", "The exact pre-admission claim has no shared allocation; no parser was started."
		out.ResourcesHeld, out.ReaderSettled, out.CleanupConfirmed = false, true, true
		return out, persist(out)
	}
	if err != nil || lease.ID != req.AllocationID {
		return out, backend.ErrLaunchFenceOwnership
	}
	if out.Identity != (backend.DocumentExtractionIdentity{}) && out.Identity != lease.Identity {
		return out, backend.ErrLaunchFenceOwnership
	}
	if out.Ref != "" && lease.Ref != "" && out.Ref != lease.Ref {
		return out, backend.ErrLaunchFenceOwnership
	}
	if len(out.Result.Data) > 0 && (!checkHex(out.Result.Digest, 32) || lease.ResultDigest != "" && out.Result.Digest != lease.ResultDigest) {
		return out, backend.ErrLaunchFenceOwnership
	}
	if lease.State == "finished" {
		if documentOutcomeDigest(out) != lease.OutcomeDigest || !out.CleanupConfirmed || !out.ReaderSettled {
			return out, errors.New("exact final private document receipt is unavailable")
		}
		out.ResourcesHeld = false
		return out, persist(out)
	}
	// The lifecycle lock proves the previous process owner is gone. No SDK
	// capture is reopened. A lost capture stays unavailable after safe cleanup.
	out.Identity, out.Ref, out.ResourcesHeld, out.ReaderSettled = lease.Identity, lease.Ref, true, true
	out.State, out.Stage, out.ReasonCode, out.Message = "unavailable", "recovery", "document_capture_not_replayed", "Explicit recovery retires exact resources only; missing capture is not reconstructed."
	if err = persist(out); err != nil {
		return out, err
	}
	executor := a.deps.DocumentExtractor
	if executor == nil {
		return out, backend.ErrUnsupported
	}
	if lease.State == "reserved" {
		out.CleanupConfirmed = true
		if err = persist(out); err != nil {
			return out, err
		}
		if err = a.execution.completeDocument(bounded, lease.Identity, "", out, backend.DocumentExtractionObservation{}); err != nil {
			return out, err
		}
	} else if lease.State == "creating" || lease.State == "fencing" {
		record, err := a.execution.reserveDocumentFence(bounded, lease)
		if err != nil {
			return out, err
		}
		if err = a.authorizeExecutionRetirement(bounded, req.ProjectID); err != nil {
			return out, err
		}
		request := backend.DocumentExtractionFenceRequest{Identity: record.Identity, IntentID: record.IntentID, ExpectedOriginalRef: record.ExpectedOriginalRef}
		proof := record.Proof
		if record.State == "intent" {
			proof, err = executor.FenceDocumentLaunch(bounded, request)
		} else {
			err = executor.VerifyDocumentLaunchFence(bounded, request, proof)
		}
		if err != nil {
			return out, err
		}
		if err = a.execution.publishDocumentFence(bounded, record, proof); err != nil {
			return out, err
		}
		if err = a.authorizeExecutionRetirement(bounded, req.ProjectID); err != nil {
			return out, err
		}
		if err = executor.VerifyDocumentLaunchFence(bounded, request, proof); err != nil {
			return out, err
		}
		out.CleanupConfirmed = true
		if err = persist(out); err != nil {
			return out, err
		}
		if err = a.execution.completeFencedDocument(bounded, record, proof, out); err != nil {
			return out, err
		}
	} else {
		if !checkHex(lease.Ref, 32) {
			return out, backend.ErrLaunchFenceOwnership
		}
		if err = a.execution.markDocumentCleaning(bounded, lease.Identity, lease.Ref, out.Result.Digest); err != nil {
			return out, err
		}
		if err = a.authorizeExecutionRetirement(bounded, req.ProjectID); err != nil {
			return out, err
		}
		if err = executor.RetireDocument(bounded, lease.Identity, lease.Ref); err != nil {
			return out, err
		}
		absent, err := executor.InspectDocument(bounded, lease.Identity, lease.Ref)
		if err != nil || absent.State != "absent" || absent.Ref != lease.Ref {
			return out, backend.ErrUnavailable
		}
		if err = a.authorizeExecutionRetirement(bounded, req.ProjectID); err != nil {
			return out, err
		}
		out.CleanupConfirmed = true
		if err = persist(out); err != nil {
			return out, err
		}
		if err = a.execution.completeDocument(bounded, lease.Identity, lease.Ref, out, absent); err != nil {
			return out, err
		}
	}
	out.ResourcesHeld = false
	return out, persist(out)
}

func (g *executionAdmission) reserveDocumentFence(ctx context.Context, lease documentExtractionLease) (documentExtractionFenceRecord, error) {
	var result documentExtractionFenceRecord
	err := g.mutate(ctx, func(ledger *executionLeaseLedger) error {
		for _, old := range ledger.DocumentFences {
			if old.Scope == g.scope && old.Identity.AllocationID == lease.ID {
				if old.Identity != lease.Identity || old.Actor != lease.Actor || old.State == "retired" {
					return backend.ErrLaunchFenceOwnership
				}
				result = old
				return nil
			}
		}
		if len(ledger.Fences)+len(ledger.CheckFences)+len(ledger.DocumentFences) >= backend.MaxLaunchFences {
			return errors.New("permanent launch fence history is full")
		}
		for i := range ledger.Documents {
			value := &ledger.Documents[i]
			if value.Scope != g.scope || value.ID != lease.ID {
				continue
			}
			if *value != lease || value.State != "creating" {
				return backend.ErrLaunchFenceOwnership
			}
			var nonce [16]byte
			if _, err := rand.Read(nonce[:]); err != nil {
				return err
			}
			now := time.Now().UTC()
			result = documentExtractionFenceRecord{Scope: g.scope, Identity: lease.Identity, Actor: lease.Actor, IntentID: hex.EncodeToString(nonce[:]), ExpectedOriginalRef: lease.Ref, State: "intent", CreatedAt: now, UpdatedAt: now}
			value.State, value.UpdatedAt = "fencing", now
			ledger.DocumentFences = append(ledger.DocumentFences, result)
			return nil
		}
		return backend.ErrLaunchFenceOwnership
	})
	return result, err
}

func (g *executionAdmission) publishDocumentFence(ctx context.Context, record documentExtractionFenceRecord, proof backend.DocumentExtractionFenceProof) error {
	return g.mutate(ctx, func(ledger *executionLeaseLedger) error {
		for i := range ledger.DocumentFences {
			value := &ledger.DocumentFences[i]
			if value.Scope != g.scope || value.Identity.AllocationID != record.Identity.AllocationID {
				continue
			}
			if *value != record || value.State != "intent" && value.State != "proved" {
				return backend.ErrLaunchFenceOwnership
			}
			value.State, value.Proof, value.UpdatedAt = "proved", proof, time.Now().UTC()
			return validateDocumentExtractionRecords(ledger.Documents, ledger.DocumentFences)
		}
		return backend.ErrLaunchFenceOwnership
	})
}

func (g *executionAdmission) completeFencedDocument(ctx context.Context, record documentExtractionFenceRecord, proof backend.DocumentExtractionFenceProof, out product.DiscoveryExtractionOutcome) error {
	if !out.ReaderSettled || !out.CleanupConfirmed || out.Identity != record.Identity {
		return backend.ErrLaunchFenceOwnership
	}
	return g.mutate(ctx, func(ledger *executionLeaseLedger) error {
		fence, lease := -1, -1
		for i, value := range ledger.DocumentFences {
			if value.Scope == g.scope && value.Identity == record.Identity && value.IntentID == record.IntentID && value.State == "proved" && value.Proof == proof {
				fence = i
			}
		}
		for i, value := range ledger.Documents {
			if value.Scope == g.scope && value.Identity == record.Identity && value.Actor == record.Actor && value.State == "fencing" {
				lease = i
			}
		}
		if fence < 0 || lease < 0 {
			return backend.ErrLaunchFenceOwnership
		}
		now := time.Now().UTC()
		ledger.DocumentFences[fence].State, ledger.DocumentFences[fence].UpdatedAt = "retired", now
		ledger.Documents[lease].State, ledger.Documents[lease].OutcomeDigest, ledger.Documents[lease].UpdatedAt = "finished", documentOutcomeDigest(out), now
		return validateDocumentExtractionRecords(ledger.Documents, ledger.DocumentFences)
	})
}
