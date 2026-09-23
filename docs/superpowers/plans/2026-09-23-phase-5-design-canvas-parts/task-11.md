# Task 11 — Authority I: canvases by branch, `Apply`, subscriptions, the activity feed

**Plan:** `../2026-09-23-phase-5-design-canvas.md` · **Depends on:** 06, 10 · **Hours:** 3.5 (3, plus `resolve` over the branch lifecycle, review I7)

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
	// LastDeleted returns when the cockpit last deleted branchID: the newest
	// <unix> of phase 3's refs/panacea/archive/<branchID>/<unix>, written on
	// every delete (phase 3 task-11.md:105,111). ok is false if never.
	// Whole seconds, because the ref name is.
	LastDeleted(ctx context.Context, projectID, branchID string) (at time.Time, ok bool, err error)
}

// ErrBranchLifecycle wraps a BranchLifecycle error. The authority FAILS CLOSED
// on it: no Bind, no Rebind, no Archive, no Ensure-create — guessing would
// either hand a new branch the old canvas and its approvals, or strand the
// approved design behind a fresh empty canvas.
var ErrBranchLifecycle = errors.New("authority: the branch lifecycle could not be read")
```

**Resolving `(projectID, branchID)` to a canvas** — one function, `resolve`, behind `Ensure`,
`ForBranch` and every session caller's canvas lookup (so the lens and MCP see the same answer):

1. **Deleted?** `Lookup` hits and the lookup is *fresh* — every `Ensure` and `ForBranch` (the
   lens's routes 1 and 2, task 20a, a human action) and a session's **first** call in this process
   (the authority keeps the set of session ids it has checked) → `LastDeleted(p, b)`. `ok` and
   `at ≥ binding.CreatedAt` truncated to the second → `Store.Archive(p, b, now)`, unload the canvas
   from memory, and treat it as a miss. A binding `Ensure` creates gets `CreatedAt =
   max(now, at + 1 s)` when `ok`, so a delete in the same second as a create is never read
   backwards. A branch deleted through the cockpit had no worktrees (phase 3 refuses
   `branch_has_worktrees`), so a session on the new branch of that name is always a new session,
   and its first call runs the check.
2. **Renamed?** On a miss, `Renames(p)`: every unarchived binding of `p` whose `BranchID` reaches
   `b` through the map (follow `from → to` until no row matches, at most `len(map)` steps, so a
   cycle ends) → `Store.Rebind(p, old, b)`, and the loaded canvas's `Summary.BranchID` becomes `b`.
   The result is a hit. Phase 3 refuses a rename while the branch has worktrees (its Q4), so no
   session is on the branch while it moves.
3. **Neither** → none: `ForBranch` answers `ok: false`; `Ensure` creates.

Scope (`apply.go` step 1) compares a session's `(ProjectID, BranchID)` with the **binding's**,
after `resolve`, never with `Document.BranchID`, which keeps the branch the canvas was created on
(it is in every snapshot; rewriting it would change past revisions' bytes).
`Config.Branches == nil` skips steps 1 and 2.

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
// otherwise the first event is a reset. The channel has a buffer of 256; a
// subscriber that falls that far behind is closed, and reconnects with its
// last revision. cancel is idempotent. Every slice in an Event is [] not null.
func (s *Service) Subscribe(canvasID string, after int) (events <-chan Event, cancel func(), err error)
```

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
| `TestSubscribe_replay` | subscribing at revision 10 of 40 yields 30 change events in order | a subscriber missing changes between reconnects |
| `TestSubscribe_reset` | subscribing at revision 0 of 1,500 yields one reset whose document equals the head | replaying 1,500 events on every reconnect |
| `TestSubscribe_slowIsClosed` | a subscriber that reads nothing is closed after 256 undelivered events; others keep receiving | one stuck window stalling every writer |
| `TestEvent_stateNotOps` | a `set_props` event carries the node's full post-state (all styles), not the delta | the desktop needing op semantics (P5-V) |
| `TestEvent_slicesNotNull` | a marshalled event has `[]` for every empty list | a desktop reader crashing on `null` |
| `TestDocument_at` | `Document(At{Revision: 5})` equals the live capture at 5 | `at: approved` reading the head |
| `TestActivity_truncates` | a 300-node batch's row lists 50 nodes and `more: 250` | an unbounded feed row |
| `TestIdleUnload` | after `IdleUnload` with no call and no subscriber the canvas is evicted; the next call reloads it equal | every canvas held in memory for ever |
| `TestResolve_followsRename` (review I7) | `Ensure(p, b-old)` → `cv-1`; a fake `BranchLifecycle` whose map is `{b-old → b-new}`; `ForBranch(p, b-new)` → `cv-1`, `Summary.BranchID = b-new`, and the store's `Bindings()` differs from before in that one row's `branchId` only; `ForBranch(p, b-old)` → none; `Ensure(p, b-new)` → `created: false` | the renamed branch showing state 1 while the approved design sits on a canvas no branch shows |
| `TestResolve_followsRenameChain` | map `{a → b, b → c}` → `ForBranch(p, c)` finds the canvas bound to `a`; a map with a cycle `{a → b, b → a}` ends (the call returns within the test's 1 s deadline) and rebinds nothing | a second rename losing the canvas; a hostile map hanging the lookup |
| `TestResolve_deleteArchives` (review I7) | `Ensure(p, b)` → `cv-1`, three batches applied (revision 3); the fake's `LastDeleted(p, b)` is 2 s after `cv-1`'s `CreatedAt`; `ForBranch(p, b)` → none; `Ensure(p, b)` → a **new** `cv-2`, `created: true`, revision 0; `Summary("cv-1")` still loads, unchanged; `Bindings()` holds `cv-1` archived | a new branch of a deleted branch's name inheriting its canvas and approvals |
| `TestResolve_sessionFirstCallChecks` | a session's first `Apply` runs `LastDeleted` once; its next 10 calls do not; a second session's first call does | a session on the recreated branch writing into the deleted branch's canvas; a git read on every agent call |
| `TestResolve_notDeletedKeeps` | `LastDeleted` not ok, or older than `cv-1`'s `CreatedAt` → `ForBranch` returns `cv-1`, nothing archived | a canvas archived on every open |
| `TestResolve_lifecycleErrorFailsClosed` | `Renames` or `LastDeleted` returns an error → `Ensure` and `ForBranch` return `ErrBranchLifecycle`; no `Bind`, `Rebind`, `Archive` or create reached the store (a recording store) | a phase-3 read failure creating an empty canvas over the approved one, or handing a deleted branch's canvas to a new one |
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
