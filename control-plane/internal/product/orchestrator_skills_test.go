package product

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

func TestOrchestratorSkillsCatalogueIsBoundedAndDoesNotRewriteLegacyContext(t *testing.T) {
	state := skillsState(t)
	for i := 0; i < 30; i++ {
		v := state.Skills.Versions[0]
		v.ID = "extra-" + strings.Repeat("x", i+1)
		v.Digest = skillVersionDigest(v)
		state.Skills.Versions = append(state.Skills.Versions, v)
	}
	before, _ := json.Marshal(state)
	value, err := orchestratorSkillsContext(state)
	if err != nil || len(value.Versions) != 24 || value.OmittedVersions != 7 || value.RegistryDigest != reviewDigest(currentSkills(state)) {
		t.Fatal("catalogue identity/bounds", value, err)
	}
	after, _ := json.Marshal(state)
	if string(before) != string(after) {
		t.Fatal("projection mutated saved skills")
	}
	empty := journeyState{}
	if v, err := orchestratorSkillsContext(empty); err != nil || v != nil {
		t.Fatal("legacy absence was synthesized")
	}
	raw, _ := json.Marshal(OrchestratorContext{})
	if strings.Contains(string(raw), `"skills"`) {
		t.Fatal("legacy optional encoding changed")
	}
}

func skillSuggestionFixture(t *testing.T) (OrchestratorContext, *OrchestratorProposal) {
	t.Helper()
	state := skillsState(t)
	skills, err := orchestratorSkillsContext(state)
	if err != nil {
		t.Fatal(err)
	}
	plan := currentPlan(state, state.Brief.ProjectID)
	v := skills.Versions[0]
	return OrchestratorContext{ProjectID: state.Brief.ProjectID, Skills: skills}, &OrchestratorProposal{Target: "plan", Plan: &plan, SuggestedSkillAssignments: []OrchestratorSkillSuggestion{{TaskID: "save", Role: "coding", SkillID: v.ID, SkillRevision: v.Revision, SkillDigest: v.Digest, Reason: "Check failed saves preserve the existing draft."}}}
}

func TestOrchestratorSkillSuggestionsRequireExactCatalogueTaskAndRole(t *testing.T) {
	snapshot, proposal := skillSuggestionFixture(t)
	if err := validateOrchestratorSkillSuggestions(snapshot, proposal); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"other-target", "missing-catalogue", "unknown-task", "wrong-role", "wrong-revision", "wrong-digest", "duplicate", "omitted-version"} {
		t.Run(name, func(t *testing.T) {
			snapshot, proposal := skillSuggestionFixture(t)
			switch name {
			case "other-target":
				proposal.Target = "gtm"
			case "missing-catalogue":
				snapshot.Skills = nil
			case "unknown-task":
				proposal.SuggestedSkillAssignments[0].TaskID = "foreign"
			case "wrong-role":
				proposal.SuggestedSkillAssignments[0].Role = "reviewer"
			case "wrong-revision":
				proposal.SuggestedSkillAssignments[0].SkillRevision++
			case "wrong-digest":
				proposal.SuggestedSkillAssignments[0].SkillDigest = strings.Repeat("0", 64)
			case "duplicate":
				proposal.SuggestedSkillAssignments = append(proposal.SuggestedSkillAssignments, proposal.SuggestedSkillAssignments[0])
			case "omitted-version":
				snapshot.Skills.Versions = []OrchestratorSkillSummary{}
				snapshot.Skills.OmittedVersions = 1
			}
			if err := validateOrchestratorSkillSuggestions(snapshot, proposal); err == nil {
				t.Fatal("invalid routing accepted", name)
			}
		})
	}
}

func TestSuggestedSkillSaveFencesPlanRevisionBeforeAnyRegistryMutation(t *testing.T) {
	state := skillsState(t)
	before, _ := json.Marshal(state)
	expected := currentPlan(state, state.Brief.ProjectID).Revision + 1
	req := ProjectSkillsSave{ExpectedRevision: state.Skills.Revision, ExpectedPlanRevision: &expected, Assignments: []ProjectSkillAssignment{}}
	if _, err := applySkillsSave(state, req); err == nil {
		t.Fatal("stale plan accepted")
	}
	after, _ := json.Marshal(state)
	if string(before) != string(after) {
		t.Fatal("failed CAS partially changed stored state")
	}
	expected--
	req.ExpectedPlanRevision = &expected
	saved, err := applySkillsSave(state, req)
	if err != nil || len(saved.Assignments) != 0 {
		t.Fatal("current exact plan rejected", err)
	}
	// Existing manual editor requests omit this new optional CAS field.
	req.ExpectedPlanRevision = nil
	legacy, err := applySkillsSave(state, req)
	if err != nil || !reflect.DeepEqual(saved, legacy) {
		t.Fatal("legacy omitted plan CAS changed behavior", err)
	}
	raw, _ := json.Marshal(ProjectSkillsSave{})
	if strings.Contains(string(raw), "expectedPlanRevision") {
		t.Fatal("legacy request encoding changed")
	}
}

func TestSuggestedSkillSaveStalePlanLeavesPersistedRegistryAndHistoryUntouched(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerProjectWork(mux)
	p := createJourneyProject(t, mux, "skill-route-cas")
	handler := contextHistoryActorHandler(mux, management.User{ID: "skill-editor", Username: "editor", Role: "member"})
	base := "/v1/product/projects/" + p.ID
	var before, after ProjectSkillRegistry
	decodeEvidenceResponse(t, journeyRequest(t, handler, "POST", base+"/skills", ProjectSkillsSave{Skill: skillDraft("review", "Review saved output before using it."), Assignments: []ProjectSkillAssignment{}}, 200).Body.Bytes(), &before)
	history, err := s.readContextHistory(p.Path, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	var plan ProjectPlan
	decodeEvidenceResponse(t, journeyRequest(t, handler, "GET", base+"/plan", nil, 200).Body.Bytes(), &plan)
	stale := plan.Revision + 1
	journeyRequest(t, handler, "POST", base+"/skills", ProjectSkillsSave{ExpectedRevision: before.Revision, ExpectedPlanRevision: &stale, Assignments: []ProjectSkillAssignment{}}, 422)
	decodeEvidenceResponse(t, journeyRequest(t, handler, "GET", base+"/skills", nil, 200).Body.Bytes(), &after)
	later, err := s.readContextHistory(p.Path, p.ID)
	if err != nil || !reflect.DeepEqual(before, after) || !reflect.DeepEqual(history, later) {
		t.Fatal("stale plan CAS partially wrote registry or attributed history", err)
	}
}
