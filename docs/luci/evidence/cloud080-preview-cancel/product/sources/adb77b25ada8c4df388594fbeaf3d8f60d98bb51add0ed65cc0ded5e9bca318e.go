package product

import (
	"context"
	"errors"
	"net/http"

	"github.com/lcfrai/luci/control-plane/internal/previewruntime"
)

const previewCancellationMessage = "Cancellation requested. Waiting for the owned preview operation to report its outcome; this does not confirm Docker cleanup or release capacity."

// One entry belongs to one request and one immutable source. It is never
// restored from disk: a restarted controller cannot cancel its predecessor's
// unknown Docker work by inventing a new context or replaying the operation.
type localPreviewOperation struct {
	identity        previewruntime.Identity
	cancel          context.CancelFunc
	done            chan struct{}
	cancelRequested bool
	sealed          bool
}

func (operation *localPreviewOperation) matches(row storedRelease) bool {
	return row.Runtime != nil && operation.identity.ProjectID == row.ProjectID && operation.identity.ReleaseID == row.ID && operation.identity == row.Runtime.Identity
}

func (s *Server) beginLocalPreview(row *storedRelease, cancel context.CancelFunc) (*localPreviewOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.localPreviewsClosed || row.Runtime == nil || s.localPreviewOperations[row.ID] != nil {
		return nil, errors.New("Luci is closing or this preview already has an owned operation; no preview was started")
	}
	operation := &localPreviewOperation{identity: row.Runtime.Identity, cancel: cancel, done: make(chan struct{})}
	if err := s.storeReleaseLocked(*row); err != nil {
		return nil, err
	}
	if s.localPreviewOperations == nil {
		s.localPreviewOperations = make(map[string]*localPreviewOperation)
	}
	if s.activeLocalReleases == nil {
		s.activeLocalReleases = make(map[string]bool)
	}
	s.localPreviewOperations[row.ID], s.activeLocalReleases[row.ID] = operation, true
	row.operation = operation
	return operation, nil
}

func (s *Server) finishLocalPreview(id string, operation *localPreviewOperation) {
	s.mu.Lock()
	defer s.mu.Unlock()
	operation.sealed = true
	if s.localPreviewOperations[id] == operation {
		delete(s.localPreviewOperations, id)
		delete(s.activeLocalReleases, id)
	}
	close(operation.done)
}

func cancellationReceipt(row storedRelease) storedRelease {
	row.CancelRevision = row.Revision
	row.Revision++
	row.CancelRequested, row.CanCancel = true, false
	row.Status, row.Message = "cancelling", previewCancellationMessage
	return row
}

func (s *Server) cancelLocalRelease(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ExpectedRevision uint64 `json:"expectedRevision"`
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	if req.ExpectedRevision == 0 {
		journeyError(w, 400, "release_revision_required", "Refresh the exact preview receipt before requesting cancellation.")
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	runtime := s.PreviewRuntime()
	if runtime.CaptureActor == nil {
		journeyError(w, 503, "preview_runtime_unavailable", "Current deployment authority cannot be verified.")
		return
	}
	// Cancellation requires current Edit/Deploy, including idempotent requests;
	// it does not require permission to allocate new work or agent Execute.
	if _, err := runtime.CaptureActor(r.Context(), p.ID); err != nil {
		journeyError(w, 403, "deployment_forbidden", err.Error())
		return
	}
	s.mu.Lock()
	if err := r.Context().Err(); err != nil {
		s.mu.Unlock()
		journeyError(w, 403, "deployment_forbidden", "The current deployment control request is no longer active.")
		return
	}
	rows, err := s.readReleases(p.ID)
	if err != nil {
		s.mu.Unlock()
		journeyError(w, 500, "releases_unavailable", err.Error())
		return
	}
	for _, row := range rows {
		if row.ID != r.PathValue("releaseId") {
			continue
		}
		operation := s.localPreviewOperations[row.ID]
		if s.localPreviewsClosed || row.Target != "local" || operation == nil || operation.sealed || !operation.matches(row) {
			s.mu.Unlock()
			journeyError(w, 409, "preview_not_live", "This controller has no live operation for that exact preview. Its retained outcome is unchanged; cancellation cannot settle unknown Docker work.")
			return
		}
		if row.CancelRequested && operation.cancelRequested && row.CancelRevision == req.ExpectedRevision {
			s.mu.Unlock()
			journeyJSON(w, 200, s.browserRelease(row))
			return
		}
		if row.Revision != req.ExpectedRevision || row.CancelRequested || row.Status != "building" {
			s.mu.Unlock()
			journeyError(w, 409, "release_record_changed", "The exact preview receipt changed. Refresh its current outcome before another action.")
			return
		}
		next := cancellationReceipt(row)
		if err := s.storeReleaseLocked(next); err != nil {
			s.mu.Unlock()
			journeyError(w, 500, "release_record_failed", "Cancellation intent could not be saved; the live operation was not cancelled.")
			return
		}
		operation.cancelRequested = true
		operation.cancel()
		s.mu.Unlock()
		journeyJSON(w, 202, s.browserRelease(next))
		return
	}
	s.mu.Unlock()
	journeyError(w, 404, "release_not_found", "The selected release is not in this project's history.")
}

// One deadline bounds all joins. Cancellation is not settlement; after the
// deadline every remaining publisher is sealed and the durable receipt stays
// unknown. The shared ledger is never rewritten or released here.
func (s *Server) closeLocalPreviews(ctx context.Context) {
	s.mu.Lock()
	s.localPreviewsClosed = true
	operations := make(map[string]*localPreviewOperation, len(s.localPreviewOperations))
	for id, operation := range s.localPreviewOperations {
		operations[id] = operation
		rows, err := s.readReleases(operation.identity.ProjectID)
		if err == nil {
			for _, row := range rows {
				if row.ID == id && operation.matches(row) && !row.CancelRequested {
					_ = s.storeReleaseLocked(cancellationReceipt(row))
					break
				}
			}
		}
		operation.cancelRequested = true
		operation.cancel()
	}
	s.mu.Unlock()
	for _, operation := range operations {
		select {
		case <-operation.done:
		case <-ctx.Done():
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, operation := range operations {
		if s.localPreviewOperations[id] != operation {
			continue
		}
		operation.sealed = true
		rows, err := s.readReleases(operation.identity.ProjectID)
		if err != nil {
			continue
		}
		for _, row := range rows {
			if row.ID == id && operation.matches(row) {
				row.Revision++
				row.CancelRequested = true
				row.Status, row.URL, row.Message = "unknown", "", "Luci closed before the owned preview reported settlement. Its receipt and any reservation are retained; no replacement was started."
				_ = s.storeReleaseLocked(row)
				break
			}
		}
	}
}
