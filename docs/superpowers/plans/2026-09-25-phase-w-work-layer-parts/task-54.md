### Task 54: keys, part 2: `G`, `O` and `M` sequences, the generated `?` sheet, chords in menus, the safe triangle

Stream **WD1**, about 3 hours (an estimate, not a measurement). Depends on task 53 (`KEYMAP`,
`registerVerb`, `runVerb`, `dispatchListKey`, `rowContextOf`, `ACTIVE_SLICES`,
`DEAD_LINEAR_CHORDS`, `singleKeyShortcuts`, `task-53.md`). Reads task 45's Plan sections
(`plan-triage`, `plan-backlog`, `plan-mine`, `task-45.md:126-133`) and its `surface` /
`setSurface` on task 37's `workSlice` (`task-45.md:29-35`, `:66`); task 48's `AddKind`,
`ADD_ORDER`, `RELATION_LABEL` and `OtherIssuePicker` (`task-48.md:207-222`, `:298-306`); task 46's
`Picker` (`task-46.md:158-200`); task 31's `ViewHeader` (`task-31.md:93-117`). Task 58 registers
`issue.openByNumber` (`task-58.md:244-245`); task 62 registers `go.desk` (its Lens `"desk"`,
`task-62.md:101`). No Hub dependency.

Parity: closes **L-223** (with 53), **L-224**, **L-226**'s menus (the palette's hints are task 59)
and **L-227**'s primitive (decision 8 says why only the primitive). Design: 05 §4.3 (the table,
`05-design.md:994-1013`), §4.4 work rule 5 (`:1060-1064`); 02 C7 `menu.aim` (`02:265`) and C9
(`02:290-305`, the `kbd` hint spec at `:305`).

**What the reader gets** (list focus only, as task 53's rule 1):

| Pressed | What happens |
|---|---|
| `G` then `P` | Plan opens (task 45's surface); the `ViewHeader` showed `G …` between the two keys |
| `G` then `T`, `B` or `M` | Plan opens with Triage, Backlog or *My issues* scrolled to the top and focused |
| `G` then `W` | back to the Work list |
| `G` then `I` | the Desk (task 62 registers it; until then nothing) |
| `G`, a pause past 1.2 s, then `P` | nothing opens; `P` is the priority key again |
| `G` then `V`, or `G` then `O` | nothing in W1 (W2's cycle, W3's Orchestrator); the sheet does not list them |
| `O` then `I` | open by number (task 58's box) |
| `O` then `P` | a project picker beside the view title; choosing one moves the Work lens to it |
| `M` then `B`, `X`, `R` or `D` | on a row: *blocked by*, *blocking*, *related* or *duplicate of which issue*, beside the row |
| `?` | the shortcut sheet, generated from the key map, with the five Linear chords that do nothing here |
| `Esc`, or any key not in the sequence | the pending `G …` is dropped and nothing runs |

**Decisions made here, so nobody re-derives them:**

1. **Sequences are data beside `KEYMAP`**, `SEQUENCES: readonly SequenceBinding[]`, each a prefix
   (`G`, `O`, `M`), a second key, a verb, an optional argument, a scope and a slice. The `?` sheet,
   `chordFor` and task 59's palette read the same two tables, so none can drift from the keys.
2. **One pending prefix, judged lazily.** `feedSequence(event, row)` is the one line task 53
   left for this task in `dispatchListKey`. It keeps `{ prefix, at }` and compares
   `performance.now() - at` against `SEQ_WINDOW_MS = 1200` **when the next key arrives**, so the
   window needs no timer (task 40's acceptance forbids `setTimeout` under `src/work`). 1200 ms is
   a proposal, recorded not measured. A lone modifier keydown (`Shift`, `Control`, `Alt`, `Meta`,
   `CapsLock`) neither completes nor cancels a pending prefix.
3. **A prefix only arms where it can complete.** `G` and `O` arm in `list` scope; `M` arms only
   with a current issue row (its four targets are `issueRow`). `M` with no row, or in W1 `G`
   followed by `V`, is an unknown second key: the prefix is dropped, nothing runs, nothing is
   prevented. An unknown second key is **consumed**, not re-read as a single key, so `G` then `S`
   never opens the state picker by accident.
4. **The hint fades with the window, by CSS alone.** `ViewHeader` renders `<SequenceHint/>`, a
   `kbd` reading `G …`, while a prefix is pending. Its one animation, `seq-window`, takes
   `opacity` from 1 to 0.35 over `--seq-window` (1200ms) and its `animationend` calls
   `expirePending()`: the fade is the window closing, so it explains a cause (DESIGN-SYSTEM's
   motion rule). Under reduced motion the animation is `steps(1, end)`: no visible fade, the same
   end. The keys never read the animation; the timestamp decides (decision 2).
5. **`M` targets are one verb with an argument.** `issue.relate` carries an `AddKind`
   (`blocked_by`, `blocks`, `relates`, `duplicates`). Task 53's `VerbRunner` gains an optional
   second parameter, `arg?: string`, and `runVerb` passes it; every task 53 runner ignores it.
   `WorkLens`'s row pickers (task 53) gain a fifth field, `relation`, rendering task 48's
   `OtherIssuePicker` with that kind; task 48 exports it (it was module-private).
6. **`chordFor(verb)` prints one form everywhere**: a single key as its cap (`S`), `Shift+` as
   `⇧` (`⇧R`, beside the palette's built `⌘⇧M`, `commands.ts:102`), a sequence as its two caps
   (`G P`), and `null` for a verb with no active binding. A verb with a single key and a sequence
   prints the single key.
7. **Chords in menus means the menus that carry verbs.** Task 46's four head-field buttons show
   `S`, `P`, `A`, `L` and set `aria-keyshortcuts`; task 44's bulk bar buttons show theirs; task
   46's `Picker` gains `verb?: VerbId` and prints its chord beside its heading. `DisplayMenu`'s
   items are preferences, not verbs, and show none, rather than an invented one.
8. **The safe triangle is built as the one way a submenu opens, before any menu has one.** A grep
   of the parts finds no W1 submenu. `menuAim.ts` is geometry with no timer (02 C7), written from
   the idea (OD-11), and `Submenu` in `common/Submenu.tsx` is its one consumer shape, so the first
   menu that nests gets the triangle without choosing. L-227 closes on the primitive; its first
   real menu is a gap, named below.

**Files** (row 309 of the skeleton's file map names `sequences.ts` and `ShortcutSheet.tsx`; it
omits `menuAim.ts`, `Submenu.tsx`, `Kbd.tsx`, `goVerbs.ts` and the ⚑ edits, a gap named below):
- `desktop/src/work/keys/sequences.ts` — `SEQUENCES`, the pending prefix, `feedSequence`, `chordFor` (new)
- `desktop/src/work/keys/sequences.test.ts` — the sequence and `chordFor` tests (new)
- `desktop/src/work/keys/goVerbs.ts` — `useGoVerbs()`: `go.work`, `go.plan`, `go.triage`, `go.backlog`, `go.myIssues`, `open.project` (new)
- `desktop/src/components/work/ShortcutSheet.tsx` (+ `ShortcutSheet.test.tsx`) — `sheetModel`, the sheet, `ShortcutSheetMount` (new)
- `desktop/src/components/common/menuAim.ts` (+ `menuAim.test.ts`), `Submenu.tsx`, `Kbd.tsx` (new)
- `desktop/src/work/keys/keymap.ts` (task 53) — `VerbRunner`'s `arg`, the new `VerbId`s, the one line (modified)
- `desktop/src/components/common/ViewHeader.tsx` ⚑ (task 31) — `<SequenceHint/>` after the title (modified)
- `desktop/src/components/AppShell.tsx` ⚑ — `useGoVerbs()` and `<ShortcutSheetMount/>`, once (modified)
- `desktop/src/components/work/WorkLens.tsx` (tasks 40, 53) — the `relation` picker field (modified)
- `desktop/src/components/work/PlanSurface.tsx` (task 45), `desktop/src/store/slices/workSlice.ts` (task 37) — `planSection` (modified)
- `desktop/src/components/issue/Relations.tsx` (task 48) — `export` on `OtherIssuePicker` (modified)
- `desktop/src/components/issue/pickers/Picker.tsx`, `IssueHead.tsx` (task 46), `BulkBar.tsx` (task 44) — chords (modified)
- `desktop/src/components/common/common.css` — `.kbd`, `.seq-hint`, `@keyframes seq-window` (modified)

#### Interfaces — complete: `desktop/src/work/keys/sequences.ts`, part 1: the table

```ts
// desktop/src/work/keys/sequences.ts — task 54. G, O and M sequences (05 §4.3), judged in list
// focus only, because dispatchListKey (task 53) is their only caller. No timer: decision 2.
import { useSyncExternalStore } from "react";
import {
  ACTIVE_SLICES, KEYMAP, runVerb,
  type KeyBinding, type KeyScope, type RowContext, type Slice, type VerbId,
} from "./keymap";

export type Prefix = "G" | "O" | "M";

export interface SequenceBinding {
  prefix: Prefix;
  /** The second key's cap, one letter. */
  key: string;
  verb: VerbId;
  /** Passed to the runner as `arg` (decision 5); only issue.relate uses it. */
  arg?: string;
  scope: KeyScope;
  slice: Slice;
  label: string;
  linear: KeyBinding["linear"];
}

const s = (prefix: Prefix, key: string, verb: VerbId, scope: KeyScope, slice: Slice, label: string,
  linear: KeyBinding["linear"], arg?: string): SequenceBinding =>
  Object.freeze({ prefix, key, verb, scope, slice, label, linear, ...(arg ? { arg } : {}) });

/** 05 §4.3's three sequence rows (`05-design.md:1007`, `:1012-1013`). */
export const SEQUENCES: readonly SequenceBinding[] = Object.freeze([
  s("G", "I", "go.desk", "list", "W1", "Go to the Desk", "match"),
  s("G", "M", "go.myIssues", "list", "W1", "Go to my issues", "match"),
  s("G", "T", "go.triage", "list", "W1", "Go to Triage", "match"),
  s("G", "V", "go.cycle", "list", "W2", "Go to the current cycle", "match"),
  s("G", "B", "go.backlog", "list", "W1", "Go to the backlog", "match"),
  s("G", "W", "go.work", "list", "W1", "Go to Work", "none"),
  s("G", "P", "go.plan", "list", "W1", "Go to Plan", "none"),
  s("G", "O", "go.orchestrator", "list", "W3", "Go to my Orchestrator", "none"),
  s("O", "I", "issue.openByNumber", "list", "W1", "Open an issue by number", "match"),
  s("O", "P", "open.project", "list", "W1", "Open a project", "match"),
  s("O", "C", "open.cycle", "list", "W2", "Open a cycle", "match"),
  s("O", "F", "open.favourite", "list", "W2", "Open a favourite", "match"),
  s("M", "B", "issue.relate", "issueRow", "W1", "Blocked by…", "match", "blocked_by"),
  s("M", "X", "issue.relate", "issueRow", "W1", "Blocking…", "match", "blocks"),
  s("M", "R", "issue.relate", "issueRow", "W1", "Related to…", "match", "relates"),
  s("M", "D", "issue.relate", "issueRow", "W1", "Duplicate of…", "match", "duplicates"),
]);

export const SEQ_WINDOW_MS = 1200;
```

05's `linear` column says **match** for `G` `I` `M` `T` `V` `B` and all four `O` targets; `G W`
and `G P` are ours (Linear has no Work or Plan). `O F`'s favourites are W2 (05 parity row 109,
`05-design.md:243`). The same **UNVERIFIED** note as task 53's `KEYMAP` applies to the column.

#### Interfaces — complete: `sequences.ts`, part 2: the pending prefix and `chordFor`

```ts
let pending: { prefix: Prefix; at: number } | null = null;
const listeners = new Set<() => void>();
const notify = (): void => { for (const l of [...listeners]) l(); };
let clock: () => number = () => performance.now();

const MODIFIER = new Set(["Shift", "Control", "Alt", "Meta", "CapsLock", "AltGraph"]);
const cap = (e: KeyboardEvent): string => (e.key.length === 1 ? e.key.toUpperCase() : e.key);
const live = (b: SequenceBinding, row: RowContext): boolean =>
  ACTIVE_SLICES.has(b.slice) && row.scopes.includes(b.scope);

function setPending(next: typeof pending): void {
  if (pending === next) return;
  pending = next;
  notify();
}

/** Task 53's one line: `if (feedSequence(e, row)) return;`. true means the key was consumed. */
export function feedSequence(e: KeyboardEvent, row: RowContext): boolean {
  if (MODIFIER.has(e.key)) return pending !== null;
  const now = clock();
  if (pending !== null && now - pending.at > SEQ_WINDOW_MS) setPending(null);   // decision 2
  if (pending !== null) {
    const prefix = pending.prefix;
    setPending(null);
    if (e.key === "Escape" || e.shiftKey) return true;                           // decision 3
    const hit = SEQUENCES.find((b) => b.prefix === prefix && b.key === cap(e) && live(b, row));
    if (hit && runVerb(hit.verb, row, hit.arg)) e.preventDefault();
    return true;
  }
  if (e.shiftKey) return false;
  const prefix = cap(e) as Prefix;
  if (!SEQUENCES.some((b) => b.prefix === prefix && live(b, row))) return false;  // decision 3
  setPending({ prefix, at: now });
  e.preventDefault();
  return true;
}

/** The hint's animationend (decision 4). The keys never depend on it. */
export function expirePending(): void {
  setPending(null);
}

export function usePendingPrefix(): Prefix | null {
  return useSyncExternalStore(
    (l) => { listeners.add(l); return () => { listeners.delete(l); }; },
    () => pending?.prefix ?? null,
    () => null,
  );
}

export const printKey = (k: string): string => k.replace(/^Shift\+/, "⇧");

/** Decision 6: one printed form, for menus, the sheet and the palette (task 59). */
export function chordFor(verb: VerbId, arg?: string): string | null {
  const single = activeKeymap().find((b) => b.verb === verb && ACTIVE_SLICES.has(b.slice));
  if (single) return printKey(single.keys[0]);
  const seq = SEQUENCES.find((b) => b.verb === verb && ACTIVE_SLICES.has(b.slice)
    && (arg === undefined || b.arg === arg));
  return seq ? `${seq.prefix} ${seq.key}` : null;
}

let keymapOverride: readonly KeyBinding[] | null = null;
const activeKeymap = (): readonly KeyBinding[] => keymapOverride ?? KEYMAP;

/** Tests only. */
export function __setKeymapForTest(rows: readonly KeyBinding[] | null): void { keymapOverride = rows; }
export function __setSeqClockForTest(now: (() => number) | null): void {
  clock = now ?? (() => performance.now());
}
export function __resetSequencesForTest(): void { pending = null; listeners.clear(); }
```

`chordFor` reads `KEYMAP` through `activeKeymap()` so the drift test can change one binding and
see every menu follow; production never sets the override. A `Shift` second key cancels the
prefix and runs nothing: no sequence target is a Shift chord, and `G` then `Shift+R` must never
start an agent through a half-typed sequence.

#### Interfaces — complete: the deltas on `keymap.ts` (task 53)

```ts
// VerbId gains (task 53's decision 9: later tasks add their ids):
  | "go.desk" | "go.myIssues" | "go.triage" | "go.cycle" | "go.backlog" | "go.work" | "go.plan"
  | "go.orchestrator" | "open.project" | "open.cycle" | "open.favourite"
// VerbRunner and runVerb gain the optional argument (decision 5):
export type VerbRunner = (row: RowContext, arg?: string) => void;
export function runVerb(verb: VerbId, row: RowContext, arg?: string): boolean { /* …as task 53, */
  /* calling stack[stack.length - 1](row, arg) */ }
// registerVerb's wrapper forwards it: const entry: VerbRunner = (row, arg) => run(row, arg);
// dispatchListKey, the line task 53 reserved:
  if (feedSequence(e, row)) return;
```

`keymap.ts` now imports `feedSequence` from `sequences.ts`, which imports from `keymap.ts`: the
same shape of cycle as task 53's, read only at key time. Task 53's `keymap.cycle.test.ts` gains a
third case importing `sequences.ts` first.

#### Interfaces — complete: `desktop/src/work/keys/goVerbs.ts`

```ts
// desktop/src/work/keys/goVerbs.ts — task 54. The G targets this task owns, registered once by
// AppShell so they work from any list in any lens. go.desk is task 62's; go.cycle and
// go.orchestrator are W2's and W3's and are registered by nobody yet.
import { useCockpit } from "../../store/store";
import type { PlanSection } from "../../store/slices/workSlice";
import { useVerb } from "./keymap";

export function useGoVerbs(onOpenProject: (anchor: DOMRect | null) => void): void {
  const go = (surface: "work" | "plan", section: PlanSection | null = null) => () => {
    const s = useCockpit.getState();
    s.setLens("work");
    s.setSurface(surface, section);
  };
  useVerb("go.work", go("work"));
  useVerb("go.plan", go("plan"));
  useVerb("go.triage", go("plan", "triage"));
  useVerb("go.backlog", go("plan", "backlog"));
  useVerb("go.myIssues", go("plan", "mine"));
  useVerb("open.project", () =>
    onOpenProject(document.querySelector(".vh-title")?.getBoundingClientRect() ?? null));
}
```

`onOpenProject` is `AppShell`'s: it opens one `Popover` at that rect holding task 46's `Picker`
labelled *Open a project*, items `state.projects` (id and name), and on choose calls
`selectProject(id)`, the setter the rail's project row already uses (`rail/ProjectRow.tsx:22`,
`:38`), then `setLens("work")`. The anchor is the one `ViewHeader` title on screen
(`task-31.md:102`), so the picker opens where the reader is already looking.

#### Interfaces — complete: the `planSection` delta (tasks 37 and 45)

```ts
// workSlice.ts (task 37), beside task 45's surface. Not persisted, like the rest of the slice.
export type PlanSection = "triage" | "backlog" | "mine";
  planSection: PlanSection | null;
  /** Task 45's setter gains the section; one set, so Plan mounts already knowing it. */
  setSurface(surface: "work" | "plan", section?: PlanSection | null): void;
  // set({ surface, planSection: surface === "plan" ? section ?? null : null })
  clearPlanSection(): void;
```

```tsx
// PlanSurface.tsx (task 45): one effect. The heading ids are task 45's own (`task-45.md:126-133`).
const section = useCockpit((s) => s.planSection);
useEffect(() => {
  if (section === null) return;
  const h = document.getElementById(`plan-${section}`);
  h?.scrollIntoView({ block: "start" });
  h?.closest("section")?.querySelector<HTMLElement>("[data-work-list]")?.focus();
  useCockpit.getState().clearPlanSection();
}, [section]);
```

The focus lands on the section's list, not its heading, so the next `J` moves within Triage.
`scrollIntoView` takes no `behavior: "smooth"`: the jump is the reader's own act, and a smooth
scroll is motion with no cause the reader did not already give.

#### Interfaces — complete: `Kbd.tsx`, `SequenceHint`, and the ⚑ `ViewHeader` line

```tsx
// desktop/src/components/common/Kbd.tsx — task 54. 02 C9's kbd hint (`02:305`).
export function Kbd(props: { chord: string | null }): JSX.Element | null {
  return props.chord === null ? null : <kbd className="kbd" aria-hidden="true">{props.chord}</kbd>;
}

// Same file. The pending prefix, in the one ViewHeader (SW.13).
import { expirePending, usePendingPrefix } from "../../work/keys/sequences";
export function SequenceHint(): JSX.Element | null {
  const prefix = usePendingPrefix();
  if (prefix === null) return null;
  return (
    <kbd className="kbd seq-hint" key={prefix} onAnimationEnd={expirePending}
      aria-label={`${prefix} pressed, waiting for the next key`}>{prefix} …</kbd>
  );
}
```

```tsx
// ViewHeader.tsx (task 31), after <h1 className="vh-title">: the one added line.
      <SequenceHint />
```

`Kbd` is `aria-hidden` because the control beside it carries the chord in `aria-keyshortcuts`
(decision 7); the hint is not hidden, because it is the only place the pending state is said.

#### `common.css` — appended

```css
/* task 54. 02 C9's kbd hint: --t-micro mono (legal only here, task-26.md:102), --inset ground,
   --e-hair ring, --r-chip radius. */
.kbd { font-family: var(--mono); font-size: var(--t-micro); line-height: var(--t-micro-lh);
  padding-inline: var(--s-1); border-radius: var(--r-chip); background: var(--inset);
  box-shadow: var(--e-hair); color: var(--ink-2); font-variant-numeric: tabular-nums; }
.seq-hint { animation: seq-window var(--seq-window) linear forwards; }
@keyframes seq-window { from { opacity: 1; } to { opacity: 0.35; } }
@media (prefers-reduced-motion: reduce) { .seq-hint { animation-timing-function: steps(1, end); } }
:root[data-motion="reduce"] .seq-hint { animation-timing-function: steps(1, end); }
```

`--seq-window: 1200ms` is added to `tokens.css` beside SW.11's durations (`task-26.md:84`), and a
test pins it equal to `SEQ_WINDOW_MS`. `--mono`, `--s-1`, `--ink-2` and `--t-micro-lh` are SW.11's
names (**UNVERIFIED** until task 26 merges; its names win). `data-motion` follows task 44's
reduced-motion pair (`task-44.md:225-226`).

#### Interfaces — complete: `desktop/src/components/work/ShortcutSheet.tsx`, part 1: the model

```ts
// ShortcutSheet.tsx — task 54. GENERATED: every row comes from KEYMAP, SEQUENCES or
// DEAD_LINEAR_CHORDS. Nothing in this file names a key.
import { ACTIVE_SLICES, DEAD_LINEAR_CHORDS, KEYMAP, type KeyScope } from "../../work/keys/keymap";
import { printKey, SEQUENCES, type Prefix } from "../../work/keys/sequences";

export interface SheetRow { keys: string; label: string }
export interface SheetGroup { id: string; title: string; rows: SheetRow[] }
export interface SheetModel {
  groups: SheetGroup[];
  /** The five chords that do nothing here, each with ours (task 53's rule 3). */
  dead: { key: string; linear: string; ours: string }[];
  singleKeysOn: boolean;
}

const SCOPE_TITLE: Readonly<Record<KeyScope, string>> = {
  list: "In any list", everyRow: "On any row", issueRow: "On an issue",
  triage: "In Triage", deskDecision: "On a Desk decision",
};
const PREFIX_TITLE: Readonly<Record<Prefix, string>> = { G: "Go to", O: "Open", M: "Relation, on an issue" };
const SCOPE_ORDER: readonly KeyScope[] = ["list", "everyRow", "issueRow", "triage", "deskDecision"];

export function sheetModel(singleKeysOn: boolean): SheetModel {
  const groups: SheetGroup[] = [];
  for (const scope of SCOPE_ORDER) {
    const rows = KEYMAP.filter((b) => b.scope === scope && ACTIVE_SLICES.has(b.slice))
      .map((b) => ({ keys: b.keys.map(printKey).join(" or "), label: b.label }));
    if (rows.length > 0) groups.push({ id: scope, title: SCOPE_TITLE[scope], rows });
  }
  for (const prefix of ["G", "O", "M"] as const) {
    const rows = SEQUENCES.filter((b) => b.prefix === prefix && ACTIVE_SLICES.has(b.slice))
      .map((b) => ({ keys: `${b.prefix} ${b.key}`, label: b.label }));
    if (rows.length > 0) groups.push({ id: `seq-${prefix}`, title: PREFIX_TITLE[prefix], rows });
  }
  return {
    groups,
    dead: DEAD_LINEAR_CHORDS.map((d) => ({ key: printKey(d.key), linear: d.linear, ours: d.ours })),
    singleKeysOn,
  };
}
```

The sheet prints with `chordFor`'s own `printKey` (decision 6), so a key reads the same in the
sheet, a menu and the palette.

#### Interfaces — complete: `ShortcutSheet.tsx`, part 2: the sheet and its mount

```tsx
export function ShortcutSheet(props: { model: SheetModel; onClose(): void }): JSX.Element {
  const { model, onClose } = props;
  const ref = useDismissable<HTMLDivElement>(true, onClose);   // task 18's: Esc, outside press, focus back
  return (
    <div ref={ref} className="ks" role="dialog" aria-modal="true" aria-labelledby="ks-title">
      <h2 id="ks-title" className="ks-title">Keyboard shortcuts</h2>
      <p className="ks-state">
        Single-key shortcuts are {model.singleKeysOn ? "on" : "off"}. Ctrl chords and the palette always work.
      </p>
      {model.groups.map((g) => (
        <section key={g.id} className="ks-group" aria-labelledby={`ks-${g.id}`}>
          <h3 id={`ks-${g.id}`}>{g.title}</h3>
          <dl>{g.rows.map((r) => (
            <div key={`${r.keys} ${r.label}`} className="ks-row" data-keys={r.keys}>
              <dt><kbd className="kbd">{r.keys}</kbd></dt><dd>{r.label}</dd>
            </div>
          ))}</dl>
        </section>
      ))}
      <p className="ks-space"><kbd className="kbd">Space</kbd> peeks: tap to open, hold to look (05 §4.4 rule 5).</p>
      <section className="ks-group" aria-labelledby="ks-dead">
        <h3 id="ks-dead">Linear keys that do nothing here</h3>
        <dl>{model.dead.map((d) => (
          <div key={d.key} className="ks-row" data-dead={d.key}>
            <dt><kbd className="kbd">{d.key}</kbd></dt><dd>Linear: {d.linear}. Here: {d.ours}.</dd>
          </div>
        ))}</dl>
      </section>
    </div>
  );
}

/** Mounted once by AppShell. Registers keys.sheet; the sheet is a snapshot while open. */
export function ShortcutSheetMount(): JSX.Element | null {
  const [open, setOpen] = useState(false);
  useVerb("keys.sheet", () => setOpen(true));
  const singleKeysOn = useCockpit((s) => s.singleKeyShortcuts);
  if (!open) return null;
  return <ShortcutSheet model={sheetModel(singleKeysOn)} onClose={() => setOpen(false)} />;
}
```

`useDismissable<T>(open, onClose)` returns the ref the overlay's outer element takes
(`hooks/useDismissable.ts:47-50`); it owns Escape, an outside press and focus return, which is why
the map never handles Escape (`useKeyboardMap.ts:14-17`). The sheet's CSS (`.ks*`) sits in
`work.css` with the elevation `Popover.tsx` already uses and `--r-card` (`tokens.css:112`), one
column under 480px and two above; no glass, no gradient.

#### Interfaces — complete: `desktop/src/components/common/menuAim.ts`

```ts
// menuAim.ts — task 54. 02 C7 menu.aim: a submenu stays open while the pointer travels toward
// it inside the triangle (last point, the submenu's near top corner, its near bottom corner).
// Geometry only: no timer, no delay. Written from the idea, not from anyone's code (OD-11).
export interface Pt { x: number; y: number }
export interface Box { left: number; right: number; top: number; bottom: number }

const cross = (o: Pt, a: Pt, b: Pt): number => (a.x - o.x) * (b.y - o.y) - (a.y - o.y) * (b.x - o.x);

/** True when p lies inside or on the triangle a-b-c, whichever way it winds. */
export function inTriangle(p: Pt, a: Pt, b: Pt, c: Pt): boolean {
  const d1 = cross(a, b, p), d2 = cross(b, c, p), d3 = cross(c, a, p);
  const neg = d1 < 0 || d2 < 0 || d3 < 0, pos = d1 > 0 || d2 > 0 || d3 > 0;
  return !(neg && pos);
}

/** The submenu opens to the right of its parent, or to the left when Popover flips it. */
export function aimTriangle(from: Pt, sub: Box, parent: Box): [Pt, Pt, Pt] {
  const x = sub.left >= parent.right - 1 ? sub.left : sub.right;
  return [from, { x, y: sub.top }, { x, y: sub.bottom }];
}

/** Decision: keep the open submenu while `to` is inside the triangle from `from`, or inside the
 *  submenu itself; switch at once otherwise. */
export function keepSubmenu(from: Pt, to: Pt, sub: Box, parent: Box): boolean {
  const insideSub = to.x >= sub.left && to.x <= sub.right && to.y >= sub.top && to.y <= sub.bottom;
  return insideSub || inTriangle(to, ...aimTriangle(from, sub, parent));
}
```
