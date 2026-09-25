### Task 39: prototype, part 2: the issue view and the Desk; **the owner's review**, recorded

Stream **WD1**, no seam, about 3 hours (an estimate, not a measurement). Depends on task 38 (the
prototype directory, `world.ts`, `Rows.tsx`, `mount.ts`). It also places task 28's `StateGlyph`
(its `done_not_proven` key), task 29's `AgentGlyph`, task 31's `ViewHeader`, task 36's
`ThreadNode` and `useThreadLit`, and task 20's builders and `expectNoPrediction`; the skeleton's
row names 38 only (a gap, named in the return).

This is the `OD-1` gate for Phase W's desktop work: the owner looks at the work surfaces **in the
app, in the new look**, before tasks 40-45 are sized (05 §5.3 rows 10 and 12). It adds the last two
screens and the file that records what the owner said. It writes no verdict: until the owner has
looked, the file says *not yet reviewed*, and `feat/work-layer` is not merged onward (`OD-1`,
`OD-12`). Like task 38's screens, these read fixtures, talk to no Hub, and are deleted by the
task that builds the real surface (46-48 for the issue view, 62 for the Desk frame).

**Decisions made here, so nobody re-derives them:**

1. **One switch for four screens.** Task 38's default export switched Work and Plan with one ghost
   button in each header. It becomes a strip of four, *Work · Plan · Issue · Desk*
   (`aria-pressed` on the shown one), above the screen, labelled *Prototype screens*, so the owner
   moves between them without keys. The header buttons go (both props were optional for this).
2. **The issue view shows the world's live issue, EP-167, and EP-161.** 05 §2.1 draws EP-142 at
   `started` with an agent on it; task 38's decision 3 already stands EP-167 in for that row, so
   the Work row and the issue view agree. EP-161 is the world's `done` issue with
   `proof: "not_proven"` and a `stateReason` (`task-20.md:309`): it is the second fixture issue that
   reads ***done, not proven*** with its one-line reason. Two ghost buttons in the issue header
   switch between them.
3. **The overrides are in a new `issueWorld.ts`**, on top of `protoWorld()`, and declare no
   builder (task 20's rule): EP-167's revisions 2 and 3 (task 38's revision 1 stays first), its
   label, the files the lane was observed to change, and one *observed evidence* fact. Every value
   names the drawing line it stands in for.
4. **Nothing from W3.** No *proposed evidence* line, no *Record* button, no Orchestrator note: 05
   §2.1's drawing has all three, and W3 builds them (05 §5.3). The owner is told so in the review
   file, not on the screen.
5. **The serves line prints the fixture's key in full.** `brief:bf-00000000000000a2@2 ›
   c-91e2a4b0` in mono, then the criterion's words in sans, then the `ThreadNode`. The drawing's
   `bf-3a…@6 › done ›` abbreviates and names a section; task 20's `Serve` is `{ briefKey,
   criterionId }` (`task-20.md:175`) and has no section, and how the product shortens ids is task
   47's. The line binds `useThreadLit` fully, as task 36 decision 2 says the `serves` line does.
6. **The Desk's five sections are drawn in one fixed order, always,** NEEDS YOU → DECIDED WITHOUT
   YOU → FAILED → LANDED → RUNNING (05 §4.2, AX §5). An empty section keeps its place and says
   *None* in one line, so the order is visible even on a quiet day; DECIDED WITHOUT YOU is empty in
   W1, since only an Orchestrator agreement fills it (W3). Whether the product hides empty sections
   is task 62's.
7. **RUNNING folds.** One agent changed since the reader last looked (EP-150's agent 2, idle with a
   question, quiet 12m) gets the three-line card; the steady one folds into *1 steady, all live*,
   which expands in place. The number is a group size (Q1 A). No liveness word changes while the
   Desk is shown: the prototype has no clock at all, so this is true by construction, and task 62
   builds the rule.
8. **The zero state is a prototype control**, *Show the zero state*, which swaps the five sections
   for the one fact line of the demo's last frame: *Nothing needs you. EP-142 landed and proven at
   14:02.* No illustration, no projection. 05 §4.2's longer line adds `3 agents live on 2 issues`;
   the scope and the skeleton (`:29`) end at *14:02*, and this task follows them.
9. **The review file is a template of facts.** The implementer fills what a machine or a reading
   can establish (date, commit, scaling, themes, screens shown) before the session, and the rest
   only from the session itself: each check's result is the owner's call, written as the owner
   said it. The owner's words go in `>` quotes, dated and attributed. A file with no `>` line is
   the honest *not yet reviewed* state, not a failure and not a pass.

