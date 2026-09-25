### Task 15: the import route: a normalised batch into `triage`, *not tied · imported*

W1, stream WH, seam **SW.6** (the import batch: one normalised item per external issue, idempotent
by `(source, externalId)` per project; consumed by task 25's adapters, task 69's dialog and W5's
sync, tasks 150-153). Depends on task 11 (labels and the team policy this route must override).
Hub anchors were read in the `feat/hub` worktree at `9f2fa31` (the contract, `hub-contract:N`;
Hub task 14's part, `H14:N`). `server/` and `store/` are **planned, not on `feat/hub`**, and every
path under them is **UNVERIFIED until merge** (rule H1).

This is the first step of the W1 demo: *"Import a Linear backlog or a CSV; it lands in Triage as
not tied · imported"* (skeleton, *The demo W1 must pass*). The control plane (task 25) reads
Linear's public API or a CSV file and turns every external issue into one `ImportItem`; this route
takes a batch of them and makes issues. Three facts about an imported issue matter more than the
rest, and every one is a test:

- **It lands in `triage`, whatever the team's default** (L-9: intake always lands in triage). An
  imported backlog is somebody else's list; a person accepts each item into the team's flow.
- **It reads *not tied · imported*.** It serves no criterion yet. That is an honest open end on the
  provenance thread, named, not a silent one (05 §2.3 point 4). Sync does not cut the thread;
  attaching a criterion later (task 46's `T`) ties it.
- **No external state comes with it.** `ImportItem` has no state field (`task-03.md:543-552`), so
  a Linear issue that was *Done* arrives in `triage`, not `done`. `done` means the platform
  observed its own evidence at its own landed commit (skeleton rule 2); an import can never claim
  that, and a strict decoder refuses a `"state"` smuggled into an item.

Five decisions the scope left open, made here.

1. **The batch is all or nothing for its new items.** Every item is validated before anything is
   written; one bad item is 400 naming `items[n].field` and writes none of them. The Hub's brief
   batch works the same way (hub-contract's H3 row). A database failure part-way rolls the one
   transaction back. Items already present are not "written", so they never cause a refusal.
2. **Idempotency is per project, by `(source, externalId)`, through task 06's
   `issues_external_once`** (a partial unique index on `(org_id, project_id, external_source,
   external_id)`, `task-06.md` at the `issues` table). The lookup and the insert run under the
   org's audit lock (hub-contract §10.6), so two imports of the same file cannot both create; the
   index is the second layer. A batch that names one `externalId` twice is 400 naming the second
   (`items[n].externalId`): task 25 already refuses that case before it sends
   (`task-25.md:177-180`), and here it is a clear refusal rather than a 500 from the index.
3. **Labels are matched, never created.** A name matches a live label of the project's team
   case-insensitively; an unmatched name is reported in `unmatchedLabels`, sorted and
   deduplicated case-insensitively (the form task 25 shows, `task-25.md:195`), and nothing is
   created, because creating a label is a team admin's write (task 07's row) and an import is a
   member's. Only **created** items' labels are matched; an existing issue is not touched, so its
   labels are not read.
4. **The importer is not subscribed** to the issues they import (task 13 names the exception):
   they moved a backlog; they did not write each issue.
5. **The external `createdAt` is kept, not used as ours.** `issues.created_at` is when the issue
   entered the Hub, which is true and is what the audit says. The item's `createdAt` (RFC 3339, or
   `""`) is stored as `external_created_at` and read back as `external.createdAt`, so the issue
   view can say *created in Linear on …* without the Hub claiming an event it never saw.

**The one thing this task must not get wrong** is that an import writes nothing a person did not
ask for and claims nothing the platform did not see: no fact state, no label, no subscription, no
thread. A test kills each of those four.

