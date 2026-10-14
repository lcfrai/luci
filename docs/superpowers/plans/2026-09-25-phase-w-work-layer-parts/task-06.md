### Task 06: Migration, part 1: the Work tables, row-level security and grants

Stream **WH**, no seam of its own, about 3 hours (an estimate, not a measurement). Depends on task
03 (the enums whose sets become `CHECK`s here) and on Hub tasks **H06** (the migrate runner and
`hubtest`), **H07a** (`0001_init.sql`'s tables, row-level security and grants) and **H07b** (the
guards and owner functions), all sync-merged from `feat/hub` into `feat/work-layer` before this
task starts (rule H1). The implementer never merges.

This task writes the storage every W1 Hub route stands on, and nothing that reads or writes it. It
is one SQL file and one database test. The routes come in tasks 08-15; the guards that make a fact
state unwritable by hand come in task 07, appended to the same file.

**Six decisions, made here so no later task re-derives them.**

1. **No Hub table is altered.** The project key and the issue counter get their own tables
   (`project_keys`, `project_issue_counters`), so `projects` keeps exactly the grant §10.5 gives it
   (`UPDATE (name)`, hub-contract `:2016`) and no Hub test that reads `projects` changes. A WH task
   is additive only (rule H1).
2. **Every foreign key carries `org_id`, at the same position as its target's.** Hub review I5
   (task 07a, `:97-100`, and its `TestEveryForeignKeyCarriesTheOrg`, `:530`) made this the rule for
   `0001_init.sql`: Postgres checks a foreign key without row-level security, so a single-column
   link lets an org-A row point at an org-B parent. The Hub's catalog test reads **every** foreign
   key in schema `hub`, so it checks this migration's keys too, with no edit to the Hub's file. Each
   new parent table carries `UNIQUE (org_id, id)` as the target.
3. **The issue's project is pinned in the key, not in Go.** `issues` carries
   `UNIQUE (org_id, id, project_id)`, and `issue_revisions` and `issue_attachments` reference it
   with `(org_id, issue_id, project_id)`. A revision or an attachment filed under the wrong project
   is then a `23503` from the database, whatever the handler does.
4. **Enumerations are `text` with a named `CHECK`, never `CREATE TYPE … AS ENUM`.** The Hub uses
   enums for sets that never grow (`0001_init.sql`, hub-contract `:1468-1477`). The Work sets do
   grow: W2 adds custom states inside the human bands (task 75), W3 adds comment kinds (task 99)
   and W5 adds sync sources (task 150). A named constraint is replaced with one `ALTER TABLE … DROP
   CONSTRAINT … , ADD CONSTRAINT …` inside a later migration's transaction; `ALTER TYPE … ADD VALUE`
   cannot be used in the transaction that adds it. The constraint names are listed below and are
   part of this task's interface.
5. **`head_revision` starts at 0.** A head row must exist before its first revision (the revision's
   key references it), so the store inserts the head at 0, then revision 1, then moves the head to 1
   in the same transaction, exactly as briefs do (`briefs.head_revision DEFAULT 0`, H07a `:181`).
   Task 07's guard pins "one step forward, onto a revision that exists".
6. **The migration takes the next free number at sync time.** On the Hub plan at this writing,
   `0001_init.sql` is the only migration (`Latest()` is 1, H07a `:523`), and Phase T's Hub delta
   (TM-05) may take the next number first. The file is written below as `NNNN_work_w1.sql`; the
   implementer names it `<max+1, four digits>_work_w1.sql` from `ls migrate/sql/` on the sync base
   and records the number in the commit message. Tasks 07 and 10 **append** to this same file before
   it is applied anywhere shared (the runner refuses an edited, applied migration: `ErrEdited`, H06
   `:54`). Once task 06 is merged into `feat/hub`, 07 and 10 take a new number instead.

**Two Hub tests this migration touches without editing.** Both are Hub files; neither is edited
here (rule H1: the Hub plan's file wins whole).

- `TestEveryForeignKeyCarriesTheOrg` (H07a `:530`) reads every foreign key in schema `hub`. This
  migration is written to pass it; a red there is a finding against this task's SQL.
- `TestHubAppGrantsAreExactlyTheList` (H07a `:532`) compares `hub_app`'s grants with §10.5's list.
  If it reads every table in the schema rather than a named list, it goes red as soon as this
  migration is embedded, because the Work tables add grants. Whether it is scoped is **UNVERIFIED**
  (its body is not on `feat/hub` at this writing). If it is not, the fix is a proposed edit to the
  Hub owner (the route task 01 uses) that scopes it to `0001`'s tables, and this task's own
  `TestWorkGrantsAreExactlyTheList` covers the Work tables. It is listed as a skeleton gap.

**One Hub process this migration constrains.** The Hub's janitor deletes old `project_blobs` that
nothing references (H task 24, `task-24.md:93-96`). An attached blob is referenced by
`issue_attachments`, whose key makes the janitor's `DELETE` fail with `23503` rather than lose the
file. The janitor's reference list must name `issue_attachments` so the sweep skips it instead of
failing on it. That list is Hub task 24's; the addition is a proposed edit, **UNVERIFIED** until
the Hub owner takes it.

**Files:**
- `control-plane/internal/hub/migrate/sql/NNNN_work_w1.sql` — the fourteen Work tables, their
  row-level security and their grants (new; tasks 07 and 10 append to it)
- `control-plane/internal/hub/migrate/work_w1_integration_test.go` — `//go:build integration`; the
  database tests below, and `seedWork`, the Work seed helper tasks 07-15 reuse (new)

No Go outside the test. `migrate.Embedded()` (H07a) embeds `sql/*.sql`, so the new file is applied
by `hub migrate` and by `hubtest.Migrated` with no code change; a runner that needs a list edited
is a finding against H07a, not a reason to edit it here.

**Interfaces:**
- Produces (the schema, consumed by tasks 07-15 and every later Work migration):
  - the tables `project_keys`, `project_issue_counters`, `issues`, `issue_revisions`,
    `issue_serves`, `labels`, `issue_labels`, `issue_relations`, `team_work_policies`,
    `issue_comments`, `comment_mentions`, `comment_reactions`, `issue_subscriptions`,
    `issue_attachments`, with the columns in the SQL below, byte for byte
  - the named constraints a later slice replaces: `issues_state_known` (W2, task 75),
    `issues_priority_known`, `issues_not_tied_reason_known`, `issues_source_known`,
    `issues_external_source_known` (W5, task 150), `issue_comments_kind_known` (W3, task 99),
    `issue_relations_kind_known`, `issue_subscriptions_cause_known`,
    `team_work_policies_default_state_known`
  - the named constraints Go matches on: `issues_project_number_key` (task 08's gapless-number
    test), `issues_external_once` (task 15's idempotency, SW.6), `project_keys_team_key_key`
    (task 08's 409 on a key already used in the team), `labels_team_live_name` (task 11)
  - `seedWork(t, db) workSeed`, in the test file: two orgs, each with a team, a project, a member,
    a viewer and a project-scope brief at revision 1, written in the order the Hub writes (H07a
    `:512-519`), so task 07's guards accept the same seed
- Consumes:
  - H07a: `orgs`, `users`, `teams`, `team_members`, `projects`, `briefs`, `project_blobs`, each
    with its `UNIQUE (org_id, id)` (or `(org_id, project_id, digest)`) target; the seed helper in
    `schema_integration_test.go`
  - H06: `hubtest.Migrated(t)`, `(*DB).AppPool`, `(*DB).OwnerConn`, `migrate.Apply`
  - task 03: the value sets of `IssueState`, `Priority`, `RelationKind`, `NotTiedReason`,
    `IssueSource`, `SubscriptionCause`, and `ImportBatch.source`; `AuthoredText.author` (`H`, `A`).
    **The SQL's sets must equal task 03's Go sets**; task 07's `TestWorkCheckSetsMatchHubapi`
    compares them (a Go constant added without a migration is caught there, not in production)

**The shape of each table, and why.**

| Table | Key | Mutable by `hub_app` | Why it exists |
|---|---|---|---|
| `project_keys` | `project_id` | `key`, `set_at`, `set_by` | `EP` in `EP-142`; changeable by a team admin, so never part of a key (05 §3.6) |
| `project_issue_counters` | `project_id` | `last_number` | gapless numbers under a row lock; a sequence leaves gaps on rollback (task 08) |
| `issues` | `id` (`is-`) | state, reason, assignee, priority, head, not-tied reason, `updated_at` | the mutable head (skeleton rule 1) |
| `issue_revisions` | `(issue_id, revision)` | nothing (erase only, task 07) | the immutable, digested `issue:is-…@r` |
| `issue_serves` | `(issue_id, revision, brief_key, criterion_id)` | nothing | an index of each revision's `serves`, so the lens groups by criterion without parsing bytes |
| `labels` | `id` (`lb-`) | `name`, `color`, `archived_at` | team labels (L-17) |
| `issue_labels`, `issue_relations`, `comment_reactions` | composite | `DELETE` only | the three toggles |
| `team_work_policies` | `team_id` | every column but the key | the team's Work settings (L-9, L-48, the `stopAtLandedWithoutServes` switch) |
| `issue_comments` | `id` (`ic-`) | `resolved_at`, `resolved_by` | comments with an author class and an anchor (L-41, L-42, L-124) |
| `comment_mentions` | `(comment_id, user_id)` | nothing | who a comment named, at write time (L-117) |
| `issue_subscriptions` | `(issue_id, user_id)` | `subscribed`, `cause`, `updated_at` | who hears about an issue (L-117) |
| `issue_attachments` | `(issue_id, digest)` | nothing | an attached project blob, by digest (L-45) |

`erased_at` on `issues`, `issue_revisions` and `issue_comments` is **not** granted: only task 07's
owner functions set it. `issue_proofs` is task 10's table, appended by task 10; it is not here.

#### The migration — `migrate/sql/NNNN_work_w1.sql`

Complete. Ids carry their grammar as `CHECK`s, as `0001_init.sql` does (hub-contract `:1461-1462`),
so a bug in Go cannot store a malformed one. Digests are raw 32-byte `bytea`.

```sql
-- NNNN_work_w1.sql — Phase W, W1: the Work tables. Applied by `hub migrate` as hub_owner,
-- in one transaction, after 0001_init.sql. Additive only: no Hub table is altered.
SET LOCAL search_path = hub;

-- ── the project key and the issue counter ────────────────────────────────────
CREATE TABLE project_keys (
  org_id     text        NOT NULL REFERENCES orgs (id),
  project_id text        NOT NULL PRIMARY KEY,
  team_id    text        NOT NULL,
  key        text        NOT NULL CHECK (key ~ '^[A-Z]{2,5}$'),   -- hubapi.ValidProjectKey
  set_at     timestamptz NOT NULL DEFAULT now(),
  set_by     text        NOT NULL,
  CONSTRAINT project_keys_team_key_key UNIQUE (org_id, team_id, key),
  FOREIGN KEY (org_id, project_id) REFERENCES projects (org_id, id),
  FOREIGN KEY (org_id, team_id)    REFERENCES teams (org_id, id),
  FOREIGN KEY (org_id, set_by)     REFERENCES users (org_id, id)
);

CREATE TABLE project_issue_counters (
  org_id      text    NOT NULL REFERENCES orgs (id),
  project_id  text    NOT NULL PRIMARY KEY,
  last_number integer NOT NULL DEFAULT 0 CHECK (last_number >= 0),
  FOREIGN KEY (org_id, project_id) REFERENCES projects (org_id, id)
);

-- ── issue heads ──────────────────────────────────────────────────────────────
CREATE TABLE issues (
  id               text        PRIMARY KEY CHECK (id ~ '^is-[0-9a-f]{16}$'),
  org_id           text        NOT NULL REFERENCES orgs (id),
  project_id       text        NOT NULL,
  team_id          text        NOT NULL,
  number           integer     NOT NULL CHECK (number >= 1),
  state            text        NOT NULL,
  state_reason     text        NOT NULL DEFAULT '' CHECK (char_length(state_reason) <= 500),
  assignee_user_id text,                                       -- a u- id; never an agent (OD-16)
  priority         text        NOT NULL DEFAULT 'none',
  head_revision    integer     NOT NULL DEFAULT 0 CHECK (head_revision >= 0),
  not_tied_reason  text        NOT NULL DEFAULT '',
  source           text        NOT NULL,
  external_source  text,
  external_id      text        CHECK (char_length(external_id) BETWEEN 1 AND 200),
  external_url     text        NOT NULL DEFAULT ''
                               CHECK (external_url = '' OR (external_url ~ '^https://'
                                                            AND octet_length(external_url) <= 2048)),
  created_at       timestamptz NOT NULL DEFAULT now(),
  created_by       text        NOT NULL,
  updated_at       timestamptz NOT NULL DEFAULT now(),
  erased_at        timestamptz,
  CONSTRAINT issues_state_known CHECK (state IN ('triage', 'backlog', 'todo', 'started',
                                    'in_review', 'landed', 'done', 'cancelled', 'duplicate')),
  CONSTRAINT issues_priority_known CHECK (priority IN ('none', 'urgent', 'high', 'medium', 'low')),
  CONSTRAINT issues_not_tied_reason_known
    CHECK (not_tied_reason IN ('', 'chore', 'spike', 'imported', 'support')),
  CONSTRAINT issues_source_known CHECK (source IN ('person', 'orchestrator', 'intake')),
  CONSTRAINT issues_external_source_known
    CHECK (external_source IS NULL OR external_source IN ('linear', 'csv')),
  CONSTRAINT issues_external_pair CHECK ((external_source IS NULL) = (external_id IS NULL)),
  CONSTRAINT issues_project_number_key UNIQUE (project_id, number),
  UNIQUE (org_id, id),
  UNIQUE (org_id, id, project_id),         -- decision 3: revisions and attachments pin the project
  FOREIGN KEY (org_id, project_id)       REFERENCES projects (org_id, id),
  FOREIGN KEY (org_id, team_id)          REFERENCES teams (org_id, id),
  FOREIGN KEY (org_id, assignee_user_id) REFERENCES users (org_id, id),
  FOREIGN KEY (org_id, created_by)       REFERENCES users (org_id, id)
);
CREATE UNIQUE INDEX issues_external_once ON issues (org_id, project_id, external_source, external_id)
  WHERE external_id IS NOT NULL;                                 -- SW.6: one issue per (source, id)
CREATE INDEX issues_project_state ON issues (project_id, state, number DESC);
CREATE INDEX issues_assignee ON issues (org_id, assignee_user_id) WHERE assignee_user_id IS NOT NULL;

-- ── issue revisions: immutable, digested; the stored C byte for byte (hub-contract §3.4) ──
CREATE TABLE issue_revisions (
  org_id        text        NOT NULL REFERENCES orgs (id),
  issue_id      text        NOT NULL,
  revision      integer     NOT NULL CHECK (revision >= 1),
  project_id    text        NOT NULL,
  canonical     bytea,                                     -- NULL exactly when erased (task 07)
  digest        bytea       NOT NULL CHECK (octet_length(digest) = 32),
  written_at    timestamptz NOT NULL DEFAULT now(),
  written_by    text        NOT NULL,
  restored_from integer     CHECK (restored_from IS NULL OR restored_from BETWEEN 1 AND revision - 1),
  erased_at     timestamptz,
  PRIMARY KEY (issue_id, revision),
  UNIQUE (org_id, issue_id, revision),
  FOREIGN KEY (org_id, issue_id, project_id) REFERENCES issues (org_id, id, project_id),
  FOREIGN KEY (org_id, written_by)           REFERENCES users (org_id, id),
  CONSTRAINT issue_revisions_erased_iff_no_content CHECK ((canonical IS NULL) = (erased_at IS NOT NULL)),
  CONSTRAINT issue_revisions_canonical_hashes
    CHECK (canonical IS NULL OR (octet_length(canonical) <= 65536 AND sha256(canonical) = digest))
);

-- An index of each revision's serves, written with the revision and never changed.
CREATE TABLE issue_serves (
  org_id       text NOT NULL REFERENCES orgs (id),
  issue_id     text NOT NULL,
  revision     integer NOT NULL,
  brief_key    text NOT NULL CHECK (brief_key ~ '^brief:bf-[0-9a-f]{16}@[1-9][0-9]{0,8}$'),
  criterion_id text NOT NULL CHECK (char_length(criterion_id) BETWEEN 1 AND 128),  -- as evidence
  PRIMARY KEY (issue_id, revision, brief_key, criterion_id),
  FOREIGN KEY (org_id, issue_id, revision) REFERENCES issue_revisions (org_id, issue_id, revision)
);
CREATE INDEX issue_serves_criterion ON issue_serves (org_id, brief_key, criterion_id);

-- ── labels and the three toggles ─────────────────────────────────────────────
CREATE TABLE labels (
  id          text        PRIMARY KEY CHECK (id ~ '^lb-[0-9a-f]{16}$'),
  org_id      text        NOT NULL REFERENCES orgs (id),
  team_id     text        NOT NULL,
  name        text        NOT NULL CHECK (char_length(name) BETWEEN 1 AND 60),
  color       text        NOT NULL CHECK (color ~ '^[a-z][a-z0-9-]{0,31}$'),  -- a token name, never a hex
  archived_at timestamptz,
  created_at  timestamptz NOT NULL DEFAULT now(),
  created_by  text        NOT NULL,
  UNIQUE (org_id, id),
  FOREIGN KEY (org_id, team_id)    REFERENCES teams (org_id, id),
  FOREIGN KEY (org_id, created_by) REFERENCES users (org_id, id)
);
CREATE UNIQUE INDEX labels_team_live_name ON labels (team_id, lower(name)) WHERE archived_at IS NULL;

CREATE TABLE issue_labels (
  org_id   text NOT NULL REFERENCES orgs (id),
  issue_id text NOT NULL,
  label_id text NOT NULL,
  PRIMARY KEY (issue_id, label_id),
  FOREIGN KEY (org_id, issue_id) REFERENCES issues (org_id, id),
  FOREIGN KEY (org_id, label_id) REFERENCES labels (org_id, id)
);
CREATE INDEX issue_labels_label ON issue_labels (label_id);

CREATE TABLE issue_relations (
  org_id         text        NOT NULL REFERENCES orgs (id),
  issue_id       text        NOT NULL,
  other_issue_id text        NOT NULL,
  kind           text        NOT NULL,
  created_at     timestamptz NOT NULL DEFAULT now(),
  created_by     text        NOT NULL,
  PRIMARY KEY (issue_id, kind, other_issue_id),
  CONSTRAINT issue_relations_kind_known CHECK (kind IN ('blocks', 'duplicates', 'relates')),
  CONSTRAINT issue_relations_not_self CHECK (issue_id <> other_issue_id),
  FOREIGN KEY (org_id, issue_id)       REFERENCES issues (org_id, id),
  FOREIGN KEY (org_id, other_issue_id) REFERENCES issues (org_id, id),
  FOREIGN KEY (org_id, created_by)     REFERENCES users (org_id, id)
);
CREATE INDEX issue_relations_other ON issue_relations (other_issue_id, kind);

-- ── team policies ────────────────────────────────────────────────────────────
CREATE TABLE team_work_policies (
  org_id                        text        NOT NULL REFERENCES orgs (id),
  team_id                       text        NOT NULL PRIMARY KEY,
  default_state                 text        NOT NULL DEFAULT 'backlog',  -- task 11's DefaultTeamPolicy
  priority_to_leave_triage      boolean     NOT NULL DEFAULT false,
  stop_at_landed_without_serves boolean     NOT NULL DEFAULT false,
  updated_at                    timestamptz NOT NULL DEFAULT now(),
  updated_by                    text        NOT NULL,
  CONSTRAINT team_work_policies_default_state_known
    CHECK (default_state IN ('triage', 'backlog', 'todo')),      -- hubapi.InitialStates only
  FOREIGN KEY (org_id, team_id)    REFERENCES teams (org_id, id),
  FOREIGN KEY (org_id, updated_by) REFERENCES users (org_id, id)
);

-- ── comments ─────────────────────────────────────────────────────────────────
CREATE TABLE issue_comments (
  id           text        PRIMARY KEY CHECK (id ~ '^ic-[0-9a-f]{16}$'),
  org_id       text        NOT NULL REFERENCES orgs (id),
  issue_id     text        NOT NULL,
  parent_id    text,                                       -- a reply; same issue, by the key below
  kind         text        NOT NULL DEFAULT 'comment',
  author_class text        NOT NULL CHECK (author_class IN ('H', 'A')),    -- text·H / text·A
  body         text        NOT NULL CHECK (octet_length(body) <= 16384),
  anchor       jsonb       CHECK (anchor IS NULL OR jsonb_typeof(anchor) = 'object'),
  created_at   timestamptz NOT NULL DEFAULT now(),
  created_by   text        NOT NULL,
  resolved_at  timestamptz,
  resolved_by  text,
  erased_at    timestamptz,
  CONSTRAINT issue_comments_kind_known CHECK (kind IN ('comment')),     -- W3 widens (task 99)
  CONSTRAINT issue_comments_resolved_pair CHECK ((resolved_at IS NULL) = (resolved_by IS NULL)),
  CONSTRAINT issue_comments_erased_is_blank
    CHECK (erased_at IS NULL OR (body = '' AND anchor IS NULL)),
  UNIQUE (org_id, id),
  UNIQUE (org_id, issue_id, id),
  FOREIGN KEY (org_id, issue_id)            REFERENCES issues (org_id, id),
  FOREIGN KEY (org_id, issue_id, parent_id) REFERENCES issue_comments (org_id, issue_id, id),
  FOREIGN KEY (org_id, created_by)          REFERENCES users (org_id, id),
  FOREIGN KEY (org_id, resolved_by)         REFERENCES users (org_id, id)
);
CREATE INDEX issue_comments_issue ON issue_comments (issue_id, created_at);

CREATE TABLE comment_mentions (
  org_id     text NOT NULL REFERENCES orgs (id),
  comment_id text NOT NULL,
  user_id    text NOT NULL,
  PRIMARY KEY (comment_id, user_id),
  FOREIGN KEY (org_id, comment_id) REFERENCES issue_comments (org_id, id),
  FOREIGN KEY (org_id, user_id)    REFERENCES users (org_id, id)
);

CREATE TABLE comment_reactions (
  org_id     text        NOT NULL REFERENCES orgs (id),
  comment_id text        NOT NULL,
  user_id    text        NOT NULL,
  emoji      text        NOT NULL CHECK (char_length(emoji) BETWEEN 1 AND 16 AND emoji !~ '^:'),
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (comment_id, user_id, emoji),
  FOREIGN KEY (org_id, comment_id) REFERENCES issue_comments (org_id, id),
  FOREIGN KEY (org_id, user_id)    REFERENCES users (org_id, id)
);

-- ── subscriptions and attachments ────────────────────────────────────────────
CREATE TABLE issue_subscriptions (
  org_id     text        NOT NULL REFERENCES orgs (id),
  issue_id   text        NOT NULL,
  user_id    text        NOT NULL,
  subscribed boolean     NOT NULL,
  cause      text        NOT NULL,
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (issue_id, user_id),
  CONSTRAINT issue_subscriptions_cause_known
    CHECK (cause IN ('create', 'assign', 'mention', 'comment', 'manual')),
  FOREIGN KEY (org_id, issue_id) REFERENCES issues (org_id, id),
  FOREIGN KEY (org_id, user_id)  REFERENCES users (org_id, id)
);
CREATE INDEX issue_subscriptions_user ON issue_subscriptions (org_id, user_id) WHERE subscribed;

CREATE TABLE issue_attachments (
  org_id     text        NOT NULL REFERENCES orgs (id),
  issue_id   text        NOT NULL,
  project_id text        NOT NULL,
  digest     bytea       NOT NULL,
  added_at   timestamptz NOT NULL DEFAULT now(),
  added_by   text        NOT NULL,
  PRIMARY KEY (issue_id, digest),
  FOREIGN KEY (org_id, issue_id, project_id) REFERENCES issues (org_id, id, project_id),
  FOREIGN KEY (org_id, project_id, digest)   REFERENCES project_blobs (org_id, project_id, digest),
  FOREIGN KEY (org_id, added_by)             REFERENCES users (org_id, id)
);

-- ── row-level security: hub-contract §10.3's loop (:1908-1920), over exactly these tables ──
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['project_keys', 'project_issue_counters', 'issues', 'issue_revisions',
      'issue_serves', 'labels', 'issue_labels', 'issue_relations', 'team_work_policies',
      'issue_comments', 'comment_mentions', 'comment_reactions', 'issue_subscriptions',
      'issue_attachments'] LOOP
    EXECUTE format('ALTER TABLE hub.%I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('CREATE POLICY org_isolation ON hub.%I TO hub_app
                    USING (org_id = current_setting(''hub.org_id'', true))
                    WITH CHECK (org_id = current_setting(''hub.org_id'', true))', t);
  END LOOP;
END $$;

-- ── grants: hub-contract §10.5's shape (:2003-2027). 0001's GRANT SELECT ON ALL TABLES
--    covered only the tables that existed then, so every grant here is explicit. ──
GRANT SELECT, INSERT ON project_keys, project_issue_counters, issues, issue_revisions,
                        issue_serves, labels, issue_labels, issue_relations, team_work_policies,
                        issue_comments, comment_mentions, comment_reactions, issue_subscriptions,
                        issue_attachments TO hub_app;

GRANT UPDATE (key, set_at, set_by)                                  ON project_keys           TO hub_app;
GRANT UPDATE (last_number)                                          ON project_issue_counters TO hub_app;
GRANT UPDATE (state, state_reason, assignee_user_id, priority, head_revision, not_tied_reason,
              updated_at)                                           ON issues                 TO hub_app;
GRANT UPDATE (name, color, archived_at)                             ON labels                 TO hub_app;
GRANT UPDATE (default_state, priority_to_leave_triage, stop_at_landed_without_serves,
              updated_at, updated_by)                               ON team_work_policies     TO hub_app;
GRANT UPDATE (resolved_at, resolved_by)                             ON issue_comments         TO hub_app;
GRANT UPDATE (subscribed, cause, updated_at)                        ON issue_subscriptions    TO hub_app;
GRANT DELETE                  ON issue_labels, issue_relations, comment_reactions             TO hub_app;
-- No UPDATE or DELETE on: issue_revisions, issue_serves, comment_mentions, issue_attachments.
-- No UPDATE of: issues.number, project_id, team_id, source, external_*, created_*, erased_at;
--               issue_comments.body, anchor, kind, author_class, erased_at. Erase is task 07's.
```

The column lists above are the whole of what `hub_app` may change. Two things are deliberate and
easy to "fix" by mistake. **`issues.external_url` is not granted**: an imported issue's link back
is set at insert and cleared only by erase (task 07). **`issue_comments.body` is not granted**: W1
has no comment edit (a correction is a new comment; the W1 route list in task 07 has no comment
`PATCH`); a
later slice that adds editing adds the grant in its own migration with its own test.

#### Tests to write first — `migrate/work_w1_integration_test.go` (`//go:build integration`)

Each runs on `hubtest.Migrated(t)`, which now applies `0001` and this file. "Refused" means the
statement returns a `*pgconn.PgError` whose `Code` is the one named; where a constraint name is
given, `ConstraintName` must equal it too, because tasks 08, 11 and 15 branch on that name.

The seed helper is this file's, and tasks 07-15 reuse it. It calls H07a's seed helper for the orgs,
users, teams, members and projects, then adds a project-scope brief at revision 1 and one project
blob per project, in the order the Hub writes:

```go
//go:build integration

package migrate_test

// workOrg is one org's seeded parents. Every id is a real ids.Wide id.
type workOrg struct {
	Org      string // org-…
	Team     string // tm-…
	Project  string // pj-…, the team's first project
	Project2 string // pj-…, the team's second project (decision 3's tests)
	Admin    string // u-…, team admin
	Member   string // u-…, team member
	Viewer   string // u-…, team viewer
	Brief    string // brief:bf-…@1, project scope, on Project
	Blob     []byte // a 32-byte digest in project_blobs for Project
	Blob2    []byte // a 32-byte digest in project_blobs for Project2
}

type workSeed struct{ A, B workOrg }

// seedWork seeds two orgs through H07a's helper, then briefs and blobs as hub_app
// with hub.org_id set. It writes no Work row: each test writes its own.
func seedWork(t *testing.T, db *hubtest.DB) workSeed

// asApp runs fn in one hub_app transaction with hub.org_id = org ("" leaves it unset),
// and rolls back, so a test's refused statement never poisons the next one.
func asApp(t *testing.T, db *hubtest.DB, org string, fn func(tx pgx.Tx) error) error

// insertIssue writes a head at head_revision 0 as hub_app; the columns not named take
// their defaults. It returns the new is- id.
func insertIssue(t *testing.T, tx pgx.Tx, o workOrg, project string, number int) string
```

`pgcode(err)` and `pgconstraint(err)` unwrap with `errors.As(err, &pgErr)`; a test never matches on
the message text.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestEveryOrgTableHasOrgIsolation` | reads `pg_tables` for schema `hub` joined to `information_schema.columns` (`column_name = 'org_id'`), **not a list**; each such table has `relrowsecurity` and a `pg_policies` row `org_isolation` for role `hub_app` whose `qual` and `with_check` both name `hub.org_id` | a table left out of the loop; `USING` without `WITH CHECK`. It also covers task 10's `issue_proofs` when it lands |
| `TestAnUnsetOrgSeesNoIssues` | one issue and one comment seeded for org A; as `hub_app` with `hub.org_id` unset, `SELECT count(*)` from `issues`, `issue_revisions` and `issue_comments` is 0 each; with it set to B, also 0 | a permissive policy (`USING (true)`), or one that reads a session-level setting |
| `TestAnotherOrgsRowFailsTheCheck` | `hub.org_id` = A; `INSERT INTO issues` with `org_id` = B → `42501` | a policy with `USING` only |
| `TestIssueNumberAndProjectAreFrozen` | as `hub_app`: `UPDATE issues SET number = 9`, `SET project_id = …`, `SET external_url = ''`, `SET erased_at = now()` → `42501` each; `SET priority = 'high'` → ok | a table-wide `UPDATE` grant; `external_url` or `erased_at` added to the column list |
| `TestNoDeleteBeyondTheToggles` | `DELETE` from `issues`, `issue_revisions`, `issue_serves`, `issue_comments`, `comment_mentions`, `issue_attachments`, `labels`, `project_keys`, `team_work_policies`, `issue_subscriptions`, `project_issue_counters` → `42501` each; from `issue_labels`, `issue_relations`, `comment_reactions` → ok | a `DELETE` grant beyond the three toggles |
| `TestWorkGrantsAreExactlyTheList` | `information_schema.role_table_grants` and `column_privileges` for `hub_app`, **restricted to the fourteen Work tables**, equal the grants block above, table by table and column by column | a grant nobody reviewed (the Work half of H07a's `TestHubAppGrantsAreExactlyTheList`) |
| `TestIssueNumbersAreUniquePerProject` | two issues `(Project, 7)` → `23505`, constraint `issues_project_number_key`; `(Project2, 7)` → ok | a missing unique; a unique on `number` alone |
| `TestAnExternalIssueLandsOnce` | two issues `('linear', 'LIN-1')` in `Project` → `23505`, constraint `issues_external_once`; the same pair in `Project2` → ok; `external_source` set with `external_id` NULL → `23514` | SW.6's idempotency left to Go; a pair half-set |
| `TestClosedSetsRefuseStrangers` | `state = 'shipped'`, `priority = 'p1'`, `not_tied_reason = 'misc'`, `source = 'sync'`, `external_source = 'jira'`, a relation kind `'parent'`, a comment kind `'note'`, a policy `default_state = 'done'` → `23514` each, each naming its `…_known` constraint | 05 §3.3's derived `shipped` stored as a state; a set widened before its slice (W3, W5) |
| `TestARevisionMustHashToItsDigest` | `INSERT` a revision whose `digest` is not `sha256(canonical)` → `23514`, constraint `issue_revisions_canonical_hashes`; a revision with `canonical` NULL and `erased_at` NULL → `23514` | a digest the Go side computed wrong, stored anyway |
| `TestARevisionPinsItsIssuesProject` | an issue in `Project`; a revision naming it with `project_id` = `Project2` → `23503` | decision 3 dropped: a key on `(org_id, issue_id)` alone |
| `TestAReplyStaysOnItsIssue` | comment `c1` on issue X; a reply on issue Y with `parent_id` = `c1` → `23503`; on X → ok | a single-column `parent_id` key, which lets a thread span issues |
| `TestAnAttachmentNeedsItsProjectsBlob` | an issue in `Project`; attaching `Blob2` (Project2's) → `23503`; attaching `Blob` → ok; an unknown digest → `23503` | attaching another project's file; a key to `blobs` instead of `project_blobs` |
| `TestWorkMigrationReappliesClean` | `migrate.Apply` on a migrated database → `[]`; `Current` = `Latest()`; `Latest()` is `0001`'s number plus the Work migrations on the base | a migration the runner re-runs; a file not embedded |

H07a's `TestEveryForeignKeyCarriesTheOrg` and `TestACrossOrgReferenceIsRefused` run on the same
migrated database and read the catalog, so they check every key above with no edit. The second of
them seeds "one parent of every kind" from its own list (H07a `:531`); if its closing assertion
("the set of constraint names exercised equals the catalog's set") goes red on the Work keys, that
is the gap named at the top, and the fix is the same proposed edit to the Hub owner.

#### Steps

- [ ] **Step 1: Check the base.** On the sync base, `ls control-plane/internal/hub/migrate/sql/`
  and read hub-contract §10.1-§10.5 and H07a's SQL. Take the next free number for the file name.
  If `0001_init.sql` changed a parent's `UNIQUE (org_id, id)` target, or `project_blobs`' key, the
  Hub's file wins and this SQL follows it (rule H1); record the change in the commit message.
- [ ] **Step 2: The tests first.** Write `work_w1_integration_test.go` with `seedWork`, `asApp`,
  `insertIssue` and the fourteen tests. Run them: each fails, because no Work table exists.
- [ ] **Step 3: The migration.** Write `NNNN_work_w1.sql` exactly as above. Run the tests against
  the compose stack until every one passes, then run H07a's and H07b's schema and guard tests on
  the same database; they must pass unchanged.
- [ ] **Step 4: The two Hub tests.** If `TestHubAppGrantsAreExactlyTheList` or
  `TestACrossOrgReferenceIsRefused` goes red **only** because the Work tables exist, do not edit
  them. Write the proposed scoping edit into the WH proposal (task 01's file), name the failing
  assertion in the commit message, and stop for the orchestrator. Any other red is a finding against
  this SQL.
- [ ] **Step 5: Commit by pathspec**, the two files, with the migration number in the message.
  Do not migrate a persistent database (`hub-pg-dev`) from this commit: tasks 07 and 10 append to
  the same file, and the runner would then refuse it (`ErrEdited`).

#### Acceptance

```bash
# the compose stack from control-plane/cmd/hub/compose.yaml is up; the URL is the local admin's
export PANACEA_APP_HUB_TEST_DATABASE_URL='postgres://postgres@127.0.0.1:5433/postgres?sslmode=disable'
go test ./control-plane/internal/hub/migrate/... -tags integration -race -count=1 -v \
  -run 'Work|OrgIsolation|UnsetOrg|AnotherOrgs|IssueNumber|NoDelete|ExternalIssue|ClosedSets|Revision|Reply|Attachment|EveryForeignKey|CrossOrg|GrantsAreExactly'
```

Expected: every test named in the table above reports `--- PASS`, H07a's
`TestEveryForeignKeyCarriesTheOrg` and `TestACrossOrgReferenceIsRefused` report `--- PASS`, and the
run ends `ok  github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/migrate`. With the
variable unset every integration test reports `--- SKIP` naming `PANACEA_APP_HUB_TEST_DATABASE_URL`
(H06's `hubtest.New`); a skip is **not** a pass and is not reported as one.

```bash
go test ./control-plane/... -race -count=1        # the runner's unit tests see a second file
grep -n "AS ENUM" control-plane/internal/hub/migrate/sql/*_work_w1.sql   # decision 4
git diff --stat -- go.work control-plane/go.mod    # the go directive stays 1.23
```

Expected: the first ends with every package `ok` (no `FAIL`); the `grep` prints nothing; the
`diff` prints nothing.

#### What this task does not claim

- **No guard.** A hand-written `UPDATE issues SET state = 'done'` succeeds after this task alone;
  task 07's trigger refuses it. Revisions are protected here only by the grant (no `UPDATE`), and
  the owner role can still change one until task 07's `guard_issue_revision` lands.
- **No route, store or event.** Nothing here writes an issue; tasks 08-15 do, each with its one
  event per write.
- **Not that the number is final.** The migration's number is the next free one on the sync base
  and may be renumbered at merge (rule H1).
- **Not that the Hub's two catalog-wide tests pass unchanged.** It is written to; whether
  `TestHubAppGrantsAreExactlyTheList` is scoped to `0001` is **UNVERIFIED** (see the top).
- **Not the janitor's reference list.** Attached blobs are protected by a key; the janitor skipping
  them cleanly is Hub task 24's, as a proposed edit (**UNVERIFIED**).
- **No measured size or speed.** No query here has been planned against real data; the indexes are
  the ones the W1 routes' filters name (tasks 08, 11, 13), chosen by reading, not by `EXPLAIN`.

**Parity rows served:** none closed alone. This is the storage behind **L-3** (key and number),
**L-8**, **L-9**, **L-10** (states and policies), **L-13** (priority), **L-17** (labels), **L-24**
(relations), **L-34** (revisions), **L-41**, **L-42** (reactions, anchors), **L-45** (attachments),
**L-48** (team policy), **L-117** (subscriptions) and **L-220** (the imported external link).
