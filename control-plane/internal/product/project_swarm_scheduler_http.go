package product

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

func (s *Server) registerSwarmScheduler(mux Registrar) {
	mux.HandleFunc("GET /v1/product/swarm-scheduler", s.getSwarmScheduler)
	mux.HandleFunc("PUT /v1/product/swarm-scheduler", s.configureSwarmScheduler)
	mux.HandleFunc("GET /v1/product/projects/{id}/swarm-queue", s.getProjectSwarmQueue)
	mux.HandleFunc("POST /v1/product/projects/{id}/runs/{runId}/swarm/tasks/{taskId}/queue", s.enqueueProjectSwarmTask)
	mux.HandleFunc("POST /v1/product/projects/{id}/runs/{runId}/swarm/queue", s.enqueueProjectSwarmTask)
	for _, action := range []string{"pause", "resume", "cancel", "priority"} {
		mux.HandleFunc("POST /v1/product/projects/{id}/swarm-queue/{queueId}/"+action, s.controlProjectSwarmQueue)
	}
	mux.HandleFunc("POST /v1/product/projects/{id}/swarm-queue/{queueId}/stop", s.stopProjectSwarmQueue)
}

// A stop request changes one exact active queue item, not its whole run or
// another project. The durable pending receipt precedes every backend effect.
// A lost response is read back at the same queue ID; the scheduler continues
// the stop after restart without issuing any replacement launch.
func (s *Server) stopProjectSwarmQueue(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	var req struct {
		ExpectedRevision int `json:"expectedRevision"`
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	actor, err := management.CaptureExecutionControlActor(r.Context(), p.ID)
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	s.swarmScheduler.tick.Lock()
	defer s.swarmScheduler.tick.Unlock()
	s.swarmScheduler.mu.Lock()
	row, err := s.readSwarmScheduler()
	index := -1
	if err == nil {
		for i := range row.Items {
			if row.Items[i].ID == r.PathValue("queueId") && row.Items[i].ProjectID == p.ID {
				index = i
				break
			}
		}
		if index < 0 {
			err = ErrProjectRunNotFound
		}
	}
	var item swarmQueueRecord
	if err == nil {
		selected := &row.Items[index]
		if selected.Revision != req.ExpectedRevision {
			err = ErrProjectRunConflict
		} else if !swarmQueueActive(selected.State) || selected.OperationID == "" || selected.ChildID == "" ||
			selected.ReasonCode == "context_superseded_identity_pending" || selected.ReasonCode == "context_superseded_unallocated_claim" {
			err = errors.New("Select an active queue item with an exact retained operation and child. An unidentified or superseded launch requires its existing recovery path.")
		} else {
			selected.StopActor = actor
			if selected.ReasonCode != "context_superseded_stop_pending" {
				schedulerReason(selected, "unknown", "operator_stop_pending", "Stopping this exact task. The child and its capacity remain held until absence and release are confirmed; retained output remains available.")
			}
			selected.Revision++
			selected.UpdatedAt = s.schedulerNow().Format(time.RFC3339Nano)
			err = s.saveSwarmScheduler(&row)
			item = *selected
		}
	}
	runtime := s.swarmScheduler.runtime
	s.swarmScheduler.mu.Unlock()
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	settled, _ := s.settleOwnedQueueStop(r.Context(), item, runtime)
	s.swarmScheduler.mu.Lock()
	current, readErr := s.readSwarmScheduler()
	s.swarmScheduler.mu.Unlock()
	if readErr != nil {
		projectRunProblem(w, readErr)
		return
	}
	s.WakeSwarmScheduler()
	status := http.StatusAccepted
	if settled {
		status = http.StatusOK
	}
	for _, latest := range current.Items {
		if latest.ID == item.ID && latest.ProjectID == p.ID {
			journeyJSON(w, status, map[string]any{"item": publicSwarmQueueItem(latest.SwarmQueueItem), "queue": queueProjectSnapshot(current, p.ID)})
			return
		}
	}
	projectRunProblem(w, ErrProjectRunNotFound)
}
func (s *Server) getProjectSwarmQueue(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	snapshot, err := s.ProjectSwarmQueueSnapshot(p.ID)
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	journeyJSON(w, 200, snapshot)
}
func schedulerAdministrator(r *http.Request) bool {
	if _, ok := management.TeamAccessFromContext(r.Context()); ok || management.TeamRuntimeIDFromContext(r.Context()) != "" {
		return false
	}
	actor, _, ok := management.ActorFromContext(r.Context())
	return !ok || actor.Role == "admin" || actor.Role == "super-admin"
}

// CanInspectHostResources shares the scheduler's host-operator boundary with
// narrow historical resource diagnostics. Team administrators are excluded.
func CanInspectHostResources(r *http.Request) bool { return schedulerAdministrator(r) }

func (s *Server) getSwarmScheduler(w http.ResponseWriter, r *http.Request) {
	if !schedulerAdministrator(r) {
		problem(w, 403, "scheduler_policy_forbidden", "Only the workspace administrator can inspect machine-wide scheduling settings.")
		return
	}
	ctx := r.Context()
	if ctx.Err() != nil {
		return
	}
	s.swarmScheduler.mu.Lock()
	// This short shared mutex also protects mutations. Cancellation cannot
	// interrupt an existing waiter, but must prevent any read after it acquires.
	if ctx.Err() != nil {
		s.swarmScheduler.mu.Unlock()
		return
	}
	row, err := s.readSwarmScheduler()
	resources := s.swarmScheduler.runtime.Resources
	s.swarmScheduler.mu.Unlock()
	if ctx.Err() != nil {
		return
	}
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	var observed any
	if resources != nil {
		observed = resources(ctx)
	}
	if ctx.Err() != nil {
		return
	}
	journeyJSON(w, 200, map[string]any{"revision": row.PolicyRevision, "observedAt": row.ObservedAt, "dispatchPaused": row.Paused, "policy": row.Policy, "resources": observed})
}
func (s *Server) configureSwarmScheduler(w http.ResponseWriter, r *http.Request) {
	if !schedulerAdministrator(r) {
		problem(w, 403, "scheduler_policy_forbidden", "Only the workspace administrator can change machine-wide scheduling settings.")
		return
	}
	var req struct {
		ExpectedRevision int                  `json:"expectedRevision"`
		Policy           SwarmSchedulerPolicy `json:"policy"`
		DispatchPaused   bool                 `json:"dispatchPaused"`
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	if !validateSwarmSchedulerPolicy(req.Policy) {
		problem(w, 400, "invalid_scheduler_policy", fmt.Sprintf("Choose 1–%d active agents, 1–%d per project, and retain at least 10%% memory and 512 MiB memory/disk headroom. Actual starts remain subject to resource, account and provider limits.", MaxSwarmScheduledTasks, MaxSwarmParallelTasks))
		return
	}
	if s.swarmSchedulerPolicyDir != "" {
		if err := s.updateSharedSwarmPolicy(r.Context(), req.ExpectedRevision, req.Policy, req.DispatchPaused); err != nil {
			projectRunProblem(w, err)
			return
		}
		s.WakeSwarmScheduler()
		s.getSwarmScheduler(w, r)
		return
	}
	s.swarmScheduler.mu.Lock()
	row, err := s.readSwarmScheduler()
	if err == nil && row.PolicyRevision != req.ExpectedRevision {
		err = ErrProjectRunConflict
	}
	if err == nil {
		row.Policy = req.Policy
		row.Paused = req.DispatchPaused
		row.PolicyRevision++
		err = s.saveSwarmScheduler(&row)
	}
	s.swarmScheduler.mu.Unlock()
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	s.WakeSwarmScheduler()
	s.getSwarmScheduler(w, r)
}
func (s *Server) enqueueProjectSwarmTask(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	var req struct {
		ExpectedRevision int      `json:"expectedRevision"`
		IdempotencyKey   string   `json:"idempotencyKey"`
		Priority         string   `json:"priority"`
		TaskIDs          []string `json:"taskIds"`
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	if req.Priority == "" {
		req.Priority = "normal"
	}
	single := r.PathValue("taskId") != ""
	if single {
		if len(req.TaskIDs) > 0 {
			projectRunProblem(w, ErrProjectRunInvalid)
			return
		}
		req.TaskIDs = []string{r.PathValue("taskId")}
	}
	if !validRunKey(req.IdempotencyKey) || !swarmQueuePriority(req.Priority) || len(req.TaskIDs) < 1 || len(req.TaskIDs) > 200 {
		projectRunProblem(w, errors.New("Choose a valid queue key, one to 200 reviewed tasks, and low, normal or high priority."))
		return
	}
	sort.Strings(req.TaskIDs)
	for i, id := range req.TaskIDs {
		if !designIDPattern.MatchString(id) || i > 0 && req.TaskIDs[i-1] == id {
			projectRunProblem(w, ErrProjectRunInvalid)
			return
		}
	}
	s.swarmScheduler.mu.Lock()
	runtime := s.swarmScheduler.runtime
	s.swarmScheduler.mu.Unlock()
	if runtime.CaptureActor == nil || runtime.Admit == nil || runtime.Dispatch == nil || runtime.Release == nil {
		problem(w, 503, "scheduler_unavailable", "The execution scheduler is unavailable. No work was queued.")
		return
	}
	actor, err := runtime.CaptureActor(r.Context(), p.ID)
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	requestBytes, _ := json.Marshal(struct {
		RunID, Priority string
		TaskIDs         []string
		Actor           management.ExecutionActorRef
	}{r.PathValue("runId"), req.Priority, req.TaskIDs, actor})
	requestDigest := reviewDigest(string(requestBytes))
	batchKey := reviewDigest(req.IdempotencyKey)
	respond := func(status int, items []SwarmQueueItem, row swarmSchedulerLedger) {
		if single {
			journeyJSON(w, status, map[string]any{"item": items[0], "queue": queueProjectSnapshot(row, p.ID)})
		} else {
			journeyJSON(w, status, map[string]any{"items": items, "queue": queueProjectSnapshot(row, p.ID)})
		}
	}
	s.swarmScheduler.mu.Lock()
	row, err := s.readSwarmScheduler()
	prior := []SwarmQueueItem{}
	if err == nil {
		for _, item := range row.Items {
			if item.ProjectID == p.ID && item.BatchKeyDigest == batchKey {
				if item.RequestDigest != requestDigest {
					err = ErrProjectRunKeyConflict
					break
				}
				prior = append(prior, publicSwarmQueueItem(item.SwarmQueueItem))
			}
		}
	}
	s.swarmScheduler.mu.Unlock()
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	if len(prior) > 0 {
		if len(prior) != len(req.TaskIDs) {
			projectRunProblem(w, ErrProjectRunInvalid)
			return
		}
		respond(200, prior, row)
		return
	}
	run, err := s.projectRuns.Get(p.ID, p.Path, r.PathValue("runId"))
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	if run.Revision != req.ExpectedRevision || run.Swarm == nil || run.Swarm.State == "cancelled" || run.State == "complete" {
		projectRunProblem(w, ErrProjectRunConflict)
		return
	}
	if err = validateSwarmTaskRoute(run); err != nil {
		projectRunProblem(w, err)
		return
	}
	for _, id := range req.TaskIDs {
		task := swarmTask(&run, id)
		if task == nil || task.Integration != nil || task.OperationID != "" && (swarmOperation(&run, task.OperationID) == nil || swarmOperation(&run, task.OperationID).Status != "cancelled") {
			projectRunProblem(w, ErrProjectRunConflict)
			return
		}
		if task.OperationID != "" && swarmRetirementPending(*swarmOperation(&run, task.OperationID)) {
			projectRunProblem(w, errors.New("Finish this task's missing-launch recovery before authorizing a fresh attempt. Its execution reservation has not been confirmed released."))
			return
		}
	}
	if _, err = s.resolveProjectRunSelection(r.Context(), run); err != nil {
		projectRunProblem(w, err)
		return
	}
	now := s.schedulerNow().Format(time.RFC3339Nano)
	records := []swarmQueueRecord{}
	items := []SwarmQueueItem{}
	for _, id := range req.TaskIDs {
		task := swarmTask(&run, id)
		item := swarmQueueRecord{SwarmQueueItem: SwarmQueueItem{Revision: 1, ID: newID("queue-"), ProjectID: p.ID, RunID: run.ID, TaskID: task.ID, TaskTitle: task.Title, Priority: req.Priority, State: "queued", QueuedAt: now, UpdatedAt: now, ReasonCode: "ready", Reason: "Queued with the reviewed task and saved model connection. Capacity and dependencies are checked before starting."}, Actor: actor, InputDigest: queueInputDigest(run), KeyDigest: reviewDigest(req.IdempotencyKey + "/" + id), BatchKeyDigest: batchKey, RequestDigest: requestDigest}
		records = append(records, item)
		items = append(items, item.SwarmQueueItem)
	}
	// Keep the reviewed revision stable while this entire consent is appended.
	s.projectRunDispatchMu.Lock()
	current, err := s.projectRuns.Get(p.ID, p.Path, run.ID)
	if err == nil && current.Revision != req.ExpectedRevision {
		err = ErrProjectRunConflict
	}
	if err == nil {
		for _, taskID := range req.TaskIDs {
			if err = s.requireNoSwarmPendingLaunch(r.Context(), current, taskID); err != nil {
				break
			}
		}
	}
	s.swarmScheduler.mu.Lock()
	if err == nil {
		row, err = s.readSwarmScheduler()
	}
	if err == nil {
		for _, old := range row.Items {
			if old.ProjectID != p.ID {
				continue
			}
			if old.BatchKeyDigest == batchKey {
				err = ErrProjectRunConflict
				break
			}
			for _, item := range records {
				if !swarmQueueTerminal(old.State) && old.RunID == run.ID && old.TaskID == item.TaskID {
					err = ErrProjectRunConflict
					break
				}
			}
		}
	}
	if err == nil && len(row.Items)+len(records) > swarmQueueLimit {
		err = ErrProjectRunLimit
	}
	if err == nil {
		row.Items = append(row.Items, records...)
		err = s.saveSwarmScheduler(&row)
	}
	s.swarmScheduler.mu.Unlock()
	s.projectRunDispatchMu.Unlock()
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	respond(201, items, row)
	s.WakeSwarmScheduler()
}
func (s *Server) controlProjectSwarmQueue(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	var req struct {
		ExpectedRevision int    `json:"expectedRevision"`
		Priority         string `json:"priority"`
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	action := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
	s.swarmScheduler.mu.Lock()
	row, err := s.readSwarmScheduler()
	index := -1
	if err == nil {
		for i := range row.Items {
			if row.Items[i].ID == r.PathValue("queueId") && row.Items[i].ProjectID == p.ID {
				index = i
				break
			}
		}
		if index < 0 {
			err = ErrProjectRunNotFound
		}
	}
	if err == nil {
		item := &row.Items[index]
		if item.Revision != req.ExpectedRevision {
			err = ErrProjectRunConflict
		} else if swarmQueueActive(item.State) {
			err = errors.New("This task already holds an execution reservation. Pause its swarm to prevent new launches, or use the active session's stop control; stopping a queue entry cannot prove its process has stopped.")
		} else if swarmQueueTerminal(item.State) {
			err = ErrProjectRunConflict
		} else {
			switch action {
			case "pause":
				item.State = "paused"
				item.ReasonCode = "operator_paused"
				item.Reason = "Queued execution is paused. No running process was stopped."
			case "resume":
				if item.State != "paused" {
					err = ErrProjectRunConflict
				} else {
					// This route uses private receipts only, so it remains usable
					// while source admission is held. Never upgrade legacy task
					// permission settings when resuming its saved authorization.
					var run ProjectRun
					run, err = s.projectRuns.Get(p.ID, p.Path, item.RunID)
					if err == nil {
						err = validateSwarmTaskRoute(run)
					}
					if err == nil {
						item.State = "queued"
						item.ReasonCode = "ready"
						item.Reason = "Queued execution is enabled."
					}
				}
			case "cancel":
				item.State = "cancelled"
				item.ReasonCode = "operator_cancelled"
				item.Reason = "Future execution was cancelled. Retained attempts and results remain available."
			case "priority":
				if !swarmQueuePriority(req.Priority) {
					err = ErrProjectRunInvalid
				} else {
					item.Priority = req.Priority
				}
			}
			if err == nil {
				item.Revision++
				item.UpdatedAt = s.schedulerNow().Format(time.RFC3339Nano)
				err = s.saveSwarmScheduler(&row)
			}
		}
	}
	s.swarmScheduler.mu.Unlock()
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	s.WakeSwarmScheduler()
	journeyJSON(w, 200, map[string]any{"item": publicSwarmQueueItem(row.Items[index].SwarmQueueItem), "queue": queueProjectSnapshot(row, p.ID)})
}
