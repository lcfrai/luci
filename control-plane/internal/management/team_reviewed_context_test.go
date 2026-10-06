package management

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func reviewedInspectionTeamFixture(t *testing.T, serve func(http.ResponseWriter, *http.Request)) *teamProxyFixture {
	t.Helper()
	f := newTeamProxyFixture(t)
	f.g.mu.Lock()
	row := f.g.state.Teams[f.team.ID]
	member := row.Members[f.member.ID]
	member.Role, member.Grants = "viewer", TeamGrants{}
	row.Members[f.member.ID] = member
	grant := row.Projects["p-alpha"][f.member.ID]
	grant.Role, grant.Grants = "viewer", TeamGrants{}
	row.Projects["p-alpha"][f.member.ID] = grant
	f.g.state.Teams[f.team.ID] = row
	err := f.g.persistLocked()
	f.g.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	original := f.g.opts.TeamHandler
	f.g.opts.TeamHandler = func(ctx context.Context, teamID string) (http.Handler, error) {
		next, err := original(ctx, teamID)
		if err != nil {
			return nil, err
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/reviewed-context") {
				actor, _, authenticated := ActorFromContext(r.Context())
				access, scoped := TeamAccessFromContext(r.Context())
				if !authenticated || !scoped || actor.ID != f.member.ID || access.ProjectID != "p-alpha" || !access.Permissions.Read || access.Permissions.Edit || access.Permissions.Execute || access.Permissions.Secrets {
					t.Error("inspection did not receive the actual read-only project authority")
				}
				serve(w, r)
				return
			}
			next.ServeHTTP(w, r)
		}), nil
	}
	return f
}

func TestTeamViewerReadsCapturedContextWithoutTerminalPrivilegesOrHiddenProjectAccess(t *testing.T) {
	var reads atomic.Int32
	// Large enough to exercise bounded buffering while staying below the
	// product's 7MiB inspection envelope and the existing 8MiB proxy limit.
	witness := strings.Repeat("captured input ", 400000)
	f := reviewedInspectionTeamFixture(t, func(w http.ResponseWriter, r *http.Request) {
		reads.Add(1)
		jsonReply(w, 200, map[string]any{"sessionId": "s-provider", "projectId": "p-alpha", "worktreeId": "wt-alpha", "version": 1, "digest": strings.Repeat("a", 64), "document": map[string]any{"kind": "plan", "value": map[string]string{"title": witness}}})
	})
	w := request(t, f.h, "GET", f.path("/v1/sessions/s-provider/reviewed-context?document=plan"), f.memberToken, nil, 200)
	if reads.Load() != 1 || !strings.Contains(w.Body.String(), witness) || strings.Contains(w.Body.String(), "attachPath") || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("inspection was truncated, changed into a terminal response, or cached")
	}
	for _, id := range []string{"s-hidden", "s-other", "s-unknown"} {
		request(t, f.h, "GET", f.path("/v1/sessions/"+id+"/reviewed-context?document=definition"), f.memberToken, nil, 403)
	}
	request(t, f.h, "GET", f.path("/v1/sessions/s-provider/attach"), f.memberToken, nil, 403)
	request(t, f.h, "POST", f.path("/v1/sessions/s-provider/reviewed-context"), f.memberToken, nil, 403)
	if reads.Load() != 1 || f.writes.Load() != 0 || f.attaches.Load() != 0 {
		t.Fatal("read-only inspection reached a hidden project or dispatched execution")
	}
}

func TestTeamReviewedContextResponseIsDiscardedWhenProjectReadIsRevoked(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	f := reviewedInspectionTeamFixture(t, func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		jsonReply(w, 200, map[string]any{"sessionId": "s-provider", "projectId": "p-alpha", "document": map[string]any{"kind": "definition", "value": map[string]string{"purpose": "private captured purpose"}}})
	})
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		r := httptest.NewRequest("GET", f.path("/v1/sessions/s-provider/reviewed-context?document=definition"), nil)
		r.Header.Set("Authorization", "Bearer "+f.memberToken)
		w := httptest.NewRecorder()
		f.h.ServeHTTP(w, r)
		done <- w
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("inspection did not reach the private worker")
	}
	f.g.mu.Lock()
	revision := f.g.state.Teams[f.team.ID].Team.Revision
	f.g.mu.Unlock()
	request(t, f.h, "DELETE", "/v1/teams/"+f.team.ID+"/projects/p-alpha/members/"+f.member.ID, f.ownerToken, map[string]int{"expectedRevision": revision}, 200)
	close(release)
	select {
	case w := <-done:
		if w.Code != 403 || strings.Contains(w.Body.String(), "private captured purpose") {
			t.Fatal("revoked reader received the delayed captured document", w.Code, w.Body.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("revoked inspection did not finish")
	}
	request(t, f.h, "GET", f.path("/v1/sessions/s-provider/reviewed-context?document=definition"), f.memberToken, nil, 403)
}

// Pin the race boundary without a scheduler delay: the first Err observes the
// admitted request, then revokes its real grant before returning that observed
// result. Projection and the eventual response must therefore recheck access.
type revokeAfterInspectionCheck struct {
	context.Context
	once   sync.Once
	revoke func()
}

func (c *revokeAfterInspectionCheck) Err() error {
	err := c.Context.Err()
	c.once.Do(c.revoke)
	return err
}

func TestTeamReviewedContextRevocationAfterInitialResponseCheck(t *testing.T) {
	f := reviewedInspectionTeamFixture(t, func(http.ResponseWriter, *http.Request) {
		t.Fatal("finished-response regression must not dispatch another worker request")
	})
	access, err := f.g.resolveTeamAccess(f.team.ID, f.member.ID, "p-alpha")
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "/v1/sessions/s-provider/reviewed-context?document=definition", nil)
	r.Header.Set("Authorization", "Bearer "+f.memberToken)
	r = r.WithContext(withActor(r.Context(), f.member, f.g.state.Policy))
	w := httptest.NewRecorder()
	admitted, finish, err := f.g.RegisterTeamRequest(w, r, f.team.ID, "p-alpha", access.Member.Revision, access.ProjectRevision)
	if err != nil {
		t.Fatal(err)
	}
	defer finish()
	ctx := &revokeAfterInspectionCheck{Context: admitted.Context(), revoke: func() {
		f.g.mu.Lock()
		revision := f.g.state.Teams[f.team.ID].Team.Revision
		f.g.mu.Unlock()
		request(t, f.h, "DELETE", "/v1/teams/"+f.team.ID+"/projects/p-alpha/members/"+f.member.ID, f.ownerToken, map[string]int{"expectedRevision": revision}, 200)
		if admitted.Context().Err() == nil {
			t.Fatal("durable grant removal did not revoke the admitted request")
		}
	}}
	capture := newTeamCapture()
	jsonReply(capture, 200, map[string]any{"sessionId": "s-provider", "document": map[string]any{"kind": "definition", "value": map[string]string{"purpose": strings.Repeat("private captured purpose ", 50000)}}})
	writeTeamSessionResponse(w, ctx, capture, teamWorkspacePrefix(f.team.ID))
	if w.Code != 403 || strings.Contains(w.Body.String(), "private captured purpose") {
		t.Fatalf("revoked reader received projected document: status=%d bytes=%d", w.Code, w.Body.Len())
	}
}
