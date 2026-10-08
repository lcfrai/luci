package management

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestTeamHandoffUsesCurrentProjectEditWithoutExecutionGrant(t *testing.T) {
	f := newTeamProxyFixture(t)
	base := "/v1/product/projects/p-alpha/runs/run-one/swarm/operations/op-one/handoff"
	reads, writes := 0, 0
	f.g.opts.TeamHandler = func(_ context.Context, teamID string) (http.Handler, error) {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "/handoff") {
				access, ok := TeamAccessFromContext(r.Context())
				if !ok || access.ProjectID != "p-alpha" || teamID != f.team.ID || !access.Permissions.Read {
					t.Error("handoff lost exact current project scope")
					w.WriteHeader(500)
					return
				}
				if r.Method == "GET" || r.Method == "HEAD" {
					reads++
				} else {
					if !access.Permissions.Edit {
						t.Error("ungranted report mutation reached worker")
						w.WriteHeader(500)
						return
					}
					writes++
				}
				jsonReply(w, 200, map[string]any{"handoff": map[string]any{"status": "not-recorded"}})
				return
			}
			f.worker(t, teamID, w, r)
		}), nil
	}
	setGrant := func(role string) {
		t.Helper()
		f.g.mu.Lock()
		revision := f.g.state.Teams[f.team.ID].Team.Revision
		f.g.mu.Unlock()
		request(t, f.h, http.MethodPut, "/v1/teams/"+f.team.ID+"/projects/p-alpha/members/"+f.member.ID, f.ownerToken, teamMemberBody(revision, "", role, TeamGrants{}), 200)
	}
	setGrant("editor")
	for _, suffix := range []string{"/collect", "/questions/question-one/answer"} {
		request(t, f.h, "POST", f.path(base+suffix), f.memberToken, map[string]any{"expectedRevision": 1}, 200)
	}
	setGrant("viewer")
	for _, method := range []string{"GET", "HEAD"} {
		request(t, f.h, method, f.path(base), f.memberToken, nil, 200)
	}
	for _, suffix := range []string{"/collect", "/questions/question-one/answer"} {
		request(t, f.h, "POST", f.path(base+suffix), f.memberToken, map[string]any{"expectedRevision": 1}, 403)
	}
	request(t, f.h, "GET", f.path(strings.Replace(base, "p-alpha", "p-secret", 1)), f.memberToken, nil, 403)
	f.g.mu.Lock()
	revision := f.g.state.Teams[f.team.ID].Team.Revision
	f.g.mu.Unlock()
	request(t, f.h, "DELETE", "/v1/teams/"+f.team.ID+"/projects/p-alpha/members/"+f.member.ID, f.ownerToken, map[string]int{"expectedRevision": revision}, 200)
	request(t, f.h, "GET", f.path(base), f.memberToken, nil, 403)
	if reads != 2 || writes != 2 {
		t.Fatal("revoked/foreign grant reached retained task output", reads, writes)
	}
}
