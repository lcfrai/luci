### Task 74: the W2 migration, its guards and matrix rows

**Slice:** W2. **Size:** 3 focused hours (an estimate, not a measurement). **Stream:** WH. **Seam:**
none of its own; it is the storage under SW.18. **Depends on** task 72 (the W2 types, whose JSON
shapes fix the column names and the `CHECK` sets) and, through it, on tasks 06 and 07 (the W1
migration, its row-level security, its grants and its guards), sync-merged from `feat/hub` before
this task starts (rule H1). The implementer never merges.

This task writes one SQL file and one database test. It writes no Go route: tasks 75-81 do. Every
rule the routes enforce in Go is enforced **again here**, in the database, so a bug in a handler
cannot bypass it — the reason task 07 exists for W1, applied to W2.

**The head table is `issues`, not `issue_heads`.** Task 06 creates `issues` (`task-06.md:156`) and
no `issue_heads` anywhere in the tree (`grep -c issue_heads task-06.md` → `0`). The scope
paragraph and task 79's part both say `issue_heads`; both mean the head, which is `issues`. This
part follows the tree (rule H1) and reports the wording as a gap.

**Anchors, re-read on 2026-09-27** (parts of this branch, none yet code): `task-06.md:135-360`
(every W1 table), `:370-398` (the row-level-security loop and the grants), `:443-462` (`asApp`,
the catalog tests); `task-07.md:16-28` (`hub.work_cause`, `hub.work_actor_kind`), `:114-210` (the
guard functions and their triggers); `task-72.md:268-309` (`BandHolds`, `ValidCustomStateKey`,
`ValidDate`, `ValidEstimate`, `FeatureBranchRef`), `:351-410` (the codes, `IssuePlan`,
`IssueAlias`); `task-75.md:32-72`, `task-77.md:34-95`, `task-78.md:245-265` (its migration
clause, which this file absorbs), `task-79.md:33-93` (decisions (d), (f) and (g), whose two asks
land here). **UNVERIFIED until merge:** the Hub's next free migration number, the Hub's
migration-test file path, and `phase-h-hub.md:298`'s matrix file, all followed as found.

#### Decisions the scope left open, made here

**(a) One file, the next free number at sync time.** `NNNN_work_plan.sql`, renumbered on a clash
in the sync-merge commit, never re-applied anywhere shared (rule H1). Task 78's migration clause
(`task-78.md:245-265`) is **appended to this file**, not written as a second one: two W2 migrations
would have to be ordered by hand every sync.

**(b) The additive rule holds, with two stated exceptions.** No Hub table is altered and no W1
column is dropped. The two exceptions are task 79's decision (g): `issue_revisions` and
`issue_attachments` lose their three-column foreign key and take `(org_id, issue_id)`, with a
`BEFORE INSERT` trigger that refuses a row whose `project_id` is not the issue's current one. New
rows keep task 06's decision 3 guarantee; old rows keep the project they were written in; a move
becomes possible. Both are named in the migration's header comment, because a reviewer who knows
task 06 will look for them.

**(c) Every rule that the database can hold, it holds.** Five triggers: the band check on
`sub_state`, the exclusive-label check, the parent-loop check, the alias immutability, and the
extension of task 07's head guard to the new columns. A sixth, `guard_issue_move`, admits the
`UPDATE (project_id, team_id, number)` on `issues` only under `hub.work_cause = 'move'`. The Go
routes check the same things first and answer a clean 422 or 409; the trigger's `42501` or `P0001`
is the second line, never the message a person reads.

**(d) `custom_states.band` is a `CHECK`, not an enum.** Task 06's decision 4: `text` with a named
`CHECK (band IN ('before_started', 'after_landed'))`, so adding a third band later is one
migration and no `ALTER TYPE`. The five fact states are **not rows**: they are not custom, and a
`custom_states.key` equal to one of them is refused by the same `CHECK`.

**(e) Drafts get a second policy, not a second table.** `drafts` carries `org_id` and takes the
loop's `org_isolation` like every other table, then one more policy `draft_author_only` with
`USING (author_id = current_setting('hub.user_id', true))` and the same `WITH CHECK`. Postgres
`AND`s the two `USING` clauses for `hub_app`'s `PERMISSIVE`-by-default policies only if the second
is declared `RESTRICTIVE`; it is. A teammate reading another person's draft then sees nothing at
all, which is what L-33's *404, not 403* needs from the storage.

**(f) `issue_aliases` is insert-only and never deleted.** Primary key `(org_id, project_id,
number)` — the org column leads, per task 06's decision 2 — with `issue_id` the target. A
`BEFORE UPDATE OR DELETE OR TRUNCATE` trigger refuses every change. A number is therefore never
reused even after the alias's issue is erased: the row stays and the read answers `erased`.

**(g) The janitor may write exactly one head column.** Task 07's rule is *no sync principal writes
a fact state or `done`*. Extended: with `hub.work_actor_kind = 'integration'`, an `UPDATE` on
`issues` may touch `cycle_id` and nothing else, and only with `hub.work_cause = 'rollover'`. The
rollover event's `actor` is null (L-60) and the database now says so too.

**(h) Every W1 head takes a rank in this migration** (task 79's decision (d)): one `UPDATE` in
`(created_at, id)` order per team, spreading eight-byte keys, so no W1 issue sorts at random. It
runs once, inside the migration, over rows that exist at merge time; task 08's create takes
`store.BottomRank` from then on.

#### Files

| Path | Change |
|---|---|
| `control-plane/internal/hub/migrate/sql/NNNN_work_plan.sql` | new: every table, column, policy, grant and trigger below, plus task 78's clause |
| `control-plane/internal/hub/migrate/migrate_work_plan_test.go` | new: the database tests below (`//go:build integration`); the Hub's own migration-test file is read, not edited, and if its pattern is one file per migration this name follows it (rule H1) |
| `control-plane/internal/hub/server/work_matrix_rows_integration_test.go` ⚑ (task 07's) | `workMatrixRows["74"]` — no routes of its own; the database halves listed under each route task |
| `control-plane/internal/hub/store/work_settings.go` ⚑ (task 07's) | `hub.work_cause` gains `move` and `rollover`; `hub.user_id` is set on every transaction, for the draft policy |

The scope named `server/matrix_integration_test.go`; the Work rows live in task 07's file (task
75's correction, `task-75.md:86-88`). `hub.user_id` may already be set by the Hub's own middleware;
if it is, that line is deleted and the part says so in its commit message.

#### The tables (complete)

```sql
-- NNNN_work_plan.sql — W2 storage. Additive but for two stated exceptions:
--   1. issue_revisions and issue_attachments trade their (org_id, issue_id,
--      project_id) key for (org_id, issue_id) plus a BEFORE INSERT trigger
--      (task 79 decision (g): a move changes issues.project_id).
--   2. issues gains a guarded UPDATE grant on (project_id, team_id, number).
-- Every table carries org_id and joins the row-level-security loop below.
SET LOCAL search_path TO hub;

CREATE TABLE custom_states (
  org_id      text        NOT NULL,
  id          text        NOT NULL,
  team_id     text        NOT NULL,
  key         text        NOT NULL,
  name        text        NOT NULL,
  band        text        NOT NULL CHECK (band IN ('before_started', 'after_landed')),
  position    int         NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (org_id, id),
  UNIQUE (org_id, team_id, key),
  CHECK (key NOT IN ('triage', 'backlog', 'todo', 'started', 'in_review', 'landed', 'done', 'cancelled', 'duplicate')),
  FOREIGN KEY (org_id, team_id) REFERENCES teams (org_id, id) ON DELETE CASCADE
);

CREATE TABLE features (
  org_id      text        NOT NULL,
  id          text        NOT NULL,
  project_id  text        NOT NULL,
  team_id     text        NOT NULL,
  title       text        NOT NULL,
  branch_ref  text        NOT NULL,
  brief_id    text        NOT NULL,
  state       text        NOT NULL CHECK (state IN ('planned', 'active', 'landed', 'cancelled')),
  created_by  text        NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now(),
  archived_at timestamptz,
  PRIMARY KEY (org_id, id),
  UNIQUE (org_id, id, project_id),
  UNIQUE (org_id, project_id, branch_ref),          -- the reservation, 04 §2.5
  FOREIGN KEY (org_id, project_id) REFERENCES projects (org_id, id) ON DELETE CASCADE,
  FOREIGN KEY (org_id, brief_id)   REFERENCES briefs   (org_id, id)
);

CREATE TABLE cycles (
  org_id      text        NOT NULL,
  id          text        NOT NULL,
  team_id     text        NOT NULL,
  number      int         NOT NULL CHECK (number > 0),
  name        text        NOT NULL DEFAULT '',
  starts_on   date        NOT NULL,
  ends_on     date        NOT NULL,
  state       text        NOT NULL CHECK (state IN ('upcoming', 'active', 'closed')),
  created_at  timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (org_id, id),
  UNIQUE (org_id, team_id, number),                 -- gapless, per team, L-60
  CHECK (ends_on > starts_on),
  FOREIGN KEY (org_id, team_id) REFERENCES teams (org_id, id) ON DELETE CASCADE
);
```

```sql
CREATE TABLE cadences (
  org_id        text        NOT NULL,
  team_id       text        NOT NULL,
  length_days   int         NOT NULL CHECK (length_days BETWEEN 1 AND 56),
  cooldown_days int         NOT NULL DEFAULT 0 CHECK (cooldown_days >= 0),
  starts_on     date        NOT NULL,
  ahead         int         NOT NULL DEFAULT 2 CHECK (ahead BETWEEN 0 AND 6),
  rollover      boolean     NOT NULL DEFAULT true,
  updated_at    timestamptz NOT NULL DEFAULT now(),
  updated_by    text        NOT NULL,
  PRIMARY KEY (org_id, team_id),                    -- one cadence per team
  FOREIGN KEY (org_id, team_id) REFERENCES teams (org_id, id) ON DELETE CASCADE
);

CREATE TABLE cycle_scope (
  org_id     text        NOT NULL,
  id         bigserial   NOT NULL,
  cycle_id   text        NOT NULL,
  issue_id   text        NOT NULL,
  kind       text        NOT NULL CHECK (kind IN ('added', 'removed')),
  at         timestamptz NOT NULL DEFAULT now(),
  actor_id   text,                                  -- null: the janitor's rollover
  PRIMARY KEY (org_id, id),
  FOREIGN KEY (org_id, cycle_id) REFERENCES cycles (org_id, id) ON DELETE CASCADE,
  FOREIGN KEY (org_id, issue_id) REFERENCES issues (org_id, id) ON DELETE CASCADE
);

CREATE TABLE closure_proposals (
  org_id      text        NOT NULL,
  id          text        NOT NULL,
  project_id  text        NOT NULL,
  proposed_by text        NOT NULL,
  proposed_at timestamptz NOT NULL DEFAULT now(),
  answered_at timestamptz,
  answered_by text,
  issue_ids   text[]      NOT NULL CHECK (cardinality(issue_ids) BETWEEN 1 AND 100),
  closed_ids  text[]      NOT NULL DEFAULT '{}',
  PRIMARY KEY (org_id, id),
  FOREIGN KEY (org_id, project_id) REFERENCES projects (org_id, id) ON DELETE CASCADE
);

CREATE TABLE templates (
  org_id      text        NOT NULL,
  id          text        NOT NULL,
  team_id     text,                                 -- null: an org template (§5.2)
  name        text        NOT NULL,
  body        jsonb       NOT NULL,
  created_by  text        NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now(),
  archived_at timestamptz,
  PRIMARY KEY (org_id, id),
  FOREIGN KEY (org_id, team_id) REFERENCES teams (org_id, id) ON DELETE CASCADE
);

CREATE TABLE label_groups (
  org_id     text        NOT NULL,
  id         text        NOT NULL,
  team_id    text,                                  -- null: an org group
  name       text        NOT NULL,
  exclusive  boolean     NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (org_id, id),
  UNIQUE (org_id, id, team_id),
  FOREIGN KEY (org_id, team_id) REFERENCES teams (org_id, id) ON DELETE CASCADE
);

ALTER TABLE labels ADD COLUMN group_id text;
ALTER TABLE labels ADD FOREIGN KEY (org_id, group_id) REFERENCES label_groups (org_id, id);
ALTER TABLE labels ALTER COLUMN team_id DROP NOT NULL;   -- null: an org label, L-17
```

`labels.team_id` losing `NOT NULL` is the third and last change to a W1 table; it widens nothing a
W1 row relied on (every existing row keeps its team) and task 06's unique key is rewritten below
to keep org labels distinct from a team's of the same name.

```sql
CREATE TABLE views (
  org_id          text        NOT NULL,
  id              text        NOT NULL,
  team_id         text        NOT NULL,
  name            text        NOT NULL,
  filter          jsonb       NOT NULL,             -- hubapi.FilterGroup, task 72
  grouping        text        NOT NULL DEFAULT '',
  sort            text        NOT NULL DEFAULT 'rank',
  is_team_default boolean     NOT NULL DEFAULT false,
  created_by      text        NOT NULL,
  created_at      timestamptz NOT NULL DEFAULT now(),
  archived_at     timestamptz,
  PRIMARY KEY (org_id, id),
  FOREIGN KEY (org_id, team_id) REFERENCES teams (org_id, id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX views_one_team_default ON views (org_id, team_id)
  WHERE is_team_default AND archived_at IS NULL;    -- one default per team, L-105

CREATE TABLE drafts (
  org_id      text        NOT NULL,
  id          text        NOT NULL,
  author_id   text        NOT NULL,
  project_id  text,
  body        jsonb       NOT NULL,
  updated_at  timestamptz NOT NULL DEFAULT now(),
  expires_at  timestamptz NOT NULL,                 -- task 77's janitor deletes past this
  PRIMARY KEY (org_id, id),
  FOREIGN KEY (org_id, project_id) REFERENCES projects (org_id, id) ON DELETE CASCADE
);

CREATE TABLE issue_aliases (
  org_id     text        NOT NULL,
  project_id text        NOT NULL,
  number     int         NOT NULL,
  issue_id   text        NOT NULL,
  moved_at   timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (org_id, project_id, number),         -- never deleted, never reused, L-35
  FOREIGN KEY (org_id, project_id) REFERENCES projects (org_id, id),
  FOREIGN KEY (org_id, issue_id)   REFERENCES issues   (org_id, id)
);

-- The head's SW.18 columns. Every one is nullable and defaults to null, so
-- every W1 row stays valid and no W1 test changes.
ALTER TABLE issues ADD COLUMN estimate   int,
                   ADD COLUMN due_on     date,
                   ADD COLUMN rank       text,
                   ADD COLUMN parent_id  text,
                   ADD COLUMN feature_id text,
                   ADD COLUMN cycle_id   text,
                   ADD COLUMN sub_state  text;
ALTER TABLE issues ADD FOREIGN KEY (org_id, parent_id)  REFERENCES issues   (org_id, id);
ALTER TABLE issues ADD FOREIGN KEY (org_id, feature_id, project_id)
                        REFERENCES features (org_id, id, project_id);   -- a feature is per project
ALTER TABLE issues ADD FOREIGN KEY (org_id, cycle_id)   REFERENCES cycles   (org_id, id);
ALTER TABLE issues ADD CONSTRAINT issues_parent_not_self CHECK (parent_id IS DISTINCT FROM id);
CREATE INDEX issues_rank   ON issues (org_id, team_id, rank);
CREATE INDEX issues_parent ON issues (org_id, parent_id) WHERE parent_id IS NOT NULL;
CREATE INDEX issues_cycle  ON issues (org_id, cycle_id)  WHERE cycle_id  IS NOT NULL;

-- Task 79 decision (g): the two W1 keys loosen so a move can change project_id.
ALTER TABLE issue_revisions   DROP CONSTRAINT issue_revisions_org_id_issue_id_project_id_fkey;
ALTER TABLE issue_attachments DROP CONSTRAINT issue_attachments_org_id_issue_id_project_id_fkey;
ALTER TABLE issue_revisions   ADD FOREIGN KEY (org_id, issue_id) REFERENCES issues (org_id, id);
ALTER TABLE issue_attachments ADD FOREIGN KEY (org_id, issue_id) REFERENCES issues (org_id, id);

-- Task 79 decision (h): every W1 head takes a rank, in (created_at, id) order
-- per team, eight bytes apart, so nothing sorts at random after the merge.
UPDATE issues SET rank = lpad(to_hex(n * 4096), 8, '0')
FROM (SELECT org_id, id, row_number() OVER (PARTITION BY org_id, team_id
                                            ORDER BY created_at, id) AS n
      FROM issues) s
WHERE issues.org_id = s.org_id AND issues.id = s.id AND issues.rank IS NULL;
```

The constraint names in the two `DROP`s are Postgres's defaults for task 06's unnamed keys. If
task 06 names them, the names there win; the implementer reads `\d issue_revisions` first.

#### Row-level security and grants (complete)

```sql
-- Task 06's loop, over this file's tables. It is written again, not edited, so
-- 06's file is untouched; 06's catalog test reads pg_tables and covers both.
DO $$ DECLARE t text; BEGIN
  FOREACH t IN ARRAY ARRAY['custom_states','features','cycles','cadences','cycle_scope',
                           'closure_proposals','templates','label_groups','views','drafts',
                           'issue_aliases'] LOOP
    EXECUTE format('ALTER TABLE hub.%I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE hub.%I FORCE ROW LEVEL SECURITY', t);
    EXECUTE format('CREATE POLICY org_isolation ON hub.%I TO hub_app
                    USING (org_id = current_setting(''hub.org_id'', true))
                    WITH CHECK (org_id = current_setting(''hub.org_id'', true))', t);
  END LOOP;
END $$;

-- Decision (e): a second, RESTRICTIVE policy, so the two AND together.
CREATE POLICY draft_author_only ON drafts AS RESTRICTIVE TO hub_app
  USING      (author_id = current_setting('hub.user_id', true))
  WITH CHECK (author_id = current_setting('hub.user_id', true));

GRANT SELECT, INSERT ON custom_states, features, cycles, cadences, cycle_scope,
                        closure_proposals, templates, label_groups, views, drafts,
                        issue_aliases                                        TO hub_app;
GRANT UPDATE (name, position)                       ON custom_states      TO hub_app;
GRANT UPDATE (title, state, archived_at)            ON features           TO hub_app;
GRANT UPDATE (name, starts_on, ends_on, state)      ON cycles             TO hub_app;
GRANT UPDATE (length_days, cooldown_days, starts_on, ahead, rollover,
              updated_at, updated_by)               ON cadences           TO hub_app;
GRANT UPDATE (answered_at, answered_by, closed_ids) ON closure_proposals  TO hub_app;
GRANT UPDATE (name, body, archived_at)              ON templates          TO hub_app;
GRANT UPDATE (name, exclusive)                      ON label_groups       TO hub_app;
GRANT UPDATE (name, filter, grouping, sort, is_team_default, archived_at)
                                                    ON views              TO hub_app;
GRANT UPDATE (body, updated_at, expires_at)         ON drafts             TO hub_app;
GRANT DELETE                                        ON drafts, custom_states TO hub_app;
GRANT UPDATE (group_id, team_id)                    ON labels             TO hub_app;
-- The head's new columns, and the move's three, guarded below.
GRANT UPDATE (estimate, due_on, rank, parent_id, feature_id, cycle_id, sub_state,
              project_id, team_id, number)          ON issues             TO hub_app;
```

`issue_aliases` gets `SELECT, INSERT` and nothing else: decision (f)'s immutability is the grant
first and the trigger second. `drafts` gets `DELETE` for the janitor; `custom_states` for a team
admin's removal, which the route refuses while issues hold the state (409 `state_in_use`).

#### The guards (complete)

```sql
-- 1. A sub-state must sit in the band of the head's current state (05 §3.3).
CREATE FUNCTION guard_sub_state() RETURNS trigger
  LANGUAGE plpgsql SECURITY DEFINER SET search_path = hub, pg_temp AS $$
DECLARE b text;
BEGIN
  IF NEW.sub_state IS NULL THEN RETURN NEW; END IF;
  SELECT band INTO b FROM custom_states
   WHERE org_id = NEW.org_id AND team_id = NEW.team_id AND key = NEW.sub_state;
  IF b IS NULL THEN RAISE EXCEPTION 'sub_state_unknown' USING ERRCODE = 'P0001'; END IF;
  IF (b = 'before_started' AND NEW.state NOT IN ('triage','backlog','todo'))
  OR (b = 'after_landed'   AND NEW.state <> 'landed') THEN
    RAISE EXCEPTION 'sub_state_band' USING ERRCODE = 'P0001';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER issues_sub_state_guard BEFORE INSERT OR UPDATE ON issues
  FOR EACH ROW EXECUTE FUNCTION guard_sub_state();
```

```sql
-- 2. Two labels of one exclusive group on one issue (L-18).
CREATE FUNCTION guard_exclusive_labels() RETURNS trigger
  LANGUAGE plpgsql SECURITY DEFINER SET search_path = hub, pg_temp AS $$
DECLARE g text;
BEGIN
  SELECT l.group_id INTO g FROM labels l
   WHERE l.org_id = NEW.org_id AND l.id = NEW.label_id AND l.group_id IS NOT NULL;
  IF g IS NULL THEN RETURN NEW; END IF;
  IF NOT (SELECT exclusive FROM label_groups WHERE org_id = NEW.org_id AND id = g)
    THEN RETURN NEW; END IF;
  IF EXISTS (SELECT 1 FROM issue_labels il JOIN labels l2
                ON l2.org_id = il.org_id AND l2.id = il.label_id
              WHERE il.org_id = NEW.org_id AND il.issue_id = NEW.issue_id
                AND l2.group_id = g AND il.label_id <> NEW.label_id) THEN
    RAISE EXCEPTION 'exclusive_label_group' USING ERRCODE = 'P0001';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER issue_labels_exclusive_guard BEFORE INSERT ON issue_labels
  FOR EACH ROW EXECUTE FUNCTION guard_exclusive_labels();

-- 3. A parent chain may not loop (L-20). Walks upward, at most 64 steps.
CREATE FUNCTION guard_parent_loop() RETURNS trigger
  LANGUAGE plpgsql SECURITY DEFINER SET search_path = hub, pg_temp AS $$
DECLARE cur text := NEW.parent_id; n int := 0;
BEGIN
  WHILE cur IS NOT NULL AND n < 64 LOOP
    IF cur = NEW.id THEN RAISE EXCEPTION 'parent_loop' USING ERRCODE = 'P0001'; END IF;
    SELECT parent_id INTO cur FROM issues WHERE org_id = NEW.org_id AND id = cur;
    n := n + 1;
  END LOOP;
  IF n >= 64 THEN RAISE EXCEPTION 'parent_too_deep' USING ERRCODE = 'P0001'; END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER issues_parent_loop_guard BEFORE INSERT OR UPDATE OF parent_id ON issues
  FOR EACH ROW WHEN (NEW.parent_id IS NOT NULL) EXECUTE FUNCTION guard_parent_loop();

-- 4. An alias row never changes (decision (f)). Task 07's refusal idiom.
CREATE FUNCTION guard_issue_alias() RETURNS trigger
  LANGUAGE plpgsql SECURITY DEFINER SET search_path = hub, pg_temp AS $$
BEGIN RAISE EXCEPTION 'issue_aliases is append only' USING ERRCODE = '42501'; END $$;
CREATE TRIGGER issue_aliases_guard BEFORE UPDATE OR DELETE ON issue_aliases
  FOR EACH ROW EXECUTE FUNCTION guard_issue_alias();
CREATE TRIGGER issue_aliases_no_truncate BEFORE TRUNCATE ON issue_aliases
  EXECUTE FUNCTION guard_issue_alias();

-- 5. The W1 head guard, extended (task 07, decision (g) here). The sync
--    principal writes cycle_id under cause 'rollover', and nothing else.
CREATE OR REPLACE FUNCTION guard_issue_head_columns() RETURNS trigger
  LANGUAGE plpgsql SECURITY DEFINER SET search_path = hub, pg_temp AS $$
BEGIN
  IF current_setting('hub.work_actor_kind', true) = 'person' THEN RETURN NEW; END IF;
  IF NEW.cycle_id IS DISTINCT FROM OLD.cycle_id
     AND current_setting('hub.work_cause', true) = 'rollover'
     AND (NEW.estimate,  NEW.due_on,  NEW.rank,  NEW.parent_id,  NEW.feature_id,  NEW.sub_state)
  IS NOT DISTINCT FROM
         (OLD.estimate,  OLD.due_on,  OLD.rank,  OLD.parent_id,  OLD.feature_id,  OLD.sub_state)
    THEN RETURN NEW;
  END IF;
  IF (NEW.estimate, NEW.due_on, NEW.rank, NEW.parent_id, NEW.feature_id, NEW.cycle_id, NEW.sub_state)
  IS DISTINCT FROM
     (OLD.estimate, OLD.due_on, OLD.rank, OLD.parent_id, OLD.feature_id, OLD.cycle_id, OLD.sub_state)
    THEN RAISE EXCEPTION 'only a person writes a plan field' USING ERRCODE = '42501';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER issues_plan_guard BEFORE UPDATE ON issues
  FOR EACH ROW EXECUTE FUNCTION guard_issue_head_columns();

-- 6. project_id, team_id and number change only under a move.
CREATE FUNCTION guard_issue_move() RETURNS trigger
  LANGUAGE plpgsql SECURITY DEFINER SET search_path = hub, pg_temp AS $$
BEGIN
  IF (NEW.project_id, NEW.team_id, NEW.number) IS DISTINCT FROM
     (OLD.project_id, OLD.team_id, OLD.number)
     AND coalesce(current_setting('hub.work_cause', true), '') <> 'move' THEN
    RAISE EXCEPTION 'an issue changes project only under a move' USING ERRCODE = '42501';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER issues_move_guard BEFORE UPDATE ON issues
  FOR EACH ROW EXECUTE FUNCTION guard_issue_move();
```

```sql
-- The label uniqueness, rewritten for a nullable team (two partial indexes,
-- because UNIQUE (org_id, team_id, name) treats every null team as distinct).
DROP INDEX IF EXISTS labels_org_id_team_id_name_key;
ALTER TABLE labels DROP CONSTRAINT IF EXISTS labels_org_id_team_id_name_key;
CREATE UNIQUE INDEX labels_team_name ON labels (org_id, team_id, name) WHERE team_id IS NOT NULL;
CREATE UNIQUE INDEX labels_org_name  ON labels (org_id, name)          WHERE team_id IS NULL;
```

Task 78's migration clause (`task-78.md:245-265`) is appended here, after this block, unchanged.

#### The matrix rows (`server/work_matrix_rows_integration_test.go` ⚑)

This task adds **no route**, so `workMatrixRows["74"]` is empty and `"74"` is **not** in
`workMatrixLive`. What it adds instead is the database half of every W2 role rule, as the tests
below: a route whose role check is missing still fails, because the principal that reaches the
`UPDATE` is refused by a grant or a trigger. The scope's *"one row per W2 route × every
principal"* is tasks 75-81's, each in its own part, and is recorded here as a correction so no
implementer writes the rows twice.

#### Tests to write first, each with the mutant it kills

`migrate/migrate_work_plan_test.go`, `//go:build integration`, against
`control-plane/cmd/hub/compose.yaml`. `asApp(org, fn)` is task 06's helper (`task-06.md:446`);
`asSync` is the same with `hub.work_actor_kind = 'integration'`. Each test is seen failing before
the migration line that satisfies it.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestTheBandCheckHolds` | `INSERT INTO custom_states (band: 'later')` → `23514`; `band: 'after_landed'` → ok; `key: 'landed'` → `23514` | the `CHECK` dropped; a custom state shadowing a fact state |
| `TestASubStateOutsideItsBandIsRefused` | a `before_started` state on a `started` issue → `P0001` `sub_state_band`; an unknown key → `sub_state_unknown`; the same key on a `todo` issue → ok | the trigger missing, leaving only Go |
| `TestADraftIsInvisibleToATeammate` | draft by Linus; `asApp` with `hub.user_id` = Grace → `SELECT count(*)` is 0 and `UPDATE` touches 0 rows; as Linus → 1 | org-wide isolation only, so a teammate reads a draft (L-33) |
| `TestAnAliasNeverChanges` | `UPDATE issue_aliases` → `42501`; `DELETE` → `42501`; `TRUNCATE` → `42501`; `INSERT` of a second row on one `(project, number)` → `23505` | an alias edited, so a number is reused (L-35) |
| `TestAnExclusivePairIsRefusedInTheDatabase` | group *Type* exclusive, `bug` then `feature` on one issue → `P0001` `exclusive_label_group`; a non-exclusive group takes both; one label twice → the primary key, not the trigger | the trigger missing (L-18) |
| `TestAParentLoopIsRefusedInTheDatabase` | A→B, then `UPDATE B SET parent_id = A` → `P0001` `parent_loop`; a 64-deep chain → `parent_too_deep`; `parent_id = id` → `23514` | a cycle written straight to the table (L-20) |
| `TestTheJanitorWritesOnlyTheCycle` | `asSync` with cause `rollover`: `UPDATE issues SET cycle_id` → ok; `SET estimate` → `42501`; `SET state = 'done'` → task 07's refusal, unchanged; cause `hand` with `cycle_id` → `42501` | the janitor writing a plan field or a state |
| `TestAMoveNeedsItsCause` | `UPDATE issues SET project_id` with no cause → `42501`; with `hub.work_cause = 'move'` → ok, and the revisions written before the move still read their old `project_id` | a project rewritten by any handler; history rewritten (decision (b)) |
| `TestEveryW2TableHasOrgIsolation` | task 06's catalog test, re-run: each of the eleven tables has `relrowsecurity`, `relforcerowsecurity` and an `org_isolation` policy naming `hub.org_id` in both clauses | a table left out of the loop |
| `TestAnotherOrgsW2RowFailsTheCheck` | `hub.org_id` = A, `INSERT INTO cycles` with `org_id` = B → `42501`; the same for `views` and `features` | `USING` without `WITH CHECK` |
| `TestTheFeatureRefIsReserved` | two features on one `(project, branch_ref)` → `23505`; the same ref in another project → ok | two features on one branch (04 §2.5) |
| `TestCycleNumbersAreUniquePerTeam` | two cycles numbered 1 in one team → `23505`; number 1 in a second team → ok; number 0 → `23514` | a global sequence (L-60) |
| `TestOneTeamDefaultView` | two `is_team_default` views in one team → `23505`; archiving the first then setting the second → ok | two defaults, so the team's view is undefined (L-105) |
| `TestTheW1BackfillRanksEveryHead` | three W1 issues created before the migration → after it, three distinct eight-byte ranks in `(created_at, id)` order, and a fourth issue inserted with a null rank is still allowed | heads that sort at random (task 79 decision (d)) |
| `TestTheMigrationIsIdempotentUnderTheRunner` | the runner applied twice → the second is a no-op and the version row is written once | a migration that half-applies on a retry |

#### Steps

- [ ] **Step 1: Check the base.** `\d issues`, `\d issue_revisions` and `\d issue_attachments` as
  task 06 merged them — the real constraint names, the real head columns; the next free migration
  number; whether the Hub's middleware already sets `hub.user_id`; task 07's guard function names.
- [ ] **Step 2: The tests.** All fifteen, against an empty database. They fail: the tables do not
  exist.
- [ ] **Step 3: The file, in this order.** The eleven tables; the three `ALTER`s on W1 tables; the
  head's seven columns, keys and indexes; the two loosened foreign keys; the rank backfill; the
  label indexes; the row-level-security loop and the draft policy; the grants; the six guards;
  task 78's appended clause.
- [ ] **Step 4: Green.** This file's tests, then task 06's and 07's suites unchanged (every W1 row
  keeps every W1 column, and each new column is nullable), then `go test ./internal/hub/... -tags
  integration`.
- [ ] **Step 5: Commit by pathspec**, naming task 07's store file as touched and listing the two
  exceptions to the additive rule in the message, so the sync-merge reviewer sees them first.

#### Acceptance

```bash
cd control-plane
docker compose -f cmd/hub/compose.yaml up -d && sleep 5
export PANACEA_APP_HUB_TEST_DATABASE_URL='postgres://postgres@127.0.0.1:5433/postgres?sslmode=disable'
go test ./internal/hub/migrate/ -tags integration -race -count=1 -v 2>&1 | grep -c '^--- PASS'   # 15
go test ./internal/hub/... -tags integration -race -count=1                       # every package ok
psql "$PANACEA_APP_HUB_TEST_DATABASE_URL" -c "\dt hub.*" | grep -cE "custom_states|features|cycles|cadences|cycle_scope|closure_proposals|templates|label_groups|views|drafts|issue_aliases"   # 11
psql "$PANACEA_APP_HUB_TEST_DATABASE_URL" -Atc "SELECT count(*) FROM pg_policies WHERE schemaname='hub' AND policyname='org_isolation'" # 11 or more
psql "$PANACEA_APP_HUB_TEST_DATABASE_URL" -Atc "SELECT count(*) FROM pg_trigger WHERE tgname IN ('issues_sub_state_guard','issue_labels_exclusive_guard','issues_parent_loop_guard','issue_aliases_guard','issues_plan_guard','issues_move_guard')"   # 6
grep -c "ALTER TABLE issues ADD COLUMN\|ADD COLUMN" internal/hub/migrate/sql/*_work_plan.sql       # 3 or more
```

Expected: `15`, every package `ok`, `11`, `11` or more, `6`. None has been run; the file does not
exist yet (OD-27), and the `6` is the point: every rule the routes check in Go is checked again
here.

#### What this task does not claim

- **Not a migration applied anywhere shared.** W2 migrations are applied nowhere but a developer's
  compose database until `feat/work-layer` merges (rule H1). No dev, no prod.
- **Not W4's or W5's tables.** Recurring issues, intake forms, SLAs and the customer request
  surface have no storage here.
- **Not a measured backfill.** The rank `UPDATE` walks every issue once. On the largest tree
  anyone has (**UNVERIFIED**, no production data exists) it is one pass; nobody has timed it, and
  no lock window is claimed.
- **Not the Go halves.** Every trigger here answers with an error code, never a message a person
  reads. Tasks 75-81 own the clean 422 and 409; a `P0001` reaching a user is a bug in those tasks.
- **Not the Hub's own schema.** No table of `0001_init.sql` is altered. Three W1 **Work** tables
  are (`issues`, `issue_revisions`, `issue_attachments`, plus `labels`), all written by task 06 in
  this branch, all named in the header comment.

#### Corrections to the scope

1. **The head table is `issues`, not `issue_heads`.** The scope and task 79's part both use a name
   that exists nowhere (`grep -c issue_heads` over `task-06.md` → `0`). **Reported as a gap** in
   the skeleton's wording and in task 79's test names; the code here follows the tree.
2. **Task 79's decision (g) lands here**: two W1 foreign keys loosen and `issues` gains a guarded
   `UPDATE (project_id, team_id, number)`. The scope did not anticipate it, and without it a move
   is `23503` that no Go code can fix.
3. **Task 79's decision (d) lands here**: the W1 rank backfill. Without it a W1 issue has no rank
   and sorts at random in every view.
4. **The matrix rows belong to the route tasks**, not to this one: this task adds no route. The
   scope's *"one row per W2 route × every principal"* is satisfied by tasks 75-81, each in its own
   part; `workMatrixRows["74"]` stays empty.
5. **Two label indexes replace one unique key**, because `team_id` becomes nullable and Postgres
   treats every null as distinct — without the partial index two org labels could share a name.
6. **A sixth guard, `guard_issue_move`**, and two new `hub.work_cause` values (`move`,
   `rollover`), which task 07's store file must accept.
7. **`hub.user_id` must be set on every transaction** for the draft policy. If the Hub's
   middleware does not set it, task 07's store does, and that line is this task's.

**Parity rows served:** none alone. This is the storage under the Hub halves of **L-8**, **L-11**,
**L-14**, **L-16**, **L-17**, **L-18**, **L-20**, **L-21**, **L-23**, **L-25**, **L-26**,
**L-33**, **L-35**, **L-37**, **L-60**, **L-61**, **L-63**, **L-93**-**L-99** and **L-105**,
closed by tasks 75-81 with the desktop and recorded by task 97.

#### Addendum, after reading task 77's part

8. **The janitor's actor kind is `janitor`, not `integration`.** Task 77 (`task-77.md:147-163`)
   widens this file's head guard with `current_setting('hub.work_actor_kind') = 'janitor'` and
   three permitted writes (`cycle_id`, `archived_at`, and a stale `cancelled`). `guard_issue_head_
   columns` above is written with that branch named `janitor`; `integration` keeps task 07's
   blanket refusal, so a sync principal writes no head column at all. The implementer writes the
   guard once, here, in the shape task 77 replaces — task 77's `CREATE OR REPLACE` then only adds
   `archived_at` and the stale close.
9. **Task 77 appends to this file too** (`calendar_tokens`, `archived_at`), as task 78 does. Three
   tasks write one migration; the number is taken once, here, and neither 77 nor 78 takes another.
10. **Task 77's part also writes `issue_heads`** (`task-77.md:105`). Same gap as correction 1: the
    table is `issues`. Three parts now carry the wrong name, which is why it is reported rather
    than quietly fixed.
