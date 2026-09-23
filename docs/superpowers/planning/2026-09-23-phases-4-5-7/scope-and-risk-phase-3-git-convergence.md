# Scope and risk — Phase 3, git, worktrees, coordination and convergence (product journey: **Develop → Merge**)

Written 2026-09-23 for the product owner's review. **Not an implementation plan** — `CLAUDE.md`:
*"The product owner reviews prototypes before implementation plans, and plans before code."*
Companion survey: `survey-phase-3-git-convergence.md` (this directory). Companion prototype:
`prototype/phase-3-convergence.html` (one self-contained file, open by double-click).

Decisions are numbered **`3-D1` … `3-D10`** so they cannot collide with `4-D*`, `5-D*` and `7-D*`.

## 0. The decisions you must make before a plan can be written

Ten. The first is the largest, and four others change shape depending on it. Section 0.2 lists
what is already on your desk elsewhere and also gates phase 3; section 0.3 lists what this brief
**asserts** — overrule any of them and the named sections change.

### 3-D1 — What is a "worktree" in git, and where does its repository live?

**The finding under it (survey §3.1, §4).** The object model says a branch holds *many* worktrees
(`FUNCTIONAL-SPEC.md:13`) and the prototype's Merge lens lands worktrees **into their branch**
(`prototype/app.html:1147`). git allows a branch to be checked out in **one** worktree. Phase 2
today bind-mounts one host directory per session (`handleSessionCreate`, `api/sessions.go`); for a
*linked* git worktree that directory's `.git` points at a host path the sandbox cannot see, so —
**inferred, not run** — git fails inside the sandbox on every worktree but the primary. And the Kata
backend, where the locked architecture puts every session, refuses mounts altogether
(`katabe/kata.go:84-85`).

| Option | What a worktree is | Agent's blast radius | Under Kata (phase 8) | Cost |
|---|---|---|---|---|
| a. **git worktree, plus the repo's `.git` mounted** into the sandbox | a linked worktree on its own **lane branch**, whose upstream is the product branch | **every ref in the repository** — an agent can move `main` or delete another lane | does not work — no shared filesystem into a pod | lanes clutter `git branch`; phase 2's rail must regroup them |
| **b. A local clone per worktree** (objects hard-linked on the same disk, so cheap) | a clone checked out **on the product branch's own name**; the control plane fetches its work back into the project repo under `refs/panacea/lanes/…` | **its own clone** | **the same model** — a pod gets a clone, the control plane fetches from it | `git worktree list` no longer lists them; "worktree" becomes the product's word, not git's |
| c. (a) now, (b) at phase 8 | changes between phases | changes | only the second half | two models, one migration |

**Recommendation: (b).** It is the only option whose isolation matches `AGENTIC-UX.md:51`'s
*"blast radius: own worktree only"* and the only one that survives phase 8. It also makes the rail
literally true: inside every worktree of `evidence-upload`, the agent is on a branch called
`evidence-upload`. The price is that an engineer running `git worktree list` sees one entry, and
**you should decide whether that is acceptable** before anything else in this brief is planned.

### 3-D2 — Two agents in one worktree: allowed, warned, or refused?

`FUNCTIONAL-SPEC.md:18` calls it routine. In git it means **one index and one `HEAD`** shared by
two sandboxes (survey §3.2): A's `git add -A` stages B's half-finished edit; A's `git stash` or
`reset --hard` destroys it. No file claim can be *enforced* here without wrapping the CLI, which
phase 2 forbids.

| Option | What happens | Recommendation |
|---|---|---|
| a. Allowed, silently | the hazard is invisible until work is lost | no |
| **b. Allowed, and the worktree says so** — *"2 agent sessions share this worktree; they share one index"*, plus a one-click *"start in a new worktree instead"* on the second | honest; costs one line | **Recommended** |
| c. Refused for a second **agent** (a shell beside an agent stays allowed) | safest; contradicts the functional spec | only if you change the spec |

### 3-D3 — When a lane conflicts, who resolves it, and is anything automatic?

Conflicts are always resolved **in the lane**, never in the product branch (asserted, §0.3 A2). The
question is who starts it.

| Option | Recommendation |
|---|---|
| a. Human only, in a shell in that lane | the floor; always available |
| **b. One click starts a new agent session in the lane with an opening prompt** — *"Merge `evidence-upload` into this branch and resolve the conflicts in `routes.go`, `upload.ts`"* — shown and editable before it starts | **Recommended.** Uses phase 2's opening prompt (`FUNCTIONAL-SPEC.md:29`); writes nothing into a running agent's terminal |
| c. Automatic: a conflicting lane is re-queued and an agent resolves it unasked | no — an unasked agent rewriting a lane is an ASSUMED event (`AGENTIC-UX.md:29`) nobody legislated |

### 3-D4 — How a lane lands: merge commit, squash, or rebase?

| Option | Session trailers on the lane's commits | Merge link | Undo | Recommendation |
|---|---|---|---|---|
| **a. Merge commit** (`--no-ff`) | kept | a trailer on the merge commit | exact, by moving the ref back | **Recommended** |
| b. Squash | **lost** — one commit, one session | on the squash commit | exact | loses "which sessions produced this" (phase 7's `7-D3`) |
| c. Rebase then fast-forward | kept, but **every SHA changes** | nowhere natural | exact | evidence and release links name SHAs; rebasing breaks them |

This is lane → product branch only. Product branch → trunk goes through a PR, and the remote's
merge setting is the team's, not ours (survey §7.3).

### 3-D5 — What counts as evidence that a criterion is met, and who records it?

Contract §2.3 gives phase 3 the evaluator. Nothing says what evidence **is**. A natural-language
criterion cannot be judged mechanically; an LLM's verdict is a claim, not a fact
(`AGENTIC-UX.md:202`). The survey (§8.4) defines the evaluator as **a function over recorded
evidence and git ancestry**, so the question is only who records it.

| Option | Recommendation |
|---|---|
| **a. A human, in the Merge lens, per criterion, at the commit in front of them — verdict plus a required *because*** | **Recommended for v1.** A recorded human judgement is a fact: who, when, at which commit, why |
| b. (a), plus agents *propose* evidence the human accepts | needs a channel from agent to cockpit — MCP or a harness hook — both exposed to the managed Claude policy (risk R5). Later |
| c. Test-linked: a criterion bound to a test, the verdict from a run at that commit | needs a test runner at an arbitrary commit. A later phase |

Until evidence exists every criterion reads `open`, as phases 4 and 7 already draw it.

### 3-D6 — What enters git history?

Four things could. They are one decision because a regulated customer will ask about all four at
once. Contract Q4 and the phase 5 brief's §2.3 question are folded in here.

| Item | Contains | Recommendation | Why |
|---|---|---|---|
| **Merge link trailer on every land** — `Panacea-Brief: brief:b-42@3` + digest | an id and a hash, **no brief text** | **Yes** | the only home written in the same operation as the land (survey §7.2); visible to teammates; survives our state directory |
| **Session trailer on agent commits** — `Panacea-Session: s-3f2a91c0` | an internal id | **Yes** | obligation 5; lets phase 7 say which sessions produced a release |
| **`CLAUDE.md`'s managed brief block** | **the brief's text** | **No** | two lanes that inherited different revisions would **conflict on every land** (survey §9.1); the merge trailer gives most of what committing would |
| **Design handoff bundle** | approved screens and tokens | **No** | same reasons; what an agent *builds* from it is code and is committed normally |

If you answer *yes* to `CLAUDE.md`, the contract's conditional cross-check (§3.4) becomes real and
phase 3 must resolve managed-block conflicts on every land — a feature in its own right.

### 3-D7 — May phase 3 read a harness's transcript **file** (never its terminal) to link each diff hunk to the span that wrote it?

`AGENTIC-UX.md:206` calls hunk → transcript-span back-links *"the strongest differentiator
available"*. Phase 2's binding rule is *"Agent CLIs are never modified, wrapped or parsed. No
feature in this phase may read PTY bytes to derive meaning"*
(`2026-09-18-phase-2-backend-and-cockpit.md:49`). A transcript file — Claude Code's JSON-lines
conversation log, which the resume fix is about to name by an id **the control plane mints** and to
persist beyond the sandbox — is not PTY bytes, and `PROGRAM.md:116` forbids parsing *"agent UI"*,
which it is not. But it is the CLI's private, undocumented format. **The two rules read differently
here, and only you can say which one you meant.**

| Option | What a hunk links to | Recommendation |
|---|---|---|
| a. No | the commit and session that introduced it (tier 1, survey §8.3) — every harness, shells included | the floor; always built |
| **b. Yes, read-only, Claude Code only, pinned to the image's CLI version with a canary test; tier 1 when the format moves** | the exact tool call | **Recommended** — it is the differentiator, and it degrades to (a) rather than breaking |
| c. Yes, for every harness | tool call, per harness | each harness's file read separately — later, one at a time |

### 3-D8 — Which remote host, and which credential store?

Push runs from the control plane with the user's own credentials, never from the sandbox (§0.3 A4).
For **push**, the machine's existing git credential helper is enough. For **PR creation**, the
control plane needs a host API and a token, and it has no credential store (contract §6.6) — phase 7
needs one for its `pnc_` token too.

- **Name the host** Panacea projects push to — GitHub (KPMG's EMU org), Azure DevOps, or both.
  Nothing I read names it (survey §11). **Recommendation: whichever one Panacea's pipeline clones
  from, only that one in v1.**
- **One credential store, chosen once, for phases 3 and 7.** Recommendation: decide it in whichever
  of the two plans is written first, and have the other consume it.

### 3-D9 — What does the Files lens show?

No document says (`completeness-critique-2.md` §6.3). `AppShell.tsx:49` says *"The worktree's
files, with what each session changed."*

| Option | Recommendation |
|---|---|
| **a. Changed paths only, across all of the branch's worktrees** — each path shows which worktrees touch it, marks a path two of them touch, and opens its diff with back-links | **Recommended.** It is where file-claim coordination becomes visible, and Merge is the same facts on the other axis |
| b. The full file tree, changes highlighted | an IDE's job; the agent has one |
| c. No Files lens — a second tab inside Merge | removes a lens `types.ts:173` already declares; a phase 2 contract edit |

### 3-D10 — Authority is not phase 3. Which phase is it?

The top bar's disabled *Authority* button says *"It arrives in Phase 3"* (`TopBar.tsx:29`).
`PROGRAM.md`'s phase 3 does not mention authority; it is `AGENTIC-UX.md` §7's grid, and it needs the
agent-runtime signal that document calls *"the single most important non-UI implication"*
(`AGENTIC-UX.md:60`). **Recommendation: name its phase, or remove the phase number from that
title** until one owns it. Phase 2's open carry 8 already says one decision settles Files, Merge
and Authority together; `3-D9` and this one are that decision.

### 0.1 In one table

| # | Decision | Recommendation | Blocks |
|---|---|---|---|
| **3-D1** | What a worktree is in git; where its repo lives | **local clone per worktree**, fetched back under `refs/panacea/lanes/…` | every other section; phase 8's first task |
| 3-D2 | Two agents in one worktree | allowed, **and the worktree says so** | session start, worktree row copy |
| 3-D3 | Who resolves a conflict | **one click starts a resolving session, prompt shown first** | Merge lens conflict state |
| 3-D4 | Land as merge, squash or rebase | **merge commit** | merge link, undo, `7-D3` |
| 3-D5 | What evidence is; who records it | **a human, with a because, at a commit** | the evaluator; every non-`open` state in phases 3, 4, 7 |
| 3-D6 | What enters git history | trailers **yes**; `CLAUDE.md` block and handoff bundle **no** | phase 4's Materialise location; phase 5's bundle |
| 3-D7 | Read transcript files for span back-links | **yes, Claude Code only, pinned, degrading to commit-level** | diff review depth |
| 3-D8 | Remote host; one credential store | **the host Panacea clones from; one store shared with phase 7** | push, PR |
| 3-D9 | What Files shows | **changed paths across the branch's worktrees** | the Files lens |
| 3-D10 | Authority's phase | **not 3 — name one or drop the number** | `TopBar.tsx:29` copy |

### 0.2 Already on your desk elsewhere — they gate phase 3 as well

| Asked in | Question | What phase 3 adds |
|---|---|---|
| contract Q5 | Erasure of a brief | phase 3's deletion keeps **commits** alive with archive refs (survey §6.2). An erasure request now covers those refs too |
| contract Q6, `completeness-critique-2.md` §7.1 item 4 | Who owns control-plane authentication | phase 3 adds a **push** route behind the loopback-only boundary. Today another local user's process could drive sessions; after phase 3 it could push as you |
| `7-D1` | Provenance per person or shared | the merge-link trailer is **in the repository**, so lands — unlike releases — are visible to every teammate who fetches. A partial answer to `7-D1` for free |
| `completeness-critique-2.md` §7.1 item 3 | Which revisions a key may name | the land records the head key the human saw above the queue. If only complete revisions are pinnable, a land against an incomplete brief records *"no complete brief"*, not a key |
| `7-D3` | "Which sessions produced this release" | answered by `3-D4` (a) plus the session trailer: the manifest lists the sessions whose trailers are in the shipped range |

### 0.3 Asserted by this brief — overrule any of them

| # | Assertion | Sections that change |
|---|---|---|
| A1 | **git is the truth for every git fact.** Phase 3 persists no tip, dirty flag, ahead/behind or conflict — not in the registry, not in the desktop | §2.6, survey §6.3 |
| A2 | **Conflicts are resolved in the lane, never in the product branch.** The product branch only ever receives clean merges | §2.4, `3-D3` |
| A3 | **No force-push, in v1 or by default later.** Undo after push is a revert commit | §2.4, survey §7.4 |
| A4 | **Push and PR run in the control plane with the user's credentials. A sandbox never holds a git credential** | §2.5, `3-D8` |
| A5 | **A land is plumbing with compare-and-swap** — `merge-tree`, `commit-tree`, `update-ref <new> <old>` — and **refuses a product branch that is checked out somewhere** | §2.4, survey §7.1 |
| A6 | **Phase 3's git operations sit behind an interface keyed by machine**, local implementation only, so phase 8 adds one rather than rewriting phase 3 | §4.4, survey §4.4 |
| A7 | **Deleting a branch archives its tip as a ref**, so commits a release, land or evidence record names stay reachable; and it asks phase 4 to archive the brief head | §2.6, survey §6.2 |
| A8 | **A rename done by the cockpit is recorded as `old id → new id`** so branch-keyed stores can follow it; a rename done in a terminal reads as a delete plus a create | §2.1, survey §6.1 |
| A9 | **No file claim is declared or enforced in v1.** Claims are observed from git and forecast with `merge-tree` | §2.3, survey §3.5 |

---

## 1. What this phase is

A branch in Panacea is a line of product work that several agents pursue at once, each in its own
worktree. Phase 3 makes those worktrees **real git**: it creates and removes them, gives every agent
a working git with the user's own identity inside its sandbox, and stamps each commit with the
session that made it. It then **converges** them: it shows, from git alone, which worktrees are
changing which files and which of them will conflict; it lands them into the branch one at a time,
in an order the human sets, with the brief they were reviewed against written into the merge
commit; it lets the human take a land back; and it pushes the branch and opens a PR. Its two
surfaces are **Merge** — what lands, in what order, against which brief — and **Files** — what is
changing, and who is changing it. It is the phase where provenance meets git: the Brief from
Discover is shown above the queue, each land records which revision it was reviewed against, and
every later release can find that record in the code it shipped.

---

## 2. In scope

### 2.1 Worktree lifecycle

- Create a worktree on a branch, as `3-D1` decides it; it resolves through phase 2's
  `worktree.Provider.Path` (seam S3.2) and appears in the rail under its branch.
- Remove a worktree: refuses while a session is live on it or files are uncommitted, unless the
  human confirms in a question that names both (survey §6.2).
- Create, rename (with the id map, A8) and delete a product branch (with the archive ref, A7).
- Fetch from the remote on demand and on a slow timer — never on the 2 s poll — with its time shown.

### 2.2 Git inside the sandbox

- One environment block on every session create and restart: the user's `user.name` and
  `user.email` from the machine's git config; `core.hooksPath` pointing at the image's hook
  directory; the session id (survey §5).
- A `prepare-commit-msg` hook that appends `Panacea-Session: <id>` and **chains to the
  repository's own hook** of the same name, so a repo using husky keeps its hooks.
- Under `3-D1` (b), the clone is checked out on the product branch's name, so the agent's git world
  matches the rail.

### 2.3 Coordination — observed and forecast, never enforced

- **Observed claims:** for every worktree of a branch, the paths changed since it left the branch
  (`git diff --name-only`) and the paths changed and not committed (`git status --porcelain`).
- **Forecast:** for each worktree marked ready, `git merge-tree --write-tree` against the branch as
  it will be after the worktrees above it land — *lands clean* or *conflicts with `test` in 2 files*.
- **The shared-worktree warning** of `3-D2`.
- Nothing asks an agent to declare a claim, and nothing refuses an agent's write (A9).

### 2.4 Convergence

- The Merge queue: worktrees marked ready (`Worktree.ready`, seam S3.3), in the human's order
  (`moveWithinVisible`, seam S3.4). Ready is set by a human; nothing infers it (known defect 9).
- Land one, or *land all in order*: plumbing with compare-and-swap (A5). A land that loses a race
  re-forecasts and may stop at a conflict.
- On conflict: the `3-D3` resolution; the product branch is never left conflicted (A2).
- Landed history: every land on the branch, its merge link, and its undo — exact before push and
  before a later land, a revert commit otherwise (A3).

### 2.5 Out to the remote

- Push the product branch from the control plane (A4); git's own sentence when the credential helper
  has nothing cached.
- Ahead/behind against the remote for each product branch, *"never pushed"* when there is no
  upstream, and the time of the last fetch — phase 7's *"2 local commits are not pushed and will not
  ship"* reads the first number.
- Open a PR on the host `3-D8` names, with the brief key in its description **as text, labelled as
  text** (survey §7.3).

### 2.6 Provenance — where the thread meets git

- **Mount `DefinitionOfDone`** (phase 4's component) above the Merge queue, from the branch's head
  `Ref`; states from the evaluator at the branch head.
- **The merge link** as a trailer on the merge commit, written by the land itself; the ledger, if
  kept, is an index rebuilt from trailers (`3-D6`).
- **The evaluator** — `(brief key, commit) → states`, over evidence records and ancestry (survey
  §8.4). Evidence recorded as `3-D5` decides.
- **Deletion** archives commits (A7) and asks phase 4 to archive the brief head; it never touches a
  revision (contract P13).
- **Diff review with back-links:** every hunk links to its commit and session (tier 1); to its
  transcript span if `3-D7` allows (tier 2).

### 2.7 The two lenses

- **Merge**, as survey §8.5 lists it, element by element with the fact under each.
- **Files**, as `3-D9` decides it.
- Both fill the lens files task 26 creates (seam SX.1). Neither edits `AppShell.tsx`.

---

## 3. Explicitly out of scope

| Out | Why | Where it goes |
|---|---|---|
| Force-push, history rewriting of anything pushed | A3 | — |
| Declared or enforced file claims | A9; enforcement needs CLI wrapping | a later phase, if an agent channel exists |
| Getting a clone into a Kata pod | pod plumbing | phase 8, behind A6's interface |
| Merging a PR at the remote, or reading its review | the remote's server does it; we would be re-implementing GitHub | — |
| Resolving conflicts ourselves (no merge editor in the cockpit) | the agent or the human resolves in the lane | — |
| Criterion verdicts from an LLM, or from tests | `3-D5` (b) and (c) | later |
| Transcript back-links for Codex, OpenCode, Gemini CLI | `3-D7` (c) | one harness at a time, later |
| Authority, the LEGISLATE grid | `3-D10` | the phase you name |
| A full file browser or an editor | `3-D9` (b) | — |
| Control-plane authentication | contract Q6 | its owner |

---

## 4. Seams

### 4.1 Against phase 2 (live in this tree)

| Seam | What phase 3 does there | Hazard |
|---|---|---|
| `internal/project` — three read-only git calls, whitelist-tested (`project_test.go:427-464`, task 26 S3.1) | **leaves it alone**; every git write lives in a new package | widening the whitelist is the wrong fix and the test says so |
| `ExecRunner` (`gitrunner.go:46-78`) | copies its stdout-only and headless discipline into a runner for writes, **without** `GIT_OPTIONAL_LOCKS=0` and with a longer timeout class | a write that skips its lock corrupts an index |
| `handleSessionCreate` / `handleSessionRestart` (`api/sessions.go`) | adds the environment block (§2.2) | **Phase 4's Materialise lands in the same handler** — `dependency-graph-and-order.md:128` marks it *SERIALISING*, and `orchestrator-rulings.md` R1 already names two occupants of that spot (Materialise, and the resume fix's minted session id). **Phase 3's environment block is a third.** Its natural order is after both: the block carries the session id the resume fix mints |
| `ProblemKind` (`api/problems.ts`) and `orchestrator-rulings.md` R4's `upstream-down` | **no new kind.** A failed push or fetch is git's own sentence through `server-said`; nothing in these lenses is switched off because a remote is down — the fetch time shows staleness instead | if a later design wants Push disabled while the remote is unreachable, it needs R4's kind with `details.upstream` naming the git remote |
| `backend.Spec.Env` / `EnvEntries` (`backend.go:175-205`) | carries the block; the session id as a new reserved `PANACEA_APP_*` variable is a Go-contract amendment (survey §5.2) | both backends must produce the same environment — `kata_test.go:357-372` pins it |
| Lens files (task 26 SX.1) | fills `FilesLens.tsx` and `MergeLens.tsx` | **task 26 has not landed**; today `AppShell.tsx:44-58` still routes both to `LensPlaceholder`. Phase 3's desktop half waits for task 26, or it re-opens the `AppShell.tsx` collision |
| `store/types.ts` (byte-gated, SX.2) | **does not change.** Git facts come from phase 3's own endpoints into an unpersisted slice, as the contract does for provenance (§6.4 there) | adding `ahead`, `dirty` or `tip` to `Worktree` is a contract §2 amendment and a byte-gate change for no benefit |
| `Worktree.ready` + `moveWithinVisible` (S3.3, S3.4) | gates and orders the queue through them | known defects 9 and 10 are what these seams exist to prevent |
| `palette/commands.ts` (SX.6) | appends *Land*, *Mark ready*, *Push* | a destructive verb one keystroke from a filter — *Remove worktree* must not be |
| `--drift` (`tokens.css:77`, SX.9) | may use it for "this worktree is behind its branch" | only the day that is derived, and task 24's audit assertion changes in the same commit |
| Route recorder (`api.go:172`, SX.4) | registers every route through it; `GET` never writes | a fetch triggered by a `GET` is a side-effecting GET |

**Rail regrouping.** Under `3-D1` (b), `git worktree list` (`project.go:282`) no longer enumerates
worktrees, so phase 2's scanner either learns where clones live or phase 3 serves the worktree
list. Either is a change to a phase 2 seam the plan must declare; under (a), every lane is a local
head and the branch list needs regrouping instead. Neither option is free.

### 4.2 Against the provenance contract draft

| Contract says | Phase 3 does | Fit |
|---|---|---|
| Merge link — *"phase 3 decides: ledger or a git trailer"* (§2.2) | **trailer**, ledger as derived index | fits; the only home atomic with the land |
| Evaluator takes `(brief key, commit)`, never a branch id (§2.3) | evidence records + ancestry | fits; **defines** what the contract left open |
| Evidence keyed by brief key and criterion id (§2.3) | adds a **criterion text digest**, so evidence carries across revisions only when the text is unchanged | **amendment proposed** — the positional id breaks on insertion (`provenance-contract-draft.md:209-213`) |
| `DefinitionOfDone` renders the head `Ref`, states at the branch head (§4.4) | mounts it above the queue; states at the product branch's head | fits |
| P13 — no revision deleted on branch deletion | archive ref for commits; asks phase 4 to archive the head | fits, and widens P13 to git objects |
| P5 — no key parsed outside `provenance` | copies the key string into a trailer and back, never splits it | fits |
| P7 — a link written by a second call is lost in a crash | the merge link is inside the merge commit | fits |
| §3.4 conditional cross-check from a committed `CLAUDE.md` | recommends **not** committing it (`3-D6`) | **the cross-check does not exist** if you agree; the merge trailer replaces most of it |
| Six-state vocabulary (§4.1) | every land's brief key renders in it: `current`, `superseded`, `none` (*"no brief was shown when this landed"*), `missing`, `resolving`, `unavailable` | fits |

### 4.3 Against phases 4, 5 and 7

- **Phase 4** — `DefinitionOfDone` is phase 4's component, mounted by phase 3 (S10). Phase 4's
  Materialise location depends on `3-D6`: if the managed block must never be committed and a repo
  already tracks `CLAUDE.md`, phase 4 needs another location (survey §9.1). Branch deletion calls
  phase 4 to archive a head; phase 4 needs that entry point.
- **Phase 5** — the handoff bundle is materialised into a worktree; under `3-D6` it is excluded,
  not committed. A worktree started *from* a design is created by phase 3 like any other.
- **Phase 7** — reads ahead/behind for Ship; reads merge-link trailers in the shipped range for the
  manifest's *"reviewed against rev 3 at land"*; reads session trailers for *"which sessions produced
  it"* (`7-D3`); passes `Deployment.Commit` to the evaluator. **Phase 7b (Ship) waits on phase 3's
  push** (`dependency-graph-and-order.md:275`). Phase 7a (read) does not.

### 4.4 Against phase 8

Phase 8's first problem is getting a worktree into a pod (task 26 S8.3). Under `3-D1` (b) that
problem is *"give the pod a clone and fetch from it"*, which is the same model phase 3 builds
locally, behind the interface of A6. Under (a) it has no answer that keeps git worktrees.

---

## 5. Risks, and who decides each

### 5.1 Decide or verify before a plan is written

| # | Risk | Severity | Who | Where |
|---|---|---|---|---|
| **R1** | ✅ **VERIFIED 2026-09-23, and WORSE than inferred: git fails inside the sandbox on EVERY worktree, primary included.** See the verification note below this table | **High — it is a defect in shipped phase 2, not only a phase 3 risk** | Primary: fixed in phase 2 (`safe.directory`). Linked: **`3-D1`** | survey §4.2 |
| **R2** | ✅ **VERIFIED 2026-09-23: no commit identity inside the sandbox.** `git config --global --get user.name` exits 1 in `panacea-app/sandbox:dev` | High, cheap | **Phase 2** sets it; phase 3 adds the session trailer (§2.2) | survey §4.3 |

**Verification of R1 and R2, by the orchestrator.** A throwaway repository with a real linked worktree,
each mounted at the **real** session mount point `/home/dev/work` (`control-plane/internal/api/sessions.go:33`)
in `panacea-app/sandbox:dev`, running as the image's `dev` user. Exit codes captured by redirecting to
a file, not through a pipe:

| Mounted | `git status` | Cause |
|---|---|---|
| **primary** worktree | **exit 128** — `fatal: detected dubious ownership in repository at '/home/dev/work'` | The bind mount's owner does not match the container's `dev` uid, so git's `safe.directory` protection refuses the repository. **This is the half the survey did not predict** — it makes git fail even on the primary. |
| **linked** worktree | **exit 128** — `fatal: not a git repository: /home/dev/work/C:/Users/…/main/.git/worktrees/linked` | The linked worktree's `.git` is a **file** containing `gitdir: C:/Users/…` — a Windows host path that cannot exist inside a Linux container. |
| primary, with `-c safe.directory=/home/dev/work` | **exit 0** — `On branch main, nothing to commit` | One configuration line fixes the primary case. |

**Nothing configures git anywhere today** — not the Dockerfile, not `entrypoint.sh` (which only
`exec`s `sandboxd`), not the control plane. So every phase 2 session so far has run an agent with **no
working git**. Phase 1's end-to-end proof tested the terminal round trip, not git, which is how this
passed. **The linked case is the common one**: the product's premise is one worktree per agent.

**Ruling.** Phase 2 fixes what it cheaply can — `safe.directory` for the mount point, and a commit
identity passed in from the user's own host configuration — and makes a session in a **linked**
worktree **say plainly that git is unavailable there**, rather than letting the agent discover it
from a `fatal:`. **The linked-worktree fix itself is `3-D1`'s** — a git worktree cannot work across the
Windows-to-Linux boundary as mounted, which is one more argument for the brief's recommended *local
clone per worktree*.
| R3 | **Phase 3 and phase 4 both edit `handleSessionCreate`** | Medium — certain collision, small | **Orchestrator** — schedule | §4.1 |
| R4 | **The managed Claude policy may apply inside the sandbox** — the laptop's policy strips `settings.json` hooks and MCP; a sandbox Claude Code that `/login`s as the same KPMG account probably fetches it too | Medium — rules out any Claude-hook or MCP mechanism, which is why this brief uses none | **Orchestrator — verify** with one `/login` in a sandbox | survey §5.3 item 6 |
| R5 | **Task 26 has not landed**, so the lens files do not exist | Medium — the `AppShell.tsx` collision returns if phase 3 starts its desktop half first | Orchestrator | §4.1 |
| R6 | **Loopback-only control plane gains a push route** | Medium — another local user's process could push as you | **You** — contract Q6 | §0.2 |

### 5.2 Designed for, or someone else's

| # | Risk | How it is handled | Who |
|---|---|---|---|
| R7 | The session hook shadows a repository's own hooks (husky) | the image's hook chains to the repo's; tested against a real husky repo before it ships | Phase 3 implementer |
| R8 | The session trailer is advisory — an agent can bypass it | *"no session recorded"* on screen, with the same words for "made outside the cockpit" and "bypassed", because we cannot tell them apart | drawn in the prototype |
| R9 | The conflict forecast covers committed work only | uncommitted overlap is labelled *"both edit this file — not yet committed, so not forecast"* | drawn in the prototype |
| R10 | Transcript format drift, if `3-D7` is (b) | a canary test against the pinned image; tier 1 when it fails | Phase 3 implementer |
| R11 | Unpushed commits become unreachable on branch deletion and `gc` deletes them | archive refs (A7) | Phase 3; erasure is contract Q5 |
| R12 | Lane clones on Windows: hard links, path length, antivirus scanning a clone per worktree | unmeasured; the plan's first task measures clone time and disk on this laptop | Phase 3 implementer |
| R13 | A land races another land | compare-and-swap in `update-ref` (A5); the loser re-forecasts | designed |
| R14 | The criterion id is positional, so evidence attaches to the wrong sentence after an insertion | criterion text digest (§4.2 amendment) | **Phase 4** owns the `Ref` schema |
| R15 | `sessions.go` moved by ~100 lines while this brief was written — the resume fix is landing in it | cite by function name; re-read before planning | Plan author |

---

## 6. What I could not verify

- **Nothing was run.** No build, no test, no container, no git command that writes. Reads only —
  listed in survey §11.
- **R1 and R2 are inferences.** Both are one-minute experiments; neither was done.
- **git behaviours taken from documentation:** `worktree add` refusing a checked-out branch;
  `merge-tree --write-tree` (needs git ≥ 2.38 — this laptop has 2.53.0); `update-ref`'s
  compare-and-swap; `prepare-commit-msg` not being skipped by `--no-verify`; `GIT_CONFIG_COUNT`
  outranking repository config; hard-linked local clones on NTFS.
- **Claude Code behaviours taken from documentation, not run in the pinned image:** transcript file
  location and format; `~/.claude/CLAUDE.md` read as user memory; whether the managed remote policy
  reaches a sandbox.
- **The resume fix is in flight** — the conversation-store mount is described in the working-tree
  Dockerfile and not yet in `api/`. `3-D7`'s option (b) depends on it landing as described.
- **Task 26 is a plan.** Every seam id cited is its proposal.
- **The remote host** Panacea projects use is not named anywhere I read (`3-D8`).
- **The prototype was rendered in a stub DOM, not a browser** (§7.3). Visual acceptance is yours.

---

## 7. How the prototype maps to this brief

`prototype/phase-3-convergence.html` — double-click to open. The review bar's menu walks fourteen
states; **Show sources** prints, under each element, the git command, route or record it stands
on (`AGENTIC-UX.md` §2 property 7). "Now" is 23 Sep 15:40, after v1.6.0 shipped and while run
`a90d3f4` builds.

### 7.1 The states

| State | What to look at | Decisions and assertions it shows |
|---|---|---|
| **Merge · Now** | `DefinitionOfDone` at the branch head (obligation 8); two worktrees ready — `upload-ui` lands clean, `upload-api` *"lands after upload-ui and then conflicts with it"*; `upload-api`'s shared-worktree warning; `scan-hook` not ready; landed history with a direct commit that is **not** a land | `3-D2`, `3-D5`, A1, A9; criterion 4 **contested** with its because; criterion 4 carried from rev 5 **where it was criterion 3** — matched by text, not position |
| Merge · Just landed | the land's merge link names `brief:b-42@6`; criterion 3 turns settled because its evidence is now reachable; `1 commit not pushed`; **Undo is exact** | `3-D4`, `3-D6`, A3, §2.4 |
| Merge · Conflict | resolution happens in `upload-api`; *Start a session to resolve* shows the opening prompt before anything starts | `3-D3`, A2 |
| Merge · Branch moved while landing | git's compare-and-swap refused; nothing landed; the forecast is recomputed | A5 |
| Merge · Checked out elsewhere | landing is off, and says why | A5 |
| Merge · Push had no credential | git's own sentence, verbatim; the land and its exact undo survive | A4, `3-D8` |
| Merge · One worktree unreadable | git's sentence; the other worktrees stay current; *Remove worktree* asks first — the one confirmation in these lenses | §2.1, survey §6.2 |
| Merge · Control plane not answering | the last reading, dimmed; landing, pushing and fetching off; the brief shows `unavailable` | contract §4.1 |
| Merge · Nothing marked ready | the zero state, and every worktree offered *Mark ready* | seam S3.3 |
| Merge · New branch | no worktree, and the `none` provenance state — *"no brief on this branch"* | P9 |
| **Files · Now** | changed paths across all three worktrees; `handler.ts` marked **conflicts on landing** (a fact from `merge-tree`); `limits.ts` marked **both edit this file — not committed, so it cannot be forecast** (a suspicion, drawn as one) | `3-D9`, A9, R9 |
| Files · A path open | each worktree's hunk with its **tier 1** back-link (commit → session) and **tier 2** (the transcript span, labelled `3-D7`); an uncommitted hunk says no session is recorded yet | `3-D7`, R8 |
| Files · Nothing changing | the zero state | — |
| Files · One worktree unreadable | its paths withheld, with git's sentence | — |

### 7.2 The sample world — the fact sheet, with one departure

The file draws `prototype-fact-sheet.md`'s world: Engagement Portal, `evidence-upload`, `b-42` at
head revision 6 with criteria B, sessions `upload-api` (`brief:b-42@5`), `scan-hook` (`@5`) and
`upload-ui` (`@6`), and commits `4e7a91c` (v1.6.0), `c81f02a` (v1.5.2) and `a90d3f4` (the run
building now). Lane session counts match the sheet's *"sessions that produced it"* for both
releases. Facts the sheet does not hold — worktree names (one per session, named after it), the
merge commit `e19b7c2`, lane commits, file paths and evidence records — are marked as phase 3
sample under **Show sources**.

**One departure, deliberate, for the reconciler to rule on.** The sheet gives v1.6.0 a merge link
of **rev 5** (sheet §6) and also says `upload-ui` **started after rev 6 was saved** (sheet §5), and
v1.6.0 contains two `upload-ui` commits. Under contract §4.4 a land is reviewed against the
**head** shown above the queue, so any land carrying `upload-ui`'s commits was reviewed against
rev 6. The prototype therefore records `e19b7c2` (the `upload-api` land, before rev 6) as rev 5 and
`4e7a91c` (the `upload-ui` land) as **rev 6**. Two fixes are possible: the sheet's v1.6.0 row shows
two merge links (rev 5 and rev 6), or `upload-ui`'s commits leave v1.6.0. It is a sheet change
either way; this file does not edit the sheet.

### 7.3 Checked mechanically, not by eye

Run on 2026-09-23 with a throwaway Node script (scratchpad, not committed), which also rendered all
fourteen states in a stub DOM and exercised land, undo, reorder, resolve and remove:

| Check | Result |
|---|---|
| `gradient` anywhere in the file | **0** — the half-filled "superseded" disc is a clipped pseudo-element, not a gradient |
| `transition: all` | **0**. Transitions: `background-color` and `color` on rows and buttons only, at `--dur-fast` (phase 2 rule 4) |
| Hex or `rgb()`/`oklch()`/`hsl()` outside `:root` | **0** |
| `1px solid` | **0** — hairlines are `box-shadow` |
| `color: var(--ink-4)` | **0** — `--ink-4` is used for shapes and hairlines only |
| Network references | **0** |
| Radius values | the four tokens, plus `50%` and `1px` for status shapes (fact sheet §12) |
| Heartbeat | one CSS animation, one negative `animation-delay` from one `EPOCH` — `heartbeat.ts`'s mechanism. No `setInterval` |
| Reduced motion | the running disc gains a static outer ring, so running and still-green stay apart (phase 2 rule 6) |
| Status shapes | fact sheet §10: triangle needs-you/contested, half disc superseded, square failed, hollow ring not measured, still disc settled |
| Every render | no `undefined`, `NaN` or `[object` in any state's output |

**Tokens the file declares that `tokens.css` lacks** — `--diff-add`, `--diff-del` and `--src` (the
last is prototype-only chrome) — would be added to `tokens.css` under phase 2's rule 2 before any of
this is ported. Nothing was rendered in a real browser.



