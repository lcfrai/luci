package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

func TestDeploymentBlueprintAuthorizationAndBoundedPlanOnlyResponse(t *testing.T) {
	projects := t.TempDir()
	dir := filepath.Join(projects, "example")
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	server, err := product.New(product.Options{StateDir: t.TempDir(), ProjectsDir: projects})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	a := &API{deps: Deps{Product: server}}
	id := project.ProjectID(dir)
	blueprint := deploymentplan.Blueprint{Version: 1, ProjectID: id, Infrastructure: deploymentplan.Infrastructure{ID: "cluster", Kind: "kubernetes"}, Services: []deploymentplan.Service{{ID: "web", Kind: "app"}}}
	raw, err := json.Marshal(blueprint)
	if err != nil {
		t.Fatal(err)
	}
	request := func(ctx context.Context, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/v1/product/projects/"+id+"/deployment-blueprint", strings.NewReader(body)).WithContext(ctx)
		r.SetPathValue("id", id)
		w := httptest.NewRecorder()
		a.handleDeploymentBlueprint(w, r)
		return w
	}
	if got := request(context.Background(), string(raw)); got.Code != http.StatusForbidden {
		t.Fatalf("anonymous planning accepted: %d", got.Code)
	}
	actor := orchestratorExecutionActor(t)
	a.deps.AuthorizeProjectRead = func(context.Context, management.ExecutionActorRef) (management.ExecutionAuthority, error) {
		return management.ExecutionAuthority{}, management.ErrExecutionAuthority
	}
	if got := request(actor, string(raw)); got.Code != http.StatusForbidden {
		t.Fatalf("denied planning accepted: %d", got.Code)
	}
	calls := 0
	a.deps.AuthorizeProjectRead = func(_ context.Context, ref management.ExecutionActorRef) (management.ExecutionAuthority, error) {
		calls++
		if ref.ProjectID != id {
			t.Fatal("wrong project authorization")
		}
		return management.ExecutionAuthority{}, nil
	}
	got := request(actor, string(raw))
	var plan deploymentplan.Plan
	if got.Code != 200 || json.Unmarshal(got.Body.Bytes(), &plan) != nil || plan.State != "decisions-required" || len(plan.Artifacts) != 0 || plan.SourceVerified || plan.EffectsApplied || calls != 2 || got.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("unexpected plan-only result: %d %s calls=%d", got.Code, got.Body.String(), calls)
	}
	for _, body := range []string{string(raw) + " {}", `{"unknown":true}`, strings.Repeat(" ", 128<<10) + string(raw)} {
		if got := request(actor, body); got.Code != 400 {
			t.Fatalf("non-strict/oversize JSON accepted: %d", got.Code)
		}
	}
	blueprint.ProjectID = "another-project"
	mismatch, _ := json.Marshal(blueprint)
	if got := request(actor, string(mismatch)); got.Code != 422 {
		t.Fatalf("cross-project blueprint accepted: %d", got.Code)
	}
	calls = 0
	a.deps.AuthorizeProjectRead = func(context.Context, management.ExecutionActorRef) (management.ExecutionAuthority, error) {
		calls++
		if calls == 1 {
			return management.ExecutionAuthority{}, nil
		}
		return management.ExecutionAuthority{}, management.ErrExecutionAuthority
	}
	if got := request(actor, string(raw)); got.Code != 403 || calls != 2 {
		t.Fatalf("late access revocation disclosed plan: %d calls=%d", got.Code, calls)
	}
}
