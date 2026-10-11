package management

import (
	"context"
	"encoding/json"
	"net"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func serviceBody(revision int, mode, status string) map[string]any {
	return map[string]any{"expectedRevision": revision, "mode": mode, "status": status, "reason": "Explicit operator decision"}
}

func TestTeamServiceDefaultHasNoSubscriptionAndOperatorChangesPersist(t *testing.T) {
	f := newTeamProxyFixture(t)
	path := "/v1/admin/team-services/" + f.team.ID
	view := request(t, f.h, "GET", "/v1/teams/"+f.team.ID+"/service", f.memberToken, nil, 200)
	var initial TeamService
	if json.Unmarshal(view.Body.Bytes(), &initial) != nil || initial.Mode != "self-hosted" || initial.Status != "active" || initial.Revision != 0 {
		t.Fatalf("self-hosted team acquired subscription requirements: %s", view.Body.String())
	}
	request(t, f.h, "POST", f.path("/v1/product/projects/p-alpha/checks"), f.memberToken, map[string]any{}, 200)
	request(t, f.h, "PUT", path, f.memberToken, serviceBody(0, "managed", "active"), 403)
	admin := addUser(t, f.h, f.ownerToken, "ordinary-admin", "admin")
	request(t, f.h, "PUT", path, loginAs(t, f.h, admin.Username), serviceBody(0, "managed", "active"), 403)
	request(t, f.h, "GET", "/v1/admin/team-services", f.memberToken, nil, 403)
	request(t, f.h, "PUT", path, f.ownerToken, serviceBody(0, "managed", "suspended"), 200)
	request(t, f.h, "PUT", path, f.ownerToken, serviceBody(0, "managed", "active"), 409)
	request(t, f.h, "PUT", path, f.ownerToken, serviceBody(1, "self-hosted", "suspended"), 400)
	request(t, f.h, "PUT", path, f.ownerToken, serviceBody(1, "managed", "paid"), 400)
	listing := request(t, f.h, "GET", "/v1/admin/team-services", f.ownerToken, nil, 200).Body.String()
	if !strings.Contains(listing, `"billingConnected":false`) || !strings.Contains(listing, `"executionIsolationVerified":false`) || strings.Contains(listing, "passwordHash") {
		t.Fatal("operator metadata invented commercial proof or exposed identity secrets")
	}
	reopened, err := New(f.g.opts)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close(context.Background())
	if reopened.state.Teams[f.team.ID].Service.Status != "suspended" || reopened.state.Teams[f.team.ID].Service.Revision != 1 {
		t.Fatal("suspension did not survive a service restart")
	}
	request(t, reopened.Handler(), "POST", f.path("/v1/product/projects/p-alpha/checks"), f.memberToken, map[string]any{}, 403)
	request(t, f.h, "PUT", path, f.ownerToken, serviceBody(1, "self-hosted", "active"), 200)
	request(t, f.h, "POST", f.path("/v1/product/projects/p-alpha/checks"), f.memberToken, map[string]any{}, 200)
	if f.g.state.Events[len(f.g.state.Events)-1].Action != "team.service.active" {
		t.Fatal("operator decision was not audited")
	}
}

func TestTeamServiceSuspensionBlocksWorkBeforeWorkerStartupAndPreservesRecovery(t *testing.T) {
	f := newTeamProxyFixture(t)
	request(t, f.h, "PUT", "/v1/admin/team-services/"+f.team.ID, f.ownerToken, serviceBody(0, "managed", "suspended"), 200)
	for _, tc := range []struct{ method, path, body string }{
		{"POST", "/v1/sessions", `{"worktreeId":"wt-alpha","harness":"shell"}`},
		{"GET", "/v1/sessions/s-visible/attach", ``},
		{"POST", "/v1/sessions/s-visible/restart", `{}`},
		{"POST", "/v1/product/projects/p-alpha/checks", `{}`},
		{"POST", "/v1/product/projects/p-alpha/assist", `{}`},
		{"POST", "/v1/product/projects/p-alpha/releases", `{}`},
		{"POST", "/v1/product/projects/p-alpha/releases/release-one/rollback", `{}`},
		{"POST", "/v1/platform/projects/p-alpha/database", `{"action":"start"}`},
		{"POST", "/v1/platform/projects/p-alpha/database", `{"action":"stop","Action":"start"}`},
		{"POST", "/v1/platform/projects/p-alpha/data", `{"action":"restore"}`},
		{"POST", "/v1/setup/agent-connections", `{"action":"save"}`},
		{"POST", "/v1/setup/agents/openai", `{"apiKey":"retained"}`},
		{"POST", "/v1/product/projects/p-alpha/configuration", `{"updates":[{"name":"API_KEY","value":"retained"}]}`},
		{"POST", "/v1/product/projects/p-alpha/configuration", `{"updates":[{"name":"API_KEY","remove":true,"Remove":false,"value":"changed"}]}`},
	} {
		r := httptest.NewRequest(tc.method, f.path(tc.path), strings.NewReader(tc.body))
		r.Header.Set("Authorization", "Bearer "+f.ownerToken)
		w := httptest.NewRecorder()
		f.h.ServeHTTP(w, r)
		if w.Code != 403 || !strings.Contains(w.Body.String(), "team_service_suspended") {
			t.Fatalf("%s %s bypassed suspension: %d %s", tc.method, tc.path, w.Code, w.Body.String())
		}
	}
	if f.starts.Load() != 0 || f.writes.Load() != 0 || f.attaches.Load() != 0 {
		t.Fatal("suspended work reached the private execution worker")
	}
	request(t, f.h, "POST", "/v1/auth/socket-ticket", f.ownerToken, map[string]string{"path": f.path("/v1/sessions/s-visible/attach")}, 403)
	if len(f.g.tickets) != 0 || f.starts.Load() != 0 {
		t.Fatal("suspended terminal minted a ticket or started its worker")
	}
	for _, tc := range []struct {
		method, path string
		body         any
	}{
		{"GET", "/v1/product/projects/p-alpha/brief", nil},
		{"POST", "/v1/product/projects/p-alpha/brief", map[string]string{"summary": "retained context"}},
		{"POST", "/v1/product/projects/p-alpha/backups", map[string]any{}},
		{"GET", "/v1/platform/projects/p-alpha/backups/backup-one", nil},
		{"DELETE", "/v1/sessions/s-visible", nil},
		{"POST", "/v1/product/projects/p-alpha/checks/check-one/cancel", map[string]any{}},
		{"POST", "/v1/product/projects/p-alpha/releases/release-one/stop", map[string]any{}},
		{"POST", "/v1/platform/projects/p-alpha/database", map[string]string{"action": "stop"}},
		{"POST", "/v1/platform/projects/p-alpha/data", map[string]string{"action": "backup"}},
		{"POST", "/v1/setup/agent-connections", map[string]string{"action": "delete", "id": "agent-one"}},
		{"DELETE", "/v1/setup/agents/codex/account", nil},
		{"POST", "/v1/setup/agents/openai", map[string]string{"apiKey": ""}},
		{"POST", "/v1/product/projects/p-alpha/configuration", map[string]any{"expectedRevision": 1, "updates": []map[string]any{{"name": "API_KEY", "remove": true}}}},
	} {
		request(t, f.h, tc.method, f.path(tc.path), f.ownerToken, tc.body, 200)
	}
	// Cleanup exceptions cannot create project or team capability authority.
	request(t, f.h, "POST", f.path("/v1/platform/projects/p-secret/database"), f.memberToken, map[string]string{"action": "stop"}, 403)
	request(t, f.h, "POST", teamWorkspacePrefix(f.other.ID)+"/v1/product/projects/p-other/checks", f.memberToken, map[string]any{}, 200)
}

func TestTeamServiceSuspensionCommitsBeforeRevokingAndRechecksAdmission(t *testing.T) {
	f := newTeamProxyFixture(t)
	access, err := f.g.resolveTeamAccess(f.team.ID, f.member.ID, "p-alpha")
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", "/v1/product/projects/p-alpha/checks", nil)
	r = r.WithContext(withActor(r.Context(), f.member, f.g.state.Policy))
	admitted, finish, err := f.g.RegisterTeamRequest(httptest.NewRecorder(), r, f.team.ID, "p-alpha", access.Member.Revision, access.ProjectRevision)
	if err != nil {
		t.Fatal(err)
	}
	defer finish()
	server, client := net.Pipe()
	defer client.Close()
	conn := &observedIdentitySocket{Conn: server}
	defer conn.Close()
	f.g.sockets[f.member.ID] = map[net.Conn]string{conn: hashToken(f.memberToken)}
	f.g.teamSockets = map[net.Conn]teamSocketScope{conn: {TeamID: f.team.ID, UserID: f.member.ID}}
	f.g.tickets["pending"] = socketTicket{TeamID: f.team.ID, UserID: f.member.ID, ExpiresAt: time.Now().Add(time.Minute)}
	before := cloneIdentityState(f.g.state)
	restore := blockIdentityCommit(t, f.g)
	request(t, f.h, "PUT", "/v1/admin/team-services/"+f.team.ID, f.ownerToken, serviceBody(0, "managed", "suspended"), 500)
	assertIdentityUnchanged(t, f.g, before)
	if admitted.Context().Err() != nil || conn.closed.Load() || len(f.g.tickets) != 1 {
		t.Fatal("unsaved suspension revoked live access")
	}
	restore()
	request(t, f.h, "PUT", "/v1/admin/team-services/"+f.team.ID, f.ownerToken, serviceBody(0, "managed", "suspended"), 200)
	if admitted.Context().Err() == nil || !conn.closed.Load() || len(f.g.tickets) != 0 {
		t.Fatal("durable suspension did not revoke active requests, terminals and reconnect tickets")
	}
	if _, done, err := f.g.RegisterTeamRequest(httptest.NewRecorder(), r, f.team.ID, "p-alpha", access.Member.Revision, access.ProjectRevision); err == nil {
		done()
		t.Fatal("request admitted after suspension with old membership")
	}
	read := httptest.NewRequest("GET", "/v1/product/projects/p-alpha/brief", nil)
	read = read.WithContext(withActor(read.Context(), f.member, f.g.state.Policy))
	_, done, err := f.g.RegisterTeamRequest(httptest.NewRecorder(), read, f.team.ID, "p-alpha", access.Member.Revision, access.ProjectRevision)
	if err != nil {
		t.Fatal("suspension prevented permitted context read")
	}
	done()
	if len(f.g.state.Teams[f.team.ID].Projects) != len(before.Teams[f.team.ID].Projects) || len(f.g.state.Teams[f.team.ID].Members) != len(before.Teams[f.team.ID].Members) {
		t.Fatal("suspension deleted team ownership or project grants")
	}
}

func TestTeamServiceCleanupPreservesExactBodyAndRejectsUnboundedActions(t *testing.T) {
	for _, raw := range []string{`{"action":"stop","Action":"start"}`, `{"Action":"stop"}`, strings.Repeat(" ", 8193) + `{"action":"stop"}`} {
		r := httptest.NewRequest("POST", "/v1/platform/projects/p-one/database", strings.NewReader(raw))
		prepareTeamServiceCleanup(httptest.NewRecorder(), r)
		if teamServiceCleanup(r) {
			t.Fatal("ambiguous or oversized cleanup admitted")
		}
	}
	raw := ` {"action":"stop","expectedRevision":2} `
	r := httptest.NewRequest("POST", "/v1/platform/projects/p-one/database", strings.NewReader(raw))
	prepareTeamServiceCleanup(httptest.NewRecorder(), r)
	var got map[string]any
	if !teamServiceCleanup(r) || json.NewDecoder(r.Body).Decode(&got) != nil || got["action"] != "stop" || got["expectedRevision"] != float64(2) {
		t.Fatal("service check consumed or changed worker action body")
	}
}

func TestPreviewCancellationSuspendedServiceRechecksCurrentGrants(t *testing.T) {
	for _, suspended := range []bool{false, true} {
		name := "active"
		if suspended {
			name = "suspended"
		}
		t.Run(name, func(t *testing.T) {
			f := newTeamProxyFixture(t)
			memberPath := "/v1/teams/" + f.team.ID + "/members/" + f.member.ID
			projectPath := "/v1/teams/" + f.team.ID + "/projects/p-alpha/members/" + f.member.ID
			setGrant := func(path, role string, deploy bool) {
				t.Helper()
				f.g.mu.Lock()
				revision := f.g.state.Teams[f.team.ID].Team.Revision
				f.g.mu.Unlock()
				request(t, f.h, "PUT", path, f.ownerToken, teamMemberBody(revision, "", role, TeamGrants{Deploy: deploy}), 200)
			}
			// Both current grants permit Deploy, with no Execute or Secrets grant.
			setGrant(memberPath, "editor", true)
			setGrant(projectPath, "editor", true)
			if suspended {
				request(t, f.h, "PUT", "/v1/admin/team-services/"+f.team.ID, f.ownerToken, serviceBody(0, "managed", "suspended"), 200)
			}
			path := f.path("/v1/product/projects/p-alpha/releases/release-one/cancel")
			body := map[string]int{"expectedRevision": 7}
			request(t, f.h, "POST", path, f.memberToken, body, 200)
			if f.writes.Load() != 1 {
				t.Fatal("current deployment authority did not reach the exact cleanup worker")
			}
			// An existing login and the same release revision confer no authority
			// after either the team or project Deploy grant is revoked.
			setGrant(memberPath, "editor", false)
			before := f.writes.Load()
			request(t, f.h, "POST", path, f.memberToken, body, 403)
			if f.writes.Load() != before {
				t.Fatal("revoked team deployment grant reached cleanup")
			}
			setGrant(memberPath, "editor", true)
			setGrant(projectPath, "editor", false)
			request(t, f.h, "POST", path, f.memberToken, body, 403)
			if f.writes.Load() != before {
				t.Fatal("revoked project deployment grant reached cleanup")
			}
			setGrant(projectPath, "viewer", true)
			request(t, f.h, "POST", path, f.memberToken, body, 403)
			request(t, f.h, "POST", f.path("/v1/product/projects/p-secret/releases/release-one/cancel"), f.memberToken, body, 403)
			if f.writes.Load() != before {
				t.Fatal("cleanup exemption bypassed edit or project ownership")
			}
			setGrant(projectPath, "editor", true)
			request(t, f.h, "POST", path, f.memberToken, body, 200)
			if f.writes.Load() != before+1 {
				t.Fatal("restored current deployment authority did not reach cleanup")
			}
			if suspended {
				request(t, f.h, "POST", f.path("/v1/product/projects/p-alpha/releases"), f.memberToken, map[string]any{}, 403)
				if f.writes.Load() != before+1 {
					t.Fatal("cleanup exemption permitted a new release under suspension")
				}
			}
		})
	}
}
