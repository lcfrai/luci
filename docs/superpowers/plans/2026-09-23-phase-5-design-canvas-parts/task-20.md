# Task 20 — `designapi` routes, the SSE window stream, configuration, wiring

**Plan:** `../2026-09-23-phase-5-design-canvas.md` · **Depends on:** 11, 12, 13 · **External:** **Hub task C3 — Hub plan tasks 37–38 (`localkey`), a hard dependency: this task does not start until both have landed** (plan rule 4); phase 4 task 10 (`api.RouteMounter`), phase 4 task 07 (`discoverhub`) · **Hours:** 3

## Why

The desktop reaches the authority only through the control plane's loopback surface, as it reaches
everything else (Hub contract §5.4's rule, applied here). This task is that surface for everything
except approval and handoff (tasks 21, 22): the human's ops, undo, revert, comments, selection,
assets, the code panel, render results — and the **window stream** `GET /v1/events` (plan P5-B,
UI contract §10.5). It also wires the whole canvas stack into `cmd/api/main.go`.

## Files

- `control-plane/internal/designapi/` — `routes.go`, `canvas.go`, `ops.go`, `assets.go`, `comments.go`,
  `render.go`, `export.go`, `briefs.go`, `problems.go`, `mount.go` + tests (new)
- `control-plane/internal/events/broker.go`, `sse.go` + tests (new)
- `control-plane/internal/httpx/problem.go` — the codes below
- `control-plane/internal/api/api.go` — `Deps.Design RouteMounter`; in `New`, after the Discover mount: `if d.Design != nil { d.Design.Mount(guarded.Handle) }`
- `control-plane/internal/api/routes_audit_test.go` — the GET routes in `catalogReadOnlyRoutes`
- `control-plane/cmd/api/config.go`, `main.go` — two variables; the wiring

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
| 11 | PUT | `/v1/design/canvases/{canvasId}/assets/{sha}` | raw bytes; `Content-Type` one of the four image types; ≤ 10 MiB | `200 {"asset": doc.Asset, "revision": n}` — stored and `add_asset` applied as one human batch |
| 12 | GET | `/v1/design/canvases/{canvasId}/assets/{sha}` | — | the bytes; `Content-Type` as stored; `Cache-Control: private, max-age=31536000, immutable` (content-addressed); `X-Content-Type-Options: nosniff` |
| 13 | POST | `/v1/design/canvases/{canvasId}/threads` | `{"nodeId":"n-…","text":"…"}` | `201 doc.Thread` |
| 14 | POST | `/v1/design/canvases/{canvasId}/threads/{threadId}/messages` | `{"text":"…"}` | `201 doc.Message` |
| 15 | POST | `/v1/design/canvases/{canvasId}/threads/{threadId}/status` | `{"status":"open"\|"resolved"}` | `200 doc.Thread` |
| 16 | POST | `/v1/design/canvases/{canvasId}/reviews/{reviewId}/close` | `{}` | `200 doc.ReviewRequest` |
| 17 | PUT | `/v1/design/canvases/{canvasId}/selection` | `{"nodeIds":["n-…"]}` | `200 authority.Selection` |
| 18 | GET | `/v1/design/canvases/{canvasId}/export?nodeId=&format=html\|jsx-inline\|jsx-tailwind&at=` | — | `200 {"format":"…","text":"…"}` (the code panel) |
| 19 | POST | `/v1/design/render-results/{requestId}` | `authority.RenderResult`, ≤ 32 MiB (its own decoder, not `httpx.DecodeJSON`'s 1 MiB) | `204` |
| 20 | GET | `/v1/events?window={id}&topic=…` | — | `200 text/event-stream` (below) |

Every human route builds `authority.Caller{Author: {Kind: human, UserID: "local", Handle: <the
signed-in name from the Hub's /v1/auth/status, else "You">}}` — **no body field sets it** (plan
rule 4). Routes 21–27 (approval, handoff, links) are tasks 21 and 22.

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
	Hub       Designs          // task 21's port over hubclient.Designs; nil until task 21
	Worktrees worktree.Provider // task 22
	Sessions  registry.Store    // task 22
	Identity  func(ctx context.Context) (displayName string) // the Hub's signed-in name, or "You"
	Log       *slog.Logger

	// KeyGuard is REQUIRED (review C1). main.go passes
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
```

```go
// control-plane/cmd/api/config.go — two variables; neither is ever silently ignored (config.go:69-70)
	envCanvasMCP = "PANACEA_APP_CANVAS_MCP" // "on" (default) | "off"
	envCanvasDir = "PANACEA_APP_CANVAS_DIR" // absolute; default statepath.Dir("canvas")
```

**Wiring, in `main.go`**: `store.NewFile(canvasDir)` → `authority.New` →
`mcp.NewServer(info, mcp.Instructions, []string{"127.0.0.1:8731", "localhost:8731"}, log)` →
`mcp.RegisterAll(server, mcp.Deps{Authority, Briefs: designapi.BriefsPort(briefs), Approvals: …})`
→ `mcprelay.NewSupervisor{Sessions: store, Handler: server.Handler(), Release: auth.ReleaseSession}`
run under the process context → `designapi.New(…)` with `KeyGuard` built from the key `main.go`
already mints for `api.Deps.LocalKey` (Hub task 37; one key, one `Mint` call) into `api.Deps.Design`, and
`api.Deps.CanvasMCP = cfg.CanvasMCP`. Shutdown closes the supervisor, then the authority.

## Tests first

| Test | Asserts | Kills the mutant |
|---|---|---|
| `TestRoutes_audit` (`routes_audit_test.go`) | routes 1, 3, 4, 10, 12, 18, 20 are in `catalogReadOnlyRoutes`; the audit passes | a GET added outside the audit |
| `TestOps_authorIsHuman` | a body with `"author":{…}` is `400 bad_request` (unknown field); the applied change's author is `human` | an actor field in a request (plan rule 4) |
| `TestOps_conflictAndRejected` | stale `ifRevision` → 409 `canvas_conflict` with `currentRevision`; a cycle → 422 `op_rejected` with `index` | errors flattened to 500 |
| `TestAssets_putGet` | PUT a PNG → asset + revision; GET returns the bytes with `nosniff`; a JPEG body labelled PNG → 415; 10 MiB + 1 → 413 | a mislabelled asset served to the renderer |
| `TestRenderResults_bigBody` | a 20 MiB result is accepted (its own limit); 33 MiB → 413 | screenshots refused by the 1 MiB JSON cap |
| `TestRenderResults_wrongWindow` | a result for another window's request → 404 `render_not_found` | one window answering for another |
| `TestEvents_sseFormat` | a subscriber sees `event: canvas`, `id:`, `data:` lines, flushed per event, and a ping within 15 s (injected ticker) | buffered events arriving in bursts |
| `TestEvents_replayOnReconnect` | reconnect with `@41` after revisions 42–45 → four events | missed changes after a drop |
| `TestEvents_topicLimit` | 17 topics → 400 | unbounded fan-out |
| `TestEvents_phantomRenderer` | a renderer topic that answers nothing: after two timeouts it gets `gone` and requests go elsewhere | a stale window starving agents |
| `TestNew_refusesNoKeyGuard` | `New(Config{…everything but KeyGuard})` → `ErrNoKeyGuard`, no `*API` | the guard made optional again (review C1) |
| `TestRoutes_localKeyRequired` (table, one row per route class: 5 human ops, 8 revert, 9 break claim, 17 selection, 19 render result, 20 the stream) | through `Mount` with a real `localkey.Middleware` over a key minted into `t.TempDir()`: no `Authorization` → 401 `local_key_required` and **the authority unchanged** (revision, claims, selection, pending render requests all as before); `Bearer <wrong key>` → the same; `Authorization: Bearer ` (empty) → the same (plan HA5-6); the right key → not 401. Requests carry **no `Origin`** and `Host: 127.0.0.1:8730` — exactly what a sandboxed `curl` sends | a sandbox editing as `human`, reverting another agent, breaking claims, forging the selection, answering screenshot requests, or reading the stream (review C1) |
| `TestMount_wrapsEveryRoute` | walks every pattern `Mount` hands to `handle` (a recording `handle`) and sends each one keyless → 401; the count equals the route table's (20 here; tasks 21, 22 and 29 raise it to 27, and each extends this test) | one route mounted without the wrap |
| `TestBriefsPort_stub` | the stub answers `unavailable` with its sentence | a nil port panicking before phase 4 |
| `TestConfig_canvasVars` | `PANACEA_APP_CANVAS_MCP=maybe` exits with a message; `off` sets `CanvasMCP` false; a relative `PANACEA_APP_CANVAS_DIR` exits | a silently ignored variable |
| `TestMain_wiring` (the `main_test.go` pattern) | the process starts with a temp state dir and serves route 1 **with** the key read from `PANACEA_APP_LOCAL_KEY_FILE`; route 1 and route 5 **without** it, and with `Bearer ` (empty), → 401 `local_key_required` | a stack that compiles but is never mounted; `main.go` passing an identity `KeyGuard` |

## Steps

1. Tests with a `MemStore` authority and the fake renderer. 2. `events`, then `designapi`, then the
`httpx` codes, `api.go`, the audit, `config.go`, `main.go`. 3. If phase 4's `discoverhub` exists in
the tree, write `briefs_phase4.go`; else leave the stub wired and note it in the commit.
4. Acceptance; commit by pathspec.

## Acceptance

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/phase-5 && go test ./control-plane/... -race -count=1 && go vet ./control-plane/...
```
Expected: every package `ok`; vet silent.

```bash
go run ./control-plane/cmd/api & sleep 3; curl -s http://127.0.0.1:8730/v1/design/canvases/cv-0000000000000000 ; kill %1
```
Expected: `local_key_required` (401) — the curl sends no key. Then again with
`-H "Authorization: Bearer $(cat "$PANACEA_APP_LOCAL_KEY_FILE")"` (set that variable to a scratch path
before starting the process, Hub task 37): `{"error":"canvas_not_found",…}`.
Hub task C3 is a hard dependency, so there is no "before the key" answer to expect. (Run the binary,
not `go run`, if the orphan-process gotcha bites: build it, start it, kill it by PID.)

## Not claimed

- Approvals, handoff and session links are tasks 21 and 22.
- The desktop's reader is task 27; this task tests the stream with a Go client.
- `briefs_phase4.go` exists only if phase 4 is in the tree when this runs.
