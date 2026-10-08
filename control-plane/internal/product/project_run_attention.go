package product

// ProjectRunAttention describes a recorded review obligation. It grants no
// dispatch, integration, verification, or capacity authority. Compact identities
// let the orchestrator retain every obligation within the 200-run history cap.
type ProjectRunAttention struct {
	ID            string `json:"id"`
	State         string `json:"state"`
	Kind          string `json:"kind"`
	ContextStatus string `json:"contextStatus"`
}

func projectRunAttentionKind(run ProjectRun) string {
	// A reported question survives integration, a newer attempt and acceptance.
	// It invites review but cannot grant authority or block unrelated dispatch.
	for _, op := range run.Operations {
		if op.TaskID != "" && op.Handoff != nil && op.Handoff.Status == "captured" && op.Handoff.Unanswered > 0 {
			return "task-question"
		}
	}
	// Accepted work stays in history. Cancelling future dispatch is different:
	// retained task output below still needs an explicit review/retirement.
	if run.State == "complete" {
		return ""
	}
	if run.Swarm != nil {
		for _, task := range run.Swarm.Tasks {
			if task.Integration != nil && task.Integration.State == "integrated" {
				continue
			}
			op := swarmOperation(&run, task.OperationID)
			if op != nil && (op.Status == "review-required" || op.Status == "succeeded" || op.Status == "failed" && op.Allocation == "ready") {
				return "task-review"
			}
		}
	}
	if run.State == "cancelled" {
		return ""
	}
	switch run.State {
	case "awaiting-checks":
		return "verification"
	case "awaiting-release":
		return "release-review"
	case "blocked":
		return "blocked"
	default:
		return ""
	}
}

func projectRunAttentionProjection(runs []ProjectRun, state journeyState) []ProjectRunAttention {
	result := []ProjectRunAttention{}
	for _, run := range runs {
		if kind := projectRunAttentionKind(run); kind != "" {
			freshness := projectRunContextFreshness(run, state, nil)
			result = append(result, ProjectRunAttention{ID: run.ID, State: run.State, Kind: kind, ContextStatus: freshness.Status})
		}
	}
	return result
}

func projectRunAttentionAction(run ProjectRunAttention) OrchestratorAction {
	action := OrchestratorAction{ID: "inspect-run", RunID: run.ID, Lens: "develop", Label: "Review earlier build", Reason: "This earlier build has a recorded unresolved step. Inspect its saved scope and evidence; a newer run does not resolve it."}
	switch run.Kind {
	case "task-question":
		action.Label = "Review reported task questions"
		action.Reason = "A retained worker report contains unanswered questions. Inspect the exact attempt and its evidence; a worker's assessment does not change approved scope or execution authority."
	case "task-review":
		action.Label = "Review earlier task changes"
	case "verification":
		action.Label = "Review earlier verification"
	case "release-review":
		action.Label = "Review earlier release"
	case "blocked":
		action.Label = "Review earlier blocked work"
	}
	if run.ContextStatus != "current" {
		action.Reason = "This earlier build still needs review, and its saved context differs from the current project or cannot be compared. Inspect the original scope before deciding what to retain, amend, or cancel."
	}
	return action
}
