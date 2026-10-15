package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
)

func documentOutcomeDigest(out product.DiscoveryExtractionOutcome) string {
	out.ResourcesHeld = false
	data, err := json.Marshal(out)
	if err != nil {
		return ""
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}
func (g *executionAdmission) documentLease(ctx context.Context, projectID, documentID, attemptID string) (documentExtractionLease, error) {
	ledger, err := g.readReservationLedger(ctx)
	if err != nil {
		return documentExtractionLease{}, err
	}
	var result documentExtractionLease
	for _, lease := range ledger.Documents {
		if lease.Scope == g.scope && lease.Identity.ProjectID == projectID && lease.Identity.DocumentID == documentID && lease.Identity.AttemptID == attemptID {
			if result.ID != "" {
				return result, errors.New("duplicate exact document ownership")
			}
			result = lease
		}
	}
	if result.ID == "" {
		return result, errors.New("exact document ownership unavailable")
	}
	return result, nil
}
func (g *executionAdmission) requireReservedDocument(ctx context.Context, identity backend.DocumentExtractionIdentity, actor management.ExecutionActorRef) error {
	lease, err := g.documentLease(ctx, identity.ProjectID, identity.DocumentID, identity.AttemptID)
	if err != nil {
		return err
	}
	if lease.Identity != identity || lease.Actor != actor || lease.State != "reserved" || lease.Ref != "" {
		return errors.New("the exact document claim was already changed or settled")
	}
	return nil
}
func (g *executionAdmission) transitionDocument(ctx context.Context, identity backend.DocumentExtractionIdentity, from, to, ref string) error {
	return g.mutate(ctx, func(ledger *executionLeaseLedger) error {
		for index := range ledger.Documents {
			lease := &ledger.Documents[index]
			if lease.Scope != g.scope || lease.ID != identity.AllocationID {
				continue
			}
			if lease.Identity != identity || lease.State != from || lease.Ref != "" && lease.Ref != ref {
				return errors.New("document launch phase or immutable ownership changed")
			}
			allowed := from == "reserved" && to == "creating" || from == "creating" && to == "created" || from == "created" && to == "starting" || from == "starting" && to == "running"
			if !allowed {
				return errors.New("document launch phase transition refused")
			}
			lease.State, lease.Ref, lease.UpdatedAt = to, ref, time.Now().UTC()
			if !validDocumentExtractionLease(*lease) {
				return errors.New("invalid document launch publication")
			}
			return nil
		}
		return errors.New("document claim is absent or retired")
	})
}
func (g *executionAdmission) markDocumentCleaning(ctx context.Context, identity backend.DocumentExtractionIdentity, ref, resultDigest string) error {
	return g.mutate(ctx, func(ledger *executionLeaseLedger) error {
		for index := range ledger.Documents {
			lease := &ledger.Documents[index]
			if lease.Scope != g.scope || lease.ID != identity.AllocationID {
				continue
			}
			if lease.Identity != identity || lease.Ref != ref || !checkHex(ref, 32) || lease.ResultDigest != "" && lease.ResultDigest != resultDigest || resultDigest != "" && !checkHex(resultDigest, 32) {
				return errors.New("document cleanup/capture ownership changed")
			}
			if lease.State != "created" && lease.State != "starting" && lease.State != "running" && lease.State != "cleaning" {
				return errors.New("unacknowledged document Create requires fencing")
			}
			lease.State, lease.ResultDigest, lease.UpdatedAt = "cleaning", resultDigest, time.Now().UTC()
			return nil
		}
		return errors.New("document cleanup claim unavailable")
	})
}
func (g *executionAdmission) completeDocument(ctx context.Context, identity backend.DocumentExtractionIdentity, ref string, out product.DiscoveryExtractionOutcome, absence backend.DocumentExtractionObservation) error {
	if out.Identity != identity || out.Ref != ref || !out.CleanupConfirmed || !out.ReaderSettled {
		return errors.New("document outcome has no settled reader/cleanup proof")
	}
	return g.mutate(ctx, func(ledger *executionLeaseLedger) error {
		for index := range ledger.Documents {
			lease := &ledger.Documents[index]
			if lease.Scope != g.scope || lease.ID != identity.AllocationID {
				continue
			}
			if lease.Identity != identity || lease.Ref != ref || !(lease.State == "reserved" && ref == "" || lease.State == "cleaning" && checkHex(ref, 32) && absence.Ref == ref && absence.State == "absent") {
				return errors.New("document allocation or cleanup remains unconfirmed")
			}
			lease.State, lease.OutcomeDigest, lease.UpdatedAt = "finished", documentOutcomeDigest(out), time.Now().UTC()
			if !validDocumentExtractionLease(*lease) {
				return errors.New("document completion publication invalid")
			}
			return nil
		}
		return errors.New("document completion claim unavailable")
	})
}
