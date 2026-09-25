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
