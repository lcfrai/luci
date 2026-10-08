package product

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

const ProjectSwarmFenceRecoveryMethod = "docker-name-fence-v1"

// Only server-resolved receipts cross this boundary. A browser supplies neither
// container identities nor a previously saved actor's authority.
type ProjectSwarmFenceRecoveryRequest struct {
	ProjectSwarmRecoveryRequest
	RequestDigest        string
	ImplementationDigest string
	Intent               *ProjectSwarmRecoveryIntent
}

// Immutable intent is recorded before any backend effects. It is comparable so
// every retry can require exactly the same reviewed action and launch binding.
type ProjectSwarmRecoveryIntent struct {
	ID                   string                     `json:"id"`
	RequestedAt          string                     `json:"requestedAt"`
	RequestedBy          string                     `json:"requestedBy"`
	OriginalUpdatedAt    string                     `json:"originalUpdatedAt"`
	RequestDigest        string                     `json:"requestDigest"`
	ImplementationDigest string                     `json:"implementationDigest"`
	LeaseID              string                     `json:"leaseId"`
	QueueID              string                     `json:"queueId,omitempty"`
	ScopeDigest          string                     `json:"scopeDigest"`
	Protocol             int                        `json:"protocol"`
	FenceRequest         backend.LaunchFenceRequest `json:"fenceRequest"`
}

type ProjectSwarmRecoveryAvailability struct {
	Supported  bool   `json:"supported"`
	ReasonCode string `json:"reasonCode"`
	Message    string `json:"message"`
}

type ProjectSwarmRecoveryView struct {
	State       string `json:"state"`
	RequestedAt string `json:"requestedAt"`
	ProvedAt    string `json:"provedAt,omitempty"`
	FinalizedAt string `json:"finalizedAt,omitempty"`
}

func (s *Server) SetProjectSwarmFenceRecovery(
	availability func(context.Context, ProjectSwarmFenceRecoveryRequest) (ProjectSwarmRecoveryAvailability, error),
	authorize func(context.Context, ProjectSwarmFenceRecoveryRequest) error,
	acquire func(context.Context, ProjectSwarmFenceRecoveryRequest) (ProjectSwarmRecoveryIntent, func(), error),
	prove func(context.Context, ProjectSwarmFenceRecoveryRequest, ProjectSwarmRecoveryIntent) (ProjectSwarmRecoveryProof, error),
	finalize func(context.Context, ProjectSwarmFenceRecoveryRequest, ProjectSwarmRecoveryIntent, ProjectSwarmRecoveryProof) error,
) {
	s.projectRunDispatchMu.Lock()
	defer s.projectRunDispatchMu.Unlock()
	s.projectSwarmFenceAvailability = availability
	s.projectSwarmFenceAuthorize = authorize
	s.projectSwarmFenceAcquire = acquire
	s.projectSwarmFenceProve = prove
	s.projectSwarmFenceFinalize = finalize
}

// Stable across later intent/proof/cancellation revision writes. The original
// claim time and both reviewed input digests are part of the operation binding.
func ProjectSwarmFenceOperationDigest(req ProjectSwarmFenceRecoveryRequest) string {
	return reviewDigest(struct {
		ProjectSwarmRecoveryRequest
		RequestDigest, ImplementationDigest string
	}{req.ProjectSwarmRecoveryRequest, req.RequestDigest, req.ImplementationDigest})
}

func validSwarmFenceIdentity(value string) bool {
	return value != "" && len(value) <= 200 && !strings.ContainsAny(value, "\x00\r\n")
}

func validSwarmRecoveryIntent(intent ProjectSwarmRecoveryIntent, req ProjectSwarmFenceRecoveryRequest) bool {
	requested, e1 := time.Parse(time.RFC3339Nano, intent.RequestedAt)
	claimed, e2 := time.Parse(time.RFC3339Nano, intent.OriginalUpdatedAt)
	fence := intent.FenceRequest
	return e1 == nil && e2 == nil && !requested.Before(claimed) &&
		validSwarmFenceIdentity(intent.ID) && validSwarmFenceIdentity(intent.RequestedBy) && validSwarmFenceIdentity(intent.LeaseID) &&
		intent.Protocol == backend.LaunchFenceProtocol && intent.QueueID == req.QueueID && intent.OriginalUpdatedAt == req.UpdatedAt &&
		intent.RequestDigest == req.RequestDigest && hexDigest(intent.RequestDigest) && intent.ImplementationDigest == req.ImplementationDigest && hexDigest(intent.ImplementationDigest) &&
		hexDigest(intent.ScopeDigest) && fence.ScopeDigest == intent.ScopeDigest && fence.IntentID == intent.ID && fence.SessionID == req.ChildID &&
		validSwarmFenceIdentity(fence.EngineID) && validSwarmFenceIdentity(fence.InstanceID) && validSwarmFenceIdentity(fence.SandboxID) &&
		(fence.ExpectedOriginalRef == "" || hexDigest(fence.ExpectedOriginalRef)) && fence.OperationDigest == ProjectSwarmFenceOperationDigest(req) &&
		(req.QueueID == "" || intent.LeaseID == req.QueueID)
}

func swarmFenceProofMatches(proof ProjectSwarmRecoveryProof, intent ProjectSwarmRecoveryIntent, queueID string) bool {
	fence := proof.FenceProof
	request := intent.FenceRequest
	acknowledged, e := time.Parse(time.RFC3339Nano, fence.AcknowledgedAt)
	requested, e2 := time.Parse(time.RFC3339Nano, intent.RequestedAt)
	return validSwarmRecoveryProof(proof, request.SessionID) && proof.Method == ProjectSwarmFenceRecoveryMethod && proof.QueueID == queueID && proof.LeaseID == intent.LeaseID &&
		fence.EngineID == request.EngineID && fence.InstanceID == request.InstanceID && fence.SandboxID == request.SandboxID && fence.SessionID == request.SessionID &&
		fence.IntentID == intent.ID && fence.ScopeDigest == intent.ScopeDigest && fence.OperationDigest == request.OperationDigest &&
		e == nil && e2 == nil && !acknowledged.Before(requested)
}

func (s *Server) swarmFenceRecoveryRequest(run ProjectRun, op ProjectRunOperation) (ProjectSwarmFenceRecoveryRequest, error) {
	queueID, e := s.SwarmQueueIDForOperation(run.ProjectID, run.ID, op.ID)
	if e != nil {
		return ProjectSwarmFenceRecoveryRequest{}, e
	}
	req := ProjectSwarmFenceRecoveryRequest{ProjectSwarmRecoveryRequest: ProjectSwarmRecoveryRequest{ProjectID: run.ProjectID, RunID: run.ID, OperationID: op.ID, ChildID: op.ChildID, QueueID: queueID, UpdatedAt: op.UpdatedAt}, RequestDigest: op.RequestDigest, ImplementationDigest: implementationDigest(op.Implementation)}
	if op.LaunchRecovery != nil {
		saved := *op.LaunchRecovery
		req.Intent = &saved
		req.UpdatedAt = saved.OriginalUpdatedAt
	}
	return req, nil
}

// Read-only lookup intentionally avoids the dispatch mutex already held by
// finalization. The API still checks current authority and exact retained fence.
func (s *Server) ProjectSwarmLaunchRecovery(projectID, runID, operationID, childID, queueID string) (*ProjectSwarmRecoveryIntent, *ProjectSwarmRecoveryProof, error) {
	run, e := s.GetProjectSwarmRun(projectID, runID)
	if e != nil {
		return nil, nil, e
	}
	op := swarmOperation(&run, operationID)
	if op == nil || op.TaskID == "" || op.ChildID != childID || op.LaunchRecovery == nil {
		return nil, nil, ErrProjectRunNotFound
	}
	req, e := s.swarmFenceRecoveryRequest(run, *op)
	if e != nil || req.QueueID != queueID || !validSwarmRecoveryIntent(*op.LaunchRecovery, req) {
		return nil, nil, ErrProjectRunInvalid
	}
	intent := *op.LaunchRecovery
	var proof *ProjectSwarmRecoveryProof
	if op.LaunchRecoveryProof != nil {
		copy := *op.LaunchRecoveryProof
		proof = &copy
	}
	return &intent, proof, nil
}

func (s *Server) projectSwarmRecoveryAvailability(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	if _, e := management.CaptureExecutionControlActor(r.Context(), p.ID); e != nil {
		journeyError(w, http.StatusForbidden, "execution_control_forbidden", e.Error())
		return
	}
	run, e := s.projectRuns.Get(p.ID, p.Path, r.PathValue("runId"))
	if e != nil {
		projectRunProblem(w, e)
		return
	}
	op := swarmOperation(&run, r.PathValue("operationId"))
	if op == nil || op.TaskID == "" {
		projectRunProblem(w, ErrProjectRunNotFound)
		return
	}
	req, e := s.swarmFenceRecoveryRequest(run, *op)
	if e != nil {
		projectRunProblem(w, e)
		return
	}
	availability := ProjectSwarmRecoveryAvailability{ReasonCode: "recovery_unsupported", Message: "This execution service cannot safely stop an uncertain launch. Its output and capacity remain retained."}
	if s.projectSwarmFenceAvailability != nil {
		availability, e = s.projectSwarmFenceAvailability(r.Context(), req)
		if e != nil {
			projectRunProblem(w, e)
			return
		}
	}
	if e := eligibleSwarmFenceRecovery(run, op, op.ChildID); e != nil {
		availability = ProjectSwarmRecoveryAvailability{ReasonCode: "attempt_not_eligible", Message: e.Error()}
	}
	journeyJSON(w, http.StatusOK, map[string]any{"expectedRevision": run.Revision, "operationId": op.ID, "childId": op.ChildID, "availability": availability})
}

func eligibleSwarmFenceRecovery(run ProjectRun, op *ProjectRunOperation, childID string) error {
	if op == nil || op.TaskID == "" || op.ChildID != childID || op.Allocation != "ready" {
		return errors.New("Select the exact retained operation and child before stopping an uncertain launch.")
	}
	task := swarmTask(&run, op.TaskID)
	if task == nil || task.OperationID != op.ID || task.Integration != nil {
		return errors.New("Resolve this task's retained integration before recovering its launch.")
	}
	if op.AbsenceRetirement != nil && op.AbsenceRetirement.Method != ProjectSwarmFenceRecoveryMethod {
		return errors.New("Existing missing-child recovery is a separate action; continue that recovery.")
	}
	switch op.Status {
	case "dispatching", "unknown", "running", "failed", "review-required":
	case "cancelled":
		if op.LaunchRecovery == nil {
			return errors.New("This cancelled attempt has no uncertain-launch recovery intent.")
		}
	default:
		return errors.New("This action only recovers a supported uncertain launch.")
	}
	return nil
}

func (s *Server) recoverFencedSwarmChildLocked(ctx context.Context, p project.Project, run ProjectRun, operationID, childID string) (ProjectRun, error) {
	if _, e := management.CaptureExecutionControlActor(ctx, p.ID); e != nil {
		return run, e
	}
	op := swarmOperation(&run, operationID)
	if e := eligibleSwarmFenceRecovery(run, op, childID); e != nil {
		return run, e
	}
	if s.projectSwarmFenceAuthorize == nil || s.projectSwarmFenceAcquire == nil || s.projectSwarmFenceProve == nil || s.projectSwarmFenceFinalize == nil {
		return run, errors.New("The execution service does not support guarded launch recovery. Output and capacity remain retained.")
	}
	req, e := s.swarmFenceRecoveryRequest(run, *op)
	if e != nil {
		return run, e
	}
	observedOperationTime := op.UpdatedAt
	intent, release, e := s.projectSwarmFenceAcquire(ctx, req)
	if release != nil {
		defer release()
	}
	if e != nil {
		return run, e
	}
	// The API may have published the exact private intent before a previous
	// product CAS failed. Recover that original claim time while still fencing
	// this write against the current observed revision and operation timestamp.
	if req.Intent == nil {
		original, parseOriginal := time.Parse(time.RFC3339Nano, intent.OriginalUpdatedAt)
		current, parseCurrent := time.Parse(time.RFC3339Nano, observedOperationTime)
		if parseOriginal != nil || parseCurrent != nil || original.After(current) {
			return run, ErrProjectRunConflict
		}
		req.UpdatedAt = intent.OriginalUpdatedAt
	}
	if release == nil || !validSwarmRecoveryIntent(intent, req) || req.Intent != nil && intent != *req.Intent {
		return run, errors.New("The guarded recovery intent does not match this exact launch. Nothing was stopped or released.")
	}
	// The guard callback may wait for another process. Recheck revision and exact
	// operation after acquisition and before any durable intent/backend effects.
	current, e := s.projectRuns.Get(p.ID, p.Path, run.ID)
	if e != nil {
		return run, e
	}
	currentOp := swarmOperation(&current, operationID)
	if current.Revision != run.Revision || currentOp == nil || currentOp.ChildID != childID || currentOp.RequestDigest != op.RequestDigest {
		return run, ErrProjectRunConflict
	}
	if e = s.projectSwarmFenceAuthorize(ctx, req); e != nil {
		return run, e
	}
	if op.LaunchRecovery == nil {
		run, e = s.projectRuns.mutateSwarm(p.ID, p.Path, run.ID, run.Revision, func(current *ProjectRun) error {
			target := swarmOperation(current, operationID)
			if target == nil || target.ChildID != childID || target.UpdatedAt != observedOperationTime || target.LaunchRecovery != nil {
				return ErrProjectRunConflict
			}
			copy := intent
			target.LaunchRecovery = &copy
			target.Message = "Stop-and-retain recovery is pending. The exact launch may be stopped; files and frozen context remain retained, and capacity is still held."
			current.Reason = target.Message
			return nil
		})
		if e != nil {
			return run, e
		}
	}
	req.Intent = &intent
	op = swarmOperation(&run, operationID)
	var proof ProjectSwarmRecoveryProof
	if op.LaunchRecoveryProof != nil {
		proof = *op.LaunchRecoveryProof
	} else {
		proof, e = s.projectSwarmFenceProve(ctx, req, intent)
		if e != nil {
			return run, e
		}
		observed, parseErr := time.Parse(time.RFC3339Nano, proof.ObservedAt)
		now := time.Now().UTC()
		if !swarmFenceProofMatches(proof, intent, req.QueueID) || proof.FinalizedAt != "" || parseErr != nil || observed.After(now) || now.Sub(observed) > 5*time.Second {
			return run, errors.New("The acknowledged recovery fence is stale or does not match this launch. Output and capacity remain retained.")
		}
		// Publish backend acknowledgement separately, before operation cancellation.
		run, e = s.projectRuns.mutateSwarm(p.ID, p.Path, run.ID, run.Revision, func(current *ProjectRun) error {
			target := swarmOperation(current, operationID)
			if target == nil || target.ChildID != childID || target.LaunchRecovery == nil || *target.LaunchRecovery != intent || target.LaunchRecoveryProof != nil {
				return ErrProjectRunConflict
			}
			copy := proof
			target.LaunchRecoveryProof = &copy
			target.Message = "The launch fence was acknowledged and saved. Recovery still holds capacity until exact retirement finishes."
			current.Reason = target.Message
			return nil
		})
		if e != nil {
			return run, e
		}
	}
	op = swarmOperation(&run, operationID)
	if op.AbsenceRetirement != nil && op.AbsenceRetirement.FinalizedAt != "" {
		return run, nil
	}
	if op.AbsenceRetirement == nil {
		run, e = s.projectRuns.mutateSwarm(p.ID, p.Path, run.ID, run.Revision, func(current *ProjectRun) error {
			target := swarmOperation(current, operationID)
			if target == nil || target.ChildID != childID || target.LaunchRecoveryProof == nil || *target.LaunchRecoveryProof != proof {
				return ErrProjectRunConflict
			}
			copy := proof
			target.AbsenceRetirement = &copy
			target.Status = "cancelled"
			target.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
			target.Message = "The operator stopped the uncertain launch behind an acknowledged permanent recovery marker. Output remains retained; its execution reservation is not yet confirmed released."
			current.Reason = target.Message
			return nil
		})
		if e != nil {
			return run, e
		}
	}
	if e = s.projectSwarmFenceFinalize(ctx, req, intent, proof); e != nil {
		return run, e
	}
	if req.QueueID != "" {
		if e = s.schedulerUpdate(req.QueueID, func(item *swarmQueueRecord) error {
			if item.ProjectID != p.ID || item.RunID != run.ID || item.OperationID != "" && item.OperationID != operationID || item.ChildID != "" && item.ChildID != childID {
				return ErrProjectRunConflict
			}
			item.OperationID = operationID
			item.ChildID = childID
			item.MemoryMiB = 0
			item.CPUMillis = 0
			schedulerReason(item, "cancelled", "uncertain_launch_retired", "The exact uncertain launch was stopped and retired. Retained output remains available; a fresh attempt requires new consent.")
			return nil
		}); e != nil {
			return run, e
		}
	}
	run, e = s.projectRuns.mutateSwarm(p.ID, p.Path, run.ID, run.Revision, func(current *ProjectRun) error {
		target := swarmOperation(current, operationID)
		if target == nil || target.ChildID != childID || target.Status != "cancelled" || target.AbsenceRetirement == nil || *target.AbsenceRetirement != proof || target.LaunchRecovery == nil || *target.LaunchRecovery != intent {
			return ErrProjectRunConflict
		}
		target.AbsenceRetirement.FinalizedAt = time.Now().UTC().Format(time.RFC3339Nano)
		target.UpdatedAt = target.AbsenceRetirement.FinalizedAt
		target.Message = "The uncertain launch was retired and its exact reservation released. Output and frozen context remain retained; its permanent Docker recovery marker must remain in place. A fresh attempt requires new consent."
		current.Reason = target.Message
		return nil
	})
	if e == nil {
		s.WakeSwarmScheduler()
	}
	return run, e
}

// Public views carry progress only. Private intent, actor attribution, original
// daemon/container identities and acknowledged fence evidence stay in storage.
func publicSwarmOperation(op ProjectRunOperation) ProjectRunOperation {
	op.UnclaimedRetirement = false
	op.Recovery = nil
	if op.LaunchRecovery != nil {
		view := &ProjectSwarmRecoveryView{State: "pending", RequestedAt: op.LaunchRecovery.RequestedAt}
		if op.LaunchRecoveryProof != nil {
			view.State = "fenced"
			view.ProvedAt = op.LaunchRecoveryProof.ObservedAt
		}
		if op.AbsenceRetirement != nil && op.AbsenceRetirement.FinalizedAt != "" {
			view.State = "finalized"
			view.FinalizedAt = op.AbsenceRetirement.FinalizedAt
		}
		op.Recovery = view
	}
	op.LaunchRecovery = nil
	op.LaunchRecoveryProof = nil
	if op.AbsenceRetirement != nil && op.AbsenceRetirement.Method == ProjectSwarmFenceRecoveryMethod {
		op.AbsenceRetirement = nil
	}
	return op
}

func validSwarmOperationRecovery(run ProjectRun, op ProjectRunOperation) bool {
	if op.Recovery != nil {
		return false
	}
	if op.LaunchRecovery == nil {
		return op.LaunchRecoveryProof == nil && (op.AbsenceRetirement == nil || op.AbsenceRetirement.Method != ProjectSwarmFenceRecoveryMethod)
	}
	intent := *op.LaunchRecovery
	req := ProjectSwarmFenceRecoveryRequest{ProjectSwarmRecoveryRequest: ProjectSwarmRecoveryRequest{ProjectID: run.ProjectID, RunID: run.ID, OperationID: op.ID, ChildID: op.ChildID, QueueID: intent.QueueID, UpdatedAt: intent.OriginalUpdatedAt}, RequestDigest: op.RequestDigest, ImplementationDigest: implementationDigest(op.Implementation)}
	if op.Allocation != "ready" || op.Status == "reserved" || !validSwarmRecoveryIntent(intent, req) {
		return false
	}
	if op.LaunchRecoveryProof != nil && (!swarmFenceProofMatches(*op.LaunchRecoveryProof, intent, intent.QueueID) || op.LaunchRecoveryProof.FinalizedAt != "") {
		return false
	}
	if op.AbsenceRetirement != nil {
		proof := *op.AbsenceRetirement
		proof.FinalizedAt = ""
		if op.LaunchRecoveryProof == nil || proof != *op.LaunchRecoveryProof {
			return false
		}
	}
	return op.Status != "cancelled" || op.AbsenceRetirement != nil
}
