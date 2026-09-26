### Task 157: the Teams app, part 1: intake from a Teams message with its thread synced to the request; a channel per feature; view changes to a channel

W5, stream WH. Depends on task 146 (requests `rq-`, `store.CreateRequest`, the Triage landing) and
task 156 (the janitor's step 5, whose cursor rule step 6 here reuses). Consumes task 147's
`intake.Route` (`task-147.md:211`), task 119's `work.update_posted` (`task-119.md:182-190`), task
80's one Go evaluator of `FilterGroup` (`store/work_filter.go`, its scope; no part at this writing)
and task 131's *shipped* fact (its event kind is fixed by that part; cited here as **UNVERIFIED**
until it is). Types from task 143 (SW.21: `Request`, `RequestMessage`, `TeamsBinding`). Hub files
are **planned, not on `feat/hub`** at `3df9fab8`, and every path in `server/` and `store/` is
**UNVERIFIED until merge**. 05 §1 L-137, L-178, L-80, L-81, L-97 and L-207 (its digest and *ask*
half is task 165).

Teams is where a KPMG requester already is. This task lets anyone in a linked channel turn a
message into a request without leaving Teams, keeps that thread and the request in step, and lets
a team follow a feature or a view in a channel. It adds **no** new way to change work: a request is
still accepted in the desktop's Triage by a person (task 146), and nothing a bot receives moves an
issue's state.

**The one thing this task must not get wrong** is what leaves for Teams. Agent text (`text·A`:
agent comments, notes, proposals, 05 §3.4) is never posted. Posts carry aliases, closed words and
https links; titles and a person's comment text go only when the project is not *AU data only*.
And what arrives from Teams is customer content: the quote is stored as the requester's words and
enters no event and no agent's managed block (task 146's rule).

Six decisions the scope left open, made here.

1. **A Teams team is linked to a PanaceaApp team by a pairing code, never by a pasted id.** In a
   channel, anyone types `@PanaceaApp connect`; the bot answers in the channel with a 12-character
   code, valid 10 minutes, stored as a SHA-256 hash beside the channel's `channelRef`. A team admin
   enters the code in the desktop (a settings row no task owns yet; task 173's admin surface is the
   nearest, a gap), and `PUT …/teams-bindings` resolves it.
   The admin's authority is the Hub's own (an Entra-backed Hub token), never a Teams identity.
2. **Intake works in a channel of a linked Teams team only.** *Send to PanaceaApp* in a chat or an
   unlinked channel answers with a card: *"This channel is not linked to a PanaceaApp team. A team
   admin links it with @PanaceaApp connect."* No request is made.
3. **The quote is the message's words, not its markup.** Teams delivers most messages as HTML.
   `teams.PlainText` is a fixed, tested conversion (tags dropped, `<br>` and block ends to `\n`,
   entities decoded, nothing else); a `text` message is kept byte for byte. The stored quote is
   `PlainText(content)`, and the test asserts that equality byte for byte.
4. **The requester is a person when the Hub knows them.** The message author's `(tid, aadObjectId)`
   is looked up (never provisioned: only a sign-in creates a user, hub-contract §5.1 step 5); found →
   `kind: person, userId`; not found → `kind: teams`, `displayName` from the activity.
5. **The bot writes as the `integration` actor kind** (task 145's guard), with `hub.work_actor_user`
   set to the invoking person when known. It may create a request, append a `RequestMessage`, and
   append a comment on an accepted request's issue attributed to the mapped author; the guard refuses
   it every state column. A reply by an unmapped author stays a `RequestMessage` only.
6. **A channel gets at most one post per 15 minutes** (AX anti-pattern 8, `docs/AGENTIC-UX.md:243`,
   applied per channel). Changes collect; the post is one message listing them.

**Files:**

- `control-plane/internal/hub/teams/bot.go`, `bot_test.go` (new): Bot Framework token validation
- `control-plane/internal/hub/teams/activity.go`, `activity_test.go` (new): the activity subset,
  `ChannelRef`, `PlainText`, the pairing code
- `control-plane/internal/hub/teams/post.go`, `post_test.go` (new): composing a channel post; the
  throttle; the outbound Bot Connector call
- `control-plane/internal/hub/server/work_teams.go` (new): the binding routes and `POST
  /teams/messages`; `server/work_teams_integration_test.go` (new, `-tags integration`)
- `control-plane/internal/hub/store/work_teams.go` (new): bindings, pairings, post cursors
- `control-plane/internal/hub/server/janitor.go` ⚑: step 6, `sweepTeamsPosts`, after task 156's step 5
- `control-plane/internal/hub/server/work_matrix_rows.go` ⚑: the binding routes' rows
- `integrations/teams-app/manifest.json`, `color.png` (192 × 192), `outline.png` (32 × 32) (new;
  the icons drawn by us, `OD-11`)
- **Not in the File map:** `teams/{bot,activity,post}.go` and `integrations/teams-app/` (gaps to add)

**Interfaces:**

- Produces: `POST /teams/messages` (outside `/v1`: a host call, not a user's; not in task 158's
  published document); `PUT /v1/teams/{teamId}/teams-bindings`, `GET` of the same, `POST
  /v1/teams/{teamId}/teams-bindings/remove`; janitor step 6; the Teams app package
- Consumes: task 146's `store.CreateRequest(ctx, tx, NewRequest)` and its `request.created` event;
  task 143's `AppendRequestMessage`; task 12's comment writer (`store.InsertComment`, its part's
  name); task 147's `intake.Route`; task 80's `store.EvalFilter`; task 156's cursor rule
- Consumed by: task 165 (the Desk digest and *ask your Orchestrator* add `digest` bindings and bot
  commands to this bot), task 182 (Slack reuses `post.go`'s composer through an interface)

#### `teams/bot.go` (complete)

```go
// Package teams is the Hub's Microsoft Teams bot: it authenticates Bot Framework
// activities, turns a message action into a request, keeps a request's thread in
// step, and composes channel posts. Standard library only.
package teams

// BotAuthConfig pins what a Bot Framework token must say. Values from the Azure Bot
// registration; every one is a PANACEA_APP_HUB_TEAMS_* variable (§12.2 gains them).
type BotAuthConfig struct {
	AppID       string // the bot's app id: the token's aud (…_TEAMS_APP_ID)
	TenantID    string // single-tenant: the only channelData.tenant.id accepted (…_TEAMS_TENANT_ID)
	Issuer      string // "https://api.botframework.com" (UNVERIFIED for a single-tenant bot; see below)
	JWKSURL     string // from https://login.botframework.com/v1/.well-known/openidconfiguration
	ChannelID   string // "msteams": a signing key must list it in its endorsements
	ClockSkew   time.Duration // 5 min
	HTTP        *http.Client  // fetches the JWKS; refreshed at most once per 5 min, cached 24 h
	Now         func() time.Time
}

var (
	ErrBotToken   = errors.New("teams: bot token refused")
	ErrBotTenant  = errors.New("teams: activity from another tenant")
	ErrBotService = errors.New("teams: serviceUrl does not match the token")
)

// Authenticate runs before the body is used for anything. It requires
// "Authorization: Bearer <jwt>"; alg RS256 only (none, HS*, and every other alg
// refused); kid found in the endorsed key set; signature valid; iss == Issuer;
// aud == AppID; now within nbf-skew .. exp+skew; the token's serviceurl claim equal
// to the activity's serviceUrl; channelData.tenant.id == TenantID. Any failure is
// ErrBotToken, ErrBotService or ErrBotTenant, answered 401 with no body, and nothing
// is read, stored or logged but the reason word.
func (c *BotAuthConfig) Authenticate(r *http.Request, body []byte) (Activity, error)
```

**The issuer is pinned, not discovered at run time**, because a Bot Framework token is not a realm
token and must never reach `oidcauth.Validator` (and a realm token must never pass here: its `iss`
is `PANACEA_APP_OIDC_ISSUER`, which is not `Issuer`, so it fails). Whether a single-tenant bot's
channel tokens are issued by `https://api.botframework.com` or by the tenant's Entra issuer is
**UNVERIFIED** (Microsoft's documentation could not be read from the corporate network); the part
pins both values as configuration and the test covers the one configured.

#### `teams/activity.go` (complete: the fields read, and nothing else)

```go
package teams

// Activity is the subset of a Bot Framework activity the Hub reads. Unknown fields
// are ignored (the one place the Hub decodes a body that is not its own wire).
type Activity struct {
	Type         string        `json:"type"` // "message" | "invoke" | "conversationUpdate"
	ID           string        `json:"id"`
	Name         string        `json:"name"` // invoke: "composeExtension/submitAction"
	ServiceURL   string        `json:"serviceUrl"`
	ReplyToID    string        `json:"replyToId"`
	From         Account       `json:"from"`
	Conversation Conversation  `json:"conversation"`
	Recipient    Account       `json:"recipient"`
	Text         string        `json:"text"`
	TextFormat   string        `json:"textFormat"` // "plain" | "markdown" | "xml"
	ChannelData  ChannelData   `json:"channelData"`
	Value        *ActionValue  `json:"value"` // submitAction only
}
type Account struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	AADObjectID string `json:"aadObjectId"`
}
type Conversation struct {
	ID       string `json:"id"` // "19:…@thread.tacv2;messageid=…" for a channel thread
	TenantID string `json:"tenantId"`
}
type ChannelData struct {
	Tenant  struct{ ID string `json:"id"` } `json:"tenant"`
	Team    struct{ ID string `json:"id"` } `json:"team"`
	Channel struct{ ID string `json:"id"` } `json:"channel"`
}
type ActionValue struct {
	CommandID      string `json:"commandId"` // "sendToPanaceaApp"
	MessagePayload struct {
		ID   string `json:"id"`
		From struct {
			User Account `json:"user"`
		} `json:"from"`
		Body struct {
			ContentType string `json:"contentType"` // "html" | "text"
			Content     string `json:"content"`
		} `json:"body"`
	} `json:"messagePayload"`
}

// ChannelRef names one Teams place: "teams:<tenant>/<team>/<channel>" for a
// channel, with "/<root message id>" appended for a thread. It is an opaque key in
// the Hub (SW.21's channelRef), parsed only here.
type ChannelRef struct{ Tenant, Team, Channel, Thread string }

func (c ChannelRef) String() string
func ParseChannelRef(s string) (ChannelRef, error)
// Channel drops the thread: the key a binding and the throttle use.
func (c ChannelRef) Channel() ChannelRef

// PlainText is decision 3's conversion, total and deterministic: "text" content is
// returned unchanged; "html" content has <br>, </p>, </div> and </li> turned into
// "\n", every other tag dropped, HTML entities decoded by html.UnescapeString, runs
// of more than two "\n" kept as they are (a person's spacing is theirs), and
// nothing trimmed. It never fails; a malformed tag is dropped up to the next '>'.
func PlainText(contentType, content string) string

// PairingCode is 12 characters of Crockford base32 from crypto/rand (60 bits),
// shown as "XXXX-XXXX-XXXX"; no word list, so no third-party list's licence. The Hub
// stores only SHA-256(code), beside the channel's ChannelRef, for 10 minutes; one
// use; five wrong codes for one team in 10 minutes lock that team's pairing for 10.
func PairingCode() (code string, hash [32]byte, err error)
```

Only two activities do anything. An `invoke` named `composeExtension/submitAction` with
`commandId: "sendToPanaceaApp"` is intake. A `message` in a thread whose root is a request's
`channelRef` is a reply; a `message` that @mentions the bot with the text `connect` asks for a
pairing code. Every other activity answers 200 with no body and changes nothing (Bot Framework
retries a non-2xx). The bot's own messages are recognised by `From.ID == Recipient.ID` of the
inbound activity's bot account, and dropped before any lookup.

#### The bot endpoint (`server/work_teams.go`): what each activity does

`POST /teams/messages` is registered on the Hub's mux **outside** the `/v1` subtree, so Hub task
13's `Authenticate` (a realm bearer) never wraps it; `teams.Authenticate` does instead. It is
exempt from the `Origin` refusal only because Bot Framework sends none; a request that carries
one is still refused 403 first (hub-contract §5.1 step 1 is kept). The pre-auth per-IP limit
applies (§11.3). Bodies over 256 KiB are 413 before parsing.

1. **Intake** (`submitAction`, `sendToPanaceaApp`). The channel is looked up among `intake`
   bindings (`Channel()` of the activity's place); none → decision 2's card, nothing stored. Found:
   in one transaction under the binding's org (`set_config('hub.org_id', …)`), with
   `hub.work_actor_kind = 'integration'` and `hub.work_actor_user` = the invoker's user id when the
   invoker's `(tid, aadObjectId)` is a Hub user, else null:
   - `intake.Route(set, Incoming{Channel: "teams", Text: quote}, defaults, isMember)` (task 147),
     where `isMember` is the **author's** membership of the bound team;
   - `store.CreateRequest` (task 146) with `channel: teams`, `requester` per decision 4, `quote`
     `{text: PlainText(body), author: "H"}`, `channelRef` = the thread (`Channel` + the message id),
     `ruleId` from the route; task 146 emits `request.created` with metadata only;
   - the optional `note` parameter, when non-empty, as the first `RequestMessage` by the invoker.
   The bot answers the invoke with a card: *"Sent to PanaceaApp as a request in {team name}'s
   Triage."* It names no title and no rule.
2. **A reply in a request's thread** (`message`, `ReplyToID` or the conversation's `messageid`
   matching a request's `channelRef`). Appended as a `RequestMessage {requestId, channelRef (the
   reply's), authorName, userId?, body: PlainText(…)}`. If the request is `accepted` **and** the
   reply's author maps to a Hub user, the same transaction appends a comment on its issue, `text·H`,
   `author: "H"`, attributed to that user, with `source: "teams"` and the reply's `channelRef` (task
   12's comment row gains these two nullable columns; a requirement on task 145's migration). A
   reply whose Teams message id is already a `RequestMessage` is a no-op (Bot Framework redelivers).
3. **`connect`** (an @mention whose text, mentions removed and trimmed, is `connect`). A
   `PairingCode` is minted and posted in reply; the channel's `ChannelRef` and the hash are stored.

**Post-back** (the other direction, janitor step 6). A **person's** comment (`author: "H"`, actor
kind `person`) on an issue accepted from a Teams request is posted into the request's thread as
*"{displayName} (PanaceaApp): {body}"*, or, when the project is *AU data only*, as *"{displayName}
commented on {alias} in PanaceaApp: {https link}"*. A comment whose `source` is `teams` is never
posted back (it came from there: the echo loop is closed by data, not by timing). Agent comments,
notes and proposals are never posted: the step selects `author = 'H' AND actor_kind = 'person'`,
and nothing else.

#### The store (`store/work_teams.go`, signatures complete)

What this part needs from task 145's `work_teams_bindings` (a requirement on that part; its file wins
on names): `subject_kind`, `subject_id NULL`, `channel_ref`, `created_by`, `cursor_seq bigint`,
`last_posted_at timestamptz NULL`, `members text[] NULL` (a view binding's last membership, at most
2,000 ids), and a unique key on `(team_id, subject_kind, coalesce(subject_id, ''), channel_ref)`;
plus a new table `work_teams_pairings (org_id, team_ref text, code_hash bytea, channel_ref,
expires_at, used_at NULL, failures int)` with RLS and grants like its neighbours.

```go
package store

func IntakeBindingForChannel(ctx context.Context, q Querier, channelRef string) (TeamsBindingRow, bool, error)
func PutTeamsBinding(ctx context.Context, tx pgx.Tx, b TeamsBindingRow) (created bool, err error)
func RemoveTeamsBinding(ctx context.Context, tx pgx.Tx, teamID, subjectKind, subjectID, channelRef string) (bool, error)
func ListTeamsBindings(ctx context.Context, q Querier, teamID string) ([]hubapi.TeamsBinding, error)
func InsertPairing(ctx context.Context, tx pgx.Tx, channelRef string, hash [32]byte, expires time.Time) error
// RedeemPairing marks the code used and returns its channel, or found=false for an
// unknown, used or expired code (counting a failure).
func RedeemPairing(ctx context.Context, tx pgx.Tx, hash [32]byte, now time.Time) (channelRef string, found bool, err error)
func RequestByChannelRef(ctx context.Context, q Querier, threadRef string) (RequestRow, bool, error)
func PostDueBindings(ctx context.Context, tx pgx.Tx, now time.Time) ([]TeamsBindingRow, error) // last_posted_at <= now - 15 min, or null
func MarkPosted(ctx context.Context, tx pgx.Tx, b TeamsBindingRow, cursor int64, members []string, at time.Time) error

type TeamsBindingRow struct {
	hubapi.TeamsBinding
	CreatedBy    string
	CursorSeq    int64
	LastPostedAt *time.Time
	Members      []string
}
```

**Which org.** The bot endpoint has no Hub principal, so row-level security has no `hub.org_id` yet.
The activity's `channelData.tenant.id` (already checked equal to `TenantID`) is mapped to an org by
a new `SECURITY DEFINER` function `hub.org_for_tenant(tid text) RETURNS text`, the org id only (a
requirement on task 145's migration, modelled on Hub task 07a's `hub.org_ids()`); no org → 200, no
body, nothing stored. Every later statement runs after `set_config('hub.org_id', org, true)`.

#### The binding routes and their wire

```go
func (s *Server) registerWorkTeams(mux *http.ServeMux) {
	mux.HandleFunc("PUT /v1/teams/{teamId}/teams-bindings", s.putTeamsBinding)          // team admin
	mux.HandleFunc("GET /v1/teams/{teamId}/teams-bindings", s.listTeamsBindings)        // team member
	mux.HandleFunc("POST /v1/teams/{teamId}/teams-bindings/remove", s.removeTeamsBinding) // team admin
	mux.HandleFunc("POST /teams/messages", s.teamsMessages)                             // Bot Framework only
}
```

```jsonc
// PUT /v1/teams/tm-…/teams-bindings                  team admin; BindTeams (task 143)
{ "subject": { "kind": "feature", "id": "ft-…" }, "pairingCode": "7K2Q-MX9D-4TRB" }
// → 200 TeamsBinding (201 when new); the code is used up
{ "teamId": "tm-…", "subject": { "kind": "feature", "id": "ft-…" },
  "channelRef": "teams:<tenant>/19:…@thread.tacv2/19:…@thread.tacv2" }
// subject.kind "intake": no id. "feature": an ft- of the team. "view": a SHARED vw- of the team
//   (a private view is 404: posting it would share it). "digest": 400 until task 165.
// an unknown, used or expired code → 400 bad_request, field pairingCode; the fifth in 10 min
//   for one team → 429 rate_limited, details.scope "pairing"
// the same subject already bound to that channel → 200, no event
// POST /v1/teams/tm-…/teams-bindings/remove { "subject": {…}, "channelRef": "teams:…" } → 204
// GET /v1/teams/tm-…/teams-bindings → 200 { "items": [ TeamsBinding … ], "next": null }
```

Matrix rows: `PUT` and `remove` ✓ `A`, 403 `V M`, 404 `O N X`, 403 for every task 145 fixture;
`GET` ✓ `V M A`. Events: `teams.bound {subjectKind, hasSubjectId}` and `teams.unbound
{subjectKind}`, team-scoped, `VisibleToReaders`, added to SW.5's registry beside task 156's; the
`channelRef` is not in `data` (it names a customer's Teams team).

#### `teams/post.go` (complete) and janitor step 6

```go
package teams

// Change is one line of a post, built from feed events and heads, never from text
// the caller has not been allowed to post.
type Change struct {
	Alias string // "EP-142"; a feature's or view's name is never used as a line key
	Word  string // "landed" | "shipped" | "entered" | "left" | "update: on track" | …
	Title string // "" when the project is AU data only (the composer never adds one)
	Link  string // task 80's https page for the id; task 93's form for the desktop
}

// Compose makes one message for a channel: a heading naming the subject by its
// kind and alias ("Feature FT-12" | "View: shared view vw-…"), then at most 20
// lines "Alias · Word · Title" (Title omitted when ""), then "and N more" when
// over 20, then one link to the subject. Plain text; Teams renders it as sent.
func Compose(subjectLine string, changes []Change, subjectLink string) string

// Due reports whether a channel may be posted to now: never posted, or last posted
// at least 15 minutes ago (decision 6).
func Due(last *time.Time, now time.Time) bool

// Poster sends one message to a conversation through the Bot Connector at the
// activity's serviceUrl, authenticated as the bot (client credentials for
// https://api.botframework.com/.default). Its credential is the open question in
// "does NOT claim"; the tests use a recording fake.
type Poster interface {
	Post(ctx context.Context, serviceURL string, ref ChannelRef, text string) (messageID string, err error)
}
```

**Step 6, `sweepTeamsPosts`** (after task 156's step 5, the same lock; queue in the org's
transaction, send outside it, exactly as step 5). For each `PostDueBindings` row: a `feature`
binding reads events after `cursor_seq` visible to the binding's **creator** (H26's query as that
user): `work.update_posted` for that feature (the health word and the pinned brief key), `issue.state`
to `landed` for issues whose `featureId` is it, and task 131's *shipped*; a `view` binding
evaluates the view's filter with task 80's `store.EvalFilter` over the team's issue heads, diffs the
set against `members`, and lists `entered` and `left`. No change → the cursor still advances and
nothing is posted. Post-back comments (above) are sent in the same step, one per comment, and are
**not** throttled: they answer a person in their own thread. A post that fails is retried at the
next due time with the cursor unmoved; the Teams message id is stored so a retry never posts twice.

#### `integrations/teams-app/manifest.json` (complete)

The ids are placeholders the release step fills from `PANACEA_APP_HUB_TEAMS_APP_ID`; the schema
version and the resource-specific consent permission names are **UNVERIFIED** against Microsoft's
current manifest schema (not readable from the corporate network), and the part's step 1 checks
them with the Teams Developer Portal's validator before any upload.

```json
{
  "$schema": "https://developer.microsoft.com/json-schemas/teams/v1.17/MicrosoftTeams.schema.json",
  "manifestVersion": "1.17",
  "version": "1.0.0",
  "id": "{{TEAMS_APP_ID}}",
  "developer": {
    "name": "KPMG Australia",
    "websiteUrl": "https://{{HUB_HOST}}/",
    "privacyUrl": "https://{{HUB_HOST}}/p/privacy",
    "termsOfUseUrl": "https://{{HUB_HOST}}/p/terms"
  },
  "name": { "short": "PanaceaApp", "full": "PanaceaApp work intake" },
  "description": {
    "short": "Send a message to your team's PanaceaApp Triage.",
    "full": "Send a Teams message to a PanaceaApp team as a request, keep its thread in step, and follow a feature or a shared view in a channel. Agent text is never posted here."
  },
  "icons": { "color": "color.png", "outline": "outline.png" },
  "accentColor": "#0A0908",
  "bots": [
    { "botId": "{{TEAMS_APP_ID}}", "scopes": ["team"], "isNotificationOnly": false,
      "supportsFiles": false, "supportsCalling": false, "supportsVideo": false }
  ],
  "composeExtensions": [
    { "botId": "{{TEAMS_APP_ID}}", "canUpdateConfiguration": false,
      "commands": [
        { "id": "sendToPanaceaApp", "type": "action", "context": ["message"],
          "title": "Send to PanaceaApp", "description": "Make this message a request in Triage",
          "fetchTask": false,
          "parameters": [
            { "name": "note", "title": "Anything to add? (optional)", "inputType": "textarea" }
          ] }
      ] }
  ],
  "authorization": {
    "permissions": {
      "resourceSpecific": [ { "name": "ChannelMessage.Read.Group", "type": "Application" } ]
    }
  },
  "validDomains": ["{{HUB_HOST}}"]
}
```

`scopes: ["team"]` only: no personal or group-chat install, so decision 2 is enforced by where the
app can live, and the check in the endpoint is the second defence. `ChannelMessage.Read.Group` is
what lets the bot see a thread's replies without being @mentioned; without it, only replies that
mention the bot arrive (the tests cover both). The `privacyUrl` and `termsOfUseUrl` pages are
Hub `/p/` pages that do not exist yet (a gap). `#0A0908` is the `--page` token
(`desktop/src/styles/tokens.css:30`), not a brand gradient; the two icons are a plain mark in `--ink` (`#f2ede7`, `:45`) on that ground and are the owner's to accept.

#### Tests to write first, part 1 — `teams/*_test.go` (pure)

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestForgedOrExpiredBotTokenRefused` | a token signed by a key not in the set; `alg: none`; `alg: HS256` with the JWKS `n` as the secret; `exp` 10 min ago; `aud` another app id; `iss` the realm's issuer; a `serviceurl` claim unlike the body's → each an error, and the handler func the test passes was never called | unauthenticated bot; alg confusion; a realm token accepted as a bot's |
| `TestOtherTenantRefused` | a valid token, `channelData.tenant.id` another tenant → `ErrBotTenant` | a multi-tenant bot by accident |
| `TestPlainTextIsFixed` | `text` content unchanged byte for byte (with `<b>` in it, kept); `html` `<p>Hi&amp;<br>there</p>` → `"Hi&\nthere\n"`; a lone `<` kept; ten vectors in `testdata/plaintext/*.json` | a paraphrased or trimmed quote; entities left encoded |
| `TestChannelRefRoundTrips` | `ParseChannelRef(r.String()) == r` for a channel and a thread; `Channel()` drops only the thread; a string without `teams:` → error | a thread keyed as a channel (the throttle per thread) |
| `TestComposeCarriesNoTitleWhenEmpty` | `Compose` with `Title: ""` rows → no ` · ` after the word; 25 changes → 20 lines and `and 5 more` | a title slipping in; an unbounded post |
| `TestDueIsFifteenMinutes` | last 14 m 59 s ago → false; 15 m → true; nil → true | a firehose channel |
| `TestPairingCodeShape` | 12 Crockford characters in three groups; 1,000 codes distinct; the hash is SHA-256 of the dashed form | a guessable or stored-plain code |

#### Tests to write first, part 2 — `server/work_teams_integration_test.go` (`//go:build integration`)

A test key pair signs Bot Framework tokens; `BotAuthConfig.JWKSURL` points at an `httptest` JWKS;
`Poster` is a recording fake. Team `tm-a` (admin Ada, member Linus), a Teams channel paired to it.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestForgedActivityCreatesNothing` | a forged or expired token on `submitAction` → 401; `work_requests` count unchanged | unauthenticated bot |
| `TestQuoteIsTheMessageText` | `submitAction` on a `text` message with a canary → one request, `channel: teams`, `quote.text` equal to the content byte for byte, `author: "H"`; its `request.created` event has no canary | paraphrased quote; quote in the feed |
| `TestUnlinkedChannelMakesNoRequest` | the same action from an unpaired channel → the card, no row | intake from anywhere |
| `TestReplyAfterAcceptIsOneCommentAndNoEcho` | accept (task 146); Linus replies in the thread → one `ic-` comment by Linus, `source: teams`; step 6 runs → nothing posted for that comment; Ada comments in the desktop → one post into the thread, and that post's arrival as an activity is a no-op | echo loop |
| `TestAgentTextNeverPosted` | an Orchestrator `note` and an agent comment (`author: "A"`) on the issue → step 6 posts nothing | agent text in Teams |
| `TestThirtyViewChangesOnePost` | a view binding; 30 issues enter the view over 10 minutes of fake clock with a sweep each minute → exactly one post, listing 20 and `and 10 more` | a firehose channel |
| `TestAUDataOnlyPostsNoTitle` | an *AU data only* project; a feature binding; an issue with a title canary lands → the post has the alias and `landed`, and no canary; Ada's comment there posts the link form, not its body | title leaves for Teams |
| `TestUnbindStopsPosts` | remove the binding, then a change → the next sweep posts nothing | stale binding |
| `TestPrivateViewCannotBeBound` | `PUT` with a private `vw-` → 404 | a private view shared by a channel |
| `TestBindingRoutesMatrix` | the three routes' rows, including 403 for every task 145 fixture | a missing role row |

#### Steps

- [ ] **Step 1: Check the base and the tenant.** Tasks 146, 147 and 156 merged; task 145 carries
  the columns and `hub.org_for_tenant`. Validate `manifest.json` in the Teams Developer Portal and
  record the schema version it accepts; confirm the Bot Framework issuer for a single-tenant bot and
  set `Issuer` to it. Both are owner or tenant acts; the code does not wait on them.
- [ ] **Step 2: Part 1's tests**, then `bot.go`, `activity.go`, `post.go`, until green.
- [ ] **Step 3: Part 2's tests** (they fail), then the store, the routes, the endpoint and step 6.
- [ ] **Step 4: Draw the two icons** (a plain mark; no third-party art) and add the package files.
- [ ] **Step 5: Run the neighbours** (tasks 146, 147, 156 and Hub task 24's janitor tests), then
  commit by pathspec.

#### Acceptance

```bash
go test ./control-plane/internal/hub/teams/ -race -count=1 -v 2>&1 | grep -c '^--- PASS'     # 7
export PANACEA_APP_HUB_TEST_DATABASE_URL='postgres://postgres@127.0.0.1:5433/postgres?sslmode=disable'
go test ./control-plane/internal/hub/server/ -tags integration -race -count=1 -v \
  -run '^Test(ForgedActivityCreatesNothing|QuoteIsTheMessageText|UnlinkedChannelMakesNoRequest|ReplyAfterAcceptIsOneCommentAndNoEcho|AgentTextNeverPosted|ThirtyViewChangesOnePost|AUDataOnlyPostsNoTitle|UnbindStopsPosts|PrivateViewCannotBeBound|BindingRoutesMatrix)$' \
  2>&1 | grep -c '^--- PASS'                                                                # 10
go test ./control-plane/internal/hub/... -tags integration -race -count=1                  # every package ok
python -c "import json;json.load(open('integrations/teams-app/manifest.json'))"; echo $?     # 0
```

Expected: `7`, `10`, every package `ok`, then `0` (the manifest parses; schema validity is step 1's).

#### What this task does NOT claim

- **Not that KPMG's Teams admin permits a custom app**, or grants `ChannelMessage.Read.Group`
  (**UNVERIFIED**; a tenant act). Without that consent, only replies that @mention the bot sync.
- **Not the bot's credential.** Posting through the Bot Connector needs the bot to obtain a token
  (client credentials against `login.microsoftonline.com`); whether the Azure Bot can use a managed
  identity rather than a secret is **UNVERIFIED**. The Hub's contract allows no client secret and
  no outbound call beyond sign-in discovery (`hub-contract:2083-2085`, `:2211`), so the bot's
  credential and its calls to the Bot Connector's `serviceUrl` are part of the proposed
  Hub-contract addition task 145 names. Until it is accepted, `Poster` is nil in `serve.go`, the
  endpoint still records intake and replies, and step 6 posts nothing.
- **Not the Bot Framework issuer** for a single-tenant bot (see *bot.go*), nor the manifest's
  schema version: both are pinned as configuration and checked in step 1.
- **Not the digest or asking your own Orchestrator** (L-207's other half): task 165.
- **Not Slack** (task 182), and not a Teams message edit or delete after intake: the request keeps
  the words as they were sent (the quote is frozen, task 145).
- **Not that Teams stores content in Australia.** An *AU data only* project posts no titles and no
  comment bodies for that reason; the residency of Teams itself is the tenant's.

**Parity rows:** closes **L-80**, **L-81**, **L-97** and **L-137**, and with task 149 **L-178**;
**L-207** closes with task 165.

#### Gaps found while writing this part

1. The File map has no rows for `teams/{bot,activity,post}.go`, `integrations/teams-app/`, or
   step 6 in `server/janitor.go`.
2. Task 145 must add `hub.org_for_tenant(tid)`, the `work_teams_bindings` columns above, the
   `work_teams_pairings` table, and two nullable columns on task 12's comments (`source`,
   `channel_ref`). None is in its scope.
3. `teams.bound` and `teams.unbound` join SW.5's registry; task 143's `TeamsBinding` needs no
   change, but `BindTeams`'s request carries `pairingCode`, which task 143 must name.
4. The Hub `/p/privacy` and `/p/terms` pages the manifest needs have no task.
5. Task 131's *shipped* event kind is not fixed in its part; step 6 reads it by the name that part
   settles on.
6. No desktop task owns the settings row that takes a pairing code and lists bindings: task 173 (admin: apps, keys, webhooks, connectors, harness) names no Teams row. It should gain one.
