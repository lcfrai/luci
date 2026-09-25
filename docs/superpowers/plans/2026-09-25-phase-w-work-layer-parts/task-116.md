### Task 116: the W4 migration, its guards and matrix rows

**Slice:** W4. **Size:** 3 focused hours (an estimate, not a measurement). **Stream:** WH.
**dependsOn:** 114 (SW.20's shapes and words), and the W1-W3 migrations it builds on (tasks 06-07,
74, 99's), sync-merged from `feat/hub` first (rule H1). **Seams:** none new; it extends **SW.3**
(no sync or integration principal writes a Work fact) to every W4 table.

Tasks 117-123 each write one family of W4 rows. If the database trusted Go to keep an update
immutable, a dependency acyclic, a cursor one-way and the sync principal out, one handler bug
would break a rule the product promises (L-76's pinned update, OD-27's measured-only insights).
So the rules live in the schema: `CHECK`s, triggers and row-level security, each with a test
that fails when the guard is dropped. This task also adds the matrix's new principal, a member of
a **granted** team, so every route task can add its rows for it.

**Anchors, re-read on 2026-09-25 at `feat/hub` `1271ddb`:** the contract
`docs/superpowers/contracts/2026-09-23-hub-contract.md:309` (`set_config('hub.org_id', …)`, the
row-level-security key), `:333` (*"There are no per-project grants in v1"*, which this task's
table begins to change, additively), `:335-359` (§5.3: the columns `V M A O N X`, *"the
authorisation test (H2) runs every row as every column"*).

**Read in this branch's parts, UNVERIFIED until merge:** task 06's row-level-security loop
(`task-06.md:370-381`, `TO hub_app`, `current_setting('hub.org_id', true)`) and its explicit
grants (`:383-386`); task 07's actor setting `hub.work_actor_kind` ∈ {`person`, `integration`,
`app`} and its guard shape (`task-07.md:29-31`, `:205-222`), its matrix files and
`workMatrixLive` (`:44-51`, `:57-66`); task 74's `features`, `templates` and `teams`-side tables
(scope `tasks-61-80.md:469-497`; no part yet); TM-46's `notes` table (Phase T plan; **UNVERIFIED**
name). The sibling parts that append to this file: task 119 (`work_update_pins`, reusing this
part's `guard_work_update_meta()`, `task-119.md:112-137`) and task 120 (`work_brief_templates`
columns and `work_brief_criteria`, `task-120.md:128-161`; the cycle trigger's `P0001` and message
prefix `hub: dependency_cycle`, `:241-242`).

#### Decisions the scope left open, made here

1. **No text lives in `work_update_meta`, so its guard has no erase exception.** The update's
   words are TM-46's note (`text·H`); this table holds the subject, the health word, the pin and
   the cite keys. TM-46's erase blanks the note's text and never touches this row, so the trigger
   refuses **every** `UPDATE` and `DELETE`, and the scope's "except through the erase function" is
   unneeded. The erase test proves the row survives the note's erase whole.
2. **The cycle guard serialises per org.** Two concurrent inserts, A→B and B→A, would each pass a
   recursive check that cannot see the other. The trigger takes
   `pg_advisory_xact_lock(hashtext('work_feature_deps:' || org_id))` before its CTE, so the second
   waits and then sees the first. It raises `P0001` with the message prefix `hub: dependency_cycle`,
   which task 120's store maps (`task-120.md:241-242`).
3. **A lower cursor is kept, not refused.** A `BEFORE UPDATE` trigger sets
   `NEW.seq := GREATEST(OLD.seq, NEW.seq)`, so the store's upsert may send any seq and the row
   never goes back (task 115's row sends 7 then 5 and reads 7). No `DELETE` grant.
4. **SW.3 extends by one trigger function on every W4 table.** `guard_work_w4_actor()` refuses any
   write unless `hub.work_actor_kind` is `person`, with one exception: the janitor (task 121) may
   `UPDATE` only `last_fired_on` on `work_template_schedules` and `last_reminded_on` on
   `work_reminder_policies`, under the actor kind task 77's janitor sets (`janitor`,
   **UNVERIFIED** spelling until task 77's part).
5. **Cursors are private by a second policy.** Like task 74's drafts, `work_read_cursors` adds a
   policy on `user_id = current_setting('hub.user_id', true)` (task 74's key, **UNVERIFIED**), so
   no query in any handler can read another person's cursor.
6. **The granted-team member is rows, not a column** (the scope's rule): the fixture gains a
   principal `G`, and the G rows are grouped under task **117**'s id in `workMatrixLive`, so they
   switch on with the route that makes a grant mean something (task 07's decision 6).
7. **`features.initiative_id`** gets its foreign key here. Task 72's `Feature.initiativeId` puts
   the column in task 74's table; this task adds the column if absent and the constraint.
