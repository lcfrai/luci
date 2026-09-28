### Task 96b: the Design lens's issue list; *implements* on the issue view

W2, desktop. A **delta on phase 5** under rule H2 (the owner phase's file wins), plus one new
component in this plan's own tree. Seams: **SW.14** (the thread primitives), **SW.16** (the
replica), and it consumes **SW.8**, **SW.11** and **SW.12** like every surface task. Depends on
task 83 (the desktop W2 wire types) and task 81b (the read this renders), and on **P5-27** and
**P5-28**.

Anchors read on 2026-09-28: `docs/superpowers/planning/2026-09-25-linear-parity/05-design.md`
(cited `05:N`); the phase 5 plan
`.claude/worktrees/phase-5/docs/superpowers/plans/2026-09-23-phase-5-design-canvas.md` (cited
`P5:N`), on **another worktree's branch** — every `src/design/` path below is **UNVERIFIED until
phase 5 is sync-merged here**, and phase 5's file wins any conflict.

**What this closes.** `05:1082`, the Design row of the lens table: *"a frame shows the issues whose
revision `implements` it"*, and the second half — *"a design approval can close a design issue
(04 §7.7)"* — whose server half is task 81b. Until this task lands, the thread the Goal states has
no visible hop at Design: an issue can name the canvas it implements and nobody can see it from the
canvas.

**The one thing this task must not get wrong** is that it adds a **group to phase 5's existing
side panel**, not a second design surface. `P5:420` builds `DesignLens.tsx`, `DesignSide.tsx`,
`ApprovalBar.tsx` and the rest in task 28; this task adds one group inside `DesignSide.tsx` and
nothing else to the chrome. A second panel, a second store or a second issue view all break 05's
*one issue view* rule (`05:1087`, rule 1) and rule 3's *three homes, never mixed*.

**Files:**

- `desktop/src/design/DesignSide.tsx` ⚑ (P5 task 28; `P5:420`; **UNVERIFIED until merge**): one
  *Issues* group listing the canvas revision's issues, each row a `<ThreadNode/>` (SW.14) that
  opens the one issue view at the revision the chip names
- `desktop/src/design/DesignLens.tsx` ⚑ (same source): the group's empty and error states join the
  lens's seven existing states rather than inventing an eighth
- `desktop/src/components/issue/Implements.tsx` (new) and `implements.test.tsx`: the design keys the
  revision implements, beside task 47's `Serves.tsx`, each with *Open design* — **and the writer**,
  because without one the field is dead. Round 1 shipped a read-only block while all twelve write
  sites in the plan (`task-07.md:414`, `task-08.md:144`, `task-09.md:112`, `task-20.md:250`,
  `:323`, `:326`, `task-38.md:230`, `task-40.md:260`, `task-57.md:301`, `task-68.md:211`, `:282`,
  `task-140.md:472`) send `implements: []`, so the Design hop rendered empty for every real user.
- `desktop/src/components/work/DesignPicker.tsx` (new) and its test: `implements`'s writer, built on
  task 42's `Picker` exactly as `CriterionPicker` is (`task-42.md:124-160`), listing the project's
  canvases and their revisions. It follows task 47's decision 3 unchanged: **adding or removing a
  key writes a new revision, never a patch**, so the digest and the thread stay honest. It is the
  only writer this plan has; if it is cut, say so in the return and the field stays `[]`.
- `desktop/src/components/issue/IssueView.tsx` ⚑ (task 46's file): the new block, under *Serves*
- `desktop/src/api/work.ts` ⚑ (task 37's file): `listIssuesForDesign(designKey)` over task 81b's
  read route, through task 16's relay
- `desktop/src/components/work/work.css` ⚑ (task 41's file): the group, on existing tokens only

**Rules it must not break.**

1. **One store.** Reads go through SW.16's `useWork(selector)`. No second store, no fetch in a
   component, and **no Hub data on disk** (rule 3; the grep test in task 55 covers it).
2. **Rows never move under the reader** (rule 4). Arrivals are held on `arrivals.hold`; a landed
   issue flashes in place and its row index does not change; the list never re-sorts on a state
   change while the reader is looking at it.
3. **No prediction** (`OD-27`, SW.8). The group header is a plain count. Never *"3 of 7 designed"*,
   never a bar, never a percentage, never an ETA. `expectNoPrediction` runs over the rendered lens.
4. **One heartbeat** (`DESIGN-SYSTEM.md`), never a spinner per row; the design key and the alias are
   Commit Mono at `--t-fact` (`OD-28`); `--ink-3` carries no fact.
5. **Clean room** (`OD-11`): our own glyphs (SW.12), no Linear asset, icon or copy, and nothing read
   from any AGPL canvas source.

**Tests first.** Each names the mutant it kills.

1. The group lists an issue whose current revision implements the open canvas revision and omits one
   implementing an earlier revision. *Mutant:* matching on canvas id, which shows stale issues.
2. A landed issue flashes in place and its row index is unchanged. *Mutant:* a re-sort on state.
3. The *implements* line opens the design at the key it names. *Mutant:* opens the canvas head,
   which silently drops the revision and breaks the thread at the hop this task exists to build.
4. `expectNoPrediction` over the rendered lens. *Mutant:* the count rendered as *"3 of 7"*.
5. A row opens the **same** issue view component the Work lens opens, at the named revision.
   *Mutant:* a second, lighter issue view inside the design panel.
6. With phase 5 absent from the tree, the new component still builds and its test runs against the
   fixtures in SW.8. *Mutant:* a hard import of `src/design/` that breaks the desktop build until
   phase 5 merges.

**Acceptance (the owner's eyes, not a screenshot).** Open a canvas with two approved revisions and
three issues; the *Issues* group shows the issues of the open revision, each row opening the one
issue view; the issue view shows *Implements* under *Serves* with *Open design*; nothing on either
surface predicts completion. Visual acceptance is the owner's, per `CLAUDE.md`.

**Does not claim.** The Discover half of the lens table — *each criterion lists the issues that
serve it* is task 67's and W4's. Editing the canvas, commenting on a frame, or presence: all phase
5's. A frame-level list: phase 5's key names a canvas revision, not a subset (P5-H), so a
frame-level list has nothing to key on. Multiplayer cursors (L-121, a Gaps row, not this task's).
