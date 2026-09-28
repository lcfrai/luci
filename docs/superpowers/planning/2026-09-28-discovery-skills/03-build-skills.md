# Build-side skills — `tdd`, `code-review`, `prototype`, `handoff`

**Date of research: 2026-09-28. All four pages were fetched successfully.** Source catalogue:
<https://www.aihero.dev/skills>, author Matt Pocock, repository <https://github.com/mattpocock/skills>.

**Licence.** None of the four skill pages carries a licence statement. The GitHub repository that
publishes them is **MIT** (read 2026-09-28 at <https://github.com/mattpocock/skills>). MIT would
permit reuse with attribution, but this document does not rely on that: per the research rules,
**nothing is copied**. Every description below is written from the method observed on the page and
restated in our own words. Where a phrase of theirs is load-bearing enough that paraphrase would
lose the point, it is quoted in inline quotes and attributed — quotation for criticism, not reuse.
No prompt text, file or wording of theirs enters this repository or the product.

**URL derivation, honestly.** The catalogue's featured table uses the pattern `/skills-<name>`. Its
*category* pages (for example `/the-main-flow-jnjkc`, `/upkeep~6heor`) **both returned HTTP 404**
when fetched on 2026-09-28, so the category listings could not be read and the four URLs below were
derived from the featured-table pattern instead. All four resolved:

| Asked for | Their own name for it | URL, fetched 2026-09-28 | Fetched? |
|---|---|---|---|
| `tdd` | `/tdd` | <https://www.aihero.dev/skills-tdd> | yes |
| `review` | **`/code-review`** | <https://www.aihero.dev/skills-review> | yes |
| `prototype` | `/prototype` | <https://www.aihero.dev/skills-prototype> | yes |
| `handoff` | `/handoff` | <https://www.aihero.dev/skills-handoff> | yes |

## Verdicts, up front

| Skill | PRODUCT (a capability in PanaceaApp) | PRACTICE (our build agents use it) |
|---|---|---|
| `code-review` | **Yes — strongest of the four.** Phase 3 diff review gains a second, unmerged axis fed by the brief. Provenance made visible at review time. | **Yes, but scoped** — Critical-bearing and whole-branch reviews only. Costs an agent per round, which fights `OD-31`. |
| `prototype` | **Yes** — phase 4/5 boundary: an open question in the brief carries a runnable, disposable answer. | **Yes — cheapest to adopt today.** Two-artifact rule and the never-merge branch. |
| `handoff` | **Qualified yes, but not where it looks.** Not the cross-user channel. Yes as the session-fork/resume baton in phase 2 and phase T. | **Yes, narrowly, with their trigger rejected** — see below; a watchdog-killed agent never gets to write one. |
| `tdd` | **No** as a headline surface. One small piece (seam agreement as an approval) is optional later. | **Yes, one pre-step only** — a blocking seam agreement before the first test. |

---

## 1. `tdd` — <https://www.aihero.dev/skills-tdd> (fetched 2026-09-28)

**What it does.** Drives a feature or a bug fix as a red-green loop, and puts a human confirmation
gate in front of the very first test so that the boundary being tested is agreed before any test
file exists.

**The method, as the page states it.**

1. **Seam agreement, and it is step one.** Before writing anything, the agent names the public
   boundaries it intends to test at and **stops** for confirmation. The page states the rule
   absolutely: "no test at an unconfirmed seam".
2. **Red.** One failing test, one behaviour.
3. **Green.** Only the minimum code that passes that one test.
4. **Repeat** for the next behaviour.

**There is no refactor phase, and the stated reason is empirical, not aesthetic:** the page says it
was dropped in June 2026 "because agents essentially never performed it", and refactoring was moved
into `code-review` instead. Standing rules: mocks only at system boundaries (external APIs, time,
randomness, sometimes filesystem or database) and never one's own modules; vertical slicing — one
seam, one test, one minimal implementation. Three named anti-patterns it exists to prevent:
implementation-coupled tests, tautological tests, horizontal slicing.

**Branching.** It only accepts work that is already a concrete behaviour with defined inputs and
outputs. If the specification is unclear it hands off to `to-spec`; if the question is the shape of
the interface, to `codebase-design`; if the job is a full implementation run, to `implement`.

**Artifacts.** None of its own. The page calls it stateless: the test file and the implementation
are ordinary outputs of the cycle, not a record the skill keeps.

**What it does that our method does NOT.** We already enforce red-before-green. Two real deltas:

- **A blocking pause that names the test boundary before the first test exists.** Ours starts at
  "write the failing test", so the boundary is chosen silently by whichever agent got the task, and
  is only visible afterwards in review. This is the one genuinely new move, and it is cheap.
- **No refactor phase at all.** Ours keeps one.

**A conflation to avoid.** We already have `docs/SEAMS.md`, and it uses "seam" in a different sense:
a place in phase 2's shipped code where a **later phase** plugs in, with a manifest test that fails
when a row and an anchor disagree. Their seam is a **test boundary**. Adopting their step must not
overload our word; if we adopt it, call it a *test-boundary agreement*.

**PRODUCT — no, not as a surface.** How an agent writes its tests is the agent's business, and
exposing a TDD ceremony as a platform feature adds configuration for no user-visible gain, which is
exactly the bar `CLAUDE.md` says we fail. The one piece with product shape is small and optional:
the work layer (phase W) could let a task carry "test boundary agreed: `<names>`" as a recorded
decision on the provenance thread, so a reviewer can see what the author thought the boundary was.
Park it; do not plan it now.

**PRACTICE — yes, one pre-step.** Add a test-boundary agreement to our existing TDD gate: the
implementing agent names the boundaries in its task report and the reviewer checks the tests landed
there. Written by us, in our words. Do **not** copy their no-refactor ruling on their evidence — it
is their observation about their agents. If we want it, test it against our own record first.

---

## 2. `code-review` — <https://www.aihero.dev/skills-review> (fetched 2026-09-28)

**What it does.** Reviews a diff against a fixed reference point along **two axes that never meet**:
*Standards* asks whether the code is written the way this repository writes code, *Spec* asks
whether it does what the originating issue or specification asked for. It deliberately refuses to
produce a combined verdict.

**The method, as the page states it.**

1. The user names a **fixed point** — a commit, branch, tag, `main`, or a relative ref such as
   `HEAD~5`.
2. **Pre-check before any work.** The skill verifies the ref resolves and the diff is non-empty, and
   stops there if either fails, before spawning anything.
3. **Two sub-agents run independently and do not see each other's reasoning.**
   - *Standards* reads the repository's own documented standards (`CODING_STANDARDS.md`,
     `CONTRIBUTING.md` and similar) and falls back to a built-in baseline only where none exist.
   - *Spec* finds the specification by a fixed priority order: issue references in commit messages
     (`#123`, `Closes #45`) → a path given as an argument → a spec file matching the branch or
     feature name under `docs/`, `specs/` or `.scratch/` → ask the user.
4. **Report** in two labelled sections, worst issue named per axis.

**The rules that make it work, and they are the interesting part.**

- **Findings are never merged and never re-ranked across the axes, and no overall winner is
  declared.** The page's justification is the whole argument: code that follows every convention
  while implementing the wrong thing passes Standards and fails Spec.
- **The repository always overrides.** Its own documentation is the primary authority on Standards;
  the built-in baseline is twelve code smells from Fowler's *Refactoring* ch. 3, and each is applied
  as a labelled heuristic ("possible Feature Envy"), explicitly never a hard violation.
- **Every finding carries a citation.** A Standards finding names a repo rule or one of the twelve
  smells **and quotes the hunk**; a Spec finding **quotes a line of the spec**.
- **"No spec available" is a permitted output.** With no specification located, the Spec section says
  so rather than inferring what the requirements must have been.
- **Three-dot diff** (`<fixed-point>...HEAD`), so staged and uncommitted work is out of scope.
- The page admits a live bug: sub-agents can re-invoke the skill recursively, unresolved in the
  shipped version. Worth knowing before we imitate the shape.

**Artifacts.** None persisted. A console report with a `## Standards` section and a `## Spec`
section, and a closing summary that names the worst issue on each axis without picking between them.

**What it does that our method does NOT.** We run adversarial review with Critical / Important /
Minor severities and a single ranked list, and our reviewers cite the plan part. Three deltas:

- **The refusal to merge the axes, or to declare a winner.** Our single ranked list silently trades
  the two questions against each other. Concrete failure our process can produce: a task that
  satisfies every citation in its plan part while breaching `docs/DESIGN-SYSTEM.md` lands one Minor
  and is deferred; the inverse — immaculate style, wrong feature — can rank the same way. Two lists
  that both have to be answered cannot do that.
- **The fixed point, validated, with an empty-diff stop.** `docs/superpowers/RESUME.md` records that
  "plan re-reviews do not converge" and that residual Important and Minor findings are carried
  across rounds. A named fixed point per round makes "what actually changed since the last round" a
  mechanical question instead of a judgement.
- **"No spec available" as a legitimate answer.** Our reviewers always have a plan part, so this
  looks irrelevant — until the whole-branch close-out review, where no single part covers the diff
  and the reviewer currently has to invent its own frame.

**PRODUCT — yes, and this is the strongest of the four.** Phase 3 already ships diff review with
transcript back-links. Two-axis review maps onto our spine exactly: **the Spec axis *is* the brief
written in Define**, pulled from the Hub's brief store. What the user sees on a lane's diff: two
columns that are never combined into a score — *follows how this repo writes code* and *does what
the brief asked* — every finding in the left column quoting a repo rule and a hunk, every finding in
the right column quoting a line of the brief. That is the provenance thread doing visible work at
the moment it matters most, and a release in Deploy can carry both verdicts. It also gives the
canvas and the work layer a shared review vocabulary. Plan it into phase 3's diff review, with the
brief pointer supplied by the Hub.

**PRACTICE — yes, but scoped, because it costs a round.** Split our reviewer into a
plan-faithfulness pass and a repo-standards pass that do not see each other, and stop producing one
ranked list across both. That is one more agent per review round, which runs straight into `OD-31`'s
instruction to cut spend and hold reviews to one round. Recommendation: apply the split to
**whole-branch close-out reviews and any review that has produced a Critical**, and leave ordinary
task reviews single-pass.

---

## 3. `prototype` — <https://www.aihero.dev/skills-prototype> (fetched 2026-09-28)

**What it does.** Writes deliberately disposable code whose only job is to settle one design
question, and keeps the *answer* and the *code* as two separate artifacts that go to two different
places.

**The method, as the page states it.**

1. **The question comes first, and it decides the shape of everything after it.** The page is blunt
   that a prototype answering the wrong question is pure waste however good it looks, and that one
   question is the unit — "what is the whole app?" is not a question it accepts.
2. **The question picks one of two branches.**
   - *Logic or state model* → **one self-contained HTML file**, no build step and no server: a
     labelled state panel that re-renders after every interaction, free-play buttons for poking the
     model, tabbed guided walkthroughs that replay an ordered button sequence per scenario, labels in
     domain language so a non-technical person can drive it, and the logic kept as a pure module
     separate from the DOM so it can be reused later.
   - *Visual design* → **radically different** variations, switched by a floating bar and a
     `?variant=` URL parameter, rendered against real data and real density. Variants must differ
     structurally: the page's test is that three tweaked card grids is wallpaper, not a prototype.
3. **Stop conditions, and they are unusually concrete.** One sitting — still building a day later
   means the question was too big and should be split. The answer fits in one sentence. A
   non-technical stakeholder can describe what they are looking at in their own words. Someone spots
   an idea-level bug ("that shouldn't be possible").
4. **Hardening is the stop signal.** The moment you add a test, wire the real database, or
   generalise for a case you might want later, you have stopped prototyping.

**Artifacts — two of them, and this is the part worth stealing.**

- **The answer**: the verdict plus the question it settled, recorded in a commit message, an ADR or
  the implementation issue. This is what merges to `main`.
- **The prototype**: committed to a throwaway `prototype/<name>` branch off `main` and **never
  merged**, with a context pointer on the implementation issue linking back to it. Main stays clean;
  the exploration stays findable and re-runnable.

**What it does that our method does NOT.** Our working agreement already says the owner reviews
prototypes before implementation plans. Three deltas:

- **The two-artifact split with a never-merge branch and a back-link.** Our prototypes live loose.
  The answer and the code have the same fate today, which means either the code pollutes `main` or
  the answer is lost with it.
- **The structural-variation rule.** `docs/DESIGN-SYSTEM.md` exists because the first prototype
  failed by looking machine-generated. "Three tweaked card grids is wallpaper" is a one-line test for
  precisely that failure, applied at prototype time rather than at review time.
- **The single self-contained HTML file with a state panel and guided walkthroughs.** This matters
  more for us than for them: `CLAUDE.md` records that the corporate network blocks headless-browser
  screenshots and that visual acceptance is the owner's eyes. A single file the owner opens locally
  and drives by button is the one acceptance channel that survives our network constraints, and a
  state panel makes a state machine reviewable without reading code.

**PRODUCT — yes, phase 4/5 boundary.** The surface is not "we have a prototyping tool"; it is **an
open question in the brief carries a runnable answer**. In Discover, a question the conversation
cannot settle becomes a prototype request; Design produces the disposable artifact; the one-sentence
answer is written back onto the brief as a provenance entry and the artifact is linked, not merged.
Keep it distinct from the canvas (`OD-11`): the canvas edits real HTML frames that become the design;
the prototype is a throwaway answer-machine attached to a question. Concretely, the user sees an
unresolved question in the brief with a *Prototype* action, and later the same question showing a
settled answer plus a link that still runs.

**PRACTICE — yes, and it is the cheapest of the four to adopt.** Two changes, both today:
the two-artifact rule with a `prototype/<name>` branch that is never merged and is linked from the
task, and the structural-variation test added to our own prototype guidance in our own words.

---

## 4. `handoff` — <https://www.aihero.dev/skills-handoff> (fetched 2026-09-28)

**What it does.** Compresses the conversation you are currently in into one markdown file so the
work can be picked up somewhere else. Its own framing is that it optimises for **portability, not
compression**: it is for when work has to physically travel, not for when a context is merely full.

**The method, as the page states it.**

1. The user invokes it **with a note saying what the next session is for** — and that note is what
   decides which details survive the compression.
2. The agent identifies the live thread, the context and the pending work.
3. **Secrets are redacted before the file is written.**
4. It writes the thread, the current reasoning and blockers, and the next steps.
5. It inserts **references** — paths and URLs to specs, plans, ADRs, issues, commits and diffs —
   **and deliberately does not carry anything already written down elsewhere.** That is what keeps
   the file small and keeps the settled detail in one place.
6. It adds a section naming the skills the next agent should use.
7. It writes one markdown file **to the operating system's temporary directory**.
8. It stops. **It does not perform the transfer** — the human carries the path to the new session.

**Branching.** Four triggers: a harness swap, a move to another directory or repository, a colleague
picking the work up, or a parallel fork where this session continues and a copy goes to a second
agent. The page is explicit that the document is a **secondary source** — a summary of the
conversation, not the conversation — which is why staying in session is preferred when it is
possible at all.

**Artifacts.** One markdown file in the OS temp directory. Nothing committed.

**What it does that our method does NOT.** `docs/superpowers/RESUME.md` is already a handoff
document, and a far heavier one: committed, per-workflow, carrying ledger state. Three deltas, all
real:

- **The destination shapes the content.** Their note — *what the next session is for* — is supplied
  before compression and decides what survives. Ours compresses against a fixed template, so a
  resume written for a review round and one written for a build round contain the same things.
- **References, never copies.** RESUME.md copies settled detail, and we have already measured what
  that costs: `docs/SEAMS.md` records that on 2026-09-28, of twenty-two citations resolved by hand,
  every one pointed at the wrong line because other chains had grown the files. A second, unchecked
  copy of a fact rots. A baton that carries only pointers cannot rot the same way.
- **Temp, not committed.** Ours is committed, which is right for a program ledger and wrong for a
  per-session baton — a committed baton invites edits and becomes another thing to reconcile.

**Where it is WEAKER than what phase T needs, and this is the important judgement.** The task flags
`handoff` as the interesting one because passing context between agents is phase T. It is not the
model for that, and `OD-15` and `OD-16` are why:

- `OD-15` requires that **agents see facts, not words** — a prose summary is words, and a
  second-hand summary at that, by the skill's own description.
- `OD-16` requires **continuous negotiation between Orchestrators**, escalating to inform or to ask.
  A handoff file is one shot, one direction, point to point, and needs a human to carry it.
- `OD-16` also forbids one user steering another user's agent. A document that carries "next steps"
  into someone else's agent is exactly the shape of that, if it ever crossed a user boundary.

So: **reject `handoff` as the cross-user channel.** It fits one narrow slot *inside* phase T — what
an Orchestrator hands **its own** sub-agent when that sub-agent is replaced or forked. Same owner,
same accountability, no boundary crossed.

**PRODUCT — qualified yes, and not where it first looks.** Not the team channel. Yes as the
**session baton**: phase 2 already has fork and resume in the session lifecycle, and today a fork
starts cold. What the user sees: forking or resuming a session offers to carry the thread, and the
new session opens with a baton already loaded that **references** the brief, the lane and the last
commits rather than repeating them. The provenance value is concrete — the baton's references are
ledger entries, so the thread survives a session boundary, which today it does not.

**PRACTICE — yes, narrowly, and their trigger must be rejected.** Our workflow agents die mid-run
(spend caps, restarts, the 180-second watchdog) and currently re-derive their state from a committed
ledger. A destination-shaped, references-only baton would beat re-deriving. But their method writes
the file **on request, at the end** — and an agent killed by a watchdog never reaches the end, which
is precisely the case we need it for. Adopt the **shape** (destination note first, references not
copies, secrets redacted, uncommitted scratch) and **reject the trigger**: write it incrementally as
the agent works, so the last good version already exists when the agent dies.

---

## Two things to carry into the plan amendment

1. **`code-review`'s two unmerged axes are the one product-shaped idea here**, because the Spec axis
   is our brief and the citation rule is our provenance thread. It belongs in phase 3's diff review.
2. **`prototype`'s two-artifact rule is the one practice change worth making today**, and it costs
   nothing: an answer that merges, code on a branch that never does, linked from the task.

*No text, prompt or file from <https://www.aihero.dev> or <https://github.com/mattpocock/skills> was
copied into this repository. Methods described from the pages listed above, all read 2026-09-28.*

