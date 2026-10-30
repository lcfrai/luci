package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

func TestPortfolioFiltersProjectsAndRechecksRevocationBeforeResponse(t *testing.T) {
	e := sessionsNewEnv(t)
	server, err := product.New(product.Options{StateDir: t.TempDir(), ProjectsDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	e.api.deps.Product = server
	e.loc.projects = append(e.loc.projects, project.Project{ID: "p-hidden", Name: "Hidden private name", Path: t.TempDir()})
	calls := 0
	revoke := false
	revokeAfter := 1
	e.api.deps.AuthorizeProjectRead = func(_ context.Context, ref management.ExecutionActorRef) (management.ExecutionAuthority, error) {
		if ref.ProjectID == "p-hidden" {
			return management.ExecutionAuthority{}, management.ErrExecutionAuthority
		}
		calls++
		if revoke && calls > revokeAfter {
			return management.ExecutionAuthority{}, management.ErrExecutionAuthority
		}
		return management.ExecutionAuthority{}, nil
	}
	read := func() *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", sessionsTarget("/v1/product/portfolio"), nil).WithContext(orchestratorExecutionActor(t))
		w := httptest.NewRecorder()
		e.api.handlePortfolio(w, r)
		return w
	}
	w := read()
	if w.Code != 200 || strings.Contains(w.Body.String(), "p-hidden") || strings.Contains(w.Body.String(), "Hidden private") {
		t.Fatalf("denied project leaked: %d %s", w.Code, w.Body.String())
	}
	var view product.Portfolio
	if json.Unmarshal(w.Body.Bytes(), &view) != nil || len(view.Projects) != 1 || view.TotalProjects != 1 {
		t.Fatalf("invalid portfolio: %s", w.Body.String())
	}
	for _, revokeAfterRead := range []int{1, 2} {
		calls, revoke, revokeAfter = 0, true, revokeAfterRead
		w = read()
		if w.Code != 403 || strings.Contains(w.Body.String(), "p-8c1d0e42") || calls != revokeAfterRead+1 {
			t.Fatalf("late revocation leaked evidence: %d %s calls=%d", w.Code, w.Body.String(), calls)
		}
	}
}

func TestPortfolioRequiresCurrentActorAndDoesNotSubstituteAnonymousScope(t *testing.T) {
	e := sessionsNewEnv(t)
	server, err := product.New(product.Options{StateDir: t.TempDir(), ProjectsDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	e.api.deps.Product = server
	w := httptest.NewRecorder()
	e.api.handlePortfolio(w, httptest.NewRequest("GET", sessionsTarget("/v1/product/portfolio"), nil))
	if w.Code != 503 || strings.Contains(w.Body.String(), "p-8c1d0e42") {
		t.Fatalf("anonymous projection: %d %s", w.Code, w.Body.String())
	}
}
