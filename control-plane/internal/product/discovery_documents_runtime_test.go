package product

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
)

func TestContainedDiscoveryAcceptsOnlyExactResultAfterPositiveRetirement(t *testing.T) {
	now := time.Now().UTC()
	actor := ContextActor{ID: "person-one", DisplayName: "One"}
	request := DiscoveryExtractionRequest{ProjectID: "project-one", DocumentID: "doc-" + strings.Repeat("a", 64), AttemptID: "extract-one", AllocationID: "document-allocation", OriginalPath: "private-original", Original: DiscoveryDocumentOriginal{Digest: strings.Repeat("b", 64), Bytes: 10, MimeType: "application/pdf"}, Actor: management.ExecutionActorRef{UserID: actor.ID, ProjectID: "project-one"}, CreatedAt: now, ComputeDeadline: now.Add(backend.DocumentComputeBudget), WholeDeadline: now.Add(backend.DocumentWholeBudget)}
	payload := containedDiscoveryResult{Protocol: 1, Profile: backend.DocumentExtractionProfile, AttemptID: request.AttemptID, OriginalDigest: request.Original.Digest, ExtractorVersion: "luci-document-v1", Status: "ready", Issues: []string{}, Pages: []DiscoveryDocumentPage{{Page: 1, Segments: []DiscoveryDocumentSegment{{ID: "p1-text", Kind: "text", Text: "Source fact", TextDigest: discoveryTextDigest("Source fact"), Location: DiscoveryDocumentLocation{Width: 10000, Height: 10000}}}}}}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	zero := 0
	outcome := DiscoveryExtractionOutcome{Identity: backend.DocumentExtractionIdentity{Protocol: 1, Profile: backend.DocumentExtractionProfile, EngineID: "fixture-engine", InstanceID: "fixture-instance", AllocationID: request.AllocationID, ScopeDigest: strings.Repeat("c", 64), ActorDigest: backend.DocumentExtractionActorDigest(actor.ID, "", request.ProjectID, false), ProjectID: request.ProjectID, DocumentID: request.DocumentID, AttemptID: request.AttemptID, OriginalDigest: request.Original.Digest, CommandDigest: strings.Repeat("d", 64)}, State: "complete", ExitCode: &zero, CleanupConfirmed: true, Result: backend.DocumentExtractionOutput{Data: raw, Digest: discoveryTextDigest(string(raw))}}
	if _, err = decodeContainedDiscoveryResult(request, outcome, 1, actor, now.Add(time.Second)); err != nil {
		t.Fatal("exact positively retired result refused", err)
	}
	for _, scenario := range []string{"resources-held", "cleanup-unknown", "foreign-attempt", "foreign-actor", "unsupported-label", "changed-bytes", "oom"} {
		t.Run(scenario, func(t *testing.T) {
			changed := outcome
			switch scenario {
			case "resources-held":
				changed.ResourcesHeld = true
			case "cleanup-unknown":
				changed.CleanupConfirmed = false
			case "foreign-attempt":
				changed.Identity.AttemptID = "extract-other"
			case "foreign-actor":
				changed.Identity.ActorDigest = strings.Repeat("0", 64)
			case "unsupported-label":
				changed.State = "passed"
			case "changed-bytes":
				changed.Result.Digest = strings.Repeat("0", 64)
			case "oom":
				changed.OOMKilled = true
			}
			if _, err := decodeContainedDiscoveryResult(request, changed, 1, actor, now.Add(time.Second)); err == nil {
				t.Fatal("unverified extraction accepted", scenario)
			}
		})
	}
	if _, err = decodeContainedDiscoveryResult(request, outcome, 1, actor, request.WholeDeadline); err == nil {
		t.Fatal("elapsed deadline became settled accepted evidence")
	}
}
