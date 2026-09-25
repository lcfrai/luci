### Task 105: Fleet: §3.7 rules 2 and 3 through Orchestrators; cross-owner `blocks` seeds a `sequence` proposal

Slice **W3** (agents on the work), about 3 hours (an estimate, not a measurement). Depends on task
98 (`task.issueId`, `issue.delegated`, `blocker.landed`), Phase T's TM-26 (the turn queue,
`turns.Queue.Enqueue`), TM-29 (`protocol.Action`, `Rules`) and TM-30 (`escalate.Table`, `KeyOf`).
Uses SW.5's `issue.related` and `issue.updated` (task 13, `task-13.md:120-170`), SW.19's org switch
(task 99) and task 65's reassign *decide*. Every Phase T path is `cp/`→`control-plane/` and
**UNVERIFIED until `feat/team-mode` is sync-merged here** (rule H2).

05 §3.7 says two things that only work if the right Orchestrator hears them:

1. **Rule 3: an edit across owners is a proposal, never an act.** When Alice adds *EP-150 blocks
   EP-142* and Dylan's agents are on EP-142, nothing pauses. Fleet tells **Alice's** Orchestrator,
   the editor's own, and her Orchestrator opens a `sequence` proposal on a Phase T thread with
   Dylan's (TM-05 §6.2, §6.4). Dylan's agents keep running until his side accepts.
2. **Rule 2: a hand-over winds down the former owner's agents, through the former owner's
   Orchestrator.** When Dylan answers task 65's *hand over the worktree to Alice*, Fleet tells
   **Dylan's** Orchestrator, which stops his workers at their next checkpoint (`panacea worker
   stop`, TM task 18's verb row `task-18.md:144`). Alice's Orchestrator is told nothing: nobody
   steers another person's agents (`OD-16`).

**Decisions made here, so nobody re-derives them:**

1. **"Turn origin" is Phase T's word for who started a turn, and has two values.** TM-26's
   `turns.Origin` is `human` or `fleet` (`task-26.md:158-163`). A relation or a hand-over is a
   Fleet-started turn, so its origin is `fleet`, and the scope's *origin `relation`* and *origin
   `handover`* are **event kinds**: `work.relation` and `work.handover`, appended to TM-09's
   `model.EventKinds` beside task 98's two. That is also what makes rule 7 of Phase T hold: in a
   Fleet-started turn an Orchestrator may only propose, inform and ask (Phase T plan `:148`).
2. **The editor's Orchestrator is seeded, and only when both sides have agents.** Seeded when
   the actor is the assignee of one side, the other side's assignee is someone else, and both
   assignees have a live task bound to their issue (task 98's `issueId`). Otherwise Fleet does
   nothing and the Hub's recorded relation reaches the other owner as task 65's Desk *inform*.
   An actor who is neither side's assignee seeds nothing: two other people's agents are theirs.
3. **`relates` seeds nothing; `blocks` asks for `sequence`; `duplicates` asks whose worktree
   survives.** The event says which; the Orchestrator's turn template (TM-26) does the asking.
   The resolution a `duplicates` thread reaches is always a *decide* for both humans: one row
   appended to TM-30's table, **locked** (`OD-24`'s always-decide), keyed on one added
   `protocol.Action.Relation` field that `resolve.go` fills from the thread's two issues.
4. **A hand-over is read from the assignee moving, not from a new Hub record.** Task 65's reassign
   record is itself a gap there (no Hub route yet), and a declined reassign never moves the
   assignee. So `issue.updated` with `assignee` in `fields` (`task-13.md:150-152`), on an issue
   that has live tasks bound to an owner who is no longer its assignee, is the hand-over; each such
   owner's Orchestrator gets one `work.handover`. The new assignee's Orchestrator gets nothing.
5. **The switch is read on every event, and fails closed.** SW.19's `AgentSwitch` off, or a failed
   read, enqueues nothing and returns the reason; the relation stays recorded on the Hub (it is a
   person's edit). Task 109 later puts its cached read behind the same interface.
6. **Events carry ids only.** `Ref` is `relation:<kind>:<from is->:<to is->` or the bare `is-…`;
   no title or text, per TM-09's rule that an event carries no text (`task-09.md:237-241`).
7. **The wake is task 98's.** After a `sequence` resolution, the waiting side is woken by task 98's
   `blocker.landed` when the first issue lands; this task adds no second path for it.

**Files:**
- `control-plane/internal/fleet/work/relations.go` (+ `relations_test.go`) — `OnRelated` (new;
  **path UNVERIFIED**: the skeleton's file map has no Fleet row for W3's deltas)
- `control-plane/internal/fleet/work/handover.go` (+ `handover_test.go`) — `OnAssigneeChanged` (new)
- `control-plane/internal/fleet/model/events.go` ⚑ (TM-09; task 98 also appends) — two kinds (modified)
- `control-plane/internal/fleet/protocol/{messages.go,resolve.go}` ⚑ (TM-28, TM-29) — `Action.Relation`,
  filled through one registered hook (modified)
- `control-plane/internal/fleet/escalate/rules.go` ⚑ (TM-30) — one locked row and one `KeyOf` case
  (modified; `rules_work_test.go` new)
- `control-plane/internal/fleet/delegation/feed.go` ⚑ (TM-11; task 98 maps its kinds here) — the two
  handlers called from the feed reader (modified)

#### Interfaces — complete: `control-plane/internal/fleet/work/relations.go`

```go
// Package work is Fleet's side of the work layer: it turns Work feed events into Orchestrator
// events. It holds no Hub record (Phase T rule 2) and never stops, pauses or messages an agent:
// it only enqueues, and the Orchestrator's own turn does the rest.
package work

import (
	"context"
	"fmt"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/model"
)

// The two kinds this task appends to model.EventKinds (TM-09), beside task 98's two.
const (
	EventWorkRelation model.EventKind = "work.relation" // Ref: RelationRef(kind, from, to)
	EventWorkHandover model.EventKind = "work.handover" // Ref: the bare is-… handed over
)

// Why is a closed set: every non-enqueue says which rule stopped it, for the audit and the tests.
type Why string

const (
	WhySeeded        Why = "seeded"
	WhyRelates       Why = "relates_seeds_nothing"
	WhySameOwner     Why = "same_owner"
	WhyNotAssignee   Why = "actor_owns_neither_side"
	WhyNoAgents      Why = "not_both_sides_have_agents"
	WhySolo          Why = "no_orchestrator" // solo mode: task 65's inform is the whole path
	WhyAgentsOff     Why = "agents_disabled"
	WhyNoFormerOwner Why = "no_former_owner_with_agents"
)

type Enqueuer interface { // TM-26's *turns.Queue
	Enqueue(ctx context.Context, e model.Event) error
}

type Directory interface {
	// OrchestratorOf answers the owner's Orchestrator in the org; false in solo mode.
	OrchestratorOf(ctx context.Context, org string, owner model.Owner) (model.AgentID, bool, error)
	// BoundOwners answers the owners with a live task bound to the issue (task 98's issueId).
	BoundOwners(ctx context.Context, issueID string) ([]model.Owner, error)
}

type Assignees interface { // hubclient.Work's issue head, read as Fleet's principal
	AssigneeOf(ctx context.Context, issueID string) (model.Owner, error) // "" when unassigned
}

type Switch interface { // SW.19's AgentSwitch through task 99's GetAgentSwitch; task 109 caches it
	AgentsEnabled(ctx context.Context, org string) (bool, error)
}

type Deps struct {
	Queue     Enqueuer
	Dir       Directory
	Assignees Assignees
	Switch    Switch
	Now       func() time.Time
}

// Related is task 13's issue.related, as the feed reader (TM-11) hands it on: the subject is the
// issue the row was written on, so for blocks, From blocks To.
type Related struct {
	Org, Kind, From, To string
	Actor               model.Owner
}

type Outcome struct {
	Enqueued []model.AgentID
	Why      Why
}

func RelationRef(kind, from, to string) string { return fmt.Sprintf("relation:%s:%s:%s", kind, from, to) }

func contains(xs []model.Owner, x model.Owner) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
```

```go
// OnRelated applies §3.7 rule 3 (decision 2). The order matters: the switch is read first so an
// off switch never reaches the directory, and every return names its Why.
func (d Deps) OnRelated(ctx context.Context, ev Related) (Outcome, error) {
	if ev.Kind == "relates" {
		return Outcome{Why: WhyRelates}, nil
	}
	on, err := d.Switch.AgentsEnabled(ctx, ev.Org)
	if err != nil {
		return Outcome{Why: WhyAgentsOff}, err // fail closed (decision 5)
	}
	if !on {
		return Outcome{Why: WhyAgentsOff}, nil
	}
	from, err := d.Assignees.AssigneeOf(ctx, ev.From)
	if err != nil {
		return Outcome{}, err
	}
	to, err := d.Assignees.AssigneeOf(ctx, ev.To)
	if err != nil {
		return Outcome{}, err
	}
	if from == "" || to == "" || from == to {
		return Outcome{Why: WhySameOwner}, nil
	}
	var mineIssue, theirs string
	var other model.Owner
	switch ev.Actor {
	case from:
		mineIssue, theirs, other = ev.From, ev.To, to
	case to:
		mineIssue, theirs, other = ev.To, ev.From, from
	default:
		return Outcome{Why: WhyNotAssignee}, nil
	}
	mine, err := d.Dir.BoundOwners(ctx, mineIssue)
	if err != nil {
		return Outcome{}, err
	}
	their, err := d.Dir.BoundOwners(ctx, theirs)
	if err != nil {
		return Outcome{}, err
	}
	if !contains(mine, ev.Actor) || !contains(their, other) {
		return Outcome{Why: WhyNoAgents}, nil
	}
	orch, ok, err := d.Dir.OrchestratorOf(ctx, ev.Org, ev.Actor)
	if err != nil || !ok {
		return Outcome{Why: WhySolo}, err
	}
	e := model.Event{Orchestrator: orch, Kind: EventWorkRelation,
		Ref: RelationRef(ev.Kind, ev.From, ev.To), At: d.Now()} // Author "": not a human.message
	if err := d.Queue.Enqueue(ctx, e); err != nil {
		return Outcome{}, err
	}
	return Outcome{Enqueued: []model.AgentID{orch}, Why: WhySeeded}, nil
}
```

#### Interfaces — complete: `control-plane/internal/fleet/work/handover.go`

```go
package work

import (
	"context"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/model"
)

// AssigneeMoved is task 13's issue.updated with "assignee" in Fields, after the feed reader has
// read the new assignee (decision 4). A decline moves nothing, so it never arrives here.
type AssigneeMoved struct {
	Org, IssueID string
	NewAssignee  model.Owner // "" when unassigned
}

// OnAssigneeChanged applies §3.7 rule 2's wind-down: every former owner with live tasks bound to
// the issue gets one work.handover on their own Orchestrator; the new assignee's gets nothing.
func (d Deps) OnAssigneeChanged(ctx context.Context, ev AssigneeMoved) (Outcome, error) {
	on, err := d.Switch.AgentsEnabled(ctx, ev.Org)
	if err != nil || !on {
		return Outcome{Why: WhyAgentsOff}, err
	}
	owners, err := d.Dir.BoundOwners(ctx, ev.IssueID)
	if err != nil {
		return Outcome{}, err
	}
	var out Outcome
	for _, o := range owners {
		if o == ev.NewAssignee {
			continue
		}
		orch, ok, err := d.Dir.OrchestratorOf(ctx, ev.Org, o)
		if err != nil {
			return out, err
		}
		if !ok {
			continue // solo: task 65's checkpoint-then-stop on the desktop is the whole path
		}
		e := model.Event{Orchestrator: orch, Kind: EventWorkHandover, Ref: ev.IssueID, At: d.Now()}
		if err := d.Queue.Enqueue(ctx, e); err != nil {
			return out, err
		}
		out.Enqueued = append(out.Enqueued, orch)
	}
	if len(out.Enqueued) == 0 {
		out.Why = WhyNoFormerOwner
	} else {
		out.Why = WhySeeded
	}
	return out, nil
}
```

The Orchestrator's turn template (TM-26's `template.go`) reads a `work.handover` event as: *your
owner handed issue `is-…` to someone else; stop each of your workers bound to it at its next
checkpoint with `panacea worker stop`.* The template's words are Phase T's to write; this task
names the kind and its `Ref`, and a gap asks TM-26 for the line.

#### Interfaces — complete: the registrations in Phase T's files

```go
// fleet/model/events.go (TM-09) — two lines appended to EventKinds; Valid() and the store's
// CHECK (TM-10, which reads EventKinds) follow. Task 98's two kinds sit beside them.
//	EventWorkRelation EventKind = "work.relation"
//	EventWorkHandover EventKind = "work.handover"
// work.EventWorkRelation and work.EventWorkHandover are aliases of these two, so the work
// package and the model can never disagree on the string.
// Neither kind is Immediate (task-26.md:181-184): both coalesce for up to 60 s like task 98's.

// fleet/protocol (TM-29) — one added Action field and one registered hook.
//	type Action struct { …; Relation string } // "" | "blocks" | "duplicates", between the thread's two issues
//
// RelationOf is nil until this task wires it; nil means Relation "". resolve.go sets
// a.Relation = RelationOf(ctx, t) before it calls Rules.Classify, and nowhere else.
var RelationOf func(ctx context.Context, t Thread) (string, error)
```

```go
// fleet/work/threads.go — the hook's body, in this task's package (it is the part of
// relations.go's file group that reads a thread). A thread opened in a work.relation turn has
// both issues attached (TM-05 §6.4); the relation is re-read from the Hub, never cached.
type Relations interface { // hubclient.Work's issue head, relations only
	RelationBetween(ctx context.Context, a, b string) (kind string, err error) // "" when none
}

func RelationHook(r Relations) func(context.Context, protocol.Thread) (string, error) {
	return func(ctx context.Context, t protocol.Thread) (string, error) {
		a, b, ok := t.Subject.Issues() // TM-28's Subject; its accessor is read on the sync base
		if !ok {
			return "", nil
		}
		k, err := r.RelationBetween(ctx, a, b)
		if err != nil {
			return "duplicates", err // fail toward asking: an unreadable relation decides
		}
		if k == "blocks" || k == "duplicates" {
			return k, nil
		}
		return "", nil
	}
}
```

```go
// fleet/escalate/rules.go (TM-30) — one row appended to Table and one case at the top of KeyOf.
//	{"duplicates-survivor", protocol.VerdictDecide, Both, true, "one worktree's work is discarded"},
//
//	func KeyOf(a protocol.Action) string {
//		if a.Relation == "duplicates" {
//			return "duplicates-survivor" // before the resolution switch: any resolution decides
//		}
//		switch a.Resolution { … } // unchanged
//	}
```

The row is **locked**, as `proceed-both-hm` and `build-on` are (`task-30.md:200-203`): no team
floor or user row may turn it into an act, and task 30's own rule that *nothing turns decide into
act* already holds that. A `blocks` thread keeps task 30's `sequence-short` or `sequence-long`
row unchanged: its relation adds nothing to the ruling, only the event that started the thread.

#### Tests to write first — `control-plane/internal/fleet/work/relations_test.go` and `handover_test.go`

```go
package work

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/fleet/model"
)

const (
	org             = "org-1"
	ep142, ep150    = "is-3f2a91c07b1e4d2a", "is-0000000000000150"
	dylan, alice    = model.Owner("u-d1"), model.Owner("u-a11ce")
	dylanO, aliceO  = model.AgentID("ag-orch-dylan"), model.AgentID("ag-orch-alice")
)

type queue struct{ got []model.Event }

func (q *queue) Enqueue(_ context.Context, e model.Event) error { q.got = append(q.got, e); return nil }

type world struct {
	assignee map[string]model.Owner
	bound    map[string][]model.Owner
	orch     map[model.Owner]model.AgentID
	on       bool
	switchErr error
}

func (w world) AssigneeOf(_ context.Context, id string) (model.Owner, error) { return w.assignee[id], nil }
func (w world) BoundOwners(_ context.Context, id string) ([]model.Owner, error) { return w.bound[id], nil }
func (w world) OrchestratorOf(_ context.Context, _ string, o model.Owner) (model.AgentID, bool, error) {
	a, ok := w.orch[o]
	return a, ok, nil
}
func (w world) AgentsEnabled(context.Context, string) (bool, error) { return w.on, w.switchErr }

// team: Alice owns EP-150, Dylan EP-142, both with agents, both with Orchestrators.
func team() world {
	return world{on: true,
		assignee: map[string]model.Owner{ep150: alice, ep142: dylan},
		bound:    map[string][]model.Owner{ep150: {alice}, ep142: {dylan}},
		orch:     map[model.Owner]model.AgentID{alice: aliceO, dylan: dylanO}}
}

func deps(w world, q *queue) Deps {
	return Deps{Queue: q, Dir: w, Assignees: w, Switch: w, Now: func() time.Time { return time.Unix(0, 0) }}
}

func TestCrossOwnerBlocksSeedsTheEditorsOrchestratorAndPausesNothing(t *testing.T) {
	q := &queue{}
	out, err := deps(team(), q).OnRelated(context.Background(), Related{Org: org, Kind: "blocks", From: ep150, To: ep142, Actor: alice})
	if err != nil || out.Why != WhySeeded || len(q.got) != 1 || q.got[0].Orchestrator != aliceO {
		t.Fatalf("out=%+v q=%+v err=%v; want one event on Alice's Orchestrator", out, q.got, err)
	}
	if q.got[0].Kind != EventWorkRelation || q.got[0].Author != "" {
		t.Fatalf("event %+v; want work.relation with no human author (origin fleet)", q.got[0])
	}
}

func TestSameOwnerBlocksOpensNoProposal(t *testing.T) {
	w, q := team(), &queue{}
	w.assignee[ep142] = alice
	if out, _ := deps(w, q).OnRelated(context.Background(), Related{Org: org, Kind: "blocks", From: ep150, To: ep142, Actor: alice}); out.Why != WhySameOwner || len(q.got) != 0 {
		t.Fatalf("out=%+v q=%v", out, q.got)
	}
}

func TestRelatesAndNoAgentsSeedNothing(t *testing.T) {
	w, q := team(), &queue{}
	d := deps(w, q)
	if out, _ := d.OnRelated(context.Background(), Related{Org: org, Kind: "relates", From: ep150, To: ep142, Actor: alice}); out.Why != WhyRelates {
		t.Fatalf("relates: %+v", out)
	}
	w.bound[ep142] = nil
	if out, _ := deps(w, q).OnRelated(context.Background(), Related{Org: org, Kind: "blocks", From: ep150, To: ep142, Actor: alice}); out.Why != WhyNoAgents || len(q.got) != 0 {
		t.Fatalf("no agents: %+v q=%v", out, q.got)
	}
}

func TestAnActorWhoOwnsNeitherSideSeedsNothing(t *testing.T) {
	q := &queue{}
	out, _ := deps(team(), q).OnRelated(context.Background(), Related{Org: org, Kind: "blocks", From: ep150, To: ep142, Actor: "u-carol"})
	if out.Why != WhyNotAssignee || len(q.got) != 0 {
		t.Fatalf("out=%+v q=%v", out, q.got)
	}
}
```

```go
func TestTheSwitchOffOrUnreadableEnqueuesNothing(t *testing.T) {
	for _, w := range []world{func() world { w := team(); w.on = false; return w }(),
		func() world { w := team(); w.switchErr = errors.New("hub down"); return w }()} {
		q := &queue{}
		out, _ := deps(w, q).OnRelated(context.Background(), Related{Org: org, Kind: "blocks", From: ep150, To: ep142, Actor: alice})
		if out.Why != WhyAgentsOff || len(q.got) != 0 {
			t.Fatalf("switch %v err %v: out=%+v q=%v", w.on, w.switchErr, out, q.got)
		}
	}
}

func TestASoloProjectGetsNoFleetCall(t *testing.T) {
	w, q := team(), &queue{}
	w.orch = map[model.Owner]model.AgentID{}
	if out, err := deps(w, q).OnRelated(context.Background(), Related{Org: org, Kind: "blocks", From: ep150, To: ep142, Actor: alice}); err != nil || out.Why != WhySolo || len(q.got) != 0 {
		t.Fatalf("out=%+v q=%v err=%v", out, q.got, err)
	}
}

func TestTheEventCarriesIdsOnly(t *testing.T) {
	q := &queue{}
	_, _ = deps(team(), q).OnRelated(context.Background(), Related{Org: org, Kind: "duplicates", From: ep150, To: ep142, Actor: alice})
	if !regexp.MustCompile(`^relation:(blocks|duplicates):is-[0-9a-f]{16}:is-[0-9a-f]{16}$`).MatchString(q.got[0].Ref) {
		t.Fatalf("ref %q", q.got[0].Ref)
	}
}

func TestHandoverDirectsTheFormerOwnersOrchestratorOnly(t *testing.T) {
	w, q := team(), &queue{}
	w.bound[ep142] = []model.Owner{dylan}
	out, err := deps(w, q).OnAssigneeChanged(context.Background(), AssigneeMoved{Org: org, IssueID: ep142, NewAssignee: alice})
	if err != nil || len(q.got) != 1 || q.got[0].Orchestrator != dylanO || q.got[0].Kind != EventWorkHandover || q.got[0].Ref != ep142 {
		t.Fatalf("out=%+v q=%+v err=%v; want one work.handover on Dylan's Orchestrator", out, q.got, err)
	}
}

func TestAnAssignWithNoFormerAgentsIsNoHandover(t *testing.T) {
	w, q := team(), &queue{}
	w.bound[ep142] = []model.Owner{alice} // only the new assignee's own tasks
	if out, _ := deps(w, q).OnAssigneeChanged(context.Background(), AssigneeMoved{Org: org, IssueID: ep142, NewAssignee: alice}); out.Why != WhyNoFormerOwner || len(q.got) != 0 {
		t.Fatalf("out=%+v q=%v", out, q.got)
	}
}
```

```go
// escalate/rules_work_test.go (package escalate) — the appended row.
func TestDuplicatesAlwaysDecidesForBoth(t *testing.T) {
	for _, res := range []protocol.ResolutionKind{protocol.ResYield, protocol.ResSequence, protocol.ResProceedBoth} {
		a := protocol.Action{Relation: "duplicates", Resolution: res, Proposer: "ag-a", Acceptor: "ag-d"}
		if KeyOf(a) != "duplicates-survivor" {
			t.Fatalf("%s: key %q", res, KeyOf(a))
		}
		if row := rowByKey["duplicates-survivor"]; row.Verdict != protocol.VerdictDecide || row.Who != Both || !row.Locked {
			t.Fatalf("row %+v; want a locked decide for both", row)
		}
	}
	if KeyOf(protocol.Action{Relation: "blocks", Resolution: protocol.ResSequence}) != "sequence-short" {
		t.Fatal("a blocks thread must keep task 30's sequence row")
	}
}
```

| Test | Mutant it kills |
|---|---|
| `…SeedsTheEditorsOrchestratorAndPausesNothing` | "the owner's Orchestrator is seeded"; a human author on a Fleet event (origin `human`); an act on Dylan's side ("the edit pauses agents": Dylan's Orchestrator gets nothing) |
| `TestSameOwnerBlocksOpensNoProposal` | "proposal for your own issues" |
| `TestRelatesAndNoAgentsSeedNothing` | every relation kind seeds; a proposal with nobody's agents to sequence |
| `TestAnActorWhoOwnsNeitherSideSeedsNothing` | a third person's Orchestrator steering two others' agents (`OD-16`) |
| `TestTheSwitchOffOrUnreadableEnqueuesNothing` | "switch bypassed" (SW.19); fail-open on a read error |
| `TestASoloProjectGetsNoFleetCall` | "solo path lost": a solo relation erroring or enqueuing |
| `TestTheEventCarriesIdsOnly` | a title or description in `Ref` (TM-09's no-text rule) |
| `TestHandoverDirectsTheFormerOwnersOrchestratorOnly` | "the editor stops the owner's agents": Alice's Orchestrator told to stop Dylan's workers |
| `TestAnAssignWithNoFormerAgentsIsNoHandover` | a handover sent to the new assignee's own Orchestrator |
| `TestDuplicatesAlwaysDecidesForBoth` | "duplicates as inform"; an unlocked row a team floor could loosen; a `blocks` thread re-keyed |

The feed reader's own test (`delegation/feed_test.go` ⚑, TM-11's file, two rows appended): an
`issue.related` event reaches `OnRelated` with the subject as `From` and `otherIssueId` as `To`
(mutant: the direction swapped, which seeds the wrong side on a *blocked by* added from the other
end, `task-48.md:28-33`); an `issue.updated` whose `fields` lacks `assignee` never reaches
`OnAssigneeChanged` (mutant: every update read as a hand-over).

#### Steps

- [ ] **Step 1: read on the sync base** TM-09's `model` (`EventKinds`, `Event`), TM-26's
  `turns.Queue` and template, TM-28's `Thread.Subject`, TM-29's `Action` and `resolve.go`, TM-30's
  `Table`, `KeyOf`, `rowByKey`, TM-11's feed reader and task 98's additions to it, and task 99's
  `AgentSwitch` read. Rename in this part's files only; a Phase T file not yet merged means wait
  (rule H2).
- [ ] **Step 2: the tests**; `go test ./control-plane/internal/fleet/... -race -count=1` fails on
  the missing package and the missing row.
- [ ] **Step 3: `relations.go`, `handover.go`, `threads.go`**; the work tests pass.
- [ ] **Step 4: the registrations**: the two kinds in `model`, `Action.Relation` and the
  `RelationOf` hook in `protocol`, the row and the `KeyOf` case in `escalate`, and the two feed
  handler calls in `delegation/feed.go`. Phase T's own tests stay green unchanged.
- [ ] **Step 5: the acceptance commands.** Commit by path after steps 3 and 4, each message ending
  with the `Co-Authored-By` line.

#### Acceptance

```bash
go test ./control-plane/internal/fleet/work/... -race -count=1 -v 2>&1 | grep -c "^--- PASS"
# expect: 9
go test ./control-plane/internal/fleet/... -race -count=1
# expect: ok for every package; escalate gains TestDuplicatesAlwaysDecidesForBoth; Phase T's
#   TestTable*/TestClassify* rows and the turns tests pass unchanged
go vet ./control-plane/internal/fleet/...
# expect: exit 0
grep -rnE "Stop|Pause|Demote|Yield" control-plane/internal/fleet/work/*.go | grep -v _test | wc -l
# expect: 0   (the work package only enqueues)
```

#### What this task does NOT claim

- That two Orchestrators converge on a resolution (TM-05 §14; **UNVERIFIED** until TM1 runs), or
  that an Orchestrator in a `work.relation` turn opens the thread at all: that is its instructions
  (TM-26's template, TM-41), which this task does not write.
- That workers stop at a checkpoint: `panacea worker stop` is Phase T's verb, and whether a
  running CLI reaches a checkpoint promptly is theirs to measure.
- Anything in solo mode beyond doing nothing: the relation and the reassign reach the owner
  through task 65's Desk rows, unchanged.
- That Phase T's paths and names are as cited: all are **UNVERIFIED until merge**.

**Parity.** Closes **no parity row**: this is §2.2's *more*, beyond Linear (05 §2.6).

#### Gaps found while writing this part

1. **The scope's "turn origins" are event kinds.** Phase T's `turns.Origin` has two values,
   `human` and `fleet` (`task-26.md:158-163`); the scope paragraph for 105 (and 105's file list,
   which names only `resolve.go` and `rules.go`) should read "two event kinds in TM-09's model".
2. **`protocol.Action` has no thread or relation field**, so TM-30's rules cannot see a
   `duplicates` thread; this task adds `Action.Relation` and a nil-safe hook, an ask to TM-29.
3. **TM-28's `Thread.Subject` needs an accessor for its two issues** (`Issues() (a, b string,
   ok bool)`); if the subject does not carry issue ids, the thread cannot be tied to the relation.
4. **Task 65's reassign has no Hub record** (its own gap). This task avoids needing one by reading
   the assignee move; if a WH task later adds a reassign record, the hand-over can key on it.
5. **TM-26's template has no line for either new kind**; the Phase T owner writes it.
6. **The skeleton's file map has no Fleet row for W3's deltas** (`control-plane/internal/fleet/work/`,
   used by tasks 103, 105 and 109); it should add one.
