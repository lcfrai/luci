package product

import (
	"strings"
	"testing"
)

func TestCoordinatorIntegratedRunRequiresPassedReviewerButNoReviewerSourceMerge(t *testing.T) {
	sha := strings.Repeat("a", 64)
	digest := strings.Repeat("b", 64)
	run := ProjectRun{ID: "run-reviewed", ProjectID: "p-reviewed", State: "awaiting-checks", Input: ProjectRunInput{Requirements: []RequirementSnapshot{{ID: "AC-save"}}}, Swarm: &ProjectSwarm{State: "ready", Tasks: []ProjectSwarmTask{
		{ID: "code", OperationID: "op-code", Integration: &ProjectSwarmIntegration{State: "integrated", Commit: sha, ReviewDigest: digest}},
		{ID: "review", OperationID: "op-review", ReviewOnly: true},
	}, ReviewObligations: []ProjectSwarmReviewObligation{{TaskID: "code", CandidateOperationID: "op-code", ReviewerOperationID: "op-review", State: "passed", ReviewerHandoffDigest: digest}}}, Operations: []ProjectRunOperation{
		{ID: "op-code", TaskID: "code", Status: "succeeded", Allocation: "ready"},
		{ID: "op-review", TaskID: "review", Status: "succeeded", Allocation: "ready", Handoff: &TaskHandoffReference{Digest: digest}},
	}}
	row := coordinatorContinuationRecord{CoordinatorContinuation: CoordinatorContinuation{ProjectID: run.ProjectID, RunID: run.ID}, Run: projectRunCreateRequest{ImplementationContext: ImplementationContextRequest{TaskIDs: []string{"code", "review"}}}}
	if _, ready := coordinatorIntegratedRun(run, row); !ready {
		t.Fatal("passed independent reviewer was incorrectly required to merge source")
	}
	run.Swarm.ReviewObligations[0].State = "repair-required"
	if _, ready := coordinatorIntegratedRun(run, row); ready {
		t.Fatal("failed independent finding opened combined checks")
	}
	run.Swarm.ReviewObligations[0].State = "passed"
	run.Operations[1].Handoff.Digest = strings.Repeat("c", 64)
	if _, ready := coordinatorIntegratedRun(run, row); ready {
		t.Fatal("changed reviewer handoff opened combined checks")
	}
}
