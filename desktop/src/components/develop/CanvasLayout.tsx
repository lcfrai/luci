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
import type { JSX, KeyboardEvent, PointerEvent as ReactPointerEvent } from "react";
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

/** The z-stack's one updater: `id` moves to the top. Added by task 23's fix round 1, under
 *  carry 1's existing departure, and exported so its contract can be pinned outside rendering.
 *
 *  The id ALREADY on top returns `prev` ITSELF — the same reference, not an equal copy. A click
 *  raises twice (the tile's capture-phase raise, then the focus effect's echo), and React bails
 *  out of the echo only because the state it gets back is the state it had. A fresh array with
 *  the same contents defeats that and re-renders every tile on every click. Reference identity
 *  is asserted directly (CanvasLayout.test.tsx), so no memoisation of the tiles can hide it. */
export function raiseToTop(prev: SessionId[], id: SessionId): SessionId[] {
  return prev[prev.length - 1] === id ? prev : [...prev.filter((x) => x !== id), id];
}

export function CanvasLayout({ sessionIds, singleSessionId }: { sessionIds: SessionId[]; singleSessionId?: SessionId }): JSX.Element {
  const paneIds = useCockpit(useShallow(selectPaneSessionIds));
  const tiles = useCockpit((s) => s.tiles);
  const setTile = useCockpit((s) => s.setTile);
  const pinSession = useCockpit((s) => s.pinSession);
  const unpinSession = useCockpit((s) => s.unpinSession);
  const focusedId = useCockpit((s) => s.selection.sessionId);
  const [stack, setStack] = useState<SessionId[]>([]);

  /* Decision (a), as amended twice. Nothing in Group D renders a "pin to canvas" verb, so an
   * unseeded canvas stays blank for ever and the layout toggle reads as broken.
   *
   * `seen` is what separates "this id is NEW" from "this id is merely absent", and the two
   * earlier versions of this effect each got one of those wrong: topping up unconditionally
   * re-pinned whatever the user had just unpinned, and seeding only an empty canvas meant a
   * session created while Canvas was open never got a tile at all. The second is the worse
   * failure — a new session that silently does not appear — which is why this is not that.
   *
   * Same device, same reason, as ColumnsLayout's defect-4 ref four files away; the two layouts
   * now answer "is this id new?" the same way instead of each inventing a rule.
   *
   *   - an id NOT in `seen` is new → it gets a tile, and joins `seen`.
   *   - an id the user UNPINS stays in `seen` → it is never pinned again.
   *
   * Residual, because `pinned` cannot tell "never pinned" from "deliberately unpinned" without
   * a persisted flag, and that is a schema change at version 1. `seen` lives as long as this
   * component and ACCUMULATES across branches, so an unpin survives a branch switch — but not
   * a flip to Columns and back, nor a restart, either of which remounts and re-initialises it
   * from `pinned`. It dies the moment task 22 ships a pin verb.
   *
   * Room is computed before pinning rather than letting pinSession no-op, because pinSession
   * toasts the cap (§7.7) and a toast on mount the user cannot act on is worse than the blank
   * plane it replaces. `Math.max` keeps a negative room from slicing off the END of the list if
   * the fast path above is ever removed. `paneIds` is the scope: the cap counts what Canvas
   * SHOWS, which is this branch. */
  const seen = useRef<Set<SessionId> | null>(null);

  useEffect(() => {
    if (seen.current === null) seen.current = new Set(sessionIds);
    const known = seen.current;
    const room = CANVAS_MAX_TILES - sessionIds.length;
    if (room <= 0) return;
    const fresh = paneIds.filter((id) => !known.has(id)).slice(0, Math.max(room, 0));
    for (const id of fresh) {
      known.add(id);
      pinSession(id, paneIds);
    }
  }, [paneIds, sessionIds, pinSession]);

  /* pinSession enforces the cap, but a pin list persisted by an older build rehydrates without
   * passing through it. Render the cap, not the list. */
  const visible = sessionIds.slice(0, CANVAS_MAX_TILES);
  const singleId = singleSessionId ? (visible.includes(singleSessionId) ? singleSessionId : visible[0]) : undefined;

  const raise = useCallback((id: SessionId): void => {
    setStack((prev) => raiseToTop(prev, id));
  }, []);

  /* A focus that arrives without a pointer raises its tile too. ⌘1-9, ⌘\, a palette row and a
   * rail click all move focus without touching the plane, and a focused tile left under its
   * neighbours reads as a key that did nothing. Declared departure by task 23: the canvas owns
   * its z-order, so the keyboard map writes focus and never learns this stack exists.
   *
   * Keyed on a PRIMITIVE that is null unless the focused session has a tile. `visible` is a
   * fresh array every render, so depending on it would re-raise the focused tile on every 2 s
   * poll, burying whichever tile the user last dragged. An id absent from `visible` must not
   * enter the stack either — `sessionIds` is branch-filtered, so a session focused from another
   * branch's rail row is absent, and a stray entry surfaces raised when that branch opens. And
   * because the key stays null until the tile exists, a focused session is raised the moment
   * it GAINS one: ⌘⏎ on Canvas focuses first, and the pin that gives it a tile lands a render
   * later.
   *
   * A click raises twice: the tile's capture-phase raise, then this effect when the pane's
   * pointerdown moves focus. The second is a no-op only because `raiseToTop` returns `prev`
   * itself for the tile already on top; drop that check and every click writes the same stack
   * twice. */
  const focusedTile = focusedId !== null && visible.includes(focusedId) ? focusedId : null;
  useEffect(() => {
    if (focusedTile !== null) raise(focusedTile);
  }, [focusedTile, raise]);

  const commit = useCallback(
    (id: SessionId, next: TilePosition): void => {
      setTile(id, next);
    },
    [setTile],
  );

  return (
    <div className={`cv-plane${singleSessionId ? ' cv-plane--single' : ''}`} data-testid="cv-plane">
      <div className="cv-sheet" style={{ width: PLANE_W, height: PLANE_H }}>
        {visible.map((id, i) => {
          const raised = stack.indexOf(id);
          return (
            /* `key={id}`, never the index, and it is load-bearing rather than idiomatic: a
             * positional key makes React re-use the tile at slot N for whatever session lands
             * there, which unmounts one pane and mounts another in the same DOM slot and
             * re-parents the registry's element on every pin-order change. Keyed by session,
             * a re-order MOVES the node and the rendered scrollback goes with it. Pinned by
             * "keeps a tile's DOM node when the pin order changes". */
            <CanvasTile
              key={id}
              sessionId={id}
              position={tiles[id] ?? defaultTilePosition(i)}
              z={raised === -1 ? i + 1 : CANVAS_MAX_TILES + 1 + raised}
              onRaise={raise}
              onCommit={commit}
              onUnpin={unpinSession}
              hidden={!!singleId && id !== singleId}
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
  onUnpin: (id: SessionId) => void;
  hidden?: boolean;
}

function samePosition(a: TilePosition, b: TilePosition): boolean {
  return a.x === b.x && a.y === b.y && a.w === b.w && a.h === b.h;
}

/* The inner functions below are `function` declarations rather than const arrows on purpose:
 * `end` names `onUp` and `onUp` names `end`, and declarations hoist, so neither ordering is a
 * temporal-dead-zone trap. */
function CanvasTile({
  sessionId,
  position,
  z,
  onRaise,
  onCommit,
  onUnpin,
  hidden = false,
}: TileProps): JSX.Element {
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
    /* Clamped on the way IN, not only on the way out. A tile persisted against a larger plane —
     * a wider monitor, or a future PLANE_W — rehydrates with its far edge outside the sheet and
     * would otherwise render off-screen with no handle to drag it back by. Clamping here puts
     * it on the plane immediately; the first gesture then persists the correction, so this
     * needs no store write on mount. "move" is the right gesture: a rehydrate fixes the size
     * and moves the position, exactly as a drag does. */
    const landed = clampTile(position, "move");
    live.current = landed;
    apply(landed);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [position.x, position.y, position.w, position.h]);

  // A tile unmounted mid-gesture must not leave its window listeners behind.
  useEffect(() => () => endRef.current?.(), []);

  function adjustByKeyboard(event: KeyboardEvent<HTMLElement>, mode: 'move' | 'resize') {
    if (event.target !== event.currentTarget || !['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown'].includes(event.key) || gesture) return;
    event.preventDefault(); event.stopPropagation();
    const step = event.shiftKey ? 40 : 10;
    const dx = event.key === 'ArrowLeft' ? -step : event.key === 'ArrowRight' ? step : 0;
    const dy = event.key === 'ArrowUp' ? -step : event.key === 'ArrowDown' ? step : 0;
    const current = live.current;
    const next = clampTile(mode === 'move' ? { ...current, x: current.x + dx, y: current.y + dy }
      : { ...current, w: current.w + dx, h: current.h + dy }, mode);
    live.current = next; apply(next); onRaise(sessionId); onCommit(sessionId, next);
  }

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
    /* The gesture belongs to ONE pointer. Without this, a second finger landing anywhere while
     * a drag is in flight feeds its own coordinates to this handler and the tile teleports
     * between the two touch points. This is what declared survivor #2 — the missing
     * `button !== 0` coverage — was hiding: both are "the handler accepts an input it should
     * have refused", and only one of them had been noticed. */
    const { pointerId } = e;
    setGesture(mode);

    /* `in` rather than a bare compare, so the two cases are explicit rather than accidental:
     * a real PointerEvent always carries the id and is filtered, while a plain MouseEvent has
     * no `pointerId` on its prototype at all and is let through rather than compared against
     * `undefined`. */
    function fromAnotherPointer(ev: Event): boolean {
      return "pointerId" in ev && (ev as PointerEvent).pointerId !== pointerId;
    }

    function onMove(ev: MouseEvent): void {
      if (fromAnotherPointer(ev)) return;
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

    function onUp(ev: Event): void {
      if (fromAnotherPointer(ev)) return;
      end();
      /* The ONE store write of the whole gesture, and only if the gesture moved something. A
       * bare click on the grip — which is every click that lands on it, since the grip is not
       * a button — otherwise runs the whole `persist` middleware to write back the position the
       * tile already had. */
      const next = live.current;
      if (samePosition(next, origin)) return;
      onCommit(sessionId, next);
    }

    function end(): void {
      window.removeEventListener("pointermove", onMove);
      window.removeEventListener("pointerup", onUp);
      window.removeEventListener("pointercancel", onUp);
      window.removeEventListener("blur", onBlur);
      endRef.current = null;
      setGesture(null);
    }

    /* A window that loses focus mid-drag never delivers the pointerup — alt-tab, a native
     * dialog, the OS taking the pointer — and the gesture would otherwise stay armed, so the
     * next pointermove over the plane drags a tile nobody is holding. Blur carries no
     * pointerId, hence the separate entry point rather than reusing onUp's filter. */
    function onBlur(): void {
      end();
      const next = live.current;
      if (samePosition(next, origin)) return;
      onCommit(sessionId, next);
    }

    endRef.current = end;
    // Listeners on window, not setPointerCapture: capture is unimplemented in jsdom and a
    // release that never fires would wedge the pointer in a real webview too.
    window.addEventListener("pointermove", onMove);
    window.addEventListener("pointerup", onUp);
    window.addEventListener("pointercancel", onUp);
    window.addEventListener("blur", onBlur);
  }

  return (
    <div
      ref={tileRef}
      className="cv-tile"
      data-cv-tile={sessionId}
      hidden={hidden}
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
        tabIndex={0}
        aria-label={`Move ${title}`}
        title="Move with arrow keys; hold Shift for larger steps"
        onKeyDown={event => adjustByKeyboard(event, 'move')}
        onPointerDown={(e) => begin(e, "move")}
      >
        <span className="cv-grip-bar" aria-hidden="true" />
        <span className="cv-coords" ref={coordsRef} aria-hidden="true" />
        {/* The cap's toast says "Unpin one to add another" and, before this, nothing in the
            tree called unpinSession — the product instructed an action it could not perform.
            stopPropagation on pointerdown is what stops a click here starting a drag, because
            the grip's own onPointerDown is this element's parent. Note the tile still comes to
            the front: onRaise is on the CAPTURE phase, which a descendant cannot intercept —
            that is the expiry condition declared on it, now actually exercised. */}
        <button
          type="button"
          className="cv-unpin"
          aria-label={`Unpin ${title}`}
          onPointerDown={(e) => e.stopPropagation()}
          onClick={() => onUnpin(sessionId)}
        >
          <span aria-hidden="true">×</span>
        </button>
      </div>
      <div className="cv-tile-body">
        <SessionPane sessionId={sessionId} variant="tile" />
      </div>
      <div
        className="cv-resize"
        role="button"
        tabIndex={0}
        aria-label={`Resize ${title}`}
        title="Resize with arrow keys; hold Shift for larger steps"
        onKeyDown={event => adjustByKeyboard(event, 'resize')}
        onPointerDown={(e) => begin(e, "resize")}
      />
    </div>
  );
}
