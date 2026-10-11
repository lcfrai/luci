package product

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

func coordinationArchiveFixture(t *testing.T) (*Server, project.Project, OrchestratorTurn, OrchestratorTurn, coordinationLedger) {
	t.Helper()
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "coordination-archive")
	s.SetCoordinatorAuthorizer(func(context.Context, string) error { return nil })
	first := coordinationProposalTest(t, s, p.ID, "alice", "purpose", "A confirmed amendment")
	coordinationApplyTest(t, s, p.ID, "alice", first.ID, 200)
	pending := coordinationProposalTest(t, s, p.ID, "bob", "audience", "A separate pending amendment")
	saved, err := s.readCoordination(p)
	if err != nil {
		t.Fatal(err)
	}
	return s, p, first, pending, saved
}

func coordinationArchiveNow(t *testing.T, s *Server, p project.Project, saved *coordinationLedger) {
	t.Helper()
	// A storage-seam fixture uses the production retirement decision with an
	// incoming batch bound. The separate HTTP regression proves the real trigger.
	if err := s.compactCoordinationLocked(p, saved, coordinationRetainThreshold); err != nil {
		t.Fatal(err)
	}
	if err := s.saveCoordination(*saved); err != nil {
		t.Fatal(err)
	}
}

func TestCoordinationArchiveRestartExactEventsAndCurrentPendingOrigin(t *testing.T) {
	s, p, first, pending, saved := coordinationArchiveFixture(t)
	original := saved.Events[1]
	originIntent := saved.Intents[0].ID
	coordinationArchiveNow(t, s, p, &saved)
	if len(saved.Intents) != 1 || saved.Intents[0].TurnID != pending.ID || len(saved.Archives) != 1 || saved.EventSequence != 3 {
		t.Fatal("retirement changed pending ownership or sequence")
	}
	restarted, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot()})
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	restarted.SetCoordinatorAuthorizer(s.coordinatorAuthorize)
	path, _ := s.coordinationPath(p.ID, false)
	before, _ := os.ReadFile(path)
	receipt := CoordinationEventReceipt{EventID: original.ID, Sequence: original.Sequence, OriginIntentID: originIntent}
	observation, err := restarted.CoordinationEventByIdentity(orchestratorActor(t, "bob"), p.ID, receipt)
	if err != nil || !reflect.DeepEqual(observation.Event, original) || observation.Origin.ID != first.ID || observation.Intent.Status != "applied" || observation.Receipt.ArchiveDigest != saved.Archives[0].Digest {
		t.Fatal("exact retired event/origin unavailable", err)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("readback mutated coordination")
	}
	if _, err = restarted.CoordinationEventByIdentity(context.Background(), p.ID, receipt); !errors.Is(err, errCoordinatorAuthority) {
		t.Fatal("untrusted event read accepted", err)
	}
	wrong := receipt
	wrong.Sequence++
	if _, err = restarted.CoordinationEventByIdentity(orchestratorActor(t, "alice"), p.ID, wrong); !errors.Is(err, ErrOrchestratorConflict) {
		t.Fatal("wrong sequence accepted", err)
	}
	wrong = receipt
	wrong.ArchiveDigest = strings.Repeat("a", 64)
	if _, err = restarted.CoordinationEventByIdentity(orchestratorActor(t, "alice"), p.ID, wrong); !errors.Is(err, ErrOrchestratorConflict) {
		t.Fatal("wrong archive accepted", err)
	}
	coordinationApplyTest(t, restarted, p.ID, "bob", pending.ID, 200)
	current, err := restarted.CoordinationViewForProject(orchestratorActor(t, "alice"), p.ID)
	if err != nil || current.EventSequence != 4 || current.Events[len(current.Events)-1].PreviousEventID != saved.EventHead {
		t.Fatal("new event severed immutable history", err)
	}
}

func TestCoordinationArchiveProtectsIncomingDependencyAndUnknownApply(t *testing.T) {
	for _, mode := range []string{"incoming-dependency", "unknown-apply", "unresolved-question"} {
		t.Run(mode, func(t *testing.T) {
			s, p, first, _, saved := coordinationArchiveFixture(t)
			protected := []string{saved.Intents[0].ID}
			if mode != "incoming-dependency" {
				turns, _ := s.readOrchestrator(p)
				turn := coordinationTurn(turns, first.ID)
				if mode == "unknown-apply" {
					turn.ApplyIntent = "unconfirmed-original-write"
				} else {
					turn.Answer.Blocking = true
					turn.Answer.Question = "Which audience should guide this work?"
				}
				if err := s.saveOrchestrator(turns); err != nil {
					t.Fatal(err)
				}
				protected = nil
			}
			if err := s.compactCoordinationLocked(p, &saved, coordinationRetainThreshold, protected...); err != nil {
				t.Fatal(err)
			}
			if len(saved.Archives) != 0 || len(saved.Intents) != 2 {
				t.Fatal("required origin retired")
			}
		})
	}
}

func TestCoordinationArchiveLateDependencyUsesVerifiedReceipt(t *testing.T) {
	s, p, _, _, saved := coordinationArchiveFixture(t)
	firstID := saved.Intents[0].ID
	coordinationArchiveNow(t, s, p, &saved)
	// The explicit dependency request has no automatic origin; it uses the
	// supported legacy completion seam, then current Edit at registration.
	s.SetCoordinatorAuthorizer(nil)
	turn := coordinationProposalTest(t, s, p.ID, "alice", "purpose", "A later dependent amendment")
	s.SetCoordinatorAuthorizer(func(context.Context, string) error { return nil })
	h := coordinationHandler(s, "alice")
	coordinationRegisterTest(t, h, p.ID, turn.ID, "late-archived-dependency", []string{firstID})
	view, err := s.CoordinationViewForProject(orchestratorActor(t, "alice"), p.ID)
	if err != nil || len(view.HistoricalCauses) != 1 || view.HistoricalCauses[0].ID != firstID || view.HistoricalCauses[0].Status != "applied" {
		t.Fatal("archived cause not verified", err)
	}
	coordinationApplyTest(t, s, p.ID, "alice", turn.ID, 200)
	// A corrupt archive cannot supply a dependency merely through index metadata.
	dir, _ := s.coordinationArchiveDir(p.ID, false)
	if err = os.WriteFile(filepath.Join(dir, saved.Archives[0].Digest+".json"), []byte("{}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CoordinationViewForProject(orchestratorActor(t, "alice"), p.ID); err == nil {
		t.Fatal("corrupt causal witness accepted")
	}
}

func TestCoordinationArchiveCrashBeforePointerAndStagingRemainConservative(t *testing.T) {
	s, p, _, _, saved := coordinationArchiveFixture(t)
	path, _ := s.coordinationPath(p.ID, false)
	before, _ := os.ReadFile(path)
	if err := s.compactCoordinationLocked(p, &saved, coordinationRetainThreshold); err != nil {
		t.Fatal(err)
	}
	// The archive is durable but active publication has not happened.
	current, err := s.readCoordination(p)
	if err != nil || len(current.Intents) != 2 || len(current.Archives) != 0 {
		t.Fatal("uncommitted archive became authoritative", err)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("pre-commit compaction altered active receipt")
	}
	dir, _ := s.coordinationArchiveDir(p.ID, false)
	staging := filepath.Join(dir, ".luci-123")
	if err = os.WriteFile(staging, []byte("incomplete private archive"), 0600); err != nil {
		t.Fatal(err)
	}
	coordinationArchiveNow(t, s, p, &current)
	if current.Archives[0].Digest != saved.Archives[0].Digest {
		t.Fatal("retry duplicated archive identity")
	}
	data, err := os.ReadFile(staging)
	if err != nil || string(data) != "incomplete private archive" {
		t.Fatal("uncommitted staging adopted or erased", err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 2 {
		t.Fatal("bounded retry wrote duplicate pages")
	}
}

func TestCoordinationArchiveTruncatedHeadAndForeignPageRefuse(t *testing.T) {
	for _, fault := range []string{"head", "foreign-scope", "event-origin", "missing-page"} {
		t.Run(fault, func(t *testing.T) {
			s, p, _, _, saved := coordinationArchiveFixture(t)
			receipt := CoordinationEventReceipt{EventID: saved.Events[1].ID, Sequence: 2, OriginIntentID: saved.Intents[0].ID}
			coordinationArchiveNow(t, s, p, &saved)
			switch fault {
			case "head":
				saved.Archives = nil
				if err := s.saveCoordination(saved); err != nil {
					t.Fatal(err)
				}
				if _, err := s.readCoordination(p); err == nil {
					t.Fatal("missing history accepted")
				}
				return
			case "event-origin":
				saved.Archives[0].Events[0].OriginIntentID = saved.Intents[0].ID
				if err := s.saveCoordination(saved); err != nil {
					t.Fatal(err)
				}
			case "foreign-scope":
				saved.ProjectScope = strings.Repeat("b", 64)
				if err := s.saveCoordination(saved); err != nil {
					t.Fatal(err)
				}
			case "missing-page":
				dir, _ := s.coordinationArchiveDir(p.ID, false)
				if err := os.Remove(filepath.Join(dir, saved.Archives[0].Digest+".json")); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := s.CoordinationEventByIdentity(orchestratorActor(t, "alice"), p.ID, receipt); err == nil {
				t.Fatal("invalid archived event accepted")
			}
		})
	}
}

func TestCoordinationRetentionPinsFailedCompletionOrigin(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "coordination-unrecorded")
	s.SetCoordinatorAuthorizer(func(context.Context, string) error { return nil })
	turn := coordinationProposalTest(t, s, p.ID, "alice", "purpose", "A completion awaiting origin durability")
	path, _ := s.coordinationPath(p.ID, false)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	retained, err := s.retainedCoordinationTurnsLocked(p)
	if err != nil || !retained[turn.ID] {
		t.Fatal("failed origin write released original proposal", err)
	}
}

func TestCoordinationArchiveProtectsPaidRequestAfterOrdinaryHistoryRotation(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "coordination-paid-history")
	s.SetCoordinatorAuthorizer(func(context.Context, string) error { return nil })
	ctx := orchestratorActor(t, "alice")
	view, err := s.OrchestratorView(ctx, p.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	req := OrchestratorTurnRequest{ExpectedRevision: view.Revision, ExpectedContextDigest: view.Context.Digest, IdempotencyKey: "original-paid-request", Message: "A durable personal amendment", RouteDigest: strings.Repeat("a", 64)}
	turn, snapshot, existing, err := s.ReserveOrchestratorTurn(ctx, p.ID, req, "assist-original-owned")
	if err != nil || existing {
		t.Fatal(err)
	}
	spec := snapshot.Specification
	spec.Status = "draft"
	spec.Digest = ""
	spec.Purpose = "Retain the actual paid request"
	answer := OrchestratorAnswer{Summary: "Proposed amendment", Proposal: &OrchestratorProposal{Target: "specification", Summary: "Purpose", Specification: &spec}}
	if err = s.CompleteOrchestratorTurn(p.ID, turn.ID, turn.JobID, "succeeded", &answer); err != nil {
		t.Fatal(err)
	}
	coordinationApplyTest(t, s, p.ID, "alice", turn.ID, 200)
	saved, err := s.readCoordination(p)
	if err != nil {
		t.Fatal(err)
	}
	coordinationArchiveNow(t, s, p, &saved)
	for round := 0; round < 3; round++ {
		for i := 0; i < 12; i++ {
			explanation, _ := reserveOrchestratorTest(t, s, p.ID, ctx, "Explain existing context", "")
			if err = s.CompleteOrchestratorTurn(p.ID, explanation.ID, explanation.JobID, "succeeded", &OrchestratorAnswer{Summary: "An explanation without a proposal"}); err != nil {
				t.Fatal(err)
			}
		}
		current, e := s.OrchestratorView(ctx, p.ID, 0)
		if e != nil {
			t.Fatal(e)
		}
		if e = s.rotateOrchestratorHistory(ctx, p.ID, orchestratorRotateRequest{ExpectedRevision: current.Revision, ExpectedStorageDigest: current.StorageDigest, AcknowledgeExportedHistory: true}); e != nil {
			t.Fatal(e)
		}
	}
	active, _ := s.readOrchestrator(p)
	storage, err := s.orchestratorStorageLocked(p, active)
	if err != nil {
		t.Fatal(err)
	}
	if original, _ := findOrchestratorRequest(storage, reviewDigest("alice:"+req.IdempotencyKey), reviewDigest([]any{"alice", req.Message, "", req.ExpectedContextDigest, req.RouteDigest})); original != nil {
		t.Fatal("fixture still has original in ordinary history")
	}
	again, _, existing, err := s.ReserveOrchestratorTurn(ctx, p.ID, req, "assist-must-never-replace")
	if err != nil || !existing || again.ID != turn.ID || again.JobID != turn.JobID || again.AppliedRevision != 1 {
		t.Fatal("archived request created replacement or lost outcome", err)
	}
	resolution, err := s.ResolveOrchestratorRequest(ctx, p.ID, req)
	if err != nil || resolution.Status != "accepted" || resolution.Turn == nil || resolution.Turn.ID != turn.ID {
		t.Fatal("archived request could not resolve", err)
	}
	current, err := s.OrchestratorView(ctx, p.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	s.SetCoordinatorAuthorizer(func(context.Context, string) error { return errors.New("current permission revoked") })
	if err = s.applyOrchestratorProposal(ctx, p.ID, turn.ID, current.Revision, current.Context.Digest); !errors.Is(err, errCoordinatorAuthority) {
		t.Fatal("archive bypassed current authority", err)
	}
	s.SetCoordinatorAuthorizer(func(context.Context, string) error { return nil })
	coordinationApplyTest(t, s, p.ID, "alice", turn.ID, 200)
	confirmed, err := s.OrchestratorView(ctx, p.ID, 0)
	if err != nil || confirmed.Context.Specification.Revision != 1 {
		t.Fatal("archived apply confirmation replayed canonical mutation", err)
	}
	req.Message = "Different paid inference"
	if _, _, _, err = s.ReserveOrchestratorTurn(ctx, p.ID, req, "assist-forged-replacement"); !errors.Is(err, ErrOrchestratorConflict) {
		t.Fatal("archived key accepted changed work", err)
	}
}

func TestCoordinationArchivedOriginContinuesThroughCurrentTypedPreparation(t *testing.T) {
	s, p, h, ctx, req, starts := coordinatorContinuationFixture(t)
	turn, snapshot := reserveOrchestratorTest(t, s, p.ID, ctx, "Amend the reviewed task title", "")
	plan := snapshot.Plan
	plan.Tasks = append([]ProjectTask{}, plan.Tasks...)
	plan.Tasks[0].Title = "Implement reviewed saving"
	answer := OrchestratorAnswer{Summary: "A routine reviewed-plan amendment", Proposal: &OrchestratorProposal{Target: "plan", Summary: "Clarify the task title", Plan: &plan}}
	if err := s.CompleteOrchestratorTurn(p.ID, turn.ID, turn.JobID, "succeeded", &answer); err != nil {
		t.Fatal(err)
	}
	coordinationApplyTest(t, s, p.ID, "alice", turn.ID, 200)
	saved, err := s.readCoordination(p)
	if err != nil {
		t.Fatal(err)
	}
	coordinationArchiveNow(t, s, p, &saved)
	origins, err := s.readOrchestrator(p)
	if err != nil {
		t.Fatal(err)
	}
	kept := []storedOrchestratorTurn{}
	for _, row := range origins.Turns {
		if row.ID != turn.ID {
			kept = append(kept, row)
		}
	}
	origins.Turns = kept
	if err = s.saveOrchestrator(origins); err != nil {
		t.Fatal(err)
	}
	current, err := s.OrchestratorView(ctx, p.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	req.OriginTurnID = turn.ID
	req.ExpectedContextDigest = current.Context.Digest
	req.Run.ImplementationContext.PlanRevision = current.Context.Plan.Revision
	out := coordinatorCreate(t, h, p, req, 201)
	if out.OriginTurnID != turn.ID || out.Status != "queued" || out.RunID == "" || *starts != 0 {
		t.Fatal("archived origin failed typed preparation or dispatched early")
	}
	again := coordinatorCreate(t, h, p, req, 200)
	if again.ID != out.ID || *starts != 0 {
		t.Fatal("archived continuation replayed preparation")
	}
}
