### Task 07: Hub routes, part 2: threads, messages, escalations, directives, turns, human messages

**Slice TM1-α. Depends on: 06. About 3 hours. Seam: ST.2.**

This task gives the Hub the cross-user channel's records: rows 60 to 71 of task 01's §7.0 team
table and its contract §7.18 to §7.22 (`task-01.md:249-260`, `:374-432`). **Task 01 is
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
layer (§7.2, task 31); the Hub is the second (`task-01.md:417-421`). A directive whose `origin` is
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
- `control-plane/internal/hub/server/taint.go` + `taint_integration_test.go`: the Hub's copy of
  the rule (new)
- `control-plane/internal/hub/server/team2_world_test.go`: task 06's world plus threads (new)
- `control-plane/internal/hub/server/escalations_test.go`: `TestFleetIsNeverAnAuditor`, a unit test (new)
- `control-plane/internal/hub/store/{threads,messages,escalations,directives,humanmsgs}.go` (new)
- `control-plane/internal/hub/server/server.go`: `s.registerTeamPart2()` (modify: one line)
- `control-plane/internal/hub/server/auth.go` (task 05): `Principal.Auditor bool` from a desktop
  token's `Hub.Auditor` app role, always false under a delegation (modify; task 05 leaves it to
  this task, `task-05.md:851-852`)

**Interfaces:**
- Consumes:
  - from task 01: rows 60 to 71, §7.18 to §7.22, the §8 rows `thread.opened` to
    `human_message.appended` (`task-01.md:557-564`), and §5.2's org *auditor* (`:587-591`).
  - from task 03: `Thread`, `CreateThreadRequest`, `PatchThreadRequest`, `Message`,
    `AppendMessageRequest`, `MessageKind`, `Resolution`, `Escalation`, `CreateEscalationRequest`,
    `AnswerEscalationRequest`, `Directive`, `AppendDirectiveRequest`, `DirectiveKind`, `Rung`,
    `TurnOrigin`, `RecordTurnRequest`, `HumanMessage`, `CreateHumanMessageRequest`, and
    `teamtext.Text` on every text field; the `RouteName` constants (`task-03.md:384-393`). Task 03
    wins wherever a name here differs (`task-03.md:27-32`).
  - from task 05: `Principal.Via`, `Principal.Delegation`, and `Delegable(name, h)`, through which
    every Fleet-reachable route here is registered (`task-05.md:49-53`, `:315-327`, `:837-839`).
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
The refusals follow task 01's §5.3 row for route 63 (`task-01.md:565`) exactly: a member's or
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

#### Events

Task 01's §8 rows (`task-01.md:557-564`), copied. `data` never holds text; every delegated write's
event is `v: 2`.

| Kind | `subject` | `data` | In the feed |
|---|---|---|---|
| `thread.opened` | thread id | `{"subjectKind","participants"}` (the two ids joined by `,`) | fleet |
| `thread.updated` | thread id | `{"state","rounds"}` | fleet |
| `message.appended` | message id | `{"threadId","kind","senderAgentId"}` | fleet |
| `escalation.created` | escalation id | `{"kind","deadline","fromAgentId"}` | fleet |
| `escalation.answered` | escalation id | `{"choice"}` (`-1` for an acknowledged inform) | fleet |
| `directive.appended` | directive id | `{"kind","rung","origin","fromAgentId","toAgentId","approvedBy"}` | fleet |
| `orchestrator_turn.recorded` | turn id | `{"orchestratorAgentId","origin","consumed","issued","inputTokens","outputTokens"}` (`consumed` and `issued` are counts) | no |
| `human_message.appended` | human message id | `{"toAgentId"}` | fleet |

None of these is `yes`: escalations, directives and human messages are private to their owner
(§3.4), and threads and messages to their parties, while the member feed of contract §7.11 is read
by every member. Task 08 delivers the `fleet` rows to Fleet's per-org feed only.

- [ ] **Step 1: Write the failing test: the taint rule, as a table**

`taint_test.go` is an integration test (it reads an escalation row), but its table is the rule
itself, so it is written first and every row names its mutant.

```go
// control-plane/internal/hub/server/taint_integration_test.go
//go:build integration

package server_test

func TestTaintAllows(t *testing.T) {
	w := newTeam2World(t)
	answered := w.decideAnsweredBy(t, w.Linus)      // es-… of Linus's, answered by Linus
	unanswered := w.decideUnanswered(t, w.Linus)    // es-… of Linus's, no answer
	gracesAnswered := w.decideAnsweredBy(t, w.Grace) // es-… of Grace's, answered by Grace
	informAnswered := w.informAcknowledged(t, w.Linus)

	for _, c := range []struct {
		name, origin, kind, approvedBy string
		want                           int
		mutant                         string
	}{
		{"human free", "human", "free", "", 201, "origin ignored: every free directive needs a decide"},
		{"fleet pause", "fleet", "pause", "", 201, "the three safe verbs not exempt"},
		{"fleet narrow", "fleet", "narrow", "", 201, "narrow missing from the set"},
		{"fleet wait-for", "fleet", "wait-for", "", 201, "wait-for missing from the set"},
		{"fleet resume bare", "fleet", "resume", "", 403, "resume treated as safe: it restarts a paused worker"},
		{"fleet rebase bare", "fleet", "rebase", "", 403, "rebase treated as safe"},
		{"fleet free bare", "fleet", "free", "", 403, "the rule not applied at the Hub at all"},
		{"fleet free answered", "fleet", "free", answered, 201, "a real approval refused"},
		{"fleet free unanswered", "fleet", "free", unanswered, 403, "answered_at not checked"},
		{"fleet free other owner's", "fleet", "free", gracesAnswered, 403, "owner not compared: Grace's yes approves Linus's worker"},
		{"fleet free by inform", "fleet", "free", informAnswered, 403, "kind not compared: an acknowledged inform approves"},
	} {
		t.Run(c.name, func(t *testing.T) {
			res := w.fleet(w.DLLinusOrch).post(t, w.path("directives"), w.directive(c.origin, c.kind, c.approvedBy))
			if res.Status != c.want {
				t.Fatalf("status %d, want %d; mutant: %s", res.Status, c.want, c.mutant)
			}
			if c.want == 403 {
				res.wantProblem(t, 403, "forbidden", map[string]string{"required": "decide"})
			}
		})
	}
}
```

`newTeam2World` is task 06's `teamWorld` plus one open thread between `AGLinusOrch` and
`AGGraceOrch`, a worker `AGLinusWorker` whose parent is `AGLinusOrch`, and an org *auditor*,
**Audrey**, added to `dev-tenant.json` (Hub task 11) with the `Hub.Auditor` app role and no team.
`w.directive` builds a valid body from `AGLinusOrch` to `AGLinusWorker` with `text` only for `free`.

- [ ] **Step 2: Write the failing tests for the routes**

The three the scope paragraph names come first; the rest follow. Callers are task 06's plus
`audrey()` (the auditor's desktop).

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestGraceCannotReadLinussEscalation` | Linus's `es-…` exists; Grace's desktop, `fleet(DLGraceOrch)` and Grace as team admin each `GET` route 66 → 200, list without it; Linus → it; Audrey → it | the filter applied to the desktop but not to Fleet·O, or an admin exemption |
| `TestDirectivesHaveNoPatch` | `PATCH` and `DELETE` on `…/directives/{dv}` → 405 `method_not_allowed`; a direct `UPDATE directives` as `hub_app` fails (task 04's guard) | a patch route registered, or the table grant widened |
| `TestEscalationAnsweredBySomeoneElseIs403` | Grace's desktop answers Linus's → 403 `required: "owner"`; `fleet(DLLinusOrch)` → 403 `required: "desktop"`; Linus → 200; Linus again → 409 `conflict` | the answer route accepts Fleet, so an Orchestrator answers its own question |
| `TestThreadParticipantsAreTwoOwners` | two Orchestrators of Linus → 422; one worker as participant → 422; opener not among them → 422; a participant whose owner was removed from T → 422 | the different-owner check dropped |
| `TestThreadOpenerIsTheDelegationOwners` | opener `AGGraceOrch` under `DLLinusOrch` → 403 `agent_not_owner` | rule 5 applied to `participants[1]` |
| `TestThreadStateAndRounds` | `open → resolved` → 200; `resolved → open` → 409; `rounds` 2 then 1 → 422 | `rounds` settable downward, which resets the §6.6 cap |
| `TestThreadDeadlineBounds` | 29 min → 422; 24 h 1 s → 422; 30 min → 201 | the 30-minute floor counted from the client's clock |
| `TestMessageKindsAndFields` | one valid body per kind → 201; `ask` without text → 422; `withdraw` with text → 422; `accept` of the sender's own proposal → 422; `split` whose sides are not the two participants → 422 | `fields` decoded as `map[string]any` and never validated |
| `TestMessageTextBounds` | `ask` 601 runes → 422 `text_refused`; `reject` reason 301 → 422; ANSI `ESC[2J` in text → 422 | one bound for every kind |
| `TestMessageSenderMustBeAParticipantAndTheOwners` | a third Orchestrator → 403 `required: "participant"`; `AGGraceOrch` under `DLLinusOrch` → 403 `agent_not_owner` | only one of the two checks |
| `TestMessagesAreAppendOnly` | no `PATCH`/`DELETE` route (405); `UPDATE messages` as `hub_app` fails | as for directives |
| `TestMessageToAClosedThreadIs409` | after `resolved`, append → 409 `conflict` | state not read in the append's transaction |
| `TestThreadMessagesReadByPartiesAndAuditorOnly` | route 63: `fleet(DLLinusOrch)` → 200; Linus's desktop → 403 `required: "fleet"`; Fleet under a third member's delegation → 403 `required: "party"`; Vera → 404; Audrey → 200 | the party check on the thread's project, not its participants |
| `TestEscalationShape` | `inform` with options → 422; `decide` with 1 or 5 options → 422; `default` 4 of 4 → 422; `deadline` 29 min → 422; label 81 → 422 | option count unchecked, so an Orchestrator offers one "choice" |
| `TestEscalationFromIsTheOwnersOrchestratorOrFleet` | `fromAgentId` Grace's under `DLLinusOrch` → 403 `agent_not_owner`; `""` → 201 and the event's `fromAgentId` is `""` | `""` rejected, so Fleet cannot escalate for itself |
| `TestEscalationAnswerAfterTheDeadline` | clock past `deadline`, answer → 200, `answeredAt > deadline` | late answers refused, so a human's choice is lost |
| `TestDirectiveEndsAreOwnersAndParented` | `to` Grace's worker → 403 `agent_not_owner`; `to` a Linus worker of another Orchestrator → 422 `details.field = "toAgentId"`; `text` on `pause` → 422 | the parent check dropped, so one Orchestrator steers another's worker |
| `TestDirectiveReadsAreOwnersAndAuditors` | as `TestGraceCannotReadLinussEscalation`, route 68 | as there |
| `TestTurnIsAnEventWithNoRow` | route 69 → 201; `SELECT count(*)` of every team table unchanged; one `orchestrator_turn.recorded`, absent from both feeds; no `Idempotency-Key` → 400 | a turns table added; the event in the feed |
| `TestHumanMessageToOwnOrchestratorOnly` | Linus to `AGGraceOrch` → 403 `agent_not_owner`; to `AGLinusWorker` → 422; 2,001 runes → 422; Vera → 403; `fleet(DLLinusOrch)` → 403 `required: "desktop"` | Fleet allowed to write a human message, which would make a Fleet-started turn `human` |
| `TestHumanMessageReadsAreOwnersAndAuditors` | as `TestGraceCannotReadLinussEscalation`, route 71; the text is `author: "human"` | the author stamped from the body |
| `TestTeamPart2EventsAreFleetFeedOnly` | every kind above is written with task 04's `fleet_feed = true` and is absent from Linus's own `GET /v1/changes`; the turn has `fleet_feed = false` too | a kind left `yes`, so every member reads who escalated what |

One in full, the one the whole task exists for:

```go
// control-plane/internal/hub/server/escalations_integration_test.go (excerpt)
//go:build integration

package server_test

// Mutant killed: the read filter applied in Go after an unfiltered query, with a branch
// that skips it for team admins. Grace is T's admin and still sees none of Linus's.
func TestGraceCannotReadLinussEscalation(t *testing.T) {
	w := newTeam2World(t)
	es := w.decideUnanswered(t, w.Linus)
	for name, c := range map[string]*caller{
		"grace desktop": w.grace(), "fleet under grace": w.fleet(w.DLGraceOrch),
	} {
		page := c.mustGet(t, w.path("escalations"), 200)
		if page.containsID(es) {
			t.Fatalf("%s read Linus's escalation %s", name, es)
		}
	}
	if !w.linus().mustGet(t, w.path("escalations"), 200).containsID(es) {
		t.Fatal("Linus cannot read his own escalation")
	}
	if !w.audrey().mustGet(t, w.path("escalations"), 200).containsID(es) {
		t.Fatal("the org auditor cannot read it")
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

```bash
cd control-plane && go test -tags integration ./internal/hub/server/ -run 'TestTaint|TestThread|TestMessage|TestEscalation|TestGrace|TestDirective|TestTurn|TestHumanMessage|TestTeamPart2' -count=1
```

Expected: a build failure first (`undefined: newTeam2World`, then `undefined: TaintAllows` once
the world exists); after the stubs, `FAIL` with every route answering 404 `route_not_found`.

- [ ] **Step 4: Write the store functions, the taint rule, then the handlers**

The store: one file per record, each insert a single statement `RETURNING` the row; the reads
take `owner_user_id` as a **required** parameter, and the one query that serves an auditor is a
separate function (`store.EscalationsForAuditor`), so no call site passes an empty owner to mean
"all". The append-only tables have **no** update function at all. Registration, in full:

```go
// control-plane/internal/hub/server/threads.go (excerpt)
func (s *Server) registerTeamPart2() {
	// Desktop only (routes 65, 70): no route name, so Fleet is 403 by construction.
	s.mux.HandleFunc("POST /v1/escalations/{es}/answer", s.answerEscalation)
	s.mux.HandleFunc("POST /v1/projects/{p}/human-messages", s.writeHumanMessage)

	// Fleet-reachable: each under its task 03 name (task 05 decision (d)).
	d := func(pattern string, name hubapi.RouteName, h http.HandlerFunc) {
		s.mux.Handle(pattern, Delegable(name, h))
	}
	d("POST /v1/projects/{p}/threads", hubapi.RouteThreadsCreate, s.openThread)
	d("PATCH /v1/projects/{p}/threads/{th}", hubapi.RouteThreadsUpdate, s.patchThread)
	d("POST /v1/projects/{p}/threads/{th}/messages", hubapi.RouteMessagesAppend, s.appendMessage)
	d("GET /v1/projects/{p}/threads/{th}/messages", hubapi.RouteMessagesRead, s.listMessages)
	d("POST /v1/projects/{p}/escalations", hubapi.RouteEscalationsCreate, s.createEscalation)
	d("GET /v1/projects/{p}/escalations", hubapi.RouteEscalationsRead, s.listEscalations)
	d("POST /v1/projects/{p}/directives", hubapi.RouteDirectivesAppend, s.appendDirective)
	d("GET /v1/projects/{p}/directives", hubapi.RouteDirectivesRead, s.listDirectives)
	d("POST /v1/projects/{p}/turns", hubapi.RouteTurnsRecord, s.recordTurn)
	d("GET /v1/projects/{p}/human-messages", hubapi.RouteHumanMessagesRead, s.listHumanMessages)
}
```

Two bare, ten named. Task 06's `TestEveryFleetRouteIsNamed` gains these twelve rows (ten named,
routes 65 and 70 bare); its mutant is the same, one `d(…)` written as a bare `HandleFunc`.

The private reads share one helper so the filter cannot be forgotten on one of them:

```go
// control-plane/internal/hub/server/escalations.go (excerpt)
// privateOwner answers whose rows a private read returns: the caller's own, or, for
// an org auditor's desktop, every owner's (all = true). Fleet is never an auditor.
func privateOwner(p Principal) (owner string, all bool) {
	if p.Auditor && p.Via == "" {
		return "", true
	}
	return p.UserID, false
}
```

`Principal.Auditor` is read from a desktop token's `roles` claim (`Hub.Auditor`, task 01 §5.2)
where Hub task 13 reads `Hub.OrgAdmin` for `OrgAdmin`, and task 05's `resolveDelegated` leaves it
false exactly as it leaves `OrgAdmin` false (task 05 decision (e), `task-05.md:875-876`). So the
`p.Via == ""` term above is a second layer, not the only one. Its test is a plain unit test (no
database) in `escalations_test.go`, written with Step 1's and run by the default `go test`:

```go
// Mutant killed: the p.Via == "" term dropped, so a principal that is both
// delegated and marked Auditor (task 05's decision (e) regressed) reads every
// member's escalations through Fleet.
func TestFleetIsNeverAnAuditor(t *testing.T) {
	if o, all := privateOwner(Principal{UserID: "u-L", Auditor: true, Via: hubapi.ViaFleet}); all || o != "u-L" {
		t.Fatalf("delegated auditor: owner %q, all %v", o, all)
	}
	if _, all := privateOwner(Principal{UserID: "u-A", Auditor: true}); !all {
		t.Fatal("a desktop auditor was narrowed to its own rows")
	}
}
```

- [ ] **Step 5: Run the tests to verify they pass**

```bash
cd control-plane && go test -tags integration ./internal/hub/server/ -count=1
```

Expected: `ok  	github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/server	<n>s`, with
task 06's tests still passing. Two failures are worth predicting. `TestTaintAllows/fleet_free_by_inform`
failing means `AnsweredDecide` dropped its `kind = 'decide'` term; it is the easiest to lose,
because an acknowledged inform also has `answered_at`. `TestTurnIsAnEventWithNoRow` failing on the
feed means the turn's kind was not given `fleet_feed = false`, the only `no` row in this task.

- [ ] **Step 6: Vet, format and the whole suite**

```bash
gofmt -l control-plane/internal/hub
cd control-plane && go vet ./... && go test ./... -count=1
cd control-plane && go test -tags integration ./internal/hub/... -count=1
```

`gofmt -l` prints nothing; no line says `FAIL`. Without Hub task 06's Postgres the integration runs
are deferred to task 33, and the record says so.

- [ ] **Step 7: Commit**

Write `.superpowers/sdd/team/msg/t07.txt` (what changed; that the private reads filter in SQL;
that the Hub re-checks the taint rule as a second layer), ending with
`Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>`, then:

```bash
git add control-plane/internal/hub/server/{threads,escalations,directives,turns,humanmsgs,taint}*.go \
        control-plane/internal/hub/server/team2_world_test.go \
        control-plane/internal/hub/store/{threads,messages,escalations,directives,humanmsgs}.go
git commit -F .superpowers/sdd/team/msg/t07.txt -- control-plane/internal/hub/server control-plane/internal/hub/store
```

#### Acceptance

1. Step 5 passes, and the three tests the scope paragraph names exist by those names:
   `TestGraceCannotReadLinussEscalation` (Bob cannot read Alice's), `TestDirectivesHaveNoPatch`,
   `TestEscalationAnsweredBySomeoneElseIs403`.
2. `grep -c 's.mux.HandleFunc(' control-plane/internal/hub/server/threads.go` prints `2` (routes 65,
   70) and `grep -c '^	d("' control-plane/internal/hub/server/threads.go` prints `10`.
3. `grep -n 'func.*Update\|UPDATE ' control-plane/internal/hub/store/{messages,directives,humanmsgs}.go`
   prints nothing: the append-only records have no update path in Go (task 04 holds the database
   half).
4. `grep -n 'owner_user_id' control-plane/internal/hub/store/{escalations,directives,humanmsgs}.go`
   shows the owner in the `WHERE` of every non-auditor read.
5. Every `TestTaintAllows` sub-test passes, eleven of them.

#### What this task does not claim

- **Not that the approval matches the directive.** `TaintAllows` proves a `decide` of the same
  owner was answered by that owner; it does not prove the chosen option authorised *this*
  directive. That needs a typed link from option to directive, which task 01's contract does not
  carry. Fleet's verb handler (task 31) holds the tighter check; the Hub's is the floor.
- **Not that `origin: "human"` is true.** The Hub records turn origin; it cannot see Fleet's batch
  (task 01's R12).
- **Not notes or erasure.** Notes are route 72, task 46; tombstoning a human message or a message
  on erasure is also task 46. The `tombstoned_at` columns exist (task 04) and nothing sets them yet.
- **Not the Z5 matrix rows**, including *compliance* (task 43). This task's tests cover owner,
  teammate, admin, viewer, auditor and Fleet under each delegation.
- **Not delivery.** Nothing here reaches an Orchestrator or a human; Fleet's outbox (task 11) and
  the cockpit (task 52) do.
- **Not that it runs as built.** The Hub's task 13, 14 and 17 files are planned, not built, on
  2026-09-25; **UNVERIFIED until merge**.

#### Gaps found in the skeleton

1. **The auditor principal.** §3.4 and task 01 §5.2 give reads to an org *auditor*, but neither
   task 05's scope nor task 43's ("*auditor* and *compliance*") says which task adds
   `Principal.Auditor` before task 07 needs it. Task 05 leaves it here (`task-05.md:851-852`), so
   this part adds it; task 43 should only add its matrix rows. The skeleton's scope for 07 should
   say so.
2. **Thread and escalation reads for rebuild.** Task 11's `rebuild.go` needs Fleet's open threads
   and unanswered escalations after a restart. Route 63 lists a thread's messages but there is no
   route that lists **threads**. Fleet can rebuild them from its own feed cursor only if it never
   loses the cursor; a `GET …/threads?open=true` (Fleet·O, a participant's owner) would close it.
   Not added here, because task 01 does not list it.
3. **`Idempotency-Key` required on route 69.** Task 01 does not say so; a turn has no row, so a
   reused `ot-…` is otherwise undetectable. Task 01's §7.21 should state it.
