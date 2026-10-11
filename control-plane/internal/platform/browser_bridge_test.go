package platform

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/previewruntime"
)

func bridgePlatformRequest(t *testing.T, s *Service, method, address string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, address, strings.NewReader("app-payload"))
	actor, _ := json.Marshal(management.User{ID: "owned-user", Role: "member"})
	policy, _ := json.Marshal(management.Policy{InstanceName: "test", BackingMode: "docker", MaxSessionsPerUser: 1, MaxConcurrentChecksPerUser: 1, SessionMemoryMiB: 1024, SessionCPUMillis: 1000, CheckMemoryMiB: 1024, CheckCPUMillis: 1000, AllowedHarnesses: []string{"shell"}})
	for key, value := range headers {
		r.Header.Set(key, value)
	}
	r.Header.Set("Authorization", "Bearer "+strings.Repeat("w", 43))
	r.Header.Set("X-Luci-Actor", base64.RawURLEncoding.EncodeToString(actor))
	r.Header.Set("X-Luci-Policy", base64.RawURLEncoding.EncodeToString(policy))
	w := httptest.NewRecorder()
	management.WorkerHandler(strings.Repeat("w", 43), s.Handler()).ServeHTTP(w, r)
	return w
}

func seedBridgeData(t *testing.T, s *Service, f *dataFake, stack, tenant, address string, secret dataSecret) dataInstallation {
	t.Helper()
	u, _ := url.Parse(address)
	port, _ := strconv.Atoi(u.Port())
	i := dataInstallation{ProjectID: "owned", StackID: stack, StorageTenantID: tenant, APIPort: port}
	for name, value := range map[string]any{"owned-data-installation": i, stack + "-secret": secret, "owned-data": DataStatus{ProjectID: "owned", Provider: "supabase", Status: "ready", EndpointScope: "selected-docker-host-loopback", URL: address, PublishableKey: secret.Anon}} {
		if err := s.write(name, value); err != nil {
			t.Fatal(err)
		}
	}
	v := containerInfo{}
	v.State.Running = true
	v.State.Health.Status = "healthy"
	v.Config.Labels = map[string]string{labelInstance: s.instance, labelProject: "owned", labelResource: "data-" + stack + "-api-gw"}
	v.NetworkSettings.Ports = map[string][]struct{ HostIP, HostPort string }{"8000/tcp": {{"127.0.0.1", u.Port()}}}
	f.mu.Lock()
	f.containers[s.dataName(i, "api-gw")] = v
	f.mu.Unlock()
	return i
}

func TestPlatformBrowserBridgeResolvesActiveDataAndRejectsForgedAccess(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		respond(w, 200, map[string]string{"path": r.URL.EscapedPath(), "query": r.URL.RawQuery, "auth": r.Header.Get("Authorization"), "control": r.Header.Get("X-Luci-Actor") + r.Header.Get("X-Luci-Preview-Bridge"), "body": string(body)})
	}))
	defer upstream.Close()
	s, f := newDataFake(t)
	secret := newDataSecret()
	i := seedBridgeData(t, s, f, "stack-first", "durable-data", upstream.URL, secret)
	metadata := request(t, s, "GET", "/v1/platform/projects/owned/browser-target?kind=data", "member", nil)
	if metadata.Code != 200 || !strings.Contains(metadata.Body.String(), `"resourceId":"durable-data"`) || strings.Contains(metadata.Body.String(), upstream.URL) || strings.Contains(metadata.Body.String(), secret.ServiceRole) || strings.Contains(metadata.Body.String(), secret.JWT) {
		t.Fatal("metadata exposed private state or omitted stable identity", metadata.Code)
	}
	base := "/v1/platform/projects/owned/browser/data/durable-data"
	headers := map[string]string{"X-Luci-Preview-Bridge": "1", "X-Luci-Preview-App-Authorization": "Bearer " + secret.Anon, "apikey": secret.Anon, "X-Luci-Forged": "secret", "X-Forwarded-Host": "forged"}
	w := bridgePlatformRequest(t, s, "POST", base+"/storage/v1/object/bucket/a%2Fb?ticket=control-ticket&apikey="+url.QueryEscape(secret.Anon), headers)
	var observed map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &observed)
	if w.Code != 200 || observed["path"] != "/storage/v1/object/bucket/a%2Fb" || strings.Contains(observed["query"], "ticket") || observed["auth"] != "Bearer "+secret.Anon || observed["control"] != "" || observed["body"] != "app-payload" {
		t.Fatalf("isolated client relay failed: %d %#v", w.Code, observed)
	}
	for _, tc := range []struct {
		path    string
		headers map[string]string
		code    int
	}{
		{base + "/rest/v1/items", nil, 403}, {base + "/rest/v1/items", map[string]string{"X-Luci-Preview-Bridge": "1", "Sec-Fetch-Site": "same-origin"}, 403},
		{base + "/pg/query", headers, 404}, {base + "/rest/v1/items", map[string]string{"X-Luci-Preview-Bridge": "1", "X-Luci-Preview-App-Authorization": "Bearer " + secret.ServiceRole}, 401},
		{base + "/realtime/v1/websocket?access_token=" + url.QueryEscape(secret.ServiceRole), headers, 401},
		{"/v1/platform/projects/foreign/browser/data/durable-data/rest/v1/items", headers, 404},
		{"/v1/platform/projects/owned/browser/data/foreign-data/rest/v1/items", headers, 409},
	} {
		if w := bridgePlatformRequest(t, s, "GET", tc.path, tc.headers); w.Code != tc.code {
			t.Fatalf("boundary accepted %s: %d want %d", tc.path, w.Code, tc.code)
		}
	}
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("restored-active")) }))
	defer second.Close()
	seedBridgeData(t, s, f, "stack-restored", i.storageTenant(), second.URL, secret)
	if w := bridgePlatformRequest(t, s, "GET", base+"/rest/v1/items", headers); w.Code != 200 || w.Body.String() != "restored-active" {
		t.Fatal("stable identity did not follow active restore")
	}
	f.mu.Lock()
	v := f.containers[s.dataName(dataInstallation{ProjectID: "owned", StackID: "stack-restored"}, "api-gw")]
	v.State.Running = false
	f.containers[s.dataName(dataInstallation{ProjectID: "owned", StackID: "stack-restored"}, "api-gw")] = v
	f.mu.Unlock()
	if w := bridgePlatformRequest(t, s, "GET", base+"/rest/v1/items", headers); w.Code != 409 {
		t.Fatal("stopped ownership target accepted")
	}
}

func TestCloudBrowserDataConfigurationRequiresBoundServerTransport(t *testing.T) {
	s, f := newDataFake(t)
	secret := newDataSecret()
	seedBridgeData(t, s, f, "stack-first", "durable-data", "http://127.0.0.1:34567", secret)
	origin := "http://" + strings.Repeat("a", 32) + "." + strings.Repeat("a", 32) + ".localhost:8741"
	makeRequest := func() *http.Request {
		r := httptest.NewRequest("POST", "http://127.0.0.1:8730/v1/platform/projects/owned/deployments", nil)
		r.Header.Set("X-Luci-Preview-Bridge", "1")
		r.Header.Set("X-Luci-Workspace-Data-Origin", origin)
		r.Header.Set("X-Luci-Workspace-Data-Resource", "durable-data")
		return r
	}
	values, _ := s.dataBindings("owned")
	if err := s.deploymentBrowserData(makeRequest(), "owned", values); err != nil || values["SUPABASE_PUBLIC_URL"] != origin || values["SUPABASE_URL"] != "http://supabase-api:8000" || values["SUPABASE_SERVICE_ROLE_KEY"] != secret.ServiceRole {
		t.Fatal("server/client data configuration mixed", err)
	}
	for _, mutate := range []func(*http.Request){func(r *http.Request) { r.Header.Del("X-Luci-Preview-Bridge") }, func(r *http.Request) { r.Header.Set("Sec-Fetch-Mode", "cors") }, func(r *http.Request) { r.Header.Set("X-Luci-Workspace-Data-Resource", "foreign") }, func(r *http.Request) { r.Header.Set("X-Luci-Workspace-Data-Origin", "https://evil.example") }, func(r *http.Request) { r.Header.Set("X-Luci-Workspace-Data-Origin", "http://127.0.0.1:8730") }, func(r *http.Request) { r.Header.Del("X-Luci-Workspace-Data-Resource") }} {
		r := makeRequest()
		mutate(r)
		values, _ := s.dataBindings("owned")
		if err := s.deploymentBrowserData(r, "owned", values); err == nil {
			t.Fatal("spoofed or stale browser binding accepted")
		}
	}
	for _, malformed := range []string{"http://" + strings.Repeat("a", 64) + ".localhost:8741", "http://" + strings.Repeat("a", 31) + "." + strings.Repeat("a", 33) + ".localhost:8741", "http://extra." + strings.Repeat("a", 32) + "." + strings.Repeat("a", 32) + ".localhost:8741"} {
		r := makeRequest()
		r.Header.Set("X-Luci-Workspace-Data-Origin", malformed)
		values, _ := s.dataBindings("owned")
		if err := s.deploymentBrowserData(r, "owned", values); err == nil {
			t.Fatal("invalid DNS capability layout was accepted as a browser binding")
		}
	}
}

func TestCloudBrowserDataOriginReachesActualPrivateDeploymentRuntime(t *testing.T) {
	s, f := newDataFake(t)
	secret := newDataSecret()
	seedBridgeData(t, s, f, "stack-first", "durable-data", "http://127.0.0.1:34567", secret)
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	defer app.Close()
	appURL, _ := url.Parse(app.URL)
	s.opts.HTTPClient = &http.Client{}
	origin := "http://" + strings.Repeat("a", 32) + "." + strings.Repeat("a", 32) + ".localhost:8741"
	r := httptest.NewRequest("POST", "http://127.0.0.1:8730/v1/platform/projects/owned/deployments", nil)
	r.Header.Set("X-Luci-Preview-Bridge", "1")
	r.Header.Set("X-Luci-Workspace-Data-Origin", origin)
	r.Header.Set("X-Luci-Workspace-Data-Resource", "durable-data")
	_, values, err := s.environment("owned")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.deploymentBrowserData(r, "owned", values); err != nil {
		t.Fatal(err)
	}
	deployment := Deployment{ID: "deploy-browser", ProjectID: "owned", Provider: "docker", Commit: strings.Repeat("a", 40), ConfigurationRevision: 7, Port: 3000, HealthPath: "/", Revision: 1}
	deployment.Runtime = &previewruntime.Outcome{Identity: previewruntime.Identity{ProjectID: deployment.ProjectID, ReleaseID: deployment.ID, Target: "native-docker", Commit: deployment.Commit, SourceDigest: strings.Repeat("d", 64)}, Phase: "reserved"}
	if err = s.saveDeployment(deployment); err != nil {
		t.Fatal(err)
	}
	containerID := "preview-container-" + deployment.ID
	network := s.resource(deployment.ProjectID, "net")
	networkID := strings.Repeat("b", 64)
	f.mu.Lock()
	f.networks[network] = map[string]string{labelInstance: s.instance, labelProject: deployment.ProjectID, labelResource: "network"}
	f.mu.Unlock()
	var runtimeBytes []byte
	var runtimeFile, runtimeNetwork string
	s.opts.Runner = func(ctx context.Context, exe string, args []string, dir string, input []byte) ([]byte, error) {
		if exe == "git" {
			return tarSource(t, map[string]string{"Dockerfile": "FROM scratch", "LUCI.md": "approved"}), nil
		}
		if len(args) >= 3 && args[0] == "network" && args[1] == "inspect" {
			if args[2] != network {
				t.Error("inspected another project's network")
			}
			return json.Marshal(map[string]any{"Id": networkID, "Driver": "bridge", "Labels": map[string]string{labelInstance: s.instance, labelProject: deployment.ProjectID, labelResource: "network"}})
		}
		if args[0] == "build" {
			if bytes.Contains(input, []byte(secret.ServiceRole)) || bytes.Contains(input, []byte(origin)) {
				t.Error("runtime input entered build context")
			}
			return []byte("built"), nil
		}
		if args[0] == "run" {
			for index, arg := range args {
				if arg == "--network" {
					runtimeNetwork = args[index+1]
				}
				if arg == "--env-file" {
					runtimeFile = args[index+1]
					runtimeBytes, err = os.ReadFile(runtimeFile)
					if err != nil {
						return nil, err
					}
				}
			}
			v := containerInfo{ID: containerID, Image: "sha256:immutable"}
			v.State.Running = true
			v.Config.Labels = map[string]string{labelInstance: s.instance, labelProject: "owned", labelResource: deployment.ID}
			v.NetworkSettings.Ports = map[string][]struct{ HostIP, HostPort string }{"3000/tcp": {{"127.0.0.1", appURL.Port()}}}
			f.mu.Lock()
			f.containers[containerID] = v
			f.mu.Unlock()
			return []byte("container"), nil
		}
		return f.run(ctx, exe, args, dir, input)
	}
	p, _ := s.opts.ResolveProject("owned")
	request := previewruntime.Request{Identity: deployment.Runtime.Identity, Environment: values, Port: deployment.Port, Dockerfile: "Dockerfile", MemoryMiB: 512, CPUMillis: 500,
		Labels: map[string]string{labelInstance: s.instance, labelProject: deployment.ProjectID, labelResource: deployment.ID},
	}
	if err = s.buildAndRun(context.Background(), p, &deployment, request); err != nil {
		t.Fatal(err)
	}
	if runtimeNetwork != networkID {
		t.Fatal("runtime did not join the existing owned data network")
	}
	for _, expected := range []string{"SUPABASE_PUBLIC_URL=" + origin + "\n", "SUPABASE_URL=http://supabase-api:8000\n", "SUPABASE_SERVICE_ROLE_KEY=" + secret.ServiceRole + "\n"} {
		if !bytes.Contains(runtimeBytes, []byte(expected)) {
			t.Fatal("cloud runtime lost its bound browser/internal data configuration")
		}
	}
	if _, err = os.Stat(runtimeFile); !os.IsNotExist(err) {
		t.Fatal("temporary private runtime configuration retained")
	}
}
