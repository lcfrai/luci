package product

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

// Exercise the production worker trust boundary to construct an actor context;
// arbitrary headers supplied directly to the product handler remain untrusted.
func workspaceAdminActor(handler http.Handler, role string) http.Handler {
	secret := strings.Repeat("w", 43)
	worker := management.WorkerHandler(secret, handler)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actor, _ := json.Marshal(management.User{ID: "local-actor", Role: role})
		policy, _ := json.Marshal(management.Policy{InstanceName: "Local", BackingMode: "docker", MaxSessionsPerUser: 1, MaxConcurrentChecksPerUser: 1, SessionMemoryMiB: 1024, SessionCPUMillis: 1000, CheckMemoryMiB: 1024, CheckCPUMillis: 1000, AllowedHarnesses: []string{"shell"}})
		r.Header.Set("Authorization", "Bearer "+secret)
		r.Header.Set("X-Luci-Actor", base64.RawURLEncoding.EncodeToString(actor))
		r.Header.Set("X-Luci-Policy", base64.RawURLEncoding.EncodeToString(policy))
		worker.ServeHTTP(w, r)
	})
}

func workspaceAdminFixture(t *testing.T, remote http.Handler) (*Server, http.Handler, *Tunnel) {
	t.Helper()
	server := httptest.NewServer(remote)
	t.Cleanup(server.Close)
	s, mux := journeyFixture(t)
	s.registerWorkspace(mux)
	tunnel := workspaceTunnel(s, server.URL)
	s.workspace = Workspace{Mode: "cloud", ConnectionID: tunnel.ID, Host: tunnel.Host, Ready: true}
	return s, workspaceAdminActor(s.ProxyWorkspace(mux), "super-admin"), tunnel
}

func TestWorkspaceAdminUsesRealRemoteIdentityPolicyUsersAndRevocation(t *testing.T) {
	root := t.TempDir()
	gateway, err := management.New(management.Options{StateDir: root, Version: Version, InstanceName: "Remote backing", Backing: "aws", InitialHandler: http.NotFoundHandler()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = gateway.Close(context.Background()) })
	s, handler, tunnel := workspaceAdminFixture(t, gateway.Handler())
	code, err := os.ReadFile(filepath.Join(root, "control-plane", "bootstrap-token"))
	if err != nil {
		t.Fatal(err)
	}
	var owner struct {
		Token string          `json:"token"`
		User  management.User `json:"user"`
	}
	if err := remoteWorkspaceAPI(context.Background(), tunnel.APIURL, "POST", "/v1/auth/bootstrap", "", map[string]string{"bootstrapToken": strings.TrimSpace(string(code)), "username": "remote-owner", "displayName": "Remote Owner", "password": "remote-owner-password-long"}, &owner); err != nil {
		t.Fatal(err)
	}
	setToken := func(token string) {
		s.mu.Lock()
		s.workspaceAuth[workspaceCredentialKey(*tunnel)] = workspaceCredential{Token: token}
		s.mu.Unlock()
	}
	setToken(owner.Token)
	base := "/v1/setup/workspace/admin"
	me := journeyRequest(t, handler, "GET", base+"/me", nil, 200)
	if !strings.Contains(me.Body.String(), `"instanceName":"Remote backing"`) || !strings.Contains(me.Body.String(), `"managePolicy":true`) || strings.Contains(me.Body.String(), owner.Token) {
		t.Fatal(me.Body.String())
	}
	create := journeyRequest(t, handler, "POST", base+"/users", map[string]string{"username": "remote-member", "displayName": "Remote Member", "password": "remote-member-password-long", "role": "member"}, 201)
	var member management.User
	if json.Unmarshal(create.Body.Bytes(), &member) != nil || member.Role != "member" || strings.Contains(create.Body.String(), "password") {
		t.Fatal(create.Body.String())
	}
	listed := journeyRequest(t, handler, "GET", base+"/users", nil, 200)
	if !strings.Contains(listed.Body.String(), member.ID) {
		t.Fatal(listed.Body.String())
	}
	policy := journeyRequest(t, handler, "PATCH", base+"/policy", map[string]any{"instanceName": "Managed remotely", "monthlyBudgetUsd": 15}, 200)
	if !strings.Contains(policy.Body.String(), `"instanceName":"Managed remotely"`) || !strings.Contains(policy.Body.String(), `"monthlyBudgetUsd":15`) {
		t.Fatal(policy.Body.String())
	}
	journeyRequest(t, handler, "GET", base+"/policy", nil, 200)
	journeyRequest(t, handler, "HEAD", base+"/policy", nil, 200)
	journeyRequest(t, handler, "GET", base+"/users/"+member.ID+"/runtime", nil, 200)
	journeyRequest(t, handler, "POST", base+"/users/"+member.ID+"/runtime/stop", map[string]any{}, 200)
	journeyRequest(t, handler, "POST", base+"/users/"+owner.User.ID+"/runtime/stop", map[string]any{}, 409)
	journeyRequest(t, handler, "PATCH", base+"/users/"+owner.User.ID, map[string]any{"disabled": true}, 409)
	journeyRequest(t, handler, "POST", base+"/users", map[string]string{"username": member.Username, "displayName": "Duplicate", "password": "duplicate-password-long", "role": "member"}, 409)
	audit := journeyRequest(t, handler, "GET", base+"/audit", nil, 200)
	if !strings.Contains(audit.Body.String(), "policy.update") || strings.Contains(audit.Body.String(), "password-long") {
		t.Fatal(audit.Body.String())
	}
	var login struct {
		Token string `json:"token"`
	}
	if err := remoteWorkspaceAPI(context.Background(), tunnel.APIURL, "POST", "/v1/auth/login", "", map[string]string{"username": member.Username, "password": "remote-member-password-long"}, &login); err != nil {
		t.Fatal(err)
	}
	setToken(login.Token)
	memberMe := journeyRequest(t, handler, "GET", base+"/me", nil, 200)
	if !strings.Contains(memberMe.Body.String(), `"manageUsers":false`) {
		t.Fatal(memberMe.Body.String())
	}
	journeyRequest(t, handler, "GET", base+"/users", nil, 403)
	journeyRequest(t, handler, "PATCH", base+"/policy", map[string]string{"instanceName": "Forbidden"}, 403)
	setToken(owner.Token)
	journeyRequest(t, handler, "PATCH", base+"/users/"+member.ID, map[string]any{"disabled": true}, 200)
	setToken(login.Token)
	revoked := journeyRequest(t, handler, "GET", base+"/me", nil, 409)
	if !strings.Contains(revoked.Body.String(), "backing_sign_in_required") {
		t.Fatal(revoked.Body.String())
	}
	// The remote revocation neither removes nor changes the local trusted actor.
	setToken(owner.Token)
	journeyRequest(t, handler, "GET", base+"/users", nil, 200)
}

func TestWorkspaceAdminRequiresLocalRoleAndSelectedVerifiedCredential(t *testing.T) {
	var calls atomic.Int32
	s, handler, tunnel := workspaceAdminFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); writeJSON(w, 200, management.Policy{}) }))
	base := "/v1/setup/workspace/admin/policy"
	mux := http.NewServeMux()
	s.registerWorkspaceAdmin(mux)
	request := httptest.NewRequest("GET", base, nil)
	request.Header.Set("X-Luci-Actor", "pretend-super-admin")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != 403 {
		t.Fatal(response.Code)
	}
	journeyRequest(t, workspaceAdminActor(mux, "member"), "GET", base, nil, 403)
	journeyRequest(t, handler, "GET", base+"?connectionId=other", nil, 400)
	journeyRequest(t, handler, "PATCH", base, map[string]string{"baseUrl": "http://example.com"}, 400)
	s.workspace.Mode = "local"
	journeyRequest(t, handler, "GET", base, nil, 409)
	s.workspace.Mode = "cloud"
	tunnel.Status = "disconnected"
	journeyRequest(t, handler, "GET", base, nil, 503)
	tunnel.Status = "connected"
	tunnel.KnownHost += "different-pin"
	journeyRequest(t, handler, "GET", base, nil, 409)
	tunnel.KnownHost = ""
	journeyRequest(t, handler, "GET", base, nil, 503)
	if calls.Load() != 0 {
		t.Fatal("A rejected request reached the remote backing")
	}
}

func TestWorkspaceAdminBoundsTransportAndReturnsOnlyPublicFields(t *testing.T) {
	var calls atomic.Int32
	var mode atomic.Int32
	remoteToken := strings.Repeat("r", 43)
	password := "private-fixture-password-long"
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(100); w.WriteHeader(200) }))
	defer destination.Close()
	s, handler, tunnel := workspaceAdminFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "Bearer "+remoteToken || r.Header.Get("X-Luci-Actor") != "" || r.Header.Get("X-Luci-Policy") != "" || r.Header.Get("Cookie") != "" || r.Header.Get("Origin") != "" || r.Header.Get("X-Forwarded-For") != "" {
			t.Error("Local identity or browser headers were forwarded")
		}
		switch mode.Load() {
		case 1:
			http.Redirect(w, r, destination.URL, 307)
		case 2:
			_, _ = w.Write(bytes.Repeat([]byte("x"), workspaceAdminLimit+1))
		case 3:
			writeJSON(w, 403, map[string]string{"error": "permission_denied", "message": "Denied", "token": remoteToken, "password": password})
		default:
			writeJSON(w, 201, map[string]string{"id": "user-public", "username": "public", "displayName": password + " " + remoteToken, "role": "member", "password": password, "token": remoteToken})
		}
	}))
	base := "/v1/setup/workspace/admin/users"
	r := httptest.NewRequest("POST", base, strings.NewReader(`{"username":"public","password":"`+password+`"}`))
	r.Header.Set("Cookie", "local-browser-secret")
	r.Header.Set("Origin", "http://local.invalid")
	r.Header.Set("X-Forwarded-For", "local-address")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 201 || strings.Contains(w.Body.String(), password) || strings.Contains(w.Body.String(), remoteToken) || strings.Contains(w.Body.String(), `"token"`) || !strings.Contains(w.Body.String(), "[redacted]") {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, pair := range []struct {
		mode   int32
		status int
	}{{1, 502}, {2, 502}, {3, 403}} {
		mode.Store(pair.mode)
		result := journeyRequest(t, handler, "POST", base, map[string]string{"username": "public", "password": password}, pair.status)
		if strings.Contains(result.Body.String(), password) || strings.Contains(result.Body.String(), remoteToken) {
			t.Fatal("Credential appeared in an error")
		}
	}
	before := calls.Load()
	large := httptest.NewRequest("POST", base, strings.NewReader(`{"password":"`+strings.Repeat("x", workspaceAdminLimit)+`"}`))
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, large)
	if w.Code != 413 || calls.Load() != before {
		t.Fatal("Oversized input reached backing", w.Code)
	}
	for _, badAddress := range []string{"http://localhost:8730", "https://example.com", "http://127.0.0.1:8730/other", "http://user:secret@127.0.0.1:8730"} {
		s.mu.Lock()
		tunnel.APIURL = badAddress
		s.mu.Unlock()
		journeyRequest(t, handler, "GET", base, nil, 503)
	}
	if calls.Load() != 4 {
		t.Fatal("Request followed redirect or used untrusted address", calls.Load())
	}
}

func TestWorkspaceAdminRedactionPreservesJSONFieldsAndNonStringValues(t *testing.T) {
	value := map[string]any{"message": "e", "enabled": false, "budget": json.Number("0.1234567890123456789"), "items": []any{"e"}}
	redacted := redactWorkspaceAdminStrings(value, []string{"e"})
	data, err := json.Marshal(redacted)
	if err != nil || !json.Valid(data) || !bytes.Contains(data, []byte(`"enabled":false`)) || !bytes.Contains(data, []byte(`0.1234567890123456789`)) || !bytes.Contains(data, []byte(`"message":"[redacted]"`)) {
		t.Fatal(string(data), err)
	}
}
