package product

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"
	"time"
)

func missingSwarmClaimFixture(t *testing.T) (*Server, *http.ServeMux, ProjectRun, ProjectRunOperation, string, string) {
	t.Helper()
	s, mux, run, base, admitted, _, _ := schedulerFixture(t)
	*admitted = true
	s.swarmScheduler.runtime.Dispatch = func(_ context.Context, req SwarmDispatchRequest) (SwarmDispatchResult, error) {
		p, err := s.resolveProject(req.ProjectID)
		if err != nil {
			return SwarmDispatchResult{}, err
		}
		current, err := s.GetProjectSwarmRun(req.ProjectID, req.RunID)
		if err != nil {
			return SwarmDispatchResult{}, err
		}
		_, op, claimed, err := s.projectRuns.ClaimOperation(p.ID, p.Path, current.ID, current.Revision, req.OperationID)
		if err != nil || !claimed {
			t.Fatal("missing durable launch claim", err)
		}
		return SwarmDispatchResult{State: "unknown", SessionID: op.ChildID}, nil
	}
	items := enqueueSchedulerFixture(t, mux, run, base, []string{"task-a"})
	if err := s.RunSwarmSchedulerOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	current, err := s.GetProjectSwarmRun(run.ProjectID, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	return s, mux, current, current.Operations[0], base, items[0].ID
}
func recoveryRequest(run ProjectRun, op ProjectRunOperation) map[string]any {
	return map[string]any{"expectedRevision": run.Revision, "operationId": op.ID, "childId": op.ChildID, "recoverMissingChild": true}
}
func freshSwarmAbsenceProof(req ProjectSwarmRecoveryRequest) ProjectSwarmRecoveryProof {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	return ProjectSwarmRecoveryProof{ChildID: req.ChildID, QueueID: req.QueueID, LeaseID: req.QueueID, Backend: "docker", ObservedAt: now, InventoryObservedAt: now}
}
func TestProjectSwarmMissingClaimRetiresBeforeCapacityAndPreservesOutput(t *testing.T) {
	s, mux, run, op, base, queueID := missingSwarmClaimFixture(t)
	path, err := s.swarmPath(run.ProjectID, run.ID, op.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(path, "retained-output.txt"), []byte("preserve this exact output\n"), 0600); err != nil {
		t.Fatal(err)
	}
	before := auditTree(t, path)
	proofs, finalizations := 0, 0
	failFinalize := true
	s.SetProjectSwarmRecovery(func(_ context.Context, req ProjectSwarmRecoveryRequest) (ProjectSwarmRecoveryProof, error) {
		proofs++
		if req.ProjectID != run.ProjectID || req.RunID != run.ID || req.OperationID != op.ID || req.ChildID != op.ChildID || req.QueueID != queueID {
			t.Fatal("proof scope changed")
		}
		return freshSwarmAbsenceProof(req), nil
	}, func(_ context.Context, req ProjectSwarmRecoveryRequest, proof ProjectSwarmRecoveryProof) error {
		finalizations++
		saved, err := s.ProjectSwarmRetirement(req.ProjectID, req.RunID, req.OperationID, req.ChildID, req.QueueID)
		if err != nil || !reflect.DeepEqual(saved, &proof) {
			t.Fatal("capacity finalization preceded durable cancellation", err)
		}
		if failFinalize {
			return errors.New("capacity finalization interrupted")
		}
		return nil
	})
	journeyRequest(t, mux, "POST", base+"/tasks/task-a/abandon", recoveryRequest(run, op), 409)
	retired, err := s.GetProjectSwarmRun(run.ProjectID, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	saved := swarmOperation(&retired, op.ID)
	if saved.Status != "cancelled" || saved.AbsenceRetirement == nil || saved.AbsenceRetirement.FinalizedAt != "" || saved.ChildID != op.ChildID || !reflect.DeepEqual(before, auditTree(t, path)) {
		t.Fatal("lost tombstone or modified retained output")
	}
	snapshot, err := s.ProjectSwarmQueueSnapshot(run.ProjectID)
	if err != nil || snapshot.Items[0].MemoryMiB != 1024 || snapshot.Items[0].State != "unknown" {
		t.Fatalf("released capacity before finalization %+v %v", snapshot, err)
	}
	journeyRequest(t, mux, "POST", base+"/tasks/task-a/reserve", map[string]any{"expectedRevision": retired.Revision, "idempotencyKey": "refuse-before-retirement-finalized"}, 409)
	journeyRequest(t, mux, "POST", base+"/tasks/task-a/queue", map[string]any{"expectedRevision": retired.Revision, "idempotencyKey": "refuse-queue-before-finalized", "priority": "normal"}, 409)
	journeyRequest(t, mux, "POST", base+"/tasks/task-a/abandon", map[string]any{"expectedRevision": retired.Revision}, 409)
	// Reopen the actual run store before continuing finalization: pending
	// acknowledgement must survive losing all in-memory operation state.
	s.projectRuns, err = NewProjectRunStore(s.stateDir)
	if err != nil {
		t.Fatal(err)
	}
	failFinalize = false
	result := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/tasks/task-a/abandon", recoveryRequest(retired, *saved), 200)).ProjectRun
	if proofs != 1 || finalizations != 2 {
		t.Fatalf("retirement replayed initial proof or skipped finalization %d %d", proofs, finalizations)
	}
	finalOp := swarmOperation(&result, op.ID)
	if finalOp.AbsenceRetirement == nil || finalOp.AbsenceRetirement.FinalizedAt == "" || swarmSlotPending(*finalOp) {
		t.Fatal("missing durable completion marker")
	}
	completed := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/tasks/task-a/abandon", recoveryRequest(result, *finalOp), 200)).ProjectRun
	if completed.Revision != result.Revision || finalizations != 2 {
		t.Fatal("completed recovery repeated publication or execution proof")
	}
	immutableProof, err := s.ProjectSwarmRetirement(run.ProjectID, run.ID, op.ID, op.ChildID, queueID)
	if err != nil || immutableProof.FinalizedAt != "" || immutableProof.ObservedAt != saved.AbsenceRetirement.ObservedAt {
		t.Fatal("completion changed the execution service's immutable proof identity", err)
	}
	snapshot, err = s.ProjectSwarmQueueSnapshot(run.ProjectID)
	if err != nil || snapshot.Counts.Cancelled != 1 || snapshot.Items[0].MemoryMiB != 0 || snapshot.Items[0].ChildID != op.ChildID {
		t.Fatalf("missing final queue receipt %+v %v", snapshot, err)
	}
	if !reflect.DeepEqual(before, auditTree(t, path)) {
		t.Fatal("recovery changed retained output")
	}
	journeyRequest(t, mux, "POST", base+"/tasks/task-a/queue", map[string]any{"expectedRevision": result.Revision, "idempotencyKey": "explicit-fresh-after-proved-absence", "priority": "normal"}, 201)
}
func TestProjectSwarmMissingClaimRefusesStaleForeignAndAmbiguousProof(t *testing.T) {
	for _, variant := range []string{"stale", "foreign-child", "foreign-queue", "pre-claim", "uncertain-backend", "premature-finalization"} {
		t.Run(variant, func(t *testing.T) {
			s, mux, run, op, base, _ := missingSwarmClaimFixture(t)
			finalized := false
			s.SetProjectSwarmRecovery(func(_ context.Context, req ProjectSwarmRecoveryRequest) (ProjectSwarmRecoveryProof, error) {
				proof := freshSwarmAbsenceProof(req)
				switch variant {
				case "stale":
					proof.ObservedAt = time.Now().Add(-10 * time.Second).UTC().Format(time.RFC3339Nano)
					proof.InventoryObservedAt = proof.ObservedAt
				case "foreign-child":
					proof.ChildID = "s-foreign1"
				case "foreign-queue":
					proof.QueueID = "queue-other"
				case "pre-claim":
					claimed, _ := time.Parse(time.RFC3339Nano, req.UpdatedAt)
					proof.InventoryObservedAt = claimed.Add(-time.Nanosecond).Format(time.RFC3339Nano)
				case "uncertain-backend":
					return proof, errors.New("backend absence cannot be established")
				case "premature-finalization":
					proof.FinalizedAt = proof.ObservedAt
				}
				return proof, nil
			}, func(context.Context, ProjectSwarmRecoveryRequest, ProjectSwarmRecoveryProof) error {
				finalized = true
				return nil
			})
			before := auditTree(t, s.stateDir)
			journeyRequest(t, mux, "POST", base+"/tasks/task-a/abandon", recoveryRequest(run, op), 409)
			if finalized || !reflect.DeepEqual(before, auditTree(t, s.stateDir)) {
				t.Fatal("refused proof retired or released the claim")
			}
		})
	}
}
func TestProjectSwarmMissingClaimWaitsForActiveAllocatorFence(t *testing.T) {
	s, mux, run, op, base, _ := missingSwarmClaimFixture(t)
	var proofs atomic.Int32
	s.SetProjectSwarmRecovery(func(_ context.Context, req ProjectSwarmRecoveryRequest) (ProjectSwarmRecoveryProof, error) {
		proofs.Add(1)
		return freshSwarmAbsenceProof(req), nil
	}, func(context.Context, ProjectSwarmRecoveryRequest, ProjectSwarmRecoveryProof) error { return nil })
	payload, err := json.Marshal(recoveryRequest(run, op))
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("POST", base+"/tasks/task-a/abandon", bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	// This is the mutex held by BeginProjectRunSession through registry/backend
	// allocation. Recovery must not sample absence while that section can proceed.
	s.projectRunDispatchMu.Lock()
	done := make(chan struct{})
	go func() { defer close(done); mux.ServeHTTP(recorder, request) }()
	select {
	case <-done:
		s.projectRunDispatchMu.Unlock()
		t.Fatal("recovery passed an active admission")
	case <-time.After(20 * time.Millisecond):
	}
	if proofs.Load() != 0 {
		s.projectRunDispatchMu.Unlock()
		t.Fatal("sampled absence before admission finished")
	}
	p, err := s.resolveProject(run.ProjectID)
	if err != nil {
		s.projectRunDispatchMu.Unlock()
		t.Fatal(err)
	}
	_, err = s.projectRuns.ObserveOperation(p.ID, p.Path, run.ID, run.Revision, op.ID, op.ChildID, "running", "The admitted allocator created its child.")
	s.projectRunDispatchMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("recovery did not finish after admission released")
	}
	if recorder.Code != 409 || proofs.Load() != 0 {
		t.Fatalf("recovered a live/changed admission: status%d proofs%d", recorder.Code, proofs.Load())
	}
	current, err := s.GetProjectSwarmRun(run.ProjectID, run.ID)
	if err != nil || swarmOperation(&current, op.ID).Status != "running" {
		t.Fatal("retired the continuing allocator", err)
	}
}
func TestSwarmSchedulerDelayedObservationCannotUndoPauseOrCancellation(t *testing.T) {
	s, mux, run, base, _, _, _ := schedulerFixture(t)
	items := enqueueSchedulerFixture(t, mux, run, base, []string{"task-a", "task-b"})
	controls := "/v1/product/projects/" + run.ProjectID + "/swarm-queue/"
	journeyRequest(t, mux, "POST", controls+items[0].ID+"/pause", map[string]int{"expectedRevision": items[0].Revision}, 200)
	journeyRequest(t, mux, "POST", controls+items[1].ID+"/cancel", map[string]int{"expectedRevision": items[1].Revision}, 200)
	for _, item := range items {
		if err := s.schedulerSetReason(item.ID, "queued", "capacity", "A late resource observation must not undo user intent."); err != nil {
			t.Fatal(err)
		}
	}
	snapshot, err := s.ProjectSwarmQueueSnapshot(run.ProjectID)
	if err != nil || snapshot.Counts.Paused != 1 || snapshot.Counts.Cancelled != 1 {
		t.Fatalf("stale observation resurrected work %+v %v", snapshot, err)
	}
}
