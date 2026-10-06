package management

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestManagedTeamBindingPersistsAndRequiresOneExistingTeam(t *testing.T) {
	g, h, token, _ := fixture(t, nil)
	opts := g.opts
	for _, id := range []string{"wrong-id", "team-aaaaaaaaaaaaaaaaaaaaaa"} {
		opts.ManagedTeamID = id
		if opened, err := New(opts); err == nil {
			opened.Close(context.Background())
			t.Fatal("bound an unknown or malformed team")
		}
	}
	path := filepath.Join(g.dir, "managed-team.json")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("rejected binding created a marker")
	}
	team := createTestTeam(t, h, token, "Dedicated team")
	opts.ManagedTeamID = team.ID
	bound, err := New(opts)
	if err != nil {
		t.Fatal(err)
	}
	bound.Close(context.Background())
	if bound.managedTeamID != team.ID {
		t.Fatal("binding did not select requested team")
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("binding is not private", err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	opts.ManagedTeamID = ""
	reopened, err := New(opts)
	if err != nil {
		t.Fatal(err)
	}
	reopened.Close(context.Background())
	if reopened.managedTeamID != team.ID {
		t.Fatal("omitted startup environment disabled a saved binding")
	}
	opts.ManagedTeamID = "team-aaaaaaaaaaaaaaaaaaaaaa"
	if opened, err := New(opts); err == nil {
		opened.Close(context.Background())
		t.Fatal("retargeted an immutable installation binding")
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("failed retarget replaced the original binding")
	}
}

func TestManagedTeamBindingRejectsMultipleTeamsAndDamagedMarkers(t *testing.T) {
	t.Run("multiple existing teams", func(t *testing.T) {
		g, h, token, _ := fixture(t, nil)
		team := createTestTeam(t, h, token, "First")
		createTestTeam(t, h, token, "Second")
		opts := g.opts
		opts.ManagedTeamID = team.ID
		if opened, err := New(opts); err == nil {
			opened.Close(context.Background())
			t.Fatal("bound a host with other team state")
		}
		if _, err := os.Stat(filepath.Join(g.dir, "managed-team.json")); !os.IsNotExist(err) {
			t.Fatal("ambiguous host created a binding")
		}
	})
	for _, bad := range []string{`{`, `{}`, `{"version":1,"teamId":"team-aaaaaaaaaaaaaaaaaaaaaa"}`, strings.Repeat("x", 4097)} {
		t.Run(bad[:1], func(t *testing.T) {
			g, h, token, _ := fixture(t, nil)
			createTestTeam(t, h, token, "Team")
			if err := os.WriteFile(filepath.Join(g.dir, "managed-team.json"), []byte(bad), 0600); err != nil {
				t.Fatal(err)
			}
			if opened, err := New(g.opts); err == nil {
				opened.Close(context.Background())
				t.Fatal("invalid binding silently became unrestricted")
			}
		})
	}
	t.Run("nonregular binding", func(t *testing.T) {
		g, _, _, _ := fixture(t, nil)
		if err := os.Mkdir(filepath.Join(g.dir, "managed-team.json"), 0700); err != nil {
			t.Fatal(err)
		}
		if opened, err := New(g.opts); err == nil {
			opened.Close(context.Background())
			t.Fatal("nonregular binding ignored")
		}
	})
}

func TestManagedTeamRoutesCannotEscapeThroughPersonalWorkspacesOrNewTeams(t *testing.T) {
	var personal, shared atomic.Int32
	g, h, token, owner := fixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { personal.Add(1); jsonReply(w, 200, map[string]any{}) }))
	outsider := addUser(t, h, token, "no-team-access", "member")
	outsiderToken := loginAs(t, h, outsider.Username)
	team := createTestTeam(t, h, token, "Only team")
	opts := g.opts
	opts.ManagedTeamID = team.ID
	opts.TeamHandler = func(ctx context.Context, teamID string) (http.Handler, error) {
		if teamID != team.ID {
			t.Error("foreign team started")
		}
		shared.Add(1)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/v1/projects" {
				jsonReply(w, 200, map[string]any{"projects": []map[string]string{{"id": "p-owned"}}})
				return
			}
			jsonReply(w, 200, map[string]string{"teamId": TeamRuntimeIDFromContext(r.Context())})
		}), nil
	}
	bound, err := New(opts)
	if err != nil {
		t.Fatal(err)
	}
	defer bound.Close(context.Background())
	h = bound.Handler()
	for _, actor := range []string{token, outsiderToken} {
		for _, endpoint := range []string{"GET /v1/projects", "GET /api/v1/projects", "HEAD /v1/setup/status", "POST /v1/product/projects", "POST /v1/sessions", "GET /v1/sessions/old/attach", "POST /v1/setup/agent-connections", "POST /v1/teams"} {
			method, path, _ := strings.Cut(endpoint, " ")
			request(t, h, method, path, actor, map[string]string{"name": "Bypass", "harness": "shell"}, 403)
		}
		request(t, h, "POST", "/v1/auth/socket-ticket", actor, map[string]string{"path": "/v1/sessions/personal/attach"}, 403)
	}
	if personal.Load() != 0 || shared.Load() != 0 || len(bound.runtimes) != 0 || len(bound.tickets) != 0 {
		t.Fatal("personal bypass reached execution or reserved a terminal")
	}
	for _, actor := range []string{token, outsiderToken} {
		var view struct {
			Teams         []json.RawMessage `json:"teams"`
			ManagedTeamID string            `json:"managedTeamId"`
		}
		response := request(t, h, "GET", "/v1/teams", actor, nil, 200)
		if json.Unmarshal(response.Body.Bytes(), &view) != nil || view.ManagedTeamID != team.ID {
			t.Fatal("managed selector omitted on authenticated team list")
		}
		if actor == outsiderToken && len(view.Teams) != 0 {
			t.Fatal("binding granted an outsider team membership")
		}
	}
	request(t, h, "GET", "/v1/admin/users", token, nil, 200)
	request(t, h, "GET", "/v1/admin/users/"+owner.ID+"/runtime", token, nil, 200)
	request(t, h, "GET", teamWorkspacePrefix(team.ID)+"/v1/projects", outsiderToken, nil, 403)
	request(t, h, "GET", teamWorkspacePrefix(team.ID)+"/v1/projects", token, nil, 200)
	if shared.Load() == 0 || personal.Load() != 0 {
		t.Fatal("bound team was not routed to its own worker")
	}
	request(t, h, "PUT", "/v1/admin/team-services/"+team.ID, token, serviceBody(0, "managed", "suspended"), 200)
	request(t, h, "POST", teamWorkspacePrefix(team.ID)+"/v1/product/projects/p-owned/checks", token, map[string]any{}, 403)
	request(t, h, "GET", teamWorkspacePrefix(team.ID)+"/v1/product/projects/p-owned/brief", token, nil, 200)
	if _, err := bound.executionHandler(context.Background(), owner, bound.state.Policy); err == nil {
		t.Fatal("direct personal handler bypassed binding")
	}
	if _, err := bound.ensureRuntime(context.Background(), outsider, bound.state.Policy); err == nil {
		t.Fatal("direct personal worker bypassed binding")
	}
}

func TestManagedTeamBindingNeverLeaksIntoChildConfiguration(t *testing.T) {
	environment := isolatedEnvironment([]string{"PATH=/bin", "LUCI_MANAGED_TEAM_ID=team-aaaaaaaaaaaaaaaaaaaaaa"}, t.TempDir(), strings.Repeat("x", 44), "/tmp/address", User{ID: "u", Role: "member"})
	for _, entry := range environment {
		if strings.HasPrefix(entry, "LUCI_MANAGED_TEAM_ID=") {
			t.Fatal("controller-only binding leaked into child startup")
		}
	}
}
