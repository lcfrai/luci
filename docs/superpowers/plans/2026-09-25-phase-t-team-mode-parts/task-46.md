### Task 46: Hub delta: notes and erasure tombstones

**Slice TM1. Depends on: 07, H27. About 2 hours. Seam: ST.2.**

This task gives the Hub the one piece of team text a person types *about other people*: the note
(`05-design.md` §3.3 row "note", `:312`), routes 72 and 73 of task 01's §7.0 team table and its
contract §7.23 (`task-01.md:283-284`, `:471-476`). It also closes `07` M5 (`07-critique-security.md:490`):
notes are immutable, messages and human messages append-only, and yet an erasure request must be
able to remove their words. **Task 01 is authoritative for every field, bound and code of routes 72
and 73; where this part disagrees, task 01 wins.** It is a delta on the Hub branch, built after the
sync-merge of `feat/hub`, on the sibling files Hub task 27 names (`server/erase.go`,
`store/erase.go`, `task-27.md:15-16`, **UNVERIFIED until merge**: the Hub server package is Hub
task 14's, unbuilt on 2026-09-25; only `hubapi` and `hubclient` exist, checked on
`.claude/worktrees/hub` at `4dff13e`).

**The one thing this task must not get wrong is where a note's words can go.** A note is 1,000
characters of `text·H` by a colleague, and a persuasive one is the strongest injector the design has
(`07` M2, `07-critique-security.md:487`). So a note reaches **an Orchestrator only, only through
`panacea inbox`, quoted with its author's name** — never a worker, never `team.json`, never a
directive's text, never the PTY. The worker half is not a filter added here: `notes.read` is
outside the `worker` class route set (`task-01.md:303-305`), so a worker-class delegation cannot
even fetch one, and task 22's `WorkerSnapshot` has no field that could hold one. This task pins
both with tests, so a later "helpful" change cannot route a note to a worker silently.

**The second thing is that erasure removes words and keeps accountability.** A tombstone sets
`text` to `NULL` and `tombstoned_at` to the erase time on the row; the row, its ids, its target and
its one audit event stay. The Hub's rule stays: only an **org admin** erases (`task-27.md:31-33`,
`04-enterprise-constraints.md:207-208`). One addition, which `04` asks for (`:209-210`): **the
author of a note, and the owner of a human message, may tombstone their own record**, because they
typed the words. Nobody may tombstone another person's record but an org admin.

**Files:**
- `control-plane/internal/hub/server/notes.go` + `notes_integration_test.go`: routes 72, 73 (new)
- `control-plane/internal/hub/server/teamerase.go` + `teamerase_integration_test.go`: routes 81
  to 83 (new)
- `control-plane/internal/hub/store/notes.go`, `store/teamerase.go` (new)
- `control-plane/internal/hub/migrate/sql/00NN_team_erase.sql`: three `SECURITY DEFINER`
  tombstone functions, numbered after task 04's `00NN_team.sql` (new)
- `control-plane/internal/hub/hubapi/team.go` (task 03): add `EraseRecordResponse`; the request
  is the Hub's `EraseRequest`, reused. Routes 81 to 83 get no route name, so no delegation can
  list them (modify)
- `control-plane/internal/hub/hubapi/codes.go` (task 03): `CodeNoteNotFound`,
  `CodeHumanMessageNotFound`, `CodeMessageNotFound`, appended in §4.2's order; `keys_test.go`'s
  own-code count in `TestHubCodesAreContractSection42s` rises by exactly three from its value at
  merge (48 after task 01, `task-03.md:37-40`) (modify)
- `control-plane/internal/hub/server/humanmsgs.go` (task 07): route 70's stored reply nulls `text`
  (modify, three lines; see "The stored reply")
- `control-plane/internal/hub/server/server.go`: `s.registerNotesAndErase()` (modify: one line)
- `control-plane/internal/hub/store/changes.go` (task 08): three kinds (modify)
- `control-plane/internal/fleet/protocol/inbox.go` (task 28): `Envelope.Notes` (modify)
- `control-plane/internal/fleet/protocol/inbox_test.go`, `.../verbs/team_test.go` (task 22):
  the two worker tests (modify)
- `docs/superpowers/contracts/2026-09-23-hub-contract.md`: §7.0 rows 81 to 83, §7.23a, §8 rows,
  §5.3 team rows (modify)

**Interfaces:**
- Consumes:
  - from task 01: rows 72 and 73, §7.23, the `note.created` §8 row (`task-01.md:565`), the team
    matrix row "write a note (72)" (`:617`) and "read … notes (73)" (`:620`).
  - from task 03: `Note`, `NoteTarget`, `NoteTargetKind`, `CreateNoteRequest`, `RouteNotesRead`,
    `PrefixNote` (`task-03.md:876-895`, `:394`, `:1124`); `HumanMessage` and `Message`, whose
    `Text` is a `*teamtext.Text` and `TombstonedAt` a `*string` (`:858-867`, `:737`).
  - from task 04: tables `notes` (immutable), `messages` and `human_messages` (append-only), each
    with a nullable `text` and `tombstoned_at timestamptz NULL`, under RLS, with task 04's guard
    trigger that refuses any `UPDATE` by `hub_app` (**task 04's part is not yet written**; the
    column names are the skeleton's, `2026-09-25-phase-t-team-mode.md:460`).
  - from task 07: `RequireDesktop`, `RequireFleet`, the `team2World` fixture, route 70's handler.
  - from Hub task 27: `hubapi.EraseRequest` (`hubapi/types.go:565-567`, built), `markerHits`
    (`task-27.md:72`), the org-admin check and the feed's reason-stripping (`:76`).
- Produces: routes 72, 73, 81, 82, 83; `hubapi.EraseRecordResponse`; the kinds `note.erased`,
  `message.erased`, `human_message.erased`; `protocol.NoteItem` and `Envelope.Notes`.
- Required by: task 52 (the desktop writes and reads notes, erases its own), task 54 (a note
  addressed to me is an inform row, `05-design.md:735`), task 57 (TM1 proof).

#### The routes — complete wire

Conventions are task 06's and 07's: plain-string text in requests, stamped `Text` in answers,
strict decoding that refuses any owner, author, actor, `via`, delegation or session-id field.

**Route 72, `POST /v1/projects/{p}/notes`**: the member's own desktop, role `member` or `admin`, as
themselves. No route name, so Fleet cannot write one under any delegation (403 `forbidden`,
`required: "desktop"`, as task 07's `RequireDesktop`).

```jsonc
{ "target": { "kind": "project" | "task" | "user", "id": "pj-…" | "tk-…" | "u-…" },
  "text": "…" }                                  // 1–1,000 runes after G3 hygiene; author "human"
// 201
{ "id": "nt-…", "projectId": "pj-…", "authorUserId": "u-…",   // from the token, never the body
  "target": { "kind": "task", "id": "tk-…" },
  "text": { "body": "…", "author": "human", "authorAgentId": null,
            "ownerUserId": "u-…", "at": "…" },                // task 02's Text (task-02.md:245-249)
  "createdAt": "…", "tombstonedAt": null }
```

The target must be in `{p}`: `project` names `{p}` itself; `task` a task of `{p}` (task 06's
`tasks` row, any owner); `user` a **current** member of `{p}`'s team, the author included. Any
other target is 422 `bad_request`, `details.field = "target"`. A note on another member's task is
allowed (matrix column `M′`, `task-01.md:615`): writing about someone's work is not writing their
record. The Hub mints `nt-…` from `ids.New` (phase 2's `internal/ids`). `Idempotency-Key` is
optional, as on route 70.

**Route 73, `GET /v1/projects/{p}/notes?target=<kind>:<id>&after=<nt-…>&limit=<1–200>`**: a
viewer or above on the desktop, or Fleet·O with `notes.read`. Oldest first by `(created_at, id)`;
`after` is the last id of the previous page; `limit` defaults to 50. `target` is optional and
filters to one target. `200 { "notes": [Note…], "next": "nt-…" | "", "more": bool }`. A
tombstoned note is **listed** with `text: null` and `tombstonedAt` set, so a reader sees that a
note was there and was erased; it is never omitted, and never shows words.

**Route 81, `POST /v1/notes/{noteId}/erase`**, **route 82, `POST /v1/human-messages/{humanMessageId}/erase`**,
**route 83, `POST /v1/messages/{messageId}/erase`**: desktop only, body `hubapi.EraseRequest`
(`reason` 1–500 characters after trimming, else 400 `bad_request`, as Hub route 45).

| Route | Who may erase | Otherwise |
|---|---|---|
| 81 note | the org admin; the note's `authorUserId` | 403 `forbidden`, `required: "org_admin"` |
| 82 human message | the org admin; the message's `ownerUserId` | 403 `forbidden`, `required: "org_admin"` |
| 83 thread message | the org admin only | 403 `forbidden`, `required: "org_admin"` |

The record is found by id under row-level security alone, with no team check, as route 45
(`task-27.md:32-34`), so an admin of another org gets 404 (`note_not_found`,
`human_message_not_found`, `message_not_found`, three new codes in task 03's `codes.go`). A
thread message is agent text (`text·A`), so no person "typed" it; its sender's owner may not erase
it alone, because it is half of a negotiation the other owner relied on (`05-design.md` §6.4),
and an org admin decides. In one transaction: `SELECT hub.tombstone_<kind>($1, tx.Now)`, then
`Emit <kind>.erased`. The answer:

```go
// control-plane/internal/hub/hubapi/team.go (append)

// EraseRecordResponse answers routes 81 to 83. Erased is true only when
// this call set the tombstone; a second erase answers false, keeps the
// first time, and is still audited (an attempt is a security event,
// ADR-H1 §5.6, as Hub route 45).
type EraseRecordResponse struct {
	ID           string `json:"id"`           // nt-…, hm-… or ms-…
	Erased       bool   `json:"erased"`
	TombstonedAt string `json:"tombstonedAt"` // the first erasure time, RFC 3339 ms
}
```

**The stored reply.** Hub task 14 stores a keyed write's 2xx body in `idempotency_keys.response`
(`task-27.md:41-43`). Routes 70 and 72 answer with the words, so a keyed write would keep them
there past an erasure: Hub review I6's leak, again (`phase-h-hub-review.md:258-272`). Both routes
therefore store their reply with `text: null` and rebuild `text` and `tombstonedAt` from the row on
replay, the pattern Hub route 26 uses (A17, `task-27.md:41-46`). A replay before erasure is the
first answer byte for byte; after it, `text: null` and `tombstonedAt` set. Route 70 is task 07's;
this task changes only its stored form.

#### The migration — complete source

Task 04's append-only guards refuse **every** `UPDATE` on the three tables, grants and triggers
both (G4). A tombstone is an `UPDATE`, so it runs inside a `SECURITY DEFINER` function owned by
`hub_owner`, and each guard gains exactly the exception Hub task 07b gives `guard_revision`
(`task-07b.md:63-80`): the owner may move the body from a value to `NULL` and `tombstoned_at` from
`NULL` to a time, and nothing else. **Task 04's part is not yet written**: its guard function names
and text column names are written here as `guard_note`, `guard_message`, `guard_human_message` and
`text_body`, and are renamed mechanically to task 04's at build time. Every other column of the row
(ids, owner, target, author class, `text_agent`, `created_at`) is compared unchanged.

```sql
-- control-plane/internal/hub/migrate/sql/00NN_team_erase.sql   (NN = task 04's + 1)
SET search_path = hub, pg_temp;

-- ── the guards: a row changes once, by erasure, and only as hub_owner ────────
CREATE OR REPLACE FUNCTION guard_note() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    RAISE EXCEPTION 'hub: notes are never deleted' USING ERRCODE = 'insufficient_privilege';
  END IF;
  IF current_user <> 'hub_owner'
     OR OLD.tombstoned_at IS NOT NULL OR NEW.tombstoned_at IS NULL OR NEW.text_body IS NOT NULL
     OR (to_jsonb(NEW) - 'text_body' - 'tombstoned_at') IS DISTINCT FROM
        (to_jsonb(OLD) - 'text_body' - 'tombstoned_at')
  THEN
    RAISE EXCEPTION 'hub: a note is immutable' USING ERRCODE = 'insufficient_privilege';
  END IF;
  RETURN NEW;
END $$;
-- guard_message() and guard_human_message() are the same body with the table's noun in both
-- messages; written out in full in the file, not generated, so a grep finds each.

-- ── the tombstones: one per kind, the session's org only ─────────────────────
CREATE FUNCTION tombstone_note(p_id text, p_at timestamptz)
RETURNS TABLE (erased boolean, tombstoned_at timestamptz)
LANGUAGE plpgsql SECURITY DEFINER SET search_path = hub, pg_temp AS $$
DECLARE v_org text := current_setting('hub.org_id', true);
BEGIN
  IF v_org IS NULL OR NOT EXISTS (SELECT 1 FROM notes n WHERE n.id = p_id AND n.org_id = v_org) THEN
    RAISE EXCEPTION 'hub: no note % in this org', p_id USING ERRCODE = 'no_data_found';
  END IF;
  UPDATE notes n SET text_body = NULL, tombstoned_at = p_at
   WHERE n.id = p_id AND n.org_id = v_org AND n.tombstoned_at IS NULL;
  RETURN QUERY SELECT FOUND, n.tombstoned_at FROM notes n WHERE n.id = p_id AND n.org_id = v_org;
END $$;
-- tombstone_message(text, timestamptz) and tombstone_human_message(text, timestamptz): the same
-- body on messages and human_messages, written out in full.

REVOKE ALL ON FUNCTION tombstone_note(text, timestamptz), tombstone_message(text, timestamptz),
                       tombstone_human_message(text, timestamptz) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION tombstone_note(text, timestamptz), tombstone_message(text, timestamptz),
                          tombstone_human_message(text, timestamptz) TO hub_app;
```

`to_jsonb(NEW) - … IS DISTINCT FROM to_jsonb(OLD) - …` compares every other column without naming
them, so a column task 04 adds later is protected without an edit here. `FOUND` after the `UPDATE`
is true only when this call set the tombstone (PL/pgSQL sets it from the last statement's row
count, from knowledge, **UNVERIFIED** until the test runs), which is `EraseRecordResponse.Erased`.
The `SELECT` after it reads the first erasure time on a second call. **`idempotency_keys` is not
touched**, for the reason `erase_brief` gives (`task-07b.md:226-230`): no stored reply holds the
words (see "The stored reply").

#### Events

Task 01's §8 table gains three rows. `data` carries the `reason` in the audit only; the feed
projection drops it, as Hub task 27 does for `brief.erased` (`task-27.md:76`), because a reason
can name a client's matter.

| Kind | `subject` | `data` | In the feed |
|---|---|---|---|
| `note.erased` | note id | `{"by","reason"}`; `by` is `author` or `org_admin` | yes, without `reason` |
| `human_message.erased` | human message id | `{"by","reason"}`; `by` is `owner` or `org_admin` | fleet, without `reason` |
| `message.erased` | message id | `{"threadId","reason"}` | fleet, without `reason` |

`note.created` is task 01's row (`task-01.md:565`), feed `yes`: every member's desktop and Fleet
learn that a note exists, and read its words through route 73. A second erasure emits a second
event with the same `subject`, so an attempt always leaves a trace.

#### Fleet: a note in `panacea inbox`, and nowhere else — complete source

Task 28's `Envelope` (`task-28.md:502-508`) gains one field. `omitempty` keeps every task 09 and
task 28 golden fixture byte-identical when no note is unread, so the control wire stays `v: 1`.

```go
// control-plane/internal/fleet/protocol/inbox.go (modify: one field, one type, one func)

type Envelope struct {
	V        int          `json:"v"` // 1
	Notice   string       `json:"notice"`
	Messages []InboxItem  `json:"messages"`
	More     bool         `json:"more"`
	Threads  []ThreadView `json:"threads"`
	Notes    []NoteItem   `json:"notes,omitempty"` // task 46: unread notes for this owner, ≤ 20
}

// NoteItem is a person's note, quoted. By is the author's display name
// (never an agent id: a note is text·H); Text.Author is always "human".
// A tombstoned note is never delivered, and one erased after delivery is
// not recalled: the Orchestrator's transcript is not the Hub's to edit.
type NoteItem struct {
	ID     string             `json:"id"`     // nt-…
	Target hubapi.NoteTarget  `json:"target"`
	By     string             `json:"by"`     // "Grace Hopper"
	Text   teamtext.Text      `json:"text"`
	At     time.Time          `json:"at"`
}

// NoteRecipients is who a note is for: the owners whose Orchestrators
// receive it through inbox. A project note: every current member but the
// author. A task note: the task's owner. A user note: that user. The author
// is always removed, so a note never echoes to its own writer.
func NoteRecipients(n hubapi.Note, members []string, taskOwner func(string) string) []string
```

Fleet learns of a note from `note.created` on its per-org feed (task 08), reads it through route 73
under the recipient's `orchestrator` delegation, and holds it **only** in the in-memory unread set
that task 28's inbox already keeps per Orchestrator (no Fleet Postgres row: a note is a Hub
record, rule 2 of the global constraints). A restart rebuilds the set from route 73 with `after`
at the owner's last-read id, which task 28 already persists for messages. A `note.erased` event
drops the id from every unread set. Delivery is the inbox only: task 38's PTY writer carries a
fixed template, never a note; task 31's piggyback carries directive ids, never a note; task 16's
`team.json` and task 22's `WorkerSnapshot` have no note field. **A note addressed to me also makes
an `inform`** (`05-design.md:735`), raised by task 30's rules for the recipient's own desktop; that
row is task 30's, and this task only feeds it the note id.

#### Tests to write first

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestAnErasedNoteIsATombstoneAndItsAuditRemains` | Linus writes `nt-A` on the project (marker `ZQX-NOTE-7731` in the text); Ada (org admin) erases it → `{erased: true}`; route 73 lists `nt-A` with `text: null`, `tombstonedAt` set, target and author unchanged; the audit (route 44) still holds `note.created` for `nt-A` **and** one `note.erased` with `by: org_admin`; `markerHits` (Hub task 27) over schema `hub` is empty, `idempotency_keys` included | a tombstone that deletes the row, or the audit event with it; the stored reply keeping the words |
| `TestAWorkersTeamNeverContainsANote` | with three notes in the project (on the project, on the worker's task, on its owner), `panacea team` from a worker-class session and task 16's `team.json` snapshot contain none of the marker strings and no `nt-…`; a `worker`-class delegation calling route 73 → 403 `forbidden` (`task-01.md:126-127`); creating a `worker` delegation that lists `notes.read` → 422 `details.route` (`:299-305`) | `notes.read` added to the worker class set, or `WorkerSnapshot` given a notes field |
| `TestANoteReachesAnOrchestratorOnlyByInboxQuoted` | Grace's Orchestrator's `panacea inbox` holds `nt-A` in `notes` with `by: "Linus Torvalds"`, `text.author: "human"`; her `team`, the turn template (task 26) and the PTY writer's output (task 38) hold neither the text nor the id | a note spliced into the turn template or a nudge |
| `TestTheAuthorMayEraseTheirOwnNoteOnly` | Linus erases his `nt-A` → 200 `by: author`; Grace (team admin) erasing `nt-A` → 403 `required: "org_admin"`; Linus erasing Grace's note → 403 | the team admin treated as an eraser; the author check by team, not by user |
| `TestAThreadMessageIsErasedByTheOrgAdminOnly` | Linus (the sender's owner) → 403; Ada → 200; route 63 then lists the message with `text: null`, `kind` and typed `fields` unchanged | the sender's owner allowed to rewrite a negotiation the other side relied on |
| `TestAHumanMessageIsErasedByItsOwner` | Linus erases his `hm-…` → 200 `by: owner`; Audrey (auditor) → 403; Grace → 404 (she cannot see it, and the route does not reveal it exists) | the auditor treated as an eraser; a 403 that confirms the record exists |
| `TestASecondEraseIsFalseAndAudited` | erase twice → `erased: true`, then `false` with the same `tombstonedAt`; two `note.erased` events | an attempt that leaves no trace; the time moved by the second call |
| `TestEraseIsOrgScoped` | Xavier (admin of org 2) erasing `nt-A` → 404 `note_not_found` | a `SECURITY DEFINER` function that ignores `hub.org_id` |
| `TestTheGuardAllowsOnlyTheTombstoneShape` | as `hub_app`, a direct `UPDATE notes SET text_body = NULL` → `42501`; as `hub_owner` outside the function, changing `target_id` with the tombstone → `42501`; un-tombstoning → `42501`; `DELETE` → `42501` | a guard that trusts the caller, or allows a second write |
| `TestTheFeedHidesTheEraseReason` | Grace's `GET /v1/changes` shows `note.erased` with `by` and no `reason`; Ada's route 44 shows the reason | a client's matter number broadcast to the team |
| `TestNoteTargetsAreInTheProject` | a `tk-…` of another project → 422 `target`; a `u-…` who left the team → 422; `project` naming another `pj-…` → 422; a note on Grace's task by Linus → 201 | a target validated by prefix only |
| `TestNoteTextBoundsAndHygiene` | 1,001 runes → 422 `text_refused`; ANSI `ESC[2J` → 422; a body field `authorUserId` → 400 | the author taken from the body |
| `TestFleetCannotWriteANote` | Fleet·O under Linus's delegation `POST`s route 72 → 403 `required: "desktop"` | an Orchestrator writing notes that other Orchestrators then quote as a person's |
| `TestAReplayAfterEraseHasNoWords` | route 72 with `Idempotency-Key: k1`; erase; `k1` again → 201, `Idempotent-Replay: true`, `text: null`, `tombstonedAt` set; same for route 70 | the stored-reply leak (Hub review I6) on the team routes |

The first two are the scope paragraph's (`2026-09-25-phase-t-team-mode.md:915-917`) and are written
first. Every Hub test is an integration test (`//go:build integration`) on Hub task 06's harness
and task 07's `team2World`, plus **Ada** (org admin, `Hub.OrgAdmin`, Hub task 11's
`dev-tenant.json`) and **Xavier** (org 2). The three Fleet tests run on task 08's `hubclient` fake
and task 22's and 28's test worlds, with no database.

**Z5 rows** (task 43's matrix test; this task adds them, as the global constraints require):

| Operation | V | M | M′ | A | Au | N | X | F | F∅ | Fx | D |
|---|---|---|---|---|---|---|---|---|---|---|---|
| write a note (72) | 403 | ✓ | ✓ | ✓ | 404 | 404 | 404 | 403 | 401d | 403 | 403 |
| read notes (73) | ✓ | ✓ | ✓ | ✓ | 404 | 404 | 404 | ✓ (orchestrator class) | 401d | ✓ | 403 |
| erase a note (81) | 403 | ✓ own | 403 | 403 | 403 | 404 | 404 | 403 | 403 | 403 | 403 |
| erase a human message (82) | ∅404 | ✓ own | 404 | 404 | 403 | 404 | 404 | 403 | 403 | 403 | 403 |
| erase a thread message (83) | 403 | 403 | 403 | 403 | 403 | 404 | 404 | 403 | 403 | 403 | 403 |

The org admin, not a column of task 01's team table, gets ✓ on 81 to 83 in one extra row of the
same test. `∅404` is 404 `human_message_not_found`: a human message is invisible to anyone but its
owner and an auditor, so the erase route does not reveal it.

#### Steps

- [ ] **Step 1: Read.** Contract §7.23, §8 and task 01's team matrix (`task-01.md:471-476`,
  `:530`, `:577-579`); Hub task 27's routes 44–45 and `markerHits` (`task-27.md:22-46`, `:72`);
  Hub task 07b's `erase_brief` and `guard_revision` (`task-07b.md:63-80`, `:212-232`); task 04's
  migration as merged, to take its guard and column names.
- [ ] **Step 2: Tests.** Write the fourteen tests and the Z5 rows; run them; they fail (no routes,
  no functions).
- [ ] **Step 3: Migration.** `00NN_team_erase.sql` with task 04's real names; run
  `TestTheGuardAllowsOnlyTheTombstoneShape` and `TestEraseIsOrgScoped`; they pass.
- [ ] **Step 4: Types and codes.** `EraseRecordResponse`, the three codes, the contract count;
  `go test ./internal/hub/hubapi/` passes.
- [ ] **Step 5: Notes.** `store/notes.go` and `server/notes.go` (routes 72, 73), the stored reply
  with `text: null`, and route 70's stored reply the same; the note tests pass.
- [ ] **Step 6: Erase.** `store/teamerase.go` and `server/teamerase.go` (routes 81 to 83), the
  three events, the feed projection without `reason`; the erase tests pass.
- [ ] **Step 7: Fleet.** `Envelope.Notes`, `NoteItem`, `NoteRecipients`, the unread set, the
  `note.erased` drop; the three Fleet tests pass, and task 09's and task 28's golden fixtures are
  unchanged (`git diff --stat testdata/panacea-control/` is empty).
- [ ] **Step 8: Contract.** §7.0 rows 81 to 83, a new §7.23a "Erasure of team text", the §8 rows,
  the Z5 rows; `go test ./internal/hub/hubapi/ -run Contract` passes.
- [ ] **Step 9: Commit.** `git add` the new files, `git commit -F <msg> -- <paths>`.

#### Acceptance

```bash
cd control-plane
go test -tags integration ./internal/hub/server/ -run 'TestAnErasedNote|TestAWorkersTeam|TestTheAuthorMay|TestAThreadMessage|TestAHumanMessage|TestASecondErase|TestEraseIsOrgScoped|TestTheGuardAllows|TestTheFeedHides|TestNoteTarget|TestNoteText|TestFleetCannotWrite|TestAReplayAfterErase' -count=1
# ok  	github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/server	…
go test ./internal/fleet/protocol/ ./internal/fleet/verbs/ -run 'Note|Worker' -count=1
# ok  	…/internal/fleet/protocol	…
# ok  	…/internal/fleet/verbs	…
go test ./internal/hub/hubapi/ -count=1 && go vet ./...
# ok  	…/internal/hub/hubapi	…   (and no vet output)
```

#### What this task does not claim

- **Not erasure from storage.** A tombstone's `UPDATE` leaves the words in dead tuples until
  `VACUUM`, in WAL, and in every backup taken before it, exactly as Hub task 27 says of briefs
  (`task-27.md:113-126`). The horizon is phase 8's backup retention.
- **Not recall from an Orchestrator.** A note delivered before its erasure stays in that session's
  transcript and in any sandbox scratch the Orchestrator wrote. Rotation (task 40) ends that
  session within its rotation window; nothing edits a transcript (OD-8).
- **Not erasure of directives, escalations or tasks.** Their text is `text·A` or a question Fleet
  templated; `05-design.md` §3.3 names only notes, messages and human messages. A request to erase
  one is an org-admin procedure outside this plan.
- **Not a UI.** The desktop's note composer, list and "erase my note" action are task 52's and
  54's; this task gives them routes.
- **Not the informs.** The `inform` a note raises for its addressee is task 30's rule.

#### Gaps found in the skeleton

1. **No erasure routes in task 01's §7.0.** Its team table stops at 79 (`task-01.md:254-291`);
   task 14 took 80 (`task-14.md:52-62`). This part claims **81 to 83**. If another part claims
   them first, the lower task id keeps the numbers and this part renumbers mechanically.
2. **Who erases.** The skeleton says "Hub task 27's erasure gains tombstones"; task 27 lets only an
   org admin erase. This part adds the author (note) and the owner (human message), per
   `04-enterprise-constraints.md:209-210`, and keeps thread messages admin-only. The owner should
   confirm.
3. **Task 04's guard and column names** are unknown until its part is written; this part names
   them provisionally and says so.
4. **Task 28's `Envelope` has no notes field** (`task-28.md:502-508`); this part adds one,
   `omitempty`, so task 28's fixtures do not move. Task 28's writer should know.
5. **Route 70's stored reply** (task 07) holds the human message's words; this part changes it to
   `text: null` with a replay rebuild. Task 07's part does not mention idempotent storage.
