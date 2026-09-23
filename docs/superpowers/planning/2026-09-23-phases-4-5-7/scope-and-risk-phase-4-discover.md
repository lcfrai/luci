# Phase 4, Discover — scope and risk brief, for owner review

**Status: DRAFT for product-owner review. Not an implementation plan.** Written 2026-09-23.
`CLAUDE.md`'s gate stands: *"The product owner reviews prototypes before implementation plans,
and plans before code."* This brief goes with `prototype/phase-4-discover.html`. Neither is a plan.

Product journey name: **Define**. Phase name: **Discover** (phase 4 of 9; order kept per
`dependency-graph-and-order.md`).

---

## 0. The decisions you need to make before a plan can be written

Three decisions. Each one changes the shape of the Brief or of session start, so the plan cannot
be written until you make them. Each has a recommendation. The prototype draws the recommended
option and labels it as a proposal.

**D1. When the Brief cannot be written into a worktree before a session starts, does the session
start anyway?**

- **Recommended: refuse the start and say why.** One button, *Start without the brief*, lets the
  person go on. That session's chip then shows `none` with the reason. The override is
  deliberate and visible.
- Alternative: start anyway and show `none` on the chip. This is faster, but it is exactly the
  silent non-inheritance the phase 4 plan warns about three times. The only safeguard left is the
  person noticing a chip.
- Why you, not the orchestrator: this decides whether a failed write blocks work or just gets
  flagged, and that is a product stance. The orchestrator separately owns decision 0c, *where* the
  write happens (§5, R1).

**D2. Can a model-written answer be inherited by agents before a person accepts it?**

- **Recommended: no.** A drafted answer from the repo on-ramp or a follow-up is held as
  *drafted* until the person accepts or changes it. Drafted answers are not written to `CLAUDE.md`.
  The Brief records who wrote each answer: *person*, or *drafted by the model, accepted by the person*.
  That adds an author field to `BriefRevision` (contract §1.3).
- Alternative: drafted answers are inherited immediately and marked as drafted. This is faster to a
  first session, but the definition of done shown at merge and on the release may be a guess
  nobody confirmed.

**D3. When the project brief changes, does every branch that inherits it get a new revision?**

- **Recommended: yes.** A branch brief records the project revision it inherits. A project edit
  creates a new revision of each inheriting branch brief, and running sessions on those branches
  show `superseded`. One key then still names exactly the text an agent read (§4.2 gap 1).
- Alternative: the key names only the branch brief, and inherited project text is unpinned. It is
  simpler, but a session's chip can say `current` about text that no longer exists in any store.
- Visible cost of *yes*: one project edit can mark many sessions superseded at once. The prototype
  shows what that looks like on the branch view.

**Also needed before a plan, not yours:** the orchestrator's decision 0c (server-side Materialise,
option 1 or 2) and decision 0d (`writeAtomic`, lift or duplicate). **Carried from the contract, and
yours, but it gates phase 3 more than phase 4:** open question 4, whether `CLAUDE.md` with its
Brief block is committed to git. If the repo already tracks `CLAUDE.md`, every Materialise leaves
a tracked file modified. An agent that stages everything will commit the Brief whether or not anyone decided it should.
The phase 4 plan needs that answer before it fixes *where* the block is written.

## 1. What Discover is, in one paragraph

A person opens a project or a branch and answers five questions, one at a time: what is this for,
who is it for, what must it not break, what has to be true before it merges, what is out of scope.
The answers become **the Brief**: the artifact every later phase refers to. The Brief is written
into a managed block in `CLAUDE.md` in the worktree before an agent starts, so a session does not
get the Brief as advice; it **inherits** it when its process starts. Inheritance runs **project → branch → session**. A branch
answer replaces the project answer for the same question, and the file says so. A session inherits
whatever revision was on disk when its process started. This is where the provenance thread starts.
Nothing upstream writes provenance. Develop, Design, Merge and Deploy each hold a **brief key**,
`brief:<id>@<revision>` (provenance contract draft §1.2). The key names a revision that never
changes.

Sources: `survey-phase-4-discover.md` §1 and §5; `provenance-contract-draft.md` §1–§3; the phase 4
plan's `BaseQuestions()` (`:872-925`), inheritance rule (`:1574`) and golden managed-block render
(`:1880-1905`). I read those plan regions directly. I did not read the whole plan.

## 2. In scope

| # | Item | Why it is here, and its source |
|---|---|---|
| S1 | **The five-question flow, at project scope and branch scope.** Each question has a project prompt and a branch prompt (`BaseQuestions()`). A question can be answered with a pick or free text, skipped, edited or reopened | The product's "Define" |
| S2 | **Inheritance project → branch → session**, shown on screen. Each branch answer says whether it is inherited or set on this branch | Plan `:1574`; `CLAUDE.md` product spine |
| S3 | **Editing one answer never changes a later one** | Plan Task 2 `Brief.Edit`; regression for `FUNCTIONAL-SPEC.md` known defect 7 (per the survey; I did not re-read the spec) |
| S4 | **Model-assisted follow-ups**, and on an existing repo, **a brief drafted from the code** that the person then corrects. Both fall back to the plain five questions when no model is configured | Survey §1; plan Task 1 `modelclient.Completer` |
| S5 | **Materialise**: render the managed block deterministically, merge it into `CLAUDE.md`, and keep all user text outside the markers | Plan Tasks 4–5; golden render `:1880` |
| S6 | **An append-only revision log** in the Brief store. Every revision stays readable after later edits | Contract §2.1, assertion A1. **This changes the plan**, which today overwrites |
| S7 | **The `provenance` package**: the key grammar (with the only parser), the amended `Ref`, `GET …/branches/{id}/provenance` (head) and `GET …/discover/refs?key=` (batch by key) | Contract §1.4, §3.1 |
| S8 | **The Discover lens in the desktop**, mounted in place of `LensPlaceholder phase={4}` (`AppShell.tsx:26-33`, per the survey). A non-persisted store slice. `ContextChip` goes on every session pane header, resolved from the session's `contextRef`, not from the branch | Survey §2, §4.2; contract §4.3, §6.4 |
| S9 | **A past revision opened read-only.** Every chip in the `current`, `superseded` or `missing` state opens Discover at that revision | Contract §4.1, which calls this a new obligation on phase 4 |
| S10 | **Building `DefinitionOfDone` and `ShippedManifest`** as presentational components that take a `Ref`. Phase 3 mounts the first and phase 7 mounts the second | Plan; contract §6.4 |

## 3. Explicitly out of scope

| Out | Whose it is |
|---|---|
| **Evaluating criteria.** Every criterion shows `open` in this phase. `settled` and `contested` need merge and test evidence | Phase 3's evaluator. It takes `(brief key, commit)` and never a branch id (contract §2.3) |
| **Creating and destroying worktrees.** Discover only needs `Path(worktreeId) → dir` | Phase 2 today (`project.Scanner`), phase 3 later |
| **Any git operation.** Discover never commits, stages or reads git | Phase 3. Whether `CLAUDE.md` is committed is **decision D3** below |
| **The design link and the canvas guideline mirror** | Phase 5 (contract §3.2, §3.3) |
| **The release link, the Ship handler, and every Panacea call** | Phase 7 (contract §3.4, P7) |
| **Choosing a subscription or rotating between them.** Discover calls one `Completer` | Phase 6 replaces the `Completer` |
| **Control-plane authentication.** `client.ts` says phase 8, the phase 4 plan says phase 9 | Unowned. Contract open question 6 |
| **Brief erasure for a regulated customer.** Retention is built. Erasure policy is not | Owner, contract open question 5. It does not block a phase 4 plan, but it shapes the revision log's tombstone format if one is ever needed |
| **Several people or agents editing one Brief.** This includes an agent writing a Brief over MCP | Not scheduled. If it appears, contract §6.4's "head refs are not polled" ruling reopens |

## 4. Seams

### 4.1 Against phase 2 (shipped, live in this tree)

Phase 2 is 21 of 26 tasks done and still moving. `AppShell.tsx` is modified in the working tree as I
write. Rows marked *survey* come from `survey-phase-4-discover.md` §2 and §4. I did not re-read
those. Rows marked *read* I checked in the tree today.

| Seam | State | Source |
|---|---|---|
| `worktree.Provider.Path(ctx, id)`, `project.Scanner.ProjectName/BranchName`, `statepath.Dir("discover")` | **Exact match** to what the phase 4 plan expects. No adapter needed | survey |
| `/api/v1/` alias | Only on the `guarded` mux. A Discover handler mounted on `root` would not get it | survey. Untested, because the handler does not exist |
| **The session-start hook** | **Does not exist.** `api.Deps` has no `Materialiser`. `handleSessionCreate` resolves the worktree, then goes on to `Backend.Create` with no step in between. Today the guarantee that `CLAUDE.md` is on disk before the PTY starts depends on the client alone | survey §4.1. This is **decision 0c**, and it belongs to the orchestrator |
| `Session.contextRef` | Exists and is opaque. The contract changes who writes it: the server path that materialised stamps it on every process start, and the create request's value is not trusted. `types.ts` does not change | survey; contract §2.4, §6.3 |
| The lens mount | `AppShell.tsx:28` has `case "discover":` routing to a placeholder. `SubRail.tsx:10` already lists `{ id: "discover", label: "Discover", glyph: "◇" }`. The prototype keeps that label and glyph | read |
| Token layer | `desktop/src/styles/tokens.test.ts` pins the 28 names phase 4 depends on, forbids hex literals in declarations and forbids `transition: all`. **The prototype uses those token names**, so what the owner reviews ports without renaming | read |
| Store | zustand. Lens components take no props. Phase 4 adds root keys and **zero persisted keys**, so `PERSIST_VERSION` stays at 1 | survey §4.2; contract §6.5 |
| Poller | One loader on the tick fetches only keys it has not seen. The poller's "one request per tick" comment needs a declared amendment | contract §6.4 |
| `desktop/vitest.config.ts` | Plan Task 10 step 3 would overwrite it and drop `setupFiles`, `clearMocks` and the `?raw` CSS include. The last of those turns `tokens.test.ts` into a pass against an empty string (its own first test says so) | survey §4.3; `tokens.test.ts:42-49` read |
| `writeAtomic` | Unexported in `registry/filestore.go`. Either duplicate it or lift it into `statepath` | survey §4.4. Decision 0d, orchestrator |
| Keyboard map and palette | Phase 2 tasks 25 and 26 are in flight (`useKeyboardMap`, `CommandPalette` are modified in the working tree). Discover's keys (1–4 to pick, Enter to answer, E to edit) must register through that map, not through a second listener. **Inferred.** I did not read `useKeyboardMap` | git status; inference |

### 4.2 Against the provenance contract draft

The contract is present (`provenance-contract-draft.md`, DRAFT, not approved). Discover is the
**origin** of the thread, so most of the contract's phase 4 obligations land here. The prototype
draws them.

**Taken from the contract as written:**

- The artifact is named **the Brief**. It is identified by `brief:<id>@<revision>`, shown in mono
  wherever it appears (§1.2, §4.3).
- A revision never changes, and every revision is kept (§2.1, A1, P3, P13).
- The six-state vocabulary is `current`, `superseded`, `none`, `missing`, `resolving` and
  `unavailable`. `none` is shown, never hidden (§4.1, P9).
- A past revision opens read-only (§4.1).
- There is no denominator, no percentage and no "2 of 4 met". Criteria are `open` with no evidence
  yet (P12; AGENTIC-UX principle 10). The one exception is `QUESTION n OF 5`, which survives because the base set is
  closed. Follow-ups carry no denominator (survey §3).

**Three gaps Discover exposes in the contract.** Each needs a ruling before the plan amendment is
written:

1. **One key cannot name inherited text. This is a real gap and I verified it in the plan.**
   `discover.Document` carries exactly one `BriefID` and one `Revision` (plan `:1724-1730`).
   `Resolve(project, branch)` copies the project brief's answers into it (plan `:1791`). So a
   branch session's `contextRef = brief:b-42@5` names the branch brief. The block that session read
   also contains **project** text, and no key pins it. Edit the project brief and the next session on
   that branch reads different text under the **same key**. That breaks the contract's central
   claim that a key names immutable text (§3). → **Decision D3.**
2. **Answers have no author.** `BriefRevision` (§1.3) carries questions and answers, but not whether
   a person or a model wrote each answer. Discover's repo on-ramp and its follow-ups both produce
   model-written text. → **Decision D2.**
3. **The drift check only compares keys.** Contract §3.3 permits one read-back: *"does the marker's
   key match?"* A hand edit **inside** the managed block leaves the key unchanged. The plan's render
   says such edits "will be lost" (plan `:1884`), so they are lost silently. The revision log
   makes a content check possible: render the marker's revision and compare it with the file. The
   prototype draws that state. **This is a proposal, not something the contract or the plan
   contains.** It belongs to the plan author, not the owner.

## 5. Risks, and who decides each

### 5.1 Risks that block a plan

| # | Risk | What goes wrong if nobody decides | Decides |
|---|---|---|---|
| R1 | **Nothing is inherited, and nothing says so.** `CLAUDE.md` lands after the PTY starts, or never lands | The agent works without the Brief. No error appears anywhere, and the feature looks built | **Orchestrator**: decision 0c, option 1 or 2 (contract A3 already rules out option 3). **Owner**: D1, what the person sees when the write fails |
| R2 | **Inherited project text drifts under a fixed key** (§4.2 gap 1) | A session chip reads `current` while the text that agent read is gone from every store. Deploy later pins a key whose meaning moved | **Owner**: D3. The contract author then amends §1.2 |
| R3 | **Model-written answers pass as a person's answers** (§4.2 gap 2) | A definition of done the model guessed is inherited by every agent and later shown at merge and on the release. For a regulated customer, nobody can say who set the bar | **Owner**: D2 |
| R4 | **The plan overwrites the Brief on every save** (contract §0.2) | No release can be pinned to what the Brief said when it shipped | **Owner**: approving contract A1. The plan author builds the revision log |
| R5 | **Phase 2 files are shared while phase 2 is still editing them**: `api.go`, `sessions.go`, `AppShell.tsx`, `SessionPaneHeader.tsx`, `poller.ts` | Merge conflicts, or phase 4 editing files phase 2 has not finished | **Orchestrator**: schedule phase 4's plan to start after phase 2 task 26 merges, or name the file owner for the overlap |

### 5.2 Risks the plan author handles, flagged so they are not lost

| # | Risk | Suggested handling |
|---|---|---|
| R6 | **Criterion ids are positional** (`criterionID(briefID, index)`). Inserting a criterion shifts every later id | Mint a stable id per criterion when it is first written. The contract's key-plus-id rule then becomes a belt on top of braces |
| R7 | **Claude Code's read behaviour is assumed, not verified.** Does it re-read `CLAUDE.md` on `--resume`? What does it do if the file changes mid-session? | Run one experiment before the plan fixes restamp-on-resume (contract §2.4 flags the same thing) |
| R8 | **A hand edit inside the managed block is lost silently** (§4.2 gap 3) | Content drift check against the marker's revision. The prototype draws the state |
| R9 | **Revision numbers count mutations, not versions a person would recognise.** Each answer, skip and edit bumps `Revision`, so the first complete five-question Brief is `rev 5` (contract §0.2) | Keep it. Renumbering would break the key. The prototype shows `rev 5` without explaining it away. If the owner finds it confusing, the fix is copy, not a second numbering scheme |
| R10 | **Where the prototype and the design system disagree on radius.** `app.html` uses 7px and 9px. `DESIGN-SYSTEM.md` and the shipped `tokens.css` use 6, 8, 10 and 14 | The prototype follows `tokens.css`. No action, noted so a reviewer does not read it as drift |

## 6. What the prototype shows, and what it deliberately does not

`prototype/phase-4-discover.html` is one self-contained file. Open it with a double-click. It has no
external CSS, no fonts from a network, no network calls and no build step. A strip across the top
is marked **owner review**. It switches between states and is not product UI. `[` and `]` step
through the states.

**The states drawn**, in the order the switcher lists them:

| State | What it shows | Why it has to exist |
|---|---|---|
| No brief anywhere | Project and branch both empty. The screen says sessions inherit nothing from Discover | DESIGN-SYSTEM floor: empty. Contract P9: `none` is shown |
| Branch inherits the project | The branch has no brief of its own. It lists what it inherits and from which project revision | Inheritance project → branch, before any answer |
| Asking | One question at a time, with `QUESTION n OF 5` and the inherited project answer beside it. Pick 1–4 or write your own. The Brief forms in a side panel as answers land. The flow is **interactive**: answer all five and you reach the next state | S1, S2 |
| **The Brief is born** | Answering the fifth question mints `brief:b-42@5`. The key is named and the managed block is previewed exactly as the plan's golden render has it. The panel names where it will be written and who inherits it (nobody yet) | **The provenance thread starts here.** One flash and decay on the key, only when it is minted, never on revisit |
| Follow-up from the model | A question marked as drafted by the model, with **no denominator** | Survey §3; AGENTIC-UX principle 10 |
| No model configured | The follow-up request failed. The page says why and carries on with the five questions | Survey §1: every model path degrades |
| Drafted from the code | Repo on-ramp. Each answer is *drafted*, lists the files it was read from, and is **not inherited** until accepted | D2, recommended option |
| Brief moved on | An answer was edited after sessions started. Sessions are listed by name, each with the revision it inherited, and a restart offer appears on the superseded ones. The page shows the `CLAUDE.md` diff between the two revisions. An Undo window replaces a confirm dialog, and Undo **saves a new revision**, because none is ever deleted | Contract §4.3, P3; DESIGN-SYSTEM interaction standards |
| Project brief changed | D3's visible cost. A project edit saves a new revision of the branch brief, running sessions become superseded, and the diff shows only the inherited text moving | D3, recommended option |
| Write failed, session not started | Materialise failed at session start. The page gives the reason, a retry, and an explicit *start without the brief* | D1, recommended option |
| Block edited by hand | The `CLAUDE.md` block no longer matches the revision its marker names. The page shows the changed lines | §4.2 gap 3, a proposal |
| Past revision, read-only | Opened from a chip. It says which answers have changed since that revision | Contract §4.1 |
| Revision not found | The key is shown verbatim in mono with *not found*, framed as data loss in our store | Contract §4.1 `missing` |
| Control plane unreachable | The answer is **not saved**, the typed text is kept, and there is a retry. The page never pretends the answer saved | Contract §4.1 `unavailable` |
| On a session header | The six chip states as they appear downstream in Develop | The thread leaving Discover |

**Deliberately absent**, and why:

- There is **no progress bar, completion percentage or answered-count**. The only denominator is
  `QUESTION n OF 5` (AGENTIC-UX principle 10).
- There are **no relative timestamps** ("2m ago"). They are banned as a primary time display
  (AGENTIC-UX property 1). A revision number is the Brief's identity, so the prototype shows no
  save times at all.
- There is **no digest value on screen**. The prototype cannot compute a real SHA-256 without
  network or secure-context APIs it cannot rely on over `file://`, and a made-up digest would be an
  invented fact.
- **Criterion states other than `open`**: the evaluator is phase 3's. Every criterion reads
  *open, no evidence recorded yet*.

**Design-system rules that took the most care:**

- **One heartbeat.** Every live dot reads one CSS variable, `--pulse`, and one
  `requestAnimationFrame` loop writes it. Rows rendered later still share the same phase. It
  animates opacity only, and `prefers-reduced-motion` holds it at 1. Separate CSS animations
  would each start their own phase.
- **Motion only where it explains a cause.** The key flashes and decays over 600ms (opacity on a
  pseudo-element) only when a save mints a revision, never when a state is revisited. There are
  no entrance animations, and hover changes rings, not layout.
- **Colour plus shape**, for every provenance state: a half-filled disc for superseded, a square for
  missing, `!` for unavailable, `◌` for drafted, and for `none` an unfilled outlined chip with the words "no brief". Colour is never the only
  signal. The shapes are the one table all three prototypes share (`prototype-fact-sheet.md` §10);
  until 2026-09-23 this prototype used `◐`, `✕` and `△` glyphs, which the other two drew differently
  (completeness-critique-2 §2.4).
- **Fixed-width chips on session headers**, so `resolving → current` never shifts a pixel. The
  placeholder is a static bar, not a spinner.
- **Tokens only.** Names come from the shipped `tokens.css` and values from `app.html`, except at
  least nine the file defines that `tokens.css` lacks (`--drift-tint`, `--need-ring` and others —
  completeness-critique-2 §3.4). There is no colour literal outside `:root`. Radius is 6, 8, 10 or
  14 by element size, through the `--r-*` tokens; status dots are shapes and use `50%` or `1px`, as
  phase 2's `common.css` does. **Corrected 2026-09-23:** this sentence first claimed the scale while
  the file held 2px, 3px and 5px literals (completeness-critique-2 §4.4); those are now tokens.
  Elevation is always a ring plus a layered shadow.

## 7. What I could not verify

- **Nothing was run.** No build, no test and no browser pass. **The owner's eyes are the visual
  acceptance.** Headless screenshots are blocked on the corporate network, and a screenshot would
  not prove the prototype works anyway.
- **I did not read the whole 6,527-line plan.** I read `BaseQuestions()` (`:872-925`), the
  inheritance rule and `Document`/`Resolve` (`:1574`, `:1694-1793`) and the golden render
  (`:1880-1905`). Everything else about the plan comes through the survey and the contract.
- **Phase 2 claims marked *survey*** come from the survey, not from re-reading the code. Line
  numbers will drift, because phase 2 is still committing.
- **Claude Code's `CLAUDE.md` read behaviour** on `--resume` and mid-session is unverified (R7).
- **How a tracked `CLAUDE.md` interacts with Materialise** is inferred from how git works, not
  tested against this code.
- **Sample data in the prototype** (project and branch names, brief ids, session names, paths) is
  illustrative. Every revision number shown is **computed from the sample's own mutation history**.
  None is typed in as a literal, so the numbers stay consistent with the rule that each answer, skip and edit bumps
  the revision.
- **Worktree paths** use a neutral `…/<worktree>/CLAUDE.md` form, one worktree per session. Phase 3
  has not fixed the real layout.
- **A half-answered brief is not inherited** in the prototype. Sessions inherit the branch brief
  only once all five questions are answered or skipped, and inherit the project brief until then.
  That is **inferred** from the plan's only Materialise trigger, which fires on brief completion
  (survey §4.1). I did not find it stated as a rule. If the owner wants a partial brief inherited,
  the prototype's "Not inherited yet" copy changes.
- **"Retried five times"** in the write-failed state assumes decision 0d adopts phase 2's
  `writeAtomic`, which the survey says retries five times. If phase 4 writes its own helper, the count
  comes from that helper.
- **Checks I did run on the prototype:** `node --check` on the extracted script. A jsdom pass
  (using `desktop/node_modules/jsdom`, read-only) rendered all 15 states with no script error and
  no `undefined` or `NaN` leaking into text. It drove the interactive path through five answers,
  where the key went from `@1` to `@5` and the flash fired on each save, and then made an edit,
  which produced `@6`. A grep found no colour literal outside the `:root` token block, no
  `transition: all` and no `http(s)` reference except the SVG namespace inside the grain data
  URI. **None of this is visual acceptance.**
