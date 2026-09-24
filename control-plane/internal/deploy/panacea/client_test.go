package panacea

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"
)

const testToken = "pnc_test"

type staticTokens struct {
	tok string
	err error
}

func (s staticTokens) PanaceaToken(context.Context) (string, error) { return s.tok, s.err }

// sleeps records every wait do asks for and returns at once, so retry tests are fast and exact.
type sleeps struct {
	mu sync.Mutex
	d  []time.Duration
}

func (s *sleeps) sleep(_ context.Context, d time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.d = append(s.d, d)
	return nil
}

func (s *sleeps) got() []time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]time.Duration(nil), s.d...)
}

// logBuf is a concurrency-safe buffer for a Debug-level slog handler.
type logBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *logBuf) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *logBuf) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

type harness struct {
	c    *Client
	srv  *httptest.Server
	sl   *sleeps
	log  *logBuf
	hits atomic.Int32
}

// newHarness starts a TLS test server (ParseOrigin refuses http://, and tests never relax it)
// and a Client pointed at it, with a recorded sleep and a captured Debug log.
func newHarness(t *testing.T, h http.HandlerFunc, mods ...func(*Config)) *harness {
	t.Helper()
	hs := &harness{sl: &sleeps{}, log: &logBuf{}}
	hs.srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hs.hits.Add(1)
		h(w, r)
	}))
	t.Cleanup(hs.srv.Close)
	cfg := Config{
		Origin:     hs.srv.URL,
		Tokens:     staticTokens{tok: testToken},
		HTTPClient: hs.srv.Client(),
		Log:        slog.New(slog.NewTextHandler(hs.log, &slog.HandlerOptions{Level: slog.LevelDebug})),
		UserAgent:  "PanaceaApp/test",
	}
	for _, m := range mods {
		m(&cfg)
	}
	c, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	c.sleep = hs.sl.sleep
	hs.c = c
	return hs
}

func asDeployError(t *testing.T, err error) *deploy.Error {
	t.Helper()
	var e *deploy.Error
	if !errors.As(err, &e) || e == nil {
		t.Fatalf("error %v (%T) is not a *deploy.Error", err, err)
	}
	return e
}

func TestParseOriginTable(t *testing.T) {
	const (
		scheme   = "The Panacea address must start with https://."
		path     = "Give the address without a path: https://dev.panacea.kpmg, not https://dev.panacea.kpmg/api."
		userinfo = "The address must not contain a user name or password."
		noHost   = "That is not a web address."
	)
	ok := []struct{ in, want string }{
		{"https://dev.panacea.kpmg", "https://dev.panacea.kpmg"},
		{"HTTPS://Dev.Panacea.KPMG/", "https://dev.panacea.kpmg"},
		{"https://dev.panacea.kpmg:443", "https://dev.panacea.kpmg"},
		{"https://dev.panacea.kpmg:443/", "https://dev.panacea.kpmg"},
		{"https://dev.panacea.kpmg:8443", "https://dev.panacea.kpmg:8443"},
		{"  https://dev.panacea.kpmg  ", "https://dev.panacea.kpmg"},
		{"https://127.0.0.1:8790", "https://127.0.0.1:8790"},
		{"https://[::1]:8443", "https://[::1]:8443"},
		{"https://[::1]:443", "https://[::1]"},
	}
	for _, c := range ok {
		got, err := ParseOrigin(c.in)
		if err != nil || got != c.want {
			t.Errorf("ParseOrigin(%q) = %q, %v; want %q", c.in, got, err, c.want)
		}
	}
	bad := []struct{ in, want string }{
		{"http://dev.panacea.kpmg", scheme},
		{"ftp://dev.panacea.kpmg", scheme},
		{"dev.panacea.kpmg", scheme},
		{"https://dev.panacea.kpmg/api", path},
		{"https://dev.panacea.kpmg/api/v1", path},
		{"https://dev.panacea.kpmg?x=1", path},
		{"https://dev.panacea.kpmg/?", path},
		{"https://dev.panacea.kpmg#top", path},
		{"https://user:pw@dev.panacea.kpmg", userinfo},
		{"https://user@dev.panacea.kpmg", userinfo},
		{"https://", noHost},
		{"https://:443", noHost},
		{"", noHost},
		{"https:dev.panacea.kpmg", noHost},
		{"https://dev.panacea.kpmg:xyz", noHost},
	}
	for _, c := range bad {
		got, err := ParseOrigin(c.in)
		if err == nil {
			t.Errorf("ParseOrigin(%q) = %q, nil; want refusal %q", c.in, got, c.want)
			continue
		}
		e := asDeployError(t, err)
		if e.Kind != deploy.KindRejected || e.Message != c.want || got != "" {
			t.Errorf("ParseOrigin(%q) = %q, {Kind %q, Message %q}; want rejected %q", c.in, got, e.Kind, e.Message, c.want)
		}
	}
}

var uuidRE = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestBearerOnEveryRequest(t *testing.T) {
	type seen struct{ method, auth, cookie, origin, accept, ua, rid, ctype string }
	var mu sync.Mutex
	var got []seen
	hs := newHarness(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		got = append(got, seen{r.Method, r.Header.Get("Authorization"), r.Header.Get("Cookie"),
			r.Header.Get("Origin"), r.Header.Get("Accept"), r.Header.Get("User-Agent"),
			r.Header.Get("X-Request-Id"), r.Header.Get("Content-Type")})
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}, func(c *Config) {
		// A client carrying an SSO cookie for this host: the bearer must be the only credential.
		jar, _ := cookiejar.New(nil)
		u, _ := url.Parse(c.Origin)
		jar.SetCookies(u, []*http.Cookie{{Name: "_oauth2_proxy", Value: "sso-session"}})
		hc := *c.HTTPClient
		hc.Jar = jar
		c.HTTPClient = &hc
	})
	methods := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete}
	for _, m := range methods {
		var in any
		if m == http.MethodPost || m == http.MethodPut {
			in = map[string]string{"branch": "main"}
		}
		var out struct{ OK bool }
		if err := hs.c.do(context.Background(), "test", m, "/api/v1/projects/p", in, &out); err != nil || !out.OK {
			t.Fatalf("%s: err %v, out %+v", m, err, out)
		}
	}
	if len(got) != len(methods) {
		t.Fatalf("server saw %d requests, want %d", len(got), len(methods))
	}
	ids := map[string]bool{}
	for i, s := range got {
		if s.method != methods[i] || s.auth != "Bearer "+testToken {
			t.Errorf("%s: Authorization %q, want %q", s.method, s.auth, "Bearer "+testToken)
		}
		if s.cookie != "" || s.origin != "" {
			t.Errorf("%s: Cookie %q, Origin %q; want neither", s.method, s.cookie, s.origin)
		}
		if s.accept != "application/json" || s.ua != "PanaceaApp/test" {
			t.Errorf("%s: Accept %q, User-Agent %q", s.method, s.accept, s.ua)
		}
		if !uuidRE.MatchString(s.rid) || ids[s.rid] {
			t.Errorf("%s: X-Request-Id %q is not a fresh UUID", s.method, s.rid)
		}
		ids[s.rid] = true
		wantCT := ""
		if s.method == http.MethodPost || s.method == http.MethodPut {
			wantCT = "application/json"
		}
		if s.ctype != wantCT {
			t.Errorf("%s: Content-Type %q, want %q", s.method, s.ctype, wantCT)
		}
	}
}

func TestTokenNeverInErrors(t *testing.T) {
	for _, status := range []int{401, 403, 500} {
		hs := newHarness(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			_, _ = w.Write([]byte(`{"error":"api key ` + got + ` revoked or expired (also pnc_te)"}`))
		})
		err := hs.c.do(context.Background(), "whoami", http.MethodGet, "/api/v1/me", nil, nil)
		e := asDeployError(t, err)
		for what, s := range map[string]string{"Error()": err.Error(), "Message": e.Message, "log": hs.log.String()} {
			// "pnc_te" also catches the truncated key the body quotes.
			if strings.Contains(s, "pnc_te") || strings.Contains(s, "Bearer") {
				t.Errorf("%d: %s carries the credential: %s", status, what, s)
			}
		}
		if e.Cause == nil || strings.Contains(e.Cause.Error(), testToken) {
			t.Errorf("%d: Cause %v is nil or carries the credential", status, e.Cause)
		}
		if status < 500 && !strings.Contains(e.Message, "revoked or expired") {
			t.Errorf("%d: Message %q lost the sentence around the redaction", status, e.Message)
		}
		if !strings.Contains(hs.log.String(), "revoked or expired") {
			t.Errorf("%d: the redacted body is not in the Debug log: %s", status, hs.log.String())
		}
	}
}

func TestNotConnectedShortCircuits(t *testing.T) {
	storeErr := errors.New("credential manager: element not found")
	cases := []struct {
		name string
		src  staticTokens
		want func(error) bool
	}{
		{"ErrNotConnected", staticTokens{err: deploy.ErrNotConnected},
			func(err error) bool { return err == deploy.ErrNotConnected }},
		{"empty token", staticTokens{},
			func(err error) bool { return errors.Is(err, deploy.ErrNotConnected) }},
		{"store failure", staticTokens{err: storeErr},
			func(err error) bool {
				return deploy.KindOf(err) == deploy.KindNotConnected && errors.Is(err, storeErr)
			}},
	}
	for _, c := range cases {
		hs := newHarness(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}, func(cfg *Config) { cfg.Tokens = c.src })
		err := hs.c.do(context.Background(), "environments", http.MethodGet, "/api/v1/projects", nil, nil)
		if !c.want(err) {
			t.Errorf("%s: do = %v (kind %q)", c.name, err, deploy.KindOf(err))
		}
		if n := hs.hits.Load(); n != 0 {
			t.Errorf("%s: the server saw %d requests; want none", c.name, n)
		}
	}
}

// failThen answers the first n requests with status (and header h), then 200 {"ok":true}.
func failThen(n int32, status int, h map[string]string) (http.HandlerFunc, *atomic.Int32) {
	var calls atomic.Int32
	return func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) <= n {
			for k, v := range h {
				w.Header().Set(k, v)
			}
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"error":"upstream"}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}, &calls
}

func TestGetRetriedPostNot(t *testing.T) {
	for _, status := range []int{502, 503, 504} {
		h, calls := failThen(2, status, nil)
		hs := newHarness(t, h)
		var out struct{ OK bool }
		if err := hs.c.do(context.Background(), "environments", http.MethodGet, "/api/v1/projects", nil, &out); err != nil || !out.OK {
			t.Fatalf("GET after two %d: err %v, out %+v", status, err, out)
		}
		if calls.Load() != 3 {
			t.Errorf("GET after two %d: %d attempts, want 3", status, calls.Load())
		}
		if got := hs.sl.got(); len(got) != 2 || got[0] != 250*time.Millisecond || got[1] != time.Second {
			t.Errorf("GET after two %d: sleeps %v, want [250ms 1s]", status, got)
		}
	}
	// Three failures exhaust the two retries.
	h, calls := failThen(5, 503, nil)
	hs := newHarness(t, h)
	err := hs.c.do(context.Background(), "environments", http.MethodGet, "/api/v1/projects", nil, nil)
	if e := asDeployError(t, err); e.Kind != deploy.KindUnavailable || e.Status != 503 || calls.Load() != 3 {
		t.Errorf("GET after five 503: %v, status %d, %d attempts; want unavailable 503 after 3", err, e.Status, calls.Load())
	}
	for _, m := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		h, calls := failThen(2, 503, nil)
		hs := newHarness(t, h)
		err := hs.c.do(context.Background(), "ship", m, "/api/v1/projects/p/deploy", map[string]string{"branch": "main"}, nil)
		if deploy.KindOf(err) != deploy.KindUnavailable || calls.Load() != 1 || len(hs.sl.got()) != 0 {
			t.Errorf("%s after 503: %v, %d attempts, sleeps %v; want unavailable after exactly 1", m, err, calls.Load(), hs.sl.got())
		}
	}
	// Only a transport failure, 502, 503 and 504 are retried.
	for _, status := range []int{500, 429, 404, 401} {
		h, calls := failThen(2, status, nil)
		hs := newHarness(t, h)
		_ = hs.c.do(context.Background(), "environments", http.MethodGet, "/api/v1/projects", nil, nil)
		if calls.Load() != 1 {
			t.Errorf("GET after %d: %d attempts, want 1", status, calls.Load())
		}
	}
}

func TestGetRetryHonoursRetryAfter(t *testing.T) {
	get := func(hs *harness) error {
		return hs.c.do(context.Background(), "environments", http.MethodGet, "/api/v1/projects", nil, nil)
	}
	cases := []struct {
		name      string
		status    int
		ra        string
		wantCalls int32
		wantSleep []time.Duration
		wantRA    time.Duration // on the error; -1 = success expected
	}{
		{"503 Retry-After 1 waits 1s, not 250ms", 503, "1", 2, []time.Duration{time.Second}, -1},
		{"503 Retry-After 2 is the longest waited", 503, "2", 2, []time.Duration{2 * time.Second}, -1},
		{"503 Retry-After 0 keeps the fixed delay", 503, "0", 2, []time.Duration{250 * time.Millisecond}, -1},
		{"503 Retry-After 3 ends the retries", 503, "3", 1, nil, 3 * time.Second},
		{"503 Retry-After 30 ends the retries", 503, "30", 1, nil, 30 * time.Second},
		{"429 Retry-After 5 is never retried", 429, "5", 1, nil, 5 * time.Second},
		{"502 Retry-After is ignored", 502, "30", 2, []time.Duration{250 * time.Millisecond}, -1},
	}
	for _, c := range cases {
		h, calls := failThen(1, c.status, map[string]string{"Retry-After": c.ra})
		hs := newHarness(t, h)
		err := get(hs)
		if c.wantRA < 0 {
			if err != nil {
				t.Errorf("%s: %v, want success", c.name, err)
			}
		} else if e := asDeployError(t, err); e.Kind != deploy.KindUnavailable || e.RetryAfter != c.wantRA || e.Status != c.status {
			t.Errorf("%s: {%q %d RetryAfter %v}, want unavailable %d RetryAfter %v", c.name, e.Kind, e.Status, e.RetryAfter, c.status, c.wantRA)
		}
		if calls.Load() != c.wantCalls {
			t.Errorf("%s: %d attempts, want %d", c.name, calls.Load(), c.wantCalls)
		}
		if got := hs.sl.got(); len(got) != len(c.wantSleep) || (len(got) > 0 && got[0] != c.wantSleep[0]) {
			t.Errorf("%s: sleeps %v, want %v", c.name, got, c.wantSleep)
		}
	}
	// An HTTP-date Retry-After more than 2 s away also ends the retries.
	when := time.Now().Add(time.Minute).UTC().Format(http.TimeFormat)
	h, calls := failThen(1, 503, map[string]string{"Retry-After": when})
	hs := newHarness(t, h)
	if e := asDeployError(t, get(hs)); e.RetryAfter < 50*time.Second || e.RetryAfter > time.Minute || calls.Load() != 1 {
		t.Errorf("HTTP-date Retry-After: RetryAfter %v after %d attempts, want ~1m after 1", e.RetryAfter, calls.Load())
	}
}

// resetThen hijacks and closes the first n connections without answering, then answers 200.
func resetThen(n int32) (http.HandlerFunc, *atomic.Int32) {
	var calls atomic.Int32
	return func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) <= n {
			conn, _, err := w.(http.Hijacker).Hijack()
			if err == nil {
				_ = conn.Close()
			}
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}, &calls
}

func TestGetRetriedOnReset(t *testing.T) {
	h, calls := resetThen(2)
	hs := newHarness(t, h)
	var out struct{ OK bool }
	if err := hs.c.do(context.Background(), "environments", http.MethodGet, "/api/v1/projects", nil, &out); err != nil || !out.OK {
		t.Fatalf("GET after two resets: %v, %+v", err, out)
	}
	if calls.Load() != 3 {
		t.Errorf("GET after two resets: %d attempts, want 3", calls.Load())
	}
	h, calls = resetThen(2)
	hs = newHarness(t, h)
	err := hs.c.do(context.Background(), "ship", http.MethodPost, "/api/v1/projects/p/deploy", map[string]string{}, nil)
	if e := asDeployError(t, err); e.Kind != deploy.KindUnavailable || e.Status != 0 || e.Message != "" || calls.Load() != 1 {
		t.Errorf("POST after a reset: {%q %d %q}, %d attempts; want unavailable 0 after 1", e.Kind, e.Status, e.Message, calls.Load())
	}
}

// cutThen answers the first n requests 200 with a body cut short mid-stream (the connection is
// aborted after 5 of 100 promised bytes), then 200 {"ok":true}.
func cutThen(n int32) (http.HandlerFunc, *atomic.Int32) {
	var calls atomic.Int32
	return func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) <= n {
			w.Header().Set("Content-Length", "100")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok"`))
			w.(http.Flusher).Flush()
			panic(http.ErrAbortHandler)
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}, &calls
}

func TestGetRetriedOnCutBody(t *testing.T) {
	h, calls := cutThen(1)
	hs := newHarness(t, h)
	var out struct{ OK bool }
	if err := hs.c.do(context.Background(), "environments", http.MethodGet, "/api/v1/projects", nil, &out); err != nil || !out.OK {
		t.Fatalf("GET after a cut body: %v, %+v; want the retry to succeed", err, out)
	}
	if calls.Load() != 2 {
		t.Errorf("GET after a cut body: %d attempts, want 2", calls.Load())
	}
	// A POST is never retried; its cut 200 is unavailable with Status 200: Panacea answered.
	h, calls = cutThen(1)
	hs = newHarness(t, h)
	err := hs.c.do(context.Background(), "ship", http.MethodPost, "/api/v1/projects/p/deploy", map[string]string{}, &out)
	if e := asDeployError(t, err); e.Kind != deploy.KindUnavailable || e.Status != 200 || calls.Load() != 1 {
		t.Errorf("POST with a cut body: {%q %d} after %d attempts; want unavailable 200 after 1", e.Kind, e.Status, calls.Load())
	}
}

func TestOutUntouchedWhenBodyDoesNotDecode(t *testing.T) {
	hs := newHarness(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"name":"filled","count":"not a number"}`))
	})
	var out struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	err := hs.c.do(context.Background(), "ship", http.MethodPost, "/api/v1/projects/p/deploy", nil, &out)
	if deploy.KindOf(err) != deploy.KindUnavailable || out.Name != "" {
		t.Errorf("a 200 that half-decodes: %v, out %+v; want unavailable and out untouched", err, out)
	}
}

func TestTimeoutPerAttempt(t *testing.T) {
	var calls atomic.Int32
	slowFirst := func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 || r.Method == http.MethodPost {
			select {
			case <-r.Context().Done():
			case <-time.After(200 * time.Millisecond):
			}
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}
	hs := newHarness(t, slowFirst, func(c *Config) { c.Timeout = 50 * time.Millisecond })
	start := time.Now()
	err := hs.c.do(context.Background(), "ship", http.MethodPost, "/api/v1/projects/p/deploy", map[string]string{}, nil)
	if e := asDeployError(t, err); e.Kind != deploy.KindUnavailable || e.Status != 0 || time.Since(start) > time.Second {
		t.Errorf("POST to a 200 ms server with a 50 ms timeout: {%q %d} after %v", e.Kind, e.Status, time.Since(start))
	}
	// The timeout bounds each attempt, not the loop: the first GET attempt times out, the
	// retry gets a fresh budget and succeeds. 150 ms leaves room for a TLS handshake under -race.
	calls.Store(0)
	slowOnce := func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			select {
			case <-r.Context().Done():
			case <-time.After(600 * time.Millisecond):
			}
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}
	hs = newHarness(t, slowOnce, func(c *Config) { c.Timeout = 150 * time.Millisecond })
	var out struct{ OK bool }
	if err := hs.c.do(context.Background(), "environments", http.MethodGet, "/api/v1/projects", nil, &out); err != nil || !out.OK {
		t.Errorf("GET whose first attempt times out: %v, %+v; want the retry to succeed", err, out)
	}
}

// versionServer serves h at /api/v1/version and counts every request it sees.
func versionServer(t *testing.T, h http.HandlerFunc) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		h(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func TestRedirectNotFollowed(t *testing.T) {
	srv, hits := versionServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/version" {
			http.Redirect(w, r, "/oauth2/sign_in?rd=%2Fapi%2Fv1%2Fversion", http.StatusTemporaryRedirect)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html>Sign in with Entra</html>"))
	})
	// srv.Client() follows redirects by default: Version must not, whichever client it is given.
	v, err := Version(context.Background(), srv.Client(), srv.URL)
	if v != "" || !errors.Is(err, ErrSignInRedirect) {
		t.Fatalf("Version = %q, %v; want ErrSignInRedirect", v, err)
	}
	e := asDeployError(t, err)
	if e.Kind != deploy.KindRejected || e.Message != ErrSignInRedirect.Message || e.Status != 307 || e.Op != "version" {
		t.Errorf("redirect error = %+v; want rejected, ErrSignInRedirect's sentence, Status 307", e)
	}
	if ErrSignInRedirect.Status != 0 || ErrSignInRedirect.Cause != nil {
		t.Errorf("the shared ErrSignInRedirect was mutated: %+v", ErrSignInRedirect)
	}
	if n := hits.Load(); n != 1 {
		t.Errorf("the server saw %d requests, want 1", n)
	}
}

func TestVersionDecodes(t *testing.T) {
	var auth atomic.Value
	body := `{"version":"v1.163.0"}`
	var mu sync.Mutex
	srv, hits := versionServer(t, func(w http.ResponseWriter, r *http.Request) {
		auth.Store(r.Header.Get("Authorization") + "|" + r.Method + " " + r.URL.Path)
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	})
	v, err := Version(context.Background(), srv.Client(), srv.URL)
	if err != nil || v != "v1.163.0" {
		t.Fatalf("Version = %q, %v; want v1.163.0", v, err)
	}
	if got := auth.Load(); got != "|GET /api/v1/version" {
		t.Errorf("Version sent %q; want no Authorization and GET /api/v1/version", got)
	}
	const notAPI = "That address answered, but not as the Panacea API."
	for _, b := range []string{`{"version":""}`, `{}`, `{"version":5}`, `<html>portal</html>`, ``, `null`} {
		mu.Lock()
		body = b
		mu.Unlock()
		v, err := Version(context.Background(), srv.Client(), srv.URL)
		if v != "" || err == nil {
			t.Errorf("body %q: Version = %q, %v; want the not-the-API error", b, v, err)
			continue
		}
		if e := asDeployError(t, err); e.Kind != deploy.KindRejected || e.Message != notAPI || e.Status != 200 {
			t.Errorf("body %q: %+v; want rejected %q with Status 200", b, e, notAPI)
		}
	}
	// One attempt, never retried: a 503 is answered at once.
	hits.Store(0)
	srv503, hits503 := versionServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	if _, err := Version(context.Background(), srv503.Client(), srv503.URL); deploy.KindOf(err) != deploy.KindUnavailable || hits503.Load() != 1 {
		t.Errorf("503: %v after %d requests; want unavailable after 1", err, hits503.Load())
	}
	// An address ParseOrigin refuses is refused before any request.
	if _, err := Version(context.Background(), srv.Client(), srv.URL+"/api"); deploy.KindOf(err) != deploy.KindRejected || hits.Load() != 0 {
		t.Errorf("origin with a path: %v after %d requests; want rejected before any", err, hits.Load())
	}
}

func TestVersionSendsNoCookie(t *testing.T) {
	var cookie atomic.Value
	srv, _ := versionServer(t, func(w http.ResponseWriter, r *http.Request) {
		cookie.Store(r.Header.Get("Cookie"))
		_, _ = w.Write([]byte(`{"version":"v1.163.0"}`))
	})
	jar, _ := cookiejar.New(nil)
	u, _ := url.Parse(srv.URL)
	jar.SetCookies(u, []*http.Cookie{{Name: "_oauth2_proxy", Value: "sso-session"}})
	hc := srv.Client()
	hc.Jar = jar
	if _, err := Version(context.Background(), hc, srv.URL); err != nil {
		t.Fatal(err)
	}
	if got := cookie.Load(); got != "" {
		t.Errorf("Version sent Cookie %q; an SSO cookie must never prove an address is the API", got)
	}
}

func TestCAFileAddsRoot(t *testing.T) {
	srv, _ := versionServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"version":"v1.163.0"}`))
	})
	dir := t.TempDir()
	caPath := filepath.Join(dir, "panacea-ca.pem")
	block := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
	if err := os.WriteFile(caPath, block, 0o600); err != nil {
		t.Fatal(err)
	}
	withCA, err := BuildHTTPClient(caPath, 5*time.Second)
	if err != nil {
		t.Fatalf("BuildHTTPClient(ca): %v", err)
	}
	if v, err := Version(context.Background(), withCA, srv.URL); err != nil || v != "v1.163.0" {
		t.Fatalf("through the CA file: %q, %v; want v1.163.0", v, err)
	}
	plain, err := BuildHTTPClient("", 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Version(context.Background(), plain, srv.URL)
	var unknown x509.UnknownAuthorityError
	if !errors.As(err, &unknown) || deploy.KindOf(err) != deploy.KindUnavailable {
		t.Fatalf("without the CA file: %v (kind %q); want an x509 unknown-authority unavailable", err, deploy.KindOf(err))
	}
	// A CA file that cannot be applied is refused, never ignored.
	if _, err := BuildHTTPClient(filepath.Join(dir, "missing.pem"), 0); err == nil || !strings.Contains(err.Error(), "missing.pem") {
		t.Errorf("missing CA file: %v; want an error naming it", err)
	}
	junk := filepath.Join(dir, "junk.pem")
	_ = os.WriteFile(junk, []byte("not a certificate"), 0o600)
	if _, err := BuildHTTPClient(junk, 0); err == nil || !strings.Contains(err.Error(), "junk.pem") {
		t.Errorf("CA file with no certificate: %v; want an error naming it", err)
	}
}

func TestProxyFromEnvironmentHonoured(t *testing.T) {
	hc, err := BuildHTTPClient("", 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	tr, ok := hc.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport is %T, want *http.Transport", hc.Transport)
	}
	def := http.DefaultTransport.(*http.Transport)
	if tr == def || tr.Proxy == nil || !tr.ForceAttemptHTTP2 {
		t.Errorf("transport: shared %v, Proxy nil %v, ForceAttemptHTTP2 %v", tr == def, tr.Proxy == nil, tr.ForceAttemptHTTP2)
	}
	if tr.IdleConnTimeout != def.IdleConnTimeout || tr.TLSHandshakeTimeout != def.TLSHandshakeTimeout ||
		tr.MaxIdleConns != def.MaxIdleConns || tr.ExpectContinueTimeout != def.ExpectContinueTimeout {
		t.Errorf("transport dropped DefaultTransport's defaults: idle %v/%v handshake %v/%v max %d/%d continue %v/%v",
			tr.IdleConnTimeout, def.IdleConnTimeout, tr.TLSHandshakeTimeout, def.TLSHandshakeTimeout,
			tr.MaxIdleConns, def.MaxIdleConns, tr.ExpectContinueTimeout, def.ExpectContinueTimeout)
	}
	tc := tr.TLSClientConfig
	// Verification being on is TestCAFileAddsRoot's: without the CA file the handshake must fail.
	if tc == nil || tc.MinVersion != tls.VersionTLS12 || tc.RootCAs != nil {
		t.Errorf("TLSClientConfig = %+v; want MinVersion TLS 1.2 and the platform roots", tc)
	}
	if hc.Timeout != 5*time.Second || hc.Jar != nil || hc.CheckRedirect == nil ||
		hc.CheckRedirect(nil, nil) != http.ErrUseLastResponse {
		t.Errorf("client: Timeout %v, Jar %v, CheckRedirect never-follow %v", hc.Timeout, hc.Jar, hc.CheckRedirect != nil)
	}

	// ProxyFromEnvironment reads the environment once per process, so a child process runs Version.
	var mu sync.Mutex
	var seen []string
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.Method+" "+r.Host)
		mu.Unlock()
		conn, buf, err := w.(http.Hijacker).Hijack()
		if err != nil {
			return
		}
		_, _ = buf.WriteString("HTTP/1.1 502 P7 proxy saw it\r\nContent-Length: 0\r\nConnection: close\r\n\r\n")
		_ = buf.Flush()
		_ = conn.Close()
	}))
	t.Cleanup(proxy.Close)
	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperProxyChild$", "-test.count=1", "-test.v")
	cmd.Env = append(os.Environ(), "P7_PROXY_CHILD=1", "http_proxy=", "HTTP_PROXY=", "no_proxy=", "NO_PROXY=",
		"https_proxy="+proxy.URL, "HTTPS_PROXY="+proxy.URL)
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "--- PASS: TestHelperProxyChild") {
		t.Fatalf("child: %v\n%s", err, out)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(seen) != 1 || seen[0] != "CONNECT panacea.invalid:443" {
		t.Errorf("the proxy saw %q; want exactly [CONNECT panacea.invalid:443]", seen)
	}
}

// TestHelperProxyChild is TestProxyFromEnvironmentHonoured's child. It skips unless run as one.
func TestHelperProxyChild(t *testing.T) {
	if os.Getenv("P7_PROXY_CHILD") != "1" {
		t.Skip("child of TestProxyFromEnvironmentHonoured")
	}
	hc, err := BuildHTTPClient("", 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err = Version(ctx, hc, "https://panacea.invalid")
	var e *deploy.Error
	if !errors.As(err, &e) || e.Kind != deploy.KindUnavailable || e.Cause == nil ||
		!strings.Contains(e.Cause.Error(), "P7 proxy saw it") {
		t.Fatalf("Version through the proxy = %v (cause %v); want the proxy's 502", err, causeOf(e))
	}
}

func causeOf(e *deploy.Error) error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func TestNewValidatesConfig(t *testing.T) {
	tokens := staticTokens{tok: testToken}
	c, err := New(Config{Origin: "HTTPS://Dev.Panacea.KPMG/", Tokens: tokens})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c.cfg.Origin != "https://dev.panacea.kpmg" || c.timeout != 15*time.Second || c.http.Timeout != 15*time.Second {
		t.Errorf("New kept origin %q, timeout %v, client timeout %v; want canonical, 15s, 15s", c.cfg.Origin, c.timeout, c.http.Timeout)
	}
	if c.log == nil || c.sleep == nil || c.cfg.UserAgent == "" {
		t.Errorf("New left log %v, sleep nil %v, user agent %q", c.log, c.sleep == nil, c.cfg.UserAgent)
	}
	refused := []struct {
		name string
		cfg  Config
		want string
	}{
		{"origin with a path", Config{Origin: "https://dev.panacea.kpmg/api", Tokens: tokens}, "without a path"},
		{"no origin", Config{Tokens: tokens}, "not a web address"},
		{"no token source", Config{Origin: "https://dev.panacea.kpmg"}, "credential"},
		{"negative timeout", Config{Origin: "https://dev.panacea.kpmg", Tokens: tokens, Timeout: -time.Second}, "timeout"},
		{"unreadable CA file", Config{Origin: "https://dev.panacea.kpmg", Tokens: tokens,
			CAFile: filepath.Join(t.TempDir(), "absent.pem")}, "absent.pem"},
		{"a test client would drop the CA file", Config{Origin: "https://dev.panacea.kpmg", Tokens: tokens,
			CAFile: "ca.pem", HTTPClient: &http.Client{}}, "CAFile"},
	}
	for _, r := range refused {
		if c, err := New(r.cfg); err == nil || c != nil || !strings.Contains(err.Error(), r.want) {
			t.Errorf("%s: New = %v, %v; want an error containing %q", r.name, c, err, r.want)
		}
	}
}

func TestDoRefusesPathOutsideAPI(t *testing.T) {
	hs := newHarness(t, func(w http.ResponseWriter, r *http.Request) {})
	for _, p := range []string{"/api/v2/projects", "api/v1/projects", "//evil.example/api/v1/", ""} {
		if err := hs.c.do(context.Background(), "test", http.MethodGet, p, nil, nil); err == nil {
			t.Errorf("do(%q) = nil; want a refusal", p)
		}
	}
	if n := hs.hits.Load(); n != 0 {
		t.Errorf("the server saw %d requests; want none", n)
	}
}

func TestPathSentVerbatim(t *testing.T) {
	var uri atomic.Value
	hs := newHarness(t, func(w http.ResponseWriter, r *http.Request) {
		uri.Store(r.RequestURI)
		_, _ = w.Write([]byte(`{}`))
	})
	p := "/api/v1/projects/p/environments/" + url.PathEscape("feat/x") + "/pods?limit=5"
	if err := hs.c.do(context.Background(), "pods", http.MethodGet, p, nil, nil); err != nil {
		t.Fatal(err)
	}
	if got := uri.Load(); got != p {
		t.Errorf("request URI %q, want %q (an escaped / must stay escaped)", got, p)
	}
}

func TestUnusable2xxIsUnavailable(t *testing.T) {
	big := `{"x":"` + strings.Repeat("a", maxBody) + `"}`
	cases := []struct {
		name   string
		status int
		body   string
		ok     []int
		out    bool
		fails  bool
	}{
		{"HTML 200 into out", 200, "<html>portal</html>", nil, true, true},
		{"empty 200 into out", 200, "", nil, true, true},
		{"body over 8 MiB", 200, big, nil, true, true},
		{"200 where only 202 is accepted", 200, `{"ok":true}`, []int{202}, true, true},
		{"202 where 202 is accepted", 202, `{"ok":true}`, []int{202}, true, false},
		{"204 with no out", 204, "", nil, false, false},
		{"HTML 200 with no out", 200, "<html>", nil, false, false},
	}
	for _, c := range cases {
		hs := newHarness(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(c.status)
			_, _ = w.Write([]byte(c.body))
		})
		var out map[string]any
		var dst any
		if c.out {
			dst = &out
		}
		err := hs.c.do(context.Background(), "ship", http.MethodPost, "/api/v1/projects/p/deploy", nil, dst, c.ok...)
		if !c.fails {
			if err != nil {
				t.Errorf("%s: %v, want success", c.name, err)
			}
			continue
		}
		e := asDeployError(t, err)
		if e.Kind != deploy.KindUnavailable || e.Status != c.status || e.Op != "ship" || e.Message != "" || e.Cause == nil {
			t.Errorf("%s: %+v; want unavailable, Status %d, Op ship, a Cause", c.name, e, c.status)
		}
		if out != nil {
			t.Errorf("%s: out = %v; want nothing handed back", c.name, out)
		}
	}
}

func TestOneLogLinePerFailedCall(t *testing.T) {
	hs := newHarness(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-Id", "pan-req-77")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"kubernetes client unavailable"}`))
	})
	_ = hs.c.do(context.Background(), "environments", http.MethodGet, "/api/v1/projects/p", nil, nil)
	var warns []string
	for _, l := range strings.Split(hs.log.String(), "\n") {
		if strings.Contains(l, "level=WARN") {
			warns = append(warns, l)
		}
	}
	if len(warns) != 1 {
		t.Fatalf("%d WARN lines for one failed call (3 attempts); want 1:\n%s", len(warns), hs.log.String())
	}
	for _, want := range []string{"op=environments", "method=GET", "path=/api/v1/projects/p", "status=503",
		"panacea_request_id=pan-req-77", "duration=", "attempts=3"} {
		if !strings.Contains(warns[0], want) {
			t.Errorf("WARN line lacks %q: %s", want, warns[0])
		}
	}
	if strings.Contains(warns[0], "kubernetes client unavailable") {
		t.Errorf("the body reached the WARN line; it belongs at Debug: %s", warns[0])
	}
	// A call that succeeds logs nothing at Warn.
	ok := newHarness(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{}`)) })
	_ = ok.c.do(context.Background(), "environments", http.MethodGet, "/api/v1/projects/p", nil, nil)
	if strings.Contains(ok.log.String(), "level=WARN") {
		t.Errorf("a successful call logged a warning: %s", ok.log.String())
	}
}
