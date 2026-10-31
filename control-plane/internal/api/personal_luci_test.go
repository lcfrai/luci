package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/assistance"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
)

func TestPersonalUtilitySessionIsAbsentFromEveryGenericReadAttachAndControl(t *testing.T) {
	e := sessionsNewEnv(t)
	for _, owner := range []string{strings.Repeat("a", 64), ""} {
		id := "s-01234567"
		if owner == "" {
			id = "s-01234568"
		}
		row := sessionsStarting(id)
		row.PersonalOwner, row.ContextRef, row.OpeningPrompt = owner, "luci:personal", "PRIVATE-PROMPT"
		sessionsSeed(t, e.st, row)
		for _, route := range []struct{ method, suffix, body string }{{"GET", "", ""}, {"GET", "/attach", ""}, {"GET", "/reviewed-context", ""}, {"GET", "/native-turn", ""}, {"PATCH", "", `{"title":"not allowed"}`}, {"DELETE", "", ""}, {"POST", "/restart", "{}"}} {
			w := e.do(t, route.method, "/v1/sessions/"+id+route.suffix, route.body)
			if w.Code != 404 || strings.Contains(w.Body.String(), "PRIVATE-PROMPT") {
				t.Fatal("generic route exposed or controlled personal session", route, w.Code, w.Body.String())
			}
		}
		retained, err := e.st.Get(id)
		if err != nil || retained.Title != row.Title {
			t.Fatal("private session mutated", err)
		}
	}
	list := e.do(t, "GET", "/v1/sessions", "")
	if list.Code != 200 || strings.Contains(list.Body.String(), "s-0123456") || e.be.destroyCount() != 0 || controlledAllocationCount(e.be) != 0 {
		t.Fatal("generic list or control crossed private boundary", list.Body.String())
	}
	legacy := sessionsStarting("s-01234569")
	sessionsSeed(t, e.st, legacy)
	if w := e.do(t, "GET", "/v1/sessions/"+legacy.ID, ""); w.Code != 200 {
		t.Fatal("legacy generic access changed", w.Code)
	}
}

func TestPersonalJobCannotBeReadCancelledAppliedOrReconciledThroughComputeProject(t *testing.T) {
	f := newSwarmSessionFixtureTasks(t)
	job := assistance.Job{ID: "assist-personal-private", Kind: "personal-luci", PersonalOwner: strings.Repeat("a", 64), PersonalTurnID: "personal-turn-private", RequestedBy: "alice", ProjectID: f.owner.ID, Status: "queued", CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), UserMessage: "PRIVATE-HISTORY", PersonalContext: json.RawMessage(`{"private":"PRIVATE-HISTORY"}`)}
	if err := f.env.api.assistanceStore().Create(job); err != nil {
		t.Fatal(err)
	}
	base := "/v1/product/projects/" + f.owner.ID + "/assist/" + job.ID
	for _, test := range []struct{ method, suffix, body string }{{"GET", "", ""}, {"POST", "/cancel", "{}"}, {"POST", "/reconcile", "{}"}, {"POST", "/apply", `{"expectedRevision":0,"expectedBriefRevision":0}`}} {
		w := f.env.do(t, test.method, base+test.suffix, test.body)
		if w.Code != 404 || strings.Contains(w.Body.String(), "PRIVATE-HISTORY") {
			t.Fatal("project grant exposed private job", test, w.Code)
		}
	}
	if w := f.env.do(t, "GET", "/v1/product/projects/"+f.owner.ID+"/assist", ""); w.Code != 200 || strings.Contains(w.Body.String(), job.ID) || strings.Contains(w.Body.String(), "PRIVATE-HISTORY") {
		t.Fatal("private job listed", w.Code, w.Body.String())
	}
	retained, err := f.env.api.assistanceStore().Read(job.ID)
	if err != nil || retained.Status != "queued" || controlledAllocationCount(f.env.be) != 0 {
		t.Fatal("private job changed through shared controls", retained, err)
	}
}

func TestPersonalComputeAuthorityNeedsExactActorTeamProjectAndSecrets(t *testing.T) {
	f := newSwarmSessionFixtureTasks(t)
	ctx := orchestratorExecutionActor(t)
	owner, err := f.env.api.deps.Product.PersonalScope(ctx)
	if err != nil {
		t.Fatal(err)
	}
	prefs := product.PersonalPreferences{ComputeProjectID: f.owner.ID, Harness: "codex", Model: "gpt-5.4", ConnectionID: f.connectionID, ProjectIDs: []string{}, GoalIDs: []string{}}
	allow := func(context.Context, string, bool) error { return nil }
	view, err := f.env.api.deps.Product.SavePersonalPreferences(ctx, 0, prefs, allow)
	if err != nil {
		t.Fatal(err)
	}
	turn, _, err := f.env.api.deps.Product.ReservePersonalTurn(ctx, product.PersonalTurnRequest{ExpectedRevision: view.Revision, ExpectedContextDigest: view.Context.Digest, IdempotencyKey: "private-compute-original-123456", Message: "Private compute authority"}, "assist-compute-authority", allow)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(turn.Context)
	actor, _, _ := management.ActorFromContext(ctx)
	job := assistance.Job{ID: turn.JobID, Kind: "personal-luci", PersonalTurnID: turn.ID, PersonalOwner: owner, PersonalContext: raw, ProjectID: f.owner.ID, Harness: "codex", RequestedBy: actor.ID}
	calls := 0
	f.env.api.deps.AuthorizeExecution = func(_ context.Context, ref management.ExecutionActorRef, harness string, secrets bool) (management.ExecutionAuthority, error) {
		calls++
		if ref.UserID != actor.ID || ref.ProjectID != f.owner.ID || harness != "codex" || !secrets {
			t.Fatal("authority scope widened", ref, harness, secrets)
		}
		return management.ExecutionAuthority{}, management.ErrExecutionAuthority
	}
	if _, err := f.env.api.authorizePersonalJob(ctx, job); err == nil || calls != 1 || controlledAllocationCount(f.env.be) != 0 {
		t.Fatal("readable private context substituted execution/secret authority", err, calls)
	}
	job.PersonalOwner = "malformed"
	if _, err := f.env.api.authorizePersonalJob(ctx, job); err == nil || calls != 1 {
		t.Fatal("malformed owner reached privileged resolver", err)
	}
}

func TestPersonalClosedResponseRejectsUnincludedProjectAndUnknownActions(t *testing.T) {
	snapshot := product.PersonalContext{Preferences: product.PersonalPreferences{ProjectIDs: []string{"project-one"}}}
	raw, _ := json.Marshal(snapshot)
	job := assistance.Job{Kind: "personal-luci", PersonalOwner: strings.Repeat("a", 64), PersonalContext: raw}
	for _, answer := range []string{`{"summary":"Hi","question":"","options":[],"discussion":{"projectId":"project-other","message":"unselected"}}`, `{"summary":"Hi","question":"","options":[],"discussion":null,"execute":true}`} {
		if _, err := decodePersonalAnswer(job, []byte(answer)); err == nil {
			t.Fatal("model widened project or action scope")
		}
	}
	if _, err := decodePersonalAnswer(job, []byte(`{"summary":"Review the next decision","question":"","options":[],"discussion":null}`)); err != nil {
		t.Fatal("bounded discussion answer rejected", err)
	}
}

func TestPersonalExplicitRecoveryClosesOnlyThePositivelyAbsentOriginalJob(t *testing.T) {
	for _, scenario := range []string{"resolve", "cancel", "store-error", "revoked"} {
		t.Run(scenario, func(t *testing.T) {
			f := newSwarmSessionFixtureTasks(t)
			ctx := orchestratorExecutionActor(t)
			actor, _, _ := management.ActorFromContext(ctx)
			prefs := product.PersonalPreferences{ComputeProjectID: f.owner.ID, Harness: "codex", Model: "saved-model", ConnectionID: f.connectionID, ProjectIDs: []string{}, GoalIDs: []string{}}
			allow := func(context.Context, string, bool) error { return nil }
			view, err := f.env.api.deps.Product.SavePersonalPreferences(ctx, 0, prefs, allow)
			if err != nil {
				t.Fatal(err)
			}
			turn, _, err := f.env.api.deps.Product.ReservePersonalTurn(ctx, product.PersonalTurnRequest{ExpectedRevision: view.Revision, ExpectedContextDigest: view.Context.Digest, IdempotencyKey: "crashed-before-original-job-create", Message: "Keep this original"}, "assist-original-missing", allow)
			if err != nil {
				t.Fatal(err)
			}
			writes, reads, grants := 0, 0, 0
			f.env.api.assistStore = assistance.NewStore(func(string) ([]byte, error) {
				reads++
				if scenario == "store-error" {
					return nil, errors.New("job store read unconfirmed")
				}
				return nil, os.ErrNotExist
			}, func(string, []byte) error { writes++; return errors.New("must not create or rewrite assistance job") })
			f.env.api.deps.AuthorizeExecution = func(_ context.Context, ref management.ExecutionActorRef, harness string, secrets bool) (management.ExecutionAuthority, error) {
				grants++
				if ref.UserID != actor.ID || ref.ProjectID != f.owner.ID || ref.TeamID != "" || harness != "codex" || !secrets {
					t.Fatal("recovery authority changed original scope", ref, harness, secrets)
				}
				if scenario == "revoked" {
					return management.ExecutionAuthority{}, management.ErrExecutionAuthority
				}
				return management.ExecutionAuthority{Actor: actor}, nil
			}
			request := func(method string) *http.Request {
				r := httptest.NewRequest(method, "/v1/product/personal-luci/turns/"+turn.ID, nil).WithContext(ctx)
				r.SetPathValue("turnId", turn.ID)
				return r
			}
			status := httptest.NewRecorder()
			f.env.api.handlePersonalStatus(status, request("GET"))
			unchanged, err := f.env.api.deps.Product.PersonalTurn(ctx, turn.ID, allow)
			if status.Code != 503 || err != nil || unchanged.Status != "reserved" {
				t.Fatal("GET mutated or inferred missing job success", status.Code, unchanged, err)
			}
			response := httptest.NewRecorder()
			if scenario == "cancel" {
				f.env.api.handlePersonalCancel(response, request("POST"))
			} else {
				f.env.api.handlePersonalResolve(response, request("POST"))
			}
			retained, err := f.env.api.deps.Product.PersonalTurn(ctx, turn.ID, allow)
			if err != nil {
				t.Fatal(err)
			}
			if writes != 0 || controlledAllocationCount(f.env.be) != 0 || f.env.be.destroyCount() != 0 {
				t.Fatal("absence recovery allocated or controlled a resource", writes)
			}
			if scenario == "store-error" || scenario == "revoked" {
				if response.Code < 400 || retained.Status != "reserved" {
					t.Fatal("unconfirmed absence or current denial retired original", response.Code, retained)
				}
				return
			}
			want := "failed"
			if scenario == "cancel" {
				want = "cancelled"
			}
			if response.Code != 200 || retained.Status != want || retained.ID != turn.ID || retained.JobID != turn.JobID || grants != 1 || reads < 3 {
				t.Fatal("original gap did not settle exactly once", response.Code, retained, grants, reads)
			}
			current, err := f.env.api.deps.Product.PersonalView(ctx, 0, allow)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := f.env.api.deps.Product.ContinuePersonalConversation(ctx, current.Revision, current.ConversationID, allow); err != nil {
				t.Fatal("closed original still stranded same-workspace continuation", err)
			}
		})
	}
}
