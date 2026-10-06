package product

import (
	"encoding/json"
	"sort"
)

const orchestratorRunScopeLimit = 64 << 10

// These summaries describe durable receipts, not a live provider probe. Reads
// never reconcile children or silently infer that cancellation stopped them.
func orchestratorRunSummary(run ProjectRun) OrchestratorRunSummary {
	summary := OrchestratorRunSummary{ID: run.ID, State: run.State, Reason: run.Reason, Activity: "none", DispatchCancelled: run.State == "cancelled", Operations: []OrchestratorRunOperation{}, Tasks: []OrchestratorRunTask{}, Requirements: append([]RequirementSnapshot{}, run.Input.Requirements...), SpecificationRevision: run.Input.Implementation.SpecificationRevision, DesignRevision: run.Input.Implementation.DesignPlanRevision, Outcome: run.Input.Outcome}
	summary.TaskCount = len(run.Input.Implementation.TaskIDs)
	summary.RequirementCount = len(run.Input.Requirements)
	for i, id := range run.Input.Implementation.TaskIDs {
		title := id
		if i < len(run.Input.Implementation.TaskTitles) {
			title = run.Input.Implementation.TaskTitles[i]
		}
		summary.Tasks = append(summary.Tasks, OrchestratorRunTask{ID: id, Title: title, Status: "selected"})
	}
	if run.Observation != nil {
		summary.Valid = run.Observation.Valid
		if !summary.DispatchCancelled {
			summary.Reason = run.Observation.Reason
		}
	}
	for _, op := range run.Operations {
		activity := ""
		switch op.Status {
		case "running":
			activity = "running"
		case "dispatching", "unknown":
			activity = "unknown"
		case "reserved":
			activity = "pending"
		}
		if activity == "" {
			continue
		}
		summary.Operations = append(summary.Operations, OrchestratorRunOperation{ID: op.ID, ChildID: op.ChildID, Status: op.Status, UpdatedAt: op.UpdatedAt})
		if orchestratorActivityPriority(activity) < orchestratorActivityPriority(summary.Activity) {
			summary.Activity = activity
		}
	}
	summary.OperationCount = len(summary.Operations)
	switch summary.Activity {
	case "running":
		summary.ActivityReason = "A coding child was last recorded as running. Inspect its session and reconcile to observe the current outcome."
	case "unknown":
		summary.ActivityReason = "A coding child was dispatched, but its current outcome is unconfirmed. Reconcile the existing child before starting another attempt."
	case "pending":
		summary.ActivityReason = "A coding operation is reserved and has not been dispatched. Review that reservation before continuing."
	default:
		summary.ActivityReason = "No unfinished coding operation is recorded for this run. Recorded outcomes still require independent verification."
	}
	if summary.DispatchCancelled && summary.Activity != "none" {
		summary.ActivityReason += " Future dispatch is cancelled; this does not stop an already allocated child. Use its explicit stop control when needed."
	}
	return summary
}
func orchestratorActivityPriority(activity string) int {
	switch activity {
	case "running":
		return 0
	case "unknown":
		return 1
	case "pending":
		return 2
	default:
		return 3
	}
}

// ProjectRunStore.List is newest-created first. Preserve that useful latest
// plan separately from every still-unfinished immutable execution scope.
func orchestratorRunProjection(runs []ProjectRun) (*OrchestratorRunSummary, []OrchestratorRunSummary) {
	var latest *OrchestratorRunSummary
	active := []OrchestratorRunSummary{}
	remaining := orchestratorRunScopeLimit
	// Prioritize the exact scope of unfinished children over a newer plan. The
	// remaining summaries keep every identity and explicit scope counts, never
	// silently substitute the mutable current plan for omitted immutable text.
	summaries := make([]OrchestratorRunSummary, len(runs))
	for i, run := range runs {
		summaries[i] = orchestratorRunSummary(run)
	}
	order := make([]int, len(summaries))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool {
		return orchestratorActivityPriority(summaries[order[i]].Activity) < orchestratorActivityPriority(summaries[order[j]].Activity)
	})
	for _, i := range order {
		summary := &summaries[i]
		if i != 0 && summary.Activity == "none" {
			continue
		}
		encoded, _ := json.MarshalIndent(summary, "", "  ")
		// Count the complete detail row, including reasons and child identities;
		// those can be large even when the selected task list is small.
		bytes := len(encoded)
		if i == 0 && summary.Activity != "none" {
			bytes *= 2
		}
		if bytes > remaining {
			summary.ScopeOmitted = summary.TaskCount > 0 || summary.RequirementCount > 0
			summary.Tasks = []OrchestratorRunTask{}
			summary.Requirements = []RequirementSnapshot{}
			summary.OperationsOmitted = summary.OperationCount > 0
			summary.Operations = []OrchestratorRunOperation{}
			summary.Reason = ""
			summary.ActivityReason = "Last recorded activity. Inspect this run for its full saved scope and reconcile its child outcome."
		} else {
			remaining -= bytes
		}

		if i == 0 {
			copy := *summary
			latest = &copy
		}
		if summary.Activity != "none" {
			active = append(active, *summary)
		}
	}
	return latest, active
}
