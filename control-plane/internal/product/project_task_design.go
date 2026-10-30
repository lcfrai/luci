package product

import "errors"

// Frames are optional navigation into the existing approved handoff. They do
// not grant execution authority, replace its constraints, or approve a draft.
func validateTaskDesignFrameIDs(ids []string) error {
	if len(ids) > 100 {
		return errors.New("Link at most 100 approved design frames per task.")
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if !designIDPattern.MatchString(id) || seen[id] {
			return errors.New("Design frame links need unique stable frame IDs.")
		}
		seen[id] = true
	}
	return nil
}

func hasTaskDesignFrames(tasks []ProjectTask) bool {
	for _, task := range tasks {
		if len(task.DesignFrameIDs) > 0 {
			return true
		}
	}
	return false
}

func validateTaskDesignReferences(tasks []ProjectTask, design ProjectDesignPlan) error {
	if !hasTaskDesignFrames(tasks) {
		return nil
	}
	if design.Status != "approved" || design.Scene == nil || validateDesignPlan(design) != nil {
		return errors.New("Approve the current design handoff with its frames before linking tasks to screens.")
	}
	frames := map[string]bool{}
	for _, frame := range design.Scene.Frames {
		frames[frame.ID] = true
	}
	for _, task := range tasks {
		if err := validateTaskDesignFrameIDs(task.DesignFrameIDs); err != nil {
			return err
		}
		for _, id := range task.DesignFrameIDs {
			if !frames[id] {
				return errors.New("A linked design frame is unavailable. Refresh the approved design or remove the old frame link.")
			}
		}
	}
	return nil
}

// The save/apply caller holds the existing journey lock and obtains all frame
// text and approval digests from the authoritative private review state.
func bindTaskDesignFrames(state journeyState, tasks []ProjectTask, expected *int) (int, string, error) {
	if !hasTaskDesignFrames(tasks) {
		return 0, "", nil
	}
	_, design := currentEvidenceApprovals(state, state.Brief.ProjectID)
	if design.Status != "approved" {
		return 0, "", &evidenceBindingError{422, "design_approval_required", "Approve the current design handoff before linking its frames to implementation tasks."}
	}
	if expected == nil || *expected != design.Revision {
		return 0, "", &evidenceBindingError{409, "revision_conflict", "The approved design changed. Refresh its frames before saving this plan."}
	}
	if err := validateTaskDesignReferences(tasks, design); err != nil {
		return 0, "", &evidenceBindingError{400, "invalid_design_frame_link", err.Error()}
	}
	return design.Revision, design.Digest, nil
}

func validateBoundTaskDesignFrames(plan ProjectPlan, design ProjectDesignPlan) error {
	if !hasTaskDesignFrames(plan.Tasks) {
		return nil
	}
	if plan.ProjectID != design.ProjectID || plan.DesignPlanRevision != design.Revision || plan.DesignPlanDigest != design.Digest {
		return errors.New("The plan's linked design changed. Review the current approved frames and save the plan again before starting work.")
	}
	return validateTaskDesignReferences(plan.Tasks, design)
}
