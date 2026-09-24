### Task 07: `sql/0001_init.sql` — the complete first schema, and a database test for every guard

**Lane A. Depends on: 06. About 4 hours (3 before review I5's cross-org key tests). Contract task H0 (and the database half of G4, G5).**

The contract's §10.2–§10.5 is the schema, written from knowledge and never applied (contract §15).
This task applies it and proves, **in the database and without any Go handler**, each guarantee of
§10.7's "Database" column: a revision never changes, a head moves one step onto a real revision, a
pin never moves, append-only tables are append-only, the audit chain is gapless and hashed, one
org never sees another's rows, **and no row links to another org's row**.

**One correction that changes behaviour**, folded into the contract by task 44a before this task
runs, so the contract and this SQL agree:
- **A16 (review I5):** every foreign key except the one to `orgs` carries `org_id`, onto a
  `UNIQUE (org_id, …)` target. Row-level security checks only a row's own `org_id`, and Postgres
  checks a foreign key without it, so a single-column key let an org-A row name an org-B parent.

**A17 (review I6) leaves this SQL as the contract has it.** Route 26's stored response kept the
text for 24 hours. The fix is that it no longer stores the text at all: tasks 14 and 19 store it
with every `content` null and rebuild the content on replay from `brief_revisions`. So
`erase_brief` deletes nothing from `idempotency_keys`, and `TestEraseBriefLeavesEveryStoredResponse`
pins that (re-review b06 N1).

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
-- 0001_init.sql, continued: the guards, row-level security, the owner functions, the grants.
-- ── append-only tables ───────────────────────────────────────────────────────
CREATE FUNCTION forbid_change() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'hub: % is append-only', TG_TABLE_NAME USING ERRCODE = 'insufficient_privilege';
END $$;

DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['design_approvals', 'design_approval_blobs', 'session_links',
                           'release_deployments', 'evidence', 'audit_events'] LOOP
    EXECUTE format('CREATE TRIGGER %I BEFORE UPDATE OR DELETE ON hub.%I
                    FOR EACH ROW EXECUTE FUNCTION hub.forbid_change()', t || '_append_only', t);
    EXECUTE format('CREATE TRIGGER %I BEFORE TRUNCATE ON hub.%I
                    FOR EACH STATEMENT EXECUTE FUNCTION hub.forbid_change()', t || '_no_truncate', t);
  END LOOP;
END $$;

-- ── a brief revision changes once, by erasure, and only inside hub.erase_brief ─
CREATE FUNCTION guard_revision() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    RAISE EXCEPTION 'hub: brief revisions are never deleted' USING ERRCODE = 'insufficient_privilege';
  END IF;
  IF current_user <> 'hub_owner'
     OR OLD.canonical IS NULL OR NEW.canonical IS NOT NULL OR NEW.erased_at IS NULL
     OR (NEW.org_id, NEW.brief_id, NEW.revision, NEW.project_id, NEW.scope, NEW.branch_ref,
         NEW.status, NEW.digest, NEW.saved_at, NEW.saved_by)
        IS DISTINCT FROM
        (OLD.org_id, OLD.brief_id, OLD.revision, OLD.project_id, OLD.scope, OLD.branch_ref,
         OLD.status, OLD.digest, OLD.saved_at, OLD.saved_by)
  THEN
    RAISE EXCEPTION 'hub: a brief revision is immutable' USING ERRCODE = 'insufficient_privilege';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER brief_revisions_guard BEFORE UPDATE OR DELETE ON brief_revisions
  FOR EACH ROW EXECUTE FUNCTION guard_revision();
CREATE TRIGGER brief_revisions_no_truncate BEFORE TRUNCATE ON brief_revisions
  FOR EACH STATEMENT EXECUTE FUNCTION forbid_change();

-- ── a head moves forward one revision at a time, onto a revision that exists ──
CREATE FUNCTION guard_brief() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    RAISE EXCEPTION 'hub: briefs are archived, never deleted' USING ERRCODE = 'insufficient_privilege';
  END IF;
  IF (NEW.id, NEW.org_id, NEW.project_id, NEW.scope, NEW.created_at, NEW.created_by)
       IS DISTINCT FROM (OLD.id, OLD.org_id, OLD.project_id, OLD.scope, OLD.created_at, OLD.created_by)
     OR NEW.head_revision NOT IN (OLD.head_revision, OLD.head_revision + 1)
     OR NEW.latest_complete_revision < OLD.latest_complete_revision
     OR (OLD.archived_at IS NOT NULL AND (NEW.archived_at IS DISTINCT FROM OLD.archived_at
                                          OR NEW.head_revision <> OLD.head_revision
                                          OR NEW.branch_ref <> OLD.branch_ref))
     OR (OLD.erased_at IS NOT NULL AND NEW.erased_at IS DISTINCT FROM OLD.erased_at)
     OR (NEW.head_revision <> OLD.head_revision AND NOT EXISTS
           (SELECT 1 FROM hub.brief_revisions r WHERE r.brief_id = NEW.id AND r.revision = NEW.head_revision))
  THEN
    RAISE EXCEPTION 'hub: illegal change to brief %', OLD.id USING ERRCODE = 'insufficient_privilege';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER briefs_guard BEFORE UPDATE OR DELETE ON briefs
  FOR EACH ROW EXECUTE FUNCTION guard_brief();

-- ── a release's pin and identity never change; only a reserved release moves ─
CREATE FUNCTION guard_release() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    RAISE EXCEPTION 'hub: releases are never deleted' USING ERRCODE = 'insufficient_privilege';
  END IF;
  IF (NEW.id, NEW.org_id, NEW.project_id, NEW.kind, NEW.panacea_project,
      NEW.branch_ref, NEW.commit_sha, NEW.source_deployment_id, NEW.source_release_id,
      NEW.pin_reason, NEW.pin_brief_id, NEW.pin_brief_revision, NEW.pin_brief_digest,
      NEW.pin_head_revision_at_ship, NEW.design_key, NEW.reserved_at, NEW.reserved_by, NEW.expires_at)
     IS DISTINCT FROM
     (OLD.id, OLD.org_id, OLD.project_id, OLD.kind, OLD.panacea_project,
      OLD.branch_ref, OLD.commit_sha, OLD.source_deployment_id, OLD.source_release_id,
      OLD.pin_reason, OLD.pin_brief_id, OLD.pin_brief_revision, OLD.pin_brief_digest,
      OLD.pin_head_revision_at_ship, OLD.design_key, OLD.reserved_at, OLD.reserved_by, OLD.expires_at)
  THEN
    RAISE EXCEPTION 'hub: a release''s pin never changes' USING ERRCODE = 'insufficient_privilege';
  END IF;
  -- the environment is written once (HA7-1, task 00): a value never changes or returns
  -- to NULL; NULL -> a value only while reserved or confirmed (the join is confirmed-only)
  IF NEW.environment IS DISTINCT FROM OLD.environment AND
     (OLD.environment IS NOT NULL OR OLD.state NOT IN ('reserved', 'confirmed')) THEN
    RAISE EXCEPTION 'hub: release %''s environment is set once', OLD.id USING ERRCODE = 'insufficient_privilege';
  END IF;
  -- a non-reserved release takes no change but that one
  IF OLD.state <> 'reserved' AND
     (NEW.state, NEW.pipeline_run, NEW.confirmed_at, NEW.abandoned_at, NEW.abandon_reason,
      NEW.abandon_message, NEW.expired_at)
     IS DISTINCT FROM
     (OLD.state, OLD.pipeline_run, OLD.confirmed_at, OLD.abandoned_at, OLD.abandon_reason,
      OLD.abandon_message, OLD.expired_at)
  THEN
    RAISE EXCEPTION 'hub: release % is %, not reserved', OLD.id, OLD.state USING ERRCODE = 'insufficient_privilege';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER releases_guard BEFORE UPDATE OR DELETE ON releases
  FOR EACH ROW EXECUTE FUNCTION guard_release();

-- ── the audit chain: gapless, in order, and it advances its own head ─────────
-- M11 (review), A19: SECURITY DEFINER, so the head moves only here. hub_app holds no UPDATE
-- on audit_heads (grants below), so a compromised Hub cannot move a head by a direct
-- UPDATE. As the owner this function is not bound by row-level security, so it checks the
-- org itself, FIRST, before it locks anything: without that check an org-A session could
-- lock, and for a moment move, org B's head. WITH CHECK would still refuse the row, but only
-- after BEFORE triggers have run (from knowledge; TestAuditAppendChecksTheOrgFirst pins the
-- lock half).
CREATE FUNCTION audit_append() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path = hub, pg_temp AS $$
DECLARE h hub.audit_heads%ROWTYPE;
BEGIN
  IF NEW.org_id IS DISTINCT FROM current_setting('hub.org_id', true) THEN
    RAISE EXCEPTION 'hub: audit event for org %, not this session''s', NEW.org_id
      USING ERRCODE = 'insufficient_privilege';
  END IF;
  SELECT * INTO h FROM hub.audit_heads WHERE org_id = NEW.org_id FOR UPDATE;
  IF NOT FOUND OR NEW.seq <> h.seq + 1 OR NEW.prev_hash <> h.hash THEN
    RAISE EXCEPTION 'hub: audit event % out of order', NEW.seq USING ERRCODE = 'serialization_failure';
  END IF;
  UPDATE hub.audit_heads SET seq = NEW.seq, hash = NEW.hash WHERE org_id = NEW.org_id;
  RETURN NEW;
END $$;
CREATE TRIGGER audit_events_append BEFORE INSERT ON audit_events
  FOR EACH ROW EXECUTE FUNCTION audit_append();

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
-- only SELECT it (no INSERT or UPDATE grant), so the policy needs no WITH CHECK. The owner
-- functions below read and write orgs as hub_owner, the table's owner, which this policy
-- does not bind (row-level security is ENABLEd, never FORCEd), so ensure_org and
-- resolve_principal are unaffected.
ALTER TABLE hub.orgs ENABLE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON hub.orgs TO hub_app
  USING (id = current_setting('hub.org_id', true));

-- ── the five functions hub_app calls that run as the owner (contract §10.4; ─────
-- ── the contract's three, and A18's org_ids and A19's lock_audit_head) ──────────
CREATE FUNCTION ensure_org(p_tenant uuid, p_name text, p_id text) RETURNS text
LANGUAGE plpgsql SECURITY DEFINER SET search_path = hub, pg_temp AS $$
DECLARE v_id text;
BEGIN
  INSERT INTO orgs (id, entra_tenant_id, display_name) VALUES (p_id, p_tenant, p_name)
  ON CONFLICT (entra_tenant_id) DO UPDATE SET display_name = EXCLUDED.display_name
  RETURNING id INTO v_id;
  INSERT INTO audit_heads (org_id) VALUES (v_id) ON CONFLICT (org_id) DO NOTHING;
  RETURN v_id;
END $$;

CREATE FUNCTION resolve_principal(p_tenant uuid, p_oid uuid, p_name text, p_email text,
                                  p_new_user_id text)
RETURNS TABLE (org_id text, user_id text, created boolean)
LANGUAGE plpgsql SECURITY DEFINER SET search_path = hub, pg_temp AS $$
DECLARE v_org text; v_user text; v_name text := left(p_name, 256); v_email text := lower(left(p_email, 254));
BEGIN
  SELECT o.id INTO v_org FROM orgs o WHERE o.entra_tenant_id = p_tenant;
  IF v_org IS NULL THEN RETURN; END IF;
  INSERT INTO users (id, org_id, entra_oid, display_name, email)
  VALUES (p_new_user_id, v_org, p_oid, v_name, v_email)
  ON CONFLICT ON CONSTRAINT users_org_id_entra_oid_key DO NOTHING
  RETURNING users.id INTO v_user;
  IF v_user IS NOT NULL THEN
    RETURN QUERY SELECT v_org, v_user, true; RETURN;
  END IF;
  UPDATE users u SET display_name = v_name, email = v_email, last_seen_at = now()
   WHERE u.org_id = v_org AND u.entra_oid = p_oid
     AND (u.display_name IS DISTINCT FROM v_name OR u.email IS DISTINCT FROM v_email
          OR u.last_seen_at < now() - interval '1 hour');
  SELECT u.id INTO v_user FROM users u WHERE u.org_id = v_org AND u.entra_oid = p_oid;
  RETURN QUERY SELECT v_org, v_user, false;
END $$;

CREATE FUNCTION erase_brief(p_brief text) RETURNS integer
LANGUAGE plpgsql SECURITY DEFINER SET search_path = hub, pg_temp AS $$
DECLARE v_org text := current_setting('hub.org_id', true); v_n integer;
BEGIN
  IF v_org IS NULL OR NOT EXISTS (SELECT 1 FROM briefs b WHERE b.id = p_brief AND b.org_id = v_org) THEN
    RAISE EXCEPTION 'hub: no brief % in this org', p_brief USING ERRCODE = 'no_data_found';
  END IF;
  UPDATE brief_revisions SET canonical = NULL, erased_at = now()
   WHERE brief_id = p_brief AND canonical IS NOT NULL;
  GET DIAGNOSTICS v_n = ROW_COUNT;
  UPDATE briefs SET archived_at = coalesce(archived_at, now()),
                    erased_at   = coalesce(erased_at, now()),
                    updated_at  = now()
   WHERE id = p_brief;
  -- I6 (review), A17: idempotency_keys is NOT touched. No stored response holds brief text:
  -- route 26 stores its reply with every content null and rebuilds content on replay from
  -- these rows (tasks 14, 19), so a replay after this call answers content null, as /v1/refs
  -- does. A purge here would delete the replays of approvals, evidence, releases, co-batched
  -- briefs and this erase itself (re-review b06 N1), and turn their retries into second writes.
  RETURN v_n;
END $$;

-- M11 (review), A19: the org's write lock (contract §10.6's first statement after set_config).
-- SELECT … FOR UPDATE needs UPDATE privilege on at least one column of the table (from
-- knowledge; TestTheHeadLockNeedsTheFunction pins it), so with hub_app's UPDATE on
-- audit_heads revoked the lock is taken here, as the owner, and only on the session's org:
-- there is no parameter to name another. The row lock belongs to the calling transaction.
CREATE FUNCTION lock_audit_head() RETURNS TABLE (seq bigint, hash bytea)
LANGUAGE plpgsql SECURITY DEFINER SET search_path = hub, pg_temp AS $$
DECLARE v_org text := current_setting('hub.org_id', true);
BEGIN
  RETURN QUERY SELECT h.seq, h.hash FROM audit_heads h WHERE h.org_id = v_org FOR UPDATE;
  IF NOT FOUND THEN
    RAISE EXCEPTION 'hub: no audit head for org %', v_org USING ERRCODE = 'no_data_found';
  END IF;
END $$;

-- M10 (review), A18: the org ids, and nothing else, for the two callers that must visit
-- every org before any hub.org_id is set: the janitor (task 24) and hub verify (tasks 20,
-- 27), both on hub_app's connection. With orgs under row-level security, a plain SELECT
-- there sees no org at all, and the janitor would sweep nothing and hub verify would pass
-- on an empty walk. No name and no tenant id leaves this function.
CREATE FUNCTION org_ids() RETURNS SETOF text
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = hub, pg_temp AS $$
  SELECT id FROM orgs ORDER BY id
$$;

REVOKE ALL ON FUNCTION ensure_org(uuid, text, text), resolve_principal(uuid, uuid, text, text, text),
                       erase_brief(text), lock_audit_head(), org_ids() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ensure_org(uuid, text, text), resolve_principal(uuid, uuid, text, text, text),
                          erase_brief(text), lock_audit_head(), org_ids() TO hub_app;

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
-- lock_audit_head, both SECURITY DEFINER (M11, A19).
-- A later migration that adds a table grants on it explicitly: there are no default privileges.
```

**Two Postgres facts the tests pin rather than trust** (both from knowledge, unverified here):
`current_setting('hub.org_id', true)` returns `NULL` in a session that never set it, but `''`
after a transaction-local `set_config` has committed on the same pooled connection — either way
no `org-…` id equals it, so policies match nothing (`TestAnUnsetOrgSeesNothingEvenOnAReusedConnection`);
and inside a `SECURITY DEFINER` function `current_user` is the owner, which is what lets
`erase_brief` pass `guard_revision` (`TestOnlyEraseBriefCanEraseARevision`).

#### Tests to write first — `schema_integration_test.go` (`//go:build integration`)

Each runs on `hubtest.Migrated(t)`. Seeding (an org via `ensure_org`, users via
`resolve_principal`, rows by plain `INSERT` as `hub_app` with `hub.org_id` set) is a helper in
the test file. "Refused" means the statement returns a `*pgconn.PgError` whose `Code` is the one
named.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestMigrationAppliesAndReappliesClean` | `Apply` → `[1]`; again → `[]`; `Current` = `Latest()` = 1 | a migration that is not idempotent to the runner |
| `TestNamedConstraintsExist` | `pg_constraint` holds `users_org_id_entra_oid_key`, `brief_revisions_canonical_hashes`, `brief_revisions_erased_iff_no_content`, `audit_events_hash_chains` | `resolve_principal` depending on a default name |
| `TestHubAppCannotUpdateOrDeleteImmutableTables` | as `hub_app`, `UPDATE`/`DELETE` on `brief_revisions`, `design_approvals`, `session_links`, `release_deployments`, `evidence`, `audit_events`, `orgs`, `users`, `audit_heads` → `42501` | a grant widened by accident (G4's grant layer); for `audit_heads`, the review's M11 grant `UPDATE (seq, hash)` put back |
| `TestAppendOnlyTriggersHoldEvenForTheOwner` | as `hub_owner`, `UPDATE`, `DELETE`, `TRUNCATE` on each of the six append-only tables → `42501` from `forbid_change` | append-only enforced by grants alone |
| `TestARevisionMustHashToItsDigest` | `INSERT` a revision whose `digest` is not `sha256(canonical)` → `23514` | a digest the Go side computed wrong, stored anyway |
| `TestOnlyEraseBriefCanEraseARevision` | as the owner outside the function, an erasure-shaped `UPDATE` passes `guard_revision` only because `current_user` is `hub_owner`; any other column change → `42501`; `erase_brief` as `hub_app` erases and returns the count | the revision guard trusting the caller |
| `TestAHeadMovesOneStepOntoARealRevision` | `head_revision` +2 → `42501`; +1 with no revision row → `42501`; +1 after inserting that revision → ok; `latest_complete_revision` backwards → `42501` | a head pointing at nothing (P3) |
| `TestAnArchivedBriefIsFrozen` | after `archived_at` is set: head, ref and `archived_at` changes → `42501`; a new live brief on the same ref → ok | branch deletion that frees the ref but not the old brief |
| `TestAPinNeverMoves` | as `hub_app`, `UPDATE releases SET pin_brief_revision = …` → `42501` (column grant); as the owner → `42501` (trigger) | P7 held by one layer only |
| `TestOnlyAReservedReleaseMoves` | confirm a release; any further `UPDATE` of `state` → `42501` | a confirmed release re-confirmed with another run |
| `TestANullEnvironmentIsARunsOnly` | `INSERT` a run with `environment` NULL and no source → ok; a rollback, or a run with `source_deployment_id`, with NULL → `23514` (check) | a rollback recorded with no environment (HA7-1) |
| `TestANullEnvironmentReleaseConfirmsAbandonsAndExpires` | three NULL-environment runs: `UPDATE` to `confirmed` (with `pipeline_run`), to `abandoned`, to `expired` → each ok, `environment` still NULL | the review's `state = 'reserved'` check, which refuses all three (task 00) |
| `TestTheEnvironmentIsSetOnce` | a NULL-environment run: `SET environment = 'feat-x'` while `confirmed` → ok; then `= 'feat-y'` → `42501`; then `= NULL` → `42501`; a NULL one `abandoned` then `SET environment` → `42501`; a non-NULL reserved one `SET environment` → `42501` | a join that re-points a release at another environment, or a deleted one |
| `TestReleaseGuardCoversEveryColumn` | reads **the installed function**, not a copy: `SELECT pg_get_functiondef('hub.guard_release'::regproc)`. From that text it takes every `( NEW.a, NEW.b, … ) IS DISTINCT FROM ( OLD.a, OLD.b, … )` pair and expects exactly two. The first is the immutable tuple and the second the mutable one. In each pair the `NEW` list equals the `OLD` list, name for name and in order. `NEW.environment IS DISTINCT FROM OLD.environment` appears exactly once, outside both. Then, against `information_schema.columns` for `hub.releases`, every column appears **exactly once** across immutable, mutable and `{environment}`, and no name appears there that is not a column. Last, mutable ∪ `{environment}` = `hub_app`'s `UPDATE` columns (`information_schema.column_privileges`) | a column `x` added to `releases`, and to any Go list, but not to the trigger's tuple: the test goes red on the missing column, where a copied list would stay green. `x` would otherwise stay mutable after confirmation for the owner role, which bypasses the grant. The old `NEW IS DISTINCT FROM OLD` caught this; the tuples do not. It also kills a `NEW`/`OLD` mismatch (`NEW.pin_brief_digest` beside `OLD.pin_brief_id`) |
| `TestTheAuditChainIsGaplessAndHashed` | insert seq 1 then 2 correctly → ok and `audit_heads` advanced; seq 3 with the wrong `prev_hash`, or seq 4 skipping 3 → `40001`; a `hash` not equal to `sha256(prev_hash ‖ canonical)` → `23514` | a chain the Go side can fork (G5) |
| `TestRowLevelSecuritySeparatesOrgs` | two orgs with a user each; `set_config('hub.org_id', A, true)`; `SELECT` from each of the 20 tables sees only A: the loop's 19, and `orgs`, where `SELECT id, display_name FROM hub.orgs` returns A's one row (review M10); `INSERT` of an org-B row → `42501` (WITH CHECK) | a table missing from the RLS loop (T3); `orgs` without its policy, which returns both orgs' ids and names |
| `TestAnUnsetOrgSeesNothingEvenOnAReusedConnection` | on one connection: a transaction that sets org A and commits; then a new transaction that sets nothing → zero rows from `users` and zero from `orgs` | a pooled connection carrying one request's org into the next |
| `TestResolvePrincipal` | an unknown tenant → no row; a new oid → `created = true`; again → `created = false`, same id; a changed name → refreshed | provisioning twice, or never refreshing the display name |
| `TestEnsureOrgIsIdempotent` | twice with two fresh ids → the first id both times; one `audit_heads` row | a second org row per restart |
| `TestEraseBriefChecksTheOrg` | with `hub.org_id` = B, erasing A's brief → `P0002` (`no_data_found`), and A's revisions keep their content | the owner function bypassing RLS without its own check (T3's residual) |
| `TestEveryForeignKeyCarriesTheOrg` | from `pg_constraint` (`contype = 'f'`, schema `hub`), every foreign key whose `confrelid` is not `hub.orgs` has `org_id` in `conkey` and `org_id` in `confkey` **at the same position**; and every such key's target is a `UNIQUE` or primary key that holds `org_id`. The test prints the offending constraint's name (review I5) | a single-column link added by a later edit (`team_id REFERENCES teams (id)`), which Postgres checks without row-level security; a key that carries `org_id` but pairs it with another column |
| `TestACrossOrgReferenceIsRefused` | two orgs A and B, each seeded with one parent of every kind (user, team, project, brief, revision, blob, project blob, approval, release). As `hub_app` with `hub.org_id` = A, **one `INSERT` per foreign key** of the catalog query above: an org-A row whose every column is A's, except the one link under test, which names B's parent → `23503` (`foreign_key_violation`). The same row with A's parent → ok, so the refusal is the key and nothing else. The set of constraint names exercised equals the catalog's set, so a new foreign key with no case goes red (review I5) | a cross-org link that row-level security lets through: `INSERT INTO projects (org_id, team_id, …) VALUES (A, <B's team>, …)`, which the single-column key accepted |
| `TestEraseBriefLeavesEveryStoredResponse` | org A, one project, briefs A1 and A2. Four `idempotency_keys` rows, each byte-copied before the call: r1 is route 26's stored form of a batch that wrote A1 **and** A2 together (every `content` null, as task 19 stores it); r2 is a stored route-30 `DesignApproval` body naming `brief:<A1>@1` (text-free); r3's `response` is `NULL` (in flight); r4 is **in org B**, and its response contains A1's id. `erase_brief(A1)` with `hub.org_id` = A → all four rows kept, byte for byte (review I6, re-review b06 N1) | a purge by the brief id, which deletes r1 and r2: A2's co-batched retry becomes 409 `brief_archived` and the approval's retry re-runs into 422 `key_unresolvable`; a purge by `path`, which deletes r1; a purge without `org_id = v_org`, which also deletes r4 |
| `TestHubAppGrantsAreExactlyTheList` | `information_schema.role_table_grants` and `column_privileges` for `hub_app` equal §10.5's list, table by table, column by column. The list is this part's grants, so `audit_heads` is `SELECT` only: no `UPDATE` on any column (review M11, A19), and `orgs` is `SELECT` only | a grant nobody reviewed; `GRANT UPDATE (seq, hash) ON audit_heads` put back, which lets a compromised Hub move a head outside `audit_append` |
| `TestAuditAppendChecksTheOrgFirst` | orgs A and B. Connection 1, as `hub_app`: `BEGIN`, `hub.org_id` = A, `INSERT` an event with `org_id` = B, correctly chained on B's head → `42501`; the transaction is left open (aborted, not rolled back). Connection 2, as `hub_owner`: `SELECT 1 FROM hub.audit_heads WHERE org_id = B FOR UPDATE NOWAIT` → ok, and B's head is unchanged. Then connection 1 rolls back (review M11, A19) | an `audit_append` without its org check: it runs as the owner, bypasses row-level security, locks B's head before `WITH CHECK` refuses the row, and connection 2 gets `55P03` (`lock_not_available`) |
| `TestTheHeadLockNeedsTheFunction` | as `hub_app` with `hub.org_id` = A: `SELECT seq, hash FROM hub.audit_heads WHERE org_id = A FOR UPDATE` → `42501` (no `UPDATE` privilege); `SELECT seq, hash FROM hub.lock_audit_head()` → A's head; then, on a second connection as the owner, the same row `FOR UPDATE NOWAIT` → `55P03` until the first commits. With `hub.org_id` unset, `lock_audit_head()` → `P0002` (review M11, A19) | a lock that no longer locks: `lock_audit_head` without `FOR UPDATE`, so two writers read one head and one gets `40001` from the trigger instead of waiting its turn |
| `TestOrgIdsListsEveryOrgAndOnlyIds` | orgs A and B; as `hub_app` with `hub.org_id` unset: `SELECT id FROM hub.orgs` → zero rows; `SELECT * FROM hub.org_ids()` → exactly A's and B's ids, in id order, and the result has one column (review M10, A18) | an `org_ids` that returns `display_name` or `entra_tenant_id`; the janitor or `hub verify` reading `orgs` directly and walking no org at all |
| `TestHubAppExecutesExactlyTheOwnerFunctions` | `information_schema.role_routine_grants` for `hub_app` in schema `hub` = `ensure_org`, `resolve_principal`, `erase_brief`, `lock_audit_head`, `org_ids`, and no other; `PUBLIC` holds `EXECUTE` on none of them | an owner function left executable by `PUBLIC` (Postgres's default on `CREATE FUNCTION`), or a sixth one nobody reviewed |

#### Steps

- [ ] **Step 1:** Re-read contract §10.1–§10.7. If it changed after this part was written, the
  contract's words win except for the three named constraints above. A16 is already in the
  contract (task 44a); a contract that still shows a single-column key, or an `erase_brief` that
  deletes from `idempotency_keys`, is stale, and this SQL wins.
- [ ] **Step 2:** Tests, then `0001_init.sql` exactly as above, `embed.go`, and `hubtest.Migrated`.
- [ ] **Step 3:** Run against the task-06 container. Fix the SQL until every test passes; **any
  change to the SQL beyond the three names is recorded in the commit message** and in the
  contract by task 44b (a build departure).
- [ ] **Step 4:** Commit by pathspec, the four files.

#### Acceptance

```bash
go test -tags integration ./control-plane/internal/hub/migrate/ -race -count=1 -v 2>&1 | grep -c "^--- PASS"   # 38 (10 from task 06 + 28)
go build ./control-plane/...                                                                               # ok
grep -c "CREATE TABLE" control-plane/internal/hub/migrate/sql/0001_init.sql                                # 20
```

#### What this task does not claim

- That this schema runs on **Azure** Postgres Flexible Server 16 unchanged; it is tested on
  `postgres:16-alpine`. Phase 8's first apply is the check (ADR-H1 §11).
- Performance of the per-org audit lock under load (ADR-H1 R10): measured later, not here.
- That the owner role cannot rewrite history: it can, by dropping a constraint or disabling a
  trigger. `hub verify` (tasks 20, 27) is what catches that afterwards (contract T5).

