package product

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

// Seed immutable, individually valid original revisions through the existing
// private store creation seam. No production mutation/validation is bypassed.
func seedGTMArchiveRows(t *testing.T, s *Server, p project.Project, source DistributeArtifactSource, actor ContextActor, count int) (gtmLedger, []gtmSaveRequest) {
	t.Helper()
	path, ledger, err := s.readGTMLedger(p)
	if err != nil {
		t.Fatal(err)
	}
	requests := []gtmSaveRequest{}
	for len(ledger.Revisions) < count {
		revision := len(ledger.Revisions) + 1
		content := gtmFixtureContent()
		content.Offer = fmt.Sprintf("Saved offer %d", revision)
		base := ""
		if revision > 1 {
			base = ledger.Revisions[revision-2].Digest
		}
		request := gtmSaveRequest{ExpectedRevision: revision - 1, ExpectedDigest: base, ExpectedSourceDigest: reviewDigest(source), IdempotencyKey: fmt.Sprintf("gtm-retained-original-%04d", revision), Content: content}
		row := GTMWorkspace{ProjectID: p.ID, Revision: revision, Source: source, SourceDigest: reviewDigest(source), Content: content, Actor: actor, At: time.Date(2026, 10, 31, 0, 0, revision, 0, time.UTC).Format(time.RFC3339Nano), RequestKeyDigest: artifactRequestKey(p.ID, actor.ID, request.IdempotencyKey), RequestDigest: reviewDigest(request)}
		row.Digest = gtmDigest(row)
		ledger.Revisions = append(ledger.Revisions, row)
		requests = append(requests, request)
	}
	if err = durableRunJSON(path, ledger); err != nil {
		t.Fatal(err)
	}
	return ledger, requests
}
func prepareGTMArchiveTestSave(t *testing.T, s *Server, p project.Project, ledger gtmLedger, source DistributeArtifactSource, ctx context.Context, actor ContextActor) preparedGTMRevision {
	t.Helper()
	last := ledger.Revisions[len(ledger.Revisions)-1]
	content := last.Content
	content.Offer = "Next saved offer"
	request := gtmSaveRequest{ExpectedRevision: last.Revision, ExpectedDigest: last.Digest, ExpectedSourceDigest: reviewDigest(source), IdempotencyKey: fmt.Sprintf("gtm-next-original-%04d", last.Revision+1), Content: content}
	s.mu.Lock()
	s.teamControlsMu.Lock()
	prepared, err := s.prepareGTMRevisionLocked(ctx, p, actor, request, nil)
	s.teamControlsMu.Unlock()
	s.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	return prepared
}
func TestGTMArchiveSave51KeepsGlobalCASAndArchivedRequestReplay(t *testing.T) {
	s, handler, id := controlTestHandler(t)
	base := "/v1/product/projects/" + id + "/distribute/gtm"
	var initial GTMView
	if json.Unmarshal(controlTestRequest(t, handler, "GET", base, nil, 200).Body.Bytes(), &initial) != nil {
		t.Fatal("initial view")
	}
	p, _ := s.resolveProject(id)
	ledger, requests := seedGTMArchiveRows(t, s, p, initial.Source, ContextActor{ID: "admin", DisplayName: "admin"}, 50)
	beforeDigest := ledger.Revisions[0].Digest
	request := gtmSaveRequest{ExpectedRevision: 50, ExpectedDigest: ledger.Revisions[49].Digest, ExpectedSourceDigest: initial.SourceDigest, IdempotencyKey: "gtm-original-save-fifty-one", Content: gtmFixtureContent()}
	var view GTMView
	json.Unmarshal(controlTestRequest(t, handler, "POST", base, request, 200).Body.Bytes(), &view)
	if view.Workspace == nil || view.Workspace.Revision != 51 || len(view.History) != 1 || view.ArchivedRevisions != 50 || len(view.Archives) != 1 {
		t.Fatal("save51 became a reset or stopped at the old limit", view)
	}
	var replay GTMView
	json.Unmarshal(controlTestRequest(t, handler, "POST", base, requests[0], 200).Body.Bytes(), &replay)
	if replay.Freshness != "readback" || replay.Workspace == nil || replay.Workspace.Revision != 1 || replay.Workspace.Digest != beforeDigest {
		t.Fatal("archived exact request did not return original", replay)
	}
	changed := requests[0]
	changed.Content.Offer = "A different request"
	controlTestRequest(t, handler, "POST", base, changed, 409)
	_, current, err := s.readGTMLedger(p)
	if err != nil || current.Revisions[0].Revision != 51 || len(current.Revisions) != 1 {
		t.Fatal("archived replay appended or rewound current workspace", err)
	}
	var history GTMHistoryView
	json.Unmarshal(controlTestRequest(t, handler, "GET", base+"?history="+view.Archives[0].ID, nil, 200).Body.Bytes(), &history)
	if len(history.Revisions) != 50 || history.Revisions[0].Digest != beforeDigest || !reflect.DeepEqual(history.Revisions[0], ledger.Revisions[0]) {
		t.Fatal("old rows or private correlation metadata changed")
	}
	controlTestRequest(t, handler, "GET", base+"?revision=1", nil, 200)
	controlTestRequest(t, handler, "GET", base+"?revision=01", nil, 409)
	controlTestRequest(t, handler, "POST", base, request, 200)
}
func TestGTMArchiveInterruptedPublishReusesExactImmutableChunkAndHoldsPartial(t *testing.T) {
	for _, boundary := range []string{"archive_before_ledger", "partial_archive", "oversized_archive"} {
		t.Run(boundary, func(t *testing.T) {
			s, handler, id := controlTestHandler(t)
			var initial GTMView
			json.Unmarshal(controlTestRequest(t, handler, "GET", "/v1/product/projects/"+id+"/distribute/gtm", nil, 200).Body.Bytes(), &initial)
			p, _ := s.resolveProject(id)
			actor := ContextActor{ID: "admin", DisplayName: "admin"}
			ledger, _ := seedGTMArchiveRows(t, s, p, initial.Source, actor, 50)
			prepared := prepareGTMArchiveTestSave(t, s, p, ledger, initial.Source, orchestratorActor(t, "admin"), actor)
			if prepared.Archive == nil {
				t.Fatal("rollover was not prepared")
			}
			if boundary == "partial_archive" || boundary == "oversized_archive" {
				partial := []byte("{partial")
				if boundary == "oversized_archive" {
					partial = make([]byte, prepared.Archive.Descriptor.Bytes+1)
				}
				if os.WriteFile(prepared.Archive.Path, partial, 0600) != nil {
					t.Fatal("seed partial")
				}
				if persistGTMRevisionLocked(context.Background(), prepared) == nil {
					t.Fatal("partial immutable archive was overwritten")
				}
				_, retained, err := s.readGTMLedger(p)
				if err != nil || len(retained.Revisions) != 50 || len(retained.Archives) != 0 {
					t.Fatal("failed rollover discarded active history", err)
				}
				raw, _ := os.ReadFile(prepared.Archive.Path)
				if !reflect.DeepEqual(raw, partial) {
					t.Fatal("uncertain archive changed")
				}
				if boundary == "oversized_archive" {
					descriptor := prepared.Archive.Descriptor
					if _, err = s.chooseGTMArchiveSlot(p, ledger, &descriptor); err == nil {
						t.Fatal("oversized original was bypassed or undercharged")
					}
					return
				}
				continued := prepareGTMArchiveTestSave(t, s, p, ledger, initial.Source, orchestratorActor(t, "admin"), actor)
				if continued.Archive.Descriptor.Slot != 1 || continued.Archive.Descriptor.StorageBytes != continued.Archive.Descriptor.Bytes*2 {
					t.Fatal("partial attempt not preserved/accounted")
				}
				if err = persistGTMRevisionLocked(context.Background(), continued); err != nil {
					t.Fatal("explicit save could not continue in one fresh slot", err)
				}
				after, _ := os.ReadFile(prepared.Archive.Path)
				if !reflect.DeepEqual(after, partial) {
					t.Fatal("continuation rewrote partial original")
				}
				return
			}
			if err := persistGTMArchive(context.Background(), prepared); err != nil {
				t.Fatal(err)
			}
			raw, _ := os.ReadFile(prepared.Archive.Path)
			if len(raw) != prepared.Archive.Descriptor.Bytes || gtmRawHash(raw) != prepared.Archive.Descriptor.SHA256 {
				t.Fatal("archive limit does not match actual indented bytes plus LF")
			}
			again := prepareGTMArchiveTestSave(t, s, p, ledger, initial.Source, orchestratorActor(t, "admin"), actor)
			if err := persistGTMRevisionLocked(context.Background(), again); err != nil {
				t.Fatal("exact interrupted archive could not be reconciled", err)
			}
			after, _ := os.ReadFile(prepared.Archive.Path)
			if !reflect.DeepEqual(raw, after) {
				t.Fatal("immutable archived bytes were rewritten")
			}
		})
	}
}
func TestGTMArchiveScopesAndReadOnlyHistoryNeverReplaceCurrentStrategy(t *testing.T) {
	s, handler, id := controlTestHandler(t)
	base := "/v1/product/projects/" + id + "/distribute/gtm"
	var initial GTMView
	json.Unmarshal(controlTestRequest(t, handler, "GET", base, nil, 200).Body.Bytes(), &initial)
	p, _ := s.resolveProject(id)
	actor := ContextActor{ID: "admin", DisplayName: "admin"}
	ledger, _ := seedGTMArchiveRows(t, s, p, initial.Source, actor, 50)
	prepared := prepareGTMArchiveTestSave(t, s, p, ledger, initial.Source, orchestratorActor(t, "admin"), actor)
	if persistGTMRevisionLocked(context.Background(), prepared) != nil {
		t.Fatal("rollover")
	}
	path, current, err := s.readGTMLedger(p)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	controlTestRequest(t, handler, "GET", base+"?history="+current.Archives[0].ID, nil, 200)
	after, _ := os.ReadFile(path)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("history GET changed canonical ledger")
	}
	owner := current
	owner.AccountScope = "another-account"
	if _, err = s.readGTMArchive(p, owner, current.Archives[0]); err == nil {
		t.Fatal("archive crossed account scope")
	}
	request := httptest.NewRequest("GET", base+"?history="+current.Archives[0].ID, nil)
	request.SetPathValue("id", id)
	request = request.WithContext(management.WithTeamAccess(orchestratorActor(t, "admin"), management.TeamAccess{ProjectID: id}))
	response := httptest.NewRecorder()
	if !s.gtmHistoryResponse(response, request, p, current) || response.Code != 403 {
		t.Fatal("historical source was published without current project Read", response.Code)
	}
}
func TestGTMArchiveMetricImportsAndLegacyDigestsSurviveContinuation(t *testing.T) {
	s, handler, id := controlTestHandler(t)
	base := "/v1/product/projects/" + id + "/distribute/gtm"
	var initial GTMView
	json.Unmarshal(controlTestRequest(t, handler, "GET", base, nil, 200).Body.Bytes(), &initial)
	p, _ := s.resolveProject(id)
	actor := ContextActor{ID: "admin", DisplayName: "admin"}
	ledger, requests := seedGTMArchiveRows(t, s, p, initial.Source, actor, 50)
	original := ledger.Revisions[0]
	imports := []GTMMetricImport{{Version: 1, SourceName: "outcomes.csv", SourceSHA256: reviewDigest("fixture"), SourceBytes: 100, Rows: []GTMMetricRow{{Row: 2, Metric: "activations", Start: "2026-10-01", End: "2026-10-31", Value: 4}}}}
	ledger.Revisions[49].Content.MetricImports = &imports
	requests[49].Content = ledger.Revisions[49].Content
	ledger.Revisions[49].RequestDigest = reviewDigest(requests[49])
	ledger.Revisions[49].Digest = gtmDigest(ledger.Revisions[49])
	path, _, _ := s.readGTMLedger(p)
	if durableRunJSON(path, ledger) != nil {
		t.Fatal("seed imports")
	}
	missing := gtmSaveRequest{ExpectedRevision: 50, ExpectedDigest: ledger.Revisions[49].Digest, ExpectedSourceDigest: initial.SourceDigest, IdempotencyKey: "gtm-omitted-imports-rollover", Content: gtmFixtureContent()}
	controlTestRequest(t, handler, "POST", base, missing, 422)
	prepared := prepareGTMArchiveTestSave(t, s, p, ledger, initial.Source, orchestratorActor(t, "admin"), actor)
	if persistGTMRevisionLocked(context.Background(), prepared) != nil {
		t.Fatal("rollover imports")
	}
	_, current, err := s.readGTMLedger(p)
	if err != nil {
		t.Fatal(err)
	}
	archived, err := s.readGTMArchive(p, current, current.Archives[0])
	if err != nil || !reflect.DeepEqual(archived.Revisions[0], original) || !reflect.DeepEqual(archived.Revisions[49].Content.MetricImports, &imports) || !reflect.DeepEqual(current.Revisions[0].Content.MetricImports, &imports) {
		t.Fatal("legacy omission or imported evidence lost its original meaning", err)
	}
}
func TestGTMArchiveOrchestratorRecoveryFindsOriginalAppliedRowAfterRollover(t *testing.T) {
	s, mux := journeyFixture(t)
	created := createJourneyProject(t, mux, "gtm-archive-origin")
	ctx := orchestratorActor(t, "alice")
	turn, _ := gtmOrchestratorTurnForTest(t, s, created.ID)
	view, err := s.OrchestratorView(ctx, created.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.applyOrchestratorProposal(ctx, created.ID, turn.ID, view.Revision, view.Context.Digest); err != nil {
		t.Fatal(err)
	}
	p, _ := s.resolveProject(created.ID)
	_, first, _ := s.readGTMLedger(p)
	actor, _ := authenticatedContextActor(ctx)
	ledger, _ := seedGTMArchiveRows(t, s, p, first.Revisions[0].Source, actor, 50)
	prepared := prepareGTMArchiveTestSave(t, s, p, ledger, first.Revisions[0].Source, ctx, actor)
	if err = persistGTMRevisionLocked(ctx, prepared); err != nil {
		t.Fatal(err)
	}
	saved, err := s.readOrchestrator(p)
	if err != nil {
		t.Fatal(err)
	}
	index := len(saved.Turns) - 1
	saved.Turns[index].ApplyIntent = "applying"
	saved.Turns[index].AppliedRevision = 0
	s.mu.Lock()
	err = s.resumeOrchestratorGTMLocked(ctx, p, &saved, index)
	s.mu.Unlock()
	if err != nil {
		t.Fatal("exact archived original was mistaken for an absent append", err)
	}
	_, after, err := s.readGTMLedger(p)
	if err != nil || len(after.Revisions) != 1 || after.Revisions[0].Revision != 51 || saved.Turns[index].AppliedRevision != 1 {
		t.Fatal("recovery appended or substituted another model-origin row", err)
	}
}
