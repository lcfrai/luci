# Task 12 — Authority II: claims, `ifRevision`, notices, per-author undo, revert by session

**Plan:** `../2026-09-23-phase-5-design-canvas.md` · **Depends on:** 08, 11 · **Hours:** 3.5 (3, plus token-change notices through task 08's `serialize.TokensUsed`, review I11)

## Why

This is where we beat Paper (spec §6, *"Parallel agents are not addressed"*; §7.4). Five rules:

1. **Claims.** A session's first write into an artboard claims it — Paper's working indicator
   promoted to a lease. Another session's write into it fails `claimed`, naming the holder and the
   expiry, unless both ask for a shared claim. **Humans are never blocked** and never claim.
2. **Preconditions.** A write may carry `ifRevision`; if any node it modifies or deletes has moved
   past it, the write fails `conflict` with the current revision and the nodes, instead of silently
   overwriting. Without it, per-property last-writer-wins stands (task 06).
3. **Notices.** A human edit inside a session's claimed artboard succeeds, and the session is told on
   its next call — every tool result carries `notices` (spec §7.4 rule 2). **A token change is an
   edit to every artboard that uses the token**: tokens are canvas-wide and claim nothing, so when
   anyone else sets or deletes a token, every session holding an artboard whose subtree uses it is
   told, with the token and the author (review I11). Without this, T restyles S's claimed artboard
   through `var(--brand)` and S never hears of it: a silent collision.
4. **Per-author undo.** A human's undo undoes the human's own batches, never an agent's.
5. **Revert by session.** *"Revert everything `hero-v2` did"*, as one batch, from the log
   (`AGENTIC-UX.md` §4 rule 6: reversible with a rewind, not a manual unpick).

## Files

- `canvas/authority/claims.go`, `precondition.go`, `notices.go`, `presence.go`, `undo.go`, `revert.go` (new)
- `canvas/authority/apply.go` — the two hooks named in task 11, plus the token-use capture around
  `Commit` (below; review I11). `canvas/authority` now imports `canvas/serialize` (task 08), which
  builds on tasks 05 and 07 only (`doc`, `dialect`) and never on `authority`, so no import cycle
- tests beside each

## Contracts

```go
// canvas/authority/claims.go

const ClaimLease = 120 * time.Second // renewed by every write, start_working or read by a holder

type Claim struct {
	ArtboardID string       `json:"artboardId"`
	Holders    []doc.Author `json:"holders"` // sessions; more than one only when Shared
	Shared     bool         `json:"shared"`
	Since      string       `json:"since"`
	ExpiresAt  string       `json:"expiresAt"`
}

// StartWorking claims artboards explicitly (the start_working tool, spec §8.2).
// All or nothing: ClaimedError for the first artboard another session holds
// exclusively. A human caller is refused (ErrHumanNoClaim): humans never claim.
func (s *Service) StartWorking(ctx context.Context, canvasID string, c Caller, artboardIDs []string, shared bool) ([]Claim, error)

// FinishWorking releases the caller's claims on those artboards, or on every
// artboard of the canvas when artboardIDs is empty (finish_working).
func (s *Service) FinishWorking(ctx context.Context, canvasID string, c Caller, artboardIDs []string) error

// ReleaseSession drops every claim a session holds on every canvas. The relay
// supervisor calls it when the session's channel closes for good (task 19).
func (s *Service) ReleaseSession(sessionID string)

// BreakClaim lets a HUMAN release another session's claim (REST only; spec
// §8.4 keeps this out of MCP). The session sees a notice.
func (s *Service) BreakClaim(ctx context.Context, canvasID string, c Caller, artboardID string) error

var ErrHumanNoClaim = errors.New("authority: humans do not claim artboards")
```

**The claim rule inside `Apply`** (before Prepare's result is committed; the artboards are the
prepared `Change.Artboards`, so a move from A to B needs both):

| Caller | Artboard state | Result |
|---|---|---|
| human | any | allowed; no claim taken; holders get a notice |
| session S, `Shared: false` | unclaimed or lapsed | S claims it exclusively |
| session S | held by S | renewed |
| session S, `Shared: false` | held by T (exclusive or shared) | `ClaimedError` naming T; the batch is refused whole |
| session S, `Shared: true` | shared, held by T | S joins the holders |
| session S, `Shared: true` | exclusive, held by T | `ClaimedError` |

Lapsed claims are dropped lazily on every check and by one ticker in the service every 5 s, which
publishes a `claims` event when it drops any. (The ticker is Go-side; the desktop adds no interval,
G10.)

```go
// canvas/authority/precondition.go

// checkIfRevision: when r.IfRevision > 0, every node the prepared change
// MODIFIES or DELETES (created nodes are exempt; a replace-mode write_html
// deletes, so its old subtree counts) must have Revision ≤ r.IfRevision,
// else *ConflictError{Current: head, Nodes: those that moved}, sorted.
// Tokens and pages carry no revision and are not checked. A token write is
// last-writer-wins per token; what keeps it from being SILENT is the token
// notice below (review I11), not a precondition.
```

```go
// canvas/authority/notices.go

type Notice struct {
	Revision  int        `json:"revision"`
	At        string     `json:"at"`
	Author    doc.Author `json:"author"`
	Label     string     `json:"label"`
	Artboards []string   `json:"artboards"` // the caller's held artboards this change reached, sorted
	Nodes     []string   `json:"nodes"`     // at most 20; [] for a change that reached them only through tokens
	Tokens    []string   `json:"tokens"`    // set or deleted tokens those artboards use, sorted; [] when none (review I11)
	Text      string     `json:"text"`      // e.g. "Dylan Mackie edited 3 layers in Settings (revision 41)"; a broken claim: "Dylan Mackie released your claim on Settings"; a token: "design-2 changed the token brand, which Settings uses (revision 44)"
}

// Notices returns, for a SESSION caller, every change by someone else since
// the caller's last call on this canvas that REACHED an artboard the caller
// holds or held at that time, oldest first, at most 20 (then one summary
// notice), and advances the caller's mark. A human caller gets [].
// A change reaches artboard A when it touched A (Change.Artboards) or when it
// set or deleted a token A uses (the token uses recorded for that revision,
// below). One change is at most one notice, whichever way it reached.
// Every MCP tool result carries this list (task 15, 16, 17).
func (s *Service) Notices(canvasID string, c Caller) []Notice

// tokenUses: for a batch whose ops include set_token or delete_token, Apply
// records, under the canvas lock, which artboards use which of the batch's
// tokens, as tokenUses[revision] = map[artboardID][]tokenName:
//   before Commit: pre[A]  = names of serialize.TokensUsed(doc, A) for every artboard A
//                  (the document is still the pre-batch state, so a DELETED
//                  token and an old value's references are seen);
//   after Commit:  post[A] = the same over the committed document (a NEW
//                  token, and a new value's references, are seen);
//   uses[A] = (pre[A] ∪ post[A]) ∩ Change.Tokens, kept where non-empty.
// TokensUsed is transitive (a token referenced by a token A uses counts, task
// 08), so changing `brand` reaches an artboard that uses only `button:
// var(--brand)`. A token batch touches no artboard, so it claims nothing and
// no claim refuses it: the notice is how holders learn of it. The cost is two
// walks of every artboard, for token batches only. The record is in memory,
// like the claims and marks it is read against (a restart loses all three
// together); an entry at or below every present session's mark is pruned.
```

```go
// canvas/authority/presence.go

// Presence is one session on the canvas, for the lens and get_canvas_info.
type Presence struct {
	Author    doc.Author `json:"author"`
	Artboards []string   `json:"artboards"` // claimed now
	Working   bool       `json:"working"`   // holds at least one claim
	LastAt    string     `json:"lastAt"`    // its last batch or tool call
	LastLabel string     `json:"lastLabel"`
}
// A session is present while it holds a claim or has called within 10 minutes.
```

```go
// canvas/authority/undo.go

type UndoResult struct {
	Revision int      `json:"revision"` // the new batch's revision; 0 when nothing was left to undo
	Undid    int      `json:"undid"`    // the revision undone
	Skipped  []string `json:"skipped"`  // node ids whose state someone else changed since
	Message  string   `json:"message"`  // e.g. "Undid Move 3 layers. 1 layer was changed by design-2 since and was left as it is."
}

// Undo undoes the caller's own most recent content batch (never another
// author's), as a new batch labelled "Undo: <label>" by the same author.
// Each inverse op is applied only where the current state still equals what
// the undone batch left: a property only if it still has the value the batch
// set; a move only if the node is still where the batch put it; a re-create
// only if the id is free and the parent exists; a delete only if no one else
// has edited the node since. Skipped nodes are listed, never forced.
// Stacks are per author and per canvas, in memory, 200 deep; a new content
// batch by the author clears their redo stack. A restart clears the stacks
// (revert by session, which reads the log, survives it).
func (s *Service) Undo(ctx context.Context, canvasID string, c Caller) (UndoResult, error)
func (s *Service) Redo(ctx context.Context, canvasID string, c Caller) (UndoResult, error)
```

```go
// canvas/authority/revert.go

type RevertResult struct {
	Revision int      `json:"revision"` // 0 when the session changed nothing that is still there
	Batches  int      `json:"batches"`  // how many of the session's batches were reverted
	Skipped  []string `json:"skipped"`
	Message  string   `json:"message"`
}

// RevertSession reverts every content batch sessionID made on the canvas
// with Revision > since (0: all), newest first, with Undo's skip rules, as
// ONE batch labelled "Revert <handle>" authored by the caller. Human callers
// only (the lens's activity feed, spec §7.4 rule 4).
func (s *Service) RevertSession(ctx context.Context, canvasID string, c Caller, sessionID string, since int) (RevertResult, error)
```

## Tests first

| Test | Asserts | Kills the mutant |
|---|---|---|
| `TestClaims_firstWriteClaims` | S writes into artboard A; `Summary.Claims` shows A held by S, expiring in 120 s | a write that never claims |
| `TestClaims_secondSessionRefused` | T's write into A → `*ClaimedError{ArtboardID: A, Holder: S}`; nothing written | the spec's done-when: *"on one artboard the second gets claimed"* |
| `TestClaims_twoArtboardsNoCollision` | S on A and T on B, 200 interleaved batches each, all accepted | the other done-when: *"two agents on two artboards never collide"* |
| `TestClaims_moveNeedsBoth` | S holds A; T holds B; S moving a node from A into B → `ClaimedError{B}` | a move checked only at the source |
| `TestClaims_humanNeverBlocked` | the human edits inside A while S holds it: accepted; S's next `Notices` names the human | a human locked out of their own canvas |
| `TestClaims_shared` | S and T both `Shared: true` on A: both accepted; T exclusive on A shared by S → refused | a shared claim that admits an exclusive writer |
| `TestClaims_lapse` (injected clock) | 121 s after S's last touch, T's write succeeds and a `claims` event was published | a claim that never lapses (a crashed agent locks the artboard) |
| `TestClaims_finishAndRelease` | `FinishWorking` with no ids releases every claim; `ReleaseSession` does too across canvases | claims leaking past the session |
| `TestClaims_humanCannotClaim` | `StartWorking` as a human → `ErrHumanNoClaim` | humans claiming by accident |
| `TestIfRevision_conflict` | S reads at 10; the human changes node N at 11; S's `set_props` on N with `IfRevision: 10` → `*ConflictError{Current: 11, Nodes: [N]}` | a stale agent write silently overwriting |
| `TestIfRevision_createdExempt` | inserting children under a parent changed after `IfRevision` succeeds | conflicts on pure inserts |
| `TestIfRevision_replaceCountsSubtree` | a replace whose old subtree has a node changed after `IfRevision` → conflict | replace escaping the precondition |
| `TestNotices_advance` | two consecutive calls: the second has no repeat of the first's notices | notices repeating for ever |
| `TestNotices_tokenChange` | S holds *Settings* and T holds *Billing*, both drawn with `var(--brand)`; U holds *Profile*, which uses no token. T's `set_token` on `brand` → S's next `Notices` is one notice: `Author` T, `Tokens: [brand]`, `Artboards: [Settings]`, `Nodes: []`, a `Text` naming `brand` and T's handle; U's is `[]`; T's is `[]` (its own change) | a notice only for a change that touched the caller's artboard (review I11); a notice to every holder, whether its artboard uses the token or not |
| `TestNotices_tokenTransitiveAndDelete` | *Settings* uses only `var(--button)`, and `button` is `var(--brand)`. The human sets `brand` → S is told, `Tokens: [brand]`. The human then deletes `button` → S is told, `Tokens: [button]` | uses read one level deep (`brand` missed); uses read only after the batch (the deleted `button` is unknown then, and missed) |
| `TestUndo_ownOnly` | human batch H1, agent batch A1, human batch H2: undo → H2; undo → H1; A1 intact | undo taking the agent's work |
| `TestUndo_skipsChangedByOthers` | human sets colour; agent changes the same colour; human undo skips that node and says so | undo clobbering someone's later edit |
| `TestUndo_redoCleared` | undo, then a new human batch; redo → nothing to redo | redo resurrecting a branch of history |
| `TestRevert_session` | session S made 5 batches across 2 artboards; one node was later edited by the human; revert → one batch, 5 reverted, 1 skipped, S's other work gone | revert as five batches (five undo steps), or forced over the human's edit |
| `TestRevert_humanOnly` | a session caller → error | an agent reverting another agent |

## Steps

1. Tests with an injected clock. 2. Implement, wiring the two hooks into `apply.go`. 3. Acceptance;
commit by pathspec.

## Acceptance

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/phase-5 && go test ./canvas/authority/... -race -count=1
```
Expected: `ok  	github.com/kpmg-au-emu/panacea-app/canvas/authority`.

## Not claimed

- Undo stacks do not survive a restart (stated in the contract). Revert by session does.
- No shared vocabulary with phase 3's claims (plan P5-R).
- Token edits carry no precondition: tokens have no revision, and a token write is last-writer-wins
  per token. It is never silent: every session holding an artboard that uses the token is told
  (review I11). A per-token `ifRevision` was the review's optional extra and is not built: it
  would add a revision to `doc.Token` (task 05) and a `tokens` field to `ConflictError` (task 11),
  the MCP error map (task 15) and the REST 409 (task 20) for a collision the notice already
  reports. It is the seam if agents are seen overwriting each other's tokens.
