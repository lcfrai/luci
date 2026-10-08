package management

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The worker projects task clones under their original project. The gateway
// must use that catalogue ownership, not derive an ACL from the clone's ID or
// accept a projectId supplied with a session request.
func TestTeamSwarmTaskCheckoutRetainsOriginalProjectGrant(t *testing.T) {
	f := newTeamProxyFixture(t)
	foreignRow := false
	f.g.opts.TeamHandler = func(_ context.Context, teamID string) (http.Handler, error) {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if teamID == f.team.ID && r.URL.Path == "/v1/projects/p-alpha/worktrees" {
				owner := "p-alpha"
				if foreignRow {
					owner = "p-secret"
				}
				jsonReply(w, 200, map[string]any{"worktrees": []map[string]string{{"id": "w-swarmtask", "projectId": owner}}})
				return
			}
			if teamID == f.team.ID && r.URL.Path == "/v1/projects/p-secret/worktrees" {
				t.Error("member enumerated a task checkout in an ungranted project")
			}
			if r.URL.Path == "/v1/sessions" && r.Method == "POST" {
				f.writes.Add(1)
				var body map[string]string
				_ = json.NewDecoder(r.Body).Decode(&body)
				access, ok := TeamAccessFromContext(r.Context())
				if !ok || access.ProjectID != "p-alpha" || body["worktreeId"] != "w-swarmtask" {
					t.Fatal("session dispatch did not retain the original project authorization")
				}
				jsonReply(w, 201, map[string]string{"id": "s-task", "projectId": "p-alpha", "worktreeId": "w-swarmtask", "harness": "shell", "attachPath": "/v1/sessions/s-task/attach"})
				return
			}
			f.worker(t, teamID, w, r)
		}), nil
	}
	body := map[string]string{"worktreeId": "w-swarmtask", "harness": "shell"}
	created := request(t, f.h, "POST", f.path("/v1/sessions"), f.memberToken, body, 201)
	if f.writes.Load() != 1 || !strings.Contains(created.Body.String(), f.path("/v1/sessions/s-task/attach")) {
		t.Fatal("owned task did not receive exactly one scoped session")
	}
	foreignRow = true
	request(t, f.h, "POST", f.path("/v1/sessions"), f.memberToken, body, 403)
	foreignRow = false
	// Removing the original project grant immediately removes task authority.
	f.g.mu.Lock()
	team := f.g.state.Teams[f.team.ID]
	delete(team.Projects["p-alpha"], f.member.ID)
	f.g.state.Teams[f.team.ID] = team
	f.g.mu.Unlock()
	body["projectId"] = "p-alpha"
	request(t, f.h, "POST", f.path("/v1/sessions"), f.memberToken, body, 403)
	if f.writes.Load() != 1 {
		t.Fatal("foreign or revoked task ownership reached session allocation")
	}
}

func TestTeamSwarmRoutesRequireExactCapabilities(t *testing.T) {
	base := "/v1/product/projects/p-one/runs/run-one/swarm"
	for _, test := range []struct {
		method, suffix string
		edit, execute  bool
	}{
		{"POST", "", true, true}, {"POST", "/tasks/api/reserve", true, true}, {"POST", "/tasks/api/integrate", true, true},
		{"POST", "/pause", true, false}, {"POST", "/resume", true, false}, {"POST", "/cancel", true, false}, {"POST", "/reconcile", true, false}, {"POST", "/tasks/api/abandon", true, false},
		{"GET", "/tasks/api/review", false, false}, {"GET", "/operations/op-one/workspace", false, false}, {"GET", "/operations/op-one/workspace/file?path=result.txt", false, false},
	} {
		for _, permission := range []TeamPermissions{{Read: true}, {Read: true, Edit: true}, {Read: true, Edit: true, Execute: true}, {}} {
			w := httptest.NewRecorder()
			allowed := authorizeTeamRoute(w, httptest.NewRequest(test.method, base+test.suffix, nil), permission)
			want := permission.Read && (!test.edit || permission.Edit) && (!test.execute || permission.Execute)
			if allowed != want {
				t.Fatalf("%s %s with %+v got %v wanted %v: %s", test.method, test.suffix, permission, allowed, want, w.Body.String())
			}
		}
	}
}

func TestTeamSwarmUnknownRouteFailsClosed(t *testing.T) {
	permissions := TeamPermissions{Read: true, Edit: true, Execute: true, Secrets: true, Deploy: true, Admin: true}
	for _, endpoint := range []string{"POST /v1/product/projects/p-one/runs/run-one/swarm/tasks/api/replay", "POST /v1/product/projects/p-one/runs/run-one/swarm/operations/op-one/workspace/file"} {
		method, path, _ := strings.Cut(endpoint, " ")
		w := httptest.NewRecorder()
		if authorizeTeamRoute(w, httptest.NewRequest(method, path, nil), permissions) || w.Code != http.StatusForbidden {
			t.Fatal("unknown swarm mutation permitted", endpoint, w.Code)
		}
	}
}

func TestTeamSwarmWorkspaceReadRetainsProjectGrant(t *testing.T) {
	f := newTeamProxyFixture(t)
	f.g.mu.Lock()
	team := f.g.state.Teams[f.team.ID]
	member := team.Members[f.member.ID]
	member.Role = "viewer"
	member.Grants = TeamGrants{}
	team.Members[f.member.ID] = member
	grant := team.Projects["p-alpha"][f.member.ID]
	grant.Role = "viewer"
	grant.Grants = TeamGrants{}
	team.Projects["p-alpha"][f.member.ID] = grant
	f.g.state.Teams[f.team.ID] = team
	f.g.mu.Unlock()
	reads := 0
	f.g.opts.TeamHandler = func(_ context.Context, teamID string) (http.Handler, error) {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "/swarm/operations/") {
				access, ok := TeamAccessFromContext(r.Context())
				if !ok || access.ProjectID != "p-alpha" || access.Permissions.Edit || access.Permissions.Execute || access.Permissions.Secrets {
					t.Fatal("workspace read received ungranted capability")
				}
				reads++
				jsonReply(w, 200, map[string]any{"projectId": "p-alpha", "files": []string{"result.txt"}})
				return
			}
			f.worker(t, teamID, w, r)
		}), nil
	}
	for _, suffix := range []string{"/workspace", "/workspace/file?path=result.txt"} {
		request(t, f.h, "GET", f.path("/v1/product/projects/p-alpha/runs/run-one/swarm/operations/op-one"+suffix), f.memberToken, nil, 200)
		request(t, f.h, "GET", f.path("/v1/product/projects/p-secret/runs/run-one/swarm/operations/op-one"+suffix), f.memberToken, nil, 403)
	}
	if reads != 2 {
		t.Fatal("workspace output reached ungranted project")
	}
}
