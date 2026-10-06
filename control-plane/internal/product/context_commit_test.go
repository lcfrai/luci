package product

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestApprovedDefinitionAndDesignCanBeCommittedForDeployment(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "approved-context")
	base := "/v1/product/projects/" + p.ID
	spec := specificationFromBrief(Brief{ProjectID: p.ID, Purpose: "Keep saved work", Audience: "The owner"})
	spec.Requirements = []SpecificationRequirement{{ID: "AC-001", Title: "Recover work", Priority: "must", Acceptance: "Saved work returns after reopening"}}
	journeyRequest(t, mux, "POST", base+"/specification", map[string]any{"expectedRevision": 0, "expectedBriefRevision": 0, "specification": spec}, 200)
	journeyRequest(t, mux, "POST", base+"/specification/approve", map[string]int{"revision": 1, "briefRevision": 1}, 200)
	var design ProjectDesignPlan
	json.Unmarshal(journeyRequest(t, mux, "GET", base+"/design-plan", nil, 200).Body.Bytes(), &design)
	design.Screens = []DesignScreen{{ID: "screen-home", Name: "Home", Route: "/", Purpose: "Resume saved work", States: []string{"Empty", "Saved", "Error"}, Widths: []int{390, 1440}, RequirementIDs: []string{"AC-001"}}}
	journeyRequest(t, mux, "POST", base+"/design-plan", map[string]any{"expectedRevision": 0, "expectedBriefRevision": 1, "expectedSpecificationRevision": 2, "plan": design}, 200)
	journeyRequest(t, mux, "POST", base+"/design-plan/approve", map[string]int{"revision": 1}, 200)
	privateFile := filepath.Join(p.Path, ".luci", "credentials.json")
	if err := os.WriteFile(privateFile, []byte(`{"token":"private fixture"}`), 0600); err != nil {
		t.Fatal(err)
	}
	refused := journeyRequest(t, mux, "POST", base+"/git/commit", reviewedCommitRequest(t, mux, base+"/git/commit", "Must retain private context"), 409)
	var failure map[string]any
	if err := json.Unmarshal(refused.Body.Bytes(), &failure); err != nil {
		t.Fatal(err)
	}
	if failure["error"] != "sensitive_file" {
		t.Fatalf("Private files must remain excluded: %v", failure["error"])
	}
	if err := os.Remove(privateFile); err != nil {
		t.Fatal(err)
	}
	committed := journeyRequest(t, mux, "POST", base+"/git/commit", reviewedCommitRequest(t, mux, base+"/git/commit", "Approve solution context"), 200)
	var receipt struct {
		Commit string `json:"commit"`
	}
	if err := json.Unmarshal(committed.Body.Bytes(), &receipt); err != nil {
		t.Fatal(err)
	}
	if err := s.ValidateDeploymentSource(context.Background(), p.ID, receipt.Commit); err != nil {
		t.Fatalf("Approved source could not proceed to deployment: %v", err)
	}
	for _, file := range []string{".luci/specification.json", ".luci/specification.md", ".luci/design-handoff.json", ".luci/design-handoff.md"} {
		if _, err := s.journeyGit(context.Background(), p.Path, "show", receipt.Commit+":"+file); err != nil {
			t.Fatalf("Missing committed context %s: %v", file, err)
		}
	}
}
