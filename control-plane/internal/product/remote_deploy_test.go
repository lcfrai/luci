package product

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

type cloudProviderFixture struct {
	mu                                           sync.Mutex
	projects, applications, updates, deployments int
	commit, status                               string
	appFailure                                   bool
}

func (f *cloudProviderFixture) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if r.Header.Get("Authorization") != "Bearer "+testCoolifyToken {
			t.Error("missing deployment token")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "GET /api/v1/teams/current":
			_, _ = w.Write([]byte(`{"id":1}`))
		case "GET /api/v1/projects":
			if f.projects == 0 {
				_, _ = w.Write([]byte(`[]`))
			} else {
				_, _ = w.Write([]byte(`[{"uuid":"remote-project","name":"existing"}]`))
			}
		case "POST /api/v1/projects":
			f.projects++
			_, _ = w.Write([]byte(`{"uuid":"remote-project"}`))
		case "POST /api/v1/applications/public", "PATCH /api/v1/applications/remote-app":
			if f.appFailure {
				w.WriteHeader(500)
				_, _ = w.Write([]byte(testCoolifyToken))
				return
			}
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			f.commit, _ = body["git_commit_sha"].(string)
			if r.Method == http.MethodPost {
				f.applications++
			} else {
				f.updates++
			}
			_, _ = w.Write([]byte(`{"uuid":"remote-app"}`))
		case "POST /api/v1/deploy":
			f.deployments++
			_, _ = w.Write([]byte(`{"deployments":[{"resource_uuid":"remote-app","deployment_uuid":"remote-deployment"}]}`))
		case "GET /api/v1/deployments/remote-deployment":
			_ = json.NewEncoder(w).Encode(map[string]string{"deployment_uuid": "remote-deployment", "status": f.status, "commit": f.commit})
		default:
			t.Errorf("unexpected remote operation: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	}
}

func cloudReleaseFixture(t *testing.T, provider *cloudProviderFixture) (*Server, *http.ServeMux, project.Project, CloudReleaseRequest, *httptest.Server) {
	t.Helper()
	s, mux := journeyFixture(t)
	s.registerCloudReleases(mux)
	p := createJourneyProject(t, mux, "cloud-app")
	journeyRequest(t, mux, "POST", "/v1/product/projects/"+p.ID+"/brief", map[string]any{"revision": 0, "purpose": "Public service", "criteria": []string{"HTTPS health succeeds"}}, 200)
	if err := os.WriteFile(filepath.Join(p.Path, "Dockerfile"), []byte("FROM scratch\n"), 0600); err != nil {
		t.Fatal(err)
	}
	commitPreviewFixture(t, p.Path)
	commit, err := s.journeyGit(context.Background(), p.Path, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	remote := httptest.NewServer(provider.handler(t))
	t.Cleanup(remote.Close)
	target := DeploymentTarget{ID: "target-test", Name: "Test", BaseURL: remote.URL, Token: testCoolifyToken, ServerUUID: "server-test"}
	s.mu.Lock()
	s.targets[target.ID] = target
	s.mu.Unlock()
	if err := atomicJSON(filepath.Join(s.stateDir, "deployment-targets.json"), []storedTarget{{Target: target, Token: target.Token}}); err != nil {
		t.Fatal(err)
	}
	req := CloudReleaseRequest{TargetID: target.ID, Repository: "https://github.com/example/project.git", Branch: "main", Commit: strings.TrimSpace(commit), Domain: "https://app.example.com", Port: 3000, HealthPath: "/health", Dockerfile: "Dockerfile"}
	return s, mux, p, req, remote
}

func decodeCloudRelease(t *testing.T, w *httptest.ResponseRecorder) CloudRelease {
	t.Helper()
	var row CloudRelease
	if err := json.Unmarshal(w.Body.Bytes(), &row); err != nil {
		t.Fatal(err)
	}
	return row
}

func TestCloudReleasePinsCommitPersistsAndReusesApplication(t *testing.T) {
	provider := &cloudProviderFixture{status: "in_progress"}
	s, mux, p, req, remote := cloudReleaseFixture(t, provider)
	base := "/v1/product/projects/" + p.ID + "/releases"
	row := decodeCloudRelease(t, journeyRequest(t, mux, "POST", base+"/cloud", req, 202))
	if row.Status != "queued" || row.Release.Commit != req.Commit || row.CloudReleaseRequest.Commit != req.Commit || row.BriefRevision != 1 || row.ApplicationID != "remote-app" || row.DeploymentID != "remote-deployment" || row.URL != "" {
		t.Fatalf("incomplete or premature release: %+v", row)
	}
	duplicate := decodeCloudRelease(t, journeyRequest(t, mux, "POST", base+"/cloud", req, 202))
	if duplicate.ID != row.ID {
		t.Fatal("duplicate release request created another reservation")
	}
	data, err := os.ReadFile(s.cloudReleaseFile(p.ID))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), testCoolifyToken) || strings.Contains(string(data), remote.URL) {
		t.Fatal("connection secret or URL leaked to release history")
	}
	if _, err := os.Stat(filepath.Join(p.Path, "cloud-releases")); !os.IsNotExist(err) {
		t.Fatal("release history was written inside the repository")
	}
	row = decodeCloudRelease(t, journeyRequest(t, mux, "POST", base+"/"+row.ID+"/refresh", nil, 200))
	if row.Status != "deploying" {
		t.Fatalf("provider status not observed: %+v", row)
	}
	restored, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot()})
	if err != nil {
		t.Fatal(err)
	}
	rows, err := restored.cloudReleases(p.ID)
	if err != nil || len(rows) != 1 || rows[0].Commit != req.Commit || rows[0].BriefRevision != 1 {
		t.Fatalf("restart lost provenance: %+v %v", rows, err)
	}
	if err := s.storeRelease(storedRelease{Release: Release{ID: "rel-local-fixture", ProjectID: p.ID, Target: "local", Status: "stopped", Commit: req.Commit}}); err != nil {
		t.Fatal(err)
	}
	listed := journeyRequest(t, mux, "GET", base, nil, 200)
	var combined struct {
		Releases []Release `json:"releases"`
	}
	if err := json.Unmarshal(listed.Body.Bytes(), &combined); err != nil || len(combined.Releases) != 2 {
		t.Fatalf("local/cloud history not merged: %s", listed.Body.String())
	}
	health := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Error("wrong health path")
		}
		w.WriteHeader(204)
	}))
	defer health.Close()
	provider.mu.Lock()
	provider.status = "finished"
	provider.mu.Unlock()
	client, _ := NewCoolifyClient(remote.URL, testCoolifyToken)
	row, err = observeCloudRelease(context.Background(), row, client, func(ctx context.Context, address string) error {
		if address != "https://app.example.com/health" {
			t.Error("health URL lost domain binding")
		}
		return probeReleaseHealth(ctx, health.URL+"/health", health.Client())
	})
	if err != nil || row.Status != "ready" || row.URL != req.Domain {
		t.Fatalf("real TLS success not recorded: %+v %v", row, err)
	}
	if err = s.storeCloudRelease(row); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(p.Path, "app.txt"), []byte("second release\n"), 0600); err != nil {
		t.Fatal(err)
	}
	commitPreviewFixture(t, p.Path)
	commit, _ := s.journeyGit(context.Background(), p.Path, "rev-parse", "HEAD")
	req.Commit = strings.TrimSpace(commit)
	next := decodeCloudRelease(t, journeyRequest(t, mux, "POST", base+"/cloud", req, 202))
	if next.ID == row.ID || next.ApplicationID != row.ApplicationID || next.Release.Commit != req.Commit {
		t.Fatalf("next release did not retain app identity: %+v", next)
	}
	provider.mu.Lock()
	defer provider.mu.Unlock()
	if provider.projects != 1 || provider.applications != 1 || provider.updates != 1 || provider.deployments != 2 {
		t.Fatalf("resource duplication: %+v", provider)
	}
}

func TestCloudReleaseRejectsUncommittedOrMismatchedProvenance(t *testing.T) {
	provider := &cloudProviderFixture{status: "queued"}
	s, mux, p, req, _ := cloudReleaseFixture(t, provider)
	base := "/v1/product/projects/" + p.ID
	bad := req
	bad.Commit = testCoolifyCommit
	journeyRequest(t, mux, "POST", base+"/releases/cloud", bad, 409)
	for _, domain := range []string{"http://app.example.com", "https://127.0.0.1", "https://169.254.169.254", "https://app.example.com/elsewhere"} {
		bad = req
		bad.Domain = domain
		journeyRequest(t, mux, "POST", base+"/releases/cloud", bad, 400)
	}
	journeyRequest(t, mux, "POST", base+"/brief", map[string]any{"revision": 1, "purpose": "Updated, uncommitted brief"}, 200)
	journeyRequest(t, mux, "POST", base+"/releases/cloud", req, 409)
	provider.mu.Lock()
	defer provider.mu.Unlock()
	if provider.projects != 0 || provider.applications != 0 || provider.deployments != 0 {
		t.Fatal("invalid provenance caused cloud mutation")
	}
	rows, err := s.cloudReleases(p.ID)
	if err != nil || len(rows) != 0 {
		t.Fatalf("invalid request created a reservation: %+v %v", rows, err)
	}
}

func TestCloudReleaseKeepsAmbiguousFailureAndDoesNotRetry(t *testing.T) {
	provider := &cloudProviderFixture{status: "queued", appFailure: true}
	s, mux, p, req, _ := cloudReleaseFixture(t, provider)
	base := "/v1/product/projects/" + p.ID + "/releases"
	row := decodeCloudRelease(t, journeyRequest(t, mux, "POST", base+"/cloud", req, 502))
	if row.Status != "attention" || row.ProviderProjectID != "remote-project" || row.URL != "" || strings.Contains(row.Message, testCoolifyToken) {
		t.Fatalf("ambiguous outcome unsafe: %+v", row)
	}
	provider.mu.Lock()
	provider.appFailure = false
	provider.mu.Unlock()
	duplicate := decodeCloudRelease(t, journeyRequest(t, mux, "POST", base+"/cloud", req, 202))
	if duplicate.ID != row.ID || duplicate.Status != "attention" {
		t.Fatal("ambiguous request retried automatically")
	}
	journeyRequest(t, mux, "POST", base+"/"+row.ID+"/refresh", nil, 409)
	rows, err := s.cloudReleases(p.ID)
	if err != nil || len(rows) != 1 {
		t.Fatal("ambiguous release history lost")
	}
	provider.mu.Lock()
	defer provider.mu.Unlock()
	if provider.projects != 1 || provider.applications != 0 || provider.deployments != 0 {
		t.Fatal("ambiguous failure caused duplicate resources")
	}
}

func TestCloudReleaseRequiresMatchingCommitAndSuccessfulTLSHealth(t *testing.T) {
	provider := &cloudProviderFixture{status: "finished", commit: testCoolifyCommit}
	remote := httptest.NewServer(provider.handler(t))
	defer remote.Close()
	client, _ := NewCoolifyClient(remote.URL, testCoolifyToken)
	row := CloudRelease{Release: Release{Commit: testCoolifyCommit}, CloudReleaseRequest: CloudReleaseRequest{Domain: "https://app.example.com", HealthPath: "/health"}, DeploymentID: "remote-deployment"}
	checks := 0
	observed, err := observeCloudRelease(context.Background(), row, client, func(context.Context, string) error { checks++; return errors.New("not ready") })
	if err != nil || observed.Status != "verifying" || observed.URL != "" || checks != 1 {
		t.Fatal("failed health marked ready")
	}
	provider.mu.Lock()
	provider.commit = strings.Repeat("a", 40)
	provider.mu.Unlock()
	observed, err = observeCloudRelease(context.Background(), row, client, func(context.Context, string) error { checks++; return nil })
	if err != nil || observed.Status != "attention" || checks != 1 {
		t.Fatal("wrong provider commit passed health verification")
	}
	redirectHits := 0
	destination := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { redirectHits++; w.WriteHeader(200) }))
	defer destination.Close()
	source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, destination.URL, 302) }))
	defer source.Close()
	if err := probeReleaseHealth(context.Background(), source.URL, source.Client()); err == nil || redirectHits != 0 {
		t.Fatal("health redirect followed or accepted")
	}
	if err := probeReleaseHealth(context.Background(), source.URL, &http.Client{}); err == nil {
		t.Fatal("untrusted TLS certificate accepted")
	}
}

func TestCloudReleaseHealthBlocksPrivateAddressesAndURLOverrides(t *testing.T) {
	for _, path := range []string{"//evil.example.com", "https://evil.example.com", "/health?token=x", "/health#x", "/%2f%2fevil.example.com", "/\\evil.example.com"} {
		if _, err := releaseHealthURL("https://app.example.com", path); err == nil {
			t.Fatalf("unsafe health path accepted: %q", path)
		}
	}
	for _, address := range []string{"127.0.0.1", "::1", "192.168.1.10", "10.0.0.1", "169.254.169.254", "100.100.100.200", "0.0.0.0", "::ffff:127.0.0.1"} {
		if publicReleaseIP(net.ParseIP(address)) {
			t.Fatalf("non-public address accepted: %s", address)
		}
	}
	if !publicReleaseIP(net.ParseIP("1.1.1.1")) {
		t.Fatal("public IP rejected")
	}
	if err := checkPublicReleaseHealth(context.Background(), "https://127.0.0.1/health"); err == nil {
		t.Fatal("private network health request allowed")
	}
}
