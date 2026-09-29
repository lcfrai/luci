### Task 42: Triage: the view, number keys, `T` with a text-match criterion suggestion

Stream **WD1**, about 3 hours (an estimate, not a measurement). Depends on task 40 (the Work lens,
`IssueRow`, `work.css`) and task 41 (the hold model and `listKeys.ts`). **Also depends on task 46**,
which the skeleton's row does not name (a gap, named in the return): the duplicate key renders
task 46's `DuplicateOfPicker`, the priority refusal opens its `PriorityPicker`, the moves use its
`handMove`, and `CriterionPicker` is built on its `Picker` (`task-46.md:158-175`, `:247`,
`:295-305`, `:328-331`). Uses SW.1 (`serves [{briefKey, criterionId}]`), SW.8, SW.13
(`ViewHeader`'s `actions`), SW.14 (`ThreadNode`), SW.15 and SW.16 (`useWork`, `mutate`,
`onRefused`), and task 11's team policy and snooze.

Parity: closes **L-46** (the Triage view; its Desk rows are task 63), **L-47** (the number keys;
`Shift+R` accept-and-Start is task 61) and **L-48**'s surface (the refusal shown; the check is
task 11's `TriageExitRefused`, `task-11.md:187-199`), and **L-10** with task 48. Design: 05 §4.3
(`T` and `1`-`4`, the key table's rows at `05-design.md:1006` and `:1008`), §3.3 (`triage`), §2.3 point 4 (the
thread state, `05-design.md:613-618`), §5.3's demo (`05-design.md:1175`).

**What the reader does** (the demo, 05 §5.3): the imported backlog is in Triage, every row reading
*not tied · imported*. The reader moves to EP-142 and presses `T`. A picker opens beside the row
with *Uploads survive a dropped connection* first, its why reading `matches: dropped, connection`.
`Enter` attaches it: a new revision, and the row's thread state becomes the criterion's node.
`1` accepts it. Had the team required a priority, the row would flash `--fail`, say why, and open
the priority picker in place.

**Decisions made here, so nobody re-derives them:**

1. **Suggestion is shared words, shown as words.** `suggestCriteria` normalises both texts the same
   way (lower case; NFD with combining marks removed; split on anything that is not a letter or a
   digit; a fixed English stop-word list removed; tokens of one character removed), scores a
   criterion by the count of **distinct** shared terms, drops criteria with none, and breaks ties
   in brief order. There is no stemming (`upload` and `uploads` do not match) and no weighting.
   The picker shows `matches: dropped, connection`, never a number, a percentage or a bar (AX
   Principle 7; anti-pattern 12). A score is an internal sort key and is not exported.
2. **`1` accepts into the team's default state, and never into `triage`.** `TeamPolicy.defaultState`
   is the state a created issue takes (`task-11.md:370`); the Hub's own default is `backlog`
   (`task-11.md:184`). If a policy names `triage` (task 20's fixture does, `task-20.md:273`),
   accepting there would be a no-op, so `1` moves to `backlog`. The move is `handMove(to)`
   (`cause: "hand"`), written as the person.
3. **`2` is one act.** It opens task 46's `DuplicateOfPicker`, which already sends the relation and
   the `duplicate` move as **one** `mutate([...])` batch, so one Undo (`task-46.md:295-305`). Task 42
   adds no second duplicate path.
4. **`3` declines with an optional one-line reason.** The row opens an inline field in place (no
   dialog): `Enter` sends `setState` to `cancelled` with `handMove("cancelled", reason)`, an empty
   reason allowed; `Esc` closes it and sends nothing.
5. **`4` snoozes to the next 09:00 after tomorrow begins, in the reader's time zone.** Task 11 gives
   snooze a Hub field (`IssueHead.snoozedUntil`, `PatchIssueRequest.snoozedUntil`,
   `task-11.md:175-176`), so the scope's **UNVERIFIED** resolves: `4` is not a stub and no finding
   against task 03 is needed. A snoozed issue stays in Triage and sorts after the rest; it is
   never hidden (task 11, AX §7.4), and its row says `snoozed until Tue 09:00` in mono. Choosing
   another time and a reason is W2's *defer with a reason* (`H`, 05:1237), not this task's.
6. **The refusal names its reason and opens the fix.** The Hub answers a refused accept with 422
   `policy_refused`, `details.policy = "priorityToLeaveTriage"` (`task-11.md:379-381`). The replica
   rolls the optimistic move back (task 56) and calls `onRefused`; the Triage row then flashes
   `--fail` once, shows *Needs a priority to leave Triage* under its title, and opens
   `PriorityPicker` in place. Choosing a priority does **not** re-send the accept by itself: the
   reader presses `1` again, so no state move is ever sent twice without a key.
7. **The thread state reads on every row.** `threadState.kind === "not_tied"` renders
   `not tied · <reason>` in sans at `--ink-2`; `on_thread` renders one `ThreadNode` per served
   criterion (SW.14); `project_level` renders `project-level`. After `T`, the Hub's new revision
   carries the serve and the head's `threadState` becomes `on_thread` (task 09's rule; the desktop
   draws what the head says and never sets the thread state itself).
8. **Keys are the Triage list element's own.** `1`-`4` and `T` are bound in `TriageView`'s list
   `onKeyDown`, after `isBareListKey` (task 41). The Work list does not see them, and neither does
   a text field, the decline field included. Task 53 moves them into `KEYMAP` unchanged.

**Files** (the skeleton's file map lists `TriageView.tsx`; it omits `suggestCriterion.ts`,
`CriterionPicker.tsx` and their tests, a gap named in the return):
- `desktop/src/work/suggestCriterion.ts` (+ `suggestCriterion.test.ts`) — the pure matcher (new)
- `desktop/src/components/work/CriterionPicker.tsx` (+ `CriterionPicker.test.tsx`) — `T`'s picker; tasks 46 and 47 reuse it (new)
- `desktop/src/components/work/TriageView.tsx` (+ `TriageView.test.tsx`) — the view and its keys (new)
- `desktop/src/components/work/WorkLens.tsx` — a *Triage* action in its `ViewHeader`, switching the lens's view (modified, task 40's)
- `desktop/src/components/work/work.css` — `.wk-triage-*`, the `--fail` flash, the decline field (modified)

#### Interfaces — complete: `desktop/src/work/suggestCriterion.ts`

```ts
// desktop/src/work/suggestCriterion.ts — task 42. W1's criterion suggestion is text match
// (05 §2.3 point 4); W4's gateway suggestion (L-51, L-52) replaces the source, not the shape.

/** A criterion as the picker lists it: which brief it is in, in brief order. */
export interface BriefCriterion { briefKey: string; criterionId: string; text: string }

export interface CriterionSuggestion {
  briefKey: string;
  criterionId: string;
  text: string;
  /** The shared normalised terms, in the order they appear in the criterion: the why. */
  matched: readonly string[];
}

// A fixed list, so a suggestion never changes because a library did.
const STOP = new Set(("a an and are as at be but by for from has have if in into is it its not of on or " +
  "our so than that the their then there these this to was we were when where which while who will " +
  "with without after before must should can may").split(" "));

/** Lower case, marks removed, split on non-letters and non-digits, stop words and 1-char tokens out. */
export function terms(text: string): string[] {
  return text.normalize("NFD").replace(/\p{M}/gu, "").toLowerCase()
    .split(/[^\p{L}\p{N}]+/u)
    .filter((t) => t.length > 1 && !STOP.has(t));
}

export function suggestCriteria(
  issue: { title: string; description: string },
  criteria: readonly BriefCriterion[],   // brief order: the tie-break
  limit = 3,
): CriterionSuggestion[] {
  const mine = new Set(terms(`${issue.title} ${issue.description}`));
  return criteria
    .map((c, order) => {
      const matched = [...new Set(terms(c.text))].filter((t) => mine.has(t));
      return { order, s: { briefKey: c.briefKey, criterionId: c.criterionId, text: c.text, matched } };
    })
    .filter((x) => x.s.matched.length > 0)
    .sort((x, y) => y.s.matched.length - x.s.matched.length || x.order - y.order)
    .slice(0, Math.max(0, limit))
    .map((x) => x.s);
}
```

`CriterionSuggestion` has no `score`, `confidence` or `rank` field: the order is the only place the
count shows, and the why is the words.

#### Interfaces — complete: `desktop/src/components/work/CriterionPicker.tsx`

```tsx
// desktop/src/components/work/CriterionPicker.tsx — task 42; tasks 46 and 47 reuse it.
import type { JSX } from "react";
import { Picker, type PickerItem } from "../issue/pickers/Picker";          // task 46
import { suggestCriteria, type BriefCriterion } from "../../work/suggestCriterion";

type ServeRef = { briefKey: string; criterionId: string };
const same = (a: ServeRef, b: ServeRef): boolean => a.briefKey === b.briefKey && a.criterionId === b.criterionId;

/** Suggestions first, each with its matched words as the note; then every other criterion in
 *  brief order with no note. A criterion the issue already serves is not offered again. */
export function CriterionPicker(props: {
  issue: { title: string; description: string };
  criteria: readonly BriefCriterion[];   // every criterion of the project's briefs, in brief order
  serving: readonly ServeRef[];
  onChoose(c: BriefCriterion): void;
  onClose(): void;
}): JSX.Element {
  const { issue, criteria, serving, onChoose, onClose } = props;
  const open = criteria.filter((c) => !serving.some((s) => same(s, c)));
  const suggested = suggestCriteria(issue, open);
  const rest = open.filter((c) => !suggested.some((s) => same(s, c)));
  const ordered: { c: BriefCriterion; note?: string }[] = [
    ...suggested.map((s) => ({ c: s as BriefCriterion, note: `matches: ${s.matched.join(", ")}` })),
    ...rest.map((c) => ({ c })),
  ];
  // Item ids are positions in this list: a criterion id is unique only within its brief.
  const items: PickerItem[] = ordered.map((o, i) => ({ id: `crit-${i}`, label: o.c.text, note: o.note }));
  return (
    <Picker label="Serves which criterion" items={items} onClose={onClose}
      onChoose={(id) => { const o = ordered[Number(id.slice(5))]; if (o !== undefined) onChoose(o.c); }} />
  );
}
```

The note is sans (`PickerItem.note`, `task-46.md:161`), so the matched words read as words. No
suggestion yields no note on any row, and the list is still every criterion: the reader can always
choose by hand.

#### Interfaces — complete: `desktop/src/components/work/TriageView.tsx`, the exported half

```tsx
// desktop/src/components/work/TriageView.tsx — task 42. Embedded by the Work lens (its Triage
// view) and by Plan's first section (task 45). It draws no ViewHeader: its host does.
import type { JSX } from "react";
import type { IssueHead, IssueState, TeamPolicy } from "../../api/work";   // SW.15, task 37

/** The four number keys, by `event.key` (digits carry no agent act; Shift makes them symbols). */
export const TRIAGE_KEYS: Readonly<Record<string, "accept" | "duplicate" | "decline" | "snooze">> = {
  "1": "accept", "2": "duplicate", "3": "decline", "4": "snooze",
};

/** Decision 2: the team's default state, never triage (the Hub's own default is backlog). */
export function acceptTarget(policy: TeamPolicy | null): IssueState {
  const s = policy?.defaultState ?? "backlog";
  return s === "triage" ? "backlog" : s;
}

/** Decision 5: 09:00 on the reader's next calendar day, in their zone, as RFC 3339 UTC. */
export function nextMorning(nowMs: number, timeZone: string): string {
  const parts = (ms: number): Record<string, string> => Object.fromEntries(
    new Intl.DateTimeFormat("en-CA", { timeZone, year: "numeric", month: "2-digit", day: "2-digit",
      hour: "2-digit", minute: "2-digit", hourCycle: "h23" }).formatToParts(ms).map((p) => [p.type, p.value]));
  const p = parts(nowMs);
  const asUtc = Date.UTC(+p.year, +p.month - 1, +p.day + 1, 9, 0);
  const q = parts(asUtc);
  const offset = Date.UTC(+q.year, +q.month - 1, +q.day, +q.hour, +q.minute) - asUtc;
  return new Date(asUtc - offset).toISOString();
}

/** Decision 7, in words; `null` for on_thread, which draws ThreadNodes instead. */
export function threadWords(head: Pick<IssueHead, "threadState">): string | null {
  const t = head.threadState;
  if (t.kind === "on_thread") return null;
  if (t.kind === "project_level") return "project-level";
  return t.reason === "" ? "not tied" : `not tied · ${t.reason}`;
}

export function TriageView(props: {
  projectId: string;
  timeZone: string;       // the reader's; tests pin it
  now(): number;          // the heartbeat's clock in the app (hooks/heartbeat.ts), FIXED_NOW in tests
}): JSX.Element;
```

#### The ops this task sends (SW.16 `mutate`; the union is task 56's)

```ts
// Already in task 46's part (task-46.md:271, :298-301, :335): reused as written.
{ kind: "setState", issueId, move: handMove(acceptTarget(policy)) }            // 1
{ kind: "setState", issueId, move: handMove("cancelled", reason) }             // 3
[{ kind: "relation", issueId, add: { kind: "duplicates", issueId: otherId } },
 { kind: "setState", issueId, move: handMove("duplicate") }]                    // 2, via DuplicateOfPicker

// New here; task 56's WorkOp union must carry both (a gap, named in the return).
/** PatchIssueRequest.snoozedUntil (task 11). "" clears it. */
{ kind: "snooze", issueId: string, until: string }                              // 4
/** One new revision whose serves = head serves − remove + add (SW.1), matched on {briefKey, criterionId}; a kept pair keeps its `check` (23b). Task 43 re-points with it. */
{ kind: "serves", issueId: string, add: readonly Serve[], remove: readonly Serve[] }   // T
```

`serves` writes a revision through task 09's revise route with the head's title and description
unchanged; the replica's optimistic copy bumps `headRevision` and is replaced by the Hub's answer
(task 56). A `serves` op whose result equals the current list is not sent.

#### The refusal (decision 6), in `TriageView`

```tsx
// Refusal is SW.16's; its shape is task 56's, UNVERIFIED here beyond these four fields.
//   interface Refusal { op: WorkOp; code: string; message: string; details: Record<string, string> }
useEffect(() => onRefused((r) => {
  if (Array.isArray(r.op) || r.op.kind !== "setState" || !inView.has(r.op.issueId)) return;
  const issueId = r.op.issueId;
  const priority = r.code === "policy_refused" && r.details.policy === "priorityToLeaveTriage";
  setFailed({ issueId, why: priority ? "Needs a priority to leave Triage" : r.message, seq: Date.now() });
  if (priority) setOpen({ issueId, kind: "priority" });
}), [inView]);
```

The failed row carries `data-fail={seq}` for one `--dur-flash` (the `--fail` ink on the same
pseudo-element task 41 flashes), and the reason stays under its title until the row's next change.
`onRefused` returns its unsubscribe, which the effect returns.

#### `work.css`, appended

```css
/* task 42 — Triage. */
.wk-thread-words { color: var(--ink-2); font-size: var(--t-ui); }
.wk-snoozed { font-family: var(--mono); font-size: var(--t-fact); color: var(--ink-2); font-variant-numeric: tabular-nums; }
.wk-fail-why { color: var(--fail); font-size: var(--t-ui); }
.wk-row[data-fail] { position: relative; }
.wk-row[data-fail]::after { content: ""; position: absolute; inset: 0; pointer-events: none; border-radius: inherit;
  background: var(--fail); opacity: 0; animation: wk-flash var(--dur-flash) var(--ease) 1; }
.wk-decline { display: flex; gap: var(--s-2); align-items: center; min-block-size: var(--row-h); padding-inline: var(--s-3); }
@media (prefers-reduced-motion: reduce) { .wk-row[data-fail]::after { animation: none; } }
:root[data-motion="reduce"] .wk-row[data-fail]::after { animation: none; }
```

`--fail` is the built status token (`desktop/src/styles/tokens.css:76`, 6.06:1); `wk-flash` is
task 41's keyframes, which animate `opacity` only.

#### Tests to write first

`suggestCriterion.test.ts` (pure), `CriterionPicker.test.tsx` and `TriageView.test.tsx` (rendered on
task 20's `workScenario()` through task 40's test seam, **UNVERIFIED** name as in task 41). The
world's EP-160 is the imported triage row (`task-20.md:305-308`); the tests that need EP-142 in
triage override it to `{state: "triage", threadState: {kind: "not_tied", reason: "imported"}}` with
the title *Keep an upload going after a dropped connection*, and add the criterion *Uploads
survive a dropped connection* (`05-design.md:885`) **after** the fixture's two, so brief order is
wrong on purpose. (The title avoids *resume*, which would match the fixture's first criterion and
make test 1's *not the first* false; its terms are `keep upload going dropped connection`.)

| Test | What it asserts | Mutant it kills |
|---|---|---|
| `ranks the criterion with more shared words first` | for EP-142's text and criteria [*the resume test passes*, *upload works for PDF, XLSX and images*, *Uploads survive a dropped connection*], `suggestCriteria` returns the third first with `matched: ["dropped", "connection"]`, then the second with `["upload"]`, and not the first | brief-order output |
| `ties keep brief order; no match is no suggestion` | two criteria sharing one word each come back in brief order; a criterion sharing only stop words (`the`, `a`) is absent | a sort that is unstable, or counts stop words |
| `normalises case and diacritics` | `Résumé` in the issue matches `resume` in a criterion | an accent-sensitive match |
| `shows the matched words and no number` | open `CriterionPicker`; the first option's note reads `matches: dropped, connection`; no option's text contains a digit; `expectNoPrediction(container)` passes | a similarity percentage or a score |
| `1 in a text field and 1 in the Work list do nothing` | `keyDown {key: "1"}` on the decline field and on task 40's Work list send no op (a `mutate` spy); on a Triage row it sends one `setState` to `backlog` | a global number-key binding |
| `2 sends the relation and the state as one op with one Undo` | `2`, choose EP-150; the spy saw **one** `mutate` call whose argument is an array of two ops; one Undo chip | a state-only duplicate, or two chips |
| `3 declines with an optional reason; Esc sends nothing` | `3`, type *Superseded*, `Enter` → one `setState` to `cancelled` with that reason; `3`, `Esc` → none | a decline without its reason, or a field that sends on close |
| `4 snoozes to the next 09:00 and the row stays` | `now` = `FIXED_NOW` (09:00Z, 19:00 in Sydney), zone `Australia/Sydney`; `4` sends `snooze` with `until: "2026-09-18T23:00:00.000Z"`; the row is still listed, last, reading `snoozed until` | a snooze that hides the row |
| `a priority refusal rolls the row back and opens the picker` | the fake refuses with `policy_refused` / `priorityToLeaveTriage`; the row is back in Triage, has `data-fail`, reads *Needs a priority to leave Triage*, and a `listbox` labelled *Change priority* is open (task 46's `PriorityPicker`) | an accept that never rolls back, or a refusal shown as a toast only |
| `an imported row reads not tied · imported until T` | the row's text contains `not tied · imported` and no `.thr-node`; `T`, `Enter` on the first suggestion; after the fake's answer the row has a `ThreadNode` and not the words | hiding the thread state, or a desktop that sets it |

#### Steps

- [ ] Write `suggestCriterion.test.ts`; it fails to import. Write `suggestCriterion.ts`; three pass.
      Mutate: sort by `order` only (test 1 red); drop the stop-word filter (test 2 red); drop the
      `normalize` call (test 3 red). Revert each.
- [ ] Write `CriterionPicker.test.tsx`, then `CriterionPicker.tsx`; its test passes.
- [ ] Write `TriageView.test.tsx`; it fails to import. Write `TriageView.tsx` (the list, the rows
      with `threadWords` or `ThreadNode`s, the key handler after `isBareListKey`, the decline field,
      the snooze, the refusal effect), then the *Triage* action in `WorkLens`'s `ViewHeader`.
- [ ] Append the CSS. Run the whole suite and the typecheck. Commit each file as it goes green.

#### Acceptance

```bash
~/.claude/tools/claudezsh/claudezsh 'npm --prefix desktop test -- src/work/suggestCriterion.test.ts src/components/work/CriterionPicker.test.tsx src/components/work/TriageView.test.tsx'
# expected: 3 files, 10 tests, all passing
~/.claude/tools/claudezsh/claudezsh 'npm --prefix desktop test'
# expected: every suite passing
~/.claude/tools/claudezsh/claudezsh 'npm --prefix desktop run typecheck'
# expected: exit 0
~/.claude/tools/claudezsh/claudezsh 'grep -n "score\|confidence\|%" desktop/src/work/suggestCriterion.ts desktop/src/components/work/CriterionPicker.tsx'
# expected: no output
```

#### What this task does NOT claim

- **Gateway suggestions** of team, assignee, labels or criterion, and their why-links (L-51, L-52):
  W4. The shape `CriterionSuggestion` is kept for them; the source changes.
- **Defer with a reason and a chosen time** (`H`): W2 (05:1237). `4` is a fixed next-morning snooze.
- **`Shift+R` accept-and-Start** (L-47's EXCEED half): task 61.
- **Triage items on the Desk** (L-46's second half): task 63.
- **Stemming or synonyms** in the matcher; `upload` does not match `uploads`.
- That the suggestion is *good*: it is a text match, named as one, and the owner judges it at the
  demo (task 70), not this task's tests.

#### Gaps found while writing this part

1. The skeleton's row gives 42 `dependsOn: 40`; it also needs **46** (`Picker`, `handMove`,
   `DuplicateOfPicker`, `PriorityPicker`) and **41** (`listKeys.ts`, the hold model). The scope's
   *which tasks 46 and 47 reuse* runs the other way for `CriterionPicker`; 46's part does not use it.
2. The file map omits `suggestCriterion.ts`, `CriterionPicker.tsx` and their tests.
3. Task 56's `WorkOp` union needs `snooze` and `serves` (above); its `Refusal` shape is assumed.
4. The scope's `4` **UNVERIFIED** is resolved by task 11 (`snoozedUntil`); no finding against task 03.
5. Where the view reads the project's briefs' criteria in brief order is task 37/55's; phase 4's
   loader (**UNVERIFIED until merge**) is the source task 45 names.

