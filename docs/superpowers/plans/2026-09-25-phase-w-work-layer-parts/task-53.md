### Task 53: keys, part 1: `keys.list`, `isCockpitChord` extended, `event.code` for agent keys, the dead Linear chords, the single-key switch

Stream **WD1**, about 3 hours (an estimate, not a measurement). Depends on task 40 (the Work
list, `data-work-list`, `IssueRow`'s `data-issue-id`, `data-cursor`). Folds in the list-local
handlers of tasks 41 (`N`), 42 (`1`-`4`, `T`) and 44 (`X`, `Shift+↓`, `Shift+↑`), which were written
to move here unchanged (`task-41.md:48`, `task-42.md:64`, `task-44.md:46`). Uses task 41's
`isTextTarget` (`desktop/src/work/listKeys.ts`, `task-41.md:319-335`). No Hub dependency.

Parity: closes **L-223**'s single-key half (sequences are task 54's); binds **L-47**'s and
**L-38**'s keys. Design: 05 §4.3 (the four rules and the table, `05-design.md:960-1017`); 02 C9
(`02:292`); the skeleton's *Global constraints*, Keys.

**What the reader gets** (05 §4.3, W1 rows only):

| Pressed | Where | What happens |
|---|---|---|
| `S` | the Work list, a row current | the state picker opens beside the row (task 46's `StatePicker`) |
| `S` | a text field, the editor, a terminal | the letter is typed; nothing else |
| `Ctrl+K` | anywhere, the list included | the palette, as built, with OD-9's toggle |
| `Shift+R` | a row current; any layout, Caps Lock on or off | **Start** (task 61's verb) |
| `R` with Caps Lock and no Shift | a row current | nothing: no record verb is on `R` for an issue row, and Start needs Shift |
| `Shift+D`, `Shift+E`, `Shift+P`, `Shift+M`, `Shift+S` | anywhere | nothing; the `?` sheet (task 54) names our key for each |
| `Shift+O`, `Shift+A` | anywhere, in W1 | nothing (W3's Orchestrator keys, present and inert) |
| any single key, *Single-key shortcuts* off | a list | nothing; chords and the palette still work (WCAG 2.1.4) |

**Decisions made here, so nobody re-derives them:**

1. **The chord predicate is extended by a sibling branch, not by new keys.** `CHORD_KEYS`,
   `SHIFTED_CHORD_KEYS` (`hooks/useKeyboardMap.ts:55-56`) and `isCockpitChord`'s pinned source
   (`:66-80`) do not change, because a key added to them is a key xterm skips and the agent stops
   receiving (`:63-71`). The one change to `useKeyboardMap`'s listener is its first return: where it
   said `if (!isCockpitChord(event)) return;` it now says
   `if (!isCockpitChord(event)) { dispatchListKey(event); return; }`. `dispatchListKey` returns at
   once for any event with Ctrl, Meta or Alt held, so no chord can reach it, and for any target
   outside a `[data-work-list]`, so a terminal's keys (its host is never inside a list) and the
   body's keys are untouched. The window listener stays the one listener (`:1-3`).
2. **Record keys match the character; agent keys match the physical key.** A binding with
   `code` (only `shift: true` agent rows have one) matches `event.code === code && event.shiftKey`,
   so an AZERTY board and Caps Lock both start or do not start exactly as the key cap says. A
   record binding matches `event.key.toLowerCase()` with `event.shiftKey === false`, so the letter
   the reader sees is the letter that acts, and `Shift`+that letter does nothing (rule 2: no
   `Shift`+letter is a record act). Punctuation (`?`, `/`) sets `shift` to `undefined`, meaning not
   tested, because `?` is `Shift+/` on most layouts. Arrows set it `false` or `true` explicitly.
3. **One registry, and a binding does nothing until its verb is registered.** `registerVerb(verb,
   run)` returns its own unregister; a second registration of the same verb replaces the first
   until it unregisters (the newest mount wins, as a nested view needs). `runVerb(verb, row)`
   returns `false` when nothing is registered, and the dispatcher then does **not** call
   `preventDefault`, so an unbuilt verb never swallows a key.
4. **Where a key acts is declared by the DOM, not guessed.** A list element carries
   `data-work-list`, `tabIndex={0}`, `data-project-id` and `data-key-scope` (space-separated
   `KeyScope`s: the Work list and the board are `"list"`, Triage `"list triage"`). A row carries
   its own: every issue row `data-key-scope="issueRow"` (task 40's `.wk-item`, task 43's card),
   a Desk decision row `"deskDecision"` (task 62). The current row is the event target's nearest
   `[data-issue-id]`, else the list's `[data-cursor]` row; a current row adds its tokens and
   `everyRow`. `rowContextOf(target)` builds that once per key, so `S` with no current row matches
   nothing.
5. **`ACTIVE_SLICES` decides what is live.** It is `new Set(["W1"])` in this task; W2's `E`, `Y`,
   `B`, `D` and W3's `Shift+O`, `Shift+A` are rows in `KEYMAP` whose slice is not in it, so they
   match nothing and the sheet omits them. Task 102 widens the set to W3 (`task-102.md:244`).
6. **The five dead Linear chords are data**, `DEAD_LINEAR_CHORDS`, each with our equivalent in
   words. The dispatcher checks them first and returns without `preventDefault`, so `Shift+S` in a
   list is simply not a subscribe, and in a field it types `S`.
7. **The single-key switch** is `singleKeyShortcuts`, a persisted preference beside
   `sendChordsToTerminal` (`store/slices/prefsSlice.ts:40`, `:69`), default `true`. Off,
   `dispatchListKey` returns before matching, which turns off every single key and, because task
   54's sequence state is fed only by `dispatchListKey`, every sequence too. 05 puts it in
   *Settings → Keyboard*, which the desktop does not have, so it is a palette toggle row built on
   OD-9's pattern (`components/palette/commands.ts:129-144`), labelled *Single-key shortcuts*.
8. **The element handlers of tasks 41, 42 and 44 are deleted, not duplicated.** Each becomes a
   `registerVerb` call in the component that owns the behaviour (decision 3), made in a
   `useEffect` on mount and undone on unmount: `WorkLens` registers `held.apply`, `TriageView`
   the four `triage.*` verbs and `issue.criterion`, the bulk bar's owner `select.*`. The keys and
   their conditions are the ones those parts wrote; only where the match runs moves. Two
   precisions: task 41's `N` and task 44's `X` tested `event.code` (`KeyN`, `KeyX`); as record
   keys they now match the character (decision 2), which differs only on a layout that moves
   those letters. **`Esc` stays on task 44's list element**: the window map never handles Escape
   (`useKeyboardMap.ts:14-17`, task 18's `useDismissable` owns it), so clearing a selection with
   `Esc` remains the list's own handler, and `KEYMAP` has no `Esc` row.
9. **Verb ids are `<noun>.<verb>`**, the grammar the later parts already use (`optimistic.undo`,
   `task-56.md:462`; `list.search` and `issue.openByNumber`, `task-58.md:244-245`; `issue.start`,
   `task-61.md:112`; `timeline.zoomIn`, `task-128.md:526`), and task 126's write test reads their
   suffixes (`task-126.md:381-384`). `VerbId` is a string-literal union, so a misspelt
   registration fails typecheck; later tasks add their ids to it.

**Files** (the skeleton's file map names `keymap.ts` and `commands.ts` against 59 only; it omits
`store/store.ts` and the edits to tasks 40-44's files, a gap named below):
- `desktop/src/work/keys/keymap.ts` — types, `KEYMAP`, `DEAD_LINEAR_CHORDS`, the registry, the dispatcher (new)
- `desktop/src/work/keys/keymap.test.ts` — the table and dispatcher tests (new)
- `desktop/src/hooks/useKeyboardMap.ts` ⚑ — the first return calls `dispatchListKey` (modified)
- `desktop/src/hooks/useKeyboardMap.test.ts` ⚑ — three tests appended (modified)
- `desktop/src/store/slices/prefsSlice.ts` ⚑, `desktop/src/store/store.ts` ⚑ — `singleKeyShortcuts`, persisted (modified)
- `desktop/src/components/palette/commands.ts` ⚑ — the *Single-key shortcuts* toggle row (modified)
- `desktop/src/components/work/WorkLens.tsx`, `IssueRow.tsx` (task 40), `TriageView.tsx` (task 42), the bulk bar's owner (task 44) — attributes and registrations in place of `onKeyDown` (modified)

#### Interfaces — complete: `desktop/src/work/keys/keymap.ts`, part 1: the types

```ts
// desktop/src/work/keys/keymap.ts — task 53. keys.list (02 C9): the one table of single-key
// bindings, the one verb registry, and the dispatcher useKeyboardMap calls for every key that
// is not a cockpit chord. Chords are NOT here: CHORD_KEYS and isCockpitChord are untouched.
import { isInsideTerminal } from "../../hooks/useKeyboardMap";
import { useCockpit } from "../../store/store";
import { isTextTarget } from "../listKeys"; // task 41

/** Where a binding acts. Later tasks add scopes (task 128's "timeline"). */
export type KeyScope = "list" | "issueRow" | "triage" | "deskDecision" | "everyRow";

export type Slice = "W1" | "W2" | "W3" | "W4" | "W5";

/** Every verb a single key, a sequence (task 54) or a palette row (task 59) can name. */
export type VerbId =
  | "focus.next" | "focus.prev" | "open.issue" | "select.toggle" | "select.extendDown" | "select.extendUp"
  | "list.search" | "keys.sheet" | "optimistic.undo" | "held.apply" | "issue.create" | "row.defer"
  | "decision.approve" | "decision.defer" | "decision.rewind"
  | "issue.assign" | "issue.assignMe" | "issue.state" | "issue.priority" | "issue.label" | "list.filter"
  | "issue.criterion" | "triage.accept" | "triage.duplicate" | "triage.decline" | "triage.snooze"
  | "issue.start"
  | "issue.estimate" | "issue.cycle" | "issue.feature" | "issue.due"      // W2
  | "issue.delegate" | "row.ask"                                          // W3
  | "issue.openByNumber" | "issue.relate";                                // task 54's sequences' targets

export interface KeyBinding {
  verb: VerbId;
  /** What the sheet prints: "S", "Shift+R", "?", "Shift+↓". One entry per alternative. */
  keys: readonly string[];
  /** Set only on agent rows: the physical key (decision 2). */
  code?: string;
  /** true: Shift required; false: Shift forbidden; undefined: not tested (punctuation). */
  shift?: boolean;
  scope: KeyScope;
  slice: Slice;
  /** The sheet's and the palette's words, sentence case. */
  label: string;
  /** Against Linear's grammar (05 §4.3): same key, our own departure, or no Linear key. */
  linear: "match" | "depart" | "none";
}

/** What a runner acts on: the current row, if any, and the list it sits in. Task 59's
 *  openPalette passes the same shape, so a verb runs the same from a key or the palette. */
export interface RowContext {
  issueId: string | null;
  projectId: string | null;
  scopes: readonly KeyScope[];
  /** The row's element, for a picker or the palette to open beside (task 59). */
  element: HTMLElement | null;
}

export type VerbRunner = (row: RowContext) => void;

/** The slices whose rows are live. Task 102 widens it to W3. */
export const ACTIVE_SLICES: ReadonlySet<Slice> = new Set<Slice>(["W1"]);
```

`keymap.ts` imports `isInsideTerminal` from `useKeyboardMap.ts`, which imports `dispatchListKey`
from here: a cycle of function declarations, each read only when a key is pressed, so neither
module reads the other at load time. A `desktop/src/work/keys/keymap.cycle.test.ts` imports each
first in a fresh module graph (`vi.resetModules`) and expects both exports defined.

#### Interfaces — complete: `keymap.ts`, part 2: `KEYMAP` and `DEAD_LINEAR_CHORDS`

```ts
const r = (verb: VerbId, keys: string[], scope: KeyScope, slice: Slice, label: string,
  linear: KeyBinding["linear"], shift: boolean | undefined = false, code?: string): KeyBinding =>
  Object.freeze({ verb, keys: Object.freeze(keys), scope, slice, label, linear, shift, ...(code ? { code } : {}) });

/** 05 §4.3's table (`05-design.md:994-1013`), single keys only; G, O and M are task 54's. */
export const KEYMAP: readonly KeyBinding[] = Object.freeze([
  r("focus.next", ["J", "↓"], "list", "W1", "Next row", "match"),
  r("focus.prev", ["K", "↑"], "list", "W1", "Previous row", "match"),
  r("open.issue", ["Enter"], "issueRow", "W1", "Open the issue", "match"),
  r("select.toggle", ["X"], "list", "W1", "Select the row", "match"),
  r("select.extendDown", ["Shift+↓"], "list", "W1", "Extend the selection down", "match", true),
  r("select.extendUp", ["Shift+↑"], "list", "W1", "Extend the selection up", "match", true),
  r("list.search", ["/"], "list", "W1", "Search", "match", undefined),
  r("keys.sheet", ["?"], "list", "W1", "Keyboard shortcuts", "match", undefined),
  r("optimistic.undo", ["U"], "list", "W1", "Undo", "none"),
  r("held.apply", ["N"], "list", "W1", "Apply held changes", "none"),
  r("issue.create", ["C"], "list", "W1", "New issue", "match"),
  r("row.defer", ["H"], "everyRow", "W1", "Defer with a reason", "match"),
  r("decision.approve", ["A"], "deskDecision", "W1", "Approve", "none"),
  r("decision.defer", ["D"], "deskDecision", "W1", "Defer", "none"),
  r("decision.rewind", ["R"], "deskDecision", "W1", "Rewind an assumed decision", "none"),
  r("issue.assign", ["A"], "issueRow", "W1", "Assignee", "match"),
  r("issue.assignMe", ["I"], "issueRow", "W1", "Assign to me", "match"),
  r("issue.state", ["S"], "issueRow", "W1", "State", "match"),
  r("issue.priority", ["P"], "issueRow", "W1", "Priority", "match"),
  r("issue.label", ["L"], "issueRow", "W1", "Label", "match"),
  r("list.filter", ["F"], "list", "W1", "Filter", "match"),
  r("issue.criterion", ["T"], "issueRow", "W1", "Attach a criterion", "depart"),
  r("triage.accept", ["1"], "triage", "W1", "Accept", "match"),
  r("triage.duplicate", ["2"], "triage", "W1", "Mark as duplicate", "match"),
  r("triage.decline", ["3"], "triage", "W1", "Decline", "match"),
  r("triage.snooze", ["4"], "triage", "W1", "Snooze", "match"),
  r("issue.start", ["Shift+R"], "issueRow", "W1", "Start an agent on this issue", "depart", true, "KeyR"),
  r("issue.estimate", ["E"], "issueRow", "W2", "Estimate", "depart"),
  r("issue.cycle", ["Y"], "issueRow", "W2", "Cycle", "none"),
  r("issue.feature", ["B"], "issueRow", "W2", "Feature", "none"),
  r("issue.due", ["D"], "issueRow", "W2", "Due date", "depart"),
  r("issue.delegate", ["Shift+O"], "issueRow", "W3", "Delegate to my Orchestrator", "none", true, "KeyO"),
  r("row.ask", ["Shift+A"], "everyRow", "W3", "Ask my Orchestrator", "none", true, "KeyA"),
]);

/** Rule 3: Linear's five Shift chords do nothing here. The sheet prints `ours` beside each. */
export const DEAD_LINEAR_CHORDS: readonly { key: string; code: string; linear: string; ours: string }[] = Object.freeze([
  { key: "Shift+D", code: "KeyD", linear: "due date", ours: "D, from W2" },
  { key: "Shift+E", code: "KeyE", linear: "estimate", ours: "E, from W2" },
  { key: "Shift+P", code: "KeyP", linear: "project", ours: "B (feature), from W2" },
  { key: "Shift+M", code: "KeyM", linear: "milestone", ours: "the palette" },
  { key: "Shift+S", code: "KeyS", linear: "subscribe", ours: "Subscribe in the palette" },
]);
```

The `linear` column follows 05's table: **match** where it says so; `T` departs (Linear's `T`
is not a criterion), `E` and `D` depart from Linear's `Shift+E` and `Shift+D` (rule 3), and the
rest carry no Linear key. 01 §15.1's list is partly **UNVERIFIED** (`05-design.md:966-968`), so
the column is a claim to check against Linear's shortcut page before release, not a measurement.
*Shift+M → the palette* has no W1 milestone verb; the sheet's line says so and no more.

#### Interfaces — complete: `keymap.ts`, part 3: the registry and the dispatcher

```ts
const registry = new Map<VerbId, VerbRunner[]>();

/** Decision 3. The newest registration runs; its unregister removes exactly itself. */
export function registerVerb(verb: VerbId, run: VerbRunner): () => void {
  const stack = registry.get(verb) ?? [];
  const entry: VerbRunner = (row) => run(row);
  registry.set(verb, [...stack, entry]);
  return () => {
    const now = registry.get(verb) ?? [];
    const left = now.filter((r) => r !== entry);
    if (left.length === 0) registry.delete(verb); else registry.set(verb, left);
  };
}

export function hasVerb(verb: VerbId): boolean {
  return (registry.get(verb)?.length ?? 0) > 0;
}

/** false when nothing is registered, so the caller leaves the key alone (decision 3). */
export function runVerb(verb: VerbId, row: RowContext): boolean {
  const stack = registry.get(verb);
  if (!stack || stack.length === 0) return false;
  stack[stack.length - 1](row);
  return true;
}

/** Inside a [data-work-list]; never inside a terminal host, never in a field. */
export function isListKeyTarget(target: EventTarget | null): boolean {
  if (!(target instanceof Element)) return false;
  if (isInsideTerminal(target) || isTextTarget(target)) return false;
  return target.closest("[data-work-list]") !== null;
}

const tokens = (el: Element | null): KeyScope[] =>
  ((el?.getAttribute("data-key-scope") ?? "").split(/\s+/).filter(Boolean)) as KeyScope[];

/** Decision 4: the row's scopes first, then everyRow, then the list's. */
export function rowContextOf(target: EventTarget | null): RowContext {
  const el = target instanceof Element ? target : null;
  const list = el?.closest<HTMLElement>("[data-work-list]") ?? null;
  const row = el?.closest<HTMLElement>("[data-issue-id]")
    ?? list?.querySelector<HTMLElement>("[data-cursor][data-issue-id]") ?? null;
  const rowScopes = row ? [...tokens(row), "everyRow" as const] : [];
  return {
    issueId: row?.getAttribute("data-issue-id") ?? null,
    projectId: list?.getAttribute("data-project-id") ?? null,
    scopes: [...rowScopes, ...tokens(list)],
    element: row,
  };
}

const ARROW: Record<string, string> = { "↓": "ArrowDown", "↑": "ArrowUp" };
const baseOf = (k: string): string => { const b = k.replace(/^Shift\+/, ""); return ARROW[b] ?? b; };

function keyMatches(b: KeyBinding, e: KeyboardEvent): boolean {
  if (b.code !== undefined) return e.code === b.code && e.shiftKey;      // decision 2, agent keys
  if (b.shift !== undefined && e.shiftKey !== b.shift) return false;
  return b.keys.some((k) => {
    const base = baseOf(k);
    return base.length === 1 ? e.key.toLowerCase() === base.toLowerCase() : e.key === base;
  });
}

export function isDeadLinearChord(e: KeyboardEvent): boolean {
  return e.shiftKey && DEAD_LINEAR_CHORDS.some((d) => d.code === e.code);
}

/** The first active binding, in the context's scope order, whose key matches. */
export function matchListKey(e: KeyboardEvent, row: RowContext): KeyBinding | null {
  for (const scope of row.scopes) {
    const hit = KEYMAP.find((b) => b.scope === scope && ACTIVE_SLICES.has(b.slice) && keyMatches(b, e));
    if (hit) return hit;
  }
  return null;
}

/** The one entry useKeyboardMap calls, for every keydown that is not a cockpit chord. */
export function dispatchListKey(e: KeyboardEvent): void {
  if (e.ctrlKey || e.metaKey || e.altKey) return;                  // no chord ever reaches here
  if (!isListKeyTarget(e.target)) return;                          // fields, terminals, the body
  if (!useCockpit.getState().singleKeyShortcuts) return;           // decision 7, WCAG 2.1.4
  if (isDeadLinearChord(e)) return;                                // decision 6: no preventDefault
  const row = rowContextOf(e.target);
  // Task 54 inserts ONE line here: `if (feedSequence(e, row)) return;`
  const hit = matchListKey(e, row);
  if (hit && runVerb(hit.verb, row)) e.preventDefault();
}

/** A component's registration, made on mount and undone on unmount. The ref keeps the
 *  registration stable across renders while the runner reads the newest props. */
export function useVerb(verb: VerbId, run: VerbRunner): void {
  const ref = useRef(run);
  ref.current = run;
  useEffect(() => registerVerb(verb, (row) => ref.current(row)), [verb]);
}

/** Tests only: drops every registration between cases. */
export function resetVerbsForTest(): void {
  registry.clear();
}
```

`keymap.ts` also imports `useEffect` and `useRef` from `react` (for `useVerb` only); the rest of
the module is plain functions that a test calls with a synthetic `KeyboardEvent`.

#### Interfaces — complete: the deltas on ⚑ `useKeyboardMap.ts`, `prefsSlice.ts`, `store.ts`

```ts
// useKeyboardMap.ts, at :126 — the one changed line; nothing else in the file moves.
import { dispatchListKey } from "../work/keys/keymap";
      if (!isCockpitChord(event)) { dispatchListKey(event); return; }
```

```ts
// prefsSlice.ts — beside sendChordsToTerminal at :40, :69, :79 and :155-158.
  /** Task 53. Off turns every single key and sequence off; chords and the palette stay. */
  singleKeyShortcuts: boolean;
  setSingleKeyShortcuts(on: boolean): void;
// PrefsState's Pick gains "singleKeyShortcuts"; initialPrefsState gains `singleKeyShortcuts: true`.
  setSingleKeyShortcuts: (on) => {
    if (get().singleKeyShortcuts === on) return;
    set({ singleKeyShortcuts: on });
  },
```

```ts
// store.ts — Persisted's Pick (:21-28) gains | "singleKeyShortcuts"; partialize (:34-48) gains
  singleKeyShortcuts: s.singleKeyShortcuts,
```

Same version, no migration, for the reason `store.ts:30-33` gives for `sendChordsToTerminal`: a
snapshot written before the key rehydrates with the initial value, here `true`, which is how the
app behaved when it was written. `store.test.ts` gains that case beside OD-9's.

#### Interfaces — complete: the toggle row on ⚑ `commands.ts`

```ts
// commands.ts — directly after OD-9's row (:129-144), on its pattern: label fixed, state in `checked`.
export const SINGLE_KEY_LABEL = "Single-key shortcuts";
  const singleKeys = state.singleKeyShortcuts;
  commands.push({
    id: "verb:single-key-shortcuts",
    label: SINGLE_KEY_LABEL,
    glyph: GLYPH.verb,
    checked: singleKeys,
    run: () => {
      state.setSingleKeyShortcuts(!singleKeys);
      state.pushToast(
        singleKeys
          ? "Single-key shortcuts are off. Ctrl chords and this palette still work."
          : "Single-key shortcuts are on in lists.",
        () => state.setSingleKeyShortcuts(singleKeys),
      );
    },
  });
```

No hint: no key toggles it, because a single key that turned single keys back on would be off
with them. The toast's second sentence says the one thing the reader needs to get back.

#### Interfaces — complete: the edits to tasks 40-44's files

- **`WorkLens.tsx`** (task 40). The list element gains `tabIndex={0}`, `data-project-id={projectId}`
  and `data-key-scope="list"`. The lens registers, with `useVerb`: `focus.next` and `focus.prev`
  (move the cursor one row, skipping task 41's held records as `task-41.md:308` says), `open.issue`
  (the row's title link's own action), `held.apply` (task 41's apply, the body of its deleted
  `onKeyDown`), and the four row pickers below. `keys.sheet` is task 54's; `list.search` is task 58's.
- **The row pickers**, in `WorkLens.tsx`: one state `open: { field: "state" | "priority" | "assignee"
  | "labels"; issueId: string; at: DOMRect } | null`, set by `issue.state`, `issue.priority`,
  `issue.assign` and `issue.label` from `row.element.getBoundingClientRect()`, and one `Popover`
  (`anchor={open?.at ?? null}`) holding task 46's `StatePicker`, `PriorityPicker`, `AssigneePicker`
  or `LabelPicker` with the props `IssueHead` passes them (`task-46.md:624-629`), read through
  `useWork((r) => r.issue(open.issueId))`. `issue.assignMe` calls task 46's `assign(head, me, me)`
  (`task-46.md:358`) with `me` from `useWork((r) => r.me())`, and opens nothing. A runner given a
  row with `issueId === null` returns at once; `matchListKey` has already made that rare (decision 4).
- **`IssueRow.tsx`** (task 40). `.wk-row` gains `data-issue-id={issue.id}` and
  `data-key-scope="issueRow"`. Task 43's card gains the same two.
- **`TriageView.tsx`** (task 42). Its list element gains `data-key-scope="list triage"`; its
  `onKeyDown` is deleted and its body becomes `useVerb` calls for `triage.accept`, `triage.duplicate`,
  `triage.decline`, `triage.snooze` and `issue.criterion`, each doing what that key did.
- **The bulk bar's owner** (task 44). Its `onKeyDown` for `X`, `Shift+↓` and `Shift+↑` is deleted and
  becomes `useVerb` calls for `select.toggle`, `select.extendDown` and `select.extendUp`. `Esc` stays
  where it is (decision 8).

`row.defer`, `decision.*` (task 62), `issue.create` (task 68), `optimistic.undo` (task 56) and
`issue.start` (task 61) are bound here and registered by their own tasks; until then the key does
nothing and is not prevented (decision 3).

#### Tests to write first, part 1 — `desktop/src/work/keys/keymap.test.ts` (the table and the dispatcher)

A fixture builds `<div data-work-list tabindex="0" data-project-id="p1" data-key-scope="list">`
with two `.wk-row`s (`data-issue-id="i1"` with `data-cursor`, and `"i2"`, both
`data-key-scope="issueRow"`), an `<input>` (`listInput`) and a `<div contenteditable="true">`
(`listEditor`) inside the list, and a `<div data-terminal-host><textarea/></div>` outside it; `key(target, init)` dispatches a
`KeyboardEvent("keydown", { bubbles: true, cancelable: true, ...init })` on the target and returns
it. `beforeEach` calls `resetVerbsForTest()` and sets `singleKeyShortcuts: true`.

```ts
it("S on a row runs issue.state with that row; S in a field or a terminal runs nothing", () => {
  const seen: RowContext[] = [];
  registerVerb("issue.state", (r) => seen.push(r));
  dispatchListKey(key(row("i2"), { key: "s", code: "KeyS" }));
  dispatchListKey(key(listInput, { key: "s", code: "KeyS" }));
  dispatchListKey(key(listEditor, { key: "s", code: "KeyS" }));   // contenteditable, as task 49's
  dispatchListKey(key(terminalTextarea, { key: "s", code: "KeyS" }));
  dispatchListKey(key(document.body, { key: "s", code: "KeyS" }));
  expect(seen.map((r) => r.issueId)).toEqual(["i2"]);
});
// kills: a document-level single key, and a list key that fires inside a field or a terminal.

it("with focus on the list itself, the cursor row is the current row", () => {
  const seen: (string | null)[] = [];
  registerVerb("issue.priority", (r) => seen.push(r.issueId));
  dispatchListKey(key(list, { key: "p", code: "KeyP" }));
  expect(seen).toEqual(["i1"]);
});
// kills: rowContextOf that ignores [data-cursor], so P on a focused list acts on nothing.

it("Shift+R by code starts; Caps Lock R without Shift does not; AZERTY key with KeyR still starts", () => {
  const runs: string[] = [];
  registerVerb("issue.start", (r) => runs.push(r.issueId!));
  dispatchListKey(key(row("i1"), { key: "R", code: "KeyR", shiftKey: true }));
  dispatchListKey(key(row("i1"), { key: "R", code: "KeyR", shiftKey: false })); // Caps Lock
  dispatchListKey(key(row("i1"), { key: "Q", code: "KeyR", shiftKey: true }));  // a moved layout
  expect(runs).toEqual(["i1", "i1"]);
});
// kills: matching the agent key by event.key, and an agent verb reachable without Shift.

it("each dead Linear chord dispatches no verb and is not prevented", () => {
  for (const v of ["issue.due", "issue.estimate", "issue.feature", "issue.state", "issue.label"] as const)
    registerVerb(v, () => { throw new Error(`${v} ran`); });
  for (const d of DEAD_LINEAR_CHORDS) {
    const e = key(row("i1"), { key: d.key.slice(-1), code: d.code, shiftKey: true });
    dispatchListKey(e);
    expect(e.defaultPrevented).toBe(false);
  }
});
// kills: a Linear Shift chord wired to a verb (Shift+S as subscribe, Shift+D as due).

it("every Shift+letter in KEYMAP is an agent verb with a code, and no record verb needs Shift", () => {
  const AGENT = new Set<VerbId>(["issue.start", "issue.delegate", "row.ask"]);
  for (const b of KEYMAP) {
    const shiftLetter = b.keys.some((k) => /^Shift\+[A-Z]$/.test(k));
    expect(shiftLetter).toBe(AGENT.has(b.verb));
    if (shiftLetter) expect(b.code).toMatch(/^Key[A-Z]$/);
  }
});
// kills: a Shift record act (rule 2), and an agent row that forgot its physical code.

it("Shift+O and Shift+A are inert in W1 and nothing is prevented", () => {
  registerVerb("issue.delegate", () => { throw new Error("delegate ran"); });
  registerVerb("row.ask", () => { throw new Error("ask ran"); });
  const o = key(row("i1"), { key: "O", code: "KeyO", shiftKey: true });
  const a = key(row("i1"), { key: "A", code: "KeyA", shiftKey: true });
  dispatchListKey(o); dispatchListKey(a);
  expect([o.defaultPrevented, a.defaultPrevented]).toEqual([false, false]);
});
// kills: an Orchestrator key live before W3 (ACTIVE_SLICES ignored).
```

#### Tests to write first, part 2 — the switch, the registry, and the three appended to `useKeyboardMap.test.ts`

```ts
// keymap.test.ts, continued.
it("with the switch off, S, N and 1 do nothing and are not prevented", () => {
  useCockpit.setState({ singleKeyShortcuts: false });
  for (const v of ["issue.state", "held.apply", "triage.accept"] as const)
    registerVerb(v, () => { throw new Error(`${v} ran`); });
  triageList.setAttribute("data-key-scope", "list triage");
  for (const [k, c] of [["s", "KeyS"], ["n", "KeyN"], ["1", "Digit1"]] as const) {
    const e = key(row("i1"), { key: k, code: c });
    dispatchListKey(e);
    expect(e.defaultPrevented).toBe(false);
  }
});
// kills: a switch read after the match, or one that only some keys honour.

it("a bound key with nothing registered is left alone; the newest registration wins, and unwinds", () => {
  const e1 = key(row("i1"), { key: "c", code: "KeyC" });
  dispatchListKey(e1);
  expect(e1.defaultPrevented).toBe(false);
  const who: string[] = [];
  const offOuter = registerVerb("issue.create", () => who.push("outer"));
  const offInner = registerVerb("issue.create", () => who.push("inner"));
  dispatchListKey(key(row("i1"), { key: "c", code: "KeyC" }));
  offInner();
  dispatchListKey(key(row("i1"), { key: "c", code: "KeyC" }));
  offOuter();
  expect(who).toEqual(["inner", "outer"]);
  expect(hasVerb("issue.create")).toBe(false);
});
// kills: an unbuilt verb that swallows its key, and an unregister that drops the wrong entry.

it("A on a Desk decision row approves; A on an issue row assigns", () => {
  const got: string[] = [];
  registerVerb("decision.approve", () => got.push("approve"));
  registerVerb("issue.assign", () => got.push("assign"));
  row("i2").setAttribute("data-key-scope", "deskDecision");
  dispatchListKey(key(row("i2"), { key: "a", code: "KeyA" }));
  dispatchListKey(key(row("i1"), { key: "a", code: "KeyA" }));
  expect(got).toEqual(["approve", "assign"]);
});
// kills: scope order ignored, so the list-wide or first-declared binding wins on every row.
```

```ts
// useKeyboardMap.test.ts, appended in a describe("task 53 — the list branch").
it("CHORD_KEYS and SHIFTED_CHORD_KEYS equal their values before task 53", () => {
  expect(CHORD_KEYS).toEqual(["w", "k", "b", "Enter", "\\", "1", "2", "3", "4", "5", "6", "7", "8", "9"]);
  expect(SHIFTED_CHORD_KEYS).toEqual(["m"]);
});
// kills: a single key added to the chord lists, which xterm would then skip.

it("Ctrl+K on a focused work list opens the palette and runs no list verb", () => {
  registerVerb("focus.prev", () => { throw new Error("K reached the list"); });
  expect(pressOn(list, "k", { ctrlKey: true }).defaultPrevented).toBe(true);
  expect(useCockpit.getState().paletteOpen).toBe(true);
});
// kills: a list branch placed before the chord branch, which swallows chords (Ctrl+K as K).

it("Ctrl+K with the switch off still opens the palette", () => {
  useCockpit.setState({ singleKeyShortcuts: false });
  pressOn(list, "k", { ctrlKey: true });
  expect(useCockpit.getState().paletteOpen).toBe(true);
});
// kills: a switch that turns chords off with the single keys.
```

`pressOn(target, key, init)` is the file's existing helper (`useKeyboardMap.test.ts:68-72`), and
`paletteOpen` is the flag its `⌘K toggles the palette` case reads (`:188-199`); `list` is the same
fixture as `keymap.test.ts`'s, appended to `document.body`. The five existing non-interception
cases (`:103-147`) stay unchanged and green: `/` and bare letters there are pressed on a
`focusedTerminal()` textarea outside any `[data-work-list]`, which `isListKeyTarget` rejects.

`keymap.cycle.test.ts`: `vi.resetModules()`, import `keymap.ts` first and check `dispatchListKey`
and `isInsideTerminal` are functions; reset, import `useKeyboardMap.ts` first, check the same.
**Kills:** a module-level read across the cycle (a `const` computed from the other module at load).

`store.test.ts` gains: a v1 snapshot without `singleKeyShortcuts` rehydrates it as `true`, and
`partialize`'s keys include it. **Kills:** a switch that resets on every launch.

`commands.test.ts` gains: the row labelled `SINGLE_KEY_LABEL` is present, `checked` follows the
store, `run` flips it and its toast's undo restores it, and it has no `hint`. **Kills:** a switch
the reader cannot reach, and a hint for a key that does not exist.

The suites of tasks 41, 42 and 44 that press `N`, `1`-`4`, `T`, `X` or `Shift+↓` on the list
element keep their assertions unchanged; each gains `renderHook(() => useKeyboardMap())` in its
`beforeEach`, because the key now reaches the verb through the window listener and no longer
through the element's own handler. A suite that still passes **without** that line has kept a
stale `onKeyDown`, which is the point of adding it.

#### Steps

- [ ] **Step 1: read on the sync base** `useKeyboardMap.ts` (`:55-80`, `:105-135`), `prefsSlice.ts`,
      `store.ts`, `commands.ts` (`:110-144`) and tasks 40-44 and 46's merged files, so each line
      cited above still names what it names; correct any citation that moved.
- [ ] **Step 2: write the tests** of both parts, the cycle test and the store and commands cases;
      run them and see each fail for its stated reason (no module, or the verb not reached).
- [ ] **Step 3: write `keymap.ts`** in its three parts; the table and dispatcher cases pass.
- [ ] **Step 4: the ⚑ deltas**: the one line in `useKeyboardMap.ts`, the preference in
      `prefsSlice.ts` and `store.ts`, the toggle row in `commands.ts`; their suites pass, the
      pre-existing cases included.
- [ ] **Step 5: move the handlers**: delete the `onKeyDown`s of tasks 41, 42 and 44, add the
      attributes and the `useVerb` calls, add the row pickers to `WorkLens.tsx`; add the
      `renderHook` line to those suites and see them pass.
- [ ] **Step 6: commit** with `git add` on the new files and `git commit -F <msg> -- <paths>`.

#### Acceptance

```bash
npm --prefix desktop test -- keymap
# expected: 2 files (keymap.test.ts, keymap.cycle.test.ts), 11 tests, all passing
npm --prefix desktop test -- useKeyboardMap store commands
# expected: every suite passing; 3 new useKeyboardMap cases, 2 store, 1 commands
npm --prefix desktop test -- work
# expected: every suite passing, tasks 40-44's included, with the renderHook line
npm --prefix desktop run typecheck
# expected: exit 0
git grep -nE '"KeyN"|"KeyX"' -- desktop/src/components/work ':!*.test.*'
# expected: no output (tasks 41 and 44's element handlers are gone; KEYMAP matches the letter)
git grep -n "isBareListKey" -- desktop/src/components/work/TriageView.tsx desktop/src/components/work/WorkLens.tsx
# expected: no output (task 44's Esc and its confirm keys, if they still use it, live elsewhere)
git diff --stat <sync-base> -- desktop/src/hooks/useKeyboardMap.ts
# expected: 1 file changed, 2 insertions(+), 1 deletion(-) (the import and the one line)
```

The count is this part's: 9 cases in `keymap.test.ts` (6 in part 1, 3 in part 2) and 2 in the
cycle file.

#### What this task does NOT claim

- Sequences (`G …`, `O …`, `M …`), the `?` sheet, chords in menus and the safe triangle: task 54.
  Task 54 adds its one line to `dispatchListKey` and its `SEQUENCES` beside `KEYMAP`.
- `E`, `Y`, `B`, `D` as W2 row keys, and `Shift+O`, `Shift+A` (W3, task 102): rows present, inert.
- The verbs' own behaviour: Start (61), Undo (56), create (68), search (58), the Desk (62), defer.
  A key whose verb nobody has registered does nothing and is not prevented.
- A *Settings → Keyboard* page. The switch is a palette row until one exists.
- That `KEYMAP`'s `linear` column is right: 01 §15.1 is partly **UNVERIFIED** (see part 2's note).
- Any measured keyboard latency; nothing here is timed.

#### Gaps found while writing this part

1. **File map omissions** (row 309 names `useKeyboardMap.ts` and `keymap.ts` only):
   `keymap.cycle.test.ts`, ⚑ `prefsSlice.ts`, ⚑ `store.ts` (and `store.test.ts`), the toggle row in
   ⚑ `commands.ts` (row 310 names that file against 59 only), and the edits to tasks 40-44's
   components and their suites.
2. **The verbs bound here and registered later have no part yet on this tree** for tasks 62
   (`decision.*`), 68 (`issue.create`) and the owner of `row.defer` (`H`, *defer with a reason*):
   no task row names `row.defer`'s registration. Until one does, `H` does nothing.
3. **`A` and `D` mean two things by scope** (`decision.approve` on a Desk decision row,
   `issue.assign` on an issue row; `decision.defer` in W1, `issue.due` from W2). Decision 4's scope
   order settles it for W1. If task 62's Desk row is also an issue row, its `data-key-scope` must
   list `deskDecision` first, and task 62's part must say so.
4. **The suites of tasks 41, 42 and 44 change** (a `renderHook` line each), which those parts did
   not foresee; their assertions do not change.
5. **`N` and `X` move from `event.code` to the character** (decision 8). On a layout that moves
   those letters the key cap now acts where the position did before; recorded, not measured.
6. **The size is provisional**: the handler move and the row pickers make this part closer to the
   top of its 3 h than the middle.
