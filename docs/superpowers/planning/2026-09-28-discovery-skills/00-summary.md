# 00 — Summary: the aihero.dev skill catalogue against PanaceaApp's plans

**Date 2026-09-28.** Inputs: `01-define-skills.md`, `02-work-skills.md`, `03-build-skills.md`,
`04-meta-skills.md` (all four read in full). Plans read by task table and by targeted range:
phase 4 (`.claude/worktrees/phase-4/…/2026-09-23-phase-4-discover.md` + its 23 parts), phase W
(`…/work/…/2026-09-25-phase-w-work-layer.md`, 187 tasks), phase T
(`…/team/…/2026-09-25-phase-t-team-mode.md`, 69 tasks), phase 5
(`…/phase-5/…/2026-09-23-phase-5-design-canvas.md`, 30 tasks). **Read-only throughout; no plan
was edited.**

**Catalogue:** <https://www.aihero.dev/skills> (Matt Pocock), source
<https://github.com/mattpocock/skills>, **MIT** (repo page, 2026-09-28). Seventeen catalogue
entries; **fourteen real skills** — `to-prd` is the dead slug of `to-spec`, `domain-model` serves
the `grill-with-docs` page, `writing-great-skills` is the former name of `writing-for-agents`, and
`to-issues` is almost certainly the former name of `to-tickets` (the changelog that would confirm
it 404s, so it stays unconfirmed).

---

## 1. Verdicts

`P4` = phase 4 Discover · `P5` = canvas · `PW` = work layer · `PT` = team mode · `P3` = Develop/Merge.

| Skill | Our phase | Do our plans do it? | Verdict | Why, in one line |
|---|---|---|---|---|
| `grilling` (`grill-me`) | P4 | **Partly** — one question at a time, one model follow-up (task 04); no round, no frontier | **Both** | The round-of-drafted-answers screen already exists as the repo on-ramp (tasks 13, 21); the frontier ordering is the missing piece |
| `domain-modeling` (glossary + ADR gate) | P4 | **Not at all** — zero mentions of glossary, `CONTEXT.md` or decision record in the whole plan | **Both** | The thread carries intent and criteria; it carries no *language* and no *decisions*, which is exactly what an agent gets wrong alone |
| `grill-with-docs` | P4 | **Partly** — it is the composition of the two above | **Both** | Plan as one capability; splitting it puts the conclusions only in a transcript `OD-8` forbids us to depend on |
| `writing-for-agents` | P4 task 05 | **Partly** — the renderer exists, no lens applied to it | **Both** | A quality bar on `claudemd.Render`, not a button; the no-op test already finds two human-facing lines paid for in every session forever |
| `to-tickets` | PW (W1/W2) | **Partly** — `serves`, issue states and `split into issues` (task 88) exist; nothing turns a settled brief into the issue set | **Product** | Attach `serves` by generator, not by a human typing 40 issues; `uncovered` stops being a report |
| `wayfinder` | P4 second mode | **Not at all** — P4 is a single-session flow | **Both** | Puts *decisions* into the ledger, so the thread reads question → decision → criterion → issue → commit → release |
| `code-review` (two unmerged axes) | P3 diff review | **Partly** (unverified — phase 3's plan was not in my reading set) | **Both** | The Spec axis *is* the brief; one ranked list silently trades "right code" against "right feature" |
| `prototype` | P4/P5 boundary | **Partly** — P5 has the canvas, nothing attaches a disposable answer to an open brief question | **Both** | Two artifacts, two fates: the answer merges, the code never does |
| `handoff` | P2 fork/resume; PT (own sub-agents only) | **Partly** — fork/resume exist and start cold | **Both** | References not copies, destination-shaped; **reject** it as the cross-user channel (`OD-15`, `OD-16`) |
| `wait-what` | PT escalation card | **Not at all** | **Product** | An escalation is second-hand reasoning arriving at someone who was not in the conversation — the highest-incomprehension surface we ship |
| `triage` | PW W1 + W5 | **Mostly already** — `triage` state, inbox, intake rules, `rq-` requests with verbatim quote, AI suggestions with a "why" | **Product, 3 pieces only** | Take the verify gate, the concept-level rejection memory and the AI disclosure line; our nine-state fact-driven model beats their five |
| `to-spec` | PW | **Mostly already** — P4's brief is our spec; out-of-scope is question 5 | **Skip the template; 2 rules** | Seams-agreed-before-prose and synthesis-without-new-questions; the template is a competing brief format |
| `tdd` | — | **Already** (red-green enforced) | **Practice, 1 pre-step** | A blocking *test-boundary* agreement before the first test; do not overload our `SEAMS.md` word |
| `ask-matt` | — | **n/a** | **Skip** | A router cures a catalogue with no spine; Define→Design→Develop→Deploy is a spine, on screen |
| `to-issues` · `domain-model` · `writing-great-skills` · `to-prd` | — | — | **Skip — not skills** | Dead slugs and renames; do not plan, evaluate or install twice |

---

## 2. `grill-with-docs` against phase 4, mechanism by mechanism

The owner asked for this one. Its three mechanisms, each judged against the plan as written.

| Mechanism | Phase 4 as planned | Verdict |
|---|---|---|
| **Round-based interview that waits between rounds** | `Step` is singular: `StepQuestion` / `StepFollowUp` / `StepReview` / `StepComplete`, with `Ordinal` and `Total` (part `task-03.md:186-200`). Task 04 adds **one** model follow-up, degrading to nothing. There is no round and no frontier | **Partly — and closer than it looks.** See below |
| **Glossary written as vocabulary settles** | **Zero** occurrences of `glossary`, `CONTEXT.md` or `vocabulary` in the 87 KB plan (grep, 2026-09-28). `Content` holds `Questions`, `Answers`, `Inherits`, `Change` and nothing else | **Not at all** |
| **The ADR test (hard to reverse · surprising · a real trade-off)** | No decision record of any kind. `Change{Kind, Key, Restores}` records *what op produced a revision*, and the plan states it is never rendered into `CLAUDE.md` | **Not at all** |

### 2.1 Rounds — the screen is already built, the ordering is not

**The strongest already-does-it finding in this study.** Phase 4's repository on-ramp is a grilling
round in all but name:

- `Author` is already `person · model_drafted · model_accepted`, and a drafted answer **is not
  settled**, so a revision holding one is never complete and is never materialised (C5,
  `task-03.md:119-131`).
- `Content.Pending()` returns the drafted answers awaiting a person, in question order, and
  `StepReview` means *only drafted answers remain; accept or change each*.
- Task 13 drafts a brief from a mechanical repo scan; task 21 is the desktop screen where a person
  accepts or changes each drafted answer.

That is "a batch of questions, each carrying the agent's recommended answer, accept wholesale or
override individually" — already designed, already test-planned. A grill mode is a **second
producer** into the same `ChangeDraft` op and the same review screen.

**What is genuinely missing is the frontier**: a dependency order over questions, so round 2 is
mechanically whatever round 1's answers made answerable. Without it there is only ever one round.

**Two constraints the plan already gets right for free.** `Step.Total` is zero for every kind but
`StepQuestion`, and G5 says the literal `"QUESTION n OF m"` is its only legal rendering — so a
grill round **cannot** print a denominator it could not defend (`AGENTIC-UX.md` anti-pattern 2).
And the facts-are-the-agent's-job rule is our execution shape already: dispatch a sub-agent, block
only the questions downstream of it.

### 2.2 The answer that matters: hang both on the thread, not in loose files

Their glossary is a repo-root `CONTEXT.md` and their decisions are `docs/adr/NNNN-*.md`. **Both are
wrong for us**, for one reason each: `OD-3` puts shared state in the Hub, so a glossary living in
one person's worktree breaks the moment a teammate opens the project; and a loose file has no
revision, so nothing downstream can cite *which* definition an agent was handed.

**The rule: a term and a decision are fields of a brief revision, not documents beside it.** Then
they inherit every property the brief already has — Hub-numbered gapless revisions, immutability,
digests, the `4-D3` project→branch pin, materialisation before the PTY starts, and a key every
surface resolves through one `provenance.Ref`.

### 2.3 What that means for the content model (task 03, `discover/content.go` + `ops.go`)

Two collections beside `Answers`, in the same `Content` object, so they ride the same revision:

| Add | Shape | Modelled on |
|---|---|---|
| `Terms []Term` | `ID` (`t-` + 8 hex), `Word`, `Is` (1–2 sentences, what it **is**), `Avoid []string` (rejected synonyms), `Author` | `Criterion` exactly: id minted once, kept across edits by exact `Word`, plus a text digest |
| `Decisions []Decision` | `ID` (`d-` + 8 hex), `Title`, `Because` (1–3 sentences: context, decision, why), `Gate{Irreversible, Surprising, TradeOff bool}`, `Author`, `Supersedes string` | `Criterion` for identity; `OD-1…OD-35` for the one-paragraph format |

Five new `Op` kinds beside the nine that exist — `define_term`, `rename_term`, `retire_term`,
`record_decision`, `supersede_decision` — all through the single `Apply` path, so the Hub numbers
the revision and it is immutable. Four rules that must be written down or this goes wrong:

1. **The three-part gate is enforced in the domain, not in the prompt.** `Apply` refuses a
   `record_decision` whose `Gate` is not all three true — a new `ErrGateNotMet` beside
   `ErrReservedText` and `ErrTooLong`. Their gate rejects almost everything **by design**; a gate
   that only lives in a model prompt will not.
2. **Terms and decisions never gate completeness.** `Status` is derived from `Answer.Settled()`
   today. A glossary is open-ended; if it counted, a brief could never complete and nothing would
   ever materialise. State it explicitly in task 03.
3. **Glossary means glossary.** Project-specific nouns only, no implementation detail, one
   canonical word with the rejected synonyms listed. Without that rule the glossary becomes a
   second brief, and we have two competing descriptions inside one thread.
4. **Same caps, same encoding.** The 300-code-point cap (rule 13) and LF-only stored text (rule 14)
   apply unchanged.

### 2.4 What that means for the key grammar (task 06, `provenance/key.go` + `ref.go`)

**Do not add a fourth `Kind`.** The grammar is `brief:` · `design:` · `none:`, with one parser (P5)
and one `Ref` every surface reads. A `term:` or `adr:` kind would start a second thread and defeat
the point.

**Mirror criteria instead — the machinery is already there.** `Ref` already carries per-criterion
digests, and `CriterionReading`/`Readings` keep the *moving* criterion states out of the `Ref`
(review I14). Do the same twice:

- `Ref` gains `termDigests` and `decisionDigests`; `Build` computes them the way it computes
  criterion digests.
- Citation form is the existing brief key plus a fragment: `brief:bf-…@7#c-…` for a criterion,
  `#t-…` for a term, `#d-…` for a decision. One parser, one `Ref`, three fragment families.

**The payoff is zero new machinery downstream.** Phase 3 already keys evidence by brief key +
criterion id + text digest (`task-03.md:110-118`); an identical shape means a Develop commit can
cite the decision it implements and phase 7's release manifest, which already mounts `Ref`, carries
it to Deploy with no change. That is the provenance thread doing the work, not a parallel one.

### 2.5 What that means for the Materialiser (task 09 `contextwrite`, task 05 `claudemd.Render`)

The Materialiser is the seam that makes this real rather than decorative: because the term lives
*inside* the brief revision, and `Materialise` runs **before** `ptysession.Start`, and the session
is stamped with the key it wrote — **the agent that uses the word is provably the agent that was
handed the definition.** No repo-local `CONTEXT.md` can have that property.

Four consequences, and the third is the actual cost:

1. **Two new sections in the same managed block**, rendered from the same resolved `Document`. No
   second file, no second write, no second atomic-write hazard.
2. **`discover/inherit.go` needs two new resolution rules.** `Resolve(project, branch)` today is
   project → branch with origins. Terms **union**, branch overriding project by exact `Word`;
   decisions **concatenate**, project first. Both refuse the same way `ErrInheritMismatch` and
   `ErrIncomplete` already do.
3. **The golden render tests and every block digest change.** `claudemd`'s goldens, `SameText`,
   `DiffLines`, `Inspect`'s drift detection and the marker/neutraliser tests are the real work here
   — not the Go types. Budget for that, not for the structs.
4. **Apply `writing-for-agents` to the block before it grows.** A glossary is precisely the
   material paid for in **every session on that branch, forever**. Two rules: a cap on how much
   glossary is inlined, and inherited *project* terms descend to a pointer rather than being
   re-inlined per branch (progressive disclosure). The same pass should move `Render`'s two
   human-facing lines out of the managed block into the marker comment, where the machine-facing
   `brief=` / `rev=` metadata already lives — they change no agent's behaviour and cost context in
   every session.

---

## 3. Changes worth making — ranked, best value first

**Product.** Costs are plan tasks at phase-4 sizing (1–4 h each) and are floors, like every other
estimate in these plans.

| # | Change | Phase | Tasks it touches | Cost |
|---|---|---|---|---|
| 1 | **Terms and decisions as fields of a brief revision** (§2.3–§2.5), with the three-part gate enforced in `Apply` | P4 | amend **03, 05, 06, 09, 18**; +1 task for the render goldens, +1 for the Language/Decisions surface | **2 new, 5 amended, ~12–16 h** |
| 2 | **Break a settled brief into work**: a generator proposes the issue set, attaches `serves` itself, and a review screen merges/splits/re-points before anything is saved | PW | beside **88** (*split into issues*) and **136** (triage suggestions with `serves`) | **2 new, ~6 h** |
| 3 | **Grill rounds as Discover's deep mode**: a frontier ordering over questions, `StepRound`, a second producer into the existing `ChangeDraft` + `StepReview` path | P4 | amend **03, 04, 13, 18, 21**; +2 (`discover/grill.go`, the round screen) | **2 new, 5 amended, ~10–12 h** |
| 4 | **Two unmerged review axes** — *how this repo writes code* and *what the brief asked* — never combined into one ranked list, every finding quoting its source | P3 | **UNVERIFIED**: phase 3's plan was not in this study's reading set | **~2 new, unsized** |
| 5 | **Run the no-op test over `claudemd.Render`'s managed block**; move the two human-facing lines into the marker comment | P4 | amend **05** only | **~1 h — do this first** |
| 6 | **A decision map as Discover's other second mode** (`wayfinder`): typed, blocking decision tickets in the Hub beside the brief, so the thread starts at *question*, not at *brief* | P4 | a slice of its own, after the single-session flow works | **~8–10 new, ~30 h** |
| 7 | **An open brief question carries a runnable, disposable answer**: *Prototype* on the question; the one-sentence answer is written back as a provenance entry; the artifact is linked, never merged | P4/P5 | P4 **18**; P5 after **28** | **~2 new, ~6 h** |
| 8 | **Triage's three pieces**: a reproduction-or-attestation gate before an issue can be picked up by an agent; a **concept-level** rejection memory; an AI-disclosure line on every platform-written comment | PW | gate + disclosure in **W1**; rejection memory with **146/147/170** in W5 | **~3 new, ~8 h** |
| 9 | **The session baton**: fork and resume carry a destination-shaped, **references-only** thread instead of starting cold | P2; PT (an Orchestrator's own sub-agents only) | P2 session lifecycle | **~2 new, ~5 h** |
| 10 | **A "you lost me" re-pitch** on agent messages and on the escalation card, written in the brief's vocabulary | PT | **54** (Desk rows: NEEDS YOU / DECIDED WITHOUT YOU), whose escalations task **30** produces | **~1 new, ~2 h** |

**Practice** — our own agents, building PanaceaApp. All cheap; none is an install.

| # | Change | Cost |
|---|---|---|
| 1 | **Two artifacts for a prototype**: the answer merges (commit message / `OD` / task), the code lives on `prototype/<name>` off `main` and is **never** merged, linked from the task. Plus the structural-variation test — three tweaked card grids is not a prototype | free, today |
| 2 | **Frontier ordering + facts-are-the-agent's-job** in our owner interviews: never ask a question whose answer depends on one still open in the same round; dispatch lookups to sub-agents and keep asking the rest | free |
| 3 | **Two columns on `planning/2026-09-23-owner-decisions.md`** — *unblocked by* and *unblocks* — and one on `2026-09-25-owner-questions-pending.md` — *what it holds up*. Order the next wave by the frontier, not by workflow id | ~1 h |
| 4 | **The three-part gate on what earns an `OD`.** Keep the register; adopt the test. Do **not** create `docs/adr/` — a second decision record is the failure `PROGRAM.md` already paid for | free |
| 5 | **A root `CONTEXT.md` for this repo**: machine · project · branch · worktree · lane · session · brief · harness · sandbox · cockpit · rail · Hub. Project-specific nouns only, 1–2 sentences, rejected synonyms listed | ~2 h |
| 6 | **One acceptance test per plan task** in `writing-plans`: *name the behaviour this task makes demonstrable*. No answer means it was cut by layer and gets re-cut | free |
| 7 | **A test-boundary agreement** before the first test in our TDD gate — and call it *test boundary*, never *seam*, which `docs/SEAMS.md` already owns in a different sense | free |
| 8 | **An incremental baton for workflow agents**: destination note first, references not copies, secrets redacted, uncommitted scratch — written **as the agent works**, because a watchdog-killed agent never reaches the end, which is the case we need it for | ~1 h |
| 9 | **Install `writing-for-agents`'s lens** (our own wording) and point it at three targets: the generated per-branch `CLAUDE.md`, this repo's `CLAUDE.md`, and the phase plans. Apply it, do not obey it — some of our repetition guards a silent failure and is load-bearing | ~1 h |

---

## 4. Skip list — settled, do not research again

| Item | Reason |
|---|---|
| `ask-matt` | Routes only over its author's own skills, off a hand-maintained map its own page admits lags and sometimes misdescribes. We have a visible spine and `superpowers:using-superpowers` already routes over what is actually installed. **One idea kept**, logged as a design question, not a feature: the *phase boundary* — the named moment where someone decides what accumulated context carries across a phase or a handoff |
| `to-issues` | Not a separate skill. Its URL serves the `to-tickets` page, and the repository's `skills/` tree has no `to-issues`. The rename is unconfirmed (the changelog 404s) and it does not matter: there is one method |
| `domain-model` | Not a page and not a user-invoked skill. The URL serves `grill-with-docs`. The real thing is `domain-modeling`, a model-invoked discipline, and it is judged as row 2 of §1 |
| `to-prd` | Dead slug; renamed `to-spec` in v1.1 |
| `writing-great-skills` | Former name of `writing-for-agents`. Same page, same content. Installing both installs one skill twice |
| `to-spec`'s template | Long as-an-actor-I-want lists are their house style. Our brief is short and dense on purpose (`AGENTIC-UX.md`). A second brief format inside one provenance thread is a defect, not a feature. **Two rules kept**: seams agreed before prose, and synthesis that asks the user nothing new |
| `triage` as a skill for our agents | Its input is work arriving **from outside**, and it explicitly excludes issues we wrote ourselves — which is our entire population. Installing it gives an agent a workflow with an empty queue. Revisit if this repo takes external contributions |
| `triage`'s five-state label model | Ours is nine states with five set by facts. Adopting theirs is a downgrade |
| `tdd` as a product surface | How an agent writes its tests is the agent's business; exposing the ceremony adds configuration for no user-visible gain, which is the bar `CLAUDE.md` says we fail. Their no-refactor ruling is their observation about their agents — do not adopt it on their evidence |
| `handoff` as the cross-user channel | `OD-15` wants facts not words; `OD-16` wants continuous Orchestrator negotiation and forbids one user steering another's agent. A one-shot prose summary carried by a human is the opposite of all three. It fits **inside** one owner's boundary only |
| `wait-what` for subagents and workflow agents | A subagent's final message is a return value. No human is present to say "you lost me", so it can never fire |
| `docs/adr/` as a directory | We have `OD-1…OD-35` with owner authority and `CLAUDE.md` says they win over anything older. Take the test, leave the layout |
| Publishing to an issue tracker from inside a skill | `gh` is on KPMG's 63-rule managed ask list; a skill that publishes on its own is the wrong shape here |
| `QUESTION n OF m` over grill rounds | Rounds have no closed count. `Step.Total` is already zero outside `StepQuestion`; keep it that way |

---

## 5. What we must not copy

These are someone else's published skills. **Study the method, write our own.**

1. **No prompt text, no wording, no file** from <https://www.aihero.dev> or
   <https://github.com/mattpocock/skills> enters this repository or the product. Nothing has been
   copied so far — all four study reports state this, and this summary describes method only.
2. **MIT is recorded, not relied on.** The repository is MIT (repo page, 2026-09-28); no aihero.dev
   page carries a licence statement. MIT would permit reuse with attribution. We still do not take
   the text, and the `LICENSE` file itself has **not** been read — read it before adopting anything,
   if anyone ever decides to.
3. **A method or a workflow shape is not protected; a body of prose is.** The frontier rule, the
   three-part decision gate, the tracer-bullet granularity question, the two unmerged review axes
   and the two-artifact prototype rule are ideas. Their sentences are theirs.
4. **Do not install their skills into our agents.** They know nothing about the claudezsh wrapper,
   KPMG's managed policy, or `OD-3`'s server-side state. Where a method is worth having, write our
   own skill in our own words.
5. **Attribute in our planning documents, never in shipped product text.** Cite the URL and the date
   read, as these five files do. A product surface carries our words only.

---

## 6. What this summary did not verify

- **Phase 3's plan was not read** (not in the reading set). Item 4 of §3 — two-axis diff review —
  is carried from `03-build-skills.md` and its "partly" is unverified against phase 3 as planned.
- Phase W and phase T were read by **task table and targeted range**, not in full; phase 5 by task
  table and seams. A method could be present in a part file I did not open.
- Task sizings are floors on the plans' own sizings, which those plans already call low.
- No skill was installed, no plan was edited, and no source outside this directory was written.

