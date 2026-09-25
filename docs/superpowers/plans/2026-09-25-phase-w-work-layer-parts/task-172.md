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

**Two more decisions, made while writing the components:**

9. **A loop never waits on a loop.** The *when* picker offers `LOOP_EVENT_KINDS` only: W1 kinds
   copied by name from task 13's `hubapi/workevents.go`, and W5 kinds from task 169's
   `W5_FEED_KINDS`. It never offers `loop.*` or `work.loop_fired`, so one loop's run cannot fire
   another's (a chain the owner never wrote). Task 162 checks the list on the Hub as well.
10. **An unreadable owner grid fails closed.** Until gap 2 names where Phase T keeps the owner's
    own grid, the editor caps every class at ASK FIRST except `read` (FULL), shows the line
    *Your own grid is not readable yet, so this loop can only ask first*, and never guesses a
    wider cap. A narrower cap than the real one is safe; a wider one is not.

#### Interfaces — complete: `loopModel.ts`, continued: trigger words, events, rights

```ts
// appended to desktop/src/components/loops/loopModel.ts
import type { Loop, LoopTrigger } from "../../api/work";

/** Decision 9. W1: task 13's hubapi/workevents.go. W5: task 169's W5_FEED_KINDS. Test-held. */
export const LOOP_EVENT_KINDS = [
  "issue.created", "issue.state", "issue.attached", "comment.created", "comment.mentioned",
  "request.created", "request.accepted", "request.linked", "sync.run", "external_link.created",
] as const;
export type LoopEventKind = (typeof LOOP_EVENT_KINDS)[number];
export const EVENT_WORDS: Readonly<Record<LoopEventKind, string>> = {
  "issue.created": "an issue is created", "issue.state": "an issue moves",
  "issue.attached": "evidence is attached", "comment.created": "a comment is posted",
  "comment.mentioned": "someone is mentioned", "request.created": "a request arrives",
  "request.accepted": "a request is accepted", "request.linked": "a request is linked",
  "sync.run": "a tracker sync runs", "external_link.created": "an item is imported",
};
const EVERY_WORDS = { day: "every day", weekday: "every weekday", week: "every week" } as const;

/** "every weekday at 08:30 (Australia/Sydney)", or "when an issue moves, triage → todo, label ux".
 *  A kind this build does not know renders as itself, never dropped. */
export function triggerWords(t: LoopTrigger, labelName: (id: string) => string): string {
  if (t.kind === "schedule" && t.schedule)
    return `${EVERY_WORDS[t.schedule.every]} at ${t.schedule.at} (${t.schedule.tz})`;
  const what = t.eventKinds.map((k) => EVENT_WORDS[k as LoopEventKind] ?? k).join(" or ");
  const f = t.filter;
  const move = f.fromState || f.toState ? `, ${f.fromState ?? "any"} → ${f.toState ?? "any"}` : "";
  const label = f.labelId ? `, label ${labelName(f.labelId)}` : "";
  return `when ${what}${move}${label}`;
}

/** Decision 10: the cap used while the owner's own grid cannot be read. */
export const FALLBACK_OWNER_GRID: Readonly<Record<ActionClass, AutonomyLevel>> = Object.fromEntries(
  ACTION_CLASSES.map((c) => [c, c === "read" ? "full" : "ask_first"])) as Record<ActionClass, AutonomyLevel>;

/** Decision 4, from ids and the team role only (task 162's rule, OD-16). */
export interface LoopRights { edit: boolean; pause: boolean; resume: boolean; runNow: boolean }
export function loopRights(loop: Loop, me: string, teamAdmin: boolean): LoopRights {
  const own = loop.owner.id === me;
  const on = loop.state === "on";
  return { edit: own, pause: on && (own || teamAdmin), resume: own && !on, runNow: own && on };
}

/** Builds a bound's levels in ACTION_CLASSES order, so equal grids stringify equally. */
export function orderedLevels(levels: LoopBound["levels"]): LoopBound["levels"] {
  const out: LoopBound["levels"] = {};
  for (const c of ACTION_CLASSES) if (levels[c] !== undefined) out[c] = levels[c];
  return out;
}
```

`loopRights` is the only place the list and the editor ask *may I*; the Hub's 403 (task 162) is
the second check. A team admin who owns a loop is simply its owner: `own` wins.

#### Interfaces — complete: `LoopList.tsx` and `LoopEditor.tsx`, the exported halves

```tsx
// desktop/src/components/loops/LoopList.tsx — task 172 (decisions 4, 8).
import type { JSX } from "react";
import "./loops.css";

export function LoopList(props: {
  projectId: string;
  me: string;                 // the signed-in user's id
  teamAdmin: boolean;         // the caller's role on the project's team (hub-contract §5.2)
  hub: "cloud" | "local";     // gap 3: the host says which; "local" draws decision 8's one line
  now(): number;
}): JSX.Element;
// Reads replica.intake().loops filtered by projectId (task 169 loads projectLoops). Two groups:
// "Your loops" (owner.id === me), each row opening LoopEditor, and, for a team admin only,
// "Teammates' loops" read-only, each row: name, owner's displayName, triggerWords, state, and a
// Pause button when loopRights().pause. A member who is not an admin never sees the second
// group. A paused row reads "Paused" and its pausedCause in words ("owner left the team" for
// owner_left; any other cause as itself). "New loop" opens LoopEditor with no loop.

// desktop/src/components/loops/LoopEditor.tsx — task 172 (decisions 1-3, 5, 9, 10).
import type { ActionClass, AutonomyLevel, Loop, LoopVersion } from "../../api/work";

export function LoopEditor(props: {
  projectId: string;
  loop: Loop | null;                  // null: a new loop
  current: LoopVersion | null;        // the version read on open (client.loop), null when new
  ownerGrid: Readonly<Record<ActionClass, AutonomyLevel>> | null;   // null: gap 2, decision 10
  labels: readonly { id: string; name: string }[];                  // the team's labels
  tz: string;                         // the owner's IANA zone, from the OS (Intl)
  onSaved(v: LoopVersion): void;
  onClose(): void;
}): JSX.Element;
// Fields, in order: Name (1-80, counted "12 / 80"); Trigger, a two-choice radio group
// "When" | "Every"; for When, checkboxes over LOOP_EVENT_KINDS in EVENT_WORDS, then optional
// label, from-state and to-state selects; for Every, day | weekday | week and a time input
// (HH:MM) with the zone shown beside it in mono, not editable; Instruction, task 49's Editor
// with the count "1,284 / 2,000"; Action, "Run a turn" | "Start on the accepted issue" (the
// second needs a label); Turns a day (1-24, a number input); the grid; the consequence line.
// Save is disabled, with the reason beside it, while any field fails; a refusal from the Hub
// (loop_bound_exceeds_owner, loop_outside_grid) shows under the grid with details.class named.
```

**The ops Save sends** (task 169's `IntakeOp`, through SW.16 `mutate`):

```ts
const body: LoopVersionBody = {
  trigger, instruction: { text: doc.markdown, author: fieldAuthor(doc.authorship) },
  action, labelId: action === "start" ? labelId : null,   // the Start label only (task 163)
  bound: { turnsPerDay, levels: orderedLevels(levels) },
};
if (loop === null) mutate({ kind: "loop.create", projectId, body: { name, first: body } });
else mutate({ kind: "loop.version", loopId: loop.id, body, prev: toBody(current!) });
// toBody drops loopId, version, savedAt and savedBy: the LoopVersionBody of the version read.
```

A rename of an existing loop is not a version field (task 162 keeps `name` on the head and names
no rename route), so the Name field is read-only once a loop exists (gap 4). Save with nothing
changed sends nothing: `versionDiff(current, candidate)` is empty and the button reads *No
changes*, so the history never gains a copy the owner did not mean. The grid is ten rows of four
`role="radio"` inputs in one `role="radiogroup"` per row, labelled by `CLASS_WORDS`; an inherited
row's owner-level radio carries `data-inherited` and the word *inherited*; a disabled radio's
`title` and the visible text under the row both hold `why`. Arrow keys move within a row only.

#### Interfaces — complete: `LoopVersions.tsx` and `LoopRuns.tsx`, the exported halves

```tsx
// desktop/src/components/loops/LoopVersions.tsx — task 172 (decision 1).
export function LoopVersions(props: {
  loopId: string;
  labelName(id: string): string;
}): JSX.Element;
// On open: client.loopVersions(loopId), held in component state (task 169: versions are not
// replica records), newest first. Each row: "v3" (mono), savedBy.displayName, savedAt (mono,
// local time), and a checkbox. With two checked, a side-by-side table of versionDiff(older,
// newer): one row per changed field, "Before" and "After" columns. trigger renders through
// triggerWords, levels as "commit: ASK FIRST" lines in ACTION_CLASSES order, instruction as
// plain text (white-space: pre-wrap), never as rendered Markdown. A third check unchecks the
// oldest of the two. No row offers Restore: restoring is saving the old body as n+1 from the
// editor ("Start from v2" loads it into LoopEditor; the owner still presses Save).

// desktop/src/components/loops/LoopRuns.tsx — task 172 (decisions 6, 7).
export function LoopRuns(props: {
  loop: Loop;
  rights: LoopRights;
  now(): number;              // task 20's FIXED_NOW in tests; Date.now in the app
}): JSX.Element;
// Rows: replica.intake().loopRuns.get(loop.id) (task 169 keeps the last 50), newest first,
// then "Show older" pages client.loopRuns(loopId, {after}). Columns: firedAt (mono), the
// trigger ("schedule" | "event" | "run now"), OUTCOME_WORDS[outcome], turnId (mono) and, for
// each started lane, issueKey · branch (mono). No filter control exists, so nothing hides a
// skipped run (decision 7).
```

```tsx
// LoopRuns.tsx, the Run now button and the queued row (decision 6)
const [sending, setSending] = useState(false);
const runNow = async () => {
  if (sending) return;                     // a second press while the first is out does nothing
  setSending(true);
  try { await mutate({ kind: "loop.runNow", loopId: loop.id }); }   // op id = requestId (task 169)
  finally { setSending(false); }
};
// Only the queued row subscribes: a <QueuedSince> child calls useHeartbeat (hooks/heartbeat.ts:64),
// so a log of 50 finished runs re-renders nothing once a second (the hook's own warning, :60-63).
function QueuedSince(p: { firedAt: string }): JSX.Element {
  const nowMs = useHeartbeat();
  return <span className="lp-fact">queued since {hhmm(p.firedAt)} · {formatElapsed(p.firedAt, nowMs)}</span>;
}
// {rights.runNow && <button type="button" disabled={sending} onClick={runNow}>Run now</button>}
// {loop.state === "paused" && <p className="lp-fact">Paused: resume to run</p>}
```

`formatElapsed` is `desktop/src/lib/time.ts:27` (*41m*, *1h 06m*), fed the heartbeat's own
epoch-ms as that file asks (`:7-8`): a measured time since a recorded instant, never a remaining
time. The row holds no animation and no `aria-busy`. `hhmm` formats in the owner's local zone
with `tabular-nums`. The `now()` prop is for the non-live columns only (tests pass `FIXED_NOW`).

#### Tests to write first — `loopModel.test.ts`

```ts
import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import { W5_FEED_KINDS } from "../../work/replica/kinds";
import { ACTION_CLASSES, type ActionClass, type AutonomyLevel, type LoopVersion } from "../../api/work";
import { ALICE, DYLAN } from "../../test/workFixtures";
import { consequence, FALLBACK_OWNER_GRID, gridCells, LOOP_EVENT_KINDS, loopRights, versionDiff } from "./loopModel";

const grid = (over: Partial<Record<ActionClass, AutonomyLevel>> = {}) =>
  ({ ...Object.fromEntries(ACTION_CLASSES.map((c) => [c, "act_and_tell"])), push: "ask_first",
     deploy: "ask_first", ...over }) as Record<ActionClass, AutonomyLevel>;

describe("gridCells", () => {
  // Kills: the Hub's 422 loop_bound_exceeds_owner as the only guard.
  it("disables every level above the owner's own and names why", () => {
    const push = gridCells({ turnsPerDay: 6, levels: {} }, grid()).push;
    expect(push.filter((c) => c.disabled).map((c) => c.level)).toEqual(["propose_and_wait", "act_and_tell", "full"]);
    expect(push[3]!.why).toBe("Above your own grid (ASK FIRST)");
  });
  // Kills: an inherited row drawn as unset, or as chosen.
  it("an unset class shows the owner's level as inherited, not chosen", () => {
    const commit = gridCells({ turnsPerDay: 6, levels: {} }, grid()).commit;
    expect(commit.find((c) => c.inherited)!.level).toBe("act_and_tell");
    expect(commit.some((c) => c.chosen)).toBe(false);
  });
  // Kills: a wider guessed cap when the grid cannot be read (decision 10).
  it("the fallback grid lets nothing but read above ASK FIRST", () => {
    const cells = gridCells({ turnsPerDay: 1, levels: {} }, FALLBACK_OWNER_GRID);
    for (const c of ACTION_CLASSES.filter((k) => k !== "read"))
      expect(cells[c].filter((x) => !x.disabled).map((x) => x.level)).toEqual(["ask_first"]);
  });
});

describe("consequence", () => {
  // Kills: AX:181's "expect about 4 interruptions per hour" kept.
  it("states limits and levels, never a rate", () => {
    const s = consequence({ turnsPerDay: 6, levels: { commit: "ask_first", edit_outside_worktree: "ask_first" } }, grid());
    expect(s).toBe("Edits stay in its own worktree. Commits only after you approve. " +
      "Asks before it would push, deploy. At most 6 turns a day.");
    expect(s).not.toMatch(/expect|about|per hour|interruption|likely/i);
  });
});

describe("versionDiff", () => {
  const v1 = { loopId: "lp-1", version: 1, action: "turn", labelId: null, savedAt: "2026-09-25T22:30:00Z",
    savedBy: DYLAN, instruction: { text: "Summarise Triage", author: "H" },
    trigger: { kind: "schedule", eventKinds: [], filter: { labelId: null, fromState: null, toState: null },
      schedule: { every: "weekday", at: "08:30", tz: "Australia/Sydney" } },
    bound: { turnsPerDay: 6, levels: { commit: "ask_first" } } } satisfies LoopVersion;
  // Kills: a diff that reports the stamp fields (version, savedAt) as changes.
  it("names only the fields the owner changed", () => {
    const v2 = { ...v1, version: 2, savedAt: "2026-09-26T01:00:00Z", bound: { ...v1.bound, turnsPerDay: 4 } };
    expect(versionDiff(v1, v2)).toEqual([{ field: "turnsPerDay", before: "6", after: "4" }]);
  });
});

describe("LOOP_EVENT_KINDS", () => {
  // Kills: a kind retyped from memory, and a loop that waits on a loop (decision 9).
  it("each kind is a W1 constant in workevents.go or a W5 feed kind, and none is a loop's", () => {
    const go = readFileSync(new URL("../../../../control-plane/internal/hub/hubapi/workevents.go", import.meta.url), "utf8");
    const w1 = new Set([...go.matchAll(/Event\w+\s*=\s*"([a-z_.]+)"/g)].map((m) => m[1]));
    for (const k of LOOP_EVENT_KINDS)
      expect(w1.has(k) || (W5_FEED_KINDS as readonly string[]).includes(k)).toBe(true);
    expect(LOOP_EVENT_KINDS.some((k) => k.startsWith("loop.") || k === "work.loop_fired")).toBe(false);
  });
});

describe("loopRights", () => {
  // Kills: an admin who edits, resumes or runs another's loop (OD-16, task 162).
  it("a teammate's loop gives a team admin Pause and nothing else", () => {
    const loop = { id: "lp-1", teamId: "tm-pay", projectId: "pj-ep", owner: DYLAN, name: "Morning triage",
      state: "on", pausedCause: null, currentVersion: 1 } as const;
    expect(loopRights(loop, ALICE.id, true)).toEqual({ edit: false, pause: true, resume: false, runNow: false });
    expect(loopRights(loop, ALICE.id, false)).toEqual({ edit: false, pause: false, resume: false, runNow: false });
  });
});
```

#### Tests to write first — the components (`LoopEditor`, `LoopList`, `LoopVersions`, `LoopRuns`)

Each uses task 170's `setupWork` (`desktop/src/components/intake/testWork.ts`) with a replica
seeded with one loop `lp-1` owned by `DYLAN`, version 1 as in `loopModel.test.ts`, and
`ownerGrid = grid()`.

1. **A save is a new version** — open `lp-1` in `LoopEditor`, change *Turns a day* to 4, press
   *Save*: `mutate` is called once with `kind: "loop.version"`, `body.bound.turnsPerDay === 4` and
   `prev.bound.turnsPerDay === 6`; the fake answers version 2; `LoopVersions` then lists *v2* and
   *v1*, and v1's row still reads 6 in its diff. **Kills** "the version overwritten".
2. **Above the grid cannot be chosen** — the *push* row's FULL radio is `disabled`, the text
   *Above your own grid (ASK FIRST)* is visible under the row, and a click on it changes nothing
   and sends nothing. **Kills** "the Hub's 422 as the only guard".
3. **No interruption estimate** — render the editor with every class set; the consequence line is
   present and `expectNoPrediction(container)` passes; the text holds no match for
   `/interruption|per hour|expect about/i`. **Kills** "AX:181's line kept".
4. **A teammate's loop offers Pause only** — `LoopList` with `me = ALICE.id`, `teamAdmin = true`:
   `lp-1`'s row has one button, *Pause*; no *Edit*, *Resume* or *Run now*; clicking the row opens
   no editor. Pause sends one `loop.state` with `state: "paused"`, `prev: "on"`. With
   `teamAdmin = false` the *Teammates' loops* group is absent. **Kills** "edit another's loop".
5. **A double press is one run** — `LoopRuns` for `lp-1` as `DYLAN`: two clicks on *Run now*
   within one tick; `mutate` is called once with `kind: "loop.runNow"`, and the fake `send` saw
   one `POST …/runs` whose body `requestId` equals its `Idempotency-Key`. **Kills** "two runs".
6. **Skipped runs are facts** — seed runs 1-3 with outcomes `ran`, `skipped_budget` and
   `skipped_paused`: three rows, the second reading *skipped: budget spent*, the third
   *skipped: paused*; no filter control exists. **Kills** "skipped runs filtered out".
7. **No spinner while queued** — seed run 4 `queued` fired 12 minutes before the heartbeat's
   tick: its row reads *queued since* the local time · *12m*; no node in the log has
   `aria-busy="true"`, `role="progressbar"`, or a class matching `/spin|loading/`. **Kills** "a
   per-row spinner, against the one heartbeat".
8. **Local mode has no loops** — `LoopList` with `hub: "local"` renders the one line of decision 8
   and no *New loop* button. **Kills** "a loop editor that cannot run".
9. **The cap blocks Save** — type 2,001 characters into the instruction: the count reads
   *2,001 / 2,000*, *Save* is disabled with *Over 2,000 characters* beside it, and nothing is
   sent. **Kills** "the Hub's 422 as the only check".
10. **An unchanged save sends nothing** — open `lp-1` and press *Save* with no edit: the button
    reads *No changes* and is disabled; `mutate` is not called. **Kills** "a duplicate version
    for every open".

`vi.useFakeTimers()` with `__resetHeartbeat()` in `afterEach` (heartbeat.ts's test seam) sets
the tick for test 7; `FIXED_NOW` is task 20's.

#### Steps

1. Write `loopModel.test.ts` and the ten component tests; run them; they fail (no modules).
2. Write `loopModel.ts`; its tests pass.
3. Write `LoopEditor.tsx` with the grid and the consequence line; tests 2, 3, 9, 10 pass.
4. Write `LoopList.tsx`, `LoopVersions.tsx` and `LoopRuns.tsx`; tests 1, 4-8 pass.
5. Write `loops.css`: `.lp-grid` (a 5-column CSS grid: the class word, then four levels),
   `.lp-fact` (mono, `tabular-nums`), `.lp-inherited` (the `--ink-3` token, no opacity trick on
   text), a 1px ring plus layered shadow for the editor card (DESIGN-SYSTEM elevation); no
   `transition: all`, and a new run row enters with `opacity` only.
6. Mount `LoopList` in the person's settings (gap 1); `npm --prefix desktop run typecheck`;
   commit with `git commit -F <msgfile> -- <paths>`.

#### Acceptance

```bash
npm --prefix desktop test -- src/components/loops/
# expect: loopModel.test.ts 7 passed; LoopEditor.test.tsx, LoopList.test.tsx, LoopVersions.test.tsx
#         and LoopRuns.test.tsx together 10 passed; no failed
npm --prefix desktop run typecheck
# expect: exit 0
grep -rn -i -E "spinner|aria-busy|interruption|per hour" desktop/src/components/loops --exclude=*.test.*
# expect: no output, exit 1
```

Visual acceptance is the owner's eyes: the editor with a greyed inherited row and a disabled
level, a version diff, and a run log holding each of the six outcome words, opened in the
running app against the fake client.

#### What this task does NOT claim

- That a loop has run for a real Orchestrator: the fake answers `queued` and then the outcome a
  test seeds; task 162-163's janitor and Fleet delta are **UNVERIFIED until Phase T merges**.
- That the grid the editor caps against is the owner's real grid (gap 2); decision 10's fallback
  is narrower than any real grid, never wider.
- That *skipped: budget spent* reflects TM-37's real budget: the desktop shows the outcome the
  Hub recorded.
- Any count of how often a loop will fire or interrupt: the only numbers shown are the limit the
  owner set (*at most 6 turns a day*) and measured times.

#### Gaps found while writing this part

1. **No home for loops, and no person-settings host.** The File map's
   `src/components/{plan,roadmap,intake,admin}/*` row has no `loops/`; this part adds it. No part
   on the tree names where *Settings* is drawn (task 176 also says only "settings"); step 6 mounts
   `LoopList` wherever the File map settles it.
2. **Where the owner's own grid lives.** Task 162's scope (`tasks-161-180.md:109-110`) marks it
   **UNVERIFIED**, and a grep of the team worktree's plans for an autonomy-grid route finds none.
   The editor needs a read, for example `GET /v1/me/autonomy` answering
   `Record<ActionClass, AutonomyLevel>`, owned by Phase T; until then `ownerGrid` is `null` and
   decision 10 applies.
3. **No cloud-or-local signal is named.** No module on the tree or part exports whether the
   desktop is signed in to a team Hub; `LoopList` takes `hub: "cloud" | "local"` from its host.
4. **No rename.** Task 162's `PATCH /v1/work/loops/{loopId}` carries `{state}` only, and `name` is
   on the head, so a loop cannot be renamed; the field is read-only after create. A rename is
   one field on that PATCH (owner only) and one row in task 169's relay.
5. **The event list is this part's.** Task 162 checks *SW.5 and SW.21 feed kinds only* but names
   no list and does not refuse `loop.*` or `work.loop_fired`; decision 9 asks it to, so a loop
   cannot chain another on the Hub either.
6. **Two codes for one refusal.** `loop_bound_exceeds_owner` (task 162) and `loop_outside_grid`
   (task 143) are both drawn under the grid the same way; task 169's G1 decides which survives.
