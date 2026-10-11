package product

import (
	"context"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

func coordinationHandler(s *Server, actor string) http.Handler {
	mux := http.NewServeMux()
	s.Register(mux)
	return contextHistoryActorHandler(mux, management.User{ID: actor, Username: actor, DisplayName: actor, Role: "member"})
}

func coordinationProposalTest(t *testing.T, s *Server, id, actor, field, value string) OrchestratorTurn {
	t.Helper()
	ctx := orchestratorActor(t, actor)
	turn, snapshot := reserveOrchestratorTest(t, s, id, ctx, "Change "+field+" to "+value, "")
	spec := snapshot.Specification
	spec.Status, spec.Digest = "draft", ""
	switch field {
	case "purpose":
		spec.Purpose = value
	case "audience":
		spec.Audience = value
	default:
		t.Fatal("unknown field")
	}
	answer := OrchestratorAnswer{Summary: "Proposed change", Options: []string{}, Impact: []string{}, Proposal: &OrchestratorProposal{Target: "specification", Summary: "Change " + field, Specification: &spec}}
	if err := s.CompleteOrchestratorTurn(id, turn.ID, turn.JobID, "succeeded", &answer); err != nil {
		t.Fatal(err)
	}
	return turn
}

func coordinationHTTPView(t *testing.T, h http.Handler, id string) map[string]any {
	t.Helper()
	var view map[string]any
	decodeEvidenceResponse(t, journeyRequest(t, h, "GET", "/v1/product/projects/"+id+"/orchestrator/coordination", nil, 200).Body.Bytes(), &view)
	return view
}

func coordinationRegisterTest(t *testing.T, h http.Handler, id, turn, key string, dependencies []string) map[string]any {
	t.Helper()
	view := coordinationHTTPView(t, h, id)
	for _, raw := range view["intents"].([]any) {
		existing := raw.(map[string]any)
		if existing["turnId"] == turn && len(dependencies) == 0 {
			return existing
		}
	}
	var intent map[string]any
	decodeEvidenceResponse(t, journeyRequest(t, h, "POST", "/v1/product/projects/"+id+"/orchestrator/coordination/intents", map[string]any{"expectedRevision": view["revision"], "expectedContextDigest": view["contextDigest"], "turnId": turn, "idempotencyKey": "coordination-test-" + key, "dependsOn": dependencies}, 201).Body.Bytes(), &intent)
	return intent
}

func coordinationApplyTest(t *testing.T, s *Server, id, actor, turn string, want int) {
	t.Helper()
	view, err := s.OrchestratorView(orchestratorActor(t, actor), id, 0)
	if err != nil {
		t.Fatal(err)
	}
	journeyRequest(t, coordinationHandler(s, actor), "POST", "/v1/product/projects/"+id+"/orchestrator/turns/"+turn+"/apply", map[string]any{"expectedRevision": view.Revision, "expectedContextDigest": view.Context.Digest}, want)
}

// This same HTTP body also runs against the pre-coordination product overlay.
// It must fail there at the absent route, not at compilation or a fixture stub.
func TestCoordinationHTTPDisjointIntentsPreserveBothPeopleAndReplay(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "shared-intent-http")
	s.SetCoordinatorAuthorizer(func(context.Context, string) error { return nil })
	a := coordinationProposalTest(t, s, p.ID, "alice", "purpose", "A shared notebook")
	b := coordinationProposalTest(t, s, p.ID, "bob", "audience", "Team editors")
	originalA, _ := s.OrchestratorTurn(p.ID, a.ID)
	originalB, _ := s.OrchestratorTurn(p.ID, b.ID)
	alice, bob := coordinationHandler(s, "alice"), coordinationHandler(s, "bob")
	ia := coordinationRegisterTest(t, alice, p.ID, a.ID, "alice-first", nil)
	ib := coordinationRegisterTest(t, bob, p.ID, b.ID, "bob-first", nil)
	if ia["luciId"] == ib["luciId"] || ia["baseContextDigest"] != ib["baseContextDigest"] {
		t.Fatal("personal origins or shared base lost")
	}
	if len(coordinationHTTPView(t, alice, p.ID)["conflicts"].([]any)) != 0 {
		t.Fatal("disjoint intentions require a vote")
	}
	coordinationApplyTest(t, s, p.ID, "alice", a.ID, 200)
	coordinationApplyTest(t, s, p.ID, "bob", b.ID, 200)
	coordinationApplyTest(t, s, p.ID, "bob", b.ID, 200)
	view, err := s.OrchestratorView(orchestratorActor(t, "alice"), p.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if view.Context.Specification.Purpose != "A shared notebook" || view.Context.Specification.Audience != "Team editors" || view.Context.Specification.Revision != 2 || view.Context.Specification.Status != "draft" {
		t.Fatal("lost update, duplicate apply or invented approval")
	}
	for _, original := range []OrchestratorTurn{originalA, originalB} {
		after, _ := s.OrchestratorTurn(p.ID, original.ID)
		if !reflect.DeepEqual(original.Context, after.Context) || !reflect.DeepEqual(original.Answer, after.Answer) {
			t.Fatal("original model input/proposal rewritten")
		}
	}
	final := coordinationHTTPView(t, alice, p.ID)
	if len(final["events"].([]any)) != 4 {
		t.Fatal("duplicate or missing causal acceptance")
	}
	for _, raw := range final["intents"].([]any) {
		intent := raw.(map[string]any)
		if intent["status"] != "applied" || intent["appliedTargetDigest"] == nil || intent["candidateProposalDigest"] == nil {
			t.Fatal("acceptance lacks exact canonical candidate")
		}
	}
	if strings.Contains(string(journeyRequest(t, alice, "GET", "/v1/product/projects/"+p.ID+"/orchestrator/coordination", nil, 200).Body.Bytes()), "keyDigest") {
		t.Fatal("private request key leaked")
	}
}
