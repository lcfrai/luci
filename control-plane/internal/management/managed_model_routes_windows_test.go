//go:build windows

package management

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/lcfrai/luci/control-plane/internal/managedmodel"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func managedRouteFixture(t *testing.T) (*Gateway, *managedModelManager) {
	t.Helper()
	g := &Gateway{dir: t.TempDir(), state: storedState{Users: map[string]userRecord{"admin": {User: User{ID: "admin", Role: "admin"}}, "member": {User: User{ID: "member", Role: "member"}}}}, opts: Options{ManagedModelDependents: func(context.Context, string, string, bool) error { return nil }}}
	m := &managedModelManager{gateway: g, platform: inertManagedPlatform{}, path: filepath.Join(g.dir, "status.json"), boot: "boot", status: managedmodel.Status{State: "disabled", Generation: "0", ObservedAt: time.Now().UTC()}}
	g.localModels = m
	return g, m
}
func modelRequest(g *Gateway, method, path, actor, role string, body any) *httptest.ResponseRecorder {
	raw, _ := json.Marshal(body)
	r := httptest.NewRequest(method, path, bytes.NewReader(raw))
	r = r.WithContext(withActor(r.Context(), User{ID: actor, Role: role}, Policy{}))
	w := httptest.NewRecorder()
	mux := http.NewServeMux()
	g.registerManagedModel(mux)
	mux.ServeHTTP(w, r)
	return w
}
func TestManagedModelSetupSaveIsScopedRevisionedAndHasNoLaunchSideEffect(t *testing.T) {
	g, m := managedRouteFixture(t)
	body := map[string]any{"expectedRevision": 0, "enabled": true, "startWithLuci": false, "executablePath": `C:\Tools\Ollama\ollama.exe`, "modelStore": `E:\Models`, "port": 24567, "model": "installed:latest"}
	if w := modelRequest(g, "POST", "/v1/setup/local-model", "member", "member", body); w.Code != 403 {
		t.Fatal("member changed host settings", w.Code)
	}
	if w := modelRequest(g, "POST", "/v1/setup/local-model", "member", "admin", body); w.Code != 403 {
		t.Fatal("stale or private actor promoted host authority", w.Code)
	}
	w := modelRequest(g, "POST", "/v1/setup/local-model", "admin", "admin", body)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var snapshot struct {
		Available, CanManage bool
		Revision             int
		Config               managedModelConfig
		Status               managedmodel.Status
	}
	if json.Unmarshal(w.Body.Bytes(), &snapshot) != nil || !snapshot.Available || !snapshot.CanManage || snapshot.Revision != 1 || snapshot.Status.State != "stopped" || snapshot.Status.Generation != "0" {
		t.Fatal("mutation did not return the agreed full snapshot", w.Body.String())
	}
	if m.owner != nil || m.done != nil || m.saved.AuthorizingActorID != "admin" {
		t.Fatal("Save launched a process or failed to retain authorizing actor")
	}
	if w = modelRequest(g, "POST", "/v1/setup/local-model", "admin", "admin", body); w.Code != 409 || m.saved.Revision != 1 {
		t.Fatal("stale revision overwrote config")
	}
}
func TestManagedModelStatusRedactsPathsAndCurrentActorRevocationBlocksStart(t *testing.T) {
	g, m := managedRouteFixture(t)
	m.saved = managedModelSaved{Revision: 1, AuthorizingActorID: "admin", Config: managedModelConfig{Enabled: true, ExecutablePath: `C:\Tools\Ollama\ollama.exe`, ModelStore: `E:\Models`, Port: 24567, Model: "installed:latest", Profile: managedmodel.FixedProfile()}}
	w := modelRequest(g, "GET", "/v1/setup/local-model", "member", "member", nil)
	var out map[string]any
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &out) != nil {
		t.Fatal(w.Code)
	}
	config := out["config"].(map[string]any)
	if _, ok := config["executablePath"]; ok {
		t.Fatal("member received host executable path")
	}
	if _, ok := config["modelStore"]; ok {
		t.Fatal("member received model-store path")
	}
	row := g.state.Users["admin"]
	row.User.Disabled = true
	g.state.Users["admin"] = row
	if _, e := m.beginStart(1); e == nil || m.owner != nil || m.done != nil || m.status.Failure == nil || m.status.Failure.Code != "authority_changed" {
		t.Fatal("revoked persisted authority started a service")
	}
}

type delayedManagedStart struct{ entered, cancelled chan struct{} }

func (s delayedManagedStart) start(ctx context.Context, _ managedModelConfig, _ uint64) (managedModelService, error) {
	close(s.entered)
	<-ctx.Done()
	close(s.cancelled)
	return nil, ctx.Err()
}
func TestManagedCloseCancelsDelayedStartupBeforeDependentWaitWithoutNewBudget(t *testing.T) {
	g, m := managedRouteFixture(t)
	start := delayedManagedStart{make(chan struct{}), make(chan struct{})}
	m.platform = start
	m.status.Generation = "7"
	m.status.State = "starting"
	m.saved.Revision = 1
	run, cancelRun := context.WithCancel(context.Background())
	m.cancel = cancelRun
	m.done = make(chan struct{})
	done := m.done
	go m.run(run, managedModelConfig{authorize: func() error { return nil }}, 7, done)
	<-start.entered
	deadline := time.Now().Add(time.Second)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	g.opts.ManagedModelDependents = func(c context.Context, _ string, _ string, _ bool) error {
		select {
		case <-start.cancelled:
		case <-c.Done():
			return errors.New("startup was not cancelled before dependent wait")
		}
		d, _ := c.Deadline()
		if !d.Equal(deadline) {
			t.Fatal("dependent cleanup received a new budget")
		}
		return nil
	}
	if e := m.stopExpected(ctx, true, 1, "7"); e != nil {
		t.Fatal(e)
	}
	select {
	case <-done:
	default:
		t.Fatal("startup original owner did not settle before close")
	}
}
func TestManagedControlPublicationDoesNotRenewStaleReadiness(t *testing.T) {
	_, m := managedRouteFixture(t)
	m.boot = strings.Repeat("a", 64)
	m.saved.Revision = 1
	observed := time.Now().Add(-6 * time.Second).UTC()
	m.status = managedmodel.Status{State: "available", Generation: "7", ObservedAt: observed, Endpoint: "http://127.0.0.1:24567/v1", Model: "installed:latest"}
	m.withdrawing = true
	if e := m.publishLocked(); e != nil {
		t.Fatal(e)
	}
	m.withdrawing = false
	if e := m.publishLocked(); e != nil {
		t.Fatal(e)
	}
	if e := managedmodel.Configure(m.path, m.boot); e != nil {
		t.Fatal(e)
	}
	defer managedmodel.ReleaseAuthority(m.path, m.boot)
	original, e := managedmodel.Read(m.path)
	if e != nil || !original.Status.ObservedAt.Equal(observed) {
		t.Fatal("control-only status republish invented a new observation", e)
	}
	if _, e = managedmodel.Check(&managedmodel.Reference{Revision: 1}, m.status.Endpoint, m.status.Model); e == nil {
		t.Fatal("stale readiness was admitted after failed-stop restoration")
	}
}
