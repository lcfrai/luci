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
  a `slack_person_links` row that person made on a Hub page behind Entra, in the install's org.
  Unlinked, a Slack user can send a message to intake (the requester is then `kind: "slack"` with a
  display name) and can do nothing else.
- **No fact from chat.** Slack's actions are *Open* and *Subscribe* only. No state, assignee or
  priority action exists, and no act from Slack writes a fact state or `done` (SW.3).

**Install is registration, not an OAuth flow.** An org admin records the workspace id and types no
secret reference: the Hub names the app's signing secret and its bot token from the install's own key, its org and workspace (`slack.SecretNames`), and an operator places both in Key
Vault under those names, for that org alone: the `secret_ref` pattern task 144 proposes for webhook secrets
(`tasks-141-160.md:241-245`; Key Vault access is itself a **proposed Hub-contract addition**,
since `hub-contract.md:2083-2085` says the Hub has none in v1). Slack's OAuth install is not built. The install is the workspace's one org (*The org line*), and the PUT that makes it reads both secrets before it writes (*The install's secrets*, at the end).

**Files** (the File map has no row for chat; these follow its `hub/` layout and are proposals):
- `control-plane/internal/hub/chat/poster.go` (+ test) — `Poster`, `Post`, the provider-neutral
  feed and digest message shapes (new)
- `control-plane/internal/hub/chat/slack/verify.go` (+ test) — `Verify`; `SecretNames` and `Secrets`, an install's two Key Vault names and the read of them (new)
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
  nonces, seen events, and `PruneSlack` (new); `server/janitor.go` ⚑ (the Hub's janitor, where task 157 adds step 6, `task-157.md:60`) — the step that calls it per org (modified)
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

// SlackInstall is one workspace registered to an org. The two refs are read-only, derived by the Hub
// (slack.SecretNames), never typed; no secret is ever in a column, a response or a log.
type SlackInstall struct {
	SlackTeamID     string `json:"slackTeamId"` // Slack's workspace id, "T…"
	SigningSecretRef string `json:"signingSecretRef"` // read-only: slack.SecretNames(org, SlackTeamID), no column
	BotTokenRef     string `json:"botTokenRef"`      // read-only, the same: the two Key Vault names to place
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
// PUT  /v1/orgs/{orgId}/slack-installs/{slackTeamId}   org admin of {orgId}; no body; both secrets read first, else 422 (*The install's secrets*)
// DELETE /v1/orgs/{orgId}/slack-installs/{slackTeamId} org admin of {orgId}; also stops every Slack post
// PUT  /v1/teams/{teamId}/slack-channels/{channelId}   team admin; {slackTeamId} → SlackChannelLink; not an install of the caller's org → 404; linked to another team → 409
// DELETE /v1/teams/{teamId}/slack-channels/{channelId} team admin
// GET  /v1/integrations/slack/link?n=<nonce>           Entra; the confirm page (HTML, no Slack text); another org's nonce → 410
// POST /v1/integrations/slack/link                     Entra; {nonce} → SlackPersonLink for the signed-in user; another org's nonce → 410
// DELETE /v1/me/slack-links/{slackTeamId}              Entra; the person removes their own link
```

```sql
-- migrate/sql/NNNN_work_wl_slack.sql. Every table is org-owned: each row's org_id is its install's.
CREATE TABLE slack_installs (             -- the one binding of a Slack workspace to an org
  org_id             text NOT NULL REFERENCES orgs (id),
  slack_team_id      text PRIMARY KEY,    -- global: one workspace, one org; another org's claim → 422, or 409 once secrets are placed for it
  CONSTRAINT slack_installs_team_id_form CHECK (slack_team_id ~ '^T[0-9A-Z]{1,31}$'), -- no ref column: slack.SecretNames(org_id, slack_team_id)
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

**How each inbound call runs** (every one but step 1's: buffer the raw body, find the install's org by `team_id`
with `slack_install_org` and run the rest in that org's transaction (*The org line*), read its
signing secret under the install's `slack.SecretNames`, `Verify`, then decode; an unknown workspace is 401, not 404, so no install is confirmed):

1. **`url_verification`** answers the `challenge` string as `text/plain` and writes nothing. Its body names no workspace (`token`, `challenge`, `type`: Slack's documentation, **UNVERIFIED**), so it is read first, before the org step, and is **not verified**, since no install's secret can be chosen for it: it reads no row, returns only the caller's own string, and a `challenge` outside `^[A-Za-z0-9_-]{1,128}$` is 400, so nothing but a token is echoed.
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
without it. Golden payloads in `testdata/` are marked **UNVERIFIED** in a header comment. Fix round 2's rows are at the end, beside their rules (*The install's secrets*; *One team per channel, and pruning*).

| Test (file) | What it asserts | Mutant it kills |
|---|---|---|
| `TestVerifyRejectsWrongSignature` (`verify_test.go`) | one flipped hex digit → `ErrBadSignature`; the route answers 401 and `requests` has no new row | an unchecked signature |
| `TestVerifyRejectsSixMinutesOld` (`verify_test.go`) | a valid signature over a timestamp 6 minutes old → `ErrStale`, 401; 6 minutes in the future → `ErrStale` | a replay window, or a one-sided check |
| `TestVerifyConstantTime` (`verify_test.go`) | the comparison is `hmac.Equal` (a source check over `verify.go` for `==` on the signature bytes fails the test) | a timing-leaky compare |
| `TestShortcutMakesOneRequest` (`work_slack_integration_test.go`) | a shortcut on a message in a linked channel → one `rq-`, `channel` `slack`, `quote.text` equals the golden message `text` byte for byte, in the team a task 147 rule routes it to, `state` `open`, and `request.created` carries the rule id | rewritten text; a request that skips intake rules |
| `TestUnlinkedChannelWritesNothing` | the same shortcut in an unlinked channel → the ephemeral sentence and no row | intake guessing a team |
| `TestRetriedEventNoDouble` | a thread reply delivered twice with one `event_id` → one `RequestMessage` | a double sync |
| `TestThreadReplyAttributed` | a reply by a linked Slack user after accept → one `ic-` comment by that person, `text·H`; by an unlinked user → a request message only, with the Slack display name | an unsynced thread; a comment with no Hub author |
| `TestNoCustomerTextLeaves` | an issue titled `ZZ-TITLE-CANARY` with a comment `ZZ-BODY-CANARY`: every recorded outbound body (feed, sink, unfurl, digest; a fake Slack server records them) contains neither canary; the feed's Slack binding is inserted as the owner, since no route writes one (*Feed bindings*) | a text leak |
| `TestUnlinkedSubscribeRefused` | *Subscribe* from an unlinked Slack user → the ephemeral *Link your Slack account first* and no subscription row | "acts as the installer" |
| `TestDispatchHasNoStateVerb` (`interact_test.go`) | `Dispatch`'s keys are exactly `send_to_panacea`, `open`, `subscribe` | a fact written from chat |
| `TestOneDigestAcrossProviders` | a person due on Teams and Slack in one window → exactly one `PostDigest` across the two fakes; the next after 15 minutes | a per-provider budget |
| `TestDigestCarriesCountsOnly` | the Slack digest body holds each Desk section's name and count and one link, no alias list and no title | a digest that lists items |
| `TestUnfurlOnlyForReadable` | a link to an issue the linked sharer cannot read → no `chat.unfurl` call; an unlinked sharer → none | an unfurl confirming a hidden issue |
| `TestUnknownWorkspace401` | a well-formed call with an unregistered `team_id` → 401, not 404 | a route that enumerates installs |
| `TestLinkNonceOnce` | a nonce used twice → the second is 410 and writes nothing; one 11 minutes old → 410 | a replayable link |
| `TestChannelLinkCrossOrgRefused` | orgs A and B enrolled, install `T_B` in B. A's team admin PUTs `/v1/teams/{A's team}/slack-channels/C1` with `{slackTeamId: "T_B"}` → 404 `slack_install_not_found`, and `slack_channel_links` holds no row, counted as the owner (`hubtest.OwnerConn`, which row-level security does not filter); the same with the Go check off (H16b's test-only hook) → 404, no row; then `T_B`'s shortcut in `C1` → the *not linked* ephemeral and no `rq-` in either org; and A's org admin PUTting an install of `T_B` → 422 `slack_secrets_missing`, since nothing is placed under A's names for it, B's row unchanged (as the owner; the 409 is `TestInstallReadsOnlyItsOrgsSecrets`', *The install's secrets*). Control: the same admin linking A's own install `T_A` succeeds with one row, so the test is red while the route is missing | the link's org taken from the team, the path or the body instead of the install; an install read outside row-level security; a workspace re-pointed to a second org |
| `TestPersonLinkCrossOrgRefused` | an unlinked `T_B` user presses *Subscribe*, minting a nonce in org B; an org A user GETs its link page and POSTs it → 410 each, the body a used nonce gets, and no `slack_person_links` row (counted as the owner); the same with the Go check off. Control: the nonce is still there, and an org B user redeeming it gets the link, in org B | a redeem that checks the nonce and not its org; a person link that takes the signed-in user's org instead of the nonce's |
| `TestSlackTablesOrgIsolated` | for each of the five tables, a row inserted in org B is invisible to `hub_app` with `hub.org_id` = A (task 116's `TestW4EveryTableHasRLS` form, `task-116.md:368`), and an insert whose `org_id` is not `hub.org_id` is `42501`; a channel link `(A, T_B, …)` inserted as the owner is `23503`; `slack_install_org('T_B')` answers B's id, an unknown id `NULL` | row-level security left as a comment; a single-column key to `slack_installs`, which H07a's `TestEveryForeignKeyCarriesTheOrg` fails too |
| matrix rows (`matrix_integration_test.go`) | every Entra route above as every §5.3 column (`hub-contract.md:335-363`): a channel-link PUT or DELETE → `V` 403, `M` 403, `A` ✓, `O`, `N` and `X` 404; an install PUT or DELETE → `O` ✓ (the fixture's two secrets placed, its fake `auth.test` answering the workspace), `V`, `M`, `A` and `N` 403, `X` 404; the link GET and POST → ✓ in the nonce's org, `X` 410, the POST's `body` minting a fresh nonce for a fresh Slack user per cell (`task-07.md:382`), since a nonce redeems once; `DELETE /v1/me/slack-links/…` removes only the caller's own; the `X` column once more with the Go check off, where an install PUT can read only `X`'s own names, so it answers no ✓ and writes no row of the fixture's org | a missing matrix row; an org line held by Go alone |
| re-runs | task 157's and task 165's own tests pass unchanged after the re-pointing | a refactor that changes Teams |

#### Steps

- [ ] **Step 1: Preflight.** Tasks 143, 146, 147, 148, 156, 157 and 165 merged on the sync base,
  or wait. Re-read each one's real names (the request create function, `Route`, the binding
  record, the feed step, the digest clock, `DigestSettings`); where they differ from this part,
  theirs win and this part's code follows. Re-check the migration number.
- [ ] **Step 2: `verify.go`, tests first.** `go test ./control-plane/internal/hub/chat/slack/ -run
  'Verify|SecretNames' -count=1`. Expected: FAIL, `undefined: slack.Verify`. Implement; re-run. Expected: `ok`.
- [ ] **Step 3: `chat.Poster`, then the re-pointing.** Write `poster.go` and its test (a fake
  poster, `ClaimDigest` on the test database). Re-point task 157's feed step and task 165's digest
  step at `chat.Post`. Run their suites unchanged. Expected: `ok`, the same row count as before.
- [ ] **Step 4: The store and the migration, then the inbound routes, tests first.** Write the
  integration rows for the shortcut, the thread, dedup, unlinked and unknown workspace, and
  `TestChannelLinkCrossOrgRefused`, `TestSlackTablesOrgIsolated` and round 2's rows at the end; expected FAIL (404 on the
  route, and no such table). Implement `events.go`, `interact.go`, `server/work_slack.go` and
  `store/work_slack.go` and the migration, both halves (*The org line*), and the janitor step. Re-run with
  `-tags integration`. Expected: `ok`.
- [ ] **Step 5: Outbound, tests first.** The canary, the digest, the unfurl and the sink rows
  against a fake Slack server (`httptest.Server`, recording bodies). Implement `feeds.go`,
  `unfurl.go`, `digest.go` and the `deliver.go` sink. Re-run. Expected: `ok`.
- [ ] **Step 6: The person link page and the matrix rows, tests first.** Write
  `TestPersonLinkCrossOrgRefused` and the matrix rows; expected FAIL (404: the link routes do not
  exist, so each control fails). Implement; re-run. Expected: `ok`. Then the whole acceptance.
- [ ] **Step 7: Commit** (`git add` the new files, then `git commit -F <msgfile> -- <paths>`).

#### Acceptance

```bash
go test ./control-plane/internal/hub/chat/... -race -count=1
# expected: "ok" for chat and chat/slack
go test -tags integration ./control-plane/internal/hub/server/... ./control-plane/internal/hub/webhooks/... -count=1 \
  -run 'Slack|Matrix|Deliver|Shortcut|Unlinked|Retried|ThreadReply|NoCustomerText|OneDigest|DigestCarries|Unfurl|UnknownWorkspace|LinkNonce|CrossOrg|InstallReads'
# expected: "ok" per package; with -v, every server row of the table above and of round 2's reports --- PASS,
# among them TestNoCustomerTextLeaves, TestOneDigestAcrossProviders, the three org rows and TestInstallReadsOnlyItsOrgsSecrets
go test -tags integration ./control-plane/internal/hub/migrate/... -run 'EveryForeignKey|CrossOrg|GrantsAreExactly' -count=1
# expected: "ok": H07a's catalog tests read the Slack tables' keys (task-06.md:475-479); their grants are TestSlackGrantsAreExactlyTheList's, in the server run above;
# one red only because a Work table exists is task 06's Step 4 case (task-06.md:492-496)
go test -tags integration ./control-plane/internal/hub/teams/... -count=1
# expected: "ok": task 157's and 165's rows, unchanged, after the re-pointing
```

#### What this task does not claim

- **Not that Slack can reach the Hub** through phase 8's edge allowlist (L-242), nor that the Hub
  can reach slack.com; both **UNVERIFIED**.
- **Not that KPMG uses Slack, or will approve a Slack app.** 05 §1 says *"KPMG runs on
  Microsoft"*; this is for a customer who does. No app is published.
- **Not Slack's OAuth install.** An org admin registers the workspace; an operator places its two secrets in
  Key Vault under the names the Hub derives for that org (*The install's secrets*), itself a proposed Hub-contract addition (task 144's).
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
  **L-206**, and the *Slack next* clause of **L-97** and **L-137**, whose W5 verdicts Teams already meets (task 157);
  **L-80**'s *Slack next* clause (Teams: tasks 157, 165) is proven safe by `TestNoCustomerTextLeaves` and reachable by no user until a route writes a Slack feed binding (*Feed bindings*: a gap, returned).

**Interfaces:**
- Produces:
  - `chat.Poster`, `chat.FeedItem`, `chat.DigestCounts`, `chat.DigestWindow`, `chat.ClaimDigest`,
    `chat.Post`
  - `slack.Verify`, `slack.MaxSkew`, `slack.Dispatch`, `slack.SlackPoster`, `slack.SecretNames`, `slack.Secrets`, `store.PruneSlack`
  - SW.21 additions: `ChannelSlack`, `RequesterKindSlack`, `SlackInstall`, `SlackChannelLink`,
    `SlackPersonLink`, `DigestSettings.slack`
  - the routes above; the five tables, each org-owned, and `hub.slack_install_org`; the store's
    org reads (*The org line*); webhook sink kind `slack`
- Consumes:
  - task 06 — the `org_isolation` policy text and decision 2's org-carrying keys; H07a — the
    `UNIQUE (org_id, id)` of `teams` and `users`, and its catalog tests; H16b — the matrix, its `X`
    column and its test-only hook
  - task 146 — the request create function and `Request`; task 147 — `Route`; task 143 —
    `RequestMessage`, `DigestSettings`; task 148 — the Entra page pattern of `work_forms.go`
  - task 156 — SW.22's payload and `deliver.go`; task 157 — the binding record (task 145's
    `work_teams_bindings`), the feed step and `hub.org_for_tenant`, the model for the org read;
    task 165 — the digest step and its per-person clock; task 80 — the https issue page; task 12 —
    subscriptions; task 144 — the `secret_ref` pattern, here a name the Hub derives and no one types; the Hub's janitor (§12.5)
- Amends: SW.21 gains two enum values and three types (additive); task 157's and 165's steps call
  `chat.Post` with their behaviour unchanged.

#### The org line: every Slack row is its install's org's (fix batch b13, S19-C1)

The install row is the only thing that binds a Slack workspace to an org. So every other Slack row
takes its `org_id` **from the install, read inside the transaction that writes the row**, never
from the path, the body or the team, and the line holds in three layers where the Hub contract
asks for two (*"row-level security **and** Go"*, `hub-contract.md:2082`):

1. **Go refuses first.** Each write reads the install's org in its own transaction and answers
   before it writes anything (the table below).
2. **Row-level security hides the rest.** Task 06's policy text is on all five tables, so another
   org's install, link or nonce is not there to be read, even with the Go check off.
3. **The keys make a crossed row unrepresentable.** Every foreign key carries `org_id` at its
   target's position (task 06 decision 2, `task-06.md:19-24`), so a link whose org is not its
   install's, its team's or its person's is a `23503`, whatever the handler does.

| Route or step | Its org | Across orgs |
|---|---|---|
| the two Slack routes | `slack_install_org(team_id)`, then `set_config('hub.org_id', org, true)` as the transaction's first statement; the dedup row, the nonce, the request and its thread are that org's | no install: 401, before any transaction |
| `PUT`, `DELETE …/slack-installs/{slackTeamId}` | the token's, which `{orgId}` must equal; the PUT reads both secrets under that org's `slack.SecretNames` before it reads or writes `slack_installs` (*The install's secrets*) | another org's `{orgId}` → 404; a secret not placed under the caller's org's names → 422 `slack_secrets_missing`, nothing written, so no org claims a workspace with another org's secrets; installed in another org although secrets were placed for both (an operator's error) → 409 `slack_workspace_taken`, since the key is global |
| `PUT …/slack-channels/{channelId}` | `InstallOrg`, in the caller's transaction | no install of the caller's org by that id → 404 `slack_install_not_found`, nothing written |
| `GET`, `POST /v1/integrations/slack/link` | the nonce's, which its install gave it; it must equal the token's | 410 with a used nonce's body, nothing written, the nonce kept for its own org |
| the feed step, the digest | the binding's or the person's, per org as the janitor runs (`hub-contract.md:2232-2233`) | a Slack binding whose channel link is not its team's posts nothing |

**Feed bindings.** Task 145's `work_teams_bindings` (`task-145.md:235-251`, the binding record task
157 reads) has no provider column, so this migration adds one. A Slack binding's channel ref is
`<slackTeamId>/<channelId>`, the prefix of a request's `channelRef`. The feed step resolves it
through that channel's `slack_channel_links` row, under the binding's org and team, and
`SlackPoster` reads the bot token under that row's install's `slack.SecretNames`: no row, no post. Another org's link
and install are not there to be read, so no binding posts into another org's workspace, and
unlinking a channel stops its posts. No route in this part writes a Slack binding (the
`SlackChannelLink` comment says a link *carries* them, and names no route): a gap for the return, and
whichever route it is refuses 404 unless the channel link is the binding's team's. Until one exists, L-80's *Slack next* clause has a proof and no user path (`task-187.md:90` says so).

**Inside one org** a team admin can still link any channel of the org's workspace by its id. Task
157 links a Teams channel by a pairing code shown in that channel instead (decision 1,
`task-157.md:27-32`). That is a same-org question, not S19-C1's, and is returned as a gap.

```sql
-- migrate/sql/NNNN_work_wl_slack.sql, second half. Row-level security: task 06's text over exactly
-- these tables (task-06.md:367-380). No FORCE, as task 06 and hub-contract.md:1914-1918: the owner
-- runs slack_install_org before hub.org_id is set, and under FORCE a policy TO hub_app would hide
-- every install from it (task-145.md:434-438 makes the same call for app_for_client).
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['slack_installs', 'slack_channel_links', 'slack_person_links',
      'slack_link_nonces', 'slack_events_seen'] LOOP
    EXECUTE format('ALTER TABLE hub.%I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('CREATE POLICY org_isolation ON hub.%I TO hub_app
                    USING (org_id = current_setting(''hub.org_id'', true))
                    WITH CHECK (org_id = current_setting(''hub.org_id'', true))', t);
  END LOOP;
END $$;

-- Grants: task 06's shape (task-06.md:382-387), each explicit; TestSlackGrantsAreExactlyTheList pins them. An install
-- or a link is removed and made again, never edited; a nonce is deleted when used; the janitor prunes the rest (at the end).
GRANT SELECT, INSERT, DELETE ON slack_installs, slack_channel_links, slack_person_links,
                                slack_link_nonces, slack_events_seen TO hub_app;
-- No UPDATE on any Slack table: an install holds no secret ref; a secret is rotated in Key Vault under its own name.

-- slack_install_org answers one workspace's org and nothing else, for the two Slack routes, which
-- call it before they know the org (task 157's hub.org_for_tenant is the model, task-157.md:273-277).
-- It is one more SECURITY DEFINER function for the contract's T3 residual (hub-contract.md:2071).
CREATE FUNCTION slack_install_org(p_slack_team_id text) RETURNS text
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = hub, pg_temp AS $$
  SELECT i.org_id FROM slack_installs i WHERE i.slack_team_id = p_slack_team_id
$$;
REVOKE ALL ON FUNCTION slack_install_org(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION slack_install_org(text) TO hub_app;

-- The feed bindings' provider (above); task 06 decision 4's named CHECK.
ALTER TABLE work_teams_bindings ADD COLUMN provider text NOT NULL DEFAULT 'teams'
  CONSTRAINT work_teams_bindings_provider_known CHECK (provider IN ('teams', 'slack'));
```

```go
// store/work_slack.go — the org reads. An org is an argument only where the Hub already holds it
// from an install, a nonce or the token; no request body supplies one.

// InboundOrg calls hub.slack_install_org before any org is set; found=false is the 401.
func InboundOrg(ctx context.Context, q Querier, slackTeamID string) (orgID string, found bool, err error)

// InstallOrg reads the install in tx, under row-level security, so another org's is not found
// (404 slack_install_not_found); the handler also compares orgID with the principal's org.
func InstallOrg(ctx context.Context, tx pgx.Tx, slackTeamID string) (orgID string, found bool, err error)

// PutChannelLink inserts with InstallOrg's orgID; the org-carrying keys refuse any other. Linked to this team: created=false; to another: ErrChannelLinked (409).
func PutChannelLink(ctx context.Context, tx pgx.Tx, orgID string, l hubapi.SlackChannelLink) (created bool, err error)

// InsertLinkNonce runs in the inbound transaction, with InboundOrg's orgID.
func InsertLinkNonce(ctx context.Context, tx pgx.Tx, orgID string, hash [32]byte, slackTeamID, slackUserID string, expires time.Time) error

// PeekLinkNonce (the GET page) and RedeemLinkNonce (the POST, which deletes it) answer
// found=false for a used, an expired or another org's nonce; row-level security hides the last,
// so a refused redeem cannot delete it.
func PeekLinkNonce(ctx context.Context, tx pgx.Tx, hash [32]byte, now time.Time) (n LinkNonce, found bool, err error)
func RedeemLinkNonce(ctx context.Context, tx pgx.Tx, hash [32]byte, now time.Time) (n LinkNonce, found bool, err error)

// PutPersonLink inserts with n.OrgID, after the handler has checked it equals the principal's org.
func PutPersonLink(ctx context.Context, tx pgx.Tx, n LinkNonce, userID string) error

type LinkNonce struct{ OrgID, SlackTeamID, SlackUserID string }
```

#### The install's secrets: named by the Hub, read before the row (fix batch b13 round 2, N-I1)

The install is the org line, so a claim of a workspace must rest on something the claimant org
cannot take from another org. A typed ref cannot be that: task 145's decision 10 fixes no form for
a `secret_ref` (`task-145.md:96-100`), so an org admin could name another org's two secrets and
claim a workspace no org had installed yet. So **no one types a ref**. The Hub derives both names
from the install's own key, the operator places the two secrets under those names for that org
alone, and the PUT reads both before it touches `slack_installs`. A claim of another org's
workspace then finds nothing under the claimant's names, writes no row, confirms no install and
can verify no event.

**The PUT, in order** (`PUT /v1/orgs/{orgId}/slack-installs/{slackTeamId}`, no body: `DecodeJSON`
rejects unknown fields, `hub-contract.md:2076`, so a body naming a ref is 400):

1. `{orgId}` is the token's org, else 404; the caller is its org admin, else 403.
2. `slackTeamId` matches `^T[0-9A-Z]{1,31}$` (Slack's form, **UNVERIFIED**), else 400 `field:
   slackTeamId`, before any name is made; `slack_installs_team_id_form` holds the same in the table.
3. `Secrets.Secret` reads the two secrets `SecretNames(orgId, slackTeamId)` names. Either absent → 422
   `slack_secrets_missing`, whose body says which of the two names is missing (both are the
   caller's own org's), and nothing is read from or written to `slack_installs`.
4. `SlackPoster.WorkspaceOf` calls Slack's `auth.test` with the bot token (Slack's documentation,
   **UNVERIFIED**); its `team_id` must equal `slackTeamId`, else 422 `slack_token_wrong_workspace`,
   nothing written; Slack unreachable, or `ok: false`, → 502 `slack_unreachable`, nothing written.
   A signing secret has no such check: the first event that fails `Verify` is 401.
5. `PutInstall` inserts `(org_id, slack_team_id, installed_by)` with `ON CONFLICT (slack_team_id)
   DO NOTHING`, and reads a conflict back under row-level security: visible is this org's own
   install, 200 and unchanged; hidden is another org's, 409 `slack_workspace_taken` (that `DO
   NOTHING` skips a row the policy hides, with no error, is Postgres's documented behaviour, not
   run here; `TestInstallReadsOnlyItsOrgsSecrets` fails if it is not). The 409 is
   reachable only where an operator placed secrets for this org too, an operator's error, so a
   claimant with nothing placed learns nothing of another org's install (N-M2).

The answer is `SlackInstall`, 201 or 200, with its two read-only names. `DELETE` removes the row
and, by cascade, its links, person links and nonces; the secrets stay in Key Vault until the
operator removes them, and no other org's names reach them.

```go
// chat/slack/verify.go — additions.

// SecretNames is the one place an install's two Key Vault names are made, from its own key, so no
// org can name another org's secret. orgID is "org-" and 16 hex (hub-contract.md:1481) and
// slackTeamID matches slack_installs_team_id_form, so each name is letters, digits and '-' (Key
// Vault's rule for a secret name, UNVERIFIED here), and two orgs never share one.
func SecretNames(orgID, slackTeamID string) (signing, bot string) // "slack-<orgID>-<slackTeamID>-signing", "…-bot"

// Secrets reads one Key Vault secret by name under the Hub's workload identity, the proposed
// Hub-contract addition task 145 names (task-145.md:96-100); the Hub's own seam, when it lands, wins.
type Secrets interface {
	Secret(ctx context.Context, name string) ([]byte, error) // ErrSecretNotFound when absent
}

var ErrSecretNotFound = errors.New("slack: secret not found")

// WorkspaceOf (feeds.go, beside SlackPoster) calls auth.test with botToken and answers its team_id.
func (p *SlackPoster) WorkspaceOf(ctx context.Context, botToken []byte) (slackTeamID string, err error)
```

```go
// store/work_slack.go — the install, written only after steps 3 and 4.
func PutInstall(ctx context.Context, tx pgx.Tx, orgID, slackTeamID, installedBy string) (created bool, err error)

var ErrWorkspaceTaken = errors.New("store: slack workspace installed in another org") // 409 slack_workspace_taken
```

| Test (file) | What it asserts | Mutant it kills |
|---|---|---|
| `TestSecretNamesAreTheOrgs` (`verify_test.go`) | `SecretNames(A, T)` and `SecretNames(B, T)` share neither name; each is `slack-<org>-<team>-signing` or `-bot` and matches `^[0-9A-Za-z-]{1,127}$` | a name without the org; a name Key Vault refuses |
| `TestInstallReadsOnlyItsOrgsSecrets` (`work_slack_integration_test.go`) | a fake `Secrets` holds `SecretNames(B, T_B)` only, and a fake `auth.test` answers `T_B` for B's token. A's org admin PUTs `/v1/orgs/A/slack-installs/T_B` → 422 `slack_secrets_missing` naming A's two names, no row (counted as the owner) and no `auth.test` call; B's admin PUTs `T_B` → 201 with B's two names; A's PUT again → 422, not 409; B's again → 200, the row unchanged. Then A's names are placed for `T_B` as well (an operator's error): A's PUT → 409 `slack_workspace_taken`, B's row unchanged. With B's names for `T_C` placed and its token's `auth.test` answering `T_B`, B's PUT of `T_C` → 422 `slack_token_wrong_workspace`, no row. A body `{"signingSecretRef": "x"}` → 400; a `slackTeamId` of `t_b` → 400 `field: slackTeamId`, and `Secrets` is never called. B's 201 is the control: a missing route answers none of these | a ref typed by the claimant; a PUT that reads the install, or writes, before the secrets; a 409 answered before them (the oracle); a token of another workspace |
| `TestSlackGrantsAreExactlyTheList` (`work_slack_integration_test.go`) | `role_table_grants` and `column_privileges` for `hub_app` on the five Slack tables equal the grants above: `SELECT`, `INSERT` and `DELETE` on each, and no `UPDATE`, table-wide or on a column; `slack_install_org` is callable as `hub_app` and not as `PUBLIC` (`task-145.md:784`'s form) | a grant nobody reviewed; an `UPDATE` left from the ref columns; `EXECUTE` left to `PUBLIC` |

In every test of this part, `T_A`, `T_B` and `T_C` stand for three well-formed ids (`TA0000001`
and so on), since `slack_installs_team_id_form` refuses `_`.

**Rulings (the root-cause option each, declared).**
1. **The Hub names the secrets.** Re-review 1 offered this or its equal, a typed ref that must sit
   under the caller's org's own prefix. Naming wins: nothing typed means nothing to parse, one
   field fewer for an admin, and the name is a function of the row's key, so no later route can
   reopen the hole. If the owner wants refs typed, the prefix rule is the fallback.
2. **Decision 10, read for its intent.** An install holds neither a ref nor a version: the Hub
   reads a secret's current version at each use, and the operator rotates one by adding a version
   under the same name. Decision 10's point, never the secret in a column, holds more strictly.
3. **`auth.test` is added beyond the finding's ask** (step 4). A claim then needs a bot token
   Slack itself says is that workspace's, so a mistyped workspace id, the admin's or the
   operator's, cannot squat a workspace another org will need. An operator who places org B's
   real secrets under org A's names is outside what the Hub can see; that is the operator's trust.

#### One team per channel, and pruning (fix batch b13 round 2: N-M4, N-M5, N-M8)

**One team per channel.** A channel link is a team's intake, so a second team's admin cannot take
it over: a PUT for a channel linked to another team is 409 `slack_channel_linked`, naming no team,
the link unchanged, and the linked team's admin deletes it first. A PUT for a channel already
linked to the same team is 200, unchanged. The 409 is reachable only inside the install's org,
since another org's install is 404 before it (*The org line*).

**Pruning.** A seen event is kept 24 h and a nonce 10 minutes, and neither is read after. A janitor
step in `server/janitor.go` ⚑ calls `PruneSlack` once per org in the janitor's pass, with
`hub.org_id` set as for every step (§12.5, `hub-contract.md:2232-2233`), so row-level security
holds for it too: it deletes that org's `slack_events_seen` rows older than 24 h and its
`slack_link_nonces` rows past `expires_at`, and nothing else. `DELETE` is already granted.

```go
// store/work_slack.go — the janitor's, and the link's refusal.
func PruneSlack(ctx context.Context, tx pgx.Tx, now time.Time) (events, nonces int64, err error)

var ErrChannelLinked = errors.New("store: slack channel linked to another team") // 409 slack_channel_linked
```

| Test (file) | What it asserts | Mutant it kills |
|---|---|---|
| `TestSlackChannelLinksOneTeam` (`work_slack_integration_test.go`) | `C1` linked to team X; the admin of team Y, same org, PUTs `C1` for Y → 409 `slack_channel_linked`, the body naming no team, X's link unchanged; X's admin PUTs it again → 200, one row; after X's `DELETE`, Y's PUT → 201 | a silent re-point of a team's intake; a second row for one channel |
| `TestSlackPruneIsPerOrg` | orgs A and B each hold a seen event 25 h old and one 1 h old, an expired nonce and a live one; the step runs for A → A's old event and expired nonce are gone, its fresh two kept, and B's four rows untouched (counted as the owner) | a table that grows without end; a prune that crosses orgs |
| `TestSlackURLVerification` | a `url_verification` body, which has no `team_id`, with no valid signature → 200 `text/plain`, the challenge byte for byte, no row in any Slack table and no `Secrets` read; a challenge `<b>x</b>` → 400 | Slack's Request URL check refused as an unknown workspace (the old order); markup echoed |
