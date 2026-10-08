package product

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

func allowFixtureFenceRecovery(context.Context, ProjectSwarmFenceRecoveryRequest) error { return nil }

func fenceRecoveryRequest(run ProjectRun, op ProjectRunOperation) map[string]any {
	return map[string]any{"expectedRevision": run.Revision, "operationId": op.ID, "childId": op.ChildID, "stopUncertainLaunch": true}
}
func fixtureFenceIntent(req ProjectSwarmFenceRecoveryRequest) ProjectSwarmRecoveryIntent {
	if req.Intent != nil {
		return *req.Intent
	}
	lease := req.QueueID
	if lease == "" {
		lease = "manual-lease"
	}
	return ProjectSwarmRecoveryIntent{ID: "recovery-controlled", RequestedAt: time.Now().UTC().Format(time.RFC3339Nano), RequestedBy: "current-editor", OriginalUpdatedAt: req.UpdatedAt, RequestDigest: req.RequestDigest, ImplementationDigest: req.ImplementationDigest, LeaseID: lease, QueueID: req.QueueID, ScopeDigest: strings.Repeat("a", 64), Protocol: 1, FenceRequest: backend.LaunchFenceRequest{EngineID: "private-engine", InstanceID: "private-instance", SandboxID: req.ChildID, SessionID: req.ChildID, IntentID: "recovery-controlled", ScopeDigest: strings.Repeat("a", 64), OperationDigest: ProjectSwarmFenceOperationDigest(req)}}
}
func fixtureFenceProof(req ProjectSwarmFenceRecoveryRequest, intent ProjectSwarmRecoveryIntent) ProjectSwarmRecoveryProof {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	f := intent.FenceRequest
	return ProjectSwarmRecoveryProof{Method: ProjectSwarmFenceRecoveryMethod, ChildID: req.ChildID, QueueID: req.QueueID, LeaseID: intent.LeaseID, Backend: "docker", ObservedAt: now, FenceProof: backend.LaunchFenceProof{Protocol: 1, EngineID: f.EngineID, InstanceID: f.InstanceID, SandboxID: f.SandboxID, SessionID: f.SessionID, IntentID: intent.ID, ScopeDigest: f.ScopeDigest, OperationDigest: f.OperationDigest, FenceRef: strings.Repeat("b", 64), ConfigDigest: strings.Repeat("c", 64), AcknowledgedAt: now, ObservedAt: now}}
}

func TestProjectSwarmFenceRecoveryDurableSequenceAndRetainedOutput(t *testing.T) {
	s, mux, run, op, base, queueID := missingSwarmClaimFixture(t)
	path, e := s.swarmPath(run.ProjectID, run.ID, op.ID)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(path, "retained.txt"), []byte("retain exact output\n"), 0600); e != nil {
		t.Fatal(e)
	}
	original := auditTree(t, path)
	guarded, acquires, proofs, finalizes, releases := false, 0, 0, 0, 0
	failProof, failFinalize := true, true
	var savedIntent ProjectSwarmRecoveryIntent
	s.SetProjectSwarmFenceRecovery(func(context.Context, ProjectSwarmFenceRecoveryRequest) (ProjectSwarmRecoveryAvailability, error) {
		return ProjectSwarmRecoveryAvailability{Supported: true, ReasonCode: "supported", Message: "Explicit consent required."}, nil
	}, allowFixtureFenceRecovery, func(_ context.Context, req ProjectSwarmFenceRecoveryRequest) (ProjectSwarmRecoveryIntent, func(), error) {
		if guarded {
			t.Fatal("overlapping lifecycle guard")
		}
		guarded = true
		acquires++
		intent := fixtureFenceIntent(req)
		if savedIntent.ID != "" && intent != savedIntent {
			t.Fatal("retry changed immutable intent")
		}
		savedIntent = intent
		return intent, func() { guarded = false; releases++ }, nil
	}, func(_ context.Context, req ProjectSwarmFenceRecoveryRequest, intent ProjectSwarmRecoveryIntent) (ProjectSwarmRecoveryProof, error) {
		proofs++
		durable, proof, e := s.ProjectSwarmLaunchRecovery(req.ProjectID, req.RunID, req.OperationID, req.ChildID, req.QueueID)
		if !guarded || e != nil || durable == nil || *durable != intent || proof != nil {
			t.Fatal("backend effect preceded durable intent/lifecycle guard", e)
		}
		if failProof {
			return ProjectSwarmRecoveryProof{}, errors.New("acknowledgement uncertain")
		}
		return fixtureFenceProof(req, intent), nil
	}, func(_ context.Context, req ProjectSwarmFenceRecoveryRequest, intent ProjectSwarmRecoveryIntent, proof ProjectSwarmRecoveryProof) error {
		finalizes++
		durable, retained, e := s.ProjectSwarmLaunchRecovery(req.ProjectID, req.RunID, req.OperationID, req.ChildID, req.QueueID)
		retired, e2 := s.ProjectSwarmRetirement(req.ProjectID, req.RunID, req.OperationID, req.ChildID, req.QueueID)
		if !guarded || e != nil || e2 != nil || *durable != intent || retained == nil || *retained != proof || retired == nil || *retired != proof {
			t.Fatal("release preceded durable proof/cancellation", e, e2)
		}
		if failFinalize {
			return errors.New("lease retirement storage unavailable")
		}
		return nil
	})
	journeyRequest(t, mux, "POST", base+"/tasks/task-a/abandon", fenceRecoveryRequest(run, op), 409)
	pending, e := s.GetProjectSwarmRun(run.ProjectID, run.ID)
	if e != nil {
		t.Fatal(e)
	}
	pendingOp := swarmOperation(&pending, op.ID)
	if pendingOp.LaunchRecovery == nil || pendingOp.LaunchRecoveryProof != nil || pendingOp.AbsenceRetirement != nil || !swarmSlotPending(*pendingOp) || guarded || releases != 1 {
		t.Fatal("failed proof lost pending intent/capacity")
	}
	journeyRequest(t, mux, "POST", base+"/tasks/task-a/abandon", recoveryRequest(pending, *pendingOp), 409)
	journeyRequest(t, mux, "POST", base+"/tasks/task-a/abandon", map[string]any{"expectedRevision": pending.Revision}, 409)
	journeyRequest(t, mux, "POST", base+"/tasks/task-a/queue", map[string]any{"expectedRevision": pending.Revision, "idempotencyKey": "cannot-replace-pending-fence", "priority": "normal"}, 409)
	if _, e = s.projectRuns.ObserveOperation(run.ProjectID, pathForProject(t, s, run.ProjectID), run.ID, pending.Revision, op.ID, op.ChildID, "running", "stale observer"); !errors.Is(e, ErrProjectRunPending) {
		t.Fatal("observer overwrote recovery intent", e)
	}
	// Reopen durable storage after a failed backend acknowledgement. Explicit
	// retry uses the same intent, without manufacturing a successful proof.
	s.projectRuns, e = NewProjectRunStore(s.stateDir)
	if e != nil {
		t.Fatal(e)
	}
	failProof = false
	journeyRequest(t, mux, "POST", base+"/tasks/task-a/abandon", fenceRecoveryRequest(pending, *pendingOp), 409)
	cancelled, e := s.GetProjectSwarmRun(run.ProjectID, run.ID)
	if e != nil {
		t.Fatal(e)
	}
	cancelledOp := swarmOperation(&cancelled, op.ID)
	if cancelledOp.Status != "cancelled" || cancelledOp.LaunchRecoveryProof == nil || cancelledOp.AbsenceRetirement == nil || cancelledOp.AbsenceRetirement.FinalizedAt != "" || !swarmSlotPending(*cancelledOp) {
		t.Fatal("release failure discarded pending finalization")
	}
	queue, e := s.ProjectSwarmQueueSnapshot(run.ProjectID)
	if e != nil || queue.Items[0].MemoryMiB != 1024 || queue.Items[0].State != "unknown" {
		t.Fatal("queue acknowledged before exact lease release", e)
	}
	view := s.projectRunView(cancelled)
	data, _ := json.Marshal(view)
	if view.Operations[0].Recovery == nil || view.Operations[0].Recovery.State != "fenced" {
		t.Fatal("missing public fenced progress")
	}
	for _, private := range []string{"launchRecovery", "fenceProof", "private-engine", "private-instance", "current-editor", savedIntent.ScopeDigest, savedIntent.FenceRequest.OperationDigest} {
		if strings.Contains(string(data), private) {
			t.Fatal("private recovery identity leaked", private)
		}
	}
	// Actual run GET and list must not expose private proof/actor identities.
	for _, url := range []string{strings.TrimSuffix(base, "/swarm"), strings.TrimSuffix(base, "/runs/"+run.ID+"/swarm") + "/runs"} {
		response := journeyRequest(t, mux, "GET", url, nil, 200)
		for _, private := range []string{"launchRecovery", "fenceProof", "private-engine", "private-instance", "current-editor", savedIntent.ScopeDigest} {
			if strings.Contains(response.Body.String(), private) {
				t.Fatal("HTTP run view leaked private recovery", url, private)
			}
		}
		if !strings.Contains(response.Body.String(), `"state":"fenced"`) {
			t.Fatal("HTTP view lost pending fence progress", response.Body.String())
		}
	}
	failFinalize = false
	result := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/tasks/task-a/abandon", fenceRecoveryRequest(cancelled, *cancelledOp), 200)).ProjectRun
	if result.Operations[0].LaunchRecovery != nil || result.Operations[0].LaunchRecoveryProof != nil || result.Operations[0].AbsenceRetirement != nil {
		t.Fatal("abandon response leaked private fencing receipt")
	}
	if result.Operations[0].Recovery == nil || result.Operations[0].Recovery.State != "finalized" || proofs != 2 || finalizes != 2 || acquires != 3 || releases != 3 || guarded {
		t.Fatal("retry repeated proof or lost final acknowledgement")
	}
	final, e := s.GetProjectSwarmRun(run.ProjectID, run.ID)
	if e != nil {
		t.Fatal(e)
	}
	if swarmSlotPending(final.Operations[0]) || final.Operations[0].LaunchRecoveryProof == nil {
		t.Fatal("finalized proof not retained")
	}
	queue, e = s.ProjectSwarmQueueSnapshot(run.ProjectID)
	if e != nil || queue.Counts.Cancelled != 1 || queue.Items[0].ID != queueID || queue.Items[0].MemoryMiB != 0 {
		t.Fatal("exact final queue acknowledgement missing", e)
	}
	if !reflect.DeepEqual(original, auditTree(t, path)) {
		t.Fatal("recovery changed retained output")
	}
}

func pathForProject(t *testing.T, s *Server, id string) string {
	t.Helper()
	p, e := s.resolveProject(id)
	if e != nil {
		t.Fatal(e)
	}
	return p.Path
}

func TestProjectSwarmFenceRecoveryGuardAndConsentBeforeIntent(t *testing.T) {
	s, mux, run, op, base, _ := missingSwarmClaimFixture(t)
	acquired, effects := 0, 0
	s.SetProjectSwarmFenceRecovery(nil, allowFixtureFenceRecovery, func(context.Context, ProjectSwarmFenceRecoveryRequest) (ProjectSwarmRecoveryIntent, func(), error) {
		acquired++
		return ProjectSwarmRecoveryIntent{}, nil, errors.New("live allocator holds lifecycle guard")
	}, func(context.Context, ProjectSwarmFenceRecoveryRequest, ProjectSwarmRecoveryIntent) (ProjectSwarmRecoveryProof, error) {
		effects++
		return ProjectSwarmRecoveryProof{}, nil
	}, func(context.Context, ProjectSwarmFenceRecoveryRequest, ProjectSwarmRecoveryIntent, ProjectSwarmRecoveryProof) error {
		effects++
		return nil
	})
	mixed := fenceRecoveryRequest(run, op)
	mixed["recoverMissingChild"] = true
	journeyRequest(t, mux, "POST", base+"/tasks/task-a/abandon", mixed, 409)
	stale := fenceRecoveryRequest(run, op)
	stale["expectedRevision"] = run.Revision - 1
	journeyRequest(t, mux, "POST", base+"/tasks/task-a/abandon", stale, 409)
	missing := map[string]any{"expectedRevision": run.Revision, "stopUncertainLaunch": true}
	journeyRequest(t, mux, "POST", base+"/tasks/task-a/abandon", missing, 409)
	if acquired != 0 {
		t.Fatal("guard acquired before exact consent/CAS")
	}
	journeyRequest(t, mux, "POST", base+"/tasks/task-a/abandon", fenceRecoveryRequest(run, op), 409)
	saved, e := s.GetProjectSwarmRun(run.ProjectID, run.ID)
	if e != nil || saved.Revision != run.Revision || saved.Operations[0].LaunchRecovery != nil || effects != 0 || acquired != 1 {
		t.Fatal("live allocator refusal published intent or ran effects", e)
	}
}

func TestProjectSwarmFenceProofMethodAndPrivateStoreValidation(t *testing.T) {
	s, _, run, op, _, _ := missingSwarmClaimFixture(t)
	req, e := s.swarmFenceRecoveryRequest(run, op)
	if e != nil {
		t.Fatal(e)
	}
	intent := fixtureFenceIntent(req)
	proof := fixtureFenceProof(req, intent)
	if !validSwarmRecoveryProof(proof, op.ChildID) || !validSwarmRecoveryIntent(intent, req) {
		t.Fatal("valid controlled fence rejected")
	}
	for _, change := range []func(*ProjectSwarmRecoveryProof){
		func(p *ProjectSwarmRecoveryProof) { p.Method = "" },
		func(p *ProjectSwarmRecoveryProof) { p.InventoryObservedAt = p.ObservedAt },
		func(p *ProjectSwarmRecoveryProof) {
			p.FenceProof.AcknowledgedAt = time.Now().Add(time.Hour).Format(time.RFC3339Nano)
		},
		func(p *ProjectSwarmRecoveryProof) { p.FenceProof.Protocol = 0 },
		func(p *ProjectSwarmRecoveryProof) { p.FenceProof.SessionID = "foreign-child" },
	} {
		bad := proof
		change(&bad)
		if validSwarmRecoveryProof(bad, op.ChildID) {
			t.Fatal("ambiguous fence method/proof accepted")
		}
	}
	op.LaunchRecovery = &intent
	op.LaunchRecoveryProof = &proof
	if !validSwarmOperationRecovery(run, op) {
		t.Fatal("durable proof before cancellation rejected")
	}
	altered := intent
	altered.RequestDigest = strings.Repeat("f", 64)
	op.LaunchRecovery = &altered
	if validSwarmOperationRecovery(run, op) {
		t.Fatal("changed reviewed request accepted")
	}
	op.LaunchRecovery = &intent
	badProof := proof
	badProof.FenceProof.OperationDigest = strings.Repeat("f", 64)
	op.LaunchRecoveryProof = &badProof
	if validSwarmOperationRecovery(run, op) {
		t.Fatal("foreign operation fence accepted")
	}
	op.LaunchRecoveryProof = &proof
	op.Recovery = &ProjectSwarmRecoveryView{State: "finalized"}
	if validSwarmOperationRecovery(run, op) {
		t.Fatal("public display state treated as private evidence")
	}
}

func TestProjectSwarmFenceRecoveryRefusesBusyAndRevokedBeforeIntent(t *testing.T) {
	s, mux, run, op, base, _ := missingSwarmClaimFixture(t)
	effects, released := 0, false
	s.SetProjectSwarmFenceRecovery(nil, func(context.Context, ProjectSwarmFenceRecoveryRequest) error {
		return errors.New("editor revoked while lifecycle guard waited")
	}, func(_ context.Context, req ProjectSwarmFenceRecoveryRequest) (ProjectSwarmRecoveryIntent, func(), error) {
		return fixtureFenceIntent(req), func() { released = true }, nil
	}, func(context.Context, ProjectSwarmFenceRecoveryRequest, ProjectSwarmRecoveryIntent) (ProjectSwarmRecoveryProof, error) {
		effects++
		return ProjectSwarmRecoveryProof{}, nil
	}, func(context.Context, ProjectSwarmFenceRecoveryRequest, ProjectSwarmRecoveryIntent, ProjectSwarmRecoveryProof) error {
		effects++
		return nil
	})
	s.projectRunDispatchMu.Lock()
	// Do not issue a same-goroutine locking request: the helper goroutine lets a
	// regression fail with a bounded timeout rather than deadlocking the suite.
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		journeyRequest(t, mux, "POST", base+"/tasks/task-a/abandon", fenceRecoveryRequest(run, op), 409)
	}()
	select {
	case <-finished:
	case <-time.After(time.Second):
		s.projectRunDispatchMu.Unlock()
		<-finished
		t.Fatal("same-process allocator refusal was not bounded")
	}
	s.projectRunDispatchMu.Unlock()
	journeyRequest(t, mux, "POST", base+"/tasks/task-a/abandon", fenceRecoveryRequest(run, op), 409)
	saved, e := s.GetProjectSwarmRun(run.ProjectID, run.ID)
	if e != nil || saved.Revision != run.Revision || saved.Operations[0].LaunchRecovery != nil || effects != 0 || !released {
		t.Fatal("revoked actor published intent or ran effects", e)
	}
}

func TestProjectSwarmFenceRecoveryReadOnlyCapabilityAndEligibility(t *testing.T) {
	s, mux, run, op, base, _ := missingSwarmClaimFixture(t)
	reads, effects := 0, 0
	s.SetProjectSwarmFenceRecovery(func(_ context.Context, req ProjectSwarmFenceRecoveryRequest) (ProjectSwarmRecoveryAvailability, error) {
		reads++
		if req.ChildID != op.ChildID || req.OperationID != op.ID {
			t.Fatal("capability scope changed")
		}
		return ProjectSwarmRecoveryAvailability{Supported: true, ReasonCode: "supported", Message: "Current exact launch supports explicit recovery."}, nil
	}, allowFixtureFenceRecovery, func(context.Context, ProjectSwarmFenceRecoveryRequest) (ProjectSwarmRecoveryIntent, func(), error) {
		effects++
		return ProjectSwarmRecoveryIntent{}, nil, errors.New("unexpected acquire")
	}, nil, nil)
	response := journeyRequest(t, mux, "GET", base+"/operations/"+op.ID+"/recovery", nil, 200)
	var capability struct {
		ExpectedRevision int                              `json:"expectedRevision"`
		OperationID      string                           `json:"operationId"`
		ChildID          string                           `json:"childId"`
		Availability     ProjectSwarmRecoveryAvailability `json:"availability"`
	}
	if e := json.Unmarshal(response.Body.Bytes(), &capability); e != nil || !capability.Availability.Supported || capability.ExpectedRevision != run.Revision || capability.OperationID != op.ID || capability.ChildID != op.ChildID || effects != 0 || reads != 1 {
		t.Fatal("read-only capability contract mismatch", e, response.Body.String())
	}
	p, e := s.resolveProject(run.ProjectID)
	if e != nil {
		t.Fatal(e)
	}
	run, e = s.projectRuns.mutateSwarm(p.ID, p.Path, run.ID, run.Revision, func(current *ProjectRun) error {
		target := swarmOperation(current, op.ID)
		target.Status = "reserved"
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
	response = journeyRequest(t, mux, "GET", base+"/operations/"+op.ID+"/recovery", nil, 200)
	if e = json.Unmarshal(response.Body.Bytes(), &capability); e != nil || capability.Availability.Supported || capability.Availability.ReasonCode != "attempt_not_eligible" || effects != 0 {
		t.Fatal("GET advertised an ineligible POST", e, response.Body.String())
	}
}

func TestProjectSwarmFenceRecoveryInterruptedProofAndFinalAck(t *testing.T) {
	for _, boundary := range []string{"proof-publication", "final-acknowledgement"} {
		t.Run(boundary, func(t *testing.T) {
			s, mux, run, op, base, _ := missingSwarmClaimFixture(t)
			interrupt := true
			proofs, finalizations := 0, 0
			mutateRevision := func() {
				t.Helper()
				p, e := s.resolveProject(run.ProjectID)
				if e != nil {
					t.Fatal(e)
				}
				current, e := s.GetProjectSwarmRun(run.ProjectID, run.ID)
				if e != nil {
					t.Fatal(e)
				}
				_, e = s.projectRuns.mutateSwarm(p.ID, p.Path, run.ID, current.Revision, func(current *ProjectRun) error {
					current.Reason = "Simulated concurrent persistence boundary"
					return nil
				})
				if e != nil {
					t.Fatal(e)
				}
			}
			s.SetProjectSwarmFenceRecovery(nil, allowFixtureFenceRecovery, func(_ context.Context, req ProjectSwarmFenceRecoveryRequest) (ProjectSwarmRecoveryIntent, func(), error) {
				return fixtureFenceIntent(req), func() {}, nil
			}, func(_ context.Context, req ProjectSwarmFenceRecoveryRequest, intent ProjectSwarmRecoveryIntent) (ProjectSwarmRecoveryProof, error) {
				proofs++
				proof := fixtureFenceProof(req, intent)
				if boundary == "proof-publication" && interrupt {
					mutateRevision()
				}
				return proof, nil
			}, func(context.Context, ProjectSwarmFenceRecoveryRequest, ProjectSwarmRecoveryIntent, ProjectSwarmRecoveryProof) error {
				finalizations++
				if boundary == "final-acknowledgement" && interrupt {
					mutateRevision()
				}
				return nil
			})
			journeyRequest(t, mux, "POST", base+"/tasks/task-a/abandon", fenceRecoveryRequest(run, op), 409)
			retained, e := s.GetProjectSwarmRun(run.ProjectID, run.ID)
			if e != nil {
				t.Fatal(e)
			}
			retainedOp := retained.Operations[0]
			if retainedOp.LaunchRecovery == nil || !swarmSlotPending(retainedOp) {
				t.Fatal("interrupted recovery lost identity or pending state")
			}
			if boundary == "proof-publication" && (retainedOp.LaunchRecoveryProof != nil || retainedOp.AbsenceRetirement != nil || finalizations != 0) {
				t.Fatal("failed proof write advanced cancellation/release")
			}
			if boundary == "final-acknowledgement" && (retainedOp.AbsenceRetirement == nil || retainedOp.AbsenceRetirement.FinalizedAt != "" || finalizations != 1) {
				t.Fatal("failed final ack invented completion")
			}
			s.projectRuns, e = NewProjectRunStore(s.stateDir)
			if e != nil {
				t.Fatal(e)
			}
			interrupt = false
			journeyRequest(t, mux, "POST", base+"/tasks/task-a/abandon", fenceRecoveryRequest(retained, retainedOp), 200)
			saved, e := s.GetProjectSwarmRun(run.ProjectID, run.ID)
			if e != nil || swarmSlotPending(saved.Operations[0]) {
				t.Fatal("explicit retry did not complete", e)
			}
			expectedProofs := 1
			if boundary == "proof-publication" {
				expectedProofs = 2
			}
			if proofs != expectedProofs {
				t.Fatal("retry discarded durable proof", proofs)
			}
		})
	}
}

func TestProjectSwarmFenceRecoveryResumesPrivateIntentAfterProductCASConflict(t *testing.T) {
	s, mux, run, op, base, _ := missingSwarmClaimFixture(t)
	var retained ProjectSwarmRecoveryIntent
	acquires, proofs := 0, 0
	s.SetProjectSwarmFenceRecovery(nil, allowFixtureFenceRecovery, func(_ context.Context, req ProjectSwarmFenceRecoveryRequest) (ProjectSwarmRecoveryIntent, func(), error) {
		acquires++
		if retained.ID == "" {
			retained = fixtureFenceIntent(req)
			// Model a private API intent already committed while another controller's
			// observation changes the product revision before this handler can publish.
			p, e := s.resolveProject(run.ProjectID)
			if e != nil {
				t.Fatal(e)
			}
			_, e = s.projectRuns.ObserveOperation(p.ID, p.Path, run.ID, run.Revision, op.ID, op.ChildID, "unknown", "concurrent observation before intent publication")
			if e != nil {
				t.Fatal(e)
			}
		}
		return retained, func() {}, nil
	}, func(_ context.Context, req ProjectSwarmFenceRecoveryRequest, intent ProjectSwarmRecoveryIntent) (ProjectSwarmRecoveryProof, error) {
		proofs++
		if req.UpdatedAt != retained.OriginalUpdatedAt || intent != retained || ProjectSwarmFenceOperationDigest(req) != retained.FenceRequest.OperationDigest {
			t.Fatal("retry changed original reviewed recovery binding")
		}
		return fixtureFenceProof(req, intent), nil
	}, func(context.Context, ProjectSwarmFenceRecoveryRequest, ProjectSwarmRecoveryIntent, ProjectSwarmRecoveryProof) error {
		return nil
	})
	journeyRequest(t, mux, "POST", base+"/tasks/task-a/abandon", fenceRecoveryRequest(run, op), 409)
	current, e := s.GetProjectSwarmRun(run.ProjectID, run.ID)
	if e != nil || current.Revision == run.Revision || current.Operations[0].LaunchRecovery != nil || proofs != 0 {
		t.Fatal("conflicting intent publication advanced backend effects", e)
	}
	journeyRequest(t, mux, "POST", base+"/tasks/task-a/abandon", fenceRecoveryRequest(current, current.Operations[0]), 200)
	final, e := s.GetProjectSwarmRun(run.ProjectID, run.ID)
	if e != nil || final.Operations[0].LaunchRecovery == nil || *final.Operations[0].LaunchRecovery != retained || swarmSlotPending(final.Operations[0]) || acquires != 2 || proofs != 1 {
		t.Fatal("retry did not recover the same private intent", e)
	}
}
