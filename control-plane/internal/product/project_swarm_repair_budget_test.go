package product

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestAutomaticRepairLimitPolicyHTTPPreservesZeroAndRejectsInvalid(t *testing.T) {
	s, p, handler, _ := coordinatorPolicyFixture(t)
	url := "/v1/product/projects/" + p.ID + "/orchestrator/policy"
	var initial CoordinatorPolicy
	decodeEvidenceResponse(t, journeyRequest(t, handler, http.MethodGet, url, nil, 200).Body.Bytes(), &initial)
	if initial.AutomaticRepairLimit != nil {
		t.Fatal("legacy absent policy was normalized")
	}
	limit, err := effectiveAutomaticRepairLimit(initial.AutomaticRepairLimit)
	if err != nil || limit != 2 {
		t.Fatal("legacy default changed", limit, err)
	}
	var saved CoordinatorPolicy
	decodeEvidenceResponse(t, journeyRequest(t, handler, http.MethodPost, url, map[string]any{"expectedRevision": 0, "mode": "review", "allowedTargets": []string{"plan"}, "automaticRepairLimit": 0}, 200).Body.Bytes(), &saved)
	if saved.AutomaticRepairLimit == nil || *saved.AutomaticRepairLimit != 0 {
		t.Fatal("explicit zero was defaulted", saved)
	}
	for _, null := range []bool{false, true} {
		body := map[string]any{"expectedRevision": saved.Revision, "mode": "review", "allowedTargets": []string{"plan"}}
		if null {
			body["automaticRepairLimit"] = nil
		}
		decodeEvidenceResponse(t, journeyRequest(t, handler, http.MethodPost, url, body, 200).Body.Bytes(), &saved)
		if saved.AutomaticRepairLimit == nil || *saved.AutomaticRepairLimit != 0 {
			t.Fatal("omitted/null old-client save reset zero", saved)
		}
	}
	for _, invalid := range []any{-1, 3, 1.5, "0", false} {
		journeyRequest(t, handler, http.MethodPost, url, map[string]any{"expectedRevision": saved.Revision, "mode": "review", "allowedTargets": []string{"plan"}, "automaticRepairLimit": invalid}, 400)
	}
	journeyRequest(t, handler, http.MethodPost, url, map[string]any{"expectedRevision": 0, "mode": "review", "allowedTargets": []string{"plan"}, "automaticRepairLimit": 2}, 409)
	s.SetCoordinatorAuthorizer(nil)
	journeyRequest(t, handler, http.MethodPost, url, map[string]any{"expectedRevision": saved.Revision, "mode": "review", "allowedTargets": []string{"plan"}, "automaticRepairLimit": 2}, 403)
	again, err := s.readCoordinatorPolicy(p)
	if err != nil || again.Revision != saved.Revision || again.AutomaticRepairLimit == nil || *again.AutomaticRepairLimit != 0 {
		t.Fatal("rejected save changed durable zero", again, err)
	}
}

func TestAutomaticRepairLimitContractCaptureAndPolicyStaleness(t *testing.T) {
	s, handler, req, base, projectPath, _ := contractFixture(t)
	journeyRequest(t, handler, http.MethodPost, base+"/orchestrator/policy", map[string]any{"expectedRevision": 0, "mode": "review", "allowedTargets": []string{"plan"}, "automaticRepairLimit": 0}, 200)
	view := contractResponse(t, journeyRequest(t, handler, http.MethodPost, base+"/orchestration-contracts/prepare", contractRequest(req), 201).Body.Bytes())
	if view.Contract.Input.Controls.AutomaticRepairLimit == nil || *view.Contract.Input.Controls.AutomaticRepairLimit != 0 || view.Contract.Digest != reviewDigest(view.Contract.Input) {
		t.Fatal("prepared contract did not digest-bind zero", view)
	}
	approved := contractResponse(t, journeyRequest(t, handler, http.MethodPost, base+"/orchestration-contracts/"+view.Contract.ID+"/approve", map[string]any{"expectedRevision": 1, "expectedDigest": view.Contract.Digest}, 200).Body.Bytes())
	req.OrchestrationContract = &OrchestrationContractSelection{ID: approved.Contract.ID, Revision: approved.Contract.Revision, Digest: approved.Contract.Digest, ApprovalID: approved.Contract.Approval.ID}
	run := projectRunHTTPResponse(t, journeyRequest(t, handler, http.MethodPost, base+"/runs", req, 201)).ProjectRun
	run = projectRunHTTPResponse(t, journeyRequest(t, handler, http.MethodPost, base+"/runs/"+run.ID+"/operations", map[string]any{"expectedRevision": run.Revision, "idempotencyKey": "budget-zero-capture-123456", "kind": "session"}, 201)).ProjectRun
	capture, err := s.CaptureProjectRunTaskContext(context.Background(), req.ImplementationContext.ProjectID, run.ID, run.Operations[0].ID, req.WorktreeID, projectPath, req.ImplementationContext)
	if err != nil {
		t.Fatal(err)
	}
	delivered := false
	for _, file := range capture.Files {
		if file.Path == "LUCI.md" {
			delivered = bytes.Contains(file.Data, []byte(`"automaticRepairLimit": 0`))
		}
	}
	if !delivered {
		t.Fatal("approved zero budget was absent from exact delivered context")
	}
	journeyRequest(t, handler, http.MethodPost, base+"/orchestrator/policy", map[string]any{"expectedRevision": 1, "mode": "review", "allowedTargets": []string{"plan"}, "automaticRepairLimit": 1}, 200)
	stale := contractResponse(t, journeyRequest(t, handler, http.MethodGet, base+"/orchestration-contracts/"+view.Contract.ID, nil, 200).Body.Bytes())
	if stale.Freshness.Status != "changed" || stale.Contract.Digest != approved.Contract.Digest || *stale.Contract.Input.Controls.AutomaticRepairLimit != 0 {
		t.Fatal("policy change edited or revived prior approval", stale)
	}
	journeyRequest(t, handler, http.MethodPost, base+"/runs", req, 200) // exact readback remains available
	if err := s.ValidateSwarmOperationExecutionContract(req.ImplementationContext.ProjectID, run, run.Operations[0].ID); err == nil {
		t.Fatal("stale operation contract authorized new native effect")
	}
	req.IdempotencyKey += "-stale-new"
	journeyRequest(t, handler, http.MethodPost, base+"/runs", req, 409)
}

func TestAutomaticRepairLimitLegacyReadDoesNotRewriteDigests(t *testing.T) {
	s, handler, req, base, _, _ := contractFixture(t)
	view := contractResponse(t, journeyRequest(t, handler, http.MethodPost, base+"/orchestration-contracts/prepare", contractRequest(req), 201).Body.Bytes())
	p, err := s.resolveProject(req.ImplementationContext.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	s.orchestrationContractMu.Lock()
	path, ledger, err := s.readOrchestrationContracts(p)
	if err == nil {
		ledger.Records[0].Contract.Input.Controls.AutomaticRepairLimit = nil
		ledger.Records[0].Contract.Digest = reviewDigest(ledger.Records[0].Contract.Input)
		err = writeOrchestrationContracts(path, ledger)
	}
	s.orchestrationContractMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil || bytes.Contains(before, []byte("automaticRepairLimit")) {
		t.Fatal("fixture was not a legacy receipt", err)
	}
	legacy := contractResponse(t, journeyRequest(t, handler, http.MethodGet, base+"/orchestration-contracts/"+view.Contract.ID, nil, 200).Body.Bytes())
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) || legacy.Contract.Input.Controls.AutomaticRepairLimit != nil || legacy.Freshness.Status != "current" || legacy.Contract.Digest != reviewDigest(legacy.Contract.Input) {
		t.Fatal("legacy read changed retained bytes/digest/default", legacy, err)
	}
	input := ProjectRunInput{OrchestrationContract: &legacy.Contract}
	run := ProjectRun{Input: input}
	digest := queueInputDigest(run)
	encoded, _ := json.Marshal(run.Input)
	var reopened ProjectRunInput
	if err := json.Unmarshal(encoded, &reopened); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "automaticRepairLimit") || digest != queueInputDigest(ProjectRun{Input: reopened}) {
		t.Fatal("legacy run digest gained a budget field")
	}
}

func TestAutomaticRepairBudgetExactOrdinalAndNoRefund(t *testing.T) {
	zero, one := 0, 1
	commit := strings.Repeat("a", 40)
	run := ProjectRun{Input: ProjectRunInput{OrchestrationContract: &OrchestrationContract{Input: OrchestrationPreparedInput{Controls: OrchestrationControls{AutomaticRepairLimit: &one}}}}, Swarm: &ProjectSwarm{ReviewObligations: []ProjectSwarmReviewObligation{{TaskID: "task-a", State: "repair-required", CandidateOperationID: "candidate", CandidateChildID: "child-a", CandidateCommit: commit, RepairOperationID: "repair"}}}, Operations: []ProjectRunOperation{{ID: "candidate", TaskID: "task-a", ChildID: "child-a"}, {ID: "repair", TaskID: "task-a", ChildID: "child-r", Status: "cancelled", Attempt: 9, Checkpoint: &ProjectSwarmCheckpoint{SourceOperationID: "candidate", SourceChildID: "child-a", SourceCommit: commit}}, {ID: "reviewer", TaskID: "review-a", Attempt: 10}, {ID: "foreign-retry", TaskID: "task-a", Attempt: 11}}}
	if err := swarmRepairBudgetForOperation(run, "repair"); err != nil || swarmRepairsConsumed(run, "task-a") != 1 {
		t.Fatal("cancelled reservation was refunded or global attempt consumed budget", err)
	}
	if err := swarmRepairBudgetForOperation(run, "reviewer"); err != nil {
		t.Fatal("final independent reviewer consumed repair budget", err)
	}
	if err := swarmRepairBudgetForOperation(run, "foreign-retry"); err == nil {
		t.Fatal("unlinked retry bypassed failed finding")
	}
	run.Swarm.ReviewObligations = append(run.Swarm.ReviewObligations, ProjectSwarmReviewObligation{TaskID: "task-other", State: "repair-required", CandidateOperationID: "other", CandidateChildID: "child-other"})
	run.Operations = append(run.Operations, ProjectRunOperation{ID: "other", TaskID: "task-other", ChildID: "child-other"})
	if err := swarmRepairBudgetForCheckpoint(run, "other"); err != nil || swarmRepairsConsumed(run, "task-a") != 1 {
		t.Fatal("another task inherited the first task's consumed budget", err)
	}
	run.Input.OrchestrationContract.Input.Controls.AutomaticRepairLimit = &zero
	if err := swarmRepairBudgetForOperation(run, "repair"); !errors.Is(err, errAutomaticRepairLimit) {
		t.Fatal("zero permitted prior unstarted repair", err)
	}
	if err := swarmRepairBudgetForOperation(run, "reviewer"); err != nil {
		t.Fatal("zero blocked existing final review", err)
	}
	run.Input.OrchestrationContract.Input.Controls.AutomaticRepairLimit = &one
	run.Operations[1].Checkpoint.SourceChildID = "another-child"
	if err := swarmRepairBudgetForOperation(run, "repair"); err == nil {
		t.Fatal("changed retained child identity passed")
	}
}
