package product

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

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
