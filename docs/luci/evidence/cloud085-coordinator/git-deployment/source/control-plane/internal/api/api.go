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
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/assistance"
	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/httpx"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/project"
	"github.com/lcfrai/luci/control-plane/internal/registry"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
	"github.com/lcfrai/luci/control-plane/internal/worktree"
)

// DefaultVersion is what /healthz reports when Deps.Version is empty.
const DefaultVersion = "0.3.0"

// LocalMachineID is the id of the one machine Phase 2 has. Cloud machines are
// claimed from a warm pool at first sign-in, which is Phase 8.
const LocalMachineID = "local"

func (a *API) projectsDir() string {
	if a.deps.ProjectsDirCurrent != nil {
		return a.deps.ProjectsDirCurrent()
	}
	return a.deps.ProjectsDir
}

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
	// ObserveTaskCompletions enables trusted task/check completion observation
	// and confirmed-absent manual capacity reconciliation.
	// Tests and embedded consumers may drive ObserveTaskSessionsOnce explicitly.
	ObserveTaskCompletions bool

	ExecutionStateDir         string
	ResourceSnapshot          func() resourcebudget.Snapshot
	AuthorizeExecution        management.ExecutionAuthorizer
	AuthorizeChecks           management.CheckAuthorizer
	AuthorizeExecutionControl management.ExecutionControlAuthorizer
	AuthorizeDeployment       management.DeploymentAuthorizer
	AuthorizeRuntimeInstaller management.RuntimeInstallerAuthorizer
	ManagedExecution          bool
	ExecutionReady            func() bool
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

	// ProjectsDir is the host directory the rail enumerates. It is here so that
	// a file the host's own git config names can be refused when reaching it
	// passes through a project (X-3, X-1 review m-5): every project is under
	// this directory, and a session mounts its worktree read-write, so a link
	// hop there is the agent's to swap.
	//
	// It is a path, not a Provider, because the question is about a region of
	// the filesystem rather than about resolution, and because the resolver it
	// guards never sees a worktree id at all.
	//
	// It is not the whole of "a sandbox can write it": a linked worktree the
	// project registers may sit OUTSIDE this directory, and it is mounted
	// read-write too (desktop/e2e/fixture.sh puts one there on purpose). That
	// residue is the X-1 reviewer's, recorded in X-3, and R4 rule 1 closes it in
	// phase 3. "" is an unconfigured projects directory and refuses nothing.
	ProjectsDir        string
	ProjectsDirCurrent func() string
	Product            *product.Server

	// Origin is the allowlist and the loopback Host guard, from task 9.
	Origin *httpx.OriginPolicy

	// Log is where a 500's real cause goes. Nil means slog.Default().
	Log *slog.Logger

	// RecordFailure optionally retains bounded, classified host diagnostics.
	// It must not expose raw errors or request data. Recording failure never
	// replaces the original HTTP error; ordinary reads still change no product state.
	RecordFailure func(operation string, cause error) error

	// Image is the sandbox image every session runs, e.g.
	// "luci/sandbox:dev". Empty means that default.
	Image string

	// Resources sizes every sandbox. The zero value means
	// backend.DefaultResources.
	Resources backend.Resources

	// ReadyTimeout bounds the post-create readiness watch of contract §5.1
	// step 7. Zero means backend.ReadyTimeout. Task 13 feeds it from
	// LUCI_READY_TIMEOUT.
	ReadyTimeout time.Duration

	// ConversationsDir is the host directory every worktree's conversation
	// store lives under — see sessionConversationStore for the layout and
	// sessionConversationDir for what is mounted where. Without it a sandbox's
	// transcripts die with the sandbox and no resume can find them, so it is
	// required, and absolute because it becomes a bind mount's host path.
	// Task 13's wiring derives it from the state directory.
	ConversationsDir string

	// GitIdentity resolves who an agent's commits are made as. Sessions call it
	// at every create and restart and hand the answer to the sandbox as
	// GIT_AUTHOR_* and GIT_COMMITTER_*. Users authenticate as themselves, so
	// this is the person running the cockpit and never a shared or invented
	// identity; cmd/api's resolver reads LUCI_GIT_NAME/EMAIL, else the
	// host's own git outside any repository.
	//
	// None of the three git resolvers is given a worktree (X-1, phase 3 ruling
	// R4 rule 6). A worktree's .git/config is in its sandbox's mount and the
	// agent writes it, so an answer read there is the agent's choice: its
	// core.excludesFile named an SSH key, and the session mounted the key. The
	// answers come from host-owned config only, the same for every session.
	//
	// Nil, or an error, means no identity: the session still starts, the error
	// is logged as the sentence the user acts on, and the sandbox's git refuses
	// to commit rather than guessing (see sessionEnv's user.useConfigOnly).
	GitIdentity func(ctx context.Context) (GitIdentity, error)

	// GitLineEndings resolves how the HOST's git normalises line endings, from
	// its own system and global config: core.autocrlf, and the ending core.eol
	// means there. Sessions call it at every create and restart and hand the
	// answer to the sandbox as git configuration.
	//
	// It is required, and an error fails the create or restart before anything
	// is recorded or destroyed, because there is no safe guess. The host's
	// system and global config are invisible to the sandbox, and Git for
	// Windows' installer sets core.autocrlf=true at system scope. A sandbox
	// that guessed "false" there sees every CRLF file as modified, and one
	// commit -a writes CRLF blobs for files the agent never touched.
	GitLineEndings func(ctx context.Context) (GitLineEndings, error)

	// GitGlobalFiles resolves the two files the HOST's git reads beyond its
	// config, as its own system and global config name them: the global
	// excludes file and the global attributes file, as host paths
	// (gitfix-rereview I-A). Every absolute one that is a file, and is not
	// reached through the projects directory, is mounted, so no worktree's
	// config may name one (X-1; AMENDED by X-3, was "Every one is mounted",
	// which contradicted the sentence below it once X-1 review m-4 landed and
	// is wrong again for X-1 review m-5). Sessions call it at
	// every create and restart, and sessionEnv hands the sandbox's git the
	// ones that exist, mounted read-only, a relative one inside the
	// repository as it is, and /dev/null for the rest.
	//
	// It is required, and an error fails the create or restart before anything
	// is recorded or destroyed, like GitLineEndings. Git reads
	// $XDG_CONFIG_HOME/git/ignore (else ~/.config/git/ignore) even with
	// core.excludesFile unset, and the sandbox's HOME has none: without this
	// an agent's `git add -A` commits every file the host's git ignores there,
	// .claude/settings.local.json among them, and that is a user's personal
	// permission allow-list.
	GitGlobalFiles func(ctx context.Context) (GitGlobalFiles, error)

	// CABundle is the absolute host path of a PEM bundle every sandbox trusts in
	// addition to its own roots, mounted read-only at sessionCABundlePath and
	// named by NODE_EXTRA_CA_CERTS. "" mounts nothing and sets nothing. cmd/api
	// has already refused a path that is missing or holds no certificate.
	CABundle string
}

// GitIdentity is one person's commit identity: both fields, or neither.
type GitIdentity struct {
	Name  string
	Email string
}

// GitLineEndings is how the host's own config says git converts line endings,
// in the two words the sandbox's git needs to do the same.
//
//   - AutoCRLF is core.autocrlf as git itself reads it: "true", "false" or
//     "input".
//   - EOL is the ending a text file gets on checkout when core.autocrlf is
//     false: "lf" or "crlf". It is never "native", because native means CRLF
//     to Git for Windows and LF to the sandbox's Linux git. The resolver
//     turns it into the host's own answer.
type GitLineEndings struct {
	AutoCRLF string
	EOL      string
}

// GitGlobalFiles is where the host's own config says git looks for the two
// per-user files that change what a commit contains:
//
//   - Excludes is core.excludesFile, else git's own default,
//     $XDG_CONFIG_HOME/git/ignore or ~/.config/git/ignore.
//   - Attributes is core.attributesFile, else $XDG_CONFIG_HOME/git/attributes
//     or ~/.config/git/attributes.
//
// Each names the file git would read whether or not one is there: git skips
// a missing file silently, and sessions do the same. An absolute host path is
// mounted when it is a file. A relative one is git's own answer, read against
// each repository: inside the repository it goes to the sandbox unmounted,
// and outside it fails the create (X-1 review m-4). The resolver answers the
// question; sessions look at the file.
type GitGlobalFiles struct {
	Excludes   string
	Attributes string
	// Origins are host config files a mounted value's provenance depends on:
	// the file that SET an absolute Excludes or Attributes, read with git's own
	// -z --show-origin so an [include] the host config pulls in is named as the
	// real source, raw and never C-quoted (X-3 review 4 I-C). A session refuses
	// the create unless every one is PROVED outside the projects directory — an
	// origin that is not absolute, that the walk cannot follow, that is reached
	// through the projects directory, or that has a second (hard-linked) name
	// fails it (sessionGitConfigOrigins; X-3 review 3 I-C): --includes is on by design
	// (X-1), so under m-5's own premise — a dotfiles repository that is also a
	// project — the line that NAMES a host file can live inside a project even
	// when the named file does not. m-5 guards the named file's path; this
	// guards the provenance of the line that names it. Only an absolute value's
	// origin is here: a relative value names a file in each repository, not a
	// host file, and an unset value's origin is git's command line, not a file.
	Origins []string
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
	sessionPreparations     sessionPreparationSet
	taskObservations        taskCompletionObservations
	execution               *executionAdmission
	deps                    Deps
	handler                 http.Handler
	codexAccountMu          sync.Mutex // serializes this user's retained account and private snapshots
	assistMu                sync.Mutex
	orchestratorAdmissionMu sync.Mutex
	assistRecoveryOnce      sync.Once
	assistStore             *assistance.Store
	assistCancels           map[string]context.CancelFunc

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
		{"GitLineEndings", d.GitLineEndings == nil},
		{"GitGlobalFiles", d.GitGlobalFiles == nil},
	} {
		// BackendReady has no safe default and that is why it is here rather
		// than below: true fabricates health, false sends an operator after a
		// phantom, and only task 13 knows which is true.
		if dep.missing {
			return nil, errors.New("api: Deps." + dep.name + " is nil")
		}
	}
	// Not in the table above because it is a string, not a nil: the sentence
	// has to say which of the two ways it is wrong.
	if d.ConversationsDir == "" {
		return nil, errors.New("api: Deps.ConversationsDir is empty; every sandbox's transcripts would die with it and no resume could find them")
	}
	if !filepath.IsAbs(d.ConversationsDir) {
		return nil, errors.New("api: Deps.ConversationsDir " + strconv.Quote(d.ConversationsDir) + " is not absolute, and it becomes a bind mount's host path")
	}
	if d.CABundle != "" && !filepath.IsAbs(d.CABundle) {
		return nil, errors.New("api: Deps.CABundle " + strconv.Quote(d.CABundle) + " is not absolute, and it becomes a bind mount's host path")
	}
	if d.Version == "" {
		d.Version = DefaultVersion
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}

	if d.Product != nil {
		owned := &swarmWorktrees{base: d.Projects, resolver: d.Worktrees, owned: d.Product.ProjectSwarmWorktrees, private: d.Product.IsProjectSwarmPath}
		d.Projects, d.Worktrees = owned, owned
	}
	a := &API{deps: d}
	a.execution = newExecutionAdmission(a)
	if d.Product != nil {
		d.Product.SetPreviewRuntime(a.previewRuntime())
		d.Product.SetRuntimeInstaller(a.runtimeInstaller())
		d.Product.SetProjectSwarmRecovery(a.proveSwarmMissingChild, a.finalizeSwarmMissingChild)
		d.Product.SetProjectSwarmFenceRecovery(a.projectSwarmFenceRecoveryAvailability, a.authorizeSwarmFenceRecovery, a.acquireSwarmFenceRecovery, a.proveSwarmFencedLaunch, a.finalizeSwarmFencedLaunch)
		d.Product.SetProjectSwarmPendingLaunch(a.projectSwarmPendingLaunch)
		d.Product.SetProjectSwarmCheckpointGuard(a.authorizeTaskCheckpoint, a.acquireStoppedTaskOutput)
		d.Product.SetTaskHandoffCollector(a.authorizeTaskOutput, a.collectTaskHandoff)
		d.Product.SetRuntimeResourceSnapshot(d.ResourceSnapshot)
		a.deps.Product.SetExternalHandoffAuthorizer(a.authorizeExecutionRetirement)
		d.Product.SetCoordinatorAuthorizer(a.authorizeExecutionRetirement)
		d.Product.SetCoordinatorExecutionAuthorizer(a.authorizeCoordinatorExecution)
		d.Product.SetCoordinatorDeploymentAuthorizer(a.authorizePreview)
		checkRuntime := product.ProjectCheckRuntime{AcknowledgeCompleted: a.acknowledgeProjectCheckResults, CaptureActor: a.captureCheckActor, AuthorizeControl: a.authorizeExecutionRetirement, Execute: a.executeProjectCheck, Busy: a.projectCheckBusy, Recovery: a.projectCheckRecovery, Recover: a.recoverProjectCheck}
		if d.ObserveTaskCompletions {
			checkRuntime.Observe = a.ObserveChecksOnce
		}
		d.Product.SetProjectCheckRuntime(checkRuntime)
		d.Product.SetProjectRunSelectionResolver(a.ResolveProjectRunSelection)
		d.Product.SetProjectRunSessionObserver(a.ObserveProjectRunSessions)
		var observeTasks func(context.Context) error
		if d.ObserveTaskCompletions {
			observeTasks = a.observeExecutionCompletions
		}
		d.Product.SetSwarmSchedulerRuntime(product.SwarmSchedulerRuntime{ObserveTasks: observeTasks, CaptureActor: a.captureQueuedActor, Admit: a.admitQueuedSwarm, Dispatch: a.dispatchQueuedSwarm, Release: a.execution.release, Resources: a.queuedResourceView})
		d.Product.SetProjectSwarmWorktreeObserver(a.observeSwarmWorktree)
		d.Product.SetCoordinatorRuntime(a.reauthorizeCoordinator)
	}

	// guarded holds every route the origin middleware protects.
	guarded := newRoutes()
	a.guarded = guarded
	a.registerCatalog(guarded)
	if d.Product != nil {
		d.Product.Register(guarded)
	}
	a.registerSessions(guarded)
	a.registerCodexConnection(guarded)
	a.registerAssistance(guarded)
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

// internal answers 500 with a generic recovery message and reports the error
// to the configured logger. A private worker may discard its output, so the
// response must not promise a persisted log or reveal a path, container or token.
func (a *API) internal(w http.ResponseWriter, r *http.Request, op string, err error) {
	if a.deps.RecordFailure != nil {
		_ = a.deps.RecordFailure(op, err)
	}
	a.deps.Log.Error("api: "+op+" failed",
		"method", r.Method, "path", r.URL.Path, "err", err)
	httpx.WriteProblem(w, http.StatusInternalServerError, httpx.CodeInternal,
		"Luci couldn't complete this request. Please try again.", nil)
}
