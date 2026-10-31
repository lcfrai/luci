package product

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

func reuseFixture() TaskReuseSource {
	u := "https://github.com/example/component"
	sum := sha256.Sum256([]byte(u))
	s := TaskReuseSource{ID: hex.EncodeToString(sum[:])[:20], Name: "Component", URL: u, Description: "A saved candidate", Licence: "MIT", LicenceStatus: "known", LicenceScope: "repository", LicenceBasis: "upstream-metadata", ObservedAt: "2026-10-30T01:00:00Z", CatalogueGeneratedAt: "2026-10-30T02:00:00Z", CapturedAt: "2026-10-31T01:00:00Z", CatalogueHash: strings.Repeat("a", 64), ResponseDigest: strings.Repeat("b", 64), CitationCount: 1, Citations: []TaskReuseCitation{{URL: u, Digest: strings.Repeat("c", 64), ObservedAt: "2026-10-30T01:00:00Z"}}}
	s.Digest = TaskReuseSourceDigest(s)
	return s
}
func TestTaskReusePreservesOmissionAndExplicitRemoval(t *testing.T) {
	rows := []TaskReuseDecision{{Source: reuseFixture(), Decision: "investigate"}}
	old := []ProjectTask{{ID: "task-1", Title: "Task", Status: "todo", Reuse: &rows}}
	next := preserveTaskReuse(old, []ProjectTask{{ID: "task-1", Title: "Edited", Status: "todo"}})
	if next[0].Reuse == nil || len(*next[0].Reuse) != 1 {
		t.Fatal("lost old reference")
	}
	(*next[0].Reuse)[0].Source.Citations[0].URL = "https://example.org/changed"
	if (*old[0].Reuse)[0].Source.Citations[0].URL != rows[0].Source.URL {
		t.Fatal("snapshot aliases source")
	}
	empty := []TaskReuseDecision{}
	cleared := preserveTaskReuse(old, []ProjectTask{{ID: "task-1", Reuse: &empty}})
	raw, _ := json.Marshal(cleared[0])
	if !strings.Contains(string(raw), "\"reuse\":[]") {
		t.Fatal("explicit empty lost", string(raw))
	}
	legacy, _ := json.Marshal(ProjectTask{ID: "task-2"})
	if strings.Contains(string(legacy), "reuse") {
		t.Fatal("legacy bytes gained field")
	}
}
func TestTaskReuseRequiresReviewAndPinnedPublicMetadata(t *testing.T) {
	s := reuseFixture()
	if err := ValidateTaskReuseSource(s); err != nil {
		t.Fatal(err)
	}
	rows := []TaskReuseDecision{{Source: s, Decision: "use"}}
	if validateTaskReuse(&rows) == nil {
		t.Fatal("accepted unreviewed use")
	}
	rows[0].Rationale = "Fits data table"
	rows[0].LicenceReview = "MIT notice retained"
	rows[0].CompatibilityReview = "React version needs local check"
	if err := validateTaskReuse(&rows); err != nil {
		t.Fatal(err)
	}
	rows[0].Source.Name = "Changed after capture"
	if validateTaskReuse(&rows) == nil {
		t.Fatal("accepted changed source digest")
	}
	for _, u := range []string{"http://example.com/x", "https://127.0.0.1/x", "https://example.local/x", "https://user:pass@example.com/x", "https://example.com/x?token=secret"} {
		if TaskReusePublicURL(u) {
			t.Fatal("unsafe source link", u)
		}
	}
}
func TestTaskReusePlanCaptureRetainsSourceWithoutAcquisition(t *testing.T) {
	rows := []TaskReuseDecision{{Source: reuseFixture(), Decision: "investigate", Rationale: "Compare to current requirements"}}
	plan := ProjectPlan{Tasks: []ProjectTask{{ID: "task-1", Title: "Build table", Status: "todo", Reuse: &rows}}}
	if err := validatePlan(plan); err != nil {
		t.Fatal(err)
	}
	snapshot := snapshotProjectTasks(plan.Tasks)
	before := implementationDigest(plan)
	(*snapshot[0].Reuse)[0].Decision = "reject"
	if implementationDigest(plan) != before {
		t.Fatal("snapshot changed canonical plan")
	}
	if implementationDigest(ProjectPlan{Tasks: snapshot}) == before {
		t.Fatal("reuse decision omitted from context digest")
	}
}

func TestAssistantReuseSourcesMustBelongToCapturedPlan(t *testing.T) {
	rows := []TaskReuseDecision{{Source: reuseFixture(), Decision: "investigate"}}
	snapshot := OrchestratorContext{ProjectID: "project-1", Plan: ProjectPlan{ProjectID: "project-1", Tasks: []ProjectTask{{ID: "task-1", Title: "Build table", Status: "todo", Reuse: &rows}}}}
	proposal := ProjectPlan{ProjectID: "project-1", Tasks: snapshotProjectTasks(snapshot.Plan.Tasks)}
	answer := OrchestratorAnswer{Summary: "Compare this saved source before implementation.", Proposal: &OrchestratorProposal{Target: "plan", Summary: "Retain the selected source", Plan: &proposal}}
	if err := ValidateOrchestratorAnswer(snapshot, answer); err != nil {
		t.Fatal("exact captured source rejected", err)
	}
	(*proposal.Tasks[0].Reuse)[0].Decision = "reject"
	(*proposal.Tasks[0].Reuse)[0].Rationale = "Does not fit the project's requirements"
	if err := ValidateOrchestratorAnswer(snapshot, answer); err != nil {
		t.Fatal("review notes cannot change", err)
	}
	changed := &(*proposal.Tasks[0].Reuse)[0].Source
	changed.Licence = "Apache-2.0"
	changed.Digest = TaskReuseSourceDigest(*changed)
	if err := ValidateTaskReuseSource(*changed); err != nil {
		t.Fatal("fixture must be self-consistent", err)
	}
	if ValidateOrchestratorAnswer(snapshot, answer) == nil {
		t.Fatal("assistant invented licence metadata")
	}
	proposal.Tasks = snapshotProjectTasks(snapshot.Plan.Tasks)
	empty := []TaskReuseDecision{}
	snapshot.Plan.Tasks[0].Reuse = &empty
	if ValidateOrchestratorAnswer(snapshot, answer) == nil {
		t.Fatal("history-only source resurrected after removal")
	}
	if err := validateStoredOrchestratorAnswer(snapshot, answer); err != nil {
		t.Fatal("historical proposal became unreadable", err)
	}
	proposal.Tasks[0].Reuse = &empty
	if err := ValidateOrchestratorAnswer(snapshot, answer); err != nil {
		t.Fatal("explicit removal rejected", err)
	}
	proposal.Tasks[0].Reuse = nil
	if err := ValidateOrchestratorAnswer(snapshot, answer); err != nil {
		t.Fatal("omitted legacy reference rejected", err)
	}
}
