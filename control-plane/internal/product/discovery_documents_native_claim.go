package product

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

type discoveryNativeClaimWriters struct {
	private  func(project.Project, discoveryNativeReceipt) error
	document func(storedDiscoveryDocument) error
}

// Called under s.mu, before the runtime job is registered or Execute is called.
// A failure therefore owns only this positively unallocated staged original and
// initial receipt. It cannot remove anything that may have reached admission.
func (s *Server) publishDiscoveryNativeClaim(p project.Project, current storedDiscoveryDocument, receipt discoveryNativeReceipt, writers discoveryNativeClaimWriters) error {
	if receipt.Outcome.State != "claimed" || receipt.Outcome.Identity != (backend.DocumentExtractionIdentity{}) || receipt.Outcome.ResourcesHeld || receipt.Outcome.Ref != "" || current.Document.ID != receipt.Request.DocumentID {
		return errDiscoveryDocument
	}
	err := writers.private(p, receipt)
	if err == nil {
		err = writers.document(current)
	}
	if err == nil {
		return nil
	}
	return errors.Join(err, s.rollbackUnallocatedDiscoveryNativeClaim(p, receipt))
}

func (s *Server) rollbackUnallocatedDiscoveryNativeClaim(p project.Project, receipt discoveryNativeReceipt) error {
	if receipt.Outcome.State != "claimed" || receipt.Outcome.Identity != (backend.DocumentExtractionIdentity{}) || receipt.Outcome.ResourcesHeld || receipt.Outcome.Ref != "" {
		return errDiscoveryDocument
	}
	stored, privateErr := s.readDiscoveryNative(p, receipt.Request.DocumentID, receipt.Request.AttemptID)
	if privateErr != nil && !errors.Is(privateErr, os.ErrNotExist) {
		return privateErr
	}
	if privateErr == nil && (stored.Request.AllocationID != receipt.Request.AllocationID || stored.Request.OriginalPath != receipt.Request.OriginalPath || stored.Request.Original != receipt.Request.Original || stored.Outcome.State != "claimed" || stored.Outcome.Identity != (backend.DocumentExtractionIdentity{}) || stored.Outcome.ResourcesHeld || stored.Outcome.Ref != "") {
		return ErrProjectRunConflict
	}
	current, err := s.readDiscoveryDocument(p, receipt.Request.DocumentID)
	if err != nil {
		return err
	}
	published := false
	for i, attempt := range current.Document.Attempts {
		if attempt.ID != receipt.Request.AttemptID {
			continue
		}
		if attempt.ExecutionProfile != backend.DocumentExtractionProfile {
			return ErrProjectRunConflict
		}
		published = true
		// A lost save ACK can have published the exact public attempt. Preserve
		// its history as interrupted; never erase a possibly published revision.
		if attempt.Status == "extracting" {
			current.Document.Attempts[i].Status = "interrupted"
			current.Document.Attempts[i].FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
			current.Document.Revision++
			if err = s.saveDiscoveryDocument(current); err != nil {
				return err
			} // leave recoverable private claim
		}
		break
	}
	closed := receipt
	closed.Outcome = DiscoveryExtractionOutcome{State: "blocked", Stage: "claim-publication", ReasonCode: "document_claim_not_started", Message: "The pre-execution publication failed. No parser was admitted or started; the exact stage is retired.", ReaderSettled: true, CleanupConfirmed: true}
	// Only the exact original copy is retired. The immutable intake original,
	// other attempts, public extractions and any allocated ownership survive.
	if err = s.retireDiscoveryNativeOriginal(p, closed); err != nil {
		return err
	}
	if published {
		return s.saveDiscoveryNative(p, closed)
	}
	path, err := s.discoveryNativePath(p.ID, receipt.Request.DocumentID, receipt.Request.AttemptID, ".json", false)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	stored, err = s.readDiscoveryNative(p, receipt.Request.DocumentID, receipt.Request.AttemptID)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if stored.Request.AllocationID != receipt.Request.AllocationID || stored.Request.OriginalPath != receipt.Request.OriginalPath || stored.Request.Original != receipt.Request.Original || stored.Outcome.State != "claimed" || stored.Outcome.Identity != (backend.DocumentExtractionIdentity{}) || stored.Outcome.ResourcesHeld || stored.Outcome.Ref != "" {
		return ErrProjectRunConflict
	}
	if err = os.Remove(path); err != nil {
		return err
	}
	return syncProjectRunDirectory(filepath.Dir(path))
}
