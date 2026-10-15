package product

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
)

// A product-derived private original and durable attempt, never browser argv.
// Both deadlines start with the same claim; serial sublimits do not extend it.
type DiscoveryExtractionRequest struct {
	ProjectID, DocumentID, AttemptID, AllocationID string
	OriginalPath                                   string
	Original                                       DiscoveryDocumentOriginal
	Actor                                          management.ExecutionActorRef
	CreatedAt, ComputeDeadline, WholeDeadline      time.Time
}
type DiscoveryExtractionOutcome struct {
	Identity                          backend.DocumentExtractionIdentity
	State, Stage, ReasonCode, Message string
	Ref                               string
	ExitCode                          *int
	OOMKilled                         bool
	ResourcesHeld, CleanupConfirmed   bool
	Result                            backend.DocumentExtractionOutput
}
type DiscoveryExtractionRecoveryRequest struct {
	ProjectID, DocumentID, AttemptID, AllocationID string
	Actor                                          management.ExecutionActorRef
	// Only explicit retirement/settlement is allowed; recovery never starts a
	// missing process or automatically creates a replacement extraction.
	Action string
}
type DiscoveryExtractionRuntime struct {
	CaptureActor     func(context.Context, string) (management.ExecutionActorRef, error)
	AuthorizeControl func(context.Context, string) error
	Busy             func(context.Context, string, string) (bool, error)
	Execute          func(context.Context, DiscoveryExtractionRequest, func(DiscoveryExtractionOutcome) error) (DiscoveryExtractionOutcome, error)
	// Read returns private durable lifecycle status without repair or dispatch.
	Read    func(context.Context, string, string, string) (DiscoveryExtractionOutcome, error)
	Recover func(context.Context, DiscoveryExtractionRecoveryRequest, func(DiscoveryExtractionOutcome) error) (DiscoveryExtractionOutcome, error)
}

func validDiscoveryExtractionRequest(request DiscoveryExtractionRequest) bool {
	return prototypeReleaseID.MatchString(request.ProjectID) && discoveryDocumentID.MatchString(request.DocumentID) && prototypeReleaseID.MatchString(request.AttemptID) && prototypeReleaseID.MatchString(request.AllocationID) && request.Actor.ProjectID == request.ProjectID && request.Original.Bytes > 0 && request.Original.Bytes <= backend.DocumentOriginalMaxBytes && hexDigest(request.Original.Digest) && !request.CreatedAt.IsZero() && request.ComputeDeadline.Sub(request.CreatedAt) == backend.DocumentComputeBudget && request.WholeDeadline.Sub(request.ComputeDeadline) == backend.DocumentCleanupBudget
}

type containedDiscoveryResult struct {
	Protocol         int                     `json:"protocol"`
	Profile          string                  `json:"profile"`
	AttemptID        string                  `json:"attemptId"`
	OriginalDigest   string                  `json:"originalDigest"`
	ExtractorVersion string                  `json:"extractorVersion"`
	Status           string                  `json:"status"`
	Pages            []DiscoveryDocumentPage `json:"pages"`
	Issues           []string                `json:"issues"`
}

// Completion labels, prose and an elapsed deadline cannot authorize evidence.
// The API must have durably captured this exact result AND proved retirement;
// uncertain leases/fences remain charged and cannot enter this decoder.
func decodeContainedDiscoveryResult(request DiscoveryExtractionRequest, outcome DiscoveryExtractionOutcome, revision int, actor ContextActor, now time.Time) (DiscoveryDocumentExtraction, error) {
	identity := outcome.Identity
	if identity.ActorDigest != backend.DocumentExtractionActorDigest(request.Actor.UserID, request.Actor.TeamID, request.Actor.ProjectID, request.Actor.Local) || !request.Actor.Local && actor.ID != request.Actor.UserID {
		return DiscoveryDocumentExtraction{}, errDiscoveryDocument
	}
	if !validDiscoveryExtractionRequest(request) || !backend.ValidateDocumentExtractionIdentity(identity, true) || identity.ProjectID != request.ProjectID || identity.DocumentID != request.DocumentID || identity.AttemptID != request.AttemptID || identity.AllocationID != request.AllocationID || identity.OriginalDigest != request.Original.Digest || outcome.State != "complete" || outcome.ResourcesHeld || !outcome.CleanupConfirmed || outcome.ExitCode == nil || *outcome.ExitCode != 0 || outcome.OOMKilled || !now.Before(request.WholeDeadline) || len(outcome.Result.Data) < 1 || len(outcome.Result.Data) > backend.DocumentResultMaxBytes || discoveryTextDigest(string(outcome.Result.Data)) != outcome.Result.Digest {
		return DiscoveryDocumentExtraction{}, errors.New("The exact extraction result and positive retirement are not verified; preserve its private receipt before restarting.")
	}
	decoder := json.NewDecoder(bytes.NewReader(outcome.Result.Data))
	decoder.DisallowUnknownFields()
	var result containedDiscoveryResult
	if decoder.Decode(&result) != nil || decoder.Decode(new(any)) != io.EOF || result.Protocol != backend.DocumentExtractionProtocol || result.Profile != backend.DocumentExtractionProfile || result.AttemptID != request.AttemptID || result.OriginalDigest != request.Original.Digest {
		return DiscoveryDocumentExtraction{}, errDiscoveryDocument
	}
	extraction := DiscoveryDocumentExtraction{Revision: revision, AttemptID: request.AttemptID, OriginalDigest: request.Original.Digest, Extractor: backend.DocumentExtractionProfile, ExtractorVersion: result.ExtractorVersion, Status: result.Status, Pages: result.Pages, Issues: result.Issues, At: now.UTC().Format(time.RFC3339Nano), Actor: actor}
	if err := validateDiscoveryExtraction(extraction, request.Original); err != nil {
		return DiscoveryDocumentExtraction{}, err
	}
	return extraction, nil
}
