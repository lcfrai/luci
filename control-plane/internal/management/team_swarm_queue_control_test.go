package management

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

// CLOUD-001 removes source-edit exclusion from queue observation and controls.
// The production gateway must still resolve current project grants on every
// request; an earlier successful control never becomes enduring authority.
func TestTeamSwarmQueueControlsReauthorizeAfterProjectEditRevocation(t *testing.T) {
	f := newTeamProxyFixture(t)
	base := "/v1/product/projects/p-alpha/swarm-queue"
	reads, writes := 0, 0
	f.g.opts.TeamHandler = func(_ context.Context, teamID string) (http.Handler, error) {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != base && !strings.HasPrefix(r.URL.Path, base+"/") {
				f.worker(t, teamID, w, r)
				return
			}
			access, ok := TeamAccessFromContext(r.Context())
			if !ok || teamID != f.team.ID || access.Team.ID != f.team.ID || access.ProjectID != "p-alpha" || !access.Permissions.Read {
				t.Error("queue request lost its exact current project scope")
				w.WriteHeader(500)
				return
			}
			if r.Method == http.MethodGet || r.Method == http.MethodHead {
				reads++
			} else {
				if !access.Permissions.Edit || (strings.HasSuffix(r.URL.Path, "/resume") && !access.Permissions.Execute) {
					t.Error("queue control reached worker without required current grants")
					w.WriteHeader(500)
					return
				}
				writes++
			}
			jsonReply(w, 200, map[string]any{"projectId": "p-alpha", "items": []any{}})
		}), nil
	}
	setProjectGrant := func(role string, execute bool) {
		t.Helper()
		f.g.mu.Lock()
		revision := f.g.state.Teams[f.team.ID].Team.Revision
		f.g.mu.Unlock()
		request(t, f.h, http.MethodPut, "/v1/teams/"+f.team.ID+"/projects/p-alpha/members/"+f.member.ID, f.ownerToken, teamMemberBody(revision, "", role, TeamGrants{Execute: execute}), 200)
	}
	controls := []string{"pause", "resume", "cancel", "priority"}
	for _, action := range controls {
		request(t, f.h, http.MethodPost, f.path(base+"/queue-owned/"+action), f.memberToken, map[string]any{"expectedRevision": 1, "priority": "high"}, 200)
	}
	if writes != len(controls) {
		t.Fatal("authorized controls did not reach worker exactly once", writes)
	}
	// Revoke only the project Edit role through the real management API. Team
	// membership still grants Edit/Execute and the caller retains the same token.
	setProjectGrant("viewer", true)
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		request(t, f.h, method, f.path(base), f.memberToken, nil, 200)
	}
	for _, action := range controls {
		request(t, f.h, http.MethodPost, f.path(base+"/queue-owned/"+action), f.memberToken, map[string]any{"expectedRevision": 1, "priority": "high"}, 403)
	}
	if writes != len(controls) || reads != 2 {
		t.Fatal("revoked project edit still reached queue control", writes, reads)
	}
	// Restore Edit without Execute: bookkeeping works, resuming dispatch still
	// requires its separate execution capability.
	setProjectGrant("editor", false)
	for _, action := range []string{"pause", "cancel", "priority"} {
		request(t, f.h, http.MethodPost, f.path(base+"/queue-owned/"+action), f.memberToken, map[string]any{"expectedRevision": 1, "priority": "normal"}, 200)
	}
	request(t, f.h, http.MethodPost, f.path(base+"/queue-owned/resume"), f.memberToken, map[string]any{"expectedRevision": 1}, 403)
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		request(t, f.h, method, f.path("/v1/product/projects/p-secret/swarm-queue"), f.memberToken, nil, 403)
	}
	for _, action := range controls {
		request(t, f.h, http.MethodPost, f.path("/v1/product/projects/p-secret/swarm-queue/queue-owned/"+action), f.memberToken, map[string]any{"expectedRevision": 1}, 403)
	}
	if writes != 7 || reads != 2 {
		t.Fatal("foreign or missing capability reached queue worker", writes, reads)
	}
	// Full project revocation also removes observation; neither a queue ID nor a
	// previous successful response creates a substitute project grant.
	f.g.mu.Lock()
	revision := f.g.state.Teams[f.team.ID].Team.Revision
	f.g.mu.Unlock()
	request(t, f.h, http.MethodDelete, "/v1/teams/"+f.team.ID+"/projects/p-alpha/members/"+f.member.ID, f.ownerToken, map[string]int{"expectedRevision": revision}, 200)
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		request(t, f.h, method, f.path(base), f.memberToken, nil, 403)
	}
	if reads != 2 || writes != 7 {
		t.Fatal("revoked project retained queue access", reads, writes)
	}
}
