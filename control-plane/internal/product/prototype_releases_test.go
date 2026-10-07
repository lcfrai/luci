package product

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

func seedPrototypeSources(t *testing.T, s *Server, p project.Project) []Release {
	t.Helper()
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	t.Cleanup(app.Close)
	rows := []Release{
		{ID: "rel-local", ProjectID: p.ID, Commit: strings.Repeat("a", 40), Target: "local", Status: "ready", URL: app.URL, CreatedAt: "2026-10-06T01:00:00Z"},
		{ID: "cloud-rel-owned", ProjectID: p.ID, Commit: strings.Repeat("b", 40), Target: "target-owned", Status: "ready", URL: "https://app.example.com", CreatedAt: "2026-10-06T02:00:00Z"},
		{ID: "pages-owned", ProjectID: p.ID, Commit: strings.Repeat("c", 40), Target: "github-pages", Status: "ready", URL: "https://owner.github.io/project/", CreatedAt: "2026-10-06T03:00:00Z"},
		{ID: "deploy-owned", ProjectID: p.ID, Commit: strings.Repeat("d", 40), Target: "application", Status: "ready", BrowserConnectionRequired: true, CreatedAt: "2026-10-06T04:00:00Z"},
	}
	seedVerifiedPreview(t, s, rows[0])
	if err := atomicJSON(s.cloudReleaseFile(p.ID), []CloudRelease{{Release: rows[1]}}); err != nil {
		t.Fatal(err)
	}
	if err := atomicJSON(s.pagesReleaseFile(p.ID), []pagesRelease{{Release: rows[2], Repository: "owner/project"}}); err != nil {
		t.Fatal(err)
	}
	s.nativePrototypeReleases = func(ctx context.Context, id string) ([]Release, error) {
		// Real integration resolves through product state. A save that holds its
		// journey write lock here would deadlock, as the old local-only flow did.
		resolved, err := s.ResolveProject(id)
		if err != nil || resolved.ID != p.ID {
			return nil, project.ErrUnknownProject
		}
		return []Release{rows[3]}, ctx.Err()
	}
	return rows
}

func TestPrototypeCatalogueAndSaveUseEveryOwnedReleaseSource(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "prototypes")
	rows := seedPrototypeSources(t, s, p)
	base := "/v1/product/projects/" + p.ID
	var catalogue struct {
		Releases []Release `json:"releases"`
	}
	if err := json.Unmarshal(journeyRequest(t, mux, "GET", base+"/prototype-releases", nil, 200).Body.Bytes(), &catalogue); err != nil {
		t.Fatal(err)
	}
	if len(catalogue.Releases) != 4 || catalogue.Releases[0].ID != rows[3].ID || catalogue.Releases[0].URL != "" || !catalogue.Releases[0].BrowserConnectionRequired {
		t.Fatal("catalogue did not safely include the native application")
	}
	var shared struct {
		Releases []Release `json:"releases"`
	}
	json.Unmarshal(journeyRequest(t, mux, "GET", base+"/releases", nil, 200).Body.Bytes(), &shared)
	if len(shared.Releases) != 3 {
		t.Fatal("native prototypes changed shared deployment action routing")
	}
	var plan ProjectDesignPlan
	json.Unmarshal(journeyRequest(t, mux, "GET", base+"/design-plan", nil, 200).Body.Bytes(), &plan)
	for _, row := range rows {
		plan.PrototypeReleaseID = row.ID
		plan.PrototypeCommit = strings.Repeat("f", 40)
		body := map[string]any{"expectedRevision": plan.Revision, "expectedBriefRevision": 0, "expectedSpecificationRevision": 0, "plan": plan}
		request := httptest.NewRequest("POST", base+"/design-plan", strings.NewReader(mustPrototypeJSON(t, body)))
		done := make(chan *httptest.ResponseRecorder, 1)
		go func() { w := httptest.NewRecorder(); mux.ServeHTTP(w, request); done <- w }()
		select {
		case w := <-done:
			if w.Code != 200 {
				t.Fatalf("owned %s save returned %d", row.Target, w.Code)
			}
			if err := json.Unmarshal(w.Body.Bytes(), &plan); err != nil {
				t.Fatal(err)
			}
			if plan.PrototypeCommit != row.Commit || plan.PrototypeReleaseID != row.ID {
				t.Fatal("browser commit replaced admitted release provenance")
			}
		case <-time.After(3 * time.Second):
			t.Fatal("prototype resolver ran under the journey write lock")
		}
	}
}

func mustPrototypeJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestPrototypeURLMatchesCanonicalGatewayProducer(t *testing.T) {
	key := strings.Repeat("a", 32) + "." + strings.Repeat("b", 32)
	for _, raw := range []string{"http://127.0.0.1:31000", "http://localhost:31000", "http://" + key + ".localhost:8743"} {
		if !validPrototypeURL(Release{Target: "local", URL: raw}) {
			t.Fatal("producer browser URL was rejected")
		}
	}
	for _, raw := range []string{"http://" + strings.Repeat("a", 64) + ".localhost:8743", "http://arbitrary.localhost:8743", "http://127.0.0.1", "http://127.0.0.1:0", "http://127.0.0.1:65536", "http://private.example:31000"} {
		if validPrototypeURL(Release{Target: "local", URL: raw}) {
			t.Fatal("unusable/private browser URL was admitted")
		}
	}
}

func TestPrototypeCatalogueRejectsAmbiguousOrInvalidBindings(t *testing.T) {
	for _, reason := range []string{"foreign", "bad-commit", "duplicate", "private-url", "credential-url"} {
		t.Run(reason, func(t *testing.T) {
			s, mux := journeyFixture(t)
			p := createJourneyProject(t, mux, "invalid-prototype")
			row := Release{ID: "deploy-owned", ProjectID: p.ID, Commit: strings.Repeat("a", 40), Target: "application", Status: "ready", BrowserConnectionRequired: true}
			rows := []Release{row}
			switch reason {
			case "foreign":
				rows[0].ProjectID = "different"
			case "bad-commit":
				rows[0].Commit = "short"
			case "duplicate":
				rows = append(rows, row)
			case "private-url":
				rows[0].URL = "http://127.0.0.1:31000"
			case "credential-url":
				rows[0].Target = "target-owned"
				rows[0].URL = "https://user:secret@example.com/"
			}
			s.nativePrototypeReleases = func(context.Context, string) ([]Release, error) { return rows, nil }
			base := "/v1/product/projects/" + p.ID
			journeyRequest(t, mux, "GET", base+"/prototype-releases", nil, 500)
			plan := ProjectDesignPlan{ProjectID: p.ID, PrototypeReleaseID: row.ID}
			journeyRequest(t, mux, "POST", base+"/design-plan", map[string]any{"plan": plan}, 500)
			var saved ProjectDesignPlan
			json.Unmarshal(journeyRequest(t, mux, "GET", base+"/design-plan", nil, 200).Body.Bytes(), &saved)
			if saved.Revision != 0 || saved.PrototypeCommit != "" {
				t.Fatal("invalid catalogue changed saved design")
			}
		})
	}
}

func TestPrototypeSaveRefusesStoppedUnknownAndStaleRevision(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "stopped-prototype")
	base := "/v1/product/projects/" + p.ID
	row := Release{ID: "deploy-owned", ProjectID: p.ID, Commit: strings.Repeat("a", 40), Target: "application", Status: "stopped", BrowserConnectionRequired: true}
	s.nativePrototypeReleases = func(context.Context, string) ([]Release, error) { return []Release{row}, nil }
	for _, id := range []string{row.ID, "does-not-exist"} {
		journeyRequest(t, mux, "POST", base+"/design-plan", map[string]any{"plan": ProjectDesignPlan{ProjectID: p.ID, PrototypeReleaseID: id, PrototypeCommit: strings.Repeat("f", 40)}}, 422)
	}
	s.nativePrototypeReleases = func(context.Context, string) ([]Release, error) {
		s.mu.Lock()
		defer s.mu.Unlock()
		state, err := readJourney(p.Path, p.ID)
		if err != nil {
			return nil, err
		}
		state.DesignPlan = &ProjectDesignPlan{ProjectID: p.ID, Revision: 1, Status: "draft", Screens: []DesignScreen{}}
		if err = writeJourney(p.Path, state); err != nil {
			return nil, err
		}
		row.Status = "ready"
		return []Release{row}, nil
	}
	journeyRequest(t, mux, "POST", base+"/design-plan", map[string]any{"plan": ProjectDesignPlan{ProjectID: p.ID, PrototypeReleaseID: row.ID}}, 409)
}

func TestPrototypeCatalogueSelectedWorkspaceRewritesPrivatePreview(t *testing.T) {
	const route = "/v1/product/projects/p-owned/prototype-releases"
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != route {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, 200, map[string]any{"releases": []Release{{ID: "rel-owned", ProjectID: "p-owned", Commit: strings.Repeat("a", 40), Target: "local", Status: "ready", URL: "http://127.0.0.1:31000"}, {ID: "deploy-owned", ProjectID: "p-owned", Commit: strings.Repeat("b", 40), Target: "application", Status: "ready", BrowserConnectionRequired: true}}})
	}))
	defer remote.Close()
	s, _ := journeyFixture(t)
	tunnel := workspaceTunnel(s, remote.URL)
	s.workspace = Workspace{Mode: "cloud", ConnectionID: tunnel.ID, Ready: true}
	w := journeyRequest(t, s.ProxyWorkspace(http.NotFoundHandler()), "GET", route, nil, 200)
	var v struct {
		Releases []Release `json:"releases"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	if len(v.Releases) != 2 || v.Releases[0].URL != "" || v.Releases[1].URL != "" || !v.Releases[1].BrowserConnectionRequired {
		t.Fatal("selected workspace exposed an unconnected private browser address")
	}
}
