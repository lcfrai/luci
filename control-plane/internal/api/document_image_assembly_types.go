package api

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
)

// Additive shared-ledger types, not inserted into API Deps/New or any route.
// Expiration never settles ownership or removes demand.
type documentImageAssemblyLease struct {
	ID               string                                `json:"id"`
	Scope            string                                `json:"scope"`
	Actor            management.ExecutionActorRef          `json:"actor"`
	Identity         backend.DocumentImageAssemblyIdentity `json:"identity"`
	Resources        backend.Resources                     `json:"resources"`
	BaseImageID      string                                `json:"baseImageId"`
	CreateDiskBytes  uint64                                `json:"createDiskBytes"`
	CaptureDiskBytes uint64                                `json:"captureDiskBytes"`
	HostLaunchBytes  uint64                                `json:"hostLaunchBytes"`
	State            string                                `json:"state"`
	Ref              string                                `json:"ref,omitempty"`
	Capture          backend.DocumentImageAssemblyCapture  `json:"capture,omitzero"`
	OutcomeDigest    string                                `json:"outcomeDigest,omitempty"`
	CreatedAt        time.Time                             `json:"createdAt"`
	UpdatedAt        time.Time                             `json:"updatedAt"`
	ComputeDeadline  time.Time                             `json:"computeDeadline"`
	WholeDeadline    time.Time                             `json:"wholeDeadline"`
}
type documentImageAssemblyFenceRecord struct {
	Scope       string                                  `json:"scope"`
	Identity    backend.DocumentImageAssemblyIdentity   `json:"identity"`
	Actor       management.ExecutionActorRef            `json:"actor"`
	IntentID    string                                  `json:"intentId"`
	ExpectedRef string                                  `json:"expectedRef,omitempty"`
	State       string                                  `json:"state"`
	Proof       backend.DocumentImageAssemblyFenceProof `json:"proof,omitzero"`
	CreatedAt   time.Time                               `json:"createdAt"`
	UpdatedAt   time.Time                               `json:"updatedAt"`
}

type documentImageAssemblyRequest struct {
	Spec                                      backend.DocumentImageAssemblySpec
	Actor                                     management.ExecutionActorRef
	CreatedAt, ComputeDeadline, WholeDeadline time.Time
	// Server-owned current installation grant/source/CAS, not request code.
	ValidateCurrent func(context.Context) error
}
type documentImageAssemblyOutcome struct {
	Identity         backend.DocumentImageAssemblyIdentity `json:"identity"`
	Ref              string                                `json:"ref,omitempty"`
	Stage            string                                `json:"stage"`
	State            string                                `json:"state"`
	Capture          backend.DocumentImageAssemblyCapture  `json:"capture,omitzero"`
	ExitCode         *int                                  `json:"exitCode,omitempty"`
	OOMKilled        bool                                  `json:"oomKilled"`
	ReaderSettled    bool                                  `json:"readerSettled"`
	CleanupConfirmed bool                                  `json:"cleanupConfirmed"`
	ResourcesHeld    bool                                  `json:"resourcesHeld"`
	ReasonCode       string                                `json:"reasonCode,omitempty"`
}

// Cloud's allocated shared implementation must adapt the existing ledger lock,
// current-grant checks, resource observation and lifecycle lock to these hooks.
// Nil/partial hooks remain unsupported; no standalone replacement ledger exists.
type documentImageAssemblyHooks struct {
	Authorize   func(context.Context, management.ExecutionActorRef, backend.DocumentImageAssemblyIdentity) error
	Admit       func(context.Context, backend.PreparedDocumentImageAssembly, documentImageAssemblyRequest) (documentImageAssemblyLease, error)
	Lock        func(context.Context, backend.DocumentImageAssemblyIdentity) (func(), error)
	Transition  func(context.Context, backend.DocumentImageAssemblyIdentity, string, string, string) error
	HoldFence   func(context.Context, backend.DocumentImageAssemblyFenceRequest) error
	RecordFence func(context.Context, backend.DocumentImageAssemblyFenceRequest, backend.DocumentImageAssemblyFenceProof) error
	Finish      func(context.Context, backend.DocumentImageAssemblyIdentity, documentImageAssemblyOutcome) error
	Headroom    func(context.Context, string, uint64) error
	// A positively allocated exclusive, bounded private artifact writer. Failure
	// or partial bytes never become an OCI/import receipt. Close cannot gate stop.
	CaptureSink func(context.Context, backend.DocumentImageAssemblyIdentity) (io.WriteCloser, error)
	Publish     func(context.Context, documentImageAssemblyOutcome) error
}

func validDocumentImageAssemblyLease(value documentImageAssemblyLease) bool {
	identity := value.Identity
	if !backend.ValidateDocumentImageAssemblyIdentity(identity, true) || value.ID != identity.AllocationID || value.Scope != identity.ScopeDigest || value.Actor.ProjectID != identity.ProjectID || identity.ActorDigest != backend.DocumentExtractionActorDigest(value.Actor.UserID, value.Actor.TeamID, value.Actor.ProjectID, value.Actor.Local) || value.Actor.Local && (value.Actor.UserID != "" || value.Actor.TeamID != "") || !value.Actor.Local && value.Actor.UserID == "" || len(value.Actor.UserID) > 200 || len(value.Actor.TeamID) > 200 || value.Resources != backend.DocumentImageAssemblyResources() || value.BaseImageID != backend.DocumentImageAssemblyBaseID || value.CreateDiskBytes == 0 || value.CreateDiskBytes > 1<<50 || value.CaptureDiskBytes != backend.DocumentImageAssemblyOutputMaxBytes || value.HostLaunchBytes > 4<<30 || value.CreatedAt.IsZero() || value.UpdatedAt.Before(value.CreatedAt) || value.ComputeDeadline.Sub(value.CreatedAt) != backend.DocumentImageAssemblyComputeBudget || value.WholeDeadline.Sub(value.ComputeDeadline) != backend.DocumentImageAssemblyCleanupBudget {
		return false
	}
	if value.Capture != (backend.DocumentImageAssemblyCapture{}) && (!checkHex(value.Capture.Digest, 32) || value.Capture.Bytes < 1 || value.Capture.Bytes > backend.DocumentImageAssemblyOutputMaxBytes) {
		return false
	}
	if value.State != "finished" && value.OutcomeDigest != "" {
		return false
	}
	switch value.State {
	case "reserved", "creating":
		return value.Ref == "" && value.Capture == (backend.DocumentImageAssemblyCapture{})
	case "created", "starting", "running", "cleaning":
		return checkHex(value.Ref, 32)
	case "fencing":
		return value.Ref == "" || checkHex(value.Ref, 32)
	case "finished":
		return (value.Ref == "" || checkHex(value.Ref, 32)) && checkHex(value.OutcomeDigest, 32)
	default:
		return false
	}
}
func validateDocumentImageAssemblyRecords(values []documentImageAssemblyLease, fences []documentImageAssemblyFenceRecord) error {
	if len(values) > 4096 || len(fences) > 4096 {
		return errors.New("assembly ownership history bound")
	}
	seen := map[string]documentImageAssemblyLease{}
	for _, value := range values {
		key := value.Scope + ":" + value.ID
		if _, duplicate := seen[key]; duplicate || !validDocumentImageAssemblyLease(value) {
			return errors.New("assembly lease ownership invalid")
		}
		seen[key] = value
	}
	marked := map[string]bool{}
	for _, fence := range fences {
		key := fence.Scope + ":" + fence.Identity.AllocationID
		if marked[key] || !backend.ValidateDocumentImageAssemblyIdentity(fence.Identity, true) || fence.Scope != fence.Identity.ScopeDigest || fence.Actor.ProjectID != fence.Identity.ProjectID || fence.Identity.ActorDigest != backend.DocumentExtractionActorDigest(fence.Actor.UserID, fence.Actor.TeamID, fence.Actor.ProjectID, fence.Actor.Local) || !checkHex(fence.IntentID, 16) || fence.ExpectedRef != "" && !checkHex(fence.ExpectedRef, 32) || fence.CreatedAt.IsZero() || fence.UpdatedAt.Before(fence.CreatedAt) {
			return errors.New("assembly fence ownership invalid")
		}
		marked[key] = true
		switch fence.State {
		case "intent":
			if fence.Proof != (backend.DocumentImageAssemblyFenceProof{}) {
				return errors.New("unacknowledged assembly fence proof")
			}
		case "proved", "retired":
			proof := fence.Proof
			ack, e1 := time.Parse(time.RFC3339Nano, proof.AcknowledgedAt)
			observed, e2 := time.Parse(time.RFC3339Nano, proof.ObservedAt)
			if proof.Identity != fence.Identity || proof.IntentID != fence.IntentID || !checkHex(proof.FenceRef, 32) || !checkHex(proof.ConfigDigest, 32) || e1 != nil || e2 != nil || ack.Before(fence.CreatedAt) || observed.Before(ack) || observed.After(time.Now().UTC()) {
				return errors.New("assembly fence ACK invalid")
			}
		default:
			return errors.New("assembly fence state invalid")
		}
		lease, found := seen[key]
		if !found && fence.State != "retired" || found && (lease.Identity != fence.Identity || lease.Actor != fence.Actor || fence.State == "retired" && lease.State != "finished" || fence.State != "retired" && lease.State != "fencing") {
			return errors.New("assembly fence lost capacity claim")
		}
	}
	for key, lease := range seen {
		if lease.State == "fencing" && !marked[key] {
			return errors.New("assembly fencing intent absent")
		}
	}
	return nil
}
