package product

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/previewruntime"
)

func (s *Server) SetPreviewRuntime(value previewruntime.Runtime) { s.previewRuntime = value }
func (s *Server) PreviewRuntime() previewruntime.Runtime         { return s.previewRuntime }

// PreviewSourceDigest reads the original private admission provenance. A tar
// checksum is not interchangeable with the source digest used by check evidence.
func (s *Server) PreviewSourceDigest(ctx context.Context, projectID, releaseID, commit string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, err := s.readReleaseRunProvenance(projectID, releaseID)
	if err != nil {
		return "", err
	}
	if v.Commit != commit || !hexDigest(v.SourceDigest) {
		return "", errors.New("release source provenance changed")
	}
	return v.SourceDigest, nil
}

func (s *Server) storeAdmittedRelease(before storedRelease, after *storedRelease) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if before.operation != nil && (s.localPreviewOperations[before.ID] != before.operation || before.operation.sealed) {
		return errors.New("the preview publisher is no longer owned by this controller")
	}
	rows, err := s.readReleases(before.ProjectID)
	if err != nil {
		return err
	}
	for i, current := range rows {
		if current.ID != before.ID {
			continue
		}
		if current.Kind != before.Kind || after.Kind != before.Kind || current.Commit != before.Commit || current.ConfigurationRevision != before.ConfigurationRevision || current.Runtime == nil || before.Runtime == nil || after.Runtime == nil || current.Runtime.Kind != current.Kind || before.Runtime.Kind != before.Kind || after.Runtime.Kind != after.Kind || current.Runtime.Identity != before.Runtime.Identity {
			return errors.New("the exact release receipt has a newer outcome")
		}
		if current.Revision != before.Revision {
			// Cancel is the only concurrent writer whose intent the live producer
			// may merge. Never absorb a newer runtime outcome or changed source.
			expected := cancellationReceipt(before)
			expected.operation = nil
			if before.operation == nil || !before.operation.cancelRequested || !reflect.DeepEqual(current, expected) {
				return errors.New("the exact release receipt has a newer outcome")
			}
		}
		after.CancelRequested, after.CancelRevision = current.CancelRequested, current.CancelRevision
		if before.operation != nil && before.operation.cancelRequested {
			after.CancelRequested = true
		}
		if after.CancelRequested {
			switch after.Status {
			case "building":
				after.Status, after.Message = "cancelling", previewCancellationMessage
			case "ready", "failed", "unknown":
				after.Status, after.URL, after.Message = "unknown", "", "Cancellation was requested. The preview outcome or cleanup is unconfirmed; its exact receipt and any reservation are retained. No replacement was started."
				if after.Runtime != nil && !after.Runtime.ResourcesHeld && (after.Runtime.CleanupConfirmed || !after.Runtime.Retained()) {
					after.Status, after.Message = "cancelled", "Preview preparation was cancelled with no remaining acknowledged resource demand. The release history is retained."
				}
			}
		}
		after.Revision = current.Revision + 1
		rows[i] = *after
		if err := atomicJSON(s.releaseFile(before.ProjectID), rows); err != nil {
			return err
		}
		if before.operation != nil {
			before.operation.identity = after.Runtime.Identity
		}
		return nil
	}
	return errors.New("the exact release receipt is unavailable")
}

func (s *Server) publishPreviewOutcome(row *storedRelease, out previewruntime.Outcome, environment map[string]string) error {
	if row.Runtime == nil || row.Kind != out.Kind || row.Runtime.Kind != row.Kind || !previewruntime.Continues(row.Runtime.Identity, out.Identity) {
		return errors.New("preview outcome does not match its exact source and allocation")
	}
	next := *row
	copyOutcome := out
	copyOutcome.Message = redactPreviewConfiguration(out.Message, environment)
	next.Runtime = &copyOutcome
	next.ContainerID, next.Image = out.ContainerID, out.Image
	next.URL, next.Message = "", copyOutcome.Message
	next.Status = "unknown"
	switch out.Phase {
	case "reserved", "building", "built", "creating", "created", "starting", "running":
		next.Status = "building"
	case "finished":
		if out.CleanupConfirmed && !out.ResourcesHeld {
			next.Status = "stopped"
		}
	}
	if err := s.storeAdmittedRelease(*row, &next); err != nil {
		return err
	}
	*row = next
	return nil
}

func (s *Server) stopAdmittedRelease(w http.ResponseWriter, r *http.Request, row storedRelease) {
	if row.Runtime.Phase == "finished" && row.Runtime.CleanupConfirmed && !row.Runtime.ResourcesHeld {
		journeyJSON(w, 200, s.browserRelease(row))
		return
	}
	runtime := s.PreviewRuntime()
	if runtime.Stop == nil {
		journeyError(w, 503, "preview_runtime_unavailable", "The shared preview runtime is unavailable; its reservation is retained.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	out, err := runtime.Stop(ctx, row.Runtime.Identity, func(value previewruntime.Outcome) error { return s.publishPreviewOutcome(&row, value, nil) })
	if err == nil {
		err = s.publishPreviewOutcome(&row, out, nil)
	}
	if err != nil || row.Runtime.ResourcesHeld || !row.Runtime.CleanupConfirmed {
		journeyError(w, 503, "preview_cleanup_unconfirmed", "The exact preview cleanup is unconfirmed. Its history and reservation are retained; refresh or retry Stop.")
		return
	}
	journeyJSON(w, 200, s.browserRelease(row))
}
