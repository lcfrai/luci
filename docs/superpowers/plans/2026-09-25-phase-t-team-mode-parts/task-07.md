### Task 07: Hub routes, part 2: threads, messages, escalations, directives, turns, human messages

**Slice TM1-α. Depends on: 06. About 3 hours. Seam: ST.2.**

This task gives the Hub the cross-user channel's records: rows 60 to 71 of task 01's §7.0 team
table and its contract §7.18 to §7.22 (`task-01.md:243-254`, `:365-423`). **Task 01 is
authoritative for every field, bound, code and kind; where this part disagrees, task 01 wins.** It
is a delta on the Hub branch, built after the sync-merge of `feat/hub`, on the same sibling files
task 06 names (Hub task 14's `tx.go`, Hub task 17's `ProjectRole`, both **UNVERIFIED until
merge**), and it reuses task 06's four guards unchanged.

**The one thing this task must not get wrong is who may read.** Every record here but the thread
itself is private. Escalations, directives and human messages are readable by **their owner and an
org *auditor* only**; thread messages by **the parties' owners (through Fleet·O) and an auditor
only** (§3.4 rows 3 and 4). A teammate, a team admin and an org admin read none of them. The
reads therefore filter in SQL on `owner_user_id = tx.P.UserID`, never in Go after the fact, so a
missed branch returns an empty list rather than someone else's decision. A second teammate's
`GET` is `200` with an empty list (task 01's `∅`), not 403, so the route does not reveal that
records exist.

**The second thing is that the Hub re-checks the taint rule.** Fleet's verb handler is the first
layer (§7.2, task 31); the Hub is the second (`task-01.md:408-412`). A directive whose `origin` is
`fleet` and whose `kind` is not `pause`, `narrow` or `wait-for` must cite, in `approvedBy`, a
`decide` escalation **of the same owner, answered by that owner**. Otherwise it is 403 `forbidden`,
`required: "decide"`, and nothing is written. A compromised Fleet can still write in the owner's
name (task 01's R12), but it cannot make a Fleet-started turn look approved without a human answer
on record.

**Files:**
- `control-plane/internal/hub/server/threads.go` + `threads_integration_test.go` (new)
- `control-plane/internal/hub/server/escalations.go` + `escalations_integration_test.go` (new)
- `control-plane/internal/hub/server/directives.go` + `directives_integration_test.go` (new)
- `control-plane/internal/hub/server/turns.go` + `turns_integration_test.go` (new)
- `control-plane/internal/hub/server/humanmsgs.go` + `humanmsgs_integration_test.go` (new)
- `control-plane/internal/hub/server/taint.go` + `taint_test.go`: the Hub's copy of the rule (new)
- `control-plane/internal/hub/server/team2_world_test.go`: task 06's world plus threads (new)
- `control-plane/internal/hub/store/{threads,messages,escalations,directives,humanmsgs}.go` (new)
- `control-plane/internal/hub/server/server.go`: `s.registerTeamPart2()` (modify: one line)
- `control-plane/internal/hub/server/auth.go` (task 05): `Principal.Auditor bool` from the
  token's `Hub.Auditor` app role (modify, if task 05 has not already added it; see "Gaps")

**Interfaces:**
- Consumes:
  - from task 01: rows 60 to 71, §7.18 to §7.22, the §8 rows `thread.opened` to
    `human_message.appended` (`task-01.md:488-495`), and §5.2's org *auditor* (`:508-512`).
  - from task 03: `Thread`, `OpenThreadRequest`, `PatchThreadRequest`, `Message`,
    `AppendMessageRequest`, `MessageKind`, `Resolution`, `Escalation`, `CreateEscalationRequest`,
    `AnswerEscalationRequest`, `Directive`, `AppendDirectiveRequest`, `DirectiveKind`, `Rung`,
    `TurnOrigin`, `RecordTurnRequest`, `HumanMessage`, `WriteHumanMessageRequest`, and
    `teamtext.Text` on every text field. **Task 03's part is not yet written; where it names a
    type differently, task 03 wins and this part is renamed mechanically.**
  - from task 04: tables `threads`, `messages` (append-only), `escalations`, `directives`
    (append-only), `human_messages` (append-only, `tombstoned_at`), each keyed by `org_id` under
    RLS, with the database `CHECK` bounds that mirror task 02.
  - from task 06: `RequireDesktop`, `RequireFleet`, `OwnedAgent`, `EitherAgent`, the `teamWorld`
    test fixture and `everyTeamWrite`.
- Produces: twelve handlers (routes 60 to 71); `server.TaintAllows`; `store.AnsweredDecide`;
  the §8 kinds of this task's records.
- Required by: task 08 (feed kinds, `hubclient.Team` methods for these routes), task 11 (Fleet's
  outbox delivers every one of these writes), tasks 28 to 31 (Fleet's protocol, escalation and
  directive verbs call them), task 43 (Z5 rows), task 46 (notes, and erasure of human messages
  and messages), task 52 (the desktop reads its own escalations and writes human messages).

#### The routes — complete wire

Conventions are task 06's: plain-string text in requests, stamped `Text` in answers; Fleet mints
every id but `hm-…`; a reused id with another body is 409 `conflict`; strict decoding refuses any
owner, actor, `via`, delegation or session-id field. Route names are task 01's.

**Route 60, `POST /v1/projects/{p}/threads`**: Fleet·O, `threads.create`, the opener's owner.

```jsonc
{ "id": "th-…", "openerAgentId": "ag-…",                 // the delegation owner's Orchestrator
  "participants": [ "ag-…", "ag-…" ],                     // exactly 2 Orchestrators, different owners, opener among them
  "subject": { "kind": "conflict" | "lease" | "free", "id": "cf-…" | "ls-…" | "" },
  "deadline": "…" }                                       // Fleet-set: 30 min to 24 h after tx.Now
// 201
{ "id": "th-…", "projectId": "pj-…", "openerAgentId": "ag-…", "participants": [ … ],
  "subject": { … }, "deadline": "…", "state": "open", "rounds": 0, "revision": 1, "createdAt": "…" }
```

Both participants must be live Orchestrators (`role = orchestrator`, not ended) of **current**
project members; a participant whose owner left the team is 422 `bad_request`, `details.field =
"participants"`. A `conflict` subject must name a conflict in `{p}` whose `agentA` or `agentB`
belongs to a participant's owner; a `lease` subject a lease in `{p}`; a `free` subject has `id` "".

**Route 61, `PATCH …/threads/{th}`**: Fleet·O, `threads.update`, the owner of **either**
participant (`EitherAgent`). Body `{ "revision", "state"?, "rounds"? }`. `state` moves `open →
resolved | escalated | expired` only (409 `conflict` otherwise); `rounds` only grows (a smaller or
equal value is 422 `bad_request`, `details.field = "rounds"`).

**Route 62, `POST …/threads/{th}/messages`**: Fleet·O, `messages.append`, **append-only**.

```jsonc
{ "id": "ms-…", "senderAgentId": "ag-…", "kind": "ask", "fields": { … }, "text": "…", "templated": true }
```

| `kind` | `fields` | `text` |
|---|---|---|
| `ask` | `{"subject": {"kind": "task"\|"lease"\|"path"\|"conflict", "ref"}}` | required, ≤ 600 |
| `answer` | `{"inReplyTo": "ms-…"}`, a message of this thread | ≤ 600, only when `templated` is `false`; required then |
| `inform` | `{"subject": …}` as `ask` | required, ≤ 600 |
| `propose` · `counter` | `{"resolution": Resolution}` | optional rationale, ≤ 600 |
| `accept` · `reject` | `{"proposal": "ms-…"}`, a `propose` or `counter` of this thread by the **other** party | optional reason, ≤ 300 |
| `withdraw` | `{"proposal": "ms-…"}`, the sender's own | refused |

`Resolution` is one of `{"kind":"split","sides":{"<ag>":[globs],"<ag>":[globs]}}` (the two keys
are exactly the participants), `{"kind":"sequence","first":"ag-…"}` (a participant),
`{"kind":"yield","globs":[…]}`, `{"kind":"build-on","base":"ag-…"}` (a participant),
`{"kind":"proceed-both","paths":[…]}`; globs ≤ 20 per set, `ValidGlob` each. A sender who is not a
participant is 403 `forbidden`, `required: "participant"`; one who is not the delegation owner's is
403 `agent_not_owner`; a message to a thread not `open` is 409 `conflict`. **The Hub stores
`templated` as sent but does not judge it**: whether a free-text `answer` was allowed is the
owner's standing permission, which Fleet holds (§6.5, task 30). Text is always `author: "agent"`.

**Route 63, `GET …/threads/{th}/messages`**: Fleet·O under the delegation of **a participant's
owner** (`messages.read`), or an org *auditor*'s desktop. Oldest first, `hubapi.Page[Message]`.
The refusals follow task 01's §5.3 row for route 63 (`task-01.md:532`) exactly: a member's or
admin's desktop 403 `forbidden`, `required: "fleet"`; Fleet under a non-party's delegation 403
`forbidden`, `required: "party"`; a viewer, a non-member and another org 404 `thread_not_found`.

**Route 64, `POST …/escalations`**: Fleet·O, `escalations.create`, under the delegation of the
escalation's **owner**, the human being asked. "Fleet for itself" (§6.5 "written by Fleet") is
`fromAgentId: ""` under that same owner's delegation; there is no Fleet principal without one.

```jsonc
{ "id": "es-…", "fromAgentId": "ag-…" | "",               // the owner's Orchestrator, or "" (Fleet)
  "kind": "inform" | "decide", "threadId": "th-…",        // optional; a thread with a participant of the owner's
  "question": "…",                                        // ≤ 300
  "options": [ { "label": "Pause my worker", "consequence": "Nothing is lost; it waits" } ],   // 0 for inform; 2–4 for decide
  "default": 0,                                           // an option index; required for decide, refused for inform
  "deadline": "…" }                                       // ≥ 30 min after tx.Now; required for decide
// 201
{ "id": "es-…", "projectId": "pj-…", "ownerUserId": "u-…", "fromAgentId": "ag-…", "kind": "decide",
  "threadId": "th-…", "question": Text, "options": [ { "label": Text, "consequence": Text } ],
  "default": 0, "deadline": "…", "answer": null, "createdAt": "…" }
```

`label` ≤ 80, `consequence` ≤ 200, all `text·A`. The Hub checks that `deadline` is at least 30
minutes out and that `default` is a valid index; **which option is safe is Fleet's judgment** (§6.5),
and the Hub records it, it does not second-guess it.

**Route 65, `POST /v1/escalations/{es}/answer`**: **the owner's own desktop only**. Body
`{ "choice": 1, "text": "…" }`; `choice` an option index (a `decide`) or absent (an `inform`, which
takes an acknowledgement with optional text); `text` ≤ 300, `author: "human"`. Anyone else is 403
`forbidden` (Fleet included, `required: "desktop"`; a teammate or admin, `required: "owner"`); a
second answer is 409 `conflict`. An answer after the deadline is **accepted**: task 01 does not
refuse it, Fleet has already applied the safe default, and the human's later choice is still a
fact Fleet acts on (task 30), which it tells apart by comparing `answeredAt` with `deadline`.
→ `200` the escalation with `answer: {"choice", "text", "answeredAt"}`.

**Route 66, `GET …/escalations`**: the owner's desktop or Fleet·O under the owner's delegation
(`escalations.read`), filtered to `owner_user_id = tx.P.UserID`; an org *auditor* sees every
owner's. Everyone else in the project gets `200` and an empty list.

**Route 67, `POST …/directives`**: Fleet·O, `directives.append`, **append-only**.

```jsonc
{ "id": "dv-…", "fromAgentId": "ag-…", "toAgentId": "ag-…",   // the owner's Orchestrator; the owner's worker
  "kind": "pause" | "resume" | "narrow" | "wait-for" | "rebase" | "free",
  "text": "…",                                            // ≤ 600; required for free, refused otherwise
  "rung": "piggyback" | "inbox-socket" | "pty-nudge",
  "origin": "human" | "fleet",
  "read": [ "ms-…", "tk-…", "nt-…", "es-…" ],             // ≤ 100: what the session read since its human last spoke
  "approvedBy": "es-…" }                                  // optional; see the taint rule
```

`to` must be a worker whose `parentAgentId` is `from`, so an Orchestrator directs only its own
workers (§6.2, "neither side can move the other side's worker"). Both ends failing ownership is 403
`agent_not_owner` with `details.agentId` naming the first that failed. Then the taint rule below.

**Route 68, `GET …/directives`**: as route 66, `directives.read`.

**Route 69, `POST …/turns`**: Fleet·O, `turns.record`. Body `{ "id": "ot-…",
"orchestratorAgentId", "origin", "consumed": [≤ 200 event ids], "issued": [≤ 100 dv-/ms-/es- ids],
"tokens": {"input", "output"} }` (non-negative integers). **One audit event, no table row**
(§3.3 "audit event only"). → `201 {"id": "ot-…"}`. A replay with the same key answers the same;
the Hub cannot detect a reused `ot-…` without a row, so `Idempotency-Key` is **required** on this
route (400 `bad_request`, `details.field = "Idempotency-Key"`, without it).

**Route 70, `POST …/human-messages`**: the owner's desktop, role `member` or `admin`, to their own
Orchestrator. Body `{ "toAgentId": "ag-…", "text": "…" }`, `text` ≤ 2,000, `author: "human"`.
The Hub mints `hm-…`. `toAgentId` not the caller's Orchestrator is 403 `agent_not_owner`. → `201`
`{ "id", "projectId", "ownerUserId", "toAgentId", "text": Text, "createdAt", "tombstonedAt": null }`.

**Route 71, `GET …/human-messages`**: as route 66, `human_messages.read`.

#### The taint rule — complete source

```go
// control-plane/internal/hub/server/taint.go
package server

import (
	"context"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/store"
)

// fleetTurnVerbs are the only directive kinds a Fleet-started turn may issue on its
// own (§7.2): they pause, narrow or sequence the Orchestrator's own workers, and none
// of them can make a worker do something new.
var fleetTurnVerbs = map[hubapi.DirectiveKind]bool{
	hubapi.DirectivePause:   true,
	hubapi.DirectiveNarrow:  true,
	hubapi.DirectiveWaitFor: true,
}

// TaintAllows is the Hub's second copy of Fleet's verb check (task 31 holds the
// first). A directive from a Fleet-started turn that is not one of the three needs
// approvedBy to name a decide escalation of the same owner, answered by that owner.
func TaintAllows(ctx context.Context, tx *Tx, req hubapi.AppendDirectiveRequest) error {
	if req.Origin == hubapi.TurnOriginHuman || fleetTurnVerbs[req.Kind] {
		return nil
	}
	if req.ApprovedBy == "" {
		return Forbidden("decide")
	}
	ok, err := store.AnsweredDecide(ctx, tx, req.ApprovedBy, tx.P.UserID)
	if err != nil {
		return err
	}
	if !ok {
		return Forbidden("decide")
	}
	return nil
}
```

`store.AnsweredDecide(ctx, tx, esID, ownerUserID) (bool, error)` is one query: the escalation
exists in the org, `kind = 'decide'`, `owner_user_id = $2`, and `answered_at IS NOT NULL`. It
does **not** check which option was chosen. That is deliberate and it is a limit: the Hub proves a
human was asked and answered, not that the answer was *yes*. Matching a directive to a chosen
option needs a typed option-to-directive map the contract does not have (see "What this task does
not claim").

**Why an `origin: "human"` directive passes without a check.** Turn origin is Fleet's fact
(task 26): a turn is `human` when its batch holds a message from the Orchestrator's own human. The
Hub cannot see the batch, so it cannot verify origin; it records it, and task 01's R12 names the
residue (a compromised Fleet can claim `human`). Route 69's turn record carries the same origin and
its `consumed` ids, so an auditor can cross-check a `human` directive against a human message.
