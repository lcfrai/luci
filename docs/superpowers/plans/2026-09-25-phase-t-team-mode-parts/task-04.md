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
`team_event_kinds` holds task 01's 21 rows. A `BEFORE INSERT` trigger **sets** both flags for a
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
