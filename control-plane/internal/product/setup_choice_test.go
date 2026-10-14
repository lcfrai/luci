package product

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func setupChoiceFixture(t *testing.T, root string) (*Server, *http.ServeMux) {
	t.Helper()
	s, err := New(Options{StateDir: filepath.Join(root, "state"), ProjectsDir: filepath.Join(root, "projects")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /v1/setup/choice", s.handleSetupChoice)
	mux.HandleFunc("GET /v1/setup/status", s.handleStatus)
	return s, mux
}

func TestSetupChoicePersistsWithoutChangingProjectsConnectionOrLegacyConfig(t *testing.T) {
	root := t.TempDir()
	s, mux := setupChoiceFixture(t, root)
	beforeConfig, beforeWorkspace := s.Config(), s.status().Workspace
	if err := os.WriteFile(filepath.Join(beforeConfig.ProjectsDir, "retained.txt"), []byte("retained project"), 0600); err != nil {
		t.Fatal(err)
	}
	// A project file cannot supply a setup preference.
	if err := os.WriteFile(filepath.Join(beforeConfig.ProjectsDir, "setup-choice.json"), []byte(`{"revision":99,"mode":"luci-cloud"}`), 0600); err != nil {
		t.Fatal(err)
	}
	for i, mode := range []string{"local", "bring-your-own-cloud", "luci-cloud"} {
		journeyRequest(t, mux, "PUT", "/v1/setup/choice", map[string]any{"expectedRevision": i + 1, "mode": mode}, 200)
		if !reflect.DeepEqual(s.Config(), beforeConfig) || !reflect.DeepEqual(s.status().Workspace, beforeWorkspace) || s.configured {
			t.Fatal("preference changed active setup", s.Config(), s.status().Workspace)
		}
	}
	reopened, _ := setupChoiceFixture(t, root)
	if reopened.setupChoice.Mode != "luci-cloud" || reopened.setupChoice.Revision != 4 || !reflect.DeepEqual(reopened.Config(), beforeConfig) || reopened.configured {
		t.Fatal("selection lost or fabricated readiness", reopened.setupChoice)
	}
	if data, err := os.ReadFile(filepath.Join(beforeConfig.ProjectsDir, "retained.txt")); err != nil || string(data) != "retained project" {
		t.Fatal("project lost", err)
	}
}

func TestSetupChoiceCASAndAccountStateIsolation(t *testing.T) {
	a, amux := setupChoiceFixture(t, t.TempDir())
	b, bmux := setupChoiceFixture(t, t.TempDir())
	journeyRequest(t, amux, "PUT", "/v1/setup/choice", map[string]any{"expectedRevision": 1, "mode": "bring-your-own-cloud"}, 200)
	journeyRequest(t, amux, "PUT", "/v1/setup/choice", map[string]any{"expectedRevision": 1, "mode": "local"}, 409)
	journeyRequest(t, bmux, "PUT", "/v1/setup/choice", map[string]any{"expectedRevision": 1, "mode": "luci-cloud"}, 200)
	if a.setupChoice.Mode != "bring-your-own-cloud" || b.setupChoice.Mode != "luci-cloud" {
		t.Fatal("account choices crossed")
	}
	var status Status
	if err := json.Unmarshal(journeyRequest(t, amux, "GET", "/v1/setup/status", nil, 200).Body.Bytes(), &status); err != nil || status.SetupChoice.Mode != "bring-your-own-cloud" || status.SetupChoice.Revision != 2 {
		t.Fatal("status readback lost choice", status.SetupChoice, err)
	}
	journeyRequest(t, amux, "PUT", "/v1/setup/choice", map[string]any{"expectedRevision": 2, "mode": "hosted-connected"}, 400)
	journeyRequest(t, amux, "PUT", "/v1/setup/choice", map[string]any{"expectedRevision": 2, "mode": "local", "stateDir": b.stateDir}, 400)
	if a.setupChoice.Revision != 2 || b.setupChoice.Revision != 2 {
		t.Fatal("invalid input changed a choice")
	}
}
