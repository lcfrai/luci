package product

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

func overviewFixture(t *testing.T) (*Server, *http.ServeMux) {
	t.Helper()
	s, mux := journeyFixture(t)
	s.registerProjectOverview(s.guardProjectSourceRoutes(mux))
	return s, mux
}
func overviewGet(t *testing.T, mux http.Handler, id string) ProjectOverview {
	t.Helper()
	var view ProjectOverview
	data := journeyRequest(t, mux, "GET", "/v1/product/projects/"+id+"/overview", nil, 200).Body.Bytes()
	if err := json.Unmarshal(data, &view); err != nil {
		t.Fatal(err)
	}
	return view
}
func TestProjectOverviewShowsAcceptedTeamChangeAcrossRestartWithoutBorrowingAnotherProject(t *testing.T) {
	s, mux := overviewFixture(t)
	p := createJourneyProject(t, mux, "shared-overview")
	other := createJourneyProject(t, mux, "unrelated-overview")
	s.SetCoordinatorAuthorizer(func(context.Context, string) error { return nil })
	turn := coordinationProposalTest(t, s, p.ID, "bob", "purpose", "A shared reading list")
	coordinationRegisterTest(t, coordinationHandler(s, "bob"), p.ID, turn.ID, "overview-event", nil)
	coordinationApplyTest(t, s, p.ID, "bob", turn.ID, 200)
	got := overviewGet(t, mux, p.ID)
	if got.SharedChange.Status != "recorded" || got.SharedChange.Event == nil ||
		got.SharedChange.Event.Actor.ID != "bob" || got.SharedChange.Event.OriginTurnID != turn.ID ||
		got.SharedChange.Event.AppliedTargetDigest == "" || got.SharedChange.Event.AcceptedContextDigest != "" {
		t.Fatalf("project summary lost or overstated accepted team change: %+v", got.SharedChange)
	}
	if foreign := overviewGet(t, mux, other.ID); foreign.SharedChange.Status != "none" || foreign.SharedChange.Event != nil {
		t.Fatalf("another project borrowed a team change: %+v", foreign.SharedChange)
	}
	restarted, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot()})
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	second := http.NewServeMux()
	restarted.registerProjectOverview(second)
	again := overviewGet(t, second, p.ID)
	if !reflect.DeepEqual(again.SharedChange, got.SharedChange) {
		t.Fatalf("read-only restart lost accepted project activity: %+v", again.SharedChange)
	}
}
func TestProjectOverviewFourProjectsKeepContextAndQuestionsSeparate(t *testing.T) {
	s, mux := overviewFixture(t)
	projects := make([]project.Project, 4)
	for i := range projects {
		p := createJourneyProject(t, mux, fmt.Sprintf("overview-project-%d", i))
		projects[i] = p
		state := forgedReviewState(p.ID)
		state.Brief.Purpose = fmt.Sprintf("Only project %d owns this outcome", i)
		state.Specification.Questions = []SpecificationQuestion{{ID: "open-decision", Question: fmt.Sprintf("Who reviews project %d?", i)}, {ID: "answered", Question: "Already decided?", Answer: "Yes"}}
		state.Plan.Tasks[0].Status = "done"
		if err := writeJourney(p.Path, state); err != nil {
			t.Fatal(err)
		}
	}
	for i, p := range projects {
		got := overviewGet(t, mux, p.ID)
		if got.ProjectID != p.ID || got.Status != "ready" || got.Goal.Text != fmt.Sprintf("Only project %d owns this outcome", i) || got.Context.Tasks.RecordedDone != 1 || got.Context.DefinitionStatus == "approved" || got.Phase.ID != "define" {
			t.Fatalf("wrong source/authority: %+v", got)
		}
		if got.Questions.Count == nil || *got.Questions.Count != 1 || len(got.Questions.Items) != 1 || got.Questions.Items[0].Text != fmt.Sprintf("Who reviews project %d?", i) || got.NextAction.ID != "answer-question" {
			t.Fatalf("wrong question scope: %+v", got.Questions)
		}
		if got.Execution.Status != "recorded" || got.Execution.Queued == nil || *got.Execution.Queued != 0 {
			t.Fatalf("new empty queue not represented: %+v", got.Execution)
		}
	}
	// Concurrent identical reads share a single compact entry per source.
	var wg sync.WaitGroup
	for _, p := range projects {
		for range 4 {
			wg.Add(1)
			go func(p project.Project) {
				defer wg.Done()
				got, err := s.cachedProjectOverview(context.Background(), p)
				if err != nil || got.ProjectID != p.ID {
					t.Errorf("cross-project cache result")
				}
			}(p)
		}
	}
	wg.Wait()
	if len(s.projectOverview.entries) != 4 {
		t.Fatalf("cache entries %d", len(s.projectOverview.entries))
	}
}
func TestProjectOverviewInvalidatesAmendmentsAndPrivateApprovals(t *testing.T) {
	s, mux := overviewFixture(t)
	p := createJourneyProject(t, mux, "overview-amendment")
	spec, _ := reviewDeploymentFixture(t, mux, p.ID)
	got := overviewGet(t, mux, p.ID)
	if got.Context.DefinitionStatus != "approved" || got.Context.DesignStatus != "approved" || got.Phase.ID != "develop" {
		t.Fatalf("actual private approval missing: %+v", got.Context)
	}
	approval, err := s.approvalPath(p.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(approval); err != nil {
		t.Fatal(err)
	}
	got = overviewGet(t, mux, p.ID)
	if got.Context.DefinitionStatus == "approved" || got.Context.DesignStatus == "approved" {
		t.Fatal("cached private approval survived revocation")
	}
	state, err := readJourney(p.Path, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	state.Brief.Revision++
	state.Brief.Purpose = "An explicitly amended outcome"
	state.Brief.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if err = writeJourney(p.Path, state); err != nil {
		t.Fatal(err)
	}
	got = overviewGet(t, mux, p.ID)
	if got.Goal.Text != state.Brief.Purpose || got.Goal.Revision != state.Brief.Revision || got.Context.SpecificationRevision != spec.Revision || got.Context.DefinitionStatus != "outdated" {
		t.Fatalf("amendment was stale: %+v", got)
	}
}
func TestProjectOverviewCorruptContextAndQueueRemainUnknown(t *testing.T) {
	s, mux := overviewFixture(t)
	p := createJourneyProject(t, mux, "overview-corrupt")
	if err := writeJourney(p.Path, forgedReviewState(p.ID)); err != nil {
		t.Fatal(err)
	}
	_ = overviewGet(t, mux, p.ID)
	if err := os.WriteFile(filepath.Join(p.Path, ".luci", "state.json"), []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.stateDir, "swarm-scheduler.json"), []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	got := overviewGet(t, mux, p.ID)
	if got.Status != "unavailable" || got.Goal != nil || got.Context != nil || got.Questions.Count != nil || got.Execution.Queued != nil || got.Execution.Running != nil || got.Execution.Status != "unavailable" || len(got.Limitations) < 2 {
		t.Fatalf("corruption became fabricated progress: %+v", got)
	}
}
func TestProjectOverviewPrivateQuestionsSurviveRestartAndRejectCopiedProjection(t *testing.T) {
	s, mux := overviewFixture(t)
	p := createJourneyProject(t, mux, "overview-discussion")
	ctx := orchestratorActor(t, "overview-owner")
	turn, _ := reserveOrchestratorTest(t, s, p.ID, ctx, "Clarify project visibility", "")
	answer := OrchestratorAnswer{Summary: "Visibility is unresolved", Question: "Who can read this project's records?", Blocking: true, Rationale: "This affects access control."}
	if err := s.CompleteOrchestratorTurn(p.ID, turn.ID, turn.JobID, "succeeded", &answer); err != nil {
		t.Fatal(err)
	}
	got := overviewGet(t, mux, p.ID)
	if got.Questions.Count == nil || *got.Questions.Count != 1 || got.Questions.Items[0].Source != "orchestrator" || got.Questions.Items[0].ID != turn.ID {
		t.Fatalf("private question missing: %+v", got.Questions)
	}
	restarted, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot()})
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	nextMux := http.NewServeMux()
	restarted.registerProjectOverview(restarted.guardProjectSourceRoutes(nextMux))
	restored := overviewGet(t, nextMux, p.ID)
	if !reflect.DeepEqual(restored.Questions, got.Questions) {
		t.Fatalf("restart lost questions: %+v", restored.Questions)
	}
	other := createJourneyProject(t, mux, "overview-other")
	source, _ := s.overviewQuestionsPath(p.ID, false)
	data, _ := os.ReadFile(source)
	dest, _ := s.overviewQuestionsPath(other.ID, true)
	if err = os.WriteFile(dest, data, 0600); err != nil {
		t.Fatal(err)
	}
	// An unrelated private sidecar alone cannot invent a conversation.
	foreign := overviewGet(t, mux, other.ID)
	if foreign.Questions.Count == nil || *foreign.Questions.Count != 0 || len(foreign.Questions.Items) != 0 {
		t.Fatal("foreign question leaked")
	}
	// Copying both private files is still rejected by account/project scopes.
	ledger, _ := s.orchestratorPath(p.ID, false)
	bytes, _ := os.ReadFile(ledger)
	foreignLedger, _ := s.orchestratorPath(other.ID, true)
	if err = os.WriteFile(foreignLedger, bytes, 0600); err != nil {
		t.Fatal(err)
	}
	foreign = overviewGet(t, mux, other.ID)
	if foreign.Questions.Count != nil || foreign.Status != "partial" || len(foreign.Questions.Items) != 0 {
		t.Fatal("copied private conversation trusted")
	}
	// Answering the original question updates the projection and cached attention.
	response, _ := reserveOrchestratorTest(t, s, p.ID, ctx, "Only assigned staff can read records", turn.ID)
	if err = s.CompleteOrchestratorTurn(p.ID, response.ID, response.JobID, "succeeded", &OrchestratorAnswer{Summary: "Visibility resolved."}); err != nil {
		t.Fatal(err)
	}
	resolved := overviewGet(t, mux, p.ID)
	if resolved.Questions.Count == nil || *resolved.Questions.Count != 0 {
		t.Fatalf("resolved question retained: %+v", resolved.Questions)
	}
}
func TestProjectOverviewLegacyLargeDialogueIsPartialWithoutMigrationWrites(t *testing.T) {
	s, mux := overviewFixture(t)
	p := createJourneyProject(t, mux, "overview-large-dialogue")
	ledger, err := s.orchestratorPath(p.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	// An over-budget old ledger is not decoded, repaired or migrated on a GET.
	if err = os.WriteFile(ledger, []byte(strings.Repeat(" ", overviewLegacyConversationLimit+1)), 0600); err != nil {
		t.Fatal(err)
	}
	beforeState, beforeProject := auditTree(t, s.stateDir), auditTree(t, p.Path)
	for _, method := range []string{"GET", "HEAD"} {
		journeyRequest(t, mux, method, "/v1/product/projects/"+p.ID+"/overview", nil, 200)
	}
	got := overviewGet(t, mux, p.ID)
	if got.Status != "partial" || got.Questions.Count != nil || got.Questions.Omitted != nil || len(got.Limitations) == 0 {
		t.Fatalf("unread history became zero questions: %+v", got)
	}
	if !reflect.DeepEqual(beforeState, auditTree(t, s.stateDir)) || !reflect.DeepEqual(beforeProject, auditTree(t, p.Path)) {
		t.Fatal("overview wrote or repaired private/project state")
	}
}
func TestProjectOverviewRequiresProjectReadBeforePrivateCache(t *testing.T) {
	s, mux := overviewFixture(t)
	p := createJourneyProject(t, mux, "overview-access")
	s.runner = func(context.Context, string, []string, string) (string, error) {
		t.Fatal("overview must not run a provider or process")
		return "", nil
	}
	for _, access := range []management.TeamAccess{{ProjectID: p.ID}, {ProjectID: "another-project", Permissions: management.TeamPermissions{Read: true}}} {
		req := httptest.NewRequest("GET", "/v1/product/projects/"+p.ID+"/overview", nil)
		req = req.WithContext(management.WithTeamAccess(req.Context(), access))
		out := httptest.NewRecorder()
		mux.ServeHTTP(out, req)
		if out.Code != 403 {
			t.Fatalf("unauthorized read returned %d", out.Code)
		}
		if len(s.projectOverview.entries) != 0 {
			t.Fatal("private context read before permission check")
		}
	}
	req := httptest.NewRequest("GET", "/v1/product/projects/"+p.ID+"/overview", nil)
	req = req.WithContext(management.WithTeamAccess(req.Context(), management.TeamAccess{ProjectID: p.ID, Permissions: management.TeamPermissions{Read: true}}))
	out := httptest.NewRecorder()
	mux.ServeHTTP(out, req)
	if out.Code != 200 {
		t.Fatalf("read-only viewer rejected: %d %s", out.Code, out.Body.String())
	}
}
func TestProjectOverviewCacheAndTextAreBounded(t *testing.T) {
	s, _ := overviewFixture(t)
	// Independent concrete project roots exercise the real fill/eviction path
	// without allocating a Git history, process, or source tree per cache entry.
	for i := 0; i < projectOverviewCacheLimit+5; i++ {
		dir := filepath.Join(t.TempDir(), "source")
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
		p := project.Project{ID: project.ProjectID(dir), Path: dir}
		state := forgedReviewState(p.ID)
		state.Brief.Purpose = strings.Repeat("界", 2000)
		if err := writeJourney(dir, state); err != nil {
			t.Fatal(err)
		}
		got, err := s.cachedProjectOverview(context.Background(), p)
		if err != nil {
			t.Fatal(err)
		}
		if got.Goal == nil || !got.Goal.Truncated || len(got.Goal.Text) > projectOverviewTextLimit {
			t.Fatalf("unbounded goal: %+v", got.Goal)
		}
		if len(s.projectOverview.entries) > projectOverviewCacheLimit {
			t.Fatal("cache grew beyond its project cap")
		}
	}
	if len(s.projectOverview.entries) != projectOverviewCacheLimit {
		t.Fatalf("unexpected retained count %d", len(s.projectOverview.entries))
	}
}

func TestProjectOverviewLargeCurrentDialogueUsesBoundedIdentityCheckedProjection(t *testing.T) {
	s, mux := overviewFixture(t)
	p := createJourneyProject(t, mux, "overview-indexed-dialogue")
	state := forgedReviewState(p.ID)
	state.Brief.Constraints = strings.Repeat("context ", 7500)
	state.Specification.Constraints = state.Brief.Constraints
	if err := writeJourney(p.Path, state); err != nil {
		t.Fatal(err)
	}
	ctx := orchestratorActor(t, "overview-large-owner")
	for i := 0; i < 3; i++ {
		turn, _ := reserveOrchestratorTest(t, s, p.ID, ctx, fmt.Sprintf("Clarify decision %d", i), "")
		answer := OrchestratorAnswer{Summary: "A decision is needed", Question: fmt.Sprintf("Who approves decision %d?", i), Blocking: true}
		if err := s.CompleteOrchestratorTurn(p.ID, turn.ID, turn.JobID, "succeeded", &answer); err != nil {
			t.Fatal(err)
		}
	}
	ledger, _ := s.orchestratorPath(p.ID, false)
	info, err := os.Stat(ledger)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() <= overviewLegacyConversationLimit {
		t.Fatalf("fixture must exceed fallback budget, got %d", info.Size())
	}
	projection, _ := s.overviewQuestionsPath(p.ID, false)
	bytes, err := os.ReadFile(projection)
	if err != nil {
		t.Fatal(err)
	}
	if len(bytes) > overviewQuestionProjectionLimit || strings.Contains(string(bytes), "context context") || strings.Contains(string(bytes), "keyDigest") {
		t.Fatal("projection retained prompt or context")
	}
	got := overviewGet(t, mux, p.ID)
	if got.Status != "ready" || got.Questions.Count == nil || *got.Questions.Count != 3 {
		t.Fatalf("large dialogue index unavailable: %+v", got.Questions)
	}
	// Replacing the ledger with equal bytes and restored timestamps must still
	// invalidate its private sidecar through the opened file's native identity.
	ledgerBytes, err := os.ReadFile(ledger)
	if err != nil {
		t.Fatal(err)
	}
	replacement := ledger + ".replacement"
	if err = os.WriteFile(replacement, ledgerBytes, 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.Chtimes(replacement, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(replacement, ledger); err != nil {
		t.Fatal(err)
	}
	got = overviewGet(t, mux, p.ID)
	if got.Status != "partial" || got.Questions.Count != nil {
		t.Fatal("replaced ledger retained stale indexed claims")
	}
	before := auditTree(t, s.stateDir)
	_ = overviewGet(t, mux, p.ID)
	if !reflect.DeepEqual(before, auditTree(t, s.stateDir)) {
		t.Fatal("read repaired stale index")
	}
}
func TestProjectOverviewQuestionAndResponseBounds(t *testing.T) {
	s, mux := overviewFixture(t)
	p := createJourneyProject(t, mux, "overview-many-decisions")
	state := forgedReviewState(p.ID)
	// Public imported metadata is not restricted to timestamps by readJourney.
	// It must not turn a compact response/cache entry into a retained MiB string.
	state.Brief.UpdatedAt = strings.Repeat("x", 1<<20)
	for i := 0; i < 8; i++ {
		state.Specification.Questions = append(state.Specification.Questions, SpecificationQuestion{ID: fmt.Sprintf("decision-%d", i), Question: strings.Repeat("界", 700)})
	}
	if err := writeJourney(p.Path, state); err != nil {
		t.Fatal(err)
	}
	got := overviewGet(t, mux, p.ID)
	if got.Goal == nil || got.Goal.UpdatedAt != "" {
		t.Fatal("invalid oversized historical timestamp retained")
	}
	if got.Questions.Count == nil || *got.Questions.Count != 8 || got.Questions.Omitted == nil || *got.Questions.Omitted != 3 || len(got.Questions.Items) != 5 {
		t.Fatalf("question counts: %+v", got.Questions)
	}
	for _, q := range got.Questions.Items {
		if len(q.Text) > projectOverviewTextLimit || !q.Truncated {
			t.Fatal("question text not bounded")
		}
	}
	data, _ := json.Marshal(got)
	if len(data) > 16<<10 {
		t.Fatalf("overview unexpectedly large: %d", len(data))
	}
	other, err := New(Options{StateDir: t.TempDir(), ProjectsDir: s.projectRoot()})
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	scoped, err := other.cachedProjectOverview(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	if scoped.Context.DefinitionStatus == "approved" || scoped.ProjectID != p.ID {
		t.Fatal("account cached authority crossed scopes")
	}
}

func TestProjectOverviewQueueCountsStayProjectScopedAndRefreshOutsideContextCache(t *testing.T) {
	s, mux := overviewFixture(t)
	row, err := s.readSwarmScheduler()
	if err != nil {
		t.Fatal(err)
	}
	states := []string{"queued", "running", "blocked", "unknown"}
	projects := []project.Project{}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for i, status := range states {
		p := createJourneyProject(t, mux, fmt.Sprintf("overview-queue-%d", i))
		projects = append(projects, p)
		row.Items = append(row.Items, swarmQueueRecord{SwarmQueueItem: SwarmQueueItem{ID: newID("queue-"), ProjectID: p.ID, RunID: newID("run-"), TaskID: "same-task-id", TaskTitle: "Recorded queue fixture", Priority: "normal", State: status, Revision: 1, QueuedAt: now, UpdatedAt: now}, Actor: management.ExecutionActorRef{Local: true, ProjectID: p.ID}, InputDigest: strings.Repeat("a", 64), KeyDigest: strings.Repeat("b", 64), BatchKeyDigest: strings.Repeat("c", 64), RequestDigest: strings.Repeat("d", 64)})
	}
	if err = s.saveSwarmScheduler(&row); err != nil {
		t.Fatal(err)
	}
	s.swarmScheduler.runtime.Resources = func(context.Context) any { t.Fatal("project overview sampled machine resources"); return nil }
	for i, p := range projects {
		got := overviewGet(t, mux, p.ID)
		counts := []*int{got.Execution.Queued, got.Execution.Running, got.Execution.Blocked, got.Execution.Unknown}
		for j, count := range counts {
			want := 0
			if i == j {
				want = 1
			}
			if count == nil || *count != want {
				t.Fatalf("project %d received foreign queue %d: %+v", i, j, got.Execution)
			}
		}
		if got.Execution.Scope != "swarm-queue" || got.Execution.Status != "recorded" {
			t.Fatal("recorded queue state confused with process observation")
		}
	}
	// The canonical context is still cached, but new queue state must be visible
	// immediately without launching a model, sampling the host or mutating a run.
	row.Items[0].State = "paused"
	if err = s.saveSwarmScheduler(&row); err != nil {
		t.Fatal(err)
	}
	before := auditTree(t, s.stateDir)
	got := overviewGet(t, mux, projects[0].ID)
	if got.Execution.Queued == nil || *got.Execution.Queued != 0 || got.Execution.Paused == nil || *got.Execution.Paused != 1 || !reflect.DeepEqual(before, auditTree(t, s.stateDir)) {
		t.Fatalf("queue was cached/stale or GET mutated state: %+v", got.Execution)
	}
}
