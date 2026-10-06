package product

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

func TestOrchestratorRotationRecoversCapacityWithVerifiedPrivateExport(t *testing.T) {
	s, initial := journeyFixture(t)
	p := createJourneyProject(t, initial, "discussion-capacity")
	ctx := orchestratorActor(t, "alice")
	first, _ := reserveOrchestratorTest(t, s, p.ID, ctx, "Remember the agreed audience", "")
	answer := OrchestratorAnswer{Summary: "The agreed audience is the project team.", Question: "Should guests be admitted?", Blocking: true, Options: []string{"No", "With an invitation"}, Impact: []string{"Changes permission requirements"}}
	if err := s.CompleteOrchestratorTurn(p.ID, first.ID, first.JobID, "succeeded", &answer); err != nil {
		t.Fatal(err)
	}
	// Fill the exact supported record bound with valid terminal receipts, without
	// repeatedly invoking Git just to manufacture historical test input.
	saved, err := s.readOrchestrator(p)
	if err != nil {
		t.Fatal(err)
	}
	template := saved.Turns[0]
	for i := 1; i < orchestratorTurnLimit; i++ {
		next := template
		next.ID = newID("turn-")
		next.JobID = newID("assist-")
		next.Sequence = i + 1
		next.Answer = &OrchestratorAnswer{Summary: "The project intent is unchanged.", Options: []string{}, Impact: []string{}}
		next.KeyDigest = reviewDigest(next.ID)
		next.RequestDigest = reviewDigest(next.JobID)
		saved.Turns = append(saved.Turns, next)
		saved.Revision += 2
	}
	if err = s.saveOrchestrator(saved); err != nil {
		t.Fatal(err)
	}
	view, err := s.OrchestratorView(ctx, p.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	request := OrchestratorTurnRequest{ExpectedRevision: view.Revision, ExpectedContextDigest: view.Context.Digest, IdempotencyKey: "request-after-capacity", Message: "Answer the question", ReplyToTurnID: first.ID, RouteDigest: strings.Repeat("a", 64)}
	if _, _, _, err = s.ReserveOrchestratorTurn(ctx, p.ID, request, "assist-capacity"); !errors.Is(err, ErrOrchestratorCapacity) {
		t.Fatal("capacity was not bounded", err)
	}
	mux := http.NewServeMux()
	s.Register(mux)
	handler := contextHistoryActorHandler(mux, management.User{ID: "alice", Username: "alice", DisplayName: "Alice", Role: "member"})
	base := "/v1/product/projects/" + p.ID + "/orchestrator"
	var download orchestratorExport
	decodeEvidenceResponse(t, journeyRequest(t, handler, "GET", base+"/export", nil, 200).Body.Bytes(), &download)
	if len(download.Ledger.Turns) != 200 || len(download.Archive) != 0 || download.StorageDigest != view.StorageDigest {
		t.Fatal("export omitted current history")
	}
	req := orchestratorRotateRequest{ExpectedRevision: view.Revision, ExpectedStorageDigest: download.StorageDigest}
	journeyRequest(t, handler, "POST", base+"/rotate", req, 409)
	req.AcknowledgeExportedHistory = true
	var rotated OrchestratorView
	decodeEvidenceResponse(t, journeyRequest(t, handler, "POST", base+"/rotate", req, 200).Body.Bytes(), &rotated)
	if len(rotated.Turns) != 9 || rotated.Turns[0].ID != first.ID || !rotated.ArchivePresent || rotated.Revision <= view.Revision || len(rotated.BlockingQuestions) != 1 {
		t.Fatal("rotation lost an unresolved question or monotonic identity")
	}
	journeyRequest(t, handler, "POST", base+"/rotate", req, 409)
	// An archived request with its original revision cannot allocate new work.
	request.ExpectedRevision = view.Revision
	if _, _, _, err = s.ReserveOrchestratorTurn(ctx, p.ID, request, "assist-replay"); !errors.Is(err, ErrOrchestratorConflict) {
		t.Fatal("old revision replayed work", err)
	}
	next, _ := reserveOrchestratorTest(t, s, p.ID, ctx, "No guests", first.ID)
	if next.Sequence != 201 {
		t.Fatal("rotation reused a sequence", next.Sequence)
	}
	answer.Question = ""
	answer.Blocking = false
	if err = s.CompleteOrchestratorTurn(p.ID, next.ID, next.JobID, "succeeded", &answer); err != nil {
		t.Fatal(err)
	}
	if err = s.OrchestratorExecutionGate(p.ID); err != nil {
		t.Fatal("carried question could not be resolved", err)
	}
	// Export remains a read: canonical bytes and mtime do not change.
	path, _ := s.orchestratorPath(p.ID, false)
	before, _ := os.ReadFile(path)
	info, _ := os.Stat(path)
	decodeEvidenceResponse(t, journeyRequest(t, handler, "GET", base+"/export", nil, 200).Body.Bytes(), &download)
	after, _ := os.ReadFile(path)
	afterInfo, _ := os.Stat(path)
	if string(before) != string(after) || !info.ModTime().Equal(afterInfo.ModTime()) || len(download.Archive) != 1 || len(download.Archive[0].Ledger.Turns) != 200 {
		t.Fatal("export mutated state or omitted archive")
	}
	restarted, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = restarted.OrchestratorView(ctx, p.ID, 0); err != nil {
		t.Fatal("restart rejected rotated history", err)
	}
}

func TestOrchestratorRotationAccountsForInterruptedArchiveAndStaleDownload(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "interrupted-discussion")
	ctx := orchestratorActor(t, "alice")
	for i := 0; i < 10; i++ {
		turn, _ := reserveOrchestratorTest(t, s, p.ID, ctx, "Clarify the current work", "")
		answer := OrchestratorAnswer{Summary: "Current work remains a reviewed draft.", Options: []string{}, Impact: []string{}}
		if err := s.CompleteOrchestratorTurn(p.ID, turn.ID, turn.JobID, "succeeded", &answer); err != nil {
			t.Fatal(err)
		}
	}
	saved, _ := s.readOrchestrator(p)
	old, _ := s.orchestratorStorageLocked(p, saved)
	dir, err := s.orchestratorArchiveDir(p.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	orphanDigest := reviewDigest(saved)
	// Simulate a durable new archive before the active pointer was committed.
	if err = atomicJSON(filepath.Join(dir, orphanDigest+".json"), saved); err != nil {
		t.Fatal(err)
	}
	// A pre-rename temporary file is uncommitted, but still changes cleanup CAS.
	stagingPath := filepath.Join(dir, ".luci-1234")
	if err = os.WriteFile(stagingPath, []byte(`{"version":1,"turns":[`), 0600); err != nil {
		t.Fatal(err)
	}
	current, err := s.orchestratorStorageLocked(p, saved)
	if err != nil {
		t.Fatal(err)
	}
	if old.StorageDigest == current.StorageDigest || len(current.Archive) != 1 || len(current.Staging) != 1 {
		t.Fatal("orphan escaped export CAS")
	}
	req := orchestratorRotateRequest{ExpectedRevision: saved.Revision, ExpectedStorageDigest: old.StorageDigest, AcknowledgeExportedHistory: true}
	if err = s.rotateOrchestratorHistory(ctx, p.ID, req); !errors.Is(err, ErrOrchestratorConflict) {
		t.Fatal("stale download deleted unseen archive", err)
	}
	if _, err = os.Stat(filepath.Join(dir, orphanDigest+".json")); err != nil {
		t.Fatal("failed CAS mutated archive")
	}
	req.ExpectedStorageDigest = current.StorageDigest
	if err = s.rotateOrchestratorHistory(ctx, p.ID, req); err != nil {
		t.Fatal("interrupted rotation could not recover", err)
	}
	if _, err = os.Stat(stagingPath); !os.IsNotExist(err) {
		t.Fatal("acknowledged uncommitted staging was not recovered", err)
	}
	next, _ := reserveOrchestratorTest(t, s, p.ID, ctx, "Continue after recovery", "")
	active, _ := s.OrchestratorView(ctx, p.ID, 0)
	req = orchestratorRotateRequest{ExpectedRevision: active.Revision, ExpectedStorageDigest: active.StorageDigest, AcknowledgeExportedHistory: true}
	if err = s.rotateOrchestratorHistory(ctx, p.ID, req); !errors.Is(err, ErrOrchestratorBusy) {
		t.Fatal("rotation discarded active provider ownership", err)
	}
	if err = s.CompleteOrchestratorTurn(p.ID, next.ID, next.JobID, "failed", nil); err != nil {
		t.Fatal(err)
	}
	saved, _ = s.readOrchestrator(p)
	current, err = s.orchestratorStorageLocked(p, saved)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, current.Archive[0].Digest+".json"), []byte(`{"version":1}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = s.OrchestratorView(ctx, p.ID, 0); !errors.Is(err, ErrOrchestratorInvalid) {
		t.Fatal("corrupt archive silently dropped", err)
	}
}
