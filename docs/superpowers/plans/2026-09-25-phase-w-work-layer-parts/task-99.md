### Task 99: the W3 Hub wire, fake and conformance — comment kinds, guidance, the org switch

**Slice:** W3. **Size:** 3 focused hours (an estimate, not a measurement). **Stream:** WH.
**dependsOn:** 98 (97 is an ordering constraint only — see gap 6). **Seam:** defines **SW.19**
(the agent-facing Work wire), which task 100 implements on the real Hub.

The wire, the fake and the conformance rows for everything an agent may write in the work layer:
comment kinds, attribution, evidence **proposals** that are not evidence, negotiation metadata,
versioned guidance, and the org's one switch that turns agent writing off. No route and no store:
task 100 makes the real Hub pass these rows, and this task is what "pass" means.

Three sentences from 05 fix the shape, and every decision below serves one of them: comment
`ic-` carries kinds `note`, `evidence_proposal`, `negotiation` (§3.2); *"an agent writes only
comments on its own owner's behalf, and in team mode only its Orchestrator does"* (§2.1, §3.6);
a proposal is a proposal until a person records it (§3.6 with hub-contract §7.10).

**Anchors, checked on 2026-09-27.** Task 02's `workids.go` (`task-02.md:149-174`): twelve Work
prefixes `is ic ft ml cy in rq lb tp vw lp df`, declared there and not in `keys.go` because
`TestPrefixesAndReasonsAreTheContracts` holds `keys.go` and contract §3.1 to ten each; the comment
above them records that they are *"disjoint from §3.1's ten, the control plane's `s- p- b- w- m-`,
and Phase T's twelve (`TestWorkPrefixesAreDisjoint`)"*, and that `dc` is reserved. Task 04's
`hubclient/work.go` (`task-04.md:98`): `type Work interface { WorkIssues }`, and `Work` embedded
in the client interface (`:130`); its fake files and the `fake_work_stub.go` convention, whose
methods answer 501 `not_in_fake_yet` until built (`:101`). Task 05's
`hubclient/conformance/work.go` (`task-05.md:32`, `:79-80`, `:87-125`): `GroupWork = "work"`,
`WorkEnv`, `RunWork(t, newEnv, rows...)`, `W-NN` rows, and `work_fake_test.go` running them
against `NewFake()`. Task 06's `issue_comments` with `kind` checked `IN ('comment')` and the note
*"W3 widens (task 99)"* (`task-06.md:291-316`). Task 98's `IssueDelegatedData` and its two codes
(`task-98.md`, this phase). Hub contract on `feat/hub` at `63e7232`: §7.10 evidence
(`:1060-1088`, *"`recordedBy` is the principal"*, append-only), §8 events (`:1131`).
**UNVERIFIED:** Phase T's merged prefix set (`dv ot hm nt` at plan `:447` are four of its twelve;
the other eight are read at Step 1), and TM-13/TM-14's policy store, which gap 3 asks about.

#### Seven decisions the scope left open, made here

**(a) Guidance takes the prefix `gd`, and the disjointness test is extended, not trusted.**
05 §3.6's list gives guidance no prefix. `gd` is free of task 02's twelve
(`is ic ft ml cy in rq lb tp vw lp df`), of contract §3.1's ten, of the control plane's
`s- p- b- w- m-` and of Phase T's `dv ot hm nt` (plan `:447`); the remaining eight Phase T
prefixes are read at Step 1 and `TestWorkPrefixesAreDisjoint` is re-run before the constant is
kept. It is declared in `workids.go` as the thirteenth, and this part records the addition so
05 §3.6 can be corrected.

**(b) Attribution is a required struct on every agent comment, and it is the principal's.**
`note`, `evidence_proposal` and `negotiation` all carry `AgentAttribution{ownerId, agentId,
model, sessionId}` — all four, non-empty (L-173). The fake refuses a missing `model` or
`sessionId` with 422 `attribution_incomplete`; the real Hub additionally pins `ownerId` to the
delegation's owner (task 100's decision (b), code `agent_not_owner`). Two layers, one rule.

**(c) `comment` is the default kind and the only one a person may write; `negotiation` is the
only one a person may never write.** A person writing `note` is allowed (people take notes);
a person writing `negotiation` is 403 `kind_not_permitted`, because a negotiation points at a
Phase T thread that a person does not own. An agent writing plain `comment` is 403 too: an agent's
voice is always marked. The rule lives in one function, `CommentKind.PermittedFor(principalKind)`,
so the fake and the Hub cannot drift.

**(d) A proposal is inert until a person records evidence, and the done rule must ignore it.**
`EvidenceProposal` is metadata on a comment. It never appears in the Hub's evidence tables, never
satisfies a criterion, and SW.4's done rule (task 08) must not count it. The conformance row that
proves this is the one that would catch the cheapest possible bug: an agent proposing evidence and
the issue going `done` by itself. Linking a proposal to a real record is `LinkEvidenceProposal`
(task 104's need, task 100's route), declared here so both exist on the same interface.

**(e) Guidance is append-only versions, resolved by narrowest scope.** `PutGuidance` writes a new
version and never rewrites one; `ResolveGuidance(orgId, teamId, personId)` returns the layers it
used, narrowest last, so a caller can show *why* a line applies. `GuidanceVersions(id)` returns
the history (task 109's surface). Deleting is a version with empty text, not a delete: L-183's
append-only is the point.

**(f) The org switch is one record per org, read by anyone, written by an org admin.**
`AgentSwitch{orchestrators bool, by, at, reason}`. `reason` is required when turning it **off**
(L-158: a kill switch with no reason is a mystery at 3 a.m.) and optional when turning it on. The
fake refuses a non-admin writer with 403 `forbidden`, and while `orchestrators` is false every
delegated **write** answers 403 `agents_disabled` while every **read** passes — Fleet must be able
to read the switch that refuses it.

**(g) One sub-interface, embedded, so nothing in W1's files changes.** `WorkAgents` is declared
in `hubclient/work.go` and embedded in `Work` (`type Work interface { WorkIssues; WorkAgents }`),
exactly as task 04 embedded `WorkIssues`. Its fake is a new file, `fake_work_agents.go`, and its
rows are a new file, `conformance/work_agents.go`. No W1 file is edited except the one line that
embeds it and the stub line it removes.

#### The files this task touches

| File | New? | What it gains |
|---|---|---|
| `control-plane/internal/hub/hubapi/work_agents.go` | new | every type below, with `Valid()` per enum |
| `control-plane/internal/hub/hubapi/work_agents_golden_test.go` | new | a golden vector per type |
| `control-plane/internal/hub/hubapi/workids.go` ⚑ | no | `PrefixGuidance = "gd"`, appended to `WorkPrefixes` |
| `control-plane/internal/hub/hubapi/codes.go` ⚑ | no | the five codes listed below |
| `control-plane/internal/hub/hubapi/events.go` ⚑ | no | five kinds and their data types |
| `control-plane/internal/hub/hubclient/work.go` ⚑ | no | `WorkAgents`, embedded in `Work` |
| `control-plane/internal/hub/hubclient/fake_work_agents.go` | new | the fake's state and every rule |
| `control-plane/internal/hub/hubclient/fake_work_stub.go` ⚑ | no | the W3 stub lines deleted |
| `control-plane/internal/hub/hubclient/conformance/work_agents.go` | new | `RunWorkAgents`, the `WA-NN` rows |
| `control-plane/internal/hub/hubclient/conformance/work_agents_fake_test.go` | new | the rows against `NewFake()` |

#### Interfaces — complete: `hubapi/work_agents.go`

```go
package hubapi

// CommentKind is W3's widening of task 06's `kind` column, which W1 checked IN ('comment').
// Empty means "comment": every W1 comment on the wire round-trips unchanged.
type CommentKind string

const (
	CommentKindComment    CommentKind = "comment"           // a person's comment (W1)
	CommentKindNote       CommentKind = "note"              // an agent's note, or a person's
	CommentKindProposal   CommentKind = "evidence_proposal" // never evidence: see EvidenceProposal
	CommentKindNegotiation CommentKind = "negotiation"      // metadata for a Phase T thread
)

func (k CommentKind) Valid() bool {
	switch k {
	case "", CommentKindComment, CommentKindNote, CommentKindProposal, CommentKindNegotiation:
		return true
	}
	return false
}

// PermittedFor is decision (c), in one place so the fake and the Hub cannot drift.
// principal is "person" or "agent".
func (k CommentKind) PermittedFor(principal string) bool {
	switch k {
	case "", CommentKindComment:
		return principal == "person"
	case CommentKindNote, CommentKindProposal:
		return true
	case CommentKindNegotiation:
		return principal == "agent"
	}
	return false
}

// AgentAttribution is required on every comment an agent writes (L-173). All four fields are
// non-empty; the Hub additionally pins OwnerID to the delegation's owner (task 100).
type AgentAttribution struct {
	OwnerID   string `json:"ownerId"`
	AgentID   string `json:"agentId"`   // Phase T's ag- (TM-04's agents key)
	Model     string `json:"model"`     // as the session reported it, verbatim
	SessionID string `json:"sessionId"` // the harness session, opaque here
}
```

```go
// EvidenceProposal is what an agent may say about evidence. It is NOT evidence: nothing here
// satisfies a criterion until a person records it through the Hub's own evidence route
// (hub-contract §7.10: "recordedBy is the principal", append-only). Decision (d).
type EvidenceProposal struct {
	BriefKey    string            `json:"briefKey"`
	CriterionID string            `json:"criterionId"`
	IssueID     string            `json:"issueId"`
	Commit      string            `json:"commit"`
	Artefact    ProposalArtefact  `json:"artefact"`
	Because     string            `json:"because"` // the agent's reason, text·A
	RecordedAs  *string           `json:"recordedAs,omitempty"` // set once, by a person (task 100)
}

type ProposalArtefact struct {
	Kind string `json:"kind"` // "test" | "screenshot" | "log" | "diff" | "doc"
	Ref  string `json:"ref"`  // a path, a run id or a blob key; never the artefact itself
}

// Negotiation is metadata only. The words live in Phase T's messages; this points at them so
// the issue shows that a negotiation is happening without copying a syllable of it.
type Negotiation struct {
	ThreadID          string `json:"threadId"`          // Phase T's th-
	CounterpartIssueID string `json:"counterpartIssueId"`
}

// Guidance is append-only, versioned, and layered (L-183). A delete is a version with empty
// Text, never a row removed.
type GuidanceScope string

const (
	GuidanceScopeOrg    GuidanceScope = "org"
	GuidanceScopeTeam   GuidanceScope = "team"
	GuidanceScopePerson GuidanceScope = "person"
)

func (s GuidanceScope) Valid() bool {
	return s == GuidanceScopeOrg || s == GuidanceScopeTeam || s == GuidanceScopePerson
}

type Guidance struct {
	ID      string        `json:"id"`      // gd- (decision (a))
	Scope   GuidanceScope `json:"scope"`
	ScopeID string        `json:"scopeId"` // org, team or user id; matches Scope
	Text    string        `json:"text"`    // text·H: a person writes guidance
	Version int           `json:"version"` // 1, 2, 3 …; never reused
	Author  string        `json:"author"`
	At      string        `json:"at"`      // RFC3339
	ErasedAt *string      `json:"erasedAt,omitempty"` // task 14's erase reaches this too
}

// AgentSwitch is one record per org (decision (f), L-158, L-246).
type AgentSwitch struct {
	Orchestrators bool   `json:"orchestrators"`
	By            string `json:"by"`
	At            string `json:"at"`
	Reason        string `json:"reason"` // required when turning it off
}
```

#### Interfaces — complete: codes, events and the client sub-interface

Task 100's part assumed these names before this part existed (`task-100.md`, its gap 1). They are
adopted here verbatim, so the two agree rather than each being half right.

```go
// hubapi/codes.go — five added, beside W1's and task 98's two.
const (
	CodeAgentsDisabled          = "agents_disabled"           // 403: the org switch is off
	CodeAttributionIncomplete   = "attribution_incomplete"    // 422: a missing attribution field
	CodeKindNotPermitted        = "kind_not_permitted"        // 403: decision (c)
	CodeOrchestratorUnavailable = "orchestrator_unavailable"  // 422: no live delegation to delegate to
	CodeProposalMismatch        = "proposal_mismatch"         // 422: the record does not match the proposal
)

// hubapi/events.go — W3's kinds. issue.delegated is task 98's; these five are this task's.
const (
	EventIssueUndelegated   = "issue.undelegated"        // the assignee withdrew (task 100, Undo)
	EventCommentProposalLinked = "comment.proposal_linked" // a person recorded the evidence
	EventGuidanceVersioned  = "guidance.versioned"
	EventOrgAgentSwitchSet  = "org.agent_switch_set"
	EventOrgAgentWriteRefused = "org.agent_write_refused" // metadata only: route + delegation
)

type IssueUndelegatedData struct{ IssueID string `json:"issueId"` }
type CommentProposalLinkedData struct {
	CommentID  string `json:"commentId"`
	EvidenceID string `json:"evidenceId"`
}
type GuidanceVersionedData struct {
	GuidanceID string `json:"guidanceId"`
	Version    int    `json:"version"`
	Scope      string `json:"scope"`
}
type AgentSwitchSetData struct{ Orchestrators bool `json:"orchestrators"` }
type AgentWriteRefusedData struct {
	Route        string `json:"route"`
	DelegationID string `json:"delegationId"`
}
```

```go
// hubclient/work.go — the sub-interface, embedded as task 04 embedded WorkIssues.
type Work interface {
	WorkIssues
	WorkAgents // Phase W, task 99: SW.19
}

// WorkAgents is everything an agent may write in the work layer, plus the two reads a person
// needs to govern it. Every method takes the caller's context; the principal comes from it.
type WorkAgents interface {
	DelegateIssue(ctx context.Context, issueID string) (hubapi.Delegation, error)
	AddAgentComment(ctx context.Context, issueID string, in hubapi.AgentCommentInput) (hubapi.IssueComment, error)
	ListEvidenceProposals(ctx context.Context, projectID string, f hubapi.ProposalFilter) ([]hubapi.EvidenceProposal, error)
	LinkEvidenceProposal(ctx context.Context, commentID, evidenceID string) error
	PutGuidance(ctx context.Context, in hubapi.GuidanceInput) (hubapi.Guidance, error)
	ResolveGuidance(ctx context.Context, orgID, teamID, personID string) ([]hubapi.Guidance, error)
	GuidanceVersions(ctx context.Context, guidanceID string) ([]hubapi.Guidance, error)
	GetAgentSwitch(ctx context.Context, orgID string) (hubapi.AgentSwitch, error)
	SetAgentSwitch(ctx context.Context, orgID string, in hubapi.AgentSwitch) error
}
```

Three more names task 100 needs, adopted here so neither part invents them twice:

```go
// hubclient/work.go — WorkAgents, one more method (task 100 decision (f): Undo).
//   WithdrawDelegation(ctx context.Context, issueID string) error

// hubapi/codes.go — three more.
const (
	CodeEvidenceNotFound    = "evidence_not_found"   // 404: no such record in the issue's project
	CodeDelegationNotFound  = "delegation_not_found" // 404: nothing live to withdraw
	CodeThreadUnresolvable  = "thread_unresolvable"  // 422: a negotiation's threadId is not readable
)

// hubapi/work_agents.go — the two inputs the methods above take.
type AgentCommentInput struct {
	Kind        CommentKind       `json:"kind"`
	Body        string            `json:"body"`        // text·A
	ParentID    string            `json:"parentId,omitempty"`
	Attribution AgentAttribution  `json:"attribution"`
	Proposal    *EvidenceProposal `json:"proposal,omitempty"`    // kind == evidence_proposal
	Negotiation *Negotiation      `json:"negotiation,omitempty"` // kind == negotiation
}

type GuidanceInput struct {
	Scope   GuidanceScope `json:"scope"`
	ScopeID string        `json:"scopeId"`
	Text    string        `json:"text"`
}

type ProposalFilter struct {
	IssueID     string `json:"issueId,omitempty"`
	CriterionID string `json:"criterionId,omitempty"`
	Unlinked    bool   `json:"unlinked,omitempty"` // RecordedAs == nil only
}
```

#### Interfaces — complete: the conformance rows (`conformance/work_agents.go`)

Task 05's shape: `RunWorkAgents(t, newEnv, rows...)` beside `RunWork`, rows named `WA-NN`, run
against the fake in `work_agents_fake_test.go` and against the real Hub by task 100. Each row is
one rule, and the table below is the row list in full.

| Row | What it asserts | The mutant it kills |
|---|---|---|
| WA-01 | an agent comment with no `model` → 422 `attribution_incomplete` | attribution optional |
| WA-02 | an agent comment with no `sessionId` → 422 `attribution_incomplete` | half the struct checked |
| WA-03 | a **person** writing `negotiation` → 403 `kind_not_permitted` | any principal may negotiate |
| WA-04 | an **agent** writing bare `comment` → 403 `kind_not_permitted` | an agent's voice unmarked |
| WA-05 | a W1 comment with no `kind` round-trips as `comment` | the widening breaks W1's fixtures |
| WA-06 | an `evidence_proposal` on the last open criterion leaves the issue **not** done (SW.4) | the fake counts a proposal as evidence |
| WA-07 | `ListEvidenceProposals{Unlinked:true}` omits a linked one | the filter ignored |
| WA-08 | `LinkEvidenceProposal` twice with the same id → no second event; with another → 409 | a link that silently moves |
| WA-09 | team guidance overrides org **for that team only**; another team still sees org | one global layer |
| WA-10 | `PutGuidance` on an existing id writes version n+1 and version n still reads back | `PutGuidance` overwrites |
| WA-11 | a non-admin `SetAgentSwitch` → 403 `forbidden` | anyone can disarm the switch |
| WA-12 | switch off: a delegated write → 403 `agents_disabled`; a person's write → 200 | people blocked too |
| WA-13 | switch off: `GetAgentSwitch` by Fleet still → 200 | Fleet cannot read what refuses it |
| WA-14 | switch off: one `org.agent_write_refused` event per refusal, metadata only | the switch is a flag only the UI reads |
| WA-15 | `DelegateIssue` by anyone but the assignee → 403 `forbidden` | any member delegates (L-39, `OD-16`) |
| WA-16 | `DelegateIssue` twice → the same record, no second event | a duplicate delegation per click |
| WA-17 | task 14's erase covers `note`, `evidence_proposal`, `negotiation` and `Guidance` | agent kinds skip erase |
| WA-18 | no payload of any W3 event carries a title or body | text in `data` (hub-contract §8) |

#### Tests to write first

The conformance rows above are the tests, and they are written before the fake. Two files run
first and fail for the right reason:

```go
// conformance/work_agents_fake_test.go — the whole table against NewFake(). Written first; it
// fails to compile until WorkAgents exists, then fails 18 rows, then passes one row at a time.
func TestWorkAgentsFake(t *testing.T) {
	conformance.RunWorkAgents(t, func(t *testing.T) conformance.WorkAgentsEnv {
		return newFakeAgentsEnv(t) // a fake per subtest: no shared state, no order dependence
	}, conformance.WorkAgentsRows...)
}
```

```go
// hubapi/work_agents_golden_test.go — one vector per type, bytes pinned.
// Mutant it kills: a json tag renamed, which would pass every behavioural test and break the
// desktop and Fleet at once.
func TestAgentCommentInputGolden(t *testing.T) { assertGolden(t, "agent_comment_input.json", sample()) }
// … one per: AgentAttribution, EvidenceProposal, Negotiation, Guidance, AgentSwitch,
//     GuidanceInput, ProposalFilter, and each of the five event data types.
```

Two more, outside the table, because they guard the seam rather than a rule:

- `TestWorkPrefixesAreDisjoint` (task 02's, re-run) with `gd` added — mutant: a prefix collision
  with Phase T, which would make two kinds of id indistinguishable in a log.
- `TestEveryWorkAgentsMethodHasARow` — reflects over `WorkAgents` and asserts every method name
  appears in at least one `WA-NN` row's `Methods` field. Mutant: a method added to the interface
  and never covered, which is how SW.19 would quietly stop meaning anything.

#### Steps

1. Re-read Phase T's merged prefix set and re-run `TestWorkPrefixesAreDisjoint` before keeping
   `gd` (decision (a)). If it collides, pick the next free two letters and record it.
2. Write `conformance/work_agents.go` — the rows, `WorkAgentsEnv`, `RunWorkAgents` — and the two
   test files above. Watch them fail to compile.
3. `hubapi/work_agents.go`: the types, the two `Valid()`s and `PermittedFor`. Golden tests green.
4. `workids.go`, `codes.go`, `events.go`: the prefix, the eight codes, the five kinds with their
   data types and registry rows (task 13's registry check must see each).
5. `hubclient/work.go`: `WorkAgents`, embedded; delete the W3 lines from `fake_work_stub.go`.
6. `fake_work_agents.go`, one rule at a time, in row order WA-01 to WA-18. A row goes green only
   with its mutant checked by hand: flip the fake, see the row fail, flip it back.
7. Run `go test ./control-plane/internal/hub/... -race -count=1` and W1's conformance too: W1's
   rows must be untouched (WA-05 is the canary).
8. Record the `gd` addition, the eight codes and the five kinds in the phase's Hub-delta list for
   task 100 and task 113.

#### Acceptance

```bash
# 1. Every conformance row passes against the fake, and there are eighteen of them.
go test ./control-plane/internal/hub/hubclient/conformance/... -run WorkAgents -race -count=1 -v \
  | grep -c '^=== RUN   TestWorkAgentsFake/WA-'
# expect: 18

# 2. W1 is untouched: its rows and its golden vectors still pass.
go test ./control-plane/internal/hub/hubclient/... ./control-plane/internal/hub/hubapi/... -count=1
# expect: ok, no golden file rewritten (git status clean under testdata/)

# 3. Every WorkAgents method is covered by a row.
go test ./control-plane/internal/hub/hubclient/conformance/... -run EveryWorkAgentsMethodHasARow
# expect: ok

# 4. The prefix is disjoint, with gd added.
go test ./control-plane/internal/hub/hubapi/... -run WorkPrefixesAreDisjoint -v
# expect: PASS, and the test reports 13 Work prefixes

# 5. The stub is gone: nothing in W3 answers not_in_fake_yet.
grep -n 'workNotInFakeYet' control-plane/internal/hub/hubclient/fake_work_stub.go
# expect: no W3 method named (task 05 deletes the file entirely if it is now empty)
```

#### What this task does NOT claim

- **Any route, any store, any migration.** This is the wire, the fake and the rows. The real Hub
  is task 100's, and until it lands `RunWorkAgents` runs against the fake only.
- **That guidance text may enter an agent's context.** It is spec-class text, stored and
  resolved. Whether it joins Phase T's policy layers (TM-13, TM-14) is **UNVERIFIED** and is
  gap 3's question to the Phase T owner.
- **That the switch stops a running session.** It refuses the next delegated write. Stopping a
  live turn is task 109's gate and Phase T's *pause all*.
- **Any desktop surface.** The comment kinds, the proposal card, the guidance editor and the
  switch are tasks 102-104, 109 and 111; this task gives them nothing to render but a type.
- **That a proposal is ever evidence.** WA-06 exists precisely because that is the bug this
  design is shaped to prevent (hub-contract §7.10).
- **Any parity row alone.** It carries the wire half of L-39, L-156, L-158, L-166, L-173, L-183,
  L-193 and L-246; tasks 100, 102-104, 109 and 111 close them and task 113 records them.

#### Gaps found while writing this part

1. **05 §3.6 gives guidance no id prefix.** `gd` is proposed here and checked free; 05 and the
   Hub contract's §3.1 note should be corrected, or the owner picks another.
2. **Task 06's `kind` check must be widened by a migration nobody owns yet.** `task-06.md:291-316`
   says *"W3 widens (task 99)"*, but this task writes no SQL — task 100 does. The skeleton should
   move that note to task 100, or give this task the migration.
3. **Guidance and TM-14's versioned policy store may be the same mechanism.** Two append-only,
   scoped, versioned text stores in one product is one too many. The Phase T owner decides;
   **UNVERIFIED** until then, and this part's `Guidance` is the placeholder either way.
4. **The org switch has no home in the skeleton's file map.** `AgentSwitch` is per org, not per
   project: it needs an org-scoped table and an org-scoped route, and W3's store tasks are all
   project-scoped. Task 100 carries it here; the skeleton should say so.
5. **`Delegation` is Phase T's type, returned by `DelegateIssue`.** That makes `hubclient/work.go`
   depend on Phase T's `hubapi/team.go`, which is fine after the sync-merge and impossible before
   it. Ordering: this task lands after task 98's merge, never before.
6. **The skeleton's dependency row for this task says "97, 98".** Task 97 is W2's results and
   produces no code this task uses. The real dependency is 98; 97 is ordering only, and W3's wire
   should not wait on a fortnight-long run.
