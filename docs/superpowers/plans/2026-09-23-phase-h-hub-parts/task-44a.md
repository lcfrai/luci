### Task 44a: the contract made true **before** anyone builds — A1–A17, the additions, and the sibling asks' answers

**`feat/hub`, FIRST: before task 01, and before the four lanes branch. Depends on: nothing. About
1.5 hours. Documents only.** The orchestrator announces the merged commit to phases 3, 4, 5 and 7,
with the read path each sibling uses from its own worktree (step 3).

ADR-H1 §7.2 lists amendments *"each made by the phase that next edits the file"*; this phase is
that phase for four documents, and it edits two cells of ADR-H1 itself (A15, owner question Q12). This plan made seventeen corrections to the Hub contract (A1–A17), several
additions, and answered every sibling ask (task 00). They are folded in **now**, because every
sibling reads the contract tonight — §7.4's repo-key regex (phase 4 HA4-6 and phase 7 already flag
it), §12.1's four-service compose, §9.4's missing `Authorization` CORS header — and a phase that
builds against the old text is the failure this program has paid for a dozen times. What the
build changes afterwards is task 44b's, last.

**Files:**
- `docs/superpowers/contracts/2026-09-23-hub-contract.md` (modify)
- `docs/PROGRAM.md` (modify ⚑ — other phases edit it too; additions only)
- `docs/superpowers/contracts/2026-09-18-phase-2-ui-contract.md` (modify: one note in §4.5; §4.4's `openAttach` row and §4.6's paragraph edited in place, re-review b02 m3)
- `docs/superpowers/planning/2026-09-23-phases-4-5-7/provenance-contract-draft.md` (modify: one amendments block at the top)
- `docs/superpowers/adr/ADR-H1-hub-architecture.md` (modify: exactly two cells, both A15's — §5.3's invite row and §9 item 3; owner question Q12, plan re-review b05 N1 and N3)

#### What changes, exactly

**The Hub contract.** Each change is made **in place**, at the section it corrects, and marked
`(Phase H plan, A<n>)` — or `(Phase H plan, task 00, <ask id>)` for a sibling answer — so a reader
can trace it; a new §16, *"Amendments from the Phase H plan"*, lists them in one table with the task
that builds each:

| Section | Change |
|---|---|
| §7.4 | A1: the two repo-key regexes and the lowercase rule; the `repo_already_linked` message sentence (task 17) |
| §12.1 | A2: Postgres-only compose; `devstack.sh`; why |
| §9.4, §13 C3 | A3: the only routes without the key are `/v1/healthz` and `/api/v1/healthz`, mounted outside the guard. Review I3 replaces §9.4's attach exemption and its *"later work"* sentence: the attach WebSocket carries the key as its second subprotocol (`["panacea.v1", "panacea.key." + key]`), compared in constant time, and the control plane echoes only `panacea.v1` (task 37); *"calls it once at start"* gains *"and again for each attach handshake"* (task 38) |
| §9.4 | A4: `Access-Control-Allow-Headers: Content-Type, Authorization`; the key check inside the origin guard |
| §2, §6.4 | A5: `internal/devissuer` as a package, `cmd/devissuer` its `main`; three dev tenants and five users (task 11) |
| §6.1 step 9, §9.1 | A6: the `id_token` checked without a signature under OIDC Core §3.1.3.7(6), and the tenant cross-check; `AfterSignIn` returns the Hub tenant id |
| §4.2, §6.3 | A7: `ErrKeySetUnavailable` → 503 `unavailable` |
| §9.1, §9.3 | A8: the three credential routes |
| §2 | A9: `hubapi` may import `httpx`; `hubclient` may import `httpx` and `ids` |
| §3.6 | A10: `teamId` in the audit envelope; `hub verify` checks `feed`; golden vectors 3 and 4 (task 02) |
| §9.1, §6.2 | A11: §9.1's refresh rows (`hub-contract.md:1172-1173`) become `PanaceaApp/entra/<tenantId>/<clientId>/refresh/<gen>/<i>` (`gen` 16 lowercase hex, one per write) and the account record gains `"refreshGen"`; §6.2's *"a crash in between leaves the old one"* gains the sequence that makes it true — new generation, then the record (last), then the old generation's delete; start-up deletes every generation the record does not name, validates the stored `refreshGen` and `refreshChunks` (1 to 16) and treats a bad record as a missing one, and deletes nothing on a read error that is not "not found"; sign-out deletes the record first (review I1, plan re-review b03 N1 and m3; tasks 28, 30, 31) |
| §6.2 | A12: *"Failure"* (`hub-contract.md:408-411`) is read by the scope redeemed. A refusal for `Hub.Access` keeps the contract's sign-out. A refusal for any other scope returns `ErrScopeNeedsConsent` or `ErrScopeRefused` (a `*ScopeRefusal`), deletes nothing, and drops only that scope's cached token. A plain `invalid_grant` on another scope is settled by one `Hub.Access` redemption. `invalid_scope` on another scope is `ErrScopeNeedsConsent`. Any other 4xx, on any scope, deletes nothing and leaves `Status` unchanged: only `invalid_grant` and `interaction_required` for `Hub.Access` prove the refresh token dead, so a misconfigured client never costs a sign-in. A `*ScopeRefusal` is remembered per scope until the refresh token's generation changes, `Invalidate`, or 15 min. The Azure DevOps `Store` returns both refusals, stopping the `Chain`. Neither refusal names an interactive step: v1 has no per-resource step-up (review I2; re-review b04 N1, m2, m4; task 31) |
| §6.2 | A13: *"while `exp − now > 5 min`"* (`hub-contract.md:397-398`) becomes *"while `exp − now > min(5 min, lifetime / 2)`"*, `lifetime` being `exp` minus the time the token response arrived (review I9; task 31) |
| §11.3, §12.2 | A14: the `pre-auth` row's key (`hub-contract.md:2091`) becomes *"the peer address; with `PANACEA_APP_HUB_TRUSTED_HOPS=N` ≥ 1, the N-th `X-Forwarded-For` entry from the right (all lines joined in order), falling back to the peer address when there are fewer than N entries or the entry is not an IP — never the left-most, which the client writes"*. §12.2's `PANACEA_APP_HUB_TRUST_PROXY` row (`:2165`) is replaced by `PANACEA_APP_HUB_TRUSTED_HOPS`, default `0`, *"integer 0–5: the proxies in front of the Hub that append to `X-Forwarded-For` (phase 8's gateway: `1`); a set `TRUST_PROXY` is refused"* (review I7; tasks 08, 13) |
| §5.1, §7.2, §6.5, §6.4 | A15: §5.1's *"`email`"* paragraph (`hub-contract.md:317-318`) keeps its rule but now says *"display text only; never used to authorise"*, and gains **`bindEmail`**: *"the `email` claim alone, lowercased, when the token's `acct` is the number `0` (a member) or its `xms_edov` is `true` (email domain owner verified); otherwise empty. Never `preferred_username`. An absent optional claim, or one of another JSON type, fails closed."* §7.2's binding (`:655-657`) matches `bindEmail`, not `email`; with an empty `bindEmail` nothing binds. §6.5's optional-claims row (`:542`) becomes *"access token: `email`, `acct`, `xms_edov`"*. §6.4's `devissuer` token claims (`:515`) gain `acct` (`0`, or `1` for a user marked `guest`) and `xms_edov` (the user's `emailVerified`), and its user schema (`:511`) the two fields (review I12; tasks 11, 13, 16, 43) |
| §10.2, §10.3, §10.7, §11.1 | A16: §10.2's schema (`hub-contract.md:1450`) takes task 07's keys: every foreign key except the one to `orgs` carries `org_id` (`FOREIGN KEY (org_id, team_id) REFERENCES teams (org_id, id)`, and so on), and each parent table gains `UNIQUE (org_id, id)` as the target; the digest-carrying target becomes `brief_revisions (org_id, brief_id, revision, digest)`, the revision target `(org_id, brief_id, revision)`, and the blob and design targets `project_blobs (org_id, project_id, digest)` and `design_approvals (org_id, project_id, design_key)`. §10.3 (`:1914`) gains a bullet: *"Postgres checks a foreign key without row-level security, so a single-column key would let an org-A row name an org-B parent. Every key carries `org_id`, and the database refuses a cross-org link (`23503`) with every Go check off."* §10.7 (`:2049`) gains the row *"a row never links another org's row \| every lookup under row-level security \| composite org-carrying foreign keys"*, and `:2044`'s target gains `org_id`. §11.1 T3's controls (`:2062`) gain *"composite org-carrying foreign keys (§10.2)"*, and *"the Go check disabled"* is defined: authorisation predicates answer "allowed"; lookups still run under row-level security (review I5; tasks 07, 16, 17) |
| §1, §7.5, §7.12, §10.6 | A17: §1's idempotency bullet (`hub-contract.md:50-55`) gains, after *"returns the stored status and body"*: *"except route 26, which stores its reply with every `content` null and, on a repeat, rebuilds each `content` and `erasedAt` from `brief_revisions`: the first answer byte for byte until an erasure, and after one the erased revision with `content: null`, as `/v1/refs` answers it"*. §7.5's revision write says the same in one sentence. §10.6's `UPDATE hub.idempotency_keys` line (`:2030`) gains the comment *"route 26: its reply with every content null"*. §7.12's list (`:1113`) gains the bullet *"stored idempotency responses are not changed: none holds brief text (§1), so a retried revision write still replays, with the erased revision's `content` null"*. §3.4's rule (`:170-171`, content served *"by parsing `C`, never from a second copy"*) now holds for replays too. §10.4's `erase_brief` (`:1969`) is **unchanged**: a purge of stored responses that name the brief also deleted the text-free replays of approvals, evidence, releases, co-batched briefs and the erase itself (review I6, re-review b06 N1; tasks 03, 07, 14, 19, 24, 27) |
| §9.1 | A8, the Azure DevOps half: the target `PanaceaApp/git/azure-devops/<org>` (a PAT per organisation, `credstore.LocalDevOps`, `Scheme: BasicPAT`), and phase 3's chain order `Local → LocalDevOps → entraauth.DevOps → GitHelper` (review I10; tasks 28, 29, 39) |
| §9.2 | additive: `Teams`, `Projects.Project`/`AddRemote`, `Designs.Approval`/`SessionLinks` (HA5-2: §9.2's `Designs` becomes the seven methods), `Releases.Release`, `ErrSignedOut`, `ErrNotConfigured`, `ErrTokenUnavailable`, `Invalidator`, `WriteError`, `VerifyRevision`, `FormatRecorder`. One sentence at the head of §9.2: *"Siblings consume these interfaces and `NewFake()`; they never implement them. A double is `NewFake()` or an embedded interface, so an added method is additive."* |
| §9.3 | additive: the `Panacea-Auth-State` header (task 36); the hub-link and team routes (task 35) |
| §6.2 | the refinement: redemptions serialised across scopes (task 31) |
| §7.9 | an overdue reservation reads `expired` before the janitor runs (task 23) |
| §15 | remove what this plan verified from the tree and documentation: `opener:default` permits `https` URLs; `x/sys/windows` has no Cred wrappers. (The pgx pin and the `go 1.23.0` directive stay listed until task 06 builds them: task 44b) |

**The sibling asks' answers (task 00), folded in the same way:**

| Section | Change | Ask |
|---|---|---|
| §7.5, §7.1 | `BriefWithRevision.latestComplete` on routes 21 and 22, its null rule | HA4-4 |
| §7.5 | the batch holds 1–64 revisions (`hubapi.MaxRevisionBatch`); route 26's body cap is 4 MiB of **wire** bytes (`hubapi.MaxRevisionBatchBytes`); every other route keeps 1 MiB. The wire rule, stated in §1 as well: a `content` travels as its canonical bytes, and no JSON body in either direction is HTML-escaped (`SetEscapeHTML(false)`; task 00, *Sizes*) | HA4-3 |
| §9.2 | `Briefs.List(ctx, projectID, hubapi.BriefQuery)`; `PutBlob(…, body io.ReadSeeker)` and the rewind rule; `JoinDeployment(ctx, releaseID, hubapi.DeploymentJoinRequest)`; `Error.RetryAfter`; `Relay(err) (status, httpx.Problem, ok)`; the 8 MiB reply cap and the wire rule it rests on | HA4-2, HA5-1, HA7-1, HA4-5 |
| §4.3 | `Relay` is the rule as a value; `WriteError` is `Relay` plus the write | HA4-5 |
| §7.9, §8 | `environment` nullable for a run without `sourceDeploymentId`; the join sets it once, 409 `conflict` on a different one; `release.deployment_joined` gains `data.environment`, the release's environment after the join (`""` while null); a rollback confirm is `{}`, and one naming its own source deployment is 409 `conflict`; `kind=` on the release list | HA7-1, HA7-2, HA7-5 |
| §9.3 | the `CursorSource` mechanism: one `Panacea-<Name>-Cursor` per source, names unique ignoring case, the expose list built from the names. The desktop half is a **declared difference** from HA7-4's ask: one `onHubHeaders(h)` hook, not `onCursor(name, value)`, which is not built. `h.cursors` is keyed by the **lowercased** name (`hub`, `deploy`), and each consumer compares its own entry with the last value it saw | HA7-4 |
| §7.7 | R5's *"audited with its session context"* is read as: the Hub records the principal and the time; no client-asserted field. The ADR's own text is the owner's (task 00) | HA5-4 |
| §9.3 | `Subscribe(kindPrefix, fn)` stands — `Subscribe("", fn)` is phase 4's shape — delivery in commit order | HA4-8 |
| §14 | a row per ask (HA4-1…HA4-9, HA5-1…HA5-5, HA7-1…HA7-5) naming the answer and the section that now holds it; HA7-3 and HA5-5 recorded as reserved | all |

**`PROGRAM.md`.** The identity row of *Decisions locked* (`PROGRAM.md:18`) gains: *"The control plane
runs the flow and holds the tokens; the Hub validates Entra access tokens itself — no broker
(ADR-H1 §5.2, §6.1)."* The Phase H paragraph (`:94-95`) gains one sentence naming the plan and its
seams: *"Plan: `docs/superpowers/plans/2026-09-23-phase-h-hub.md`; seams S-CRED (the credential
store phases 3, 6 and 7 use), S-TOKEN, S-HUB, S-LINK, S-FEED."* Nothing else in the file changes.

**UI contract §4.5.** A note: *"Superseded by the local caller key (Phase H plan, tasks 37–38):
every REST call carries `Authorization: Bearer <local key>`, and the attach WebSocket offers it as
its second subprotocol, `["panacea.v1", "panacea.key." + key]`, never in its URL. The per-sandbox
`attachToken` is unchanged and still never reaches the desktop."*

**UI contract §4.4 and §4.6** (plan re-review b02 m3; both still describe the synchronous,
keyless open that task 38 replaces):

- §4.4's row (`2026-09-18-phase-2-ui-contract.md:1169`) becomes
  `openAttach(session): Promise<WebSocket>`, with the note *"async since Phase H task 38: it
  reads the local caller key before the handshake"*.
- §4.6's paragraph (`:1219-1221`) becomes: *"`openAttach(session)` reads the local caller key
  afresh (`localKey.refresh()`), then calls `opts.webSocket(attachUrl(...), protocols)` —
  default `(url, protocols) => new WebSocket(url, protocols)` — with protocols
  `["panacea.v1", "panacea.key." + key]`, and sets `binaryType = "arraybuffer"` before
  resolving. The browser sends `Origin` on the handshake itself; the key rides only in the
  subprotocol, never in `attachUrl`, and the control plane echoes only `panacea.v1`. Without a
  key source (phase 2's own tests) the protocols are `[]`. (Phase H plan, tasks 37–38.)"*

These are edits in place, marked `(Phase H plan, task 38)`, not notes: §4.4 and §4.6 are the
signatures a builder copies, and a note beside a wrong signature still gets the old one copied.

**ADR-H1 is edited in exactly two cells, both A15's; nothing else in it changes** (re-review b02
m3; plan re-review b05 N1, N3). §5.4 still says *"every control-plane route except the WebSocket
attach requires `Authorization: Bearer <key>`"* (`ADR-H1-hub-architecture.md:451`). The ADR's
text is the owner's (as the HA5-4 row above says), so the plan does not rewrite it. That departure
is recorded where a reader of the plan meets it: the plan's coverage row for ADR-H1 §5.4
(plan.md:81), and the Hub contract §9.4 row (A3) in the table above.

A15's two cells are different, and are edited in place, because a note beside them does not
reach the reader who acts on them:

- **§9 item 3** (`:725`) is the list IT builds the Hub API registration from (*"so the whole ask
  goes to IT once"*, `:719`). *"Optional claim **`email`** on the access token"* becomes
  *"Optional claims **`email`**, **`acct`** and **`xms_edov`** on the access token — invites
  bind only a member (`acct` 0) or a verified email domain (`xms_edov` true) (amended by the
  Phase H plan, A15)"*. Unedited, IT configures `email` alone, Entra emits neither other claim,
  and no deployed invite binds, with no error.
- **§5.3's invite row** (`:406`): *"a matching `preferred_username` or `email`"* becomes *"a
  matching `email` claim, and only when the token shows a member account (`acct` 0) or a verified
  email domain (`xms_edov` true); never `preferred_username` (amended by the Phase H plan, A15)"*.
  The contract defers to the ADR where they disagree (`hub-contract.md:4-5`), so unedited, this
  cell would make the fallback A15 removes the governing rule.

Both keep ADR-H1's decision (§10 Q4: an email invite binds at first sign-in, with no Graph call);
they change only which claim may bind. The owner accepts or rejects them as Q12; they are made now
so that no one builds from the old text in the meantime.

**Provenance contract draft.** One block at the top, *"Amended by Phase H (ADR-H1 §7.2)"*, listing:
§2.1's store locations are now the Hub's; §1.4 and §4.1 gain the `erased` state; §6.4's *"head refs
are not polled"* is replaced by the change feed (tasks 26, 36, 40); §6.6's open exposure is closed
by the local caller key (tasks 37–38); A5 is widened to the whole machine (ADR-H1 §5.4). Each line
points at the Hub contract section that now holds the rule. The draft's body is not rewritten: its
owner phase does that.

#### Tests to write first

Documents have no unit tests; the checks are mechanical and are the acceptance below. The one
executable check, `TestTheContractsRepoKeyRegexIsTheCodes`, moved to **task 01** (`keys_test.go`),
because the code it compares against does not exist yet when this task runs; it passes there
because this task wrote A1 first.

#### Steps

- [ ] **Step 1:** Re-read ADR-H1 §7.2, this plan's corrections table, and task 00.
- [ ] **Step 2:** Make each change above.
- [ ] **Step 3:** Commit by pathspec — the four documents and ADR-H1 — on `feat/hub`, before any lane
  branches. Append a dated entry to `.superpowers/sdd/hub/progress.md` that records the commit's
  sha and these two read paths, and put the same three lines in the task report. The orchestrator
  sends the sibling phases exactly those lines. A sibling works in its own worktree on its own
  branch, and there the contract file on disk is still the **old** text, so a bare sha is not
  enough:
  - `git show <44a-sha>:docs/superpowers/contracts/2026-09-23-hub-contract.md`. This works from
    any worktree, because all of them share one object store. It is the path to use.
  - `C:/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/hub/docs/superpowers/contracts/2026-09-23-hub-contract.md`.
    Read it only while `feat/hub` is checked out at or after `<44a-sha>`.

#### Acceptance

```bash
C=docs/superpowers/contracts/2026-09-23-hub-contract.md
grep -c "(Phase H plan, A" $C                                     # ≥ 10
grep -c "(Phase H plan, task 00" $C                               # ≥ 9
grep -n "^## 16" $C                                               # present
grep -n "MaxRevisionBatch\|latestComplete\|CursorSource\|io.ReadSeeker" $C   # each present
grep -n "no broker" docs/PROGRAM.md                               # the identity row
grep -n "Amended by Phase H" docs/superpowers/planning/2026-09-23-phases-4-5-7/provenance-contract-draft.md   # present
A=docs/superpowers/adr/ADR-H1-hub-architecture.md
grep -c "amended by the Phase H plan, A15" $A                     # exactly 2
grep -n "preferred_username" $A                                   # no hit reads as a binding rule
grep -n "xms_edov" $A                                             # §5.3's row and §9 item 3
git diff --stat <parent-of-44a> <44a-sha> -- $A                   # the two cells only: 2 insertions(+), 2 deletions(-)
git log -1 --format=%H -- $C                                      # older than task 01's first commit
grep -n "git show .*:docs/superpowers/contracts/2026-09-23-hub-contract.md" .superpowers/sdd/hub/progress.md   # the read path, with the sha
```

#### What this task does not claim

- That sibling phases have re-read the amended contract: the orchestrator sends them the sha and
  the `git show` read path (step 3); a phase plan
  written against the old §7.4 regex or the old §12.1 compose needs its own correction.
- That the build matches this text: task 44b reconciles what the build changed.
- Any change to the ADR's decision: ADR-H1 stands. Its text moves in two cells only, A15's, which
  change which claim binds an invite and not the decision (§10 Q4); the owner accepts or rejects
  them as Q12. HA5-4's ADR wording is an owner question.
- That IT has configured `acct` and `xms_edov`: task 43's R-4 is the first real look.
