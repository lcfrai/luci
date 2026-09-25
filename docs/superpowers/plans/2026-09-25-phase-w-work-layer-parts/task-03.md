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

**(c) `PatchIssueRequest.AssigneeID` is a tri-state `json.RawMessage`.** Absent keeps, `null`
clears, a `u-` id sets. A `*string` cannot tell absent from `null` (encoding/json sets a pointer
to nil for both), and Go 1.23 has no `omitzero` for a struct wrapper, but a `RawMessage` field
is `nil` when absent, the bytes `null` when null, and omitted on marshal when `nil`.
`AssigneeChange()` reads it; `AssignTo(id)` and `Unassign()` write it. Task 08 otherwise decodes
the body twice to tell absent from `null`; with this it drops the second decode (its part says
so).

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

The file is the concatenation of the six blocks below, in order.

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

```go
type RelationRequest struct {
	Kind    RelationKind `json:"kind"`
	IssueID string       `json:"issueId"` // the other issue
}

type CreateLabelRequest struct {
	Name  string `json:"name"`
	Color string `json:"color"` // a token name
}

type PutTeamPolicyRequest struct {
	DefaultState              IssueState `json:"defaultState"`
	PriorityToLeaveTriage     bool       `json:"priorityToLeaveTriage"`
	StopAtLandedWithoutServes bool       `json:"stopAtLandedWithoutServes"`
}

type SetProjectKeyRequest struct {
	Key string `json:"key"` // ValidProjectKey; team admin only
}

type CreateCommentRequest struct {
	Body     AuthoredText   `json:"body"`
	ParentID *string        `json:"parentId"`
	Anchor   *CommentAnchor `json:"anchor"`
	Mentions []string       `json:"mentions"`
}

type ReactRequest struct {
	Emoji string `json:"emoji"`
	On    bool   `json:"on"`
}

type ResolveRequest struct {
	Resolved bool `json:"resolved"`
}

type SubscribeRequest struct {
	Subscribed bool `json:"subscribed"`
}

// ImportBatch is SW.6's shape; task 15 fixes its rules (1-500 items,
// idempotent per project by (source, externalId), all or nothing).
type ImportBatch struct {
	Source string       `json:"source"` // ImportSourceLinear | ImportSourceCSV
	Items  []ImportItem `json:"items"`
}

// ImportItem is one normalised external issue. Priority is already mapped by
// the adapter (task 25); Labels are names, matched to the team's labels.
type ImportItem struct {
	ExternalID  string   `json:"externalId"`
	URL         string   `json:"url"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Priority    Priority `json:"priority"`
	Labels      []string `json:"labels"`
	AuthorName  string   `json:"authorName"`
	CreatedAt   string   `json:"createdAt"`
}

// ── query parameters (not bodies: no json tags) ─────────────────────────────

// IssueQuery is GET …/issues' filter. A zero field does not filter.
type IssueQuery struct {
	States   []IssueState // ?state=, repeatable; each must be a known state
	Assignee string       // ?assignee=u-… or "none"
	Label    string       // ?label=lb-…
}

// IssueSearchQuery is GET /v1/issues/search: title substring, readable
// projects only, newest first (task 14).
type IssueSearchQuery struct {
	ProjectID string // "" searches every readable project
	Q         string // 1-200 characters
	Limit     int    // 0 is 50; at most 50
}
```

And the one method on task 02's type that texts outside a revision need (a comment's body, task
12; an import's texts, task 15), appended as the file's last lines:

```go
// ValidAuthor reports whether the author class is H or A. A revision's texts
// are checked by ValidateIssueRevisionContent; this is for every other text.
func (a AuthoredText) ValidAuthor() bool { return validAuthor(a.Author) }
```

That is the whole file. It declares no route, store, code or event: those are tasks 06-15's.

---

#### Tests first: `work_test.go`, complete, each test with the mutant it kills

Three blocks; the file is their concatenation. It reuses `types_test.go`'s helpers (`exampleJSON`
with its `USER`, `TIME`, `DIG` placeholders at `:147`, `jsonKeyPaths` `:112`, `jsonFields`
`:213`, `decodeInto` `:361`, `problemOf` `:367`) and `canonical_test.go:281`'s `decodeAny`, and
declares no helper of the same name.

```go
package hubapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"reflect"
	"slices"
	"testing"
)

// workExamples is one example per W1 record, every field present, every list
// non-empty, so a misspelt tag cannot hide behind an empty value.
var workExamples = []struct {
	label  string
	target func() any
	json   string
}{
	{"IssueHead", func() any { return new(IssueHead) }, `{"id":"is-3f2a91c07b1e4d2a","projectId":"pj-0123456789abcdef","number":142,"alias":"EP-142","state":"landed","stateReason":"","proof":"waiting","landCommit":"77aa310000000000000000000000000000000abc","assignee":USER,"priority":"high","labels":["lb-0123456789abcdef"],"relations":[{"kind":"blocks","issueId":"is-00000000000000c4"}],"headRevision":3,"headKey":"issue:is-3f2a91c07b1e4d2a@3","threadState":{"kind":"on_thread","reason":""},"source":"person","external":{"source":"linear","externalId":"ENG-9","url":"https://linear.app/x/issue/ENG-9","authorName":"A. Person"},"createdAt":TIME,"createdBy":USER,"updatedAt":TIME}`},
	{"IssueRevision", func() any { return new(IssueRevision) }, `{"issueId":"is-3f2a91c07b1e4d2a","revision":3,"key":"issue:is-3f2a91c07b1e4d2a@3","digest":DIG,"projectId":"pj-0123456789abcdef","content":{"title":{"text":"Resume","author":"H"},"description":{"text":"","author":"A"},"serves":[{"briefKey":"brief:bf-3f2a91c07b1e4d2a@5","criterionId":"c-91e2a0b3c4d5e6f7"}],"implements":["design:cv-7c01d2e3f4a5b6c7@11"]},"writtenAt":TIME,"writtenBy":USER,"restoredFrom":1,"erasedAt":null}`},
	{"Label", func() any { return new(Label) }, `{"id":"lb-0123456789abcdef","teamId":"tm-0123456789abcdef","name":"Infra","color":"accent-2"}`},
	{"TeamPolicy", func() any { return new(TeamPolicy) }, `{"teamId":"tm-0123456789abcdef","defaultState":"backlog","priorityToLeaveTriage":true,"stopAtLandedWithoutServes":false,"updatedAt":TIME,"updatedBy":USER}`},
	{"ProjectKey", func() any { return new(ProjectKey) }, `{"projectId":"pj-0123456789abcdef","key":"EP","setAt":TIME,"setBy":USER}`},
	{"Comment", func() any { return new(Comment) }, `{"id":"ic-0123456789abcdef","issueId":"is-3f2a91c07b1e4d2a","parentId":null,"kind":"comment","body":{"text":"Looks right","author":"H"},"anchor":{"kind":"text","revision":3,"start":0,"end":6,"criterionId":"","ref":""},"mentions":["u-00000000000000a2"],"reactions":[{"emoji":"👍","users":[USER]}],"resolvedAt":null,"resolvedBy":null,"createdAt":TIME,"createdBy":USER,"erasedAt":null}`},
	{"Subscription", func() any { return new(Subscription) }, `{"issueId":"is-3f2a91c07b1e4d2a","user":USER,"cause":"mention","subscribed":true}`},
	{"ImportResult", func() any { return new(ImportResult) }, `{"created":[{"issueId":"is-3f2a91c07b1e4d2a","number":1,"externalId":"ENG-9"}],"existing":[],"unmatchedLabels":["Infra"]}`},
	{"StateMove", func() any { return new(StateMove) }, `{"to":"done","cause":"evidence_observed","reason":"","landCommit":"77aa310000000000000000000000000000000abc","evidence":["ev-0123456789abcdef"]}`},
	{"CreateIssueRequest", func() any { return new(CreateIssueRequest) }, `{"title":{"text":"Resume","author":"H"},"description":{"text":"","author":"H"},"serves":[],"implements":[],"state":"triage","priority":"none","assigneeId":null,"labels":[],"notTiedReason":"chore"}`},
	{"ImportBatch", func() any { return new(ImportBatch) }, `{"source":"csv","items":[{"externalId":"7","url":"","title":"T","description":"","priority":"low","labels":["Infra"],"authorName":"","createdAt":TIME}]}`},
}

// Mutant: a tag spelt criterion_id, issueID or head_revision, or a field
// dropped from a struct. The example must decode strictly and come back with
// exactly its own keys.
func TestWorkFieldNamesMatchTheExamples(t *testing.T) {
	for _, ex := range workExamples {
		t.Run(ex.label, func(t *testing.T) {
			in := []byte(exampleJSON(ex.json))
			dst := ex.target()
			dec := json.NewDecoder(bytes.NewReader(in))
			dec.DisallowUnknownFields()
			if err := dec.Decode(dst); err != nil {
				t.Fatalf("the example does not decode into %T: %v", dst, err)
			}
			out, err := json.Marshal(dst)
			if err != nil {
				t.Fatalf("json.Marshal(%T): %v", dst, err)
			}
			if got, want := jsonKeyPaths(decodeAny(t, out), nil), jsonKeyPaths(decodeAny(t, in), nil); !slices.Equal(got, want) {
				t.Errorf("%T round trip:\n got keys %v\nwant keys %v", dst, got, want)
			}
		})
	}
}
```

```go
var workResponseTypes = []any{IssueHead{}, IssueRevision{}, IssueWithRevision{}, ThreadState{},
	Relation{}, ExternalRef{}, ProjectKey{}, Label{}, TeamPolicy{}, Comment{}, CommentAnchor{},
	Reaction{}, Subscription{}, IssueRef{}, ImportResult{}}

var workRequestTypes = []any{CreateIssueRequest{}, PatchIssueRequest{}, WriteIssueRevisionRequest{},
	RestoreRevisionRequest{}, StateMove{}, RelationRequest{}, CreateLabelRequest{},
	PutTeamPolicyRequest{}, SetProjectKeyRequest{}, CreateCommentRequest{}, ReactRequest{},
	ResolveRequest{}, SubscribeRequest{}, ImportBatch{}}

// Mutant: omitempty on any response field; the desktop indexes serves,
// labels, relations, mentions and reactions, and a missing key reads as
// undefined there, never as [].
func TestWorkResponsesNeverOmitAField(t *testing.T) {
	for _, v := range workResponseTypes {
		typ := reflect.TypeOf(v)
		for _, f := range jsonFields(typ) {
			if f.omitempty {
				t.Errorf("%s.%s has omitempty; a response never omits a field", typ.Name(), f.name)
			}
		}
		b, _ := json.Marshal(v)
		var got map[string]json.RawMessage
		if err := json.Unmarshal(b, &got); err != nil || len(got) != len(jsonFields(typ)) {
			t.Errorf("%s{} encodes %d keys, want %d (%v)", typ.Name(), len(got), len(jsonFields(typ)), err)
		}
	}
}

// Mutant: a list field whose empty value does not survive the wire as [].
// A decoded [] must re-encode as [], never null and never absent.
func TestWorkEmptyListsStayArrays(t *testing.T) {
	var h IssueHead
	if err := json.Unmarshal([]byte(`{"labels":[],"relations":[]}`), &h); err != nil {
		t.Fatal(err)
	}
	var c Comment
	if err := json.Unmarshal([]byte(`{"mentions":[],"reactions":[]}`), &c); err != nil {
		t.Fatal(err)
	}
	var r IssueRevisionContent
	if err := json.Unmarshal([]byte(`{"serves":[],"implements":[]}`), &r); err != nil {
		t.Fatal(err)
	}
	for name, v := range map[string]any{"IssueHead": h, "Comment": c, "IssueRevisionContent": r} {
		b, _ := json.Marshal(v)
		var m map[string]json.RawMessage
		_ = json.Unmarshal(b, &m)
		for _, k := range []string{"labels", "relations", "mentions", "reactions", "serves", "implements"} {
			if raw, ok := m[k]; ok && string(raw) != "[]" {
				t.Errorf("%s.%s re-encodes as %s, want []", name, k, raw)
			}
		}
	}
}

// Mutants: a lenient decoder; a "state":"done" smuggled into a patch (a state
// moves only through StateMove, where the done rule runs); an alias, number or
// createdBy accepted from a client.
func TestWorkRequestsRejectUnknownFields(t *testing.T) {
	for _, v := range workRequestTypes {
		typ := reflect.TypeOf(v)
		for _, body := range []string{`{"bogus":1}`, `{"createdBy":"u-0123456789abcdef"}`, `{"alias":"EP-1"}`} {
			rec, ok := decodeInto(reflect.New(typ).Interface(), body)
			if ok || rec.Code != http.StatusBadRequest || problemOf(t, rec).Error != CodeBadRequest {
				t.Errorf("%s with %s: ok=%v %d, want 400 bad_request", typ.Name(), body, ok, rec.Code)
			}
		}
	}
	for _, body := range []string{`{"state":"done"}`, `{"state":"landed","priority":"high"}`, `{"number":7}`} {
		if rec, ok := decodeInto(new(PatchIssueRequest), body); ok || rec.Code != http.StatusBadRequest {
			t.Errorf("PatchIssueRequest %s: ok=%v %d, want 400", body, ok, rec.Code)
		}
	}
}
```

```go
// Mutant: an issue created already started, landed or done, which would skip
// the done rule; or a "shipped" state, which 05-design §3.3 derives and never stores.
func TestWorkStateSets(t *testing.T) {
	if len(IssueStates) != 9 || len(FactStates) != 4 || len(InitialStates) != 3 {
		t.Fatalf("states %d, facts %d, initial %d; want 9, 4, 3", len(IssueStates), len(FactStates), len(InitialStates))
	}
	for _, s := range IssueStates {
		if s == "shipped" {
			t.Error(`IssueStates holds "shipped", which is derived, never a state`)
		}
		if IsFactState(s) && ValidInitialState(s) {
			t.Errorf("%s is both a fact and an initial state", s)
		}
	}
	for _, s := range FactStates {
		if ValidInitialState(s) {
			t.Errorf("ValidInitialState(%q) = true; no issue is created in a fact state", s)
		}
	}
	if ValidIssueState("shipped") || ValidIssueState("") || ValidInitialState("") {
		t.Error(`"shipped" or "" accepted as a state`)
	}
}

// Mutant: a duplicate or empty member in any value list, which task 06's
// CHECK sets and task 07's TestWorkCheckSetsMatchHubapi are compared against.
func TestWorkValueListsAreSets(t *testing.T) {
	lists := map[string][]string{
		"IssueStates": strs(IssueStates), "Priorities": strs(Priorities),
		"RelationKinds": strs(RelationKinds), "NotTiedReasons": strs(NotTiedReasons),
		"IssueSources": strs(IssueSources), "SubscriptionCauses": strs(SubscriptionCauses),
		"MoveCauses": strs(MoveCauses),
	}
	want := map[string]int{"IssueStates": 9, "Priorities": 5, "RelationKinds": 3,
		"NotTiedReasons": 4, "IssueSources": 3, "SubscriptionCauses": 5, "MoveCauses": 6}
	for name, l := range lists {
		c := slices.Clone(l)
		slices.Sort(c)
		if len(slices.Compact(c)) != len(l) || slices.Contains(l, "") || len(l) != want[name] {
			t.Errorf("%s = %v: want %d distinct non-empty values", name, l, want[name])
		}
	}
	if len(Proofs) != 6 || Proofs[0] != ProofNone {
		t.Errorf("Proofs = %v: want six, the first \"\"", Proofs)
	}
}

func strs[T ~string](xs []T) []string {
	out := make([]string, len(xs))
	for i, x := range xs {
		out[i] = string(x)
	}
	return out
}

// Mutant: not_tied with no reason (05-design §2.3's "silent open end"), or a
// reason carried by an issue that is on the thread.
func TestThreadStateValid(t *testing.T) {
	for _, c := range []struct {
		ts   ThreadState
		want bool
	}{
		{ThreadState{ThreadOnThread, ""}, true},
		{ThreadState{ThreadProjectLevel, ""}, true},
		{ThreadState{ThreadNotTied, NotTiedImported}, true},
		{ThreadState{ThreadNotTied, ""}, false},
		{ThreadState{ThreadNotTied, "later"}, false},
		{ThreadState{ThreadOnThread, NotTiedChore}, false},
		{ThreadState{"", ""}, false},
	} {
		if got := c.ts.Valid(); got != c.want {
			t.Errorf("%+v.Valid() = %v, want %v", c.ts, got, c.want)
		}
	}
}

// Mutant: an author class other than H or A accepted on a comment or import text.
func TestAuthoredTextValidAuthor(t *testing.T) {
	for a, want := range map[string]bool{"H": true, "A": true, "": false, "h": false, "agent": false, "HA": false} {
		if got := (AuthoredText{Text: "x", Author: a}).ValidAuthor(); got != want {
			t.Errorf("author %q: ValidAuthor() = %v, want %v", a, got, want)
		}
	}
}

// Mutants: absent read as "clear"; null read as "keep"; a number accepted as
// a user id; AssignTo or Unassign writing a shape AssigneeChange cannot read.
func TestAssigneeChangeIsTriState(t *testing.T) {
	for _, c := range []struct {
		body       string
		set        bool
		user       string
		wantErrors bool
	}{
		{`{}`, false, "", false},
		{`{"assigneeId":null}`, true, "", false},
		{`{"assigneeId":"u-00000000000000a2"}`, true, "u-00000000000000a2", false},
		{`{"assigneeId":7}`, true, "", true},
		{`{"assigneeId":""}`, true, "", true},
	} {
		var r PatchIssueRequest
		if err := json.Unmarshal([]byte(c.body), &r); err != nil {
			t.Fatalf("%s: %v", c.body, err)
		}
		set, user, err := r.AssigneeChange()
		if set != c.set || user != c.user || (err != nil) != c.wantErrors {
			t.Errorf("%s: (%v, %q, %v), want (%v, %q, error=%v)", c.body, set, user, err, c.set, c.user, c.wantErrors)
		}
	}
	for _, r := range []PatchIssueRequest{{}, {AssigneeID: Unassign()}, {AssigneeID: AssignTo("u-00000000000000a2")}} {
		b, _ := json.Marshal(r)
		var back PatchIssueRequest
		_ = json.Unmarshal(b, &back)
		s1, u1, _ := r.AssigneeChange()
		s2, u2, _ := back.AssigneeChange()
		if s1 != s2 || u1 != u2 {
			t.Errorf("%s does not round-trip: (%v, %q) became (%v, %q)", b, s1, u1, s2, u2)
		}
	}
}
```

Every test fails to compile before Step 3 (`undefined: IssueHead` and the rest).
