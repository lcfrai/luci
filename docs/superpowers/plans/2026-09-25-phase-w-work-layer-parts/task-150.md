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
     AND (NEW.state IS DISTINCT FROM OLD.state OR NEW.assignee_user_id IS DISTINCT FROM OLD.assignee_user_id) THEN
    RAISE EXCEPTION 'hub: work: principal_cannot_move_state: % may not change issue %', v_conn, NEW.id
      USING ERRCODE = 'insufficient_privilege';
  END IF;
  IF TG_TABLE_NAME = 'issues' AND TG_OP = 'INSERT'
     AND (NEW.state <> 'triage' OR NEW.assignee_user_id IS NOT NULL) THEN
    RAISE EXCEPTION 'hub: work: principal_cannot_move_state: % creates only in triage, unassigned', v_conn
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

`assignee_user_id` is task 06's column (`task-06.md:164`). An `INSERT` with an assignee is refused
too, by the same rule as a state: task 15's `importOne` never sets one. The guard is keyed on `hub.work_sync_connection`, not on `hub.work_actor_kind`, so it binds exactly the
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

// UpstreamChange names which thing changed on an item already imported, never
// its new value (decision 5).
type UpstreamChange struct {
	ExternalID string `json:"externalId"`
	Field      string `json:"field"` // closed, reopened, assignee, title, description, deleted
	At         string `json:"at"`    // the tracker's own time, RFC 3339
}

var UpstreamFields = []string{"closed", "reopened", "assignee", "title", "description", "deleted"}

// SyncRun is one run of one connection; N counts from 1 per connection.
type SyncRun struct {
	ConnectionID string           `json:"connectionId"`
	N            int              `json:"n"`
	State        string           `json:"state"` // running, ok, failed
	StartedAt    string           `json:"startedAt"`
	FinishedAt   string           `json:"finishedAt,omitempty"`
	Pages        int              `json:"pages"`
	Created      int              `json:"created"`
	Existing     int              `json:"existing"`
	Upstream     []UpstreamChange `json:"upstream"`
	Failure      string           `json:"failure,omitempty"` // e.g. "page 3: items[7].title"
}

// ExternalLink is how an issue reads where it came from, on the issue view.
type ExternalLink struct {
	ConnectionID      string `json:"connectionId"`
	Source            string `json:"source"`
	ExternalID        string `json:"externalId"`
	IssueID           string `json:"issueId"`
	LinkedAt          string `json:"linkedAt"`
	UpstreamField     string `json:"upstreamField,omitempty"`
	UpstreamChangedAt string `json:"upstreamChangedAt,omitempty"`
	Disconnected      bool   `json:"disconnected"`
}

const (
	EventTrackerConnected    = "tracker.connected"
	EventTrackerDisconnected = "tracker.disconnected"
	EventTrackerPage         = "tracker.page"
)

type TrackerConnectedData struct {
	Source string `json:"source"`
	Mode   string `json:"mode"`
}
type TrackerDisconnectedData struct{}

// TrackerPageData is decision 3's event: one per committed page, the
// connection and its creator named, counts only.
type TrackerPageData struct {
	ConnectionID string `json:"connectionId"`
	Run          int64  `json:"run"`
	Created      int64  `json:"created"`
	Existing     int64  `json:"existing"`
	Upstream     int64  `json:"upstream"`
	By           string `json:"by"` // the connection's creator, a u- id
}
// Appended to WorkEventKinds, all ScopeProject, VisibleToReaders:
//	{EventTrackerConnected, TrackerConnectedData{}, …}, {EventTrackerDisconnected, …}, {EventTrackerPage, …}
```

`tracker.` joins task 13's prefix list. The feed never carries a target (it may name a client's
Azure DevOps organisation), a credential reference, or any upstream text.

#### `worksync/engine.go` (complete)

```go
// Package worksync brings items from outside trackers into a team's Triage,
// one way. It never writes back (task 153 adds comments only) and it never
// moves an issue: the Sink's transaction is the sync principal (decision 2).
package worksync

// Cursor is an adapter's own position, opaque to the engine; "" is the start.
type Cursor string

// Pulled is one page from a tracker. Items are new or known; the planner
// (links.go) decides which by the connection's links.
type Pulled struct {
	Items    []hubapi.ImportItem     // SW.6's shape, mapped by the adapter
	Upstream []hubapi.UpstreamChange // changes the adapter saw on items it reported before
	Comments []ExternalComment       // task 153's; ignored in mode "in"
	Next     Cursor
	More     bool // another page follows in this run
}

// ExternalComment is task 153's; declared here so the page type is whole.
type ExternalComment struct {
	ExternalID string // the item's
	CommentID  string
	AuthorName string
	Text       string
	At         string
}

// Adapter talks to one tracker. Pull must be safe to repeat with the same
// cursor: the engine re-pulls a page whose commit failed.
type Adapter interface {
	Source() string
	CheckTarget(t hubapi.TrackerTarget) *hubapi.FieldError
	Pull(ctx context.Context, conn hubapi.TrackerConnection, cur Cursor) (Pulled, error)
}

// Sink is the server side (decision 1). ApplyPage runs the page, the links,
// the run's counts and the cursor in one transaction as the sync principal.
type Sink interface {
	StartRun(ctx context.Context, connID string) (hubapi.TrackerConnection, Cursor, hubapi.SyncRun, error)
	ApplyPage(ctx context.Context, conn hubapi.TrackerConnection, run *hubapi.SyncRun, p Pulled) error
	FinishRun(ctx context.Context, run hubapi.SyncRun, failure string) (hubapi.SyncRun, error)
}

// ErrPageRefused is ApplyPage's answer for a page that fails SW.6's check.
type ErrPageRefused struct{ Field string }

func (e *ErrPageRefused) Error() string { return "worksync: page refused at " + e.Field }

type Engine struct {
	Adapters map[string]Adapter
	Sink     Sink
	MaxPages int // per run; 0 means 20
}

// Run pulls and applies pages until the adapter says there are no more, the
// page cap is reached, or a page fails. It never holds a transaction while a
// tracker is being called.
func (e *Engine) Run(ctx context.Context, connID string) (hubapi.SyncRun, error) {
	conn, cur, run, err := e.Sink.StartRun(ctx, connID)
	if err != nil {
		return hubapi.SyncRun{}, err
	}
	a, ok := e.Adapters[conn.Source]
	if !ok {
		return e.Sink.FinishRun(ctx, run, "no adapter for "+conn.Source)
	}
	limit := e.MaxPages
	if limit == 0 {
		limit = 20
	}
	for page := 1; page <= limit; page++ {
		p, err := a.Pull(ctx, conn, cur)
		if err != nil {
			return e.Sink.FinishRun(ctx, run, fmt.Sprintf("page %d: pull failed", page)) // no upstream text
		}
		if conn.Mode != hubapi.TrackerInAndComments {
			p.Comments = nil
		}
		if err := e.Sink.ApplyPage(ctx, conn, &run, p); err != nil {
			var refused *ErrPageRefused
			if errors.As(err, &refused) {
				return e.Sink.FinishRun(ctx, run, fmt.Sprintf("page %d: %s", page, refused.Field))
			}
			return e.Sink.FinishRun(ctx, run, fmt.Sprintf("page %d: not committed", page))
		}
		cur = p.Next // only after the commit, which moved the stored cursor too
		if !p.More {
			break
		}
	}
	return e.Sink.FinishRun(ctx, run, "")
}
```

A pull error's text is **not** stored: a tracker's error body can echo a query or an item, and
`work_sync_runs.failure` is capped at 300 bytes of our own words. The adapter logs the error with
the connection id (Hub task 24's logger, no body), so an operator can find it. The 20-page cap
keeps one connection from holding the janitor for a whole sweep; a run that hits it leaves the
cursor where the last page put it and the next run continues.

#### `worksync/links.go` (complete)

```go
package worksync

// PagePlan is what one page will write. Known items are never written again:
// an item seen before adds no revision, however its text changed upstream.
type PagePlan struct {
	New      []hubapi.ImportItem
	Known    map[string]string // externalId → issueId, for the run's existing count
	Upstream []hubapi.UpstreamChange
}

// Plan splits a page by the connection's links. known maps an externalId the
// connection has linked to its issue. An upstream change on an id with no link
// is dropped (the adapter saw it before our first import, or on an item we
// refused); a change whose field is not in UpstreamFields is dropped too.
// Two changes to one item in one page are kept in order, each shown.
func Plan(p Pulled, known map[string]string) PagePlan {
	plan := PagePlan{Known: map[string]string{}}
	seen := map[string]bool{}
	for _, it := range p.Items {
		if id, ok := known[it.ExternalID]; ok {
			plan.Known[it.ExternalID] = id
			continue
		}
		if seen[it.ExternalID] {
			continue // a tracker that repeats an item in one page: once
		}
		seen[it.ExternalID] = true
		plan.New = append(plan.New, it)
	}
	for _, u := range p.Upstream {
		if _, ok := known[u.ExternalID]; ok && slices.Contains(hubapi.UpstreamFields, u.Field) {
			plan.Upstream = append(plan.Upstream, u)
		}
	}
	return plan
}
```

`Plan` is pure and gets the links the Sink read **inside** the page's transaction, under the org's
write lock, so two runs of one connection can never both see an item as new (the advisory lock
already makes that impossible across replicas; the write lock covers a run and a person's import
of the same source at once, and `issues_external_once` is the third layer). A duplicate inside
one page is dropped here, before `ValidateImportBatch`, which would refuse the whole page for it
(`ErrImportDuplicate`, `task-15.md:193`): a tracker's paging glitch should not stop a connection.

#### The Sink (`server/work_trackers.go`, contract complete)

```go
package server

// ApplyPage, in one s.inOrg transaction for conn.OrgID, in this order:
//  1. set_config('hub.work_actor_kind', 'integration', true) and
//     set_config('hub.work_sync_connection', conn.ID, true) (decision 2);
//  2. LockConnection(conn.ID): disconnected since StartRun → ErrPageRefused{"connection"};
//  3. known := LinksOf(conn.ID, ids of p.Items and p.Upstream); plan := worksync.Plan(p, known);
//  4. ValidateImportBatch({source: conn.Source, items: plan.New}) when plan.New is not
//     empty → a failure is ErrPageRefused{field} and the transaction rolls back;
//  5. IssuesByExternal (task 15) on conn.ProjectID for plan.New: a hit made by another path
//     (a person's CSV of the same ids is refused by source; this is the third layer) is
//     linked, not created;
//  6. for each remaining item: importOne(ctx, tx, conn.TeamID, conn.ProjectID, conn.Source,
//     item, nil /* no labels: decision 2's guard */, conn.CreatedBy.ID); InsertLink;
//  7. for each plan.Upstream: SetLinkUpstream(conn.ID, externalId, field, at);
//  8. run.Pages++, run.Created += new, run.Existing += len(plan.Known), run.Upstream += plan.Upstream;
//     UpdateRunCounts; SetConnectionCursor(conn.ID, p.Next);
//  9. emitWork(tx, tracker.page, conn.TeamID, conn.ProjectID, conn.ID, TrackerPageData{…}).
func (s *Server) ApplyPage(ctx context.Context, conn hubapi.TrackerConnection, run *hubapi.SyncRun, p worksync.Pulled) error

// StartRun: in one transaction, the connection (not disconnected), the next n,
// INSERT work_sync_runs (state running), clear run_requested_at.
func (s *Server) StartRun(ctx context.Context, connID string) (hubapi.TrackerConnection, worksync.Cursor, hubapi.SyncRun, error)

// FinishRun: state ok when failure is "", else failed; finished_at; the
// connection's last_finished_at. A failed run's pages that committed stay.
func (s *Server) FinishRun(ctx context.Context, run hubapi.SyncRun, failure string) (hubapi.SyncRun, error)
```

`importOne`'s new last argument is the issue's `created_by` and the revision's `written_by`; task
15's route passes `tx.P.UserID`, exactly as before. `importOne` also reads `tx.P.OrgID`; a janitor
transaction from `s.inOrg` must carry the org there with an empty user and a nil actor, which is
how task 121's passes already need it (`task-121.md:241-246`); that Hub task 24's janitor `Tx` has
this shape is **UNVERIFIED** until it lands, and if it does not, `importOne` takes the org as an
argument too.

#### The routes (`server/work_trackers.go`)

- `POST /v1/projects/{projectId}/trackers` (team admin of the project's team): `Decode`;
  `ProjectRole` + `Require(admin)`; `source` in `TrackerSources` (`field: source`); the adapter's
  `CheckTarget` (`field: target.<name>`); `mode` in the two (`field: mode`); the credential is
  `workload`, or `secret_ref` with a `ref` of task 145's Key Vault reference form (`field:
  credential`) and **never** anything that looks like a secret value (a `ref` that does not parse
  as a reference is 400, so a pasted token is refused, not stored); insert with `ids.Wide("tc-")`;
  emit `tracker.connected`; 201 `TrackerConnection`.
- `GET /v1/teams/{teamId}/trackers` (viewer) → `Page[TrackerConnection]`, disconnected ones
  included, newest first.
- `DELETE /v1/trackers/{connectionId}` (admin): sets `disconnected_at`, blanks `credential_ref`,
  clears `run_requested_at`; links stay (decision 4); emit `tracker.disconnected`; 200. A second
  DELETE is 200 with nothing written.
- `GET /v1/trackers/{connectionId}/runs` (viewer) → `Page[SyncRun]`, newest first.
- `POST /v1/trackers/{connectionId}/run` (admin): disconnected → 409 `tracker_disconnected`;
  sets `run_requested_at`; 202 `{requestedAt}` (decision 6). No event: a request to run is not a
  change to the record anyone reads.

`GET /v1/issues/{issueId}` (task 08) gains `links: [ExternalLink]` on the head, read by the issue
id, so an imported issue says where it came from and what changed upstream; an issue with none
reads `[]`. This is an additive field on task 03's `IssueHead` (shown as a diff in the part's
commit).

**The janitor step** (⚑ `janitor.go`, after task 149's): after the per-org loop, list the
connections that are not disconnected and are due (`run_requested_at` set, or `last_finished_at`
older than the interval, or never run), oldest first, at most 10 per sweep; `Engine.Run` each.
The advisory lock is held for the step, so a second replica skips it.

#### Wire shapes

```jsonc
// POST /v1/projects/pj-…/trackers                     team admin; 201
{ "source": "azure_boards", "mode": "in",
  "target": { "organization": "acme-eng", "project": "Payments", "filter": "Payments\\Uploads" },
  "credential": { "kind": "workload" } }
// → 201
{ "id": "tc-8d41…", "teamId": "tm-…", "projectId": "pj-…", "source": "azure_boards", "mode": "in",
  "target": { … }, "credential": { "kind": "workload" }, "createdAt": "…", "createdBy": { "id": "u-ada…", … },
  "lastRun": null }

// GET /v1/trackers/tc-8d41…/runs                       viewer; 200
{ "items": [ { "connectionId": "tc-8d41…", "n": 3, "state": "ok", "pages": 2, "created": 0, "existing": 57,
               "upstream": [ { "externalId": "1432", "field": "closed", "at": "2026-09-26T01:02:00Z" } ],
               "startedAt": "…", "finishedAt": "…" } ], "next": null }

// GET /v1/issues/is-…  (an issue it imported)
{ "state": "triage", "threadState": { "kind": "not_tied", "reason": "imported" }, "source": "intake",
  "external": { "source": "azure_boards", "externalId": "1432", "url": "https://dev.azure.com/…", … },
  "links": [ { "connectionId": "tc-8d41…", "source": "azure_boards", "externalId": "1432", "issueId": "is-…",
               "upstreamField": "closed", "upstreamChangedAt": "2026-09-26T01:02:00Z", "disconnected": false } ], … }

// the feed for one page: task 13's issue.created per new item (actor null), and
{ "kind": "tracker.page", "actor": null, "subject": "tc-8d41…",
  "data": { "connectionId": "tc-8d41…", "run": 3, "created": 0, "existing": 57, "upstream": 1, "by": "u-ada…" } }

400 bad_request          { "field": "credential" }  // a pasted token in ref
400 bad_request          { "field": "source" }      // "trello"; or POST …/imports with "azure_boards"
409 tracker_disconnected                            // run now after DELETE
```
 With `matched` nil, `importOne` attaches no
label and returns every name as missing; the page records them nowhere else (the names are
upstream words). `run.Upstream` is capped at 500 entries per run on the row; later ones still
update their links and are counted in the page event.

#### Reconciliation with the committed wire (task 143) and the adapters (tasks 151-153)

Task 143's part fixed the W5 wire after this part's first half was written, and tasks 151-153 were
written against the scope and name what they need from this engine. **Where this section and the
blocks above disagree, this section wins**; the blocks above stay as the record of what was
proposed. Each point cites what it follows.

1. **The wire types are task 143's** (`task-143.md:178-280`): `TrackerConnection` (raw `target`,
   `principalId`, `state`, `lastRunAt`), the three target variants, `UpstreamChange` with `to`,
   `ExternalLink` with an `xl-` id, `SyncError`, `SyncRun` (one pass: `at`, four counts,
   `errors`) and `ConnectTrackerRequest {target, mode}`. The `hubapi` block above (`:197-320`) is
   withdrawn except `TrackerSources` and the event data below. The prefixes are task 143's `tc`
   and `xl` (its decision 6), which closes decision 8's missing prefix.
2. **The source rides in the target.** The create route reads the target's own `source` key, then
   calls `hubapi.ParseTrackerTarget(source, raw)` (`task-143.md:585-600`), which refuses an unknown
   key. A source with no adapter registered in the engine is 400 `field: target` as well, so no
   Boards connection can be made before task 151 lands. `Adapter.CheckTarget` is withdrawn: an
   adapter is `Source()` and `Pull()`, as the scope and tasks 151-153 have it (`task-151.md:419-442`).
3. **No credential is stored or sent.** Task 143 has no credential on the wire, and task 151 reaches
   a tracker as the Hub's own identity through a `TokenFunc` (`task-151.md:51-56`, `:105-108`),
   wired where the engine is built. The `credential_*` columns, the route's credential check and
   the `credential` wire key are withdrawn. A pasted token has nowhere to go: any key outside the
   variant is refused by `ParseTrackerTarget`.
4. **`principalId` is the connection's own id.** Decision 3 stands (no Entra token; the Hub's
   process acts; `actor: null`), so `principalId == id` on every connection this task makes. Task
   143's *"That Entra issues the integration … principals"* (`task-143.md:1134`) is not needed here.
5. **`UpstreamChange` inside `worksync` is the adapters' type**, with `At time.Time`
   (`task-151.md:494`, `task-152.md:331`, `task-153.md:406`). The fields they emit are `state` and
   `assignees` (`task-152.md:309-310`), not decision 5's six, so `UpstreamFields` changes (below).
   `To` carries the tracker's own word for display only (task 143's decision 7). The Sink converts
   it to `hubapi.UpstreamChange` with `at` in RFC 3339 UTC.
6. **Pull until the cursor stops** (`task-151.md`, its gap 3). The adapters set no `More`; the
   engine pulls again while the cursor it gets back differs from the one it sent, up to
   `MaxPages`. `More` is withdrawn.
7. **One refused item does not stop the page** (`task-151.md`, its gap 2; task 143's `SyncError`).
   Decision 7 is replaced: the Sink checks each new item alone with `ValidateImportBatch`; a
   refused item becomes `SyncError{externalId, code: "item_refused"}` on the run, its field name
   goes to the Hub's log (no value), and the rest of the page lands. The item is not silent (the
   run shows it) and it comes back the next time it changes upstream, because every adapter pulls
   by change time. A page that fails to **commit** still moves nothing and keeps the cursor.
8. **Links are keyed by task 143's `(projectId, source, externalId)`**, not by connection
   (decision 4 is narrowed): two connections of one project and source that both see an item share
   one link and one issue. A link also records the connection that made it, and `UNIQUE (issue_id)`
   holds task 169's *"at most one per issue"* (`task-169.md:616`).
9. **The events are the ones the desktop already names** (`task-169.md:569`, `task-172.md:174-182`):
   `tracker.connected`, `tracker.disconnected`, `sync.run` (one per run, in `FinishRun`'s
   transaction) and `external_link.created` (one per new link). `tracker.page` is withdrawn, and
   `tracker.mode_set` is task 153's.
10. **The issue head gains nothing.** The `links` addition to task 08's `GET /v1/issues/{issueId}`
    is withdrawn; the desktop reads links through `GET /v1/trackers/{connectionId}/links` (task 169's
    `ListTrackerLinks`, added below), so no W1 type changes.
11. **`PullError` and its four codes move here.** Task 151's `fetch.go` declares them
    (`task-151.md:110-132`), but this task lands first and the engine must read a pull error's
    code and retry time. They are declared in `engine.go` exactly as task 151 wrote them, and task
    151's block drops those lines (gap 3).
12. **Files this section adds**: `worksync/fake.go` (new; the fake adapter, outside a `_test.go` file
    so the server's integration tests can drive it), ⚑ `server/config.go` (the engine's adapters
    and `PANACEA_APP_HUB_SYNC_INTERVAL`) and ⚑ `hubapi/work_intake_test.go` (two rows).

#### `worksync/engine.go`, reconciled (complete; replaces the block at `:325-420`)

```go
package worksync

type Cursor string // unchanged: an adapter's own position, "" is the start

// UpstreamChange is what an adapter saw change on an item it reported before
// (points 5 and 8). To is the tracker's word, shown and never mapped to ours.
type UpstreamChange struct {
	ExternalID string
	Field      string // one of UpstreamFields
	At         time.Time
	To         string // the Sink keeps at most 60 code points
}

// UpstreamFields replaces decision 5's list: what tasks 151-153 emit, and
// three kept for the day one does.
var UpstreamFields = []string{"state", "assignees", "title", "description", "deleted"}

// Pulled is one page from a tracker; the engine sees no More (point 6).
type Pulled struct {
	Items    []hubapi.ImportItem
	Upstream []UpstreamChange
	Comments []hubapi.ExternalComment // task 153's type (task-153.md:115); nil in mode "in"
	Next     Cursor
}

// Adapter talks to one tracker. Pull must be safe to repeat with one cursor:
// the engine re-pulls a page whose commit failed.
type Adapter interface {
	Source() string
	Pull(ctx context.Context, conn hubapi.TrackerConnection, cur Cursor) (Pulled, error)
}

// RunRef names a run inside the Hub; the wire's SyncRun has no number.
type RunRef struct {
	ConnectionID string
	N            int
}

// Sink is the server side (decision 1). ApplyPage writes one page, its links,
// the run's counts and the cursor in one transaction as the sync principal.
type Sink interface {
	StartRun(ctx context.Context, connID string) (hubapi.TrackerConnection, Cursor, RunRef, error)
	ApplyPage(ctx context.Context, conn hubapi.TrackerConnection, run RunRef, p Pulled) error
	FinishRun(ctx context.Context, run RunRef, failure *hubapi.SyncError) (hubapi.SyncRun, error)
}

// PullError is task 151's (task-151.md:110-126), declared here (point 11).
type PullError struct {
	Code    string
	RetryAt time.Time
	Err     error
}

func (e *PullError) Error() string {
	if e.RetryAt.IsZero() {
		return fmt.Sprintf("worksync: %s: %v", e.Code, e.Err)
	}
	return fmt.Sprintf("worksync: %s until %s: %v", e.Code, e.RetryAt.UTC().Format(time.RFC3339), e.Err)
}
func (e *PullError) Unwrap() error { return e.Err }

// SyncError codes: worksync constants, never HTTP codes (task 143's decision 7).
const (
	CodeTrackerThrottled   = "tracker_throttled"   // 429, or a rate limit at zero (task 152)
	CodeTrackerUnreachable = "tracker_unreachable" // transport error or 5xx
	CodeTrackerRefused     = "tracker_refused"     // 401, 403 (not a rate limit), 404
	CodeTrackerShape       = "tracker_shape"       // a body we could not decode
	CodeItemRefused        = "item_refused"        // SW.6 refused one item (point 7)
	CodePageNotCommitted   = "page_not_committed"  // the page's transaction failed
	CodeNoAdapter          = "tracker_no_adapter"  // a connection whose source has no adapter
	CodeRunInterrupted     = "run_interrupted"     // a run left running by a stopped process
	CodeTrackerDisconnect  = "tracker_disconnected" // disconnected while the run was in flight
)
```

```go
// engine.go, continued

type Engine struct {
	Adapters map[string]Adapter // by Source(); config.go fills it (point 12)
	Sink     Sink
	MaxPages int // per run; 0 means 20
}

// Run pulls and applies pages until the cursor stops moving, the page cap is
// reached, or a pull or a commit fails. It never holds a transaction while a
// tracker is being called, and it stores no upstream text.
func (e *Engine) Run(ctx context.Context, connID string) (hubapi.SyncRun, error) {
	conn, cur, run, err := e.Sink.StartRun(ctx, connID)
	if err != nil {
		return hubapi.SyncRun{}, err
	}
	a, ok := e.Adapters[conn.Source]
	if !ok {
		return e.Sink.FinishRun(ctx, run, &hubapi.SyncError{Code: CodeNoAdapter})
	}
	limit := e.MaxPages
	if limit == 0 {
		limit = 20
	}
	for page := 1; page <= limit; page++ {
		p, err := a.Pull(ctx, conn, cur)
		if err != nil {
			return e.Sink.FinishRun(ctx, run, pullFailure(err))
		}
		if conn.Mode != hubapi.TrackerInAndComments {
			p.Comments = nil
		}
		if err := e.Sink.ApplyPage(ctx, conn, run, p); err != nil {
			code := CodePageNotCommitted
			if errors.Is(err, ErrDisconnected) {
				code = CodeTrackerDisconnect
			}
			return e.Sink.FinishRun(ctx, run, &hubapi.SyncError{Code: code})
		}
		if p.Next == cur {
			break // nothing past this cursor (point 6)
		}
		cur = p.Next // only after the commit, which moved the stored cursor too
	}
	return e.Sink.FinishRun(ctx, run, nil)
}

// ErrDisconnected is ApplyPage's answer when the connection was disconnected
// after StartRun; the page is not written.
var ErrDisconnected = errors.New("worksync: connection disconnected")

// pullFailure keeps a PullError's code and retry time and drops its text: a
// tracker's error body can echo a query or an item.
func pullFailure(err error) *hubapi.SyncError {
	var pe *PullError
	if !errors.As(err, &pe) || pe.Code == "" {
		return &hubapi.SyncError{Code: CodeTrackerUnreachable}
	}
	se := &hubapi.SyncError{Code: pe.Code}
	if !pe.RetryAt.IsZero() {
		at := pe.RetryAt.UTC().Format(time.RFC3339)
		se.RetryAt = &at
	}
	return se
}
```

`links.go`'s `Plan` (`:446-466`) keeps its body; only its types change: `Plan(p Pulled, known
map[string]string) PagePlan` reads `p.Upstream` as `[]worksync.UpstreamChange`, `PagePlan.Upstream`
is `[]UpstreamChange`, and the filter is the new `UpstreamFields`. `known` is now keyed by the
project's links for the connection's source (point 8), not by the connection's own.
