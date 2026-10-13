package product

import (
	"context"
	"reflect"
	"testing"
)

func TestCoordinatorDerivesExactReviewedPrerequisiteWithoutManualHandoff(t *testing.T) {
	s, mux, p, old, selected := reuseIntegratedFixture(t)
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	expanded, err := s.coordinatorExpandAffectedPrerequisites(context.Background(), p, state, []string{"task-c"})
	if err != nil || !reflect.DeepEqual(expanded, []string{"task-c"}) {
		t.Fatal("unchanged reviewed prerequisite was needlessly restarted", expanded, err)
	}
	next := reuseCreateAffected(t, s, mux, p, selected, "coordinator-reuse-affected-123")
	refs, err := s.coordinatorReuseReferences(context.Background(), p, next)
	if err != nil || !reflect.DeepEqual(refs, []ProjectSwarmReuseReference{reuseRef(old)}) {
		t.Fatal("coordinator did not select the unchanged exact prerequisite", refs, err)
	}
	base := "/v1/product/projects/" + p.ID + "/runs/" + next.ID + "/swarm"
	next = projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base, map[string]any{"expectedRevision": next.Revision, "maxParallel": 1, "reusedDependencies": refs}, 201)).ProjectRun
	if next.Swarm == nil || len(next.Swarm.Tasks) != 1 || next.Swarm.Tasks[0].ID != "task-c" || len(next.Swarm.ReusedDependencies) != 1 || next.Swarm.ReusedDependencies[0].TaskID != "task-a" {
		t.Fatal("affected task was not prepared against exact prior integration", next.Swarm)
	}
}

func TestCoordinatorIncludesUnprovenPrerequisiteInNewAffectedRun(t *testing.T) {
	s, _, p, _, _, _ := swarmHTTPFixture(t)
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	selected, err := s.coordinatorExpandAffectedPrerequisites(context.Background(), p, state, []string{"task-c"})
	if err != nil || !reflect.DeepEqual(selected, []string{"task-a", "task-c"}) {
		t.Fatal("missing prerequisite was left to a manual handoff", selected, err)
	}
}
