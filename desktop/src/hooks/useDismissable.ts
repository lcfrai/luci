// Outside-click, Escape and focus return for popovers, menus and the palette. Contract §6.7.
//
// The stack is module-level on purpose. §6.9 says Escape closes the TOPMOST overlay and does
// nothing when there is none; "topmost" is a property of all open overlays at once, which no
// single component can know. Listeners are attached only while the stack is non-empty, so with
// no overlay open Escape is not intercepted at all and reaches the terminal (§6.9).
import { useLayoutEffect, useRef } from "react";
import type { RefObject } from "react";

interface Entry {
  close: () => void;
}

const stack: Entry[] = [];

/** True while any overlay registered through useDismissable is open. Task 23's keyboard map
 *  reads this instead of tracking overlay state itself. */
export function hasOpenOverlay(): boolean {
  return stack.length > 0;
}

/** Closes the topmost overlay and returns true, or returns false when there is none. This is
 *  the ONLY implementation of §6.9's Escape ordering. */
export function closeTopOverlay(): boolean {
  const top = stack[stack.length - 1];
  if (top === undefined) return false;
  top.close();
  return true;
}

/** Test seam: module state survives vitest's restoreMocks/clearMocks (§1.6). */
export function __resetOverlays(): void {
  stack.length = 0;
}

/** Attach the returned ref to the overlay's outermost element. While `open`, a pointerdown
 *  outside it or Escape closes it, and on close DOM focus returns to whatever had it before —
 *  without which ⌘K, Escape, keep-typing is impossible (§6.8).
 *
 *  ONE CONTRACT FOR CALLERS, and it is two attributes, not one: the control that opens an
 *  overlay must carry `data-overlay-trigger`, must own the toggle itself, and must report that
 *  overlay's open state in `aria-expanded`. This listener is capture-phase on `document`, so it
 *  runs BEFORE the trigger's own click; without the attributes it closes the overlay and the
 *  click that follows re-opens it, giving a button that can never close what it opened.
 *  MachineChip's chip and the split button's caret are the two in this task; task 22's ⋯ button
 *  and task 23's palette launcher need both attributes too. */
export function useDismissable<T extends HTMLElement>(
  open: boolean,
  onClose: () => void,
  restoreFocus = true,
): RefObject<T | null> {
  const ref = useRef<T | null>(null);
  const onCloseRef = useRef(onClose);

  // After every render, before paint: keeps the latest callback without making it a dependency
  // of the effect below, which would push and pop the stack on every parent re-render.
  useLayoutEffect(() => {
    onCloseRef.current = onClose;
  });

  useLayoutEffect(() => {
    if (!open) return undefined;
    const restoreTo = document.activeElement as HTMLElement | null;
    const entry: Entry = { close: () => onCloseRef.current() };
    stack.push(entry);

    const isTop = (): boolean => stack[stack.length - 1] === entry;

    const onPointerDown = (event: PointerEvent): void => {
      if (!isTop()) return;
      const target = event.target;
      const el = ref.current;
      if (el !== null && target instanceof Node && el.contains(target)) return;
      // The control that opened THIS overlay is not "outside" it — but another overlay's
      // trigger is, and matching every [data-overlay-trigger] document-wide would treat them
      // as the same thing: with the machines popover open, a pointerdown on the split button's
      // caret would be exempted, the popover would stay open behind the picker with no
      // outside-click left to dismiss it, and task 23's chords — gated on hasOpenOverlay() —
      // would go quiet. So the exemption is scoped by aria-expanded, which the trigger already
      // carries for the screen reader: only the trigger of an OPEN overlay reports "true".
      const trigger =
        target instanceof Element ? target.closest("[data-overlay-trigger]") : null;
      if (trigger !== null && trigger.getAttribute("aria-expanded") === "true") return;
      onCloseRef.current();
    };
    const onKeyDown = (event: KeyboardEvent): void => {
      if (event.key !== "Escape" || !isTop()) return;
      event.preventDefault();
      event.stopPropagation();
      onCloseRef.current();
    };

    document.addEventListener("pointerdown", onPointerDown, true);
    document.addEventListener("keydown", onKeyDown, true);

    return () => {
      document.removeEventListener("pointerdown", onPointerDown, true);
      document.removeEventListener("keydown", onKeyDown, true);
      const index = stack.indexOf(entry);
      if (index >= 0) stack.splice(index, 1);
      if (restoreFocus && restoreTo !== null && document.contains(restoreTo) && !restoreTo.closest("[hidden], [inert], [aria-hidden='true']")) restoreTo.focus();
    };
  }, [open, restoreFocus]);

  return ref;
}
