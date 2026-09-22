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
	"time"

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

	// Image is the sandbox image every session runs, e.g.
	// "panacea-app/sandbox:dev". Empty means that default.
	Image string

	// Resources sizes every sandbox. The zero value means
	// backend.DefaultResources.
	Resources backend.Resources

	// ReadyTimeout bounds the post-create readiness watch of contract §5.1
	// step 7. Zero means backend.ReadyTimeout. Task 13 feeds it from
	// PANACEA_APP_READY_TIMEOUT.
	ReadyTimeout time.Duration
}

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
	a.registerSessions(guarded)
	a.registerAttach(guarded)
	// Task 12 added a.registerAttach(guarded) on the line above. One line, taking
	// *routes, and in the SAME step that defines the method it names: a call
	// added ahead of its definition leaves this file uncompilable for every
	// step in between, so no suite in them can be green. The only other edit
	// task 11 made here is its three Deps fields: Image, Resources and
	// ReadyTimeout.
	//
	// ANY LATER TASK REGISTERING A GET OR HEAD owes it a line in
	// routes_audit_test.go, and there are now TWO maps to choose between — the
	// §10a audit covers the route automatically and fails until one of them
	// names it:
	//
	//   - catalogReadOnlyRoutes, if the handler only reads. This is the usual
	//     case and the one to reach for.
	//   - catalogUpgradeGatedRoutes, if it writes but every write sits behind a
	//     WebSocket upgrade. Contract §10a.1 is the ruling that created this
	//     category, and it is conditional: the entry is only true while the
	//     writes follow the gates, so it costs a test pinning that ordering as
	//     well as a line. Task 12's attach route is the only member today.
	//
	// A handler that writes and is NOT upgrade-gated belongs in neither map: it
	// needs POST, PATCH or DELETE, or a Phase 8 token. Adding it to the
	// read-only map to quiet the audit is the specific mistake §10a.1 rejected,
	// because that map is what a later author reads to learn what is read-only.
	//
	// The position of that line is a readability preference and not a
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
