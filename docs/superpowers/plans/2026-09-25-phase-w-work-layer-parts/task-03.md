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

```go
// ThreadStateKind is computed on every head read from the head revision
// (task 10); never set by a client.
type ThreadStateKind string

const (
	ThreadOnThread     ThreadStateKind = "on_thread"     // serves a branch brief's criterion
	ThreadProjectLevel ThreadStateKind = "project_level" // serves only the project brief's
	ThreadNotTied      ThreadStateKind = "not_tied"      // serves nothing; needs a reason
)

// NotTiedReason says why an issue is off the thread: never a silent open end
// (05-design §2.3 point 4).
type NotTiedReason string

const (
	NotTiedChore    NotTiedReason = "chore"
	NotTiedSpike    NotTiedReason = "spike"
	NotTiedImported NotTiedReason = "imported"
	NotTiedSupport  NotTiedReason = "support"
)

var NotTiedReasons = []NotTiedReason{NotTiedChore, NotTiedSpike, NotTiedImported, NotTiedSupport}

func ValidNotTiedReason(r NotTiedReason) bool { return slices.Contains(NotTiedReasons, r) }

// IssueSource is who opened the issue (04-internal-fit §2.4).
type IssueSource string

const (
	SourcePerson       IssueSource = "person"
	SourceOrchestrator IssueSource = "orchestrator"
	SourceIntake       IssueSource = "intake" // import, form, sync
)

var IssueSources = []IssueSource{SourcePerson, SourceOrchestrator, SourceIntake}

// SubscriptionCause is why a person is subscribed (L-117).
type SubscriptionCause string

const (
	SubscribeCreate  SubscriptionCause = "create"
	SubscribeAssign  SubscriptionCause = "assign"
	SubscribeMention SubscriptionCause = "mention"
	SubscribeComment SubscriptionCause = "comment"
	SubscribeManual  SubscriptionCause = "manual"
)

var SubscriptionCauses = []SubscriptionCause{SubscribeCreate, SubscribeAssign, SubscribeMention,
	SubscribeComment, SubscribeManual}

// MoveCause is why a state moved: the complete W1 set (SW.4; task 10 owns the
// rule, declared here so task 04's fake can hold it).
type MoveCause string

const (
	CauseHand             MoveCause = "hand"              // a person, among the non-fact states, or a hand-set done
	CauseLaneBound        MoveCause = "lane_bound"        // → started (task 22)
	CauseLaneQueued       MoveCause = "lane_queued"       // → in_review (task 22)
	CauseLandTrailer      MoveCause = "land_trailer"      // → landed, with LandCommit (task 22)
	CauseEvidenceObserved MoveCause = "evidence_observed" // → done, with Evidence (task 23)
	CauseEvidenceRecorded MoveCause = "evidence_recorded" // → done, with Evidence (task 23)
)

var MoveCauses = []MoveCause{CauseHand, CauseLaneBound, CauseLaneQueued, CauseLandTrailer,
	CauseEvidenceObserved, CauseEvidenceRecorded}

// Proof is the done rule's result word on IssueHead.Proof (SW.4). "" before landed.
type Proof string

const (
	ProofNone         Proof = ""
	ProofProven       Proof = "proven"               // every record observed
	ProofSelfAttested Proof = "proven_self_attested" // recorded by the issue's own starter
	ProofNotProven    Proof = "not_proven"           // a hand-set done: "done, not proven"
	ProofNoCriteria   Proof = "no_criteria"          // empty serves
	ProofWaiting      Proof = "waiting"              // landed; a criterion lacks evidence
)

var Proofs = []Proof{ProofNone, ProofProven, ProofSelfAttested, ProofNotProven, ProofNoCriteria, ProofWaiting}

// CommentKindComment is W1's only comment kind; W3 adds note,
// evidence_proposal and negotiation (SW.19, task 99).
const CommentKindComment = "comment"

// Import sources in W1 (SW.6; task 15 fixes the semantics, W5 adds more).
const (
	ImportSourceLinear = "linear"
	ImportSourceCSV    = "csv"
)
```

```go
// ── records ─────────────────────────────────────────────────────────────────

// ThreadState: Reason is "" unless Kind is not_tied, and then it is required.
type ThreadState struct {
	Kind   ThreadStateKind `json:"kind"`
	Reason NotTiedReason   `json:"reason"`
}

func (t ThreadState) Valid() bool {
	switch t.Kind {
	case ThreadOnThread, ThreadProjectLevel:
		return t.Reason == ""
	case ThreadNotTied:
		return ValidNotTiedReason(t.Reason)
	}
	return false
}

type Relation struct {
	Kind    RelationKind `json:"kind"`
	IssueID string       `json:"issueId"`
}

// ExternalRef names the tracker item an imported issue came from. AuthorName
// is display text (erasable), never a user id.
type ExternalRef struct {
	Source     string `json:"source"` // ImportSourceLinear | ImportSourceCSV (W1)
	ExternalID string `json:"externalId"`
	URL        string `json:"url"` // https, or ""
	AuthorName string `json:"authorName"`
}

// IssueHead is the mutable head, outside every digest. Alias is formatted at
// read time (DisplayAlias) and is "" while the project has no key.
type IssueHead struct {
	ID           string       `json:"id"`
	ProjectID    string       `json:"projectId"`
	Number       int          `json:"number"`
	Alias        string       `json:"alias"`
	State        IssueState   `json:"state"`
	StateReason  string       `json:"stateReason"` // a hand-set done's reason; else ""
	Proof        Proof        `json:"proof"`       // SW.4's result word; "" before landed
	LandCommit   string       `json:"landCommit"`  // 40 hex once landed; else ""
	Assignee     *User        `json:"assignee"`    // a person, never an agent
	Priority     Priority     `json:"priority"`
	Labels       []string     `json:"labels"` // lb- ids of the project's team
	Relations    []Relation   `json:"relations"`
	HeadRevision int          `json:"headRevision"`
	HeadKey      string       `json:"headKey"` // issue:is-…@headRevision
	ThreadState  ThreadState  `json:"threadState"`
	Source       IssueSource  `json:"source"`
	External     *ExternalRef `json:"external"`
	CreatedAt    string       `json:"createdAt"`
	CreatedBy    User         `json:"createdBy"`
	UpdatedAt    string       `json:"updatedAt"`
}

// IssueRevision is one immutable revision. Content is parsed from the stored
// canonical bytes; Digest is IssueRevisionDigest of them. An erased revision
// keeps Key, Digest and Revision; its texts read "" and ErasedAt is set.
type IssueRevision struct {
	IssueID      string               `json:"issueId"`
	Revision     int                  `json:"revision"`
	Key          string               `json:"key"`
	Digest       Digest               `json:"digest"`
	ProjectID    string               `json:"projectId"`
	Content      IssueRevisionContent `json:"content"`
	WrittenAt    string               `json:"writtenAt"`
	WrittenBy    User                 `json:"writtenBy"`
	RestoredFrom *int                 `json:"restoredFrom"` // the revision a restore copied
	ErasedAt     *string              `json:"erasedAt"`
}

// IssueWithRevision is a head with its head revision, content included, so a
// client can re-hash it (hub-contract §3.4).
type IssueWithRevision struct {
	Head     IssueHead     `json:"head"`
	Revision IssueRevision `json:"revision"`
}

// ProjectKey is the project's display key and who set it last.
type ProjectKey struct {
	ProjectID string `json:"projectId"`
	Key       string `json:"key"`
	SetAt     string `json:"setAt"`
	SetBy     User   `json:"setBy"`
}
```

```go
// Label is team-owned (L-17). Color is a token name ("accent-2"), never a hex.
type Label struct {
	ID     string `json:"id"` // lb-…
	TeamID string `json:"teamId"`
	Name   string `json:"name"`
	Color  string `json:"color"`
}

// TeamPolicy is a team's Work switches (L-9, L-48, 05-design §3.3). UpdatedAt
// and UpdatedBy are null for a team that never set one.
type TeamPolicy struct {
	TeamID                    string     `json:"teamId"`
	DefaultState              IssueState `json:"defaultState"` // one of InitialStates
	PriorityToLeaveTriage     bool       `json:"priorityToLeaveTriage"`
	StopAtLandedWithoutServes bool       `json:"stopAtLandedWithoutServes"`
	UpdatedAt                 *string    `json:"updatedAt"`
	UpdatedBy                 *User      `json:"updatedBy"`
}

// Comment is append-only: there is no edit route; a correction is a new one.
type Comment struct {
	ID         string         `json:"id"` // ic-…
	IssueID    string         `json:"issueId"`
	ParentID   *string        `json:"parentId"` // a top-level comment's id, or null
	Kind       string         `json:"kind"`     // CommentKindComment in W1
	Body       AuthoredText   `json:"body"`
	Anchor     *CommentAnchor `json:"anchor"`
	Mentions   []string       `json:"mentions"` // u- ids of team members
	Reactions  []Reaction     `json:"reactions"`
	ResolvedAt *string        `json:"resolvedAt"`
	ResolvedBy *User          `json:"resolvedBy"`
	CreatedAt  string         `json:"createdAt"`
	CreatedBy  User           `json:"createdBy"`
	ErasedAt   *string        `json:"erasedAt"`
}

// CommentAnchor pins a comment to a text range of one revision, a criterion
// that revision serves, a transcript span or a hunk (L-42). Fields that do not
// apply are 0 or "", never omitted; task 12 declares the kinds and the checks.
type CommentAnchor struct {
	Kind        string `json:"kind"` // text | criterion | span | hunk
	Revision    int    `json:"revision"`
	Start       int    `json:"start"`
	End         int    `json:"end"`
	CriterionID string `json:"criterionId"`
	Ref         string `json:"ref"` // a provenance key or a span id
}

type Reaction struct {
	Emoji string `json:"emoji"`
	Users []User `json:"users"`
}

type Subscription struct {
	IssueID    string            `json:"issueId"`
	User       User              `json:"user"`
	Cause      SubscriptionCause `json:"cause"`
	Subscribed bool              `json:"subscribed"`
}

// IssueRef names an issue an import created or found.
type IssueRef struct {
	IssueID    string `json:"issueId"`
	Number     int    `json:"number"`
	ExternalID string `json:"externalId"`
}

// ImportResult: Created and Existing hold one ref per item; UnmatchedLabels
// are names the team has no label for (reported, never created; task 15).
type ImportResult struct {
	Created         []IssueRef `json:"created"`
	Existing        []IssueRef `json:"existing"`
	UnmatchedLabels []string   `json:"unmatchedLabels"`
}
```

```go
// ── request bodies (decoded strictly: an unknown field is 400) ──────────────

// CreateIssueRequest writes the head and revision 1 in one write. State "" is
// the team policy's DefaultState; it must be one of InitialStates. Priority ""
// is none. NotTiedReason is required when Serves is empty.
type CreateIssueRequest struct {
	Title         AuthoredText  `json:"title"`
	Description   AuthoredText  `json:"description"`
	Serves        []Serve       `json:"serves"`
	Implements    []string      `json:"implements"`
	State         IssueState    `json:"state"`
	Priority      Priority      `json:"priority"`
	AssigneeID    *string       `json:"assigneeId"`
	Labels        []string      `json:"labels"`
	NotTiedReason NotTiedReason `json:"notTiedReason"`
}

// PatchIssueRequest changes head fields only and never writes a revision.
// It has no state field: a state moves only through StateMove. An absent
// field is kept. AssigneeID is tri-state: absent keeps, null clears, "u-…"
// sets; read it with AssigneeChange, write it with AssignTo or Unassign.
type PatchIssueRequest struct {
	Priority      *Priority       `json:"priority,omitempty"`
	AssigneeID    json.RawMessage `json:"assigneeId,omitempty"`
	Labels        *[]string       `json:"labels,omitempty"` // &[]string{} clears every label
	NotTiedReason *NotTiedReason  `json:"notTiedReason,omitempty"`
}

var jsonNull = []byte("null")

// Unassign is the AssigneeID that clears the assignee.
func Unassign() json.RawMessage { return json.RawMessage(jsonNull) }

// AssignTo is the AssigneeID that sets userID.
func AssignTo(userID string) json.RawMessage {
	b, _ := json.Marshal(userID)
	return b
}

// AssigneeChange reads AssigneeID: set is false when the field was absent;
// userID is "" when it was null. Anything but null or a string is an error.
func (r PatchIssueRequest) AssigneeChange() (set bool, userID string, err error) {
	if len(r.AssigneeID) == 0 {
		return false, "", nil
	}
	if bytes.Equal(bytes.TrimSpace(r.AssigneeID), jsonNull) {
		return true, "", nil
	}
	if err := json.Unmarshal(r.AssigneeID, &userID); err != nil || userID == "" {
		return true, "", &FieldError{"assigneeId", ErrBadEnvelope}
	}
	return true, userID, nil
}

// WriteIssueRevisionRequest: a stale BaseRevision is 409 revision_conflict and
// writes nothing, as briefs (hub-contract §7.5).
type WriteIssueRevisionRequest struct {
	BaseRevision int                  `json:"baseRevision"`
	Content      IssueRevisionContent `json:"content"`
}

// RestoreRevisionRequest writes a NEW revision copying FromRevision's content,
// with RestoredFrom set; history is never rewritten.
type RestoreRevisionRequest struct {
	FromRevision int `json:"fromRevision"`
}

// StateMove is SW.4's shape. LandCommit rides with land_trailer; Evidence (ev-
// ids) with the two evidence causes; Reason with a hand-set done.
type StateMove struct {
	To         IssueState `json:"to"`
	Cause      MoveCause  `json:"cause"`
	Reason     string     `json:"reason"`
	LandCommit string     `json:"landCommit"`
	Evidence   []string   `json:"evidence"`
}
```
