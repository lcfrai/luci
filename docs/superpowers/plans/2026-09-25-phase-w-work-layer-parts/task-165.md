### Task 165: the Teams app, part 2: the Desk digest, at most once per 15 minutes; ask **your own** Orchestrator from Teams

W5, stream WH with one Phase T delta, about 3 hours (an estimate, not a measurement). Depends on
task 94 (the digest and defer, W2), task 81 (`BuildDigest`, `DigestItem`, `DigestSettings`, the
janitor's digest step; scope `tasks-81-100.md:35-66`), task 157 (the bot, `teams.Authenticate`,
`Activity`, `ChannelRef`, `PlainText`, `Poster`, janitor step 6; `task-157.md`, cited `157:N`),
and Phase T's TM-07 (route 70, `POST /v1/projects/{p}/human-messages`, `T07:186-189`), TM-26 (the
kept reply, `T26:171-172`, `:620-626`, and its gap 5 at `:880-882`), TM-27 (the `text·A` reply,
`T27:28`) and TM-28 (the protocol). Phase T is read on the team worktree at `997e5869`, cited
`Tnn:N`. Parts 81, 94 and 143 do not exist, so their names are their scopes' and **UNVERIFIED**;
every `server/`, `store/` and `teams/` path is **UNVERIFIED until merge** (rules H1, H2).

Parity: closes **L-207** jointly with task 157 (05 §1 N). Design: §1 N, §4.2, §5.4 W5; OD-16; AX
§6 item 8 and anti-pattern 8 (`docs/AGENTIC-UX.md:171`, `:243`).

**What the reader does.** Dylan installs the PanaceaApp bot in Teams and writes to it: *"what is
blocking EP-150?"* He has an Orchestrator in two projects, so the bot answers with a card holding
two buttons, *Checkout* and *Evidence upload*. He taps *Checkout*. A minute later the chat shows
*Your Orchestrator (agent):* and its reply, word for word. At 09:00 his hourly Teams digest
arrives as one card: *EP-150 · landed*, *EP-151 · due tomorrow*, each linking to the Hub's https
page. His daily email digest, due at 09:05, waits to 09:15: one digest per person per 15 minutes,
across both channels. When he types *"ask Alice's orchestrator to hurry"*, it still goes to his
own Orchestrator, as his words; if that Orchestrator decides Alice's must be asked, it uses the
TM-28 protocol itself.

**Decisions made here, so nobody re-derives them:**

1. **The personal chat links itself; an admin never links it.** Task 157 links channels by a
   pairing code and a team admin (`157:27-32`). A person's own chat with the bot is different: the
   Teams sender is verified by `teams.Authenticate` (`157:104-111`) and mapped to a Hub user by
   `(tid, aadObjectId)` (`157:40-42`), and the chat is theirs. So the first personal-scope
   `message` or `conversationUpdate` (bot added) from a **mapped** user stores one `digest`
   binding: task 143's `TeamsBinding {subject {kind:"digest", id: <the user's id>}, channelRef}`
   (`tasks-141-160.md:161`), with `teamId` null (gap 1). The bot removed from the chat
   (`conversationUpdate`, `membersRemoved` holding the bot) deletes it. An unmapped sender gets a
   card, *"Sign in to PanaceaApp once, then write again."*, and nothing is stored.
2. **`digest` stays 400 on the admin route.** Task 157 answers `subject.kind "digest"` 400 *"until
   task 165"* (`157:297`). It stays 400 for good: a digest posted to a channel shares one person's
   view with everyone in it. The personal binding is written only by decision 1.
3. **One clock per person, not per channel.** Task 81's sender keeps `last_digest_at` per person.
   A channel's digest is due when its cadence (`hourly`, `daily`) has passed since that channel's
   last send **and** 15 minutes have passed since `last_digest_at`. When both are due in one sweep,
   Teams sends and email waits for the next due sweep; each channel's digest covers the events
   since its own last send, so waiting loses nothing.
4. **The card is task 81's items, nothing more.** One Adaptive Card per digest: a heading, at most
   20 lines *alias · cause · title* and *"and N more"*, one `Action.OpenUrl` per item to task 80's
   https link. Title is omitted when the item's project is *AU data only* (task 157's rule,
   `157:21`). No description, comment, request body or agent text is ever in it.
5. **An ask has no target.** A personal-scope `message` from a mapped user is an ask. The
   addressee is always the sender's own Orchestrator; the text is never read for a name. The only
   choice is which **project**, among the sender's own: exactly one with an Orchestrator → that
   one; several → a card with one `Action.Submit` per project whose data is `{askProject, text}`;
   none → *"You have no Orchestrator yet. Start one in PanaceaApp."* The text rides in the card's
   data, so nothing waits in the Hub; the submit is re-checked in full (bound, project, owner).
6. **The ask writes TM-07's human message, in-process.** The bot endpoint is inside the Hub
   (`157:203`), so it calls the store function route 70 calls, in one transaction with
   `hub.work_actor_kind = 'integration'`, `hub.work_actor_user` = the sender, and the message's
   `via = "teams"`, `toAgentId` = the sender's Orchestrator in that project (a TM-07 delta, gap 2).
   `text` is `PlainText` of the message (`157:180-185`), at most 2,000 characters (`T07:187`);
   2,001 is refused with a card naming the bound, never cut.
7. **The reply goes back only to the owner's own chat.** When TM-26's turn record for a
   human-origin turn consumed an `hm-…` that came from Teams, the Hub posts its kept reply
   (`T26:620-626`) to that chat as a plain-text message (`textFormat: "plain"`), headed *"Your
   Orchestrator (agent):"*, verbatim; `ReplyWithheld` is posted as it is; a cut reply ends
   *"(cut at 8,000 bytes)"*. For an *AU data only* project the message is only *"Your Orchestrator
   answered in PanaceaApp: <https link>"*. This is the one `text·A` that leaves for Teams, and only
   to the person who asked (task 157's channel rule stands, `157:19-21`).
8. **Nothing is read out of the reply** (OD-8): it is posted, never parsed, never stored twice.

**Files** (task 157's package is `control-plane/internal/hub/teams/`, `157:52-58`; the scope's
`teamsapp/` is not used):
- `control-plane/internal/hub/teams/digest.go` (+ `digest_test.go`) (new) — the card, the one clock
- `control-plane/internal/hub/teams/ask.go` (+ `ask_test.go`) (new) — the ask, the project card, the reply message
- `control-plane/internal/hub/teams/activity.go` ⚑ (task 157) — `Conversation.ConversationType`, `MembersAdded`/`MembersRemoved`, `ChannelRef.Personal`
- `control-plane/internal/hub/teams/post.go` ⚑ (task 157) — `Poster` gains `PostCard`
- `control-plane/internal/hub/server/work_teams.go` ⚑ (task 157) — the personal-scope branch of `POST /teams/messages`
- `control-plane/internal/hub/store/work_teams.go` ⚑ (task 157) — the personal binding, `work_teams_asks`
- `control-plane/internal/hub/server/janitor.go` ⚑ — task 81's digest step sends Teams too (not a new step); step 6 gains the ask replies
- `control-plane/internal/hub/hubapi/work_digest.go` ⚑ (task 81, its scope's name) — `DigestSettings.Teams`
- `control-plane/internal/hub/server/work_teams_integration_test.go` ⚑, `server/matrix_integration_test.go` ⚑ — this task's rows
- `integrations/teams-app/manifest.json` ⚑ (task 157) — `bots[0].scopes` from `["team"]` to `["team", "personal"]` (`157:381`); `groupChat` stays out. Task 157's intake refusal outside a linked channel (`157:33-35`) becomes the only defence for a personal chat, where it was the second (`157:404-405`); `TestIntakeInAPersonalChatIsRefused` below keeps it
- TM-07's route 70 handler and store call ⚑ — decision 6's in-process caller (gap 2)

**What is on the tree today (verified 2026-09-26).** Task 157's part exists; parts for 80, 81, 94
and 143 do not. On `feat/work-layer` there is no `control-plane/internal/hub` package at all. TM-07
(route 70) and TM-26 (the kept reply) are parts on `feat/team-mode`, not merged anywhere.

#### The wire (complete; task 143 wins on names)

```go
// hubapi, task 81's DigestSettings, one field appended (task 143's wire).
type DigestSettings struct {
	Email string `json:"email"` // "off" | "hourly" | "daily" (task 81)
	Teams string `json:"teams"` // "off" | "hourly" | "daily"; default "off"; set through task
	//                              94's settings write (its route name is UNVERIFIED)
	TeamsLinked bool `json:"teamsLinked"` // read-only: a digest binding exists (decision 1)
}
```

A `teams` value other than the three is 400 `bad_request`, field `teams`. `teams: "hourly"` with
`teamsLinked: false` is accepted and stored, and sends nothing until the chat links; the desktop's
settings row says *"Write to the PanaceaApp bot in Teams once to link it."*

```go
// teams/activity.go ⚑ (task 157): the fields this task reads, appended.
type Conversation struct {
	ID               string `json:"id"`               // "a:…" for a personal chat
	TenantID         string `json:"tenantId"`
	ConversationType string `json:"conversationType"` // "personal" | "channel" | "groupChat"
}
// Activity gains:
//	MembersAdded   []Account `json:"membersAdded"`   // conversationUpdate only
//	MembersRemoved []Account `json:"membersRemoved"`
//	Value          may also be *AskValue (an Action.Submit from decision 5's card)

// AskValue is the Action.Submit data of the project card. Everything in it is
// re-checked: it comes back through Teams and could have been edited.
type AskValue struct {
	AskProject string `json:"askProject"` // pj-…, one of the sender's own
	Text       string `json:"text"`       // the original message, PlainText'd
}

// ChannelRef gains one field. A personal ref prints "teams:<tenant>/personal/<conversation id>";
// Team, Channel and Thread are empty on it, and Channel() returns it unchanged.
//	Personal string // the conversation id, "a:…"
```

**What task 145 must hold** (a requirement on its migration; its file wins on names):
`work_teams_bindings.team_id` nullable, with `CHECK ((team_id IS NULL) = (subject_kind =
'digest'))`; a unique index on `(subject_id) WHERE subject_kind = 'digest'` (one chat per
person); `service_url text NOT NULL` if task 157's row does not already keep it; and a new table
`work_teams_asks (org_id, hm_id text PRIMARY KEY CHECK (hm_id LIKE 'hm-%'), user_id, project_id,
channel_ref, service_url, asked_at, replied_at NULL, reply_message_id NULL)`, RLS and grants like
its neighbours, `GRANT UPDATE (replied_at, reply_message_id)` only. It stores no text: the words
are TM-07's `hm-…` row, erasable there (TM-46's erase covers route 70, `T46:292`).

**TM-07's delta** (gap 2): route 70's store write accepts a second caller, the Teams bot, in-process,
when `hub.work_actor_kind = 'integration'` and `hub.work_actor_user` is the Orchestrator's owner;
the row gains `via text NULL CHECK (via IN ('teams'))`. Over HTTP nothing changes: route 70 still
takes only the owner's desktop token (`T07:186`). No other Phase T write is reachable from the bot.
