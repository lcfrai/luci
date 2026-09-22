### Task 21: Canvas layout and persisted tiles

Columns is the layout you read in; Canvas is the layout you arrange in. Task 20 shipped the first one and the pane that fills it, and this task ships the second: a 2400×1600 scrollable plane, a tile per pinned session, pointer-driven drag and resize, z-order, and a position that survives both a flip back to Columns and a restart of the app. That last clause is the whole point — it is known defect 6 (contract §8), the prototype's canvas kept `x`/`y` on a JavaScript object that died with the page, and the regression test named in the defect table, *"a dragged tile writes its position to the store"*, is written here in step 2.

**This task modifies three files task 20 created**, which Group D's shared-shape document calls the exact shape that produced Group C's worst defect, so its three rules govern the plan and are honoured explicitly. Rule 1 is task 20's, and it reserved a second point as well as the obvious one. The first is in `DevelopLens.tsx`: a `TASK 21` comment carrying the string `<CanvasLayout` deliberately mid-line. The second is in `ColumnsLayout.test.tsx`, where `it("renders Columns for the canvas layout too, until task 21 lands")` asserts `.cols-scroll` renders under `layout: "canvas"` and the comment beside it says task 21 replaces that expectation. **The second reservation is the one an author skips**, because it sits in another task's test file rather than in its source — and the moment `DevelopLens` returns `<CanvasLayout/>` that assertion is false. A plan that leaves it hands the implementer a red suite whose obvious reading is "my edit broke Columns", and acting on that reading means reverting the wiring: the component-that-exists-only-in-its-own-tests failure rule 3 exists to catch. Step 4 repoints it, in the same step as the wiring. Rule 2 is this task's and it fixes the step order: **step 4 creates `CanvasLayout.tsx`, edits `DevelopLens.tsx` and repoints task 20's expectation in one step**, because a branch that calls a component defined two steps later leaves the tree uncompilable in between and makes every claimed-green run between them impossible. Rule 3 is step 7's anchored guard, `grep -cE '^[[:space:]]*<CanvasLayout'`, which reads **0** before the change and **1** after. The shared shape's naive form reads 2 where the reserving comment survives the consumer's edit; here step 4 replaces that comment, so the naive form reads **1** before and **1** after and cannot tell the wired state from the reserved one at all, which is the same argument for anchoring one step further on. Shared-shape §2 splits the stylesheet by prefix: task 20 owns `.pane-*` and `.cols-*`, this task owns `.cv-*` and appends under a header at the foot of the file without reflowing a rule above it.

**The one thing this task must not get wrong is who owns the tile's geometry.** The obvious implementation puts `transform`, `width` and `height` in the tile's JSX `style` prop and writes the store on every `pointermove`. Both halves are wrong and they fail together. A store write per frame runs the whole `persist` middleware — a JSON serialise of the cockpit into `localStorage` sixty times a second — and it re-renders every pane on the plane while the user is mid-gesture. And once React owns `transform`, any unrelated re-render during the drag rewrites it from the last *committed* position: the 2 s poller lands, the tile jumps back to where the pointer was 400 ms ago, and it looks like a physics bug rather than a data-flow one. So: **React's `style` prop carries `zIndex` and nothing else.** The committed position is applied by a `useLayoutEffect` keyed on the four numbers, the drag writes `el.style.transform` directly through a ref, and `setTile` is called exactly once, on `pointerup`. React never learns about the property, so React can never stomp it.

Motion follows `docs/DESIGN-SYSTEM.md`: position is `transform: translate3d(...)`, never `left`/`top`, so a drag composites instead of forcing layout on every pointer sample; and `develop.css` declares no `transition` at all, which is task 20's stance in this file and is also the only honest one — a tile that eases toward the pointer is a tile that is behind the pointer. Size is the documented departure: resize writes `width` and `height`, because `transform: scale` would blur xterm's glyph grid and would not change the PTY's cols and rows. Direct manipulation at pointer rate is not an animation, and nothing here is transitioned. Shared-shape §3B binds the same block twice more: no colour function outside the token layer, and **no gradient anywhere** — task 24's audit greps every stylesheet for `linear|radial|conic-gradient(`. An earlier draft of this plan shipped two, a `radial-gradient` dot grid on the plane and a `linear-gradient` corner on the resize handle, and both are gone: the plane is a flat `var(--canvas)` and the corner is an inset ring.

**Files:**
- `desktop/src/components/develop/CanvasLayout.tsx` — the plane, the tile, drag, resize, z-order and the write-through to `prefsSlice` (new)
- `desktop/src/components/develop/CanvasLayout.test.tsx` — thirteen tests, including defect 6's named regression and the `DevelopLens` wiring test (new)
- `desktop/src/components/develop/DevelopLens.tsx` — the reserved Canvas branch, replaced (modify, task 20's file)
- `desktop/src/components/develop/develop.css` — the `.cv-*` block appended under its header (modify, task 20's file)
- `desktop/src/components/develop/ColumnsLayout.test.tsx` — task 20's interim canvas expectation, repointed from `.cols-scroll` to the canvas plane (modify, task 20's file)

**Interfaces:**
- Produces:
  - `components/develop/CanvasLayout.CanvasLayout` — `{ sessionIds: SessionId[] }`, exactly contract §6.5. Owns the plane, tile drag, tile resize, z-order and writing `TilePosition` to the store
  - The DOM contract its own tests query, and task 20's repointed test with them: `[data-testid="cv-plane"]` on the scroll container, `[data-cv-tile="<sessionId>"]` on each tile, `.cv-grip` and `.cv-resize` on the two handles, and `data-cv-dragging="none" | "move" | "resize"` on a tile mid-gesture. Task 24's defect-6 entry does **not** query any of them: it names `CanvasLayout.test.tsx` and `store.test.ts` as the tests, and anchors on `tiles` in `prefsSlice.ts` and `partialize` in `store.ts`. So these attribute names are this task's to change and the persistence underneath them is not
- Consumes: `SessionPane` and `develop.css` (task 20); `useCockpit`, `resetCockpit` (task 17's `store/store.ts`); `selectPaneSessionIds`, `selectCanvasSessionIds` (task 17's `store/selectors.ts`); `prefsSlice.tiles`, `prefsSlice.pinned`, `setTile`, `pinSession`, `CANVAS_MAX_TILES` (task 17's `store/slices/prefsSlice.ts`); `TilePosition`, `SessionId`, `Session` (task 15's `store/types.ts`); `aProject`, `aBranch`, `aWorktree`, `aSession` (task 14's `test/fixtures.ts`, typed and filled by **task 15** in the insertion point task 14 reserves — shared-shape §3.1. This task declares no builder of its own and passes override objects, per §3.2)
- Required by: task 23 (`⌘1`–`⌘9` focus a session that may be on a tile rather than in a column), task 24 (the defect suite asserts defect 6 end to end through `prefsSlice`'s `tiles` and the store's `partialize`, and the design-system audit greps `develop.css` for literal hex, any colour function, any gradient, `transition: all` and the banned hues)
- Adds no selector and no store action. Every read it needs is already in contract §3.7 and §3.4, which is why this task touches no file outside `components/develop/`

**The decisions below are where the contract is silent, and each names its own departure where it is not.** (b) departs from §6.5, which makes the pane header the tile's drag handle; (c) defers the zoom §6.8 assigns to this component; (a) reads §3.4 further than its literal text goes. The resize that writes `width` and `height`, argued above, departs from `docs/DESIGN-SYSTEM.md`'s motion rule rather than from the contract. The commit message names the same departures, and neither place counts them — a count here would have to rule on whether reading §3.4 past its literal text is a departure or a gap, and nothing downstream needs that ruling.

**(a) `CanvasLayout` seeds the pinned set from the Columns order, capped.** Contract §3.7 makes the canvas render `pinned` and §3.4 makes `pinSession` the only way in — but **no component in Group D calls it**: §6.5's `SessionMenu` verb list has no "Pin to canvas", and neither does task 22's row in §9. Rendered literally, the canvas is blank for ever and the layout toggle looks broken. So an effect here pins pane sessions that are not pinned yet, up to the room left under `CANVAS_MAX_TILES`. It computes the room first rather than calling `pinSession` and letting it no-op, because §7.7 says the no-op *toasts the cap*, and a toast on every mount of the canvas would be noise the user cannot act on. If a reviewer would rather have an explicit pin verb, it is one entry in task 22's menu plus one call here.

**(b) The tile carries its own grip, and the grip is the drag handle — a departure from §6.5**, which says the pane header is the handle in `tile` variant. `SessionPaneHeader.tsx` is task 20's file and §9's row 21 does not list it, so honouring §6.5 would mean either editing a file outside this task's row or matching on task 20's class names from inside my pointer handler, and shared-shape §1 exists to stop exactly that. The 15px grip is mine, its cursor rule is in my prefix, and the header keeps its verbs unambiguously clickable. Row 21 gives this task `CanvasLayout.tsx` and its test, plus `develop.css` and `DevelopLens.tsx` to modify, and no other source file; this is the one place it and §6.5 disagree. Step 4 does edit one file row 21 also omits — task 20's `ColumnsLayout.test.tsx` — and that is the opposite case rather than a second exception: task 20 wrote that expectation *for this task to repoint*, which shared-shape §1 rule 1 puts here, while `SessionPaneHeader.tsx` reserves nothing and would be edited on this task's own initiative.

**(c) No grid snap, no zoom, and no dot grid.** The prototype snapped to nothing, and hand placement on a dark plane reads as intentional without a lattice. The 24px dot grid an earlier draft drew for texture is gone too, and not for taste: it was a `radial-gradient`, shared-shape §3B bans gradients outright and task 24's audit greps for them, so the plane is a flat `var(--canvas)` — the token whose own comment calls it "the Canvas backdrop". Pan is native `overflow: auto` scrolling. §6.8 assigns "Canvas pan/zoom" to this component, and zoom is deferred rather than faked — a zoomed plane needs a transform on the sheet that every pointer delta must then divide by, and there is no requirement asking for it.

**(d) Z-order is component state, not persisted.** §2's `TilePosition` is exactly `{x, y, w, h}`, and adding a fifth field would change the shape task 17 persists at schema version 1. Stacking order is cheap to re-derive (pin order) and nobody misses it across a restart.

**One accessibility limit, stated rather than hidden.** Drag and resize are pointer-only; there is no keyboard equivalent in Phase 2. Both handles are `tabIndex={-1}` so they are not tab stops that do nothing, both carry an accessible name (the last test of the canvas suite pins it), and Columns — reachable from the same toggle, and the layout `⌘1`–`⌘9` indexes — remains the keyboard-complete surface.

- [ ] **Step 1: Preflight — confirm what tasks 17 and 20 left, and do not patch it**

Symbols from tasks 20, 17 and 15 are imported or edited by this one. Every one is a compile error rather than a test failure if it is missing, which sounds safe until you notice that a missing symbol makes this task's author reach into another task's file — shared-shape §1 is explicit that a silent local patch is how Group C produced nine Criticals. **If any check below fails, stop and report it to the orchestrator.**

```bash
cd desktop
# (a) task 20 — the files this task modifies, and the points it reserved in them
grep -cE "^export function DevelopLens" src/components/develop/DevelopLens.tsx   # expect 1
grep -cE "^export function SessionPane" src/components/develop/SessionPane.tsx   # expect 1
grep -c "TASK 21" src/components/develop/DevelopLens.tsx                         # expect 1
grep -c "until task 21 lands" src/components/develop/ColumnsLayout.test.tsx      # expect 1
grep -cE "^[[:space:]]*<CanvasLayout" src/components/develop/DevelopLens.tsx     # expect 0
grep -cE "^\.cv-" src/components/develop/develop.css                             # expect 0
grep -cE '^@import "(\.\./)+styles/tokens\.css";' src/components/develop/develop.css  # expect 1
grep -cE "^\.(pane|cols)-" src/components/develop/develop.css                    # RECORD the number
# (b) task 17 — the prefs this task writes through, and the two orderings it reads
grep -cE "^export const CANVAS_MAX_TILES" src/store/slices/prefsSlice.ts         # expect 1
grep -c "setTile" src/store/slices/prefsSlice.ts                                 # expect 2 or more
grep -c "pinSession" src/store/slices/prefsSlice.ts                              # expect 2 or more
grep -cE "^export const selectCanvasSessionIds" src/store/selectors.ts           # expect 1
grep -cE "^export const selectPaneSessionIds" src/store/selectors.ts             # expect 1
# (c) task 15 — the six fixture builders, used with overrides and never re-declared
grep -cE "^export function aSession" src/test/fixtures.ts                        # expect 1
```

Notes on what these checks mean. The second reservation is checked too, and it is checked by its prose rather than by an anchor, because a test title is prose: `until task 21 lands` must read **1** now and **0** after step 4, which is the same before/after shape as the `<CanvasLayout` guard and catches the case where task 20's fix wave moved or renamed that test. **Both `expect 0` checks are anchored on purpose** and they are the ones with teeth: task 20's reservation comment contains the string `<CanvasLayout` mid-line, and task 20 writes `.cv-*` in its own prose twice — once in the prefix-ownership paragraph of the file's top header and once in the reserved Canvas header at its foot — so the unanchored forms return **1** and **2** and tell you the work is already done. They must read 0 and 0 now, and step 7 re-runs the first of them expecting 1.

The last two checks in group (a) are not about this task's own code. The token import is **task 20's line 1** — shared-shape §3B makes `@import "../../styles/tokens.css";` the first line of every stylesheet and task 24 asserts it, so if this reads 0, report it and do **not** prepend it here: prepending is precisely the reflow §2 forbids, and the fix belongs to the file's owner. The `.pane-|.cols-` count is not an assertion at all — write the number down. Step 7 re-runs it and it must be unchanged, because this task appends and never reflows. It reads 27 against task 20's plan as that plan is written today; take whatever the tree actually gives you and compare against that, not against 27.

`CANVAS_MAX_TILES` is declared in contract §3.4's `prefsSlice` block, so that is where the grep looks. If task 17 re-exported it from `store/store.ts` or parked it in `store/types.ts` instead, import it from wherever it actually lives and change nothing else — it is a `const`, not a decision.

And nothing here checks `CanvasLayout`, which must not exist yet: this task creates it in step 4. A hit means another agent has been in this directory; stop and report rather than overwriting.

- [ ] **Step 2: Write the failing test for the plane, the drag and the store write**

Thirteen tests in one file, and that number is load-bearing: step 6 checks the run against it. Some are the task itself — position in, position out, clamped, persisted — and the rest are the failure modes this component has in it: a store write per frame, a listener that outlives its gesture, a tile that renders from a rehydrated pin list longer than the cap, and a component that exists only in its own tests.

The decisions below are worth their paragraphs, because each is a test that silently passes when written the obvious way.

**`SessionPane` is mocked.** It is task 20's component with task 20's own suite, it constructs a real `<Terminal/>`, and §1.5 is explicit that Phase 2 does not put xterm under jsdom. Mocking it also makes `variant="tile"` assertable in one line rather than inferred from rendered chrome. `../../api` is mocked for the same reason task 20 mocks it: `DevelopLens`'s module graph reaches it through `EmptyDevelop`, and this suite is not the place to discover that.

**Pointer events are constructed as `MouseEvent`s carrying the pointer type name.** jsdom's `PointerEvent` support varies by version, and `@testing-library/dom`'s `createEvent` falls back to `window.Event` when the constructor named by the event map is absent — an `Event` has no `clientX`, so `fireEvent.pointerDown(el, { clientX: 100 })` can dispatch an event whose coordinate is `undefined`, the handler computes `NaN`, and the assertion on the transform fails in a way that reads as a component bug. A `MouseEvent` always carries the coordinates, React's `onPointerDown` dispatches on the type name rather than the constructor, and `PointerEvent extends MouseEvent` so the production handler's parameter type accepts both. The helper removes the whole question.

**The mid-drag assertion is the point of *a dragged tile writes its position to the store*, not the post-drag one.** `expect(...tiles["s-1"]).toBeUndefined()` after a `pointermove` is what fails if someone moves the `setTile` call into the move handler; the `toEqual` after `pointerup` passes either way.

Create `desktop/src/components/develop/CanvasLayout.test.tsx`:

```tsx
import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { CanvasLayout } from "./CanvasLayout";
import { DevelopLens } from "./DevelopLens";
import { resetCockpit, useCockpit } from "../../store/store";
import { aBranch, aProject, aSession, aWorktree } from "../../test/fixtures";
import type { CockpitState } from "../../store/store";
import type { Session, SessionId } from "../../store/types";

// SessionPane is task 20's, with task 20's suite. It builds a real <Terminal/>, and §1.5 keeps
// xterm out of jsdom. The stub also makes `variant` assertable directly.
vi.mock("./SessionPane", () => ({
  SessionPane: ({ sessionId, variant }: { sessionId: string; variant: string }) => (
    <div data-testid="pane" data-session={sessionId} data-variant={variant} />
  ),
}));

// DevelopLens reaches ../../api through EmptyDevelop. Same mock task 20 uses.
vi.mock("../../api", () => ({ api: { createSession: vi.fn() } }));

const project = aProject({ id: "p-1", name: "engagement-portal", path: "C:\\work\\portal" });
const branch = aBranch({ id: "b-1", projectId: "p-1", name: "evidence-upload", isHead: false });
const worktree = aWorktree({
  id: "w-1",
  projectId: "p-1",
  branchId: "b-1",
  name: "wt-evidence",
  path: "C:\\work\\wt\\evidence",
  sessionCount: 3,
});
const base = { projectId: "p-1", branchId: "b-1", worktreeId: "w-1", harness: "claude-code" };

const ids = (n: number): SessionId[] => Array.from({ length: n }, (_, i) => `s-${i + 1}`);

/** Seed `n` sessions on one worktree, all pinned, canvas layout, no stored tiles. */
function seed(n: number, over: Partial<CockpitState> = {}): SessionId[] {
  const list = ids(n);
  const sessions: Record<SessionId, Session> = {};
  for (const id of list) sessions[id] = aSession({ ...base, id, title: `task ${id}` });
  resetCockpit();
  useCockpit.setState({
    projects: [project],
    branchesByProject: { "p-1": [branch] },
    worktreesByBranch: { "b-1": [worktree] },
    sessions,
    sessionIdsByWorktree: { "w-1": list },
    selection: { projectId: "p-1", branchId: "b-1", worktreeId: "w-1", sessionId: null },
    layout: "canvas",
    pinned: list,
    tiles: {},
    ...over,
  });
  return list;
}

/** jsdom's PointerEvent support is version-dependent and fireEvent silently degrades to a bare
 *  Event when it is absent, which drops clientX/clientY. A MouseEvent named for the pointer type
 *  always carries them, and React dispatches on the type name. */
function pointer(type: "pointerdown" | "pointermove" | "pointerup", x: number, y: number): MouseEvent {
  return new MouseEvent(type, { bubbles: true, cancelable: true, clientX: x, clientY: y, button: 0 });
}

function tileEl(id: SessionId): HTMLElement {
  const el = document.querySelector<HTMLElement>(`[data-cv-tile="${id}"]`);
  if (el === null) throw new Error(`no tile rendered for ${id}`);
  return el;
}

function handleEl(id: SessionId, which: "cv-grip" | "cv-resize"): HTMLElement {
  const el = tileEl(id).querySelector<HTMLElement>(`.${which}`);
  if (el === null) throw new Error(`no .${which} inside the tile for ${id}`);
  return el;
}

/** One complete gesture: down on the handle, one move, one up. */
function gesture(
  handle: HTMLElement,
  from: readonly [number, number],
  to: readonly [number, number],
): void {
  fireEvent(handle, pointer("pointerdown", from[0], from[1]));
  fireEvent(window, pointer("pointermove", to[0], to[1]));
  fireEvent(window, pointer("pointerup", to[0], to[1]));
}

describe("CanvasLayout", () => {
  beforeEach(() => {
    resetCockpit();
  });

  it("renders one tile per pinned session, in pin order, each as a tile-variant pane", () => {
    seed(3);
    render(<CanvasLayout sessionIds={["s-1", "s-2", "s-3"]} />);

    const panes = screen.getAllByTestId("pane");
    expect(panes.map((p) => p.getAttribute("data-session"))).toEqual(["s-1", "s-2", "s-3"]);
    expect(panes.map((p) => p.getAttribute("data-variant"))).toEqual(["tile", "tile", "tile"]);
  });

  it("sizes the sheet to the 2400x1600 plane", () => {
    seed(1);
    const { container } = render(<CanvasLayout sessionIds={["s-1"]} />);

    const sheet = container.querySelector<HTMLElement>(".cv-sheet");
    expect(sheet?.style.width).toBe("2400px");
    expect(sheet?.style.height).toBe("1600px");
  });

  it("gives a session with no stored tile a default lattice position", () => {
    seed(2);
    render(<CanvasLayout sessionIds={["s-1", "s-2"]} />);

    expect(tileEl("s-1").style.transform).toBe("translate3d(32px, 28px, 0)");
    expect(tileEl("s-2").style.transform).toBe("translate3d(520px, 28px, 0)");
    expect(tileEl("s-1").style.width).toBe("460px");
    expect(tileEl("s-1").style.height).toBe("320px");
  });
```

Same file, continuing inside the `describe`:

```tsx
  it("applies a stored position as a transform, never as left or top", () => {
    seed(1, { tiles: { "s-1": { x: 700, y: 400, w: 520, h: 360 } } });
    render(<CanvasLayout sessionIds={["s-1"]} />);

    const el = tileEl("s-1");
    expect(el.style.transform).toBe("translate3d(700px, 400px, 0)");
    expect(el.style.width).toBe("520px");
    expect(el.style.height).toBe("360px");
    expect(el.style.left).toBe("");
    expect(el.style.top).toBe("");
  });

  it("a dragged tile writes its position to the store", () => {
    seed(1);
    render(<CanvasLayout sessionIds={["s-1"]} />);
    const grip = handleEl("s-1", "cv-grip");

    fireEvent(grip, pointer("pointerdown", 100, 100));
    fireEvent(window, pointer("pointermove", 140, 130));

    // Moved on screen, from the default 32,28 by the pointer's 40,30.
    expect(tileEl("s-1").style.transform).toBe("translate3d(72px, 58px, 0)");
    // And NOT written yet. A setTile here is a localStorage serialise per frame.
    expect(useCockpit.getState().tiles["s-1"]).toBeUndefined();

    fireEvent(window, pointer("pointerup", 140, 130));

    expect(useCockpit.getState().tiles["s-1"]).toEqual({ x: 72, y: 58, w: 460, h: 320 });
  });

  it("clamps a drag to the plane at both ends", () => {
    seed(1);
    render(<CanvasLayout sessionIds={["s-1"]} />);

    gesture(handleEl("s-1", "cv-grip"), [100, 100], [0, 0]);
    expect(tileEl("s-1").style.transform).toBe("translate3d(0px, 0px, 0)");
    expect(useCockpit.getState().tiles["s-1"]).toEqual({ x: 0, y: 0, w: 460, h: 320 });

    gesture(handleEl("s-1", "cv-grip"), [10, 10], [9000, 9000]);
    // 2400 - 460 and 1600 - 320: the far edge of the plane, not past it.
    expect(useCockpit.getState().tiles["s-1"]).toEqual({ x: 1940, y: 1280, w: 460, h: 320 });
  });

  it("a resize honours the minimum, and caps at the plane edge instead of sliding the tile", () => {
    seed(1);
    render(<CanvasLayout sessionIds={["s-1"]} />);

    // The default tile spans 32,28 to 492,348. Drag its corner well inside the minimum.
    gesture(handleEl("s-1", "cv-resize"), [492, 348], [300, 200]);

    const el = tileEl("s-1");
    expect(el.style.width).toBe("300px");
    expect(el.style.height).toBe("180px");
    expect(el.style.transform).toBe("translate3d(32px, 28px, 0)");
    expect(useCockpit.getState().tiles["s-1"]).toEqual({ x: 32, y: 28, w: 300, h: 180 });

    // The asymmetry, and the reason clampTile takes the gesture. A tile at the right edge,
    // widened by 200: the WIDTH caps at the plane and x does not move. Clamping x against
    // the grown w instead slides the tile left while the user drags its corner right.
    act(() => {
      useCockpit.getState().setTile("s-1", { x: 2000, y: 28, w: 380, h: 320 });
    });
    gesture(handleEl("s-1", "cv-resize"), [2380, 348], [2580, 348]);

    expect(tileEl("s-1").style.width).toBe("400px");
    expect(tileEl("s-1").style.transform).toBe("translate3d(2000px, 28px, 0)");
    expect(useCockpit.getState().tiles["s-1"]).toEqual({ x: 2000, y: 28, w: 400, h: 320 });
  });

  it("stops following the pointer once the gesture has ended", () => {
    seed(1);
    render(<CanvasLayout sessionIds={["s-1"]} />);

    gesture(handleEl("s-1", "cv-grip"), [100, 100], [140, 130]);
    fireEvent(window, pointer("pointermove", 900, 900));

    expect(tileEl("s-1").style.transform).toBe("translate3d(72px, 58px, 0)");
    expect(useCockpit.getState().tiles["s-1"]).toEqual({ x: 72, y: 58, w: 460, h: 320 });
  });

  it("raises a tile above its neighbours on pointerdown anywhere in it", () => {
    seed(3);
    render(<CanvasLayout sessionIds={["s-1", "s-2", "s-3"]} />);

    expect(tileEl("s-1").style.zIndex).toBe("1");
    expect(tileEl("s-3").style.zIndex).toBe("3");

    fireEvent(tileEl("s-1"), pointer("pointerdown", 50, 50));
    expect(Number(tileEl("s-1").style.zIndex)).toBeGreaterThan(Number(tileEl("s-3").style.zIndex));

    fireEvent(tileEl("s-2"), pointer("pointerdown", 50, 50));
    expect(Number(tileEl("s-2").style.zIndex)).toBeGreaterThan(Number(tileEl("s-1").style.zIndex));
  });

  it("renders no more than the ten-tile cap, whatever the pin list says", () => {
    // A pin list of 11 cannot come from pinSession, which no-ops at the cap. It can come from a
    // rehydrated blob, which bypasses the action entirely.
    seed(11);
    render(<CanvasLayout sessionIds={ids(11)} />);

    expect(screen.getAllByTestId("pane")).toHaveLength(10);
    expect(document.querySelector('[data-cv-tile="s-11"]')).toBeNull();
  });

  it("names both handles for a screen reader", () => {
    seed(1);
    render(<CanvasLayout sessionIds={["s-1"]} />);

    expect(screen.getByRole("button", { name: "Move task s-1" })).toBeDefined();
    expect(screen.getByRole("button", { name: "Resize task s-1" })).toBeDefined();
  });
});
```

And the second `describe`, at the foot of the same file. Both of them fail if `CanvasLayout` ends up existing only in its own tests — shared-shape §1's stated fear, and the reason rule 3's grep exists. The first renders `DevelopLens` rather than `CanvasLayout`, so it fails unless step 4's edit to task 20's file actually landed. The second is also the only test that exercises the pin seeding, because seeding changes the store and only `DevelopLens` re-reads `selectCanvasSessionIds` afterwards:

```tsx
describe("DevelopLens on canvas", () => {
  beforeEach(() => {
    resetCockpit();
  });

  it("renders the canvas plane when layout is canvas, and columns when it is not", () => {
    seed(2);
    render(<DevelopLens />);

    expect(screen.getByTestId("cv-plane")).toBeDefined();

    act(() => {
      useCockpit.getState().setLayout("columns");
    });

    expect(screen.queryByTestId("cv-plane")).toBeNull();
    expect(screen.getAllByTestId("pane")).toHaveLength(2);
  });

  it("seeds an empty pin list from the columns order, capped at ten", () => {
    seed(12, { pinned: [] });
    render(<DevelopLens />);

    expect(useCockpit.getState().pinned).toEqual(ids(10));
    expect(screen.getAllByTestId("pane")).toHaveLength(10);
    // The cap was respected by arithmetic, not by pinSession no-opping, so no toast was raised.
    expect(useCockpit.getState().toasts).toHaveLength(0);
  });
});
```

- [ ] **Step 3: Watch it fail, for the right reason**

```bash
cd desktop
npm test -- src/components/develop/CanvasLayout.test.tsx
```

**What you must see:** a transform-time resolution failure, not an assertion failure. `./CanvasLayout` does not exist, so vitest fails the *file* and collects zero tests from it:

```
 FAIL  src/components/develop/CanvasLayout.test.tsx [ src/components/develop/CanvasLayout.test.tsx ]
Error: Failed to resolve import "./CanvasLayout" from "src/components/develop/CanvasLayout.test.tsx". Does the file exist?
```

The frames printed under that line come from Vite's transform pipeline and their paths move with the Vite version, so they are not reproduced here — match on those two lines and on `Test Files  1 failed (1)` in the summary.

**If you instead see `Failed to resolve import "./DevelopLens"`, task 20 has not landed** and this task cannot start; stop and report it. **If you see assertion failures rather than a resolution failure**, a `CanvasLayout.tsx` already exists and another agent has been in this directory; stop rather than overwriting it.

- [ ] **Step 4: Create `CanvasLayout.tsx`, wire the branch into `DevelopLens.tsx`, repoint task 20's interim test — one step**

Shared-shape rule 2, and it is not a stylistic preference: Group C's task 11 added a call in step 1 and defined the callee in step 5, which left the package uncompilable from step 2 to step 4 and made the green suite claimed in step 3 impossible. The edits below are one change — the component, the branch that calls it, and the one assertion in task 20's suite that was written to hold only until that branch existed.

Read the component in parts. **`CanvasLayout`** owns the plane, the pin seeding, the cap and the z-stack. **`CanvasTile`** owns one tile's geometry and its two gestures. **The two pure helpers** own the arithmetic, which is where the plane's dimensions live — the sheet's inline size and `clampTile` read the same constants, so they cannot drift the way a CSS width and a TypeScript clamp would.

`clampTile` takes the gesture as a second argument, and that is not decoration. A move changes `x`/`y` with the size fixed, so `x` clamps against `w`; a resize changes `w`/`h` with the position fixed, so `w` clamps against `x`. The obvious single-branch version clamps the position in both cases, and it slides a tile at the right edge leftwards while the user drags its corner rightwards: from `{x: 2000, w: 380}` a +200 drag gives `{x: 1820, w: 580}` where it should give `{x: 2000, w: 400}`. I ran both forms over every position this suite asserts — the lattice, the drag, both clamp ends, the minimum and the post-gesture reads — and they agree on all of them except that one, which is why step 2's resize test now carries it.

The `useLayoutEffect` is the heart of it. Its dependency array is the four numbers, not the `position` object, and that matters: `tiles[id] ?? defaultTilePosition(i)` builds a fresh object on every render of the parent, so an object dependency would re-apply the committed geometry on every poll — including mid-drag, which is the snap-back this task exists to avoid. With four primitive dependencies the effect runs only when the *committed* position actually changes, and the committed position only changes on `pointerup`.

Create `desktop/src/components/develop/CanvasLayout.tsx`:

```tsx
import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import type { JSX, PointerEvent as ReactPointerEvent } from "react";
import { useShallow } from "zustand/react/shallow";
import { SessionPane } from "./SessionPane";
import { selectPaneSessionIds } from "../../store/selectors";
import { useCockpit } from "../../store/store";
import { CANVAS_MAX_TILES } from "../../store/slices/prefsSlice";
import type { SessionId, TilePosition } from "../../store/types";

/* The plane. Contract §6.5 fixes 2400x1600. The sheet is sized from these constants and clampTile
 * bounds against them, so a CSS width and a TypeScript clamp cannot drift apart. */
const PLANE_W = 2400;
const PLANE_H = 1600;

/* A new tile, and the lattice new tiles land on. Ten - the cap - fit in three rows of four
 * without overlapping and without touching either edge: 32 + 3*488 + 460 = 1956 of 2400 across,
 * 28 + 2*348 + 320 = 1044 of 1600 down. */
const TILE_W = 460;
const TILE_H = 320;
const MIN_W = 300;
const MIN_H = 180;
const COLS = 4;
const X0 = 32;
const Y0 = 28;
const DX = 488;
const DY = 348;

function defaultTilePosition(index: number): TilePosition {
  return {
    x: X0 + (index % COLS) * DX,
    y: Y0 + Math.floor(index / COLS) * DY,
    w: TILE_W,
    h: TILE_H,
  };
}

/** A tile can never be smaller than its minimum and can never have a far edge outside the
 *  plane. WHICH of the two gives way depends on the gesture: a move has a fixed size and
 *  clamps the position against it, a resize has a fixed position and clamps the size against
 *  it. Clamping the position in both cases slides a tile at the right edge leftwards while
 *  its corner is dragged rightwards. */
function clampTile(t: TilePosition, gesture: "move" | "resize"): TilePosition {
  const w = Math.max(Math.round(t.w), MIN_W);
  const h = Math.max(Math.round(t.h), MIN_H);
  if (gesture === "move") {
    const fw = Math.min(w, PLANE_W);
    const fh = Math.min(h, PLANE_H);
    return {
      x: Math.min(Math.max(Math.round(t.x), 0), PLANE_W - fw),
      y: Math.min(Math.max(Math.round(t.y), 0), PLANE_H - fh),
      w: fw,
      h: fh,
    };
  }
  const x = Math.min(Math.max(Math.round(t.x), 0), PLANE_W - MIN_W);
  const y = Math.min(Math.max(Math.round(t.y), 0), PLANE_H - MIN_H);
  return { x, y, w: Math.min(w, PLANE_W - x), h: Math.min(h, PLANE_H - y) };
}

export function CanvasLayout({ sessionIds }: { sessionIds: SessionId[] }): JSX.Element {
  const paneIds = useCockpit(useShallow(selectPaneSessionIds));
  const tiles = useCockpit((s) => s.tiles);
  const setTile = useCockpit((s) => s.setTile);
  const pinSession = useCockpit((s) => s.pinSession);
  const [stack, setStack] = useState<SessionId[]>([]);

  /* Decision (a). Nothing in Group D renders a "pin to canvas" verb, so an unseeded canvas stays
   * blank for ever and the layout toggle reads as broken. The room left under the cap is computed
   * BEFORE pinning: pinSession no-ops past the cap and toasts it (§7.7), and a toast on every
   * mount that the user cannot act on is worse than the blank plane it replaces. */
  useEffect(() => {
    const room = CANVAS_MAX_TILES - sessionIds.length;
    if (room <= 0) return;
    const missing = paneIds.filter((id) => !sessionIds.includes(id)).slice(0, room);
    for (const id of missing) pinSession(id);
  }, [paneIds, sessionIds, pinSession]);

  /* pinSession enforces the cap, but a pin list persisted by an older build rehydrates without
   * passing through it. Render the cap, not the list. */
  const visible = sessionIds.slice(0, CANVAS_MAX_TILES);

  const raise = useCallback((id: SessionId): void => {
    setStack((prev) =>
      prev[prev.length - 1] === id ? prev : [...prev.filter((x) => x !== id), id],
    );
  }, []);

  const commit = useCallback(
    (id: SessionId, next: TilePosition): void => {
      setTile(id, next);
    },
    [setTile],
  );

  return (
    <div className="cv-plane" data-testid="cv-plane">
      <div className="cv-sheet" style={{ width: PLANE_W, height: PLANE_H }}>
        {visible.map((id, i) => {
          const raised = stack.indexOf(id);
          return (
            <CanvasTile
              key={id}
              sessionId={id}
              position={tiles[id] ?? defaultTilePosition(i)}
              z={raised === -1 ? i + 1 : CANVAS_MAX_TILES + 1 + raised}
              onRaise={raise}
              onCommit={commit}
            />
          );
        })}
      </div>
    </div>
  );
}
```

Same file, the tile. The inner functions are `function` declarations rather than `const` arrows on purpose: `end` names `onUp` and `onUp` names `end`, and declarations hoist, so neither ordering is a temporal-dead-zone trap for whoever transcribes this:

```tsx
interface TileProps {
  sessionId: SessionId;
  position: TilePosition;
  z: number;
  onRaise: (id: SessionId) => void;
  onCommit: (id: SessionId, next: TilePosition) => void;
}

function CanvasTile({ sessionId, position, z, onRaise, onCommit }: TileProps): JSX.Element {
  // A primitive read: stable across a poll that did not rename the session.
  const title = useCockpit((s) => s.sessions[sessionId]?.title ?? sessionId);
  const tileRef = useRef<HTMLDivElement | null>(null);
  const coordsRef = useRef<HTMLSpanElement | null>(null);
  const live = useRef<TilePosition>(position);
  const endRef = useRef<(() => void) | null>(null);
  const [gesture, setGesture] = useState<"move" | "resize" | null>(null);

  function apply(t: TilePosition): void {
    const el = tileRef.current;
    if (el !== null) {
      el.style.transform = `translate3d(${t.x}px, ${t.y}px, 0)`;
      el.style.width = `${t.w}px`;
      el.style.height = `${t.h}px`;
    }
    // §7.6: a tile's own coordinates while it is being dragged are one of the three places
    // Phase 2 allows a number. CSS hides this span unless a move is in progress.
    if (coordsRef.current !== null) coordsRef.current.textContent = `${t.x} · ${t.y}`;
  }

  /* Four primitive dependencies, never the object: the parent rebuilds `position` on every render
   * when the session has no stored tile, so an object dependency would re-apply the committed
   * geometry on every 2 s poll - including mid-drag, which is the snap-back this avoids. */
  useLayoutEffect(() => {
    live.current = position;
    apply(position);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [position.x, position.y, position.w, position.h]);

  // A tile unmounted mid-gesture must not leave its window listeners behind.
  useEffect(() => () => endRef.current?.(), []);

  function begin(e: ReactPointerEvent<HTMLElement>, mode: "move" | "resize"): void {
    if (e.button !== 0) return;
    e.preventDefault();
    const startX = e.clientX;
    const startY = e.clientY;
    const origin = live.current;
    setGesture(mode);

    function onMove(ev: MouseEvent): void {
      const dx = ev.clientX - startX;
      const dy = ev.clientY - startY;
      const next = clampTile(
        mode === "move"
          ? { ...origin, x: origin.x + dx, y: origin.y + dy }
          : { ...origin, w: origin.w + dx, h: origin.h + dy },
        mode,
      );
      live.current = next;
      apply(next);
    }

    function onUp(): void {
      end();
      // The ONE store write of the whole gesture.
      onCommit(sessionId, live.current);
    }

    function end(): void {
      window.removeEventListener("pointermove", onMove);
      window.removeEventListener("pointerup", onUp);
      window.removeEventListener("pointercancel", onUp);
      endRef.current = null;
      setGesture(null);
    }

    endRef.current = end;
    // Listeners on window, not setPointerCapture: capture is unimplemented in jsdom and a
    // release that never fires would wedge the pointer in a real webview too.
    window.addEventListener("pointermove", onMove);
    window.addEventListener("pointerup", onUp);
    window.addEventListener("pointercancel", onUp);
  }

  return (
    <div
      ref={tileRef}
      className="cv-tile"
      data-cv-tile={sessionId}
      data-cv-dragging={gesture ?? "none"}
      style={{ zIndex: z }}
      onPointerDownCapture={() => onRaise(sessionId)}
    >
      <div
        className="cv-grip"
        role="button"
        tabIndex={-1}
        aria-label={`Move ${title}`}
        onPointerDown={(e) => begin(e, "move")}
      >
        <span className="cv-grip-bar" aria-hidden="true" />
        <span className="cv-coords" ref={coordsRef} aria-hidden="true" />
      </div>
      <div className="cv-tile-body">
        <SessionPane sessionId={sessionId} variant="tile" />
      </div>
      <div
        className="cv-resize"
        role="button"
        tabIndex={-1}
        aria-label={`Resize ${title}`}
        onPointerDown={(e) => begin(e, "resize")}
      />
    </div>
  );
}
```

Now the other half of the same step: `desktop/src/components/develop/DevelopLens.tsx`, which is **task 20's file**. What changes: the `CanvasLayout` import, `selectCanvasSessionIds` added to the selectors import, the `canvasIds` and `layout` reads at the top of the body, and the `TASK 21` reservation comment plus its `return` replaced by the ternary. The selectors import is shown broken across lines below because it grew a third name; that is formatting, not a requirement. Nothing else in the file is touched — if the surrounding lines differ from what is shown here, keep task 20's and change only what this step names.

The ternary's formatting is load-bearing. Written on one line, `<CanvasLayout` would not begin a line and step 7's anchored guard would read 0 after the change instead of 1 — the guard would then be checking nothing, which is precisely the state shared-shape rule 3 exists to prevent.

```tsx
import "./develop.css";
import type { JSX } from "react";
import { useShallow } from "zustand/react/shallow";
import { CanvasLayout } from "./CanvasLayout";
import { ColumnsLayout } from "./ColumnsLayout";
import { EmptyDevelop } from "./EmptyDevelop";
import {
  selectCanvasSessionIds,
  selectPaneSessionIds,
  selectSelectedBranch,
} from "../../store/selectors";
import { useCockpit } from "../../store/store";

export function DevelopLens(): JSX.Element {
  // useShallow is mandatory here: this selector builds a new array every call (§3.7).
  const sessionIds = useCockpit(useShallow(selectPaneSessionIds));
  const canvasIds = useCockpit(useShallow(selectCanvasSessionIds));
  const layout = useCockpit((s) => s.layout);
  const branch = useCockpit(selectSelectedBranch);

  if (sessionIds.length === 0) {
    return (
      <div className="cols-empty">
        <EmptyDevelop branch={branch} />
      </div>
    );
  }

  /* Canvas renders the pinned working set (§3.7 selectCanvasSessionIds); Columns renders every
     session on the branch (§3.7 selectPaneSessionIds). The empty case above is decided on the
     pane list, so Canvas is never reached with nothing to show, and CanvasLayout seeds an empty
     pin list from the pane order rather than rendering a bare plane. */
  return layout === "canvas" ? (
    <CanvasLayout sessionIds={canvasIds} />
  ) : (
    <ColumnsLayout sessionIds={sessionIds} />
  );
}
```

And the third file, which is the same rule running the other way. Task 20's `ColumnsLayout.test.tsx` asserts that Canvas renders **Columns** *until task 21 lands* — and the line above is task 21 landing. That assertion is false from this moment, so it is repointed here rather than in a later step: leaving a knowingly-red suite across a step boundary is how an implementer ends up reverting correct wiring. Replace the title, the comment and the assertion of that one `it`, and nothing else in the file:

```tsx
  it("renders the Canvas plane when the layout is canvas", () => {
    seedBranch({ worktrees: ["w-1"], sessionIdsByWorktree: { "w-1": ["s-1"] } });
    useCockpit.setState({ layout: "canvas" });
    const { container } = render(<DevelopLens />);

    // Task 20 reserved this expectation for task 21 to replace. It asserted `.cols-scroll`
    // while Canvas still fell through to Columns; the plane is what it falls through to now.
    expect(container.querySelector('[data-testid="cv-plane"]')).not.toBeNull();
    expect(container.querySelector(".cols-scroll")).toBeNull();
  });
```

What that replacement does, and does not do. `not.toBeNull()` and `toBeNull()` rather than `toBeInTheDocument()`: jest-dom is not installed and shared-shape §3C forbids its matchers. It needs no new mock — that file already mocks `../Terminal` and `../../api`, and a tile reaches xterm only through the first of them. And it adds no test, so task 20's own `Tests  10 passed (10)` still holds; if that number moves, something other than this expectation was changed.


- [ ] **Step 5: Append the `.cv-*` block to `develop.css`**

Shared-shape §2: this task owns `.cv-*`, task 20 owns `.pane-*` and `.cols-*`, and this block goes **under the reserved header at the foot of the file** without reflowing a rule above it, so the diff stays reviewable.

The rules that govern what may go in it are task 24's `designsystem.test.ts`, mechanically: the file's first line is the token import, no literal hex, no colour function outside the token layer, **no gradient of any kind**, no `transition: all` and no transition that names no property, no indigo or violet hue, and every `box-shadow` built from a token or from `currentColor`. That last one is why the resize corner is `inset -2px -2px 0 0 var(--line-strong)` rather than a flat border, and the focus ring `var(--e-card), 0 0 0 1px var(--accent-ring)`: both carry `var(--`, which is what that assertion reads. The gradient rule is a shared-shape §3B correction to an earlier draft of this very block, which broke it in two places — a `radial-gradient` dot grid and a `linear-gradient` resize corner — and would have made task 24's audit unpassable through no fault of task 24's. One more rule is this file's own stance rather than task 24's, inherited from task 20: **no `transition` at all**. Nothing here has causality to explain. A tile that eases toward the pointer is a tile that is behind the pointer, and an elevation that fades up on grab is wrong for 180 ms of every gesture.

The token import is the one §3B rule this block cannot satisfy: it belongs on line 1 and line 1 is task 20's. Step 1 checks it is there and says to report rather than prepend it, because prepending is the reflow §2 forbids.

Some of the rules below do structural work rather than decorative work. `.cv-plane { height: 100% }` is one of them and it is easy to get wrong: the plane's parent is task 18's `<div className="shell-lens">`, which is a **block** container with `overflow: hidden`, not a flex one. A `flex: 1` there does nothing at all, the 1600px sheet renders at full height inside a clipped box and the plane never scrolls — the bug looks like a missing `overflow` and is not. Task 20's `.cols-scroll` sits in the same slot and uses `height: 100%` for exactly this reason. `.cv-tile { top: 0; left: 0 }` is what makes `transform` the only positioning channel: the tile is pinned at the sheet's origin and translated from there, so a stray `left` write would have to fight a stylesheet rule rather than silently taking over. And `.cv-tile-body > *` sizes whatever `SessionPane` renders without naming a single one of task 20's class names — the pane fills the tile by layout rather than by agreement between two files.

Append to `desktop/src/components/develop/develop.css`:

```css
/* ── Canvas (task 21) ─────────────────────────────────────────────────────────────────
 * The 2400x1600 plane, its tiles and their two handles. Sizes come from CanvasLayout.tsx
 * (PLANE_W / PLANE_H) and are set inline, so this file never states them a second time.
 * No transition in this block and no gradient in it either, both deliberately: see the
 * plan's step 5. Both words do appear in prose here and in the rule comments below,
 * which is why step 7 counts `transition:` at the start of a line and `-gradient(` with
 * its parenthesis rather than the bare words (shared-shape §3A). */

.cv-plane {
  /* height, not flex: the parent is task 18's `.shell-lens`, a BLOCK container with
     overflow: hidden, so a flex child grows nothing and the 1600px sheet would be clipped
     with no scrollbar at all. Task 20's `.cols-scroll` fills the same slot the same way. */
  height: 100%;
  min-height: 0;
  position: relative;
  overflow: auto;
  /* Flat. The dot grid an earlier draft drew here was a radial-gradient, and §3B bans every
     gradient; `--canvas` is the token whose own comment calls it the Canvas backdrop. */
  background: var(--canvas);
}

.cv-sheet {
  position: relative;
}

.cv-tile {
  position: absolute;
  top: 0;
  left: 0; /* position is transform-only; these two pin the origin it translates from */
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border-radius: var(--r-card);
  background: var(--page);
  box-shadow: var(--e-card);
  will-change: transform;
}

.cv-tile[data-cv-dragging="move"],
.cv-tile[data-cv-dragging="resize"] {
  box-shadow: var(--e-raised);
}

.cv-grip {
  flex: 0 0 15px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  background: var(--surface);
  cursor: grab;
  touch-action: none; /* or a pen drag scrolls the plane instead of moving the tile */
}

.cv-tile[data-cv-dragging="move"] .cv-grip {
  cursor: grabbing;
}

.cv-grip-bar {
  width: 26px;
  height: 3px;
  border-radius: var(--r-chip);
  background: var(--ink-4);
}

.cv-coords {
  display: none;
  font: 500 10px/1 var(--mono);
  /* The `font` shorthand RESETS font-variant-numeric, which app.css sets once on <body> for
     everything to inherit. §7.3 is the rule that reset would break — it names tile
     coordinates in its enforcement list — and §7.6 is why these digits exist at all, so
     restate it here or they jitter under the pointer. */
  font-variant-numeric: var(--numeric);
  color: var(--ink-3);
}

.cv-tile[data-cv-dragging="move"] .cv-coords {
  display: block;
}

.cv-tile-body {
  flex: 1;
  min-height: 0;
  display: flex;
}

/* Sizes whatever SessionPane renders without naming one of task 20's class names. */
.cv-tile-body > * {
  flex: 1;
  min-width: 0;
}

.cv-resize {
  position: absolute;
  right: 0;
  bottom: 0;
  width: 16px;
  height: 16px;
  cursor: nwse-resize;
  touch-action: none;
  /* Two 2px edges meeting in the corner, drawn as an inset ring rather than the diagonal
     gradient an earlier draft used: §3B bans gradients and task 24 greps for them. */
  box-shadow: inset -2px -2px 0 0 var(--line-strong);
}

.cv-tile:focus-within {
  box-shadow: var(--e-card), 0 0 0 1px var(--accent-ring);
}
```

- [ ] **Step 6: Watch the suite pass, then the whole suite and the types**

```bash
cd desktop
npm test -- src/components/develop/CanvasLayout.test.tsx
npm test
npm run typecheck
```

**What you must see** from the first command: thirteen passing tests in one file, eleven in `CanvasLayout` and two in `DevelopLens on canvas`. **Vitest 5 names a file only when it fails**, so a clean run prints the summary and nothing above it — there is no `✓ src/…/CanvasLayout.test.tsx (13 tests)` line. That line is vitest 3's shape, an earlier draft of this plan printed it, and an implementer who waits for it waits for ever. The `Start at` clock and the `Duration` line are machine-dependent and are not reproduced:

```
 Test Files  1 passed (1)
      Tests  13 passed (13)
```

Those two lines are copied off a real run rather than reasoned about. I extracted every TSX block this plan ships — the two halves of `CanvasLayout.tsx`, the `DevelopLens.tsx` branch and the blocks that make up the test file — by line range into a scratch project outside the repository on vitest 5.0.1, jsdom 30.1.0, `@vitejs/plugin-react` 6.0.2 and `@testing-library/react` 16.3.3, which are task 14's pins; stubbed `store/store.ts`, `store/selectors.ts`, `store/slices/prefsSlice.ts` and `test/fixtures.ts` to the shapes tasks 15 and 17 declare, and ran the thirteen. They pass. The same tree under this repo's own `tsc` (6.0.3) with `desktop/tsconfig.json`'s options prints nothing and exits 0. And reverting `clampTile` to the single branch that clamps the position in both gestures turns exactly one test red, `expected '580px' to be '400px'` — which is how I know the resize assertion is load-bearing rather than decorative.

`npm test` runs every Group D suite written so far and must stay green. This task adds a branch to a file other suites render, and one of them — task 20's `ColumnsLayout.test.tsx` — **asserted on that branch deliberately**, which is why step 4 repointed its expectation in the same step as the wiring. So a red `ColumnsLayout.test.tsx` here means one of two things and they need opposite fixes: either step 4's repoint was skipped, in which case the failing assertion names `.cols-scroll` and the fix is to finish step 4; or the `DevelopLens` edit changed a Columns case it should not have, in which case it names something else. Read the failing assertion before touching any source — reverting the wiring on the first reading is exactly how a component ends up existing only in its own tests. `npm run typecheck` prints **nothing at all** and exits 0; `tsc --noEmit` is silent on success.

The failures below are worth predicting because each means something specific drifted rather than the work being wrong.

An unresolved `prefsSlice` means task 17 put `CANVAS_MAX_TILES` somewhere else; step 1's grep should have caught it, so re-run step 1 rather than guessing. Measured by running this repo's own `tsc` (6.0.3) over the import block above with `desktop/tsconfig.json`'s compiler options. The **line** depends on where your transcription puts that import and is shown as `NN`; the **column** does not, because it points at the module specifier, and the specifier is the assertion:

```
src/components/develop/CanvasLayout.tsx(NN,34): error TS2307: Cannot find module '../../store/slices/prefsSlice' or its corresponding type declarations.
```

An assertion failure of the shape below means the pointer helper degraded and the handler computed with `NaN` — check that the test file constructs `MouseEvent`s and is not calling `fireEvent.pointerDown`:

```
AssertionError: expected 'translate3d(NaNpx, NaNpx, 0)' to be 'translate3d(72px, 58px, 0)' // Object.is equality

Expected: "translate3d(72px, 58px, 0)"
Received: "translate3d(NaNpx, NaNpx, 0)"
```

The `// Object.is equality` tail and the two lines under it are vitest 5.0.1's own formatting for a failed `toBe` on two strings, taken from a run of that assertion against that value — a diff that stops at the first line is a different reporter and probably a different runner.

- [ ] **Step 7: The pre-commit guards**

The checks below start with the one shared-shape rule 3 was written for. A component that renders only inside its own test file is the Group C defect this group is trying not to repeat, and the naive `grep -c "<CanvasLayout"` cannot see it: it reads **1** before step 4, satisfied by task 20's reservation comment alone, and **1** after, satisfied by the wired line — the same answer for both states. Anchoring on the first non-space character makes a comment unable to satisfy it, so the anchored form reads 0 and then 1.

```bash
cd desktop
# 1. The branch is wired in production code, not only in tests. Anchored: a // or {/* */}
#    comment cannot match, and this read 0 in step 1.
grep -cE "^[[:space:]]*<CanvasLayout" src/components/develop/DevelopLens.tsx   # expect 1
# 2. Task 20's interim expectation was repointed, not left to fail.
grep -c "until task 21 lands" src/components/develop/ColumnsLayout.test.tsx   # expect 0
grep -c "cv-plane" src/components/develop/ColumnsLayout.test.tsx             # expect 1
# 3. Position is transform-only. No left/top write anywhere in the component.
grep -cE "style\.(left|top)" src/components/develop/CanvasLayout.tsx          # expect 0
# 4. The store is written once per gesture, from the pointerup path only.
grep -c "setTile(" src/components/develop/CanvasLayout.tsx                    # expect 1
# 5. Prefix discipline: every selector this task added to develop.css is .cv-*.
grep -cE "^\.(pane|cols)-" src/components/develop/develop.css                 # the number from step 1
grep -cE "^\.cv-" src/components/develop/develop.css                          # expect 14
# 6. The rules task 24's design-system audit greps for, run here instead of there.
grep -cE "^[[:space:]]*transition[[:space:]]*:" src/components/develop/develop.css   # expect 0
grep -cE "(linear|radial|conic)-gradient\(" src/components/develop/develop.css      # expect 0
grep -cE "#[0-9a-fA-F]{3,8}" src/components/develop/develop.css               # expect 0
# 7. Group D touches no Go.
git diff --name-only | grep -c "\.go$"                                        # expect 0
```

**Every number above was read off the assembled files, not reasoned about**, and several are not what the obvious form of the guard would say — shared-shape §3A is the rule, and this block applies it to `<CanvasLayout`, `setTile`, `transition` and `gradient`.

Check 4 counts `setTile(` **with the open parenthesis**, and expects **1**: the single call inside `commit`. The bare `grep -c "setTile"` reads **3** against correct code — the `const setTile = useCockpit(...)` binding, the call, and the `[setTile]` dependency array — so a guard written that way fails on a correct component and sends the implementer hunting a frame-rate write path that is not there. Two matches for `setTile(` would be the real defect, and the second one is almost always inside `onMove`: the `localStorage` serialise per frame this whole design exists to avoid.

Check 5's first line is a change-detector rather than a value assertion: whatever step 1 recorded it must still read now, because this task appends and never reflows. Its second line is 14, counted off the block step 5 appends.

Check 6 is anchored twice over. A bare `grep -c "transition"` reads **4** on the assembled file and a bare `grep -c "gradient"` reads **6**: both headers and the rule comments below them say the words in prose, and `grep -c` counts matching lines rather than mentions. Both were counted over the file the two plans assemble — task 20's block, then this one's. Counting the declaration (`transition:` at the start of a line) and the function form (`-gradient(`) makes prose unable to satisfy either, and both then read 0, which is the true answer about the CSS.

**What none of these can check** is that the thing looks right, and this is the first Group D surface where that matters. `npm run dev`, flip the top bar's toggle to Canvas, and drag a tile by its grip: the pane should track the pointer exactly, the coordinate readout should count in the grip, the elevation should lift while the gesture runs, and the terminal inside should not blink, reflow or lose its scrollback. Then flip to Columns and back, and restart the app. The tile is where you left it, twice. `CLAUDE.md` is explicit that a screenshot is not evidence and that visual acceptance is the owner's eyes; this is a thing to show, not to self-report.

- [ ] **Step 8: Commit**

```bash
cd ..   # steps 1 to 7 ran from desktop/; these paths are repository-relative
git add desktop/src/components/develop/CanvasLayout.tsx \
        desktop/src/components/develop/CanvasLayout.test.tsx \
        desktop/src/components/develop/DevelopLens.tsx \
        desktop/src/components/develop/ColumnsLayout.test.tsx \
        desktop/src/components/develop/develop.css
git commit -m "feat(cockpit): canvas layout with tiles that stay where you put them

Known defect 6: the prototype's canvas kept x and y on a JavaScript object
that died with the page, so a hand-arranged plane survived neither a flip to
Columns nor a reload. Positions now write through prefsSlice.tiles, keyed by
session, inside partialize, and the regression test the defect table names is
in CanvasLayout.test.tsx.

React does not own the tile's geometry and that is the whole design. transform,
width and height are applied by a layout effect keyed on four numbers and
written directly through a ref during a gesture; the style prop carries zIndex
and nothing else. If React owned transform, the 2 s poller landing mid-drag
would rewrite it from the last committed position and the tile would snap back
to where the pointer was 400ms ago. The store is written once, on pointerup,
because a write per pointermove is a JSON serialise of the cockpit into
localStorage sixty times a second.

Position is transform only, never left/top, and the block declares no
transition at all: a tile that eases toward the pointer is a tile that is
behind the pointer. It declares no gradient either - the dot grid and the
resize triangle an earlier draft drew were both gradients, which the shared
shape bans outright and task 24's audit greps for. Size is the documented
exception - transform: scale would blur xterm's glyph grid and would not
change the PTY's cols and rows - and it is direct manipulation at pointer
rate, not an animation.

The departures are argued in the plan and counted in neither place. The tile
carries its own 15px grip rather than dragging by the pane header, because
SessionPaneHeader.tsx belongs to task 20 and contract section 9 row 21 does
not list it; matching on another task's class names from inside a pointer
handler is the coupling Group D's shared shape exists to stop. The canvas
seeds its pin list from the Columns order, because no component in Group D
renders a pin verb and a canvas that renders only hand-pinned sessions
renders nothing, for ever. And zoom is deferred rather than faked, though
section 6.8 assigns canvas pan/zoom to this component; pan is the plane's
own overflow scrolling.

Modifies three files task 20 created. The branch, the component it calls and
the one expectation in ColumnsLayout.test.tsx that asserted Canvas fell
through to Columns until this task landed all change in one step, per
shared-shape rule 2: task 20 reserved that expectation for exactly this, and
leaving it red would read as the wiring being wrong. The CSS is confined to
the .cv-* prefix per shared-shape section 2's split, and every pre-commit
guard is anchored so a comment cannot satisfy it - the bare forms read 3, 4
and 6 against correct code.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```
