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

#### Files

- `control-plane/internal/hub/worksync/engine.go`, `engine_test.go` (new): `Adapter`, `Sink`,
  `Engine`, `Pulled`, `UpstreamChange`, `Cursor`, and the fake adapter the tests use.
- `control-plane/internal/hub/worksync/links.go`, `links_test.go` (new): the pure page planner
  that splits a pulled page into new items, known items and upstream changes.
- `control-plane/internal/hub/server/work_trackers.go` (new): the five routes and the `Sink`.
- `control-plane/internal/hub/store/work_trackers.go` (new) and
  `server/work_trackers_integration_test.go` (new, `-tags integration`).
- ⚑ `control-plane/internal/hub/server/janitor.go`: one sync step after task 149's, outside the
  per-org transaction, under the advisory lock.
- ⚑ `control-plane/internal/hub/server/work_imports.go` (task 15): `importOne` gains a `createdBy`
  argument (the principal's id on the route, the connection's creator here); nothing else changes.
- ⚑ `hubapi/work.go` (task 15's `ImportSources`), ⚑ `hubapi/work_intake.go` (task 143),
  ⚑ `hubapi/workevents.go`, ⚑ `hubapi/workids.go` (`tc-`), ⚑ `server/work_matrix_rows_integration_test.go`.

#### The tables and the guard this task needs from task 145's migration (requirement; complete)

```sql
-- ── task 150 (in task 145's NNNN_work_w5.sql): tracker connections, links, runs ─────
ALTER TABLE issues DROP CONSTRAINT issues_external_source_known;          -- task 06's; W1 had two
ALTER TABLE issues ADD CONSTRAINT issues_external_source_known
  CHECK (external_source IS NULL OR external_source IN ('linear', 'csv', 'azure_boards', 'github', 'jira'));

CREATE TABLE work_tracker_connections (
  id               text PRIMARY KEY CHECK (id ~ '^tc-[0-9a-f]{16}$'),
  org_id           text NOT NULL REFERENCES orgs (id),
  team_id          text NOT NULL,
  project_id       text NOT NULL,
  source           text NOT NULL CHECK (source IN ('azure_boards', 'github', 'jira')),
  target           jsonb NOT NULL,
  mode             text NOT NULL CHECK (mode IN ('in', 'in_and_comments')),
  credential_kind  text NOT NULL CHECK (credential_kind IN ('secret_ref', 'workload')),
  credential_ref   text NOT NULL DEFAULT '' CHECK (octet_length(credential_ref) <= 512),
  cursor           text NOT NULL DEFAULT '' CHECK (octet_length(cursor) <= 4096),
  run_requested_at timestamptz,
  last_finished_at timestamptz,
  created_at       timestamptz NOT NULL,
  created_by       text NOT NULL,
  disconnected_at  timestamptz,
  FOREIGN KEY (org_id, team_id)    REFERENCES teams (org_id, id),
  FOREIGN KEY (org_id, project_id) REFERENCES projects (org_id, id),
  FOREIGN KEY (org_id, created_by) REFERENCES users (org_id, id),
  CONSTRAINT work_tracker_ref_iff_secret CHECK ((credential_kind = 'secret_ref') = (credential_ref <> '')
                                                OR disconnected_at IS NOT NULL)
);

CREATE TABLE work_external_links (
  org_id              text NOT NULL REFERENCES orgs (id),
  connection_id       text NOT NULL REFERENCES work_tracker_connections (id),
  external_id         text NOT NULL CHECK (char_length(external_id) BETWEEN 1 AND 200),
  issue_id            text NOT NULL,
  linked_at           timestamptz NOT NULL,
  upstream_field      text CHECK (upstream_field IN ('closed', 'reopened', 'assignee', 'title', 'description', 'deleted')),
  upstream_changed_at timestamptz,
  PRIMARY KEY (connection_id, external_id),
  FOREIGN KEY (org_id, issue_id) REFERENCES issues (org_id, id),
  CONSTRAINT work_links_upstream_whole CHECK ((upstream_field IS NULL) = (upstream_changed_at IS NULL))
);

CREATE TABLE work_sync_runs (
  org_id        text NOT NULL REFERENCES orgs (id),
  connection_id text NOT NULL REFERENCES work_tracker_connections (id),
  n             integer NOT NULL CHECK (n >= 1),
  started_at    timestamptz NOT NULL,
  finished_at   timestamptz,
  state         text NOT NULL CHECK (state IN ('running', 'ok', 'failed')),
  pages         integer NOT NULL DEFAULT 0,
  created       integer NOT NULL DEFAULT 0,
  existing      integer NOT NULL DEFAULT 0,
  upstream      jsonb NOT NULL DEFAULT '[]'::jsonb,   -- [{externalId, field, at}], ids and words we chose
  failure       text NOT NULL DEFAULT '' CHECK (octet_length(failure) <= 300), -- a code and a field, never upstream text
  PRIMARY KEY (connection_id, n)
);
-- RLS org_isolation on all three, as task 149's loop does; grants:
GRANT SELECT, INSERT ON work_tracker_connections, work_external_links, work_sync_runs TO hub_app;
GRANT UPDATE (cursor, run_requested_at, last_finished_at, disconnected_at, credential_kind, credential_ref)
  ON work_tracker_connections TO hub_app;
GRANT UPDATE (upstream_field, upstream_changed_at) ON work_external_links TO hub_app;
GRANT UPDATE (finished_at, state, pages, created, existing, upstream, failure) ON work_sync_runs TO hub_app;

-- ── the sync principal's guard (decision 2; SW.3, task 145's to own) ───────────────
CREATE FUNCTION guard_sync_principal() RETURNS trigger
LANGUAGE plpgsql SET search_path = hub, pg_temp AS $$
DECLARE v_conn text := coalesce(current_setting('hub.work_sync_connection', true), '');
BEGIN
  IF v_conn = '' THEN
    RETURN NEW;
  END IF;
  IF TG_TABLE_NAME = 'issues' AND TG_OP = 'UPDATE'
     AND (NEW.state IS DISTINCT FROM OLD.state OR NEW.assignee_id IS DISTINCT FROM OLD.assignee_id) THEN
    RAISE EXCEPTION 'hub: work: principal_cannot_move_state: % may not change issue %', v_conn, NEW.id
      USING ERRCODE = 'insufficient_privilege';
  END IF;
  IF TG_TABLE_NAME = 'issues' AND TG_OP = 'INSERT' AND NEW.state <> 'triage' THEN
    RAISE EXCEPTION 'hub: work: principal_cannot_move_state: % creates only in triage', v_conn
      USING ERRCODE = 'insufficient_privilege';
  END IF;
  IF TG_TABLE_NAME = 'issue_revisions' AND NEW.revision > 1 THEN
    RAISE EXCEPTION 'hub: work: principal_cannot_move_state: % writes revision 1 only', v_conn
      USING ERRCODE = 'insufficient_privilege';
  END IF;
  IF TG_TABLE_NAME = 'issue_labels' THEN
    RAISE EXCEPTION 'hub: work: principal_cannot_move_state: % sets no label', v_conn
      USING ERRCODE = 'insufficient_privilege';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER issues_sync_guard BEFORE INSERT OR UPDATE ON issues
  FOR EACH ROW EXECUTE FUNCTION guard_sync_principal();
CREATE TRIGGER issue_revisions_sync_guard BEFORE INSERT ON issue_revisions
  FOR EACH ROW EXECUTE FUNCTION guard_sync_principal();
CREATE TRIGGER issue_labels_sync_guard BEFORE INSERT ON issue_labels
  FOR EACH ROW EXECUTE FUNCTION guard_sync_principal();
```

`assignee_id` is task 06's column name for the assignee as task 08's `PatchIssue` writes it
(**UNVERIFIED** until task 06's table is read at merge; the guard follows the real name). The
guard is keyed on `hub.work_sync_connection`, not on `hub.work_actor_kind`, so it binds exactly the
sync engine and leaves task 07's rows for other integrations as they are. **Labels are not
attached on import** as a result: task 15's `importOne` attaches matched labels, and a sync item's
labels are reported as `unmatchedLabels`-style names on the run log instead (task 151-153's adapters
still map them, for the day the owner allows it). A person adds labels in Triage. This differs
from a person's CSV import, which is the person's act and keeps its labels.

#### `hubapi` additions (requirement on task 143, and task 15's list; complete)

```go
// ── Task 150: tracker sync (SW.21; SW.6 widened) ────────────────────────────

const (
	PrefixTrackerConnection = "tc-" // decision 8; task 02's workids.go gains it
	ImportSourceAzureBoards = "azure_boards"
	ImportSourceGitHub      = "github"
	ImportSourceJira        = "jira"
)

// ImportSources, task 15's list, widened in this task:
//	var ImportSources = []string{ImportSourceLinear, ImportSourceCSV,
//		ImportSourceAzureBoards, ImportSourceGitHub, ImportSourceJira}
// POST /v1/projects/{projectId}/imports still refuses the three tracker sources
// (field: source): they come in only through a connection, so their issues are
// always the sync principal's.
var TrackerSources = []string{ImportSourceAzureBoards, ImportSourceGitHub, ImportSourceJira}

type TrackerMode string

const (
	TrackerIn            TrackerMode = "in"
	TrackerInAndComments TrackerMode = "in_and_comments" // task 153
)

// TrackerTarget says which items; each adapter names the fields it needs and
// refuses the rest (Adapter.CheckTarget).
type TrackerTarget struct {
	Organization string `json:"organization,omitempty"` // Azure DevOps org; GitHub owner
	Project      string `json:"project,omitempty"`      // Azure DevOps project; Jira project key
	Repository   string `json:"repository,omitempty"`   // GitHub repo
	Site         string `json:"site,omitempty"`         // Jira https host
	Filter       string `json:"filter,omitempty"`       // WIQL area path, GitHub label or JQL; adapter-checked
}

type TrackerCredential struct {
	Kind string `json:"kind"`          // "secret_ref" or "workload"
	Ref  string `json:"ref,omitempty"` // a Key Vault secret reference (task 145); never a secret
}

type TrackerConnection struct {
	ID             string            `json:"id"`
	TeamID         string            `json:"teamId"`
	ProjectID      string            `json:"projectId"`
	Source         string            `json:"source"`
	Target         TrackerTarget     `json:"target"`
	Mode           TrackerMode       `json:"mode"`
	Credential     TrackerCredential `json:"credential"`
	CreatedAt      string            `json:"createdAt"`
	CreatedBy      User              `json:"createdBy"`
	DisconnectedAt string            `json:"disconnectedAt,omitempty"`
	LastRun        *SyncRun          `json:"lastRun"`
}

type CreateTrackerRequest struct {
	Source     string            `json:"source"`
	Target     TrackerTarget     `json:"target"`
	Mode       TrackerMode       `json:"mode"`
	Credential TrackerCredential `json:"credential"`
}
```
