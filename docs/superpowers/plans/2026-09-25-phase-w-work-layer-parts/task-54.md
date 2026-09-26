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
