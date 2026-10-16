package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
)

type checkExecutionFenceRecord struct {
	Scope               string                       `json:"scope"`
	Identity            backend.CheckIdentity        `json:"identity"`
	Actor               management.ExecutionActorRef `json:"actor"`
	IntentID            string                       `json:"intentId"`
	ExpectedOriginalRef string                       `json:"expectedOriginalRef,omitempty"`
	State               string                       `json:"state"`
	Proof               backend.CheckFenceProof      `json:"proof,omitzero"`
	CreatedAt           time.Time                    `json:"createdAt"`
	UpdatedAt           time.Time                    `json:"updatedAt"`
}

func validCheckFenceProof(record checkExecutionFenceRecord) bool {
	p := record.Proof
	ack, e1 := time.Parse(time.RFC3339Nano, p.AcknowledgedAt)
	seen, e2 := time.Parse(time.RFC3339Nano, p.ObservedAt)
	return p.Identity == record.Identity && p.IntentID == record.IntentID && checkHex(p.FenceRef, 32) && checkHex(p.ConfigDigest, 32) && e1 == nil && e2 == nil && !ack.Before(record.CreatedAt) && !seen.Before(ack) && !seen.After(time.Now().UTC())
}

func validateCheckFenceRecords(l executionLeaseLedger) error {
	if len(l.Fences)+len(l.CheckFences) > backend.MaxLaunchFences {
		return errors.New("execution fence history exceeds its permanent host limit")
	}
	seen := map[string]bool{}
	for _, record := range l.CheckFences {
		key := record.Scope + ":" + record.Identity.AllocationID
		if !validCheckIdentity(record.Identity) || record.Scope != record.Identity.ScopeDigest || !checkHex(record.IntentID, 16) || seen[key] || record.CreatedAt.IsZero() || record.UpdatedAt.Before(record.CreatedAt) || record.Actor.ProjectID != record.Identity.ProjectID || len(record.Actor.UserID) > 200 || len(record.Actor.TeamID) > 200 || record.Actor.Local && (record.Actor.UserID != "" || record.Actor.TeamID != "") || !record.Actor.Local && record.Actor.UserID == "" || record.ExpectedOriginalRef != "" && !checkHex(record.ExpectedOriginalRef, 32) {
			return errors.New("check fence ownership cannot be verified")
		}
		seen[key] = true
		switch record.State {
		case "intent":
			if record.Proof != (backend.CheckFenceProof{}) {
				return errors.New("unacknowledged check fence has a proof")
			}
		case "proved", "retired":
			if !validCheckFenceProof(record) {
				return errors.New("check fence proof cannot be verified")
			}
		default:
			return errors.New("check fence phase cannot be verified")
		}
		matches := 0
		for _, lease := range l.Checks {
			if lease.Scope == record.Scope && lease.ID == record.Identity.AllocationID {
				matches++
				if lease.Identity != record.Identity || record.State == "retired" && lease.State != "finished" || record.State != "retired" && lease.State != "fencing" {
					return errors.New("check fence capacity binding changed")
				}
			}
		}
		if record.State != "retired" && matches != 1 || matches > 1 {
			return errors.New("check fence has no exact capacity claim")
		}
	}
	for _, lease := range l.Checks {
		if lease.State == "fencing" && !seen[lease.Scope+":"+lease.ID] {
			return errors.New("fenced check has no durable recovery intent")
		}
	}
	return nil
}

func checkRecoveryDigest(row product.ProjectCheckRun, lease checkExecutionLease) string {
	data, _ := json.Marshal(struct {
		CheckID  string
		Revision int
		Lease    checkExecutionLease
	}{row.ID, row.Revision, lease})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (a *API) exactCheckLease(projectID, checkID string) (*checkExecutionLease, error) {
	l, err := a.execution.readFenceLedger()
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var found, finished *checkExecutionLease
	for _, v := range l.Checks {
		if v.Scope == a.execution.scope && v.Identity.ProjectID == projectID && v.Identity.CheckID == checkID {
			if v.State == "finished" {
				copy := v
				if finished == nil || copy.Identity.CommandIndex > finished.Identity.CommandIndex {
					finished = &copy
				}
				continue
			}
			if found != nil {
				return nil, errors.New("more than one exact command claim needs review")
			}
			copy := v
			found = &copy
		}
	}
	if found == nil {
		found = finished
	}
	return found, nil
}

func (a *API) projectCheckRecovery(ctx context.Context, projectID, checkID string) (product.ProjectCheckRecoveryView, error) {
	row, err := a.deps.Product.GetProjectCheckRun(projectID, checkID)
	if err != nil {
		return product.ProjectCheckRecoveryView{}, err
	}
	view := product.ProjectCheckRecoveryView{ProjectID: projectID, CheckID: checkID, Revision: row.Revision, State: "unavailable", ReasonCode: "check_recovery_not_required", Message: "This check has no retained allocation requiring recovery."}
	lease, err := a.exactCheckLease(projectID, checkID)
	if err != nil {
		return view, err
	}
	if lease == nil {
		if row.ResourcesHeld != nil && !*row.ResourcesHeld && row.Status != "queued" && row.Status != "running" {
			view.State = "complete"
		}
		return view, nil
	}
	if row.Revision < 1 || lease.Identity.CommandIndex >= len(row.Commands) || row.Commands[lease.Identity.CommandIndex].AllocationID != lease.ID {
		return view, errors.New("the exact approved command receipt does not match retained execution")
	}
	if lease.State == "finished" {
		view.State, view.ReasonCode, view.Message = "pending", "check_result_publication_pending", "Execution cleanup is confirmed; retained result publication is pending."
		return view, nil
	}
	if _, ok := a.deps.Backend.(backend.CheckExecutor); !ok {
		view.ReasonCode, view.Message = "check_recovery_unsupported", "This runtime cannot prove isolated check recovery."
		return view, nil
	}
	method := "remove-exact"
	if lease.State == "creating" || lease.State == "fencing" {
		method = "fence-launch"
	}
	digest, index := checkRecoveryDigest(row, *lease), lease.Identity.CommandIndex
	view.State, view.Method, view.CommandIndex, view.RecoveryDigest = "available", &method, &index, &digest
	view.ReasonCode, view.Message = "check_stop_review_required", "Stop only this exact check allocation and retain its recorded output. No command will be replayed."
	if lease.State == "fencing" {
		view.State, view.ReasonCode, view.Message = "pending", "check_recovery_pending", "The prior reviewed check recovery remains incomplete. Review its current state to finish the same recovery."
	}
	return view, nil
}

func (a *API) recoverProjectCheck(ctx context.Context, projectID, checkID string, req product.ProjectCheckRecoveryRequest) (product.ProjectCheckRun, error) {
	refuse := func(code, message string) (product.ProjectCheckRun, error) {
		return product.ProjectCheckRun{}, &product.CheckControlError{Status: 409, Code: code, Message: message}
	}
	if !req.AcknowledgeStop || req.ExpectedRevision < 1 || !checkHex(req.RecoveryDigest, 32) {
		return refuse("check_recovery_review_required", "Review the current check before confirming its exact stop and cleanup.")
	}
	if err := a.authorizeExecutionRetirement(ctx, projectID); err != nil {
		return product.ProjectCheckRun{}, &product.CheckControlError{Status: 403, Code: "check_control_forbidden", Message: "Current project edit permission is required to stop this check."}
	}
	lease, err := a.exactCheckLease(projectID, checkID)
	if err != nil || lease == nil {
		return refuse("check_recovery_changed", "The retained check allocation changed. Read its current state before another recovery.")
	}
	unlock, err := a.execution.lockChildLifecycle(ctx, lease.ID)
	if err != nil {
		return refuse("check_lifecycle_busy", "The exact check still has a current execution or recovery controller. Stop it or wait before reviewing recovery.")
	}
	defer unlock()
	if err = a.authorizeExecutionRetirement(ctx, projectID); err != nil {
		return product.ProjectCheckRun{}, &product.CheckControlError{Status: 403, Code: "check_control_forbidden", Message: "Current project edit permission is required to stop this check."}
	}
	current, err := a.exactCheckLease(projectID, checkID)
	if err != nil || current == nil || *current != *lease {
		return product.ProjectCheckRun{}, product.ErrProjectRunConflict
	}
	row, err := a.deps.Product.GetProjectCheckRun(projectID, checkID)
	if err != nil {
		return row, err
	}
	if row.Revision != req.ExpectedRevision || checkRecoveryDigest(row, *lease) != req.RecoveryDigest {
		return product.ProjectCheckRun{}, product.ErrProjectRunConflict
	}
	index := lease.Identity.CommandIndex
	if index >= len(row.Commands) || row.Commands[index].AllocationID != lease.ID {
		return refuse("check_recovery_changed", "The retained check no longer matches the approved command.")
	}
	row, err = a.deps.Product.BeginProjectCheckRecovery(projectID, checkID, req.ExpectedRevision)
	if err != nil {
		return row, err
	}
	command := row.Commands[index]
	out := product.ProjectCheckCommandOutcome{Status: "cancelled", Stage: "recovery", ReasonCode: "check_stopped_by_review", Message: "The reviewed exact check was stopped. Its prior output is retained; no command was replayed.", AllocationID: lease.ID, Output: command.Output, Truncated: command.Truncated, ResourcesHeld: true}
	if (command.Status == "passed" || command.Status == "failed") && command.ExitCode != nil {
		out.Status, out.ExitCode = command.Status, command.ExitCode
	}
	persist := func(value product.ProjectCheckCommandOutcome) error {
		next, e := a.deps.Product.RecordProjectCheckObservation(row, index, value)
		if e == nil {
			row = next
			out = value
		}
		return e
	}
	if lease.State == "reserved" {
		_, err = a.finishUnstartedCheck(ctx, lease.Identity, out, persist, false)
		return row, err
	}
	if lease.State != "creating" && lease.State != "fencing" {
		if err = a.authorizeExecutionRetirement(ctx, projectID); err != nil {
			return row, err
		}
		_, err = a.finishAcknowledgedCheck(ctx, lease.Identity, lease.Ref, out, persist, true, false, func(context.Context) error { return a.authorizeExecutionRetirement(ctx, projectID) })
		return row, err
	}
	executor, ok := a.deps.Backend.(backend.CheckExecutor)
	if !ok {
		return row, backend.ErrUnsupported
	}
	actor, err := management.CaptureExecutionControlActor(ctx, projectID)
	if err != nil {
		return row, err
	}
	record, err := a.execution.reserveCheckFence(ctx, *lease, actor)
	if err != nil {
		return row, err
	}
	if err = persist(out); err != nil {
		return row, err
	}
	request := backend.CheckFenceRequest{Identity: record.Identity, IntentID: record.IntentID, ExpectedOriginalRef: record.ExpectedOriginalRef}
	if err = a.authorizeExecutionRetirement(ctx, projectID); err != nil {
		return row, err
	}
	proof := record.Proof
	if record.State == "intent" {
		proof, err = executor.FenceCheckLaunch(ctx, request)
	} else {
		err = executor.VerifyCheckLaunchFence(ctx, request, proof)
	}
	if err != nil {
		return row, err
	}
	if err = a.execution.publishCheckFence(ctx, record, proof); err != nil {
		return row, err
	}
	if err = a.authorizeExecutionRetirement(ctx, projectID); err != nil {
		return row, err
	}
	if err = executor.VerifyCheckLaunchFence(ctx, request, proof); err != nil {
		return row, err
	}
	if err = a.authorizeExecutionRetirement(ctx, projectID); err != nil {
		return row, err
	}
	out.CleanupConfirmed = true
	if err = a.execution.completeFencedCheck(ctx, record, proof, out); err != nil {
		return row, err
	}
	out.ResourcesHeld = false
	if err = persist(out); err != nil {
		return row, err
	}
	if err = a.execution.acknowledgeCheckCompletion(ctx, lease.Identity); err != nil {
		return row, err
	}
	return row, nil
}

func (g *executionAdmission) reserveCheckFence(ctx context.Context, lease checkExecutionLease, actor management.ExecutionActorRef) (checkExecutionFenceRecord, error) {
	var out checkExecutionFenceRecord
	err := g.mutate(ctx, func(l *executionLeaseLedger) error {
		for _, v := range l.CheckFences {
			if v.Scope == g.scope && v.Identity.AllocationID == lease.ID {
				if v.Identity != lease.Identity || v.State == "retired" {
					return errors.New("check recovery identity changed")
				}
				out = v
				return nil
			}
		}
		if executionFenceCount(*l) >= backend.MaxLaunchFences {
			return errors.New("the permanent launch recovery fence limit is occupied")
		}
		for i := range l.Checks {
			v := &l.Checks[i]
			if *v == lease && v.Scope == g.scope && v.State == "creating" {
				var nonce [16]byte
				if _, err := rand.Read(nonce[:]); err != nil {
					return err
				}
				now := time.Now().UTC()
				out = checkExecutionFenceRecord{Scope: g.scope, Identity: v.Identity, Actor: actor, IntentID: hex.EncodeToString(nonce[:]), ExpectedOriginalRef: v.Ref, State: "intent", CreatedAt: now, UpdatedAt: now}
				v.State, v.UpdatedAt = "fencing", now
				l.CheckFences = append(l.CheckFences, out)
				return nil
			}
		}
		return errors.New("the unacknowledged check launch changed before recovery")
	})
	return out, err
}

func (g *executionAdmission) publishCheckFence(ctx context.Context, record checkExecutionFenceRecord, proof backend.CheckFenceProof) error {
	return g.mutate(ctx, func(l *executionLeaseLedger) error {
		for i := range l.CheckFences {
			v := &l.CheckFences[i]
			if v.Scope == g.scope && v.IntentID == record.IntentID {
				if v.Identity != record.Identity || v.State == "retired" || v.State == "proved" && v.Proof != proof {
					return errors.New("check fence publication changed")
				}
				v.Proof, v.State, v.UpdatedAt = proof, "proved", time.Now().UTC()
				if !validCheckFenceProof(*v) {
					return errors.New("check fence acknowledgment cannot be verified")
				}
				return nil
			}
		}
		return errors.New("check fence intent is unavailable")
	})
}

func (g *executionAdmission) completeFencedCheck(ctx context.Context, record checkExecutionFenceRecord, proof backend.CheckFenceProof, out product.ProjectCheckCommandOutcome) error {
	if !out.CleanupConfirmed || out.AllocationID != record.Identity.AllocationID {
		return errors.New("fenced check outcome binding is invalid")
	}
	return g.mutate(ctx, func(l *executionLeaseLedger) error {
		for i := range l.CheckFences {
			v := &l.CheckFences[i]
			if v.Scope != g.scope || v.IntentID != record.IntentID {
				continue
			}
			if v.State != "proved" || v.Proof != proof || v.Identity != record.Identity {
				return errors.New("check recovery proof changed before capacity release")
			}
			for j := range l.Checks {
				lease := &l.Checks[j]
				if lease.Scope == g.scope && lease.Identity == record.Identity && lease.State == "fencing" {
					lease.State, lease.ResultDigest, lease.UpdatedAt = "finished", checkOutcomeDigest(out), time.Now().UTC()
					v.State, v.UpdatedAt = "retired", lease.UpdatedAt
					return nil
				}
			}
			return errors.New("check recovery lost its exact retained capacity claim")
		}
		return errors.New("check recovery proof is unavailable")
	})
}
