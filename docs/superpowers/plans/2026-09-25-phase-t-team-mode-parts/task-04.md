### Task 04: Hub migration: team tables, RLS, append-only guards, tombstone columns

**Slice:** TM1-α. **Size:** 3 focused hours. **dependsOn:** 03 (the JSON names the columns
mirror), Hub tasks 06 (the runner and `hubtest`), 07a (`0001_init.sql`'s tables, RLS loop and
grants), 07b (`forbid_change`, the owner-function pattern). **Seam defined:** ST.2, in SQL.

Task 03 put the team records into Go. This task puts them into the Hub's Postgres: twelve tables,
two columns on existing tables, the append-only and immutability guards, and the two owner
functions Fleet's authentication needs. It adds no Go handler; tasks 05 to 07 do. Every claim the
database makes here is tested **in the database, with every Go check off**, as Hub task 07a does.

The Hub's migration is **planned, not built**: `control-plane/internal/hub/migrate/` does not exist
on `feat/hub` (listed 2026-09-25: `internal/hub/` holds `hubapi` and `hubclient` only). Every
`0001_init.sql` line cited below is Hub task 07a's or 07b's part text
(`.claude/worktrees/hub/docs/superpowers/plans/2026-09-23-phase-h-hub-parts/task-07a.md`,
`task-07b.md`), **UNVERIFIED until merge**. Step 1 re-reads the merged file before anything else.

Seven decisions.

**(a) One file, numbered at merge.** `sql/00NN_team.sql`, where `NN` is the merged Hub's highest
migration plus one. Every test below reads the number from `migrate.Latest()`, never a literal,
so tasks 46 and 66, which each add a later migration, do not edit these tests.

**(b) A free text is three columns.** `teamtext.Text` (task 02, `task-02.md:229-235`) is `Body`,
`Author`, `AuthorAgent`, `Owner`, `At`. The row already holds the owner and the time, so a text
field `x` is `x_body text`, `x_author text_author`, `x_agent text`. The single text of a message,
directive, human message and note is `text_body`, `text_author`, `text_agent`: the names task 46
already writes against (`task-46.md:155-158`), so its guards need no rename. `char_length` counts
characters, which in a UTF-8 database are code points, the unit task 02 bounds in
(`task-02.md:21`). The bounds are task 02's `Max…` constants (`task-02.md:135-148`), one `CHECK` per
text column. A tombstoned row keeps its `x_author` and `x_agent` and loses only `x_body`.

**(c) The owner's agents are held by a foreign key, not only by Go.** ADR-H1 §5.10 rule 5 (task
01) says every agent a record says *acts* is the delegation owner's. `agents` carries
`UNIQUE (org_id, owner_user_id, id)`, and a record's acting agent references it together with the
record's owner: `FOREIGN KEY (org_id, owner_user_id, sender_agent_id) REFERENCES agents (org_id,
owner_user_id, id)`. A Fleet mis-stamp that passed Go would still fail `23503`. Agents that are
*named* but do not act (a conflict's `agentB`, a thread's other participant, a lease's holder)
reference `agents (org_id, id)` only, because they are another owner's by design (§7.17, §7.18)
or, for an `admin-released` lease event, written under an admin's delegation (§7.16). Every key
keeps `org_id` first on both sides, which Hub task 07a's `TestEveryForeignKeyCarriesTheOrg` checks
for every key in the catalog, these included (`task-07a.md:530`).

**(d) Append-only and immutable are two layers, as G4.** `messages`, `directives`,
`human_messages`, `notes` and `lease_events` get `INSERT` and no `UPDATE` or `DELETE` grant, **and**
a `BEFORE UPDATE OR DELETE` trigger that refuses the owner too, **and** a `BEFORE TRUNCATE` one.
`directives` and `lease_events` join 07b's `forbid_change` loop (`task-07b.md:46-61`). `messages`,
`human_messages` and `notes` get their own guard functions, `guard_message`, `guard_human_message`
and `guard_note`, which today refuse everything; task 46 replaces their bodies with `CREATE OR
REPLACE` to admit exactly one erasure move (`task-46.md:150-175`). The tombstone column exists
from this task; nothing may set it until task 46.

**(e) The two feed flags come from the kind, in the database.** Task 01's §8 has three feed
values; task 08 reads two booleans (`task-08.md:207-213`): `yes` is `feed, fleet_feed`; `fleet`
is `fleet_feed` only; `no` is neither. `audit_events` gains `fleet_feed`, and a reference table
`team_event_kinds` holds task 01's 22 rows (`task-01.md:512-533`). A `BEFORE INSERT` trigger **sets** both flags for a
kind in that table, whatever the Go side wrote, and sets `fleet_feed = false` for every other
kind. So a Go bug that writes `feed = true` for `escalation.created` cannot put an escalation in
every member's feed (§3.4). The trigger's comment says it overrides, and a test pins it: this is
the database deciding, stated, not a silent default.

**(f) A project's mode is a column, and it moves one way.** `projects.mode` (`solo`·`team`,
default `solo`), which task 01 §7.26 and task 08 filter on. `team` to `solo` is refused by a
trigger: a team project's delegations, tasks and messages would outlive the mode that admits them.
This is this part's choice; `05-design.md` §2.3 names Share and no un-share (**UNVERIFIED** that
the owner wants none). Which route moves it is not this task's (task 43 or the Share task).

**(g) Fleet's two lookups are owner functions.** Before a delegated request has an org, row-level
security hides every delegation. §5.10 rule 6 fixes the shape: a `SECURITY DEFINER` function, the
pattern of `resolve_principal` (`task-07b.md:189`). `fleet_org(tenant)` maps Fleet's token `tid` to
its enrolled org (the exempt feed route, rule 7); `fleet_delegation(tenant, id)` returns the
delegation only when its org is that tenant's. An unknown id and another org's id both return no
row, which task 05 answers as 401 `delegation_revoked`, `reason = "unknown"` (task 01, `:225`).

**Files** (all under `control-plane/internal/hub/migrate/`, after the Hub's merge):
- `sql/00NN_team.sql` — the types, helpers, two new columns, twelve tables, guards, RLS, grants
  and the two owner functions, in the order below (new)
- `team_schema_integration_test.go` — `//go:build integration`, the tests of this part (new)
- `schema_integration_test.go` — Hub task 07a's: `TestMigrationAppliesAndReappliesClean` reads
  the version list from `Embedded()`; `TestHubAppGrantsAreExactlyTheList` gains this part's grants;
  `TestACrossOrgReferenceIsRefused` gains one case per team foreign key (modify)
- `guards_integration_test.go` — Hub task 07b's: `TestHubAppExecutesExactlyTheOwnerFunctions`
  gains `fleet_org` and `fleet_delegation` (modify)

**Interfaces:**
- Produces: the tables `delegations`, `team_tasks`, `agents`, `lease_events`, `conflicts`,
  `threads`, `messages`, `escalations`, `directives`, `human_messages`, `notes`, `team_policies`,
  `team_event_kinds`; `projects.mode`; `audit_events.fleet_feed`; the guard functions
  `guard_message`, `guard_human_message`, `guard_note`, `guard_escalation_answer`,
  `guard_project_mode`, `audit_feed_flags`; the owner functions `fleet_org(uuid) → text` and
  `fleet_delegation(uuid, text) → TABLE`; the helpers `is_agent_id`, `globs_ok`, `ids_ok`,
  `text_ok`, `options_ok`, `conflict_paths_ok`. Every column name is the snake case of task 03's
  JSON key, except the text triples of decision (b).
- Consumes: `0001_init.sql`'s `orgs`, `users`, `projects`, `audit_events`, `forbid_change()`, the
  roles `hub_owner` and `hub_app`, and `current_setting('hub.org_id', true)` as the tenancy key
  (`task-07a.md:452-473`, `task-07b.md:46-61`; **UNVERIFIED until merge**); `hubtest.Migrated`,
  `AppPool`, `OwnerConn` (Hub task 06, `task-06.md:146-156`).
- Required by: 05 (`fleet_org`, `fleet_delegation`), 06 (the first six tables, `mode`,
  `fleet_feed`), 07 (threads to directives, `human_messages`), 08 (`fleet_feed`, `mode`), 43
  (`team_policies`), 46 (`notes`, the three guards, `text_body`, `tombstoned_at`), 66 (numbers its
  own migration after this one). **A later task that adds a team event kind inserts its
  `team_event_kinds` row in its own migration** (task 46's three erasure kinds), or its events fall
  to the `ELSE` branch and Fleet never sees them.

The migration, in six pieces for reading; the file is their concatenation, in this order.

```sql
-- control-plane/internal/hub/migrate/sql/00NN_team.sql
-- 00NN_team.sql — Phase T task 04: the team records (contract §7.13–§7.24, task 01).
-- Applied by `hub migrate` as hub_owner, in one transaction, after 0001_init.sql.
SET LOCAL search_path = hub;

CREATE TYPE project_mode     AS ENUM ('solo', 'team');
CREATE TYPE text_author      AS ENUM ('agent', 'human');
CREATE TYPE delegation_class AS ENUM ('orchestrator', 'worker');
CREATE TYPE revoke_reason    AS ENUM ('owner', 'admin', 'member_removed', 'account_disabled');
CREATE TYPE task_state       AS ENUM ('planned', 'active', 'blocked', 'done', 'abandoned');
CREATE TYPE agent_role       AS ENUM ('orchestrator', 'worker');
CREATE TYPE exit_class       AS ENUM ('done', 'stopped', 'failed', 'access_removed', 'lapsed');
CREATE TYPE lease_mode       AS ENUM ('intent', 'hold', 'bulk');
CREATE TYPE lease_state      AS ENUM ('requested', 'granted', 'contested', 'yielded', 'released',
                                      'expired', 'admin-released');
CREATE TYPE conflict_tier    AS ENUM ('P', 'H', 'M', 'S');
CREATE TYPE conflict_state   AS ENUM ('raised', 'negotiating', 'accepted', 'resolved',
                                      'escalated', 'cleared');
CREATE TYPE resolution_kind  AS ENUM ('split', 'sequence', 'yield', 'build-on', 'proceed-both');
CREATE TYPE thread_state     AS ENUM ('open', 'resolved', 'escalated', 'expired');
CREATE TYPE thread_subject   AS ENUM ('conflict', 'lease', 'free');
CREATE TYPE message_kind     AS ENUM ('ask', 'answer', 'inform', 'propose', 'accept', 'reject',
                                      'counter', 'withdraw');
CREATE TYPE escalation_kind  AS ENUM ('inform', 'decide');
CREATE TYPE directive_kind   AS ENUM ('pause', 'resume', 'narrow', 'wait-for', 'rebase', 'free');
CREATE TYPE delivery_rung    AS ENUM ('piggyback', 'inbox-socket', 'pty-nudge');
CREATE TYPE turn_origin      AS ENUM ('human', 'fleet');
CREATE TYPE note_target      AS ENUM ('project', 'task', 'user');
CREATE TYPE inference_route  AS ENUM ('global', 'au');
```

Every enum's labels are task 03's constants, in its order (`task-03.md:97-279`); step 4's
`TestTeamEnumsAreTask03s` compares them, label by label, against `pg_enum`.

**Piece 2, the helpers the `CHECK`s call.** Each is `IMMUTABLE`, so a `CHECK` may call it, and
names everything with `hub.`, since a `CHECK` runs under the inserting session's `search_path`.
`CASE` orders every guard that must run before a call that raises on the wrong type: SQL does not
promise that `AND` and `OR` evaluate left to right, and `jsonb_array_length` raises on a scalar.

```sql
-- ── helpers ──────────────────────────────────────────────────────────────────
-- An ag- id, and so never a machine-local session id (05-design.md §3.3 rule 2).
CREATE FUNCTION is_agent_id(v text) RETURNS boolean LANGUAGE sql IMMUTABLE AS
  $$ SELECT v ~ '^ag-[0-9a-f]{16}$' $$;

-- The database half of teamtext.ValidGlob (task 02): at most 20, each 1 to 200 characters,
-- relative, no backslash (chr 92), no '..' segment. The rest of the grammar, and breadth,
-- stay Go's: the Hub holds no file list (task 01 §7.13 conventions).
CREATE FUNCTION globs_ok(g text[]) RETURNS boolean LANGUAGE sql IMMUTABLE AS $$
  SELECT cardinality(g) <= 20 AND NOT EXISTS (
    SELECT 1 FROM unnest(g) AS x
     WHERE x IS NULL OR char_length(x) NOT BETWEEN 1 AND 200 OR left(x, 1) = '/'
        OR strpos(x, chr(92)) > 0 OR x ~ '(^|/)\.\.(/|$)')
$$;

-- At most n ids, each matching pattern.
CREATE FUNCTION ids_ok(ids text[], n integer, pattern text) RETURNS boolean
LANGUAGE sql IMMUTABLE AS $$
  SELECT cardinality(ids) <= n
     AND NOT EXISTS (SELECT 1 FROM unnest(ids) AS x WHERE x IS NULL OR x !~ pattern)
$$;

-- A text triple (decision b). A body is 1 to max characters and has an author; an agent's
-- text names its agent, a human's names none. A NULL body (no text, or tombstoned) passes
-- here; whether the table requires one is the table's own CHECK.
CREATE FUNCTION text_ok(body text, author hub.text_author, agent text, max integer)
RETURNS boolean LANGUAGE sql IMMUTABLE AS $$
  SELECT CASE WHEN body IS NOT NULL AND author IS NULL THEN false
              WHEN body IS NOT NULL AND char_length(body) NOT BETWEEN 1 AND max THEN false
              WHEN author = 'agent' THEN coalesce(hub.is_agent_id(agent), false)
              ELSE agent IS NULL END
$$;

-- Each entry 1 to max characters: an escalation's option labels and consequences.
CREATE FUNCTION texts_ok(t text[], max integer) RETURNS boolean LANGUAGE sql IMMUTABLE AS $$
  SELECT NOT EXISTS (SELECT 1 FROM unnest(t) AS x
                      WHERE x IS NULL OR char_length(x) NOT BETWEEN 1 AND max)
$$;

-- A conflict's paths (§7.17): 1 to 50 of {"path", "lines"}; each path passes globs_ok; each
-- has at most 20 ranges [start, end] with 1 <= start <= end.
CREATE FUNCTION conflict_paths_ok(p jsonb) RETURNS boolean LANGUAGE sql IMMUTABLE AS $$
  SELECT CASE
    WHEN jsonb_typeof(p) IS DISTINCT FROM 'array' THEN false
    WHEN jsonb_array_length(p) NOT BETWEEN 1 AND 50 THEN false
    ELSE NOT EXISTS (
      SELECT 1 FROM jsonb_array_elements(p) AS e
       WHERE CASE
         WHEN jsonb_typeof(e) <> 'object' THEN true
         WHEN jsonb_typeof(e->'path') IS DISTINCT FROM 'string' THEN true
         WHEN jsonb_typeof(e->'lines') IS DISTINCT FROM 'array' THEN true
         WHEN NOT hub.globs_ok(ARRAY[e->>'path']) THEN true
         WHEN jsonb_array_length(e->'lines') > 20 THEN true
         ELSE EXISTS (
           SELECT 1 FROM jsonb_array_elements(e->'lines') AS r
            WHERE CASE
              WHEN jsonb_typeof(r) <> 'array' THEN true
              WHEN jsonb_array_length(r) <> 2 THEN true
              WHEN jsonb_typeof(r->0) <> 'number' OR jsonb_typeof(r->1) <> 'number' THEN true
              ELSE (r->>0)::numeric < 1 OR (r->>1)::numeric < (r->>0)::numeric END)
       END)
  END
$$;
```

**Piece 3, the two columns on `0001`'s tables, the event kinds, and their two triggers.**

```sql
-- ── projects.mode (decision f) ───────────────────────────────────────────────
ALTER TABLE projects ADD COLUMN mode project_mode NOT NULL DEFAULT 'solo';

CREATE FUNCTION guard_project_mode() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF OLD.mode = 'team' AND NEW.mode IS DISTINCT FROM 'team' THEN
    RAISE EXCEPTION 'hub: project % is a team project and stays one', OLD.id
      USING ERRCODE = 'insufficient_privilege';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER projects_mode_one_way BEFORE UPDATE OF mode ON projects
  FOR EACH ROW EXECUTE FUNCTION guard_project_mode();

-- ── audit_events.fleet_feed and the team kinds (decision e) ──────────────────
ALTER TABLE audit_events ADD COLUMN fleet_feed boolean NOT NULL DEFAULT false;
CREATE INDEX audit_fleet_feed ON audit_events (org_id, seq) WHERE fleet_feed;

-- Reference data, not org data: no org_id, no row-level security, SELECT only for hub_app.
CREATE TABLE team_event_kinds (
  kind       text    PRIMARY KEY CHECK (kind ~ '^[a-z]+(_[a-z]+)*\.[a-z]+(_[a-z]+)*$'),
  feed       boolean NOT NULL,           -- the member feed, contract §7.11
  fleet_feed boolean NOT NULL,           -- Fleet's feed, contract §7.26
  CHECK (fleet_feed OR NOT feed)         -- a member-feed kind is Fleet's too
);
INSERT INTO team_event_kinds (kind, feed, fleet_feed) VALUES
  ('delegation.created', false, true),       ('delegation.renewed', false, true),
  ('delegation.revoked', false, true),       ('task.created', true, true),
  ('task.updated', true, true),              ('agent.started', true, true),
  ('agent.ended', true, true),               ('lease.state_changed', true, true),
  ('conflict.raised', true, true),           ('conflict.updated', true, true),
  ('thread.opened', false, true),            ('thread.updated', false, true),
  ('message.appended', false, true),         ('escalation.created', false, true),
  ('escalation.answered', false, true),      ('directive.appended', false, true),
  ('orchestrator_turn.recorded', false, false), ('human_message.appended', false, true),
  ('note.created', true, true),              ('team_policy.updated', true, true),
  ('team_policy.route_changed', true, true), ('usage.rolled_up', false, false);

-- OVERRIDES the Hub's Go: for a team kind both flags are the table's, whatever the INSERT
-- said, so no Go bug can put an escalation, directive or human message in every member's
-- feed (§3.4). Every other kind keeps its feed and is not Fleet's; Fleet reads the org's
-- team.member_removed by kind, in task 08's query, not by this flag. Named to fire after
-- 07b's audit_events_append (same-timing triggers fire in name order); the flags are not in
-- the canonical bytes, so the order does not touch the chain.
CREATE FUNCTION audit_feed_flags() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE k hub.team_event_kinds%ROWTYPE;
BEGIN
  SELECT * INTO k FROM hub.team_event_kinds WHERE kind = NEW.kind;
  IF FOUND THEN
    NEW.feed := k.feed;
    NEW.fleet_feed := k.fleet_feed;
  ELSE
    NEW.fleet_feed := false;
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER audit_events_feed_flags BEFORE INSERT ON audit_events
  FOR EACH ROW EXECUTE FUNCTION audit_feed_flags();
```

Postgres fires `BEFORE` row triggers of one event in name order (from knowledge, **UNVERIFIED**
until `TestFeedFlagsComeFromTheKind` runs); nothing here depends on it except the comment.
