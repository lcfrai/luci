package product

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func roadmapSpecification(projectID string) ProjectSpecification {
	spec := specificationFromBrief(Brief{ProjectID: projectID, Purpose: "Help neighbours request repairs", Audience: "Repair volunteers"})
	spec.Requirements = []SpecificationRequirement{{ID: "AC-request", Title: "Request a repair", Priority: "must", Acceptance: "A submitted request remains available after reopening"}}
	spec.Roadmap = []SpecificationRoadmapPhase{
		{ID: "phase-request", Title: "First useful request flow", Outcome: "Neighbours can request one repair", RequirementIDs: []string{"AC-request"}, DependsOn: []string{}, CompletionCriteria: []string{"A volunteer can reopen the submitted request"}},
		{ID: "phase-review", Title: "Review the request", Outcome: "Volunteers can review the saved request", RequirementIDs: []string{"AC-request"}, DependsOn: []string{"phase-request"}, CompletionCriteria: []string{"The review preserves the original request details"}},
	}
	return spec
}

func TestSpecificationRoadmapValidatesReferencesGraphAndBounds(t *testing.T) {
	cases := []struct {
		name string
		edit func(*ProjectSpecification)
	}{
		{"unknown requirement", func(s *ProjectSpecification) { s.Roadmap[0].RequirementIDs = []string{"AC-missing"} }},
		{"duplicate requirement", func(s *ProjectSpecification) { s.Roadmap[0].RequirementIDs = []string{"AC-request", "AC-request"} }},
		{"unknown dependency", func(s *ProjectSpecification) { s.Roadmap[1].DependsOn = []string{"phase-missing"} }},
		{"duplicate dependency", func(s *ProjectSpecification) { s.Roadmap[1].DependsOn = []string{"phase-request", "phase-request"} }},
		{"self dependency", func(s *ProjectSpecification) { s.Roadmap[0].DependsOn = []string{"phase-request"} }},
		{"cycle", func(s *ProjectSpecification) { s.Roadmap[0].DependsOn = []string{"phase-review"} }},
		{"duplicate phase ID", func(s *ProjectSpecification) { s.Roadmap[1].ID = s.Roadmap[0].ID }},
		{"ID collides with requirement", func(s *ProjectSpecification) { s.Roadmap[0].ID = "AC-request" }},
		{"missing arrays", func(s *ProjectSpecification) { s.Roadmap[0].DependsOn = nil }},
		{"oversized outcome", func(s *ProjectSpecification) { s.Roadmap[0].Outcome = strings.Repeat("x", 12001) }},
		{"empty criterion", func(s *ProjectSpecification) { s.Roadmap[0].CompletionCriteria = []string{" "} }},
		{"oversized criterion", func(s *ProjectSpecification) { s.Roadmap[0].CompletionCriteria = []string{strings.Repeat("x", 2001)} }},
		{"too many phases", func(s *ProjectSpecification) { s.Roadmap = make([]SpecificationRoadmapPhase, 101) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec := roadmapSpecification("p-roadmap")
			tc.edit(&spec)
			if err := validateSpecification(spec); err == nil {
				t.Fatal("invalid roadmap was accepted")
			}
		})
	}
	if err := validateSpecification(roadmapSpecification("p-roadmap")); err != nil {
		t.Fatal("valid dependency graph rejected:", err)
	}
}

func TestSpecificationRoadmapLegacyEncodingAndApprovalDigestStayUnchanged(t *testing.T) {
	const legacy = `{"projectId":"p-legacy","version":1,"revision":2,"briefRevision":1,"startingPoint":"new","purpose":"Repair requests","audience":"Neighbours","constraints":"","outOfScope":[],"requirements":[],"journeys":[],"roles":[],"entities":[],"integrations":[],"glossary":[],"decisions":[],"questions":[],"status":"approved","digest":"","updatedAt":"fixed"}`
	var spec ProjectSpecification
	if err := json.Unmarshal([]byte(legacy), &spec); err != nil {
		t.Fatal(err)
	}
	for _, phases := range [][]SpecificationRoadmapPhase{nil, {}} {
		spec.Roadmap = phases
		encoded, err := json.Marshal(spec)
		if err != nil || string(encoded) != legacy {
			t.Fatalf("legacy bytes changed: %s (%v)", encoded, err)
		}
		sum := sha256.Sum256([]byte(legacy))
		if specificationContentDigest(spec) != hex.EncodeToString(sum[:]) {
			t.Fatal("legacy private approval content digest changed")
		}
	}
	legacyDigest := specificationContentDigest(spec)
	spec.Roadmap = roadmapSpecification("p-legacy").Roadmap
	if specificationContentDigest(spec) == legacyDigest {
		t.Fatal("roadmap content was not hashed")
	}
	before := specificationContentDigest(spec)
	spec.Roadmap[0].Outcome = "A different promised outcome"
	if specificationContentDigest(spec) == before {
		t.Fatal("changed roadmap retained its old approval content digest")
	}
}

func TestSpecificationRoadmapOmissionCannotSilentlyEraseSavedPhases(t *testing.T) {
	_, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "roadmap-preservation")
	base := "/v1/product/projects/" + p.ID
	spec := roadmapSpecification(p.ID)
	if err := json.Unmarshal(journeyRequest(t, mux, "POST", base+"/specification", map[string]any{"expectedRevision": 0, "expectedBriefRevision": 0, "specification": spec}, 200).Body.Bytes(), &spec); err != nil {
		t.Fatal(err)
	}
	spec.Roadmap = nil
	response := journeyRequest(t, mux, "POST", base+"/specification", map[string]any{"expectedRevision": 1, "expectedBriefRevision": 1, "specification": spec}, 400)
	if !strings.Contains(response.Body.String(), "roadmap_omitted") {
		t.Fatal("omission did not identify preservation requirement")
	}
	// A real JSON empty array distinguishes an explicit removal from omission.
	encoded, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	var cleared map[string]any
	if err := json.Unmarshal(encoded, &cleared); err != nil {
		t.Fatal(err)
	}
	cleared["roadmap"] = []any{}
	var result ProjectSpecification
	if err := json.Unmarshal(journeyRequest(t, mux, "POST", base+"/specification", map[string]any{"expectedRevision": 1, "expectedBriefRevision": 1, "specification": cleared}, 200).Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Roadmap) != 0 || result.Revision != 2 {
		t.Fatal("explicit reviewed removal did not save a new revision")
	}
}

func TestSpecificationRoadmapSaveReloadHistoryAndAgentContext(t *testing.T) {
	_, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "roadmap-app")
	base := "/v1/product/projects/" + p.ID
	spec := roadmapSpecification(p.ID)
	response := journeyRequest(t, mux, "POST", base+"/specification", map[string]any{"expectedRevision": 0, "expectedBriefRevision": 0, "specification": spec}, 200)
	if err := json.Unmarshal(response.Body.Bytes(), &spec); err != nil {
		t.Fatal(err)
	}
	var reloaded ProjectSpecification
	if err := json.Unmarshal(journeyRequest(t, mux, "GET", base+"/specification", nil, 200).Body.Bytes(), &reloaded); err != nil {
		t.Fatal(err)
	}
	if len(reloaded.Roadmap) != 2 || reloaded.Roadmap[1].DependsOn[0] != "phase-request" || reloaded.Roadmap[0].RequirementIDs[0] != "AC-request" {
		t.Fatal("roadmap links did not survive canonical save/reload")
	}
	for _, name := range []string{"specification.json", "specification.md"} {
		data, err := os.ReadFile(filepath.Join(p.Path, ".luci", name))
		if err != nil || !strings.Contains(string(data), "phase-review") || !strings.Contains(string(data), "A volunteer can reopen the submitted request") {
			t.Fatalf("agent context %s lost the roadmap: %v", name, err)
		}
	}
	if err := json.Unmarshal(journeyRequest(t, mux, "POST", base+"/specification/approve", map[string]int{"revision": spec.Revision, "briefRevision": spec.BriefRevision}, 200).Body.Bytes(), &spec); err != nil {
		t.Fatal(err)
	}
	approvedDigest := spec.Digest
	spec.Roadmap[0].Outcome = "Neighbours can request and amend one repair"
	request := map[string]any{"expectedRevision": spec.Revision, "expectedBriefRevision": spec.BriefRevision, "specification": spec}
	if err := json.Unmarshal(journeyRequest(t, mux, "POST", base+"/specification", request, 200).Body.Bytes(), &reloaded); err != nil {
		t.Fatal(err)
	}
	journeyRequest(t, mux, "POST", base+"/specification", request, 409)
	if reloaded.Status != "draft" || reloaded.Digest != "" {
		t.Fatal("roadmap amendment retained approval")
	}
	var history struct {
		Revisions []ProjectSpecification `json:"revisions"`
	}
	if err := json.Unmarshal(journeyRequest(t, mux, "GET", base+"/specification/history", nil, 200).Body.Bytes(), &history); err != nil {
		t.Fatal(err)
	}
	if len(history.Revisions) != 3 || history.Revisions[1].Digest != approvedDigest || history.Revisions[1].Roadmap[0].Outcome != "Neighbours can request one repair" {
		t.Fatal("amending a roadmap rewrote its approved historical snapshot")
	}
}

func TestSpecificationRoadmapIncompleteDraftRequiresReviewBeforeApproval(t *testing.T) {
	for _, field := range []string{"outcome", "requirements", "criteria"} {
		t.Run(field, func(t *testing.T) {
			_, mux := journeyFixture(t)
			p := createJourneyProject(t, mux, "unfinished-roadmap")
			spec := roadmapSpecification(p.ID)
			switch field {
			case "outcome":
				spec.Roadmap[0].Outcome = ""
			case "requirements":
				spec.Roadmap[0].RequirementIDs = []string{}
			case "criteria":
				spec.Roadmap[0].CompletionCriteria = []string{}
			}
			base := "/v1/product/projects/" + p.ID
			journeyRequest(t, mux, "POST", base+"/specification", map[string]any{"expectedRevision": 0, "expectedBriefRevision": 0, "specification": spec}, 200)
			response := journeyRequest(t, mux, "POST", base+"/specification/approve", map[string]int{"revision": 1, "briefRevision": 1}, 422)
			if !strings.Contains(response.Body.String(), "roadmap_incomplete") {
				t.Fatal("approval refusal did not identify the incomplete roadmap")
			}
		})
	}
}
