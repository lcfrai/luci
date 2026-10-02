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

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
)

func TestManagedSessionCreateRestartForkAndCatalogUseCurrentPolicyWithinOperatorCeiling(t *testing.T) {
	env := sessionsNewEnv(t)
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	state := t.TempDir()
	gateway, err := management.New(management.Options{StateDir: state, InitialHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/machines" {
			env.api.handleMachines(w, r)
			return
		}
		env.mux.ServeHTTP(w, r)
	})})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = gateway.Close(context.Background()) })
	token := ""
	do := func(method, path, body string, status int) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, sessionsTarget(path), strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		gateway.Handler().ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s %s: %d want %d: %s", method, path, w.Code, status, w.Body.String())
		}
		return w
	}
	bootstrap, err := os.ReadFile(filepath.Join(state, "control-plane", "bootstrap-token"))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]string{"bootstrapToken": strings.TrimSpace(string(bootstrap)), "username": "owner", "displayName": "Owner", "password": "a long test password"})
	var identity struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(do("POST", "/v1/auth/bootstrap", string(body), 201).Body.Bytes(), &identity); err != nil {
		t.Fatal(err)
	}
	token = identity.Token
	first := sessionsDecodeOne(t, do("POST", "/v1/sessions", `{"worktreeId":"w-4a77b1c9","harness":"codex","authOnly":true}`, 201), 201)
	sessionsAwaitWatch(t, done)
	if got := env.be.spec(t, 0).Resources; got.MemoryMiB != 1024 || got.CPUMillis != 1000 {
		t.Fatalf("create ignored managed default: %#v", got)
	}
	do("PATCH", "/v1/admin/policy", `{"sessionMemoryMiB":2048,"sessionCPUMillis":1500}`, 200)
	do("POST", "/v1/sessions/"+first.ID+"/restart", `{}`, 200)
	sessionsAwaitWatch(t, done)
	second := env.be.spec(t, 1)
	if second.Resources.MemoryMiB != 2048 || second.Resources.CPUMillis != 1500 {
		t.Fatalf("restart used stale request policy: %#v", second.Resources)
	}
	nativeWrite(t, nativeStateMount(t, second), "sessions/2026/fork.jsonl", "owned history")
	do("DELETE", "/v1/sessions/"+first.ID, "", 200)
	env.api.deps.Resources = backend.Resources{MemoryMiB: 512, CPUMillis: 500}
	do("POST", "/v1/sessions", `{"worktreeId":"w-4a77b1c9","harness":"codex","resume":"`+first.ID+`"}`, 201)
	sessionsAwaitWatch(t, done)
	if got := env.be.spec(t, 2).Resources; got.MemoryMiB != 512 || got.CPUMillis != 500 {
		t.Fatalf("fork exceeded the operator ceiling: %#v", got)
	}
	var catalog machinesEnvelope
	if err := json.Unmarshal(do("GET", "/v1/machines", "", 200).Body.Bytes(), &catalog); err != nil || len(catalog.Machines) != 1 {
		t.Fatalf("invalid machine catalog: %#v %v", catalog, err)
	}
	if got := catalog.Machines[0]; got.SessionMemoryMiB != 512 || got.SessionCPUMillis != 500 {
		t.Fatalf("catalog did not report the effective ceiling: %#v", got)
	}
}
