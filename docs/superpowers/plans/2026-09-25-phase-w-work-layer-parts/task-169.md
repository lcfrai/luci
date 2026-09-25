### Task 169: desktop W5 wire types and replica kinds

W5, stream WD, the desktop half of seam **SW.21**, as tasks 37, 83 and 125 were for W1, W2 and W4.
SW.15 (the desktop Work types) and SW.16 (the replica) grow by **added** types, calls and kinds
only. Depends on task 125 (the W4 wire, the last one added before this: `task-125.md`) and task 144
(the fake's W5 rules, whose event kinds this task copies by name). It also reads task 143's types
and codes, task 168's route table, task 37's `WORK_RELAY`, `relayPath`, `WorkSend`, `WIRE_KEYS`
and `workProblem` (`task-37.md`), and task 57's `classify` and `admit` (`task-57.md`). Estimate
0.25 (2 h).

After this task the desktop can name every W5 record, call every W5 relay route, and hold the
**Work** W5 records (requests, their thread, links and loops) in its replica. The **admin** W5
records (apps, keys, webhooks, deliveries, connectors, the harness allow-list and team skills)
never enter the replica. It draws nothing: tasks 170 to 173 do.

**Decisions made here, so nobody re-derives them:**

1. **Task 143's list wins over this task's scope paragraph.** The scope names `SyncLogEntry`,
   `LoopVersion`, `LoopTrigger`, `LoopBound`, `TeamAgentPolicy` and `DigestSettings.teams`; task
   143's paragraph (`…-scopes/tasks-141-160.md:139-174`) names `SyncRun`, `Loop.version`,
   `Loop.trigger`, `Loop.grid`, `HarnessAllow` plus `TeamSkill`, and `TeamsBinding`. The desktop
   mirrors Go, so the types below carry task 143's names. `LoopVersion` survives as a desktop-only
   name for "a `Loop` as it was at one `version`" (decision 6), and `SyncLogEntry` as a desktop
   **view** derived from a `SyncRun`, never a wire type (decision 7). The mismatch is a gap, below.
2. **Field for field with Go, by value, not by eye.** Each W5 type has a `WIRE_KEYS` entry held to
   its type both ways by task 37's `keys<T>()`, and a golden test reads task 143's
   `control-plane/internal/hub/hubapi/testdata/work-intake/*.json` **by path** and requires every
   golden's keys to equal the type's list. A field renamed on either side (`quoteText` for
   `quote`) fails that test even where `tsc` passes, because `tsc` never sees the Go side.
3. **Kinds are copied by name, never retyped.** `W5_FEED_KINDS` is a literal list, and a test reads
   task 144's `fake_work_intake.go` by path, extracts every `Kind: "…"` literal it appends, and
   requires the Work subset and the admin subset together to equal that set. A kind the Hub adds
   later and this list lacks is **ignored** by the replica, never thrown (hub-contract §1).
4. **Admin records are not replica kinds.** Apps, keys, webhooks, deliveries, connectors, the
   harness allow-list and team skills are fetched on demand through `WorkClient` by task 173 and
   held in component state only. Their feed kinds are in `W5_ADMIN_KINDS`, which the replica reads
   **only** to skip; nothing in the replica, `workSlice` or `localStorage` keeps them (§3.1, *no Hub
   data on disk*; ADR-H1). Task 55's no-persistence grep is extended to the new modules.
5. **A secret lives in one variable, once.** `CreatedAPIKey`, `CreatedWebhook` and
   `CreatedHostHook` are the only W5 types that carry `secret` (task 143). The client returns them
   and nothing else stores them: `ops.ts` has no op for them, `workSlice` has no field typed with
   them, and a test walks the slice and the replica after the create resolves and finds no string
   equal to the secret.
6. **A loop edit is a new version.** `ops.ts` gains `putLoop`, which sends the whole loop with the
   `version` it was read at; the Hub answers the next `version` (task 162). There is no op that
   patches a loop in place, and `ListLoopVersions` answers `Loop[]`, newest first; the desktop
   aliases one element as `LoopVersion`.
7. **The sync log is a view over `SyncRun`, not a wire type.** Task 143's `SyncRun` carries counts
   (`created`, `existing`, `commentsIn`, `commentsOut`) and `errors [{externalId, code}]`. Task
   171's log needs one row per external item. `syncLogRows(run)` derives one `SyncLogEntry` per
   `errors` element (result `refused`, the reason from the code) and one summary row per run; it
   never invents per-item `imported` rows the Hub did not name. The gap, below, asks task 150 for
   per-item rows.
8. **The request quote is team-scoped.** A `Request` enters the replica only when the caller is a
   member of `request.teamId` (task 57's `admit`, team branch); a `request.*` event for another
   team's request is skipped, even when the caller can read the linked issue's project by grant.

**Files:**
- `desktop/src/api/work.ts` ⚑ (task 37; tasks 83 and 125 before this) — the W5 records, request
  bodies, `WORK_RELAY_W5`, the `WorkIntakeClient` members, `WIRE_KEYS` entries (modified)
- `desktop/src/api/work.test.ts` ⚑ — the W5 goldens, the relay table against task 168's (modified)
- `desktop/src/api/client.ts` ⚑ — the ten W5 codes join `ProblemCode` (modified)
- `desktop/src/work/replica/kinds.ts` ⚑ (task 125) — `W5_FEED_KINDS`, `W5_ADMIN_KINDS` (modified)
- `desktop/src/work/replica/kinds.test.ts` ⚑ (task 125) — the copied-by-name test for W5 (modified)
- `desktop/src/work/replica/replica.ts` ⚑ (task 55) — the W5 record maps and their load (modified)
- `desktop/src/work/replica/ops.ts` ⚑ (task 56) — the W5 ops, no loop patch, no secret op (modified)
- `desktop/src/work/replica/visibility.ts` ⚑ (task 57) — the request team rule (modified)
- `desktop/src/work/replica/intake.test.ts` — order, admin skip, secret absence, unknown kind (new)

**What is on the tree today (verified 2026-09-26).** On this branch's tree, parts exist for tasks
37, 42, 56, 57 and 125 (`…-parts/task-37.md`, `task-42.md`, `task-56.md`, `task-57.md`,
`task-125.md`). There is **no part** for tasks 55, 143, 144 or 146-168, so every name taken from
them is their scope's (`…-scopes/tasks-141-160.md:124-229`, `tasks-161-180.md:29-365`) and
**UNVERIFIED**; task 143's part wins any difference, and this file follows it. The desktop source
files named above are planned by those tasks and do not exist on `main` yet.
