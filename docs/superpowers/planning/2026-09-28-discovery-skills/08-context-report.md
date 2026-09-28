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
