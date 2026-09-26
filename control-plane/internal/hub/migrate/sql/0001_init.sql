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
