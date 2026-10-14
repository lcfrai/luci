package management

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSetupChoiceRetainsAuthenticationOwnerAndTeamAuthority(t *testing.T) {
	called := 0
	g, h, token, owner := fixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		jsonReply(w, 200, map[string]any{"revision": 2, "mode": "local"})
	}))
	request(t, h, "PUT", "/v1/setup/choice", "", map[string]any{"expectedRevision": 1, "mode": "local"}, 401)
	request(t, h, "PUT", "/v1/setup/choice", token, map[string]any{"expectedRevision": 1, "mode": "local"}, 200)
	for _, team := range []bool{false, true} {
		actor := owner
		actor.Role = "admin"
		authority := ExecutionAuthority{Actor: actor, Policy: g.state.Policy}
		if team {
			authority.Team = &TeamAccess{Team: Team{ID: "team-owned"}, Permissions: TeamPermissions{Admin: true, Read: true, Edit: true}}
		}
		r := httptest.NewRequest("PUT", "/v1/setup/choice", strings.NewReader(`{"expectedRevision":1,"mode":"luci-cloud"}`)).WithContext(WithExecutionAuthority(context.Background(), authority))
		w := httptest.NewRecorder()
		g.route(w, r)
		if w.Code != 403 {
			t.Fatal("setup authority bypassed", team, w.Code, w.Body.String())
		}
	}
	if called != 1 {
		t.Fatal("forbidden request reached backing", called)
	}
}
