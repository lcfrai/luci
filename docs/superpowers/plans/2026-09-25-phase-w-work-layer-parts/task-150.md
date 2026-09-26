### Task 150: tracker sync, part 1: the engine, the integration principal, links by `(source, externalId)`, one way in to `triage`

W5, stream WH. Seam **SW.6**, consumed (task 15's import batch). Depends on task 15 (the import
path into `triage`, *not tied · imported*) and task 145 (the W5 guards: no sync principal writes a
state, a fact or `done`). Estimate 0.375 (3 h). Closes nothing alone; the three adapters close
**L-202** (Azure Boards, task 151), **L-215** (GitHub Issues, task 152) and the W5 half of **L-220**
(Jira, task 153). Every `server/` and `store/` path is **UNVERIFIED until merge** (rule H1).

A team that already keeps work in Azure Boards, GitHub Issues or Jira should not have to move it by
hand, and should not be made to stop using the old tracker the day it starts here. Sync brings
their items **in**, one way, into the team's Triage as *not tied · imported* issues, and keeps
bringing new ones. It never writes back (part 4 adds comments only), and it **never moves our
issue**: when an item is closed or reassigned upstream, our issue says so, *closed in Azure Boards
at 11:02*, and a person decides. This task is the engine and its rules; tasks 151-153 are the three
adapters that talk to each tracker.

Eight decisions the scope left open, made here.

1. **The engine owns the loop; the server owns the writes.** `worksync` cannot import
   `internal/hub/server` (the server imports `worksync` for the janitor), so the engine talks to a
   `Sink` the server implements. The engine pulls a page from the adapter with **no transaction
   open**, then hands it to `Sink.ApplyPage`, which runs the whole page in **one** transaction:
   imports, links, upstream changes, the run's page count and the cursor. The cursor moves in the
   same commit as the page, so *"the cursor advances only after the page commits"* holds by
   construction, and a crash between pull and commit re-reads the page next run, which links make
   harmless.
2. **The integration principal is the connection, and it is marked in the database.** The
   `ApplyPage` transaction sets `hub.work_actor_kind = 'integration'` (task 07's setting) and a new
   `hub.work_sync_connection = 'tc-…'`. Task 145's guard (below, a requirement on it) refuses, with
   `principal_cannot_move_state`, any `UPDATE OF state` on `issues`, any `issue_revisions` insert
   past revision 1, any `issue_labels` insert and any assignee change while that setting is set.
   Task 07's own guard already refuses a fact state to any non-person; this one is stricter because
   a sync may not move **any** state, which task 07 allows an `integration` for `backlog`
   (`task-07.md:510`). The Go code never tries; the guard is there for the day it does.
3. **Who it acts as, and what the events say.** No Entra token: the Hub's own process acts, on a
   janitor transaction with no principal, so every event it appends has `actor: null` (Hub task
   24's rule, as task 121 cites it). The issue's `created_by` is the connection's creator, as task
   121 makes a recurring issue's (`task-121.md:241-246`), so the list reads *"imported by Ada's Azure
   Boards connection"*. Task 13's `IssueCreatedData` has no `via` field and this task does not
   change a W1 type; the connection's id and creator ride on one **`tracker.page`** event per
   committed page, `{connectionId, run, created, existing, upstream, by}`, exactly as task 121 puts
   a schedule's author on `template.schedule_ran` (decision 5 there). The scope's *"`data.via` the
   `tc-` id"* is held by that event, not by `issue.created`; this is recorded in *Gaps*.
4. **A link is `(connection, externalId)` → issue, and it outlives the connection.** Idempotency is
   task 15's: `IssuesByExternal` on the connection's project finds an item imported before, and
   `issues_external_once` is the second layer (`task-15.md:417-418`). The link row adds what the
   issue row cannot hold: which connection brought it, its upstream URL at link time, and the last
   upstream change. `DELETE /v1/trackers/{id}` **disconnects** (sets `disconnected_at`, stops runs,
   drops the credential reference) and keeps every link, so an imported issue still reads *from
   Azure Boards, 1432* and its link still resolves.
5. **Upstream changes are shown, never applied.** An adapter reports an item it has seen before as
   an `UpstreamChange{externalId, field, at}` with `field` one of `closed`, `reopened`, `assignee`,
   `title`, `description`, `deleted`: **which** thing changed, never the new value, since a new
   title is a customer's words and would be written by nobody. The link keeps the latest change;
   the run log keeps every one; the issue view (task 170) reads *closed in Azure Boards at 11:02*.
   Whether an upstream edit should ever update our text is a choice recorded for the owner
   (*does not claim*).
6. **"Run now" asks; the janitor runs.** `POST /v1/trackers/{id}/run` sets `run_requested_at` and
   answers 202; the next sweep runs it. A request handler never waits on a tracker's network.
   Scheduled runs happen every `PANACEA_APP_HUB_SYNC_INTERVAL` (default 5 minutes) after the last
   finished run.
7. **A page that fails validation fails the run and moves nothing.** An adapter maps an item into
   SW.6's `ImportItem`; `hubapi.ValidateImportBatch` (task 15) checks the page. A page that fails
   is recorded on the run as `failed` with the first failing field, the transaction rolls back,
   the cursor stays, and the next run retries the same page. A tracker that keeps sending an item
   we refuse therefore stops that connection, visibly, until an adapter fix; skipping the item
   would lose a customer's ask without a word.
8. **Connections need an id prefix, and the skeleton has none.** The scope names `tc-`; it is not
   in the skeleton's list (`:185-187`) or 05 §3.6, and collides with none of them. Task 02's
   `workids.go` and its disjointness test must add it (gap 1).
