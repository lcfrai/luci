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
