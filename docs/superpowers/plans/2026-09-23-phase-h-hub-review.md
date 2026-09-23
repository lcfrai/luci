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
