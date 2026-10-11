package product

import (
	"context"
	"errors"
	"fmt"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// LOCAL-026: nonexistent proposal identity is an invalid request, not a failed
// store operation. The actual registered route must refuse without writing any
// canonical context, coordination receipt or execution reservation.
func TestCoordinationMissingTurnReturnsBadRequestWithoutMutation(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "coordination-invalid-turn")
	s.SetCoordinatorAuthorizer(func(context.Context, string) error { return nil })
	h := coordinationHandler(s, "alice")
	view, err := s.OrchestratorView(orchestratorActor(t, "alice"), p.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	beforeState, beforeProject := auditTree(t, s.stateDir), auditTree(t, p.Path)
	request := map[string]any{"expectedRevision": 0, "expectedContextDigest": view.Context.Digest, "turnId": "turn-nonexistent", "idempotencyKey": "local026-missing-turn-request", "dependsOn": []string{}}
	response := journeyRequest(t, h, "POST", "/v1/product/projects/"+p.ID+"/orchestrator/coordination/intents", request, 400)
	if !strings.Contains(response.Body.String(), `"error":"invalid_orchestrator_request"`) {
		t.Fatal("wrong client failure code", response.Body.String())
	}
	if !reflect.DeepEqual(beforeState, auditTree(t, s.stateDir)) || !reflect.DeepEqual(beforeProject, auditTree(t, p.Path)) {
		t.Fatal("invalid turn changed private or source state")
	}
	if len(s.previews) != 0 || len(s.jobs) != 0 || len(s.tunnels) != 0 {
		t.Fatal("invalid turn created a resource")
	}
}

func TestOrchestratorProblemKeepsConflictAndStorageFailureDistinct(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"invalid", ErrOrchestratorInvalid, 400, "invalid_orchestrator_request"},
		{"wrapped-invalid", fmt.Errorf("request: %w", ErrOrchestratorInvalid), 400, "invalid_orchestrator_request"},
		{"conflict", ErrOrchestratorConflict, 409, "revision_conflict"},
		{"storage", errors.New("private storage failed"), 500, "orchestrator_unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			orchestratorProblem(response, tc.err)
			if response.Code != tc.status || !strings.Contains(response.Body.String(), tc.code) {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			if tc.name == "storage" && strings.Contains(response.Body.String(), tc.err.Error()) {
				t.Fatal("unexpected storage internals disclosed")
			}
		})
	}
}
