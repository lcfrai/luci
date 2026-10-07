package previewgateway

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func testGateway(t *testing.T, verify Verify) (*Gateway, *Registry) {
	t.Helper()
	root := t.TempDir()
	state := filepath.Join(root, "sessions")
	if err := os.MkdirAll(state, 0700); err != nil {
		t.Fatal(err)
	}
	r, err := NewRegistry(state, 8731)
	if err != nil {
		t.Fatal(err)
	}
	g, err := New(root, 8731, verify)
	if err != nil {
		t.Fatal(err)
	}
	return g, r
}
func appTarget(upstream string) Target {
	return Target{Kind: "application", ProjectID: "project-1", ResourceID: "release-1", Upstream: upstream, Container: "owned-app", Labels: map[string]string{"luci.project-id": "project-1", "luci.release-id": "release-1"}}
}
func request(g *Gateway, method, origin, path string, headers map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, origin+path, strings.NewReader("{}"))
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	g.ServeHTTP(w, r)
	return w
}
func TestGatewayOwnedOriginPathsCookiesAuthorizationAndRedirect(t *testing.T) {
	seen := make(chan *http.Request, 1)
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen <- r.Clone(context.Background())
		w.Header().Add("Set-Cookie", "session=app; Domain=.localhost; Path=/; HttpOnly; SameSite=Lax")
		w.Header().Set("Location", "http://"+r.Host+"/next?x=1")
		w.WriteHeader(302)
	}))
	defer up.Close()
	checks := 0
	g, registry := testGateway(t, func(_ context.Context, target Target) error {
		checks++
		if target.Container != "owned-app" {
			return errors.New("wrong owner")
		}
		return nil
	})
	origin, err := registry.Publish(appTarget(up.URL))
	if err != nil {
		t.Fatal(err)
	}
	res := request(g, "POST", origin, "/files/a%2Fb?q=1%2F2", map[string]string{"Origin": origin, "Authorization": "Bearer application-token", "X-Luci-Actor": "never-forward", "X-Forwarded-Host": "attacker.test", "Referer": "http://127.0.0.1:8730/"})
	if res.Code != 302 || checks != 1 {
		t.Fatalf("status=%d ownership checks=%d", res.Code, checks)
	}
	got := <-seen
	if got.URL.EscapedPath() != "/files/a%2Fb" || got.URL.RawQuery != "q=1%2F2" {
		t.Fatalf("path/query altered: %s", got.URL.String())
	}
	if got.Header.Get("Authorization") != "Bearer application-token" || got.Header.Get("X-Luci-Actor") != "" || got.Header.Get("X-Forwarded-Host") != "" || got.Header.Get("Referer") != "" || got.Header.Get("Origin") != up.URL {
		t.Fatalf("application/control header boundary failed")
	}
	if res.Header().Get("Location") != origin+"/next?x=1" {
		t.Fatal("redirect exposes inner Docker address")
	}
	cookie := res.Header().Get("Set-Cookie")
	if strings.Contains(strings.ToLower(cookie), "domain=") || !strings.Contains(cookie, "HttpOnly") || !strings.Contains(cookie, "SameSite=Lax") {
		t.Fatalf("cookie isolation failed: %s", cookie)
	}
	for _, host := range []string{"127.0.0.1:8731", "attacker.test:8731", strings.Replace(strings.TrimPrefix(origin, "http://"), "8731", "8732", 1)} {
		r := httptest.NewRequest("GET", up.URL+"/", nil)
		r.Host = host
		w := httptest.NewRecorder()
		g.ServeHTTP(w, r)
		if w.Code != 404 {
			t.Fatalf("host %s accepted", host)
		}
	}
	if res := request(g, "POST", origin, "/write", map[string]string{"Origin": "https://attacker.test"}); res.Code != 403 {
		t.Fatal("foreign mutation origin accepted")
	}
	if res := request(g, "POST", origin, "/write", map[string]string{"Sec-Fetch-Site": "cross-site"}); res.Code != 403 {
		t.Fatal("cross-site mutation accepted")
	}
}
func clientJWT(role string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256"}`)) + "." + base64.RawURLEncoding.EncodeToString([]byte(`{"role":"`+role+`"}`)) + "." + base64.RawURLEncoding.EncodeToString([]byte("signature-for-contract"))
}
func TestDataClientAuthenticationAndProjectCORS(t *testing.T) {
	var authorization string
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization = r.Header.Get("Authorization")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Write([]byte("ok"))
	}))
	defer up.Close()
	g, registry := testGateway(t, func(context.Context, Target) error { return nil })
	app, err := registry.Publish(appTarget(up.URL))
	if err != nil {
		t.Fatal(err)
	}
	target := appTarget(up.URL)
	target.Kind = "data"
	target.ResourceID = "data-1"
	target.PublishableKey = clientJWT("anon")
	data, err := registry.Publish(target)
	if err != nil {
		t.Fatal(err)
	}
	headers := map[string]string{"Origin": app, "Authorization": "Bearer " + clientJWT("authenticated"), "apikey": target.PublishableKey}
	res := request(g, "POST", data, "/rest/v1/notes", headers)
	if res.Code != 200 || authorization != headers["Authorization"] || res.Header().Get("Access-Control-Allow-Origin") != app {
		t.Fatalf("legitimate Supabase authentication/CORS rejected status=%d", res.Code)
	}
	if res := request(g, "OPTIONS", data, "/storage/v1/object", map[string]string{"Origin": app, "Access-Control-Request-Method": "POST"}); res.Code != 204 || !strings.Contains(res.Header().Get("Access-Control-Allow-Headers"), "authorization") {
		t.Fatal("client preflight unavailable")
	}
	for _, token := range []string{"opaque-luci-session-token", clientJWT("service_role"), clientJWT("postgres")} {
		if res := request(g, "GET", data, "/rest/v1/notes", map[string]string{"Authorization": "Bearer " + token}); res.Code != 401 {
			t.Fatal("privileged/control token accepted")
		}
	}
	if res := request(g, "GET", data, "/rest/v1/notes?apikey=other-project-key", nil); res.Code != 401 {
		t.Fatal("foreign project publishable key accepted")
	}
	if res := request(g, "GET", data, "/realtime/v1/websocket?access_token="+clientJWT("service_role"), nil); res.Code != 401 {
		t.Fatal("privileged websocket token accepted")
	}
	if res := request(g, "GET", data, "/pg/query", nil); res.Code != 404 {
		t.Fatal("management API exposed")
	}
	otherState := filepath.Join(g.stateDir, "control-plane", "users", "user2", "runtime", "state", "sessions")
	if err := os.MkdirAll(otherState, 0700); err != nil {
		t.Fatal(err)
	}
	otherRegistry, err := NewRegistry(otherState, 8731)
	if err != nil {
		t.Fatal(err)
	}
	otherApp, err := otherRegistry.Publish(appTarget(up.URL))
	if err != nil {
		t.Fatal(err)
	}
	if res := request(g, "GET", data, "/rest/v1/notes", map[string]string{"Origin": otherApp}); res.Code != 403 {
		t.Fatal("other account's same project ID crossed ownership boundary")
	}
}
func TestDataCrossOriginResponseHasOneTrustedAllowOrigin(t *testing.T) {
	for _, upstreamOrigins := range [][]string{nil, {"*"}, {"https://untrusted.example", "*"}} {
		t.Run(fmt.Sprintf("upstream-headers-%d", len(upstreamOrigins)), func(t *testing.T) {
			up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				for _, origin := range upstreamOrigins {
					w.Header().Add("Access-Control-Allow-Origin", origin)
				}
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`[]`))
			}))
			defer up.Close()
			g, registry := testGateway(t, func(context.Context, Target) error { return nil })
			app, err := registry.Publish(appTarget(up.URL))
			if err != nil {
				t.Fatal(err)
			}
			target := appTarget(up.URL)
			target.Kind, target.ResourceID, target.PublishableKey = "data", "data-cors", clientJWT("anon")
			data, err := registry.Publish(target)
			if err != nil {
				t.Fatal(err)
			}
			preflight := request(g, "OPTIONS", data, "/rest/v1/notes", map[string]string{"Origin": app, "Access-Control-Request-Method": "GET", "Access-Control-Request-Headers": "apikey,authorization"})
			if origins := preflight.Header().Values("Access-Control-Allow-Origin"); preflight.Code != 204 || len(origins) != 1 || origins[0] != app {
				t.Fatal("the approved project's preflight did not have one trusted origin")
			}
			res := request(g, "GET", data, "/rest/v1/notes", map[string]string{"Origin": app, "apikey": target.PublishableKey, "Authorization": "Bearer " + clientJWT("authenticated")})
			origins := res.Result().Header.Values("Access-Control-Allow-Origin")
			// Header.Get checks only the first field and misses this browser-
			// rejecting duplicate introduced by ReverseProxy's header append.
			if res.Code != 200 || len(origins) != 1 || origins[0] != app {
				t.Fatalf("the approved project's proxied response must have one trusted origin: status=%d fields=%d", res.Code, len(origins))
			}
			if res.Header().Get("Access-Control-Allow-Credentials") != "" || !strings.Contains(res.Header().Get("Vary"), "Origin") || res.Body.String() != `[]` {
				t.Fatal("the proxied client response changed its credential, cache, or body boundary")
			}
		})
	}
}
func TestRegistryRejectsHostileTargetsAndOwnershipReuse(t *testing.T) {
	g, r := testGateway(t, func(context.Context, Target) error { return errors.New("container label or binding changed") })
	for _, upstream := range []string{"http://localhost:3000", "http://127.0.0.2:3000", "https://127.0.0.1:3000", "http://user:pass@127.0.0.1:3000", "http://127.0.0.1:3000/path", "http://127.0.0.1:3000?query=1", "http://127.0.0.1:8730", "http://127.0.0.1:8731", "http://169.254.169.254:80"} {
		if _, err := r.Publish(appTarget(upstream)); err == nil {
			t.Fatalf("hostile upstream accepted %s", upstream)
		}
	}
	target := appTarget("http://127.0.0.1:3000")
	target.Labels["luci.project-id"] = "someone-else"
	if _, err := r.Publish(target); err == nil {
		t.Fatal("project ownership mismatch accepted")
	}
	origin, err := r.Publish(appTarget("http://127.0.0.1:3000"))
	if err != nil {
		t.Fatal(err)
	}
	again, err := r.Publish(appTarget("http://127.0.0.1:3000"))
	if err != nil || origin != again {
		t.Fatal("browser origin changed on a refresh")
	}
	if res := request(g, "GET", origin, "/", nil); res.Code != 503 {
		t.Fatal("stopped/rebound container was proxied")
	}
	if err := r.Remove("application", "project-1", "release-1"); err != nil {
		t.Fatal(err)
	}
	if res := request(g, "GET", origin, "/", nil); res.Code != 404 {
		t.Fatal("revoked target remained available")
	}
	b := []byte(`{"running":true,"labels":{"luci.project-id":"project-1","luci.release-id":"release-1"},"ports":{"3000/tcp":[{"HostIP":"127.0.0.1","HostPort":"3000"}]}}`)
	if err := verifyContainer(appTarget("http://127.0.0.1:3000"), b); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{strings.Replace(string(b), `"project-1"`, `"foreign"`, 1), strings.Replace(string(b), `"running":true`, `"running":false`, 1), strings.Replace(string(b), `"127.0.0.1"`, `"0.0.0.0"`, 1), strings.Replace(string(b), `"HostPort":"3000"`, `"HostPort":"3001"`, 1)} {
		if err := verifyContainer(appTarget("http://127.0.0.1:3000"), []byte(bad)); err == nil {
			t.Fatal("unowned/reused/public container binding accepted")
		}
	}
}
func TestRegistryDoesNotFollowLinks(t *testing.T) {
	g, registry := testGateway(t, func(context.Context, Target) error { return nil })
	origin, err := registry.Publish(appTarget("http://127.0.0.1:3000"))
	if err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join(registry.dir, "*.json"))
	if err != nil || len(files) != 1 {
		t.Fatal("registration missing")
	}
	outside := filepath.Join(t.TempDir(), "copied.json")
	if err := os.Rename(files[0], outside); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(outside, files[0]); err != nil {
		t.Skipf("host cannot create hardlinks: %v", err)
	}
	if res := request(g, "GET", origin, "/", nil); res.Code != 404 {
		t.Fatal("hardlinked registration accepted")
	}
}
func TestGatewayPreservesWebSocketProtocol(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer c.Close(websocket.StatusNormalClosure, "")
		kind, b, err := c.Read(r.Context())
		if err == nil {
			_ = c.Write(r.Context(), kind, b)
		}
	}))
	defer up.Close()
	root := t.TempDir()
	g, err := New(root, 8731, func(context.Context, Target) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(g)
	defer server.Close()
	_, p, _ := net.SplitHostPort(server.Listener.Addr().String())
	g.port, _ = strconv.Atoi(p)
	state := filepath.Join(root, "sessions")
	if err := os.MkdirAll(state, 0700); err != nil {
		t.Fatal(err)
	}
	registry, err := NewRegistry(state, g.port)
	if err != nil {
		t.Fatal(err)
	}
	origin, err := registry.Publish(appTarget(up.URL))
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: &http.Transport{Proxy: nil, DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}}}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, res, err := websocket.Dial(ctx, origin+"/socket", &websocket.DialOptions{HTTPClient: client, HTTPHeader: http.Header{"Origin": []string{origin}}})
	if err != nil {
		if res != nil {
			b, _ := io.ReadAll(res.Body)
			t.Fatalf("websocket rejected %d %s: %v", res.StatusCode, b, err)
		}
		t.Fatal(err)
	}
	defer c.CloseNow()
	if err := c.Write(ctx, websocket.MessageText, []byte("echo")); err != nil {
		t.Fatal(err)
	}
	kind, b, err := c.Read(ctx)
	if err != nil || kind != websocket.MessageText || string(b) != "echo" {
		t.Fatal(fmt.Sprintf("websocket relay failed %v %s", err, b))
	}
	u, _ := url.Parse(origin)
	if !strings.HasSuffix(u.Hostname(), ".localhost") {
		t.Fatal("preview shares control origin")
	}
}

func TestBridgeLivenessRejectsReusedPort(t *testing.T) {
	key, err := NewBridgeKey()
	if err != nil {
		t.Fatal(err)
	}
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(BridgeProofHeader, BridgeProof(key, r.Header.Get(BridgeNonceHeader)))
		w.WriteHeader(204)
	}))
	defer up.Close()
	target := Target{Kind: "bridge", ProjectID: "project-1", ResourceID: "release-1", Upstream: up.URL, BridgeKey: key}
	if err := DockerVerifier("")(context.Background(), target); err != nil {
		t.Fatal(err)
	}
	target.BridgeKey, _ = NewBridgeKey()
	if err := DockerVerifier("")(context.Background(), target); err == nil {
		t.Fatal("different bridge reusing port accepted")
	}
	if BridgeProof(key, "malformed-nonce") != "" {
		t.Fatal("malformed bridge challenge accepted")
	}
	if _, err := validate(Target{Kind: "bridge", ProjectID: "project-1", ResourceID: "release-1", Upstream: up.URL}, 8731); err == nil {
		t.Fatal("unproven bridge registration accepted")
	}
}
