package product

import (
	"bytes"
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
	return SwarmSchedulerPolicy{MaxActive: 4, MaxPerProject: 2, HeadroomPercent: 25, MinHeadroomMiB: 2048, DiskReserveMiB: 2048}
}
func validateSwarmSchedulerPolicy(p SwarmSchedulerPolicy) bool {
	return p.MaxActive >= 1 && p.MaxActive <= MaxSwarmScheduledTasks && p.MaxPerProject >= 1 && p.MaxPerProject <= MaxSwarmParallelTasks && p.MaxPerProject <= p.MaxActive && p.HeadroomPercent >= 10 && p.HeadroomPercent <= 75 && p.MinHeadroomMiB >= 512 && p.MinHeadroomMiB <= 65536 && p.DiskReserveMiB >= 512 && p.DiskReserveMiB <= 1048576
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
