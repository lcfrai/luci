# 06 — The case against adopting any of it

**Date 2026-09-28.** Read: `00-summary.md` and `05-design.md` in full; `01`–`04` by header and by
targeted range. Then — read-only, in the sibling worktrees — phase 4's plan and **its built code**,
phase W's plan and parts, phase T's parts, the Hub contract and the Hub's plan. **No plan was
edited and nothing outside this file was written.**

**The verdict first.** The central insight is right: a glossary and a decision record belong to a
brief revision, not to loose files. Everything built on top of it is not safe to hand to a plan
editor yet, for one reason that dominates all others:

> **`05-design.md` prices a change to shipped code as though it were a change to paper, and its
> centrepiece — the key fragment of §5.1 — edits a grammar that belongs to the Hub, is already
> implemented in two built parsers, is quoted in 64 plan files across six worktrees, and is
> *already solved differently* by phase W in canonical, byte-golden JSON.** §10.4 of the design
> admits: *"The Hub contract was not opened."* That is the whole problem in one sentence.

---

## 1. Redundancy — what our plans already do

### 1.1 The one honest gap: there is no glossary and no decision record

I checked rather than took it on trust. Across phase 4's 23 part files (502 KB), `glossary`,
`vocabulary`, `CONTEXT.md`, `decision record` and `ADR` appear in seven files, **every hit
incidental** (task-07 ×7 are `hub-contract.md` references, not a glossary). The built
`discover/content.go` carries `Criterion`, `Answer`, `Section`, `Inherits`, `Change` — and nothing
that holds a word or a choice. **The gap is real. `00-summary.md` §2 is correct on the only
factual claim that matters to its argument.**

### 1.2 But the criterion citation is not a gap — phase W already shipped its shape

`05-design.md` §5.1 proposes `brief:bf-…@7#c-…` as a criterion citation, and §9.1 makes that
fragment the **prerequisite** for the brief→work generator. Phase W already cites a criterion, and
not that way:

```
work/…/task-02.md:461  "serves":[{"briefKey":"brief:bf-00000000000000a1@5","criterionId":"c-91e2a0b3c4d5e6f7"}]
work/…/task-02.md:971  | drop `check` … | TestIssueRevisionGoldenVectors (v4: 439 bytes, and
                         `check` sorts between `briefKey` and `criterionId`) |
```

A **struct of two fields**, inside canonical JSON, pinned by a golden vector to an exact byte
count. So `#c-…` is not a missing capability; it is a **second** citation form for a thing already
cited, in a plan whose bytes are digested. Adopting it means either two ways to name a criterion
inside one provenance thread — which is the defect `CLAUDE.md` names — or reopening phase W's
canonical encoding and its golden vectors.

### 1.3 And the premise under §5.1 is stale

§5.1 opens: *"The grammar stays `brief:` · `design:` · `none:`"*, and argues that a fourth `Kind`
"would start a second thread". Phase W's own plan, line 69:

> `issue:is-…@r` is immutable and digested, and its `serves [{briefKey, criterionId}]` is what…

A fourth key form is already planned, and the thread survived it. The argument that rules out
`term:` therefore does not hold as stated — which does not make `term:` right, but it does mean
§5.1's reasoning was done against a grammar snapshot that is a phase out of date.

### 1.4 Rounds: the reuse claimed is smaller than claimed, and the design silently agrees

`00-summary.md` §2.1 calls phase 4's on-ramp *"a grilling round in all but name"* and *"already
designed, already test-planned"*. Task 21's own first line:

> ### Task 21: Desktop — import a repository: the brief drafted from the code

It is a one-shot on-ramp for a repository that has code and no brief: five fixed answers, drafted
once, accepted or changed. There is no round, no second pass, and nothing that re-enters it. The
design concedes the point by budgeting **new task 27, "Desktop — the round screen", 2–3 h** — a
screen the summary said was already built.

### 1.5 The provability seed largely re-asks what `done` already asks

§2.5 mints one question per criterion: *what would show this is true?* The built base question is:

```go
KeyDone … ProjectPrompt: "What has to be true before work here is finished?"
Help: "This becomes the definition of done, shown again at merge."
```

Each criterion **is** the observable, and phase 3 already keys evidence to it by id and digest. The
seed is defensible — a criterion can be unfalsifiable — but it is an *increment* on a mechanism that
exists, not a missing one, and §2.5's claim that it prevents an interview "stopping before the
criteria are provable" does not survive §2.4: terminator 1 fires on an **empty frontier**, and a
question that was asked and skipped is no longer in the frontier. The seed guarantees the question
is *put once*, nothing more.

---

## 2. Cost — the design amends code, not a plan

### 2.1 Five of the eight "amended" tasks are already built and committed

This is the finding that changes the price by an order of magnitude. Phase 4 is **not a plan waiting
for an editor**. In `.claude/worktrees/phase-4`, on `feat/phase-4-discover`, the working tree is
clean and the log runs through task 10:

```
ca737d92 docs(plan): task 09 fix round 1, the part follows the code
5fb80e09 feat(contextwrite): the Materialiser, Materialise and Inspect (t09)
190f7249 feat(discoverhub): writes, the 4-D3 fan-out, archive and rebind (t08)
```

The commit convention — *"the part follows the code"* — says it plainly: on this plan the part file
is updated **after** the code. So "amend task 03" does not mean editing a document. It means
editing this:

| Package (built, committed) | Lines | Test funcs | Goldens |
|---|---|---|---|
| `internal/discover` (t03, t04) | 4,501 | 51 | — |
| `internal/claudemd` (t05) | 1,246 | 20 | 3 `.md` |
| `internal/provenance` (t06) | 1,526 | 27 | 1 `.json` |
| `internal/contextwrite` (t09) | 3,667 | — | — |
| **Total** | **10,940** | **98+** | **4** |

§8 of the design sizes tasks 03/04/05/06/09 at **4–5 h + 0.5 + 3–4 + 2 + 0.5 = 10–12 h**. Those are
plan-authoring hours for tasks that do not exist as plans any more. The real work is: 11 or 12 new
`OpKind`s beside the 9 that shipped (`OpRecord`…`OpDraft`), a `Content` struct change in a package
with 51 test functions, four goldens regenerated, and a re-run of the plan's own mutation rounds —
which this repo's own history shows are not free (`cc0d1916 test(discoverhub): t07 fix round 1, kill
the round's two predicted survivors`).

**Honest floor: 3–4× the design's §8 number for the built tasks**, and that is before the wire
format. §1 of the design says the fields go into **`ContentVersion = 1`**, because "phase 4 has not
shipped a revision to any real Hub". The premise is plausible — the Hub worktree is at task 07,
SQL only — but the conclusion is wrong anyway: v1 has *shipped in code* with 98 tests and a
`Decode` that pins it. Widening v1 after the fact is the thing wire-format rules exist to stop.

### 2.2 The key grammar is the Hub's, not phase 4's

`provenance/key.go`, the built file the design amends for "2 h", says so itself:

```go
// The grammar. brief and design are the Hub's, verbatim (Hub contract §3.2,
// hub-contract.md:120-121 at hub da8c7da); none is local to the control plane
```

And the Hub contract, at that line:

```
hub/docs/superpowers/contracts/2026-09-23-hub-contract.md:120
brief key    ^brief:(bf-[0-9a-f]{16})@([1-9][0-9]{0,8})$
```

…and the Hub's own compiled copy at `phase-h-hub-parts/task-01.md:196` — a **built** task. So
adding `(#((c|t|d)-[0-9a-f]{8}))?` is not a phase-4 edit. It is a Hub contract amendment, a change
to two parsers in two repos, and a re-validation of everything that sends a key to the Hub. The
blast radius in plans alone, measured: **64 plan files across six worktrees** contain `brief:bf-`
(work 41, team 4, phase-5 2, hub 1, phase-7 1, phase-4 the rest).

The design cannot have weighed this, because §10.4 says: *"The Hub contract was not opened."*

### 2.3 What amendment actually costs in the frozen plans

Phase W and phase T are the plans the summary's items 2, 8 and 10 land in. Measured today:

| Plan | Part files | Cross-file citations (`file.ext:NNN`) |
|---|---|---|
| Phase W work layer | 190 | 2,544 |
| Phase T team mode | 69 | 1,344 |

And the maintenance cost of moving anything in them is on the record, in phase W's own log:

```
1799f26f plan(work): the re-anchor missed the plan's other citation form; 40 more shifted
```

One re-anchor, 40 citations shifted, and a follow-up commit because the first pass missed a citation
*form*. That is the price of "amend task 88", and neither the summary nor the design pays it: both
quote task-sizing hours (`~6 h`) with no line for re-anchoring.

---

## 3. Does it survive contact with an agent, and with our user?

### 3.1 Our Define phase is five pickable questions. Theirs is a chat window.

This is the deepest objection, and it is not about agents — it is about who is sitting there. The
built interview, `discover/content.go`:

```go
Key: KeyAudience, ProjectPrompt: "Who is this project for?",
Options: []string{"Our own team", "KPMG staff broadly", "Client users", "Nobody yet — a spike"},
```

Five questions, each with four **pickable** options and one line of help. A person who answers
*"Nobody yet — a spike"* by clicking is the person this phase was designed for, and `AGENTIC-UX.md`
line 22 states the assumption: *"The human is intermittently present; the work is continuously
running."*

Now read what §2 and §6.3 would put in front of that person:

- up to **40 grill questions** over **6 rounds of 8** (§2.2);
- a glossary row asking for a canonical word, a 1–2 sentence definition of what it *is* and the
  rejected synonyms (§6.2);
- a decision form with a title, 1–3 sentences of context, **three mandatory checkboxes**, a fourth
  required field that appears when the third is ticked, and **Save disabled until all four** (§6.3).

`CLAUDE.md`: *"If a feature is powerful but takes a wall of configuration to reach, it has failed our
bar."* A four-gate form in front of writing one sentence is that wall. The source skill's user is a
staff engineer who *asked* to be grilled in a terminal; ours picked an option from a list of four.
The design never names that difference, and §6 holds itself only to the visual rules of
`DESIGN-SYSTEM.md`, never to the ceremony budget.

### 3.2 Three mechanisms that do not transfer

1. **"Wait between rounds" assumes a session.** Their round boundary is a chat turn: the human is
   there. Ours is a person who may return in two days, on another machine, to a brief a teammate has
   since revised. §2 has no answer for a round whose frontier changed underneath it — `Apply` will
   happily accept `grill_round` against a revision that is no longer head, and §2.6 gives
   `grill_round` no expected-revision guard, while `Grill.Rounds++` makes every round a write.
2. **A stall detector needs an interlocutor.** §2.4 terminator 3 closes the interview when a round
   "settles no new question". Asynchronously, "the person skipped everything" and "the person has not
   come back yet" look identical, and only one of them should close an interview permanently
   (`Grill.Closed` has no reopen op).
3. **The model proposing vocabulary is not the model that will use it.** §2.7 and §3.2 correctly make
   a proposed term `model_drafted` and unrendered until a person settles it. Good — but it means the
   grill's *output* is invisible to every agent until a human does per-row review work, in a product
   whose premise is that the human is intermittently present. The mechanism is right and its
   throughput assumption is the chat window's.

---

## 4. Does it break the thread?

Mostly no — and that is the design's real achievement. Terms and decisions as **fields of a brief
revision** (§3.1) inherit revision identity, immutability, the `4-D3` pin and materialisation before
`ptysession.Start`. `contextwrite/materialise.go` is built and does exactly what §5.4 relies on. On
front 4 the design largely does what it claims.

**Three places where it breaks the thread anyway.**

### 4.1 `Also defined:` hands an agent a word with no way to reach its meaning

§5.4 rule 6 caps the block at `MaxInlineTerms = 40`; past it, *"the remainder renders as words only,
one line, comma separated, under `Also defined: `"*. The design sells one property above all others:

> the agent that uses the word is provably the agent that was handed the definition

Past the cap, that is false — the agent is handed the **word** and not the definition, the session is
still stamped with the key, and **no lookup path is proposed anywhere in the document**: no MCP read,
no route the sandbox may call, no file. "Progressive disclosure" with nothing to disclose *to* is not
disclosure; it is a silent truncation that the provenance stamp then certifies. Either build the
lookup or drop the cap and pay the context — but do not ship a stamp that stops being true at term 41.

### 4.2 Loose-file glossary is forbidden for the product and recommended for us

§3.1 rules out a repo-root `CONTEXT.md` in two lines: `OD-3` puts shared state in the Hub, and a
loose file has no revision so nothing can cite which definition an agent read. `00-summary.md` §3
practice item 5 then recommends exactly that file for this repository (`~2 h`). It is defensible —
we have no Hub for our own repo — but neither document says so, and an argument that a mechanism is
wrong in principle and right for us needs the sentence that distinguishes them.

### 4.3 Process state inside immutable content

§2.6 puts `Grill{Rounds, Closed, ClosedBy}` inside `Content`, calling it *"the only stored process
state"*. `Content` is what the revision digest covers and what `claudemd` renders. So `Rounds++`
mints a revision, changes the content digest, and changes nothing a downstream consumer can use —
six rounds plus per-answer accepts is tens of revisions on a Hub whose revision numbers are gapless
and user-visible. The thread is not broken, but it is diluted: *revision 31* stops meaning anything.

---

## 5. Two internal contradictions that would reach an implementer

These are not objections to the idea. They are defects in the document a plan editor would build.

### 5.1 The branch's only way to dissent from a project decision is refused by `Apply`

§5.3 makes cross-scope supersede the mechanism by which a branch disagrees:

> The supersede chain is followed **across scopes**: a branch decision with `Supersedes` naming a
> project decision drops the project's from the document. That is the mechanism by which a branch
> says *not here* about a project-wide choice, and it is one field, not a new concept.

§4.3 forbids it:

> `record_decision` … `Supersedes` **must name a live decision of this content** or `""`

A branch content never holds a project decision, so `Apply` rejects every write §5.3 depends on.
The design itself states why the two cannot both be true — §3.3: *"`Apply` has no scope-crossing
path to do so, because it only ever sees one content object."*

### 5.2 Retiring an inherited term has no op

§5.3: *"a branch may **retire** a project term for its own work by defining a retired term with the
same word and a `Because`."* §3.3's op table has no op that does this: `define_term` appends a term
(no `Retired`), and `retire_term` sets `Retired` on a term **of this content**, which a project term
is not. Same class of error as §5.1, and the same root cause: §5.3 was written against the resolved
`Document` and §3–§4 against a single `Content`, and nobody reconciled them.

### 5.3 A third, smaller one

§5.3 says a project/branch decision `ID` collision is *"`ErrBadOp` at write time"*. `Apply` cannot
detect it at write time for the same reason as §5.1: it sees one content object. It is a resolve-time
condition or it is nothing.

---

## 6. Intellectual property — clean, with one caveat

I checked rather than assumed.

- **No prose of theirs is in this repository.** Grepping all six files for quoted strings over 40
  characters returns five hits, and every one quotes **our own** documents (`"Materialise runs
  BEFORE ptysession.Start"`, `"usability and iteration on powerful tools"` from `CLAUDE.md`). There
  are no block quotes in `01-define-skills.md`. Every skill description is paraphrase with a URL and
  a date.
- **`01-define-skills.md` states the position correctly and does not lean on the licence:** *"MIT
  permits reuse with attribution, but this document deliberately does not rely on that."*
- **Method vs expression is drawn right.** A frontier ordering, a three-leg decision test, a
  two-artifact prototype rule — ideas. What we would build carries our field names and our wording.
- **Caveat, already flagged by the summary and not closed:** the MIT claim was read off the GitHub
  repository page; **`LICENSE` itself was not opened**. Nothing depends on it today because nothing
  was copied. Open it before anyone ever copies a line.
- **One residual risk worth a sentence in the plan:** §4.1's three legs are labelled *hard to
  reverse* / *surprising without context* / *a genuine trade-off*. If those are close to the source's
  own phrasing, they must not become on-screen product copy verbatim (§6.3 puts them on checkboxes).
  Reword them in the lens, or verify the source's wording differs first.

---

## 7. The one change worth making now

> **Run the no-op pass over `claudemd.Render`'s managed block, and move its two human-facing lines
> into the marker comment.** `05-design.md` §9.2, `00-summary.md` §3 product 5. About an hour.

It is in the built golden today:

```
testdata/golden-branch.md:1  <!-- panacea:discover:begin brief=bf-00000000000000a2 rev=5 -->
:4  Written by Discover in Panacea App. Every session started on this branch reads it first.
:5  Change it in Discover, not here: this block is regenerated and your edits inside it will be lost.
```

Two lines that change no agent's behaviour, paid for in **every session on that branch, for ever**.
Why this one and not the others:

1. **It is a deletion.** No new type, no new op, no new key form, no new screen.
2. **It touches nothing outside `claudemd`** — no Hub contract, no wire format, no `ContentVersion`,
   none of the 64 plan files that quote a brief key.
3. **It is the only proposal whose cost estimate survived contact with the built tree.**
4. It should be done **whether or not** anything else here is ever adopted.

**What earns a place on the shelf, not in the plan.** *Terms and decisions as fields of a brief
revision* is a genuinely good idea and the gap it fills is real (§1.1). Put it behind three
conditions: phase 4 ships first; the fields land as **`ContentVersion = 2`**, not by widening a v1
that has shipped in code; and the criterion fragment is **dropped entirely** — cite a criterion the
way phase W already does, `{briefKey, criterionId}`, and give terms and decisions the same shape.
That removes the Hub contract change, the two-parser edit and the 64-file blast radius, and it costs
the design nothing it actually needed.

**Why the rest waits.** Grill rounds (§2) need a product decision about how much ceremony our user
will bear, and that is the owner's call, not a plan editor's. The brief→work generator needs phase W
to stop moving. Everything in phases T and W needs a re-anchoring budget nobody has written down.
The practice items are mostly free and mostly fine — the three-leg test for what earns an `OD`, the
*unblocked by* / *unblocks* columns, the two-artifact prototype rule — but they are habits, not work,
and they do not need a plan at all.

---

## 8. What this critique did not verify

1. **No page of aihero.dev was fetched.** Every claim about the source method is carried from
   `01`–`04`, which record their own fetches on 2026-09-28. I could not therefore check whether
   §4.1's three leg labels are close to the source's wording — the residual IP risk in §6 stays open.
2. **Phase 3's plan was not read.** Summary item 4 (two unmerged review axes) is unassessed here, as
   it was there.
3. **I did not run `go test`.** Line and test-function counts are `wc -l` and `grep -c 'func Test'`
   over the phase-4 worktree at `ca737d92`; the 3–4× cost multiplier in §2.1 is a judgement from
   those counts, not a measurement.
4. **Phase W and phase T were read by grep and by targeted range**, not in full. The citation counts
   (2,544 / 1,344) use the pattern `file.ext:NNN` and will differ from the brief's figures, which
   used a different pattern; the conclusion — that these plans are expensive to move — does not turn
   on which pattern is right.
5. **The Hub's built parser was read in its plan part, not in its source.** `phase-h-hub-parts/
   task-01.md:196` carries the regexp; I did not open the Hub's `key.go`.
6. **Read-only throughout.** No plan, no source and no worktree was modified; this file is the only
   thing written.

---

**Recommendation quality: Needs fixes**

