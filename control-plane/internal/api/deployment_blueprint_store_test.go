package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

func TestSavedBlueprintAPIRequiresCurrentIdentityAndWithholdsLateRevokedRead(t *testing.T) {
	projects := t.TempDir()
	dir := filepath.Join(projects, "saved-blueprint")
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	s, err := product.New(product.Options{StateDir: t.TempDir(), ProjectsDir: projects})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := &API{deps: Deps{Product: s}}
	id, ctx := project.ProjectID(dir), orchestratorExecutionActor(t)
	request := func(method string, ctx context.Context, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/v1/product/projects/"+id+"/deployment-blueprint/saved", strings.NewReader(body)).WithContext(ctx)
		r.SetPathValue("id", id)
		w := httptest.NewRecorder()
		a.handleSavedDeploymentBlueprint(w, r)
		return w
	}
	if got := request(http.MethodGet, context.Background(), ""); got.Code != 403 {
		t.Fatal("anonymous private read accepted", got.Code)
	}
	actor, policy, _ := management.ActorFromContext(ctx)
	calls := 0
	a.deps.AuthorizeProjectRead = func(_ context.Context, ref management.ExecutionActorRef) (management.ExecutionAuthority, error) {
		calls++
		if ref.ProjectID != id || ref.UserID != actor.ID {
			t.Fatal("wrong identity/project")
		}
		return management.ExecutionAuthority{Actor: actor, Policy: policy}, nil
	}
	if got := request(http.MethodGet, ctx, ""); got.Code != 200 || !strings.Contains(got.Body.String(), `"saved":null`) || got.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatal("empty saved view failed", got.Code, got.Body.String())
	}
	if got := request(http.MethodPut, ctx, `{"unknown":true}`); got.Code != 400 {
		t.Fatal("unknown save field accepted", got.Code)
	}
	if got := request(http.MethodPut, ctx, strings.Repeat(" ", 136<<10)+`{}`); got.Code != 400 {
		t.Fatal("oversize save accepted", got.Code)
	}
	calls = 0
	a.deps.AuthorizeProjectRead = func(context.Context, management.ExecutionActorRef) (management.ExecutionAuthority, error) {
		calls++
		if calls >= 3 {
			return management.ExecutionAuthority{}, management.ErrExecutionAuthority
		}
		return management.ExecutionAuthority{Actor: actor, Policy: policy}, nil
	}
	if got := request(http.MethodGet, ctx, ""); got.Code != 403 || strings.Contains(got.Body.String(), `"saved"`) {
		t.Fatal("late revocation disclosed saved view", got.Code, got.Body.String())
	}
}
