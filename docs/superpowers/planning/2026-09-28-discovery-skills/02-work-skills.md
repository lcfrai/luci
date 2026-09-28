# 02 — The work skills: `to-issues`, `to-tickets`, `triage`, `wayfinder`

**Date: 2026-09-28.** Study of four skills in the aihero.dev catalogue (Matt Pocock),
covering the stretch from *a settled specification* to *tracked, ordered work*. Phase W (the work
layer, 187 tasks, `OD-26`/`OD-35`) and Phase 4 (Discover) both land in this stretch.

**Intellectual property.** These are someone else's published skills. Everything below is a
description of **method in our own words**, written from pages we fetched and cited. No prompt text,
no skill file and no wording of theirs is copied into this repository or into the product. A method
or workflow shape is not protected; a body of text is. Source repository:
`https://github.com/mattpocock/skills`.

## What was fetched, and what was not

| Skill | URL read | Fetched? | Note |
|---|---|---|---|
| catalogue | `https://www.aihero.dev/skills` | yes (2026-09-28) | Links extracted; the six *set* pages (`getting-started~svvtb`, `the-main-flow-jnjkc`, …) **404'd** when fetched directly, so set membership below comes from the individual skill pages' own cross-links, not from the set pages. |
| `to-tickets` | `https://www.aihero.dev/skills-to-tickets` | yes (2026-09-28) | Full method on the page. |
| `to-issues` | `https://www.aihero.dev/skills-to-issues` | yes (2026-09-28) | **The URL resolves, but the page it serves is titled "The /to-tickets Skill" and names the skill `to-tickets`.** It does not mention `to-issues` anywhere, and does not state a rename. See §1. |
| `triage` | `https://www.aihero.dev/burn-through-your-backlog-with-my-triage-skill` | yes (2026-09-28) | The catalogue's own link for triage. `https://aihero.dev/skills-triage` is cross-linked from other skill pages; not separately fetched. |
| `wayfinder` | `https://www.aihero.dev/skills-wayfinder` | yes (2026-09-28) | Full method on the page. Corroborated by a web search returning `https://github.com/mattpocock/skills/blob/main/docs/engineering/wayfinder.md` (not fetched). |
| changelog v1.1 | `https://www.aihero.dev/skills/skills-changelog-v1-1-wayfinder-to-spec-to-tickets-grilling-improvements` | **no — HTTP 404** | Appeared in search results; the URL does not resolve. So the rename question in §1 stays **unconfirmed**, and we do not assert it. |

**Licence: MIT, at the repository, and stated nowhere on the product pages.** None of the four
aihero.dev pages we read carries a licence statement. The source repository
`https://github.com/mattpocock/skills` states **MIT** (repo page, fetched 2026-09-28). MIT would
permit copying with attribution — **we still do not copy.** The rule in force for this study is
study-the-method-write-our-own, and an MIT grant does not make a body of someone else's prose the
right thing to carry in our plans. The licence is recorded so the question is answered, not so it
is used.

**One more fact from the repository page, and it settles §1:** the `skills/` tree lists
`to-tickets` and does **not** list `to-issues`.

---

## 1. `to-issues` — almost certainly the former name of `to-tickets`

**What it does.** Nothing separate that we could observe. The catalogue lists `/to-issues` and links
it to `https://www.aihero.dev/skills-to-issues`; that page serves the `to-tickets` skill under the
title "The /to-tickets Skill", names the skill `to-tickets` throughout, and mentions `to-issues`
nowhere. The v1.1 changelog, whose title in search results reads
"…`/wayfinder`, `/to-spec`, `/to-tickets`…", **404'd** when we tried to read it.

**The method: see §2.** There is no second method to extract. Treating `to-issues` as a distinct
skill would mean inventing content for a page that does not exist.

**Honest statement of uncertainty.** Two readings fit what we saw: (a) `to-issues` was renamed
`to-tickets` and the old slug still serves the new page; (b) the catalogue is stale. We could not
distinguish them, because the changelog that would settle it did not resolve. **We did not read a
`to-issues` page and we do not describe one.**

**PRODUCT: not applicable.** Nothing distinct to adopt.
**PRACTICE: not applicable.** Install `to-tickets`' method, once, under one name.

---

## 2. `to-tickets` — a spec becomes vertical slices, quizzed before anything is published

Source: `https://www.aihero.dev/skills-to-tickets` and `https://www.aihero.dev/skills-to-issues`,
both fetched 2026-09-28.

**What it does, in our words.** Takes a settled plan, a spec, or the conversation you are already
in, and cuts it into tracker tickets — each one a thin slice that goes all the way through the
stack rather than a layer of it, each one naming the tickets that must finish before it can start.

### THE METHOD

1. **Take the input without a round trip.** Plan, spec, or the live conversation. The skill's own
   guidance is to run it in the *same context window* as the spec-writing step, with no clear and no
   compact between, so the spec is never fetched back out of the tracker and never truncates.
2. **Find the prefactoring first.** Structural work that has to exist before the feature work can be
   cut cleanly is identified and **published ahead of** the feature tickets.
3. **Cut vertically, not horizontally.** The unit is a *tracer bullet*: a narrow but complete path
   through every layer the change touches — schema, API, UI, tests — that can be demonstrated on its
   own the moment it lands. Layer-shaped tickets (all the schema, then all the API) are the failure
   mode the skill exists to prevent.
4. **Declare blocking edges on every ticket.** Ordering is a property of the tickets, not a separate
   plan document.
5. **Quiz the human before publishing.** The breakdown comes back as a numbered list, blockers
   first, each with its *Blocked by* line, and asks three things: is the granularity right, are the
   blocking edges accurate, should any two merge or any one split.
6. **Publish only after approval.** Then it stops. No dispatch, no ticket-state management, no
   follow-through.

**The granularity rule, and it is the part worth having.** One question decides one-ticket versus
two: *what can I demo when this is done?* The answer has to be a **behaviour**. A ticket with no
answer is a horizontal slice and gets re-cut. A second, weaker test sits beside it: a ticket should
read as something a fresh session could finish with nobody in the room, and should fit one context
window.

**The evidence the page offers for it** (their number, not ours): a 26-ticket stack sliced by layer
took roughly twenty agent runs per closed ticket, about three quarters of them rework.

**The wide-refactor branch.** Mechanical changes with a broad blast radius do not get tracer
bullets; they get **expand → migrate → contract**. Introduce the new form beside the old; move call
sites in batches, one batch per package or directory, each blocked by the expand ticket; delete the
old form once the last caller has moved.

### Artifacts, and where they live

- **Markdown tracker:** one file per ticket at `.scratch/<feature-slug>/issues/<NN>-<slug>.md`,
  numbered blockers-first, so ticket 01 has no blockers and can start immediately.
- **GitHub or Linear:** native issues with native blocking links or sub-issues.
- **Each ticket carries** a title, a description, a *Blocked by* line, and acceptance criteria that
  state the demo path. It deliberately carries **no file paths and no line numbers** (the exception
  is a snippet lifted from a prototype).
- **Link back to the source:** by blocking relationship to the originating spec issue. The page
  records a **known gap** — on GitHub the tickets were not created as sub-issues, so the parent link
  is wired by hand afterwards.

### PRODUCT — yes, and it is Phase W's missing verb

**Phase W already has the nouns and none of this verb.** `05-design.md` §3.3 gives us issue states
where five of the nine are set by *facts*, a `serves` edge from issue to brief criterion, and
`Panacea-Issue` commit trailers. What no part of Phase W or Phase 4 currently describes is **how a
settled brief becomes that set of issues in the first place**. Today the answer is "a person types
them", and that is the step where the provenance thread is most likely to be dropped, because a
human writing 40 issues will not attach `serves` to all 40.

Concretely, what the user sees: on a brief whose criteria are settled, one action — *Break into
work* — returns a numbered list in the Work surface, **not yet saved**: each row a one-line
behaviour, a *Blocked by* row beneath it, and the criterion it `serves` shown as a chip. The user
merges two rows, splits one, re-points a chip, and presses one key to publish. Every published
issue carries `serves` because the generator, not the human, attached it.

**Why this is the highest-value single item in this study.** It closes the thread's worst gap —
`uncovered` criteria (a criterion no issue serves, `05-design.md` §3.3) stop being a report and
become impossible to create by accident — and it is small: it is one generator plus one review
screen over records Phase W already defines.

**Two rules of theirs we should take unchanged.** (1) The quiz is mandatory and nothing is written
before approval: this is the same gate as `CLAUDE.md`'s prototypes-before-plans. (2) No file paths
in tickets — they rot, and Phase W's issues outlive the code layout.

**One rule of theirs we should change.** Their demo test asks *what can I demo?* Ours should ask
**what evidence closes this, against which criterion?** — because `05-design.md` §3.3 already says
`done` means an evidence record citing this issue at or after its land commit. Their test and ours
agree on shape and ours is stricter, so it is a substitution, not an addition.

**Slice:** W1 for the generator and the review screen (it is the same surface W1 already builds);
the `serves` suggestion quality belongs with the W4 suggestion work (`05-design.md` L-51).

### PRACTICE — yes, but as an amendment, not an install

Our own build already does most of this: `superpowers:writing-plans` produces numbered tasks and
`subagent-driven-development` executes them. What our plans do **not** consistently do is the
vertical cut. Phase W's own 187 tasks and Phase T's 69 were written per-part; whether they slice
vertically is not something this study verified, and the rework number on their page is the reason
to check.

**Do this, not an install:** add the demo question to `writing-plans` as an acceptance test on each
task — *name the behaviour this task makes demonstrable* — and treat a task with no answer as one to
re-cut. No new skill, no new prompt text, no copying.

---

## 3. `triage` — verify before you label, and remember what you rejected

Source: `https://www.aihero.dev/burn-through-your-backlog-with-my-triage-skill`, fetched 2026-09-28.

**What it does, in our words.** Works a queue of *inbound* work — bug reports, feature requests,
unsolicited pull requests — and moves each item to exactly one category and one state, after
actually trying to reproduce or locate the thing, producing either a brief an agent can act on, a
question back to the reporter, or a recorded rejection.

### THE METHOD

1. **Scope check first.** It handles only work that arrived **from outside**. Issues the user wrote,
   and issues a ticket-generator produced, are excluded — they are already agent-ready. This is the
   branch most triage tooling gets wrong.
2. **Two orthogonal labels, one value each.** A *category* role (`bug` or `enhancement`) and a
   *state* role (`needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`).
   Canonical role names are mapped to whatever label strings the tracker actually uses, through a
   config file the repo holds (`docs/agents/issue-tracker.md` on their page).
3. **Verify before briefing** — three checks, in order:
   a. **Reproduce.** Run the reporter's steps; for a PR, check the branch out and run the tests.
   b. **Search the codebase for redundancy**, by concept rather than by keyword — is this already
      built, or already covered.
   c. **Check the rejection memory** (`.out-of-scope/`) for the same concept turned down before.
4. **Report what verification found**: confirmed, with the code path; failed to reproduce; or not
   enough detail to try.
5. **Branch on that result.** Confirmed → write the brief. Cannot reproduce → `needs-info`. Not
   enough detail to even test → `needs-info` (their page calls this the strongest signal). Already
   implemented → `wontfix`, with a comment pointing at where it lives. Enhancement rejected →
   `wontfix` plus a new file in `.out-of-scope/`. Bug rejected → `wontfix`, closed politely.
6. **Recommend, then wait.** It shows its category-and-state recommendation with reasoning and its
   codebase findings, and applies nothing until the human confirms.

### Artifacts, and where they live

- **The agent brief:** a structured comment on the `ready-for-agent` issue naming types, signatures
  and behavioural contracts — deliberately **no file paths and no line numbers** (the same rule
  `to-tickets` uses).
- **`.out-of-scope/<concept>.md`:** one file per rejected *concept*, not per issue, holding the
  reason and the issues that asked for it. This is the durable part: it is why the same request
  arriving three times gets the same answer without three deliberations.
- **Labels on the tracker**, through the canonical-role mapping.
- **Every AI-written comment opens with a disclosure line** saying it was generated during triage.

### PRODUCT — yes, three specific pieces; no, to the whole skill

Phase W already has triage as a first-class surface: a `triage` state, a per-team Triage inbox,
number-key actions, ordered intake rules, AI suggestions with a "why", duplicate detection
(`05-design.md` §C, L-46 to L-56), and *requests* from outside the team that keep the requester's
verbatim quote (§3.5). So the **queue** is designed. Three things on their page are not in ours:

1. **Verify before `ready-for-agent`.** Our L-48 requires a *priority* before an item may leave
   triage. Theirs requires a *reproduction*. Ours is cheaper and weaker. Add a second gate on the
   one transition that matters: an issue may not reach a state where an agent picks it up until
   either a reproduction was observed or a person attested that none is possible. This fits our
   existing machinery exactly — it is an evidence record, the same kind `05-design.md` §3.3 already
   uses for `done`, moved to the front of the issue's life.
2. **A rejection memory.** We have no `.out-of-scope/` equivalent, and with `rq-` requests arriving
   from business owners and clients by web form, email and Teams (§3.5), we will re-litigate the
   same ask. What the user sees: declining a request asks for a one-line reason and offers *decline
   this concept, not just this request*; the next similar arrival shows **"declined before, here is
   why, here are the three that asked"** on the triage row. Their concept-level, not issue-level,
   granularity is the good idea — a per-issue "wontfix" teaches nothing.
3. **The AI disclosure line.** Every platform-generated comment says so, inline. For KPMG this is
   close to free and worth having from W1, and it belongs in `DESIGN-SYSTEM.md`/`AGENTIC-UX.md`
   rather than in the triage feature alone.

**No** to their label state machine as such: ours is richer and fact-driven (five of nine states set
by facts), and adopting a five-state model would be a downgrade.

**Slice:** the verify gate and the disclosure line in **W1**; the rejection memory with intake rules
in **W5**, where requests land (`05-design.md` L-49, §3.5).

### PRACTICE — no, not yet

PanaceaApp has no inbound external issue queue. There are no outside reporters, no unsolicited PRs,
and the only issues that exist are ones we generate — precisely the population their skill excludes.
Installing it now would give our agents a workflow with an empty input.

**Revisit when** the repo takes external contributions, or when the owner's review comments become a
tracked queue rather than chat. The one piece worth borrowing before then is behavioural, not a
skill: **an agent must reproduce a reported defect before it is allowed to propose a fix** — which
`superpowers:systematic-debugging` already requires, so nothing is missing today.

---

## 4. `wayfinder` — when the planning itself is too big for one session

Source: `https://www.aihero.dev/skills-wayfinder`, fetched 2026-09-28; corroborated by a web search
returning `https://github.com/mattpocock/skills/blob/main/docs/engineering/wayfinder.md` (**not**
fetched — cited only as existing).

**What it does, in our words.** For an effort whose destination you can name but whose route you
cannot yet see, it builds a map of **decisions still to be made**, each one sized to a single agent
session, each one blocking the ones that depend on it, and works the unblocked edge of that map
until nothing is left to decide.

### THE METHOD

1. **Precondition: a tracker with real blocking relationships** — GitHub, GitLab, or a markdown
   fallback. Ordering must be a first-class edge, not prose.
2. **Name the destination.** What "done" means for the *whole* effort, before any ticket exists.
3. **Create one parent issue, labelled as the map**, with four fixed sections: **Destination**,
   **Decisions so far**, **Not yet specified** (their name for it: the fog of war), and **Out of
   scope**.
4. **The rule that defines the whole thing, and the one their page says agents break most often:
   every ticket holds a question whose resolution is a *decision*, never a slice of build to
   execute.** The map is finished when nothing remains to be decided before someone goes and builds.
5. **Type each ticket**, with a label: *grilling* (settled by live conversation), *prototype*
   (behavioural or aesthetic — needs an artifact before anyone can answer), *research* (factual;
   handed to parallel subagents), *task* (a manual blocker such as provisioning or a data
   migration). Mark each **human-in-the-loop** or **agent-only**.
6. **Resolve the frontier.** A session claims an unblocked ticket by assigning itself, works
   **one ticket per session** (research is the exception), posts a resolution comment — and then
   **stops rather than starting to build**.
7. **Fold the answer back.** A closed ticket adds a one-line summary to the map's *Decisions so far*,
   and its resolution unblocks whatever it blocked.
8. **Stop and hand off** when the fog is gone and no tickets remain — or, if the effort turns out to
   be small enough for one session, abandon the map and use the single-session interview skill
   instead. The cleared map goes to the spec-writing step, then to `to-tickets`.

### Artifacts, and where they live

One parent map issue on the tracker; decision tickets with native blocking edges; a growing
*Decisions so far* list on the map; and, downstream, a spec collapsed from the map. **The map is the
plan document** — there is no separate file that can drift from it.

### PRODUCT — yes, and it is the missing front half of Phase 4

**Phase 4 as planned is a single-session flow.** Its plan (`plans/2026-09-18-phase-4-discover.md`)
is a question flow with model-tailored options and **at most one follow-up** (Task 3), a drafted
brief (Task 6), a brief store (Task 7) and provenance (Task 8). That is right for a project a person
can describe in one sitting. It has no answer for the case the owner's own program is living
through: *the deciding is the work, and it spans weeks.*

What the user would see, and it should be a **mode of Discover, not a new phase**: when a brief's
open questions exceed what one sitting can settle, Discover offers a **map**. The map shows the
destination in the user's own words, the decisions already made as one line each, the questions not
yet answerable, and what is explicitly out of scope. Each open question is typed — *ask me*,
*prototype it*, *go and find out*, *someone must do a thing* — and shows what it is waiting on.
Agents work the unblocked ones; a *prototype* question opens the Phase 5 canvas; a *research* one
fans out to sandboxes; an *ask me* one arrives on the Desk as a `NEEDS YOU` row that answers in
place (`05-design.md` §4.2). When the last question closes, the map collapses into the brief, and
the brief's criteria flow to `to-tickets`' generator (§2) and on to issues.

**Why this is worth building rather than admiring.** It puts the *decisions* into the provenance
ledger, which today starts at the brief. Our thread currently reads brief → issue → commit →
release; a decision made before the brief existed is invisible to it. With a map, the thread reads
**question → decision → criterion → issue → commit → release**, and a criterion can point at the
decision that produced it. That is a strictly stronger version of the thing `CLAUDE.md` calls the
product's spine, and it is the phase the owner says they care about most.

**Two adaptations we must make, not copy.**
- **Their map lives on a tracker; ours must live in the Hub**, because `OD-3` puts briefs, approvals
  and the ledger server-side, and Phase W already owns issues. The map is a record beside the brief,
  not a GitHub issue.
- **Their decision ticket is untyped as to reversibility; ours should not be.** The hard-to-reverse
  ones are exactly what an ADR is for, and that is the other half of this study (see the
  `grill-with-docs` write-up, `01-*`). A map without that distinction produces 200 equal-weight
  decisions and no one can find the five that matter.

**Slice:** this is **Phase 4**, as a second mode, after the single-session flow works. It should not
displace it — most projects genuinely are one sitting, and a map imposed on a small project is
overhead. Their own skill agrees: it tells you to abandon the map when the effort is small.

### PRACTICE — yes, and this is the strongest practice item of the four

Our program has the exact failure this targets. The evidence is in our own repo:

- `planning/2026-09-23-owner-decisions.md` is a **flat table of 35 decisions with no blocking
  edges** and no record of what each one unblocked. It is already out of order — `OD-31` is written
  above `OD-30` in the file — which is what happens when a decision register has no structure to
  keep it honest.
- `planning/2026-09-25-owner-questions-pending.md` is a fog-of-war register in all but name, but it
  is grouped **by the workflow run that raised each question**, not by what the answer would unblock.
  So nothing tells us which pending question is holding up the most work.
- `OD-31` was an owner intervention about spend — 964 agents, 125M output tokens, about 11% of the
  non-phase-2 build approved. A map that showed which decisions were blocked on which would have made
  the wasted parallelism visible earlier.

**What to do, cheaply, and it is not an install.** Give the decision register two columns it lacks —
*unblocked by* and *unblocks* — and give the pending-questions file one: *what it holds up*. Then the
next planning wave can be ordered by the frontier instead of by workflow id. That is an hour of
editing and it is testable immediately: if the frontier is empty and work is still queued, the queue
is wrong.

**What not to do:** do not install their skill as-is into our agents. Our orchestration already
assigns one task per agent session and already stops at a review gate; the borrowed part is the
**map's structure**, not another workflow.

---

## 5. Verdicts, ranked

| Skill | PRODUCT | PRACTICE | The one thing worth having |
|---|---|---|---|
| `wayfinder` | **Yes — Phase 4, second mode** | **Yes — as structure on our decision register, not an install** | A decision ticket holds a *question*, never a slice of build; the map ends when nothing is left to decide. |
| `to-tickets` | **Yes — Phase W, W1** | **Yes — as an amendment to `writing-plans`** | Granularity is decided by one question: what behaviour can be demonstrated when this is done. No answer means it was cut by layer. |
| `triage` | **Partly — three pieces (verify gate, rejection memory, AI disclosure)** | **No, not yet — no inbound queue exists** | Rejections are remembered at the level of the *concept*, not the issue. |
| `to-issues` | n/a | n/a | Not a separate skill as far as we could observe: the URL serves the `to-tickets` page and the repository's `skills/` tree has no `to-issues`. |

**Cross-cutting, and it is the sharpest idea in all four:** *the tracker is the plan*. Both
`wayfinder` and `to-tickets` refuse to keep a planning document beside the tickets, because two
copies drift. Our program keeps both — plans in `docs/superpowers/plans/` and, from Phase W, issues
in the Hub — and has already paid for the drift once (`PROGRAM.md`'s phase table and its phase
prose disagreed on every phase from 3 onward until 2026-09-23). Phase W should state which of the
two is authoritative for in-flight work, and this study's recommendation is: **the plan is
authoritative until its issues are published, and the issues are authoritative after.**

## Open items

1. **Licence:** MIT at the repository (`https://github.com/mattpocock/skills`, 2026-09-28); no
   licence statement on any aihero.dev page read. We copied nothing either way.
2. **Unverified:** whether `to-issues` was renamed to `to-tickets`. The changelog that would say so
   returned HTTP 404.
3. **Not read:** the six catalogue *set* pages (404 on direct fetch) and the skill source files in
   the GitHub repository. Set membership above is inferred from cross-links on the skill pages.
4. **Not read:** `https://aihero.dev/skills-triage`, `skills-to-spec`, `skills-implement` — outside
   this agent's four, and cited only as links that exist.
