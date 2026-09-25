### Task 15: `gatewayclient` and the local control plane's launch

This task is the launcher's side of the gateway for local mode: the one client every launcher uses to talk to keysmith, and the local control plane using it. A session starts exactly as it does today; beside it, the control plane mints the session's token, writes it where `panacea token` (task 14) reads it, refreshes it, and revokes it when the session ends. In the cloud the broker does the same through the same client (ask P8-1); this task does not build that.

**The one thing this task must not get wrong is that session start never waits on a mint.** Panacea's `/key/generate` ran inside a request's shared 15-second context and took the next call down with it (skeleton pitfall 3). Here the sandbox is created, the session is recorded and answered `201`, and only then, in a goroutine with its own deadline, is the token minted. The one process that waits is `panacea token`, inside the sandbox, for at most 20 seconds.

Seven decisions where the skeleton is silent:

- **(a) The route is read before the sandbox exists.** `CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS=1` must be in an AU session's launch environment (P6-D3), and the environment is fixed at create. So `Prepare` reads the project's effective route from task 13's `File` source first. An **unknown** route sets the variable too: the mint will refuse that project anyway (`route_unknown`), and the conservative environment costs nothing. A project **raised** to AU mid-session keeps its launch environment; its requests are served AU from the next one (P6-D4) but may carry pre-release betas Bedrock rejects. The session JSON's `gatewayRoute` shows the route at launch so the cockpit can say "restart to apply AU fully"; task 22 decides whether to.
- **(b) The pod address is the container's address on the gateway network.** `backend.Spec.Network` joins it (docker only, skeleton); `backend.Sandbox.NetworkAddr` is new and carries the address back, read from `docker inspect`'s `NetworkSettings.Networks[<network>]`. A sandbox created on the network but reporting no address is recorded `gateway: "error"` and gets no token.
- **(c) State survives a control-plane restart.** `registry.Session` gains `Gateway`, `GatewayError`, `GatewayGeneration` and `GatewayRoute`. On start, `cmd/api` resumes a refresher for every live session whose gateway is `on`, and its first action is a refresh, because nobody knows how long the control plane was down. Without this, every running session's token dies an hour after a restart.
- **(d) A restart of one session re-mints, it does not refresh.** Restart destroys the sandbox and makes a new one with a new address, and the binding is pod-bound (task 10). So restart revokes the session's keys, then mints generation `GatewayGeneration + 1` for the new address. Whether keysmith accepts a mint for a session it has revoked is task 07's; this part assumes it does, keyed on the new generation, and lists it as a gap.
- **(e) The token file is `0644` in a `0755` session directory under a `0700` root.** The skeleton says `0600`. On a Linux Docker host a bind mount keeps the host's owner and mode, and the sandbox's `dev` user (uid 1001, `images/sandbox/Dockerfile:19`) is not the host user, so a `0600` token would be unreadable inside and every session would fail. The `0700` root (`<state dir>/gateway-tokens/`) keeps other host users out; inside the container only the one mounted directory is visible, read-only. On Docker Desktop for Windows, where the product runs today, file modes on bind mounts are emulated and none of this bites either way.
- **(f) Retries repeat the same generation and nothing else.** Mint and refresh retry twice on a timeout, a connection failure or a 5xx (including keysmith's own `504 mint_timeout`), with the same request body, which task 07 makes safe. A 4xx is never retried: `budget_exhausted` or `route_unknown` will not change in 500 ms.
- **(g) Revocation failures do not block deletion.** Delete destroys the sandbox whatever keysmith says; a failed revoke is logged, and the key dies at its one-hour expiry anyway, off a pod that no longer exists (pod binding makes it useless meanwhile).

**Files:**
- `control-plane/internal/gatewayclient/client.go` — `Client`, `New`, `Mint`, `Refresh`, `RevokeSession`, `Usage`, `Problem`, `WriteToken` (new)
- `control-plane/internal/gatewayclient/refresh.go` — `Refresher`, `Launcher`, `Prepare`, `Start`, `Stop`, `Resume` (new)
- `control-plane/internal/gatewayclient/client_test.go` — the client and launcher tests (new)
- `control-plane/internal/backend/backend.go` — `Spec.Network`, `Sandbox.NetworkAddr`, `LabelNetwork`, the network-name check in `ValidateSpec` (modified)
- `control-plane/internal/backend/backend_test.go` — the network-name table (modified)
- `control-plane/internal/backend/dockerbe/docker.go`, `docker_test.go` — join the network, label it, read the address back (modified)
- `control-plane/internal/backend/katabe/kata.go`, `kata_test.go` — refuse `Network` (modified)
- `control-plane/internal/registry/session.go` — the four gateway fields (modified)
- `control-plane/internal/api/api.go` (`Deps.Gateway`), `control-plane/internal/api/sessions.go` (create, restart, delete, `sessionJSON`) (modified)
- `control-plane/internal/api/sessions_gateway_test.go` (new), `control-plane/internal/api/sessions_test.go` (the fake backend reports a `NetworkAddr`, one field) (modified)
- `control-plane/cmd/api/config.go`, `config_test.go`, `main.go` — the gateway variables, the `Launcher`, the resume loop (modified)

**Interfaces:**
- Produces, package `gatewayclient`:
  - `New(keysmithURL, callerTokenFile string, o Options) (*Client, error)`; `Options{Timeout time.Duration (default 20 s, MINT_TIMEOUT); Retries int (default 2; negative means none); Backoff time.Duration (default 500 ms); HTTP *http.Client}`
  - `(*Client) Mint(ctx, wire.MintRequest) (wire.MintResponse, error)`, `Refresh(ctx, wire.RefreshRequest) (wire.MintResponse, error)`, `RevokeSession(ctx, sessionID string) error`, `Usage(ctx, scope, id string) ([]wire.UsageRow, error)` (task 21's reader; no retry)
  - `Problem{Status int; Code, Message string}`, the error a keysmith refusal becomes; `errors.As` finds it
  - `WriteToken(dir, token string) error`: `token.tmp` at `0644`, `fsync`, rename to `token`
  - `Launcher{Client *Client; Routes wire.RouteSource; BaseURL, Network, TokenRoot string; RefreshAt, RaiseCheck time.Duration (`RaiseCheck`, 0 meaning one minute: how often a Global-minted session re-reads its route, so a raise refreshes at once, review C-1); Owner wire.Owner; TeamID string; Record func(sessionID string, s State); Log *slog.Logger}`, built by `NewLauncher(l Launcher) (*Launcher, error)`, which creates `TokenRoot` at `0700`
  - `State{Gateway string ("pending"|"on"|"error"); Error string; Generation int; Route dataclass.Route}`
  - `Prep{Mount backend.Mount; Env map[string]string; Network string; Route dataclass.Route}`
  - `(*Launcher) Prepare(ctx, sessionID, projectID string) (Prep, error)`; `Start(sessionID, projectID, agentID string, generation int, pod netip.Addr)`, which returns at once; `Stop(ctx, sessionID string, keepDir bool) error`; `Resume(sessionID, projectID string, generation int)`
- Produces, package `backend`: `Spec.Network string` (docker only; `""` is the default bridge); `Sandbox.NetworkAddr netip.Addr` (docker only; the zero value when `Spec.Network` was empty); `LabelNetwork = "panacea-app.network"`.
- Produces, the session wire shape (additive to phase 2's `sessionJSON`, `control-plane/internal/api/sessions.go:593-620`): `"gateway": "off"|"pending"|"on"|"error"`, `"gatewayError": "<one sentence>"`, `"gatewayRoute": "global"|"au"|""`.
- Produces, `cmd/api` variables: `PANACEA_APP_GATEWAY_KEYSMITH_URL` (unset: the gateway is off), `_BASE_URL` (default `http://gatekeeper:4000`, the address **inside** the gateway network), `_NETWORK` (default `panacea-app-gateway`, task 17's network), `_CALLER_TOKEN_FILE` (required when on), `_ROUTE_SOURCE` (required when on; `file:<path>` only until S6.1), `_MINT_TIMEOUT` (`20s`), `_REFRESH_AT` (`45m`, and below `55m`), `_OWNER` (`<tid>:<oid>`, default `local:developer`), `_TEAM` (default `local`).
- Consumes: `wire.MintRequest`, `MintResponse`, `RefreshRequest`, `RevokeRequest`, `UsageRow`, `Owner`, `RouteSource`, `ErrRouteUnknown`, `ProjectInference.LiveRoute` (task 03; `Prepare` and the raise check read the route by it, so the refresher sees every raise keysmith retires on, re-review R-6); keysmith's `POST /v1/mint` (201), `POST /v1/refresh` (201), `POST /v1/revoke` (204), `GET /v1/usage?scope=&id=` (task 08); `routesource.NewFile` (task 13); `dataclass.Route` (task 01); `httpx.Problem` (`problem.go:26-34`).
- Required by: task 17 (the compose network and the callers file this reads), task 21 (`Usage`, and the same `ROUTE_SOURCE` file), task 22 (renders `gateway`, `gatewayError`), task 23 (the whole local path), ask P8-1 (the broker uses this client).

- [ ] **Step 1: Write the failing client tests**

A fake keysmith records every request and answers from a script. Nothing here needs Docker.

Create `control-plane/internal/gatewayclient/client_test.go`, first part:

```go
package gatewayclient

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/dataclass"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/wire"
)

// reply is one scripted answer; delay runs before it is written.
type reply struct {
	delay  time.Duration
	status int
	body   string
}

type keysmith struct {
	mu     sync.Mutex
	script []reply // the last one repeats
	calls  []string
	bodies []string
	auths  []string
}
```

The fake's server and the retry tests, same file. Bodies are built from task 03's own types with `json.Marshal`, so this file never spells a JSON key task 03 owns:

```go
func (k *keysmith) serve(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		k.mu.Lock()
		k.calls = append(k.calls, r.Method+" "+r.URL.RequestURI())
		k.bodies = append(k.bodies, string(b))
		k.auths = append(k.auths, r.Header.Get("Authorization"))
		next := k.script[min(len(k.calls), len(k.script))-1]
		k.mu.Unlock()
		select {
		case <-time.After(next.delay):
		case <-r.Context().Done():
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(next.status)
		_, _ = io.WriteString(w, next.body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func (k *keysmith) snapshot() (calls, bodies, auths []string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	return append([]string(nil), k.calls...), append([]string(nil), k.bodies...), append([]string(nil), k.auths...)
}

func minted(token string, route dataclass.Route) string {
	b, _ := json.Marshal(wire.MintResponse{Token: token, KeyID: "k-1", Route: route,
		ExpiresAt: time.Now().Add(time.Hour), BaseURL: "http://gatekeeper:4000", MaxBudgetUSD: 25})
	return string(b)
}

func newClient(t *testing.T, k *keysmith, timeout time.Duration) *Client {
	t.Helper()
	file := filepath.Join(t.TempDir(), "caller")
	if err := os.WriteFile(file, []byte("control-plane-bearer\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := New(k.serve(t).URL, file, Options{Timeout: timeout, Backoff: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

var mintReq = wire.MintRequest{SessionID: "s-1", Generation: 1, Owner: wire.Owner{TID: "local", OID: "developer"},
	TeamID: "local", ProjectID: "p-8c1d0e42", AgentID: "claude-code", SessionClass: "solo", PodAddr: "172.20.0.5"}

// Kills minting a new generation on retry (two budgeted keys for one session),
// kills no retry, and kills a retry without the caller's bearer.
func TestMintRetriesTheSameGenerationOnTimeoutAnd5xx(t *testing.T) {
	k := &keysmith{script: []reply{{delay: 200 * time.Millisecond, status: 201, body: minted("sk-pa-late", "global")},
		{status: 503, body: `{"error":"backend_unavailable","message":"x"}`}, {status: 201, body: minted("sk-pa-3", "global")}}}
	resp, err := newClient(t, k, 50*time.Millisecond).Mint(context.Background(), mintReq)
	if err != nil || resp.Token != "sk-pa-3" {
		t.Fatalf("Mint = %+v, %v", resp, err)
	}
	calls, bodies, auths := k.snapshot()
	if len(calls) != 3 {
		t.Fatalf("%d calls, want 3", len(calls))
	}
	for i := range calls {
		var got wire.MintRequest
		_ = json.Unmarshal([]byte(bodies[i]), &got)
		if calls[i] != "POST /v1/mint" || got != mintReq || auths[i] != "Bearer control-plane-bearer" {
			t.Errorf("call %d: %s %+v %q", i, calls[i], got, auths[i])
		}
	}
}

// Kills retrying a refusal: a budget or a route does not change in 500 ms.
func TestMintDoesNotRetryARefusal(t *testing.T) {
	k := &keysmith{script: []reply{{status: 402, body: `{"error":"budget_exhausted","message":"The project's budget is spent.","details":{"dimension":"project"}}`}}}
	_, err := newClient(t, k, time.Second).Mint(context.Background(), mintReq)
	var p *Problem
	if !errors.As(err, &p) || p.Status != 402 || p.Code != "budget_exhausted" {
		t.Fatalf("err = %v, want Problem 402 budget_exhausted (task-03.md:278)", err)
	}
	if calls, _, _ := k.snapshot(); len(calls) != 1 {
		t.Fatalf("%d calls, want 1", len(calls))
	}
}
```

The token file, same file:

```go
// Kills an error that carries a response body: a 500 that echoes a token
// must not put it in the session's gatewayError or a log line.
func TestErrorsNeverCarryABody(t *testing.T) {
	k := &keysmith{script: []reply{{status: 500, body: `{"error":"internal","message":"sk-pa-LEAKED"}`}, {status: 200, body: `not json sk-pa-LEAKED`}}}
	c := newClient(t, k, time.Second)
	c.retries = 0
	for i := 0; i < 2; i++ {
		if _, err := c.Mint(context.Background(), mintReq); err == nil || strings.Contains(err.Error(), "LEAKED") {
			t.Errorf("attempt %d: err = %v", i, err)
		}
	}
}

// Kills a direct write: a reader polling as panacea token does must see the
// old token or the new one, never a prefix and never an empty file.
func TestWriteTokenIsNeverSeenHalfWritten(t *testing.T) {
	dir := t.TempDir()
	long, short := "sk-pa-"+strings.Repeat("L", 3000), "sk-pa-S"
	if err := WriteToken(dir, long); err != nil {
		t.Fatal(err)
	}
	stop := make(chan struct{})
	var bad atomic.Int32
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			b, err := os.ReadFile(filepath.Join(dir, "token"))
			if err == nil && string(b) != long && string(b) != short {
				bad.Add(1)
			}
		}
	}()
	for i := 0; i < 300; i++ {
		tok := long
		if i%2 == 0 {
			tok = short
		}
		if err := WriteToken(dir, tok); err != nil {
			t.Fatal(err)
		}
	}
	close(stop)
	if n := bad.Load(); n != 0 {
		t.Fatalf("a reader saw %d partial tokens", n)
	}
	if _, err := os.Stat(filepath.Join(dir, "token.tmp")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("token.tmp left behind: %v", err)
	}
}
```

On Windows a reader can briefly get a sharing violation while the rename replaces the file; that is an error, not a partial read, and `panacea token` runs on Linux, so the test counts only successful reads that are wrong.

The launcher, same file. `routes` is a two-project fake; `recorder` collects what the launcher reports:

```go
type routes map[string]dataclass.Route

// A class LiveRoute accepts, so the chosen route decides (task 03 decision (g)).
var (
	anyRes = dataclass.Class{Level: dataclass.LevelInternal, Residency: dataclass.ResidencyAny}
	auOnly = dataclass.Class{Level: dataclass.LevelInternal, Residency: dataclass.ResidencyAUOnly}
)

func (r routes) Project(_ context.Context, id string) (wire.ProjectInference, error) {
	route, ok := r[id]
	if !ok {
		return wire.ProjectInference{}, wire.ErrRouteUnknown
	}
	return wire.ProjectInference{ProjectID: id, DataClass: anyRes, Route: route, Effective: route}, nil
}

type recorder struct {
	mu     sync.Mutex
	states map[string][]State
}

func (r *recorder) record(id string, s State) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.states[id] = append(r.states[id], s)
}

func (r *recorder) last(id string) State {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s := r.states[id]; len(s) > 0 {
		return s[len(s)-1]
	}
	return State{}
}

func newLauncher(t *testing.T, k *keysmith) (*Launcher, *recorder) {
	t.Helper()
	rec := &recorder{states: map[string][]State{}}
	l, err := NewLauncher(Launcher{Client: newClient(t, k, 5*time.Second), BaseURL: "http://gatekeeper:4000",
		Routes: routes{"p-global": dataclass.RouteGlobal, "p-au": dataclass.RouteAU}, Network: "panacea-app-gateway",
		TokenRoot: filepath.Join(t.TempDir(), "gateway-tokens"), RefreshAt: time.Hour, TeamID: "local",
		Owner: wire.Owner{TID: "local", OID: "developer"}, Record: rec.record, Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	return l, rec
}
```

The launcher tests, same file:

```go
func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); !cond(); time.Sleep(5 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
	}
}

func tokenIn(l *Launcher, session string) string {
	b, _ := os.ReadFile(filepath.Join(l.TokenRoot, session, "token"))
	return string(b)
}

var pod = netip.MustParseAddr("172.20.0.5")

// Kills the Panacea shared-deadline chain: Start must not wait for keysmith.
func TestStartReturnsBeforeTheMintLands(t *testing.T) {
	k := &keysmith{script: []reply{{delay: 2 * time.Second, status: 201, body: minted("sk-pa-1", "global")}}}
	l, rec := newLauncher(t, k)
	if _, err := l.Prepare(context.Background(), "s-1", "p-global"); err != nil {
		t.Fatal(err)
	}
	began := time.Now()
	l.Start("s-1", "p-global", "claude-code", 1, pod)
	if d := time.Since(began); d > 200*time.Millisecond {
		t.Fatalf("Start took %s with keysmith still sleeping", d)
	}
	eventually(t, "the token", func() bool { return tokenIn(l, "s-1") == "sk-pa-1" })
	if s := rec.last("s-1"); s.Gateway != "on" || s.Generation != 1 || s.Route != dataclass.RouteGlobal {
		t.Errorf("state %+v", s)
	}
	_, bodies, _ := k.snapshot()
	var got wire.MintRequest
	_ = json.Unmarshal([]byte(bodies[0]), &got)
	if got.PodAddr != "172.20.0.5" || got.ProjectID != "p-global" || got.Generation != 1 {
		t.Errorf("mint request %+v", got)
	}
}

// Kills a failure that leaves a file behind or says nothing.
func TestAFailedMintRecordsAnErrorAndWritesNoToken(t *testing.T) {
	k := &keysmith{script: []reply{{status: 503, body: `{"error":"route_unknown","message":"This project's inference route is not known."}`}}}
	l, rec := newLauncher(t, k)
	_, _ = l.Prepare(context.Background(), "s-1", "p-global")
	l.Start("s-1", "p-global", "claude-code", 1, pod)
	eventually(t, "the error", func() bool { return rec.last("s-1").Gateway == "error" })
	if s := rec.last("s-1"); !strings.Contains(s.Error, "route is not known") || tokenIn(l, "s-1") != "" {
		t.Errorf("state %+v, token %q", s, tokenIn(l, "s-1"))
	}
}

// Kills a credential in the launch environment (it outranks the helper), and
// kills the AU flag keyed on anything but the route at launch.
func TestPrepareSetsTheEnvironmentByRoute(t *testing.T) {
	l, _ := newLauncher(t, &keysmith{script: []reply{{status: 201, body: minted("x", "global")}}})
	for project, wantFlag := range map[string]bool{"p-global": false, "p-au": true, "p-unknown": true} {
		p, err := l.Prepare(context.Background(), "s-"+project, project)
		if err != nil {
			t.Fatal(err)
		}
		if p.Env["ANTHROPIC_BASE_URL"] != "http://gatekeeper:4000" || (p.Env["CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS"] == "1") != wantFlag {
			t.Errorf("%s: env %v", project, p.Env)
		}
		for _, banned := range []string{"ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "CLAUDE_CODE_USE_BEDROCK"} {
			if _, ok := p.Env[banned]; ok {
				t.Errorf("%s: env sets %s", project, banned)
			}
		}
		if p.Mount.ContainerPath != "/run/panacea/gateway" || !p.Mount.ReadOnly || p.Network != "panacea-app-gateway" {
			t.Errorf("%s: mount %+v network %q", project, p.Mount, p.Network)
		}
		if st, err := os.Stat(p.Mount.HostPath); err != nil || !st.IsDir() {
			t.Errorf("%s: the token directory does not exist: %v", project, err)
		}
	}
}
```

The session's lifetime, same file:

```go
// Kills a refresher that never fires, one that sends the current generation
// instead of the new one (task 03's RefreshRequest names the NEW generation,
// and keysmith refuses a refresh of generation 1 as invalid, task 07), and one
// that outlives its session.
func TestTheRefresherRefreshesThenStopsWithTheSession(t *testing.T) {
	k := &keysmith{script: []reply{{status: 201, body: minted("sk-pa-1", "global")},
		{status: 201, body: minted("sk-pa-2", "global")}, {status: 204}}}
	l, rec := newLauncher(t, k)
	l.RefreshAt = 50 * time.Millisecond
	_, _ = l.Prepare(context.Background(), "s-1", "p-global")
	l.Start("s-1", "p-global", "claude-code", 1, pod)
	eventually(t, "the refreshed token", func() bool { return tokenIn(l, "s-1") == "sk-pa-2" })
	eventually(t, "generation 2", func() bool { return rec.last("s-1").Generation == 2 })
	if err := l.Stop(context.Background(), "s-1", false); err != nil {
		t.Fatal(err)
	}
	calls, bodies, _ := k.snapshot()
	var refresh wire.RefreshRequest
	_ = json.Unmarshal([]byte(bodies[1]), &refresh)
	if calls[1] != "POST /v1/refresh" || refresh != (wire.RefreshRequest{SessionID: "s-1", Generation: 2}) {
		t.Errorf("second call %s %+v, want a refresh minting generation 2", calls[1], refresh)
	}
	n := len(calls)
	time.Sleep(200 * time.Millisecond) // four refresh periods
	if after, _, _ := k.snapshot(); len(after) != n {
		t.Errorf("%d calls after Stop, want none", len(after)-n)
	}
}

// raisable is a route source a test can raise while a refresher reads it, by
// its route or by its class. A class raise leaves Effective stale on Global,
// as a source that has not recomputed it would (re-review R-6).
type raisable struct {
	mu    sync.Mutex
	r     dataclass.Route
	class dataclass.Class
}

func (s *raisable) Project(_ context.Context, id string) (wire.ProjectInference, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return wire.ProjectInference{ProjectID: id, DataClass: s.class, Route: s.r, Effective: s.r}, nil
}

// Review C-1, task 07 decision (h). Kills a refresher that waits for
// RefreshAt (an hour here) after its project is raised from Global to AU,
// and (the class row, re-review R-6) one that reads Effective alone, which
// misses an AU-only class whose Effective is still Global.
func TestARaiseRefreshesAtOnce(t *testing.T) {
	raises := map[string]func(*raisable){
		"route": func(s *raisable) { s.r = dataclass.RouteAU },
		"class": func(s *raisable) { s.class = auOnly },
	}
	for name, raise := range raises {
		t.Run(name, func(t *testing.T) {
			k := &keysmith{script: []reply{{status: 201, body: minted("sk-pa-1", "global")},
				{status: 201, body: minted("sk-pa-2", "au")}, {status: 204}}}
			l, rec := newLauncher(t, k)
			src := &raisable{r: dataclass.RouteGlobal, class: anyRes}
			l.Routes, l.RaiseCheck = src, 10*time.Millisecond
			_, _ = l.Prepare(context.Background(), "s-1", "p-global")
			l.Start("s-1", "p-global", "claude-code", 1, pod)
			eventually(t, "the first token", func() bool { return tokenIn(l, "s-1") == "sk-pa-1" })
			time.Sleep(50 * time.Millisecond) // five checks on Global: no refresh yet
			if calls, _, _ := k.snapshot(); len(calls) != 1 {
				t.Fatalf("%d keysmith calls before the raise, want only the mint", len(calls))
			}
			src.mu.Lock()
			raise(src)
			src.mu.Unlock()
			eventually(t, "the AU token", func() bool { return tokenIn(l, "s-1") == "sk-pa-2" })
			eventually(t, "the AU route recorded", func() bool { return rec.last("s-1").Route == dataclass.RouteAU })
			if err := l.Stop(context.Background(), "s-1", false); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// Kills a leaked live key on delete, and kills a directory left behind.
func TestStopRevokesAndRemovesTheDirectory(t *testing.T) {
	k := &keysmith{script: []reply{{status: 201, body: minted("sk-pa-1", "global")}, {status: 204}}}
	l, _ := newLauncher(t, k)
	p, _ := l.Prepare(context.Background(), "s-1", "p-global")
	l.Start("s-1", "p-global", "claude-code", 1, pod)
	eventually(t, "the token", func() bool { return tokenIn(l, "s-1") == "sk-pa-1" })
	if err := l.Stop(context.Background(), "s-1", false); err != nil {
		t.Fatal(err)
	}
	calls, bodies, _ := k.snapshot()
	var rv wire.RevokeRequest
	_ = json.Unmarshal([]byte(bodies[len(bodies)-1]), &rv)
	if calls[len(calls)-1] != "POST /v1/revoke" || rv.SessionID != "s-1" {
		t.Errorf("last call %s %+v, want a revoke of s-1", calls[len(calls)-1], rv)
	}
	if _, err := os.Stat(p.Mount.HostPath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the token directory survived Stop: %v", err)
	}
}

// Kills a restart that waits REFRESH_AT before its first refresh: the
// control plane may have been down for most of the token's hour.
func TestResumeRefreshesAtOnce(t *testing.T) {
	k := &keysmith{script: []reply{{status: 201, body: minted("sk-pa-4", "au")}}}
	l, rec := newLauncher(t, k)
	_, _ = l.Prepare(context.Background(), "s-1", "p-au")
	l.Resume("s-1", "p-au", 3)
	eventually(t, "the resumed token", func() bool { return tokenIn(l, "s-1") == "sk-pa-4" })
	if s := rec.last("s-1"); s.Generation != 4 || s.Gateway != "on" {
		t.Errorf("state %+v, want generation 4, on", s)
	}
	if calls, _, _ := k.snapshot(); calls[0] != "POST /v1/refresh" {
		t.Errorf("first call %s, want a refresh", calls[0])
	}
	_ = l.Stop(context.Background(), "s-1", true)
}
```

- [ ] **Step 2: Write the failing backend tests**

`control-plane/internal/backend/backend_test.go`, one table added:

```go
// Kills passing a caller string to the daemon as a network name unchecked.
func TestValidateSpecChecksTheNetworkName(t *testing.T) {
	for name, ok := range map[string]bool{"": true, "panacea-app-gateway": true, "net_1.a": true,
		"host": false, "none": false, "-lead": false, "a b": false, "container:abc": false, strings.Repeat("n", 64): false} {
		spec := validSpec() // this file's existing helper (package backend_test, backend_test.go:12)
		spec.Network = name
		if err := backend.ValidateSpec(spec); (err == nil) != ok {
			t.Errorf("Network %q: err = %v, want ok=%v", name, err, ok)
		}
	}
}
```

`host` and `none` are refused by name: `host` would put the sandbox on the host's network namespace (the dockerbe comment at `docker.go:235-238` already rules host networking out), and `container:<id>` would join another container's.

`control-plane/internal/backend/dockerbe/docker_test.go`, one test added, built from the file's own helpers (`healthzAddr`, `portOf`, `labelsFor`, `inspectOf`, `notFoundErr`, `newBackend`, `validSpec`; `docker_test.go:213-266`, `:311-317`). Add `net/netip` and the SDK's `network` package to its imports if absent:

```go
// Kills a sandbox that never joins the gateway network, and kills reading the
// address off the wrong network: a container on the default bridge as well
// would otherwise bind its token to an address the gatekeeper never sees.
func TestCreateJoinsTheGatewayNetworkAndReadsItsAddress(t *testing.T) {
	addr := healthzAddr(t)
	labels := labelsFor("s-3f2a91c0", "sess-1")
	labels[backend.LabelNetwork] = "panacea-app-gateway"
	running := inspectOf("c0ffeecafe", "running", "127.0.0.1", portOf(t, addr), labels)
	if running.NetworkSettings == nil {
		t.Fatal("inspectOf built no NetworkSettings")
	}
	running.NetworkSettings.Networks = map[string]*network.EndpointSettings{
		"bridge": {IPAddress: "172.17.0.9"}, "panacea-app-gateway": {IPAddress: "172.20.0.5"}}
	f := &fakeDocker{createID: "c0ffeecafe"}
	f.inspects = []inspectResult{{err: notFoundErr{"panacea-app-s-3f2a91c0"}}, {resp: running}}
	spec := validSpec()
	spec.Network = "panacea-app-gateway"
	sb, err := newBackend(t, f).Create(context.Background(), spec)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	c := f.snapshotCreates()[0]
	if string(c.hostConfig.NetworkMode) != "panacea-app-gateway" || c.config.Labels[backend.LabelNetwork] != "panacea-app-gateway" {
		t.Errorf("NetworkMode %q, label %q", c.hostConfig.NetworkMode, c.config.Labels[backend.LabelNetwork])
	}
	if sb.NetworkAddr != netip.MustParseAddr("172.20.0.5") {
		t.Errorf("NetworkAddr = %s, want the gateway network's 172.20.0.5", sb.NetworkAddr)
	}
}
```

The existing `TestCreateBuildsTheContractedContainerAndReturnsALoopbackEndpoint` keeps passing unchanged, and that is its own assertion that nothing moved when `Network` is empty: it checks `networking == nil` (`:333`) and exactly five labels (`:369`).

`control-plane/internal/backend/katabe/kata_test.go`, beside `TestCreateRejectsMountsAndCreatesNoPod` (`kata_test.go:208`):

```go
// Kills a Kata backend that quietly ignores Network: in the cloud the pod's
// network is phase 8's NetworkPolicy, never a Docker network name.
func TestCreateRejectsANetworkAndCreatesNoPod(t *testing.T) {
	client := fake.NewSimpleClientset()
	b := newBackend(t, client, 0, time.Second)
	spec := validSpec()
	spec.Network = "panacea-app-gateway"
	if _, err := b.Create(context.Background(), spec); !errors.Is(err, backend.ErrUnsupported) {
		t.Fatalf("Create(with a network) = %v, want ErrUnsupported", err)
	}
	pods, err := client.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil || len(pods.Items) != 0 {
		t.Fatalf("pods %d, err %v; the check must run before the API call", len(pods.Items), err)
	}
}
```

- [ ] **Step 3: Write the failing API tests**

In `control-plane/internal/api/sessions_test.go`, the fake backend (`sessions_test.go:399-429`) gains one field, `networkAddr netip.Addr`, and its `Create` sets `NetworkAddr: f.networkAddr` on the `backend.Sandbox` it returns. Nothing else in that file changes.

Create `control-plane/internal/api/sessions_gateway_test.go`:

```go
package api

import (
	"context"
	"net/http"
	"net/netip"
	"sync"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/dataclass"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/gatewayclient"
)

type startCall struct {
	session, project, agent string
	gen                     int
	pod                     netip.Addr
}

type fakeGateway struct {
	mu      sync.Mutex
	prep    gatewayclient.Prep
	started []startCall
	stopped map[string]bool // session → keepDir
}

func (f *fakeGateway) Prepare(context.Context, string, string) (gatewayclient.Prep, error) { return f.prep, nil }

func (f *fakeGateway) Start(s, p, a string, gen int, pod netip.Addr) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.started = append(f.started, startCall{s, p, a, gen, pod})
}

func (f *fakeGateway) Stop(_ context.Context, s string, keepDir bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopped[s] = keepDir
	return nil
}
```

The four API tests, same file, on phase 2's harness (`sessionsNewEnvOverWith`, `sessionsMustStore`, `sessionsStubReady`, `sessionsCreateBody`, `sessionsDecodeOne`; `sessions_test.go:374`, `:593`, `:709`, `:754`, `:680`):

```go
func gatewayEnv(t *testing.T, gw *fakeGateway) (*sessionsEnv, *sessionsStore) {
	t.Helper()
	st := sessionsMustStore(t)
	env := sessionsNewEnvOverWith(t, t.TempDir(), st, st, func(d *Deps) {
		if gw != nil {
			d.Gateway = gw
		}
	})
	env.be.networkAddr = netip.MustParseAddr("172.20.0.5")
	sessionsStubReady(t, nil)
	return env, st
}

func auPrep(dir string) gatewayclient.Prep {
	return gatewayclient.Prep{Network: "panacea-app-gateway", Route: dataclass.RouteAU,
		Mount: backend.Mount{HostPath: dir, ContainerPath: "/run/panacea/gateway", ReadOnly: true},
		Env:   map[string]string{"ANTHROPIC_BASE_URL": "http://gatekeeper:4000", "CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS": "1"}}
}

// Kills forgetting the network, the environment, the mount or the mint, and
// kills minting before the sandbox (and so its address) exists.
func TestCreateWithTheGatewayJoinsTheNetworkAndStartsTheMintAfterTheSandbox(t *testing.T) {
	gw := &fakeGateway{prep: auPrep(t.TempDir()), stopped: map[string]bool{}}
	env, _ := gatewayEnv(t, gw)
	env.be.onCreate = func(backend.Spec) {
		gw.mu.Lock()
		defer gw.mu.Unlock()
		if len(gw.started) != 0 {
			t.Error("Start ran before Backend.Create")
		}
	}
	s := sessionsDecodeOne(t, env.do(t, http.MethodPost, "/v1/sessions", sessionsCreateBody), http.StatusCreated)
	if s.Gateway != "pending" || s.GatewayRoute != "au" {
		t.Errorf("gateway %q route %q, want pending au", s.Gateway, s.GatewayRoute)
	}
	spec := env.be.specs[0]
	if spec.Network != "panacea-app-gateway" || spec.Env["ANTHROPIC_BASE_URL"] != "http://gatekeeper:4000" ||
		spec.Env["CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS"] != "1" || spec.Mounts[len(spec.Mounts)-1] != gw.prep.Mount {
		t.Errorf("spec network %q env %v mounts %v", spec.Network, spec.Env, spec.Mounts)
	}
	want := startCall{s.ID, "p-8c1d0e42", "claude-code", 1, netip.MustParseAddr("172.20.0.5")}
	if len(gw.started) != 1 || gw.started[0] != want {
		t.Errorf("Start calls %+v, want %+v", gw.started, want)
	}
}

// Kills a recorder that writes nothing the wire shows.
func TestTheRecorderIsWhatTheSessionShows(t *testing.T) {
	gw := &fakeGateway{prep: auPrep(t.TempDir()), stopped: map[string]bool{}}
	env, st := gatewayEnv(t, gw)
	s := sessionsDecodeOne(t, env.do(t, http.MethodPost, "/v1/sessions", sessionsCreateBody), http.StatusCreated)
	record := GatewayRecorder(st, env.api.sessionLog())
	record(s.ID, gatewayclient.State{Gateway: "error", Error: "A budget for this session is spent, so no token was issued."})
	got := sessionsDecodeOne(t, env.do(t, http.MethodGet, "/v1/sessions/"+s.ID, ""), http.StatusOK)
	if got.Gateway != "error" || got.GatewayError != "A budget for this session is spent, so no token was issued." {
		t.Errorf("gateway %q error %q", got.Gateway, got.GatewayError)
	}
}

// Kills a delete that leaves the session's key live.
func TestDeleteStopsTheGateway(t *testing.T) {
	gw := &fakeGateway{prep: auPrep(t.TempDir()), stopped: map[string]bool{}}
	env, _ := gatewayEnv(t, gw)
	s := sessionsDecodeOne(t, env.do(t, http.MethodPost, "/v1/sessions", sessionsCreateBody), http.StatusCreated)
	env.do(t, http.MethodDelete, "/v1/sessions/"+s.ID, "")
	if keep, ok := gw.stopped[s.ID]; !ok || keep {
		t.Errorf("Stop(%s) called %v with keepDir %v, want called and false", s.ID, ok, keep)
	}
}

// Kills half a gateway: with it off, the sandbox is exactly phase 2's.
func TestGatewayOffLeavesTheSessionAsItWas(t *testing.T) {
	env, _ := gatewayEnv(t, nil)
	s := sessionsDecodeOne(t, env.do(t, http.MethodPost, "/v1/sessions", sessionsCreateBody), http.StatusCreated)
	spec := env.be.specs[0]
	if _, set := spec.Env["ANTHROPIC_BASE_URL"]; set || spec.Network != "" || s.Gateway != "off" {
		t.Errorf("gateway off: network %q env %v gateway %q", spec.Network, spec.Env, s.Gateway)
	}
}
```

`sessionsDecodeOne` decodes into `sessionJSON`, so the three new fields are read by the tests exactly as the desktop will read them.

- [ ] **Step 4: Run the tests to verify they fail**

```bash
cd control-plane
go test ./internal/gatewayclient/ ./internal/backend/... ./internal/api/ 2>&1 | head -20
```

Expected: build failures naming this task's symbols: `undefined: New`, `undefined: NewLauncher`, `undefined: WriteToken` (gatewayclient); `spec.Network undefined`, `sb.NetworkAddr undefined`, `undefined: backend.LabelNetwork` (backend); `d.Gateway undefined`, `undefined: GatewayRecorder`, `s.Gateway undefined` (api). If one names `wire.MintRequest` or `routesource`, task 03 or 13 is not merged: stop.

- [ ] **Step 5: Write the client**

No error this file returns quotes a reply body. A keysmith refusal becomes a `Problem` carrying the status and the **code** only (a code is checked against `^[a-z_]{1,40}$`, else `unknown`); the human sentence for it is chosen here, in `refresh.go`'s table, not taken from the wire. That is what `TestErrorsNeverCarryABody` pins, and it is why `gatewayError` on a session can never echo a token a broken keysmith put in a message.

Create `control-plane/internal/gatewayclient/client.go`:

```go
// Package gatewayclient is the one keysmith client every launcher uses: mint
// beside start, write the token atomically, refresh, revoke (phase 6 task 15).
package gatewayclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/wire"
)

const (
	DefaultTimeout = 20 * time.Second // PANACEA_APP_GATEWAY_MINT_TIMEOUT
	defaultRetries = 2
	defaultBackoff = 500 * time.Millisecond
	maxReply       = 64 << 10
	tokenMode      = 0o644 // decision (e)
)

var codeRe = regexp.MustCompile(`^[a-z_]{1,40}$`)

type Options struct {
	Timeout time.Duration // per attempt; 0 is DefaultTimeout
	Retries int           // 0 is 2; negative is none
	Backoff time.Duration // 0 is 500 ms
	HTTP    *http.Client
}

// Client talks to keysmith. Every attempt runs on its own deadline under the
// caller's context, and the body of a retry is the first attempt's, byte for byte.
type Client struct {
	base, tokenFile  string
	timeout, backoff time.Duration
	retries          int
	http             *http.Client
}

// Problem is a keysmith answer that was not the one asked for.
type Problem struct {
	Status int
	Code   string
}

func (p *Problem) Error() string { return fmt.Sprintf("gatewayclient: keysmith answered %d %s", p.Status, p.Code) }

func New(keysmithURL, callerTokenFile string, o Options) (*Client, error) {
	u, err := url.Parse(keysmithURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" ||
		(u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.User != nil {
		return nil, errors.New("gatewayclient: the keysmith URL must be http(s)://host:port with no path, query or credentials")
	}
	if callerTokenFile == "" {
		return nil, errors.New("gatewayclient: a caller token file is required")
	}
	c := &Client{base: u.Scheme + "://" + u.Host, tokenFile: callerTokenFile,
		timeout: o.Timeout, backoff: o.Backoff, retries: o.Retries, http: o.HTTP}
	if c.timeout <= 0 {
		c.timeout = DefaultTimeout
	}
	if c.backoff <= 0 {
		c.backoff = defaultBackoff
	}
	switch {
	case c.retries == 0:
		c.retries = defaultRetries
	case c.retries < 0:
		c.retries = 0
	}
	if c.http == nil {
		c.http = &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	return c, nil
}

func (c *Client) Mint(ctx context.Context, req wire.MintRequest) (wire.MintResponse, error) {
	var out wire.MintResponse
	if err := c.do(ctx, http.MethodPost, "/v1/mint", req, http.StatusCreated, &out, true); err != nil {
		return wire.MintResponse{}, err
	}
	if out.Token == "" {
		return wire.MintResponse{}, errors.New("gatewayclient: keysmith's mint reply held no token")
	}
	return out, nil
}

func (c *Client) Refresh(ctx context.Context, req wire.RefreshRequest) (wire.MintResponse, error) {
	var out wire.MintResponse
	if err := c.do(ctx, http.MethodPost, "/v1/refresh", req, http.StatusCreated, &out, true); err != nil {
		return wire.MintResponse{}, err
	}
	if out.Token == "" {
		return wire.MintResponse{}, errors.New("gatewayclient: keysmith's refresh reply held no token")
	}
	return out, nil
}

func (c *Client) RevokeSession(ctx context.Context, sessionID string) error {
	return c.do(ctx, http.MethodPost, "/v1/revoke", wire.RevokeRequest{SessionID: sessionID}, http.StatusNoContent, nil, true)
}

// Usage is task 21's read. It is not retried: the cockpit polls.
func (c *Client) Usage(ctx context.Context, scope, id string) ([]wire.UsageRow, error) {
	var out struct {
		Rows []wire.UsageRow `json:"rows"` // task 08: {"rows": [...]}, never null
	}
	q := url.Values{"scope": {scope}, "id": {id}}
	if err := c.do(ctx, http.MethodGet, "/v1/usage?"+q.Encode(), nil, http.StatusOK, &out, false); err != nil {
		return nil, err
	}
	return out.Rows, nil
}
```

The transport and the token file, same file:

```go
// do runs one call with the retry policy of decision (f).
func (c *Client) do(ctx context.Context, method, path string, in any, want int, out any, retry bool) error {
	var body []byte
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("gatewayclient: encode %s: %w", path, err)
		}
		body = b
	}
	attempts := 1
	if retry {
		attempts += c.retries
	}
	var last error
	for i := 0; i < attempts; i++ {
		if i > 0 {
			select {
			case <-time.After(c.backoff):
			case <-ctx.Done():
				return last
			}
		}
		again, err := c.once(ctx, method, path, body, want, out)
		if err == nil || !again {
			return err
		}
		last = err
	}
	return last
}

// once is one attempt. again reports whether a retry could help.
func (c *Client) once(ctx context.Context, method, path string, body []byte, want int, out any) (again bool, err error) {
	tok, err := c.bearer()
	if err != nil {
		return false, err
	}
	actx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(actx, method, c.base+path, rd)
	if err != nil {
		return false, fmt.Errorf("gatewayclient: build %s %s: %w", method, path, err)
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return true, fmt.Errorf("gatewayclient: %s %s: keysmith did not answer within %s", method, path, c.timeout)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxReply))
	if resp.StatusCode == want {
		if out != nil && json.Unmarshal(raw, out) != nil {
			return false, fmt.Errorf("gatewayclient: %s %s: keysmith's reply could not be decoded", method, path)
		}
		return false, nil
	}
	p := &Problem{Status: resp.StatusCode, Code: "unknown"}
	var hp httpx.Problem
	if json.Unmarshal(raw, &hp) == nil && codeRe.MatchString(hp.Error) {
		p.Code = hp.Error
	}
	return resp.StatusCode >= 500, p
}

func (c *Client) bearer() (string, error) {
	b, err := os.ReadFile(c.tokenFile)
	tok := strings.TrimSpace(string(b))
	if err != nil || tok == "" {
		return "", errors.New("gatewayclient: the caller token file is unreadable or empty (PANACEA_APP_GATEWAY_CALLER_TOKEN_FILE)")
	}
	return tok, nil
}

// WriteToken replaces dir/token atomically: write token.tmp, fsync, rename.
// panacea token reads only "token", so it sees the old value or the new one.
func WriteToken(dir, token string) error {
	tmp, final := filepath.Join(dir, "token.tmp"), filepath.Join(dir, "token")
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, tokenMode)
	if err != nil {
		return fmt.Errorf("gatewayclient: open %s: %w", tmp, err)
	}
	_, werr := f.WriteString(token)
	serr := f.Sync()
	cerr := f.Close()
	if err := errors.Join(werr, serr, cerr); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("gatewayclient: write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, final); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("gatewayclient: rename to %s: %w", final, err)
	}
	return nil
}
```

The `*PathError`s that `WriteToken` wraps name a path and an errno, never the token.

- [ ] **Step 6: Write the launcher and the refresher**

One goroutine per session does everything after `Start`: mint, write, then refresh every `RefreshAt`, and at once when a Global-minted session's project rises to AU (checked every `RaiseCheck`, review C-1), until `Stop` cancels its context. Each refresh names the **new** generation, as task 03's `RefreshRequest` requires. A failed refresh keeps the current token (it is valid until its hour ends) and retries a minute later; the session's `gatewayError` says so, while `gateway` stays `on`.

Create `control-plane/internal/gatewayclient/refresh.go`:

```go
package gatewayclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/dataclass"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/wire"
)

// ContainerTokenDir is the skeleton's shared name; task 14 reads it.
const ContainerTokenDir = "/run/panacea/gateway"

const refreshRetry = time.Minute

var sessionRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

// State is what the launcher reports about one session.
type State struct {
	Gateway    string // "pending" | "on" | "error"
	Error      string // one sentence, from the table below, never from the wire
	Generation int
	Route      dataclass.Route
}

// Prep is what a session's sandbox needs before it is created.
type Prep struct {
	Mount   backend.Mount
	Env     map[string]string
	Network string
	Route   dataclass.Route // effective at launch; "" when unknown
}

// Launcher is the local control plane's use of the client.
type Launcher struct {
	Client    *Client
	Routes    wire.RouteSource
	BaseURL   string
	Network   string
	TokenRoot string
	RefreshAt time.Duration
	// RaiseCheck is how often a Global-minted session re-reads its project's
	// route, so a raise to AU refreshes at once (review C-1). 0: one minute.
	RaiseCheck time.Duration
	Owner      wire.Owner
	TeamID     string
	Record     func(sessionID string, s State)
	Log        *slog.Logger

	st *launcherState // a pointer, so a Launcher value copies no lock
}

type launcherState struct {
	mu      sync.Mutex
	cancels map[string]context.CancelFunc
}

// NewLauncher validates l and creates TokenRoot at 0700.
func NewLauncher(l Launcher) (*Launcher, error) {
	switch {
	case l.Client == nil || l.Routes == nil || l.Record == nil:
		return nil, errors.New("gatewayclient: Launcher needs a Client, Routes and Record")
	case l.BaseURL == "" || l.TokenRoot == "" || l.RefreshAt <= 0:
		return nil, errors.New("gatewayclient: Launcher needs BaseURL, TokenRoot and a positive RefreshAt")
	}
	if l.Log == nil {
		l.Log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if err := os.MkdirAll(l.TokenRoot, 0o700); err != nil {
		return nil, fmt.Errorf("gatewayclient: create %s: %w", l.TokenRoot, err)
	}
	if err := os.Chmod(l.TokenRoot, 0o700); err != nil {
		return nil, fmt.Errorf("gatewayclient: restrict %s: %w", l.TokenRoot, err)
	}
	l.st = &launcherState{cancels: map[string]context.CancelFunc{}}
	return &l, nil
}

func (l *Launcher) dir(sessionID string) (string, error) {
	if !sessionRe.MatchString(sessionID) {
		return "", fmt.Errorf("gatewayclient: session id %q cannot name a directory", sessionID)
	}
	return filepath.Join(l.TokenRoot, sessionID), nil
}

// Prepare makes the session's token directory and returns what the sandbox
// needs: the read-only mount, the launch environment, the network (decision a).
func (l *Launcher) Prepare(ctx context.Context, sessionID, projectID string) (Prep, error) {
	dir, err := l.dir(sessionID)
	if err != nil {
		return Prep{}, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Prep{}, fmt.Errorf("gatewayclient: create %s: %w", dir, err)
	}
	var route dataclass.Route // "" (unknown) unless wire's one rule reads it (task 03 decision (g))
	if pi, err := l.Routes.Project(ctx, projectID); err == nil {
		route, _ = pi.LiveRoute()
	}
	env := map[string]string{"ANTHROPIC_BASE_URL": l.BaseURL}
	if route != dataclass.RouteGlobal { // AU, or unknown: the conservative environment
		env["CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS"] = "1"
	}
	return Prep{Mount: backend.Mount{HostPath: dir, ContainerPath: ContainerTokenDir, ReadOnly: true},
		Env: env, Network: l.Network, Route: route}, nil
}
```

The session's goroutine, same file:

```go
// track gives a session a fresh context, cancelling any previous one.
func (l *Launcher) track(sessionID string) context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	l.st.mu.Lock()
	defer l.st.mu.Unlock()
	if old := l.st.cancels[sessionID]; old != nil {
		old()
	}
	l.st.cancels[sessionID] = cancel
	return ctx
}

func (l *Launcher) untrack(sessionID string) {
	l.st.mu.Lock()
	defer l.st.mu.Unlock()
	if cancel := l.st.cancels[sessionID]; cancel != nil {
		cancel()
		delete(l.st.cancels, sessionID)
	}
}

// Start mints generation for the sandbox at pod and returns at once.
func (l *Launcher) Start(sessionID, projectID, agentID string, generation int, pod netip.Addr) {
	ctx := l.track(sessionID)
	go func() {
		if !pod.IsValid() {
			l.Record(sessionID, State{Gateway: "error", Generation: generation - 1,
				Error: "The sandbox has no address on the gateway network, so it cannot be given a token."})
			return
		}
		resp, err := l.Client.Mint(ctx, wire.MintRequest{SessionID: sessionID, Generation: generation, Owner: l.Owner,
			TeamID: l.TeamID, ProjectID: projectID, AgentID: agentID, SessionClass: "solo", PodAddr: pod.String()})
		if ctx.Err() != nil {
			return // stopped meanwhile; Stop revokes whatever landed
		}
		if err == nil {
			err = l.write(sessionID, resp.Token)
		}
		if err != nil {
			l.Log.Warn("gateway mint failed", "sessionId", sessionID, "err", err)
			l.Record(sessionID, State{Gateway: "error", Generation: generation - 1, Error: sentence(err)})
			return
		}
		l.Record(sessionID, State{Gateway: "on", Generation: generation, Route: resp.Route})
		l.refreshLoop(ctx, sessionID, projectID, generation, resp.Route, l.RefreshAt)
	}()
}

// Resume takes over a session that was live before a restart: refresh first.
// The route at mint is unknown until that refresh answers.
func (l *Launcher) Resume(sessionID, projectID string, generation int) {
	ctx := l.track(sessionID)
	go l.refreshLoop(ctx, sessionID, projectID, generation, "", 0)
}

// refreshLoop refreshes every RefreshAt, and at once when a Global-minted
// session's project rises to AU: keysmith retires that key Grace after its
// next lookup (task 07 decision (h)), so waiting for RefreshAt would leave the
// session refused. RefreshRequest.Generation is the NEW one (task 03).
func (l *Launcher) refreshLoop(ctx context.Context, sessionID, projectID string, gen int, route dataclass.Route, wait time.Duration) {
	for {
		if !l.sleepUnlessRaised(ctx, projectID, route, wait) {
			return
		}
		resp, err := l.Client.Refresh(ctx, wire.RefreshRequest{SessionID: sessionID, Generation: gen + 1})
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			l.Log.Warn("gateway refresh failed", "sessionId", sessionID, "generation", gen, "err", err)
			l.Record(sessionID, State{Gateway: "on", Generation: gen, Error: "The last token refresh failed; retrying in a minute. " + sentence(err)})
			wait = refreshRetry
			continue
		}
		gen++ // keysmith minted gen+1 whether or not the write below lands
		if err := l.write(sessionID, resp.Token); err != nil {
			l.Log.Warn("gateway token write failed", "sessionId", sessionID, "err", err)
			wait = refreshRetry
			continue
		}
		route = resp.Route
		l.Record(sessionID, State{Gateway: "on", Generation: gen, Route: resp.Route})
		wait = l.RefreshAt
	}
}

// sleepUnlessRaised waits for wait, re-reading the project's route every
// RaiseCheck while the key was minted on Global. It returns false when ctx
// ends, and true when it is time to refresh: the wait is over, or the route
// is now AU. An unreadable route keeps waiting; the gatekeeper fails closed.
func (l *Launcher) sleepUnlessRaised(ctx context.Context, projectID string, route dataclass.Route, wait time.Duration) bool {
	check := l.RaiseCheck
	if check <= 0 {
		check = time.Minute
	}
	deadline := time.Now().Add(wait)
	for {
		step := time.Until(deadline)
		if route == dataclass.RouteGlobal && step > check {
			step = check
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(max(step, 0)):
		}
		if !time.Now().Before(deadline) {
			return true
		}
		if raisedToAU(ctx, l.Routes, projectID) {
			return true
		}
	}
}

// raisedToAU reads a raise by wire's one rule (task 03 decision (g)), the rule
// keysmith retires a Global-minted key on (task 07 decision (h)). Reading
// Effective alone missed a raise through the data class (re-review R-6). An
// unreadable route is no raise: keysmith changes nothing on one either.
func raisedToAU(ctx context.Context, src wire.RouteSource, projectID string) bool {
	pi, err := src.Project(ctx, projectID)
	if err != nil {
		return false
	}
	r, err := pi.LiveRoute()
	return err == nil && r == dataclass.RouteAU
}

func (l *Launcher) write(sessionID, token string) error {
	dir, err := l.dir(sessionID)
	if err != nil {
		return err
	}
	return WriteToken(dir, token)
}

// Stop ends the session's goroutine, revokes every generation, and removes the
// token (and, unless keepDir, its directory). The revoke's error is returned
// for the log; the caller never blocks a delete on it (decision g).
func (l *Launcher) Stop(ctx context.Context, sessionID string, keepDir bool) error {
	l.untrack(sessionID)
	err := l.Client.RevokeSession(ctx, sessionID)
	if dir, derr := l.dir(sessionID); derr == nil {
		_ = os.Remove(filepath.Join(dir, "token"))
		if !keepDir {
			_ = os.RemoveAll(dir)
		}
	}
	return err
}

// sentence is the one place a failure becomes words a user reads.
func sentence(err error) string {
	var p *Problem
	if !errors.As(err, &p) {
		return "The inference gateway could not be reached."
	}
	switch p.Code {
	case "route_unknown":
		return "This project's inference route is not known."
	case "budget_exhausted":
		return "A budget for this session is spent, so no token was issued."
	case "mint_timeout":
		return "The inference gateway took too long to issue a token."
	case "caller_forbidden", "unauthenticated":
		return "The inference gateway did not accept this control plane's credential."
	}
	return fmt.Sprintf("The inference gateway refused to issue a token (%d %s).", p.Status, p.Code)
}
```

`Generation: generation - 1` on a failed mint records the last generation that **did** land, so a later restart mints `GatewayGeneration + 1`, which is this same generation again, and keysmith's idempotency on `(session, generation)` (task 07) covers the case where the failed mint had in fact succeeded.

- [ ] **Step 7: Extend the backend seam**

`control-plane/internal/backend/backend.go`. Beside the other labels (`backend.go:84-88`):

```go
	LabelNetwork   = "panacea-app.network"    // Spec.Network, set only when it is non-empty
```

In `Spec` (`backend.go:181-192`), after `Mounts`:

```go
	// Network is a Docker network the sandbox joins instead of the default
	// bridge: phase 6's gateway network, so the gatekeeper sees the sandbox's
	// own address (task 15). Docker only; katabe refuses it, as it refuses Mounts.
	Network string
```

In `Sandbox` (`backend.go:303-330`), after `Endpoint`:

```go
	// NetworkAddr is the sandbox's address on Spec.Network, read back from the
	// daemon; the zero Addr when Spec.Network was empty. It is the pod address a
	// gateway token is bound to (phase 6 task 10). Docker only.
	NetworkAddr netip.Addr
```

And in `ValidateSpec`, before its final `return nil` (`backend.go:294`), with `networkPattern` beside `idPattern`:

```go
	if spec.Network != "" && (!networkPattern.MatchString(spec.Network) || reservedNetworks[spec.Network]) {
		return fmt.Errorf("spec.Network %q is not a user-defined Docker network name: %w", spec.Network, ErrUnsupported)
	}
```

```go
// networkPattern is a user-defined Docker network name. The reserved names
// would put a sandbox on the host's namespace, on none, or on the default bridge
// by another spelling; "container:<id>" fails the pattern on its colon.
var (
	networkPattern   = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,62}$`)
	reservedNetworks = map[string]bool{"host": true, "none": true, "bridge": true, "default": true}
)
```

`control-plane/internal/backend/dockerbe/docker.go`. In `Create`, after `hostCfg` is built (`docker.go:219-233`), and in `containerLabels` (`:287-295`):

```go
	if spec.Network != "" {
		hostCfg.NetworkMode = container.NetworkMode(spec.Network) // one network, not two: see NetworkAddr
	}
```

```go
	labels := map[string]string{ /* the five, unchanged */ }
	if spec.Network != "" {
		labels[backend.LabelNetwork] = spec.Network
	}
	return labels
```

The comment at `docker.go:235-238` ("networkingConfig nil and NetworkMode unset means the default bridge") gains one sentence: "With Spec.Network set, NetworkMode names that user-defined network instead; host networking stays refused (ValidateSpec)." In `Get` (`docker.go:354-388`), the returned `backend.Sandbox` gains `NetworkAddr: networkAddrOf(resp, labels)`:

```go
// networkAddrOf reads the container's address on the network its label names,
// and on no other: an address on a second network would bind a token to a
// source the gatekeeper never sees.
func networkAddrOf(resp container.InspectResponse, labels map[string]string) netip.Addr {
	name := labels[backend.LabelNetwork]
	if name == "" || resp.NetworkSettings == nil {
		return netip.Addr{}
	}
	ep := resp.NetworkSettings.Networks[name]
	if ep == nil {
		return netip.Addr{}
	}
	a, err := netip.ParseAddr(ep.IPAddress)
	if err != nil {
		return netip.Addr{}
	}
	return a.Unmap()
}
```

A missing network is the daemon's error at `ContainerCreate` (`network panacea-app-gateway not found`, from training knowledge, **UNVERIFIED** wording), which `Create` already maps to `ErrUnavailable` (`docker.go:239-242`); task 17's `docker compose up` creates the network.

`control-plane/internal/backend/katabe/kata.go`, beside the `Mounts` refusal (`kata.go:94-96`):

```go
	if spec.Network != "" {
		return backend.Sandbox{}, fmt.Errorf("katabe: spec.Network %q is a Docker network; a pod's network is its NetworkPolicy: %w", spec.Network, backend.ErrUnsupported)
	}
```

Both files add `net/netip` to their imports.

- [ ] **Step 8: Record the gateway on the session, and launch with it**

`control-plane/internal/registry/session.go`, at the end of `Session` (after `ContextRef`, `session.go:104`):

```go
	// Phase 6 task 15. Gateway is "" (the gateway was off at launch), "pending",
	// "on" or "error". GatewayGeneration is the last token generation that landed;
	// GatewayRoute the effective route at launch. None of them is a secret.
	Gateway           string `json:"gateway"`
	GatewayError      string `json:"gatewayError"`
	GatewayGeneration int    `json:"gatewayGeneration"`
	GatewayRoute      string `json:"gatewayRoute"`
```

A registry document written before this task decodes with all four empty, which reads as "off". If `registry/session_test.go` or the `registrytest` suite round-trips a fully populated record field by field, its fixture gains the four values in the same commit.

`control-plane/internal/api/api.go`, in `Deps` (`api.go:68`), after `CABundle`:

```go
	// Gateway launches each session with its inference token (phase 6 task 15).
	// Nil means the gateway is off and sessions start exactly as in phase 2.
	Gateway GatewayLauncher
```

```go
// GatewayLauncher is the part of *gatewayclient.Launcher the API calls.
type GatewayLauncher interface {
	Prepare(ctx context.Context, sessionID, projectID string) (gatewayclient.Prep, error)
	Start(sessionID, projectID, agentID string, generation int, pod netip.Addr)
	Stop(ctx context.Context, sessionID string, keepDir bool) error
}

// GatewayRecorder is the Launcher's Record: it writes the four gateway fields
// and nothing else. A session deleted meanwhile is not an error.
func GatewayRecorder(store registry.Store, log *slog.Logger) func(string, gatewayclient.State) {
	return func(id string, s gatewayclient.State) {
		_, err := store.Update(id, func(sess *registry.Session) error {
			sess.Gateway, sess.GatewayError = s.Gateway, s.Error
			sess.GatewayGeneration, sess.GatewayRoute = s.Generation, string(s.Route)
			return nil
		})
		if err != nil && !errors.Is(err, registry.ErrNotFound) {
			log.Error("record the session's gateway state", "sessionId", id, "err", err)
		}
	}
}
```

`registry.ErrNotFound` is the store's sentinel for an unknown id (`control-plane/internal/registry/registry.go:21-22`).

`control-plane/internal/api/sessions.go`, `handleSessionCreate`. After `sandboxMounts` is built (`sessions.go:884`) and before the record is written (`:902`):

```go
	// Phase 6 task 15: the gateway's half of the sandbox, before it exists.
	var gw gatewayclient.Prep
	gatewayState := ""
	if a.deps.Gateway != nil {
		if gw, err = a.deps.Gateway.Prepare(ctx, sessionID, projectID); err != nil {
			a.internal(w, r, "prepare the gateway token directory for session "+sessionID, err)
			return
		}
		for k, v := range gw.Env {
			sandboxEnv[k] = v
		}
		sandboxMounts = append(sandboxMounts, gw.Mount)
		gatewayState = "pending"
	}
```

The `registry.Session` literal (`:902-924`) gains `Gateway: gatewayState, GatewayRoute: string(gw.Route)`; the `backend.Spec` literal (`:951-962`) gains `Network: gw.Network`. On both failure paths below them (`:924-948`, `:963-970`), before `return`: `if a.deps.Gateway != nil { _ = a.deps.Gateway.Stop(context.Background(), sessionID, false) }`, which removes the directory (nothing was minted, so the revoke is a no-op). After the sandbox is recorded (`:972-981`) and before the readiness watch (`:985`):

```go
	if a.deps.Gateway != nil {
		a.deps.Gateway.Start(sessionID, projectID, string(harnessID), 1, sb.NetworkAddr) // returns at once
	}
```

`handleSessionRestart` (`sessions.go:1453`). Before its `Destroy` (`:1543`), `Stop(ctx, sess.ID, true)` (revoke, keep the directory); its `sessionEnv`/`sessionMounts` (`:1536-1539`) get the same `Prepare` block as create; its `backend.Spec` (`:1637-1648`) gains `Network`; after the new sandbox is recorded, `Start(sess.ID, sess.ProjectID, sess.Harness, sess.GatewayGeneration+1, sb.NetworkAddr)` (decision d).

`handleSessionDelete` (`sessions.go:1681`), after its `Destroy` (`:1708`), whatever that returned:

```go
	if a.deps.Gateway != nil && sess.Gateway != "" {
		if err := a.deps.Gateway.Stop(ctx, sess.ID, false); err != nil {
			a.sessionLog().Warn("revoke the session's gateway keys; they expire within the hour", "sessionId", sess.ID, "err", err)
		}
	}
```

`sessionJSON` (`sessions.go:593-620`) gains three fields, and `toJSON` (`:624-652`) fills them:

```go
	Gateway      string `json:"gateway"`      // "off" | "pending" | "on" | "error"
	GatewayError string `json:"gatewayError"` // one sentence, "" when none
	GatewayRoute string `json:"gatewayRoute"` // "global" | "au" | ""
```

```go
		Gateway:      gatewayWire(s.Gateway),
		GatewayError: s.GatewayError,
		GatewayRoute: s.GatewayRoute,
```

```go
// gatewayWire renders the registry's "" (off at launch) as "off".
func gatewayWire(s string) string {
	if s == "" {
		return "off"
	}
	return s
}
```

- [ ] **Step 9: Configure and wire it in `cmd/api`**

`control-plane/cmd/api/config.go`. `Config` (`config.go:104`) gains `Gateway GatewayConfig`; `loadConfig` (`:165`) calls `loadGatewayConfig(raw, cfg.Backend)` after the backend is parsed and returns its error unchanged:

```go
// GatewayConfig is phase 6's local launch. On is false when KEYSMITH_URL is unset.
type GatewayConfig struct {
	On              bool
	KeysmithURL     string
	BaseURL         string
	Network         string
	CallerTokenFile string
	RouteFile       string
	MintTimeout     time.Duration
	RefreshAt       time.Duration
	Owner           wire.Owner
	Team            string
}

const gw = "PANACEA_APP_GATEWAY_"

func loadGatewayConfig(raw func(string) string, backendKind string) (GatewayConfig, error) {
	c := GatewayConfig{KeysmithURL: raw(gw + "KEYSMITH_URL")}
	if c.KeysmithURL == "" {
		return c, nil // off: sessions start as in phase 2
	}
	c.On = true
	get := func(name, def string) string {
		if v := raw(gw + name); v != "" {
			return v
		}
		return def
	}
	if backendKind != "docker" {
		return c, fmt.Errorf("%sKEYSMITH_URL is set but the backend is %q: the local gateway joins a Docker network, and a cloud session's token is the broker's (ask P8-1)", gw, backendKind)
	}
	c.BaseURL, c.Network, c.Team = get("BASE_URL", "http://gatekeeper:4000"), get("NETWORK", "panacea-app-gateway"), get("TEAM", "local")
	if c.CallerTokenFile = get("CALLER_TOKEN_FILE", ""); c.CallerTokenFile == "" {
		return c, fmt.Errorf("%sCALLER_TOKEN_FILE is required with %sKEYSMITH_URL", gw, gw)
	}
	src := get("ROUTE_SOURCE", "")
	if !strings.HasPrefix(src, "file:") || len(src) == len("file:") {
		return c, fmt.Errorf("%sROUTE_SOURCE %q: the local control plane reads file:<path>; the Hub is seam S6.1", gw, src)
	}
	c.RouteFile = strings.TrimPrefix(src, "file:")
	owner := strings.SplitN(get("OWNER", "local:developer"), ":", 2)
	if len(owner) != 2 || owner[0] == "" || owner[1] == "" {
		return c, fmt.Errorf("%sOWNER must be <tid>:<oid>", gw)
	}
	c.Owner = wire.Owner{TID: owner[0], OID: owner[1]}
	var err error
	if c.MintTimeout, err = parseDuration(gw+"MINT_TIMEOUT", raw(gw+"MINT_TIMEOUT"), 20*time.Second); err != nil {
		return c, err
	}
	if c.RefreshAt, err = parseDuration(gw+"REFRESH_AT", raw(gw+"REFRESH_AT"), 45*time.Minute); err != nil {
		return c, err
	}
	if c.MintTimeout <= 0 || c.RefreshAt < time.Minute || c.RefreshAt >= 55*time.Minute {
		return c, fmt.Errorf("%sMINT_TIMEOUT must be positive and %sREFRESH_AT between 1m and 55m (the token lives 1h)", gw, gw)
	}
	return c, nil
}
```

`parseDuration(name, value, def)` is phase 2's (`config.go:421`). `config_test.go` gains a table in the file's existing style: unset `KEYSMITH_URL` gives `On == false` and no other variable is read; set, each missing or bad variable fails naming itself (`CALLER_TOKEN_FILE`, `ROUTE_SOURCE=hub`, `OWNER=nobody`, `REFRESH_AT=58m`), and the kata backend with the gateway on fails naming `KEYSMITH_URL`.

`control-plane/cmd/api/main.go`, after the registry store is opened and before `api.New` (`main.go:147`):

```go
	var launcher api.GatewayLauncher // nil interface when off: never a typed nil
	if cfg.Gateway.On {
		client, err := gatewayclient.New(cfg.Gateway.KeysmithURL, cfg.Gateway.CallerTokenFile,
			gatewayclient.Options{Timeout: cfg.Gateway.MintTimeout})
		if err != nil {
			return fail(err)
		}
		routes, err := routesource.NewFile(cfg.Gateway.RouteFile)
		if err != nil {
			return fail(err)
		}
		l, err := gatewayclient.NewLauncher(gatewayclient.Launcher{Client: client, Routes: routes,
			BaseURL: cfg.Gateway.BaseURL, Network: cfg.Gateway.Network, RefreshAt: cfg.Gateway.RefreshAt,
			TokenRoot: filepath.Join(filepath.Dir(cfg.SessionsPath), "gateway-tokens"),
			Owner:     cfg.Gateway.Owner, TeamID: cfg.Gateway.Team, Record: api.GatewayRecorder(store, log), Log: log})
		if err != nil {
			return fail(err)
		}
		launcher = l
		// Decision (c): take over every live session whose token was on.
		if live, err := store.List(); err == nil {
			for _, s := range live {
				if s.Gateway == "on" && s.Status != registry.StatusExited {
					l.Resume(s.ID, s.ProjectID, s.GatewayGeneration)
				}
			}
		}
	}
```

and `api.Deps{…}` gains `Gateway: launcher`. `fail`, `store` and `log` stand for `main.go`'s own names for its error exit, its registry and its logger; the executor matches them. `store.List()` is `registry.Store`'s (`registry.go:51`); `TokenRoot` sits beside the sessions document, inside the state directory the statepath ladder already chose.

- [ ] **Step 10: Run the tests to verify they pass**

```bash
cd control-plane
go test -race -count=1 ./internal/gatewayclient/                                   # expect: ok
go test -count=1 -v ./internal/gatewayclient/ | grep -c '^--- PASS: Test'          # expect 11
go test -race -count=1 ./internal/backend/... ./internal/registry/... ./internal/api/ ./cmd/api/   # expect: ok for each, phase 2's tests included
```

The last line is the regression gate: every phase 2 session test runs with `Deps.Gateway` nil and must be unchanged.

- [ ] **Step 11: Kill eight mutants by hand**

| Edit | Test that must fail |
|---|---|
| `Start`: the `go func() { … }()` becomes a direct call | `TestStartReturnsBeforeTheMintLands` |
| `do`: the retry re-marshals `in` with `Generation+1` (or `once` is called with a fresh body per attempt built from a counter) | `TestMintRetriesTheSameGenerationOnTimeoutAnd5xx` |
| `WriteToken`: write `final` directly, no `token.tmp` | `TestWriteTokenIsNeverSeenHalfWritten` |
| `handleSessionDelete`: the `Stop` block deleted | `TestDeleteStopsTheGateway` |
| `Prepare`: `if route != dataclass.RouteGlobal` → `if route == dataclass.RouteAU` | `TestPrepareSetsTheEnvironmentByRoute` (the unknown row) |
| `refreshLoop`: `Generation: gen + 1` → `Generation: gen` | `TestTheRefresherRefreshesThenStopsWithTheSession` |
| `sleepUnlessRaised`: delete the `if raisedToAU(…) { return true }` block | `TestARaiseRefreshesAtOnce` (both rows; review C-1) |
| `raisedToAU`: `r, err := pi.LiveRoute()` → `r, err := pi.Effective, error(nil)` | `TestARaiseRefreshesAtOnce/class` alone (re-review R-6) |

- [ ] **Step 12: Vet and format**

```bash
cd control-plane
go vet ./internal/gatewayclient/ ./internal/backend/... ./internal/api/ ./cmd/api/     # expect no output
gofmt -l internal/gatewayclient/ internal/backend/ internal/registry/ internal/api/ cmd/api/   # expect no output
```

`go vet`'s copylocks check is why `Launcher` holds its lock behind a pointer: `NewLauncher` takes the struct by value.

- [ ] **Step 13: Commit**

```bash
git add control-plane/internal/gatewayclient/client.go control-plane/internal/gatewayclient/refresh.go control-plane/internal/gatewayclient/client_test.go control-plane/internal/api/sessions_gateway_test.go
git commit -F <message file> -- control-plane/internal/gatewayclient/ control-plane/internal/backend/backend.go control-plane/internal/backend/backend_test.go control-plane/internal/backend/dockerbe/docker.go control-plane/internal/backend/dockerbe/docker_test.go control-plane/internal/backend/katabe/kata.go control-plane/internal/backend/katabe/kata_test.go control-plane/internal/registry/session.go control-plane/internal/api/api.go control-plane/internal/api/sessions.go control-plane/internal/api/sessions_test.go control-plane/internal/api/sessions_gateway_test.go control-plane/cmd/api/config.go control-plane/cmd/api/config_test.go control-plane/cmd/api/main.go
```

Message: `feat(gateway): the keysmith client and the local control plane's launch (phase 6 task 15)`, a body naming decisions (a) to (g), and the attribution line.

**Acceptance:** step 10 prints `ok`, `11`, and `ok` for every listed package; step 11's eight edits each fail the named test; step 12 prints nothing. The end-to-end run, a real sandbox on the gateway network getting a token, is task 17's acceptance and task 23's proof, not this task's.

**What this task does not claim:**

- **The cloud launch.** The broker minting, writing the token into a pod-owned Secret mounted as the directory, refreshing and revoking is ask P8-1. It can reuse `Client` and `WriteToken`'s contract, not `Launcher`, which is Docker-shaped.
- **That a raise mid-session applies the AU environment.** Decision (a): the route is served AU from the next request; `CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS` stays as launched until a restart. The key is re-minted on AU within `RaiseCheck` of the raise (`sleepUnlessRaised`, task 07 decision (h)); the environment is not.
- **That keysmith accepts a mint after a revoke** (decision (d)); task 07's part decides.
- **That the token is safe from the user's own code in the sandbox.** Any `dev` process can read `/run/panacea/gateway/token`, as it can run `panacea token`; pod binding (task 10) is what makes a copy useless elsewhere.
- **Per-person attribution in local mode.** Every local session mints as `PANACEA_APP_GATEWAY_OWNER` (default `local:developer`), because the local control plane without the Hub has no signed-in identity. With phase H's sign-in, the owner is the signed-in `tid`/`oid`; that wiring is not built here.
- **That file modes protect the token on Docker Desktop for Windows.** Decision (e).

**Gaps against the skeleton, for the orchestrator:**

1. **More files than the skeleton lists:** `registry/session.go` (four persisted fields, decision (c)), `api/api.go` (`Deps.Gateway`, `GatewayRecorder`), `cmd/api/main.go` (the launcher and the resume loop), and the backend and katabe tests. Without the registry fields a control-plane restart silently kills every live session's token within the hour.
2. **Variables beyond the skeleton's four:** `_ROUTE_SOURCE` (read here too, decision (a); task 21 writes the same file), `_MINT_TIMEOUT`, `_REFRESH_AT`, `_OWNER`, `_TEAM`.
3. **The token file mode is `0644`, not `0600`** (decision (e)).
4. **The session wire shape gains `gateway`, `gatewayError`, `gatewayRoute`.** Additive, but phase 2's UI contract and `desktop/src/store/types.ts` should list them; task 22 is the first reader.
5. **`backend.Sandbox.NetworkAddr` and `LabelNetwork` are new seam fields** the skeleton did not name; phase 8's broker has its own pod IP and does not need them.
6. **Assumed from task 03:** `wire.MintRequest` is comparable (no slice or map fields) and `PodAddr` is a string; `wire.MintResponse` carries no generation, so the client counts generations itself. Both checked against `task-03.md:385-394`. `GET /v1/usage` answers `{"rows": [...]}` (`task-08.md:23`), which `Usage` decodes.
7. **Fix batch b01 changed the refresher in two ways.** First, it refreshes at once when a Global-minted session's project rises to AU (review C-1, task 07 decision (h) and gap 7). Second, it sends `Generation: gen + 1`. The part used to send the current generation, which task 03's `RefreshRequest` ("mints the NEW generation (≥ 2)", `task-03.md:447`) and task 07's `Refresh` refuse as invalid for generation 1. So every first refresh would have failed. That defect was not in the review, and it is fixed at its root here.
