package product

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOrchestratorResolutionDurablyFencesLateRequestAfterRestart(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "request-fence")
	ctx := orchestratorActor(t, "alice")
	snapshot, err := s.OrchestratorSnapshot(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	req := OrchestratorTurnRequest{ExpectedRevision: 0, ExpectedContextDigest: snapshot.Digest, IdempotencyKey: "request-never-accepted", Message: "Please help our project", RouteDigest: strings.Repeat("a", 64)}
	resolved, err := s.ResolveOrchestratorRequest(ctx, p.ID, req)
	if err != nil || resolved.Status != "not-sent" || resolved.Turn != nil {
		t.Fatal(resolved, err)
	}
	path, _ := s.orchestratorPath(p.ID, false)
	before, _ := os.ReadFile(path)
	reopened, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(reopened.Close)
	resolved, err = reopened.ResolveOrchestratorRequest(ctx, p.ID, req)
	if err != nil || resolved.Status != "not-sent" {
		t.Fatal(resolved, err)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("exact recovery rewrote its fence")
	}
	if _, _, _, err = reopened.ReserveOrchestratorTurn(ctx, p.ID, req, "assist-late-original"); !errors.Is(err, ErrOrchestratorNotSent) {
		t.Fatal("late original was not fenced", err)
	}
	req.Message = "A different paid instruction"
	if _, err = reopened.ResolveOrchestratorRequest(ctx, p.ID, req); !errors.Is(err, ErrOrchestratorConflict) {
		t.Fatal("changed payload reused fence", err)
	}
	if _, _, _, err = reopened.ReserveOrchestratorTurn(ctx, p.ID, req, "assist-late-changed"); !errors.Is(err, ErrOrchestratorNotSent) {
		t.Fatal("changed payload reopened closed key", err)
	}
	ledger, err := reopened.readOrchestrator(p)
	if err != nil || len(ledger.Turns) != 0 || len(ledger.NoDispatch) != 1 {
		t.Fatal(ledger, err)
	}
}

func TestOrchestratorResolutionExactIdentityAndForeignActor(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "request-identity")
	ctx := orchestratorActor(t, "alice")
	snapshot, err := s.OrchestratorSnapshot(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	req := OrchestratorTurnRequest{ExpectedContextDigest: snapshot.Digest, IdempotencyKey: "request-exact-identity", Message: "Review our next step", RouteDigest: strings.Repeat("a", 64)}
	turn, _, _, err := s.ReserveOrchestratorTurn(ctx, p.ID, req, "assist-original-receipt")
	if err != nil {
		t.Fatal(err)
	}
	path, _ := s.orchestratorPath(p.ID, false)
	before, _ := os.ReadFile(path)
	resolved, err := s.ResolveOrchestratorRequest(ctx, p.ID, req)
	if err != nil || resolved.Status != "accepted" || resolved.Turn == nil || resolved.Turn.ID != turn.ID {
		t.Fatal(resolved, err)
	}
	for _, field := range []string{"message", "revision", "context", "reply", "route"} {
		t.Run(field, func(t *testing.T) {
			changed := req
			switch field {
			case "message":
				changed.Message += " changed"
			case "revision":
				changed.ExpectedRevision++
			case "context":
				changed.ExpectedContextDigest = strings.Repeat("b", 64)
			case "reply":
				changed.ReplyToTurnID = "turn-other"
			case "route":
				changed.RouteDigest = strings.Repeat("b", 64)
			}
			result, _ := s.ResolveOrchestratorRequest(ctx, p.ID, changed)
			if result.Status != "unknown" {
				t.Fatal("changed request resolved as exact", result)
			}
		})
	}
	foreign, err := s.ResolveOrchestratorRequest(orchestratorActor(t, "bob"), p.ID, req)
	if err != nil || foreign.Status != "unknown" || foreign.Turn != nil {
		t.Fatal("foreign actor acquired receipt", foreign, err)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("resolution changed the accepted ledger")
	}
	// Legacy receipts still support their original send retry, but cannot invent
	// an exact admission revision they never recorded.
	ledger, _ := s.readOrchestrator(p)
	ledger.Turns[0].AdmissionRevision = nil
	if err = s.saveOrchestrator(ledger); err != nil {
		t.Fatal(err)
	}
	resolved, err = s.ResolveOrchestratorRequest(ctx, p.ID, req)
	if err != nil || resolved.Status != "unknown" || resolved.Turn == nil {
		t.Fatal(resolved, err)
	}
	if existing, _, duplicate, err := s.ReserveOrchestratorTurn(ctx, p.ID, req, "assist-never-repeat"); err != nil || !duplicate || existing.ID != turn.ID {
		t.Fatal("legacy retry compatibility changed", err)
	}
}

func TestOrchestratorResolutionSearchesArchiveAndRetainsFencesAcrossRotation(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "request-archive")
	ctx := orchestratorActor(t, "alice")
	snapshot, err := s.OrchestratorSnapshot(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	fence := OrchestratorTurnRequest{ExpectedContextDigest: snapshot.Digest, IdempotencyKey: "request-absent-before-rotation", Message: "Never sent", RouteDigest: strings.Repeat("a", 64)}
	if result, err := s.ResolveOrchestratorRequest(ctx, p.ID, fence); err != nil || result.Status != "not-sent" {
		t.Fatal(result, err)
	}
	req := fence
	req.ExpectedRevision = 1
	req.IdempotencyKey = "request-accepted-before-rotation"
	turn, _, _, err := s.ReserveOrchestratorTurn(ctx, p.ID, req, "assist-archived-original")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.CompleteOrchestratorTurn(p.ID, turn.ID, turn.JobID, "failed", nil); err != nil {
		t.Fatal(err)
	}
	fillAndRotate := func() {
		t.Helper()
		ledger, err := s.readOrchestrator(p)
		if err != nil {
			t.Fatal(err)
		}
		template := ledger.Turns[len(ledger.Turns)-1]
		for i := 0; i < 10; i++ {
			next := template
			next.ID = newID("turn-")
			next.JobID = newID("assist-")
			next.Sequence = nextOrchestratorSequence(ledger)
			next.KeyDigest = reviewDigest(next.ID)
			next.RequestDigest = reviewDigest(next.JobID)
			ledger.Turns = append(ledger.Turns, next)
			ledger.Revision++
		}
		if err = s.saveOrchestrator(ledger); err != nil {
			t.Fatal(err)
		}
		storage, err := s.orchestratorStorageLocked(p, ledger)
		if err != nil {
			t.Fatal(err)
		}
		if err = s.rotateOrchestratorHistory(ctx, p.ID, orchestratorRotateRequest{ExpectedRevision: ledger.Revision, ExpectedStorageDigest: storage.StorageDigest, AcknowledgeExportedHistory: true}); err != nil {
			t.Fatal(err)
		}
	}
	fillAndRotate()
	resolved, err := s.ResolveOrchestratorRequest(ctx, p.ID, req)
	if err != nil || resolved.Status != "accepted" || resolved.Turn.ID != turn.ID {
		t.Fatal("archive receipt not recovered", resolved, err)
	}
	if recovered, _, duplicate, err := s.ReserveOrchestratorTurn(ctx, p.ID, req, "assist-never-repeat-archive"); err != nil || !duplicate || recovered.ID != turn.ID {
		t.Fatal("archive retry allocated", err)
	}
	fillAndRotate()
	resolved, err = s.ResolveOrchestratorRequest(ctx, p.ID, fence)
	if err != nil || resolved.Status != "not-sent" {
		t.Fatal("rotation lost no-dispatch fence", resolved, err)
	}
	if _, _, _, err = s.ReserveOrchestratorTurn(ctx, p.ID, fence, "assist-never-after-rotation"); !errors.Is(err, ErrOrchestratorNotSent) {
		t.Fatal(err)
	}
	resolved, err = s.ResolveOrchestratorRequest(ctx, p.ID, req)
	if err != nil || resolved.Status != "unknown" {
		t.Fatal("discarded history was treated as authoritative absence", resolved, err)
	}
}

func TestOrchestratorResolutionCorruptionAndCapacityStayUnknown(t *testing.T) {
	for _, failure := range []string{"ledger", "archive", "staging", "capacity"} {
		t.Run(failure, func(t *testing.T) {
			s, mux := journeyFixture(t)
			p := createJourneyProject(t, mux, "request-uncertain")
			ctx := orchestratorActor(t, "alice")
			snapshot, err := s.OrchestratorSnapshot(ctx, p.ID)
			if err != nil {
				t.Fatal(err)
			}
			req := OrchestratorTurnRequest{ExpectedContextDigest: snapshot.Digest, IdempotencyKey: "request-uncertain-identity", Message: "Continue", RouteDigest: strings.Repeat("a", 64)}
			switch failure {
			case "ledger":
				path, _ := s.orchestratorPath(p.ID, true)
				if err = os.WriteFile(path, []byte("{corrupt"), 0600); err != nil {
					t.Fatal(err)
				}
			case "archive", "staging":
				dir, err := s.orchestratorArchiveDir(p.ID, true)
				if err != nil {
					t.Fatal(err)
				}
				name := strings.Repeat("a", 64) + ".json"
				if failure == "staging" {
					name = ".luci-1234"
				}
				if err = os.WriteFile(filepath.Join(dir, name), []byte("{interrupted"), 0600); err != nil {
					t.Fatal(err)
				}
			case "capacity":
				ledger, err := s.readOrchestrator(p)
				if err != nil {
					t.Fatal(err)
				}
				for i := 0; i < orchestratorNoDispatchLimit; i++ {
					ledger.NoDispatch = append(ledger.NoDispatch, orchestratorNoDispatch{KeyDigest: reviewDigest(i), RequestDigest: reviewDigest([]int{i}), CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)})
				}
				if err = s.saveOrchestrator(ledger); err != nil {
					t.Fatal(err)
				}
			}
			before := auditTree(t, s.stateDir)
			result, _ := s.ResolveOrchestratorRequest(ctx, p.ID, req)
			if result.Status != "unknown" || result.Turn != nil {
				t.Fatal("uncertainty released the request", result)
			}
			after := auditTree(t, s.stateDir)
			if reviewDigest(before) != reviewDigest(after) {
				t.Fatal("uncertain resolution changed private storage")
			}
		})
	}
}
