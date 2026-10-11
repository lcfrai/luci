package product

import (
	"encoding/json"
	"reflect"
	"testing"
)

func coordinationMergeFixture(target string) (storedOrchestratorTurn, OrchestratorContext) {
	base := OrchestratorContext{ProjectID: "project-test", Specification: ProjectSpecification{ProjectID: "project-test", Version: 1, StartingPoint: "new", Status: "draft", Purpose: "Notebook", Audience: "Editors", Requirements: []SpecificationRequirement{{ID: "AC-one", Title: "One", Priority: "must", Acceptance: "One works"}, {ID: "AC-two", Title: "Two", Priority: "must", Acceptance: "Two works"}}}, DesignPlan: ProjectDesignPlan{ProjectID: "project-test", Status: "draft", Tokens: []DesignToken{{Name: "primary", Value: "blue", Purpose: "Primary actions"}, {Name: "secondary", Value: "gray", Purpose: "Secondary actions"}}}, Plan: ProjectPlan{ProjectID: "project-test", Tasks: []ProjectTask{{ID: "task-one", Title: "One", Status: "todo"}, {ID: "task-two", Title: "Two", Status: "todo"}}}}
	normalizeOrchestratorCollections(&base)
	raw, _ := json.Marshal(base)
	var current OrchestratorContext
	_ = json.Unmarshal(raw, &current)
	proposal := &OrchestratorProposal{Target: target, Summary: "Change one selected scope"}
	switch target {
	case "specification":
		var v ProjectSpecification
		r, _ := json.Marshal(base.Specification)
		_ = json.Unmarshal(r, &v)
		proposal.Specification = &v
	case "design-plan":
		var v ProjectDesignPlan
		r, _ := json.Marshal(base.DesignPlan)
		_ = json.Unmarshal(r, &v)
		proposal.DesignPlan = &v
	case "plan":
		var v ProjectPlan
		r, _ := json.Marshal(base.Plan)
		_ = json.Unmarshal(r, &v)
		proposal.Plan = &v
	}
	return storedOrchestratorTurn{OrchestratorTurn: OrchestratorTurn{Context: base, Answer: &OrchestratorAnswer{Summary: "A scoped amendment", Proposal: proposal}}}, current
}

func TestCoordinationThreeWayMergePreservesContentAndMetadata(t *testing.T) {
	cases := []struct {
		name, target string
		change       func(*storedOrchestratorTurn, *OrchestratorContext)
		check        func(*testing.T, OrchestratorAnswer, OrchestratorContext)
		conflict     bool
	}{
		{"different requirement records", "specification", func(turn *storedOrchestratorTurn, current *OrchestratorContext) {
			turn.Answer.Proposal.Specification.Requirements[0].Title = "Improved one"
			current.Specification.Requirements[1].Title = "Improved two"
			current.Specification.Revision = 7
		}, func(t *testing.T, a OrchestratorAnswer, c OrchestratorContext) {
			if a.Proposal.Specification.Requirements[0].Title != "Improved one" || a.Proposal.Specification.Requirements[1].Title != "Improved two" || a.Proposal.Specification.Revision != 7 {
				t.Fatal("lost contribution or current metadata")
			}
		}, false},
		{"same record contradictory edit", "specification", func(turn *storedOrchestratorTurn, current *OrchestratorContext) {
			turn.Answer.Proposal.Specification.Requirements[0].Title = "Private"
			current.Specification.Requirements[0].Title = "Public"
		}, nil, true},
		{"record deletion against edit", "specification", func(turn *storedOrchestratorTurn, current *OrchestratorContext) {
			turn.Answer.Proposal.Specification.Requirements = turn.Answer.Proposal.Specification.Requirements[1:]
			current.Specification.Requirements[0].Title = "Changed"
		}, nil, true},
		{"same content already present", "specification", func(turn *storedOrchestratorTurn, current *OrchestratorContext) {
			turn.Answer.Proposal.Specification.Audience = "Teams"
			current.Specification.Audience = "Teams"
		}, nil, false},
		{"legacy brief projected change", "specification", func(turn *storedOrchestratorTurn, current *OrchestratorContext) {
			turn.Answer.Proposal.Specification.Audience = "Teams"
			current.Specification.Purpose = "New human brief"
		}, func(t *testing.T, a OrchestratorAnswer, c OrchestratorContext) {
			if a.Proposal.Specification.Purpose != "New human brief" {
				t.Fatal("projected legacy brief lost")
			}
		}, false},
		{"keyed reorder remains explicit", "specification", func(turn *storedOrchestratorTurn, current *OrchestratorContext) {
			r := turn.Answer.Proposal.Specification.Requirements
			r[0], r[1] = r[1], r[0]
		}, func(t *testing.T, a OrchestratorAnswer, c OrchestratorContext) {
			if a.Proposal.Specification.Requirements[0].ID != "AC-two" {
				t.Fatal("requested order silently discarded")
			}
		}, false},
		{"named design tokens", "design-plan", func(turn *storedOrchestratorTurn, current *OrchestratorContext) {
			turn.Answer.Proposal.DesignPlan.Tokens[0].Value = "green"
			current.DesignPlan.Tokens[1].Value = "black"
		}, func(t *testing.T, a OrchestratorAnswer, c OrchestratorContext) {
			if a.Proposal.DesignPlan.Tokens[0].Value != "green" || a.Proposal.DesignPlan.Tokens[1].Value != "black" {
				t.Fatal("design token contribution lost")
			}
		}, false},
		{"changed prototype dependency", "design-plan", func(turn *storedOrchestratorTurn, current *OrchestratorContext) {
			turn.Answer.Proposal.DesignPlan.ImplementationNotes = "Use native inputs"
			current.DesignPlan.PrototypeReleaseID = "prototype-new"
		}, nil, true},
		{"current task progress cannot regress", "plan", func(turn *storedOrchestratorTurn, current *OrchestratorContext) {
			turn.Answer.Proposal.Plan.Tasks[0].Title = "Improved title"
			current.Plan.Tasks[0].Status = "doing"
		}, nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			turn, current := coordinationMergeFixture(tc.target)
			tc.change(&turn, &current)
			before, _ := json.Marshal(turn)
			answer, conflicts := coordinationRebase(turn, current)
			if (len(conflicts) > 0) != tc.conflict {
				t.Fatalf("conflict=%v expected %v", conflicts, tc.conflict)
			}
			if !tc.conflict {
				if err := ValidateOrchestratorAnswer(current, answer); err != nil {
					t.Fatal("merged proposal invalid", err)
				}
				if tc.check != nil {
					tc.check(t, answer, current)
				}
			}
			after, _ := json.Marshal(turn)
			if !reflect.DeepEqual(before, after) {
				t.Fatal("original input changed")
			}
		})
	}
}

func TestCoordinationRevalidatesMergedDependencyGraph(t *testing.T) {
	turn, current := coordinationMergeFixture("plan")
	turn.Answer.Proposal.Plan.Tasks[0].DependsOn = []string{"task-two"}
	current.Plan.Tasks[1].DependsOn = []string{"task-one"}
	if err := ValidateOrchestratorAnswer(turn.Context, *turn.Answer); err != nil {
		t.Fatal("original fixture invalid", err)
	}
	if err := validatePlan(current.Plan); err != nil {
		t.Fatal("concurrent fixture invalid", err)
	}
	answer, conflicts := coordinationRebase(turn, current)
	if len(conflicts) != 0 {
		t.Fatal("fixture should be structurally disjoint")
	}
	if err := ValidateOrchestratorAnswer(current, answer); err == nil {
		t.Fatal("cycle formed by individually valid changes accepted")
	}
}
