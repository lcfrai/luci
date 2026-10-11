package management

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestTeamExternalHandoffNeedsCurrentEditWithoutExecution(t *testing.T) {
	f := newTeamProxyFixture(t)
	base := "/v1/product/projects/p-alpha/external-handoffs"
	reads, writes := 0, 0
	f.g.opts.TeamHandler = func(_ context.Context, teamID string) (http.Handler, error) {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.Contains(r.URL.Path, "/external-handoffs") {
				f.worker(t, teamID, w, r)
				return
			}
			access, ok := TeamAccessFromContext(r.Context())
			if !ok || access.ProjectID != "p-alpha" || teamID != f.team.ID || !access.Permissions.Read {
				t.Error("handoff lost project authority")
				w.WriteHeader(500)
				return
			}
			if r.Method == "GET" || r.Method == "HEAD" {
				reads++
			} else {
				if !access.Permissions.Edit || access.Permissions.Execute {
					t.Error("unexpected handoff permission")
				}
				writes++
			}
			jsonReply(w, 200, map[string]any{})
		}), nil
	}
	grant := func(role string) {
		t.Helper()
		f.g.mu.Lock()
		revision := f.g.state.Teams[f.team.ID].Team.Revision
		f.g.mu.Unlock()
		request(t, f.h, "PUT", "/v1/teams/"+f.team.ID+"/projects/p-alpha/members/"+f.member.ID, f.ownerToken, teamMemberBody(revision, "", role, TeamGrants{}), 200)
	}
	grant("editor")
	for _, suffix := range []string{"", "/handoff-one/returns"} {
		request(t, f.h, "POST", f.path(base+suffix), f.memberToken, map[string]any{}, 200)
	}
	grant("viewer")
	for _, suffix := range []string{"", "/handoff-one", "/handoff-one/bundle", "/handoff-one/review"} {
		for _, method := range []string{"GET", "HEAD"} {
			request(t, f.h, method, f.path(base+suffix), f.memberToken, nil, 200)
		}
	}
	for _, suffix := range []string{"", "/handoff-one/returns"} {
		request(t, f.h, "POST", f.path(base+suffix), f.memberToken, map[string]any{}, 403)
	}
	request(t, f.h, "GET", f.path(strings.Replace(base, "p-alpha", "p-secret", 1)), f.memberToken, nil, 403)
	if reads != 8 || writes != 2 {
		t.Fatal("foreign/revoked request reached worker", reads, writes)
	}
}
