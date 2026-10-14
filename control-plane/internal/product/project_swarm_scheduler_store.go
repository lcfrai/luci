package product

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
	"github.com/lcfrai/luci/control-plane/internal/management"
)

const swarmQueueLimit = MaxSwarmScheduledTasks
const swarmQueueFileLimit = 2 << 20

func defaultSwarmSchedulerPolicy() SwarmSchedulerPolicy {
	return SwarmSchedulerPolicy{MaxActive: 4, MaxPerProject: 2, HeadroomPercent: 25, MinHeadroomMiB: 512, MaxHeadroomMiB: 1024, DiskReserveMiB: 2048}
}
func swarmSupersededStopReason(code string) bool {
	return code == "context_superseded_stop_pending" || code == "context_superseded_unallocated_claim" || code == "context_superseded_identity_pending" || code == "operator_stop_pending"
}

// The accepted plan can remove or rewire a dependency. Fence old running or
// waiting dependents using the graph saved with their own run, not only the
// new graph. This does not add removed tasks to the revised run.
func oldRunTaskAffected(run ProjectRun, taskID string, selected map[string]bool) bool {
	if selected[taskID] {
		return true
	}
	if run.Swarm == nil {
		return false
	}
	deps := make(map[string][]string, len(run.Swarm.Tasks))
	for _, task := range run.Swarm.Tasks {
		deps[task.ID] = task.DependsOn
	}
	seen := map[string]bool{}
	var affected func(string) bool
	affected = func(id string) bool {
		if selected[id] {
			return true
		}
		if seen[id] {
			return false
		}
		seen[id] = true
		for _, dep := range deps[id] {
			if affected(dep) {
				return true
			}
		}
		return false
	}
	return affected(taskID)
}

func (s *Server) settleOwnedQueueStop(ctx context.Context, item swarmQueueRecord, runtime SwarmSchedulerRuntime) (bool, error) {
	if runtime.StopOwned == nil || item.ReasonCode == "context_superseded_identity_pending" {
		return false, nil
	}
	actor := item.Actor
	if item.StopActor.ProjectID == item.ProjectID {
		actor = item.StopActor
	}
	settled, err := runtime.StopOwned(ctx, SwarmSupersededRequest{QueueID: item.ID, ProjectID: item.ProjectID, RunID: item.RunID, TaskID: item.TaskID, OperationID: item.OperationID, ChildID: item.ChildID, Actor: actor})
	if err != nil || !settled {
		return false, err
	}
	if err = s.releaseQueueLease(ctx, item.ID, runtime); err != nil {
		return false, err
	}
	code, message := "operator_stopped", "The exact owned attempt was stopped and its reservation released. Retained output remains available for review."
	if item.ReasonCode != "operator_stop_pending" {
		code, message = "context_superseded", "The exact old-version attempt was stopped and its reservation released. Retained output remains available for review."
	}
	if err = s.schedulerSetReason(item.ID, "cancelled", code, message); err != nil {
		return false, err
	}
	return true, nil
}

// An accepted amendment first records a durable stop intent against each
// affected, old-version active attempt. The scheduler will neither resume nor
// replace that exact child. A new run waits for backend absence and release of
// its charged reservation; failure or a lost stop ACK leaves the intent intact
// for the next pass after restart. The scheduler tick serializes this edge with
// dispatch, while other projects keep their normal scheduling turns.
func (s *Server) stopSupersededActiveTasks(ctx context.Context, projectID string, current ImplementationContextRequest) (bool, error) {
	s.swarmScheduler.tick.Lock()
	defer s.swarmScheduler.tick.Unlock()
	selected := map[string]bool{}
	for _, id := range current.TaskIDs {
		selected[id] = true
	}
	p, err := s.resolveProject(projectID)
	if err != nil {
		return false, err
	}
	stopActor, stopActorErr := management.CaptureExecutionControlActor(ctx, projectID)
	s.swarmScheduler.mu.Lock()
	row, err := s.readSwarmScheduler()
	runtime := s.swarmScheduler.runtime
	if err != nil {
		s.swarmScheduler.mu.Unlock()
		return false, err
	}
	pending := []swarmQueueRecord{}
	heldTerminals := []swarmQueueRecord{}
	changed := false
	allIdentified := true
	for i := range row.Items {
		item := &row.Items[i]
		if item.ProjectID != projectID ||
			(!swarmQueueActive(item.State) && !swarmSupersededStopReason(item.ReasonCode) && !(swarmQueueTerminal(item.State) && (item.MemoryMiB > 0 || item.CPUMillis > 0))) {
			continue
		}
		run, readErr := s.projectRuns.Get(projectID, p.Path, item.RunID)
		if readErr != nil {
			s.swarmScheduler.mu.Unlock()
			return false, readErr
		}
		if !oldRunTaskAffected(run, item.TaskID, selected) {
			continue
		}
		prior := ImplementationContextSelection(&run.Input.Implementation)
		if prior == nil {
			s.swarmScheduler.mu.Unlock()
			return false, ErrProjectRunInvalid
		}
		if prior.BriefRevision == current.BriefRevision && prior.PlanRevision == current.PlanRevision &&
			prior.SpecificationRevision == current.SpecificationRevision && prior.DesignPlanRevision == current.DesignPlanRevision {
			continue
		}
		if swarmQueueTerminal(item.State) {
			heldTerminals = append(heldTerminals, *item)
			continue // retain outcome; release still needs exact lease proof
		}
		code := "context_superseded_stop_pending"
		itemChanged := false
		if item.OperationID == "" || item.ChildID == "" {
			_, op, found, lookupErr := s.projectRuns.FindOperationByKey(p.ID, p.Path, item.RunID, schedulerReserveKey(*item), "session")
			if lookupErr != nil {
				s.swarmScheduler.mu.Unlock()
				return false, lookupErr
			}
			if found {
				item.OperationID, item.ChildID = op.ID, op.ChildID
				itemChanged = true
			} else if item.State == "claiming" || item.ReasonCode == "context_superseded_unallocated_claim" {
				// The serialized claim never created an operation. Release its
				// private admission lease before admitting revised work.
				code = "context_superseded_unallocated_claim"
			} else {
				code = "context_superseded_identity_pending"
			}
		}
		if item.ReasonCode != code || item.State != "unknown" || itemChanged {
			// Unknown still counts against scheduler capacity and cannot be
			// resumed. Never mark cancelled or erase charge before exact proof.
			schedulerReason(item, "unknown", code, "Accepted context changed this task. Its exact launch and capacity remain held until owned stop or absence is confirmed; revised work waits.")
			item.Revision++
			item.UpdatedAt = s.schedulerNow().Format(time.RFC3339Nano)
			changed = true
		}
		if stopActorErr == nil && item.StopActor != stopActor {
			item.StopActor = stopActor
			item.Revision++
			item.UpdatedAt = s.schedulerNow().Format(time.RFC3339Nano)
			changed = true
		}
		if code == "context_superseded_stop_pending" || code == "context_superseded_unallocated_claim" {
			pending = append(pending, *item)
		} else {
			allIdentified = false
		}
	}
	if changed {
		err = s.saveSwarmScheduler(&row)
	}
	s.swarmScheduler.mu.Unlock()
	if err != nil {
		return false, err
	}
	if len(pending) == 0 && len(heldTerminals) == 0 {
		return allIdentified, nil
	}
	allSettled := allIdentified
	for _, item := range heldTerminals {
		if runtime.Release == nil || s.releaseQueueLease(ctx, item.ID, runtime) != nil {
			allSettled = false
		}
	}
	for _, item := range pending {
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		settled, stopErr := s.settleOwnedQueueStop(ctx, item, runtime)
		if stopErr != nil || !settled {
			allSettled = false
			continue
		}
	}
	return allSettled, nil
}
func validateSwarmSchedulerPolicy(p SwarmSchedulerPolicy) bool {
	return p.MaxActive >= 1 && p.MaxActive <= MaxSwarmScheduledTasks && p.MaxPerProject >= 1 && p.MaxPerProject <= MaxSwarmParallelTasks && p.MaxPerProject <= p.MaxActive && p.HeadroomPercent >= 10 && p.HeadroomPercent <= 75 && p.MinHeadroomMiB >= 512 && p.MinHeadroomMiB <= 65536 && (p.MaxHeadroomMiB == 0 || p.MaxHeadroomMiB >= p.MinHeadroomMiB && p.MaxHeadroomMiB <= 65536) && p.DiskReserveMiB >= 512 && p.DiskReserveMiB <= 1048576
}
func swarmQueuePriority(priority string) bool {
	return priority == "low" || priority == "normal" || priority == "high"
}
func swarmQueueState(state string) bool {
	switch state {
	case "queued", "paused", "claiming", "reserved", "dispatching", "running", "completed", "failed", "cancelled", "blocked", "unknown":
		return true
	}
	return false
}
func swarmQueueActive(state string) bool {
	switch state {
	case "claiming", "reserved", "dispatching", "running", "unknown":
		return true
	}
	return false
}
func swarmQueueTerminal(state string) bool {
	return state == "completed" || state == "failed" || state == "cancelled"
}
func (s *Server) schedulerNow() time.Time {
	if s.swarmScheduler.now != nil {
		return s.swarmScheduler.now().UTC()
	}
	return time.Now().UTC()
}
func (s *Server) readSwarmScheduler() (swarmSchedulerLedger, error) {
	defaultLedger := swarmSchedulerLedger{Version: 1, AccountScope: s.projectRuns.accountScope, Revision: 1, PolicyRevision: 1, Policy: defaultSwarmSchedulerPolicy(), LastServed: map[string]uint64{}, Items: []swarmQueueRecord{}}
	path, err := guardedProjectPath(s.stateDir, "swarm-scheduler.json", false)
	if errors.Is(err, os.ErrNotExist) {
		return s.applySharedSwarmPolicy(defaultLedger)
	}
	if err != nil {
		return swarmSchedulerLedger{}, err
	}
	if hostpath.OneName(path) != nil {
		return swarmSchedulerLedger{}, ErrProjectRunInvalid
	}
	file, err := os.Open(path)
	if err != nil {
		return swarmSchedulerLedger{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > swarmQueueFileLimit {
		return swarmSchedulerLedger{}, ErrProjectRunInvalid
	}
	data, err := io.ReadAll(io.LimitReader(file, swarmQueueFileLimit+1))
	if err != nil {
		return swarmSchedulerLedger{}, err
	}
	var row swarmSchedulerLedger
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if len(data) > swarmQueueFileLimit || dec.Decode(&row) != nil || dec.Decode(new(any)) != io.EOF || s.validateSwarmScheduler(row) != nil {
		return swarmSchedulerLedger{}, ErrProjectRunInvalid
	}
	return s.applySharedSwarmPolicy(row)
}
func (s *Server) validateSwarmScheduler(row swarmSchedulerLedger) error {
	if row.Version != 1 || row.AccountScope != s.projectRuns.accountScope || row.Revision < 1 || row.PolicyRevision < 1 || !validateSwarmSchedulerPolicy(row.Policy) || len(row.Items) > swarmQueueLimit || row.LastServed == nil || len(row.LastServed) > swarmQueueLimit {
		return ErrProjectRunInvalid
	}
	ids := map[string]bool{}
	keys := map[string]bool{}
	activeTasks := map[string]bool{}
	for projectID, sequence := range row.LastServed {
		if !prototypeReleaseID.MatchString(projectID) || sequence > row.Sequence {
			return ErrProjectRunInvalid
		}
	}
	for _, item := range row.Items {
		if !prototypeReleaseID.MatchString(item.ID) || !prototypeReleaseID.MatchString(item.ProjectID) || !validRunID(item.RunID) || !designIDPattern.MatchString(item.TaskID) || item.TaskTitle == "" || len(item.TaskTitle) > 2000 || item.Revision < 1 || !swarmQueuePriority(item.Priority) || !swarmQueueState(item.State) || !hexDigest(item.InputDigest) || !hexDigest(item.KeyDigest) || !hexDigest(item.BatchKeyDigest) || !hexDigest(item.RequestDigest) || ids[item.ID] || keys[item.ProjectID+item.KeyDigest] || item.Actor.ProjectID != item.ProjectID || item.Actor.Local && (item.Actor.UserID != "" || item.Actor.TeamID != "") || !item.Actor.Local && item.Actor.UserID == "" || len(item.Reason) > 2000 || len(item.ReasonCode) > 100 || item.MemoryMiB < 0 || item.CPUMillis < 0 {
			return ErrProjectRunInvalid
		}
		if _, err := time.Parse(time.RFC3339Nano, item.QueuedAt); err != nil {
			return ErrProjectRunInvalid
		}
		if _, err := time.Parse(time.RFC3339Nano, item.UpdatedAt); err != nil {
			return ErrProjectRunInvalid
		}
		if item.OperationID != "" && !prototypeReleaseID.MatchString(item.OperationID) || item.ChildID != "" && !prototypeReleaseID.MatchString(item.ChildID) {
			return ErrProjectRunInvalid
		}
		if item.StopActor.ProjectID != "" && (item.StopActor.ProjectID != item.ProjectID || item.StopActor.Local && (item.StopActor.UserID != "" || item.StopActor.TeamID != "") || !item.StopActor.Local && item.StopActor.UserID == "") || item.ReasonCode == "operator_stop_pending" && item.StopActor.ProjectID != item.ProjectID {
			return ErrProjectRunInvalid
		}
		ids[item.ID] = true
		keys[item.ProjectID+item.KeyDigest] = true
		key := item.ProjectID + ":" + item.RunID + ":" + item.TaskID
		if !swarmQueueTerminal(item.State) {
			if activeTasks[key] {
				return ErrProjectRunInvalid
			}
			activeTasks[key] = true
		}
	}
	return nil
}
func (s *Server) saveSwarmScheduler(row *swarmSchedulerLedger) error {
	row.Revision++
	row.ObservedAt = s.schedulerNow().Format(time.RFC3339Nano)
	if err := s.validateSwarmScheduler(*row); err != nil {
		return err
	}
	data, err := json.Marshal(row)
	if err != nil {
		return err
	}
	if len(data) > swarmQueueFileLimit {
		return ErrProjectRunLimit
	}
	path, err := guardedProjectPath(s.stateDir, "swarm-scheduler.json", true)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(path); err == nil {
		if hostpath.OneName(path) != nil {
			return ErrProjectRunInvalid
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err = durableRunJSON(path, row); err != nil {
		return err
	}
	return syncProjectRunDirectory(filepath.Dir(path))
}
func queueInputDigest(run ProjectRun) string {
	data, _ := json.Marshal(run.Input)
	return reviewDigest(string(data))
}
func queueProjectSnapshot(row swarmSchedulerLedger, projectID string) ProjectSwarmQueue {
	result := ProjectSwarmQueue{Revision: row.PolicyRevision, ObservedAt: "", DispatchPaused: row.Paused, Items: []SwarmQueueItem{}}
	for _, item := range row.Items {
		if item.ProjectID != projectID {
			continue
		}
		result.Items = append(result.Items, publicSwarmQueueItem(item.SwarmQueueItem))
		result.Revision += item.Revision
		if item.UpdatedAt > result.ObservedAt {
			result.ObservedAt = item.UpdatedAt
		}
		switch item.State {
		case "queued":
			result.Counts.Queued++
		case "paused":
			result.Counts.Paused++
		case "claiming", "reserved", "dispatching":
			result.Counts.Reserved++
		case "running":
			result.Counts.Running++
		case "blocked":
			result.Counts.Blocked++
		case "unknown":
			result.Counts.Unknown++
		case "completed":
			result.Counts.Completed++
		case "failed":
			result.Counts.Failed++
		case "cancelled":
			result.Counts.Cancelled++
		}
	}
	return result
}

// ProjectSwarmQueueSnapshot never observes a child, samples a host, or repairs a ledger.
func (s *Server) ProjectSwarmQueueSnapshot(projectID string) (ProjectSwarmQueue, error) {
	if !prototypeReleaseID.MatchString(projectID) {
		return ProjectSwarmQueue{}, ErrProjectRunInvalid
	}
	s.swarmScheduler.mu.Lock()
	defer s.swarmScheduler.mu.Unlock()
	row, err := s.readSwarmScheduler()
	if err != nil {
		return ProjectSwarmQueue{}, err
	}
	return queueProjectSnapshot(row, projectID), nil
}
func (s *Server) SwarmSchedulerPolicy() (SwarmSchedulerPolicy, error) {
	s.swarmScheduler.mu.Lock()
	defer s.swarmScheduler.mu.Unlock()
	row, err := s.readSwarmScheduler()
	return row.Policy, err
}
func (s *Server) GetProjectSwarmRun(projectID, runID string) (ProjectRun, error) {
	p, err := s.resolveProject(projectID)
	if err != nil {
		return ProjectRun{}, err
	}
	return s.projectRuns.Get(projectID, p.Path, runID)
}

// A project receives one turn before a previously served project receives another.
// Priority ages only within that fair project turn, avoiding cross-project starvation.
func orderedSwarmQueue(row swarmSchedulerLedger, now time.Time) []swarmQueueRecord {
	items := []swarmQueueRecord{}
	for _, item := range row.Items {
		if item.State == "queued" || item.State == "claiming" || item.State == "reserved" {
			items = append(items, item)
		}
	}
	score := func(item swarmQueueRecord) int {
		base := map[string]int{"low": 0, "normal": 1, "high": 2}[item.Priority]
		at, _ := time.Parse(time.RFC3339Nano, item.QueuedAt)
		return base + int(now.Sub(at)/(5*time.Minute))
	}
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if row.LastServed[a.ProjectID] != row.LastServed[b.ProjectID] {
			return row.LastServed[a.ProjectID] < row.LastServed[b.ProjectID]
		}
		if a.ProjectID != b.ProjectID {
			return a.QueuedAt < b.QueuedAt
		}
		if score(a) != score(b) {
			return score(a) > score(b)
		}
		return a.QueuedAt < b.QueuedAt
	})
	return items
}

// SwarmQueueIDForOperation prevents manual session creation from borrowing a
// queue's reserved checkout, actor consent, or resource lease. It also covers
// the interval after the run reservation is persisted but before its queue
// receipt is updated.
func (s *Server) SwarmQueueIDForOperation(projectID, runID, operationID string) (string, error) {
	s.swarmScheduler.mu.Lock()
	row, err := s.readSwarmScheduler()
	s.swarmScheduler.mu.Unlock()
	if err != nil {
		return "", err
	}
	p, err := s.resolveProject(projectID)
	if err != nil {
		return "", err
	}
	for _, item := range row.Items {
		if item.ProjectID != projectID || item.RunID != runID {
			continue
		}
		if item.OperationID == operationID {
			return item.ID, nil
		}
		if item.OperationID == "" && swarmQueueActive(item.State) {
			_, op, found, readErr := s.projectRuns.FindOperationByKey(p.ID, p.Path, runID, schedulerReserveKey(item), "session")
			if readErr != nil {
				return "", readErr
			}
			if found && op.ID == operationID {
				return item.ID, nil
			}
		}
	}
	return "", nil
}

// CloudSwarmQueueOwner resolves an exact retained task's original actor. The
// execution service must reauthorize this identity against current project,
// team, connection and cost policy before each provider effect. A client or a
// cloud artifact never supplies the actor for a resumed allocation.
func (s *Server) CloudSwarmQueueOwner(scope ProjectSwarmMicroVMImportScope) (management.ExecutionActorRef, error) {
	return s.cloudSwarmQueueOwner(scope, false)
}

// CloudSwarmCleanupOwner is only for exact owned VM retirement. It does not
// grant source import, task launch or provider inference after revocation.
func (s *Server) CloudSwarmCleanupOwner(scope ProjectSwarmMicroVMImportScope) (management.ExecutionActorRef, error) {
	return s.cloudSwarmQueueOwner(scope, true)
}

func (s *Server) cloudSwarmQueueOwner(scope ProjectSwarmMicroVMImportScope, cleanup bool) (management.ExecutionActorRef, error) {
	s.swarmScheduler.mu.Lock()
	row, err := s.readSwarmScheduler()
	s.swarmScheduler.mu.Unlock()
	if err != nil {
		return management.ExecutionActorRef{}, err
	}
	p, err := s.resolveProject(scope.ProjectID)
	if err != nil {
		return management.ExecutionActorRef{}, err
	}
	run, err := s.projectRuns.Get(p.ID, p.Path, scope.RunID)
	if err != nil || run.Input.Route.ExecutionLocation != "cloud-microvm" {
		return management.ExecutionActorRef{}, ErrProjectRunConflict
	}
	op := swarmOperation(&run, scope.OperationID)
	if op == nil || op.ChildID != scope.ChildID || op.TaskID == "" || run.State == "cancelled" && !cleanup {
		return management.ExecutionActorRef{}, ErrProjectRunConflict
	}
	for _, item := range row.Items {
		if item.ProjectID == scope.ProjectID && item.RunID == scope.RunID && item.OperationID == scope.OperationID &&
			item.ChildID == scope.ChildID && item.TaskID == op.TaskID && item.Actor.ProjectID == scope.ProjectID &&
			(item.State == "dispatching" || item.State == "running" || item.State == "unknown" || item.State == "completed" || item.State == "failed" || cleanup && item.State == "cancelled") {
			return item.Actor, nil
		}
	}
	return management.ExecutionActorRef{}, ErrProjectRunConflict
}

func publicSwarmQueueItem(item SwarmQueueItem) SwarmQueueItem {
	if item.State == "claiming" || item.State == "dispatching" {
		item.State = "reserved"
	}
	return item
}

// A run-level cancellation also cancels future queue entries immediately,
// including while global dispatch is paused. Active attempts retain their
// independent stop/reconciliation controls.
func (s *Server) cancelQueuedSwarmRun(projectID, runID string) error {
	s.swarmScheduler.mu.Lock()
	defer s.swarmScheduler.mu.Unlock()
	row, err := s.readSwarmScheduler()
	if err != nil {
		return err
	}
	changed := false
	for i := range row.Items {
		item := &row.Items[i]
		if item.ProjectID == projectID && item.RunID == runID && !swarmQueueTerminal(item.State) && !swarmQueueActive(item.State) {
			schedulerReason(item, "cancelled", "swarm_cancelled", "This swarm was cancelled. No future queued work will start.")
			item.Revision++
			item.UpdatedAt = s.schedulerNow().Format(time.RFC3339Nano)
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return s.saveSwarmScheduler(&row)
}

// An accepted context amendment invalidates only waiting copies of the tasks
// it actually changes. Already active/uncertain attempts retain their exact
// child and lease for normal observation; unrelated tasks keep progressing.
// Repeated coordinator ticks are idempotent, including after a restart.
func (s *Server) blockSupersededQueuedTasks(projectID string, current ImplementationContextRequest) error {
	selected := map[string]bool{}
	for _, taskID := range current.TaskIDs {
		selected[taskID] = true
	}
	s.swarmScheduler.mu.Lock()
	defer s.swarmScheduler.mu.Unlock()
	row, err := s.readSwarmScheduler()
	if err != nil {
		return err
	}
	p, err := s.resolveProject(projectID)
	if err != nil {
		return err
	}
	changed := false
	for i := range row.Items {
		item := &row.Items[i]
		if item.ProjectID != projectID ||
			(item.State != "queued" && item.State != "paused" && item.State != "blocked") ||
			item.ReasonCode == "context_superseded" {
			continue
		}
		run, readErr := s.projectRuns.Get(projectID, p.Path, item.RunID)
		if readErr != nil {
			return readErr // Never guess which task input a retained queue owns.
		}
		if !oldRunTaskAffected(run, item.TaskID, selected) {
			continue
		}
		prior := ImplementationContextSelection(&run.Input.Implementation)
		if prior == nil || prior.BriefRevision == current.BriefRevision && prior.PlanRevision == current.PlanRevision &&
			prior.SpecificationRevision == current.SpecificationRevision && prior.DesignPlanRevision == current.DesignPlanRevision {
			continue
		}
		schedulerReason(item, "blocked", "context_superseded", "An accepted context amendment changed this task. Its waiting old-version work will not launch; review the new affected run.")
		item.Revision++
		item.UpdatedAt = s.schedulerNow().Format(time.RFC3339Nano)
		changed = true
	}
	if changed {
		return s.saveSwarmScheduler(&row)
	}
	return nil
}
