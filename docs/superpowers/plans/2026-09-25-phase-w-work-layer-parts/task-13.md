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

#### `server/work_emit.go` (complete)

```go
package server

// emitWork is the only way a Work handler appends an event. It checks the kind
// is in SW.5's registry, that data is exactly the kind's data type, and that the
// ids the kind's scope needs are present, then hands Tx.Emit the map. A mismatch
// is a bug in the handler, so it is a 500 internal, never a stored event.
func emitWork(tx *Tx, kind, teamID, projectID, subject string, data any) error {
	k, ok := hubapi.LookupWorkEvent(kind)
	if !ok || reflect.TypeOf(data) != reflect.TypeOf(k.Data) {
		return fmt.Errorf("work event %q: undeclared kind or wrong data type %T", kind, data)
	}
	if teamID == "" || subject == "" || (k.Scope == hubapi.ScopeProject && projectID == "") {
		return fmt.Errorf("work event %q: missing team, project or subject", kind)
	}
	if k.Scope == hubapi.ScopeTeam {
		projectID = ""
	}
	tx.Emit(Event{Kind: kind, TeamID: teamID, ProjectID: projectID, Subject: subject,
		Data: hubapi.EventData(data)})
	return nil
}
```

A plain `error` returned from `Write`'s `fn` rolls the transaction back, so a mis-declared event
never reaches the audit chain. That it answers 500 `internal` (hub-contract §4.2: *"a bug; the
detail is in the log"*) is what Hub task 14's `Write` must do with a non-`*APIError`; its part
states the `*APIError` path (`H14:109`) and the commit-failure path (`H14:115`) but not this one,
so it is **UNVERIFIED** and listed as a gap for the Hub owner.

#### The store (`store/work_subscriptions.go`, signatures complete)

```go
package store

// SubscribeOnTrigger is the trigger: INSERT … ON CONFLICT (issue_id, user_id) DO
// NOTHING. inserted is false when a row existed, subscribed or not, which is
// what keeps an explicit "subscribed: false" and the first cause.
//   INSERT INTO issue_subscriptions (org_id, issue_id, user_id, subscribed, cause, updated_at)
//   VALUES ($1, $2, $3, true, $4, $5) ON CONFLICT (issue_id, user_id) DO NOTHING
func SubscribeOnTrigger(ctx context.Context, tx pgx.Tx, org, issueID, userID string, cause hubapi.SubscriptionCause, at time.Time) (inserted bool, err error)

// SetSubscription is the manual choice: an upsert with cause 'manual'.
// changed is false when the row already held that value (the cause is not
// rewritten then, so a no-op PUT leaves the first cause in place).
func SetSubscription(ctx context.Context, tx pgx.Tx, org, issueID, userID string, subscribed bool, at time.Time) (s hubapi.Subscription, changed bool, err error)

// MySubscriptions lists the caller's rows on issues the caller can read now
// (the join to issues runs under row-level security, and a membership join
// drops a team the caller has left), newest change first, keyset on
// (updated_at, issue_id). An erased issue's row is listed: the key still resolves.
func MySubscriptions(ctx context.Context, q Querier, userID string, pr hubapi.PageRequest) (hubapi.Page[hubapi.Subscription], error)
```

**The trigger points** (each in the triggering write's transaction, after the write and before its
own event, so the events read in cause order):

| Write | Who is subscribed | Cause | Where |
|---|---|---|---|
| `POST /v1/projects/{projectId}/issues` | the creator | `create` | task 08's create |
| `PATCH /v1/issues/{issueId}` setting `assigneeId` to a user | that user | `assign` | task 08's patch |
| `POST /v1/issues/{issueId}/comments` | the commenter | `comment` | task 12's create |
| the same, per `mentions` entry | each mentioned user | `mention` | task 12's create |

An import (task 15) subscribes **nobody**: the person who ran it moved a backlog, they did not
write 500 issues, and subscribing them to every one would make W2's digest a flood.

Each `inserted == true` appends one `subscription.set` `{issueId, subscribed: true, cause}` with
`Subject` the subscribed user. An assign to nobody (clearing `assigneeId`) subscribes nobody and
unsubscribes nobody.

#### The feed predicate (`store/changes.go` ⚑, Hub task 26's query, one added clause)

```sql
-- Hub task 26's visible-events query, H26:25-28, with one clause appended by Phase W task 13.
-- $4 is the caller's user id; $5 is hubapi.SubjectOnlyKinds() as a text[].
SELECT …
  FROM audit_events e
 WHERE e.org_id = $1 AND e.seq > $2 AND e.seq <= $3 AND e.feed
   AND ( <task 26's visibility: e.team_id in the caller's teams, or e.project_id in their projects> )
   AND ( e.kind <> ALL ($5::text[]) OR e.subject = $4 )          -- Phase W: SubjectOnly kinds
 ORDER BY e.seq
 LIMIT $6
```

The clause only removes rows, so every Hub test of task 26 still passes; `next` still advances to
`H` when a caller sees nothing (`H26:32-34`), so a filtered subscription event never stalls a
cursor. The column names follow Hub task 07's `audit_events` table and are **UNVERIFIED** until
that table lands; the clause is re-written against it, not the other way round.

#### The routes (`server/work_subscriptions.go`) and their wire

```go
func (s *Server) registerWorkSubscriptions(mux *http.ServeMux) {
	mux.HandleFunc("PUT /v1/issues/{issueId}/subscription", s.putSubscription)
	mux.HandleFunc("GET /v1/me/subscriptions", s.mySubscriptions)
}
```

```jsonc
// PUT /v1/issues/is-0a1b…/subscription           viewer, member or admin of the issue's team
{ "subscribed": false }
// → 200 Subscription (the caller's own row; nobody sets another person's)
{ "issueId": "is-0a1b…", "user": { "id": "u-…", "displayName": "Grace", "email": "…" },
  "cause": "manual", "subscribed": false }
// Issue outside the caller's readable set → 404 issue_not_found. An unchanged PUT → 200, no event.

// GET /v1/me/subscriptions?after=&limit=           any caller; their rows on issues they can read now
{ "items": [ { "issueId": "is-0a1b…", "user": { … }, "cause": "manual", "subscribed": false } ],
  "next": null }
```

A viewer may subscribe: following an issue is reading, not writing it (07's row: ✓ for `V M A`).
There is no route to subscribe someone else; the triggers are the only way a row is written for a
person other than the caller, and each is caused by a write that person's team can see.

#### Tests to write first, part 1 — `hubapi/workevents_test.go` (pure)

```go
package hubapi

import (
	"reflect"
	"testing"
)

func TestEveryWorkDataFieldIsAScalar(t *testing.T) {
	for _, k := range WorkEventKinds {
		rt := reflect.TypeOf(k.Data)
		if rt.Kind() != reflect.Struct {
			t.Errorf("%s: data %s is not a struct", k.Kind, rt)
			continue
		}
		for i := 0; i < rt.NumField(); i++ {
			f := rt.Field(i)
			switch f.Type.Kind() {
			case reflect.String, reflect.Int64, reflect.Bool:
			default:
				t.Errorf("%s: %s.%s is %s; data holds only string, int64 and bool (hub-contract §8)",
					k.Kind, rt.Name(), f.Name, f.Type)
			}
			if tag := f.Tag.Get("json"); tag == "" || tag == "-" {
				t.Errorf("%s: %s.%s has no json name", k.Kind, rt.Name(), f.Name)
			}
		}
	}
}

func TestWorkKindsAreUniqueAndNamed(t *testing.T) {
	seen := map[string]bool{}
	for _, k := range WorkEventKinds {
		if seen[k.Kind] {
			t.Errorf("kind %q declared twice", k.Kind)
		}
		seen[k.Kind] = true
	}
	if len(WorkEventKinds) != 18 {
		t.Errorf("W1 declares 18 Work kinds, the registry has %d", len(WorkEventKinds))
	}
}

func TestOnlySubscriptionSetIsSubjectOnlyInW1(t *testing.T) {
	if got := SubjectOnlyKinds(); !reflect.DeepEqual(got, []string{EventSubscriptionSet}) {
		t.Fatalf("SubjectOnlyKinds() = %v", got)
	}
}

func TestEventDataUsesTheJSONNames(t *testing.T) {
	got := EventData(CommentReactedData{Emoji: "👍", On: true})
	want := map[string]any{"emoji": "👍", "on": true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("EventData = %#v, want %#v", got, want)
	}
	if got := EventData(IssueRevisedData{Revision: 4}); got["revision"] != int64(4) {
		t.Fatalf("revision = %#v, want int64(4)", got["revision"])
	}
}
```

| Test | Mutant it kills |
|---|---|
| `TestEveryWorkDataFieldIsAScalar` | an array, a map, a pointer or a float in `data` (a `[]string` for `fields`, a `float64` count), which the canonical encoding cannot carry (hub-contract:1166) |
| `TestWorkKindsAreUniqueAndNamed` | a kind declared twice with two data types; a kind dropped from the registry (the count is W1's 18; W2 raises it) |
| `TestOnlySubscriptionSetIsSubjectOnlyInW1` | a private kind shown to the whole team, or a public kind hidden from it |
| `TestEventDataUsesTheJSONNames` | stored keys in Go field names (`OtherIssueID`) instead of the wire names the desktop reads; an `int` stored where `Tx.Emit` expects `int64` |

#### Tests to write first, part 2 — `server/work_feed_integration_test.go` (`//go:build integration`)

The cast and harness are task 11's. `fullRun(t)` drives **every W1 write route that exists at this
task** once, as Grace or Ada, with the canary `CANARY-13` embedded in every text field: an issue's
title and description, a later revision, a comment body and a hunk ref. A label name carries no
canary, because it is team metadata and reaches the feed by design (task 11). Task 15 appends its import to `fullRun`
(an import title, description, URL and author name, each carrying the canary). `feed(as, after)`
follows `next` until `more: false` and returns every event.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestNoWorkTextReachesTheFeed` | after `fullRun`, every page of `/v1/changes` as Ada, Grace, Vera and Linus, and every `data` value in `hub.audit_events` read as the owner, contains no `CANARY-13` | content in the feed or the audit chain (hub-contract:1133-1134) |
| `TestEveryWorkEventCarriesItsScope` | every event of `fullRun` whose kind is in `WorkEventKinds` has a non-null `teamId`, and a non-null `projectId` when its scope is `ScopeProject`; every `ScopeTeam` kind has `projectId: null` | an event nobody sees (no team) or everybody sees |
| `TestTheHubWritesOnlyDeclaredWorkKinds` | every kind in `audit_events` after `fullRun` with a Work prefix (`issue.`, `comment.`, `label.`, `team.work_`, `project.key_`, `subscription.`) is in `WorkEventKinds`, and its stored `data` keys equal its struct's JSON names | a writer's literal drifting from SW.5, which the desktop would silently ignore |
| `TestASubscriptionIsSeenOnlyByItsSubject` | Grace unsubscribes from EP-1 and comments on it; Ada's feed shows Grace's `comment.created` and not Grace's `subscription.set`; Grace's shows both | a private kind in the team feed |
| `TestAFilteredEventStillMovesTheCursor` | only Grace's `subscription.set` is written after Ada's cursor; Ada's poll → no events and `next` equal to the head | a per-caller filter that stalls a cursor on invisible rows |
| `TestAnUnsubscribeSurvivesAMention` | Vera `PUT subscribed: false` on EP-1; Grace mentions Vera on EP-1; Vera's `GET /v1/me/subscriptions` still reads EP-1 `subscribed: false`, cause `manual`; no `subscription.set` for Vera from the mention | a trigger overriding the person's choice |
| `TestTheCreatorIsSubscribedOnCreate` | Grace creates EP-5 → her row `subscribed: true, cause: create`; the events are `issue.created` then `subscription.set` | a creator who never hears about their own issue |
| `TestAssigningSubscribesInTheSameTransaction` | Ada creates EP-7 and assigns it to Grace (a member; a viewer is not assignable, task 08's `Assignable`, `task-08.md:300-301`): `issue.updated` and Grace's `subscription.set` have consecutive `seq` and the same `at`; on EP-8, a `PATCH {assigneeId: grace, labels: [billingLabel]}` refused 422 by the label check leaves Grace with no row on EP-8 | a subscription written after commit, which a crash between the two would lose |
| `TestACommenterAndTheMentionedAreSubscribed` | Grace comments on EP-6 mentioning Ada → Grace `cause: comment`, Ada `cause: mention`; a second comment by Grace appends no second `subscription.set` | a trigger that fires on every write, or none |
| `TestALeaverKeepsTheRowAndStopsSeeingIt` | Vera subscribed to EP-1 is removed from Portal; her `GET /v1/me/subscriptions` omits EP-1; the owner's view of `issue_subscriptions` still has the row; re-added, she sees it again | a row deleted on leaving (her choice lost), or a row that still leaks the issue |
| `TestSubscriptionRoutesFollowTheMatrix` | Vera `PUT` → 200; Linus `PUT` on EP-1 → 404 `issue_not_found`; Xavier `GET /v1/me/subscriptions` → 200 and an empty page | a viewer refused a read-like action, or a non-member probing ids |
| `TestAnUnchangedPutAppendsNoEvent` | `subscribed: false` twice → one `subscription.set` | an event per `PUT` whatever changed |

#### Steps

- [ ] **Step 1:** Confirm the sync base (rule H1): `feat/hub` at or after Hub task 26, sync-merged;
  tasks 08-12 committed. Re-read hub-contract §7.11 and §8, `H26` and `H14:67-75` at the new base.
- [ ] **Step 2:** Write `hubapi/workevents_test.go`; it fails to compile. Write `workevents.go`;
  move task 08's three constants into it; `go build ./control-plane/...` stays green.
- [ ] **Step 3:** Write `server/work_emit.go`; replace every `tx.Emit` in tasks 08-12's Work
  server files with `emitWork` and the kind's struct. `TestTheHubWritesOnlyDeclaredWorkKinds`
  (once written) is the check that none was missed.
- [ ] **Step 4:** Write `server/work_feed_integration_test.go`; the subscription rows fail (no
  routes, no triggers); `TestNoWorkTextReachesTheFeed` should already pass, which is correct:
  tasks 08-12 wrote no text into `data`, and this test is the lock on that.
- [ ] **Step 5:** Write `store/work_subscriptions.go`, `server/work_subscriptions.go` and the
  trigger calls; add the clause to `store/changes.go`; run until green.
- [ ] **Step 6:** Re-read task 08's event-count test by kind (see *Files*); rerun tasks 08-12's
  integration tests and Hub task 26's `changes_integration_test.go`, which must stay green.
- [ ] **Step 7:** Run task 05's subscription conformance rows against the fake; fix the fake.
- [ ] **Step 8:** `git diff -- go.work control-plane/go.mod` shows nothing; commit by pathspec.

#### Acceptance

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/work
go test ./control-plane/internal/hub/hubapi/ -race -count=1 -v -run '^Test(EveryWorkDataField|WorkKindsAreUnique|OnlySubscriptionSetIs|EventDataUses)' 2>&1 | grep -c "^--- PASS"   # 4
go test -tags integration ./control-plane/internal/hub/server/ -race -count=1 -v \
  -run '^Test(NoWorkTextReaches|EveryWorkEventCarries|TheHubWritesOnlyDeclared|ASubscriptionIsSeenOnly|AFilteredEventStill|AnUnsubscribeSurvives|TheCreatorIsSubscribed|AssigningSubscribes|ACommenterAndTheMentioned|ALeaverKeepsTheRow|SubscriptionRoutesFollow|AnUnchangedPutAppends)' \
  2>&1 | grep -c "^--- PASS"                                                                    # 12
go test -tags integration ./control-plane/internal/hub/server/ -race -count=1 -run 'Changes|Latest|OnlyVisible|Paging|Gapless'   # ok
go test ./control-plane/... -race -count=1                                                      # ok
```

#### What this task does not claim

- Any delivery: no mail, digest, toast or badge (W2, task 81 onward; AGENTIC-UX §10 anti-pattern 8).
- The replica that applies these kinds (task 55), the Desk (62), Fleet (98) or webhooks (156).
- W2-W5 kinds (tasks 72, 99, 114, 143 append them), or the draft kinds' subject-only rule until W2
  declares them.
- That the feed is fast: the added clause is one `<> ALL` over a list of one; nothing is measured.

**Parity rows served:** **L-117** (automatic and manual subscriptions; closed with task 52's
control), **L-229** (the delta-sync log gains the Work kinds; closed with the replica, task 55),
and the metadata-only base of **L-251** (W5's webhooks).

