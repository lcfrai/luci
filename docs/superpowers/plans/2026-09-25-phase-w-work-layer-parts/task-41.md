### Task 41: `arrivals.hold`, `moves.hold` and departures; `N` applies

Stream **WD1**, about 3 hours (an estimate, not a measurement). Depends on task 40 (the Work lens,
`CriterionGroup`, `IssueRow`, `work.css`). Uses SW.8 (task 20's `expectNoPrediction` and
`workScenario()`), SW.11 (`--dur-flash`, task 26), SW.12 (`StateGlyph`, task 28) and SW.16's
signatures (`useWork(selector)`), through task 20's fixtures until task 55 lands. No Hub dependency.

This is Architecture rule 4, *rows never move under the reader* (05 §4.1, work rule 3; 02 C7
`arrivals.hold`, `02:267`). It closes no parity row alone; L-101's list (task 40), the board
(task 43), Plan (task 45) and the Desk (task 62) all read the model this task builds.

**What the reader sees** (05 §4.1 D C2, §4.4 C7; the prototype's words, `prototype/work-layer.html:620-638`):

| Change arriving from the replica | What happens on screen | Until |
|---|---|---|
| a new issue joins a group (an **arrival**) | it is not drawn; one `HeldLine` at the top of its group reads *One new issue is held, so no row moves under you.* with *Apply `N`* | `N`, or the view unmounts |
| an issue's group or state changes (a **move**) | the row **stays at its index**, takes its new `StateGlyph` at once, flashes once over `--dur-flash`, and shows one mono cause line, e.g. `→ done · own evidence at 7c1f0a2 (test run, exit 0) · 14:02`; its group header adds `· 1 moving` | `N`, unmount, or the reader's attention is outside its group |
| an issue leaves the view (a **departure**) | the row becomes a one-line ghost: `EP-128 left this view · cancelled`, with *`N` applies* | `N`, or unmount |
| more than three changes in one group in one update | the **header** flashes once and **no row** flashes (D M9) | — |

**Decisions made here, so nobody re-derives them:**

1. **The model is pure and lives in `desktop/src/work/hold.ts`.** It knows nothing of React, the
   replica or the DOM. Task 40's `WorkLens` owns one `HoldState` in component state (`useState`),
   so it dies with the view: remounting starts from `initialHold(next)`, which is everything
   applied (the scope's *held state that outlives the view*). It is **not** in `workSlice`: a hold
   is a fact about this mount, not a view model other surfaces read.
2. **Attention, not just focus.** The scope says a row relocates when *focus is outside its group*.
   A reader reading with the pointer has no keyboard focus, and a row that jumps under the pointer
   breaks rule 4 as surely as one that jumps under the caret. So `focus.groupKey` is the group
   holding keyboard focus **or else** the group under the pointer; the lens tracks both on each
   `CriterionGroup` (`onFocus`/`onBlur` bubble; `onPointerEnter`/`onPointerLeave`). `null` means
   the reader is in neither, and held moves relocate at once. Arrivals and ghosts are held
   regardless of attention: an arrival is invisible until asked for, and a ghost is the only
   record that something left.
3. **The cause line is formatted here, from SW.4's words.** `causeLine(move, timeZone)` maps task
   03's six `MoveCause` values (`task-03.md:211-216`) to fixed phrases and never guesses:
   `hand` → *set by <name>*, `lane_bound` → *a lane started*, `lane_queued` → *worktree queued*,
   `land_trailer` → *landed at <commit7>*, `evidence_observed` → *own evidence at <commit7>*,
   `evidence_recorded` → *evidence recorded at <commit7>*. The parenthesis `(test run, exit 0)` is
   printed only when the caller passes `detail`; W1's feed data carries `to` and `cause`
   (`task-20.md:330-331`), and the evidence detail's source is task 23's evidence record
   (**UNVERIFIED** until task 37 fixes the desktop shape). No detail, no parenthesis: nothing is
   invented.
4. **`N` is bound on the list element's own `onKeyDown`** in `WorkLens`, never on `document`. It
   ignores an event whose target is an `input`, `textarea`, `select` or `[contenteditable]`, and any
   event with `ctrlKey`, `metaKey` or `altKey`. It tests `event.code === "KeyN"` without
   `shiftKey`, the agent-key rule of Global constraints. Task 53 moves it into `KEYMAP` unchanged.
5. **One flash per update, by sequence number.** `HoldState.flash` carries the ids to flash and a
   `seq`; a row renders `data-flash={seq}` only while it is in `flash.rows`. CSS restarts the
   animation because the element's attribute value changes. The animation is on a pseudo-element's
   `opacity` only, over `var(--dur-flash)`.
6. **`data-changed="unseen"`** sits on every row whose move is held (it has a cause line). Task 34's
   reduced-motion layer draws it as the static 2px tick (05 §4.4 C7); this task sets the attribute
   and styles nothing for it. Task 34 is not upstream of task 41 in `dependsOn`, so under reduced
   motion today the row shows its cause line and no tick (named, not a gap: the tick is task 34's).
7. **Group counts are plain.** A header reads `3 issues` or `3 issues · 1 moving`. The count is the
   live rows shown in that group, ghosts excluded. No `of`, no total, no fraction.

**Files** (the skeleton's file map lists `HeldLine.tsx` against tasks 40-41; it omits `hold.ts` and
`hold.test.ts`, a gap named in the return):
- `desktop/src/work/hold.ts` — the pure model (new)
- `desktop/src/work/hold.test.ts` — six model tests (new)
- `desktop/src/components/work/HeldLine.tsx` — the per-group held line and the ghost row (new)
- `desktop/src/components/work/WorkLens.tsx` — owns `HoldState`, binds `N`, tracks attention (modified, task 40's)
- `desktop/src/components/work/CriterionGroup.tsx` — renders `HeldLine`, ghosts, `· N moving`, the header flash (modified, task 40's)
- `desktop/src/components/work/IssueRow.tsx` — `data-flash`, `data-changed`, the cause line (modified, task 40's)
- `desktop/src/components/work/work.css` — `.wk-held`, `.wk-ghost`, `.wk-cause`, the flash keyframes (modified, task 40's)
- `desktop/src/components/work/hold.test.tsx` — four surface tests (new)

#### Interfaces — complete: `desktop/src/work/hold.ts`, the types and signatures

```ts
// desktop/src/work/hold.ts — task 41. arrivals.hold, moves.hold and departures (05 §4.1 D C2,
// §4.4 C7; 02 C7). Pure: no React, no replica, no DOM. The lens owns one HoldState per mount.
import type { IssueState } from "../components/glyphs/StateGlyph"; // SW.12

/** Task 03's six causes (`task-03.md:211-216`), mirrored until task 37 exports them (SW.15). */
export type MoveCause =
  | "hand" | "lane_bound" | "lane_queued" | "land_trailer" | "evidence_observed" | "evidence_recorded";

/** The last state move the replica holds for an issue: SW.4's StateMove plus when and by whom. */
export interface MoveFact {
  to: IssueState;
  cause: MoveCause;
  at: string;       // RFC 3339, the move event's time
  byName: string;   // the person whose act caused it (rule 2: facts are written as the person)
  commit: string;   // 40 hex, or "" unless land_trailer or an evidence cause
  detail: string;   // e.g. "test run, exit 0" from the evidence record; "" when none is known
}

/** One row as the view's selector hands it over: already filtered, grouped and ordered. */
export interface RowFact {
  issueId: string;  // is-…, the key; never the alias
  alias: string;    // "EP-142", display only (SW.1 DisplayAlias)
  state: IssueState;
  cause: string;    // causeLine(lastMove, tz), or "" when the replica holds no move
}

export interface GroupedIssues {
  groups: readonly { key: string; rows: readonly RowFact[] }[];
  /** Issues the replica knows that this view's filter now excludes, with the one word why
   *  ("cancelled", "duplicate", "moved to another team"). Absent means "no longer visible". */
  left: ReadonlyMap<string, string>;
}

export type HeldChange =
  | { kind: "arrival"; issueId: string; groupKey: string }
  | { kind: "move"; issueId: string; fromGroup: string; toGroup: string; cause: string }
  | { kind: "departure"; issueId: string; groupKey: string; alias: string; why: string };

export type ShownRow =
  /** `home` is the group the facts put it in at the last reconcile; `cause` non-null means a
   *  change the reader has not applied: the row shows the line and carries data-changed="unseen". */
  | { kind: "row"; row: RowFact; home: string; cause: string | null }
  | { kind: "ghost"; issueId: string; alias: string; why: string };

export interface ShownGroup {
  key: string;
  rows: readonly ShownRow[];
  arrivals: number;  // held arrivals into this group: the HeldLine's count
  moving: number;    // rows held here whose home is another group: the header's "· N moving"
}

/** The order the reader sees, plus what is held. */
export interface HoldState {
  shown: readonly ShownGroup[];
  held: readonly HeldChange[];
  /** One flash per update: rows flash unless their group had more than HEADER_FLASH_OVER changes,
   *  in which case the header flashes and no row does (D M9). `seq` restarts the CSS animation. */
  flash: { seq: number; rows: ReadonlySet<string>; groups: ReadonlySet<string> };
}

/** D M9: more than three changes in one group in one update flash the header, not the rows. */
export const HEADER_FLASH_OVER = 3;

/** Where the reader is: the group with keyboard focus, else the group under the pointer. */
export interface Attention { groupKey: string | null }

export function initialHold(next: GroupedIssues, seq?: number): HoldState;
export function reconcile(prev: HoldState, next: GroupedIssues, focus: Attention): HoldState;
export function applyHeld(state: HoldState, next: GroupedIssues): HoldState;
export function causeLine(move: MoveFact, timeZone: string): string;
/** "3 issues", "1 issue", "3 issues · 1 moving": a plain count, never "of". */
export function groupSize(group: ShownGroup): string;
/** The HeldLine's words: "One change is held, so no row moves under you." / "2 changes are held, …" */
export function heldWords(n: number): string;
```

`RowFact.cause` is formatted by the selector (task 40's), so the model is free of time zones and
compares strings only. A move is held **in place** while the reader's attention is in its shown
group **or** its home group; it relocates at once only when attention is in neither. That is how the
scope's *focus outside its group* is read here, so that a row can never vanish from one group
while the other holds it out: a gap in the scope's wording, named in the return.

#### Reference implementation: `reconcile` (the tests below are the contract; this is one way to pass them)

```ts
const EMPTY: ReadonlySet<string> = new Set();

export function initialHold(next: GroupedIssues, seq = 0): HoldState {
  return {
    shown: next.groups.map((g) => ({
      key: g.key, arrivals: 0, moving: 0,
      rows: g.rows.map((row): ShownRow => ({ kind: "row", row, home: g.key, cause: null })),
    })),
    held: [],
    flash: { seq, rows: EMPTY, groups: EMPTY },
  };
}

/** N, or the view unmounting: everything held is laid out from the latest facts. */
export function applyHeld(state: HoldState, next: GroupedIssues): HoldState {
  return initialHold(next, state.flash.seq);
}

export function reconcile(prev: HoldState, next: GroupedIssues, focus: Attention): HoldState {
  const home = new Map<string, string>();
  const fact = new Map<string, RowFact>();
  for (const g of next.groups) for (const r of g.rows) { home.set(r.issueId, g.key); fact.set(r.issueId, r); }
  const watched = (k: string): boolean => k === focus.groupKey;
  const wasLive = new Set<string>();
  const wasArrival = new Set(prev.held.flatMap((h) => (h.kind === "arrival" ? [h.issueId] : [])));
  const pinned = new Map<string, { at: number; row: ShownRow }[]>(); // kept at their old index
  const pinnedIn = new Set<string>();
  const carried = new Map<string, string | null>();                 // unseen causes carried forward
  const changes = new Map<string, Set<string>>();
  const note = (k: string, id: string): void => { changes.set(k, (changes.get(k) ?? new Set()).add(id)); };

  // 1. Walk what the reader sees. Ghosts stay; a vanished row becomes one; a changed row stays at
  //    its index while attention is on its shown group or its home group.
  for (const g of prev.shown) {
    const keep: { at: number; row: ShownRow }[] = [];
    g.rows.forEach((s, at) => {
      if (s.kind === "ghost") { keep.push({ at, row: s }); return; }
      const id = s.row.issueId;
      wasLive.add(id);
      const r = fact.get(id);
      if (r === undefined) {
        keep.push({ at, row: { kind: "ghost", issueId: id, alias: s.row.alias, why: next.left.get(id) ?? "no longer visible" } });
        note(g.key, id);
        return;
      }
      const to = home.get(id)!;
      const changedNow = r.state !== s.row.state || to !== s.home;
      if (changedNow) note(g.key, id);
      const cause = changedNow ? r.cause : s.cause;
      carried.set(id, cause);
      if (watched(g.key) || (to !== g.key && watched(to))) {
        keep.push({ at, row: { kind: "row", row: r, home: to, cause } });
        pinnedIn.add(id);
      }
    });
    pinned.set(g.key, keep);
  }

  // 2. Lay out: a watched group keeps exactly its pinned rows; any other takes next's order with
  //    its pinned rows and ghosts put back at their old index. A new issue is held, never drawn.
  const held: HeldChange[] = [];
  const nextKeys = next.groups.map((g) => g.key);
  const keys = [...nextKeys, ...prev.shown.map((g) => g.key)
    .filter((k) => !nextKeys.includes(k) && (pinned.get(k)?.length ?? 0) > 0)];
  const shown: ShownGroup[] = keys.map((key) => {
    const keep = pinned.get(key) ?? [];
    const rows: ShownRow[] = [];
    let arrivals = 0;
    for (const r of next.groups.find((g) => g.key === key)?.rows ?? []) {
      if (!wasLive.has(r.issueId)) {
        arrivals += 1;
        held.push({ kind: "arrival", issueId: r.issueId, groupKey: key });
        if (!wasArrival.has(r.issueId)) note(key, r.issueId);
        continue;
      }
      if (pinnedIn.has(r.issueId) || watched(key)) continue;          // drawn where it is pinned
      rows.push({ kind: "row", row: r, home: key, cause: carried.get(r.issueId) ?? null });
    }
    for (const k of keep) rows.splice(Math.min(k.at, rows.length), 0, k.row);
    for (const { row } of keep) {
      if (row.kind === "ghost") held.push({ kind: "departure", issueId: row.issueId, groupKey: key, alias: row.alias, why: row.why });
      else if (row.home !== key) held.push({ kind: "move", issueId: row.row.issueId, fromGroup: key, toGroup: row.home, cause: row.cause ?? "" });
    }
    const moving = keep.filter((k) => k.row.kind === "row" && k.row.home !== key).length;
    return { key, rows, arrivals, moving };
  });

  // 3. One flash per update (D M9): rows, unless the group changed more than three times at once.
  const rowFlash = new Set<string>();
  const groupFlash = new Set<string>();
  for (const [k, ids] of changes) {
    if (ids.size > HEADER_FLASH_OVER) groupFlash.add(k);
    else for (const id of ids) if (fact.has(id) && wasLive.has(id)) rowFlash.add(id);
  }
  const flashed = rowFlash.size + groupFlash.size > 0;
  return {
    shown, held,
    flash: flashed ? { seq: prev.flash.seq + 1, rows: rowFlash, groups: groupFlash } : { seq: prev.flash.seq, rows: EMPTY, groups: EMPTY },
  };
}

const PHRASE: Readonly<Record<MoveCause, (m: MoveFact) => string>> = {
  hand: (m) => `set by ${m.byName}`,
  lane_bound: () => "a lane started",
  lane_queued: () => "worktree queued",
  land_trailer: (m) => `landed at ${m.commit.slice(0, 7)}`,
  evidence_observed: (m) => `own evidence at ${m.commit.slice(0, 7)}`,
  evidence_recorded: (m) => `evidence recorded at ${m.commit.slice(0, 7)}`,
};

/** "→ done · own evidence at 7c1f0a2 (test run, exit 0) · 14:02". Printed in Commit Mono. */
export function causeLine(move: MoveFact, timeZone: string): string {
  const hhmm = new Intl.DateTimeFormat("en-GB", { hour: "2-digit", minute: "2-digit", hourCycle: "h23", timeZone })
    .format(new Date(move.at));
  const detail = move.detail === "" ? "" : ` (${move.detail})`;
  return `→ ${move.to.replace("_", " ")} · ${PHRASE[move.cause](move)}${detail} · ${hhmm}`;
}

export function groupSize(group: ShownGroup): string {
  const n = group.rows.filter((r) => r.kind === "row").length;
  return `${n} issue${n === 1 ? "" : "s"}${group.moving > 0 ? ` · ${group.moving} moving` : ""}`;
}

export function heldWords(n: number): string {
  return n === 1 ? "One change is held, so no row moves under you." : `${n} changes are held, so no row moves under you.`;
}
```

#### Interfaces — complete: `desktop/src/components/work/HeldLine.tsx`

```tsx
// desktop/src/components/work/HeldLine.tsx — task 41. The per-group held line and the ghost row.
import { heldWords } from "../../work/hold";

/** At the top of a group with held arrivals. The button applies everything held (the same act
 *  as N); it is a button so a pointer reader has the act too. No count of the group's total. */
export function HeldLine(props: { count: number; onApply(): void }): JSX.Element | null {
  if (props.count === 0) return null;
  return (
    <div className="wk-held" role="note">
      <span className="wk-held-words">{heldWords(props.count)}</span>
      <button type="button" className="wk-held-apply" onClick={props.onApply}>
        Apply <kbd className="wk-kbd">N</kbd>
      </button>
    </div>
  );
}

/** A departure: "EP-128 left this view · cancelled", until N. Not focusable, not selectable:
 *  it is a record, not a row (task 44's selection skips it; task 53's cursor steps over it). */
export function GhostRow(props: { alias: string; why: string }): JSX.Element {
  return (
    <div className="wk-ghost" data-ghost="">
      <span><span className="wk-mono">{props.alias}</span> left this view · {props.why}</span>
      <span className="wk-ghost-hint"><kbd className="wk-kbd">N</kbd> applies</span>
    </div>
  );
}
```

#### Interfaces — complete: `desktop/src/work/listKeys.ts` (new; shared by tasks 41, 42 and 44)

```ts
// desktop/src/work/listKeys.ts — task 41. The one test every list-element key handler runs first,
// until task 53 folds the handlers into KEYMAP. A key typed into a field is the field's.
export function isTextTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  if (target.isContentEditable || target.closest("[contenteditable='true'],[contenteditable='']")) return true;
  return target instanceof HTMLInputElement || target instanceof HTMLTextAreaElement || target instanceof HTMLSelectElement;
}

/** A bare letter or digit on a list: no Ctrl, Meta or Alt, and not in a field. Shift is the
 *  caller's to test, by `event.code` (Global constraints, Keys). */
export function isBareListKey(e: { target: EventTarget | null; ctrlKey: boolean; metaKey: boolean; altKey: boolean }): boolean {
  return !e.ctrlKey && !e.metaKey && !e.altKey && !isTextTarget(e.target);
}
```

#### The edits to task 40's files (props and behaviour; task 40's part owns the rest)

- **`WorkLens.tsx`.** Holds `const [hold, setHold] = useState(() => initialHold(grouped))` and
  `const [attention, setAttention] = useState<Attention>({ groupKey: null })`. An effect on
  `[grouped, attention.groupKey]` runs `setHold((h) => reconcile(h, grouped, attention))`.
  `grouped` must be referentially stable between replica changes (task 40's selector memoises it
  on the replica's version); an unstable `grouped` re-runs the effect every render, and test 9
  below fails on it. The list element gets
  `onKeyDown={(e) => { if (e.code === "KeyN" && !e.shiftKey && isBareListKey(e)) { e.preventDefault(); setHold((h) => applyHeld(h, grouped)); } }}`.
  The lens renders from `hold.shown`, never from `grouped` directly.
- **`CriterionGroup.tsx`** gains `group: ShownGroup`, `flash: boolean` (its key is in
  `hold.flash.groups`), `flashSeq: number`, `onApply(): void` and `onAttention(key: string | null)`.
  It renders `<HeldLine count={group.arrivals} onApply={onApply}/>` first, then each `ShownRow`
  (`GhostRow` for a ghost), and its header's size from `groupSize(group)` in a `.wk-count` span
  with `tabular-nums`. The header carries `data-flash={flash ? flashSeq : undefined}`.
- **`IssueRow.tsx`** gains `cause: string | null`, `flash: boolean` and `flashSeq: number`. It sets
  `data-changed="unseen"` when `cause !== null`, `data-flash` like the header, and renders the
  cause under the title in a `.wk-cause` line in Commit Mono at `--t-fact`. Its `StateGlyph` takes
  `row.state`, the new state, at once.

#### `work.css`, appended (the `.wk-` prefix is task 40's)

```css
/* task 41 — held changes. No layout property animates; the flash is opacity on a pseudo-element. */
.wk-held { display: flex; align-items: center; gap: var(--s-3); min-block-size: var(--row-h);
  padding-inline: var(--s-3); color: var(--ink-2); font-size: var(--t-ui); }
.wk-held-words { flex: 1; }
.wk-held-apply { color: var(--ink); }
.wk-ghost { display: flex; justify-content: space-between; align-items: center; min-block-size: var(--row-h);
  padding-inline: var(--s-3); color: var(--ink-2); font-size: var(--t-ui); }
.wk-cause { font-family: var(--mono); font-size: var(--t-fact); color: var(--ink-2); font-variant-numeric: tabular-nums; }
.wk-row[data-flash], .wk-group-head[data-flash] { position: relative; }
.wk-row[data-flash]::after, .wk-group-head[data-flash]::after {
  content: ""; position: absolute; inset: 0; pointer-events: none; border-radius: inherit;
  background: var(--accent-ring); opacity: 0; animation: wk-flash var(--dur-flash) var(--ease) 1;
}
@keyframes wk-flash { from { opacity: 1; } to { opacity: 0; } }
@media (prefers-reduced-motion: reduce) { .wk-row[data-flash]::after, .wk-group-head[data-flash]::after { animation: none; } }
:root[data-motion="reduce"] .wk-row[data-flash]::after, :root[data-motion="reduce"] .wk-group-head[data-flash]::after { animation: none; }
```

`--accent-ring`, `--ease`, `--ink-2`, `--t-ui` and `--s-3` are the built tokens and task 26's; the
class names `.wk-row` and `.wk-group-head` are task 40's (**UNVERIFIED** until its part exists: its
file was not on the tree at this writing, so the names here are this task's proposal and task 40's
part wins). The reduced-motion rule is written twice, task 36's decision 6.

#### Tests to write first, part 1 — `desktop/src/work/hold.test.ts` (pure)

```ts
import { describe, expect, it } from "vitest";
import { applyHeld, causeLine, initialHold, reconcile, type GroupedIssues, type RowFact } from "./hold";

const R = (issueId: string, alias: string, state: RowFact["state"], cause = ""): RowFact => ({ issueId, alias, state, cause });
const G = (groups: Record<string, RowFact[]>, left: [string, string][] = []): GroupedIssues =>
  ({ groups: Object.entries(groups).map(([key, rows]) => ({ key, rows })), left: new Map(left) });
const ids = (s: ReturnType<typeof initialHold>, key: string) =>
  s.shown.find((g) => g.key === key)!.rows.map((r) => (r.kind === "row" ? r.row.alias : `ghost:${r.alias}`));
const a = R("is-a", "EP-1", "todo"), b = R("is-b", "EP-2", "todo"), c = R("is-c", "EP-3", "todo");
const start = G({ "c-1": [a, b, c], "c-2": [] });
const IN_C1 = { groupKey: "c-1" }, NOWHERE = { groupKey: null };

describe("the hold model", () => {
  it("keeps an arriving issue out of the shown order until applyHeld", () => {
    const next = G({ "c-1": [R("is-n", "EP-9", "todo"), a, b, c], "c-2": [] });
    const s = reconcile(initialHold(start), next, NOWHERE);
    expect(ids(s, "c-1")).toEqual(["EP-1", "EP-2", "EP-3"]);          // mutant: arrivals inserted at once
    expect(s.shown[0].arrivals).toBe(1);
    expect(ids(applyHeld(s, next), "c-1")).toEqual(["EP-9", "EP-1", "EP-2", "EP-3"]);
  });

  it("keeps a re-grouped row at its index, with its new state and cause, while the reader is in its group", () => {
    const moved = R("is-a", "EP-1", "done", "→ done · own evidence at 7c1f0a2 · 14:02");
    const s = reconcile(initialHold(start), G({ "c-1": [b, c], "c-2": [moved] }), IN_C1);
    expect(ids(s, "c-1")).toEqual(["EP-1", "EP-2", "EP-3"]);          // mutant: re-groups on every change
    const row = s.shown[0].rows[0];
    expect(row.kind === "row" && row.row.state).toBe("done");
    expect(row.kind === "row" && row.cause).toBe(moved.cause);
    expect(s.held).toContainEqual({ kind: "move", issueId: "is-a", fromGroup: "c-1", toGroup: "c-2", cause: moved.cause });
    expect(s.shown[0].moving).toBe(1);
  });

  it("relocates a held row without N once the reader's attention leaves both its groups", () => {
    const next = G({ "c-1": [b, c], "c-2": [R("is-a", "EP-1", "done")] });
    const held = reconcile(initialHold(start), next, IN_C1);
    const s = reconcile(held, next, NOWHERE);
    expect(ids(s, "c-1")).toEqual(["EP-2", "EP-3"]);                  // mutant: relocates only on N
    expect(ids(s, "c-2")).toEqual(["EP-1"]);
    expect(s.held.filter((h) => h.kind === "move")).toEqual([]);
  });

  it("turns a departure into a ghost until N", () => {
    const next = G({ "c-1": [a, c], "c-2": [] }, [["is-b", "cancelled"]]);
    const s = reconcile(initialHold(start), next, NOWHERE);
    expect(ids(s, "c-1")).toEqual(["EP-1", "ghost:EP-2", "EP-3"]);    // mutant: drops the row at once
    expect(ids(reconcile(s, next, NOWHERE), "c-1")).toEqual(["EP-1", "ghost:EP-2", "EP-3"]);
    expect(ids(applyHeld(s, next), "c-1")).toEqual(["EP-1", "EP-3"]);
  });

  it("flashes the header once and no row when four changes land in one group at once", () => {
    const d = R("is-d", "EP-4", "todo");
    const from = G({ "c-1": [a, b, c, d] });
    const to = G({ "c-1": [a, b, c, d].map((r) => ({ ...r, state: "started" as const })) });
    const s = reconcile(initialHold(from), to, IN_C1);
    expect([...s.flash.groups]).toEqual(["c-1"]);                     // mutant: per-row flashing
    expect(s.flash.rows.size).toBe(0);
    const three = reconcile(initialHold(start), G({ "c-1": [a, b, c].map((r) => ({ ...r, state: "started" as const })), "c-2": [] }), IN_C1);
    expect(three.flash.rows.size).toBe(3);
    expect(three.flash.groups.size).toBe(0);
  });

  it("formats the cause from SW.4's words and prints no detail it was not given", () => {
    const m = { to: "done" as const, cause: "evidence_observed" as const, at: "2026-09-18T04:02:00Z", byName: "Dylan Example",
      commit: "7c1f0a2e9b3d5c6e8f0a1b2c3d4e5f6a7b8c9d0e", detail: "test run, exit 0" };
    expect(causeLine(m, "Australia/Sydney")).toBe("→ done · own evidence at 7c1f0a2 (test run, exit 0) · 14:02");
    expect(causeLine({ ...m, detail: "" }, "Australia/Sydney")).toBe("→ done · own evidence at 7c1f0a2 · 14:02"); // mutant: invented detail
    expect(causeLine({ ...m, to: "in_review", cause: "lane_queued", commit: "" }, "UTC")).toBe("→ in review · worktree queued · 04:02");
  });
});
```

`2026-09-18T04:02:00Z` is 14:02 in Sydney (AEST, UTC+10; daylight time starts on 4 October 2026),
so the time-zone assertion is fixed, not the machine's zone.

