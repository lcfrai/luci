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

Seven blocks; the file is their concatenation. Block 1 is the state and the helpers.

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
	citations []fakeCitation // at the current landing; cleared by a reopen
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
state is `backlog`, task 11's `DefaultTeamPolicy` for a team with no policy; no subscription is
kept); task 05 deletes that file and gives each its rule in
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
	if err := w.erasedWrite(is); err != nil {
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
	var fields []string
	if req.Priority != nil && *req.Priority != is.head.Priority {
		is.head.Priority, fields = *req.Priority, append(fields, "priority")
	}
	if setA && assignee != is.assignee {
		is.assignee, fields = assignee, append(fields, "assigneeId")
	}
	if req.Labels != nil && !slices.Equal(*req.Labels, is.head.Labels) {
		is.head.Labels, fields = slices.Clone(*req.Labels), append(fields, "labels")
	}
	if req.NotTiedReason != nil && *req.NotTiedReason != is.notTied {
		is.notTied, fields = *req.NotTiedReason, append(fields, "notTiedReason")
	}
	if len(fields) == 0 {
		return w.renderIssue(is), nil
	}
	is.head.UpdatedAt = w.stamp() // HeadRevision is untouched: no revision per head write
	w.workEvent(me, fkEventIssueUpdated, is.teamID, is.head.ProjectID, is.head.ID,
		map[string]any{"fields": strings.Join(fields, ",")})
	if slices.Contains(fields, "assigneeId") && assignee != "" && assignee != me {
		w.subscribeOn(is, assignee, hubapi.SubscribeAssign)
	}
	return w.renderIssue(is), nil
}
```

Block 5: revisions. A write and a restore both go through `appendRevision`, so the digest is
always `IssueRevisionDigest` of exactly what is stored.

```go
// appendRevision stores revision head+1 of content, digested as stored, and
// emits issue.revised. from is the restored revision, or 0.
func (w *fakeWorld) appendRevision(me string, is *fakeIssue, content hubapi.IssueRevisionContent, from int) (*hubapi.IssueRevision, error) {
	n := is.head.HeadRevision + 1
	env := hubapi.IssueRevisionEnvelope{IssueID: is.head.ID, Revision: n, ProjectID: is.head.ProjectID, Content: content}
	digest, _, err := hubapi.IssueRevisionDigest(env)
	if err != nil {
		return nil, contentError("content", err)
	}
	rev := &hubapi.IssueRevision{IssueID: is.head.ID, Revision: n,
		Key: hubapi.IssueKey{IssueID: is.head.ID, Revision: n}.String(), Digest: digest,
		ProjectID: is.head.ProjectID, Content: content, WrittenAt: w.stamp(), WrittenBy: *w.userRef(me)}
	if from > 0 {
		rev.RestoredFrom = &from
	}
	is.revisions = append(is.revisions, rev)
	is.head.HeadRevision, is.head.UpdatedAt = n, rev.WrittenAt
	w.workEvent(me, fkEventIssueRevised, is.teamID, is.head.ProjectID, is.head.ID, map[string]any{
		"key": rev.Key, "revision": int64(n), "digest": digest.String(),
		"servesCount": int64(len(content.Serves)), "restoredFrom": int64(from)})
	return rev, nil
}

// WriteIssueRevision is optimistic: a base that is not the head is 409 and
// writes nothing (task 09's decision 1).
func (f *Fake) WriteIssueRevision(_ context.Context, issueID string, req hubapi.WriteIssueRevisionRequest) (hubapi.IssueRevision, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("WriteIssueRevision", issueID, req)
	if err != nil {
		return hubapi.IssueRevision{}, err
	}
	is, role, err := w.issueOf(issueID, me)
	if err != nil {
		return hubapi.IssueRevision{}, err
	}
	if err := need(role, hubapi.RoleMember); err != nil {
		return hubapi.IssueRevision{}, err
	}
	if err := w.erasedWrite(is); err != nil {
		return hubapi.IssueRevision{}, err
	}
	if req.BaseRevision != is.head.HeadRevision {
		return hubapi.IssueRevision{}, problem(409, hubapi.CodeRevisionConflict,
			"Someone saved this issue after you opened it.", "headRevision", strconv.Itoa(is.head.HeadRevision))
	}
	if err := hubapi.ValidateIssueRevisionContent(req.Content); err != nil {
		return hubapi.IssueRevision{}, contentError("content", err)
	}
	if err := w.checkServes(is.head.ProjectID, "content.serves", req.Content.Serves); err != nil {
		return hubapi.IssueRevision{}, err
	}
	if len(req.Content.Serves) == 0 && is.notTied == "" {
		return hubapi.IssueRevision{}, problem(422, fkNotTiedReasonRequired,
			"Say why this issue serves no criterion before removing the last one.", "field", "content.serves")
	}
	rev, err := w.appendRevision(me, is, req.Content, 0)
	if err != nil {
		return hubapi.IssueRevision{}, err
	}
	return *rev, nil
}

// IssueRevisions is oldest first; after is the last revision number seen.
func (f *Fake) IssueRevisions(_ context.Context, issueID string, page hubapi.PageRequest) (hubapi.Page[hubapi.IssueRevision], error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	var zero hubapi.Page[hubapi.IssueRevision]
	me, err := f.enter("IssueRevisions", issueID, page)
	if err != nil {
		return zero, err
	}
	is, _, err := w.issueOf(issueID, me)
	if err != nil {
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
	rows := is.revisions[min(after, len(is.revisions)):]
	return pageOf(rows, limit, func(r *hubapi.IssueRevision) string { return strconv.Itoa(r.Revision) },
		func(r *hubapi.IssueRevision) hubapi.IssueRevision { return *r }), nil
}

// IssueRevisionAt answers issue_not_found for a revision that does not exist,
// as task 09's route does.
func (f *Fake) IssueRevisionAt(_ context.Context, issueID string, rev int) (hubapi.IssueRevision, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("IssueRevisionAt", issueID, rev)
	if err != nil {
		return hubapi.IssueRevision{}, err
	}
	is, _, err := w.issueOf(issueID, me)
	if err != nil {
		return hubapi.IssueRevision{}, err
	}
	if rev < 1 || rev > len(is.revisions) {
		return hubapi.IssueRevision{}, problem(404, fkIssueNotFound, "There is no such issue revision that you can see.",
			"issueId", issueID)
	}
	return *is.revisions[rev-1], nil
}

// RestoreIssueRevision writes a NEW revision with FromRevision's content and
// restoredFrom set; the old revision is never touched (task 09's decision 2).
// Task 09 adds baseRevision's 409 here with its field (decision 3).
func (f *Fake) RestoreIssueRevision(_ context.Context, issueID string, req hubapi.RestoreRevisionRequest) (hubapi.IssueRevision, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("RestoreIssueRevision", issueID, req)
	if err != nil {
		return hubapi.IssueRevision{}, err
	}
	is, role, err := w.issueOf(issueID, me)
	if err != nil {
		return hubapi.IssueRevision{}, err
	}
	if err := need(role, hubapi.RoleMember); err != nil {
		return hubapi.IssueRevision{}, err
	}
	if err := w.erasedWrite(is); err != nil {
		return hubapi.IssueRevision{}, err
	}
	from := req.FromRevision
	if from < 1 || from > len(is.revisions) {
		return hubapi.IssueRevision{}, problem(404, fkIssueNotFound, "There is no such issue revision that you can see.",
			"issueId", issueID)
	}
	old := is.revisions[from-1]
	if old.ErasedAt != nil {
		return hubapi.IssueRevision{}, problem(409, "revision_erased", "That revision was erased.", "revision", strconv.Itoa(from))
	}
	rev, err := w.appendRevision(me, is, old.Content, from)
	if err != nil {
		return hubapi.IssueRevision{}, err
	}
	return *rev, nil
}
```

Block 6: the move table and the evidence clauses (decision (g)).

```go
// fakeMoveAllowed is task 10's hubapi.MoveAllowed table (task-10.md:267-287),
// line for line. Task 10's step replaces the body with a call to it.
func fakeMoveAllowed(from, to hubapi.IssueState, cause hubapi.MoveCause) bool {
	in := func(s hubapi.IssueState, set ...hubapi.IssueState) bool { return slices.Contains(set, s) }
	switch cause {
	case hubapi.CauseHand:
		return in(to, hubapi.StateTriage, hubapi.StateBacklog, hubapi.StateTodo, hubapi.StateCancelled,
			hubapi.StateDuplicate, hubapi.StateDone)
	case hubapi.CauseLaneBound:
		return to == hubapi.StateStarted && in(from, hubapi.StateTriage, hubapi.StateBacklog, hubapi.StateTodo,
			hubapi.StateStarted)
	case hubapi.CauseLaneQueued:
		return to == hubapi.StateInReview && in(from, hubapi.StateTriage, hubapi.StateBacklog, hubapi.StateTodo,
			hubapi.StateStarted, hubapi.StateInReview)
	case hubapi.CauseLandTrailer:
		return to == hubapi.StateLanded && in(from, hubapi.StateTriage, hubapi.StateBacklog, hubapi.StateTodo,
			hubapi.StateStarted, hubapi.StateInReview, hubapi.StateLanded, hubapi.StateDone)
	case hubapi.CauseEvidenceObserved, hubapi.CauseEvidenceRecorded:
		return to == hubapi.StateDone && from == hubapi.StateLanded
	}
	return false
}

// fakeCitation is one cited evidence record at the current landing.
type fakeCitation struct {
	rec   hubapi.EvidenceRecord
	cause hubapi.MoveCause // CauseEvidenceObserved or CauseEvidenceRecorded
}

// evidenceRecord reads H05's evidence store (UNVERIFIED field name; Step 1
// takes the merged one). It is the fake's only read of that store.
func (w *fakeWorld) evidenceRecord(id string) (hubapi.EvidenceRecord, bool) {
	e, ok := w.evidence[id]
	return e, ok
}

// whyNotApplicable is task 10's decision 3 and 4 for one record, as a
// details.why word, or "" when the record may be cited by this issue.
func (w *fakeWorld) whyNotApplicable(is *fakeIssue, id string) (hubapi.EvidenceRecord, string) {
	rec, ok := w.evidenceRecord(id)
	switch {
	case !ok || rec.ProjectID != is.head.ProjectID:
		return rec, "project"
	case !slices.Contains(is.headRev().Content.Serves, hubapi.Serve{BriefKey: rec.BriefKey, CriterionID: rec.CriterionID}):
		return rec, "criterion"
	case rec.Verdict != hubapi.VerdictSettled:
		return rec, "verdict"
	case rec.RecordedAt < is.landedAt: // one fixed-width UTC format (contract §1)
		return rec, "before_landed"
	case rec.Commit != is.head.LandCommit:
		return rec, "commit"
	}
	if other := w.wk().issues[w.wk().cited[id]]; other != nil && other != is && other.head.LandCommit != is.head.LandCommit {
		return rec, "cited_elsewhere"
	}
	return rec, ""
}

// clearProof is a reopen's, a cancel's and a duplicate's: the word, the land
// commit and every citation go (task 10's decision 6).
func (w *fakeWorld) clearProof(is *fakeIssue) {
	for _, c := range is.citations {
		if w.wk().cited[c.rec.ID] == is.head.ID {
			delete(w.wk().cited, c.rec.ID)
		}
	}
	is.citations, is.landedAt, is.handSet = nil, "", false
	is.head.Proof, is.head.LandCommit, is.head.StateReason = hubapi.ProofNone, "", ""
}
```

Block 7: the move. A repeat of a fact already recorded is 200 with no write and no event
(task 10's decision 7); every other accepted move is one write and one `issue.state`.

```go
func (f *Fake) MoveIssueState(_ context.Context, issueID string, move hubapi.StateMove) (hubapi.IssueHead, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("MoveIssueState", issueID, move)
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
	from, serves := is.head.State, is.headRev().Content.Serves
	switch {
	case !hubapi.ValidIssueState(move.To):
		return hubapi.IssueHead{}, badRequest("to", "That is not a state.")
	case !slices.Contains(hubapi.MoveCauses, move.Cause):
		return hubapi.IssueHead{}, badRequest("cause", "That is not a cause.")
	case !fakeMoveAllowed(from, move.To, move.Cause):
		return hubapi.IssueHead{}, problem(422, fkStateNotAllowed, "That move is not allowed from this state.",
			"from", string(from), "to", string(move.To), "cause", string(move.Cause))
	}
	if err := w.policyRefuses(is, move.To); err != nil { // before any write: no event
		return hubapi.IssueHead{}, err
	}
	repeat := from == move.To && move.Cause != hubapi.CauseHand
	switch move.Cause {
	case hubapi.CauseHand:
		if move.To == from {
			return w.renderIssue(is), nil
		}
		reason := strings.TrimSpace(move.Reason)
		if move.To == hubapi.StateDone && len(serves) > 0 && reason == "" {
			return hubapi.IssueHead{}, badRequest("reason", "Say why this is done without its evidence.")
		}
		if move.To != hubapi.StateDone { // a reopen, a cancel, a duplicate
			w.clearProof(is)
		}
		if move.To == hubapi.StateDone {
			is.handSet, is.head.StateReason, is.head.Proof = true, reason, hubapi.ProofNotProven
			if len(serves) == 0 {
				is.head.Proof = hubapi.ProofNoCriteria
			}
		}
	case hubapi.CauseLaneBound, hubapi.CauseLaneQueued:
		if repeat {
			return w.renderIssue(is), nil
		}
		if move.To == hubapi.StateStarted {
			is.startedBy = me
		}
	case hubapi.CauseLandTrailer:
		if !landCommitRe.MatchString(move.LandCommit) {
			return hubapi.IssueHead{}, badRequest("landCommit", "A land commit is 40 lowercase hex characters.")
		}
		if from == hubapi.StateLanded || from == hubapi.StateDone {
			if move.LandCommit == is.head.LandCommit {
				return w.renderIssue(is), nil
			}
			return hubapi.IssueHead{}, problem(409, fkStateConflict, "This issue already landed at another commit; reopen it first.",
				"landCommit", is.head.LandCommit)
		}
		is.head.LandCommit, is.landedAt, is.head.Proof = move.LandCommit, w.stamp(), hubapi.ProofWaiting
		if len(serves) == 0 {
			is.head.Proof = hubapi.ProofNoCriteria
			if !w.stopAtLanded(is.teamID) {
				move.To = hubapi.StateDone // lands at done in the same write
			}
		}
	case hubapi.CauseEvidenceObserved, hubapi.CauseEvidenceRecorded:
		if len(move.Evidence) == 0 {
			return hubapi.IssueHead{}, badRequest("evidence", "An evidence move cites one record or more.")
		}
		var add []fakeCitation
		for i, id := range move.Evidence {
			rec, why := w.whyNotApplicable(is, id)
			if why != "" {
				return hubapi.IssueHead{}, problem(422, fkEvidenceNotApplicable, "That evidence cannot close this issue.",
					"field", "evidence["+strconv.Itoa(i)+"]", "why", why)
			}
			add = append(add, fakeCitation{rec: rec, cause: move.Cause})
		}
		for _, c := range add {
			is.citations = append(is.citations, c)
			w.wk().cited[c.rec.ID] = is.head.ID
		}
		is.head.Proof = w.proofOf(is)
		if is.head.Proof == hubapi.ProofWaiting {
			return w.renderIssue(is), nil // recorded; the issue stays landed (decision 5)
		}
	}
	is.head.State, is.head.UpdatedAt = move.To, w.stamp()
	w.workEvent(me, fkEventIssueState, is.teamID, is.head.ProjectID, is.head.ID, map[string]any{
		"from": string(from), "to": string(move.To), "cause": string(move.Cause),
		"proof": string(is.head.Proof), "landCommit": move.LandCommit})
	return w.renderIssue(is), nil
}

// proofOf is task 10's DecideProof over the citations: waiting until every
// served pair is cited; then proven, or proven_self_attested when a recorded
// (not observed) record is by the person who created the issue and started it.
func (w *fakeWorld) proofOf(is *fakeIssue) hubapi.Proof {
	self := false
	for _, s := range is.headRev().Content.Serves {
		covered := false
		for _, c := range is.citations {
			if c.rec.BriefKey == s.BriefKey && c.rec.CriterionID == s.CriterionID {
				covered = true
				self = self || (c.cause == hubapi.CauseEvidenceRecorded &&
					c.rec.RecordedBy.ID == is.createdBy && c.rec.RecordedBy.ID == is.startedBy)
			}
		}
		if !covered {
			return hubapi.ProofWaiting
		}
	}
	if self {
		return hubapi.ProofSelfAttested
	}
	return hubapi.ProofProven
}
```

That is the whole of `fake_work.go`. A partial evidence move (decision 5 of task 10: valid
records that do not yet cover every criterion) records its citations and appends **no** event,
because the head does not change; whether the Hub audits a citation-only write is task 10's to
say (UNVERIFIED: its part names no event for it), and a conformance row of task 05 pins what
both do.

#### `fake_work_stub.go`, complete

```go
package hubclient

import (
	"context"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
)

// Part 2 of the fake's Work rules, as placeholders. Task 05 deletes this file
// whole and gives every hook and method below its rule in fake_work_social.go
// and fake_work_intake.go; no line of fake_work.go changes when it does.

// The six hooks part 1 calls, at their part-1 bodies. erasedWrite is task
// 14's rule (an erased issue takes no writes, 409); policyRefuses is task 11's
// priorityToLeaveTriage. Neither can refuse before task 05.
func (w *fakeWorld) erasedWrite(*fakeIssue) error                    { return nil }
func (w *fakeWorld) policyRefuses(*fakeIssue, hubapi.IssueState) error { return nil }
func (w *fakeWorld) checkIssueLabels(_, field string, labels []string) error {
	if len(labels) > 0 { // task 08's decision 8 until team labels exist
		return badRequest(field, "Labels arrive with team labels (Phase W task 05).")
	}
	return nil
}
func (w *fakeWorld) defaultState(string) hubapi.IssueState                  { return hubapi.StateBacklog } // task 11's DefaultTeamPolicy
func (w *fakeWorld) stopAtLanded(string) bool                               { return false }
func (w *fakeWorld) subscribeOn(*fakeIssue, string, hubapi.SubscriptionCause) {}

// workNotInFakeYet is fake.go's notInFakeYet, kept here because H05 deletes that
// one (decision (d)): logged and switched like any call, then 501.
func (f *Fake) workNotInFakeYet(method string, args ...any) error {
	f.w.mu.Lock()
	defer f.w.mu.Unlock()
	if _, err := f.enter(method, args...); err != nil {
		return err
	}
	return problem(501, "not_in_fake_yet", "The fake Hub does not answer "+method+" until Phase W task 05.")
}

func (f *Fake) AddRelation(_ context.Context, issueID string, req hubapi.RelationRequest) (hubapi.IssueHead, error) {
	return hubapi.IssueHead{}, f.workNotInFakeYet("AddRelation", issueID, req)
}
func (f *Fake) RemoveRelation(_ context.Context, issueID string, kind hubapi.RelationKind, otherID string) (hubapi.IssueHead, error) {
	return hubapi.IssueHead{}, f.workNotInFakeYet("RemoveRelation", issueID, string(kind), otherID)
}
func (f *Fake) Labels(_ context.Context, teamID string) ([]hubapi.Label, error) {
	return nil, f.workNotInFakeYet("Labels", teamID)
}
func (f *Fake) CreateLabel(_ context.Context, teamID string, req hubapi.CreateLabelRequest) (hubapi.Label, error) {
	return hubapi.Label{}, f.workNotInFakeYet("CreateLabel", teamID, req)
}
func (f *Fake) ArchiveLabel(_ context.Context, labelID string) (hubapi.Label, error) {
	return hubapi.Label{}, f.workNotInFakeYet("ArchiveLabel", labelID)
}
func (f *Fake) TeamPolicy(_ context.Context, teamID string) (hubapi.TeamPolicy, error) {
	return hubapi.TeamPolicy{}, f.workNotInFakeYet("TeamPolicy", teamID)
}
func (f *Fake) PutTeamPolicy(_ context.Context, teamID string, req hubapi.PutTeamPolicyRequest) (hubapi.TeamPolicy, error) {
	return hubapi.TeamPolicy{}, f.workNotInFakeYet("PutTeamPolicy", teamID, req)
}
func (f *Fake) Comments(_ context.Context, issueID string, page hubapi.PageRequest) (hubapi.Page[hubapi.Comment], error) {
	return hubapi.Page[hubapi.Comment]{}, f.workNotInFakeYet("Comments", issueID, page)
}
func (f *Fake) CreateComment(_ context.Context, issueID string, req hubapi.CreateCommentRequest) (hubapi.Comment, error) {
	return hubapi.Comment{}, f.workNotInFakeYet("CreateComment", issueID, req)
}
func (f *Fake) React(_ context.Context, commentID string, req hubapi.ReactRequest) (hubapi.Comment, error) {
	return hubapi.Comment{}, f.workNotInFakeYet("React", commentID, req)
}
func (f *Fake) ResolveComment(_ context.Context, commentID string, req hubapi.ResolveRequest) (hubapi.Comment, error) {
	return hubapi.Comment{}, f.workNotInFakeYet("ResolveComment", commentID, req)
}
func (f *Fake) Subscribe(_ context.Context, issueID string, req hubapi.SubscribeRequest) (hubapi.Subscription, error) {
	return hubapi.Subscription{}, f.workNotInFakeYet("Subscribe", issueID, req)
}
func (f *Fake) Subscriptions(_ context.Context, page hubapi.PageRequest) (hubapi.Page[hubapi.Subscription], error) {
	return hubapi.Page[hubapi.Subscription]{}, f.workNotInFakeYet("Subscriptions", page)
}
func (f *Fake) SearchIssues(_ context.Context, q hubapi.IssueSearchQuery) ([]hubapi.IssueHead, error) {
	return nil, f.workNotInFakeYet("SearchIssues", q)
}
func (f *Fake) ImportBatch(_ context.Context, projectID string, batch hubapi.ImportBatch) (hubapi.ImportResult, error) {
	return hubapi.ImportResult{}, f.workNotInFakeYet("ImportBatch", projectID, batch)
}
func (f *Fake) AttachBlob(_ context.Context, issueID string, req hubapi.AttachRequest) (hubapi.IssueAttachment, error) {
	return hubapi.IssueAttachment{}, f.workNotInFakeYet("AttachBlob", issueID, req)
}
func (f *Fake) Attachments(_ context.Context, issueID string) ([]hubapi.IssueAttachment, error) {
	return nil, f.workNotInFakeYet("Attachments", issueID)
}
func (f *Fake) EraseIssue(_ context.Context, issueID string, req hubapi.EraseRequest) (hubapi.WorkEraseResponse, error) {
	return hubapi.WorkEraseResponse{}, f.workNotInFakeYet("EraseIssue", issueID, req)
}
func (f *Fake) EraseComment(_ context.Context, commentID string, req hubapi.EraseRequest) (hubapi.WorkEraseResponse, error) {
	return hubapi.WorkEraseResponse{}, f.workNotInFakeYet("EraseComment", commentID, req)
}
```

#### `client_work_stub.go`, complete

```go
package hubclient

import (
	"context"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
)

// The HTTP client's Work methods, as placeholders in client.go's own idiom:
// every one answers ErrNotConfigured. Phase W task 16 deletes this file in the
// commit that adds methods_work.go. It is not client.go, which H33 replaces
// file for file (decision (j)).

type (
	iwr   = hubapi.IssueWithRevision
	ihead = hubapi.IssueHead
	irev  = hubapi.IssueRevision
)

func (*Client) SetProjectKey(context.Context, string, string) (hubapi.ProjectKey, error) {
	return hubapi.ProjectKey{}, ErrNotConfigured
}
func (*Client) CreateIssue(context.Context, string, hubapi.CreateIssueRequest) (iwr, error) {
	return iwr{}, ErrNotConfigured
}
func (*Client) Issue(context.Context, string) (iwr, error) { return iwr{}, ErrNotConfigured }
func (*Client) IssueByNumber(context.Context, string, int) (iwr, error) {
	return iwr{}, ErrNotConfigured
}
func (*Client) Issues(context.Context, string, hubapi.IssueQuery, hubapi.PageRequest) (hubapi.Page[ihead], error) {
	return hubapi.Page[ihead]{}, ErrNotConfigured
}
func (*Client) PatchIssue(context.Context, string, hubapi.PatchIssueRequest) (ihead, error) {
	return ihead{}, ErrNotConfigured
}
func (*Client) WriteIssueRevision(context.Context, string, hubapi.WriteIssueRevisionRequest) (irev, error) {
	return irev{}, ErrNotConfigured
}
func (*Client) IssueRevisions(context.Context, string, hubapi.PageRequest) (hubapi.Page[irev], error) {
	return hubapi.Page[irev]{}, ErrNotConfigured
}
func (*Client) IssueRevisionAt(context.Context, string, int) (irev, error) { return irev{}, ErrNotConfigured }
func (*Client) RestoreIssueRevision(context.Context, string, hubapi.RestoreRevisionRequest) (irev, error) {
	return irev{}, ErrNotConfigured
}
func (*Client) MoveIssueState(context.Context, string, hubapi.StateMove) (ihead, error) {
	return ihead{}, ErrNotConfigured
}
func (*Client) AddRelation(context.Context, string, hubapi.RelationRequest) (ihead, error) {
	return ihead{}, ErrNotConfigured
}
func (*Client) RemoveRelation(context.Context, string, hubapi.RelationKind, string) (ihead, error) {
	return ihead{}, ErrNotConfigured
}
func (*Client) Labels(context.Context, string) ([]hubapi.Label, error) { return nil, ErrNotConfigured }
func (*Client) CreateLabel(context.Context, string, hubapi.CreateLabelRequest) (hubapi.Label, error) {
	return hubapi.Label{}, ErrNotConfigured
}
func (*Client) ArchiveLabel(context.Context, string) (hubapi.Label, error) {
	return hubapi.Label{}, ErrNotConfigured
}
func (*Client) TeamPolicy(context.Context, string) (hubapi.TeamPolicy, error) {
	return hubapi.TeamPolicy{}, ErrNotConfigured
}
func (*Client) PutTeamPolicy(context.Context, string, hubapi.PutTeamPolicyRequest) (hubapi.TeamPolicy, error) {
	return hubapi.TeamPolicy{}, ErrNotConfigured
}
func (*Client) Comments(context.Context, string, hubapi.PageRequest) (hubapi.Page[hubapi.Comment], error) {
	return hubapi.Page[hubapi.Comment]{}, ErrNotConfigured
}
func (*Client) CreateComment(context.Context, string, hubapi.CreateCommentRequest) (hubapi.Comment, error) {
	return hubapi.Comment{}, ErrNotConfigured
}
func (*Client) React(context.Context, string, hubapi.ReactRequest) (hubapi.Comment, error) {
	return hubapi.Comment{}, ErrNotConfigured
}
func (*Client) ResolveComment(context.Context, string, hubapi.ResolveRequest) (hubapi.Comment, error) {
	return hubapi.Comment{}, ErrNotConfigured
}
func (*Client) Subscribe(context.Context, string, hubapi.SubscribeRequest) (hubapi.Subscription, error) {
	return hubapi.Subscription{}, ErrNotConfigured
}
func (*Client) Subscriptions(context.Context, hubapi.PageRequest) (hubapi.Page[hubapi.Subscription], error) {
	return hubapi.Page[hubapi.Subscription]{}, ErrNotConfigured
}
func (*Client) SearchIssues(context.Context, hubapi.IssueSearchQuery) ([]ihead, error) {
	return nil, ErrNotConfigured
}
func (*Client) ImportBatch(context.Context, string, hubapi.ImportBatch) (hubapi.ImportResult, error) {
	return hubapi.ImportResult{}, ErrNotConfigured
}
func (*Client) AttachBlob(context.Context, string, hubapi.AttachRequest) (hubapi.IssueAttachment, error) {
	return hubapi.IssueAttachment{}, ErrNotConfigured
}
func (*Client) Attachments(context.Context, string) ([]hubapi.IssueAttachment, error) {
	return nil, ErrNotConfigured
}
func (*Client) EraseIssue(context.Context, string, hubapi.EraseRequest) (hubapi.WorkEraseResponse, error) {
	return hubapi.WorkEraseResponse{}, ErrNotConfigured
}
func (*Client) EraseComment(context.Context, string, hubapi.EraseRequest) (hubapi.WorkEraseResponse, error) {
	return hubapi.WorkEraseResponse{}, ErrNotConfigured
}
```

The three aliases are unexported and exist only in this file; task 16 deletes them with it.

---

#### Tests first: `fake_work_test.go`, complete, each test with the mutant it kills

Four blocks; the file is their concatenation. It is `package hubclient`, so it reuses
`fake_test.go`'s helpers (`bg`, `must`, `noErr`, `wantProblem`, `newFake`, `withTeam`,
`withProject`, `branchBrief`, `write`; `fake_test.go:19-110` at `9f2fa31`). Block 1 is the rig.

```go
package hubclient

import (
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
)

// workRig is withTeam's Ada (admin) and Grace (member), Linus joined as a
// viewer, project P with a GitHub remote, and branch brief B at revision 1.
type workRig struct {
	f, ada, grace, linus *Fake
	team                 hubapi.Team
	p                    hubapi.Project
	briefKey             string // brief:bf-…@1
}

var workRemote = hubapi.RemoteRequest{RepoKey: "github.com/lcfrai/panaceaapp", Host: hubapi.HostGitHub}

func newWorkRig(t *testing.T) workRig {
	t.Helper()
	f, ada, grace, linus, team := withTeam(t)
	must(ada.CreateInvite(bg, team.ID, hubapi.CreateInviteRequest{Email: FakeLinus.Email, Role: hubapi.RoleViewer}))(t)
	must(linus.Me(bg))(t) // binds the invite
	p := withProject(t, ada, team.ID, "Checkout", workRemote)
	b := branchBrief(t, ada, p.ID, "refs/heads/evidence-upload")
	return workRig{f, ada, grace, linus, team, p, write(t, ada, p.ID, b.ID, 0, "complete").Key}
}

func htext(s string) hubapi.AuthoredText { return hubapi.AuthoredText{Text: s, Author: hubapi.AuthorHuman} }

// chore is the smallest valid create: no criterion, so a reason.
func chore(title string) hubapi.CreateIssueRequest {
	return hubapi.CreateIssueRequest{Title: htext(title), Description: htext(""), Serves: []hubapi.Serve{},
		Implements: []string{}, Labels: []string{}, NotTiedReason: hubapi.NotTiedChore}
}

// serving is an issue created by Grace serving the given criteria of B.
func (r workRig) serving(t *testing.T, criteria ...string) hubapi.IssueWithRevision {
	t.Helper()
	req := chore("Upload evidence")
	req.NotTiedReason = ""
	for _, c := range criteria {
		req.Serves = append(req.Serves, hubapi.Serve{BriefKey: r.briefKey, CriterionID: c})
	}
	return must(r.grace.CreateIssue(bg, r.p.ID, req))(t)
}

// eventsOf is every feed event so far whose kind starts with prefix. It reads
// the feed as a caller does, through Changes (H05; after "0" is the start).
func eventsOf(t *testing.T, view *Fake, prefix string) []hubapi.Event {
	t.Helper()
	var out []hubapi.Event
	for _, e := range must(view.Changes(bg, "0", 200))(t).Events {
		if strings.HasPrefix(e.Kind, prefix) {
			out = append(out, e)
		}
	}
	return out
}

func move(to hubapi.IssueState, cause hubapi.MoveCause) hubapi.StateMove {
	return hubapi.StateMove{To: to, Cause: cause, Evidence: []string{}}
}

const commitA, commitB = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
```

Block 2: numbers, roles, the key, and the one event of a create.

```go
// Mutant: the counter read or bumped outside the lock (a number repeats or is
// skipped); and a refused create that takes a number anyway.
func TestIssueNumbersAreGaplessUnderRace(t *testing.T) {
	r := newWorkRig(t)
	bad := chore("elsewhere")
	bad.Serves, bad.NotTiedReason = []hubapi.Serve{{BriefKey: "brief:bf-0000000000000000@1", CriterionID: "c1"}}, ""
	_, err := r.grace.CreateIssue(bg, r.p.ID, bad)
	wantProblem(t, err, 422, hubapi.CodeKeyUnresolvable, "field", "serves[0]")
	got, errs := make([]int, 20), make([]error, 20)
	var wg sync.WaitGroup
	for i := range got {
		wg.Add(1)
		go func() {
			defer wg.Done()
			iw, err := r.grace.CreateIssue(bg, r.p.ID, chore("n"))
			got[i], errs[i] = iw.Head.Number, err
		}()
	}
	wg.Wait()
	noErr(t, errors.Join(errs...))
	slices.Sort(got)
	for i, n := range got {
		if n != i+1 {
			t.Fatalf("numbers = %v, want 1 to 20, each once", got)
		}
	}
}

// Mutant: a missing role check on create, on the key, or a 403 that tells an
// outsider the issue exists.
func TestWorkRolesAreChecked(t *testing.T) {
	r := newWorkRig(t)
	_, err := r.linus.CreateIssue(bg, r.p.ID, chore("viewer"))
	wantProblem(t, err, 403, hubapi.CodeForbidden, "required", "member")
	_, err = r.grace.SetProjectKey(bg, r.p.ID, "EP")
	wantProblem(t, err, 403, hubapi.CodeForbidden, "required", "admin")
	is := must(r.grace.CreateIssue(bg, r.p.ID, chore("mine")))(t)
	out := r.f.As(FakeUser{ID: "u-00000000000000a4", DisplayName: "Out Sider", Email: "out@example.test"})
	_, err = out.Issue(bg, is.Head.ID)
	wantProblem(t, err, 404, fkIssueNotFound, "issueId", is.Head.ID)
}

// Mutant: a stored alias (a key change leaves PAY-142 reading EP-142), or a key
// change that rewrites a revision key; and a second key event for the same key.
func TestAKeyChangeChangesAliasesNotKeys(t *testing.T) {
	r := newWorkRig(t)
	must(r.ada.SetProjectKey(bg, r.p.ID, "EP"))(t)
	var last hubapi.IssueWithRevision
	for range 142 {
		last = must(r.grace.CreateIssue(bg, r.p.ID, chore("n")))(t)
	}
	if last.Head.Alias != "EP-142" {
		t.Fatalf("alias = %q, want EP-142", last.Head.Alias)
	}
	must(r.ada.SetProjectKey(bg, r.p.ID, "PAY"))(t)
	must(r.ada.SetProjectKey(bg, r.p.ID, "PAY"))(t) // the same key: no write
	got := must(r.grace.IssueByNumber(bg, r.p.ID, 142))(t)
	if got.Head.Alias != "PAY-142" || got.Revision.Key != last.Revision.Key || got.Head.HeadKey != last.Head.HeadKey {
		t.Errorf("after the rename: alias %q, key %q, headKey %q; want PAY-142 and the keys unchanged",
			got.Head.Alias, got.Revision.Key, got.Head.HeadKey)
	}
	if n := len(eventsOf(t, r.ada, "project.key_set")); n != 2 {
		t.Errorf("project.key_set events = %d, want 2", n)
	}
	other := withProject(t, r.ada, r.team.ID, "Refunds")
	_, err := r.ada.SetProjectKey(bg, other.ID, "PAY")
	wantProblem(t, err, 409, fkProjectKeyTaken, "key", "PAY", "projectId", r.p.ID)
}

// Mutant: head and revision 1 emitted as two events (issue.created and
// issue.revised), or an event whose key is the alias.
func TestACreateIsOneIssueEvent(t *testing.T) {
	r := newWorkRig(t)
	must(r.ada.SetProjectKey(bg, r.p.ID, "EP"))(t)
	is := must(r.grace.CreateIssue(bg, r.p.ID, chore("one")))(t)
	evs := eventsOf(t, r.ada, "issue.")
	if len(evs) != 1 || evs[0].Kind != fkEventIssueCreated || evs[0].Subject != is.Head.ID {
		t.Fatalf("issue events = %+v, want one issue.created for %s", evs, is.Head.ID)
	}
	if evs[0].Data["key"] != is.Revision.Key || evs[0].Data["digest"] != is.Revision.Digest.String() {
		t.Errorf("data = %v, want the revision key and digest", evs[0].Data)
	}
}
```

Block 3: revisions and the head write.

```go
// Mutant: last writer wins (a stale base overwrites, or writes a second revision).
func TestAStaleBaseIsAConflictAndWritesNothing(t *testing.T) {
	r := newWorkRig(t)
	is := must(r.grace.CreateIssue(bg, r.p.ID, chore("v1")))(t)
	c := is.Revision.Content
	c.Title = htext("v2")
	_, err := r.grace.WriteIssueRevision(bg, is.Head.ID, hubapi.WriteIssueRevisionRequest{BaseRevision: 0, Content: c})
	wantProblem(t, err, 409, hubapi.CodeRevisionConflict, "headRevision", "1")
	if revs := must(r.grace.IssueRevisions(bg, is.Head.ID, hubapi.PageRequest{}))(t); len(revs.Items) != 1 {
		t.Fatalf("revisions after a stale write = %d, want 1", len(revs.Items))
	}
	if got := must(r.grace.WriteIssueRevision(bg, is.Head.ID, hubapi.WriteIssueRevisionRequest{BaseRevision: 1, Content: c}))(t); got.Revision != 2 {
		t.Errorf("revision = %d, want 2", got.Revision)
	}
}

// Mutant: a revision per head write (reprioritising invalidates an agent's
// context); an event for a patch that changes nothing.
func TestAPatchWritesNoRevision(t *testing.T) {
	r := newWorkRig(t)
	is := must(r.grace.CreateIssue(bg, r.p.ID, chore("p")))(t)
	high := hubapi.PriorityHigh
	for range 2 { // the second patch changes nothing
		h := must(r.grace.PatchIssue(bg, is.Head.ID, hubapi.PatchIssueRequest{Priority: &high}))(t)
		if h.HeadRevision != 1 || h.Priority != high {
			t.Fatalf("head = rev %d, %s; want rev 1, high", h.HeadRevision, h.Priority)
		}
	}
	if evs := eventsOf(t, r.ada, fkEventIssueUpdated); len(evs) != 1 || evs[0].Data["fields"] != "priority" {
		t.Errorf("issue.updated = %+v, want one naming priority", evs)
	}
}

// Mutant: a digest over bytes other than IssueRevisionDigest's (json.Marshal
// of the content, or the envelope without projectId); a restore that edits the
// old revision instead of writing a new one.
func TestTheStoredDigestIsTheCanonicalOne(t *testing.T) {
	r := newWorkRig(t)
	is := r.serving(t, "c1")
	c := is.Revision.Content
	c.Title = htext("second")
	must(r.grace.WriteIssueRevision(bg, is.Head.ID, hubapi.WriteIssueRevisionRequest{BaseRevision: 1, Content: c}))(t)
	back := must(r.grace.RestoreIssueRevision(bg, is.Head.ID, hubapi.RestoreRevisionRequest{FromRevision: 1}))(t)
	if back.Revision != 3 || back.RestoredFrom == nil || *back.RestoredFrom != 1 || back.Content.Title != is.Revision.Content.Title {
		t.Fatalf("restore = %+v, want revision 3 restored from 1 with 1's title", back)
	}
	for _, rev := range must(r.grace.IssueRevisions(bg, is.Head.ID, hubapi.PageRequest{}))(t).Items {
		d, _, err := hubapi.IssueRevisionDigest(hubapi.IssueRevisionEnvelope{IssueID: rev.IssueID,
			Revision: rev.Revision, ProjectID: rev.ProjectID, Content: rev.Content})
		noErr(t, err)
		if d != rev.Digest {
			t.Errorf("revision %d: stored %s, canonical %s", rev.Revision, rev.Digest, d)
		}
	}
	if one := must(r.grace.IssueRevisionAt(bg, is.Head.ID, 1))(t); one.Digest != is.Revision.Digest {
		t.Errorf("revision 1 changed: %s, was %s", one.Digest, is.Revision.Digest)
	}
}
```

Block 4 (the same file, continued): the state moves.

```go
// Mutant: the silent done (a hand-set done on a serving issue with no reason),
// or a hand-set done that reads proven.
func TestAHandSetDoneNeedsAReason(t *testing.T) {
	r := newWorkRig(t)
	is := r.serving(t, "c1")
	_, err := r.grace.MoveIssueState(bg, is.Head.ID, move(hubapi.StateDone, hubapi.CauseHand))
	wantProblem(t, err, 400, hubapi.CodeBadRequest, "field", "reason")
	m := move(hubapi.StateDone, hubapi.CauseHand)
	m.Reason = "shipped in the hotfix"
	h := must(r.grace.MoveIssueState(bg, is.Head.ID, m))(t)
	if h.State != hubapi.StateDone || h.Proof != hubapi.ProofNotProven || h.StateReason != m.Reason {
		t.Errorf("head = %s, %q, %q; want done, not_proven, the reason", h.State, h.Proof, h.StateReason)
	}
	c := must(r.grace.CreateIssue(bg, r.p.ID, chore("tidy")))(t)
	if h := must(r.grace.MoveIssueState(bg, c.Head.ID, move(hubapi.StateDone, hubapi.CauseHand)))(t); h.Proof != hubapi.ProofNoCriteria {
		t.Errorf("a chore's hand-set done reads %q, want no_criteria", h.Proof)
	}
}

// Mutant: a person setting a fact state by hand; a repeated fact that writes a
// second event (task 22's hook fires twice).
func TestFactStatesNeedTheirFactCause(t *testing.T) {
	r := newWorkRig(t)
	is := r.serving(t, "c1")
	_, err := r.grace.MoveIssueState(bg, is.Head.ID, move(hubapi.StateStarted, hubapi.CauseHand))
	wantProblem(t, err, 422, fkStateNotAllowed, "from", "backlog", "to", "started", "cause", "hand")
	for range 2 {
		must(r.grace.MoveIssueState(bg, is.Head.ID, move(hubapi.StateStarted, hubapi.CauseLaneBound)))(t)
	}
	if n := len(eventsOf(t, r.ada, fkEventIssueState)); n != 1 {
		t.Errorf("issue.state events = %d, want 1", n)
	}
	_, err = r.grace.MoveIssueState(bg, is.Head.ID, move(hubapi.StateDone, hubapi.CauseEvidenceObserved))
	wantProblem(t, err, 422, fkStateNotAllowed, "from", "started", "to", "done")
}

// Mutant: a second landing that silently replaces the first proof; a chore
// that waits at landed forever.
func TestALandingIsRecordedOnce(t *testing.T) {
	r := newWorkRig(t)
	is := r.serving(t, "c1")
	land := func(commit string) (hubapi.IssueHead, error) {
		m := move(hubapi.StateLanded, hubapi.CauseLandTrailer)
		m.LandCommit = commit
		return r.grace.MoveIssueState(bg, is.Head.ID, m)
	}
	h := must(land(commitA))(t)
	if h.State != hubapi.StateLanded || h.Proof != hubapi.ProofWaiting || h.LandCommit != commitA {
		t.Fatalf("head = %s, %q, %s; want landed, waiting, A", h.State, h.Proof, h.LandCommit)
	}
	must(land(commitA))(t) // the same commit: 200, no write
	_, err := land(commitB)
	wantProblem(t, err, 409, fkStateConflict, "landCommit", commitA)
	if n := len(eventsOf(t, r.ada, fkEventIssueState)); n != 1 {
		t.Errorf("issue.state events = %d, want 1", n)
	}
	c := must(r.grace.CreateIssue(bg, r.p.ID, chore("tidy")))(t)
	m := move(hubapi.StateLanded, hubapi.CauseLandTrailer)
	m.LandCommit = commitA
	if h := must(r.grace.MoveIssueState(bg, c.Head.ID, m))(t); h.State != hubapi.StateDone || h.Proof != hubapi.ProofNoCriteria {
		t.Errorf("a chore lands at %s, %q; want done, no_criteria", h.State, h.Proof)
	}
}

// Mutant: done on the first criterion's evidence; evidence at another commit
// accepted; a partial evidence move refused instead of recorded.
func TestEvidenceClosesWhenEveryCriterionIsCited(t *testing.T) {
	r := newWorkRig(t)
	is := r.serving(t, "c1", "c2")
	m := move(hubapi.StateLanded, hubapi.CauseLandTrailer)
	m.LandCommit = commitA
	must(r.grace.MoveIssueState(bg, is.Head.ID, m))(t)
	record := func(criterion, commit string) string {
		return must(r.grace.RecordEvidence(bg, hubapi.EvidenceRequest{Repo: workRemote.RepoKey, BriefKey: r.briefKey,
			CriterionID: criterion, CriterionDigest: hubapi.DigestOf([]byte(criterion)), Commit: commit,
			Verdict: hubapi.VerdictSettled, Because: "the check exited 0"}))(t).ID
	}
	cite := func(ids ...string) (hubapi.IssueHead, error) {
		e := move(hubapi.StateDone, hubapi.CauseEvidenceObserved)
		e.Evidence = ids
		return r.grace.MoveIssueState(bg, is.Head.ID, e)
	}
	_, err := cite(record("c1", commitB))
	wantProblem(t, err, 422, fkEvidenceNotApplicable, "field", "evidence[0]", "why", "commit")
	if h := must(cite(record("c1", commitA)))(t); h.State != hubapi.StateLanded || h.Proof != hubapi.ProofWaiting {
		t.Fatalf("after c1: %s, %q; want landed, waiting", h.State, h.Proof)
	}
	if h := must(cite(record("c2", commitA)))(t); h.State != hubapi.StateDone || h.Proof != hubapi.ProofProven {
		t.Errorf("after c2: %s, %q; want done, proven", h.State, h.Proof)
	}
}
```

Eleven top-level tests: `TestIssueNumbersAreGaplessUnderRace`, `TestWorkRolesAreChecked`,
`TestAKeyChangeChangesAliasesNotKeys`, `TestACreateIsOneIssueEvent`,
`TestAStaleBaseIsAConflictAndWritesNothing`, `TestAPatchWritesNoRevision`,
`TestTheStoredDigestIsTheCanonicalOne`, `TestAHandSetDoneNeedsAReason`,
`TestFactStatesNeedTheirFactCause`, `TestALandingIsRecordedOnce`,
`TestEvidenceClosesWhenEveryCriterionIsCited`; and one compile-time check, `var _ Hub =
(*Fake)(nil)` and `(*Client)(nil)` (`hubclient.go:147-148`), which kills a Work method missing
from the fake or the client stub. Every test that counts events reads H05's `Changes`, and the
evidence test calls H05's `RecordEvidence`, so they run once H05 is merged, which is this task's
dependency.

---

#### Steps

- [ ] **Step 1: Check the sync base and take H05's names.** Tasks 03 and H04-H05 are on the
  sync base (rule H1): `grep -n "type IssueWithRevision\|type WorkEraseResponse"
  control-plane/internal/hub/hubapi/work.go` prints two lines; `grep -n "func (f \*Fake) Changes"
  control-plane/internal/hub/hubclient/fake_feed.go` prints one. Then find H05's two internals:
  `grep -n "func (w \*fakeWorld)" control-plane/internal/hub/hubclient/fake_feed.go` names the
  function that appends one `hubapi.Event` (assigning `Seq` and `At`), and `grep -n "evidence"
  control-plane/internal/hub/hubclient/fake.go` names the evidence store. If they are not
  `appendEvent` and a map `evidence` by id, change the one call in `workEvent` and the one read in
  `evidenceRecord` to the merged names, and note them here. Re-read every anchor in *Anchors*.
- [ ] **Step 2: Write the tests.** Create `fake_work_test.go` exactly as above. Run
  `go test ./control-plane/internal/hub/hubclient/ -count=1` and watch it fail to compile
  (`r.grace.CreateIssue undefined`, `undefined: fkIssueNotFound`).
- [ ] **Step 3: Write the code**, in this order so each build tells you one thing: `work.go`; the
  one line in `hubclient.go` (the build now fails: `*Fake` and `*Client` do not implement
  `Hub`); `client_work_stub.go` (the client error goes); `fake_work_stub.go` and `fake_work.go`.
- [ ] **Step 4: Run to green**, then `go vet ./control-plane/internal/hub/hubclient/` and
  `gofmt -l control-plane/internal/hub/hubclient/` (prints nothing).
- [ ] **Step 5: Prove the tests bite.** Apply each mutant by hand, run, see the named test fail,
  revert. None is committed.

| Mutant, applied by hand | Test that must fail |
|---|---|
| move `wk.numbers[projectID]++` above `checkServes` | `TestIssueNumbersAreGaplessUnderRace` (the refused create burns 1) |
| drop `need(role, hubapi.RoleAdmin)` from `SetProjectKey` | `TestWorkRolesAreChecked` |
| set `is.head.Alias` at create and drop the recompute in `renderIssue` | `TestAKeyChangeChangesAliasesNotKeys` |
| emit `fkEventIssueRevised` in `CreateIssue` as well | `TestACreateIsOneIssueEvent` |
| drop the `BaseRevision` check | `TestAStaleBaseIsAConflictAndWritesNothing` |
| call `appendRevision` at the end of `PatchIssue` | `TestAPatchWritesNoRevision` |
| digest `json.Marshal(content)` in `appendRevision` | `TestTheStoredDigestIsTheCanonicalOne` |
| drop the `reason == ""` refusal | `TestAHandSetDoneNeedsAReason` |
| `case hubapi.CauseHand: return true` in `fakeMoveAllowed` | `TestFactStatesNeedTheirFactCause` |
| overwrite `LandCommit` when `from` is `landed` | `TestALandingIsRecordedOnce` |
| `is.head.Proof = hubapi.ProofProven` after the first cited record | `TestEvidenceClosesWhenEveryCriterionIsCited` |

- [ ] **Step 6: Commit**, one commit for the code and its tests: `git add` the five new files,
  then `git commit -F <msgfile> --` the six paths of *Files*. Message file under
  `.superpowers/sdd/work/msg/`; last line
  `Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>`.

#### Acceptance

```bash
go build ./control-plane/... && go vet ./control-plane/internal/hub/hubclient/
# (no output)
go test ./control-plane/internal/hub/hubclient/ -race -count=1 -v -run 'TestIssueNumbersAreGaplessUnderRace|TestWorkRolesAreChecked|TestAKeyChangeChangesAliasesNotKeys|TestACreateIsOneIssueEvent|TestAStaleBaseIsAConflictAndWritesNothing|TestAPatchWritesNoRevision|TestTheStoredDigestIsTheCanonicalOne|TestAHandSetDoneNeedsAReason|TestFactStatesNeedTheirFactCause|TestALandingIsRecordedOnce|TestEvidenceClosesWhenEveryCriterionIsCited' 2>&1 | grep -c '^--- PASS'
# 11
go test ./control-plane/internal/hub/... -race -count=1
# ok for every package: the Hub's own fake_test.go and conformance rows unchanged
git diff --numstat START -- control-plane/internal/hub/hubclient/hubclient.go control-plane/internal/hub/hubclient/fake.go
# 1	0	control-plane/internal/hub/hubclient/hubclient.go   (fake.go does not appear)
grep -c 'func (f \*Fake) ' control-plane/internal/hub/hubclient/fake_work.go control-plane/internal/hub/hubclient/fake_work_stub.go
# fake_work.go:11  fake_work_stub.go:20   (eleven part-1 methods; nineteen part-2 and workNotInFakeYet)
```

`START` is the task's start commit.

**What this task does not claim.**

- The HTTP client's Work methods or the relay (task 16), the real Hub's routes (tasks 08-10), or
  that the real Hub passes any row (task 05 writes the rows; task 16 runs them against it).
- Part 2's rules: relations, labels, policies, comments, subscriptions, import, search,
  attachments, erase (task 05). Until then those methods answer 501 and the six hooks hold their
  part-1 bodies (labels `[]`, default `backlog`, no stop at `landed`, no subscription, nothing
  erased, no policy refusal).
- That `fakeMoveAllowed` **is** `hubapi.MoveAllowed`. It is the same table by reading; task 10
  makes it one function.
- `evidenceCommit` (a later commit than the land commit, task 10's decision 3), a restore's
  `baseRevision` (task 09's decision 3), or `revision_erased` being reachable: nothing erases
  until task 05.
- That the code and event spellings here are the declared ones. They are copied from tasks 08-10
  and 13's parts; task 05's rows assert them and task 16 runs the rows against the Hub.
- Anything the desktop shows. Tasks 37 onward.

**Parity rows served** (each closed by tasks 08-10 and its desktop surface): **L-3** (the display
alias), **L-8** (fixed states), **L-9** (the default state; task 05 adds the team's), **L-13**
(priority), **L-34** (immutable, digested revisions), **L-38** (the assignee, a person).

**Corrections to the scope paragraph** (the skeleton row wins; this part records them):
`Resolve` is `ResolveComment` (decision (a)); `Attachments` is added (b); erase takes
`hubapi.EraseRequest` and answers `hubapi.WorkEraseResponse` (c); the stub code is
`not_in_fake_yet` (d); the event kinds are task 13's, `issue.updated` for the scope's
`issue.changed` (e); `RestoreIssueRevision` takes `hubapi.RestoreRevisionRequest`, not a bare
revision; `fakeWorld` is not edited (i); and a sixth file, `client_work_stub.go` (j).

**Gaps found while writing this part** (for the skeleton's owner and the named tasks):

1. Adding `Work` to `Hub` breaks `var _ Hub = (*Client)(nil)` until task 16. This part adds
   `client_work_stub.go`; task 16's file list must delete it.
2. Task 10 spells the cause and result types `StateCause` and `ProofWord`
   (`task-10.md:236-313`); task 03 declared `MoveCause` and `Proof`. Task 10 must use task 03's
   names, and its file list must gain `hubclient/fake_work.go` (the `fakeMoveAllowed` body).
3. Task 08 gives the not-tied refusal a status and no code; this part spells
   `not_tied_reason_required`.
4. Task 16's part drafted `hubapi.Attachment`, a bodyless erase answering a head, and
   `RestoreIssueRevision(ctx, id, rev int)`; this part's signatures differ (its Step 2 takes them).
5. H05's feed-append helper and evidence store are unnamed in its part (UNVERIFIED; Step 1).
