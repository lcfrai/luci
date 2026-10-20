package api

import (
	"context"
	"errors"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/product"
)

// Dedicated typed adapter: ordinary CheckExecutor configuration and semantics
// are unchanged. A nil optional dependency truthfully leaves native OCR disabled.
func (a *API) documentExtractionRuntime() product.DiscoveryExtractionRuntime {
	return product.DiscoveryExtractionRuntime{
		CaptureActor: a.captureCheckActor, AuthorizeControl: a.authorizeExecutionRetirement,
		Busy: a.documentExtractionBusy, Execute: a.executeDocumentExtraction,
		Read: a.readDocumentExtraction, Recover: a.recoverDocumentExtraction,
	}
}

func (a *API) documentExtractionBusy(ctx context.Context, projectID, documentID string) (bool, error) {
	if _,err:=a.resolveDocumentExtractor(ctx);err!=nil{return false,err}
	busy := false
	// Called only by an explicit extraction POST, not a GET. Initialize an
	// absent ledger under its usual lock; unknown/corrupt ownership stays busy.
	err := a.execution.mutate(ctx, func(ledger *executionLeaseLedger) error {
		for _, lease := range ledger.Documents {
			if lease.State != "finished" {
				busy = true
				break
			}
		}
		return nil
	})
	return busy || err != nil, err
}

func (a *API) readDocumentExtraction(ctx context.Context, projectID, documentID, attemptID string) (product.DiscoveryExtractionOutcome, error) {
	lease, err := a.execution.documentLease(ctx, projectID, documentID, attemptID)
	if err != nil {
		return product.DiscoveryExtractionOutcome{}, err
	}
	// This is a read projection, not an output receipt or a recovery. A finished
	// lease alone never creates extracted evidence or invents a successful exit.
	return product.DiscoveryExtractionOutcome{Identity: lease.Identity, Ref: lease.Ref,
		State: "unavailable", Stage: lease.State, ReasonCode: "document_private_receipt_required",
		Message: "Inspect the exact retained private extraction receipt.", ResourcesHeld: lease.State != "finished"}, nil
}

type documentCapture struct {
	output backend.DocumentExtractionOutput
	err    error
}

func (a *API) executeDocumentExtraction(ctx context.Context, req product.DiscoveryExtractionRequest, persist func(product.DiscoveryExtractionOutcome) error) (product.DiscoveryExtractionOutcome, error) {
	out := product.DiscoveryExtractionOutcome{State: "blocked", Stage: "admission", ReaderSettled: true, CleanupConfirmed: true,
		ReasonCode: "document_not_started", Message: "No document parser was started."}
	publish := func() (product.DiscoveryExtractionOutcome, error) {
		if persist == nil {
			return out, errors.New("document receipt publisher missing")
		}
		return out, persist(out)
	}
	fail := func(code, message string) (product.DiscoveryExtractionOutcome, error) {
		out.ReasonCode, out.Message = code, message
		return publish()
	}
	if !product.ValidateDiscoveryExtractionRequest(req) || req.ValidateCurrent == nil || req.OriginalUserID < 1 || req.OriginalGroupID < 1 || req.ExpectedDocumentRevision < 1 || !time.Now().Before(req.ComputeDeadline) {
		return fail("document_scope_invalid", "The exact private original, attempt or total deadline cannot be verified.")
	}
	executor,resolveErr := a.resolveDocumentExtractor(ctx)
	if resolveErr!=nil || executor == nil {
		return fail("document_extractor_unavailable", "No reviewed contained extractor is configured; the original remains retained.")
	}
	compute, cancel := context.WithDeadline(ctx, req.ComputeDeadline)
	defer cancel()
	identity := backend.DocumentExtractionIdentity{Protocol: backend.DocumentExtractionProtocol, Profile: backend.DocumentExtractionProfile,
		ScopeDigest: a.execution.scope, AllocationID: req.AllocationID, ProjectID: req.ProjectID, DocumentID: req.DocumentID,
		AttemptID: req.AttemptID, OriginalDigest: req.Original.Digest,
		ActorDigest: backend.DocumentExtractionActorDigest(req.Actor.UserID, req.Actor.TeamID, req.Actor.ProjectID, req.Actor.Local)}
	spec := backend.DocumentExtractionSpec{Identity: identity, ImageID: executor.DocumentImageID(), OriginalPath: req.OriginalPath,
		OriginalBytes: req.Original.Bytes, MimeType: req.Original.MimeType, UserID: req.OriginalUserID, GroupID: req.OriginalGroupID}
	spec.Identity.CommandDigest = backend.DocumentExtractionCommandDigest(spec)
	if err := req.ValidateCurrent(compute); err != nil {
		return fail("document_context_changed", "The exact extraction draft changed before admission.")
	}
	prepared, err := executor.PrepareDocument(compute, spec)
	if err != nil || prepared.Spec.Identity.CommandDigest != spec.Identity.CommandDigest || backend.DocumentExtractionCommandDigest(prepared.Spec) != spec.Identity.CommandDigest || prepared.Spec.Identity.AllocationID != req.AllocationID || !backend.ValidateDocumentExtractionIdentity(prepared.Spec.Identity, true) {
		return fail("document_prepare_unavailable", "The selected local engine, private original and pinned contained image could not be verified. No image was acquired or parser started.")
	}
	identity, out.Identity = prepared.Spec.Identity, prepared.Spec.Identity
	admit := func() error {
		if err := req.ValidateCurrent(compute); err != nil {
			return err
		}
		authority, err := a.authorizeChecks(compute, req.Actor)
		if err != nil {
			return err
		}
		policy, err := a.deps.Product.SwarmSchedulerPolicy()
		if err != nil {
			return err
		}
		if err := a.documentExtractionHeadroom(identity.EngineID, 8<<30); err != nil {
			return err
		}
		decision, err := a.execution.reserveDocument(compute, prepared, req, authority, policy)
		if err != nil {
			return err
		}
		if decision.State != "admitted" {
			return errors.New(decision.Code + ": " + decision.Message)
		}
		return nil
	}
	if err = admit(); err != nil {
		if lease, e := a.execution.documentLease(compute, req.ProjectID, req.DocumentID, req.AttemptID); e == nil && lease.Identity == identity && lease.State != "finished" {
			out.ResourcesHeld = true
			out.CleanupConfirmed = false
			out.State = "unknown"
		}
		return fail("document_admission_unavailable", "Fresh authority, exact document context or shared host capacity did not admit this extraction.")
	}
	out.ResourcesHeld = true
	out.CleanupConfirmed = false
	release, err := a.execution.lockChildLifecycle(compute, req.AllocationID)
	if err != nil {
		out.State = "unknown"
		return fail("document_lifecycle_busy", "The exact lifecycle is busy; its capacity remains held.")
	}
	readerOwnsLock := false
	defer func() {
		if !readerOwnsLock {
			release()
		}
	}()
	if err = a.execution.requireReservedDocument(compute, identity, req.Actor); err != nil {
		return out, err
	}
	out.State, out.Stage, out.ReasonCode, out.Message = "working", "reserved", "", "Capacity is reserved; the exact parser has not started."
	if _, err = publish(); err != nil {
		return out, err
	}
	cleanup := func(stream backend.DocumentExtractionResultStream, captured <-chan documentCapture, code, message string) (product.DiscoveryExtractionOutcome, error) {
		// The one absolute whole deadline includes all closure and publication;
		// cancelling compute never resets or extends the reserved cleanup window.
		bounded, stop := context.WithDeadline(context.Background(), req.WholeDeadline)
		defer stop()
		out.Stage, out.ReasonCode, out.Message = "cleanup", code, message
		var absent backend.DocumentExtractionObservation
		retired := false
		retire := func() {
			if err := executor.RetireDocument(bounded, identity, out.Ref); err != nil {
				out.State, out.ReasonCode = "unknown", "document_cleanup_unconfirmed"
				return
			}
			observed, err := executor.InspectDocument(bounded, identity, out.Ref)
			if err != nil || observed.State != "absent" || observed.Ref != out.Ref {
				out.State, out.ReasonCode = "unknown", "document_absence_unconfirmed"
				return
			}
			absent, retired, out.CleanupConfirmed = observed, true, true
		}
		// Emergency retirement of a positively ACKed CID must not depend on
		// capture closure, private receipt publication or a ledger write. The
		// typed backend revalidates its exact identity/config before removal.
		// Keep the lease/lock charged until reader and durable state catch up.
		emergency := code != "" && out.Ref != ""
		if emergency {
			retire()
		}
		if stream != nil {
			stream.Close()
			if captured != nil {
				select {
				case value := <-captured:
					out.ReaderSettled = true
					if value.err == nil {
						out.Result = value.output
					}
				case <-bounded.Done():
					// Keep the OS lifecycle lock until this exact reader settles.
					// A later process can acquire it only after reader closure or
					// process death; it cannot race an orphan capture publisher.
					readerOwnsLock = true
					go func() { <-captured; release() }()
					out.State = "unknown"
					out.ReasonCode = "document_reader_unsettled"
					return publish()
				}
			}
		}
		// Ordinary already-exited success preserves complete capture before
		// retirement. Emergency stop has already been attempted independently.
		if _, err := publish(); err != nil {
			return out, err
		}
		if out.Ref == "" {
			out = documentSettled(out)
			if _, err := publish(); err != nil {
				return out, err
			}
			if err := a.execution.completeDocument(bounded, identity, "", out, backend.DocumentExtractionObservation{}); err != nil {
				return out, err
			}
			out.ResourcesHeld = false
			return publish()
		}
		if err := a.execution.markDocumentCleaning(bounded, identity, out.Ref, out.Result.Digest); err != nil {
			out.State = "unknown"
			return publish()
		}
		if !emergency {
			retire()
		}
		if !retired {
			return publish()
		}
		out = documentSettled(out)
		if _, err = publish(); err != nil {
			return out, err
		}
		if err = a.execution.completeDocument(bounded, identity, out.Ref, out, absent); err != nil {
			return out, err
		}
		out.ResourcesHeld = false
		return publish()
	}
	if err = admit(); err != nil {
		out.State = "blocked"
		return cleanup(nil, nil, "document_authority_changed", "Admission changed before Create; no parser was allocated.")
	}
	if err = a.execution.transitionDocument(compute, identity, "reserved", "creating", ""); err != nil {
		out.State = "unknown"
		return fail("document_create_journal_unconfirmed", "The exact Create journal is unconfirmed; its claim is retained.")
	}
	out.Stage = "creating"
	ref, err := executor.CreateDocument(compute, prepared)
	if err != nil || !checkHex(ref, 32) {
		out.State = "unknown"
		return fail("document_create_unconfirmed", "Create has no verified acknowledgment. Explicit fence recovery is required; no replacement or readback adoption is allowed.")
	}
	out.Ref = ref
	ack, stop := context.WithDeadline(context.Background(), req.WholeDeadline)
	err = a.execution.transitionDocument(ack, identity, "creating", "created", ref)
	stop()
	if err != nil {
		out.State = "unknown"
		return fail("document_create_ack_unconfirmed", "The exact Create acknowledgment could not be durably saved; fence recovery is required.")
	}
	stream, err := executor.AttachDocumentResult(compute, identity, ref)
	if err != nil {
		out.State = "unavailable"
		return cleanup(nil, nil, "document_attach_unavailable", "The exact pre-Start capture could not be established; no Start was sent.")
	}
	out.ReaderSettled = false
	captured := make(chan documentCapture, 1)
	go func() { result, err := stream.Capture(compute); captured <- documentCapture{result, err} }()
	if err = admit(); err != nil {
		out.State = "blocked"
		return cleanup(stream, captured, "document_authority_changed", "Authority, context or capacity changed before Start.")
	}
	if err = a.execution.transitionDocument(compute, identity, "created", "starting", ref); err != nil {
		out.State = "unknown"
		return cleanup(stream, captured, "document_start_journal_unconfirmed", "Start journal is unconfirmed; no Start was sent.")
	}
	if err = executor.StartDocument(compute, identity, ref); err != nil {
		out.State = "unavailable"
		return cleanup(stream, captured, "document_start_unconfirmed", "Start is unconfirmed; this exact CID is retired without replaying Start.")
	}
	if err = a.execution.transitionDocument(compute, identity, "starting", "running", ref); err != nil {
		out.State = "unavailable"
		return cleanup(stream, captured, "document_start_ack_unconfirmed", "The Start acknowledgment could not be saved; no Start will be replayed.")
	}
	out.Stage, out.Message = "running", "The owned contained extractor is running; output is untrusted evidence."
	if _, err = publish(); err != nil {
		out.State = "unavailable"
		return cleanup(stream, captured, "document_publication_unavailable", "Private progress publication failed; retiring the owned parser.")
	}
	for {
		if compute.Err() != nil {
			out.State = "cancelled"
			return cleanup(stream, captured, "document_compute_stopped", "Extraction was cancelled or reached its compute deadline; partial output is not evidence.")
		}
		if err := a.documentExtractionHeadroom(identity.EngineID, 4<<30); err != nil {
			out.State = "unavailable"
			return cleanup(stream, captured, "document_host_pressure", "Fresh host/engine reserves are unavailable; retiring the exact parser before accepting evidence.")
		}
		observed, err := executor.InspectDocument(compute, identity, ref)
		if err != nil || observed.Ref != ref {
			out.State = "unavailable"
			return cleanup(stream, captured, "document_observation_unavailable", "The exact parser state is unavailable; retiring only its owned CID.")
		}
		if observed.State == "exited" && observed.ExitCode != nil {
			out.ExitCode, out.OOMKilled = observed.ExitCode, observed.OOMKilled
			select {
			case value := <-captured:
				out.ReaderSettled = true
				captured = nil
				if value.err == nil {
					out.Result = value.output
				}
				out.State = "unavailable"
				if value.err == nil && *observed.ExitCode == 0 && !observed.OOMKilled && compute.Err() == nil {
					out.State = "complete"
				}
				return cleanup(stream, nil, "", "The exact exit and bounded capture were recorded; retirement is pending.")
			case <-compute.Done():
				out.State = "unavailable"
				return cleanup(stream, captured, "document_capture_incomplete", "The complete output stream was not captured before the compute deadline.")
			}
		}
		if observed.State != "running" {
			out.State = "unavailable"
			return cleanup(stream, captured, "document_exit_unconfirmed", "No verified process exit was observed; output cannot become evidence.")
		}
		timer := time.NewTimer(500 * time.Millisecond)
		select {
		case <-compute.Done():
			timer.Stop()
		case <-timer.C:
		}
	}
}

func documentSettled(out product.DiscoveryExtractionOutcome) product.DiscoveryExtractionOutcome {
	out.CleanupConfirmed = true
	if out.State == "complete" && out.ReasonCode == "" {
		out.Message = "Complete bounded capture, exact exit and retirement are recorded; evidence acceptance still requires the current document revision."
	}
	return out
}

func (a *API) documentExtractionHeadroom(engineID string, windowsMinimum uint64) error {
	if a.deps.ResourceSnapshot == nil {
		return backend.ErrUnavailable
	}
	snap, now := a.deps.ResourceSnapshot(), time.Now().UTC()
	if !freshExecutionCapacity(snap, now) || snap.Execution.EngineID != engineID {
		return backend.ErrUnavailable
	}
	policy, err := a.deps.Product.SwarmSchedulerPolicy()
	if err != nil {
		return err
	}
	if *snap.Host.Memory.EffectiveAvailableBytes < policy.MemoryHeadroomBytes(*snap.Host.Memory.EffectiveTotalBytes) {
		return backend.ErrUnavailable
	}
	if snap.Host.Platform == "windows" {
		physical, commit := snap.Host.Memory.PhysicalAvailableBytes, snap.Host.Memory.CommitAvailableBytes
		if physical == nil || commit == nil || *physical < windowsMinimum || *commit < windowsMinimum {
			return backend.ErrUnavailable
		}
		if windowsMinimum == 8<<30 && (snap.Host.CPU.UtilizationPercent == nil || *snap.Host.CPU.UtilizationPercent >= 50) {
			return backend.ErrUnavailable
		}
	}
	return nil
}
