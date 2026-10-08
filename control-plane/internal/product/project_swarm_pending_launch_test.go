package product

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestProjectSwarmPendingLaunchBlocksReplacingPrivateOnlyClaims(t *testing.T) {
	for _, mode := range []string{"pending", "unavailable", "missing callback"} {
		t.Run(mode, func(t *testing.T) {
			s, mux, run, op, base, _ := missingSwarmClaimFixture(t)
			p, err := s.resolveProject(run.ProjectID)
			if err != nil {
				t.Fatal(err)
			}
			run, err = s.projectRuns.ObserveOperation(p.ID, p.Path, run.ID, run.Revision, op.ID, op.ChildID, "failed", "controlled exited child without a launch acknowledgement")
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			check := func(_ context.Context, projectID, runID, operationID, childID string) (bool, error) {
				calls++
				if projectID != run.ProjectID || runID != run.ID || operationID != op.ID || childID != op.ChildID {
					t.Fatal("pending check used foreign identity")
				}
				if mode == "unavailable" {
					return false, errors.New("private execution state unavailable")
				}
				return true, nil
			}
			if mode == "missing callback" {
				s.SetProjectSwarmPendingLaunch(nil)
			} else {
				s.SetProjectSwarmPendingLaunch(check)
			}
			for _, action := range []string{"abandon", "reserve", "queue"} {
				request := map[string]any{"expectedRevision": run.Revision}
				if action != "abandon" {
					request["idempotencyKey"] = "keep-original-private-claim"
				}
				if action == "queue" {
					request["priority"] = "normal"
				}
				journeyRequest(t, mux, "POST", base+"/tasks/task-a/"+action, request, 409)
			}
			got, err := s.GetProjectSwarmRun(run.ProjectID, run.ID)
			if err != nil || !reflect.DeepEqual(run, got) {
				t.Fatal("ordinary action mutated unresolved attempt", err)
			}
			if mode != "missing callback" && calls != 2 {
				t.Fatal("abandon/reserve missed authoritative launch check (failed tasks already refuse queue consent)", calls)
			}
		})
	}
}

func TestProjectSwarmPendingLaunchUntouchedReservationNeedsNoLedger(t *testing.T) {
	run := ProjectRun{ProjectID: "project", ID: "run", Swarm: &ProjectSwarm{Tasks: []ProjectSwarmTask{{ID: "task", OperationID: "op"}}}, Operations: []ProjectRunOperation{{ID: "op", ChildID: "child", Status: "reserved"}}}
	s := &Server{}
	if err := s.requireNoSwarmPendingLaunch(context.Background(), run, "task"); err != nil {
		t.Fatal("untouched reservation incorrectly needs execution state", err)
	}
	run.Operations[0].Status = "failed"
	if err := s.requireNoSwarmPendingLaunch(context.Background(), run, "task"); err == nil {
		t.Fatal("claimed attempt proceeded without authoritative service")
	}
}

func TestProjectSwarmPendingLaunchQueueChecksPreviouslyCancelledAttempt(t *testing.T) {
	s, mux, run, op, base, _ := missingSwarmClaimFixture(t)
	p, err := s.resolveProject(run.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	run, err = s.projectRuns.ObserveOperation(p.ID, p.Path, run.ID, run.Revision, op.ID, op.ChildID, "cancelled", "older cancelled product state with retained private launch")
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	s.SetProjectSwarmPendingLaunch(func(_ context.Context, projectID, runID, operationID, childID string) (bool, error) {
		calls++
		if projectID != run.ProjectID || runID != run.ID || operationID != op.ID || childID != op.ChildID {
			t.Fatal("queue checked another attempt")
		}
		return true, nil
	})
	journeyRequest(t, mux, "POST", base+"/tasks/task-a/queue", map[string]any{"expectedRevision": run.Revision, "idempotencyKey": "keep-cancelled-private-claim", "priority": "normal"}, 409)
	got, err := s.GetProjectSwarmRun(run.ProjectID, run.ID)
	if err != nil || !reflect.DeepEqual(run, got) || calls != 1 {
		t.Fatal("cancelled attempt queue consent escaped private pending check", calls, err)
	}
}

func TestProjectSwarmPendingLaunchUnclaimedProofCannotCoexistWithClaimOrRecovery(t *testing.T) {
	s, _, run, op, _, _ := missingSwarmClaimFixture(t)
	p, err := s.resolveProject(run.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"claimed", "recovery intent", "absence proof", "integration"} {
		t.Run(mode, func(t *testing.T) {
			_, err := s.projectRuns.mutateSwarm(p.ID, p.Path, run.ID, run.Revision, func(current *ProjectRun) error {
				target := swarmOperation(current, op.ID)
				target.UnclaimedRetirement = true
				target.Status = "cancelled"
				switch mode {
				case "claimed":
					target.Status = "dispatching"
				case "recovery intent":
					req := ProjectSwarmFenceRecoveryRequest{ProjectSwarmRecoveryRequest: ProjectSwarmRecoveryRequest{ProjectID: run.ProjectID, RunID: run.ID, OperationID: op.ID, ChildID: op.ChildID, UpdatedAt: op.UpdatedAt}, RequestDigest: op.RequestDigest, ImplementationDigest: reviewDigest("context")}
					intent := fixtureFenceIntent(req)
					target.LaunchRecovery = &intent
				case "absence proof":
					proof := freshSwarmAbsenceProof(ProjectSwarmRecoveryRequest{ChildID: op.ChildID})
					target.AbsenceRetirement = &proof
				case "integration":
					target.Integration = &ProjectSwarmIntegration{State: "unknown"}
				}
				return nil
			})
			if !errors.Is(err, ErrProjectRunInvalid) {
				t.Fatal("incompatible unclaimed retirement persisted", mode, err)
			}
			unchanged, e := s.GetProjectSwarmRun(run.ProjectID, run.ID)
			if e != nil || unchanged.Revision != run.Revision || unchanged.Operations[0].UnclaimedRetirement {
				t.Fatal("invalid record partially persisted", e)
			}
		})
	}
}
