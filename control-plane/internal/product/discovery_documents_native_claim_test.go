package product

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

func TestDiscoveryNativeClaimPublicationFaultsRetireOnlyUnallocatedStage(t *testing.T) {
	for _, boundary := range []string{"private-before", "private-after", "document-before", "document-after"} {
		t.Run(boundary, func(t *testing.T) {
			s, mux := journeyFixture(t)
			p := createJourneyProject(t, mux, "claim-publication-fault")
			handler := contextHistoryActorHandler(mux, management.User{ID: "person-one", Username: "One", Role: "member"})
			base := "/v1/product/projects/" + p.ID + "/discovery/documents"
			original := []byte("%PDF-private-intake-kept")
			document := discoveryUploadRequest(t, handler, base, "claim.pdf", "claim-upload-123456789", original, 201)
			actor := ContextActor{ID: "person-one", DisplayName: "One"}
			key := "claim-extract-123456789"
			s.mu.Lock()
			current, err := s.readDiscoveryDocument(p, document.ID)
			if err != nil {
				s.mu.Unlock()
				t.Fatal(err)
			}
			now := time.Now().UTC()
			attempt, err := discoveryStartAttempt(&current, document.Revision, key, actor, now)
			if err != nil {
				s.mu.Unlock()
				t.Fatal(err)
			}
			attempt.ExecutionProfile = backend.DocumentExtractionProfile
			attempt.ExpiresAt = now.Add(backend.DocumentWholeBudget).Format(time.RFC3339Nano)
			current.Document.Attempts[len(current.Document.Attempts)-1] = attempt
			request := DiscoveryExtractionRequest{ProjectID: p.ID, DocumentID: document.ID, AttemptID: attempt.ID, AllocationID: newID("document-"), Original: document.Original, Actor: management.ExecutionActorRef{UserID: actor.ID, ProjectID: p.ID}, ExpectedDocumentRevision: current.Document.Revision, CreatedAt: now, ComputeDeadline: now.Add(backend.DocumentComputeBudget), WholeDeadline: now.Add(backend.DocumentWholeBudget)}
			if err = s.stageDiscoveryNativeOriginal(p, &request); err != nil {
				s.mu.Unlock()
				t.Fatal(err)
			}
			scope, err := s.approvalScopes(p.Path, p.ID)
			if err != nil {
				s.mu.Unlock()
				t.Fatal(err)
			}
			receipt := discoveryNativeReceipt{Version: 1, AccountScope: scope.AccountScope, ProjectScope: scope.ProjectScope, Request: request, Outcome: DiscoveryExtractionOutcome{State: "claimed", ReaderSettled: true}}
			writers := discoveryNativeClaimWriters{
				private: func(p project.Project, value discoveryNativeReceipt) error {
					if boundary == "private-before" {
						return errors.New("inert private publication failure")
					}
					err := s.saveDiscoveryNative(p, value)
					if err == nil && boundary == "private-after" {
						return errors.New("inert lost private save ACK")
					}
					return err
				},
				document: func(value storedDiscoveryDocument) error {
					if boundary == "document-before" {
						return errors.New("inert public publication failure")
					}
					err := s.saveDiscoveryDocument(value)
					if err == nil && boundary == "document-after" {
						return errors.New("inert lost public save ACK")
					}
					return err
				},
			}
			err = s.publishDiscoveryNativeClaim(p, current, receipt, writers)
			s.mu.Unlock()
			if err == nil {
				t.Fatal("inert publication failure disappeared")
			}
			if _, err = os.Stat(request.OriginalPath); !os.IsNotExist(err) {
				t.Fatal("unallocated staged original accumulated", err)
			}
			s.mu.RLock()
			after, err := s.readDiscoveryDocument(p, document.ID)
			s.mu.RUnlock()
			if err != nil {
				t.Fatal(err)
			}
			if boundary == "document-after" {
				if len(after.Document.Attempts) != 1 || after.Document.Attempts[0].Status != "interrupted" {
					t.Fatal("published public attempt was erased or left working")
				}
			} else if len(after.Document.Attempts) != 0 {
				t.Fatal("unpublished attempt entered public history")
			}
			staged, err := os.ReadDir(filepath.Dir(request.OriginalPath))
			if err != nil {
				t.Fatal(err)
			}
			want := 0
			if boundary == "document-after" {
				want = 1
			}
			if len(staged) != want {
				t.Fatal("unallocated private receipt accumulated", len(staged), want)
			}
			// Retry the ordinary route with the same original key. A surviving public
			// ACK is read back; otherwise only the newly published attempt may execute.
			executions := 0
			s.SetDiscoveryExtractionRuntime(DiscoveryExtractionRuntime{
				CaptureActor: func(ctx context.Context, projectID string) (management.ExecutionActorRef, error) {
					return management.ExecutionActorRef{UserID: actor.ID, ProjectID: projectID}, ctx.Err()
				},
				Busy: func(context.Context, string, string) (bool, error) { return false, nil },
				Execute: func(ctx context.Context, req DiscoveryExtractionRequest, persist func(DiscoveryExtractionOutcome) error) (DiscoveryExtractionOutcome, error) {
					executions++
					if err := req.ValidateCurrent(ctx); err != nil {
						t.Fatal("unpublished attempt dispatched", err)
					}
					out := DiscoveryExtractionOutcome{State: "blocked", Stage: "inert-no-parser", ReaderSettled: true, CleanupConfirmed: true}
					return out, persist(out)
				},
			})
			journeyRequest(t, handler, "POST", base+"/"+document.ID+"/extractions", map[string]any{"expectedRevision": document.Revision, "idempotencyKey": key}, 200)
			wantExecutions := 1
			if boundary == "document-after" {
				wantExecutions = 0
			}
			if executions != wantExecutions {
				t.Fatal("same request replayed an unpublished or already-published attempt", executions)
			}
			staged, err = os.ReadDir(filepath.Dir(request.OriginalPath))
			if err != nil || len(staged) != 1 {
				t.Fatal("retry accumulated orphan stages/claims", len(staged), err)
			}
			path, err := s.discoveryDocumentPath(p.ID, document.ID+".original", false)
			if err != nil {
				t.Fatal(err)
			}
			retained, err := discoveryReadPrivate(path, discoveryOriginalBytes)
			if err != nil || string(retained) != string(original) {
				t.Fatal("rollback changed immutable original", err)
			}
		})
	}
}

func TestDiscoveryNativeUnallocatedRollbackRefusesPossiblyAllocatedReceipt(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "claim-foreign-owner")
	handler := contextHistoryActorHandler(mux, management.User{ID: "person-one", Username: "One", Role: "member"})
	document := discoveryUploadRequest(t, handler, "/v1/product/projects/"+p.ID+"/discovery/documents", "claim.pdf", "allocated-upload-123456789", []byte("%PDF-retained"), 201)
	now := time.Now().UTC()
	request := DiscoveryExtractionRequest{ProjectID: p.ID, DocumentID: document.ID, AttemptID: newID("extract-"), AllocationID: newID("document-"), Original: document.Original, Actor: management.ExecutionActorRef{UserID: "person-one", ProjectID: p.ID}, ExpectedDocumentRevision: 2, CreatedAt: now, ComputeDeadline: now.Add(backend.DocumentComputeBudget), WholeDeadline: now.Add(backend.DocumentWholeBudget)}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.stageDiscoveryNativeOriginal(p, &request); err != nil {
		t.Fatal(err)
	}
	scope, err := s.approvalScopes(p.Path, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	receipt := discoveryNativeReceipt{Version: 1, AccountScope: scope.AccountScope, ProjectScope: scope.ProjectScope, Request: request, Outcome: DiscoveryExtractionOutcome{State: "claimed", ResourcesHeld: true, ReaderSettled: true}}
	if err = s.rollbackUnallocatedDiscoveryNativeClaim(p, receipt); err == nil {
		t.Fatal("possibly allocated claim was retired")
	}
	if _, err = os.Stat(request.OriginalPath); err != nil {
		t.Fatal("possibly allocated original copy removed", err)
	}
}
