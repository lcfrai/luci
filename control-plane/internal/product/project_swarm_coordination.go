package product

import (
	"strings"
	"time"
)

const maxSwarmMessages = 512
const maxSwarmReviewObligations = 200
const maxSwarmReviewRepairs = 2

// Messages are retained inside the existing scoped run receipt. They are
// attributed claims, never execution instructions, permission or check proof.
type ProjectSwarmMessageSender struct {
	TaskID        string `json:"taskId"`
	OperationID   string `json:"operationId"`
	ChildID       string `json:"childId"`
	WorktreeID    string `json:"worktreeId"`
	ContextDigest string `json:"contextDigest"`
}

type ProjectSwarmMessageEvidence struct {
	Kind   string `json:"kind"`
	ID     string `json:"id"`
	Digest string `json:"digest"`
}

type ProjectSwarmMessageAck struct {
	OperationID string `json:"operationId"`
	ChildID     string `json:"childId"`
	ActorID     string `json:"actorId"`
	At          string `json:"at"`
}

type ProjectSwarmMessage struct {
	ID              string                      `json:"id"`
	Sequence        int                         `json:"sequence"`
	Kind            string                      `json:"kind"`
	Sender          ProjectSwarmMessageSender   `json:"sender"`
	RecipientTaskID string                      `json:"recipientTaskId"`
	CausalMessageID string                      `json:"causalMessageId,omitempty"`
	Evidence        ProjectSwarmMessageEvidence `json:"evidence"`
	Summary         string                      `json:"summary"`
	Attribution     string                      `json:"attribution"`
	CreatedAt       string                      `json:"createdAt"`
	Ack             *ProjectSwarmMessageAck     `json:"ack"`
	KeyDigest       string                      `json:"keyDigest"`
	RequestDigest   string                      `json:"requestDigest"`
}

// Review obligations require a different exact task attempt. A failed
// review retains its finding and the linked repair attempt; passing review
// alone still cannot integrate or accept an unverified app.
type ProjectSwarmReviewObligation struct {
	ID                    string `json:"id"`
	TaskID                string `json:"taskId"`
	CandidateOperationID  string `json:"candidateOperationId"`
	CandidateChildID      string `json:"candidateChildId"`
	CandidateCommit       string `json:"candidateCommit"`
	ContextDigest         string `json:"contextDigest"`
	ReviewDigest          string `json:"reviewDigest"`
	ReviewerOperationID   string `json:"reviewerOperationId"`
	ReviewerChildID       string `json:"reviewerChildId"`
	ReviewerHandoffDigest string `json:"reviewerHandoffDigest,omitempty"`
	FindingMessageID      string `json:"findingMessageId,omitempty"`
	RepairOperationID     string `json:"repairOperationId,omitempty"`
	State                 string `json:"state"` // pending, passed, repair-required, unknown
	Attempt               int    `json:"attempt"`
	CreatedAt             string `json:"createdAt"`
	ResolvedAt            string `json:"resolvedAt,omitempty"`
	KeyDigest             string `json:"keyDigest"`
	RequestDigest         string `json:"requestDigest"`
}

// The public view keeps the complete immutable obligation plus bounded repair
// accounting derived from the same run revision. No extra store read occurs.
type ProjectSwarmReviewObligationView struct {
	ProjectSwarmReviewObligation
	RepairLimit     int `json:"repairLimit"`
	RepairsConsumed int `json:"repairsConsumed"`
}

func swarmReviewObligationView(run ProjectRun, item ProjectSwarmReviewObligation) ProjectSwarmReviewObligationView {
	consumed := 0
	if run.Swarm != nil {
		for _, prior := range run.Swarm.ReviewObligations {
			if prior.TaskID == item.TaskID && prior.State == "repair-required" {
				consumed++
			}
		}
	}
	return ProjectSwarmReviewObligationView{ProjectSwarmReviewObligation: item, RepairLimit: maxSwarmReviewRepairs, RepairsConsumed: consumed}
}

func swarmReviewObligationViews(run ProjectRun) []ProjectSwarmReviewObligationView {
	views := []ProjectSwarmReviewObligationView{}
	if run.Swarm != nil {
		for _, item := range run.Swarm.ReviewObligations {
			views = append(views, swarmReviewObligationView(run, item))
		}
	}
	return views
}

func swarmMessageKind(kind string) bool {
	switch kind {
	case "discovery", "question", "dependency-result", "review-finding", "decision", "lifecycle-request":
		return true
	}
	return false
}

func validateSwarmCoordination(run ProjectRun) error {
	if run.Swarm == nil {
		return nil
	}
	if len(run.Swarm.Messages) > maxSwarmMessages || len(run.Swarm.ReviewObligations) > maxSwarmReviewObligations {
		return ErrProjectRunLimit
	}
	messageIDs := make(map[string]bool, len(run.Swarm.Messages))
	for i, msg := range run.Swarm.Messages {
		op := swarmOperation(&run, msg.Sender.OperationID)
		if msg.Sequence != i+1 || !prototypeReleaseID.MatchString(msg.ID) || messageIDs[msg.ID] || !swarmMessageKind(msg.Kind) ||
			op == nil || op.TaskID != msg.Sender.TaskID || op.ChildID != msg.Sender.ChildID || op.WorktreeID != msg.Sender.WorktreeID ||
			!hexDigest(msg.Sender.ContextDigest) || swarmTask(&run, msg.RecipientTaskID) == nil ||
			!handoffText(msg.Summary, 1000) || strings.TrimSpace(msg.Summary) == "" || !hexDigest(msg.KeyDigest) || !hexDigest(msg.RequestDigest) ||
			(msg.Attribution != "coordinator-recorded" && msg.Attribution != "owned-worker-report") {
			return ErrProjectRunInvalid
		}
		if msg.CausalMessageID != "" && !messageIDs[msg.CausalMessageID] {
			return ErrProjectRunInvalid
		}
		if !validSwarmMessageEvidenceShape(msg.Evidence) {
			return ErrProjectRunInvalid
		}
		if _, err := time.Parse(time.RFC3339Nano, msg.CreatedAt); err != nil {
			return ErrProjectRunInvalid
		}
		if msg.Ack != nil {
			ackOp := swarmOperation(&run, msg.Ack.OperationID)
			if ackOp == nil || ackOp.TaskID != msg.RecipientTaskID || ackOp.ChildID != msg.Ack.ChildID || !validConnectionToken(msg.Ack.ActorID, 128, false) {
				return ErrProjectRunInvalid
			}
			if _, err := time.Parse(time.RFC3339Nano, msg.Ack.At); err != nil {
				return ErrProjectRunInvalid
			}
		}
		messageIDs[msg.ID] = true
	}
	obligations := map[string]bool{}
	obligationKeys := map[string]bool{}
	for _, item := range run.Swarm.ReviewObligations {
		candidate, reviewer := swarmOperation(&run, item.CandidateOperationID), swarmOperation(&run, item.ReviewerOperationID)
		if !prototypeReleaseID.MatchString(item.ID) || obligations[item.ID] || obligationKeys[item.KeyDigest] || candidate == nil || reviewer == nil || candidate.ID == reviewer.ID ||
			candidate.TaskID != item.TaskID || candidate.ChildID != item.CandidateChildID || reviewer.ChildID != item.ReviewerChildID || reviewer.TaskID == item.TaskID ||
			!deliverySHA.MatchString(item.CandidateCommit) || !hexDigest(item.ContextDigest) || !hexDigest(item.ReviewDigest) ||
			!hexDigest(item.KeyDigest) || !hexDigest(item.RequestDigest) || item.Attempt < 1 || item.Attempt > projectRunOperationLimit ||
			(item.State != "pending" && item.State != "passed" && item.State != "repair-required" && item.State != "unknown") {
			return ErrProjectRunInvalid
		}
		if item.FindingMessageID != "" && !messageIDs[item.FindingMessageID] {
			return ErrProjectRunInvalid
		}
		if item.State == "pending" && (item.ReviewerHandoffDigest != "" || item.FindingMessageID != "" || item.ResolvedAt != "") ||
			(item.State == "passed" || item.State == "repair-required") && (!hexDigest(item.ReviewerHandoffDigest) || item.FindingMessageID == "" || item.ResolvedAt == "") {
			return ErrProjectRunInvalid
		}
		if item.RepairOperationID != "" {
			repair := swarmOperation(&run, item.RepairOperationID)
			if repair == nil || repair.TaskID != item.TaskID || repair.ID == candidate.ID {
				return ErrProjectRunInvalid
			}
		}
		if _, err := time.Parse(time.RFC3339Nano, item.CreatedAt); err != nil {
			return ErrProjectRunInvalid
		}
		if item.ResolvedAt != "" {
			if _, err := time.Parse(time.RFC3339Nano, item.ResolvedAt); err != nil {
				return ErrProjectRunInvalid
			}
		}
		obligations[item.ID] = true
		obligationKeys[item.KeyDigest] = true
	}
	return nil
}

func validateSwarmMessageEvidence(op ProjectRunOperation, evidence ProjectSwarmMessageEvidence) error {
	if !validSwarmMessageEvidenceShape(evidence) {
		return ErrProjectRunConflict
	}
	switch evidence.Kind {
	case "none":
		if evidence.ID == "" && evidence.Digest == "" {
			return nil
		}
	case "handoff":
		if op.Handoff != nil && evidence.ID == op.ID && evidence.Digest == op.Handoff.Digest {
			return nil
		}
	case "source-commit":
		if deliverySHA.MatchString(evidence.ID) && evidence.Digest == implementationDigest(evidence.ID) && (evidence.ID == op.BaseCommit || op.Checkpoint != nil && evidence.ID == op.Checkpoint.SeedCommit) {
			return nil
		}
	}
	return ErrProjectRunConflict
}

func validSwarmMessageEvidenceShape(evidence ProjectSwarmMessageEvidence) bool {
	switch evidence.Kind {
	case "none":
		return evidence.ID == "" && evidence.Digest == ""
	case "handoff":
		return prototypeReleaseID.MatchString(evidence.ID) && hexDigest(evidence.Digest)
	case "source-commit":
		return deliverySHA.MatchString(evidence.ID) && hexDigest(evidence.Digest)
	default:
		return false
	}
}

func appendSwarmMessage(run *ProjectRun, msg ProjectSwarmMessage) (ProjectSwarmMessage, error) {
	if run.Swarm == nil || len(run.Swarm.Messages) >= maxSwarmMessages {
		return ProjectSwarmMessage{}, ErrProjectRunLimit
	}
	for _, prior := range run.Swarm.Messages {
		if prior.Sender.OperationID == msg.Sender.OperationID && prior.KeyDigest == msg.KeyDigest {
			if prior.RequestDigest == msg.RequestDigest {
				return prior, nil
			}
			return ProjectSwarmMessage{}, ErrProjectRunKeyConflict
		}
	}
	msg.Sequence = len(run.Swarm.Messages) + 1
	run.Swarm.Messages = append(run.Swarm.Messages, msg)
	if err := validateSwarmCoordination(*run); err != nil {
		return ProjectSwarmMessage{}, err
	}
	return msg, nil
}
