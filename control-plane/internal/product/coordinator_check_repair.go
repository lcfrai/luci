package product

import (
	"context"
	"strconv"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

type coordinatorCheckRepairContextKey struct{}

func coordinatorCheckActionID(id string, ordinal int) string {
	if ordinal == 0 { return id + "-checks" }
	return id + "-checks-" + strconv.Itoa(ordinal+1)
}

func coordinatorDeliveryCheckID(id string, ordinal int) string {
	if ordinal == 0 { return coordinatorCheckID(id) }
	return "check-" + reviewDigest(coordinatorCheckActionID(id, ordinal))[:24]
}

func coordinatorCheckRepairEnabled(run ProjectRun) bool {
	return run.Input.OrchestrationContract != nil && run.Input.OrchestrationContract.Input.Controls.CheckFailureRepairs
}

// The original delivery identity and actual command receipts are required.
// A generic failed/blocked status or a model's diagnostic is never sufficient.
func coordinatorCheckRepairEligible(run ProjectRun, row coordinatorContinuationRecord, check ProjectCheckRun) bool {
	if run.State == "blocked" || run.State == "cancelled" || run.State == "complete" { return false }
	if !coordinatorCheckRepairEnabled(run) || row.CheckPlan == nil || row.Delivery == nil || row.Delivery.Status != "blocked" || check.ID != row.Delivery.CheckID || check.ProjectID != run.ProjectID || check.Status != "failed" || check.Dirty || check.SourceChanged || check.ResourcesHeld == nil || *check.ResourcesHeld || check.Commit != row.Delivery.Commit || check.SourceDigest != row.Delivery.SourceDigest || check.CoordinatorOriginDigest != coordinatorCheckBinding(row) || !settledFailedCommands(check.Commands) || len(check.Commands) != len(row.CheckPlan.Commands) { return false }
	for i, command := range check.Commands { if reviewDigest(command.CheckCommand) != reviewDigest(row.CheckPlan.Commands[i]) { return false } }
	integration, ready := coordinatorIntegratedRun(run, row)
	return ready && integration == row.Delivery.IntegrationDigest
}

func (s *Server) coordinatorCheckRepairBackgroundCandidate(p project.Project, row coordinatorContinuationRecord) bool {
	if row.Delivery == nil || row.Delivery.Status != "blocked" || row.CheckPlan == nil { return false }
	run, err := s.projectRuns.Get(p.ID, p.Path, row.RunID)
	if err != nil || !coordinatorCheckRepairEnabled(run) { return false }
	if activeCoordinatorCheckRepair(&run, row) != nil { return true }
	check, err := s.GetProjectCheckRun(p.ID, row.Delivery.CheckID)
	if err != nil || !coordinatorCheckRepairEligible(run, row, check) { return false }
	limit, err := swarmAutomaticRepairLimit(run)
	if err != nil { return false }
	covered, err := checkRepairCoveredTasks(run, row.CheckPlan.RequirementIDs)
	if err != nil { return false }
	for _, id := range covered { if swarmRepairsConsumed(run, id) >= limit { return false } }
	return true
}

func activeCoordinatorCheckRepair(run *ProjectRun, row coordinatorContinuationRecord) *ProjectSwarmCheckRepair {
	if run.Swarm == nil || row.Delivery == nil { return nil }
	for i := range run.Swarm.CheckRepairs { wave := &run.Swarm.CheckRepairs[i]; if wave.ContinuationID == row.ID && wave.CheckID == row.Delivery.CheckID { return wave } }
	return nil
}

// This method performs at most one durable intent per pass. A saved wave owns
// all task budget charges before scheduler admission or checkout side effects.
func (s *Server) continueCoordinatorCheckRepair(ctx context.Context, p project.Project, row coordinatorContinuationRecord) (coordinatorContinuationRecord, bool, error) {
	if row.Delivery == nil || row.Delivery.Status != "blocked" || row.CheckPlan == nil { return row, false, nil }
	run, err := s.projectRuns.Get(p.ID, p.Path, row.RunID)
	if err != nil || !coordinatorCheckRepairEnabled(run) { return row, false, err }
	if !s.coordinatorIndependentReviewCurrent(ctx, p, row) || s.contractCurrentForRun(p, run) != nil || s.validateProjectSwarmReuseCurrent(ctx, p, run) != nil { return row, true, ErrOrchestratorConflict }
	wave := activeCoordinatorCheckRepair(&run, row)
	if wave == nil {
		check, readErr := s.GetProjectCheckRun(p.ID, row.Delivery.CheckID)
		if readErr != nil || !coordinatorCheckRepairEligible(run, row, check) { return row, false, readErr }
		guard := s.branchGuard(p.ID)
		if !guard.TryLock() { return row, true, errCoordinatorSourceBusy }
		defer guard.Unlock()
		bounded, cancel := context.WithTimeout(ctx, 20*time.Second)
		digest, commit, dirty, sourceErr := s.checkSource(bounded, p.Path)
		cancel()
		if sourceErr != nil || dirty || digest != check.SourceDigest || commit != check.Commit { return row, true, ErrProjectRunConflict }
		limit, limitErr := swarmAutomaticRepairLimit(run)
		if limitErr != nil || limit == 0 { return row, false, nil }
		created := ProjectSwarmCheckRepair{ID: newID("check-repair-"), ContinuationID: row.ID, CheckID: check.ID, CheckBinding: check.CoordinatorOriginDigest, Commit: commit, SourceDigest: digest, IntegrationDigest: row.Delivery.IntegrationDigest, RequirementIDs: append([]string(nil), row.CheckPlan.RequirementIDs...), CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
		covered, coverageErr := checkRepairCoveredTasks(run, created.RequirementIDs)
		if coverageErr != nil { return row, true, coverageErr }
		// Full selected command coverage does not identify a guilty task. Repair
		// each captured coding scope covered by that plan, without adding scopes.
		for _, id := range covered {
			task := swarmTask(&run, id)
			if task == nil || task.Integration == nil || task.Integration.State != "integrated" { return row, true, ErrProjectRunConflict }
			ordinal := swarmRepairsConsumed(run, task.ID)+1
			if ordinal > limit { return row, false, nil }
			created.Tasks = append(created.Tasks, ProjectSwarmCheckRepairTask{TaskID: task.ID, SourceOperationID: task.OperationID, SourceIntegration: *task.Integration, BudgetOrdinal: ordinal})
		}
		created.Tasks, err = orderCheckRepairTasks(run, created.Tasks)
		if err != nil { return row, true, err }
		// Commands retain structured exit/cleanup proof only; raw output is not
		// copied into a new task's context or the public repair projection.
		for _, command := range check.Commands { command.Output = ""; command.Message = ""; created.Commands = append(created.Commands, command) }
		if _, diagnosticErr := checkRepairDiagnostic(created); diagnosticErr != nil { return row, true, diagnosticErr }
		s.projectRunDispatchMu.Lock()
		defer s.projectRunDispatchMu.Unlock()
		if !s.coordinatorIndependentReviewCurrent(ctx, p, row) { return row, true, ErrOrchestratorConflict }
		s.mu.RLock()
		defer s.mu.RUnlock()
		if err = s.swarmRepairPolicyCurrentLocked(p, run); err != nil { return row, true, err }
		_, err = s.projectRuns.mutateSwarm(p.ID, p.Path, run.ID, run.Revision, func(current *ProjectRun) error {
			currentCovered, coverageErr := checkRepairCoveredTasks(*current, created.RequirementIDs)
			if coverageErr != nil || reviewDigest(currentCovered) != reviewDigest(covered) || activeCoordinatorCheckRepair(current, row) != nil || !coordinatorCheckRepairEligible(*current, row, check) { return ErrProjectRunConflict }
			for _, member := range created.Tasks { if swarmRepairsConsumed(*current, member.TaskID)+1 != member.BudgetOrdinal { return ErrProjectRunConflict } }
			current.Swarm.CheckRepairs = append(current.Swarm.CheckRepairs, created)
			return nil
		})
		return row, true, err
	}
	if row.Status != "queued" { row, err = s.persistCoordinatorCheckRepair(ctx, p, row, false); return row, true, err }
	// Every member must obtain a new passed independent review and integration.
	complete := true
	for _, member := range wave.Tasks {
		task := swarmTask(&run, member.TaskID)
		if member.OperationID == "" {
			complete = false
			if task != nil && swarmCheckRepairReady(run, *task, wave.ID) == nil {
				created, queueErr := s.queueCoordinatorCheckRepair(ctx, p, row, run, *wave, member)
				if created || queueErr != nil { return row, true, queueErr }
			}
			continue
		}
		if !checkRepairMemberIntegrated(run, member) { complete = false }
	}
	if !complete { return row, true, nil }
	if _, ready := coordinatorIntegratedRun(run, row); !ready { return row, true, nil }
	row, err = s.persistCoordinatorCheckRepair(ctx, p, row, true)
	return row, true, err
}

func (s *Server) persistCoordinatorCheckRepair(ctx context.Context, p project.Project, row coordinatorContinuationRecord, finished bool) (coordinatorContinuationRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.readContinuations(p, row.Actor.ID)
	index := findContinuation(rows, row.ID)
	if err != nil || index < 0 { return row, ErrOrchestratorConflict }
	current := rows.Rows[index]
	if current.Revision != row.Revision || s.coordinatorDeliveryAuthorityLocked(ctx, p, current) != nil { return current, ErrOrchestratorConflict }
	run, err := s.projectRuns.Get(p.ID, p.Path, row.RunID)
	if err != nil || activeCoordinatorCheckRepair(&run, current) == nil { return current, ErrOrchestratorConflict }
	if finished {
		wave := activeCoordinatorCheckRepair(&run, current)
		for _, member := range wave.Tasks { if !checkRepairMemberIntegrated(run, member) { return current, ErrOrchestratorConflict } }
		if _, ready := coordinatorIntegratedRun(run, current); !ready { return current, ErrOrchestratorConflict }
		current.PriorDeliveries = append(current.PriorDeliveries, *current.Delivery)
		current.Delivery = nil
		current.Message = "Repaired coding scopes have fresh independent reviews and integrations. A new exact combined-check identity will be reserved next."
	} else { current.Message = "A settled combined-check failure has a bounded repair wave. Its failed check and prior attempts remain retained; fresh candidates require independent review and integration." }
	current.Status = "queued"
	current.Revision++
	current.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	rows.Rows[index] = current
	return current, s.saveContinuations(rows)
}

func (s *Server) queueCoordinatorCheckRepair(ctx context.Context, p project.Project, row coordinatorContinuationRecord, run ProjectRun, wave ProjectSwarmCheckRepair, member ProjectSwarmCheckRepairTask) (bool, error) {
	task := swarmTask(&run, member.TaskID)
	if task == nil || swarmCheckRepairReady(run, *task, wave.ID) != nil { return false, nil }
	s.swarmScheduler.mu.Lock(); runtime := s.swarmScheduler.runtime; s.swarmScheduler.mu.Unlock()
	if runtime.CaptureActor == nil || runtime.Admit == nil || runtime.Dispatch == nil || runtime.Release == nil { return false, nil }
	actor, err := runtime.CaptureActor(ctx, p.ID)
	if err != nil || actor.ProjectID != p.ID { return false, errCoordinatorAuthority }
	s.projectRunDispatchMu.Lock(); defer s.projectRunDispatchMu.Unlock()
	current, err := s.projectRuns.Get(p.ID, p.Path, run.ID)
	if err != nil || current.Revision != run.Revision || !s.coordinatorIndependentReviewCurrent(ctx, p, row) { return false, ErrProjectRunConflict }
	s.mu.RLock(); defer s.mu.RUnlock()
	if err = s.swarmRepairPolicyCurrentLocked(p, current); err != nil { return false, err }
	wake := false
	s.swarmScheduler.mu.Lock()
	defer func() { s.swarmScheduler.mu.Unlock(); if wake { s.WakeSwarmScheduler() } }()
	ledger, err := s.readSwarmScheduler()
	if err != nil { return false, err }
	key := "coordinator-check-repair-" + reviewDigest(wave.ID+"/"+member.TaskID)[:24]
	for _, prior := range ledger.Items { if prior.ProjectID == p.ID && prior.BatchKeyDigest == reviewDigest(key) { if prior.RunID != run.ID || prior.CheckRepairWaveID != wave.ID || prior.TaskID != member.TaskID { return false, ErrProjectRunKeyConflict }; return false, nil } }
	if len(ledger.Items) >= swarmQueueLimit || ledger.PolicyRevision != run.Input.OrchestrationContract.Input.Controls.SchedulerPolicyRevision || reviewDigest(ledger.Policy) != run.Input.OrchestrationContract.Input.Controls.SchedulerPolicyDigest { return false, ErrProjectRunConflict }
	for _, prior := range ledger.Items { if prior.ProjectID == p.ID && prior.RunID == run.ID && prior.TaskID == member.TaskID && !swarmQueueTerminal(prior.State) { return false, nil } }
	now := s.schedulerNow().Format(time.RFC3339Nano)
	ledger.Items = append(ledger.Items, swarmQueueRecord{SwarmQueueItem: SwarmQueueItem{Revision: 1, ID: newID("queue-"), ProjectID: p.ID, RunID: run.ID, TaskID: task.ID, TaskTitle: task.Title, Priority: "normal", State: "queued", QueuedAt: now, UpdatedAt: now, ReasonCode: "check_repair_ready", Reason: "The exact failed combined source awaits a bounded coding repair, then a new independent review."}, Actor: actor, InputDigest: queueInputDigest(run), KeyDigest: reviewDigest(key+"/"+task.ID), BatchKeyDigest: reviewDigest(key), RequestDigest: reviewDigest(swarmCheckRepairOrigin(wave, member)), CheckRepairWaveID: wave.ID, CheckRepairContinuationID: row.ID, CheckRepairActorID: row.Actor.ID})
	err = s.saveSwarmScheduler(&ledger)
	wake = err == nil
	return true, err
}

func (s *Server) checkCoordinatorCheckRepairGate(ctx context.Context, p project.Project, run ProjectRun, waveID string) error {
	if run.State == "blocked" || run.State == "cancelled" || run.State == "complete" || run.Swarm == nil || run.Swarm.State != "ready" { return ErrProjectRunConflict }
	gate, ok := ctx.Value(coordinatorIndependentReviewContextKey{}).(coordinatorIndependentReviewGate)
	wave := swarmCheckRepairWave(&run, waveID)
	if !ok || gate.Kind != "check-repair" || wave == nil || gate.Row.RunID != run.ID || gate.Row.ID != wave.ContinuationID || gate.Row.Delivery == nil || gate.Row.Delivery.CheckID != wave.CheckID || wave.CheckBinding != coordinatorCheckBinding(gate.Row) || wave.Commit != gate.Row.Delivery.Commit || wave.SourceDigest != gate.Row.Delivery.SourceDigest || wave.IntegrationDigest != gate.Row.Delivery.IntegrationDigest || gate.Row.CheckPlan == nil || reviewDigest(wave.RequirementIDs) != reviewDigest(gate.Row.CheckPlan.RequirementIDs) || gate.Row.Status != "queued" || !validSwarmCheckRepairs(run) || !s.coordinatorIndependentReviewCurrent(ctx, p, gate.Row) || s.contractCurrentForRun(p, run) != nil { return ErrProjectRunConflict }
	return nil
}

func (s *Server) coordinatorCheckRepairAuthority(ctx context.Context, p project.Project, item swarmQueueRecord) (context.Context, context.CancelFunc, error) {
	// Reuse exact retained actor reauthorization without granting reviewer role.
	copy := item
	copy.AutoReviewContinuationID, copy.AutoReviewActorID = item.CheckRepairContinuationID, item.CheckRepairActorID
	fresh, _, cancel, err := s.coordinatorReviewerRetryAuthority(ctx, p, copy)
	if err != nil { return nil, cancel, err }
	gate := fresh.Value(coordinatorIndependentReviewContextKey{}).(coordinatorIndependentReviewGate)
	gate.Kind = "check-repair"
	fresh = context.WithValue(fresh, coordinatorIndependentReviewContextKey{}, gate)
	fresh = context.WithValue(fresh, coordinatorCheckRepairContextKey{}, item.CheckRepairWaveID)
	run, err := s.projectRuns.Get(p.ID, p.Path, item.RunID)
	wave := swarmCheckRepairWave(&run, item.CheckRepairWaveID)
	member := swarmCheckRepairTask(wave, item.TaskID)
	if err != nil || member == nil || wave.ContinuationID != item.CheckRepairContinuationID || item.InputDigest != queueInputDigest(run) || item.RequestDigest != reviewDigest(swarmCheckRepairOrigin(*wave, *member)) || s.checkCoordinatorCheckRepairGate(fresh, p, run, wave.ID) != nil {
		cancel(); return nil, nil, ErrProjectRunConflict
	}
	if item.OperationID != "" && (member.OperationID != item.OperationID || !swarmCheckRepairCandidate(run, item.OperationID)) { cancel(); return nil, nil, ErrProjectRunConflict }
	return fresh, cancel, nil
}
