package management

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
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

func teamFromResponse(t *testing.T, response *httptest.ResponseRecorder) Team {
	t.Helper()
	var body struct {
		Team Team `json:"team"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || body.Team.ID == "" {
		t.Fatalf("invalid team response: %s", response.Body.String())
	}
	return body.Team
}

func createTestTeam(t *testing.T, h http.Handler, token, name string) Team {
	t.Helper()
	return teamFromResponse(t, request(t, h, "POST", "/v1/teams", token, map[string]any{"name": name}, 201))
}

func teamMemberBody(revision int, username, role string, grants TeamGrants) map[string]any {
	return map[string]any{"expectedRevision": revision, "username": username, "role": role, "grants": grants}
}

func TestTeamMembershipIsPrivatePersistentAndIndependentOfInstanceRole(t *testing.T) {
	g, h, ownerToken, owner := fixture(t, nil)
	member := addUser(t, h, ownerToken, "collaborator", "member")
	outsider := addUser(t, h, ownerToken, "outsider-admin", "admin")
	memberToken, outsiderToken := loginAs(t, h, member.Username), loginAs(t, h, outsider.Username)
	team := createTestTeam(t, h, ownerToken, " Shared project team ")
	if team.Name != "Shared project team" || team.Revision != 1 || !teamIDPattern.MatchString(team.ID) {
		t.Fatalf("invalid created team: %+v", team)
	}
	if len(g.runtimes) != 0 {
		t.Fatal("creating membership started an execution worker")
	}
	base := "/v1/teams/" + team.ID
	request(t, h, "GET", base, outsiderToken, nil, 404)
	if body := request(t, h, "GET", "/v1/teams", outsiderToken, nil, 200).Body.String(); strings.Contains(body, team.ID) {
		t.Fatal("instance admin discovered a private team without membership")
	}
	request(t, h, "POST", base+"/members", outsiderToken, teamMemberBody(team.Revision, outsider.Username, "owner", TeamGrants{Admin: true}), 403)
	team = teamFromResponse(t, request(t, h, "POST", base+"/members", ownerToken, teamMemberBody(team.Revision, member.Username, "editor", TeamGrants{Execute: true}), 200))
	view := request(t, h, "GET", base, memberToken, nil, 200)
	if strings.Contains(view.Body.String(), "passwordHash") || strings.Contains(view.Body.String(), testPassword) || strings.Contains(view.Body.String(), ownerToken) {
		t.Fatal("team membership projection exposed an authentication secret")
	}
	request(t, h, "PATCH", base, memberToken, map[string]any{"expectedRevision": team.Revision, "name": "unauthorized rename"}, 403)
	access, err := g.resolveTeamAccess(team.ID, member.ID, "")
	if err != nil || !access.Permissions.Edit || !access.Permissions.Execute || access.Permissions.Secrets || access.Permissions.Deploy || access.Permissions.Admin {
		t.Fatalf("membership grants were not retained exactly: %+v %v", access, err)
	}
	if _, err := g.resolveTeamAccess(team.ID, member.ID, "p-private"); err == nil {
		t.Fatal("team membership implied access to every project")
	}
	if _, err := g.resolveTeamAccess(team.ID, owner.ID, "p-private"); err != nil {
		t.Fatalf("team owner with explicit admin lost team-wide project authority: %v", err)
	}
	reopened, err := New(g.opts)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close(context.Background())
	request(t, reopened.Handler(), "GET", base, memberToken, nil, 200)
	request(t, reopened.Handler(), "GET", base, outsiderToken, nil, 404)
	data, err := os.ReadFile(filepath.Join(g.dir, "identity.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte(team.ID)) || bytes.Contains(data, []byte(memberToken)) {
		t.Fatal("membership did not persist privately without raw session credentials")
	}
}

func TestTeamCompareAndSwapRejectsConcurrentLostUpdates(t *testing.T) {
	g, h, token, _ := fixture(t, nil)
	team := createTestTeam(t, h, token, "Original name")
	statuses := make(chan int, 2)
	var start sync.WaitGroup
	start.Add(1)
	for _, name := range []string{"First proposed name", "Second proposed name"} {
		go func(name string) {
			start.Wait()
			data, _ := json.Marshal(map[string]any{"expectedRevision": team.Revision, "name": name})
			r := httptest.NewRequest("PATCH", "/v1/teams/"+team.ID, bytes.NewReader(data))
			r.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			statuses <- w.Code
		}(name)
	}
	start.Done()
	first, second := <-statuses, <-statuses
	if !((first == 200 && second == 409) || (first == 409 && second == 200)) {
		t.Fatalf("concurrent updates returned %d and %d", first, second)
	}
	if got := g.state.Teams[team.ID].Team; got.Revision != team.Revision+1 || got.Name == team.Name {
		t.Fatalf("lost update or incorrect revision: %+v", got)
	}
	before := cloneIdentityState(g.state)
	request(t, h, "PATCH", "/v1/teams/"+team.ID, token, map[string]any{"expectedRevision": team.Revision, "name": "stale overwrite"}, 409)
	assertIdentityUnchanged(t, g, before)
}

func TestTeamOwnerCannotBeRemovedOrDemotedWithoutAnotherOwner(t *testing.T) {
	g, h, token, owner := fixture(t, nil)
	admin := addUser(t, h, token, "team-admin", "member")
	adminToken := loginAs(t, h, admin.Username)
	team := createTestTeam(t, h, token, "Owner continuity")
	base := "/v1/teams/" + team.ID
	team = teamFromResponse(t, request(t, h, "POST", base+"/members", token, teamMemberBody(team.Revision, admin.Username, "editor", TeamGrants{Admin: true}), 200))
	before := cloneIdentityState(g.state)
	request(t, h, "DELETE", base+"/members/"+owner.ID, token, map[string]int{"expectedRevision": team.Revision}, 409)
	assertIdentityUnchanged(t, g, before)
	request(t, h, "PUT", base+"/members/"+owner.ID, token, teamMemberBody(team.Revision, "", "editor", TeamGrants{Admin: true}), 409)
	assertIdentityUnchanged(t, g, before)
	request(t, h, "PUT", base+"/members/"+admin.ID, adminToken, teamMemberBody(team.Revision, "", "owner", TeamGrants{Admin: true}), 403)
	request(t, h, "DELETE", base+"/members/"+owner.ID, adminToken, map[string]int{"expectedRevision": team.Revision}, 403)
	assertIdentityUnchanged(t, g, before)
	team = teamFromResponse(t, request(t, h, "PUT", base+"/members/"+admin.ID, token, teamMemberBody(team.Revision, "", "owner", TeamGrants{Admin: true}), 200))
	request(t, h, "DELETE", base+"/members/"+owner.ID, token, map[string]int{"expectedRevision": team.Revision}, 200)
	request(t, h, "GET", base, token, nil, 404)
	request(t, h, "GET", base, adminToken, nil, 200)
}

func TestTeamFailedPersistenceLeavesMembershipAndLiveAccessUnchanged(t *testing.T) {
	g, h, token, _ := fixture(t, nil)
	member := addUser(t, h, token, "retained-member", "member")
	memberToken := loginAs(t, h, member.Username)
	team := createTestTeam(t, h, token, "Retained access")
	base := "/v1/teams/" + team.ID
	team = teamFromResponse(t, request(t, h, "POST", base+"/members", token, teamMemberBody(team.Revision, member.Username, "editor", TeamGrants{Execute: true}), 200))
	access, err := g.resolveTeamAccess(team.ID, member.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	server, client := net.Pipe()
	defer client.Close()
	conn := &observedIdentitySocket{Conn: server}
	defer conn.Close()
	g.sockets[member.ID] = map[net.Conn]string{conn: hashToken(memberToken)}
	g.teamSockets = map[net.Conn]teamSocketScope{conn: {TeamID: team.ID, UserID: member.ID, MemberRevision: access.Member.Revision}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pending := &teamRequest{teamSocketScope: teamSocketScope{TeamID: team.ID, UserID: member.ID, MemberRevision: access.Member.Revision}, SessionHash: hashToken(memberToken)}
	g.teamRequests = map[*teamRequest]context.CancelFunc{pending: cancel}
	g.tickets["retained-team-ticket"] = socketTicket{TeamID: team.ID, UserID: member.ID, SessionHash: hashToken(memberToken), MemberRevision: access.Member.Revision, ExpiresAt: time.Now().Add(time.Minute)}
	before := cloneIdentityState(g.state)
	restore := blockIdentityCommit(t, g)
	request(t, h, "POST", "/v1/teams", token, map[string]string{"name": "unsaved team"}, 500)
	assertIdentityUnchanged(t, g, before)
	request(t, h, "PATCH", base, token, map[string]any{"expectedRevision": team.Revision, "name": "unsaved name"}, 500)
	assertIdentityUnchanged(t, g, before)
	request(t, h, "PUT", base+"/members/"+member.ID, token, teamMemberBody(team.Revision, "", "viewer", TeamGrants{}), 500)
	assertIdentityUnchanged(t, g, before)
	request(t, h, "DELETE", base+"/members/"+member.ID, token, map[string]int{"expectedRevision": team.Revision}, 500)
	assertIdentityUnchanged(t, g, before)
	if conn.closed.Load() || ctx.Err() != nil || len(g.tickets) != 1 {
		t.Fatal("failed persistence revoked a live team connection, request or reconnect ticket")
	}
	restore()
	request(t, h, "PUT", base+"/members/"+member.ID, token, teamMemberBody(team.Revision, "", "viewer", TeamGrants{}), 200)
	if !conn.closed.Load() || ctx.Err() == nil || len(g.tickets) != 0 {
		t.Fatal("committed reduction did not revoke previous live team access")
	}
	if g.teamAccessCurrentLocked(team.ID, member.ID, "", access.Member.Revision, 0) {
		t.Fatal("the old member revision remained usable after a durable change")
	}
}

func TestTeamProjectGrantsIntersectMembershipAndAreDeeplyCloned(t *testing.T) {
	g, h, token, owner := fixture(t, nil)
	member := addUser(t, h, token, "project-member", "member")
	team := createTestTeam(t, h, token, "Project boundaries")
	team = teamFromResponse(t, request(t, h, "POST", "/v1/teams/"+team.ID+"/members", token, teamMemberBody(team.Revision, member.Username, "editor", TeamGrants{Execute: true}), 200))
	row := g.state.Teams[team.ID]
	row.Projects["p-allowed"] = map[string]TeamProjectGrant{member.ID: {UserID: member.ID, Role: "editor", Grants: TeamGrants{Execute: true, Secrets: true, Deploy: true, Admin: true}, Revision: team.Revision}}
	g.state.Teams[team.ID] = row
	if err := g.persistLocked(); err != nil {
		t.Fatal(err)
	}
	access, err := g.resolveTeamAccess(team.ID, member.ID, "p-allowed")
	if err != nil || !access.Permissions.Read || !access.Permissions.Edit || !access.Permissions.Execute || access.Permissions.Secrets || access.Permissions.Deploy || access.Permissions.Admin {
		t.Fatalf("project grant escalated membership or lost granted execution: %+v %v", access, err)
	}
	if _, err := g.resolveTeamAccess(team.ID, member.ID, "p-other"); err == nil {
		t.Fatal("project grant authorized an unrelated project")
	}
	before := cloneIdentityState(g.state)
	copy := cloneTeamRecords(g.state.Teams)
	changed := copy[team.ID]
	delete(changed.Members, owner.ID)
	delete(changed.Projects["p-allowed"], member.ID)
	copy[team.ID] = changed
	if !reflect.DeepEqual(g.state, before) {
		t.Fatal("team mutation clone shares nested membership/project maps")
	}
	row = g.state.Teams[team.ID]
	grant := row.Projects["p-allowed"][member.ID]
	grant.Role, grant.Revision = "viewer", grant.Revision+1
	row.Team.Revision++
	row.Projects["p-allowed"][member.ID] = grant
	g.state.Teams[team.ID] = row
	if g.teamAccessCurrentLocked(team.ID, member.ID, "p-allowed", access.Member.Revision, access.ProjectRevision) {
		t.Fatal("old project grant revision remained usable")
	}
	current, err := g.resolveTeamAccess(team.ID, member.ID, "p-allowed")
	if err != nil || current.Permissions.Edit {
		t.Fatal("project viewer retained context editing")
	}
}

func TestMalformedPrivateTeamStateFailsClosedOnReopen(t *testing.T) {
	g, h, token, owner := fixture(t, nil)
	team := createTestTeam(t, h, token, "Private state validation")
	valid := cloneIdentityState(g.state)
	for _, corruption := range []string{"unknown-user", "no-owner", "owner-without-admin", "member-future-revision", "project-nonmember", "project-owner-role", "project-future-revision", "nil-projects"} {
		t.Run(corruption, func(t *testing.T) {
			g.state = cloneIdentityState(valid)
			row := g.state.Teams[team.ID]
			member := row.Members[owner.ID]
			switch corruption {
			case "unknown-user":
				delete(g.state.Users, owner.ID)
			case "no-owner":
				member.Role = "editor"
				row.Members[owner.ID] = member
			case "owner-without-admin":
				member.Grants.Admin = false
				row.Members[owner.ID] = member
			case "member-future-revision":
				member.Revision++
				row.Members[owner.ID] = member
			case "project-nonmember":
				row.Projects["p-one"] = map[string]TeamProjectGrant{"other": {UserID: "other", Role: "viewer", Revision: 1}}
			case "project-owner-role":
				row.Projects["p-one"] = map[string]TeamProjectGrant{owner.ID: {UserID: owner.ID, Role: "owner", Revision: 1}}
			case "project-future-revision":
				row.Projects["p-one"] = map[string]TeamProjectGrant{owner.ID: {UserID: owner.ID, Role: "viewer", Revision: 2}}
			case "nil-projects":
				row.Projects = nil
			}
			g.state.Teams[team.ID] = row
			if err := g.persistLocked(); err != nil {
				t.Fatal(err)
			}
			reopened, err := New(g.opts)
			if err == nil {
				reopened.Close(context.Background())
				t.Fatal("malformed private team authority was accepted")
			}
		})
	}
	g.state = valid
	if err := g.persistLocked(); err != nil {
		t.Fatal(err)
	}
}

func TestDisablingAccountPreservesAnActiveTeamOwner(t *testing.T) {
	g, h, ownerToken, owner := fixture(t, nil)
	member := addUser(t, h, ownerToken, "team-creator", "member")
	memberToken := loginAs(t, h, member.Username)
	team := createTestTeam(t, h, memberToken, "Member owned team")
	before := cloneIdentityState(g.state)
	request(t, h, "PATCH", "/v1/admin/users/"+member.ID, ownerToken, map[string]bool{"disabled": true}, 409)
	assertIdentityUnchanged(t, g, before)
	request(t, h, "GET", "/v1/teams/"+team.ID, memberToken, nil, 200)
	request(t, h, "POST", "/v1/teams/"+team.ID+"/members", memberToken, teamMemberBody(team.Revision, owner.Username, "owner", TeamGrants{Admin: true}), 200)
	request(t, h, "PATCH", "/v1/admin/users/"+member.ID, ownerToken, map[string]bool{"disabled": true}, 200)
	request(t, h, "GET", "/v1/teams/"+team.ID, memberToken, nil, 401)
	request(t, h, "GET", "/v1/teams/"+team.ID, ownerToken, nil, 200)
}
