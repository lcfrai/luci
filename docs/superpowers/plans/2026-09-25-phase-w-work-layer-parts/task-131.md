### Task 131: phase 7 delta: notes by criterion, freeze per environment, path filters, *shipped* on the issue

W4, a delta on phase 7 under **rule H2: phase 7's files win**, and every phase 7 name below is
**UNVERIFIED until `feat/phase-7-deploy` merges**. Seam **SW.9** (the `Panacea-Issue: is-…` trailer
grammar task 71 already walks). Depends on task 71 (issues in the release manifest). Estimate 0.375
(3 h). Closes **L-145** and **L-148**; **L-147** with task 139 (the drafted prose); **L-144** once
the freeze record lands in the Hub (gap 1).

After this task a release's drawer lists what it settled, by criterion, with the issues that served
each and the evidence keys, plus every other issue it shipped; an admin can freeze an environment,
and Ship refuses a frozen one before any Panacea call, saying who froze it and why; a project can
commit `.panacea/release.yaml` so only commits touching its paths add issues to a release; and an
issue lists every environment it reached, newest first, with the confirm time and pinned brief.

**Phase 7 names verified against its plan on `feat/phase-7-deploy` (2026-09-26).** `{P}` is
`/v1/projects/{projectId}/deploy` (P7 `task-11.md:27`). The file map rows the scope cites by line
are at: `ship/ship.go` `2026-09-23-phase-7-deploy.md:585` (task 09), `sessions/sessions.go` `:587`
(task 19), `api/deploy_drill.go` `:593`, `routes_audit_test.go` `:596`, `ReleaseDrawer.tsx` `:613`
(task 17), `GovernancePanel.tsx` `:614` (task 18). The scope's `:578`-`:607` are stale by 7-8 lines.
`Precondition`, `Orchestrator.Ship`, `Readiness` are P7 `task-09.md:134-179`; `sessions.Range`,
`Result`, `Reader.Read` and the *previous release* rule are P7 `task-19.md:38-171`. `desktop/src/
deploy/api.ts` is named by the scope; its P7 row was not found in the file map by this part
(**UNVERIFIED**).

**Decisions made here, so nobody re-derives them:**

1. **Notes use phase 7's range, not a second one.** *The previous confirmed release in that
   environment* is task 19's `From`: the commit the environment served just before this row went
   live (`task-19.md:108-137`, rollbacks included). So the route is `GET {P}/notes?deploymentId=…`
   beside task 19's `GET {P}/sessions?deploymentId=…`, not the scope's
   `…/releases/{releaseId}/notes`: phase 7 keys every drill by `deploymentId`, and rule H2 says its
   shape wins. When task 19 answers `previous_unknown`, the notes answer the same state and list
   nothing as *settled by this release*: never a guess.
2. **A criterion is settled by this release** when phase 3's `Evaluator.States` (P3
   `task-15.md:159`, *"phase 7 for a release at Deployment.Commit"*) reads it `settled` at
   `Commit`, and not `settled` at `From`. With `From ""` (no earlier release), every criterion
   settled at `Commit` is listed. The criteria are those of the release's pinned brief revision
   (the Hub's pin, P7 task 09's reserve), resolved by phase 4's resolver as task 23 resolves them.
3. **Every shipped issue appears exactly once.** An issue serving a criterion settled by this
   release is listed under that criterion (under each, if it serves several); every other issue
   of the range is in `alsoShipped`, never dropped. Titles are each issue's head revision title
   read from the Hub at request time, `text·H`, never stored in the manifest (task 71's rule
   stands: the manifest holds ids).
4. **No model text in the notes.** The route calls no gateway and returns only Hub text and ids.
   Task 139 drafts prose from this JSON; nothing here does.
5. **A freeze is a Hub record, and SW.20 has none (gap 1).** The shape is proposed below for
   tasks 114, 116 and 124. Until it lands, `ship.go` reads freezes through a one-method interface,
   `Freezes`, satisfied in tests by a map and in the product by the Hub client once 124 adds it;
   with no implementation wired, **no environment is frozen**, and the Governance panel says the
   control is not yet available rather than drawing a switch that does nothing.
6. **The freeze check is a precondition, after the environment is named and before the reserve.**
   P7 task 09 names the environment in step 4 (`task-09.md:308-312`) and reserves after; the freeze
   check sits between them, so a frozen Ship reserves nothing, builds nothing and calls Panacea not
   at all. It answers **`409 deploy_frozen`**, its own code (like `deploy_environment_invalid`),
   with `details {environment, by, at}`. `Readiness` gains `Reason "frozen"` when its own
   `Environment` is resolved and frozen, so the Ship bar says why before the person clicks.
7. **The path filter is versioned with the code.** `.panacea/release.yaml` is read at `Commit`
   with `git show <Commit>:.panacea/release.yaml` (Class Read). Absent file: no filter. A file
   present but unreadable or with a bad glob: the notes and issue walk answer
   `state "filter_invalid"` with the parse error, and Ship is **not** blocked (a filter shapes a
   list, not a release). This home for the filter is this part's choice; 05 names none
   (**UNVERIFIED pending the owner's review**).
8. ***Shipped* is a list of facts, per environment.** For each environment in task 08's view,
   the issue reached it at the first succeeded deployment, oldest first, whose commit has one of
   the issue's trailer commits as an ancestor; `servingNow` says whether the environment's current
   commit still has it (a rollback can take it away). Staging never reads as production: each row
   names its own environment. Newest first by the reach time.
