# 08 — Independent review: `CONTEXT.md`

Reviewed 2026-09-28 against the tree at `1c7799ce`, read-only. Commits under review:
`a5706570` (CONTEXT.md, 298 lines + 2 lines of CLAUDE.md) and `1c7799ce` (the implementer's report).

**Verdict in one line.** The file is true — every citation I opened resolves, and no definition
contradicts the file it cites — and it is worth having. Four things should be fixed before it is
treated as settled: one of them is a sentence in `CLAUDE.md` four lines above the pointer this work
added, which the file's own table forbids.

---

## 1. Are the citations true? — 14 opened, 14 resolve

| Entry | Citation | Opened | Result |
|---|---|---|---|
| Machine | `ui-contract.md:408-439`, `go-contract.md:1866`, `PROGRAM.md:46`, `FUNCTIONAL-SPEC.md:44-46` | yes | ✔ `Machine`/`MachineKind`/`MachineState` are in range; `§3.4 GET /v1/machines` is at 1866 exactly; `PROGRAM.md:46` is the "the Docker backend *is* that feature" sentence |
| Session | `ui-contract.md:529-557`, `FUNCTIONAL-SPEC.md:35-38` | yes | ✔ `Session` is 529-557 exactly; the lifecycle section carries "Finished sessions stay listed and readable — the transcript is the artifact" and "Resume and Fork both start a **new** session" verbatim |
| contextRef | `ui-contract.md:555-556`, `go-contract.md:3045` | yes | ✔ both exact; §7.6 is titled "`provenance.Ref` and `contextRef` — the field Phase 2 must carry" |
| Lens | `ui-contract.md:564-566` | yes | ✔ exact, and the type's own comment reads "The six lens rows of the sub-rail … rule 12" |
| The rail | `FUNCTIONAL-SPEC.md:50`, `ui-contract.md:617-631` | yes | ✔ §2.1 carries "four levels, and a worktree row is never a session row" |
| Harness | `ui-contract.md:483-520`, `PROGRAM.md:129` | yes | ✔ `KnownHarnessId` is the five named ids; :129 is the never-reimplemented rule |
| Worktree | `OD-2` at `owner-decisions.md:50`, phase-3 plan `:68-75` | yes | ✔ OD-2 is line 50; "**The model (OD-2).** A cockpit worktree is a local clone…" starts at 68 |
| Lane | phase-3 plan goal 3 `:28-31`, syncer `:96-100` | yes | ✔ `refs/panacea/lanes/<worktree-id>` at 29; the lane syncer paragraph starts at 96 |
| Convergence | `task-08.md:129-146`, goal 4 `:32-35` | yes | ✔ `type Convergence struct` is the body of `GET …/convergence`; goal 4 is "A human lands worktrees … one at a time, in the order they set" |
| Brief | `discover/content.go:1-6` | yes | ✔ package comment; and `BaseQuestions is the closed set` at `:90` independently supports "a **closed** set of questions" |
| Brief revision | `provenance/key.go:23-29`, `hub-contract.md:120` | yes | ✔ the grammar block is 23-29 exactly; :120 is the `brief key` line |
| Criterion | `content.go:160-166` | yes | ✔ exact, including the "minted once and kept for as long as a line with exactly this Text survives" rule |
| The Hub | `hub-contract.md:9-11` | yes | ✔ §Words, exact: "*The Hub* is the server `control-plane/cmd/hub`. *The control plane* is the existing local process `control-plane/cmd/api`." |
| The Materialiser | `contextwrite/materialise.go:1-21` | yes | ✔ "THE ORDERING RULE … Called afterwards, the session inherits nothing, silently, and the feature looks built" is the source's own sentence |

Also checked and correct: `SEAMS.md:1-11` (definition + the two check commands), `RESUME.md:96-118`
and `:176-184` (the never-relaunch rule and the seven stream ids), `AGENTIC-UX.md:79` rule 12,
`OD-3:51`, `OD-5:53`, `OD-16:164`, `OD-19:177`, `OD-24:189` — all exact.

**No Critical.** Nothing in the file is contradicted by the file it cites. The one place a
definition disagrees with a citation — Lens, six against rule 12's four — is disclosed in the entry
and carried to Open questions, which is the right handling.

---

## 2. Important — four to fix

**I1. `CLAUDE.md` line 3 uses the phrase its own new pointer forbids.** The unchanged sentence reads
"one **git worktree** per agent, streaming back to a single cockpit". `CONTEXT.md`'s table bans
*git worktree* for *worktree* (`OD-2`), and the worktree entry says a worktree holds **many
sessions** and outlives them — so "one per agent" is also the wrong cardinality (`ui-contract.md`
§2.1). This is four lines above the line the commit added, in the file that tells the reader to go
read the table. Fix: "one isolated worktree per session" or similar, in `CLAUDE.md` line 3.

**I2. `Authority` is missing, and it is the clearest case the file exists for.** `AGENTIC-UX.md:37`
rule 6 and §7 (`:177-183`) define Authority as a **grid**: nine action classes × four named levels
(ASK FIRST · PROPOSE AND WAIT · ACT AND TELL ME · FULL), inherited project → branch → agent, and
movable at runtime by the agent itself. `FUNCTIONAL-SPEC.md:52` puts it in the Develop top bar. The
word a reader will reach for instead is the shipped `PermissionMode = "ask" | "bypass" | "plan"`
(`ui-contract.md:495`) — three values, one axis, no inheritance. Getting that wrong changes what
someone builds, which is this file's own admission test.

**I3. "Queue" has two meanings and `CONTEXT.md` uses both without saying so.** `AGENTIC-UX.md` rule
4 has **one ranked attention queue** ("Fails if a count appears anywhere outside the queue", 10
uses) — which the cockpit entry leans on: "counts outside the queue are banned". Phase 3 has a
different queue: `QueueEntry`, ready lanes in landing order (`task-08.md:119-127`, 9 uses in the
plan) — which the Lane entry leans on: "plus the queue row it lands from". Two senses, both load
bearing, neither distinguished. Needs an entry, or the Orchestrator entry's two-senses treatment.

**I4. `Approval`, `Release` and `Evidence` are named in an entry and left undefined.** The Hub entry
says the Hub holds "briefs, approvals, the provenance ledger, **releases** and teams". The Hub
contract gives each a section — §7.7 *Design approvals — the design link*, §7.9 *Releases — reserve,
deploy, confirm*, §7.10 *Evidence — phase 3's H-2* — and uses them 52, 43 and dozens of times. Three
of the Hub's five listed contents have no entry. The implementer's note that phases 5 and 7 were not
opened is fair for canvas *frame*, but these three are in a contract `CONTEXT.md` already cites.

---

## 3. Minor — and what to cut

**M1. Cut `Agent`.** "One running instance of a harness doing work, with an owner" is what any
engineer assumes. The only sharp part — harness is the program, agent is what runs in it — is
already in the Harness entry *and* in the do-not-use table. What remains (billing follows the owner,
no cross-user steering) is `OD-16` policy, not vocabulary, and is already under Orchestrator. Fold
*sub-agent* into Orchestrator and delete the entry.

**M2. Cut or compress `Warm pool`.** "Sandboxes pre-provisioned and kept running" is the standard
meaning of the phrase. The non-obvious content is product behaviour — claimed at first sign-in, no
create form, sizing only asked for a second machine — which belongs in the Machine entry as a clause.

**M3. `Branch` is the third-weakest.** Keep it, but trim: "a git branch **and** the product's main
unit of work" is nearly assumed; the load-bearing half is that it carries its own brief, its own
Environment and its own sub-rail.

**M4. The `Provenance` entry cites the wrong section.** `provenance-contract-draft.md` §2.1 is
"Two stores, one of them new" — storage layout, and the section the *ledger* entry correctly cites
at `:160` and `:168-170`. It does not fix the meaning of provenance or the thread. §1.1 *Name*,
§1.4 *The view every surface renders*, or §2.4 *What `contextRef` now means* do.

**M5. `PROGRAM.md:35-46` is labelled "the `Backend` interface".** The interface is lines 34-40; 46
is the prose sentence about the local machine (which the Machine entry already cites). Narrow it.

**M6. The closing rule contradicts the Lens entry.** "When a definition and its cited file disagree,
the file wins and this file is wrong" has no exception, yet Lens knowingly states six against a
cited file that says four. Add the exception the file already practises: *unless two cited files
disagree with each other — then record it under Open questions and change nothing.*

**M7. `The Desk` is undefined.** `AGENTIC-UX.md:85-87` names it the primary surface,
"**account-level, above projects** … a surface you **empty**". It is not in the phase 2 contracts,
so this is low priority — but it is the one surface noun a reader meets in the governing UX document,
and without an entry they will call it a dashboard, which the table bans.

---

## 4. Is the `CLAUDE.md` addition small, and does it disturb anything?

Yes and no, respectively. `git show a5706570 -- CLAUDE.md` is a clean two-line insertion (one
sentence plus its blank line) at line 7, after the "may host more phases later" paragraph and before
`## Why this exists`. No surrounding line moved; no heading, rule or decision was touched. The style
matches — bold lead, em-dash list, backticked path. The only complaint is I1 above: the sentence it
now sits under is the one the pointer would have the reader correct.

---

## 5. Open question for the owner — unchanged from the implementer's, and correctly left open

The lens count. `AGENTIC-UX.md:79` rule 12 says "**Four lenses**, no pipeline" and `:217` repeats
"Keep four lenses". The shipped `Lens` type (`ui-contract.md:566`) and the sub-rail
(`FUNCTIONAL-SPEC.md:51`) both carry **six**, adding Files and Merge — and the type's own comment
cites rule 12 while listing six, so the contract already made this choice silently. The rule (no
order, no numbering, no completion state) is not in doubt; only the count is. Recording it rather
than deciding it was right. I add one fact the implementer did not: the conflict is already inside
the contract, not only between two documents, so whichever way it goes, `ui-contract.md:564-565`
needs the same edit.

The second conflict — `CLAUDE.md`'s **Define** against the lens, phase and package all called
**Discover** — is real and also correctly left open.

---

## Summary

| | Count |
|---|---|
| Critical | 0 |
| Important | 4 — I1 `CLAUDE.md` "git worktree per agent"; I2 `Authority` missing; I3 two queues; I4 approval/release/evidence |
| Minor | 7 — cut `Agent`, compress `Warm pool`, trim `Branch`, 2 citation fixes, the self-contradicting closing rule, `The Desk` |

The file is accurate and it is short enough to read in full, which is the thing that makes a glossary
survive. Fix I1 in the same sweep that adds I2 and I3, and it is done.

**Task quality: Needs fixes**
