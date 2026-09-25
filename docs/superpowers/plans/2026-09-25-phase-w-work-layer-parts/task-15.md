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

Three consequences of the decisions, stated so nobody "fixes" them.

- **An erased import stays erased.** Task 14's erase blanks the title, the URL and (from this task)
  the author name, but keeps `external_source` and `external_id`: a tracker id is not a person's
  words. A re-import of the same file therefore finds the erased issue and answers it under
  `existing`; it never re-creates the text an org admin erased. The test is
  `TestAReimportDoesNotResurrectAnErasedIssue`.
- **The route answers 200, never 201**, as task 07's matrix row says (`task-07.md:463-468`) and task 16's
  `WorkRoute{"ImportBatch", …, Created: false}` (`task-16.md:155`): a batch may create nothing.
- **The reply holds no issue text.** `ImportResult` carries ids, numbers, external ids and unmatched
  label names only, so it is stored whole in `idempotency_keys` when the relay sends an
  `Idempotency-Key` (`s.WriteLimit`, `H14:123-126`), and no replay can re-serve an erased title.
  An import that answered heads would not be (task 14 put the title on the head).

**The body cap.** `httpx.MaxBodyBytes` is 1 MiB (`control-plane/internal/httpx/problem.go:22`,
verified at `feat/hub` `3b0dcad`), and 500 items of 16 KiB descriptions is 8 MiB. The route
therefore uses `s.WriteLimit(…, hubapi.MaxImportBatchBytes, …)` with 9 MiB, the second route after
Hub route 26 to name its own cap (`H14:123-125` says "only route 26" of the Hub plan; this plan
adds one, a delta on sibling code under rule H2). A body over it is 400 `field: "body"` before
`BEGIN`. What 9 MiB costs in Hub memory under load is **unmeasured**.

**Files:**

- `control-plane/internal/hub/hubapi/work.go` ⚑ (task 03's, additive): `ExternalRef.CreatedAt`,
  `ImportSources`, the `MaxImport*` limits, the `ErrImport*` errors, `ValidateImportBatch`,
  `ImportLabelNames`
- `control-plane/internal/hub/hubapi/work_test.go` ⚑ (task 03's): the `IssueHead` example gains
  `"createdAt"` inside `external` (task 03's `workExamples` has every field present, `:644-649`)
- `control-plane/internal/hub/hubapi/work_import_test.go` (new): the pure rules
- `control-plane/internal/hub/migrate/sql/NNNN_work_w1.sql` ⚑ (task 06's file; a section appended,
  or the next free number if task 06 has merged into `feat/hub`, rule H1)
- `control-plane/internal/hub/store/work_imports.go` (new)
- `control-plane/internal/hub/store/work_issues.go` ⚑ (task 08's): `IssueRow` gains
  `ExternalAuthorName` and `ExternalCreatedAt`; `InsertIssue` writes them; the head projection reads
  them
- `control-plane/internal/hub/server/work_imports.go` (new); `server/server.go` ⚑ gains
  `s.registerWorkImports(mux)` beside task 13's `registerWorkSubscriptions`
- `control-plane/internal/hub/server/work_issues.go` ⚑ (task 08's): `issueHead` fills
  `external.authorName` and `external.createdAt`
- `control-plane/internal/hub/server/work_imports_integration_test.go` (new, `-tags integration`)
- `control-plane/internal/hub/server/work_feed_integration_test.go` ⚑ (task 13's): `fullRun` gains
  one import carrying the canary in its title, description, URL and author name (`task-13.md:516`)
- `control-plane/internal/hub/hubclient/fake_work*.go` ⚑ (task 05's file): only where a rule here
  differs from what 05 wrote; the conformance rows are the arbiter. Task 05's part is not written
  at this writing, so its import rows are **UNVERIFIED**

**Interfaces:**

- Produces:
  - the route, in 07's list and 07's matrix row `"15"` (`task-07.md:463-468`): `POST
    /v1/projects/{projectId}/imports` (✓ for `M A O`; 403 `required: member` for `V`; 404
    `project_not_found` for `N X`)
  - `hubapi.ExternalRef.CreatedAt`, `hubapi.ValidateImportBatch`, `hubapi.ImportLabelNames`,
    `hubapi.MaxImportBatchBytes` and the other limits below
  - `store.IssuesByExternal`, `store.MatchLabels`, `store.InsertIssueLabels`
  - one `issue.created` event per created issue (task 13's `EventIssueCreated`, `task-13.md:122`,
    and `IssueCreatedData`, `:142-148`), `source: "intake"`, `state: "triage"`
- Consumes:
  - task 03: `ImportBatch`, `ImportItem`, `ImportResult`, `IssueRef`, `ExternalRef`,
    `ImportSourceLinear`, `ImportSourceCSV` (`task-03.md:240-244`, `:271-277`, `:398-411`, `:534-552`)
  - task 02: `ValidateIssueRevisionContent`, `IssueRevisionDigest`, `AuthorHuman`,
    `MaxIssueTitleChars`, `MaxIssueDescriptionBytes` (`task-02.md:204-256`)
  - task 06: `issues`, `issue_labels`, `labels`, `issues_external_once` (`task-06.md:155-196`,
    `:236-256`); task 08: `NextIssueNumber`, `InsertIssue`, `InsertIssueRevision`,
    `MoveIssueHead`, `issueHead` (`task-08.md:205-263`, `:400-416`); task 11: the project's team
    and the rule that intake never takes the policy's default (`task-11.md:369-372`); task 13:
    `emitWork` (`task-13.md:332`); task 14: `SetHeadTitle` (`task-14.md:206`), `erase_issue`
  - Hub task 14: `server.WriteLimit`, `server.Decode`, `ProjectRole`, `Require`, `(*Tx)`
    (`H14:117-138`); the org's write lock, hub-contract §10.6 (`:2035`)
- Consumed by: task 16 (`hubclient.Work.ImportBatch` and its relay), task 25 (the Linear and CSV
  adapters, `task-25.md:186-188`), task 69 (the import dialog), tasks 150-153 (W5's sync)

