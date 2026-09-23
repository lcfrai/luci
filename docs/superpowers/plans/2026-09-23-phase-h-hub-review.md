# Phase H (Hub) plan — adversarial review

**Verdict: Needs fixes — 2 critical, 12 important, 15 minor.** The design is sound: layered
immutability, one write transaction, a matrix meta-test, a conformance suite shared by the fake and
the real Hub, and an honest unverified list. What fails is (a) the plan is **stale against its
consumers**: phases 4, 5 and 7 now have full plans with 20 named Hub asks, and several change the
interfaces S0 promises will not change; (b) the local caller key (task 37) has no answer for the
zero value, so it either fails open or turns every existing and sibling API test red; (c) a set of
auth-flow and tenancy defects that only show up when it runs against a real browser, a real
module graph, or a real Entra tenant.

**Reviewed:** `docs/superpowers/plans/2026-09-23-phase-h-hub.md` (484 lines) and its 44 parts in
`2026-09-23-phase-h-hub-parts/`, on `feat/hub` at `2f84662`, 2026-09-24. Read with it: the owner
decisions (OD-1…OD-13), the built tree at `3303fd6`, the module cache, and the sibling plans by
absolute path (phase 3 at `665083a`, phase 4 at `d307ec4`, phase 5 at `3526b3d`, phase 7 at
`d6e1989`, phase 8 at `8f38842`), all read-only.

**Method.** Every file, line, type and route this review cites was opened. The last section lists
which of the plan's citations were checked and which are wrong. Two things were **run**: a Python
recomputation of golden vectors 3 and 4 (task 02), and a listing of the Go module cache. Nothing
was compiled. Where a claim below rests on behaviour that was not observed — Chromium's CSP rules,
Entra's error codes — it says so.

---

## Coverage

### Against the owner decisions

| Decision | Covered? | Where / gap |
|---|---|---|
| OD-1 (build now; every plan adversarially reviewed; UI branches unmerged until the owner looks) | partly | Reviewed, yes. But OD-1's waiver names phases 3, 4, 5 and 7, **not H**. Tasks 41–42 build two surfaces with no prototype, and lane D merges into `feat/hub` with no rule that `feat/hub` waits for the owner's look (Minor M14) |
| OD-3 shared from day one | yes | tasks 13–27 |
| OD-4 Azure DevOps **and** GitHub | **partly** | GitHub: a pasted token (task 39). Azure DevOps: only through the Entra sign-in, and `PANACEA_APP_ENTRA_DEVOPS` defaults to **false** (task-32.md:84). No write path exists for an ADO PAT, so by default the store holds **no** ADO credential (Important I10) |
| OD-5 own server side, in this repo | yes | `cmd/hub` in the control-plane module |
| OD-6 Entra auth code + PKCE, tokens in Credential Manager, same store for ADO and GitHub | yes, with defects | tasks 28–32. Rotation is not crash-safe (I1). A consent or conditional-access refusal on a **secondary** resource signs the user out entirely (I2). ADO: see OD-4 |
| OD-8 transcript rule | n/a | the Hub sees no terminal byte (plan.md:67) |
| OD-11 permissive licences | yes | pgx v5.7.5 and its closure are MIT/BSD (module cache checked) |
| OD-12 owner steps batched to the end | yes | tasks 41, 42, 43 say *unverified by the owner* |
| OD-13 Panacea stays separate | yes | the `pnc_` token lives in the control plane's store (tasks 29, 39); the Hub holds none |

### Against the phase brief (ADR-H1) and the Hub contract

- Grafts G1–G6 are each mapped to a task (plan.md:71), and each mapping holds: G1 (task 23, the pin
  is taken in the reservation's transaction), G2 (task 21), G3 (tasks 14, 26), G4 (task 07),
  G5 (tasks 14, 27), G6 (task 27).
- The ten contract corrections A1–A10 are real defects of the contract. Where this review could
  check them against the tree, they hold (see Strengths).
- **The sibling asks are not covered.** The plan says phases 4, 5 and 7 "had only 6, 13 and 34
  lines of plan, so their asks of the Hub are not known" (plan.md:463-466). They now run to 517,
  511 and 715 lines, with nine asks from phase 4, five from phase 5 and five from phase 7. See
  Critical C1.

### Against the prototypes

- The rail-footer person is in every prototype (`prototype-fact-sheet.md:26`: *"Dylan Mackie — rail
  footer in Design and Deploy"*). Task 41 puts the signed-in name there. Covered.
- **No prototype shows the share surface**, team creation or invites. Task 42 designs a new surface
  from the design system alone (M14).

---

## Strengths

- **Corrections that check out.** A1: phase 3's `RepoKey` really is `azure-devops/<org>/<project>/<repo>`
  (phase-3 `task-13.md:136-139`), and phase 3's review I10 ruled that form correct (review `:350-356`).
  A3: health is `GET /v1/healthz` and `/api/v1/healthz`, mounted outside the guard
  (`api.go:303-306`). A4: `corsHeaders = "Content-Type"` (`origin.go:168-171`), and the preflight
  is answered before `next` (`origin.go:231-237`). Without A4, every request fails. A7: phase 8's
  `ErrKeySetUnavailable` is at `task-16.md:102-106`. A2's premise holds too:
  `images/sandbox/Dockerfile:3-6` says module downloads fail inside the image build.
- **Golden vectors.** Vectors 3 and 4 (task-02.md:187-198) were recomputed here: 271 bytes and hash
  `c6a78d78…fbaa`; 249 bytes and hash `bc782237…010c`. Both match the plan.
- **Immutability in layers.** No `UPDATE` grant is given to `hub_app` on revisions. `guard_revision`
  holds even for the owner. A `CHECK` ties every revision's digest to its bytes. Tests drop each
  layer on purpose and expect `hub verify` to catch the tampering (tasks 07, 20, 27). This fully
  answers "a revision never changes".
- **One transaction shape**, with the audit-head lock taken first (task-14.md:88-106). That one
  rule makes the feed gapless, makes idempotency claims visible only once committed, and removes
  deadlocks between writers.
- **The matrix meta-test** (task-16.md:103-106) walks `Server.Patterns()`, so a later route cannot
  skip its authorisation rows. **The conformance suite** holds the fake and the real Hub to one
  standard.
- **The `go` directive** was checked against the real module cache: pgx v5.7.5's `go.mod` says
  `go 1.23.0`, and so do x/crypto v0.37.0, x/text v0.24.0 and x/sync v0.13.0.
- **Honesty.** Every UI claim says *unverified by the owner*. The real-tenant items (R-1…R-10) wait
  for IT's registrations, and "nothing was compiled" comes first.

---

## Issues

### Critical

#### C1. The plan does not meet its consumers' Hub asks, and S0 would freeze interfaces they already reject

- **Where:** plan.md:318 (S0: *"Nothing below S0 changes an interface they code to"*),
  plan.md:463-466 (the sibling plans had only a few lines, so their asks are "not known"),
  task-04.md:79-100, task-03.md:297-299, task-07.md:270 and :455-463, task-36.md:68.
- **What is wrong:** the sibling plans now name their asks precisely. Checked against this plan:

  | Ask | What it needs | This plan | Kind of change |
  |---|---|---|---|
  | **HA4-2** (phase-4 plan:467) | `Briefs.List(ctx, projectID, BriefQuery) (Page[BriefHead], error)`; *"task 08 is blocked on it"* | `Briefs` has no list method (task-04.md:79-89), though route 19 exists | additive method |
  | **HA4-3** (:468) | revision batch **64**, route body cap 4 MiB, `hubapi.MaxRevisionBatch` | 1–16 (task-03.md:297-299; task-19.md:25; task-04.md:243) | a contract constant; fake, Hub, tests |
  | **HA4-4** (:469) | `latestComplete` beside `revision` in `ForBranch` | head revision only (task-03.md:280-283) | additive field |
  | **HA4-5** (:470) | `Relay(err) (status, Problem, ok)` | `WriteError(w, log, err)` only (task-33.md:84) | additive func |
  | **HA4-8** (:473) | `Subscribe(func(Event)) (cancel func())` | `Subscribe(kindPrefix, fn)` (task-36.md:62) | a signature difference; this plan's shape is the better one, so confirm it |
  | **HA4-9** (:474) | `BriefRevisionDigest(...)` | met in substance: `hubapi.BriefEnvelope{…}.Digest()` (task-02.md:118-130) is exported | confirm only |
  | **HA5-1** (phase-5 plan:454) | `PutBlob(…, body io.ReadSeeker)`, because a retry cannot replay a consumed reader | `body io.Reader` (task-04.md:93), and task 33 retries every call twice | **parameter type** |
  | **HA7-1** (phase-7 plan:540-563) | `environment` may be `null` at reserve and is set by the deployment join; *"blocks the first Ship of a never-deployed branch"* | `environment text NOT NULL CHECK (…)` (task-07.md:270), and `environment` is in `guard_release`'s immutable tuple (task-07.md:455-463) | **schema + trigger + types** (`string` → `*string`) |
  | **HA7-4** (:586-611) | a generic `CursorSource` list and one `Panacea-<Name>-Cursor` header per source | one hard-coded header and expose list (task-36.md:68, :74-83) | design of task 36 |
  | HA7-2, HA7-5, HA5-4 | a sentence of the contract; a `kind` filter; the R5 phrase | not addressed | text, or additive |

- **Why it matters:** S0 (tasks 01–05) is the first thing the siblings code against, tonight.
  HA7-1 contradicts task 07's migration and its guard. Once `0001_init.sql` has been applied
  anywhere, that becomes a `0002` that rewrites a trigger function. HA5-1 and HA7-1 change a
  parameter type and a field type, which breaks S0's own promise. HA4-2 blocks phase 4's task 08
  outright. The plan's premise that later asks "should be additive" (plan.md:466) is false for
  three of them.
- **How to fix:** before task 01, add a part named "S0 absorbs the sibling asks" and edit tasks 03,
  04, 05, 07, 19, 23, 24, 33 and 36. Build HA4-2, HA4-4, HA4-5 and HA5-1 as asked. Decide HA4-3's
  number (64 costs one constant and a body cap). Build HA7-1: make `environment` nullable, add
  `CHECK (environment IS NOT NULL OR state = 'reserved')`, and let `guard_release` allow exactly
  `environment: NULL → value` on a reserved release. Generalise task 36's header as HA7-4 asks.
  Replace plan.md:463-466 with a table of every ask and its answer. Task 44 then folds the answers
  into the contract.

#### C2. Task 37's local key has no defined zero value, so it either fails open or turns every API test red — this plan's and four siblings'

- **Where:** task-37.md:19-24 (files), :70 (the wrap), :99-103 (steps). `api.New`'s required-deps
  table is at `control-plane/internal/api/api.go:213-231`.
- **What is wrong:** `Deps` gains `LocalKey`, and the wrap is unconditional. The part never says
  what a zero `localkey.Key` means. Five existing test files build the API
  (`internal/api/api_test.go`, `attach_test.go`, `catalog_test.go`, `routes_audit_test.go`,
  `sessions_test.go`), and none sends a key. Phases 3, 4, 5 and 7 are adding handler tests to the
  same package tonight. None of those files is in task 37's list.
- **Why it matters:** there are only two outcomes. (a) A zero key refuses everything: task 37's own
  acceptance (`go test ./control-plane/internal/api/`) fails on the first run, and merging 37 turns
  every sibling phase's API suite red. (b) The implementer "fixes" it by treating a zero key as
  *no check*: that fails open, and a wiring slip in `main.go` removes the key in production with
  every test still green. The plan names the desktop half of this hazard (37 and 38 must land
  together) but not the Go half.
- **How to fix:**
  1. Fail closed: `api.New` returns `"api: Deps.LocalKey is zero"`, beside the other required deps.
  2. Add `localkey.ForTest()` (a fixed key) and one `export_test.go` helper that adds the header to
     a request.
  3. Put the five files in task 37's list, each changed only by that helper.
  4. Tell the orchestrator that every sibling's API tests need the same one-line change when 37
     merges.
  5. Add `TestTheProcessRefusesToStartWithoutAKey` to `cmd/api/main_test.go`.

### Important

#### I1. Refresh-token rotation is not crash-safe: the chunks are overwritten in place

- **Where:** task-28.md:100-104 (`PutChunked` writes `prefix+"0"`, `prefix+"1"`, … in place);
  task-31.md:39-42 (*"If that write fails, Token returns the error and the old refresh token stays
  in the store"*); task-30.md:110; task-31.md:107 (`TestAFailedRotationWriteKeepsTheOldToken`).
- **What is wrong:** the account record (*"written last, so it marks a complete write"*) protects
  only the **first** sign-in. On rotation the record already exists. Chunk 0 is overwritten with
  the new token, and if chunk 1's `Put` then fails, or the process dies, the store holds new chunk 0
  followed by old chunk 1 behind a valid record. That is neither token. The test passes only if
  the fault is injected on the **first** `Put`, and a KPMG-sized token is three chunks (R1).
- **Why it matters:** the contract's promise, *"a crash in between leaves the old one"* (§6.2 as
  quoted), is broken exactly where it matters: large enterprise tokens. The user is signed out with
  `reauth_required` and no cause anyone can find.
- **How to fix:** give each write its own generation. Chunks go to `…/refresh/<gen>/<i>`; the
  account record carries `refreshGen` and `refreshChunks` and is written last, pointing at the new
  generation. Only then are the old generation's chunks deleted. Start-up deletes any generation
  the record does not name. Change the test to fail the **second** chunk's `Put` of a 6,000-byte
  token, and assert that the old token reads back whole.

#### I2. A consent or conditional-access refusal on a **secondary** resource signs the user out of everything

- **Where:** task-31.md:43-45 (*"invalid_grant or interaction_required → both entries deleted,
  every cached token dropped, Status {signed_out, reauth_required}"*); task-43.md:63 (R-8).
- **What is wrong:** one refresh token serves the Hub, phase 8's broker and, if Q5 is yes, Azure
  DevOps. A refusal for **one resource** is not a dead refresh token. As I understand Entra's
  behaviour (not observed here), an unconsented resource answers `invalid_grant` with
  `suberror: consent_required` (AADSTS65001), and a conditional-access policy scoped to one
  resource answers `interaction_required` (AADSTS50076/50079) for that resource only. Under
  task 31's rule, either refusal deletes the refresh token that still works for the Hub.
- **Why it matters:** enterprises often put a conditional-access policy on Azure DevOps. With
  Q5 = yes, the first push would sign the user out of the Hub. Signing back in asks only for
  `Hub.Access`, so the next push signs them out again: a loop. The same happens to the broker
  scope whenever it is not consented.
- **How to fix:** only a refusal while redeeming for `HubScope` means *reauth required*. For any
  other scope, return a scope-specific error (`ErrScopeNeedsConsent` or
  `ErrScopeNeedsInteraction`), keep the refresh token, and let the caller surface it. `DevOps`
  maps these to a stopping error, not `ErrNoCredential`, per phase 3's `Chain` rule. Optionally
  add the broker and ADO scopes to the authorize request so consent is asked once (the token
  endpoint still answers for the first resource; `devissuer` rule 5 already allows several API
  scopes, task-12.md:44). Add `TestASecondaryScopeRefusalKeepsTheSession`.

#### I3. The attach WebSocket stays exempt from the key, and it is the one route that types into a terminal

- **Where:** task-37.md:53-58 (`Exempt`), :119 (*"The attach WebSocket carrying the key … later
  work"*); task-38.md:8-9. Session ids are 32 bits (`control-plane/internal/ids/ids.go:17-26`).
- **What is wrong:** task 37 exists because *"a sandboxed agent that can reach the host's
  loopback"* must not drive the control plane (task-37.md:5-10). The defences kept on attach —
  loopback bind, the Host guard, the Origin allowlist — stop browsers. A non-browser process sets
  `Host` and `Origin` to whatever it likes. So the exempt route lets such a process open another
  session's PTY and type into it, once it has a session id. The only barrier is guessing 32 bits.
  The "cannot carry a header" premise is also wrong: a browser WebSocket can carry a secret in its
  subprotocol list (`new WebSocket(url, [...])` sets `Sec-WebSocket-Protocol`), which the part
  itself names as the later fix.
- **Why it matters:** the key protects every route **except** the most powerful one, against the
  exact threat it names.
- **How to fix:** do it now — about an hour on each side. The desktop opens
  `new WebSocket(url, ["panacea.v1", "panacea.key." + key])`. The control plane checks the second
  entry with `subtle.ConstantTimeCompare` and echoes only `panacea.v1`. `Exempt` shrinks to the
  two health routes. `TestAttachIsExempt` becomes `TestAttachNeedsTheKeyInItsSubprotocol`.
  Phase 2's `openAttach` gains the key source. If deferred, record it in the plan's unverified
  list as an accepted risk.

#### I4. `devissuer`'s page forbids the redirect it exists to perform

- **Where:** task-12.md:49-52 — the page's CSP is `form-action 'self'`, each user button POSTs to
  the same path, and the POST answers `302` to `http://127.0.0.1:<listener port>/?code=…`.
- **What is wrong:** the listener's origin (a different port) is not `'self'`. As I understand it,
  Chromium — and so Edge, the corporate system browser — applies `form-action` to redirects that
  follow a form submission (not observed here). The click would be refused with a console error,
  and the tab would not reach the listener.
- **Why it matters:** every automated test uses a scripted non-browser client (task-30.md:116-119),
  so all of them pass. The first real click — task 12 step 4, then tasks 41, 42 and 43 — fails.
  That is the whole local-first sign-in.
- **How to fix:** drop `form-action`, or allow `form-action 'self' http://127.0.0.1:* http://localhost:*`.
  Or make each user a plain link: a `GET` with a one-time, page-scoped nonce. Add a check in step 4:
  *"in Edge, clicking Grace lands on the listener URL"*.

#### I5. Cross-org references are enforced by Go alone; the single-column foreign keys let an org-A row point at an org-B parent

- **Where:** task-07.md:95-96 (`team_members.team_id`, `user_id`), :108 (`team_invites.team_id`),
  :127 (`projects.team_id`), :148 (`briefs.project_id`), :171-173 (`brief_revisions.brief_id`,
  `project_id`), :233-234 (the approval→revision FK), :259-260, :274, :302-304, :326, :337;
  task-16.md:128 (`TestTheXColumnHoldsWithoutGoChecks`), :135-136 (`disableGoAuthz`).
- **What is wrong:** row-level security checks only the row's **own** `org_id`
  (task-07.md:498-500). Postgres checks foreign keys without row-level security. So a `hub_app`
  insert whose `org_id` is the caller's org, but whose `team_id`, `brief_id` or `pin_brief_id`
  belongs to another org, succeeds. Task 16's X-column test runs with the Go checks switched off.
  What `TeamRole` answers in that mode is not specified. If it answers "allowed", then
  `POST /v1/projects {teamId: <org 1's team>}` from Xavier inserts a project in org 2 pointing at
  org 1's team, and the test gets 201, not 404. Either the test goes red and exposes the hole, or
  the switch is defined so it hides it.
- **Why it matters:** the plan's claim for the database layer — *"one org never sees another's
  rows"* — holds for reads but not for links. Tenancy is the one line the plan says holds even
  with every Go check off (§5.3, G4).
- **How to fix:** add `UNIQUE (org_id, id)` to each parent table. Make every foreign key composite
  and org-carrying:
  - `FOREIGN KEY (org_id, team_id) REFERENCES teams (org_id, id)`, and the same for users,
    projects and briefs.
  - Add `org_id` to the `brief_revisions (brief_id, revision, digest)` target.

  Define `disableGoAuthz` as "authorisation predicates answer allowed; lookups still run under
  RLS". Add a DB test that inserts one cross-org reference per foreign key and expects `23503`.

#### I6. Erasure leaves the brief's text in `idempotency_keys.response` for up to 24 hours

- **Where:** task-07.md:539-554 (`erase_brief` touches only `brief_revisions` and `briefs`);
  task-14.md:100 (the 2xx body is stored); task-03.md:187 and :301-302 (`WriteRevisionsResponse`
  carries every revision's `Content`); task-24.md:53 and :73 (idempotency keys kept 24 h).
- **What is wrong:** each `POST …/brief-revisions` response is stored whole, content included, and
  `erase_brief` never touches that table.
- **Why it matters:** G6's promise — erased means no text, for the case where a client's matter
  number must go — fails for a day, and a backup taken that day keeps the text for the backup's
  whole life.
- **How to fix:** `erase_brief` also runs
  `DELETE FROM idempotency_keys WHERE org_id = v_org AND path = '/v1/projects/' || <project> || '/brief-revisions'`,
  or revision-write responses are stored without `content` and rebuilt on replay. Extend
  `TestErasureKeepsKeyAndDigest` (task 27): after erasure, no `text` or `bytea` column of any
  table holds a marker string from the erased content.

#### I7. The per-IP rate limit keys on the left-most `X-Forwarded-For` entry, which the client writes

- **Where:** task-13.md:56 (*"take the client address from X-Forwarded-For's first hop"*), :144
  (`TestForwardedForOnlyWhenTrusted` pins `203.0.113.9` from `203.0.113.9, 10.0.0.1`).
- **What is wrong:** an ingress **appends** to `X-Forwarded-For`, so the left-most entry is
  whatever the client sent. A fresh made-up value on each request gets a fresh bucket. The test's
  own mutant (*"a spoofed header that resets anyone's bucket"*) survives with `TrustProxy` on.
- **Why it matters:** the per-IP limit runs **before** token validation to bound the cost of bad
  tokens (task-13.md:141). Behind phase 8's ingress, that bound disappears.
- **How to fix:** take the entry the trusted proxy appended — the **right-most** one, or the N-th
  from the right for N trusted hops (`PANACEA_APP_HUB_TRUSTED_HOPS`, default 1). Change the test:
  `X-Forwarded-For: 198.51.100.7, 203.0.113.9` from the ingress keys on `203.0.113.9`, and a
  changing left-most value does not reset the bucket.

#### I8. pgx moves `x/sys` to v0.32.0; task 28 then asks for v0.24.0, which would downgrade pgx; the "it is cached" premise is also false

- **Where:** task-06.md:183-186 (*"the module is in the cache"*); task-28.md:25 (*"`golang.org/x/sys`
  becomes direct at **v0.24.0**, the version in the graph"*), :174-175 (`go get golang.org/x/sys@v0.24.0`);
  plan.md:45-47, :96-100.
- **What is wrong (module cache read here):** pgx v5.7.5 requires x/crypto v0.37.0, x/text v0.24.0
  and x/sync v0.13.0. x/crypto v0.37.0 requires **x/sys v0.32.0** and **x/term v0.31.0**. The
  control-plane module holds x/sys v0.24.0, x/term v0.21.0 and x/text v0.16.0
  (`control-plane/go.mod:54-57`). So after task 06, the graph selects x/sys ≥ v0.32.0. The
  download cache has neither `x/sys@v0.32.0` nor `x/term@v0.31.0`: it has x/sys v0.24.0, v0.39.0,
  … and x/term v0.21.0 only.
- **Why it matters:** task 06's `go get` needs the network for two modules, behind the proxy.
  Task 28's literal `go get golang.org/x/sys@v0.24.0` then **downgrades** x/crypto, and with it
  pgx, below v5.7.5. Task 28's own diff check would show the damage, but the instruction causes
  it. The x/sys "no Credential Manager wrappers" check was also run on a version that will not be
  built. (It still holds: v0.39.0 and v0.47.0 were grepped here and have none either.)
- **How to fix:**
  - Task 06: say that x/sys, x/term, x/text, x/crypto and x/sync move with pgx, and that `go get`
    may need the network for x/sys v0.32.0 and x/term v0.31.0.
  - Task 28: make x/sys direct **at whatever version the graph already holds**. `go mod tidy`
    after the first import does that; never name a version.
  - Update plan.md:45-47.

#### I9. The end-to-end test cannot pass as written: a 90-second access token is always inside the five-minute refresh window

- **Where:** task-43.md:19 (`devissuer … AccessTTL 90 s`), :36 (step 9: *"devissuer saw **one**
  refresh grant"*); task-31.md:34 (*"serves the cached access token for scope while exp − now > 5 min"*).
- **What is wrong:** with a 90 s lifetime, `exp − now` is never above five minutes, so every
  `Token` call redeems and rotates. Steps 1–8 make dozens of refresh grants before step 9 counts
  "one". The test also mixes clocks: step 5's *"within 3 s"* is wall-clock, while step 9
  "advances past" the token's life.
- **Why it matters:** this is the plan's only proof of *the thread*. It will fail for a reason that
  is not a product defect, and the likely "fix" — loosening the assertion — hides a real problem:
  any lifetime under five minutes means a refresh on every call.
- **How to fix:** make the skew `min(5 min, lifetime/2)` and test that rule in task 31. Give the
  e2e test an `AccessTTL` of 10 minutes and **one injected clock** shared by devissuer, entraauth
  and the Hub's validator. Advance that clock by 10 minutes in step 9. Keep step 5's latency
  assertion on wall time, with the clock frozen.

#### I10. OD-4 and OD-6 promise one store that holds the Azure DevOps token; by default it holds none

- **Where:** task-39.md:26-31 (`kind` ∈ `github`, `panacea` only); task-32.md:84
  (`PANACEA_APP_ENTRA_DEVOPS` default `false`); task-31.md:75-84 (`DevOps` answers
  `ErrNoCredential` when disabled); phase 3's `Credential` already has
  `BasicPAT … an Azure DevOps personal access token` (task-29.md:60).
- **What is wrong:** until KPMG IT grants the desktop client Azure DevOps consent, which is
  unscheduled, the store has no ADO credential and no way to be given one. Phase 3's push and PR
  on Azure DevOps fall through to the machine's git credential helper.
- **Why it matters:** OD-4 is *"both, from v1"*, and OD-6 names the same store for both. Phase 3
  is building its ADO path tonight against a store that will be empty.
- **How to fix:** add `kind: azure-devops` to task 39: scope = the organisation
  (`^[a-z0-9-]{1,50}$`), a PAT of 52 base32 characters (or a generous bound), stored as
  `PanaceaApp/git/azure-devops/<org>` and served by a `credstore.LocalDevOps` with
  `Scheme: BasicPAT`. Order the chain `Local → LocalDevOps → entraauth.DevOps → GitHelper`.
  When Q5 flips, the Entra path takes over with no write-path change.

#### I11. The ten contract corrections are folded in **last**, though the plan says siblings must read them first

- **Where:** plan.md:137-143, :371 (task 44 depends on 43), :391 (`feat/hub`: `43 → 44`);
  task-44.md:6-8 (*"must be folded into the contract **before** another phase reads it and builds
  against the old text — the failure this program has paid for a dozen times"*).
- **What is wrong:** the part states the ordering rule, and the plan's schedule breaks it. Phases 4
  and 7 already flag A1's repo-key disagreement (phase-4 plan:471, HA4-6; phase-7 plan:624-628),
  and every sibling reads contract §12.1's four-service compose, §9.4's missing `Authorization`
  CORS header and §7.4's regex tonight.
- **Why it matters:** the whole build window runs against a contract its own plan calls wrong in
  ten places.
- **How to fix:** split task 44. **44a** — A1–A10, the additions table and C1's answers, a
  1.5-hour documents-only task — runs **first**, before task 01, on `feat/hub`, and the
  orchestrator announces it to the siblings. **44b** — whatever the build changed afterwards —
  stays last.

#### I12. Team membership is granted by the `email` claim, which Microsoft says not to use for authorisation

- **Where:** task-13.md:35 (*"Email: the email claim, else preferred_username, lowercased"*);
  task-16.md:28-31 (binding); phase 8's `Claims.Key()` comment (*"never preferred_username
  (mutable)"*, phase-8 `task-16.md:108-110`).
- **What is wrong:** an invite binds to whoever presents a matching `email`, or failing that a
  matching `preferred_username`. Both are mutable, and in a tenant with B2B guests neither is
  verified by the tenant. Binding stays inside the org (`TestBindingStaysInTheOrg`), which rules
  out the cross-tenant case. It does not rule out a guest or a renamed account in the same tenant,
  or the multi-tenant customer tier (seam S-BYO).
- **Why it matters:** membership is the only authorisation inside an org (T4, a single-layer
  control), and an invite is a grant.
- **How to fix:** bind only a principal whose token shows a member account (`acct == 0`) or a
  verified email (`xms_edov == true`, an optional claim — add it to R-4). Never fall back to
  `preferred_username` for **binding**; it may still be display text. `devissuer` emits both
  claims, so every column of the matrix is still reachable locally. Add
  `TestAGuestCannotBindAMembersInvite`.

### Minor

| # | Where | What is wrong | Fix |
|---|---|---|---|
| M1 | plan.md:127; plan.md:152; task-38.md:97; task-04.md:39 | Wrong citations. `problem.go:60-62` is the end of the code block; the "standard library only" rule is `problem.go:5`, with imports at `:10-17`. `problem.go:115` is inside `DecodeJSON`'s comment; `backend_unavailable` is at `:60`. `RailFooter.tsx:74-76` does not exist: the file has 25 lines, and the error line is `:16-21`. Task 04 says "the boundary test in task 08"; it is task 09 (plan.md:229) | Correct the four references |
| M2 | task-12.md:120; task-11.md:198; task-36.md:104 | Task 12 counts 25 passing tests in `./internal/devissuer` alone, but `TestLoopbackOnlyBind` tests the `ADDR` rule that `cmd/devissuer` reads (task-11.md:180-184), so it lives there, and the count is 24. `TestMoreIsDrainedWithinATick` mixes two scenarios: 450 events at 200 a page is three pages, and the "eleventh page" belongs to a separate over-10-pages case | Fix the count; split the test into `…WithinATick` and `…AtMostTenPagesATick` |
| M3 | plan.md:374-377 | "About six working days for two engineers" divides 96 h by two. The server lane to S7 is serial: tasks 01–08 and 13–24 add up to 57 h on one engineer, with task 10 blocking 13. That is seven or more working days | State the critical path (lane A to S7 ≈ 57 h) rather than the average |
| M4 | task-07.md:3; task-16.md:3; task-43.md:3 | Undersized. Task 07 is a ~330-line schema plus 17 database tests, debugged against a live Postgres, in 3 h. Task 16 is three routes, binding, and a six-principal matrix harness with a per-cell fixture, in 3 h. Task 43 is a 12-step end-to-end test plus re-running 42 acceptance blocks, in 3 h | Split 07 into 07a (tables, grants, RLS) and 07b (guards, owner functions and their tests). Split 16 into 16a (invites) and 16b (the harness). Estimate 43 at 5 h |
| M5 | plan.md:172-175; task-29.md:15-17, :96-97; phase-3 `task-13.md:68` | "Verbatim, so the later merge is a no-op" is untrue twice. Task 29's copy of `credstore.go` changes phase 3's `Source` comment (`"hub" \| "git-credential-helper"` becomes `"credential-manager" \| "entra" \| …`). And neither part holds the method bodies, so two implementers write two bodies at one path: an add/add conflict | Say so, and name who wins (the owner phase), in the file-sharing table. Send phase 3 a one-line ask to adopt the `Source` values |
| M6 | task-42.md:60; task-32.md:8-10 | *Share with a team…* is added to the project menu unconditionally, which contradicts "with `PANACEA_APP_HUB_URL` unset, nothing changes for the user" | Hide the item unless `auth.hubConfigured`; add `the menu item is absent without a Hub` to task 42's tests |
| M7 | task-38.md:71-73; `desktop/src/api/client.ts:40-48`; task-42.md:89 | `ProblemCode` gains only the control plane's four new codes. The relayed Hub codes (`repo_already_linked`, `revision_conflict`, `team_not_found`, `forbidden`, …) pass at runtime (`client.ts:199-206`), but task 42's test builds `new ApiError("repo_already_linked", …)`, which fails `npm run typecheck` | Add the Hub's §4.2 codes to the union in task 38, or type the relay's codes as `HubProblemCode` and widen `ApiError.code` |
| M8 | task-31.md:63-66; task-32.md:81 | `New` finds the account record by the configured authority's tenant with **no network**. `PANACEA_APP_ENTRA_AUTHORITY` may name a domain (`…/kpmg.com.au`) or `organizations`, and then no tid is known | Require the authority's path to be a tenant GUID (a config error otherwise), or have `New` use `List("PanaceaApp/entra/")` filtered by client id |
| M9 | task-37.md:41-46, :87 | On Windows the key file only inherits its directory's ACL. Under `PANACEA_APP_STATE_DIR=C:\somewhere`, that ACL may admit `Users`. `TestTheKeyFileIsPrivate` skips Windows | Set an explicit owner-only DACL (`windows.SetNamedSecurityInfo`), or refuse a state directory whose ACL admits `Users`; add the Windows test |
| M10 | task-07.md:493-496, :563 | `orgs` has no row-level security, and `hub_app` has `SELECT` on every table, so any query path can read every enrolled tenant's id and name | Add `orgs` to the RLS loop with `USING (id = current_setting('hub.org_id', true))`; `ensure_org`/`resolve_principal` are `SECURITY DEFINER` and unaffected |
| M11 | task-07.md:476-487, :580 | `audit_append` runs as the invoker, so `hub_app` needs `UPDATE (seq, hash)` on `audit_heads`, and a compromised Hub can move the head directly (a gap that `hub verify` finds, but only later) | Make `audit_append` `SECURITY DEFINER`, revoke `hub_app`'s `UPDATE` on `audit_heads`, and add it to `TestHubAppGrantsAreExactlyTheList` |
| M12 | task-27.md:106-111 | Erasure's "does not claim" omits storage: the `UPDATE … canonical = NULL` leaves dead tuples until vacuum, and the text stays in WAL and in every backup taken before it | Add the sentence; name the backup retention phase 8 sets as the real erasure horizon |
| M13 | `control-plane/internal/api/api.go:9-13`; `origin.go:164-167`; `routes_audit_test.go:25-27` | Three phase 2 comments say there is no authentication or `Authorization` on this hop "before Phase 8". Task 37 makes each false, and none is in its file list (only `client.ts:6-8` is, task-38.md:79) | Add the three comment edits to task 37 |
| M14 | owner-decisions.md:12; tasks 41, 42; plan.md:391 | OD-1 waives prototype-before-plan for phases 3, 4, 5 and 7 only. Phase H's two surfaces have no prototype, and nothing keeps `feat/hub` (with lane D merged) from moving on before the owner looks | Record it as owner question Q11 (recommendation: a static prototype of the sign-in states and the share popover before task 41), and state that `feat/hub` is not merged onward until the owner's look (OD-1) |
| M15 | plan.md:340, :357, :361; task-13.md:148; task-30.md:37, :134; task-31.md:19; task-11.md:127-130 | Undeclared dependencies. Task 13's `TestADevissuerTokenPassesTheRealValidator` needs task 11; task 30 needs task 04 (`hubclient.ErrSignedOut`, `ErrUnavailable`); task 34's conformance needs task 11's `MintAccessToken`. Task 31, in lane C, edits lane A's `hubclient.go` | Add 11 to 13's and 34's dependencies and 04 to 30's; move the `Invalidator` declaration into task 04 |

---

## The two defects a first implementer is most likely to hit

1. **Every API test goes 401 the moment task 37 lands (C2).** The first `go test
   ./control-plane/internal/api/` after the wrap goes in fails across `api_test.go`,
   `attach_test.go`, `catalog_test.go`, `routes_audit_test.go` and `sessions_test.go`, because none
   sends a key, and none is in task 37's file list. **Does the plan prevent it? No.** It names the
   desktop half ("land 37 and 38 together") but gives no zero-value rule, no test helper and no
   file list, so the likely "quick fix" is a zero key that disables the check — fail-open.
2. **The module graph moves under the pins (I8).** Task 06's `go get …pgx/v5@v5.7.5` needs
   `x/sys v0.32.0` and `x/term v0.31.0`, neither of which is cached. Once it succeeds, task 28's
   `go get golang.org/x/sys@v0.24.0` would downgrade x/crypto and pgx. **Does the plan prevent it?
   Partly.** Both tasks check `git diff control-plane/go.mod`, which shows the damage, but task 28's
   instruction causes it and task 06's "it is in the cache" premise is false.

Runner-up: **I4**, the `devissuer` page's `form-action 'self'`. It is invisible to every automated
test and would stop the first real click in Edge at task 12's step 4.

---

## Citations checked

**Correct** (opened in this worktree unless stated):
- `control-plane/go.mod:1`, `:3`, `:54`, `:57`; `go.work`; `azure-pipelines.yml:6`, `:11`, `:14`;
  `images/sandbox/Dockerfile:3-6`.
- `cmd/api/config.go:18-51`, `:54`, `:63-82`; `internal/api/api.go:15-21`, `:213-231`, `:292`,
  `:303-306`; `attach.go:42`; `httpx/origin.go:97-116`, `:168-171`, `:192-198`, `:231-237`.
- `statepath/statepath.go:30-45`; `routes_audit_test.go:32`; `project/project.go:29`;
  `internal/api/export_test.go:21` (`RegisteredPatterns`).
- `desktop/src/api/client.ts:6-8`, `:242`, `:199-206`; `poller.ts:1-3`, `:48`, `:74-76`;
  `StatusDot.tsx:2`; `ContextMenu.tsx:44`, `:98-110`; `tauri.conf.json` (`"csp": null`);
  `capabilities/default.json`; `lib.rs` (only `greet`).
- `styles/tokens.css:111-135` (`--r-ctl`, `--r-card`, `--e-btn`, `--e-card`, `--ease`).
- The four `classify` call sites (`EmptyDevelop.tsx:57`, `SessionPane.tsx:48`, `sessionVerbs.ts:85`,
  `useCreateSession.ts:38`); `prototype-fact-sheet.md:26`.
- In sibling worktrees: phase-3 `task-13.md:136-139`; phase-3 review `:350-356`; phase-8
  `task-16.md:102-106`.
- In the module cache: pgx v5.7.5's `go 1.23.0`.

**Wrong:**
- plan.md:127 (`problem.go:60-62`) and plan.md:152 (`problem.go:115`).
- task-38.md:97 (`RailFooter.tsx:74-76`); task-04.md:39 (task 08 → 09).
- task-28.md:25 (x/sys v0.24.0 "the version in the graph" after task 06).
- task-06.md:184 ("the module is in the cache": two of its requirements are not).
- plan.md:463-466 (the sibling plans' sizes, now 517/511/715 lines).

**Not checked by this review:**
- Contract and ADR section contents beyond the passages the plan quotes.
- The phase 7 brief's `:119`.
- Every Entra behaviour (R-1…R-10), Chromium's `form-action` behaviour on redirects (I4), and
  Entra's error codes for per-resource refusals (I2).

---

## Assessment

The plan is careful, well-cut into lanes, and unusually honest about what it has not verified.
Its server design — RLS plus grants plus triggers, one lock order, and a hash-chained audit log
that doubles as the change feed — is right for the owner's decisions. It runs without Azure:
Postgres in Docker, the Hub and `devissuer` on the host. It does not yet survive contact with its
consumers or its own first runs.

**Fix before any code, in this order:**
1. **C1** — absorb the sibling asks into S0 (tasks 03, 04, 05, 07, 19, 23, 33, 36), especially
   HA7-1's schema change and HA5-1's type change. About 3 h of plan edits.
2. **I11** — move the contract fold-in (task 44a) to the front. About 1.5 h.
3. **C2** — define `Deps.LocalKey` as fail-closed and give task 37 its test helper and file list.
   About 30 min.
4. **I1, I2, I4, I9** — the auth-flow fixes (rotation generations, scope-specific refusals, the
   page's CSP, the e2e clock). About 1 h of plan edits.
5. **I5, I6, I7, I12** — tenancy and privacy (composite foreign keys, erasure of stored responses,
   right-most `X-Forwarded-For`, invite binding). About 1 h.
6. **I3, I8, I10** and the Minor table as the parts are touched.

**Plan quality: Needs fixes**

---

## Fix batch b01 — C1 and I11 (2026-09-24)

Paths are relative to `docs/superpowers/plans/`; `plan.md` is `2026-09-23-phase-h-hub.md`, `parts/`
is `2026-09-23-phase-h-hub-parts/`. Line numbers are as committed by this batch. The task list
**must be re-derived**: it gains 00 and 44a, and 44 is now 44b (`task-44.md` was renamed).

### C1 — the sibling asks absorbed into S0

The asks were re-read from the sibling worktrees on 2026-09-24 (phase 4 plan `:475-483`, phase 5
`:458-462`, phase 7 `:540-616`); the review's line numbers had moved by 8–9 lines, the ask ids had
not.

| Ask | Change | Where |
|---|---|---|
| — | the new part *"S0 absorbs the sibling asks"*, before task 01: every ask, its answer, the task that builds it | `parts/task-00.md` (all); plan.md:334 (task row), :323 (S0 row: after 44a, 00, 01–05; the reopening rule) |
| — | plan.md:463-466 replaced by the ask/answer table | plan.md:477-502 |
| HA4-1 | confirmed: task 03's structs, frozen at S0 | `parts/task-00.md` table |
| HA4-2 | `BriefQuery` (with `Page PageRequest`, a declared difference from the ask's flat fields); `Briefs.List`; the fake's rule and test; the conformance row; the HTTP method | parts/task-03.md:285; task-04.md:82, :249-253, :281; task-05.md:202; task-34.md:28 |
| HA4-3 | `MaxRevisionBatch` 64, `MaxRevisionBatchBytes` 4 MiB; route 26 through `WriteLimit`; the fake; ~~`json.Compact` in the client~~ replaced in round 2 by the wire rule (R1 below) | task-03.md:313; task-14.md:112, :184; task-19.md:6, :23-27, :87; task-04.md:256, :280; task-05.md:203; task-34.md:31 |
| HA4-3 (exposed) | `hubclient`'s JSON reply cap 1 MiB → **8 MiB**, derived from `/v1/refs` (100 × 64 KiB) and route 26's reply (64 × 64 KiB). At 1 MiB a **committed** 16-revision batch was already reported `ErrUnavailable` — a defect the review did not name | task-33.md:63, :139 |
| HA4-4 | `BriefWithRevision.LatestComplete`; routes 21 and 22; tests (the with-revisions test sits in task 19, where revisions are written) | task-03.md:280; task-18.md:24-25, :70; task-19.md:92; task-04.md:282 |
| HA4-5 | `Relay`; `WriteError` = `Relay` + write; `Error.RetryAfter` (without it `WriteError`'s *"Retry-After copied"* had nothing to copy) | task-33.md:98, :121, :141; task-04.md:145 |
| HA4-8, HA4-9, HA4-7, HA5-3 | confirmed, no change; `Subscribe("", fn)` is phase 4's shape, delivery in commit order | task-36.md:59-65; `parts/task-00.md` table |
| HA5-1 | `PutBlob(…, io.ReadSeeker)`; the transport seeks to 0 before every attempt; a failed `Seek` ends the call | task-04.md:97; task-05.md:129, :204; task-33.md:54, :140; task-34.md:33, :86 |
| HA5-2, HA5-4 | 44a writes `Designs`' seven methods; R5 read as principal and time — ADR wording is owner question **Q11** | task-44a.md §9.2 row and answers table; plan.md:456 |
| HA7-1 | nullable `environment`; ReserveRequest/Release `*string`; `DeploymentJoinRequest.Environment`; `JoinDeployment` takes the request; schema, CHECK, guard, grant, four database tests; reserve and join routes and tests | task-03.md:443, :470, :503; task-04.md:112; task-05.md:151-158, :198-199; task-07.md:270, :302, :471-487, :595, :630-633; task-23.md:26, :79; task-24.md:52-64, :123-124 |
| HA7-2 | a rollback confirm naming its own source deployment → 409 `conflict` | task-24.md:24-30, :122; task-05.md:200 |
| HA7-3 | reserved, seam S-DEPLOY-TARGET | plan.md:438 |
| HA7-4 | `CursorSource` in `Deps.CursorSources`; the expose list built from the names by the `api` wrap; `origin.go` untouched by task 36; the desktop's `HubHeaders.cursors` map | task-36.md:17, :78-83, :138-141, :157; task-40.md:77, :107, :159; plan.md:406, :428-430 |
| HA7-5 | `ReleaseQuery.Kind`; `kind=` on route 36 | task-03.md:508; task-23.md:46-48; task-05.md:201; task-34.md:36 |

**Two corrections to the review's *how to fix*, on the facts.** (1) The review's HA7-1 CHECK,
`environment IS NOT NULL OR state = 'reserved'`, would refuse to confirm, abandon or expire a
null-environment run (each leaves `reserved` before any join) — a 500 on every refused first Ship.
Built instead: `CHECK (environment IS NOT NULL OR (kind = 'run' AND source_deployment_id IS NULL))`.
(2) The review let `guard_release` allow NULL → value *"on a reserved release"*; route 40 joins
**confirmed** releases only (task-24.md:46-47), so that allowance would never fire. Built instead:
NULL → value once, while `reserved` or `confirmed`. Both are argued in `parts/task-00.md`
*"HA7-1, exactly"*. Because the guard now compares explicit column tuples rather than `NEW IS
DISTINCT FROM OLD`, `TestReleaseGuardCoversEveryColumn` (task-07.md:633) pins the tuples to the
table and to `hub_app`'s column grant.

### I11 — the contract fold-in split

| Change | Where |
|---|---|
| **44a**: A1–A10, the additions table and task 00's answers; documents only; runs first on `feat/hub`, before task 01 and before the lanes branch; the orchestrator announces it | `parts/task-44a.md` (new); plan.md:333, :391, :402 |
| **44b**: only what the build changed (`(Phase H build, task NN)`), §15's build-verified items, the seams' landing commits; stays last | `parts/task-44b.md` (was `task-44.md`); plan.md:378, :402 |
| the contract-vs-code repo-key test moves to task 01, which creates `keys_test.go` after 44a wrote A1; task 01 depends on 44a | task-01.md:365; plan.md:335 |
| the corrections and additions tables say 44a folds them first | plan.md:142-145, :160-162 |
| restatements of "task 44" | task-02.md:243-245 (44a), task-07.md:649 (44b) |
| totals: 46 tasks; ≈ 119 h; minimum ≈ 98 h with 44a and 00 | plan.md:316-319, :380-386 |

Unverified: the sibling plans' line numbers will move again as those plans are edited; nothing was
compiled or run (documents only).

### Fix batch b01, round 2 — the re-review's open issues (2026-09-24)

This round answers `.superpowers/sdd/hub/plan-rereview-b01-1.md`, which is local and git-ignored.
Line numbers are as committed by this round. The task list is unchanged from round 1.

| Issue | Change | Where |
|---|---|---|
| **R1** (Important) | Took the re-review's option (a): name the encoder on both sides, so the caps stay 4 MiB and 8 MiB as HA4-3 asked. Task 00 now states **the wire rule**. Its evidence was read from go1.26.2's source on this machine: `encoding/json/encode.go:487` compacts a marshaller's output through `appendCompact(…, opts.escapeHTML)`, and `indent.go:57-71` escapes `<`, `>`, `&`, U+2028 and U+2029 only when that flag is set. (1) `hubclient` encodes each JSON request once, with `SetEscapeHTML(false)`, never `json.Marshal`. (2) `WriteRevisions` sends each `content` as `hubapi.Canonical`, not `json.Compact`, which kept a writer's `é` escapes. (3) The Hub's `server.WriteJSON` encodes without HTML escaping and no longer calls the shared `httpx.WriteJSON`, which is untouched. That is safe because every Hub response carries `application/json` and `nosniff`. "Guarantees" is gone. Every test the re-review named now uses `<>&`/U+2028-dense content, and task 19's test asserts the 201 reply is under 8 MiB | task-00.md:28 (HA4-3 row), :100-101, :103-125; task-08.md:212-221, :280; task-33.md:51-55, :143-144; task-34.md:31, :82; task-19.md:87; task-03.md:311-314; task-44a.md:51-52; plan.md:489 |
| **m1** | task 01's header depends on 44a | task-01.md:3-4 |
| **m2** | `release.deployment_joined`'s `data.environment` is the release's environment **after** the join, as task 24 already said. Task 00 and 44a now say so too, and the `{d-3}` step asserts `"feat-x"`. A join naming none on a null release asserts `""` | task-00.md:89-91; task-44a.md:54; task-24.md:123 |
| **m3** | HA7-4's desktop half is recorded as a **declared difference**: one `onHubHeaders(h)` hook, `onCursor` not built, `h.cursors` keyed by the lowercased name (`deploy`). Phase 7 adds no second hook. Also found in passing: HTTP header names ignore case, so `Hub` and `HUB` are one header and one key. `api.New` now refuses a case-insensitive duplicate, and the test says so | task-00.md:43, :127-151; task-40.md:162-165; task-44a.md:55; task-36.md:79, :85-88, :141; plan.md:432, :503 |
| **m4** | The 44a announcement carries the sha and the read path `git show <44a-sha>:docs/superpowers/contracts/2026-09-23-hub-contract.md`. It works from a sibling's own worktree because the worktrees share one object store. 44a step 3 records the sha and the path in `progress.md`, and the acceptance greps for them. 44b's reopened-ask notice does the same | plan.md:390-393; task-44a.md:3-4, :87-96, :109, :114-115; task-00.md:168-170; task-44b.md:22-24 |
| **m5** | `TestReleaseGuardCoversEveryColumn` reads the installed function through `pg_get_functiondef('hub.guard_release'::regproc)`. It expects exactly two `NEW…IS DISTINCT FROM OLD…` pairs with matching lists, and requires every `information_schema.columns` column exactly once across immutable, mutable and `{environment}`. It keeps the grant check | task-07.md:633 |
| **m6** | *"Siblings consume these interfaces and `NewFake()`; they never implement them"* is now in task 00, the S0 and S-HUB rows, and 44a's §9.2 row. I checked it by grepping the phase 3, 4, 5 and 7 plans for a type, a `var _` assertion or a stub that implements a `hubclient` interface. None exists | task-00.md:12-16; plan.md:323, :430; task-44a.md:40 |
| **m7** | route 40's *"confirmed releases only"* cite corrected to task-24.md:46-47; round 1's HA4-3 row marks `json.Compact` as superseded by R1 | this file :513, :495 |

Unverified: nothing was compiled or run (documents only). The encoder behaviour was read from the
Go source, not executed. A sibling agent's use of `git show` was not tried from a sibling worktree.
