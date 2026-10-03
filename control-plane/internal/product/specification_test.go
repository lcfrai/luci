package product

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStructuredDefinitionMigratesBriefAndPreservesStableHistory(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "definition-app")
	base := "/v1/product/projects/" + p.ID
	journeyRequest(t, mux, "POST", base+"/brief", map[string]any{"revision": 0, "purpose": "A useful app", "audience": "People", "constraints": "Offline", "criteria": []string{"Returns saved work"}}, 200)
	var spec ProjectSpecification
	json.Unmarshal(journeyRequest(t, mux, "GET", base+"/specification", nil, 200).Body.Bytes(), &spec)
	if spec.Revision != 0 || spec.BriefRevision != 1 || len(spec.Requirements) != 1 || spec.Requirements[0].ID != "AC-001" {
		t.Fatalf("legacy migration lost content: %#v", spec)
	}
	before, _ := os.ReadFile(filepath.Join(p.Path, ".luci", "state.json"))
	journeyRequest(t, mux, "GET", base+"/specification/history", nil, 200)
	after, _ := os.ReadFile(filepath.Join(p.Path, ".luci", "state.json"))
	if string(before) != string(after) {
		t.Fatal("definition reads changed persisted state")
	}
	spec.Roles = []SpecificationRole{{ID: "role-owner", Name: "Owner", CanDo: "Edit own work", CannotDo: "Read another account"}}
	spec.Glossary = []SpecificationTerm{{ID: "term-work", Term: "Work", Meaning: "The person's saved project"}}
	spec.Decisions = []SpecificationDecision{{ID: "decision-local", Title: "Local first", Decision: "Store on the person's computer", Reason: "Offline access", Alternatives: "Hosted service"}}
	body := map[string]any{"expectedRevision": 0, "expectedBriefRevision": 1, "specification": spec}
	json.Unmarshal(journeyRequest(t, mux, "POST", base+"/specification", body, 200).Body.Bytes(), &spec)
	journeyRequest(t, mux, "POST", base+"/specification", body, 409)
	if spec.Revision != 1 || spec.BriefRevision != 2 || spec.Requirements[0].ID != "AC-001" {
		t.Fatal("save lost provenance or stable ID")
	}
	md, err := os.ReadFile(filepath.Join(p.Path, ".luci", "specification.md"))
	if err != nil || !strings.Contains(string(md), "AC-001") || !strings.Contains(string(md), "Local first") || !strings.Contains(string(md), "another account") {
		t.Fatalf("full agent context not exported: %v %s", err, md)
	}
	json.Unmarshal(journeyRequest(t, mux, "POST", base+"/specification/approve", map[string]int{"revision": 1, "briefRevision": 2}, 200).Body.Bytes(), &spec)
	if spec.Status != "approved" || spec.Revision != 2 || len(spec.Digest) != 64 {
		t.Fatal("approval is not a bound snapshot")
	}
	journeyRequest(t, mux, "POST", base+"/brief", map[string]any{"revision": 2, "purpose": "Changed purpose", "audience": "People", "constraints": "Offline", "criteria": []string{"Returns saved work", "Can export data"}}, 200)
	var stale ProjectSpecification
	json.Unmarshal(journeyRequest(t, mux, "GET", base+"/specification", nil, 200).Body.Bytes(), &stale)
	if stale.Status != "outdated" || stale.BriefRevision != 3 || stale.Purpose != "Changed purpose" || stale.Requirements[0].ID != "AC-001" {
		t.Fatalf("superseded approval remained current: %#v", stale)
	}
	journeyRequest(t, mux, "POST", base+"/specification/approve", map[string]int{"revision": 2, "briefRevision": 3}, 409)
	state, err := readJourney(p.Path, p.ID)
	if err != nil || len(state.SpecificationRevisions) != 2 || state.SpecificationRevisions[1].Status != "approved" || state.SpecificationRevisions[1].Purpose != "A useful app" {
		t.Fatal("immutable history changed", err)
	}
	stale.Digest = "forged"
	json.Unmarshal(journeyRequest(t, mux, "POST", base+"/specification", map[string]any{"expectedRevision": 2, "expectedBriefRevision": 3, "specification": stale}, 200).Body.Bytes(), &stale)
	if stale.Status != "draft" || stale.Digest != "" {
		t.Fatal("client forged an approval")
	}
	_ = s
}

func TestDefinitionRejectsBrokenLinksAndUnresolvedApproval(t *testing.T) {
	_, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "definition-validation")
	base := "/v1/product/projects/" + p.ID
	spec := specificationFromBrief(Brief{ProjectID: p.ID, Purpose: "Build", Audience: "People"})
	spec.Requirements = []SpecificationRequirement{{ID: "AC-001", Title: "Save", Priority: "must", Acceptance: ""}}
	spec.Journeys = []SpecificationJourney{{ID: "journey-save", Name: "Save", RequirementIDs: []string{"missing"}}}
	body := map[string]any{"expectedRevision": 0, "expectedBriefRevision": 0, "specification": spec}
	journeyRequest(t, mux, "POST", base+"/specification", body, 400)
	spec.Journeys[0].RequirementIDs = []string{"AC-001"}
	body["specification"] = spec
	journeyRequest(t, mux, "POST", base+"/specification", body, 200)
	journeyRequest(t, mux, "POST", base+"/specification/approve", map[string]int{"revision": 1, "briefRevision": 1}, 422)
	spec.Revision = 1
	spec.BriefRevision = 1
	spec.Requirements[0].Acceptance = "Given saved data, when reopened, the same work returns"
	spec.Questions = []SpecificationQuestion{{ID: "question-storage", Question: "Where does data live?", Answer: ""}}
	body["expectedRevision"] = 1
	body["expectedBriefRevision"] = 1
	body["specification"] = spec
	journeyRequest(t, mux, "POST", base+"/specification", body, 200)
	journeyRequest(t, mux, "POST", base+"/specification/approve", map[string]int{"revision": 2, "briefRevision": 2}, 422)
}
