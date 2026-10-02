package management

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func invite(t *testing.T, h http.Handler, token, role string) (Invitation, string) {
	t.Helper()
	w := request(t, h, "POST", "/v1/admin/invitations", token, map[string]string{"role": role}, 201)
	var v struct {
		Invitation Invitation `json:"invitation"`
		InviteCode string     `json:"inviteCode"`
	}
	if json.Unmarshal(w.Body.Bytes(), &v) != nil || v.InviteCode == "" {
		t.Fatal("invalid invitation result")
	}
	return v.Invitation, v.InviteCode
}
func acceptBody(code, name string) map[string]string {
	return map[string]string{"inviteCode": code, "username": name, "displayName": name, "password": testPassword}
}

func TestInvitationsArePersistedHashedSingleUseAndDoNotAcceptRoles(t *testing.T) {
	g, h, ownerToken, owner := fixture(t, nil)
	row, code := invite(t, h, ownerToken, "member")
	if row.ExpiresAt.Sub(row.CreatedAt) != 48*time.Hour || row.CreatedBy != owner.ID {
		t.Fatal("wrong invitation expiry or attribution")
	}
	data, _ := os.ReadFile(filepath.Join(g.dir, "identity.json"))
	if bytes.Contains(data, []byte(code)) || !bytes.Contains(data, []byte(hashToken(code))) {
		t.Fatal("invitation code not hashed at rest")
	}
	listed := request(t, h, "GET", "/v1/admin/invitations", ownerToken, nil, 200).Body.String()
	if strings.Contains(listed, code) || strings.Contains(listed, hashToken(code)) || strings.Contains(listed, "codeHash") {
		t.Fatal("invitation secret disclosed by list")
	}
	injected := acceptBody(code, "invited")
	injected["role"] = "super-admin"
	request(t, h, "POST", "/v1/auth/invitations/accept", "", injected, 400)
	w := request(t, h, "POST", "/v1/auth/invitations/accept", "", acceptBody(code, "invited"), 201)
	var auth struct {
		Token string `json:"token"`
		User  User   `json:"user"`
	}
	json.Unmarshal(w.Body.Bytes(), &auth)
	if auth.User.Role != "member" || g.state.OwnerID != owner.ID {
		t.Fatal("redemption escalated role or replaced owner")
	}
	request(t, h, "GET", "/v1/auth/me", auth.Token, nil, 200)
	request(t, h, "GET", "/v1/admin/invitations", auth.Token, nil, 403)
	request(t, h, "POST", "/v1/admin/invitations", auth.Token, map[string]string{"role": "member"}, 403)
	request(t, h, "POST", "/v1/auth/invitations/accept", "", acceptBody(code, "reused"), 403)
	reopened, err := New(g.opts)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close(context.Background())
	request(t, reopened.Handler(), "POST", "/v1/auth/invitations/accept", "", acceptBody(code, "afterrestart"), 403)
	if reopened.state.Invitations[row.ID].AcceptedBy != auth.User.ID {
		t.Fatal("redemption attribution not durable")
	}
}

func TestInvitationsRejectExpiredRevokedAndChangedIssuers(t *testing.T) {
	g, h, token, _ := fixture(t, nil)
	admin := addUser(t, h, token, "admin", "admin")
	adminToken := loginAs(t, h, "admin")
	request(t, h, "POST", "/v1/admin/invitations", adminToken, map[string]string{"role": "admin"}, 403)
	request(t, h, "POST", "/v1/admin/invitations", adminToken, map[string]string{"role": "super-admin"}, 403)
	for _, mode := range []string{"expired", "revoked", "disabled", "demoted"} {
		row, code := invite(t, h, adminToken, "member")
		switch mode {
		case "expired":
			current := g.state.Invitations[row.ID]
			current.ExpiresAt = time.Now().Add(-time.Second)
			g.state.Invitations[row.ID] = current
		case "revoked":
			request(t, h, "DELETE", "/v1/admin/invitations/"+row.ID, adminToken, nil, 200)
		case "disabled":
			current := g.state.Users[admin.ID]
			current.User.Disabled = true
			g.state.Users[admin.ID] = current
		case "demoted":
			current := g.state.Users[admin.ID]
			current.User.Role = "member"
			g.state.Users[admin.ID] = current
		}
		request(t, h, "POST", "/v1/auth/invitations/accept", "", acceptBody(code, "candidate-"+mode), 403)
		restored := g.state.Users[admin.ID]
		restored.User = admin
		g.state.Users[admin.ID] = restored
	}
	request(t, h, "POST", "/v1/auth/invitations/accept", "", acceptBody(strings.Repeat("x", 43), "unknown"), 403)
}

func TestInvitationsConcurrentRedemptionCreatesExactlyOneAccount(t *testing.T) {
	g, h, token, _ := fixture(t, nil)
	_, code := invite(t, h, token, "member")
	start := make(chan struct{})
	codes := make(chan int, 2)
	var wg sync.WaitGroup
	for _, name := range []string{"first", "second"} {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			<-start
			data, _ := json.Marshal(acceptBody(code, name))
			r := httptest.NewRequest("POST", "/v1/auth/invitations/accept", bytes.NewReader(data))
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			codes <- w.Code
		}(name)
	}
	close(start)
	wg.Wait()
	close(codes)
	successes, denials := 0, 0
	for code := range codes {
		if code == 201 {
			successes++
		} else if code == 403 {
			denials++
		} else {
			t.Fatalf("unexpected concurrent response%d", code)
		}
	}
	if successes != 1 || denials != 1 || len(g.state.Users) != 2 {
		t.Fatal("invitation redemption was not atomic")
	}
}

func TestInvitationSaveFailureDoesNotConsumeCodeOrCreateUser(t *testing.T) {
	g, h, token, _ := fixture(t, nil)
	row, code := invite(t, h, token, "member")
	bad := filepath.Join(t.TempDir(), "not-a-directory")
	os.WriteFile(bad, []byte("file"), 0600)
	dir := g.dir
	g.dir = bad
	request(t, h, "POST", "/v1/auth/invitations/accept", "", acceptBody(code, "retryable"), 500)
	g.dir = dir
	if len(g.state.Users) != 1 || g.state.Invitations[row.ID].AcceptedAt != nil {
		t.Fatal("failed write consumed invitation")
	}
	request(t, h, "POST", "/v1/auth/invitations/accept", "", acceptBody(code, "retryable"), 201)
}
