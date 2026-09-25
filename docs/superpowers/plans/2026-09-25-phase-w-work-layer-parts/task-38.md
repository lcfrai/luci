### Task 38: prototype, part 1: the Work lens and Plan in the app, behind `flag.compare`, on fixtures

Stream **WD1**, no seam, about 3 hours (an estimate, not a measurement). Depends on task 32
(`lib/flags.ts`, the dev toolbar), task 36 (SW.14, the thread) and task 37 (SW.15, the types and
`workSlice`). It also places task 28's `StateGlyph`, task 29's `AgentGlyph` and `MarkGlyph`, task
31's `ViewHeader`, task 20's `workScenario()` and `expectNoPrediction`, and registers with task 33's
`registerStressScreen`; the skeleton's row names 32, 36 and 37 only (a gap, named in the return).

`OD-1` keeps Phase W's review gate: the owner sees the work surfaces **in the app, in the new look**,
before tasks 40-45 are sized (05 §5.3 row 10; row 11 is "resized after row 10's review"). This is
a prototype, not the product. It reads task 20's fixtures, talks to no Hub and no replica, and lives
in its own dev-only directory, `desktop/src/prototype/work/`, which the skeleton's file map does not
list. Task 40 promotes what the owner kept into `components/work/` and deletes the rest.

**Decisions made here, so nobody re-derives them:**

1. **Three gates, all needed.** The prototype mounts when `lens === "work"`, `import.meta.env.DEV`
   holds, and the new flag `prototype` is `"on"`. The DEV gate is a `lazy()` behind a literal, as
   task 32's `components/devtools/mount.ts` does it, so a production build emits no chunk; the
   flag keeps the prototype out of a developer's way until they ask for it.
2. **How the owner gets there.** `G W` is task 54's. A dev-toolbar button, *Open the Work
   prototype*, sets the flag and calls task 37's `openWork(project, "feat/resumable-uploads")`.
   Inside the prototype one ghost button switches Work and Plan (the prototype's stand-in for
   `G W` and `G P`), so the review needs no keys.
3. **The world is task 20's, with overrides only.** Task 20's rule holds: no builder is declared
   here. `world.ts` takes `workScenario()` and adds, as override objects, what 05 §4.1's drawing
   needs and the scenario does not carry: which criterion each head serves (task 20 gives
   revisions for EP-142 and EP-164 only), EP-167 moved to `started` so the reader has a live
   agent, and one agent line. The drawing's issue numbers are 05's; the prototype uses the world's:

   | 05 §4.1's row | In the prototype | Why this one |
   |---|---|---|
   | EP-142, the reader's issue with `dylan · agent 1` | **EP-167**, `todo` → `started` (override) | the world's EP-142 is already `done` |
   | EP-150, a teammate's, *touching upload/\*.go* | **EP-163**, Alice's, `started` | the world's one teammate issue |
   | EP-139, held, `→ done · own evidence at 7c1f0a2 …` | **EP-162**, `proven_self_attested` | "own evidence" is exactly self-attested |
   | EP-131, settled | **EP-142**, `done`, proven | the world's proven issue |
   | EP-155, *not tied · chore* | **EP-165** (and EP-160, *not tied · imported*) | both not-tied heads |
   | EP-128, the departure ghost | **EP-168**, `cancelled` | the world's cancelled head |

4. **What is shown and what is not.** Cancelled and duplicate heads are not rows (EP-169 is
   absent; EP-168 is its ghost only). The convergence word per group is a fixture value here;
   task 45 computes it (`task-45.md:81`, `Convergence`). Group sizes are plain counts (Q1 A).
5. **Lit and selected, together.** At mount, three rows of the first group are selected through
   `workSlice.select` and the keyboard cursor sits on a row of the second group; focusing that row
   lights its criterion (task 36's `bind`, focus half only, on the rows; the full bind on the
   header). The three selected rows ring and carry a leading `MarkGlyph kind="selected"`; the lit
   group brightens only its gutter.
6. **`moves.hold`, faked honestly.** *Replay a land* moves EP-150 from `in_review` to `landed` in
   the prototype's own state. The list order is frozen at mount, so the row stays, takes its new
   glyph, sets `data-unseen="true"` (task 34's flash or tick) and shows one mono cause line; the
   group header gains `· 1 moving`, and *Apply held changes* re-sorts. Task 41 builds the real
   rule; this only lets the owner see it.
7. **Plan** (05 §4.1, `G P`): Triage, Backlog, every criterion with its issues, and *My issues*.
   No cycles, features board or saved views (W2). Rows are the same row component as Work.
8. **Old and new look, graphite and paper** come from task 32's toolbar for free: the prototype
   writes only tokens and task 30's sans/mono rule, so `data-look` and `data-theme` restyle it.
   Both screens register with task 33's `stress.sets` by name.

**Files** (the skeleton's file map has no row for any of these; a gap, named in the return):
- `desktop/src/prototype/work/world.ts` — the scenario plus its overrides (new)
- `desktop/src/prototype/work/mount.ts` — the DEV gate and the stress registration (new)
- `desktop/src/prototype/work/WorkPrototype.tsx`, `PlanPrototype.tsx` — the two screens (new)
- `desktop/src/prototype/work/Rows.tsx` — the row and the group, shared by both screens (new)
- `desktop/src/prototype/work/Launcher.tsx` — the toolbar's *Open the Work prototype* (new)
- `desktop/src/prototype/work/prototype.css` — owns `.pw-*` (new)
- `desktop/src/prototype/work/prototype.test.tsx` — eight tests (new)
- `desktop/src/lib/flags.ts` ⚑ (task 32) — `prototype: "off" | "on"` (modified)
- `desktop/src/components/devtools/DevToolbar.tsx` ⚑ (task 32) — mounts `Launcher` (modified)
- `desktop/src/components/AppShell.tsx` ⚑ — `LensRouter` gains its `"work"` arm (modified)

#### Interfaces — complete: the delta on ⚑ `lib/flags.ts`

```ts
export type PrototypeFlag = "off" | "on";

export interface Flags {
  look: Look;
  sans: SansCandidate;
  theme: ThemeName;
  /** Task 38: the dev-only Work and Plan prototype. Off is the production DOM. */
  prototype: PrototypeFlag;
}
// FLAG_DEFAULTS gains   prototype: "off",
// ATTRIBUTE gains       prototype: "data-prototype",
```

No stylesheet reads `data-prototype`; the attribute exists because task 32's rule is one attribute
per flag, and `resetFlags` clears it with the others.

#### Interfaces — complete: `desktop/src/prototype/work/mount.ts`

```ts
// desktop/src/prototype/work/mount.ts — task 38. The one import of the prototype from product
// code. Vite replaces import.meta.env.DEV with a literal, so in a production build both branches
// below are dead and no prototype chunk is emitted (the acceptance grep over dist/ holds that).
import { lazy } from "react";
import type { JSX, LazyExoticComponent } from "react";

export const WorkPrototype: LazyExoticComponent<() => JSX.Element> | null = import.meta.env.DEV
  ? lazy(() => import("./WorkPrototype"))
  : null;

if (import.meta.env.DEV) {
  // Registers "Work prototype" and "Plan prototype" with task 33's stress.sets.
  void import("./register").then((m) => m.register());
}
```

`register.ts` (new, six lines, in the same directory):

```ts
import { registerStressScreen } from "../../components/devtools/StressSets";
import WorkPrototype from "./WorkPrototype";
import { PlanPrototype } from "./PlanPrototype";

export function register(): void {
  registerStressScreen("Work prototype", () => <WorkPrototype />);
  registerStressScreen("Plan prototype", () => <PlanPrototype />);
}
```

It holds JSX, so its name is `register.tsx`; the `import("./register")` above resolves it.

#### Interfaces — complete: the delta on ⚑ `components/AppShell.tsx`

```tsx
import { Suspense } from "react";
import { useFlags } from "../lib/flags";
import { WorkPrototype } from "../prototype/work/mount";

// LensRouter, a new arm before `default` (AppShell.tsx:67). Task 40 replaces it with the lens.
    case "work":
      return <WorkArm />;

/** Nothing unless all three gates hold (decision 1). A hook, so it lives in its own component. */
function WorkArm(): JSX.Element {
  const { prototype } = useFlags();
  if (WorkPrototype === null || prototype !== "on") return <></>;
  return (
    <Suspense fallback={null}>
      <WorkPrototype />
    </Suspense>
  );
}
```

#### Interfaces — complete: `Launcher.tsx`, and the delta on ⚑ `DevToolbar.tsx`

```tsx
// desktop/src/prototype/work/Launcher.tsx — task 38, decision 2. Loaded only by the dev toolbar.
import type { JSX } from "react";
import { setFlag } from "../../lib/flags";
import { useCockpit } from "../../store/store";
import { WORK_IDS } from "../../test/workFixtures";

export function Launcher(): JSX.Element {
  const open = () => {
    setFlag("prototype", "on");
    useCockpit.getState().openWork(WORK_IDS.project, "feat/resumable-uploads");
  };
  return <button type="button" className="dt-reset" onClick={open}>Open the Work prototype</button>;
}
```

In `DevToolbar.tsx`, `import { Launcher } from "../../prototype/work/Launcher";` and
`<Launcher />` after `<FlagCompare />`. It borrows task 32's `.dt-reset` look, so the toolbar
gains no class.

