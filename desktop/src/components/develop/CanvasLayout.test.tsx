// Contract §6.5 and known defect 6: a hand-arranged canvas that survived neither a flip back to
// Columns nor a reload. The regression the defect table names — "a dragged tile writes its
// position to the store" — is below, and its mid-drag assertion is the load-bearing half.
//
// No jest-dom (shared-shape §3C): plain DOM property assertions only.
import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { CanvasLayout } from "./CanvasLayout";
import { DevelopLens } from "./DevelopLens";
import { resetCockpit, useCockpit } from "../../store/store";
import { aBranch, aProject, aSession, aWorktree } from "../../test/fixtures";
import type { CockpitState } from "../../store/store";
import type { Session, SessionId } from "../../store/types";

// SessionPane is task 20's, with task 20's suite. It builds a real <Terminal/>, and §1.5 keeps
// xterm out of jsdom. The stub also makes `variant` assertable directly, and counts its renders:
// a tile re-renders its pane every time the plane renders, so the count is how a stack write
// that changed nothing becomes visible (shared-shape §3A.1 clause 7).
const { paneRenders } = vi.hoisted(() => ({ paneRenders: new Map<string, number>() }));
vi.mock("./SessionPane", () => ({
  SessionPane: ({ sessionId, variant }: { sessionId: string; variant: string }) => {
    paneRenders.set(sessionId, (paneRenders.get(sessionId) ?? 0) + 1);
    return <div data-testid="pane" data-session={sessionId} data-variant={variant} />;
  },
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
function pointer(
  type: "pointerdown" | "pointermove" | "pointerup" | "pointercancel",
  x: number,
  y: number,
  pointerId?: number,
): MouseEvent {
  const ev = new MouseEvent(type, {
    bubbles: true,
    cancelable: true,
    clientX: x,
    clientY: y,
    button: 0,
  });
  // jsdom's MouseEvent carries no `pointerId`, and the production filter tests for the
  // property's PRESENCE before comparing. So a case that needs two pointers defines it, and
  // every other case leaves the event looking exactly like the single-pointer path.
  if (pointerId !== undefined) Object.defineProperty(ev, "pointerId", { value: pointerId });
  return ev;
}

function tileEl(id: SessionId): HTMLElement {
  const el = document.querySelector<HTMLElement>(`[data-cv-tile="${id}"]`);
  if (el === null) throw new Error(`no tile rendered for ${id}`);
  return el;
}

function z(id: SessionId): number {
  return Number(tileEl(id).style.zIndex);
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

  it("holds its dragged position when an unrelated store write re-renders the plane", () => {
    seed(1);
    render(<CanvasLayout sessionIds={["s-1"]} />);

    fireEvent(handleEl("s-1", "cv-grip"), pointer("pointerdown", 100, 100));
    fireEvent(window, pointer("pointermove", 140, 130));
    expect(tileEl("s-1").style.transform).toBe("translate3d(72px, 58px, 0)");

    // A neighbouring tile's position landing in the store - or any poll that writes `tiles` or
    // changes the pane list - re-renders the plane mid-gesture, and a tile with no stored
    // position of its own gets a FRESH defaultTilePosition object out of that render. A layout
    // effect keyed on that object rather than on the four numbers re-applies the COMMITTED
    // geometry here, and the tile snaps back to where it sat when the gesture started.
    act(() => {
      useCockpit.getState().setTile("s-2", { x: 900, y: 600, w: 460, h: 320 });
    });

    expect(tileEl("s-1").style.transform).toBe("translate3d(72px, 58px, 0)");

    // And the gesture is still live rather than merely undisturbed.
    fireEvent(window, pointer("pointermove", 150, 140));
    fireEvent(window, pointer("pointerup", 150, 140));
    expect(useCockpit.getState().tiles["s-1"]).toEqual({ x: 82, y: 68, w: 460, h: 320 });
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

  // ── fix round 2: one case per surviving mutant the reviewer found ──────────────────────

  it("marks the tile with the gesture that is running", () => {
    seed(1);
    render(<CanvasLayout sessionIds={["s-1"]} />);
    const grip = handleEl("s-1", "cv-grip");

    expect(tileEl("s-1").getAttribute("data-cv-dragging")).toBe("none");
    fireEvent(grip, pointer("pointerdown", 100, 100));
    expect(tileEl("s-1").getAttribute("data-cv-dragging")).toBe("move");
    fireEvent(window, pointer("pointerup", 100, 100));
    expect(tileEl("s-1").getAttribute("data-cv-dragging")).toBe("none");
  });

  it("keeps a tile's DOM node when the pin order changes", () => {
    seed(2);
    const { rerender } = render(<CanvasLayout sessionIds={["s-1", "s-2"]} />);
    const before = tileEl("s-2");

    rerender(<CanvasLayout sessionIds={["s-2", "s-1"]} />);

    // Carry 1's mechanism at the React level, and the claim my report argues from. Keyed by
    // session, a re-order MOVES the node, so the registry's element — and the scrollback
    // rendered inside it — is never re-parented. Keyed by index, React re-uses slot 0's node
    // for s-2 and this identity breaks.
    expect(tileEl("s-2")).toBe(before);
  });

  it("ends the gesture on pointercancel and keeps what was dragged", () => {
    seed(1);
    render(<CanvasLayout sessionIds={["s-1"]} />);

    fireEvent(handleEl("s-1", "cv-grip"), pointer("pointerdown", 100, 100));
    fireEvent(window, pointer("pointermove", 140, 130));
    fireEvent(window, pointer("pointercancel", 140, 130));

    expect(useCockpit.getState().tiles["s-1"]).toEqual({ x: 72, y: 58, w: 460, h: 320 });
    // And the listeners went with it: a later move is not still dragging.
    fireEvent(window, pointer("pointermove", 900, 900));
    expect(tileEl("s-1").style.transform).toBe("translate3d(72px, 58px, 0)");
  });

  it("ends the gesture when the window loses focus mid-drag", () => {
    // Alt-tab, a native dialog, the OS taking the pointer: no pointerup ever arrives. Left
    // armed, the next pointermove over the plane drags a tile nobody is holding.
    seed(1);
    render(<CanvasLayout sessionIds={["s-1"]} />);

    fireEvent(handleEl("s-1", "cv-grip"), pointer("pointerdown", 100, 100));
    fireEvent(window, pointer("pointermove", 140, 130));
    fireEvent(window, new Event("blur"));

    expect(tileEl("s-1").getAttribute("data-cv-dragging")).toBe("none");
    expect(useCockpit.getState().tiles["s-1"]).toEqual({ x: 72, y: 58, w: 460, h: 320 });
    // Disarmed, not merely committed.
    fireEvent(window, pointer("pointermove", 900, 900));
    expect(tileEl("s-1").style.transform).toBe("translate3d(72px, 58px, 0)");
  });

  it("writes nothing when the window loses focus without a move", () => {
    // The blur path has its own no-op guard, and until this case existed nothing measured it.
    // The two-site mutant that would have caught it was refused as malformed — correctly, and
    // refusing it left this unmeasured, which is the part that needed saying.
    seed(1);
    render(<CanvasLayout sessionIds={["s-1"]} />);

    fireEvent(handleEl("s-1", "cv-grip"), pointer("pointerdown", 100, 100));
    fireEvent(window, new Event("blur"));

    expect(useCockpit.getState().tiles["s-1"]).toBeUndefined();
  });

  it("stops listening for blur once the gesture is over", () => {
    // Counting WRITES, not comparing values: a leaked blur listener re-commits the position the
    // tile already has, so the store's contents look identical and only the extra write shows
    // it happened. Left registered, one listener leaks per gesture, each closed over a dead
    // tile's origin.
    seed(1);
    render(<CanvasLayout sessionIds={["s-1"]} />);

    let writes = 0;
    const unsubscribe = useCockpit.subscribe((s, prev) => {
      if (s.tiles !== prev.tiles) writes += 1;
    });
    try {
      gesture(handleEl("s-1", "cv-grip"), [100, 100], [140, 130]);
      expect(writes).toBe(1);

      fireEvent(window, new Event("blur"));

      expect(writes).toBe(1);
    } finally {
      unsubscribe();
    }
  });

  it("counts the tile's coordinates in the grip while a move runs", () => {
    seed(1);
    render(<CanvasLayout sessionIds={["s-1"]} />);
    const coords = tileEl("s-1").querySelector(".cv-coords");

    fireEvent(handleEl("s-1", "cv-grip"), pointer("pointerdown", 100, 100));
    fireEvent(window, pointer("pointermove", 140, 130));

    expect(coords?.textContent).toBe("72 · 58");
  });

  // ── fix round 2: the minors ───────────────────────────────────────────────────────────

  it("writes nothing for a click that moves the tile nowhere", () => {
    seed(1);
    render(<CanvasLayout sessionIds={["s-1"]} />);

    // The grip is not a button, so every click on it lands here. Committing unconditionally
    // runs the whole persist middleware to write back the position the tile already had.
    fireEvent(handleEl("s-1", "cv-grip"), pointer("pointerdown", 100, 100));
    fireEvent(window, pointer("pointerup", 100, 100));

    expect(useCockpit.getState().tiles["s-1"]).toBeUndefined();
  });

  it("clamps a rehydrated tile that no longer fits the plane", () => {
    // Saved against a larger plane — a wider monitor, or a future PLANE_W. Unclamped this
    // renders off-screen with no handle left on the plane to drag it back by.
    seed(1, { tiles: { "s-1": { x: 3000, y: 2000, w: 520, h: 360 } } });
    render(<CanvasLayout sessionIds={["s-1"]} />);

    expect(tileEl("s-1").style.transform).toBe("translate3d(1880px, 1240px, 0)");
  });

  it("ignores a second pointer while a gesture is in flight", () => {
    seed(1);
    render(<CanvasLayout sessionIds={["s-1"]} />);

    fireEvent(handleEl("s-1", "cv-grip"), pointer("pointerdown", 100, 100, 1));
    fireEvent(window, pointer("pointermove", 140, 130, 1));
    expect(tileEl("s-1").style.transform).toBe("translate3d(72px, 58px, 0)");

    // A second finger lands and drags: unfiltered, the tile teleports to it.
    fireEvent(window, pointer("pointermove", 900, 900, 2));
    expect(tileEl("s-1").style.transform).toBe("translate3d(72px, 58px, 0)");

    // And the second pointer's release must not end the first pointer's gesture.
    fireEvent(window, pointer("pointerup", 900, 900, 2));
    fireEvent(window, pointer("pointermove", 150, 140, 1));
    expect(tileEl("s-1").style.transform).toBe("translate3d(82px, 68px, 0)");
  });

  it("unpins from the tile chrome, which is the action the cap's toast names", () => {
    seed(2);
    render(<CanvasLayout sessionIds={["s-1", "s-2"]} />);

    fireEvent.click(screen.getByRole("button", { name: "Unpin task s-1" }));

    expect(useCockpit.getState().pinned).toEqual(["s-2"]);
  });

  it("raises the tile but does not start a drag when the unpin button is pressed", () => {
    seed(2);
    render(<CanvasLayout sessionIds={["s-1", "s-2"]} />);
    const unpin = screen.getByRole("button", { name: "Unpin task s-1" });

    fireEvent(unpin, pointer("pointerdown", 50, 50));

    // stopPropagation keeps the grip's drag handler out of it...
    expect(tileEl("s-1").getAttribute("data-cv-dragging")).toBe("none");
    // ...while onRaise still fires, because it is on the CAPTURE phase, which a descendant
    // cannot intercept. This is the expiry condition declared on that choice, now exercised.
    expect(Number(tileEl("s-1").style.zIndex)).toBeGreaterThan(Number(tileEl("s-2").style.zIndex));
  });

  it("names both handles for a screen reader", () => {
    seed(1);
    render(<CanvasLayout sessionIds={["s-1"]} />);

    expect(screen.getByRole("button", { name: "Move task s-1" })).toBeDefined();
    expect(screen.getByRole("button", { name: "Resize task s-1" })).toBeDefined();
  });

  // ── task 23, carry 1: a focus that arrives without a pointer ──────────────────────────

  it("raises the focused tile when focus arrives without a pointer, as ⌘1-9 does", () => {
    seed(3);
    render(<CanvasLayout sessionIds={["s-1", "s-2", "s-3"]} />);
    expect(z("s-1")).toBeLessThan(z("s-3"));

    act(() => {
      useCockpit.getState().focusSession("s-1");
    });

    expect(z("s-1")).toBeGreaterThan(z("s-3"));
    expect(z("s-1")).toBeGreaterThan(z("s-2"));
  });

  it("raises a clicked tile with one stack write, though the focus the click causes asks again", () => {
    seed(3);
    render(<CanvasLayout sessionIds={["s-1", "s-2", "s-3"]} />);
    paneRenders.clear();

    // One dispatch in the product: the tile's capture-phase raise, then SessionPane's own
    // pointerdown focusing the session (SessionPane.tsx, onPointerDown). The stub has no
    // handler, so the focus is written inside the same act, which batches it the same way.
    act(() => {
      fireEvent(tileEl("s-1"), pointer("pointerdown", 50, 50));
      useCockpit.getState().focusSession("s-1");
    });

    expect(z("s-1")).toBeGreaterThan(z("s-3"));
    // The final stack is identical whether the effect's second raise wrote or not, so the
    // count of plane renders is the observable: one for the click, none for the echo. It reads
    // them through the tiles, which re-render with the plane only because CanvasTile is not
    // memoised and gets a fresh `position` object; give a tile stable props behind memo() and
    // this passes with the check in `raise` deleted, so count the plane some other way first.
    expect(paneRenders.get("s-1")).toBe(1);
  });

  it("leaves a tile the user raised on top when the plane re-renders under a standing focus", () => {
    seed(3);
    render(<CanvasLayout sessionIds={["s-1", "s-2", "s-3"]} />);
    act(() => {
      useCockpit.getState().focusSession("s-1");
    });

    // The grip belongs to the tile, not to the pane, so a press on s-2's grip raises s-2 and
    // leaves the focus on s-1. No move, so the gesture writes nothing to the store.
    fireEvent(handleEl("s-2", "cv-grip"), pointer("pointerdown", 100, 100));
    fireEvent(window, pointer("pointerup", 100, 100));
    expect(z("s-2")).toBeGreaterThan(z("s-1"));
    expect(useCockpit.getState().selection.sessionId).toBe("s-1");

    // Any store write the plane reads re-renders it; a 2 s poll does the same. The focus has
    // not changed, so nothing may raise s-1 back over the tile the user just brought forward.
    act(() => {
      useCockpit.getState().setTile("s-3", { x: 900, y: 600, w: 460, h: 320 });
    });

    expect(z("s-2")).toBeGreaterThan(z("s-1"));
  });
});

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

  it("shows only the pins that belong to the selected branch", () => {
    // Two branches, and `pinned` is one flat global list (§3.4). Before fix round 2 the second
    // branch you visited rendered the first branch's tiles.
    seed(2);
    const other = aBranch({ id: "b-2", projectId: "p-1", name: "audit-trail", isHead: false });
    const otherWt = aWorktree({ id: "w-2", projectId: "p-1", branchId: "b-2", name: "wt-audit" });
    act(() => {
      useCockpit.setState((s) => ({
        branchesByProject: { "p-1": [branch, other] },
        worktreesByBranch: { ...s.worktreesByBranch, "b-2": [otherWt] },
        sessions: {
          ...s.sessions,
          "s-9": aSession({ ...base, id: "s-9", branchId: "b-2", worktreeId: "w-2", title: "task s-9" }),
        },
        sessionIdsByWorktree: { ...s.sessionIdsByWorktree, "w-2": ["s-9"] },
        pinned: ["s-1", "s-2", "s-9"],
      }));
    });

    render(<DevelopLens />);
    expect(screen.getAllByTestId("pane").map((p) => p.getAttribute("data-session"))).toEqual([
      "s-1",
      "s-2",
    ]);

    act(() => {
      useCockpit.setState({
        selection: { projectId: "p-1", branchId: "b-2", worktreeId: "w-2", sessionId: null },
      });
    });

    // The other branch's arrangement is still in `pinned` and comes back on a switch, which is
    // the reason a global set filtered for display beats re-keying the store.
    expect(screen.getAllByTestId("pane").map((p) => p.getAttribute("data-session"))).toEqual(["s-9"]);
    expect(useCockpit.getState().pinned).toEqual(["s-1", "s-2", "s-9"]);
  });

  it("gives a session that arrives while Canvas is open a tile of its own", () => {
    // The failure the seed-only-when-empty version had: the user creates a session, is looking
    // at Canvas, and nothing appears. An id absent from `seen` is new and gets a tile.
    seed(2);
    render(<DevelopLens />);
    expect(screen.getAllByTestId("pane")).toHaveLength(2);

    act(() => {
      useCockpit.setState((s) => ({
        sessions: { ...s.sessions, "s-3": aSession({ ...base, id: "s-3", title: "task s-3" }) },
        sessionIdsByWorktree: { "w-1": ["s-1", "s-2", "s-3"] },
      }));
    });

    expect(screen.getAllByTestId("pane").map((p) => p.getAttribute("data-session"))).toEqual([
      "s-1",
      "s-2",
      "s-3",
    ]);
  });

  it("keeps a topped-up session unpinned once the user unpins it", () => {
    // The other half, and the one that needs `seen` to be UPDATED rather than only read: s-3
    // was not pinned at mount, so only having been added on arrival keeps it out of the next
    // pass's `fresh` list.
    seed(2);
    render(<DevelopLens />);
    act(() => {
      useCockpit.setState((s) => ({
        sessions: { ...s.sessions, "s-3": aSession({ ...base, id: "s-3", title: "task s-3" }) },
        sessionIdsByWorktree: { "w-1": ["s-1", "s-2", "s-3"] },
      }));
    });

    fireEvent.click(screen.getByRole("button", { name: "Unpin task s-3" }));

    expect(useCockpit.getState().pinned).toEqual(["s-1", "s-2"]);
    expect(screen.getAllByTestId("pane")).toHaveLength(2);
  });

  it("keeps a session unpinned, rather than re-seeding it on the next render", () => {
    // This case has to go through DevelopLens. The CanvasLayout-level unpin test passes a
    // LITERAL sessionIds array, so the prop cannot change when the pin list does — and a
    // seeding effect that topped up any unpinned pane session would sail straight through it.
    // Here the selector feeds the prop, which is the shape the product has.
    seed(2);
    render(<DevelopLens />);
    expect(screen.getAllByTestId("pane")).toHaveLength(2);

    fireEvent.click(screen.getByRole("button", { name: "Unpin task s-1" }));

    expect(useCockpit.getState().pinned).toEqual(["s-2"]);
    expect(screen.getAllByTestId("pane").map((p) => p.getAttribute("data-session"))).toEqual([
      "s-2",
    ]);
  });

  it("does not raise a session focused from another branch, before or after that branch opens", () => {
    // A rail row focuses without switching branch, so the focused session can be one this
    // plane does not show. Raising it anyway puts an id with no tile in the stack.
    seed(2);
    const other = aBranch({ id: "b-2", projectId: "p-1", name: "audit-trail", isHead: false });
    const otherWt = aWorktree({ id: "w-2", projectId: "p-1", branchId: "b-2", name: "wt-audit" });
    const onOther = { ...base, branchId: "b-2", worktreeId: "w-2" };
    act(() => {
      useCockpit.setState((s) => ({
        branchesByProject: { "p-1": [branch, other] },
        worktreesByBranch: { ...s.worktreesByBranch, "b-2": [otherWt] },
        sessions: {
          ...s.sessions,
          "s-9": aSession({ ...onOther, id: "s-9", title: "task s-9" }),
          "s-10": aSession({ ...onOther, id: "s-10", title: "task s-10" }),
        },
        sessionIdsByWorktree: { ...s.sessionIdsByWorktree, "w-2": ["s-9", "s-10"] },
        pinned: ["s-1", "s-2", "s-9", "s-10"],
      }));
    });
    render(<DevelopLens />);
    const before = [z("s-1"), z("s-2")];

    act(() => {
      useCockpit.getState().focusSession("s-9");
    });
    expect([z("s-1"), z("s-2")]).toEqual(before);

    // The half that can fail. A stray stack entry changes no visible tile's z — every tile
    // keeps its own index — so it shows only when its tile appears, raised over a neighbour
    // nobody touched. selectBranch clears the focus, so nothing here raises s-9 legitimately.
    act(() => {
      useCockpit.getState().selectBranch("p-1", "b-2");
    });
    expect(z("s-9")).toBeLessThan(z("s-10"));
  });

  it("raises a session created on Canvas once its tile arrives", () => {
    // ⌘⏎'s shape: useCreateSession upserts the session and focuses it in one go, and the pin
    // that gives it a tile lands a render later. The focus changed before the tile existed.
    seed(2);
    render(<DevelopLens />);
    fireEvent(tileEl("s-1"), pointer("pointerdown", 50, 50));
    expect(z("s-1")).toBeGreaterThan(z("s-2"));

    act(() => {
      useCockpit.setState((s) => ({
        sessions: { ...s.sessions, "s-3": aSession({ ...base, id: "s-3", title: "task s-3" }) },
        sessionIdsByWorktree: { "w-1": ["s-1", "s-2", "s-3"] },
      }));
      useCockpit.getState().focusSession("s-3");
    });

    expect(useCockpit.getState().pinned).toContain("s-3");
    expect(z("s-3")).toBeGreaterThan(z("s-1"));
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
