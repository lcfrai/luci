package product

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

func commitPreviewFixture(t *testing.T, dir string, tracePaths ...string) {
	t.Helper()
	if err := validateOwnedFixtureWorktree(t, dir); err != nil {
		t.Fatalf("preview fixture worktree is not owned: %v", err)
	}
	root, err := ownedFixtureRoot(t, dir)
	if err != nil {
		t.Fatal(err)
	}
	env := append(project.ManagedGitEnvironment(os.Environ()), "GIT_CEILING_DIRECTORIES="+root)
	if len(tracePaths) > 1 {
		t.Fatal("preview fixture accepts at most one explicit trace path")
	}
	if len(tracePaths) == 1 {
		trace := tracePaths[0]
		if !filepath.IsAbs(trace) || filepath.Clean(trace) != trace {
			t.Fatal("preview fixture trace needs an explicit absolute path")
		}
		if err := validateOwnedFixtureDirectory(t, filepath.Dir(trace)); err != nil {
			t.Fatalf("preview fixture trace directory is not owned: %v", err)
		}
		if _, err := os.Lstat(trace); !os.IsNotExist(err) {
			t.Fatal("preview fixture trace must be a new file")
		}
		env = append(env, "GIT_TRACE2_EVENT="+filepath.ToSlash(trace))
	}
	for _, args := range [][]string{{"add", "--all"}, {"-c", "user.name=Luci Test Person", "-c", "user.email=person@example.com", "commit", "-m", "Prepare preview"}} {
		// This native fixture deliberately bypasses the public commit policy to
		// seed adversarial exports/secrets. It must still finish all its writers
		// before deployment starts its strict metadata inspection.
		args = append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)
		if out, err := runCommandEnvironment(context.Background(), "git", args, dir, env); err != nil {
			t.Fatalf("commit fixture: %v: %s", err, out)
		}
	}
}

func TestLocalReleaseBuildsCommittedSafeContextThenVerifiesHealthAndOwnership(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "preview-app")
	base := "/v1/product/projects/" + p.ID
	journeyRequest(t, mux, "POST", base+"/brief", map[string]any{"revision": 0, "purpose": "A local preview", "criteria": []string{"Answers HTTP"}}, 200)
	for name, body := range map[string]string{"Dockerfile": "FROM scratch\n", ".dockerignore": "ignored.txt\n", "ignored.txt": "Docker will ignore this", ".env": "TOKEN=must-not-reach-builder"} {
		if err := os.WriteFile(filepath.Join(p.Path, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	reviewed, _ := reviewDeploymentFixture(t, mux, p.ID)
	commitPreviewFixture(t, p.Path)
	var healthHits atomic.Int32
	health := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		healthHits.Add(1)
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ready"))
	}))
	defer health.Close()
	container := strings.Repeat("a", 64)
	releaseID := ""
	removed := 0
	inspected := 0
	builds := 0
	s.runner = func(ctx context.Context, exe string, args []string, dir string) (string, error) {
		if exe != "docker" {
			return RunCommand(ctx, exe, args, dir)
		}
		switch args[0] {
		case "build":
			builds++
			snapshot := args[len(args)-1]
			if snapshot == p.Path {
				t.Fatal("build used live working tree instead of committed snapshot")
			}
			for _, name := range []string{"Dockerfile", "LUCI.md", ".dockerignore"} {
				if _, err := os.Stat(filepath.Join(snapshot, name)); err != nil {
					t.Fatalf("snapshot missing %s: %v", name, err)
				}
			}
			for _, name := range []string{".env", ".git", ".luci"} {
				if _, err := os.Stat(filepath.Join(snapshot, name)); !os.IsNotExist(err) {
					t.Fatalf("snapshot exposed %s: %v", name, err)
				}
			}
			return "built", nil
		case "run":
			joined := strings.Join(args, " ")
			if strings.Contains(joined, "--env") {
				t.Fatal("empty application configuration changed the container environment arguments")
			}
			if !strings.Contains(joined, "127.0.0.1::3000") || strings.Contains(joined, "--volume") {
				t.Fatalf("unsafe container arguments: %v", args)
			}
			for _, arg := range args {
				if strings.HasPrefix(arg, "luci.release-id=") {
					releaseID = strings.TrimPrefix(arg, "luci.release-id=")
				}
			}
			return container + "\n", nil
		case "port":
			return strings.TrimPrefix(health.URL, "http://") + "\n", nil
		case "inspect":
			inspected++
			if args[2] != "{{json .Config.Labels}}" {
				return previewInspection(storedRelease{Release: Release{ID: releaseID, ProjectID: p.ID, URL: health.URL}}), nil
			}
			data, _ := json.Marshal(map[string]string{"luci.preview": "true", "luci.release-id": releaseID, "luci.project-id": p.ID})
			return string(data), nil
		case "rm":
			removed++
			if args[len(args)-1] != container {
				t.Fatalf("removing another container: %v", args)
			}
			return container, nil
		default:
			t.Fatalf("unexpected Docker operation %v", args)
			return "", nil
		}
	}
	w := journeyRequest(t, mux, "POST", base+"/releases", map[string]any{"target": "local", "port": 3000, "dockerfile": "Dockerfile"}, 201)
	var release Release
	_ = json.Unmarshal(w.Body.Bytes(), &release)
	if release.Status != "ready" || release.BriefRevision != reviewed.BriefRevision || release.Commit == "" || release.URL != health.URL || healthHits.Load() == 0 || builds != 1 {
		t.Fatalf("release lacks real health/provenance: %#v, checks=%d, builds=%d", release, healthHits.Load(), builds)
	}
	stored, err := s.readReleases(p.ID)
	if err != nil || len(stored) != 1 || stored[0].ContainerID != container {
		t.Fatalf("container not recorded durably: %#v %v", stored, err)
	}
	journeyRequest(t, mux, "GET", base+"/releases", nil, 200)
	stop := base + "/releases/" + release.ID + "/stop"
	journeyRequest(t, mux, "POST", stop, nil, 200)
	journeyRequest(t, mux, "POST", stop, nil, 200)
	if removed != 1 || inspected != 2 {
		t.Fatalf("stop was not verified and idempotent: removed=%d inspected=%d", removed, inspected)
	}
	restored, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot()})
	if err != nil {
		t.Fatal(err)
	}
	rows, err := restored.readReleases(p.ID)
	if err != nil || len(rows) != 1 || rows[0].Status != "stopped" || rows[0].BriefRevision != reviewed.BriefRevision {
		t.Fatalf("restart lost release provenance: %#v %v", rows, err)
	}
}

func TestLocalReleaseRefusesDirtyBuildAndRecordsFailuresWithoutClaimingReady(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "failed-preview")
	base := "/v1/product/projects/" + p.ID
	_ = os.WriteFile(filepath.Join(p.Path, "Dockerfile"), []byte("FROM scratch\n"), 0600)
	body := map[string]any{"target": "local", "port": 3000, "dockerfile": "Dockerfile"}
	journeyRequest(t, mux, "POST", base+"/releases", body, 409)
	reviewDeploymentFixture(t, mux, p.ID)
	commitPreviewFixture(t, p.Path)
	runs := 0
	s.runner = func(ctx context.Context, exe string, args []string, dir string) (string, error) {
		if exe != "docker" {
			return RunCommand(ctx, exe, args, dir)
		}
		if args[0] == "run" {
			runs++
		}
		return "", errors.New("Docker build failed: deliberate test failure")
	}
	w := journeyRequest(t, mux, "POST", base+"/releases", body, 422)
	var release Release
	_ = json.Unmarshal(w.Body.Bytes(), &release)
	if release.Status != "failed" || release.URL != "" || !strings.Contains(release.Message, "deliberate test failure") || runs != 0 {
		t.Fatalf("build failure misreported: %#v runs=%d", release, runs)
	}
	rows, err := s.readReleases(p.ID)
	if err != nil || len(rows) != 1 || rows[0].Status != "failed" {
		t.Fatalf("failure not durable: %#v %v", rows, err)
	}
}

func TestStopReleaseRefusesUnrelatedContainer(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "owned-preview")
	row := storedRelease{Release: Release{ID: "rel-test", ProjectID: p.ID, Target: "local", Status: "ready"}, ContainerID: strings.Repeat("b", 64)}
	if err := s.storeRelease(row); err != nil {
		t.Fatal(err)
	}
	s.runner = func(_ context.Context, exe string, args []string, _ string) (string, error) {
		if exe != "docker" || args[0] != "inspect" {
			t.Fatalf("unexpected mutation: %s %v", exe, args)
		}
		return `{"luci.preview":"true","luci.release-id":"another-release","luci.project-id":"another-project"}`, nil
	}
	journeyRequest(t, mux, "POST", "/v1/product/projects/"+p.ID+"/releases/rel-test/stop", nil, 409)
	rows, _ := s.readReleases(p.ID)
	if rows[0].Status != "ready" {
		t.Fatal("ownership refusal changed the stored release")
	}
}

func TestPreviewHealthDoesNotFollowRedirectsOrClaimFailedHTTPIsReady(t *testing.T) {
	outsideHits := 0
	outside := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { outsideHits++; w.WriteHeader(200) }))
	defer outside.Close()
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, outside.URL, 302) }))
	defer local.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	if err := waitPreviewHealthy(ctx, local.URL); err == nil {
		t.Fatal("redirect was incorrectly accepted as a healthy preview")
	}
	if outsideHits != 0 {
		t.Fatal("health check followed a redirect outside the preview")
	}
	for _, mapping := range []string{"0.0.0.0:1234", "127.0.0.1:0", "127.0.0.1:1234\n0.0.0.0:1234", "example.com:1234"} {
		if _, err := localPreviewURL(mapping); err == nil {
			t.Fatalf("unsafe port mapping accepted: %q", mapping)
		}
	}
}

func TestPreviewArchiveRejectsTraversalLinksAndOversizedEntries(t *testing.T) {
	for _, header := range []*tar.Header{{Name: "../escape", Mode: 0600, Size: 0, Typeflag: tar.TypeReg}, {Name: "link", Linkname: "/etc/passwd", Typeflag: tar.TypeSymlink}, {Name: "huge", Mode: 0600, Size: 101 << 20, Typeflag: tar.TypeReg}} {
		t.Run(header.Name, func(t *testing.T) {
			var data bytes.Buffer
			writer := tar.NewWriter(&data)
			if err := writer.WriteHeader(header); err != nil {
				t.Fatal(err)
			}
			_ = writer.Close()
			root := t.TempDir()
			if err := os.Chmod(root, 0700); err != nil {
				t.Fatal(err)
			}
			if err := extractPreviewArchive(&data, root); err == nil {
				t.Fatalf("unsafe archive entry accepted: %#v", header)
			}
		})
	}
}
