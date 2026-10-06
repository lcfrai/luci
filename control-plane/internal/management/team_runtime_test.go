package management

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func teamRuntimeTestPolicy() Policy {
	return Policy{InstanceName: "team-test", BackingMode: "docker", MaxSessionsPerUser: 1, MaxConcurrentChecksPerUser: 1, SessionMemoryMiB: 1024, SessionCPUMillis: 1000, CheckMemoryMiB: 1024, CheckCPUMillis: 1000, AllowedHarnesses: []string{"shell"}, AllowHostCommands: true}
}

func runtimeTestTeam(t *testing.T, g *Gateway, h http.Handler, token string, user User) TeamAccess {
	t.Helper()
	w := request(t, h, "POST", "/v1/teams", token, map[string]string{"name": "Shared runtime"}, 201)
	var result struct {
		Team Team `json:"team"`
	}
	if json.Unmarshal(w.Body.Bytes(), &result) != nil {
		t.Fatal("invalid team creation")
	}
	access, err := g.resolveTeamAccess(result.Team.ID, user.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	return access
}

func TestTeamRuntimeEnvironmentSeparatesCredentialsAndPinsTeam(t *testing.T) {
	teamID := "team-" + strings.Repeat("a", 22)
	root := filepath.Join(t.TempDir(), "team-runtime")
	env := isolatedTeamEnvironment([]string{"PATH=/trusted/bin", "LUCI_TEAM_ID=other-team", "LUCI_ACTOR_ID=personal-owner", "GH_TOKEN=private-github", "OPENAI_API_KEY=private-model", "AWS_ACCESS_KEY_ID=private-cloud", "HOME=/personal/home"}, root, "worker-secret", filepath.Join(root, "address.json"), teamID, User{ID: "actual-user", Role: "member"})
	values := map[string]string{}
	for _, entry := range env {
		key, value, _ := strings.Cut(entry, "=")
		values[key] = value
	}
	if values["LUCI_TEAM_ID"] != teamID || values["LUCI_ACTOR_ID"] != "" || values["LUCI_ACTOR_ROLE"] != "" || values["HOME"] != filepath.Join(root, "home") || values["LUCI_PROJECTS_DIR"] != filepath.Join(root, "projects") || values["LUCI_STATE_DIR"] != filepath.Join(root, "state") || values["LUCI_BACKEND"] != "docker" {
		t.Fatal("team environment lost its fixed private boundary")
	}
	for _, key := range []string{"GH_TOKEN", "OPENAI_API_KEY", "AWS_ACCESS_KEY_ID"} {
		if values[key] != "" {
			t.Fatal("team inherited personal credential", key)
		}
	}
}

func TestTeamWorkerRequiresFixedScopeAndPreservesHumanActor(t *testing.T) {
	teamID := "team-" + strings.Repeat("a", 22)
	t.Setenv("LUCI_TEAM_ID", teamID)
	t.Setenv("LUCI_ACTOR_ID", "never-impersonate-this-user")
	token := randomToken()
	called := 0
	handler := WorkerHandler(token, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		actor, policy, ok := ActorFromContext(r.Context())
		access, scoped := TeamAccessFromContext(r.Context())
		if !ok || !scoped || actor.ID != access.Member.UserID || actor.Role != "member" || policy.AllowHostCommands || TeamRuntimeIDFromContext(r.Context()) != teamID || r.Header.Get(teamWorkerHeader) != "" || r.Header.Get(teamWorkerAccessHeader) != "" {
			t.Fatal("team worker lost human identity or retained privileged transport headers")
		}
		io.WriteString(w, "ok")
	}))
	for _, userID := range []string{"first-human", "second-human"} {
		access := TeamAccess{Team: Team{ID: teamID, Revision: 1}, Member: TeamMember{UserID: userID, Role: "viewer", Revision: 1}, Permissions: TeamPermissions{Read: true}}
		encoded, _ := json.Marshal(access)
		for _, scenario := range []string{"valid", "wrong-team", "missing-scope", "wrong-actor", "wrong-token"} {
			r := httptest.NewRequest("GET", "/v1/projects", nil)
			setWorkerHeaders(r, User{ID: userID, Role: "super-admin"}, teamRuntimeTestPolicy(), token)
			r.Header.Set(teamWorkerHeader, teamID)
			r.Header.Set(teamWorkerAccessHeader, base64.RawURLEncoding.EncodeToString(encoded))
			want := 403
			switch scenario {
			case "valid":
				want = 200
			case "wrong-team":
				r.Header.Set(teamWorkerHeader, "team-"+strings.Repeat("b", 22))
			case "missing-scope":
				r.Header.Del(teamWorkerAccessHeader)
			case "wrong-actor":
				wrong, _ := json.Marshal(User{ID: "different-human", Role: "member"})
				r.Header.Set("X-Luci-Actor", base64.RawURLEncoding.EncodeToString(wrong))
			case "wrong-token":
				r.Header.Set("Authorization", "Bearer wrong-token")
				want = 401
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != want {
				t.Fatal(scenario, w.Code, w.Body.String())
			}
		}
	}
	if called != 2 {
		t.Fatal("an invalid team request reached the worker")
	}
}

func TestTeamWorkerProcessSharesOnlyItsDedicatedStateAndKeepsActors(t *testing.T) {
	g, h, token, owner := fixture(t, nil)
	g.opts.WorkerArgs = []string{"-test.run=^TestTeamWorkerHelper$"}
	g.opts.BaseEnvironment = append(os.Environ(), "GH_TOKEN=fixture-personal-token", "OPENAI_API_KEY=fixture-personal-model", "AWS_ACCESS_KEY_ID=fixture-personal-cloud")
	access := runtimeTestTeam(t, g, h, token, owner)
	other := addUser(t, h, token, "team-colleague", "member")
	request(t, h, "POST", "/v1/teams/"+access.Team.ID+"/members", token, map[string]any{"expectedRevision": access.Team.Revision, "username": other.Username, "role": "editor", "grants": TeamGrants{Execute: true}}, 200)
	var firstRoot string
	for _, user := range []User{owner, other} {
		current, err := g.resolveTeamAccess(access.Team.ID, user.ID, "")
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(WithTeamAccess(context.Background(), current), 35*time.Second)
		worker, err := g.teamExecutionHandler(ctx, current.Team.ID, user, g.state.Policy)
		cancel()
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest("GET", "/v1/projects", nil)
		w := httptest.NewRecorder()
		worker.ServeHTTP(w, r)
		var proof map[string]string
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &proof) != nil {
			t.Fatal(w.Code, w.Body.String())
		}
		want := filepath.Join(g.dir, "teams", current.Team.ID, "runtime")
		if proof["actor"] != user.ID || proof["team"] != current.Team.ID || proof["projects"] != filepath.Join(want, "projects") || proof["home"] != filepath.Join(want, "home") || proof["ghToken"] != "" || proof["modelToken"] != "" || proof["awsToken"] != "" {
			t.Fatal("team worker leaked account state or impersonated its starter")
		}
		if firstRoot != "" && firstRoot != proof["projects"] {
			t.Fatal("team members reached different workspaces")
		}
		firstRoot = proof["projects"]
	}
}

func TestTeamWorkerHelper(t *testing.T) {
	if os.Getenv("LUCI_WORKER") != "1" || os.Getenv("LUCI_TEAM_ID") == "" {
		return
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		os.Exit(2)
	}
	mux := http.NewServeMux()
	server := &http.Server{Handler: WorkerHandler(os.Getenv("LUCI_WORKER_TOKEN"), mux)}
	mux.HandleFunc("GET /v1/setup/status", func(w http.ResponseWriter, r *http.Request) { jsonReply(w, 200, map[string]bool{"ready": true}) })
	mux.HandleFunc("POST /v1/setup/shutdown", func(w http.ResponseWriter, r *http.Request) {
		jsonReply(w, 200, map[string]bool{"stopping": true})
		go server.Shutdown(context.Background())
	})
	mux.HandleFunc("GET /v1/projects", func(w http.ResponseWriter, r *http.Request) {
		actor, _, ok := ActorFromContext(r.Context())
		if !ok {
			os.Exit(3)
		}
		jsonReply(w, 200, map[string]string{"actor": actor.ID, "team": TeamRuntimeIDFromContext(r.Context()), "projects": os.Getenv("LUCI_PROJECTS_DIR"), "home": os.Getenv("HOME"), "ghToken": os.Getenv("GH_TOKEN"), "modelToken": os.Getenv("OPENAI_API_KEY"), "awsToken": os.Getenv("AWS_ACCESS_KEY_ID")})
	})
	if WriteWorkerAddress(os.Getenv("LUCI_WORKER_ADDRESS_FILE"), listener.Addr().String()) != nil {
		os.Exit(4)
	}
	_ = server.Serve(listener)
	os.Exit(0)
}

func TestTeamRevocationCancelsOnlyMatchingLiveRequests(t *testing.T) {
	g, h, token, owner := fixture(t, nil)
	first := runtimeTestTeam(t, g, h, token, owner)
	second := runtimeTestTeam(t, g, h, token, owner)
	requests := []*http.Request{}
	for _, access := range []TeamAccess{first, second} {
		r := httptest.NewRequest("GET", "/v1/projects", nil)
		r.Header.Set("Authorization", "Bearer "+token)
		r = r.WithContext(WithTeamAccess(withActor(r.Context(), owner, g.state.Policy), access))
		registered, finish, err := g.RegisterTeamRequest(httptest.NewRecorder(), r, access.Team.ID, "", access.Member.Revision, 0)
		if err != nil {
			t.Fatal(err)
		}
		defer finish()
		requests = append(requests, registered)
	}
	g.mu.Lock()
	g.revokeTeamAccessLocked(first.Team.ID, owner.ID, "")
	g.mu.Unlock()
	if requests[0].Context().Err() == nil || requests[1].Context().Err() != nil {
		t.Fatal("team revocation failed or canceled an unrelated workspace")
	}
}

func TestTeamTerminalTicketsAndUpgradedSocketsRevokeWithMembership(t *testing.T) {
	engine := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/v1/sessions/owned" {
			jsonReply(w, 200, map[string]string{"id": "owned", "projectId": "p-shared", "harness": "shell", "status": "running"})
			return
		}
		if r.URL.Path == "/v1/projects" {
			jsonReply(w, 200, map[string]any{"projects": []map[string]string{{"id": "p-shared"}}})
			return
		}
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		for {
			typ, payload, err := c.Read(r.Context())
			if err != nil || c.Write(r.Context(), typ, payload) != nil {
				return
			}
		}
	})
	g, h, token, owner := fixture(t, nil)
	g.opts.TeamHandler = func(context.Context, string) (http.Handler, error) { return engine, nil }
	access := runtimeTestTeam(t, g, h, token, owner)
	path := "/v1/teams/" + access.Team.ID + "/workspace/v1/sessions/owned/attach"
	mint := func() string {
		w := request(t, h, "POST", "/v1/auth/socket-ticket", token, map[string]string{"path": path}, 200)
		var ticket struct {
			Ticket string `json:"ticket"`
		}
		if json.Unmarshal(w.Body.Bytes(), &ticket) != nil {
			t.Fatal("invalid team terminal ticket")
		}
		return ticket.Ticket
	}
	activeTicket, unusedTicket := mint(), mint()
	server := httptest.NewServer(h)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	base := strings.Replace(server.URL, "http://", "ws://", 1) + path
	c, _, err := websocket.Dial(ctx, base+"?ticket="+activeTicket, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()
	if err := c.Write(ctx, websocket.MessageText, []byte("team-proof")); err != nil {
		t.Fatal(err)
	}
	if _, data, err := c.Read(ctx); err != nil || string(data) != "team-proof" {
		t.Fatal("team socket did not carry the authorized session", err)
	}
	request(t, h, "PUT", "/v1/teams/"+access.Team.ID+"/members/"+owner.ID, token, map[string]any{"expectedRevision": access.Team.Revision, "role": "owner", "grants": TeamGrants{Admin: true}}, 200)
	if _, _, err := c.Read(ctx); err == nil {
		t.Fatal("terminal remained open after execution access was revoked")
	}
	if conn, _, err := websocket.Dial(ctx, base+"?ticket="+unusedTicket, nil); err == nil {
		conn.CloseNow()
		t.Fatal("old team membership ticket remained usable")
	}
}
