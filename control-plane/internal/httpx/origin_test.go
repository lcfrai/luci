package httpx

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// policy builds a policy whose log the test can read back. Passing no origins
// exercises the "empty means DefaultOrigins" rule.
func policy(t *testing.T, allowed ...string) (*OriginPolicy, *bytes.Buffer) {
	t.Helper()
	var logs bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return NewOriginPolicy(allowed, log), &logs
}

// guard wraps a handler that records whether it was reached. A middleware bug
// that lets a rejected request through is the failure that matters most here,
// and it is invisible unless something downstream says "I ran".
func guard(t *testing.T, p *OriginPolicy, reached *bool) http.Handler {
	t.Helper()
	return p.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*reached = true
		WriteJSON(w, http.StatusOK, map[string]string{"ok": "yes"})
	}))
}

func request(method, origin string) *http.Request {
	req := httptest.NewRequest(method, "http://127.0.0.1:8730/v1/machines", nil)
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	return req
}

func TestDefaultOriginsCoverEveryTauriBuild(t *testing.T) {
	// Contract 2.12 derives these five from tauri's own source. If one is
	// dropped, exactly one platform's packaged build stops working and no
	// other test in this repository notices.
	want := []string{
		"http://tauri.localhost",
		"https://tauri.localhost",
		"tauri://localhost",
		"http://localhost:1420",
		"http://127.0.0.1:1420",
	}
	got := DefaultOrigins()
	if len(got) != len(want) {
		t.Fatalf("got %d origins %q, want %d %q", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("origin %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestAllowMatchesTheWholeOriginNotJustTheHost(t *testing.T) {
	p, _ := policy(t)
	cases := []struct {
		origin string
		want   bool
		why    string
	}{
		{"http://tauri.localhost", true, "packaged on Windows"},
		{"https://tauri.localhost", true, "packaged on Windows with useHttpsScheme"},
		{"tauri://localhost", true, "packaged on macOS and Linux"},
		{"http://localhost:1420", true, "the vite dev server"},
		{"http://127.0.0.1:1420", true, "the same dev server by address"},
		{"", true, "curl, the Go test client, anything that is not a browser"},
		{"https://localhost:1420", false, "right host and port, wrong scheme"},
		{"http://localhost:5173", false, "right host, wrong port"},
		{"http://tauri.localhost.evil.example", false, "suffix, not a match"},
		{"null", false, "a sandboxed iframe or a file:// page"},
		{"http://evil.example", false, "the whole point"},
	}
	for _, c := range cases {
		if got := p.Allow(c.origin); got != c.want {
			t.Errorf("Allow(%q) = %v, want %v (%s)", c.origin, got, c.want, c.why)
		}
	}
}

func TestAnExplicitListReplacesTheDefaultsAndIsNormalised(t *testing.T) {
	// Contract section 4: PANACEA_APP_ALLOWED_ORIGINS replaces the defaults, it
	// does not extend them, because "extend" makes lock-down impossible. The
	// surrounding space, trailing slash and capitals are what an operator
	// actually pastes out of a browser address bar.
	p, _ := policy(t, "  HTTP://Tauri.Localhost/  ")

	if !p.Allow("http://tauri.localhost") {
		t.Error("a configured origin was rejected after normalisation")
	}
	if p.Allow("http://localhost:1420") {
		t.Error("an explicit list extended the defaults instead of replacing them")
	}
}

func TestAnAllowlistThatReducesToNothingStaysEmpty(t *testing.T) {
	// Decision (f). An operator who configured something and got nothing back
	// gets a warning and a locked-down server, not five origins they did not
	// ask for silently restored.
	p, logs := policy(t, "   ")

	if got := p.Allowed(); len(got) != 0 {
		t.Errorf("Allowed() = %q, want an empty list", got)
	}
	if p.Allow("http://tauri.localhost") {
		t.Error("a default origin was allowed by a policy configured with none")
	}
	if !p.Allow("") {
		t.Error("a non-browser caller must still be allowed; it sends no Origin to check")
	}
	if !strings.Contains(logs.String(), "empty") {
		t.Errorf("log = %q, want a warning that the allowlist is empty", logs.String())
	}
}

func TestMiddlewareGrantsAnAllowedOrigin(t *testing.T) {
	p, _ := policy(t)
	var reached bool
	rec := httptest.NewRecorder()

	guard(t, p, &reached).ServeHTTP(rec, request(http.MethodGet, "http://localhost:1420"))

	if !reached {
		t.Fatalf("the handler was never reached: %d %s", rec.Code, rec.Body.String())
	}
	if got, want := rec.Header().Get("Access-Control-Allow-Origin"), "http://localhost:1420"; got != want {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q echoed verbatim", got, want)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got == "*" {
		t.Error("the allowlist is the single decision point; a wildcard discards it")
	}
	if got := rec.Header().Get("Vary"); !strings.Contains(got, "Origin") {
		t.Errorf("Vary = %q, want it to contain Origin so no cache serves one origin's response to another", got)
	}
	// No cookies are used anywhere on this API, and this header with an echoed
	// origin would make the allowlist the only thing between a rejected page
	// and an authenticated one.
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Errorf("Access-Control-Allow-Credentials = %q, want it absent", got)
	}
}

func TestMiddlewareAnswersPreflightBeforeTheMux(t *testing.T) {
	// Without this the mux answers 405 and every POST, PATCH and DELETE from
	// the cockpit fails before its real request is ever sent.
	p, _ := policy(t)
	var reached bool
	req := request(http.MethodOptions, "http://tauri.localhost")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "content-type")
	rec := httptest.NewRecorder()

	guard(t, p, &reached).ServeHTTP(rec, req)

	if reached {
		t.Error("the preflight reached the mux; it must be answered by the middleware")
	}
	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	for header, want := range map[string]string{
		"Access-Control-Allow-Origin":  "http://tauri.localhost",
		"Access-Control-Allow-Methods": "GET, POST, PATCH, DELETE, OPTIONS",
		"Access-Control-Allow-Headers": "Content-Type",
		"Access-Control-Max-Age":       "600",
	} {
		if got := rec.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
	if rec.Body.Len() != 0 {
		t.Errorf("204 carried %d bytes of body", rec.Body.Len())
	}
}

func TestMiddlewareRejectsAPreflightFromADisallowedOrigin(t *testing.T) {
	// The security-relevant half of the preflight path. Answering 204 with the
	// CORS headers to an origin the allowlist rejects would hand a hostile page
	// exactly the permission the allowlist exists to withhold. Without this,
	// hoisting the OPTIONS branch above the origin check is a silent mutation:
	// the other preflight test uses an allowed origin and the other rejection
	// tests use GET, so neither ever reaches the combination.
	p, _ := policy(t)
	var reached bool
	req := request(http.MethodOptions, "http://evil.example")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "content-type")
	rec := httptest.NewRecorder()

	guard(t, p, &reached).ServeHTTP(rec, req)

	if reached {
		t.Fatal("a rejected preflight reached the mux")
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	var problem Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("body %q is not a Problem: %v", rec.Body.String(), err)
	}
	if problem.Error != CodeForbiddenOrigin {
		t.Errorf("error = %q, want %q", problem.Error, CodeForbiddenOrigin)
	}
	for _, header := range []string{
		"Access-Control-Allow-Origin",
		"Access-Control-Allow-Methods",
		"Access-Control-Allow-Headers",
		"Access-Control-Max-Age",
	} {
		if got := rec.Header().Get(header); got != "" {
			t.Errorf("the rejected preflight was granted %s: %q", header, got)
		}
	}
}

func TestMiddlewareLetsAnOptionsWithNoOriginReachTheMux(t *testing.T) {
	// Deliberate: an OPTIONS with no Origin is not a preflight, because a real
	// preflight always carries one. Answering it in the middleware would bury a
	// genuine routing question behind something that knows nothing about routes.
	// Deleting the `&& origin != ""` is otherwise silent - nothing else sends an
	// OPTIONS without an Origin.
	p, _ := policy(t)
	var reached bool
	rec := httptest.NewRecorder()

	guard(t, p, &reached).ServeHTTP(rec, request(http.MethodOptions, ""))

	if !reached {
		t.Fatalf("an OPTIONS with no Origin was answered by the middleware: %d %s",
			rec.Code, rec.Body.String())
	}
	if rec.Code == http.StatusNoContent {
		t.Error("status = 204; the middleware preflighted a request that is not a preflight")
	}
}

func TestMiddlewareRejectsAnUnknownOriginAndSaysWhatIsAllowed(t *testing.T) {
	p, logs := policy(t)
	var reached bool
	rec := httptest.NewRecorder()

	guard(t, p, &reached).ServeHTTP(rec, request(http.MethodGet, "http://evil.example"))

	if reached {
		t.Fatal("a rejected origin reached the mux")
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("a rejected origin was granted Access-Control-Allow-Origin %q", got)
	}
	var problem Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("body %q is not a Problem: %v", rec.Body.String(), err)
	}
	if problem.Error != CodeForbiddenOrigin {
		t.Errorf("error = %q, want %q", problem.Error, CodeForbiddenOrigin)
	}
	if problem.Details["origin"] != "http://evil.example" {
		t.Errorf("details = %v, want the rejected origin", problem.Details)
	}
	// The allowlist goes to the log, not to the body: the body is read by the
	// page that was just rejected.
	if strings.Contains(rec.Body.String(), "tauri.localhost") {
		t.Error("the response body disclosed the allowlist to the rejected caller")
	}
	line := logs.String()
	for _, want := range []string{"http://evil.example", "http://tauri.localhost", "http://localhost:1420"} {
		if !strings.Contains(line, want) {
			t.Errorf("log %q does not name %q", line, want)
		}
	}
}

func TestMiddlewareRejectsANonLoopbackHostButLetsTheBrowserReadWhy(t *testing.T) {
	// DNS rebinding: evil.example resolves to 127.0.0.1, so the connection is
	// local and only the Host header gives it away.
	p, logs := policy(t)
	var reached bool
	req := request(http.MethodGet, "http://tauri.localhost")
	req.Host = "panacea.evil.example"
	rec := httptest.NewRecorder()

	guard(t, p, &reached).ServeHTTP(rec, req)

	if reached {
		t.Fatal("a rebound Host reached the mux")
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	var problem Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("body %q is not a Problem: %v", rec.Body.String(), err)
	}
	if problem.Error != CodeForbiddenHost {
		t.Errorf("error = %q, want %q", problem.Error, CodeForbiddenHost)
	}
	// Decision (e): the origin was fine, so the grant stands and a misconfigured
	// PANACEA_APP_ADDR shows up in the console as this JSON rather than as an
	// opaque CORS failure.
	if got, want := rec.Header().Get("Access-Control-Allow-Origin"), "http://tauri.localhost"; got != want {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, want)
	}
	if !strings.Contains(logs.String(), "panacea.evil.example") {
		t.Errorf("log %q does not name the rejected Host", logs.String())
	}
}

func TestMiddlewareGrantsNothingWhenThereIsNoOrigin(t *testing.T) {
	p, _ := policy(t)
	var reached bool
	rec := httptest.NewRecorder()

	guard(t, p, &reached).ServeHTTP(rec, request(http.MethodGet, ""))

	if !reached {
		t.Fatalf("curl was rejected: %d %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, want it absent when there is no origin to echo", got)
	}
	if got := rec.Header().Get("Vary"); !strings.Contains(got, "Origin") {
		t.Errorf("Vary = %q, want Origin even here, or a cache serves this response to an origin", got)
	}
}

func TestLoopbackHost(t *testing.T) {
	cases := map[string]bool{
		"127.0.0.1:8730":              true,
		"127.0.0.1":                   true, // no port, HTTP/1.0 or a hand-built request
		"127.0.0.53:8730":             true, // all of 127.0.0.0/8 is loopback
		"127.0.0.53":                  true, // the same, without a port
		"localhost:8730":              true,
		"LocalHost:8730":              true, // Host is case-insensitive
		"localhost.:8730":             true, // the fully-qualified form a resolver may produce
		"[::1]:8730":                  true,
		"[::1]":                       true,
		"::1":                         true,
		"::ffff:127.0.0.1":            true,  // IPv4-mapped IPv6; it really is loopback
		"":                            false, // no browser omits it; nothing is lost by refusing
		"evil.example":                false,
		"evil.example:8730":           false,
		"localhost.evil.example:8730": false, // prefix, not a match
		"localhost.evil.com":          false, // the same trick without a port
		"127.0.0.1.evil.com":          false, // the address as a LABEL, not an address
		"127.0.0.1.evil.com:8730":     false, // and with a port, which splits off cleanly
		"[2001:db8::1]:8730":          false, // a real IPv6 address that is not loopback
		"[::1%25eth0]":                false, // a zone makes ParseIP fail, so this fails closed
		"0177.0.0.1":                  false, // octal-looking IPv4; Go 1.17+ refuses it outright
		"2130706433":                  false, // 127.0.0.1 as a bare integer is not an IP to Go
		"0.0.0.0:8730":                false,
		"192.168.1.10:8730":           false,
	}
	for host, want := range cases {
		if got := LoopbackHost(host); got != want {
			t.Errorf("LoopbackHost(%q) = %v, want %v", host, got, want)
		}
	}
}
