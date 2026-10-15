package api

import (
	"errors"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
)

// These private records join the existing shared host ledger under its current
// lock/publication. They must never be mapped into Checks or registry sessions.
type documentExtractionLease struct {
	ID              string                             `json:"id"`
	Scope           string                             `json:"scope"`
	Actor           management.ExecutionActorRef       `json:"actor"`
	Identity        backend.DocumentExtractionIdentity `json:"identity"`
	ImageID         string                             `json:"imageId"`
	Resources       backend.Resources                  `json:"resources"`
	HostLaunchBytes uint64                             `json:"hostLaunchBytes,omitempty"`
	CreateDiskBytes uint64                             `json:"createDiskBytes"`
	State           string                             `json:"state"`
	Ref             string                             `json:"ref,omitempty"`
	ResultDigest    string                             `json:"resultDigest,omitempty"`
	OutcomeDigest   string                             `json:"outcomeDigest,omitempty"`
	CreatedAt       time.Time                          `json:"createdAt"`
	UpdatedAt       time.Time                          `json:"updatedAt"`
	ComputeDeadline time.Time                          `json:"computeDeadline"`
	WholeDeadline   time.Time                          `json:"wholeDeadline"`
}
type documentExtractionFenceRecord struct {
	Scope               string                               `json:"scope"`
	Identity            backend.DocumentExtractionIdentity   `json:"identity"`
	Actor               management.ExecutionActorRef         `json:"actor"`
	IntentID            string                               `json:"intentId"`
	ExpectedOriginalRef string                               `json:"expectedOriginalRef,omitempty"`
	State               string                               `json:"state"`
	Proof               backend.DocumentExtractionFenceProof `json:"proof,omitzero"`
	CreatedAt           time.Time                            `json:"createdAt"`
	UpdatedAt           time.Time                            `json:"updatedAt"`
}

func validDocumentExtractionLease(value documentExtractionLease) bool {
	if value.State != "finished" && value.OutcomeDigest != "" || value.ResultDigest != "" && !checkHex(value.ResultDigest, 32) {
		return false
	}
	if value.Identity.ActorDigest != backend.DocumentExtractionActorDigest(value.Actor.UserID, value.Actor.TeamID, value.Actor.ProjectID, value.Actor.Local) {
		return false
	}
	if value.ID != value.Identity.AllocationID || !backend.ValidateDocumentExtractionIdentity(value.Identity, true) || value.Scope != value.Identity.ScopeDigest || value.Actor.ProjectID != value.Identity.ProjectID || value.Actor.Local && (value.Actor.UserID != "" || value.Actor.TeamID != "") || !value.Actor.Local && value.Actor.UserID == "" || len(value.Actor.UserID) > 200 || len(value.Actor.TeamID) > 200 || value.Resources != backend.DocumentExtractionResources() || len(value.ImageID) != 71 || value.ImageID[:7] != "sha256:" || !checkHex(value.ImageID[7:], 32) || value.CreateDiskBytes == 0 || value.CreateDiskBytes > 1<<50 || value.HostLaunchBytes > 4<<30 || value.CreatedAt.IsZero() || value.UpdatedAt.Before(value.CreatedAt) || value.ComputeDeadline.Sub(value.CreatedAt) != backend.DocumentComputeBudget || value.WholeDeadline.Sub(value.ComputeDeadline) != backend.DocumentCleanupBudget {
		return false
	}
	// Expiration is deliberately absent from this state machine. A timeout is
	// not proof of absence and cannot release capacity or permit replacement.
	switch value.State {
	case "reserved", "creating":
		return value.Ref == "" && value.ResultDigest == ""
	case "created", "starting", "running", "cleaning":
		return checkHex(value.Ref, 32) && (value.ResultDigest == "" || checkHex(value.ResultDigest, 32))
	case "fencing":
		return (value.Ref == "" || checkHex(value.Ref, 32)) && (value.ResultDigest == "" || checkHex(value.ResultDigest, 32))
	case "finished":
		return (value.Ref == "" || checkHex(value.Ref, 32)) && checkHex(value.OutcomeDigest, 32)
	default:
		return false
	}
}

// Shared ledger readers/writers call this alongside existing lease/fence
// validators. History and permanent marker bounds remain joint host bounds.
func validateDocumentExtractionRecords(documents []documentExtractionLease, fences []documentExtractionFenceRecord) error {
	seen := map[string]documentExtractionLease{}
	for _, lease := range documents {
		key := lease.Scope + ":" + lease.ID
		if _, duplicate := seen[key]; duplicate || !validDocumentExtractionLease(lease) {
			return errors.New("document extraction capacity ownership is invalid")
		}
		seen[key] = lease
	}
	fenced := map[string]bool{}
	for _, record := range fences {
		identity := record.Identity
		key := record.Scope + ":" + identity.AllocationID
		if !backend.ValidateDocumentExtractionIdentity(identity, true) || record.Scope != identity.ScopeDigest || record.Actor.ProjectID != identity.ProjectID || identity.ActorDigest != backend.DocumentExtractionActorDigest(record.Actor.UserID, record.Actor.TeamID, record.Actor.ProjectID, record.Actor.Local) || !checkHex(record.IntentID, 16) || record.ExpectedOriginalRef != "" && !checkHex(record.ExpectedOriginalRef, 32) || fenced[key] || record.CreatedAt.IsZero() || record.UpdatedAt.Before(record.CreatedAt) {
			return errors.New("document extraction fence ownership is invalid")
		}
		fenced[key] = true
		switch record.State {
		case "intent":
			if record.Proof != (backend.DocumentExtractionFenceProof{}) {
				return errors.New("unacknowledged document fence has a proof")
			}
		case "proved", "retired":
			proof := record.Proof
			ack, e1 := time.Parse(time.RFC3339Nano, proof.AcknowledgedAt)
			observed, e2 := time.Parse(time.RFC3339Nano, proof.ObservedAt)
			if proof.Identity != identity || proof.IntentID != record.IntentID || !checkHex(proof.FenceRef, 32) || !checkHex(proof.ConfigDigest, 32) || e1 != nil || e2 != nil || ack.Before(record.CreatedAt) || observed.Before(ack) || observed.After(time.Now().UTC()) {
				return errors.New("document extraction fence proof is invalid")
			}
		default:
			return errors.New("document extraction fence state is invalid")
		}
		lease, found := seen[key]
		if !found && record.State != "retired" || found && (lease.Identity != identity || lease.Actor != record.Actor || record.State == "retired" && lease.State != "finished" || record.State != "retired" && lease.State != "fencing") {
			return errors.New("document extraction fence lost its exact capacity claim")
		}
	}
	for key, lease := range seen {
		if lease.State == "fencing" && !fenced[key] {
			return errors.New("document extraction has no durable fence intent")
		}
	}
	return nil
}
