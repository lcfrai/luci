package product

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func workspaceTunnel(s *Server, remote string) *Tunnel {
	key := []byte("\x00\x00\x00\x0bssh-ed25519\x00\x00\x00\x20" + strings.Repeat("\x01", 32))
	tunnel := &Tunnel{ID: "tunnel-fixture", Host: "203.0.113.7", APIURL: remote, LocalURL: "http://127.0.0.1:53232", Status: "connected", KnownHostsPath: filepath.Join(s.stateDir, "fixture.known_hosts"), KnownHost: "203.0.113.7 ssh-ed25519 " + base64.StdEncoding.EncodeToString(key), Request: SSHRequest{Host: "203.0.113.7", Port: 22, User: "ubuntu"}}
	s.mu.Lock()
	s.tunnels[tunnel.ID] = tunnel
	s.mu.Unlock()
	return tunnel
}

func TestWorkspaceActivationRequiresMatchingRemoteAndPersistsPinnedSelection(t *testing.T) {
	var compatible atomic.Bool
	var probes atomic.Int32
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		probes.Add(1)
		if r.URL.Path != "/v1/setup/status" || r.Method != "GET" {
			t.Errorf("unexpected readiness request %s %s", r.Method, r.URL.Path)
		}
		version := "0.2.0"
		if compatible.Load() {
			version = Version
		}
		writeJSON(w, 200, map[string]string{"product": "Luci", "version": version})
	}))
	defer remote.Close()
	s, mux := journeyFixture(t)
	s.registerWorkspace(mux)
	mux.HandleFunc("GET /v1/setup/status", s.handleStatus)
	tunnel := workspaceTunnel(s, remote.URL)
	journeyRequest(t, mux, "GET", "/v1/setup/status", nil, 200)
	journeyRequest(t, mux, "GET", "/v1/setup/workspace/install", nil, 200)
	if probes.Load() != 0 {
		t.Fatal("cached setup reads probed a remote workspace")
	}
	journeyRequest(t, mux, "POST", "/v1/setup/workspace/activate", map[string]string{"connectionId": "missing"}, 409)
	journeyRequest(t, mux, "POST", "/v1/setup/workspace/activate", map[string]string{"connectionId": tunnel.ID}, 409)
	if s.workspace.Mode != "local" {
		t.Fatal("incompatible remote became active")
	}
	compatible.Store(true)
	w := journeyRequest(t, mux, "POST", "/v1/setup/workspace/activate", map[string]string{"connectionId": tunnel.ID}, 200)
	var status Status
	_ = json.Unmarshal(w.Body.Bytes(), &status)
	if status.Workspace.Mode != "cloud" || !status.Workspace.Ready || status.Workspace.ConnectionID != tunnel.ID {
		t.Fatalf("activated workspace missing from setup: %#v", status.Workspace)
	}
	data, err := os.ReadFile(filepath.Join(s.stateDir, "workspace.json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved savedWorkspace
	if err = json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.KnownHost != tunnel.KnownHost || saved.Request != tunnel.Request {
		t.Fatal("workspace persistence lost the verified SSH host pin")
	}
	// Returning local must survive restart without opening an SSH process.
	journeyRequest(t, mux, "POST", "/v1/setup/workspace/local", map[string]any{}, 200)
	restarted, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot(), Runner: func(context.Context, string, []string, string) (string, error) {
		t.Error("local restart ran a command")
		return "", errors.New("unexpected command")
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	if restarted.workspace.Mode != "local" || !restarted.workspace.Ready {
		t.Fatal("local workspace choice did not survive restart")
	}
}

func TestWorkspaceReadinessRefusesRedirectAndNonLuciService(t *testing.T) {
	var destinationCalls atomic.Int32
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		destinationCalls.Add(1)
		writeJSON(w, 200, Status{Product: "Luci", Version: Version})
	}))
	defer destination.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, destination.URL, 302) }))
	defer redirect.Close()
	if err := workspaceReady(context.Background(), redirect.URL); err == nil || destinationCalls.Load() != 0 {
		t.Fatal("readiness accepted or followed a redirect")
	}
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"product": "Another app", "version": Version})
	}))
	defer other.Close()
	if workspaceReady(context.Background(), other.URL) == nil {
		t.Fatal("a different localhost service was accepted as Luci")
	}
}

func TestCloudWorkspaceUsesRemoteSetupAndKeepsCloudAccountsOnTheHost(t *testing.T) {
	var remoteChecks atomic.Int32
	var remoteConfigPosts atomic.Int32
	var remoteMu sync.Mutex
	remoteConfig := Config{ProjectsDir: "/home/ubuntu/projects", Mode: "local", MonthlyBudgetUSD: 30}
	remoteStatus := func() Status {
		return Status{Product: "Luci", Version: Version, Configured: true, Config: remoteConfig, Checks: []Check{{ID: "git", Status: "ready", Message: "remote Git"}, {ID: "docker", Status: "ready", Message: "remote Docker"}, {ID: "sandbox-image", Status: "ready", Message: "remote image"}}, CloudProviders: []Provider{}}
	}
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		remoteMu.Lock()
		defer remoteMu.Unlock()
		switch r.Method + " " + r.URL.Path {
		case "GET /v1/setup/status":
			writeJSON(w, 200, remoteStatus())
		case "POST /v1/setup/check":
			remoteChecks.Add(1)
			writeJSON(w, 200, remoteStatus())
		case "POST /v1/setup/config":
			if err := json.NewDecoder(r.Body).Decode(&remoteConfig); err != nil {
				t.Error(err)
			}
			remoteConfigPosts.Add(1)
			writeJSON(w, 200, remoteStatus())
		default:
			http.NotFound(w, r)
		}
	}))
	defer remote.Close()
	s, mux := journeyFixture(t)
	mux.HandleFunc("GET /v1/setup/status", s.handleStatus)
	mux.HandleFunc("POST /v1/setup/check", s.handleCheck)
	mux.HandleFunc("POST /v1/setup/config", s.handleConfig)
	tunnel := workspaceTunnel(s, remote.URL)
	s.workspace = Workspace{Mode: "cloud", ConnectionID: tunnel.ID, Ready: true}
	s.checks = []Check{{ID: "git", Status: "missing"}, {ID: "docker", Status: "missing"}, {ID: "aws", Status: "ready", Message: "host AWS account"}, {ID: "azure", Status: "ready", Message: "host Azure account"}}
	localConfig := s.Config()
	s.runner = func(_ context.Context, exe string, _ []string, _ string) (string, error) {
		if exe != "aws" && exe != "az" {
			t.Errorf("remote readiness checked laptop prerequisite %q", exe)
			return "", errors.New("laptop Docker absent")
		}
		return "host account", nil
	}
	handler := s.ProxyWorkspace(mux)
	w := journeyRequest(t, handler, "GET", "/v1/setup/status", nil, 200)
	var status Status
	_ = json.Unmarshal(w.Body.Bytes(), &status)
	if !status.Configured || status.Config.ProjectsDir != "/home/ubuntu/projects" || status.Workspace.Mode != "cloud" || len(status.CloudProviders) != 2 {
		t.Fatalf("cloud status did not combine remote setup and local controls: %#v", status)
	}
	if s.Config() != localConfig {
		t.Fatal("reading remote status changed local configuration")
	}
	w = journeyRequest(t, handler, "POST", "/v1/setup/check", map[string]any{}, 200)
	_ = json.Unmarshal(w.Body.Bytes(), &status)
	if remoteChecks.Load() != 1 {
		t.Fatal("readiness did not check the remote workspace")
	}
	checks := map[string]string{}
	for _, check := range status.Checks {
		checks[check.ID] = check.Status
	}
	if checks["docker"] != "ready" || checks["git"] != "ready" || checks["aws"] != "ready" || checks["azure"] != "ready" {
		t.Fatalf("readiness mixed up local and remote prerequisites: %#v", checks)
	}
	remoteMu.Lock()
	changed := remoteConfig
	remoteMu.Unlock()
	changed.ProjectsDir = "/home/ubuntu/another-projects-folder"
	journeyRequest(t, handler, "POST", "/v1/setup/config", changed, 200)
	remoteMu.Lock()
	defer remoteMu.Unlock()
	if remoteConfigPosts.Load() != 1 || remoteConfig.ProjectsDir != changed.ProjectsDir || s.Config() != localConfig {
		t.Fatal("cloud settings did not update only the remote workspace")
	}
}

func TestCloudWorkspaceProxiesProjectRequestsAndKeepsHostSetupLocal(t *testing.T) {
	const management = "http://127.0.0.1:53232"
	var upstreamCalls atomic.Int32
	var requestMu sync.Mutex
	var lastMethod, lastPath, lastQuery, lastHost, postedBase string
	var leakedHeader bool
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestMu.Lock()
		defer requestMu.Unlock()
		upstreamCalls.Add(1)
		lastMethod, lastPath, lastQuery, lastHost = r.Method, r.URL.Path, r.URL.RawQuery, r.Host
		for _, name := range []string{"Origin", "Cookie", "Authorization", "Forwarded", "X-Forwarded-Host"} {
			if r.Header.Get(name) != "" {
				leakedHeader = true
			}
		}
		if r.URL.Path == "/v1/setup/targets" {
			if r.Method == "POST" {
				var body map[string]any
				_ = json.NewDecoder(r.Body).Decode(&body)
				postedBase, _ = body["baseUrl"].(string)
				writeJSON(w, 201, map[string]string{"id": "target-1", "baseUrl": postedBase})
				return
			}
			writeJSON(w, 200, map[string]any{"targets": []map[string]string{{"id": "target-1", "baseUrl": "http://127.0.0.1:8000"}, {"id": "external", "baseUrl": "https://deploy.example.com"}}})
			return
		}
		writeJSON(w, 200, map[string]string{"source": "remote"})
	}))
	defer remote.Close()
	s, _ := journeyFixture(t)
	tunnel := workspaceTunnel(s, remote.URL)
	s.workspace = Workspace{Mode: "cloud", ConnectionID: tunnel.ID, Host: tunnel.Host, Ready: true}
	var localCalls atomic.Int32
	handler := s.ProxyWorkspace(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		localCalls.Add(1)
		writeJSON(w, 200, map[string]string{"source": "local"})
	}))
	r := httptest.NewRequest("GET", "/api/v1/product/projects/p-fixture/brief?revision=4&name=first%20project", nil)
	for _, name := range []string{"Origin", "Cookie", "Authorization", "Forwarded", "X-Forwarded-Host"} {
		r.Header.Set(name, "local-browser-value")
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	requestMu.Lock()
	if w.Code != 200 || !strings.Contains(w.Body.String(), "remote") || lastMethod != "GET" || lastPath != "/v1/product/projects/p-fixture/brief" || lastQuery != "revision=4&name=first%20project" || leakedHeader {
		requestMu.Unlock()
		t.Fatal("project request was not forwarded safely", w.Code, w.Body.String())
	}
	remoteURL, _ := url.Parse(remote.URL)
	if lastHost != remoteURL.Host {
		requestMu.Unlock()
		t.Fatal("proxy retained the local Host header")
	}
	requestMu.Unlock()
	w = journeyRequest(t, handler, "POST", "/v1/setup/targets", map[string]string{"name": "My machine", "baseUrl": management, "token": "explicit-body-token"}, 201)
	requestMu.Lock()
	if postedBase != "http://127.0.0.1:8000" {
		requestMu.Unlock()
		t.Fatal("local SSH management address was not translated for the remote API")
	}
	requestMu.Unlock()
	w = journeyRequest(t, handler, "GET", "/v1/setup/targets", nil, 200)
	var response struct {
		Targets []DeploymentTarget `json:"targets"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &response)
	if len(response.Targets) != 2 || response.Targets[0].BaseURL != management || response.Targets[1].BaseURL != "https://deploy.example.com" {
		t.Fatalf("remote target addresses were not translated for this client: %#v", response.Targets)
	}
	count := upstreamCalls.Load()
	journeyRequest(t, handler, "GET", "/v1/setup/status", nil, 200)
	journeyRequest(t, handler, "GET", "/v1/setup/cloud/jobs", nil, 200)
	if localCalls.Load() != 2 || upstreamCalls.Load() != count {
		t.Fatal("host account/settings controls leaked into remote workspace")
	}
	s.mu.Lock()
	tunnel.Status = "failed"
	s.mu.Unlock()
	journeyRequest(t, handler, "GET", "/v1/projects", nil, 503)
	if localCalls.Load() != 2 {
		t.Fatal("disconnected remote silently fell back to local projects")
	}
	s.mu.Lock()
	s.workspace = Workspace{Mode: "local", Ready: true}
	s.mu.Unlock()
	journeyRequest(t, handler, "GET", "/v1/projects", nil, 200)
	if localCalls.Load() != 3 {
		t.Fatal("returning local did not restore local project routing")
	}
}

func TestCloudWorkspaceProxyCarriesTerminalWebSocketFrames(t *testing.T) {
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/sessions/session-fixture/attach" {
			http.NotFound(w, r)
			return
		}
		connection, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer connection.CloseNow()
		kind, data, err := connection.Read(r.Context())
		if err == nil {
			_ = connection.Write(r.Context(), kind, data)
		}
	}))
	defer remote.Close()
	s, _ := journeyFixture(t)
	tunnel := workspaceTunnel(s, remote.URL)
	s.workspace = Workspace{Mode: "cloud", ConnectionID: tunnel.ID, Ready: true}
	local := httptest.NewServer(s.ProxyWorkspace(http.NotFoundHandler()))
	defer local.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	connection, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(local.URL, "http")+"/api/v1/sessions/session-fixture/attach", &websocket.DialOptions{HTTPHeader: http.Header{"Origin": []string{local.URL}}})
	if err != nil {
		t.Fatal(err)
	}
	defer connection.CloseNow()
	frame := []byte{1, 0, 2, 255, 10}
	if err = connection.Write(ctx, websocket.MessageBinary, frame); err != nil {
		t.Fatal(err)
	}
	kind, echo, err := connection.Read(ctx)
	if err != nil || kind != websocket.MessageBinary || !bytes.Equal(frame, echo) {
		t.Fatalf("PTY frame was not preserved: %v, %v", kind, err)
	}
}

func TestCloudWorkspacePreviewAllocatesOnlyOnPostAndSeparatesAppOrigins(t *testing.T) {
	const projectID = "p-fixture"
	const releasePrefix = "/v1/product/projects/" + projectID + "/releases/"
	var appCalls atomic.Int32
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == strings.TrimSuffix(releasePrefix, "/") {
			writeJSON(w, 200, map[string]any{"releases": []Release{
				{ID: "release-first", ProjectID: projectID, Target: "local", Status: "ready", URL: "http://127.0.0.1:34567"},
				{ID: "release-second", ProjectID: projectID, Target: "local", Status: "ready", URL: "http://127.0.0.1:34568"},
			}})
			return
		}
		if !strings.HasPrefix(r.URL.Path, releasePrefix) || !strings.Contains(r.URL.Path, "/preview/") {
			t.Errorf("preview reached a control route: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		appCalls.Add(1)
		if strings.HasSuffix(r.URL.Path, "/socket") {
			if r.Header.Get("X-Luci-Preview-Bridge") != "1" || r.Header.Get("Authorization") != "" || r.Header.Get("Origin") != "" {
				t.Error("preview WebSocket did not isolate browser credentials")
			}
			connection, err := websocket.Accept(w, r, nil)
			if err != nil {
				return
			}
			defer connection.CloseNow()
			kind, data, err := connection.Read(r.Context())
			if err == nil {
				_ = connection.Write(r.Context(), kind, data)
			}
			return
		}
		body, _ := io.ReadAll(r.Body)
		writeJSON(w, 200, map[string]string{"method": r.Method, "path": r.URL.Path, "query": r.URL.RawQuery, "body": string(body), "cookie": r.Header.Get("Cookie"), "authorization": r.Header.Get("Authorization"), "origin": r.Header.Get("Origin"), "forwarded": r.Header.Get("X-Forwarded-Host"), "marker": r.Header.Get("X-Luci-Preview-Bridge")})
	}))
	defer remote.Close()
	s, mux := journeyFixture(t)
	s.registerWorkspace(mux)
	tunnel := workspaceTunnel(s, remote.URL)
	s.workspace = Workspace{Mode: "cloud", ConnectionID: tunnel.ID, Ready: true}
	handler := s.ProxyWorkspace(mux)
	list := journeyRequest(t, handler, "GET", strings.TrimSuffix(releasePrefix, "/"), nil, 200)
	var history struct {
		Releases []Release `json:"releases"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &history); err != nil {
		t.Fatal(err)
	}
	if len(history.Releases) != 2 || history.Releases[0].URL != "" || len(s.previews) != 0 {
		t.Fatal("reading release history opened a listener or exposed a remote loopback URL")
	}
	journeyRequest(t, handler, "GET", "/v1/setup/workspace/preview", nil, 405)
	if len(s.previews) != 0 {
		t.Fatal("a GET allocated a preview listener")
	}
	journeyRequest(t, handler, "POST", "/v1/setup/workspace/preview", map[string]string{"projectId": projectID, "releaseId": "missing"}, 409)
	open := func(id string) string {
		t.Helper()
		w := journeyRequest(t, handler, "POST", "/v1/setup/workspace/preview", map[string]string{"projectId": projectID, "releaseId": id}, 200)
		var opened struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &opened); err != nil {
			t.Fatal(err)
		}
		return opened.URL
	}
	first, second := open("release-first"), open("release-second")
	if first == second || first == remote.URL || open("release-first") != first || len(s.previews) != 2 {
		t.Fatal("release previews do not have separate reusable origins")
	}
	client := &http.Client{Timeout: 5 * time.Second}
	for _, route := range []struct{ method, path string }{{"GET", "/"}, {"GET", "/assets/app.js?v=2"}, {"POST", "/api/v1/setup/agents/anthropic"}} {
		req, _ := http.NewRequest(route.method, first+route.path, strings.NewReader("app-payload"))
		req.Header.Set("Origin", first)
		req.Header.Set("Cookie", "app-session=fixture")
		req.Header.Set("Authorization", "local-control-credential")
		req.Header.Set("X-Forwarded-Host", "forged-control-host")
		req.Header.Set("X-Luci-Preview-Bridge", "forged-marker")
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		var app map[string]string
		err = json.NewDecoder(res.Body).Decode(&app)
		res.Body.Close()
		parsed, _ := url.Parse(route.path)
		if err != nil || res.StatusCode != 200 || app["method"] != route.method || app["path"] != releasePrefix+"release-first/preview"+parsed.Path || app["query"] != parsed.RawQuery || app["body"] != "app-payload" || app["cookie"] != "app-session=fixture" || app["authorization"] != "" || app["origin"] != "" || app["forwarded"] != "" || app["marker"] != "1" {
			t.Fatalf("isolated preview forwarding failed: %s %s: %#v (%v)", route.method, route.path, app, err)
		}
	}
	for _, rejected := range []string{"origin", "host"} {
		req, _ := http.NewRequest("POST", first+"/change", nil)
		if rejected == "origin" {
			req.Header.Set("Origin", second)
		} else {
			req.Host = "untrusted.example"
		}
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != 403 {
			t.Fatalf("preview accepted another %s", rejected)
		}
	}
	count := appCalls.Load()
	journeyRequest(t, handler, "GET", releasePrefix+"release-first/preview/", nil, 403)
	if appCalls.Load() != count {
		t.Fatal("preview content was served from the control API origin")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	connection, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(first, "http")+"/socket", &websocket.DialOptions{HTTPHeader: http.Header{"Origin": []string{first}, "Authorization": []string{"local-control-credential"}}})
	if err != nil {
		t.Fatal(err)
	}
	frame := []byte{1, 0, 255, 2}
	if err = connection.Write(ctx, websocket.MessageBinary, frame); err != nil {
		connection.CloseNow()
		t.Fatal(err)
	}
	kind, data, err := connection.Read(ctx)
	connection.CloseNow()
	if err != nil || kind != websocket.MessageBinary || !bytes.Equal(data, frame) {
		t.Fatalf("preview WebSocket failed: %v", err)
	}
	journeyRequest(t, handler, "POST", "/v1/setup/workspace/local", map[string]any{}, 200)
	if len(s.previews) != 0 {
		t.Fatal("switching local retained a cloud preview listener")
	}
	if res, err := client.Get(first); err == nil {
		res.Body.Close()
		t.Fatal("old preview origin remained reachable after switching workspace")
	}
}

func cloudWorkspaceBundle(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "luci-cloud-runtime.tar.gz")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(file)
	archive := tar.NewWriter(gz)
	body := []byte("test executable fixture\n")
	if err = archive.WriteHeader(&tar.Header{Name: "luci", Mode: 0755, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err = archive.Write(body); err != nil {
		t.Fatal(err)
	}
	for _, closer := range []io.Closer{archive, gz, file} {
		if err = closer.Close(); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func awaitWorkspaceInstall(t *testing.T, s *Server, id string) WorkspaceInstall {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		s.mu.RLock()
		job := s.workspaceInstalls[id]
		s.mu.RUnlock()
		if job.Status != "installing" {
			return job
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("cloud runtime installation did not finish")
	return WorkspaceInstall{}
}

func TestCloudWorkspaceInstallRequiresAcknowledgementBundleAndVerifiedTransfer(t *testing.T) {
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, Status{Product: "Luci", Version: Version})
	}))
	defer remote.Close()
	s, mux := journeyFixture(t)
	s.registerWorkspace(mux)
	tunnel := workspaceTunnel(s, remote.URL)
	var calls atomic.Int32
	var failInstall atomic.Bool
	var transferArgs, installArgs []string
	s.runner = func(_ context.Context, exe string, args []string, _ string) (string, error) {
		calls.Add(1)
		if exe == "scp" {
			transferArgs = append([]string{}, args...)
			return "", nil
		}
		if exe == "ssh" {
			installArgs = append([]string{}, args...)
			if failInstall.Load() {
				return "", errors.New("fixture sudo refusal")
			}
			return "", nil
		}
		return "", errors.New("unexpected executable")
	}
	path := "/v1/setup/ssh/connections/" + tunnel.ID + "/install-luci"
	journeyRequest(t, mux, "POST", path, map[string]bool{"acknowledgeInstall": false}, 400)
	journeyRequest(t, mux, "POST", path, map[string]bool{"acknowledgeInstall": true}, 503)
	journeyRequest(t, mux, "GET", "/v1/setup/workspace/install", nil, 200)
	if calls.Load() != 0 {
		t.Fatal("read, missing bundle or unacknowledged request launched SSH/SCP")
	}
	s.cloudBundle = cloudWorkspaceBundle(t)
	w := journeyRequest(t, mux, "POST", path, map[string]bool{"acknowledgeInstall": true}, 202)
	var accepted WorkspaceInstall
	_ = json.Unmarshal(w.Body.Bytes(), &accepted)
	job := awaitWorkspaceInstall(t, s, accepted.ID)
	if job.Status != "ready" || calls.Load() != 2 {
		t.Fatalf("verified installation did not become ready: %#v", job)
	}
	for _, args := range [][]string{transferArgs, installArgs} {
		joined := strings.Join(args, "\n")
		if !strings.Contains(joined, "StrictHostKeyChecking=yes") || !strings.Contains(joined, "UserKnownHostsFile="+tunnel.KnownHostsPath) || !strings.Contains(joined, "BatchMode=yes") {
			t.Fatal("installation did not use pinned noninteractive SSH")
		}
	}
	if transferArgs[len(transferArgs)-2] != s.cloudBundle || !strings.HasSuffix(transferArgs[len(transferArgs)-1], "/"+accepted.ID+".tar.gz") {
		t.Fatal("transfer did not use the bundled archive and unique destination")
	}
	bundle, err := os.ReadFile(s.cloudBundle)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(bundle)
	script := installArgs[len(installArgs)-1]
	if !strings.Contains(script, hex.EncodeToString(sum[:])) || !strings.Contains(script, "sha256sum --check") || !strings.Contains(script, "LUCI_ADDR=127.0.0.1:8730") {
		t.Fatal("remote installation omitted integrity verification or loopback binding")
	}
	failInstall.Store(true)
	w = journeyRequest(t, mux, "POST", path, map[string]bool{"acknowledgeInstall": true}, 202)
	_ = json.Unmarshal(w.Body.Bytes(), &accepted)
	if job = awaitWorkspaceInstall(t, s, accepted.ID); job.Status != "failed" {
		t.Fatal("SSH installation failure was labelled successful")
	}
}
