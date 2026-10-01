# CONTEXT.md — this project's own words

**What this is.** The nouns PanaceaApp uses whose meaning here is **not** the obvious one, so that
neither a person nor an agent has to re-derive the vocabulary from the code. Every entry cites the
file that fixes it, so a definition can be checked and cannot quietly drift.

**What this is not.** Not a dictionary, not documentation, not a glossary of the domain. A term that
means what any engineer would assume is deliberately absent — there is no entry for *commit*,
*token*, *PTY* or *Postgres*. Written 2026-09-28; every term below was grepped in the tree before it
was defined.

**Citations.** A path with no prefix is from the repository root. Paths under
`.claude/worktrees/<name>/` are in a sibling worktree because that phase's work has not landed on
`main` yet; when it lands, the path shortens and the citation still resolves by file and
declaration. Line numbers are given only where the file is stable; prefer the named declaration.

---

## Where work happens

**Machine** — a host that can run sessions: `local` or `cloud`. It is the thing a person names and
switches between, and the **local machine is a shipped product feature**, not a development
stand-in — the Docker backend *is* that feature, and it is badged honestly with what it cannot do.
A cloud machine is **claimed** from a warm pool at first sign-in, with no create form; sizing is
asked only for a second machine or after an OOM.
*Fixed by:* `desktop/src/store/types.ts` `Machine` / `MachineKind` / `MachineState` (UI contract §2,
`docs/superpowers/contracts/2026-09-18-phase-2-ui-contract.md:408-439`); `GET /v1/machines`
(Go contract §3.4, `docs/superpowers/contracts/2026-09-18-phase-2-go-contract.md:1866-1896`);
`docs/PROGRAM.md:43-46`; `docs/FUNCTIONAL-SPEC.md:44-46`.
*Not «environment»* — an **Environment** is one deployed branch (`docs/FUNCTIONAL-SPEC.md:16`), so
"spin up an environment" and "spin up a machine" are two different requests. Not «node», not «VM».

**Project** — one git repository on one machine, **enumerated from disk, never created by us**. It
carries an absolute `path` on that machine, so the same project on two machines is two records.
*Fixed by:* `Project` in UI contract §2 (`…ui-contract.md:443-448`); `internal/project` —
read-only enumeration (Go contract §2.11); `docs/FUNCTIONAL-SPEC.md:11`.
*Not «repo»* in any user-facing text, and not «workspace» — a workspace is nothing here.

**Branch** — the product's main unit of work, not just the git ref: it carries **its own brief, its
own Environment and its own sub-rail of lenses**. Most product sentences that say "branch" mean this
object.
*Fixed by:* `Branch` in UI contract §2 (`…ui-contract.md:450-456`); `docs/FUNCTIONAL-SPEC.md:12`.

**Worktree** — an isolated checkout of one branch that holds **many sessions** and outlives all of
them. Since `OD-2` it is a **local clone** of the project repository, not a `git worktree`; killing
the last session on it does not remove it.
*Fixed by:* `OD-2` (`docs/superpowers/planning/2026-09-23-owner-decisions.md:13`); phase 3 plan
"The model (OD-2)"
(`.claude/worktrees/phase-3/docs/superpowers/plans/2026-09-23-phase-3-git-convergence.md:68-75`);
cardinality in UI contract §2.1 (`…ui-contract.md:617-631`).
*Not «git worktree»* — the word survived `OD-2`, the mechanism did not, and the difference decides
whether `.git` is a file or a directory inside the sandbox. Never a synonym for «session».

**Lane** — the same worktree seen **from the project repository**: the ref
`refs/panacea/lanes/<worktree-id>` the control plane fetches its work back into, plus the
landing-queue row it lands from. A worktree is where an agent works; a lane is what converges.
*Fixed by:* phase 3 plan goal 3 (`…2026-09-23-phase-3-git-convergence.md:28-31`) and the lane syncer
(`:96-100`); `LaneView` / `QueueEntry` in `internal/converge`
(`.claude/worktrees/phase-3/docs/superpowers/plans/2026-09-23-phase-3-git-convergence-parts/task-08.md:112-127`).
*A "lane clone" is the on-disk clone; a "lane" unqualified is the ref and the landing-queue row.*

**Convergence** — landing several lanes into their branch **one at a time, in a human-set order**,
each forecast against the branch *as it will be* after the lanes above it. It is the ordered whole,
including the forecast, the landing queue, the land and the undo.
*Fixed by:* the `Convergence` struct — `GET …/convergence`'s body — (`…phase-3…parts/task-08.md:129-146`);
phase 3 plan goal 4 (`:32-35`).
*Not «merge»* — a **land** is one lane becoming one merge commit (`3-D4`); convergence is the
sequence. Not «integration», which names nothing here.

**Session** — one harness process on one worktree, with its own id, transcript, status and
lifecycle. Many per worktree, and a plain shell is one. Finished sessions stay listed and readable:
the transcript is the artifact.
*Fixed by:* `Session` in UI contract §2 (`…ui-contract.md:529-557`); `docs/FUNCTIONAL-SPEC.md:14`
and Session lifecycle (`:35-38`).
*Not «conversation», «chat» or «terminal»* — the terminal is the pane it renders in, and Resume and
Fork both start a **new** session on a fork of the source's conversation.

**Sandbox** — the ephemeral isolated machine **one running session** runs in: a Kata microVM in the
cloud, a Docker container locally, behind one `Backend` interface. The user never sees the word;
they see "isolated" and blast radius.
*Fixed by:* `docs/FUNCTIONAL-SPEC.md:10`; the `Backend` interface (`docs/PROGRAM.md:34-40`);
`internal/backend` (Go contract §2.2).
*Not «container»* — that is one of the two backends. Not «machine» — many sandboxes live on one.

**Harness** — the **unmodified** agent CLI a session runs: `claude-code`, `codex`, `opencode`,
`gemini-cli`, or `shell`. It is a server-side catalogue entry carrying models, efforts, permission
modes and an `available` flag; the control plane owns argv construction and the CLI is never
reimplemented or wrapped.
*Fixed by:* `Harness` / `HarnessId` in UI contract §2 (`…ui-contract.md:483-520`);
`internal/harness` — the catalogue and argv construction (Go contract §2.9); `docs/PROGRAM.md:129`.
*Not «agent»* — the harness is the program, the agent is what is running in it. Not «tool», not
«model»: a harness offers many models.

---

## What we agree, and what carries it

**Brief** — the durable answer to what a project or a branch is for: a **closed** set of questions
with settled answers, plus criteria. It is a Hub object with numbered immutable revisions, not a
document, and a model-drafted answer is shown, marked, and **not settled**.
*Fixed by:* `control-plane/internal/discover/content.go:1-6` and `Author` (`:57-59`)
(`.claude/worktrees/phase-4/…`); the `brief:` key grammar, Hub contract §3.2.
*Not «spec», «PRD» or «requirements»* — those invite a second, competing description inside one
provenance thread, which is the defect, not the feature.

**Brief revision** — one immutable numbered version of a brief's content, and **the** unit everything
downstream cites: `brief:bf-<16 hex>@<n>`. The Hub numbers revisions; nothing else may.
*Fixed by:* the grammar in `control-plane/internal/provenance/key.go:23-29`
(`.claude/worktrees/phase-4/…`), verbatim from the Hub contract
(`.claude/worktrees/hub/docs/superpowers/contracts/2026-09-23-hub-contract.md:120`).
*Not «version» and not «draft»* — a draft is an unsettled answer inside a revision, not a state of one.

**Criterion** — one line of a definition of done. Its id is minted once and kept for as long as a
line with exactly that text survives edits; phase 3 keys merge evidence to it by brief key + id +
text digest.
*Fixed by:* `Criterion` in `control-plane/internal/discover/content.go:160-166`
(`.claude/worktrees/phase-4/…`); the `serves [{briefKey, criterionId}]` citation shape used by
phase W.
*Not «acceptance criterion» abbreviated to «AC», not «requirement», not «test»* — a criterion is
what must be true, not how it is shown.

**Provenance** — the binding of a thing (a session, a merge commit, an approved design, a release)
to the **brief revision it was done against**. **The thread** is that binding shown end to end
across Define → Design → Develop → Deploy: same artifact, one thread, shown throughout.
*Fixed by:* `CLAUDE.md` §Product spine; `provenance-contract-draft.md` §1.4 *The view every surface
renders* (`:129`) and §2.4 *What `contextRef` now means* (`:215`)
(`.claude/worktrees/phase-3/docs/superpowers/planning/2026-09-23-phases-4-5-7/`).
*Not «traceability»* — that names a property; the thread is an object with records you can open.

**contextRef** — the one opaque string field on a session naming the brief revision it was started
against. **Phase 2 stores and carries it and never interprets it.** `""` means no brief existed;
`none:declined` means a person chose to start without one (`4-D1`).
*Fixed by:* `Session.contextRef` in UI contract §2 (`…ui-contract.md:555-556`); Go contract §7.6
(`…go-contract.md:3045`); `provenance.Declined` in `key.go`.
*Not «briefId» and not «the brief key»* — a brief key is one legal value of `contextRef`, and the
empty string and `none:declined` are the other two.

**The ledger** — the append-only store of provenance **link records**: one record saying *this
subject was bound to this brief key*, never edited, never deleted. It is a store separate from the
brief because its records are written by other phases at other times.
*Fixed by:* `provenance-contract-draft.md:160` and `:168-170`
(`.claude/worktrees/phase-3/docs/superpowers/planning/2026-09-23-phases-4-5-7/`).
*Not «audit log»* (phase 9 builds one, for a different purpose) and *not «history»* — a branch's
landed history is phase 3's undo-able git record.

**Design approval** — the Hub record that a design was approved, written by **one** call that fixes
the design revision, the brief revision it was approved against and the snapshot of its files
together, so an approval cannot exist apart from the thread it was approved on. Phase 5 uploads the
snapshot first, then approves. It is not an *approval row*: that is a decision under **Authority**.
*Fixed by:* Hub contract §7.7 *Design approvals — the design link* (`…hub-contract.md:872`).
*Not «sign-off» and not «review»* — the review is the conversation; the approval is the record.

**Release** — the Deploy end of the thread, and a **reservation**, never a link written afterwards:
the Hub pins the brief revision (or `none`, with a reason) and answers `reserved` **before** the
deploy exists, or Ship does not happen. The control plane then calls Panacea with phase 7's client,
and writes back only the run it became — `confirmed`, else `abandoned` or, unconfirmed, `expired`.
*Fixed by:* Hub contract §7.9 *Releases — reserve, deploy, confirm* (`…hub-contract.md:957`).
*Not «deployment»* — a Deployment is what Panacea runs; the release is our pinned record of it.

**Evidence** — one record saying a named **criterion** is `settled` or `contested` at one commit,
with the sentence saying why. It is keyed by brief key + criterion id + criterion digest, so an
edited criterion line does not inherit the old verdict.
*Fixed by:* Hub contract §7.10 *Evidence — phase 3's H-2* (`…hub-contract.md:1060`).
*Not «proof» and not «test result»* — a test result is one thing the `because` sentence may cite.

**The Hub** — the server `control-plane/cmd/hub`: briefs, design approvals, the provenance ledger, releases
and teams — the shared state `OD-3` put server-side so a teammate sees the same thread. **The
control plane** is the *different*, local process `control-plane/cmd/api` that calls it as the user.
*Fixed by:* Hub contract §Words
(`.claude/worktrees/hub/docs/superpowers/contracts/2026-09-23-hub-contract.md:9-11`); `OD-3` and
`OD-5` (`…owner-decisions.md:14,16`).
*Not «the backend» and not «the API»* — both are ambiguous between two processes that run in
different places under different identities.

**The Materialiser** — phase 4's `contextwrite` package: it writes a session's inherited brief into
the worktree as a managed block in an **untracked** file, **before the harness process starts**.
Called afterwards the session inherits nothing, silently, and the feature still looks built.
*Fixed by:* `control-plane/internal/contextwrite/materialise.go:1-21` (phase 4 task 09,
`.claude/worktrees/phase-4/…`).
*Not «the generator» and not «render»* — `claudemd.Render` produces the text; the Materialiser is
the ordering rule and the on-disk write, and that ordering is the whole point.

---

## What a person looks at

**The cockpit** — the whole Tauri desktop client: the rail, the session panes, Columns and Canvas
layouts, the palette. It is the application, not one screen of it.
*Fixed by:* `docs/PROGRAM.md:78-81` (Phase 2 — The cockpit); the `types.ts` header comment, "The
cockpit's domain model" (`…ui-contract.md:396`).
*Not «dashboard»* — nothing is aggregated and `docs/AGENTIC-UX.md` bans counts outside the attention
queue.
Not «IDE», not «console».

**The rail** — the left navigation, nesting **exactly four levels**: project → branch → worktree →
session, under a machine chip. A worktree row is never a session row, even when it holds one
session.
*Fixed by:* `docs/FUNCTIONAL-SPEC.md:50`; UI contract §2.1 (`…ui-contract.md:617-631`).
The **sub-rail** is the different thing inside a branch: the list of lenses
(`docs/FUNCTIONAL-SPEC.md:51`).
*Not «sidebar» and not «tree»* — both lose the fixed four levels, which is the part a task author
must not undo.

**Lens** — a **view on a branch, not a stage**: no order, no arrows, no numbering, no completion
state, and entry into any lens is legal from any state. The union shipped in code is six —
`discover`, `design`, `develop`, `files`, `merge`, `deploy`.
*Fixed by:* `Lens` in UI contract §2 (`…ui-contract.md:564-566`); `docs/AGENTIC-UX.md:79` rule 12;
the sub-rail list, `docs/FUNCTIONAL-SPEC.md:51`.
*Not «tab», «stage» or «step»*, and *not «phase»* — a **phase** is our build order
(`docs/PROGRAM.md`), invisible to the user; a lens is what the user switches between.
*Answered by the owner, 2026-10-01, in his own answer through the in-session question form ("Accept all"): four lenses, Files and Merge inside Develop (`docs/superpowers/planning/2026-09-30-assemble/00-plan.md` §5 call 9). The type and the sub-rail still carry six until the shell is rebuilt. See §Open questions.*

**Queue** — **two senses, and they must be kept apart.**
1. *The attention queue:* the one ranked list of what needs a human, and the only place in the
   cockpit a count may appear (`docs/AGENTIC-UX.md:71`, §4 rule 4; hard rule 10, `:245`). At 10–30
   agents the interface's job is ranking and batching, not display, so a pane is a cost (§2, `:31`).
2. *The landing queue:* the ready lanes of one branch in the order a human set them to land, each
   row carrying its forecast — `QueueEntry` in `internal/converge`
   (`.claude/worktrees/phase-3/…/task-08.md:119-127`). It belongs to convergence, not to attention.
**Rule:** bare "the queue" means sense 1. Sense 2 is written **"the landing queue"**, never bare —
"counts outside the queue are banned" is a statement about sense 1 only.

---

## Platform words

**Authority** — what an agent may do without asking, **legislated in advance and never one number**:
a grid of action classes (read · edit in own worktree · edit outside worktree · run tests ·
install dependency · run migration · touch infra/config · commit · push · deploy) against four named
levels — **ASK FIRST · PROPOSE AND WAIT · ACT AND TELL ME · FULL**. Never a slider. Defaults inherit
project → branch → agent, the inherited value shown greyed in place, and the level moves at runtime
in both directions: the agent may request elevation and must self-demote on uncertainty.
*Fixed by:* `docs/AGENTIC-UX.md:37` (§2, item 6) and §7 Autonomy and trust (`:177-183`);
`docs/FUNCTIONAL-SPEC.md:52` (the Develop top bar). An **approval row** in the attention queue is a
person's decision on one action an agent asked for (`docs/AGENTIC-UX.md:241`), never a Hub record.
*Not «permission mode»* — `PermissionMode = "ask" | "bypass" | "plan"` (`…ui-contract.md:495`) is a
per-session flag passed to one harness: three values, one axis, no inheritance and no action classes.

**Data classification** — a property set when a **project is created** (and updatable afterwards)
that decides two things: whether that project's inference may use only **AU**-resident providers (Bedrock
in Sydney among them) or **any** provider, and which security scans the deploy side requires.
*Fixed by:* `OD-24` (`docs/superpowers/planning/2026-09-23-owner-decisions.md:152`) and `OD-19`
(`:140`), both amended by `OD-46` (two fixed paths become classes of provider).
*Not «sensitivity label»* — that is a Microsoft Purview object we do not implement — and not
«tenancy», which is about isolation, not about data.

**Orchestrator** — **two senses, and they must be kept apart.**
1. *Product (`OD-16`):* each user's own **Orchestrator agent**, which runs their **sub-agents** —
   agents it started, shown as nested steps (`docs/FUNCTIONAL-SPEC.md:15`), never a cross-user
   channel — and is itself the **only** cross-user channel: orchestrators negotiate peer to peer and
   escalate to their own human, either to inform or to ask for a decision. Accountability and
   billing follow the owner, and no user steers another user's agent.
2. *Our process:* the workflow agent that dispatches build agents and holds the review gates while
   this repository is built.
*Fixed by:* `OD-16` (`…owner-decisions.md:127`) for sense 1; `docs/superpowers/RESUME.md` for sense 2.
**Rule:** in product text, "Orchestrator" always means sense 1. Sense 2 is written **"the workflow
orchestrator"**, never bare.

**Stream** — one line of **our own** build work: a worktree, a branch, a plan, and a workflow that
runs that plan's tasks through review gates (`p3`, `p4`, `p5`, `p7`, `hub`, `env`, `p6`). A stream is
continued, never relaunched fresh — a fresh launch re-runs its gate and loses the work on disk.
*Fixed by:* `docs/superpowers/RESUME.md:96-118` and `:176-184`.
*Never for bytes* — the PTY over the WebSocket is **the attach** or **PTY bytes**, never "the
stream". `CLAUDE.md`'s "streaming back to a single cockpit" is prose about the product, not this
noun.

---

## How we build it

**Seam** — a **named place in phase 2's shipped code where a later phase plugs in**, with an id, a
row in `docs/SEAMS.md` saying what that phase may and may not do there, and a test anchor that fails
if the seam moves. A row without an anchor fails; an anchor without a row fails.
*Fixed by:* `docs/SEAMS.md:1-11` — the definition and its two check commands.
*Not any interface, module boundary, API edge or test-double boundary.* The word is reserved. In
particular, the TDD practice of agreeing where a test cuts is a **test boundary**, never a seam
(`docs/superpowers/planning/2026-09-28-discovery-skills/00-summary.md` §3, practice 7).

---

## Words we do not use

Each row is a word someone will reach for by default, and the word to reach for instead. These are
the ones that have actually caused confusion in this tree.

| Do not say | Say | Because |
|---|---|---|
| **seam**, for any boundary | *test boundary*, *interface*, *package edge* | `docs/SEAMS.md` owns "seam" as an id with a test anchor; borrowing it makes the manifest unfalsifiable |
| **stream**, for PTY bytes | *the attach*, *PTY bytes* | "stream" is one line of our build work (`RESUME.md`) |
| **the backend** / **the API**, for the server | *the Hub* or *the control plane* | two processes, two identities, two hosts (Hub contract §Words) |
| **dashboard**, for the cockpit | *the cockpit* | nothing is aggregated; counts outside the attention queue are banned |
| **the queue**, bare, for landing | *the landing queue* | bare "the queue" is rule 4's attention queue |
| **permission mode**, for authority | *authority* | `PermissionMode` is a three-value per-session harness flag |
| **stage**, **step**, **tab**, for a lens | *lens* | lenses have no order and no completion state (`AGENTIC-UX.md` rule 12) |
| **phase**, for a lens | *lens* (user) / *phase* (our build order) | `docs/PROGRAM.md` phases are invisible to the user. **One exception:** *Phase*, the billing tag beside Job Code and Cost Centre (`OD-47`): Panacea's free-text engagement phase, at most 10 characters, on a project. It is the one user-facing "Phase", and the banned-word scan allows it only as that field's label |
| **spec**, **PRD**, **requirements**, for a brief | *brief* | a brief is a Hub object with numbered immutable revisions |
| **git worktree**, for a worktree | *worktree* (a local clone, `OD-2`) | the mechanism changed and the word did not |
| **agent**, for a harness | *harness* | the harness is the program; the agent is what runs in it |
| **environment**, for a machine | *machine* | an Environment is one deployed branch |
| **merge**, for convergence | *land* (one lane) / *convergence* (the ordered whole) | a land is one git operation inside convergence |
| **briefId**, for `contextRef` | *contextRef* | `""` and `none:declined` are also legal values |
| **Orchestrator**, bare, for ours | *the workflow orchestrator* | `OD-16` owns "Orchestrator" as the product's cross-user channel |
| **ADR directory** (`docs/adr/`) | *`OD-n`* in `2026-09-23-owner-decisions.md` | one decision record, with owner authority |

---

## Open questions — the owner's call, not an agent's

1. **How many lenses are there, four or six?** *Answered by the owner, 2026-10-01, through the in-session question form ("Accept all"): four, with Files and Merge inside Develop (`00-plan.md` §5 call 9); the text below is the question as it stood.* `docs/AGENTIC-UX.md:79` rule 12 says *"Four lenses"*
   (Discover, Design, Develop, Deploy) and forbids sequence. The shipped `Lens` type
   (`…ui-contract.md:564-566`) and the sub-rail (`docs/FUNCTIONAL-SPEC.md:51`) both carry **six**,
   adding Files and Merge. The *rule* — no order, no numbering, no completion state — is not in
   doubt; the **count in that sentence** is. Either rule 12's count is stale and should read six, or
   Files and Merge are something other than lenses and need their own word. This file states six
   because that is what the code and the sub-rail carry, and flags the conflict rather than
   resolving it (§Adding a term, the exception). Note that the conflict is **already inside the
   contract**: the `Lens` comment at `…ui-contract.md:564-565` lists six rows while citing rule 12,
   so whichever way the owner rules, that comment needs the same edit.

2. **Does `CLAUDE.md`'s "Define" mean the Discover lens?** *Answered by the owner, 2026-10-01, through the in-session question form ("Accept all"): yes, and the word is Define, not Discover (`00-plan.md` §5 call 9).* `CLAUDE.md` names the journey
   **Define → Design → Develop → Deploy**; the lens, the phase and the package are all called
   **Discover** (`docs/PROGRAM.md` Phase 4, `internal/discover`). No entry above defines "Define"
   because nothing in the tree fixes it. If they are the same thing, one word should go. `CLAUDE.md`
   also calls the four **phases** (`:5`, `:15`, `:27`), which the table reserves for our build order.

---

## Adding a term

A term belongs here when **our meaning is not the obvious one** and getting it wrong changes what
someone builds. One or two sentences, the file that fixes the meaning, and — where a synonym was
rejected — which one and why, because the rejected word is what someone will otherwise use. If a
term means what any engineer would assume, leave it out: this file earns its place by being short
enough to read in full.

**When a definition and its cited file disagree, the file wins and this file is wrong** — fix it in
the same commit that found it. **The one exception:** when two cited files disagree with *each
other*, or a file disagrees with itself, no entry here can settle it — state what the code and the
shipped surfaces carry, say so in the entry, and record the conflict under Open questions. That is
why **Lens** states six against a rule that says four.
