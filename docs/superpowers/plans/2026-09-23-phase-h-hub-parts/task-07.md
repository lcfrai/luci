### Task 07: `sql/0001_init.sql` — the complete first schema, and a database test for every guard

**Lane A. Depends on: 06. About 3 hours. Contract task H0 (and the database half of G4, G5).**

The contract's §10.2–§10.5 is the schema, written from knowledge and never applied (contract §15).
This task applies it and proves, **in the database and without any Go handler**, each guarantee of
§10.7's "Database" column: a revision never changes, a head moves one step onto a real revision, a
pin never moves, append-only tables are append-only, the audit chain is gapless and hashed, and one
org never sees another's rows.

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
  CONSTRAINT users_org_id_entra_oid_key UNIQUE (org_id, entra_oid)
);
CREATE INDEX users_org_email ON users (org_id, email);

CREATE TABLE teams (
  id         text        PRIMARY KEY CHECK (id ~ '^tm-[0-9a-f]{16}$'),
  org_id     text        NOT NULL REFERENCES orgs (id),
  name       text        NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
  created_at timestamptz NOT NULL DEFAULT now(),
  created_by text        NOT NULL REFERENCES users (id)
);
CREATE UNIQUE INDEX teams_org_name ON teams (org_id, lower(name));

CREATE TABLE team_members (
  org_id   text        NOT NULL REFERENCES orgs (id),
  team_id  text        NOT NULL REFERENCES teams (id),
  user_id  text        NOT NULL REFERENCES users (id),
  role     team_role   NOT NULL,
  added_at timestamptz NOT NULL DEFAULT now(),
  added_by text        REFERENCES users (id),
  PRIMARY KEY (team_id, user_id)
);
CREATE INDEX team_members_user ON team_members (user_id);

CREATE TABLE team_invites (
  id            text         PRIMARY KEY CHECK (id ~ '^iv-[0-9a-f]{16}$'),
  org_id        text         NOT NULL REFERENCES orgs (id),
  team_id       text         NOT NULL REFERENCES teams (id),
  email         text         NOT NULL CHECK (email = lower(email) AND char_length(email) BETWEEN 3 AND 254),
  role          team_role    NOT NULL,
  state         invite_state NOT NULL DEFAULT 'pending',
  created_at    timestamptz  NOT NULL DEFAULT now(),
  created_by    text         NOT NULL REFERENCES users (id),
  bound_user_id text         REFERENCES users (id),
  bound_at      timestamptz,
  revoked_at    timestamptz,
  CHECK ((state = 'bound')   = (bound_user_id IS NOT NULL AND bound_at IS NOT NULL)),
  CHECK ((state = 'revoked') = (revoked_at IS NOT NULL))
);
CREATE UNIQUE INDEX team_invites_one_pending ON team_invites (team_id, email) WHERE state = 'pending';
CREATE INDEX team_invites_binding ON team_invites (org_id, email) WHERE state = 'pending';

-- ── projects ─────────────────────────────────────────────────────────────────
CREATE TABLE projects (
  id         text        PRIMARY KEY CHECK (id ~ '^pj-[0-9a-f]{16}$'),
  org_id     text        NOT NULL REFERENCES orgs (id),
  team_id    text        NOT NULL REFERENCES teams (id),
  name       text        NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
  created_at timestamptz NOT NULL DEFAULT now(),
  created_by text        NOT NULL REFERENCES users (id)
);
CREATE UNIQUE INDEX projects_team_name ON projects (team_id, lower(name));

CREATE TABLE project_remotes (
  org_id     text        NOT NULL REFERENCES orgs (id),
  project_id text        NOT NULL REFERENCES projects (id),
  repo_key   text        NOT NULL CHECK (char_length(repo_key) BETWEEN 3 AND 300),
  host       git_host    NOT NULL,
  added_at   timestamptz NOT NULL DEFAULT now(),
  added_by   text        NOT NULL REFERENCES users (id),
  PRIMARY KEY (project_id, repo_key),
  UNIQUE (org_id, repo_key)                -- a repository is one project's, org-wide
);

-- ── briefs ───────────────────────────────────────────────────────────────────
CREATE TABLE briefs (
  id                       text        PRIMARY KEY CHECK (id ~ '^bf-[0-9a-f]{16}$'),
  org_id                   text        NOT NULL REFERENCES orgs (id),
  project_id               text        NOT NULL REFERENCES projects (id),
  scope                    brief_scope NOT NULL,
  branch_ref               text        NOT NULL DEFAULT '',
  head_revision            integer     NOT NULL DEFAULT 0 CHECK (head_revision >= 0),
  latest_complete_revision integer     NOT NULL DEFAULT 0,
  archived_at              timestamptz,
  erased_at                timestamptz,
  created_at               timestamptz NOT NULL DEFAULT now(),
  created_by               text        NOT NULL REFERENCES users (id),
  updated_at               timestamptz NOT NULL DEFAULT now(),
  CHECK ((scope = 'project' AND branch_ref = '')
      OR (scope = 'branch'  AND branch_ref LIKE 'refs/heads/_%' AND octet_length(branch_ref) <= 255)),
  CHECK (latest_complete_revision BETWEEN 0 AND head_revision),
  CHECK (erased_at IS NULL OR archived_at IS NOT NULL)
);
CREATE UNIQUE INDEX briefs_live_project ON briefs (project_id)
  WHERE scope = 'project' AND archived_at IS NULL;
CREATE UNIQUE INDEX briefs_live_branch ON briefs (project_id, branch_ref)
  WHERE scope = 'branch' AND archived_at IS NULL;

CREATE TABLE brief_revisions (
  org_id     text         NOT NULL REFERENCES orgs (id),
  brief_id   text         NOT NULL REFERENCES briefs (id),
  revision   integer      NOT NULL CHECK (revision >= 1),
  project_id text         NOT NULL REFERENCES projects (id),
  scope      brief_scope  NOT NULL,
  branch_ref text         NOT NULL,
  status     brief_status NOT NULL,
  canonical  bytea,                                   -- contract §3.4's C; NULL exactly when erased
  digest     bytea        NOT NULL CHECK (octet_length(digest) = 32),
  saved_at   timestamptz  NOT NULL DEFAULT now(),
  saved_by   text         NOT NULL REFERENCES users (id),
  erased_at  timestamptz,
  PRIMARY KEY (brief_id, revision),
  UNIQUE (brief_id, revision, digest),                -- the target of every digest-carrying link
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
  project_id   text        NOT NULL REFERENCES projects (id),
  digest       bytea       NOT NULL,
  content_type text        NOT NULL CHECK (char_length(content_type) BETWEEN 3 AND 100),
  uploaded_at  timestamptz NOT NULL DEFAULT now(),
  uploaded_by  text        NOT NULL REFERENCES users (id),
  PRIMARY KEY (project_id, digest),
  FOREIGN KEY (org_id, digest) REFERENCES blobs (org_id, digest)
);
CREATE INDEX project_blobs_janitor ON project_blobs (uploaded_at);

-- ── design approvals and session links ───────────────────────────────────────
CREATE TABLE design_approvals (
  id              text        PRIMARY KEY CHECK (id ~ '^ap-[0-9a-f]{16}$'),
  org_id          text        NOT NULL REFERENCES orgs (id),
  project_id      text        NOT NULL REFERENCES projects (id),
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
  approved_by     text        NOT NULL REFERENCES users (id),
  UNIQUE (project_id, design_key),
  CHECK (design_key = 'design:' || canvas_id || '@' || design_revision),
  FOREIGN KEY (brief_id, brief_revision, brief_digest)
    REFERENCES brief_revisions (brief_id, revision, digest),
  FOREIGN KEY (project_id, manifest_digest) REFERENCES project_blobs (project_id, digest)
);

CREATE TABLE design_approval_blobs (
  org_id      text  NOT NULL REFERENCES orgs (id),
  approval_id text  NOT NULL REFERENCES design_approvals (id),
  project_id  text  NOT NULL,
  digest      bytea NOT NULL,
  PRIMARY KEY (approval_id, digest),
  FOREIGN KEY (project_id, digest) REFERENCES project_blobs (project_id, digest)
);

CREATE TABLE session_links (
  id             text        PRIMARY KEY CHECK (id ~ '^ln-[0-9a-f]{16}$'),
  org_id         text        NOT NULL REFERENCES orgs (id),
  project_id     text        NOT NULL REFERENCES projects (id),
  design_key     text        NOT NULL,
  brief_id       text        NOT NULL,
  brief_revision integer     NOT NULL,
  session_id     text        NOT NULL CHECK (session_id ~ '^[a-z]{1,4}-[0-9a-f]{8,32}$'),
  branch_ref     text        NOT NULL,
  created_at     timestamptz NOT NULL DEFAULT now(),
  created_by     text        NOT NULL REFERENCES users (id),
  UNIQUE (created_by, session_id, design_key),
  FOREIGN KEY (project_id, design_key) REFERENCES design_approvals (project_id, design_key),
  FOREIGN KEY (brief_id, brief_revision) REFERENCES brief_revisions (brief_id, revision)
);

-- ── releases ─────────────────────────────────────────────────────────────────
CREATE TABLE releases (
  id                        text          PRIMARY KEY CHECK (id ~ '^rl-[0-9a-f]{16}$'),
  org_id                    text          NOT NULL REFERENCES orgs (id),
  project_id                text          NOT NULL REFERENCES projects (id),
  kind                      release_kind  NOT NULL,
  panacea_project           text          NOT NULL CHECK (panacea_project ~ '^[a-z0-9-]{1,63}$'),
  environment               text          NOT NULL CHECK (environment ~ '^[a-z0-9-]{1,63}$'),
  branch_ref                text          NOT NULL DEFAULT '',
  commit_sha                text          CHECK (commit_sha ~ '^[0-9a-f]{40}$'),
  source_deployment_id      text          CHECK (char_length(source_deployment_id) BETWEEN 1 AND 128),
  source_release_id         text          REFERENCES releases (id),
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
  reserved_by               text          NOT NULL REFERENCES users (id),
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
  CHECK ((state = 'confirmed') = (confirmed_at IS NOT NULL)),
  CHECK ((state = 'abandoned') = (abandoned_at IS NOT NULL AND abandon_reason IS NOT NULL)),
  CHECK ((state = 'expired')   = (expired_at IS NOT NULL)),
  FOREIGN KEY (pin_brief_id, pin_brief_revision, pin_brief_digest)
    REFERENCES brief_revisions (brief_id, revision, digest),
  FOREIGN KEY (project_id, design_key) REFERENCES design_approvals (project_id, design_key)
);
CREATE UNIQUE INDEX releases_one_per_run ON releases (org_id, panacea_project, pipeline_run)
  WHERE pipeline_run IS NOT NULL;
CREATE INDEX releases_by_project ON releases (project_id, reserved_at DESC);
CREATE INDEX releases_to_expire ON releases (expires_at) WHERE state = 'reserved';

CREATE TABLE release_deployments (
  org_id          text        NOT NULL REFERENCES orgs (id),
  release_id      text        NOT NULL REFERENCES releases (id),
  panacea_project text        NOT NULL,
  deployment_id   text        NOT NULL CHECK (char_length(deployment_id) BETWEEN 1 AND 128),
  joined_at       timestamptz NOT NULL DEFAULT now(),
  joined_by       text        NOT NULL REFERENCES users (id),
  PRIMARY KEY (release_id, deployment_id),
  UNIQUE (org_id, panacea_project, deployment_id)   -- a deployment belongs to one release
);

-- ── evidence (phase 3, H-2) ──────────────────────────────────────────────────
CREATE TABLE evidence (
  id               text        PRIMARY KEY CHECK (id ~ '^ev-[0-9a-f]{16}$'),
  org_id           text        NOT NULL REFERENCES orgs (id),
  project_id       text        NOT NULL REFERENCES projects (id),
  repo_key         text        NOT NULL,          -- verbatim; survives the remote's removal
  brief_id         text        NOT NULL,
  brief_revision   integer     NOT NULL,
  criterion_id     text        NOT NULL CHECK (char_length(criterion_id) BETWEEN 1 AND 128),
  criterion_digest bytea       NOT NULL CHECK (octet_length(criterion_digest) = 32),
  commit_sha       text        NOT NULL CHECK (commit_sha ~ '^[0-9a-f]{40}$'),
  verdict          verdict     NOT NULL,
  because          text        NOT NULL CHECK (char_length(btrim(because)) BETWEEN 1 AND 2000),
  recorded_at      timestamptz NOT NULL DEFAULT now(),
  recorded_by      text        NOT NULL REFERENCES users (id),
  FOREIGN KEY (brief_id, brief_revision) REFERENCES brief_revisions (brief_id, revision)
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
  actor_user_id text        REFERENCES users (id),
  canonical     bytea       NOT NULL,             -- contract §3.6, with teamId (plan A10)
  prev_hash     bytea       NOT NULL CHECK (octet_length(prev_hash) = 32),
  hash          bytea       NOT NULL,
  PRIMARY KEY (org_id, seq),
  CONSTRAINT audit_events_hash_chains CHECK (hash = sha256(prev_hash || canonical))
);
CREATE INDEX audit_feed_by_project ON audit_events (org_id, project_id, seq) WHERE feed;
CREATE INDEX audit_feed_by_team    ON audit_events (org_id, team_id, seq)    WHERE feed;

CREATE TABLE idempotency_keys (
  org_id         text        NOT NULL REFERENCES orgs (id),
  user_id        text        NOT NULL REFERENCES users (id),
  key            text        NOT NULL CHECK (key ~ '^[A-Za-z0-9_-]{1,64}$'),
  method         text        NOT NULL,
  path           text        NOT NULL,
  request_sha256 bytea       NOT NULL,
  status         smallint    NOT NULL DEFAULT 0,   -- 0 while the first request is in flight
  response       bytea,
  created_at     timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, key)
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
  IF (NEW.id, NEW.org_id, NEW.project_id, NEW.kind, NEW.panacea_project, NEW.environment,
      NEW.branch_ref, NEW.commit_sha, NEW.source_deployment_id, NEW.source_release_id,
      NEW.pin_reason, NEW.pin_brief_id, NEW.pin_brief_revision, NEW.pin_brief_digest,
      NEW.pin_head_revision_at_ship, NEW.design_key, NEW.reserved_at, NEW.reserved_by, NEW.expires_at)
     IS DISTINCT FROM
     (OLD.id, OLD.org_id, OLD.project_id, OLD.kind, OLD.panacea_project, OLD.environment,
      OLD.branch_ref, OLD.commit_sha, OLD.source_deployment_id, OLD.source_release_id,
      OLD.pin_reason, OLD.pin_brief_id, OLD.pin_brief_revision, OLD.pin_brief_digest,
      OLD.pin_head_revision_at_ship, OLD.design_key, OLD.reserved_at, OLD.reserved_by, OLD.expires_at)
  THEN
    RAISE EXCEPTION 'hub: a release''s pin never changes' USING ERRCODE = 'insufficient_privilege';
  END IF;
  IF OLD.state <> 'reserved' AND NEW IS DISTINCT FROM OLD THEN
    RAISE EXCEPTION 'hub: release % is %, not reserved', OLD.id, OLD.state USING ERRCODE = 'insufficient_privilege';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER releases_guard BEFORE UPDATE OR DELETE ON releases
  FOR EACH ROW EXECUTE FUNCTION guard_release();

-- ── the audit chain: gapless, in order, and it advances its own head ─────────
CREATE FUNCTION audit_append() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE h hub.audit_heads%ROWTYPE;
BEGIN
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

-- ── the three functions that run as the owner (contract §10.4) ───────────────
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
  RETURN v_n;
END $$;

REVOKE ALL ON FUNCTION ensure_org(uuid, text, text), resolve_principal(uuid, uuid, text, text, text),
                       erase_brief(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ensure_org(uuid, text, text), resolve_principal(uuid, uuid, text, text, text),
                          erase_brief(text) TO hub_app;

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
              expired_at)                                      ON releases         TO hub_app;
GRANT DELETE                                                   ON blobs, project_blobs TO hub_app;
GRANT UPDATE (seq, hash)                                       ON audit_heads      TO hub_app;
GRANT UPDATE (status, response), DELETE                        ON idempotency_keys TO hub_app;
-- No UPDATE or DELETE on: orgs, users, brief_revisions, design_approvals,
-- design_approval_blobs, session_links, release_deployments, evidence, audit_events.
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
| `TestHubAppCannotUpdateOrDeleteImmutableTables` | as `hub_app`, `UPDATE`/`DELETE` on `brief_revisions`, `design_approvals`, `session_links`, `release_deployments`, `evidence`, `audit_events`, `orgs`, `users` → `42501` | a grant widened by accident (G4's grant layer) |
| `TestAppendOnlyTriggersHoldEvenForTheOwner` | as `hub_owner`, `UPDATE`, `DELETE`, `TRUNCATE` on each of the six append-only tables → `42501` from `forbid_change` | append-only enforced by grants alone |
| `TestARevisionMustHashToItsDigest` | `INSERT` a revision whose `digest` is not `sha256(canonical)` → `23514` | a digest the Go side computed wrong, stored anyway |
| `TestOnlyEraseBriefCanEraseARevision` | as the owner outside the function, an erasure-shaped `UPDATE` passes `guard_revision` only because `current_user` is `hub_owner`; any other column change → `42501`; `erase_brief` as `hub_app` erases and returns the count | the revision guard trusting the caller |
| `TestAHeadMovesOneStepOntoARealRevision` | `head_revision` +2 → `42501`; +1 with no revision row → `42501`; +1 after inserting that revision → ok; `latest_complete_revision` backwards → `42501` | a head pointing at nothing (P3) |
| `TestAnArchivedBriefIsFrozen` | after `archived_at` is set: head, ref and `archived_at` changes → `42501`; a new live brief on the same ref → ok | branch deletion that frees the ref but not the old brief |
| `TestAPinNeverMoves` | as `hub_app`, `UPDATE releases SET pin_brief_revision = …` → `42501` (column grant); as the owner → `42501` (trigger) | P7 held by one layer only |
| `TestOnlyAReservedReleaseMoves` | confirm a release; any further `UPDATE` of `state` → `42501` | a confirmed release re-confirmed with another run |
| `TestTheAuditChainIsGaplessAndHashed` | insert seq 1 then 2 correctly → ok and `audit_heads` advanced; seq 3 with the wrong `prev_hash`, or seq 4 skipping 3 → `40001`; a `hash` not equal to `sha256(prev_hash ‖ canonical)` → `23514` | a chain the Go side can fork (G5) |
| `TestRowLevelSecuritySeparatesOrgs` | two orgs with a user each; `set_config('hub.org_id', A, true)`; `SELECT` from each of the 19 tables sees only A; `INSERT` of an org-B row → `42501` (WITH CHECK) | a table missing from the RLS loop (T3) |
| `TestAnUnsetOrgSeesNothingEvenOnAReusedConnection` | on one connection: a transaction that sets org A and commits; then a new transaction that sets nothing → zero rows from `users` | a pooled connection carrying one request's org into the next |
| `TestResolvePrincipal` | an unknown tenant → no row; a new oid → `created = true`; again → `created = false`, same id; a changed name → refreshed | provisioning twice, or never refreshing the display name |
| `TestEnsureOrgIsIdempotent` | twice with two fresh ids → the first id both times; one `audit_heads` row | a second org row per restart |
| `TestEraseBriefChecksTheOrg` | with `hub.org_id` = B, erasing A's brief → `P0002` (`no_data_found`), and A's revisions keep their content | the owner function bypassing RLS without its own check (T3's residual) |
| `TestHubAppGrantsAreExactlyTheList` | `information_schema.role_table_grants` and `column_privileges` for `hub_app` equal §10.5's list, table by table, column by column | a grant nobody reviewed |

#### Steps

- [ ] **Step 1:** Re-read contract §10.1–§10.7. If it changed after this part was written, the
  contract's words win except for the three named constraints above.
- [ ] **Step 2:** Tests, then `0001_init.sql` exactly as above, `embed.go`, and `hubtest.Migrated`.
- [ ] **Step 3:** Run against the task-06 container. Fix the SQL until every test passes; **any
  change to the SQL beyond the three names is recorded in the commit message** and in the
  contract by task 44.
- [ ] **Step 4:** Commit by pathspec, the four files.

#### Acceptance

```bash
go test -tags integration ./control-plane/internal/hub/migrate/ -race -count=1 -v 2>&1 | grep -c "^--- PASS"   # 27 (10 from task 06 + 17)
go build ./control-plane/...                                                                               # ok
grep -c "CREATE TABLE" control-plane/internal/hub/migrate/sql/0001_init.sql                                # 20
```

#### What this task does not claim

- That this schema runs on **Azure** Postgres Flexible Server 16 unchanged; it is tested on
  `postgres:16-alpine`. Phase 8's first apply is the check (ADR-H1 §11).
- Performance of the per-org audit lock under load (ADR-H1 R10): measured later, not here.
- That the owner role cannot rewrite history: it can, by dropping a constraint or disabling a
  trigger. `hub verify` (tasks 20, 27) is what catches that afterwards (contract T5).

