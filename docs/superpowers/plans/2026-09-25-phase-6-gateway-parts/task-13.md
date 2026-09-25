### Task 13: Project route source: Hub, file, cache, fail closed

This task answers one question for the gatekeeper (task 10, on every request) and for keysmith (task 07, at every mint): **on which route may this project's inference run, right now?** It never answers "Global" because it could not find out (P6-D5). Three `wire.RouteSource` implementations: `Hub` reads the project record from the Hub, `File` reads local mode's `projects.json`, and `Cache` sits in front of either.

**The one thing this task must not get wrong is that the effective route is computed, never read.** A record says `inferenceRoute`, and a record can be wrong: written before a reclassification, hand-edited in local mode, or written by a Hub build that predates ask H-1. So every source computes `Effective` as `dataclass.Decide(record.DataClass, record.InferenceRoute)` (task 01), and a record that `Decide` refuses is a refusal here. A project whose class says `au-only` is served AU whatever its route field says.

Six decisions where the skeleton is silent:

- **(a) Two kinds of failure, one sentinel for callers.** A *definitive* answer (the Hub says 404, or the record has no valid class or route) and an *outage* (a 5xx, a 429, a refused or failed token, a network error, a missing file) are different: an outage may be bridged by a recent cached answer, a definitive refusal may not. So `ErrUnavailable` is declared as wrapping `wire.ErrRouteUnknown`. Every error any source returns satisfies `errors.Is(err, wire.ErrRouteUnknown)`, so tasks 07 and 10 map one sentinel to `503 route_unknown`, and only `Cache` looks at the difference.
- **(b) A definitive refusal evicts, and is never cached.** A project whose record turns invalid is refused at the next inner call even if a good entry is cached. No negative caching: the project id comes from a binding, not from the request, so a sandbox cannot make the gatekeeper hammer the Hub with made-up ids.
- **(c) Misses are single-flight.** When an entry expires under load, twenty concurrent requests for one project make one Hub call, and the call runs on a context detached from the first caller's (the Panacea shared-deadline lesson, skeleton pitfall 3), bounded by the `Hub` client's own 3-second timeout.
- **(d) `Version` is a content hash.** The Hub's `Project` object has no version field (`hub-contract.md:637-639`), so `Version` is the first 16 hex characters of the SHA-256 of the canonical fields JSON. It changes exactly when the class, the route or the change stamp changes, and it is the same for the same fields from either source.
- **(e) The Hub source refuses redirects and sends no `Origin`.** The Hub refuses any request carrying `Origin` before authentication (`hub-contract.md:38-39`), and a redirect is a way to send the bearer somewhere else. The client's `CheckRedirect` returns an error, and the request sets exactly `Authorization` and `Accept`.
- **(f) The Hub source reports the record's team** (review I-7). The `Project` object carries `teamId`, never null (`hub-contract.md:637-639`), and keysmith budgets and attributes by the team (task 07 decision (i)), so `Hub` sets `ProjectInference.TeamID` from it (task 03 decision (h)). A record whose `teamId` is missing or is not an id (`validID`) is refused definitively, like one with no class: there is no team to charge. `File` has no teams and leaves `TeamID` `""`. `Version` stays the hash of the inference fields alone, because a project's team is not an inference field and does not change the route.

**Files:**
- `control-plane/internal/routesource/cache.go` — package doc, `ErrUnavailable`, `fromFields` (the one place `Effective` is computed), `Cache`, `NewCache` (new)
- `control-plane/internal/routesource/hub.go` — `Hub`, `NewHub`, the one `GET` (new)
- `control-plane/internal/routesource/file.go` — `File`, `NewFile`, re-read on change (new)
- `control-plane/internal/routesource/token.go` — `TokenSource`, `TokenFunc`, `ClientCredentials`, the realm's client-credentials grant (new; `OD-30`)
- `control-plane/internal/routesource/{cache_test.go,hub_test.go,file_test.go,token_test.go}` — every test below, `package routesource` (new)

**Interfaces:**
- Produces:
  - `routesource.ErrUnavailable`, with `errors.Is(ErrUnavailable, wire.ErrRouteUnknown) == true`
  - `routesource.NewCache(inner wire.RouteSource, ttl, staleMax time.Duration, now func() time.Time) (*Cache, error)`: refuses `ttl <= 0` and `staleMax < ttl`; nil `now` is `time.Now`
  - `routesource.NewHub(baseURL string, tokens TokenSource, client *http.Client) (*Hub, error)`: `baseURL` absolute `http` or `https`, no path, no query; nil `client` is a 3-second client that refuses redirects
  - `routesource.NewFile(path string) (*File, error)`: refuses an empty path; the file need not exist yet
  - `routesource.TokenSource` = `interface{ Token(ctx context.Context) (string, error) }`; `routesource.TokenFunc` adapts a function
  - `routesource.ClientCredentials{Issuer, ClientID, SecretFile, Scope string; HTTP *http.Client; Now func() time.Time}`, built by `ClientCredentialsFromEnv(scope string) (*ClientCredentials, error)` from `PANACEA_APP_OIDC_ISSUER` (the realm URL, required), `PANACEA_APP_GATEWAY_OIDC_CLIENT_ID` (default `panacea-app-gateway`) and `PANACEA_APP_GATEWAY_OIDC_CLIENT_SECRET_FILE` (required, the Secrets Store CSI mount). **`OD-30`** (ruling `2026-09-25-identity-keycloak-broker.md` §6): the gateway calls the Hub as the realm's confidential client `panacea-app-gateway`, `grant_type=client_credentials`, `scope=panacea-app-hub`; this replaces the Entra `WorkloadToken` (federated assertion from `AZURE_FEDERATED_TOKEN_FILE`). Azure-plane access (Postgres, Key Vault) stays Azure workload identity, unchanged.
  - All three sources satisfy `wire.RouteSource` (compile-time assertions)
- Consumes: `dataclass.Decide`, `DecodeFields`, `FieldsFromRecord`, `ProjectInferenceFields`, `RouteAU`, `RouteGlobal` (task 01, `task-01.md:29-37`); `wire.ProjectInference`, `wire.RouteSource`, `wire.ErrRouteUnknown` (task 03); `httpx.MaxBodyBytes` (`control-plane/internal/httpx/problem.go:22`).
- Required by: task 07 (keysmith's `Deps.Routes`), task 08 and task 12 (each binary builds `Cache` over `Hub` or `File` from `PANACEA_APP_GATEWAY_ROUTE_SOURCE`), task 17 (`projects.json`), task 21 (writes the file `File` reads).

**The assumed wire shape.** Task 03 fixes `wire.ProjectInference`; this part assumes, and its tests pin, `ProjectInference{ProjectID, TeamID string; DataClass dataclass.Class; Route, Effective dataclass.Route; Version string}`. If task 03 types `Version` otherwise, `fromFields` is the one line to change.

- [ ] **Step 1: Write the failing cache tests**

The cache is tested against a scripted inner source and a hand-moved clock, so every window (TTL, the retry pause, `StaleMax`) is crossed exactly, with no sleep.

Create `control-plane/internal/routesource/cache_test.go`:

```go
package routesource

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/dataclass"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/wire"
)

var t0 = time.Date(2026, 9, 25, 3, 0, 0, 0, time.UTC)

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time      { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *clock) add(d time.Duration) { c.mu.Lock(); c.t = c.t.Add(d); c.mu.Unlock() }

// inner answers pi or err and counts calls. A non-nil hold makes each call
// wait for it, then report its own context's error if that was cancelled.
type inner struct {
	mu    sync.Mutex
	pi    wire.ProjectInference
	err   error
	calls atomic.Int32
	hold  chan struct{}
}

func (s *inner) set(pi wire.ProjectInference, err error) { s.mu.Lock(); s.pi, s.err = pi, err; s.mu.Unlock() }

func (s *inner) Project(ctx context.Context, _ string) (wire.ProjectInference, error) {
	s.calls.Add(1)
	if s.hold != nil {
		<-s.hold
		if err := ctx.Err(); err != nil {
			return wire.ProjectInference{}, err
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pi, s.err
}

var auOnly = dataclass.Class{Level: dataclass.LevelConfidential, Residency: dataclass.ResidencyAUOnly}

var auProject = wire.ProjectInference{ProjectID: "pj-1", DataClass: auOnly,
	Route: dataclass.RouteAU, Effective: dataclass.RouteAU, Version: "v1"}

func newCache(t *testing.T, in wire.RouteSource, c *clock) *Cache {
	t.Helper()
	cache, err := NewCache(in, 30*time.Second, 10*time.Minute, c.now)
	if err != nil {
		t.Fatal(err)
	}
	return cache
}

// Kills calling through on every request (the Hub would carry the gatekeeper's
// whole request rate) and kills never expiring (a raise to AU would never land).
func TestCacheIsFreshForTTLThenAsksAgain(t *testing.T) {
	c, in := &clock{t: t0}, &inner{pi: auProject}
	cache := newCache(t, in, c)
	for i := 0; i < 3; i++ {
		if _, err := cache.Project(context.Background(), "pj-1"); err != nil {
			t.Fatal(err)
		}
	}
	c.add(29 * time.Second)
	_, _ = cache.Project(context.Background(), "pj-1")
	if n := in.calls.Load(); n != 1 {
		t.Fatalf("%d inner calls inside TTL, want 1", n)
	}
	c.add(2 * time.Second)
	_, _ = cache.Project(context.Background(), "pj-1")
	if n := in.calls.Load(); n != 2 {
		t.Fatalf("%d inner calls after TTL, want 2", n)
	}
}
```

The outage, refusal and concurrency tests, in the same file:

```go
// Kills stale-forever (an outage freezing a route indefinitely), kills no stale
// at all (every Hub blip refusing every request), and kills retrying the Hub on
// every request during an outage (each would stall on the 3-second timeout).
func TestCacheBridgesAnOutageOnlyInsideStaleMax(t *testing.T) {
	c, in := &clock{t: t0}, &inner{pi: auProject}
	cache := newCache(t, in, c)
	_, _ = cache.Project(context.Background(), "pj-1")
	in.set(wire.ProjectInference{}, fmt.Errorf("%w: hub 503", ErrUnavailable))
	c.add(9 * time.Minute)
	for i := 0; i < 5; i++ {
		if got, err := cache.Project(context.Background(), "pj-1"); err != nil || got != auProject {
			t.Fatalf("inside StaleMax: %+v, %v", got, err)
		}
	}
	if n := in.calls.Load(); n != 2 {
		t.Fatalf("%d inner calls, want 2: one fill, one failed refresh, then a pause", n)
	}
	c.add(2 * time.Minute) // 11 minutes since the last good answer
	if _, err := cache.Project(context.Background(), "pj-1"); !errors.Is(err, wire.ErrRouteUnknown) {
		t.Fatalf("outside StaleMax: err = %v, want ErrRouteUnknown", err)
	}
}

// Kills masking a definitive refusal with a cached answer: a record that loses
// its class is refused at the next inner call, with a good entry in hand.
func TestCacheDefinitiveRefusalEvicts(t *testing.T) {
	c, in := &clock{t: t0}, &inner{pi: auProject}
	cache := newCache(t, in, c)
	_, _ = cache.Project(context.Background(), "pj-1")
	in.set(wire.ProjectInference{}, fmt.Errorf("%w: record has no dataClass", wire.ErrRouteUnknown))
	c.add(31 * time.Second)
	_, err := cache.Project(context.Background(), "pj-1")
	if !errors.Is(err, wire.ErrRouteUnknown) || errors.Is(err, ErrUnavailable) {
		t.Fatalf("err = %v, want a definitive ErrRouteUnknown", err)
	}
}

// Kills leaking a raw error that a caller might not map to route_unknown.
func TestCacheErrorsAlwaysSayRouteUnknown(t *testing.T) {
	cache := newCache(t, &inner{err: context.DeadlineExceeded}, &clock{t: t0})
	if _, err := cache.Project(context.Background(), "pj-1"); !errors.Is(err, wire.ErrRouteUnknown) {
		t.Fatalf("err = %v, want ErrRouteUnknown", err)
	}
}

// Kills a cache without single-flight (an expiry under load sends the whole
// burst to the Hub) and kills running the shared call on the first caller's
// context (its cancellation would fail nineteen other requests).
func TestCacheMissesAreSingleFlightOnADetachedContext(t *testing.T) {
	in := &inner{pi: auProject, hold: make(chan struct{})}
	cache := newCache(t, in, &clock{t: t0})
	leader, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	var failed atomic.Int32
	wg.Add(1)
	go func() { defer wg.Done(); _, _ = cache.Project(leader, "pj-1") }() // its own result is not checked
	for deadline := time.Now().Add(2 * time.Second); in.calls.Load() == 0; {
		if time.Now().After(deadline) {
			t.Fatal("no inner call started")
		}
		time.Sleep(time.Millisecond)
	}
	for i := 0; i < 19; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := cache.Project(context.Background(), "pj-1"); err != nil {
				failed.Add(1)
			}
		}()
	}
	// Let the followers reach the wait. On a slow machine a late follower finds
	// the fresh entry instead: the correct code still passes, only the mutant
	// might escape, never a false failure.
	time.Sleep(20 * time.Millisecond)
	cancel()
	close(in.hold)
	wg.Wait()
	if n := in.calls.Load(); n != 1 {
		t.Errorf("%d inner calls for 20 concurrent misses, want 1", n)
	}
	if n := failed.Load(); n != 0 {
		t.Errorf("%d followers failed with the leader's cancellation", n)
	}
}

func TestNewCacheRefusesBadWindows(t *testing.T) {
	for _, w := range [][2]time.Duration{{0, time.Minute}, {-time.Second, time.Minute}, {time.Minute, time.Second}} {
		if _, err := NewCache(&inner{}, w[0], w[1], nil); err == nil {
			t.Errorf("NewCache(ttl %s, staleMax %s) accepted", w[0], w[1])
		}
	}
	if _, err := NewCache(nil, time.Second, time.Minute, nil); err == nil {
		t.Error("NewCache accepted a nil inner source")
	}
}
```

- [ ] **Step 2: Write the failing Hub tests**

The fake Hub serves the contract's `Project` object (`hub-contract.md:637-639`) with task 01's four fields added, which is what ask H-1 asks the Hub to return, and one record without them, which is what the Hub returns today.

Create `control-plane/internal/routesource/hub_test.go`:

```go
package routesource

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/dataclass"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/wire"
)

const projectBase = `{"id":"pj-00000000000000b2","teamId":"tm-00000000000000c3","name":"Portal","remotes":[],` +
	`"createdAt":"2026-09-25T00:00:00.000Z","createdBy":null`

// H-1's fields for an au-only project whose stored route says global, as it may be wrongly written.
const h1AUOnlyGlobal = `,"dataClass":{"level":"confidential","residency":"au-only"},` +
	`"inferenceRoute":"global","routeChangedBy":null,"routeChangedAt":null}`

const recordAUOnlySayingGlobal = projectBase + h1AUOnlyGlobal

// After H-1, but with no team or one that is not an id (review I-7, decision (f)).
const (
	recordNoTeam = `{"id":"pj-00000000000000b2","name":"Portal","remotes":[],` +
		`"createdAt":"2026-09-25T00:00:00.000Z","createdBy":null` + h1AUOnlyGlobal
	recordBadTeam = `{"id":"pj-00000000000000b2","teamId":"tm/../c3","name":"Portal","remotes":[],` +
		`"createdAt":"2026-09-25T00:00:00.000Z","createdBy":null` + h1AUOnlyGlobal
)

// The record the Hub returns today, before ask H-1.
const recordBeforeH1 = projectBase + `}`

type hubCall struct {
	method, path, query, auth, origin, accept string
	bodyLen                                   int
}

func hubAt(t *testing.T, tokens TokenSource, h http.HandlerFunc) (*Hub, func() []hubCall) {
	t.Helper()
	var mu sync.Mutex
	var calls []hubCall
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		calls = append(calls, hubCall{r.Method, r.URL.Path, r.URL.RawQuery, r.Header.Get("Authorization"),
			r.Header.Get("Origin"), r.Header.Get("Accept"), len(b)})
		mu.Unlock()
		h(w, r)
	}))
	t.Cleanup(srv.Close)
	if tokens == nil {
		tokens = TokenFunc(func(context.Context) (string, error) { return "tok-1", nil })
	}
	hub, err := NewHub(srv.URL, tokens, nil)
	if err != nil {
		t.Fatal(err)
	}
	return hub, func() []hubCall { mu.Lock(); defer mu.Unlock(); return append([]hubCall(nil), calls...) }
}

func answer(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}
}

func definitive(err error) bool {
	return errors.Is(err, wire.ErrRouteUnknown) && !errors.Is(err, ErrUnavailable)
}

// Kills trusting the record's route: the class forces AU. Kills dropping the
// record's team, which keysmith checks the caller's against (review I-7).
func TestHubEffectiveRouteIsDecidedNotRead(t *testing.T) {
	hub, _ := hubAt(t, nil, answer(200, recordAUOnlySayingGlobal))
	got, err := hub.Project(context.Background(), "pj-00000000000000b2")
	if err != nil {
		t.Fatal(err)
	}
	if got.TeamID != "tm-00000000000000c3" {
		t.Errorf("TeamID = %q, want the record's tm-00000000000000c3", got.TeamID)
	}
	if got.ProjectID != "pj-00000000000000b2" || got.DataClass != auOnly ||
		got.Route != dataclass.RouteGlobal || got.Effective != dataclass.RouteAU || len(got.Version) != 16 {
		t.Errorf("got %+v, want the stored global route and an effective au", got)
	}
}
```

The refusals, the outages and the request itself:

```go
// Kills reading today's record (no class, no route) as Global, and kills a 404
// counted as an outage, which the cache would bridge with a stale entry.
func TestHubRefusalsAreDefinitive(t *testing.T) {
	for name, h := range map[string]http.HandlerFunc{
		"record before H-1": answer(200, recordBeforeH1),
		"no team":           answer(200, recordNoTeam),
		"a team not an id":  answer(200, recordBadTeam),
		"unknown project":   answer(404, `{"error":"project_not_found","message":"No such project."}`),
		"not a record":      answer(200, `[]`),
	} {
		hub, _ := hubAt(t, nil, h)
		if _, err := hub.Project(context.Background(), "pj-00000000000000b2"); !definitive(err) {
			t.Errorf("%s: err = %v, want a definitive ErrRouteUnknown", name, err)
		}
	}
}

// Kills a 5xx, a 429, a refused token or a redirect being read as "no such
// project", and kills following a redirect, which would carry the bearer away.
func TestHubOutagesAreUnavailable(t *testing.T) {
	elsewhere := httptest.NewServer(answer(200, recordAUOnlySayingGlobal))
	defer elsewhere.Close()
	for name, h := range map[string]http.HandlerFunc{
		"500": answer(500, `{}`), "502": answer(502, ``), "503": answer(503, `{}`),
		"429": answer(429, `{}`), "401": answer(401, `{}`), "403": answer(403, `{}`),
		"redirect": func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, elsewhere.URL, http.StatusFound) },
	} {
		hub, _ := hubAt(t, nil, h)
		if _, err := hub.Project(context.Background(), "pj-00000000000000b2"); !errors.Is(err, ErrUnavailable) {
			t.Errorf("%s: err = %v, want ErrUnavailable", name, err)
		}
	}
	failing := TokenFunc(func(context.Context) (string, error) { return "", errors.New("the realm said no") })
	hub, calls := hubAt(t, failing, answer(200, recordAUOnlySayingGlobal))
	if _, err := hub.Project(context.Background(), "pj-00000000000000b2"); !errors.Is(err, ErrUnavailable) || len(calls()) != 0 {
		t.Errorf("token failure: err = %v with %d Hub calls, want ErrUnavailable and none", err, len(calls()))
	}
}

// Kills asking for more than the one project, kills an Origin header (the Hub
// refuses it before authentication), and kills putting a sandbox-reachable
// string into the Hub's path unchecked.
func TestHubAsksForTheProjectAndNothingElse(t *testing.T) {
	hub, calls := hubAt(t, nil, answer(200, recordAUOnlySayingGlobal))
	if _, err := hub.Project(context.Background(), "pj-00000000000000b2"); err != nil {
		t.Fatal(err)
	}
	want := hubCall{http.MethodGet, "/v1/projects/pj-00000000000000b2", "", "Bearer tok-1", "", "application/json", 0}
	if got := calls(); len(got) != 1 || got[0] != want {
		t.Fatalf("Hub saw %+v, want exactly %+v", got, want)
	}
	for _, id := range []string{"", "pj-a/../teams", "pj-a?x=1", "..", "pj-a%2Fb", "pj a"} {
		if _, err := hub.Project(context.Background(), id); !definitive(err) {
			t.Errorf("id %q: err = %v, want a definitive refusal", id, err)
		}
	}
	if n := len(calls()); n != 1 {
		t.Errorf("malformed ids reached the Hub: %d calls, want 1", n)
	}
}

func TestNewHubRefusesABadBase(t *testing.T) {
	tok := TokenFunc(func(context.Context) (string, error) { return "t", nil })
	for _, base := range []string{"", "hub:8750", "ftp://hub", "http://hub/v1", "http://hub?x=1", "https://"} {
		if _, err := NewHub(base, tok, nil); err == nil {
			t.Errorf("NewHub(%q) accepted", base)
		}
	}
	if _, err := NewHub("https://hub.example", nil, nil); err == nil {
		t.Error("NewHub accepted a nil TokenSource")
	}
}
```

- [ ] **Step 3: Write the failing file tests**

The file is local mode's stand-in for the Hub (task 17 ships one; task 21 rewrites it). The rewrite test moves the modification time explicitly, because Docker Desktop's bind mounts and some filesystems keep mtimes at one-second resolution and a test that relied on the clock ticking would pass or fail by luck.

Create `control-plane/internal/routesource/file_test.go`:

```go
package routesource

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/dataclass"
)

const twoProjects = `{
  "pj-au":  {"dataClass": {"level": "confidential", "residency": "au-only"}, "inferenceRoute": "global"},
  "pj-any": {"dataClass": {"level": "internal", "residency": "any"}, "inferenceRoute": "global"},
  "pj-bad": {"dataClass": {"level": "secret", "residency": "any"}, "inferenceRoute": "global"}
}`

func writeProjects(t *testing.T, path, body string, mtime time.Time) {
	t.Helper()
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(tmp, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, path); err != nil { // task 21's writer renames too
		t.Fatal(err)
	}
}

func newFileAt(t *testing.T, body string) (*File, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "projects.json")
	if body != "" {
		writeProjects(t, path, body, t0)
	}
	f, err := NewFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return f, path
}

// Kills trusting the stored route, kills a Global default for a project the
// file does not name, and kills one bad entry failing (or passing) its neighbours.
func TestFileDecidesEachProjectOnItsOwn(t *testing.T) {
	f, _ := newFileAt(t, twoProjects)
	ctx := context.Background()
	if got, err := f.Project(ctx, "pj-au"); err != nil || got.Effective != dataclass.RouteAU {
		t.Errorf("pj-au: %+v, %v; want effective au", got, err)
	}
	if got, err := f.Project(ctx, "pj-any"); err != nil || got.Effective != dataclass.RouteGlobal {
		t.Errorf("pj-any: %+v, %v; want effective global", got, err)
	}
	for _, id := range []string{"pj-bad", "pj-absent"} {
		if _, err := f.Project(ctx, id); !definitive(err) {
			t.Errorf("%s: err = %v, want a definitive refusal", id, err)
		}
	}
}

// Kills reading the file once: a raise written by task 21 must be seen.
func TestFileRewriteIsSeen(t *testing.T) {
	f, path := newFileAt(t, twoProjects)
	ctx := context.Background()
	if got, _ := f.Project(ctx, "pj-any"); got.Effective != dataclass.RouteGlobal {
		t.Fatalf("before: %+v", got)
	}
	raised := `{"pj-any": {"dataClass": {"level": "internal", "residency": "any"}, "inferenceRoute": "au"}}`
	writeProjects(t, path, raised, t0.Add(2*time.Second))
	if got, err := f.Project(ctx, "pj-any"); err != nil || got.Effective != dataclass.RouteAU {
		t.Errorf("after the rewrite: %+v, %v; want effective au", got, err)
	}
}

// Kills reading a missing or broken file as "no projects", which would be a
// definitive refusal the cache could not bridge while task 21 is mid-write.
func TestFileMissingOrUnreadableIsUnavailable(t *testing.T) {
	f, path := newFileAt(t, "")
	if _, err := f.Project(context.Background(), "pj-any"); !errors.Is(err, ErrUnavailable) {
		t.Errorf("missing file: err = %v, want ErrUnavailable", err)
	}
	writeProjects(t, path, `{"pj-any": `, t0)
	if _, err := f.Project(context.Background(), "pj-any"); !errors.Is(err, ErrUnavailable) {
		t.Errorf("truncated file: err = %v, want ErrUnavailable", err)
	}
	if _, err := NewFile(""); err == nil {
		t.Error("NewFile accepted an empty path")
	}
}
```

- [ ] **Step 4: Write the failing token tests**

The exchange is the realm's client-credentials grant (`OD-30`, ruling §6): `POST <issuer>/protocol/openid-connect/token` with `grant_type=client_credentials`, `scope=panacea-app-hub`, and the client authenticated as `client_secret_post` (`client_id`, `client_secret` in the form). The secret is a file the Secrets Store CSI driver mounts from Key Vault, read afresh on each exchange because the driver rotates it. That Keycloak's `client-secret` authenticator accepts the secret in the form (as well as in HTTP Basic) is from training knowledge, **UNVERIFIED** here; hub task 43's dev-realm check settles it.

Create `control-plane/internal/routesource/token_test.go`:

```go
package routesource

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func realmToken(t *testing.T, status int, body string) (*ClientCredentials, *clock, func() []url.Values, string) {
	t.Helper()
	var mu sync.Mutex
	var forms []url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/realms/panacea-app/protocol/openid-connect/token" {
			t.Errorf("the realm saw %s %s", r.Method, r.URL.Path)
		}
		_ = r.ParseForm()
		mu.Lock()
		forms = append(forms, r.PostForm)
		mu.Unlock()
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	file := filepath.Join(t.TempDir(), "client-secret")
	if err := os.WriteFile(file, []byte("s3cret-1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c := &clock{t: t0}
	wt := &ClientCredentials{Issuer: srv.URL + "/realms/panacea-app", ClientID: "panacea-app-gateway",
		SecretFile: file, Scope: "panacea-app-hub", Now: c.now}
	return wt, c, func() []url.Values { mu.Lock(); defer mu.Unlock(); return append([]url.Values(nil), forms...) }, file
}

const granted = `{"token_type":"Bearer","expires_in":3600,"access_token":"at-1"}`

// Kills a wrong grant, Entra's path, a wrong scope, and a secret read from
// somewhere other than the mounted file.
func TestClientCredentialsIsTheRealmForm(t *testing.T) {
	wt, _, forms, _ := realmToken(t, 200, granted)
	tok, err := wt.Token(context.Background())
	if err != nil || tok != "at-1" {
		t.Fatalf("Token = %q, %v", tok, err)
	}
	want := url.Values{"grant_type": {"client_credentials"}, "client_id": {"panacea-app-gateway"},
		"client_secret": {"s3cret-1"}, "scope": {"panacea-app-hub"}}
	if got := forms(); len(got) != 1 || got[0].Encode() != want.Encode() {
		t.Errorf("form %v, want %v", got, want)
	}
}

// Kills an exchange per request and kills caching past expiry; and, because
// the CSI driver rotates the secret, kills reading the file only once.
func TestClientCredentialsIsCachedAndTheFileReread(t *testing.T) {
	wt, c, forms, file := realmToken(t, 200, granted)
	_, _ = wt.Token(context.Background())
	c.add(54 * time.Minute)
	_, _ = wt.Token(context.Background())
	if n := len(forms()); n != 1 {
		t.Fatalf("%d exchanges inside the token's life, want 1", n)
	}
	if err := os.WriteFile(file, []byte("s3cret-2"), 0o600); err != nil {
		t.Fatal(err)
	}
	c.add(2 * time.Minute) // 56 minutes: inside the 5-minute margin
	_, _ = wt.Token(context.Background())
	if got := forms(); len(got) != 2 || got[1].Get("client_secret") != "s3cret-2" {
		t.Errorf("second exchange %v, want one carrying the rotated file", got)
	}
}

// Kills echoing the realm's body (it can quote the request) or the secret.
func TestClientCredentialsErrorsCarryNoSecret(t *testing.T) {
	wt, _, _, _ := realmToken(t, 401, `{"error":"unauthorized_client","error_description":"bad s3cret-1"}`)
	_, err := wt.Token(context.Background())
	if err == nil || strings.Contains(err.Error(), "s3cret") || strings.Contains(err.Error(), "error_description") {
		t.Fatalf("err = %v", err)
	}
}

// Kills a missing issuer or secret file defaulted; kills the client id's default dropped.
func TestClientCredentialsFromEnvNamesWhatIsMissing(t *testing.T) {
	t.Setenv("PANACEA_APP_OIDC_ISSUER", "https://auth.example/realms/panacea-app/")
	t.Setenv("PANACEA_APP_GATEWAY_OIDC_CLIENT_ID", "")
	t.Setenv("PANACEA_APP_GATEWAY_OIDC_CLIENT_SECRET_FILE", "")
	if _, err := ClientCredentialsFromEnv("panacea-app-hub"); err == nil ||
		!strings.Contains(err.Error(), "PANACEA_APP_GATEWAY_OIDC_CLIENT_SECRET_FILE") {
		t.Fatalf("err = %v, want one naming PANACEA_APP_GATEWAY_OIDC_CLIENT_SECRET_FILE", err)
	}
	t.Setenv("PANACEA_APP_GATEWAY_OIDC_CLIENT_SECRET_FILE", "/mnt/secrets/client-secret")
	cc, err := ClientCredentialsFromEnv("panacea-app-hub")
	if err != nil || cc.ClientID != "panacea-app-gateway" || cc.Issuer != "https://auth.example/realms/panacea-app" {
		t.Fatalf("ClientCredentialsFromEnv = %+v, %v", cc, err)
	}
	t.Setenv("PANACEA_APP_OIDC_ISSUER", "")
	if _, err := ClientCredentialsFromEnv("panacea-app-hub"); err == nil || !strings.Contains(err.Error(), "PANACEA_APP_OIDC_ISSUER") {
		t.Fatalf("err = %v, want one naming PANACEA_APP_OIDC_ISSUER", err)
	}
	if _, err := ClientCredentialsFromEnv(""); err == nil {
		t.Fatal("an empty scope was accepted")
	}
}
```

- [ ] **Step 5: Run the tests to verify they fail**

Run: `cd control-plane && go test ./internal/routesource/`
Expected: a build failure naming this task's symbols, for example `undefined: NewCache`, `undefined: ErrUnavailable`, `undefined: NewHub`, `undefined: TokenFunc`. If it names `dataclass.FieldsFromRecord` or `wire.ProjectInference`, task 01 or task 03 is not merged: stop.

- [ ] **Step 6: Write the package core and the cache**

Create `control-plane/internal/routesource/cache.go`:

```go
// Package routesource answers, for one project, the inference route it may run
// on now. Every source computes the effective route through dataclass.Decide,
// and every error any source returns satisfies errors.Is(err, wire.ErrRouteUnknown).
package routesource

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/dataclass"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/wire"
)

// ErrUnavailable is an outage: the source could not answer. It wraps
// wire.ErrRouteUnknown, so a caller that knows only that sentinel fails closed.
var ErrUnavailable = fmt.Errorf("%w: route source unavailable", wire.ErrRouteUnknown)

// retryPause suspends inner calls for one project after an outage, while a
// stale entry is served, so an outage does not stall every request on it.
const retryPause = 5 * time.Second

// validID admits Hub ids (pj-<hex>) and local ids, and nothing that could
// change the meaning of a URL path. The first character is never a dot.
var validID = regexp.MustCompile(`^[A-Za-z0-9_-][A-Za-z0-9._-]{0,127}$`)

var (
	_ wire.RouteSource = (*Cache)(nil)
	_ wire.RouteSource = (*Hub)(nil)
	_ wire.RouteSource = (*File)(nil)
)

func checkID(id string) error {
	if !validID.MatchString(id) {
		return fmt.Errorf("%w: malformed project id", wire.ErrRouteUnknown)
	}
	return nil
}

func isDefinitive(err error) bool {
	return errors.Is(err, wire.ErrRouteUnknown) && !errors.Is(err, ErrUnavailable)
}

// fromFields is the one place an effective route is computed.
func fromFields(id string, f dataclass.ProjectInferenceFields) (wire.ProjectInference, error) {
	eff, err := dataclass.Decide(f.DataClass, f.InferenceRoute)
	if err != nil {
		return wire.ProjectInference{}, fmt.Errorf("%w: project %s: %v", wire.ErrRouteUnknown, id, err)
	}
	canon, err := json.Marshal(f)
	if err != nil {
		return wire.ProjectInference{}, fmt.Errorf("%w: project %s: %v", wire.ErrRouteUnknown, id, err)
	}
	sum := sha256.Sum256(canon)
	return wire.ProjectInference{ProjectID: id, DataClass: f.DataClass, Route: f.InferenceRoute,
		Effective: eff, Version: hex.EncodeToString(sum[:8])}, nil
}
```

The cache, same file. The whole state machine is four lines of `fill`'s `switch`: a success stores, a definitive refusal evicts, an outage inside `StaleMax` keeps the entry and pauses retries, an outage outside it evicts.

```go
type entry struct {
	pi         wire.ProjectInference
	fetched    time.Time // the last successful answer; StaleMax counts from here
	retryAfter time.Time // set by an outage; inner calls wait until then
}

type flight struct {
	done chan struct{}
	pi   wire.ProjectInference
	err  error
}

// Cache fronts another RouteSource: fresh for ttl, stale across an outage for
// at most staleMax since the last good answer, single-flight on a miss.
type Cache struct {
	inner         wire.RouteSource
	ttl, staleMax time.Duration
	now           func() time.Time

	mu      sync.Mutex
	entries map[string]*entry
	flights map[string]*flight
}

// NewCache validates the windows. staleMax below ttl would make stale serving
// unreachable, which is a configuration mistake rather than a policy.
func NewCache(inner wire.RouteSource, ttl, staleMax time.Duration, now func() time.Time) (*Cache, error) {
	switch {
	case inner == nil:
		return nil, errors.New("routesource: Cache needs an inner source")
	case ttl <= 0 || staleMax < ttl:
		return nil, fmt.Errorf("routesource: need 0 < ttl (%s) <= staleMax (%s)", ttl, staleMax)
	}
	if now == nil {
		now = time.Now
	}
	return &Cache{inner: inner, ttl: ttl, staleMax: staleMax, now: now,
		entries: map[string]*entry{}, flights: map[string]*flight{}}, nil
}

// Project serves a fresh or bridgeable entry, or waits for the one inner call.
func (c *Cache) Project(ctx context.Context, id string) (wire.ProjectInference, error) {
	c.mu.Lock()
	now := c.now()
	if e := c.entries[id]; e != nil {
		age := now.Sub(e.fetched)
		if age < c.ttl || (now.Before(e.retryAfter) && age < c.staleMax) {
			pi := e.pi
			c.mu.Unlock()
			return pi, nil
		}
	}
	f := c.flights[id]
	if f == nil {
		f = &flight{done: make(chan struct{})}
		c.flights[id] = f
		go c.fill(id, f)
	}
	c.mu.Unlock()
	select {
	case <-f.done:
		return f.pi, f.err
	case <-ctx.Done():
		return wire.ProjectInference{}, fmt.Errorf("%w: %v", ErrUnavailable, ctx.Err())
	}
}

// fill runs the inner call on a context of its own (decision c).
func (c *Cache) fill(id string, f *flight) {
	pi, err := c.inner.Project(context.Background(), id)
	c.mu.Lock()
	defer func() { delete(c.flights, id); c.mu.Unlock(); close(f.done) }()
	now := c.now()
	e := c.entries[id]
	switch {
	case err == nil:
		c.entries[id], f.pi = &entry{pi: pi, fetched: now}, pi
	case isDefinitive(err):
		delete(c.entries, id)
		f.err = err
	case e != nil && now.Sub(e.fetched) < c.staleMax:
		e.retryAfter, f.pi = now.Add(retryPause), e.pi
	default:
		delete(c.entries, id)
		f.err = fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
}
```

- [ ] **Step 7: Write the Hub source**

The Hub's fields are private and set by `NewHub`, rather than the skeleton's exported `Hub{BaseURL; Tokens; HTTP}`: a struct literal would skip the base-URL check and the redirect refusal, which are the two things this file exists to guarantee. A caller-supplied client is copied and its `CheckRedirect` replaced, so no configuration can turn redirects back on.

Create `control-plane/internal/routesource/hub.go`:

```go
package routesource

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/dataclass"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/wire"
)

// Hub reads task 01's four fields from the Hub's project record (contract
// route 15, GET /v1/projects/{projectId}), which ask H-1 adds them to.
type Hub struct {
	base   *url.URL
	tokens TokenSource
	http   *http.Client
}

func noRedirect(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

// NewHub validates the base URL and pins the client's redirect policy.
func NewHub(baseURL string, tokens TokenSource, client *http.Client) (*Hub, error) {
	u, err := url.Parse(baseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" ||
		(u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.ForceQuery {
		return nil, errors.New("routesource: the Hub URL must be http(s)://host[:port] with no path or query")
	}
	if tokens == nil {
		return nil, errors.New("routesource: the Hub source needs a TokenSource")
	}
	c := http.Client{Timeout: 3 * time.Second}
	if client != nil {
		c = *client
	}
	c.CheckRedirect = noRedirect
	return &Hub{base: &url.URL{Scheme: u.Scheme, Host: u.Host}, tokens: tokens, http: &c}, nil
}

// Project makes one GET. A 404 or a record without a valid class and route is
// definitive; everything else that is not a 200 is an outage.
func (h *Hub) Project(ctx context.Context, id string) (wire.ProjectInference, error) {
	if err := checkID(id); err != nil {
		return wire.ProjectInference{}, err
	}
	tok, err := h.tokens.Token(ctx)
	if err != nil {
		return wire.ProjectInference{}, fmt.Errorf("%w: hub token: %v", ErrUnavailable, err)
	}
	u := *h.base
	u.Path = "/v1/projects/" + id
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return wire.ProjectInference{}, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Accept", "application/json")
	resp, err := h.http.Do(req)
	if err != nil {
		return wire.ProjectInference{}, fmt.Errorf("%w: hub unreachable: %v", ErrUnavailable, err)
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, httpx.MaxBodyBytes+1))
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return wire.ProjectInference{}, fmt.Errorf("%w: the Hub has no project %s", wire.ErrRouteUnknown, id)
	case resp.StatusCode != http.StatusOK:
		return wire.ProjectInference{}, fmt.Errorf("%w: the Hub answered %d", ErrUnavailable, resp.StatusCode)
	case readErr != nil || len(body) > httpx.MaxBodyBytes:
		return wire.ProjectInference{}, fmt.Errorf("%w: the Hub's reply could not be read", ErrUnavailable)
	}
	f, err := dataclass.FieldsFromRecord(body)
	if err != nil {
		return wire.ProjectInference{}, fmt.Errorf("%w: project %s: %v", wire.ErrRouteUnknown, id, err)
	}
	team, err := teamOf(body)
	if err != nil {
		return wire.ProjectInference{}, fmt.Errorf("%w: project %s: %v", wire.ErrRouteUnknown, id, err)
	}
	pi, err := fromFields(id, f)
	if err != nil {
		return wire.ProjectInference{}, err
	}
	pi.TeamID = team
	return pi, nil
}

// teamOf is the record's teamId (hub-contract.md:637-639, never null), the
// team keysmith charges and attributes by (decision (f), review I-7). A record
// without a valid one is refused, definitively: there is no team to charge.
func teamOf(body []byte) (string, error) {
	var rec struct {
		TeamID string `json:"teamId"`
	}
	if err := json.Unmarshal(body, &rec); err != nil {
		return "", err
	}
	if !validID.MatchString(rec.TeamID) {
		return "", errors.New("the record has no valid teamId")
	}
	return rec.TeamID, nil
}
```

The error text never includes the Hub's body: a Problem body is harmless, but a misrouted request could return anything, and these errors reach task 12's log line.

- [ ] **Step 8: Write the file source**

The file is `{"<projectId>": <ProjectInferenceFields>, …}` (the skeleton's shape for task 17's `projects.json`). Each entry is decoded strictly with `dataclass.DecodeFields` (task 01), so a typo in a hand-edited local file refuses that one project instead of reading as a default. The whole file is re-read when its modification time **or** size changes, and a parse failure is an outage, not "no projects", so the cache bridges the instant between task 21's rename and the next read.

Create `control-plane/internal/routesource/file.go`:

```go
package routesource

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/dataclass"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/wire"
)

// File reads local mode's projects.json.
type File struct {
	path string

	mu      sync.Mutex
	mtime   time.Time
	size    int64
	entries map[string]json.RawMessage
}

// NewFile refuses an empty path. The file may not exist yet: that is an
// outage until it does, never an empty project list.
func NewFile(path string) (*File, error) {
	if path == "" {
		return nil, errors.New("routesource: the projects file path is empty")
	}
	return &File{path: path}, nil
}

func (f *File) Project(_ context.Context, id string) (wire.ProjectInference, error) {
	if err := checkID(id); err != nil {
		return wire.ProjectInference{}, err
	}
	entries, err := f.load()
	if err != nil {
		return wire.ProjectInference{}, err
	}
	raw, ok := entries[id]
	if !ok {
		return wire.ProjectInference{}, fmt.Errorf("%w: the projects file names no project %s", wire.ErrRouteUnknown, id)
	}
	fields, err := dataclass.DecodeFields(raw)
	if err != nil {
		return wire.ProjectInference{}, fmt.Errorf("%w: project %s: %v", wire.ErrRouteUnknown, id, err)
	}
	return fromFields(id, fields)
}

func (f *File) load() (map[string]json.RawMessage, error) {
	st, err := os.Stat(f.path)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.entries != nil && st.ModTime().Equal(f.mtime) && st.Size() == f.size {
		return f.entries, nil
	}
	b, err := os.ReadFile(f.path)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil || m == nil {
		return nil, fmt.Errorf("%w: the projects file is not one JSON object", ErrUnavailable)
	}
	f.entries, f.mtime, f.size = m, st.ModTime(), st.Size()
	return m, nil
}
```

A write that keeps both the size and the modification time is not seen. Task 21 always writes through a rename, which changes the modification time on every filesystem this runs on; a hand edit on a one-second-resolution mount that keeps the size exactly is the residue, and it is local mode only.

- [ ] **Step 9: Write the token source**

Standard library only, as the Hub does for its own database token. The exchange holds the mutex, so concurrent callers share one exchange; the only caller in practice is `Cache.fill`, one per project at a time.

Create `control-plane/internal/routesource/token.go`:

```go
package routesource

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
	"strings"
	"sync"
	"time"
)

// TokenSource yields a bearer for the Hub.
type TokenSource interface {
	Token(ctx context.Context) (string, error)
}

// TokenFunc adapts a function (tests; a local bearer).
type TokenFunc func(ctx context.Context) (string, error)

func (f TokenFunc) Token(ctx context.Context) (string, error) { return f(ctx) }

const tokenMargin = 5 * time.Minute // refresh this long before expiry

// ClientCredentials is the gateway's own realm token to the Hub: the
// client-credentials grant as the confidential client panacea-app-gateway
// (OD-30, ruling §6). The Hub accepts it only on the route-source and
// usage roll-up routes.
type ClientCredentials struct {
	Issuer, ClientID, SecretFile, Scope string
	HTTP                                *http.Client
	Now                                 func() time.Time

	mu      sync.Mutex
	token   string
	expires time.Time
}

// ClientCredentialsFromEnv reads the realm URL, the client id and the
// CSI-mounted secret file. scope is the Hub's audience scope, panacea-app-hub.
func ClientCredentialsFromEnv(scope string) (*ClientCredentials, error) {
	if scope == "" {
		return nil, errors.New("routesource: the Hub scope is empty")
	}
	env := func(k string) string { return strings.TrimSpace(os.Getenv(k)) }
	c := &ClientCredentials{Scope: scope, Issuer: strings.TrimSuffix(env("PANACEA_APP_OIDC_ISSUER"), "/"),
		ClientID: env("PANACEA_APP_GATEWAY_OIDC_CLIENT_ID"), SecretFile: env("PANACEA_APP_GATEWAY_OIDC_CLIENT_SECRET_FILE")}
	if c.ClientID == "" {
		c.ClientID = "panacea-app-gateway"
	}
	if c.Issuer == "" {
		return nil, errors.New("routesource: PANACEA_APP_OIDC_ISSUER is not set (the realm URL)")
	}
	if c.SecretFile == "" {
		return nil, fmt.Errorf("routesource: %s is not set (the client secret the CSI driver mounts)",
			"PANACEA_APP_GATEWAY_OIDC_CLIENT_SECRET_FILE")
	}
	return c, nil
}
```

The exchange, same file:

```go
// Token returns the cached access token, or runs the grant with the secret
// file, read afresh each time because the CSI driver rotates it.
func (w *ClientCredentials) Token(ctx context.Context) (string, error) {
	now := time.Now
	if w.Now != nil {
		now = w.Now
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	t := now()
	if w.token != "" && t.Before(w.expires.Add(-tokenMargin)) {
		return w.token, nil
	}
	secret, err := os.ReadFile(w.SecretFile)
	if err != nil || len(bytes.TrimSpace(secret)) == 0 {
		return "", errors.New("routesource: the client secret file is unreadable or empty")
	}
	form := url.Values{"grant_type": {"client_credentials"}, "client_id": {w.ClientID},
		"client_secret": {string(bytes.TrimSpace(secret))}, "scope": {w.Scope}}
	endpoint := strings.TrimSuffix(w.Issuer, "/") + "/protocol/openid-connect/token"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", errors.New("routesource: the token endpoint URL is invalid")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := w.HTTP
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second, CheckRedirect: noRedirect}
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", errors.New("routesource: the token endpoint could not be reached")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
		return "", fmt.Errorf("routesource: the token endpoint answered %d", resp.StatusCode)
	}
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil ||
		out.AccessToken == "" || out.ExpiresIn <= 0 {
		return "", errors.New("routesource: the token endpoint's reply held no usable token")
	}
	w.token, w.expires = out.AccessToken, t.Add(time.Duration(out.ExpiresIn)*time.Second)
	return w.token, nil
}
```

No error in this file wraps a lower error: `*url.Error` from `Do` quotes the request URL, which is harmless, but a JSON syntax error quotes bytes of the reply, and the reply of a misbehaving endpoint is not ours to log.

- [ ] **Step 10: Run the tests to verify they pass**

```bash
cd control-plane
go test -race -count=1 ./internal/routesource/                                 # expect: ok
go test -count=1 -v ./internal/routesource/ | grep -c '^--- PASS: Test'        # expect 18
```

- [ ] **Step 11: Kill six mutants by hand**

| Edit | Test that must fail |
|---|---|
| `fromFields`: `Effective: eff` → `Effective: f.InferenceRoute` | `TestHubEffectiveRouteIsDecidedNotRead`, `TestFileDecidesEachProjectOnItsOwn` |
| `fill`: the `case isDefinitive(err):` arm deleted (a refusal falls to the stale arm) | `TestCacheDefinitiveRefusalEvicts` |
| `fill`: `case e != nil && now.Sub(e.fetched) < c.staleMax:` → `case e != nil:` | `TestCacheBridgesAnOutageOnlyInsideStaleMax` |
| `fill`: `context.Background()` → the caller's `ctx`, passed in from `Project` | `TestCacheMissesAreSingleFlightOnADetachedContext` |
| `Hub.Project`: delete the `teamOf` call and `pi.TeamID = team` (review I-7) | `TestHubEffectiveRouteIsDecidedNotRead` (`TeamID` empty), `TestHubRefusalsAreDefinitive` (`no team`) |
| `teamOf`: `!validID.MatchString(rec.TeamID)` → `rec.TeamID == ""` | `TestHubRefusalsAreDefinitive` (`a team not an id`) |

- [ ] **Step 12: Vet, format, and confirm the import set**

```bash
cd control-plane
go vet ./internal/routesource/ && gofmt -l internal/routesource/     # expect no output
go list -deps ./internal/routesource/ | grep kpmg-au-emu | sort
```

Expected, exactly: `…/internal/dataclass`, `…/internal/httpx`, `…/internal/inference/wire`, `…/internal/routesource`. No Azure SDK, no Kubernetes client.

- [ ] **Step 13: Commit**

```bash
git add control-plane/internal/routesource/cache.go control-plane/internal/routesource/hub.go control-plane/internal/routesource/file.go control-plane/internal/routesource/token.go control-plane/internal/routesource/cache_test.go control-plane/internal/routesource/hub_test.go control-plane/internal/routesource/file_test.go control-plane/internal/routesource/token_test.go
git commit -F <message file> -- control-plane/internal/routesource/
```

Message: `feat(routesource): the live project route from the Hub or a file, cached and failing closed (phase 6 task 13)`, a body naming decisions (a) to (f), and the attribution line.

**Acceptance:** step 10 prints `ok` and `18`; step 11's six edits each fail the named tests; step 12 prints nothing and then the four packages.

**What this task does not claim:**

- **Push, or a tightening latency below the TTL.** A raise to AU reaches the gatekeeper and keysmith within `ROUTE_CACHE_TTL` (30 s) of the Hub accepting it, per process, with no signal between. P6-D4's "tighten immediately" therefore means *from the first request after that window*. A change-feed subscription (`hub-contract.md` §7.11) could close the window and is not built.
- **Tightening during a partial outage.** If the Hub accepts a raise from the desktop while the gatekeeper's path to it is failing, the gatekeeper serves its last answer for up to `ROUTE_STALE_MAX` (10 minutes) from that answer. The window is the price of not refusing every request on a Hub blip; the owner may prefer `ROUTE_STALE_MAX` equal to the TTL, which makes any outage a refusal after 30 s.
- **That the Hub accepts the gateway's service token.** The grant is written; the Hub taking `azp == panacea-app-gateway` as a service client (`oidcauth` `ServiceClients`, no `oid`/`tid`) on route 15 only is hub task 13's (ruling §6) and ask H-3. The realm client `panacea-app-gateway` (confidential, service account on, audience scope `panacea-app-hub` optional on it) is one of the Panacea asks (ruling §10) until phase 8's own realm; its existence and the `client_secret_post` form are **UNVERIFIED** here.
- **That the Hub returns task 01's fields.** Until ask H-1 lands, every Hub project is a definitive refusal, which is the fail-closed reading of "the Hub does not know this project's class". Cloud inference is therefore blocked on H-1, by design.
- **That a Hub record matches the id asked for.** The record's own `id` is not compared with the request; a Hub that answered with another project's record would be believed.
- **A bound on the cache's size.** One entry per project ever asked for, which is the org's project count.

**Gaps against the skeleton, for the orchestrator:**

1. **New variables.** The Hub source needs the scope to ask the realm for, `panacea-app-hub` (`OD-30`; was Entra's `api://<hub client id>/.default`). This part assumes task 12 and task 08 read it as `PANACEA_APP_GATEWAY_HUB_SCOPE`, default `panacea-app-hub`, which is not in the skeleton's shared names; likewise `PANACEA_APP_GATEWAY_OIDC_CLIENT_ID` and `_OIDC_CLIENT_SECRET_FILE`, which task 19's chart sets (the file from the Secrets Store CSI driver, never an env file). `PANACEA_APP_OIDC_ISSUER` is the shared realm URL (ruling §2).
2. `wire.ProjectInference.Version` is a `string` in task 03's part (`task-03.md:517-523`), which this part fills with the content hash of decision (d).
3. The skeleton's `Hub{BaseURL; Tokens; HTTP}` literal is replaced by `NewHub` with private fields (step 7 says why). `File` and `Cache` likewise have constructors.
4. `ROUTE_SOURCE=file:<path>` and `hub` are parsed by tasks 08 and 12, not here; both binaries build `Cache` over the source with `ROUTE_CACHE_TTL` and `ROUTE_STALE_MAX`, so keysmith and the gatekeeper can disagree for at most one TTL. keysmith's disagreement is harmless: task 10 takes the tighter of the live route and the route at mint.
5. **`ProjectInference.TeamID` is filled here** (decision (f), review I-7; task 03 decision (h)). keysmith refuses a mint whose caller-asserted team differs (task 07 decision (i)). Only `Hub` has teams; `File`'s `""` lets the caller's team stand, which is right for local mode alone. Task 19's chart sets `PANACEA_APP_GATEWAY_ROUTE_SOURCE=hub` for both keysmith and the gatekeeper (`task-19.md:484`, `:549`), so the cloud never runs on `File`.
