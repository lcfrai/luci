### Task 28: drawn glyphs, part 1: the nine issue states, *done, not proven*, the priority notch

Stream **WD1**, defines `<StateGlyph/>`, the first member of seam **SW.12**, about 3 hours (an
estimate, not a measurement). Depends on task 26 (`--icon`, `--icon-sm`, their strokes and
`--icon-ink`). No Hub dependency: the component takes a state name, not an issue.

A list of issues is read by its left edge. Every row's first mark says where that issue stands, and
a reader scanning forty rows reads forty of them before any title. So the mark has three jobs, each
one a rule a test can hold:

1. **Nine states, ten shapes, told apart by shape alone.** No colour (05 §4.4 work rule 1: issue
   states and agent states never share colour; issue glyphs are neutral ink). A reader who cannot
   tell green from amber, or a Windows contrast theme that repaints everything in `CanvasText`
   (task 34), still reads every state.
2. **Nothing fills as work progresses** (02 C13's not-a-clone test fails a filling status icon).
   `started` does not become a half-disc; the component takes **no progress input at all**, so no
   later caller can pass one. Progress is a fact on the issue page, not a pie on a row.
3. **Priority is a notch, not a second icon** (work rule 2, D I4). A notch cut into the glyph's
   leading edge, deeper from `low` to `urgent`, `none` uncut, so a row carries one glyph. Urgent is
   ink like the rest: `--need` means only *someone is waiting on you*, and an urgent issue earns
   amber by entering NEEDS YOU, not by being urgent.

***Done, not proven*** (05 §3.3) is its own shape: an issue set `done` by hand, without its own
evidence at or after its land. The shape is the `done` check inside a **ring left open**: the work
is marked finished and the loop is not closed. It applies only with `state="done"`; on any other
state `notProven` is ignored, because it cannot mean anything there.

**The drawing, and where it came from.** Every path below was drawn for PanaceaApp from no
third-party source (`OD-11`; no Linear, Plane, Lucide or other icon set was opened). It follows the
sketch in our own `prototype/work-layer.html:393-401` (the `g-triage` … `g-cancel` and `g-dnp`
symbols), with three changes: the prototype has no `duplicate`, drawn here as two overlapping rings;
its `g-dnp` is a dashed ring, which would read as `backlog` with a tick, so the open ring replaces
it; and its `g-review` centre is a filled dot, which is a fill, so a bar replaces it. The 14px and
16px sets are **drawn separately**, each on its own half-pixel grid at its own stroke (1.25px and
1.5px, 02 C6), never one drawing scaled.

**The shape families carry meaning.** A ring is a commitment (`backlog` dashed, `todo`, `started`,
`done`, *done, not proven*, `cancelled`); a rounded square is code that exists in git (`in_review`,
`landed`); a diamond is not yet placed (`triage`); two rings are one thing twice (`duplicate`).
That grouping is a reading aid, not a rule a test holds; the test holds only that all ten differ.

**One correction to the scope, made here so nobody re-derives it:** the notch is a **clip path
with an even-odd hole**, not a mask. A mask needs a luminance fill (`white`/`black`), and
`designsystem.test.ts:639-646` (`namedColours`) exists to keep named colours out of the tree; a
clip path needs no colour at all. Either needs a per-instance id, which comes from React 19's
`useId()` with every character outside `[A-Za-z0-9_-]` removed, so `url(#…)` never has to escape.

**Files:**
- `desktop/src/components/glyphs/StateGlyph.tsx` — `IssueState`, `Priority`, `StateGlyph` (new)
- `desktop/src/components/glyphs/paths.ts` — the path data for both sizes, the notch geometry,
  the labels; headed with its clean-room provenance (new)
- `desktop/src/components/glyphs/glyphs.css` — owns the `.gl-*` prefix and no other (new)
- `desktop/src/components/glyphs/StateGlyph.test.tsx` — the eight tests (new)

No existing file changes. Nothing renders a `StateGlyph` until tasks 40 and 46.

#### Interfaces — complete: `StateGlyph.tsx`, the exported half

```ts
/** SW.12. The nine issue states (05 §3.3). Task 37's desktop issue-state type must be
 *  assignable to this one; its type test holds that, since this task lands first. */
export type IssueState =
  | "triage" | "backlog" | "todo" | "started" | "in_review"
  | "landed" | "done" | "cancelled" | "duplicate";

/** SW.12. The five priorities (L-13). "none" draws no notch. */
export type Priority = "none" | "urgent" | "high" | "medium" | "low";

export type GlyphSize = 14 | 16;

export function StateGlyph(props: {
  state: IssueState;
  priority?: Priority;     // default "none"
  notProven?: boolean;     // honoured only when state === "done"
  size?: GlyphSize;        // default 16; 14 inside rows and chips (02 C6)
}): JSX.Element;
```

There is deliberately no `className`, `style`, `color` or `progress` prop. A caller that wants the
glyph brighter sets `color` on the row (the glyph is `currentColor`); a caller that wants it
coloured is breaking rule 1, and has no prop to do it with.
