package product

import (
	"context"
	"errors"
	"strings"
	"time"
)

// SetSwarmSchedulerRuntime connects the actual execution service. Reads never
// run the scheduler; a wake or timed turn performs the explicitly queued work.
func (s *Server) SetSwarmSchedulerRuntime(runtime SwarmSchedulerRuntime) {
	s.swarmScheduler.mu.Lock()
	if s.swarmScheduler.cancel != nil {
		s.swarmScheduler.cancel()
	}
	ctx, cancel := context.WithCancel(s.workspaceContext)
	s.swarmScheduler.cancel = cancel
	s.swarmScheduler.runtime = runtime
	if s.swarmScheduler.wake == nil {
		s.swarmScheduler.wake = make(chan struct{}, 1)
	}
	wake := s.swarmScheduler.wake
	s.swarmScheduler.mu.Unlock()
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-wake:
			case <-ticker.C:
			}
			_ = s.RunSwarmSchedulerOnce(ctx)
		}
	}()
	s.WakeSwarmScheduler()
}
func (s *Server) WakeSwarmScheduler() {
	s.swarmScheduler.mu.Lock()
	wake := s.swarmScheduler.wake
	s.swarmScheduler.cooldowns = nil
	s.swarmScheduler.mu.Unlock()
	if wake != nil {
		select {
		case wake <- struct{}{}:
		default:
		}
	}
}
func (s *Server) schedulerUpdate(id string, change func(*swarmQueueRecord) error) error {
	s.swarmScheduler.mu.Lock()
	defer s.swarmScheduler.mu.Unlock()
	row, err := s.readSwarmScheduler()
	if err != nil {
		return err
	}
	for i := range row.Items {
		if row.Items[i].ID == id {
			before := row.Items[i]
			if err = change(&row.Items[i]); err != nil {
				return err
			}
			// A delayed observation cannot undo an operator's pause/cancel or
			// reopen a terminal receipt. Explicit controls mutate the ledger
			// through their own current-revision admission.
			if swarmQueueTerminal(before.State) && row.Items[i].State != before.State || before.State == "paused" && row.Items[i].State != "paused" && row.Items[i].State != "cancelled" {
				return nil
			}
			if row.Items[i] == before {
				return nil
			}
			row.Items[i].Revision++
			row.Items[i].UpdatedAt = s.schedulerNow().Format(time.RFC3339Nano)
			return s.saveSwarmScheduler(&row)
		}
	}
	return ErrProjectRunNotFound
}
func schedulerReason(item *swarmQueueRecord, state, code, message string) {
	item.State = state
	item.ReasonCode = code
	item.Reason = message
}
func (s *Server) schedulerSetReason(id, state, code, message string) error {
	return s.schedulerUpdate(id, func(item *swarmQueueRecord) error { schedulerReason(item, state, code, message); return nil })
}
func schedulerReserveKey(item swarmQueueRecord) string { return "scheduler-" + item.ID }

// RunSwarmSchedulerOnce is a bounded, deterministic scheduling turn. All
// process/clone side effects follow a durable claim and the shared admission
// lease. It is also the integration-test seam; no timer sleeps are required.
func (s *Server) RunSwarmSchedulerOnce(ctx context.Context) error {
	s.swarmScheduler.tick.Lock()
	defer s.swarmScheduler.tick.Unlock()
	s.swarmScheduler.mu.Lock()
	runtime := s.swarmScheduler.runtime
	row, err := s.readSwarmScheduler()
	if err == nil && s.swarmScheduler.policyRevision != row.PolicyRevision {
		s.swarmScheduler.cooldowns = nil
		s.swarmScheduler.policyRevision = row.PolicyRevision
	}
	s.swarmScheduler.mu.Unlock()
	// Completion observation survives empty/paused queues and unavailable new
	// execution authority, including a queue read failure. Exact report capture
	// still requires intact ownership records; observation must not infer a
	// missing queue binding from corrupt state. Failures leave durable rows
	// pending, while unrelated scheduling retains its own admission checks.
	if runtime.ObserveTasks != nil {
		observationCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		_ = runtime.ObserveTasks(observationCtx)
		cancel()
	}
	if err != nil {
		return err
	}
	if runtime.Admit == nil || runtime.Dispatch == nil || runtime.Release == nil {
		return nil
	}
	// Recover before capacity selection. An uncertain child is never replaced.
	for _, item := range row.Items {
		if swarmQueueTerminal(item.State) && (item.MemoryMiB > 0 || item.CPUMillis > 0) {
			if err = s.releaseQueueLease(ctx, item.ID, runtime); err != nil {
				return err
			}
		}
		if swarmQueueActive(item.State) {
			if err = s.reconcileQueuedSwarm(ctx, item, runtime); err != nil {
				return err
			}
		}
	}
	visited := map[string]bool{}
	attemptedProjects := map[string]bool{}
	admissions := 0
	for turns := 0; turns < swarmQueueLimit && admissions < 4; turns++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		s.swarmScheduler.mu.Lock()
		row, err = s.readSwarmScheduler()
		s.swarmScheduler.mu.Unlock()
		if err != nil {
			return err
		}
		if row.Paused {
			return nil
		}
		var selected *swarmQueueRecord
		active := 0
		perProject := map[string]int{}
		for _, item := range row.Items {
			if swarmQueueActive(item.State) {
				active++
				perProject[item.ProjectID]++
			}
		}
		runCache := map[string]ProjectRun{}
		runErrors := map[string]error{}
		for _, item := range orderedSwarmQueue(row, s.schedulerNow()) {
			if attemptedProjects[item.ProjectID] || s.swarmQueueCoolingDown(item.ID) {
				continue
			}
			if visited[item.ID] {
				continue
			}
			visited[item.ID] = true
			cacheKey := item.ProjectID + ":" + item.RunID
			run, cached := runCache[cacheKey]
			readErr := runErrors[cacheKey]
			if !cached {
				run, readErr = s.GetProjectSwarmRun(item.ProjectID, item.RunID)
				runCache[cacheKey] = run
				runErrors[cacheKey] = readErr
			}
			if readErr != nil || queueInputDigest(run) != item.InputDigest || run.Swarm == nil {
				if err = s.schedulerSetReason(item.ID, "blocked", "review_changed", "The saved reviewed run is unavailable or changed. Review it before authorizing another queue entry."); err != nil {
					return err
				}
				continue
			}
			task := swarmTask(&run, item.TaskID)
			if task == nil {
				if err = s.schedulerSetReason(item.ID, "blocked", "task_missing", "The reviewed task is unavailable."); err != nil {
					return err
				}
				continue
			}
			if run.Swarm.State == "cancelled" || run.State == "cancelled" {
				if err = s.schedulerSetReason(item.ID, "cancelled", "swarm_cancelled", "This swarm was cancelled; future queued work will not start."); err != nil {
					return err
				}
				if releaseErr := s.releaseQueueLease(ctx, item.ID, runtime); releaseErr != nil {
					return releaseErr
				}
				continue
			}
			if run.Swarm.State != "ready" {
				if err = s.schedulerSetReason(item.ID, item.State, "swarm_paused", "The swarm is paused. Resume it to allow queued work to start."); err != nil {
					return err
				}
				continue
			}
			if item.State == "queued" && (active >= row.Policy.MaxActive || perProject[item.ProjectID] >= row.Policy.MaxPerProject) {
				if err = s.schedulerSetReason(item.ID, "queued", "capacity", "Waiting for the shared or project agent limit. Existing work continues."); err != nil {
					return err
				}
				continue
			}
			if item.OperationID == "" {
				if readyErr := swarmTaskReady(run, *task); readyErr != nil {
					if err = s.schedulerSetReason(item.ID, item.State, "dependencies_or_scope", readyErr.Error()); err != nil {
						return err
					}
					continue
				}
			}
			if gateErr := s.OrchestratorExecutionGate(item.ProjectID); gateErr != nil {
				if err = s.schedulerSetReason(item.ID, item.State, "clarification_required", gateErr.Error()); err != nil {
					return err
				}
				continue
			}
			p, sourceErr := s.resolveProject(item.ProjectID)
			if sourceErr == nil {
				sourceErr = s.ValidateImplementationContext(p.ID, run.WorktreeID, p.Path, &run.Input.Implementation)
			}
			if sourceErr == nil {
				sourceErr = s.validateProjectRunApprovals(item.ProjectID, run.Input.Approvals)
			}
			if sourceErr != nil {
				if releaseErr := s.releaseQueueLease(ctx, item.ID, runtime); releaseErr != nil {
					return releaseErr
				}
				if err = s.schedulerSetReason(item.ID, "blocked", "review_changed", "The task's reviewed requirements, design, plan or approvals changed. Review the new inputs and explicitly queue a fresh run."); err != nil {
					return err
				}
				continue
			}
			selected = &item
			break
		}
		if selected == nil {
			return nil
		}
		item := *selected
		// The compare-and-set prevents a simultaneous cancel or priority update
		// from authorizing stale work. No global lock is held across admission/Git.
		if item.State == "queued" {
			err = s.schedulerUpdate(item.ID, func(current *swarmQueueRecord) error {
				if current.Revision != item.Revision || current.State != "queued" {
					return ErrProjectRunConflict
				}
				schedulerReason(current, "claiming", "admission", "Checking current permissions and shared resources before allocating a checkout.")
				return nil
			})
			if errors.Is(err, ErrProjectRunConflict) {
				continue
			}
			if err != nil {
				return err
			}
		}
		admissions++
		attemptedProjects[item.ProjectID] = true
		admission, admitErr := runtime.Admit(ctx, SwarmAdmissionRequest{QueueID: item.ID, ProjectID: item.ProjectID, RunID: item.RunID, Actor: item.Actor, Policy: row.Policy})
		if admitErr != nil {
			if releaseErr := s.releaseQueueLease(ctx, item.ID, runtime); releaseErr != nil {
				return releaseErr
			}
			if err = s.schedulerSetReason(item.ID, "blocked", "admission_failed", "Current execution authority or resource admission could not be established. Review this queue entry before retrying."); err != nil {
				return err
			}
			continue
		}
		if admission.State != "admitted" {
			state := "queued"
			if admission.State == "blocked" {
				state = "blocked"
			}
			if err = s.schedulerSetReason(item.ID, state, admission.ReasonCode, admission.Message); err != nil {
				return err
			}
			if state == "queued" {
				s.deferSwarmQueueAttempt(item.ID)
			}
			continue
		}
		if err = s.schedulerUpdate(item.ID, func(current *swarmQueueRecord) error {
			current.MemoryMiB = admission.MemoryMiB
			current.CPUMillis = admission.CPUMillis
			return nil
		}); err != nil {
			return err
		}
		p, resolveErr := s.resolveProject(item.ProjectID)
		if resolveErr != nil {
			if releaseErr := s.releaseQueueLease(ctx, item.ID, runtime); releaseErr != nil {
				return releaseErr
			}
			if err = s.schedulerSetReason(item.ID, "blocked", "project_unavailable", "The queued project is unavailable. No agent was started."); err != nil {
				return err
			}
			continue
		}
		run, readErr := s.projectRuns.Get(p.ID, p.Path, item.RunID)
		if readErr != nil {
			if releaseErr := s.releaseQueueLease(ctx, item.ID, runtime); releaseErr != nil {
				return releaseErr
			}
			return readErr
		}
		// A pause received after admission still prevents allocation/dispatch.
		s.swarmScheduler.mu.Lock()
		latest, readErr := s.readSwarmScheduler()
		s.swarmScheduler.mu.Unlock()
		if readErr != nil {
			return readErr
		}
		if latest.Paused || run.Swarm == nil || run.Swarm.State != "ready" {
			if releaseErr := s.releaseQueueLease(ctx, item.ID, runtime); releaseErr != nil {
				return releaseErr
			}
			if err = s.schedulerSetReason(item.ID, "queued", "swarm_paused", "Dispatch was paused before allocation. No new agent was started."); err != nil {
				return err
			}
			continue
		}
		run, op, reserveErr := s.reserveSwarmTask(ctx, p, item.RunID, item.TaskID, schedulerReserveKey(item), run.Revision)
		if reserveErr != nil {
			// A reservation may have been persisted before an I/O error. The next
			// turn must inspect its receipt, never repeat allocation optimistically.
			_, known, found, lookupErr := s.projectRuns.FindOperationByKey(p.ID, p.Path, item.RunID, schedulerReserveKey(item), "session")
			if lookupErr != nil {
				return lookupErr
			}
			if found {
				if err = s.schedulerUpdate(item.ID, func(current *swarmQueueRecord) error {
					current.OperationID = known.ID
					current.ChildID = known.ChildID
					schedulerReason(current, "unknown", "allocation_unknown", "Allocation was interrupted. Inspect the retained attempt; it will not be replayed.")
					return nil
				}); err != nil {
					return err
				}
			} else {
				if releaseErr := s.releaseQueueLease(ctx, item.ID, runtime); releaseErr != nil {
					return releaseErr
				}
				if err = s.schedulerSetReason(item.ID, "queued", "source_unavailable", reserveErr.Error()); err != nil {
					return err
				}
				s.deferSwarmQueueAttempt(item.ID)
			}
			continue
		}
		if err = s.schedulerUpdate(item.ID, func(current *swarmQueueRecord) error {
			current.OperationID = op.ID
			current.ChildID = op.ChildID
			schedulerReason(current, "reserved", "reserved", "An isolated checkout and resource reservation are ready for this exact child.")
			return nil
		}); err != nil {
			return err
		}
		if op.Allocation != "ready" || op.Status != "reserved" {
			if err = s.reconcileQueuedSwarm(ctx, swarmQueueRecord{SwarmQueueItem: SwarmQueueItem{ID: item.ID, ProjectID: item.ProjectID, RunID: item.RunID, OperationID: op.ID}}, runtime); err != nil {
				return err
			}
			continue
		}
		if err = s.schedulerSetReason(item.ID, "dispatching", "dispatching", "Starting the reserved child; an uncertain launch will never be replayed."); err != nil {
			return err
		}
		result, dispatchErr := runtime.Dispatch(ctx, SwarmDispatchRequest{QueueID: item.ID, ProjectID: item.ProjectID, RunID: item.RunID, OperationID: op.ID, Actor: item.Actor})
		if dispatchErr != nil {
			if err = s.schedulerSetReason(item.ID, "unknown", "dispatch_unknown", "The reserved child's launch outcome is unknown. Reconciliation will inspect it without starting a replacement."); err != nil {
				return err
			}
			continue
		}
		state := result.State
		if state == "failed" && result.SessionID == op.ChildID {
			// The executor result is not outcome or release authority. Observe the
			// durable exact child and use the existing lifecycle release guard.
			if err = s.reconcileQueuedSwarm(ctx, item, runtime); err != nil {
				return err
			}
			continue
		}
		if state == "waiting" || state == "blocked" {
			inspected, inspectErr := s.GetProjectSwarmRun(item.ProjectID, item.RunID)
			latestOp := swarmOperation(&inspected, op.ID)
			if inspectErr != nil || latestOp == nil || latestOp.Status != "reserved" {
				state = "unknown"
				result.ReasonCode = "dispatch_unknown"
				result.Message = "Dispatch was claimed before it stopped. Its exact child and capacity are retained; no replacement will be launched."
			}
		}
		if state == "running" && result.SessionID != op.ChildID {
			state = "unknown"
			result.SessionID = ""
			result.ReasonCode = "child_identity_unconfirmed"
			result.Message = "The executor did not confirm the reserved child identity. Inspect its existing attempt before continuing."
		}
		switch state {
		case "running":
		case "waiting":
			state = "queued"
			defer s.deferSwarmQueueAttempt(item.ID)
			if releaseErr := s.releaseQueueLease(ctx, item.ID, runtime); releaseErr != nil {
				return releaseErr
			}
		case "blocked":
			if releaseErr := s.releaseQueueLease(ctx, item.ID, runtime); releaseErr != nil {
				return releaseErr
			}
		default:
			state = "unknown"
		}
		if err = s.schedulerUpdate(item.ID, func(current *swarmQueueRecord) error {
			if result.SessionID != "" && result.SessionID != current.ChildID {
				return ErrProjectRunConflict
			}
			schedulerReason(current, state, result.ReasonCode, result.Message)
			return nil
		}); err != nil {
			return err
		}
		if state == "running" {
			s.swarmScheduler.mu.Lock()
			latest, err = s.readSwarmScheduler()
			if err == nil {
				latest.Sequence++
				latest.LastServed[item.ProjectID] = latest.Sequence
				err = s.saveSwarmScheduler(&latest)
			}
			s.swarmScheduler.mu.Unlock()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Server) reconcileQueuedSwarm(ctx context.Context, item swarmQueueRecord, runtime SwarmSchedulerRuntime) error {
	p, err := s.resolveProject(item.ProjectID)
	if err != nil {
		return s.schedulerSetReason(item.ID, "unknown", "project_unavailable", "The reserved project is unavailable. Its capacity remains held until its child can be identified.")
	}
	run, op, found, err := s.projectRuns.FindOperationByKey(p.ID, p.Path, item.RunID, schedulerReserveKey(item), "session")
	if err != nil {
		return s.schedulerSetReason(item.ID, "unknown", "receipt_unavailable", "The private operation receipt is unavailable. Its capacity remains held while other projects can continue.")
	}
	if !found {
		if item.State == "claiming" {
			return nil
		}
		return s.schedulerSetReason(item.ID, "unknown", "receipt_missing", "The reserved operation cannot be identified. No replacement will be launched.")
	}
	if swarmRetirementPending(op) {
		return s.schedulerSetReason(item.ID, "unknown", "retirement_pending", "Finish this attempt's explicit launch recovery before retrying. Its output, frozen context and execution reservation remain retained.")
	}
	if op.Status == "cancelled" || op.Allocation == "abandoned" {
		if err = s.releaseQueueLease(ctx, item.ID, runtime); err != nil {
			return s.schedulerSetReason(item.ID, "unknown", "resource_release_pending", "The task was retired, but its execution reservation cannot yet be confirmed released. No replacement will be launched.")
		}
		return s.schedulerSetReason(item.ID, "cancelled", "attempt_retired", "The attempt was retired. Retained work remains available.")
	}
	if op.Allocation != "ready" {
		return s.schedulerUpdate(item.ID, func(current *swarmQueueRecord) error {
			current.OperationID = op.ID
			current.ChildID = op.ChildID
			schedulerReason(current, "unknown", "allocation_unknown", "The retained task checkout has an unconfirmed allocation. Inspect and retire the attempt explicitly before retrying.")
			return nil
		})
	}
	if op.Status == "reserved" {
		return s.schedulerUpdate(item.ID, func(current *swarmQueueRecord) error {
			current.OperationID = op.ID
			current.ChildID = op.ChildID
			if current.State == "claiming" || current.State == "dispatching" {
				schedulerReason(current, "reserved", "reserved", "The exact reserved child has not been claimed; its existing checkout can continue safely.")
			}
			return nil
		})
	}
	if s.projectRunSessionObserver != nil && !projectRunOperationTerminal(op.Status) {
		s.projectRunDispatchMu.Lock()
		run, err = s.observeProjectSwarm(ctx, p.Path, run)
		s.projectRunDispatchMu.Unlock()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return s.schedulerSetReason(item.ID, "unknown", "observation_unavailable", "The reserved child could not be observed. Its capacity remains held; unrelated projects can continue.")
		}
		observed := swarmOperation(&run, op.ID)
		if observed != nil {
			op = *observed
		}
	}
	state, code, reason := "unknown", "child_unknown", "The reserved child cannot be identified. Its capacity remains held; no replacement will be started."
	switch op.Status {
	case "running":
		state, code, reason = "running", "running", "The exact reserved agent is running in its isolated checkout."
	case "succeeded", "review-required":
		state, code, reason = "completed", "review_required", "The agent stopped. Review its retained changes and verify the result; process exit does not integrate dependencies."
	case "failed":
		state, code, reason = "failed", "worker_failed", "The worker failed. Inspect its retained output and verify any changes before deciding whether to retry; no result was accepted or integrated."
		if op.StartFailure.Valid() {
			code, reason = op.StartFailure.ReasonCode, op.StartFailure.Message
		}
	case "cancelled":
		state, code, reason = "cancelled", "attempt_retired", "The attempt was retired. Retained work remains available."
	}
	if swarmQueueTerminal(state) {
		if err = s.releaseQueueLease(ctx, item.ID, runtime); err != nil {
			if state == "failed" {
				// Preserve the recorded failure while showing its independent held
				// reservation. Terminal cleanup retries without launching this item.
				reason += " Its execution reservation remains held pending confirmed release."
			} else {
				return s.schedulerSetReason(item.ID, "unknown", "resource_release_pending", "The child recorded an exit, but its execution reservation cannot yet be confirmed released. Capacity remains held.")
			}
		}
	}
	if err = s.schedulerUpdate(item.ID, func(current *swarmQueueRecord) error {
		current.OperationID = op.ID
		current.ChildID = op.ChildID
		schedulerReason(current, state, code, reason)
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (s *Server) releaseQueueLease(ctx context.Context, id string, runtime SwarmSchedulerRuntime) error {
	if err := runtime.Release(ctx, id); err != nil {
		return err
	}
	releasedCapacity := false
	err := s.schedulerUpdate(id, func(item *swarmQueueRecord) error {
		releasedCapacity = item.MemoryMiB > 0 || item.CPUMillis > 0
		item.MemoryMiB = 0
		item.CPUMillis = 0
		if item.State == "failed" {
			item.Reason = strings.TrimSuffix(item.Reason, " Its execution reservation remains held pending confirmed release.")
		}
		return nil
	})
	if err == nil && releasedCapacity {
		s.swarmScheduler.mu.Lock()
		s.swarmScheduler.cooldowns = nil
		s.swarmScheduler.mu.Unlock()
	}
	return err
}
func (s *Server) SwarmSchedulerDispatchEnabled() (bool, error) {
	s.swarmScheduler.mu.Lock()
	defer s.swarmScheduler.mu.Unlock()
	row, err := s.readSwarmScheduler()
	return !row.Paused, err
}

// Admission may reserve capacity, so its claim must remain durable. A short,
// ephemeral cooldown avoids repeatedly writing claim/wait receipts while the
// same measured pressure persists. It never grants authority or skips checks.
func (s *Server) deferSwarmQueueAttempt(id string) {
	s.swarmScheduler.mu.Lock()
	defer s.swarmScheduler.mu.Unlock()
	if s.swarmScheduler.cooldowns == nil {
		s.swarmScheduler.cooldowns = map[string]time.Time{}
	}
	s.swarmScheduler.cooldowns[id] = s.schedulerNow().Add(4 * time.Second)
}
func (s *Server) swarmQueueCoolingDown(id string) bool {
	s.swarmScheduler.mu.Lock()
	defer s.swarmScheduler.mu.Unlock()
	until := s.swarmScheduler.cooldowns[id]
	if !until.IsZero() && !s.schedulerNow().Before(until) {
		delete(s.swarmScheduler.cooldowns, id)
		return false
	}
	return !until.IsZero()
}
