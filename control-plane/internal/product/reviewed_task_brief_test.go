package product

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/registry"
)

func taskBriefFixture() (journeyState, *registry.ImplementationContext) {
	state := forgedReviewState("p-task-brief")
	state.Plan.Tasks = []ProjectTask{
		{ID: "schema", Title: "Define shared data", Status: "done", WritePaths: []string{"src/schema.ts"}},
		{ID: "save", Title: "Save work", Status: "todo", RequirementIDs: []string{"AC-save"}, DependsOn: []string{"schema"}, WritePaths: []string{"src/save.ts"}, Acceptance: "Save succeeds and a failed write retains the user's draft."},
		{ID: "list", Title: "List saved work", Status: "todo", RequirementIDs: []string{"AC-save"}, DependsOn: []string{"schema"}, WritePaths: []string{"src/list.ts"}, Acceptance: "Reload shows the same saved items."},
		{ID: "billing", Title: "Unassigned billing work", Status: "todo", Acceptance: "This is not this worker's assignment."},
	}
	state.Specification.Requirements[0].Acceptance = "Exact contents return, including Unicode: 日本語.\nA failed save keeps the draft.\n``` is literal project text."
	state.Specification.Requirements = append(state.Specification.Requirements, SpecificationRequirement{ID: "AC-billing", Title: "Billing", Acceptance: "Unassigned requirement"})
	// This pure capture fixture supplies a private in-memory review selection.
	// API regressions exercise actual authenticated review endpoints separately.
	state.Specification.Digest = specificationContentDigest(*state.Specification)
	state.Plan.SpecificationDigest = state.Specification.Digest
	state.approvalAuthority = &journeyApprovalStore{ProjectID: state.Brief.ProjectID,
		Specification: &journeyApprovalReceipt{ID: "unit-spec-review", Revision: state.Specification.Revision, ContentDigest: state.Specification.Digest, BriefRevision: state.Brief.Revision, BriefDigest: reviewDigest(state.Brief)},
		Design: &journeyApprovalReceipt{Revision: state.DesignPlan.Revision, ContentDigest: state.DesignPlan.Digest, BriefRevision: state.Brief.Revision, BriefDigest: reviewDigest(state.Brief),
			SketchRevision: state.Design.Revision, SketchDigest: reviewDigest(state.Design), SpecificationRevision: state.Specification.Revision, SpecificationDigest: state.Specification.Digest, SpecificationApproval: "unit-spec-review"},
	}
	binding := &registry.ImplementationContext{
		ProjectID: state.Brief.ProjectID, BriefRevision: state.Brief.Revision, BriefDigest: implementationDigest(state.Brief), PlanRevision: state.Plan.Revision, PlanDigest: implementationDigest(currentPlan(state, state.Brief.ProjectID)),
		SpecificationRevision: state.Specification.Revision, SpecificationDigest: state.Specification.Digest,
		DesignPlanRevision: state.DesignPlan.Revision, DesignPlanDigest: state.DesignPlan.Digest,
		TaskIDs: []string{"save", "list"}, TaskTitles: []string{"Save work", "List saved work"}, RequirementIDs: []string{"AC-save"},
	}
	return state, binding
}

func parsedTaskBrief(t *testing.T, data []byte) reviewedTaskBrief {
	t.Helper()
	start := bytes.Index(data, []byte("\n```json\n"))
	if start < 0 {
		t.Fatal("captured brief has no task contract")
	}
	data = data[start+len("\n```json\n"):]
	end := bytes.Index(data, []byte("\n```\n"))
	if end < 0 {
		t.Fatal("captured task contract is incomplete")
	}
	var brief reviewedTaskBrief
	if err := json.Unmarshal(data[:end], &brief); err != nil {
		t.Fatal(err)
	}
	return brief
}

func TestReviewedTaskBriefKeepsAssignmentNarrowAndAcceptanceExact(t *testing.T) {
	state, binding := taskBriefFixture()
	before, _ := json.Marshal(state)
	capture, err := captureReviewedContext(state, binding)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for _, file := range capture.Files {
		files[file.Path] = file.Data
	}
	if len(files) != 8 || !bytes.HasPrefix(files["LUCI.md"], renderBrief(state.Brief)) {
		t.Fatal("task contract changed the original brief or the existing capture inventory")
	}
	brief := parsedTaskBrief(t, files["LUCI.md"])
	if len(brief.SelectedTasks) != 2 || brief.SelectedTasks[0].ID != "save" || brief.SelectedTasks[1].ID != "list" ||
		brief.SelectedTasks[0].Acceptance != state.Plan.Tasks[1].Acceptance || !reflect.DeepEqual(brief.SelectedTasks[0].WritePaths, state.Plan.Tasks[1].WritePaths) {
		t.Fatal("captured task selection, acceptance or write scope changed", brief)
	}
	if len(brief.Dependencies) != 1 || brief.Dependencies[0].ID != "schema" || brief.Dependencies[0].RecordedStatus != "done" || brief.Dependencies[0].AlsoAssigned {
		t.Fatal("shared prerequisite was duplicated, assigned or lost", brief.Dependencies)
	}
	if len(brief.LinkedRequirements) != 1 || brief.LinkedRequirements[0].Acceptance != state.Specification.Requirements[0].Acceptance {
		t.Fatal("linked acceptance text was truncated, substituted or duplicated", brief.LinkedRequirements)
	}
	if !bytes.Contains(files["LUCI.md"], []byte("not current process status or evidence of successful checks or integration")) ||
		!bytes.Contains(files["LUCI.md"], []byte("does not replace those documents or waive unlisted project constraints")) {
		t.Fatal("contract hides its planning/evidence distinction or full-context obligation")
	}
	exports, err := journeyContextExports(state)
	if err != nil {
		t.Fatal(err)
	}
	for _, export := range exports {
		if export.Path == "LUCI.md" {
			if bytes.Contains(export.Data, []byte("## Assigned task contract")) {
				t.Fatal("session-specific contract leaked into the public project export")
			}
		} else if !bytes.Equal(export.Data, files[export.Path]) {
			t.Fatalf("full captured %s was filtered or changed", export.Path)
		}
	}
	after, _ := json.Marshal(state)
	if !bytes.Equal(before, after) {
		t.Fatal("capturing a task contract changed canonical state")
	}
	frozen := append([]byte{}, files["LUCI.md"]...)
	state.Plan.Tasks[1].Title = "Teammate's later amendment"
	state.Specification.Requirements[0].Acceptance = "Later acceptance"
	if !bytes.Equal(frozen, files["LUCI.md"]) || bytes.Contains(files["LUCI.md"], []byte("Later acceptance")) {
		t.Fatal("later canonical context changed the retained task contract")
	}
}

func TestReviewedTaskBriefRefusesMissingOrMismatchedInputs(t *testing.T) {
	for _, scenario := range []struct {
		name string
		edit func(*journeyState, *registry.ImplementationContext)
	}{
		{"foreign project", func(_ *journeyState, c *registry.ImplementationContext) { c.ProjectID = "p-foreign" }},
		{"stale plan", func(s *journeyState, _ *registry.ImplementationContext) {
			s.Plan.Tasks[1].Acceptance = "Amended acceptance"
		}},
		{"stale definition", func(s *journeyState, _ *registry.ImplementationContext) { s.Specification.Revision++ }},
		{"same revision definition amendment", func(s *journeyState, _ *registry.ImplementationContext) {
			s.Specification.Requirements[0].Acceptance = "Later requirements"
		}},
		{"same revision design amendment", func(s *journeyState, _ *registry.ImplementationContext) {
			s.DesignPlan.Screens[0].Purpose = "Later design"
		}},
		{"same revision brief amendment", func(s *journeyState, _ *registry.ImplementationContext) { s.Brief.Purpose = "Later brief" }},
		{"different selected design digest", func(_ *journeyState, c *registry.ImplementationContext) { c.DesignPlanDigest = strings.Repeat("f", 64) }},
		{"missing plan", func(s *journeyState, _ *registry.ImplementationContext) { s.Plan = nil }},
		{"missing definition", func(s *journeyState, _ *registry.ImplementationContext) { s.Specification = nil }},
		{"missing task", func(_ *journeyState, c *registry.ImplementationContext) { c.TaskIDs[0] = "removed" }},
		{"mismatched title", func(_ *journeyState, c *registry.ImplementationContext) { c.TaskTitles[0] = "Different scope" }},
		{"duplicate task", func(_ *journeyState, c *registry.ImplementationContext) { c.TaskIDs[1] = c.TaskIDs[0] }},
		{"missing dependency", func(s *journeyState, c *registry.ImplementationContext) {
			s.Plan.Tasks[1].DependsOn = []string{"removed"}
			c.PlanDigest = implementationDigest(currentPlan(*s, c.ProjectID))
		}},
		{"missing requirement", func(s *journeyState, c *registry.ImplementationContext) {
			s.Plan.Tasks[1].RequirementIDs = []string{"removed"}
			c.PlanDigest = implementationDigest(currentPlan(*s, c.ProjectID))
		}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			state, binding := taskBriefFixture()
			scenario.edit(&state, binding)
			if capture, err := captureReviewedContext(state, binding); err == nil || capture != nil {
				t.Fatal("incomplete contract was silently accepted", err)
			}
		})
	}
	state, _ := taskBriefFixture()
	if capture, err := captureReviewedContext(state, nil); err == nil || capture != nil {
		t.Fatal("missing selection was silently accepted")
	}
}

func TestReviewedTaskBriefRetainsLargeSelectionWithoutSilentTruncation(t *testing.T) {
	state, binding := taskBriefFixture()
	state.Plan.Tasks = []ProjectTask{}
	binding.TaskIDs, binding.TaskTitles = []string{}, []string{}
	for i := range 200 {
		task := ProjectTask{ID: fmt.Sprintf("task-%03d", i), Title: strings.Repeat("界", 666), Status: "todo", Acceptance: strings.Repeat("a\n", 2000), RequirementIDs: []string{"AC-save"}}
		for j := range 32 {
			task.WritePaths = append(task.WritePaths, fmt.Sprintf("src/%03d/%02d/%s.ts", i, j, strings.Repeat("x", 200)))
		}
		state.Plan.Tasks = append(state.Plan.Tasks, task)
		binding.TaskIDs = append(binding.TaskIDs, task.ID)
		binding.TaskTitles = append(binding.TaskTitles, task.Title)
	}
	if err := validatePlan(*state.Plan); err != nil {
		t.Fatal("large acceptance fixture is outside the supported plan contract", err)
	}
	binding.PlanDigest = implementationDigest(currentPlan(state, binding.ProjectID))
	capture, err := captureReviewedContext(state, binding)
	if err != nil {
		t.Fatal(err)
	}
	total := 0
	for _, file := range capture.Files {
		total += len(file.Data)
		if file.Path != "LUCI.md" {
			continue
		}
		brief := parsedTaskBrief(t, file.Data)
		if len(brief.SelectedTasks) != 200 || len(brief.LinkedRequirements) != 1 || brief.SelectedTasks[199].Acceptance != state.Plan.Tasks[199].Acceptance || len(brief.SelectedTasks[199].WritePaths) != 32 {
			t.Fatal("large selected task contract was silently shortened")
		}
	}
	if total >= ReviewedContextBytesLimit {
		t.Fatal("supported large selection exceeds delivery headroom", total)
	}
	t.Logf("200 selected tasks, 6,400 write paths, exact acceptance: %d captured bytes", total)
}
