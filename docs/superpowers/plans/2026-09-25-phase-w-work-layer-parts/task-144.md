### Task 144: the fake's W5 rules and conformance rows

**Slice:** W5. **Size:** 3 focused hours (an estimate, not a measurement). **Stream:** WH.
**dependsOn:** 143 (SW.21: every W5 type, `WorkIntake`, the codes, the stubs). **Seam:** **SW.21**,
frozen here the way task 115 froze SW.20: after this task only added methods, fields or routes.
Consumed by 146-167 (each route passes these rows against the real Hub in task 168) and 169-173
(the desktop's contract tests run on this fake).

Task 143 made `*Fake` satisfy `WorkIntake` with 501s. Every W5 handler and surface is tested
against the fake first, so the fake must refuse what the Hub refuses, for the same reason, with the
same code. W5 is the first slice whose writers are **not all people**: a tracker's integration
principal, an approved app and an API key write too. This task gives the fake those principals,
writes the W5 rules once, in memory, and writes each rule as a conformance row, so task 168 runs
the identical rows against the Hub and a rule the two disagree on fails in one place.

**Anchors: the model is task 115** (`task-115.md:76-169` the state-beside-the-world pattern,
`:751-873` the rows' head, `:874-901` the table). Its reading of H's fake (`fake.go` `enter`,
`stamp`, `problem`, `badRequest`, `need`, at `feat/hub` `1271ddb`) is taken as written, not
re-read: the Hub worktree at `3df9fab8` has no `hubclient/fake.go` yet (checked 2026-09-26:
`control-plane/internal/hub/` holds `hubapi` only), so every fake helper below is **UNVERIFIED**.

**Read in this branch's parts, UNVERIFIED until merge:** task 143's types, `WorkIntake`,
`WorkIntakeCodeStatus` and `ActorKinds` (`task-143.md`); task 03's `FactStates`, `IsFactState`
and `MoveCauses` (`task-03.md:116-124`, `:211-219`); task 04's `fakeMoveAllowed`
(`task-04.md:861`), `workEvent` and `issueOf`; task 05's `WorkEnv`, `WorkRow`, `RunWork`,
`wRefused`, `wKinds`, `wHead`; task 15's `ImportBatch` idempotency (`task-15.md:175-177`); the
route owners' rules (146 decisions 1-8, `task-146.md:27-70`; 147's `Route`, `task-147.md:211`;
148's `CheckAnswers`, `task-148.md:271`; 156's `WebhookKinds`, `task-156.md:75`).

#### Decisions the scope left open, made here

1. **W5 state lives beside the world**, `intakeWorlds sync.Map` of `*fakeWorld` → `*fakeIntake`
   (task 115's decision 1), so no earlier struct gains a field.
2. **A principal is a view of the fake, not a user.** `(*Fake).ActingAs(a hubapi.Actor) *Fake`
   returns a copy of the client bound to the same world, and every call through it carries `a`.
   `actorOf` is a `sync.Map` of `*Fake` → `hubapi.Actor`; a `*Fake` with no entry is a person. The
   real Hub gets the same effect from the caller's token (tasks 150, 159, 160), so the rows need
   only `WorkEnv` constructors: `e.Integration(t, conn)`, `e.AppFor(t, user)`, `e.KeyFor(t, user)`.
3. **One guard, called from W1-W4's writers.** `guardActor(kind, move)` is the fake's copy of task
   145's trigger: `integration` may write no state and no revision, only `ApplySyncBatch`; `app`
   and `key` may make every hand move a person may and are refused every fact state and `done`;
   `agent` and `janitor` keep W1-W4's rules. Three call sites change (⚑): task 04's
   `MoveIssueState` and `WriteIssueRevision`, and task 05's `CreateComment`.
4. **Comments from a tracker come only through the batch.** An integration principal's direct
   `CreateComment` is 403 `forbidden` in both modes; `SyncBatch.Comments` is accepted in mode
   `in_and_comments`, each written with `author` the external author's name (task 153's
   `ExternalComment`), and in mode `in` the whole batch is 403 `forbidden` before any item applies.
5. **The quote's immutability is a shape row**, as task 115's decision 3: `WorkIntake` has no
   method that writes a request's `quote` or `requester` but `EraseRequest`, and a request reads
   byte-equal across `MarkImportant`, `AcceptRequest` and a rule change except for `important`,
   `state`, `issueId`, `closedAt` and `closedBy`.
6. **The harness rule is split.** The Hub refuses a policy naming a harness outside the catalogue
   with 403 `harness_not_allowed`, `details.harness`; refusing a *session* is the control plane's
   (task 166), which reads the policy, because the Hub starts no session. The row covers the Hub
   half only.
7. **The grid is the owner's recorded levels.** A loop version whose `bound.levels` sets any class
   above the owner's own level for that class is 403 `loop_outside_grid`, `details.class`. The fake
   reads the owner's grid from `w.agentGrid(userID)` (Phase T's autonomy record, **UNVERIFIED**
   name); an owner with no recorded grid has `ask_first` everywhere, so any level above it is out.
8. **Every write appends exactly one metadata-only event**, and no read appends; the event's data
   never holds a quote, an answer, an email address or a secret (SW.5's rule, W5's kinds).

#### Files

| Path (under `control-plane/internal/hub/hubclient/`) | Change |
|---|---|
| `fake_work_intake.go` | new: W5 state, `ActingAs`, the guard, all fifty-one `WorkIntake` methods |
| `fake_work_intake_stub.go` | **deleted** (task 143's), in the same commit |
| `fake_work.go` ⚑ (task 04) | `MoveIssueState` and `WriteIssueRevision` call `guardActor` first, two lines each |
| `fake_work_social.go` ⚑ (task 05) | `CreateComment` refuses an integration principal, two lines |
| `fake_work_intake_test.go` | new: fake-only tests (per-world state, the view, the guard table) |
| `conformance/work_intake.go` | new: `WorkRowsW5`, eighteen rows |
| `conformance/work_intake_fake_test.go` | new: `RunWork(t, fakeEnv, WorkRowsW5...)` |
| `conformance/work.go` ⚑ (task 05) | `WorkEnv` gains `Integration`, `AppFor`, `KeyFor` (decision 2) |

**The file name, corrected (2026-09-26).** Task 05 already owns `hubclient/fake_work_intake.go`
(`task-05.md:76`, `:961`: import, search, attachments, erase), so the table's two
`fake_work_intake*.go` rows are this task's **`fake_work_w5.go`** and **`fake_work_w5_test.go`**.
Nothing of task 05's file changes. Task 169 reads *"task 144's `fake_work_intake.go`"* by path
(`task-169.md:32`, `:562`, `:766`) and must read `fake_work_w5.go` instead (gap 1).

#### Decisions, continued

9. **A form submission is an env helper, not a `WorkIntake` method.** Task 148's submission is a
   page route under task 80's browser session (`task-148.md:428`), so no client method exists.
   `WorkEnv.SubmitForm(t, as, formID, answers)` calls the fake-only `(*Fake).SubmitForm` here and
   posts the page with a session in task 168. Both reach task 146's `createRequest` path.
10. **Sync reuses task 05's import body.** `ApplySyncBatch` keys each item on
    `[3]string{projectID, source, externalId}` in `w.soc().imports` and creates through
    `w.insertImported` (`task-05.md:1030-1041`, `:1078-1092`), which writes `StateTriage` and
    revision 1. So SW.6's idempotency is one map for an import and a sync, and a Linear import and a
    Jira sync of the same external id stay two issues because their `source` differs.
11. **An integration principal is not a user.** `ConnectTracker` mints the connection's
    `PrincipalID` (`ip-` in the fake only, never on the wire as a user) and records it in
    `in().principals`. `e.Integration(t, conn)` is `ActingAs(Actor{Kind: integration})` bound to it.

#### `fake_work_w5.go`, complete: state, the view and the principal

```go
package hubclient

import (
	"context"
	"slices"
	"strconv"
	"sync"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/intake"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/ids"
)

type fakeIntake struct {
	requests   map[string]*hubapi.Request          // by rq id
	order      map[string][]string                 // team id → request ids, oldest first
	rules      map[string]hubapi.IntakeRuleSet     // by team id
	forms      map[string]*hubapi.IntakeForm       // by fm id
	trackers   map[string]*hubapi.TrackerConnection // by tc id
	principals map[string]string                   // principal id → tc id (decision 11)
	runs       map[string][]hubapi.SyncRun         // tc id → runs, newest last
	webhooks   map[string]*hubapi.Webhook          // by wh id
	apps       map[string]*hubapi.App              // by oa id
	keys       map[string]*hubapi.APIKey           // by ak id
	loops      map[string]*hubapi.Loop             // by lp id
	versions   map[string][]hubapi.LoopVersion     // lp id → versions, 1 first
	policies   map[string]hubapi.TeamAgentPolicy   // by team id
}

var intakeWorlds sync.Map // *fakeWorld → *fakeIntake (decision 1)

// in is w's W5 state, made on first use. Call it with w.mu held.
func (w *fakeWorld) in() *fakeIntake {
	v, _ := intakeWorlds.LoadOrStore(w, &fakeIntake{
		requests: map[string]*hubapi.Request{}, order: map[string][]string{},
		rules: map[string]hubapi.IntakeRuleSet{}, forms: map[string]*hubapi.IntakeForm{},
		trackers: map[string]*hubapi.TrackerConnection{}, principals: map[string]string{},
		runs: map[string][]hubapi.SyncRun{}, webhooks: map[string]*hubapi.Webhook{},
		apps: map[string]*hubapi.App{}, keys: map[string]*hubapi.APIKey{},
		loops: map[string]*hubapi.Loop{}, versions: map[string][]hubapi.LoopVersion{},
		policies: map[string]hubapi.TeamAgentPolicy{}})
	return v.(*fakeIntake)
}

var actorOf sync.Map // *Fake → hubapi.Actor; no entry is a person (decision 2)

// ActingAs is a client on the same world whose every call is a's. For an app
// or a key it is a.UserID's client with the actor kind attached; for an
// integration it is the connection's principal. f.asUser is H's way of making
// a second client on one world (UNVERIFIED name; Step 1 binds it).
func (f *Fake) ActingAs(a hubapi.Actor) *Fake {
	v := f.asUser(a.UserID)
	actorOf.Store(v, a)
	return v
}

func (f *Fake) actor() hubapi.Actor {
	if a, ok := actorOf.Load(f); ok {
		return a.(hubapi.Actor)
	}
	return hubapi.Actor{Kind: hubapi.ActorPerson}
}
```

#### `hubapi/work.go` ⚑, appended: the three outside causes (complete)

Task 154 moves an issue with `hubapi.CauseOutsidePR` and `CauseOutsideMerge`
(`task-154.md:711-713`) and task 145's guard names three causes, but task 143 declares none
(checked: no `outside_` in `task-143.md`). The fake's guard needs them first, so they land here,
and task 145's SQL spells the same three words.

```go
// W5's outside-host causes (task 144; task 145's guard allows only these for
// actor kind host). Each moves forward only, as the mapped person.
const (
	CauseOutsidePush  MoveCause = "outside_push"  // → started: a pushed branch named ep-142-…
	CauseOutsidePR    MoveCause = "outside_pr"    // → in_review: an opened PR naming the issue
	CauseOutsideMerge MoveCause = "outside_merge" // → landed, with LandCommit: the merge
)

// OutsideCauseTo is each outside cause's one allowed target state.
var OutsideCauseTo = map[MoveCause]IssueState{
	CauseOutsidePush: StateStarted, CauseOutsidePR: StateInReview, CauseOutsideMerge: StateLanded,
}

func init() { MoveCauses = append(MoveCauses, CauseOutsidePush, CauseOutsidePR, CauseOutsideMerge) }
```

Appending in `init` keeps task 03's `MoveCauses` declaration untouched (task 143's own pattern for
`ImportSources`). `fakeMoveAllowed` needs no row for them: `guardActor` answers for every outside
cause before it is reached, and a person's move with one is refused there (below).

#### `fake_work_w5.go`, continued: the guard (complete)

```go
// guardActor is the fake's copy of task 145's trigger (decision 3). It runs
// first in every W1-W4 writer that can change a state or a revision, so a
// refusal writes nothing and appends no event.
func (f *Fake) guardActor(to hubapi.IssueState, cause hubapi.MoveCause, revision bool) error {
	a := f.actor()
	refuse := func() error {
		return inProblem(hubapi.CodePrincipalCannotMoveState,
			"This principal may not change an issue's state or text.", "actorKind", string(a.Kind))
	}
	_, outside := hubapi.OutsideCauseTo[cause]
	switch a.Kind {
	case hubapi.ActorIntegration: // sync creates through SW.6 and never moves or revises
		return refuse()
	case hubapi.ActorApp, hubapi.ActorKey:
		if !revision && (hubapi.IsFactState(to) || outside) { // done is a fact state
			return refuse()
		}
	case hubapi.ActorHost:
		if revision || !outside || hubapi.OutsideCauseTo[cause] != to || a.UserID == "" {
			return refuse()
		}
	default: // person, agent, janitor: W1-W4's rules, but no outside cause
		if outside {
			return refuse()
		}
	}
	return nil
}

func inProblem(code, message string, kv ...string) error {
	return problem(hubapi.WorkIntakeCodeStatus[code], code, message, kv...)
}

// inEvent is the one metadata-only event each W5 write appends (decision 8).
func (w *fakeWorld) inEvent(me, kind, teamID, projectID, subject string, data map[string]any) {
	w.workEvent(me, kind, teamID, projectID, subject, data) // task 04's helper
}
```

The three call sites (⚑), each the first line after `f.enter`:

```go
// fake_work.go, MoveIssueState (task-04.md:938)
if err := f.guardActor(move.To, move.Cause, false); err != nil {
	return hubapi.IssueHead{}, err
}
// fake_work.go, WriteIssueRevision (task-04.md:729)
if err := f.guardActor("", "", true); err != nil {
	return hubapi.IssueRevision{}, err
}
// fake_work_social.go, CreateComment (task-05.md:756): decision 4
if f.actor().Kind == hubapi.ActorIntegration {
	return hubapi.Comment{}, problem(403, "forbidden", "A tracker writes comments only through its sync batch.")
}
```

`forbidden` is H's 403 code, not a W5 code, so it goes through `problem` and not `inProblem`.
