package api

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

func TestCoordinatorDeploymentPreparationUsesFreshAPICompositionWithoutAllocation(t *testing.T) {
	f, env := newPreviewCancelHTTPFixture(t)
	response := env.do(t, "POST", "/v1/product/projects", `{"name":"deployment-authority"}`)
	var p project.Project
	if response.Code != 201 || json.Unmarshal(response.Body.Bytes(), &p) != nil {
		t.Fatal("project", response.Code)
	}
	path := "/v1/product/projects/" + p.ID + "/orchestrator/deployment-plan"
	calls := 0
	revoked, ready := false, true
	env.api.deps.ExecutionReady = func() bool { return ready }
	env.api.deps.AuthorizeDeployment = func(ctx context.Context, ref management.ExecutionActorRef, allocate bool) (management.ExecutionAuthority, error) {
		calls++
		if allocate || ref.UserID != f.actor.ID || ref.ProjectID != p.ID || ref.Local {
			t.Fatal("preparation allocated or lost actor scope", ref, allocate)
		}
		if revoked {
			return management.ExecutionAuthority{}, management.ErrExecutionAuthority
		}
		return management.ExecutionAuthority{Actor: f.actor, Policy: f.policy}, nil
	}
	// A missing target can be inspected only after current trusted Deploy is
	// resolved through the API composition; no provider client or mutation runs.
	if got := f.request(env, "POST", path, `{"targetId":"missing-target"}`).Code; got != 409 || calls != 1 {
		t.Fatal("fresh composition", got, calls)
	}
	revoked = true
	if got := f.request(env, "POST", path, `{"targetId":"missing-target"}`).Code; got != 403 || calls != 2 {
		t.Fatal("revocation ignored", got, calls)
	}
	revoked = false
	ready = false
	if got := f.request(env, "POST", path, `{"targetId":"missing-target"}`).Code; got != 403 || calls != 2 {
		t.Fatal("service readiness bypassed", got, calls)
	}
	ready = true
	env.api.deps.AuthorizeDeployment = nil
	if got := f.request(env, "POST", path, `{"targetId":"missing-target"}`).Code; got != 403 {
		t.Fatal("missing resolver reused stale header", got)
	}
	f.backend.mu.Lock()
	defer f.backend.mu.Unlock()
	if f.backend.builds != 0 || f.backend.creates != 0 || f.backend.starts != 0 || f.backend.removes != 0 {
		t.Fatal("read-only preparation allocated runtime work")
	}
}
