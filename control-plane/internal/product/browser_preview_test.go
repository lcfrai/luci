package product

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/previewgateway"
)

func TestReleaseBrowserMappingRetainsOwnedDockerReceipt(t *testing.T) {
	s, _ := journeyFixture(t)
	registry, err := previewgateway.NewRegistry(s.stateDir, 8731)
	if err != nil {
		t.Fatal(err)
	}
	s.browserRegistry = registry
	row := storedRelease{Release: Release{ID: "release-1", ProjectID: "project-1", Target: "local", Status: "ready", URL: "http://127.0.0.1:35432"}, ContainerID: strings.Repeat("a", 64)}
	if err := s.storeRelease(row); err != nil {
		t.Fatal(err)
	}
	view := s.browserRelease(row)
	if !strings.Contains(view.URL, ".localhost:8731") {
		t.Fatal("ready Docker release not reachable through browser gateway")
	}
	rows, err := s.readReleases(row.ProjectID)
	if err != nil || len(rows) != 1 || rows[0].URL != row.URL {
		t.Fatal("private owned release receipt overwritten with browser URL")
	}
	row.Status = "stopped"
	row.URL = ""
	if s.browserRelease(row).URL != "" {
		t.Fatal("stopped release advertised as available")
	}
}
func TestPreviewBridgePreservesAppAuthorizationWithoutControlBearer(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerPreviewProxy(mux)
	p := createJourneyProject(t, mux, "authorization-app")
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer app-user-token" || r.Header.Get("X-Luci-Preview-App-Authorization") != "" {
			t.Error("app auth lost or private bridge header leaked")
		}
		w.WriteHeader(200)
	}))
	defer app.Close()
	seedVerifiedPreview(t, s, Release{ID: "release-auth", ProjectID: p.ID, Target: "local", Status: "ready", URL: app.URL})
	r := httptest.NewRequest("POST", "/v1/product/projects/"+p.ID+"/releases/release-auth/preview/login", strings.NewReader("{}"))
	r.Header.Set("Authorization", "Bearer luci-management-bearer")
	r.Header.Set("X-Luci-Preview-App-Authorization", "Bearer app-user-token")
	w := httptest.NewRecorder()
	testPreviewBridge(mux).ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal("authenticated application request unavailable")
	}
}

func TestPublicPreviewReprojectionUsesCurrentConfiguredOrigin(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "public-preview")
	registry, err := previewgateway.NewRegistryWithDomain(s.stateDir, 8731, "preview.projects.test")
	if err != nil {
		t.Fatal(err)
	}
	s.browserRegistry = registry
	row := storedRelease{Release: Release{ID: "release-public", ProjectID: p.ID, Target: "local", Status: "ready", URL: "http://127.0.0.1:45213"}, ContainerID: "owned-container"}
	public := s.browserRelease(row)
	if !strings.HasPrefix(public.URL, "https://p-") || !strings.HasSuffix(public.URL, ".preview.projects.test") || !validPrototypeURL(public, registry) {
		t.Fatal("owned public preview disappeared from prototype catalogue", public)
	}
	if validPrototypeURL(public) {
		t.Fatal("public local URL accepted without configured gateway")
	}
	foreign := public
	foreign.URL = strings.Replace(public.URL, "preview.projects.test", "attacker.example.test", 1)
	if validPrototypeURL(foreign, registry) {
		t.Fatal("arbitrary public origin accepted as local preview")
	}
	if row.URL != "http://127.0.0.1:45213" {
		t.Fatal("private receipt rewritten")
	}
}
