### Task 79: estimates, due dates, rank, move with redirect, the resolved-blocker rule

**Slice:** W2. **Size:** 2.5 focused hours (an estimate, not a measurement). **Stream:** WH.
**dependsOn:** 74 (the `issue_heads` plan columns, `issue_aliases`, the W2 guards), after a
sync-merge of `feat/hub` (rule H1). **Seams:** consumes **SW.18** (task 72's `IssuePlan`,
`IssueAlias`, `TeamPlanPolicy`, `TeamPlanPolicyPatch`, `EstimateScale`, `EstimatePoints`,
`ValidEstimate`, `ValidDate`, `ValidRank`, `RankMaxLen`, `Change[T]`, `PatchIssuePlanRequest`,
`RankRequest`, `MoveIssueRequest`, and the codes `estimate_off_scale`, `date_invalid`,
`rank_neighbours`, `children_open`), **SW.4** (task 10's state move) and task 11's relations;
adds four event kinds to **SW.5** (task 13's registry; additive, so SW.5's freeze allows it).

Five small planning facts, each a person's act: how big an issue is, when it is due, where it sits
in the team's shared order, which project it lives in, and what happens to the issues it blocked
once it closes. None of them is in the issue-revision canonical form (task 72 block 4), so none
re-hashes a revision, and each is one `PATCH`- or `POST`-shaped write with one event. This task is
the Hub half; task 89 is the desktop.

**Anchors, re-read on 2026-09-26.** In this branch's parts (not yet code, so **UNVERIFIED until
merge**): task 72's scales and points (`task-72.md:143-164`), `RankMaxLen` and `rankRE`
(`:249`, `:255`), `ValidDate` and `ValidEstimate` (`:285-301`), the codes (`:352-370`),
`IssuePlan`, `IssueAlias`, `TeamPlanPolicy`, `TeamPlanPolicyPatch` (`:390-430`), the bodies and
`Change[T]` (`:667-736`), the `hubclient` methods (`:935-940`); task 03's relation kinds
(`task-03.md:147-152`); task 06's `issue_relations` table (`task-06.md:259-269`); task 08's
by-number route (`task-08.md:149`, `:270`, `:320`) and `issue_not_found` (`:171`); task 10's
codes (`task-10.md:242-244`) and its `hub.work_actor_kind` setting (`:359`); task 11's relation
delete and `issue.unrelated` (`task-11.md:343`); task 13's `IssueRelationData`
(`task-13.md:166-170`). In the parity docs: 05 §1 rows 14, 16, 25, 35, 37 and 103
(`05-design.md:123-237`) and §3.6's one-event rule (`:798-812`). **Task 74's part is not written
at this writing**, so its column names (`issue_heads.estimate`, `.due_on`, `.rank`;
`issue_aliases (project_id, number)`) are the scope's (`tasks-61-80.md:479-483`),
**UNVERIFIED**; a different spelling in task 74 is a find-and-replace in this task's store files.

#### Decisions the scope left open, made here

**(a) A scale change rewrites no estimate.** Changing a team's `estimateScale` through task 11's
`work-policy` route leaves every stored estimate as it was; the head reads it, and task 89 marks
one that is off the current scale. Only a write is checked: `ValidEstimate` against the team's
policy **at write time**. Rewriting on a scale change would be a mass edit no person made issue
by issue, with one event for many rows. Clearing (`"estimate": null`) is always allowed, on scale
`none` too.

**(b) A plan patch that changes nothing writes nothing.** `PATCH /plan` with both fields absent,
or with values equal to the stored ones, answers 200 with the head and appends no event (task 75's
"the same list twice" precedent). A patch that changes one field or both writes one
`issue.planned` event.

**(c) The rank key is the server's, in three shapes.** `after` is the issue above, `before` the
one below (task 72's comment). Between two neighbours the key is the digit-wise midpoint
(`rankBetween`); at the bottom of the order (`before: null`) it is the next step in a four-digit
grid (`rankAfter`); at the top (`after: null`) the previous step (`rankBefore`). Stepping, not
bisecting, at the two ends matters because every new issue joins at the bottom: bisection there
adds one character per five creates and would force a rebalance every 140 or so issues, where the
grid gives about 839,000 bottom slots from the first key (the arithmetic is under *Rank*).

**(d) Every head has a rank.** A new issue joins its team's order at the bottom, in the same
transaction as task 08's create (one call, `store.BottomRank`). A move to another team's project
takes the target team's bottom; a move inside one team keeps its key (the team's order spans all
its projects). Task 74 backfills W1 heads in `(created_at, id)` order with `rankSpread`. Both are
asked of those tasks here and reported as gaps.

**(e) A move is refused while work is bound.** From `started` or `in_review` a lane and a branch
in the source project's repository are live, so a move is 422 `state_not_allowed` with
`details.cause: "move"` (task 10's code; no new code). Every other state moves, `landed` and the
closed states included: proof and trailers cite ids, not numbers (05 §3.6).

**(f) What a move clears.** Kept: the id, revisions, comments, `serves` (criterion ids),
relations, priority, estimate, due date, the assignee if a member of the target team (else
cleared). Cleared when the teams differ: team labels (org labels stay), `cycleId`, `subState`.
Always cleared: `featureId` (features are per project). A parent is cleared; an issue that has
children is refused 409 `children_open` with `details.open` counting every child, any state,
because task 78's parent rule keeps a child in its parent's project. The event names what was
cleared.
