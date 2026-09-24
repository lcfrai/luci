### Task 07a: `sql/0001_init.sql`, part 1 — the tables, row-level security and the grants, and a database test for each

**Lane A. Depends on: 06. About 3 hours. Contract task H0 (and the database half of G4).**

Review M4 split task 07 in two: a ~330-line schema and 28 database tests, debugged against a live
Postgres, do not fit one task. The contract's §10.2–§10.5 is the schema, written from knowledge
and never applied (contract §15). This task writes the tables, row-level security and the grants,
and proves, **in the database and without any Go handler**, the tenancy half of §10.7's "Database"
column: one org never sees another's rows, **no row links to another org's row**, and `hub_app`
holds exactly the grants listed and no more. Task 07b appends the guards and the owner functions
to the same file, and tests those.

**Two corrections that change behaviour**, folded into the contract by task 44a before this task
runs, so the contract and this SQL agree:
- **A16 (review I5):** every foreign key except the one to `orgs` carries `org_id`, onto a
  `UNIQUE (org_id, …)` target. Row-level security checks only a row's own `org_id`, and Postgres
  checks a foreign key without it, so a single-column key let an org-A row name an org-B parent.
- **A18 (review M10), A19 (review M11), the grants half:** `orgs` has its own policy, and
  `hub_app` holds no `UPDATE` on `audit_heads`. Task 07b builds the rest of both: `hub.org_ids()`,
  a `SECURITY DEFINER` `audit_append`, and `hub.lock_audit_head()`.

**Three changes from the contract's text**, each so a test can name what it tamper-tests, none
changing behaviour:
1. The `users` uniqueness is **named** — `CONSTRAINT users_org_id_entra_oid_key UNIQUE (org_id,
   entra_oid)` — because `resolve_principal`'s `ON CONFLICT ON CONSTRAINT` relies on that name and
   the contract relied on Postgres's default naming, unverified.
2. The two hash checks are named — `brief_revisions_canonical_hashes`, `audit_events_hash_chains` —
   because tasks 20 and 27 drop them, as the owner, to simulate tampering that `hub verify` must
   catch.
3. The erasure invariant is named — `brief_revisions_erased_iff_no_content`.

**Files:**
- `control-plane/internal/hub/migrate/sql/0001_init.sql` (new)
- `control-plane/internal/hub/migrate/embed.go` — `Embedded()`, `Latest()` (new)
- `control-plane/internal/hub/hubtest/hubtest.go` — add `Migrated` (modify)
- `control-plane/internal/hub/migrate/schema_integration_test.go` (new)

#### Contract — complete

```go
// control-plane/internal/hub/migrate/embed.go
package migrate

import "embed"

//go:embed sql/*.sql
var embedded embed.FS

// Embedded returns this build's migrations in version order. A bad file name is
// a build-time bug, so it panics rather than returning an error.
func Embedded() []Migration

// Latest is the highest embedded version: the schema version `hub serve` is
// built for (contract §10.1). 1 after this task.
func Latest() int
```

The migration, in three pieces for reading; the file is their concatenation, in this order.

```sql
-- control-plane/internal/hub/migrate/sql/0001_init.sql
-- 0001_init.sql — applied by `hub migrate` as hub_owner, in one transaction.
SET LOCAL search_path = hub;

CREATE TYPE team_role     AS ENUM ('viewer', 'member', 'admin');
CREATE TYPE invite_state  AS ENUM ('pending', 'bound', 'revoked');
CREATE TYPE git_host      AS ENUM ('github', 'azure-devops');
CREATE TYPE brief_scope   AS ENUM ('project', 'branch');
CREATE TYPE brief_status  AS ENUM ('open', 'complete', 'skipped');
CREATE TYPE release_kind  AS ENUM ('run', 'rollback');
CREATE TYPE release_state AS ENUM ('reserved', 'confirmed', 'abandoned', 'expired');
CREATE TYPE pin_reason    AS ENUM ('ship_head', 'inherited', 'no_brief',
                                   'no_complete_revision', 'source_unpinned', 'source_unknown');
CREATE TYPE verdict       AS ENUM ('settled', 'contested');

-- ── identity and teams ───────────────────────────────────────────────────────
CREATE TABLE orgs (
  id              text        PRIMARY KEY CHECK (id ~ '^org-[0-9a-f]{16}$'),
  entra_tenant_id uuid        NOT NULL UNIQUE,
  display_name    text        NOT NULL CHECK (char_length(display_name) BETWEEN 1 AND 200),
  created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE users (
  id           text        PRIMARY KEY CHECK (id ~ '^u-[0-9a-f]{16}$'),
  org_id       text        NOT NULL REFERENCES orgs (id),
  entra_oid    uuid        NOT NULL,
  display_name text        NOT NULL DEFAULT '' CHECK (char_length(display_name) <= 256),
  email        text        NOT NULL DEFAULT '' CHECK (email = lower(email) AND char_length(email) <= 254),
  created_at   timestamptz NOT NULL DEFAULT now(),
  last_seen_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT users_org_id_entra_oid_key UNIQUE (org_id, entra_oid),
  UNIQUE (org_id, id)                      -- the target of every org-carrying user link (I5)
);
CREATE INDEX users_org_email ON users (org_id, email);

-- I5 (review): every foreign key below, except the one to orgs, carries org_id. Postgres
-- checks a foreign key without row-level security, so a single-column link would let an
-- org-A row point at an org-B parent. The composite key makes the database refuse it (23503)
-- with every Go check off. Each parent table carries UNIQUE (org_id, id) as the target.
CREATE TABLE teams (
  id         text        PRIMARY KEY CHECK (id ~ '^tm-[0-9a-f]{16}$'),
  org_id     text        NOT NULL REFERENCES orgs (id),
  name       text        NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
  created_at timestamptz NOT NULL DEFAULT now(),
  created_by text        NOT NULL,
  UNIQUE (org_id, id),
  FOREIGN KEY (org_id, created_by) REFERENCES users (org_id, id)
);
CREATE UNIQUE INDEX teams_org_name ON teams (org_id, lower(name));

CREATE TABLE team_members (
  org_id   text        NOT NULL REFERENCES orgs (id),
  team_id  text        NOT NULL,
  user_id  text        NOT NULL,
  role     team_role   NOT NULL,
  added_at timestamptz NOT NULL DEFAULT now(),
  added_by text,                           -- nullable as before; a NULL skips the check (MATCH SIMPLE)
  PRIMARY KEY (team_id, user_id),
  FOREIGN KEY (org_id, team_id)  REFERENCES teams (org_id, id),
  FOREIGN KEY (org_id, user_id)  REFERENCES users (org_id, id),
  FOREIGN KEY (org_id, added_by) REFERENCES users (org_id, id)
);
CREATE INDEX team_members_user ON team_members (user_id);

CREATE TABLE team_invites (
  id            text         PRIMARY KEY CHECK (id ~ '^iv-[0-9a-f]{16}$'),
  org_id        text         NOT NULL REFERENCES orgs (id),
  team_id       text         NOT NULL,
  email         text         NOT NULL CHECK (email = lower(email) AND char_length(email) BETWEEN 3 AND 254),
  role          team_role    NOT NULL,
  state         invite_state NOT NULL DEFAULT 'pending',
  created_at    timestamptz  NOT NULL DEFAULT now(),
  created_by    text         NOT NULL,
  bound_user_id text,
  bound_at      timestamptz,
  revoked_at    timestamptz,
  CHECK ((state = 'bound')   = (bound_user_id IS NOT NULL AND bound_at IS NOT NULL)),
  CHECK ((state = 'revoked') = (revoked_at IS NOT NULL)),
  FOREIGN KEY (org_id, team_id)       REFERENCES teams (org_id, id),
  FOREIGN KEY (org_id, created_by)    REFERENCES users (org_id, id),
  FOREIGN KEY (org_id, bound_user_id) REFERENCES users (org_id, id)
);
CREATE UNIQUE INDEX team_invites_one_pending ON team_invites (team_id, email) WHERE state = 'pending';
CREATE INDEX team_invites_binding ON team_invites (org_id, email) WHERE state = 'pending';

-- ── projects ─────────────────────────────────────────────────────────────────
CREATE TABLE projects (
  id         text        PRIMARY KEY CHECK (id ~ '^pj-[0-9a-f]{16}$'),
  org_id     text        NOT NULL REFERENCES orgs (id),
  team_id    text        NOT NULL,
  name       text        NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
  created_at timestamptz NOT NULL DEFAULT now(),
  created_by text        NOT NULL,
  UNIQUE (org_id, id),
  FOREIGN KEY (org_id, team_id)    REFERENCES teams (org_id, id),
  FOREIGN KEY (org_id, created_by) REFERENCES users (org_id, id)
);
CREATE UNIQUE INDEX projects_team_name ON projects (team_id, lower(name));

CREATE TABLE project_remotes (
  org_id     text        NOT NULL REFERENCES orgs (id),
  project_id text        NOT NULL,
  repo_key   text        NOT NULL CHECK (char_length(repo_key) BETWEEN 3 AND 300),
  host       git_host    NOT NULL,
  added_at   timestamptz NOT NULL DEFAULT now(),
  added_by   text        NOT NULL,
  PRIMARY KEY (project_id, repo_key),
  UNIQUE (org_id, repo_key),               -- a repository is one project's, org-wide
  FOREIGN KEY (org_id, project_id) REFERENCES projects (org_id, id),
  FOREIGN KEY (org_id, added_by)   REFERENCES users (org_id, id)
);

-- ── briefs ───────────────────────────────────────────────────────────────────
CREATE TABLE briefs (
  id                       text        PRIMARY KEY CHECK (id ~ '^bf-[0-9a-f]{16}$'),
  org_id                   text        NOT NULL REFERENCES orgs (id),
  project_id               text        NOT NULL,
  scope                    brief_scope NOT NULL,
  branch_ref               text        NOT NULL DEFAULT '',
  head_revision            integer     NOT NULL DEFAULT 0 CHECK (head_revision >= 0),
  latest_complete_revision integer     NOT NULL DEFAULT 0,
  archived_at              timestamptz,
  erased_at                timestamptz,
  created_at               timestamptz NOT NULL DEFAULT now(),
  created_by               text        NOT NULL,
  updated_at               timestamptz NOT NULL DEFAULT now(),
  CHECK ((scope = 'project' AND branch_ref = '')
      OR (scope = 'branch'  AND branch_ref LIKE 'refs/heads/_%' AND octet_length(branch_ref) <= 255)),
  CHECK (latest_complete_revision BETWEEN 0 AND head_revision),
  CHECK (erased_at IS NULL OR archived_at IS NOT NULL),
  UNIQUE (org_id, id),
  FOREIGN KEY (org_id, project_id) REFERENCES projects (org_id, id),
  FOREIGN KEY (org_id, created_by) REFERENCES users (org_id, id)
);
CREATE UNIQUE INDEX briefs_live_project ON briefs (project_id)
  WHERE scope = 'project' AND archived_at IS NULL;
CREATE UNIQUE INDEX briefs_live_branch ON briefs (project_id, branch_ref)
  WHERE scope = 'branch' AND archived_at IS NULL;

CREATE TABLE brief_revisions (
  org_id     text         NOT NULL REFERENCES orgs (id),
  brief_id   text         NOT NULL,
  revision   integer      NOT NULL CHECK (revision >= 1),
  project_id text         NOT NULL,
  scope      brief_scope  NOT NULL,
  branch_ref text         NOT NULL,
  status     brief_status NOT NULL,
  canonical  bytea,                                   -- contract §3.4's C; NULL exactly when erased
  digest     bytea        NOT NULL CHECK (octet_length(digest) = 32),
  saved_at   timestamptz  NOT NULL DEFAULT now(),
  saved_by   text         NOT NULL,
  erased_at  timestamptz,
  PRIMARY KEY (brief_id, revision),
  UNIQUE (org_id, brief_id, revision),                -- the target of every revision link (I5)
  UNIQUE (org_id, brief_id, revision, digest),        -- the target of every digest-carrying link
  FOREIGN KEY (org_id, brief_id)   REFERENCES briefs (org_id, id),
  FOREIGN KEY (org_id, project_id) REFERENCES projects (org_id, id),
  FOREIGN KEY (org_id, saved_by)   REFERENCES users (org_id, id),
  CONSTRAINT brief_revisions_erased_iff_no_content CHECK ((canonical IS NULL) = (erased_at IS NOT NULL)),
  CONSTRAINT brief_revisions_canonical_hashes
    CHECK (canonical IS NULL OR (octet_length(canonical) <= 65536 AND sha256(canonical) = digest))
);
```

```sql
-- 0001_init.sql, continued: blobs, approvals, links, releases, evidence, audit.
-- ── blobs ────────────────────────────────────────────────────────────────────
CREATE TABLE blobs (
  org_id     text        NOT NULL REFERENCES orgs (id),
  digest     bytea       NOT NULL CHECK (octet_length(digest) = 32),
  size       integer     NOT NULL CHECK (size BETWEEN 0 AND 16777216),
  bytes      bytea       NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (org_id, digest),
  CHECK (octet_length(bytes) = size AND sha256(bytes) = digest)
);

CREATE TABLE project_blobs (
  org_id       text        NOT NULL REFERENCES orgs (id),
  project_id   text        NOT NULL,
  digest       bytea       NOT NULL,
  content_type text        NOT NULL CHECK (char_length(content_type) BETWEEN 3 AND 100),
  uploaded_at  timestamptz NOT NULL DEFAULT now(),
  uploaded_by  text        NOT NULL,
  PRIMARY KEY (project_id, digest),
  UNIQUE (org_id, project_id, digest),     -- the target of every org-carrying blob link (I5)
  FOREIGN KEY (org_id, digest)      REFERENCES blobs (org_id, digest),
  FOREIGN KEY (org_id, project_id)  REFERENCES projects (org_id, id),
  FOREIGN KEY (org_id, uploaded_by) REFERENCES users (org_id, id)
);
CREATE INDEX project_blobs_janitor ON project_blobs (uploaded_at);

-- ── design approvals and session links ───────────────────────────────────────
CREATE TABLE design_approvals (
  id              text        PRIMARY KEY CHECK (id ~ '^ap-[0-9a-f]{16}$'),
  org_id          text        NOT NULL REFERENCES orgs (id),
  project_id      text        NOT NULL,
  design_key      text        NOT NULL CHECK (design_key ~ '^design:[a-z]{2,4}-[0-9a-f]{4,32}@[1-9][0-9]{0,8}$'),
  canvas_id       text        NOT NULL,
  design_revision integer     NOT NULL CHECK (design_revision >= 1),
  review_digest   bytea       NOT NULL CHECK (octet_length(review_digest) = 32),
  brief_id        text        NOT NULL,
  brief_revision  integer     NOT NULL,
  brief_digest    bytea       NOT NULL,
  branch_ref      text        NOT NULL,
  manifest_digest bytea       NOT NULL,
  approved_at     timestamptz NOT NULL DEFAULT now(),
  approved_by     text        NOT NULL,
  UNIQUE (project_id, design_key),
  UNIQUE (org_id, project_id, design_key), -- the target of every org-carrying design link (I5)
  UNIQUE (org_id, id),
  CHECK (design_key = 'design:' || canvas_id || '@' || design_revision),
  FOREIGN KEY (org_id, project_id)  REFERENCES projects (org_id, id),
  FOREIGN KEY (org_id, approved_by) REFERENCES users (org_id, id),
  FOREIGN KEY (org_id, brief_id, brief_revision, brief_digest)
    REFERENCES brief_revisions (org_id, brief_id, revision, digest),
  FOREIGN KEY (org_id, project_id, manifest_digest)
    REFERENCES project_blobs (org_id, project_id, digest)
);

CREATE TABLE design_approval_blobs (
  org_id      text  NOT NULL REFERENCES orgs (id),
  approval_id text  NOT NULL,
  project_id  text  NOT NULL,
  digest      bytea NOT NULL,
  PRIMARY KEY (approval_id, digest),
  FOREIGN KEY (org_id, approval_id)        REFERENCES design_approvals (org_id, id),
  FOREIGN KEY (org_id, project_id, digest) REFERENCES project_blobs (org_id, project_id, digest)
);

CREATE TABLE session_links (
  id             text        PRIMARY KEY CHECK (id ~ '^ln-[0-9a-f]{16}$'),
  org_id         text        NOT NULL REFERENCES orgs (id),
  project_id     text        NOT NULL,
  design_key     text        NOT NULL,
  brief_id       text        NOT NULL,
  brief_revision integer     NOT NULL,
  session_id     text        NOT NULL CHECK (session_id ~ '^[a-z]{1,4}-[0-9a-f]{8,32}$'),
  branch_ref     text        NOT NULL,
  created_at     timestamptz NOT NULL DEFAULT now(),
  created_by     text        NOT NULL,
  UNIQUE (created_by, session_id, design_key),
  FOREIGN KEY (org_id, project_id) REFERENCES projects (org_id, id),
  FOREIGN KEY (org_id, created_by) REFERENCES users (org_id, id),
  FOREIGN KEY (org_id, project_id, design_key)
    REFERENCES design_approvals (org_id, project_id, design_key),
  FOREIGN KEY (org_id, brief_id, brief_revision)
    REFERENCES brief_revisions (org_id, brief_id, revision)
);

-- ── releases ─────────────────────────────────────────────────────────────────
CREATE TABLE releases (
  id                        text          PRIMARY KEY CHECK (id ~ '^rl-[0-9a-f]{16}$'),
  org_id                    text          NOT NULL REFERENCES orgs (id),
  project_id                text          NOT NULL,
  kind                      release_kind  NOT NULL,
  panacea_project           text          NOT NULL CHECK (panacea_project ~ '^[a-z0-9-]{1,63}$'),
  -- nullable (HA7-1, task 00): a never-deployed branch's run learns it at the join
  environment               text          CHECK (environment ~ '^[a-z0-9-]{1,63}$'),
  branch_ref                text          NOT NULL DEFAULT '',
  commit_sha                text          CHECK (commit_sha ~ '^[0-9a-f]{40}$'),
  source_deployment_id      text          CHECK (char_length(source_deployment_id) BETWEEN 1 AND 128),
  source_release_id         text,
  -- the pin: written by the reservation, never updated (guard_release)
  pin_reason                pin_reason    NOT NULL,
  pin_brief_id              text,
  pin_brief_revision        integer,
  pin_brief_digest          bytea,
  pin_head_revision_at_ship integer,
  design_key                text,
  state                     release_state NOT NULL DEFAULT 'reserved',
  pipeline_run              text          CHECK (pipeline_run ~ '^[a-z0-9.-]{1,253}$'),
  reserved_at               timestamptz   NOT NULL DEFAULT now(),
  reserved_by               text          NOT NULL,
  expires_at                timestamptz   NOT NULL,
  confirmed_at              timestamptz,
  abandoned_at              timestamptz,
  abandon_reason            text          CHECK (abandon_reason IN ('deploy_failed', 'cancelled')),
  abandon_message           text          CHECK (char_length(abandon_message) <= 500),
  expired_at                timestamptz,
  CHECK ((pin_reason IN ('ship_head', 'inherited')) = (pin_brief_id IS NOT NULL)),
  CHECK ((pin_brief_id IS NULL) = (pin_brief_revision IS NULL)),
  CHECK ((pin_brief_id IS NULL) = (pin_brief_digest IS NULL)),
  CHECK ((pin_brief_id IS NULL) = (pin_head_revision_at_ship IS NULL)),
  CHECK (pin_reason <> 'inherited' OR source_release_id IS NOT NULL),
  CHECK (kind <> 'rollback' OR (source_deployment_id IS NOT NULL AND pipeline_run IS NULL)),
  CHECK (kind <> 'run' OR state <> 'confirmed' OR pipeline_run IS NOT NULL),
  -- only a run with no source may lack its environment, in ANY state: confirm, abandon
  -- and expiry all happen before (or without) the join that names it (task 00, HA7-1)
  CHECK (environment IS NOT NULL OR (kind = 'run' AND source_deployment_id IS NULL)),
  CHECK ((state = 'confirmed') = (confirmed_at IS NOT NULL)),
  CHECK ((state = 'abandoned') = (abandoned_at IS NOT NULL AND abandon_reason IS NOT NULL)),
  CHECK ((state = 'expired')   = (expired_at IS NOT NULL)),
  UNIQUE (org_id, id),
  FOREIGN KEY (org_id, project_id)        REFERENCES projects (org_id, id),
  FOREIGN KEY (org_id, reserved_by)       REFERENCES users (org_id, id),
  FOREIGN KEY (org_id, source_release_id) REFERENCES releases (org_id, id),
  -- a NULL pin or design_key skips its check (MATCH SIMPLE); the CHECKs above keep the pin whole
  FOREIGN KEY (org_id, pin_brief_id, pin_brief_revision, pin_brief_digest)
    REFERENCES brief_revisions (org_id, brief_id, revision, digest),
  FOREIGN KEY (org_id, project_id, design_key)
    REFERENCES design_approvals (org_id, project_id, design_key)
);
CREATE UNIQUE INDEX releases_one_per_run ON releases (org_id, panacea_project, pipeline_run)
  WHERE pipeline_run IS NOT NULL;
CREATE INDEX releases_by_project ON releases (project_id, reserved_at DESC);
CREATE INDEX releases_to_expire ON releases (expires_at) WHERE state = 'reserved';

CREATE TABLE release_deployments (
  org_id          text        NOT NULL REFERENCES orgs (id),
  release_id      text        NOT NULL,
  panacea_project text        NOT NULL,
  deployment_id   text        NOT NULL CHECK (char_length(deployment_id) BETWEEN 1 AND 128),
  joined_at       timestamptz NOT NULL DEFAULT now(),
  joined_by       text        NOT NULL,
  PRIMARY KEY (release_id, deployment_id),
  UNIQUE (org_id, panacea_project, deployment_id),  -- a deployment belongs to one release
  FOREIGN KEY (org_id, release_id) REFERENCES releases (org_id, id),
  FOREIGN KEY (org_id, joined_by)  REFERENCES users (org_id, id)
);

-- ── evidence (phase 3, H-2) ──────────────────────────────────────────────────
CREATE TABLE evidence (
  id               text        PRIMARY KEY CHECK (id ~ '^ev-[0-9a-f]{16}$'),
  org_id           text        NOT NULL REFERENCES orgs (id),
  project_id       text        NOT NULL,
  repo_key         text        NOT NULL,          -- verbatim; survives the remote's removal
  brief_id         text        NOT NULL,
  brief_revision   integer     NOT NULL,
  criterion_id     text        NOT NULL CHECK (char_length(criterion_id) BETWEEN 1 AND 128),
  criterion_digest bytea       NOT NULL CHECK (octet_length(criterion_digest) = 32),
  commit_sha       text        NOT NULL CHECK (commit_sha ~ '^[0-9a-f]{40}$'),
  verdict          verdict     NOT NULL,
  because          text        NOT NULL CHECK (char_length(btrim(because)) BETWEEN 1 AND 2000),
  recorded_at      timestamptz NOT NULL DEFAULT now(),
  recorded_by      text        NOT NULL,
  FOREIGN KEY (org_id, project_id)  REFERENCES projects (org_id, id),
  FOREIGN KEY (org_id, recorded_by) REFERENCES users (org_id, id),
  FOREIGN KEY (org_id, brief_id, brief_revision)
    REFERENCES brief_revisions (org_id, brief_id, revision)
);
CREATE INDEX evidence_by_digest ON evidence (org_id, repo_key, criterion_digest, recorded_at);

-- ── audit, the change feed, idempotency ──────────────────────────────────────
CREATE TABLE audit_heads (
  org_id text   PRIMARY KEY REFERENCES orgs (id),
  seq    bigint NOT NULL DEFAULT 0 CHECK (seq >= 0),
  hash   bytea  NOT NULL DEFAULT decode(repeat('00', 32), 'hex') CHECK (octet_length(hash) = 32)
);

CREATE TABLE audit_events (
  org_id        text        NOT NULL REFERENCES orgs (id),
  seq           bigint      NOT NULL CHECK (seq >= 1),
  at            timestamptz NOT NULL,
  kind          text        NOT NULL CHECK (kind ~ '^[a-z]+(_[a-z]+)*\.[a-z]+(_[a-z]+)*$'),
  feed          boolean     NOT NULL,
  team_id       text,
  project_id    text,
  subject       text        NOT NULL,
  actor_user_id text,
  canonical     bytea       NOT NULL,             -- contract §3.6, with teamId (plan A10)
  prev_hash     bytea       NOT NULL CHECK (octet_length(prev_hash) = 32),
  hash          bytea       NOT NULL,
  PRIMARY KEY (org_id, seq),
  CONSTRAINT audit_events_hash_chains CHECK (hash = sha256(prev_hash || canonical)),
  FOREIGN KEY (org_id, actor_user_id) REFERENCES users (org_id, id)
);
CREATE INDEX audit_feed_by_project ON audit_events (org_id, project_id, seq) WHERE feed;
CREATE INDEX audit_feed_by_team    ON audit_events (org_id, team_id, seq)    WHERE feed;

CREATE TABLE idempotency_keys (
  org_id         text        NOT NULL REFERENCES orgs (id),
  user_id        text        NOT NULL,
  key            text        NOT NULL CHECK (key ~ '^[A-Za-z0-9_-]{1,64}$'),
  method         text        NOT NULL,
  path           text        NOT NULL,
  request_sha256 bytea       NOT NULL,
  status         smallint    NOT NULL DEFAULT 0,   -- 0 while the first request is in flight
  response       bytea,
  created_at     timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, key),
  FOREIGN KEY (org_id, user_id) REFERENCES users (org_id, id)
);
CREATE INDEX idempotency_keys_janitor ON idempotency_keys (created_at);
```

```sql
-- 0001_init.sql, continued: row-level security and the grants. Task 07b appends the guards
-- and the owner functions after the grants.
-- ── row-level security (contract §10.3) ──────────────────────────────────────
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['users', 'teams', 'team_members', 'team_invites', 'projects',
      'project_remotes', 'briefs', 'brief_revisions', 'blobs', 'project_blobs', 'design_approvals',
      'design_approval_blobs', 'session_links', 'releases', 'release_deployments', 'evidence',
      'audit_heads', 'audit_events', 'idempotency_keys'] LOOP
    EXECUTE format('ALTER TABLE hub.%I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('CREATE POLICY org_isolation ON hub.%I TO hub_app
                    USING (org_id = current_setting(''hub.org_id'', true))
                    WITH CHECK (org_id = current_setting(''hub.org_id'', true))', t);
  END LOOP;
END $$;
-- M10 (review): orgs is keyed by id, not org_id, so it sits outside the loop. hub_app may
-- only SELECT it (no INSERT or UPDATE grant), so the policy needs no WITH CHECK. Task 07b's
-- owner functions read and write orgs as hub_owner, the table's owner, which this policy
-- does not bind (row-level security is ENABLEd, never FORCEd), so ensure_org and
-- resolve_principal are unaffected. The janitor and hub verify list orgs through 07b's
-- hub.org_ids() (A18).
ALTER TABLE hub.orgs ENABLE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON hub.orgs TO hub_app
  USING (id = current_setting('hub.org_id', true));

-- ── grants: what hub_app may do, and nothing else (contract §10.5) ───────────
GRANT USAGE ON SCHEMA hub TO hub_app;
GRANT SELECT ON ALL TABLES IN SCHEMA hub TO hub_app;

GRANT INSERT ON teams, team_members, team_invites, projects, project_remotes, briefs,
                brief_revisions, blobs, project_blobs, design_approvals, design_approval_blobs,
                session_links, releases, release_deployments, evidence, audit_events,
                idempotency_keys TO hub_app;

GRANT UPDATE (name)                                            ON teams            TO hub_app;
GRANT UPDATE (role), DELETE                                    ON team_members     TO hub_app;
GRANT UPDATE (state, bound_user_id, bound_at, revoked_at)      ON team_invites     TO hub_app;
GRANT UPDATE (name)                                            ON projects         TO hub_app;
GRANT DELETE                                                   ON project_remotes  TO hub_app;
GRANT UPDATE (branch_ref, head_revision, latest_complete_revision, archived_at, updated_at)
                                                               ON briefs           TO hub_app;
GRANT UPDATE (state, pipeline_run, confirmed_at, abandoned_at, abandon_reason, abandon_message,
              expired_at, environment)                         ON releases         TO hub_app;  -- environment: HA7-1's join
GRANT DELETE                                                   ON blobs, project_blobs TO hub_app;
GRANT UPDATE (status, response), DELETE                        ON idempotency_keys TO hub_app;
-- No UPDATE or DELETE on: orgs, users, brief_revisions, design_approvals,
-- design_approval_blobs, session_links, release_deployments, evidence, audit_events,
-- audit_heads. audit_heads moves only inside audit_append and is locked only through
-- lock_audit_head, both SECURITY DEFINER and both task 07b's (M11, A19).
-- A later migration that adds a table grants on it explicitly: there are no default privileges.
```

**Two Postgres facts the tests pin rather than trust** (both from knowledge, unverified here):
`current_setting('hub.org_id', true)` returns `NULL` in a session that never set it, but `''`
after a transaction-local `set_config` has committed on the same pooled connection. Either way no
`org-…` id equals it, so policies match nothing (`TestAnUnsetOrgSeesNothingEvenOnAReusedConnection`).
And a table's owner is not bound by a policy that is `ENABLE`d but not `FORCE`d, which is how the
seed helper writes `orgs`, `users` and `audit_heads`, and how task 07b's owner functions work.
Task 07b pins the others.

#### Tests to write first — `schema_integration_test.go` (`//go:build integration`)

Each runs on `hubtest.Migrated(t)`. The seed helper lives in this file, and task 07b's tests use
it too. It writes orgs, users and `audit_heads` rows by plain `INSERT` as `hub_owner`, since
`hub_app` may not insert them and task 07b's `ensure_org` and `resolve_principal` do not exist
yet. Every other row goes in by plain `INSERT` as `hub_app` with `hub.org_id` set, **in the order
the Hub writes**, so task 07b's guards accept the same seed: audit events chained from each org's
head (`seq` 1, 2, … with `prev_hash` the previous `hash`), and a brief's head moved one step only
after its revision exists. "Refused" means the statement returns a `*pgconn.PgError` whose `Code`
is the one named.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestMigrationAppliesAndReappliesClean` | `Apply` → `[1]`; again → `[]`; `Current` = `Latest()` = 1 | a migration that is not idempotent to the runner |
| `TestNamedConstraintsExist` | `pg_constraint` holds `users_org_id_entra_oid_key`, `brief_revisions_canonical_hashes`, `brief_revisions_erased_iff_no_content`, `audit_events_hash_chains` | `resolve_principal` depending on a default name |
| `TestHubAppCannotUpdateOrDeleteImmutableTables` | as `hub_app`, `UPDATE`/`DELETE` on `brief_revisions`, `design_approvals`, `session_links`, `release_deployments`, `evidence`, `audit_events`, `orgs`, `users`, `audit_heads` → `42501` | a grant widened by accident (G4's grant layer); for `audit_heads`, the review's M11 grant `UPDATE (seq, hash)` put back |
| `TestARevisionMustHashToItsDigest` | `INSERT` a revision whose `digest` is not `sha256(canonical)` → `23514` | a digest the Go side computed wrong, stored anyway |
| `TestANullEnvironmentIsARunsOnly` | `INSERT` a run with `environment` NULL and no source → ok; a rollback, or a run with `source_deployment_id`, with NULL → `23514` (check) | a rollback recorded with no environment (HA7-1) |
| `TestRowLevelSecuritySeparatesOrgs` | two orgs with a user each; `set_config('hub.org_id', A, true)`; `SELECT` from each of the 20 tables sees only A: the loop's 19, and `orgs`, where `SELECT id, display_name FROM hub.orgs` returns A's one row (review M10); `INSERT` of an org-B row → `42501` (WITH CHECK) | a table missing from the RLS loop (T3); `orgs` without its policy, which returns both orgs' ids and names |
| `TestAnUnsetOrgSeesNothingEvenOnAReusedConnection` | on one connection: a transaction that sets org A and commits; then a new transaction that sets nothing → zero rows from `users` and zero from `orgs` | a pooled connection carrying one request's org into the next |
| `TestEveryForeignKeyCarriesTheOrg` | from `pg_constraint` (`contype = 'f'`, schema `hub`), every foreign key whose `confrelid` is not `hub.orgs` has `org_id` in `conkey` and `org_id` in `confkey` **at the same position**; and every such key's target is a `UNIQUE` or primary key that holds `org_id`. The test prints the offending constraint's name (review I5) | a single-column link added by a later edit (`team_id REFERENCES teams (id)`), which Postgres checks without row-level security; a key that carries `org_id` but pairs it with another column |
| `TestACrossOrgReferenceIsRefused` | two orgs A and B, each seeded with one parent of every kind (user, team, project, brief, revision, blob, project blob, approval, release). As `hub_app` with `hub.org_id` = A, **one `INSERT` per foreign key** of the catalog query above: an org-A row whose every column is A's, except the one link under test, which names B's parent → `23503` (`foreign_key_violation`). The same row with A's parent → ok, so the refusal is the key and nothing else. The set of constraint names exercised equals the catalog's set, so a new foreign key with no case goes red (review I5) | a cross-org link that row-level security lets through: `INSERT INTO projects (org_id, team_id, …) VALUES (A, <B's team>, …)`, which the single-column key accepted |
| `TestHubAppGrantsAreExactlyTheList` | `information_schema.role_table_grants` and `column_privileges` for `hub_app` equal §10.5's list, table by table, column by column. The list is this part's grants, so `audit_heads` is `SELECT` only: no `UPDATE` on any column (review M11, A19), and `orgs` is `SELECT` only | a grant nobody reviewed; `GRANT UPDATE (seq, hash) ON audit_heads` put back, which lets a compromised Hub move a head outside `audit_append` |

#### Steps

- [ ] **Step 1:** Re-read contract §10.1–§10.7. If it changed after this part was written, the
  contract's words win except for the three named constraints above. A16 and A18 are already in
  the contract (task 44a). A contract that still shows a single-column key, `orgs` without a
  policy, or `UPDATE (seq, hash)` on `audit_heads` for `hub_app` is stale, and this SQL wins.
- [ ] **Step 2:** The tests first. Run them and see them fail, since no table exists. Then
  `0001_init.sql` exactly as above, `embed.go`, and `hubtest.Migrated`.
- [ ] **Step 3:** Run against the task-06 container. Fix the SQL until every test passes. **Any
  change to the SQL beyond the three names is recorded in the commit message**, and in the
  contract by task 44b (a build departure).
- [ ] **Step 4:** Commit by pathspec, the four files. Do not migrate a persistent database
  (`hub-pg-dev`) from this commit: task 07b edits the same migration, and task 06's runner then
  refuses it (`ErrEdited`). The orchestrator merges 07a and 07b together.

#### Acceptance

```bash
go test -tags integration ./control-plane/internal/hub/migrate/ -race -count=1 -v 2>&1 | grep -c "^--- PASS"   # 20 (10 from task 06 + 10)
go build ./control-plane/...                                                                               # ok
grep -c "CREATE TABLE" control-plane/internal/hub/migrate/sql/0001_init.sql                                # 20
grep -c "CREATE POLICY" control-plane/internal/hub/migrate/sql/0001_init.sql                               # 2 (the loop's, and orgs')
```

#### What this task does not claim

- That this schema runs on **Azure** Postgres Flexible Server 16 unchanged; it is tested on
  `postgres:16-alpine`. Phase 8's first apply is the check (ADR-H1 §11).
- Any guard: a revision, a head, a pin, the append-only tables and the audit chain are held by
  grants alone until task 07b. Until then the owner can change any of them, and so can `hub_app`
  where it holds a column grant.

