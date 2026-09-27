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
  -- per project (task 05 D9, carried to 07a): one design key can be approved in two projects
  UNIQUE (project_id, created_by, session_id, design_key),
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
  UNIQUE (org_id, project_id, id),         -- the target of release_deployments' link (D10)
  FOREIGN KEY (org_id, project_id)        REFERENCES projects (org_id, id),
  FOREIGN KEY (org_id, reserved_by)       REFERENCES users (org_id, id),
  FOREIGN KEY (org_id, source_release_id) REFERENCES releases (org_id, id),
  -- a NULL pin or design_key skips its check (MATCH SIMPLE); the CHECKs above keep the pin whole
  FOREIGN KEY (org_id, pin_brief_id, pin_brief_revision, pin_brief_digest)
    REFERENCES brief_revisions (org_id, brief_id, revision, digest),
  FOREIGN KEY (org_id, project_id, design_key)
    REFERENCES design_approvals (org_id, project_id, design_key)
);
-- D10 (task 05 fix round 1, carried to 07a): a Panacea name is the caller's assertion, so its
-- run and deployment keys are unique per Hub project, not per org: an org member in a team of
-- their own must not block another team's confirm or join.
CREATE UNIQUE INDEX releases_one_per_run ON releases (project_id, panacea_project, pipeline_run)
  WHERE pipeline_run IS NOT NULL;
CREATE INDEX releases_by_project ON releases (project_id, reserved_at DESC);
CREATE INDEX releases_to_expire ON releases (expires_at) WHERE state = 'reserved';

CREATE TABLE release_deployments (
  org_id          text        NOT NULL REFERENCES orgs (id),
  project_id      text        NOT NULL,   -- D10: the release's project, held by the link below
  release_id      text        NOT NULL,
  panacea_project text        NOT NULL,
  deployment_id   text        NOT NULL CHECK (char_length(deployment_id) BETWEEN 1 AND 128),
  joined_at       timestamptz NOT NULL DEFAULT now(),
  joined_by       text        NOT NULL,
  PRIMARY KEY (release_id, deployment_id),
  UNIQUE (project_id, panacea_project, deployment_id),  -- a deployment belongs to one release, per project (D10)
  FOREIGN KEY (org_id, project_id, release_id) REFERENCES releases (org_id, project_id, id),
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

-- 0001_init.sql, continued (task 07b): the guards and the owner functions. Appended after
-- task 07a's grants; the EXECUTE grants for the functions are with the functions.
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

-- Fix round 1, m5 (task 07 review): an archived brief takes no new revision. Its head is
-- frozen (guard_brief), so such a revision would never be read as the head, but it would
-- still put content under a brief that is frozen, or erased (erasure archives, and briefs'
-- CHECK keeps it so). The Hub refuses the write first, 409 brief_archived (tasks 04, 19);
-- this is the database's own stop, and it holds for the owner too. A plain read is enough:
-- every Hub write takes its org's audit head lock first (contract §10.6), archiving
-- included, so an archive cannot commit between this read and the insert's commit.
-- TestAnArchivedBriefTakesNoRevision pins it.
CREATE FUNCTION guard_revision_insert() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF EXISTS (SELECT 1 FROM hub.briefs b WHERE b.id = NEW.brief_id AND b.archived_at IS NOT NULL) THEN
    RAISE EXCEPTION 'hub: brief % is archived; it takes no new revision', NEW.brief_id
      USING ERRCODE = 'insufficient_privilege';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER brief_revisions_no_insert_when_archived BEFORE INSERT ON brief_revisions
  FOR EACH ROW EXECUTE FUNCTION guard_revision_insert();

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
-- on audit_heads (grants above), so a compromised Hub cannot move a head by a direct
-- UPDATE. As the owner this function is not bound by row-level security, so it checks the
-- org itself, FIRST, before it locks anything: without that check an org-A session could
-- lock, and for a moment move, org B's head. WITH CHECK would still refuse the row, but only
-- after BEFORE triggers have run. TestAuditAppendChecksTheOrgFirst pins the check: with B's
-- head held by another session, the check returns 42501 at once, where a trigger without it
-- blocks on that lock.
-- Fix round 1, I1 (task 07 review): as the owner, this function must run only as
-- audit_events' own trigger. Every role may create a TEMP table, and CREATE TRIGGER asks
-- only that its caller own the table and may EXECUTE the function, so with PUBLIC's
-- default EXECUTE hub_app could attach it to a table of its own and move any org's head
-- with no event. PUBLIC's EXECUTE is revoked below, and the function refuses any other
-- table itself, so a later stray GRANT does not reopen it. The real trigger still fires
-- for hub_app: Postgres checks EXECUTE at CREATE TRIGGER, not when the trigger fires.
-- TestAuditAppendRunsOnlyOnAuditEvents pins both layers.
CREATE FUNCTION audit_append() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path = hub, pg_temp AS $$
DECLARE h hub.audit_heads%ROWTYPE;
BEGIN
  IF TG_RELID IS DISTINCT FROM 'hub.audit_events'::regclass THEN
    RAISE EXCEPTION 'hub: audit_append runs only on hub.audit_events' USING ERRCODE = 'insufficient_privilege';
  END IF;
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
REVOKE ALL ON FUNCTION audit_append() FROM PUBLIC;

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
-- SELECT … FOR UPDATE needs UPDATE privilege on at least one column of the table
-- (TestTheHeadLockNeedsTheFunction pins it), so with hub_app's UPDATE on audit_heads
-- revoked the lock is taken here, as the owner, and only on the session's org: there is no
-- parameter to name another. The row lock belongs to the calling transaction.
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
