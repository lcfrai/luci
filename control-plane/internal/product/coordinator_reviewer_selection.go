package product

import (
	"errors"
	"sort"
)

var errCoordinatorReviewerSelection = errors.New("More than one selected reviewer targets the same coding task. Choose one independent reviewer for each coding task in the saved plan or task selection before continuing.")

// ReviewOfTaskID is a review edge, not DependsOn. The current owned-obligation
// contract permits one reviewer per candidate. Preserve explicit selections;
// otherwise choose the first saved reviewer in plan order. The final sorted
// set still needs its existing exact current execution-contract approval.
func coordinatorIncludeSelectedReviewers(tasks []ProjectTask, selectedIDs []string) ([]string, error) {
	if len(tasks) > 200 || len(selectedIDs) > 200 || validateProjectTaskGraph(tasks) != nil {
		return nil, ErrProjectRunInvalid
	}
	byID := map[string]ProjectTask{}
	for _, task := range tasks {
		if !designIDPattern.MatchString(task.ID) {
			return nil, ErrProjectRunInvalid
		}
		if _, exists := byID[task.ID]; exists {
			return nil, ErrProjectRunInvalid
		}
		byID[task.ID] = task
	}
	selected := map[string]bool{}
	reviewers := map[string]string{}
	for _, id := range selectedIDs {
		task, known := byID[id]
		if !known || selected[id] {
			return nil, ErrProjectRunInvalid
		}
		selected[id] = true
		if task.Role == "reviewer" {
			if reviewers[task.ReviewOfTaskID] != "" {
				return nil, errCoordinatorReviewerSelection
			}
			reviewers[task.ReviewOfTaskID] = id
		}
	}
	for _, candidate := range tasks {
		if !selected[candidate.ID] || candidate.Role == "reviewer" || reviewers[candidate.ID] != "" {
			continue
		}
		for _, reviewer := range tasks {
			if reviewer.Role == "reviewer" && reviewer.ReviewOfTaskID == candidate.ID {
				selected[reviewer.ID] = true
				reviewers[candidate.ID] = reviewer.ID
				break
			}
		}
	}
	out := make([]string, 0, len(selected))
	for id := range selected {
		out = append(out, id)
	}
	sort.Strings(out)
	return out, nil
}
