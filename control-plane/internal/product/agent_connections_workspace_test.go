package product

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/managedmodel"
)

func TestManagedConnectionThinkingSaveReloadPreservesRuntimeAndReference(t *testing.T) {
	path := filepath.Join(t.TempDir(), "status.json")
	boot := strings.Repeat("b", 64)
	if err := managedmodel.Configure(path, boot); err != nil {
		t.Fatal(err)
	}
	defer managedmodel.ReleaseAuthority(path, boot)
	observation := managedmodel.Observation{Version: 1, Boot: boot, Revision: 2, Status: managedmodel.Status{State: "available", Generation: "7", ObservedAt: time.Now().UTC(), Endpoint: "http://127.0.0.1:24567/v1", Model: "installed:latest"}}
	raw, err := json.Marshal(observation)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	s, mux := journeyFixture(t)
	s.registerSetupTools(mux)
	body := map[string]any{"action": "save", "expectedRevision": 0, "name": "Managed local", "kind": "compatible-api", "enabled": true,
		"endpoints": AgentConnectionEndpoints{ChatCompletions: observation.Status.Endpoint}, "models": AgentConnectionModels{OpenCode: observation.Status.Model},
		"openCodeService": "local-ollama", "openCodeThinking": "", "managedLocalModel": managedmodel.Reference{Revision: 2}, "apiKey": ""}
	catalogue := connectionSave(t, mux, body)
	id := catalogue.Connections[0].ID
	firstReceipt, defaultEnv, err := s.ResolveAgentConnection("opencode", &id)
	if err != nil || catalogue.Connections[0].OpenCodeThinking != "" || firstReceipt.ManagedLocalModel == nil || firstReceipt.ManagedLocalModel.Revision != 2 {
		t.Fatal("explicit managed provider default was not preserved", err)
	}
	body["id"], body["expectedRevision"], body["openCodeThinking"] = id, catalogue.Revision, "off"
	catalogue = connectionSave(t, mux, body)
	offReceipt, offEnv, err := s.ResolveAgentConnection("opencode", &id)
	if err != nil || offReceipt.Revision != firstReceipt.Revision+1 || firstReceipt.Equal(offReceipt) {
		t.Fatal("thinking save did not advance the connection receipt", err)
	}
	var offConfig map[string]any
	if json.Unmarshal([]byte(offEnv["OPENCODE_CONFIG_CONTENT"]), &offConfig) != nil {
		t.Fatal("Off runtime config is invalid")
	}
	model := offConfig["provider"].(map[string]any)["luci-gateway"].(map[string]any)["models"].(map[string]any)[observation.Status.Model].(map[string]any)
	if model["options"].(map[string]any)["reasoningEffort"] != "none" {
		t.Fatal("Off did not retain its existing adapter option")
	}
	oldOffConfig := offEnv["OPENCODE_CONFIG_CONTENT"]
	body["expectedRevision"], body["openCodeThinking"] = catalogue.Revision, ""
	catalogue = connectionSave(t, mux, body)
	restarted, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot()})
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	receipt, env, err := restarted.ResolveAgentConnection("opencode", &id)
	if err != nil || receipt.Revision != offReceipt.Revision+1 || receipt.ManagedLocalModel == nil || *receipt.ManagedLocalModel != *firstReceipt.ManagedLocalModel || receipt.Equal(offReceipt) {
		t.Fatal("reloaded thinking choice changed its managed reference or revision", err)
	}
	if env["OPENCODE_CONFIG_CONTENT"] != defaultEnv["OPENCODE_CONFIG_CONTENT"] || offEnv["OPENCODE_CONFIG_CONTENT"] != oldOffConfig || catalogue.ActiveConnectionID != "" || catalogue.Connections[0].KeyPresent {
		t.Fatal("provider default changed earlier runtime config, selection or private-key boundaries")
	}
	var config map[string]any
	if json.Unmarshal([]byte(env["OPENCODE_CONFIG_CONTENT"]), &config) != nil {
		t.Fatal("default runtime config is invalid")
	}
	model = config["provider"].(map[string]any)["luci-gateway"].(map[string]any)["models"].(map[string]any)[observation.Status.Model].(map[string]any)
	if _, exists := model["options"]; exists {
		t.Fatal("provider default silently forced a reasoning option")
	}
	if _, _, err := restarted.ResolveAgentConnection("claude-code", &id); err == nil {
		t.Fatal("managed thinking choice enabled another harness")
	}
	if _, err := os.Stat(filepath.Join(s.stateDir, "agent-credentials.json")); !os.IsNotExist(err) {
		t.Fatal("managed thinking save created personal credentials")
	}
}

func TestAgentConnectionsFollowSelectedWorkspaceWithoutLocalFallback(t *testing.T) {
	local, localMux := journeyFixture(t)
	local.registerSetupTools(localMux)
	backing, backingMux := journeyFixture(t)
	backing.registerSetupTools(backingMux)
	localState := connectionSave(t, localMux, connectionSaveBody(0))
	remoteState := connectionSave(t, backingMux, connectionSaveBody(0))
	localID, remoteID := localState.Connections[0].ID, remoteState.Connections[0].ID
	if localID == remoteID {
		t.Fatal("fixture identities collide")
	}
	localPath := filepath.Join(local.stateDir, "agent-connections.json")
	localBytes, err := os.ReadFile(localPath)
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "Bearer "+strings.Repeat("r", 43) {
			t.Error("selected backing authentication is missing")
		}
		for _, name := range []string{"Cookie", "Origin", "X-Luci-Actor", "X-Luci-Policy", "Forwarded", "X-Forwarded-Host"} {
			if r.Header.Get(name) != "" {
				t.Errorf("local header %s reached backing", name)
			}
		}
		backingMux.ServeHTTP(w, r)
	}))
	defer remote.Close()
	tunnel := workspaceTunnel(local, remote.URL)
	local.workspace = Workspace{Mode: "cloud", ConnectionID: tunnel.ID, Ready: true}
	handler := local.ProxyWorkspace(localMux)
	for _, prefix := range []string{"", "/api"} {
		r := httptest.NewRequest("GET", prefix+"/v1/setup/agent-connections", nil)
		for _, name := range []string{"Authorization", "Cookie", "Origin", "X-Luci-Actor", "X-Luci-Policy", "Forwarded", "X-Forwarded-Host"} {
			r.Header.Set(name, "local-browser-value")
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != 200 || !strings.Contains(w.Body.String(), remoteID) || strings.Contains(w.Body.String(), localID) || strings.Contains(w.Body.String(), connectionFixtureKey) {
			t.Fatalf("%s connection catalogue did not come from selected backing", prefix)
		}
	}
	got := connectionSave(t, handler, map[string]any{"action": "select", "expectedRevision": 1, "id": remoteID})
	if got.ActiveConnectionID != remoteID || connectionRead(t, backingMux).ActiveConnectionID != remoteID {
		t.Fatal("connection selection did not persist on selected backing")
	}
	after, err := os.ReadFile(localPath)
	if err != nil || !bytes.Equal(localBytes, after) {
		t.Fatal("remote connection selection changed local private state")
	}
	remoteBytes, err := os.ReadFile(filepath.Join(backing.stateDir, "agent-connections.json"))
	if err != nil {
		t.Fatal(err)
	}
	beforeCalls := calls.Load()
	for _, missing := range []string{"connection", "authentication"} {
		want := 503
		local.mu.Lock()
		if missing == "connection" {
			tunnel.Status = "disconnected"
		} else {
			tunnel.Status = "connected"
			local.workspaceAuth = map[string]workspaceCredential{}
			want = 409
		}
		local.mu.Unlock()
		journeyRequest(t, handler, "GET", "/v1/setup/agent-connections", nil, want)
		journeyRequest(t, handler, "POST", "/api/v1/setup/agent-connections", map[string]any{"action": "delete", "expectedRevision": 2, "id": remoteID}, want)
	}
	if calls.Load() != beforeCalls {
		t.Fatal("disconnected backing was contacted")
	}
	for _, tc := range []struct {
		path   string
		before []byte
	}{{localPath, localBytes}, {filepath.Join(backing.stateDir, "agent-connections.json"), remoteBytes}} {
		after, err := os.ReadFile(tc.path)
		if err != nil || !bytes.Equal(tc.before, after) {
			t.Fatal("unavailable backing changed a private connection store")
		}
	}
	local.workspace = Workspace{Mode: "local", Ready: true}
	if connectionRead(t, handler).Connections[0].ID != localID {
		t.Fatal("returning local did not restore the local catalogue")
	}
	local.workspace = Workspace{Mode: "cloud", ConnectionID: tunnel.ID}
	var hostCalls int
	hostHandler := local.ProxyWorkspace(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hostCalls++; w.WriteHeader(204) }))
	for _, path := range []string{"/v1/setup/agent-connections-extra", "/v1/auth/me", "/v1/setup/cloud/jobs"} {
		journeyRequest(t, hostHandler, "GET", path, nil, 204)
	}
	if hostCalls != 3 || calls.Load() != beforeCalls {
		t.Fatal("host/account routes were moved with workspace connections")
	}
}
