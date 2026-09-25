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

The migration, in ten pieces for reading; the file is their concatenation, in this order.

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

**Piece 4, delegations and agents.** Every foreign key is named, because the cross-org test's case
table is keyed by constraint name (step 2). A delegation's times are written by Go from **one**
clock reading (task 06 asserts `expiresAt − createdAt` equals the TTL exactly, `task-06.md:481`),
so the `CHECK` compares values of one clock; the column default serves a hand-written row only.

```sql
-- ── §7.13 delegations ────────────────────────────────────────────────────────
CREATE FUNCTION all_distinct(a text[]) RETURNS boolean LANGUAGE sql IMMUTABLE AS
  $$ SELECT cardinality(a) = (SELECT count(DISTINCT x) FROM unnest(a) AS x) $$;

CREATE TABLE delegations (
  id             text             PRIMARY KEY CHECK (id ~ '^dl-[0-9a-f]{16}$'),
  org_id         text             NOT NULL REFERENCES orgs (id),
  project_id     text             NOT NULL,
  owner_user_id  text             NOT NULL,
  class          delegation_class NOT NULL,
  routes         text[]           NOT NULL,
  created_at     timestamptz      NOT NULL DEFAULT now(),
  expires_at     timestamptz      NOT NULL,
  renewed_at     timestamptz,
  revoked_at     timestamptz,
  revoked_reason revoke_reason,
  UNIQUE (org_id, id),
  CONSTRAINT delegations_project_fk FOREIGN KEY (org_id, project_id) REFERENCES projects (org_id, id),
  CONSTRAINT delegations_owner_fk   FOREIGN KEY (org_id, owner_user_id) REFERENCES users (org_id, id),
  CHECK ((revoked_at IS NULL) = (revoked_reason IS NULL)),
  -- 1 to 25 distinct route names, each in its class's set (task 03's ClassRoutes, verbatim)
  CHECK (cardinality(routes) BETWEEN 1 AND 25 AND hub.all_distinct(routes)),
  CHECK (routes <@ CASE class
    WHEN 'worker' THEN ARRAY['delegations.read', 'tasks.read', 'agents.start', 'agents.end',
      'agents.read', 'leases.events', 'conflicts.create', 'conflicts.update', 'conflicts.read',
      'policy.read']
    ELSE ARRAY['delegations.read', 'tasks.create', 'tasks.update', 'tasks.read', 'agents.start',
      'agents.end', 'agents.read', 'leases.events', 'conflicts.create', 'conflicts.update',
      'conflicts.read', 'threads.create', 'threads.update', 'messages.append', 'messages.read',
      'escalations.create', 'escalations.read', 'directives.append', 'directives.read',
      'turns.record', 'human_messages.read', 'notes.read', 'policy.read'] END),
  -- PANACEA_APP_HUB_DELEGATION_TTL is at most 24h (contract §12.2, task 01)
  CHECK (expires_at > coalesce(renewed_at, created_at)
     AND expires_at <= coalesce(renewed_at, created_at) + interval '24 hours'),
  CHECK (renewed_at IS NULL OR renewed_at >= created_at)
);
CREATE INDEX delegations_live ON delegations (org_id, owner_user_id, project_id)
  WHERE revoked_at IS NULL;

-- ── §7.15 agents: start, end and role (status is Fleet's, never here) ─────────
CREATE TABLE agents (
  id              text        PRIMARY KEY CHECK (hub.is_agent_id(id)),
  org_id          text        NOT NULL REFERENCES orgs (id),
  project_id      text        NOT NULL,
  owner_user_id   text        NOT NULL,
  role            agent_role  NOT NULL,
  parent_agent_id text        CHECK (hub.is_agent_id(parent_agent_id)),
  task_id         text        CHECK (task_id ~ '^tk-[0-9a-f]{16}$'),
  harness_id      text        NOT NULL CHECK (harness_id ~ '^[a-z][a-z0-9-]{0,31}$'),
  harness_version text        NOT NULL CHECK (harness_version ~ '^[0-9A-Za-z.+-]{1,32}$'),
  lane_ref        text        NOT NULL CHECK (left(lane_ref, 19) = 'refs/panacea/lanes/'
                                          AND char_length(lane_ref) BETWEEN 20 AND 255),
  policy_version  integer     NOT NULL CHECK (policy_version >= 1),
  policy_digest   text        NOT NULL CHECK (policy_digest ~ '^sha256:[0-9a-f]{64}$'),
  started_at      timestamptz NOT NULL DEFAULT now(),
  ended_at        timestamptz,
  exit_class      exit_class,
  UNIQUE (org_id, id),
  UNIQUE (org_id, owner_user_id, id),        -- the target of every "acts" key (decision c)
  CONSTRAINT agents_project_fk FOREIGN KEY (org_id, project_id) REFERENCES projects (org_id, id),
  CONSTRAINT agents_owner_fk   FOREIGN KEY (org_id, owner_user_id) REFERENCES users (org_id, id),
  -- a worker's parent is the same owner's (§7.15); that the parent is an Orchestrator is Go's
  CONSTRAINT agents_parent_fk  FOREIGN KEY (org_id, owner_user_id, parent_agent_id)
    REFERENCES agents (org_id, owner_user_id, id),
  CHECK (role = 'worker' OR parent_agent_id IS NULL),    -- an Orchestrator has no parent
  CHECK ((ended_at IS NULL) = (exit_class IS NULL)),
  CHECK (ended_at IS NULL OR ended_at >= started_at)
);
```

The lane ref's component rules (`hubapi.ValidBranchRef`) stay Go's; the database holds the OD-2
prefix and the 255 bound. `agents.task_id`'s key is added after `team_tasks` exists (piece 5).

**Piece 5, human messages, tasks and lease events.** `human_messages` comes before `team_tasks`
because a task's `titleSource` names one of the owner's (§7.14).

```sql
-- ── §7.22 human messages: append-only, tombstoned on erasure (task 46) ────────
CREATE TABLE human_messages (
  id            text        PRIMARY KEY CHECK (id ~ '^hm-[0-9a-f]{16}$'),
  org_id        text        NOT NULL REFERENCES orgs (id),
  project_id    text        NOT NULL,
  owner_user_id text        NOT NULL,
  to_agent_id   text        NOT NULL,
  text_body     text,
  text_author   text_author NOT NULL CHECK (text_author = 'human'),
  text_agent    text,
  created_at    timestamptz NOT NULL DEFAULT now(),
  tombstoned_at timestamptz,
  UNIQUE (org_id, id),
  UNIQUE (org_id, owner_user_id, id),
  CONSTRAINT human_messages_project_fk FOREIGN KEY (org_id, project_id) REFERENCES projects (org_id, id),
  CONSTRAINT human_messages_owner_fk   FOREIGN KEY (org_id, owner_user_id) REFERENCES users (org_id, id),
  -- to the owner's own agent (§7.22); that it is an Orchestrator is Go's
  CONSTRAINT human_messages_to_fk      FOREIGN KEY (org_id, owner_user_id, to_agent_id)
    REFERENCES agents (org_id, owner_user_id, id),
  CHECK (hub.text_ok(text_body, text_author, text_agent, 2000)),
  CHECK ((text_body IS NULL) = (tombstoned_at IS NOT NULL))   -- a body until erased, then none
);

-- ── §7.14 tasks ──────────────────────────────────────────────────────────────
CREATE TABLE team_tasks (
  id                    text        PRIMARY KEY CHECK (id ~ '^tk-[0-9a-f]{16}$'),
  org_id                text        NOT NULL REFERENCES orgs (id),
  project_id            text        NOT NULL,
  owner_user_id         text        NOT NULL,
  orchestrator_agent_id text        NOT NULL,
  title_body            text        NOT NULL,
  title_author          text_author NOT NULL,
  title_agent           text,
  title_source          text,
  -- a brief key, '#', a section id. The key's grammar is Go's (ParseBriefKey); 400 is above
  -- the longest key a 255-byte branch ref allows, plus a 64-character section.
  brief_ref             text        NOT NULL CHECK (brief_ref ~ '#[a-z0-9][a-z0-9-]{0,63}$'
                                                AND char_length(brief_ref) <= 400),
  why_body              text        NOT NULL,
  why_author            text_author NOT NULL CHECK (why_author = 'agent'),
  why_agent             text,
  scope                 text[]      NOT NULL CHECK (hub.globs_ok(scope)),
  state                 task_state  NOT NULL DEFAULT 'planned',
  revision              integer     NOT NULL DEFAULT 1 CHECK (revision >= 1),
  created_at            timestamptz NOT NULL DEFAULT now(),
  updated_at            timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, id),
  UNIQUE (org_id, owner_user_id, id),
  CONSTRAINT team_tasks_project_fk      FOREIGN KEY (org_id, project_id) REFERENCES projects (org_id, id),
  CONSTRAINT team_tasks_owner_fk        FOREIGN KEY (org_id, owner_user_id) REFERENCES users (org_id, id),
  CONSTRAINT team_tasks_orchestrator_fk FOREIGN KEY (org_id, owner_user_id, orchestrator_agent_id)
    REFERENCES agents (org_id, owner_user_id, id),
  CONSTRAINT team_tasks_title_source_fk FOREIGN KEY (org_id, owner_user_id, title_source)
    REFERENCES human_messages (org_id, owner_user_id, id),
  CHECK (hub.text_ok(title_body, title_author, title_agent, 140)),
  CHECK (hub.text_ok(why_body, why_author, why_agent, 200)),
  CHECK ((title_author = 'human') = (title_source IS NOT NULL))  -- §7.14: a source makes it human
);

-- a worker works on its owner's task
ALTER TABLE agents ADD CONSTRAINT agents_task_fk FOREIGN KEY (org_id, owner_user_id, task_id)
  REFERENCES team_tasks (org_id, owner_user_id, id);

-- ── §7.16 lease events: append-only; Fleet stays authoritative for the lease ──
CREATE TABLE lease_events (
  org_id          text        NOT NULL REFERENCES orgs (id),
  project_id      text        NOT NULL,
  lease_id        text        NOT NULL CHECK (lease_id ~ '^ls-[0-9a-f]{16}$'),
  seq             integer     NOT NULL CHECK (seq >= 1),
  recorded_by     text        NOT NULL,   -- the delegation's owner: the holder's, or an admin's
  holder_agent_id text        NOT NULL,
  mode            lease_mode  NOT NULL,
  globs           text[]      NOT NULL CHECK (cardinality(globs) >= 1 AND hub.globs_ok(globs)),
  tracked_files   integer     NOT NULL CHECK (tracked_files >= 0),
  state           lease_state NOT NULL,
  expires_at      timestamptz NOT NULL,
  at              timestamptz NOT NULL,
  created_at      timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (org_id, lease_id, seq),
  CONSTRAINT lease_events_project_fk  FOREIGN KEY (org_id, project_id) REFERENCES projects (org_id, id),
  CONSTRAINT lease_events_recorder_fk FOREIGN KEY (org_id, recorded_by) REFERENCES users (org_id, id),
  CONSTRAINT lease_events_holder_fk   FOREIGN KEY (org_id, holder_agent_id) REFERENCES agents (org_id, id)
);
```

A lease event's holder is referenced by `(org_id, id)` only: an `admin-released` event is written
under an admin's delegation (§7.16), so its recorder is not the holder's owner. That the recorder
is the holder's owner **or** an admin of the project, and that `seq` is the last plus one, are
Go's (task 06); the primary key refuses a repeated `seq`.

**Piece 6, threads, conflicts and messages.** A thread's two participants are two columns, not an
array, so each has a foreign key. Its deadline is Fleet's clock and `created_at` the Hub's, so the
database checks only that the deadline is after creation; the 30-minute to 24-hour window, with
whatever skew tolerance it needs, is Go's (task 07).

```sql
-- ── §7.18 threads ────────────────────────────────────────────────────────────
CREATE TABLE threads (
  id              text           PRIMARY KEY CHECK (id ~ '^th-[0-9a-f]{16}$'),
  org_id          text           NOT NULL REFERENCES orgs (id),
  project_id      text           NOT NULL,
  owner_user_id   text           NOT NULL,     -- the opener's owner, the delegation's
  opener_agent_id text           NOT NULL,
  participant_a   text           NOT NULL,
  participant_b   text           NOT NULL,
  subject_kind    thread_subject NOT NULL,
  subject_id      text,
  state           thread_state   NOT NULL DEFAULT 'open',
  rounds          integer        NOT NULL DEFAULT 0 CHECK (rounds >= 0),
  deadline        timestamptz    NOT NULL,
  revision        integer        NOT NULL DEFAULT 1 CHECK (revision >= 1),
  created_at      timestamptz    NOT NULL DEFAULT now(),
  UNIQUE (org_id, id),
  CONSTRAINT threads_project_fk FOREIGN KEY (org_id, project_id) REFERENCES projects (org_id, id),
  CONSTRAINT threads_owner_fk   FOREIGN KEY (org_id, owner_user_id) REFERENCES users (org_id, id),
  CONSTRAINT threads_opener_fk  FOREIGN KEY (org_id, owner_user_id, opener_agent_id)
    REFERENCES agents (org_id, owner_user_id, id),
  CONSTRAINT threads_participant_a_fk FOREIGN KEY (org_id, participant_a) REFERENCES agents (org_id, id),
  CONSTRAINT threads_participant_b_fk FOREIGN KEY (org_id, participant_b) REFERENCES agents (org_id, id),
  CHECK (participant_a <> participant_b),
  CHECK (opener_agent_id IN (participant_a, participant_b)),
  CHECK (CASE subject_kind WHEN 'free'     THEN subject_id IS NULL
                           WHEN 'conflict' THEN coalesce(subject_id ~ '^cf-[0-9a-f]{16}$', false)
                           ELSE coalesce(subject_id ~ '^ls-[0-9a-f]{16}$', false) END),
  CHECK (deadline > created_at)
);

-- ── §7.17 conflicts: Fleet's forecaster, under agent A's owner's delegation ──
CREATE TABLE conflicts (
  id            text           PRIMARY KEY CHECK (id ~ '^cf-[0-9a-f]{16}$'),
  org_id        text           NOT NULL REFERENCES orgs (id),
  project_id    text           NOT NULL,
  owner_user_id text           NOT NULL,
  agent_a       text           NOT NULL,
  agent_b       text           NOT NULL,
  tier          conflict_tier  NOT NULL,
  paths         jsonb          NOT NULL CHECK (hub.conflict_paths_ok(paths)),
  basis_a       text           NOT NULL CHECK (basis_a ~ '^([0-9a-f]{40}|[0-9a-f]{64})$'),
  basis_b       text           NOT NULL CHECK (basis_b ~ '^([0-9a-f]{40}|[0-9a-f]{64})$'),
  state         conflict_state NOT NULL DEFAULT 'raised',
  resolution    resolution_kind,
  thread_id     text,
  revision      integer        NOT NULL DEFAULT 1 CHECK (revision >= 1),
  created_at    timestamptz    NOT NULL DEFAULT now(),
  updated_at    timestamptz    NOT NULL DEFAULT now(),
  UNIQUE (org_id, id),
  CONSTRAINT conflicts_project_fk FOREIGN KEY (org_id, project_id) REFERENCES projects (org_id, id),
  CONSTRAINT conflicts_owner_fk   FOREIGN KEY (org_id, owner_user_id) REFERENCES users (org_id, id),
  CONSTRAINT conflicts_agent_a_fk FOREIGN KEY (org_id, owner_user_id, agent_a)
    REFERENCES agents (org_id, owner_user_id, id),
  CONSTRAINT conflicts_agent_b_fk FOREIGN KEY (org_id, agent_b) REFERENCES agents (org_id, id),
  CONSTRAINT conflicts_thread_fk  FOREIGN KEY (org_id, thread_id) REFERENCES threads (org_id, id),
  CHECK (agent_a <> agent_b),
  CHECK (state NOT IN ('accepted', 'resolved') OR resolution IS NOT NULL)   -- §7.17
);

-- ── §7.18 messages: append-only, tombstoned on erasure (task 46) ─────────────
CREATE TABLE messages (
  id              text         PRIMARY KEY CHECK (id ~ '^ms-[0-9a-f]{16}$'),
  org_id          text         NOT NULL REFERENCES orgs (id),
  project_id      text         NOT NULL,
  thread_id       text         NOT NULL,
  sender_agent_id text         NOT NULL,
  sender_user_id  text         NOT NULL,     -- the sender's owner, the delegation's
  kind            message_kind NOT NULL,
  fields          jsonb        NOT NULL CHECK (jsonb_typeof(fields) = 'object'),
  text_body       text,
  text_author     text_author  CHECK (text_author = 'agent'),   -- always an agent's (§7.18)
  text_agent      text,
  templated       boolean      NOT NULL,
  created_at      timestamptz  NOT NULL DEFAULT now(),
  tombstoned_at   timestamptz,
  UNIQUE (org_id, id),
  CONSTRAINT messages_project_fk FOREIGN KEY (org_id, project_id) REFERENCES projects (org_id, id),
  CONSTRAINT messages_thread_fk  FOREIGN KEY (org_id, thread_id) REFERENCES threads (org_id, id),
  CONSTRAINT messages_sender_fk  FOREIGN KEY (org_id, sender_user_id, sender_agent_id)
    REFERENCES agents (org_id, owner_user_id, id),
  CHECK (hub.text_ok(text_body, text_author, text_agent, 600)),
  CHECK (text_author IS NULL OR text_agent = sender_agent_id),     -- a text is its sender's
  CHECK (tombstoned_at IS NULL OR text_body IS NULL),
  CHECK (kind NOT IN ('ask', 'inform') OR text_author IS NOT NULL), -- text required (§7.18)
  CHECK (kind <> 'withdraw' OR text_author IS NULL)                 -- withdraw has none
);
```

A message's `text_author` stays set after erasure, so "had text" survives a tombstone and the two
kind `CHECK`s hold for an erased row. The per-kind `fields` keys, and `accept`·`reject`'s shorter
300-character reason, are Go's (task 07); the database holds 600, the widest.

**Piece 7, escalations and directives.** An escalation's options are two parallel arrays, so the
database can bound each label (80) and consequence (200) without parsing JSON; both texts are the
question's author's, so they carry no author columns of their own (`task-03.md:758-762`). A
question Fleet raises for itself is `agent` text naming **no** agent, the one place an agent text's
`x_agent` is `NULL` (`task-07.md:125`: `fromAgentId` is `""` then), so its triple is checked by
hand rather than through `text_ok`. `directives.read_ids` is task 03's `read`, renamed because a
bare `read` column reads like a verb in every query.

```sql
-- ── §7.19 escalations: readable by the owner and an org auditor only ─────────
CREATE TABLE escalations (
  id                  text            PRIMARY KEY CHECK (id ~ '^es-[0-9a-f]{16}$'),
  org_id              text            NOT NULL REFERENCES orgs (id),
  project_id          text            NOT NULL,
  owner_user_id       text            NOT NULL,
  from_agent_id       text,                                  -- NULL: Fleet raised it
  kind                escalation_kind NOT NULL,
  thread_id           text,
  question_body       text            NOT NULL CHECK (char_length(question_body) BETWEEN 1 AND 300),
  question_author     text_author     NOT NULL CHECK (question_author = 'agent'),
  question_agent      text,
  option_labels       text[]          NOT NULL CHECK (hub.texts_ok(option_labels, 80)),
  option_consequences text[]          NOT NULL CHECK (hub.texts_ok(option_consequences, 200)),
  default_option      integer,
  deadline            timestamptz     NOT NULL,
  answer_choice       integer,
  answer_body         text            CHECK (char_length(answer_body) BETWEEN 1 AND 300),
  answer_author       text_author     CHECK (answer_author = 'human'),
  answered_at         timestamptz,
  created_at          timestamptz     NOT NULL DEFAULT now(),
  UNIQUE (org_id, id),
  UNIQUE (org_id, owner_user_id, id),            -- a directive's approval is the owner's own
  CONSTRAINT escalations_project_fk FOREIGN KEY (org_id, project_id) REFERENCES projects (org_id, id),
  CONSTRAINT escalations_owner_fk   FOREIGN KEY (org_id, owner_user_id) REFERENCES users (org_id, id),
  CONSTRAINT escalations_from_fk    FOREIGN KEY (org_id, owner_user_id, from_agent_id)
    REFERENCES agents (org_id, owner_user_id, id),
  CONSTRAINT escalations_thread_fk  FOREIGN KEY (org_id, thread_id) REFERENCES threads (org_id, id),
  CHECK (question_agent IS NOT DISTINCT FROM from_agent_id),
  CHECK (cardinality(option_labels) = cardinality(option_consequences)),
  CHECK (CASE kind WHEN 'inform' THEN cardinality(option_labels) = 0 AND default_option IS NULL
         ELSE cardinality(option_labels) BETWEEN 2 AND 4
          AND coalesce(default_option BETWEEN 0 AND cardinality(option_labels) - 1, false) END),
  CHECK (deadline > created_at),
  -- an answer: by a human; a real option for a decide, no choice for an acknowledged inform
  CHECK ((answer_body IS NULL) = (answer_author IS NULL)),
  CHECK (answered_at IS NOT NULL OR (answer_choice IS NULL AND answer_body IS NULL)),
  CHECK (answered_at IS NULL OR CASE kind WHEN 'inform' THEN answer_choice IS NULL
         ELSE coalesce(answer_choice BETWEEN 0 AND cardinality(option_labels) - 1, false) END)
);

-- ── §7.20 directives: append-only ────────────────────────────────────────────
CREATE TABLE directives (
  id            text           PRIMARY KEY CHECK (id ~ '^dv-[0-9a-f]{16}$'),
  org_id        text           NOT NULL REFERENCES orgs (id),
  project_id    text           NOT NULL,
  owner_user_id text           NOT NULL,
  from_agent_id text           NOT NULL,
  to_agent_id   text           NOT NULL,
  kind          directive_kind NOT NULL,
  params        jsonb          NOT NULL CHECK (jsonb_typeof(params) = 'object'),
  text_body     text,
  text_author   text_author    CHECK (text_author = 'agent'),
  text_agent    text,
  rung          delivery_rung  NOT NULL,
  origin        turn_origin    NOT NULL,
  read_ids      text[]         NOT NULL CHECK (hub.ids_ok(read_ids, 100, '^(ms|tk|nt|es)-[0-9a-f]{16}$')),
  approved_by   text,
  created_at    timestamptz    NOT NULL DEFAULT now(),
  UNIQUE (org_id, id),
  CONSTRAINT directives_project_fk  FOREIGN KEY (org_id, project_id) REFERENCES projects (org_id, id),
  CONSTRAINT directives_owner_fk    FOREIGN KEY (org_id, owner_user_id) REFERENCES users (org_id, id),
  CONSTRAINT directives_from_fk     FOREIGN KEY (org_id, owner_user_id, from_agent_id)
    REFERENCES agents (org_id, owner_user_id, id),
  CONSTRAINT directives_to_fk       FOREIGN KEY (org_id, owner_user_id, to_agent_id)
    REFERENCES agents (org_id, owner_user_id, id),
  CONSTRAINT directives_approval_fk FOREIGN KEY (org_id, owner_user_id, approved_by)
    REFERENCES escalations (org_id, owner_user_id, id),
  CHECK (from_agent_id <> to_agent_id),
  CHECK (hub.text_ok(text_body, text_author, text_agent, 600)),
  CHECK (text_author IS NULL OR text_agent = from_agent_id),
  CHECK ((kind = 'free') = (text_author IS NOT NULL)),       -- required for free, else refused
  -- the database layer of the taint rule's kind half (§7.20): a Fleet-started free directive
  -- names an approval. That it is an answered decide is Go's (task 07's TaintAllows).
  CHECK (origin = 'human' OR kind <> 'free' OR approved_by IS NOT NULL)
);
```

**Piece 8, notes and team policy.** `team_policies` flattens task 03's nested keys
(`quotas.memberDailyCents` is `member_daily_cents`). It holds **no data class**: task 01 §7.24
makes the class the project's own, read-only here, with its home **UNVERIFIED until merge**, and
task 43 reuses whatever the merged project record carries. It has no column defaults: the
non-zero defaults of OD-24 are task 43's to write, in one place.

```sql
-- ── §7.23 notes: immutable, tombstoned on erasure (task 46) ──────────────────
CREATE TABLE notes (
  id             text        PRIMARY KEY CHECK (id ~ '^nt-[0-9a-f]{16}$'),
  org_id         text        NOT NULL REFERENCES orgs (id),
  project_id     text        NOT NULL,
  author_user_id text        NOT NULL,
  target_kind    note_target NOT NULL,
  target_id      text        NOT NULL,
  text_body      text,
  text_author    text_author NOT NULL CHECK (text_author = 'human'),
  text_agent     text,
  created_at     timestamptz NOT NULL DEFAULT now(),
  tombstoned_at  timestamptz,
  UNIQUE (org_id, id),
  CONSTRAINT notes_project_fk FOREIGN KEY (org_id, project_id) REFERENCES projects (org_id, id),
  CONSTRAINT notes_author_fk  FOREIGN KEY (org_id, author_user_id) REFERENCES users (org_id, id),
  CHECK (CASE target_kind WHEN 'project' THEN target_id = project_id
                          WHEN 'task'    THEN target_id ~ '^tk-[0-9a-f]{16}$'
                          ELSE target_id ~ '^u-[0-9a-f]{16}$' END),
  CHECK (hub.text_ok(text_body, text_author, text_agent, 1000)),
  CHECK ((text_body IS NULL) = (tombstoned_at IS NOT NULL))
);

-- ── §7.24 team policy: one row per team project ──────────────────────────────
CREATE TABLE team_policies (
  project_id                  text            PRIMARY KEY,
  org_id                      text            NOT NULL REFERENCES orgs (id),
  route                       inference_route NOT NULL,
  member_daily_cents          bigint          NOT NULL CHECK (member_daily_cents > 0),
  project_daily_cents         bigint          NOT NULL CHECK (project_daily_cents > 0),
  orchestrator_turns_per_hour integer         NOT NULL CHECK (orchestrator_turns_per_hour BETWEEN 1 AND 30),
  escalation_floors           text[]          NOT NULL CHECK (hub.all_distinct(escalation_floors)
    AND escalation_floors <@ ARRAY['yield-unedited', 'accept-split', 'proceed-both-ps', 'sequence-short']),
  per_pair_per_hour           integer         NOT NULL CHECK (per_pair_per_hour BETWEEN 1 AND 20),
  rounds_per_side             integer         NOT NULL CHECK (rounds_per_side BETWEEN 1 AND 3),
  thread_deadline_minutes     integer         NOT NULL CHECK (thread_deadline_minutes BETWEEN 30 AND 1440),
  revision                    integer         NOT NULL DEFAULT 1 CHECK (revision >= 1),
  updated_at                  timestamptz     NOT NULL DEFAULT now(),
  UNIQUE (org_id, project_id),
  CONSTRAINT team_policies_project_fk FOREIGN KEY (org_id, project_id) REFERENCES projects (org_id, id)
);
```

The ranges are task 01 §7.24's (`task-01.md:480-484`): quotas above zero, turns 1 to 30, pair
budget 1 to 20, rounds 1 to 3, deadline 30 to 1,440 minutes, floors a subset of the four. "Tighten
only" and "lengthen only" compare to the defaults, which are task 43's, so they stay Go's.

**Piece 9, the guards (decision d).** The trigger layer holds even for `hub_owner`, which the
grant layer does not bind.

```sql
-- ── append-only: directives and lease events take 07b's forbid_change ────────
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['directives', 'lease_events'] LOOP
    EXECUTE format('CREATE TRIGGER %I BEFORE UPDATE OR DELETE ON hub.%I
                    FOR EACH ROW EXECUTE FUNCTION hub.forbid_change()', t || '_append_only', t);
    EXECUTE format('CREATE TRIGGER %I BEFORE TRUNCATE ON hub.%I
                    FOR EACH STATEMENT EXECUTE FUNCTION hub.forbid_change()', t || '_no_truncate', t);
  END LOOP;
END $$;

-- ── messages, human messages, notes: their own guards, refusing every change ─
-- Task 46 replaces each body (CREATE OR REPLACE, same name, same trigger) to admit exactly one
-- move: the erasure tombstone, as hub_owner, inside its SECURITY DEFINER function.
CREATE FUNCTION guard_message() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'hub: messages are append-only' USING ERRCODE = 'insufficient_privilege';
END $$;
CREATE FUNCTION guard_human_message() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'hub: human messages are append-only' USING ERRCODE = 'insufficient_privilege';
END $$;
CREATE FUNCTION guard_note() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'hub: a note is immutable' USING ERRCODE = 'insufficient_privilege';
END $$;
CREATE TRIGGER messages_guard       BEFORE UPDATE OR DELETE ON messages
  FOR EACH ROW EXECUTE FUNCTION guard_message();
CREATE TRIGGER human_messages_guard BEFORE UPDATE OR DELETE ON human_messages
  FOR EACH ROW EXECUTE FUNCTION guard_human_message();
CREATE TRIGGER notes_guard          BEFORE UPDATE OR DELETE ON notes
  FOR EACH ROW EXECUTE FUNCTION guard_note();
CREATE TRIGGER messages_no_truncate       BEFORE TRUNCATE ON messages
  FOR EACH STATEMENT EXECUTE FUNCTION forbid_change();
CREATE TRIGGER human_messages_no_truncate BEFORE TRUNCATE ON human_messages
  FOR EACH STATEMENT EXECUTE FUNCTION forbid_change();
CREATE TRIGGER notes_no_truncate          BEFORE TRUNCATE ON notes
  FOR EACH STATEMENT EXECUTE FUNCTION forbid_change();

-- ── an escalation is answered once, and nothing else of it ever moves ────────
CREATE FUNCTION guard_escalation_answer() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    RAISE EXCEPTION 'hub: escalations are never deleted' USING ERRCODE = 'insufficient_privilege';
  END IF;
  IF OLD.answered_at IS NOT NULL OR NEW.answered_at IS NULL
     OR (to_jsonb(NEW) - 'answer_choice' - 'answer_body' - 'answer_author' - 'answered_at')
        IS DISTINCT FROM
        (to_jsonb(OLD) - 'answer_choice' - 'answer_body' - 'answer_author' - 'answered_at')
  THEN
    RAISE EXCEPTION 'hub: an escalation is answered once' USING ERRCODE = 'insufficient_privilege';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER escalations_guard       BEFORE UPDATE OR DELETE ON escalations
  FOR EACH ROW EXECUTE FUNCTION guard_escalation_answer();
CREATE TRIGGER escalations_no_truncate BEFORE TRUNCATE ON escalations
  FOR EACH STATEMENT EXECUTE FUNCTION forbid_change();
```

`to_jsonb(NEW) - … IS DISTINCT FROM to_jsonb(OLD) - …` compares every other column without naming
them, the form task 46 uses (`task-46.md:205`), so a column added to `escalations` later is frozen
without an edit here. The mutable team records (delegations, tasks, agents, conflicts, threads,
team policy) are held by column grants alone, as Hub task 07a holds `teams` and `projects`; their
state machines are Go's, returning 409 `team_state` (task 03's codes).

**Piece 10, row-level security, the grants and Fleet's two lookups.**

```sql
-- ── row-level security: 0001's policy, verbatim, on every team table ─────────
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['delegations', 'agents', 'human_messages', 'team_tasks',
      'lease_events', 'threads', 'conflicts', 'messages', 'escalations', 'directives', 'notes',
      'team_policies'] LOOP
    EXECUTE format('ALTER TABLE hub.%I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('CREATE POLICY org_isolation ON hub.%I TO hub_app
                    USING (org_id = current_setting(''hub.org_id'', true))
                    WITH CHECK (org_id = current_setting(''hub.org_id'', true))', t);
  END LOOP;
END $$;

-- ── grants: exactly these. 0001's GRANT SELECT ON ALL TABLES covered only the tables
-- ── that existed then, and there are no default privileges.
GRANT SELECT ON delegations, agents, human_messages, team_tasks, lease_events, threads,
                conflicts, messages, escalations, directives, notes, team_policies,
                team_event_kinds TO hub_app;
GRANT INSERT ON delegations, agents, human_messages, team_tasks, lease_events, threads,
                conflicts, messages, escalations, directives, notes, team_policies TO hub_app;
GRANT UPDATE (expires_at, renewed_at, revoked_at, revoked_reason)     ON delegations   TO hub_app;
GRANT UPDATE (ended_at, exit_class)                                   ON agents        TO hub_app;
GRANT UPDATE (state, scope, why_body, why_author, why_agent, revision, updated_at)
                                                                      ON team_tasks    TO hub_app;
GRANT UPDATE (state, resolution, thread_id, revision, updated_at)     ON conflicts     TO hub_app;
GRANT UPDATE (state, rounds, revision)                                ON threads       TO hub_app;
GRANT UPDATE (answer_choice, answer_body, answer_author, answered_at) ON escalations   TO hub_app;
GRANT UPDATE (route, member_daily_cents, project_daily_cents, orchestrator_turns_per_hour,
              escalation_floors, per_pair_per_hour, rounds_per_side, thread_deadline_minutes,
              revision, updated_at)                                   ON team_policies TO hub_app;
GRANT UPDATE (mode)                                                   ON projects      TO hub_app;
-- No UPDATE or DELETE on lease_events, messages, directives, human_messages, notes or
-- team_event_kinds; no DELETE on any team table. Erasure is task 46's owner functions.

-- ── Fleet's two lookups (decision g; ADR-H1 §5.10 rules 6 and 7) ─────────────
-- The org Fleet's token tenant is enrolled as, or NULL.
CREATE FUNCTION fleet_org(p_tenant uuid) RETURNS text
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = hub, pg_temp AS $$
  SELECT o.id FROM hub.orgs o WHERE o.entra_tenant_id = p_tenant
$$;

-- The named delegation, only when its org is the tenant's. No row for an unknown id and none
-- for another org's: the caller answers both 401 delegation_revoked, reason "unknown", so
-- Fleet cannot probe for ids (task 01 §4.2). Expiry and revocation are returned, not
-- filtered, so the caller can say which of the two it is.
CREATE FUNCTION fleet_delegation(p_tenant uuid, p_id text)
RETURNS TABLE (org_id text, owner_user_id text, project_id text, class hub.delegation_class,
               routes text[], expires_at timestamptz, revoked_at timestamptz,
               revoked_reason hub.revoke_reason)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = hub, pg_temp AS $$
  SELECT d.org_id, d.owner_user_id, d.project_id, d.class, d.routes, d.expires_at,
         d.revoked_at, d.revoked_reason
    FROM hub.delegations d
    JOIN hub.orgs o ON o.id = d.org_id AND o.entra_tenant_id = p_tenant
   WHERE d.id = p_id
$$;

REVOKE ALL ON FUNCTION fleet_org(uuid), fleet_delegation(uuid, text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION fleet_org(uuid), fleet_delegation(uuid, text) TO hub_app;
```

The functions run as `hub_owner`, which row-level security does not bind (`ENABLE`d, never
`FORCE`d, `task-07a.md:465-470`); the tenant join is their only tenancy check, and
`TestFleetLookupsStayInTheTenant` is what proves it. `STABLE`, not `VOLATILE`: they read and
never write.

#### Tests to write first — `team_schema_integration_test.go` (`//go:build integration`)

Each runs on `hubtest.Migrated(t)`. The seed is Hub task 07a's helper (orgs, users and
`audit_heads` as `hub_owner`; `task-07a.md:512-519`) plus a `teamSeed` in this file: per org, three
users (Linus and Grace, members; Ada, an admin), one team project (`mode = 'team'`), and as
`hub_app` with `hub.org_id` set, in the order the Hub writes: Linus's Orchestrator `agLO` and
worker `agLW`, Grace's Orchestrator `agGO`, a delegation of each, one human message, one task, one
lease event, one thread between `agLO` and `agGO`, one conflict, one message, one escalation, one
directive, one note and one team policy. **Refused** means a `*pgconn.PgError` whose `Code` is the
one named, and, where the row says so, whose message contains the guard's text. A bound is tested
at the limit and one past it, in `é` (two bytes in UTF-8), so a byte count would refuse the limit.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestTeamEnumsAreTask03s` | for each of the 21 types, `pg_enum` labels in `enumsortorder` equal a Go list: 19 built from `hubapi`'s constants (`string(hubapi.LeaseAdminReleased)`, …), `text_author` from `teamtext.AuthorAgent` and `AuthorHuman`, and `project_mode` the literal pair; so the test fails to compile if task 03 renames one | a label respelled (`admin_released`): every Go insert of it fails `22P02` in production, and no unit test sees it |
| `TestTeamTablesSeparateOrgs` | orgs A and B seeded; with `hub.org_id` = A, `SELECT` from each of the 12 tables returns A's rows only; `INSERT` of an org-B row into each → `42501`; with `hub.org_id` unset on a reused connection, zero rows from each. Then, from the catalog: **every** table in schema `hub` with an `org_id` column has `relrowsecurity` and a policy `org_isolation` | a table left out of the RLS loop, including one a later migration adds with no policy |
| `TestOnlyTheOwnersAgentsAct` | as `hub_app`, org A, owner Linus: a task whose Orchestrator is `agGO` → `23503` (`team_tasks_orchestrator_fk`); a message with `sender_agent_id` `agGO` and `sender_user_id` Linus → `23503`; a directive to Grace's agent → `23503`; a worker whose parent is `agGO` → `23503`; a directive approved by Grace's escalation → `23503`; each same row with Linus's own agent → ok | a composite key cut to `(org_id, agent_id)`: another owner's agent accepted, and §5.10 rule 5 held by Go alone |
| `TestAgentIDsAreNeverSessionIDs` | an agent `s-3f2a91c0` → `23514`; `ag-` with 15 hex → `23514`; a message naming `s-3f2a91c0` as sender → `23503` (no such agent can exist) | the `is_agent_id` check dropped: a machine-local session id reaches the Hub (§3.3 rule 2) |
| `TestTheDatabaseBoundsText` | limit ok, limit + 1 → `23514`, for: title 140, why 200, message 600, question 300, option label 80, consequence 200, answer 300, directive 600, human message 2,000, note 1,000; and an empty body → `23514` for each | `octet_length` for `char_length` (a 600-`é` message is 1,200 bytes); a bound off by one; task 02's constant changed in Go and not here |
| `TestTextCarriesItsAuthorClass` | a message whose `text_author` is `human` → `23514`; a message whose `text_agent` is not its sender → `23514`; a human message or note authored `agent` → `23514`; a note with `text_agent` set → `23514`; a task titled `human` with no `title_source` → `23514`, with Linus's own `hm-…` → ok, with Grace's → `23503` | an agent's words stored as a human's, which every reader then trusts (§3.3); a title laundered through another member's message |
| `TestGlobsAreRelativeAndBounded` | a task scope of `/etc/**`, `a/../b`, `a\b`, a 201-character glob, or 21 globs → each `23514`; `src/**` and `[]` → ok; a lease event with no globs → `23514` | `globs_ok` dropped, so a host path reaches the Hub (§3.3 rule 3) |
| `TestAppendOnlyHoldsForBothRoles` | for `messages`, `directives`, `human_messages`, `notes`, `lease_events`: as `hub_app`, `UPDATE` and `DELETE` → `42501` (no grant); as `hub_owner`, `UPDATE`, `DELETE` and `TRUNCATE` → `42501` **with the guard's own text** (`append-only`, `immutable`) | append-only by grants alone, which the owner role bypasses (G4's trigger layer missing) |
| `TestNothingSetsATombstoneYet` | `messages`, `human_messages` and `notes` each have `tombstoned_at timestamptz` nullable; as `hub_owner`, `UPDATE … SET text_body = NULL, tombstoned_at = now()` → `42501`; an `INSERT` with `tombstoned_at` set and a body → `23514`; a human message with no body and no tombstone → `23514` | an erasure path opened before task 46 writes its checks; a row that is "erased" and still holds its words |
| `TestAnEscalationIsAnsweredOnce` | a `decide` answered with choice 1 as `hub_app` → ok; answering it again → `42501`; as `hub_owner`, changing `question_body` of an unanswered one → `42501`; an `inform` acknowledged with `answer_choice` NULL → ok, with choice 0 → `23514`; a `decide` answered with choice 4 of 4 options → `23514`; an answer body with no author → `23514` | a second answer that rewrites what the human chose; the default rewritten after the fact |
| `TestEscalationShapes` | an `inform` with an option → `23514`; a `decide` with 1 or 5 options, or no default, or default 4 of 4 → `23514`; label and consequence arrays of different lengths → `23514`; `question_agent` not `from_agent_id` → `23514`; Fleet's own (both NULL) → ok | an option count unchecked, so an Orchestrator offers the human one "choice" |
| `TestAFleetFreeDirectiveNeedsAnApproval` | `origin` `fleet`, `kind` `free`, no `approved_by` → `23514`; with an escalation of Linus's → ok; `origin` `fleet`, `kind` `pause`, none → ok; `free` without text → `23514`; `pause` with text → `23514`; `read_ids` of 101, or one `s-…` → `23514` | the taint rule's database layer dropped (§7.20), leaving Fleet's verb handler the only check |
| `TestDelegationShapes` | a `worker` listing `messages.append` → `23514`; 26 routes, or one repeated → `23514`; `expires_at` 24 h + 1 s after `created_at` → `23514`, exactly 24 h → ok; `revoked_at` without a reason → `23514` | a worker delegation that can speak across owners (OD-16); a delegation that outlives the TTL cap |
| `TestConflictAndThreadShapes` | a conflict with `agent_a = agent_b` → `23514`; 51 paths → `23514`; a range `[5, 3]` or `[0, 2]` → `23514`; an absolute path → `23514`; `resolved` with no resolution → `23514`; a thread whose opener is not a participant, or with two equal participants, or `free` with a subject id, or `conflict` with an `ls-…` → `23514` | the shape checks dropped, so a malformed record reaches every reader |
| `TestFeedFlagsComeFromTheKind` | as `hub_app`, chained audit events: `escalation.created` inserted with `feed = true` → stored `feed = false, fleet_feed = true`; `task.created` with `feed = false` → `true, true`; `orchestrator_turn.recorded` → `false, false`; a Hub kind (`brief.revised`, Hub `task-26.md:49`) with `feed = true` → `true, false`. Then `team_event_kinds` equals a Go table of task 01's 22 rows | the trigger dropped: a Go bug puts escalations in every member's feed (§3.4); a kind's flags respelled |
| `TestATeamProjectStaysTeam` | `solo` → `team` ok; `team` → `solo` → `42501`; renaming a team project → ok | a project un-shared under its live delegations |
| `TestFleetLookupsStayInTheTenant` | `fleet_org(A's tenant)` → A; an unknown tenant → NULL; `fleet_delegation(A's tenant, A's dl)` → one row, org A; `(B's tenant, A's dl)` → no row; `(A's tenant, an unknown dl)` → no row; a revoked one → one row with `revoked_at` and `revoked_reason` | the tenant join dropped: a token for tenant B reads A's delegation, and task 05 would make B's Fleet act as A's member |

#### Steps

- [ ] **Step 1: Re-read the merged `0001_init.sql` for the names this part relies on.** From the
  repo root, after the sync-merge of `feat/hub`:

  ```bash
  ~/.claude/tools/claudezsh/claudezsh 'ls control-plane/internal/hub/migrate/sql/ && grep -c "UNIQUE (org_id, id)" control-plane/internal/hub/migrate/sql/0001_init.sql && grep -n "FUNCTION forbid_change\|POLICY org_isolation\|feed          boolean\|entra_tenant_id uuid" control-plane/internal/hub/migrate/sql/0001_init.sql'
  ```

  Expected: the highest file number (this part's `NN` is it plus one); a count of at least 9;
  one line each for `forbid_change`, the loop's `org_isolation`, `audit_events.feed` and
  `orgs.entra_tenant_id`. A missing name means Hub task 07a or 07b changed at build time: use the
  merged name everywhere in this file, and record it in the commit message.

- [ ] **Step 2: Move the four Hub tests this migration changes.** Each edit is the smallest that
  keeps its test true:
  1. `TestMigrationAppliesAndReappliesClean` (07a): the expected first `Apply` is every embedded
     version (`for _, m := range migrate.Embedded()`), not `[1]`; `Current` = `Latest()`, not `1`.
  2. `TestHubAppGrantsAreExactlyTheList` (07a): its expected table and column lists gain piece
     10's grants, verbatim, and `UPDATE (mode)` on `projects`.
  3. `TestACrossOrgReferenceIsRefused` (07a): one case per named key of pieces 4 to 8, 41 in all
     (`delegations_project_fk` … `team_policies_project_fk`), each an org-A row naming org B's
     parent in that one column. Its own "the set exercised equals the catalog's set" assertion is
     what makes a forgotten case fail.
  4. `TestHubAppExecutesExactlyTheOwnerFunctions` (07b): the list gains `fleet_org` and
     `fleet_delegation`.

  **UNVERIFIED until merge:** the shape of each test's expected list. If a test reads its list
  from the contract rather than from code, the edit is task 01's contract text, not this file.

- [ ] **Step 3: Write `team_schema_integration_test.go`, run it, watch it fail.** Every test fails
  with `42P01` (`relation "hub.agents" does not exist`) or its type's equivalent; that is the red run.

- [ ] **Step 4: Write `sql/00NN_team.sql` as pieces 1 to 10, in order. Run to green:**

  ```bash
  ~/.claude/tools/claudezsh/claudezsh 'cd control-plane && go test -tags integration ./internal/hub/migrate/ -race -count=1 2>&1 | tail -n 1'
  ```

  Expected: `ok  	github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/migrate	N.NNNs`.
  This needs Hub task 06's Postgres container; if Docker is not available the step is deferred to
  task 33's environment and says so in the commit message.

- [ ] **Step 5: Kill four mutants by hand**, one at a time, each reverted before the next:
  1. `text_ok`'s `char_length(body)` → `octet_length(body)`. Expected: `TestTheDatabaseBoundsText`
     fails at the limit (`é` × 600).
  2. `messages_sender_fk` → `FOREIGN KEY (org_id, sender_agent_id) REFERENCES agents (org_id, id)`.
     Expected: `TestOnlyTheOwnersAgentsAct` fails on the message row.
  3. Drop `messages_guard`. Expected: `TestAppendOnlyHoldsForBothRoles` fails for the owner.
  4. In `audit_feed_flags`, drop `NEW.feed := k.feed;`. Expected: `TestFeedFlagsComeFromTheKind`
     fails on `escalation.created`.

- [ ] **Step 6: Commit** the migration, the new test file and the two modified Hub test files, by
  pathspec, message under `.superpowers/sdd/team/msg/t04.txt`. Do not migrate a persistent
  database from this commit until task 46's migration is written: task 46 replaces three of this
  file's function bodies, which is safe, but a hand-applied `00NN` edited later is `ErrEdited`
  (Hub `task-06.md:54`).

#### Acceptance

```bash
~/.claude/tools/claudezsh/claudezsh 'cd control-plane && go test -tags integration ./internal/hub/migrate/ -count=1 -v 2>&1 | grep -c "^--- PASS: Test\(Team\|OnlyTheOwners\|AgentIDs\|TheDatabase\|TextCarries\|Globs\|AppendOnlyHolds\|Nothing\|AnEscalation\|EscalationShapes\|AFleet\|Delegation\|ConflictAnd\|FeedFlags\|ATeam\|FleetLookups\)"'   # 17
~/.claude/tools/claudezsh/claudezsh 'grep -c "^CREATE TABLE" control-plane/internal/hub/migrate/sql/*_team.sql'     # 13
~/.claude/tools/claudezsh/claudezsh 'grep -c "CREATE POLICY" control-plane/internal/hub/migrate/sql/*_team.sql'     # 1 (the loop)
~/.claude/tools/claudezsh/claudezsh 'cd control-plane && go vet -tags integration ./internal/hub/migrate/ && echo vet-clean'  # vet-clean
```

The prefix list is chosen so no Hub test matches it (Hub 07b's `TestAppendOnlyTriggersHoldEvenForTheOwner`
is why the pattern says `AppendOnlyHolds`).

#### What this task does not claim

- **That the Hub's erasure route covers these rows.** The tombstone columns exist and every guard
  refuses to set them; task 46 writes the erasure functions and replaces the three guard bodies.
- **That these checks are the first layer.** Every refusal here is the second layer behind Go
  (tasks 05 to 07), which answers first with the contract's codes. A `23514` reaching a client
  is a Go bug, answered 500 `internal` by the Hub's error mapping.
- **Any state machine.** A terminal task, a second agent end, a lease `seq` that is not the last
  plus one, an event after a terminal lease state, `rounds` only growing, a thread's two owners
  differing, a parent that is an Orchestrator, a human message's target being one, and the
  deadline windows: all Go's. The database holds shapes, owners, bounds and append-only.
- **The data class, usage roll-ups, or turn rows.** The class's home is task 43's (**UNVERIFIED
  until merge**); `usage_rollups` is task 66's own migration (`task-66.md:79-80`); an
  Orchestrator turn is an audit event with no row (§7.21).
- **That this runs on Azure Postgres Flexible Server unchanged.** Tested on Hub task 06's
  container, as Hub task 07a; phase 8's first apply is the check.
- **That `char_length` is `utf8.RuneCountInString` for every input.** Both count code points in a
  UTF-8 database; the Hub database's encoding is not stated in Hub task 06's part (**UNVERIFIED**:
  step 4's `é` cases fail loudly if it is not UTF-8).
- **That `projects.mode` has a writer.** The column, its grant and its one-way guard exist; which
  route moves a project to `team` is task 43's or the Share task's.
