package product

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
)

func TestDiscoveryNativeProfilePreservesLegacyAndRejectsCallerExtensions(t *testing.T) {
	now := time.Now().UTC()
	if !validDiscoveryAttemptProfile("", now, now.Add(120*time.Second)) || !validDiscoveryAttemptProfile(backend.DocumentExtractionProfile, now, now.Add(180*time.Second)) {
		t.Fatal("legacy/native profile refused")
	}
	for _, profile := range []string{"", "unknown", backend.DocumentExtractionProfile} {
		if validDiscoveryAttemptProfile(profile, now, now.Add(181*time.Second)) {
			t.Fatal("caller extended whole deadline")
		}
	}
	if validDiscoveryAttemptProfile(backend.DocumentExtractionProfile, now, now.Add(120*time.Second)) {
		t.Fatal("native attempt silently used legacy deadline")
	}
}

func TestDiscoveryNativeIntakeReusesHistoryAndExactIdempotencyWithoutLeakingReceipt(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "native-intake")
	handler := contextHistoryActorHandler(mux, management.User{ID: "person-one", Username: "One", Role: "member"})
	base := "/v1/product/projects/" + p.ID + "/discovery/documents"
	document := discoveryUploadRequest(t, handler, base, "source.pdf", "native-upload-123456789", []byte("%PDF-inert-test-original"), 201)
	calls := 0
	s.SetDiscoveryExtractionRuntime(DiscoveryExtractionRuntime{
		CaptureActor: func(ctx context.Context, projectID string) (management.ExecutionActorRef, error) {
			return management.ExecutionActorRef{UserID: "person-one", ProjectID: projectID}, ctx.Err()
		},
		Busy: func(context.Context, string, string) (bool, error) { return false, nil },
		Execute: func(ctx context.Context, req DiscoveryExtractionRequest, persist func(DiscoveryExtractionOutcome) error) (DiscoveryExtractionOutcome, error) {
			calls++
			if err := req.ValidateCurrent(ctx); err != nil {
				t.Fatal(err)
			}
			data, err := discoveryReadPrivate(req.OriginalPath, discoveryOriginalBytes)
			if err != nil || discoveryTextDigest(string(data)) != req.Original.Digest || req.OriginalUserID < 1 || req.OriginalGroupID < 1 {
				t.Fatal("private staged original unavailable", err)
			}
			payload, _ := json.Marshal(containedDiscoveryResult{Protocol: 1, Profile: backend.DocumentExtractionProfile, AttemptID: req.AttemptID, OriginalDigest: req.Original.Digest, ExtractorVersion: "inert-fixture", Status: "ready", Issues: []string{}, Pages: []DiscoveryDocumentPage{{Page: 1, Segments: []DiscoveryDocumentSegment{{ID: "p1-fact", Kind: "text", Text: "Attributed source", TextDigest: discoveryTextDigest("Attributed source"), Location: DiscoveryDocumentLocation{Width: 10000, Height: 10000}}}}}})
			zero := 0
			out := DiscoveryExtractionOutcome{Identity: backend.DocumentExtractionIdentity{Protocol: 1, Profile: backend.DocumentExtractionProfile, EngineID: "fixture-engine", InstanceID: "fixture-instance", ScopeDigest: strings.Repeat("a", 64), ActorDigest: backend.DocumentExtractionActorDigest(req.Actor.UserID, "", req.ProjectID, false), ProjectID: req.ProjectID, DocumentID: req.DocumentID, AttemptID: req.AttemptID, AllocationID: req.AllocationID, OriginalDigest: req.Original.Digest, CommandDigest: strings.Repeat("b", 64)}, State: "complete", Stage: "cleanup", ExitCode: &zero, ReaderSettled: true, CleanupConfirmed: true, ResourcesHeld: true, Result: backend.DocumentExtractionOutput{Data: payload, Digest: discoveryTextDigest(string(payload))}}
			if err = persist(out); err != nil {
				return out, err
			}
			out.ResourcesHeld = false
			return out, persist(out)
		},
	})
	endpoint := base + "/" + document.ID + "/extractions"
	body := map[string]any{"expectedRevision": document.Revision, "idempotencyKey": "native-extract-123456789"}
	response := journeyRequest(t, handler, "POST", endpoint, body, 200)
	for _, secret := range []string{"fixture-engine", "CommandDigest", "OriginalPath", "Result\"", "ActorDigest"} {
		if strings.Contains(response.Body.String(), secret) {
			t.Fatal("private lifecycle receipt leaked", secret)
		}
	}
	journeyRequest(t, handler, "POST", endpoint, body, 200)
	if calls != 1 {
		t.Fatal("duplicate request replayed native execution")
	}
	s.mu.RLock()
	saved, err := s.readDiscoveryDocument(p, document.ID)
	s.mu.RUnlock()
	if err != nil || len(saved.Document.Extractions) != 1 || saved.Document.Extractions[0].Pages[0].Segments[0].Text != "Attributed source" {
		t.Fatal("actual retained evidence unavailable", err)
	}
	receipt, err := s.readDiscoveryNative(p, document.ID, saved.Document.Attempts[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(receipt.Request.OriginalPath); !os.IsNotExist(err) {
		t.Fatal("positively settled staged original was not retired", err)
	}
	before, _ := json.Marshal(saved)
	journeyRequest(t, handler, http.MethodGet, base+"/"+document.ID, nil, 200)
	s.mu.RLock()
	after, _ := s.readDiscoveryDocument(p, document.ID)
	s.mu.RUnlock()
	encoded, _ := json.Marshal(after)
	if string(before) != string(encoded) || calls != 1 {
		t.Fatal("GET changed evidence or started extraction")
	}
}
