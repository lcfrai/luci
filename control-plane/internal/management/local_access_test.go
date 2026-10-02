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
	"reflect"
	"strings"
	"testing"
	"time"
)

const localTestCapability = "test-native-launch-capability-not-a-real-secret-123456"

func TestLocalAccessFreshServiceRequiresFirstOwnerBootstrap(t *testing.T) {
	g, err := New(Options{StateDir: t.TempDir(), InitialHandler: http.NotFoundHandler(), LocalAccessToken: localTestCapability})
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close(context.Background())
	h := g.Handler()
	owner := localRequest(t, h, "GET", "/v1/auth/local-owner", localTestCapability, "127.0.0.1:1", nil, 200)
	if strings.TrimSpace(owner.Body.String()) != `{"owner":null}` {
		t.Fatal(owner.Body.String())
	}
	identity := map[string]string{"username": "first-owner", "displayName": "First Owner", "password": testPassword}
	localRequest(t, h, "POST", "/v1/auth/local-account", localTestCapability, "127.0.0.1:1", identity, 409)
	localRequest(t, h, "POST", "/v1/auth/local-owner/recover", localTestCapability, "127.0.0.1:1", map[string]string{"newPassword": testPassword}, 409)
	status := request(t, h, "GET", "/v1/control-plane/status", "", nil, 200)
	if !strings.Contains(status.Body.String(), `"bootstrapRequired":true`) || len(g.state.Users) != 0 {
		t.Fatal("local helpers bypassed bootstrap")
	}
	identity["bootstrapToken"] = g.bootstrapToken
	request(t, h, "POST", "/v1/auth/bootstrap", "", identity, 201)
	status = request(t, h, "GET", "/v1/control-plane/status", "", nil, 200)
	if !strings.Contains(status.Body.String(), `"bootstrapRequired":false`) {
		t.Fatal("configured status is stale")
	}
}

func localRequest(t *testing.T, h http.Handler, method, path, capability, remote string, body any, want int) *httptest.ResponseRecorder {
	t.Helper()
	data, _ := json.Marshal(body)
	r := httptest.NewRequest(method, path, bytes.NewReader(data))
	r.RemoteAddr = remote
	r.Header.Set(localAccessHeader, capability)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != want {
		t.Fatalf("%s %s = %d want%d: %s", method, path, w.Code, want, w.Body.String())
	}
	return w
}
func TestLocalAccessRequiresNativeCapabilityAndLoopbackForEveryRoute(t *testing.T) {
	g, h, token, _ := fixture(t, nil)
	g.opts.LocalAccessToken = localTestCapability
	for _, route := range []struct {
		method, path string
		body         any
	}{
		{"GET", "/v1/auth/local-owner", nil},
		{"POST", "/v1/auth/local-account", map[string]string{"username": "another", "displayName": "Another", "password": testPassword}},
		{"POST", "/v1/auth/local-owner/recover", map[string]string{"newPassword": testPassword}},
	} {
		for _, attempt := range []struct{ cap, addr string }{{"", "127.0.0.1:1000"}, {"wrong", "127.0.0.1:1000"}, {localTestCapability, "203.0.113.1:1000"}, {localTestCapability, "not-an-address"}} {
			localRequest(t, h, route.method, route.path, attempt.cap, attempt.addr, route.body, 403)
		}
		request(t, h, route.method, route.path, token, route.body, 403)
	}
	before, _ := os.ReadFile(filepath.Join(g.dir, "identity.json"))
	got := localRequest(t, h, "GET", "/v1/auth/local-owner", localTestCapability, "[::1]:1000", nil, 200)
	if !strings.Contains(got.Body.String(), `"username":"owner"`) || strings.Contains(got.Body.String(), "passwordHash") {
		t.Fatal("private owner metadata incorrect")
	}
	after, _ := os.ReadFile(filepath.Join(g.dir, "identity.json"))
	if !bytes.Equal(before, after) {
		t.Fatal("GET mutated identity")
	}
	public := request(t, h, "GET", "/v1/control-plane/status", "", nil, 200).Body.String()
	if strings.Contains(public, localTestCapability) || strings.Contains(public, `"owner"`) || strings.Contains(public, "localAccess") {
		t.Fatal("local authority or identity exposed publicly")
	}
	g.opts.LocalAccessToken = ""
	localRequest(t, h, "GET", "/v1/auth/local-owner", localTestCapability, "127.0.0.1:1000", nil, 403)
	env := isolatedEnvironment([]string{"LUCI_LOCAL_ACCESS_TOKEN=" + localTestCapability}, t.TempDir(), "worker-key", "address", User{ID: "member", Role: "member"})
	if strings.Contains(strings.Join(env, "\n"), localTestCapability) {
		t.Fatal("native capability inherited by worker")
	}
}

func TestLocalAccountAndOwnerRecoveryPreserveWorkspaceAndOtherIdentities(t *testing.T) {
	g, h, oldToken, owner := fixture(t, nil)
	g.opts.LocalAccessToken = localTestCapability
	beforePolicy := g.state.Policy
	marker := filepath.Join(g.opts.StateDir, "saved-project-proof")
	if err := os.WriteFile(marker, []byte("existing project"), 0600); err != nil {
		t.Fatal(err)
	}
	w := localRequest(t, h, "POST", "/v1/auth/local-account", localTestCapability, "127.0.0.1:1", map[string]string{"username": "another", "displayName": "Another", "password": testPassword}, 201)
	var other struct {
		Token string `json:"token"`
		User  User   `json:"user"`
	}
	json.Unmarshal(w.Body.Bytes(), &other)
	if g.state.OwnerID != owner.ID || other.User.ID == owner.ID || other.User.Role != "super-admin" {
		t.Fatal("additional account replaced owner or has wrong privilege")
	}
	request(t, h, "GET", "/v1/auth/me", oldToken, nil, 200)
	ticket := request(t, h, "POST", "/v1/auth/socket-ticket", oldToken, map[string]string{"path": "/v1/sessions/session-1/attach"}, 200)
	if ticket.Body.Len() == 0 {
		t.Fatal("ticket missing")
	}
	server, peer := net.Pipe()
	defer peer.Close()
	g.sockets[owner.ID] = map[net.Conn]string{server: hashToken(oldToken)}
	newPassword := "a replacement owner password"
	recovered := localRequest(t, h, "POST", "/v1/auth/local-owner/recover", localTestCapability, "127.0.0.1:1", map[string]string{"newPassword": newPassword}, 200)
	var auth struct {
		Token string `json:"token"`
		User  User   `json:"user"`
	}
	json.Unmarshal(recovered.Body.Bytes(), &auth)
	if auth.User.ID != owner.ID || auth.User.Username != owner.Username || g.state.OwnerID != owner.ID || !reflect.DeepEqual(beforePolicy, g.state.Policy) {
		t.Fatal("recovery changed owner or policy")
	}
	request(t, h, "GET", "/v1/auth/me", oldToken, nil, 401)
	request(t, h, "GET", "/v1/auth/me", auth.Token, nil, 200)
	request(t, h, "GET", "/v1/auth/me", other.Token, nil, 200)
	request(t, h, "POST", "/v1/auth/login", "", map[string]string{"username": owner.Username, "password": testPassword}, 401)
	request(t, h, "POST", "/v1/auth/login", "", map[string]string{"username": owner.Username, "password": newPassword}, 200)
	if len(g.tickets) != 0 {
		t.Fatal("recovery retained owner socket tickets")
	}
	peer.SetReadDeadline(time.Now().Add(time.Second))
	var b [1]byte
	if _, err := peer.Read(b[:]); err != io.EOF {
		t.Fatalf("owner socket not closed: %v", err)
	}
	data, _ := os.ReadFile(marker)
	if string(data) != "existing project" {
		t.Fatal("project state changed")
	}
	disk, _ := os.ReadFile(filepath.Join(g.dir, "identity.json"))
	if bytes.Contains(disk, []byte(newPassword)) || bytes.Contains(disk, []byte(auth.Token)) || bytes.Contains(disk, []byte(localTestCapability)) {
		t.Fatal("secret persisted in identity")
	}
	reopened, err := New(g.opts)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close(context.Background())
	request(t, reopened.Handler(), "GET", "/v1/auth/me", auth.Token, nil, 200)
}

func TestLocalRecoveryFailedPersistenceLeavesOldCredentialsAndSessionsIntact(t *testing.T) {
	g, h, token, owner := fixture(t, nil)
	g.opts.LocalAccessToken = localTestCapability
	before := cloneIdentityState(g.state)
	bad := filepath.Join(t.TempDir(), "not-a-directory")
	os.WriteFile(bad, []byte("file"), 0600)
	dir := g.dir
	g.dir = bad
	localRequest(t, h, "POST", "/v1/auth/local-owner/recover", localTestCapability, "127.0.0.1:1", map[string]string{"newPassword": "a changed password that cannot persist"}, 500)
	g.dir = dir
	if !reflect.DeepEqual(before, g.state) {
		t.Fatal("failed recovery mutated live identity")
	}
	request(t, h, "GET", "/v1/auth/me", token, nil, 200)
	if !passwordMatches(testPassword, g.state.Users[owner.ID].PasswordHash) {
		t.Fatal("old credentials invalidated on save failure")
	}
}
