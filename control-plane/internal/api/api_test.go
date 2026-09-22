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

func TestEveryContractRouteIsRegistered(t *testing.T) {
	// Contract 3.1's table, restricted to the rows this task owns, plus the
	// PATCH from adjudicated decision 2. A 404 here means a missing
	// registration; a 405 means the method in the pattern is wrong.
	h := catalogNewAPI(t, nil)
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
			rec := catalogDo(t, h, tc.method, tc.target, tc.body, nil)
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

// catalogReadOnlyRoutes is the explicit allowlist Group C ruling 1 asks for: every
// GET and HEAD pattern this package registers must appear here, beside the reason
// it is safe to serve with no CSRF defence at all.
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
	// maintained by hand, so tasks 11 and 12's routes are audited the moment
	// they are added, with no change to this test.
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
