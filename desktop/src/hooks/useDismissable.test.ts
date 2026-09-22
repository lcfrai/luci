// The overlay stack. Before this file existed the whole module could be replaced with a stub
// that returns a bare ref — no stack, no listeners, no focus return — and all 28 tests still
// passed. It is the SOLE implementation of contract §6.9's Escape ordering, and tasks 22 and
// 23 are instructed in writing to delegate to it rather than re-derive it, so a defect here is
// a defect in three surfaces with nothing objecting.
//
// No jest-dom (shared-shape §3C): plain vitest assertions against DOM properties only.
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, render } from "@testing-library/react";
import { createElement, useState } from "react";
import type { JSX } from "react";
import {
  __resetOverlays,
  closeTopOverlay,
  hasOpenOverlay,
  useDismissable,
} from "./useDismissable";

afterEach(() => {
  __resetOverlays();
});

/** A minimal overlay. `createElement` rather than JSX so this stays a .ts file alongside the
 *  module it tests; the shape under test is the hook, not any markup. */
function Overlay(props: { onClose: () => void; children?: string }): JSX.Element | null {
  const [open, setOpen] = useState(true);
  const ref = useDismissable<HTMLDivElement>(open, () => {
    setOpen(false);
    props.onClose();
  });
  if (!open) return null;
  // tabIndex -1 so it can actually HOLD focus, which is what Popover does. Without it the
  // focus-return test is vacuous: focus never leaves the previously-focused element, so the
  // assertion passes whether or not the hook restores anything. Measured — the mutant that
  // deletes the restore survived until this was added.
  return createElement(
    "div",
    { ref, tabIndex: -1, "data-testid": "overlay" },
    props.children ?? "overlay",
  );
}

/** Dispatches a real capture-phase-visible pointerdown, which is what the hook listens for.
 *  fireEvent.pointerDown does not exist for a raw element outside RTL's event map here, so the
 *  event is constructed directly — and it must BUBBLE, or a capture listener on document
 *  never sees it. */
function pointerDownOn(target: Element): void {
  act(() => {
    target.dispatchEvent(new MouseEvent("pointerdown", { bubbles: true, cancelable: true }));
  });
}

function escape(): void {
  act(() => {
    document.dispatchEvent(
      new KeyboardEvent("keydown", { key: "Escape", bubbles: true, cancelable: true }),
    );
  });
}

describe("the overlay stack", () => {
  // Case 1. The branch that decides whether Escape reaches the terminal. §6.9: "close the
  // topmost overlay; if none, DO NOTHING" — and task 23 gates its chords on this returning
  // false, so a stack that lied here would silently disable them.
  it("closeTopOverlay does nothing and reports false with an empty stack", () => {
    expect(hasOpenOverlay()).toBe(false);
    expect(closeTopOverlay()).toBe(false);
    expect(hasOpenOverlay()).toBe(false);
  });

  it("reports an open overlay, and forgets it again on unmount", () => {
    const { unmount } = render(createElement(Overlay, { onClose: () => {} }));
    expect(hasOpenOverlay()).toBe(true);
    unmount();
    expect(hasOpenOverlay()).toBe(false);
  });

  // Case 2. TOPMOST only. Two overlays, one Escape, exactly one close.
  it("Escape closes only the topmost overlay, not the one beneath it", () => {
    const first = vi.fn();
    const second = vi.fn();
    render(createElement(Overlay, { onClose: first, children: "first" }));
    render(createElement(Overlay, { onClose: second, children: "second" }));
    expect(hasOpenOverlay()).toBe(true);

    escape();
    expect(second).toHaveBeenCalledTimes(1);
    expect(first).not.toHaveBeenCalled();

    // And the one beneath is still registered, so a second Escape reaches it.
    escape();
    expect(first).toHaveBeenCalledTimes(1);
    expect(hasOpenOverlay()).toBe(false);
  });

  // Case 3. Both directions of the containment test. Inside must NOT close, or every click on
  // a menu item would dismiss the menu before the item ran.
  it("a pointerdown outside closes, and one inside does not", () => {
    const onClose = vi.fn();
    const { getByTestId } = render(createElement(Overlay, { onClose }));

    pointerDownOn(getByTestId("overlay"));
    expect(onClose).not.toHaveBeenCalled();

    pointerDownOn(document.body);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  // Case 4. The subtlest logic in the task. The exemption is scoped by aria-expanded, NOT by
  // the presence of the data attribute: the trigger of an OPEN overlay owns its own toggle and
  // must be exempt, but ANOTHER overlay's trigger is genuinely outside and must dismiss. The
  // unscoped form — `closest("[data-overlay-trigger]") !== null` — is the obvious thing to
  // write and it leaves an overlay stuck open behind a second one with no outside-click left.
  it("exempts the trigger of an OPEN overlay, and not a trigger reporting closed", () => {
    const onClose = vi.fn();
    render(createElement(Overlay, { onClose }));

    const mine = document.createElement("button");
    mine.setAttribute("data-overlay-trigger", "");
    mine.setAttribute("aria-expanded", "true");
    document.body.appendChild(mine);

    const other = document.createElement("button");
    other.setAttribute("data-overlay-trigger", "");
    other.setAttribute("aria-expanded", "false");
    document.body.appendChild(other);

    pointerDownOn(mine);
    expect(onClose).not.toHaveBeenCalled();

    pointerDownOn(other);
    expect(onClose).toHaveBeenCalledTimes(1);

    mine.remove();
    other.remove();
  });

  // Case 5. Focus return, without which "Cmd-K, Escape, keep typing" is impossible (§6.8).
  it("returns focus to whatever had it before the overlay opened", () => {
    const before = document.createElement("input");
    document.body.appendChild(before);
    before.focus();
    expect(document.activeElement).toBe(before);

    const { getByTestId, unmount } = render(createElement(Overlay, { onClose: () => {} }));
    // Focus must genuinely LEAVE the input, or the assertion below cannot tell a restore from
    // a no-op. This mirrors Popover, which focuses its own element once it has measured.
    act(() => getByTestId("overlay").focus());
    expect(document.activeElement).not.toBe(before);

    unmount();
    expect(document.activeElement).toBe(before);
    before.remove();
  });
});
