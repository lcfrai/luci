### Task 78: sub-issues and the parent rule; templates `tp-`; label groups and org labels

**Slice:** W2. **Size:** 3 focused hours (an estimate, not a measurement). **Stream:** WH.
**dependsOn:** 74 (`issue_heads.parent_id` and its loop trigger, `templates`, `label_groups`,
`labels.group_id`, the nullable `labels.team_id`, the exclusive trigger), after a sync-merge of
`feat/hub` (rule H1). **Seams:** consumes **SW.18** (task 72's `Template`, `LabelGroup`,
`LabelPlan`, `LabelPlanRequest`, `CodeParentLoop`, `CodeParentOtherProject`, `CodeChildrenOpen`,
`CodeExclusiveLabelGroup`, `CodeTemplateNotFound`) and **SW.4** (task 10's done rule); adds one
cause to SW.4, one `Scope` to SW.5, and six event kinds, all additive.

Three small things a team expects on day one of planning. **Sub-issues**: a parent issue holds
children, and Linear closes the parent when the last child closes. Ours closes it only when the
children are closed **and** every criterion the parent serves is settled (L-21, `05-design.md:130`),
so a parent cannot read *done* while the brief it answers is still open. **Templates**: a team or
the org keeps issue shapes that may pre-fill `serves`, so a new issue starts on the thread.
**Label groups**: a group of labels, optionally exclusive (one *Type* per issue), and labels that
belong to the whole org.

**Anchors, re-read on 2026-09-26.** In the Hub worktree at `6463e568` (read, not merged here):
evidence and its `verdict` `settled` | `contested` (`hub-contract.md:1060-1083`); roles, and the
org admin who reads no project content (`:331`); the matrix columns (Hub `task-16b.md:30-35`). In
this branch's parts (**UNVERIFIED until merge**): task 72's `Template` (`task-72.md:581-598`),
`LabelGroup` (`:600-607`), `LabelPlan` and decision (c) on `Label.TeamID` (`:44-47`, `:432-440`),
codes (`:354`, `:360`, `:362`, `:364-365`); task 10's route, proof words and `DecideProof`
(`task-10.md:47-55`, `:402-435`); task 11's label store and the `labels` list path
(`task-11.md:280-284`, `:369-377`); task 13's `Scope` (`task-13.md:234-240`) and registry
(`:250-270`); task 07's `werase` cells (`task-07.md:388-390`). **Task 74's part is not written at
this writing**; its columns and trigger names are the scope's (`tasks-61-80.md:477-483`),
**UNVERIFIED**.

#### Decisions the scope left open, made here

**(a) Children are one level of people's issues, in the same project.** A parent and its child
share `project_id` (else 422 `parent_other_project`); an issue may be a parent and a child at once
(nesting is allowed, since Linear allows it), but no chain may loop (422 `parent_loop`, checked by a
walk up the parent chain in Go and by task 74's trigger). The walk is capped at 32 steps; a longer
chain is refused as a loop (a chain that deep is a data error). Agent tasks below a person's issue
are team mode's (`tk-`, TM-05 §3.3) and never appear in this list (05 L-20).

**(b) "Settled", as the Hub can read it.** A criterion `(briefKey, criterionId)` is **settled for
the Hub** when the **latest** evidence record for it in the project (by `recorded_at`, then id) has
`verdict: settled`. 04 §5.3's full reading ("all of its evidence holds at the head") needs git,
which the Hub never reads; a later `contested` record un-settles it here too. The two readings may
differ when a settled record's commit is no longer at the head; that difference is phase 4's to
show (task 87's convergence words), and is stated in *does not claim*.

**(c) Two ways a parent reaches `done`, and one hold.**
- **A parent with its own work** (it `landed` through its own lane): task 10's evidence rule
  decides, and **open children hold it**, exactly as task 75's `after_landed` sub-state does: the
  evidence move is accepted, citations and proof stored, the state left `landed`.
- **A container parent** (it never landed; any state before `landed`): when the **last open child
  closes**, the server runs the **roll-up**: every child closed, at least one child `done` (a parent
  whose children were all cancelled was abandoned, and a person decides), and every criterion the
  parent serves settled (decision (b)). Then the parent moves to `done` with the new cause
  `children_closed`, **written as the person whose act closed the last child** (skeleton rule 2).
- **A hand-set `done`** on a parent with an open child is 409 `children_open`, `details.open` the
  count, nothing written.

**(d) The roll-up's proof word is the weakest of the done children's.** The order is `not_proven`
< `proven_self_attested` < `proven`; `no_criteria` children are neutral; all neutral gives
`no_criteria`. So a parent reads *proven* only if every child that served a criterion was proven,
and a parent of hand-closed children reads *done, not proven*, as they did. This is the
"inheritance" of L-20, and it keeps 05 §3.3's glyphs honest at the parent.

**(e) `children_closed` is a new fact cause, and it bends rule 2 on purpose.** Skeleton rule 2
says `done` needs the issue's own evidence at its own land commit. A container parent has no land
commit; its work is its children's, each proven under rule 2 on its own. 05 L-21 asks for the
roll-up, so the cause exists, the database guard checks it (no open child, at the time of the
write), and the proof word never claims more than the children proved (decision (d)). This is
**reported to the orchestrator** as a rule-2 amendment for the owner, not assumed.
