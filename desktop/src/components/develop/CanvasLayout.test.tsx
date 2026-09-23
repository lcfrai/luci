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

  it("seeds an empty pin list from the columns order, capped at ten", () => {
    seed(12, { pinned: [] });
    render(<DevelopLens />);

    expect(useCockpit.getState().pinned).toEqual(ids(10));
    expect(screen.getAllByTestId("pane")).toHaveLength(10);
    // The cap was respected by arithmetic, not by pinSession no-opping, so no toast was raised.
    expect(useCockpit.getState().toasts).toHaveLength(0);
  });
});
