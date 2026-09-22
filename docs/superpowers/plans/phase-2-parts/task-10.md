### Task 10: The router and the read-only surface

This is the first task in the phase that produces an HTTP surface a human can curl, and it is the spine every later task hangs off. It owns two things that look unrelated and are not: the **router** — `Deps`, `New`, the mux, the `/api/v1` alias — and the **six read-only endpoints** that need nothing but a git enumeration and a catalogue. They are one task because the read-only surface is the only thing that can prove the router works without a Docker daemon, a session record or a WebSocket. Contract §1.2 correction 1 is the reason `api.go` is not one file: three tasks write into this package, and one file guarantees tasks 10, 11 and 12 collide in the same region. `api.go` owns wiring and nothing else; `catalog.go` owns handlers and nothing else; tasks 11 and 12 add `sessions.go` and `attach.go` beside them and touch `api.go` for exactly one line each.

**The worktree `ready` flag ships here and it is the whole of the fix for known defect 9.** `docs/FUNCTIONAL-SPEC.md` lists "nothing ever sets `ready`, so user-created worktrees can never merge" among the ten defects this pass fixes, and the plan's adjudicated Cross-group decision 2 keeps the verb and makes it work: `GET /v1/projects/{projectId}/worktrees` reports `"ready": false` on every worktree, and `PATCH /v1/projects/{projectId}/worktrees/{worktreeId}` accepts `{"ready": true}` and answers 200 with the worktree object, 400 for any other body and 404 `worktree_not_found` for an unknown id. **Neither endpoint goes through `registry.Store`.** Task 7 put the flags behind the narrow `registry.WorktreeFlagStore` — `WorktreeFlagsSnapshot()` and `SetWorktreeReady()` — precisely so `Store` stays session-only and five-method, as contract §2.8 declares it. Reaching for `Store.Update` here would widen an interface task 11 is already written against. The `PATCH` route is **not** in contract §3.1's table; it is the adjudicated decision's addition and is cited as such below.

The `/api/v1` alias is one line and it is load-bearing. Contract §7.5 resolves the prefix discrepancy in favour of `/v1` as canonical with `mux.Handle("/api/v1/", http.StripPrefix("/api", mux))` as an alias, so Phase 4's Discover routes and its entire test suite land unchanged. **Do not delete it as dead code** — nothing in Phase 2 calls it, and that is the point. It cannot recurse: `/api/v1/machines` strips to `/v1/machines`, which does not match `/api/v1/`, and `/api/api/v1/machines` matches no pattern at all and 404s.

Six decisions where the contract is silent or inconsistent, made here so no later task re-derives them. **(a) `/v1/healthz` is mounted on an outer mux, outside the origin middleware.** §3.1 says it is "unauthenticated, no origin check, for a supervisor", and a supervisor that addresses the control plane by a container name rather than `127.0.0.1` would otherwise get 403 `forbidden_host` and restart a process that is perfectly healthy. Bypassing the guard leaks nothing: the body is a status, a version, a backend name and a boolean, and the listener is loopback-bound by task 13. **(b) But healthz still echoes an allowed `Origin`.** Skipping the middleware also skips its CORS grant, and a cockpit health dot that reads red while `curl` reads green is a bug nobody debugs twice; `api.go` therefore applies the grant half of adjudicated decision 1 — `Vary: Origin` always, `Access-Control-Allow-Origin` echoed only when `OriginPolicy.Allow` says yes — and never the rejection half. **(c) `backendReady` comes from a `func() bool` on `Deps`, not from the backend.** §3.3 calls it "a cached result of the backend's `Ping`", but `backend.Backend` (§2.2) has no `Ping` and this task may not add one to a locked interface. Task 13's reaper already sweeps on a timer and is the natural owner of the cached bool; `Deps.BackendReady` is the seam, and `New` **rejects a nil one** rather than defaulting it, because both defaults are wrong — `true` fabricates health and `false` sends an operator chasing a phantom. **(d) There is no generic 404 handler.** The sixteen error codes `httpx` declares — §3.2's fifteen plus the `non_loopback_publish` that Group C's ruling 7 adds — are a closed enum the desktop switches on and none of them means "no such route", so an unrouted path gets `net/http`'s own plain-text 404 rather than a seventeenth code invented here. A test pins that, so the choice is visible rather than accidental. **(e) `sessionCount` is computed from one `Store.List()` and one flag snapshot per request**, not per worktree. §8.8 flags the per-worktree scan as the thing that stops being free at hundreds of sessions; a map built once is O(n + m) for the whole response and costs nothing to write. **(f) A `Store.List()` failure is a 500, not a zero count.** Reporting `sessionCount: 0` for a registry that could not be read is a fabricated number, and this phase rules those out.

The tests use a **real `*registry.FileStore`** in `t.TempDir()` for both `Deps.Store` and `Deps.Flags`, not a stand-in. One object satisfies both interfaces, the fixture is three lines, and — this matters — it means **task 10 introduces no substitute for `registry.Store` and therefore owes nothing to `registrytest.RunStoreContract`**. Task 11 does introduce one, and its plan must run that suite against it. Three doubles live here and every one carries the `catalog` prefix the Group C package-API contract's ruling 5 fixes, so nothing in this package can collide with task 11's `sessions*` or task 12's `attach*`: **`catalogProjects`**, an `api.Enumerator` over fixed slices, because the alternative is a git repository per test; **`catalogBackend`**, which answers `Kind()` and panics on anything else; and **`catalogBrokenList`**, a real `*registry.FileStore` with `List` alone poisoned. That last one is the only way to reach decision (f): `FileStore.List` reads an in-memory index and cannot fail on its own, so without it the 500 the decision fixes is unreachable. It is still not a substitute store — `registrytest.RunStoreContract` has nothing to say about a store deliberately broken in one method, and the store underneath it is the real one. Two traps carried forward from task 9: every test request uses an **absolute** target (`"http://127.0.0.1:8730/v1/machines"`), because `httptest.NewRequest` otherwise sets `Host: example.com` and the guard answers 403 `forbidden_host` in a way that reads as a routing bug; and both files are `package api_test`, so the compiler names missing symbols as `api.New` rather than `New`.

**Files:**
- `control-plane/internal/api/api.go` — `Deps`, `Enumerator`, `API`, `New`, the guarded and outer muxes, the `/api/v1` alias, the healthz CORS grant, the shared 500 helper (new)
- `control-plane/internal/api/api_test.go` — the router contract, `New`'s dependency validation, and the test doubles both files share (new)
- `control-plane/internal/api/catalog.go` — `/healthz`, `/machines`, `/projects`, `/branches`, `/worktrees`, `PATCH .../worktrees/{worktreeId}`, `/harnesses` (new)
- `control-plane/internal/api/catalog_test.go` — every body above, byte-exact wherever the desktop's TypeScript parses it (new)
- `control-plane/internal/api/export_test.go` — `(*API).RegisteredPatterns()`, the test-only window onto the recorded route table (new, added by ruling 1)
- `control-plane/internal/api/routes_audit_test.go` — contract §10a's audit: the two allowlists, `catalogAuditRoutes`, and the scan that stops a file registering behind the recorder (new, added by ruling 1). **Tasks 11 and 12 extend `catalogReadOnlyRoutes` here**; this is the file to look in.

Six files and not four: ruling 1 added the last two after this plan was first written. They are transcribed in step 1 like everything else, and the reason is the failure they prevent. A package built from the other four compiles, serves every route correctly and has **no §10a audit at all** — and nothing downstream notices, because a verifier that rebuilds from a plan to compile-check is testing that the code builds, not that the plan is complete.

**Interfaces:**
- Produces:
  - `api.Deps` — `Version`, `Backend`, `BackendReady`, `Store`, `Flags`, `Projects`, `Worktrees`, `Origin`, `Log`. A struct, so tasks 11 to 13 add fields without changing a signature. **These spellings are fixed** by `.superpowers/sdd/2026-09-18-phase-2-backend-and-cockpit/group-c-package-api-contract.md` ruling 1, which this task is the authority for: the field is `Origin` and not `Origins`, the receiver is `*API` and there is no `Server` type in this package, and `New` returns two values
  - `api.Enumerator` — the three read-only methods of `project.Scanner`, as an interface, so a handler test needs no git
  - `api.New(Deps) (*API, error)` and `(*API).ServeHTTP` — the whole HTTP surface as one `http.Handler`, consumed by task 13's `cmd/api/main.go`. Two return values, and an error rather than a panic: a test that expects `New` to panic expects the wrong thing
  - `api.DefaultVersion` (`"0.2.0"`), `api.LocalMachineID` (`"local"`)
  - `(*API).registerCatalog(*routes)` and `(*API).internal(w, r, op, err)` — unexported, but named here because task 11 and task 12 each add one `a.registerX(guarded)` line to `New` and both use `internal` for their own 500s
  - `isolationFor(backend.Kind) string` — unexported, and the package's **only** mapping of a backend kind to contract §3.4's three-value enum. Group C ruling 6: task 11 calls this and declares no second one, because two functions mapping one enum in one package is a defect even when they agree
- Consumes:
  - task 9 — `httpx.WriteJSON`, `httpx.WriteProblem`, `httpx.DecodeJSON`, `httpx.OriginPolicy` with `Middleware` and `Allow`, `httpx.NewOriginPolicy` (tests only), and `httpx.CodeBadRequest`, `CodeProjectNotFound`, `CodeWorktreeNotFound`, `CodeInternal`
  - task 8 — `harness.Catalogue()`, `harness.Descriptor` with `Available()`, `harness.Model`, `harness.Effort` and `harness.PermissionMode` (both only as the type argument to `stringsOf`), and `harness.ID` with all five constants — `ClaudeCode`, `Codex`, `OpenCode`, `GeminiCLI`, `Shell` (tests only); `project.Project`, `project.Branch`, `project.Worktree`, `project.ErrUnknownProject` (matched with `errors.Is`, never by string); `worktree.Provider`, and `worktree.Static` (tests only)
  - task 7 — `registry.Store.List()`, `registry.Session` with `WorktreeID` and `Status`, `registry.Status` — `StatusExited` in the handler, `StatusStarting`/`StatusRunning`/`StatusIdle` in the tests that prove they still count — `registry.WorktreeFlagStore` with `WorktreeFlagsSnapshot()` and `SetWorktreeReady()`, `registry.WorktreeFlags.Ready`, and `registry.NewFileStore` returning `*registry.FileStore` (tests only)
  - task 4 — `backend.Backend`, `backend.Backend.Kind()`, `backend.Kind`, `backend.KindDocker`, `backend.KindKata`
- Requires of later tasks, every item of it a Group C ruling this task is the authority for:
  - **Task 11** adds `a.registerSessions(guarded)` to `New` **in the same step that defines `registerSessions`** — ruling 2; adding the call in an earlier step leaves `api.go` uncompilable for the steps between, so no suite in them can be green. It adds `Image string` to `Deps` itself, in the step that first uses it — ruling 3; the "one line each; no other edit to this file" below is about route registration and does not forbid that field. It calls this file's `isolationFor` rather than declaring its own — ruling 6. And it must run `registrytest.RunStoreContract` against any `Store` double it introduces.
  - **Task 12** adds `a.registerAttach(guarded)` to `New` — ruling 2. Task 10 does not register the attach route; a task 12 that assumes otherwise ships a route the product never serves behind a fully green suite.
  - **Both 11 and 12** must pin their route with a test that drives it through `New`'s handler rather than through a mux the test assembled itself. That is the test whose absence let the missing registration through.
  - **Task 13** supplies `Deps.BackendReady` from the reaper's cached probe and wires `*project.Scanner` into both `Deps.Projects` and `Deps.Worktrees`. It passes `Image:`. It does **not** pass `IdleAfter` — ruling 4 puts the idle window on `reaper.Config`, because the reaper derives idle on its sweep and the API never computes it.
- Amends contract §3.1: adds `PATCH /v1/projects/{projectId}/worktrees/{worktreeId}`, per the plan's adjudicated Cross-group decision 2. Departs from that decision on one detail — see step 5.

- [ ] **Step 1: Write the failing router tests and the shared doubles**

This file pins the router and nothing about a body. Three of its tests would pass by accident against a mux that routed everything to one handler, so they check the wiring instead: that the `/api/v1` alias reaches the same handler as `/v1`, that a wrong method produces 405 with an `Allow` header rather than 404, and that the origin middleware is wrapped around every route except healthz. The doubles live here because `catalog_test.go` is in the same directory and the same package, so both files see them.

`catalogProjects` returns `project.ErrUnknownProject` wrapped with `%w` for an id it does not hold, exactly as `project.Scanner` does — a double that returns a bare `errors.New` would let a handler that string-matches pass. Its `err` field is what step 2 uses to reach `a.internal`, the only 500 on this surface, and `catalogBrokenList` beside it is what reaches decision (f)'s. **Every** package-level identifier this task's test files declare begins with `catalog`, per Group C ruling 5 — the doubles `catalogBackend`, `catalogBrokenList` and `catalogProjects`, the request harness `catalogBaseURL`, `catalogDo`, `catalogNewDeps`, `catalogNewAPI`, `catalogNewAPIWithSessions`, `catalogQuietLogger`, `catalogSampleProjects`, `catalogDecodeObject`, `catalogListOf` and the five `catalogTestProject`-style id constants, and equally the audit's `catalogReadOnlyRoutes`, `catalogMethodAgnosticRoutes` and `catalogAuditRoutes`. The rule is not about doubles; it is about three tasks writing test files into one directory, and a bare `newAPI` or `listOf` is exactly the generic name task 11 or task 12 would reach for without thinking. `TestXxx` functions are the one exception, because Go's own naming keeps them apart and a reader expects to read them as sentences.

Create `control-plane/internal/api/api_test.go`:

```go
package api_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/api"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/project"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/worktree"
)

// catalogBaseURL is the absolute prefix every request in this package uses. A
// path-only target makes httptest.NewRequest set Host to "example.com", which
// the loopback guard answers 403 forbidden_host — and that reads as a routing
// bug for as long as it takes to remember this.
const catalogBaseURL = "http://127.0.0.1:8730"

// catalogBackend answers Kind and nothing else. Embedding the interface supplies
// the other six methods; calling one panics with a nil dereference, which is
// the right outcome — no handler in this task may reach the daemon.
type catalogBackend struct {
	backend.Backend
	kind backend.Kind
}

func (s catalogBackend) Kind() backend.Kind {
	if s.kind == "" {
		return backend.KindDocker
	}
	return s.kind
}

// catalogBrokenList is a real *registry.FileStore with List alone poisoned. It
// is not a substitute for the store and owes registrytest.RunStoreContract
// nothing: the four other methods ARE the real store, and a suite that checks
// a store honours its contract has nothing to say about one deliberately
// broken in a single method. It exists because *FileStore.List reads an
// in-memory index and cannot fail on its own, so decision (f) — a List failure
// is a 500, never a zero count — is otherwise unreachable from a test.
type catalogBrokenList struct {
	*registry.FileStore
	err error
}

func (c catalogBrokenList) List() ([]registry.Session, error) { return nil, c.err }

// catalogProjects is an api.Enumerator over fixed slices. It wraps
// project.ErrUnknownProject with %w for an unknown id, exactly as the real
// Scanner does, so a handler that string-matched the message would still fail.
type catalogProjects struct {
	projects  []project.Project
	branches  map[string][]project.Branch
	worktrees map[string][]project.Worktree
	err       error // when non-nil, every method returns it
}

func (f *catalogProjects) Projects(context.Context) ([]project.Project, error) {
	return f.projects, f.err
}

func (f *catalogProjects) Branches(_ context.Context, projectID string) ([]project.Branch, error) {
	if f.err != nil {
		return nil, f.err
	}
	b, ok := f.branches[projectID]
	if !ok {
		return nil, fmt.Errorf("%w: %q", project.ErrUnknownProject, projectID)
	}
	return b, nil
}

func (f *catalogProjects) Worktrees(_ context.Context, projectID string) ([]project.Worktree, error) {
	if f.err != nil {
		return nil, f.err
	}
	w, ok := f.worktrees[projectID]
	if !ok {
		return nil, fmt.Errorf("%w: %q", project.ErrUnknownProject, projectID)
	}
	return w, nil
}

// catalogQuietLogger keeps a failing test's output readable. A handler that logs on
// the happy path is a handler that will be found by grepping for noise.
func catalogQuietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
```

Still `control-plane/internal/api/api_test.go`, continuing below `catalogQuietLogger` — do not repeat the `package` clause or the imports. The ids in these fixtures are the ones contract §3.5 to §3.7 use, so a body assertion in `catalog_test.go` can be compared against the contract by eye:

```go
const (
	catalogTestProject   = "p-8c1d0e42"
	catalogTestBranch    = "b-8c1d0e42-evidence-upload"
	catalogTestWorktree  = "w-4a77b1c9"
	catalogOtherBranch   = "b-8c1d0e42-main"
	catalogOtherWorktree = "w-1111aaaa"
)

// catalogSampleProjects is the enumerator every test starts from. A test that needs a
// different shape builds its own rather than mutating this one.
func catalogSampleProjects() *catalogProjects {
	return &catalogProjects{
		projects: []project.Project{
			{ID: catalogTestProject, Name: "engagement-portal", Path: `C:\work\engagement-portal`},
		},
		branches: map[string][]project.Branch{
			catalogTestProject: {
				{ID: catalogOtherBranch, ProjectID: catalogTestProject, Name: "main", IsHead: false},
				{ID: catalogTestBranch, ProjectID: catalogTestProject, Name: "evidence-upload", IsHead: true},
			},
		},
		worktrees: map[string][]project.Worktree{
			catalogTestProject: {
				{ID: catalogOtherWorktree, ProjectID: catalogTestProject, BranchID: catalogOtherBranch,
					Name: "engagement-portal", Path: `C:\work\engagement-portal`, IsPrimary: true},
				{ID: catalogTestWorktree, ProjectID: catalogTestProject, BranchID: catalogTestBranch,
					Name: "evidence-upload", Path: `C:\work\engagement-portal-evidence-upload`},
			},
		},
	}
}

// catalogNewDeps builds a Deps every field of which is real or harmless. Store and
// Flags are the SAME *registry.FileStore: it implements both interfaces, so
// there is no Store double in this task and nothing here owes
// registrytest.RunStoreContract anything. Task 11's double does.
func catalogNewDeps(t *testing.T) (api.Deps, *registry.FileStore) {
	t.Helper()
	st, err := registry.NewFileStore(filepath.Join(t.TempDir(), "sessions.json"))
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	log := catalogQuietLogger()
	return api.Deps{
		Version:      "0.2.0",
		Backend:      catalogBackend{},
		BackendReady: func() bool { return true },
		Store:        st,
		Flags:        st,
		Projects:     catalogSampleProjects(),
		Worktrees:    worktree.Static{},
		Origin:       httpx.NewOriginPolicy(nil, log),
		Log:          log,
	}, st
}

// catalogNewAPI applies mutate to the default Deps and builds the handler.
func catalogNewAPI(t *testing.T, mutate func(*api.Deps)) http.Handler {
	t.Helper()
	d, _ := catalogNewDeps(t)
	if mutate != nil {
		mutate(&d)
	}
	a, err := api.New(d)
	if err != nil {
		t.Fatalf("api.New: %v", err)
	}
	return a
}

// catalogDo issues one request against h. target is appended to
// catalogBaseURL, so no caller can forget the absolute form.
func catalogDo(t *testing.T, h http.Handler, method, target, body string, header map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, catalogBaseURL+target, nil)
	} else {
		r = httptest.NewRequest(method, catalogBaseURL+target, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	}
	for k, v := range header {
		if k == "Host" {
			r.Host = v
			continue
		}
		r.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}
```

Still `control-plane/internal/api/api_test.go`, continuing below `catalogDo`. The origin tests configure the allowlist explicitly rather than relying on `httpx.DefaultOrigins()`, so this task's tests do not break the day task 9's default list grows a sixth entry:

```go
func TestEveryContractRouteIsRegistered(t *testing.T) {
	// Contract 3.1's table, restricted to the rows this task owns, plus the
	// PATCH from adjudicated decision 2. A 404 here means a missing
	// registration; a 405 means the method in the pattern is wrong.
	//
	// Each row builds its own handler. Sharing one would let the PATCH row's
	// ready flag outlive it — harmless in this order, a trap for whoever adds
	// the next row or a t.Parallel.
	for _, tc := range []struct{ method, target, body string }{
		{http.MethodGet, "/v1/healthz", ""},
		{http.MethodGet, "/v1/machines", ""},
		{http.MethodGet, "/v1/projects", ""},
		{http.MethodGet, "/v1/projects/" + catalogTestProject + "/branches", ""},
		{http.MethodGet, "/v1/projects/" + catalogTestProject + "/worktrees", ""},
		{http.MethodPatch, "/v1/projects/" + catalogTestProject + "/worktrees/" + catalogTestWorktree, `{"ready":true}`},
		{http.MethodGet, "/v1/harnesses", ""},
	} {
		t.Run(tc.method+" "+tc.target, func(t *testing.T) {
			rec := catalogDo(t, catalogNewAPI(t, nil), tc.method, tc.target, tc.body, nil)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestTheAPIV1AliasReachesTheSameHandler(t *testing.T) {
	// Contract 7.5. Phase 4 writes every Discover route under /api/v1, so this
	// one line in New is the reason its plan and its tests land unchanged.
	h := catalogNewAPI(t, nil)
	for _, target := range []string{"/machines", "/projects", "/harnesses", "/healthz"} {
		t.Run(target, func(t *testing.T) {
			canonical := catalogDo(t, h, http.MethodGet, "/v1"+target, "", nil)
			alias := catalogDo(t, h, http.MethodGet, "/api/v1"+target, "", nil)
			if alias.Code != canonical.Code {
				t.Fatalf("/api/v1%s status = %d, /v1%s status = %d", target, alias.Code, target, canonical.Code)
			}
			if alias.Body.String() != canonical.Body.String() {
				t.Errorf("alias body differs:\n alias %s\n  /v1  %s", alias.Body.String(), canonical.Body.String())
			}
		})
	}
}

func TestTheAliasDoesNotMatchADoubledPrefix(t *testing.T) {
	// StripPrefix re-dispatches into the same mux, so the shape of the alias is
	// worth pinning: it strips exactly one /api and cannot loop.
	rec := catalogDo(t, catalogNewAPI(t, nil), http.MethodGet, "/api/api/v1/machines", "", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body = %s", rec.Code, rec.Body.String())
	}
}

func TestAWrongMethodIs405WithAnAllowHeader(t *testing.T) {
	// Go 1.22 ServeMux does this for free when every pattern on a path names a
	// method. The test exists because it stops being free the moment someone
	// registers a pattern without one.
	rec := catalogDo(t, catalogNewAPI(t, nil), http.MethodPost, "/v1/machines", `{}`, nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
	// "GET, HEAD" and not "GET". net/http's matchingMethodsPath adds HEAD to
	// the set whenever a GET pattern matches the path, because a HEAD is a GET
	// whose body is discarded, and server.go joins the sorted set into Allow.
	// It has been this way since Go 1.22, so asserting http.MethodGet here is
	// a red suite on every supported toolchain and not a wiring defect.
	if got, want := rec.Header().Get("Allow"), "GET, HEAD"; got != want {
		t.Errorf("Allow = %q, want %q", got, want)
	}
}

func TestAnUnroutedPathIsNotAProblemBody(t *testing.T) {
	// Decision (d): the sixteen codes httpx declares - contract 3.2's fifteen
	// plus ruling 7's non_loopback_publish - are a closed enum with no member
	// meaning "no such route", so this is net/http's own 404 and not an
	// invented seventeenth code. If a later task adds one, this test is the
	// place the decision gets re-opened.
	rec := catalogDo(t, catalogNewAPI(t, nil), http.MethodGet, "/v1/nope", "", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if strings.Contains(rec.Body.String(), `"error"`) {
		t.Errorf("body = %q, want net/http's plain-text 404", rec.Body.String())
	}
}
```

Still `control-plane/internal/api/api_test.go`, the last block of the file:

```go
func TestTheOriginGuardWrapsEveryRouteButHealthz(t *testing.T) {
	// The guard is the DNS-rebinding defence (task 9). It must sit in front of
	// the whole mux, and healthz must sit in front of IT, because a supervisor
	// addressing the control plane by a container name is not an attacker.
	h := catalogNewAPI(t, nil)

	guarded := catalogDo(t, h, http.MethodGet, "/v1/machines", "", map[string]string{"Host": "evil.example"})
	if guarded.Code != http.StatusForbidden {
		t.Fatalf("/v1/machines with a non-loopback Host = %d, want 403", guarded.Code)
	}
	if !strings.Contains(guarded.Body.String(), "forbidden_host") {
		t.Errorf("body = %q, want a forbidden_host Problem", guarded.Body.String())
	}

	health := catalogDo(t, h, http.MethodGet, "/v1/healthz", "", map[string]string{"Host": "evil.example"})
	if health.Code != http.StatusOK {
		t.Fatalf("/v1/healthz with a non-loopback Host = %d, want 200", health.Code)
	}
}

func TestHealthzEchoesAnAllowedOriginAndOnlyAnAllowedOne(t *testing.T) {
	// Decision (b). Skipping the middleware also skips its CORS grant, so
	// healthz applies the grant half itself. Echoed verbatim, never "*", and
	// Vary: Origin either way so a cache cannot cross-serve the two answers.
	const allowed = "http://tauri.localhost"
	h := catalogNewAPI(t, func(d *api.Deps) {
		d.Origin = httpx.NewOriginPolicy([]string{allowed}, catalogQuietLogger())
	})

	ok := catalogDo(t, h, http.MethodGet, "/v1/healthz", "", map[string]string{"Origin": allowed})
	if got := ok.Header().Get("Access-Control-Allow-Origin"); got != allowed {
		t.Errorf("ACAO = %q, want %q", got, allowed)
	}
	if got := ok.Header().Get("Vary"); !strings.Contains(got, "Origin") {
		t.Errorf("Vary = %q, want it to contain Origin", got)
	}

	bad := catalogDo(t, h, http.MethodGet, "/v1/healthz", "", map[string]string{"Origin": "http://evil.example"})
	if bad.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 — healthz does not reject an origin, it just declines to grant it", bad.Code)
	}
	if got := bad.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("ACAO = %q for a disallowed origin, want it absent", got)
	}

	// The mixed-client case, and the one an early return loses: a supervisor's
	// curl carries no Origin, and if that answer is cached without Vary the
	// next browser gets it back with no Access-Control-Allow-Origin on it.
	none := catalogDo(t, h, http.MethodGet, "/v1/healthz", "", nil)
	if got := none.Header().Get("Vary"); !strings.Contains(got, "Origin") {
		t.Errorf("Vary = %q on a request with no Origin, want it to contain Origin", got)
	}
	if got := none.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("ACAO = %q with no Origin to echo, want it absent", got)
	}
}

func TestNewRejectsEveryMissingDependency(t *testing.T) {
	// Every one of these is a nil dereference inside a handler if New lets it
	// through, i.e. a 500 at 3am instead of a refusal to start.
	for name, blank := range map[string]func(*api.Deps){
		"Backend":      func(d *api.Deps) { d.Backend = nil },
		"BackendReady": func(d *api.Deps) { d.BackendReady = nil },
		"Store":        func(d *api.Deps) { d.Store = nil },
		"Flags":        func(d *api.Deps) { d.Flags = nil },
		"Projects":     func(d *api.Deps) { d.Projects = nil },
		"Worktrees":    func(d *api.Deps) { d.Worktrees = nil },
		"Origin":       func(d *api.Deps) { d.Origin = nil },
	} {
		t.Run(name, func(t *testing.T) {
			d, _ := catalogNewDeps(t)
			blank(&d)
			a, err := api.New(d)
			if err == nil {
				t.Fatalf("New accepted a nil %s and returned %v", name, a)
			}
			if !strings.Contains(err.Error(), name) {
				t.Errorf("error = %q, want it to name the field %q", err, name)
			}
		})
	}
}

func TestNewFillsTheOptionalDependencies(t *testing.T) {
	// Version and Log are the only two with a safe default: an unset version
	// is this build's version, and an unset logger is slog's default. Neither
	// can be wrong in a way that misleads anyone.
	d, _ := catalogNewDeps(t)
	d.Version = ""
	d.Log = nil
	a, err := api.New(d)
	if err != nil {
		t.Fatalf("New rejected a Deps whose only gaps are optional: %v", err)
	}
	rec := catalogDo(t, a, http.MethodGet, "/v1/healthz", "", nil)
	if !strings.Contains(rec.Body.String(), `"version":"`+api.DefaultVersion+`"`) {
		t.Errorf("healthz body = %s, want version %q", rec.Body.String(), api.DefaultVersion)
	}
}
```

Create `control-plane/internal/api/export_test.go`. It is `package api` and not `package api_test`, which is the whole point of it: `API.guarded` and `API.root` are unexported, and an external test package cannot see them. This is also why step 3's failure below is a type-check error rather than the loader's `no non-test Go files` — the directory does have a `package api` file.

```go
package api

// RegisteredPatterns returns every ServeMux pattern New registered, across the
// guarded mux and the outer one, guarded first.
//
// It exists for the route audit Group C ruling 1 requires: contract §10a says no
// GET may change state, net/http offers no way to enumerate a mux, and an
// invariant checked against a hand-maintained list is an invariant that stops
// being checked the day someone forgets the list. Registration records instead —
// see routes in api.go — so the audit walks what the product actually serves,
// including the routes tasks 11 and 12 add.
//
// It reads both recorders AT CALL TIME. The first version of this file copied
// their contents into a field partway through New, which silently excused every
// route registered after that line — including, had it survived, the ones tasks
// 11 and 12 add at the bottom of the function.
//
// It lives in export_test.go rather than in api.go because nothing in the
// product needs it, and a route table the package exports is a route table
// something eventually branches on.
func (a *API) RegisteredPatterns() []string {
	out := make([]string, 0, len(a.guarded.registered)+len(a.root.registered))
	out = append(out, a.guarded.registered...)
	out = append(out, a.root.registered...)
	return out
}
```

Create `control-plane/internal/api/routes_audit_test.go`. **This is Group C ruling 1**, and it is a separate file because it is the one tasks 11 and 12 must edit: they add a line to `catalogReadOnlyRoutes` for each read-only route they register, and they touch nothing else in here. Three tests, and each covers a different way the invariant dies. `TestNoRegisteredGetCanChangeState` walks what `New` actually registered — live, so position in `New` is irrelevant — and checks it both ways: no unlisted `GET`, and no allowlist entry for a pattern that no longer exists, because a stale entry is standing permission for whatever is registered under it next. `TestTheRouteAuditRejectsASideEffectingGet` drives the audit with patterns that do not exist, so the check is known to fail rather than assumed to. And `TestNoFileRegistersOnTheRawMuxBehindTheRecorder` is the belt: Go cannot stop a file in this package reaching `rs.mux` directly, so the scan makes doing it a red suite that names the rule.

```go
package api_test

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/api"
)

// This file is contract §10a, checked by a machine — Group C ruling 1. It is its
// own file because it is the one tasks 11 and 12 must edit, and a name they can
// find beats a block halfway down api_test.go.

// catalogReadOnlyRoutes is the explicit allowlist: every GET and HEAD pattern
// this package registers must appear here, beside the reason it is safe to serve
// with no CSRF defence at all.
//
// Contract §10a is the rule and task 9's origin.go is the reason. That middleware
// allows a request carrying NO Origin header — curl and every non-browser tool
// send none — and a browser sets Host itself, so a hostile page's
// <img src="http://127.0.0.1:8730/…"> satisfies both of the control plane's
// checks without ever seeing the response. Seeing the response is not what a
// side effect needs. There is no token on this hop before Phase 8.
//
// ADDING A READ-ONLY ROUTE MEANS ADDING A LINE HERE. Tasks 11 and 12 extend this
// map; they do not touch catalogAuditRoutes. Adding a side-effecting GET means a
// failing test that names the rule before it can be made to pass.
var catalogReadOnlyRoutes = map[string]string{
	"GET /v1/healthz": "Reads Deps.BackendReady, which task 13 keeps as a CACHED bool. " +
		"This is the one route mounted outside the origin guard, so a hostile page reaches it " +
		"with no checks whatsoever; a healthz that refreshed a probe on request would be a " +
		"side-effecting GET on the least defended route on the surface (ruling 2).",
	"GET /api/v1/healthz":                    "Contract §7.5's alias of the same handler, on the same outer mux.",
	"GET /v1/machines":                       "Reads the backend kind and the cached readiness bool. Nothing is created and nothing is probed.",
	"GET /v1/projects":                       "A git enumeration through three read-only subcommands (project.Scanner).",
	"GET /v1/projects/{projectId}/branches":  "The same enumeration, filtered to one project.",
	"GET /v1/projects/{projectId}/worktrees": "An enumeration plus one registry read and one flag snapshot. The ready flag is only ever WRITTEN by the PATCH beside it.",
	"GET /v1/harnesses":                      "A package-level table (contract §2.9), copied on the way out.",
}

// catalogMethodAgnosticRoutes is the other half of the audit, and the half that is
// easy to forget: a pattern with NO method in it answers every verb, GET included,
// so a check that only inspects patterns labelled GET would wave it straight
// through. Each entry states why the route cannot act on a GET.
var catalogMethodAgnosticRoutes = map[string]string{
	"/api/v1/": "Contract §7.5's alias. It strips /api and re-dispatches into the same mux, " +
		"where a method-bearing pattern decides — so the audit of those patterns covers this one.",
	"/": "The outer mux's catch-all into the origin middleware and the guarded mux. " +
		"Same argument: every route it can reach is audited on its own pattern.",
}

// catalogAuditRoutes applies contract §10a to a list of registered patterns and
// returns one message per violation. It takes the list rather than reading it,
// so the same rule can be run against the real route table and against a pattern
// that does not exist yet.
func catalogAuditRoutes(patterns []string) []string {
	var violations []string
	for _, pattern := range patterns {
		method, _, hasMethod := strings.Cut(pattern, " ")
		switch {
		case !hasMethod:
			if _, allowed := catalogMethodAgnosticRoutes[pattern]; !allowed {
				violations = append(violations, fmt.Sprintf(
					"%q is registered with no method, so it answers GET as well as everything else. "+
						"Contract §10a: no GET may change state, because task 9's guard passes a request "+
						"with no Origin and the browser sets Host itself. Name the method in the pattern, "+
						"or add the pattern to catalogMethodAgnosticRoutes with the reason a GET is safe on it.",
					pattern))
			}
		case method == http.MethodGet, method == http.MethodHead:
			if _, allowed := catalogReadOnlyRoutes[pattern]; !allowed {
				violations = append(violations, fmt.Sprintf(
					"%q is a %s that is not on the read-only allowlist. Contract §10a: no GET or HEAD "+
						"may change state — a hostile page can emit <img src> at this route and both of "+
						"the control plane's checks pass. If it only reads, add it to "+
						"catalogReadOnlyRoutes with the reason. If it writes, it needs POST, PATCH or "+
						"DELETE — or a Phase 8 token — and not a line in that map.",
					pattern, method))
			}
		}
	}
	return violations
}

func TestNoRegisteredGetCanChangeState(t *testing.T) {
	// Group C ruling 1: the invariant contract §10a states in prose, checked by
	// a machine. It walks what New ACTUALLY registered rather than a list
	// maintained by hand, and it reads the recorders live rather than a snapshot,
	// so tasks 11 and 12's routes are audited the moment they are added —
	// wherever in New the registering line lands, and with no change to this test.
	d, _ := catalogNewDeps(t)
	a, err := api.New(d)
	if err != nil {
		t.Fatalf("api.New: %v", err)
	}

	registered := a.RegisteredPatterns()
	if len(registered) == 0 {
		t.Fatal("New registered no patterns; the audit would pass vacuously")
	}
	for _, violation := range catalogAuditRoutes(registered) {
		t.Error(violation)
	}

	// The reverse direction, so the allowlist cannot rot: an entry for a route
	// that no longer exists is standing permission for whatever is registered
	// under that pattern next.
	live := make(map[string]bool, len(registered))
	for _, pattern := range registered {
		live[pattern] = true
	}
	for pattern := range catalogReadOnlyRoutes {
		if !live[pattern] {
			t.Errorf("catalogReadOnlyRoutes allows %q, which is not registered — delete the line", pattern)
		}
	}
	for pattern := range catalogMethodAgnosticRoutes {
		if !live[pattern] {
			t.Errorf("catalogMethodAgnosticRoutes allows %q, which is not registered — delete the line", pattern)
		}
	}
}

func TestTheRouteAuditRejectsASideEffectingGet(t *testing.T) {
	// The audit above is only worth having if it fails, so this drives the two
	// shapes a side effect arrives in: a GET that reads like a verb, and a
	// pattern registered with no method at all.
	for name, pattern := range map[string]string{
		"a GET that is really a verb": "GET /v1/sessions/{sessionId}/kill",
		"a pattern with no method":    "/v1/sessions/{sessionId}",
	} {
		t.Run(name, func(t *testing.T) {
			violations := catalogAuditRoutes([]string{pattern})
			if len(violations) != 1 {
				t.Fatalf("audit of %q returned %d violations, want 1: %v", pattern, len(violations), violations)
			}
			if !strings.Contains(violations[0], pattern) {
				t.Errorf("violation = %q, want it to name the offending pattern", violations[0])
			}
			if !strings.Contains(violations[0], "§10a") {
				t.Errorf("violation = %q, want it to point at contract §10a", violations[0])
			}
		})
	}
}

func TestNoFileRegistersOnTheRawMuxBehindTheRecorder(t *testing.T) {
	// The belt to the recorder's braces. Making routes.mux a named unexported
	// field means mux.ServeMux.HandleFunc — the spelling the embedded version
	// promoted into every file here — no longer compiles, but Go has no barrier
	// against a.guarded.mux.Handle(…) INSIDE this package, and a route
	// registered that way is served and never audited.
	//
	// So: no source file but api.go, which declares the type, may select .mux at
	// all. Reaching past the recorder becomes a deliberate act that reddens a
	// suite and names this rule, rather than a nine-character alternative
	// spelling.
	//
	// _test.go files are out of scope, and not as an oversight: the threat is a
	// route the PRODUCT serves and no test walks, and a test file registers
	// nothing in the product binary. Scoping it this way also keeps this file,
	// which has to spell the forbidden selectors out, from matching itself.
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read the package directory: %v", err)
	}
	found := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") ||
			strings.HasSuffix(name, "_test.go") || name == "api.go" {
			continue
		}
		src, err := os.ReadFile(filepath.Clean(name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		found++
		for _, bypass := range []string{".mux.Handle", ".mux.ServeMux", ".mux.HandleFunc"} {
			if strings.Contains(string(src), bypass) {
				t.Errorf("%s contains %q. Register through the *routes recorder — mux.Handle or "+
					"mux.HandleFunc — so contract §10a's audit can see the route. Reaching the "+
					"mux behind it serves a route no test walks.", name, bypass)
			}
		}
	}
	if found == 0 {
		t.Fatal("scanned no source files; the working directory is not the package source " +
			"directory, or every handler file has been renamed and this scan is now a no-op")
	}
}
```

- [ ] **Step 2: Write the failing tests for the read-only surface**

These compare **exact bytes** wherever the desktop's TypeScript parses the field, because a round trip through `json.Unmarshal` passes even when the key is spelled `sessioncount`, and it is the desktop's parser rather than Go's that has to match. Three bodies are pinned byte-for-byte — healthz, which is asserted in both its ready and its not-ready form, an empty `projects` and an empty `worktrees` — and the rest are decoded into a map so a test failure names the field rather than printing two hundred characters of diff. Every expected body ends in `"\n"`: task 9's `WriteJSON` goes through `json.Encoder`, which appends one.

The empty-slice tests are the ones that catch a real defect rather than a typo. `json.Marshal` renders a nil slice as `null`, and `null.map(...)` in the cockpit's rail is a blank screen with a console error rather than the empty state `docs/FUNCTIONAL-SPEC.md` specifies. There is one for `projects`, one for a harness with no models and one for a `branchId` filter that matches nothing.

Two tests here exist only to reach the 500s, because a 500 nobody exercises is a 500 that is wrong the first time it fires. One drives `catalogProjects.err` and asserts that `a.internal` answers `500 internal` with contract §3.2's generic message and no path from the underlying error in the body. The other drives `catalogBrokenList` and pins decision (f): a `Store.List()` failure is a 500 and the response carries no `sessionCount` at all, because a count nobody could read is a fabricated number and this phase rules those out.

Create `control-plane/internal/api/catalog_test.go`:

```go
package api_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/api"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/harness"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/project"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry"
)

// catalogNewAPIWithSessions builds the handler over a real FileStore seeded with the
// given sessions, which is how sessionCount gets something to count.
func catalogNewAPIWithSessions(t *testing.T, sessions ...registry.Session) http.Handler {
	t.Helper()
	d, st := catalogNewDeps(t)
	for _, s := range sessions {
		if err := st.Create(s); err != nil {
			t.Fatalf("seed %s: %v", s.ID, err)
		}
	}
	a, err := api.New(d)
	if err != nil {
		t.Fatalf("api.New: %v", err)
	}
	return a
}

// catalogDecodeObject reads a JSON object out of a response body. Used where an
// assertion should name one field rather than diff two hundred characters.
func catalogDecodeObject(t *testing.T, body string) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("body %q is not a JSON object: %v", body, err)
	}
	return out
}

// catalogListOf pulls one array field out of such an object.
func catalogListOf(t *testing.T, body, field string) []map[string]any {
	t.Helper()
	raw, ok := catalogDecodeObject(t, body)[field]
	if !ok {
		t.Fatalf("body %s has no %q field", body, field)
	}
	items, ok := raw.([]any)
	if !ok {
		t.Fatalf("%q is %T, want an array — a nil slice marshals as null and the rail cannot map over it", field, raw)
	}
	out := make([]map[string]any, 0, len(items))
	for i, it := range items {
		m, ok := it.(map[string]any)
		if !ok {
			t.Fatalf("%s[%d] is %T, want an object", field, i, it)
		}
		out = append(out, m)
	}
	return out
}

func TestHealthzBodyIsExactlyTheContractShape(t *testing.T) {
	// Contract 3.3, byte for byte. A supervisor greps this.
	rec := catalogDo(t, catalogNewAPI(t, nil), http.MethodGet, "/v1/healthz", "", nil)

	want := `{"status":"ok","version":"0.2.0","backend":"docker","backendReady":true}` + "\n"
	if got := rec.Body.String(); got != want {
		t.Errorf("body =\n\t%q\nwant\n\t%q", got, want)
	}
}

func TestHealthzIs200EvenWhenTheBackendIsNotReady(t *testing.T) {
	// Contract 3.3 is explicit: the control plane is up, the backend is not,
	// and restarting the control plane would not fix Docker Desktop being shut.
	h := catalogNewAPI(t, func(d *api.Deps) { d.BackendReady = func() bool { return false } })
	rec := catalogDo(t, h, http.MethodGet, "/v1/healthz", "", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	want := `{"status":"ok","version":"0.2.0","backend":"docker","backendReady":false}` + "\n"
	if got := rec.Body.String(); got != want {
		t.Errorf("body =\n\t%q\nwant\n\t%q", got, want)
	}
}
```

Still `control-plane/internal/api/catalog_test.go`, continuing below the healthz tests. The isolation test is a table over three backend kinds, and the third row is the one Group C ruling 6 makes binding: `isolationFor` is this package's ONLY kind-to-enum mapping, task 11 calls it rather than declaring a second, and an unrecognised kind is `"none"` — a member of contract §3.4's three-value enum, not a fourth value invented at one call site:

```go
func TestMachinesDescribesTheLocalDockerMachine(t *testing.T) {
	// Contract 3.4. "container" rather than "microvm" is the honest value for
	// what Phase 2 builds, and limitations says what it means in a sentence.
	rec := catalogDo(t, catalogNewAPI(t, nil), http.MethodGet, "/v1/machines", "", nil)

	machines := catalogListOf(t, rec.Body.String(), "machines")
	if len(machines) != 1 {
		t.Fatalf("got %d machines, want exactly 1 — cloud machines are Phase 8", len(machines))
	}
	m := machines[0]
	for field, want := range map[string]any{
		"id":        api.LocalMachineID,
		"name":      "This PC",
		"kind":      "local",
		"state":     "running",
		"backend":   "docker",
		"isolation": "container",
		"default":   true,
	} {
		if got := m[field]; got != want {
			t.Errorf("machines[0].%s = %v, want %v", field, got, want)
		}
	}
	if _, present := m["message"]; present {
		t.Error("a running machine carries a message; message explains an error state only")
	}
	lim, _ := m["limitations"].([]any)
	if len(lim) == 0 {
		t.Error("limitations is empty — the local machine is badged unsandboxed and must say why")
	}
}

func TestMachinesReportsErrorAndAMessageWhenTheBackendIsDown(t *testing.T) {
	// Contract 3.4: Phase 2 emits running or error only, and error carries a
	// message naming the cause. The other four states need Phase 8 (contract 8.7).
	h := catalogNewAPI(t, func(d *api.Deps) { d.BackendReady = func() bool { return false } })
	rec := catalogDo(t, h, http.MethodGet, "/v1/machines", "", nil)

	m := catalogListOf(t, rec.Body.String(), "machines")[0]
	if got := m["state"]; got != "error" {
		t.Fatalf("state = %v, want \"error\"", got)
	}
	msg, _ := m["message"].(string)
	if !strings.Contains(msg, "docker") {
		t.Errorf("message = %q, want it to name the backend that is not responding", msg)
	}
}

func TestMachinesIsolationFollowsTheBackendKind(t *testing.T) {
	// katabe is dormant until Phase 8, but the mapping must not be a literal.
	//
	// The third row is the one ruling 6 makes binding: isolationFor is the
	// package's ONLY kind-to-enum mapping, task 11 calls it rather than
	// declaring a second, and the value an unrecognised kind gets is "none" —
	// a member of contract §3.4's three-value enum, not a fourth value like
	// "unknown" invented at one call site.
	for kind, wantIsolation := range map[backend.Kind]string{
		backend.KindDocker: "container",
		backend.KindKata:   "microvm",
		"podman":           "none",
	} {
		t.Run(string(kind), func(t *testing.T) {
			h := catalogNewAPI(t, func(d *api.Deps) { d.Backend = catalogBackend{kind: kind} })
			rec := catalogDo(t, h, http.MethodGet, "/v1/machines", "", nil)

			m := catalogListOf(t, rec.Body.String(), "machines")[0]
			if got := m["isolation"]; got != wantIsolation {
				t.Errorf("isolation = %v for the %s backend, want %q", got, kind, wantIsolation)
			}
			if got := m["backend"]; got != string(kind) {
				t.Errorf("backend = %v, want %q", got, kind)
			}
			// Every kind says what it costs the user, including the one this
			// build does not know: "unsandboxed" with no sentence under it is
			// a badge nobody can act on.
			if lim, _ := m["limitations"].([]any); len(lim) == 0 {
				t.Errorf("limitations is empty for the %s backend", kind)
			}
		})
	}
}

func TestProjectsCarriesIdNameAndAbsolutePath(t *testing.T) {
	rec := catalogDo(t, catalogNewAPI(t, nil), http.MethodGet, "/v1/projects", "", nil)

	ps := catalogListOf(t, rec.Body.String(), "projects")
	if len(ps) != 1 {
		t.Fatalf("got %d projects, want 1", len(ps))
	}
	if ps[0]["id"] != catalogTestProject || ps[0]["name"] != "engagement-portal" {
		t.Errorf("projects[0] = %v", ps[0])
	}
	if _, ok := ps[0]["path"].(string); !ok {
		t.Error("projects[0].path is missing; the desktop shows it under the name")
	}
}

func TestProjectsWithNoProjectsIsAnEmptyArrayNotNull(t *testing.T) {
	// Contract 3.5: an unconfigured PANACEA_APP_PROJECTS_DIR is 200 and an
	// empty list. null here is a blank rail and a console error.
	h := catalogNewAPI(t, func(d *api.Deps) { d.Projects = &catalogProjects{} })
	rec := catalogDo(t, h, http.MethodGet, "/v1/projects", "", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 — empty is a state the cockpit renders", rec.Code)
	}
	if got, want := rec.Body.String(), `{"projects":[]}`+"\n"; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
}

func TestAnEnumerationFailureIs500ThatLeaksNothing(t *testing.T) {
	// a.internal is the only 500 on this surface and every later task uses it,
	// so it is worth one test rather than none. Contract §3.2 makes the message
	// deliberately generic: the body is read by the cockpit, the cause goes to
	// the log, and a path on disk belongs in exactly one of those.
	h := catalogNewAPI(t, func(d *api.Deps) {
		d.Projects = &catalogProjects{err: errors.New(`open C:\work\engagement-portal\.git: permission denied`)}
	})
	rec := catalogDo(t, h, http.MethodGet, "/v1/projects", "", nil)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body = %s", rec.Code, rec.Body.String())
	}
	if got := catalogDecodeObject(t, rec.Body.String())["error"]; got != "internal" {
		t.Errorf("error = %v, want \"internal\"", got)
	}
	if strings.Contains(rec.Body.String(), `C:\work`) {
		t.Errorf("body = %s, want no path from the underlying error in it", rec.Body.String())
	}
}
```

Still `control-plane/internal/api/catalog_test.go`, the branch and worktree block. `sessionCount` is the one number on this surface, and contract §3.7 is explicit about what it counts — live things the user owns, not progress — so the exited-session test is the load-bearing one:

```go
func TestBranchesCarryIsHeadAndTheProjectId(t *testing.T) {
	rec := catalogDo(t, catalogNewAPI(t, nil), http.MethodGet, "/v1/projects/"+catalogTestProject+"/branches", "", nil)

	bs := catalogListOf(t, rec.Body.String(), "branches")
	if len(bs) != 2 {
		t.Fatalf("got %d branches, want 2", len(bs))
	}
	heads := 0
	for _, b := range bs {
		if b["projectId"] != catalogTestProject {
			t.Errorf("branch %v carries the wrong projectId", b["id"])
		}
		if b["isHead"] == true {
			heads++
		}
	}
	if heads != 1 {
		t.Errorf("got %d branches with isHead, want exactly 1", heads)
	}
}

func TestAnUnknownProjectIs404ProjectNotFound(t *testing.T) {
	// project.ErrUnknownProject is task 8's sentinel and is matched with
	// errors.Is. A handler that string-matched would pass this test and fail
	// the first time the message changed, so the fake wraps it with %w.
	h := catalogNewAPI(t, nil)
	for _, target := range []string{"/branches", "/worktrees"} {
		t.Run(target, func(t *testing.T) {
			rec := catalogDo(t, h, http.MethodGet, "/v1/projects/p-deadbeef"+target, "", nil)
			if rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404; body = %s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), "project_not_found") {
				t.Errorf("body = %s, want the project_not_found code", rec.Body.String())
			}
		})
	}
}

func TestWorktreesCarrySessionCountAndReady(t *testing.T) {
	// ready ships false on every worktree until something PATCHes it. That is
	// known defect 9 fixed rather than deleted: the field exists, so the verb can.
	h := catalogNewAPIWithSessions(t,
		registry.Session{ID: "s-00000001", WorktreeID: catalogTestWorktree, Status: registry.StatusRunning},
		registry.Session{ID: "s-00000002", WorktreeID: catalogTestWorktree, Status: registry.StatusIdle},
		registry.Session{ID: "s-00000003", WorktreeID: catalogOtherWorktree, Status: registry.StatusStarting},
	)
	rec := catalogDo(t, h, http.MethodGet, "/v1/projects/"+catalogTestProject+"/worktrees", "", nil)

	byID := map[string]map[string]any{}
	for _, w := range catalogListOf(t, rec.Body.String(), "worktrees") {
		byID[w["id"].(string)] = w
	}
	if got := byID[catalogTestWorktree]["sessionCount"]; got != float64(2) {
		t.Errorf("sessionCount = %v, want 2", got)
	}
	if got := byID[catalogOtherWorktree]["sessionCount"]; got != float64(1) {
		t.Errorf("sessionCount = %v, want 1", got)
	}
	for id, w := range byID {
		if w["ready"] != false {
			t.Errorf("%s.ready = %v, want false before anything sets it", id, w["ready"])
		}
		if _, ok := w["isPrimary"]; !ok {
			t.Errorf("%s has no isPrimary field", id)
		}
	}
}

func TestSessionCountIgnoresExitedSessions(t *testing.T) {
	// Contract 3.7: a count of live things the user owns. Counting exited
	// sessions turns it into a history, which docs/AGENTIC-UX.md deletes.
	h := catalogNewAPIWithSessions(t,
		registry.Session{ID: "s-00000001", WorktreeID: catalogTestWorktree, Status: registry.StatusExited},
		registry.Session{ID: "s-00000002", WorktreeID: catalogTestWorktree, Status: registry.StatusRunning},
	)
	rec := catalogDo(t, h, http.MethodGet, "/v1/projects/"+catalogTestProject+"/worktrees", "", nil)

	for _, w := range catalogListOf(t, rec.Body.String(), "worktrees") {
		if w["id"] == catalogTestWorktree && w["sessionCount"] != float64(1) {
			t.Fatalf("sessionCount = %v, want 1 — the exited session was counted", w["sessionCount"])
		}
	}
}

func TestAStoreListFailureIs500AndNeverAZeroCount(t *testing.T) {
	// Decision (f). The tempting alternative is sessionCount 0 and a 200, and
	// it is a fabricated number: the rail would show a worktree with no live
	// sessions on it when nobody knows how many there are.
	d, st := catalogNewDeps(t)
	d.Store = catalogBrokenList{FileStore: st, err: errors.New("registry: the document is unreadable")}
	a, err := api.New(d)
	if err != nil {
		t.Fatalf("api.New: %v", err)
	}

	rec := catalogDo(t, a, http.MethodGet, "/v1/projects/"+catalogTestProject+"/worktrees", "", nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body = %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "sessionCount") {
		t.Errorf("body = %s, want no counts in it at all", rec.Body.String())
	}
}

func TestWorktreesBranchIdFilterIncludingAMissWhichIsAnEmptyArray(t *testing.T) {
	h := catalogNewAPI(t, nil)

	hit := catalogDo(t, h, http.MethodGet, "/v1/projects/"+catalogTestProject+"/worktrees?branchId="+catalogTestBranch, "", nil)
	ws := catalogListOf(t, hit.Body.String(), "worktrees")
	if len(ws) != 1 || ws[0]["id"] != catalogTestWorktree {
		t.Fatalf("filtered worktrees = %v, want just %s", ws, catalogTestWorktree)
	}

	miss := catalogDo(t, h, http.MethodGet, "/v1/projects/"+catalogTestProject+"/worktrees?branchId=b-nope", "", nil)
	if got, want := miss.Body.String(), `{"worktrees":[]}`+"\n"; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
}
```

Still `control-plane/internal/api/catalog_test.go`, the `PATCH` block — the fix for known defect 9. **Every rejected body here is 400 `bad_request`**, adjudicated decision 2's code, unknown fields included — see the amended step 6. A body that is not JSON *at all* is still `invalid_json`, because task 9's `DecodeJSON` still owns every **body** rule in the phase. The two test blocks below pin both halves:

```go
func TestPatchWorktreeSetsReadyAndTheListThenReportsIt(t *testing.T) {
	h := catalogNewAPI(t, nil)
	target := "/v1/projects/" + catalogTestProject + "/worktrees/" + catalogTestWorktree

	rec := catalogDo(t, h, http.MethodPatch, target, `{"ready":true}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	// The response is the worktree object itself, not an envelope.
	got := catalogDecodeObject(t, rec.Body.String())
	if got["id"] != catalogTestWorktree || got["ready"] != true {
		t.Fatalf("PATCH returned %v, want the worktree with ready true", got)
	}

	list := catalogDo(t, h, http.MethodGet, "/v1/projects/"+catalogTestProject+"/worktrees", "", nil)
	for _, w := range catalogListOf(t, list.Body.String(), "worktrees") {
		want := w["id"] == catalogTestWorktree
		if w["ready"] != want {
			t.Errorf("after the PATCH, %s.ready = %v, want %v", w["id"], w["ready"], want)
		}
	}

	// And it clears again, because "mark ready" that cannot be undone is a
	// verb the user will not press twice.
	off := catalogDo(t, h, http.MethodPatch, target, `{"ready":false}`, nil)
	if off.Code != http.StatusOK || catalogDecodeObject(t, off.Body.String())["ready"] != false {
		t.Fatalf("clearing ready = %d %s", off.Code, off.Body.String())
	}
}

func TestPatchWorktreeRejectsABodyThatSetsSomethingElse(t *testing.T) {
	// Every one of these is 400 bad_request and not 400 invalid_json. The
	// controller's ruling: an unknown field is WELL-FORMED JSON that is
	// semantically wrong, and a code the cockpit switches on must not say
	// something untrue — the same principle that refuses sandbox_not_running for
	// a sandbox that is running. Contract §3.7 names bad_request for this case.
	target := "/v1/projects/" + catalogTestProject + "/worktrees/" + catalogTestWorktree
	for name, body := range map[string]string{
		"unknown field beside ready": `{"ready":true,"name":"renamed"}`,
		"only a name":                `{"name":"renamed"}`,
		"empty object":               `{}`,
		"null ready":                 `{"ready":null}`,
		"ready is not a boolean":     `{"ready":"yes"}`,
	} {
		t.Run(name, func(t *testing.T) {
			rec := catalogDo(t, catalogNewAPI(t, nil), http.MethodPatch, target, body, nil)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
			}
			if got := catalogDecodeObject(t, rec.Body.String())["error"]; got != "bad_request" {
				t.Errorf("error = %v, want \"bad_request\"; body = %s", got, rec.Body.String())
			}
		})
	}
}

func TestPatchWorktreeStillAnswersInvalidJsonForAMalformedBody(t *testing.T) {
	// The other half of the ruling above, and the reason it is a ruling about
	// LABELS rather than about which decoder runs: httpx.DecodeJSON still owns
	// every body rule in the phase, so a body that is not JSON is still
	// invalid_json. Two codes, two distinct meanings, one decoder.
	target := "/v1/projects/" + catalogTestProject + "/worktrees/" + catalogTestWorktree
	for name, body := range map[string]string{
		"truncated object": `{"ready":true`,
		"not json at all":  `ready please`,
		"two objects":      `{"ready":true}{"ready":false}`,
	} {
		t.Run(name, func(t *testing.T) {
			rec := catalogDo(t, catalogNewAPI(t, nil), http.MethodPatch, target, body, nil)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
			}
			if got := catalogDecodeObject(t, rec.Body.String())["error"]; got != "invalid_json" {
				t.Errorf("error = %v, want \"invalid_json\"; body = %s", got, rec.Body.String())
			}
		})
	}
}

func TestPatchAnUnknownWorktreeIs404WorktreeNotFound(t *testing.T) {
	rec := catalogDo(t, catalogNewAPI(t, nil), http.MethodPatch,
		"/v1/projects/"+catalogTestProject+"/worktrees/w-deadbeef", `{"ready":true}`, nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "worktree_not_found") {
		t.Errorf("body = %s, want the worktree_not_found code", rec.Body.String())
	}
}

func TestPatchIsScopedToTheProjectInThePath(t *testing.T) {
	// Without the membership check any project id in the path would do, and
	// the path segment would be decoration rather than authorisation.
	const secondProject = "p-11112222"
	h := catalogNewAPI(t, func(d *api.Deps) {
		p := catalogSampleProjects()
		p.projects = append(p.projects, project.Project{ID: secondProject, Name: "other", Path: `C:\work\other`})
		p.worktrees[secondProject] = []project.Worktree{
			{ID: "w-99998888", ProjectID: secondProject, BranchID: "b-11112222-main",
				Name: "other", Path: `C:\work\other`, IsPrimary: true},
		}
		d.Projects = p
	})

	rec := catalogDo(t, h, http.MethodPatch,
		"/v1/projects/"+secondProject+"/worktrees/"+catalogTestWorktree, `{"ready":true}`, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 — %s does not live under %s", rec.Code, catalogTestWorktree, secondProject)
	}
}
```

Still `control-plane/internal/api/catalog_test.go`, the last block. The catalogue is not injected through `Deps`: contract §2.9 makes it a package-level table, and a test against an injected catalogue proves nothing about the one the product serves:

```go
func TestHarnessesComputeAvailableServerSideAndNeverEmitNull(t *testing.T) {
	// Contract 3.8: available is installed && flagsVerified, computed here so
	// the desktop never re-derives it, and the empty slices are [] because
	// null.map is a blank picker.
	rec := catalogDo(t, catalogNewAPI(t, nil), http.MethodGet, "/v1/harnesses", "", nil)

	hs := catalogListOf(t, rec.Body.String(), "harnesses")
	if len(hs) != len(harness.Catalogue()) {
		t.Fatalf("got %d harnesses, want %d", len(hs), len(harness.Catalogue()))
	}
	seen := map[string]bool{}
	for _, h := range hs {
		id, _ := h["id"].(string)
		seen[id] = true
		want := h["installed"] == true && h["flagsVerified"] == true
		if h["available"] != want {
			t.Errorf("%s.available = %v, want installed && flagsVerified = %v", id, h["available"], want)
		}
		if h["available"] == false && h["note"] == "" {
			t.Errorf("%s is unavailable with no note; the picker renders the note as the reason", id)
		}
		for _, field := range []string{"models", "efforts", "permissionModes"} {
			if _, ok := h[field].([]any); !ok {
				t.Errorf("%s.%s is %T, want an array even when empty", id, field, h[field])
			}
		}
	}
	for _, id := range []harness.ID{harness.ClaudeCode, harness.Shell, harness.Codex,
		harness.OpenCode, harness.GeminiCLI} {
		if !seen[string(id)] {
			t.Errorf("the surface dropped %q", id)
		}
	}
	if !seen[string(harness.Shell)] {
		t.Error("the shell is a harness; a picker that models 'no agent' as an absence cannot render it")
	}
}
```

- [ ] **Step 3: Run the tests and watch the package fail to build**

Run, from the repository root:

```bash
go test ./sandboxd/... ./control-plane/... -race -count=1
```

`go test ./...` is invalid here — `go.work` `use`s `./sandboxd` and `./control-plane` and nothing at the root, so the shorter form errors with `directory prefix . does not contain modules listed in go.work`.

Expected: `internal/api` holds test files and no source, so the build fails on the first symbol `api.go` has not declared yet. The other packages report their usual `ok` and `[no test files]` lines, which are not reproduced here:

```
# github.com/kpmg-au-emu/panacea-app/control-plane/internal/api [github.com/kpmg-au-emu/panacea-app/control-plane/internal/api.test]
control-plane\internal\api\export_test.go:21:10: undefined: API
FAIL	github.com/kpmg-au-emu/panacea-app/control-plane/internal/api [build failed]
```

**This is a type-check error and not the loader's `no non-test Go files in …`, which an earlier draft of this step predicted.** That message is emitted only when every file in the directory is an *external* test file, and `export_test.go` — the file ruling 1 needs, because a `package api_test` audit has no other way to see `API.patterns` — is `package api`. So the package does have a Go file for the test build, the load succeeds, and the type checker runs instead. The line and column follow that file's doc comment and will move if it is reworded; the `undefined: API` is the assertion. The gate below is unchanged and still passes: the failure is in `internal/api` and nowhere else.

If instead the run fails inside `internal/httpx`, `internal/harness`, `internal/project`, `internal/worktree` or `internal/registry`, task 9 or task 8 is not merged into this branch yet and this task cannot start. Stop and check that before writing a line of `api.go`.

- [ ] **Step 4: Write the router**

Create `control-plane/internal/api/api.go`:

```go
// Package api is the control plane's HTTP surface: one http.Handler over a
// Go 1.22 ServeMux with the origin guard wrapped around it.
//
// THE PACKAGE IS SEVERAL FILES ON PURPOSE (contract §1.2 correction 1). api.go
// owns Deps, New and routing and nothing else; catalog.go, and later
// sessions.go and attach.go, own handlers. Three tasks write into this package
// and one file guarantees three collisions in the same region.
//
// THERE IS NO AUTHENTICATION ON THIS SURFACE and none is intended before
// Phase 8. The entire boundary is three things: task 13 binds the listener to
// loopback, httpx.OriginPolicy rejects a Host that is not loopback, and it
// rejects an Origin that is not allowlisted. Every route but /healthz sits
// behind the last two.
//
// AND SO: NO GET MAY CHANGE STATE (Group C contract §10a). Task 9's guard
// deliberately allows a request carrying no Origin — curl and every non-browser
// tool send none — while the browser sets Host itself, so a hostile page's
// <img src="http://127.0.0.1:8730/…"> satisfies both checks without ever seeing
// the reply. Seeing the reply is not what a side effect needs. Registration
// therefore goes through the recording mux below, and api_test.go audits every
// pattern it records against an explicit read-only allowlist.
package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/project"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/worktree"
)

// DefaultVersion is what /healthz reports when Deps.Version is empty.
const DefaultVersion = "0.2.0"

// LocalMachineID is the id of the one machine Phase 2 has. Cloud machines are
// claimed from a warm pool at first sign-in, which is Phase 8.
const LocalMachineID = "local"

// Enumerator is the read-only project surface the catalog handlers need: the
// three enumerating methods of *project.Scanner, as an interface, so a handler
// test needs neither git nor a repository on disk. Contract §2.11 makes the
// package read-only by design — Phase 3 owns worktree lifecycle — and this
// interface cannot express a mutation, which keeps that true by construction.
type Enumerator interface {
	Projects(ctx context.Context) ([]project.Project, error)
	Branches(ctx context.Context, projectID string) ([]project.Branch, error)
	Worktrees(ctx context.Context, projectID string) ([]project.Worktree, error)
}

// Deps is everything the HTTP surface needs. It is a struct rather than a
// constructor's argument list so tasks 11 to 13 can add a field without
// changing a signature four packages already call.
//
// THE FIELD SPELLINGS BELOW ARE FIXED and four tasks write against them. Origin
// is singular. Task 11 adds exactly one field of its own — Image, the sandbox
// image it creates sessions from — and adds it here, in the step that first
// uses it. Nothing adds IdleAfter: the idle window is a field of reaper.Config,
// because the reaper derives idle on its sweep and this package never computes
// it.
type Deps struct {
	// Version is reported by /healthz. Empty means DefaultVersion.
	Version string

	// Backend runs sandboxes. This task calls only Kind(); tasks 11 to 13 use
	// the rest of the interface.
	Backend backend.Backend

	// BackendReady is the cached liveness of that backend, refreshed by task
	// 13's reaper on its own timer. Contract §3.3 calls it "a cached result of
	// the backend's Ping", but backend.Backend (§2.2) has no Ping and §2.2 is
	// locked, so the cache is the seam instead. It must not block: /healthz
	// calls it inline and the whole point of caching is that a health check
	// never hangs on a hung Docker socket.
	//
	// It must also not PROBE. /healthz is outside the origin guard, so a
	// refresh triggered from here would be a side-effecting GET on the one
	// route with no defences at all (Group C ruling 2).
	BackendReady func() bool

	// Store is the session registry. The catalog reads it only to count live
	// sessions per worktree.
	Store registry.Store

	// Flags is the worktree ready flag, deliberately NOT part of Store —
	// contract §2.8 declares Store with five session methods and task 11 is
	// written against that shape.
	Flags registry.WorktreeFlagStore

	// Projects enumerates projects, branches and worktrees.
	Projects Enumerator

	// Worktrees resolves a worktree id to a host path. This task never calls
	// it; task 11 mounts what it returns into a sandbox. It is validated here
	// because a nil one is a panic at session create rather than at startup,
	// and one *project.Scanner satisfies this and Projects both.
	Worktrees worktree.Provider

	// Origin is the allowlist and the loopback Host guard, from task 9.
	Origin *httpx.OriginPolicy

	// Log is where a 500's real cause goes. Nil means slog.Default().
	Log *slog.Logger
}
```

Still `control-plane/internal/api/api.go`, continuing below `Deps` — do not repeat the `package` clause or the imports.

**Why the mux is wrapped, said once here rather than in every comment below.** Group C ruling 1 makes contract §10a — no `GET` may change state — a thing a test checks rather than a thing two documents assert. `net/http` can route a request but cannot list a mux's patterns, so the audit has to be handed the table, and the only table that cannot drift from what is served is one written at registration. Hence `routes`: a mux that records. Three details of it are each a defect that was found and closed, and a reader who removes any of them reopens it. The field is **named and unexported**, because embedding promoted an exported `ServeMux` selector into every file in the package and `mux.ServeMux.HandleFunc(…)` then compiled, served and recorded nothing. `ServeHTTP` is **explicit**, because de-embedding took away the promoted one that the alias and the middleware wrap both rely on. And the two recorders are kept **whole on the `API`** rather than copied into a `[]string`, because a snapshot taken partway through `New` silently excused every route registered below it — which is exactly where tasks 11 and 12 would add theirs.

```go
// route is one ServeMux pattern and the handler registered under it.
//
// Registration is a table rather than a sequence of literal mux.HandleFunc
// calls because Go 1.22 puts the METHOD IN THE PATTERN — "GET /v1/machines" —
// which makes contract §10a's no-side-effecting-GET invariant checkable by a
// machine for the first time. Group C ruling 1.
type route struct {
	Pattern string
	Handler http.Handler
}

// routes is a mux that remembers what was registered on it. net/http can route
// a request but cannot list its own patterns, so recording at registration is
// the only way a test can walk the real route table instead of a list somebody
// maintains by hand — and a hand-maintained list is one that stops matching
// reality on the first busy afternoon.
//
// TASKS 11 AND 12 TAKE *routes AND NOT *http.ServeMux, deliberately: a register
// method written against this type records whatever it registers with no extra
// step and no coordination, and one written against *http.ServeMux fails to
// compile at its call site in New rather than quietly dropping its routes out
// of the audit.
//
// THE MUX IS A NAMED, UNEXPORTED FIELD AND NOT AN EMBEDDED ONE. Embedded, it
// promoted an exported ServeMux selector into every file in this package, so
// mux.ServeMux.HandleFunc(…) compiled, served, and was never recorded — nine
// characters away from the intended call and invisible to the audit. Named and
// unexported, that spelling does not compile at all. Reaching rs.mux directly
// is still possible inside this package, because Go has no stronger barrier
// than that; it is a deliberate act rather than the obvious alternative, and
// routes_audit_test.go scans this package's own sources for it.
type routes struct {
	mux        *http.ServeMux
	registered []string
}

func newRoutes() *routes {
	return &routes{mux: http.NewServeMux()}
}

// Handle registers a pattern and records it.
func (rs *routes) Handle(pattern string, h http.Handler) {
	rs.registered = append(rs.registered, pattern)
	rs.mux.Handle(pattern, h)
}

// HandleFunc is Handle for a plain function, so a register method reads exactly
// as it would against a bare mux.
func (rs *routes) HandleFunc(pattern string, h func(http.ResponseWriter, *http.Request)) {
	rs.Handle(pattern, http.HandlerFunc(h))
}

// ServeHTTP implements http.Handler, which the embedded field used to supply.
// The alias and the middleware wrap both hand a *routes on as a handler.
func (rs *routes) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rs.mux.ServeHTTP(w, r)
}

// API is the whole HTTP surface as one handler.
type API struct {
	deps    Deps
	handler http.Handler

	// guarded and root are the two recorders, kept whole rather than
	// snapshotted into a []string. RegisteredPatterns() reads them LIVE, so a
	// registration added anywhere in New is audited — a snapshot taken at one
	// point in that function quietly excused every route registered below it,
	// which is where tasks 11 and 12 are most likely to add theirs.
	guarded *routes
	root    *routes
}

var _ http.Handler = (*API)(nil)

// New validates deps, builds the two muxes and returns the handler task 13
// serves. It returns an error rather than panicking because every one of these
// gaps is a wiring mistake, and a wiring mistake should stop a process at
// startup with a sentence naming the field.
func New(d Deps) (*API, error) {
	for _, dep := range []struct {
		name    string
		missing bool
	}{
		{"Backend", d.Backend == nil},
		{"BackendReady", d.BackendReady == nil},
		{"Store", d.Store == nil},
		{"Flags", d.Flags == nil},
		{"Projects", d.Projects == nil},
		{"Worktrees", d.Worktrees == nil},
		{"Origin", d.Origin == nil},
	} {
		// BackendReady has no safe default and that is why it is here rather
		// than below: true fabricates health, false sends an operator after a
		// phantom, and only task 13 knows which is true.
		if dep.missing {
			return nil, errors.New("api: Deps." + dep.name + " is nil")
		}
	}
	if d.Version == "" {
		d.Version = DefaultVersion
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}

	a := &API{deps: d}

	// guarded holds every route the origin middleware protects.
	guarded := newRoutes()
	a.guarded = guarded
	a.registerCatalog(guarded)
	// Task 11 adds a.registerSessions(guarded) on the next line and task 12
	// adds a.registerAttach(guarded). One line each, each taking *routes, and
	// each in the SAME step that defines the method it names: a call added
	// ahead of its definition leaves this file uncompilable for every step in
	// between, so no suite in them can be green. The only other edit either
	// makes here is task 11's three Deps fields: Image, Resources and
	// ReadyTimeout. Both also add their read-only patterns, if any, to
	// catalogReadOnlyRoutes in routes_audit_test.go — the §10a audit covers
	// their routes automatically and fails until they do.
	//
	// The position of those two lines is a readability preference and not a
	// load-bearing one: the audit reads both recorders live, so a call added at
	// the bottom of this function is audited exactly the same. What IS
	// load-bearing is that the call goes through a *routes. Neither task owes a
	// sessionsRoutes()-style table either — catalogRoutes is a task-10 style
	// choice, and the recorder is the whole requirement.

	// Contract §7.5: /v1 is canonical and /api/v1/… is an alias, so Phase 4's
	// Discover routes and its entire test suite land unchanged. Nothing in
	// Phase 2 calls it, and that is the point — DO NOT DELETE IT AS DEAD CODE.
	// It cannot loop: /api/v1/x strips to /v1/x, which does not match this
	// pattern, and /api/api/v1/x matches no pattern at all.
	guarded.Handle("/api/v1/", http.StripPrefix("/api", guarded))

	// /healthz is mounted OUTSIDE the guard. Contract §3.1 calls it
	// unauthenticated with no origin check, for a supervisor, and a supervisor
	// addressing this process by a container name must not get 403 and restart
	// a healthy build. It leaks a status, a version, a backend name and a
	// boolean, over a listener task 13 binds to loopback. Both prefixes are
	// registered here because the alias lives on the guarded mux, which this
	// path never reaches.
	root := newRoutes()
	a.root = root
	healthz := http.HandlerFunc(a.handleHealthz)
	root.Handle("GET /v1/healthz", healthz)
	root.Handle("GET /api/v1/healthz", healthz)
	root.Handle("/", d.Origin.Middleware(guarded))

	a.handler = root
	return a, nil
}

// ServeHTTP implements http.Handler.
func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.handler.ServeHTTP(w, r)
}

// grantHealthzOrigin applies the grant half of the plan's adjudicated decision
// 1 without its rejection half. Skipping the middleware also skips its CORS
// grant, which would leave the cockpit's health dot red while curl reads green.
// The origin is echoed verbatim and never "*", and Vary: Origin is set either
// way so no interposed cache serves one origin's answer to another.
func (a *API) grantHealthzOrigin(w http.ResponseWriter, r *http.Request) {
	// Vary FIRST, and before the Origin header is even read. The response
	// differs by Origin even when this particular request carried none, and a
	// cache that does not know that will serve the supervisor's answer to the
	// cockpit. Task 9's middleware sets it unconditionally for the same reason;
	// healthz is the one route that skips the middleware, so it is the one
	// route where the guarantee has to be restated here.
	w.Header().Add("Vary", "Origin")
	origin := r.Header.Get("Origin")
	if origin == "" {
		return
	}
	if a.deps.Origin.Allow(origin) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
	}
}

// internal answers 500 with contract §3.2's deliberately generic message and
// puts the real error in the log. Tasks 11 and 12 use it too, so one place
// decides how much a caller is told — and the answer is nothing that names a
// path on disk, a container id or a token.
func (a *API) internal(w http.ResponseWriter, r *http.Request, op string, err error) {
	a.deps.Log.Error("api: "+op+" failed",
		"method", r.Method, "path", r.URL.Path, "err", err)
	httpx.WriteProblem(w, http.StatusInternalServerError, httpx.CodeInternal,
		"Something went wrong inside the control plane. The detail is in its log.", nil)
}
```

- [ ] **Step 5: Run again and watch the two missing handlers**

Run: `go test ./sandboxd/... ./control-plane/... -race -count=1`

Expected: the package now has source, so the compiler reaches the type checker and names the two methods `catalog.go` has not defined yet. Both are **method** references, so the message is the `has no field or method` form and not a bare `undefined:`; both are single use sites, so there are two errors rather than nine. The **line** depends on how the file was transcribed and is shown as `NN`; the **column** does not, because it points at the selector and the two statements have a fixed shape — `4` for the tab-indented `a.registerCatalog(guarded)`, `32` for `a.handleHealthz` inside `http.HandlerFunc(…)`:

```
# github.com/kpmg-au-emu/panacea-app/control-plane/internal/api
control-plane\internal\api\api.go:NN:4: a.registerCatalog undefined (type *API has no field or method registerCatalog)
control-plane\internal\api\api.go:NN:32: a.handleHealthz undefined (type *API has no field or method handleHealthz)
FAIL	github.com/kpmg-au-emu/panacea-app/control-plane/internal/api [build failed]
```

If a third error names `a.internal` or `a.grantHealthzOrigin`, those two were dropped from this step's transcription — they belong in `api.go`, not in `catalog.go`.

- [ ] **Step 6: Write the read-only surface**

Every body on this surface goes through a named wire type rather than a `map[string]any`, so a renamed field is a compile error here instead of a silent `undefined` in the cockpit. Three of them are not the domain type: `worktreeJSON` adds the two fields git cannot supply, `harnessJSON` adds the server-computed `available` and flattens the string-kinded slices, and `machineJSON` exists only here because no package owns a machine in Phase 2.

**No departure from adjudicated decision 2 — and the reasoning, because a first draft of this step got it wrong.** The decision says the `PATCH` answers `400 bad_request` "for any other field", and it does, unknown fields included. The draft answered `400 invalid_json` for an unknown field, on the grounds that `httpx.DecodeJSON` owns every body rule and writes that code itself. **The controller ruled against it, and the ruling generalises:** an unknown field is well-formed JSON that is semantically wrong, so telling the user their JSON was invalid is false, and a code the cockpit switches on must not say something untrue. It is the same principle that refuses `sandbox_not_running` for a sandbox that is running.

The implementation keeps one body decoder. It calls `httpx.DecodeJSON` with a `map[string]json.RawMessage`, so the 1 MiB cap, the syntax rules and the exactly-one-object rule are still task 9's and still shared, and only the question of **which fields this endpoint accepts** — always the handler's — is answered here. A body that is not JSON is still `invalid_json`; a body that parses and asks for something this endpoint cannot do is `bad_request`.

Create `control-plane/internal/api/catalog.go`:

```go
// The read-only surface: everything that needs no sandbox, no session record
// and no WebSocket. Contract §3.3 to §3.8, plus the worktree PATCH from the
// plan's adjudicated Cross-group decision 2.
package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/harness"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/project"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry"
)

// catalogRoutes is the read-only surface's route table.
//
// A table rather than a sequence of literal calls, so the patterns of one
// surface are one value a reader can take in at once. That is a style choice
// and tasks 11 and 12 need not copy it: what contract §10a's audit actually
// needs is the *routes recorder in api.go, which sees a literal
// mux.HandleFunc(…) exactly as well.
func (a *API) catalogRoutes() []route {
	return []route{
		// /healthz is NOT here: api.New mounts it on the outer mux, outside the
		// origin guard. The handler still lives in this file.
		{"GET /v1/machines", http.HandlerFunc(a.handleMachines)},
		{"GET /v1/projects", http.HandlerFunc(a.handleProjects)},
		{"GET /v1/projects/{projectId}/branches", http.HandlerFunc(a.handleBranches)},
		{"GET /v1/projects/{projectId}/worktrees", http.HandlerFunc(a.handleWorktrees)},
		{"PATCH /v1/projects/{projectId}/worktrees/{worktreeId}", http.HandlerFunc(a.handlePatchWorktree)},
		{"GET /v1/harnesses", http.HandlerFunc(a.handleHarnesses)},
	}
}

// registerCatalog mounts the read-only routes. api.New calls it; tasks 11 and
// 12 add one sibling each.
func (a *API) registerCatalog(mux *routes) {
	for _, rt := range a.catalogRoutes() {
		mux.Handle(rt.Pattern, rt.Handler)
	}
}

// nonNil and stringsOf below are surface-agnostic despite living in the
// catalog's file: tasks 11 and 12 call them for their own JSON rather than
// declaring a second copy, on the same reasoning ruling 6 applies to
// isolationFor. They move to a file of their own when a third surface lands.

// nonNil renders a nil slice as an empty one. json.Marshal writes nil as null,
// and null.map(...) in the cockpit's rail is a blank screen with a console
// error instead of the empty state docs/FUNCTIONAL-SPEC.md specifies.
func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

// stringsOf flattens a slice of a string-kinded type, never to nil. Same
// reason: harness.Effort marshals as a string, but a nil []harness.Effort
// still marshals as null.
func stringsOf[T ~string](in []T) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		out = append(out, string(v))
	}
	return out
}

type healthJSON struct {
	Status       string `json:"status"`
	Version      string `json:"version"`
	Backend      string `json:"backend"`
	BackendReady bool   `json:"backendReady"`
}

type machineJSON struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
	// State is contract §3.4's six-value enum. Phase 2 emits "running" or
	// "error" only; the other four need cloud machines, i.e. Phase 8 (§8.7).
	// The JSON field carries the full enum so the cockpit needs no change then.
	State string `json:"state"`
	// Message names the cause of an error state and is absent otherwise.
	Message     string   `json:"message,omitempty"`
	Backend     string   `json:"backend"`
	Isolation   string   `json:"isolation"`
	Default     bool     `json:"default"`
	Limitations []string `json:"limitations"`
}

// worktreeJSON is project.Worktree plus the two fields git cannot supply:
// sessionCount from the registry, and ready from the flag store.
type worktreeJSON struct {
	ID           string `json:"id"`
	ProjectID    string `json:"projectId"`
	BranchID     string `json:"branchId"`
	Name         string `json:"name"`
	Path         string `json:"path"`
	IsPrimary    bool   `json:"isPrimary"`
	SessionCount int    `json:"sessionCount"`
	Ready        bool   `json:"ready"`
}

// harnessJSON is harness.Descriptor plus available, computed server-side so
// the desktop never re-derives installed && flagsVerified and gets it wrong.
type harnessJSON struct {
	ID                    string          `json:"id"`
	Name                  string          `json:"name"`
	Command               string          `json:"command"`
	Installed             bool            `json:"installed"`
	FlagsVerified         bool            `json:"flagsVerified"`
	Available             bool            `json:"available"`
	Models                []harness.Model `json:"models"`
	DefaultModel          string          `json:"defaultModel"`
	Efforts               []string        `json:"efforts"`
	PermissionModes       []string        `json:"permissionModes"`
	SupportsOpeningPrompt bool            `json:"supportsOpeningPrompt"`
	SupportsResume        bool            `json:"supportsResume"`
	Note                  string          `json:"note"`
}

type machinesEnvelope struct {
	Machines []machineJSON `json:"machines"`
}

type projectsEnvelope struct {
	Projects []project.Project `json:"projects"`
}

type branchesEnvelope struct {
	Branches []project.Branch `json:"branches"`
}

type worktreesEnvelope struct {
	Worktrees []worktreeJSON `json:"worktrees"`
}

type harnessesEnvelope struct {
	Harnesses []harnessJSON `json:"harnesses"`
}
```

Still `control-plane/internal/api/catalog.go`, continuing below the wire types. `handleHealthz` reads a cached bool and starts nothing, which is contract §10a applied to the one route with no origin guard in front of it; `enumerationFailed` is the project-enumeration error switch written once instead of once per handler, and tasks 11 and 12 call it rather than writing a fourth copy:

```go
// handleHealthz answers GET /v1/healthz. api.New mounts it on the outer mux,
// outside the origin guard; the reasoning is in the comment there.
//
// status is "ok" whenever this process is answering, including when the
// backend is not: contract §3.3 is explicit that a supervisor restarting the
// control plane would not fix Docker Desktop being closed, so the two facts
// are two fields.
//
// It reads a cached bool and starts nothing. That is contract §10a on the one
// route with no origin guard in front of it (Group C ruling 2).
func (a *API) handleHealthz(w http.ResponseWriter, r *http.Request) {
	a.grantHealthzOrigin(w, r)
	httpx.WriteJSON(w, http.StatusOK, healthJSON{
		Status:       "ok",
		Version:      a.deps.Version,
		Backend:      string(a.deps.Backend.Kind()),
		BackendReady: a.deps.BackendReady(),
	})
}

// isolationFor is contract §3.4's three-value enum. docs/FUNCTIONAL-SPEC.md
// badges the local machine "unsandboxed"; "container" is the honest value for
// what Phase 2 actually builds, and limitationsFor says what that means.
//
// THIS IS THE PACKAGE'S ONLY MAPPING OF A BACKEND KIND TO THAT ENUM. Task 11's
// session surface calls it rather than declaring a second one: an unrecognised
// kind is "none" on every surface, and a second function that answered
// "unknown" would invent a fourth value for a three-value enum.
func isolationFor(k backend.Kind) string {
	switch k {
	case backend.KindKata:
		return "microvm"
	case backend.KindDocker:
		return "container"
	default:
		return "none"
	}
}

func limitationsFor(k backend.Kind) []string {
	switch k {
	case backend.KindKata:
		return []string{
			"Sessions run in a Kata microVM with its own kernel.",
		}
	case backend.KindDocker:
		return []string{
			"Sessions run in Docker containers on this machine, not in a microVM.",
			"A container shares this machine's kernel; isolation is weaker than a cloud sandbox.",
		}
	default:
		return []string{
			"Sessions on this machine run with no sandbox isolation.",
		}
	}
}

// handleMachines answers GET /v1/machines with the one machine Phase 2 has.
func (a *API) handleMachines(w http.ResponseWriter, _ *http.Request) {
	kind := a.deps.Backend.Kind()
	m := machineJSON{
		ID:          LocalMachineID,
		Name:        "This PC",
		Kind:        "local",
		State:       "running",
		Backend:     string(kind),
		Isolation:   isolationFor(kind),
		Default:     true,
		Limitations: limitationsFor(kind),
	}
	if !a.deps.BackendReady() {
		m.State = "error"
		m.Message = fmt.Sprintf(
			"The %s backend is not responding. Sessions cannot start on this machine until it does.", kind)
	}
	httpx.WriteJSON(w, http.StatusOK, machinesEnvelope{Machines: []machineJSON{m}})
}

// projectNotFound is the only place §3.2's project_not_found body is written.
func projectNotFound(w http.ResponseWriter, projectID string) {
	httpx.WriteProblem(w, http.StatusNotFound, httpx.CodeProjectNotFound,
		"No project with id "+projectID+".",
		map[string]string{"projectId": projectID})
}

// badRequest writes §3.2's bad_request: the body parsed, and what it asked for
// is not something this endpoint can do.
func badRequest(w http.ResponseWriter, msg string) {
	httpx.WriteProblem(w, http.StatusBadRequest, httpx.CodeBadRequest, msg, nil)
}

// enumerationFailed maps a project-enumeration error onto its response and
// reports whether it wrote one, so a caller reads as
//
//	if a.enumerationFailed(w, r, "enumerate branches", projectID, err) { return }
//
// Three handlers here need it and tasks 11 and 12 will need a fourth and fifth:
// one function is what stops project_not_found being spelled two ways.
func (a *API) enumerationFailed(w http.ResponseWriter, r *http.Request, op, projectID string, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, project.ErrUnknownProject):
		// errors.Is, never a string match: task 8 owns the sentinel and its
		// message is free to change.
		projectNotFound(w, projectID)
	default:
		a.internal(w, r, op, err)
	}
	return true
}

// handleProjects answers GET /v1/projects. An unconfigured projects directory
// is 200 and an empty list, not an error: contract §3.5, and empty is a state
// the cockpit renders.
func (a *API) handleProjects(w http.ResponseWriter, r *http.Request) {
	ps, err := a.deps.Projects.Projects(r.Context())
	if err != nil {
		a.internal(w, r, "enumerate projects", err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, projectsEnvelope{Projects: nonNil(ps)})
}

// handleBranches answers GET /v1/projects/{projectId}/branches.
func (a *API) handleBranches(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	bs, err := a.deps.Projects.Branches(r.Context(), projectID)
	if a.enumerationFailed(w, r, "enumerate branches", projectID, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, branchesEnvelope{Branches: nonNil(bs)})
}
```

Still `control-plane/internal/api/catalog.go`, the worktree block — the fix for known defect 9. `readyFromBody` reads the body as raw fields on purpose: `httpx.DecodeJSON` keeps every **body** rule in the phase — the 1 MiB cap, the syntax, the exactly-one-object probe — and this endpoint decides only which **fields** it accepts, so an unknown field is `bad_request` rather than a `invalid_json` that would tell the user something untrue about JSON that parsed perfectly well:

```go
// decoratedWorktrees enumerates one project's worktrees and adds the two
// fields git cannot supply. It reads the registry ONCE and the flag store ONCE
// for the whole response rather than once per worktree: contract §8.8 notes
// the per-worktree scan stops being free at hundreds of sessions, and one map
// makes the request O(n+m) instead.
func (a *API) decoratedWorktrees(r *http.Request, projectID string) ([]worktreeJSON, error) {
	ws, err := a.deps.Projects.Worktrees(r.Context(), projectID)
	if err != nil {
		return nil, err
	}
	sessions, err := a.deps.Store.List()
	if err != nil {
		// NOT a zero count. Reporting sessionCount 0 for a registry that could
		// not be read is a fabricated number; the caller turns this into a 500.
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	live := make(map[string]int, len(ws))
	for _, s := range sessions {
		if s.Status == registry.StatusExited {
			// Contract §3.7: a count of live things the user owns, not a
			// history and not a progress metric.
			continue
		}
		live[s.WorktreeID]++
	}
	flags := a.deps.Flags.WorktreeFlagsSnapshot()

	out := make([]worktreeJSON, 0, len(ws))
	for _, wt := range ws {
		out = append(out, worktreeJSON{
			ID:           wt.ID,
			ProjectID:    wt.ProjectID,
			BranchID:     wt.BranchID,
			Name:         wt.Name,
			Path:         wt.Path,
			IsPrimary:    wt.IsPrimary,
			SessionCount: live[wt.ID],
			// A worktree the registry has never seen is not an error; it is a
			// worktree that is not ready, which is what the zero value says.
			Ready: flags[wt.ID].Ready,
		})
	}
	return out, nil
}

// handleWorktrees answers GET /v1/projects/{projectId}/worktrees[?branchId=].
func (a *API) handleWorktrees(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	ws, err := a.decoratedWorktrees(r, projectID)
	if a.enumerationFailed(w, r, "enumerate worktrees", projectID, err) {
		return
	}
	if branchID := r.URL.Query().Get("branchId"); branchID != "" {
		kept := make([]worktreeJSON, 0, len(ws))
		for _, wt := range ws {
			if wt.BranchID == branchID {
				kept = append(kept, wt)
			}
		}
		ws = kept
	}
	httpx.WriteJSON(w, http.StatusOK, worktreesEnvelope{Worktrees: nonNil(ws)})
}

// readyFromBody parses this endpoint's only body. It returns the requested flag
// and whether the handler should continue; every refusal is a 400 it has
// already written.
//
// THE BODY IS READ AS RAW FIELDS rather than straight into a struct with
// DisallowUnknownFields, and that is the whole of the reason: DecodeJSON answers
// invalid_json for a field it does not recognise, and {"ready":true,"name":"x"}
// is well-formed JSON that is semantically wrong. A code the cockpit switches on
// must not say something untrue — the same principle that refuses
// sandbox_not_running for a sandbox that is running. Contract §3.7 says
// bad_request, so bad_request is what this answers.
//
// DecodeJSON still owns every BODY rule — the 1 MiB cap, the syntax, the
// exactly-one-object rule — because the map goes through it too, so the phase
// still has one body decoder. What moved here is which FIELDS this endpoint
// accepts, which was always the handler's to decide.
func readyFromBody(w http.ResponseWriter, r *http.Request) (ready bool, ok bool) {
	var fields map[string]json.RawMessage
	if !httpx.DecodeJSON(w, r, &fields) {
		return false, false // DecodeJSON has written its own 400 invalid_json.
	}

	unknown := make([]string, 0, len(fields))
	for name := range fields {
		if name != "ready" {
			unknown = append(unknown, strconv.Quote(name))
		}
	}
	if len(unknown) > 0 {
		// Sorted, because map order is random and an error message that changes
		// between identical requests is one nobody trusts.
		slices.Sort(unknown)
		badRequest(w, "This endpoint sets one field, ready. It does not accept "+
			strings.Join(unknown, ", ")+".")
		return false, false
	}

	// Absent and null are one case: neither asks for anything, and a PATCH that
	// silently did nothing is worse than one that says so.
	raw, present := fields["ready"]
	if !present || string(raw) == "null" {
		badRequest(w, `This endpoint sets one field. Send {"ready": true} or {"ready": false}.`)
		return false, false
	}
	if err := json.Unmarshal(raw, &ready); err != nil {
		badRequest(w, "The ready field must be true or false.")
		return false, false
	}
	return ready, true
}

// handlePatchWorktree is the whole of the fix for known defect 9: nothing ever
// set ready, so user-created worktrees could never merge. The flag persists
// through registry.WorktreeFlagStore and NOT through Store, which contract
// §2.8 keeps at five session methods.
//
// This route is not in contract §3.1's table; it is the plan's adjudicated
// Cross-group decision 2. PATCH and not GET, per §10a: it is the one route on
// this surface that changes anything.
func (a *API) handlePatchWorktree(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	worktreeID := r.PathValue("worktreeId")

	ready, ok := readyFromBody(w, r)
	if !ok {
		return // readyFromBody has written the 400.
	}

	current, err := a.decoratedWorktrees(r, projectID)
	if a.enumerationFailed(w, r, "enumerate worktrees", projectID, err) {
		return
	}

	idx := -1
	for i, wt := range current {
		if wt.ID == worktreeID {
			idx = i
			break
		}
	}
	if idx < 0 {
		// Membership is checked against the project in the path on purpose.
		// Without it the path segment is decoration and any project id does.
		httpx.WriteProblem(w, http.StatusNotFound, httpx.CodeWorktreeNotFound,
			"No worktree with id "+worktreeID+" in project "+projectID+".",
			map[string]string{"projectId": projectID, "worktreeId": worktreeID})
		return
	}

	flags, err := a.deps.Flags.SetWorktreeReady(worktreeID, ready)
	if err != nil {
		a.internal(w, r, "set the worktree ready flag", err)
		return
	}

	out := current[idx]
	out.Ready = flags.Ready
	// The worktree object itself, not an envelope: adjudicated decision 2.
	httpx.WriteJSON(w, http.StatusOK, out)
}
```

Still `control-plane/internal/api/catalog.go`, the last block:

```go
// handleHarnesses answers GET /v1/harnesses, the picker's data.
//
// The catalogue is not injected through Deps. Contract §2.9 makes it a
// package-level table, and a test against an injected catalogue would prove
// nothing about the one the product actually serves.
func (a *API) handleHarnesses(w http.ResponseWriter, _ *http.Request) {
	cat := harness.Catalogue()
	out := make([]harnessJSON, 0, len(cat))
	for _, d := range cat {
		out = append(out, harnessJSON{
			ID:            string(d.ID),
			Name:          d.Name,
			Command:       d.Command,
			Installed:     d.Installed,
			FlagsVerified: d.FlagsVerified,
			// Computed here so the desktop never re-derives it, and so an
			// unverified harness is rendered disabled with its note rather
			// than started with guessed flags.
			Available:             d.Available(),
			Models:                nonNil(d.Models),
			DefaultModel:          d.DefaultModel,
			Efforts:               stringsOf(d.Efforts),
			PermissionModes:       stringsOf(d.PermissionModes),
			SupportsOpeningPrompt: d.SupportsOpeningPrompt,
			SupportsResume:        d.SupportsResume,
			Note:                  d.Note,
		})
	}
	httpx.WriteJSON(w, http.StatusOK, harnessesEnvelope{Harnesses: out})
}
```

- [ ] **Step 7: Run the suite and watch it pass**

Run: `go test ./sandboxd/... ./control-plane/... -race -count=1`

Expected: `internal/api` now reports `ok`, and so does every other package in both modules. The timing varies; the shape does not:

```
ok  	github.com/kpmg-au-emu/panacea-app/control-plane/internal/api	5.936s
```

Three failures worth recognising on sight, because each one is a real defect rather than a typo. **`worktrees is []interface {}, want an array`** is impossible — but `worktrees is <nil>, want an array` means `nonNil` was dropped and a nil slice marshalled as `null`, which is the empty-rail bug. **`/api/v1/machines status = 404`** means the alias line was written against a fresh mux rather than `guarded` itself, so the strip re-dispatches into a mux with nothing on it. And **`sessionCount = 3, want 2`** with the exited-session test also failing means the status filter compares against the wrong constant — `registry.StatusExited` is the only one excluded; `idle` and `starting` both count, because an idle agent is a live thing the user owns.

If instead every test in the package fails with a 403 and `forbidden_host`, a request was built with a path-only target somewhere: `httptest.NewRequest` then sets `Host: example.com` and task 9's guard is doing exactly its job. Every target in this package goes through `catalogDo`, which prepends `catalogBaseURL`.

- [ ] **Step 8: Vet and format**

Run, from the repository root:

```bash
gofmt -l control-plane/internal/api
go vet ./control-plane/internal/api/...
```

Expected: `gofmt -l` prints nothing and `go vet` prints nothing. `gofmt` has an opinion about the alignment of the struct tags in `harnessJSON` and `machineJSON`; take its version rather than arguing with it. `go vet`'s `structtag` check is the one that matters here — it catches a JSON tag with a space after the colon, which silently names the field `"id"` with a leading space and is invisible in review.

There is no manual `curl` step in this task, and that is not an oversight: `cmd/api/main.go` does not yet build an `api.API` — task 13 wires it — so nothing serves these routes outside a test. Task 13's verification step is the first place this surface can be curled, and its plan should exercise `/v1/healthz` and `/v1/harnesses` against the real binary.

- [ ] **Step 9: Commit**

Four files, all new, all under `control-plane/internal/api`. Nothing outside that directory changes: `go.mod` is untouched because every import is either the standard library or a package Groups B and C already produced, and `control-plane` is not vendored.

```bash
git add control-plane/internal/api/api.go control-plane/internal/api/api_test.go \
        control-plane/internal/api/catalog.go control-plane/internal/api/catalog_test.go
git commit -m "feat(api): the router and the read-only surface

api.go owns Deps, New, the mux and the /api/v1 alias; catalog.go owns the six
read-only endpoints. Splitting them is contract 1.2 correction 1: tasks 10, 11
and 12 all write into this package, and one file guarantees three collisions in
the same region.

The worktree ready flag ships here and is the whole of the fix for known defect
9 — nothing ever set it, so user-created worktrees could never merge. GET
.../worktrees reports ready on every worktree and PATCH
.../worktrees/{worktreeId} sets it, persisting through
registry.WorktreeFlagStore rather than through Store, which stays at the five
session methods contract 2.8 declares.

Six decisions the contract left open. /healthz is mounted outside the origin
guard, because 3.1 calls it unauthenticated with no origin check and a
supervisor addressing this process by a container name must not get a 403 and
restart a healthy build; it still echoes an allowlisted Origin, because
skipping the middleware also skips its CORS grant, and it sets Vary: Origin
whether or not the request carried one, because the supervisor's answer and
the cockpit's differ and a shared cache must not confuse them. backendReady
comes from a func() bool on Deps rather than from a Ping backend.Backend does
not have, and New refuses a nil one because both defaults lie. An unrouted
path gets net/http's plain-text 404, because the sixteen error codes - 3.2's
fifteen plus ruling 7's non_loopback_publish - are a closed enum with no
member meaning 'no such route'. sessionCount is computed from one Store.List
and one flag snapshot for the whole response rather than one scan per
worktree, which 8.8 names as the thing that stops being free at hundreds of
sessions. And a Store.List failure is a 500 rather than sessionCount 0,
because a count nobody could read is a fabricated number.

The PATCH answers 400 bad_request for every body it refuses, unknown fields
included, per adjudicated decision 2: an unknown field is well-formed JSON that
is semantically wrong, and a code the cockpit switches on must not say something
untrue. A body that is not JSON is still invalid_json, because httpx.DecodeJSON
still owns every body rule in the phase - the handler reads raw fields through
it and decides only which fields this endpoint accepts.

Every test double this package declares is prefixed catalog, per the Group C
package-API contract, so tasks 11 and 12 can add sessions* and attach* doubles
beside them with no rename and no cross-file coordination.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```
