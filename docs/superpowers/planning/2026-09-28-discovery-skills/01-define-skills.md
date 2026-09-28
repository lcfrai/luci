# Define-phase skills — method extraction

**Researched 2026-09-28.** Brief: study `grill-me`, `grill-with-docs`, `domain-model`, `to-prd`, `to-spec`
from Matt Pocock's catalogue, extract the **method**, and judge each twice — as a PanaceaApp
capability (PRODUCT) and as a skill our own build agents install (PRACTICE).

## Provenance and licence

| Item | Value |
|---|---|
| Catalogue | <https://www.aihero.dev/skills> — fetched 2026-09-28 |
| Source repository | <https://github.com/mattpocock/skills> — fetched 2026-09-28 |
| **Licence** | **MIT**, file named `LICENSE` at the repo root (read from the repo page, 2026-09-28) |
| Author | Matt Pocock |
| Install methods the catalogue advertises | `npx skills@latest add mattpocock/skills`, or `claude plugins install mattpocock-skills` |

**IP position.** MIT permits reuse with attribution, but this document deliberately does not rely on
that. **No text of theirs has been copied into this repository, and none may be copied into the
product.** Everything below is the *method* described in my own words, with the URL it was read at.
A workflow shape is not protected; a body of prose is. If we build any of this, we write our own.

The marketing pages were read, but the **authoritative source for every method claim below is the
skill definition file in the MIT repo**, fetched raw and read in full. Where the marketing page and
the definition file disagree, the file wins and I say so.

## Two corrections to the brief's catalogue list — read these first

**1. `to-prd` no longer exists. It is `to-spec`.** <https://www.aihero.dev/skills-to-prd> resolves
(HTTP 200) but serves the `/to-spec` page, which states the skill was renamed in v1.1 and that the
`to-prd` slug is dead. There is no `to-prd` folder in the repo. Verified 2026-09-28. **They are one
skill, not two** — the brief asks for five and there are four.

**2. `domain-model` is not a page and not a user-invoked skill.**
<https://www.aihero.dev/skills-domain-model> resolves (HTTP 200) but serves the `grill-with-docs`
page. The real thing is `domain-modeling`, a **model-invoked discipline** the agent reaches for on
its own — `skills/engineering/domain-modeling/SKILL.md`, plus two format files. It is the half of
`grill-with-docs` that writes the files. I treat it as its own entry below because it is the single
most valuable idea in the set for us.

A control check was run: a deliberately bogus `aihero.dev` slug returns a real HTTP 404, so the two
200s above are genuine same-host redirects, not a soft-404 fallback. The finding is sound.

## The shape of the set, before the individual skills

Four files, three layers, and the layering is itself worth noting:

- **`grilling`** — model-invoked discipline. The interview engine. All the method is here.
- **`domain-modeling`** — model-invoked discipline. The writing engine. All the file rules are here.
- **`grill-me`** — user-invoked. A **one-line file** that does nothing but call `grilling`.
- **`grill-with-docs`** — user-invoked. A **one-line file** that calls `grilling` *and* `domain-modeling`.

Both user-facing entry points carry `disable-model-invocation: true`: they only ever run because a
human typed them. The disciplines underneath carry no such flag and fire on their own when the topic
fits.

**The transferable idea, independent of any of the content:** a user-invoked skill is a *composition*
of model-invoked disciplines and holds no method of its own. Two entry points, one engine, zero
duplication. Our own `.claude/skills` do not work this way, and the duplication cost of that shows up
the moment two skills need the same discipline.

---

## 1. `grill-me` — the interview, with no paper trail

**Page:** <https://www.aihero.dev/skills-grill-me> — **fetched**, 2026-09-28.
**Definition:** `skills/productivity/grill-me/SKILL.md` and `skills/productivity/grilling/SKILL.md`,
MIT repo — **fetched raw and read in full**, 2026-09-28.

**What it does, in my words.** It interrogates a plan you already have in your head until the agent
and you demonstrably mean the same thing by it. It writes nothing anywhere; the only output is that
your idea is sharper and the agent now shares it.

### THE METHOD — this is the part worth having

The method is a **design tree worked in rounds**, and it has exactly four moving parts.

1. **The tree.** Every decision branches into the decisions that hang off it. The interview is a
   traversal of that tree, not a list of questions.
2. **The frontier.** The frontier is *every decision whose prerequisites are already settled* — the
   questions answerable **now**, without the agent guessing at an answer it has not yet heard.
3. **The round.** The agent asks **the entire frontier in one round**, numbered, **each question
   carrying the agent's own recommended answer**, then stops and waits. It does not ask one question
   at a time. A question whose answer depends on another question *still open in this same round*
   is illegal in this round and belongs to a later one. The marketing page offers "forty-six
   questions across four rounds" as an ordinary session, so rounds are large.
4. **Recompute, repeat.** Each set of answers settles decisions, which pushes the frontier outward
   and unblocks what depended on them. Recompute the frontier; that is the next round.

**How it decides what to ask next:** it does not choose by importance or by topic. **Dependency
order alone.** The next round is mechanically whatever became answerable.

**When it stops:** when the frontier is empty — every branch visited, nothing silently assumed. Then
it stops again, deliberately, and does not act until the user confirms shared understanding. Two
gates, not one.

**The facts/decisions split, and it is the rule I would steal first.** Finding *facts* is the
agent's job and never the user's. If a frontier question needs a fact from the environment, the agent
**dispatches a sub-agent to go and find it** rather than asking. Crucially it **does not block**: an
in-flight lookup is just an unsettled prerequisite, so only the questions downstream of it wait, and
the rest of the frontier is asked immediately. *Decisions* are always the user's, put to them and
waited on.

**Artifacts:** none. Explicitly stateless — no files, no workspace. The page says the only residue is
a sharper idea in your own head, and names a known limit: some questions are ungrillable because they
need something to react to, which is a prototype's job, not an interview's.

### PRODUCT — yes, phase 4 (Discover), as a second mode

Phase 4 today is a **closed set of five questions** plus at most one model-proposed follow-up
(`docs/superpowers/plans/2026-09-18-phase-4-discover.md`): purpose, audience, what must never break,
definition of done, out of scope. That is the right on-ramp and it should not change — it is
answerable in a minute and it is what makes the brief exist at all.

What it cannot do is go deep. A five-answer brief for a real project is thin, and the thinness is
invisible until an agent makes a wrong call three days later. **Add "Grill this brief" as an optional
second mode after the five questions are answered**: the agent reads the brief plus the repo scan
`repoanalysis` already produces, and runs frontier rounds over it. What the user sees is a round of
numbered questions, each with a pre-filled recommended answer, which they can accept wholesale,
override individually, or leave for later. Answers flow back into the brief and into the same
`CLAUDE.md` the five questions already write.

**It suits our user model better than it suits Matt's.** `docs/AGENTIC-UX.md` assumes the person is
intermittently present. A round is a batch you answer when you come back; a one-question-at-a-time
chat is a leash. The frontier rule is what makes batching safe.

**One hard design constraint, and it will be missed if it is not written down.** Phase 4 permits
exactly one denominator, `QUESTION n OF m`, and it is permitted *only* because the base set is closed
and cannot grow mid-flow. **Grilling rounds have no closed count.** Any "question 7 of 20" in this
mode is a lie the system cannot defend and is banned by `docs/AGENTIC-UX.md` anti-pattern 2. Label
the round, never the total.

### PRACTICE — yes, and it is the cheapest win in this document

We already interview the owner before big builds, and memory records the lesson that we should do it
more (`talk-before-big-builds`). What we do **not** do is the frontier rule, and that is the part
that pays: asking a question whose answer depends on an answer we have not heard is how a planning
round produces a decision that a later round invalidates, which is a plan amendment. The other
directly applicable rule is facts-are-the-agent's-job: we have sub-agents and a 180s watchdog, so
dispatching a lookup and continuing to ask the rest of the frontier is already our execution shape.

Write our own, do not install theirs — a device-wide skill install would also need to respect the
claudezsh rule and the KPMG managed policy, which their file knows nothing about.

---

## 2. `domain-modeling` — the glossary and the decision gate

**Page:** no page of its own. `https://www.aihero.dev/skills-domain-model` **fetched**, 2026-09-28,
and it serves the `grill-with-docs` page instead.
**Definition:** `skills/engineering/domain-modeling/SKILL.md`, `ADR-FORMAT.md`, `CONTEXT-FORMAT.md`,
MIT repo — **fetched raw and read in full**, 2026-09-28.

**What it does, in my words.** It is the discipline of *actively* maintaining a project's language
while you are designing: pushing back on words that clash with the glossary, forcing one canonical
term where several are floating, and writing each term down the instant it settles. It explicitly
draws a line between *changing* the model (this skill) and merely *reading* the glossary (a habit
any skill can have).

### THE METHOD — five in-session behaviours and two write gates

**In-session, while the conversation runs:**

1. **Challenge against the glossary.** A term used in a sense that contradicts the existing entry is
   called out on the spot: your glossary says X, you appear to mean Y, which is it.
2. **Sharpen fuzzy language.** An overloaded word gets a proposed canonical replacement — is
   "account" the Customer or the User, because they are different things.
3. **Stress-test with invented scenarios.** Concrete edge cases, made up on purpose, to force
   precision about where one concept ends and the next begins.
4. **Cross-reference with the code.** When the user asserts how something works, check whether the
   code agrees, and surface the contradiction rather than accepting the assertion.
5. **Write inline, never batched.** The moment a term resolves it goes into the glossary. Not at the
   end of the session — at the moment.

**Write gate 1 — what earns a glossary entry.** `CONTEXT.md`, and the rules are strict:
one or two sentences, define what a thing **is** and not what it does, be opinionated by picking one
word and listing the rejected synonyms explicitly as ones to avoid, and — the rule that makes it
work — **only terms specific to this project**. General programming vocabulary is excluded even when
the project is full of it. And the boundary: `CONTEXT.md` is **totally devoid of implementation
detail**. It is not a spec, not a scratchpad, not a decision log. A glossary and nothing else.

**Write gate 2 — the exact test a decision must pass to earn an ADR.** All three, simultaneously:

1. **Hard to reverse** — changing your mind later costs something real.
2. **Surprising without context** — a future reader will look at the code and ask why on earth.
3. **The result of a genuine trade-off** — there were real alternatives and one was chosen for
   stated reasons.

Miss any one and there is no ADR. The reasoning given for each is the sharp part: if it is easy to
reverse you will simply reverse it; if it is not surprising nobody will ever wonder; if there was no
alternative there is nothing to record beyond "we did the obvious thing". The repo also lists what
typically passes — architectural shape, how two contexts integrate, technology choices carrying real
lock-in (not every library, only the ones that would take a quarter to swap), ownership and scope
boundaries including the explicit no's, deliberate deviations from the obvious path, constraints that
are invisible in the code such as a compliance rule or a latency contract, and rejected alternatives
whose rejection was subtle enough that someone will re-propose them in six months.

**The ADR format is one paragraph.** Title, then one to three sentences: context, decision, why.
Status, considered options and consequences are optional and most ADRs are expected not to need them.
The stated value is recording *that* a decision happened and *why*, not filling in a template.
Numbering is sequential, `0001-slug.md` upward, by scanning the directory for the highest.

**Artifacts and where they live.** `CONTEXT.md` at the repo root; `docs/adr/NNNN-slug.md`. Multi-context
repos are signalled by a `CONTEXT-MAP.md` at the root, which lists each context, where its own
`CONTEXT.md` lives, and — the useful part — the **relationships between contexts** stated as directed
arrows with the mechanism named. Every file is created **lazily**: nothing exists until there is a
first real thing to write into it.

### PRODUCT — yes, and this is the strongest single idea in the set for us

**Verified gap:** the phase 4 plan contains **zero** mentions of a glossary, of `CONTEXT.md`, or of
any decision record. It writes a five-answer brief into `CLAUDE.md` and stops. So the provenance
thread currently carries *intent* and carries no *language* and no *decisions* — and language and
decisions are exactly what a coding agent gets wrong when it is left alone for an afternoon.

Concretely, what the user would see:

- **A "Language" surface in Discover**, sitting beside the five questions. It fills itself as the
  brief is answered and as Develop sessions run: when an agent proposes a term, or catches the user
  using two words for one thing, the term appears there for one-click accept, rename or reject.
- **The glossary is materialised into the worktree with the brief**, by the same
  `contextwrite` atomic write and under the same seam hazard — on disk *before* the PTY starts, or
  the session silently inherits nothing.
- **Decisions become ledger entries, not `docs/adr/` files.** We already have a provenance ledger in
  the Hub (phase H). An architectural decision taken during Develop is exactly a ledger entry that
  the release in Deploy can be traced back to. **Adopt their three-part test verbatim as the gate on
  what is allowed to become one** — it is the only mechanism I have seen that reliably keeps a
  decision log from turning into a diary.
- **Multi-context via `CONTEXT-MAP.md` maps onto our project → branch → session inheritance** and
  should be checked against `discover.inherit` when this is planned, not before.

The one thing to resist: their glossary lives in the repo the agent is editing. Ours must live in the
Hub and be *projected* into the worktree, because `OD-3` puts shared state server-side and a glossary
that only exists in one person's worktree breaks the thread the moment a teammate opens the project.

### PRACTICE — half yes, and the half matters

**The glossary: yes, and we are missing it.** This repo has a genuine vocabulary problem — machine,
project, branch, worktree, lane, session, brief, harness, sandbox, cockpit, rail, Hub — and the
definitions live scattered across `CLAUDE.md`, `PROGRAM.md`, `FUNCTIONAL-SPEC.md` and thirty
planning documents. A single root `CONTEXT.md` under their rules (project-specific terms only, one
or two sentences, explicit rejected synonyms) is a small, bounded, high-value piece of work.

**The ADR files: no.** We already have decision records and they are better suited to us than
`docs/adr/` would be: `docs/superpowers/planning/2026-09-23-owner-decisions.md` carries `OD-1`…`OD-35`
with owner authority, and `CLAUDE.md` states plainly that they win over anything older. Adding a
parallel `docs/adr/` tree would split the record in two, which is the failure mode `PROGRAM.md`
already documented when the phase table and the phase prose disagreed. **Take the three-part test and
apply it to what earns an `OD`; leave the file layout alone.**

---

## 3. `grill-with-docs` — the interview that leaves a paper trail

**Page:** <https://www.aihero.dev/grill-with-docs> — **fetched**, 2026-09-28. (The catalogue links it
at that slug, not at `skills-grill-with-docs`; `skills-domain-model` also lands here.)
**Definition:** `skills/engineering/grill-with-docs/SKILL.md`, MIT repo — **fetched raw**, 2026-09-28.

**What it does, in my words.** The same interview as `grill-me`, anchored to a repository, with the
settled vocabulary and the genuinely hard decisions landing in files as the conversation goes rather
than being reconstructed afterwards.

### THE METHOD

**There is no third method. The definition file is one line: invoke `grilling`, then invoke
`domain-modeling`.** That is the whole skill. Everything in §1 (frontier rounds, recommended answers,
dependency ordering, empty-frontier stop, facts-by-sub-agent) and everything in §2 (inline glossary
writes, the three-part ADR test, lazy file creation) applies unchanged. I am stating this plainly
because it would be easy to infer a richer method from the marketing page and there is not one.

Two behaviours the page adds on top, both consequences of running in a repo:

- **Questions the codebase can answer are not asked.** The agent reads the code instead. This is
  `grilling`'s facts-are-my-job rule with the repository as the environment.
- **Terms land the moment they resolve, not in a batch at the end** — the page treats an incremental
  `CONTEXT.md` as the signal the skill is working correctly, and a `CONTEXT.md` that appears whole at
  the end as the signal it is not.

The page is candid about two limits, and both are honest: **the session's ending is vague** — it is
named as a known rough edge — and **few or no ADRs is the normal outcome**, because the three-part
gate rejects almost everything.

**Artifacts:** `CONTEXT.md` (root, or per-context when a `CONTEXT-MAP.md` exists), `docs/adr/NNNN-*.md`,
both created lazily. Everything else stays in the conversation and is lost when the context clears.
That last point is deliberate and is the design: the two artifacts are the only things judged worth
surviving the session.

### PRODUCT — yes; this is the composition, and phase 4 is where it belongs

The judgement is the union of §1 and §2 and adds nothing new: `grill-me` supplies Discover's deep
mode, `domain-modeling` supplies the glossary and the decision gate. What the composition contributes
is the **sequencing** — the interview and the writing are the same session, so a term is captured at
the moment it is agreed and never reconstructed. Reconstruction is where provenance threads break.

If this is planned, plan it as **one capability with two halves**, not as two features. Splitting
them is how we end up with a deep interview whose conclusions live only in a transcript we have
promised (`OD-8`) never to depend on.

### PRACTICE — yes, as our own composed skill

Worth building for our own use, in the `grill-me`-over-`grilling` shape: a thin user-invoked entry
point that composes two model-invoked disciplines of ours. Do not install theirs unmodified —
it writes to `docs/adr/`, which §2 argues we should not create.

---

## 4. `to-spec` — synthesis into a spec, published as one issue *(this is the former `to-prd`)*

**Page:** <https://www.aihero.dev/skills-to-prd> — **fetched**, 2026-09-28, and it serves the
`/to-spec` page, which states the rename happened in v1.1 and the `to-prd` slug is dead.
**Definition:** `skills/engineering/to-spec/SKILL.md`, MIT repo — **fetched raw and read in full**,
2026-09-28.

**What it does, in my words.** It takes a conversation in which the decisions have *already* been
made and turns it into a single written specification, published to the issue tracker as one issue.
It is the downstream partner of `grill-with-docs`, and it is for multi-session work only.

### THE METHOD

The defining rule is a negative one, stated first and in capitals in the source: **do not interview
the user.** It synthesises only what is already known. If the decisions were not made upstream, this
skill will not discover them — it is a scribe, not an analyst.

1. **Explore the repo** if not already done, and then **use the project's own glossary vocabulary
   throughout the spec** and respect existing ADRs in the area being touched. This is the skill that
   makes §2's artifacts pay off downstream, and it is the clearest demonstration of why the glossary
   is worth keeping.
2. **Sketch the testing seams before writing any prose, and get the user to agree them.** Prefer
   seams that already exist; place a new one as high as possible; fewest seams wins and the stated
   ideal is exactly one. This is a real gate — the spec is not written until the seams are agreed.
3. **Write to a fixed template, then publish** as a single tracker issue, labelled ready for an
   agent, with no further triage. The tracker is GitHub, GitLab, or local markdown files, configured
   by a separate setup skill.

**Template sections:** problem from the user's point of view; solution from the user's point of view;
a deliberately long numbered list of as-an-actor-I-want-so-that user stories; implementation
decisions; testing decisions; **out of scope**; further notes.

**Two content rules worth noting.** File paths and code snippets are excluded from implementation
decisions because they go stale — *except* where a prototype produced a snippet that pins a decision
more exactly than prose could, such as a state machine, a reducer, a schema or a type shape, in which
case the decision-carrying fragment is inlined and attributed to the prototype. And the testing
section is required to state what makes a good test, which modules are covered, and the prior art in
the codebase to imitate.

**Stated limits:** the spec freezes at publication and is a snapshot, not a living document; there is
no deduplication against the tracker and no automatic linking back to ADRs; and a very large spec can
be truncated if the context clears between writing it and slicing it into tickets.

### PRODUCT — not phase 4; a candidate for phase W, and only partly

The template itself is not for us. A long user-story list is a house style, and ours is different —
`docs/AGENTIC-UX.md` and `docs/DESIGN-SYSTEM.md` push toward a short, dense brief that an agent reads
in full, which is what phase 4 already produces. Adopting this template would put a second,
competing brief format into the same provenance thread.

Three mechanisms inside it *are* worth carrying into the work layer (phase W):

1. **Seams agreed before prose.** In the product this reads as: before a work item is dispatched to
   an agent, the user confirms *where it will be proven*. That is an approval gate the Hub already
   has a shape for, and it is the single most effective way to stop an agent testing at the wrong
   altitude.
2. **Synthesis with no new questions.** A work item generated from a brief plus its Develop history,
   with the explicit rule that it may not ask the user anything new, is a genuinely good product
   behaviour for an intermittently-present user.
3. **An explicit out-of-scope section.** We already do this — it is the fifth base question in
   Discover, and its help text names the exact failure it prevents: an agent widening the job while
   you are away. Independent arrival at the same mechanism is the strongest signal in this document
   that the two designs agree about what matters.

### PRACTICE — no

Do not install it. It is built around a tracker and a companion setup skill we do not use, it
publishes issues (`gh` is on the KPMG managed ask-list and a skill that publishes on its own is the
wrong shape for us), and its spec template is weaker for our purposes than our own plan format.

The one rule worth lifting is seams-before-prose — and `docs/superpowers/plans/2026-09-18-phase-4-discover.md`
already opens with a **"Phase seams — read before Task 1"** table naming each cross-phase contract
and where it is restated. We arrived at the same rule independently and applied it at the phase
boundary rather than the test boundary. Nothing to install.

---

## The three mechanisms, isolated

The brief asked for three things specifically. Stated on their own, stripped of everything else:

**1. Round structure.** Questions per round: *the whole frontier*, however many that is — not a fixed
number, and sessions of forty-plus questions over four rounds are described as ordinary. Each question
is numbered and ships with the agent's own recommended answer, so a user can accept a round wholesale.
Next-question selection: **pure dependency order** — whatever the last round's answers made answerable.
A question depending on another question open in the same round is illegal in that round. Stop
condition: **the frontier is empty**, then a second, explicit stop waiting for the user to confirm
shared understanding before anything is acted on. Facts are never asked of the user; they are looked
up by a sub-agent, non-blocking, and only the questions downstream of the lookup wait for it.

**2. The ADR test.** Three conditions, all at once: hard to reverse, surprising without context, the
outcome of a real trade-off. Miss one and there is no record. The gate is designed to reject almost
everything, and the source says so: few or no ADRs is the expected result of a session.

**3. Glossary versus conversation.** Into the glossary: project-specific nouns only, one or two
sentences each, defining what the thing *is* rather than what it does, opinionated to one canonical
word with the rejected synonyms listed explicitly — and **no implementation detail of any kind**.
Into a decision record: only what passes the three-part test. Everything else — the overwhelming
majority — stays in the conversation and is allowed to die with it. Writing happens **inline, at the
moment a term settles**, never batched at the end.

## What does not transfer

- **Their spec template.** Long user-story lists are their house style, not ours, and a second brief
  format inside the same provenance thread is a defect, not a feature.
- **`docs/adr/` as a directory.** We have `OD-1`…`OD-35` with owner authority. Take the test, leave
  the layout.
- **Publishing to an issue tracker from inside a skill.** Wrong shape under the KPMG managed policy.
- **A repo-local glossary as the source of truth.** `OD-3` puts shared state in the Hub; the worktree
  gets a projection, not the original.
- **`QUESTION n OF m` over grilling rounds.** Rounds have no closed count. `docs/AGENTIC-UX.md`
  anti-pattern 2 forbids a denominator the system cannot defend.

## Summary

| Skill | Fetched | PRODUCT | PRACTICE |
|---|---|---|---|
| `grill-me` / `grilling` | Yes — page + MIT source | **Yes** — phase 4, optional deep mode after the five questions | **Yes** — write our own; frontier rule + facts-by-sub-agent |
| `domain-modeling` (the real "domain-model") | Yes — MIT source, in full | **Yes, strongest idea** — Language surface in phase 4, glossary into the worktree, three-part test gates ledger entries | **Half** — adopt a root `CONTEXT.md`; do **not** add `docs/adr/` |
| `grill-with-docs` | Yes — page + MIT source | **Yes** — it is the composition of the two above; plan as one capability | **Yes** — as our own composed skill |
| `to-spec` (was `to-prd`) | Yes — page + MIT source, in full | **Partly, phase W** — seams-before-prose, synthesis-without-questions; **not** the template | **No** |
| `to-prd` | n/a | **Does not exist** — renamed to `to-spec` in v1.1 | — |
| `domain-model` | n/a | **Not a skill** — the URL serves the `grill-with-docs` page | — |

## Verification notes and what was not read

- Every method claim above is traced to a definition file in the MIT repo, fetched raw over HTTPS on
  2026-09-28 and read in full. The marketing pages were used only for framing, for the rename fact,
  and for the stated limits, and are cited separately where relied on.
- **Nothing was blocked.** The corporate proxy passed both `aihero.dev` and `raw.githubusercontent.com`.
- The `LICENSE` file's text was **not** read — only the repo page's statement that the licence is MIT
  and the file is named `LICENSE`. If we adopt anything at all, read the file before doing so.
- The catalogue's two category pages linked from the index — the main flow, and shaping — both return
  HTTP 404 at the slugs the index gives. The per-skill pages resolve; the category pages do not.
- The remaining twelve catalogue skills (`to-issues`, `to-tickets`, `triage`, `tdd`, `review`,
  `prototype`, `handoff`, `wayfinder`, `writing-for-agents`, `writing-great-skills`, `wait-what`,
  `ask-matt`) were out of scope here and are not judged. Note for whoever takes them: the repo's real
  folder names differ from the catalogue's display names in several cases, so derive them from
  <https://api.github.com/repos/mattpocock/skills/git/trees/main?recursive=1> rather than from the
  catalogue.
- **No text from any of these skills has been copied into this repository.**




