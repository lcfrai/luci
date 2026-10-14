### Task 12: `cmd/gatekeeper`: wiring, `/v1/models`, `HEAD /api/hello`, body-free logs

This task turns task 10's authoriser, task 11's proxy and task 13's route source into the one process a sandbox can reach. It adds the three things Claude Code asks a gateway for besides inference, answered **locally** with no upstream call: `GET /v1/models` (*"a timeout of 3 seconds by default, and any redirect is treated as failure"*), `HEAD /api/hello` (a connection-warming probe the gateway *"can reject without breaking anything"*), and nothing else (both quotations from https://code.claude.com/docs/en/llm-gateway-protocol as read by the skeleton on 2026-09-25, skeleton lines 86-87; **not re-read here**).

**The one thing this task must not get wrong is that no body, no token and no header value reaches a log line** (Global constraint 3). Task 11 already made its report a type with no room for them (`proxy.Outcome`). This task is where the logger is attached, so this task's canary test is the proof: a prompt holding a canary string goes through the whole handler, LiteLLM fails with a 500 that echoes it, and every byte the process wrote to either logger is searched for it.

Five decisions where the skeleton is silent:

- **(a) No `ServeMux`.** The top-level handler is a four-case `switch` on the exact `URL.Path`. A `ServeMux` cleans paths and answers a non-canonical one (`/v1/messages/../files`) with a `301`; the docs treat any redirect on `/v1/models` as failure, and task 11's allowlist test expects that path to be a `404 path_not_allowed`, not a redirect to a path the allowlist would then judge.
- **(b) `/readyz` does not depend on keysmith, the Hub or LiteLLM.** It answers `200` once the process is serving. Readiness tied to a dependency takes every gatekeeper replica out of the Service during that dependency's blip, and a sandbox then sees a connection refused instead of a `503` with a problem code that says which part failed. Liveness and readiness are therefore the same check here, and phase 8's probes may use either.
- **(c) The keysmith client authenticates with a token file read on every miss.** `PANACEA_APP_GATEWAY_CALLER_TOKEN_FILE` is a projected service-account token with audience `keysmith` in the cloud (kubelet rotates it, so it is never read once) and a line of task 17's `secrets/callers` locally. It is the same variable name task 15 gives the local control plane, so both callers of keysmith are configured alike. Misses are at most one per token per `BINDING_CACHE_TTL`, so the read costs nothing.
- **(d) A keysmith answer is believed only for the key asked about.** The client refuses a `200` whose `keyHash` differs from the one requested, and treats a malformed hash as not found without calling keysmith. A 404 is cached for 2 s (the skeleton's negative cache); an outage is never cached.
- **(e) Shutdown waits 25 s for streams, then closes.** Kubernetes' default termination grace is 30 s (training knowledge, **UNVERIFIED** against phase 8's chart). A stream longer than that is cut by a rollout; whether Claude Code retries a cut stream is **UNVERIFIED** and task 23 does not test it.

**Files:**
- `control-plane/cmd/gatekeeper/main.go` — `main`, `run`, `buildRoutes`, `deps`, `newHandler`, `serveModels`, `newServer` (new)
- `control-plane/cmd/gatekeeper/config.go` — `config`, `loadConfig` (new)
- `control-plane/cmd/gatekeeper/bindings.go` — `bindingClient`, the cached `wire.BindingReader` over keysmith (new)
- `control-plane/cmd/gatekeeper/main_test.go` — the canary, `/v1/models`, `/api/hello`, the routing and the server (new)
- `control-plane/cmd/gatekeeper/config_test.go`, `control-plane/cmd/gatekeeper/bindings_test.go` — the config table and the binding cache (new; the skeleton names one test file, and three keep each under a screen)

**Interfaces:**
- Produces (package `main`, for its tests): `deps{catalogue proxy.Resolver; models []byte; authz proxy.Decider; upstream *url.URL; log *slog.Logger; now func() time.Time}`; `newHandler(deps) (http.Handler, error)`; `newServer(addr string, h http.Handler) *http.Server`; `loadConfig(getenv func(string) string) (config, error)`; `newBindingClient(base, tokenFile string, ttl time.Duration, now func() time.Time) *bindingClient`, which satisfies `wire.BindingReader`.
- The process: listens on `:4000` (skeleton shared names); reads `PANACEA_APP_GATEWAY_LITELLM_URL` (default `http://litellm:4001`), `_KEYSMITH_URL` (default `http://keysmith:4100`), `_CALLER_TOKEN_FILE` (required), `_ROUTE_SOURCE` (required, `hub` or `file:<path>`), `_HUB_URL` (required with `hub`) and `_HUB_SCOPE` (with `hub`; default `panacea-app-hub`, the realm's Hub audience scope since `OD-30`; task 13's gap 1), `_BINDING_CACHE_TTL` (`10s`), `_ROUTE_CACHE_TTL` (`30s`), `_ROUTE_STALE_MAX` (`10m`); with `hub`, the four `AZURE_*` variables through `routesource.WorkloadTokenFromEnv`.
- Consumes: `proxy.New`, `proxy.Config`, `proxy.Outcome`, `proxy.RequestFrom`, `proxy.Resolver`, `proxy.Decider` (task 11); `authz.Authorizer{Bindings, Routes}` (task 10, the skeleton's field names); `routesource.NewCache`, `NewFile`, `NewHub`, `ClientCredentialsFromEnv` (task 13; `OD-30` replaced `WorkloadTokenFromEnv`); `catalogue.Load`, `(*Catalogue).ModelsResponse` (task 02); `wire.Binding`, `wire.ErrNotFound`, `wire.HashToken` (task 03); `httpx.WriteProblem`, `httpx.CodeInternal` (`control-plane/internal/httpx/problem.go:58`, `:93`).
- Required by: task 17 (the compose service and `images/gateway/Dockerfile`), task 19 (the chart's probes and variables), task 23 (the log line it reads for `route=`).

- [ ] **Step 1: Write the failing handler tests**

The rig is the real handler with the real task 10 authoriser and the real task 13 file source; only the binding store, the catalogue and LiteLLM are fakes. So a log line or a problem body written anywhere in tasks 10, 11, 12 or 13 is in the canary's reach.

Create `control-plane/cmd/gatekeeper/main_test.go`:

```go
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/dataclass"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/gatekeeper/authz"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/wire"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/routesource"
)

const (
	canary     = "CANARY-7f3e91"
	token      = "sk-pa-gatekeeper-test-token"
	pod        = "10.244.1.7"
	modelsJSON = `{"data":[{"type":"model","id":"claude-test-1"}],"has_more":false}`
)

type oneModel struct{}

func (oneModel) Resolve(id string, r dataclass.Route) (string, bool) {
	if id != "claude-test-1" {
		return "", false
	}
	return string(r) + "/" + id, true
}

type bindings map[string]wire.Binding

func (b bindings) ByKeyHash(_ context.Context, h string) (wire.Binding, error) {
	if x, ok := b[h]; ok {
		return x, nil
	}
	return wire.Binding{}, wire.ErrNotFound
}

// lockedBuffer is a bytes.Buffer safe for the proxy's goroutines.
type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) { l.mu.Lock(); defer l.mu.Unlock(); return l.b.Write(p) }
func (l *lockedBuffer) String() string              { l.mu.Lock(); defer l.mu.Unlock(); return l.b.String() }

func upstreamServer(t *testing.T, h http.HandlerFunc) (*url.URL, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if h != nil {
			h(w, r)
			return
		}
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	return u, &calls
}
```

The rig and the canary, same file:

```go
func newRig(t *testing.T, up *url.URL) (http.Handler, *lockedBuffer) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "projects.json")
	body := `{"p-1": {"dataClass": {"level": "internal", "residency": "any"}, "inferenceRoute": "global"}}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	routes, err := routesource.NewFile(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	b := wire.Binding{KeyHash: wire.HashToken(token), KeyID: "k-1", SessionID: "s-1", Generation: 1,
		ProjectID: "p-1", PodAddr: netip.MustParseAddr(pod), RouteAtMint: dataclass.RouteGlobal,
		CreatedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour)}
	logs := &lockedBuffer{}
	h, err := newHandler(deps{catalogue: oneModel{}, models: []byte(modelsJSON),
		authz:    &authz.Authorizer{Bindings: bindings{b.KeyHash: b}, Routes: routes},
		upstream: up, log: slog.New(slog.NewJSONHandler(logs, nil))})
	if err != nil {
		t.Fatal(err)
	}
	return h, logs
}

func send(h http.Handler, method, target, body string, withToken bool) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://gatekeeper:4000"+target, strings.NewReader(body))
	r.RemoteAddr = pod + ":40123"
	r.Header.Set("Anthropic-Version", "2023-06-01")
	if withToken {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

// captureStdLog catches net/http's and httputil's own log lines, which go to
// the standard logger, not to slog.
func captureStdLog(t *testing.T) *lockedBuffer {
	buf := &lockedBuffer{}
	log.SetOutput(buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return buf
}

// Kills logging a body, the token or a header value on each of the three ways
// a request fails: LiteLLM's 500 echoing the prompt, a refused model named by
// the canary, and an unreachable LiteLLM. Kills a log line with any field
// beyond the six. The relayed 500 body does carry the canary back to its own
// sender (task 11 decision d); a problem body the gatekeeper writes must not.
func TestNoBodyTokenOrHeaderValueReachesALog(t *testing.T) {
	std := captureStdLog(t)
	echo, _ := upstreamServer(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write(b)
	})
	h, logs := newRig(t, echo)
	prompt := `{"model":"claude-test-1","messages":[{"role":"user","content":"` + canary + `"}]}`
	if w := send(h, http.MethodPost, "/v1/messages?beta=true", prompt, true); w.Code != 500 {
		t.Fatalf("echo: status %d, want the relayed 500", w.Code)
	}
	if w := send(h, http.MethodPost, "/v1/messages", `{"model":"`+canary+`"}`, true); w.Code != 400 ||
		strings.Contains(w.Body.String(), canary) {
		t.Errorf("refused model: %d %s", w.Code, w.Body)
	}
	dead := httptest.NewServer(http.NotFoundHandler())
	gone, _ := url.Parse(dead.URL)
	dead.Close() // a LiteLLM that is not listening
	hGone, logsGone := newRig(t, gone)
	if w := send(hGone, http.MethodPost, "/v1/messages", prompt, true); w.Code != 503 || strings.Contains(w.Body.String(), canary) {
		t.Errorf("unreachable: %d %s", w.Code, w.Body)
	}
	for name, out := range map[string]string{"slog": logs.String() + logsGone.String(), "log": std.String()} {
		for _, secret := range []string{canary, token} {
			if strings.Contains(out, secret) {
				t.Errorf("%s output contains %q:\n%s", name, secret, out)
			}
		}
	}
	assertLogShape(t, logs.String()+logsGone.String(), 3)
}
```

The log shape and the local answers, same file:

```go
// assertLogShape: exactly n lines, each slog's three keys plus the six.
func assertLogShape(t *testing.T, out string, n int) {
	t.Helper()
	const want = "level,ms,msg,path,project,route,session,status,time"
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != n {
		t.Fatalf("%d log lines, want %d:\n%s", len(lines), n, out)
	}
	for _, l := range lines {
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatalf("log line is not JSON: %q", l)
		}
		var keys []string
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		if got := strings.Join(keys, ","); got != want {
			t.Errorf("log keys %s, want %s", got, want)
		}
	}
}

// Kills serving the model list to anyone on the network, kills a redirect
// (the docs: "any redirect is treated as failure"), and kills an upstream call.
func TestModelsIsLocalAndNeedsABinding(t *testing.T) {
	up, calls := upstreamServer(t, nil)
	h, _ := newRig(t, up)
	if w := send(h, http.MethodGet, "/v1/models?limit=1000", "", false); w.Code != http.StatusUnauthorized {
		t.Errorf("without a token: %d, want 401", w.Code)
	}
	w := send(h, http.MethodGet, "/v1/models?limit=1000", "", true)
	if w.Code != 200 || w.Body.String() != modelsJSON || w.Header().Get("Location") != "" ||
		w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("with a token: %d %q %v", w.Code, w.Body, w.Header())
	}
	if n := calls.Load(); n != 0 {
		t.Errorf("LiteLLM saw %d calls, want 0", n)
	}
}

// Kills requiring a credential on the warming probe, and kills forwarding it.
func TestHelloAndHealthAreAnsweredLocally(t *testing.T) {
	up, calls := upstreamServer(t, nil)
	h, logs := newRig(t, up)
	if w := send(h, http.MethodHead, "/api/hello", "", false); w.Code != http.StatusNoContent || w.Body.Len() != 0 {
		t.Errorf("HEAD /api/hello: %d %q, want 204 and no body", w.Code, w.Body)
	}
	for _, p := range []string{"/healthz", "/readyz"} {
		if w := send(h, http.MethodGet, p, "", false); w.Code != 200 {
			t.Errorf("GET %s: %d, want 200", p, w.Code)
		}
	}
	if n := calls.Load(); n != 0 || logs.String() != "" {
		t.Errorf("%d upstream calls and logs %q, want neither", n, logs.String())
	}
}

// Kills a ServeMux, which answers a non-canonical path with a 301.
func TestNoPathIsRedirected(t *testing.T) {
	up, _ := upstreamServer(t, nil)
	h, _ := newRig(t, up)
	for _, p := range []string{"/v1/messages/../files", "//v1/models", "/v1/models/", "/v1//messages", "/api/hello/"} {
		for _, m := range []string{http.MethodGet, http.MethodPost} {
			if w := send(h, m, p, "{}", true); w.Code != http.StatusNotFound {
				t.Errorf("%s %s: %d, want 404", m, p, w.Code)
			}
		}
	}
}

// Kills a WriteTimeout (it would cut a five-minute stream) and a ReadTimeout.
func TestServerNeverCutsAStream(t *testing.T) {
	s := newServer(":4000", http.NotFoundHandler())
	if s.WriteTimeout != 0 || s.ReadTimeout != 0 || s.ReadHeaderTimeout != 10*time.Second || s.IdleTimeout != 120*time.Second {
		t.Errorf("timeouts write %s read %s header %s idle %s", s.WriteTimeout, s.ReadTimeout, s.ReadHeaderTimeout, s.IdleTimeout)
	}
}
```

- [ ] **Step 2: Write the failing config tests**

Create `control-plane/cmd/gatekeeper/config_test.go`:

```go
package main

import (
	"strings"
	"testing"
	"time"
)

const p = "PANACEA_APP_GATEWAY_"

func envOf(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func minimalEnv() map[string]string {
	return map[string]string{p + "ROUTE_SOURCE": "file:/etc/panacea/projects.json", p + "CALLER_TOKEN_FILE": "/run/secrets/caller"}
}

func TestConfigDefaults(t *testing.T) {
	c, err := loadConfig(envOf(minimalEnv()))
	if err != nil {
		t.Fatal(err)
	}
	if c.LiteLLM.String() != "http://litellm:4001" || c.KeysmithURL != "http://keysmith:4100" ||
		c.RouteSource != "file" || c.RouteFile != "/etc/panacea/projects.json" || c.CallerTokenFile != "/run/secrets/caller" ||
		c.BindingTTL != 10*time.Second || c.RouteTTL != 30*time.Second || c.RouteStaleMax != 10*time.Minute {
		t.Errorf("defaults %+v", c)
	}
}

// Kills a start-up that guesses: each bad or missing value fails, naming its
// variable, so a misconfigured pod crash-loops instead of serving a default.
func TestConfigRefusesNamingTheVariable(t *testing.T) {
	for name, tc := range map[string]struct {
		set     map[string]string
		wantVar string
	}{
		"no route source":     {map[string]string{"ROUTE_SOURCE": ""}, "ROUTE_SOURCE"},
		"bare file:":          {map[string]string{"ROUTE_SOURCE": "file:"}, "ROUTE_SOURCE"},
		"unknown source":      {map[string]string{"ROUTE_SOURCE": "static-global"}, "ROUTE_SOURCE"},
		"hub without a URL":   {map[string]string{"ROUTE_SOURCE": "hub", "HUB_SCOPE": "panacea-app-hub"}, "HUB_URL"},
		"hub, Entra scope":    {map[string]string{"ROUTE_SOURCE": "hub", "HUB_URL": "https://hub.example", "HUB_SCOPE": "api://hub/.default"}, "HUB_SCOPE"},
		"no caller token":     {map[string]string{"CALLER_TOKEN_FILE": ""}, "CALLER_TOKEN_FILE"},
		"LiteLLM with a path": {map[string]string{"LITELLM_URL": "http://litellm:4001/v1"}, "LITELLM_URL"},
		"keysmith not a URL":  {map[string]string{"KEYSMITH_URL": "keysmith"}, "KEYSMITH_URL"},
		"zero binding TTL":    {map[string]string{"BINDING_CACHE_TTL": "0s"}, "BINDING_CACHE_TTL"},
		"not a duration":      {map[string]string{"ROUTE_CACHE_TTL": "soon"}, "ROUTE_CACHE_TTL"},
		"stale below TTL":     {map[string]string{"ROUTE_STALE_MAX": "10s"}, "ROUTE_STALE_MAX"},
	} {
		env := minimalEnv()
		for k, v := range tc.set {
			env[p+k] = v
		}
		_, err := loadConfig(envOf(env))
		if err == nil || !strings.Contains(err.Error(), p+tc.wantVar) {
			t.Errorf("%s: err = %v, want one naming %s%s", name, err, p, tc.wantVar)
		}
	}
}
```

- [ ] **Step 3: Write the failing binding-cache tests**

Create `control-plane/cmd/gatekeeper/bindings_test.go`:

```go
package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/wire"
)

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time      { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *clock) add(d time.Duration) { c.mu.Lock(); c.t = c.t.Add(d); c.mu.Unlock() }

type ksCall struct{ path, auth string }

// fakeKeysmith answers status with body for every call and records each.
func fakeKeysmith(t *testing.T, status int, body any) (*bindingClient, *clock, func() []ksCall, string) {
	t.Helper()
	var mu sync.Mutex
	var calls []ksCall
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls = append(calls, ksCall{r.URL.Path, r.Header.Get("Authorization")})
		mu.Unlock()
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	}))
	t.Cleanup(srv.Close)
	file := filepath.Join(t.TempDir(), "caller-token")
	if err := os.WriteFile(file, []byte("gk-token-1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c := &clock{t: time.Date(2026, 9, 25, 3, 0, 0, 0, time.UTC)}
	return newBindingClient(srv.URL, file, 10*time.Second, c.now), c,
		func() []ksCall { mu.Lock(); defer mu.Unlock(); return append([]ksCall(nil), calls...) }, file
}

var hashA = wire.HashToken("sk-pa-a")

func bindingFor(hash string) wire.Binding {
	return wire.Binding{KeyHash: hash, SessionID: "s-1", Generation: 1, ProjectID: "p-1",
		PodAddr: netip.MustParseAddr("10.244.1.7")}
}

// Kills no cache (keysmith on every request) and caching past the TTL (a
// revocation would never be seen; P6-D7 promises 10 s).
func TestBindingsAreCachedForTheTTL(t *testing.T) {
	bc, c, calls, _ := fakeKeysmith(t, 200, bindingFor(hashA))
	for i := 0; i < 3; i++ {
		if b, err := bc.ByKeyHash(context.Background(), hashA); err != nil || b.SessionID != "s-1" {
			t.Fatalf("ByKeyHash = %+v, %v", b, err)
		}
	}
	c.add(9 * time.Second)
	_, _ = bc.ByKeyHash(context.Background(), hashA)
	if n := len(calls()); n != 1 {
		t.Fatalf("%d keysmith calls inside the TTL, want 1", n)
	}
	c.add(2 * time.Second)
	_, _ = bc.ByKeyHash(context.Background(), hashA)
	if got := calls(); len(got) != 2 || got[1] != (ksCall{"/v1/bindings/" + hashA, "Bearer gk-token-1"}) {
		t.Fatalf("calls %+v, want a second, authenticated lookup of exactly this hash", got)
	}
}
```

The misses, same file:

```go
// Kills an uncached miss (a sandbox replaying a dead token would make one
// keysmith call per request) and a miss cached as long as a hit.
func TestANotFoundIsCachedForTwoSeconds(t *testing.T) {
	bc, c, calls, _ := fakeKeysmith(t, 404, map[string]string{"error": "not_found"})
	for i := 0; i < 3; i++ {
		if _, err := bc.ByKeyHash(context.Background(), hashA); !errors.Is(err, wire.ErrNotFound) {
			t.Fatalf("err = %v, want wire.ErrNotFound", err)
		}
	}
	c.add(3 * time.Second)
	_, _ = bc.ByKeyHash(context.Background(), hashA)
	if n := len(calls()); n != 2 {
		t.Fatalf("%d keysmith calls, want 2", n)
	}
}

// Kills reporting a keysmith outage as "no such key" (task 10 would answer
// 401, and the negative cache would hold it), and kills caching an outage.
func TestAnOutageIsNeitherNotFoundNorCached(t *testing.T) {
	bc, _, calls, _ := fakeKeysmith(t, 503, map[string]string{"error": "backend_unavailable"})
	for i := 0; i < 2; i++ {
		if _, err := bc.ByKeyHash(context.Background(), hashA); err == nil || errors.Is(err, wire.ErrNotFound) {
			t.Fatalf("err = %v, want an outage error", err)
		}
	}
	if n := len(calls()); n != 2 {
		t.Fatalf("%d keysmith calls, want 2 (no caching of an outage)", n)
	}
}

// Kills reading the caller token once: kubelet rotates the projected token.
func TestTheCallerTokenIsReadOnEveryMiss(t *testing.T) {
	bc, c, calls, file := fakeKeysmith(t, 200, bindingFor(hashA))
	_, _ = bc.ByKeyHash(context.Background(), hashA)
	if err := os.WriteFile(file, []byte("gk-token-2"), 0o600); err != nil {
		t.Fatal(err)
	}
	c.add(11 * time.Second)
	_, _ = bc.ByKeyHash(context.Background(), hashA)
	if got := calls(); len(got) != 2 || got[1].auth != "Bearer gk-token-2" {
		t.Fatalf("calls %+v, want the rotated token on the second", got)
	}
}

// Kills believing an answer about another key, and kills sending a string a
// sandbox chose (the token's hash is ours, but the check is cheap) to keysmith.
func TestOnlyTheAskedKeyIsBelieved(t *testing.T) {
	bc, _, calls, _ := fakeKeysmith(t, 200, bindingFor(wire.HashToken("sk-pa-someone-else")))
	if _, err := bc.ByKeyHash(context.Background(), hashA); err == nil || errors.Is(err, wire.ErrNotFound) {
		t.Fatalf("err = %v, want an outage error for a mismatched answer", err)
	}
	for _, h := range []string{"", "../healthz", hashA[:63], hashA + "0", "ABCDEF" + hashA[6:]} {
		if _, err := bc.ByKeyHash(context.Background(), h); !errors.Is(err, wire.ErrNotFound) {
			t.Errorf("hash %q: err = %v, want wire.ErrNotFound", h, err)
		}
	}
	if n := len(calls()); n != 1 {
		t.Errorf("%d keysmith calls, want 1: malformed hashes are not sent", n)
	}
}
```

- [ ] **Step 4: Run the tests to verify they fail**

Run: `cd control-plane && go test ./cmd/gatekeeper/`
Expected: a build failure naming this task's symbols (`undefined: newHandler`, `undefined: deps`, `undefined: loadConfig`, `undefined: newBindingClient`, `undefined: newServer`). If it names `proxy`, `authz` or `routesource` symbols, task 11, 10 or 13 is not merged: stop.

- [ ] **Step 5: Write the config**

Create `control-plane/cmd/gatekeeper/config.go`:

```go
package main

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

const envPrefix = "PANACEA_APP_GATEWAY_"

type config struct {
	LiteLLM         *url.URL // task 11's Upstream: http, host only
	KeysmithURL     string
	CallerTokenFile string
	RouteSource     string // "hub" or "file"
	RouteFile       string
	HubURL          string
	HubScope        string
	BindingTTL      time.Duration
	RouteTTL        time.Duration
	RouteStaleMax   time.Duration
}

// loadConfig parses every variable once. Each error names its variable.
func loadConfig(getenv func(string) string) (config, error) {
	get := func(name, def string) string {
		if v := getenv(envPrefix + name); v != "" {
			return v
		}
		return def
	}
	bad := func(name, why string) error { return fmt.Errorf("%s%s: %s", envPrefix, name, why) }
	dur := func(name, def string) (time.Duration, error) {
		v := get(name, def)
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 {
			return 0, bad(name, fmt.Sprintf("%q is not a positive duration", v))
		}
		return d, nil
	}
	var c config
	var err error
	if c.LiteLLM, err = hostOnly(get("LITELLM_URL", "http://litellm:4001"), false); err != nil {
		return c, bad("LITELLM_URL", err.Error())
	}
	ks, err := hostOnly(get("KEYSMITH_URL", "http://keysmith:4100"), true)
	if err != nil {
		return c, bad("KEYSMITH_URL", err.Error())
	}
	c.KeysmithURL = ks.String()
	if c.CallerTokenFile = get("CALLER_TOKEN_FILE", ""); c.CallerTokenFile == "" {
		return c, bad("CALLER_TOKEN_FILE", "is required (a projected token, or a line of the local callers file)")
	}
	switch src := get("ROUTE_SOURCE", ""); {
	case src == "hub":
		c.RouteSource, c.HubURL, c.HubScope = "hub", get("HUB_URL", ""), get("HUB_SCOPE", "panacea-app-hub")
		if c.HubURL == "" {
			return c, bad("HUB_URL", "is required with ROUTE_SOURCE=hub")
		}
		if strings.ContainsAny(c.HubScope, ":/ ") { // an Entra api://…/.default is refused (OD-30)
			return c, bad("HUB_SCOPE", "must be the realm's Hub audience scope (panacea-app-hub) with ROUTE_SOURCE=hub")
		}
	case strings.HasPrefix(src, "file:") && len(src) > len("file:"):
		c.RouteSource, c.RouteFile = "file", strings.TrimPrefix(src, "file:")
	default:
		return c, bad("ROUTE_SOURCE", fmt.Sprintf("%q must be hub or file:<path>; there is no default route", src))
	}
	if c.BindingTTL, err = dur("BINDING_CACHE_TTL", "10s"); err != nil {
		return c, err
	}
	if c.RouteTTL, err = dur("ROUTE_CACHE_TTL", "30s"); err != nil {
		return c, err
	}
	if c.RouteStaleMax, err = dur("ROUTE_STALE_MAX", "10m"); err != nil {
		return c, err
	}
	if c.RouteStaleMax < c.RouteTTL {
		return c, bad("ROUTE_STALE_MAX", fmt.Sprintf("%s is below ROUTE_CACHE_TTL %s", c.RouteStaleMax, c.RouteTTL))
	}
	return c, nil
}

// hostOnly accepts scheme://host[:port] with nothing after it; https only if allowed.
func hostOnly(raw string, httpsOK bool) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && !(httpsOK && u.Scheme == "https")) ||
		(u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.User != nil {
		// The value is not echoed: a URL with credentials in it must not reach a log.
		return nil, fmt.Errorf("must be scheme://host:port with no path, query or credentials")
	}
	return &url.URL{Scheme: u.Scheme, Host: u.Host}, nil
}
```

- [ ] **Step 6: Write the binding client**

Create `control-plane/cmd/gatekeeper/bindings.go`:

```go
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/wire"
)

const (
	negativeTTL   = 2 * time.Second
	lookupTimeout = 2 * time.Second // a request's lookup, not the request
	maxCached     = 8192
)

var hashRe = regexp.MustCompile(`^[0-9a-f]{64}$`)

// errBindingsUnavailable is a keysmith outage. It is never wire.ErrNotFound,
// so task 10 does not answer 401 for it and nothing caches it.
var errBindingsUnavailable = errors.New("gatekeeper: keysmith bindings unavailable")

var _ wire.BindingReader = (*bindingClient)(nil)

type cachedBinding struct {
	b     wire.Binding
	found bool
	until time.Time
}

// bindingClient reads keysmith's GET /v1/bindings/{keyHash}, so the gatekeeper
// holds no database login (P6-D7).
type bindingClient struct {
	base, tokenFile string
	ttl             time.Duration
	now             func() time.Time
	http            *http.Client

	mu    sync.Mutex
	cache map[string]cachedBinding
}

func newBindingClient(base, tokenFile string, ttl time.Duration, now func() time.Time) *bindingClient {
	if now == nil {
		now = time.Now
	}
	return &bindingClient{base: strings.TrimSuffix(base, "/"), tokenFile: tokenFile, ttl: ttl, now: now,
		http:  &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
		cache: map[string]cachedBinding{}}
}

func (c *bindingClient) ByKeyHash(ctx context.Context, hash string) (wire.Binding, error) {
	if !hashRe.MatchString(hash) {
		return wire.Binding{}, wire.ErrNotFound
	}
	t := c.now()
	c.mu.Lock()
	e, ok := c.cache[hash]
	c.mu.Unlock()
	if ok && t.Before(e.until) {
		if e.found {
			return e.b, nil
		}
		return wire.Binding{}, wire.ErrNotFound
	}
	b, err := c.fetch(ctx, hash)
	switch {
	case err == nil:
		c.store(hash, cachedBinding{b: b, found: true, until: t.Add(c.ttl)})
	case errors.Is(err, wire.ErrNotFound):
		c.store(hash, cachedBinding{until: t.Add(negativeTTL)})
	}
	return b, err
}

func (c *bindingClient) store(hash string, e cachedBinding) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.cache) >= maxCached {
		t := c.now()
		for k, v := range c.cache {
			if !t.Before(v.until) {
				delete(c.cache, k)
			}
		}
		if len(c.cache) >= maxCached { // every entry live: start again rather than grow
			c.cache = map[string]cachedBinding{}
		}
	}
	c.cache[hash] = e
}
```

The lookup, same file. Nothing it returns wraps keysmith's body or the caller token:

```go
func (c *bindingClient) fetch(ctx context.Context, hash string) (wire.Binding, error) {
	raw, err := os.ReadFile(c.tokenFile)
	tok := string(bytes.TrimSpace(raw))
	if err != nil || tok == "" {
		return wire.Binding{}, fmt.Errorf("%w: the caller token file is unreadable or empty", errBindingsUnavailable)
	}
	ctx, cancel := context.WithTimeout(ctx, lookupTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/v1/bindings/"+hash, nil)
	if err != nil {
		return wire.Binding{}, fmt.Errorf("%w: %v", errBindingsUnavailable, err)
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return wire.Binding{}, fmt.Errorf("%w: keysmith unreachable", errBindingsUnavailable)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return wire.Binding{}, wire.ErrNotFound
	default:
		return wire.Binding{}, fmt.Errorf("%w: keysmith answered %d", errBindingsUnavailable, resp.StatusCode)
	}
	var b wire.Binding
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&b); err != nil {
		return wire.Binding{}, fmt.Errorf("%w: keysmith's binding could not be decoded", errBindingsUnavailable)
	}
	if b.KeyHash != hash { // decision (d)
		return wire.Binding{}, fmt.Errorf("%w: keysmith answered for another key", errBindingsUnavailable)
	}
	return b, nil
}
```

The lookup has its own 2-second deadline under the request's context, so a client that gives up cancels its own lookup and nobody else's (the misses are not shared, unlike task 13's; at most one lookup per token per TTL makes sharing unnecessary).

- [ ] **Step 7: Write the wiring**

Create `control-plane/cmd/gatekeeper/main.go`, first half:

```go
// Command gatekeeper is the only listener a sandbox can reach (P6-D1). It
// authorises each request (task 10), rewrites one JSON value, drops
// LiteLLM's control fields and relays the rest (task 11), and answers
// /v1/models and HEAD /api/hello itself.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/gatekeeper/authz"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/gatekeeper/proxy"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/catalogue"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/wire"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/routesource"
)

const (
	listenAddr    = ":4000"
	shutdownGrace = 25 * time.Second
)

func main() { os.Exit(run(context.Background(), os.Getenv, os.Stdout, os.Stderr)) }

func run(ctx context.Context, getenv func(string) string, stdout, stderr io.Writer) int {
	fail := func(err error) int { fmt.Fprintln(stderr, "gatekeeper:", err); return 1 }
	cfg, err := loadConfig(getenv)
	if err != nil {
		return fail(err)
	}
	cat, err := catalogue.Load()
	if err != nil {
		return fail(err)
	}
	routes, err := buildRoutes(cfg)
	if err != nil {
		return fail(err)
	}
	log := slog.New(slog.NewJSONHandler(stdout, nil))
	h, err := newHandler(deps{catalogue: cat, models: cat.ModelsResponse(), upstream: cfg.LiteLLM, log: log,
		authz: &authz.Authorizer{Bindings: newBindingClient(cfg.KeysmithURL, cfg.CallerTokenFile, cfg.BindingTTL, nil),
			Routes: routes}})
	if err != nil {
		return fail(err)
	}
	srv := newServer(listenAddr, h)
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGTERM, os.Interrupt)
	defer stop()
	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
		defer cancel()
		_ = srv.Shutdown(sctx)
	}()
	fmt.Fprintf(stderr, "gatekeeper: listening on %s, route source %s\n", listenAddr, cfg.RouteSource)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fail(err)
	}
	return 0
}
```

Second half, same file: the route source, the handler and the server.

```go
func buildRoutes(cfg config) (wire.RouteSource, error) {
	var inner wire.RouteSource
	switch cfg.RouteSource {
	case "file":
		f, err := routesource.NewFile(cfg.RouteFile)
		if err != nil {
			return nil, err
		}
		inner = f
	case "hub":
		tok, err := routesource.ClientCredentialsFromEnv(cfg.HubScope) // the realm, as panacea-app-gateway (OD-30)
		if err != nil {
			return nil, err
		}
		h, err := routesource.NewHub(cfg.HubURL, tok, nil)
		if err != nil {
			return nil, err
		}
		inner = h
	default:
		return nil, fmt.Errorf("route source %q", cfg.RouteSource) // loadConfig already refused it
	}
	return routesource.NewCache(inner, cfg.RouteTTL, cfg.RouteStaleMax, nil)
}

type deps struct {
	catalogue proxy.Resolver
	models    []byte // catalogue.ModelsResponse(), served verbatim
	authz     proxy.Decider
	upstream  *url.URL
	log       *slog.Logger
	now       func() time.Time
}

// newHandler is a switch on the exact path (decision a). Health and the
// warming probe are neither authorised nor logged.
func newHandler(d deps) (http.Handler, error) {
	if d.now == nil {
		d.now = time.Now
	}
	observe := func(o proxy.Outcome) {
		d.log.Info("request", "session", o.Session, "project", o.Project, "route", string(o.Route),
			"path", o.Path, "status", o.Status, "ms", o.Elapsed.Milliseconds())
	}
	p, err := proxy.New(proxy.Config{Upstream: d.upstream, Catalogue: d.catalogue, Authz: d.authz, Now: d.now, Observe: observe})
	if err != nil {
		return nil, err
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/hello" && r.Method == http.MethodHead:
			w.WriteHeader(http.StatusNoContent)
		case (r.URL.Path == "/healthz" || r.URL.Path == "/readyz") && r.Method == http.MethodGet:
			_, _ = w.Write([]byte("ok\n"))
		case r.URL.Path == "/v1/models" && r.Method == http.MethodGet:
			serveModels(w, r, d, observe)
		default:
			p.ServeHTTP(w, r) // task 11: the two Messages paths, or 404 path_not_allowed
		}
	}), nil
}

func serveModels(w http.ResponseWriter, r *http.Request, d deps, observe func(proxy.Outcome)) {
	start := d.now()
	o := proxy.Outcome{Path: "/v1/models"}
	defer func() { o.Elapsed = d.now().Sub(start); observe(o) }()
	dec := d.authz.Decide(r.Context(), proxy.RequestFrom(r, start))
	if !dec.Allow {
		o.Status, o.Path = dec.Status, "/v1/models"
		code := dec.Code
		if o.Status < 400 || code == "" {
			o.Status, code = http.StatusInternalServerError, httpx.CodeInternal
		}
		httpx.WriteProblem(w, o.Status, code, "The inference gateway refused this request.", nil)
		return
	}
	o.Session, o.Project, o.Route, o.Status = dec.Binding.SessionID, dec.Binding.ProjectID, dec.Route, http.StatusOK
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(d.models)
}

// newServer: no WriteTimeout and no ReadTimeout, because a stream legitimately
// runs for minutes and a long prompt uploads for as long as it takes; the
// header deadline stops a connection that never sends a request.
func newServer(addr string, h http.Handler) *http.Server {
	return &http.Server{Addr: addr, Handler: h, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 120 * time.Second}
}
```

`/v1/models` does not consult the route: every catalogue entry has both groups (task 02), so the list is the same on either route, and a project whose route is unknown fails at its first `POST` with `503 route_unknown` rather than being told it has no models. Task 10 still checks the live route for this request, so an unknown route is refused here too; that is task 10's order, not a choice made here.

- [ ] **Step 8: Run the tests to verify they pass**

```bash
cd control-plane
go test -race -count=1 ./cmd/gatekeeper/                                   # expect: ok
go test -count=1 -v ./cmd/gatekeeper/ | grep -c '^--- PASS: Test'          # expect 12
```

- [ ] **Step 9: Kill four mutants by hand, and prove the start-up refuses**

| Edit | Test that must fail |
|---|---|
| `observe`: add `"model", r.Header.Get("Anthropic-Beta")`, or any seventh field | `TestNoBodyTokenOrHeaderValueReachesALog` (the key set) |
| `newHandler`: the switch replaced by an `http.ServeMux` with `mux.Handle("/", p)` and the three paths registered | `TestNoPathIsRedirected` |
| `newServer`: `WriteTimeout: 5 * time.Minute` added | `TestServerNeverCutsAStream` |
| `ByKeyHash`: the `case errors.Is(err, wire.ErrNotFound):` arm stores with `c.ttl` instead of `negativeTTL` | `TestANotFoundIsCachedForTwoSeconds` |

Then, with no route source configured:

```bash
cd control-plane
PANACEA_APP_GATEWAY_CALLER_TOKEN_FILE=/tmp/x go run ./cmd/gatekeeper; echo "exit $?"
```

Expected, two lines: `gatekeeper: PANACEA_APP_GATEWAY_ROUTE_SOURCE: "" must be hub or file:<path>; there is no default route` and `exit 1`.

- [ ] **Step 10: Vet, format, build**

```bash
cd control-plane
go vet ./cmd/gatekeeper/ && gofmt -l cmd/gatekeeper/          # expect no output
CGO_ENABLED=0 go build -o /dev/null ./cmd/gatekeeper/          # expect no output; task 17's distroless image needs a static binary
```

- [ ] **Step 11: Commit**

```bash
git add control-plane/cmd/gatekeeper/main.go control-plane/cmd/gatekeeper/config.go control-plane/cmd/gatekeeper/bindings.go control-plane/cmd/gatekeeper/main_test.go control-plane/cmd/gatekeeper/config_test.go control-plane/cmd/gatekeeper/bindings_test.go
git commit -F <message file> -- control-plane/cmd/gatekeeper/
```

Message: `feat(gatekeeper): the binary, /v1/models and /api/hello answered locally, body-free logs (phase 6 task 12)`, a body naming decisions (a) to (e), and the attribution line.

**Acceptance:** step 8 prints `ok` and `12`; step 9's four edits each fail the named test and the start-up prints the two lines; step 10 prints nothing.

**What this task does not claim:**

- **TLS on the private path.** Sandbox to gatekeeper and gatekeeper to LiteLLM and keysmith are plain HTTP inside phase 8's network (NetworkPolicies, task 19; P8-3). The session key crosses that network in the clear; mTLS is seam S6.10.
- **That the `/v1/models` body is what Claude Code's picker reads.** The bytes are task 02's `ModelsResponse`, whose field set is **UNVERIFIED** until task 23 watches the CLI.
- **That the standard logger never carries a body.** The canary test captures `net/http` and `httputil` output for the three failure paths it drives. A panic's stack trace, or a Go release that logs more, is not covered; the recovery `net/http` does on a handler panic logs the panic value, which no code here builds from a body.
- **That streams survive a rollout.** Decision (e): 25 seconds, then cut.
- **A request rate limit.** A sandbox may send as fast as LiteLLM answers; its budget (task 06) is the bound, and LiteLLM's own rate limits, if configured, a second one.

**Gaps against the skeleton, for the orchestrator:**

1. `PANACEA_APP_GATEWAY_CALLER_TOKEN_FILE` (the gatekeeper's credential to keysmith) and `PANACEA_APP_GATEWAY_HUB_SCOPE` are not in the skeleton's shared names; task 15 already uses the first for the local control plane, and task 13's gap 1 explains the second. Tasks 17 and 19 must set both.
2. The skeleton's single `main_test.go` is three test files (decision list, Files).
3. Task 10's handling of a non-`ErrNotFound` binding error is assumed to be a `503`, not a `401`; this task's client guarantees the distinction (decision d), task 10 decides the status.
4. `authz.Authorizer` is assumed to need only `Bindings` and `Routes`, the skeleton's two fields.
