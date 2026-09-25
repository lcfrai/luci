### Task 116: the W4 migration, its guards and matrix rows

**Slice:** W4. **Size:** 3 focused hours (an estimate, not a measurement). **Stream:** WH.
**dependsOn:** 114 (SW.20's shapes and words), and the W1-W3 migrations it builds on (tasks 06-07,
74, 99's), sync-merged from `feat/hub` first (rule H1). **Seams:** none new; it extends **SW.3**
(no sync or integration principal writes a Work fact) to every W4 table.

Tasks 117-123 each write one family of W4 rows. If the database trusted Go to keep an update
immutable, a dependency acyclic, a cursor one-way and the sync principal out, one handler bug
would break a rule the product promises (L-76's pinned update, OD-27's measured-only insights).
So the rules live in the schema: `CHECK`s, triggers and row-level security, each with a test
that fails when the guard is dropped. This task also adds the matrix's new principal, a member of
a **granted** team, so every route task can add its rows for it.

**Anchors, re-read on 2026-09-25 at `feat/hub` `1271ddb`:** the contract
`docs/superpowers/contracts/2026-09-23-hub-contract.md:309` (`set_config('hub.org_id', …)`, the
row-level-security key), `:333` (*"There are no per-project grants in v1"*, which this task's
table begins to change, additively), `:335-359` (§5.3: the columns `V M A O N X`, *"the
authorisation test (H2) runs every row as every column"*).

**Read in this branch's parts, UNVERIFIED until merge:** task 06's row-level-security loop
(`task-06.md:370-381`, `TO hub_app`, `current_setting('hub.org_id', true)`) and its explicit
grants (`:383-386`); task 07's actor setting `hub.work_actor_kind` ∈ {`person`, `integration`,
`app`} and its guard shape (`task-07.md:29-31`, `:205-222`), its matrix files and
`workMatrixLive` (`:44-51`, `:57-66`); task 74's `features`, `templates` and `teams`-side tables
(scope `tasks-61-80.md:469-497`; no part yet); TM-46's `notes` table (Phase T plan; **UNVERIFIED**
name). The sibling parts that append to this file: task 119 (`work_update_pins`, reusing this
part's `guard_work_update_meta()`, `task-119.md:112-137`) and task 120 (`work_brief_templates`
columns and `work_brief_criteria`, `task-120.md:128-161`; the cycle trigger's `P0001` and message
prefix `hub: dependency_cycle`, `:241-242`).

#### Decisions the scope left open, made here

1. **No text lives in `work_update_meta`, so its guard has no erase exception.** The update's
   words are TM-46's note (`text·H`); this table holds the subject, the health word, the pin and
   the cite keys. TM-46's erase blanks the note's text and never touches this row, so the trigger
   refuses **every** `UPDATE` and `DELETE`, and the scope's "except through the erase function" is
   unneeded. The erase test proves the row survives the note's erase whole.
2. **The cycle guard serialises per org.** Two concurrent inserts, A→B and B→A, would each pass a
   recursive check that cannot see the other. The trigger takes
   `pg_advisory_xact_lock(hashtext('work_feature_deps:' || org_id))` before its CTE, so the second
   waits and then sees the first. It raises `P0001` with the message prefix `hub: dependency_cycle`,
   which task 120's store maps (`task-120.md:241-242`).
3. **A lower cursor is kept, not refused.** A `BEFORE UPDATE` trigger sets
   `NEW.seq := GREATEST(OLD.seq, NEW.seq)`, so the store's upsert may send any seq and the row
   never goes back (task 115's row sends 7 then 5 and reads 7). No `DELETE` grant.
4. **SW.3 extends by one trigger function on every W4 table.** `guard_work_w4_actor()` refuses any
   write unless `hub.work_actor_kind` is `person`, with one exception: the janitor (task 121) may
   `UPDATE` only `last_fired_on` on `work_template_schedules` and `last_reminded_on` on
   `work_reminder_policies`, under the actor kind task 77's janitor sets (`janitor`,
   **UNVERIFIED** spelling until task 77's part).
5. **Cursors are private by a second policy.** Like task 74's drafts, `work_read_cursors` adds a
   policy on `user_id = current_setting('hub.user_id', true)` (task 74's key, **UNVERIFIED**), so
   no query in any handler can read another person's cursor.
6. **The granted-team member is rows, not a column** (the scope's rule): the fixture gains a
   principal `G`, and the G rows are grouped under task **117**'s id in `workMatrixLive`, so they
   switch on with the route that makes a grant mean something (task 07's decision 6).
7. **`features.initiative_id`** gets its foreign key here. Task 72's `Feature.initiativeId` puts
   the column in task 74's table; this task adds the column if absent and the constraint.

#### Files

| Path (under `control-plane/internal/hub/`) | Change |
|---|---|
| `migrate/sql/NNNN_work_roadmap.sql` | new, the **next free number at sync time**, renumbered on a clash (rule H1); tasks 119 and 120 append sections |
| `migrate/work_roadmap_integration_test.go` | new: the migration and guard tests below |
| `server/matrix_integration_test.go` ⚑ | H16b's `fixture` gains `Granted` (a member of team U, which holds a grant on T's project) and one `buildW4Fixture(t, f)` call; additive only |
| `server/work_matrix_rows_integration_test.go` ⚑ | task 07's file: the `G` guard rows, grouped under `"117"` |

#### `NNNN_work_roadmap.sql`, complete: the tables

```sql
-- ── Phase W, W4 (task 116): initiatives, updates, milestones, dependencies,
--    brief templates, the janitor's policies, read cursors, project grants ──
SET search_path = hub, pg_temp;

CREATE TABLE work_initiatives (
  org_id      text NOT NULL,
  id          text PRIMARY KEY CHECK (id ~ '^in-[0-9a-f]{16}$'),
  team_id     text NOT NULL REFERENCES teams (id),
  name        text NOT NULL CHECK (char_length(btrim(name)) BETWEEN 1 AND 120),
  owner_id    text NOT NULL REFERENCES users (id),
  target_date date,
  state       text NOT NULL DEFAULT 'planned'
              CHECK (state IN ('planned', 'active', 'paused', 'done', 'cancelled')),
  created_at  timestamptz NOT NULL,
  created_by  text NOT NULL REFERENCES users (id),
  updated_at  timestamptz NOT NULL
);
CREATE INDEX work_initiatives_team ON work_initiatives (team_id, created_at DESC);

-- The span (Q12): ord 0 is the owning team; the others are linked by a grant (task 117).
CREATE TABLE work_initiative_teams (
  org_id        text NOT NULL,
  initiative_id text NOT NULL REFERENCES work_initiatives (id),
  team_id       text NOT NULL REFERENCES teams (id),
  ord           int  NOT NULL CHECK (ord >= 0 AND ord < 10),
  PRIMARY KEY (initiative_id, team_id),
  UNIQUE (initiative_id, ord)
);

ALTER TABLE features ADD COLUMN IF NOT EXISTS initiative_id text;
ALTER TABLE features ADD CONSTRAINT features_initiative_fk
  FOREIGN KEY (initiative_id) REFERENCES work_initiatives (id);

-- A status update's Work half, keyed by TM-46's note (decision 1). No text here.
CREATE TABLE work_update_meta (
  org_id       text NOT NULL,
  note_id      text PRIMARY KEY REFERENCES notes (id),
  subject_kind text NOT NULL CHECK (subject_kind IN ('feature', 'initiative')),
  subject_id   text NOT NULL,
  health       text NOT NULL CHECK (health IN ('on_track', 'at_risk', 'off_track')),
  pinned_brief text NOT NULL CHECK (pinned_brief ~ '^brief:bf-[0-9a-f]{16}@[1-9][0-9]{0,8}$'),
  cites        text[] NOT NULL DEFAULT '{}' CHECK (cardinality(cites) <= 20),
  created_at   timestamptz NOT NULL,
  CHECK ((subject_kind = 'feature'    AND subject_id ~ '^ft-[0-9a-f]{16}$')
      OR (subject_kind = 'initiative' AND subject_id ~ '^in-[0-9a-f]{16}$'))
);
CREATE INDEX work_update_meta_subject ON work_update_meta (subject_kind, subject_id, created_at DESC);

CREATE TABLE work_milestones (
  org_id     text NOT NULL,
  id         text PRIMARY KEY CHECK (id ~ '^ml-[0-9a-f]{16}$'),
  feature_id text NOT NULL REFERENCES features (id),
  name       text NOT NULL CHECK (char_length(btrim(name)) BETWEEN 1 AND 120),
  date       date,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL
);

-- A named subset of the branch brief's criteria; reached is never stored (04 §2.5).
CREATE TABLE work_milestone_criteria (
  org_id       text NOT NULL,
  milestone_id text NOT NULL REFERENCES work_milestones (id),
  ord          int  NOT NULL CHECK (ord >= 0 AND ord < 100),
  criterion_id text NOT NULL CHECK (char_length(criterion_id) BETWEEN 1 AND 128),
  PRIMARY KEY (milestone_id, ord),
  UNIQUE (milestone_id, criterion_id)
);

-- "from blocks blocks_id". orderViolated is derived at read; there is no column for it.
CREATE TABLE work_feature_deps (
  org_id     text NOT NULL,
  from_id    text NOT NULL REFERENCES features (id),
  blocks_id  text NOT NULL REFERENCES features (id),
  created_at timestamptz NOT NULL,
  created_by text NOT NULL REFERENCES users (id),
  PRIMARY KEY (from_id, blocks_id),
  CHECK (from_id <> blocks_id)
);
CREATE INDEX work_feature_deps_blocks ON work_feature_deps (blocks_id);

-- Beside task 78's issue templates, in the same tp- space; task 120 appends content columns.
CREATE TABLE work_brief_templates (
  org_id             text NOT NULL,
  id                 text PRIMARY KEY CHECK (id ~ '^tp-[0-9a-f]{16}$'),
  team_id            text REFERENCES teams (id),                -- NULL is an org template
  name               text NOT NULL CHECK (char_length(btrim(name)) BETWEEN 1 AND 120),
  criteria           jsonb NOT NULL CHECK (jsonb_typeof(criteria) = 'array'
                                           AND jsonb_array_length(criteria) BETWEEN 1 AND 50),
  issue_template_ids text[] NOT NULL DEFAULT '{}',
  created_at         timestamptz NOT NULL,
  updated_at         timestamptz NOT NULL
);

CREATE TABLE work_template_schedules (
  org_id        text NOT NULL,
  template_id   text PRIMARY KEY REFERENCES templates (id),   -- task 74's issue templates
  every_n       int  NOT NULL CHECK (every_n BETWEEN 1 AND 365),
  every_unit    text NOT NULL CHECK (every_unit IN ('day', 'week', 'month')),
  at_time       time NOT NULL,
  tz            text NOT NULL CHECK (tz <> '' AND tz <> 'Local'),
  last_fired_on date,                                           -- the janitor's, decision 4
  updated_at    timestamptz NOT NULL,
  updated_by    text NOT NULL REFERENCES users (id)
);

CREATE TABLE work_reminder_policies (
  org_id           text NOT NULL,
  subject_kind     text NOT NULL CHECK (subject_kind IN ('feature', 'initiative')),
  subject_id       text NOT NULL,
  every_days       int  NOT NULL CHECK (every_days BETWEEN 1 AND 90),
  last_reminded_on date,                                        -- the janitor's, decision 4
  updated_at       timestamptz NOT NULL,
  updated_by       text NOT NULL REFERENCES users (id),
  PRIMARY KEY (subject_kind, subject_id)
);

CREATE TABLE work_triage_rotations (
  org_id      text NOT NULL,
  team_id     text PRIMARY KEY REFERENCES teams (id),
  members     text[] NOT NULL CHECK (cardinality(members) BETWEEN 1 AND 50),
  period_days int  NOT NULL CHECK (period_days BETWEEN 1 AND 28),
  starts_on   date NOT NULL,
  updated_at  timestamptz NOT NULL,
  updated_by  text NOT NULL REFERENCES users (id)
);

-- One row per person and scope (task 123). seq is a feed seq; it only increases (decision 3).
CREATE TABLE work_read_cursors (
  org_id     text NOT NULL,
  user_id    text NOT NULL REFERENCES users (id),
  scope_kind text NOT NULL CHECK (scope_kind IN ('project', 'feature', 'issue')),
  scope_id   text NOT NULL,
  seq        bigint NOT NULL CHECK (seq >= 0),
  updated_at timestamptz NOT NULL,
  PRIMARY KEY (user_id, scope_kind, scope_id)
);

-- Q12's project-scoped read-and-comment grant. Revocation deletes the row.
CREATE TABLE work_project_grants (
  org_id     text NOT NULL,
  project_id text NOT NULL REFERENCES projects (id),
  team_id    text NOT NULL REFERENCES teams (id),
  scope      text NOT NULL DEFAULT 'read_comment' CHECK (scope = 'read_comment'),
  granted_by text NOT NULL REFERENCES users (id),
  granted_at timestamptz NOT NULL,
  PRIMARY KEY (project_id, team_id)
);
CREATE INDEX work_project_grants_team ON work_project_grants (team_id);
```

`teams`, `users`, `projects` are H's tables (`0001_init.sql`); `features` and `templates` are
task 74's; `notes` is TM-46's. All five names are **UNVERIFIED** until those migrations merge, and
are renamed mechanically at Step 1 if they differ. Task 119's part references `work_features (id)`
for its pins (`task-119.md:122`); the table is task 74's `features`, and 119's line takes that name
at build.

#### `NNNN_work_roadmap.sql`, continued: the guards

```sql
-- ── an update never changes (decision 1). Task 119 reuses this for its pins. ──
CREATE FUNCTION guard_work_update_meta() RETURNS trigger
LANGUAGE plpgsql SET search_path = hub, pg_temp AS $$
BEGIN
  RAISE EXCEPTION 'hub: update_immutable: % on %', TG_OP, TG_TABLE_NAME
    USING ERRCODE = 'insufficient_privilege';
END $$;
CREATE TRIGGER work_update_meta_immutable
  BEFORE UPDATE OR DELETE ON work_update_meta
  FOR EACH ROW EXECUTE FUNCTION guard_work_update_meta();

-- ── a dependency never closes a cycle (decision 2) ──
CREATE FUNCTION guard_work_feature_deps() RETURNS trigger
LANGUAGE plpgsql SET search_path = hub, pg_temp AS $$
BEGIN
  PERFORM pg_advisory_xact_lock(hashtext('work_feature_deps:' || NEW.org_id));
  IF EXISTS (
    WITH RECURSIVE reach(id) AS (
      SELECT d.blocks_id FROM work_feature_deps d WHERE d.from_id = NEW.blocks_id
      UNION
      SELECT d.blocks_id FROM work_feature_deps d JOIN reach r ON d.from_id = r.id
    )
    SELECT 1 FROM reach WHERE id = NEW.from_id
  ) OR NEW.blocks_id = NEW.from_id THEN
    RAISE EXCEPTION 'hub: dependency_cycle: % blocks %', NEW.from_id, NEW.blocks_id
      USING ERRCODE = 'P0001';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER work_feature_deps_acyclic
  BEFORE INSERT ON work_feature_deps
  FOR EACH ROW EXECUTE FUNCTION guard_work_feature_deps();

-- ── a cursor never goes back (decision 3) ──
CREATE FUNCTION guard_work_read_cursor() RETURNS trigger
LANGUAGE plpgsql SET search_path = hub, pg_temp AS $$
BEGIN
  NEW.seq := GREATEST(OLD.seq, NEW.seq);
  RETURN NEW;
END $$;
CREATE TRIGGER work_read_cursors_forward
  BEFORE UPDATE ON work_read_cursors
  FOR EACH ROW EXECUTE FUNCTION guard_work_read_cursor();

-- ── SW.3, extended: a person writes W4; the janitor touches two columns (decision 4) ──
CREATE FUNCTION guard_work_w4_actor() RETURNS trigger
LANGUAGE plpgsql SET search_path = hub, pg_temp AS $$
DECLARE v_kind text := coalesce(current_setting('hub.work_actor_kind', true), '');
BEGIN
  IF v_kind = 'person' THEN
    RETURN CASE TG_OP WHEN 'DELETE' THEN OLD ELSE NEW END;
  END IF;
  IF v_kind = 'janitor' AND TG_OP = 'UPDATE' AND (
       (TG_TABLE_NAME = 'work_template_schedules'
          AND (to_jsonb(NEW) - 'last_fired_on') = (to_jsonb(OLD) - 'last_fired_on'))
    OR (TG_TABLE_NAME = 'work_reminder_policies'
          AND (to_jsonb(NEW) - 'last_reminded_on') = (to_jsonb(OLD) - 'last_reminded_on'))) THEN
    RETURN NEW;
  END IF;
  RAISE EXCEPTION 'hub: work: a % principal does not write %', nullif(v_kind, ''), TG_TABLE_NAME
    USING ERRCODE = 'insufficient_privilege';
END $$;

DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['work_initiatives', 'work_initiative_teams', 'work_update_meta',
      'work_milestones', 'work_milestone_criteria', 'work_feature_deps', 'work_brief_templates',
      'work_template_schedules', 'work_reminder_policies', 'work_triage_rotations',
      'work_read_cursors', 'work_project_grants'] LOOP
    EXECUTE format('CREATE TRIGGER %I BEFORE INSERT OR UPDATE OR DELETE ON hub.%I
                    FOR EACH ROW EXECUTE FUNCTION hub.guard_work_w4_actor()', t || '_actor', t);
  END LOOP;
END $$;
```

Trigger order: Postgres fires `BEFORE` row triggers in name order, so on `work_update_meta` the
actor guard (`…_actor`) fires before `…_immutable`, and on `work_feature_deps` before
`…_acyclic`; either order refuses the same writes, and the tests name the error they expect.
`features.initiative_id` is written by task 118 on `features`, whose guard is task 74's.
