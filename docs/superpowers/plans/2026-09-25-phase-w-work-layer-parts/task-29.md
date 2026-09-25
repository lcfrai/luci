### Task 29: drawn glyphs, part 2: agent shapes, selection, needs-you, `thread.node`; the licence check

Stream **WD1**, completes seam **SW.12** (`<AgentGlyph/>`, `<MarkGlyph/>`), about 3 hours (an
estimate, not a measurement). Depends on task 28 (`paths.ts`, `glyphs.css`, `GlyphSize`). No Hub
dependency.

Task 28 gave issues their shapes. This task gives the other two vocabularies theirs, **agents**
and **marks**, and it fixes a defect in the built rail on the way, because the Work lens and the
rail must speak one vocabulary or a reader learns two.

**The defect.** `StatusDot.tsx:18-27` maps `running` and `idle` both to `dot-disc`. They differ
by colour (`--run` against `--idle`) and by the breath. Under `prefers-reduced-motion` the breath
stops, and the built answer is a static outer halo on `.dot-live` (`common.css:108-123`), which a
reader must learn means *running*. The `tokens.css:100-105` comment records the gap on purpose
("the shape vocabulary does NOT separate idle from RUNNING — both are discs"). 05 §4.4 C7 closes
it with a shape: **running is a disc inside a 1px ring**, so running and idle differ with colour,
motion and the halo all removed.

**The agent vocabulary** (05 §4.4 C7; rule 1), one shape each, colour on top:

| Status | Shape | Colour | Breathes |
|---|---|---|---|
| `starting` | an open ring | `--idle` (ink) | no |
| `running` | a disc inside a 1px ring, a gap between | `--run` | the disc, on the one heartbeat |
| `idle` | a plain disc | `--idle` | no |
| `exited` | a square | `--exited` | no |
| `failed` | a square with its top-right corner cut | `--fail` | no |

`failed` is drawn only where a caller passes it, and a caller passes it only where the wire
carries an **observed** failure (a bound lane failed, 05 §4.4 rule 1). The component never infers
it, and the built wire has no such value (`SessionStatus` is four statuses, `store/types.ts:134`).
Starting is ink, not accent: the accent means *you can act here* (tokens rule 2), and a machine
starting is not something you act on. The built `StatusDot` keeps its `--accent` starting tone;
this task changes only its running shape, so the rail's colour vocabulary is untouched.

**The marks** (05 §4.1, §4.4 rule 4; 02 C2):
- **`selected`** is a leading check, because *lit is not selected* (05 §4.1): a row lit by the
  thread and a row the user selected must not look alike, and a ring is what `thread.lit` uses.
- **`needs_you`** is a shape in `--need`, so an escalation is never amber alone (rule 4; a
  Windows contrast theme repaints amber and must still show the mark).
- **`thread_node`** is the 5px ring at `--e-hair` geometry (1px) in `--thread`, `tokens.css:57`,
  and in `--accent-ring` when `lit` (02 C2 `thread.lit`: lighting is interactive state, so the
  accent is legal). Task 36's `ThreadNode` renders it.

**The breath.** A running glyph's disc breathes on the one heartbeat: `beatDelayStyle()`
(`desktop/src/hooks/heartbeat.ts:23`) on the disc, and the `--beat` period, so a hundred running
glyphs and the rail's dots are at the same point of the same breath. No component starts a timer.
The live part is `aria-hidden`; the glyph's `label` is its accessible text. `StatusDot.tsx:2` says
it is "the ONLY component that may call beatDelayStyle()"; that sentence becomes "StatusDot and
AgentGlyph are the only components…", and nothing else changes in that rule.

**The licence check** (05 §5.3 row 9; 02 C6). W1 adds **no icon family**: every glyph, including
the prototype's `i-*` icons when a later task needs them, is drawn by us. `licence.test.ts` holds
the rest of the dependency tree to `OD-11`'s list: MIT, Apache-2.0, BSD-2-Clause, BSD-3-Clause, ISC
and OFL-1.1, an SPDX `OR` passing if one alternative does and an `AND` only if all do.

**Two corrections to the scope, made here so nobody re-derives them:**

1. **The licence test reads `package-lock.json`, not each dependency's `package.json`.** A
   `node:fs` read fails the typecheck (no `@types/node`; `designsystem.test.ts:8-12`), and a
   dependency's own `package.json` is often outside its `exports` map, so it cannot be imported.
   npm's lockfile (version 3 here) copies each package's `license` field into
   `packages["node_modules/<name>"].license`, and it is imported as `?raw`. All eighteen direct
   dependencies were checked while this plan was written: every one is MIT, Apache-2.0, or an
   `OR` of the two.
2. **Two existing `StatusDot` tests change, and the scope did not say so.** The case table at
   `StatusDot.test.tsx:15-24` pins `["running", "dot-disc"]`, and the test at `:67-90` pins the
   *premise* that running and idle share a shape. Both are rewritten below, not deleted: the first
   pins the new shape, the second pins that the shapes differ.

**Files** (the skeleton's file map omits the three `common/` files and `tokens.css`; a gap, named
in the return):
- `desktop/src/components/glyphs/AgentGlyph.tsx` — `AgentStatus`, `AgentGlyph` (new)
- `desktop/src/components/glyphs/MarkGlyph.tsx` — `MarkKind`, `MarkGlyph` (new)
- `desktop/src/components/glyphs/AgentGlyph.test.tsx`, `MarkGlyph.test.tsx` (new)
- `desktop/src/components/glyphs/licence.test.ts` — the dependency licence check (new)
- `desktop/src/components/glyphs/paths.ts` ⚑ (task 28) — `AGENT_PATHS`, `MARK_PATHS` appended (modified)
- `desktop/src/components/glyphs/glyphs.css` ⚑ (task 28) — `.gl-agent-*`, `.gl-mark-*` appended (modified)
- `desktop/src/components/common/StatusDot.tsx` ⚑ (64 lines; the shape map at `:18-27`,
  `FLASH_MS` at `:29`, the one-caller sentence at `:2`) — `running` → `dot-run` (modified)
- `desktop/src/components/common/common.css` ⚑ (179 lines; `.dot` at `:37-47`, the shapes at
  `:50-62`, the reduced-motion block at `:108-123`) — `.dot-run`, one reduced-motion rule, the
  comment (modified)
- `desktop/src/components/common/StatusDot.test.tsx` ⚑ (90 lines) — two tests rewritten, one added
- `desktop/src/styles/tokens.css` ⚑ — the comment at `:100-105` only; no value changes (modified)

#### Interfaces — complete: `AgentGlyph.tsx`

```tsx
import type { JSX } from "react";
import { beatDelayStyle } from "../../hooks/heartbeat";
import { AGENT_PATHS } from "./paths";
import type { GlyphSize } from "./StateGlyph";
import "./glyphs.css";

/** SW.12. An agent's status as the Work layer shows it (05 §4.4 C7). The built wire carries
 *  starting, running, idle and exited; `failed` is passed only for an OBSERVED failure (a bound
 *  lane failed, 05 §4.4 rule 1) and is never inferred here or by a caller from silence. */
export type AgentStatus = "starting" | "running" | "idle" | "exited" | "failed";

export function AgentGlyph(props: { status: AgentStatus; size?: GlyphSize; label: string }): JSX.Element {
  const { status, size = 16, label } = props;
  const live = status === "running";
  return (
    <svg
      className={`gl gl-${size} gl-agent gl-agent-${status}`}
      width={size}
      height={size}
      viewBox={`0 0 ${size} ${size}`}
      role="img"
      aria-label={label}
      data-glyph={`agent:${status}`}
      focusable="false"
    >
      {AGENT_PATHS[size][status].map((s) => (
        <path
          key={s.d}
          d={s.d}
          className={s.part === "fill" && live ? "gl-agent-core gl-agent-live" : `gl-agent-${s.part}`}
          style={s.part === "fill" && live ? beatDelayStyle() : undefined}
          aria-hidden={s.part === "fill" && live ? true : undefined}
        />
      ))}
    </svg>
  );
}
```

The whole `<svg>` is the image and its `label` is its text; the breathing core carries
`aria-hidden` as well, so no assistive technology reading the tree child by child announces a
pulse. The component calls `beatDelayStyle()` once per render and never subscribes to the
heartbeat, so a running glyph re-renders only when its props change.
