# 11b. Fact sheet: the Hub as contracted and as built, for two new record kinds

**For:** the designer of design round 2 (`10-decision-terms-beside-the-brief.md`: a project
**term** and a project **decision**, each its own Hub record referencing a brief revision).
**What this is:** facts, each with a `file:line`. It designs nothing and recommends nothing.
**Written:** 2026-09-29, against the trees named below at the commits named below.

## What could not be verified

(Filled in as the sections are written.)

## Trees read, and at which commit

(Filled in as the sections are written.)

## 1. The contract

`HC` below is `.claude/worktrees/hub/docs/superpowers/contracts/2026-09-23-hub-contract.md`
(2,385 lines). **The main tree has no copy:** `docs/superpowers/contracts/` on
`feat/phase-2-cockpit` holds only the two phase 2 contracts, and `git ls-files` finds no
`hub-contract` anywhere in the main tree. Every `HC:` line below is the Hub worktree's file.

### 1.0 Its standing

- It implements `docs/superpowers/adr/ADR-H1-hub-architecture.md`; "where the two disagree, **the
  ADR's decision wins and this file is wrong**" (`HC:3-5`).
- "Nothing in this file was compiled or run, except the two digest vectors in §3.5" (`HC:6-7`).
- Additive changes are not breaking: "a new optional request field, a new response field, a new
  route, a new error code, a new event kind". Consumers **must** ignore unknown response fields and
  unknown event kinds; the Hub **rejects** unknown request fields, 400 `bad_request` (`HC:32-35`).
- The control plane must send an `Idempotency-Key` "on every reserve, confirm, approval and
  revision write" (`HC:50-55`).

### 1.1 The key grammar, and who owns it

- The whole grammar is two forms (`HC:119-122`):
  `brief key ^brief:(bf-[0-9a-f]{16})@([1-9][0-9]{0,8})$` and
  `design key ^design:([a-z]{2,4}-[0-9a-f]{4,32})@([1-9][0-9]{0,8})$`. No other key form is written.
- **Ownership, as the contract states it:** "The key grammar belongs to phase 4's `provenance`
  package (provenance contract §1.2, P5). These are the forms **the Hub mints and accepts**; the
  `provenance` parser must accept at least these" (`HC:116-117`), under the heading "Key grammar —
  normative for the Hub" (`HC:114`).
- **The source the contract cites** is the provenance contract draft
  (`.claude/worktrees/phase-3/docs/superpowers/planning/2026-09-23-phases-4-5-7/provenance-contract-draft.md`,
  `PC` below). `PC:104-107`: "The key grammar is **owned by the `provenance` package** (phase 4). A
  `kind:` prefix is part of the grammar so that ledger links can name other subjects in the same
  form. `design:` is **reserved** for phase 5 … No other kind is defined by this draft." Its
  prohibition **P5** is "Parsing a brief key outside the `provenance` package … One grammar, one
  owner" (`PC:415`).
- **Ownership, as `10-decision-terms-beside-the-brief.md:54` states it:** "The provenance key
  grammar, which the **Hub contract** owns". `CONTEXT.md:108-110` says phase 4's `key.go:23-29` is
  "verbatim from the Hub contract". The contract's own sentence (`HC:116`) and `10`'s sentence name
  different owners. This sheet records both and decides neither.
- **Two parsers compile the grammar today, one in each tree:**
  - Phase 4, `.claude/worktrees/phase-4/control-plane/internal/provenance/key.go`: "Package
    provenance owns the key grammar (contract P5): nothing else in the product may split a key"
    (`:1-2`); brief and design "are the Hub's, verbatim (Hub contract §3.2, hub-contract.md:120-121 at
    hub da8c7da); none is local to the control plane and never reaches the Hub (P4-C)" (`:23-25`);
    three kinds `brief`, `design`, `none` (`:17-21`); "The three forms, compiled once. They are the
    grammar's only copy" (`:47-52`).
  - The Hub, `.claude/worktrees/hub/control-plane/internal/hub/hubapi/keys.go`: `KeyKind` has two
    values, `brief` and `design` (`:47-52`); the same two regexes (`:56-59`); `ParseKey` returns
    `ErrInvalidKey` for anything else (`:95-104`). So `key.go:47`'s "only copy" is not true of the
    product; it is true of phase 4's tree, which does not contain `hubapi`.
- A new key form would therefore reach both files, `key.go` and `hubapi/keys.go`, and the resolver's
  `invalid` answer for anything outside the grammar (`HC:836`).
- Revision numbers are "the Hub's, assigned by the revision write (§7.5), from 1, gapless per
  brief"; revision 0 "never appears in a key" (`HC:124-125`).
- The design key's id is **minted by phase 5 in the control plane**, stored verbatim by the Hub
  (`HC:126-128`); its revision is phase 5's and "the Hub never interprets it beyond the grammar"
  (`HC:129`). So the contract already has one key whose id the Hub does not mint.

### 1.2 Ids, and one collision a designer will meet

- Hub ids are `ids.Wide(prefix)`: prefix + `-` + 16 lowercase hex (`HC:93-101`); opaque to callers,
  "compare them, never parse them" (`HC:46-47`).
- The prefix table (`HC:103-109`) is: `org-` org, `u-` user, `tm-` team, `iv-` invite, `pj-`
  project, `bf-` brief, `ap-` design approval, `ln-` session → design link, `rl-` release, `ev-`
  evidence record. Disjoint from the control plane's `s-`, `p-`, `b-`, `w-`, `m-` (`HC:111`).
- As built, the prefixes are constants in `hubapi/keys.go:16-27`; `PrefixTeam = "tm"` (`:19`).
  `ValidID` accepts prefix + `-` + exactly 16 lowercase hex (`:29-35`).
- **`tm-` is already the team prefix** (`HC:107`). The illustrative key in
  `10-decision-terms-beside-the-brief.md:14` is `term:tm-7f2a@2`, which reuses it, and has 4 hex
  where `ids.Wide` mints 16.

### 1.3 The routes: a closed table

- §7.0 is "The whole table", 45 rows (`HC:557-607`), each naming its method, path, minimum role,
  **task** (H0 to H7) and section. "Nothing else is routed. An unknown path is 404
  `route_not_found`" (`HC:609-610`).
- Brief rows 19-27 are task H3 (`HC:581-589`); design rows 28-34 H4 (`HC:590-596`); release rows
  35-40 H5 (`HC:597-602`); evidence rows 41-42 H6 (`HC:603-604`); changes, audit and erase 43-45
  H7 (`HC:605-607`). Row 27, `GET /v1/refs?key=…`, is split: "H3 (brief), H4 (design)" (`HC:589`).
- §7.1 fixes the shared response objects; "Fields are always present; a field that has no value is
  `null` … or `[]` … never omitted" (`HC:614-615`). `BriefHead` and `BriefRevision` are defined there
  (`HC:641-650`).

### 1.4 The brief and its revisions (§7.5): the worked example of a kind

- **Division of labour:** phase 4's domain logic stays in the control plane; the Hub "stores heads
  and immutable revisions, numbers them, digests them, and resolves keys. It reads exactly one field
  of a revision's content: `status`" (`HC:742-745`).
- **`content`** is phase 4's object; the Hub requires a JSON object, `status` one of `open`,
  `complete`, `skipped`, integers only, depth at most 16, canonical envelope at most **64 KiB**;
  "Anything else inside is phase 4's to define and may grow additively" (`HC:747-753`).
- **Head and revisions are two things.** `POST /v1/projects/{projectId}/briefs` makes a head with
  `headRevision: 0`; one live project brief per project and one live branch brief per
  `(project, branchRef)`, else 409 `brief_exists`; event `brief.created` (`HC:755-761`).
  `PATCH` rebinds a branch brief after a rename, writes no revision, event `brief.rebound`
  (`HC:780-783`). Archive touches no revision, "every key still resolves", event `brief.archived`
  (`HC:785-788`).
- **The only write of brief text** is `POST /v1/projects/{projectId}/brief-revisions`: 1-16 items,
  one transaction, all or nothing; each item's `baseRevision` must equal the head's `headRevision`,
  else 409 `revision_conflict`; "the Hub never merges"; the new revision is `baseRevision + 1`; one
  `brief.revised` event per revision (`HC:798-815`).
- **The resolver** `GET /v1/refs?key=…` takes 1-100 keys, "`brief:` and `design:` mixed"
  (`HC:817-819`). Each result carries exactly `key`, `kind` (`brief`, `design` or `null`),
  `resolution`, `revision`, `approval` (`HC:822-828`). `resolution` is one of `resolved`, `missing`,
  `erased`, `invalid`; `invalid` is "not a key in §3.2's grammar" (`HC:831-836`). `heads` covers
  every brief named by a resolved or erased brief key and by every resolved design approval's
  `briefKey` (`HC:838-840`). `erased` "is a new provenance state … an amendment phase 4 makes to
  `Ref.resolution`" (`HC:841-842`).

### 1.5 The other record kinds that reference a brief revision

Every one of these carries a **brief key it was checked against**; none holds brief text.

| Kind (id prefix) | Route and section | How it references the brief | Mutability |
|---|---|---|---|
| Design approval (`ap-`) | `POST …/design-approvals`, §7.7 (`HC:872-924`) | `briefKey` + `briefDigest` in the body (`HC:882-883`). Checks in order: grammar (400); the key names a non-erased revision **in this project** (422 `key_unresolvable`); the digest matches (422 `digest_mismatch`); that revision's `status` is `complete` (422 `brief_incomplete`); blobs present; no approval yet for the design key (409) (`HC:887-897`) | "The row is immutable; there is no revocation in v1. A re-review approves a new design revision, which is a new key" (`HC:899-900`). An approval against an older complete revision "is **allowed**: it is shown as `superseded`, not refused" (`HC:915-916`) |
| Session link (`ln-`) | `POST …/session-links`, §7.8 (`HC:926-955`) | `briefKey` is the key the control plane stamped into `contextRef`; must resolve in this project, else 422; "It may differ from the approval's brief key; that difference is the point of recording both" (`HC:937-940`) | A repeat of the same `(designKey, sessionId)` by the same user returns 200 and the existing link (`HC:943-944`) |
| Release (`rl-`) | `POST …/releases`, §7.9 (`HC:957-1058`) | Not sent: the Hub **takes** the pin in the reservation's transaction, "then never again" (`HC:1011`), choosing brief then **latest complete revision** (`HC:1015`); `pin` holds `briefKey`, `briefDigest`, `headRevisionAtShip` (`HC:983-986`) | A state machine: `reserved`, `confirmed`, `abandoned`, `expired` (`HC:978`, `HC:1033-1038`) |
| Evidence (`ev-`) | `POST /v1/evidence`, §7.10 (`HC:1060-1087`) | `briefKey` "must resolve in that project … It is stored verbatim" (`HC:1071`), plus `criterionId` and `criterionDigest` (`HC:1065`, `HC:1072`) | "Append-only; no edit, no delete" (`HC:1083`) |

- **Erasure is per brief and touches only revisions.** `hub.erase_brief` removes every revision's
  content and sets `erased_at`; "key, digest, revision number, `savedAt` and `savedBy` stay"; the
  head is archived; "approvals, releases, evidence and session links are **not** changed: none of
  them holds brief text" (`HC:1115-1127`).
- **The feed** `GET /v1/changes` is the org's gapless audit sequence (`HC:1091-1100`); a caller sees an
  event whose `projectId` it can read or whose `teamId` it is in (`HC:1103-1104`); an event is
  written in the same transaction as its change, under the org's audit lock (`HC:1105-1107`).

### 1.6 Events (§8)

- "Every write in §7 appends **exactly one** event per changed record, in the write's transaction";
  `data` never holds brief content, blob bytes, evidence `because` text or a token (`HC:1133-1134`).
- The table has 26 rows naming 29 kinds (`HC:1137-1164`). The brief-bearing rows:
  `brief.created`, `brief.revised` `{"key","revision","digest","status"}`, `brief.rebound`,
  `brief.archived`, `brief.erased` (`HC:1148-1152`); `design.approved` `{"designKey","briefKey"}`
  (`HC:1153`); `session_link.created` (`HC:1154`); `release.reserved`
  `{"kind","environment","pinStatus","briefKey"}` (`HC:1155`); `evidence.recorded`
  `{"briefKey","criterionId","verdict","commit"}` (`HC:1160`).
- `data` values are strings, integers or booleans, so §3.6's encoding applies without floats
  (`HC:1166-1167`). Consumers ignore kinds they do not know (`HC:1135`).

### 1.7 The canonical form and the golden vectors

- A brief revision's digest is `"sha256:" + hex(SHA-256(C))` over a canonical **envelope**
  `{v, kind: "brief-revision", briefId, revision, projectId, scope, branchRef, content}`
  (`HC:141-155`). The encoding is RFC 8785 (JCS) restricted: sorted keys, no duplicates, no
  whitespace; defined string escapes; **integers only**, a fraction or exponent is 400 (`HC:157-168`).
- `C` is **stored** byte for byte in `brief_revisions.canonical`; "The Hub serves `content` by parsing
  `C`, never from a second copy"; one function `hubapi.Canonical(v any) ([]byte, error)`; "**No
  TypeScript implementation exists or may be written**" (`HC:170-174`).
- §3.5's two vectors are "checked into `hubapi/canonical_test.go`" (`HC:176`): vector 1, 209 bytes,
  a branch-scoped revision 1 (`HC:181-186`); vector 2, 256 bytes, exercising escapes, `-0`, 2⁵³−1,
  `null`, non-ASCII (`HC:188-195`). "A Go implementation passes when it reproduces both byte strings
  and both digests exactly" (`HC:197`). Both vectors are `kind: "brief-revision"`; there is no vector
  for any other kind.
- §3.6 "The other canonical records" lists exactly **two** envelope kinds: `brief-revision` and
  `audit-event`; the audit event is hash-chained,
  `hash = SHA-256(prev_hash ‖ canonical)` (`HC:199-206`). **No other record kind is canonically
  encoded or digested by the Hub**: approvals, links, releases and evidence carry the brief's digest
  (`briefDigest`, `pin.briefDigest`) or a caller's digest (`reviewDigest`, `criterionDigest`), not one
  of their own (`HC:883`, `HC:912-914`, `HC:985`, `HC:1065`).
- **As built (task 02), the code differs from §3.5 in two ways** (`hubapi/` below is
  `.claude/worktrees/hub/control-plane/internal/hub/hubapi/`):
  - The vectors are in `hubapi/envelope_test.go`, not `canonical_test.go`: vector 1 and 2 as the
    contract gives them (`envelope_test.go:20-30`, `:62-77`), **plus vectors 3 and 4**, two
    `audit-event` envelopes "plan correction A10's, with teamId" (`:17-18`, `:32-42`, `:92-116`).
  - The audit envelope carries `teamId` (`envelope_test.go:35`), which `HC:206`'s field list does
    not name. The ledger says §10 of the contract is stale on A17-A19 until task 44a (`PR:472`); the
    same applies to A10 here.
- The two envelope kinds are constants in code, `kindBrief = "brief-revision"` and
  `kindAudit = "audit-event"` (`hubapi/envelope.go:50-51`), each with its own type
  (`BriefEnvelope` `:129`, `AuditEnvelope` `:266`) and parser (`ParseBriefEnvelope` `:198`,
  `ParseAuditEnvelope` `:337`).
- An event kind has one grammar, `^[a-z]+(_[a-z]+)*\.[a-z]+(_[a-z]+)*$` (`hubapi/envelope.go:285`),
  described as `audit_events.kind`'s CHECK (`:278`): exactly one dot, lowercase letters, single `_`
  between words, **no digits** (`PR:288`). It is a format, not a list: the database does not
  enumerate event kinds.

### 1.8 The client every phase codes against (§9.2)

- `hubapi` holds "one Go struct per JSON object in §7 … with the JSON tags of §7 exactly"
  (`HC:1274-1277`). `hubclient` "is split by consumer so each phase depends on the smallest
  interface" (`HC:1277-1278`): `Projects`, `Briefs` (phase 4), `Designs` (phase 5), `Releases`
  (phase 7), `Evidence` (phase 3), `Feed`, composed as `Hub` (`HC:1287-1329`).
- `Briefs.Resolve` is commented "brief: and design: keys; batches of 100" (`HC:1301`).
- The client "**re-computes the digest of every brief revision it receives** with `hubapi.Canonical`
  — a mismatch turns that result into `missing`" (`HC:1348-1350`).
- `NewFake()` implements every observable §7 rule, and `hubclient/conformance` runs one suite
  against the fake and a real Hub; "each Hub task H2–H7 makes its rows pass against the real
  server" (`HC:1352-1358`).
- The desktop invalidates caches **by event-kind prefix**: `brief.*` → `headRevisionByBrief` and
  `headRefByBranch`; `release.*` and `design.*` → its release and approval lists (`HC:1382-1385`).
- "Materialise and `contextRef` are unchanged in meaning. The control plane fetches the head through
  `Briefs.ForBranch`, renders it, and stamps the key it rendered" (`HC:1387-1389`).

### 1.9 How a record kind is added: what the contract says, and where each kind appears

- **The contract has no procedure for adding a record kind.** A search of `HC` for *new kind*,
  *new record*, *additive*, *new prefix* finds only §1's additive-change rule (`HC:32-35`) and
  phase 5's v2 canvas note (`HC:128`, `HC:2279`). What follows is where the **brief** (the fullest
  existing kind) appears, section by section, as the worked example.

| Where | The brief's entry | Line |
|---|---|---|
| Id prefix | `bf-` | `HC:105` |
| Key form | `brief key ^brief:(bf-…)@(…)$` | `HC:120` |
| Canonical envelope | `kind: "brief-revision"` | `HC:141-155`, `HC:205` |
| Golden vectors | vectors 1 and 2 | `HC:181-197` |
| Routes, with task | rows 19-27, H3 | `HC:581-589` |
| Shared objects | `BriefHead`, `BriefRevision` | `HC:641-650` |
| Resolver slot | `kind: "brief"`, field `revision` | `HC:823-827` |
| Events | `brief.created` … `brief.erased` | `HC:1148-1152` |
| Authorisation matrix row | "create a brief; write revisions; rebind; archive" | `HC:351` |
| `hubclient` interface | `Briefs` | `HC:1293-1302` |
| Desktop invalidation | `brief.*` | `HC:1383` |
| Task acceptance | **H3** | `HC:2250` |

- The design approval, the lightest brief-referencing kind, has: prefix `ap-` (`HC:106`); a key
  whose id is minted **outside** the Hub (`HC:126-128`); rows 30-32, H4 (`HC:592-594`); a slot in
  the resolver as `kind: "design"` with field `approval` (`HC:826`); one event (`HC:1153`); a matrix
  row (`HC:352`); `Designs.Approve` (`HC:1307`); acceptance H4 (`HC:2251`); and **no** canonical
  envelope of its own (`HC:203-206`).
- §5.3's matrix "runs every row as every column" (`HC:335`) and the matrix test covers "every route
  in §7.0" (`HC:2260-2262`), so a new route is a new matrix row by construction.
- §15 "Not verified by this contract": nothing compiled or run except the vectors; the DDL "has not
  been applied to a Postgres" (`HC:2287-2289`); "Phase 4's content shape is read from its plan …
  not from code" (`HC:2294-2295`).

## 2. The Hub plan

`HP` below is `.claude/worktrees/hub/docs/superpowers/plans/2026-09-23-phase-h-hub.md` (683 lines);
its parts are `2026-09-23-phase-h-hub-parts/task-NN.md` (48 files: 00-43 with 07, 16 and 44 each
split into a and b). `PR` is the stream ledger `.claude/worktrees/hub/.superpowers/sdd/hub/progress.md`
(523 lines).

### 2.1 The task table

- The table is `HP:396-447`; columns are `# | Task | Contract id | Depends on | Hours | Min?`
  (`HP:398`). **`Min?` is not a build status**: ✓ marks a task inside "the minimum Hub", and `S7`
  marks the credential pair phase 7 needs (`HP:383-386`). No column of the table records what is
  built.
- Each task has its part file and is sized for 1-3 hours, with six named exceptions (`HP:368-377`).
- Four start lines: **S0** (code against the fake) after 44a, 00, 01-05; **S4** phase 4 integrates
  after 06-20 server-side; **S5** phase 5 after S4 + 21-22; **S7** phase 7 after S5 + 23-24, 29, 39;
  and S3 phase 3's shared evidence after S4 + 25, 29, 31, 39 (`HP:388-394`). S0's promise: "a later
  change that is not an added method, field or route reopens task 00 and is announced. An added
  method is additive only because siblings consume these interfaces and `NewFake()` and never
  implement them" (`HP:390`).
- Totals ≈ 133.5 engineer-hours, "5.5 h over ADR-H1's 16 engineer-days" (`HP:449-460`).

### 2.2 What is built, per the ledger

The Hub worktree's HEAD is `cafe1303` on `feat/hub`. At the time of reading it also held three
**uncommitted** modifications: `control-plane/internal/hub/hubclient/conformance/feed.go`,
`control-plane/internal/hub/migrate/schema_integration_test.go` and
`docs/superpowers/plans/2026-09-23-phase-h-hub-parts/task-07a.md` (`git status --porcelain`,
2026-09-29). `PR:493` names two uncommitted files; the third (`feed.go`) is newer than the ledger.

| Task | Ledger state | Where |
|---|---|---|
| 01 `ids.Wide`, `hubapi` codes, keys, digests | built; **approved** | `PR:239`, `PR:254`, `PR:494` |
| 02 `hubapi.Canonical`, envelopes, `ValidateContent`, golden vectors | built; **approved** | `PR:273`, `PR:280`, `PR:494-495` |
| 03 `hubapi` wire types | built; **approved** | `PR:298`, `PR:312`, `PR:495` |
| 04 `hubclient` interfaces; the fake, part 1 | built; approved only by a review "written outside the stream" | `PR:329`, `PR:366`, `PR:495-498` |
| 05 the fake, part 2; the conformance suite (**S0**) | built; **not approved**, "CARRIED under lite"; its Important I1 (the feed's team branch) "STILL NOT CARRIED ANYWHERE" | `PR:402`, `PR:417`, `PR:499-504` |
| 06 migrate runner, `roles.sql`, `hubtest` | built; **approved** | `PR:431`, `PR:447`, `PR:495` |
| 07a + 07b `sql/0001_init.sql` | built (`bab30e90`, `97254f75`, `86332241`, `7ea11387`); fix round 1 code committed `67ef06ab`; **round not closed**: four mutants unresolved | `PR:469-471`, `PR:485`, `PR:505-508` |
| 44a the contract made true before anyone builds | **unrun**, "out of order": the plan marks it first | `PR:518-521`, `HP:400` |
| 00 the sibling asks | "documents only, absorbed by 44a, and was never dispatched" | `PR:522-523` |

- "Remaining after 07: 44a, then 08 through 44b — 38 ids" (`PR:522`).
- Consequence the ledger states for any reader of the contract: "`hub-contract.md` on feat/hub
  therefore still carries the old repo-key regexes"; "The contract's §10 is stale on A17-A19 until
  task 44a" (`PR:519-520`, `PR:472`).
- Built Go packages today: `control-plane/internal/hub/{hubapi,hubclient,hubtest,migrate}`
  (directory listing, 2026-09-29). No `server/`, `store/` or `cmd/hub` exists yet, so **no route of
  §7.0 is served by a Hub process**; the only implementation of every route is `hubclient.NewFake()`.

### 2.3 How the plan changes the contract, and what that means for line numbers

- The plan found "**nineteen** places where the contract, as written, would fail when built"; "none
  adds a Hub route"; they are folded into the contract "by **task 44a, which runs first**"
  (`HP:164-170`). The rows are `HP:180-198` (A1-A19). 44a has not run (§2.2).
- "Every `hub-contract.md:<n>` in this plan and its parts is the contract at `97f74c7`. Task 01 …
  added nine lines to §7.4 at `caf18ae`, so a cited line past 716 is now 9 further down the file"
  (`HP:172-176`). Since `97f74c7` the contract has had seven commits, `caf18ae3` through `5313a337`,
  119 lines in and 23 out (`git log` and `git diff --stat 97f74c7 cafe1303` on the file), several of
  them OD-30 edits to §6, §9.1 and §17. So "add 9" is the plan's rule for task 01's shift only.
  **Every `HC:` line in this sheet was read from the file as it stands at `cafe1303`**, not derived.

### 2.4 The unbuilt tasks that enumerate record kinds, key kinds or event kinds

A task is listed when its part names the existing kinds one by one, so that a kind not yet named is
absent from it. "Unbuilt" is from §2.2. `T:` is `…/2026-09-23-phase-h-hub-parts/task-`.

| Task | What in it is per kind | Line |
|---|---|---|
| 44a | folds A1-A19 and the sibling asks into the contract; unrun | `HP:166-170`, `HP:400` |
| 08 | `hub verify` runs "each registered check in order … (tasks 20 and 27 register them)" | `T:08.md:264-266` |
| 14 | `TestInFeedIsTheContractTable`: "every kind of contract §8, row by row, with its column" | `T:14.md:213` |
| 16b | the matrix harness: one `rowsTaskNN` per route task, "listed in allRows" | `T:16b.md:61-63` |
| 20 | the resolver's per-key table: `invalid` via `hubapi.ParseKey`, then `brief:` rows, then `design:` "`missing` until task 22" | `T:20.md:29-36` |
| 20 | registers the `revisions` check in `hub verify` | `T:20.md:15` |
| 22 | "the design half" of `server/refs.go` and `store/refs.go` | `T:22.md:14`, `T:22.md:62` |
| 26 | the feed's audit-only list: `auth.user_provisioned`, `auth.forbidden`, `blob.uploaded`, `blob.deleted`, `audit.read` | `T:26.md:35-37` |
| 27 | erasure: `hub.erase_brief`; "**Nothing else changes**: approvals, releases, evidence and session links hold keys and digests, never brief text" | `T:27.md:34`, `T:27.md:40-41` |
| 27 | registers the `audit` check in `hub verify` | `T:27.md:17` |
| 34 | the digest re-check is for **brief revisions**: `VerifyRevision(r hubapi.BriefRevision)`; `Resolve` in batches of 100 | `T:34.md:6`, `T:34.md:32`, `T:34.md:52` |
| 36 | the poller's `Subscribe` by kind prefix: `"brief."`, `"design."`, `"release."`, `"project."`, or `""` | `T:36.md:61-62` |
| 40 | the desktop's `hubFeed` handlers by the same prefixes | `T:40.md:124` |
| 43, 44b | the end-to-end proof over tasks 01-42; the contract made true to what was built | `HP:446-447` |

### 2.5 Built code that names the existing kinds one by one

| Built by task | File (under `.claude/worktrees/hub/control-plane/internal/hub/`) | Line |
|---|---|---|
| 01 | `hubapi/keys.go`: ten id prefixes; `KeyKind` `brief`, `design`; the two key regexes; `ParseKey` | `:16-27`, `:47-59`, `:95-104` |
| 02 | `hubapi/envelope.go`: envelope kinds `brief-revision`, `audit-event` | `:50-51` |
| 02 | `hubapi/envelope_test.go`: vectors 1-4 | `:20-42` |
| 04, 05 | the fake emits each event kind by name: `brief.revised`, `design.approved`, `evidence.recorded` | `hubclient/fake_briefs.go:459`, `hubclient/fake_designs.go:258`, `hubclient/fake_feed.go:118` |
| 05 | the conformance suite's feed rows name every in-feed kind, one `add("<kind>", …)` each, from `team.created` to `release.expired` | `hubclient/conformance/feed.go:221-303` at HEAD `cafe1303` (`git show`); the working-tree copy is uncommitted-modified (+7 lines) |
| 07 | `migrate/sql/0001_init.sql` | §3 below |

## 3. The SQL schema as built

`SQL` below is `.claude/worktrees/hub/control-plane/internal/hub/migrate/sql/0001_init.sql`
(693 lines at `cafe1303`; not modified in the working tree). "applied by `hub migrate` as hub_owner,
in one transaction" (`SQL:2`). Built by task 07a (tables, row-level security, grants) and 07b
(guards and owner functions) (`PR:457`, `PR:463`); the ledger's acceptance count is "CREATE TABLE
20, CREATE POLICY 2, SECURITY DEFINER 6, 40 PASS" (`PR:467`), and after fix round 1 "43 PASS"
(`PR:484`).

### 3.1 Types

Nine enums, `SQL:5-14`: `team_role`, `invite_state`, `git_host`, `brief_scope`, `brief_status`
(`open`, `complete`, `skipped`), `release_kind`, `release_state`, `pin_reason`, `verdict`
(`settled`, `contested`). Every kind-specific vocabulary in the schema is one of these enums.

### 3.2 The tables, and how each brief-referencing one points at a revision

The 20 tables, in file order: `orgs` `:17`, `users` `:24`, `teams` `:41`, `team_members` `:52`,
`team_invites` `:66`, `projects` `:88`, `project_remotes` `:101`, `briefs` `:115`,
`brief_revisions` `:141`, `blobs` `:167`, `project_blobs` `:177`, `design_approvals` `:193`,
`design_approval_blobs` `:220`, `session_links` `:230`, `releases` `:252`, `release_deployments`
`:313`, `evidence` `:328`, `audit_heads` `:350`, `audit_events` `:356`, `idempotency_keys` `:376`.

**The brief, worked through:**
- `briefs.id` is checked in the database, `CHECK (id ~ '^bf-[0-9a-f]{16}$')` (`SQL:116`); the head
  carries `head_revision` and `latest_complete_revision` (`SQL:121-122`, `:130`); one live project
  brief per project and one live branch brief per `(project_id, branch_ref)` are **partial unique
  indexes** (`SQL:136-139`).
- `brief_revisions` stores `canonical bytea` — "contract §3.4's C; NULL exactly when erased"
  (`SQL:149`) — and a 32-byte `digest` (`SQL:150`). Its keys are
  `PRIMARY KEY (brief_id, revision)` (`SQL:154`), `UNIQUE (org_id, brief_id, revision)` "the target
  of every revision link (I5)" (`SQL:155`) and `UNIQUE (org_id, brief_id, revision, digest)` "the
  target of every digest-carrying link" (`SQL:156`).
- `brief_revisions_canonical_hashes`: `CHECK (canonical IS NULL OR (octet_length(canonical) <= 65536
  AND sha256(canonical) = digest))` (`SQL:161-162`). **The database itself proves that stored bytes
  hash to their digest**; `brief_revisions_erased_iff_no_content` ties erasure to `canonical`
  being null (`SQL:160`).

**How the other kinds reference a brief revision, in the database:**

| Table | Brief columns | Foreign key | Line |
|---|---|---|---|
| `design_approvals` | `brief_id`, `brief_revision`, `brief_digest` | `(org_id, brief_id, brief_revision, brief_digest) REFERENCES brief_revisions (org_id, brief_id, revision, digest)`: key **and digest** | `SQL:201-203`, `:214-215` |
| `session_links` | `brief_id`, `brief_revision` | `(org_id, brief_id, brief_revision) REFERENCES brief_revisions (org_id, brief_id, revision)`: key, no digest | `SQL:235-236`, `:247-248` |
| `releases` | `pin_brief_id`, `pin_brief_revision`, `pin_brief_digest`, `pin_head_revision_at_ship` | see 3.3 | `SQL:264-269` |

- The design approval stores its key as text under the grammar, `CHECK (design_key ~
  '^design:[a-z]{2,4}-[0-9a-f]{4,32}@[1-9][0-9]{0,8}$')` (`SQL:197`), and ties it to its parts,
  `CHECK (design_key = 'design:' || canvas_id || '@' || design_revision)` (`SQL:211`). **No table
  stores a brief key as text**: every brief reference is the pair `(brief_id, brief_revision)`, with
  or without the digest (`SQL:201-203`, `:235-236`, `:266-268`, `:333-334`).
- **The contract's DDL and the built DDL differ on the foreign keys.** The contract writes them
  single-column, `team_id … REFERENCES teams (id)` (`HC:1510`, `HC:1522`, `HC:1541`) and
  `REFERENCES brief_revisions (brief_id, revision, digest)` (`HC:1648`, `HC:1717`); the built file
  carries `org_id` in every one (`SQL:133-134`, `:157-159`, `:214-215`, `:247-248`, `:300-301`,
  `:344-345`), which is the plan's correction A16 (`HP:195`), not yet folded by 44a.

### 3.3 Releases, evidence, audit and idempotency

- **Releases.** The pin is four nullable columns (`SQL:264-269`) held whole by CHECKs: pinned iff
  `pin_reason` is `ship_head` or `inherited`, and all four null together (`SQL:281-284`). Its foreign
  key is `(org_id, pin_brief_id, pin_brief_revision, pin_brief_digest) REFERENCES brief_revisions
  (org_id, brief_id, revision, digest)`, commented "a NULL pin or design_key skips its check (MATCH
  SIMPLE); the CHECKs above keep the pin whole" (`SQL:299-301`). So a **nullable** brief reference is
  already expressed once in the schema.
- **Evidence.** `brief_id`, `brief_revision` (`SQL:333-334`), foreign key to
  `brief_revisions (org_id, brief_id, revision)`, **no digest** (`SQL:344-345`); its own
  `criterion_digest` (`SQL:336`); `because` 1-2,000 characters after trimming (`SQL:339`). The
  contract says the evidence `briefKey` "is stored verbatim" (`HC:1071`); the table stores the pair
  `(brief_id, brief_revision)`, not the text, while `repo_key` is commented "verbatim" (`SQL:332`).
- **Audit.** `audit_events.kind` is `CHECK (kind ~ '^[a-z]+(_[a-z]+)*\.[a-z]+(_[a-z]+)*$')`
  (`SQL:360`): a format, **not a list of kinds**; `feed boolean NOT NULL` is set per row
  (`SQL:361`); `canonical` is "contract §3.6, with teamId (plan A10)" (`SQL:366`);
  `audit_events_hash_chains CHECK (hash = sha256(prev_hash || canonical))` (`SQL:370`). The feed
  indexes are by project and by team, `WHERE feed` (`SQL:373-374`).
- **Idempotency.** `idempotency_keys` stores `request_sha256` and a `response` per `(user_id, key)`
  (`SQL:376-389`). The plan's A17 makes route 26's stored reply hold every `content` null
  (`HP:196`).

### 3.4 Row-level security and grants: both are written as explicit lists

- Row-level security is one loop over a **named array of 19 tables** (`SQL:397-400`), each given
  `ENABLE ROW LEVEL SECURITY` and one policy `org_isolation … USING (org_id =
  current_setting('hub.org_id', true)) WITH CHECK (…)` (`SQL:401-404`). `orgs` has its own policy
  (`SQL:407-415`). Every table in the loop therefore carries an `org_id` column.
- `GRANT SELECT ON ALL TABLES IN SCHEMA hub TO hub_app` (`SQL:419`) covers the tables that exist
  when it runs. `GRANT INSERT` names **17 tables** one by one (`SQL:421-424`). `UPDATE` is granted
  **per column** on seven tables, and `DELETE` on five (`SQL:426-436`).
- "No UPDATE or DELETE on: orgs, users, brief_revisions, design_approvals, design_approval_blobs,
  session_links, release_deployments, evidence, audit_events, audit_heads" (`SQL:437-440`).
- "**A later migration that adds a table grants on it explicitly: there are no default
  privileges**" (`SQL:441`).

### 3.5 The guards

| Guard | What it enforces | Line |
|---|---|---|
| `forbid_change()` on `design_approvals`, `design_approval_blobs`, `session_links`, `release_deployments`, `evidence`, `audit_events` | append-only: a trigger `<table>_append_only` refuses `UPDATE`/`DELETE`, `<table>_no_truncate` refuses `TRUNCATE`; the six tables are a **named array** | `SQL:446-461` |
| `guard_revision()` | "a brief revision changes once, by erasure, and only inside hub.erase_brief": never deleted; an update only by `hub_owner`, only `canonical` to null with `erased_at` set, every other column unchanged | `SQL:463-484` |
| `guard_revision_insert()` | "an archived brief takes no new revision" (fix round 1, m5), for the owner too | `SQL:486-503` |
| `guard_brief()` | "a head moves forward one revision at a time, onto a revision that exists": identity columns frozen; `head_revision` only stays or steps by one; `latest_complete_revision` never falls; archived and erased freeze | `SQL:505-527` |

| `guard_release()` | "a release's pin and identity never change; only a reserved release moves": 18 identity and pin columns frozen; `environment` set once; a non-reserved release takes no other change | `SQL:529-566` |
| `audit_append()`, `SECURITY DEFINER` | runs only as `audit_events`' trigger (`TG_RELID` check, fix round 1 I1); refuses an event for another org **before** locking; requires `seq = head + 1` and `prev_hash = head.hash`; moves the head | `SQL:568-605` |

### 3.6 The owner functions (`SECURITY DEFINER`)

"The five functions hub_app calls that run as the owner (contract §10.4; the contract's three, and
A18's org_ids and A19's lock_audit_head)" (`SQL:607-608`), plus `audit_append` above, six in all
(`PR:484`: "SECURITY DEFINER 6"):

| Function | Kind-specific? | Line |
|---|---|---|
| `ensure_org(p_tenant, p_name, p_id)` | no: org and its audit head | `SQL:609-618` |
| `resolve_principal(…)` | no: users | `SQL:620-641` |
| `erase_brief(p_brief)` | **yes, briefs only**: sets every revision's `canonical` null and `erased_at`, archives and marks the head; `idempotency_keys` "is NOT touched" (A17) | `SQL:643-663` |
| `lock_audit_head()` | no: the session's org's write lock | `SQL:665-678` |
| `org_ids()` | no: org ids for the janitor and `hub verify` | `SQL:680-688` |

`EXECUTE` on the five is revoked from `PUBLIC` and granted to `hub_app` by name (`SQL:690-693`).

### 3.7 What the schema holds that a new record kind would meet

Each item is a fact about the file, not a proposal.

- **Migrations are append-only by rule.** The runner reads `sql/NNNN_<name>.sql`, "from 1,
  contiguous" (`migrate/migrate.go:23-25`), records each file's SHA-256 (`:28`), and refuses a changed
  applied file: `ErrEdited` "an applied migration's text has changed; write a new migration
  instead" (`:32`, checked at `:145`). The ledger adds: "a persistent database migrated at bab30e90
  is ErrEdited (recreate it)" (`PR:472`). Only `0001_init.sql` exists (`sql/` listing).
- **Five places in `0001_init.sql` name tables one by one**, so a table not named in them is
  outside them: the row-level security array (`SQL:397-400`); the `INSERT` grant (`SQL:421-424`);
  the per-column `UPDATE` and `DELETE` grants (`SQL:426-436`); the "No UPDATE or DELETE" comment
  (`SQL:437-440`); the append-only array (`SQL:454-455`). A sixth, `SELECT ON ALL TABLES`, covers
  only the tables that exist when it runs, and there are "no default privileges" (`SQL:419`,
  `SQL:441`).
- **Id prefixes are checked in the database**, per table: `^bf-` (`SQL:116`), `^ap-` (`SQL:194`),
  `^ln-` (`SQL:231`), `^rl-` (`SQL:253`), `^ev-` (`SQL:329`).
- **Three shapes of brief reference exist**, each a composite foreign key carrying `org_id` (plan
  A16, `HP:195`): key and digest (`design_approvals`, `SQL:214-215`); key only (`session_links`,
  `SQL:247-248`; `evidence`, `SQL:344-345`); nullable key and digest held whole by CHECKs
  (`releases`, `SQL:281-284`, `SQL:299-301`). Their targets are `brief_revisions`' two UNIQUE
  constraints (`SQL:155-156`).
- **Two shapes of mutability exist**: append-only through `forbid_change` (`SQL:446-461`) and a
  guarded state machine with frozen identity (`guard_brief`, `guard_release`, `SQL:505-566`). The
  only versioned-and-immutable kind is `brief_revisions` under its head `briefs`, with
  `guard_revision`, `guard_revision_insert` and `guard_brief` together (`SQL:463-527`).
- **Canonical bytes and a digest CHECK exist for two tables only**: `brief_revisions`
  (`SQL:161-162`) and `audit_events` (`SQL:370`).
- **Erasure is one function for one kind**, `erase_brief` (`SQL:643-663`), and the contract's
  reason no other table is touched is that "none of them holds brief text" (`HC:1122-1123`).
- **Event kinds are not in the schema** beyond the format CHECK (`SQL:360`); whether an event is in
  the feed is a per-row boolean the writer sets (`SQL:361`), and task 14's
  `TestInFeedIsTheContractTable` is where the per-kind table is to be pinned (`T:14.md:213`).

## 4. Where the thread is consumed downstream

Every line below reads, carries, digests or renders a **brief key** or a resolved `Ref`. These are
the places a term or decision reference would reach if it travelled the same way. Nothing here
says it should.

### 4.1 Phase 5, the canvas (`.claude/worktrees/phase-5`, `feat/phase-5-canvas` at `22675f39`)

`P5` below is `docs/superpowers/plans/2026-09-23-phase-5-design-canvas.md`; `P5T:` is its parts
directory `…-parts/task-`. Phase 5 has built through task 06, ops including `review_mark`
(`.superpowers/sdd/p5/progress.md:896`, fix round `:936`).

**Built code:**

| Line | What it does with a brief key |
|---|---|
| `canvas/doc/op.go:25` | `OpReviewMark` — "BriefKey. Changes no content; exists so a re-review has a new design revision to approve" |
| `canvas/doc/op.go:59` | `BriefKey string json:"briefKey,omitempty"` — "≤ 64 bytes; never parsed here (plan G9)" |
| `canvas/doc/op.go:94` | `MaxBriefKeyBytes = 64` |
| `canvas/doc/apply.go:1128-1140` | `reviewMark` refuses an empty key and one over 64 bytes, then records the op; "The key is bounded and never parsed here" |

**Planned (unbuilt) — the rules:**

- `P5:90`: the brief comes from "`discoverhub.Service` (phase 4) ─ HeadRef/Binding ► /v1/refs,
  …/briefs/for-branch".
- `P5:183` (D3 option A, "Pin and mark"): "a brief edit never voids an approval; it shows as
  `superseded`, and a human re-reviews" by appending a `review_mark`.
- `P5:216` (S4-P5.1): "one key-source selector for the canvas's approved brief key and its branch's
  head brief key, both verbatim from route 22".
- `P5:323-325` (G9): "Only the `provenance` package (phase 4) parses a brief key … The desktop
  compares keys for equality and never splits one. **No TypeScript digest exists.**"
- `P5:534` (S5-P7.1): "`design:` keys resolve through the Hub's `/v1/refs` … phase 7's release
  manifest renders the approved design key".

**Planned (unbuilt) — the carriers:**

| Part | Line | What carries or reads the brief key |
|---|---|---|
| 08 review digest | `P5T:08.md:12`, `:113`, `:129`, `:139-146` | the review digest covers "the brief key reviewed against": `ReviewEnvelope.BriefKey` "verbatim (never parsed, G9)"; `ReviewDigest(d, briefKey)`; vectors at `@5` and `@6` |
| 15 MCP read tools, `get_brief` | `P5T:15.md:71`, `:76`, `:170` | `BriefKey` in two tool results; "never parses a brief key to work it out" |
| 20a `designapi` | `P5T:20.md:265` | `Superseded(ctx, briefKey)` "via Resolve + heads" |
| 21 approval | `P5T:21.md:51-56`, `:66`, `:152-159`, `:193-198`, `:290-296`, `:358`, `:372` | `SnapshotInput{BriefKey, BriefDigest}`; `ReviewDigest(Doc, BriefKey)`; `ManifestWant.BriefKey`; `Manifest.briefKey`/`briefDigest`; `HeadBriefKey` "verbatim"; `Approve` sends `briefKey`, `briefDigest`; re-review marks the head key |
| 22 handoff | `P5T:22.md:47`, `:64`, `:73`, `:257-259`, `:302` | the bundle's `BriefKey` checked against the approval record; `LinkSession{…, briefKey: ContextRef, …}`, the **stamped** key, not the approval's |
| 27 desktop data | `P5T:27.md:11`, `:25`, `:53`, `:155` | `selectDesignBriefKeys`, a key source for phase 4's single loader: "every loaded approval's briefKey and each ApprovalState's non-"" headBriefKey" |
| 28 Design lens | `P5T:28.md:34`, `:96`, `:100-103` | the superseded bar (*Approved against brief rev 5 … moved to rev 6*); the header chip shows `headBriefKey` before approval and the approval's `briefKey` after |
| 30 end to end | `P5T:30.md:108` | asserts `GET /v1/refs?key=design:…@n` |

### 4.2 Phase 7, the deploy connector (`.claude/worktrees/phase-7`, `feat/phase-7-deploy` at `0477c5a4`)

`P7` below is `docs/superpowers/plans/2026-09-23-phase-7-deploy.md`; `P7T:` is its parts directory
`…-parts/task-`. Phase 7 has built tasks 01-06 and slices 1-2 of task 07, the watcher
(`.superpowers/sdd/p7/progress.md:441`, `:501`, `:530`, `:683`, `:715`, `:802`, `:845`).

**Built code reads no brief key.** The watcher's Hub types are "DECLARED STAND-INS" because
"control-plane/internal holds no hubapi and no hubclient package" in that worktree
(`control-plane/internal/deploy/deploywatch/hubside.go:8-14`), and "What phase 7 deliberately does
NOT know: a Hub status code, a Hub problem body, or the Hub's key rule" (`:16-17`). Its `Release`
stand-in has no brief or pin field (`:49-60`); its `Event` has `Kind` and `ProjectID` only
(`:64-67`). `targetedPins` reads confirmed releases per deployment, to tell "no release" from "not in
the page" (`deploywatch/pins.go:10-40`).

**Planned (unbuilt):**

| Part | Line | What carries or reads the brief key |
|---|---|---|
| plan | `P7:104` | Ship takes `{branchId, commit, designKey?, noCache?}`: no brief field; the Hub takes the pin |
| plan | `P7:156` (Q2) | "releases shipped outside the cockpit read *"no brief is recorded for this release"*" |
| plan | `P7:841` | phase 5's approved `design:` key is passed as `designKey` at reserve; "the Hub never infers one" |
| 07 watcher | `P7T:07.md:344` | a release not yet read shows *"pin not loaded"*, never *"no brief is recorded"* |
| 08 the view | `P7T:08.md:114`, `:130`, `:134`, `:225`, `:416` | `release: ReleasePin \| null`; `"briefKey": … \| null` "passed to phase 4's loader; NEVER parsed here or in the desktop (P5)"; `designKey`; "The view carries `briefKey` and nothing of the brief" |
| 09 Ship | `P7T:09.md:339`, `:471` | reserve with `briefId: null` and `designKey`; `TestPinPreviewMatchesTheHubsPin` against `hubclient.NewFake()` |
| 10 rollback | `P7T:10.md:148` | `briefId: null`, the source release's `designKey` |
| 13 desktop data | `P7T:13.md:101`, `:105`, `:343`, `:361` | `briefKey: string \| null`; `selectReleasePinKeys`, "R6's key source for Deploy"; test "pin keys are opaque": no `split(`, `indexOf("@")` or regex on a key |
| 15 pin cell | `P7T:15.md:58`, `:149-150` | `usePinRef(briefKey)`; the two states *"no brief is recorded for this release"* and *"pin not loaded"* |
| 17 release drawer | `P7T:17.md:97-99`, `:112`, `:149`, `:237` | `usePinManifest(briefKey)`; the design key shown "verbatim in mono … Never resolved or parsed here (P5)"; `ShippedManifest` from phase 4 |
| 18 governance | `P7T:18.md:116` | auto-deploy says every push release will read *"no brief is recorded"* |

### 4.3 What both phases route through: phase 4's `Ref` and its one loader

Both phases hand brief keys to phase 4 rather than resolving them (`P5T:27.md:11`, `P7T:08.md:130`,
`P7T:13.md:343`). The two things they reach, in `.claude/worktrees/phase-4` (`feat/phase-4-discover`
at `ca737d92`; its ledger's last build entry is task 09, the Materialiser, fix round 1,
`.superpowers/sdd/p4/progress.md:538-545`):

- **`provenance.Ref`, built** (`control-plane/internal/provenance/ref.go:78-103`): "one brief key,
  resolved. Cacheable for ever EXCEPT headRevision and latestCompleteRevision" (`:71-72`). Every
  field is a brief field (`BriefID`, `Revision`, `Digest`, `Scope`, `Status`, `Inherits`, `Title`,
  `Headline`, `Sections`, `Criteria`, …). Its `Resolution` has six values, the Hub's four plus
  `none` and `unavailable` (`:14-31`). `10-decision-terms-beside-the-brief.md:61` records the owner's
  authorisation that `Ref` "carries term and decision digests"; `ref.go` has no such field today.
- **The one loader, planned** (phase 4 task 16, `…/2026-09-23-phase-4-discover-parts/task-16.md`,
  unbuilt): "A key source returns the keys a surface currently shows. The loader unions them. A
  later lens adds ONE entry to keySources and changes nothing else (phase 5: bound and approved
  design keys; phase 7: release pins)" (`task-16.md:165-167`); `type KeySource = (s:
  CockpitState) => readonly string[]` (`:168`); `keySources = [selectSessionContextRefs]` (`:174`);
  the loader collects "union of keySources" (`:243`). Phase 5's `selectDesignBriefKeys`
  (`P5T:27.md:155`) and phase 7's `selectReleasePinKeys` (`P7T:13.md:343`) are those entries.

### 4.4 The rule every consumer repeats

Every consumer above states that it does not parse a key: phase 5 `P5:323-325`, `canvas/doc/op.go:59`,
`P5T:15.md:76`, `P5T:28.md:103`; phase 7 `hubside.go:16-17`, `P7T:08.md:130`, `P7T:13.md:361`,
`P7T:17.md:112`, `:237`. The one parser allowed is phase 4's `provenance` (`key.go:1-2`; `PC:415`),
and the Hub's own `hubapi` (§1.1). A key whose grammar is new reaches every consumer only as an
opaque string, and reaches `Parse` in `key.go:56` and `ParseKey` in `hubapi/keys.go:96` as
`ErrInvalidKey` until each grammar is extended.

## 5. Anything already planned that resembles a glossary or a decision record

(Not started.)
