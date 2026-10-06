package management

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type teamProxyFixture struct {
	g                        *Gateway
	h                        http.Handler
	owner, member            User
	ownerToken, memberToken  string
	team, other              Team
	starts, writes, attaches atomic.Int32
	activeHidden             atomic.Bool
	oversized                atomic.Bool
	beforeSession            func(http.ResponseWriter, *http.Request) bool
	beforeSessionCatalogue   func(*http.Request)
}

type observedTeamExportWriter struct {
	*httptest.ResponseRecorder
	written atomic.Int64
}

func (w *observedTeamExportWriter) Write(data []byte) (int, error) {
	n, err := w.ResponseRecorder.Write(data)
	w.written.Add(int64(n))
	return n, err
}

func TestTeamOrchestratorExportStreamsBeyondCatalogueLimitAndRevokes(t *testing.T) {
	f := newTeamProxyFixture(t)
	// Read permission alone is sufficient to recover private conversation data.
	f.g.mu.Lock()
	row := f.g.state.Teams[f.team.ID]
	member := row.Members[f.member.ID]
	member.Role = "viewer"
	member.Grants = TeamGrants{}
	row.Members[f.member.ID] = member
	grant := row.Projects["p-alpha"][f.member.ID]
	grant.Role = "viewer"
	grant.Grants = TeamGrants{}
	row.Projects["p-alpha"][f.member.ID] = grant
	f.g.state.Teams[f.team.ID] = row
	f.g.mu.Unlock()
	payload := append([]byte(`{"ledger":{"message":"`), bytes.Repeat([]byte("a"), teamResponseLimit+1024)...)
	payload = append(payload, []byte(`"},"archive":[]}`)...)
	started, release := make(chan struct{}), make(chan struct{})
	defer close(release)
	var cancelled atomic.Bool
	f.g.opts.TeamHandler = func(_ context.Context, teamID string) (http.Handler, error) {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/product/projects/p-alpha/orchestrator/export" {
				f.worker(t, teamID, w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Content-Disposition", `attachment; filename="conversation.json"`)
			_, _ = w.Write(payload)
			close(started)
			select {
			case <-r.Context().Done():
				cancelled.Store(true)
			case <-release:
			}
		}), nil
	}
	request(t, f.h, "GET", f.path("/v1/product/projects/p-secret/orchestrator/export"), f.memberToken, nil, 403)
	w := &observedTeamExportWriter{ResponseRecorder: httptest.NewRecorder()}
	done := make(chan struct{})
	go func() {
		r := httptest.NewRequest("GET", f.path("/v1/product/projects/p-alpha/orchestrator/export"), nil)
		r.Header.Set("Authorization", "Bearer "+f.memberToken)
		f.h.ServeHTTP(w, r)
		close(done)
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("owned export did not reach streaming handler")
	}
	if w.written.Load() != int64(len(payload)) {
		t.Fatal("project export was buffered or truncated by catalogue response limit", w.written.Load())
	}
	f.g.mu.Lock()
	revision := f.g.state.Teams[f.team.ID].Team.Revision
	f.g.mu.Unlock()
	request(t, f.h, "DELETE", "/v1/teams/"+f.team.ID+"/members/"+f.member.ID, f.ownerToken, map[string]int{"expectedRevision": revision}, 200)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("revoked conversation download remained active")
	}
	if !cancelled.Load() || w.Code != 200 || w.Body.Len() != len(payload) || !json.Valid(w.Body.Bytes()) || !strings.Contains(w.Header().Get("Content-Disposition"), "conversation.json") {
		t.Fatal("export lost streaming bytes, download metadata or cancellation")
	}
}

func newTeamProxyFixture(t *testing.T) *teamProxyFixture {
	t.Helper()
	g, h, token, owner := fixture(t, nil)
	member := addUser(t, h, token, "scoped-editor", "member")
	f := &teamProxyFixture{g: g, h: h, owner: owner, member: member, ownerToken: token, memberToken: loginAs(t, h, member.Username)}
	f.team = createTestTeam(t, h, token, "Allowed team")
	f.other = createTestTeam(t, h, token, "Other team")
	for _, team := range []Team{f.team, f.other} {
		request(t, h, "POST", "/v1/teams/"+team.ID+"/members", token, teamMemberBody(team.Revision, member.Username, "editor", TeamGrants{Execute: true, Secrets: true, Deploy: true}), 200)
		row := g.state.Teams[team.ID]
		projectID := "p-alpha"
		if team.ID == f.other.ID {
			projectID = "p-other"
		}
		row.Projects[projectID] = map[string]TeamProjectGrant{member.ID: {UserID: member.ID, Role: "editor", Grants: TeamGrants{Execute: true, Secrets: true, Deploy: true}, Revision: row.Team.Revision}}
		g.state.Teams[team.ID] = row
	}
	if err := g.persistLocked(); err != nil {
		t.Fatal(err)
	}
	g.opts.TeamHandler = func(ctx context.Context, teamID string) (http.Handler, error) {
		f.starts.Add(1)
		if teamID != f.team.ID && teamID != f.other.ID {
			return nil, fmt.Errorf("foreign team")
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { f.worker(t, teamID, w, r) }), nil
	}
	return f
}

func (f *teamProxyFixture) session(teamID, id string) map[string]any {
	projectID, worktreeID, harness, authOnly := "p-alpha", "wt-alpha", "shell", false
	switch id {
	case "s-visible":
		if teamID != f.team.ID {
			return nil
		}
	case "s-hidden":
		if teamID != f.team.ID {
			return nil
		}
		projectID, worktreeID = "p-secret", "wt-secret"
	case "s-provider":
		if teamID != f.team.ID {
			return nil
		}
		harness = "codex"
	case "s-sign-in":
		if teamID != f.team.ID {
			return nil
		}
		projectID, worktreeID, harness, authOnly = "", "setup-codex", "codex", true
	case "s-other":
		if teamID != f.other.ID {
			return nil
		}
		projectID, worktreeID = "p-other", "wt-other"
	default:
		return nil
	}
	status := "exited"
	if id == "s-hidden" && f.activeHidden.Load() {
		status = "running"
	}
	return map[string]any{"id": id, "projectId": projectID, "worktreeId": worktreeID, "harness": harness, "authOnly": authOnly, "status": status, "attachPath": "/v1/sessions/" + id + "/attach", "openingPrompt": "owned context for " + projectID}
}

func (f *teamProxyFixture) worker(t *testing.T, teamID string, w http.ResponseWriter, r *http.Request) {
	t.Helper()
	u, p, ok := ActorFromContext(r.Context())
	access, scoped := TeamAccessFromContext(r.Context())
	if !ok || !scoped || access.Team.ID != teamID || access.Member.UserID != u.ID || TeamRuntimeIDFromContext(r.Context()) != teamID || p.AllowHostCommands {
		t.Error("team worker did not receive its scoped real actor and restricted policy")
	}
	if u.ID != f.owner.ID && u.ID != f.member.ID {
		t.Error("browser forged worker actor")
	}
	if r.Header.Get("X-Luci-Actor") != "" || r.Header.Get(teamWorkerHeader) != "" || r.Header.Get(teamWorkerAccessHeader) != "" {
		t.Error("browser actor/team headers reached worker")
	}
	path := r.URL.Path
	if path == "/v1/projects" && r.Method == "GET" {
		if f.oversized.Load() {
			w.WriteHeader(200)
			_, _ = w.Write(bytes.Repeat([]byte("x"), teamResponseLimit+1024))
			return
		}
		projects := []map[string]any{{"id": "p-alpha", "name": "Visible project"}, {"id": "p-secret", "name": "Hidden project"}}
		if teamID == f.other.ID {
			projects = []map[string]any{{"id": "p-other", "name": "Other team project"}}
		}
		jsonReply(w, 200, map[string]any{"projects": projects, "privateSummary": "p-secret must never escape in an extra envelope field"})
		return
	}
	if path == "/v1/sessions" && r.Method == "GET" {
		if f.beforeSessionCatalogue != nil {
			f.beforeSessionCatalogue(r)
		}
		ids := []string{"s-visible", "s-hidden", "s-provider", "s-sign-in"}
		if teamID == f.other.ID {
			ids = []string{"s-other"}
		}
		rows := []map[string]any{}
		for _, id := range ids {
			rows = append(rows, f.session(teamID, id))
		}
		jsonReply(w, 200, map[string]any{"sessions": rows, "privateSummary": "s-hidden private metadata"})
		return
	}
	if strings.HasPrefix(path, "/v1/projects/") && strings.HasSuffix(path, "/worktrees") {
		id := strings.Split(path, "/")[3]
		worktrees := []map[string]string{}
		if teamID == f.team.ID && (id == "p-alpha" || id == "p-secret") || teamID == f.other.ID && id == "p-other" {
			worktrees = append(worktrees, map[string]string{"id": "wt-" + strings.TrimPrefix(id, "p-"), "projectId": id})
		}
		jsonReply(w, 200, map[string]any{"worktrees": worktrees})
		return
	}
	if strings.HasPrefix(path, "/v1/sessions/") {
		parts := strings.Split(path, "/")
		session := f.session(teamID, parts[3])
		if session == nil {
			fail(w, 404, "missing", "missing session")
			return
		}
		if r.Method == "GET" && len(parts) == 4 && f.beforeSession != nil && f.beforeSession(w, r) {
			return
		}
		if strings.HasSuffix(path, "/attach") {
			f.attaches.Add(1)
			w.WriteHeader(204)
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			f.writes.Add(1)
		}
		jsonReply(w, 200, session)
		return
	}
	if path == "/v1/sessions" && r.Method == "POST" {
		f.writes.Add(1)
		var body struct {
			WorktreeID string `json:"worktreeId"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		projectID := "p-" + strings.TrimPrefix(body.WorktreeID, "wt-")
		jsonReply(w, 201, map[string]any{"id": "s-created", "projectId": projectID, "worktreeId": body.WorktreeID, "harness": "shell", "attachPath": "/v1/sessions/s-created/attach"})
		return
	}
	if path == "/v1/setup/agents/codex/sign-in" && r.Method == "POST" {
		f.writes.Add(1)
		jsonReply(w, 201, f.session(teamID, "s-sign-in"))
		return
	}
	if r.Method != "GET" && r.Method != "HEAD" {
		f.writes.Add(1)
	}
	jsonReply(w, 200, map[string]string{"projectId": "p-alpha", "actorId": u.ID, "role": u.Role, "teamId": teamID})
}

func (f *teamProxyFixture) path(inner string) string { return teamWorkspacePrefix(f.team.ID) + inner }

func TestTeamProxyFiltersPrivateCataloguesAndNamespacesSessionAttachments(t *testing.T) {
	f := newTeamProxyFixture(t)
	projects := request(t, f.h, "GET", f.path("/v1/projects"), f.memberToken, nil, 200)
	if strings.Contains(projects.Body.String(), "p-secret") || strings.Contains(projects.Body.String(), "privateSummary") || !strings.Contains(projects.Body.String(), "p-alpha") {
		t.Fatalf("unfiltered projects: %s", projects.Body.String())
	}
	sessions := request(t, f.h, "GET", f.path("/v1/sessions"), f.memberToken, nil, 200)
	for _, hidden := range []string{"s-hidden", "s-sign-in", "p-secret", "privateSummary"} {
		if strings.Contains(sessions.Body.String(), hidden) {
			t.Fatalf("unfiltered sessions: %s", sessions.Body.String())
		}
	}
	if !strings.Contains(sessions.Body.String(), f.path("/v1/sessions/s-visible/attach")) {
		t.Fatal("session list retained a personal attach path")
	}
	detail := request(t, f.h, "GET", f.path("/v1/sessions/s-visible"), f.memberToken, nil, 200)
	if !strings.Contains(detail.Body.String(), f.path("/v1/sessions/s-visible/attach")) {
		t.Fatal("session detail retained a personal attach path")
	}
	request(t, f.h, "GET", f.path("/v1/sessions/s-hidden"), f.memberToken, nil, 403)
	request(t, f.h, "GET", f.path("/v1/sessions/s-other"), f.memberToken, nil, 403)
	other := request(t, f.h, "GET", teamWorkspacePrefix(f.other.ID)+"/v1/sessions/s-other", f.memberToken, nil, 200)
	if strings.Contains(other.Body.String(), "p-alpha") || !strings.Contains(other.Body.String(), "p-other") {
		t.Fatal("opaque session routed into a different team's worker")
	}
}

func TestTeamProxyRejectsInsufficientWritersBeforeWorkerStartup(t *testing.T) {
	f := newTeamProxyFixture(t)
	row := f.g.state.Teams[f.team.ID]
	member := row.Members[f.member.ID]
	member.Role = "viewer"
	row.Members[f.member.ID] = member
	f.g.state.Teams[f.team.ID] = row
	for _, endpoint := range []string{"POST /v1/product/projects/p-alpha/brief", "POST /v1/product/projects/p-alpha/checks", "POST /v1/product/projects/p-alpha/releases", "POST /v1/sessions", "GET /v1/sessions/s-visible/attach", "POST /v1/setup/agent-connections", "POST /v1/setup/workspace/activate"} {
		method, path, _ := strings.Cut(endpoint, " ")
		request(t, f.h, method, f.path(path), f.memberToken, map[string]string{"worktreeId": "wt-alpha", "harness": "shell"}, 403)
	}
	if f.starts.Load() != 0 || f.writes.Load() != 0 || f.attaches.Load() != 0 {
		t.Fatal("denied team requests started a worker or reached its operation")
	}
	request(t, f.h, "GET", f.path("/v1/product/projects/p-alpha/brief"), f.memberToken, nil, 200)
	if f.starts.Load() == 0 {
		t.Fatal("authorized viewer could not read shared context")
	}
}

func TestTeamProxyResolvesSessionWorktreeAndResumeSourceGrants(t *testing.T) {
	f := newTeamProxyFixture(t)
	for _, body := range []map[string]string{
		{"worktreeId": "wt-secret", "harness": "shell"},
		{"worktreeId": "wt-other", "harness": "shell"},
		{"worktreeId": "wt-alpha", "harness": "shell", "resume": "s-hidden"},
		{"worktreeId": "wt-alpha", "harness": "shell", "resume": "s-other"},
	} {
		request(t, f.h, "POST", f.path("/v1/sessions"), f.memberToken, body, 403)
	}
	request(t, f.h, "PATCH", f.path("/v1/projects/p-alpha/worktrees/wt-secret"), f.memberToken, map[string]bool{"ready": true}, 403)
	if f.writes.Load() != 0 {
		t.Fatal("a foreign worktree or resume source caused a mutation")
	}
	created := request(t, f.h, "POST", f.path("/v1/sessions"), f.memberToken, map[string]string{"worktreeId": "wt-alpha", "harness": "shell", "resume": "s-visible"}, 201)
	if !strings.Contains(created.Body.String(), f.path("/v1/sessions/s-created/attach")) || f.writes.Load() != 1 {
		t.Fatalf("owned session did not preserve scoped attachment: %s", created.Body.String())
	}
	request(t, f.h, "GET", f.path("/v1/sessions/s-visible/attach"), f.memberToken, nil, 204)
	if f.attaches.Load() != 1 {
		t.Fatal("owned terminal was not admitted exactly once")
	}
}

func TestTeamProxyQuotaIncludesSessionsHiddenByProjectACL(t *testing.T) {
	f := newTeamProxyFixture(t)
	f.activeHidden.Store(true)
	visible := request(t, f.h, "GET", f.path("/v1/sessions"), f.memberToken, nil, 200)
	if strings.Contains(visible.Body.String(), "s-hidden") {
		t.Fatal("quota setup exposed another project's session")
	}
	request(t, f.h, "POST", f.path("/v1/sessions"), f.memberToken, map[string]string{"worktreeId": "wt-alpha", "harness": "shell"}, 409)
	if f.writes.Load() != 0 {
		t.Fatal("project filtering bypassed the shared team session quota")
	}
}

func TestTeamProxyDeviceSignInIsAnExplicitTeamAdministratorUtility(t *testing.T) {
	f := newTeamProxyFixture(t)
	request(t, f.h, "GET", f.path("/v1/sessions/s-sign-in"), f.memberToken, nil, 403)
	request(t, f.h, "POST", f.path("/v1/setup/agents/codex/sign-in"), f.memberToken, map[string]any{}, 403)
	request(t, f.h, "POST", f.path("/v1/sessions"), f.memberToken, map[string]any{"worktreeId": "wt-alpha", "harness": "codex", "authOnly": true}, 403)
	if f.writes.Load() != 0 {
		t.Fatal("project capability granted shared account administration")
	}
	created := request(t, f.h, "POST", f.path("/v1/setup/agents/codex/sign-in"), f.ownerToken, map[string]any{}, 201)
	if !strings.Contains(created.Body.String(), f.path("/v1/sessions/s-sign-in/attach")) {
		t.Fatal("device sign-in returned a personal terminal path")
	}
	list := request(t, f.h, "GET", f.path("/v1/sessions"), f.ownerToken, nil, 200)
	if !strings.Contains(list.Body.String(), "s-sign-in") {
		t.Fatal("authorized administrator could not recover its project-free sign-in")
	}
	request(t, f.h, "GET", f.path("/v1/sessions/s-sign-in/attach"), f.ownerToken, nil, 204)
	request(t, f.h, "POST", "/v1/auth/socket-ticket", f.ownerToken, map[string]string{"path": f.path("/v1/sessions/s-sign-in/attach")}, 200)
	request(t, f.h, "POST", "/v1/auth/socket-ticket", f.memberToken, map[string]string{"path": f.path("/v1/sessions/s-sign-in/attach")}, 403)
}

func TestTeamProxyProviderCredentialUseDoesNotFollowFromExecutionAlone(t *testing.T) {
	f := newTeamProxyFixture(t)
	row := f.g.state.Teams[f.team.ID]
	member := row.Members[f.member.ID]
	member.Grants.Secrets = false
	row.Members[f.member.ID] = member
	f.g.state.Teams[f.team.ID] = row
	request(t, f.h, "POST", f.path("/v1/sessions"), f.memberToken, map[string]string{"worktreeId": "wt-alpha", "harness": "codex"}, 403)
	request(t, f.h, "GET", f.path("/v1/sessions/s-provider/attach"), f.memberToken, nil, 403)
	request(t, f.h, "POST", f.path("/v1/sessions/s-provider/restart"), f.memberToken, map[string]any{}, 403)
	request(t, f.h, "POST", f.path("/v1/product/projects/p-alpha/assist"), f.memberToken, map[string]string{"harness": "codex"}, 403)
	if f.writes.Load() != 0 || f.attaches.Load() != 0 {
		t.Fatal("execution-only member used injected provider credentials")
	}
	request(t, f.h, "GET", f.path("/v1/sessions/s-visible/attach"), f.memberToken, nil, 204)
}

func TestTeamProxyDropsBufferedSessionResultAfterRevocation(t *testing.T) {
	f := newTeamProxyFixture(t)
	started, release := make(chan struct{}), make(chan struct{})
	var reads atomic.Int32
	f.beforeSession = func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path != "/v1/sessions/s-visible" || reads.Add(1) != 2 {
			return false
		}
		close(started)
		<-release
		jsonReply(w, 200, f.session(f.team.ID, "s-visible"))
		return true
	}
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		r := httptest.NewRequest("GET", f.path("/v1/sessions/s-visible"), nil)
		r.Header.Set("Authorization", "Bearer "+f.memberToken)
		w := httptest.NewRecorder()
		f.h.ServeHTTP(w, r)
		done <- w
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("session read did not reach revocation barrier")
	}
	team := f.g.state.Teams[f.team.ID].Team
	request(t, f.h, "DELETE", "/v1/teams/"+team.ID+"/members/"+f.member.ID, f.ownerToken, map[string]int{"expectedRevision": team.Revision}, 200)
	close(release)
	select {
	case response := <-done:
		if response.Code != 403 || strings.Contains(response.Body.String(), "owned context") {
			t.Fatalf("revoked read returned session data: %d %s", response.Code, response.Body.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("revoked read did not finish")
	}
}

func TestTeamProxyRevokingResumeSourceCancelsQueuedTargetCreation(t *testing.T) {
	f := newTeamProxyFixture(t)
	row := f.g.state.Teams[f.team.ID]
	row.Projects["p-secret"] = map[string]TeamProjectGrant{f.member.ID: {UserID: f.member.ID, Role: "editor", Grants: TeamGrants{Execute: true}, Revision: row.Team.Revision}}
	f.g.state.Teams[f.team.ID] = row
	if err := f.g.persistLocked(); err != nil {
		t.Fatal(err)
	}
	started, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	f.beforeSessionCatalogue = func(r *http.Request) {
		if calls.Add(1) == 1 {
			close(started)
			<-release
		}
	}
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		r := httptest.NewRequest("POST", f.path("/v1/sessions"), strings.NewReader(`{"worktreeId":"wt-alpha","harness":"shell","resume":"s-hidden"}`))
		r.Header.Set("Authorization", "Bearer "+f.memberToken)
		w := httptest.NewRecorder()
		f.h.ServeHTTP(w, r)
		done <- w
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("resume did not reach session admission")
	}
	request(t, f.h, "DELETE", "/v1/teams/"+f.team.ID+"/projects/p-secret/members/"+f.member.ID, f.ownerToken, map[string]int{"expectedRevision": row.Team.Revision}, 200)
	close(release)
	select {
	case response := <-done:
		if response.Code != 403 || f.writes.Load() != 0 {
			t.Fatalf("revoked source created a target session: %d %s writes=%d", response.Code, response.Body.String(), f.writes.Load())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("resume did not finish after source revocation")
	}
	f.g.mu.Lock()
	leaked := len(f.g.teamRequests)
	f.g.mu.Unlock()
	if leaked != 0 {
		t.Fatalf("resume left %d registered request guards", leaked)
	}
	// The target's grant remains valid: only reading the removed source is denied.
	request(t, f.h, "POST", f.path("/v1/sessions"), f.memberToken, map[string]string{"worktreeId": "wt-alpha", "harness": "shell"}, 201)
}

func TestTeamProxySerializesLastProjectSlotAcrossCreateAndRestore(t *testing.T) {
	f := newTeamProxyFixture(t)
	var count, reads, creates atomic.Int32
	count.Store(255)
	started, release, secondRead := make(chan struct{}), make(chan struct{}), make(chan struct{})
	f.g.opts.TeamHandler = func(context.Context, string) (http.Handler, error) {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "GET" && r.URL.Path == "/v1/projects" {
				current := count.Load()
				if reads.Add(1) == 2 {
					close(secondRead)
				}
				projects := make([]map[string]string, current)
				for index := range projects {
					projects[index] = map[string]string{"id": fmt.Sprintf("p-capacity-%d", index)}
				}
				jsonReply(w, 200, map[string]any{"projects": projects})
				return
			}
			if r.Method == "POST" && strings.HasPrefix(r.URL.Path, "/v1/product/projects") {
				if creates.Add(1) == 1 {
					close(started)
					<-release
				}
				id := count.Add(1)
				jsonReply(w, 201, map[string]string{"id": fmt.Sprintf("p-capacity-%d", id-1)})
				return
			}
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}), nil
	}
	done := make(chan *httptest.ResponseRecorder, 2)
	create := func(path string) {
		r := httptest.NewRequest("POST", f.path(path), strings.NewReader(`{"name":"project"}`))
		r.Header.Set("Authorization", "Bearer "+f.ownerToken)
		w := httptest.NewRecorder()
		f.h.ServeHTTP(w, r)
		done <- w
	}
	go create("/v1/product/projects")
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("first project did not reach allocation")
	}
	go create("/v1/product/projects/restore")
	// An unsynchronized importer can read 255 while creation is paused. A
	// serialized importer waits until the first allocation publishes 256.
	select {
	case <-secondRead:
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	statuses := map[int]int{}
	for index := 0; index < 2; index++ {
		select {
		case response := <-done:
			statuses[response.Code]++
		case <-time.After(5 * time.Second):
			t.Fatal("project admission did not complete")
		}
	}
	if statuses[201] != 1 || statuses[409] != 1 || count.Load() != 256 || creates.Load() != 1 {
		t.Fatalf("project limit raced: statuses=%v total=%d allocations=%d", statuses, count.Load(), creates.Load())
	}
	if visible := request(t, f.h, "GET", f.path("/v1/projects"), f.ownerToken, nil, 200); !strings.Contains(visible.Body.String(), "p-capacity-255") {
		t.Fatal("catalogue became inaccessible after reaching capacity")
	}
}

func TestTeamProxyBoundsWorkerResponsesAndRejectsBrowserAuthorityHeaders(t *testing.T) {
	f := newTeamProxyFixture(t)
	r := httptest.NewRequest("GET", f.path("/v1/product/projects/p-alpha/brief"), nil)
	r.Header.Set("Authorization", "Bearer "+f.memberToken)
	r.Header.Set("X-Luci-Actor", "browser-forged-owner")
	r.Header.Set(teamWorkerHeader, f.other.ID)
	r.Header.Set(teamWorkerAccessHeader, "browser-forged-grants")
	w := httptest.NewRecorder()
	f.h.ServeHTTP(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), f.member.ID) || strings.Contains(w.Body.String(), f.owner.ID) {
		t.Fatalf("actor was not preserved: %s", w.Body.String())
	}
	f.oversized.Store(true)
	response := request(t, f.h, "GET", f.path("/v1/projects"), f.memberToken, nil, 503)
	if response.Body.Len() > 1024 {
		t.Fatal("oversized private response leaked through error response")
	}
	capture := newTeamCapture()
	_, _ = capture.Write(bytes.Repeat([]byte("x"), teamResponseLimit+100))
	_, _ = capture.Write([]byte("extra"))
	if !capture.overflow || capture.body.Len() != teamResponseLimit {
		t.Fatalf("response limit was not enforced while writing: %d", capture.body.Len())
	}
}

func TestTeamProjectGrantCRUDChecksWorkerOwnershipAndKeepsFailedChangesPrivate(t *testing.T) {
	f := newTeamProxyFixture(t)
	base := "/v1/teams/" + f.team.ID + "/projects/"
	team := f.g.state.Teams[f.team.ID].Team
	request(t, f.h, "PUT", base+"p-other/members/"+f.member.ID, f.ownerToken, teamMemberBody(team.Revision, "", "viewer", TeamGrants{}), 404)
	request(t, f.h, "PUT", base+"p-secret/members/"+f.member.ID, f.memberToken, teamMemberBody(team.Revision, "", "editor", TeamGrants{Execute: true}), 403)
	if f.g.state.Teams[f.team.ID].Team.Revision != team.Revision {
		t.Fatal("foreign project or nonadmin request changed authority")
	}
	request(t, f.h, "PUT", base+"p-secret/members/"+f.member.ID, f.ownerToken, teamMemberBody(team.Revision, "", "viewer", TeamGrants{}), 200)
	request(t, f.h, "GET", f.path("/v1/product/projects/p-secret/brief"), f.memberToken, nil, 200)
	request(t, f.h, "POST", f.path("/v1/product/projects/p-secret/brief"), f.memberToken, map[string]any{}, 403)
	request(t, f.h, "PUT", base+"p-secret/members/"+f.member.ID, f.ownerToken, teamMemberBody(team.Revision, "", "editor", TeamGrants{Execute: true}), 409)
	team = f.g.state.Teams[f.team.ID].Team
	before := cloneIdentityState(f.g.state)
	restore := blockIdentityCommit(t, f.g)
	request(t, f.h, "PUT", base+"p-secret/members/"+f.member.ID, f.ownerToken, teamMemberBody(team.Revision, "", "editor", TeamGrants{Execute: true}), 500)
	assertIdentityUnchanged(t, f.g, before)
	request(t, f.h, "DELETE", base+"p-secret/members/"+f.member.ID, f.ownerToken, map[string]int{"expectedRevision": team.Revision}, 500)
	assertIdentityUnchanged(t, f.g, before)
	request(t, f.h, "GET", base+"p-secret/members", f.memberToken, nil, 200)
	restore()
	request(t, f.h, "DELETE", base+"p-secret/members/"+f.member.ID, f.ownerToken, map[string]int{"expectedRevision": team.Revision}, 200)
	request(t, f.h, "GET", f.path("/v1/product/projects/p-secret/brief"), f.memberToken, nil, 403)
	if f.writes.Load() != 0 {
		t.Fatal("permission administration mutated project content")
	}
}
