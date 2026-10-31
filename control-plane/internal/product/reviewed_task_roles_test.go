package product

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestReviewedTaskBriefCarriesExactReviewerRoleAndTarget(t *testing.T) {
	state, binding := taskBriefFixture()
	reviewer := ProjectTask{ID: "review-save", Title: "Review saved work", Status: "todo", Role: "reviewer", ReviewOfTaskID: "save", Acceptance: "Inspect the exact candidate without modifying repository files.", RequirementIDs: []string{"AC-save"}}
	state.Plan.Tasks = append(state.Plan.Tasks, reviewer)
	binding.PlanDigest = implementationDigest(currentPlan(state, state.Brief.ProjectID))
	binding.TaskIDs = []string{reviewer.ID}
	binding.TaskTitles = []string{reviewer.Title}
	before, _ := json.Marshal(state)
	raw, err := renderReviewedTaskBrief(state, binding)
	if err != nil {
		t.Fatal(err)
	}
	brief := parsedTaskBrief(t, raw)
	if len(brief.SelectedTasks) != 1 || brief.SelectedTasks[0].Role != "reviewer" || brief.SelectedTasks[0].ReviewOfTaskID != "save" || len(brief.SelectedTasks[0].WritePaths) != 0 || len(brief.Dependencies) != 0 {
		t.Fatal("review role broadened assignment or became a source dependency", brief)
	}
	if !strings.Contains(string(raw), "Empty reviewer writePaths never authorizes coding") {
		t.Fatal("review role lost its no-edit distinction")
	}
	after, _ := json.Marshal(state)
	if string(before) != string(after) {
		t.Fatal("brief mutated captured plan")
	}
}
func TestReviewedTaskBriefLegacyRoleFieldsStayOmitted(t *testing.T) {
	raw, err := json.Marshal(reviewedAssignedTask{ID: "save", Title: "Save", RecordedStatus: "todo"})
	if err != nil {
		t.Fatal(err)
	}
	expected := `{"id":"save","title":"Save","recordedStatus":"todo","dependsOn":null,"writePaths":null,"acceptance":"","requirementIds":null}`
	if string(raw) != expected {
		t.Fatal("legacy selected-task bytes changed", string(raw))
	}
	state, binding := taskBriefFixture()
	data, err := renderReviewedTaskBrief(state, binding)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "Captured task roles are exact saved assignments") || strings.Contains(string(data), `"reviewOfTaskId"`) {
		t.Fatal("legacy role omission gained new role metadata or guidance")
	}
}
