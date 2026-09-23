### Task 17: Desktop — `ContextChip`, `DefinitionOfDone`, `ShippedManifest`; the chip on every pane

The thread leaving Discover (prototype state 15). Three **presentational** components, each taking
a `ProvenanceRef` and the one state `selectProvenanceState` computed — **nothing else derived from a
brief** (the old plan's contract for mounting phases, `:5596`, kept). Phase 4 mounts the chip on
every session pane header; phase 3 mounts `DefinitionOfDone`; phase 7 mounts `ShippedManifest`.

**Read `docs/DESIGN-SYSTEM.md` before writing a line of CSS.** The non-negotiables this task is
held to, each with where it bites here:

- **Colour plus shape for every state** (`DESIGN-SYSTEM.md`, "Each carries a shape as well as a
  colour"). Shapes from the fact sheet §10, the one table all three prototypes share: superseded =
  **half-filled disc** in `--drift`; missing = **square** in `--fail`; unavailable = a **`!`**
  mark in neutral ink; none and declined = an **outlined, unfilled** chip with its words. `erased`
  has no shape in the fact sheet: this task draws a **hollow square** in `--ink-3` with the words
  *brief erased* — **a proposal, flagged for the owner's eyes**, not an approved shape.
- **One heartbeat, no spinner.** `resolving` is a static fixed-width bar. Nothing on a chip
  animates except the ~600 ms flash-and-decay when a pane's key changes (a restart that inherited a
  new revision), on `opacity` only, via `hooks/heartbeat.ts`'s house curve — never a second
  `requestAnimationFrame` loop (critique-2 §3.4).
- **Fixed width.** The chip is the same width in all eight states, so `resolving → current` never
  moves a pixel of the header (brief §6). Width is a token (`--chip-prov-w`, in `ch`), the headline
  truncates with an ellipsis.
- **Elevation** of the expanded view is a 1px ring plus layered shadow (`--e-overlay`), radius
  `--r-card`; the chip itself is `--r-chip`. No gradient anywhere; no `transition: all`;
  `transform`/`opacity` only.
- **`rev n` in `var(--numeric)` with `tabular-nums`.** `--ink-4` is never text.
- **No denominator.** Criteria read *open — no evidence recorded yet*; never "0 of 3".
- **Visual acceptance is the owner's eyes**, batched to the end (`OD-12`). Tests prove structure
  and words; they do not prove it looks right.

**Files:**
- `desktop/src/provenance/ContextChip.tsx`, `SessionContextChip.tsx` (the store-reading mount)
- `desktop/src/provenance/DefinitionOfDone.tsx`, `ShippedManifest.tsx`, `ProvenanceDetail.tsx`
- `desktop/src/provenance/provenance.css` (`@import` of `tokens.css` as phase 2's CSS does)
- `desktop/src/provenance/provenance.test.tsx`
- `desktop/src/components/develop/SessionPaneHeader.tsx` — one element, `<SessionContextChip
  sessionId={session.id} />`, after `.pane-sub` (modify, declared departure into task 20's file)
- `desktop/src/styles/tokens.css`, `tokens.test.ts` — `--drift-tint` and `--chip-prov-w` (modify;
  `--drift-tint` is the same token phases 5 and 7 need — if one has landed it, reuse it)

**Interfaces — complete source:**

```ts
// desktop/src/provenance/ContextChip.tsx
import type { ProvenanceRef, Head } from "../api/discover";
import type { ProvenanceState } from "../store/selectors";

export interface ContextChipProps {
  /** The key exactly as the session carries it; shown verbatim, in mono, for missing. */
  keyText: string;
  state: ProvenanceState;
  refValue: ProvenanceRef | undefined; // undefined while resolving or unavailable
  head: Head | undefined;              // for "brief now at rev n"
  /** One click: Discover at exactly this revision, read-only (contract §4.1). Offered for
   *  current, superseded and missing. */
  onOpenDiscover(key: string): void;
  /** superseded only: restart the session so it inherits the latest complete revision. */
  onRestart?(): void;
}
export function ContextChip(props: ContextChipProps): JSX.Element;
```

```ts
// desktop/src/provenance/SessionContextChip.tsx — the ONLY store-reading provenance component
export function SessionContextChip(props: { sessionId: SessionId }): JSX.Element;
// reads selectSessionRefState(sessionId) with useShallow; wires onOpenDiscover to
// discoverSlice.openPast and onRestart to phase 2's restart verb (components/develop/sessionVerbs.ts)
```

```ts
// desktop/src/provenance/DefinitionOfDone.tsx — mounted by phase 3 above the merge list
export interface DefinitionOfDoneProps {
  refValue: ProvenanceRef;             // the HEAD reading (contract §4.4)
  state: ProvenanceState;
  onOpenDiscover(key: string): void;
  onRecordEvidence?(criterionId: string): void; // phase 3 ask P4-5
}
export function DefinitionOfDone(props: DefinitionOfDoneProps): JSX.Element;
```

```ts
// desktop/src/provenance/ShippedManifest.tsx — mounted by phase 7 in Deploy
export interface ShippedManifestProps {
  refValue: ProvenanceRef;              // the PINNED ref, never the head (P1)
  state: ProvenanceState;
  head: Head | undefined;
  version: string;                      // phase 7's, e.g. "v1.6.0"
  mergeRef?: ProvenanceRef;             // phase 3's merge link, shown BESIDE the pin (contract §3.4)
  designKey?: string;                   // phase 5's, shown verbatim
  onOpenDiscover(key: string): void;
}
export function ShippedManifest(props: ShippedManifestProps): JSX.Element;
```

**Words, per state** (patterns from contract §4.1 and the prototype; final copy is the owner's):

| State | Chip | Expanded / manifest first line |
|---|---|---|
| current | headline, `rev 5` | *v1.6.0 shipped against brief bf-…a2, revision 6* |
| superseded | headline, `rev 5 · brief now at rev 6` | adds *the brief has since moved to revision 6*, and *Restart to inherit rev 6* on a session |
| none | *no brief* | *no brief is recorded for this release* (manifest) |
| declined | *started without the brief* | *This session was started without its brief, by choice.* |
| missing | the key in mono, *not found* | *Revision not found — this is data loss in our store, not a missing brief.* |
| erased | *brief erased* | *This brief was erased on <date>. Its key and digest remain.* |
| resolving | a static bar | — |
| unavailable | `!` *could not check the brief* | the call site's sentence |

**Which state a Hub outage reaches (review I6).** The `missing` sentence is shown only for a slot
the Hub itself answered as missing: task 07 rule 5 makes a Hub that is off (`hub_off`),
unreachable, failing or signed out give `unavailable` for every uncached slot, never `missing`,
and task 12 serves that as a 200. So no chip here says *data loss* because the Hub could not be
asked. Checked for this batch and **not** changed: the sentence itself, which is contract §4.1's.
The Hub's `missing` also covers *a revision the caller cannot read* (Hub contract §7.5,
indistinguishable on purpose), which is not data loss either; whether the words should say so is
copy, and final copy is the owner's (the header of the table above). It is carried as an owner question in the fix-batch b05
record of the review.

**Tests first** (`provenance.test.tsx`, Testing Library, fixtures from the fact sheet):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `every state renders its words and shape` — table of 8 | text per the table; a shape element with `data-shape="half-disc" \| "square" \| "hollow-square" \| "bang"` where the fact sheet gives one | colour as the only signal |
| `none is shown, never hidden` | `state: "none"` renders a visible element with *no brief* | P9 |
| `the chip root is the same element and class in every state` | one root, class `prov-chip`, no state changes its tag or width class | a layout shift on resolve |
| `resolving has no spinner` | no `role="progressbar"`, no element with an `animation` class | a spinner per pane |
| `no denominator anywhere` | rendered text of all three components never matches `/\b\d+\s*(of\|\/)\s*\d+\b/` | "2 of 4 met" |
| `criteria read open with no evidence` | each criterion *open — no evidence recorded yet* | an implied verdict (P12) |
| `the manifest leads with the pin and shows the head beside it` | pinned rev 5, head latest-complete 6 → first line names revision 5; a second line names 6 | P1: a release painted with the head |
| `the manifest shows disagreement as disagreement` | `mergeRef` at rev 5 beside a pin at 6 → both revisions visible, neither hidden | a silent reconciliation (contract §3.4) |
| `open in Discover passes the exact key` | clicking calls `onOpenDiscover("brief:bf-…a2@5")` | a chip opening the head |
| `SessionContextChip reads contextRef, not the branch` | two sessions on one branch at `@5` and `@6` render different chips | critique §1.3 |
| `SessionPaneHeader carries the chip in both variants` | pane and tile headers each contain one `.prov-chip` | a chip only in Columns |
| `the stylesheet obeys the design system` | `provenance.css` has no hex literal, no `-gradient(`, no `transition: all`, no `color: var(--ink-4)` | the phase 2 audit failing at merge |

**Steps:**

- [ ] **Step 1:** Read `DESIGN-SYSTEM.md`, `styles/tokens.css`, `components/common/common.css`
  (its status-dot shapes) and `components/common/Popover.tsx` (the expanded view uses it).
- [ ] **Step 2:** Write the tests; confirm they fail.
- [ ] **Step 3:** The components, the CSS, the two tokens (and their `tokens.test.ts` lines), the
  header mount.
- [ ] **Step 4:** Run the acceptance commands; commit by pathspec. **Do not merge this branch** —
  it waits for the owner (`OD-1`).

**Acceptance:**

```
$ cd desktop && npx vitest run && npx tsc --noEmit
(all pass; tsc silent)
$ grep -nE "#[0-9a-fA-F]{3,8}\b|-gradient\(|transition:\s*all" desktop/src/provenance/provenance.css
(no output)
```

Owner-observed, **unverified until the batched session** (`OD-12`): the eight states side by side
in a real pane header, the fixed width, the erased shape proposal.

**Does NOT claim:** Merge's or Deploy's layouts (phases 3 and 7 mount these where they choose).
