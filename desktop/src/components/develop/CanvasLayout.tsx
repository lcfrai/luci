// desktop/src/components/develop/CanvasLayout.tsx
//
// Contract §6.5 and known defect 6. Columns is the layout you read in; this is the one you
// arrange in, and the arrangement has to survive both a flip back to Columns and a restart.
//
// React does not own a tile's geometry, and that is the whole design. The style prop carries
// zIndex and nothing else; transform, width and height are applied by a layout effect keyed on
// four numbers and written directly through a ref during a gesture. If React owned transform,
// the 2 s poller landing mid-drag would rewrite it from the last COMMITTED position and the tile
// would snap back to where the pointer was 400 ms ago. The store is written once, on pointerup:
// a write per pointermove is a JSON serialise of the cockpit into localStorage sixty times a
// second, through the whole persist middleware.

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
    /* This early return is load-bearing, NOT a fast path, and no test pins it — a mutation
     * deleting it survives the whole suite, because every case where `room` goes non-positive
     * also happens to leave `missing` empty. It stops being equivalent the moment a canvas at
     * or over the cap has an unpinned pane session: `.slice(0, room)` with a NEGATIVE `room`
     * drops elements from the END rather than returning nothing, so without this line the loop
     * runs for all but the last |room| of them and pinSession toasts the cap once per call —
     * exactly the mount-time noise decision (a) computes the room to avoid. */
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

interface TileProps {
  sessionId: SessionId;
  position: TilePosition;
  z: number;
  onRaise: (id: SessionId) => void;
  onCommit: (id: SessionId, next: TilePosition) => void;
}

/* The inner functions below are `function` declarations rather than const arrows on purpose:
 * `end` names `onUp` and `onUp` names `end`, and declarations hoist, so neither ordering is a
 * temporal-dead-zone trap. */
function CanvasTile({ sessionId, position, z, onRaise, onCommit }: TileProps): JSX.Element {
  /* A primitive read: stable across a poll that did not rename the session. The `?? sessionId`
   * fallback is unreachable today and no test pins it — selectCanvasSessionIds filters `pinned`
   * to sessions that still exist (selectors.ts), and an exited session stays in `sessions`, so
   * a mounted tile always has one. It stops being equivalent the moment a tile can outlive its
   * session record: a forget-on-exit change, or any caller rendering CanvasLayout from an id
   * list it did not filter, turns this into a throw inside a store selector — a white screen
   * rather than a tile labelled with a raw id. */
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
   * geometry on every 2 s poll - including mid-drag, which is the snap-back this avoids. Pinned
   * by "holds its dragged position when an unrelated store write re-renders the plane"; swapping
   * this array for `[position]` fails it with translate3d(32px, 28px, 0). */
  useLayoutEffect(() => {
    live.current = position;
    apply(position);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [position.x, position.y, position.w, position.h]);

  // A tile unmounted mid-gesture must not leave its window listeners behind.
  useEffect(() => () => endRef.current?.(), []);

  function begin(e: ReactPointerEvent<HTMLElement>, mode: "move" | "resize"): void {
    /* No test pins this guard — the suite's pointer helper hardcodes `button: 0`, so deleting
     * the line survives every assertion. It stops being equivalent the moment a case dispatches
     * a pointerdown with `button: 2` on a handle, which is the day a secondary click has to
     * leave the tile alone and let a context menu through instead of starting a silent drag
     * that no pointerup from that button will ever end. */
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
      /* Capture, and no test can tell: nothing inside the tile stops pointerdown propagating,
       * so raising on the bubble phase would run just as often and only the ORDER relative to
       * begin() would change, which nothing reads. It stops being equivalent the moment any
       * descendant calls stopPropagation on pointerdown — a pane header verb is the likely
       * one — because a bubble-phase raise never runs for a stopped event and that tile would
       * silently refuse to come to the front. */
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
