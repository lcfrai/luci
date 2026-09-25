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
