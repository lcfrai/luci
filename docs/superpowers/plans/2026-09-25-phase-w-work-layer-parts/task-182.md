### Task 182: Slack, after Teams: intake, feature feeds and the Desk digest (L-206)

Stream **WL**, Hub, about 3 hours (an estimate, not a measurement); seam **SW.22** consumed.
Depends on task 157 (the Teams app, part 1: intake, thread sync, feature and view feeds), task
165 (the Desk digest in Teams and its cross-channel clock), task 156 (outbound webhooks, SW.22)
and task 174. Tasks 143, 146, 147 and 148 are under them. None of those parts is written at this
writing, so every name taken from them below is from their **scope paragraphs** and is
**UNVERIFIED** until the part exists; where a part names a thing differently, the part wins and
this one follows it.

05 row L-206 and the *"Teams first, Slack next"* clause of rows L-80, L-97 and L-137. Slack is a
**second provider behind task 157's bridge, never a second design**: a Slack message becomes a
request through the same store call, routed by the same rules, with its thread appended the same
way; a feed goes out through the same janitor step under the same 15-minute bound; the digest
is the same digest under the same per-person clock.

**Task 157 names no provider interface** (its scope, `tasks-141-160.md:680-712`, is Teams-shaped:
`hub/teams/{bot,activity,post}.go`). So this task names the smallest one that lets the shared
steps take either provider, `chat.Poster`, and re-points task 157's feed step and task 165's
digest step at it (⚑ both). The intake path needs no interface: it is task 146's store function,
which Teams and Slack both call. If task 157's part defines a bridge, its names win and this
part's `chat` package collapses into it. Named in the return as a skeleton gap, with a second
one: task 157's scope writes the Teams package as `hub/teams/` and task 165's as `hub/teamsapp/`
(`tasks-161-180.md:230`).

**The data rule, stated because Slack is outside the Microsoft tenancy.**
- **Inbound, text is welcome.** A message becomes a request's verbatim quote, `text·H` of its
  author (05 §3.5 step 2; task 146's `rq-`). Thread replies arrive as the request's messages.
- **Outbound, nothing leaves as customer text.** A feed item, an unfurl and the digest carry
  SW.22's metadata (ids, changed field names, actor), the display alias, the state's word and an
  https link to task 80's read-only issue page, which Entra protects. **Never** a title, a
  description, a quote or a comment. This is stricter than Teams (task 157 posts a person's
  comment back into the Teams thread); Slack gets no post-back at all, so there is no echo loop to
  guard, and the test still checks a canary never leaves.
- **Acts from Slack are the linked person's, or nothing.** A Slack user is a person only through
  a `slack_person_links` row that person made on a Hub page behind Entra. Unlinked, a Slack user
  can send a message to intake (the requester is then `kind: "slack"` with a display name) and
  can do nothing else.
- **No fact from chat.** Slack's actions are *Open* and *Subscribe* only. No state, assignee or
  priority action exists, and no act from Slack writes a fact state or `done` (SW.3).

**Install is registration, not an OAuth flow.** An org admin records the workspace id and two
secret references, the app's signing secret and its bot token, which an operator places in Key
Vault: the `secret_ref` pattern task 144 proposes for webhook secrets
(`tasks-141-160.md:241-245`; Key Vault access is itself a **proposed Hub-contract addition**,
since `hub-contract.md:2083-2085` says the Hub has none in v1). Slack's OAuth install is not built.

**Files** (the File map has no row for chat; these follow its `hub/` layout and are proposals):
- `control-plane/internal/hub/chat/poster.go` (+ test) — `Poster`, `Post`, the provider-neutral
  feed and digest message shapes (new)
- `control-plane/internal/hub/chat/slack/verify.go` (+ test) — `Verify` (new)
- `control-plane/internal/hub/chat/slack/events.go` (+ test) — the Events API: `url_verification`,
  thread replies, `link_shared`; `event_id` dedup (new)
- `control-plane/internal/hub/chat/slack/interact.go` (+ test) — the shortcut and the two actions;
  `Dispatch` (new; the scope's file list had no home for interactivity, so this file is added)
- `control-plane/internal/hub/chat/slack/feeds.go`, `unfurl.go`, `digest.go` (+ tests) — the
  outbound shapes and `SlackPoster` (new)
- `control-plane/internal/hub/chat/slack/testdata/*.json` — golden payloads written from Slack's
  public documentation, **UNVERIFIED** (slack.com may be blocked on the corporate network) (new)
- `control-plane/internal/hub/server/work_slack.go` (+ `_integration_test.go`) — the two Slack
  routes, the install routes, the person-link page beside task 148's `server/work_forms.go` (new)
- `control-plane/internal/hub/store/work_slack.go` — installs, channel links, person links, link
  nonces, seen events (new)
- `control-plane/internal/hub/migrate/sql/NNNN_work_wl_slack.sql` — the next free number at sync
  time (new)
- `control-plane/internal/hub/hubapi/work_intake.go` ⚑ (task 143, SW.21) — `"slack"` added to
  `Request.channel` and `requester.kind`; `SlackInstall`, `SlackChannelLink`, `SlackPersonLink`;
  `DigestSettings.slack` (modified)
- `control-plane/internal/hub/webhooks/deliver.go` ⚑ (task 156) — sink kind `slack` (modified)
- `control-plane/internal/hub/teams/post.go` ⚑ (task 157) and the digest step of task 165 ⚑ — call
  `chat.Post` (modified; behaviour unchanged, task 157's and 165's tests re-run as the proof)
- `control-plane/internal/hub/server/matrix_integration_test.go` ⚑ — the new routes' rows (modified)

#### Interfaces — complete

```go
// hubapi/work_intake.go (task 143, SW.21) — additions. Two closed enums gain a value: the
// desktop's Triage row (task 170) renders an unknown channel by its word, so an older desktop
// shows "slack" rather than failing; the conformance row pins it.
const (
	ChannelSlack       = "slack" // Request.Channel
	RequesterKindSlack = "slack" // Request.Requester.Kind: an unlinked Slack user; displayName only
)

// SlackInstall is one workspace registered to an org. The two refs name Key Vault secrets
// (task 144's secret_ref pattern); no secret is ever in a column, a response or a log.
type SlackInstall struct {
	SlackTeamID     string `json:"slackTeamId"` // Slack's workspace id, "T…"
	SigningSecretRef string `json:"signingSecretRef"`
	BotTokenRef     string `json:"botTokenRef"`
	InstalledBy     User   `json:"installedBy"`
	InstalledAt     string `json:"installedAt"`
}

// SlackChannelLink routes intake from one Slack channel to one Hub team, and carries that
// team's feed bindings through task 157's binding record with provider "slack".
type SlackChannelLink struct {
	SlackTeamID string `json:"slackTeamId"`
	ChannelID   string `json:"channelId"` // "C…"
	TeamID      string `json:"teamId"`    // tm-…
	LinkedBy    User   `json:"linkedBy"`
	LinkedAt    string `json:"linkedAt"`
}

// SlackPersonLink is the only way a Slack act is a person's act. Made by that person, signed in
// with Entra, on the Hub's link page; removable by them or an org admin.
type SlackPersonLink struct {
	UserID      string `json:"userId"`      // u-…
	SlackTeamID string `json:"slackTeamId"`
	SlackUserID string `json:"slackUserId"` // "U…"
	LinkedAt    string `json:"linkedAt"`
}

// DigestSettings (task 165's, which added `teams`) gains one field. Default "off".
//	Slack string `json:"slack"` // "off" | "hourly" | "daily"
```

```go
// chat/poster.go — the one seam this task adds. Both providers implement it.
package chat

// FeedItem is one line of a feed post: metadata only. There is no field for a title, a body or
// a quote, so a formatter cannot put one in (the canary test checks the output anyway).
type FeedItem struct {
	Alias     string // "EP-142"
	Kind      string // SW.22 event kind
	StateWord string // the state's word, "" when the event is not a state change
	Actor     string // the actor's display name, or "an agent" / "an app" by Actor.Kind
	Link      string // task 80's https issue page
}

// DigestCounts is the Desk digest as Slack carries it: each Desk section's measured count
// and one link. Teams renders task 81's DigestItem list (task 165); Slack carries counts only.
type DigestCounts struct {
	Sections []struct{ Name string; Count int } // Desk's own section names, in Desk order
	Link     string                              // the Desk, https
}

type Poster interface {
	Provider() string                                                      // "teams" | "slack"
	PostFeed(ctx context.Context, channelRef string, items []FeedItem) error
	PostDigest(ctx context.Context, userID string, d DigestCounts) error   // to that person only
}

// DigestWindow is AX anti-pattern 8's bound (docs/AGENTIC-UX.md:243): one digest per person per
// window across every provider.
const DigestWindow = 15 * time.Minute

// ClaimDigest takes the person's one digest slot for now, in tx, with SELECT … FOR UPDATE on task
// 165's per-person clock row (its table name wins): false when a digest by any provider was sent
// inside DigestWindow. The caller sends only after true, and a send failure does not give the
// slot back (a retry waits for the next window rather than risking two).
func ClaimDigest(ctx context.Context, tx pgx.Tx, userID string, now time.Time) (bool, error)
```

```go
// chat/slack/verify.go
// MaxSkew is Slack's documented replay window (UNVERIFIED here): a timestamp more than five
// minutes from now, in either direction, is refused.
const MaxSkew = 5 * time.Minute

// Verify checks X-Slack-Signature against v0=hex(HMAC-SHA256(secret, "v0:" + ts + ":" + body))
// with hmac.Equal, and ts (X-Slack-Request-Timestamp, unix seconds) within MaxSkew of now. It
// reads the raw body the handler buffered (at most 1 MiB, else 413) before any JSON decode.
func Verify(secret []byte, ts, sig string, body []byte, now time.Time) error // ErrBadSignature | ErrStale
```

```go
// chat/slack/interact.go — the whole dispatch table. A test reads it and fails on any verb
// outside these three: there is no state, assignee, priority, label or done action.
var Dispatch = map[string]Handler{
	"send_to_panacea": sendToPanacea, // the message shortcut (callback_id): intake, below
	"open":            ackOnly,        // a URL button; Slack opens the link, the Hub records nothing
	"subscribe":       subscribe,      // the linked person subscribes to the issue (task 12's rule)
}
```

```go
// server/work_slack.go — routes. The two Slack routes are authenticated by Slack's signature,
// not by Entra; every other route here is Entra as usual. Each one's org: *The org line*, at the end.
//
// POST /v1/integrations/slack/events          Slack signature; url_verification, event_callback
// POST /v1/integrations/slack/interactivity   Slack signature; form-encoded `payload`
// PUT  /v1/orgs/{orgId}/slack-installs/{slackTeamId}   org admin of {orgId}; {signingSecretRef, botTokenRef}
// DELETE /v1/orgs/{orgId}/slack-installs/{slackTeamId} org admin of {orgId}; also stops every Slack post
// PUT  /v1/teams/{teamId}/slack-channels/{channelId}   team admin; {slackTeamId} → SlackChannelLink; not an install of the caller's org → 404
// DELETE /v1/teams/{teamId}/slack-channels/{channelId} team admin
// GET  /v1/integrations/slack/link?n=<nonce>           Entra; the confirm page (HTML, no Slack text); another org's nonce → 410
// POST /v1/integrations/slack/link                     Entra; {nonce} → SlackPersonLink for the signed-in user; another org's nonce → 410
// DELETE /v1/me/slack-links/{slackTeamId}              Entra; the person removes their own link
```

```sql
-- migrate/sql/NNNN_work_wl_slack.sql. Every table is org-owned: each row's org_id is its install's.
CREATE TABLE slack_installs (             -- the one binding of a Slack workspace to an org
  org_id             text NOT NULL REFERENCES orgs (id),
  slack_team_id      text PRIMARY KEY,    -- global: one workspace, one org (another org's PUT → 409)
  signing_secret_ref text NOT NULL, bot_token_ref text NOT NULL,
  installed_by       text NOT NULL, installed_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, slack_team_id),       -- the target of every key below (task 06 decision 2)
  FOREIGN KEY (org_id, installed_by) REFERENCES users (org_id, id)
);
CREATE TABLE slack_channel_links (        -- org_id is read from the install in the write's transaction
  org_id        text NOT NULL, slack_team_id text NOT NULL,
  channel_id    text NOT NULL, team_id       text NOT NULL,
  linked_by     text NOT NULL, linked_at     timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (slack_team_id, channel_id),
  FOREIGN KEY (org_id, slack_team_id) REFERENCES slack_installs (org_id, slack_team_id) ON DELETE CASCADE,
  FOREIGN KEY (org_id, team_id)       REFERENCES teams (org_id, id),
  FOREIGN KEY (org_id, linked_by)     REFERENCES users (org_id, id)
);
CREATE TABLE slack_person_links (         -- org_id is the redeemed nonce's, so its install's
  org_id        text NOT NULL, user_id       text NOT NULL,
  slack_team_id text NOT NULL, slack_user_id text NOT NULL,
  linked_at     timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (slack_team_id, slack_user_id),
  UNIQUE (user_id, slack_team_id),
  FOREIGN KEY (org_id, slack_team_id) REFERENCES slack_installs (org_id, slack_team_id) ON DELETE CASCADE,
  FOREIGN KEY (org_id, user_id)       REFERENCES users (org_id, id)
);
CREATE TABLE slack_link_nonces (          -- one-time, 10 minutes; only the hash is kept
  org_id        text NOT NULL, nonce_hash    bytea PRIMARY KEY,   -- org_id: the minting install's
  slack_team_id text NOT NULL, slack_user_id text NOT NULL, expires_at timestamptz NOT NULL,
  FOREIGN KEY (org_id, slack_team_id) REFERENCES slack_installs (org_id, slack_team_id) ON DELETE CASCADE
);
CREATE TABLE slack_events_seen (          -- Slack retries: one row per event_id; pruned after 24 h
  org_id   text NOT NULL REFERENCES orgs (id),  -- the install's: written after the org is set
  event_id text PRIMARY KEY, seen_at timestamptz NOT NULL DEFAULT now()
);
-- Row-level security (task 06's policy text), the grants, slack_install_org and the feed-binding
-- rule are this file's second half: *The org line*, at the end of this part.
```

**How each inbound call runs** (every one: buffer the raw body, find the install's org by `team_id`
with `slack_install_org` and run the rest in that org's transaction (*The org line*), read its
signing secret by ref, `Verify`, then decode; an unknown workspace is 401, not 404, so no install is confirmed):

1. **`url_verification`** answers the `challenge` string as `text/plain` and writes nothing.
2. **An event** inserts `event_id` into `slack_events_seen` first; a conflict answers 200 and does
   nothing else (Slack's retry). A message from any bot (`bot_id` set) is ignored.
3. **The shortcut** *Send to PanaceaApp* on a message: the channel must have a
   `slack_channel_links` row, else an ephemeral reply through the payload's `response_url`, *This
   channel is not linked to a PanaceaApp team.*, and nothing is written. Linked: task 146's create
   runs with `channel: "slack"`, `quote` = the message `text` **byte for byte** (Slack's own
   escaping of `<`, `>` and `&` kept as Slack sent it; never re-rendered), `channelRef` =
   `<slackTeamId>/<channelId>/<ts>`, requester = the linked person (`kind: "person"`) or `{kind:
   "slack", displayName}`, routed by task 147's `Route` inside the same transaction. The ephemeral
   reply names the request's team and a link; no quote is echoed.
4. **A thread reply** (`thread_ts` equal to a request's `channelRef` ts) appends a
   `RequestMessage` (task 143) with the author's Slack display name and, when linked, `userId`.
   After the request is accepted, a **linked** author's reply also becomes an `ic-` comment,
   `text·H`, by that person; an unlinked author's stays a request message only, because a Hub
   comment needs a Hub author.
5. **`subscribe`** requires a person link; unlinked, an ephemeral *Link your Slack account first*
   with a button to the link page carrying a fresh nonce, whose row is the install's org's, and nothing else is written.
6. **`link_shared`** for task 80's issue URLs calls `chat.unfurl` with the alias, the state word
   and the link, nothing else, for issues the **linked** sharer can read; for an unlinked sharer,
   or an unreadable issue, no unfurl at all (an unfurl could confirm an issue exists).

**How each outbound post runs** (all through `SlackPoster`, one `chat.postMessage` or
`chat.unfurl` call to `https://slack.com/api/…` with the bot token read by ref, 10 s timeout; the
Hub's egress to slack.com through phase 8's allowlist is **UNVERIFIED**):

- **Feeds.** Task 157's feed step, re-pointed at `chat.Post`, walks every binding; a `slack`
  binding goes to `SlackPoster.PostFeed` under the **same** one-post-per-channel-per-15-minutes
  bound task 157 applies. One Block Kit message: one line per `FeedItem` (*EP-142 · landed ·
  Alice · open*), at most 20 lines, then *and N more* with N measured from the batch.
- **Webhook sink `slack`.** Task 156's `deliver.go` gains a sink kind for a webhook whose URL is a
  Slack incoming webhook (`https://hooks.slack.com/…`, checked by prefix at create time). Each
  SW.22 delivery is formatted as one Slack message from the same `FeedItem` builder, so it carries
  the same metadata and nothing more. It keeps task 156's signature header, retries and delivery
  log; it is the raw change feed a team asked for, so the 15-minute feed bound does not apply to
  it (the person's digest bound is unaffected, since a webhook is not a notification to a person).
- **The digest.** Task 165's digest step, re-pointed at `chat.Post`, runs per person: for a person
  whose `DigestSettings.slack` is due and who has a `slack_person_links` row, it calls
  `ClaimDigest` and, only on `true`, `SlackPoster.PostDigest`: a DM (`conversations.open` then
  `chat.postMessage`) with each Desk section's **measured count** and one Desk link. A person set
  to both Teams and Slack gets whichever claims the window first, and never both in 15 minutes.
- **An *AU data only* project** (the data class task 157 reads) posts exactly as any other,
  because no Slack post carries a title in the first place.

#### Tests first — each with the mutant it kills

Integration rows run under `-tags integration` on the Hub's test database; the pure rows run
without it. Golden payloads in `testdata/` are marked **UNVERIFIED** in a header comment.

| Test (file) | What it asserts | Mutant it kills |
|---|---|---|
| `TestVerifyRejectsWrongSignature` (`verify_test.go`) | one flipped hex digit → `ErrBadSignature`; the route answers 401 and `requests` has no new row | an unchecked signature |
| `TestVerifyRejectsSixMinutesOld` (`verify_test.go`) | a valid signature over a timestamp 6 minutes old → `ErrStale`, 401; 6 minutes in the future → `ErrStale` | a replay window, or a one-sided check |
| `TestVerifyConstantTime` (`verify_test.go`) | the comparison is `hmac.Equal` (a source check over `verify.go` for `==` on the signature bytes fails the test) | a timing-leaky compare |
| `TestShortcutMakesOneRequest` (`work_slack_integration_test.go`) | a shortcut on a message in a linked channel → one `rq-`, `channel` `slack`, `quote.text` equals the golden message `text` byte for byte, in the team a task 147 rule routes it to, `state` `open`, and `request.created` carries the rule id | rewritten text; a request that skips intake rules |
| `TestUnlinkedChannelWritesNothing` | the same shortcut in an unlinked channel → the ephemeral sentence and no row | intake guessing a team |
| `TestRetriedEventNoDouble` | a thread reply delivered twice with one `event_id` → one `RequestMessage` | a double sync |
| `TestThreadReplyAttributed` | a reply by a linked Slack user after accept → one `ic-` comment by that person, `text·H`; by an unlinked user → a request message only, with the Slack display name | an unsynced thread; a comment with no Hub author |
| `TestNoCustomerTextLeaves` | an issue titled `ZZ-TITLE-CANARY` with a comment `ZZ-BODY-CANARY`: every recorded outbound body (feed, sink, unfurl, digest; a fake Slack server records them) contains neither canary | a text leak |
| `TestUnlinkedSubscribeRefused` | *Subscribe* from an unlinked Slack user → the ephemeral *Link your Slack account first* and no subscription row | "acts as the installer" |
| `TestDispatchHasNoStateVerb` (`interact_test.go`) | `Dispatch`'s keys are exactly `send_to_panacea`, `open`, `subscribe` | a fact written from chat |
| `TestOneDigestAcrossProviders` | a person due on Teams and Slack in one window → exactly one `PostDigest` across the two fakes; the next after 15 minutes | a per-provider budget |
| `TestDigestCarriesCountsOnly` | the Slack digest body holds each Desk section's name and count and one link, no alias list and no title | a digest that lists items |
| `TestUnfurlOnlyForReadable` | a link to an issue the linked sharer cannot read → no `chat.unfurl` call; an unlinked sharer → none | an unfurl confirming a hidden issue |
| `TestUnknownWorkspace401` | a well-formed call with an unregistered `team_id` → 401, not 404 | a route that enumerates installs |
| `TestLinkNonceOnce` | a nonce used twice → the second is 410 and writes nothing; one 11 minutes old → 410 | a replayable link |
| matrix rows (`matrix_integration_test.go`) | a team `member` PUT of a channel link → 403; a non-admin install PUT → 403; `DELETE /v1/me/slack-links/…` removes only the caller's own | a missing matrix row |
| re-runs | task 157's and task 165's own tests pass unchanged after the re-pointing | a refactor that changes Teams |

#### Steps

- [ ] **Step 1: Preflight.** Tasks 143, 146, 147, 148, 156, 157 and 165 merged on the sync base,
  or wait. Re-read each one's real names (the request create function, `Route`, the binding
  record, the feed step, the digest clock, `DigestSettings`); where they differ from this part,
  theirs win and this part's code follows. Re-check the migration number.
- [ ] **Step 2: `verify.go`, tests first.** `go test ./control-plane/internal/hub/chat/slack/ -run
  Verify -count=1`. Expected: FAIL, `undefined: slack.Verify`. Implement; re-run. Expected: `ok`.
- [ ] **Step 3: `chat.Poster`, then the re-pointing.** Write `poster.go` and its test (a fake
  poster, `ClaimDigest` on the test database). Re-point task 157's feed step and task 165's digest
  step at `chat.Post`. Run their suites unchanged. Expected: `ok`, the same row count as before.
- [ ] **Step 4: The store and the migration, then the inbound routes, tests first.** Write the
  integration rows for the shortcut, the thread, dedup, unlinked and unknown workspace; expected
  FAIL (404 on the route). Implement `events.go`, `interact.go`, `server/work_slack.go` and `store/work_slack.go`
  and the migration. Re-run with `-tags integration`. Expected: `ok`.
- [ ] **Step 5: Outbound, tests first.** The canary, the digest, the unfurl and the sink rows
  against a fake Slack server (`httptest.Server`, recording bodies). Implement `feeds.go`,
  `unfurl.go`, `digest.go` and the `deliver.go` sink. Re-run. Expected: `ok`.
- [ ] **Step 6: The person link page and the matrix rows.** Implement, then the whole acceptance.
- [ ] **Step 7: Commit** (`git add` the new files, then `git commit -F <msgfile> -- <paths>`).

#### Acceptance

```bash
go test ./control-plane/internal/hub/chat/... -race -count=1
# expected: "ok" for chat and chat/slack
go test -tags integration ./control-plane/internal/hub/server/... ./control-plane/internal/hub/webhooks/... -run 'Slack|Matrix|Deliver' -count=1
# expected: "ok" per package, including TestNoCustomerTextLeaves and TestOneDigestAcrossProviders
go test -tags integration ./control-plane/internal/hub/teams/... -count=1
# expected: "ok": task 157's and 165's rows, unchanged, after the re-pointing
```

#### What this task does not claim

- **Not that Slack can reach the Hub** through phase 8's edge allowlist (L-242), nor that the Hub
  can reach slack.com; both **UNVERIFIED**.
- **Not that KPMG uses Slack, or will approve a Slack app.** 05 §1 says *"KPMG runs on
  Microsoft"*; this is for a customer who does. No app is published.
- **Not Slack's OAuth install.** An operator registers the workspace and places two secrets in
  Key Vault, which is itself a proposed Hub-contract addition (task 144's).
- **Not that the payload shapes are Slack's today.** Every Slack field name here and in the
  golden files is from Slack's public documentation, **UNVERIFIED here**.
- **Not actions beyond *Open* and *Subscribe*,** and not a Slack reply to a person's own
  Orchestrator (task 165 does that for Teams; Slack is outside the tenancy, and a turn reply is
  agent text, `text·A`, which would leave).
- **Not post-back of comments into the Slack thread.** Teams has it (task 157); Slack does not,
  by the data rule.
- Design: 05 §1 rows L-206, L-80, L-97, L-137 (*"Teams first, Slack next"*), §3.5 (requests),
  §3.6 (no integration writes a fact); the Desk digest as §4.2 and L-116 place it;
  `docs/AGENTIC-UX.md:243` (one notification per person per 15 minutes). Parity: closes
  **L-206**, and the *Slack next* clause of **L-80**, **L-97** and **L-137**, whose W5 verdicts
  Teams already meets (tasks 157, 165).

**Interfaces:**
- Produces:
  - `chat.Poster`, `chat.FeedItem`, `chat.DigestCounts`, `chat.DigestWindow`, `chat.ClaimDigest`,
    `chat.Post`
  - `slack.Verify`, `slack.MaxSkew`, `slack.Dispatch`, `slack.SlackPoster`
  - SW.21 additions: `ChannelSlack`, `RequesterKindSlack`, `SlackInstall`, `SlackChannelLink`,
    `SlackPersonLink`, `DigestSettings.slack`
  - the routes above; the five tables; webhook sink kind `slack`
- Consumes:
  - task 146 — the request create function and `Request`; task 147 — `Route`; task 143 —
    `RequestMessage`, `DigestSettings`; task 148 — the Entra page pattern of `work_forms.go`
  - task 156 — SW.22's payload and `deliver.go`; task 157 — the binding record and the feed step;
    task 165 — the digest step and its per-person clock; task 80 — the https issue page; task 12 —
    subscriptions; task 144 — the `secret_ref` pattern
- Amends: SW.21 gains two enum values and three types (additive); task 157's and 165's steps call
  `chat.Post` with their behaviour unchanged.
