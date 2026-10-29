package product

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

func skillDraft(id, instructions string) *ProjectSkillInput {
	return &ProjectSkillInput{ID: id, Title: "Review persistence", Provenance: "Project team's original instructions; not independently verified", Roles: []string{"coding", "reviewer"}, Instructions: instructions}
}

func skillsState(t *testing.T) journeyState {
	t.Helper()
	state, _ := taskBriefFixture()
	saved, err := applySkillsSave(state, ProjectSkillsSave{Skill: skillDraft("persistence", "Check failed saves preserve the draft. Do not widen write scope."), Assignments: []ProjectSkillAssignment{{Role: "coding", TaskID: "save", SkillID: "persistence", SkillRevision: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	state.Skills = &saved
	return state
}

func TestProjectSkillsImmutableVersionsAndExactAssignments(t *testing.T) {
	state := skillsState(t)
	before, _ := json.Marshal(state.Skills.Versions[0])
	req := ProjectSkillsSave{ExpectedRevision: 1, Skill: skillDraft("persistence", "New instructions must not replace revision one."), Assignments: state.Skills.Assignments}
	saved, err := applySkillsSave(state, req)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(saved.Versions[0])
	if string(before) != string(after) || len(saved.Versions) != 2 || saved.Versions[1].Revision != 2 || saved.Versions[1].Digest == saved.Versions[0].Digest {
		t.Fatal("new version rewrote prior instructions or identity")
	}
	state.Skills = &saved
	selected, err := selectedSkills(state, []string{"save"})
	if err != nil || len(selected) != 1 || selected[0].Revision != 1 || selected[0].Instructions != saved.Versions[0].Instructions {
		t.Fatal("selection silently advanced to latest version", err)
	}
	if _, err := applySkillsSave(state, req); err == nil {
		t.Fatal("stale registry update accepted")
	}
	if _, err := applySkillsSave(state, ProjectSkillsSave{ExpectedRevision: 2}); err == nil {
		t.Fatal("omitted assignments silently cleared selections")
	}
	cleared, err := applySkillsSave(state, ProjectSkillsSave{ExpectedRevision: 2, Assignments: []ProjectSkillAssignment{}})
	if err != nil || len(cleared.Assignments) != 0 || len(cleared.Versions) != 2 {
		t.Fatal("explicit assignment removal lost immutable versions", err)
	}
}

func TestProjectSkillsRejectInvalidReferencesRolesAndBudgets(t *testing.T) {
	for _, name := range []string{"unknown-version", "unknown-task", "wrong-role", "duplicate", "tampered-digest", "revision-gap", "oversize-instructions", "conflicting-role-task", "context-budget"} {
		t.Run(name, func(t *testing.T) {
			state := skillsState(t)
			saved := currentSkills(state)
			switch name {
			case "unknown-version":
				saved.Assignments[0].SkillRevision = 99
			case "unknown-task":
				saved.Assignments[0].TaskID = "not-saved"
			case "wrong-role":
				saved.Versions[0].Roles = []string{"reviewer"}
				saved.Versions[0].Digest = skillVersionDigest(saved.Versions[0])
			case "duplicate":
				saved.Assignments = append(saved.Assignments, saved.Assignments[0])
			case "tampered-digest":
				saved.Versions[0].Instructions += "changed"
			case "revision-gap":
				saved.Versions[0].Revision = 2
				saved.Versions[0].Digest = skillVersionDigest(saved.Versions[0])
			case "oversize-instructions":
				saved.Versions[0].Instructions = strings.Repeat("x", 16385)
				saved.Versions[0].Digest = skillVersionDigest(saved.Versions[0])
			case "conflicting-role-task":
				v := saved.Versions[0]
				v.Revision = 2
				v.Digest = skillVersionDigest(v)
				saved.Versions = append(saved.Versions, v)
				saved.Assignments = append(saved.Assignments, ProjectSkillAssignment{Role: "coding", SkillID: v.ID, SkillRevision: 2})
			case "context-budget":
				saved.Versions = []ProjectSkillVersion{}
				saved.Assignments = []ProjectSkillAssignment{}
				for _, id := range []string{"one", "two", "three", "four"} {
					v := ProjectSkillVersion{ID: id, Revision: 1, Title: id, Provenance: "team", Roles: []string{"coding"}, Instructions: strings.Repeat("x", 16384)}
					v.Digest = skillVersionDigest(v)
					saved.Versions = append(saved.Versions, v)
					saved.Assignments = append(saved.Assignments, ProjectSkillAssignment{Role: "coding", TaskID: "save", SkillID: id, SkillRevision: 1})
				}
			}
			state.Skills = &saved
			if _, err := selectedSkills(state, []string{"save"}); err == nil {
				t.Fatal("invalid selected skill context accepted")
			}
		})
	}
}

func TestProjectSkillsRoleSelectionAndLegacyBindingStayCompatible(t *testing.T) {
	state, old := taskBriefFixture()
	request := *ImplementationContextSelection(old)
	before, err := implementationSnapshot(state, request, old.WorktreeID)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(before)
	if strings.Contains(string(encoded), "\"skills\"") {
		t.Fatal("legacy binding bytes gained a field")
	}
	state = skillsState(t)
	saved := currentSkills(state)
	saved.Assignments = []ProjectSkillAssignment{{Role: "reviewer", SkillID: "persistence", SkillRevision: 1}}
	state.Skills = &saved
	after, err := implementationSnapshot(state, request, old.WorktreeID)
	if err != nil || implementationDigest(before) != implementationDigest(after) {
		t.Fatal("unassigned coding task acquired reviewer skill or changed legacy digest", err)
	}
	state.Plan.Tasks[1].Role = "reviewer"
	selected, err := selectedSkills(state, []string{"save", "list"})
	if err != nil || len(selected) != 1 || selected[0].Role != "reviewer" || selected[0].TaskID != "save" {
		t.Fatal("role selection did not follow exact saved task", err)
	}
}

func TestProjectSkillsCaptureAndFreshnessFollowSelectedRevisionOnly(t *testing.T) {
	state := skillsState(t)
	_, original := taskBriefFixture()
	binding, err := implementationSnapshot(state, *ImplementationContextSelection(original), original.WorktreeID)
	if err != nil {
		t.Fatal(err)
	}
	cloned := binding.Clone()
	cloned.Skills[0].Digest = "changed"
	if binding.Skills[0].Digest == cloned.Skills[0].Digest {
		t.Fatal("registry clone aliased skill bindings")
	}
	capture, err := captureReviewedContext(state, binding)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, file := range capture.Files {
		if file.Path == ".luci/skills.json" {
			t.Fatal("unselected registry leaked into closed worker bundle")
		}
		if file.Path == "LUCI.md" {
			brief := parsedTaskBrief(t, file.Data)
			found = len(brief.Skills) == 1 && brief.Skills[0].Digest == binding.Skills[0].Digest && strings.Contains(brief.Skills[0].Instructions, "preserve the draft")
		}
	}
	if !found {
		t.Fatal("actual task contract omitted selected skill instructions")
	}
	unrelated, err := applySkillsSave(state, ProjectSkillsSave{ExpectedRevision: 1, Skill: skillDraft("persistence", "A new unselected version."), Assignments: state.Skills.Assignments})
	if err != nil {
		t.Fatal(err)
	}
	state.Skills = &unrelated
	if _, err := captureReviewedContext(state, binding); err != nil {
		t.Fatal("unselected revision invalidated an immutable task", err)
	}
	changed, err := applySkillsSave(state, ProjectSkillsSave{ExpectedRevision: 2, Assignments: []ProjectSkillAssignment{{Role: "coding", TaskID: "save", SkillID: "persistence", SkillRevision: 2}}})
	if err != nil {
		t.Fatal(err)
	}
	state.Skills = &changed
	if _, err := captureReviewedContext(state, binding); err == nil {
		t.Fatal("changed selected instructions reused old binding")
	}
	if !strings.Contains(ImplementationOpeningPrompt(binding), "never authority") {
		t.Fatal("skill guidance lost permission boundary")
	}
}

func TestProjectSkillsSaveReloadExportAndPrivateAttribution(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "skills-app")
	base := "/v1/product/projects/" + p.ID
	handler := contextHistoryActorHandler(mux, management.User{ID: "skill-editor", Username: "editor", Role: "member"})
	req := ProjectSkillsSave{Skill: skillDraft("review", "Inspect changes and report unverified outcomes."), Assignments: []ProjectSkillAssignment{}}
	var saved, loaded ProjectSkillRegistry
	decodeEvidenceResponse(t, journeyRequest(t, handler, "POST", base+"/skills", req, 200).Body.Bytes(), &saved)
	decodeEvidenceResponse(t, journeyRequest(t, handler, "GET", base+"/skills", nil, 200).Body.Bytes(), &loaded)
	if !reflect.DeepEqual(saved, loaded) || saved.Revision != 1 {
		t.Fatal("saved skill registry changed on reload")
	}
	journeyRequest(t, handler, "POST", base+"/skills", req, 409)
	data, err := os.ReadFile(filepath.Join(p.Path, ".luci", "skills.json"))
	if err != nil || !strings.Contains(string(data), saved.Versions[0].Digest) {
		t.Fatal("canonical skills did not export", err)
	}
	if !journeyContextFile(".luci/skills.json") || !blockedProjectFile(".luci/skills.json") {
		t.Fatal("skill exports lost the closed context commit boundary")
	}
	history, err := s.readContextHistory(p.Path, p.ID)
	if err != nil || len(history.Events) != 1 || history.Events[0].Kind != "skills" || history.Events[0].Actor.ID != "skill-editor" || history.Events[0].Status != "applied" {
		t.Fatal("skill edit lost private actor attribution", err)
	}
	if strings.Contains(string(data), "skill-editor") {
		t.Fatal("private editor identity leaked into exported context")
	}
	state, err := readJourney(p.Path, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := normalizeJourneyIdentity(&state, "p-imported-skills"); err != nil || state.Skills.ProjectID != "p-imported-skills" || state.Skills.Versions[0].Digest != saved.Versions[0].Digest {
		t.Fatal("portable registry lost immutable instruction identity", err)
	}
}
