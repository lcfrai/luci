package platform

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

func fixture(t *testing.T, runner Runner) *Service {
	t.Helper()
	dir := t.TempDir()
	projectDir := filepath.Join(dir, "project")
	if e := os.Mkdir(projectDir, 0700); e != nil {
		t.Fatal(e)
	}
	s, e := New(Options{StateDir: dir, InstanceID: "test-instance", MemoryMiB: 512, CPUMillis: 500, ResolveProject: func(id string) (project.Project, error) {
		if id != "owned" {
			return project.Project{}, os.ErrNotExist
		}
		return project.Project{ID: id, Path: projectDir}, nil
	}, Runner: runner, ValidateDeploymentSource: func(_ context.Context, id, sha string) error {
		if id != "owned" || sha != strings.Repeat("a", 40) {
			return errors.New("source changed")
		}
		return nil
	}, ApplicationConfiguration: func(string) (int, map[string]string, error) {
		return 7, map[string]string{"API_TOKEN": "private-application-value", "EMPTY": ""}, nil
	}})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(s.Close)
	return s
}
func request(t *testing.T, s *Service, method, path, role string, input any) *httptest.ResponseRecorder {
	t.Helper()
	var b bytes.Buffer
	if input != nil {
		_ = json.NewEncoder(&b).Encode(input)
	}
	r := httptest.NewRequest(method, path, &b)
	token := strings.Repeat("private-worker-", 4)
	u, _ := json.Marshal(management.User{ID: "test-user", Role: role})
	p, _ := json.Marshal(management.Policy{InstanceName: "test", BackingMode: "docker", MaxSessionsPerUser: 1, MaxConcurrentChecksPerUser: 1, SessionMemoryMiB: 1024, SessionCPUMillis: 1000, CheckMemoryMiB: 1024, CheckCPUMillis: 1000, AllowedHarnesses: []string{"shell"}, AllowCloudProvisioning: true})
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("X-Luci-Actor", base64.RawURLEncoding.EncodeToString(u))
	r.Header.Set("X-Luci-Policy", base64.RawURLEncoding.EncodeToString(p))
	w := httptest.NewRecorder()
	management.WorkerHandler(token, s.Handler()).ServeHTTP(w, r)
	return w
}
func tarSource(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var b bytes.Buffer
	tw := tar.NewWriter(&b)
	for name, content := range files {
		if e := tw.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0600, Size: int64(len(content))}); e != nil {
			t.Fatal(e)
		}
		_, _ = tw.Write([]byte(content))
	}
	_ = tw.Close()
	return b.Bytes()
}
func TestPlatformRequiresActorAndConfinesProjects(t *testing.T) {
	calls := 0
	s := fixture(t, func(context.Context, string, []string, string, []byte) ([]byte, error) {
		calls++
		return []byte("version"), nil
	})
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/v1/platform/status", nil))
	if w.Code != 401 || calls != 0 {
		t.Fatalf("unauthenticated %d calls %d", w.Code, calls)
	}
	w = request(t, s, "GET", "/v1/platform/projects/foreign", "member", nil)
	if w.Code != 404 || calls != 0 {
		t.Fatalf("foreign %d", w.Code)
	}
	w = request(t, s, "GET", "/v1/platform/status", "member", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"memoryMiB":512`) || !strings.Contains(w.Body.String(), `"connectPanacea":false`) {
		t.Fatalf("status %d %s", w.Code, w.Body)
	}
	w = request(t, s, "PUT", "/v1/platform/panacea", "member", map[string]string{"baseUrl": "https://example.test", "apiKey": strings.Repeat("secret", 5)})
	if w.Code != 403 {
		t.Fatalf("member connection %d", w.Code)
	}
}
func TestArchiveUsesExactCommitAndExcludesPrivateFiles(t *testing.T) {
	var captured []string
	s := fixture(t, func(_ context.Context, exe string, args []string, _ string, _ []byte) ([]byte, error) {
		captured = args
		return tarSource(t, map[string]string{"Dockerfile": "FROM scratch", "app.txt": "committed", ".env": "secret", ".luci/state.json": "private", "nested/secret.key": "key"}), nil
	})
	p, _ := s.opts.ResolveProject("owned")
	data, e := s.archive(context.Background(), p, strings.Repeat("a", 40))
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(strings.Join(captured, " "), "archive --format=tar "+strings.Repeat("a", 40)) {
		t.Fatal(captured)
	}
	tr := tar.NewReader(bytes.NewReader(data))
	names := map[string]bool{}
	for {
		h, e := tr.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			t.Fatal(e)
		}
		names[h.Name] = true
	}
	if len(names) != 2 || !names["Dockerfile"] || !names["app.txt"] {
		t.Fatal(names)
	}
	for _, name := range []string{"../escape", "/absolute", "folder\\escape"} {
		s.opts.Runner = func(context.Context, string, []string, string, []byte) ([]byte, error) {
			return tarSource(t, map[string]string{"Dockerfile": "FROM scratch", name: "bad"}), nil
		}
		if _, e = s.archive(context.Background(), p, strings.Repeat("a", 40)); e == nil {
			t.Fatalf("unsafe %s accepted", name)
		}
	}
}
func TestDeploymentPersistsAcrossServiceRestartAndRedactsPinnedConfiguration(t *testing.T) {
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			w.WriteHeader(404)
			return
		}
		w.WriteHeader(204)
	}))
	defer app.Close()
	port := strings.TrimPrefix(app.URL, "http://127.0.0.1:")
	var mu sync.Mutex
	var s *Service
	var runArgs []string
	container, resource := "", ""
	running := true
	envFile := ""
	runner := func(_ context.Context, exe string, args []string, _ string, input []byte) ([]byte, error) {
		mu.Lock()
		defer mu.Unlock()
		if exe == "git" {
			return tarSource(t, map[string]string{"Dockerfile": "FROM scratch", "LUCI.md": "approved"}), nil
		}
		switch args[0] {
		case "build":
			if bytes.Contains(input, []byte("private-application-value")) {
				t.Error("secret in build")
			}
			return []byte("built"), nil
		case "network":
			if args[1] == "inspect" {
				return nil, errors.New("absent")
			}
			return []byte("network"), nil
		case "run":
			runArgs = append([]string{}, args...)
			for i, a := range args {
				if a == "--name" {
					container = args[i+1]
				}
				if a == "--label" && strings.HasPrefix(args[i+1], labelResource+"=") {
					resource = strings.TrimPrefix(args[i+1], labelResource+"=")
				}
				if a == "--env-file" {
					envFile = args[i+1]
				}
			}
			data, e := os.ReadFile(envFile)
			if e != nil || !strings.Contains(string(data), "API_TOKEN=private-application-value\n") || !strings.Contains(string(data), "EMPTY=\n") {
				t.Errorf("runtime file missing %v", e)
			}
			return []byte("container"), nil
		case "inspect":
			v := containerInfo{ID: container, Image: "sha256:immutable-image"}
			v.State.Running = running
			v.Config.Labels = map[string]string{labelInstance: s.instance, labelProject: "owned", labelResource: resource}
			v.NetworkSettings.Ports = map[string][]struct{ HostIP, HostPort string }{"3000/tcp": {{"127.0.0.1", port}}}
			b, _ := json.Marshal([]containerInfo{v})
			return b, nil
		case "logs":
			return []byte("token=private-application-value safe log"), nil
		case "stop":
			running = false
			return nil, nil
		case "restart":
			running = true
			return nil, nil
		}
		return nil, errors.New("unexpected command")
	}
	s = fixture(t, runner)
	w := request(t, s, "POST", "/v1/platform/projects/owned/deployments", "member", map[string]any{"commit": strings.Repeat("a", 40), "port": 3000, "healthPath": "/health"})
	if w.Code != 202 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	var d Deployment
	_ = json.Unmarshal(w.Body.Bytes(), &d)
	s.wg.Wait()
	d, e := s.findDeployment("owned", d.ID)
	if e != nil || d.Status != "ready" || d.URL != app.URL || d.ConfigurationRevision != 7 {
		t.Fatalf("%+v %v", d, e)
	}
	if _, e = os.Stat(envFile); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("temporary environment file retained")
	}
	joined := strings.Join(runArgs, " ")
	for _, expected := range []string{"--restart unless-stopped", "--memory 512m", "--cpus 0.500", "--cap-drop ALL", "--security-opt no-new-privileges", "127.0.0.1::3000"} {
		if !strings.Contains(joined, expected) {
			t.Fatal("missing", expected)
		}
	}
	if strings.Contains(joined, "private-application-value") {
		t.Fatal("secret in argv")
	}
	s.opts.ApplicationConfiguration = func(string) (int, map[string]string, error) {
		return 8, map[string]string{"API_TOKEN": "changed-value"}, nil
	}
	w = request(t, s, "GET", "/v1/platform/projects/owned/deployments/"+d.ID+"/logs", "member", nil)
	if w.Code != 200 || strings.Contains(w.Body.String(), "private-application-value") || !strings.Contains(w.Body.String(), "[redacted]") {
		t.Fatalf("logs %s", w.Body)
	}
	s.Close()
	second, e := New(s.opts)
	if e != nil {
		t.Fatal(e)
	}
	defer second.Close()
	w = request(t, second, "GET", "/v1/platform/projects/owned", "member", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), d.ID) || strings.Contains(w.Body.String(), "private-application-value") {
		t.Fatalf("durable receipt %s", w.Body)
	}
	w = request(t, second, "POST", "/v1/platform/projects/owned/deployments/"+d.ID+"/stop", "member", map[string]any{})
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"status":"stopped"`) {
		t.Fatalf("stop %s", w.Body)
	}
}
func TestForeignContainerCannotBeStopped(t *testing.T) {
	mutated := false
	s := fixture(t, func(_ context.Context, _ string, args []string, _ string, _ []byte) ([]byte, error) {
		if args[0] == "inspect" {
			return []byte(`[{"Config":{"Labels":{"ai.lcfr.luci.platform.instance":"someone-else"}}}]`), nil
		}
		mutated = true
		return nil, nil
	})
	d := Deployment{ID: "deploy-owned", ProjectID: "owned", Provider: "docker", Container: "foreign"}
	_ = s.saveDeployment(d)
	w := request(t, s, "POST", "/v1/platform/projects/owned/deployments/deploy-owned/stop", "member", map[string]any{})
	if w.Code != 409 || mutated {
		t.Fatalf("foreign mutation %d %v", w.Code, mutated)
	}
}
func TestDatabaseBackupIsRealPrivateBoundAndChecksumVerified(t *testing.T) {
	var s *Service
	dump := []byte("PGDMPfixture-bytes")
	s = fixture(t, func(_ context.Context, _ string, args []string, _ string, _ []byte) ([]byte, error) {
		if args[0] == "inspect" {
			v := containerInfo{}
			v.State.Running = true
			v.Config.Labels = map[string]string{labelInstance: s.instance, labelProject: "owned", labelResource: "postgres"}
			b, _ := json.Marshal([]containerInfo{v})
			return b, nil
		}
		if strings.Join(args[2:], " ") != "pg_dump -U luci -d app --format=custom" {
			t.Fatal(args)
		}
		return dump, nil
	})
	w := request(t, s, "POST", "/v1/platform/projects/owned/database", "member", map[string]string{"action": "backup"})
	if w.Code != 201 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	var b Backup
	_ = json.Unmarshal(w.Body.Bytes(), &b)
	w = request(t, s, "GET", "/v1/platform/projects/owned/backups/"+b.ID, "member", nil)
	if w.Code != 200 || !bytes.Equal(w.Body.Bytes(), dump) {
		t.Fatalf("download %d", w.Code)
	}
	w = request(t, s, "GET", "/v1/platform/projects/foreign/backups/"+b.ID, "member", nil)
	if w.Code != 404 {
		t.Fatal("cross-project backup exposed")
	}
	file, _ := s.path(b.ID)
	_ = os.WriteFile(file, []byte("changed"), 0600)
	w = request(t, s, "GET", "/v1/platform/projects/owned/backups/"+b.ID, "member", nil)
	if w.Code != 409 {
		t.Fatal("changed bytes accepted")
	}
}
func TestInvalidSourcesAndHealthPathsNeverStartDocker(t *testing.T) {
	calls := 0
	s := fixture(t, func(context.Context, string, []string, string, []byte) ([]byte, error) { calls++; return nil, nil })
	for _, health := range []string{"http://other/", "//evil/", "/../admin", "/health?secret=1", "/%2fprivate", "/\\evil"} {
		w := request(t, s, "POST", "/v1/platform/projects/owned/deployments", "member", map[string]any{"commit": strings.Repeat("a", 40), "port": 3000, "healthPath": health})
		if w.Code != 400 {
			t.Fatalf("%s %d", health, w.Code)
		}
	}
	w := request(t, s, "POST", "/v1/platform/projects/owned/deployments", "member", map[string]any{"commit": strings.Repeat("b", 40), "port": 3000, "healthPath": "/"})
	if w.Code != 409 || calls != 0 {
		t.Fatalf("invalid source %d calls %d", w.Code, calls)
	}
}
func TestPanaceaRequiresExactProviderCommitLiveReplicaAndHTTPSHealth(t *testing.T) {
	secret := strings.Repeat("scoped-private-key", 2)
	reported := strings.Repeat("b", 40)
	replicas := 0
	var endpoint string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && r.Header.Get("Authorization") != "Bearer "+secret {
			t.Error("provider auth missing")
		}
		switch r.URL.Path {
		case "/api/v1/me":
			io.WriteString(w, `{"data":{"username":"operator"}}`)
		case "/api/v1/projects/demo/deploy":
			var v map[string]string
			_ = json.NewDecoder(r.Body).Decode(&v)
			if v["commit"] != strings.Repeat("a", 40) || v["branch"] != "main" {
				t.Error(v)
			}
			w.WriteHeader(202)
			io.WriteString(w, `{"data":{"project":"demo","status":"running","pipelineRun":"build-123"}}`)
		case "/api/v1/projects/demo/deployments":
			io.WriteString(w, `{"data":[{"id":"build-123","commit":"`+reported+`","status":"Succeeded","liveReadyReplicas":`+strconv.Itoa(replicas)+`}]}`)
		case "/api/v1/projects/demo":
			io.WriteString(w, `{"data":{"name":"demo","url":"`+endpoint+`"}}`)
		case "/":
			if r.Header.Get("Authorization") != "" {
				t.Error("provider credential leaked to app")
			}
			w.WriteHeader(204)
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	endpoint = server.URL
	s := fixture(t, func(context.Context, string, []string, string, []byte) ([]byte, error) {
		return nil, errors.New("unexpected Docker")
	})
	s.opts.HTTPClient = server.Client()
	s.opts.HTTPClient.Timeout = 3 * time.Second
	s.opts.HTTPClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	w := request(t, s, "PUT", "/v1/platform/panacea", "super-admin", map[string]string{"baseUrl": server.URL, "apiKey": secret})
	if w.Code != 200 || strings.Contains(w.Body.String(), secret) {
		t.Fatalf("connection %d %s", w.Code, w.Body)
	}
	w = request(t, s, "PUT", "/v1/platform/projects/owned/panacea", "member", map[string]string{"projectName": "demo"})
	if w.Code != 200 {
		t.Fatal(w.Body)
	}
	w = request(t, s, "POST", "/v1/platform/projects/owned/panacea/deploy", "member", map[string]string{"commit": strings.Repeat("a", 40), "branch": "main"})
	if w.Code != 202 {
		t.Fatalf("deploy %d %s", w.Code, w.Body)
	}
	var d Deployment
	_ = json.Unmarshal(w.Body.Bytes(), &d)
	route := "/v1/platform/projects/owned/panacea/deployments/" + d.ID + "/refresh"
	for _, step := range []int{0, 1, 2} {
		if step > 0 {
			reported = strings.Repeat("a", 40)
		}
		if step == 2 {
			replicas = 1
		}
		w = request(t, s, "POST", route, "member", map[string]any{})
		if w.Code != 200 {
			t.Fatal(w.Body)
		}
		_ = json.Unmarshal(w.Body.Bytes(), &d)
		if step < 2 && d.Status == "ready" {
			t.Fatal("premature ready")
		}
		if step == 2 && (d.Status != "ready" || d.URL != server.URL) {
			t.Fatalf("readiness %+v", d)
		}
	}
}

func TestDatabaseStartsWithOwnedVolumePrivateCredentialsAndReadiness(t *testing.T) {
	var s *Service
	started := false
	envPath := ""
	var runArgs []string
	runner := func(_ context.Context, _ string, args []string, _ string, _ []byte) ([]byte, error) {
		switch args[0] {
		case "inspect":
			if !started {
				return nil, errors.New("not created")
			}
			v := containerInfo{}
			v.State.Running = true
			v.Config.Labels = map[string]string{labelInstance: s.instance, labelProject: "owned", labelResource: "postgres"}
			v.NetworkSettings.Ports = map[string][]struct{ HostIP, HostPort string }{"5432/tcp": {{"127.0.0.1", "54321"}}}
			b, _ := json.Marshal([]containerInfo{v})
			return b, nil
		case "network", "volume":
			if args[1] == "inspect" {
				return nil, errors.New("not created")
			}
			return []byte("created"), nil
		case "run":
			runArgs = append([]string{}, args...)
			for i, a := range args {
				if a == "--env-file" {
					envPath = args[i+1]
				}
			}
			data, e := os.ReadFile(envPath)
			if e != nil || !strings.Contains(string(data), "POSTGRES_PASSWORD=") || !strings.Contains(string(data), "POSTGRES_DB=app") {
				t.Fatalf("private credential file %v", e)
			}
			started = true
			return []byte("container-id"), nil
		case "exec":
			return []byte("1\n"), nil
		default:
			return nil, errors.New("unexpected command")
		}
	}
	s = fixture(t, runner)
	w := request(t, s, "POST", "/v1/platform/projects/owned/database", "member", map[string]string{"action": "start"})
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"status":"ready"`) {
		t.Fatalf("start %d %s", w.Code, w.Body)
	}
	var secret databaseSecret
	if e := s.read("owned-database-secret", &secret); e != nil || secret.Password == "" {
		t.Fatal("credential was not persisted privately")
	}
	if strings.Contains(w.Body.String(), secret.Password) || strings.Contains(strings.Join(runArgs, " "), secret.Password) {
		t.Fatal("database credential exposed")
	}
	if _, e := os.Stat(envPath); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("temporary credential file remains")
	}
	joined := strings.Join(runArgs, " ")
	for _, part := range []string{postgresImage, "--restart unless-stopped", "type=volume,source=", "--network-alias database", "--memory 512m", "127.0.0.1::5432"} {
		if !strings.Contains(joined, part) {
			t.Fatal("missing resource property", part)
		}
	}
	_, values, e := s.environment("owned")
	if e != nil || values["DATABASE_URL"] != "postgresql://luci:"+secret.Password+"@database:5432/app?sslmode=disable" {
		t.Fatal("private application binding missing")
	}
}

func TestPanaceaOriginAndOperationAdmission(t *testing.T) {
	for _, raw := range []string{"http://provider.test", "https://user:secret@provider.test", "https://provider.test/api", "https://provider.test?token=secret", "https://provider.test/#token"} {
		if _, e := connectionURL(raw); e == nil {
			t.Fatalf("unsafe provider origin %q", raw)
		}
	}
	s := fixture(t, func(context.Context, string, []string, string, []byte) ([]byte, error) { return nil, nil })
	if !s.claim("one", 1) || s.claim("two", 1) {
		t.Fatal("concurrent operation limit was not enforced")
	}
	s.release("one")
	if !s.claim("two", 1) {
		t.Fatal("operation slot was not released")
	}
	s.release("two")
}
