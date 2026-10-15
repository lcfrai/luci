package product

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

func contractFixture(t *testing.T) (*Server, http.Handler, projectRunCreateRequest, string, string, *ProjectRunSelection) {
	t.Helper()
	s, mux, p, req, selected, _ := projectRunHTTPFixture(t)
	s.registerSwarmScheduler(mux)
	s.SetCoordinatorAuthorizer(func(context.Context, string) error { return nil })
	actor := contextHistoryActorHandler(mux, management.User{ID: "contract-owner", Username: "owner", Role: "member"})
	return s, actor, req, "/v1/product/projects/" + p.ID, p.Path, selected
}

func contractRequest(req projectRunCreateRequest) OrchestrationPrepareRequest {
	return OrchestrationPrepareRequest{IdempotencyKey: "prepare-contract-key-123456", WorktreeID: req.WorktreeID, ImplementationContext: req.ImplementationContext,
		Route: OrchestrationRouteRequest{Harness: req.Harness, Model: req.Model, PermissionMode: req.PermissionMode, ExecutionLocation: "local-docker", AgentConnectionID: req.AgentConnectionID}, Outcome: req.Outcome}
}

func contractResponse(t *testing.T, body []byte) OrchestrationContractView {
	t.Helper()
	var view OrchestrationContractView
	if err := json.Unmarshal(body, &view); err != nil {
		t.Fatal(err)
	}
	return view
}

func TestOrchestrationContractPrepareApproveCaptureAndRestart(t *testing.T) {
	s, handler, req, base, projectPath, _ := contractFixture(t)
	prepare := contractRequest(req)
	endpoint := base + "/orchestration-contracts"
	view := contractResponse(t, journeyRequest(t, handler, "POST", endpoint+"/prepare", prepare, 201).Body.Bytes())
	before, err := s.projectRuns.List(req.ImplementationContext.ProjectID, projectPath)
	if err != nil || len(before) != 0 {
		t.Fatal("preparation created a run", err, len(before))
	}
	if view.Contract.State != "prepared" || view.Freshness.Status != "current" || view.Contract.Approval != nil || len(view.Contract.Input.Tasks) != 1 || view.Contract.Input.Standards.Status != "legacy-unassessed" && view.Contract.Input.Standards.Status != "not-configured" {
		t.Fatal("prepare did not capture reviewed Design without executing", view)
	}
	replay := contractResponse(t, journeyRequest(t, handler, "POST", endpoint+"/prepare", prepare, 200).Body.Bytes())
	if !reflect.DeepEqual(view.Contract, replay.Contract) {
		t.Fatal("request replay changed contract identity")
	}
	changed := prepare
	changed.Outcome = "publish"
	journeyRequest(t, handler, "POST", endpoint+"/prepare", changed, 409)
	journeyRequest(t, handler, "POST", endpoint+"/"+view.Contract.ID+"/approve", map[string]any{"expectedRevision": 1, "expectedDigest": "wrong"}, 409)
	approved := contractResponse(t, journeyRequest(t, handler, "POST", endpoint+"/"+view.Contract.ID+"/approve", map[string]any{"expectedRevision": 1, "expectedDigest": view.Contract.Digest}, 200).Body.Bytes())
	if approved.Contract.State != "approved" || approved.Contract.Revision != 2 || approved.Contract.Approval == nil || approved.Contract.Approval.Digest != view.Contract.Digest {
		t.Fatal(approved)
	}
	again := contractResponse(t, journeyRequest(t, handler, "POST", endpoint+"/"+view.Contract.ID+"/approve", map[string]any{"expectedRevision": 1, "expectedDigest": view.Contract.Digest}, 200).Body.Bytes())
	if again.Contract.Approval.ID != approved.Contract.Approval.ID {
		t.Fatal("approval replay minted a new receipt")
	}
	selection := &OrchestrationContractSelection{ID: approved.Contract.ID, Revision: approved.Contract.Revision, Digest: approved.Contract.Digest, ApprovalID: approved.Contract.Approval.ID}
	req.OrchestrationContract = selection
	run := projectRunHTTPResponse(t, journeyRequest(t, handler, "POST", base+"/runs", req, 201))
	if run.Input.OrchestrationContract == nil || !reflect.DeepEqual(*run.Input.OrchestrationContract, approved.Contract) {
		t.Fatal("run did not freeze complete approved contract")
	}
	stored, err := s.projectRuns.Get(req.ImplementationContext.ProjectID, projectPath, run.ID)
	if err != nil || stored.Input.OrchestrationContract == nil {
		t.Fatal(err)
	}
	reopened, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot()})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	_, retained, err := reopened.readOrchestrationContracts(project.Project{ID: req.ImplementationContext.ProjectID, Path: projectPath})
	if err != nil || len(retained.Records) != 1 || !reflect.DeepEqual(retained.Records[0].Contract, approved.Contract) {
		t.Fatal("contract changed across restart", err)
	}
}

func TestOrchestrationContractRevokedAuthorityAndChangedPolicyFence(t *testing.T) {
	s, handler, req, base, _, selected := contractFixture(t)
	endpoint := base + "/orchestration-contracts"
	prepare := contractRequest(req)
	s.SetCoordinatorAuthorizer(func(context.Context, string) error { return errors.New("revoked") })
	journeyRequest(t, handler, "POST", endpoint+"/prepare", prepare, 403)
	s.SetCoordinatorAuthorizer(func(context.Context, string) error { return nil })
	view := contractResponse(t, journeyRequest(t, handler, "POST", endpoint+"/prepare", prepare, 201).Body.Bytes())
	s.SetCoordinatorAuthorizer(func(context.Context, string) error { return errors.New("revoked") })
	journeyRequest(t, handler, "POST", endpoint+"/"+view.Contract.ID+"/approve", map[string]any{"expectedRevision": 1, "expectedDigest": view.Contract.Digest}, 403)
	s.SetCoordinatorAuthorizer(func(context.Context, string) error { return nil })
	s.swarmScheduler.mu.Lock()
	policy, err := s.readSwarmScheduler()
	if err == nil {
		policy.PolicyRevision++
		policy.Policy.MaxActive++
		err = s.saveSwarmScheduler(&policy)
	}
	s.swarmScheduler.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	stale := contractResponse(t, journeyRequest(t, handler, "GET", endpoint+"/"+view.Contract.ID, nil, 200).Body.Bytes())
	if stale.Freshness.Status != "changed" {
		t.Fatal("policy change retained fresh contract", stale)
	}
	journeyRequest(t, handler, "POST", endpoint+"/"+view.Contract.ID+"/approve", map[string]any{"expectedRevision": 1, "expectedDigest": view.Contract.Digest}, 409)
	// A second prepared contract with the same project but a changed saved
	// connection must refuse approval rather than silently route elsewhere.
	prepare.IdempotencyKey = "prepare-route-key-123456789"
	other := contractResponse(t, journeyRequest(t, handler, "POST", endpoint+"/prepare", prepare, 201).Body.Bytes())
	selected.Connection.Revision++
	journeyRequest(t, handler, "POST", endpoint+"/"+other.Contract.ID+"/approve", map[string]any{"expectedRevision": 1, "expectedDigest": other.Contract.Digest}, 409)
}

func TestOrchestrationContractChangedPlanCannotBeApproved(t *testing.T) {
	_, handler, req, base, _, _ := contractFixture(t)
	endpoint := base + "/orchestration-contracts"
	view := contractResponse(t, journeyRequest(t, handler, "POST", endpoint+"/prepare", contractRequest(req), 201).Body.Bytes())
	journeyRequest(t, handler, "POST", base+"/plan", map[string]any{
		"expectedRevision":              req.ImplementationContext.PlanRevision,
		"expectedSpecificationRevision": req.ImplementationContext.SpecificationRevision,
		"tasks":                         []ProjectTask{{ID: "task-save", Title: "Revised saving task", Status: "todo", RequirementIDs: []string{"AC-save"}}},
	}, 200)
	changed := contractResponse(t, journeyRequest(t, handler, "GET", endpoint+"/"+view.Contract.ID, nil, 200).Body.Bytes())
	if changed.Freshness.Status != "changed" {
		t.Fatal(changed)
	}
	journeyRequest(t, handler, "POST", endpoint+"/"+view.Contract.ID+"/approve", map[string]any{"expectedRevision": 1, "expectedDigest": view.Contract.Digest}, 409)
}

func TestOrchestrationContractChangedSourceIsVisibleAndCannotBeApproved(t *testing.T) {
	_, handler, req, base, projectPath, _ := contractFixture(t)
	endpoint := base + "/orchestration-contracts"
	view := contractResponse(t, journeyRequest(t, handler, "POST", endpoint+"/prepare", contractRequest(req), 201).Body.Bytes())
	if err := os.WriteFile(filepath.Join(projectPath, "new-implementation.txt"), []byte("changed source"), 0600); err != nil {
		t.Fatal(err)
	}
	changed := contractResponse(t, journeyRequest(t, handler, "GET", endpoint+"/"+view.Contract.ID, nil, 200).Body.Bytes())
	if changed.Freshness.Status != "changed" {
		t.Fatal("source drift was not visible to the reviewer", changed.Freshness)
	}
	journeyRequest(t, handler, "POST", endpoint+"/"+view.Contract.ID+"/approve", map[string]any{"expectedRevision": 1, "expectedDigest": view.Contract.Digest}, 409)
}

func TestOrchestrationContractQueuedTaskBlockedWithoutChangingRunCapture(t *testing.T) {
	s, handler, req, base, projectPath, selected := contractFixture(t)
	s.SetProjectSwarmPendingLaunch(func(context.Context, string, string, string, string) (bool, error) { return false, nil })
	s.branchSessions = func(context.Context, string, string) (bool, error) { return false, nil }
	req.PermissionMode, selected.PermissionMode = "bypass", "bypass"
	prepare := contractRequest(req)
	prepare.Route.PermissionMode = "bypass"
	endpoint := base + "/orchestration-contracts"
	view := contractResponse(t, journeyRequest(t, handler, "POST", endpoint+"/prepare", prepare, 201).Body.Bytes())
	approved := contractResponse(t, journeyRequest(t, handler, "POST", endpoint+"/"+view.Contract.ID+"/approve", map[string]any{"expectedRevision": 1, "expectedDigest": view.Contract.Digest}, 200).Body.Bytes())
	req.OrchestrationContract = &OrchestrationContractSelection{ID: approved.Contract.ID, Revision: 2, Digest: approved.Contract.Digest, ApprovalID: approved.Contract.Approval.ID}
	run := projectRunHTTPResponse(t, journeyRequest(t, handler, "POST", base+"/runs", req, 201)).ProjectRun
	run = projectRunHTTPResponse(t, journeyRequest(t, handler, "POST", base+"/runs/"+run.ID+"/swarm", map[string]int{"expectedRevision": run.Revision, "maxParallel": 1}, 201)).ProjectRun
	starts := 0
	s.swarmScheduler.runtime = SwarmSchedulerRuntime{CaptureActor: management.CaptureExecutionActor, Admit: func(context.Context, SwarmAdmissionRequest) (SwarmAdmissionResult, error) {
		starts++
		return SwarmAdmissionResult{State: "admitted"}, nil
	}, Dispatch: func(context.Context, SwarmDispatchRequest) (SwarmDispatchResult, error) {
		starts++
		return SwarmDispatchResult{State: "running"}, nil
	}, Release: func(context.Context, string) error { return nil }}
	var queued struct {
		Items []SwarmQueueItem `json:"items"`
	}
	decodeEvidenceResponse(t, journeyRequest(t, handler, "POST", base+"/runs/"+run.ID+"/swarm/queue", map[string]any{"expectedRevision": run.Revision, "idempotencyKey": "contract-queue-key-123456", "priority": "normal", "taskIds": []string{"task-save"}}, 201).Body.Bytes(), &queued)
	if len(queued.Items) != 1 {
		t.Fatal(queued)
	}
	immutable := *run.Input.OrchestrationContract
	s.swarmScheduler.mu.Lock()
	policy, err := s.readSwarmScheduler()
	if err == nil {
		policy.PolicyRevision++
		policy.Policy.MaxActive++
		err = s.saveSwarmScheduler(&policy)
	}
	s.swarmScheduler.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RunSwarmSchedulerOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	after, err := s.ProjectSwarmQueueSnapshot(req.ImplementationContext.ProjectID)
	if err != nil || starts != 0 || len(after.Items) != 1 || after.Items[0].State != "blocked" {
		t.Fatal("stale task dispatched", starts, after, err)
	}
	retained, err := s.projectRuns.Get(req.ImplementationContext.ProjectID, projectPath, run.ID)
	if err != nil || !reflect.DeepEqual(*retained.Input.OrchestrationContract, immutable) {
		t.Fatal("captured run contract changed", err)
	}
}

func TestOrchestrationContractCurrentTeamAuthorityAtEffectTime(t *testing.T) {
	policy := management.Policy{MonthlyBudgetUSD: 30, SessionMemoryMiB: 1024, AllowedHarnesses: []string{"codex"}}
	team := &management.TeamAccess{Team: management.Team{ID: "team-exact", Revision: 4}, Member: management.TeamMember{Revision: 7}, ProjectRevision: 11}
	contract := &OrchestrationContract{Input: OrchestrationPreparedInput{Controls: OrchestrationControls{AuthorityPolicyDigest: reviewDigest(policy), TeamID: team.Team.ID, TeamRevision: 4, MemberRevision: 7, ProjectGrantRevision: 11}}}
	run := ProjectRun{Input: ProjectRunInput{OrchestrationContract: contract}}
	authority := management.ExecutionAuthority{Policy: policy, Team: team}
	if !OrchestrationAuthorityMatches(run, authority) {
		t.Fatal("current team authority refused")
	}
	team.ProjectRevision++
	if OrchestrationAuthorityMatches(run, authority) {
		t.Fatal("changed project grant reused approval")
	}
	team.ProjectRevision--
	authority.Policy.MonthlyBudgetUSD++
	if OrchestrationAuthorityMatches(run, authority) {
		t.Fatal("changed cost policy reused approval")
	}
}
