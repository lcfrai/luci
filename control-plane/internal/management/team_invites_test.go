package management

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func inviteToTeam(t *testing.T, h http.Handler, token string, team Team, role string, grants TeamGrants) (Team, Invitation, string) {
	t.Helper()
	w := request(t, h, "POST", "/v1/teams/"+team.ID+"/invitations", token, map[string]any{"expectedRevision": team.Revision, "role": role, "grants": grants}, 201)
	var result struct {
		Team       Team       `json:"team"`
		Invitation Invitation `json:"invitation"`
		Code       string     `json:"inviteCode"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || result.Code == "" {
		t.Fatalf("invalid team invitation: %s", w.Body.String())
	}
	return result.Team, result.Invitation, result.Code
}
func acceptTeam(t *testing.T, h http.Handler, token, code string, status int) *httptest.ResponseRecorder {
	t.Helper()
	return request(t, h, "POST", "/v1/teams/invitations/accept", token, map[string]string{"inviteCode": code}, status)
}

func TestTeamInvitationsCreateNewMembersWithoutInstanceAdministration(t *testing.T) {
	g, h, rootToken, root := fixture(t, nil)
	lead := addUser(t, h, rootToken, "team-lead", "member")
	leadToken := loginAs(t, h, lead.Username)
	team := createTestTeam(t, h, leadToken, "Research team")
	team, invitation, code := inviteToTeam(t, h, leadToken, team, "editor", TeamGrants{Execute: true})
	if invitation.Role != "member" || invitation.Team.ID != team.ID || invitation.Team.Name != team.Name || invitation.CreatedBy != lead.ID || invitation.ExpiresAt.Sub(invitation.CreatedAt) != 48*time.Hour {
		t.Fatalf("incorrect pinned invitation: %+v", invitation)
	}
	request(t, h, "POST", "/v1/admin/invitations", leadToken, map[string]string{"role": "member"}, 403)
	request(t, h, "GET", "/v1/teams/"+team.ID+"/invitations", rootToken, nil, 403)
	request(t, h, "DELETE", "/v1/admin/invitations/"+invitation.ID, rootToken, nil, 404)
	global := request(t, h, "GET", "/v1/admin/invitations", rootToken, nil, 200).Body.String()
	if strings.Contains(global, invitation.ID) {
		t.Fatal("team invitation leaked to unrelated instance administrator")
	}
	listed := request(t, h, "GET", "/v1/teams/"+team.ID+"/invitations", leadToken, nil, 200).Body.String()
	if strings.Contains(listed, code) || strings.Contains(listed, hashToken(code)) || strings.Contains(listed, "issuerMembershipRevision") || strings.Contains(listed, "codeHash") {
		t.Fatal("private invitation authority leaked")
	}
	data, _ := os.ReadFile(filepath.Join(g.dir, "identity.json"))
	if bytes.Contains(data, []byte(code)) || !bytes.Contains(data, []byte(hashToken(code))) {
		t.Fatal("raw invite persisted")
	}
	injected := acceptBody(code, "new-collaborator")
	injected["teamId"] = "another-team"
	request(t, h, "POST", "/v1/auth/invitations/accept", "", injected, 400)
	w := request(t, h, "POST", "/v1/auth/invitations/accept", "", acceptBody(code, "new-collaborator"), 201)
	var auth struct {
		Token  string `json:"token"`
		User   User   `json:"user"`
		TeamID string `json:"teamId"`
	}
	json.Unmarshal(w.Body.Bytes(), &auth)
	if auth.User.Role != "member" || auth.TeamID != team.ID || g.state.OwnerID != root.ID {
		t.Fatal("team invite escalated instance identity")
	}
	access, err := g.resolveTeamAccess(team.ID, auth.User.ID, "")
	if err != nil || !access.Permissions.Edit || !access.Permissions.Execute || access.Permissions.Admin || access.Permissions.Secrets || access.Permissions.Deploy {
		t.Fatalf("grants not exact: %+v %v", access, err)
	}
	if _, err = g.resolveTeamAccess(team.ID, auth.User.ID, "p-existing"); err == nil {
		t.Fatal("invite implicitly granted existing project access")
	}
	request(t, h, "GET", "/v1/admin/users", auth.Token, nil, 403)
	request(t, h, "POST", "/v1/auth/invitations/accept", "", acceptBody(code, "reused"), 403)
	if len(g.runtimes) != 0 {
		t.Fatal("inviting or joining started a workspace")
	}
	reopened, err := New(g.opts)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close(context.Background())
	request(t, reopened.Handler(), "GET", "/v1/teams/"+team.ID, auth.Token, nil, 200)
	if reopened.state.Invitations[invitation.ID].AcceptedBy != auth.User.ID {
		t.Fatal("membership and receipt did not persist together")
	}
}

func TestTeamInvitationOwnerScopeRevisionAndRevocation(t *testing.T) {
	g, h, token, owner := fixture(t, nil)
	admin := addUser(t, h, token, "delegated", "member")
	viewer := addUser(t, h, token, "reader", "member")
	adminToken, viewerToken := loginAs(t, h, admin.Username), loginAs(t, h, viewer.Username)
	team := createTestTeam(t, h, token, "One")
	base := "/v1/teams/" + team.ID
	team = teamFromResponse(t, request(t, h, "POST", base+"/members", token, teamMemberBody(team.Revision, admin.Username, "editor", TeamGrants{Admin: true}), 200))
	team = teamFromResponse(t, request(t, h, "POST", base+"/members", token, teamMemberBody(team.Revision, viewer.Username, "viewer", TeamGrants{}), 200))
	body := map[string]any{"expectedRevision": team.Revision, "role": "owner", "grants": TeamGrants{Admin: true}}
	request(t, h, "POST", base+"/invitations", adminToken, body, 403)
	request(t, h, "POST", base+"/invitations", viewerToken, body, 403)
	request(t, h, "GET", base+"/invitations", viewerToken, nil, 403)
	beforeRevision := team.Revision
	team, row, code := inviteToTeam(t, h, token, team, "owner", TeamGrants{Admin: true})
	request(t, h, "POST", base+"/invitations", token, map[string]any{"expectedRevision": beforeRevision, "role": "viewer", "grants": TeamGrants{}}, 409)
	request(t, h, "DELETE", base+"/invitations/"+row.ID, adminToken, map[string]int{"expectedRevision": team.Revision}, 403)
	other := createTestTeam(t, h, token, "Two")
	request(t, h, "DELETE", "/v1/teams/"+other.ID+"/invitations/"+row.ID, token, map[string]int{"expectedRevision": other.Revision}, 404)
	request(t, h, "DELETE", base+"/invitations/"+row.ID, token, map[string]int{"expectedRevision": team.Revision}, 200)
	acceptTeam(t, h, viewerToken, code, 403)
	if len(g.state.Teams[team.ID].Members) != 3 || g.state.Teams[team.ID].Members[owner.ID].Role != "owner" {
		t.Fatal("revocation changed membership")
	}
}

func TestTeamInvitationsRejectChangedIssuerAndExpiredCodes(t *testing.T) {
	for _, mode := range []string{"expired", "disabled", "removed", "changed-and-restored"} {
		t.Run(mode, func(t *testing.T) {
			g, h, token, owner := fixture(t, nil)
			admin := addUser(t, h, token, "delegated", "member")
			adminToken := loginAs(t, h, admin.Username)
			team := createTestTeam(t, h, token, "Team")
			base := "/v1/teams/" + team.ID
			team = teamFromResponse(t, request(t, h, "POST", base+"/members", token, teamMemberBody(team.Revision, admin.Username, "editor", TeamGrants{Admin: true}), 200))
			team, invitation, code := inviteToTeam(t, h, adminToken, team, "editor", TeamGrants{})
			switch mode {
			case "expired":
				row := g.state.Invitations[invitation.ID]
				row.CreatedAt = time.Now().Add(-49 * time.Hour)
				row.ExpiresAt = time.Now().Add(-time.Hour)
				g.state.Invitations[invitation.ID] = row
			case "disabled":
				user := g.state.Users[admin.ID]
				user.User.Disabled = true
				g.state.Users[admin.ID] = user
			case "removed":
				request(t, h, "DELETE", base+"/members/"+admin.ID, token, map[string]int{"expectedRevision": team.Revision}, 200)
			case "changed-and-restored":
				team = teamFromResponse(t, request(t, h, "PUT", base+"/members/"+admin.ID, token, teamMemberBody(team.Revision, "", "editor", TeamGrants{}), 200))
				request(t, h, "PUT", base+"/members/"+admin.ID, token, teamMemberBody(team.Revision, "", "editor", TeamGrants{Admin: true}), 200)
			}
			request(t, h, "POST", "/v1/auth/invitations/accept", "", acceptBody(code, "candidate"), 403)
			acceptTeam(t, h, token, code, 403)
			if g.state.Invitations[invitation.ID].AcceptedAt != nil || len(g.state.Users) != 2 || g.state.Teams[team.ID].Members[owner.ID].Role != "owner" {
				t.Fatal("invalid invite consumed or changed identity")
			}
			list := request(t, h, "GET", base+"/invitations", token, nil, 200).Body.String()
			if !strings.Contains(list, `"status":"expired"`) && !strings.Contains(list, `"status":"unavailable"`) {
				t.Fatal("invalid pending invite shown available")
			}
		})
	}
}

func TestExistingTeamInvitationIsAtomicAndCannotOverwriteMembership(t *testing.T) {
	g, h, token, owner := fixture(t, nil)
	member := addUser(t, h, token, "existing", "member")
	memberToken := loginAs(t, h, member.Username)
	team := createTestTeam(t, h, token, "Team")
	team, invitation, code := inviteToTeam(t, h, token, team, "viewer", TeamGrants{})
	acceptTeam(t, h, token, code, 409)
	if g.state.Invitations[invitation.ID].AcceptedAt != nil || g.state.Teams[team.ID].Members[owner.ID].Role != "owner" {
		t.Fatal("invite demoted existing owner or consumed retryable code")
	}
	acceptTeam(t, h, memberToken, code, 200)
	acceptTeam(t, h, memberToken, code, 403)
	request(t, h, "DELETE", "/v1/teams/"+team.ID+"/invitations/"+invitation.ID, token, map[string]int{"expectedRevision": g.state.Teams[team.ID].Team.Revision}, 409)
	if len(g.state.Users) != 2 || g.state.Teams[team.ID].Members[member.ID].Role != "viewer" {
		t.Fatal("existing acceptance changed account")
	}
	_, globalCode := invite(t, h, token, "member")
	acceptTeam(t, h, memberToken, globalCode, 403)
}

func TestTeamInvitationConcurrentAcceptanceJoinsExactlyOneAccount(t *testing.T) {
	g, h, token, _ := fixture(t, nil)
	first := addUser(t, h, token, "first", "member")
	second := addUser(t, h, token, "second", "member")
	tokens := []string{loginAs(t, h, first.Username), loginAs(t, h, second.Username)}
	team := createTestTeam(t, h, token, "Team")
	_, _, code := inviteToTeam(t, h, token, team, "editor", TeamGrants{})
	start := make(chan struct{})
	statuses := make(chan int, 2)
	var wg sync.WaitGroup
	for _, candidate := range tokens {
		wg.Add(1)
		go func(token string) {
			defer wg.Done()
			<-start
			body, _ := json.Marshal(map[string]string{"inviteCode": code})
			r := httptest.NewRequest("POST", "/v1/teams/invitations/accept", bytes.NewReader(body))
			r.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			statuses <- w.Code
		}(candidate)
	}
	close(start)
	wg.Wait()
	a, b := <-statuses, <-statuses
	if !((a == 200 && b == 403) || (a == 403 && b == 200)) || len(g.state.Teams[team.ID].Members) != 2 {
		t.Fatalf("non-atomic team acceptance: %d,%d", a, b)
	}
}

func TestTeamInvitationFailuresRollbackMembershipConsumptionAndAccount(t *testing.T) {
	g, h, token, _ := fixture(t, nil)
	member := addUser(t, h, token, "existing", "member")
	memberToken := loginAs(t, h, member.Username)
	team := createTestTeam(t, h, token, "Team")
	team, invitation, code := inviteToTeam(t, h, token, team, "editor", TeamGrants{})
	dir := g.dir
	bad := filepath.Join(t.TempDir(), "not-a-directory")
	os.WriteFile(bad, []byte("file"), 0600)
	for _, mode := range []string{"create", "revoke", "new-account", "existing-account"} {
		t.Run(mode, func(t *testing.T) {
			before := cloneIdentityState(g.state)
			g.dir = bad
			switch mode {
			case "create":
				request(t, h, "POST", "/v1/teams/"+team.ID+"/invitations", token, map[string]any{"expectedRevision": team.Revision, "role": "viewer", "grants": TeamGrants{}}, 500)
			case "revoke":
				request(t, h, "DELETE", "/v1/teams/"+team.ID+"/invitations/"+invitation.ID, token, map[string]int{"expectedRevision": team.Revision}, 500)
			case "new-account":
				request(t, h, "POST", "/v1/auth/invitations/accept", "", acceptBody(code, "candidate"), 500)
			case "existing-account":
				acceptTeam(t, h, memberToken, code, 500)
			}
			g.dir = dir
			if !reflect.DeepEqual(g.state, before) {
				t.Fatal("failed persistence changed identities, membership or invite")
			}
		})
	}
	acceptTeam(t, h, memberToken, code, 200)
}
