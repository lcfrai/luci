### Tasks 08a and 08b: `cmd/keysmith`: caller authentication (08a); API, config, fail-closed start (08b)

keysmith is the only process besides LiteLLM that holds the master key, so it is the one that must refuse to run half-configured and refuse to serve an unknown caller. This task puts task 07's `Service` behind an HTTP API on `:4100`, decides who may call each route, parses every `PANACEA_APP_GATEWAY_*` variable keysmith reads, and wires the pieces (tasks 04, 05, 06, 09, 13) into one binary.

**The one thing this task must not get wrong is starting when it should not.** Panacea's LiteLLM took its master key from a `secretKeyRef` marked `optional: true` (`~/Projects/Panacea/deploy/portal/deploy-portal.yaml:779-784`, verified 2026-09-25: `name: LITELLM_MASTER_KEY` … `optional: true`), so a missing Secret gave an empty key and a pod that still reported ready. Here the master key is a file, and a missing, empty or malformed file stops start-up with the variable's name on stderr, before any listener opens. The same holds for the database URL file, the caller table, the route source and every budget figure.

**Two tasks, one package (review I-10).** The work was 3 hours in one piece; it is two, run by two agents in turn, both in `package main` of `cmd/keysmith`:

| Task | Hours | Depends on | Writes | Tests | Steps |
|---|---|---|---|---|---|
| **08a** caller authentication | 1.5 h | 03; env 17 step 0 (`oidcauth`) | `callerauth.go`, `calleroidc.go`, `callerauth_test.go`, `calleroidc_test.go`, the placeholder `main.go` | 6 | every block marked **[08a]** |
| **08b** API, config, fail-closed start | 2 h | 02, 07, 08a, 09, 13 | `api.go`, `config.go`, `owner.go`, `main.go` (replaces 08a's), `api_test.go`, `config_test.go`, `main_test.go`, `owner_test.go` | 11 (17 in the package) | every block marked **[08b]** |

08a needs nothing of task 07: its code reads only `wire` (task 03) and `oidcauth`, so it runs in wave 3 beside task 05 (once env 17 step 0 is merged) and leaves the critical path. A block marked neither is read by both. **The placeholder `main.go` (08a) is there only so the tree links:** a `package main` with no `func main` passes `go test` and `go vet` but fails `go build` with `function main is undeclared in the main package` (measured 2026-09-26, the Go toolchain on this machine, `go test` exit 0, `go vet` exit 0, `go build ./...` exit 1). So 08a ships a `main` that prints one line and exits `2`, the configuration-refusal status; a binary built between 08a and 08b refuses to start, as keysmith must when it is not fully configured. 08b replaces the file whole.

**Files:**
- `control-plane/cmd/keysmith/callerauth.go` **[08a]**: `Route`, `Caller`, `Authenticator`, the caller table, `bearerAuth`, `kubeAuth` (new)
- `control-plane/cmd/keysmith/calleroidc.go` **[08a]**: `oidcAuth`, `chainAuth`, `issuerOf` (new; decision (g), `OD-30`)
- `control-plane/cmd/keysmith/{callerauth_test.go,calleroidc_test.go}` **[08a]**: six tests, and the `tokenAuth` fake 08b's tests share (new)
- `control-plane/cmd/keysmith/main.go` **[08a]** the placeholder (new); **[08b]** `main`, `run`, `serve`, the wiring (replaced)
- `control-plane/cmd/keysmith/config.go` **[08b]**: `Config`, `loadConfig`, the parsers (new)
- `control-plane/cmd/keysmith/api.go` **[08b]**: `apiDeps`, `newAPI`, the seven routes, `problemFor` (new)
- `control-plane/cmd/keysmith/owner.go` **[08b]**: `loadOIDC`, `commaList`, `callerOf`, `ownerMayAct` (new; decision (g); they read `envPrefix` and `apiDeps`, so they cannot land in 08a)
- `control-plane/cmd/keysmith/{main_test.go,config_test.go,api_test.go,owner_test.go}` **[08b]**: eleven tests (new)

**Interfaces:**
- Produces, on the wire (every body JSON, every error the problem shape of `control-plane/internal/httpx/problem.go:93`):

| Route | Caller route name | Request | Success | Errors |
|---|---|---|---|---|
| `POST /v1/mint` | `mint` | `wire.MintRequest` | `201` `wire.MintResponse` | `400 bad_request`, `401 unauthenticated`, `403 caller_forbidden`, `409 conflict`, `503 route_unknown`, `budget_exhausted`, `503 budget_unknown`, `503 backend_unavailable`, `504 mint_timeout` |
| `POST /v1/refresh` | `refresh` | `wire.RefreshRequest` | `201` `wire.MintResponse` | as mint, plus `404 session_not_found` |
| `POST /v1/revoke` | `revoke` | `wire.RevokeRequest` | `204` | `400`, `401`, `403`, `503 backend_unavailable` |
| `GET /v1/bindings/{keyHash}` | `bindings` | 64 lowercase hex | `200` `wire.Binding` | `400 bad_request`, `401`, `403`, `404 binding_not_found`, `503 backend_unavailable` |
| `GET /v1/usage?scope=&id=` | `usage` | `scope` one of `person`, `project`, `team` | `200` `{"rows": [wire.UsageRow]}`, never `null` | `400 bad_request`, `401`, `403`, `503 backend_unavailable` |
| `GET /healthz` | none | | `200 ok` | |
| `GET /readyz` | none | | `200 ok` | `503 backend_unavailable` (database or LiteLLM) |

  Any other method or path is `404 path_not_allowed`. The status of each `wire` code is `wire.Status(code)` (task 03); the table's `budget_exhausted` status is task 03's.
- Produces, for operators: `PANACEA_APP_GATEWAY_CALLER_AUTH` = a comma list of at most one table mode (`bearer-file:<path>`, `kube`, or `kube:<path>`) and at most one `oidc` (the cloud sets `kube,oidc`; decision (g)); with `oidc`, `PANACEA_APP_OIDC_ISSUER` (required), `PANACEA_APP_OIDC_PROFILE` (`keycloak`, the only value accepted), `PANACEA_APP_GATEWAY_TENANTS` (required, enrolled `tid` GUIDs), `PANACEA_APP_GATEWAY_AUDIENCE` (default `panacea-app-gateway`), `PANACEA_APP_GATEWAY_ALLOWED_CLIENTS` (default `panacea-app-desktop`); the caller table's line format `name credential routes` (below); exit status `2` on any configuration refusal.
- Consumes:
  - task 07: `keysmith.New`, `Deps`, `(*Service).Mint`, `Refresh`, `RevokeSession`, and its errors (`task-07.md`, committed)
  - task 03: the `wire` request, response and binding types, `wire.Code…`, `wire.Status`, `wire.CheckBaseURL`, `wire.ScopePerson`, `ScopeProject`, `ScopeTeam`, `wire.HashToken`, `wire.ErrNotFound`, `wire.ErrRouteUnknown` (`task-03.md` interface list)
  - task 04: `litellmadmin.New(baseURL, masterKeyFile string, o Options) (*Client, error)`, `litellmadmin.ErrNoMasterKey`, `(*Client).Health(ctx) error`
  - task 05: `binding.Migrate(ctx, *pgxpool.Pool) error`, `binding.NewPGStore(*pgxpool.Pool) *PGStore`
  - task 06: `budget.NewStatic`, `budget.Layered`, `budget.Proposed`, `budget.Limits`, `budget.Monthly` (`task-06.md`, committed)
  - task 09: `usage.NewPGStore`, `usage.NewReader`, `usage.Sweeper` (`task-09.md`, this writer)
  - task 13: `routesource.NewFile`, `NewHub`, `NewCache`, `ClientCredentialsFromEnv` (`task-13.md` interface list; `OD-30` replaced `WorkloadTokenFromEnv`)
  - env task 17 step 0 (ruling `2026-09-25-identity-keycloak-broker.md` §5, NOT BUILT; the rename of the built `control-plane/internal/entra`): `oidcauth.New(Config) (*JWKSValidator, error)`, `oidcauth.Validator`, `Config{Profile, Issuer, TenantIDs, Audience, RequiredScope, AllowedClients, ServiceClients}`, `Claims{TenantID, ObjectID, ClientID, Service}`, `(Claims).Key()`, `ProfileKeycloak`, `ErrKeySetUnavailable`, `ErrSignature`
  - task 02: `catalogue.Load`, `(*Catalogue).Groups` (`task-02.md`)
  - `k8s.io/client-go v0.31.1`, `k8s.io/api`, `k8s.io/apimachinery` (already in `control-plane/go.mod:8-10`); `github.com/jackc/pgx/v5` (task 05 adds it)
- Required by: task 08b (08a's authenticators); tasks 15a and 12 code against the route table above, not a merged binary (15a's client, 12's binding reader; both test against a fake keysmith); task 17 (compose and `secrets/callers`, 08b), task 19 (the chart, 08b), task 21 (usage, through 15a's client), ask P8-1 (the broker)

**Decisions where the skeleton is silent:**

**(a) A configuration refusal exits `2`, not `1`.** The skeleton said "exits 1" until fix batch b04 round 2; phase 2's control plane already fixes `2` as "a configuration refusal the operator must fix" and `1` as a runtime failure (`control-plane/cmd/api/main.go:45-47`, verified 2026-09-25). keysmith follows the house convention; the test asserts `2` and the variable's name.

**(b) One caller-table format for both modes.** Each non-blank, non-`#` line is `name credential routes`: a lowercase name, a credential, and a comma-separated subset of `mint,refresh,revoke,bindings,usage`. Under `bearer-file:<path>` the credential is a bearer token of at least 32 characters; under `kube:<path>` it is a Kubernetes username (`system:serviceaccount:<ns>:<sa>`). `kube` alone uses the built-in table below. This is exactly the file task 17's `init.sh` writes (`task-17.md`, "The bearer file, complete"): `controlplane <64 hex> mint,refresh,revoke,usage` and `gatekeeper <64 hex> bindings`. A parse error names the file and the line number, never the credential.

```text
# The built-in table for CALLER_AUTH=kube. Fleet has no line: it is off until seam S6.5.
broker     system:serviceaccount:panacea-broker:broker       mint,refresh,revoke
gatekeeper system:serviceaccount:panacea-gateway:gatekeeper  bindings
```

The namespace `panacea-broker` is phase 8's (`.claude/worktrees/env/docs/superpowers/plans/2026-09-23-phase-8-own-environment.md:48`, verified 2026-09-25); the service-account names `broker` and `gatekeeper` are this part's, and ask P8-2 names the real ones. No cloud caller has `usage` yet (gap 3).

**(c) The API answers `504` two seconds before `MINT_TIMEOUT`.** The mint runs on a context detached from the request (`context.WithoutCancel`), so a client that gives up does not abort it half-way (task 04's rule, restated here because this is where the request ends). The handler waits `MINT_TIMEOUT − 2 s` (at least 1 s) and then answers `504 mint_timeout`. A client on the same `MINT_TIMEOUT` (task 15) therefore receives the 504 instead of its own timeout, and retries the **same generation**, which task 07 makes safe.

**(d) A binding miss is `404 binding_not_found`.** The gatekeeper must tell a miss (negative-cache it, answer `401`) from an outage (`503`). Task 03's list has no code for it; gap 1.

**(e) `kube` checks the audience and caches a review for 60 s.** A `TokenReview` names audience `keysmith` and the reply's audiences must contain it, so a projected token minted for the Hub or the API server is refused. A positive result is cached by `wire.HashToken(token)` for 60 s (1,024 entries at most); a failed review is never cached. keysmith's service account needs `create` on `tokenreviews` (the `system:auth-delegator` ClusterRole, from training knowledge, **UNVERIFIED**), which task 19's chart must grant (gap 4).

**(f) Error text reaches the log, not the body, for every `5xx`.** A `5xx` body carries a fixed sentence per code; the error itself is logged as `err` beside `route`, `caller` and `status`. Task 07 already scrubs tokens from its errors; this is the second wall.

**(g) `oidc`: a person's own realm token (`OD-30`; ruling `2026-09-25-identity-keycloak-broker.md` §6).** `CALLER_AUTH` is a comma list: at most one table mode and at most one `oidc`; the cloud sets `kube,oidc`. `oidc` validates the bearer with `oidcauth` under the `keycloak` profile (any other `PANACEA_APP_OIDC_PROFILE` is refused): issuer `PANACEA_APP_OIDC_ISSUER`, audience and required scope `panacea-app-gateway` (the audience scope carries the audience's name, ruling §4), `azp` in `panacea-app-desktop`, `tid` in `PANACEA_APP_GATEWAY_TENANTS`, and **no** service client (`ServiceClients` empty, and a `Service` claim is refused again here). Such a caller is named `person:<tid>/<oid>`, holds `mint`, `refresh` and `revoke` only, and carries its `Owner`, the token's `(tid, oid)`. For it, a mint whose body names another `Owner` is `403 caller_forbidden`, and so is any mint, refresh or revoke of a session one of whose bindings (task 05's `BySession`) has another owner. A table caller carries no `Owner` and is checked exactly as before: the `Owner` the broker sends is the one it derived from the person's `panacea-app-broker` token (P8-D4). **The token's unverified `iss` chooses who decides:** equal to `PANACEA_APP_OIDC_ISSUER`, `oidc`; anything else, the table mode. The peek only chooses; the chosen authenticator verifies everything, and a refusal never falls through to the other. A realm key-set outage is `503`, as a TokenReview outage is. This is the local-mode laptop minting against the cloud gateway (task 15's `oidc` credential). LiteLLM's `user_id` stays `"<tid>:<oid>"` (P6-D6).

- [ ] **Step 1: Preflight**

**[08a]**

```bash
cd control-plane
test -d cmd/keysmith && echo "STOP: cmd/keysmith exists" || echo "ok: no keysmith binary yet"   # expect ok
test -d internal/inference/wire && echo "ok inference/wire" || echo "MISSING inference/wire"  # task 03
grep -n 'k8s.io/client-go v0.31.1' go.mod                                                     # expect one
grep -n 'ProfileKeycloak\|ServiceClients\|Service  *bool' internal/oidcauth/*.go               # expect all three (env 17 step 0, OD-30)
```

**[08b]**

```bash
cd control-plane
grep -c 'the API is not wired (task 08b)' cmd/keysmith/main.go                                # expect 1: 08a's placeholder
for p in keysmith litellmadmin binding budget usage routesource inference/wire inference/catalogue; do
  test -d internal/$p && echo "ok $p" || echo "MISSING $p"; done
grep -n 'jackc/pgx/v5' go.mod                                                                 # expect one
grep -n 'func NewPGStore\|func Migrate' internal/binding/*.go                                  # expect both (task 05)
grep -n 'ErrNoMasterKey\|func (c \*Client) Health' internal/litellmadmin/client.go             # expect both (task 04)
grep -n 'func (s \*PGStore) BySession' internal/binding/*.go                                   # expect one (task 05)
go test ./cmd/keysmith/ -count=1 > /tmp/ks-08a.txt 2>&1; echo "08a exit $?"                 # expect 0: 08a's six pass
```

A `MISSING` line, a `0` from a `grep -c`, or a non-zero `08a exit` is an unmerged dependency; stop and report. If a grep shows a different identifier from gap 5's list, rename in this part's code to match the merged one.

- [ ] **Step 2: Write the failing tests**

Seventeen tests over six files, all `package main`: **[08a]** writes six in `callerauth_test.go` and `calleroidc_test.go`; **[08b]** writes eleven in `api_test.go`, `config_test.go`, `main_test.go` and `owner_test.go`, and reuses 08a's `tokenAuth` fake and `realm` constant. None needs a database, LiteLLM, a realm or a cluster: the service, the binding reader, the usage reader, the realm validator and the TokenReview API are fakes (the last is client-go's own `fake.Clientset`, which phase 2 already uses in `internal/backend/katabe/kata_test.go`).

**[08a]** Create `control-plane/cmd/keysmith/callerauth_test.go`:

```go
package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	authv1 "k8s.io/api/authentication/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

var (
	tokA = strings.Repeat("a1", 32) // 64 characters, the shape init.sh writes
	tokB = strings.Repeat("b2", 32)
)

func writeFile(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func withBearer(tok string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/v1/mint", nil)
	if tok != "" {
		r.Header.Set("Authorization", "Bearer "+tok)
	}
	return r
}

// Mutant killed: a parse error that quotes its line, which puts a bearer token
// on stderr and in whatever collects stderr.
func TestCallerTableRefusesBadLinesWithoutQuotingThem(t *testing.T) {
	good := "# task 17's two lines\n\ncontrolplane " + tokA + " mint,refresh,revoke,usage\ngatekeeper " + tokB + " bindings\n"
	if _, err := parseCallers([]byte(good), true); err != nil {
		t.Fatalf("good table refused: %v", err)
	}
	bad := map[string]string{
		"unknown route":        "cp " + tokA + " mint,destroy",
		"duplicate name":       "cp " + tokA + " mint\ncp " + tokB + " bindings",
		"duplicate credential": "cp " + tokA + " mint\ngk " + tokA + " bindings",
		"short token":          "cp short-secret-canary mint",
		"missing routes":       "cp " + tokA,
		"upper-case name":      "CP " + tokA + " mint",
		"empty table":          "# nothing\n",
	}
	for name, table := range bad {
		_, err := parseCallers([]byte(table), true)
		if err == nil {
			t.Errorf("%s: accepted", name)
			continue
		}
		if strings.Contains(err.Error(), tokA) || strings.Contains(err.Error(), "short-secret-canary") {
			t.Errorf("%s: the error quotes a credential", name)
		}
	}
}
```

```go
// Mutants killed: a token matched by prefix or case-folded; a request with no
// header given some default caller.
func TestBearerAuthIdentifiesExactTokensOnly(t *testing.T) {
	a, err := newBearerAuth(writeFile(t, "controlplane "+tokA+" mint,usage\n"))
	if err != nil {
		t.Fatal(err)
	}
	c, err := a.Identify(withBearer(tokA))
	if err != nil || c.Name != "controlplane" || c.RouteList() != "mint,usage" {
		t.Fatalf("Identify(tokA) = %+v, %v", c, err)
	}
	for _, tok := range []string{"", tokA[:63], strings.ToUpper(tokA), tokA + "a", tokB} {
		if _, err := a.Identify(withBearer(tok)); !errors.Is(err, ErrUnauthenticated) {
			t.Errorf("a token of length %d: err = %v, want ErrUnauthenticated", len(tok), err)
		}
	}
}

// fakeReviews answers TokenReviews: "broker-token" is the broker's for the
// keysmith audience, "hub-token" the broker's for another audience,
// "stranger" a service account in no table, and "down" an API-server failure.
func fakeReviews(t *testing.T, calls *int) *fake.Clientset {
	cs := fake.NewSimpleClientset()
	cs.PrependReactor("create", "tokenreviews", func(a k8stesting.Action) (bool, runtime.Object, error) {
		*calls++
		tr := a.(k8stesting.CreateAction).GetObject().(*authv1.TokenReview)
		if !reflect.DeepEqual(tr.Spec.Audiences, []string{"keysmith"}) {
			t.Errorf("review asked for audiences %v, want [keysmith]", tr.Spec.Audiences)
		}
		out := tr.DeepCopy()
		broker := authv1.UserInfo{Username: "system:serviceaccount:panacea-broker:broker"}
		switch tr.Spec.Token {
		case "broker-token":
			out.Status = authv1.TokenReviewStatus{Authenticated: true, Audiences: []string{"keysmith"}, User: broker}
		case "hub-token":
			out.Status = authv1.TokenReviewStatus{Authenticated: true, Audiences: []string{"panacea-hub"}, User: broker}
		case "stranger":
			out.Status = authv1.TokenReviewStatus{Authenticated: true, Audiences: []string{"keysmith"},
				User: authv1.UserInfo{Username: "system:serviceaccount:default:default"}}
		case "down":
			return true, nil, errors.New("apiserver unavailable")
		}
		return true, out, nil
	})
	return cs
}

// Mutants killed: no audience check (a token minted for the Hub accepted); an
// authenticated stranger given routes; a failed review cached, or read as
// "unauthenticated" instead of "unavailable"; no cache, or a cache forever.
func TestKubeAuthChecksAudienceTableAndCache(t *testing.T) {
	calls := 0
	now := time.Date(2026, 9, 25, 9, 0, 0, 0, time.UTC)
	entries, err := parseCallers([]byte(builtinKubeTable), false)
	if err != nil {
		t.Fatal(err)
	}
	a := newKubeAuth(fakeReviews(t, &calls).AuthenticationV1().TokenReviews(), entries, func() time.Time { return now })
	c, err := a.Identify(withBearer("broker-token"))
	if err != nil || c.Name != "broker" || c.RouteList() != "mint,refresh,revoke" {
		t.Fatalf("broker = %+v, %v", c, err)
	}
	if _, err := a.Identify(withBearer("broker-token")); err != nil || calls != 1 {
		t.Errorf("second call: err %v, %d reviews, want 1 (cached)", err, calls)
	}
	now = now.Add(61 * time.Second)
	if _, err := a.Identify(withBearer("broker-token")); err != nil || calls != 2 {
		t.Errorf("after 61 s: err %v, %d reviews, want 2", err, calls)
	}
	if _, err := a.Identify(withBearer("hub-token")); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("wrong audience: err = %v, want ErrUnauthenticated", err)
	}
	if c, err := a.Identify(withBearer("stranger")); err != nil || c.RouteList() != "" {
		t.Errorf("stranger = %+v, %v: want authenticated with no routes", c, err)
	}
	before := calls
	for i := 0; i < 2; i++ {
		if _, err := a.Identify(withBearer("down")); !errors.Is(err, ErrReviewUnavailable) {
			t.Errorf("review failure: err = %v, want ErrReviewUnavailable", err)
		}
	}
	if calls != before+2 {
		t.Errorf("a failed review was cached: %d reviews, want %d", calls, before+2)
	}
}

// Mutant killed: Fleet enabled before seam S6.5, or the gatekeeper granted mint.
func TestBuiltinKubeTableIsBrokerAndGatekeeperOnly(t *testing.T) {
	entries, err := parseCallers([]byte(builtinKubeTable), false)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, e := range entries {
		got[e.caller.Name] = e.caller.RouteList()
	}
	want := map[string]string{"broker": "mint,refresh,revoke", "gatekeeper": "bindings"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("built-in table = %v, want %v", got, want)
	}
}
```

**[08b]** Create `control-plane/cmd/keysmith/api_test.go` (the `tokenAuth` fake it uses is 08a's, in `calleroidc_test.go`):

```go
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/budget"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/dataclass"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/wire"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/keysmith"
)

type fakeKeys struct {
	mu      sync.Mutex
	calls   []string
	err     error
	release chan struct{} // when set, Mint waits for it
	ctxErr  error         // Mint's ctx.Err() once released
	done    chan struct{} // closed when a released Mint returns
}

func (f *fakeKeys) record(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, name)
}

func (f *fakeKeys) Mint(ctx context.Context, _ wire.MintRequest) (wire.MintResponse, error) {
	f.record("mint")
	if f.release != nil {
		<-f.release
		f.mu.Lock()
		f.ctxErr = ctx.Err()
		f.mu.Unlock()
		defer close(f.done)
	}
	if f.err != nil {
		return wire.MintResponse{}, f.err
	}
	return wire.MintResponse{Token: "sk-pa-test", KeyID: "k-1", Route: dataclass.RouteGlobal,
		BaseURL: "http://gatekeeper:4000", MaxBudgetUSD: 25}, nil
}

func (f *fakeKeys) Refresh(context.Context, wire.RefreshRequest) (wire.MintResponse, error) {
	f.record("refresh")
	return wire.MintResponse{Token: "sk-pa-test-2"}, f.err
}

func (f *fakeKeys) RevokeSession(context.Context, wire.RevokeRequest) (int, error) {
	f.record("revoke")
	return 1, f.err
}

type fakeBindings map[string]wire.Binding

func (f fakeBindings) ByKeyHash(_ context.Context, h string) (wire.Binding, error) {
	if b, ok := f[h]; ok {
		return b, nil
	}
	return wire.Binding{}, wire.ErrNotFound
}

type fakeUsage struct{}

func (fakeUsage) Rows(context.Context, string, string) ([]wire.UsageRow, error) { return nil, nil }

var hash64 = strings.Repeat("0f", 32)

const mintBody = `{"sessionId":"s-1","generation":1,"owner":{"tid":"t-1","oid":"o-1"},"teamId":"team-1",` +
	`"projectId":"p-1","agentId":"a-1","sessionClass":"solo","podAddr":"10.244.1.7"}`
```

```go
func testAPI(keys *fakeKeys, auth Authenticator, mut ...func(*apiDeps)) http.Handler {
	d := apiDeps{Keys: keys, Bindings: fakeBindings{hash64: {KeyHash: hash64, SessionID: "s-1"}},
		Usage: fakeUsage{}, Auth: auth, MintWait: time.Second,
		Ready: func(context.Context) error { return nil },
		Log:   slog.New(slog.NewJSONHandler(io.Discard, nil))}
	for _, m := range mut {
		m(&d)
	}
	return newAPI(d)
}

func do(h http.Handler, method, path, body, tok string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if tok != "" {
		r.Header.Set("Authorization", "Bearer "+tok)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func problem(t *testing.T, w *httptest.ResponseRecorder) httpx.Problem {
	t.Helper()
	var p httpx.Problem
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil || p.Error == "" {
		t.Fatalf("body %q is not a problem: %v", w.Body.String(), err)
	}
	return p
}

func statusOf(t *testing.T, code string) int {
	t.Helper()
	s, ok := wire.Status(code)
	if !ok {
		t.Fatalf("wire.Status(%q) is unknown", code)
	}
	return s
}

func only(r Route) tokenAuth { return tokenAuth{"c": {Name: "c", Routes: map[Route]bool{r: true}}} }

// Mutant killed: checking that a caller is authenticated but not that it holds
// the route, so the gatekeeper's credential could mint.
func TestEveryRouteRefusesACallerNotListedForIt(t *testing.T) {
	routes := []struct {
		method, path, body string
		route              Route
	}{
		{http.MethodPost, "/v1/mint", mintBody, RouteMint},
		{http.MethodPost, "/v1/refresh", `{"sessionId":"s-1","generation":2}`, RouteRefresh},
		{http.MethodPost, "/v1/revoke", `{"sessionId":"s-1"}`, RouteRevoke},
		{http.MethodGet, "/v1/bindings/" + hash64, "", RouteBindings},
		{http.MethodGet, "/v1/usage?scope=project&id=p-1", "", RouteUsage},
	}
	for _, rt := range routes {
		others := Caller{Name: "others", Routes: map[Route]bool{}}
		for _, r := range allRoutes {
			others.Routes[r] = r != rt.route
		}
		keys := &fakeKeys{}
		h := testAPI(keys, tokenAuth{"others": others, "only": {Name: "only", Routes: map[Route]bool{rt.route: true}}})
		if w := do(h, rt.method, rt.path, rt.body, ""); w.Code != statusOf(t, wire.CodeUnauthenticated) ||
			problem(t, w).Error != wire.CodeUnauthenticated {
			t.Errorf("%s with no credential: %d %s", rt.path, w.Code, w.Body.String())
		}
		if w := do(h, rt.method, rt.path, rt.body, "others"); w.Code != statusOf(t, wire.CodeCallerForbidden) ||
			problem(t, w).Error != wire.CodeCallerForbidden {
			t.Errorf("%s by a caller holding every other route: %d %s", rt.path, w.Code, w.Body.String())
		}
		if len(keys.calls) != 0 {
			t.Errorf("%s: the service ran for a refused caller: %v", rt.path, keys.calls)
		}
		if w := do(h, rt.method, rt.path, rt.body, "only"); w.Code == http.StatusUnauthorized || w.Code == http.StatusForbidden {
			t.Errorf("%s by its own caller: %d %s", rt.path, w.Code, w.Body.String())
		}
	}
}

// Mutant killed: the mint run on the request's context, so the 504 or a
// client hanging up aborts it half-way (the Panacea chain).
func TestMintTimeoutAnswers504AndTheMintCompletes(t *testing.T) {
	keys := &fakeKeys{release: make(chan struct{}), done: make(chan struct{})}
	h := testAPI(keys, only(RouteMint), func(d *apiDeps) { d.MintWait = 50 * time.Millisecond })
	ctx, cancel := context.WithCancel(context.Background())
	r := httptest.NewRequest(http.MethodPost, "/v1/mint", strings.NewReader(mintBody)).WithContext(ctx)
	r.Header.Set("Authorization", "Bearer c")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	cancel() // the client is gone
	if w.Code != statusOf(t, wire.CodeMintTimeout) || problem(t, w).Error != wire.CodeMintTimeout {
		t.Fatalf("slow mint: %d %s, want mint_timeout", w.Code, w.Body.String())
	}
	close(keys.release)
	select {
	case <-keys.done:
	case <-time.After(2 * time.Second):
		t.Fatal("the mint never finished")
	}
	if keys.ctxErr != nil {
		t.Errorf("the mint's context died with the request: %v", keys.ctxErr)
	}
}
```

```go
// Mutants killed: every service error written as 500 (a broker cannot tell a
// budget refusal from an outage); an error's text in a 5xx body.
func TestServiceErrorsBecomeProblems(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{fmt.Errorf("%w: generation 0", keysmith.ErrInvalid), http.StatusBadRequest, httpx.CodeBadRequest},
		{fmt.Errorf("%w: p-1", wire.ErrRouteUnknown), statusOf(t, wire.CodeRouteUnknown), wire.CodeRouteUnknown},
		{&keysmith.BudgetExhaustedError{Dimension: budget.Team}, statusOf(t, wire.CodeBudgetExhausted), wire.CodeBudgetExhausted},
		{keysmith.ErrBudgetUnknown, statusOf(t, wire.CodeBudgetUnknown), wire.CodeBudgetUnknown},
		{keysmith.ErrBusy, http.StatusConflict, httpx.CodeConflict},
		{keysmith.ErrSuperseded, http.StatusConflict, httpx.CodeConflict},
		{fmt.Errorf("%w: litellm said sk-pa-canary", keysmith.ErrAdmin), http.StatusServiceUnavailable, httpx.CodeBackendUnavailable},
		{errors.New("boom sk-pa-canary"), http.StatusInternalServerError, httpx.CodeInternal},
	}
	for _, c := range cases {
		w := do(testAPI(&fakeKeys{err: c.err}, only(RouteMint)), http.MethodPost, "/v1/mint", mintBody, "c")
		if p := problem(t, w); w.Code != c.status || p.Error != c.code {
			t.Errorf("%v: %d %s, want %d %s", c.err, w.Code, p.Error, c.status, c.code)
		}
		if strings.Contains(w.Body.String(), "sk-pa-canary") {
			t.Errorf("%v: the body carries the error's text", c.err)
		}
	}
	w := do(testAPI(&fakeKeys{err: &keysmith.BudgetExhaustedError{Dimension: budget.Team}}, only(RouteMint)),
		http.MethodPost, "/v1/mint", mintBody, "c")
	if d := problem(t, w).Details["dimension"]; d != "team" {
		t.Errorf("exhausted dimension detail = %q, want team", d)
	}
	w = do(testAPI(&fakeKeys{err: keysmith.ErrNoSession}, only(RouteRefresh)),
		http.MethodPost, "/v1/refresh", `{"sessionId":"s-1","generation":2}`, "c")
	if w.Code != http.StatusNotFound || problem(t, w).Error != httpx.CodeSessionNotFound {
		t.Errorf("refresh with no session: %d %s", w.Code, w.Body.String())
	}
}

// Mutants killed: a miss answered 401 or 500, which the gatekeeper cannot
// negative-cache; a path value passed to the store unchecked.
func TestBindingsRoute(t *testing.T) {
	h := testAPI(&fakeKeys{}, only(RouteBindings))
	w := do(h, http.MethodGet, "/v1/bindings/"+hash64, "", "c")
	var b wire.Binding
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &b) != nil || b.KeyHash != hash64 {
		t.Errorf("hit: %d %s", w.Code, w.Body.String())
	}
	w = do(h, http.MethodGet, "/v1/bindings/"+strings.Repeat("0e", 32), "", "c")
	if w.Code != http.StatusNotFound || problem(t, w).Error != codeBindingNotFound {
		t.Errorf("miss: %d %s, want 404 %s", w.Code, w.Body.String(), codeBindingNotFound)
	}
	for _, bad := range []string{"short", strings.ToUpper(hash64), hash64 + "00"} {
		if w := do(h, http.MethodGet, "/v1/bindings/"+bad, "", "c"); w.Code != http.StatusBadRequest {
			t.Errorf("hash %q: %d, want 400", bad, w.Code)
		}
	}
}

// Mutants killed: an unknown scope passed to the reader; rows written as null
// (task 22's mirror type has no null); readyz leaking its error; the health
// routes behind caller authentication (the kubelet has no token).
func TestUsageAndHealthRoutes(t *testing.T) {
	h := testAPI(&fakeKeys{}, only(RouteUsage), func(d *apiDeps) {
		d.Ready = func(context.Context) error { return errors.New("dial 10.0.0.5:5432: password=pw-canary") }
	})
	if w := do(h, http.MethodGet, "/v1/usage?scope=project&id=p-1", "", "c"); w.Code != http.StatusOK ||
		strings.TrimSpace(w.Body.String()) != `{"rows":[]}` {
		t.Errorf("usage: %d %s, want 200 {\"rows\":[]}", w.Code, w.Body.String())
	}
	for _, q := range []string{"scope=org&id=x", "scope=project", "id=p-1"} {
		if w := do(h, http.MethodGet, "/v1/usage?"+q, "", "c"); w.Code != http.StatusBadRequest {
			t.Errorf("usage?%s: %d, want 400", q, w.Code)
		}
	}
	if w := do(h, http.MethodGet, "/healthz", "", ""); w.Code != http.StatusOK {
		t.Errorf("healthz: %d", w.Code)
	}
	if w := do(h, http.MethodGet, "/readyz", "", ""); w.Code != http.StatusServiceUnavailable ||
		strings.Contains(w.Body.String(), "pw-canary") {
		t.Errorf("readyz: %d %s", w.Code, w.Body.String())
	}
	if w := do(h, http.MethodGet, "/v1/nothing", "", "c"); w.Code != http.StatusNotFound ||
		problem(t, w).Error != wire.CodePathNotAllowed {
		t.Errorf("unknown path: %d %s", w.Code, w.Body.String())
	}
}
```

**[08b]** Create `control-plane/cmd/keysmith/config_test.go`:

```go
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/budget"
)

const pre = "PANACEA_APP_GATEWAY_"

// baseEnv is the least a valid keysmith needs, with its files in a temp dir.
func baseEnv(t *testing.T) map[string]string {
	t.Helper()
	dir := t.TempDir()
	write := func(name, content string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	return map[string]string{
		pre + "MASTER_KEY_FILE":   write("master_key", "sk-"+strings.Repeat("m", 40)+"\n"),
		pre + "DATABASE_URL_FILE": write("database_url", "postgresql://gateway:pw-canary@postgres:5432/gateway?sslmode=disable\n"),
		pre + "ROUTE_SOURCE":      "file:" + filepath.Join(dir, "projects.json"),
		pre + "CALLER_AUTH":       "bearer-file:" + write("callers", "controlplane "+tokA+" mint,refresh,revoke,usage\n"),
		pre + "BASE_URL":          "http://gatekeeper:4000",
	}
}

func getenv(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

// Mutants killed: a default changed or dropped; budgets off by default; the
// database URL printable through %v.
func TestConfigDefaults(t *testing.T) {
	cfg, err := loadConfig(getenv(baseEnv(t)))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LiteLLMURL != "http://litellm:4001" || cfg.TokenTTL != time.Hour || cfg.MintTimeout != 20*time.Second ||
		cfg.RouteCacheTTL != 30*time.Second || cfg.RouteStaleMax != 10*time.Minute {
		t.Errorf("defaults: litellm %q ttl %v mint %v cache %v stale %v",
			cfg.LiteLLMURL, cfg.TokenTTL, cfg.MintTimeout, cfg.RouteCacheTTL, cfg.RouteStaleMax)
	}
	if cfg.Limits != budget.Proposed {
		t.Errorf("limits = %+v, want budget.Proposed", cfg.Limits)
	}
	if len(cfg.Callers) != 1 || cfg.Callers[0].caller.Name != "controlplane" {
		t.Errorf("callers = %d entries", len(cfg.Callers))
	}
	if s := fmt.Sprintf("%v %+v", cfg, cfg); strings.Contains(s, "pw-canary") || strings.Contains(s, tokA) {
		t.Error("printing the config prints the database password or a caller token")
	}
}

// Mutants killed: an unusable setting defaulted instead of refused (the
// Panacea silent-empty key), refused without naming its variable, or refused
// with a secret file's content in the message.
func TestConfigRefusesNamingTheVariable(t *testing.T) {
	dir, n := t.TempDir(), 0
	file := func(content string) string {
		n++
		p := filepath.Join(dir, fmt.Sprintf("f%d", n))
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	cases := []struct{ variable, value string }{
		{"MASTER_KEY_FILE", ""},
		{"MASTER_KEY_FILE", file(" \n")},
		{"MASTER_KEY_FILE", file("not-a-litellm-key")},
		{"MASTER_KEY_FILE", filepath.Join(dir, "absent")},
		{"DATABASE_URL_FILE", ""},
		{"DATABASE_URL_FILE", file("\n\n")},
		{"DATABASE_URL_FILE", file("mysql://gateway:pw-canary@db/gateway")},
		{"ROUTE_SOURCE", ""},
		{"ROUTE_SOURCE", "s3:bucket"},
		{"HUB_URL", ""},   // with ROUTE_SOURCE=hub, below
		{"HUB_SCOPE", "api://hub/.default"}, // likewise; an Entra scope, refused since OD-30 (task 13's gap 1)
		{"CALLER_AUTH", "open"},
		{"CALLER_AUTH", "bearer-file:" + filepath.Join(dir, "absent")},
		{"BASE_URL", "http://gatekeeper:4000/v1"},
		{"TOKEN_TTL", "4m"}, // not above the five-minute refresh grace
		{"MINT_TIMEOUT", "0s"},
		{"ROUTE_STALE_MAX", "10s"}, // below ROUTE_CACHE_TTL
		{"SESSION_CAP_USD", "0"},
		{"DEFAULT_TEAM_BUDGET_USD", "-5"},
		{"DEFAULT_USER_BUDGET_USD", "NaN"},
		{"DEFAULT_PROJECT_BUDGET_USD", "lots"},
	}
	for _, c := range cases {
		m := baseEnv(t)
		if c.variable == "HUB_URL" || c.variable == "HUB_SCOPE" {
			m[pre+"ROUTE_SOURCE"], m[pre+"HUB_URL"], m[pre+"HUB_SCOPE"] = "hub", "https://hub.example", "panacea-app-hub"
		}
		m[pre+c.variable] = c.value
		_, err := loadConfig(getenv(m))
		if err == nil || !strings.Contains(err.Error(), pre+c.variable) {
			t.Errorf("%s=%q: err = %v, want a refusal naming %s", c.variable, c.value, err, pre+c.variable)
			continue
		}
		if strings.Contains(err.Error(), "pw-canary") || strings.Contains(err.Error(), "not-a-litellm-key") {
			t.Errorf("%s: the refusal quotes a secret file", c.variable)
		}
	}
}
```

**[08b]** Create `control-plane/cmd/keysmith/main_test.go`:

```go
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Mutant killed: keysmith starting without a master key (Panacea's
// optional: true), or exiting 1, which phase 2 reserves for runtime failures.
func TestRunRefusesAMissingOrEmptyMasterKey(t *testing.T) {
	for name, set := range map[string]func(m map[string]string, dir string){
		"unset": func(m map[string]string, _ string) { delete(m, pre+"MASTER_KEY_FILE") },
		"empty": func(m map[string]string, dir string) {
			p := filepath.Join(dir, "master_key")
			if err := os.WriteFile(p, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			m[pre+"MASTER_KEY_FILE"] = p
		},
	} {
		m := baseEnv(t)
		set(m, t.TempDir())
		var stderr bytes.Buffer
		if code := run(getenv(m), &stderr); code != 2 || !strings.Contains(stderr.String(), pre+"MASTER_KEY_FILE") {
			t.Errorf("%s: exit %d, stderr %q; want 2 naming %sMASTER_KEY_FILE", name, code, stderr.String(), pre)
		}
	}
}

// Mutant killed: a start-up refusal that prints a secret file's content.
func TestRunNeverPrintsASecretFile(t *testing.T) {
	m := baseEnv(t)
	p := filepath.Join(t.TempDir(), "database_url")
	if err := os.WriteFile(p, []byte("mysql://gateway:pw-canary@db/gateway"), 0o600); err != nil {
		t.Fatal(err)
	}
	m[pre+"DATABASE_URL_FILE"] = p
	var stderr bytes.Buffer
	code := run(getenv(m), &stderr)
	if code != 2 || strings.Contains(stderr.String(), "pw-canary") || !strings.Contains(stderr.String(), pre+"DATABASE_URL_FILE") {
		t.Errorf("exit %d, stderr %q: want 2, the variable's name, and no password", code, stderr.String())
	}
}
```

**[08a]** Create `control-plane/cmd/keysmith/calleroidc_test.go` (decision (g)). It also holds `tokenAuth`, the fake authenticator 08b's API tests share:

```go
package main

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/wire"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/oidcauth"
)

// tokenAuth: the bearer token is the caller's name.
type tokenAuth map[string]Caller

func (a tokenAuth) Identify(r *http.Request) (Caller, error) {
	if tok, ok := bearerToken(r); ok {
		if c, found := a[tok]; found {
			return c, nil
		}
	}
	return Caller{}, ErrUnauthenticated
}

const realm = "https://auth.example/realms/panacea"

// jwtWithIssuer is an unsigned three-part token whose payload names iss. Only
// issuerOf reads it; fakeValidator never checks a signature.
func jwtWithIssuer(iss string) string {
	enc := base64.RawURLEncoding.EncodeToString
	return enc([]byte(`{"alg":"RS256"}`)) + "." + enc([]byte(`{"iss":"`+iss+`"}`)) + ".c2ln"
}

// fakeValidator answers by token text and counts its calls.
type fakeValidator struct {
	claims map[string]oidcauth.Claims
	err    error
	calls  int
}

func (f *fakeValidator) Validate(_ context.Context, bearer string) (oidcauth.Claims, error) {
	f.calls++
	if f.err != nil {
		return oidcauth.Claims{}, f.err
	}
	if c, ok := f.claims[bearer]; ok {
		return c, nil
	}
	return oidcauth.Claims{}, oidcauth.ErrSignature
}

// Mutants killed: a service token accepted as a person; a person given
// bindings or usage; a realm key-set outage answered 401 instead of 503.
func TestOIDCAuthNamesThePersonOnly(t *testing.T) {
	alice, fleet := jwtWithIssuer(realm), jwtWithIssuer(realm)+"f"
	v := &fakeValidator{claims: map[string]oidcauth.Claims{
		alice: {TenantID: "t-1", ObjectID: "o-1", ClientID: "panacea-app-desktop"},
		fleet: {ClientID: "panacea-app-fleet", Service: true},
	}}
	a := oidcAuth{v: v}
	c, err := a.Identify(withBearer(alice))
	if err != nil || c.Name != "person:t-1/o-1" || c.RouteList() != "mint,refresh,revoke" ||
		c.Owner == nil || *c.Owner != (wire.Owner{TID: "t-1", OID: "o-1"}) {
		t.Fatalf("Identify(alice) = %+v, %v", c, err)
	}
	if _, err := a.Identify(withBearer(fleet)); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("a service token: err = %v, want ErrUnauthenticated", err)
	}
	v.err = oidcauth.ErrKeySetUnavailable
	if _, err := a.Identify(withBearer(alice)); !errors.Is(err, ErrReviewUnavailable) {
		t.Errorf("key set down: err = %v, want ErrReviewUnavailable (503)", err)
	}
}

// Mutants killed: a realm token that oidc refuses falling through to the
// table; the table asked about a realm token; a foreign token sent to oidc.
func TestChainAuthChoosesByIssuerAndNeverFallsThrough(t *testing.T) {
	realmTok, other := jwtWithIssuer(realm), jwtWithIssuer("https://kubernetes.default.svc")
	v := &fakeValidator{claims: map[string]oidcauth.Claims{}}
	broker := Caller{Name: "broker", Routes: map[Route]bool{RouteMint: true}}
	table := tokenAuth{realmTok: broker, other: broker, "opaque": {Name: "gatekeeper"}}
	a := chainAuth{issuer: realm, oidc: oidcAuth{v: v}, table: table}
	if _, err := a.Identify(withBearer(realmTok)); !errors.Is(err, ErrUnauthenticated) || v.calls != 1 {
		t.Errorf("a refused realm token: err = %v, oidc calls %d; want oidc's refusal alone", err, v.calls)
	}
	if c, err := a.Identify(withBearer(other)); err != nil || c.Name != "broker" || v.calls != 1 {
		t.Errorf("a service-account token: %+v, %v, oidc calls %d", c, err, v.calls)
	}
	if c, err := a.Identify(withBearer("opaque")); err != nil || c.Name != "gatekeeper" || v.calls != 1 {
		t.Errorf("an opaque token: %+v, %v", c, err)
	}
	if _, err := (chainAuth{issuer: realm, oidc: oidcAuth{v: v}}).Identify(withBearer(other)); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("oidc alone, a foreign token: err = %v", err)
	}
}
```

**[08b]** Create `control-plane/cmd/keysmith/owner_test.go` (decision (g); `realm` and `tokenAuth` are 08a's):

```go
package main

import (
	"context"
	"maps"
	"net/http"
	"strings"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/wire"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/oidcauth"
)

type fakeSessions map[string][]wire.Binding

func (f fakeSessions) BySession(_ context.Context, id string) ([]wire.Binding, error) { return f[id], nil }

// Mutants killed: the body-owner compare deleted; the BySession check deleted
// (another's session minted, refreshed or revoked); a table caller refused.
func TestAPIRefusesAPersonActingForAnotherOwner(t *testing.T) {
	me := wire.Owner{TID: "t-1", OID: "o-1"}
	person := Caller{Name: "person:t-1/o-1", Owner: &me,
		Routes: map[Route]bool{RouteMint: true, RouteRefresh: true, RouteRevoke: true}}
	auth := tokenAuth{"me": person, "broker": {Name: "broker", Routes: person.Routes}}
	sessions := fakeSessions{"s-1": {{Owner: me}}, "s-2": {{Owner: me}, {Owner: wire.Owner{TID: "t-1", OID: "o-2"}}}}
	keys := &fakeKeys{}
	h := testAPI(keys, auth, func(d *apiDeps) { d.Sessions = sessions })
	other := strings.Replace(mintBody, `"oid":"o-1"`, `"oid":"o-2"`, 1)
	cases := []struct {
		path, body, tok string
		want            int
	}{
		{"/v1/mint", mintBody, "me", http.StatusCreated},
		{"/v1/mint", other, "me", http.StatusForbidden},
		{"/v1/mint", strings.Replace(mintBody, `"s-1"`, `"s-2"`, 1), "me", http.StatusForbidden},
		{"/v1/mint", other, "broker", http.StatusCreated}, // a table caller names the owner
		{"/v1/refresh", `{"sessionId":"s-1","generation":2}`, "me", http.StatusCreated},
		{"/v1/refresh", `{"sessionId":"s-2","generation":2}`, "me", http.StatusForbidden},
		{"/v1/revoke", `{"sessionId":"s-2"}`, "me", http.StatusForbidden},
		{"/v1/revoke", `{"sessionId":"s-1"}`, "me", http.StatusNoContent},
	}
	for _, c := range cases {
		w := do(h, http.MethodPost, c.path, c.body, c.tok)
		if w.Code != c.want {
			t.Errorf("%s %s as %s: status %d, want %d", c.path, c.body, c.tok, w.Code, c.want)
		} else if c.want == http.StatusForbidden && problem(t, w).Error != wire.CodeCallerForbidden {
			t.Errorf("%s as %s: code %q", c.path, c.tok, problem(t, w).Error)
		}
	}
	if got := strings.Join(keys.calls, ","); got != "mint,mint,refresh,revoke" {
		t.Errorf("service calls = %s; a refused request reached the service", got)
	}
}

// Mutants killed: a second table mode or a second oidc accepted; oidc without
// its issuer or tenants defaulted instead of refused; the entra profile let in.
func TestConfigCallerAuthIsACommaList(t *testing.T) {
	m := baseEnv(t)
	m[pre+"CALLER_AUTH"] = "kube,oidc"
	m["PANACEA_APP_OIDC_ISSUER"] = realm + "/"
	m[pre+"TENANTS"] = "11111111-1111-1111-1111-111111111111"
	cfg, err := loadConfig(getenv(m))
	if err != nil || cfg.CallerMode != "kube" || cfg.OIDC == nil || cfg.OIDC.Issuer != realm ||
		cfg.OIDC.Audience != "panacea-app-gateway" || cfg.OIDC.RequiredScope != "panacea-app-gateway" ||
		cfg.OIDC.Profile != oidcauth.ProfileKeycloak || len(cfg.OIDC.ServiceClients) != 0 ||
		strings.Join(cfg.OIDC.AllowedClients, ",") != "panacea-app-desktop" {
		t.Fatalf("kube,oidc: %v %+v, %v", cfg, cfg.OIDC, err)
	}
	for _, c := range []struct{ variable, value string }{
		{pre + "CALLER_AUTH", "kube,kube"},
		{pre + "CALLER_AUTH", "oidc,oidc"},
		{"PANACEA_APP_OIDC_ISSUER", ""},
		{pre + "TENANTS", ""},
		{"PANACEA_APP_OIDC_PROFILE", "entra"},
	} {
		bad := maps.Clone(m)
		bad[c.variable] = c.value
		if _, err := loadConfig(getenv(bad)); err == nil || !strings.Contains(err.Error(), c.variable) {
			t.Errorf("%s=%q: err = %v, want one naming %s", c.variable, c.value, err, c.variable)
		}
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

```bash
cd control-plane && go test ./cmd/keysmith/ -count=1 > /tmp/ks-red.txt 2>&1; echo "exit $?"; head -5 /tmp/ks-red.txt
```

Expected, **[08a]**: a non-zero exit and a build failure naming `undefined: parseCallers`, `undefined: bearerToken` or `undefined: oidcAuth`. **[08b]**: a non-zero exit and one naming `undefined: apiDeps`, `undefined: loadConfig` or `undefined: ownerMayAct` (08a's six still compile; the new files do not).

- [ ] **Step 4: Write the implementation**

**[08a]** writes `callerauth.go`, `calleroidc.go` and the placeholder `main.go` below. **[08b]** writes `owner.go`, `api.go`, `config.go` and replaces `main.go`.

**[08a]** Create `control-plane/cmd/keysmith/callerauth.go`:

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	authv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	authnclient "k8s.io/client-go/kubernetes/typed/authentication/v1"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/wire"
)

// Route is a keysmith route a caller may hold.
type Route string

const (
	RouteMint     Route = "mint"
	RouteRefresh  Route = "refresh"
	RouteRevoke   Route = "revoke"
	RouteBindings Route = "bindings"
	RouteUsage    Route = "usage"
)

// allRoutes is every route, in the order RouteList prints them.
var allRoutes = []Route{RouteMint, RouteRefresh, RouteRevoke, RouteBindings, RouteUsage}

// Caller is an authenticated caller and the routes it holds. Owner is set
// only for a person's own realm token (decision (g)); a table caller has none.
type Caller struct {
	Name   string
	Routes map[Route]bool
	Owner  *wire.Owner
}

// RouteList is the caller's routes, comma-separated, in allRoutes order.
func (c Caller) RouteList() string {
	var out []string
	for _, r := range allRoutes {
		if c.Routes[r] {
			out = append(out, string(r))
		}
	}
	return strings.Join(out, ",")
}

// Authenticator names the caller of a request.
type Authenticator interface {
	Identify(r *http.Request) (Caller, error)
}

var (
	ErrUnauthenticated   = errors.New("keysmith: caller not authenticated")
	ErrReviewUnavailable = errors.New("keysmith: the TokenReview API is unavailable")
)

// builtinKubeTable is CALLER_AUTH=kube's table (decision (b)).
const builtinKubeTable = `# Fleet has no line: it is off until seam S6.5.
broker     system:serviceaccount:panacea-broker:broker       mint,refresh,revoke
gatekeeper system:serviceaccount:panacea-gateway:gatekeeper  bindings
`

// entry is one line of a caller table. credential is a token or a username.
type entry struct {
	caller     Caller
	credential string
}

var callerName = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)

// parseCallers reads "name credential routes" lines. With bearer, the
// credential is a token of 32 characters or more; otherwise a service-account
// username. An error names the line and the caller, never the credential.
func parseCallers(data []byte, bearer bool) ([]entry, error) {
	var out []entry
	names, creds := map[string]bool{}, map[string]bool{}
	for i, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		n, f := i+1, strings.Fields(line)
		if len(f) != 3 {
			return nil, fmt.Errorf("line %d: want three fields: name credential routes", n)
		}
		name, cred := f[0], f[1]
		switch {
		case !callerName.MatchString(name):
			return nil, fmt.Errorf("line %d: a name is lowercase letters, digits and hyphens", n)
		case names[name]:
			return nil, fmt.Errorf("line %d (%s): the name repeats an earlier line's", n, name)
		case creds[cred]:
			return nil, fmt.Errorf("line %d (%s): the credential repeats an earlier line's", n, name)
		case bearer && len(cred) < 32:
			return nil, fmt.Errorf("line %d (%s): the token is shorter than 32 characters", n, name)
		case !bearer && !strings.HasPrefix(cred, "system:serviceaccount:"):
			return nil, fmt.Errorf("line %d (%s): the credential is not a service-account username", n, name)
		}
		routes := map[Route]bool{}
		for _, r := range strings.Split(f[2], ",") {
			if !slices.Contains(allRoutes, Route(r)) {
				return nil, fmt.Errorf("line %d (%s): unknown route %q", n, name, r)
			}
			routes[Route(r)] = true
		}
		names[name], creds[cred] = true, true
		out = append(out, entry{caller: Caller{Name: name, Routes: routes}, credential: cred})
	}
	if len(out) == 0 {
		return nil, errors.New("the caller table has no callers")
	}
	return out, nil
}
```

```go
// readCallers reads and parses a caller table file. The error names the path.
func readCallers(path string, bearer bool) ([]entry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("the caller table %s cannot be read", path)
	}
	entries, err := parseCallers(data, bearer)
	if err != nil {
		return nil, fmt.Errorf("the caller table %s: %w", path, err)
	}
	return entries, nil
}

// bearerToken is the Authorization: Bearer value, if there is one.
func bearerToken(r *http.Request) (string, bool) {
	tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	return tok, ok && tok != ""
}

// bearerAuth is local mode's caller authentication: a token looked up by its
// hash, so no comparison ever runs over the secret itself.
type bearerAuth struct{ byHash map[string]Caller }

func newBearerAuth(path string) (*bearerAuth, error) {
	entries, err := readCallers(path, true)
	if err != nil {
		return nil, err
	}
	return bearerAuthFrom(entries), nil
}

func bearerAuthFrom(entries []entry) *bearerAuth {
	a := &bearerAuth{byHash: map[string]Caller{}}
	for _, e := range entries {
		a.byHash[wire.HashToken(e.credential)] = e.caller
	}
	return a
}

func (a *bearerAuth) Identify(r *http.Request) (Caller, error) {
	tok, ok := bearerToken(r)
	if !ok {
		return Caller{}, ErrUnauthenticated
	}
	if c, found := a.byHash[wire.HashToken(tok)]; found {
		return c, nil
	}
	return Caller{}, ErrUnauthenticated
}

const (
	kubeAudience   = "keysmith"
	reviewTTL      = 60 * time.Second
	reviewCacheMax = 1024
)

type reviewed struct {
	c     Caller
	until time.Time
}

// kubeAuth is the cloud's caller authentication: a projected service-account
// token for audience keysmith, checked by TokenReview (decision (e)).
type kubeAuth struct {
	reviews   authnclient.TokenReviewInterface
	bySubject map[string]Caller
	now       func() time.Time
	mu        sync.Mutex
	cache     map[string]reviewed
}

func newKubeAuth(reviews authnclient.TokenReviewInterface, entries []entry, now func() time.Time) *kubeAuth {
	if now == nil {
		now = time.Now
	}
	k := &kubeAuth{reviews: reviews, bySubject: map[string]Caller{}, now: now, cache: map[string]reviewed{}}
	for _, e := range entries {
		k.bySubject[e.credential] = e.caller
	}
	return k
}

func (k *kubeAuth) Identify(r *http.Request) (Caller, error) {
	tok, ok := bearerToken(r)
	if !ok {
		return Caller{}, ErrUnauthenticated
	}
	key, now := wire.HashToken(tok), k.now()
	k.mu.Lock()
	hit, found := k.cache[key]
	k.mu.Unlock()
	if found && now.Before(hit.until) {
		return hit.c, nil
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	tr, err := k.reviews.Create(ctx, &authv1.TokenReview{
		Spec: authv1.TokenReviewSpec{Token: tok, Audiences: []string{kubeAudience}},
	}, metav1.CreateOptions{})
	if err != nil {
		return Caller{}, fmt.Errorf("%w: %v", ErrReviewUnavailable, err)
	}
	if !tr.Status.Authenticated || !slices.Contains(tr.Status.Audiences, kubeAudience) {
		return Caller{}, ErrUnauthenticated
	}
	c, listed := k.bySubject[tr.Status.User.Username]
	if !listed { // authenticated, but no route: every route answers 403
		c = Caller{Name: tr.Status.User.Username, Routes: map[Route]bool{}}
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	if len(k.cache) >= reviewCacheMax {
		for h, v := range k.cache {
			if !now.Before(v.until) {
				delete(k.cache, h)
			}
		}
	}
	if len(k.cache) < reviewCacheMax {
		k.cache[key] = reviewed{c: c, until: now.Add(reviewTTL)}
	}
	return c, nil
}
```

**[08a]** Create `control-plane/cmd/keysmith/calleroidc.go` (decision (g), `OD-30`):

```go
package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/wire"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/oidcauth"
)

// oidcConfig lets config.go hold the validator's settings without the import.
type oidcConfig = oidcauth.Config

// personRoutes are the only routes a person's own token holds.
var personRoutes = map[Route]bool{RouteMint: true, RouteRefresh: true, RouteRevoke: true}

// oidcAuth is a person's own realm token, audience panacea-app-gateway.
type oidcAuth struct{ v oidcauth.Validator }

func newOIDCAuth(cfg oidcConfig) (oidcAuth, error) {
	v, err := oidcauth.New(cfg)
	if err != nil {
		return oidcAuth{}, err
	}
	return oidcAuth{v: v}, nil
}

func (a oidcAuth) Identify(r *http.Request) (Caller, error) {
	tok, ok := bearerToken(r)
	if !ok {
		return Caller{}, ErrUnauthenticated
	}
	cl, err := a.v.Validate(r.Context(), tok)
	switch {
	case errors.Is(err, oidcauth.ErrKeySetUnavailable): // nobody can be authenticated: 503
		return Caller{}, fmt.Errorf("%w: %v", ErrReviewUnavailable, err)
	case err != nil, cl.Service, cl.TenantID == "", cl.ObjectID == "":
		return Caller{}, ErrUnauthenticated
	}
	owner := wire.Owner{TID: cl.TenantID, OID: cl.ObjectID}
	return Caller{Name: "person:" + cl.Key(), Routes: personRoutes, Owner: &owner}, nil
}

// issuerOf is a JWT's unverified iss, or "" for anything else. It only
// chooses an authenticator; the chosen one verifies everything.
func issuerOf(tok string) string {
	parts := strings.Split(tok, ".")
	if len(parts) != 3 || len(parts[1]) > 16<<10 {
		return ""
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var p struct {
		Iss string `json:"iss"`
	}
	if json.Unmarshal(raw, &p) != nil {
		return ""
	}
	return p.Iss
}

// chainAuth is CALLER_AUTH's comma list: oidc decides a token from the realm,
// the table mode every other token, and neither falls through to the other.
type chainAuth struct {
	issuer string
	oidc   Authenticator // nil: no oidc mode
	table  Authenticator // nil: no table mode
}

func (a chainAuth) Identify(r *http.Request) (Caller, error) {
	tok, ok := bearerToken(r)
	switch {
	case !ok:
		return Caller{}, ErrUnauthenticated
	case a.oidc != nil && issuerOf(tok) == a.issuer:
		return a.oidc.Identify(r)
	case a.table != nil:
		return a.table.Identify(r)
	}
	return Caller{}, ErrUnauthenticated
}
```

**[08a]** Create `control-plane/cmd/keysmith/main.go`, the placeholder. It exists only so `go build ./...` links between 08a and 08b, and it refuses to start, as keysmith must when it is not fully configured:

```go
package main

import (
	"fmt"
	"os"
)

// main is 08a's placeholder; task 08b replaces this file whole.
func main() {
	fmt.Fprintln(os.Stderr, "keysmith: refusing to start: the API is not wired (task 08b)")
	os.Exit(2)
}
```

**[08b]** Create `control-plane/cmd/keysmith/owner.go` (decision (g)). It reads `envPrefix` (`config.go`) and `apiDeps` (`api.go`), which is why it is 08b's and not 08a's:

```go
package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/wire"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/oidcauth"
)

// loadOIDC reads decision (g)'s variables; each refusal names its variable.
func loadOIDC(getenv func(string) string) (*oidcConfig, error) {
	env := func(k string) string { return strings.TrimSpace(getenv(k)) }
	bad := func(k, why string) (*oidcConfig, error) { return nil, fmt.Errorf("%s %s", k, why) }
	profile := oidcauth.Profile(env("PANACEA_APP_OIDC_PROFILE"))
	if profile == "" {
		profile = oidcauth.ProfileKeycloak
	}
	if profile != oidcauth.ProfileKeycloak {
		return bad("PANACEA_APP_OIDC_PROFILE", `must be "keycloak" when CALLER_AUTH holds oidc (ruling §6)`)
	}
	issuer := strings.TrimSuffix(env("PANACEA_APP_OIDC_ISSUER"), "/")
	if issuer == "" {
		return bad("PANACEA_APP_OIDC_ISSUER", "is not set; CALLER_AUTH=oidc needs the realm URL")
	}
	tenants := commaList(env(envPrefix + "TENANTS"))
	if len(tenants) == 0 {
		return bad(envPrefix+"TENANTS", "is not set; CALLER_AUTH=oidc needs the enrolled tenant ids")
	}
	aud := env(envPrefix + "AUDIENCE")
	if aud == "" {
		aud = "panacea-app-gateway"
	}
	clients := commaList(env(envPrefix + "ALLOWED_CLIENTS"))
	if len(clients) == 0 {
		clients = []string{"panacea-app-desktop"}
	}
	return &oidcConfig{Profile: profile, Issuer: issuer, TenantIDs: tenants, Audience: aud,
		RequiredScope: aud, AllowedClients: clients}, nil // ServiceClients stays empty
}

func commaList(s string) []string {
	var out []string
	for _, f := range strings.Split(s, ",") {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}

// sessionBindings is task 05's *binding.PGStore as the owner check uses it.
type sessionBindings interface {
	BySession(ctx context.Context, sessionID string) ([]wire.Binding, error)
}

type callerKey struct{}

// callerOf is the Caller guard authenticated.
func callerOf(r *http.Request) Caller {
	c, _ := r.Context().Value(callerKey{}).(Caller)
	return c
}

// ownerMayAct passes a table caller. A person passes only when body (if any)
// is its own Owner and every binding of the session is too; else it writes
// 403 caller_forbidden, or 503 when the store cannot say.
func (d apiDeps) ownerMayAct(w http.ResponseWriter, r *http.Request, route string, body *wire.Owner, sessionID string) bool {
	c := callerOf(r)
	if c.Owner == nil {
		return true
	}
	refuse := func() bool {
		d.Log.Warn("caller refused: another owner", "route", route, "caller", c.Name)
		d.wireProblem(w, wire.CodeCallerForbidden, "a person may act only for their own sessions", nil)
		return false
	}
	if body != nil && *body != *c.Owner {
		return refuse()
	}
	bs, err := d.Sessions.BySession(r.Context(), sessionID)
	if err != nil {
		d.Log.Error("session owner lookup failed", "route", route, "err", err.Error())
		httpx.WriteProblem(w, http.StatusServiceUnavailable, httpx.CodeBackendUnavailable, "the binding store is unavailable", nil)
		return false
	}
	for _, b := range bs {
		if b.Owner != *c.Owner {
			return refuse()
		}
	}
	return true
}
```

**[08b]** Create `control-plane/cmd/keysmith/api.go`:

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/wire"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/keysmith"
)

// codeBindingNotFound is not in task 03's list yet (gap 1). Its status is 404.
const codeBindingNotFound = "binding_not_found"

// keyService is task 07's *keysmith.Service as the API uses it.
type keyService interface {
	Mint(ctx context.Context, r wire.MintRequest) (wire.MintResponse, error)
	Refresh(ctx context.Context, r wire.RefreshRequest) (wire.MintResponse, error)
	RevokeSession(ctx context.Context, r wire.RevokeRequest) (int, error)
}

// usageRows is task 09's *usage.Reader as the API uses it.
type usageRows interface {
	Rows(ctx context.Context, scope, id string) ([]wire.UsageRow, error)
}

type apiDeps struct {
	Keys     keyService
	Bindings wire.BindingReader
	Usage    usageRows
	Auth     Authenticator
	Sessions sessionBindings // task 05's store: a person's session-owner check (decision (g))
	MintWait time.Duration   // Config.MintWait: MINT_TIMEOUT less 2 s (decision (c))
	Ready    func(context.Context) error
	Log      *slog.Logger
}

var keyHashPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func newAPI(d apiDeps) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("POST /v1/mint", d.guard(RouteMint, d.mint))
	mux.Handle("POST /v1/refresh", d.guard(RouteRefresh, d.refresh))
	mux.Handle("POST /v1/revoke", d.guard(RouteRevoke, d.revoke))
	mux.Handle("GET /v1/bindings/{keyHash}", d.guard(RouteBindings, d.binding))
	mux.Handle("GET /v1/usage", d.guard(RouteUsage, d.usage))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { writeOK(w) })
	mux.HandleFunc("GET /readyz", d.ready)
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		d.wireProblem(w, wire.CodePathNotAllowed, "keysmith has no such route", nil)
	})
	return mux
}

func writeOK(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

// guard authenticates the caller and checks it holds route, before the
// handler reads a byte of the body.
func (d apiDeps) guard(route Route, h func(http.ResponseWriter, *http.Request, string)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := d.Auth.Identify(r)
		switch {
		case errors.Is(err, ErrReviewUnavailable):
			d.Log.Error("caller review failed", "route", string(route), "err", err.Error())
			httpx.WriteProblem(w, http.StatusServiceUnavailable, httpx.CodeBackendUnavailable,
				"caller authentication is unavailable", nil)
		case err != nil:
			d.wireProblem(w, wire.CodeUnauthenticated, "no valid caller credential", nil)
		case !c.Routes[route]:
			d.Log.Warn("caller refused", "route", string(route), "caller", c.Name)
			d.wireProblem(w, wire.CodeCallerForbidden, "this caller may not use "+string(route), nil)
		default: // the handler reads c back through callerOf (decision (g))
			h(w, r.WithContext(context.WithValue(r.Context(), callerKey{}, c)), c.Name)
		}
	})
}

// wireProblem writes one of task 03's codes with its status.
func (d apiDeps) wireProblem(w http.ResponseWriter, code, msg string, details map[string]string) {
	status, ok := wire.Status(code)
	if !ok {
		d.Log.Error("unknown wire code", "code", code)
		status, code, msg, details = http.StatusInternalServerError, httpx.CodeInternal, "internal error", nil
	}
	httpx.WriteProblem(w, status, code, msg, details)
}
```

```go
func (d apiDeps) mint(w http.ResponseWriter, r *http.Request, caller string) {
	var req wire.MintRequest
	if !httpx.DecodeJSON(w, r, &req) || !d.ownerMayAct(w, r, "mint", &req.Owner, req.SessionID) {
		return
	}
	d.await(w, r, caller, "mint", func(ctx context.Context) (wire.MintResponse, error) { return d.Keys.Mint(ctx, req) })
}

func (d apiDeps) refresh(w http.ResponseWriter, r *http.Request, caller string) {
	var req wire.RefreshRequest
	if !httpx.DecodeJSON(w, r, &req) || !d.ownerMayAct(w, r, "refresh", nil, req.SessionID) {
		return
	}
	d.await(w, r, caller, "refresh", func(ctx context.Context) (wire.MintResponse, error) { return d.Keys.Refresh(ctx, req) })
}

// await runs fn on a context the request cannot cancel and answers with its
// result, or with 504 mint_timeout after MintWait while fn carries on. A late
// result is dropped; the caller's retry of the same generation supersedes it.
func (d apiDeps) await(w http.ResponseWriter, r *http.Request, caller, route string,
	fn func(context.Context) (wire.MintResponse, error)) {
	type result struct {
		resp wire.MintResponse
		err  error
	}
	done := make(chan result, 1)
	ctx := context.WithoutCancel(r.Context())
	go func() {
		resp, err := fn(ctx)
		done <- result{resp, err}
	}()
	timer := time.NewTimer(d.MintWait)
	defer timer.Stop()
	select {
	case res := <-done:
		if res.err != nil {
			d.fail(w, route, caller, res.err)
			return
		}
		d.Log.Info("minted", "route", route, "caller", caller, "inference", string(res.resp.Route))
		httpx.WriteJSON(w, http.StatusCreated, res.resp)
	case <-timer.C:
		d.Log.Warn("mint still running", "route", route, "caller", caller)
		d.wireProblem(w, wire.CodeMintTimeout, "the mint is still running; retry the same generation", nil)
	}
}

func (d apiDeps) revoke(w http.ResponseWriter, r *http.Request, caller string) {
	var req wire.RevokeRequest
	if !httpx.DecodeJSON(w, r, &req) || !d.ownerMayAct(w, r, "revoke", nil, req.SessionID) {
		return
	}
	if _, err := d.Keys.RevokeSession(context.WithoutCancel(r.Context()), req); err != nil {
		d.fail(w, "revoke", caller, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (d apiDeps) binding(w http.ResponseWriter, r *http.Request, caller string) {
	h := r.PathValue("keyHash")
	if !keyHashPattern.MatchString(h) {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.CodeBadRequest, "keyHash is 64 lowercase hex characters", nil)
		return
	}
	b, err := d.Bindings.ByKeyHash(r.Context(), h)
	switch {
	case errors.Is(err, wire.ErrNotFound):
		httpx.WriteProblem(w, http.StatusNotFound, codeBindingNotFound, "no binding for that key", nil)
	case err != nil:
		d.fail(w, "bindings", caller, fmt.Errorf("%w: %v", keysmith.ErrStore, err))
	default:
		httpx.WriteJSON(w, http.StatusOK, b)
	}
}

func (d apiDeps) usage(w http.ResponseWriter, r *http.Request, caller string) {
	scope, id := r.URL.Query().Get("scope"), r.URL.Query().Get("id")
	if (scope != wire.ScopePerson && scope != wire.ScopeProject && scope != wire.ScopeTeam) || id == "" {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.CodeBadRequest,
			"scope is person, project or team, and id is required", nil)
		return
	}
	rows, err := d.Usage.Rows(r.Context(), scope, id)
	if err != nil {
		d.fail(w, "usage", caller, fmt.Errorf("%w: %v", keysmith.ErrStore, err))
		return
	}
	if rows == nil {
		rows = []wire.UsageRow{}
	}
	httpx.WriteJSON(w, http.StatusOK, struct {
		Rows []wire.UsageRow `json:"rows"`
	}{rows})
}
```

```go
// ready is the readiness probe. Ready returns fixed sentences (main.go), and
// this handler writes neither of them: the body is one fixed line.
func (d apiDeps) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	if err := d.Ready(ctx); err != nil {
		d.Log.Warn("not ready", "why", err.Error())
		httpx.WriteProblem(w, http.StatusServiceUnavailable, httpx.CodeBackendUnavailable,
			"the database or LiteLLM is unreachable", nil)
		return
	}
	writeOK(w)
}

// fail writes a service error as a problem: a 4xx with its sentence, a 5xx
// with a fixed sentence and the error in the log (decision (f)).
func (d apiDeps) fail(w http.ResponseWriter, route, caller string, err error) {
	logged := func() { d.Log.Error("request failed", "route", route, "caller", caller, "err", err.Error()) }
	var be *keysmith.BudgetExhaustedError
	switch {
	case errors.Is(err, keysmith.ErrInvalid):
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error(), nil)
	case errors.Is(err, wire.ErrRouteUnknown):
		logged()
		d.wireProblem(w, wire.CodeRouteUnknown, "the project's inference route is unknown; nothing was minted", nil)
	case errors.As(err, &be):
		dim := string(be.Dimension)
		d.wireProblem(w, wire.CodeBudgetExhausted, "the "+dim+" budget is exhausted this month",
			map[string]string{"dimension": dim})
	case errors.Is(err, keysmith.ErrBudgetUnknown):
		logged()
		d.wireProblem(w, wire.CodeBudgetUnknown, "the budget could not be read; nothing was minted", nil)
	case errors.Is(err, keysmith.ErrNoSession):
		httpx.WriteProblem(w, http.StatusNotFound, httpx.CodeSessionNotFound, "no live generation to refresh", nil)
	case errors.Is(err, keysmith.ErrBusy), errors.Is(err, keysmith.ErrSuperseded):
		httpx.WriteProblem(w, http.StatusConflict, httpx.CodeConflict,
			"another mint of this generation is in flight; retry the same generation", nil)
	case errors.Is(err, keysmith.ErrAdmin), errors.Is(err, keysmith.ErrStore):
		logged()
		httpx.WriteProblem(w, http.StatusServiceUnavailable, httpx.CodeBackendUnavailable,
			"LiteLLM or the binding store is unavailable", nil)
	default:
		logged()
		httpx.WriteProblem(w, http.StatusInternalServerError, httpx.CodeInternal, "internal error", nil)
	}
}
```

**[08b]** Create `control-plane/cmd/keysmith/config.go`:

```go
package main

import (
	"fmt"
	"math"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/budget"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/wire"
)

const envPrefix = "PANACEA_APP_GATEWAY_"

// refreshGrace is task 07's default Deps.Grace; TOKEN_TTL must exceed it.
const refreshGrace = 5 * time.Minute

// secret prints as [redacted] under every verb that honours String.
type secret string

func (secret) String() string   { return "[redacted]" }
func (secret) GoString() string { return "[redacted]" }

// Config is every variable keysmith reads, parsed once (global constraint 5).
type Config struct {
	LiteLLMURL    string
	MasterKeyFile string
	DatabaseURL   secret
	RouteSource   string // "hub" or "file:<path>"
	HubURL        string
	HubScope      string      // the realm audience scope, default panacea-app-hub (task 13, OD-30)
	CallerMode    string      // the table mode: "bearer", "kube", or "" when CALLER_AUTH is oidc alone
	Callers       []entry
	OIDC          *oidcConfig // set when CALLER_AUTH holds oidc (decision (g))
	BaseURL       string
	TokenTTL      time.Duration
	MintTimeout   time.Duration
	RouteCacheTTL time.Duration
	RouteStaleMax time.Duration
	Limits        budget.Limits
}

// String is what %v and %+v print: no secret, no caller credential.
func (c Config) String() string {
	return fmt.Sprintf("litellm=%s routes=%s callers=%s(%d) oidc=%t base=%s ttl=%v mint=%v",
		c.LiteLLMURL, c.RouteSource, c.CallerMode, len(c.Callers), c.OIDC != nil, c.BaseURL, c.TokenTTL, c.MintTimeout)
}

// MintWait is how long the API waits before answering 504 (decision (c)).
func (c Config) MintWait() time.Duration {
	return max(c.MintTimeout-2*time.Second, time.Second)
}
```

```go
// loadConfig reads every variable through getenv, once. Each refusal names
// its variable and never quotes a secret file.
func loadConfig(getenv func(string) string) (Config, error) {
	get := func(k string) string { return strings.TrimSpace(getenv(envPrefix + k)) }
	bad := func(k, why string) (Config, error) { return Config{}, fmt.Errorf("%s%s %s", envPrefix, k, why) }
	var c Config
	var err error

	if c.LiteLLMURL = get("LITELLM_URL"); c.LiteLLMURL == "" {
		c.LiteLLMURL = "http://litellm:4001"
	}
	if wire.CheckBaseURL(c.LiteLLMURL) != nil {
		return bad("LITELLM_URL", "must be http(s)://host[:port] with no path")
	}
	if c.MasterKeyFile = get("MASTER_KEY_FILE"); c.MasterKeyFile == "" {
		return bad("MASTER_KEY_FILE", "is not set; keysmith does not start without LiteLLM's master key")
	}
	key, why := readSecretFile(c.MasterKeyFile)
	if why != "" {
		return bad("MASTER_KEY_FILE", why)
	}
	if !strings.HasPrefix(key, "sk-") {
		return bad("MASTER_KEY_FILE", "does not hold a LiteLLM key (no sk- prefix)")
	}
	dbFile := get("DATABASE_URL_FILE")
	if dbFile == "" {
		return bad("DATABASE_URL_FILE", "is not set")
	}
	dsn, why := readSecretFile(dbFile)
	if why != "" {
		return bad("DATABASE_URL_FILE", why)
	}
	// url.Parse's error quotes its input, so it is never shown.
	if u, perr := url.Parse(dsn); perr != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Host == "" {
		return bad("DATABASE_URL_FILE", "does not hold a postgres:// or postgresql:// URL")
	}
	c.DatabaseURL = secret(dsn)

	switch rs := get("ROUTE_SOURCE"); {
	case rs == "hub":
		c.RouteSource = rs
		if c.HubURL = get("HUB_URL"); wire.CheckBaseURL(c.HubURL) != nil {
			return bad("HUB_URL", "must be the Hub's http(s) base URL when ROUTE_SOURCE is hub")
		}
		if c.HubScope = get("HUB_SCOPE"); c.HubScope == "" {
			c.HubScope = "panacea-app-hub"
		}
		if strings.ContainsAny(c.HubScope, ":/ ") { // an Entra api://…/.default is refused (OD-30)
			return bad("HUB_SCOPE", "must be the realm's Hub audience scope (panacea-app-hub) when ROUTE_SOURCE is hub")
		}
	case strings.HasPrefix(rs, "file:") && len(rs) > len("file:"):
		c.RouteSource = rs
	default:
		return bad("ROUTE_SOURCE", `must be "hub" or "file:<path>"`)
	}

	for _, mode := range strings.Split(get("CALLER_AUTH"), ",") { // decision (g)
		mode = strings.TrimSpace(mode)
		if mode == "oidc" && c.OIDC == nil {
			if c.OIDC, err = loadOIDC(getenv); err != nil {
				return Config{}, err
			}
			continue
		}
		if c.CallerMode != "" || mode == "oidc" {
			return bad("CALLER_AUTH", "names a mode twice: at most one of kube, kube:<path>, bearer-file:<path>, and oidc once")
		}
		switch {
		case mode == "kube":
			c.CallerMode = "kube"
			c.Callers, err = parseCallers([]byte(builtinKubeTable), false)
		case strings.HasPrefix(mode, "kube:"):
			c.CallerMode = "kube"
			c.Callers, err = readCallers(strings.TrimPrefix(mode, "kube:"), false)
		case strings.HasPrefix(mode, "bearer-file:"):
			c.CallerMode = "bearer"
			c.Callers, err = readCallers(strings.TrimPrefix(mode, "bearer-file:"), true)
		default:
			return bad("CALLER_AUTH", `must be a comma list of one of "kube", "kube:<path>", "bearer-file:<path>", and "oidc"`)
		}
		if err != nil {
			return bad("CALLER_AUTH", err.Error())
		}
	}

	if c.BaseURL = get("BASE_URL"); wire.CheckBaseURL(c.BaseURL) != nil {
		return bad("BASE_URL", "must be what a sandbox sets as ANTHROPIC_BASE_URL: http(s)://host[:port], no path")
	}
	durations := []struct {
		k   string
		dst *time.Duration
		def time.Duration
	}{
		{"TOKEN_TTL", &c.TokenTTL, time.Hour},
		{"MINT_TIMEOUT", &c.MintTimeout, 20 * time.Second},
		{"ROUTE_CACHE_TTL", &c.RouteCacheTTL, 30 * time.Second},
		{"ROUTE_STALE_MAX", &c.RouteStaleMax, 10 * time.Minute},
	}
	for _, f := range durations {
		*f.dst = f.def
		if v := get(f.k); v != "" {
			d, perr := time.ParseDuration(v)
			if perr != nil || d <= 0 {
				return bad(f.k, fmt.Sprintf("must be a positive duration such as %v", f.def))
			}
			*f.dst = d
		}
	}
	if c.TokenTTL <= refreshGrace {
		return bad("TOKEN_TTL", "must be longer than the five-minute refresh grace")
	}
	if c.RouteStaleMax < c.RouteCacheTTL {
		return bad("ROUTE_STALE_MAX", "must be at least ROUTE_CACHE_TTL")
	}
```

```go
	// Budgets are on by default (OD-24): an unset figure is budget.Proposed's,
	// and zero, negative, NaN or infinite is refused, never "unlimited".
	c.Limits = budget.Proposed
	budgets := []struct {
		k   string
		dst *float64
	}{
		{"SESSION_CAP_USD", &c.Limits.SessionCapUSD},
		{"DEFAULT_USER_BUDGET_USD", &c.Limits.PersonUSD},
		{"DEFAULT_TEAM_BUDGET_USD", &c.Limits.TeamUSD},
		{"DEFAULT_PROJECT_BUDGET_USD", &c.Limits.ProjectUSD},
	}
	for _, f := range budgets {
		if v := get(f.k); v != "" {
			x, perr := strconv.ParseFloat(v, 64)
			if perr != nil || !(x > 0) || math.IsInf(x, 1) {
				return bad(f.k, "must be a positive number of US dollars a month; budgets cannot be switched off")
			}
			*f.dst = x
		}
	}
	return c, nil
}

// readSecretFile returns the trimmed content, or why it cannot be used. The
// reason never contains the content.
func readSecretFile(path string) (string, string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "names a file that cannot be read"
	}
	v := strings.TrimSpace(string(data))
	if v == "" {
		return "", "names an empty file"
	}
	return v, ""
}
```

**[08b]** Replace `control-plane/cmd/keysmith/main.go` (08a's placeholder) whole:

```go
// Command keysmith mints, refreshes and revokes the per-session LiteLLM keys
// and serves their bindings and the usage rollup (phase 6, P6-D6 to P6-D8).
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/binding"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/budget"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/catalogue"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/wire"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/keysmith"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/litellmadmin"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/routesource"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/usage"
)

const listenAddr = ":4100"

// errConfig marks a refusal found after loadConfig, which also exits 2.
var errConfig = errors.New("configuration")

func main() { os.Exit(run(os.Getenv, os.Stderr)) }

// run: 2 is a configuration refusal, 1 a runtime failure, 0 a clean signal
// (phase 2's convention, control-plane/cmd/api/main.go:45-47; decision (a)).
func run(getenv func(string) string, errOut io.Writer) int {
	cfg, err := loadConfig(getenv)
	if err != nil {
		fmt.Fprintf(errOut, "keysmith: %v\n", err)
		return 2
	}
	log := slog.New(slog.NewJSONHandler(errOut, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := serve(ctx, cfg, log); err != nil {
		log.Error("keysmith stopped", "err", err.Error())
		if errors.Is(err, errConfig) {
			return 2
		}
		return 1
	}
	return 0
}
```

```go
// serve wires tasks 02, 04, 05, 06, 07, 09 and 13 into one listener. Every
// refusal it finds is errConfig, so it exits 2 like loadConfig's.
func serve(ctx context.Context, cfg Config, log *slog.Logger) error {
	admin, err := litellmadmin.New(cfg.LiteLLMURL, cfg.MasterKeyFile, litellmadmin.Options{})
	if err != nil {
		return fmt.Errorf("%w: %sMASTER_KEY_FILE: %v", errConfig, envPrefix, err)
	}
	cat, err := catalogue.Load()
	if err != nil {
		return fmt.Errorf("%w: the model catalogue: %v", errConfig, err)
	}
	pool, err := pgxpool.New(ctx, string(cfg.DatabaseURL))
	if err != nil { // pgx may quote the string it could not parse; never shown
		return fmt.Errorf("%w: %sDATABASE_URL_FILE: the database URL is not usable", errConfig, envPrefix)
	}
	defer pool.Close()
	if err := binding.Migrate(ctx, pool); err != nil {
		return fmt.Errorf("migrating the keysmith schema: %w", err)
	}
	store := binding.NewPGStore(pool)
	static, err := budget.NewStatic(cfg.Limits)
	if err != nil {
		return fmt.Errorf("%w: budgets: %v", errConfig, err)
	}
	policy := budget.Layered{Fallback: static} // the Hub primary is ask H-2, seam S6.2
	routes, err := routeSource(cfg)
	if err != nil {
		return fmt.Errorf("%w: %sROUTE_SOURCE: %v", errConfig, envPrefix, err)
	}
	auth, err := authenticator(cfg)
	if err != nil {
		return fmt.Errorf("%w: %sCALLER_AUTH: %v", errConfig, envPrefix, err)
	}
	report := func(err error) { log.Warn("best-effort step failed", "err", err.Error()) }
	spend := usage.NewPGStore(pool)
	reader := usage.NewReader(spend, policy, nil)
	// task 09 decisions (c) and (g): Prices and Groups turn on the check that
	// LiteLLM prices every group above zero (review I-5).
	go (&usage.Sweeper{Keys: spend, Admin: admin, Store: spend, Report: report,
		Prices: admin, Groups: cat.Groups()}).Run(ctx)
	svc, err := keysmith.New(keysmith.Deps{Admin: admin, Store: store, Budgets: policy, Spent: reader,
		Routes: routes, Groups: cat.Groups(), TTL: cfg.TokenTTL, BaseURL: cfg.BaseURL, Report: report})
	if err != nil {
		return fmt.Errorf("%w: %v", errConfig, err)
	}
	// Bindings is the Service, not the store: a lookup of a raised project's
	// Global-minted key retires it (task 07 decision (h), review C-1).
	h := newAPI(apiDeps{Keys: svc, Bindings: svc, Usage: reader, Auth: auth, Sessions: store, MintWait: cfg.MintWait(), Log: log,
		Ready: func(ctx context.Context) error { // fixed sentences only: the probe never shows an error chain
			if pool.Ping(ctx) != nil {
				return errors.New("database unreachable")
			}
			if admin.Health(ctx) != nil {
				return errors.New("LiteLLM unreachable")
			}
			return nil
		}})
	srv := &http.Server{Addr: listenAddr, Handler: h, ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout: 30 * time.Second, WriteTimeout: cfg.MintWait() + 10*time.Second, IdleTimeout: 120 * time.Second}
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	log.Info("keysmith listening", "addr", listenAddr, "config", cfg.String())
	select {
	case <-ctx.Done():
		shut, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shut)
	case err := <-errc:
		return err
	}
}

func routeSource(cfg Config) (wire.RouteSource, error) {
	var inner wire.RouteSource
	if path, ok := strings.CutPrefix(cfg.RouteSource, "file:"); ok {
		f, err := routesource.NewFile(path)
		if err != nil {
			return nil, err
		}
		inner = f
	} else {
		tok, err := routesource.ClientCredentialsFromEnv(cfg.HubScope) // the realm, as panacea-app-gateway (OD-30)
		if err != nil {
			return nil, err
		}
		hub, err := routesource.NewHub(cfg.HubURL, tok, nil)
		if err != nil {
			return nil, err
		}
		inner = hub
	}
	c, err := routesource.NewCache(inner, cfg.RouteCacheTTL, cfg.RouteStaleMax, nil)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// authenticator is CALLER_AUTH's comma list as one chainAuth (decision (g)).
func authenticator(cfg Config) (Authenticator, error) {
	var a chainAuth
	switch cfg.CallerMode {
	case "bearer":
		a.table = bearerAuthFrom(cfg.Callers)
	case "kube":
		restCfg, err := rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("kube caller authentication needs the in-cluster service account: %v", err)
		}
		cs, err := kubernetes.NewForConfig(restCfg)
		if err != nil {
			return nil, err
		}
		a.table = newKubeAuth(cs.AuthenticationV1().TokenReviews(), cfg.Callers, nil)
	}
	if cfg.OIDC != nil {
		o, err := newOIDCAuth(*cfg.OIDC)
		if err != nil {
			return nil, fmt.Errorf("oidc caller authentication: %v", err)
		}
		a.issuer, a.oidc = cfg.OIDC.Issuer, o
	}
	return a, nil
}
```

- [ ] **Step 5: Run the tests to verify they pass**

```bash
cd control-plane && go test ./cmd/keysmith/ -count=1 -race -v > /tmp/ks-green.txt 2>&1; echo "exit $?"
grep -c '^--- PASS' /tmp/ks-green.txt; grep -c '^--- FAIL' /tmp/ks-green.txt
```

Expected: `exit 0`, then **[08a]** `6` and `0`; **[08b]** `17` and `0` (08a's six still pass beside 08b's eleven).

- [ ] **Step 6: Prove the mutants die**

Apply each edit alone, run `go test ./cmd/keysmith/ -count=1 > /tmp/ks-mut.txt 2>&1; echo "exit $?"`, confirm a non-zero exit and that the named test is the one failing in the file, and revert. Each task runs only the rows marked with its id: **[08a]** four, **[08b]** nine.

| Task | Edit | Test that must fail |
|---|---|---|
| 08b | `api.go`, in `guard`: delete the `case !c.Routes[route]:` branch | `TestEveryRouteRefusesACallerNotListedForIt` |
| 08b | `api.go`, in `await`: `ctx := context.WithoutCancel(r.Context())` → `ctx := r.Context()` | `TestMintTimeoutAnswers504AndTheMintCompletes` |
| 08b | `api.go`, in `fail`: the `ErrAdmin`/`ErrStore` sentence → `err.Error()` | `TestServiceErrorsBecomeProblems` |
| 08b | `api.go`, in `binding`: the `wire.ErrNotFound` case → `http.StatusUnauthorized` | `TestBindingsRoute` |
| 08a | `callerauth.go`, in `kubeAuth.Identify`: delete `\|\| !slices.Contains(tr.Status.Audiences, kubeAudience)` | `TestKubeAuthChecksAudienceTableAndCache` |
| 08a | `callerauth.go`, in `parseCallers`: the short-token message gains `+ ": " + cred` | `TestCallerTableRefusesBadLinesWithoutQuotingThem` |
| 08b | `config.go`: `if !strings.HasPrefix(key, "sk-")` → `if false` | `TestConfigRefusesNamingTheVariable` |
| 08b | `main.go`, in `run`: `return 2` → `return 1` | `TestRunRefusesAMissingOrEmptyMasterKey` |
| 08a | `calleroidc.go`, in `oidcAuth.Identify`: delete `cl.Service,` from the refusal case | `TestOIDCAuthNamesThePersonOnly` |
| 08a | `calleroidc.go`, in `chainAuth.Identify`: the oidc case → `if c, err := a.oidc.Identify(r); err == nil { return c, nil }` falling on to the table | `TestChainAuthChoosesByIssuerAndNeverFallsThrough` |
| 08b | `owner.go`, in `ownerMayAct`: delete the `body != nil && *body != *c.Owner` check | `TestAPIRefusesAPersonActingForAnotherOwner` |
| 08b | `owner.go`, in `ownerMayAct`: delete the `for _, b := range bs` loop | `TestAPIRefusesAPersonActingForAnotherOwner` |
| 08b | `config.go`: `if c.CallerMode != "" \|\| mode == "oidc"` → `if mode == "oidc"` | `TestConfigCallerAuthIsACommaList` |

- [ ] **Step 7: Vet, format, build, and prove the binary refuses**

```bash
cd control-plane
go vet ./cmd/keysmith/ && gofmt -l cmd/keysmith/            # expect no output
go build -o /tmp/keysmith ./cmd/keysmith/ > /tmp/ks-build.txt 2>&1; echo "build exit $?"
/tmp/keysmith 2> /tmp/ks-run.txt; echo "exit $?"; cat /tmp/ks-run.txt
```

Expected: `build exit 0`, then one stderr line and `exit 2`. **[08a]**: `keysmith: refusing to start: the API is not wired (task 08b)`. **[08b]**: `keysmith: PANACEA_APP_GATEWAY_MASTER_KEY_FILE is not set; keysmith does not start without LiteLLM's master key`. No listener opened in either.

- [ ] **Step 8: Commit**

**[08a]**

```bash
git add control-plane/cmd/keysmith/callerauth.go control-plane/cmd/keysmith/calleroidc.go control-plane/cmd/keysmith/main.go \
  control-plane/cmd/keysmith/callerauth_test.go control-plane/cmd/keysmith/calleroidc_test.go
git commit -F <message file> -- control-plane/cmd/keysmith/
```

Message: `feat(keysmith): caller authentication, the table, TokenReview and the realm token (phase 6 task 08a)`, a body naming decisions (b), (e) and (g)'s authenticator half, and the attribution line.

**[08b]**

```bash
git add control-plane/cmd/keysmith/owner.go control-plane/cmd/keysmith/config.go control-plane/cmd/keysmith/api.go \
  control-plane/cmd/keysmith/main_test.go control-plane/cmd/keysmith/config_test.go \
  control-plane/cmd/keysmith/api_test.go control-plane/cmd/keysmith/owner_test.go
git commit -F <message file> -- control-plane/cmd/keysmith/
```

Message: `feat(keysmith): the HTTP API, config and a fail-closed start (phase 6 task 08b)`, a body naming decisions (a) to (g), and the attribution line. The pathspec also takes the replaced `main.go`.

**Acceptance:** **[08a]** step 5 prints `6` under `-race`, step 6's four 08a rows each fail the named test, and step 7 prints the placeholder's refusal and `exit 2`. **[08b]** step 5 prints `17`, step 6's nine 08b rows each fail the named test, and step 7 prints the master-key refusal and `exit 2`. The live path (a mint through the compose stack answering `201`, then a `POST /v1/messages` through the gatekeeper) is task 17's acceptance, which runs this binary.

**What this task does not claim:**

- **That `TokenReview` works as written in AKS.** The fake clientset proves the request shape and the audience check; the real API server's audience handling, and keysmith's RBAC to create reviews, are **UNVERIFIED** until task 19's chart is applied (never in this phase) and task 24's P02 runs.
- **The broker's side of the call** (ask P8-1): its service-account name, its projected-token audience, and when it calls.
- **Transport security.** keysmith listens in plain HTTP on the private network; phase 8's NetworkPolicies are the boundary, and mTLS is seam S6.10.
- **That a late mint's token reaches anyone.** A mint that finishes after its 504 is superseded by the caller's retry (task 07); its key is live until then, bound to the pod and unused.
- **That the usage route has a cloud caller.** Only the local control plane holds `usage` (task 17's table); a person's `oidc` token holds `mint`, `refresh` and `revoke` only.
- **That the realm issues what `oidc` expects.** The token's `oid` and `tid` claims, audience and scope `panacea-app-gateway`, and `azp` `panacea-app-desktop` are Panacea's realm settings (ruling §4 mappers, §10 asks), checked by hub task 43 against the dev realm; **UNVERIFIED** here, where the validator is a fake. `oidcauth` itself is env task 17 step 0's, not this task's.

**Gaps against the skeleton, for the orchestrator:**

1. **`binding_not_found` (404)** is not in task 03's code list. The gatekeeper needs a miss told apart from an outage; this part writes it as a local constant until task 03 adds it.
2. **Exit status `2`, not `1`,** for a configuration refusal (decision (a)). Resolved in fix batch b04 round 2: the skeleton's 08 summary now says `2`.
3. **No cloud caller holds `usage`.** Resolved in fix batch b05 (review I-9) as seam S6.11: the Hub shows a cloud project's spend and reads it here itself, so when asks H-5 and P8-6 are built, the built-in table gains `hub system:serviceaccount:panacea-hub:hub usage` (account name a proposal for P8-2) and `TestKubeBuiltInTable`'s `want` map gains `"hub": "usage"`. Not before: this phase grants `usage` to no identity that does not yet call it. Task 21's gap 5 points here. **And the grant is per route, never per scope:** the `kube` table authorises a caller for `usage`, so a holder may read any `scope`/`id` it asks for. Locally that holder is the one person at this PC. In the cloud it is the Hub, whose own callers are every viewer of every team, and only ask H-5's own rule — the person is the caller and never a value from the query — keeps one member's figure from another. Narrowing the grant to a scope is a change to this table and to task 07's key model that nobody has asked for yet; seam S6.11 carries it as a residual so phase H's implementer does not read this table as a second defence.
4. **Task 19 must grant keysmith `create` on `tokenreviews`** (a ClusterRoleBinding to `system:auth-delegator`, **UNVERIFIED** name) and set the broker's and gatekeeper's projected tokens to audience `keysmith`. The service-account names `panacea-broker:broker` and `panacea-gateway:gatekeeper` are proposals for ask P8-2.
5. **Identifiers assumed from parts not yet merged:** task 04's `litellmadmin.New`, `Options`, `ErrNoMasterKey`, `(*Client).Health`; task 05's `binding.Migrate`, `binding.NewPGStore` (now in `task-05.md`). Task 09's `*usage.PGStore` is the sweep's `KeySource` (task 09, decision (c)), so task 05's `Store` needs no new method. Step 1 checks them.
6. **`PANACEA_APP_GATEWAY_HUB_SCOPE`** is read here as task 13's gap 1 proposes; it is not in the skeleton's shared names. Since `OD-30` it is the realm audience scope, default `panacea-app-hub`, and an Entra `api://…/.default` is refused.
7. **`OD-30` adds `PANACEA_APP_GATEWAY_TENANTS`, `_AUDIENCE` and `_ALLOWED_CLIENTS`**, and reads the shared `PANACEA_APP_OIDC_ISSUER` and `_PROFILE` (ruling §2), none in the skeleton's shared names. Task 19's chart sets `CALLER_AUTH=kube,oidc` and the three; hub task 10 waits on env 17 step 0 for `oidcauth`, and so does task 08a (step 1's **[08a]** block); 08b inherits the wait through 08a.
8. **`apiDeps.Bindings` is the `*keysmith.Service`, not the store** (fix batch b01, review C-1). `ByKeyHash` retires a raised project's Global-minted key as the gatekeeper looks it up (task 07 decision (h)), and task 07's tests prove that behaviour. The wiring is one line in `run`, and no test here catches a revert to `Bindings: store`. Such a revert would leave a Global-minted key its `global/*` groups until its scheduled refresh, and nothing would fail. Check the line in review; an end-to-end raise is task 23's to observe, if the owner wants it proven.
