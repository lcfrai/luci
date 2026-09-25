### Task 05: the fake, part 2, and the Work conformance rows: relations, labels, policies, comments, subscriptions, imports, search, erase (**WS0**)

**Slice:** W1. **Size:** 3 focused hours (an estimate, not a measurement). **Stream:** WH.
**dependsOn:** 04. **Seams:** defines **SW.2**, second half, and reaches start line **WS0**: when
this task is green, streams WP, WD1 and WD2 build every Work-backed handler and surface against
`hubclient.Work` and `NewFake()`. After it, a change to SW.1 or SW.2 that is not an added method,
field or route reopens this task and is announced (skeleton, *Start lines*).

Task 04 fixed the method set and part 1's rules. This task gives the other nineteen methods their
rules, deletes the stub, and writes the **rows**: one named assertion per rule, run against the
fake here and against the real Hub in task 16. The rows are the promise WS0 makes: a handler that
passes on the fake passes on the Hub, because a rule the two disagree on fails a named row.

**Anchors, re-read on 2026-09-25 at `feat/hub` `9f2fa31`**, beyond task 04's: the conformance
design, `phase-h-hub-parts/task-05.md:21-73` (`Env{Admin, Member, Viewer, Outsider, TeamID,
Advance, ReservationTTL}`, `Group`, `Run`, rows named `group/row`), `:75-121` (the fake's `Env`
builder), `:209-218`; `hubapi/types.go:349-354` (`BlobContentTypes`), `:565-567`
(`EraseRequest`); `hubclient/fake.go:37-40` (`FakeUser.OrgAdmin`). The Phase W parts whose server
rules these are, read on `feat/work-layer` at `9c4a600`: task 11 (`task-11.md:140-200`,
`:340-372`, tests `:457-494`), task 12 (`:135-146`, tests `:462-492`), task 13 (`:122-139`, tests
`:524-533`), task 14 (tests `:346-357`), task 15 (tests `:646-664`), task 09 (`:306`,
attachments).

**Not yet on any branch, so UNVERIFIED until merge:** Hub task H05's `conformance.go` and its
blob store (`fake_designs.go`); tasks 11-15's `hubapi` additions. Where this part names one, it
says so.

Eight decisions the scope left open, made here.

**(a) The rows run through `RunWork`, beside `Run`, not inside it.** H05's `conformance.go` is a
Hub file; adding a group to its `AllGroups` would change its `TestRunNamesEveryRow` count and put a
Phase W edit in a Hub file. So `conformance/work.go` declares `GroupWork = "work"`, the rows, and
`RunWork(t, newEnv)`, which runs each row as `t.Run("work/<row>")` exactly as `Run` names its own.
One suite in shape, no Hub file touched. Task 16's `work_integration_test.go` calls `RunWork` with
the real Hub.

**(b) `WorkEnv` is `Env` plus one principal.** Erase is an org admin's (task 14:
`TestOnlyAnOrgAdminErases`), and `Env` has no org admin. `WorkEnv{Env; OrgAdmin hubclient.Hub}`;
a `nil` `OrgAdmin` makes the erase rows `t.Skip` and say so, as `Advance: nil` does. A second team
comes from `Outsider`: the outsider creates one, so no other new principal is needed.

**(c) Later tasks' names are the fake's own until those tasks land** (task 04's decisions (e)-(g),
the same way). Task 11's `blocked_by` and `duplicated_by`, `LabelColors`, `TriageExitRefused` and
its codes; task 12's codes and emoji rule; task 13's kinds; task 14's `org_admin`; task 15's import
bounds. Each is an unexported `fk…` constant or function with the later task's exact spelling, and
the rows assert the **strings**, so drift fails a row on either side. Tasks 11-15 each list
`fake_work*.go` ⚑ (tasks 11 and 12 already do, `task-11.md:64`, `task-12.md:56`) and swap the
fake's copy for `hubapi`'s.

**(d) The other end of a relation reads the inverse word** (task 11), stored once. `relates` is
its own inverse and stored once from either end; a repeat is 200 with the same head and no event.

**(e) Text anchors count UTF-16 code units of the named revision's description** (task 12,
`task-12.md:135`), with `unicode/utf16`; task 12's `hubapi.UTF16Len` replaces it.

**(f) Subscriptions follow task 13.** Create subscribes the creator, assign the assignee, a comment
its author and each mentioned member, each once; a row the person set (`manual`) is never changed
by a trigger; an unchanged `Subscribe` appends no event; an import subscribes nobody.

**(g) Erase follows task 14,** which is more exact than the scope: texts read `""` with `erasedAt`
set (the scope's *"resolves `erased`"* is `/v1/refs`'s word, task 09's resolver); ids, keys,
digests, numbers and the alias stay; a second erase answers zeros and still appends its event; an
erased issue refuses every write with 409 `conflict`; search skips it.

**(h) Import follows task 15:** 1-500 items, validated all before any is written, each in
`triage` whatever the team's default, `not_tied`/`imported`, `source: intake`, both texts
`author: H`, labels matched to the team's live labels by name case-insensitively (never created),
idempotent per `(project, source, externalId)`. `ExternalRef` has no `createdAt` until task 15 adds
it (additive), so the fake keeps none.

**Files** (under `control-plane/internal/hub/hubclient/`):

| Path | Change |
|---|---|
| `fake_work_social.go` | new: relations, labels, the team policy, comments, reactions, resolve, subscriptions; five of task 04's hooks |
| `fake_work_intake.go` | new: import, search, attachments, erase; the sixth hook, `erasedWrite` |
| `fake_work_social_test.go` | new: the fake-only tests, each with its mutant |
| `fake_work_stub.go` | **deleted** |
| `conformance/work.go` | new: `GroupWork`, `WorkEnv`, `RunWork`, the `W-NN` rows |
| `conformance/work_fake_test.go` | new: `RunWork` against `NewFake()` |

No other file changes: task 04's `fake_work.go` calls the six hooks by name, and they move from
the stub to these files unchanged in signature.

---

#### Interfaces, complete: what `conformance/work.go` exports

```go
// control-plane/internal/hub/hubclient/conformance/work.go (the head; the rows follow)
package conformance

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubclient"
)

// GroupWork is Phase W's group. Its rows run through RunWork, not Run, so the
// Hub's AllGroups and its row count are untouched (task 05's decision (a)).
const GroupWork Group = "work"

// WorkEnv is one fresh world per row: Env's team T and four principals, and
// an org admin, who is in no team. OrgAdmin nil skips the erase rows.
type WorkEnv struct {
	Env
	OrgAdmin hubclient.Hub
}

// WorkRow is one named assertion, run against the fake (task 05) and against
// the real Hub through the HTTP client (task 16). W2-W5 append their own
// slices (tasks 73, 99, 115, 144) and a RunWork call each.
type WorkRow struct {
	ID   string // "W-01" …; never reused, never renumbered
	Name string // "numbers_gapless" …
	Run  func(t *testing.T, e WorkEnv)
}

// RunWork runs every row of rows (WorkRowsW1 when none are given), each as
// t.Run("work/<ID>_<Name>") with a fresh WorkEnv from newEnv.
func RunWork(t *testing.T, newEnv func(t *testing.T) WorkEnv, rows ...WorkRow) {
	t.Helper()
	if len(rows) == 0 {
		rows = WorkRowsW1
	}
	for _, r := range rows {
		t.Run(string(GroupWork)+"/"+r.ID+"_"+r.Name, func(t *testing.T) { r.Run(t, newEnv(t)) })
	}
}

// wctx, not bg: H05's conformance.go may declare its own name for this.
var wctx = context.Background()

// project is a new project of T, made by T's admin; key is "" for none.
func (e WorkEnv) project(t *testing.T, name, key string) hubapi.Project {
	t.Helper()
	p, err := e.Admin.CreateProject(wctx, hubapi.CreateProjectRequest{TeamID: e.TeamID, Name: name,
		Remotes: []hubapi.RemoteRequest{}})
	if err != nil {
		t.Fatal(err)
	}
	if key != "" {
		if _, err := e.Admin.SetProjectKey(wctx, p.ID, key); err != nil {
			t.Fatal(err)
		}
	}
	return p
}

// wChore is the smallest valid create: no criterion, so a reason.
func wChore(title string) hubapi.CreateIssueRequest {
	return hubapi.CreateIssueRequest{Title: wText(title), Description: wText(""), Serves: []hubapi.Serve{},
		Implements: []string{}, Labels: []string{}, NotTiedReason: hubapi.NotTiedChore}
}

func wText(s string) hubapi.AuthoredText { return hubapi.AuthoredText{Text: s, Author: hubapi.AuthorHuman} }

// issue is a new chore in p, created by the member.
func (e WorkEnv) issue(t *testing.T, p hubapi.Project, title string) hubapi.IssueWithRevision {
	t.Helper()
	iw, err := e.Member.CreateIssue(wctx, p.ID, wChore(title))
	if err != nil {
		t.Fatal(err)
	}
	return iw
}

// wRefused asserts err is a Hub refusal with this status and code, and each
// detail named in kv ("key", "value", …).
func wRefused(t *testing.T, err error, status int, code string, kv ...string) {
	t.Helper()
	var he *hubclient.Error
	if !errors.As(err, &he) || he.Status != status || he.Problem.Error != code {
		t.Fatalf("err = %v, want %d %s", err, status, code)
	}
	for i := 0; i+1 < len(kv); i += 2 {
		if he.Problem.Details[kv[i]] != kv[i+1] {
			t.Errorf("details[%q] = %q, want %q", kv[i], he.Problem.Details[kv[i]], kv[i+1])
		}
	}
}

// wKinds is the kinds of every event after seq "after" that start with prefix.
func wKinds(t *testing.T, view hubclient.Hub, after, prefix string) []string {
	t.Helper()
	page, err := view.Changes(wctx, after, 200)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, ev := range page.Events {
		if strings.HasPrefix(ev.Kind, prefix) {
			out = append(out, ev.Kind)
		}
	}
	return out
}

// wHead is the feed's cursor now: events after it are this row's. Every
// unexported name in work.go starts with w, so none meets one of H05's.
func wHead(t *testing.T, view hubclient.Hub) string {
	t.Helper()
	page, err := view.Changes(wctx, "latest", 1)
	if err != nil {
		t.Fatal(err)
	}
	return page.Next
}
```

`wHead` reads `Changes("latest", 1)`, which H05's part fixes as *"no events and the head as
`next`"* (`phase-h-hub-parts/task-05.md:163`), so a row counts only its own events, on the fake and
on a real Hub with history.

---

#### `fake_work_social.go`, complete

Five blocks; the file is their concatenation. Block 1: the names of later tasks, the state, and
four of task 04's hooks at their real rules.

```go
package hubclient

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"sync"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/ids"
)

// Phase W, part 2: relations, labels, the team policy, comments, reactions,
// resolve and subscriptions. The rules are tasks 11-13's; the names below are
// theirs, spelled here until each lands in hubapi (task 05's decision (c)).
const (
	fkRelationBlockedBy    hubapi.RelationKind = "blocked_by"    // task 11
	fkRelationDuplicatedBy hubapi.RelationKind = "duplicated_by" // task 11

	fkPolicyPriorityToLeaveTriage = "priorityToLeaveTriage"

	fkRelationNotFound   = "relation_not_found"    // 404, details.kind, .otherId
	fkRelationCycle      = "relation_cycle"        // 422, details.kind, .otherId
	fkLabelNotFound      = "label_not_found"       // 404, details.labelId
	fkLabelExists        = "label_exists"          // 409, details.labelId
	fkLabelUnavailable   = "label_unavailable"     // 422, details.field "labels[n]"
	fkPolicyRefused      = "policy_refused"        // 422, details.policy, .issueId
	fkCommentNotFound    = "comment_not_found"     // 404, details.commentId
	fkCommentNotTopLevel = "comment_not_top_level" // 422, details.field
	fkAnchorUnresolvable = "anchor_unresolvable"   // 422, details.field

	fkEventIssueRelated      = "issue.related"
	fkEventIssueUnrelated    = "issue.unrelated"
	fkEventLabelCreated      = "label.created"
	fkEventLabelArchived     = "label.archived"
	fkEventTeamWorkPolicySet = "team.work_policy_set"
	fkEventCommentCreated    = "comment.created"
	fkEventCommentMentioned  = "comment.mentioned"
	fkEventCommentReacted    = "comment.reacted"
	fkEventCommentResolved   = "comment.resolved"
	fkEventSubscriptionSet   = "subscription.set"
)

// fkLabelColors is task 11's LabelColors: token names, never the thread's.
var fkLabelColors = []string{"ink-2", "accent", "run", "need", "fail", "drift"}

// fakeSocial is one world's part-2 state, found as task 04's wk() is (its
// decision (i)). Every field is guarded by the world's mu.
type fakeSocial struct {
	relations []*fakeRelation
	labels    map[string]*fakeLabel         // by id
	policies  map[string]*hubapi.TeamPolicy // by team id; absent reads DefaultTeamPolicy
	comments  map[string]*fakeComment       // by id
	subs      map[[2]string]*fakeSub        // by (issue id, user id)
	imports   map[[3]string]string          // (project, source, externalId) → issue id
	attached  map[string][]hubapi.IssueAttachment
	erased    map[string]bool // issue ids
}

type fakeRelation struct {
	kind     hubapi.RelationKind // blocks | duplicates | relates, the writer's direction
	from, to string
}

type fakeLabel struct {
	label      hubapi.Label
	archivedAt string
	seq        int64
}

func (l *fakeLabel) order() int64 { return l.seq }

type fakeComment struct {
	c         hubapi.Comment // Reactions and CreatedBy are made at read
	teamID    string
	projectID string
	createdBy string
	reactions [][2]string // (emoji, user id), in order; one row per pair
	seq       int64
}

func (c *fakeComment) order() int64 { return c.seq }

// seqOr0 is a nil-safe seq: a cursor the list did not issue lists from the start.
func (c *fakeComment) seqOr0() int64 {
	if c == nil {
		return 0
	}
	return c.seq
}

type fakeSub struct {
	cause      hubapi.SubscriptionCause
	subscribed bool
	seq        int64
}

var workSocials sync.Map // *fakeWorld → *fakeSocial

func (w *fakeWorld) soc() *fakeSocial {
	v, _ := workSocials.LoadOrStore(w, &fakeSocial{labels: map[string]*fakeLabel{},
		policies: map[string]*hubapi.TeamPolicy{}, comments: map[string]*fakeComment{},
		subs: map[[2]string]*fakeSub{}, imports: map[[3]string]string{},
		attached: map[string][]hubapi.IssueAttachment{}, erased: map[string]bool{}})
	return v.(*fakeSocial)
}
```

Block 2: task 04's hooks at their rules, and the team policy.

```go
// checkIssueLabels is task 11's: each id a live label of the issue's team, and
// no id twice; else 422 label_unavailable naming labels[i].
func (w *fakeWorld) checkIssueLabels(teamID, field string, labels []string) error {
	for i, id := range labels {
		l := w.soc().labels[id]
		if l == nil || l.label.TeamID != teamID || l.archivedAt != "" || slices.Index(labels, id) != i {
			return problem(422, fkLabelUnavailable, "That label is not one of this team's live labels.",
				"field", field+"["+strconv.Itoa(i)+"]")
		}
	}
	return nil
}

// policyOf is the team's policy, or task 11's DefaultTeamPolicy: backlog,
// every switch off, updatedAt and updatedBy null.
func (w *fakeWorld) policyOf(teamID string) hubapi.TeamPolicy {
	if p := w.soc().policies[teamID]; p != nil {
		return *p
	}
	return hubapi.TeamPolicy{TeamID: teamID, DefaultState: hubapi.StateBacklog}
}

func (w *fakeWorld) defaultState(teamID string) hubapi.IssueState { return w.policyOf(teamID).DefaultState }
func (w *fakeWorld) stopAtLanded(teamID string) bool             { return w.policyOf(teamID).StopAtLandedWithoutServes }

// policyRefuses is task 11's TriageExitRefused: under priorityToLeaveTriage an
// issue with no priority leaves triage only to be declined or marked a duplicate.
func (w *fakeWorld) policyRefuses(is *fakeIssue, to hubapi.IssueState) error {
	if !w.policyOf(is.teamID).PriorityToLeaveTriage || is.head.State != hubapi.StateTriage ||
		to == hubapi.StateTriage || to == hubapi.StateCancelled || to == hubapi.StateDuplicate ||
		is.head.Priority != hubapi.PriorityNone {
		return nil
	}
	return problem(422, fkPolicyRefused, "This team needs a priority before an issue leaves triage.",
		"policy", fkPolicyPriorityToLeaveTriage, "issueId", is.head.ID)
}

// subscribeOn is task 13's trigger: a first row with this cause, one event; a
// row that exists is never changed, so a manual unsubscribe survives.
func (w *fakeWorld) subscribeOn(is *fakeIssue, userID string, cause hubapi.SubscriptionCause) {
	k := [2]string{is.head.ID, userID}
	if w.soc().subs[k] != nil {
		return
	}
	w.soc().subs[k] = &fakeSub{cause: cause, subscribed: true, seq: w.next()}
	w.workEvent(userID, fkEventSubscriptionSet, is.teamID, is.head.ProjectID, userID,
		map[string]any{"issueId": is.head.ID, "subscribed": true, "cause": string(cause)})
}

// TeamPolicy is any member's read (a viewer too); a non-member's is 404.
func (f *Fake) TeamPolicy(_ context.Context, teamID string) (hubapi.TeamPolicy, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("TeamPolicy", teamID)
	if err != nil {
		return hubapi.TeamPolicy{}, err
	}
	if _, _, err := w.teamOf(teamID, me); err != nil {
		return hubapi.TeamPolicy{}, err
	}
	return w.policyOf(teamID), nil
}

// PutTeamPolicy is a team admin's; an unchanged policy is 200 and no event.
func (f *Fake) PutTeamPolicy(_ context.Context, teamID string, req hubapi.PutTeamPolicyRequest) (hubapi.TeamPolicy, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("PutTeamPolicy", teamID, req)
	if err != nil {
		return hubapi.TeamPolicy{}, err
	}
	_, role, err := w.teamOf(teamID, me)
	if err != nil {
		return hubapi.TeamPolicy{}, err
	}
	if err := need(role, hubapi.RoleAdmin); err != nil {
		return hubapi.TeamPolicy{}, err
	}
	if !hubapi.ValidInitialState(req.DefaultState) {
		return hubapi.TeamPolicy{}, badRequest("defaultState", "A new issue starts in triage, backlog or todo.")
	}
	old := w.policyOf(teamID)
	var fields []string
	if req.DefaultState != old.DefaultState {
		fields = append(fields, "defaultState")
	}
	if req.PriorityToLeaveTriage != old.PriorityToLeaveTriage {
		fields = append(fields, "priorityToLeaveTriage")
	}
	if req.StopAtLandedWithoutServes != old.StopAtLandedWithoutServes {
		fields = append(fields, "stopAtLandedWithoutServes")
	}
	if len(fields) == 0 {
		return old, nil
	}
	at := w.stamp()
	p := &hubapi.TeamPolicy{TeamID: teamID, DefaultState: req.DefaultState,
		PriorityToLeaveTriage: req.PriorityToLeaveTriage, StopAtLandedWithoutServes: req.StopAtLandedWithoutServes,
		UpdatedAt: &at, UpdatedBy: w.userRef(me)}
	w.soc().policies[teamID] = p
	w.workEvent(me, fkEventTeamWorkPolicySet, teamID, "", teamID, map[string]any{"fields": strings.Join(fields, ",")})
	return *p, nil
}
```

Block 3: relations (task 11: stored once, the other end reads the inverse word).

```go
func fkInverse(k hubapi.RelationKind) hubapi.RelationKind {
	switch k {
	case hubapi.RelationBlocks:
		return fkRelationBlockedBy
	case fkRelationBlockedBy:
		return hubapi.RelationBlocks
	case hubapi.RelationDuplicates:
		return fkRelationDuplicatedBy
	case fkRelationDuplicatedBy:
		return hubapi.RelationDuplicates
	}
	return k // relates is its own inverse
}

// findRelation is the stored row for (kind, from, to); relates matches either way.
func (w *fakeWorld) findRelation(kind hubapi.RelationKind, from, to string) int {
	return slices.IndexFunc(w.soc().relations, func(r *fakeRelation) bool {
		return r.kind == kind && (r.from == from && r.to == to ||
			kind == hubapi.RelationRelates && r.from == to && r.to == from)
	})
}

func (w *fakeWorld) relationEvent(me, kind string, is, other *fakeIssue, k hubapi.RelationKind) {
	owner := other.assignee
	if owner == me {
		owner = ""
	}
	w.workEvent(me, kind, is.teamID, is.head.ProjectID, is.head.ID, map[string]any{
		"kind": string(k), "otherIssueId": other.head.ID, "otherAssigneeId": owner})
}

func (f *Fake) AddRelation(_ context.Context, issueID string, req hubapi.RelationRequest) (hubapi.IssueHead, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("AddRelation", issueID, req)
	if err != nil {
		return hubapi.IssueHead{}, err
	}
	is, role, err := w.issueOf(issueID, me)
	if err != nil {
		return hubapi.IssueHead{}, err
	}
	if err := need(role, hubapi.RoleMember); err != nil {
		return hubapi.IssueHead{}, err
	}
	if err := w.erasedWrite(is); err != nil {
		return hubapi.IssueHead{}, err
	}
	switch req.Kind {
	case hubapi.RelationBlocks, hubapi.RelationDuplicates, hubapi.RelationRelates:
	default:
		return hubapi.IssueHead{}, badRequest("kind", "A relation is blocks, duplicates or relates.")
	}
	if req.IssueID == issueID {
		return hubapi.IssueHead{}, badRequest("issueId", "An issue cannot relate to itself.")
	}
	other, _, err := w.issueOf(req.IssueID, me) // unreadable is the same 404, naming the other id
	if err != nil {
		return hubapi.IssueHead{}, err
	}
	if w.findRelation(req.Kind, issueID, other.head.ID) >= 0 {
		return w.renderIssue(is), nil // a repeat: the same head, no event
	}
	if req.Kind != hubapi.RelationRelates && w.findRelation(req.Kind, other.head.ID, issueID) >= 0 {
		return hubapi.IssueHead{}, problem(422, fkRelationCycle, "The other issue already points back at this one.",
			"kind", string(req.Kind), "otherId", other.head.ID)
	}
	w.soc().relations = append(w.soc().relations, &fakeRelation{kind: req.Kind, from: issueID, to: other.head.ID})
	is.head.Relations = append(is.head.Relations, hubapi.Relation{Kind: req.Kind, IssueID: other.head.ID})
	other.head.Relations = append(other.head.Relations, hubapi.Relation{Kind: fkInverse(req.Kind), IssueID: issueID})
	w.relationEvent(me, fkEventIssueRelated, is, other, req.Kind)
	return w.renderIssue(is), nil
}

// RemoveRelation takes any of the five words from either end (task 11).
func (f *Fake) RemoveRelation(_ context.Context, issueID string, kind hubapi.RelationKind, otherID string) (hubapi.IssueHead, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("RemoveRelation", issueID, string(kind), otherID)
	if err != nil {
		return hubapi.IssueHead{}, err
	}
	is, role, err := w.issueOf(issueID, me)
	if err != nil {
		return hubapi.IssueHead{}, err
	}
	if err := need(role, hubapi.RoleMember); err != nil {
		return hubapi.IssueHead{}, err
	}
	stored, from, to := kind, issueID, otherID
	if kind == fkRelationBlockedBy || kind == fkRelationDuplicatedBy {
		stored, from, to = fkInverse(kind), otherID, issueID
	}
	i := w.findRelation(stored, from, to)
	if i < 0 {
		return hubapi.IssueHead{}, problem(404, fkRelationNotFound, "There is no such relation.",
			"kind", string(kind), "otherId", otherID)
	}
	r := w.soc().relations[i]
	w.soc().relations = slices.Delete(w.soc().relations, i, i+1)
	a, b := w.wk().issues[r.from], w.wk().issues[r.to]
	a.head.Relations = slices.DeleteFunc(a.head.Relations, func(x hubapi.Relation) bool { return x.IssueID == r.to && x.Kind == r.kind })
	b.head.Relations = slices.DeleteFunc(b.head.Relations, func(x hubapi.Relation) bool { return x.IssueID == r.from && x.Kind == fkInverse(r.kind) })
	other := b
	if b == is {
		other = a
	}
	w.relationEvent(me, fkEventIssueUnrelated, is, other, r.kind)
	return w.renderIssue(is), nil
}
```

Block 4: team labels (task 11).

```go
// Labels is the team's live labels, oldest first; any member reads them.
func (f *Fake) Labels(_ context.Context, teamID string) ([]hubapi.Label, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("Labels", teamID)
	if err != nil {
		return nil, err
	}
	if _, _, err := w.teamOf(teamID, me); err != nil {
		return nil, err
	}
	out := []hubapi.Label{}
	for _, l := range inOrder(w.soc().labels) {
		if l.label.TeamID == teamID && l.archivedAt == "" {
			out = append(out, l.label)
		}
	}
	return out, nil
}

// CreateLabel is a team admin's; the name is unique among live labels,
// case-insensitively; the colour is a token name.
func (f *Fake) CreateLabel(_ context.Context, teamID string, req hubapi.CreateLabelRequest) (hubapi.Label, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("CreateLabel", teamID, req)
	if err != nil {
		return hubapi.Label{}, err
	}
	_, role, err := w.teamOf(teamID, me)
	if err != nil {
		return hubapi.Label{}, err
	}
	if err := need(role, hubapi.RoleAdmin); err != nil {
		return hubapi.Label{}, err
	}
	name := strings.TrimSpace(req.Name)
	if n := utf8.RuneCountInString(name); n < 1 || n > 40 || strings.ContainsFunc(name, func(r rune) bool { return r < 0x20 || r == 0x7f }) {
		return hubapi.Label{}, badRequest("name", "A label name is 1 to 40 characters.")
	}
	if !slices.Contains(fkLabelColors, req.Color) {
		return hubapi.Label{}, badRequest("color", "A label colour is one of the label tokens.")
	}
	for _, l := range w.soc().labels {
		if l.label.TeamID == teamID && l.archivedAt == "" && sameName(l.label.Name, name) {
			return hubapi.Label{}, problem(409, fkLabelExists, "This team already has that label.", "labelId", l.label.ID)
		}
	}
	l := &fakeLabel{label: hubapi.Label{ID: ids.Wide(hubapi.PrefixLabel), TeamID: teamID, Name: name, Color: req.Color},
		seq: w.next()}
	w.soc().labels[l.label.ID] = l
	w.workEvent(me, fkEventLabelCreated, teamID, "", l.label.ID, map[string]any{"name": name})
	return l.label, nil
}

// ArchiveLabel is a team admin's, once; a label outside the caller's teams is
// 404. Issues keep an archived label they carry.
func (f *Fake) ArchiveLabel(_ context.Context, labelID string) (hubapi.Label, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("ArchiveLabel", labelID)
	if err != nil {
		return hubapi.Label{}, err
	}
	l := w.soc().labels[labelID]
	if l == nil || w.teams[l.label.TeamID].members[me] == nil {
		return hubapi.Label{}, problem(404, fkLabelNotFound, "There is no such label that you can see.", "labelId", labelID)
	}
	if err := need(w.teams[l.label.TeamID].members[me].role, hubapi.RoleAdmin); err != nil {
		return hubapi.Label{}, err
	}
	if l.archivedAt == "" {
		l.archivedAt = w.stamp()
		w.workEvent(me, fkEventLabelArchived, l.label.TeamID, "", labelID, map[string]any{"name": l.label.Name})
	}
	return l.label, nil
}
```

`sameName` is `fake_projects.go:159`'s case-insensitive compare. `inOrder` needs `order()`, which
`fakeLabel` and `fakeComment` declare in block 1.

Block 5: comments (task 12: append-only, one level of replies, checked anchors, team mentions).

```go
func (w *fakeWorld) renderComment(fc *fakeComment) hubapi.Comment {
	c := fc.c
	c.CreatedBy = *w.userRef(fc.createdBy)
	c.Mentions = append([]string{}, c.Mentions...)
	c.Reactions = []hubapi.Reaction{}
	for _, r := range fc.reactions {
		i := slices.IndexFunc(c.Reactions, func(x hubapi.Reaction) bool { return x.Emoji == r[0] })
		if i < 0 {
			c.Reactions, i = append(c.Reactions, hubapi.Reaction{Emoji: r[0], Users: []hubapi.User{}}), len(c.Reactions)
		}
		c.Reactions[i].Users = append(c.Reactions[i].Users, *w.userRef(r[1]))
	}
	return c
}

// commentOf is commentID's comment and the caller's role in its team; one 404
// for unknown and unreadable.
func (w *fakeWorld) commentOf(commentID, me string) (*fakeComment, hubapi.Role, error) {
	if fc := w.soc().comments[commentID]; fc != nil {
		if m := w.teams[fc.teamID].members[me]; m != nil {
			return fc, m.role, nil
		}
	}
	return nil, "", problem(404, fkCommentNotFound, "There is no such comment that you can see.", "commentId", commentID)
}

// checkAnchor is task 12's: text offsets in UTF-16 code units of the named
// revision's description; a criterion that revision serves; a span or hunk
// ref kept verbatim, at most 512 bytes, one line.
func (w *fakeWorld) checkAnchor(is *fakeIssue, a *hubapi.CommentAnchor) error {
	if a == nil {
		return nil
	}
	bad := func(field string) error {
		return problem(422, fkAnchorUnresolvable, "That anchor is not in the revision it names.", "field", field)
	}
	switch a.Kind {
	case "text", "criterion":
		if a.Revision < 1 || a.Revision > len(is.revisions) {
			return bad("anchor.revision")
		}
		rev := is.revisions[a.Revision-1]
		if a.Kind == "text" {
			n := len(utf16.Encode([]rune(rev.Content.Description.Text)))
			if a.CriterionID != "" || a.Ref != "" {
				return badRequest("anchor", "A text anchor names a range only.")
			}
			if a.Start < 0 || a.End <= a.Start || a.End > n {
				return bad("anchor.end")
			}
			return nil
		}
		if !slices.ContainsFunc(rev.Content.Serves, func(s hubapi.Serve) bool { return s.CriterionID == a.CriterionID }) {
			return bad("anchor.criterionId")
		}
	case "span", "hunk":
		if a.Ref == "" || len(a.Ref) > 512 || strings.ContainsAny(a.Ref, "\r\n") {
			return badRequest("anchor.ref", "A ref is one line of at most 512 bytes.")
		}
	default:
		return badRequest("anchor.kind", "An anchor is text, criterion, span or hunk.")
	}
	return nil
}

func (f *Fake) Comments(_ context.Context, issueID string, page hubapi.PageRequest) (hubapi.Page[hubapi.Comment], error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	var zero hubapi.Page[hubapi.Comment]
	me, err := f.enter("Comments", issueID, page)
	if err != nil {
		return zero, err
	}
	if _, _, err := w.issueOf(issueID, me); err != nil {
		return zero, err
	}
	limit, err := pageLimit(page.Limit)
	if err != nil {
		return zero, err
	}
	var rows []*fakeComment
	for _, fc := range inOrder(w.soc().comments) { // oldest first
		if fc.c.IssueID == issueID && (page.After == "" || fc.seq > w.soc().comments[page.After].seqOr0()) {
			rows = append(rows, fc)
		}
	}
	return pageOf(rows, limit, func(fc *fakeComment) string { return fc.c.ID }, w.renderComment), nil
}
```

The cursor is the last comment id seen; `seqOr0` (block 1) makes an unknown one list from the
start.

```go
// CreateComment appends. There is no edit method: a correction is a new comment.
func (f *Fake) CreateComment(_ context.Context, issueID string, req hubapi.CreateCommentRequest) (hubapi.Comment, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("CreateComment", issueID, req)
	if err != nil {
		return hubapi.Comment{}, err
	}
	is, role, err := w.issueOf(issueID, me)
	if err != nil {
		return hubapi.Comment{}, err
	}
	if err := need(role, hubapi.RoleMember); err != nil {
		return hubapi.Comment{}, err
	}
	if err := w.erasedWrite(is); err != nil {
		return hubapi.Comment{}, err
	}
	body := strings.TrimSpace(req.Body.Text)
	switch {
	case !req.Body.ValidAuthor():
		return hubapi.Comment{}, badRequest("body.author", `An author is "H" or "A".`)
	case body == "" || len(req.Body.Text) > hubapi.MaxIssueDescriptionBytes:
		return hubapi.Comment{}, badRequest("body.text", "A comment is 1 character to 16 KiB.")
	case len(req.Mentions) > 20:
		return hubapi.Comment{}, badRequest("mentions", "A comment mentions at most 20 people.")
	}
	if req.ParentID != nil {
		p := w.soc().comments[*req.ParentID]
		if p == nil || p.c.IssueID != issueID || p.c.ParentID != nil {
			return hubapi.Comment{}, problem(422, fkCommentNotTopLevel, "A reply answers a top-level comment of this issue.",
				"field", "parentId")
		}
	}
	if err := w.checkAnchor(is, req.Anchor); err != nil {
		return hubapi.Comment{}, err
	}
	var mentions []string
	for i, u := range req.Mentions {
		if w.teams[is.teamID].members[u] == nil { // a viewer is a member of the team
			return hubapi.Comment{}, badRequest("mentions["+strconv.Itoa(i)+"]", "A mention names a member of this team.")
		}
		if !slices.Contains(mentions, u) {
			mentions = append(mentions, u)
		}
	}
	fc := &fakeComment{teamID: is.teamID, projectID: is.head.ProjectID, createdBy: me, seq: w.next(),
		c: hubapi.Comment{ID: ids.Wide(hubapi.PrefixIssueComment), IssueID: issueID, ParentID: req.ParentID,
			Kind: hubapi.CommentKindComment, Body: req.Body, Anchor: req.Anchor, Mentions: mentions,
			CreatedAt: w.stamp()}}
	w.soc().comments[fc.c.ID] = fc
	anchorKind, parent := "", ""
	if req.Anchor != nil {
		anchorKind = req.Anchor.Kind
	}
	if req.ParentID != nil {
		parent = *req.ParentID
	}
	w.workEvent(me, fkEventCommentCreated, is.teamID, is.head.ProjectID, fc.c.ID, map[string]any{
		"issueId": issueID, "parentId": parent, "kind": hubapi.CommentKindComment,
		"author": req.Body.Author, "anchorKind": anchorKind})
	for _, u := range mentions {
		w.workEvent(me, fkEventCommentMentioned, is.teamID, is.head.ProjectID, u,
			map[string]any{"commentId": fc.c.ID, "issueId": issueID})
	}
	w.subscribeOn(is, me, hubapi.SubscribeComment)
	for _, u := range mentions {
		w.subscribeOn(is, u, hubapi.SubscribeMention)
	}
	return w.renderComment(fc), nil
}

// fkEmojiOK stands in for task 12's ValidReactionEmoji until it lands: no
// ASCII at all (so no :shortcode: and no plain text), 1 to 32 bytes.
func fkEmojiOK(s string) bool {
	return s != "" && len(s) <= 32 && utf8.ValidString(s) &&
		!strings.ContainsFunc(s, func(r rune) bool { return r < 0x80 })
}

// React is idempotent per (comment, user, emoji): on twice is one row and one
// event; off removes it; off again is 200 and no event.
func (f *Fake) React(_ context.Context, commentID string, req hubapi.ReactRequest) (hubapi.Comment, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("React", commentID, req)
	if err != nil {
		return hubapi.Comment{}, err
	}
	fc, role, err := w.commentOf(commentID, me)
	if err != nil {
		return hubapi.Comment{}, err
	}
	if err := need(role, hubapi.RoleMember); err != nil {
		return hubapi.Comment{}, err
	}
	if !fkEmojiOK(req.Emoji) {
		return hubapi.Comment{}, badRequest("emoji", "A reaction is one emoji.")
	}
	pair := [2]string{req.Emoji, me}
	i := slices.Index(fc.reactions, pair)
	switch {
	case req.On && i < 0:
		fc.reactions = append(fc.reactions, pair)
	case !req.On && i >= 0:
		fc.reactions = slices.Delete(fc.reactions, i, i+1)
	default:
		return w.renderComment(fc), nil
	}
	w.workEvent(me, fkEventCommentReacted, fc.teamID, fc.projectID, commentID,
		map[string]any{"emoji": req.Emoji, "on": req.On})
	return w.renderComment(fc), nil
}

// ResolveComment resolves a top-level comment only; a repeat is no event.
func (f *Fake) ResolveComment(_ context.Context, commentID string, req hubapi.ResolveRequest) (hubapi.Comment, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("ResolveComment", commentID, req)
	if err != nil {
		return hubapi.Comment{}, err
	}
	fc, role, err := w.commentOf(commentID, me)
	if err != nil {
		return hubapi.Comment{}, err
	}
	if err := need(role, hubapi.RoleMember); err != nil {
		return hubapi.Comment{}, err
	}
	if fc.c.ParentID != nil {
		return hubapi.Comment{}, problem(422, fkCommentNotTopLevel, "Only a top-level comment is resolved.", "field", "commentId")
	}
	if req.Resolved == (fc.c.ResolvedAt != nil) {
		return w.renderComment(fc), nil
	}
	fc.c.ResolvedAt, fc.c.ResolvedBy = nil, nil
	if req.Resolved {
		at := w.stamp()
		fc.c.ResolvedAt, fc.c.ResolvedBy = &at, w.userRef(me)
	}
	w.workEvent(me, fkEventCommentResolved, fc.teamID, fc.projectID, commentID, map[string]any{"resolved": req.Resolved})
	return w.renderComment(fc), nil
}

// Subscribe is the person's own choice (cause manual), a viewer's too; an
// unchanged choice is no event. No trigger ever changes this row again.
func (f *Fake) Subscribe(_ context.Context, issueID string, req hubapi.SubscribeRequest) (hubapi.Subscription, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("Subscribe", issueID, req)
	if err != nil {
		return hubapi.Subscription{}, err
	}
	is, _, err := w.issueOf(issueID, me)
	if err != nil {
		return hubapi.Subscription{}, err
	}
	k := [2]string{issueID, me}
	s := w.soc().subs[k]
	if s != nil && s.subscribed == req.Subscribed {
		return hubapi.Subscription{IssueID: issueID, User: *w.userRef(me), Cause: s.cause, Subscribed: s.subscribed}, nil
	}
	s = &fakeSub{cause: hubapi.SubscribeManual, subscribed: req.Subscribed, seq: w.next()}
	w.soc().subs[k] = s
	w.workEvent(me, fkEventSubscriptionSet, is.teamID, is.head.ProjectID, me,
		map[string]any{"issueId": issueID, "subscribed": req.Subscribed, "cause": string(s.cause)})
	return hubapi.Subscription{IssueID: issueID, User: *w.userRef(me), Cause: s.cause, Subscribed: s.subscribed}, nil
}

// Subscriptions is the caller's rows on issues they can read now, newest
// first; a leaver's rows are kept and hidden (task 13). after is an issue id.
func (f *Fake) Subscriptions(_ context.Context, page hubapi.PageRequest) (hubapi.Page[hubapi.Subscription], error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	var zero hubapi.Page[hubapi.Subscription]
	me, err := f.enter("Subscriptions", page)
	if err != nil {
		return zero, err
	}
	limit, err := pageLimit(page.Limit)
	if err != nil {
		return zero, err
	}
	type row struct {
		issueID string
		s       *fakeSub
	}
	var rows []row
	var after int64 = 1 << 62
	if s := w.soc().subs[[2]string{page.After, me}]; s != nil {
		after = s.seq
	}
	for k, s := range w.soc().subs {
		if is := w.wk().issues[k[0]]; k[1] == me && s.seq < after && is != nil && w.teams[is.teamID].members[me] != nil {
			rows = append(rows, row{k[0], s})
		}
	}
	slices.SortFunc(rows, func(a, b row) int { return int(b.s.seq - a.s.seq) })
	return pageOf(rows, limit, func(r row) string { return r.issueID }, func(r row) hubapi.Subscription {
		return hubapi.Subscription{IssueID: r.issueID, User: *w.userRef(me), Cause: r.s.cause, Subscribed: r.s.subscribed}
	}), nil
}
```

That is the whole of `fake_work_social.go`.

#### `fake_work_intake.go`, complete

Three blocks; the file is their concatenation. Block 1: import (task 15, decision (h)).

```go
package hubclient

import (
	"context"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/ids"
)

// Phase W, part 2: import, search, attachments and erase. The rules are tasks
// 09, 14 and 15's, the names theirs (task 05's decision (c)).
const (
	fkMaxImportItems     = 500
	fkEventIssueAttached = "issue.attached"
	fkEventIssueErased   = "issue.erased"
	fkEventCommentErased = "comment.erased"
)

// ImportBatch checks every item before it writes any; a re-run answers the
// same ids under existing and writes nothing.
func (f *Fake) ImportBatch(_ context.Context, projectID string, batch hubapi.ImportBatch) (hubapi.ImportResult, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("ImportBatch", projectID, batch)
	if err != nil {
		return hubapi.ImportResult{}, err
	}
	p, role, err := w.projectOf(projectID, me)
	if err != nil {
		return hubapi.ImportResult{}, err
	}
	if err := need(role, hubapi.RoleMember); err != nil {
		return hubapi.ImportResult{}, err
	}
	if batch.Source != hubapi.ImportSourceLinear && batch.Source != hubapi.ImportSourceCSV {
		return hubapi.ImportResult{}, badRequest("source", "An import is from linear or csv.")
	}
	if n := len(batch.Items); n < 1 || n > fkMaxImportItems {
		return hubapi.ImportResult{}, badRequest("items", "An import is 1 to 500 items.")
	}
	seen := map[string]bool{}
	for i, it := range batch.Items {
		at := "items[" + strconv.Itoa(i) + "]"
		u, uerr := url.Parse(it.URL)
		switch {
		case it.ExternalID == "" || seen[it.ExternalID]:
			return hubapi.ImportResult{}, badRequest(at+".externalId", "Each item has its own external id.")
		case it.URL != "" && (uerr != nil || u.Scheme != "https" || u.Host == ""):
			return hubapi.ImportResult{}, badRequest(at+".url", "A link is https, or empty.")
		case it.Priority != "" && !hubapi.ValidPriority(it.Priority):
			return hubapi.ImportResult{}, badRequest(at+".priority", "That is not a priority.")
		}
		seen[it.ExternalID] = true
		if err := hubapi.ValidateIssueRevisionContent(importContent(it)); err != nil {
			return hubapi.ImportResult{}, contentError(at, err)
		}
	}
	res := hubapi.ImportResult{Created: []hubapi.IssueRef{}, Existing: []hubapi.IssueRef{}, UnmatchedLabels: []string{}}
	for _, it := range batch.Items {
		key := [3]string{projectID, batch.Source, it.ExternalID}
		if id := w.soc().imports[key]; id != "" {
			res.Existing = append(res.Existing, hubapi.IssueRef{IssueID: id, Number: w.wk().issues[id].head.Number, ExternalID: it.ExternalID})
			continue
		}
		labels := w.matchLabels(p.teamID, it.Labels, &res.UnmatchedLabels)
		is := w.insertImported(me, p, batch.Source, it, labels)
		w.soc().imports[key] = is.head.ID
		res.Created = append(res.Created, hubapi.IssueRef{IssueID: is.head.ID, Number: is.head.Number, ExternalID: it.ExternalID})
	}
	return res, nil
}

// importContent is an item's revision 1: both texts a person's, no criterion.
func importContent(it hubapi.ImportItem) hubapi.IssueRevisionContent {
	return hubapi.IssueRevisionContent{Title: hubapi.AuthoredText{Text: it.Title, Author: hubapi.AuthorHuman},
		Description: hubapi.AuthoredText{Text: it.Description, Author: hubapi.AuthorHuman},
		Serves: []hubapi.Serve{}, Implements: []string{}}
}
```

Block 2: the imported issue, label matching, and search.

```go
// matchLabels is the team's live label ids for names, case-insensitively; a
// name with none is added once to unmatched. An import never creates a label.
func (w *fakeWorld) matchLabels(teamID string, names []string, unmatched *[]string) []string {
	out := []string{}
	for _, n := range names {
		i := slices.IndexFunc(inOrder(w.soc().labels), func(l *fakeLabel) bool {
			return l.label.TeamID == teamID && l.archivedAt == "" && sameName(l.label.Name, n)
		})
		switch {
		case i >= 0:
			if id := inOrder(w.soc().labels)[i].label.ID; !slices.Contains(out, id) {
				out = append(out, id)
			}
		case !slices.ContainsFunc(*unmatched, func(u string) bool { return sameName(u, n) }):
			*unmatched = append(*unmatched, n)
		}
	}
	return out
}

// insertImported is CreateIssue's write for an intake item: triage whatever
// the team's default, not_tied/imported, source intake, one issue.created,
// and no subscription for the importer (task 15, task 13's exception).
func (w *fakeWorld) insertImported(me string, p *fakeProject, source string, it hubapi.ImportItem, labels []string) *fakeIssue {
	wk := w.wk()
	id := ids.Wide(hubapi.PrefixIssue)
	content := importContent(it)
	digest, _, _ := hubapi.IssueRevisionDigest(hubapi.IssueRevisionEnvelope{IssueID: id, Revision: 1,
		ProjectID: p.id, Content: content}) // checked before the first write
	priority := it.Priority
	if priority == "" {
		priority = hubapi.PriorityNone
	}
	wk.numbers[p.id]++
	now := w.stamp()
	is := &fakeIssue{teamID: p.teamID, createdBy: me, notTied: hubapi.NotTiedImported, seq: w.next(),
		head: hubapi.IssueHead{ID: id, ProjectID: p.id, Number: wk.numbers[p.id], State: hubapi.StateTriage,
			Priority: priority, Labels: labels, HeadRevision: 1, Source: hubapi.SourceIntake,
			External: &hubapi.ExternalRef{Source: source, ExternalID: it.ExternalID, URL: it.URL, AuthorName: it.AuthorName},
			CreatedAt: now, UpdatedAt: now}}
	is.revisions = []*hubapi.IssueRevision{{IssueID: id, Revision: 1, Key: hubapi.IssueKey{IssueID: id, Revision: 1}.String(),
		Digest: digest, ProjectID: p.id, Content: content, WrittenAt: now, WrittenBy: *w.userRef(me)}}
	wk.issues[id] = is
	w.workEvent(me, fkEventIssueCreated, p.teamID, p.id, id, map[string]any{"number": int64(is.head.Number),
		"key": is.revisions[0].Key, "digest": digest.String(), "state": string(hubapi.StateTriage),
		"source": string(hubapi.SourceIntake)})
	return is
}

// SearchIssues matches titles only, case-insensitively, in the caller's
// readable projects, never an erased issue's; newest first; at most 50.
func (f *Fake) SearchIssues(_ context.Context, q hubapi.IssueSearchQuery) ([]hubapi.IssueHead, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("SearchIssues", q)
	if err != nil {
		return nil, err
	}
	needle := strings.ToLower(strings.TrimSpace(q.Q))
	limit := q.Limit
	switch {
	case needle == "" || utf8.RuneCountInString(needle) > 200:
		return nil, badRequest("q", "A search is 1 to 200 characters.")
	case limit == 0:
		limit = 50
	case limit < 0 || limit > 50:
		return nil, badRequest("limit", "A search returns at most 50 issues.")
	}
	if q.ProjectID != "" {
		if _, _, err := w.projectOf(q.ProjectID, me); err != nil {
			return nil, err
		}
	}
	var rows []*fakeIssue
	for _, is := range w.wk().issues {
		if w.teams[is.teamID].members[me] != nil && !w.soc().erased[is.head.ID] &&
			(q.ProjectID == "" || is.head.ProjectID == q.ProjectID) &&
			strings.Contains(strings.ToLower(is.headRev().Content.Title.Text), needle) {
			rows = append(rows, is)
		}
	}
	slices.SortFunc(rows, func(a, b *fakeIssue) int { return int(b.seq - a.seq) })
	out := []hubapi.IssueHead{}
	for _, is := range rows[:min(limit, len(rows))] {
		out = append(out, w.renderIssue(is))
	}
	return out, nil
}
```

Block 3: attachments and the sixth hook. `AttachBlob` reads the blob through the fake's own
`GetBlob`, outside the world's lock, so this file knows nothing of H05's blob store: a digest that
another project holds and one never put are the same 404 `blob_not_found`, the body `GetBlob`
already answers (task 09's `TestAttachmentsAreThisProjectsBlobs`). The cost is one extra
`GetBlob(…)` line in `Calls()` after the `AttachBlob(…)` line, which the fake-only test pins.

```go
// AttachBlob names a blob already put to the issue's project. A repeat
// answers the first attachment and appends no event.
func (f *Fake) AttachBlob(ctx context.Context, issueID string, req hubapi.AttachRequest) (hubapi.IssueAttachment, error) {
	projectID, err := f.attachTarget(issueID, req)
	if err != nil {
		return hubapi.IssueAttachment{}, err
	}
	rc, blob, err := f.GetBlob(ctx, projectID, req.Digest.String()) // H05's 404, same body both ways
	if err != nil {
		return hubapi.IssueAttachment{}, err
	}
	rc.Close()
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	is := w.wk().issues[issueID]
	if err := w.erasedWrite(is); err != nil { // erased between the two locks
		return hubapi.IssueAttachment{}, err
	}
	list := w.soc().attached[issueID]
	if i := slices.IndexFunc(list, func(a hubapi.IssueAttachment) bool { return a.Digest == req.Digest }); i >= 0 {
		return list[i], nil
	}
	me := f.as.ID
	a := hubapi.IssueAttachment{IssueID: issueID, Digest: req.Digest, ContentType: blob.ContentType,
		Size: int(blob.Size), AddedAt: w.stamp(), AddedBy: *w.userRef(me)}
	w.soc().attached[issueID] = append(list, a)
	w.workEvent(me, fkEventIssueAttached, is.teamID, is.head.ProjectID, issueID, map[string]any{"digest": req.Digest.String()})
	return a, nil
}

// attachTarget is the issue's project, once the caller may attach to it.
func (f *Fake) attachTarget(issueID string, req hubapi.AttachRequest) (string, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("AttachBlob", issueID, req)
	if err != nil {
		return "", err
	}
	is, role, err := w.issueOf(issueID, me)
	switch {
	case err != nil:
		return "", err
	case req.Digest.IsZero():
		return "", badRequest("digest", "An attachment names a blob by its digest.")
	}
	if err := need(role, hubapi.RoleMember); err != nil {
		return "", err
	}
	if err := w.erasedWrite(is); err != nil {
		return "", err
	}
	return is.head.ProjectID, nil
}

// Attachments is oldest first; a viewer reads them.
func (f *Fake) Attachments(_ context.Context, issueID string) ([]hubapi.IssueAttachment, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("Attachments", issueID)
	if err != nil {
		return nil, err
	}
	if _, _, err := w.issueOf(issueID, me); err != nil {
		return nil, err
	}
	return append([]hubapi.IssueAttachment{}, w.soc().attached[issueID]...), nil
}

// erasedWrite is task 14's rule: an erased issue takes no write, 409 conflict.
func (w *fakeWorld) erasedWrite(is *fakeIssue) error {
	if is != nil && w.soc().erased[is.head.ID] {
		return problem(409, hubapi.CodeConflict, "That issue was erased; it takes no more writes.", "issueId", is.head.ID)
	}
	return nil
}
```

Block 3, continued: erase (task 14, decision (g)). The fake keeps no reason: it has no audit log
(H05 has none), so the reason is checked and dropped, and no feed event can carry it.

```go
const fkRequiredOrgAdmin = "org_admin" // task 14: forbidden's details.required

// EraseIssue is an org admin's, in any team or none. Every revision's texts
// read "" with ErasedAt set; ids, keys, digests, the number and the alias
// stay; the state reason and the import's link and author name go; so does
// every comment not yet erased. One issue.erased each time; a second answers
// zeros and still appends its event (task 14's TestASecondEraseIsZeroAndAudited).
func (f *Fake) EraseIssue(_ context.Context, issueID string, req hubapi.EraseRequest) (hubapi.WorkEraseResponse, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := f.enterErase("EraseIssue", issueID, req); err != nil {
		return hubapi.WorkEraseResponse{}, err
	}
	is := w.wk().issues[issueID]
	if is == nil {
		return hubapi.WorkEraseResponse{}, problem(404, fkIssueNotFound, "There is no such issue.", "issueId", issueID)
	}
	now := w.stamp()
	res := hubapi.WorkEraseResponse{IssueID: issueID, ErasedAt: now}
	for _, r := range is.revisions {
		if r.ErasedAt == nil {
			r.Content.Title.Text, r.Content.Description.Text, r.ErasedAt = "", "", &now
			res.ErasedRevisions++
		}
	}
	for _, c := range w.soc().comments {
		if c.c.IssueID == issueID && eraseComment(c, now) {
			res.ErasedComments++
		}
	}
	is.head.StateReason = ""
	if x := is.head.External; x != nil {
		x.URL, x.AuthorName = "", "" // task 15's addition; ExternalID stays, so a re-import is existing
	}
	w.soc().erased[issueID] = true
	w.workEvent(f.as.ID, fkEventIssueErased, is.teamID, is.head.ProjectID, issueID, map[string]any{
		"erasedRevisions": int64(res.ErasedRevisions), "erasedComments": int64(res.ErasedComments)})
	return res, nil
}

// EraseComment blanks one comment; its replies and its issue are untouched.
func (f *Fake) EraseComment(_ context.Context, commentID string, req hubapi.EraseRequest) (hubapi.WorkEraseResponse, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := f.enterErase("EraseComment", commentID, req); err != nil {
		return hubapi.WorkEraseResponse{}, err
	}
	c := w.soc().comments[commentID]
	if c == nil {
		return hubapi.WorkEraseResponse{}, problem(404, fkCommentNotFound, "There is no such comment.", "commentId", commentID)
	}
	now := w.stamp()
	res := hubapi.WorkEraseResponse{IssueID: c.c.IssueID, CommentID: commentID, ErasedAt: now}
	if eraseComment(c, now) {
		res.ErasedComments = 1
	}
	w.workEvent(f.as.ID, fkEventCommentErased, c.teamID, c.projectID, commentID, map[string]any{}) // task 13's CommentErasedData{}
	return res, nil
}

// enterErase is both erases' first checks, in EraseBrief's order: the log and
// switches, the org admin (before any lookup), then the reason.
func (f *Fake) enterErase(method, id string, req hubapi.EraseRequest) error {
	if _, err := f.enter(method, id, req); err != nil {
		return err
	}
	if !f.as.OrgAdmin {
		return forbidden(fkRequiredOrgAdmin)
	}
	if strings.TrimSpace(req.Reason) == "" || utf8.RuneCountInString(req.Reason) > 500 {
		return badRequest("reason", "A reason is 1 to 500 characters, not only spaces.")
	}
	return nil
}

// eraseComment blanks c's body; false when c was erased already.
func eraseComment(c *fakeComment, now string) bool {
	if c.c.ErasedAt != nil {
		return false
	}
	c.c.Body.Text, c.c.ErasedAt = "", &now
	return true
}
```

That is the whole of `fake_work_intake.go`.

---

#### The rows, complete: `conformance/work.go`, after the head

Each row is a function named `w<Name>`, so a failing row's stack names it, and `WorkRowsW1` (the
last block) lists them in `ID` order. A row asserts **strings** (codes, detail values, event
kinds), never a fake-only field, so the same row runs on the real Hub in task 16. Rows W-01 to
W-09 hold part 1 (task 04's rules; tasks 08-10's server); W-10 to W-25 hold part 2. The rows
add `"bytes"` and `"slices"` to the head's import block above; nothing else in the head changes.

Block 1: numbers, roles, the key, revisions, the patch.

```go
func wNumbersGapless(t *testing.T, e WorkEnv) {
	p := e.project(t, "Portal", "")
	var got []int
	for i, title := range []string{"one", "", "two", "three"} { // the "" create is refused
		iw, err := e.Member.CreateIssue(wctx, p.ID, wChore(title))
		if i == 1 {
			wRefused(t, err, 400, hubapi.CodeBadRequest)
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, iw.Head.Number)
	}
	if !slices.Equal(got, []int{1, 2, 3}) {
		t.Errorf("numbers = %v, want [1 2 3]: a refused create takes no number", got)
	}
}

func wViewerCannotWrite(t *testing.T, e WorkEnv) {
	p := e.project(t, "Portal", "")
	_, err := e.Viewer.CreateIssue(wctx, p.ID, wChore("no"))
	wRefused(t, err, 403, hubapi.CodeForbidden, "required", "member")
	iw := e.issue(t, p, "yes")
	_, err = e.Outsider.Issue(wctx, iw.Head.ID)
	wRefused(t, err, 404, "issue_not_found", "issueId", iw.Head.ID)
}

func wKeyIsTheAlias(t *testing.T, e WorkEnv) {
	p := e.project(t, "Portal", "EP")
	iw := e.issue(t, p, "first")
	if iw.Head.Alias != "EP-1" || iw.Head.HeadKey != "issue:"+iw.Head.ID+"@1" {
		t.Errorf("alias %q, headKey %q; want EP-1 and the id key", iw.Head.Alias, iw.Head.HeadKey)
	}
	p2 := e.project(t, "Billing", "")
	_, err := e.Admin.SetProjectKey(wctx, p2.ID, "EP")
	wRefused(t, err, 409, "project_key_taken", "key", "EP")
	if _, err := e.Admin.SetProjectKey(wctx, p.ID, "PAY"); err != nil {
		t.Fatal(err)
	}
	again, err := e.Member.Issue(wctx, iw.Head.ID)
	if err != nil || again.Head.Alias != "PAY-1" || again.Head.HeadKey != iw.Head.HeadKey {
		t.Errorf("after the key moved: %+v, %v; want PAY-1 and the same key", again.Head, err)
	}
}

func wStaleBaseConflicts(t *testing.T, e WorkEnv) {
	p := e.project(t, "Portal", "")
	iw := e.issue(t, p, "v1")
	next := iw.Revision.Content
	next.Title = wText("v2")
	if _, err := e.Member.WriteIssueRevision(wctx, iw.Head.ID, hubapi.WriteIssueRevisionRequest{BaseRevision: 1, Content: next}); err != nil {
		t.Fatal(err)
	}
	next.Title = wText("lost")
	_, err := e.Admin.WriteIssueRevision(wctx, iw.Head.ID, hubapi.WriteIssueRevisionRequest{BaseRevision: 1, Content: next})
	wRefused(t, err, 409, hubapi.CodeRevisionConflict)
	if got, _ := e.Member.Issue(wctx, iw.Head.ID); got.Head.HeadRevision != 2 || got.Revision.Content.Title.Text != "v2" {
		t.Errorf("head = %d %q, want 2 v2: a stale write wrote", got.Head.HeadRevision, got.Revision.Content.Title.Text)
	}
}
```

Block 2: the restore, the patch, and the moves.

```go
func wRestoreIsANewRevision(t *testing.T, e WorkEnv) {
	p := e.project(t, "Portal", "")
	iw := e.issue(t, p, "v1")
	next := iw.Revision.Content
	next.Title = wText("v2")
	if _, err := e.Member.WriteIssueRevision(wctx, iw.Head.ID, hubapi.WriteIssueRevisionRequest{BaseRevision: 1, Content: next}); err != nil {
		t.Fatal(err)
	}
	r, err := e.Member.RestoreIssueRevision(wctx, iw.Head.ID, hubapi.RestoreRevisionRequest{FromRevision: 1})
	if err != nil {
		t.Fatal(err)
	}
	if r.Revision != 3 || r.RestoredFrom == nil || *r.RestoredFrom != 1 || r.Content.Title.Text != "v1" {
		t.Errorf("restore = rev %d from %v %q; want 3 from 1 \"v1\"", r.Revision, r.RestoredFrom, r.Content.Title.Text)
	}
	one, err := e.Member.IssueRevisionAt(wctx, iw.Head.ID, 1)
	if err != nil || one.Digest != iw.Revision.Digest || one.WrittenAt != iw.Revision.WrittenAt {
		t.Errorf("revision 1 changed: %+v, %v", one, err)
	}
}

func wPatchWritesNoRevision(t *testing.T, e WorkEnv) {
	p := e.project(t, "Portal", "")
	iw := e.issue(t, p, "one")
	from := wHead(t, e.Member)
	high := hubapi.PriorityHigh
	h, err := e.Member.PatchIssue(wctx, iw.Head.ID, hubapi.PatchIssueRequest{Priority: &high})
	if err != nil || h.Priority != high || h.HeadRevision != 1 {
		t.Fatalf("patch = %+v, %v; want high at revision 1", h, err)
	}
	if got := wKinds(t, e.Member, from, "issue."); !slices.Equal(got, []string{"issue.updated"}) {
		t.Errorf("events = %v, want [issue.updated]: a patch never revises", got)
	}
}

func wHandCannotSetAFact(t *testing.T, e WorkEnv) {
	p := e.project(t, "Portal", "")
	iw := e.issue(t, p, "one")
	for _, to := range []hubapi.IssueState{hubapi.StateStarted, hubapi.StateInReview, hubapi.StateLanded} {
		_, err := e.Member.MoveIssueState(wctx, iw.Head.ID, hubapi.StateMove{To: to, Cause: hubapi.CauseHand, Evidence: []string{}})
		wRefused(t, err, 422, "state_not_allowed", "from", string(iw.Head.State), "to", string(to), "cause", "hand")
	}
}

func wHandDoneOfAChoreHasNoCriteria(t *testing.T, e WorkEnv) {
	p := e.project(t, "Portal", "")
	iw := e.issue(t, p, "one")
	h, err := e.Member.MoveIssueState(wctx, iw.Head.ID, hubapi.StateMove{To: hubapi.StateDone, Cause: hubapi.CauseHand, Evidence: []string{}})
	if err != nil || h.State != hubapi.StateDone || h.Proof != hubapi.ProofNoCriteria {
		t.Errorf("hand done = %s %q, %v; want done no_criteria, never proven", h.State, h.Proof, err)
	}
}

func wReopenClearsTheProofWord(t *testing.T, e WorkEnv) {
	p := e.project(t, "Portal", "")
	iw := e.issue(t, p, "one")
	done := hubapi.StateMove{To: hubapi.StateDone, Cause: hubapi.CauseHand, Reason: "shipped by hand", Evidence: []string{}}
	if _, err := e.Member.MoveIssueState(wctx, iw.Head.ID, done); err != nil {
		t.Fatal(err)
	}
	h, err := e.Member.MoveIssueState(wctx, iw.Head.ID, hubapi.StateMove{To: hubapi.StateTodo, Cause: hubapi.CauseHand, Evidence: []string{}})
	if err != nil || h.Proof != hubapi.ProofNone || h.StateReason != "" {
		t.Errorf("reopen = proof %q reason %q, %v; want both empty", h.Proof, h.StateReason, err)
	}
}
```

`wHandDoneOfAChoreHasNoCriteria` holds task 04's rule that a hand-set done of an issue serving
nothing reads `no_criteria`, not `not_proven`: the word a chore earns, never the word proof earns.

Block 3: relations and labels.

```go
func wRelationReadsTheInverse(t *testing.T, e WorkEnv) {
	p := e.project(t, "Portal", "")
	a, b := e.issue(t, p, "a"), e.issue(t, p, "b")
	from := wHead(t, e.Member)
	for range 2 { // the repeat is the same head and no event
		if _, err := e.Member.AddRelation(wctx, a.Head.ID, hubapi.RelationRequest{Kind: hubapi.RelationBlocks, IssueID: b.Head.ID}); err != nil {
			t.Fatal(err)
		}
	}
	other, _ := e.Viewer.Issue(wctx, b.Head.ID)
	want := []hubapi.Relation{{Kind: "blocked_by", IssueID: a.Head.ID}}
	if !slices.Equal(other.Head.Relations, want) {
		t.Errorf("b's relations = %v, want %v", other.Head.Relations, want)
	}
	if got := wKinds(t, e.Member, from, "issue.related"); len(got) != 1 {
		t.Errorf("issue.related events = %d, want 1", len(got))
	}
}

func wRelationCycleRefused(t *testing.T, e WorkEnv) {
	p := e.project(t, "Portal", "")
	a, b := e.issue(t, p, "a"), e.issue(t, p, "b")
	if _, err := e.Member.AddRelation(wctx, a.Head.ID, hubapi.RelationRequest{Kind: hubapi.RelationBlocks, IssueID: b.Head.ID}); err != nil {
		t.Fatal(err)
	}
	_, err := e.Member.AddRelation(wctx, b.Head.ID, hubapi.RelationRequest{Kind: hubapi.RelationBlocks, IssueID: a.Head.ID})
	wRefused(t, err, 422, "relation_cycle", "kind", "blocks", "otherId", a.Head.ID)
	_, err = e.Member.AddRelation(wctx, a.Head.ID, hubapi.RelationRequest{Kind: hubapi.RelationRelates, IssueID: a.Head.ID})
	wRefused(t, err, 400, hubapi.CodeBadRequest, "field", "issueId")
}

func wRelationRemovedFromTheOtherEnd(t *testing.T, e WorkEnv) {
	p := e.project(t, "Portal", "")
	a, b := e.issue(t, p, "a"), e.issue(t, p, "b")
	if _, err := e.Member.AddRelation(wctx, a.Head.ID, hubapi.RelationRequest{Kind: hubapi.RelationDuplicates, IssueID: b.Head.ID}); err != nil {
		t.Fatal(err)
	}
	h, err := e.Member.RemoveRelation(wctx, b.Head.ID, "duplicated_by", a.Head.ID)
	if err != nil || len(h.Relations) != 0 {
		t.Fatalf("remove from b = %v, %v; want no relations", h.Relations, err)
	}
	if got, _ := e.Member.Issue(wctx, a.Head.ID); len(got.Head.Relations) != 0 {
		t.Errorf("a still reads %v: a relation is stored once", got.Head.Relations)
	}
	_, err = e.Member.RemoveRelation(wctx, a.Head.ID, hubapi.RelationDuplicates, b.Head.ID)
	wRefused(t, err, 404, "relation_not_found", "kind", "duplicates", "otherId", b.Head.ID)
}

func wLabelNamesAreTheTeams(t *testing.T, e WorkEnv) {
	l, err := e.Admin.CreateLabel(wctx, e.TeamID, hubapi.CreateLabelRequest{Name: "Bug", Color: "fail"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = e.Admin.CreateLabel(wctx, e.TeamID, hubapi.CreateLabelRequest{Name: " bug ", Color: "run"})
	wRefused(t, err, 409, "label_exists", "labelId", l.ID)
	_, err = e.Member.CreateLabel(wctx, e.TeamID, hubapi.CreateLabelRequest{Name: "Chore", Color: "run"})
	wRefused(t, err, 403, hubapi.CodeForbidden, "required", "admin")
	_, err = e.Admin.CreateLabel(wctx, e.TeamID, hubapi.CreateLabelRequest{Name: "Pink", Color: "#ff00aa"})
	wRefused(t, err, 400, hubapi.CodeBadRequest, "field", "color")
}

func wArchivedLabelIsUnavailable(t *testing.T, e WorkEnv) {
	p := e.project(t, "Portal", "")
	iw := e.issue(t, p, "one")
	l, err := e.Admin.CreateLabel(wctx, e.TeamID, hubapi.CreateLabelRequest{Name: "Bug", Color: "fail"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Admin.ArchiveLabel(wctx, l.ID); err != nil {
		t.Fatal(err)
	}
	_, err = e.Member.PatchIssue(wctx, iw.Head.ID, hubapi.PatchIssueRequest{Labels: &[]string{l.ID}})
	wRefused(t, err, 422, "label_unavailable", "field", "labels[0]")
	if live, _ := e.Viewer.Labels(wctx, e.TeamID); len(live) != 0 {
		t.Errorf("labels = %v, want none live", live)
	}
}
```

Block 4: the team policy, comments, reactions and subscriptions.

```go
func wPolicyGatesTheTriageExit(t *testing.T, e WorkEnv) {
	if pol, err := e.Viewer.TeamPolicy(wctx, e.TeamID); err != nil || pol.DefaultState != hubapi.StateBacklog || pol.UpdatedAt != nil {
		t.Fatalf("default policy = %+v, %v; want backlog, never set", pol, err)
	}
	put := hubapi.PutTeamPolicyRequest{DefaultState: hubapi.StateTriage, PriorityToLeaveTriage: true}
	_, err := e.Member.PutTeamPolicy(wctx, e.TeamID, put)
	wRefused(t, err, 403, hubapi.CodeForbidden, "required", "admin")
	if _, err := e.Admin.PutTeamPolicy(wctx, e.TeamID, put); err != nil {
		t.Fatal(err)
	}
	p := e.project(t, "Portal", "")
	iw := e.issue(t, p, "one")
	if iw.Head.State != hubapi.StateTriage {
		t.Fatalf("state = %s, want the team's default, triage", iw.Head.State)
	}
	_, err = e.Member.MoveIssueState(wctx, iw.Head.ID, hubapi.StateMove{To: hubapi.StateTodo, Cause: hubapi.CauseHand, Evidence: []string{}})
	wRefused(t, err, 422, "policy_refused", "policy", "priorityToLeaveTriage", "issueId", iw.Head.ID)
	if _, err := e.Member.MoveIssueState(wctx, iw.Head.ID, hubapi.StateMove{To: hubapi.StateCancelled, Cause: hubapi.CauseHand, Evidence: []string{}}); err != nil {
		t.Errorf("declining from triage = %v, want allowed without a priority", err)
	}
}

// wSay is a top-level comment, or a reply when parent is set.
func wSay(view hubclient.Hub, issueID, text string, parent *string, anchor *hubapi.CommentAnchor, mentions ...string) (hubapi.Comment, error) {
	return view.CreateComment(wctx, issueID, hubapi.CreateCommentRequest{Body: wText(text), ParentID: parent,
		Anchor: anchor, Mentions: append([]string{}, mentions...)})
}

func wRepliesAreOneLevel(t *testing.T, e WorkEnv) {
	iw := e.issue(t, e.project(t, "Portal", ""), "one")
	top, err := wSay(e.Member, iw.Head.ID, "top", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	reply, err := wSay(e.Admin, iw.Head.ID, "reply", &top.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = wSay(e.Member, iw.Head.ID, "deeper", &reply.ID, nil)
	wRefused(t, err, 422, "comment_not_top_level", "field", "parentId")
	_, err = e.Member.ResolveComment(wctx, reply.ID, hubapi.ResolveRequest{Resolved: true})
	wRefused(t, err, 422, "comment_not_top_level", "field", "commentId")
}

func wTextAnchorsCountUTF16(t *testing.T, e WorkEnv) {
	req := wChore("anchored")
	req.Description = wText("é😀x") // UTF-16: 1+2+1 = 4; code points 3; UTF-8 bytes 7
	iw, err := e.Member.CreateIssue(wctx, e.project(t, "Portal", "").ID, req)
	if err != nil {
		t.Fatal(err)
	}
	at := func(end int) *hubapi.CommentAnchor { return &hubapi.CommentAnchor{Kind: "text", Revision: 1, Start: 0, End: end} }
	if _, err := wSay(e.Member, iw.Head.ID, "whole", nil, at(4)); err != nil {
		t.Errorf("end 4 = %v, want accepted: the description is 4 UTF-16 units", err)
	}
	_, err = wSay(e.Member, iw.Head.ID, "past", nil, at(5))
	wRefused(t, err, 422, "anchor_unresolvable", "field", "anchor.end")
}
```

`é😀x` is where the three counts differ: a code-point counter refuses `end: 4`, and a byte
counter accepts `end: 5`, so either mutant fails `wTextAnchorsCountUTF16`.

Block 5: reactions, subscriptions and import.

```go
func wReactionsAreIdempotent(t *testing.T, e WorkEnv) {
	iw := e.issue(t, e.project(t, "Portal", ""), "one")
	c, err := wSay(e.Member, iw.Head.ID, "top", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	from := wHead(t, e.Member)
	for _, on := range []bool{true, true, false, false} {
		if _, err := e.Admin.React(wctx, c.ID, hubapi.ReactRequest{Emoji: "👍", On: on}); err != nil {
			t.Fatal(err)
		}
	}
	if got := wKinds(t, e.Member, from, "comment.reacted"); len(got) != 2 {
		t.Errorf("comment.reacted events = %d, want 2: one on, one off", len(got))
	}
	_, err = e.Admin.React(wctx, c.ID, hubapi.ReactRequest{Emoji: ":+1:", On: true})
	wRefused(t, err, 400, hubapi.CodeBadRequest, "field", "emoji")
}

// wSub is view's own row for issueID, or ok false.
func wSub(t *testing.T, view hubclient.Hub, issueID string) (hubapi.Subscription, bool) {
	t.Helper()
	page, err := view.Subscriptions(wctx, hubapi.PageRequest{})
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(page.Items, func(s hubapi.Subscription) bool { return s.IssueID == issueID })
	if i < 0 {
		return hubapi.Subscription{}, false
	}
	return page.Items[i], true
}

func wSubscriptionsFollowTriggers(t *testing.T, e WorkEnv) {
	iw := e.issue(t, e.project(t, "Portal", ""), "one")
	if s, ok := wSub(t, e.Member, iw.Head.ID); !ok || s.Cause != hubapi.SubscribeCreate || !s.Subscribed {
		t.Fatalf("creator's row = %+v %v, want create, subscribed", s, ok)
	}
	for range 2 { // the second is unchanged: no event
		if _, err := e.Member.Subscribe(wctx, iw.Head.ID, hubapi.SubscribeRequest{Subscribed: false}); err != nil {
			t.Fatal(err)
		}
	}
	from := wHead(t, e.Member)
	viewer, err := e.Viewer.Me(wctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wSay(e.Member, iw.Head.ID, "ping", nil, nil, viewer.User.ID); err != nil {
		t.Fatal(err)
	}
	if s, _ := wSub(t, e.Member, iw.Head.ID); s.Subscribed || s.Cause != hubapi.SubscribeManual {
		t.Errorf("after commenting: %+v; a trigger changed a manual row", s)
	}
	if s, ok := wSub(t, e.Viewer, iw.Head.ID); !ok || s.Cause != hubapi.SubscribeMention {
		t.Errorf("the mentioned viewer = %+v %v, want mention", s, ok)
	}
	if got := wKinds(t, e.Member, from, "subscription.set"); len(got) != 1 {
		t.Errorf("subscription.set events = %d, want 1 (the viewer's)", len(got))
	}
}

func wImportIsIdempotent(t *testing.T, e WorkEnv) {
	p := e.project(t, "Portal", "")
	if _, err := e.Admin.CreateLabel(wctx, e.TeamID, hubapi.CreateLabelRequest{Name: "Bug", Color: "fail"}); err != nil {
		t.Fatal(err)
	}
	batch := hubapi.ImportBatch{Source: hubapi.ImportSourceCSV, Items: []hubapi.ImportItem{
		{ExternalID: "1", Title: "one", Labels: []string{"BUG", "nope"}},
		{ExternalID: "2", Title: "two", Priority: hubapi.PriorityHigh, Labels: []string{}}}}
	first, err := e.Member.ImportBatch(wctx, p.ID, batch)
	if err != nil || len(first.Created) != 2 || !slices.Equal(first.UnmatchedLabels, []string{"nope"}) {
		t.Fatalf("import = %+v, %v; want 2 created, nope unmatched", first, err)
	}
	from := wHead(t, e.Member)
	again, err := e.Member.ImportBatch(wctx, p.ID, batch)
	if err != nil || len(again.Created) != 0 || len(again.Existing) != 2 || again.Existing[0].IssueID != first.Created[0].IssueID {
		t.Errorf("re-run = %+v, %v; want the same two under existing", again, err)
	}
	if got := wKinds(t, e.Member, from, "issue."); len(got) != 0 {
		t.Errorf("a re-run wrote %v", got)
	}
	one, _ := e.Member.Issue(wctx, first.Created[0].IssueID)
	h := one.Head
	if h.State != hubapi.StateTriage || h.Source != hubapi.SourceIntake || h.ThreadState.Reason != hubapi.NotTiedImported || len(h.Labels) != 1 {
		t.Errorf("imported head = %+v; want triage, intake, not_tied imported, one label", h)
	}
	if _, ok := wSub(t, e.Member, h.ID); ok {
		t.Errorf("the importer was subscribed; an import subscribes nobody")
	}
}
```

Block 6: import refusals, search and attachments.

```go
func wImportIsAllOrNothing(t *testing.T, e WorkEnv) {
	p := e.project(t, "Portal", "")
	_, err := e.Member.ImportBatch(wctx, p.ID, hubapi.ImportBatch{Source: hubapi.ImportSourceLinear, Items: []hubapi.ImportItem{
		{ExternalID: "A-1", Title: "fine"}, {ExternalID: "A-2", Title: "bad link", URL: "http://example.test/A-2"}}})
	wRefused(t, err, 400, hubapi.CodeBadRequest, "field", "items[1].url")
	if page, _ := e.Member.Issues(wctx, p.ID, hubapi.IssueQuery{}, hubapi.PageRequest{}); len(page.Items) != 0 {
		t.Errorf("a refused batch wrote %d issues", len(page.Items))
	}
}

func wSearchReadsOnlyReadableTitles(t *testing.T, e WorkEnv) {
	p := e.project(t, "Portal", "")
	iw := e.issue(t, p, "Resume an upload")
	got, err := e.Viewer.SearchIssues(wctx, hubapi.IssueSearchQuery{Q: "RESUME"})
	if err != nil || len(got) != 1 || got[0].ID != iw.Head.ID {
		t.Errorf("viewer's search = %v, %v; want the one issue", got, err)
	}
	if none, err := e.Outsider.SearchIssues(wctx, hubapi.IssueSearchQuery{Q: "resume"}); err != nil || len(none) != 0 {
		t.Errorf("outsider's search = %v, %v; want nothing", none, err)
	}
	_, err = e.Viewer.SearchIssues(wctx, hubapi.IssueSearchQuery{Q: "x", Limit: 51})
	wRefused(t, err, 400, hubapi.CodeBadRequest, "field", "limit")
}

func wAttachmentsAreThisProjectsBlobs(t *testing.T, e WorkEnv) {
	p, p2 := e.project(t, "Portal", ""), e.project(t, "Billing", "")
	iw := e.issue(t, p, "one")
	put := func(projectID, s string) hubapi.Digest {
		b := []byte(s)
		blob, err := e.Member.PutBlob(wctx, projectID, hubapi.DigestOf(b).String(), "text/plain; charset=utf-8", int64(len(b)), bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		return blob.Digest
	}
	mine, theirs := put(p.ID, "log line"), put(p2.ID, "other log")
	from := wHead(t, e.Member)
	for range 2 {
		if _, err := e.Member.AttachBlob(wctx, iw.Head.ID, hubapi.AttachRequest{Digest: mine}); err != nil {
			t.Fatal(err)
		}
	}
	if got := wKinds(t, e.Member, from, "issue.attached"); len(got) != 1 {
		t.Errorf("issue.attached events = %d, want 1", len(got))
	}
	list, err := e.Viewer.Attachments(wctx, iw.Head.ID)
	if err != nil || len(list) != 1 || list[0].Size != len("log line") || list[0].ContentType != "text/plain; charset=utf-8" {
		t.Errorf("attachments = %+v, %v; want the one blob's type and size", list, err)
	}
	_, err = e.Member.AttachBlob(wctx, iw.Head.ID, hubapi.AttachRequest{Digest: theirs})
	wRefused(t, err, 404, hubapi.CodeBlobNotFound)
	_, err = e.Viewer.AttachBlob(wctx, iw.Head.ID, hubapi.AttachRequest{Digest: mine})
	wRefused(t, err, 403, hubapi.CodeForbidden, "required", "member")
}
```

`put` reads the digest back from `PutBlob`'s answer, not from `DigestOf`, so the row holds H05's
upload and this task's attach to one digest. `hubapi.DigestOf` is `hubapi/keys.go:114` on
`feat/hub`; `hubapi.IssueQuery{}` is task 03's list filter, its zero value filtering nothing.

Block 7: erase, and the list.

```go
// wErased is an issue with two revisions and one comment, erased by the org
// admin; nil OrgAdmin skips the row, as Advance nil does.
func wErased(t *testing.T, e WorkEnv) (hubapi.IssueWithRevision, hubapi.Comment) {
	t.Helper()
	if e.OrgAdmin == nil {
		t.Skip("this Hub has no org-admin principal; the erase rows need one")
	}
	iw := e.issue(t, e.project(t, "Portal", "EP"), "secret title")
	next := iw.Revision.Content
	next.Description = wText("secret words")
	if _, err := e.Member.WriteIssueRevision(wctx, iw.Head.ID, hubapi.WriteIssueRevisionRequest{BaseRevision: 1, Content: next}); err != nil {
		t.Fatal(err)
	}
	c, err := wSay(e.Member, iw.Head.ID, "secret comment", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = e.Admin.EraseIssue(wctx, iw.Head.ID, hubapi.EraseRequest{Reason: "matter 7"})
	wRefused(t, err, 403, hubapi.CodeForbidden, "required", "org_admin")
	_, err = e.OrgAdmin.EraseIssue(wctx, iw.Head.ID, hubapi.EraseRequest{Reason: "   "})
	wRefused(t, err, 400, hubapi.CodeBadRequest, "field", "reason")
	res, err := e.OrgAdmin.EraseIssue(wctx, iw.Head.ID, hubapi.EraseRequest{Reason: "matter 7"})
	if err != nil || res.ErasedRevisions != 2 || res.ErasedComments != 1 {
		t.Fatalf("erase = %+v, %v; want 2 revisions, 1 comment", res, err)
	}
	return iw, c
}

func wEraseKeepsKeysAndBlanksTexts(t *testing.T, e WorkEnv) {
	iw, c := wErased(t, e)
	got, err := e.Member.Issue(wctx, iw.Head.ID)
	if err != nil || got.Head.Number != 1 || got.Head.Alias != "EP-1" || got.Head.HeadKey != "issue:"+iw.Head.ID+"@2" {
		t.Fatalf("erased head = %+v, %v; want number, alias and key kept", got.Head, err)
	}
	one, _ := e.Member.IssueRevisionAt(wctx, iw.Head.ID, 1)
	if one.Content.Title.Text != "" || one.ErasedAt == nil || one.Digest != iw.Revision.Digest {
		t.Errorf("revision 1 = %q erasedAt %v; want \"\", set, and the same digest", one.Content.Title.Text, one.ErasedAt)
	}
	page, _ := e.Member.Comments(wctx, iw.Head.ID, hubapi.PageRequest{})
	if len(page.Items) != 1 || page.Items[0].ID != c.ID || page.Items[0].Body.Text != "" || page.Items[0].ErasedAt == nil {
		t.Errorf("comments = %+v; want the one id kept, its body blank", page.Items)
	}
	from := wHead(t, e.Member)
	again, err := e.OrgAdmin.EraseIssue(wctx, iw.Head.ID, hubapi.EraseRequest{Reason: "matter 7"})
	if err != nil || again.ErasedRevisions != 0 || again.ErasedComments != 0 {
		t.Errorf("second erase = %+v, %v; want zeros", again, err)
	}
	if got := wKinds(t, e.Member, from, "issue.erased"); len(got) != 1 {
		t.Errorf("the second erase appended %d issue.erased, want 1: every attempt is audited", len(got))
	}
}

func wErasedTakesNoWrites(t *testing.T, e WorkEnv) {
	iw, _ := wErased(t, e)
	high := hubapi.PriorityHigh
	_, err := e.Member.PatchIssue(wctx, iw.Head.ID, hubapi.PatchIssueRequest{Priority: &high})
	wRefused(t, err, 409, hubapi.CodeConflict, "issueId", iw.Head.ID)
	_, err = wSay(e.Member, iw.Head.ID, "new words", nil, nil)
	wRefused(t, err, 409, hubapi.CodeConflict, "issueId", iw.Head.ID)
	_, err = e.Member.MoveIssueState(wctx, iw.Head.ID, hubapi.StateMove{To: hubapi.StateTodo, Cause: hubapi.CauseHand, Evidence: []string{}})
	wRefused(t, err, 409, hubapi.CodeConflict, "issueId", iw.Head.ID)
	if hits, _ := e.Member.SearchIssues(wctx, hubapi.IssueSearchQuery{Q: "secret"}); len(hits) != 0 {
		t.Errorf("search found %d erased issues", len(hits))
	}
}
```

```go
// WorkRowsW1 is W1's rows in ID order. An ID is never reused or renumbered;
// a retired row stays in the list as a t.Skip naming its replacement.
var WorkRowsW1 = []WorkRow{
	{"W-01", "numbers_gapless", wNumbersGapless},
	{"W-02", "viewer_cannot_write", wViewerCannotWrite},
	{"W-03", "key_is_the_alias", wKeyIsTheAlias},
	{"W-04", "stale_base_conflicts", wStaleBaseConflicts},
	{"W-05", "restore_is_a_new_revision", wRestoreIsANewRevision},
	{"W-06", "patch_writes_no_revision", wPatchWritesNoRevision},
	{"W-07", "hand_cannot_set_a_fact", wHandCannotSetAFact},
	{"W-08", "hand_done_of_a_chore_has_no_criteria", wHandDoneOfAChoreHasNoCriteria},
	{"W-09", "reopen_clears_the_proof_word", wReopenClearsTheProofWord},
	{"W-10", "relation_reads_the_inverse", wRelationReadsTheInverse},
	{"W-11", "relation_cycle_refused", wRelationCycleRefused},
	{"W-12", "relation_removed_from_the_other_end", wRelationRemovedFromTheOtherEnd},
	{"W-13", "label_names_are_the_teams", wLabelNamesAreTheTeams},
	{"W-14", "archived_label_is_unavailable", wArchivedLabelIsUnavailable},
	{"W-15", "policy_gates_the_triage_exit", wPolicyGatesTheTriageExit},
	{"W-16", "replies_are_one_level", wRepliesAreOneLevel},
	{"W-17", "text_anchors_count_utf16", wTextAnchorsCountUTF16},
	{"W-18", "reactions_are_idempotent", wReactionsAreIdempotent},
	{"W-19", "subscriptions_follow_triggers", wSubscriptionsFollowTriggers},
	{"W-20", "import_is_idempotent", wImportIsIdempotent},
	{"W-21", "import_is_all_or_nothing", wImportIsAllOrNothing},
	{"W-22", "search_reads_only_readable_titles", wSearchReadsOnlyReadableTitles},
	{"W-23", "attachments_are_this_projects_blobs", wAttachmentsAreThisProjectsBlobs},
	{"W-24", "erase_keeps_keys_and_blanks_texts", wEraseKeepsKeysAndBlanksTexts},
	{"W-25", "erased_takes_no_writes", wErasedTakesNoWrites},
}
```

That is the whole of `conformance/work.go`: the head, blocks 1 to 7, and this list.

#### `conformance/work_fake_test.go`, complete

H05's `fake_test.go` builds its `Env` inside `TestFakeConforms`'s closure
(`phase-h-hub-parts/task-05.md:89-120`), with no named builder to call. This file builds the same
world the same way, and adds the org admin, so no H05 file changes.

```go
package conformance_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubclient"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubclient/conformance"
)

// workFakeEnv is TestFakeConforms's world, plus an org admin in no team (the
// user hubclient's fake_test.go:1044 names orgBoss).
func workFakeEnv(t *testing.T) conformance.WorkEnv {
	ctx := context.Background()
	f := hubclient.NewFake()
	now := time.Date(2026, 9, 25, 4, 0, 0, 0, time.UTC)
	f.SetNow(func() time.Time { return now })
	f.SetReservationTTL(15 * time.Minute)
	admin := f.As(hubclient.FakeAda)
	team, err := admin.CreateTeam(ctx, hubapi.CreateTeamRequest{Name: "Work conformance"})
	if err != nil {
		t.Fatal(err)
	}
	member, viewer := f.As(hubclient.FakeGrace), f.As(hubclient.FakeLinus)
	for _, p := range []struct {
		who  *hubclient.Fake
		u    hubclient.FakeUser
		role hubapi.Role
	}{{member, hubclient.FakeGrace, hubapi.RoleMember}, {viewer, hubclient.FakeLinus, hubapi.RoleViewer}} {
		if _, err := admin.CreateInvite(ctx, team.ID, hubapi.CreateInviteRequest{Email: p.u.Email, Role: p.role}); err != nil {
			t.Fatal(err)
		}
		if _, err := p.who.Me(ctx); err != nil { // binds the invite
			t.Fatal(err)
		}
	}
	return conformance.WorkEnv{
		Env: conformance.Env{Admin: admin, Member: member, Viewer: viewer,
			Outsider: f.As(hubclient.FakeUser{ID: "u-00000000000000a4", DisplayName: "Out Sider", Email: "out@example.test"}),
			TeamID:   team.ID, Advance: func(d time.Duration) { now = now.Add(d) }, ReservationTTL: 15 * time.Minute},
		OrgAdmin: f.As(hubclient.FakeUser{ID: "u-00000000000000b1", DisplayName: "Org Boss",
			Email: "boss@example.test", OrgAdmin: true}),
	}
}

func TestWorkRowsOnTheFake(t *testing.T) { conformance.RunWork(t, workFakeEnv) }

// TestWorkRowIDsAreInOrder: W-01 upward, once each, so an ID names one rule
// for good and task 16's run lists the same names as this one.
func TestWorkRowIDsAreInOrder(t *testing.T) {
	for i, r := range conformance.WorkRowsW1 {
		if want := fmt.Sprintf("W-%02d", i+1); r.ID != want || r.Name == "" || r.Run == nil {
			t.Errorf("row %d = %s %q, want %s with a name and a body", i, r.ID, r.Name, want)
		}
	}
}
```

---

#### Tests first: the rows, each with the mutant it kills

The rows are written before the fake's bodies: against task 04's stub, W-01 to W-09 pass and
W-10 to W-25 fail with 501 `not_in_fake_yet` (step 2 pins that).

| Row | Asserts | Mutant it kills |
|---|---|---|
| W-01 `numbers_gapless` | 1, 2, 3 across a refused create | a number taken before validation |
| W-02 `viewer_cannot_write` | viewer 403 `required: member`; outsider 404 naming the id | a viewer who writes; a 403 that says it exists |
| W-03 `key_is_the_alias` | `EP-1`, then `PAY-1` with the same `headKey`; a taken key 409 | a stored alias; a key that changes the provenance key |
| W-04 `stale_base_conflicts` | base 1 after revision 2 → 409 `revision_conflict`, head still `v2` | last writer wins |
| W-05 `restore_is_a_new_revision` | revision 3, `restoredFrom` 1, revision 1's digest and time unchanged | an in-place restore |
| W-06 `patch_writes_no_revision` | `headRevision` 1 and exactly one `issue.updated` | a head write that revises |
| W-07 `hand_cannot_set_a_fact` | `started`, `in_review`, `landed` by hand → 422 with from, to, cause | a person asserting a fact |
| W-08 `hand_done_of_a_chore_has_no_criteria` | proof `no_criteria` | a hand-set done that reads proven |
| W-09 `reopen_clears_the_proof_word` | proof `""`, reason `""` after a reopen | a stale proof word on an open issue |
| W-10 `relation_reads_the_inverse` | the other end reads `blocked_by`; a repeat is one event | two stored rows; a repeat that appends |
| W-11 `relation_cycle_refused` | the back edge 422 `relation_cycle`; a self relation 400 | a two-issue block cycle |
| W-12 `relation_removed_from_the_other_end` | `duplicated_by` removed from B clears A; again 404 | a delete that leaves the inverse |
| W-13 `label_names_are_the_teams` | ` bug ` after `Bug` 409 with its id; member 403; a hex colour 400 | a case-sensitive name; a raw colour |
| W-14 `archived_label_is_unavailable` | 422 naming `labels[0]`; the list is empty | an archived label put on an issue |
| W-15 `policy_gates_the_triage_exit` | default backlog; triage then 422 `policy_refused`; cancel allowed | a policy that blocks declining |
| W-16 `replies_are_one_level` | a reply's reply and a reply's resolve → 422 | nested threads |
| W-17 `text_anchors_count_utf16` | end 4 accepted, 5 refused on `é😀x` | code-point or byte offsets |
| W-18 `reactions_are_idempotent` | on, on, off, off → two events; `:+1:` 400 | a second row per repeat; a shortcode |
| W-19 `subscriptions_follow_triggers` | create subscribes; manual survives a comment; a mention subscribes; one event | a trigger overwriting a person's choice |
| W-20 `import_is_idempotent` | 2 created, `nope` unmatched; re-run 2 existing, no event; triage, intake, imported; no subscription | a re-import that duplicates; a label created by import |
| W-21 `import_is_all_or_nothing` | `items[1].url` 400 and no issue | a half-written batch |
| W-22 `search_reads_only_readable_titles` | viewer finds it case-insensitively; outsider nothing; limit 51 400 | search before readability |
| W-23 `attachments_are_this_projects_blobs` | one event for two attaches; type and size the blob's; another project's blob 404; viewer 403 | attaching another project's file |
| W-24 `erase_keeps_keys_and_blanks_texts` | 403 then 400 then `2, 1`; number, alias, key, digest kept; texts `""`; second erase zeros and one more event | deleting rows; an unaudited retry |
| W-25 `erased_takes_no_writes` | patch, comment, move → 409 `conflict`; search finds nothing | new text on an erased record |

W-25's search clause is weak on the fake, where the title is read from the blanked revision; on
the Hub it is the check on task 14's denormalised `head_title`, which is where the leak would be.

#### Tests first: `fake_work_social_test.go`, complete, the fake-only tests

What no row can say, because it is the fake's own shape: no stub is left, the attach's project
check is `GetBlob`'s, the feed never carries the reason, an erased import stays erased. It is
`package hubclient` and reuses task 04's rig (`newWorkRig`, `chore`, `htext`, `eventsOf`) and
`fake_test.go`'s `bg`, `must` and `orgBoss` (`fake_test.go:1044` at `9f2fa31`). Two blocks.

```go
package hubclient

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
)

// codeOf is the Hub code of err, or "" for nil and for a non-Hub error.
func codeOf(err error) string {
	var he *Error
	if errors.As(err, &he) {
		return he.Problem.Error
	}
	return ""
}

// TestNoPartTwoMethodIsAStub: every one of the nineteen answers by its rule.
// Kills: a method left at fake_work_stub.go's 501 when the file goes.
func TestNoPartTwoMethodIsAStub(t *testing.T) {
	r := newWorkRig(t)
	is := must(r.grace.CreateIssue(bg, r.p.ID, chore("one")))(t).Head.ID
	c := must(r.grace.CreateComment(bg, is, hubapi.CreateCommentRequest{Body: htext("hi"), Mentions: []string{}}))(t)
	boss := r.f.As(orgBoss)
	calls := map[string]error{
		"AddRelation":    second(r.grace.AddRelation(bg, is, hubapi.RelationRequest{Kind: hubapi.RelationRelates, IssueID: "is-x"})),
		"RemoveRelation": second(r.grace.RemoveRelation(bg, is, hubapi.RelationRelates, "is-x")),
		"Labels":         second(r.grace.Labels(bg, r.team.ID)),
		"CreateLabel":    second(r.ada.CreateLabel(bg, r.team.ID, hubapi.CreateLabelRequest{Name: "Bug", Color: "fail"})),
		"ArchiveLabel":   second(r.ada.ArchiveLabel(bg, "lb-x")),
		"TeamPolicy":     second(r.grace.TeamPolicy(bg, r.team.ID)),
		"PutTeamPolicy":  second(r.ada.PutTeamPolicy(bg, r.team.ID, hubapi.PutTeamPolicyRequest{DefaultState: hubapi.StateTodo})),
		"Comments":       second(r.grace.Comments(bg, is, hubapi.PageRequest{})),
		"CreateComment":  second(r.grace.CreateComment(bg, is, hubapi.CreateCommentRequest{Body: htext("two"), Mentions: []string{}})),
		"React":          second(r.grace.React(bg, c.ID, hubapi.ReactRequest{Emoji: "👍", On: true})),
		"ResolveComment": second(r.grace.ResolveComment(bg, c.ID, hubapi.ResolveRequest{Resolved: true})),
		"Subscribe":      second(r.grace.Subscribe(bg, is, hubapi.SubscribeRequest{Subscribed: false})),
		"Subscriptions":  second(r.grace.Subscriptions(bg, hubapi.PageRequest{})),
		"SearchIssues":   second(r.grace.SearchIssues(bg, hubapi.IssueSearchQuery{Q: "one"})),
		"ImportBatch": second(r.grace.ImportBatch(bg, r.p.ID, hubapi.ImportBatch{Source: hubapi.ImportSourceCSV,
			Items: []hubapi.ImportItem{{ExternalID: "1", Title: "in"}}})),
		"AttachBlob":   second(r.grace.AttachBlob(bg, is, hubapi.AttachRequest{Digest: hubapi.DigestOf([]byte("x"))})),
		"Attachments":  second(r.grace.Attachments(bg, is)),
		"EraseComment": second(boss.EraseComment(bg, c.ID, hubapi.EraseRequest{Reason: "test"})),
		"EraseIssue":   second(boss.EraseIssue(bg, is, hubapi.EraseRequest{Reason: "test"})),
	}
	if len(calls) != 19 {
		t.Fatalf("%d methods called, want part 2's nineteen", len(calls))
	}
	for m, err := range calls {
		if codeOf(err) == "not_in_fake_yet" {
			t.Errorf("%s still answers the stub's 501", m)
		}
	}
}

// second drops a call's value and keeps its error.
func second[T any](_ T, err error) error { return err }
```

The calls that are refusals by rule (`is-x` unknown, `lb-x` unknown, a digest never put) count:
the test asks only that no answer is the stub's. `EraseIssue` runs last, after `EraseComment`, so
neither refuses on the other's account; Go evaluates the map literal's values in source order.

Block 2: the attach's two calls, the reason, the erased import, another team's label.

```go
// TestAttachChecksTheProjectThroughGetBlob: the project rule is GetBlob's.
// Kills: an attach that reads a blob by digest alone, across projects.
func TestAttachChecksTheProjectThroughGetBlob(t *testing.T) {
	r := newWorkRig(t)
	is := must(r.grace.CreateIssue(bg, r.p.ID, chore("one")))(t).Head.ID
	_, err := r.grace.AttachBlob(bg, is, hubapi.AttachRequest{Digest: hubapi.DigestOf([]byte("never put"))})
	calls := r.f.Calls()
	i := slices.IndexFunc(calls, func(c string) bool { return strings.HasPrefix(c, "AttachBlob(") })
	if codeOf(err) != hubapi.CodeBlobNotFound || i < 0 || i+1 >= len(calls) || !strings.HasPrefix(calls[i+1], "GetBlob("+r.p.ID) {
		t.Errorf("err %v, calls %v; want 404 blob_not_found from GetBlob on the issue's project", err, calls)
	}
}

// TestTheFeedNeverCarriesTheReason: the reason is checked and dropped.
// Kills: an erase event that broadcasts a client's matter number.
func TestTheFeedNeverCarriesTheReason(t *testing.T) {
	r := newWorkRig(t)
	is := must(r.grace.CreateIssue(bg, r.p.ID, chore("one")))(t).Head.ID
	must(r.f.As(orgBoss).EraseIssue(bg, is, hubapi.EraseRequest{Reason: "matter ZQX-14"}))(t)
	ev := eventsOf(t, r.grace, "issue.erased")
	if len(ev) != 1 || len(ev[0].Data) != 2 || ev[0].Data["erasedRevisions"] != int64(1) || ev[0].Data["erasedComments"] != int64(0) {
		t.Fatalf("issue.erased = %+v, want exactly the two counts", ev)
	}
	for _, e := range eventsOf(t, r.grace, "") {
		for k, v := range e.Data {
			if s, ok := v.(string); ok && strings.Contains(s, "ZQX-14") {
				t.Errorf("%s data.%s carries the reason", e.Kind, k)
			}
		}
	}
}

// TestAnErasedImportStaysErased: link and author name go, the external id
// stays, so a re-import answers existing. Kills: a re-import that re-creates
// the text an org admin erased (task 15's rule).
func TestAnErasedImportStaysErased(t *testing.T) {
	r := newWorkRig(t)
	batch := hubapi.ImportBatch{Source: hubapi.ImportSourceLinear, Items: []hubapi.ImportItem{{ExternalID: "ENG-7",
		URL: "https://linear.example/ENG-7", Title: "secret", AuthorName: "A. Person", Labels: []string{}}}}
	id := must(r.grace.ImportBatch(bg, r.p.ID, batch))(t).Created[0].IssueID
	must(r.f.As(orgBoss).EraseIssue(bg, id, hubapi.EraseRequest{Reason: "request 9"}))(t)
	x := must(r.grace.Issue(bg, id))(t).Head.External
	if x == nil || x.URL != "" || x.AuthorName != "" || x.ExternalID != "ENG-7" {
		t.Errorf("external = %+v, want the id kept and the words gone", x)
	}
	again := must(r.grace.ImportBatch(bg, r.p.ID, batch))(t)
	if len(again.Created) != 0 || len(again.Existing) != 1 || again.Existing[0].IssueID != id {
		t.Errorf("re-import = %+v, want the erased issue under existing", again)
	}
}

// TestAnotherTeamsLabelIsUnavailable. Kills: a label check by id alone.
func TestAnotherTeamsLabelIsUnavailable(t *testing.T) {
	r := newWorkRig(t)
	other := must(r.grace.CreateTeam(bg, hubapi.CreateTeamRequest{Name: "Billing"}))(t)
	l := must(r.grace.CreateLabel(bg, other.ID, hubapi.CreateLabelRequest{Name: "Bug", Color: "fail"}))(t)
	is := must(r.grace.CreateIssue(bg, r.p.ID, chore("one")))(t).Head.ID
	_, err := r.grace.PatchIssue(bg, is, hubapi.PatchIssueRequest{Labels: &[]string{l.ID}})
	var he *Error
	if !errors.As(err, &he) || he.Status != 422 || he.Problem.Error != "label_unavailable" || he.Problem.Details["field"] != "labels[0]" {
		t.Errorf("err = %v, want 422 label_unavailable naming labels[0]", err)
	}
}
```
