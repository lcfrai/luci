package product

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func reviewerClosureTasks() []ProjectTask {
	return []ProjectTask{
		{ID: "task-a", Title: "Shared code", Status: "todo", WritePaths: []string{"alpha.txt"}, RequirementIDs: []string{"AC-save"}},
		{ID: "task-b", Title: "Unrelated code", Status: "todo", WritePaths: []string{"beta.txt"}, RequirementIDs: []string{"AC-save"}},
		{ID: "task-c", Title: "Use shared code", Status: "todo", DependsOn: []string{"task-a"}, WritePaths: []string{"gamma.txt"}, RequirementIDs: []string{"AC-save"}},
		{ID: "review-a", Title: "Review shared code", Status: "todo", Role: "reviewer", ReviewOfTaskID: "task-a", Acceptance: "Inspect the exact stopped shared candidate and report defects."},
		{ID: "review-b", Title: "Review unrelated code", Status: "todo", Role: "reviewer", ReviewOfTaskID: "task-b", Acceptance: "Inspect the exact stopped unrelated candidate and report defects."},
		{ID: "review-c", Title: "Review dependent code", Status: "todo", Role: "reviewer", ReviewOfTaskID: "task-c", Acceptance: "Inspect the exact stopped dependent candidate and report defects."},
	}
}
func TestCoordinatorAffectedSelectionIncludesOnlySavedReviewersOfAffectedCoding(t *testing.T) {
	tasks := reviewerClosureTasks()
	before, _ := json.Marshal(tasks)
	events := []CoordinationEventObservation{{Intent: CoordinationIntent{TaskIDs: []string{"task-a"}}}}
	got := coordinatorAffectedTaskIDs(tasks, events)
	want := []string{"review-a", "review-c", "task-a", "task-c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatal("affected coding omitted required reviewers or included unrelated work", got)
	}
	after, _ := json.Marshal(tasks)
	if string(before) != string(after) {
		t.Fatal("selection changed saved tasks")
	}
	only, err := coordinatorIncludeSelectedReviewers(tasks, []string{"review-a"})
	if err != nil || !reflect.DeepEqual(only, []string{"review-a"}) {
		t.Fatal("reviewer-only selection silently rebuilt coding", only, err)
	}
	legacy := coordinatorAffectedTaskIDs(tasks[:3], events)
	if !reflect.DeepEqual(legacy, []string{"task-a", "task-c"}) {
		t.Fatal("legacy no-reviewer ordering changed", legacy)
	}
}
func TestCoordinatorReviewerClosureRejectsUnknownRolesAndCycles(t *testing.T) {
	for _, kind := range []string{"unknown selected task", "reviewer self reference", "reviewer targets reviewer", "coding dependency cycle"} {
		t.Run(kind, func(t *testing.T) {
			tasks := reviewerClosureTasks()
			selected := []string{"task-a"}
			switch kind {
			case "unknown selected task":
				selected = []string{"missing"}
			case "reviewer self reference":
				tasks[3].ReviewOfTaskID = "review-a"
			case "reviewer targets reviewer":
				tasks[3].ReviewOfTaskID = "review-c"
			case "coding dependency cycle":
				tasks[0].DependsOn = []string{"task-c"}
			}
			if got, err := coordinatorIncludeSelectedReviewers(tasks, selected); err == nil || got != nil {
				t.Fatal("invalid closed selection accepted", got, err)
			}
		})
	}
}
func TestCoordinatorRebuiltPrerequisiteIncludesItsSavedReviewer(t *testing.T) {
	tasks := reviewerClosureTasks()
	s, _, p, _, _, _ := swarmHTTPFixture(t, tasks...)
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.coordinatorExpandAffectedPrerequisites(context.Background(), p, state, []string{"task-c"})
	if err != nil || !reflect.DeepEqual(got, []string{"review-a", "review-c", "task-a", "task-c"}) {
		t.Fatal("newly rebuilt prerequisite lost its independent review or selected unrelated work", got, err)
	}
}

func TestCoordinatorReviewerChoicePreservesExplicitSelectionOrSavedOrder(t *testing.T) {
	tasks := reviewerClosureTasks()
	tasks = append(tasks, ProjectTask{ID: "review-a-second", Title: "Alternative reviewer", Status: "todo", Role: "reviewer", ReviewOfTaskID: "task-a", Acceptance: "Inspect the exact shared candidate independently."})
	first, err := coordinatorIncludeSelectedReviewers(tasks, []string{"task-a"})
	if err != nil || !reflect.DeepEqual(first, []string{"review-a", "task-a"}) {
		t.Fatal("default reviewer did not follow saved plan order", first, err)
	}
	explicit, err := coordinatorIncludeSelectedReviewers(tasks, []string{"task-a", "review-a-second"})
	if err != nil || !reflect.DeepEqual(explicit, []string{"review-a-second", "task-a"}) {
		t.Fatal("explicit reviewer was silently replaced", explicit, err)
	}
	both := []string{"task-a", "review-a", "review-a-second"}
	if got, err := coordinatorIncludeSelectedReviewers(tasks, both); !errors.Is(err, errCoordinatorReviewerSelection) || got != nil {
		t.Fatal("ambiguous reviewer set was silently dropped or admitted", got, err)
	}
	events := []CoordinationEventObservation{{Intent: CoordinationIntent{TaskIDs: both}}}
	if _, err := coordinatorAffectedTaskSelection(tasks, events); !errors.Is(err, errCoordinatorReviewerSelection) {
		t.Fatal("impact path lost precise review-required reason", err)
	}
}
