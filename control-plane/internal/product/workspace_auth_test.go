package product

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

func TestWorkspaceSignsIntoRealRemoteGatewayAndPinsCredential(t *testing.T) {
	root := t.TempDir()
	var seenBearer string
	remoteEngine := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenBearer = r.Header.Get("Authorization")
		actor, _, ok := management.ActorFromContext(r.Context())
		if !ok || actor.Role != "super-admin" {
			t.Error("No trusted remote actor")
		}
		if r.URL.Path == "/v1/setup/status" {
			writeJSON(w, 200, Status{Product: "Luci", Version: Version, Configured: true, Config: Config{ProjectsDir: "/home/luci/projects", Mode: "local"}})
			return
		}
		writeJSON(w, 200, map[string]string{"source": "remote-owner"})
	})
	gateway, err := management.New(management.Options{StateDir: root, Version: Version, InitialHandler: remoteEngine})
	if err != nil {
		t.Fatal(err)
	}
	defer gateway.Close(context.Background())
	remote := httptest.NewServer(gateway.Handler())
	defer remote.Close()
	s, mux := journeyFixture(t)
	s.registerWorkspace(mux)
	tunnel := workspaceTunnel(s, remote.URL)
	// Fixture helper seeds old proxy tests; this eval starts genuinely unauthenticated.
	delete(s.workspaceAuth, workspaceCredentialKey(*tunnel))
	metadata := journeyRequest(t, mux, "GET", "/v1/setup/workspace/auth?connectionId="+tunnel.ID, nil, 200)
	if !strings.Contains(metadata.Body.String(), `"bootstrapRequired":true`) {
		t.Fatal(metadata.Body.String())
	}
	journeyRequest(t, mux, "POST", "/v1/setup/workspace/activate", map[string]string{"connectionId": tunnel.ID}, 401)
	original := s.runner
	s.runner = func(ctx context.Context, name string, args []string, dir string) (string, error) {
		if name != "ssh" {
			return original(ctx, name, args, dir)
		}
		if args[len(args)-1] != "cat /home/ubuntu/.local/state/luci/control-plane/bootstrap-token" {
			t.Fatal(args)
		}
		data, err := os.ReadFile(filepath.Join(root, "control-plane", "bootstrap-token"))
		return string(data), err
	}
	journeyRequest(t, mux, "POST", "/v1/setup/workspace/auth", map[string]string{"connectionId": tunnel.ID, "username": "remote-owner", "displayName": "Remote Owner", "password": "remote-password-long-enough"}, 200)
	journeyRequest(t, mux, "POST", "/v1/setup/workspace/activate", map[string]string{"connectionId": tunnel.ID}, 200)
	proxy := s.ProxyWorkspace(http.NotFoundHandler())
	req := httptest.NewRequest("GET", "/v1/projects", nil)
	req.Header.Set("Authorization", "Bearer local-secret-must-not-leak")
	w := httptest.NewRecorder()
	proxy.ServeHTTP(w, req)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "remote-owner") || seenBearer == "Bearer local-secret-must-not-leak" {
		t.Fatal(w.Code, w.Body.String(), seenBearer)
	}
	data, err := os.ReadFile(filepath.Join(s.stateDir, "workspace-auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved map[string]workspaceCredential
	if json.Unmarshal(data, &saved) != nil || len(saved) != 1 {
		t.Fatal("Credential was not stored")
	}
	if strings.Contains(string(data), "remote-password") {
		t.Fatal("Password was persisted")
	}
	// A new SSH identity/host key cannot reuse the saved remote session.
	s.mu.Lock()
	tunnel.KnownHost += "changed"
	s.mu.Unlock()
	journeyRequest(t, proxy, "GET", "/v1/projects", nil, 401)
}

func TestRemoteWorkspaceCredentialsRejectUntrustedAddressAndRedirect(t *testing.T) {
	for _, address := range []string{"https://example.com", "http://localhost:8730", "http://127.0.0.1:8730/other", "http://user:secret@127.0.0.1:8730"} {
		if remoteWorkspaceAPI(context.Background(), address, "POST", "/v1/auth/login", "", map[string]string{"password": "secret"}, nil) == nil {
			t.Fatal(address)
		}
	}
	calls := 0
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(200) }))
	defer destination.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, destination.URL, 307) }))
	defer redirect.Close()
	if remoteWorkspaceAPI(context.Background(), redirect.URL, "POST", "/v1/auth/login", "", map[string]string{"password": "secret"}, nil) == nil || calls != 0 {
		t.Fatal("Redirect exposed credentials")
	}
}
