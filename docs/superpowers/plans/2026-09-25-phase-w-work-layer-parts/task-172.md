### Task 172: loops on the desktop: the editor, versions, the run log, *run now*

W5, stream WD, about 2.5 hours (an estimate, not a measurement). Depends on task 169 (the W5
desktop wire: `Loop`, `LoopVersion`, `LoopTrigger`, `LoopBound`, `LoopRun`, `ACTION_CLASSES`,
`AUTONOMY_LEVELS`, the loop members of `WorkIntakeClient` and the `loop.*` ops, `task-169.md`,
which follows tasks 162-163's scopes for the loop wire). It also uses task 49's `Editor`,
`MarkdownDoc` and `docFromAuthored` (`task-49.md:108-125`, `:522-536`), task 44's
`ConfirmInPlace`, task 20's `expectNoPrediction`, task 13's SW.5 kinds
(`task-13.md:112-133`) and task 170's `testWork.ts` harness.

Parity: closes, jointly with tasks 162-163, **L-160** (loops: event or schedule triggers,
`05-design.md:322`) and **L-161** (the loop's versions and run log). Design: 05 §2.2, §5.4 W5;
AX §7 (`docs/AGENTIC-UX.md:175-185`); 05 L-160, L-161. OD-16 (a person's agents are theirs; an
admin may pause, not edit), OD-27 (no prediction without its rule and span).

**What the reader does.** Dylan opens *Settings → Loops* and presses *New loop*. He names it
*Morning triage*, picks *every weekday at 08:30*, writes the instruction *Summarise what arrived
in Triage overnight and flag anything that names a release*, keeps the action *run a turn*, and
sets *commit* to ASK FIRST in the grid; *push* and *deploy* are greyed at his own level, and
FULL is disabled with *Above your own grid (ACT AND TELL ME)*. He saves: version 1. The next
morning the run log reads *08:30 · schedule · ran · ot-3fa2…*. Tuesday reads *skipped: budget
spent*. He presses *Run now*; the row reads *queued since 14:02*, then *ran*.

**Decisions made here, so nobody re-derives them:**

1. **A save is a new version.** *Save* sends one `loop.version` op (task 169) with the whole
   `LoopVersionBody` to task 162's `POST …/versions`; the old version stays listed. The editor
   never patches a version, and a loop's head changes only by *Pause*/*Resume* (`loop.state`).
2. **The grid is AX §7's, drawn as a grid.** Rows are `ACTION_CLASSES`, columns the four
   `AUTONOMY_LEVELS` in AX's words (ASK FIRST · PROPOSE AND WAIT · ACT AND TELL ME · FULL), one
   radio group per row, never a slider. A row the loop does not set shows the owner's own level
   **greyed in place** with *inherited*. A level above the owner's own level for that row is
   `disabled` with a visible reason, *Above your own grid (ACT AND TELL ME)*, so the Hub's 422
   `loop_bound_exceeds_owner` is a second check, not the only one.
3. **The consequence line names no prediction.** Under the grid the editor states the blast
   radius in words, derived from the chosen levels only: *Can commit in its own worktree after
   you approve. Cannot push or deploy.* It **omits** AX:181's *"expect about 4 interruptions per
   hour"*: that is a projection that names no rule and no span (OD-27; Q13's fourth amendment
   allows only projections that name them). `turnsPerDay` shows as *at most 6 turns a day*, a
   bound the owner set, which is a limit, not a forecast.
4. **Only your loops are editable.** The list shows the caller's loops (owner id equals the
   signed-in user) first, editable. A team admin also sees teammates' loops in the same project,
   read-only, with *Pause* as the only action (OD-16; task 162: an admin may pause, never edit or
   resume). A member who is not an admin sees only their own.
5. **The instruction is the person's text, capped and counted as a number.** The one `Editor`
   (task 49), `value` from `docFromAuthored(version.instruction)`, the count shown as *1,284 /
   2,000*: two numbers, the second a limit, which is a fact about the field, not progress. Over
   2,000 blocks *Save* in place. The text is `text·H` of the owner; it goes into the loop as data
   and never into a turn's managed block (task 163's `loop.show` returns it as data).
6. **Run now is one run, and waits on the heartbeat.** *Run now* mints one op id, sends
   `loop.runNow` through `keyed(op.id)` with `requestId = op.id` (task 169), and disables itself
   until the answer; a double press is one request. While the run is `queued`, its row reads
   *queued since 14:02* and the time since, on the one heartbeat (`hooks/heartbeat.ts`), with no
   spinner node. On a paused loop the button is absent and the row says *Paused: resume to run*.
7. **Every outcome is shown, in words.** The log shows every `LoopRun`, newest first, including
   `skipped_budget` and `skipped_paused`; no filter hides them by default. Words: *queued*,
   *ran*, *skipped: budget spent*, *skipped: paused*, *refused: bound*, *failed*. Ids (`ot-…`, a
   Start's session and branch from `started`) render in mono.
8. **Local mode has no loops.** Task 162 does not claim a loop runs in local mode (no
   Orchestrator there, OD-20). When the desktop runs local-only, *Settings → Loops* shows one
   line, *Loops run on your cloud Orchestrator. Sign in to the team to use them.*, and no editor.

**Files:**
- `desktop/src/components/loops/LoopList.tsx` (+ `LoopList.test.tsx`) — yours, and teammates' for an admin (new)
- `desktop/src/components/loops/LoopEditor.tsx` (+ `LoopEditor.test.tsx`) — name, trigger, instruction, action, grid (new)
- `desktop/src/components/loops/LoopVersions.tsx` (+ `LoopVersions.test.tsx`) — the list and a side-by-side diff (new)
- `desktop/src/components/loops/LoopRuns.tsx` (+ `LoopRuns.test.tsx`) — the run log and *Run now* (new)
- `desktop/src/components/loops/loopModel.ts` (+ `loopModel.test.ts`) — pure: grid bounds, words, diff (new)
- `desktop/src/components/loops/loops.css` — `.lp-*` (new)

The File map's `src/components/{plan,roadmap,intake,admin}/*` row has no home for loops; this
part adds `loops/`, as scope 172 does (gap 1).

**What is on the tree today (verified 2026-09-26).** Parts exist for tasks 13, 20, 44, 49 and
169; task 13's W1 kinds are on its part (`task-13.md:120-138`). There is **no part** for tasks 162
or 163, so every route, field and outcome word is their scopes' (`tasks-161-180.md:85-177`) and
**UNVERIFIED**; where Phase T stores the owner's own grid is **UNVERIFIED** in task 162's scope
too (`:109-110`), so how the editor reads it is gap 2. The one heartbeat is on the tree:
`desktop/src/hooks/heartbeat.ts` exports `useHeartbeat()` (`:64`) and `BEAT_MS = 2400` (`:13`).

#### Interfaces — complete: `desktop/src/components/loops/loopModel.ts`

```ts
// desktop/src/components/loops/loopModel.ts — task 172. Pure: no React, no network.
import { ACTION_CLASSES, AUTONOMY_LEVELS, type ActionClass, type AutonomyLevel, type LoopBound,
  type LoopOutcome, type LoopVersion } from "../../api/work";   // task 169

export const LEVEL_WORDS: Readonly<Record<AutonomyLevel, string>> = {
  ask_first: "ASK FIRST", propose_and_wait: "PROPOSE AND WAIT", act_and_tell: "ACT AND TELL ME", full: "FULL",
};
export const CLASS_WORDS: Readonly<Record<ActionClass, string>> = {
  read: "read", edit_own_worktree: "edit in own worktree", edit_outside_worktree: "edit outside worktree",
  run_tests: "run tests", install_dependency: "install dependency", run_migration: "run migration",
  touch_infra: "touch infra/config", commit: "commit", push: "push", deploy: "deploy",
};
const rank = (l: AutonomyLevel) => AUTONOMY_LEVELS.indexOf(l);

/** One grid cell as the editor draws it (decision 2). */
export interface Cell { level: AutonomyLevel; chosen: boolean; inherited: boolean; disabled: boolean; why: string | null }

/** owner: the owner's own grid, every class set (gap 2 says where it comes from). */
export function gridCells(bound: LoopBound, owner: Readonly<Record<ActionClass, AutonomyLevel>>): Record<ActionClass, Cell[]> {
  const out = {} as Record<ActionClass, Cell[]>;
  for (const c of ACTION_CLASSES) {
    const set = bound.levels[c];
    const cap = owner[c];
    out[c] = AUTONOMY_LEVELS.map((level) => {
      const above = rank(level) > rank(cap);
      return { level, chosen: set === level, inherited: set === undefined && level === cap,
        disabled: above, why: above ? `Above your own grid (${LEVEL_WORDS[cap]})` : null };
    });
  }
  return out;
}

/** The blast radius in words, from the levels alone (decision 3). Never a rate or a count. */
export function consequence(bound: LoopBound, owner: Readonly<Record<ActionClass, AutonomyLevel>>): string {
  const eff = (c: ActionClass) => bound.levels[c] ?? owner[c];
  const can = (c: ActionClass) => eff(c) === "act_and_tell" || eff(c) === "full";
  const asks = (c: ActionClass) => eff(c) === "ask_first" || eff(c) === "propose_and_wait";
  const parts: string[] = [];
  parts.push(can("edit_outside_worktree") ? "Can edit outside its worktree." : "Edits stay in its own worktree.");
  if (can("commit")) parts.push("Commits and tells you.");
  else if (asks("commit")) parts.push("Commits only after you approve.");
  const never = (["push", "deploy", "run_migration"] as const).filter((c) => eff(c) === "ask_first");
  if (never.length > 0) parts.push(`Asks before it would ${never.map((c) => CLASS_WORDS[c]).join(", ")}.`);
  parts.push(`At most ${bound.turnsPerDay} ${bound.turnsPerDay === 1 ? "turn" : "turns"} a day.`);
  return parts.join(" ");
}

export const OUTCOME_WORDS: Readonly<Record<LoopOutcome, string>> = {
  queued: "queued", ran: "ran", skipped_budget: "skipped: budget spent", skipped_paused: "skipped: paused",
  refused_bound: "refused: bound", failed: "failed",
};

/** A field-by-field text diff of two versions, for the side-by-side view. */
export function versionDiff(a: LoopVersion, b: LoopVersion): { field: string; before: string; after: string }[] {
  const f = (v: LoopVersion): Record<string, string> => ({
    trigger: JSON.stringify(v.trigger), instruction: v.instruction.text, action: v.action,
    label: v.labelId ?? "", turnsPerDay: String(v.bound.turnsPerDay), levels: JSON.stringify(v.bound.levels),
  });
  const x = f(a), y = f(b);
  return Object.keys(x).filter((k) => x[k] !== y[k]).map((k) => ({ field: k, before: x[k]!, after: y[k]! }));
}

/** Decision 5: over the cap blocks Save. */
export const INSTRUCTION_MAX = 2000;
```

`JSON.stringify` of `trigger` and `levels` is a text diff over a stable shape (task 169's field
order; `levels` is built from `ACTION_CLASSES` in order by the editor, so two equal grids
stringify equally). The diff view renders `trigger` through `triggerWords` below, never raw JSON.
