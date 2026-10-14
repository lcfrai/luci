### Task 145: the W5 migration, its guards and matrix rows: no sync principal or app writes a fact state or `done`

**Slice:** W5. **Size:** 3 focused hours (an estimate, not a measurement). **Stream:** WH.
**dependsOn:** 07 (SW.3: `guard_issue_state`, the settings, the matrix file) and 143 (the seven
`ActorKind`s, the W5 wire). **Seam:** **SW.3**, extended: after this task, a W5 principal's reach
into issue state is fixed in the database, and each W5 route task (146-167) appends only its own
tables' columns and its route rows. Consumed by 146-167 and by task 168's end-to-end run.

W5 is the first slice whose writers are not all people. A tracker's integration principal, an
approved app, an API key and an outside host each write to the Hub. The design's rule is one
sentence (05 §5.4 W5, `05-design.md:1273`): *"Sync never writes a state, a fact or `done`"*, and
§1 L-250 adds that *"an app acts as a person, never as a free agent"* (`05-design.md:437`). Go
enforces both in each route; this task makes the database refuse them too, so a Go bug in any of
twenty routes cannot close an issue.

**Anchors, re-read on 2026-09-26** in the Hub worktree at `feat/hub` `3df9fab8`
(`docs/superpowers/contracts/2026-09-23-hub-contract.md`): §10.3 row-level security
(`:1904-1932`, the `org_isolation` policy on `current_setting('hub.org_id', true)`); §10.5 grants
(`:2001-2027`); §5.3's matrix columns `V M A O N X` (`:337-341`); `forbid_change()` (`:1799`); and
the process's reach, *"No Microsoft Graph, no client secret, no certificate, no storage account, no
Key Vault access in v1"* (`:2083-2085`).

**Read in this branch's parts, UNVERIFIED until merge:** task 07's guards (`task-07.md:101-245`),
its settings (`:16-31`) and matrix file (`:317-391`); task 10's replacement of
`guard_issue_state()` (`task-10.md:166-204`), the body this task replaces again; task 143's
`ActorKinds` (`task-143.md:152-166`); task 144's outside causes (`task-144.md`, *`hubapi/work.go`
⚑*); and the eleven route parts that wrote their tables for this migration (decision 1).

#### Decisions the scope left open, made here

1. **The route owner's SQL wins, and is taken as written** (task 143's decision 1, for SQL). Eleven
   parts already hold complete SQL for this file, each headed *"in task 145's NNNN_work_w5.sql"*.
   This part places each block in the file, in task order, and writes nothing over it; it writes
   only the tables no part has written, the guards and the matrix:

   | Block, in file order | Owner | Source in this plan |
   |---|---|---|
   | `work_requests`, its freeze and `erase_request` | 146 | `task-146.md:97-172` |
   | `work_intake_rule_sets`, the rule-version key on requests | 147 | `task-147.md:79-103` |
   | `work_intake_forms`, the form key on requests | 148 | `task-148.md:77-109` |
   | `work_intake_settings`, `work_release_issues`, `work_request_notices`, `work_mail_cursor` | 149 | `task-149.md:98-170` |
   | `work_tracker_connections`, `work_external_links`, `work_sync_runs`, `guard_sync_principal` | 150 | `task-150.md:90-187` |
   | `issue_comments`' external columns | 153 | `task-153.md:140-160` |
   | `work_hook_deliveries`, `project_key_history` | 154 | `task-154.md:110-139` |
   | **`work_host_hooks`, `work_outside_pr_links`, `work_outside_commit_links`** | **145** | below |
   | the linkback and autolink columns, `work_outside_pr_filings` | 155 | `task-155.md:145-178` |
   | **`work_webhooks`, `work_webhook_deliveries`** | **145** (156's columns) | below |
   | **`work_teams_bindings`, `work_teams_pairings`, `work_request_messages`** | **145** (157's columns) | below |
   | **`work_apps`, `hub.app_for_client`, `hub.find_principal`** | **145** (159's columns) | below |
   | `work_api_keys`, `hub.key_by_id` | 160 | `task-160.md:165-235` |
   | `work_mcp_connectors` | 161 | `task-161.md:439-463` |
   | `work_loops`, `work_loop_versions`, `work_loop_runs`, `work_loop_cursors` | 162 | `task-162.md:212-250` |
   | **the run-outcome columns and their freeze** | **145** (163's columns) | below |
   | **`work_team_agent_policies`** | **145** (143 decision 5) | below |
   | **the actor guards** (decisions 3-6) | **145** | below |

2. **The file is `NNNN_work_w5.sql`.** The scope says `NNNN_work_intake.sql`; nine of the eleven
   owners wrote `NNNN_work_w5.sql` and two (161, 162) wrote `_work_intake`. The majority wins, and
   `_intake` would be wrong anyway: most of W5 is not intake. `NNNN` is the next free number at
   sync time, renumbered on a clash (rule H1).

3. **Seven kinds, and a third setting.** `hub.work_actor_kind` takes task 143's seven words
   (`person`, `agent`, `janitor`, `integration`, `app`, `key`, `host`); any other non-empty word
   raises. Unset keeps task 07's reading (*not a person*, `task-07.md:19-21`): no fact, and a `hand`
   move among the non-fact states still passes, as W1-W4 were written against it. A new setting,
   `hub.work_actor_user`, names the person a `host` write is attributed to (task 154's
   `SetWorkActor`, `task-154.md:716`). All three are set with `set_config(…, true)`.
4. **An integration principal writes no state at all.** Task 07 let `integration` with `hand` reach
   `backlog` (`task-07.md:510`, the last clause of `TestAnIntegrationNeverWritesAFact`). That was a
   reserved name in W1; W5 is where it acts, and the design says a state (§5.4 W5), not only a fact.
   This task tightens it, and that one clause of task 07's test flips to `42501` (⚑). Task 07's own
   rule forbids loosening a guard, not tightening one (`task-07.md:95`).
5. **An app or a key acts as its person, short of a fact.** Every move into `started`,
   `in_review`, `landed` or `done` is refused whatever the cause; a `hand` move among the other
   states passes, as its person's would. This is task 144's decision 3 and §1 L-250. The scope's
   sentence *"`integration`, `app` and `key` are refused every state column write at all"* is not
   taken for `app` and `key`: an app that cannot move an issue from `backlog` to `todo` is not
   acting as a person, and Linear parity for keys (L-245) needs that move. **Task 160 decides the
   other way** (`task-160.md:57`, and its test `backlog → todo refused`, `:475`): gap 1.
6. **A host moves forward, three ways, as a mapped person.** With `host`, only the pairs
   (`outside_push`, `started`), (`outside_pr`, `in_review`) and (`outside_merge`, `landed`) pass,
   each only from an earlier state (task 154's `before`, `task-154.md:722-725`), and only with
   `hub.work_actor_user` a user of the issue's org. `done` is never a host's. An outside cause
   under any other kind raises, so a person's route cannot forge a host move.
7. **Revisions:** `integration` inserts revision 1 only, of an issue in `triage` with an outside
   `external_source`; `host` inserts none; every other kind is unchanged. Revision 1 is enough to
   mean *"the issue it is creating"*: `(issue_id, revision)` is the key, and an issue's revision 1
   is written in the transaction that creates it (task 06's shape, **UNVERIFIED**).
8. **The two sync guards must agree.** Task 150's `guard_sync_principal` is keyed on
   `hub.work_sync_connection` and stays as written. `work_actor_consistent()` raises when that
   setting is set and the kind is not `integration`, or the kind is `integration` and it is unset,
   so a store that sets one and forgets the other fails on its first write, not silently.
9. **One refusal prefix.** Every new `RAISE` reads `hub: work: principal_cannot_move_state: …`
   with SQLSTATE `42501`, task 150's form (`task-150.md:163`), so the store maps it to 403
   `principal_cannot_move_state`; task 07's and 10's own refusals keep their text and their 422.
10. **Secrets are references.** Every table that holds an HMAC or signing secret holds a
    `secret_ref` and a version, never the secret; an API key holds a SHA-256 hash and its first 8
    characters (task 160's block). The Hub process has no Key Vault access (`hub-contract.md:2085`),
    so **Key Vault under the Hub's workload identity is a proposed Hub-contract addition** for the
    `feat/hub` owner, and the routes that must read a secret (151-152, 154, 156) wait on it.
11. **Principals are matrix rows, not columns.** H16b's row is six statuses for `V M A O N X`
    (`task-07.md:379-390`); a W5 principal is a seventh caller only for a few routes, so a
    `principalRow` names one caller, one route and one status. The file is task 07's corrected
    `server/work_matrix_rows_integration_test.go` (`task-07.md:48-51`), not the scope's
    `work_matrix_rows.go`.

#### Files

| Path (under `control-plane/internal/hub/`) | Change |
|---|---|
| `migrate/sql/NNNN_work_w5.sql` | new: the owners' blocks in decision 1's order, and every block below |
| `migrate/work_w5_integration_test.go` | new: the migration and guard tests |
| `migrate/work_guards_integration_test.go` ⚑ (task 07) | one clause flips (decision 4) |
| `server/matrix_integration_test.go` ⚑ (H16b) | the W5 principals on `fixture`, one builder call |
| `server/work_matrix_rows_integration_test.go` ⚑ (task 07) | `principalRow`, the guard rows, `"145"` in `workMatrixLive` |

#### `NNNN_work_w5.sql`, the header and the outside-host tables (complete)

Every id pattern is task 06's (`'^ic-[0-9a-f]{16}$'`, `task-06.md:292`) with task 143's prefix;
every foreign key is org-scoped `(org_id, id)`, as task 06's are (`task-06.md:309-313`).

```sql
-- NNNN_work_w5.sql (task 145): W5's tables, in task order (decision 1), then the
-- actor guards. Each owner's block is placed as written; 145's blocks are marked.
-- The settings this file reads, each set with set_config(…, true) by the store:
--   hub.org_id             hub-contract §10.3
--   hub.work_cause         task 07; W5 adds outside_push, outside_pr, outside_merge
--   hub.work_actor_kind    task 07; W5 widens it to task 143's seven kinds
--   hub.work_actor_user    task 145: the person a host write is attributed to
--   hub.work_sync_connection  task 150: the tc- id a sync write is for

-- ── 146, 147, 148, 149, 150, 153, 154: placed here as written ─────────────────

-- ── task 145: outside hosts (tasks 154-155 read and write these) ─────────────
CREATE TABLE work_host_hooks (
  org_id         text        NOT NULL REFERENCES orgs (id),
  project_id     text        NOT NULL,
  host           text        NOT NULL CHECK (host IN ('azure_devops', 'github')),
  remote_url     text        NOT NULL CHECK (remote_url ~ '^https://' AND octet_length(remote_url) <= 2048),
  secret_ref     text        NOT NULL CHECK (octet_length(secret_ref) BETWEEN 1 AND 512), -- decision 10
  secret_version integer     NOT NULL DEFAULT 1 CHECK (secret_version >= 1),
  created_at     timestamptz NOT NULL DEFAULT now(),
  created_by     text        NOT NULL,
  PRIMARY KEY (project_id, host),
  FOREIGN KEY (org_id, project_id) REFERENCES projects (org_id, id),
  FOREIGN KEY (org_id, created_by) REFERENCES users (org_id, id)
);

CREATE TABLE work_outside_pr_links (
  id           text        PRIMARY KEY CHECK (id ~ '^hl-[0-9a-f]{16}$'),
  org_id       text        NOT NULL REFERENCES orgs (id),
  issue_id     text        NOT NULL,
  host         text        NOT NULL CHECK (host IN ('azure_devops', 'github')),
  repo         text        NOT NULL CHECK (char_length(repo) BETWEEN 1 AND 400),
  number       integer     NOT NULL CHECK (number >= 1),
  url          text        NOT NULL CHECK (url ~ '^https://' AND octet_length(url) <= 2048),
  pr_state     text        NOT NULL CHECK (pr_state IN ('open', 'merged', 'closed')),
  word         text        NOT NULL CHECK (word IN ('closing', 'non_closing', 'branch')),
  merge_commit text        CHECK (merge_commit IS NULL OR merge_commit ~ '^[0-9a-f]{40}$'),
  linked_at    timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (issue_id, host, repo, number),
  FOREIGN KEY (org_id, issue_id) REFERENCES issues (org_id, id),
  CONSTRAINT work_pr_links_merge_iff_merged CHECK ((pr_state = 'merged') = (merge_commit IS NOT NULL))
);

-- Append-only: a commit that named an issue named it (the append trigger is below).
CREATE TABLE work_outside_commit_links (
  id        text        PRIMARY KEY CHECK (id ~ '^hl-[0-9a-f]{16}$'),
  org_id    text        NOT NULL REFERENCES orgs (id),
  issue_id  text        NOT NULL,
  host      text        NOT NULL CHECK (host IN ('azure_devops', 'github')),
  repo      text        NOT NULL CHECK (char_length(repo) BETWEEN 1 AND 400),
  sha       text        NOT NULL CHECK (sha ~ '^[0-9a-f]{40}$'),
  url       text        NOT NULL CHECK (url ~ '^https://' AND octet_length(url) <= 2048),
  word      text        NOT NULL CHECK (word IN ('closing', 'non_closing', 'branch')),
  pushed_by text,                                    -- the mapped u- id; NULL when none maps
  linked_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (repo, sha, issue_id),
  FOREIGN KEY (org_id, issue_id)  REFERENCES issues (org_id, id),
  FOREIGN KEY (org_id, pushed_by) REFERENCES users (org_id, id)
);

-- ── 155: placed here as written (its ALTERs fold into the two tables above) ──
```

`work_host_hooks` has no id: a project has at most one hook per host, and `HostHook` carries none
(`task-143.md:327-333`). `pr_state` and `merge_commit` move together, so a merged link always
names its commit and task 154's `landed` has a `LandCommit` to give.

#### `NNNN_work_w5.sql`, continued: webhooks, Teams and the request thread (complete)

Task 156's column list (`task-156.md:441-457`) and task 157's (`task-157.md:242-247`), in this
file's names; each part said *"its file wins on names"*, and none is renamed.

```sql
-- ── task 145 (156's columns): outbound webhooks ───────────────────────────────
CREATE TABLE work_webhooks (
  id                   text        PRIMARY KEY CHECK (id ~ '^wh-[0-9a-f]{16}$'),
  org_id               text        NOT NULL REFERENCES orgs (id),
  team_id              text        NOT NULL,
  project_id           text,                              -- NULL: every project of the team
  url                  text        NOT NULL CHECK (url ~ '^https://' AND octet_length(url) <= 2048),
  kinds                text[]      NOT NULL CHECK (cardinality(kinds) BETWEEN 1 AND 64),
  state                text        NOT NULL DEFAULT 'active' CHECK (state IN ('active', 'disabled')),
  secret_ref           text        NOT NULL CHECK (octet_length(secret_ref) BETWEEN 1 AND 512),
  secret_version       bigint      NOT NULL DEFAULT 1 CHECK (secret_version >= 1),
  prev_secret_version  bigint      CHECK (prev_secret_version IS NULL OR prev_secret_version < secret_version),
  rotated_at           timestamptz,
  cursor_seq           bigint      NOT NULL DEFAULT 0,
  consecutive_failures integer     NOT NULL DEFAULT 0 CHECK (consecutive_failures >= 0),
  created_at           timestamptz NOT NULL DEFAULT now(),
  created_by           text        NOT NULL,
  FOREIGN KEY (org_id, team_id)    REFERENCES teams (org_id, id),
  FOREIGN KEY (org_id, project_id) REFERENCES projects (org_id, id),
  FOREIGN KEY (org_id, created_by) REFERENCES users (org_id, id)
);

-- Append-only: an attempt is a new row (the append trigger is below).
CREATE TABLE work_webhook_deliveries (
  id              text        NOT NULL CHECK (id ~ '^wd-[0-9a-f]{16}$'),
  org_id          text        NOT NULL REFERENCES orgs (id),
  webhook_id      text        NOT NULL REFERENCES work_webhooks (id),
  event_seq       bigint      NOT NULL,
  attempt         integer     NOT NULL CHECK (attempt >= 0),      -- 0 is queued (task 156)
  status          text        NOT NULL CHECK (status IN ('ok', 'failed', 'pending')),
  response_code   integer     CHECK (response_code IS NULL OR response_code BETWEEN 100 AND 599),
  at              timestamptz NOT NULL DEFAULT now(),
  next_attempt_at timestamptz,                                    -- a scheduled retry, not a forecast
  PRIMARY KEY (id, attempt),
  UNIQUE (webhook_id, event_seq, attempt)
);

-- ── task 145 (157's columns): Teams bindings, pairing codes, the request thread ──
CREATE TABLE work_teams_bindings (
  org_id         text        NOT NULL REFERENCES orgs (id),
  team_id        text        NOT NULL,
  subject_kind   text        NOT NULL CHECK (subject_kind IN ('intake', 'feature', 'view', 'digest')),
  subject_id     text,
  channel_ref    text        NOT NULL CHECK (octet_length(channel_ref) BETWEEN 1 AND 512),
  cursor_seq     bigint      NOT NULL DEFAULT 0,
  last_posted_at timestamptz,
  members        text[]      CHECK (members IS NULL OR cardinality(members) <= 2000),
  created_at     timestamptz NOT NULL DEFAULT now(),
  created_by     text        NOT NULL,
  FOREIGN KEY (org_id, team_id)    REFERENCES teams (org_id, id),
  FOREIGN KEY (org_id, created_by) REFERENCES users (org_id, id),
  CONSTRAINT work_teams_subject_id_iff CHECK ((subject_kind IN ('feature', 'view')) = (subject_id IS NOT NULL))
);
CREATE UNIQUE INDEX work_teams_bindings_once
  ON work_teams_bindings (team_id, subject_kind, coalesce(subject_id, ''), channel_ref);

CREATE TABLE work_teams_pairings (
  org_id      text        NOT NULL REFERENCES orgs (id),
  team_ref    text        NOT NULL CHECK (octet_length(team_ref) BETWEEN 1 AND 512),
  code_hash   bytea       NOT NULL CHECK (octet_length(code_hash) = 32),   -- SHA-256; never the code
  channel_ref text        NOT NULL CHECK (octet_length(channel_ref) BETWEEN 1 AND 512),
  expires_at  timestamptz NOT NULL,
  used_at     timestamptz,
  failures    integer     NOT NULL DEFAULT 0 CHECK (failures BETWEEN 0 AND 10),
  PRIMARY KEY (org_id, code_hash)
);

-- Append-only: an outside reply on a request's thread, text·H (the append trigger is below).
CREATE TABLE work_request_messages (
  org_id      text        NOT NULL REFERENCES orgs (id),
  request_id  text        NOT NULL REFERENCES work_requests (id),
  channel_ref text        NOT NULL CHECK (octet_length(channel_ref) BETWEEN 1 AND 998),
  author_name text        NOT NULL CHECK (char_length(author_name) BETWEEN 1 AND 200),
  user_id     text,                                          -- the org user, when one maps
  body        text        NOT NULL CHECK (octet_length(body) <= 16384),
  at          timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (request_id, channel_ref),                     -- a retry of one message is one row
  FOREIGN KEY (org_id, user_id) REFERENCES users (org_id, id)
);
```

`work_requests` has no `UNIQUE (org_id, id)` (`task-146.md:100`, its key is `id` alone), so the
thread's key to it is `id`; row-level security keeps the org line. The 998-byte cap on
`channel_ref` is task 146 decision 8's (the RFC 5322 line limit), as a mail reply is one channel.
`work_request_messages.body` has no erase path yet: task 146's `erase_request` blanks the request
only, so an erased request's thread still holds the requester's words (gap 4).

#### `NNNN_work_w5.sql`, continued: apps, loop outcomes and team agent policy (complete)

```sql
-- ── task 145 (159's columns): apps that act as a person ──────────────────────
CREATE TABLE work_apps (
  id            text        PRIMARY KEY CHECK (id ~ '^oa-[0-9a-f]{16}$'),
  org_id        text        NOT NULL REFERENCES orgs (id),
  client_id     uuid        NOT NULL,                        -- the Entra application id
  manifest      jsonb       NOT NULL CHECK (jsonb_typeof(manifest) = 'object'),
  scopes        text[]      NOT NULL CHECK (scopes <@ ARRAY['work.read', 'work.write'] AND cardinality(scopes) >= 1),
  state         text        NOT NULL DEFAULT 'pending' CHECK (state IN ('pending', 'approved', 'revoked')),
  registered_by text        NOT NULL,
  registered_at timestamptz NOT NULL DEFAULT now(),
  decided_by    text,
  decided_at    timestamptz,
  reason        text        CHECK (reason IS NULL OR char_length(reason) <= 500),
  FOREIGN KEY (org_id, registered_by) REFERENCES users (org_id, id),
  FOREIGN KEY (org_id, decided_by)    REFERENCES users (org_id, id),
  CONSTRAINT work_apps_decided_whole CHECK ((decided_by IS NULL) = (decided_at IS NULL)),
  CONSTRAINT work_apps_pending_undecided CHECK ((state = 'pending') = (decided_at IS NULL))
);
CREATE UNIQUE INDEX work_apps_one_live ON work_apps (org_id, client_id) WHERE state IN ('pending', 'approved');

-- Both run as the owner, before hub.org_id is known (the caller holds only a
-- token's tid, oid and azp), as §10.4's resolve_principal does. Neither inserts.
CREATE FUNCTION app_for_client(p_tenant uuid, p_client uuid)
RETURNS TABLE (org_id text, app_id text, scopes text[])
LANGUAGE sql SECURITY DEFINER SET search_path = hub, pg_temp AS $$
  SELECT a.org_id, a.id, a.scopes FROM work_apps a JOIN orgs o ON o.id = a.org_id
   WHERE o.entra_tenant_id = p_tenant AND a.client_id = p_client AND a.state = 'approved'
$$;
CREATE FUNCTION find_principal(p_tenant uuid, p_oid uuid)
RETURNS TABLE (org_id text, user_id text)
LANGUAGE sql SECURITY DEFINER SET search_path = hub, pg_temp AS $$
  SELECT u.org_id, u.id FROM users u JOIN orgs o ON o.id = u.org_id
   WHERE o.entra_tenant_id = p_tenant AND u.entra_oid = p_oid
$$;
REVOKE ALL ON FUNCTION app_for_client(uuid, uuid), find_principal(uuid, uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION app_for_client(uuid, uuid), find_principal(uuid, uuid) TO hub_app;

-- ── 160, 161, 162: placed here as written ────────────────────────────────────

-- ── task 145 (163's columns): a run's outcome, closed once ───────────────────
ALTER TABLE work_loop_runs
  ADD COLUMN issue_key text CHECK (issue_key IS NULL OR issue_key ~ '^[A-Z][A-Z0-9]{1,9}-[1-9][0-9]*$'),
  ADD COLUMN turn_id   text CHECK (turn_id IS NULL OR turn_id LIKE 'ot-%'),
  ADD COLUMN started   jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(started) = 'array'),
  ADD COLUMN ended_at  timestamptz,
  ADD CONSTRAINT work_loop_runs_outcome_known CHECK (outcome IN
    ('queued', 'ran', 'skipped_budget', 'skipped_paused', 'refused_bound', 'failed')),
  ADD CONSTRAINT work_loop_runs_open_iff_queued CHECK ((outcome = 'queued') = (ended_at IS NULL));
CREATE UNIQUE INDEX work_loop_runs_request ON work_loop_runs (loop_id, request_id) WHERE request_id IS NOT NULL;
GRANT UPDATE (outcome, turn_id, started, ended_at) ON work_loop_runs TO hub_app;

CREATE FUNCTION guard_loop_run() RETURNS trigger
LANGUAGE plpgsql SET search_path = hub, pg_temp AS $$
BEGIN
  IF TG_OP = 'DELETE' OR OLD.outcome <> 'queued'
     OR (NEW.org_id, NEW.loop_id, NEW.run, NEW.version, NEW.trigger_kind, NEW.fired_at)
        IS DISTINCT FROM (OLD.org_id, OLD.loop_id, OLD.run, OLD.version, OLD.trigger_kind, OLD.fired_at) THEN
    RAISE EXCEPTION 'hub: work: a closed loop run is never rewritten' USING ERRCODE = 'insufficient_privilege';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER work_loop_runs_guard BEFORE UPDATE OR DELETE ON work_loop_runs
  FOR EACH ROW EXECUTE FUNCTION guard_loop_run();

-- ── task 145 (143 decision 5): one agent policy per team ─────────────────────
CREATE TABLE work_team_agent_policies (
  org_id         text        NOT NULL REFERENCES orgs (id),
  team_id        text        PRIMARY KEY,
  harnesses      text[]      NOT NULL CHECK (cardinality(harnesses) BETWEEN 1 AND 16),
  skills_source  text        NOT NULL DEFAULT 'image' CHECK (skills_source IN ('image', 'library')),
  skills_repo    text,
  skills_path    text        CHECK (skills_path IS NULL OR octet_length(skills_path) <= 400),
  skills_commit  text        CHECK (skills_commit IS NULL OR skills_commit ~ '^[0-9a-f]{40}$'),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  updated_by     text        NOT NULL,
  FOREIGN KEY (org_id, team_id)    REFERENCES teams (org_id, id),
  FOREIGN KEY (org_id, updated_by) REFERENCES users (org_id, id),
  CONSTRAINT work_team_skills_library_whole CHECK ((skills_source = 'library') =
    (skills_repo IS NOT NULL AND skills_path IS NOT NULL AND skills_commit IS NOT NULL))
);
```

`harnesses` is not checked against the catalogue here: the catalogue is the control plane's
(`control-plane/internal/harness/harness.go:216`, checked 2026-09-26) and changes with a release,
so a `CHECK` would need a migration per harness. The route checks it (task 143's
`ValidateTeamAgentPolicy`), and a team with no row reads `["claude-code"]` (`task-143.md:562`).
The scope's `work_harness_allow` and `work_team_skills` are this one table (143 decision 5).

#### `NNNN_work_w5.sql`, continued: row-level security, append-only and grants (complete)

Checked in each owner's block on 2026-09-26: 146, 147, 148, 149 (its own loop), 154, 155, 160 and
161 enable row-level security on their tables themselves; **150 and 162 do not** (*"RLS
org_isolation on all three, as task 149's loop does"*, `task-150.md:146`; *"the four tables get
task 116's RLS form"*, `task-162.md:213`). This loop covers theirs and this task's, and no table
twice, so no `CREATE POLICY` meets an existing one.

```sql
-- ── task 145: org isolation on every W5 table no owner block covers ──────────
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY[
      'work_tracker_connections', 'work_external_links', 'work_sync_runs',          -- 150
      'work_host_hooks', 'work_outside_pr_links', 'work_outside_commit_links',      -- 145
      'work_webhooks', 'work_webhook_deliveries',                                   -- 145 (156)
      'work_teams_bindings', 'work_teams_pairings', 'work_request_messages',        -- 145 (157)
      'work_apps', 'work_team_agent_policies',                                      -- 145
      'work_loops', 'work_loop_versions', 'work_loop_runs', 'work_loop_cursors'     -- 162
  ] LOOP
    EXECUTE format('ALTER TABLE hub.%I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('CREATE POLICY org_isolation ON hub.%I TO hub_app
                    USING (org_id = current_setting(''hub.org_id'', true))
                    WITH CHECK (org_id = current_setting(''hub.org_id'', true))', t);
  END LOOP;
END $$;

-- ── append-only: task 07's loop, for this task's three logs ──────────────────
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['work_outside_commit_links', 'work_webhook_deliveries', 'work_request_messages'] LOOP
    EXECUTE format('CREATE TRIGGER %I BEFORE UPDATE OR DELETE ON hub.%I
                    FOR EACH ROW EXECUTE FUNCTION hub.forbid_change()', t || '_append_only', t);
    EXECUTE format('CREATE TRIGGER %I BEFORE TRUNCATE ON hub.%I
                    FOR EACH STATEMENT EXECUTE FUNCTION hub.forbid_change()', t || '_no_truncate', t);
  END LOOP;
END $$;

-- ── grants: §10.5's shape; SELECT and INSERT, then only the columns that move ──
GRANT SELECT, INSERT ON work_host_hooks, work_outside_pr_links, work_outside_commit_links,
                        work_webhooks, work_webhook_deliveries, work_teams_bindings,
                        work_teams_pairings, work_request_messages, work_apps,
                        work_team_agent_policies TO hub_app;
GRANT UPDATE (remote_url, secret_ref, secret_version)                ON work_host_hooks          TO hub_app;
GRANT UPDATE (pr_state, merge_commit, word, updated_at)              ON work_outside_pr_links    TO hub_app;
GRANT UPDATE (state, secret_ref, secret_version, prev_secret_version, rotated_at,
              cursor_seq, consecutive_failures)                      ON work_webhooks            TO hub_app;
GRANT UPDATE (cursor_seq, last_posted_at, members), DELETE           ON work_teams_bindings      TO hub_app;
GRANT UPDATE (used_at, failures)                                     ON work_teams_pairings      TO hub_app;
GRANT UPDATE (state, decided_by, decided_at, reason)                 ON work_apps                TO hub_app;
GRANT UPDATE (harnesses, skills_source, skills_repo, skills_path, skills_commit,
              updated_at, updated_by)                                ON work_team_agent_policies TO hub_app;
-- No UPDATE or DELETE on: work_outside_commit_links, work_webhook_deliveries,
-- work_request_messages (append-only), and no UPDATE of a webhook's url or kinds:
-- a changed webhook is a new one (task 156 has no patch route).
```

**No `FORCE`**, the Hub contract's form (`hub-contract.md:1914-1918`), though task 146 forces its
own table (`task-146.md:147-148`). The policy is `TO hub_app`, so under `FORCE` the owner matches
no policy and reads nothing: `app_for_client`, which runs as the owner before `hub.org_id` is
known, would never find an app. Unforced, the owner still runs only `hub migrate` and the
`SECURITY DEFINER` functions (`hub-contract.md:1929-1930`), and `hub_app` is bound by the policy.

#### `NNNN_work_w5.sql`, continued: the state guard, replaced (complete)

Task 10's body (`task-10.md:168-199`) is kept line for line below the marked W5 block; the block is
the only change. If a later part has replaced `guard_issue_state()` again by sync time, the same
block goes at the top of that body instead, and the tests below decide.

```sql
-- guard_issue_state, replaced (task 145): task 10's function with the W5 block
-- first. CREATE OR REPLACE keeps task 07's trigger bound to the new body.
CREATE OR REPLACE FUNCTION guard_issue_state() RETURNS trigger
LANGUAGE plpgsql SET search_path = hub, pg_temp AS $$
DECLARE
  v_cause  text := coalesce(current_setting('hub.work_cause', true), '');
  v_kind   text := coalesce(current_setting('hub.work_actor_kind', true), '');
  v_user   text := coalesce(current_setting('hub.work_actor_user', true), '');
  v_serves boolean := EXISTS (SELECT 1 FROM issue_serves s
                               WHERE s.issue_id = NEW.id AND s.revision = NEW.head_revision);
  v_fact   boolean := NEW.state IN ('started', 'in_review', 'landed', 'done');
  v_order  text[]  := ARRAY['triage', 'backlog', 'todo', 'started', 'in_review', 'landed', 'done'];
BEGIN
  -- ── W5 (task 145): who, before why ─────────────────────────────────────────
  PERFORM work_actor_consistent();                                    -- decisions 3 and 8
  IF v_kind = 'integration' THEN                                      -- decision 4
    RAISE EXCEPTION 'hub: work: principal_cannot_move_state: integration may not move %', NEW.id
      USING ERRCODE = 'insufficient_privilege';
  END IF;
  IF v_kind = 'host' THEN                                             -- decision 6
    IF (v_cause, NEW.state) NOT IN (('outside_push', 'started'), ('outside_pr', 'in_review'),
                                    ('outside_merge', 'landed'))
       OR coalesce(array_position(v_order, OLD.state), 99) >= array_position(v_order, NEW.state)
       OR NOT EXISTS (SELECT 1 FROM users u WHERE u.org_id = NEW.org_id AND u.id = v_user) THEN
      RAISE EXCEPTION 'hub: work: principal_cannot_move_state: host may not move % into % with "%"',
        NEW.id, NEW.state, v_cause USING ERRCODE = 'insufficient_privilege';
    END IF;
    RETURN NEW;                  -- a host fact passes neither the person rule nor the cause table
  END IF;
  IF v_cause IN ('outside_push', 'outside_pr', 'outside_merge') THEN -- only a host's
    RAISE EXCEPTION 'hub: work: principal_cannot_move_state: % is a host cause', v_cause
      USING ERRCODE = 'insufficient_privilege';
  END IF;
  IF v_fact AND v_kind IN ('app', 'key') THEN                         -- decision 5
    RAISE EXCEPTION 'hub: work: principal_cannot_move_state: % may not move % into %', v_kind, NEW.id, NEW.state
      USING ERRCODE = 'insufficient_privilege';
  END IF;
  -- ── task 10's body, unchanged ───────────────────────────────────────────────
  IF v_fact AND v_kind <> 'person' THEN
    RAISE EXCEPTION 'hub: work: only a person moves % into %', NEW.id, NEW.state
      USING ERRCODE = 'insufficient_privilege';
  END IF;
  IF v_cause IN ('evidence_observed', 'evidence_recorded') AND OLD.state <> 'landed' THEN
    RAISE EXCEPTION 'hub: work: evidence closes a landed issue, and % is %', NEW.id, OLD.state
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
```

`OLD.state` of `cancelled` or `duplicate` has no position in `v_order`, so `coalesce(…, 99)` makes
every host move from them refused: an outside PR never revives a declined issue, the rule task
154's `before` states in Go (`task-154.md:722-725`). A custom state from W2 (task 75) is a human
band state with no position either, and is refused the same way (gap 5).

#### `NNNN_work_w5.sql`, continued: the actor, the created issue and the revision (complete)

PL/pgSQL resolves `work_actor_consistent()` when the trigger runs, not when `guard_issue_state` is
created, so defining it after the replacement above is legal; the file keeps it here, beside its
other two callers.

```sql
-- decisions 3 and 8: a known kind, and the two sync settings agree.
CREATE FUNCTION work_actor_consistent() RETURNS void
LANGUAGE plpgsql SET search_path = hub, pg_temp AS $$
DECLARE
  v_kind text := coalesce(current_setting('hub.work_actor_kind', true), '');
  v_conn text := coalesce(current_setting('hub.work_sync_connection', true), '');
BEGIN
  IF v_kind NOT IN ('', 'person', 'agent', 'janitor', 'integration', 'app', 'key', 'host') THEN
    RAISE EXCEPTION 'hub: work: principal_cannot_move_state: unknown actor kind "%"', v_kind
      USING ERRCODE = 'insufficient_privilege';
  END IF;
  IF (v_conn <> '') <> (v_kind = 'integration') THEN
    RAISE EXCEPTION 'hub: work: principal_cannot_move_state: actor kind "%" with sync connection "%"',
      v_kind, v_conn USING ERRCODE = 'insufficient_privilege';
  END IF;
END $$;

-- An integration creates an issue only in triage, from an outside tracker.
CREATE FUNCTION guard_issue_actor_insert() RETURNS trigger
LANGUAGE plpgsql SET search_path = hub, pg_temp AS $$
BEGIN
  PERFORM work_actor_consistent();
  IF current_setting('hub.work_actor_kind', true) = 'integration'
     AND (NEW.state <> 'triage' OR NEW.external_source IS NULL
          OR NEW.external_source NOT IN ('azure_boards', 'github', 'jira')) THEN
    RAISE EXCEPTION 'hub: work: principal_cannot_move_state: integration creates only in triage, from its tracker'
      USING ERRCODE = 'insufficient_privilege';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER issues_actor_insert_guard BEFORE INSERT ON issues
  FOR EACH ROW EXECUTE FUNCTION guard_issue_actor_insert();

-- decision 7: integration writes revision 1 of an issue it is creating; host writes none.
CREATE FUNCTION guard_issue_revision_actor() RETURNS trigger
LANGUAGE plpgsql SET search_path = hub, pg_temp AS $$
DECLARE v_kind text := coalesce(current_setting('hub.work_actor_kind', true), '');
BEGIN
  PERFORM work_actor_consistent();
  IF v_kind = 'host' THEN
    RAISE EXCEPTION 'hub: work: principal_cannot_move_state: a host writes no issue text'
      USING ERRCODE = 'insufficient_privilege';
  END IF;
  IF v_kind = 'integration' AND (NEW.revision <> 1 OR NOT EXISTS (
       SELECT 1 FROM issues i WHERE i.id = NEW.issue_id AND i.state = 'triage'
          AND i.external_source IN ('azure_boards', 'github', 'jira'))) THEN
    RAISE EXCEPTION 'hub: work: principal_cannot_move_state: integration never revises %', NEW.issue_id
      USING ERRCODE = 'insufficient_privilege';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER issue_revisions_actor_guard BEFORE INSERT ON issue_revisions
  FOR EACH ROW EXECUTE FUNCTION guard_issue_revision_actor();
```

**The two layers, side by side.** Task 150's `guard_sync_principal` refuses on the connection
setting; these refuse on the actor kind; `work_actor_consistent` makes the two settings move
together, so each layer catches what the other would miss only if the store set neither, which is
a person's write. The revision rule relies on `(issue_id, revision)` being unique: revision 1 of
an existing issue already exists, so the insert fails on the key before it could revise.

**What the guards do not cover:** a comment. Mode `in` versus `in_and_comments` is a property of
one connection, so the refusal of an integration comment in mode `in` is Go's (task 153), with
task 153's `issue_comments` columns marking a crossed comment (`task-153.md:140-160`).

#### The matrix: `server/work_matrix_rows_integration_test.go` ⚑, appended (complete)

A principal can authenticate only once its owner's route task has landed (150 the tracker
principal, 159 the app token, 160 the keys, 154 the host signature). So a `principalRow` is live
when **both** `"145"` and its principal's owner are in `workMatrixLive` (task 07 decision 6's
switch, `task-07.md:41-47`), and 145 lands every row dark.

```go
// principal is a W5 caller that is not one of §5.3's six columns (decision 11).
type principal string

const (
	asIntegration principal = "integration" // a tracker connection's principal (task 150)
	asAppForM     principal = "app_for_M"   // an approved app acting for M (task 159)
	asPersonalKey principal = "key_M"       // M's personal key (task 160)
	asTeamKey     principal = "key_team"    // a key scoped to the fixture's team (task 160)
	asHost        principal = "host"        // an outside host's signed webhook (task 154)
)

// principalOwner is the task whose commit makes the principal able to call.
var principalOwner = map[principal]string{asIntegration: "150", asAppForM: "159",
	asPersonalKey: "160", asTeamKey: "160", asHost: "154"}

// principalRow is one principal, one route, one status. H16b's sender sends it
// with f.credential(p) (the builder below) and wants exactly want and code.
type principalRow struct {
	name      string
	principal principal
	method    string
	path      string // H16b's placeholders, filled from the fixture as row.path is
	body      func(*fixture) any
	want      int
	code      string // "" when want is 2xx
}

func move(to, cause string) func(*fixture) any {
	return func(*fixture) any { return map[string]any{"to": to, "cause": cause, "reason": "matrix"} }
}

const pcms = "principal_cannot_move_state"

// w5GuardRows are SW.3's rows for W5 (the scope's "guard rows"). The route is
// task 10's; the fixture issue is in todo, and serves nothing.
var w5GuardRows = []principalRow{
	{"sync_hand_backlog", asIntegration, "POST", "/v1/issues/{issueId}/state", move("backlog", "hand"), 403, pcms},
	{"sync_started", asIntegration, "POST", "/v1/issues/{issueId}/state", move("started", "lane_bound"), 403, pcms},
	{"sync_done_forged", asIntegration, "POST", "/v1/issues/{issueId}/state", move("done", "evidence_observed"), 403, pcms},
	{"sync_revision", asIntegration, "POST", "/v1/issues/{issueId}/revisions", reviseBody, 403, pcms},
	{"sync_comment", asIntegration, "POST", "/v1/issues/{issueId}/comments", commentBody, 403, "forbidden"},
	{"app_hand_backlog", asAppForM, "POST", "/v1/issues/{issueId}/state", move("backlog", "hand"), 200, ""},
	{"app_started", asAppForM, "POST", "/v1/issues/{issueId}/state", move("started", "lane_bound"), 403, pcms},
	{"app_done_by_hand", asAppForM, "POST", "/v1/issues/{issueId}/state", move("done", "hand"), 403, pcms},
	{"key_hand_backlog", asPersonalKey, "POST", "/v1/issues/{issueId}/state", move("backlog", "hand"), 200, ""},
	{"key_landed", asPersonalKey, "POST", "/v1/issues/{issueId}/state", move("landed", "land_trailer"), 403, pcms},
	{"team_key_done", asTeamKey, "POST", "/v1/issues/{issueId}/state", move("done", "evidence_recorded"), 403, pcms},
	{"person_host_cause", "", "POST", "/v1/issues/{issueId}/state", move("in_review", "outside_pr"), 403, pcms}, // M, no principal
}

func init() {
	for _, r := range w5GuardRows {
		if live("145") && (r.principal == "" || live(principalOwner[r.principal])) {
			allPrincipalRows = append(allPrincipalRows, r)
		}
	}
}

func live(id string) bool { return slices.Contains(workMatrixLive, id) }
```

`reviseBody` and `commentBody` are task 07's W1 row bodies (the smallest valid revision and
comment, **UNVERIFIED** names). `allPrincipalRows` is new beside H16b's `allRows`, run by one new
loop in H16b's file (⚑ below); the host principal has no `/v1` row here because a host never
calls `/v1`: task 154 appends its `/hooks/…` rows to `w5GuardRows`' sibling slice.

#### `server/matrix_integration_test.go` ⚑ (H16b), additive only (complete)

H16b's `fixture` takes fields *"added by later tasks, never renamed"* (task 07's reading of H16b
`:45`, **UNVERIFIED** until H16b's file is on this branch). The additions:

```go
// fixture, W5 fields (task 145). Seeded as hub_owner, not through routes, so
// they exist before tasks 150, 159 and 160 register theirs.
W5Connection string // a tc- row in the fixture's project, mode in
W5AppID      string // an oa- row, approved, acting for M
W5KeyM       string // M's personal key, the secret as issued (hashed in the row)
W5KeyTeam    string // a key scoped to the fixture's team
W5HostSecret string // the fixture project's GitHub hook secret, for task 154's signature

// credential is how a principal row authenticates. Each case names the task
// whose middleware reads it; until that task is live its rows never run.
func (f *fixture) credential(p principal) (header, value string) {
	switch p {
	case asIntegration:
		return "Authorization", "Bearer " + f.syncToken(f.W5Connection) // task 150's principal token
	case asAppForM:
		return "Authorization", "Bearer " + f.idp.AppToken(f.W5AppID, f.MemberOID) // task 159: azp + oid
	case asPersonalKey:
		return "Authorization", "Bearer " + f.W5KeyM // task 160's key form
	case asTeamKey:
		return "Authorization", "Bearer " + f.W5KeyTeam
	}
	return "Authorization", "Bearer " + f.MemberToken // "" is M, the person
}
```

The builder gains one call, `buildW5Fixture(t, f)`, after task 07's `buildWorkFixture`. It inserts
the four rows as `hub_owner` and gives **each principal row a fresh issue in `todo`**
(`f.freshIssue(t)`), so a row that moves its issue never moves the next row's. The runner is one
loop beside H16b's, over `allPrincipalRows`, sending `f.credential(r.principal)` and asserting
`want` and, when set, the problem `code`. `syncToken`, `idp.AppToken`, `MemberOID` and
`MemberToken` are the owner tasks' and H16b's names, all **UNVERIFIED**; each owner binds its own
case when it goes live.

#### `migrate/work_guards_integration_test.go` ⚑ (task 07): the one clause that flips

```go
// TestAnIntegrationNeverWritesAFact, last clause (task-07.md:510): was "integration and
// hand → ok for backlog"; W5 refuses every state write by an integration (task 145 decision 4).
wantRaise(t, asIntegration(tx), "UPDATE issues SET state = 'backlog' WHERE id = $1", "principal_cannot_move_state")
```

`asIntegration` sets `hub.work_actor_kind = 'integration'` **and** `hub.work_sync_connection`
(decision 8); `wantRaise` is task 07's helper shape (**UNVERIFIED** name).

#### Tests first — `migrate/work_w5_integration_test.go`

On task 06's `seedWork`, `asApp` and `insertIssue` (`task-06.md:440-452`), and its `pgcode`
(`:455`): a test matches the SQLSTATE and the `principal_cannot_move_state` prefix, never the
rest of the text. Two are complete; the rest follow the table in the same shape.

```go
//go:build integration

package migrate_test

// asActor is asApp with the W5 settings set first (decision 3): kind, the
// attributed user, the cause, and the sync connection when kind is integration.
func asActor(t *testing.T, db *hubtest.DB, o workOrg, kind, user, cause string, fn func(tx pgx.Tx) error) error {
	t.Helper()
	return asApp(t, db, o.ID, func(tx pgx.Tx) error {
		conn := ""
		if kind == "integration" {
			conn = seedTracker(t, tx, o) // a tc- row in o.Project, mode in
		}
		for k, v := range map[string]string{"hub.work_actor_kind": kind, "hub.work_actor_user": user,
			"hub.work_cause": cause, "hub.work_sync_connection": conn} {
			if _, err := tx.Exec(ctx, "SELECT set_config($1, $2, true)", k, v); err != nil {
				return err
			}
		}
		return fn(tx)
	})
}

// kills "an app closes an issue" and "the guard trusts a forged cause".
func TestAnAppNeverWritesAFact(t *testing.T) {
	db := hubtest.New(t)
	s := seedWork(t, db)
	for _, c := range []struct{ state, cause string }{{"started", "lane_bound"}, {"in_review", "lane_queued"},
		{"landed", "land_trailer"}, {"done", "evidence_observed"}, {"done", "hand"}} {
		for _, kind := range []string{"app", "key"} {
			err := asActor(t, db, s.A, kind, s.A.Member, c.cause, func(tx pgx.Tx) error {
				id := insertIssue(t, tx, s.A, s.A.Project, 1)
				_, err := tx.Exec(ctx, "UPDATE issues SET state = $1, state_reason = 'r' WHERE id = $2", c.state, id)
				return err
			})
			if pgcode(err) != "42501" || !strings.Contains(err.Error(), "principal_cannot_move_state") {
				t.Errorf("%s into %s by %s: %v", kind, c.state, c.cause, err)
			}
		}
	}
	err := asActor(t, db, s.A, "key", s.A.Member, "hand", func(tx pgx.Tx) error { // decision 5
		_, err := tx.Exec(ctx, "UPDATE issues SET state = 'backlog' WHERE id = $1", insertIssue(t, tx, s.A, s.A.Project, 2))
		return err
	})
	if err != nil {
		t.Fatalf("a key's hand move to backlog refused: %v", err)
	}
}
```

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestW5AppliesOnEmptyAndOnW4` | `hub migrate` from zero, and from W4's last migration with seeded rows, both reach the same `pg_dump --schema-only` | "depends on unapplied state" |
| `TestEveryOrgTableHasOrgIsolation` (task 06's, re-run) | every W5 table with `org_id` has the policy; as `X` (org B set) every W5 table reads 0 rows | "RLS missing on a table" |
| `TestAnIntegrationWritesNoState` | kind `integration`: `backlog` by `hand` and `done` by `evidence_observed` → `42501` | "sync may write a human state" |
| `TestAnAppNeverWritesAFact` | above | "an app closes an issue" |
| `TestAHostMovesForwardOnly` | `host`, a member user: `outside_merge` → `landed` from `in_review` ok; `done` → `42501`; `outside_pr` from `landed` → `42501`; from `cancelled` → `42501` | "host writes done"; "a host move backwards" |
| `TestAHostNeedsAPerson` | `host`, `hub.work_actor_user` unset, then a user of org B: `started` by `outside_push` → `42501` both | "a fact with no person" |
| `TestAPersonCannotForgeAHostCause` | `person`, `outside_pr` → `in_review` → `42501` | "outside cause open to all" |
| `TestTheSyncSettingsAgree` | connection set with kind `person` → `42501`; kind `integration` with no connection → `42501`; kind `robot` → `42501` | decision 8; "an unknown kind reads as a person" |
| `TestIntegrationRevisesNothing` | kind `integration`: revision 1 of a new `triage` issue from `jira` ok; revision 2 → `42501`; revision 1 of a `backlog` issue → `42501`; `host`, any revision → `42501` | "sync revises an issue" |
| `TestTheQuoteIsFrozen` | as `hub_app`, `UPDATE work_requests SET quote = 'x'` and `SET requester_email = 'x'` → `42501` each (no column grant); `erase_request` blanks both | "mutable quote" |
| `TestNoSecretAtRest` | issue a key through task 160's insert path; no column of `work_api_keys`, `work_webhooks` or `work_host_hooks` equals the issued secret (every `text` and `bytea` column compared) | "plaintext key at rest" |
| `TestW5LogsAreAppendOnly` | `UPDATE` and `DELETE` on `work_outside_commit_links`, `work_webhook_deliveries`, `work_request_messages` → `42501` each | "a log rewritten" |
| `TestAClosedLoopRunIsFrozen` | a run `ran` with `ended_at` set; `UPDATE … SET outcome = 'failed'` → `42501`; a `queued` run → `ran` ok | "a closed run rewritten" |
| `TestW5GrantsAreExactlyTheList` | `role_table_grants` and `column_privileges` for `hub_app` on this task's ten tables equal the grants block | "a grant nobody reviewed" |
| `TestAppLookupIgnoresPending` | `app_for_client` returns the approved app and nothing for a pending or revoked one; as `hub_app` it is callable, as `PUBLIC` not | "an unapproved app resolves" |

The quote's column is `quote` (`task-146.md:109`, checked); `requester_email` is **UNVERIFIED**:
Step 1 reads task 146's requester columns and names each one the test updates. Task 146's grant
(`task-146.md:153`) moves only `important`, `state`, `issue_id`, `decline_reason`, `closed_at` and
`closed_by`, so the freeze is the grant plus the owner-only `erase_request`, not the scope's
`forbid_change()` on the columns; the test pins that form.

#### Steps

1. **Sync base (rules H1, H2).** Confirm tasks 07, 10 and 143 are merged and read the numbering
   in `migrate/sql/`; take the next free `NNNN`. Re-read each owner block in decision 1's table at
   its cited lines (a part may have moved since 2026-09-26) and bind the **UNVERIFIED** names:
   task 146's requester columns, task 07's `wantRaise`, H16b's `fixture` and sender. 20 minutes.
2. **Write the tests first** (the table above, `work_w5_integration_test.go`), and flip task 07's
   one clause. Run them: every one fails on a missing table, function or trigger. 45 minutes.
3. **Write `NNNN_work_w5.sql`**: the owner blocks in order, then this part's blocks in the order
   they appear here, the guards last. `hub migrate` against the compose stack. 50 minutes.
4. **Run the tests to green**, then task 06's and 07's migrate tests unchanged but for the one
   flipped clause, and task 10's. 20 minutes.
5. **The matrix**: append `principalRow`, `w5GuardRows` and the runner loop; add `"145"` to
   `workMatrixLive`. With 150, 159 and 160 not yet live, no principal row runs except
   `person_host_cause`. 25 minutes. Then commit each file.

#### Acceptance

```bash
cd control-plane
go run ./cmd/hub migrate --dsn "$HUB_TEST_DSN" && echo migrated                                 # migrated
go test -tags integration ./internal/hub/migrate/ -count=1 -v \
  -run 'Test(W5AppliesOnEmptyAndOnW4|AnIntegrationWritesNoState|AnAppNeverWritesAFact|AHostMovesForwardOnly|AHostNeedsAPerson|APersonCannotForgeAHostCause|TheSyncSettingsAgree|IntegrationRevisesNothing|TheQuoteIsFrozen|NoSecretAtRest|W5LogsAreAppendOnly|AClosedLoopRunIsFrozen|W5GrantsAreExactlyTheList|AppLookupIgnoresPending)$' \
  2>&1 | grep -c '^--- PASS'                                                                    # 14
go test -tags integration ./internal/hub/migrate/ -count=1 -run 'Test(EveryOrgTableHasOrgIsolation|AnIntegrationNeverWritesAFact|AnUnsetActorIsNotAPerson)$' 2>&1 | tail -1   # ok
go test -tags integration ./internal/hub/server/ -count=1 -run 'TestMatrix' -v 2>&1 | grep -c 'principal/person_host_cause.*PASS'   # 1
grep -c 'FORCE ROW LEVEL' internal/hub/migrate/sql/*_work_w5.sql                                # 3: tasks 146-148's own (checked 2026-09-26); none from this part's loop
```

**Expected, not observed**: no W5 migration exists on this branch on 2026-09-26, and none of these
commands has run (OD-27). `$HUB_TEST_DSN` and `cmd/hub migrate` are Phase H's names
(**UNVERIFIED**; H's plan fixes them).

#### What this task does NOT claim

- The migration number: it is the next free one at sync time (rule H1).
- That any principal authenticates: tasks 150, 159, 160 and 154 own the credentials, and each
  switches its rows on. Until then only the person row runs.
- That the owner blocks are right: each owner's tests prove its table; this task proves only that
  they apply together and that the guards hold across them.
- That `host` causes are in 05 §3.3's cause table: they extend it (the gap task 154 noted).
- That any host identity maps to an Entra user: that is task 154's mapping.
- That Key Vault is available to the Hub: it is a **proposed** contract addition (decision 10).
- Any parity row alone. It pins the authorisation behind **L-202, L-215, L-244, L-245 and L-250**
  (05 §1), which their route tasks close.

#### Gaps found while writing this part

1. **Keys and state: task 160 disagrees.** Task 160 decision 7 and `TestKeysNeverMoveState`
   (`task-160.md:57`, `:475`) refuse a key every state write, on the scope's sentence; this part
   (decision 5) and task 144 (decision 3, row W5-12) let an app or a key make a person's `hand`
   moves and refuse only facts and `done`. One side must change before 160 builds; this part
   recommends 160's, for L-245 and L-250.
2. **Labels on a synced issue: task 150 and task 144 disagree.** Task 150's guard refuses any
   `issue_labels` row by the sync principal (`task-150.md:175-178`); task 144's fake
   `ApplySyncBatch` matches labels through task 05's `insertImported` (144 decision 10). The fake
   should pass no labels, or 150's guard should allow matched ones.
3. **The file name.** The scope names `NNNN_work_intake.sql`; nine owners wrote
   `NNNN_work_w5.sql` and two (`task-161.md:440`, `task-162.md:213`) `_work_intake`. This part
   takes `_work_w5` (decision 2); 161 and 162 should re-point.
4. **A request's thread survives erase.** `erase_request` (`task-146.md`, its block) blanks the
   request, not `work_request_messages.body`, so an erased requester's replies stay. Task 146
   (or 157, which writes the thread) should extend the erase; this part adds no erase path.
5. **Custom states have no host order.** W2's custom states (task 75) are not in `v_order`, so a
   host move from one is refused. That is safe (default deny), and may be what the owner wants;
   it is not decided anywhere.
6. **Key Vault is a Hub-contract proposal with no task.** The process has no Key Vault access
   (`hub-contract.md:2085`); tasks 151, 152, 154 and 156 need it to read a `secret_ref`. By rule
   H1 it is a Hub delta for the `feat/hub` owner, and the skeleton has no W5 Hub-delta task for it.
7. **Task 06's closed-set test flips.** `TestClosedSetsRefuseStrangers` asserts
   `external_source = 'jira'` is refused (`task-06.md:468`); task 150's `ALTER` widens that set,
   so that clause must flip in 150's commit, not this one.
8. **`work_teams_pairings` under row-level security.** A pairing code is redeemed from a Teams
   message, perhaps before the Hub knows the org; if so, task 157 needs a `SECURITY DEFINER`
   lookup like `app_for_client`. **UNVERIFIED**: 157's flow decides.
9. **The skeleton's row for 145** names seam SW.3 only; this part also adds `hub.work_actor_user`
   and the matrix's `principalRow`, which 150, 154, 159 and 160 consume.
