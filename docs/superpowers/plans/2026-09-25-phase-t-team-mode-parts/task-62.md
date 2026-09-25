### Task 62: Resolutions `counter`, `split`, `build-on`

**Slice TM2. Depends on: 29, 30, 60. About 3 hours. Seam: none new (extends ST.1's `peer`).**

Task 28 put all eight message kinds and all five resolutions in the wire from day one, and refused
`counter`, `split` and `build-on` with `kind_not_enabled` (`task-28.md:44-47`, `:241`, `:246-247`).
This task enables them. It is an extension of three sibling parts on this branch: task 28's
validation and `Service.Send`, task 29's `Resolver` and appliers, and task 30's rules and options.
No new package; every file below is a modification except `split.go`, `buildon.go` and their tests.

The design's three rows (`05-design.md:654`, `:661`, `:664`, and §6.5's rows at `:727`, `:731`):

| Kind | What Fleet does | §6.5 |
|---|---|---|
| `counter` (a message kind) | the countered proposal stops being live; the counter is the live proposal; it counts as a round for its sender | — (its resolution is ruled when it is accepted) |
| `split` (a resolution) | re-issues each side's leases on the contested paths to that side's own globs | act if the acceptor's side keeps every path it has edited, else **decide**; Fleet informs either way |
| `build-on` (a resolution) | brings the base side's lane into the builder's worktree as a bundle through the mirror | **decide, always, locked**, with every agent-config path the lane changes on its own line |

Four properties, each pinned by a test.

1. **A counter spends a round like a propose.** Task 28's cap and stale-run rules apply to it
   unchanged: a side's fourth counting message is refused `rounds_exhausted` and the thread
   escalates (task 30's `ThreadExhausted`), so a counter past the cap cannot keep a thread alive.
2. **A split moves only the contested area, and only by whole globs.** It is validated at
   propose time and again at accept time, like task 29's `yield` (decision b there).
3. **A `build-on` accept applies nothing until the builder's human answers.** Its row is locked
   (`task-30.md:203`); no floor, user row or error path can make it act. No bundle is exported, no
   lease or conflict moves, and the builder's worker gets no directive, until task 30's
   `OnAnswered` calls `ApplyApproved`.
4. **The decide lists every agent-config path the lane changes, each on its own line**, from the
   mirror's typed diff, classified by task 49's `converge.IsAgentConfigPath`. If the mirror cannot
   list them, the decide says so and does **not** offer the apply option.

Five decisions the scope left open, made here.

**(a) A counter names the proposal it counters.** `counter` needs `thread`, `proposal` (a live
`propose` or `counter` of this thread by the **other** side) and `resolution`, and takes an
optional rationale ≤ 600 (task 28's propose row). The countered proposal leaves the thread's live
set as a `withdraw` by the other side would, and the counter joins it. A counter of one's own
proposal is `bad_request` (withdraw and propose again). Task 29's `CheckProposal` checks the
counter's resolution exactly as it checks a propose's, and task 28 (b)'s "new typed content"
applies: a counter whose canonical resolution is already in the thread is not new.

**(b) A split partitions the conflict's paths.** `Resolution.Sides` (task 28's field) has exactly
two keys, the thread's two participants, each 1 to `teamtext.MaxGlobs` globs, each `ValidGlob` and
under task 23's 10% breadth cap. Every path of the conflict must be matched by the globs of
**exactly one** side (task 02's `Match`): a path matched by neither leaves the conflict open; one
matched by both is the conflict again. Otherwise `bad_request` with `split_not_a_partition` and
the offending paths. A split is only for a conflict thread (it needs the conflict's paths).

**(c) What "re-issues" means.** For each side S and each live lease L of S's side (its
Orchestrator and the workers whose `parentAgentId` it is) with a glob that matches a conflict path:
L goes to `yielded`, and one new lease is requested for L's holder, mode and task, with L's globs
that match no conflict path **plus** S's split globs, through task 23's own rules (so it is
`granted` or `contested`, never forced). A side with no such lease gets one new `intent` lease on
its split globs for its party agent. Leases stay advisory (P7).

**(d) The acceptor's edited paths decide act or decide.** `Action.EditedByAcceptor` is the
acceptor side's observed edits (task 35's activity table) on the conflict's paths. If any is not
matched by `Sides[acceptor]`, task 30's `KeyOf` returns `accept-split-moves-edited` (a decide for
the acceptor's human); otherwise `accept-split` (act, floorable). This is the test task 30's
`KeyOf` left for this task (`task-30.md:246-247`).

**(e) Build-on brings a ref in, and the builder's own Orchestrator directs the merge.** Fleet
moves no worker (§6.2). So on apply, Fleet exports `mergeBase..baseTip` of the base side's lane
from the mirror and stages it through the broker into the builder's party worker's clone as
`refs/panacea/build-on/<th-id>`, touching no index or working tree (like phase 3 task 09's
`RefreshClones`, refs only). The builder's Orchestrator then directs **its own** worker with a
new typed directive, `build-on` with that `ref`; Fleet's verb handler admits it in a Fleet-started
turn only for a ref an applied `Outcome` named for that worker. `baseTip` is the tip the decide
described: the bundle carries exactly what the human approved, never a later tip. Build-on also
writes one task 29 `land_hints` row, `first` the base side, so task 61's queue lands the base first.

**Files:**
- `control-plane/internal/fleet/protocol/messages.go` (task 28) — the `counter`, `split` and
  `build-on` rows of the validation table replace `kind_not_enabled` (modify)
- `control-plane/internal/fleet/protocol/service.go` (task 28) — `Send` step 9: a stored counter
  retires the countered proposal from the live set (modify)
- `control-plane/internal/fleet/protocol/resolve.go` (task 29) — `Action` and `Outcome` gain
  fields; `CheckProposal` and `Resolve` dispatch to the two new appliers (modify)
- `control-plane/internal/fleet/protocol/split.go` + `split_test.go` — `checkSplit`,
  `applySplit` (new)
- `control-plane/internal/fleet/protocol/buildon.go` + `buildon_test.go` — `BuildOnPort`,
  `AgentConfigLine`, `checkBuildOn`, `buildOnLines`, `applyBuildOn` (new)
- `control-plane/internal/fleet/leases/yield.go` (task 29) — `Engine.Reissue` (modify)
- `control-plane/internal/fleet/escalate/rules.go` (task 30) — `KeyOf`'s split test (modify)
- `control-plane/internal/fleet/escalate/options.go`, `service.go` (task 30) — the build-on
  decide: its question, its lines, the apply option withheld when the lines are unknown (modify)
- `control-plane/internal/fleet/verbs/direct.go`, `directive/directive.go` (task 31) — the typed
  `build-on` directive kind and its Fleet-turn check (modify)
- `control-plane/internal/hub/hubapi/team.go` (task 03), `server/escalations.go` (task 07),
  `migrate/sql/00NN_escalation_agent_config.sql` (after task 04's) — `agentConfig` on a decide
  and `DirectiveBuildOn` with `ref` (modify / new; Hub deltas on this branch's own Hub code)
- `docs/superpowers/contracts/2026-09-23-hub-contract.md` (task 01's §7 entries for route 64
  and the directive route) — the `agentConfig` field and the `build-on` kind (modify)
- `sandboxd/internal/panaceacli/direct.go` (task 18) — `panacea direct <ag-…> build-on --ref`
  (modify)
- `control-plane/internal/fleet/instructions/worker-cloud.md` (task 41) — one paragraph: what
  a `build-on` directive asks the worker to do (modify)
- `testdata/panacea-control/v1/{peer-counter,peer-split,peer-build-on,direct-build-on}.json` —
  four fixtures both modules read (new)

**Interfaces:**
- Produces: the three kinds enabled; `protocol.BuildOnPort`; `leases.Engine.Reissue`;
  `protocol.AgentConfigLine`; `Outcome.BuildOn`; the Hub escalation's `agentConfig`; the
  directive kind `build-on`.
- Consumes: task 28's `Thread`, `Stored`, `Resolution` (its `Sides` and `Base` fields,
  `task-28.md:166-167`), `Canonical`, `RoundsLeft`; task 29's `Resolver`, `leaseEvent`,
  `DecidedWithoutYou`, `fleet.land_hints`; task 30's `Rules.Classify`, `Service.Decide`,
  `AcceptNeedsDecision`; task 23's `Engine.Request`, `Live`, `Validate` (breadth); task 35's
  `activity.Table.Of(ctx, agent)` for each acceptor-side agent's observed paths (`task-35.md:165`); task 61's
  mirror ops `ExportRange` and its `PathChange`; task 49's `converge.IsAgentConfigPath` and
  `converge.PackageScriptMembers` (exported by task 61).
- Requires of **task 59's mirror client**: `LaneChanges` (below), beside task 61's three ops.

```go
// control-plane/internal/fleet/protocol/resolve.go (modify): the fields added.

type Action struct {
	// … task 29's fields, unchanged …
	Sides            map[model.AgentID][]string // split: participant → its globs
	Base             model.AgentID              // build-on: whose lane is built on
	BaseTip          string                     // build-on: the lane tip the decide describes
	EditedByAcceptor []string                   // split: acceptor side's observed edits on the conflict's paths
	AgentConfig      []AgentConfigLine          // build-on: every agent-config path, sorted by Path
	AgentConfigKnown bool                       // build-on: false when the mirror could not list them
}

type Outcome struct {
	// … task 29's fields, unchanged …
	Split   map[model.AgentID][]string `json:"split,omitempty"`
	BuildOn *BuildOnOutcome            `json:"buildOn,omitempty"`
}

type BuildOnOutcome struct {
	Base     model.AgentID `json:"base"`     // the base side's Orchestrator
	Builder  model.AgentID `json:"builder"`  // the builder side's Orchestrator
	Worker   model.AgentID `json:"worker"`   // the builder's party worker, whose clone got the ref
	Ref      string        `json:"ref"`      // refs/panacea/build-on/<th-…>
	BaseTip  string        `json:"baseTip"`
	HintID   int64         `json:"hintId"`   // the land_hints row, first = Base
}
```

```go
// control-plane/internal/fleet/protocol/buildon.go
package protocol

// AgentConfigLine is one agent-config path a build-on would bring into the
// builder's sandbox. Kind is task 49's four; Members is set for package.json.
type AgentConfigLine struct {
	Path    string   `json:"path"`
	Kind    string   `json:"kind"`    // instructions | agent-settings | git-hooks | package-scripts
	Status  string   `json:"status"`  // A | M | D
	Members []string `json:"members,omitempty"` // package-scripts: which members differ
}

// LaneDiff is the mirror's typed answer for one lane since its merge base
// with the integration head: paths and statuses only, never content (ST.7).
type LaneDiff struct {
	MergeBase      string              `json:"mergeBase"`
	Tip            string              `json:"tip"`
	Changes        []PathChange        `json:"changes"`        // task 61's shape, --no-renames
	PackageChanged map[string][]string `json:"packageChanged"` // path → members that differ
}

type PathChange struct {
	Path   string `json:"path"`
	Status string `json:"status"`
}

// BuildOnPort is the wiring's adapter over task 59's mirror client and task
// 17's broker; protocol runs no git and imports neither.
type BuildOnPort interface {
	// LaneChanges asks the mirror for laneRef at tip against
	// refs/panacea/integration/<branch>. tip must equal the ref's sha, else
	// ErrStale.
	LaneChanges(ctx context.Context, project, laneRef, tip, branch string, members []string) (LaneDiff, error)
	// Bring exports mergeBase..tip from the mirror and stages it into the
	// builder worker's clone under ref, through the broker, under the
	// builder's owner's delegation. Refs only: no index, no working tree.
	Bring(ctx context.Context, builderOwner, project, worktreeID, ref, mergeBase, tip string) error
}

// buildOnLines classifies a LaneDiff with converge.IsAgentConfigPath, plus
// every package.json with a non-empty member list. Sorted by Path. Pure.
func buildOnLines(d LaneDiff) []AgentConfigLine

const buildOnRefPrefix = "refs/panacea/build-on/" // + th-…

// BuildOnRef is the one ref name a build-on uses for a thread.
func BuildOnRef(thread string) string { return buildOnRefPrefix + thread }
```

`PathChange` is written again here, not imported from `landq`, so that `protocol` imports no
queue; both are pinned by the one fixture `testdata/panacea-control/v1/mirror-lane-diff.json`.

```go
// control-plane/internal/fleet/leases/yield.go (modify): one method.

// Reissue moves l to yielded and requests one new lease for l's holder, mode
// and task with exactly globs, through Request's own rules (granted or
// contested, never forced), in one transaction. globs must pass
// teamtext.CheckSet and the breadth cap; else nothing moves.
func (e *Engine) Reissue(ctx context.Context, org string, l model.Lease, globs []string) (newID string, err error)
```

```go
// control-plane/internal/fleet/escalate/rules.go (modify): KeyOf's split case.
	case protocol.ResSplit:
		mine := a.Sides[a.Acceptor]
		for _, p := range a.EditedByAcceptor {
			if !matchAny(mine, p) {
				return "accept-split-moves-edited"
			}
		}
		return "accept-split"
```

Task 02 exports `Match(glob, path)` only (`task-02.md:898`), so `matchAny` is a three-line helper
in `rules.go` over it.

**Validation, the three rows that replace `kind_not_enabled`** (task 28's table, `task-28.md:236-247`):

| Kind / resolution | Required | Refused | Text |
|---|---|---|---|
| `counter` | `thread`, `proposal` (a live `propose`·`counter` of this thread by the **other** side), `resolution` | `to`, `subject`, `inReplyTo` | optional rationale, ≤ 600 |
| `split` | `sides`: exactly the two participants as keys, each 1 to `MaxGlobs` globs, each `ValidGlob`; a conflict thread | `globs`, `first`, `paths`, `base` | — |
| `build-on` | `base`: a participant; a conflict thread whose base-side party has a lane (a non-empty `Agent.LaneRef`, `task-03.md:560`) | `globs`, `first`, `paths`, `sides` | — |

New codes, joining task 09's ST.1 list: `split_not_a_partition` (details: `paths`, the uncovered
or doubly-covered ones), `build_on_no_lane`, `counter_own_proposal`. `resolution_not_held` and
`resolution_stale` are task 29's, reused.

**`Send` for a counter** (task 28's step 9, one branch): in the same transaction that stores the
counter, the countered proposal's id leaves `liveProposals` and the counter's id joins it; the
thread's `rounds` for the sender go up by one (task 28 (a) already counts `counter`); the stale-run
rule reads the counter's canonical resolution (task 28 (b)). If the countered proposal was
`awaiting-human` (task 29 step 2), the counter is refused `resolution_stale`: a proposal a human
is deciding on cannot be swapped out under them. A counter is **not** an accept, so `Resolve` is
not called.

**`checkSplit` and `applySplit`.** `CheckProposal` for a split, at propose or counter time and
again in `Resolve` step 3: the partition rule (decision b) against the conflict's paths
(`verbs.Conflicts.Open`, task 22), and `Engine.Validate` on each side's globs (breadth). Apply
(task 29's step 4), for each side S in key order: for each live lease L of S's side whose globs
match a conflict path, `Reissue(L, keep(L) ∪ Sides[S])`, where `keep(L)` is L's globs matching no
conflict path; a side with none gets `Engine.Request` of an `intent` lease on `Sides[S]` for its
party agent. Close (task 29's step 5): conflict → `resolved`, resolution `split`.

**`checkBuildOn` and `applyBuildOn`.** At propose or counter time: `base` is a participant and
its party agent has a lane (`build_on_no_lane`). On accept, before `Classify`: find the base
party's `laneRef` and the mirror's tip for it; `BuildOnPort.LaneChanges(…, PackageScriptMembers)`
→ `Action.BaseTip`, `AgentConfig = buildOnLines(d)`, `AgentConfigKnown = true`; an error →
`AgentConfigKnown = false`, lines empty. `Classify` → `build-on`, a decide for the builder's human
(always; `task-30.md:203`). Apply (`ApplyApproved`, after the human chose *apply*): (1) the base
lane's mirror tip still descends from `BaseTip` (else `resolution_stale`: the base lane was
rewritten); (2) `Bring(builderOwner, project, builderWorker's worktree, BuildOnRef(thread),
mergeBase, BaseTip)`; (3) one `land_hints` row `first = Base`, `second = Builder`, `first_task` the
base party's task; (4) conflict → `resolved`, resolution `build-on`; (5) `Outcome.BuildOn`.

**The build-on decide** (task 30's `options.go`, one row added to its table at `task-30.md:414-418`):

| Decide | Asked | Option 0 (default) | Option 1 | Option 2 |
|---|---|---|---|---|
| **Build on** (`AcceptNeedsDecision`, row `build-on`) | the builder's human | *Hold: nothing is brought in* · `hold` | *Bring <other>'s lane into <worker>* · `apply-proposal` — **offered only when `AgentConfigKnown`** | *Do not build on it* · `decline-proposal` |

The question, Fleet's template, ≤ 300 runes, from typed fields only: *"<other>'s orchestrator and
yours agreed that your worker <ag-…> builds on <other>'s lane <wt-…> at <sha7>. It changes N
agent-config files, listed below."* With `AgentConfigKnown` false: *"… The agent-config files it
changes could not be listed, so it cannot be applied from here."* N = 0 says *"It changes no
agent-config files."* The lines themselves go in the record's new `agentConfig` field, one entry
per path, never folded into the question, so each renders on its own line (task 54's NEEDS YOU
row) and no path is truncated.

**Hub delta: `agentConfig` on a decide** (task 03's `team.go`, task 07's route 64, a migration
after task 04's; this plan's own Hub code, after `feat/hub`'s sync-merge):

```go
// control-plane/internal/hub/hubapi/team.go (modify)

// EscalationPath is one agent-config line of a Fleet-written decide.
type EscalationPath struct {
	Path    string   `json:"path"`              // teamtext.ValidPath, ≤ 200
	Kind    string   `json:"kind"`              // instructions | agent-settings | git-hooks | package-scripts
	Status  string   `json:"status"`            // A | M | D
	Members []string `json:"members,omitempty"` // package-scripts only; each of task 49's four
}

// CreateEscalationRequest gains (after Deadline):
//	AgentConfig []EscalationPath `json:"agentConfig,omitempty"` // ≤ 50; decide only; Fleet only
// Escalation gains:
//	AgentConfig []EscalationPath `json:"agentConfig"` // [] never null

const DirectiveBuildOn DirectiveKind = "build-on" // CreateDirectiveRequest gains Ref *string `json:"ref,omitempty"`
```

Route 64 refuses `agentConfig` when `fromAgentId` is present (an Orchestrator cannot author the
lines a human reads as Fleet's), on an `inform`, past 50 entries, or with a bad path, kind or
status: 400 `bad_request`, `field: "agentConfig"`. The migration:

```sql
ALTER TABLE team_escalations ADD COLUMN IF NOT EXISTS agent_config jsonb NOT NULL DEFAULT '[]'
  CHECK (jsonb_typeof(agent_config) = 'array' AND jsonb_array_length(agent_config) <= 50);
ALTER TABLE team_directives ADD COLUMN IF NOT EXISTS ref text
  CHECK (ref IS NULL OR ref LIKE 'refs/panacea/build-on/th-%');
```

The table names are task 04's as written in its part (**UNVERIFIED**: task 04's part is not on
disk as this is written; the builder uses whatever task 04 named them). The `kind` CHECK on
directives, if task 04 has one, gains `'build-on'`.

**The directive.** `panacea direct <ag-…> build-on --ref refs/panacea/build-on/<th-…>` (task 18's
CLI, task 31's `DirectArgs` gains `Ref string` for this kind only). Task 31's handler admits it,
from either turn origin, iff an applied `Outcome.BuildOn` has `Worker == To` and `Ref == Ref`
and the caller is `Outcome.BuildOn.Builder`; otherwise `bad_request`. It is typed, so it passes
the taint rule the way `narrow` does (`task-31.md:473`). The worker's instructions (task 41's
`worker-cloud.md`) gain one fixed paragraph: *merge the named ref into your branch with `git
merge --no-ff`, run the project's checks, and report* — the worker's own act in its own clone.

- [ ] **Step 1: Write the tests, each with the mutant it kills**

On task 10's `memstore`, task 08's Hub fake, task 30's `Service` with a fake `Sources`, and a fake
`BuildOnPort`. `oA` is Alice's Orchestrator, `oB` Bob's; `wA`, `wB` their party workers.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestCounterPastTheRoundCapEscalates` | `oA` proposes, `oB` counters, `oA` counters, `oB` counters, `oA` counters, `oB` counters: all stored, `oB` has 0 left; `oB`'s fourth counter → `rounds_exhausted`, not stored, thread `escalated`, `ThreadExhausted(rounds)` once, two decides | a counter that does not count; the cap skipped for counters |
| `TestCounterReplacesTheProposal` | after `oB` counters `ms-1` with `ms-2`: `liveProposals == [ms-2]`; `oB` accepting `ms-2` → `bad_request` (own); `oA` accepting `ms-1` → `bad_request` (not live); `oA` accepting `ms-2` → `Resolve` with `ms-2`'s resolution | both proposals left live; the old one applied |
| `TestCounterOfOwnProposalRefused` | `oA` counters `oA`'s own proposal → `counter_own_proposal` | counter used as an un-counted withdraw-and-propose |
| `TestCounterRepeatingAResolutionIsStale` | propose X, counter Y, counter X: the third is not new; with one more non-new counting message the thread closes `escalated` (task 28 (b)) | canonical form ignored for counters |
| `TestCounterOfAnAwaitingHumanProposalRefused` | a `proceed-both` on tier H accepted (decide pending); `oB` counters it → `resolution_stale` | a proposal swapped under a human's decision |
| `TestSplitMustPartitionTheConflict` | conflict paths `a.go`, `b.go`: sides `{oA: [a.go], oB: [b.go]}` stored; `{oA: [a.go], oB: [a.go]}` → `split_not_a_partition` naming `a.go`; `{oA: [a.go], oB: []}` → refused naming `b.go`; a third key → `bad_request` | overlap or gap accepted |
| `TestSplitReissuesEachSidesLeases` | `wA` holds `auth/**`, `wB` holds `auth/session.go`; split `{oA: [auth/login.go], oB: [auth/session.go]}` on conflict path `auth/session.go`: `wA`'s lease `yielded`, a new one `[auth/login.go]`; `wB`'s re-issued `[auth/session.go]`; no other lease moved | a split that touches unrelated leases; a merge of old and new globs |
| `TestSplitMovingAcceptorsEditsIsADecide` | `wB` edited `auth/session.go`; the split gives it to `oA`; `oB` accepts → `Ruling{decide, accept-split-moves-edited, [oB]}`, nothing applied; with `wB`'s edits all inside `Sides[oB]` → act, both informed | the edited-path test missing (the task 30 stub) |
| `TestBuildOnAcceptWithoutTheHumanAppliesNothing` | `oB` proposes `build-on base: oB` (Alice's side builds on Bob's lane); `oA` accepts: one decide, to Alice's human only; the fake port saw `LaneChanges` and **no** `Bring`; no lease event, no conflict patch, no `land_hints` row, no directive admitted for `wA` | build-on applied on the agents' accept |
| `TestBuildOnDecideListsAgentConfigPaths` | `LaneChanges` → `M .claude/settings.json`, `M src/x.go`, `package.json` members `["scripts"]`: the Hub fake's decide has `agentConfig` `[{.claude/settings.json, agent-settings, M}, {package.json, package-scripts, M, [scripts]}]`, and `src/x.go` in neither the lines nor the question | lines folded into the question; a non-config path listed; package.json dropped |
| `TestBuildOnUnlistableWithholdsApply` | `LaneChanges` errors: the decide has options `hold` and `decline-proposal` only, and its question says the files could not be listed | "could not list" read as "none" |
| `TestBuildOnLockedAgainstFloorsAndErrors` | a user row, a team floor and a `Sources` error each leave the `build-on` ruling `decide` | an unlocked always-decide row |
| `TestBuildOnApplyBringsExactlyTheApprovedTip` | Alice chooses `apply-proposal`; the base lane has since moved to a descendant: `Bring` gets `BaseTip`, not the new tip; ref `refs/panacea/build-on/<th>`; one `land_hints` row `first = oB`; conflict `resolved`, `build-on`; a rewritten base (not a descendant) → `resolution_stale`, nothing brought | the latest tip brought; no hint |
| `TestBuildOnDirectiveOnlyForTheAppliedRef` | `oA` `direct wA build-on --ref <the ref>` in a Fleet turn → issued; another ref, another worker, or `oB` calling → `bad_request` | a free-form ref directive through a typed kind |
| `TestRoute64AgentConfigIsFleetOnly` | route 64 with `fromAgentId` and `agentConfig` → 400; Fleet's own with 51 entries → 400; with 2 → 201 and the read returns them, `[]` when absent | an Orchestrator forging lines |

Task 28's `TestKindsNotEnabled` (or whatever its suite names the `kind_not_enabled` rows) is
**changed** here, not deleted: its three rows now expect the validation above. Both modules'
fixture round-trip tests read the four new fixtures.

- [ ] **Step 2: Run the tests and watch them fail**

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/team/control-plane && GOPROXY=off go test ./internal/fleet/protocol/ ./internal/fleet/escalate/ ./internal/fleet/leases/ ./internal/fleet/verbs/ -run "Counter|Split|BuildOn|Reissue"
```

Expected: failures, not compile errors, for the counter and split tests (`kind_not_enabled`
where a result is wanted), and `undefined: BuildOnPort`, `(*Engine).Reissue undefined`.

- [ ] **Step 3: `Reissue` in `leases/yield.go`; run task 23's transition-table test unchanged.**
- [ ] **Step 4: The three validation rows and `Send`'s counter branch; run task 28's suite.**
- [ ] **Step 5: `split.go`, then `KeyOf`'s split case; run task 29's and task 30's suites.**
- [ ] **Step 6: `buildon.go`, the decide row and its question, the directive kind; the Hub delta
  (types, route 64's check, the migration); the CLI verb; the instruction paragraph.**
- [ ] **Step 7: Run everything**

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/team/control-plane && GOPROXY=off go test ./... && GOPROXY=off go vet ./... && gofmt -l internal
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/team/sandboxd && GOFLAGS=-mod=vendor go test ./internal/panaceacli/... ./internal/control/... && GOFLAGS=-mod=vendor go vet ./...
```

Expected: every package `ok`; tasks 23, 28, 29, 30 and 31's suites pass with only the changed
`kind_not_enabled` rows; the Hub's route 64 integration test passes against its test database
(skipped, and said so, without one); `gofmt -l` prints nothing.

- [ ] **Step 8: Commit**

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/team && git add control-plane/internal/fleet/protocol/split.go control-plane/internal/fleet/protocol/split_test.go control-plane/internal/fleet/protocol/buildon.go control-plane/internal/fleet/protocol/buildon_test.go testdata/panacea-control/v1 && git commit -F .superpowers/sdd/team/msg/task-62.txt -- control-plane sandboxd/internal/panaceacli testdata/panacea-control/v1
```

The real-cluster `build-on` decide listing an agent-config path is task 69's, because it needs
task 58's mirror pod and two signed-in people.

**What this task does not claim:**

- **That the worker merges well.** Fleet brings a ref in and admits one typed directive; the merge
  is the worker's, it can conflict, and a failed merge is the worker's `report`, not Fleet's.
- **That listing agent-config paths makes a build-on safe.** It tells the human what could change
  their agents' behaviour; the managed files still lock hooks, permission rules and
  `apiKeyHelper` (§7.3), and anything else in the lane runs in the builder's sandbox as code.
- **That a split is fair.** It only checks the partition and the acceptor's edits. Right of way
  (task 23) still applies to the re-issued leases.
- **`counter` on a free thread with no resolution to counter.** A counter needs a live proposal.

**Gaps against the skeleton, raised here:**

1. **A typed `build-on` directive** is not in task 03's or task 31's kinds; this task adds it
   (Hub delta and Fleet). Without it, the builder's Orchestrator could only reach its worker with
   free text, which a Fleet-started turn turns into a decide (§7.2), so every build-on would ask
   twice.
2. **Who decides a build-on.** §6.5 says "decide" without naming whose; task 30 made it the
   builder's human (`Who = Builder`). The base side's human hears of it through task 29's DECIDED
   WITHOUT YOU. The owner may want both.
3. **`agentConfig` on the Hub's escalation record** is a contract change (task 01's §7 route 64)
   that task 01's part did not foresee; the contract doc gains the field in this task.
4. **Split and build-on need a conflict thread.** §6.2 does not say whether a free thread may
   carry either; this part refuses both on a free thread (no paths to partition, no party lane).
