# Completeness critique 2 — the redo of the phases 4/5/7 planning pass

**Date:** 2026-09-23. **Role:** completeness critic, second pass. **Job:** find what is missing,
unverified or contradictory — not what is good. Where I verified something myself I cite
`path:line`; where I carry another document's claim through, I say so.

**Read:** every file in this directory, the three `prototype/phase-*.html` files, and the first
critique. **Ran:** scoped greps over the three prototypes, task 15's `types.ts` gate, and read-only
checks against `~/Projects/Paperdesign` and `~/Projects/Panacea`.

---

## 0. The headline

1. **The prototypes do not share one artifact.** `brief:b-42` is a different brief, in a different
   project on a different branch, in each of the three files, and `cv-7` is two different canvases
   (§2.1). Worse than sample data: Design approves against, and Deploy renders criteria for,
   `b-42@3` — a revision that under Discover's own model has no definition of done and was never
   inheritable (§2.2). **Fix before the owner opens them: about an hour.**
2. **The contract resolves the first critique on paper.** Of the six findings: one resolved
   outright (§1.2); one resolved in rule but resting on orchestrator decision 0c and an untested
   inference (§1.3); one resolved with a hole — inherited project text is not named by the key
   (§1.1); one resolved for sessions only (§1.4); one deferred to the orchestrator (§2.3); one
   half-deferred to an owner nobody has named (§2.4).
3. **Nothing breaches phase 2's `types.ts` gate** — I ran it: `IDENTICAL` (§3.1). Two prototype
   states need facts the model cannot carry, and several CSS choices would fail phase 2's audit if
   lifted (§3.2, §3.4).
4. **All three pass the three mechanical checks** — no gradient, no `transition: all`, no colour
   outside `:root` — and all three draw empty and failure states (§5). Two of their briefs
   overstate radius compliance (§4.4).
5. **Unowned and growing:** phase 3 now carries nine obligations and has no document to hold them
   (§6.3). **The biggest question the contract never asks** — only the phase 7 brief does: whether
   the Brief store and ledger are per person or shared (§7.1, item 2).

## 1. Does the contract resolve the first critique's six disagreements?

One line each, then the holes. "Resolved" below means **resolved on paper**: every ruling is a change
to the phase 4 plan (`provenance-contract-draft.md` §8.1) and none of those amendments is written.

| First critique | Verdict | Where | Why that verdict |
|---|---|---|---|
| §1.1 release attached to the brief's present tense | **Resolved, with a hole; the hole is deferred-with-owner (owner, `4-D3`)** | contract §1.2, §2.1, §3.4, §6.1, P1 | Append-only revision log plus an immutable `brief:<id>@<rev>` key fixes it for a brief with no parent. It does **not** cover inherited text — see 1.1 below |
| §1.2 phase 7's fix unbuildable against phase 4 | **Resolved** | §6.2 | Third option: a pointer to an immutable revision. No snapshot type. Rollback inherits the source pin; no pin by observation (P2) |
| §1.3 `contextRef` bypassed | **Resolved in rule; deferred-with-owner (orchestrator, decision 0c)** | §2.4, §6.3, P6, §7.3 | Rule is clear. Rests on one self-labelled inference (Claude Code re-reads `CLAUDE.md` on `--resume`, §2.4), and the phase 5 prototype breaks it — §2.3 below |
| §1.4 `useProvenance` breaks the one-poller rule | **Resolved for sessions only** | §6.4, P8 | The loader collects **session** `contextRef`s. Release pins and design-link keys are not sessions and are not in its collection set — see 1.2 below |
| §2.3 persist-version bump unowned | **Deferred-with-owner (orchestrator)** | §6.5, §7.3 | Provenance persists nothing, so it needs no bump — correct. The allocation rule for everything else is proposed, not adopted |
| §2.4 two auth models | **Half resolved; the gating half is unresolved** | §6.6, §7.2 Q6 | Which process holds the token: resolved (the control plane). Whether a loopback-only boundary may guard it: deferred to "the owner of control-plane authentication" — and §6.6 itself says that owner is not named anywhere (`client.ts` says phase 8, the phase 4 plan says phase 9). A deferral to nobody is not a deferral |

### 1.1 The key does not name what an agent read when the brief inherits

The contract's digest is *"SHA-256 over a canonical encoding of its questions, answers and status"*
(§1.2) and a `BriefRevision` carries only its own answers (§1.3). But the managed block an agent
reads is a **resolved** render: `prototype/phase-4-discover.html:307-311` resolves project → branch
exactly as the phase 4 plan does (`:1787-1793`, per the prototype's own citation), and the block
prints *"Inherited from the project brief"* sections (`:321`).

So `brief:b-42@5` names the branch brief's text, but what a session inherited also depends on
**which revision of the project brief `b-7` was current**. Edit the project brief, start a new
session, and it inherits different text under the same key. That is critique §1.1 one level up.

The phase 4 brief found this independently and verified it in the plan — `discover.Document`
carries one `BriefID` and one `Revision` (plan `:1724-1730`, per `scope-and-risk-phase-4-discover.md:147-153`,
gap 1). It routes the fix to the owner as **4-D3**, *"a project edit saves a new revision of every
branch brief that inherits it"*, and the prototype draws it (`phase-4-discover.html:572`). But 4-D3
is **not in the contract**, and the brief says *"the contract author then amends §1.2"* (R2) — an
amendment nobody has scheduled. Verified by grep: the contract's only structural mention of a parent is
*`branchId "" for a project-scoped brief`* (`provenance-contract-draft.md:118`); every "inherit" in it
means a session inheriting a revision or a rollback inheriting a pin. Project → branch resolution is
never mentioned. Either the contract adopts D3, or `BriefRevision` records the parent key it resolved against
and the digest covers the resolved text. Until one of those is written, §1.1 is resolved only for
briefs without a parent.

A second, smaller consequence: contract §1.4 builds `Ref.audience`, `constraints` etc. *"from the
revision's answers"*. For a branch that **skipped** audience to keep the project's answer
(`phase-4-discover.html:292`), the by-key `Ref` either shows no audience or must resolve the
project brief at some revision the key does not name. Unspecified.

### 1.2 The one-loader ruling covers one of three consumers

§6.4's loader *"collects the `contextRef`s of sessions in the store that are absent from
`refsByKey`"*. Two other surfaces hold keys that are not session `contextRef`s:

- **Deploy** — each release row's pinned key arrives on phase 7's releases list (§3.1).
- **Design** — the design link's brief key arrives on phase 5's canvas binding (§3.1).

Neither is collected. As written, each lens either fetches for itself (P8's prohibition) or the
loader grows two more collection sources, which moves `poller.ts`'s contract again. The ruling
needs one sentence naming how non-session keys enter the batch.

### 1.3 The rest of the first critique, scored

| First critique | Now |
|---|---|
| §0 — the deliverable does not exist | **Resolved.** Three prototypes, three scope-and-risk briefs, a phase 5 survey and a contract draft exist (listing, 2026-09-23 16:35) |
| §2.2 — `AppShell.tsx` says `phase={5}` on deploy | **Fixed in the working tree, uncommitted** (§3.3) |
| §3.2 — AGPL framed as distribution | **Half.** `PROGRAM.md:94-96` corrected, survey §5.5 asks legal the right question; three stale lines remain (§4.3) and the question has not been sent |
| §3.3 — no keychain mechanism | **Unresolved, now acknowledged.** Contract §6.6 checked `go.mod` and `Cargo.toml` and names it phase 7's cost; nobody has picked a library. Deploy's onboarding copy still promises *"the operating system's credential store"* (`phase-7-deploy.html:802`) |
| §3.4 — no phase 5 survey | **Resolved** — `survey-phase-5-design.md`, 605 lines |
| §3.4 — API base URL unresolved | **Unresolved.** Deploy now makes it a field with a *Check address* action (`phase-7-deploy.html:806-808`), which is honest — but the one call is still unmade |
| §3.4 — nobody ran anything | **Partly.** Each prototype was run in a stub DOM by its author (briefs §7, §6.3, §7); I ran the `types.ts` gate. No Go or desktop test was run by anyone |
| §4.1 offline, §4.2 branch deletion | **Partly** — §6.1, §6.2 |
| §4.3 — Go-side document versioning | **Unresolved, and the contract makes it sharper.** Revisions are now immutable and kept forever (P3), so no migration may rewrite one: the reader must handle every schema a revision was ever written in. And the digest is over *"a canonical encoding"* (§1.2) — change that encoding and every old revision fails its digest check and renders `missing`. The encoding needs a version inside the digest input. Not mentioned in the contract |
| §4.4 — onboarding surface nobody designed | **Resolved as design** — Deploy draws connect, 403-pending, 401 (`phase-7-deploy.html:794-827`). Storage still open (above) |
| §4.5 — Files and Merge unowned | **Grown** — §6.3 |
| §4.6 — phase 5 entirely | **Resolved as analysis** — survey, brief and prototype cover multiplayer (O7), the handoff bundle (I7), the design language (R10) and the window (D1) |
| §5 — canvas grid versus the audit | **Resolved for phase 5** — flat viewport (§5.1) |
| §6 item 3 — which of the seven old prototypes is approved | **Unresolved.** All three new files take their colour values from `app.html`; nobody has asked the owner whether `app.html` is the approved one |
| §6 item 6 — event stream before phase 5 | **Unresolved** — still the orchestrator's (phase 5 brief R9) |

### 1.4 What the contract settles that the first critique did not ask for

Recorded so it is not lost: §4.1's six-state vocabulary, P9 (never hide absence), P11 (no
provenance inside a frame) and P13 (branch deletion never deletes a revision) are all new, all
load-bearing, and all adopted by at least one prototype. They are the contract's best work.

## 2. Do the three prototypes agree with the contract and with each other?

**Each prototype agrees with the contract on its own. They do not agree with each other. The
artifact born in Discover is not the object Design references, and neither is the object Deploy
attaches.** This is finding one.

### 2.1 `brief:b-42` is three different briefs

All three prototypes use the id `b-42`. Read from the sample data:

| | Discover (`phase-4-discover.html`) | Design (`phase-5-design.html`) | Deploy (`phase-7-deploy.html`) |
|---|---|---|---|
| Project | `Engagement Portal` (`:280`) | `Control Testing` (`:288`) | `harbourline` (`:422`) |
| Branch | `evidence-upload` (`:281`) | `feature/evidence-upload` (`:291`) | `feat/audit-export` (`:502`) |
| `b-42` headline | *"Let client users upload evidence files against an open request."* (`:291`) | *"Testers upload evidence; every file is scanned and classified before a reviewer sees it"* (`:238`) | *"Reviewers can export a claim's audit trail as a signed PDF"* (`:425`) |
| Head revision | 5, then 6 after an edit (`:290-296`) | 3, then 5 (`:249`, `:257`) | 5 (`:423`) |
| Canvas `cv-7` | — | approved at design rev 11 (`:255`) | linked as `design:cv-7@4` (`:436`) |

Discover's `b-42` is for client users and puts *"the reviewer side of evidence"* **out of scope**
(`phase-4-discover.html:291`, `:295`); Design's `b-42` is for testers, with a reviewer in the
headline; Deploy's `b-42` is a different feature altogether. The owner reviewing the three
files in sequence cannot follow one thread through them, and the product spine is the one thing
a prototype set exists to show. **Fix before owner review:** one sample brief, one project, one
branch, one canvas, walked through all three files. About an hour; data tables only.

### 2.2 The revision models disagree — this is not only sample data

Under Discover's model, every answer, skip and edit saves a revision
(`phase-4-discover.html:260-263`, `:299` — `rev = history.length`),
and a branch brief is *"Not inherited yet"* until all five questions are answered or skipped
(`:466`). Walking its own history, `b-42@3` holds a purpose, a skipped audience and one
constraint — **no definition of done** (`:291-294`).

- **Design approves against `b-42@3`** (`phase-5-design.html:255`) and hands a Develop session
  `contextRef = brief:b-42@3` (`:434`). Under Discover's rules that revision was never inheritable.
- **Deploy renders `b-42@3` with three acceptance criteria** (`phase-7-deploy.html:428-430`). Under
  Discover's rules rev 3 has no criteria at all.

The contract is silent on the question underneath: **which revisions may be pinned, approved
against, or materialised?** §3.4 pins *"the head key at the moment of Ship"* — including, as
written, an incomplete head mid-answering, a follow-up-only revision (`:494-499`), or a D3
re-inherit revision with no answer change (`:574`). One sentence is needed, probably "only a
complete revision is pinnable", and the owner should see it drawn.

### 2.3 A Develop session started from Design breaks the contract's own rule

`phase-5-design.html:551` starts `dev-4` *"from design rev 11 · inherits `brief:b-42@3`"*, and the
button stays enabled in the superseded state (state 4, head at rev 5 — `approvedNow()` at `:268` is
still true). Discover says the opposite: *"Every session started on this branch from now on inherits
`<headKey>`"* (`phase-4-discover.html:559`).

Contract §2.4 says `contextRef` is *"the brief key of the revision that was materialised"* but never
says **which** revision Materialise writes. So a new session on one branch gets rev 5 from Develop
and rev 3 from Design, and the Design-started one is born `superseded`. Either:

1. Materialise always writes the head, and the design link's brief key differs from the session's
   `contextRef` — Design's carry card (`:434`) is then wrong; or
2. Materialise takes a key, and phase 4 must be able to render a past revision **into a worktree**,
   not only read-only in Discover (§4.1's obligation is the read-only one).

Unresolved, and it decides a phase 4 interface (`Materialiser`, §6.3).

### 2.4 One state, three markers; one shape, two meanings

Contract §4.1 requires `superseded` to carry `--drift` *plus a static shape or text marker*. Each
file complies alone; together they are three vocabularies:

| State | Discover | Design | Deploy |
|---|---|---|---|
| superseded | `◐` glyph (`:372`) | rotated-square "diamond" `.st.drift` (`:104`, `:315`) | `↻ now rev 5` (`:734`) |
| missing | `✕` glyph (`:374`) | square `.st.fail` (`:318`) | text only, no shape (`:731`) |
| needs you | `△` in notes (`:603`); rail dots are **all circles** (`:95-96`) | **triangle** (`:101`) | **diamond** (`:152`, `:155`) |

**A diamond means "superseded" in Design and "needs you" in Deploy.** `DESIGN-SYSTEM.md:39` makes
shape part of the status vocabulary; this splits it. The shape table belongs in the contract (or in
`tokens.css`'s status block), not in three prototypes.

### 2.5 The lens set drifts from the one phase 2 ships

`desktop/src/store/types.ts:173` is `"discover" | "design" | "develop" | "files" | "merge" | "deploy"`.

- Discover: all six, in that order (`phase-4-discover.html:350`). Matches.
- Design: all six, **Files moved last**, and `▤` means Develop here but Files in Discover
  (`phase-5-design.html:302` vs `phase-4-discover.html:350`).
- Deploy: **five — Files is absent** (`phase-7-deploy.html:577`), under a comment that says
  *"Four lenses"* (`:576`).

### 2.6 Where they do agree with the contract — verified, not assumed

- All three draw the six §4.1 states, and none hides `none` (P9). Specimens:
  `phase-4-discover.html:647-661`, `phase-5-design.html:455-464`, `phase-7-deploy.html:727-735`.
- Deploy never paints a row with the head (P1, `:726`), inherits the source pin on rollback and on
  rebuild-after-409 (`:689`, `:695`), and says *"no brief is recorded for this release"* for
  outside-the-cockpit releases (P2, `:730`).
- Design draws provenance only in our chrome and marks the frame viewport as untouchable
  (P11, `:329`, `:505`).
- Deploy's token copy says the window never holds it after hand-off (`:811`), matching §6.6.
- Resolving is fixed-width with no spinner in all three.

### 2.7 Three things the prototypes show that the contract has no home for

1. **"Sessions that produced it"** — Deploy's manifest lists sessions whose commits fall in the
   shipped range (`phase-7-deploy.html:842-849`). The contract's ledger (§2.2) has no
   session → release link kind and no writer for one. The prototype labels it *"depends on
   decision D3"*, but it is a new provenance claim and needs a contract row.
2. **"Chose to start without the brief"** — Discover's write-failed state says *"The session record
   shows that the person chose to start without the brief"* and the chip prints the reason
   (`phase-4-discover.html:590-593`). `Session` has no field for it, and `types.ts` is gated (§3
   below). It would have to be a ledger record; the contract does not define one.
3. **Drafted-answer authorship** — Discover records who wrote each answer (`:305`, D2). Contract
   §1.3's `answers []Answer` does not say whether authorship is part of the digest. If it is not,
   accepting a drafted answer unchanged produces a new revision with an identical digest.

## 3. Does anything contradict what phase 2 shipped?

### 3.1 The `types.ts` byte gate — run, and clean

I ran task 15's gate (`docs/superpowers/plans/phase-2-parts/task-15.md:277-280`) with the output
redirected to my scratchpad: **`IDENTICAL`**. The extraction is now **221 lines**, not the 215 the
task recorded — the contract and `types.ts` grew together (last `types.ts` commit `32b51af`,
*"keep types.ts byte-identical to the contract"*). `git diff HEAD -- desktop/src/store/types.ts` is
empty. No document or prototype in this directory proposes a `types.ts` field. **No breach.**

### 3.2 But two prototype states need facts `types.ts` cannot carry

1. **"Needs you" on a session.** Design draws `test harness` and `design-2` in a needs-you state
   (`phase-5-design.html:244`, `:253`, `:390-393`), and its notes attribute session status to
   *"the phase-2 poller"* (`:485`). `SessionStatus` is `"starting" | "running" | "idle" | "exited"`
   (`types.ts:134`). The poller cannot report needs-you. Either the canvas-access block comes from
   Northstar through a new path, or the state is drawn from a fact that does not exist —
   `AGENTIC-UX.md` §2 property 7, as the phase 7 brief quotes it, forbids that.
2. **The reason a session started without its brief** (§2.7 item 2). Not a `Session` field; needs
   a ledger record the contract does not define.

Neither is a gate breach. Both are states the owner will approve that the model cannot yet say.

### 3.3 First critique §2.2 — fixed in the working tree, not on the branch

`AppShell.tsx`'s deploy arm reads `phase={7}` in the working tree. `git show HEAD:` still reads
`phase={5}`; the one-line change is uncommitted (verified: `git diff --stat` shows 1 insertion,
1 deletion). Resolved only when phase 2 commits it.

### 3.4 Port hazards — the prototypes obey `CLAUDE.md` but not phase 2's stricter rules

Phase 2's six CSS rules (`survey-phase-2-substrate.md` §7.1) are enforced by task 24's grep audit.
Lifting prototype CSS into `desktop/` would fail it in these places:

| Hazard | Where | Phase 2 rule |
|---|---|---|
| **A `transform` transition** on the toggle thumb | `phase-7-deploy.html:280` | Rule 4: the transitionable set is exhaustive — `opacity`; `background-color`/`color` on rows and buttons; `grid-template-columns` on `.shell` only. `transform` is not on it |
| **Running and succeeded are the same shape** — `.st.ok` and `.st.run` are both a filled `--run` circle; only the `live` animation separates them | `phase-7-deploy.html:154`, `:158` | Rule 6: under `prefers-reduced-motion` a running pipeline and a green build become identical. WCAG 1.4.1 for a colour-blind reader |
| **Rail status dots are colour-only** — every `.sd` is a circle | `phase-4-discover.html:95-96` | Rule 6, and `DESIGN-SYSTEM.md:39` (*"Each carries a shape as well as a colour"*). The brief's "colour plus shape for every provenance state" (§6) is true for chips, not for dots |
| **A second heartbeat mechanism** — a `requestAnimationFrame` loop writing `--pulse` on `:root` every frame | `phase-4-discover.html:336-339` | *"The heartbeat is a hook, not a pattern to reimplement"* (substrate §7.1). `hooks/heartbeat.ts` already solves shared phase with a negative `animation-delay`; Design and Deploy mirror it, Discover does not. Writing a root custom property at display rate also restyles the whole tree each frame — inferred, not measured |
| **`--ink-4` used as text colour** | 6 / 9 / 17 declarations of `color:var(--ink-4)` in Discover / Design / Deploy | `tokens.css`: `--ink-4` is *"explicitly never text"* (1.92:1 on `--page`, substrate §7). I counted declarations; I did not check each one lands on text |
| **Token names that do not exist in `tokens.css`** | `--drift-tint` in all three; `--r-win` and `--on-accent` in Design and Deploy (tokens: `--r-window`, `--accent-ink`); `--need-ring`, `--fail-ring`, `--drift-ring` in Discover and Deploy | Rule 2 says add missing tokens to `tokens.css`, and `tokens.test.ts` pins its names. So all three phases need the same new `--drift-tint` in one phase 2 file — a small, scheduled three-way collision |
| **Flat `1px solid` borders for structure** — 17 in Design: 13 in our chrome (rail, sub-rail, header, side panel, approval bar; one is the prototype-only notes layer) and 4 in frame "paper" | `phase-5-design.html:67-189` | Not banned — `DESIGN-SYSTEM.md:25` lets hairlines carry structure — but shipped phase 2 CSS expresses hairlines as inset `box-shadow`. Discover and Deploy do the same; Design does not. `grep` over `desktop/src/**/*.css` finds one `1px solid`, in `App.css` |

### 3.5 Two phases solve "a backend other than Docker is unreachable" two ways

`problems.ts` has three kinds and adding a fourth is, by design, a compile error at every call
site (`problems.ts:29-30`, read). Deploy's brief recommends routing every Panacea failure through
`server-said` so phase 2 needs **no** change (`scope-and-risk-phase-7-deploy.md:210`). Design's brief
leaves "code or kind" to the phase 2 contract owner (`scope-and-risk-phase-5-design.md:85`, S5).

**Deploy's own prototype cannot be driven by its recommendation.** Its offline state dims every
fact, marks it stale and disables Ship and rollback (`phase-7-deploy.html:670-674`, `:595`).
`server-said` is a sentence rendered verbatim; the desktop cannot switch Ship off on a sentence
without parsing it. It needs at least a distinct problem code — itself a phase 2 contract edit.
One ruling should cover both phases, written once.

## 4. Asserted rather than verified

### 4.1 What I verified this pass

| Claim | Where it is made | Result |
|---|---|---|
| Paperdesign is `AGPL-3.0-only` | phase 5 survey §5.1, brief R1 | **True** — `~/Projects/Paperdesign/package.json:5` |
| §13 is at `LICENSE:540` onwards | survey §5.1 (`:540-548`) | **True** — heading at `LICENSE:540` |
| The fork is 75 uncommitted entries with no remote of ours | brief R2, survey §0.2 | **True** — `git status --porcelain` = 75; `git remote -v` shows only `upstream → kgoedecke/doop`; last commit `bbb174e`, 2026-09-06 |
| `3c54287` fixed the AGPL framing | task framing | **Only in `PROGRAM.md:94-96`.** `git show --stat` = 1 file. The old framing survives in three places — see 4.3 |
| `Rollback` at `projects.go:1629`; 400 at `:1657`; 409 at `:1686` | phase 7 brief §6, prototype `SRC.r409` | **True** — `~/Projects/Panacea/services/panacea-api/handlers/projects.go` |
| `RequirePortalGrant` at `middleware/rbac.go:374` | prototype `SRC.r403` | **True.** But the prototype calls its 403 text *verbatim* (`phase-7-deploy.html:818`) and drops the tail: the real body at `rbac.go:399` ends *"— finish onboarding first"* |
| `RollbackWarmth`, `ColdStartSeconds`, `TriggerDeployOptions` | phase 7 brief §6 | **True** — `models/models.go:704`, `:708`, `:2342` |
| `types.ts` unchanged and gate-clean | contract §8.2 | **True** — §3.1 |

### 4.2 Asserted — each would change a design if wrong

**Paperdesign / Northstar** (every fact below is the phase 5 survey's reading; the brief says so):

1. **A REST route for the human `VISION` approval** that our control plane can call. Unverified
   (`scope-and-risk-phase-5-design.md:93`, `:144`). Without it the "same operation" design-link
   write — the design analogue of P7 — needs a Northstar change, which waits on legal.
2. **A frame's last editor is recoverable.** Design's state 5 prints *"edited by design-1"*
   (`phase-5-design.html:362`); the brief admits the field was not found (`:147-150`).
3. **Headless agents cannot complete MCP OAuth** (R7) — no device flow found; not searched further.
4. **The keyboard map is deaf inside the iframe in WebView2** (S2) — standard DOM, untested in Tauri.
5. **`data/` holds real canvases** — *"most likely the PGlite store"* (R2). Unopened.
6. **Transitive dependency licences and the Tauri crates** — not examined (survey §5.4).

**AGPL — legal conclusions written by engineers.** Survey §5.5 correctly hands the question to
legal. The phase 5 brief then answers parts of it in advance:

- *"Does not block this prototype or a plan written against an embedded, separable server"*
  (`scope-and-risk-phase-5-design.md:31-32`, and R1). Whether an iframe-embedded, MCP-driven server
  is *separable* from PanaceaApp **is** survey §5.5's question (1). The brief assumes legal's
  answer. The safe wording is the survey's: *design so every answer stays open*.
- *"An internal push is not a §13 event"* (`:35-36`). Plausible, and the brief does say *"confirm
  with legal"* — but it is phrased as a fact.

**Panacea:**

- **Rollback creates no PipelineRun** — inferred (phase 7 brief §6), and it decides the release
  link's key shape (§6.3 below).
- **Which branch `GET …/environments` reports after a 20-character collision** — presumed the last
  deployer; Deploy's collision state depends on it.
- **The API base URL** — still two candidates. The first critique asked for one
  `GET /api/v1/version`; nobody has made it. Still a sixty-second job.
- Every other `routes.go` / `models.go` line number is still carried from the survey.

**Claude Code:** whether it re-reads `CLAUDE.md` on `--resume` (contract §2.4, phase 4 R7). The
contract's restamp-on-resume rule rests on it. It is one experiment on this laptop; it was flagged
twice today and run zero times.

**Provenance model:**

- **The ledger and Brief store are per person** (phase 7 brief D1) — inferred from contract §2.1
  and §6.6. The contract never discusses a second user. See §7, decision 1.
- **A half-answered brief is not inherited** — inferred from the plan's only Materialise trigger
  (`scope-and-risk-phase-4-discover.md:263-267`). Design and Deploy contradict it (§2.2).
- **Whether `CLAUDE.md` is committed** — contract open question 4, unanswered.

### 4.3 The old AGPL framing is still live in three places

| File | Text | Problem |
|---|---|---|
| `dependency-graph-and-order.md:55` | *"AGPL-3.0 clearance ──► gates PHASE 5 external distribution, not internal build"* | Distribution framing, in the document whose order was adopted |
| `dependency-graph-and-order.md:210` | *"a legal gate on distribution, not on internal build, so it does not block phase 5 starting"* | Same. The phase 5 survey flagged it (`survey-phase-5-design.md:407-411`); unfixed |
| `docs/PROGRAM.md:127` | *"AGPL-3.0 is fine internally, blocking for sale."* | Contradicts `PROGRAM.md:94-96`, 33 lines above it. Also flagged by the survey; unfixed |

Three one-line edits. None is in a directory this pass may write, so they are listed for the
orchestrator.

### 4.4 Prototype self-reports that the files contradict

| Brief says | File says |
|---|---|
| Discover: *"Radius is 6, 8, 10 or 14 by element size"* (`scope-and-risk-phase-4-discover.md:241`) | `border-radius` of 2px, 3px ×2, 5px ×2 (`phase-4-discover.html:64`, `:87`, `:90`, `:130`, `:185`) |
| Deploy: *"Radius — DESIGN-SYSTEM.md's scale: chip 6, control 8, card 10, window 14"* (brief §7) | 1px, 3px ×2, 4px ×2, 5px ×2, 8px, 9px as literals (`phase-7-deploy.html:62`, `:64`, `:100`, `:156`, `:227`, `:230`, `:278`, `:300`, `:318`) |
| Discover: *"The prototype uses those token names, so what the owner reviews ports without renaming"* (`scope-and-risk-phase-4-discover.md:119`) | True of the 28 pinned names; the file also defines at least nine that `tokens.css` lacks (§3.4) |
| Deploy: *"Motion — `transform` and `opacity` only"* (brief §7) | True to `CLAUDE.md`; fails phase 2's rule 4 on one transition (§3.4) |

Design's brief admits its extra 4px radius step and new token names openly (§6.2). It is the only
one of the three whose self-report I could not fault.

## 5. Design-system compliance, checked mechanically

Every grep below was scoped to the three named files and nothing else. Run 2026-09-23 through the
wrapper (one early run was not wrapped by mistake; a note is in the trail).

### 5.1 The three required checks

| File | `gradient` | `transition: all` | Literal hex / `rgba()` outside `:root` | Network refs (`fetch(`, `http` src/href, `@import`, `<link`) |
|---|---:|---:|---:|---:|
| `phase-4-discover.html` | **0** | **0** | **0** (`:root` ends `:56`) | **0** |
| `phase-5-design.html` | 1 — a comment, *"no gradient dot grid (R13)"* (`:145`). **0 in CSS** | **0** | **0** (`:root` ends `:39`) | **0** |
| `phase-7-deploy.html` | **0** | **0** | **0** (`:root` ends `:52`) | **0** |

All three pass the three checks the task names. The first critique's canvas-grid conflict
(`app.html`'s `radial-gradient` versus task 24's audit) is **resolved for phase 5**: Design draws a
flat viewport and says why. The only SVG namespace URIs are inside grain data-URIs, which make no
request.

### 5.2 Checks beyond the three, because the three pass too easily

| Check | Discover | Design | Deploy |
|---|---|---|---|
| Transitions other than the reduced-motion reset | 0 | 0 | **1** — `transform` (`:280`), fails phase 2 rule 4 |
| Off-scale `border-radius` literals | 2px, 3px ×2, 5px ×2 | 1px ×2 (status shapes, declared) | 1px, 3px ×2, 4px ×2, 5px ×2, 8px, 9px |
| Flat `1px solid` borders | 0 | 17 | 0 (one `3px solid transparent` scrollbar inset) |
| `color:var(--ink-4)` (never-text token) | 6 | 9 | 17 |
| Status separated by shape without motion | **no** — rail dots are circles | yes | **no** — running = succeeded circle |
| One heartbeat, `heartbeat.ts` mechanism | no — rAF loop | yes | yes |
| `tabular-nums` document-wide | yes | yes | yes |

### 5.3 Empty and failure states — each file draws them

**Empty:**

- Discover: *No brief anywhere* (state 1), *branch inherits the project brief* (state 2), *no
  sessions yet* in the inheritance strip (`:410`).
- Design: *No canvas yet* (state 1), *no frames yet* (`:382-385`), *no design sessions* (`:398-399`).
- Deploy: *Not connected* (onboarding), *Never deployed* (`:653-656`), *no releases yet* (`:739`),
  *branch has no brief*.

**Failure:**

- Discover: write failed and session not started (state 10), revision not found (13), control plane
  unreachable (14), model not configured (6), hand-edited block (11).
- Design: canvas server unreachable (6), design changed after approval (5), agent blocked on
  access (2), `missing` and `unavailable` in the specimen (7).
- Deploy: 401, 403, Panacea unreachable/stale, build failed, green-but-down, scan blocking, scans
  incomplete, collision, rollback 409, brief store unavailable — 17 states in all.

**Under-drawn in two of three: the local control plane itself going down while the lens is open.**
Discover draws it as a lens state (a failed save, state 14). Design draws it only as one chip in
the specimen (`phase-5-design.html:321-322`) — no lens state shows what the canvas, the sessions
panel and the approval bar do. Deploy does not draw it at all: its *"brief store could not be
read"* state (`:740-743`) is the control plane **answering** with an error. Yet the control plane
is the process that holds the Panacea token (contract §6.6) and hosts the ledger, so in Deploy this
is the state where the pins, the connection and Ship all go at once.

**Missing from Deploy specifically:** `Deployment.provisioning[]` — the brief names it as the next
state to add (`scope-and-risk-phase-7-deploy.md:304-306`).

## 6. Still uncovered from the first critique

### 6.1 Offline — partly covered, and the partial is the easy half

**Now covered:** a *third-party* backend down while the laptop is fine. Deploy keeps the last
reading, marks it stale and turns Ship and rollback off (`phase-7-deploy.html:670-674`); Design keeps
the approval because the link is ours (`phase-5-design.html:375-381`); Discover keeps typed text
and says it did not save (state 14).

**Still uncovered** — `grep` for `offline|vpn|no network|laptop` across the contract and the three
briefs finds only "no network calls" (about the prototypes) and one "offline" used as a design-floor
label for the canvas being down:

1. **The laptop itself offline, or VPN off.** Under the locked architecture the agents run on the
   user's cloud machines, so this is Panacea, the agent machines and (under Design's D1 option A)
   the canvas all unreachable at once. No document says what the cockpit shows, or whether a
   running agent keeps working unobserved.
2. **The local control plane down** — drawn only by Discover (§5.3).
3. **An undo window that expires while offline.** Deploy's Ship and rollback wait five seconds, then
   send (`phase-7-deploy.html:944`). If the network drops inside that window, what happens to the
   release link, which P7 says must be written in the same operation as the send?
4. **Which problem code "unreachable" is** — §3.5. Still no ruling, now with two phases wanting one.

### 6.2 Branch deletion — revisions settled, everything around them open

**Settled by the contract:** a brief revision survives branch and worktree deletion (P13,
`provenance-contract-draft.md:92`, `:423`, `:646`), so a release's manifest can still resolve.

**Still open:**

1. **The brief head** on branch deletion — P13 says *"may archive"*. Archive or delete is undecided,
   and no document owns it (contract §8.3 hands it to phase 3, which has no plan).
2. **No route to a deleted branch's releases.** Deploy is a per-branch lens (`phase-7-deploy.html:578`).
   Delete the branch and its releases resolve but nothing navigates to them. Whether Panacea also
   deletes the environment is unread. Phase 7 puts *delete branch* out of scope (`scope-and-risk-phase-7-deploy.md:197`).
3. **The canvas binding** `(projectId, branchId) → canvasId` is new state (phase 5 brief I2). Branch
   deletion is not mentioned; only canvas deletion is (P-g, R8, unowned).
4. **`pinned` prune** (substrate §8 item 4) — still an open phase 2 carry. Provenance now adds no
   persisted key (contract §6.5), so the contract does not copy the defect. Good; the defect remains.
5. **`missing` is now overloaded, and its required copy would be false.** The contract defines
   `missing` as *"data loss in our own store"* and makes that the wording (§4.1). This pass has
   found three causes that are not data loss and would render `missing`:
   - a teammate's release key under per-person stores (phase 7 D1 option c);
   - a design link whose canvas was deleted in Northstar (phase 5 P-g);
   - a brief erased at a regulated customer's request (contract Q5 offers *"render `missing`"*).
   Either `missing` gets a cause, or these get a seventh state. The six-state vocabulary is the
   contract's best idea; it should not tell a user their data was lost when it was deleted on
   purpose.

### 6.3 Files and Merge — owned by default, planned by nobody

`AppShell.tsx` still routes both to `LensPlaceholder phase={3}` (`:44-58`, read). Files is described
there as *"The worktree's files, with what each session changed."*

- **Files: zero mentions** in the contract or any of the three briefs. The Deploy prototype drops it
  from the lens list (§2.5). Nobody has said what it shows or whether it depends on Discover's key.
- **Merge:** implicitly phase 3 through `DefinitionOfDone` (phase 4 builds it; *"Phase 3 mounts"*,
  `scope-and-risk-phase-4-discover.md:88`) and the merge link (*"phase 3 decides"*, contract §2.2).
- **Phase 3 has no plan and no survey** — `docs/superpowers/plans/` holds phases 1, 2 and 4 only
  (read). It is next in the adopted order.

**What this pass has placed on phase 3, counted:**

| # | Obligation | Placed by |
|---|---|---|
| 1 | Criterion evaluator over `(brief key, commit)`, never a branch id | contract §2.3 |
| 2 | Write the merge link and choose its home (ledger or trailer) | contract §2.2 |
| 3 | Decide whether `CLAUDE.md` is committed (with the owner, Q4) | contract §3.4, §7.2 |
| 4 | Branch deletion archives a head and never deletes a revision | contract P13, §8.3 |
| 5 | Stamp a session id on every agent commit | phase 7 D3 option b |
| 6 | Report ahead/behind so Ship can say *"2 local commits will not ship"* | phase 7 brief §4.2 |
| 7 | Decide whether the design handoff bundle is committed | phase 5 brief §2.3 |
| 8 | Mount `DefinitionOfDone` above the merge list | phase 4 S10 |
| 9 | Own the Files and Merge lenses | `AppShell.tsx` placeholders |

Nine obligations on a phase with no document to receive them. Obligation 5 is not in contract §8.3
yet (the phase 7 brief says it would be added). **This is the first critique's §4.5 finding, grown:**
the pass keeps specifying work whose mount point lives in an unplanned phase.

## 7. What the product owner must decide — one list

**The three briefs each number their decisions D1–D3, and the nine mean nine different things.**
Discover's D3 is project inheritance, Design's D3 is approval-versus-brief-edit, Deploy's D3 is
session attribution — and Deploy's prototype prints *"depends on decision D3"* on screen
(`phase-7-deploy.html:848`). Below, every decision carries its source: `4-D1`, `5-D2`, `7-D3`,
contract `Q1`–`Q6` and `A1`–`A6`. Duplicates across documents are merged into one line.

### 7.1 Decide first — each one changes the shape of every plan

| # | Decision | Merges | Blocks | Recommendation on file |
|---|---|---|---|---|
| **1** | **Approve the contract's spine:** every revision kept, every reference an immutable `brief:<id>@<rev>` key with a digest; a release pinned at Ship; `contextRef` stamped by the server that materialised; nothing provenance-related persisted in the desktop | contract A1, A2, A3, A5; `4` R4 | Phase 4 tasks 5, 7, 8, 9; all of phases 5 and 7 | Approve — the contract argues it well and all three prototypes already draw it |
| **2** | **Per person, or shared?** Do the Brief store and the ledger live on each user's machine or somewhere the team shares? The contract never considers a second user | `7-D1`; `5-D1` option B (per-project canvas implies shared approvals); §6.2 item 5 | Where every store in contract §2.1 lives; whether a teammate's release shows a brief; whether the Panacea metadata mirror is recovery or source | Deploy's brief: per person for phase 7, said out loud; shared as the end state. **It is a hosting decision, and deferring it means migrating two stores later** |
| **3** | **What a key names.** Only complete revisions can be pinned, approved against or materialised? A project edit re-revisions every inheriting branch brief? Is answer authorship part of the digest? | `4-D3`, `4-D2`, §1.1, §2.2 | Phase 4's revision schema, and every sample revision in the Design and Deploy prototypes | Discover's brief: yes to re-revisioning, no to inheriting drafted text. Completeness rule: **not proposed anywhere — new** |
| **4** | **Who owns control-plane authentication, and does phase 7 wait for it?** The control plane is about to hold a `pnc_` token behind a loopback-only boundary | contract Q6; `7` R1 | Phase 7's first task, reads included; the canvas server's exposure under `5-D1` | None — the named owners disagree (`client.ts`: phase 8; phase 4 plan: phase 9; `PROGRAM.md`: neither) |
| **5** | **Start the legal clock on AGPL §13** — send `survey-phase-5-design.md` §5.5's four questions this week | `5` R1; first critique §3.2 | Hosting the canvas; porting; any Northstar server fix (P-a's REST route, R5, R6) | Send now. Do not rely on the brief's *"does not block a plan"* until legal says so (§4.2) |

### 7.2 Decide before the phase's own plan

| # | Decision | Merges | Blocks |
|---|---|---|---|
| 6 | **A Develop session started from an approved design inherits which revision** — the head, or the one the design was approved against? | §2.3 (new); contract §6.3 `Materialiser` | The `Materialiser` interface, and Design's carry card |
| 7 | **Where the canvas server runs, and does Northstar's second spine go?** | `5-D1`, `5-D2`, contract A6 / P10 | The whole phase 5 plan |
| 8 | **Does a brief edit void an approved design?** Pin-and-mark or void | `5-D3`, contract Q3 | When the canvas brief mirror is rewritten |
| 9 | **When the brief cannot be written before a session starts** — refuse and offer an override, or start anyway | `4-D1` | Phase 4 session start |
| 10 | **"Which sessions produced this release"** — commit trailers, drop it, or snapshot | `7-D3` | A new ledger link kind and a new phase 3 obligation |

### 7.3 Decide at the gate — cheap, but conscious

| # | Decision | Merges |
|---|---|---|
| 11 | Pin to the Ship-time head, or to the revision the merge was reviewed against | contract Q1 |
| 12 | Releases shipped outside the cockpit say *"no brief is recorded"* — acceptable? (With auto-deploy on, that is every release) | contract Q2, `7` §4.2 |
| 13 | Is `CLAUDE.md` — and the design handoff bundle — committed to git? | contract Q4, `5` §2.3 |
| 14 | Erasure, and what `missing` means: keep `missing` for data loss only and give deleted-on-purpose its own state? | contract Q5, §6.2 item 5 (new) |
| 15 | 20-character environment collisions: show after the fact, and ask Panacea for a hash; URL prefix in scope | `7-D2` |

### 7.4 Not the owner's — the orchestrator's, before the owner opens the prototypes

1. **Make the three prototypes one thread** (§2.1–§2.2): one project, one branch, one brief, one
   canvas; revision numbers that obey Discover's model. About an hour. Without it the owner is
   reviewing three unrelated demos, not a spine.
2. **One status-shape table** for all lenses (§2.4), and one lens list matching `types.ts:173` (§2.5).
3. **Decision 0c** (Materialise, option 1 or 2), **0d** (`writeAtomic`), persist-version allocation
   (contract §6.5), the problem-code ruling (§3.5), the rollback key shape for the release link
   (`scope-and-risk-phase-7-deploy.md:225`), and how non-session keys enter the one loader (§1.2).
4. **Commission the phase 3 survey** — nine obligations are waiting for it (§6.3).
5. **Four sixty-second jobs:** `GET /api/v1/version`; one `claude --resume` experiment on
   `CLAUDE.md` re-reads; the three stale AGPL lines (§4.3); commit phase 2's `phase={7}` fix.

## 8. What I did not verify

- **I rendered nothing.** No browser, no stub DOM. Every statement about what a prototype shows is a
  reading of its source, not of a screen. Visual acceptance is the owner's.
- **I ran no Go, desktop or Paperdesign test.** The one command I executed against the tree was the
  `types.ts` gate, with its output in my scratchpad.
- **I did not open the phase 4 plan.** Its line numbers here come through the contract and the
  phase 4 brief.
- **Panacea:** I spot-checked seven line citations (§4.1), all correct. Every other route and model
  line number is still carried from the phase 7 survey.
- **Paperdesign:** I verified the licence field, the §13 heading, the git state and the remote.
  Every behavioural claim about Northstar (MCP, the `VISION` gate, frame scripts, OAuth) is the
  survey's reading and I did not re-read it.
- **`--ink-4` counts** are declarations, not rendered text; some may sit on glyphs.
- **The AGPL points in §4.2** are about what our documents assert, not about what the licence
  requires. I am not a lawyer and none of this is legal advice.
- **I read `dependency-graph-and-order.md` and the phase 4 and phase 7 surveys only where the first
  critique and the new briefs cite them.** I did not re-audit them.
- Phase 2 is live in this tree. `AppShell.tsx`, `useKeyboardMap` and the palette are modified as I
  write; line numbers in `desktop/` may already have moved.
