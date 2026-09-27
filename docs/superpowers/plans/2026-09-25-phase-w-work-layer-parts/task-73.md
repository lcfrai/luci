### Task 73: the fake's W2 rules and conformance rows

**Slice:** W2. **Size:** 3 focused hours (an estimate, not a measurement). **Stream:** WH.
**Depends on** task 72 (every W2 type, validator and code, and `WorkPlan` declared standalone).
**Seam:** **SW.18**, frozen here the way WS0 froze SW.2: after this commit only added methods,
fields or routes. `Work` embeds `WorkPlan` from this task, in the same commit as the fake's
implementation and the client's stub, so `var _ hubclient.Work = hubclient.NewFake()` and
`var _ Hub = (*Client)(nil)` never fail to compile between tasks 72 and 73.

**The rules are the route tasks'.** Tasks 75-79's parts were on the tree when this part was
written, and each fixes its rule more exactly than the scope paragraph. The fake follows them, and
where a route part corrected the scope (the held move, the parent hold, neighbours by order), the
fake takes the correction and this part records it under *Corrections*. Task 80's part was **not**
on the tree: the views, drafts and export rules here are the scope's and task 72's, and task 80's
part must either match them or edit this fake in its own commit.

**Anchors, re-read on 2026-09-26** (all parts of this branch, none yet code):
`task-72.md:872-958` (`WorkPlan`, 46 methods) and `:961-995` (routes and roles), `:351-385`
(codes and `WorkPlanCodeStatus`), `:394-410` (`IssuePlan`, `IssueAlias`), `:268-309` (`BandHolds`,
`ValidCustomStateKey`, `ValidDate`, `ValidEstimate`, `FeatureBranchRef`); `task-04.md:280-326`
(`fakeWork`, `fakeIssue`, `wk()`, `workEvent`), `:333-340` (`issueOf`), `:934-1036`
(`MoveIssueState` and its hook calls); `task-05.md:103-212` (`GroupWork`, `WorkEnv`, `WorkRow`,
`RunWork`, `wRefused`, `wKinds`, `wHead`), `:276-335` (`fakeSocial`, `soc()`), `:341-352`
(`checkIssueLabels`), `:466-480` (`findRelation`, `relationEvent`), `:1301-1312` (`f.as.OrgAdmin`,
`forbidden`), `:1905-1941` (`workFakeEnv`); `task-75.md:32-72` (decisions (a)-(g)), `:153-154`
(event kinds); `task-76.md:31-70`, `:139-155`, `:166-169`; `task-77.md:34-95`, `:241-248`;
`task-78.md:33-94`, `:210-215`; `task-79.md:33-93`, `:284-291`, `:298-303`. Hub part 1:
`phase-h-hub-parts/task-05.md:38-51` (`Env.Advance`), `task-04.md:92` (`CreateBrief`).
**UNVERIFIED until merge:** H05's `fake_briefs.go` and `fake_feed.go` (the fake's brief and feed
internals; this part calls only `f.CreateBrief` and `w.workEvent`), the fake's evidence store that
task 78's *settled* reads (named `w.latestVerdict` here), and `hubapi.Me.Email`.

#### Decisions the scope left open, made here

**(a) The fake follows the route parts, not the scope, where they differ.** The held evidence move
is accepted and recorded, not applied (task 75 (f)); a hand `done` on a held issue is `409
held_by_substate`; clearing an `after_landed` sub-state re-runs the proof decision and closes the
issue as the clearer (75 (e)). Open children hold a parent's evidence move the same way, and a
container parent rolls up with cause `children_closed` (78 (c)). Rank neighbours need order, not
adjacency (79 (i)). Rollover writes a `removed` scope entry on the closing cycle only (77 (e)).

**(b) One new file, and eight hook lines in three W1 files.** The rules live in
`fake_work_plan.go`. W1's move, create, number read and label check call into it through named
hooks, one line each, so the W1 files keep their shape and a reviewer reads each W2 rule in one
place. The hooks are listed with their diff below; the scope named none of these files, recorded
under *Corrections*.

**(c) The fake's rank keys are its own; the rows assert order only.** Task 79's rank functions
live in the server's `store` package, which the fake cannot import (it would pull the database
driver into every desktop-side test binary). The fake keeps a twelve-line bisection
(`fkRankBetween`) and a bottom join, and the conformance rows assert only that a ranked issue sorts
between its neighbours on a fresh read, L-14's promise. If task 79's four functions move to
`hubapi` (they are pure), the fake's copy is deleted; reported as a gap.

**(d) The janitor is a fake-only method, driven by a new `WorkEnv.Janitor`.** `(*Fake).RunWorkJanitor()`
runs task 77's sweep once (the cycle walk, rollover, stale proposals, draft expiry) at the fake's
`now`. `WorkEnv` gains `Janitor func(t *testing.T)`; the fake's env sets it, and a nil `Janitor`
makes the four janitor rows `t.Skip` and say so, exactly as `Advance: nil` does. Task 82 decides
whether the real Hub gets a trigger or skips them.

**(e) A second team comes from the outsider.** `MoveIssue` needs a member of both teams. The row
helper `e.otherTeam(t)` has the outsider create team U and invite the member by the email
`Member.Me` answers; the member's `Me` binds it (`task-05.md:1925-1927`'s idiom). A project of U is
then the target.

**(f) Drafts write no event.** The feed is visible to a team's readers (SW.5); an event would leak
a private draft's existence, which L-33's `404` exists to hide.

**(g) Event kinds are the route tasks', spelled as the fake's own constants** until tasks 75-79
land them in task 13's registry (task 04's decision (e), the same way): `team.states_changed`,
`issue.sub_state_changed` (75); `feature.created`, `feature.updated`, `issue.feature_set`,
`issue.promoted` (76); `team.cadence_set`, `cycle.created`, `cycle.updated`, `cycle.closed`,
`issue.cycle_set`, `closure.proposed`, `closure.answered` (77); `issue.parent_set`,
`template.created`, `template.updated`, `template.archived`, `label_group.created`,
`issue.rolled_up` (78); `issue.planned`, `issue.ranked`, `issue.moved`, `issue.blocks_resolved`
(79). Views have no kind in any part; the fake spells `view.created`, `view.updated`,
`view.archived` and `view.team_default_set`, and task 80 confirms or renames them (a gap).

**(h) New causes are the route tasks' strings.** `children_closed` (78 (e)) and `stale_policy`
(77 (f)) are `hubapi.MoveCause` values the fake spells as `fkCauseChildrenClosed` and
`fkCauseStalePolicy`; W1's `fakeMoveAllowed` never sees them, because only the fake's own roll-up
and janitor write them, never a caller.

**(i) The criterion-settled read is a hook.** Task 78 (b) reads *settled* from the latest evidence
record per `(briefKey, criterionId)` in the project. The fake's evidence store is H05's
(`w.evidenceRecord`, `task-04.md:890`); `w.latestVerdict(projectID, briefKey, criterionID)` is
the one function that walks it, written against H05's merged names at Step 1 (**UNVERIFIED**).

**Files** (under `control-plane/internal/hub/hubclient/`):

| Path | Change |
|---|---|
| `work.go` ⚑ (task 04, 72) | `Work` embeds `WorkPlan`: one line |
| `client_work_plan_stub.go` | new: the HTTP client's 46 `WorkPlan` methods answer `ErrNotConfigured`; task 82 deletes it |
| `fake_work_plan.go` | new: the W2 state, the 46 methods, the hooks and `RunWorkJanitor` |
| `fake_work.go` ⚑ (task 04) | five hook lines: create, number read, the move (three) |
| `fake_work_social.go` ⚑ (task 05) | three lines: org labels in `checkIssueLabels`, the exclusive check, org labels in `Labels` |
| `fake_work_plan_test.go` | new: the fake-only tests (rank keys, janitor idempotence, the hooks) |
| `conformance/work.go` ⚑ (task 05) | `WorkEnv` gains `Janitor` |
| `conformance/work_plan.go` | new: `WorkRowsW2`, rows `W-26` to `W-43`, their helpers |
| `conformance/work_fake_test.go` ⚑ (task 05) | the env sets `Janitor`; one `RunWork` call for W2; the order test covers both slices |

#### Interfaces, complete: the `work.go` delta

```diff
 type Work interface {
 	WorkIssues
+	WorkPlan // SW.18, frozen from task 73: afterwards only added methods
 }
```

`WorkPlan` itself is task 72's (`task-72.md:882-958`), unchanged here.

#### `client_work_plan_stub.go`, complete

```go
package hubclient

import (
	"context"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
)

// The HTTP client's WorkPlan methods, as placeholders in client_work_stub.go's
// idiom: every one answers ErrNotConfigured. Task 82 deletes this file in the
// commit that adds methods_work_plan.go. Task 16 deleted client_work_stub.go
// and its aliases in W1, so these are this file's own, none shared.

type (
	ph  = hubapi.IssueHead
	ft  = hubapi.Feature
	cy  = hubapi.Cycle
	tpl = hubapi.Template
	lg  = hubapi.LabelGroup
	vw  = hubapi.View
	df  = hubapi.Draft
	pp  = hubapi.PageRequest
)
```

The 46 bodies are one line each, generated by the same `go generate` comment task 16 removed:

```go
func (c *Client) PutCustomStates(context.Context, string, []hubapi.CustomState) ([]hubapi.CustomState, error) {
	return nil, ErrNotConfigured
}
// …45 more, each returning the zero value and ErrNotConfigured, in WorkPlan's
// declaration order (task-72.md:872-958). None is called by any test: the
// conformance rows run against NewFake() here and against a configured Client
// in task 82.
```

#### The fake (`fake_work_plan.go`), complete in its state and its helpers

`fakeWork` (task 04) gains one embedded struct so the W2 state is one field, not eleven, and task
04's file needs no edit beyond the five hook lines:

```go
// fake_work_plan.go — the W2 half of the in-memory Work fake. State hangs off
// fakeWork.plan; every method takes w.mu, already held by the W1 helpers.

type fakeWorkPlan struct {
	states    map[string][]hubapi.CustomState // teamID -> the team's list, in order
	subState  map[string]string               // issueID -> custom state key
	features  map[string]*hubapi.Feature      // ft- id -> feature
	branchRef map[string]string               // projectID+"\x00"+ref -> ft- id (the reservation)
	cycles    map[string]*hubapi.Cycle        // cy- id -> cycle
	cycleNo   map[string]int                  // teamID -> the last number issued (gapless)
	cadence   map[string]hubapi.Cadence       // teamID -> cadence
	scope     map[string][]hubapi.ScopeEntry  // cycleID -> entries, append only
	proposals map[string]*hubapi.ClosureProposal
	templates map[string]*hubapi.Template // tp- id
	groups    map[string]*hubapi.LabelGroup
	views     map[string]*hubapi.View  // vw- id
	drafts    map[string]*hubapi.Draft // df- id
	aliases   map[string]string        // projectID+"#"+number -> is- id, never deleted
	rank      map[string]string        // issueID -> fractional key
	plan      map[string]hubapi.IssuePlan
	parent    map[string]string   // issueID -> parent issueID
	children  map[string][]string // parentID -> children, in rank order
	digestAt  map[string]time.Time // personID -> the last digest sent
	now       func() time.Time     // the env's clock (task 05's WorkEnv.Now)
}
```

Five helpers, each used by more than one method, each one screenful:

```go
// bandOf answers the human band a fact state sits in, and false when the state
// takes no sub-state at all (hubapi.BandHolds, task-72.md:268).
func (w *fakeWork) bandOf(state string) (hubapi.Band, bool)

// holdsDone reports whether issue id carries an after_landed sub-state, which
// refuses the evidence move with 409 held_by_substate until a person clears it
// (05 §3.3, task 75 (e)).
func (w *fakeWork) holdsDone(id string) bool

// rankBetween returns a key strictly between lo and hi (either may be ""),
// and false when the gap is exhausted, at which point the caller rebalances
// the whole team and writes one issue.ranked per moved row (L-14, L-103).
func (w *fakeWork) rankBetween(lo, hi string) (string, bool)

// reserveRef claims refs/heads/<slug> for a feature inside project p, and
// answers false when the ref is already a feature's or a branch brief's
// (hub-contract.md:253, 04 §2.5).
func (w *fakeWork) reserveRef(p, ref, featureID string) bool

// rollCycle moves every issue of cycle c that is not done, cancelled or
// duplicate into cycle next, one cycle.rolled event per issue, actor nil
// (L-60). It is idempotent: a second call with the same cycle writes nothing.
func (w *fakeWork) rollCycle(c, next string) int
```

#### The conformance rows (`conformance/work_plan.go`), complete

Task 05's shapes are reused unchanged: `WorkRow{Name, Run}`, `RunWork(t, env, rows)`, `wRefused`,
`wKinds`, `wHead`. `WorkEnv` gains one field, because the cycle rows need the janitor to run on
demand:

```go
// conformance/work.go ⚑ — one added field, nothing renamed (SW.18 is frozen).
type WorkEnv struct {
	// …task 05's fields, unchanged…

	// Janitor runs the implementation's scheduled work once, synchronously, and
	// returns when it is done. The fake calls rollCycle and the cycle-ahead
	// step; the real Hub (task 82) runs task 77's janitor under its advisory
	// lock. A nil Janitor skips the four cycle rows with t.Skip, named.
	Janitor func(ctx context.Context) error
}
```

`WorkRowsW2` is a `[]WorkRow` of eighteen rows. Each is stated as its rule, its call and its
assertion; the code is one function per row in the file, in this order:

| Row | The rule it fixes | Asserts | Mutant it kills |
|---|---|---|---|
| `W-26` | a sub-state is valid only inside its head's band (05 §3.3) | `PutCustomStates` *Triage done* (`before_started`) and *In QA* (`after_landed`); `SetSubState` *In QA* on a `todo` issue → `wRefused(422, "sub_state_band")` | bands unchecked |
| `W-27` | an `after_landed` sub-state holds the evidence `done` move | issue `landed`, *In QA* set, evidence move → `wRefused(409, "held_by_substate")`; `wHead(state)` still `landed` | `after_landed` decorative |
| `W-28` | clearing it closes the issue as the clearer | `SetSubState(nil)` → head `done`, `wKinds("issue.sub_state_changed", "issue.state_changed")`, the state event's `actor` the clearer, `cause` `evidence` | `actor: null`; the issue stranded at `landed` |
| `W-29` | a state move out of the band clears the sub-state in the same write | `before_started` sub-state, `backlog`→`todo` keeps it, `todo`→`started` clears it, one event pair | the sub-state surviving `started` |
| `W-30` | creating a feature reserves the ref and writes its branch brief at once | `CreateFeature(p, "login")` → `ft-`, `BranchRef` `refs/heads/login`, `BriefID` non-empty; a second feature on the ref → `wRefused(409, "brief_exists")` | a second feature on one ref |
| `W-31` | *promote to feature* keeps the issue's `serves` on its own revision | `PromoteToFeature(is-)` → a new `ft-` whose first child is the issue; the issue's revision `serves` byte-equal before and after; the feature's brief names no criterion of its own | `serves` copied onto the feature (L-23) |
| `W-32` | cycle numbers are gapless per team | `PutCadence(team, ahead: 3)`, `Janitor` → numbers `1,2,3`; a second `Janitor` adds none; a second team starts again at `1` | a global sequence |
| `W-33` | rollover moves only unfinished work, as nobody | cycle 1 with `done`, `cancelled`, `duplicate` and `todo` issues; `Janitor` past `endsOn` → only the `todo` issue is in cycle 2, one `cycle.rolled` for it, `actor` nil | finished work rolled; the actor set |
| `W-34` | a `started` move puts the issue in the current cycle, as the person | `MoveIssueState(started)` → `wHead(cycleId)` the current cycle, and the `issue.cycle_changed` event's actor the mover (L-61) | `actor: null` on an act a person did |
| `W-35` | a cycle change after `startsOn` is scope, recorded | `Janitor` starts cycle 1, then `SetIssueCycle` → one `ScopeEntry{kind: "added"}` on cycle 1 and `{kind: "removed"}` on the issue's old cycle (task 77 (e)) | silent scope change (L-63) |
| `W-36` | a parent chain may not loop | A→B then B→A → `wRefused(422, "parent_loop")`; a three-deep chain closed at the root → the same | cycles accepted (L-20) |
| `W-37` | a parent stays out of `done` while a child is open or a criterion unsettled | hand `done` with one `todo` child → `wRefused(409, "children_open")`; the last child closed with a `contested` criterion → the parent unchanged | the parent closing early (L-21) |
| `W-38` | two labels of one exclusive group is refused | group *Type* `exclusive`; `SetIssueLabels(bug, feature)` → `wRefused(422, "exclusive_label_group")`; a non-exclusive group takes both | the last label winning (L-18) |
| `W-39` | an estimate must sit on the team's scale | Fibonacci team, `PatchIssuePlan(estimate: 4)` → `wRefused(422, "estimate_off_scale")`; `0` → the same unless `estimateZero`; `5` → 200 | any integer (L-16) |
| `W-40` | rank sorts between the neighbours named | `Rank(C, after: A, before: B)` → a fresh `ListIssues` reads A, C, B; a rank sent in the body is ignored | client-trusted rank (L-14) |
| `W-41` | a move renumbers and the old alias redirects | `MoveIssue(is-, Q)` → the same `is-`, `Number` the next in Q; `IssueByNumber(P, old)` → the moved issue with `MovedFrom`; a new issue in P never takes the old number | the number reused, or 404 (L-35) |
| `W-42` | closing a blocker turns `blocks` into `relates` | B blocks A; B → `done` → one `issue.relation_changed`, kind `relates`, `actor` the closer; a `todo` B keeps `blocks` | the relation deleted (L-25) |
| `W-43` | the private surfaces answer 404, not 403, and only an admin sets a default | a second person's `GetDraft` → `wRefused(404, "draft_not_found")` (L-33); a member's `PutViewTeamDefault` → `wRefused(403, "forbidden")` (L-105); an answered `ClosureProposal` closes only the ids named, as the assignee (L-11) | `403` leaking a draft's existence; any member setting the team default |

#### The hook lines into tasks 04 and 05 (complete, and no more than these)

```go
// fake_work.go ⚑ (task 04)
// 1. CreateIssue, after the number is taken: w.plan.rank[id] = w.bottomRank(teamID)
// 2. CreateIssue, at the end:              w.plan.plan[id] = hubapi.IssuePlan{}
// 3. IssueByNumber, on a miss:             if to, ok := w.plan.aliases[key]; ok { … MovedFrom }
// 4. MoveIssueState, before the write:     if to == "done" && cause == "evidence" &&
//                                             (w.holdsDone(id) || w.openChildren(id) > 0) { refuse }
// 5. MoveIssueState, after the write:      w.onClosed(id, actor)  // cycle, blockers, roll-up

// fake_work_social.go ⚑ (task 05)
// 6. checkIssueLabels: org labels (team_id nil) resolve too
// 7. checkIssueLabels: two labels of one exclusive group -> 422 exclusive_label_group
// 8. Labels: the team's labels followed by the org's, each list in name order
```

`onClosed` is this file's, not task 04's: it moves the blockers (W-42), rolls the parent up (W-37)
and leaves the cycle alone (a closed issue keeps the cycle it was in, 04 §2.5).

#### Tests to write first, each with the mutant it kills

`conformance/work_fake_test.go` ⚑ gains one `RunWork(t, workFakeEnv(t), conformance.WorkRowsW2)`
call and sets `Janitor`; the order test (task 05) covers both slices, so a row added to either
without a name is caught. `fake_work_plan_test.go` is new and holds only what is the fake's own:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestW2ConformanceAgainstTheFake` | all eighteen rows green against `NewFake()` | a rule stated in the table and implemented nowhere |
| `TestRankBetweenStaysShort` | 1,000 inserts between the same two neighbours → every key ≤ 32 bytes and the read order unchanged | a key that grows without bound |
| `TestRankBetweenRefusesAnExhaustedGap` | `rankBetween("a", "ab")` at the 32-byte limit → `false`, and the caller's rebalance writes one event per moved row | a silent collision |
| `TestTheJanitorIsIdempotent` | `Janitor` twice past `endsOn` → the same cycle numbers, one `cycle.rolled` per issue | a second run rolling again |
| `TestTheFakeRefusesAnUnknownSubState` | `SetSubState("nope")` → 422 `sub_state_unknown` | a free-text sub-state |
| `TestEveryWorkPlanMethodIsImplemented` | reflection over `hubclient.WorkPlan` → the fake implements all 46 and none panics with a zero request | a method left as a `panic("TODO")` |
| `TestTheStubAnswersNotConfigured` | every `WorkPlan` method on `&Client{}` → `ErrNotConfigured` | a stub that returns nil, nil and reads as success |

#### Steps

- [ ] **Step 1: Check the base.** Task 72 merged (`WorkPlan`, the codes, `BandHolds`,
  `ValidEstimate`, `ValidDate`, `FeatureBranchRef`); task 04's `fakeWork` field names and
  `MoveIssueState` hook points; task 05's `WorkEnv` and `RunWork`; whether H05's `fake_briefs.go`
  exposes `CreateBrief` as the anchor says (**UNVERIFIED** at writing).
- [ ] **Step 2: The rows first.** Write `conformance/work_plan.go`'s eighteen rows and the one
  `RunWork` call. Run them: eighteen failures, each naming its rule.
- [ ] **Step 3: The seam, in one commit.** `work.go`'s `Work` embeds `WorkPlan`,
  `fake_work_plan.go` and `client_work_plan_stub.go` together, so the two `var _` assertions never
  break the build between commits.
- [ ] **Step 4: Green.** `fake_work_plan_test.go`, then the conformance call, then task 04's and
  05's suites unchanged (an issue with no sub-state, parent or feature takes none of the new
  paths).
- [ ] **Step 5: Commit by pathspec**, naming task 04's and 05's files as touched, with the eight
  hook lines listed in the message.

#### Acceptance

```bash
cd control-plane && go build ./internal/hub/hubclient/...                                  # ok
go test ./internal/hub/hubclient/... -race -count=1                                        # ok
go test ./internal/hub/hubclient/conformance/ -race -count=1 -run '^TestW2ConformanceAgainstTheFake$' -v 2>&1 | grep -c '^    --- PASS'   # 18
go test ./internal/hub/hubclient/ -race -count=1 -run '^TestEveryWorkPlanMethodIsImplemented$'       # ok
grep -c 'W-[0-9]\+' internal/hub/hubclient/conformance/work_plan.go                        # 18 or more
grep -rn 'panic("TODO")' internal/hub/hubclient/fake_work_plan.go | wc -l                  # 0
```

Expected: `ok`, `18`, `18`, `0`. None has been run; no file exists yet (OD-27).

#### What this task does not claim

- **Not the real Hub's behaviour.** Every row here is run against `NewFake()` only. Task 82 runs
  the same eighteen against the HTTP client and a live Hub with `-tags integration`; until it is
  green, a row passing here is evidence about the fake and nothing else (OD-27).
- **Not task 80's rules.** Task 80's part was not on the tree when this was written. The views,
  drafts, export and page behaviour in `W-43` follow the scope and task 72's types; if task 80
  decides otherwise, task 80 edits this fake in its own commit and says so.
- **Not the digest** (task 81): `digestAt` exists so the fake can hold a send time, and no row
  reads it.
- **Not performance.** `rankBetween` is tested for key length, not for speed; a rebalance over a
  team of 10,000 issues has not been timed by anyone.
- **Not a frozen route list.** SW.18 is frozen for *removal* from this commit: methods, fields and
  routes may still be added by tasks 75-82, and each addition names this part.

#### Corrections to the scope

1. **`WorkEnv` gains `Janitor`**, and the four cycle rows (`W-32`, `W-33`, `W-34`, `W-35`) skip,
   named, when it is nil. The scope assumed the cycle rules could be driven through the methods
   alone; rollover is the janitor's, so the environment has to expose it.
2. **Rank neighbours need order, not adjacency** (task 79 (i)). The scope's *"between two
   neighbours"* reads as adjacency; `W-40` asserts order.
3. **The held `done` is accepted and recorded, not applied** (task 75 (f)), so `W-27` asserts the
   409 at the route and the evidence still stored; the scope's *"refuses the evidence move"* is
   the caller's view, not the record's.
4. **Rollover writes a `removed` scope entry on the closing cycle only** (task 77 (e)), so `W-33`
   counts one entry, not two.
5. **A container parent's roll-up carries cause `children_closed`** (task 78 (c)); `W-37` names
   it, and the scope's bare *"a parent stays out of `done`"* does not.
6. **Eight hook lines, not five.** The scope counted five in `fake_work.go`; the exclusive check
   and org labels need three more in `fake_work_social.go`, listed above.

**Parity rows served:** none alone. These rows are the shared proof of the Hub halves of **L-8**,
**L-11**, **L-14**, **L-16**, **L-18**, **L-20**, **L-21**, **L-23**, **L-25**, **L-33**,
**L-35**, **L-60**, **L-61**, **L-63** and **L-105**, closed by tasks 75-80 with 84-94 and
recorded by task 97.
