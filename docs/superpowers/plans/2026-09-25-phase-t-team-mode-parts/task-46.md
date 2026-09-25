### Task 46: Hub delta: notes and erasure tombstones

**Slice TM1. Depends on: 07, H27. About 2 hours. Seam: ST.2.**

This task gives the Hub the one piece of team text a person types *about other people*: the note
(`05-design.md` §3.3 row "note", `:312`), routes 72 and 73 of task 01's §7.0 team table and its
contract §7.23 (`task-01.md:277-278`, `:462-467`). It also closes `07` M5 (`07-critique-security.md:490`):
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
outside the `worker` class route set (`task-01.md:294-296`), so a worker-class delegation cannot
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
  - from task 01: rows 72 and 73, §7.23, the `note.created` §8 row (`task-01.md:530`), the team
    matrix row "write a note (72)" (`:577`) and "read … notes (73)" (`:579`).
  - from task 03: `Note`, `NoteTarget`, `NoteTargetKind`, `CreateNoteRequest`, `RouteNotesRead`,
    `PrefixNote` (`task-03.md:869-888`, `:387`, `:1086`); `HumanMessage` and `Message`, whose
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
  "text": { "body": "…", "author": "human", "by": "u-…" },     // task 02's Text, as stamped
  "createdAt": "…", "tombstonedAt": null }
```

The target must be in `{p}`: `project` names `{p}` itself; `task` a task of `{p}` (task 06's
`tasks` row, any owner); `user` a **current** member of `{p}`'s team, the author included. Any
other target is 422 `bad_request`, `details.field = "target"`. A note on another member's task is
allowed (matrix column `M′`, `task-01.md:577`): writing about someone's work is not writing their
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
