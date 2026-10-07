package product

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func testPreviewBridge(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set("X-Luci-Preview-Bridge", "1")
		next.ServeHTTP(w, r)
	})
}

func previewInspection(row storedRelease) string {
	u, _ := url.Parse(row.URL)
	b, _ := json.Marshal(map[string]any{"running": true, "labels": map[string]string{"luci.preview": "true", "luci.release-id": row.ID, "luci.project-id": row.ProjectID}, "ports": map[string]any{"3000/tcp": []map[string]string{{"HostIP": "127.0.0.1", "HostPort": u.Port()}}}})
	return string(b)
}

func seedVerifiedPreview(t *testing.T, s *Server, release Release) {
	t.Helper()
	row := storedRelease{Release: release, ContainerID: strings.Repeat("a", 64)}
	if err := s.storeRelease(row); err != nil {
		t.Fatal(err)
	}
	s.runner = func(ctx context.Context, exe string, args []string, dir string) (string, error) {
		want := []string{"inspect", "--format", `{"running":{{json .State.Running}},"labels":{{json .Config.Labels}},"ports":{{json .NetworkSettings.Ports}}}`, row.ContainerID}
		if exe != "docker" || !reflect.DeepEqual(args, want) || dir != "" {
			t.Error("preview verification inspected more than owned state, labels and bindings")
			return "", errors.New("unexpected command")
		}
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > 3*time.Second {
			t.Error("preview verification has no bounded request deadline")
		}
		return previewInspection(row), nil
	}
}

func TestPreviewProxyRechecksOwnedLiveContainerBeforeAnyUpstreamTraffic(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerPreviewProxy(mux)
	p := createJourneyProject(t, mux, "proxy-owner")
	var upstream atomic.Int32
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { upstream.Add(1); w.WriteHeader(200) }))
	defer app.Close()
	bridge := httptest.NewServer(testPreviewBridge(mux))
	defer bridge.Close()
	row := storedRelease{Release: Release{ID: "rel-owner", ProjectID: p.ID, Target: "local", Status: "ready", URL: app.URL}, ContainerID: strings.Repeat("a", 64)}
	u, _ := url.Parse(app.URL)
	good := previewInspection(row)
	for _, tc := range []struct {
		name, id, snapshot string
		err                error
	}{
		{"missing-container", "", good, nil},
		{"invalid-container", "--help", good, nil},
		{"container-missing", row.ContainerID, "", errors.New("not found")},
		{"malformed-inspection", row.ContainerID, "invalid", nil},
		{"stopped", row.ContainerID, strings.Replace(good, `"running":true`, `"running":false`, 1), nil},
		{"wrong-project", row.ContainerID, strings.Replace(good, p.ID, "foreign-project", 1), nil},
		{"wrong-release", row.ContainerID, strings.Replace(good, row.ID, "foreign-release", 1), nil},
		{"not-preview", row.ContainerID, strings.Replace(good, `"luci.preview":"true"`, `"luci.preview":"false"`, 1), nil},
		{"port-rebound", row.ContainerID, strings.Replace(good, `"HostPort":"`+u.Port()+`"`, `"HostPort":"1"`, 1), nil},
		{"public-binding", row.ContainerID, strings.Replace(good, `"HostIP":"127.0.0.1"`, `"HostIP":"0.0.0.0"`, 1), nil},
		{"multiple-bindings", row.ContainerID, strings.Replace(good, `"3000/tcp":[`, `"3000/tcp":[{"HostIP":"127.0.0.1","HostPort":"1"},`, 1), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := row
			fixture.ContainerID = tc.id
			if err := s.storeRelease(fixture); err != nil {
				t.Fatal(err)
			}
			var inspections int
			s.runner = func(_ context.Context, exe string, args []string, _ string) (string, error) {
				inspections++
				if exe != "docker" || len(args) != 4 || args[0] != "inspect" || args[3] != tc.id {
					t.Error("unexpected container action")
				}
				return tc.snapshot, tc.err
			}
			base := "/v1/product/projects/" + p.ID + "/releases/" + row.ID + "/preview/"
			journeyRequest(t, testPreviewBridge(mux), "POST", base+"write", map[string]string{"payload": "must not arrive"}, 409)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			conn, response, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(bridge.URL, "http")+base+"socket", nil)
			if conn != nil {
				conn.CloseNow()
			}
			if err == nil || response == nil || response.StatusCode != 409 {
				t.Fatal("unowned WebSocket upgrade was not refused")
			}
			if upstream.Load() != 0 {
				t.Fatal("unowned/rebound app received a request")
			}
			if (tc.id == "" || tc.id == "--help") && inspections != 0 {
				t.Fatal("invalid container identity reached Docker")
			}
		})
	}
}

func TestPreviewProxyForwardsRecordedAppMethodsPathsAndCookies(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerPreviewProxy(mux)
	forward := testPreviewBridge(mux)
	p := createJourneyProject(t, mux, "proxy-app")
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.NewEncoder(w).Encode(map[string]string{"method": r.Method, "path": r.URL.EscapedPath(), "query": r.URL.RawQuery, "body": string(body), "cookie": r.Header.Get("Cookie"), "authorization": r.Header.Get("Authorization"), "bridge": r.Header.Get("X-Luci-Token"), "forwarded": r.Header.Get("X-Forwarded-For"), "origin": r.Header.Get("Origin"), "host": r.Host})
	}))
	defer app.Close()
	seedVerifiedPreview(t, s, Release{ID: "rel-proxy", ProjectID: p.ID, Target: "local", Status: "ready", URL: app.URL})
	base := "/v1/product/projects/" + p.ID + "/releases/rel-proxy/preview/"
	journeyRequest(t, mux, "GET", base, nil, 403)
	browser := httptest.NewRequest("GET", base, nil)
	browser.Header.Set("Sec-Fetch-Site", "same-origin")
	browserResponse := httptest.NewRecorder()
	forward.ServeHTTP(browserResponse, browser)
	if browserResponse.Code != 403 {
		t.Fatalf("control-origin browser request accepted: %d", browserResponse.Code)
	}
	for _, method := range []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"} {
		t.Run(method, func(t *testing.T) {
			r := httptest.NewRequest(method, base+"api/a%2Fb?q=one%20two", strings.NewReader("app payload"))
			r.Header.Set("Cookie", "appSession=abc")
			r.Header.Set("Authorization", "Bearer control-secret")
			r.Header.Set("X-Luci-Token", "bridge-secret")
			r.Header.Set("X-Forwarded-For", "fake-host")
			r.Header.Set("Origin", "http://127.0.0.1:12345")
			w := httptest.NewRecorder()
			forward.ServeHTTP(w, r)
			var got map[string]string
			if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &got) != nil {
				t.Fatalf("response %d: %s", w.Code, w.Body.String())
			}
			if got["method"] != method || got["path"] != "/api/a%2Fb" || got["query"] != "q=one%20two" || got["body"] != "app payload" || got["cookie"] != "appSession=abc" || got["origin"] != app.URL || got["host"] != strings.TrimPrefix(app.URL, "http://") {
				t.Fatalf("app request changed: %#v", got)
			}
			if got["authorization"] != "" || got["bridge"] != "" || got["forwarded"] != "" {
				t.Fatalf("control credentials forwarded: %#v", got)
			}
		})
	}
	w := journeyRequest(t, forward, "GET", base, nil, 200)
	if !strings.Contains(w.Body.String(), `"path":"/"`) {
		t.Fatalf("root path not forwarded: %s", w.Body.String())
	}
	journeyRequest(t, forward, "TRACE", base, nil, 405)
	journeyRequest(t, forward, "HEAD", base, nil, 200)
}

func TestPreviewProxyKeepsAppRedirectsOnBridgeAndReportsUnavailableApp(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerPreviewProxy(mux)
	forward := testPreviewBridge(mux)
	p := createJourneyProject(t, mux, "proxy-redirect")
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		location := "http://" + r.Host + "/login?next=%2F#form"
		if r.URL.Path == "/external" {
			location = "https://accounts.example.com/sign-in"
		}
		w.Header().Set("Location", location)
		w.WriteHeader(http.StatusFound)
	}))
	defer app.Close()
	seedVerifiedPreview(t, s, Release{ID: "rel-redirect", ProjectID: p.ID, Target: "local", Status: "ready", URL: app.URL})
	base := "/v1/product/projects/" + p.ID + "/releases/rel-redirect/preview/"
	w := journeyRequest(t, forward, "GET", base, nil, 302)
	if w.Header().Get("Location") != "/login?next=%2F#form" {
		t.Fatalf("app redirect escaped bridge: %q", w.Header().Get("Location"))
	}
	w = journeyRequest(t, forward, "GET", base+"external", nil, 302)
	if w.Header().Get("Location") != "https://accounts.example.com/sign-in" {
		t.Fatalf("external redirect changed: %q", w.Header().Get("Location"))
	}
	app.Close()
	journeyRequest(t, forward, "GET", base, nil, 502)
}

func TestPreviewProxyRejectsMissingStoppedCloudAndMalformedRecordedOrigins(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerPreviewProxy(mux)
	forward := testPreviewBridge(mux)
	p := createJourneyProject(t, mux, "proxy-validation")
	base := "/v1/product/projects/" + p.ID + "/releases/rel-test/preview/"
	journeyRequest(t, forward, "GET", base, nil, 404)
	for _, tc := range []struct{ name, target, status, url string }{
		{"stopped", "local", "stopped", "http://127.0.0.1:3100"},
		{"cloud", "cloud", "ready", "http://127.0.0.1:3100"},
		{"remote", "local", "ready", "http://example.com:3100"},
		{"localhost", "local", "ready", "http://localhost:3100"},
		{"ipv6", "local", "ready", "http://[::1]:3100"},
		{"https", "local", "ready", "https://127.0.0.1:3100"},
		{"credentials", "local", "ready", "http://token@127.0.0.1:3100"},
		{"query", "local", "ready", "http://127.0.0.1:3100?upstream=bad"},
		{"path", "local", "ready", "http://127.0.0.1:3100/other"},
		{"fragment", "local", "ready", "http://127.0.0.1:3100#fragment"},
		{"port-zero", "local", "ready", "http://127.0.0.1:0"},
		{"port-overflow", "local", "ready", "http://127.0.0.1:65536"},
		{"port-ambiguous", "local", "ready", "http://127.0.0.1:03100"},
		{"control", "local", "ready", "http://127.0.0.1:8730"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := s.storeRelease(storedRelease{Release: Release{ID: "rel-test", ProjectID: p.ID, Target: tc.target, Status: tc.status, URL: tc.url}}); err != nil {
				t.Fatal(err)
			}
			journeyRequest(t, forward, "POST", base, nil, 409)
		})
	}
}

func TestPreviewProxyCarriesWebSocketFrames(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerPreviewProxy(mux)
	p := createJourneyProject(t, mux, "proxy-websocket")
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/socket" {
			http.NotFound(w, r)
			return
		}
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		kind, data, err := conn.Read(ctx)
		if err == nil {
			_ = conn.Write(ctx, kind, data)
		}
	}))
	defer app.Close()
	seedVerifiedPreview(t, s, Release{ID: "rel-ws", ProjectID: p.ID, Target: "local", Status: "ready", URL: app.URL})
	bridge := httptest.NewServer(testPreviewBridge(mux))
	defer bridge.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	address := "ws" + strings.TrimPrefix(bridge.URL, "http") + "/v1/product/projects/" + p.ID + "/releases/rel-ws/preview/socket"
	conn, _, err := websocket.Dial(ctx, address, &websocket.DialOptions{HTTPHeader: http.Header{"Origin": []string{bridge.URL}}})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	if err := conn.Write(ctx, websocket.MessageText, []byte("interactive preview")); err != nil {
		t.Fatal(err)
	}
	_, data, err := conn.Read(ctx)
	if err != nil || string(data) != "interactive preview" {
		t.Fatalf("websocket did not pass through: %q %v", data, err)
	}
}
