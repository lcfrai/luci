package management

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTeamAuditShowsActualActorsAndNeverOtherTeamOrGlobalEvents(t *testing.T) {
	g, h, ownerToken, owner := fixture(t, nil)
	admin := addUser(t, h, ownerToken, "team-admin", "member")
	adminToken := loginAs(t, h, admin.Username)
	viewer := addUser(t, h, ownerToken, "team-reader", "member")
	viewerToken := loginAs(t, h, viewer.Username)
	team := createTestTeam(t, h, ownerToken, "Team one")
	other := createTestTeam(t, h, ownerToken, "Team two")
	base := "/v1/teams/" + team.ID
	team = teamFromResponse(t, request(t, h, "POST", base+"/members", ownerToken, teamMemberBody(team.Revision, admin.Username, "editor", TeamGrants{Admin: true}), 200))
	team = teamFromResponse(t, request(t, h, "POST", base+"/members", ownerToken, teamMemberBody(team.Revision, viewer.Username, "viewer", TeamGrants{}), 200))
	_, _, code := inviteToTeam(t, h, adminToken, team, "viewer", TeamGrants{})
	request(t, h, "POST", "/v1/auth/invitations/accept", "", acceptBody(code, "new-reader"), 201)
	before, _ := os.ReadFile(filepath.Join(g.dir, "identity.json"))
	w := request(t, h, "GET", base+"/audit", adminToken, nil, 200)
	var result struct {
		TeamID    string           `json:"teamId"`
		Events    []teamAuditEvent `json:"events"`
		Limit     int              `json:"limit"`
		Retention int              `json:"installationRetentionLimit"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.TeamID != team.ID || result.Limit != 200 || result.Retention != 1000 {
		t.Fatal("incorrect history scope or limits")
	}
	actors := map[string]bool{}
	joined := false
	for _, event := range result.Events {
		if event.Target != team.ID || !strings.HasPrefix(event.Action, "team.") || event.Actor.ID != event.ActorID || event.Actor.DisplayName == "" {
			t.Fatalf("invalid actor or event projection: %+v", event)
		}
		actors[event.Actor.ID] = true
		joined = joined || event.Action == "team.invitation.accept"
	}
	if !actors[admin.ID] || !actors[owner.ID] || !joined {
		t.Fatal("team history lost real issuers, owners or new-account acceptance")
	}
	if strings.Contains(w.Body.String(), other.ID) || strings.Contains(w.Body.String(), code) || strings.Contains(w.Body.String(), "passwordHash") || strings.Contains(w.Body.String(), "auth.login") {
		t.Fatal("team audit exposed unrelated or private activity")
	}
	request(t, h, "GET", base+"/audit", viewerToken, nil, 403)
	request(t, h, "GET", "/v1/teams/"+other.ID+"/audit", adminToken, nil, 403)
	after, _ := os.ReadFile(filepath.Join(g.dir, "identity.json"))
	if !bytes.Equal(before, after) {
		t.Fatal("reading audit mutated private state")
	}
	reopened, err := New(g.opts)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close(context.Background())
	request(t, reopened.Handler(), "GET", base+"/audit", adminToken, nil, 200)
}

func TestTeamAuditBoundsNewestOwnedEventsAndDeniesRemovedAdmin(t *testing.T) {
	g, h, token, _ := fixture(t, nil)
	admin := addUser(t, h, token, "delegated", "member")
	adminToken := loginAs(t, h, admin.Username)
	team := createTestTeam(t, h, token, "Team")
	base := "/v1/teams/" + team.ID
	team = teamFromResponse(t, request(t, h, "POST", base+"/members", token, teamMemberBody(team.Revision, admin.Username, "editor", TeamGrants{Admin: true}), 200))
	g.mu.Lock()
	for i := 0; i < 300; i++ {
		g.auditLocked(admin.ID, "team.rename", team.ID, "success")
		g.auditLocked(admin.ID, "auth.login", "unrelated", "success")
	}
	newest := g.state.Events[len(g.state.Events)-2].ID
	g.mu.Unlock()
	var result struct {
		Events []teamAuditEvent `json:"events"`
	}
	w := request(t, h, "GET", base+"/audit", adminToken, nil, 200)
	json.Unmarshal(w.Body.Bytes(), &result)
	if len(result.Events) != teamAuditLimit || result.Events[0].ID != newest {
		t.Fatal("team history was not bounded after ownership filtering")
	}
	request(t, h, "DELETE", base+"/members/"+admin.ID, token, map[string]int{"expectedRevision": team.Revision}, 200)
	request(t, h, "GET", base+"/audit", adminToken, nil, 403)
}
