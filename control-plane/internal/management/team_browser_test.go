package management

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/lcfrai/luci/control-plane/internal/previewgateway"
)

func teamBrowserFixture(t *testing.T, app http.Handler) (*Gateway, http.Handler, string, Team, *previewgateway.Gateway, string) {
	t.Helper()
	g, h, token, _ := fixture(t, nil)
	team := createTestTeam(t, h, token, "Browser team")
	upstream := httptest.NewServer(app)
	t.Cleanup(upstream.Close)
	state := filepath.Join(g.dir, "teams", team.ID, "runtime", "state", "sessions")
	if err := os.MkdirAll(state, 0700); err != nil {
		t.Fatal(err)
	}
	registry, err := previewgateway.NewRegistry(state, 8731)
	if err != nil {
		t.Fatal(err)
	}
	origin, err := registry.Publish(previewgateway.Target{Kind: "application", ProjectID: "p-browser", ResourceID: "app-browser", Upstream: upstream.URL, Container: "owned-app", Labels: map[string]string{"luci.project-id": "p-browser", "luci.resource": "app-browser"}})
	if err != nil {
		t.Fatal(err)
	}
	browser, err := previewgateway.New(g.opts.StateDir, 8731, func(context.Context, previewgateway.Target) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	g.SetBrowserGateway(browser)
	return g, h, token, team, browser, origin
}

func issueBrowserHandoff(t *testing.T, h http.Handler, token string, team Team, origin string) string {
	t.Helper()
	w := request(t, h, "POST", "/v1/teams/"+team.ID+"/projects/p-browser/browser-access", token, map[string]string{"url": origin + "/files/a%2Fb?client=1#view"}, 200)
	var data struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(data.URL, origin+teamBrowserHandoffPath+"?ticket=") || strings.Contains(data.URL, token) {
		t.Fatal("unsafe browser handoff")
	}
	return data.URL
}

func browserRequest(h http.Handler, address string, cookie *http.Cookie) *httptest.ResponseRecorder {
	r := httptest.NewRequest("GET", address, nil)
	if cookie != nil {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func consumeBrowserHandoff(t *testing.T, browser http.Handler, handoff string) *http.Cookie {
	t.Helper()
	w := browserRequest(browser, handoff, nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"destination":"/files/a%2Fb?client=1#view"`) {
		t.Fatalf("handoff failed: %d %s", w.Code, w.Body.String())
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != teamBrowserCookie || !cookies[0].HttpOnly || !cookies[0].Secure || !cookies[0].Partitioned || cookies[0].Domain != "" || cookies[0].SameSite != http.SameSiteNoneMode {
		t.Fatal("browser cookie lost host/HTTP-only boundary")
	}
	return cookies[0]
}

func TestTeamBrowserHandoffProtectsOriginAndPreservesAppTraffic(t *testing.T) {
	seen := make(chan *http.Request, 1)
	_, h, token, team, browser, origin := teamBrowserFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen <- r.Clone(context.Background())
		w.Header().Add("Set-Cookie", teamBrowserCookie+" = application-forgery; Secure; Path=/")
		w.Header().Add("Set-Cookie", "app-session=legitimate; Path=/; HttpOnly")
		io.WriteString(w, "owned application")
	}))
	if w := browserRequest(browser, origin+"/", nil); w.Code != 401 {
		t.Fatal("known team URL bypassed membership cookie", w.Code)
	}
	handoff := issueBrowserHandoff(t, h, token, team, origin)
	cookie := consumeBrowserHandoff(t, browser, handoff)
	if w := browserRequest(browser, handoff, nil); w.Code != 403 {
		t.Fatal("single-use handoff replayed")
	}
	r := httptest.NewRequest("POST", origin+"/api/a%2Fb?q=1%2F2", strings.NewReader("app body"))
	r.AddCookie(cookie)
	r.AddCookie(&http.Cookie{Name: "app-session", Value: "keep-me"})
	r.Header.Set("Authorization", "Bearer application-token")
	r.Header.Set("X-Luci-Actor", "forged")
	r.Header.Set("Origin", origin)
	w := httptest.NewRecorder()
	browser.ServeHTTP(w, r)
	if w.Code != 200 || w.Body.String() != "owned application" {
		t.Fatalf("app blocked: %d %s", w.Code, w.Body.String())
	}
	app := <-seen
	if app.URL.EscapedPath() != "/api/a%2Fb" || app.URL.RawQuery != "q=1%2F2" || app.Header.Get("Authorization") != "Bearer application-token" || app.Header.Get("Cookie") != "app-session=keep-me" || app.Header.Get("X-Luci-Actor") != "" {
		t.Fatal("browser app/control boundary changed")
	}
	if cookies := w.Result().Cookies(); len(cookies) != 1 || cookies[0].Name != "app-session" {
		t.Fatal("application overwrote Luci's browser cookie")
	}
	for _, site := range []string{"same-site", "cross-site"} {
		r := httptest.NewRequest("GET", origin+"/mutation", nil)
		r.AddCookie(cookie)
		r.Header.Set("Sec-Fetch-Site", site)
		r.Header.Set("Sec-Fetch-Dest", "iframe")
		w := httptest.NewRecorder()
		browser.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatal("cross-origin GET bypassed project origin boundary", site, w.Code)
		}
	}
	request(t, h, "POST", "/v1/teams/"+team.ID+"/projects/p-other/browser-access", token, map[string]string{"url": origin}, 403)
	request(t, h, "POST", "/v1/teams/"+team.ID+"/projects/p-browser/browser-access", token, map[string]string{"url": "http://127.0.0.1:8730"}, 403)
}

func TestTeamBrowserLogoutRevokesHTTPAndPendingHandoff(t *testing.T) {
	_, h, token, team, browser, origin := teamBrowserFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "app") }))
	cookie := consumeBrowserHandoff(t, browser, issueBrowserHandoff(t, h, token, team, origin))
	pending := issueBrowserHandoff(t, h, token, team, origin)
	request(t, h, "POST", "/v1/auth/logout", token, nil, 200)
	if w := browserRequest(browser, origin+"/", cookie); w.Code != 403 {
		t.Fatal("logout left browser session active")
	}
	if w := browserRequest(browser, pending, nil); w.Code != 403 {
		t.Fatal("logout left pending browser handoff active")
	}
}

func TestTeamBrowserProjectGrantRevocationClosesLiveWebSocket(t *testing.T) {
	g, h, token, team, browser, origin := teamBrowserFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer conn.CloseNow()
		for {
			kind, data, err := conn.Read(r.Context())
			if err != nil {
				return
			}
			if conn.Write(r.Context(), kind, data) != nil {
				return
			}
		}
	}))
	member := addUser(t, h, token, "browser-editor", "member")
	memberToken := loginAs(t, h, member.Username)
	team = teamFromResponse(t, request(t, h, "POST", "/v1/teams/"+team.ID+"/members", token, teamMemberBody(team.Revision, member.Username, "editor", TeamGrants{Execute: true}), 200))
	g.opts.TeamHandler = func(context.Context, string) (http.Handler, error) {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			jsonReply(w, 200, map[string]any{"projects": []map[string]string{{"id": "p-browser"}}})
		}), nil
	}
	grantPath := "/v1/teams/" + team.ID + "/projects/p-browser/members/" + member.ID
	team = teamFromResponse(t, request(t, h, "PUT", grantPath, token, teamMemberBody(team.Revision, "", "editor", TeamGrants{Execute: true}), 200))
	cookie := consumeBrowserHandoff(t, browser, issueBrowserHandoff(t, h, memberToken, team, origin))
	server := httptest.NewServer(browser)
	defer server.Close()
	u, _ := url.Parse(server.URL)
	client := &http.Client{Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", u.Host)
	}}}
	defer client.CloseIdleConnections()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(origin, "http")+"/socket", &websocket.DialOptions{HTTPClient: client, HTTPHeader: http.Header{"Cookie": []string{cookie.String()}, "Origin": []string{origin}}})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	if err = conn.Write(ctx, websocket.MessageText, []byte("team app")); err != nil {
		t.Fatal(err)
	}
	if _, data, err := conn.Read(ctx); err != nil || string(data) != "team app" {
		t.Fatal("app websocket unavailable", err)
	}
	request(t, h, "DELETE", grantPath, token, map[string]any{"expectedRevision": team.Revision}, 200)
	if _, _, err = conn.Read(ctx); err == nil {
		t.Fatal("revocation retained live browser connection")
	}
	if w := browserRequest(browser, origin+"/", cookie); w.Code != 403 {
		t.Fatal("project revocation retained browser cookie")
	}
}

// The optional browser runner exercises real Chromium cookie rules, which an
// HTTP recorder cannot prove. It publishes only short-lived fixture handoffs in
// a private file and ends when the runner removes that file.
func TestTeamBrowserChromiumFixture(t *testing.T) {
	file := os.Getenv("LUCI_TEAM_BROWSER_FIXTURE_FILE")
	if file == "" {
		t.Skip("started by the local Chromium browser acceptance runner")
	}
	publicHTTPS := os.Getenv("LUCI_TEAM_BROWSER_PUBLIC_HTTPS") == "1"
	domain := ""
	if publicHTTPS {
		domain = "preview.projects.test"
	}
	dataOrigin := ""
	var foreignRequests atomic.Int32
	publicKey := strings.Repeat("a", 64)
	dataServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/realtime/v1/") {
			conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
			if err != nil {
				return
			}
			defer conn.CloseNow()
			for {
				kind, data, err := conn.Read(r.Context())
				if err != nil {
					return
				}
				if conn.Write(r.Context(), kind, data) != nil {
					return
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"data":"authorized"}`)
	}))
	defer dataServer.Close()
	g, h, token, team, _, origin := teamBrowserFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/cross-origin-") {
			foreignRequests.Add(1)
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<h1>Authenticated team application</h1><script>fetch('/asset').then(r=>r.text()).then(()=>document.body.dataset.asset='loaded');fetch(%q+'/rest/v1/notes',{credentials:'include',headers:{apikey:%q}}).then(r=>r.json()).then(data=>document.body.dataset.data=data.data);const socket=new WebSocket(%q+'/realtime/v1/websocket?apikey='+%q);socket.onopen=()=>socket.send('authorized realtime');socket.onmessage=event=>document.body.dataset.realtime=event.data;socket.onclose=()=>document.body.dataset.closed='true';</script>`, dataOrigin, publicKey, strings.Replace(strings.Replace(dataOrigin, "https:", "wss:", 1), "http:", "ws:", 1), publicKey)
	}))
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	registry, err := previewgateway.NewRegistryWithDomain(filepath.Join(g.dir, "teams", team.ID, "runtime", "state", "sessions"), port, domain)
	if err != nil {
		t.Fatal(err)
	}
	dataOrigin, err = registry.Publish(previewgateway.Target{Kind: "data", ProjectID: "p-browser", ResourceID: "data-browser", Upstream: dataServer.URL, Container: "owned-data", Labels: map[string]string{"luci.project-id": "p-browser", "luci.resource": "data-browser"}, PublishableKey: publicKey})
	if err != nil {
		t.Fatal(err)
	}
	browser, err := previewgateway.New(g.opts.StateDir, port, func(context.Context, previewgateway.Target) error { return nil })
	if publicHTTPS {
		if err := g.bindManagedTeam(team.ID); err != nil {
			t.Fatal(err)
		}
		browser, err = previewgateway.NewWithPublicIngress(g.opts.StateDir, port, func(context.Context, previewgateway.Target) error { return nil }, domain, g.ManagedTeamID())
	}
	if err != nil {
		t.Fatal(err)
	}
	g.SetBrowserGateway(browser)
	server := httptest.NewUnstartedServer(browser)
	server.Listener = listener
	server.Start()
	defer server.Close()
	if publicHTTPS {
		old, _ := url.Parse(origin)
		key := strings.ReplaceAll(strings.TrimSuffix(old.Hostname(), ".localhost"), ".", "")
		origin = "https://" + previewgateway.PublicCapabilityHostname(key, domain)
	} else {
		origin = strings.TrimSuffix(origin, "8731") + strconv.Itoa(port)
	}
	handoff := issueBrowserHandoff(t, h, token, team, origin)
	pageHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/foreign-count" {
			jsonReply(w, 200, map[string]int32{"count": foreignRequests.Load()})
			return
		}
		if r.URL.Path == "/revoke" {
			g.mu.Lock()
			owner := g.state.OwnerID
			g.mu.Unlock()
			request(t, h, "PUT", "/v1/teams/"+team.ID+"/members/"+owner, token, teamMemberBody(team.Revision, "", "owner", TeamGrants{Admin: true}), 200)
			w.WriteHeader(204)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		io.WriteString(w, `<iframe title="Team prototype" sandbox="allow-scripts allow-forms allow-same-origin" src="`+handoff+`"></iframe>`)
	})
	config := map[string]string{"origin": origin, "dataOrigin": dataOrigin}
	if publicHTTPS {
		upstream, _ := url.Parse(server.URL)
		proxy := httputil.NewSingleHostReverseProxy(upstream)
		front := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Host == "app.luci.test" {
				pageHandler.ServeHTTP(w, r)
			} else {
				proxy.ServeHTTP(w, r)
			}
		}))
		defer front.Close()
		pin := sha256.Sum256(front.Certificate().RawSubjectPublicKeyInfo)
		config["page"], config["tlsAddress"], config["tlsSPKI"] = "https://app.luci.test", front.Listener.Addr().String(), base64.StdEncoding.EncodeToString(pin[:])
	} else {
		page := httptest.NewServer(pageHandler)
		defer page.Close()
		config["page"] = page.URL
	}
	encoded, _ := json.Marshal(config)
	if err := os.WriteFile(file, encoded, 0600); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(45 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-deadline:
			t.Fatal("browser acceptance runner did not finish")
		case <-ticker.C:
			if _, err := os.Stat(file); os.IsNotExist(err) {
				return
			}
		}
	}
}
