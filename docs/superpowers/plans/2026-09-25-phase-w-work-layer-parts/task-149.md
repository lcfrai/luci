### Task 149: mail: an intake address per team; the requester hears *shipped*, with the release name, where they asked

W5, stream WH; no seam. Depends on task 81 (the `mailer` seam, its Graph sender and fake), task
131 (*shipped to*, derived from `Panacea-Issue` trailers) and task 146 (`createRequest`,
`RequestsOfIssue`). Estimate 0.375 (3 h). Closes **L-31** (*"An intake request `rq-` by email
address per team"*), **L-138** (PARTIAL: plain-text bodies only, no attachments) and **L-140**
(*"The requester hears when it **shipped**, with the release that proves it"*), jointly with task
157 for the Teams path; and **L-178**'s email half (intake from a thread keeps the quote). Every
`server/`, `store/` and `mailer/` path is **UNVERIFIED until merge** (rule H1); task 81's part is not
written at this writing, so its names below are taken from its scope (`tasks-81-100.md:42-46`:
`mailer.Mailer{Send(ctx, Message) error}`, `Message{To, Subject, Text, HTML, IdempotencyKey}`).

Two things, both about people outside the team. **In:** each team gets an email address; a mail to
it becomes a request with the sender's words as the quote. **Out:** when a release that carries an
issue is confirmed, everyone whose request became or joined that issue hears, once, where they
asked: a reply in their own mail thread, or a mail to the person who used the form. The message
says what shipped, where and when, and names the release, because *shipped* is a fact the release
proves (05 §2.3), not a ticket's state.

Eight decisions the scope left open, made here.

1. **One shared mailbox, one alias per team.** The Hub reads one intake mailbox
   (`PANACEA_APP_HUB_MAIL_INTAKE_MAILBOX`); each team's address is an alias on it, set by a team
   admin (`PUT /v1/teams/{teamId}/intake-address`). Creating the alias in Exchange is a tenant
   admin's act the Hub cannot do (**UNVERIFIED** that any tenant grants it; *does not claim*); the
   Hub records which address is whose and refuses one already taken, in any org.
2. **The mailbox is global, so its cursor and its address book are not an org's.** A mail arrives
   before the Hub knows its org. The inbound pass runs once per janitor sweep, **outside** the
   per-org loop, under the janitor's advisory lock: it reads the delta page, finds each recipient's
   `(org, team)` through a `SECURITY DEFINER` function that answers only that pair, and creates the
   request in a transaction of that org with `hub.org_id` set, as every other write. The delta
   cursor is one row in a table with no org column and no row-level security, readable and writable
   only by `hub_janitor` (Hub task 24's role, **UNVERIFIED** name), never by `hub_app`.
3. **A mail's sender is an `email` requester, always.** The scope says *"a `person` when the
   address matches an org user"*. A mail's `From` is not authenticated to the Hub: anyone can send
   a mail that claims a member's address, and a `person` requester would get member defaults (task
   147) and read *"Ada Lovelace asked"*. So the Hub never promotes a mail to a person; the Triage
   view (task 170) shows *same address as Ada Lovelace, not verified* by a lookup at read time.
   Promoting on a verified signal (Exchange's `Authentication-Results`, `dmarc=pass`, inside the
   tenant) is **owner question Q-149** in *Gaps*; it is one branch in `fromMail` and one test.
4. **The quote is the new text only.** The plain-text body (Graph's
   `Prefer: outlook.body-content-type="text"`) is cut at the first reply marker (`On … wrote:`,
   `-----Original Message-----`, an Outlook `From:` header block, a `________________________________`
   rule, the first line that starts with `>` after a blank line) and at a signature delimiter
   (`-- ` alone on a line); trailing blank lines go. Lines cut are counted in `quoteTrimmed`, shown
   as *trimmed 14 lines*. A body still over 16 KiB (task 146's `MaxRequestQuoteBytes`) is cut at
   the last line boundary under it and those lines are counted too, so a mail can never fail to
   become a request by being long. A body that is empty after the cut keeps its first 16 KiB whole:
   an ask is never recorded as nothing.
5. **A message becomes one request per team it names.** A mail to two team aliases makes two
   requests, one each, with the same `Message-ID` as `channel_ref` (task 146's
   `work_requests_channel_once` is per team). A mail whose recipients name no alias (a Bcc, a
   retired alias) makes none, and the sweep counts it as `unrouted` so an operator can see it.
6. **The Hub learns which issues a release carries from the release's reserver.** The Hub's
   release (`hub-contract.md:976-991`) holds a commit, not issues; *shipped* is derived in the
   control plane from the trailer walk (task 71, `tasks-61-80.md:359-368`; task 131 decision 8).
   So the Ship flow, after confirm, sends the ids it already derived:
   `PUT /v1/releases/{releaseId}/issues {issueIds}`, reserver only, confirmed releases only, once
   (a second PUT with the same set is 200, a different set 409). Ids only, never titles. This route
   is **not in the skeleton** (gap 1); without it the Hub cannot tell anyone anything.
7. **Once per request and environment, on announced environments only.** A team's intake settings
   carry `announceEnvironments` (default `["production"]`); a confirmed `run` release to one of
   them tells each linked requester once for that environment, idempotency key `rq-…/rl-…` of the
   first such release. A redeploy of the same commit to the same environment tells nobody again; a
   rollback tells nobody; a branch preview environment is silent unless the team names it. Without
   this, a requester would hear about every preview and every redeploy.
8. **The message names facts and no text.** *"EP-142 shipped to production at 14:02 AEST in
   release engagement-portal-build-x7k2p."* The alias (task 02's `DisplayAlias`), the environment,
   the confirm time in `PANACEA_APP_HUB_MAIL_TZ` (default `Australia/Sydney`) with its zone, and
   the release's name. **The Hub's release has no name field**, so the name is its `pipelineRun`,
   else its `rl-` id (gap 3; the scope's *2026.09.25-2* is not a field the contract has). No
   description, comment, title or anyone else's request: whether the title may be added follows
   task 81's open question, and until it is answered the title is left out.

#### Files

- `control-plane/internal/hub/mailer/inbound.go` and `inbound_test.go` (new): `Inbox`,
  `InboundMessage`, and the pure `CutReply`.
- ⚑ `control-plane/internal/hub/mailer/graph.go`, `fake.go` (task 81): the read side (`Delta`) and
  `Reply`; `graph.go` is tested against an `httptest` server only.
- ⚑ `control-plane/internal/hub/mailer/mailer.go` (task 81): `Reply` joins the interface.
- `control-plane/internal/hub/server/work_mail.go` (new): the intake-address and release-issues
  routes, `passInbound`, `passShipped`.
- `control-plane/internal/hub/store/work_mail.go` (new) and
  `server/work_mail_integration_test.go` (new, `-tags integration`).
- ⚑ `control-plane/internal/hub/server/janitor.go` (Hub task 24; tasks 77, 81, 121): `passInbound`
  once per sweep before the per-org loop, and `passShipped` in the per-org transaction after task
  121's passes, in task order.
- ⚑ `control-plane/cmd/hub/config.go`: `PANACEA_APP_HUB_MAIL_INTAKE_MAILBOX`,
  `PANACEA_APP_HUB_MAIL_TZ`.
- ⚑ `control-plane/internal/hub/hubclient/work.go` (task 04; task 143's `WorkIntake`):
  `RecordReleaseIssues`. ⚑ `control-plane/internal/deploy/ship/ship.go` (P7; task 131's delta):
  one call after confirm (rule H2: phase 7's file wins).
- ⚑ `hubapi/work_intake.go`, ⚑ `hubapi/workevents.go`, ⚑ `server/work_matrix_rows_integration_test.go`.

#### The tables this task needs from task 145's migration (requirement; complete)

```sql
-- ── task 149 (in task 145's NNNN_work_w5.sql): mail intake and shipped notices ──────
CREATE TABLE work_intake_settings (
  team_id               text PRIMARY KEY,
  org_id                text NOT NULL REFERENCES orgs (id),
  address               text UNIQUE CHECK (address = lower(address) AND octet_length(address) <= 320),
  announce_environments text[] NOT NULL DEFAULT ARRAY['production']
                        CHECK (cardinality(announce_environments) <= 20),
  updated_at            timestamptz NOT NULL,
  updated_by            text NOT NULL,
  FOREIGN KEY (org_id, team_id)    REFERENCES teams (org_id, id),
  FOREIGN KEY (org_id, updated_by) REFERENCES users (org_id, id)
);
-- address is UNIQUE across every org: one mailbox, so one owner per alias (decision 1).

CREATE TABLE work_release_issues (
  org_id      text NOT NULL REFERENCES orgs (id),
  release_id  text NOT NULL,
  issue_id    text NOT NULL,
  recorded_at timestamptz NOT NULL,
  PRIMARY KEY (release_id, issue_id),
  FOREIGN KEY (org_id, release_id) REFERENCES releases (org_id, id),
  FOREIGN KEY (org_id, issue_id)   REFERENCES issues (org_id, id)
);

CREATE TABLE work_request_notices (
  org_id      text NOT NULL REFERENCES orgs (id),
  request_id  text NOT NULL,
  environment text NOT NULL,
  release_id  text NOT NULL,
  channel     text NOT NULL CHECK (channel IN ('email', 'form', 'teams')),
  state       text NOT NULL DEFAULT 'pending' CHECK (state IN ('pending', 'sent', 'failed')),
  attempts    integer NOT NULL DEFAULT 0,
  created_at  timestamptz NOT NULL,
  sent_at     timestamptz,
  PRIMARY KEY (request_id, environment),                       -- decision 7: once per environment
  FOREIGN KEY (org_id, request_id) REFERENCES work_requests (org_id, id),
  FOREIGN KEY (org_id, release_id) REFERENCES releases (org_id, id)
);

DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['work_intake_settings', 'work_release_issues', 'work_request_notices'] LOOP
    EXECUTE format('ALTER TABLE hub.%I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE hub.%I FORCE ROW LEVEL SECURITY', t);
    EXECUTE format('CREATE POLICY org_isolation ON hub.%I TO hub_app
                    USING (org_id = current_setting(''hub.org_id'', true))
                    WITH CHECK (org_id = current_setting(''hub.org_id'', true))', t);
  END LOOP;
END $$;
GRANT SELECT, INSERT ON work_intake_settings, work_release_issues, work_request_notices TO hub_app;
GRANT UPDATE (address, announce_environments, updated_at, updated_by) ON work_intake_settings TO hub_app;
GRANT UPDATE (state, attempts, sent_at) ON work_request_notices TO hub_app;

-- The mailbox's own state: no org column, no RLS, the janitor's role only (decision 2).
CREATE TABLE work_mail_cursor (
  mailbox     text PRIMARY KEY,
  delta_link  text NOT NULL CHECK (octet_length(delta_link) <= 4096),
  updated_at  timestamptz NOT NULL
);
REVOKE ALL ON work_mail_cursor FROM PUBLIC, hub_app;
GRANT SELECT, INSERT, UPDATE ON work_mail_cursor TO hub_janitor;

-- intake_address_owner answers exactly one (org, team) pair for an address, and
-- nothing else; the janitor calls it before it knows the org.
CREATE FUNCTION intake_address_owner(p_address text, OUT org_id text, OUT team_id text)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = hub, pg_temp AS $$
  SELECT s.org_id, s.team_id FROM work_intake_settings s WHERE s.address = lower(p_address)
$$;
REVOKE ALL ON FUNCTION intake_address_owner(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION intake_address_owner(text) TO hub_janitor;
```

`releases` is the Hub's table for §7's releases (Hub task 20's, **UNVERIFIED** name until it
lands; it must carry a unique `(org_id, id)` for the composite keys, as `issues` does). The
`work_request_notices` row is an **outbox**: `passShipped` inserts it `pending` in the same
transaction that reads the release, and sends it after commit; the primary key is what makes
*told twice* impossible even when two sweeps race.

#### `mailer/inbound.go` (complete)

```go
package mailer

// Inbox reads the shared intake mailbox. Delta returns the messages new since
// cursor ("" is the start) and the cursor to store once they are all handled.
type Inbox interface {
	Delta(ctx context.Context, cursor string) (msgs []InboundMessage, next string, err error)
}

// InboundMessage is what the Hub keeps of a mail: the ids, the sender, the
// recipients it routes by, and the plain-text body. Attachments are not read.
type InboundMessage struct {
	InternetMessageID string   // the Message-ID header, "<…@…>"; the request's channel_ref
	FromAddress       string   // lower-cased
	FromName          string
	Recipients        []string // To and Cc, lower-cased; Bcc is not visible and never routes
	TextBody          string
	ReceivedAt        time.Time
}

// Replier answers a mail in its own thread. Graph's reply action on the
// original message sets In-Reply-To and References itself, which a sendMail
// cannot: Graph accepts only X- custom headers on a new message.
type Replier interface {
	Reply(ctx context.Context, r Reply) error
}

type Reply struct {
	InternetMessageID string // the message to answer, found by $filter=internetMessageId eq '…'
	Text              string
	IdempotencyKey    string // "rq-…/rl-…"; a second Reply with the same key sends nothing
}

var markers = []*regexp.Regexp{
	regexp.MustCompile(`^On .+ wrote:\s*$`),
	regexp.MustCompile(`^-{2,}\s*Original Message\s*-{2,}\s*$`),
	regexp.MustCompile(`^_{20,}\s*$`),
	regexp.MustCompile(`^From: .+`), // Outlook's quoted header block starts here
	regexp.MustCompile(`^-- $`),     // RFC 3676's signature delimiter
}

// CutReply keeps the new text of a reply and counts the lines it drops
// (decision 4). It never returns "" for a non-empty body.
func CutReply(body string, maxBytes int) (kept string, trimmed int) {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	cut := len(lines)
	for i, l := range lines {
		if slices.ContainsFunc(markers, func(re *regexp.Regexp) bool { return re.MatchString(l) }) ||
			(i > 0 && strings.HasPrefix(l, ">") && strings.TrimSpace(lines[i-1]) == "") {
			cut = i
			break
		}
	}
	keep := lines[:cut]
	for len(keep) > 0 && strings.TrimSpace(keep[len(keep)-1]) == "" {
		keep = keep[:len(keep)-1]
	}
	if len(keep) == 0 { // the whole body was a quote: keep it rather than nothing
		keep = lines
	}
	for len(keep) > 1 && len(strings.Join(keep, "\n")) > maxBytes {
		keep = keep[:len(keep)-1]
	}
	kept = strings.Join(keep, "\n")
	if len(kept) > maxBytes { // one line over the cap: cut at a rune boundary
		kept = strings.ToValidUTF8(kept[:maxBytes], "")
	}
	return kept, len(lines) - len(keep)
}
```

The body is normalised to `\n` **before** it is kept, so a mail's quote holds `\n` line ends. That
is the one change the Hub makes to a mailed quote, and it is made because mail transport rewrites
line ends anyway (RFC 5322 requires CRLF on the wire): the requester did not choose them. Task
146's *byte for byte* holds from the stored quote on. A body whose **first** line begins *From:*
matches the Outlook marker at once and would cut to nothing; the empty-result rule then keeps the
whole body, and `TestAFirstLineFromIsKept` pins that. The loop that drops lines to fit the cap
joins the lines on every pass, quadratic in the line count; the 16 KiB cap bounds it at a few
hundred lines, **unmeasured**.

**`graph.go`'s read side** (task 81's file, appended): `Delta` calls
`GET /v1.0/users/{mailbox}/mailFolders/inbox/messages/delta?$select=internetMessageId,from,toRecipients,ccRecipients,body,receivedDateTime`
with `Prefer: outlook.body-content-type="text"`, follows `@odata.nextLink` to the end of the round,
and returns `@odata.deltaLink` as `next`; a `cursor` that is a stored delta link is called as
given. `Reply` finds the message by `$filter=internetMessageId eq '<…>'` and calls `POST
…/messages/{id}/reply` with `{"comment": text}`. Both use the Hub's workload identity through task
81's token source (no secret in any variable). A `410 Gone` on a delta link (Graph's *sync state
not found*) restarts from `""` and relies on `work_requests_channel_once` to create nothing twice.
Every Graph path and header here is from Microsoft's public Graph v1.0 reference as the author
knows it, **UNVERIFIED** against a tenant; `graph_test.go` pins the requests the code makes to an
`httptest` server and nothing more. The fake records `Delta` pages it is given and every `Reply`.

#### `hubapi/work_intake.go`, the additions (requirement on task 143; complete)

```go
// ── Task 149: mail intake and shipped notices (SW.21) ──────────────────────

// IntakeSettings is GET and PUT /v1/teams/{teamId}/intake-address.
type IntakeSettings struct {
	TeamID               string   `json:"teamId"`
	Address              string   `json:"address"`              // "" when the team has none
	AnnounceEnvironments []string `json:"announceEnvironments"` // default ["production"]
	UpdatedAt            string   `json:"updatedAt"`
	UpdatedBy            *User    `json:"updatedBy"`
}

type PutIntakeSettingsRequest struct {
	Address              string   `json:"address"`              // lower-case mailbox, or "" to clear
	AnnounceEnvironments []string `json:"announceEnvironments"` // 0-20 Panacea environment names
}

// RecordReleaseIssuesRequest is PUT /v1/releases/{releaseId}/issues (decision 6).
type RecordReleaseIssuesRequest struct {
	IssueIDs []string `json:"issueIds"` // 0-2000 is- ids, distinct; ids only, never titles
}

const MaxReleaseIssues = 2000

const (
	EventIntakeAddressSet      = "intake.address_set"
	EventReleaseIssuesRecorded = "release.issues_recorded"
	EventRequestTold           = "request.told"
)

type IntakeAddressSetData struct {
	HasAddress bool  `json:"hasAddress"` // the address itself is not in the feed
	Announce   int64 `json:"announce"`   // how many environments
}
type ReleaseIssuesRecordedData struct {
	Count int64 `json:"count"`
}
// RequestToldData: which release told the requester, and on which environment.
type RequestToldData struct {
	ReleaseID   string `json:"releaseId"`
	Environment string `json:"environment"`
	Channel     string `json:"channel"`
}
// Appended to WorkEventKinds:
//	{EventIntakeAddressSet, IntakeAddressSetData{}, ScopeTeam, VisibleToReaders},
//	{EventReleaseIssuesRecorded, ReleaseIssuesRecordedData{}, ScopeProject, VisibleToReaders},
//	{EventRequestTold, RequestToldData{}, ScopeTeam, VisibleToReaders},

// On hubclient.WorkIntake (task 143), for the Ship flow (decision 6):
//	RecordReleaseIssues(ctx context.Context, releaseID string, issueIDs []string) error
```

`release.` is the Hub's own prefix (`release.reserved`, `.confirmed`, `.abandoned`,
`hub-contract.md:1155-1157`), so `release.issues_recorded` is a **Work kind under a Hub prefix**:
task 13's prefix check (`task-13.md:524`) does not see `release.` today, and this task adds the one
kind to its list by name rather than the whole prefix, so a Hub kind is never mistaken for a Work
one. The address is not in `intake.address_set`: it may name a client (*payments-acme@…*), and the
feed carries ids and counts.

#### The passes (`server/work_mail.go`, contract complete)

```go
package server

// passInbound runs once per sweep, before the per-org loop, under the
// janitor's advisory lock (decision 2). It reads one delta round, makes the
// requests, and stores the cursor only after every message is committed; an
// error leaves the cursor where it was and the next sweep reads the round
// again, which work_requests_channel_once makes harmless.
func (s *Server) passInbound(ctx context.Context, jc *JanitorConn, r *SweepReport) error {
	cur, err := store.MailCursor(ctx, jc, s.cfg.MailIntakeMailbox)
	if err != nil {
		return err
	}
	msgs, next, err := s.inbox.Delta(ctx, cur)
	if err != nil {
		return err
	}
	for _, m := range msgs {
		owners, err := store.IntakeAddressOwners(ctx, jc, m.Recipients) // intake_address_owner, per recipient
		if err != nil {
			return err
		}
		if len(owners) == 0 {
			r.MailUnrouted++ // decision 5: counted, never guessed
			continue
		}
		for _, o := range owners { // one request per team named (decision 5)
			if err := s.inOrg(ctx, o.OrgID, func(tx *Tx) error { return s.fromMail(ctx, tx, o.TeamID, m, r) }); err != nil {
				return err
			}
		}
	}
	return store.SetMailCursor(ctx, jc, s.cfg.MailIntakeMailbox, next, s.cfg.Now())
}

// fromMail makes one request of a mail for one team: channel email, requester
// kind email (decision 3), the cut quote, channel_ref the Message-ID. A
// re-delivery is ErrRequestSeen, counted and not an error.
func (s *Server) fromMail(ctx context.Context, tx *Tx, teamID string, m mailer.InboundMessage, r *SweepReport) error {
	quote, trimmed := mailer.CutReply(m.TextBody, hubapi.MaxRequestQuoteBytes)
	_, err := s.createRequest(ctx, tx, store.RequestRow{TeamID: teamID, Channel: string(hubapi.ChannelEmail),
		ChannelRef: m.InternetMessageID, RequesterKind: string(hubapi.RequesterEmail),
		RequesterEmail: m.FromAddress, RequesterName: truncRunes(m.FromName, hubapi.MaxRequesterNameChars),
		Quote: quote, QuoteTrimmed: trimmed})
	if errors.Is(err, store.ErrRequestSeen) {
		r.MailSeen++
		return nil
	}
	if err == nil {
		r.MailRequests++
	}
	return err
}

// passShipped runs in each org's janitor transaction after task 121's passes.
// For each confirmed run release to an announced environment, each issue it
// carries (work_release_issues), and each request accepted into or linked to
// that issue: INSERT a pending notice ON CONFLICT (request_id, environment)
// DO NOTHING; for each row inserted, emit request.told. After the transaction
// commits, sendNotices delivers the pending ones (a crash between the two
// leaves them pending for the next sweep).
func (s *Server) passShipped(ctx context.Context, tx *Tx, now time.Time, r *SweepReport) error

// sendNotices, after commit, for each pending notice with attempts < 5: build
// the text (noticeText), then email → s.replier.Reply on the stored Message-ID;
// form → s.mailer.Send to the requester user's address; teams → task 157's
// sender (a no-op until it lands, the notice stays pending). Mark sent, or
// attempts+1 and failed at 5. The idempotency key is "rq-…/rl-…".
func (s *Server) sendNotices(ctx context.Context, org string, r *SweepReport) error

// noticeText is pure (decision 8):
//	"EP-142 shipped to production at 14:02 AEST in release engagement-portal-build-x7k2p."
func noticeText(alias, environment string, confirmedAt time.Time, loc *time.Location, releaseName string) string {
	return fmt.Sprintf("%s shipped to %s at %s in release %s.", alias, environment,
		confirmedAt.In(loc).Format("15:04 MST"), releaseName)
}
```

`JanitorConn`, `s.inOrg` (a transaction with `hub.org_id` set, the org's write lock, as Hub task
24's per-org loop opens it) and `SweepReport` are Hub task 24's (`H24:80-101`, as task 121 cites
them); the three counters `MailRequests`, `MailSeen`, `MailUnrouted`, and `Told`, are added to
`SweepReport` beside task 121's. `truncRunes` cuts a display name to 200 code points. The date is
not in the message: a requester reads the mail the day it is sent, and a time with its zone is the
fact the release proves. Whether a mail on the day after should say the date is a copy question
for the owner, not a rule.

**What `passShipped` selects**, one statement, under the org's row-level security:

```sql
INSERT INTO work_request_notices (org_id, request_id, environment, release_id, channel, created_at)
SELECT DISTINCT ON (q.id, rl.environment) q.org_id, q.id, rl.environment, rl.id, q.channel, $1
  FROM work_requests q
  JOIN work_release_issues ri ON ri.issue_id = q.issue_id
  JOIN releases rl           ON rl.id = ri.release_id AND rl.state = 'confirmed' AND rl.kind = 'run'
  JOIN work_intake_settings s ON s.team_id = q.team_id AND rl.environment = ANY (s.announce_environments)
 WHERE q.state IN ('accepted', 'linked') AND q.erased_at IS NULL AND q.channel <> 'member'
 ORDER BY q.id, rl.environment, rl.confirmed_at, rl.id
ON CONFLICT (request_id, environment) DO NOTHING
RETURNING request_id, environment, release_id, channel;
```

`DISTINCT ON … ORDER BY confirmed_at` picks the **first** release that reached the environment, so
the notice names the release that proved it first. A request linked **after** its issue shipped is
told on the next sweep: *it shipped* is still true, and the requester asked. A `member` request is
never told (a team member recorded it, and there is no address to answer); an erased one is never
told (its address is gone). A team with no settings row announces nothing, which is the default
until an admin sets an address. The statement scans linked requests and their issues' releases on
every sweep; its cost at a real team's volume is **unmeasured**, and a `closed_at` watermark is the
knob if a measurement says so.

#### The routes (`server/work_mail.go`)

- `GET /v1/teams/{teamId}/intake-address` (viewer) → 200 `IntakeSettings`; a team with no row
  reads `address: ""`, `announceEnvironments: ["production"]`.
- `PUT /v1/teams/{teamId}/intake-address` (team admin), in `s.Write`: `Decode`; the role; the
  address is `""` or a lower-case mailbox that `net/mail` parses with no name, whose domain is the
  intake mailbox's (`@` + the part after `@` of `PANACEA_APP_HUB_MAIL_INTAKE_MAILBOX`), else 400
  `field: address`; each environment is Panacea's `[a-z0-9-]{1,63}` (`hub-contract.md:1008`), at
  most 20, distinct, else 400 `field: announceEnvironments`; upsert; a `23505` on `address` is 409
  `intake_address_taken` (the same answer whichever org holds it); emit `intake.address_set`; 200.
- `PUT /v1/releases/{releaseId}/issues` (the release's reserver), in `s.Write`: `Decode`; the
  release under row-level security, else 404 `release_not_found`; not the reserver → 403
  `not_reserver` (the Hub's own code for confirm, `hub-contract.md:240`, `:1028`); not `confirmed` → 409
  `release_state`; every id is an issue of the release's project, else 400 `field: issueIds[k]`;
  a recorded set equal to this one → 200 with nothing written; a different one → 409
  `release_issues_recorded`; else insert, emit `release.issues_recorded {count}`; 200.

The Ship flow's call (⚑ `ship/ship.go`, after the Hub's confirm succeeds): `hub.Work().
RecordReleaseIssues(ctx, releaseID, ids)` with the ids task 71's walk already returned for the
release's range. A failure is logged and retried by the same flow's retry (the Hub's
`Idempotency-Key` rule, `hub-contract.md:970-972`), and never fails the Ship: the deployment
happened, and a missing notice is a lesser fault than a Ship that reports failure for a deployment
that succeeded. That the Ship flow has the ids at that moment is task 131's decision 8 and task
71's walk, **UNVERIFIED until `feat/phase-7-deploy` merges** (rule H2).
