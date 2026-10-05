package product

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/platform"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

func cloudClientJWT(role string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256"}`)) + "." + base64.RawURLEncoding.EncodeToString([]byte(`{"role":"`+role+`"}`)) + "." + base64.RawURLEncoding.EncodeToString([]byte(strings.Repeat("s", 32)))
}

func TestCloudPlatformBridgeRealManagementRESTStorageRealtimeAndRevocation(t *testing.T) {
	anon, userJWT, serviceKey := cloudClientJWT("anon"), cloudClientJWT("authenticated"), cloudClientJWT("service_role")
	var mu sync.Mutex
	var upstreamSeen []map[string]string
	dataAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen := map[string]string{"path": r.URL.EscapedPath(), "query": r.URL.RawQuery, "auth": r.Header.Get("Authorization"), "key": r.Header.Get("apikey"), "cookie": r.Header.Get("Cookie"), "control": r.Header.Get("X-Luci-Actor") + r.Header.Get("X-Luci-Policy") + r.Header.Get("X-Luci-Preview-Bridge") + r.Header.Get("X-Luci-Preview-App-Authorization")}
		mu.Lock()
		upstreamSeen = append(upstreamSeen, seen)
		mu.Unlock()
		if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
			connection, err := websocket.Accept(w, r, nil)
			if err != nil {
				return
			}
			defer connection.CloseNow()
			for {
				kind, body, err := connection.Read(r.Context())
				if err != nil {
					return
				}
				if connection.Write(r.Context(), kind, body) != nil {
					return
				}
			}
		}
		writeJSON(w, 200, seen)
	}))
	defer dataAPI.Close()
	appAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("cloud application")) }))
	defer appAPI.Close()
	remoteRoot := t.TempDir()
	remoteState := filepath.Join(remoteRoot, "sessions")
	_ = os.MkdirAll(remoteState, 0700)
	instanceSum := sha256.Sum256([]byte(remoteState + "test-cloud"))
	instance := hex.EncodeToString(instanceSum[:8])
	dataAddress, _ := url.Parse(dataAPI.URL)
	appAddress, _ := url.Parse(appAPI.URL)
	currentStack := "stack-first"
	stopped := false
	runner := func(_ context.Context, exe string, args []string, _ string, _ []byte) ([]byte, error) {
		mu.Lock()
		defer mu.Unlock()
		if exe != "docker" || len(args) != 2 || args[0] != "inspect" {
			return nil, errors.New("only owned metadata inspection expected")
		}
		container := args[1]
		projectID, resource, port := "owned", "data-"+currentStack+"-api-gw", dataAddress.Port()
		internal := 8000
		if !strings.HasSuffix(container, currentStack+"-api-gw") {
			resource = "deploy-fixed"
			port = appAddress.Port()
			internal = 3000
		}
		if strings.Contains(container, "other") {
			projectID = "other"
		}
		value := []any{map[string]any{"Image": "sha256:immutable", "Config": map[string]any{"Labels": map[string]string{"ai.lcfr.luci.platform.instance": instance, "ai.lcfr.luci.platform.project": projectID, "ai.lcfr.luci.platform.resource": resource}}, "State": map[string]any{"Running": !stopped, "Health": map[string]string{"Status": "healthy"}}, "NetworkSettings": map[string]any{"Ports": map[string]any{map[bool]string{true: "8000/tcp", false: "3000/tcp"}[internal == 8000]: []any{map[string]string{"HostIP": "127.0.0.1", "HostPort": port}}}}}}
		return json.Marshal(value)
	}
	remoteService, err := platform.New(platform.Options{StateDir: remoteState, InstanceID: "test-cloud", ResolveProject: func(id string) (project.Project, error) {
		if id != "owned" && id != "other" {
			return project.Project{}, os.ErrNotExist
		}
		return project.Project{ID: id, Path: remoteRoot}, nil
	}, Runner: runner})
	if err != nil {
		t.Fatal(err)
	}
	defer remoteService.Close()
	seed := func(stack, address string) {
		t.Helper()
		u, _ := url.Parse(address)
		values := map[string]any{"owned-data-installation": map[string]any{"projectId": "owned", "stackId": stack, "storageTenantId": "stable-data", "apiPort": uPortNumber(t, u)}, stack + "-secret": map[string]string{"jwt": strings.Repeat("j", 32), "password": strings.Repeat("p", 24), "anon": anon, "serviceRole": serviceKey}, "owned-data": platform.DataStatus{ProjectID: "owned", Provider: "supabase", Status: "ready", EndpointScope: "selected-docker-host-loopback", URL: address, PublishableKey: anon}, "owned": platform.ProjectState{ProjectID: "owned", Deployments: []platform.Deployment{{ID: "deploy-fixed", ProjectID: "owned", Provider: "docker", Status: "ready", Container: "app-owned", Port: 3000, URL: appAPI.URL}}}, "other": platform.ProjectState{ProjectID: "other", Deployments: []platform.Deployment{{ID: "deploy-fixed", ProjectID: "other", Provider: "docker", Status: "ready", Container: "app-other", Port: 3000, URL: appAPI.URL}}}}
		for name, value := range values {
			bytes, _ := json.Marshal(value)
			if err := os.WriteFile(filepath.Join(remoteState, "platform", name), bytes, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	seed(currentStack, dataAPI.URL)
	workerSecret := strings.Repeat("w", 43)
	var deploymentOrigin, deploymentResource, deploymentFetch string
	var deploymentCalls int
	remoteWorker := management.WorkerHandler(workerSecret, remoteService.Handler())
	workerTransport := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := management.ActorFromContext(r.Context())
		if !ok {
			http.Error(w, "identity missing", 403)
			return
		}
		if r.Method == "POST" && r.URL.Path == "/v1/platform/projects/owned/deployments" {
			deploymentCalls++
			deploymentOrigin = r.Header.Get("X-Luci-Workspace-Data-Origin")
			deploymentResource = r.Header.Get("X-Luci-Workspace-Data-Resource")
			deploymentFetch = r.Header.Get("Sec-Fetch-Mode")
		}
		actor, _ := json.Marshal(u)
		policy, _ := json.Marshal(p)
		r.Header.Set("Authorization", "Bearer "+workerSecret)
		r.Header.Set("X-Luci-Actor", base64.RawURLEncoding.EncodeToString(actor))
		r.Header.Set("X-Luci-Policy", base64.RawURLEncoding.EncodeToString(policy))
		remoteWorker.ServeHTTP(w, r)
	})
	gateway, err := management.New(management.Options{StateDir: remoteRoot, Version: Version, InitialHandler: workerTransport})
	if err != nil {
		t.Fatal(err)
	}
	defer gateway.Close(context.Background())
	s, handler, tunnel := workspaceAdminFixture(t, gateway.Handler())
	defer s.closePreviews()
	bootstrap, _ := os.ReadFile(filepath.Join(remoteRoot, "control-plane/bootstrap-token"))
	var owner struct {
		Token string          `json:"token"`
		User  management.User `json:"user"`
	}
	if err := remoteWorkspaceAPI(context.Background(), tunnel.APIURL, "POST", "/v1/auth/bootstrap", "", map[string]string{"bootstrapToken": strings.TrimSpace(string(bootstrap)), "username": "cloud-owner", "displayName": "Cloud Owner", "password": "cloud-owner-password-long"}, &owner); err != nil {
		t.Fatal(err)
	}
	s.workspaceAuth[workspaceCredentialKey(*tunnel)] = workspaceCredential{Token: owner.Token, User: owner.User}
	before := journeyRequest(t, handler, "GET", "/v1/platform/projects/owned", nil, 200)
	beforeData := journeyRequest(t, handler, "GET", "/v1/platform/projects/owned/data", nil, 200)
	if strings.Contains(before.Body.String(), appAPI.URL) || strings.Contains(beforeData.Body.String(), dataAPI.URL) || !strings.Contains(before.Body.String(), `"browserConnectionRequired":true`) || !strings.Contains(beforeData.Body.String(), `"endpointScope":"workspace-browser-pending"`) || len(s.previews) != 0 {
		t.Fatal("GET exposed unusable remote URL or allocated listener")
	}
	journeyRequest(t, handler, "GET", "/v1/setup/workspace/platform/browser", nil, 405)
	open := func(kind, projectID string) string {
		t.Helper()
		body := map[string]string{"kind": kind, "projectId": projectID}
		if kind == "application" {
			body["resourceId"] = "deploy-fixed"
		}
		w := journeyRequest(t, handler, "POST", "/v1/setup/workspace/platform/browser", body, 200)
		if strings.Contains(w.Body.String(), owner.Token) || strings.Contains(w.Body.String(), serviceKey) {
			t.Fatal("bridge DTO leaked private credentials")
		}
		var opened struct {
			URL string `json:"url"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &opened)
		return opened.URL
	}
	appOrigin := open("application", "owned")
	dataOrigin := open("data", "owned")
	otherOrigin := open("application", "other")
	if open("data", "owned") != dataOrigin {
		t.Fatal("data origin changed on repeated connect")
	}
	deployRequest := httptest.NewRequest("POST", "/v1/platform/projects/owned/deployments", strings.NewReader(`{"commit":"`+strings.Repeat("a", 40)+`","port":3000,"healthPath":"/"}`))
	deployRequest.Header.Set("X-Luci-Workspace-Data-Origin", "http://127.0.0.1:8730")
	deployRequest.Header.Set("X-Luci-Workspace-Data-Resource", "forged-data")
	deployRequest.Header.Set("Sec-Fetch-Mode", "cors")
	deployReceipt := httptest.NewRecorder()
	handler.ServeHTTP(deployReceipt, deployRequest)
	if deployReceipt.Code != 503 || deploymentCalls != 1 || deploymentOrigin != dataOrigin || deploymentResource != "stable-data" || deploymentFetch != "" {
		t.Fatal("deployment POST did not replace spoofed headers with owned client bridge", deployReceipt.Code)
	}
	client := &http.Client{Timeout: 5 * time.Second}
	do := func(method, address, origin, auth string, code int) map[string]string {
		t.Helper()
		req, _ := http.NewRequest(method, address, strings.NewReader("payload"))
		req.Header.Set("Origin", origin)
		req.Header.Set("Authorization", auth)
		req.Header.Set("apikey", anon)
		req.Header.Set("X-Luci-Actor", "forged")
		req.Header.Set("X-Luci-Policy", "forged")
		req.Header.Set("Cookie", "luci-control-cookie=never-forward")
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var seen map[string]string
		_ = json.NewDecoder(res.Body).Decode(&seen)
		if res.StatusCode != code {
			t.Fatalf("bridge status %d want %d", res.StatusCode, code)
		}
		if code == 200 && res.Header.Get("Access-Control-Allow-Origin") != origin {
			t.Fatal("same project CORS lost")
		}
		return seen
	}
	for _, path := range []string{"/auth/v1/token", "/rest/v1/notes", "/storage/v1/object/bucket/nested%2Ffile"} {
		seen := do("POST", dataOrigin+path, appOrigin, "Bearer "+userJWT, 200)
		if seen["auth"] != "Bearer "+userJWT || seen["control"] != "" || seen["cookie"] != "" || strings.Contains(seen["auth"], owner.Token) {
			t.Fatal("upstream received control authorization")
		}
	}
	do("POST", dataOrigin+"/rest/v1/notes", otherOrigin, "Bearer "+userJWT, 403)
	do("GET", dataOrigin+"/rest/v1/notes", appOrigin, "Bearer "+serviceKey, 401)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	wsURL := "ws" + strings.TrimPrefix(dataOrigin, "http") + "/realtime/v1/websocket?apikey=" + url.QueryEscape(anon) + "&access_token=" + url.QueryEscape(userJWT) + "&ticket=must-not-reach-data"
	connection, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPHeader: http.Header{"Origin": []string{appOrigin}, "Authorization": []string{"Bearer " + userJWT}}})
	if err != nil {
		t.Fatal("real managed Realtime bridge upgrade failed", err)
	}
	defer connection.CloseNow()
	payload := []byte{1, 0, 255, 2}
	if err := connection.Write(ctx, websocket.MessageBinary, payload); err != nil {
		t.Fatal(err)
	}
	_, echo, err := connection.Read(ctx)
	if err != nil || string(echo) != string(payload) {
		t.Fatal("Realtime frame relay failed", err)
	}
	mu.Lock()
	last := upstreamSeen[len(upstreamSeen)-1]
	mu.Unlock()
	if strings.Contains(last["query"], "ticket") || last["control"] != "" || last["auth"] != "Bearer "+userJWT {
		t.Fatal("Realtime carried control credentials")
	}
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"active": "restored"})
	}))
	defer second.Close()
	mu.Lock()
	currentStack = "stack-restored"
	dataAddress, _ = url.Parse(second.URL)
	mu.Unlock()
	seed("stack-restored", second.URL)
	if seen := do("GET", dataOrigin+"/rest/v1/notes", appOrigin, "Bearer "+userJWT, 200); seen["active"] != "restored" {
		t.Fatal("stable cloud data address retained old stack")
	}
	if err := remoteWorkspaceAPI(context.Background(), tunnel.APIURL, "POST", "/v1/auth/logout", owner.Token, map[string]any{}, nil); err != nil {
		t.Fatal(err)
	}
	do("GET", dataOrigin+"/rest/v1/notes", appOrigin, "Bearer "+userJWT, 401)
	if _, _, err := connection.Read(ctx); err == nil {
		t.Fatal("revoked remote account kept managed Realtime socket alive")
	}
	s.workspaceAuth[workspaceCredentialKey(*tunnel)] = workspaceCredential{Token: owner.Token, User: management.User{ID: "another-account"}}
	do("GET", dataOrigin+"/rest/v1/notes", appOrigin, "Bearer "+userJWT, 503)
	tunnel.Status = "disconnected"
	journeyRequest(t, handler, "POST", "/v1/setup/workspace/platform/browser", map[string]string{"projectId": "owned", "kind": "data"}, 409)
}

func uPortNumber(t *testing.T, u *url.URL) int {
	t.Helper()
	var port int
	if _, err := fmt.Sscan(u.Port(), &port); err != nil {
		t.Fatal(err)
	}
	return port
}
