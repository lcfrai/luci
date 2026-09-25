### Task 38: prototype, part 1: the Work lens and Plan in the app, behind `flag.compare`, on fixtures

Stream **WD1**, no seam, about 3 hours (an estimate, not a measurement). Depends on task 32
(`lib/flags.ts`, the dev toolbar), task 36 (SW.14, the thread) and task 37 (SW.15, the types and
`workSlice`). It also places task 28's `StateGlyph`, task 29's `AgentGlyph` and `MarkGlyph`, task
31's `ViewHeader`, task 20's `workScenario()` and `expectNoPrediction`, and registers with task 33's
`registerStressScreen`; the skeleton's row names 32, 36 and 37 only (a gap, named in the return).

`OD-1` keeps Phase W's review gate: the owner sees the work surfaces **in the app, in the new look**,
before tasks 40-45 are sized (05 §5.3 row 10; row 11 is "resized after row 10's review"). This is
a prototype, not the product. It reads task 20's fixtures, talks to no Hub and no replica, and lives
in its own dev-only directory, `desktop/src/prototype/work/`, which the skeleton's file map does not
list. Task 40 promotes what the owner kept into `components/work/` and deletes the rest.

**Decisions made here, so nobody re-derives them:**

1. **Three gates, all needed.** The prototype mounts when `lens === "work"`, `import.meta.env.DEV`
   holds, and the new flag `prototype` is `"on"`. The DEV gate is a `lazy()` behind a literal, as
   task 32's `components/devtools/mount.ts` does it, so a production build emits no chunk; the
   flag keeps the prototype out of a developer's way until they ask for it.
2. **How the owner gets there.** `G W` is task 54's. A dev-toolbar button, *Open the Work
   prototype*, sets the flag and calls task 37's `openWork(project, "feat/resumable-uploads")`.
   Inside the prototype one ghost button switches Work and Plan (the prototype's stand-in for
   `G W` and `G P`), so the review needs no keys.
3. **The world is task 20's, with overrides only.** Task 20's rule holds: no builder is declared
   here. `world.ts` takes `workScenario()` and adds, as override objects, what 05 §4.1's drawing
   needs and the scenario does not carry: which criterion each head serves (task 20 gives
   revisions for EP-142 and EP-164 only), EP-167 moved to `started` so the reader has a live
   agent, and one agent line. The drawing's issue numbers are 05's; the prototype uses the world's:

   | 05 §4.1's row | In the prototype | Why this one |
   |---|---|---|
   | EP-142, the reader's issue with `dylan · agent 1` | **EP-167**, `todo` → `started` (override) | the world's EP-142 is already `done` |
   | EP-150, a teammate's, *touching upload/\*.go* | **EP-163**, Alice's, `started` | the world's one teammate issue |
   | EP-139, held, `→ done · own evidence at 7c1f0a2 …` | **EP-162**, `proven_self_attested` | "own evidence" is exactly self-attested |
   | EP-131, settled | **EP-142**, `done`, proven | the world's proven issue |
   | EP-155, *not tied · chore* | **EP-165** (and EP-160, *not tied · imported*) | both not-tied heads |
   | EP-128, the departure ghost | **EP-168**, `cancelled` | the world's cancelled head |

4. **What is shown and what is not.** Cancelled and duplicate heads are not rows (EP-169 is
   absent; EP-168 is its ghost only). The convergence word per group is a fixture value here;
   task 45 computes it (`task-45.md:81`, `Convergence`). Group sizes are plain counts (Q1 A).
5. **Lit and selected, together.** At mount, three rows of the first group are selected through
   `workSlice.select` and the keyboard cursor sits on a row of the second group; focusing that row
   lights its criterion (task 36's `bind`, focus half only, on the rows; the full bind on the
   header). The three selected rows ring and carry a leading `MarkGlyph kind="selected"`; the lit
   group brightens only its gutter.
6. **`moves.hold`, faked honestly.** *Replay a land* moves EP-150 from `in_review` to `landed` in
   the prototype's own state. The list order is frozen at mount, so the row stays, takes its new
   glyph, sets `data-unseen="true"` (task 34's flash or tick) and shows one mono cause line; the
   group header gains `· 1 moving`, and *Apply held changes* re-sorts. Task 41 builds the real
   rule; this only lets the owner see it.
7. **Plan** (05 §4.1, `G P`): Triage, Backlog, every criterion with its issues, and *My issues*.
   No cycles, features board or saved views (W2). Rows are the same row component as Work.
8. **Old and new look, graphite and paper** come from task 32's toolbar for free: the prototype
   writes only tokens and task 30's sans/mono rule, so `data-look` and `data-theme` restyle it.
   Both screens register with task 33's `stress.sets` by name.

