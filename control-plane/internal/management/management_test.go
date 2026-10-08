package management

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/lcfrai/luci/control-plane/internal/conversationpath"
)

const testPassword = "a long test password"

func fixture(t *testing.T, initial http.Handler) (*Gateway, http.Handler, string, User) {
	t.Helper()
	if initial == nil {
		initial = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			jsonReply(w, 200, map[string]string{"workspace": "legacy"})
		})
	}
	g, err := New(Options{StateDir: t.TempDir(), InitialHandler: initial, Version: "test", WorkerArgs: []string{"-test.run=^TestWorkerHelper$"}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := g.Close(ctx); err != nil {
			t.Error(err)
		}
	})
	h := g.Handler()
	w := request(t, h, "POST", "/v1/auth/bootstrap", "", map[string]string{"bootstrapToken": g.bootstrapToken, "username": "owner", "displayName": "Owner", "password": testPassword}, 201)
	var response struct {
		Token string `json:"token"`
		User  User   `json:"user"`
	}
	if err = json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	return g, h, response.Token, response.User
}
func request(t *testing.T, h http.Handler, method, path, token string, body any, status int) *httptest.ResponseRecorder {
	t.Helper()
	var data []byte
	if body != nil {
		data, _ = json.Marshal(body)
	}
	r := httptest.NewRequest(method, path, bytes.NewReader(data))
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != status {
		t.Fatalf("%s %s status%d want%d: %s", method, path, w.Code, status, w.Body.String())
	}
	return w
}
func addUser(t *testing.T, h http.Handler, token, name, role string) User {
	t.Helper()
	w := request(t, h, "POST", "/v1/admin/users", token, map[string]string{"username": name, "displayName": name, "password": testPassword, "role": role}, 201)
	var u User
	if json.Unmarshal(w.Body.Bytes(), &u) != nil {
		t.Fatal("invalid user")
	}
	return u
}
func loginAs(t *testing.T, h http.Handler, name string) string {
	t.Helper()
	w := request(t, h, "POST", "/v1/auth/login", "", map[string]string{"username": name, "password": testPassword}, 200)
	var out struct {
		Token string `json:"token"`
	}
	json.Unmarshal(w.Body.Bytes(), &out)
	return out.Token
}

func TestBootstrapAuthPersistenceAndSecretFreePublicRecords(t *testing.T) {
	g, h, token, owner := fixture(t, nil)
	request(t, h, "GET", "/v1/projects", "", nil, 401)
	request(t, h, "POST", "/v1/auth/bootstrap", "", map[string]string{"bootstrapToken": "invalid", "username": "second", "displayName": "Second", "password": testPassword}, 403)
	w := request(t, h, "GET", "/v1/auth/me", token, nil, 200)
	if !strings.Contains(w.Body.String(), `"role":"super-admin"`) {
		t.Fatal("owner lacks super-admin")
	}
	if _, err := os.Stat(filepath.Join(g.dir, "bootstrap-token")); !os.IsNotExist(err) {
		t.Fatal("bootstrap token retained after setup")
	}
	data, err := os.ReadFile(filepath.Join(g.dir, "identity.json"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte(testPassword)) || bytes.Contains(data, []byte(token)) {
		t.Fatal("plaintext password or sign-in token persisted")
	}
	if !bytes.Contains(data, []byte("argon2id:v19:m65536:t3:p4:")) {
		t.Fatal("Argon2id parameters missing")
	}
	reopened, err := New(g.opts)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close(context.Background())
	request(t, reopened.Handler(), "GET", "/v1/auth/me", token, nil, 200)
	request(t, h, "POST", "/v1/auth/login", "", map[string]string{"username": "owner", "password": "wrong"}, 401)
	newToken := loginAs(t, h, "owner")
	if newToken == token || newToken == "" {
		t.Fatal("fresh login did not mint random token")
	}
	audit := request(t, h, "GET", "/v1/admin/audit", token, nil, 200)
	if strings.Contains(audit.Body.String(), testPassword) || strings.Contains(audit.Body.String(), token) || !strings.Contains(audit.Body.String(), owner.ID) {
		t.Fatal("audit has missing attribution or secret data")
	}
}

func TestRolesProtectLastSuperAdminAndRevokeDisabledAccounts(t *testing.T) {
	_, h, token, owner := fixture(t, nil)
	member := addUser(t, h, token, "member", "member")
	admin := addUser(t, h, token, "admin", "admin")
	memberToken := loginAs(t, h, "member")
	adminToken := loginAs(t, h, "admin")
	request(t, h, "GET", "/v1/admin/users", memberToken, nil, 403)
	request(t, h, "PATCH", "/v1/admin/users/"+owner.ID, token, map[string]any{"disabled": true}, 409)
	request(t, h, "PATCH", "/v1/admin/users/"+owner.ID, token, map[string]string{"role": "member"}, 409)
	request(t, h, "POST", "/v1/admin/users", adminToken, map[string]string{"username": "escalated", "displayName": "Escalated", "password": testPassword, "role": "super-admin"}, 403)
	request(t, h, "PATCH", "/v1/admin/users/"+owner.ID, adminToken, map[string]string{"password": testPassword}, 403)
	request(t, h, "PATCH", "/v1/admin/policy", adminToken, map[string]any{"allowHostCommands": true}, 403)
	request(t, h, "PATCH", "/v1/admin/users/"+member.ID, adminToken, map[string]bool{"disabled": true}, 200)
	request(t, h, "GET", "/v1/auth/me", memberToken, nil, 401)
	request(t, h, "POST", "/v1/auth/login", "", map[string]string{"username": "member", "password": testPassword}, 401)
	request(t, h, "PATCH", "/v1/admin/users/"+admin.ID, token, map[string]string{"role": "member"}, 200)
	request(t, h, "GET", "/v1/auth/me", adminToken, nil, 401)
}

func TestPolicyEnforcesActualSessionCountAndTrustedActor(t *testing.T) {
	var active atomic.Bool
	var ran atomic.Int32
	initial := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := ActorFromContext(r.Context())
		if !ok || u.Role != "super-admin" || p.MaxSessionsPerUser != 1 {
			t.Error("missing trusted actor or policy")
		}
		if r.Header.Get("X-Luci-Actor") != "" {
			t.Error("spoofed actor header reached engine")
		}
		if r.Method == "GET" && r.URL.Path == "/v1/sessions" {
			status := "exited"
			if active.Load() {
				status = "running"
			}
			jsonReply(w, 200, map[string]any{"sessions": []map[string]string{{"id": "existing", "status": status, "harness": "shell"}}})
			return
		}
		ran.Add(1)
		jsonReply(w, 200, map[string]bool{"ok": true})
	})
	_, h, token, _ := fixture(t, initial)
	request(t, h, "PATCH", "/v1/admin/policy", token, map[string]any{"maxSessionsPerUser": 1, "allowedHarnesses": []string{"shell"}, "allowPublicPublishing": false, "allowCloudProvisioning": false}, 200)
	request(t, h, "POST", "/v1/sessions", token, map[string]string{"harness": "codex"}, 403)
	request(t, h, "POST", "/v1/product/projects/test/git/github", token, map[string]string{"visibility": "public"}, 403)
	request(t, h, "POST", "/v1/setup/cloud/create", token, map[string]any{}, 403)
	request(t, h, "POST", "/v1/product/projects/test/checks", token, map[string]string{"execution": "host"}, 403)
	active.Store(true)
	request(t, h, "POST", "/v1/sessions", token, map[string]string{"harness": "shell"}, 409)
	active.Store(false)
	request(t, h, "POST", "/v1/sessions", token, map[string]string{"harness": "shell"}, 200)
	r := httptest.NewRequest("GET", "/v1/projects", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("X-Luci-Actor", "spoof")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	if ran.Load() != 2 {
		t.Fatalf("blocked operation reached engine: %d", ran.Load())
	}
}

func TestSocketTicketsAreSingleUsePathBoundExpiringAndRevocable(t *testing.T) {
	g, h, token, _ := fixture(t, nil)
	ticket := func() string {
		w := request(t, h, "POST", "/v1/auth/socket-ticket", token, map[string]string{"path": "/v1/sessions/owned/attach"}, 200)
		var out struct {
			Ticket string `json:"ticket"`
		}
		json.Unmarshal(w.Body.Bytes(), &out)
		return out.Ticket
	}
	attach := func(value, path string, status int) {
		r := httptest.NewRequest("GET", path+"?ticket="+value, nil)
		r.Header.Set("Upgrade", "websocket")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("attach %d want%d %s", w.Code, status, w.Body.String())
		}
	}
	one := ticket()
	attach(one, "/v1/sessions/owned/attach", 200)
	attach(one, "/v1/sessions/owned/attach", 401)
	wrong := ticket()
	attach(wrong, "/v1/sessions/other/attach", 401)
	attach(wrong, "/v1/sessions/owned/attach", 401)
	expired := ticket()
	g.mu.Lock()
	entry := g.tickets[hashToken(expired)]
	entry.ExpiresAt = time.Now().Add(-time.Second)
	g.tickets[hashToken(expired)] = entry
	g.mu.Unlock()
	attach(expired, "/v1/sessions/owned/attach", 401)
	revoked := ticket()
	request(t, h, "POST", "/v1/auth/logout", token, map[string]any{}, 200)
	attach(revoked, "/v1/sessions/owned/attach", 401)
	request(t, h, "GET", "/v1/auth/me", token, nil, 401)
}

func TestPasswordChangeRevokesExistingTokens(t *testing.T) {
	_, h, token, _ := fixture(t, nil)
	other := loginAs(t, h, "owner")
	request(t, h, "POST", "/v1/auth/password", token, map[string]string{"currentPassword": testPassword, "newPassword": "a different secure password"}, 200)
	request(t, h, "GET", "/v1/auth/me", token, nil, 401)
	request(t, h, "GET", "/v1/auth/me", other, nil, 401)
	request(t, h, "POST", "/v1/auth/login", "", map[string]string{"username": "owner", "password": "a different secure password"}, 200)
}

func TestResourcePolicyDefaultsBoundsAndOlderStateMigration(t *testing.T) {
	g, h, token, _ := fixture(t, nil)
	if g.state.Policy.MaxSessionsPerUser != 1 || g.state.Policy.SessionMemoryMiB != 1024 || g.state.Policy.CheckCPUMillis != 1000 {
		t.Fatal("cheap defaults missing")
	}
	request(t, h, "PATCH", "/v1/admin/policy", token, map[string]int{"sessionMemoryMiB": 127}, 400)
	request(t, h, "PATCH", "/v1/admin/policy", token, map[string]int{"checkCPUMillis": 16001}, 400)
	request(t, h, "PATCH", "/v1/admin/policy", token, map[string]int{"sessionMemoryMiB": 512, "checkCPUMillis": 500}, 200)
	file := filepath.Join(g.dir, "identity.json")
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var saved map[string]json.RawMessage
	json.Unmarshal(data, &saved)
	var policy map[string]json.RawMessage
	json.Unmarshal(saved["policy"], &policy)
	for _, key := range []string{"sessionMemoryMiB", "sessionCPUMillis", "checkMemoryMiB", "checkCPUMillis"} {
		delete(policy, key)
	}
	saved["policy"], _ = json.Marshal(policy)
	data, _ = json.Marshal(saved)
	if err = atomicPrivate(file, data); err != nil {
		t.Fatal(err)
	}
	reopened, err := New(g.opts)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close(context.Background())
	if reopened.state.Policy.SessionMemoryMiB != 1024 || reopened.state.Policy.CheckCPUMillis != 1000 {
		t.Fatal("older policy lost safe defaults")
	}
}

func TestActualOwnedWorkerIsolatesStateEnvironmentAndStops(t *testing.T) {
	g, h, token, _ := fixture(t, nil)
	const selectedEngine = "unix:///run/luci-engine/docker.sock"
	g.opts.ExecutionStateDir = filepath.Join(g.opts.StateDir, "host-owned-execution")
	g.opts.BaseEnvironment = append(os.Environ(), "GH_TOKEN=parent-secret", "OPENAI_API_KEY=parent-model-secret", "AWS_ACCESS_KEY_ID=parent-cloud-secret", "LUCI_CONVERSATIONS_DIR=parent-provider-home", "LUCI_EXECUTION_STATE_DIR=foreign-execution-state", "DOCKER_HOST="+selectedEngine, "LUCI_DOCKER_HOST="+selectedEngine)
	member := addUser(t, h, token, "worker", "member")
	memberToken := loginAs(t, h, "worker")
	request(t, h, "GET", "/v1/projects", token, nil, 200)
	w := request(t, h, "GET", "/v1/projects", memberToken, nil, 200)
	var proof map[string]string
	if json.Unmarshal(w.Body.Bytes(), &proof) != nil {
		t.Fatal(w.Body.String())
	}
	root := filepath.Join(g.dir, "users", member.ID, "runtime")
	if proof["projects"] != filepath.Join(root, "projects") || proof["home"] != filepath.Join(root, "home") || proof["actor"] != member.ID || proof["ghToken"] != "" || proof["modelToken"] != "" || proof["awsToken"] != "" {
		t.Fatalf("isolation proof %+v", proof)
	}
	if proof["dockerHost"] != selectedEngine || proof["luciDockerHost"] != selectedEngine {
		t.Fatal("actual managed worker lost the controller's selected private Docker engine")
	}
	if proof["executionState"] != g.opts.ExecutionStateDir {
		t.Fatal("actual worker did not receive the host-owned shared execution configuration")
	}
	wantConversations := filepath.Join(root, "state", "conversations")
	if runtime.GOOS == "windows" {
		wantConversations = conversationpath.UserRoot(g.dir, member.ID)
	}
	if proof["conversations"] != wantConversations {
		t.Fatal("worker inherited another identity's provider home")
	}
	request(t, h, "POST", "/v1/setup/config", memberToken, map[string]string{"projectsDir": g.opts.StateDir}, 403)
	request(t, h, "GET", "/v1/admin/users/"+member.ID+"/runtime", token, nil, 200)
	request(t, h, "POST", "/v1/admin/users/"+member.ID+"/runtime/stop", token, map[string]any{}, 200)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		g.mu.Lock()
		rt := g.runtimes[member.ID]
		g.mu.Unlock()
		select {
		case <-rt.done:
			if rt.snapshot().Status != "stopped" {
				t.Fatal("stopped process state not retained")
			}
			return
		default:
			time.Sleep(20 * time.Millisecond)
		}
	}
	t.Fatal("owned worker did not stop")
}

func TestWorkerHelper(t *testing.T) {
	if os.Getenv("LUCI_WORKER") != "1" {
		return
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		os.Exit(3)
	}
	mux := http.NewServeMux()
	server := &http.Server{Handler: WorkerHandler(os.Getenv("LUCI_WORKER_TOKEN"), mux)}
	mux.HandleFunc("/v1/setup/status", func(w http.ResponseWriter, r *http.Request) { jsonReply(w, 200, map[string]bool{"ready": true}) })
	mux.HandleFunc("/v1/setup/shutdown", func(w http.ResponseWriter, r *http.Request) {
		jsonReply(w, 200, map[string]bool{"stopping": true})
		go server.Close()
	})
	mux.HandleFunc("/v1/projects", func(w http.ResponseWriter, r *http.Request) {
		u, _, ok := ActorFromContext(r.Context())
		if !ok {
			os.Exit(4)
		}
		jsonReply(w, 200, map[string]string{"actor": u.ID, "projects": os.Getenv("LUCI_PROJECTS_DIR"), "home": os.Getenv("HOME"), "conversations": os.Getenv("LUCI_CONVERSATIONS_DIR"), "executionState": os.Getenv("LUCI_EXECUTION_STATE_DIR"), "dockerHost": os.Getenv("DOCKER_HOST"), "luciDockerHost": os.Getenv("LUCI_DOCKER_HOST"), "ghToken": os.Getenv("GH_TOKEN"), "modelToken": os.Getenv("OPENAI_API_KEY"), "awsToken": os.Getenv("AWS_ACCESS_KEY_ID")})
	})
	if WriteWorkerAddress(os.Getenv("LUCI_WORKER_ADDRESS_FILE"), listener.Addr().String()) != nil {
		os.Exit(5)
	}
	_ = server.Serve(listener)
	os.Exit(0)
}

func TestWorkerBearerAndActorCannotBeSpoofed(t *testing.T) {
	t.Setenv("LUCI_ACTOR_ID", "owned")
	t.Setenv("LUCI_ACTOR_ROLE", "member")
	token := randomToken()
	called := false
	handler := WorkerHandler(token, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		u, _, ok := ActorFromContext(r.Context())
		if !ok || u.ID != "owned" {
			t.Fatal("actor missing")
		}
		io.WriteString(w, "ok")
	}))
	p := Policy{InstanceName: "test", BackingMode: "docker", MaxSessionsPerUser: 1, MaxConcurrentChecksPerUser: 1, SessionMemoryMiB: 1024, SessionCPUMillis: 1000, CheckMemoryMiB: 1024, CheckCPUMillis: 1000, AllowedHarnesses: []string{"shell"}}
	r := httptest.NewRequest("GET", "/v1/projects", nil)
	setWorkerHeaders(r, User{ID: "owned", Role: "member"}, p, token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 200 || !called {
		t.Fatal("valid gateway request rejected")
	}
	r = httptest.NewRequest("GET", "/v1/projects", nil)
	setWorkerHeaders(r, User{ID: "other", Role: "super-admin"}, p, token)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("spoofed identity accepted")
	}
	r = httptest.NewRequest("GET", "/v1/projects", nil)
	setWorkerHeaders(r, User{ID: "owned", Role: "member"}, p, "wrong")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatal("invalid bearer accepted")
	}
}

func TestAlreadyUpgradedInlineSocketClosesOnLogout(t *testing.T) {
	initial := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		for {
			typ, payload, err := c.Read(r.Context())
			if err != nil {
				return
			}
			if c.Write(r.Context(), typ, payload) != nil {
				return
			}
		}
	})
	_, h, token, _ := fixture(t, initial)
	server := httptest.NewServer(h)
	defer server.Close()
	w := request(t, h, "POST", "/v1/auth/socket-ticket", token, map[string]string{"path": "/v1/sessions/owned/attach"}, 200)
	var ticket struct {
		Ticket string `json:"ticket"`
	}
	json.Unmarshal(w.Body.Bytes(), &ticket)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, strings.Replace(server.URL, "http://", "ws://", 1)+"/v1/sessions/owned/attach?ticket="+ticket.Ticket, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()
	if err = c.Write(ctx, websocket.MessageText, []byte("proof")); err != nil {
		t.Fatal(err)
	}
	_, payload, err := c.Read(ctx)
	if err != nil || string(payload) != "proof" {
		t.Fatalf("echo=%q err%v", payload, err)
	}
	request(t, h, "POST", "/v1/auth/logout", token, map[string]any{}, 200)
	if _, _, err = c.Read(ctx); err == nil {
		t.Fatal("revoked inline socket remained open")
	}
}

func TestAuthenticatedPrivatePreviewPreservesEscapedAssetAndApplicationSession(t *testing.T) {
	initial := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/v1/product/projects/p1/releases/r1/preview/api/a%2Fb" || r.URL.Query().Get("ticket") != "application-ticket" || r.Header.Get("Cookie") != "appSession=abc" {
			t.Errorf("preview changed: %s %s %s", r.URL.EscapedPath(), r.URL.RawQuery, r.Header.Get("Cookie"))
		}
		jsonReply(w, 200, map[string]bool{"ok": true})
	})
	_, h, token, _ := fixture(t, initial)
	makeRequest := func(browser bool, auth bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "/v1/product/projects/p1/releases/r1/preview/api/a%2Fb?ticket=application-ticket", nil)
		r.Header.Set("X-Luci-Preview-Bridge", "1")
		r.Header.Set("Cookie", "appSession=abc")
		if auth {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		if browser {
			r.Header.Set("Sec-Fetch-Site", "same-origin")
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := makeRequest(false, true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := makeRequest(true, true); w.Code != 400 {
		t.Fatal("browser escaped bridge accepted", w.Code)
	}
	if w := makeRequest(false, false); w.Code != 401 {
		t.Fatal("unauthenticated bridge accepted", w.Code)
	}
}
