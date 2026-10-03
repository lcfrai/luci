package product

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesignHandoffLinksRequirementsAndBecomesOutdated(t *testing.T) {
	_, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "design-handoff")
	base := "/v1/product/projects/" + p.ID
	spec := specificationFromBrief(Brief{ProjectID: p.ID, Purpose: "Saved work", Audience: "People"})
	spec.Requirements = []SpecificationRequirement{{ID: "AC-001", Title: "Save work", Priority: "must", Acceptance: "Saved work survives reopening"}}
	journeyRequest(t, mux, "POST", base+"/specification", map[string]any{"expectedRevision": 0, "expectedBriefRevision": 0, "specification": spec}, 200)
	journeyRequest(t, mux, "POST", base+"/specification/approve", map[string]int{"revision": 1, "briefRevision": 1}, 200)
	var plan ProjectDesignPlan
	json.Unmarshal(journeyRequest(t, mux, "GET", base+"/design-plan", nil, 200).Body.Bytes(), &plan)
	if plan.Revision != 0 || plan.SpecificationRevision != 2 || plan.Screens == nil {
		t.Fatal("new design context is not bound")
	}
	plan.Screens = []DesignScreen{{ID: "screen-home", Name: "Home", Route: "/", Purpose: "Resume work", States: []string{"No saved work", "Loading", "Saved work", "Error and retry"}, Widths: []int{390, 1440}, RequirementIDs: []string{"missing"}}}
	body := map[string]any{"expectedRevision": 0, "expectedBriefRevision": 1, "expectedSpecificationRevision": 2, "plan": plan}
	journeyRequest(t, mux, "POST", base+"/design-plan", body, 400)
	plan.Screens[0].RequirementIDs = []string{"AC-001"}
	body["plan"] = plan
	json.Unmarshal(journeyRequest(t, mux, "POST", base+"/design-plan", body, 200).Body.Bytes(), &plan)
	journeyRequest(t, mux, "POST", base+"/design-plan", body, 409)
	json.Unmarshal(journeyRequest(t, mux, "POST", base+"/design-plan/approve", map[string]int{"revision": 1}, 200).Body.Bytes(), &plan)
	if plan.Status != "approved" || plan.Revision != 2 || len(plan.Digest) != 64 {
		t.Fatal("approval is not a snapshot")
	}
	text, err := os.ReadFile(filepath.Join(p.Path, ".luci", "design-handoff.md"))
	if err != nil || !strings.Contains(string(text), "AC-001") || !strings.Contains(string(text), "Error and retry") {
		t.Fatal("handoff is not in agent context", err)
	}
	journeyRequest(t, mux, "POST", base+"/design", map[string]any{"revision": 0, "frames": []Frame{}}, 200)
	json.Unmarshal(journeyRequest(t, mux, "GET", base+"/design-plan", nil, 200).Body.Bytes(), &plan)
	if plan.Status != "outdated" {
		t.Fatal("changed sketch left design approved")
	}
	journeyRequest(t, mux, "POST", base+"/design-plan/approve", map[string]int{"revision": 2}, 409)
}

func TestPlatformPathsAreExecutionWorkspaceRequests(t *testing.T) {
	for _, path := range []string{"/v1/platform/status", "/v1/platform/projects/p-1", "/v1/platform/projects/p-1/database"} {
		if !projectRequest(path) {
			t.Fatalf("%s could act on the local machine while cloud is selected", path)
		}
	}
}
