# 08 — `CONTEXT.md`: report

**Date 2026-09-28.** Adopts `00-summary.md` §3 practice item 5, under the constraint `06-sceptic.md`
§4.2 raised against it. Committed as `a5706570` on `feat/phase-2-cockpit`: `CONTEXT.md` (new, 298
lines) and two lines added near the top of `CLAUDE.md`. **Documentation and convention only** — no
product code changed, no test suite run, no container started.

---

## 1. What was written

`CONTEXT.md` at the repository root: **26 entries**, one or two sentences each, every one grepped in
the real tree before it was defined and every one citing the file that fixes its meaning.

All 26 terms the task named are covered: machine (with local vs cloud), project, brief, brief
revision, criterion, branch, worktree, lane, convergence, session, sandbox, harness, agent, the
cockpit, the rail, lens, the Hub, the ledger, provenance and the thread, `contextRef`, the
Materialiser, seam, warm pool, data classification, orchestrator, stream.

Grouped so a reader can find one without reading all of them: *Where work happens* (machine →
sandbox → harness → agent) · *What we agree, and what carries it* (brief → revision → criterion →
provenance → `contextRef` → ledger → Hub → Materialiser) · *What a person looks at* (cockpit, rail,
lens) · *Platform words* (warm pool, data classification, orchestrator, stream) · *How we build it*
(seam).

**The discipline that keeps it short.** A term that means what any engineer would assume is left
out — there is no entry for commit, token, PTY, digest or Postgres. Every entry that rejected a
synonym names it and says why, because the rejected word is what someone reaches for otherwise.

## 2. Where each meaning is fixed

The citations that carry the most weight, all resolved against the file on 2026-09-28:

| Term | Fixed by |
|---|---|
| seam | `docs/SEAMS.md:1-11` — an id, a row, a test anchor; not any boundary |
| lane, convergence | phase 3 plan `:28-35`, `:96-100`; `LaneView`/`QueueEntry`/`Convergence` in `…phase-3…parts/task-08.md:112-146` |
| worktree | `OD-2` (`owner-decisions.md:13`) + phase 3 plan `:68-75` — a local clone, not a `git worktree` |
| the Materialiser | `control-plane/internal/contextwrite/materialise.go:1-21` — the ordering rule is the definition |
| brief, criterion | `control-plane/internal/discover/content.go:1-6`, `:57-59`, `:160-166` |
| brief revision | `provenance/key.go:23-29`, verbatim from `hub-contract.md:120` |
| `contextRef` | UI contract `:555-556`; Go contract §7.6 `:3045`; `provenance.Declined` |
| the Hub vs the control plane | Hub contract §Words `:9-11` — two processes, and the words are not interchangeable |
| the ledger | `provenance-contract-draft.md:160`, `:168-170` |
| rail, lens, cockpit | UI contract `:396`, `:564-566`, `:617-631`; `AGENTIC-UX.md:79`; `FUNCTIONAL-SPEC.md:50-51` |
| machine, sandbox | UI contract `:408-439`; Go contract `:1866-1896`; `PROGRAM.md:35-46` |
| warm pool | `PROGRAM.md:137`, `:119-122`; `FUNCTIONAL-SPEC.md:46` |
| data classification | `OD-24` `:189`, `OD-19` `:177` |
| orchestrator | `OD-16` `:164` (product) vs `docs/superpowers/RESUME.md` (ours) |
| stream | `docs/superpowers/RESUME.md:96-118`, `:176-184` |

Twelve of the citations point into sibling worktrees (`phase-3`, `phase-4`, `hub`) because that work
has not landed on `main`. `CONTEXT.md` says so in its header and tells the reader to prefer the
named declaration over the line number — the rule `docs/SEAMS.md` learned the expensive way.

## 3. Words we do not use

Fourteen rows, each a word someone will reach for by default and the word to reach for instead. The
four that have actually cost time in this tree:

1. **seam**, for any interface or test boundary. `docs/SEAMS.md` owns it as an id with an anchor;
   borrowing it makes the manifest unfalsifiable. TDD's boundary is a **test boundary**
   (`00-summary.md` §3, practice 7).
2. **stream**, for PTY bytes. A stream is one line of our own build work; the bytes are *the attach*.
3. **the backend** / **the API**, for the server. Two processes, two identities, two hosts.
4. **Orchestrator**, bare, for ours. `OD-16` owns the word for the product's only cross-user channel;
   ours is *the workflow orchestrator*.

## 4. The one thing that needs an owner decision

**How many lenses are there — four or six?**

- `docs/AGENTIC-UX.md:79` rule 12: *"Four lenses, no pipeline. Discover, Design, Develop, Deploy are
  simultaneous views on a branch"*, and `:217` repeats "Keep four lenses".
- The shipped type, UI contract §2 (`…ui-contract.md:564-566`):
  `type Lens = "discover" | "design" | "develop" | "files" | "merge" | "deploy"` — **six**.
- The sub-rail, `docs/FUNCTIONAL-SPEC.md:51`: *Discover, Design, Develop, Files, Merge, Deploy* —
  **six**.

The *rule* is not in doubt and nothing here weakens it: no order, no arrows, no numbering, no
completion state, entry legal from any state. What is in doubt is the **count in that one sentence**,
and it matters because phase 3 ships two of the six (Merge and Files are its lenses, plan goal 7).

Two ways out, and it is the owner's call, not a plan editor's:

1. **Rule 12's count is stale** — amend it to six and name Files and Merge as lenses. Cheapest; one
   line in `AGENTIC-UX.md`, and the type, the sub-rail and phase 3's plan all already agree.
2. **Files and Merge are not lenses** — they are something else (views on a worktree rather than on a
   branch) and need their own word, which would mean renaming the `Lens` type and its six members.

`CONTEXT.md` states **six**, because that is what the code and the sub-rail carry, and records the
conflict as an open question rather than resolving it (rule 6 of this chain's brief).

A second, smaller one is recorded beside it: `CLAUDE.md` names the journey **Define** → Design →
Develop → Deploy, while the lens, the phase and the Go package are all called **Discover**. No file
in the tree fixes "Define" against "Discover", so no entry defines it. If they are the same thing,
one of the two words should go.

## 5. What this did not do, and what it did not verify

- **No product code, no test run, no container.** The commit touches `CONTEXT.md` and two lines of
  `CLAUDE.md` only.
- **No file owned by another chain was touched.** `control-plane/**`, `sandboxd/**`, `desktop/**` and
  the phase 8 and phase 6 worktrees were read only. Nothing needed was locked by another agent.
- **Line numbers in sibling worktrees will drift** as those streams commit. The file says so and
  names the declaration in every case where the number could rot; the phase-4 and hub numbers in
  particular sit on branches that are mid-build.
- **Phase 5's and phase 7's plans were not opened.** Two terms that might deserve entries later —
  *frame* (canvas) and *release* / *environment* (deploy) — were out of the task's list and are not
  defined here; *Environment* appears only as the rejected synonym for *machine*.
- **`CONTEXT.md` is a loose file with no revision**, which is exactly what `06-sceptic.md` §4.2 said
  the *product* must not ship. That objection is answered rather than ignored: it holds for the
  product because `OD-3` puts shared state in the Hub and a loose file cannot be cited by revision;
  it does not hold for this repository, which has no Hub of its own and whose agents read the working
  tree directly. The file is under git, so it does have a revision — the commit.
- **Nothing from aihero.dev is in this file.** The method (a project glossary, own nouns only,
  rejected synonyms named) is theirs; every sentence, every term and every citation is ours.

---

## 6. Fix round — 2026-09-28, against review `08-context-review.md`

Commit `5bc068b8`, `CONTEXT.md` + `CLAUDE.md`, no product code, no test run, no container. The
review's verdict was *true but incomplete*, so this round adds what was missing and repairs one
sentence that the file itself forbids. Every citation added below was opened in the real tree before
it was written; none was carried over from the review on trust.

### The four Importants

**I1 — `CLAUDE.md` line 3, fixed first because it was self-refuting.** It read "one **git worktree**
per agent", which is the phrase `CONTEXT.md`'s own table bans (*git worktree* → *worktree*, `OD-2`)
and the wrong cardinality (a worktree holds many sessions, UI contract §2.1) — four lines above the
pointer telling the reader to go read that table. It now reads "each session in its own sandbox on
an isolated worktree". That phrasing is true to both entries at once: a **sandbox** is per running
session, a **worktree** is the isolated checkout many sessions share. Nothing else in `CLAUDE.md`
moved — no heading, no rule, no decision — because every agent in every worktree reads it.
"Streaming back to a single cockpit" was kept deliberately: the Stream entry already names it as
product prose, not the noun.

**I2 — `Authority` added**, in Platform words. Nine action classes × four named levels (ASK FIRST ·
PROPOSE AND WAIT · ACT AND TELL ME · FULL), never a slider, inherited project → branch → agent with
the inherited value shown greyed in place, and movable at runtime in both directions — the agent may
request elevation and must self-demote on uncertainty. Cited to `docs/AGENTIC-UX.md:37` (rule 6) and
§7 (`:177-183`), plus `docs/FUNCTIONAL-SPEC.md:52` for the Develop top bar. `PermissionMode`
(`…ui-contract.md:495`) is named as the thing it is **not** — three values, one axis, no inheritance,
no action classes — both in the entry and as a new row in the do-not-use table.

**I3 — `Queue` split into its two senses**, using the Orchestrator entry's treatment. Sense 1 is the
attention queue (`docs/AGENTIC-UX.md:31`, rule 4): one ranked list, the only place a count may
appear. Sense 2 is the landing queue: `QueueEntry`, the ready lanes of one branch in human-set
landing order with their forecasts (`…task-08.md:119-127`). Rule: bare "the queue" is sense 1; sense
2 is always written "the landing queue". Three existing sentences that leaned on one sense without
saying which were amended — the Lane entry ("the landing-queue row it lands from"), the cockpit entry
and the *dashboard* table row ("counts outside the **attention** queue are banned").

**I4 — `Design approval`, `Release` and `Evidence` defined.** Three of the five things the Hub entry
lists as the Hub's contents had no entry. Each is now one sentence of the thing that is not obvious,
cited to its own section of the Hub contract: the approval is written by **one** call that fixes
design revision, brief revision and file snapshot together (§7.7, `:872`); the release is a
**reservation taken before the deploy exists** — `reserved` → `confirmed`, else `abandoned` or
`expired`, and if the pin fails Ship does not happen (§7.9, `:957`); evidence is a `settled` /
`contested` verdict at one commit, keyed by brief key + criterion id + criterion digest so an edited
criterion line does not inherit the old verdict (§7.10, `:1060`).

### The Minors

- **M1, `Agent` cut.** Its only sharp half — harness is the program, agent is what runs in it — was
  already in the Harness entry and in the table. *Sub-agent* and the `OD-16` ownership rule
  (accountability and billing follow the owner; no user steers another user's agent) moved into
  Orchestrator sense 1, where the cross-user rule already lived.
- **M2, `Warm pool` cut.** "Pre-provisioned and kept running" is the standard meaning. The product
  behaviour that was not obvious — a cloud machine is **claimed**, no create form, sizing asked only
  for a second machine or after an OOM — is now a clause in the Machine entry, with `PROGRAM.md`
  `:119-122` added to that entry's citations.
- **M3, `Branch` trimmed** to the load-bearing half: its own brief, its own Environment, its own
  sub-rail.
- **M4 and M5, both citations fixed.** Provenance cited `provenance-contract-draft.md` §2.1, which is
  storage layout and the *ledger* entry's section; it now cites §1.4 *The view every surface renders*
  (`:129`) and §2.4 *What `contextRef` now means* (`:215`). The `Backend` interface is
  `docs/PROGRAM.md:34-40`; `:46` is the prose sentence the Machine entry already cites.
- **M6, the closing rule now states the exception it already practises.** "When a definition and its
  cited file disagree, the file wins" gains: when two cited files disagree with *each other*, or a
  file disagrees with itself, state what the code and shipped surfaces carry, say so in the entry,
  and record it under Open questions — which is why Lens states six against a rule that says four.
- **M7, `The Desk`, not done and deliberately so.** It is the one surface noun in `AGENTIC-UX.md`
  (`:85-87`, account-level, above projects, a surface you empty) with nothing in the phase 2
  contracts to fix it. Adding an entry whose only citation is the UX document would define a word the
  code has not yet met. Left for the phase that builds it; recorded here so it is not lost.

Net effect on length: 298 → 340 lines. Five entries added, two removed, three trimmed.

### Owner question added — the lens conflict is inside one contract

The review's new fact is now in `CONTEXT.md` Open question 1 rather than being ruled on: the `Lens`
type's own comment at `…ui-contract.md:564-565` lists **six** lens rows while citing rule 12, and
rule 12 says **four**. The contract therefore contradicts itself, not merely the UX document. This is
a judgement for the owner, and it has a consequence either way: whichever count is correct,
`ui-contract.md:564-565` needs the same edit as `AGENTIC-UX.md:79`. No file was changed on the
strength of a guess.
