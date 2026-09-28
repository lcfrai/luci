# 08 — Independent review, round 2: `CONTEXT.md` after the fix round

Reviewed 2026-09-29 against the tree at `343a0276`, read-only. Under review: `5bc068b8`
(CONTEXT.md + CLAUDE.md), `24c5b7a4` (report §6), `31229e7a` (the Authority count), and
`343a0276`, which is a *later* commit from the sibling context chain that re-anchored five
`owner-decisions.md` line numbers inside `CONTEXT.md`. All five re-anchors were opened and
resolve.

**Verdict in one line.** The four Importants were done and the two cuts were the right two —
but the round's own shape repeated itself twice: the entry that split "queue" into two senses
cites the wrong one of `AGENTIC-UX.md`'s two numbered lists, and the file breaks its new
"never bare" rule twice in the entries the fix was meant to reach. No Critical: nothing in the
file contradicts the file it cites.

---

## 1. Question 1 — is `CLAUDE.md` line 3 fixed, minimal, and in voice?

Yes to all three. `git show 5bc068b8 -- CLAUDE.md` is `2 +- 1`: one clause inside one sentence.
No heading, rule, decision or blank line moved, and the pointer added last round (`:7`) is
untouched.

"each session in its own sandbox on an isolated worktree" clears both halves of I1 — the banned
`git worktree` is gone, and the cardinality is now right against `ui-contract.md` §2.1 and
`FUNCTIONAL-SPEC.md:18` ("a worktree holds many sessions"). "isolated worktree" is the house
phrasing (`FUNCTIONAL-SPEC.md:13`, "an isolated checkout"), and "in its own sandbox" matches the
Sandbox entry's own cardinality (`FUNCTIONAL-SPEC.md:10`, "one per running session"). Voice
matches: the sentence still reads as owner prose, not as a definition.

One residual, and one thing the round did not look for — the second is **I3** below.

*Nit:* "each session in its own sandbox on an isolated worktree" survives a hasty reading that
carries "its own" across to "worktree". The careful reading is right and is the one the report
states; if it is ever re-touched, "on a worktree that holds many" removes the last ambiguity.

---

## 2. Question 2 — the five definitions against their cited files

Every citation below was opened this run.

| Entry | Cited | Result |
|---|---|---|
| **Authority** | `AGENTIC-UX.md:37`, `:177-183`; `FUNCTIONAL-SPEC.md:52` | ✔ The ten action classes are `:179` **verbatim, in order**; the four level names, "never a slider", the project → branch → agent inheritance and greyed-in-place are all `:179`; request-elevation and self-demote-on-uncertainty are `:183`. `:52` is the Develop top bar and names Authority. Label nit at **M2**. |
| **Not «permission mode»** | `ui-contract.md:495` | ✔ `export type PermissionMode = "ask" \| "bypass" \| "plan";` at 495 exactly. |
| **Queue, sense 2 (landing)** | `task-08.md:119-127` | ✔ exact: `// QueueEntry is one ready worktree in landing order`, struct through `Settles`, `Forecast` on every row. |
| **Queue, sense 1 (attention)** | `AGENTIC-UX.md:31` "rule 4" | ✘ **wrong list** — see **I1**. The content is true; the anchor does not carry it. |
| **Design approval** | hub-contract `:872` | ✔ §7.7 exactly; `:874` is "One route writes **the approval, the design link and the snapshot reference together**", `:875-876` is "Phase 5 uploads the snapshot's files first (§7.6), then approves". |
| **Release** | hub-contract `:957` | ✔ on the reservation model, `reserved`/`confirmed`/`abandoned`/`expired`, and the pin taken before the deploy exists (`:960`, `:965-973`). One compression at **M3**. |
| **Evidence** | hub-contract `:1060` | ✔ §7.10 exactly: `briefKey` + `criterionId` + `criterionDigest` + `commit` + `verdict: settled \| contested` + `because`. |

**No Critical.** No definition contradicts its citation.

Also re-opened and correct: `provenance-contract-draft.md:129` (§1.4 *The view every surface
renders*) and `:215` (§2.4 *What `contextRef` now means*) — M4 fixed properly; `PROGRAM.md:34-40`
(the `Backend` interface, M5 fixed); `owner-decisions.md` `:13` OD-2, `:14` OD-3, `:16` OD-5,
`:127` OD-16, `:140` OD-19, `:152` OD-24 — all six re-anchors land on the right rows, and OD-24
supports the Data classification entry word for word ("set on creation … can be updated … AU vs
Global … which security scans the deploy side requires").

**The implementer's correction to the last review is right, and I confirm it independently.**
`AGENTIC-UX.md:179` states no count and lists **ten** classes. "Nine" was the review's error;
dropping the count rather than inventing a ruling was the correct handling.

---

## 3. Important — four

**I1. The Queue entry cites the wrong one of `AGENTIC-UX.md`'s two numbered lists, and the
sentence that carries the whole entry is uncited.** `CONTEXT.md:216` cites "`docs/AGENTIC-UX.md:31`
rule 4". `:31` is §2's item 4, *"Attention is the scarce resource, not screen space"* — which
supports only the entry's **second** sentence ("ranking and batching, not display … a pane is a
cost"). The **first** sentence — "the one ranked list of what needs a human, and the only place in
the cockpit a count may appear" — is fixed by a different line in a different list: §4 *Principles*
("Twelve rules", `:66`) **rule 4 at `:71`**: *"One ranked queue. No scattered badges. Rank = cost
of delay × blast radius × irreversibility … Fails if a count appears anywhere outside the queue."*
Hard rule 10 at `:245` repeats the ban. This matters beyond tidiness: the file's premise is that a
reader can open the citation and check the definition, and opening `:31` checks nothing about
ranking or counts. It also mislabels — §4 rule 4 and §2 item 4 are different objects.
*Fix:* cite `:71` for the ranked-queue and count rule (`:245` optionally), keep `:31` for the
pane-is-a-cost sentence, and drop the "rule" label from `:31`.

**I2. The file breaks its own new "never bare" rule twice, in the two entries the fix was for.**
`CONTEXT.md:220` now says: *bare "the queue" means sense 1; sense 2 is written "the landing queue",
**never bare**.* Two sense-2 uses were left bare:
- `:64` (Convergence) — "It is the ordered whole, including the forecast, **the queue**, the land
  and the undo." Applying the file's own rule, this reads "the attention queue is part of
  convergence", which `:219` explicitly denies ("It belongs to convergence, not to attention").
- `:60` (Lane) — "a *lane* unqualified is the ref and **the queue row**." `:56` in the same entry
  was amended to "the landing-queue row"; four lines down it was not.
Two words. Same self-refuting shape as last round's I1, one file inward.

**I3. `CLAUDE.md:15` still uses a banned word, and it is the paragraph `CONTEXT.md` cites.**
*"The connective tissue is provenance: **the spec** written in Define is visibly referenced in
Design, carried into Develop, and attached to the release in Deploy."* The table at `:296` bans
**spec** for a brief. Worse than a stray word: `CONTEXT.md:125` cites this exact paragraph
(`CLAUDE.md` §Product spine) as one of the two things that fix **Provenance**, and translates its
"spec" into "the brief revision it was done against". The round fixed line 3 because the review
named line 3; the same defect twelve lines down was never searched for. *Fix:* "the brief written
in Define".
*Owner's, not mine:* the same sentence says "The four **phases** are states of one project", using
*phase* for what the table reserves for our build order (`:295`), as does `:5` and `:27`. That is
entangled with Open question 2 (Define vs Discover) and should be added to it, not fixed.

**I4. "Approval" now has the same two-sense problem "queue" just had — created by this round.**
The Hub entry (`:166`) lists the Hub's contents as "briefs, **approvals**, the provenance ledger,
releases and teams", bare. The new entry defines **Design approval**: a Hub record, id prefix
`ap-` (hub-contract `:106`), §7.7. But the other half of the same commit added **Authority**, whose
entire surface in the governing UX document is *approvals* in the other sense — "approval row"
(`AGENTIC-UX.md:190`, `:241`), "the bare approval card" (`:226`), "approve the pattern for all"
(`:189`) — a human's in-queue decision on an agent's requested action, which is not a Hub record at
all. Two objects, one word, both introduced in one commit, no disambiguation. *Fix:* one clause on
the Design approval entry ("an *approval row* in the attention queue is a decision about an
agent's action under Authority, not this record"), and "design approvals" in the Hub entry.

---

## 4. Minor — six

**M1. A citation was dragged across with the deleted `Agent` entry and no longer supports its
sentence.** `:252`: "Accountability and billing follow the owner, and no user steers another user's
agent (`docs/FUNCTIONAL-SPEC.md:14-15`)." `:14-15` are the **Session** and **Sub-agent** rows of the
entities table; neither mentions accountability, billing or steering. `OD-16`
(`owner-decisions.md:127`) says both verbatim — "Accountability and billing stay with each agent's
owner" and "**No user steers another user's agent**" — and is already the entry's *Fixed by*. Move
`:14-15` to the "shown as nested steps" clause, which `:15` does fix, or drop it.

**M2. `AGENTIC-UX.md:37` is labelled "rule 6"; §4's rule 6 is `:73`.** `:37` is §2's item 6
("Authority must be legislated in advance, not vetoed in real time") and does support the entry;
`:73` is "Unilateral decisions are a first-class class". Same two-list collision as I1, but here
only the label is wrong. (Inherited from my predecessor's wording, which said the same.)

**M3. Release: "If the pin fails, Ship does not happen" compresses a different rule.** §7.9 `:971`
says "If **step 1** fails, Ship does not happen" — step 1 being the reservation. A reservation that
takes **no** pin is a normal success: `:1015` gives `none` / `no_brief` and `none` /
`no_complete_revision` as outcomes of a `201`, and `:984` enumerates them in `pin.reason`. Read
literally, the entry says a project with no brief cannot Ship, which the cited section denies. One
word: "if the **reservation** fails". Second, smaller: "phase 7 then calls Panacea" — `:966` has the
**control plane** call Panacea using phase 7's client.

**M4. A dangling citation survived the `Warm pool` cut.** The Machine entry (`:29`) now cites
`docs/PROGRAM.md:119-122` under the claimed-from-a-warm-pool clause. `:119-122` is Phase 8's scope
paragraph — it names warm pools as something phase 8 *builds*, not as the product behaviour the
clause states. That behaviour is fixed by `FUNCTIONAL-SPEC.md:46`, already cited on the same line.
Harmless, but it is a citation that does not fix what sits above it.

**M5. The recorded reason for deferring M7 (`The Desk`) is false, and the deferral's own commit
contradicts it.** Report §6: "nothing in the phase 2 contracts to fix it … its only citation would
be the UX document". In fact `ui-contract.md:45` names the Desk, and `:64` (§0.2 *What Group D
deliberately does not build*) lists it — in the **same sentence** as "the ranked queue" and "the
authority grid", both of which this round added entries for. `owner-decisions.md:172` puts "the Desk
frame … in W1". Deferring on brevity grounds is defensible; the reason as written will stop the next
reader from looking. Correct the reason to "deferred until W1 builds the surface".

**M6. "Not «trust level», which names the calibration record" asserts a meaning no file fixes.**
`AGENTIC-UX.md:185` defines trust *calibration* from the user's own history per action class and
bans a confidence number without its calibration record; it never defines "trust level". Either cite
`:185` and reword, or drop the line — the file's own bar is that a rejected synonym is one someone
actually reaches for, with a reason from a file.

---

## 5. Question 3 — are the cuts right?

**`Agent`: yes, nothing was lost.** Its three parts all survive. "One running instance of a harness"
is in the Harness entry's *Not «agent»* line and in the table row at `:298`. *Sub-agent* is in
Orchestrator sense 1 with `FUNCTIONAL-SPEC.md:15`'s "nested steps". The `OD-16` ownership rule moved
to the entry where the cross-user rule already lived, which is where a reader looks for it. The one
casualty is the citation, at **M1**.

**`Warm pool`: yes, with one real loss and one that does not matter.** The product behaviour moved
intact into Machine. Lost: the *why* — Kata cold start is seconds, so a pool exists to keep it off a
person's path (`CLAUDE.md` §Architecture: "Warm pools are mandatory"). Not lost in any way that
hurts: *not «autoscaling», not «cache»*, which nobody in this tree has confused. The word is
standard infra vocabulary and did not belong under a file whose test is "not the obvious meaning".

---

## 6. Question 4 — anything a competent engineer would already assume?

**Nothing left to cut.** I re-read all 29 headwords against the file's own admission test. The
weakest two now are:
- **Sandbox** — "isolated execution environment" is assumed; it earns its place on the cardinality
  (*one per running session*, not per worktree) and on "the user never sees the word".
- **Project** — "one git repository" is assumed; it earns its place on *enumerated from disk, never
  created by us* and on "the same project on two machines is two records".
Both keep. Everything else is either a word we redefined (worktree, lane, lens, stream, seam,
convergence) or an object with a non-obvious mechanism (brief revision, contextRef, the
Materialiser, release, evidence).

One thing to watch rather than fix: **298 → 340 lines**, +14% for five new entries. The file's own
warranty is "short enough to read in full", and it still is (~10 minutes). The next round that adds
five more should be asked to remove something.

*Nit:* Criterion (`:114`) and Evidence (`:160-162`) now state the same digest-keying rule from both
ends. Defensible as cross-referencing; worth one editor's pass if a third entry ever repeats it.

---

## 7. Question 5 — was the lens conflict recorded, not decided?

**Yes, in four places, and nothing was ruled on.** Open question 1 (`:309-318`) states both counts,
names both files, and carries the reviewer's added fact verbatim — the conflict is *inside*
`ui-contract.md:564-565`, whose comment lists six rows while citing rule 12 — with the consequence
that whichever way the owner rules, that comment needs the same edit. The Lens entry flags it inline
at `:211`. The closing rule's new exception (`:336-340`) names Lens as the case it exists for. Report
§6 repeats it. I verified both sides again: `AGENTIC-UX.md:79` says "Four lenses"; `ui-contract.md:566`
ships six, `:564-565` cites rule 12 above it, `FUNCTIONAL-SPEC.md:51` lists six, and
`ui-contract.md:65-68` has the sub-rail render all six rows. Open question 2 (Define vs Discover) is
carried unchanged and is still unfixed anywhere in the tree — I3 above adds a second sentence to it.

---

## Summary

| | Count |
|---|---|
| Critical | 0 |
| Important | 4 — I1 Queue cites `AGENTIC-UX.md:31` instead of rule 4 at `:71`; I2 two bare "the queue"s at `:60`, `:64` against the file's own new rule; I3 `CLAUDE.md:15` "the spec written in Define"; I4 "approval" now has two senses and no disambiguation |
| Minor | 6 — M1 `FUNCTIONAL-SPEC.md:14-15` dragged onto a sentence it does not support; M2 "rule 6" for `:37`; M3 "if the pin fails"; M4 dangling `PROGRAM.md:119-122`; M5 the false reason for deferring The Desk; M6 "trust level" |

The round did the work it was set: all four Importants are fixed, the cuts were the right two, the
new definitions match their sources, the owner's judgement was left to the owner, and the
implementer caught an error in the review that reviewed it. What it did not do is apply its own
findings one step past where they were pointed — the bare-queue rule to the two entries it did not
touch, the citation discipline to the two new `AGENTIC-UX.md` anchors, and the banned-word sweep to
the rest of the file it was correcting. Those are line-exact and cheap.

**Task quality: Needs fixes**
