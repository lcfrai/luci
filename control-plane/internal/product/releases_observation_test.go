package product

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func observedReleaseHistory(t *testing.T, handler http.Handler, projectID string) []Release {
	t.Helper()
	var view struct {
		Releases []Release `json:"releases"`
	}
	w := journeyRequest(t, handler, "GET", "/v1/product/projects/"+projectID+"/releases", nil, 200)
	if json.Unmarshal(w.Body.Bytes(), &view) != nil {
		t.Fatal("invalid release history")
	}
	return view.Releases
}

func TestLocalReleaseObservationRejectsStalePortAndHealthWithoutRewritingHistory(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "live-preview")
	var appHits, outsideHits atomic.Int32
	var response atomic.Int32
	response.Store(200)
	outside := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { outsideHits.Add(1); w.WriteHeader(200) }))
	defer outside.Close()
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		appHits.Add(1)
		if response.Load() == 302 {
			w.Header().Set("Location", outside.URL)
		}
		w.WriteHeader(int(response.Load()))
	}))
	defer app.Close()
	row := storedRelease{Release: Release{ID: "rel-live", ProjectID: p.ID, Target: "local", Status: "ready", URL: app.URL, Commit: strings.Repeat("a", 40), BriefRevision: 2, ConfigurationRevision: 3}, ContainerID: strings.Repeat("b", 64)}
	if err := s.storeRelease(row); err != nil {
		t.Fatal(err)
	}
	var inspection atomic.Value
	inspection.Store(previewInspection(row))
	s.runner = func(_ context.Context, exe string, args []string, _ string) (string, error) {
		if exe != "docker" || len(args) != 4 || args[0] != "inspect" || args[3] != row.ContainerID {
			t.Error("read replayed or changed a resource operation")
			return "", errors.New("unexpected command")
		}
		return inspection.Load().(string), nil
	}
	beforeState, beforeProject := auditTree(t, s.stateDir), auditTree(t, p.Path)
	read := func(want string, count int) {
		t.Helper()
		history := observedReleaseHistory(t, mux, p.ID)
		if len(history) != 1 || history[0].Status != want || history[0].Commit != row.Commit || history[0].ConfigurationRevision != 3 || history[0].BriefRevision != 2 {
			t.Fatal("observation lost current status or immutable provenance")
		}
		if want != "ready" && history[0].URL != "" {
			t.Fatal("unverified preview retained a clickable URL")
		}
		var catalogue struct {
			Releases []Release `json:"releases"`
		}
		json.Unmarshal(journeyRequest(t, mux, "GET", "/v1/product/projects/"+p.ID+"/prototype-releases", nil, 200).Body.Bytes(), &catalogue)
		if len(catalogue.Releases) != count {
			t.Fatal("prototype catalogue disagrees with native liveness")
		}
		if !reflect.DeepEqual(beforeState, auditTree(t, s.stateDir)) || !reflect.DeepEqual(beforeProject, auditTree(t, p.Path)) {
			t.Fatal("read-only reconciliation changed saved history/source")
		}
	}
	read("ready", 1)
	for _, tc := range []string{"stopped", "foreign", "rebound"} {
		snapshot := previewInspection(row)
		switch tc {
		case "stopped":
			snapshot = strings.Replace(snapshot, `"running":true`, `"running":false`, 1)
		case "foreign":
			snapshot = strings.Replace(snapshot, p.ID, "foreign", 1)
		case "rebound":
			snapshot = strings.Replace(snapshot, `"HostIP":"127.0.0.1"`, `"HostIP":"0.0.0.0"`, 1)
		}
		inspection.Store(snapshot)
		hits := appHits.Load()
		read("unknown", 0)
		if appHits.Load() != hits {
			t.Fatal("failed ownership/binding verification still probed the app")
		}
		plan := ProjectDesignPlan{ProjectID: p.ID, PrototypeReleaseID: row.ID}
		journeyRequest(t, mux, "POST", "/v1/product/projects/"+p.ID+"/design-plan", map[string]any{"plan": plan}, 422)
	}
	inspection.Store(previewInspection(row))
	response.Store(503)
	read("unknown", 0)
	response.Store(302)
	read("unknown", 0)
	if outsideHits.Load() != 0 {
		t.Fatal("health observation followed an external redirect")
	}
	response.Store(200)
	read("ready", 1)
	app.Close()
	read("unknown", 0)
}

func TestLocalReleaseObservationBoundsWholeHistoryAndHonoursCancellation(t *testing.T) {
	s, _ := journeyFixture(t)
	rows := make([]storedRelease, 80)
	for i := range rows {
		rows[i] = storedRelease{Release: Release{ID: "rel-bounded", ProjectID: "p-bounded", Target: "local", Status: "ready", URL: "http://127.0.0.1:31000"}, ContainerID: strings.Repeat("a", 64)}
	}
	var calls atomic.Int32
	s.runner = func(ctx context.Context, _ string, _ []string, _ string) (string, error) {
		calls.Add(1)
		<-ctx.Done()
		return "", ctx.Err()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Millisecond)
	defer cancel()
	start := time.Now()
	views := s.observeLocalReleases(ctx, rows)
	if calls.Load() != 1 || time.Since(start) > time.Second || len(views) != len(rows) {
		t.Fatal("history reset a per-row timeout or continued execution after cancellation")
	}
	for i, row := range views {
		if row.Status != "unknown" || row.URL != "" || rows[i].Status != "ready" {
			t.Fatal("canceled observation fabricated health or modified saved state")
		}
	}
	ctx2, cancel2 := context.WithCancel(context.Background())
	cancel2()
	s.observeLocalReleases(ctx2, rows)
	if calls.Load() != 1 {
		t.Fatal("already canceled read executed Docker")
	}
}

func TestLocalReleaseLiveBuildAndControllerRestartAreDistinguished(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "interrupted-preview")
	if err := os.WriteFile(filepath.Join(p.Path, "Dockerfile"), []byte("FROM scratch\n"), 0600); err != nil {
		t.Fatal(err)
	}
	reviewDeploymentFixture(t, mux, p.ID)
	commitPreviewFixture(t, p.Path)
	entered, finish := make(chan struct{}), make(chan struct{})
	defer func() {
		select {
		case <-finish:
		default:
			close(finish)
		}
	}()
	s.runner = func(ctx context.Context, exe string, args []string, dir string) (string, error) {
		if exe != "docker" {
			return RunCommand(ctx, exe, args, dir)
		}
		if args[0] != "build" {
			t.Error("unexpected resource replay")
			return "", errors.New("unexpected")
		}
		close(entered)
		select {
		case <-finish:
			return "", errors.New("fixture build intentionally failed")
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	body := `{"target":"local","port":3000,"dockerfile":"Dockerfile"}`
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("POST", "/v1/product/projects/"+p.ID+"/releases", strings.NewReader(body)))
		done <- w
	}()
	select {
	case <-entered:
	case <-time.After(15 * time.Second):
		t.Fatal("real preview build did not start")
	}
	views := observedReleaseHistory(t, mux, p.ID)
	if len(views) != 1 || views[0].Status != "building" {
		t.Fatal("live worker build was misclassified as interrupted")
	}
	s.mu.RLock()
	saved, err := s.readReleases(p.ID)
	s.mu.RUnlock()
	if err != nil || len(saved) != 1 {
		t.Fatal("live build receipt missing")
	}
	restarted, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot(), Runner: func(context.Context, string, []string, string) (string, error) {
		t.Error("interrupted observation replayed a resource")
		return "", errors.New("unexpected")
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	restartedMux := http.NewServeMux()
	restarted.registerProjects(restartedMux)
	before := auditTree(t, s.stateDir)
	afterRestart := observedReleaseHistory(t, restartedMux, p.ID)
	if len(afterRestart) != 1 || afterRestart[0].Status != "unknown" || afterRestart[0].ID != saved[0].ID || afterRestart[0].Commit != saved[0].Commit || !strings.Contains(afterRestart[0].Message, "interrupted") || !reflect.DeepEqual(before, auditTree(t, s.stateDir)) {
		t.Fatal("controller restart did not expose an honest recoverable build status")
	}
	close(finish)
	select {
	case w := <-done:
		if w.Code != 422 {
			t.Fatal("fixture build failure was not retained")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("owned build did not finish")
	}
	s.mu.RLock()
	active := len(s.activeLocalReleases)
	s.mu.RUnlock()
	if active != 0 || observedReleaseHistory(t, mux, p.ID)[0].Status != "failed" {
		t.Fatal("completed worker stayed active or failure lost")
	}
}
