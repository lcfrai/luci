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
