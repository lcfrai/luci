package product

import (
	"encoding/json"
	"errors"
	"time"
)

// Check repairs are a separate origin, never an invented reviewer finding.
// A wave charges each selected coding task before any checkout is allocated.
type ProjectSwarmCheckRepair struct {
	ID string `json:"id"`
	ContinuationID string `json:"continuationId"`
	CheckID string `json:"checkId"`
	CheckBinding string `json:"checkBinding"`
	Commit string `json:"commit"`
	SourceDigest string `json:"sourceDigest"`
	IntegrationDigest string `json:"integrationDigest"`
	RequirementIDs []string `json:"requirementIds"`
	Commands []CheckCommandResult `json:"commands"`
	Tasks []ProjectSwarmCheckRepairTask `json:"tasks"`
	CreatedAt string `json:"createdAt"`
}

type ProjectSwarmCheckRepairTask struct {
	TaskID string `json:"taskId"`
	SourceOperationID string `json:"sourceOperationId"`
	SourceIntegration ProjectSwarmIntegration `json:"sourceIntegration"`
	BudgetOrdinal int `json:"budgetOrdinal"`
	OperationID string `json:"operationId,omitempty"`
}

type ProjectSwarmCheckRepairOrigin struct {
	WaveID string `json:"waveId"`
	CheckID string `json:"checkId"`
	FailedCommit string `json:"failedCommit"`
	SourceOperationID string `json:"sourceOperationId"`
	BudgetOrdinal int `json:"budgetOrdinal"`
}

func swarmCheckRepairWave(run *ProjectRun, id string) *ProjectSwarmCheckRepair {
	if run.Swarm != nil { for i := range run.Swarm.CheckRepairs { if run.Swarm.CheckRepairs[i].ID == id { return &run.Swarm.CheckRepairs[i] } } }
	return nil
}

func swarmCheckRepairTask(wave *ProjectSwarmCheckRepair, taskID string) *ProjectSwarmCheckRepairTask {
	if wave != nil { for i := range wave.Tasks { if wave.Tasks[i].TaskID == taskID { return &wave.Tasks[i] } } }
	return nil
}

// A genuine failed new reviewer may consume the remaining shared budget via
// the existing checkpoint path. Preserve the wave's first reservation; follow
// only exact retained finding/checkpoint edges to its current descendant.
func checkRepairMemberIntegrated(run ProjectRun, member ProjectSwarmCheckRepairTask) bool {
	task := swarmTask(&run, member.TaskID)
	if task == nil || member.OperationID == "" || task.Integration == nil || task.Integration.State != "integrated" { return false }
	id := member.OperationID
	for step := 0; step <= maxSwarmReviewRepairs; step++ {
		if task.OperationID == id { return true }
		next := ""
		for _, finding := range run.Swarm.ReviewObligations {
			if finding.TaskID != member.TaskID || finding.State != "repair-required" || finding.CandidateOperationID != id || finding.RepairOperationID == "" { continue }
			op := swarmOperation(&run, finding.RepairOperationID)
			if op == nil || op.Checkpoint == nil || op.Checkpoint.SourceOperationID != id || op.Checkpoint.SourceChildID != finding.CandidateChildID || op.Checkpoint.SourceCommit != finding.CandidateCommit { return false }
			if next != "" { return false }
			next = op.ID
		}
		if next == "" { return false }
		id = next
	}
	return false
}

func swarmCheckRepairOrigin(wave ProjectSwarmCheckRepair, task ProjectSwarmCheckRepairTask) *ProjectSwarmCheckRepairOrigin {
	return &ProjectSwarmCheckRepairOrigin{wave.ID, wave.CheckID, wave.Commit, task.SourceOperationID, task.BudgetOrdinal}
}

func orderCheckRepairTasks(run ProjectRun, tasks []ProjectSwarmCheckRepairTask) ([]ProjectSwarmCheckRepairTask, error) {
	ordered := []ProjectSwarmCheckRepairTask{}
	done, selected := map[string]bool{}, map[string]bool{}
	for _, task := range tasks { selected[task.TaskID] = true }
	for len(ordered) < len(tasks) {
		progress := false
		for _, member := range tasks {
			if done[member.TaskID] { continue }
			task := swarmTask(&run, member.TaskID)
			if task == nil { return nil, ErrProjectRunConflict }
			ready := true
			for _, dep := range task.DependsOn { if selected[dep] && !done[dep] { ready = false } }
			if ready { ordered = append(ordered, member); done[member.TaskID] = true; progress = true }
		}
		if !progress { return nil, ErrProjectRunConflict }
	}
	return ordered, nil
}

func checkRepairCoveredTasks(run ProjectRun, requirementIDs []string) ([]string, error) {
	if run.Swarm == nil || run.Input.OrchestrationContract == nil || len(requirementIDs) == 0 || validateRequirementIDs(requirementIDs) != nil { return nil, ErrProjectRunConflict }
	covered, known := map[string]bool{}, map[string]bool{}
	for _, item := range run.Input.Requirements { known[item.ID] = true }
	for _, id := range requirementIDs { if !known[id] { return nil, ErrProjectRunConflict }; covered[id] = true }
	selected := []string{}
	for _, task := range run.Input.OrchestrationContract.Input.Tasks {
		current := swarmTask(&run, task.ID)
		if current == nil { return nil, ErrProjectRunConflict }
		if current.ReviewOnly || task.Role == "reviewer" { continue }
		for _, id := range task.RequirementIDs { if covered[id] { selected = append(selected, task.ID); break } }
	}
	if len(selected) == 0 { return nil, ErrProjectRunConflict }
	return selected, nil
}

func swarmCheckRepairOperation(run ProjectRun, op ProjectRunOperation) bool {
	if op.CheckRepair == nil || op.Checkpoint != nil { return false }
	wave := swarmCheckRepairWave(&run, op.CheckRepair.WaveID)
	task := swarmCheckRepairTask(wave, op.TaskID)
	return task != nil && task.OperationID == op.ID && (op.Implementation == nil || op.Implementation.SourceCommit == op.BaseCommit && !op.Implementation.SourceDirty) && reviewDigest(op.CheckRepair) == reviewDigest(swarmCheckRepairOrigin(*wave, *task))
}

func validSwarmCheckRepairs(run ProjectRun) bool {
	if run.Swarm == nil { return true }
	if len(run.Swarm.CheckRepairs) > maxSwarmReviewRepairs { return false }
	seen := map[string]bool{}
	for _, wave := range run.Swarm.CheckRepairs {
		if run.Input.OrchestrationContract == nil || !run.Input.OrchestrationContract.Input.Controls.CheckFailureRepairs || !prototypeReleaseID.MatchString(wave.ID) || !prototypeReleaseID.MatchString(wave.ContinuationID) || !prototypeReleaseID.MatchString(wave.CheckID) || seen[wave.ID] || !hexDigest(wave.CheckBinding) || !deliverySHA.MatchString(wave.Commit) || !hexDigest(wave.SourceDigest) || !hexDigest(wave.IntegrationDigest) || len(wave.Tasks) == 0 || !settledFailedCommands(wave.Commands) { return false }
		seen[wave.ID] = true
		if _, err := time.Parse(time.RFC3339Nano, wave.CreatedAt); err != nil { return false }
		covered, err := checkRepairCoveredTasks(run, wave.RequirementIDs)
		if err != nil || len(covered) != len(wave.Tasks) { return false }
		wanted := map[string]bool{}
		for _, id := range covered { wanted[id] = true }
		tasks := map[string]bool{}
		for _, member := range wave.Tasks {
			task := swarmTask(&run, member.TaskID)
			source := swarmOperation(&run, member.SourceOperationID)
			limit, err := swarmAutomaticRepairLimit(run)
			prior := member.SourceIntegration
			if !wanted[member.TaskID] || task == nil || task.ReviewOnly || tasks[member.TaskID] || source == nil || source.TaskID != member.TaskID || prior.State != "integrated" || !validSwarmIntegrationDiagnostic(prior) || !deliverySHA.MatchString(prior.Commit) || !deliverySHA.MatchString(prior.SourceCommit) || !deliverySHA.MatchString(prior.TargetCommit) || !hexDigest(prior.ReviewDigest) || err != nil || member.BudgetOrdinal < 1 || member.BudgetOrdinal > limit { return false }
			if _, err := time.Parse(time.RFC3339Nano, prior.IntegratedAt); err != nil { return false }
			tasks[member.TaskID] = true
			if member.OperationID != "" { op := swarmOperation(&run, member.OperationID); if op == nil || !swarmCheckRepairOperation(run, *op) { return false } }
		}
	}
	for _, op := range run.Operations { if op.CheckRepair != nil && !swarmCheckRepairOperation(run, op) { return false } }
	return true
}

// Timeout, pre-start, cancelled and unknown exits cannot authorize a repair.
func settledFailedCommands(commands []CheckCommandResult) bool {
	if len(commands) == 0 { return false }
	failed := false
	for _, command := range commands {
		if command.Status == "skipped" { if !failed || !command.NotStartedAfterFailure || command.ExitCode != nil || command.Stage != "" { return false }; continue }
		if failed || command.NotStartedAfterFailure { return false }
		if command.AllocationID == "" || !command.CleanupConfirmed || command.ExitCode == nil || command.Status != "passed" && command.Status != "failed" { return false }
		if command.Status == "failed" { if *command.ExitCode == 0 { return false }; failed = true } else if *command.ExitCode != 0 { return false }
	}
	return failed
}

func swarmCheckRepairReady(run ProjectRun, task ProjectSwarmTask, waveID string) error {
	if run.State == "blocked" { return ErrProjectRunConflict }
	wave := swarmCheckRepairWave(&run, waveID)
	member := swarmCheckRepairTask(wave, task.ID)
	if wave == nil || member == nil || len(run.Swarm.CheckRepairs) == 0 || run.Swarm.CheckRepairs[len(run.Swarm.CheckRepairs)-1].ID != waveID || member.OperationID != "" || task.OperationID != member.SourceOperationID || task.Integration == nil || task.Integration.State != "integrated" || reviewDigest(*task.Integration) != reviewDigest(member.SourceIntegration) { return ErrProjectRunConflict }
	// Only actual selected dependency edges wait for repaired source. Unrelated
	// ready members use the existing scheduler's scope/resource parallel limits.
	for _, dep := range task.DependsOn {
		prior := swarmCheckRepairTask(wave, dep)
		if prior == nil { continue }
		if !checkRepairMemberIntegrated(run, *prior) { return ErrProjectRunConflict }
	}
	task.Integration = nil
	return swarmTaskReady(run, task)
}

func swarmCheckRepairBudget(run ProjectRun, op ProjectRunOperation) error {
	if !swarmCheckRepairOperation(run, op) { return ErrProjectRunConflict }
	task := swarmTask(&run, op.TaskID)
	if task == nil || task.OperationID != op.ID { return ErrProjectRunConflict }
	limit, err := swarmAutomaticRepairLimit(run)
	if err != nil || op.CheckRepair.BudgetOrdinal > limit || swarmRepairsConsumed(run, op.TaskID) > limit { return errAutomaticRepairLimit }
	wave := swarmCheckRepairWave(&run, op.CheckRepair.WaveID)
	if wave.ID != run.Swarm.CheckRepairs[len(run.Swarm.CheckRepairs)-1].ID { return ErrProjectRunConflict }
	return nil
}

func swarmCheckRepairCandidate(run ProjectRun, id string) bool {
	op := swarmOperation(&run, id)
	return op != nil && op.CheckRepair != nil && swarmCheckRepairBudget(run, *op) == nil
}

// This is a private scheduling projection, not a persisted review obligation.
// The normal reviewer still creates and resolves a genuine new obligation.
type coordinatorRepairReviewCandidate struct { ID, TaskID, CandidateOperationID, RepairOperationID string }

func coordinatorRepairReviewCandidates(run ProjectRun) []coordinatorRepairReviewCandidate {
	if run.Swarm == nil { return nil }
	items := []coordinatorRepairReviewCandidate{}
	for _, finding := range run.Swarm.ReviewObligations { if finding.State == "repair-required" && finding.RepairOperationID != "" { items = append(items, coordinatorRepairReviewCandidate{finding.ID, finding.TaskID, finding.CandidateOperationID, finding.RepairOperationID}) } }
	for _, wave := range run.Swarm.CheckRepairs { for _, member := range wave.Tasks {
		if member.OperationID != "" && swarmCheckRepairCandidate(run, member.OperationID) {
			items = append(items, coordinatorRepairReviewCandidate{wave.ID, member.TaskID, member.SourceOperationID, member.OperationID})
		}
	} }
	return items
}

func checkRepairDiagnostic(wave ProjectSwarmCheckRepair) ([]byte, error) {
	// Only structured declared commands and actual exit codes are supplied.
	// Raw command output and model prose are not copied as repair context.
	type result struct { CheckCommand; Status string `json:"status"`; ExitCode *int `json:"exitCode"` }
	results := []result{}
	for _, command := range wave.Commands { results = append(results, result{command.CheckCommand, command.Status, command.ExitCode}) }
	data, err := json.Marshal(struct { CheckID, FailedCommit string; Commands []result }{wave.CheckID, wave.Commit, results})
	if err != nil || len(data) > 65536 { return nil, errors.New("The recorded check diagnostic exceeds its bounded repair context.") }
	return data, nil
}

func appendCheckRepairContext(run ProjectRun, op ProjectRunOperation, capture *ReviewedContextCapture) (*ReviewedContextCapture, error) {
	if op.CheckRepair == nil { return capture, nil }
	if !swarmCheckRepairOperation(run, op) { return nil, ErrProjectRunConflict }
	wave := swarmCheckRepairWave(&run, op.CheckRepair.WaveID)
	data, err := checkRepairDiagnostic(*wave)
	if err != nil { return nil, err }
	capture.Files = append(capture.Files, ReviewedContextFile{Path: "check-failure.json", Data: data})
	for i := range capture.Files { if capture.Files[i].Path == "LUCI.md" { capture.Files[i].Data = append(capture.Files[i].Data, []byte("\n\n## Combined-check repair\n\nThis fresh coding attempt starts from the recorded failed combined source. Read check-failure.json for the exact declared commands and recorded exits. Skipped commands did not execute. Repair only this task's approved scope. A distinct reviewer must assess the new commit before integration and a fresh combined check; the earlier review/check cannot be reused.\n")...) } }
	return capture, nil
}
