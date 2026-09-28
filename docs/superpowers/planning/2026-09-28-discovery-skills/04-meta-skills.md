# 04 — Meta-skills: how agents read and write

**Date:** 2026-09-28. **Scope:** four of the seventeen aihero.dev skills —
`writing-for-agents`, `writing-great-skills`, `wait-what`, `ask-matt`.
**Catalogue:** https://www.aihero.dev/skills (fetched 2026-09-28).

**Intellectual property.** These are Matt Pocock's published skills. The source repository
https://github.com/mattpocock/skills carries an **MIT licence** (footer, fetched 2026-09-28); the
catalogue page at https://www.aihero.dev/skills states no licence and no terms of use. MIT permits
reuse with attribution, and **we are still not taking the text.** Everything below is the *method*
described in our own words, with the URL it was read at. No prompt text, no wording and no file from
those skills is reproduced here or is to enter this repository or the product. A method is not
protected; a body of text is.

## Fetch status

| Skill | URL | Fetched? |
|---|---|---|
| `writing-for-agents` | https://www.aihero.dev/skills-writing-for-agents | **Yes**, twice (overview, then the five levers in detail) |
| `writing-great-skills` | https://www.aihero.dev/skills-writing-great-skills | **Resolves, but it is the same page.** See the finding below |
| `wait-what` | https://www.aihero.dev/skills-wait-what | **Yes** |
| `ask-matt` | https://www.aihero.dev/skills-ask-matt | **Yes** |

**Catalogue-link caveat, stated plainly.** The catalogue page's own extraction did not surface direct
links for these four; it lists them only in changelog entries. The four URLs above were formed from
the `skills-<name>` pattern the catalogue uses for the skills it does link, and **all four resolved
to real skill pages** — so the pattern is confirmed by the responses, not assumed. One catalogue entry
disagrees with the pattern: the catalogue links `grill-with-docs` without the `skills-` prefix, so the
prefix is a convention rather than a rule.

**Finding: seventeen catalogue entries, sixteen skills.** `writing-great-skills` is the **former name**
of `writing-for-agents`. The page states the rename happened at v1.1 and tells readers to reinstall
under the new name; the two URLs serve the same content (both fetched 2026-09-28). Treat them as one
skill in the catalogue count, and do not plan two evaluations, two installs or two product features.

---

## 1. `writing-for-agents` (formerly `writing-great-skills`)

**What it is, in our words.** A reference a writing agent loads before authoring anything another
agent will read — a skill, an `AGENTS.md` or `CLAUDE.md`, a spec, a prompt, a README. Its stance is
that the default edit is **deletion**, not addition: a line that does not change the reading agent's
behaviour is costing tokens for nothing.
Source: https://www.aihero.dev/skills-writing-for-agents, fetched 2026-09-28.

### The method

It is a **lens, not a pipeline** — there is no question loop, no rounds, no stopping condition of the
kind `grill-with-docs` has. It supplies one test and five levers, and the agent applies them to a
document it is writing or holds.

**The two budgets it optimises, and they pull against each other:**

1. **Context load** — material that is *always* in the reading agent's window: `AGENTS.md` lines,
   skill descriptions. Its stated proportionality rule: material that applies in one case out of ten
   is paying context load in the nine where it does not.
2. **Cognitive load** — the *human's* burden of maintaining the documents and knowing which exist.

Minimising one alone is easy and wrong. Push everything out of context into files and the human
drowns; inline everything and the agent drowns.

**The no-op test, which is the whole thing in one line:** delete the line, and ask whether the
agent's behaviour changed. If nothing changed, the line is a no-op and goes. It is applied sentence
by sentence, not paragraph by paragraph.

**The five levers**, each with its own failure mode:

| Lever | The rule | Failure it fixes |
|---|---|---|
| **Context pointers** | The in-context line that names out-of-context material and encodes *when* to reach for it. The **wording of the pointer**, not the quality of the target, decides whether the agent retrieves it | Good document, never opened |
| **Information hierarchy / progressive disclosure** | Material descends: in-file steps → in-file references → references disclosed behind a pointer. Moving material down deliberately keeps the top legible | The top of the file becomes unscannable and the agent parses explanation before it can act |
| **Completion criteria and legwork** | Every step states its done-condition. A stricter criterion demands more *legwork* — more work the agent must do to satisfy it — and that is the point | Premature stopping: "make it better" cannot be measured, so the agent stops early |
| **Leading words** | Use a compact concept already dense in pretraining (its examples: *tight*, *red*, *tracer bullet*) to anchor both the execution inside the document and the invocation in the pointer | Generic execution; the document fails to shape behaviour |
| **Pruning** | Remove duplication, **sediment** (explanation the model already knows, accumulated over time) and **sprawl** (breadth that dilutes focus). One source of truth | Wasted context; contradictory behaviour where two copies drifted |

**No numeric limits are stated.** The only threshold given is the proportional one-in-ten rule above.
Anyone who wants "keep `CLAUDE.md` under N lines" will not find it here.

**How it says to verify:** run the document, do not admire it. The stated success signals are that the
document got *shorter while getting better*, that no duplication remains, and that a leading word is
observably doing work in more than one context.

**Artifacts.** None of its own. It leaves no `CONTEXT.md`, no ADR, no log. Its output is the edited
document itself.

### PRODUCT — yes, and it is the highest-value item in this file

**Phase 4, task 5 (`contextwrite.Materialise`) and its renderer `internal/claudemd`.** Not as a skill
the user invokes. As a **quality bar the Materialiser's output is held to**, because phase 4's whole
claim is that the generated `CLAUDE.md` is a file "every agent session started there genuinely reads"
(`plans/2026-09-18-phase-4-discover.md:5`). A file the agent reads and ignores passes every test in
that plan and fails the product.

**A concrete defect this lens finds in the renderer as planned.** `claudemd.Render`
(`plans/2026-09-18-phase-4-discover.md:2039-2040`) emits, inside the managed block, two lines that
tell the reader the file was written by Discover and that edits inside the block will be lost. Both
are addressed to a **human**. Apply the no-op test: delete them and no agent behaves differently.
They are paid for in the context window of **every session on that branch, forever**. The fix is not
to delete the warning — a human does need it — but to move it **outside** the managed block, or into
the marker comment, which is where the machine-facing metadata (`brief=`, `rev=`) already lives.
That is one lever (pruning) finding one real cost in one function; the other four have not been run
over the renderer at all.

**What the user would see.** Nothing new in the UI. What changes is that the brief the agent inherits
is written the way an agent reads, so the Develop phase's context chip points at something that
actually steered the session. **Two levers map directly onto phase 4 mechanics we already have:**

- **Completion criteria** → Discover's questions already elicit a definition of done (the same brief
  is "the definition of done in Merge", `plans/2026-09-18-phase-4-discover.md:5`). This skill's point
  is that the done-condition must be *demanding enough to create legwork*. A brief whose acceptance
  criteria are vague produces an agent that stops early — and in our product that stop is visible to
  a whole team. Worth one question in the Discover flow, phrased to force a measurable criterion.
- **Progressive disclosure** → the project→branch→session inheritance chain (task 4) is exactly the
  hierarchy this lever describes. Inherited project sections should descend to a pointer, not be
  re-inlined at every level, or a large project brief is paid for in every branch and every session.

**Not a user-facing skill.** Do not put a "/writing-for-agents" button in the cockpit. It is a rule
for our renderer, not a capability for our user.

### PRACTICE — yes, install it, and point it at three files

Our agents write agent-facing documents constantly: this repo's `CLAUDE.md`, `docs/PROGRAM.md`, the
per-task plan parts, and the skills under `~/.claude/skills`. Three specific targets where the no-op
test would pay for itself immediately:

1. The generated per-branch `CLAUDE.md` (above).
2. This repo's own `CLAUDE.md` — it carries superseded-decision archaeology (`*Was: …*` lines) that is
   valuable to a human auditor and is sediment to an agent that only needs the current decision.
3. The phase plans, which are read by implementer subagents under a watchdog.

**Caveat, and it is real.** This skill's bias is deletion, and some of our repetition is deliberate
and load-bearing — the "`Materialise` runs BEFORE `ptysession.Start`" warning is stated three times on
purpose (`plans/2026-09-18-phase-4-discover.md:42`, `:2139`, and the package comment) because the
failure is silent. That is not duplication to prune; it is redundancy against a silent failure. Apply
the lens, do not obey it. The no-op test answers the question correctly if you ask it honestly:
delete the second copy and an agent reading only the plan body *does* behave differently.

---

## 2. `writing-great-skills` — not a separate skill

**It is the old name of `writing-for-agents`.** The page at
https://www.aihero.dev/skills-writing-great-skills resolves and serves the same content as
`skills-writing-for-agents`; the page records the rename at v1.1 and instructs readers to reinstall
under the new name (both fetched 2026-09-28).

The rename is itself the argument. "Great skills" scoped the reference to one artifact type; "writing
for agents" scopes it by **audience** — anything an agent reads. That is the broader and the correct
frame, and it is the frame our Materialiser sits in.

**PRODUCT: n/a.** **PRACTICE: n/a.** Both judgements are section 1's. The only action here is
bookkeeping: when anyone counts the catalogue or picks skills to install, this is not a seventeenth
item, and installing both would install one skill twice under two names.

---

## 3. `wait-what`

**What it is, in our words.** A one-word repair move for the moment a human stops understanding their
agent. The user types `/wait-what` and the agent re-explains what it just said — shorter, in plain
English, and in the project's *own* vocabulary rather than terms it invented mid-answer.
Source: https://www.aihero.dev/skills-wait-what, fetched 2026-09-28.

### The method

Deliberately tiny — the page describes it as a three-line prompt. Three design choices carry it:

1. **It reports a symptom, not an instruction.** It says "you lost me", not "be brief". Telling a
   model to be brief gets you the same explanation with the connective tissue cut out. Reporting
   incomprehension makes the model diagnose *why* it was not understood, which typically means adding
   a missing premise back, not deleting more words.
2. **The agent chooses the rewind distance.** It is not "rephrase the last paragraph". The agent
   decides how far back the misunderstanding started, and re-pitches from there — usually further
   back than one paragraph.
3. **It reaches for `CONTEXT.md`.** The re-explanation is written in the project's settled glossary
   (the artifact `grill-with-docs` builds), with `CLAUDE.md` supplying the global language standards
   and an optional `CONTEXT-MAP.md` routing to the right context file. Without `CONTEXT.md` the skill
   still runs; it just loses the domain vocabulary and is a generic plain-English re-pitch.

**When it stops:** immediately, after one better explanation. No rounds, no state. It is designed to
be invoked several times in a row without the answers degrading into ever-blunter fragments.

**Its own success test, which is the useful part:** the re-pitch must be *shorter and clearer, not
shorter and blunter* — missing premises added back, invented jargon replaced by the project's terms.

**Artifacts.** None. It writes nothing and is a pure conversational move. It *consumes* `CONTEXT.md`;
it never updates it.

### PRODUCT — yes, small, and it lands squarely on our wedge

Our stated wedge is "usability and iteration on powerful tools", and `AGENTIC-UX.md` assumes the user
is **intermittently present**. Someone who steps away for an hour and comes back to a wall of agent
output is precisely the person this move exists for.

**What the user would see.** On any agent message in the cockpit — and on any Orchestrator escalation
in team mode — one affordance meaning *"you lost me"*. Pressing it re-pitches that message in plain
English, in the branch brief's vocabulary. Three things make it fit rather than merely sound nice:

1. **It does not break the harness rule.** It types a prompt **into** the session's PTY. It does not
   parse PTY bytes and does not reimplement any CLI UI, so it is clear of `PROGRAM.md`'s cross-cutting
   constraint and of `OD-8`.
2. **We already have the vocabulary source.** `wait-what` gets its terms from `CONTEXT.md`. Our
   equivalent is the branch brief that phase 4 materialises — already on disk, already in the
   session's context. Nothing new to build to feed it.
3. **Team mode is where it earns its place.** `OD-16` has one user's Orchestrator escalating a peer
   Orchestrator's reasoning to a human, "either to inform or to ask for a decision". That is
   second-hand reasoning arriving unannounced at a person who was not in the conversation — the
   highest-incomprehension surface in the product. A re-pitch button belongs on the escalation card.

**Where.** Phase 2 owns the message surface, Phase T owns the escalation card. It is a small item on
an existing surface, not a phase of its own, and it should be logged against **Phase T** so it lands
with the escalation card rather than ahead of it.

**Do not overbuild it.** The skill is three lines. Ours should be one prompt and one affordance. The
moment it grows a settings panel it has failed the thing it was copied for.

### PRACTICE — marginal yes for the owner's own sessions, no for workflow agents

For the owner talking to an agent interactively, it costs nothing and repairs a real failure. But it
**overlaps heavily with what is already in force**: the device-wide `i-have-adhd` output shape already
mandates plain language, no jargon and no figurative phrasing. That shape is *preventive*; `wait-what`
is a *repair*. The repair is only worth installing if the prevention is observed to fail.

**No for subagents and workflow agents.** A subagent's final message is a return value to an
orchestrator. There is no human present to say "you lost me", so the skill can never fire.

---

## 4. `ask-matt`

**What it is, in our words.** A router. The user describes where they are stuck, and it names the
skill — or the sequence of skills — to reach for next, then stops. It recommends; it never executes.
Source: https://www.aihero.dev/skills-ask-matt, fetched 2026-09-28.

### The method

- **Hand-maintained map, not discovery.** It routes from a map its author wrote, and does **not** scan
  what is actually installed in your session.
- **User-invoked only.** The agent does not reach for it on its own.
- **It returns the next thing to type, and halts.** Naming the move is the whole output.
- **It teaches two organising ideas**, which are the durable part: **flows** — a main path through
  several skills, plus on-ramps for entering mid-stream, standalones, and a vocabulary layer running
  underneath — and **phase boundaries**, the decision points between phases where the human chooses
  what happens to the context: carry on, clear it, hand off, delegate to subagents, or compact.

**Its own stated limitations, and they are disqualifying for us:** it routes only over that author's
promoted skill set, never over user-created or third-party skills; the hand-maintained map lags the
repository; it sometimes reports installed skills as missing; and it sometimes describes a skill in
terms that do not match what the skill actually does.

**Artifacts.** None.

### PRODUCT — no

**A router is the cure for a catalogue with no spine. We have a spine.** Define → Design → Develop →
Deploy *is* the flow, it is on screen, and the project's current phase already says what comes next.
Adding a "what should I do now?" oracle on top of a visible lifecycle would be an admission that the
lifecycle is not legible — and the right fix for that is to fix the lifecycle, not to bolt a guide
onto it.

**One idea inside it is worth keeping, and it is not the routing.** The **phase-boundary** concept —
that the moment between phases is a decision about *what happens to the accumulated context* — is a
real question our product must answer and currently answers implicitly. When a project moves Discover
→ Develop, or a branch's work is handed to a teammate under `OD-16`, something must decide what
context carries over. We already have the machinery (the brief, the provenance ledger, `OD-8`'s
transcript adapter, phase 2's fork/resume/restart lifecycle). What we do not have is the *named
moment*. Recording it as a design question for Phase 4/Phase T is cheap; building a router is not.

**Verdict: no feature. One design question, logged.**

### PRACTICE — no

Four reasons, any one of which is sufficient:

1. It routes only over its author's skills, and our agents run **superpowers**, **aisthesis**,
   **impeccable** and this repo's own set. It would point our agents at skills we do not have.
2. `superpowers:using-superpowers` already does the job — and it routes over what is *actually*
   installed rather than a hand-written map.
3. Its own page admits the map lags the repository and sometimes misdescribes skills. A router that is
   confidently wrong is worse than no router, because the agent acts on it.
4. It is user-invoked. Our build agents are launched by a workflow script with an explicit task; there
   is no human present to type `/ask-matt`.

---

## Summary

| Skill | PRODUCT | PRACTICE |
|---|---|---|
| `writing-for-agents` | **Yes** — a quality bar on phase 4's `claudemd` renderer and the Discover question set, not a user-facing button | **Yes** — install; point it at the generated `CLAUDE.md`, this repo's `CLAUDE.md`, and the phase plans |
| `writing-great-skills` | n/a — renamed to the above | n/a — do not install twice |
| `wait-what` | **Yes, small** — a "you lost me" re-pitch on agent messages and on Phase T escalation cards | **Marginal** — owner's interactive sessions only; never for subagents |
| `ask-matt` | **No** — one design question logged (the phase boundary), no feature | **No** |

**Two of four are real. One is a duplicate. One is a personal workflow tool with no place here.**

### What to do with this, concretely

1. **Amend phase 4, task 5.** Move the two human-facing lines out of `claudemd.Render`'s managed block
   (`plans/2026-09-18-phase-4-discover.md:2039-2040`) and run the no-op test over the rest of the
   rendered block. This is the one item with a demonstrated cost today.
2. **Add one Discover question** that forces a measurable done-condition, on the completion-criteria
   lever. It also serves Merge, which uses the same brief as its definition of done.
3. **Check the inheritance renderer for re-inlining.** Task 4 inherits project sections into every
   branch; if they are re-inlined rather than pointed at, a large project brief is paid for in every
   session on every branch.
4. **Log the re-pitch affordance against Phase T**, on the escalation card.
5. **Log the phase-boundary question** — what context carries across a phase or a handoff — as an open
   design question for Phase 4 / Phase T.

### Unverified

- The skills' actual prompt text was **not** read and is not reproduced. Every method description
  above comes from the public skill pages, which describe their own behaviour. Where a page did not
  reveal a step, this file says so rather than inferring it.
- The repository licence (MIT) was read from the GitHub page footer, not from the `LICENSE` file
  itself; the file's text was not fetched.
- The catalogue's own link list was not fully extracted, so the four URLs were derived from the
  `skills-<name>` pattern. All four resolved, which confirms them; nothing here depends on a URL that
  did not resolve.
