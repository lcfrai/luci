package management

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Force the final atomic rename to fail while preserving the prior durable
// identity. Directory permissions alone do not fail when tests run as root.
func blockIdentityCommit(t *testing.T, g *Gateway) func() {
	t.Helper()
	path := filepath.Join(g.dir, "identity.json")
	saved := path + ".test-retained"
	hadIdentity := true
	if err := os.Rename(path, saved); os.IsNotExist(err) {
		hadIdentity = false
	} else if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	var once sync.Once
	restore := func() {
		t.Helper()
		once.Do(func() {
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if hadIdentity {
				if err := os.Rename(saved, path); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
	t.Cleanup(restore)
	return restore
}

func assertIdentityUnchanged(t *testing.T, g *Gateway, before storedState) {
	t.Helper()
	if !reflect.DeepEqual(before, g.state) {
		t.Fatal("failed persistence changed the live identity, sessions or audit history")
	}
}

func TestBootstrapPersistenceFailureKeepsFirstOwnerRetryable(t *testing.T) {
	g, err := New(Options{StateDir: t.TempDir(), InitialHandler: http.NotFoundHandler()})
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close(context.Background())
	h := g.Handler()
	before := cloneIdentityState(g.state)
	bootstrap := g.bootstrapToken
	restore := blockIdentityCommit(t, g)
	body := map[string]string{"bootstrapToken": bootstrap, "username": "owner", "displayName": "Owner", "password": testPassword}
	request(t, h, "POST", "/v1/auth/bootstrap", "", body, 500)
	assertIdentityUnchanged(t, g, before)
	if g.bootstrapToken != bootstrap {
		t.Fatal("failed bootstrap consumed the owner capability")
	}
	if _, err := os.Stat(filepath.Join(g.dir, "bootstrap-token")); err != nil {
		t.Fatal("failed bootstrap removed the owner's bootstrap file")
	}
	status := request(t, h, "GET", "/v1/control-plane/status", "", nil, 200)
	if !bytes.Contains(status.Body.Bytes(), []byte(`"bootstrapRequired":true`)) {
		t.Fatal("failed bootstrap reported an installed owner")
	}
	restore()
	request(t, h, "POST", "/v1/auth/bootstrap", "", body, 201)
}

func TestLoginPersistenceFailurePreservesExistingSessionsAndAudit(t *testing.T) {
	g, h, token, owner := fixture(t, nil)
	// A new login would evict the earliest of these sixteen live sessions and
	// prune the expired one. Neither may happen when its disk commit fails.
	for i := 0; i < 15; i++ {
		g.state.Sessions[hashToken(randomToken())] = sessionRecord{UserID: owner.ID, ExpiresAt: time.Now().Add(time.Duration(i+1) * time.Minute)}
	}
	g.state.Sessions[hashToken("expired-session")] = sessionRecord{UserID: owner.ID, ExpiresAt: time.Now().Add(-time.Minute)}
	if err := g.persistLocked(); err != nil {
		t.Fatal(err)
	}
	before := cloneIdentityState(g.state)
	restore := blockIdentityCommit(t, g)
	request(t, h, "POST", "/v1/auth/login", "", map[string]string{"username": owner.Username, "password": testPassword}, 500)
	assertIdentityUnchanged(t, g, before)
	request(t, h, "GET", "/v1/auth/me", token, nil, 200)
	request(t, h, "POST", "/v1/auth/login", "", map[string]string{"username": owner.Username, "password": "incorrect"}, 401)
	assertIdentityUnchanged(t, g, before)
	restore()
	loginAs(t, h, owner.Username)
}

func TestSessionEvictionRevokesOnlyItsConnectionsAfterSuccessfulCommit(t *testing.T) {
	g, h, token, owner := fixture(t, nil)
	old := g.state.Sessions[hashToken(token)]
	old.ExpiresAt = time.Now().Add(time.Minute)
	g.state.Sessions[hashToken(token)] = old
	for i := 0; i < 15; i++ {
		g.state.Sessions[hashToken(randomToken())] = sessionRecord{UserID: owner.ID, ExpiresAt: time.Now().Add(time.Duration(i+2) * time.Minute)}
	}
	if err := g.persistLocked(); err != nil {
		t.Fatal(err)
	}
	request(t, h, "POST", "/v1/auth/socket-ticket", token, map[string]string{"path": "/v1/sessions/owned/attach"}, 200)
	conn, stopped := observeIdentityRuntime(t, g, owner, token)
	restore := blockIdentityCommit(t, g)
	request(t, h, "POST", "/v1/auth/login", "", map[string]string{"username": owner.Username, "password": testPassword}, 500)
	if conn.closed.Load() || stopped.Load() || len(g.tickets) != 1 {
		t.Fatal("failed login evicted a live terminal or its reconnect ticket")
	}
	request(t, h, "GET", "/v1/auth/me", token, nil, 200)
	restore()
	currentToken := loginAs(t, h, owner.Username)
	if !conn.closed.Load() || stopped.Load() || len(g.tickets) != 0 {
		t.Fatal("committed session eviction did not revoke only its connections")
	}
	request(t, h, "GET", "/v1/auth/me", token, nil, 401)
	request(t, h, "GET", "/v1/auth/me", currentToken, nil, 200)
}

type observedIdentitySocket struct {
	net.Conn
	closed atomic.Bool
}

func (c *observedIdentitySocket) Close() error {
	c.closed.Store(true)
	return c.Conn.Close()
}

// Register an owned socket and worker without spawning an OS process. The
// observable close/kill callbacks prove that rollback preserves live work.
func observeIdentityRuntime(t *testing.T, g *Gateway, user User, token string) (*observedIdentitySocket, *atomic.Bool) {
	t.Helper()
	server, client := net.Pipe()
	conn := &observedIdentitySocket{Conn: server}
	g.sockets[user.ID] = map[net.Conn]string{conn: hashToken(token)}
	stopped := &atomic.Bool{}
	g.runtimes[user.ID] = &userRuntime{user: user, status: RuntimeStatus{Status: "running"}, done: make(chan struct{}), kill: func() { stopped.Store(true) }}
	t.Cleanup(func() {
		delete(g.runtimes, user.ID)
		delete(g.sockets, user.ID)
		conn.Close()
		client.Close()
	})
	return conn, stopped
}

func TestFailedAccountMutationPreservesLiveAccessUntilDurableCommit(t *testing.T) {
	for _, operation := range []string{"logout", "password", "disable", "role", "reset-password"} {
		t.Run(operation, func(t *testing.T) {
			g, h, ownerToken, owner := fixture(t, nil)
			u, token := owner, ownerToken
			method, path, status := "POST", "/v1/auth/"+operation, 200
			body := any(map[string]string{})
			switch operation {
			case "password":
				body = map[string]string{"currentPassword": testPassword, "newPassword": "a replacement owner password"}
			case "disable", "role", "reset-password":
				u = addUser(t, h, ownerToken, "member", "member")
				token = loginAs(t, h, u.Username)
				method, path = "PATCH", "/v1/admin/users/"+u.ID
				if operation == "disable" {
					body = map[string]bool{"disabled": true}
				} else if operation == "role" {
					body = map[string]string{"role": "admin"}
				} else {
					body = map[string]string{"password": "a replacement member password"}
				}
			}
			request(t, h, "POST", "/v1/auth/socket-ticket", token, map[string]string{"path": "/v1/sessions/owned/attach"}, 200)
			before := cloneIdentityState(g.state)
			tickets := make(map[string]socketTicket, len(g.tickets))
			for key, value := range g.tickets {
				tickets[key] = value
			}
			conn, stopped := observeIdentityRuntime(t, g, u, token)
			restore := blockIdentityCommit(t, g)
			request(t, h, method, path, ownerToken, body, 500)
			assertIdentityUnchanged(t, g, before)
			if !reflect.DeepEqual(tickets, g.tickets) || conn.closed.Load() || stopped.Load() {
				t.Fatal("failed identity commit consumed socket tickets or interrupted live work")
			}
			request(t, h, "GET", "/v1/auth/me", token, nil, 200)
			restore()
			// A restart must agree with the identity still served in memory.
			reopened, err := New(g.opts)
			if err != nil {
				t.Fatal(err)
			}
			request(t, reopened.Handler(), "GET", "/v1/auth/me", token, nil, 200)
			reopened.Close(context.Background())
			request(t, h, method, path, ownerToken, body, status)
			request(t, h, "GET", "/v1/auth/me", token, nil, 401)
			if !conn.closed.Load() || len(g.tickets) != 0 {
				t.Fatal("successful identity commit left revoked connections usable")
			}
			if stopped.Load() != (operation != "logout") {
				t.Fatal("worker stop did not follow the successfully committed account mutation")
			}
		})
	}
}

func TestFailedAdministratorWritesLeaveNoUsersOrPolicyChanges(t *testing.T) {
	g, h, token, _ := fixture(t, nil)
	before := cloneIdentityState(g.state)
	restore := blockIdentityCommit(t, g)
	newUser := map[string]string{"username": "new-user", "displayName": "New user", "password": testPassword, "role": "admin"}
	request(t, h, "POST", "/v1/admin/users", token, newUser, 500)
	assertIdentityUnchanged(t, g, before)
	policy := map[string]any{"allowHostCommands": true, "allowedHarnesses": []string{"shell"}, "maxSessionsPerUser": 2}
	request(t, h, "PATCH", "/v1/admin/policy", token, policy, 500)
	assertIdentityUnchanged(t, g, before)
	restore()
	request(t, h, "POST", "/v1/admin/users", token, newUser, 201)
	request(t, h, "PATCH", "/v1/admin/policy", token, policy, 200)
	if !g.state.Policy.AllowHostCommands || !reflect.DeepEqual(g.state.Policy.AllowedHarnesses, []string{"shell"}) {
		t.Fatal("successful policy retry was not applied")
	}
}
