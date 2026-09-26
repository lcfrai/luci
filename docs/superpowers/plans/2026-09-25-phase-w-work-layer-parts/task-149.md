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
