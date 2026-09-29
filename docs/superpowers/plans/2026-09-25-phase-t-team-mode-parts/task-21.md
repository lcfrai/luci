### Task 21: `fleetbe` and the team launch on the local control plane

The desktop is the portal (OD-20), and phase 2's cockpit already drives every sandbox through the
seven methods of `backend.Backend` (`control-plane/internal/backend/backend.go:337-345`, seam
S8.1). `fleetbe` is the fourth implementation, beside `dockerbe`, `katabe` and phase 8's
`remotebe`, and **the interface does not change**. It calls Fleet (task 12's chassis, task 20's
sessions) as the signed-in user, with a token from the Hub's token custody
(`hubclient.TokenSource`, `.claude/worktrees/hub/control-plane/internal/hub/hubclient/hubclient.go:19-21`,
built; satisfied by `entraauth` in Hub task 31, planned). `teamlaunch.go` is the other half: on a
team project, a launch first creates a delegation **as the user** in the Hub (task 06's
`POST /v1/delegations`), then asks Fleet for the session under it (§2.4 item 2).

Three things phase 2's code assumes that are false for a Fleet session, and how this task handles
each without adding an eighth method. All three are the same answers phase 8 task 22 gives for
`remotebe` (`.claude/worktrees/env/docs/superpowers/plans/2026-09-23-phase-8-own-environment-parts/task-22.md:11-20`),
and this task reuses its types rather than inventing a second set:

1. **`Endpoint` is dialable.** `attach.go:182` refuses an empty `Endpoint`, and
   `watchSessionReady` (`sessions.go:1143`) polls its `/healthz`. A Kata pod in Australia East is
   not reachable from a laptop, so `fleetbe` reports an **opaque, non-empty** `Endpoint`,
   `fleet:<ag-id>`, never dialled, and implements phase 8's optional `backend.ReadyWaiter`
   (`control-plane/internal/backend/ready.go`, phase 8 task 22, **UNVERIFIED until merge**), which
   phase 8 task 23 teaches `watchSessionReady` to prefer.
2. **The reaper owns what `List` returns.** Phase 2's reaper destroys a sandbox that `List`
   returns and the registry does not know. Fleet itself creates the Orchestrator (task 20) and an
   Orchestrator creates workers (`panacea worker spawn`, task 24). Neither is in this laptop's
   registry. If `fleetbe.List` returned them, **the first reaper sweep would destroy the user's
   Orchestrator**. So `List` returns only agents whose `instance` equals this control plane's own
   instance id, and an agent with no instance (every Fleet-spawned one) never appears. The
   desktop sees those through the team read model (task 52), not through `Backend`.
3. **`Create` has nowhere to put a delegation.** `backend.Spec` carries no project, class or
   delegation. `teamlaunch.go` puts a typed `fleetbe.Launch` on the context with
   `fleetbe.WithLaunch`, and `Create` without one refuses with `ErrUnsupported`. A context value
   is the only route that leaves S8.1 at seven methods; the key is unexported, so nothing outside
   the package can forge or read it.

Two rules from phase 2 hold unchanged. **No token reaches the session JSON** (`sessions.go:586-593`,
the `sessionJSON` rule): Fleet's bearer token lives only in the `Authorization` header of each
request, and `Spec.AttachToken`, which `ValidateSpec` still requires, is **never sent to Fleet**,
because Fleet authorises every attach per request by the user's own Entra token (global
constraint 11), not by a secret the laptop chose. And **the owner comes from the token**: no
request body `fleetbe` sends carries an owner, `tid` or `oid` (global constraint 3).

A session still running locally on a team project is labelled *outside the team* (§2.3): it holds
no leases and others see its work only once it is pushed. This task adds the label to
`sessionJSON` as `teamScope` (`""` solo project, `"team"` a Fleet session, `"outside"` a local
session on a team project). The desktop's `types.ts` gains the field in task 52, which owns ST.8.

**Files:**
- `control-plane/internal/backend/fleetbe/fleetbe.go` — the `Backend`, `Launch`, `WithLaunch`, the
  consumed Fleet REST shape, error mapping (new)
- `control-plane/internal/backend/fleetbe/attach.go` — the WebSocket dial to Fleet's attach route (new)
- `control-plane/internal/backend/fleetbe/fleetbe_test.go`, `fakefleet_test.go` (new)
- `control-plane/internal/api/teamlaunch.go` — `TeamLauncher`, the delegation renewer, `teamScope` (new)
- `control-plane/internal/api/teamlaunch_test.go` (new)
- `control-plane/internal/api/sessions.go` — `sessionJSON` gains `teamScope`; the create handler
  routes a team project's launch to `TeamLauncher` (modify, two anchored hunks)
- `control-plane/internal/api/catalog.go` — `isolationFor` maps `fleetbe.Kind` to `"microvm"` (modify)
- `control-plane/internal/registry/session.go` — `Delegation` and `DelegationExpiresAt`, persisted,
  never on the wire (modify)
- `control-plane/internal/registry/teamdelegations.go` — the per-project orchestrator delegation
  record and its store (new; b07, S02-C7/S03-C1)
- `control-plane/internal/api/teamrelay.go` + `teamrelay_test.go` — the four `/api/team/*` human
  relays of step 6 (new; b07, G33-1/S04-I8)
- `control-plane/internal/api/teammode.go` + `teammode_test.go` — `hubTeamProjects` and the
  `POST /v1/projects/{projectId}/team-mode` route (new; fix batch b01, X-C1)

**Interfaces:**
- Produces: `fleetbe.New(Config) (*Backend, error)`; `fleetbe.Kind` (`backend.Kind("fleet")`,
  declared in this package so `backend.go` is not touched); `fleetbe.Launch`, `fleetbe.WithLaunch`;
  `fleetbe.FleetEndpoint(agID)`; `fleetbe.Error`; `api.TeamLauncher`, `api.TeamProjects` (with
  `MakeTeam`, b01); the local team-mode route (task 33's E5 and task 56's Share call it); the
  `teamScope` field of `sessionJSON`; `api.TeamLauncher.EnsureOrchestratorDelegation` and the
  registry's `TeamDelegations` record (b07); the four `/api/team/*` human relays (b07; task 33's E3
  calls them, task 52 extends them).
- Consumes: `backend.Backend`, `backend.Spec`, `backend.Sandbox` and the sentinels (phase 2,
  built); `backend.ReadyWaiter` (phase 8 task 22, planned); `hubclient.TokenSource` (built) and
  task 08's `hubclient.Team` methods `CreateDelegation`, `RenewDelegation`, `RevokeDelegation`,
  `SetProjectMode` (task 08, this plan); `hubapi.Delegation`, `hubapi.CreateDelegationRequest`,
  `hubapi.Project.Mode` (task 03); Hub task 35's `hublink.Resolver`, its `Project` and
  `Invalidate` (S-LINK, planned, **UNVERIFIED until merge**);
  `model.Class` (task 09); Fleet's session routes below (served by task 20 over task 12's mux).
- Required by: task 33 (the scripted run launches through it), task 52 (team read models reuse its
  client and token), task 55 (the Orchestrator panel attaches through it), task 64 (moving lanes).

#### The Fleet session routes `fleetbe` calls (complete)

Every route takes `Authorization: Bearer <Entra token, audience Fleet>` and derives the owner as
`(tid, oid)` from it (task 12's `authz.go`). Every error body is phase 2's `httpx.Problem`. Task 20
implements these handlers over `sessions.Launch`; this part fixes the wire so both sides agree.

| Method and path | Body | Success | Errors `fleetbe` maps |
|---|---|---|---|
| `POST /v1/projects/{p}/agents` | `CreateAgentRequest` | `201 AgentJSON` | `409 already_exists` → `ErrAlreadyExists`; `403 role_forbids` (viewer); `401 delegation_expired`; `429 quota_exceeded` → `ErrQuotaExceeded` |
| `GET /v1/agents?instance=<id>` | — | `200 {"agents":[AgentJSON]}`, the caller's own only | — |
| `GET /v1/agents?instance=<id>&ref=<ref>` | — | `200 {"agents":[…]}`, zero or one | — |
| `DELETE /v1/agents/{ag}` | — | `204` | `404 agent_not_found` → `ErrNotFound` |
| `GET /v1/agents/{ag}/attach` | WebSocket upgrade | `101`, phase 2's attach frames unchanged | `409 agent_not_running` → `ErrNotRunning`; `404` |

`{p}` is the Hub's project id; `{ag}` is an `ag-…` id; `ref` is the laptop's `Spec.ID`, unique per
owner and instance. **Fleet's attach handler is task 39 (TM1).** Until it lands, a real `fleetd`
answers `GET /v1/agents/{ag}/attach` with `501 not_implemented`, which `fleetbe` maps to
`ErrNotRunning` so the cockpit says *"This session has no running sandbox"* rather than crashing.
TM1-α's scripted run (task 33) drives agents through `panacea`, not through a terminal, so it does
not need attach.

```go
package fleetbe

// CreateAgentRequest is POST /v1/projects/{p}/agents. It carries no owner:
// the owner is the token's (tid, oid). It carries no attach token: Fleet
// authorises every attach by the caller's own Entra token (constraint 11).
type CreateAgentRequest struct {
	Ref        string   `json:"ref"`        // backend.Spec.ID; ^[a-z0-9]([a-z0-9-]{0,38}[a-z0-9])?$
	Instance   string   `json:"instance"`   // this control plane's instance id (phase 2 instance pkg)
	Class      string   `json:"class"`      // model.ClassWorker or model.ClassOrchestrator
	Harness    string   `json:"harness"`    // catalogue id, e.g. "claude-code"
	Command    string   `json:"command"`    // backend.Spec.Command, the harness binary
	Args       []string `json:"args"`       // backend.Spec.Args; may hold the private opening prompt (OD-24)
	Delegation string   `json:"delegation"` // dl-…, created by the user's own Hub call
	Task       string   `json:"task"`       // tk-… or ""; a worker the user launches by hand has none
	// OrchestratorDelegation is the owner's per-project orchestrator-class
	// delegation (b07). Optional: empty falls back to task 11's cache, and
	// Fleet re-reads and re-checks whichever it uses, so this is a hint that
	// saves a feed round trip, never an authority. Ignored on an orchestrator
	// launch (Class == model.ClassOrchestrator uses Delegation itself).
	OrchestratorDelegation string `json:"orchestratorDelegation,omitempty"`
}

// AgentJSON is Fleet's answer about one agent, as the laptop may see it. It
// holds no token, no pod IP, no node name and no opening prompt.
type AgentJSON struct {
	ID        string            `json:"id"`        // ag-…
	Ref       string            `json:"ref"`       // "" for a Fleet-spawned agent
	Instance  string            `json:"instance"`  // "" for a Fleet-spawned agent
	Project   string            `json:"project"`
	Class     string            `json:"class"`
	Status    string            `json:"status"`    // starting|working|waiting|idle|ended (§3.3)
	CreatedAt string            `json:"createdAt"` // RFC 3339, UTC
	Labels    map[string]string `json:"labels"`    // panacea-app/* only; Fleet strips the rest
}

// agentList is GET /v1/agents.
type agentList struct {
	Agents []AgentJSON `json:"agents"`
}
```

**The opening prompt is private (OD-24).** It travels in `Args` to Fleet, which passes it to the
harness and **never** stores it outside the pod spec, never echoes it in `AgentJSON`, and never
writes it to the Hub. The test `TestAgentJSONHasNoArgs` asserts the decoded `AgentJSON` type has no
field that could carry it, by reflection over its JSON tags, so the rule survives a later edit.

#### `fleetbe.go` (complete public surface)

```go
// Package fleetbe is backend.Backend over Fleet (phase T). The interface is
// phase 2's, unchanged (seam S8.1). Every call is the signed-in user's own:
// the bearer token comes from the Hub's token custody, per request.
package fleetbe

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubclient"
)

// Kind is declared here, not in backend.go, so phase 2's file is not edited.
const Kind backend.Kind = "fleet"

// Config is read once by the control plane's main from PANACEA_APP_FLEET_URL
// and PANACEA_APP_FLEET_SCOPE. An empty URL means "no team mode on this
// laptop": the API never routes a launch here.
type Config struct {
	BaseURL  string                // https://fleet.<env>/, no path
	Scope    string                // api://<fleet app id>/.default
	Instance string                // instance.ID() of this control plane
	Tokens   hubclient.TokenSource // entraauth.Authenticator (Hub task 31)
	HTTP     *http.Client          // nil means a client with a 15 s timeout
	Now      func() time.Time      // nil means time.Now
}

// Backend implements backend.Backend and backend.ReadyWaiter.
type Backend struct{ cfg Config }

func New(cfg Config) (*Backend, error) // error when BaseURL is not https or Instance/Scope/Tokens is empty

// Launch is what Create needs and backend.Spec cannot carry.
type Launch struct {
	Project    string // Hub project id
	Class      string // model.ClassWorker | model.ClassOrchestrator
	Harness    string // catalogue id
	Delegation string // dl-…
	Task       string // tk-… or ""
}

type launchKey struct{}

// WithLaunch returns ctx carrying l. Only teamlaunch.go calls it.
func WithLaunch(ctx context.Context, l Launch) context.Context {
	return context.WithValue(ctx, launchKey{}, l)
}

func launchFrom(ctx context.Context) (Launch, bool) {
	l, ok := ctx.Value(launchKey{}).(Launch)
	return l, ok && l.Project != "" && l.Delegation != "" && l.Class != ""
}

// FleetEndpoint is the opaque Endpoint every Sandbox from this backend
// carries: non-empty, so attach.go:182 lets it through; never dialled.
func FleetEndpoint(agID string) string { return "fleet:" + agID }

func (b *Backend) Kind() backend.Kind { return Kind }

// Create validates spec with backend.ValidateSpec first (both phase 2
// backends do), then refuses without a Launch on ctx, then POSTs
// CreateAgentRequest. Spec.AttachToken, Spec.Env, Spec.Mounts and
// Spec.Resources are NOT sent: attach is authorised per request, env and size
// are server policy (the class's policy file, task 13), and mounts are
// meaningless off the laptop. A non-empty Spec.Mounts is ErrUnsupported, as
// katabe does.
func (b *Backend) Create(ctx context.Context, spec backend.Spec) (backend.Sandbox, error)

// Get resolves ref=id with GET /v1/agents?instance=…&ref=…; zero agents is
// backend.ErrNotFound.
func (b *Backend) Get(ctx context.Context, id string) (backend.Sandbox, error)

// List returns only agents whose Instance equals cfg.Instance. It re-checks
// on the laptop even though it sent ?instance=, so a Fleet that ignored the
// query still cannot hand the reaper an Orchestrator to destroy.
func (b *Backend) List(ctx context.Context) ([]backend.Sandbox, error)

// Attach resolves id, then dials wss://…/v1/agents/{ag}/attach with the
// user's bearer token. token (phase 2's attach secret) is ignored: Fleet
// never saw it. See attach.go.
func (b *Backend) Attach(ctx context.Context, id, token string) (*websocket.Conn, error)

// Destroy resolves id, then DELETE /v1/agents/{ag}. Already gone is nil,
// as dockerbe and katabe treat it.
func (b *Backend) Destroy(ctx context.Context, id string) error

// WaitReady polls Get every 500 ms until State is running, or timeout.
// Fleet's create answers before the pod is ready (task 20), so this is a
// real wait, not a confirmation.
func (b *Backend) WaitReady(ctx context.Context, id string, timeout time.Duration) error

func (b *Backend) Close() error { return nil }

// Error is every problem Fleet answered. errors.Is(err, sentinel) holds for
// the mapped backend sentinel, so the API layer maps it exactly as it maps
// dockerbe's, and Code(err) gives Fleet's own code for the relay.
type Error struct {
	Status   int
	Problem  httpx.Problem
	sentinel error
}

func (e *Error) Error() string { return "fleet: " + e.Problem.Error + ": " + e.Problem.Message }
func (e *Error) Unwrap() error { return e.sentinel }

// ErrSignedOut is hubclient.ErrSignedOut, re-exported so the API maps a
// signed-out user to "Sign in to use team mode", never to backend_unavailable.
var ErrSignedOut = hubclient.ErrSignedOut

var errNoLaunch = errors.New("fleetbe: Create needs a team launch on the context (api.TeamLauncher)")
```

The `websocket` import is `github.com/coder/websocket`, as in `backend.go`.

**Status to `backend.State`**, one table, in `toSandbox`:

| Fleet `status` | `backend.State` |
|---|---|
| `starting` | `StateStarting` |
| `working`, `waiting`, `idle` | `StateRunning` |
| `ended` | `StateStopped` |
| anything else | `StateStopped`, and the unknown value is logged once per id (Fleet is newer than the laptop; a guess of "running" would let the cockpit attach to nothing) |

**Problem code to sentinel**, one table, in `mapProblem`:

| Status and code | Sentinel |
|---|---|
| `404 agent_not_found` | `backend.ErrNotFound` |
| `409 already_exists` | `backend.ErrAlreadyExists` |
| `409 agent_not_running`, `501 not_implemented` | `backend.ErrNotRunning` |
| `429 quota_exceeded` | `backend.ErrQuotaExceeded` (phase 8 task 22) |
| `400 bad_request`, `400 invalid_json`, `403 *`, `401 *` | none: the `*Error` alone, relayed with its code |
| `5xx`, transport error, timeout | `backend.ErrUnavailable` |

#### `teamlaunch.go` (complete public surface)

```go
package api

// TeamProjects answers whether a project is a team project, and makes one.
// Production is hubTeamProjects (teammode.go, fix batch b01): the local id
// resolved through Hub task 35's S-LINK, then hubapi.Project.Mode (task 03),
// and MakeTeam through task 08's hubclient.SetProjectMode (Hub route 87), as
// the signed-in user. Tests use a map. A launch never calls MakeTeam: only
// the team-mode route below does, so no launch turns a project team behind
// the Share consent (05-design.md §9.5).
type TeamProjects interface {
	IsTeam(ctx context.Context, projectID string) (bool, error)
	MakeTeam(ctx context.Context, projectID string) (hubapi.Project, error)
}

// Delegations is the slice of task 08's hubclient.Team this file calls, as
// the signed-in user (the desktop token; task 06 refuses a Fleet token here).
type Delegations interface {
	CreateDelegation(ctx context.Context, req hubapi.CreateDelegationRequest) (hubapi.Delegation, error)
	RenewDelegation(ctx context.Context, id string) (hubapi.Delegation, error)
	RevokeDelegation(ctx context.Context, id string) error
}

// TeamLauncher is set in Deps only when PANACEA_APP_FLEET_URL is set.
type TeamLauncher struct {
	Projects    TeamProjects
	Delegations Delegations
	Fleet       *fleetbe.Backend
	Now         func() time.Time
	// RenewBefore is how long before expiresAt a delegation is renewed.
	// Default 1 h, so a laptop asleep for under 23 h wakes with a live one.
	RenewBefore time.Duration
}

// routesFor is the class's route set of task 01 (task-01.md:365-374), which
// task 06's route 46 enforces. Neither set is copied: both come from task 03's
// ClassRoutes map (task-03.md:410-421), so a new route name cannot go missing here,
// and the worker set cannot drift from the Hub's. classOf is the one place a
// Fleet model.Class becomes a hubapi.DelegationClass.
func classOf(c string) hubapi.DelegationClass {
	if c == model.ClassOrchestrator {
		return hubapi.DelegationClassOrchestrator
	}
	return hubapi.DelegationClassWorker
}

func routesFor(c hubapi.DelegationClass) []hubapi.RouteName { return hubapi.ClassRoutes[c] }

// EnsureOrchestratorDelegation returns the owner's live orchestrator-class
// delegation for this project, creating one if there is none (fix batch b07,
// S02-C7/S03-C1: it closes task 20's Gap S20-1 and task 33's E2). It is
// per PROJECT, not per session: one Orchestrator serves every session the
// member runs there (task 20 decision 3's partial unique index), so a second
// launch reuses it. Idempotent on the Hub's Key (owner, project, class), so
// two replicas racing yield one delegation and two identical answers.
// It is NOT revoked when a session is destroyed (S03-I2 revokes the worker
// one); it lapses at 24 h if nothing renews it, and route 9's member removal
// revokes it with the worker one (task-44.md:327).
func (t *TeamLauncher) EnsureOrchestratorDelegation(ctx context.Context,
	projectID string) (hubapi.Delegation, error)

// Launch is called by the create handler for a team project, in place of
// a.backend.Create. In order: EnsureOrchestratorDelegation, so the owner has
// an Orchestrator delegation before Fleet is asked for anything; then create
// the session's own worker delegation as the user; then Create on fleetbe
// with the Launch on ctx, carrying BOTH ids (the worker's as Delegation, the
// orchestrator's as OrchestratorDelegation, so task 20 decision 4 does not
// depend on task 11's cache having caught up with a delegation created
// seconds ago). A Hub failure returns the Hub's
// *hubclient.Error unchanged, so the handler relays the Hub's code
// (hubclient.Code), never a Fleet one. A Fleet failure after the delegation
// exists revokes THE WORKER delegation (best effort, logged), so no live
// session delegation outlives a launch that did not happen. The orchestrator
// one is not revoked: it is per project, a second launch reuses it, and on a
// first launch it may already have been created by a parallel one.
func (t *TeamLauncher) Launch(ctx context.Context, projectID, class, harness string,
	spec backend.Spec) (backend.Sandbox, hubapi.Delegation, error)

// RunRenewer renews every delegation the registry holds for a live team
// session, AND every per-project orchestrator delegation with at least one
// live team session on that project (b07), when Now() >= ExpiresAt -
// RenewBefore. Both kinds arrive through the same live() set, tagged by
// Kind, so one timer covers them and the orchestrator one cannot be
// forgotten. Signed out
// (hubclient.ErrSignedOut): it stops renewing and logs once; the delegation
// then lapses at 24 h and Fleet pauses the sessions (§2.4 item 4). That is
// the design's intent, not a failure to fix here. A revoked delegation
// (task 06 route 48: 409 conflict) is dropped from the set and never retried.
func (t *TeamLauncher) RunRenewer(ctx context.Context, live func() []LiveDelegation)

// LiveDelegation is what the renewer needs per session, and (b07) per
// project for the orchestrator one. Kind is "worker" or "orchestrator";
// for an orchestrator row SessionID is empty and ProjectID is set, and the
// row is present only while that project has at least one live team session.
type LiveDelegation struct {
	Kind       string
	SessionID  string
	ProjectID  string
	Delegation string
	ExpiresAt  time.Time
}

// teamScopeOf labels a session for sessionJSON.teamScope.
//   solo project                    -> ""
//   team project, Backend == "fleet" -> "team"
//   team project, any other backend -> "outside"
func teamScopeOf(isTeam bool, backendKind string) string
```

**Where the delegation id is kept.** The registry row (`registry.Session`) gains one field,
`Delegation string`, persisted, and **not** in `sessionJSON`. It is not a secret (the Hub audits
it, and it is useless without Fleet's workload identity), but it is a Hub record id the cockpit has
no use for, and phase 2's rule is that the wire carries only what the cockpit renders. The
expiry is kept beside it as `DelegationExpiresAt`. Both are restart-safe, so a control plane
restarted at hour 23 renews on its first tick.

**Where the ORCHESTRATOR delegation is kept (b07).** Not on a session: it outlives every session
in its project. `registry` gains a second table, `TeamDelegations`, keyed `(projectID)` with
`{Delegation, ExpiresAt, CreatedAt}`, persisted beside the sessions file and written through the
same store. `EnsureOrchestratorDelegation` reads it, and on a miss **or** an expiry in the past
calls `CreateDelegation` with `Class: hubapi.DelegationClassOrchestrator` and
`routesFor(hubapi.DelegationClassOrchestrator)`, then writes the row. A stale row whose delegation
the Hub has revoked (route 47 answers 404/409) is deleted and re-created once, so a member removed
and re-added is not stuck on a dead id. This is restart-safe for the same reason the session one
is, and it is why a second launch makes no Hub call.

**How Fleet learns it.** `CreateAgentRequest` gains `OrchestratorDelegation string` (defined above
with this task's other wire shapes; task 20 decodes it into `LaunchRequest.OrchestratorDelegation`,
`task-20.md:166-171`). Task 20 decision 4 (`task-20.md:45-55`) uses that id when
it is set, and falls back to task 11's cache lookup when it is not (a non-team or replayed
request), so an Orchestrator is created on the member's **first** launch rather than on the first
launch after the change feed catches up. Fleet still resolves the delegation (task 11's cache under the member's own key, else `Adopt`'s Hub read) and checks it against the member before
using it (S02-C6's owner, org, project and class check, landed in fix batch b06; `Adopt`'s since b06 round 2, which the Hub also audits), so the field is a
hint, never an authority: a `CreateAgentRequest` naming another member's `dl-…` is refused with
`401 delegation_expired` and nothing is created.

**The mode, read and written (fix batch b01, X-C1; was a gap).** The field is task 03's
`hubapi.Project.Mode` (Block 7; `hubapi/types.go:140-147` has none today, checked), and its one
writer is task 06's Hub route 87. Hub task 42 is Share's **UI** and adds no field. This task owns
the control plane's half, so that one local route is how the desktop, and task 33's E5, make a
project team:

| Method | Path | Body | Success | Errors (besides the Hub's, relayed) |
|---|---|---|---|---|
| POST | `/v1/projects/{projectId}/team-mode` | `{}` (strict) | `200 {"project": Project}`, `mode: "team"` | 404 `project_not_found` (the local id); 409 `conflict`, `details.reason = "not_shared"` when S-LINK says `unlinked` or `no_remote`, and the Hub is not called; 503 `hub_not_configured` when `Deps.Hub` is nil (Hub task 35's rule) |

The Hub's own answers are relayed with their code (`a.relayHub`, as the launch hunk): 403
`forbidden`, `required: "admin"`; 503 `unavailable`, `details.reason = "audit_not_streaming"`.
The route needs the Hub, not Fleet, so it is served whenever `Deps.Hub` is set, and
`hubTeamProjects` is built from `Deps.Hub` and task 35's `Resolver` alone.

**The S-LINK cache (b01 round 2, re-review I-2).** `Resolver.Project` caches a local project's
`Link` for 5 minutes (hub `task-35.md:73-77`), and resolving the local id for this route, or
Share's `shareProject` just before it, warms that cache with `mode: "solo"`. So
`hubTeamProjects.MakeTeam`, on the Hub's 200 and before it returns, calls
`Resolver.Invalidate(localProjectID)` (`task-35.md:87-89`); the next `IsTeam` asks route 13
again and reads `team` (task 06's `store/projects.go` change makes route 13 carry it). On an
error it invalidates nothing. A teammate's change on another laptop still arrives the Hub's
way: `project.mode_set` is a `project.*` kind in the member feed (task 01 §8, `yes`), and Hub
task 36's subscriber calls `Invalidate("")` on it.

**Ruling (declared; not the owner's words): the product caller is Share, not the first launch.**
The review offered both. A launch that made a project team would do it without the consent
line that §9.5 puts at Share, so `Launch` never calls `MakeTeam`, and a test pins it. Task 56's
Share path calls this route after the link (task 56, b01), and task 33's E5 calls it directly.

- [ ] **Step 1: Write the failing tests first**

`fakefleet_test.go` is an `httptest.Server` that serves the five routes above from a map keyed by
`(owner, ag-id)`, where the owner is read from a fake bearer token `tok:<oid>`; it records every
request's headers and body. It also holds one Fleet-spawned agent (`Ref: ""`, `Instance: ""`) for
every owner, the Orchestrator task 20 would create.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestCreateSendsNoAttachTokenNoOwner` | the recorded create body has no `attachToken`, `owner`, `tid`, `oid`, `env` or `resources` key (decoded to `map[string]any`, keys listed) | `Create` marshalling the whole `Spec`, or adding the owner from config |
| `TestCreateWithoutLaunchIsUnsupported` | `Create(context.Background(), validSpec)` is `errors.Is(err, backend.ErrUnsupported)` and made **no** HTTP request | a `Create` that posts with an empty delegation and lets Fleet refuse it |
| `TestCreateValidatesSpecFirst` | a spec with `ID: "Bad_ID"` fails before any request | skipping `backend.ValidateSpec` |
| `TestListHidesFleetSpawnedAgents` | with the fake returning the Orchestrator **even when** `?instance=` is set (a fake that ignores the query), `List` omits it | trusting the query, which hands the reaper the Orchestrator |
| `TestListHidesOtherInstance` | an agent created by instance `i-b` is absent from `i-a`'s `List` | filtering on owner only; two laptops' reapers destroy each other's sessions |
| `TestEndpointIsOpaqueAndNonEmpty` | `Get(...).Endpoint == "fleet:ag-…"` | an empty `Endpoint`, which `attach.go:182` reads as not running |
| `TestStateMapping` | each row of the status table, plus `"weird"` → `StateStopped` | mapping unknown to `StateRunning` |
| `TestProblemMapping` | each row of the code table, with `errors.Is` on the sentinel and `errors.As` to `*fleetbe.Error` keeping `Problem.Error` | string-matching messages; losing Fleet's code |
| `TestAttachUsesBearerNotAttachToken` | the upgrade request carries `Authorization: Bearer tok:alice` and no `panacea-app` subprotocol token; `token` argument `"s3cret"` appears in no header or URL | reusing phase 2's attach-token header |
| `TestAttach501IsNotRunning` | a fake answering `501 not_implemented` gives `errors.Is(err, backend.ErrNotRunning)` | treating it as `ErrUnavailable`, which the cockpit shows as "Fleet is down" |
| `TestDestroyGoneIsNil` | `Destroy` on an id `Get` cannot find returns nil | a reaper loop stuck on a 404 |
| `TestSignedOutIsNotUnavailable` | a `TokenSource` returning `hubclient.ErrSignedOut` gives `errors.Is(err, fleetbe.ErrSignedOut)` and **not** `backend.ErrUnavailable` | "Fleet is down" shown to a signed-out user |
| `TestAgentJSONHasNoArgs` | reflection over `AgentJSON`'s JSON tags: none of `args`, `openingPrompt`, `command`, `token` | a later field that echoes the private prompt (OD-24) |

`teamlaunch_test.go` uses a fake `Delegations` and the fake Fleet above:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestLaunchCreatesDelegationThenAgent` | call order: `CreateDelegation` then Fleet `POST …/agents`; the agent body's `delegation` is the returned `dl-…` | calling Fleet first, or with a stale id |
| `TestLaunchHubDownRelaysHubCode` | `CreateDelegation` returning `&hubclient.Error{Status: 503, Problem: {Error: "upstream_unavailable"}}` makes the handler answer **503 with the Hub's code**, and Fleet got no request | wrapping every failure as `backend_unavailable` |
| `TestLaunchFleetFailRevokes` | Fleet `409 already_exists` after a created delegation calls `RevokeDelegation(dl)` once | a live delegation for a launch that never happened |
| `TestWorkerAsksWorkerRoutes` | a worker launch's `CreateDelegationRequest.Routes` equals `hubapi.ClassRoutes[hubapi.DelegationClassWorker]` exactly, and contains no `messages.*`, `directives.*` or `escalations.*` name | a worker delegation that can reach the cross-user channel (OD-16) |
| `TestFirstLaunchCreatesOrchestratorDelegation` (b07) | on a team project with an empty `TeamDelegations`: **two** `CreateDelegation` calls in order, `orchestrator` then `worker`; the orchestrator's `Routes` equals `hubapi.ClassRoutes[hubapi.DelegationClassOrchestrator]`; the `CreateAgentRequest` carries both ids | the worker-only create of S02-C7/S03-C1, which leaves TM1-α with no Orchestrator |
| `TestSecondLaunchReusesIt` (b07) | a second launch on the same project with a live row: **one** `CreateDelegation` call (the worker's), and the same `OrchestratorDelegation` id on the request | a fresh orchestrator delegation per launch, which task 20 decision 3's unique index then fights |
| `TestExpiredOrchestratorRowIsReplaced` (b07) | a `TeamDelegations` row with `ExpiresAt` in the past → one orchestrator create, the row overwritten; a row the Hub answers 404 for → deleted and re-created **once**, not in a loop | a stale id used for ever, so every launch answers `awaiting-delegation` |
| `TestRenewerRenewsTheOrchestratorOne` (b07) | `live()` returning one `Kind: "orchestrator"` row inside the window renews it once; with no live session on that project the row is absent and nothing is renewed | a renewer that walks sessions only, so the Orchestrator lapses at 24 h mid-run |
| `TestLaunchFleetFailKeepsOrchestrator` (b07) | Fleet `409 already_exists` revokes the **worker** delegation and not the orchestrator one | a rollback that revokes a per-project delegation another live session is using |
| `TestRenewerRenewsInWindow` | fake clock at `expiresAt − 59 min` renews once; at `− 61 min` does not | an off-by-sign window |
| `TestRenewerStopsWhenSignedOut` | after one `ErrSignedOut`, no further calls for 3 ticks | a renew storm against a refused token |
| `TestSessionJSONTeamScope` | the three rows of `teamScopeOf`, and a marshalled team session has `"teamScope":"team"` and no `delegation` key | leaking the delegation id, or labelling a local session `team` |
| `TestSessionJSONLeaksNoToken` | phase 2's existing byte search (`sessions_test.go`) extended: a Fleet session's JSON contains neither the attach token nor the fake bearer | a new field that carries either |
| `TestLaunchNeverMakesTeam` (b01) | a launch on a solo project goes to `a.backend.Create`, and the `TeamProjects` fake records no `MakeTeam` call | the first launch turns the project team, skipping Share's consent line |

`teammode_test.go` (b01) runs the route over `hubclient.NewFake()` (task 08's `SetProjectMode` with
the route's rules) and the **real** `hublink.Resolver` (`hublink.New(fake, remotes, log)`, Hub
task 35) over a scripted `Remotes` that gives the local project the fake project's repo key, so the
5-minute cache is the production one (b01 round 2, re-review I-2). `TestTeamModeNeedsALink` alone
scripts `Remotes` to return no key, which is how S-LINK answers `unlinked`:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestTeamModeRouteMakesTeam` | linked project; `IsTeam` → false first, which warms the `Resolver`'s cache with `solo`; the fake's admin → 200, `project.mode == "team"`; then, at once and with no clock advance, `IsTeam` → true, and the next launch goes through `TeamLauncher` | `MakeTeam` a no-op that answers 200; `IsTeam` reading anything but `Mode`; `MakeTeam` not calling `Resolver.Invalidate`, so a launch right after Share goes solo for 5 minutes (re-review I-2) |
| `TestTeamModeFailureKeepsTheCache` (b01 round 2) | the gate shut, route → 503; the count of `"ProjectForRepo"` in `fake.Calls()` (`fake.go:130`) does not rise on the next `IsTeam` | `Invalidate` called before the Hub answers, so a refused write still costs a Hub round trip per launch |
| `TestTeamModeRelaysTheGate` | after `fake.ShutTeamModeGate()` → 503 with the Hub's `details.reason = "audit_not_streaming"`; `IsTeam` still false | the Hub's 503 rewritten as `backend_unavailable`, so the cockpit says "Fleet is down" |
| `TestTeamModeRelaysAdminOnly` | the fake's member view → 403 `required: "admin"` | the control plane pre-checking a role it cannot know |
| `TestTeamModeNeedsALink` | S-LINK `unlinked` → 409 `details.reason = "not_shared"`; the fake saw no call | a Hub call with the local id, which is a path hash no Hub record names |
| `TestTeamModeBodyIsStrict` | `{"mode":"solo"}` → 400 `bad_request`, `details.field = "mode"` | the local route passing a mode through, so it becomes a two-way switch |

Run them and watch them fail to compile:

```bash
cd control-plane && go test ./internal/backend/fleetbe/... ./internal/api/... 2>&1 | head -5
```

Expected: `undefined: fleetbe.New` (and similar). That is the red.

- [ ] **Step 2: `fleetbe.go` until the backend tests pass**

Write the surface above. `Create` in this order: `backend.ValidateSpec(spec)`; `launchFrom(ctx)` or
`fmt.Errorf("%w: %v", backend.ErrUnsupported, errNoLaunch)`; refuse non-empty `spec.Mounts` with
`ErrUnsupported`; `Tokens.Token(ctx, cfg.Scope)`; `POST`. Every request goes through one
`do(ctx, method, path, body, out)` helper that sets the bearer, bounds the response body at
`httpx.MaxBodyBytes`, and decodes a non-2xx body as `httpx.Problem` into `mapProblem`. The helper
**never** puts the token in an error string or a log line; a test greps the `slog` buffer for
`tok:`.

- [ ] **Step 3: `attach.go`**

`websocket.Dial(ctx, wsURL(cfg.BaseURL, "/v1/agents/"+ag+"/attach"), &websocket.DialOptions{
HTTPHeader: http.Header{"Authorization": {"Bearer " + tok}}, HTTPClient: cfg.HTTP})`. On a failed
upgrade, read the response's status and body through `mapProblem`. The frames after `101` are
phase 2's attach frames unchanged, so the proxy (`internal/proxy`) relays them without knowing the
backend.

- [ ] **Step 4: `teamlaunch.go` and the two anchored hunks**

In `sessions.go`'s create handler, before its `a.backend.Create(...)` call, anchored on that call:

```go
if a.team != nil {
	isTeam, err := a.team.Projects.IsTeam(r.Context(), req.ProjectID)
	if err != nil { a.relayHub(w, r, err); return }
	if isTeam {
		sb, dl, err := a.team.Launch(r.Context(), req.ProjectID, model.ClassWorker, req.Harness, spec)
		// errors: a *hubclient.Error is relayed with its own code; a *fleetbe.Error by its sentinel
		...
	}
}
```

and `sessionJSON` gains a `TeamScope string` field tagged `json:"teamScope"`, filled by `teamScopeOf`. The launch class from the
cockpit's New Session is always `worker`: the **sandbox** the cockpit gets is a worker. But the
desktop does ask the Hub for the orchestrator **delegation** (b07, S02-C7/S03-C1), because route
46 is desktop-only (`task-06.md:34`, `:165`, `:616`) and Fleet calling it is 403; `Launch` calls
`EnsureOrchestratorDelegation` first, and task 20 then creates the Orchestrator pod under it.
Before b07 this line said the desktop never asks for one, which left task 20's decision 4 answering
`"awaiting-delegation"` for ever and task 33's E2 unmeetable. `a.relayHub` is Hub task 33's relay rule
(`hubclient.Code`, `Status`), **UNVERIFIED until merge**; until then the hunk writes
`httpx.WriteProblem(w, he.Status, he.Problem.Error, he.Problem.Message, he.Problem.Details)`.

- [ ] **Step 5: `catalog.go`**

`isolationFor`: `case fleetbe.Kind: return "microvm"`. `limitationsFor` gains the one line
*"Runs in Panacea's cloud; your teammates' agents see this agent's task and files."* No other
catalogue change.

- [ ] **Step 6: `teamrelay.go` — the four human routes (b07, G33-1/S04-I8)**

Hub routes 65, 66, 68 and 70 accept **only the owner's own desktop** (`task-07.md:144-147`,
`:182-184`), and the desktop's own team client is task 52's, in TM1. So until b07 nothing before
task 33 let a human answer a decide or speak to their Orchestrator, task 33's E3 could only be
BLOCKED, and the review consolidated that into X-C2. This task owns them, because it already holds
the Hub team client and `a.relayHub` on the local control plane. Task 52 extends these four; it does
not replace them.

Four routes on the local control plane, each a **relay**: it adds the caller's own stored token, it
adds nothing else, and it passes the Hub's status, code and `details` back through `a.relayHub`.
No route takes an owner, an org or an agent's owner from the body — the Hub resolves the caller from
the token, which is why one person cannot read or answer another's (task 07's own check, not ours).

```jsonc
// GET  /api/team/escalations?project=pj-…            → Hub route 66, the caller's own
{ "escalations": [ { "id": "es-…", "kind": "inform" | "decide", "question": Text,
    "options": [ { "label": Text, "consequence": Text } ], "default": 0, "deadline": "…",
    "evidence": { … } | null, "answer": null | { "choice": 1, "answeredAt": "…" } } ] }
// POST /api/team/escalations/{es}/answer  { "choice": 1 }         → Hub route 65
// POST /api/team/human-messages  { "projectId": "pj-…", "toAgentId": "ag-…", "text": "…" } → route 70
// GET  /api/team/directives?project=pj-…               → Hub route 68, the caller's own
{ "directives": [ { "id": "dv-…", "toAgentId": "ag-…", "kind": "pause" | "resume" | "narrow" |
    "wait-for" | "rebase" | "free", "params": { … } | null, "origin": "human" | "fleet",
    "issuedAt": "…", "state": "issued" | "in-force" | "spent", "approvedBy": "es-…" | null } ] }
```

`GET /api/team/directives`'s shape is written out here because it was the one of the four task 33
left implicit (S04-I8). `text` on route 70 is the human's own words and is **not** parsed, trimmed
of meaning or interpreted here: it is a body this process forwards. All four are refused `404` when
`a.team == nil` (no `PANACEA_APP_FLEET_URL`), so a solo build does not grow a team surface.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestRelaysCarryNoIdentityFromTheBody` | an `ownerUserId`, `orgId` or `asUser` key in any of the four bodies is a `400 bad_request`, and the forwarded body never gains one | a relay that lets the caller name whose escalations to list, which is the cross-owner channel OD-16 gives the Orchestrator alone |
| `TestRelayPassesTheHubsRefusal` | the Hub's `403 forbidden` on another person's `es-…` reaches the client with the Hub's own code and `details` | a relay that rewrites a refusal as `backend_unavailable`, hiding the one check that keeps the two humans apart |
| `TestDirectivesShapeIsComplete` | a golden Hub body round-trips every field above, `params` and `approvedBy` included | a shape that drops `origin` or `approvedBy`, so task 33's M4 cannot tell a Fleet turn from a human one |
| `TestTeamRoutesAreAbsentWithoutFleet` | all four → 404 with `a.team == nil` | a team surface on a solo install |

- [ ] **Step 7: Acceptance**

```bash
cd control-plane && go vet ./... && go test ./internal/backend/fleetbe/... ./internal/api/... -count=1
```

Expected: `ok` for both packages, and every test named above listed under `-v`. Then the
leak guard, which must print nothing:

```bash
cd control-plane && grep -rn "AttachToken" internal/backend/fleetbe/*.go | grep -v _test | grep -v ValidateSpec
```

- [ ] **Step 8: Commit**

```bash
git add control-plane/internal/backend/fleetbe control-plane/internal/api/teamlaunch.go \
  control-plane/internal/api/teamlaunch_test.go
git commit -F <msgfile> -- control-plane/internal/backend/fleetbe control-plane/internal/api/teamlaunch.go \
  control-plane/internal/api/teamlaunch_test.go control-plane/internal/api/sessions.go \
  control-plane/internal/api/catalog.go control-plane/internal/registry/session.go
```

**What this task does NOT claim.**
- That any of it runs against a real Fleet. It runs against `fakefleet_test.go`. Task 33 is the
  first launch through a real `fleetd`, and task 57 the first with two real people.
- That attach works end to end. Fleet's attach handler is task 39; until then `Attach` is proven
  against the fake only, and a real Fleet answers `501`.
- That `backend.ReadyWaiter` and phase 8 task 23's `watchSessionReady` change exist here. Both are
  **UNVERIFIED until the `feat/env` sync-merge**; without them the readiness watch polls
  `http://fleet:ag-…/healthz`, fails, and marks the session exited. This task therefore **starts
  after** that merge, and its dependsOn should say so (listed as a skeleton gap).
- That the Hub's project record says which projects are team projects (the gap above).
- That the renewer covers a laptop that is off for more than 23 hours. It does not, by design:
  the delegation lapses and Fleet pauses the sessions (§2.4 item 4).
