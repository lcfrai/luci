package product

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

var discoveryNativeAdmissionGate = make(chan struct{}, 1)

// These are the only registration replacements. Legacy UTF8 requests retain
// their original handler, request bodies, 120s receipts and linear extractor.
func (s *Server) startDiscoveryExtraction(w http.ResponseWriter, r *http.Request) {
	if !discoveryDocumentAccess(w, r, true) {
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	s.mu.RLock()
	saved, err := s.readDiscoveryDocument(p, r.PathValue("documentId"))
	s.mu.RUnlock()
	if err != nil {
		discoveryDocumentProblem(w, err)
		return
	}
	if saved.Document.Original.MimeType == "text/plain" || saved.Document.Original.MimeType == "text/markdown" {
		s.startDiscoveryTextExtraction(w, r)
		return
	}
	if saved.Document.Original.MimeType != "application/pdf" && saved.Document.Original.MimeType != "image/png" && saved.Document.Original.MimeType != "image/jpeg" {
		problem(w, 422, "document_format_unsupported", "The original is retained; this format has no configured extractor.")
		return
	}
	var req struct {
		ExpectedRevision int    `json:"expectedRevision"`
		IdempotencyKey   string `json:"idempotencyKey"`
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	select {
	case discoveryNativeAdmissionGate <- struct{}{}:
		defer func() { <-discoveryNativeAdmissionGate }()
	default:
		problem(w, 429, "document_extraction_busy", "Another native extraction or private original staging operation is active. Preserve the exact request key and inspect its receipt.")
		return
	}
	actor, _ := authenticatedContextActor(r.Context())
	digest := reviewDigest(struct {
		Original string
		Actor    string
	}{saved.Document.Original.Digest, actor.ID})
	if prior, found, e := discoveryRequestReadback(saved, req.IdempotencyKey, digest, "extract"); e != nil {
		discoveryDocumentProblem(w, e)
		return
	} else if found {
		s.writeDiscoveryNativeResponse(w, p, saved, prior.AttemptID)
		return
	}
	runtime := s.discoveryExtractionRuntime
	if runtime.CaptureActor == nil || runtime.Execute == nil || runtime.Busy == nil || runtime.jobs == nil {
		problem(w, 422, "extractor_unavailable", "The contained extractor is not configured. No acquisition, fallback or parser was started.")
		return
	}
	executionActor, err := runtime.CaptureActor(r.Context(), p.ID)
	if err != nil {
		problem(w, 403, "document_execution_forbidden", "Current project edit and execution authority are required.")
		return
	}
	busy, err := runtime.Busy(r.Context(), p.ID, saved.Document.ID)
	if err != nil || busy {
		problem(w, 409, "document_extraction_busy", "An active, unavailable or unconfirmed extraction retains the serial slot. Inspect and explicitly settle its exact receipt before restarting.")
		return
	}
	now := time.Now().UTC()
	s.mu.Lock()
	current, err := s.readDiscoveryDocument(p, saved.Document.ID)
	if err == nil {
		if prior, found, e := discoveryRequestReadback(current, req.IdempotencyKey, digest, "extract"); e != nil {
			s.mu.Unlock()
			discoveryDocumentProblem(w, e)
			return
		} else if found {
			s.mu.Unlock()
			s.writeDiscoveryNativeResponse(w, p, current, prior.AttemptID)
			return
		}
	}
	var attempt DiscoveryDocumentAttempt
	if err == nil {
		attempt, err = discoveryStartAttempt(&current, req.ExpectedRevision, req.IdempotencyKey, actor, now)
	}
	if err != nil {
		s.mu.Unlock()
		discoveryDocumentProblem(w, err)
		return
	}
	// This optional field is server-owned; it is never decoded from a browser.
	attempt.ExecutionProfile = backend.DocumentExtractionProfile
	attempt.ExpiresAt = now.Add(backend.DocumentWholeBudget).Format(time.RFC3339Nano)
	for i := range current.Document.Attempts {
		if current.Document.Attempts[i].ID == attempt.ID {
			current.Document.Attempts[i] = attempt
		}
	}
	request := DiscoveryExtractionRequest{ProjectID: p.ID, DocumentID: current.Document.ID, AttemptID: attempt.ID, AllocationID: newID("document-"), Original: current.Document.Original, Actor: executionActor, CreatedAt: now, ComputeDeadline: now.Add(backend.DocumentComputeBudget), WholeDeadline: now.Add(backend.DocumentWholeBudget), ExpectedDocumentRevision: current.Document.Revision}
	scope, err := s.approvalScopes(p.Path, p.ID)
	if err == nil {
		err = s.stageDiscoveryNativeOriginal(p, &request)
	}
	receipt := discoveryNativeReceipt{Version: 1, AccountScope: scope.AccountScope, ProjectScope: scope.ProjectScope, Request: request, Outcome: DiscoveryExtractionOutcome{State: "claimed", Stage: "claim", Message: "The exact native attempt is claimed; no parser has started.", ReaderSettled: true}}
	if err == nil {
		err = s.saveDiscoveryNative(p, receipt)
	}
	if err == nil {
		err = s.saveDiscoveryDocument(current)
	}
	s.mu.Unlock()
	if err != nil {
		discoveryDocumentProblem(w, err)
		return
	}
	request.ValidateCurrent = func(ctx context.Context) error { return s.validateDiscoveryNativeCurrent(ctx, p, receipt) }
	compute, cancel := context.WithDeadline(r.Context(), request.ComputeDeadline)
	stopWorkspace := context.AfterFunc(s.workspaceContext, cancel)
	defer func() { stopWorkspace(); cancel() }()
	job := &discoveryExtractionJob{cancel: cancel, done: make(chan struct{})}
	key := discoveryNativeJobKey(p.ID, request.DocumentID, request.AttemptID)
	runtime.jobs.mu.Lock()
	if _, exists := runtime.jobs.active[key]; exists {
		runtime.jobs.mu.Unlock()
		discoveryDocumentProblem(w, ErrProjectRunConflict)
		return
	}
	runtime.jobs.active[key] = job
	runtime.jobs.mu.Unlock()
	defer func() {
		runtime.jobs.mu.Lock()
		delete(runtime.jobs.active, key)
		close(job.done)
		runtime.jobs.mu.Unlock()
	}()
	persist := func(out DiscoveryExtractionOutcome) error { return s.persistDiscoveryNative(p, request, out) }
	out, executeErr := runtime.Execute(compute, request, persist)
	if executeErr != nil {
		problem(w, 409, "document_outcome_unconfirmed", "The exact private outcome could not be confirmed. Preserve the request key; no replacement was started.")
		return
	}
	if !out.ResourcesHeld && out.CleanupConfirmed && out.ReaderSettled {
		s.mu.Lock()
		retained, e := s.readDiscoveryNative(p, request.DocumentID, request.AttemptID)
		if e == nil {
			e = s.retireDiscoveryNativeOriginal(p, retained)
		}
		if e == nil {
			e = s.applyDiscoveryNativeOutcome(p, request, actor, req.IdempotencyKey, out)
		}
		s.mu.Unlock()
		if e != nil {
			problem(w, 409, "document_evidence_not_applied", "The private result is retained, but the current document changed or complete evidence could not be accepted. No output was silently applied.")
			return
		}
	}
	s.mu.RLock()
	current, err = s.readDiscoveryDocument(p, request.DocumentID)
	s.mu.RUnlock()
	if err != nil {
		discoveryDocumentProblem(w, err)
		return
	}
	s.writeDiscoveryNativeResponse(w, p, current, request.AttemptID)
}

func (s *Server) validateDiscoveryNativeCurrent(ctx context.Context, p project.Project, receipt discoveryNativeReceipt) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	current, err := s.readDiscoveryDocument(p, receipt.Request.DocumentID)
	if err != nil {
		return err
	}
	private, err := s.readDiscoveryNative(p, receipt.Request.DocumentID, receipt.Request.AttemptID)
	if err != nil {
		return err
	}
	if private.CancelKeyDigest != "" || private.Request.AllocationID != receipt.Request.AllocationID || current.Document.Revision != receipt.Request.ExpectedDocumentRevision || current.Document.Original != receipt.Request.Original {
		return ErrProjectRunConflict
	}
	for _, attempt := range current.Document.Attempts {
		if attempt.ID == receipt.Request.AttemptID && attempt.Status == "extracting" && attempt.ExecutionProfile == backend.DocumentExtractionProfile {
			return nil
		}
	}
	return ErrProjectRunConflict
}

func (s *Server) persistDiscoveryNative(p project.Project, req DiscoveryExtractionRequest, out DiscoveryExtractionOutcome) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	receipt, err := s.readDiscoveryNative(p, req.DocumentID, req.AttemptID)
	if err != nil {
		return err
	}
	if receipt.Request.AllocationID != req.AllocationID || receipt.Request.Actor != req.Actor || receipt.Request.Original != req.Original {
		return ErrProjectRunConflict
	}
	if receipt.Outcome.CleanupConfirmed && !receipt.Outcome.ResourcesHeld && receipt.Outcome.Identity != (backend.DocumentExtractionIdentity{}) && reviewDigest(receipt.Outcome) != reviewDigest(out) {
		return ErrProjectRunConflict
	}
	receipt.Outcome = out
	return s.saveDiscoveryNative(p, receipt)
}

// Caller holds the existing document lock. Actual extracted pages enter the
// same versioned document/history only after its exact original CAS is intact.
func (s *Server) applyDiscoveryNativeOutcome(p project.Project, request DiscoveryExtractionRequest, actor ContextActor, key string, out DiscoveryExtractionOutcome) error {
	current, err := s.readDiscoveryDocument(p, request.DocumentID)
	if err != nil {
		return err
	}
	if current.Document.Revision != request.ExpectedDocumentRevision {
		return ErrProjectRunConflict
	}
	if out.State == "complete" {
		result, err := decodeContainedDiscoveryResult(request, out, len(current.Document.Extractions)+1, actor, time.Now())
		if err != nil {
			return err
		}
		if err = discoveryFinishAttempt(&current, current.Document.Revision, request.AttemptID, "native-result-"+reviewDigest(key), result, actor, time.Now()); err != nil {
			return err
		}
	} else {
		for i := range current.Document.Attempts {
			if current.Document.Attempts[i].ID == request.AttemptID && current.Document.Attempts[i].Status == "extracting" {
				current.Document.Attempts[i].Status = "interrupted"
				if out.State == "cancelled" {
					current.Document.Attempts[i].Status = "cancelled"
				}
				current.Document.Attempts[i].FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
				current.Document.Revision++
				break
			}
		}
	}
	return s.saveDiscoveryDocument(current)
}

func (s *Server) writeDiscoveryNativeResponse(w http.ResponseWriter, p project.Project, saved storedDiscoveryDocument, attemptID string) {
	s.mu.RLock()
	receipt, err := s.readDiscoveryNative(p, saved.Document.ID, attemptID)
	s.mu.RUnlock()
	if err != nil {
		problem(w, 409, "document_outcome_unconfirmed", "The exact private attempt receipt is unavailable; no restart was inferred.")
		return
	}
	// Identity, raw capture, engine, private path and actor lease never leave
	// the private store. The public document remains the only evidence history.
	writeJSON(w, 200, map[string]any{"document": discoveryDocumentView(saved.Document, time.Now()), "lifecycle": map[string]any{"attemptId": attemptID, "state": receipt.Outcome.State, "stage": receipt.Outcome.Stage, "reasonCode": receipt.Outcome.ReasonCode, "message": receipt.Outcome.Message, "resourcesHeld": receipt.Outcome.ResourcesHeld, "cleanupConfirmed": receipt.Outcome.CleanupConfirmed, "readerSettled": receipt.Outcome.ReaderSettled}})
}

func (s *Server) cancelDiscoveryExtraction(w http.ResponseWriter, r *http.Request) {
	if !discoveryDocumentAccess(w, r, true) {
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	s.mu.RLock()
	saved, err := s.readDiscoveryDocument(p, r.PathValue("documentId"))
	s.mu.RUnlock()
	if err != nil {
		discoveryDocumentProblem(w, err)
		return
	}
	var native bool
	for _, attempt := range saved.Document.Attempts {
		if attempt.ID == r.PathValue("attemptId") {
			native = attempt.ExecutionProfile == backend.DocumentExtractionProfile
			break
		}
	}
	if !native {
		s.cancelDiscoveryTextExtraction(w, r)
		return
	}
	var req struct {
		ExpectedRevision int    `json:"expectedRevision"`
		IdempotencyKey   string `json:"idempotencyKey"`
		Cancel           bool   `json:"cancel"`
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	if !req.Cancel || !validRunKey(req.IdempotencyKey) {
		discoveryDocumentProblem(w, errDiscoveryDocument)
		return
	}
	runtime := s.discoveryExtractionRuntime
	if runtime.AuthorizeControl == nil || runtime.Recover == nil || runtime.jobs == nil {
		problem(w, 422, "extractor_unavailable", "Exact native recovery is unavailable; ownership remains retained.")
		return
	}
	if err = runtime.AuthorizeControl(r.Context(), p.ID); err != nil {
		problem(w, 403, "document_control_forbidden", "Current project control authority is required to retire this extraction.")
		return
	}
	actor, _ := authenticatedContextActor(r.Context())
	attemptID := r.PathValue("attemptId")
	cancelDigest := reviewDigest(struct {
		Attempt string
		Actor   string
	}{attemptID, actor.ID})
	s.mu.Lock()
	current, err := s.readDiscoveryDocument(p, saved.Document.ID)
	var receipt discoveryNativeReceipt
	if err == nil {
		receipt, err = s.readDiscoveryNative(p, saved.Document.ID, attemptID)
	}
	if err == nil && receipt.CancelKeyDigest != "" && (receipt.CancelKeyDigest != reviewDigest(req.IdempotencyKey) || receipt.CancelRequestDigest != cancelDigest) {
		err = ErrProjectRunKeyConflict
	}
	_, cancelRecorded, readbackErr := discoveryRequestReadback(current, req.IdempotencyKey, cancelDigest, "cancel")
	if err == nil && readbackErr != nil {
		err = readbackErr
	}
	if err == nil && !cancelRecorded {
		if current.Document.Revision != req.ExpectedRevision {
			err = ErrProjectRunConflict
		} else {
			if receipt.CancelKeyDigest == "" {
				receipt.CancelKeyDigest, receipt.CancelRequestDigest = reviewDigest(req.IdempotencyKey), cancelDigest
				err = s.saveDiscoveryNative(p, receipt)
			}
			if err == nil {
				err = discoveryRememberRequest(&current, req.IdempotencyKey, cancelDigest, "cancel", attemptID, 0)
			}
			if err == nil {
				for i := range current.Document.Attempts {
					if current.Document.Attempts[i].ID == attemptID && current.Document.Attempts[i].Status == "extracting" {
						current.Document.Attempts[i].Status = "cancelled"
						current.Document.Attempts[i].FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
						current.Document.Revision++
						break
					}
				}
				err = s.saveDiscoveryDocument(current)
			}
		}
	}
	s.mu.Unlock()
	if err != nil {
		discoveryDocumentProblem(w, err)
		return
	}
	bounded, stop := context.WithTimeout(r.Context(), backend.DocumentCleanupBudget)
	defer stop()
	key := discoveryNativeJobKey(p.ID, saved.Document.ID, attemptID)
	runtime.jobs.mu.Lock()
	job := runtime.jobs.active[key]
	if job != nil {
		job.cancel()
	}
	runtime.jobs.mu.Unlock()
	if job != nil {
		select {
		case <-job.done:
		case <-bounded.Done():
			problem(w, 409, "document_cleanup_pending", "The exact reader/lifecycle has not settled; its capacity remains retained.")
			return
		}
	}
	s.mu.RLock()
	receipt, err = s.readDiscoveryNative(p, saved.Document.ID, attemptID)
	s.mu.RUnlock()
	if err != nil {
		discoveryDocumentProblem(w, err)
		return
	}
	if receipt.Outcome.ResourcesHeld || !receipt.Outcome.CleanupConfirmed {
		controlActor, err := management.CaptureExecutionControlActor(bounded, p.ID)
		if err != nil {
			discoveryDocumentProblem(w, err)
			return
		}
		_, err = runtime.Recover(bounded, DiscoveryExtractionRecoveryRequest{ProjectID: p.ID, DocumentID: saved.Document.ID, AttemptID: attemptID, AllocationID: receipt.Request.AllocationID, Actor: controlActor, Action: "retire", Outcome: receipt.Outcome}, func(out DiscoveryExtractionOutcome) error { return s.persistDiscoveryNative(p, receipt.Request, out) })
		if err != nil {
			problem(w, 409, "document_cleanup_unconfirmed", "Exact retirement is unconfirmed; resources remain held and no replacement was started.")
			return
		}
	}
	s.mu.Lock()
	receipt, err = s.readDiscoveryNative(p, saved.Document.ID, attemptID)
	if err == nil {
		err = s.retireDiscoveryNativeOriginal(p, receipt)
	}
	current, readErr := s.readDiscoveryDocument(p, saved.Document.ID)
	s.mu.Unlock()
	if err != nil || readErr != nil {
		discoveryDocumentProblem(w, errors.Join(err, readErr))
		return
	}
	s.writeDiscoveryNativeResponse(w, p, current, attemptID)
}
