package product

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func strategySpecification(projectID string) ProjectSpecification {
	spec := roadmapSpecification(projectID)
	spec.Strategy = &SpecificationStrategy{
		Customer: "Repair volunteers", Problem: "Requests arrive in disconnected messages", ValueProposition: "One queue with a clear next action",
		Alternatives: []SpecificationAlternative{{ID: "alternative-chat", Name: "Group chat", Difference: "No shared request lifecycle", EvidenceIDs: []string{"source-interview"}}},
		Evidence:     []SpecificationStrategyEvidence{{ID: "source-interview", Source: "Volunteer interview notes", Observation: "The participant described losing a request in chat", ObservedAt: "2026-10-31T00:00:00Z"}},
		Assumptions:  []SpecificationAssumption{{ID: "assumption-queue", Claim: "A shared queue reduces missed requests", Status: "untested", Experiment: "Pilot the queue and compare missed requests", EvidenceIDs: []string{}, RequirementIDs: []string{"AC-request"}}},
		Metrics:      []SpecificationSuccessMetric{{ID: "metric-missed", Name: "Missed requests per week", Baseline: "", Target: "Fewer than one", Window: "Four-week pilot", Measurement: "Compare request receipts with resolved queue items", RequirementIDs: []string{"AC-request"}}},
	}
	return spec
}

func TestSpecificationStrategyRejectsUnsupportedFindingsAndBrokenReferences(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*SpecificationStrategy)
	}{
		{"unsupported finding", func(s *SpecificationStrategy) { s.Assumptions[0].Status = "supported" }},
		{"unknown evidence", func(s *SpecificationStrategy) { s.Alternatives[0].EvidenceIDs = []string{"missing"} }},
		{"duplicate evidence", func(s *SpecificationStrategy) {
			s.Alternatives[0].EvidenceIDs = []string{"source-interview", "source-interview"}
		}},
		{"missing requirement", func(s *SpecificationStrategy) { s.Metrics[0].RequirementIDs = []string{"missing"} }},
		{"credential URL", func(s *SpecificationStrategy) { s.Evidence[0].URL = "https://user:password@example.com/research" }},
		{"active URL", func(s *SpecificationStrategy) { s.Evidence[0].URL = "javascript:alert(1)" }},
		{"invalid date", func(s *SpecificationStrategy) { s.Evidence[0].ObservedAt = "yesterday" }},
		{"missing array", func(s *SpecificationStrategy) { s.Metrics = nil }},
		{"unknown status", func(s *SpecificationStrategy) { s.Assumptions[0].Status = "proved" }},
		{"duplicate id", func(s *SpecificationStrategy) { s.Metrics[0].ID = s.Assumptions[0].ID }},
		{"unbounded summary", func(s *SpecificationStrategy) { s.Problem = strings.Repeat("x", 12001) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spec := strategySpecification("p-strategy")
			tc.change(spec.Strategy)
			if validateSpecification(spec) == nil {
				t.Fatal("invalid strategy accepted")
			}
		})
	}
	spec := strategySpecification("p-strategy")
	spec.Strategy.Assumptions[0].Status = "supported"
	spec.Strategy.Assumptions[0].EvidenceIDs = []string{"source-interview"}
	if err := validateSpecification(spec); err != nil {
		t.Fatal(err)
	}
}

func TestSpecificationStrategyPersistsWithContextAndOmissionDoesNotErase(t *testing.T) {
	_, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "strategy-context")
	base := "/v1/product/projects/" + p.ID
	spec := strategySpecification(p.ID)
	json.Unmarshal(journeyRequest(t, mux, "POST", base+"/specification", map[string]any{"expectedRevision": 0, "expectedBriefRevision": 0, "specification": spec}, 200).Body.Bytes(), &spec)
	before := specificationContentDigest(spec)
	if spec.Strategy == nil || spec.Strategy.Metrics[0].Baseline != "" {
		t.Fatal("unknown baseline was lost")
	}
	spec.Strategy = nil
	spec.Purpose = "Keep all requests visible"
	json.Unmarshal(journeyRequest(t, mux, "POST", base+"/specification", map[string]any{"expectedRevision": 1, "expectedBriefRevision": 1, "specification": spec}, 200).Body.Bytes(), &spec)
	if spec.Strategy == nil || spec.Strategy.Customer != "Repair volunteers" {
		t.Fatal("legacy edit erased strategy")
	}
	journeyRequest(t, mux, "POST", base+"/specification", map[string]any{"expectedRevision": 1, "expectedBriefRevision": 1, "specification": spec}, 409)
	content, err := os.ReadFile(filepath.Join(p.Path, ".luci", "specification.md"))
	if err != nil || !strings.Contains(string(content), "assumption-queue (untested)") || !strings.Contains(string(content), "Four-week pilot") {
		t.Fatal("strategy missing from downstream context", err)
	}
	if specificationContentDigest(spec) == before {
		t.Fatal("strategy context digest did not change with revision")
	}
	spec.Strategy.Problem = "Changed problem"
	changed := specificationContentDigest(spec)
	spec.Strategy.Problem = "Another problem"
	if specificationContentDigest(spec) == changed {
		t.Fatal("strategy changes are not bound to approval digest")
	}
	var history struct {
		Revisions []ProjectSpecification `json:"revisions"`
	}
	json.Unmarshal(journeyRequest(t, mux, "GET", base+"/specification/history", nil, 200).Body.Bytes(), &history)
	if len(history.Revisions) != 2 || history.Revisions[0].Strategy.Problem != "Requests arrive in disconnected messages" {
		t.Fatal("immutable strategy history changed")
	}
}

func TestPlanningOmissionPreservesAndExplicitEmptyClears(t *testing.T) {
	current := strategySpecification("p-strategy")
	next := current
	next.Strategy, next.Roadmap = nil, nil
	preserveSpecificationPlanning(&next, current)
	if next.Strategy == nil || len(next.Roadmap) != 2 {
		t.Fatal("omission lost planning context")
	}
	next.Strategy = &SpecificationStrategy{Alternatives: []SpecificationAlternative{}, Assumptions: []SpecificationAssumption{}, Evidence: []SpecificationStrategyEvidence{}, Metrics: []SpecificationSuccessMetric{}}
	next.Roadmap = []SpecificationRoadmapPhase{}
	preserveSpecificationPlanning(&next, current)
	if len(next.Roadmap) != 0 || next.Strategy.Customer != "" {
		t.Fatal("explicit clearing was ignored")
	}
	answer := cloneOrchestratorAnswer(&OrchestratorAnswer{Proposal: &OrchestratorProposal{Target: "specification", Specification: &next}})
	if answer.Proposal.Specification.Roadmap == nil {
		t.Fatal("cloning lost explicit empty intent")
	}
}

func TestPlanningOmissionRebasesWithoutDeletingConcurrentPlanning(t *testing.T) {
	turn, current := coordinationMergeFixture("specification")
	turn.Context.Specification = strategySpecification("project-test")
	current.Specification = strategySpecification("project-test")
	proposal := strategySpecification("project-test")
	proposal.Strategy, proposal.Roadmap = nil, nil
	proposal.Purpose = "A shared request inbox"
	turn.Answer.Proposal.Specification = &proposal
	current.Specification.Strategy.Problem = "A newer human finding"
	current.Specification.Roadmap[0].Title = "Newer human phase"
	answer, conflicts := coordinationRebase(turn, current)
	if len(conflicts) != 0 {
		t.Fatal("omission was treated as deleting concurrent planning", conflicts)
	}
	result := answer.Proposal.Specification
	if result.Purpose != proposal.Purpose || result.Strategy.Problem != "A newer human finding" || result.Roadmap[0].Title != "Newer human phase" {
		t.Fatal("rebase lost a disjoint contribution")
	}
	if proposal.Strategy != nil || proposal.Roadmap != nil {
		t.Fatal("original proposal was mutated")
	}
	// Explicit clear remains distinct from omission even after JSON persistence.
	proposal.Roadmap = []SpecificationRoadmapPhase{}
	turn.RoadmapExplicitEmpty = true
	raw, _ := json.Marshal(turn)
	var restored storedOrchestratorTurn
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	current.Specification = strategySpecification("project-test")
	answer, conflicts = coordinationRebase(restored, current)
	if len(conflicts) != 0 || answer.Proposal.Specification.Roadmap == nil || len(answer.Proposal.Specification.Roadmap) != 0 {
		t.Fatal("durable explicit clearing was lost", conflicts)
	}
}

func TestPlanningCompletionReplayAndApplyPreserveOmission(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "planning-completion")
	base := "/v1/product/projects/" + p.ID
	spec := strategySpecification(p.ID)
	journeyRequest(t, mux, "POST", base+"/specification", map[string]any{"expectedRevision": 0, "expectedBriefRevision": 0, "specification": spec}, 200)
	ctx := orchestratorActor(t, "alice")
	apply := func(clear bool, purpose string) ProjectSpecification {
		turn, snapshot := reserveOrchestratorTest(t, s, p.ID, ctx, purpose, "")
		proposed := snapshot.Specification
		proposed.Purpose, proposed.Strategy, proposed.Roadmap = purpose, nil, nil
		if clear {
			proposed.Roadmap = []SpecificationRoadmapPhase{}
		}
		answer := OrchestratorAnswer{Summary: "Review the scope amendment.", Options: []string{}, Impact: []string{"Review the updated definition"}, Proposal: &OrchestratorProposal{Target: "specification", Summary: "Updated purpose", Specification: &proposed}}
		if err := s.CompleteOrchestratorTurn(p.ID, turn.ID, turn.JobID, "succeeded", &answer); err != nil {
			t.Fatal(err)
		}
		if err := s.CompleteOrchestratorTurn(p.ID, turn.ID, turn.JobID, "succeeded", &answer); err != nil {
			t.Fatal("exact completion replay failed", err)
		}
		if proposed.Strategy != nil {
			t.Fatal("completion mutated provider input")
		}
		view, err := s.OrchestratorView(ctx, p.ID, 0)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.applyOrchestratorProposal(ctx, p.ID, turn.ID, view.Revision, view.Context.Digest); err != nil {
			t.Fatal(err)
		}
		view, err = s.OrchestratorView(ctx, p.ID, 0)
		if err != nil {
			t.Fatal(err)
		}
		return view.Context.Specification
	}
	first := apply(false, "Keep planning while refining the product")
	if first.Strategy == nil || len(first.Roadmap) != 2 {
		t.Fatal("omitted planning was erased")
	}
	cleared := apply(true, "Deliberately clear roadmap")
	if cleared.Strategy == nil || len(cleared.Roadmap) != 0 {
		t.Fatal("explicit clear was not applied, or unrelated strategy was lost")
	}
}

func TestStrategyPreservationCannotBypassWholeSpecificationLimit(t *testing.T) {
	_, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "bounded-strategy")
	spec := strategySpecification(p.ID)
	for i := 0; i < 30; i++ {
		spec.Strategy.Assumptions = append(spec.Strategy.Assumptions, SpecificationAssumption{ID: fmt.Sprintf("assumption-extra-%d", i), Claim: strings.Repeat("a", 10000), Status: "untested", EvidenceIDs: []string{}, RequirementIDs: []string{}})
	}
	base := "/v1/product/projects/" + p.ID + "/specification"
	if err := json.Unmarshal(journeyRequest(t, mux, "POST", base, map[string]any{"expectedRevision": 0, "expectedBriefRevision": 0, "specification": spec}, 200).Body.Bytes(), &spec); err != nil {
		t.Fatal(err)
	}
	spec.Strategy = nil
	for i := 0; i < 25; i++ {
		spec.Requirements = append(spec.Requirements, SpecificationRequirement{ID: fmt.Sprintf("AC-extra-%d", i), Title: "Additional requirement", Priority: "could", Acceptance: strings.Repeat("b", 10000)})
	}
	// The submitted partial document fits; restoring the retained strategy does not.
	if err := validateSpecification(spec); err != nil {
		t.Fatal("partial fixture is invalid", err)
	}
	journeyRequest(t, mux, "POST", base, map[string]any{"expectedRevision": 1, "expectedBriefRevision": 1, "specification": spec}, 400)
	var current ProjectSpecification
	if err := json.Unmarshal(journeyRequest(t, mux, "GET", base, nil, 200).Body.Bytes(), &current); err != nil {
		t.Fatal(err)
	}
	if current.Revision != 1 || current.Strategy == nil {
		t.Fatal("rejected oversized amendment changed saved specification")
	}
}

func TestHistoricalCompletionWithoutPlanningWitnessStillReplays(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "legacy-planning-replay")
	ctx := orchestratorActor(t, "alice")
	turn, snapshot := reserveOrchestratorTest(t, s, p.ID, ctx, "Review the scope", "")
	spec := snapshot.Specification
	spec.Purpose, spec.Audience = "A request inbox", "Volunteers"
	spec.Roadmap = []SpecificationRoadmapPhase{}
	answer := OrchestratorAnswer{Summary: "Review scope.", Options: []string{}, Impact: []string{}, Proposal: &OrchestratorProposal{Target: "specification", Summary: "Updated scope", Specification: &spec}}
	if err := s.CompleteOrchestratorTurn(p.ID, turn.ID, turn.JobID, "succeeded", &answer); err != nil {
		t.Fatal(err)
	}
	resolved, err := s.resolveProject(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := s.readOrchestrator(resolved)
	if err != nil {
		t.Fatal(err)
	}
	for i := range stored.Turns {
		if stored.Turns[i].ID == turn.ID {
			stored.Turns[i].PlanningNormalized = false
			stored.Turns[i].RoadmapExplicitEmpty = false
		}
	}
	if err := s.saveOrchestrator(stored); err != nil {
		t.Fatal(err)
	}
	if err := s.CompleteOrchestratorTurn(p.ID, turn.ID, turn.JobID, "succeeded", &answer); err != nil {
		t.Fatal("legacy exact replay rejected by a new witness", err)
	}
}

func TestOrchestratorEvidenceOmissionKeepsIndependentAmendments(t *testing.T) {
	turn, current := coordinationMergeFixture("specification")
	fixture := discoveryCompleteFixture(t)
	segment := fixture.Document.Extractions[0].Pages[0].Segments[0]
	link := DiscoveryEvidenceLink{DocumentID: fixture.Document.ID, OriginalDigest: fixture.Document.Original.Digest, ExtractionRevision: 1, Page: 1, SegmentID: segment.ID, TextDigest: segment.TextDigest, TargetKind: "requirement", TargetID: "AC-one"}
	turn.Context.Specification.Evidence = []DiscoveryEvidenceLink{link}
	currentLink := link
	currentLink.TargetID = "AC-two"
	current.Specification.Evidence = []DiscoveryEvidenceLink{currentLink}
	turn.Answer.Proposal.Specification.Purpose = "A refined notebook"
	answer, conflicts := coordinationRebase(turn, current)
	if len(conflicts) != 0 || len(answer.Proposal.Specification.Evidence) != 1 || answer.Proposal.Specification.Evidence[0] != currentLink {
		t.Fatal("omission conflicted with or erased an independent evidence change", conflicts)
	}
	if turn.Answer.Proposal.Specification.Evidence != nil {
		t.Fatal("captured provider proposal was mutated")
	}
	// A proposal removing a cited requirement must explicitly resolve that link.
	removed := cloneOrchestratorAnswer(turn.Answer)
	removed.Proposal.Specification.Requirements = removed.Proposal.Specification.Requirements[1:]
	if ValidateOrchestratorAnswer(turn.Context, *removed) == nil {
		t.Fatal("an omitted evidence field hid a retained link to a removed requirement")
	}
	// A deliberate clear survives durable JSON and is not restored as omission.
	turn.Answer.Proposal.Specification.Evidence = []DiscoveryEvidenceLink{}
	turn.EvidenceExplicitEmpty = true
	raw, _ := json.Marshal(turn)
	var restored storedOrchestratorTurn
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	current.Specification.Evidence = []DiscoveryEvidenceLink{link}
	answer, conflicts = coordinationRebase(restored, current)
	if len(conflicts) != 0 || answer.Proposal.Specification.Evidence == nil || len(answer.Proposal.Specification.Evidence) != 0 {
		t.Fatal("explicit evidence removal was lost", conflicts)
	}
	if copy := cloneOrchestratorAnswer(&answer); copy.Proposal.Specification.Evidence == nil {
		t.Fatal("cloning erased deliberate evidence removal")
	}
	// Origin creation and restart backfill use the same persisted turn, before
	// rebase/apply. Its deletion must remain visible to opposing proposals.
	changes := coordinationChanges(&restored)
	if _, exists := changes["evidence"]; !exists {
		t.Fatal("recovered origin lost the explicit evidence deletion")
	}
	restored.Answer.Proposal.Specification.Requirements = restored.Answer.Proposal.Specification.Requirements[1:]
	recovered := restoredOrchestratorAnswer(restored)
	if err := ValidateOrchestratorAnswer(restored.Context, *recovered); err != nil {
		t.Fatal("origin recovery restored a deliberately removed link", err)
	}
	if reviewDigest(recovered.Proposal) != reviewDigest(restored.Answer.Proposal) {
		t.Fatal("intent restoration changed a legacy proposal digest")
	}
	if restored.Answer.Proposal.Specification.Evidence != nil {
		t.Fatal("recovery mutated the durable turn")
	}
}
