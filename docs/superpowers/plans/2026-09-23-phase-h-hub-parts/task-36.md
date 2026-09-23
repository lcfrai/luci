### Task 36: the change-feed poller, `/v1/hub/changes`, and the cursor header (seam S-FEED, control-plane half)

**Lane C. Depends on: 26, 35. About 2 hours. Contract task C2.**

A teammate's approval must appear without anyone pressing refresh (`OD-3`). The control plane polls
the Hub's feed every `PANACEA_APP_HUB_POLL_INTERVAL` (5 s) while signed in — one request per user,
whatever the number of projects — and tells the desktop the cursor moved **without a new request
per tick**: it sets `Panacea-Hub-Cursor` on the responses of the call `poller.ts` already makes
every 2 s (`desktop/src/api/poller.ts:48`, contract §9.3). The desktop then reads the events once
(task 40). Server-side caches — `hublink`'s, and any phase's — subscribe here.

**Files:**
- `control-plane/internal/hubfeed/poller.go` + `poller_test.go` (new)
- `control-plane/internal/api/hubchanges.go` + `hubchanges_test.go` — the route and the header (new)
- `control-plane/internal/httpx/origin.go` + `origin_test.go` — expose the header (modify ⚑: one line and one test)
- `control-plane/internal/api/api.go` — `Deps.Feed`; the header wrap; `a.registerHubChanges(guarded)` (modify ⚑)
- `control-plane/internal/api/routes_audit_test.go` — `GET /v1/hub/changes` (modify ⚑)
- `control-plane/cmd/api/main.go` — start the poller; subscribe `hublink` (modify ⚑)

#### Contract — complete

```go
// control-plane/internal/hubfeed/poller.go

// Package hubfeed polls the Hub's change feed for the signed-in user and fans
// events out to in-process subscribers (seam S-FEED). It holds the cursor in
// memory only: after a restart it starts at "latest" again, because every
// cache it would invalidate is in memory too (ADR-H1 §5.4).
package hubfeed

// SignInState is the part of entraauth the poller needs.
type SignInState interface {
	Status() entraauth.Status
}

type Poller struct{ /* unexported */ }

func New(hub interface {
	hubclient.Feed
	hubclient.Projects
}, auth SignInState, interval time.Duration, log *slog.Logger) *Poller

// Run polls until ctx ends. Each tick:
//   - not signed in → nothing; the cursor is forgotten, so the next sign-in
//     starts from "latest" (and is a different user, perhaps);
//   - signed in, no cursor yet → Me() once (it binds pending invites, contract
//     §7.2 — the fallback when sign-in's own call could not reach the Hub, task 30),
//     then Changes("latest") to learn the head;
//   - otherwise Changes(cursor, 200), delivering each event to every subscriber
//     whose prefix matches its kind, in seq order, then advancing the cursor; while
//     "more" is true it asks again at once, at most 10 pages a tick;
//   - a Hub error: the cursor is kept and the tick ends (logged at debug once per
//     distinct error, not every 5 s).
func (p *Poller) Run(ctx context.Context)

// Cursor is the last "next" the Hub gave, or "" before the first good poll.
func (p *Poller) Cursor() string

// Subscribe registers fn for events whose kind starts with kindPrefix ("brief.",
// "design.", "release.", "project.", or "" for all). fn runs on the poller's
// goroutine and must not block. The returned func unsubscribes.
func (p *Poller) Subscribe(kindPrefix string, fn func(hubapi.Event)) (unsubscribe func())
```

```go
// control-plane/internal/httpx/origin.go — inside Middleware, beside the existing
// Access-Control-Allow-Origin line (origin.go:192-198), for an allowed origin only:
w.Header().Set("Access-Control-Expose-Headers", "Panacea-Hub-Cursor, Panacea-Auth-State")
```

Without it the webview can see the headers on the wire but `fetch` hides them from script
(contract §9.3).

**The headers.** `api.go` wraps the guarded mux so that **every** response to an allowed caller
carries:

- `Panacea-Hub-Cursor: <Cursor()>` once `Cursor()` is not empty (contract §9.3); and
- `Panacea-Auth-State: signed_out | signing_in | signed_in` whenever `Deps.Auth` is set — **an
  addition to the contract**, for the same reason as the cursor: the desktop learns that a sign-in
  finished, or that a refresh failed and the user must sign in again, from the poll it already
  makes, and calls `GET /v1/auth/status` only when this value changes (task 40). Without it the
  renderer would have to poll the status route every tick — a second request per tick, which
  phase 2's one-request rule (`desktop/src/api/poller.ts:1-3`) forbids.

Every response rather than only `GET /v1/sessions`: it needs no edit to phase 2's `sessions.go`, it
survives the `/api/v1` alias without a second case, and neither value reveals anything a caller
holding the local key could not ask for. The desktop reads them from the sessions poll only
(task 40).

**The route.** `GET /v1/hub/changes?after=<cursor>&limit=` → the Hub's `/v1/changes` body,
**unchanged** (`hubapi.ChangesPage`), or the relay (task 33). It goes in `catalogReadOnlyRoutes`.

**Subscribers wired in `main.go`:** `hublink.Resolver.Invalidate("")` on `project.`. Phases 4, 5 and
7 add their own in their own wiring (seam S-FEED).

#### Tests to write first

`poller_test.go` over `hubclient.NewFake()`, a scripted `SignInState` and a manual tick:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestTheFirstPollStartsAtLatest` | signed in with 5 events already in the fake → no event delivered; `Cursor()` = the head | a desktop that replays the org's history at every start |
| `TestEventsAreDeliveredInOrderByPrefix` | three writes → a `brief.` subscriber gets the `brief.revised` events in seq order; a `release.` subscriber gets none | an invalidation missed, or sent to the wrong cache |
| `TestMoreIsDrainedWithinATick` | 450 events, limit 200 → three pages in one tick; the eleventh page is left for the next tick | a backlog that takes minutes, or a tick that never ends |
| `TestSigningOutForgetsTheCursor` | signed in (cursor 5) → out → in as another user → a fresh `latest` | one user's cursor used for another's feed |
| `TestAHubOutageKeepsTheCursor` | `SetUnavailable(true)` for two ticks, then false → events after the cursor arrive; none lost; one debug line, not two | events skipped across an outage |
| `TestMeIsCalledOncePerSignIn` | two sign-ins → two `Me` calls; ticks in between → none | invites never bound when sign-in could not reach the Hub; `/v1/me` every 5 s |
| `TestUnsubscribe` | after unsubscribing, the fn gets nothing | — |

`hubchanges_test.go` and `origin_test.go`:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestTheCursorHeaderAppearsOnceKnown` | before the first good poll: no header; after: `GET /v1/sessions` carries `Panacea-Hub-Cursor: <n>` | a desktop that polls the feed every tick |
| `TestTheAuthStateHeader` | signed out → `Panacea-Auth-State: signed_out`; during a sign-in → `signing_in`; after → `signed_in`; with `Deps.Auth` nil → absent | a renderer polling `/v1/auth/status` every tick |
| `TestTheHeadersAreExposedToAllowedOrigins` | allowed origin → `Access-Control-Expose-Headers: Panacea-Hub-Cursor, Panacea-Auth-State`; a disallowed origin is refused before any header (phase 2's behaviour unchanged) | headers the webview cannot read (contract §9.3) |
| `TestHubChangesRelaysTheBody` | the Hub's page byte-equal; signed out → 401 `signed_out`; no Hub → 503 `hub_not_configured` | — |
| `TestHubChangesIsAudited` | phase 2's route audit passes | — |
| `TestProjectEventsInvalidateTheLinkCache` | a `project.remote_added` event → `hublink`'s next `Project` asks the Hub again | a teammate's share invisible for five minutes |

#### Steps

- [ ] **Step 1:** Re-read contract §9.3, §7.11; `desktop/src/api/poller.ts` (read only, for the tick).
- [ ] **Step 2:** Tests; they fail. Then `poller.go`, `hubchanges.go`, the `origin.go` line, the
  `api.go` wrap, the wiring.
- [ ] **Step 3:** Commit by pathspec.

#### Acceptance

```bash
go test ./control-plane/internal/hubfeed/ ./control-plane/internal/api/ ./control-plane/internal/httpx/ -race -count=1   # ok
git diff --stat HEAD~1 -- control-plane/internal/httpx/origin.go                                                     # 1 file changed, 1-3 insertions(+), 0 deletions
```

#### What this task does not claim

- SSE or WebSocket push (seam S-SSE).
- The desktop's reaction to the header: task 40.
- That five seconds is the right interval: it is `PANACEA_APP_HUB_POLL_INTERVAL`, measured in task 43.
