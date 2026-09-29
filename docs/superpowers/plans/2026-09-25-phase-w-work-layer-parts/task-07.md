### Task 07: Migration, part 2: the guards, and the authorisation-matrix rows for every W1 Work route

Stream **WH**, seam **SW.3** (defined here: who may write what), about 3 hours (an estimate, not a
measurement). Depends on task 06 (the tables) and Hub task **H16b** (the matrix harness,
`server/matrix_integration_test.go`, Hub plan `:298`), sync-merged into `feat/work-layer` first
(rule H1). Tasks 10, 22, 23, 99, 145, 150 and 159 consume SW.3.

Task 06 made the Work tables and said who may touch which column. This task says **which changes
are true**. A grant can say "`hub_app` may update `issues.state`"; it cannot say "only to `done`, and
only because evidence was observed, and never for a sync token". That is a trigger's job, and it is
the database half of the skeleton's rule 2: *no sync or integration token may write a fact state or
`done`, and an authorisation row plus a test pins it.*

**Seven decisions.**

1. **Two transaction-local settings carry the cause into the database.** `hub.work_cause` and
   `hub.work_actor_kind` are **new names**, set with `set_config(…, true)` by the Work store (task
   10) after the Hub's `hub.org_id` (hub-contract `:1925-1927`) and inside the same write
   transaction (§10.6, `:2031-2041`). They are not edits to the Hub's pipeline. Unset, both read
   `NULL`, and the guard treats `NULL` as *not a person* and *no cause*: default deny, the same way
   an unset `hub.org_id` matches no row.
2. **The fact-state guard checks the cause against the target, not the route.** A move **into**
   `started` needs `lane_bound`; `in_review` needs `lane_queued`; `landed` needs `land_trailer`;
   `done` needs `evidence_observed` or `evidence_recorded`, or `land_trailer` when the head serves
   nothing (task 10's `no_criteria`), or `hand` with a non-empty `state_reason` when the head serves
   a criterion. A move into any other state needs `hand`. The set of causes is task 10's, complete
   for W1; a cause this function does not name is refused.
3. **Only a person moves a fact state.** With `hub.work_actor_kind` anything but `person`, a move
   into a fact state is refused **whatever the cause**. W5's sync principal (tasks 145, 150) and apps
   (task 159) set `integration` or `app`; neither can write `started`, `in_review`, `landed` or
   `done`, even with a forged `evidence_observed`. This is SW.3's pin.
4. **Where the rule reads the head's `serves`, it reads the index, not the bytes.** `done` by
   `land_trailer` and the reason rule both ask "does the head revision serve anything?", answered
   from `issue_serves` (task 06), never by parsing `canonical`.
5. **A revision changes once, by erasure, inside `hub.erase_issue`.** The shape is `guard_revision`
   and `erase_brief` (hub-contract `:1817-1837`, `:1978-1993`): the trigger lets an `UPDATE` through
   only as `hub_owner`, only from content to `NULL` with `erased_at` set, and only with every other
   column equal. `erase_issue` checks the issue is in `current_setting('hub.org_id')` because, as
   owner, it bypasses row-level security. `erase_comment` does the same for a comment's `body` and
   `anchor`. Whether erasing an issue also erases its comments is task 14's call (it may call both).
6. **The matrix rows are written here for every W1 route, and switched on by the task that
   registers the route.** H16b's harness runs every row in `allRows` and fails any `/v1` pattern with
   no row (`TestEveryRouteHasAMatrixRow`, H16b `:86-89`). A row for a route not yet registered would
   fail the matrix, so this task writes all the rows, grouped by the task that owns each route
   (08-15), and an `init` appends a group to `allRows` only when its task id is in
   `workMatrixLive`. Each route task adds its id to that list **in the commit that registers its
   routes**; forgetting is caught by H16b's meta-test, and a row with no route by this task's.
7. **The rows live in a test file.** The scope names `server/work_matrix_rows.go`; a file in package
   `server_test` must end in `_test.go`, and H16b's harness is `//go:build integration`, so the file
   is `server/work_matrix_rows_integration_test.go`. This is a correction to the file map, listed in
   the return.

The one edit to a Hub file is additive and is the harness's own extension point: the `fixture`
struct's comment says fields are *"added by later tasks, never renamed"* (H16b `:45`). This task adds
the Work fields and one call in the fixture builder, and nothing else in that file.

**Files:**
- `control-plane/internal/hub/migrate/sql/NNNN_work_w1.sql` — the guards and the two erase
  functions, **appended** to task 06's file (modify). If task 06 has already been merged into
  `feat/hub`, they go in a new file `<next>_work_w1_guards.sql` instead, unchanged (rule H1)
- `control-plane/internal/hub/migrate/work_guards_integration_test.go` — the guard tests, on task
  06's `seedWork` (new)
- `control-plane/internal/hub/server/work_matrix_rows_integration_test.go` — the W1 route list,
  the rows per owning task, `workMatrixLive`, the `init`, and two meta-tests (new)
- `control-plane/internal/hub/server/matrix_integration_test.go` ⚑ — H16b's file: the Work fields on
  `fixture` and one `buildWorkFixture(t, f)` call in its builder (modify, additive only)

**Interfaces:**
- Produces (SW.3):
  - the settings `hub.work_cause` ∈ {`hand`, `lane_bound`, `lane_queued`, `land_trailer`,
    `evidence_observed`, `evidence_recorded`} and `hub.work_actor_kind` ∈ {`person`, `integration`,
    `app`}; `integration` and `app` are reserved names W5 sets, and W1 sets only `person`
  - the triggers `issues_guard`, `issues_state_guard`, `issues_initial_state_guard`, `issues_team_guard`,
    `project_keys_team_guard`, `project_issue_counters_guard`, `issue_revisions_guard`,
    `issue_comments_guard`, and the append-only triggers on `issue_serves`, `comment_mentions`,
    `issue_attachments`
  - `hub.erase_issue(p_issue text) RETURNS integer` and `hub.erase_comment(p_comment text) RETURNS
    integer`, `EXECUTE` to `hub_app` only (task 14 calls them)
  - the error text prefix `hub: work:` on every Work guard's `RAISE`, with `ERRCODE =
    'insufficient_privilege'` (`42501`), so the store maps a guard refusal to one problem code
    (task 10: 422 `state_not_allowed`) without matching message text beyond the prefix
  - `workW1Routes`, the complete W1 route list, and `workMatrixRows`, its rows by owning task
  - the `fixture` fields `IssueID`, `Issue2ID`, `IssueNumber`, `IssueRevision`, `CommentID`,
    `LabelID`, `WorkBlob`
- Consumes:
  - task 06: every table and constraint name; `seedWork`, `asApp`, `insertIssue`
  - hub-contract §10.2's `forbid_change()` (`:1799-1802`), which this file reuses by name
  - H16b: `row`, `column`, `fixture`, `allRows`, the fixture builder, `TestEveryRouteHasAMatrixRow`
  - H08: `(*Server).Patterns() []string` (Hub plan `:212`)
  - task 03: `hubapi.IssueState`, `Priority`, `RelationKind`, `NotTiedReason`, `IssueSource`,
    `SubscriptionCause` and their value lists, for `TestWorkCheckSetsMatchHubapi`
- Requires of later tasks:
  - **Task 10** sets both settings in every state write, and is the only Go that sets
    `hub.work_cause`. It may replace `guard_issue_state()` with `CREATE OR REPLACE` in the same
    file (its `issue_proofs` check); it may not loosen a rule this task's tests pin.
  - **Tasks 08-15** each add their id to `workMatrixLive` in the commit that registers their routes,
    and change a row's success status only in that same commit, if their route answers differently.
  - **Task 75** (W2 custom states) replaces `issues_state_known` and must keep every fact state
    behind this guard: a custom state is a *human band* state, never a fact.

#### The guards — appended to `NNNN_work_w1.sql`

Complete. Every function has a pinned `search_path` of `hub, pg_temp`, so a table of the same name
in another schema can never answer for it.

```sql
-- NNNN_work_w1.sql, continued (task 07): the guards.

-- ── append-only: hub-contract §10.2's forbid_change, reused by name ───────────
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['issue_serves', 'comment_mentions', 'issue_attachments'] LOOP
    EXECUTE format('CREATE TRIGGER %I BEFORE UPDATE OR DELETE ON hub.%I
                    FOR EACH ROW EXECUTE FUNCTION hub.forbid_change()', t || '_append_only', t);
    EXECUTE format('CREATE TRIGGER %I BEFORE TRUNCATE ON hub.%I
                    FOR EACH STATEMENT EXECUTE FUNCTION hub.forbid_change()', t || '_no_truncate', t);
  END LOOP;
END $$;

-- ── an issue revision changes once, by erasure, and only inside hub.erase_issue ──
CREATE FUNCTION guard_issue_revision() RETURNS trigger
LANGUAGE plpgsql SET search_path = hub, pg_temp AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    RAISE EXCEPTION 'hub: work: issue revisions are never deleted' USING ERRCODE = 'insufficient_privilege';
  END IF;
  IF current_user <> 'hub_owner'
     OR OLD.canonical IS NULL OR NEW.canonical IS NOT NULL OR NEW.erased_at IS NULL
     OR (NEW.org_id, NEW.issue_id, NEW.revision, NEW.project_id, NEW.digest, NEW.written_at,
         NEW.written_by, NEW.restored_from)
        IS DISTINCT FROM
        (OLD.org_id, OLD.issue_id, OLD.revision, OLD.project_id, OLD.digest, OLD.written_at,
         OLD.written_by, OLD.restored_from)
  THEN
    RAISE EXCEPTION 'hub: work: an issue revision is immutable' USING ERRCODE = 'insufficient_privilege';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER issue_revisions_guard BEFORE UPDATE OR DELETE ON issue_revisions
  FOR EACH ROW EXECUTE FUNCTION guard_issue_revision();
CREATE TRIGGER issue_revisions_no_truncate BEFORE TRUNCATE ON issue_revisions
  FOR EACH STATEMENT EXECUTE FUNCTION forbid_change();

-- ── an issue head: identity frozen; the head moves one step onto a revision that exists ──
CREATE FUNCTION guard_issue() RETURNS trigger
LANGUAGE plpgsql SET search_path = hub, pg_temp AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    RAISE EXCEPTION 'hub: work: issues are never deleted' USING ERRCODE = 'insufficient_privilege';
  END IF;
  IF (NEW.id, NEW.org_id, NEW.project_id, NEW.team_id, NEW.number, NEW.source,
      NEW.external_source, NEW.external_id, NEW.created_at, NEW.created_by)
     IS DISTINCT FROM
     (OLD.id, OLD.org_id, OLD.project_id, OLD.team_id, OLD.number, OLD.source,
      OLD.external_source, OLD.external_id, OLD.created_at, OLD.created_by)
     OR NEW.head_revision NOT IN (OLD.head_revision, OLD.head_revision + 1)
     OR (NEW.head_revision <> OLD.head_revision AND NOT EXISTS
           (SELECT 1 FROM issue_revisions r WHERE r.issue_id = NEW.id AND r.revision = NEW.head_revision))
     OR (NEW.external_url IS DISTINCT FROM OLD.external_url
         AND (current_user <> 'hub_owner' OR NEW.external_url <> ''))
     OR (NEW.erased_at IS DISTINCT FROM OLD.erased_at
         AND (current_user <> 'hub_owner' OR OLD.erased_at IS NOT NULL))
     OR (OLD.erased_at IS NOT NULL AND current_user <> 'hub_owner')
  THEN
    RAISE EXCEPTION 'hub: work: illegal change to issue %', OLD.id USING ERRCODE = 'insufficient_privilege';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER issues_guard BEFORE UPDATE OR DELETE ON issues
  FOR EACH ROW EXECUTE FUNCTION guard_issue();
CREATE TRIGGER issues_no_truncate BEFORE TRUNCATE ON issues
  FOR EACH STATEMENT EXECUTE FUNCTION forbid_change();

-- ── a denormalised team_id is the project's team (projects.team_id never changes: §10.5) ──
CREATE FUNCTION guard_work_team() RETURNS trigger
LANGUAGE plpgsql SET search_path = hub, pg_temp AS $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM projects p WHERE p.id = NEW.project_id AND p.team_id = NEW.team_id) THEN
    RAISE EXCEPTION 'hub: work: % is not project %''s team', NEW.team_id, NEW.project_id
      USING ERRCODE = 'insufficient_privilege';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER issues_team_guard BEFORE INSERT ON issues
  FOR EACH ROW EXECUTE FUNCTION guard_work_team();
CREATE TRIGGER project_keys_team_guard BEFORE INSERT ON project_keys
  FOR EACH ROW EXECUTE FUNCTION guard_work_team();

-- ── a counter moves one step at a time; it is never reset ─────────────────────
CREATE FUNCTION guard_issue_counter() RETURNS trigger
LANGUAGE plpgsql SET search_path = hub, pg_temp AS $$
BEGIN
  IF TG_OP = 'DELETE' OR NEW.project_id <> OLD.project_id OR NEW.org_id <> OLD.org_id
     OR NEW.last_number <> OLD.last_number + 1 THEN
    RAISE EXCEPTION 'hub: work: an issue counter only counts up by one' USING ERRCODE = 'insufficient_privilege';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER project_issue_counters_guard BEFORE UPDATE OR DELETE ON project_issue_counters
  FOR EACH ROW EXECUTE FUNCTION guard_issue_counter();

-- ── the fact-state guard (SW.3): a fact is written by a person, for its one cause ──
CREATE FUNCTION guard_issue_state() RETURNS trigger
LANGUAGE plpgsql SET search_path = hub, pg_temp AS $$
DECLARE
  v_cause  text := coalesce(current_setting('hub.work_cause', true), '');
  v_kind   text := coalesce(current_setting('hub.work_actor_kind', true), '');
  v_serves boolean := EXISTS (SELECT 1 FROM issue_serves s
                               WHERE s.issue_id = NEW.id AND s.revision = NEW.head_revision);
  v_fact   boolean := NEW.state IN ('started', 'in_review', 'landed', 'done');
BEGIN
  IF v_fact AND v_kind <> 'person' THEN
    RAISE EXCEPTION 'hub: work: only a person moves % into %', NEW.id, NEW.state
      USING ERRCODE = 'insufficient_privilege';
  END IF;
  IF NOT CASE NEW.state
       WHEN 'started'   THEN v_cause = 'lane_bound'
       WHEN 'in_review' THEN v_cause = 'lane_queued'
       WHEN 'landed'    THEN v_cause = 'land_trailer'
       WHEN 'done'      THEN v_cause IN ('evidence_observed', 'evidence_recorded')
                          OR (v_cause = 'land_trailer' AND NOT v_serves)
                          OR (v_cause = 'hand' AND (NOT v_serves OR btrim(NEW.state_reason) <> ''))
       ELSE v_cause = 'hand'
     END
  THEN
    RAISE EXCEPTION 'hub: work: % cannot move into % with cause "%"', NEW.id, NEW.state, v_cause
      USING ERRCODE = 'insufficient_privilege';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER issues_state_guard BEFORE UPDATE OF state ON issues
  FOR EACH ROW WHEN (NEW.state IS DISTINCT FROM OLD.state)
  EXECUTE FUNCTION guard_issue_state();
-- An INSERT is never a fact: task 03's ValidInitialState, and the same rule here.
CREATE FUNCTION guard_issue_initial_state() RETURNS trigger
LANGUAGE plpgsql SET search_path = hub, pg_temp AS $$
BEGIN
  IF NEW.state NOT IN ('triage', 'backlog', 'todo') THEN
    RAISE EXCEPTION 'hub: work: an issue is created in triage, backlog or todo, not %', NEW.state
      USING ERRCODE = 'insufficient_privilege';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER issues_initial_state_guard BEFORE INSERT ON issues
  FOR EACH ROW EXECUTE FUNCTION guard_issue_initial_state();

-- ── a comment: identity frozen; body and anchor change only to blank, by erasure ──
CREATE FUNCTION guard_issue_comment() RETURNS trigger
LANGUAGE plpgsql SET search_path = hub, pg_temp AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    RAISE EXCEPTION 'hub: work: comments are never deleted' USING ERRCODE = 'insufficient_privilege';
  END IF;
  IF (NEW.id, NEW.org_id, NEW.issue_id, NEW.parent_id, NEW.kind, NEW.author_class,
      NEW.created_at, NEW.created_by)
     IS DISTINCT FROM
     (OLD.id, OLD.org_id, OLD.issue_id, OLD.parent_id, OLD.kind, OLD.author_class,
      OLD.created_at, OLD.created_by)
     OR ((NEW.body, NEW.anchor, NEW.erased_at) IS DISTINCT FROM (OLD.body, OLD.anchor, OLD.erased_at)
         AND (current_user <> 'hub_owner' OR OLD.erased_at IS NOT NULL OR NEW.erased_at IS NULL))
  THEN
    RAISE EXCEPTION 'hub: work: illegal change to comment %', OLD.id USING ERRCODE = 'insufficient_privilege';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER issue_comments_guard BEFORE UPDATE OR DELETE ON issue_comments
  FOR EACH ROW EXECUTE FUNCTION guard_issue_comment();

-- ── erasure: the two owner functions (hub-contract §10.4's erase_brief shape) ───
-- The caller has already checked Hub.OrgAdmin in Go (task 14); these check the org,
-- because as owner they bypass row-level security. Keys, digests, numbers kept.
CREATE FUNCTION erase_issue(p_issue text) RETURNS integer
LANGUAGE plpgsql SECURITY DEFINER SET search_path = hub, pg_temp AS $$
DECLARE v_org text := current_setting('hub.org_id', true); v_n integer;
BEGIN
  IF v_org IS NULL OR NOT EXISTS (SELECT 1 FROM issues i WHERE i.id = p_issue AND i.org_id = v_org) THEN
    RAISE EXCEPTION 'hub: work: no issue % in this org', p_issue USING ERRCODE = 'no_data_found';
  END IF;
  UPDATE issue_revisions SET canonical = NULL, erased_at = now()
   WHERE issue_id = p_issue AND canonical IS NOT NULL;
  GET DIAGNOSTICS v_n = ROW_COUNT;
  UPDATE issues SET external_url = '', erased_at = coalesce(erased_at, now()), updated_at = now()
   WHERE id = p_issue AND erased_at IS NULL;
  RETURN v_n;
END $$;

CREATE FUNCTION erase_comment(p_comment text) RETURNS integer
LANGUAGE plpgsql SECURITY DEFINER SET search_path = hub, pg_temp AS $$
DECLARE v_org text := current_setting('hub.org_id', true); v_n integer;
BEGIN
  IF v_org IS NULL OR NOT EXISTS (SELECT 1 FROM issue_comments c
                                   WHERE c.id = p_comment AND c.org_id = v_org) THEN
    RAISE EXCEPTION 'hub: work: no comment % in this org', p_comment USING ERRCODE = 'no_data_found';
  END IF;
  UPDATE issue_comments SET body = '', anchor = NULL, erased_at = now()
   WHERE id = p_comment AND erased_at IS NULL;
  GET DIAGNOSTICS v_n = ROW_COUNT;
  RETURN v_n;
END $$;

REVOKE ALL ON FUNCTION erase_issue(text), erase_comment(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION erase_issue(text), erase_comment(text) TO hub_app;
```

Erasing an issue a second time returns 0 and changes nothing (the `erased_at IS NULL` filter and
the guard's "once" rule agree), so task 14's route is idempotent without a second check in Go.
`guard_issue` lets the owner blank `external_url` and set `erased_at` once, which is exactly what
`erase_issue` does; a `hub_app` statement that tries either is refused even though the owner could.

**Why the state guard is two triggers, not one.** `issues_state_guard` fires on `UPDATE OF state`
only when the value changes, so a `PATCH` of priority (task 08) never pays for the `issue_serves`
lookup and can never be refused by it. `issues_initial_state_guard` is the `INSERT` half: without
it, `INSERT … state = 'done'` would skip every rule above, because a row that is born `done` never
*moves* into it. Task 03's `ValidInitialState` is the Go half; the two layers are tested apart.

#### The matrix rows — `server/work_matrix_rows_integration_test.go`

The rows follow §5.3's columns (`V M A O N X`, hub-contract `:337-339`) and its nearest Hub rows:
*read a project and anything under it* (`:349`) for every read; *create a brief; write revisions*
(`:351`) for every issue and comment write; *rename a project* (`:350`) for team Work settings; and
*erase a brief* (`:359`) for erase. Two rows have no Hub twin and are decided here: the
subscription `PUT` is ✓ for `V` too (following an issue is not a write to it), and
`GET /v1/me/subscriptions` answers everyone, an empty list for a caller in no team, as `GET
/v1/me` does. Bodies are `map[string]any` in task 03's JSON names, so this file depends on the wire,
not on Go field names.

```go
//go:build integration

package server_test

// workW1Routes is the complete W1 Work route list, as ServeMux patterns. It is appended to
// hub-contract §7's table as rows 46 onward (the next free number at sync time) by the WH
// proposal; no Hub row is edited. /v1/refs (row 27) and /v1/changes (row 43) are extended
// by tasks 09 and 13, not added, so they are not here.
var workW1Routes = []string{
	"PUT /v1/projects/{projectId}/key", "POST /v1/projects/{projectId}/issues",
	"GET /v1/projects/{projectId}/issues", "GET /v1/projects/{projectId}/issues/by-number/{n}",
	"GET /v1/issues/{issueId}", "PATCH /v1/issues/{issueId}",
	"POST /v1/issues/{issueId}/revisions", "GET /v1/issues/{issueId}/revisions",
	"GET /v1/issues/{issueId}/revisions/{rev}", "POST /v1/issues/{issueId}/revisions/{rev}/restore",
	"POST /v1/issues/{issueId}/attachments", "GET /v1/issues/{issueId}/attachments",
	"POST /v1/issues/{issueId}/state",
	"POST /v1/issues/{issueId}/relations", "DELETE /v1/issues/{issueId}/relations/{kind}/{otherId}",
	"GET /v1/teams/{teamId}/labels", "POST /v1/teams/{teamId}/labels", "POST /v1/labels/{labelId}/archive",
	"GET /v1/teams/{teamId}/work-policy", "PUT /v1/teams/{teamId}/work-policy",
	"GET /v1/issues/{issueId}/comments", "POST /v1/issues/{issueId}/comments",
	"POST /v1/comments/{commentId}/reactions", "POST /v1/comments/{commentId}/resolve",
	"PUT /v1/issues/{issueId}/subscription", "GET /v1/me/subscriptions",
	"GET /v1/issues/search", "POST /v1/issues/{issueId}/erase", "POST /v1/comments/{commentId}/erase",
	"POST /v1/projects/{projectId}/imports",
}

// workRow is H16b's row plus the pattern it exercises, so a meta-test can compare rows
// with the route list and with (*Server).Patterns() without parsing a path.
type workRow struct {
	pattern string
	row     row
}

// workMatrixLive names the tasks whose routes are registered. Tasks 08-15 each append
// their id here in the commit that registers their routes. Task 07 lands it empty.
var workMatrixLive = []string{}

func init() {
	for _, id := range workMatrixLive {
		rows := make([]row, 0, len(workMatrixRows[id]))
		for _, w := range workMatrixRows[id] {
			rows = append(rows, w.row)
		}
		allRows = append(allRows, rows)
	}
}

func nf(code string) map[column]string {
	return map[column]string{colO: code, colN: code, colX: code}
}
func wread(p, path, code string) workRow {
	return workRow{p, row{name: p, method: "GET", path: path, want: [6]int{200, 200, 200, 404, 404, 404}, wantCode: nf(code)}}
}
func wwrite(p, method, path string, ok int, code string, body func(*fixture) any) workRow {
	return workRow{p, row{name: p, method: method, path: path, body: body, want: [6]int{403, ok, ok, 404, 404, 404}, wantCode: nf(code)}}
}
func wadmin(p, method, path string, ok int, code string, body func(*fixture) any) workRow {
	return workRow{p, row{name: p, method: method, path: path, body: body, want: [6]int{403, 403, ok, 404, 404, 404}, wantCode: nf(code)}}
}
func werase(p, path string) workRow {
	return workRow{p, row{name: p, method: "POST", path: path, body: empty, want: [6]int{403, 403, 403, 200, 403, 403}}}
}
func empty(*fixture) any { return map[string]any{} }
func h(text string) map[string]any { return map[string]any{"text": text, "author": "H"} }

// workMatrixRows is every W1 row, keyed by the task that registers the route.
var workMatrixRows = map[string][]workRow{
	"08": {
		wadmin("PUT /v1/projects/{projectId}/key", "PUT", "/v1/projects/{ProjectID}/key", 200, "project_not_found",
			func(*fixture) any { return map[string]any{"key": "PAY"} }),
		wwrite("POST /v1/projects/{projectId}/issues", "POST", "/v1/projects/{ProjectID}/issues", 201, "project_not_found",
			func(*fixture) any {
				return map[string]any{"title": h("Resume uploads"), "description": h(""), "serves": []any{},
					"state": "triage", "priority": "none", "labels": []any{}, "notTiedReason": "chore"}
			}),
		wread("GET /v1/projects/{projectId}/issues", "/v1/projects/{ProjectID}/issues", "project_not_found"),
		wread("GET /v1/projects/{projectId}/issues/by-number/{n}", "/v1/projects/{ProjectID}/issues/by-number/{IssueNumber}", "project_not_found"),
		wread("GET /v1/issues/{issueId}", "/v1/issues/{IssueID}", "issue_not_found"),
		wwrite("PATCH /v1/issues/{issueId}", "PATCH", "/v1/issues/{IssueID}", 200, "issue_not_found",
			func(*fixture) any { return map[string]any{"priority": "high"} }),
	},
	"09": {
		wwrite("POST /v1/issues/{issueId}/revisions", "POST", "/v1/issues/{IssueID}/revisions", 201, "issue_not_found",
			func(*fixture) any {
				return map[string]any{"baseRevision": 2, "content": map[string]any{"title": h("Resume uploads, v3"),
					"description": h(""), "serves": []any{}, "implements": []any{}}}
			}),
		wread("GET /v1/issues/{issueId}/revisions", "/v1/issues/{IssueID}/revisions", "issue_not_found"),
		wread("GET /v1/issues/{issueId}/revisions/{rev}", "/v1/issues/{IssueID}/revisions/1", "issue_not_found"),
		wwrite("POST /v1/issues/{issueId}/revisions/{rev}/restore", "POST", "/v1/issues/{IssueID}/revisions/1/restore", 201, "issue_not_found",
			func(*fixture) any { return map[string]any{"fromRevision": 1, "baseRevision": 2} }), // task 09 decision 3
		wwrite("POST /v1/issues/{issueId}/attachments", "POST", "/v1/issues/{IssueID}/attachments", 201, "issue_not_found",
			func(f *fixture) any { return map[string]any{"digest": f.WorkBlob} }),
		wread("GET /v1/issues/{issueId}/attachments", "/v1/issues/{IssueID}/attachments", "issue_not_found"),
	},
	"10": {
		wwrite("POST /v1/issues/{issueId}/state", "POST", "/v1/issues/{IssueID}/state", 200, "issue_not_found",
			func(*fixture) any { return map[string]any{"to": "todo", "cause": "hand", "reason": ""} }),
	},
	"11": {
		wwrite("POST /v1/issues/{issueId}/relations", "POST", "/v1/issues/{IssueID}/relations", 201, "issue_not_found",
			func(f *fixture) any { return map[string]any{"kind": "relates", "issueId": f.Issue2ID} }),
		wwrite("DELETE /v1/issues/{issueId}/relations/{kind}/{otherId}", "DELETE", "/v1/issues/{IssueID}/relations/blocks/{Issue2ID}", 200, "issue_not_found", nil), // 200 and the head: task 11
		wread("GET /v1/teams/{teamId}/labels", "/v1/teams/{TeamID}/labels", "team_not_found"),
		wadmin("POST /v1/teams/{teamId}/labels", "POST", "/v1/teams/{TeamID}/labels", 201, "team_not_found",
			func(*fixture) any { return map[string]any{"name": "Bug", "color": "red"} }),
		wadmin("POST /v1/labels/{labelId}/archive", "POST", "/v1/labels/{LabelID}/archive", 200, "label_not_found", empty),
		wread("GET /v1/teams/{teamId}/work-policy", "/v1/teams/{TeamID}/work-policy", "team_not_found"),
		wadmin("PUT /v1/teams/{teamId}/work-policy", "PUT", "/v1/teams/{TeamID}/work-policy", 200, "team_not_found",
			func(*fixture) any {
				return map[string]any{"defaultState": "backlog", "priorityToLeaveTriage": false, "stopAtLandedWithoutServes": false}
			}),
	},
	"12": {
		wread("GET /v1/issues/{issueId}/comments", "/v1/issues/{IssueID}/comments", "issue_not_found"),
		wwrite("POST /v1/issues/{issueId}/comments", "POST", "/v1/issues/{IssueID}/comments", 201, "issue_not_found",
			func(*fixture) any { return map[string]any{"body": h("Looks right"), "mentions": []any{}} }),
		wwrite("POST /v1/comments/{commentId}/reactions", "POST", "/v1/comments/{CommentID}/reactions", 200, "comment_not_found",
			func(*fixture) any { return map[string]any{"emoji": "👍", "on": true} }),
		wwrite("POST /v1/comments/{commentId}/resolve", "POST", "/v1/comments/{CommentID}/resolve", 200, "comment_not_found",
			func(*fixture) any { return map[string]any{"resolved": true} }),
	},
	"13": {
		{"PUT /v1/issues/{issueId}/subscription", row{name: "PUT /v1/issues/{issueId}/subscription", method: "PUT",
			path: "/v1/issues/{IssueID}/subscription", body: func(*fixture) any { return map[string]any{"subscribed": false} },
			want: [6]int{200, 200, 200, 404, 404, 404}, wantCode: nf("issue_not_found")}},
		{"GET /v1/me/subscriptions", row{name: "GET /v1/me/subscriptions", method: "GET",
			path: "/v1/me/subscriptions", want: [6]int{200, 200, 200, 200, 200, 200}}},
	},
	"14": {
		wread("GET /v1/issues/search", "/v1/issues/search?projectId={ProjectID}&q=resume", "project_not_found"),
		werase("POST /v1/issues/{issueId}/erase", "/v1/issues/{IssueID}/erase"),
		werase("POST /v1/comments/{commentId}/erase", "/v1/comments/{CommentID}/erase"),
	},
	"15": {
		wwrite("POST /v1/projects/{projectId}/imports", "POST", "/v1/projects/{ProjectID}/imports", 200, "project_not_found",
			func(*fixture) any {
				return map[string]any{"source": "csv", "items": []any{map[string]any{"externalId": "row-1", "url": "",
					"title": "Imported", "description": "", "priority": "none", "labels": []any{}, "createdAt": "2026-09-25T00:00:00Z"}}}
			}),
	},
}
```

The edit to H16b's `matrix_integration_test.go`, whole:

```go
// type fixture struct { …, appended by Phase W task 07, never renamed:
	IssueID, Issue2ID, IssueNumber, IssueRevision, CommentID, LabelID, WorkBlob string

// in the fixture builder, after the project, its remote and its blob exist:
	buildWorkFixture(t, f)
```

`buildWorkFixture(t *testing.T, f *fixture)` lives in this task's file. It returns at once when
`workMatrixLive` is empty, so the Hub's rows cost nothing extra until a Work route exists. Otherwise
it writes, as `hub_app` with `hub.org_id` set and **by SQL, not by route** (task 07 lands before
any Work route): issue `IssueID` at number 1 with revisions 1 and 2 (head 2, `IssueRevision` =
`"2"`), issue `Issue2ID` at number 2, a `blocks` relation from the first to the second, the label
`LabelID` (`"Chore"`), a comment `CommentID` on the first, and `WorkBlob`, a second project blob.
Both issues are `not_tied` with reason `chore`, `state` `todo`, created by Grace. The rows it writes
obey task 06's keys and this task's guards, so a builder that breaks a guard fails loudly in the
harness rather than hiding in a cell. The database handle it writes through is the one H16b's
builder already holds; its name is **UNVERIFIED** (H16b's builder body is not on `feat/hub` at this
writing).

**Problem codes.** The 404 cells name `issue_not_found`, `label_not_found` and
`comment_not_found`, beside the Hub's own `project_not_found` and `team_not_found`. The three new
codes are added to the Hub's problem-code list additively by the task that first answers them (08,
11, 12); this task only names them.

#### Tests to write first

`migrate/work_guards_integration_test.go` (`//go:build integration`), on task 06's `seedWork`. The
state tests set the two settings with `set_config(…, true)` inside `asApp`'s transaction, exactly as
task 10's store will.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestDoneWithoutACauseIsRefused` | as `hub_app`, an issue whose head serves one criterion; `UPDATE issues SET state = 'done'` with neither setting → `42501`, message prefix `hub: work:` | a guard on the Go side only |
| `TestAHandSetDoneNeedsAReasonWhenItServes` | `person`, `hand`, empty `state_reason`, head serves a criterion → `42501`; the same with reason `"verified by hand on staging"` → ok; an issue serving nothing, `hand`, no reason → ok | a reasonless hand-set done; a reason demanded where task 10 does not |
| `TestAnIntegrationNeverWritesAFact` | `hub.work_actor_kind = 'integration'`, cause `evidence_observed` → `42501` for `done`; `land_trailer` → `42501` for `landed`; `hand` → `42501` for `started`; `integration` and `hand` → ok for `backlog` | a sync token writing `done` (SW.3's pin); a guard that trusts the cause when the actor is not a person |
| `TestAnUnsetActorIsNotAPerson` | cause `lane_bound`, `hub.work_actor_kind` unset → `42501` for `started` | a guard that reads `NULL` as allowed |
| `TestEachFactHasItsOneCause` | for each of `started`, `in_review`, `landed`: its cause → ok; each of the other five causes → `42501`; `hand` into `started` → `42501`; `lane_bound` into `todo` → `42501` | a hand-faked fact; a fact cause accepted for the wrong state |
| `TestLandTrailerLandsAtDoneOnlyWithoutServes` | cause `land_trailer`, `to done`: head serves nothing → ok; head serves one criterion → `42501` | task 10's `no_criteria` shortcut applied to an issue that has criteria |
| `TestAnIssueIsNotBornAFact` | `INSERT` an issue with `state` = `done`, `landed`, `started`, `in_review` → `42501` each; `triage` → ok | a row born `done`, which never moves into it and so skips the state guard |
| `TestARevisionIsImmutableOutsideErase` | as `hub_owner` directly, `UPDATE issue_revisions SET canonical = …` → `42501`; `DELETE` → `42501`; `TRUNCATE` → `42501` | a mutable revision; a guard that trusts the owner role for anything but erasure |
| `TestTheHeadMovesOneStepOntoARealRevision` | head 1 → 3 → `42501`; head 1 → 2 before revision 2 exists → `42501`; after → ok | a head that skips or points at nothing |
| `TestNumberProjectAndCreatorAreFrozen` | as `hub_owner`, `UPDATE issues SET number = 9`, `created_by = …`, `project_id = …` → `42501` | the grant as the only layer (06's belt, this task's braces) |
| `TestTheTeamIsTheProjectsTeam` | `INSERT` an issue or a project key whose `team_id` is another team of the same org → `42501` | a denormalised `team_id` that drifts from the project's |
| `TestACounterOnlyCountsUp` | `last_number` 4 → 5 ok; 5 → 3 → `42501`; 5 → 7 → `42501` | a counter reset, which would re-issue `EP-1` |
| `TestEraseIssueBlanksAndKeeps` | `erase_issue` as `hub_app` → returns the revision count; every revision has `canonical` NULL and its `digest` and number unchanged; `external_url` is `''`; a second call returns 0; `UPDATE issues SET priority` after erase → `42501` | an erase that drops keys or digests; an erased issue that still takes edits |
| `TestEraseRefusesAnotherOrg` | `hub.org_id` = A; `erase_issue(<B's issue>)` and `erase_comment(<B's comment>)` → `P0002` (`no_data_found`) | an owner function that trusts its argument |
| `TestEraseCommentBlanksBodyAndAnchor` | after `erase_comment`: `body` `''`, `anchor` NULL, `resolved_*` kept; as `hub_app`, `UPDATE issue_comments SET body = 'x'` → `42501` | a comment edit through the back door |
| `TestAppendOnlyWorkTables` | `UPDATE`, `DELETE` and `TRUNCATE` on `issue_serves`, `comment_mentions`, `issue_attachments`, as `hub_owner` → `42501` | a table missing from the append-only loop |
| `TestWorkCheckSetsMatchHubapi` | for each `…_known` constraint of task 06, the quoted values in `pg_get_constraintdef` equal the Go list from `hubapi` (states, priorities, reasons, sources, relation kinds, causes of subscription, default states = `InitialStates`, comment kinds = `{"comment"}`) | a Go constant added without a migration, or the reverse |

`server/work_matrix_rows_integration_test.go`, beside the rows:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestWorkRowsCoverTheW1RouteList` | pure, no server: the set of `workRow.pattern` across **every** group equals `workW1Routes`, each exactly once; each row's `name` equals its `pattern` | a W1 route with no row (the scope's "shipped without its authorisation", caught even before the route exists); a row with a typo'd pattern |
| `TestLiveWorkRowsMatchTheRouteTable` | builds the server; every pattern of every **live** group is in `(*Server).Patterns()`; and every pattern of `workW1Routes` that `Patterns()` lists belongs to a live group | a row switched on before its route (the matrix would 404 the `M` cell); a route registered while its task id is missing from `workMatrixLive` (H16b's meta-test catches it too; this one names the task) |
| `TestAWorkRowFailsWithoutItsCheck` | a fixture route `POST /v1/issues/{issueId}/probe`, registered only in this test through H16b's test-only hook with **no** authorisation call, run through the harness as a `wwrite` row → the `V` cell fails (200, want 403) | a harness that passes a row whatever the handler does |

`TestAWorkRowFailsWithoutItsCheck` needs a way to register a route in a test. If H16b's
`export_test.go` has none, the test is dropped and the gap is listed; it is **not** added to a Hub
file by this task (rule H1).

#### Steps

- [ ] **Step 1: Check the base.** Read hub-contract §5.3 and §10.2-§10.4 on the sync base, H16b's
  harness as merged, and task 06's file. If task 06 is already merged into `feat/hub`, the guards
  go in `<next>_work_w1_guards.sql`; otherwise they are appended to `NNNN_work_w1.sql`.
- [ ] **Step 2: Guard tests first.** Write `work_guards_integration_test.go`. Run it: every test
  fails (no trigger, no function).
- [ ] **Step 3: The guards.** Append the SQL above. Run the guard tests and task 06's tests
  until every one passes; then H07a's and H07b's tests on the same database, unchanged.
- [ ] **Step 4: The rows.** Write `work_matrix_rows_integration_test.go` with `workMatrixLive`
  empty, and make the two-line fixture edit in H16b's file. Run `TestWorkRowsCoverTheW1RouteList`
  (green), `TestLiveWorkRowsMatchTheRouteTable` (green: nothing live, nothing registered) and H16b's
  three tests (green, unchanged: no Work row is in `allRows` yet).
- [ ] **Step 5: Commit by pathspec**, the four files. The message names the migration file the
  guards went into and the one line changed in H16b's file.

#### Acceptance

```bash
export PANACEA_APP_HUB_TEST_DATABASE_URL='postgres://postgres@127.0.0.1:5433/postgres?sslmode=disable'
go test ./control-plane/internal/hub/migrate/... -tags integration -race -count=1 -v \
  -run '^Test(DoneWithoutACauseIsRefused|AHandSetDoneNeedsAReasonWhenItServes|AnIntegrationNeverWritesAFact|AnUnsetActorIsNotAPerson|EachFactHasItsOneCause|LandTrailerLandsAtDoneOnlyWithoutServes|AnIssueIsNotBornAFact|ARevisionIsImmutableOutsideErase|TheHeadMovesOneStepOntoARealRevision|NumberProjectAndCreatorAreFrozen|TheTeamIsTheProjectsTeam|ACounterOnlyCountsUp|EraseIssueBlanksAndKeeps|EraseRefusesAnotherOrg|EraseCommentBlanksBodyAndAnchor|AppendOnlyWorkTables|WorkCheckSetsMatchHubapi)$' \
  2>&1 | grep -c '^--- PASS'                                                     # 17
go test ./control-plane/internal/hub/server/ -tags integration -race -count=1 -v \
  -run 'TestWorkRows|TestLiveWorkRows|TestAWorkRow|TestAuthorisationMatrix|TestTheXColumn|TestEveryRoute'
go test ./control-plane/internal/hub/migrate/... ./control-plane/internal/hub/server/... -tags integration -race -count=1
```

Expected: the first prints `17`; the second shows each named test `--- PASS` (or
`TestAWorkRowFailsWithoutItsCheck` absent, per the note above) and ends `ok`; the third ends `ok`
for both packages, so no Hub test changed colour. With the variable unset, every test is `--- SKIP`
and that is reported as a skip, not a pass.

#### What this task does not claim

- **No route exists.** The rows are written and switched off; tasks 08-15 register the routes and
  switch their rows on. Until then the matrix proves nothing about a Work handler.
- **Not the evidence rule.** The guard checks that `done` has an evidence *cause*; it does not check
  that the evidence exists, matches the criterion, or was recorded at the land commit. That is task
  10's rule in Go, with the `issue_proofs` row as its record, and task 23's observation.
- **Not that Entra issues an integration principal.** `integration` and `app` are reserved values
  here; W5 (tasks 145, 150, 159) creates the principals that set them.
- **Not the janitor.** Deleting an attached blob still fails on task 06's key; the janitor skipping
  it is Hub task 24's, as a proposed edit (**UNVERIFIED**).
- **No measured cost.** The state guard adds one indexed lookup per state move; nobody has timed it.

**Parity rows served:** none closed alone. It pins 05 §3.6's authorisation rows behind **L-8**
(fact states are facts) and **L-239** (every write attributed to a person), and the database half of
skeleton rule 2 (*done means proven*; no sync token writes a fact).

#### Addendum (fix batch b02, S03-C1): SW.3's check rule — who may add or change a `check`

Appended so no line above moves. A serves pair's `check` (task 02, `Serve.Check`) is a shell
command the platform runs and then attests to, as the lander, in an `evidence_observed` move
(tasks 23, 23b). So it is **who may write what** in the sharpest sense, and SW.3 owns it:

> **A `check` is added or changed only in a person's own revision**: the principal is a person
> and the revision's title and description are both `H`. Only a member writes a revision at all:
> this task's matrix rows give every issue write hub-contract's *write revisions* row, `M` (`:319-321`).
> A check carried through unchanged passes whoever writes the revision; a restore is exempt.

What it closes, each named so no later task re-derives it:
1. **Not by an agent.** A `text·A` revision — W4's accepted edit proposals (task 141) and
   proposed drafts (task 140), both of which keep `author: "A"` — cannot add or change one.
2. **Not by intake.** SW.6's `ImportItem` has no serves (`task-03.md:543-552`); W5's intake
   drafts write the pair alone (`task-170.md:171`).
3. **Not by a sync or app principal.** Tasks 145, 150 and 159 write with `byPerson` false, the
   same fact that sets `hub.work_actor_kind` to `integration` or `app` for a state move.

**Where it is enforced.** Not in a trigger: decision 4 has the guards read the index, and
`issue_serves` carries no check. It is Go, one function, task 02's `hubapi.ValidateCheckChange`,
called at every revision write — the fake's (task 04's addendum), the Hub's create (task 08's
addendum) and the Hub's edit (task 09's addendum) — and pinned for both by task 05's row W-26.
Because the Hub refuses the revision, the observer never meets a check nobody on the team wrote:
review finding S03-I5's two tests become W-26 and `TestValidateCheckChange`, where the rule is.
Size unchanged here: this section states the rule; the code is task 02's.
