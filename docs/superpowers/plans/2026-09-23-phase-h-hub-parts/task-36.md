### Task 36: the change-feed poller, `/v1/hub/changes`, and the cursor header (seam S-FEED, control-plane half)

**Lane C. Depends on: 26, 35. About 2 hours. Contract task C2.**

A teammate's approval must appear without anyone pressing refresh (`OD-3`). The control plane polls
the Hub's feed every `PANACEA_APP_HUB_POLL_INTERVAL` (5 s) while signed in — one request per user,
whatever the number of projects — and tells the desktop the cursor moved **without a new request
per tick**: it sets `Panacea-Hub-Cursor` on the responses of the call `poller.ts` already makes
every 2 s (`desktop/src/api/poller.ts:48`, contract §9.3). The desktop then reads the events once
(task 40). Server-side caches — `hublink`'s, and any phase's — subscribe here. The header is one
instance of a generic mechanism, `CursorSource` (HA7-4, task 00): phase 7's `Panacea-Deploy-Cursor`
is a second source, not a second copy.

**Files:**
- `control-plane/internal/hubfeed/poller.go` + `poller_test.go` (new)
- `control-plane/internal/api/hubchanges.go` + `hubchanges_test.go` — the route and the header (new)
- `control-plane/internal/api/cursors.go` + `cursors_test.go` — `CursorSource`, the header wrap, the expose list (new; HA7-4, task 00)
- `control-plane/internal/api/api.go` — `Deps.Feed`, `Deps.CursorSources`; the wrap; `a.registerHubChanges(guarded)` (modify ⚑)
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
// goroutine and must not block. Events arrive in seq order, which is commit
// order (task 14's org lock serialises writes). The returned func unsubscribes.
// Phase 4's HA4-8 shape, Subscribe(fn), is Subscribe("", fn) (task 00).
func (p *Poller) Subscribe(kindPrefix string, fn func(hubapi.Event)) (unsubscribe func())

// CursorName makes the poller a CursorSource named "Hub" (below).
func (p *Poller) CursorName() string
```

```go
// control-plane/internal/api/cursors.go — the generic mechanism phase 7 asked for (HA7-4, task 00)

// CursorSource is one change cursor the desktop learns from the poll it already
// makes. Every response to an allowed caller carries "Panacea-<Name>-Cursor:
// <Cursor()>" for each source whose Cursor() is not "".
type CursorSource interface {
	CursorName() string // [A-Za-z]{1,32}, unique among the sources IGNORING CASE: "Hub", "Deploy"
	Cursor() string     // opaque; "" means "do not set the header"
}

// exposeHeaders is built once by api.New from Deps.CursorSources:
// "Panacea-<Name>-Cursor" for each, then "Panacea-Auth-State", comma-separated.
// api.New refuses a name outside the pattern or a duplicate, as it refuses any
// other bad Deps. A duplicate is compared case-insensitively ("Hub" and "HUB"
// are one header name in HTTP, and one key, "hub", in the desktop's cursors map,
// task 40).
func exposeHeaders(sources []CursorSource) (string, error)
```

**The headers.** `api.go` wraps the guarded mux (the wrap is in `cursors.go`) so that **every**
response to an allowed caller carries:

- `Access-Control-Expose-Headers: <exposeHeaders>` — without it the webview can see the headers
  on the wire but `fetch` hides them from script (contract §9.3). The wrap sets it, not
  `httpx/origin.go`: the names are known only from `Deps`, and `httpx` stays free of Hub and
  phase-7 names. The wrap sits inside the origin middleware, so a disallowed origin is refused
  before it runs, and a preflight is answered before it runs;
- `Panacea-<Name>-Cursor: <Cursor()>` for each source whose `Cursor()` is not empty — the poller
  is the first source, `Panacea-Hub-Cursor` (contract §9.3); phase 7 appends its `Deploy` source
  in its own wiring, one element of `Deps.CursorSources`; and
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
| `TestMoreIsDrainedWithinATick` | 450 events, limit 200 → three `Changes` calls (200, 200, 50) in **one** tick; all 450 delivered in seq order; `Cursor()` = the head after that tick | a backlog that takes minutes: `more` ignored, so one page a tick |
| `TestAtMostTenPagesATick` | 2,200 events, limit 200 (eleven pages) → **ten** `Changes` calls in the first tick, 2,000 events delivered, `Cursor()` at event 2,000; the second tick makes the eleventh call and delivers the last 200 (plan review M2: one scenario per test) | a tick that never ends: the ten-page cap dropped, or counted per event rather than per page |
| `TestSigningOutForgetsTheCursor` | signed in (cursor 5) → out → in as another user → a fresh `latest` | one user's cursor used for another's feed |
| `TestAHubOutageKeepsTheCursor` | `SetUnavailable(true)` for two ticks, then false → events after the cursor arrive; none lost; one debug line, not two | events skipped across an outage |
| `TestMeIsCalledOncePerSignIn` | two sign-ins → two `Me` calls; ticks in between → none | invites never bound when sign-in could not reach the Hub; `/v1/me` every 5 s |
| `TestUnsubscribe` | after unsubscribing, the fn gets nothing | — |

`hubchanges_test.go` and `cursors_test.go`:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestTheCursorHeaderAppearsOnceKnown` | before the first good poll: no header; after: `GET /v1/sessions` carries `Panacea-Hub-Cursor: <n>` | a desktop that polls the feed every tick |
| `TestEverySourceSetsItsOwnHeader` | two scripted sources `Hub` ("7") and `Deploy` ("c-3") → both `Panacea-Hub-Cursor: 7` and `Panacea-Deploy-Cursor: c-3`; `Deploy` returning `""` → its header absent, `Hub`'s still set | phase 7's second cursor needing its own copy of the mechanism (HA7-4) |
| `TestABadOrDuplicateSourceNameIsRefused` | `api.New` with a source named `Hub-2`, `""`, 33 letters, two named `Hub`, or `Hub` and `HUB` → an error naming the source | a header name the desktop cannot parse, or two sources overwriting one header (header names, and the desktop's lowercased keys, ignore case) |
| `TestTheAuthStateHeader` | signed out → `Panacea-Auth-State: signed_out`; during a sign-in → `signing_in`; after → `signed_in`; with `Deps.Auth` nil → absent | a renderer polling `/v1/auth/status` every tick |
| `TestTheHeadersAreExposedToAllowedOrigins` | allowed origin, sources `Hub` and `Deploy` → `Access-Control-Expose-Headers: Panacea-Hub-Cursor, Panacea-Deploy-Cursor, Panacea-Auth-State`; a disallowed origin is refused before any header, and a preflight is answered as before (phase 2's behaviour unchanged) | headers the webview cannot read (contract §9.3) |
| `TestHubChangesRelaysTheBody` | the Hub's page byte-equal; signed out → 401 `signed_out`; no Hub → 503 `hub_not_configured` | — |
| `TestHubChangesIsAudited` | phase 2's route audit passes | — |
| `TestProjectEventsInvalidateTheLinkCache` | a `project.remote_added` event → `hublink`'s next `Project` asks the Hub again | a teammate's share invisible for five minutes |

#### Steps

- [ ] **Step 1:** Re-read contract §9.3, §7.11; `desktop/src/api/poller.ts` (read only, for the tick).
- [ ] **Step 2:** Tests; they fail. Then `poller.go`, `hubchanges.go`, `cursors.go`, the
  `api.go` wrap and `Deps.CursorSources` (the poller its first element), the wiring.
- [ ] **Step 3:** Commit by pathspec.

#### Acceptance

```bash
go test ./control-plane/internal/hubfeed/ ./control-plane/internal/api/ ./control-plane/internal/httpx/ -race -count=1   # ok
git diff --stat HEAD~1 -- control-plane/internal/httpx/origin.go                                                     # empty: origin.go is not this task's (task 00, HA7-4)
```

#### What this task does not claim

- SSE or WebSocket push (seam S-SSE).
- The desktop's reaction to the header: task 40.
- That five seconds is the right interval: it is `PANACEA_APP_HUB_POLL_INTERVAL`, measured in task 43.
