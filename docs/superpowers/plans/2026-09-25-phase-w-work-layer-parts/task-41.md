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
```

