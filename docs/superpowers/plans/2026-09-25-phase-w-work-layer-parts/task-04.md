### Task 04: `hubclient.Work` and the fake, part 1: issues, revisions, numbers, the project key, states

**Slice:** W1. **Size:** 3 focused hours (an estimate, not a measurement). **Stream:** WH.
**dependsOn:** 03; Hub tasks H04-H05 (the interfaces, `Error`, `ErrUnavailable`, `NewFake()` and
its feed), sync-merged into `feat/work-layer` first (rule H1). **Seams:** defines **SW.2**, first
half: the whole W1 method set of `hubclient.Work`, and the fake's issue rules. SW.2 freezes at WS0
with task 05; after that, a change that is not an added method, field or route reopens task 05
and is announced (skeleton, *Start lines*).

Every Work-backed handler in the control plane (tasks 19, 21-25), the relay (task 16) and the
desktop's contract tests (task 37) are written against one Go interface and one in-memory Hub.
If the interface lagged the routes, each stream would invent its own method; if the fake's rules
were looser than the Hub's, a handler green on the fake would fail on the server. So this task
fixes the **whole** W1 method set now, and gives the fake part 1's rules exactly as tasks 08-10
build them on the server. Task 05 adds the rest of the rules and the conformance rows that hold
the fake and the Hub to one standard.

**Anchors, re-read on 2026-09-25 at `feat/hub` `9f2fa31`** (under
`control-plane/internal/hub/hubclient/`): `hubclient.go:52-64` (`Briefs`, the style: `ctx` first,
`hubapi` types in and out), `:63` (`Briefs.Resolve(ctx, keys)`), `:101-109` (`type Hub interface`),
`:111-121` (`Error`), `:146-149` (`_ Hub = (*Fake)(nil)`); `fake.go:56-71` (`fakeWorld`, one
`mu`), `:74-86` (`NewFake`), `:139-150` (`enter`), `:172-178` (`stamp`, `next`), `:184-197`
(`problem`, `badRequest`), `:200-223` (`forbidden`, `need`), `:239-262` (`pageLimit`, `pageOf`),
`:269-276` (`notInFakeYet`, 501 `not_in_fake_yet`); `fake_projects.go:42-48` (`fakeProject`),
`:72-79` (`projectOf`: the caller's role, or 404 `project_not_found`), `:81-84` (`canRead`),
`:108` (`userRef`). Under `hubapi/`: `canonical.go:282-296` (`FieldError{Field, Err}`),
`keys.go:32` (`ValidID`), `:110-116` (`Digest`, `DigestOf`), `envelope.go:394` (`FormatTime`),
`types.go:565-567` (`EraseRequest{Reason}`). Task 02's `IssueRevisionEnvelope`,
`IssueRevisionDigest`, `IssueKey`, `ValidProjectKey`, `DisplayAlias`, `PrefixIssue`; task 03's
`work.go` (every W1 record, request and enum, including `AttachRequest`, `IssueAttachment` and
`WorkEraseResponse`, which task 03 declared for this task).

**Not yet on any branch, so UNVERIFIED until merge:** Hub task H05's `fake_feed.go` (the feed
the fake appends to; its part, `phase-h-hub-parts/task-05.md:162-165`, fixes the behaviour, *"every
write … appends one event per changed record"*, and names no helper), and task 13's
`hubapi/workevents.go` (the Work event kinds and their data). Where this part names either, it
says so, and Step 1 takes the merged names.

Seven decisions the scope left open, made here.

**(a) `ResolveComment`, not `Resolve`.** The scope lists a `WorkIssues.Resolve`. `Hub` already
embeds `Briefs.Resolve(ctx, keys []string)` (`hubclient.go:63`), and Go refuses an interface that
embeds two methods of one name with different signatures, so `type Hub interface` would not
compile. The comment method is `ResolveComment(ctx, commentID, hubapi.ResolveRequest)`. Task 16's
part already uses this name (`task-16.md:936-937`).

**(b) `Attachments` is a method.** Task 07's route list has `GET /v1/issues/{issueId}/attachments`
and the scope's method list has no reader for it (task 16's gap 2). The set gains
`Attachments(ctx, issueID) ([]hubapi.IssueAttachment, error)`: task 03's type, not the
`hubapi.Attachment` task 16's part drafted, which exists nowhere.

**(c) Erase takes the Hub's own body and answers task 03's type.** `EraseIssue(ctx, issueID,
hubapi.EraseRequest) (hubapi.WorkEraseResponse, error)` and `EraseComment(ctx, commentID,
hubapi.EraseRequest) (hubapi.WorkEraseResponse, error)`: a reason is kept for every erase, as the
brief erase keeps one (`types.go:565`). Task 16's part drafted a bodyless erase answering a head;
it takes these at its Step 2 (`task-16.md:887`).

**(d) The stub has its own 501.** `fake.go:264-276`'s `notInFakeYet` is Hub part 1's placeholder
section, which Hub task H05 deletes. `fake_work_stub.go` declares `workNotInFakeYet` beside its
methods with the same answer, 501 `not_in_fake_yet`, so the file compiles whichever side of H05
it lands on and task 05 deletes it whole. The scope's `not_implemented` code would be a second
spelling of one idea; the fake's is kept.

**(e) The event kinds are task 13's (SW.5).** The scope drafted `issue.created`,
`issue.changed`, `issue.revised`, `issue.state`. Task 13's registry (`task-13.md:122-139`) spells
the head write `issue.updated` and adds `project.key_set`; this part uses task 13's five W1
issue kinds by constant, and their `data` shapes. Every fake write reaches the feed through one
function, `w.workEvent`, so Step 1's rename of H05's helper is a one-line edit.
Task 13 comes after this task, so the fake spells the kinds as its own unexported constants with
task 13's exact strings; task 05's conformance rows assert the kind strings, and task 16 runs
those rows against the real Hub, so a drift on either side fails a row.

**(f) The codes are the later tasks', spelled here as the fake's own constants.** Tasks 08, 09
and 10 declare `hubapi.CodeIssueNotFound`, `CodeProjectKeyTaken`, the revision conflict and
`CodeStateNotAllowed`, `CodeStateConflict`, `CodeEvidenceNotApplicable`. Declaring them in
`hubapi` here would make two declarations at their merge, so `fake_work.go` has unexported
`fk…` constants with the same strings (`task-08.md:171-172`, `task-09.md:113`,
`task-10.md:242-244`), and task 05's conformance rows assert the strings.

**(g) The move table is task 10's, drafted as the fake's own function until task 10 lands.**
Task 10 declares `hubapi.MoveAllowed(from, to, cause)` and says the fake calls it
(`task-10.md:20-22`, `:264-287`). This task comes first, so `fakeMoveAllowed` in `fake_work.go` is
that table line for line, over task 03's `MoveCause` names. Task 10's Step adds one edit to this
file: the body becomes `return hubapi.MoveAllowed(from, to, cause) == nil`. The evidence rule is
task 10's decisions 3-6 over what the fake can see: each cited record exists in the same project,
names a served `(briefKey, criterionId)`, is `settled`, was recorded at or after `landedAt`, and
has `commit` equal to the land commit. Task 03's `StateMove` has no `evidenceCommit` (task 10 adds
it), so here only the land commit counts.

**(h) Numbers are taken after every check.** A create that is refused, for any reason, consumes
no number, so *gapless* holds for refusals as well as for races: the counter moves in the same
locked section as the insert, after the last check.

**Files** (under `control-plane/internal/hub/hubclient/`):

| Path | Change |
|---|---|
| `work.go` | new: `type Work interface { WorkIssues }` and every W1 method of `WorkIssues` |
| `hubclient.go` ⚑ | one line: `type Hub interface` gains `Work` (the only edit to a Hub file) |
| `fake_work.go` | new: the fake's issue state, part 1's rules, `fakeMoveAllowed`, `w.workEvent` |
| `fake_work_stub.go` | new: part 2's methods answer 501 `not_in_fake_yet`; task 05 deletes it |
| `fake_work_test.go` | new: part 1's rules, each test with its mutant |

`fakeWorld` (`fake.go:56-71`) is Hub code, so it is **not** edited: the Work state hangs off one
pointer the fake creates on first use, `w.work`, declared in `fake_work.go` through a map keyed by
world (decision (i) below), so `fake.go` stays byte-identical.

**(i) The Work state lives beside the world, not in it.** `fake_work.go` declares
`var workWorlds sync.Map // *fakeWorld → *fakeWork` and `func (w *fakeWorld) wk() *fakeWork`,
which loads or stores one `*fakeWork` for that world. Every access is under `w.mu`, the fake's
one lock, like every other field; the `sync.Map` only finds the pointer. So no line of `fake.go`
or `NewFake` changes, H05's edits to `fake.go` cannot conflict with this task, and every view
(`As`) shares the one Work state because they share the world.

---

#### Interfaces, complete

`hubclient.go`'s one change, in `type Hub interface` (`hubclient.go:101-109`):

```go
type Hub interface {
	Projects
	Teams
	Briefs
	Designs
	Releases
	Evidence
	Feed
	Work // Phase W, task 04: the work layer (hubclient/work.go)
}
```

`var _ Hub = (*Fake)(nil)` (`hubclient.go:148`) then requires the fake to have every method below,
which `fake_work.go` (part 1) and `fake_work_stub.go` (part 2) give it. `var _ Hub = (*Client)(nil)`
(`:147`) requires the same of the HTTP client, which has none of them until task 16.

**(j) The client gets a stub file of its own.** `client_work_stub.go` gives `*Client` every Work
method, each answering `ErrNotConfigured` as H04's `client.go` stub does for every Hub method
(`client.go:11-16`, `:25-134`). It is its own file for the reason `client.go` is: H33 replaces
`client.go` file for file, and must not find Work methods in it. Task 16 deletes
`client_work_stub.go` in the same commit that adds `methods_work.go` (a gap for task 16's file
list, which does not name it).

| Path (continued) | Change |
|---|---|
| `client_work_stub.go` | new: `*Client`'s Work methods answer `ErrNotConfigured`; task 16 deletes it |

`work.go`, complete. Thirty methods: eleven whose rules are part 1's (this task, tasks 08-10 on
the server), nineteen whose rules are part 2's (task 05, tasks 11-15). Task 16's thirty
`WorkRoutes` rows (`task-16.md:131-162`) are these thirty methods.

```go
package hubclient

import (
	"context"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
)

// Work is the work layer (Phase W). Hub embeds it. W2-W5 append WorkPlan
// (task 72), WorkAgents (task 99), WorkRoadmap (task 114) and WorkIntake
// (task 143), each an added interface: SW.2 grows, it never changes.
type Work interface {
	WorkIssues
}

// WorkIssues is every W1 method, in the style of Briefs: ctx first, hubapi
// types in and out, an *Error for every refusal the Hub answered.
type WorkIssues interface {
	// ── part 1: task 04's fake, tasks 08-10's server ──

	// SetProjectKey sets or changes a project's key (team admin; ValidProjectKey;
	// unique in the team). A change changes every alias and no key.
	SetProjectKey(ctx context.Context, projectID, key string) (hubapi.ProjectKey, error)
	// CreateIssue writes the head and revision 1 in one write, and takes the
	// project's next number (gapless from 1). Member or admin.
	CreateIssue(ctx context.Context, projectID string, req hubapi.CreateIssueRequest) (hubapi.IssueWithRevision, error)
	// Issue is the head and its head revision.
	Issue(ctx context.Context, issueID string) (hubapi.IssueWithRevision, error)
	// IssueByNumber is how a typed alias ("EP-142") becomes an issue: the caller
	// knows the project from where it was typed; no alias parser exists.
	IssueByNumber(ctx context.Context, projectID string, number int) (hubapi.IssueWithRevision, error)
	// Issues lists heads, newest number first, filtered by q.
	Issues(ctx context.Context, projectID string, q hubapi.IssueQuery, page hubapi.PageRequest) (hubapi.Page[hubapi.IssueHead], error)
	// PatchIssue changes head fields only. It never writes a revision.
	PatchIssue(ctx context.Context, issueID string, req hubapi.PatchIssueRequest) (hubapi.IssueHead, error)
	// WriteIssueRevision writes revision baseRevision+1, or 409 when the head
	// has moved past baseRevision, writing nothing.
	WriteIssueRevision(ctx context.Context, issueID string, req hubapi.WriteIssueRevisionRequest) (hubapi.IssueRevision, error)
	// IssueRevisions lists revisions oldest first, content included.
	IssueRevisions(ctx context.Context, issueID string, page hubapi.PageRequest) (hubapi.Page[hubapi.IssueRevision], error)
	// IssueRevisionAt is one revision; an erased one keeps its digest.
	IssueRevisionAt(ctx context.Context, issueID string, rev int) (hubapi.IssueRevision, error)
	// RestoreIssueRevision writes a NEW revision copying req.FromRevision's
	// content, with restoredFrom set. The path's {rev} is req.FromRevision.
	RestoreIssueRevision(ctx context.Context, issueID string, req hubapi.RestoreRevisionRequest) (hubapi.IssueRevision, error)
	// MoveIssueState is SW.4: hand moves among the non-fact states and a
	// hand-set done with a reason; each fact state only with its fact cause.
	MoveIssueState(ctx context.Context, issueID string, move hubapi.StateMove) (hubapi.IssueHead, error)

	// ── part 2: task 05's fake, tasks 11-15's server ──

	AddRelation(ctx context.Context, issueID string, req hubapi.RelationRequest) (hubapi.IssueHead, error)
	RemoveRelation(ctx context.Context, issueID string, kind hubapi.RelationKind, otherID string) (hubapi.IssueHead, error)
	Labels(ctx context.Context, teamID string) ([]hubapi.Label, error)
	CreateLabel(ctx context.Context, teamID string, req hubapi.CreateLabelRequest) (hubapi.Label, error)
	ArchiveLabel(ctx context.Context, labelID string) (hubapi.Label, error)
	TeamPolicy(ctx context.Context, teamID string) (hubapi.TeamPolicy, error)
	PutTeamPolicy(ctx context.Context, teamID string, req hubapi.PutTeamPolicyRequest) (hubapi.TeamPolicy, error)
	Comments(ctx context.Context, issueID string, page hubapi.PageRequest) (hubapi.Page[hubapi.Comment], error)
	// CreateComment appends; there is no edit. A correction is a new comment.
	CreateComment(ctx context.Context, issueID string, req hubapi.CreateCommentRequest) (hubapi.Comment, error)
	React(ctx context.Context, commentID string, req hubapi.ReactRequest) (hubapi.Comment, error)
	// ResolveComment, not Resolve: Briefs.Resolve(ctx, keys) holds that name in Hub.
	ResolveComment(ctx context.Context, commentID string, req hubapi.ResolveRequest) (hubapi.Comment, error)
	Subscribe(ctx context.Context, issueID string, req hubapi.SubscribeRequest) (hubapi.Subscription, error)
	// Subscriptions is the caller's own, newest first.
	Subscriptions(ctx context.Context, page hubapi.PageRequest) (hubapi.Page[hubapi.Subscription], error)
	// SearchIssues matches titles in the caller's readable projects, newest first.
	SearchIssues(ctx context.Context, q hubapi.IssueSearchQuery) ([]hubapi.IssueHead, error)
	// ImportBatch is idempotent by (source, externalId).
	ImportBatch(ctx context.Context, projectID string, batch hubapi.ImportBatch) (hubapi.ImportResult, error)
	// AttachBlob names a blob already put to the issue's project (Designs.PutBlob).
	AttachBlob(ctx context.Context, issueID string, req hubapi.AttachRequest) (hubapi.IssueAttachment, error)
	Attachments(ctx context.Context, issueID string) ([]hubapi.IssueAttachment, error)
	// EraseIssue and EraseComment are org admin only. Ids, keys, digests and
	// revision numbers stay; the texts read as erased.
	EraseIssue(ctx context.Context, issueID string, req hubapi.EraseRequest) (hubapi.WorkEraseResponse, error)
	EraseComment(ctx context.Context, commentID string, req hubapi.EraseRequest) (hubapi.WorkEraseResponse, error)
}
```

#### `fake_work.go`, complete

Five blocks; the file is their concatenation. Block 1 is the state and the helpers.

```go
package hubclient

import (
	"context"
	"errors"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/ids"
)

// Phase W, part 1: issues, revisions, numbers, project keys and state moves.
// The rules are tasks 08-10's; task 05's conformance rows hold the fake and the
// Hub to them. Part 2 is task 05's fake_work_social.go and fake_work_intake.go.

// Tasks 08, 10 and 13 declare these in hubapi after this task. Until then the
// fake spells them with the same strings (task 04's decisions (e) and (f)).
const (
	fkIssueNotFound         = "issue_not_found"         // 404, details.issueId
	fkProjectKeyTaken       = "project_key_taken"       // 409, details.key, details.projectId
	fkStateNotAllowed       = "state_not_allowed"       // 422, details.from, .to, .cause
	fkStateConflict         = "state_conflict"          // 409, details.landCommit
	fkEvidenceNotApplicable = "evidence_not_applicable" // 422, details.field, .why
	// Task 08 gives this refusal a status (422) and a field, and no code: a gap
	// it closes by declaring the constant; until then, this spelling.
	fkNotTiedReasonRequired = "not_tied_reason_required" // 422, details.field

	fkEventIssueCreated  = "issue.created"
	fkEventIssueUpdated  = "issue.updated"
	fkEventIssueRevised  = "issue.revised"
	fkEventIssueState    = "issue.state"
	fkEventProjectKeySet = "project.key_set"
)

var landCommitRe = regexp.MustCompile(`^[0-9a-f]{40}$`)

// fakeWork is one world's Work state. Every field is guarded by the world's mu.
type fakeWork struct {
	issues  map[string]*fakeIssue         // by id
	numbers map[string]int                // by project id: the last number taken
	keys    map[string]*hubapi.ProjectKey // by project id
	cited   map[string]string             // evidence id → the issue citing it now
}

type fakeIssue struct {
	head      hubapi.IssueHead // Alias, HeadKey, ThreadState and the users are made at read
	teamID    string
	assignee  string // a user id, or ""
	createdBy string // a user id
	notTied   hubapi.NotTiedReason
	handSet   bool   // the done was set by hand
	landedAt  string // the landed move's time; "" before
	startedBy string // who made the started move; "" before
	citations []string
	revisions []*hubapi.IssueRevision // revisions[i] is revision i+1, immutable
	seq       int64
}

func (is *fakeIssue) order() int64 { return is.seq }

func (is *fakeIssue) headRev() *hubapi.IssueRevision { return is.revisions[len(is.revisions)-1] }

// workWorlds finds each world's Work state, so fake.go gains no field
// (decision (i)). A fake lives as long as the test process.
var workWorlds sync.Map // *fakeWorld → *fakeWork

// wk is w's Work state, made on first use. Call it with w.mu held.
func (w *fakeWorld) wk() *fakeWork {
	v, _ := workWorlds.LoadOrStore(w, &fakeWork{issues: map[string]*fakeIssue{},
		numbers: map[string]int{}, keys: map[string]*hubapi.ProjectKey{}, cited: map[string]string{}})
	return v.(*fakeWork)
}

// workEvent appends one event to the feed. It is the fake's only call of H05's
// feed helper (UNVERIFIED name, appendEvent: Step 1 takes the merged one).
func (w *fakeWorld) workEvent(me, kind, teamID, projectID, subject string, data map[string]any) {
	e := hubapi.Event{Kind: kind, TeamID: &teamID, Subject: subject, Actor: w.userRef(me), Data: data}
	if projectID != "" {
		e.ProjectID = &projectID
	}
	w.appendEvent(e) // H05 assigns Seq and At
}
```

Block 2: the lookups, the rendering, and the checks a create and a revision share.

```go
// issueOf is issueID's issue and the caller's role in its team. Unknown,
// malformed and unreadable are one 404 (task 08's decision 6).
func (w *fakeWorld) issueOf(issueID, me string) (*fakeIssue, hubapi.Role, error) {
	if is := w.wk().issues[issueID]; is != nil {
		if m := w.teams[is.teamID].members[me]; m != nil {
			return is, m.role, nil
		}
	}
	return nil, "", problem(404, fkIssueNotFound, "There is no such issue that you can see.", "issueId", issueID)
}

// renderIssue is the head as the Hub answers it: the alias from the key as it
// is now, the head key, the thread state, the users as they are now.
func (w *fakeWorld) renderIssue(is *fakeIssue) hubapi.IssueHead {
	h := is.head
	h.Alias = ""
	if k := w.wk().keys[h.ProjectID]; k != nil {
		h.Alias = hubapi.DisplayAlias(k.Key, h.Number)
	}
	h.HeadKey = hubapi.IssueKey{IssueID: h.ID, Revision: h.HeadRevision}.String()
	h.ThreadState = w.threadStateOf(is)
	h.Assignee = w.userRef(is.assignee)
	h.CreatedBy = *w.userRef(is.createdBy)
	h.Labels = append([]string{}, h.Labels...)
	h.Relations = append([]hubapi.Relation{}, h.Relations...)
	if h.External != nil {
		x := *h.External
		h.External = &x
	}
	return h
}

func (w *fakeWorld) withRevision(is *fakeIssue) hubapi.IssueWithRevision {
	return hubapi.IssueWithRevision{Head: w.renderIssue(is), Revision: *is.headRev()}
}

// threadStateOf is task 08's: on_thread when any served brief is a branch
// brief, project_level when every one is the project's, else not_tied.
func (w *fakeWorld) threadStateOf(is *fakeIssue) hubapi.ThreadState {
	serves := is.headRev().Content.Serves
	if len(serves) == 0 {
		return hubapi.ThreadState{Kind: hubapi.ThreadNotTied, Reason: is.notTied}
	}
	for _, s := range serves {
		k, _ := hubapi.ParseBriefKey(s.BriefKey) // checked when written
		if b := w.briefs[k.BriefID]; b != nil && b.scope == hubapi.ScopeBranch {
			return hubapi.ThreadState{Kind: hubapi.ThreadOnThread}
		}
	}
	return hubapi.ThreadState{Kind: hubapi.ThreadProjectLevel}
}

// checkServes is task 08's checkServes: each key parses and names a revision,
// not erased, of a brief of this project. field is "serves" or "content.serves".
func (w *fakeWorld) checkServes(projectID, field string, serves []hubapi.Serve) error {
	for i, s := range serves {
		k, err := hubapi.ParseBriefKey(s.BriefKey)
		b := w.briefs[k.BriefID]
		if err != nil || b == nil || b.projectID != projectID || k.Revision > len(b.revisions) ||
			b.revisions[k.Revision-1].erasedAt != "" {
			return problem(422, hubapi.CodeKeyUnresolvable, "A served brief revision is not one of this project's.",
				"field", field+"["+strconv.Itoa(i)+"]")
		}
	}
	return nil
}

// contentError is 400 naming the field hubapi's validation named, under
// prefix: "" on a create (its body has no content), "content" on a revision.
func contentError(prefix string, err error) error {
	field := prefix
	var fe *hubapi.FieldError
	if errors.As(err, &fe) {
		field = strings.TrimPrefix(prefix+"."+fe.Field, ".")
	}
	return badRequest(field, err.Error())
}

// assignable is task 08's decision 5: a u- id of a member or admin of the team.
// A viewer cannot act, so cannot be assigned; an agent id is never a u- id.
func (w *fakeWorld) assignable(teamID, userID string) bool {
	m := w.teams[teamID].members[userID]
	return hubapi.ValidID(hubapi.PrefixUser, userID) && m != nil && rank(m.role) >= rank(hubapi.RoleMember)
}
```

Block 3: the project key and the create.

```go
// SetProjectKey is task 08's decision 4: team admin, ValidProjectKey, unique in
// the team; the same key again is no write and no event.
func (f *Fake) SetProjectKey(_ context.Context, projectID, key string) (hubapi.ProjectKey, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("SetProjectKey", projectID, key)
	if err != nil {
		return hubapi.ProjectKey{}, err
	}
	p, role, err := w.projectOf(projectID, me)
	if err != nil {
		return hubapi.ProjectKey{}, err
	}
	if err := need(role, hubapi.RoleAdmin); err != nil {
		return hubapi.ProjectKey{}, err
	}
	if !hubapi.ValidProjectKey(key) {
		return hubapi.ProjectKey{}, badRequest("key", "A project key is 2 to 5 capital letters.")
	}
	wk := w.wk()
	for id, k := range wk.keys {
		if id != projectID && k.Key == key && w.projects[id].teamID == p.teamID {
			return hubapi.ProjectKey{}, problem(409, fkProjectKeyTaken,
				"Another project in this team has that key.", "key", key, "projectId", id)
		}
	}
	prev := ""
	if old := wk.keys[projectID]; old != nil {
		if old.Key == key {
			return *old, nil
		}
		prev = old.Key
	}
	k := &hubapi.ProjectKey{ProjectID: projectID, Key: key, SetAt: w.stamp(), SetBy: *w.userRef(me)}
	wk.keys[projectID] = k
	w.workEvent(me, fkEventProjectKeySet, p.teamID, projectID, projectID,
		map[string]any{"key": key, "previousKey": prev})
	return *k, nil
}

// CreateIssue writes the head and revision 1 as one record and one event, and
// takes the next number after the last check (decision (h)).
func (f *Fake) CreateIssue(_ context.Context, projectID string, req hubapi.CreateIssueRequest) (hubapi.IssueWithRevision, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	var zero hubapi.IssueWithRevision
	me, err := f.enter("CreateIssue", projectID, req)
	if err != nil {
		return zero, err
	}
	p, role, err := w.projectOf(projectID, me)
	if err != nil {
		return zero, err
	}
	if err := need(role, hubapi.RoleMember); err != nil {
		return zero, err
	}
	content := hubapi.IssueRevisionContent{Title: req.Title, Description: req.Description,
		Serves: req.Serves, Implements: req.Implements}
	if err := hubapi.ValidateIssueRevisionContent(content); err != nil {
		return zero, contentError("", err)
	}
	if err := w.checkServes(projectID, "serves", req.Serves); err != nil {
		return zero, err
	}
	state, priority := req.State, req.Priority
	if state == "" {
		state = w.defaultState(p.teamID)
	}
	if priority == "" {
		priority = hubapi.PriorityNone
	}
	switch {
	case !hubapi.ValidInitialState(state):
		return zero, badRequest("state", "A new issue starts in triage, backlog or todo.")
	case !hubapi.ValidPriority(priority):
		return zero, badRequest("priority", "That is not a priority.")
	case req.NotTiedReason != "" && !hubapi.ValidNotTiedReason(req.NotTiedReason):
		return zero, badRequest("notTiedReason", "That is not a reason.")
	case len(req.Serves) == 0 && req.NotTiedReason == "":
		return zero, problem(422, fkNotTiedReasonRequired,
			"An issue that serves no criterion says why: chore, spike, imported or support.", "field", "notTiedReason")
	case req.AssigneeID != nil && !w.assignable(p.teamID, *req.AssigneeID):
		return zero, badRequest("assigneeId", "An assignee is a member of this team.")
	}
	if err := w.checkIssueLabels(p.teamID, "labels", req.Labels); err != nil {
		return zero, err
	}
	id := ids.Wide(hubapi.PrefixIssue)
	env := hubapi.IssueRevisionEnvelope{IssueID: id, Revision: 1, ProjectID: projectID, Content: content}
	digest, _, err := hubapi.IssueRevisionDigest(env)
	if err != nil { // past MaxEnvelopeBytes: still before the number is taken
		return zero, contentError("", err)
	}
	wk := w.wk()
	wk.numbers[projectID]++ // the last check has passed: decision (h)
	now := w.stamp()
	is := &fakeIssue{teamID: p.teamID, createdBy: me, notTied: req.NotTiedReason, seq: w.next(),
		head: hubapi.IssueHead{ID: id, ProjectID: projectID, Number: wk.numbers[projectID],
			State: state, Priority: priority, Labels: slices.Clone(req.Labels), HeadRevision: 1,
			Source: hubapi.SourcePerson, CreatedAt: now, UpdatedAt: now}}
	if req.AssigneeID != nil {
		is.assignee = *req.AssigneeID
	}
	is.revisions = []*hubapi.IssueRevision{{IssueID: id, Revision: 1,
		Key: hubapi.IssueKey{IssueID: id, Revision: 1}.String(), Digest: digest, ProjectID: projectID,
		Content: content, WrittenAt: now, WrittenBy: *w.userRef(me)}}
	wk.issues[id] = is
	w.workEvent(me, fkEventIssueCreated, p.teamID, projectID, id, map[string]any{
		"number": int64(is.head.Number), "key": is.revisions[0].Key, "digest": digest.String(),
		"state": string(state), "source": string(hubapi.SourcePerson)})
	w.subscribeOn(is, me, hubapi.SubscribeCreate)
	if is.assignee != "" && is.assignee != me {
		w.subscribeOn(is, is.assignee, hubapi.SubscribeAssign)
	}
	return w.withRevision(is), nil
}
```

`checkIssueLabels`, `defaultState` and `subscribeOn` are part 2's rules. In this task they are
`fake_work_stub.go`'s part-1 bodies (labels must be `[]`, as task 08's decision 8; the default
state is `triage`; no subscription is kept); task 05 deletes that file and gives each its rule in
`fake_work_social.go`, so no line of this block changes. `source` is `person` for every
`CreateIssue`: task 08 names no other source for this route, and `intake` is `ImportBatch`'s
(task 05, task 15). Whether an Orchestrator's create reads `orchestrator` is task 19's to ask of
task 08 (UNVERIFIED; a gap below).

Block 4: the reads and the head write.

```go
func (f *Fake) Issue(_ context.Context, issueID string) (hubapi.IssueWithRevision, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("Issue", issueID)
	if err != nil {
		return hubapi.IssueWithRevision{}, err
	}
	is, _, err := w.issueOf(issueID, me)
	if err != nil {
		return hubapi.IssueWithRevision{}, err
	}
	return w.withRevision(is), nil
}

// IssueByNumber answers the same 404 for a number never taken and a project
// the caller cannot read, as the Hub does.
func (f *Fake) IssueByNumber(_ context.Context, projectID string, number int) (hubapi.IssueWithRevision, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("IssueByNumber", projectID, number)
	if err != nil {
		return hubapi.IssueWithRevision{}, err
	}
	if number < 1 {
		return hubapi.IssueWithRevision{}, badRequest("number", "An issue number is 1 or more.")
	}
	if _, _, err := w.projectOf(projectID, me); err != nil {
		return hubapi.IssueWithRevision{}, err
	}
	for _, is := range w.wk().issues {
		if is.head.ProjectID == projectID && is.head.Number == number {
			return w.withRevision(is), nil
		}
	}
	return hubapi.IssueWithRevision{}, problem(404, fkIssueNotFound, "There is no such issue that you can see.",
		"number", strconv.Itoa(number))
}

// Issues is task 08's list: number descending; after is the last number seen.
func (f *Fake) Issues(_ context.Context, projectID string, q hubapi.IssueQuery, page hubapi.PageRequest) (hubapi.Page[hubapi.IssueHead], error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	var zero hubapi.Page[hubapi.IssueHead]
	me, err := f.enter("Issues", projectID, q, page)
	if err != nil {
		return zero, err
	}
	if _, _, err := w.projectOf(projectID, me); err != nil {
		return zero, err
	}
	limit, err := pageLimit(page.Limit)
	if err != nil {
		return zero, err
	}
	after := 0
	if page.After != "" {
		if after, err = strconv.Atoi(page.After); err != nil || after < 1 {
			return zero, badRequest("after", "A cursor is a number the list gave you.")
		}
	}
	for i, s := range q.States {
		if !hubapi.ValidIssueState(s) {
			return zero, badRequest("state["+strconv.Itoa(i)+"]", "That is not a state.")
		}
	}
	var rows []*fakeIssue
	for _, is := range w.wk().issues {
		switch {
		case is.head.ProjectID != projectID, after > 0 && is.head.Number >= after,
			len(q.States) > 0 && !slices.Contains(q.States, is.head.State),
			q.Assignee == "none" && is.assignee != "",
			q.Assignee != "" && q.Assignee != "none" && is.assignee != q.Assignee,
			q.Label != "" && !slices.Contains(is.head.Labels, q.Label):
			continue
		}
		rows = append(rows, is)
	}
	slices.SortFunc(rows, func(a, b *fakeIssue) int { return b.head.Number - a.head.Number })
	return pageOf(rows, limit, func(is *fakeIssue) string { return strconv.Itoa(is.head.Number) },
		w.renderIssue), nil
}

// PatchIssue writes the head only: never a revision (04 §2.4). One event
// names every field that changed; a patch that changes nothing writes nothing.
func (f *Fake) PatchIssue(_ context.Context, issueID string, req hubapi.PatchIssueRequest) (hubapi.IssueHead, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("PatchIssue", issueID, req)
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
	setA, assignee, err := req.AssigneeChange()
	switch {
	case err != nil:
		return hubapi.IssueHead{}, badRequest("assigneeId", "An assignee is a user id or null.")
	case req.Priority != nil && !hubapi.ValidPriority(*req.Priority):
		return hubapi.IssueHead{}, badRequest("priority", "That is not a priority.")
	case req.NotTiedReason != nil && !hubapi.ValidNotTiedReason(*req.NotTiedReason):
		return hubapi.IssueHead{}, badRequest("notTiedReason", "That is not a reason.")
	case setA && assignee != "" && !w.assignable(is.teamID, assignee):
		return hubapi.IssueHead{}, badRequest("assigneeId", "An assignee is a member of this team.")
	}
	if req.Labels != nil {
		if err := w.checkIssueLabels(is.teamID, "labels", *req.Labels); err != nil {
			return hubapi.IssueHead{}, err
		}
	}
```
