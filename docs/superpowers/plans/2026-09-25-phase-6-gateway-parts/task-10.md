### Task 10: gatekeeper authorisation

Every request a sandbox sends reaches the gatekeeper first (P6-D1), and this task decides, before a byte of the body is read, whether it may pass and on which route. The decision has seven steps, in a fixed order, and each refusal has its own problem code so a log line says exactly why. It is pure logic over two interfaces (task 03's `wire.BindingReader` and `wire.RouteSource`); task 11 turns a decision into a forwarded request, and task 12 wires the real readers.

**The one thing this task must not get wrong is the route.** OD-24 says tighten, never loosen, and P6-D4 applies that to time: a project raised to AU is served AU **from the next request**, whatever route its key was minted on, and a project lowered to Global keeps AU until each key is re-minted. So the route is never the live route alone and never the route at mint alone: it is `dataclass.Tighter(live, Binding.RouteAtMint)`. And when the live route cannot be read, the answer is `503 route_unknown`, never Global (P6-D5).

**Files:**
- `control-plane/internal/gatekeeper/authz/authz.go`: `Request`, `Decision`, `Authorizer`, `Decide` (new)
- `control-plane/internal/gatekeeper/authz/authz_test.go`: twelve tests, one mutant each (new)

**Interfaces:**
- Produces:
  - `authz.Request{Authorization, XAPIKey string; Source netip.Addr; Now time.Time}`: the two credential headers verbatim, the socket's peer address, and the time. Nothing else, by construction: task 11's `proxy.RequestFrom` (`task-11.md`) fills it from `r.RemoteAddr` and the two headers only
  - `authz.Decision{Allow bool; Status int; Code string; Binding wire.Binding; Route dataclass.Route; Token string}`. On `Allow`, `Status` is 200, `Code` is empty, and `Binding`, `Route` and `Token` are set. On a refusal, `Status` and `Code` are set, `Token` is empty, and `Binding` is set whenever the binding was found (so task 12's log line can name the session and project)
  - `authz.Authorizer{Bindings wire.BindingReader; Routes wire.RouteSource}` with `(*Authorizer).Decide(ctx, Request) Decision`, which task 11's `proxy.Decider` names (`task-11.md`, its interface list) and task 12 constructs by those two field names (`task-12.md`)
- Consumes: `wire.Binding`, `(wire.Binding).LiveAt`, `wire.BindingReader`, `wire.RouteSource`, `wire.ProjectInference`, `wire.HashToken`, `wire.NormalAddr`, `wire.ErrNotFound`, `wire.Status`, and the codes `CodeUnauthenticated`, `CodeCredentialConflict`, `CodeTokenRevoked`, `CodeTokenExpired`, `CodeBindingMismatch`, `CodeRouteUnknown` (task 03, read in `task-03.md`); `dataclass.Decide`, `Tighter`, `Route`, `RouteGlobal`, `RouteAU`, `Class`, `LevelInternal`, `ResidencyAny`, `ResidencyAUOnly` (task 01, committed); `httpx.CodeBackendUnavailable` (`control-plane/internal/httpx/problem.go:60`); in tests only, `binding.NewMemStore()` (task 05, `task-05.md`)
- Required by: task 11 (`proxy.Config.Authz`), task 12 (wiring and `/v1/models`)

**The order, and why it is this order:**

| Step | Check | Refusal |
|---|---|---|
| 1 | The credential: `Authorization: Bearer <t>` or `x-api-key: <t>`. Both present and different, or an `Authorization` header that is not a Bearer credential | `401 credential_conflict`; `401 unauthenticated` |
| 2 | Neither present | `401 unauthenticated` |
| 3 | `Bindings.ByKeyHash(wire.HashToken(t))`: a miss; an outage | `401 unauthenticated`; `503 backend_unavailable` |
| 4 | `RevokedAt` set and `Now` not before it | `401 token_revoked` |
| 5 | `Now` not before `ExpiresAt` | `401 token_expired` |
| 6 | `wire.NormalAddr(Source) != Binding.PodAddr` | `403 binding_mismatch` |
| 7 | `Routes.Project(Binding.ProjectID)`, then `Decide(class, route)` tightened by the source's `Effective`, then `Tighter(that, Binding.RouteAtMint)`; any failure or an unknown result | `503 route_unknown` |

Cheap, local checks come first, so a stolen or stale key costs one binding read (cached by task 12) and never a route read. The pod check comes before the route read so that a replayed token from outside its pod learns nothing about the project's route. Revocation comes before expiry because a key that is both is more usefully reported revoked.

**Decisions where the skeleton is silent:**

**(a) A malformed `Authorization` header is a refusal, not an absence.** `Authorization: Basic …` beside a valid `x-api-key` is `401 unauthenticated`. A header that carries something other than the credential is either a client bug or an attempt to make the two headers disagree in a way the conflict rule does not see; neither should pass. The scheme is matched case-insensitively (`bearer` passes), per HTTP's rule that auth schemes are case-insensitive (RFC 9110 §11.1, from training knowledge).

**(b) Steps 4 and 5 are `LiveAt`, split in two.** Task 03's `(Binding).LiveAt(now)` is the one definition of "live" (keysmith uses it too, `task-07.md`). This task needs two codes, so it tests the two halves separately, and a test proves that "passes steps 4 and 5" equals `LiveAt` at every boundary, so the two can never drift.

**(c) The live route is recomputed, not trusted.** Step 7 applies `dataclass.Decide` to the source's class and chosen route and then tightens by the source's own `Effective`, exactly as keysmith does at mint (`task-07.md`, `route`). A source that ever computed `Effective` wrongly can make the route tighter, never looser.

**(d) A binding-reader outage is `503 backend_unavailable`,** phase 2's code (`problem.go:60`), not `401`. A `401` tells the client its credential is bad, and Claude Code re-runs `apiKeyHelper` on a `401` (training knowledge of the settings docs, **UNVERIFIED** here), which returns the same token and fails the same way; a `503` says the server is at fault and is retryable.

- [ ] **Step 1: Preflight**

```bash
cd control-plane
test -d internal/gatekeeper/authz && echo "STOP: authz exists" || echo "ok: no authz package yet"   # expect ok
grep -n 'func NormalAddr\|func (b Binding) LiveAt\|func Status' internal/inference/wire/wire.go  # expect three (task 03)
grep -n 'func NewMemStore' internal/binding/memstore.go                                          # expect one (task 05)
```

- [ ] **Step 2: Write the failing tests**

Twelve tests. The binding reader is task 05's real memory store behind a counter, so the revocation cases run through the same `Revoke` keysmith calls; the route source is a counting map.

Create `control-plane/internal/gatekeeper/authz/authz_test.go`:

```go
package authz

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"reflect"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/binding"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/dataclass"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/wire"
)

var (
	ctx    = context.Background()
	t0     = time.Date(2026, 9, 25, 9, 0, 0, 0, time.UTC)
	pod    = netip.MustParseAddr("10.244.1.7")
	anyRes = dataclass.Class{Level: dataclass.LevelInternal, Residency: dataclass.ResidencyAny}
	auOnly = dataclass.Class{Level: dataclass.LevelInternal, Residency: dataclass.ResidencyAUOnly}
)

const tok = "sk-pa-the-session-token"

func inference(c dataclass.Class, chosen, effective dataclass.Route) wire.ProjectInference {
	return wire.ProjectInference{ProjectID: "p-1", DataClass: c, Route: chosen, Effective: effective}
}

var (
	liveGlobal = inference(anyRes, dataclass.RouteGlobal, dataclass.RouteGlobal)
	liveAU     = inference(anyRes, dataclass.RouteAU, dataclass.RouteAU)
)

// countingRoutes answers p-1 with live, counts reads, and fails when err is set.
type countingRoutes struct {
	live wire.ProjectInference
	n    int
	err  error
}

func (c *countingRoutes) Project(_ context.Context, id string) (wire.ProjectInference, error) {
	c.n++
	if c.err != nil || id != "p-1" {
		return wire.ProjectInference{}, fmt.Errorf("%w: %s", wire.ErrRouteUnknown, id)
	}
	return c.live, nil
}

// countingBindings wraps a reader, counts lookups, and fails when err is set.
type countingBindings struct {
	wire.BindingReader
	n   int
	err error
}

func (c *countingBindings) ByKeyHash(ctx context.Context, h string) (wire.Binding, error) {
	c.n++
	if c.err != nil {
		return wire.Binding{}, c.err
	}
	return c.BindingReader.ByKeyHash(ctx, h)
}

type rig struct {
	a     *Authorizer
	b     *countingBindings
	r     *countingRoutes
	store *binding.MemStore
	bound wire.Binding
}

func newRig(t *testing.T, atMint dataclass.Route, live wire.ProjectInference, mut ...func(*wire.Binding)) *rig {
	t.Helper()
	b := wire.Binding{KeyHash: wire.HashToken(tok), KeyID: wire.HashToken(tok), SessionID: "s-1", Generation: 1,
		Owner: wire.Owner{TID: "t-1", OID: "o-1"}, TeamID: "team-1", ProjectID: "p-1", AgentID: "a-1",
		PodAddr: pod, RouteAtMint: atMint, CreatedAt: t0, ExpiresAt: t0.Add(time.Hour)}
	for _, m := range mut {
		m(&b)
	}
	store := binding.NewMemStore()
	if err := store.Put(ctx, b); err != nil {
		t.Fatal(err)
	}
	cb, cr := &countingBindings{BindingReader: store}, &countingRoutes{live: live}
	return &rig{a: &Authorizer{Bindings: cb, Routes: cr}, b: cb, r: cr, store: store, bound: b}
}
```

```go
func req(auth, key string) Request {
	return Request{Authorization: auth, XAPIKey: key, Source: pod, Now: t0.Add(time.Minute)}
}

func at(now time.Time) Request {
	r := req("Bearer "+tok, "")
	r.Now = now
	return r
}

func wantDeny(t *testing.T, d Decision, code string) {
	t.Helper()
	status, ok := wire.Status(code)
	if code == httpx.CodeBackendUnavailable {
		status, ok = http.StatusServiceUnavailable, true
	}
	if !ok || d.Allow || d.Code != code || d.Status != status || d.Token != "" {
		t.Errorf("decision = %+v, want a refusal %d %s carrying no token", d, status, code)
	}
}

func wantAllow(t *testing.T, d Decision, route dataclass.Route) {
	t.Helper()
	if !d.Allow || d.Status != http.StatusOK || d.Code != "" || d.Route != route || d.Token != tok || d.Binding.SessionID != "s-1" {
		t.Errorf("decision = %+v, want allow on %s", d, route)
	}
}

// Mutant killed: reading one header only. Claude Code sends the helper's value
// as a Bearer token and, when it is the only credential, as x-api-key too.
func TestTheCredentialComesFromEitherHeader(t *testing.T) {
	r := newRig(t, dataclass.RouteGlobal, liveGlobal)
	for name, q := range map[string]Request{
		"bearer only":       req("Bearer "+tok, ""),
		"x-api-key only":    req("", tok),
		"both, equal":       req("Bearer "+tok, tok),
		"lower-case scheme": req("bearer "+tok, ""),
	} {
		t.Run(name, func(t *testing.T) { wantAllow(t, r.a.Decide(ctx, q), dataclass.RouteGlobal) })
	}
}

// Mutant killed: silently preferring one of two different credentials.
func TestTwoDifferentCredentialsAreAConflict(t *testing.T) {
	r := newRig(t, dataclass.RouteGlobal, liveGlobal)
	wantDeny(t, r.a.Decide(ctx, req("Bearer "+tok, "sk-pa-another")), wire.CodeCredentialConflict)
	if r.b.n != 0 {
		t.Errorf("%d binding lookups for a conflicting request, want 0", r.b.n)
	}
}

// Mutants killed: a malformed Authorization treated as absent, so the
// x-api-key beside it passes (decision (a)); an empty Bearer read as a token.
func TestAMissingOrMalformedCredentialIsUnauthenticated(t *testing.T) {
	r := newRig(t, dataclass.RouteGlobal, liveGlobal)
	for name, q := range map[string]Request{
		"neither":                 req("", ""),
		"basic beside a good key": req("Basic "+tok, tok),
		"empty bearer":            req("Bearer ", ""),
		"no space after bearer":   req("Bearer"+tok, ""),
	} {
		t.Run(name, func(t *testing.T) { wantDeny(t, r.a.Decide(ctx, q), wire.CodeUnauthenticated) })
	}
	if r.b.n != 0 {
		t.Errorf("%d binding lookups without a credential, want 0", r.b.n)
	}
}

// Mutants killed: a binding-reader outage answered 401 (decision (d)) or allowed.
func TestAnUnknownTokenAndAnOutageDiffer(t *testing.T) {
	r := newRig(t, dataclass.RouteGlobal, liveGlobal)
	wantDeny(t, r.a.Decide(ctx, req("Bearer sk-pa-unknown", "")), wire.CodeUnauthenticated)
	r.b.err = errors.New("keysmith unreachable")
	wantDeny(t, r.a.Decide(ctx, req("Bearer "+tok, "")), httpx.CodeBackendUnavailable)
}

// Mutants killed: After instead of !Before at either boundary; a revocation
// dated in the future (task 07's refresh grace) treated as already in force;
// a key both revoked and expired reported as expired.
func TestRevokedAndExpiredAtTheirBoundaries(t *testing.T) {
	r := newRig(t, dataclass.RouteGlobal, liveGlobal)
	cut := t0.Add(30 * time.Minute)
	if err := r.store.Revoke(ctx, r.bound.KeyHash, cut); err != nil {
		t.Fatal(err)
	}
	wantAllow(t, r.a.Decide(ctx, at(cut.Add(-time.Nanosecond))), dataclass.RouteGlobal)
	wantDeny(t, r.a.Decide(ctx, at(cut)), wire.CodeTokenRevoked)
	wantDeny(t, r.a.Decide(ctx, at(t0.Add(2*time.Hour))), wire.CodeTokenRevoked)

	e := newRig(t, dataclass.RouteGlobal, liveGlobal)
	exp := e.bound.ExpiresAt
	wantAllow(t, e.a.Decide(ctx, at(exp.Add(-time.Nanosecond))), dataclass.RouteGlobal)
	wantDeny(t, e.a.Decide(ctx, at(exp)), wire.CodeTokenExpired)
}
```

```go
// fixed is a reader holding one binding, so the test controls every field.
type fixed struct{ b wire.Binding }

func (f fixed) ByKeyHash(_ context.Context, h string) (wire.Binding, error) {
	if h != f.b.KeyHash {
		return wire.Binding{}, wire.ErrNotFound
	}
	return f.b, nil
}

// Mutant killed: a live-ness rule that drifts from wire's LiveAt, the one
// keysmith applies (decision (b)).
func TestStepsFourAndFiveAgreeWithLiveAt(t *testing.T) {
	base := newRig(t, dataclass.RouteGlobal, liveGlobal).bound
	cut := t0.Add(20 * time.Minute)
	revoked := base
	revoked.RevokedAt = &cut
	exp := base.ExpiresAt
	for _, b := range []wire.Binding{base, revoked} {
		a := &Authorizer{Bindings: fixed{b}, Routes: &countingRoutes{live: liveGlobal}}
		for _, now := range []time.Time{t0, cut.Add(-time.Nanosecond), cut, exp.Add(-time.Nanosecond), exp, exp.Add(time.Hour)} {
			if got, want := a.Decide(ctx, at(now)).Allow, b.LiveAt(now); got != want {
				t.Errorf("revoked=%v at %v: allowed %v, LiveAt %v", b.RevokedAt != nil, now, got, want)
			}
		}
	}
}

// Mutants killed: an unnormalised comparison (a dual-stack listener's
// ::ffff:10.244.1.7 refused); a zero source treated as a match; another pod
// accepted; the route read before the pod is checked.
func TestTheSourceMustBeTheBoundPod(t *testing.T) {
	r := newRig(t, dataclass.RouteGlobal, liveGlobal)
	for src, ok := range map[string]bool{"10.244.1.7": true, "::ffff:10.244.1.7": true, "10.244.1.8": false, "": false} {
		q := req("Bearer "+tok, "")
		q.Source = netip.Addr{}
		if src != "" {
			q.Source = netip.MustParseAddr(src)
		}
		before := r.r.n
		d := r.a.Decide(ctx, q)
		if ok {
			wantAllow(t, d, dataclass.RouteGlobal)
			continue
		}
		wantDeny(t, d, wire.CodeBindingMismatch)
		if d.Binding.SessionID != "s-1" || r.r.n != before {
			t.Errorf("source %q: binding %q in the decision, %d route reads; want s-1 and none", src, d.Binding.SessionID, r.r.n-before)
		}
	}
	v6 := newRig(t, dataclass.RouteGlobal, liveGlobal, func(b *wire.Binding) { b.PodAddr = netip.MustParseAddr("fd00:10:244::7") })
	q := req("Bearer "+tok, "")
	q.Source = netip.MustParseAddr("fd00:10:244::7")
	wantAllow(t, v6.a.Decide(ctx, q), dataclass.RouteGlobal)
}

// Mutant killed: a Global default for a project the source cannot answer, or
// whose record is unparsed (P6-D5).
func TestAnUnknownRouteFailsClosed(t *testing.T) {
	for name, live := range map[string]wire.ProjectInference{
		"no class":           inference(dataclass.Class{}, dataclass.RouteGlobal, dataclass.RouteGlobal),
		"no chosen route":    inference(anyRes, "", dataclass.RouteGlobal),
		"no effective route": inference(anyRes, dataclass.RouteGlobal, ""),
	} {
		t.Run(name, func(t *testing.T) {
			wantDeny(t, newRig(t, dataclass.RouteGlobal, live).a.Decide(ctx, req("Bearer "+tok, "")), wire.CodeRouteUnknown)
		})
	}
	r := newRig(t, dataclass.RouteGlobal, liveGlobal)
	r.r.err = errors.New("hub unreachable and the cache is past ROUTE_STALE_MAX")
	wantDeny(t, r.a.Decide(ctx, req("Bearer "+tok, "")), wire.CodeRouteUnknown)
}

// Mutant killed: the route at mint used alone, so a project raised to AU
// keeps sending to Global until its keys are re-minted (P6-D4).
func TestARaiseAfterMintIsServedAU(t *testing.T) {
	wantAllow(t, newRig(t, dataclass.RouteGlobal, liveAU).a.Decide(ctx, req("Bearer "+tok, "")), dataclass.RouteAU)
}

// Mutant killed: the live route used alone, so a lowering loosens a key that
// was minted on AU (OD-24: never loosen).
func TestALoweringAfterAnAUMintStaysAU(t *testing.T) {
	wantAllow(t, newRig(t, dataclass.RouteAU, liveGlobal).a.Decide(ctx, req("Bearer "+tok, "")), dataclass.RouteAU)
}

// Mutant killed: trusting the source's Effective over the class's floor
// (decision (c)): an au-only record whose route fields both say global.
func TestAnAUOnlyRecordMarkedGlobalIsServedAU(t *testing.T) {
	live := inference(auOnly, dataclass.RouteGlobal, dataclass.RouteGlobal)
	wantAllow(t, newRig(t, dataclass.RouteGlobal, live).a.Decide(ctx, req("Bearer "+tok, "")), dataclass.RouteAU)
}

// Mutant killed: a Request field a sandbox could fill, such as a forwarded
// address. The wire-level X-Forwarded-For case is task 11's RequestFrom test.
func TestRequestCarriesNothingASandboxCanForge(t *testing.T) {
	var names []string
	for i, rt := 0, reflect.TypeOf(Request{}); i < rt.NumField(); i++ {
		names = append(names, rt.Field(i).Name)
	}
	if want := []string{"Authorization", "XAPIKey", "Source", "Now"}; !reflect.DeepEqual(names, want) {
		t.Errorf("Request fields = %v, want exactly %v", names, want)
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

```bash
cd control-plane && go test ./internal/gatekeeper/authz/ -count=1 2>&1 | head -3
```

Expected: a build failure naming `undefined: Request` or `undefined: Authorizer`.

- [ ] **Step 4: Write the implementation**

Create `control-plane/internal/gatekeeper/authz/authz.go`:

```go
// Package authz decides, before a request body is read, whether one
// gatekeeper request may pass and on which route (phase 6 task 10).
package authz

import (
	"context"
	"errors"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/dataclass"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/wire"
)

// Request is everything the decision may depend on. Source is the socket's
// peer address (task 11's RequestFrom); there is no field for anything a
// sandbox could write about where it is.
type Request struct {
	Authorization string // the Authorization header, verbatim
	XAPIKey       string // the x-api-key header, verbatim
	Source        netip.Addr
	Now           time.Time
}

// Decision is the outcome. On Allow, Status is 200 and Binding, Route and
// Token are set. On a refusal, Status and Code are set, Token is empty, and
// Binding is set whenever the binding was found, for the log line.
type Decision struct {
	Allow   bool
	Status  int
	Code    string
	Binding wire.Binding
	Route   dataclass.Route
	Token   string
}

// Authorizer holds the two readers the decision needs. Task 12 fills them
// with the cached keysmith binding client and the cached route source.
type Authorizer struct {
	Bindings wire.BindingReader
	Routes   wire.RouteSource
}

// Decide runs the seven steps in order (see the table in the plan).
func (a *Authorizer) Decide(ctx context.Context, r Request) Decision {
	tok, code := credential(r.Authorization, r.XAPIKey)
	if code != "" {
		return deny(code, wire.Binding{})
	}
	b, err := a.Bindings.ByKeyHash(ctx, wire.HashToken(tok))
	switch {
	case errors.Is(err, wire.ErrNotFound):
		return deny(wire.CodeUnauthenticated, wire.Binding{})
	case err != nil:
		return Decision{Status: http.StatusServiceUnavailable, Code: httpx.CodeBackendUnavailable}
	}
	// Steps 4 and 5 are (wire.Binding).LiveAt, split for their two codes.
	if b.RevokedAt != nil && !r.Now.Before(*b.RevokedAt) {
		return deny(wire.CodeTokenRevoked, b)
	}
	if !r.Now.Before(b.ExpiresAt) {
		return deny(wire.CodeTokenExpired, b)
	}
	if !r.Source.IsValid() || wire.NormalAddr(r.Source) != wire.NormalAddr(b.PodAddr) {
		return deny(wire.CodeBindingMismatch, b)
	}
	route, ok := a.route(ctx, b)
	if !ok {
		return deny(wire.CodeRouteUnknown, b)
	}
	return Decision{Allow: true, Status: http.StatusOK, Binding: b, Route: route, Token: tok}
}

// credential is steps 1 and 2: the token, or the refusal code.
func credential(authorization, apiKey string) (string, string) {
	var bearer string
	if authorization != "" {
		scheme, rest, ok := strings.Cut(authorization, " ")
		if !ok || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(rest) == "" {
			return "", wire.CodeUnauthenticated // decision (a)
		}
		bearer = strings.TrimSpace(rest)
	}
	apiKey = strings.TrimSpace(apiKey)
	switch {
	case bearer != "" && apiKey != "" && bearer != apiKey:
		return "", wire.CodeCredentialConflict
	case bearer != "":
		return bearer, ""
	case apiKey != "":
		return apiKey, ""
	}
	return "", wire.CodeUnauthenticated
}

// route is step 7: the live route recomputed from the class (decision (c)),
// then never looser than the route at mint (P6-D4).
func (a *Authorizer) route(ctx context.Context, b wire.Binding) (dataclass.Route, bool) {
	pi, err := a.Routes.Project(ctx, b.ProjectID)
	if err != nil {
		return "", false
	}
	live, err := dataclass.Decide(pi.DataClass, pi.Route)
	if err != nil {
		return "", false
	}
	live = dataclass.Tighter(live, pi.Effective)
	r := dataclass.Tighter(live, b.RouteAtMint)
	return r, live.Valid() && r.Valid()
}

func deny(code string, b wire.Binding) Decision {
	status, ok := wire.Status(code)
	if !ok {
		status, code = http.StatusInternalServerError, httpx.CodeInternal
	}
	return Decision{Status: status, Code: code, Binding: b}
}
```

- [ ] **Step 5: Run the tests to verify they pass**

```bash
cd control-plane && go test ./internal/gatekeeper/authz/ -count=1 -race -v 2>&1 | grep -c '^--- PASS'
```

Expected: `12` (the eleven subtests print indented and are not counted), and no `--- FAIL` line.

- [ ] **Step 6: Prove the mutants die**

Apply each edit alone, run `go test ./internal/gatekeeper/authz/ -count=1`, confirm the named test fails, and revert.

| Edit in `authz.go` | Test that must fail |
|---|---|
| `r := dataclass.Tighter(live, b.RouteAtMint)` → `r := b.RouteAtMint` | `TestARaiseAfterMintIsServedAU` |
| `r := dataclass.Tighter(live, b.RouteAtMint)` → `r := live` | `TestALoweringAfterAnAUMintStaysAU` |
| in `route`, replace the `Decide`/`Tighter` pair with `live := pi.Effective` | `TestAnAUOnlyRecordMarkedGlobalIsServedAU` |
| in `route`, `return "", false` after `Routes.Project` → `return dataclass.RouteGlobal, true` | `TestAnUnknownRouteFailsClosed` |
| `wire.NormalAddr(r.Source) != wire.NormalAddr(b.PodAddr)` → `r.Source != b.PodAddr` | `TestTheSourceMustBeTheBoundPod` |
| `!r.Now.Before(*b.RevokedAt)` → `r.Now.After(*b.RevokedAt)` | `TestRevokedAndExpiredAtTheirBoundaries`, `TestStepsFourAndFiveAgreeWithLiveAt` |
| in `credential`, the first `return "", wire.CodeUnauthenticated` → `bearer = ""` | `TestAMissingOrMalformedCredentialIsUnauthenticated` |
| the `bearer != apiKey` case → deleted | `TestTwoDifferentCredentialsAreAConflict` |
| the `case err != nil:` branch → `return deny(wire.CodeUnauthenticated, wire.Binding{})` | `TestAnUnknownTokenAndAnOutageDiffer` |

- [ ] **Step 7: Vet, format, and confirm the package is small and pure**

```bash
cd control-plane
go vet ./internal/gatekeeper/authz/ && gofmt -l internal/gatekeeper/authz/   # expect no output
go list -deps ./internal/gatekeeper/authz/ | grep -c 'kpmg-au-emu'             # expect 4: authz, dataclass, httpx, wire
grep -n 'slog\|log\.' internal/gatekeeper/authz/authz.go                       # expect no output
```

The second line keeps the decision free of I/O packages: it cannot open a socket or a file, so its only inputs are the two interfaces and the `Request`. The third keeps logging in task 12, where the body-free log line is tested.

- [ ] **Step 8: Commit**

```bash
git add control-plane/internal/gatekeeper/authz/authz.go control-plane/internal/gatekeeper/authz/authz_test.go
git commit -F <message file> -- control-plane/internal/gatekeeper/authz/
```

Message: `feat(gatekeeper): authorisation, tighten at once and loosen at the next mint (phase 6 task 10)`, a body naming decisions (a) to (d), and the attribution line.

**Acceptance:** step 5 prints `12` under `-race`; step 6's nine edits each fail the named test; step 7 prints nothing, `4`, and nothing.

**What this task does not claim:**

- **That the source address survives AKS.** Pod binding assumes the gatekeeper sees the sandbox pod's own address as the TCP peer. Whether AKS's service path (kube-proxy or Cilium, SNAT, a Kata microVM's network) preserves it is the open item at `docs/superpowers/planning/2026-09-24-team-collaboration/05-design.md:212-214` and task 24's probe P03, **UNVERIFIED**. If it does not, every cloud request is `403 binding_mismatch`, and the fallback is seam S6.10 (a broker-minted client certificate).
- **Protection against a sandbox on the same pod address.** Two processes inside one pod share its address; the binding separates pods, not processes. That is the sandbox's own boundary (Kata, one pod per session).
- **A tightening latency of zero.** A raise reaches this decision when the route cache refreshes: up to task 13's `ROUTE_CACHE_TTL` (30 s). A revocation reaches it when task 12's binding cache refreshes: up to `BINDING_CACHE_TTL` (10 s).
- **Anything about the body.** Model, path and headers are task 11's.

**Gaps against the skeleton, for the orchestrator:**

1. **A binding-reader outage** has no code in the skeleton's list; this part answers phase 2's `503 backend_unavailable` (decision (d)), which task 11 already consumes from `httpx`.
2. **The skeleton's "`401` on a miss"** is `401 unauthenticated` here, the same code as a missing credential, so a caller probing for valid hashes learns nothing from the code.
3. **The X-Forwarded-For test lives in task 11** (`proxy.RequestFrom`); here the guard is structural (`TestRequestCarriesNothingASandboxCanForge`), since `Request` has no field a header could fill.
