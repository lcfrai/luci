# Task 20a — `designapi` routes, the SSE window stream, problem codes, the route audit

**Plan:** `../2026-09-23-phase-5-design-canvas.md` · **Depends on:** 11, 12, 13, 14, 19 (14 for `canvas/mcp/ports.go`, the five port types this task names; 19 for `control-plane/go.mod`'s `canvas` requirement and the earlier `api.go` edit; b02 round 2, RR-I6-1) · **Then:** task 20b (`task-20b.md`) wires this into `cmd/api` · **External:** **Hub task C3 — Hub plan tasks 37–38 (`localkey`), a hard dependency: this task does not start until both have landed** (plan rule 4); phase 4 task 10 (`api.RouteMounter`), phase 4 task 07 (`discoverhub`) · **Hours:** 4.5 (the review's sizing: twenty routes and the SSE broker were about twice the old 3 h for the whole of task 20; plus 0.5 for the phase-3 branch adapter, review I7)

**Why two parts (review I6).** The old task 20 declared 11, 12, 13 but its wiring called
`mcp.NewServer` (14), `mcp.Instructions` (17), `mcp.RegisterAll` (15) and `mcprelay.NewSupervisor`
(19), so a lane agent starting it after 13 had a `main.go` that could not compile. **20a** (this
file, kept as `task-20.md` so earlier reviews' line citations still resolve) is everything that
compiles against 11–13 plus task 14's `canvas/mcp/ports.go`: the `designapi` package, `events`,
the `httpx` codes, `api.go`'s mount and the route audit. `designapi` names five types from that
file (`mcp.Briefs`, `mcp.Criterion`, `mcp.Section`, `mcp.Approvals`, `mcp.ApprovalInfo`), so
`ports.go` lives in task 14 (stdlib only, no server code), not task 15: 20a starts after 13 and
stays off the critical path (re-review RR-I6-1, the reviewer's "split `ports.go` out of 15"
option, folded into 14 instead of a new task). 20a also depends on **19**: 19 is the first
importer of the `canvas` module and so owns `control-plane/go.mod`'s `require` and `replace`
(plan file map), and 19 edits `internal/api/api.go` before 20a does (plan "Two lanes never edit
one file"). Without 19 declared, a `GOWORK=off` build of `designapi` fails and the two lanes race
on `api.go`. 19 finishes at 8 h, so the critical path does not move. **20b** (`task-20b.md`) is `cmd/api`: the two variables, the one builder
`newDesignAPI` and the `main.go` wiring, after 17, 19 and 20a.

## Why

The desktop reaches the authority only through the control plane's loopback surface, as it reaches
everything else (Hub contract §5.4's rule, applied here). This task is that surface for everything
except approval and handoff (tasks 21, 22): the human's ops, undo, revert, comments, selection,
assets, the code panel, render results — and the **window stream** `GET /v1/events` (plan P5-B,
UI contract §10.5). Task 20b wires the whole canvas stack into `cmd/api/main.go`; nothing in 20a
imports `canvas/mcp`'s server, `mcp.RegisterAll`, `mcp.Instructions` or `mcprelay`. It imports
`canvas/mcp` only for `ports.go`'s five types, which task 14 writes.

## Files

- `control-plane/internal/designapi/` — `routes.go`, `canvas.go`, `ops.go`, `assets.go`, `comments.go`,
  `render.go`, `export.go`, `briefs.go`, `problems.go`, `mount.go`, `approvals_none.go` + tests (new)
- `control-plane/internal/events/broker.go`, `sse.go` + tests (new)
- `control-plane/internal/httpx/problem.go` — the codes below
- `control-plane/internal/api/api.go` — `Deps.Design RouteMounter`; in `New`, after the Discover mount: `if d.Design != nil { d.Design.Mount(guarded.Handle) }`
- `control-plane/internal/api/routes_audit_test.go` — the GET routes in `catalogReadOnlyRoutes`;
  `TestPreflightAllowsEveryRegisteredMethod` (review I3)

- `control-plane/internal/designapi/branches_phase3.go` + test — **only if phase 3's `lanes`
  package is in the tree** (as `briefs_phase4.go` waits for phase 4): the `authority.BranchLifecycle`
  adapter, seam S5-P3.2 (review I7, below)

`cmd/api/config.go`, `main.go`, `design.go` and `design_test.go` are task 20b's.

**The branch-lifecycle adapter (seam S5-P3.2, review I7).** `Phase3Branches(lanes lanes.Store,
git <phase 3's gitx.Machine>, projectPath func(projectID string) (string, error))
authority.BranchLifecycle`:
- `Renames(ctx, p)` returns phase 3's `lanes.Store.Renames(p)` (oldest first,
  `.claude/worktrees/phase-3/docs/superpowers/plans/2026-09-23-phase-3-git-convergence-parts/task-03.md:72-96`)
  mapped field for field; it never errors, because phase 3's method does not.
- `Deletes(ctx, p)` runs **one** `for-each-ref --format=%(refname) refs/panacea/archive/` in the
  project's repository through phase 3's `gitx.Machine` (Scope Project, no hook, phase 3's
  hardened invocation; a fixed argument list, so nothing caller-supplied reaches git) and parses
  each `refs/panacea/archive/<branchID>/<unix>` into `branchID → [times]`, ascending (every
  delete, not the newest: re-review RR-I7-2). A ref whose `<branchID>` fails `^b-[a-z0-9-]+$`, or
  whose `<unix>` is not decimal, or that has another number of segments, is skipped and logged,
  never guessed at.
- A route whose `resolve` fails with `authority.ErrBranchLifecycle` answers `500 internal` (detail
  in the log); an MCP call answers `internal`. No new problem code.
If phase 3 is not in the tree, `authority.Config.Branches` stays nil (task 20b) and the commit says so.

**The `Approvals` seam (review I6's stub, inside `designapi`).** `canvas/mcp.Deps` takes an
`Approvals` port (task 15, `task-15.md:61-65`) that task 21 implements. 20a adds
`func (a *API) Approvals() mcp.Approvals`, which returns `noApprovals{}` from `approvals_none.go`:
`Latest` → `(mcp.ApprovalInfo{}, false, nil)`, which is true, because no route can approve until
task 21 exists. Task 20b wires the method's result, `Approvals: designAPI.Approvals()`, into
`mcp.RegisterAll`. Task 21 deletes `approvals_none.go` and makes `Approvals()` return its in-memory
port, so the MCP side needs no wiring change. (Task 21 still edits one `cmd/api/design.go` line, the
`Hub` field below, so it runs after 20b.) `TestApprovals_noneUntilTask21` pins the stub's answer.

If phase 4's task 10 has not landed, this task declares `RouteMounter` in `api` **exactly as phase 4
writes it** (`interface { Mount(handle func(pattern string, h http.Handler)) }`) and adds the
`Discover` field too, so whichever lands second merges without a change.

## Contracts

### Routes (all under the origin guard **and** the local-key guard; `/api/v1/…` works through the alias, `api.go:287-292`)

Every route below requires `Authorization: Bearer <local key>` (Hub contract §9.4, Hub plan task 37's
`localkey.Middleware`); a missing or wrong key is `401 local_key_required`. None of them is in
`localkey.Exempt` (that exempts only `…/sessions/{id}/attach`). The Host check and the Origin
allowlist stop a browser; a process in a sandbox sends no `Origin` and writes any `Host` it likes
(`httpx/origin.go:117-123`), so the key is the only guard that stops an agent calling the human
surface — and therefore the only thing behind "only a human approves" (review C1).

| # | Method | Path | Request | `2xx` response |
|---|---|---|---|---|
| 1 | GET | `/v1/design/projects/{projectId}/branches/{branchId}/canvas` | — | `200 {"canvas": Summary \| null}` |
| 2 | POST | `/v1/design/projects/{projectId}/branches/{branchId}/canvas` | `{}` | `201 Summary` (created) or `200 Summary` |
| 3 | GET | `/v1/design/canvases/{canvasId}` | — | `200 Summary` |
| 4 | GET | `/v1/design/canvases/{canvasId}/document?at=head\|<n>` | — | `200 doc.Document` |
| 5 | POST | `/v1/design/canvases/{canvasId}/ops` | `{"ops":[doc.Op],"label":"…","ifRevision":0}` | `200 authority.ApplyResult` |
| 6 | POST | `/v1/design/canvases/{canvasId}/undo` | `{}` | `200 authority.UndoResult` |
| 7 | POST | `/v1/design/canvases/{canvasId}/redo` | `{}` | `200 authority.UndoResult` |
| 8 | POST | `/v1/design/canvases/{canvasId}/revert` | `{"sessionId":"s-…","sinceRevision":0}` | `200 authority.RevertResult` |
| 9 | DELETE | `/v1/design/canvases/{canvasId}/claims/{artboardId}` | — | `204` |
| 10 | GET | `/v1/design/canvases/{canvasId}/activity?after=&limit=` | — | `200 {"rows":[authority.ActivityRow]}` |
| 11 | POST | `/v1/design/canvases/{canvasId}/assets/{sha}` | raw bytes; `Content-Type` one of the four image types; ≤ 10 MiB | `200 {"asset": doc.Asset, "revision": n}` — stored and `add_asset` applied as one human batch |
| 12 | GET | `/v1/design/canvases/{canvasId}/assets/{sha}` | — | the bytes; `Content-Type` as stored; `Cache-Control: private, max-age=31536000, immutable` (content-addressed); `X-Content-Type-Options: nosniff` |
| 13 | POST | `/v1/design/canvases/{canvasId}/threads` | `{"nodeId":"n-…","text":"…"}` | `201 doc.Thread` |
| 14 | POST | `/v1/design/canvases/{canvasId}/threads/{threadId}/messages` | `{"text":"…"}` | `201 doc.Message` |
| 15 | POST | `/v1/design/canvases/{canvasId}/threads/{threadId}/status` | `{"status":"open"\|"resolved"}` | `200 doc.Thread` |
| 16 | POST | `/v1/design/canvases/{canvasId}/reviews/{reviewId}/close` | `{}` | `200 doc.ReviewRequest` |
| 17 | POST | `/v1/design/canvases/{canvasId}/selection` | `{"nodeIds":["n-…"]}` | `200 authority.Selection` |
| 18 | GET | `/v1/design/canvases/{canvasId}/export?nodeId=&format=html\|jsx-inline\|jsx-tailwind&at=` | — | `200 {"format":"…","text":"…"}` (the code panel) |
| 19 | POST | `/v1/design/render-results/{requestId}` | `authority.RenderResult`, ≤ 32 MiB (its own decoder, not `httpx.DecodeJSON`'s 1 MiB) | `204` |
| 20 | GET | `/v1/events?window={id}&topic=…` | — | `200 text/event-stream` (below) |

Every human route builds `authority.Caller{Author: {Kind: human, UserID: "local", Handle: <the
signed-in name from the Hub's /v1/auth/status, else "You">}}` — **no body field sets it** (plan
rule 4). Routes 21–27 (approval, handoff, links) are tasks 21 and 22.

**Verbs: GET, POST and DELETE only (review I3).** Every Tauri build is cross-origin against the
control plane, so every non-GET call is preflighted, and the preflight answers
`Access-Control-Allow-Methods: GET, POST, PATCH, DELETE, OPTIONS` (`httpx/origin.go:169`, the
verb set of the phase-2 Go contract §3.1, `2026-09-18-phase-2-go-contract.md:1531`) — **no `PUT`**. Routes 11 and 17 were `PUT`, which the webview
would block at the preflight while every `httptest` and vitest test passed (neither enforces CORS).
They are `POST`: the canvas stays inside the verb set phase 2 declared, and `origin.go` (which Hub
task 37 already edits for `corsHeaders`) is not touched a second time. Route 11 is still idempotent
by construction (the path is the content's SHA-256), so `POST` loses nothing. A new design route
that needs another verb amends `corsMethods` and Go contract §3.1 in its own commit;
`TestMount_everyMethodPassesPreflight` and `TestPreflightAllowsEveryRegisteredMethod` (Tests) make
that a red test instead of a first-run surprise.

### The window stream, exactly

```
GET /v1/events?window=wn-3f2a91c07b1e&topic=canvas:cv-7c01d2e3f4a5b6c7@41&topic=renderer:cv-7c01d2e3f4a5b6c7:visible
Authorization: Bearer <local key>        ← Hub task C3 (required; no key → 401 local_key_required); fetch() can send it

200  Content-Type: text/event-stream; charset=utf-8
     Cache-Control: no-store
     X-Accel-Buffering: no

event: canvas                              one per authority.Event (task 11), in revision order
id: canvas:cv-7c01d2e3f4a5b6c7@42
data: {"canvasId":"cv-…","kind":"change","revision":42,…}

event: render                              one per authority.RenderRequest for a renderer topic
data: {"id":"rq-…","canvasId":"cv-…","revision":42,"kind":"screenshot",…}

event: gone                                a topic that cannot be served; the stream stays open
data: {"topic":"canvas:cv-…@41","code":"canvas_not_found"}

: ping                                     every 15 s
```

- `window` is `wn-` + 12 hex, minted by the desktop once per window; the renderer id is
  `<window>/<canvasId>`. At most **16 topics** per stream; more is `400 bad_request`.
- Topic grammar: `canvas:<canvasId>@<afterRevision>` or `renderer:<canvasId>:visible|hidden`.
- The desktop **replaces** its stream when its topic set changes (a new request, then it closes the
  old). One stream per window (P5-B). On reconnect it resends each canvas's last revision; the
  authority replays or resets (task 11).
- Closing the stream detaches its renderers (task 13). A renderer that lets **two** requests time
  out is detached and gets `gone` for its topic — a phantom renderer cannot starve agents.
- **Why a GET is right here** (§10a, `httpx/origin.go`'s *"no side-effecting GET"*): the stream
  changes no stored state; a renderer registration is a subscription that ends with the connection,
  like the attach WebSocket's. It is listed in `catalogReadOnlyRoutes` with that sentence.

```go
// control-plane/internal/events/broker.go
package events

type Topic struct {
	Kind     string // "canvas" | "renderer"
	CanvasID string
	After    int    // canvas topics
	Visible  bool   // renderer topics
}

func ParseTopic(s string) (Topic, error)

// Source is what the broker needs from the authority (task 11, 13).
type Source interface {
	Subscribe(canvasID string, after int) (<-chan authority.Event, func(), error)
	AttachRenderer(canvasID, rendererID string, visible bool) (<-chan authority.RenderRequest, func())
}

// Handler serves GET /v1/events.
func Handler(src Source, log *slog.Logger) http.Handler
```

### Problem codes (`httpx/problem.go`, beside `CodeConflict`)

```go
	CodeCanvasNotFound      = "canvas_not_found"      // 404
	CodeCanvasConflict      = "canvas_conflict"       // 409 ifRevision failed; details currentRevision, nodes
	CodeOpRejected          = "op_rejected"           // 422 a doc.OpError; details index, opCode, nodeId
	CodeAssetTooLarge       = "asset_too_large"       // 413 over 10 MiB
	CodeUnsupportedMedia    = "unsupported_media"     // 415 an asset type outside the four
	CodeRenderNotFound      = "render_not_found"      // 404 no such outstanding render request for this window
	CodeRendererUnavailable = "renderer_unavailable"  // 409 a route that needs a renderer found none (task 21)
	CodeDesignNotShared     = "design_not_shared"     // 409 the project is not bound to the Hub (task 21); details state
	CodeDesignNoBrief       = "design_no_brief"       // 409 no complete brief to approve against (task 21)
	CodeDesignTooLarge      = "design_too_large"      // 413 a snapshot over 500 files, 16 MiB a file or 512 MiB in all (task 21)
	CodeHandoffPathTracked  = "handoff_path_tracked"  // 409 .panacea/ is tracked in this worktree (task 22)
	CodeHandoffInvalid      = "handoff_snapshot_invalid" // 422 the Hub's snapshot fails Materialise's checks (task 22): ErrTampered, ErrUnsafePath or ErrTooLarge; details reason; nothing written
```

`local_key_required` (401) is Hub task 37's code, not this task's; every route here can answer it.

Hub codes reach the desktop unchanged through `hubclient.Error` (Hub contract §4.3).

```go
// control-plane/internal/designapi/briefs.go — the Briefs port (task 15) and task 21's brief needs

// BriefSource is phase 4's discoverhub.Service seen through neutral types, so
// this package compiles before phase 4 lands. briefs_phase4.go adapts
// *discoverhub.Service to it; until phase 4's task 07 exists, NewStubBriefs
// answers every call with state "unavailable" and the message
// "Discover (phase 4) is not built in this version."
type BriefSource interface {
	Head(ctx context.Context, projectID, branchID string) (BriefRef, error) // discoverhub.HeadRef
	Binding(ctx context.Context, projectID string) (HubBinding, error)      // discoverhub.Binding
	BranchRef(branchID string) string                                      // discoverhub.BranchRef
	Superseded(ctx context.Context, briefKey string) (bool, error)          // via Resolve + heads, phase 4's P4-B
}

type BriefRef struct {
	Key, Resolution, Status, Digest   string
	Revision, HeadRevision, LatestCompleteRevision int
	HubProjectID, BranchRef           string
	Title, Headline, Audience, Constraints, OutOfScope string
	Criteria []mcp.Criterion
	Sections []mcp.Section
}

type HubBinding struct {
	State        string // "bound" | "unbound" | "no_remote" | "hub_off" (phase 4's BindingState)
	HubProjectID string
}

func NewStubBriefs() BriefSource
func BriefsPort(src BriefSource) mcp.Briefs // maps Resolution + binding state to mcp.Brief.State
```

```go
// control-plane/internal/designapi/mount.go

type Config struct {
	Authority *authority.Service
	Briefs    BriefSource
	// Hub Designs — NOT in 20a's struct: task 21 adds the field with its type
	// (designapi/hub.go, Hub H0's hubapi) and the one cmd/api/design.go line that
	// passes Hub task 32's hubclient value (review I6: 20a compiles on 11–14 and 19, never on 21).
	Worktrees worktree.Provider // task 22
	Sessions  registry.Store    // task 22
	Identity  func(ctx context.Context) (displayName string) // the Hub's signed-in name, or "You"
	Log       *slog.Logger

	// KeyGuard is REQUIRED (review C1). main.go's newDesignAPI (task 20b) passes
	//   func(h http.Handler) http.Handler { return localkey.Middleware(key, localkey.Exempt, h) }
	// with the key localkey.Mint returned at start (Hub plan task 37). Mount wraps
	// EVERY design handler in it — routes 1–27, tasks 21, 22 and 29's included — so a
	// design route is key-guarded wherever it is mounted, not only because api.New
	// happens to wrap "/". Double-checking the same key under api.New's own wrap is
	// harmless.
	KeyGuard func(next http.Handler) http.Handler
}

// ErrNoKeyGuard: New refuses to build a human surface an unauthenticated
// caller could reach. There is no "run without the key" mode, no env var and
// no test-only bypass in production code; tests pass a real localkey.Middleware.
var ErrNoKeyGuard = errors.New("designapi: KeyGuard is required (Hub task C3, local caller key)")

type API struct{ /* unexported */ }

func New(c Config) (*API, error) // nil KeyGuard → ErrNoKeyGuard, before anything else is checked
func (a *API) Mount(handle func(pattern string, h http.Handler)) // api.RouteMounter; handle(p, a.keyGuard(h)) for every p
func (a *API) Approvals() mcp.Approvals // noApprovals{} until task 21 (approvals_none.go); the same value on every call
```

The two `PANACEA_APP_CANVAS_*` variables, `newDesignAPI` and the `main.go` wiring are task 20b's
(`task-20b.md`, *Contracts*).

## Tests first

| Test | Asserts | Kills the mutant |
|---|---|---|
| `TestRoutes_audit` (`routes_audit_test.go`) | routes 1, 3, 4, 10, 12, 18, 20 are in `catalogReadOnlyRoutes`; the audit passes | a GET added outside the audit |
| `TestOps_authorIsHuman` | a body with `"author":{…}` is `400 bad_request` (unknown field); the applied change's author is `human` | an actor field in a request (plan rule 4) |
| `TestOps_conflictAndRejected` | stale `ifRevision` → 409 `canvas_conflict` with `currentRevision`; a cycle → 422 `op_rejected` with `index` | errors flattened to 500 |
| `TestAssets_postGet` | POST a PNG → asset + revision; GET returns the bytes with `nosniff`; a JPEG body labelled PNG → 415; 10 MiB + 1 → 413 | a mislabelled asset served to the renderer |
| `TestRenderResults_bigBody` | a 20 MiB result is accepted (its own limit); 33 MiB → 413 | screenshots refused by the 1 MiB JSON cap |
| `TestRenderResults_wrongWindow` | a result for another window's request → 404 `render_not_found` | one window answering for another |
| `TestEvents_sseFormat` | a subscriber sees `event: canvas`, `id:`, `data:` lines, flushed per event, and a ping within 15 s (injected ticker) | buffered events arriving in bursts |
| `TestEvents_replayOnReconnect` | reconnect with `@41` after revisions 42–45 → four events | missed changes after a drop |
| `TestEvents_topicLimit` | 17 topics → 400 | unbounded fan-out |
| `TestEvents_phantomRenderer` | a renderer topic that answers nothing: after two timeouts it gets `gone` and requests go elsewhere | a stale window starving agents |
| `TestNew_refusesNoKeyGuard` | `New(Config{…everything but KeyGuard})` → `ErrNoKeyGuard`, no `*API` | the guard made optional again (review C1) |
| `TestRoutes_localKeyRequired` (table, one row per route class: 5 human ops, 8 revert, 9 break claim, 17 selection, 19 render result, 20 the stream) | through `Mount` with a real `localkey.Middleware` over a key minted into `t.TempDir()`: no `Authorization` → 401 `local_key_required` and **the authority unchanged** (revision, claims, selection, pending render requests all as before); `Bearer <wrong key>` → the same; `Authorization: Bearer ` (empty) → the same (plan HA5-6); the right key → not 401. Requests carry **no `Origin`** and `Host: 127.0.0.1:8730` — exactly what a sandboxed `curl` sends | a sandbox editing as `human`, reverting another agent, breaking claims, forging the selection, answering screenshot requests, or reading the stream (review C1) |
| `TestMount_wrapsEveryRoute` | walks every pattern `Mount` hands to `handle` (a recording `handle`) and sends each one keyless → 401; the count equals the route table's (20 here; task 21 raises it to 23, task 22 to 26 and task 29 to 27, and each of those parts carries its own row extending this test) | one route mounted without the wrap |
| `TestMount_everyMethodPassesPreflight` (review I3) | the same recording `handle`; for every recorded pattern, its method goes in a real preflight — `OPTIONS <path>`, `Origin: http://tauri.localhost` (from `httpx.DefaultOrigins()`), `Access-Control-Request-Method: <method>`, `Access-Control-Request-Headers: authorization, content-type` — through `httpx.NewOriginPolicy(httpx.DefaultOrigins(), log).Middleware`; the answer is 204 and its comma-split `Access-Control-Allow-Methods` contains the method, its `Access-Control-Allow-Headers` contains `Authorization` and `Content-Type`. Reads the live middleware, never a copied constant; tasks 21, 22 and 29's routes are covered the moment they mount | a route verb the preflight omits (route 11 or 17 back to `PUT`), which no `httptest` test notices because none enforces CORS |
| `TestPreflightAllowsEveryRegisteredMethod` (`control-plane/internal/api/routes_audit_test.go`, review I3) | the same check over `api.New(d).RegisteredPatterns()` — every route the control plane registers, phase 2's included — sent through `api.New`'s real handler; a pattern with no method (`catalogMethodAgnosticRoutes`) is skipped by name, not by a missing prefix | a phase-2 or later route whose verb the preflight omits |
| `TestBriefsPort_stub` | the stub answers `unavailable` with its sentence | a nil port panicking before phase 4 |
| `TestPhase3Branches_renames` (only with phase 3 in the tree) | a real `lanes` file store with two `AddRename` rows → `Renames` returns both, oldest first, field for field | a rename map read in the wrong order (a chain followed backwards) |
| `TestPhase3Branches_deletes` (only with phase 3) | a temp repository with `refs/panacea/archive/b-x-1a2b3c4d/1790000100`, `…/1790000000` and `refs/panacea/archive/b-y-5e6f7a8b/1790000050` → `{b-x-1a2b3c4d: [1790000000 s, 1790000100 s], b-y-5e6f7a8b: [1790000050 s]}` (both, ascending); no refs → an empty map; a ref `refs/panacea/archive/B;rm/1` or `…/b-x-1a2b3c4d/12ab` → skipped with a log line, the rest still returned; the recorded git argument list is the fixed one | a newest-only delete time hiding an earlier delete; a malformed ref name guessed at |
| `TestRoutes_renameAndDelete` (review I7, re-review RR-I7-3) | through `Mount` with a fake `BranchLifecycle`: route 2 creates `cv-1` on `b-old`; the fake renames `b-old → b-new`; route 1 on `b-new` → `cv-1`; the fake deletes `b-new` (a delete after `cv-1`'s `BoundAt`); route 1 on `b-new` → `{"canvas": null}`, route 2 → a new canvas id; a failing fake → `500 internal`, no canvas created. **Two more orderings** (task 11's RR-I7-1 and RR-I7-2 rows, through the routes): (a) `cv-1` on `b-login`, rename to `b-sign-in`, recreate the old name, route 1 on **`b-login` first** → `{"canvas": null}`, route 2 → a new id, then route 1 on `b-sign-in` → `cv-1`; (b) `b-x` deleted, then `cv-1` renamed onto `b-x`, route 1 on `b-x` **twice** → `cv-1` both times | the Design lens showing state 1 on a renamed branch, a recreated branch showing the old approvals, or an approved canvas archived on its second open |
| `TestApprovals_noneUntilTask21` | `Approvals().Latest(ctx, anyCanvas)` → `(ApprovalInfo{}, false, nil)`; two calls return equal values (task 20b's wiring test compares them) | a stub that errors (every `at: "approved"` read would fail instead of saying "not approved yet") |

`TestConfig_canvasVars`, `TestMain_wiring`, `TestNewDesignAPI_guardsBareMux` and the MCP wiring
tests are task 20b's.

## Steps

1. Tests with a `MemStore` authority and the fake renderer. 2. `events`, then `designapi`, then the
`httpx` codes, `api.go`, the audit. 3. If phase 4's `discoverhub` exists in
the tree, write `briefs_phase4.go`; else leave the stub in place and note it in the commit.
4. Acceptance; commit by pathspec. Nothing here edits `cmd/api` (task 20b).

## Acceptance

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/phase-5 && go test ./control-plane/... -race -count=1 && go vet ./control-plane/...
```
Expected: every package `ok`; vet silent. The process-level curl checks are task 20b's: until 20b
lands, `main.go` does not mount the design API.

## Not claimed

- `cmd/api` (the variables, `newDesignAPI`, the wiring) is task 20b.
- Approvals, handoff and session links are tasks 21 and 22.
- The desktop's reader is task 27; this task tests the stream with a Go client.
- `briefs_phase4.go` exists only if phase 4 is in the tree when this runs.
