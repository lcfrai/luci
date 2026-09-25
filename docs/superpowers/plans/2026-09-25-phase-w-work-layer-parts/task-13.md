### Task 13: subscriptions; the change feed's Work kinds, filtered per caller

W1, stream WH, seam **SW.5** (the Work feed kinds and their metadata-only payloads; consumed by the
replica 55, the Desk 62, Fleet 98, W4's handover 123 and W5's webhooks 156). Depends on tasks 11
and 12 (the last W1 writers before import) and Hub task H26 (the change feed, route 43,
hub-contract §7.11). Hub anchors were read in the `feat/hub` worktree at `9f2fa31`: the contract
(cited `hub-contract:N`), Hub task 14's part (`H14:N`) and Hub task 26's part
`docs/superpowers/plans/2026-09-23-phase-h-hub-parts/task-26.md` (cited `H26:N`). `server/` and
`store/` are **planned, not on `feat/hub`** at `9f2fa31`; `server/changes.go` and
`store/changes.go` are Hub task 26's files and are **UNVERIFIED until merge**.

This task does two things that belong together because both are about who hears what.

**First, it freezes the Work kinds.** Tasks 08 to 12 emitted their events under draft names. This
task moves every name into one file, `hubapi/workevents.go`, with **one Go struct per kind** for
its `data`, so the Hub's writers and the desktop's types (task 37) read one list. A reflection test
refuses any `data` field that is not a string, an `int64` or a `bool` (hub-contract:1166; `Event.Data`
holds "strings, int64s and bools only", `H14:72`), and a second test fails when the Hub writes a
Work-looking kind that the file does not declare. Drift between a writer and the desktop then fails
a test instead of silently dropping an event (consumers ignore unknown kinds, hub-contract:1135).

**Second, it adds subscriptions** (L-117, pulled into W1 by 05 §5.3 row 4): who is following an
issue. The rules are few and strict. The creator on create, the assignee on assign, each mentioned
user on mention and the commenter on comment are subscribed **in the same transaction** as the
write that caused it. A person who said `subscribed: false` stays unsubscribed: no later trigger
turns it back on. A person who leaves the team keeps the row and stops seeing it, because every
read is filtered by what the caller can read now. There is **no delivery** here: no mail, no
toast, no badge. The Desk digest is W2's (tasks 81 onward), and AGENTIC-UX §10's anti-pattern 8
(*"No more than 1 notification per user per 15 minutes, and it is a digest"*,
`docs/AGENTIC-UX.md:243`) is why.

**The one per-caller rule.** The feed already shows an event only to readers of its project or
members of its team (hub-contract:1103-1104; `H26:26-28`). A subscription is private: that Grace
muted EP-142 is nobody else's business. So `subscription.set` is shown **only to its subject**,
the user it is about. The rule is declared on the kind (`Visibility: SubjectOnly`) and applied in
the feed query as one additive predicate; W2 marks its draft kinds (`df-`) the same way. Nothing
else in the feed changes.

**The one thing this task must not get wrong** is that no text reaches the feed. A title, a
description, a comment body, an import's URL or author name: none of them may appear in any event
`data`, because the feed is the audit log filtered (`H26:7-9`) and the audit log is hash-chained, so
text written there can never be erased (task 14). The canary test writes one marked string into
every W1 text field through every W1 route and then reads every page of `/v1/changes` as every
principal.

Three decisions the scope left open, made here.

1. **Team-scoped kinds carry `teamId` and a `null` `projectId`.** The scope asks for non-null
   `teamId` and `projectId` on every Work kind, but `label.created`, `label.archived` and
   `team.work_policy_set` are about a team, not a project, exactly as the Hub's own `team.*`
   kinds are (hub-contract:1139-1144, `projectId` —). Giving them a project would be inventing
   one. So each kind declares its scope (`ScopeProject` or `ScopeTeam`); the test asserts
   `teamId` non-null on every Work event and `projectId` non-null on every `ScopeProject` kind.
   Listed as a correction to the scope in the return.
2. **A trigger never overwrites a row.** The trigger is `INSERT … ON CONFLICT (issue_id,
   user_id) DO NOTHING`: an existing row, subscribed or not, keeps its value **and its first
   cause**. Only `PUT /v1/issues/{issueId}/subscription` changes a row, with cause `manual`.
3. **A trigger that inserts a row appends a `subscription.set` event; one that finds a row appends
   nothing.** One event per changed record (hub-contract:1133). So a create now changes two records
   (the issue and the creator's subscription) and appends two events. Task 08's
   one-event-per-create test is re-read by kind in this task (see *Files*), and task 12's
   comment count already counts `comment.*` kinds only.

**Files:**

- `control-plane/internal/hub/hubapi/workevents.go` (new): the kind constants, one `data` struct
  per kind, `Visibility`, `Scope`, the registry `WorkEventKinds`, `EventData`
- `control-plane/internal/hub/hubapi/workevents_test.go` (new): the reflection and registry tests
- `control-plane/internal/hub/hubapi/work.go` ⚑ (task 03's): task 08's three `Event*` constants
  (`task-08.md:196-201`) move from here to `workevents.go`, unchanged in value
- `control-plane/internal/hub/store/work_subscriptions.go`, `server/work_subscriptions.go` (new)
- `control-plane/internal/hub/server/work_emit.go` (new): `emitWork`, the one way a Work handler
  appends an event; it refuses a kind or a `data` type the registry does not pair
- `control-plane/internal/hub/server/work_feed_integration_test.go` (new, `-tags integration`)
- `control-plane/internal/hub/store/changes.go` ⚑ (Hub task 26's file, `H26:13`): one additive
  predicate in the visible-events query. The scope names `server/changes.go`; Hub task 26 puts the
  query in `store/changes.go` and the route in `server/changes.go`, so the predicate goes where the
  query is, and `server/changes.go` passes the caller's user id if it does not already
- `control-plane/internal/hub/server/work_{issues,revisions,states,relations,labels,policies,comments}.go`
  ⚑ (tasks 08-12, this plan's own files, in task order): every `tx.Emit` becomes `emitWork`; the
  create, the assign and the comment and mention paths call the subscription trigger
- `control-plane/internal/hub/server/work_issues_integration_test.go` ⚑ (task 08's): its
  one-event-per-write count filters to `issue.*` and `project.*` kinds, since a create now also
  appends the creator's `subscription.set`

**Interfaces:**

- Produces:
  - **SW.5**: `hubapi.WorkEventKinds` and every `hubapi.Event*` constant below, with its `data`
    struct; `hubapi.SubjectOnlyKinds() []string`; `hubapi.EventData(v any) map[string]any`
  - the routes, in 07's list and 07's matrix rows: `PUT /v1/issues/{issueId}/subscription`
    (`SubscribeRequest` → `Subscription`; ✓ for `V M A`, 404 for `O N X`) and `GET
    /v1/me/subscriptions` (`Page[Subscription]`, any caller, their own rows only)
  - `store.SubscribeOnTrigger(ctx, tx, org, issueID, userID, cause, at) (inserted bool, err
    error)`, called by tasks 08 and 12, and by any later writer that has a trigger of its own
  - `server.emitWork(tx, kind, teamID, projectID, subject string, data any) error`
- Consumes:
  - task 03: `Subscription`, `SubscribeRequest`, `SubscriptionCause`, `Page[T]`
  - task 06: `issue_subscriptions` (`PRIMARY KEY (issue_id, user_id)`, cause set checked,
    `task-06.md:339-352`)
  - tasks 08-12: their emits, their create, assign, comment and mention paths; task 08's
    `readableIssue` and `LockIssue`
  - Hub task 14: `(*Tx).Emit`, `Event{Kind, TeamID, ProjectID, Subject, Data}` (`H14:67-75`)
  - Hub task 26: the visible-events query and `hubapi.Event` (`H26:19-37`)
- Consumed by: task 15 (its `issue.created` per imported issue, and its row in the canary test),
  task 16 (the relay passes the feed through unchanged), task 37 (the desktop's event types are
  generated from, or tested against, this registry), tasks 55 (the replica applies these kinds),
  62 (the Desk), 98 (Fleet), 123 (W4's handover), 156 (W5's webhooks: SW.22 is built on it)

#### `hubapi/workevents.go` (complete)

```go
package hubapi

// SW.5: the Work kinds of the change feed. Every Work write appends events from
// this list only (server.emitWork refuses anything else), and every data struct
// holds only string, int64 and bool fields (hub-contract §8; Event.Data, H14:72).
// A new kind is an added line here and an added struct; a rename is a break.

// Event kinds, W1.
const (
	EventIssueCreated      = "issue.created"
	EventIssueUpdated      = "issue.updated"
	EventIssueRevised      = "issue.revised"
	EventIssueState        = "issue.state"
	EventIssueRelated      = "issue.related"
	EventIssueUnrelated    = "issue.unrelated"
	EventIssueAttached     = "issue.attached"
	EventIssueErased       = "issue.erased"
	EventProjectKeySet     = "project.key_set"
	EventLabelCreated      = "label.created"
	EventLabelArchived     = "label.archived"
	EventTeamWorkPolicySet = "team.work_policy_set"
	EventCommentCreated    = "comment.created"
	EventCommentMentioned  = "comment.mentioned"
	EventCommentReacted    = "comment.reacted"
	EventCommentResolved   = "comment.resolved"
	EventCommentErased     = "comment.erased"
	EventSubscriptionSet   = "subscription.set"
)

type IssueCreatedData struct {
	Number int64  `json:"number"`
	Key    string `json:"key"`    // the revision key issue:is-…@1, never the alias
	Digest string `json:"digest"`
	State  string `json:"state"`
	Source string `json:"source"`
}
type IssueUpdatedData struct {
	Fields string `json:"fields"` // comma-joined wire names: data carries no arrays
}
type IssueRevisedData struct {
	Key          string `json:"key"`
	Revision     int64  `json:"revision"`
	Digest       string `json:"digest"`
	ServesCount  int64  `json:"servesCount"`
	RestoredFrom int64  `json:"restoredFrom"` // 0 when not a restore
}
type IssueStateData struct {
	From       string `json:"from"`
	To         string `json:"to"`
	Cause      string `json:"cause"`
	Proof      string `json:"proof"`      // "" before landed (SW.4's result words)
	LandCommit string `json:"landCommit"` // "" unless the move carried one
}
type IssueRelationData struct { // issue.related and issue.unrelated
	Kind            string `json:"kind"` // the stored kind: blocks | duplicates | relates
	OtherIssueID    string `json:"otherIssueId"`
	OtherAssigneeID string `json:"otherAssigneeId"` // "" when none or the writer
}
type IssueAttachedData struct {
	Digest string `json:"digest"`
}
type IssueErasedData struct {
	ErasedRevisions int64 `json:"erasedRevisions"`
	ErasedComments  int64 `json:"erasedComments"`
}
type ProjectKeySetData struct {
	Key         string `json:"key"`
	PreviousKey string `json:"previousKey"`
}
type LabelData struct { // label.created and label.archived
	Name string `json:"name"`
}
type TeamWorkPolicySetData struct {
	Fields string `json:"fields"`
}
type CommentCreatedData struct {
	IssueID    string `json:"issueId"`
	ParentID   string `json:"parentId"`
	Kind       string `json:"kind"`
	Author     string `json:"author"`
	AnchorKind string `json:"anchorKind"`
}
type CommentMentionedData struct {
	CommentID string `json:"commentId"`
	IssueID   string `json:"issueId"`
}
type CommentReactedData struct {
	Emoji string `json:"emoji"`
	On    bool   `json:"on"`
}
type CommentResolvedData struct {
	Resolved bool `json:"resolved"`
}
type CommentErasedData struct{}
type SubscriptionSetData struct {
	IssueID    string `json:"issueId"`
	Subscribed bool   `json:"subscribed"`
	Cause      string `json:"cause"`
}
```

`IssueErasedData.ErasedComments` is one field the scope's list (`{erasedRevisions}`) does not
name. It is added here because task 14's erase answers it and the Desk and the replica need the
count to say what went; like `brief.erased` (hub-contract's §8 row), the event holds counts only.
`issue.revised` and `issue.state` carry task 09's and task 10's names as their scopes spell them;
where those parts spell a field differently, the part that emits it wins and this file follows.

#### `hubapi/workevents.go`, the registry (complete; the same file, continued)

```go
// Visibility is who may see an event of a kind in /v1/changes.
type Visibility int

const (
	// VisibleToReaders is hub-contract §7.11's rule: readers of the event's
	// project, or members of its team.
	VisibleToReaders Visibility = iota
	// SubjectOnly adds: and only the user the event is about (Subject).
	SubjectOnly
)

// Scope is what an event of a kind is about, which fixes its non-null ids.
type Scope int

const (
	ScopeProject Scope = iota // teamId and projectId both set
	ScopeTeam                 // teamId set, projectId null (as the Hub's team.* kinds)
)

// WorkEventKind pairs a kind with its data type, scope and visibility.
type WorkEventKind struct {
	Kind       string
	Data       any // a zero value of the kind's data struct
	Scope      Scope
	Visibility Visibility
}

// WorkEventKinds is SW.5, W1. W2-W5 append their kinds (tasks 72, 99, 114, 143).
var WorkEventKinds = []WorkEventKind{
	{EventIssueCreated, IssueCreatedData{}, ScopeProject, VisibleToReaders},
	{EventIssueUpdated, IssueUpdatedData{}, ScopeProject, VisibleToReaders},
	{EventIssueRevised, IssueRevisedData{}, ScopeProject, VisibleToReaders},
	{EventIssueState, IssueStateData{}, ScopeProject, VisibleToReaders},
	{EventIssueRelated, IssueRelationData{}, ScopeProject, VisibleToReaders},
	{EventIssueUnrelated, IssueRelationData{}, ScopeProject, VisibleToReaders},
	{EventIssueAttached, IssueAttachedData{}, ScopeProject, VisibleToReaders},
	{EventIssueErased, IssueErasedData{}, ScopeProject, VisibleToReaders},
	{EventProjectKeySet, ProjectKeySetData{}, ScopeProject, VisibleToReaders},
	{EventLabelCreated, LabelData{}, ScopeTeam, VisibleToReaders},
	{EventLabelArchived, LabelData{}, ScopeTeam, VisibleToReaders},
	{EventTeamWorkPolicySet, TeamWorkPolicySetData{}, ScopeTeam, VisibleToReaders},
	{EventCommentCreated, CommentCreatedData{}, ScopeProject, VisibleToReaders},
	{EventCommentMentioned, CommentMentionedData{}, ScopeProject, VisibleToReaders},
	{EventCommentReacted, CommentReactedData{}, ScopeProject, VisibleToReaders},
	{EventCommentResolved, CommentResolvedData{}, ScopeProject, VisibleToReaders},
	{EventCommentErased, CommentErasedData{}, ScopeProject, VisibleToReaders},
	{EventSubscriptionSet, SubscriptionSetData{}, ScopeProject, SubjectOnly},
}

// LookupWorkEvent returns the registry entry for kind.
func LookupWorkEvent(kind string) (WorkEventKind, bool) {
	for _, k := range WorkEventKinds {
		if k.Kind == kind {
			return k, true
		}
	}
	return WorkEventKind{}, false
}

// SubjectOnlyKinds is the list the feed query filters on (store/changes.go).
func SubjectOnlyKinds() []string {
	var out []string
	for _, k := range WorkEventKinds {
		if k.Visibility == SubjectOnly {
			out = append(out, k.Kind)
		}
	}
	return out
}

// EventData turns a data struct into Tx.Emit's map by its JSON tags, so the
// stored keys are the struct's and nothing else. It panics on a field that is
// not string, int64 or bool: TestEveryWorkDataFieldIsAScalar makes that
// unreachable for the registry's types.
func EventData(v any) map[string]any {
	rv := reflect.ValueOf(v)
	rt := rv.Type()
	out := make(map[string]any, rt.NumField())
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		switch f.Type.Kind() {
		case reflect.String:
			out[name] = rv.Field(i).String()
		case reflect.Int64:
			out[name] = rv.Field(i).Int()
		case reflect.Bool:
			out[name] = rv.Field(i).Bool()
		default:
			panic("hubapi: event data field " + rt.Name() + "." + f.Name + " is not string, int64 or bool")
		}
	}
	return out
}
```

`workevents.go` imports `reflect` and `strings`. `hubapi` already uses `reflect` outside its
tests (`lists.go` and `required.go` import it, verified at `9f2fa31`), so this adds no new kind of
dependency to the wire package.

