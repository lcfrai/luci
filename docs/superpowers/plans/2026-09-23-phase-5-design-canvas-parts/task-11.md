# Task 11 — Authority I: canvases by branch, `Apply`, subscriptions, the activity feed

**Plan:** `../2026-09-23-phase-5-design-canvas.md` · **Depends on:** 06, 10 · **Hours:** 4 (3, plus `resolve` over the branch lifecycle, review I7, and the whole-project replay with its nine test rows, re-review RR-I7-1 and RR-I7-2)

## Why

The authority is the single writer (architecture rule 1). This task gives it its spine: a canvas
per branch (P5-L), one lock per canvas, the prepare → append → commit sequence that keeps memory and
disk agreeing, and a subscription that turns every accepted batch into an event carrying
**resulting state** (P5-V) — which the SSE stream (task 20a) delivers to the desktop and the MCP
tools read as notices (task 12). Claims, preconditions and undo are task 12's; renderers task 13's.

## Files

- `canvas/authority/service.go`, `binding.go`, `branches.go` (the branch-lifecycle port, seam
  S5-P3.2, review I7), `apply.go`, `subscribe.go`, `activity.go`, `errors.go` (new)
- `canvas/authority/*_test.go` (new); `canvas/canvastest/fixtures.go` (new: a document builder)

## Contracts

```go
// canvas/authority/service.go
package authority

type Config struct {
	Store   store.Store
	Checker doc.Checker        // dialect.Checker{} in production
	Now     func() time.Time   // UTC; tests inject
	Log     *slog.Logger
	// IdleUnload drops a canvas from memory after this long with no call and
	// no subscriber. Zero means 30 minutes.
	IdleUnload time.Duration
	// Branches is phase 3's branch lifecycle (branches.go, seam S5-P3.2). nil
	// until phase 3 is in the tree: bindings then follow neither a rename nor
	// a delete, exactly as before this seam.
	Branches BranchLifecycle
}

// Caller is who is calling, stamped by the adapter (plan rule 4): designapi
// for a human, the MCP relay for a session. Nothing in a request body can set it.
type Caller struct {
	Author    doc.Author
	ProjectID string // a session's own scope; "" for a human (the route names the canvas)
	BranchID  string
}

func (c Caller) IsHuman() bool

type Service struct{ /* unexported */ }

func New(cfg Config) (*Service, error)
func (s *Service) Close() error
```

```go
// canvas/authority/binding.go

type ArtboardSummary struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Width  string `json:"width"`  // the style value, e.g. "390px"
	Height string `json:"height"` // "844px" or "fit-content"
}

type PageSummary struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Artboards []ArtboardSummary `json:"artboards"` // by order
}

// Summary is get_canvas_info's core and the lens header's (tasks 15, 28).
// Claims and Presence are filled by task 12; until then they are [].
type Summary struct {
	CanvasID  string        `json:"canvasId"`
	ProjectID string        `json:"projectId"`
	BranchID  string        `json:"branchId"`
	Revision  int           `json:"revision"`
	CreatedAt string        `json:"createdAt"`
	Pages     []PageSummary `json:"pages"`
	Claims    []Claim       `json:"claims"`
	Presence  []Presence    `json:"presence"`
	Reviews   []doc.ReviewRequest `json:"reviews"` // open only
	FontsUsed []string      `json:"fontsUsed"` // distinct font-family first names, sorted
}

// Ensure returns the branch's canvas, creating it — one page, "Page 1" —
// when there is none. created reports which.
func (s *Service) Ensure(ctx context.Context, projectID, branchID string, by doc.Author) (sum Summary, created bool, err error)

// ForBranch is Ensure without the create: ok is false when there is none.
func (s *Service) ForBranch(ctx context.Context, projectID, branchID string) (sum Summary, ok bool, err error)

func (s *Service) Summary(ctx context.Context, canvasID string) (Summary, error)
```

```go
// canvas/authority/branches.go — seam S5-P3.2 (review I7)

// BranchRename is one row of phase 3's rename map (phase 3 task 11:
// GET /v1/projects/{projectId}/renames, {fromBranchId, toBranchId, at}).
type BranchRename struct {
	FromBranchID string
	ToBranchID   string
	At           time.Time
}

// BranchLifecycle is what the authority needs from phase 3. The control
// plane implements it (designapi/branches_phase3.go, task 20a); the canvas
// module never imports phase 3.
type BranchLifecycle interface {
	// Renames returns the project's rename map, oldest first.
	Renames(ctx context.Context, projectID string) ([]BranchRename, error)
	// Deletes returns EVERY cockpit delete of every branch of the project:
	// branch id → the <unix> suffixes of phase 3's
	// refs/panacea/archive/<branchID>/<unix>, written on every delete (phase 3
	// task-11.md:105,111), ascending. Whole seconds, because the ref name is.
	// Every delete, not the newest (re-review RR-I7-2): a replay needs the FIRST
	// delete after a binding arrived, and a later delete of a recreated branch
	// of the same name must not hide an earlier one. One git call per project.
	Deletes(ctx context.Context, projectID string) (map[string][]time.Time, error)
}

// ErrBranchLifecycle wraps a BranchLifecycle error. The authority FAILS CLOSED
// on it: no Bind, no Reconcile, no Ensure-create — guessing would
// either hand a new branch the old canvas and its approvals, or strand the
// approved design behind a fresh empty canvas.
var ErrBranchLifecycle = errors.New("authority: the branch lifecycle could not be read")
```

**Resolving `(projectID, branchID)` to a canvas** — one function, `resolve`, behind `Ensure`,
`ForBranch` and every session caller's canvas lookup (so the lens and MCP see the same answer):

A lookup is **fresh** for every `Ensure` and `ForBranch` (the lens's routes 1 and 2, task 20a, a
human action) and for a session's **first** call in this process (the authority keeps the set of
session ids it has checked). A branch deleted or renamed through the cockpit had no worktrees
(phase 3 refuses `branch_has_worktrees`, its Q4), so no session sits on a branch while it moves or
goes, and a session on a recreated branch is a new session whose first call is fresh.

A fresh lookup **reconciles the whole project first, on a hit as on a miss** (re-review RR-I7-1:
following renames only on a miss let a recreated branch of the old name hit the renamed branch's
binding and inherit its canvas and approvals). Under the authority's per-project mutex
(`binding.go`), so two fresh lookups never reconcile one project at once:

1. **Read.** `Renames(p)` and `Deletes(p)`, once each. An error → `ErrBranchLifecycle`, nothing
   changed (the fail-closed rule above).
2. **Replay each unarchived binding** of `p` from `(x, t) = (BranchID, BoundAt)` (task 10's
   `BoundAt`: when it *arrived* at `x`; never `CreatedAt`, which a move keeps, re-review RR-I7-2).
   Repeat:
   - `r` = the first rename row, in map order and after the last row this replay used, with
     `FromBranchID == x` and `At ≥ t`;
   - `d` = the first delete of `x` in `Deletes(p)[x]` with `d > t` truncated to the second.
   - **The tie rule.** Phase 3's delete times are whole seconds (the ref name), so a delete
     stamped second `s` is taken to precede every bind and rename in second `s`: it counts
     against an arrival only if `d > trunc(t)`, and it precedes a rename away if
     `d ≤ trunc(r.At)`.
   - `d` exists and (`r` does not, or `d ≤ trunc(r.At)`) → the fate is **archive at `d`**; stop.
   - else `r` exists → `(x, t) = (r.ToBranchID, r.At)`; repeat.
   - else → the fate is **bound to `(x, t)`**; stop.
   Each step consumes a later row, so the replay ends within `len(map)` steps; a map that names
   one id as `FromBranchID` in two rows (renamed away, recreated, renamed away again) is read in
   time order, never as one `from → to` edge, and a name cycle (`a → b`, then `b → a`) is just
   two steps.
3. **One owner per branch.** Two replays that end bound on one branch are possible only when a
   branch was deleted outside the cockpit (a bare `git branch -D` writes no archive ref). The
   later arrival `t` keeps the branch; each other is archived at that arrival, and the log names
   both canvas ids. The cockpit-only history the reviewer drew (`A → X`, `X` deleted, `C → X`)
   never reaches this step: A's replay meets the delete and is archived.
4. **Apply.** The fates that differ from the stored rows go to **one** `Store.Reconcile(p, fates)`
   (task 10: one `bindings.json` write, so a chain or a swap never has two bindings on one
   branch). Each loaded canvas that moved gets `Summary.BranchID = x`; each archived one is
   unloaded from memory.
5. **Look up.** `Lookup(p, b)` after the reconcile: a hit is the answer; a miss → `ForBranch`
   answers `ok: false`, `Ensure` creates with `CreatedAt = BoundAt = max(now, newest delete of
   b + 1 s)`, the tie rule's safety margin against a clock behind git's.

A non-fresh lookup (a session's later calls) is `Lookup` alone. Cost: one rename-map read and one
`for-each-ref` per fresh lookup, which is a human action or a session's first call.

**Residual, declared.** The tie rule is wrong for a rename onto a name followed by a delete of
that branch **in the same wall-clock second**: the delete reads as before the arrival, the canvas
stays bound, and a recreated branch of that name would inherit it. It needs a rename and then a
delete of the renamed branch inside one second. The reverse rule fails on the likelier order,
"delete `X`, then rename another branch onto `X`" (a clean-up that takes over a name), and there
it would archive the renamed canvas and strand its approved design. Phase 3 recording
sub-second delete times in its archive refs would close it; it is recorded as seam S5-P3.2's open
edge, not built.

Scope (`apply.go` step 1) compares a session's `(ProjectID, BranchID)` with the **binding's**,
after `resolve`, never with `Document.BranchID`, which keeps the branch the canvas was created on
(it is in every snapshot; rewriting it would change past revisions' bytes).
`Config.Branches == nil` skips the reconcile (steps 1–4).

```go
// canvas/authority/apply.go

// At names a revision to read: the head, or a past one.
type At struct {
	Head     bool
	Revision int // when !Head; 1 ≤ Revision ≤ head
}

func (s *Service) Document(ctx context.Context, canvasID string, at At) (doc.Document, error)

type ApplyRequest struct {
	Ops        []doc.Op
	Label      string // ≤ 120 runes
	IfRevision int    // 0: none. Enforced by task 12
	Shared     bool   // a session asking for a shared claim (task 12)
}

type ApplyResult struct {
	Revision int        `json:"revision"`
	Change   doc.Change `json:"change"`
	Notices  []Notice   `json:"notices"` // task 12; [] until then
}

// Apply is the only mutation. Under the canvas's lock, in this order:
//   1. scope: a session caller's (ProjectID, BranchID) must be the canvas's BINDING's (after resolve,
//      review I7), else ErrOutOfScope;
//   2. task 12's hooks: precondition, claims;
//   3. doc.Prepare (refusal → *doc.OpError, unchanged);
//   4. store.Append (failure → the error, memory unchanged);
//   5. Pending.Commit, then publish one Event, then task 12's post-commit hooks.
// Steps 1-5 are the unexported applyLocked(canvas, c, r), which requires the
// lock held; Apply is lock, applyLocked, unlock. Task 12's Undo, Redo and
// RevertSession call applyLocked inside their own hold, so their skip checks
// and their write are one critical section (re-review RR-M13-2).
func (s *Service) Apply(ctx context.Context, canvasID string, c Caller, r ApplyRequest) (ApplyResult, error)
```

```go
// canvas/authority/subscribe.go

type EventKind string

const (
	EventChange   EventKind = "change"   // one accepted batch
	EventReset    EventKind = "reset"    // the whole document: a subscriber too far behind, or new
	EventClaims   EventKind = "claims"   // claims or presence changed without a batch (task 12)
	EventSelected EventKind = "selection" // the human's selection changed (task 13)
)

// Event is what the desktop receives (plan P5-V: state, never ops).
type Event struct {
	CanvasID       string              `json:"canvasId"`
	Kind           EventKind           `json:"kind"`
	Revision       int                 `json:"revision"`
	At             string              `json:"at"`
	Author         *doc.Author         `json:"author"`   // null for reset and claims
	Label          string              `json:"label"`
	ContentChanged bool                `json:"contentChanged"`
	Nodes          []doc.Node          `json:"nodes"`    // full current state of created or modified nodes
	Deleted        []string            `json:"deleted"`
	Pages          []doc.Page          `json:"pages"`    // current state of created or modified pages
	DeletedPages   []string            `json:"deletedPages"`
	Tokens         []doc.Token         `json:"tokens"`
	DeletedTokens  []string            `json:"deletedTokens"`
	Assets         []doc.Asset         `json:"assets"`
	Threads        []doc.Thread        `json:"threads"`  // full current state of touched threads
	Reviews        []doc.ReviewRequest `json:"reviews"`
	Claims         []Claim             `json:"claims"`   // the canvas's full claim set, on change and claims events
	Presence       []Presence          `json:"presence"`
	Document       *doc.Document       `json:"document"` // reset only
	Selection      []string            `json:"selection"` // selection only
}

// Subscribe streams events after revision `after`. If the store holds every
// change since (at most 1,000 behind), they are replayed as change events;
// otherwise the first event is a reset. The channel's buffer is the replay's
// length plus 256, and the replay is in it before Subscribe returns, so a
// replay never blocks and never counts against the slow rule. A subscriber
// with 256 undelivered live events beyond its replay is closed, and
// reconnects with its last revision (re-review RR-M15-1). cancel is
// idempotent. Every slice in an Event is [] not null.
func (s *Service) Subscribe(canvasID string, after int) (events <-chan Event, cancel func(), err error)
```

**How a replayed event gets its state (review M15).** A replayed change event carries the state
*as of its own revision*, exactly as the live event for that revision did, never the head's. Under
the canvas's lock, so no live batch is missed or sent twice:

1. `after == head` → replay nothing: go to step 4 with an empty replay. `after > head` → one
   reset. Otherwise `cs, _ := Store.Changes(canvasID, after, 1000)`; if `cs` does not start at
   `after+1` or does not reach the head (more than 1,000 behind), send one reset instead
   (re-review RR-M15-2).
2. `d0, _ := Store.At(canvasID, after)`, then `d, _ := doc.FromDocument(d0)` (task 05).
3. For each `c` in `cs`, in order: `store.Replay(d, c)` (task 10), then build the event from `d`
   with the one function the live path uses (`eventFor(d, c)`): `Nodes` = `d.Node(id)` for each id
   in `c.Nodes`, `Pages` = `d.Page(id)` for each id in `c.Pages` still present (the rest go to
   `DeletedPages`), and likewise tokens, assets, threads and reviews. `Author`, `At`, `Label` and
   `ContentChanged` come from `c`.
4. `ch := make(chan Event, len(replay)+256)`, where `replay` is the slice steps 1–3 built (a
   reset counts as one), and put every replayed event in it. It fits by construction, so no send
   blocks under the lock and none trips the slow rule. Register the subscriber with `ch`, then
   release the lock. The slow rule then applies from here on: a live send that finds `ch` full
   closes the subscriber, which leaves 256 slots of headroom beyond the replay, the same as a
   subscriber at the head (re-review RR-M15-1). `Claims` and `Presence` are not logged, so every
   replayed event carries the current set; a subscriber keeps the last set it received.

One builder for live and replayed events means a replayed revision is byte-identical to the event a
subscriber connected at the time received (`TestSubscribe_replay`). Cost, per reconnect that is
behind: one `At` (at most 5,000 changes from a snapshot, task 10), up to 1,000 `Replay`s, and a
buffer of up to 1,256 events held until the subscriber drains it. **Every writer on the canvas
waits on its lock for the whole `At` and replay** (re-review RR-M15-2); the bound is the 5,000 +
1,000 changes above. `TestSubscribe_farBehind` pins the delivery.

```go
// canvas/authority/activity.go

type ActivityRow struct {
	Revision       int        `json:"revision"`
	At             string     `json:"at"`
	Author         doc.Author `json:"author"`
	Label          string     `json:"label"`
	Nodes          []string   `json:"nodes"`     // touched node ids, at most 50
	More           int        `json:"more"`      // how many more were touched
	Artboards      []string   `json:"artboards"`
	ContentChanged bool       `json:"contentChanged"`
}

// Activity is list_activity (spec §8.1) and the lens feed: rows after
// revision `after`, ascending, at most limit (≤ 200).
func (s *Service) Activity(ctx context.Context, canvasID string, after, limit int) ([]ActivityRow, error)
```

```go
// canvas/authority/errors.go — the typed errors every adapter maps (spec §8.4)

var (
	ErrNotFound            = errors.New("authority: no such canvas")
	ErrNoCanvas            = errors.New("authority: this branch has no canvas yet")
	ErrOutOfScope          = errors.New("authority: not this session's canvas")
	ErrRendererUnavailable = errors.New("authority: no renderer is attached") // task 13
	ErrTooLarge            = errors.New("authority: request too large")
)

type ConflictError struct { // task 12
	Current int      `json:"currentRevision"`
	Nodes   []string `json:"nodes"` // the nodes that moved past ifRevision
}

type ClaimedError struct { // task 12
	ArtboardID string     `json:"artboardId"`
	Holder     doc.Author `json:"holder"`
	ExpiresAt  string     `json:"expiresAt"`
}
```

## Tests first

| Test | Asserts | Kills the mutant |
|---|---|---|
| `TestEnsure_oncePerBranch` | two concurrent `Ensure` for one branch return one canvas id; the second reports `created: false` | a race creating two canvases (two bindings) |
| `TestEnsure_firstPage` | a new canvas has one page `Page 1` at revision 0 | a canvas no artboard can be put in |
| `TestApply_scope` | a session caller of branch A applying to branch B's canvas → `ErrOutOfScope`, nothing written | an agent on one branch drawing on another's canvas (spec §7.5 "Scope") |
| `TestApply_humanUnscoped` | a human caller applies to any canvas | the desktop locked out |
| `TestApply_diskFailure` | a store whose `Append` fails: `Apply` returns the error; `Document()` and the revision are unchanged; no event is published | memory ahead of disk after a failed write |
| `TestApply_orderUnderConcurrency` | 50 goroutines × 20 batches: revisions 1…1000 gapless; the store's log equals the event sequence | two batches committed under one revision |
| `TestSubscribe_replay` | 40 batches on a canvas with pages `p1` and `p2`, `n1` on `p1` (so batch 30's page delete leaves it, and batch 31 applies; re-review RR-M15-2), batch *k* setting node `n1`'s fill to a colour unique to *k*, batch 20 renaming page `p2` and batch 30 deleting it, while a subscriber from revision 0 records every live event. Subscribing at revision 10 yields 30 change events, revisions 11…40 in order, each equal as JSON to the live event of its revision (claims and presence aside, step 4): event 15's `n1` has colour 15, not 40, and event 20's `Pages` holds `p2` under its new name, though `p2` is gone at the head (review M15) | a subscriber missing changes between reconnects; a replay that sends the head's state for an old revision, or drops a page the head no longer has |
| `TestSubscribe_reset` | subscribing at revision 0 of 1,500 yields one reset whose document equals the head | replaying 1,500 events on every reconnect |
| `TestSubscribe_farBehind` (re-review RR-M15-1, RR-M15-2) | on a canvas at revision 1,300: subscribing at 1,000 (300 behind) and reading **nothing** while 10 more batches land, then draining → 310 change events, revisions 1,001…1,310 in order, on one connection, never closed; subscribing at 300 (1,000 behind) → 1,000 change events, then live ones; at 299 (1,001 behind) → one reset; at the head → no event until the next batch, whose event is a change, not a reset; at head + 1 → one reset. Each `Subscribe` returns within the test's 5 s deadline | a replay pushed into a 256-slot channel no one can read yet: a deadlock under the canvas lock, or a subscriber closed at event 257 that reconnects for ever; a reset sent to a subscriber that is already current |
| `TestSubscribe_slowIsClosed` | a subscriber at the head that reads nothing is closed at the 257th live event (256 undelivered fill its buffer); a subscriber that took a 30-event replay and reads nothing is closed at the 287th event it is owed, not before; others keep receiving | one stuck window stalling every writer |
| `TestEvent_stateNotOps` | a `set_props` event carries the node's full post-state (all styles), not the delta | the desktop needing op semantics (P5-V) |
| `TestEvent_slicesNotNull` | a marshalled event has `[]` for every empty list | a desktop reader crashing on `null` |
| `TestDocument_at` | `Document(At{Revision: 5})` equals the live capture at 5 | `at: approved` reading the head |
| `TestActivity_truncates` | a 300-node batch's row lists 50 nodes and `more: 250` | an unbounded feed row |
| `TestIdleUnload` | after `IdleUnload` with no call and no subscriber the canvas is evicted; the next call reloads it equal | every canvas held in memory for ever |
| `TestResolve_followsRename` (review I7) | `Ensure(p, b-old)` → `cv-1`; a fake `BranchLifecycle` whose map is `{b-old → b-new at t1}`; `ForBranch(p, b-new)` → `cv-1`, `Summary.BranchID = b-new`, and the store's `Bindings()` differs from before in that one row's `branchId` and `boundAt` (= t1) only; `ForBranch(p, b-old)` → none; `Ensure(p, b-new)` → `created: false` | the renamed branch showing state 1 while the approved design sits on a canvas no branch shows |
| `TestResolve_renameThenRecreateOldFirst` (re-review RR-I7-1) | `cv-1` on `b-login`; the map gains `{b-login → b-sign-in}`; **nobody opens `b-sign-in`**; the user recreates `feature/login` (same id `b-login`); `ForBranch(p, b-login)` — a **hit** before the reconcile — → none, and `Ensure(p, b-login)` → a new `cv-2`; then `ForBranch(p, b-sign-in)` → `cv-1`; a batch applied on `cv-2` stays on `cv-2` after both opens | a recreated old name hitting the renamed branch's binding and showing its canvas and approvals; a later open of the new name dragging the canvas away from the recreated branch |
| `TestResolve_followsRenameChain` | map `{a → b, b → c}` → `ForBranch(p, c)` finds the canvas bound to `a`; a name cycle `{a → b at t1, b → a at t2}` → the canvas is back on `a` with `BoundAt = t2`, and the call returns within the test's 1 s deadline; a map naming `a` as `from` twice (`a → b` at t1, recreate `a`, `a → c` at t3) with `cv-1` bound on `a` since before t1 and `cv-2` bound on `a` since t2 → `cv-1` on `b`, `cv-2` on `c` | a second rename losing the canvas; a hostile map hanging the lookup; one id read as one `from → to` edge |
| `TestResolve_swapInOneWrite` (re-review RR-I7-2) | `cv-A` on `x`, `cv-B` on `y`; map `{x → tmp at r1, y → x at r2, tmp → y at r3}`; one fresh lookup → `cv-A` on `y`, `cv-B` on `x`; the recording store saw **one** `Reconcile` call | a row-at-a-time apply failing `ErrExists` on a legal swap and leaving the lens on the wrong canvas |
| `TestResolve_deleteArchives` (review I7) | `Ensure(p, b)` → `cv-1`, three batches applied (revision 3); the fake's `Deletes(p)[b]` holds one time 2 s after `cv-1`'s `BoundAt`; `ForBranch(p, b)` → none; `Ensure(p, b)` → a **new** `cv-2`, `created: true`, revision 0; `Summary("cv-1")` still loads, unchanged; `Bindings()` holds `cv-1` archived | a new branch of a deleted branch's name inheriting its canvas and approvals |
| `TestResolve_renameOntoDeletedName` (re-review RR-I7-2) | `feature/x` deleted at t1 (`Deletes(p)[b-x] = [t1]`); `cv-1` created on `b-login` at t0 < t1 and approved; map `{b-login → b-x at t2}`, t2 > t1; `ForBranch(p, b-x)` → `cv-1`; a **second** `ForBranch(p, b-x)` → `cv-1` again, nothing archived; the same with t2 in the same second as t1 → `cv-1` (the tie rule) | an approved canvas archived on its second open because the delete is compared with `CreatedAt`, which the move kept |
| `TestResolve_deletedIncarnationNotRebound` (re-review RR-I7-2) | `cv-A` on `a`; map `{a → x at t1}`; `x` deleted at t2 (A's binding never reconciled in between); `cv-C` on `c`; map `{c → x at t3}`; one fresh lookup of `x` → `cv-C`, `cv-A` archived at t2, one `Reconcile`, no `ErrExists` | the deleted incarnation's canvas rebound first and the live one refused |
| `TestResolve_outOfCockpitDeleteTieBreak` | two replays end on `x` (a delete with no archive ref) → the later arrival keeps `x`, the other is archived at that arrival, and the log line names both canvas ids | two unarchived bindings on one branch, or an arbitrary winner |
| `TestResolve_sessionFirstCallChecks` | a session's first `Apply` runs `Renames` and `Deletes` once each; its next 10 calls do not; a second session's first call does | a session on the recreated branch writing into the deleted branch's canvas; a git read on every agent call |
| `TestResolve_notDeletedKeeps` | no delete of `b`, or only deletes at or before `cv-1`'s `BoundAt` second → `ForBranch` returns `cv-1`, nothing archived, no `Reconcile` call | a canvas archived on every open |
| `TestResolve_lifecycleErrorFailsClosed` | `Renames` or `Deletes` returns an error → `Ensure` and `ForBranch` return `ErrBranchLifecycle`; no `Bind`, `Reconcile` or create reached the store (a recording store) | a phase-3 read failure creating an empty canvas over the approved one, or handing a deleted branch's canvas to a new one |
| `TestResolve_nilLifecycle` | `Config.Branches` nil → no port call; behaviour equals the tests above without a rename or a delete | phase 5 unusable before phase 3 lands |
| `TestApply_scopeFollowsBinding` | after the rename above, a session on `b-new` applies to `cv-1`; a session on `b-old` → `ErrOutOfScope` | scope read from `Document.BranchID`, the creation branch |

## Steps

1. `canvastest/fixtures.go`: a builder that makes documents and batches for tests.
2. Tests, then the seven files. 3. Acceptance; commit by pathspec.

## Acceptance

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/phase-5 && go test ./canvas/authority/... ./canvas/canvastest/... -race -count=1
```
Expected: `ok  	…/canvas/authority` (and `[no test files]` for `canvastest`).

## Not claimed

- `IfRevision` and `Shared` are accepted and ignored until task 12; `Claims`, `Presence` and
  `Notices` are empty until then.
- No renderer calls, no selection (task 13).
- Events are in-process only; the SSE transport is task 20a.
- **A rename or delete done outside the cockpit** (`git branch -m`, `git branch -D` in a terminal)
  is not followed: phase 3 itself sees it as a delete plus a create and records no rename and no
  archive ref (phase 3 `task-11.md:198-199`), so a terminal rename leaves the canvas bound to the
  old id (not shown on any branch, still on disk), and a terminal delete-and-recreate inherits the
  canvas. The phase 3 adapter is task 20a's; before phase 3 lands, `Branches` is nil.
