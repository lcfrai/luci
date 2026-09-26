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
   release (`hub-contract.md:974-991`) holds a commit, not issues; *shipped* is derived in the
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
