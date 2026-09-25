### Task 03: `hubapi`: the W1 wire types and request bodies

**Slice:** W1. **Size:** 3 focused hours (an estimate, not a measurement). **Stream:** WH.
**dependsOn:** 02. **Seams:** defines **SW.1**, second half: every W1 Work record, enum and
request body on the wire. SW.1 freezes at WS0 with task 05; after that, a change that is not an
added field, constant or type reopens task 05 and is announced (skeleton, *Start lines*).

The desktop, the control plane's relay, the fake and the real Hub all speak one wire. If each
wrote its own struct, one of them would spell `criterion_id` or drop `serves` when it is empty,
and the thread would break silently between two correct-looking programs. So every W1 Work type
lives in one file, `hubapi/work.go`, in `types.go`'s idiom, and tasks 04-17 and 37 use it and
declare nothing twice.

**Anchors, re-read on 2026-09-25 at `feat/hub` `9f2fa31`** (under
`control-plane/internal/hub/hubapi/`): `types.go:95-99` (`User`), `:196-205` (`Page[T]`,
`PageRequest`); `types_test.go:155` (`contractExamples`), `:179` (`TestFieldNamesMatchTheContractExamples`),
`:257` (`TestResponsesNeverOmitAField`), `:390` (`TestRequestsRejectUnknownFields`), and the
helpers they use (`exampleJSON`, `decodeInto`, `problemOf`, `jsonFields`, `jsonKeyPaths`,
`decodeAny`). Task 02's `workcanonical.go` gives `AuthoredText`, `Serve`, `IssueRevisionContent`,
`AuthorHuman`, `AuthorAgent`.

**Reconciled with the parts already written** (read on `feat/work-layer` at `a7720de`): task 08
declares `IssueWithRevision{Head, Revision}`, `IssueQuery{States, Assignee, Label}` and
`ProjectKey` "if task 03 has not"; task 11 needs `RelationBlocks`/`Duplicates`/`Relates`,
`StateTriage` … `StateDuplicate`, `PriorityNone` and a `TeamPolicy` whose `UpdatedAt`/`UpdatedBy`
are pointers; task 12 declares `ValidReactionEmoji`, the anchor kinds and its codes itself; task
13 owns the event kinds (`workevents.go`); task 22 uses `StateMove{To, Cause, Reason, LandCommit,
Evidence}` with `CauseLaneBound`, `CauseLaneQueued`, `CauseLandTrailer`. This part declares every
one of those spellings **here**, so tasks 08 and 11 drop their conditional declarations, and it
declares none of task 12's or 13's.

Six decisions the scope left open, made here.

**(a) `StateMove` has its full SW.4 shape now, and the W1 causes and result words are declared
here.** Task 04's fake enforces *"a fact state only with a fact cause"* and records a hand-set
`done` as not proven, so it needs the causes and `IssueHead.Proof` before task 10 exists. The
spellings are task 10's scope's, verbatim (`hand`, `lane_bound`, `lane_queued`, `land_trailer`,
`evidence_observed`, `evidence_recorded`; `proven`, `proven_self_attested`, `not_proven`,
`no_criteria`, `waiting`, `""`). Task 10 owns the **rule** (SW.4) and declares none of these
names again; `IssueHead.Proof` and `IssueHead.LandCommit`, which task 10's scope adds, are
declared here for the same reason.

**(b) `IssueHead.Alias` is on the wire and never in a request.** It is `DisplayAlias(key, number)`
at read time, `""` while the project has no key. No request type has an `alias` field, so a
strict decoder refuses one.

**(c) `PatchIssueRequest.AssigneeID` is a tri-state `Nullable[string]`.** Absent keeps, `null`
clears, a `u-` id sets. Task 08 otherwise decodes the body twice to tell absent from `null`; one
small generic type, with its own `UnmarshalJSON`, removes that, and task 08 drops its second
decode (its part says so).

**(d) `ValidReactionEmoji` is task 12's, not this task's.** The scope's `:custom:` test moves with
it: task 12's part declares the validator and its test, and one validator is better than two.

**(e) `ErasedAt *string` is on `IssueRevision` and `Comment`, as on `BriefRevision`**
(`types.go`'s brief revision carries `erasedAt`). Task 05's fake erases, and task 14 builds the
real erase; neither may add a field later without reopening SW.1.

**(f) Enum values are typed strings with a `Valid…` function and an ordered list** (`IssueStates`,
`Priorities`, …), so task 06's `CHECK` sets and task 07's `TestWorkCheckSetsMatchHubapi` compare
against one Go list, and a missing member is a compile-time or test-time failure, never a
silent `""`.

**Files** (under `control-plane/internal/hub/hubapi/`):

| Path | Change |
|---|---|
| `work.go` | new: every W1 Work enum, record and request body |
| `work_test.go` | new: the four `types_test.go` styles, run over the Work types |

No existing file changes. Later W1 tasks append to `work.go` additively (08, 09, 11, 12, 15),
each saying so; task 13 writes `workevents.go` beside it.

---

#### `work.go`, complete

The file is the concatenation of the five blocks below, in order.

```go
package hubapi

import (
	"bytes"
	"encoding/json"
	"slices"
)

// ── Phase W, W1: the Work wire (SW.1, second half) ─────────────────────────
// Issues are Hub records on the provenance thread (05-design §3). Every list
// field encodes as [] when empty and no response field has omitempty.

// IssueState is one of nine. There is no "shipped": it is derived from the
// release manifest, never stored (05-design §3.3).
type IssueState string

const (
	StateTriage    IssueState = "triage"
	StateBacklog   IssueState = "backlog"
	StateTodo      IssueState = "todo"
	StateStarted   IssueState = "started"   // fact: a lane was bound
	StateInReview  IssueState = "in_review" // fact: a bound lane was queued to land
	StateLanded    IssueState = "landed"    // fact: a land commit carries Panacea-Issue
	StateDone      IssueState = "done"      // proven, or hand-set with a reason
	StateCancelled IssueState = "cancelled"
	StateDuplicate IssueState = "duplicate"
)

// IssueStates is every state, in the order the product shows them.
var IssueStates = []IssueState{StateTriage, StateBacklog, StateTodo, StateStarted,
	StateInReview, StateLanded, StateDone, StateCancelled, StateDuplicate}

// FactStates are written only by the fact that causes them, as the person
// whose act caused it, and never by a sync or integration principal (rule 2).
var FactStates = []IssueState{StateStarted, StateInReview, StateLanded, StateDone}

// InitialStates are the only states an issue may be created in.
var InitialStates = []IssueState{StateTriage, StateBacklog, StateTodo}

func ValidIssueState(s IssueState) bool   { return slices.Contains(IssueStates, s) }
func IsFactState(s IssueState) bool       { return slices.Contains(FactStates, s) }
func ValidInitialState(s IssueState) bool { return slices.Contains(InitialStates, s) }

// Priority (L-13). "none" is a value, never an absent field.
type Priority string

const (
	PriorityNone   Priority = "none"
	PriorityUrgent Priority = "urgent"
	PriorityHigh   Priority = "high"
	PriorityMedium Priority = "medium"
	PriorityLow    Priority = "low"
)

var Priorities = []Priority{PriorityNone, PriorityUrgent, PriorityHigh, PriorityMedium, PriorityLow}

func ValidPriority(p Priority) bool { return slices.Contains(Priorities, p) }

// RelationKind is what a person writes (L-24). Task 11 adds the two read-only
// inverse words (blocked_by, duplicated_by) beside these.
type RelationKind string

const (
	RelationBlocks     RelationKind = "blocks"
	RelationDuplicates RelationKind = "duplicates"
	RelationRelates    RelationKind = "relates"
)

var RelationKinds = []RelationKind{RelationBlocks, RelationDuplicates, RelationRelates}
```
