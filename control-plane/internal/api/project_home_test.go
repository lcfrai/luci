package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/project"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

func TestHomeFiltersCurrentReadGrantsBeforeProjectActivity(t *testing.T) {
	e := sessionsNewEnv(t)
	owned, denied := "p-8c1d0e42", "p-hidden"
	e.loc.projects = append(e.loc.projects, project.Project{ID: denied, Name: "Private denied project", Path: t.TempDir()})
	server, err := product.New(product.Options{StateDir: t.TempDir(), ProjectsDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	e.api.deps.Product = server
	allowed := map[string]bool{owned: true}
	e.api.deps.AuthorizeProjectRead = func(_ctx context.Context, ref management.ExecutionActorRef) (management.ExecutionAuthority, error) {
		if !allowed[ref.ProjectID] {
			return management.ExecutionAuthority{}, management.ErrExecutionAuthority
		}
		return management.ExecutionAuthority{}, nil
	}
	request := func(method, path, body string, serve func(http.ResponseWriter, *http.Request)) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, sessionsTarget(path), strings.NewReader(body)).WithContext(orchestratorExecutionActor(t))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		serve(w, r)
		return w
	}
	initial := request("GET", "/v1/product/home", "", e.api.handleProjectHome)
	if initial.Code != 200 || strings.Contains(initial.Body.String(), denied) || strings.Contains(initial.Body.String(), "Private denied") {
		t.Fatalf("denied project leaked: %d %s", initial.Code, initial.Body.String())
	}
	var page homeResponse
	if err := json.Unmarshal(initial.Body.Bytes(), &page); err != nil || page.Total != 1 || len(page.Projects) != 1 || page.Projects[0].ID != owned {
		t.Fatalf("wrong authorized Home: %+v %v", page, err)
	}
	pin := request("PUT", "/v1/product/home/pins", `{"projectId":"p-8c1d0e42","expectedRevision":1,"pinned":true}`, e.api.handleHomePin)
	if pin.Code != 200 {
		t.Fatalf("pin: %d %s", pin.Code, pin.Body.String())
	}
	allowed[owned] = false // revocation before either read or mutation
	hidden := request("GET", "/v1/product/home", "", e.api.handleProjectHome)
	if hidden.Code != 200 || strings.Contains(hidden.Body.String(), owned) || strings.Contains(hidden.Body.String(), "luci") {
		t.Fatalf("revoked project leaked: %d %s", hidden.Code, hidden.Body.String())
	}
	deniedPin := request("PUT", "/v1/product/home/pins", `{"projectId":"p-8c1d0e42","expectedRevision":2,"pinned":false}`, e.api.handleHomePin)
	if deniedPin.Code != 403 {
		t.Fatalf("revoked pin accepted: %d %s", deniedPin.Code, deniedPin.Body.String())
	}
	calls := 0
	e.api.deps.AuthorizeProjectRead = func(_ctx context.Context, ref management.ExecutionActorRef) (management.ExecutionAuthority, error) {
		if ref.ProjectID == owned {
			calls++
			if calls == 1 {
				return management.ExecutionAuthority{}, nil
			}
		}
		return management.ExecutionAuthority{}, management.ErrExecutionAuthority
	}
	changedDuringWrite := request("PUT", "/v1/product/home/pins", `{"projectId":"p-8c1d0e42","expectedRevision":2,"pinned":false}`, e.api.handleHomePin)
	if changedDuringWrite.Code != 403 || calls != 2 {
		t.Fatalf("late revocation passed the effect-time check: %d %s calls=%d", changedDuringWrite.Code, changedDuringWrite.Body.String(), calls)
	}
}

func TestHomeRetainsWorkingAndStoppedSessionsTogetherWithoutObservingBackend(t *testing.T) {
	e := sessionsNewEnv(t)
	server, err := product.New(product.Options{StateDir: t.TempDir(), ProjectsDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	e.api.deps.Product = server
	live := sessionsFullRecord()
	live.ID = "s-home-running"
	live.Status = registry.StatusRunning
	stopped := sessionsFullRecord()
	stopped.ID = "s-home-stopped"
	stopped.SandboxID = "s-home-stopped"
	stopped.SandboxRef = "owned-stopped-generation"
	stopped.Status = registry.StatusExited
	stopped.StoppedAt = time.Now().UTC().Add(-time.Hour)
	stopped.UpdatedAt = stopped.StoppedAt
	if err := e.st.Create(live); err != nil {
		t.Fatal(err)
	}
	if err := e.st.Create(stopped); err != nil {
		t.Fatal(err)
	}
	rows, err := e.api.homePage(orchestratorExecutionActor(t), e.loc.projects, "u-reviewed-task-fixture", product.HomePreferences{AcknowledgedStopIDs: map[string][]string{}}, time.Now().UTC(), true)
	if err != nil || len(rows) != 1 || !rows[0].Working || len(rows[0].Stops) != 1 || rows[0].Stops[0].Kind != "stopped" {
		t.Fatalf("working/stopped activity lost: %+v %v", rows, err)
	}
}
