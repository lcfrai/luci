### Task 11: gatekeeper proxy: allowlist, `model` rewrite, headers, streaming

This is the half of the gatekeeper that touches bytes. Task 10 decides whether a request may pass and on which route; this task turns that decision into one forwarded request to LiteLLM and one relayed response, and it has to do so while changing **exactly one JSON value**. Everything Claude Code sends that we do not understand, a field added in next month's release, a beta header nobody here has heard of, must reach LiteLLM unchanged, because OD-20 has the cloud track the latest CLI and the gateway docs forbid the alternative: *"Forward `anthropic-version` and `anthropic-beta` unchanged … don't allowlist individual values, because the set changes with Claude Code releases"* (https://code.claude.com/docs/en/llm-gateway-protocol, as read by the skeleton on 2026-09-25; **not re-read here**, the corporate network was not tried).

**The one thing this task must not get wrong is that the route is enforced on the value LiteLLM will actually read.** LiteLLM picks the upstream from the body's `model`. If a sandbox can get a model name past the rewrite, it chooses its own route, and an AU project leaves Australia. Three ways to do that are closed here, each by a test: naming a LiteLLM group directly (`"model": "global/claude-opus-4-8"` is not a public id, so `Resolve` refuses it); sending `model` twice (Go's decoder and Python's `json` both keep the **last** duplicate, so rewriting the first would forward the attacker's second; a duplicate is refused); and spelling the key with an escape (`"model"` decodes to `model` in both languages, so the key is compared **decoded**).

Five decisions where the skeleton is silent, made here so task 12 does not re-derive them.

- **(a) The rewrite splices, it does not re-encode.** The skeleton says "decode as `map[string]json.RawMessage` … re-encode". `json.Marshal` of a `RawMessage` is not byte-preserving: it compacts whitespace and, with its default HTML escaping, turns `<` in a prompt into `\u003c`. The value is equivalent JSON but not the same bytes, and the skeleton's own test ("keeps it byte for byte") would fail. So `rewriteModel` walks the top level with `json.Decoder.Token`, finds the byte range of the one `model` string, and replaces that range in the original slice. Every other byte, including key order and whitespace, is the sender's. The map decode survives only as the check that the body is one JSON object.
- **(b) `Config.Catalogue` is the interface `Resolver`**, which `*catalogue.Catalogue` satisfies (a compile-time assertion pins it). The skeleton names the concrete type; the interface lets these tests run on a two-line fake instead of on whatever `models.yaml` holds this month. Task 12 passes the concrete catalogue unchanged.
- **(c) Credential-shaped headers are stripped, not only `x-litellm-*`.** The skeleton forwards "the credential as `Authorization: Bearer` alone". This task makes that literal: `Authorization`, `x-api-key`, `api-key`, `x-goog-api-key`, `ocp-apim-subscription-key`, `proxy-authorization` and `cookie` are all removed before the one `Authorization` is set. The reason is pod binding. A sandbox could present its own bound token in `Authorization` to pass task 10, and a second session's stolen token in `api-key`; if LiteLLM prefers that header (its header precedence is **UNVERIFIED** against the pinned image, task 18), the stolen key spends without its pod. Stripping costs nothing.
- **(d) `x-litellm-*` and `litellm-*` are stripped from responses too.** LiteLLM answers with headers such as its model `api_base` and the key's spend (names from training knowledge, **UNVERIFIED**). They tell a sandbox the internal topology and invite tooling to depend on a component it must never reach. The rest of the response, body included, is relayed as LiteLLM sent it: an error body echoes the caller's own request back to the caller, which is not a disclosure.
- **(e) The proxy reports each request through `Config.Observe(Outcome)`**, once, after the response ends, and `Outcome` has no field that can hold a body or a header value. Task 12's logger is that callback, so "bodies are never logged" (Global constraint 3) is a property of a type, not a habit.

**Files:**
- `control-plane/internal/gatekeeper/proxy/proxy.go` — `Config`, `Decider`, `Resolver`, `Outcome`, `New`, `RequestFrom`, header hygiene, the `ReverseProxy` (new)
- `control-plane/internal/gatekeeper/proxy/rewrite.go` — `rewriteModel`, the splice, its errors (new)
- `control-plane/internal/gatekeeper/proxy/proxy_test.go` — every test below, `package proxy` (new)

**Interfaces:**
- Produces:
  - `proxy.Config{Upstream *url.URL; Catalogue Resolver; Authz Decider; MaxBody int64; Now func() time.Time; Observe func(Outcome); Transport http.RoundTripper}`; zero `MaxBody` means `DefaultMaxBody` (64 MiB), nil `Now` means `time.Now`, nil `Transport` means `NewTransport()`
  - `proxy.Decider` = `interface{ Decide(ctx context.Context, r authz.Request) authz.Decision }`, which task 10's `*authz.Authorizer` satisfies
  - `proxy.Resolver` = `interface{ Resolve(publicID string, r dataclass.Route) (group string, ok bool) }`, which task 02's `*catalogue.Catalogue` satisfies
  - `proxy.Outcome{Session, Project string; Route dataclass.Route; Path string; Status int; Elapsed time.Duration}`
  - `proxy.New(Config) (http.Handler, error)`: an error for a nil `Upstream`, `Catalogue` or `Authz`, or an `Upstream` with a path, a query or a scheme other than `http`
  - `proxy.RequestFrom(r *http.Request, now time.Time) authz.Request`: the source address from `r.RemoteAddr` alone, IPv4-mapped addresses unmapped; task 12 reuses it for `/v1/models`
  - `proxy.NewTransport() *http.Transport`: no environment proxy, no response-header deadline
  - `proxy.AllowedPaths` = `[]string{"/v1/messages", "/v1/messages/count_tokens"}`
- Consumes: `authz.Request`, `authz.Decision` (task 10); `dataclass.Route`, `RouteGlobal`, `RouteAU` (task 01); `wire.CodePathNotAllowed`, `wire.CodeModelNotOffered` (task 03; the Go identifiers are this part's assumption, see the gap list in the return); `httpx.WriteProblem`, `httpx.CodeInvalidJSON`, `httpx.CodeBackendUnavailable` (`control-plane/internal/httpx/problem.go:45`, `:60`, `:93`).
- Required by: task 12 (mounts `New` behind `/v1/models` and `/api/hello`, logs `Outcome`, reuses `RequestFrom`), task 17 (the local stack), task 18 (P08 decides whether this rewrite also drops `metadata.tags`).

- [ ] **Step 1: Write the failing tests**

Every test runs the handler against an `httptest` fake of LiteLLM that records what arrived. The Decider and the Resolver are fakes, so nothing here depends on task 10's store or on the catalogue's current ids. The streaming test uses a **real** listener (`httptest.NewServer`), not a `ResponseRecorder`: a recorder never buffers, so a proxy that hides `Flush` would pass against it.

Create `control-plane/internal/gatekeeper/proxy/proxy_test.go`:

```go
package proxy

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/dataclass"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/gatekeeper/authz"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/wire"
)

// allow says yes on one route, with the session token task 10 recovered.
type allow struct{ route dataclass.Route }

func (a allow) Decide(context.Context, authz.Request) authz.Decision {
	return authz.Decision{Allow: true, Status: http.StatusOK, Route: a.route, Token: "sk-pa-decided",
		Binding: wire.Binding{SessionID: "s-1", ProjectID: "p-1"}}
}

type deny struct {
	status int
	code   string
}

func (d deny) Decide(context.Context, authz.Request) authz.Decision {
	return authz.Decision{Status: d.status, Code: d.code}
}

// cat offers one public id, on either route.
type cat struct{}

func (cat) Resolve(id string, r dataclass.Route) (string, bool) {
	if id != "claude-opus-4-8" {
		return "", false
	}
	return string(r) + "/" + id, true
}

type arrival struct {
	path, query string
	header      http.Header
	body        []byte
}

type seen struct {
	mu  sync.Mutex
	got []arrival
}

func (s *seen) all() []arrival { s.mu.Lock(); defer s.mu.Unlock(); return append([]arrival(nil), s.got...) }

// upstream is a fake LiteLLM. A nil h answers a small JSON body and one
// LiteLLM-style response header that the proxy must not relay.
func upstream(t *testing.T, h http.HandlerFunc) (*url.URL, *seen) {
	t.Helper()
	s := &seen{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		s.mu.Lock()
		s.got = append(s.got, arrival{r.URL.Path, r.URL.RawQuery, r.Header.Clone(), b})
		s.mu.Unlock()
		if h != nil {
			h(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Litellm-Model-Api-Base", "http://fakeanthropic-global:4900")
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	return u, s
}
```

The request helpers and the first tests, in the same file:

```go
func gate(t *testing.T, up *url.URL, d Decider, mut ...func(*Config)) http.Handler {
	t.Helper()
	c := Config{Upstream: up, Catalogue: cat{}, Authz: d}
	for _, m := range mut {
		m(&c)
	}
	h, err := New(c)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

const minimal = `{"model":"claude-opus-4-8","max_tokens":1,"messages":[]}`

func post(h http.Handler, method, target, body string, hdr map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://gatekeeper:4000"+target, strings.NewReader(body))
	r.RemoteAddr = "10.244.1.7:51000"
	r.Header.Set("Authorization", "Bearer sk-pa-test")
	for k, v := range hdr {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func problemCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var p httpx.Problem
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatalf("body %q is not a Problem: %v", w.Body.String(), err)
	}
	return p.Error
}

// Kills a prefix match (strings.HasPrefix(path, "/v1/messages")), which would
// forward /v1/messages/batches, a storing API that P6-D9 keeps out.
func TestOnlyTheTwoMessagesPathsAreForwarded(t *testing.T) {
	up, s := upstream(t, nil)
	h := gate(t, up, allow{dataclass.RouteGlobal})
	for _, tc := range []struct {
		method, path string
		want         int
	}{
		{http.MethodPost, "/v1/messages", 200},
		{http.MethodPost, "/v1/messages/count_tokens", 200},
		{http.MethodPost, "/v1/files", 404},
		{http.MethodPost, "/v1/messages/batches", 404},
		{http.MethodPost, "/v1/messages/", 404},
		{http.MethodPost, "/v1/messages/count_tokens/x", 404},
		{http.MethodPost, "/v1/messagesx", 404},
		{http.MethodPost, "/v1/messages/../files", 404},
		{http.MethodGet, "/v1/messages", 404},
		{http.MethodPost, "/key/generate", 404},
	} {
		w := post(h, tc.method, tc.path, minimal, nil)
		if w.Code != tc.want {
			t.Errorf("%s %s = %d, want %d", tc.method, tc.path, w.Code, tc.want)
		}
		if tc.want == 404 && problemCode(t, w) != wire.CodePathNotAllowed {
			t.Errorf("%s %s: code %q, want %q", tc.method, tc.path, problemCode(t, w), wire.CodePathNotAllowed)
		}
	}
	if n := len(s.all()); n != 2 {
		t.Errorf("upstream saw %d requests, want exactly the 2 allowed ones", n)
	}
}

// Kills building the upstream URL from Path alone: Claude Code posts to
// /v1/messages?beta=true and the docs say to keep the query.
func TestTheQueryIsKept(t *testing.T) {
	up, s := upstream(t, nil)
	post(gate(t, up, allow{dataclass.RouteGlobal}), http.MethodPost, "/v1/messages?beta=true", minimal, nil)
	if got := s.all()[0]; got.path != "/v1/messages" || got.query != "beta=true" {
		t.Errorf("upstream saw %s?%s, want /v1/messages?beta=true", got.path, got.query)
	}
}
```

The rewrite tests. The first body has odd whitespace, a field no one has invented yet, a number with a trailing zero and HTML-significant characters, because each is something `json.Marshal` would change:

```go
// Kills re-encoding the body (json.Marshal sorts keys, compacts whitespace,
// writes 1.50 as 1.5 through float64, and escapes < and & as \u003c, \u0026)
// and kills ignoring the route (an AU decision must send au/<id>).
func TestRewriteChangesOnlyTheModelValue(t *testing.T) {
	in := "{\n  \"stream\" : true,\n  \"model\":\t\"claude-opus-4-8\",\n" +
		"  \"future_field\": {\"b\":1, \"a\" : [ 1.50 ]},\n  \"system\": \"<b>&</b>\"\n}"
	for _, route := range []dataclass.Route{dataclass.RouteGlobal, dataclass.RouteAU} {
		up, s := upstream(t, nil)
		w := post(gate(t, up, allow{route}), http.MethodPost, "/v1/messages", in, nil)
		if w.Code != 200 {
			t.Fatalf("%s: status %d, body %s", route, w.Code, w.Body)
		}
		want := strings.Replace(in, `"claude-opus-4-8"`, `"`+string(route)+`/claude-opus-4-8"`, 1)
		if got := string(s.all()[0].body); got != want {
			t.Errorf("%s: upstream body\n%s\nwant\n%s", route, got, want)
		}
	}
}

// Each row is a way for a sandbox to pick its own model group, and so its own
// route. None may reach upstream.
func TestTheRouteCannotBeChosenByTheBody(t *testing.T) {
	for name, tc := range map[string]struct {
		body string
		code string
	}{
		// Kills resolving an id that is already a group name.
		"a group named directly": {`{"model":"global/claude-opus-4-8"}`, wire.CodeModelNotOffered},
		// Kills rewriting the first "model" only: Go and Python keep the last.
		"model twice": {`{"model":"claude-opus-4-8","model":"global/claude-opus-4-8"}`, httpx.CodeInvalidJSON},
		// Kills comparing the raw key bytes instead of the decoded key.
		"escaped key": {`{"model":"claude-opus-4-8","\u006dodel":"global/claude-opus-4-8"}`, httpx.CodeInvalidJSON},
		"no model":           {`{"max_tokens":1}`, wire.CodeModelNotOffered},
		"model not a string": {`{"model":["claude-opus-4-8"]}`, wire.CodeModelNotOffered},
		"unknown model":      {`{"model":"claude-unreleased-9"}`, wire.CodeModelNotOffered},
		"not an object":      {`["claude-opus-4-8"]`, httpx.CodeInvalidJSON},
		// Kills stopping at the first value: a second object would be sent on.
		"trailing value": {`{"model":"claude-opus-4-8"} {"model":"global/x"}`, httpx.CodeInvalidJSON},
		"truncated":      {`{"model":"claude-opus-4-8",`, httpx.CodeInvalidJSON},
	} {
		t.Run(name, func(t *testing.T) {
			up, s := upstream(t, nil)
			w := post(gate(t, up, allow{dataclass.RouteAU}), http.MethodPost, "/v1/messages", tc.body, nil)
			if w.Code != http.StatusBadRequest || problemCode(t, w) != tc.code {
				t.Errorf("got %d %s, want 400 %s", w.Code, w.Body, tc.code)
			}
			if n := len(s.all()); n != 0 {
				t.Errorf("upstream saw %d requests, want 0", n)
			}
		})
	}
}

// Kills an unbounded read: the limit is the proxy's, not the upstream's.
func TestABodyOverTheLimitIsRefused(t *testing.T) {
	up, s := upstream(t, nil)
	h := gate(t, up, allow{dataclass.RouteGlobal}, func(c *Config) { c.MaxBody = 64 })
	w := post(h, http.MethodPost, "/v1/messages", `{"model":"claude-opus-4-8","pad":"`+strings.Repeat("x", 100)+`"}`, nil)
	if w.Code != http.StatusRequestEntityTooLarge || len(s.all()) != 0 {
		t.Errorf("got %d with %d upstream calls, want 413 and none", w.Code, len(s.all()))
	}
}

// Kills forwarding before authorising, and kills a proxy that writes its own
// status instead of task 10's.
func TestADenialIsTask10sStatusAndCode(t *testing.T) {
	up, s := upstream(t, nil)
	w := post(gate(t, up, deny{http.StatusForbidden, "binding_mismatch"}), http.MethodPost, "/v1/messages", minimal, nil)
	if w.Code != http.StatusForbidden || problemCode(t, w) != "binding_mismatch" || len(s.all()) != 0 {
		t.Errorf("got %d %s with %d upstream calls", w.Code, w.Body, len(s.all()))
	}
}
```

Headers, both directions. One request carries every header the proxy must pass, replace or drop, and each assertion names the mutant it kills:

```go
func TestHeadersArePassedReplacedOrDropped(t *testing.T) {
	up, s := upstream(t, nil)
	w := post(gate(t, up, allow{dataclass.RouteGlobal}), http.MethodPost, "/v1/messages", minimal, map[string]string{
		"Anthropic-Version":  "2023-06-01",
		"Anthropic-Beta":     "some-future-beta-2027,context-management-2025-06-27",
		"X-Api-Key":          "sk-pa-test",
		"Api-Key":            "sk-pa-another-sessions-key",
		"X-Litellm-Tags":     "team:finance",
		"X-Litellm-Api-Key":  "sk-guess",
		"Litellm-Anything":   "1",
		"Cookie":             "a=b",
		"X-Forwarded-For":    "10.9.9.9",
		"X-Claude-Code-Test": "kept",
	})
	if w.Code != 200 {
		t.Fatalf("status %d: %s", w.Code, w.Body)
	}
	h := s.all()[0].header
	for _, c := range []struct{ name, want, kills string }{
		{"Anthropic-Version", "2023-06-01", "dropping or pinning the version"},
		{"Anthropic-Beta", "some-future-beta-2027,context-management-2025-06-27", "a beta allowlist, which the docs forbid"},
		{"Authorization", "Bearer sk-pa-decided", "forwarding the caller's header instead of the decision's token"},
		{"X-Api-Key", "", "a second credential header LiteLLM might prefer"},
		{"Api-Key", "", "smuggling another session's key past pod binding"},
		{"X-Litellm-Tags", "", "spoofed attribution"},
		{"X-Litellm-Api-Key", "", "a LiteLLM control header"},
		{"Litellm-Anything", "", "a prefix list without litellm-"},
		{"Cookie", "", "relaying a credential-shaped header"},
		{"X-Forwarded-For", "", "a sandbox-chosen source address reaching LiteLLM"},
		{"X-Claude-Code-Test", "kept", "a header-name allowlist that would break the next CLI release"},
	} {
		if got := h.Get(c.name); got != c.want {
			t.Errorf("upstream %s = %q, want %q (kills %s)", c.name, got, c.want, c.kills)
		}
	}
	// Kills relaying LiteLLM's internal headers to the sandbox (decision d).
	if got := w.Header().Get("X-Litellm-Model-Api-Base"); got != "" {
		t.Errorf("response carried X-Litellm-Model-Api-Base %q", got)
	}
}

// Kills buffering. The fake sends the first event, then waits until the
// client has read it before sending the second. A proxy that buffers, or a
// response wrapper that hides Flush from ReverseProxy, holds the first event
// in the server's write buffer and the read below times out.
func TestAStreamIsRelayedEventByEvent(t *testing.T) {
	release := make(chan struct{})
	var once sync.Once
	free := func() { once.Do(func() { close(release) }) }
	defer free()
	up, _ := upstream(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "event: message_start\ndata: {}\n\n")
		w.(http.Flusher).Flush()
		<-release
		_, _ = io.WriteString(w, "event: message_stop\ndata: {}\n\n")
	})
	srv := httptest.NewServer(gate(t, up, allow{dataclass.RouteGlobal}))
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/v1/messages", "application/json", strings.NewReader(minimal))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	first := make(chan string, 1)
	br := bufio.NewReader(resp.Body)
	go func() { line, _ := br.ReadString('\n'); first <- line }()
	select {
	case line := <-first:
		if line != "event: message_start\n" {
			t.Fatalf("first line %q", line)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("first event not delivered before the second was sent: the proxy buffers")
	}
	free()
	rest, _ := io.ReadAll(br)
	if !strings.Contains(string(rest), "event: message_stop") {
		t.Errorf("stream ended without message_stop: %q", rest)
	}
}
```

The source address, the constructor, the transport and the outcome:

```go
// Kills trusting a header a sandbox can set (task 10's X-Forwarded-For rule
// holds only if the request task 10 sees was built from the socket), and
// kills comparing an IPv4-mapped address against an IPv4 binding.
func TestRequestFromTrustsOnlyTheSocket(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "http://gatekeeper:4000/v1/messages", nil)
	r.RemoteAddr = "[::ffff:10.244.1.7]:51000"
	r.Header.Set("X-Forwarded-For", "10.9.9.9")
	r.Header.Set("X-Real-Ip", "10.9.9.9")
	r.Header.Set("Authorization", "Bearer a")
	r.Header.Set("X-Api-Key", "b")
	now := time.Unix(1_790_000_000, 0)
	got := RequestFrom(r, now)
	want := authz.Request{Authorization: "Bearer a", XAPIKey: "b", Source: netip.MustParseAddr("10.244.1.7"), Now: now}
	if got != want {
		t.Errorf("RequestFrom = %+v, want %+v", got, want)
	}
	r.RemoteAddr = "not-an-address"
	if RequestFrom(r, now).Source.IsValid() {
		t.Error("an unparsable RemoteAddr produced a valid Source")
	}
}

// Kills accepting an Upstream with a path: "http://litellm:4001/v1" would
// forward to /v1/v1/messages, the inverse of Panacea's bare-host 405.
func TestNewRefusesAnIncompleteOrPathedConfig(t *testing.T) {
	good, _ := url.Parse("http://litellm:4001")
	for name, c := range map[string]Config{
		"nil upstream":    {Catalogue: cat{}, Authz: deny{}},
		"upstream path":   {Upstream: must(t, "http://litellm:4001/v1"), Catalogue: cat{}, Authz: deny{}},
		"upstream query":  {Upstream: must(t, "http://litellm:4001?x=1"), Catalogue: cat{}, Authz: deny{}},
		"https upstream":  {Upstream: must(t, "https://litellm:4001"), Catalogue: cat{}, Authz: deny{}},
		"nil catalogue":   {Upstream: good, Authz: deny{}},
		"nil authoriser":  {Upstream: good, Catalogue: cat{}},
		"negative limit":  {Upstream: good, Catalogue: cat{}, Authz: deny{}, MaxBody: -1},
	} {
		if _, err := New(c); err == nil {
			t.Errorf("%s: New accepted it", name)
		}
	}
}

func must(t *testing.T, s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

// Kills http.ProxyFromEnvironment (a corporate HTTPS_PROXY would receive the
// session key) and kills a response-header deadline (a long non-streaming
// request legitimately waits minutes for its first byte).
func TestTheTransportHasNoProxyAndNoResponseDeadline(t *testing.T) {
	tr := NewTransport()
	if tr.Proxy != nil {
		t.Error("Transport.Proxy is set")
	}
	if tr.ResponseHeaderTimeout != 0 {
		t.Errorf("ResponseHeaderTimeout = %s, want 0", tr.ResponseHeaderTimeout)
	}
}

// Kills a missing or doubled report, and kills putting a request-chosen path
// in the log: a refused path is reported as the constant OtherPath.
func TestOutcomeIsReportedOnceWithAFixedPathSet(t *testing.T) {
	up, _ := upstream(t, nil)
	var got []Outcome
	h := gate(t, up, allow{dataclass.RouteAU}, func(c *Config) { c.Observe = func(o Outcome) { got = append(got, o) } })
	post(h, http.MethodPost, "/v1/messages", minimal, nil)
	post(h, http.MethodPost, "/v1/secret-prompt-text-in-a-path", minimal, nil)
	if len(got) != 2 {
		t.Fatalf("%d outcomes, want 2", len(got))
	}
	if o := got[0]; o.Session != "s-1" || o.Project != "p-1" || o.Route != dataclass.RouteAU || o.Path != "/v1/messages" || o.Status != 200 {
		t.Errorf("first outcome %+v", o)
	}
	if o := got[1]; o.Path != OtherPath || o.Status != 404 || o.Session != "" {
		t.Errorf("second outcome %+v", o)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd control-plane && go test ./internal/gatekeeper/proxy/`
Expected: a build failure naming the symbols this task adds, for example `undefined: New`, `undefined: Config`, `undefined: RequestFrom`, `undefined: OtherPath`. If it instead names `authz`, `dataclass`, `wire` or `httpx`, a dependency (task 10, 01, 03) is not merged yet: stop.

- [ ] **Step 3: Write the rewrite**

`json.Decoder.InputOffset` after decoding a value is the offset just past it, and a `RawMessage` decoded by a `Decoder` holds the value's exact bytes with no leading whitespace. So the value sits at `body[end-len(raw):end]`. The code checks that equality instead of trusting it: if a future Go release changed either behaviour, the proxy would refuse every request with `invalid_json`, loudly, rather than splice at the wrong offset.

Create `control-plane/internal/gatekeeper/proxy/rewrite.go`:

```go
package proxy

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

// errNotOneObject means the body is not exactly one JSON object with unique
// top-level keys. errNoModel means it is, but "model" is absent or not a string.
var (
	errNotOneObject = errors.New("body is not one JSON object with unique top-level keys")
	errNoModel      = errors.New("body has no string model")
)

// rewriteModel replaces the value of the top-level "model" key with the result
// of resolve, and returns the new body and the public id it found. Every byte
// outside that one string literal is the caller's, in the caller's order.
//
// It refuses a body in which "model" appears twice (after unescaping the key),
// because the upstream keeps the LAST duplicate and a rewrite of the first
// would forward a model the sender chose.
func rewriteModel(body []byte, resolve func(publicID string) (group string, ok bool)) ([]byte, string, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return nil, "", errNotOneObject
	}
	keys := map[string]bool{}
	start, end := -1, -1
	var public string
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, "", errNotOneObject
		}
		key, _ := tok.(string)
		if keys[key] {
			return nil, "", errNotOneObject
		}
		keys[key] = true
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, "", errNotOneObject
		}
		if key != "model" {
			continue
		}
		stop := int(dec.InputOffset())
		begin := stop - len(raw)
		if begin < 0 || !bytes.Equal(body[begin:stop], raw) {
			return nil, "", errNotOneObject // see Step 3's note: never splice at a guessed offset
		}
		if err := json.Unmarshal(raw, &public); err != nil {
			return nil, "", errNoModel
		}
		start, end = begin, stop
	}
	if tok, err := dec.Token(); err != nil || tok != json.Delim('}') {
		return nil, "", errNotOneObject
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, "", errNotOneObject // a second value after the object
	}
	if start < 0 {
		return nil, "", errNoModel
	}
	group, ok := resolve(public)
	if !ok {
		return nil, public, errNoModel
	}
	lit, _ := json.Marshal(group) // a catalogue group id; it cannot fail
	out := make([]byte, 0, len(body)+len(lit))
	out = append(out, body[:start]...)
	out = append(out, lit...)
	return append(out, body[end:]...), public, nil
}
```

- [ ] **Step 4: Write the proxy**

The order inside `ServeHTTP` is the security argument, so it is fixed and commented: **path, then authorisation, then the body, then the rewrite, then the forward.** A refused path costs no store lookup; an unauthenticated caller never makes the proxy read 64 MiB; nothing reaches LiteLLM that has not passed all four. `httputil.ReverseProxy` runs in `Rewrite` mode, which already drops hop-by-hop and `X-Forwarded-*` headers from the outbound request (Go 1.20+ `net/http/httputil` documentation, training knowledge, **UNVERIFIED** here); the hygiene below drops them again explicitly, so a Go release that changed that default would not reopen the hole.

A body over `MaxBody` answers `413` with `httpx.CodeInvalidJSON`: `problem.go`'s own comment defines `invalid_json` as including "over the limit" (`control-plane/internal/httpx/problem.go:45`, `:101-104`), so no new code is needed. An `Allow` decision whose route is neither `global` nor `au` is `503 route_unknown` rather than a `Resolve` miss, so a broken decider fails closed with the code that says so (P6-D5).

Create `control-plane/internal/gatekeeper/proxy/proxy.go`, first half:

```go
// Package proxy is the byte-handling half of the gatekeeper: it forwards the two
// Messages paths to LiteLLM with exactly one JSON value changed.
package proxy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/dataclass"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/gatekeeper/authz"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/catalogue"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/wire"
)

// DefaultMaxBody bounds one request body. A long conversation with images is
// tens of MiB; the upstream's own limit is lower, and the proxy's is the one
// that protects the proxy.
const DefaultMaxBody int64 = 64 << 20

// OtherPath is the only path an Outcome reports for a refused request, so a
// request-chosen path never reaches a log line.
const OtherPath = "(other)"

// AllowedPaths are forwarded upstream, matched exactly on URL.Path, POST only.
var AllowedPaths = []string{"/v1/messages", "/v1/messages/count_tokens"}

// Decider is task 10's authoriser.
type Decider interface {
	Decide(ctx context.Context, r authz.Request) authz.Decision
}

// Resolver maps a public model id to a LiteLLM group on a route.
type Resolver interface {
	Resolve(publicID string, r dataclass.Route) (group string, ok bool)
}

var (
	_ Resolver = (*catalogue.Catalogue)(nil)
	_ Decider  = (*authz.Authorizer)(nil)
)

// Outcome is everything the proxy reports about a request. It has no field
// that can carry a body, a header value or a request-chosen path.
type Outcome struct {
	Session, Project string
	Route            dataclass.Route
	Path             string // one of AllowedPaths, or OtherPath
	Status           int
	Elapsed          time.Duration
}

// Config wires the proxy. See New for the defaults.
type Config struct {
	Upstream  *url.URL // LiteLLM, scheme http, host only
	Catalogue Resolver
	Authz     Decider
	MaxBody   int64
	Now       func() time.Time
	Observe   func(Outcome)
	Transport http.RoundTripper
}
```

Second half, same file: the constructor and the request path.

```go
type handler struct {
	c  Config
	rp *httputil.ReverseProxy
}

// New validates c and returns the proxy handler.
func New(c Config) (http.Handler, error) {
	switch u := c.Upstream; {
	case u == nil:
		return nil, errors.New("proxy: Upstream is required")
	case u.Scheme != "http" || u.Host == "":
		return nil, fmt.Errorf("proxy: Upstream %q must be http://host:port", u.Redacted())
	case (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.ForceQuery:
		return nil, fmt.Errorf("proxy: Upstream %q must carry no path and no query", u.Redacted())
	case c.Catalogue == nil || c.Authz == nil:
		return nil, errors.New("proxy: Catalogue and Authz are required")
	case c.MaxBody < 0:
		return nil, errors.New("proxy: MaxBody must not be negative")
	}
	if c.MaxBody == 0 {
		c.MaxBody = DefaultMaxBody
	}
	if c.Now == nil {
		c.Now = time.Now
	}
	if c.Observe == nil {
		c.Observe = func(Outcome) {}
	}
	if c.Transport == nil {
		c.Transport = NewTransport()
	}
	target := &url.URL{Scheme: c.Upstream.Scheme, Host: c.Upstream.Host}
	rp := &httputil.ReverseProxy{
		Rewrite:        func(pr *httputil.ProxyRequest) { pr.SetURL(target) },
		Transport:      c.Transport,
		FlushInterval:  -1, // every write is flushed: SSE pings are the only traffic in a long think
		ModifyResponse: func(resp *http.Response) error { dropLiteLLM(resp.Header); return nil },
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _ error) {
			// The error can name the upstream address; it is not relayed.
			httpx.WriteProblem(w, http.StatusServiceUnavailable, httpx.CodeBackendUnavailable,
				"The inference gateway could not reach its upstream.", nil)
		},
	}
	return &handler{c: c, rp: rp}, nil
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := h.c.Now()
	sw := &statusWriter{ResponseWriter: w}
	o := Outcome{Path: OtherPath}
	defer func() { o.Status, o.Elapsed = sw.code(), h.c.Now().Sub(start); h.c.Observe(o) }()

	// 1. Path: exact, POST only. Nothing else costs a lookup.
	path, ok := allowed(r)
	if !ok {
		httpx.WriteProblem(sw, http.StatusNotFound, wire.CodePathNotAllowed,
			"The gateway forwards only POST /v1/messages and /v1/messages/count_tokens.", nil)
		return
	}
	o.Path = path

	// 2. Authorisation, task 10's status and code verbatim.
	d := h.c.Authz.Decide(r.Context(), RequestFrom(r, start))
	if !d.Allow {
		status, code := d.Status, d.Code
		if status < 400 || code == "" { // a decider that says no without saying why
			status, code = http.StatusInternalServerError, httpx.CodeInternal
		}
		httpx.WriteProblem(sw, status, code, "The inference gateway refused this request.", nil)
		return
	}
	o.Session, o.Project, o.Route = d.Binding.SessionID, d.Binding.ProjectID, d.Route
	if d.Route != dataclass.RouteGlobal && d.Route != dataclass.RouteAU {
		httpx.WriteProblem(sw, http.StatusServiceUnavailable, wire.CodeRouteUnknown,
			"This project's inference route is not known.", nil)
		return
	}

	// 3. The body, bounded.
	body, err := io.ReadAll(http.MaxBytesReader(sw, r.Body, h.c.MaxBody))
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			httpx.WriteProblem(sw, http.StatusRequestEntityTooLarge, httpx.CodeInvalidJSON,
				"The request body is over the gateway's limit.", nil)
			return
		}
		httpx.WriteProblem(sw, http.StatusBadRequest, httpx.CodeInvalidJSON, "The request body could not be read.", nil)
		return
	}

	// 4. The rewrite: one value, on the decided route.
	out, _, err := rewriteModel(body, func(id string) (string, bool) { return h.c.Catalogue.Resolve(id, d.Route) })
	switch {
	case errors.Is(err, errNotOneObject):
		httpx.WriteProblem(sw, http.StatusBadRequest, httpx.CodeInvalidJSON,
			"The request body must be one JSON object with unique keys.", nil)
		return
	case err != nil:
		httpx.WriteProblem(sw, http.StatusBadRequest, wire.CodeModelNotOffered,
			"That model is not offered on this project's route.", nil)
		return
	}

	// 5. Forward: the constant path, the sender's query, the decided credential.
	in := r.Clone(r.Context())
	in.URL.Path, in.URL.RawPath = path, ""
	in.Body, in.ContentLength, in.GetBody = io.NopCloser(bytes.NewReader(out)), int64(len(out)), nil
	in.Header.Del("Content-Length")
	cleanRequestHeaders(in.Header, d.Token)
	h.rp.ServeHTTP(sw, in)
}
```

Third part, same file: header hygiene, the source address, the transport, and the status writer. The writer implements `Flush` **and** `Unwrap`; `ReverseProxy` flushes through `http.NewResponseController`, which finds the real `Flusher` through `Unwrap`, and a wrapper with neither is exactly the buffering mutant `TestAStreamIsRelayedEventByEvent` kills.

```go
func allowed(r *http.Request) (string, bool) {
	if r.Method != http.MethodPost {
		return "", false
	}
	for _, p := range AllowedPaths {
		if r.URL.Path == p {
			return p, true
		}
	}
	return "", false
}

// credentialHeaders are removed before the one Authorization is set (decision c).
var credentialHeaders = []string{"Authorization", "X-Api-Key", "Api-Key", "X-Goog-Api-Key",
	"Ocp-Apim-Subscription-Key", "Proxy-Authorization", "Cookie"}

// forwardedHeaders would let a sandbox name its own source address upstream.
var forwardedHeaders = []string{"Forwarded", "X-Forwarded-For", "X-Forwarded-Host",
	"X-Forwarded-Proto", "X-Real-Ip"}

func cleanRequestHeaders(h http.Header, token string) {
	for _, k := range credentialHeaders {
		h.Del(k)
	}
	for _, k := range forwardedHeaders {
		h.Del(k)
	}
	dropLiteLLM(h)
	h.Set("Authorization", "Bearer "+token)
}

// dropLiteLLM removes every x-litellm-* and litellm-* header, in either direction.
func dropLiteLLM(h http.Header) {
	for k := range h {
		l := strings.ToLower(k)
		if strings.HasPrefix(l, "x-litellm-") || strings.HasPrefix(l, "litellm-") {
			delete(h, k)
		}
	}
}

// RequestFrom builds task 10's input from the socket and the two credential
// headers, and from nothing a sandbox can forge about where it is.
func RequestFrom(r *http.Request, now time.Time) authz.Request {
	var src netip.Addr
	if ap, err := netip.ParseAddrPort(r.RemoteAddr); err == nil {
		src = ap.Addr().Unmap()
	}
	return authz.Request{Authorization: r.Header.Get("Authorization"), XAPIKey: r.Header.Get("X-Api-Key"),
		Source: src, Now: now}
}

// NewTransport has no environment proxy (a corporate HTTPS_PROXY must never see
// a session key), no response-header deadline, and no transparent gzip, which
// would otherwise add Accept-Encoding to requests that did not ask for it.
func NewTransport() *http.Transport {
	return &http.Transport{
		Proxy:               nil,
		MaxIdleConns:        256,
		MaxIdleConnsPerHost: 256,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  true,
	}
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (s *statusWriter) WriteHeader(code int) {
	if s.status == 0 && code >= 200 { // a relayed 1xx is not the outcome
		s.status = code
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusWriter) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	return s.ResponseWriter.Write(b)
}

func (s *statusWriter) Flush() {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	_ = http.NewResponseController(s.ResponseWriter).Flush()
}

func (s *statusWriter) Unwrap() http.ResponseWriter { return s.ResponseWriter }

func (s *statusWriter) code() int {
	if s.status == 0 {
		return http.StatusOK
	}
	return s.status
}
```

There is no idle-read deadline on the upstream connection either. The skeleton asks for one "above 300 s"; none is needed, because Claude Code aborts a stream that is silent for 300 seconds, that abort cancels the inbound request's context, and `ReverseProxy` cancels the upstream request with it. A deadline of ours would only be a second, worse copy of the client's.

- [ ] **Step 5: Run the tests to verify they pass**

```bash
cd control-plane
go test -race -count=1 ./internal/gatekeeper/proxy/                    # expect: ok
go test -count=1 -v ./internal/gatekeeper/proxy/ | grep -c '^--- PASS: Test'   # expect 12
```

- [ ] **Step 6: Kill four mutants by hand**

Each edit is made, the package run, and the edit reverted. Each must turn the named test red.

| Edit | Test that must fail |
|---|---|
| `allowed`: `r.URL.Path == p` → `strings.HasPrefix(r.URL.Path, p)` | `TestOnlyTheTwoMessagesPathsAreForwarded` |
| `ServeHTTP`: `h.c.Catalogue.Resolve(id, d.Route)` → `h.c.Catalogue.Resolve(id, dataclass.RouteGlobal)` | `TestRewriteChangesOnlyTheModelValue` (the AU row) |
| `rewriteModel`: delete the `if keys[key] { return … }` block | `TestTheRouteCannotBeChosenByTheBody/model_twice` and `/escaped_key` |
| `New`: `FlushInterval: -1` → `FlushInterval: 0`, and `Flush` removed from `statusWriter` | `TestAStreamIsRelayedEventByEvent` |

The last row needs both halves. With `FlushInterval: 0` alone, `ReverseProxy` still flushes an upstream it recognises as `text/event-stream` (training knowledge of `net/http/httputil`, **UNVERIFIED**), so the test stays green; that is a second guard, not a reason to drop the first.

- [ ] **Step 7: Vet, format, and confirm the import set**

```bash
cd control-plane
go vet ./internal/gatekeeper/proxy/ && gofmt -l internal/gatekeeper/proxy/     # expect no output
go list -deps ./internal/gatekeeper/proxy/ | grep kpmg-au-emu | sort
```

Expected, exactly these six lines (a seventh means a dependency this task did not plan):

```
github.com/kpmg-au-emu/panacea-app/control-plane/internal/dataclass
github.com/kpmg-au-emu/panacea-app/control-plane/internal/gatekeeper/authz
github.com/kpmg-au-emu/panacea-app/control-plane/internal/gatekeeper/proxy
github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx
github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/catalogue
github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/wire
```

If task 10's `authz` imports `binding` (for its tests' memory store, only in `_test.go`) it does not appear here; if it imports it from non-test code, the list has a seventh line and that is task 10's to explain, not this task's to hide.

- [ ] **Step 8: Commit**

```bash
git add control-plane/internal/gatekeeper/proxy/proxy.go control-plane/internal/gatekeeper/proxy/rewrite.go control-plane/internal/gatekeeper/proxy/proxy_test.go
git commit -F <message file> -- control-plane/internal/gatekeeper/proxy/proxy.go control-plane/internal/gatekeeper/proxy/rewrite.go control-plane/internal/gatekeeper/proxy/proxy_test.go
```

Message: `feat(gatekeeper): the proxy, one JSON value rewritten and everything else relayed (phase 6 task 11)`, a body naming decisions (a) to (e), and the attribution line.

**Acceptance:** step 5 prints `ok` and `12`; step 6's four edits each fail the named test; step 7 prints nothing and then the six lines.

**What this task does not claim:**

- **That LiteLLM reads only the body's `model` to choose a group.** If the pinned image also honours a routing header or a body field (for example `metadata.tags` with tag-based routing), a sandbox could steer through it. The header side is closed by stripping every `x-litellm-*` and `litellm-*`; the body side is task 18's P08, and if P08 shows body tags matter, this rewrite gains exactly one more edit, dropping `metadata.tags`, with its own test.
- **That LiteLLM prefers `Authorization` to any other credential header.** It does not need to: decision (c) leaves it only one.
- **That streaming survives LiteLLM.** This task proves the gatekeeper does not buffer. Whether LiteLLM relays SSE pings unbuffered is task 18's P09.
- **That the error body LiteLLM returns is safe to relay.** It is relayed unchanged to the caller that sent the request; task 12's canary test proves it never reaches a log. Whether it can carry another tenant's data is a LiteLLM property, **UNVERIFIED**.
- **`/v1/models` and `HEAD /api/hello`.** Task 12 serves both in front of this handler.

**Gaps against the skeleton, for the orchestrator:**

1. The skeleton's "decode as `map[string]json.RawMessage` … re-encode" cannot pass its own byte-for-byte test (decision (a)); this part splices instead.
2. `Config` gains `Now`, `Observe` and `Transport`, and `Catalogue` is the interface `Resolver` (decisions (b), (e)). Task 12 must pass `Observe`.
3. The Go identifiers `wire.CodePathNotAllowed`, `wire.CodeModelNotOffered` and `wire.CodeRouteUnknown` match task 03's part (`task-03.md:554-557`).
4. The skeleton's "idle-read timeout above 300 s" is not built (the note at the end of step 4 says why).
