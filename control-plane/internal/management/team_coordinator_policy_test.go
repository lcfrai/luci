package management

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestTeamCoordinatorSeparatesReadDraftAndExecutionAuthority(t *testing.T) {
	f := newTeamProxyFixture(t)
	base := "/v1/product/projects/p-alpha/orchestrator"
	reached := 0
	f.g.opts.TeamHandler = func(_ context.Context, teamID string) (http.Handler, error) {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.Contains(r.URL.Path, "/orchestrator/") {
				f.worker(t, teamID, w, r)
				return
			}
			access, ok := TeamAccessFromContext(r.Context())
			if !ok || access.ProjectID != "p-alpha" || teamID != f.team.ID {
				t.Error("lost current project scope")
				w.WriteHeader(500)
				return
			}
			reached++
			jsonReply(w, 200, map[string]any{})
		}), nil
	}
	grant := func(role string, execution bool) {
		t.Helper()
		f.g.mu.Lock()
		revision := f.g.state.Teams[f.team.ID].Team.Revision
		f.g.mu.Unlock()
		request(t, f.h, "PUT", "/v1/teams/"+f.team.ID+"/projects/p-alpha/members/"+f.member.ID, f.ownerToken, teamMemberBody(revision, "", role, TeamGrants{Execute: execution}), 200)
	}
	grant("viewer", false)
	for _, path := range []string{"/policy", "/continuations", "/continuations/continue-one", "/coordination"} {
		request(t, f.h, "GET", f.path(base+path), f.memberToken, nil, 200)
	}
	for _, path := range []string{"/policy", "/turns/turn-one/advance", "/continuations", "/continuations/continue-one/advance"} {
		request(t, f.h, "POST", f.path(base+path), f.memberToken, map[string]any{}, 403)
	}
	grant("editor", false)
	request(t, f.h, "POST", f.path(base+"/policy"), f.memberToken, map[string]any{}, 200)
	request(t, f.h, "POST", f.path(base+"/turns/turn-one/advance"), f.memberToken, map[string]any{}, 200)
	request(t, f.h, "POST", f.path(base+"/continuations"), f.memberToken, map[string]any{}, 403)
	grant("viewer", true)
	request(t, f.h, "POST", f.path(base+"/continuations"), f.memberToken, map[string]any{}, 403)
	grant("editor", true)
	request(t, f.h, "POST", f.path(base+"/continuations"), f.memberToken, map[string]any{}, 200)
	request(t, f.h, "POST", f.path(base+"/continuations/continue-one/advance"), f.memberToken, map[string]any{}, 200)
	request(t, f.h, "GET", f.path(strings.Replace(base, "p-alpha", "p-secret", 1)+"/continuations"), f.memberToken, nil, 403)
	grant("viewer", false)
	request(t, f.h, "POST", f.path(base+"/continuations/continue-one/advance"), f.memberToken, map[string]any{}, 403)
	if reached != 8 {
		t.Fatalf("refused request reached worker: %d", reached)
	}
}
