package previewgateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTeamBrowserRegistrationRequiresMembershipController(t *testing.T) {
	root := t.TempDir()
	team := "team-" + strings.Repeat("a", 22)
	state := filepath.Join(root, "control-plane", "teams", team, "runtime", "state", "sessions")
	if err := os.MkdirAll(state, 0700); err != nil {
		t.Fatal(err)
	}
	registry, err := NewRegistry(state, 8731)
	if err != nil {
		t.Fatal(err)
	}
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Fatal("unauthorized team target was reached") }))
	defer up.Close()
	origin, err := registry.Publish(appTarget(up.URL))
	if err != nil {
		t.Fatal(err)
	}
	gateway, err := New(root, 8731, func(context.Context, Target) error { t.Fatal("unauthorized ownership probe"); return nil })
	if err != nil {
		t.Fatal(err)
	}
	if result := request(gateway, "GET", origin, "/", nil); result.Code != 403 {
		t.Fatal("team registration fell through personal capability access", result.Code)
	}
	if _, err := gateway.ResolveTeamTarget("team-"+strings.Repeat("b", 22), "project-1", origin); err == nil {
		t.Fatal("foreign team registration accepted")
	}
	if _, err := gateway.ResolveTeamTarget(team, "project-other", origin); err == nil {
		t.Fatal("foreign project registration accepted")
	}
	target, err := gateway.ResolveTeamTarget(team, "project-1", origin)
	if err != nil || target.TeamID != team {
		t.Fatal("owned team registration unavailable", err)
	}
}

func TestGatewayCookieDomainWhitespaceCannotCrossProjectOrigins(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, domain := range []string{"Domain=.localhost", "Domain = .localhost", "dOmAiN\t=\t.localhost"} {
			w.Header().Add("Set-Cookie", "app=value; "+domain+"; Path=/; HttpOnly")
		}
		w.WriteHeader(200)
	}))
	defer up.Close()
	g, registry := testGateway(t, func(context.Context, Target) error { return nil })
	origin, err := registry.Publish(appTarget(up.URL))
	if err != nil {
		t.Fatal(err)
	}
	w := request(g, "GET", origin, "/", nil)
	if w.Code != 200 || len(w.Result().Cookies()) != 3 {
		t.Fatal("application cookies disappeared")
	}
	for _, cookie := range w.Result().Cookies() {
		if cookie.Domain != "" || !cookie.HttpOnly || cookie.Value != "value" {
			t.Fatal("cookie escaped isolated project origin")
		}
	}
}
