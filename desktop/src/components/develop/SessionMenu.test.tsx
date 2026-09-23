// SessionMenu's own job is small and entirely about seams: resolve a DOMRect from an id, render
// the verb list, and own the one surface a verb cannot supply for itself — the rename input.
// Everything else is sessionVerbs, already green in its own suite.
//
// No jest-dom (shared-shape §3C): plain DOM property assertions only.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useState } from "react";
import type { JSX } from "react";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { IDS, aHarness, aSession, aWorktree } from "../../test/fixtures";
import { __resetOverlays } from "../../hooks/useDismissable";
import { resetCockpit, useCockpit } from "../../store/store";

const { killMock, renameMock } = vi.hoisted(() => ({ killMock: vi.fn(), renameMock: vi.fn() }));
vi.mock("../../api", () => ({ api: { killSession: killMock, renameSession: renameMock } }));

import { SessionMenu, anchorRectFor } from "./SessionMenu";
import { __resetSessionVerbs } from "./sessionVerbs";

const RECT = new DOMRect(40, 60, 0, 0);

/** A host that gives `onClose` its PRODUCTION consequence: in the real app `onClose` is
 *  `closeSessionMenu`, which nulls `sessionMenuFor`, and `ContextMenu` then stops rendering
 *  `<SessionMenu/>` at all.
 *
 *  WHY THIS EXISTS, and it is fix round 1's I5. This suite used a bare `vi.fn()` for `onClose`,
 *  which unmounts nothing — and that inert spy is what hid a DEAD VERB from the first round:
 *  Rename was wrapped in the close-then-run wrapper, so in the real app the menu closed before
 *  the rename input could render, and every test here stayed green because the spy let the
 *  component live. The product was fixed; the spy that hid it was not, and it would have hidden
 *  the next one. Any test whose subject is a verb that closes the menu uses this. */
function Host(props: { onClose?: () => void }): JSX.Element | null {
  const [open, setOpen] = useState(true);
  if (!open) return null;
  return (
    <SessionMenu
      sessionId={IDS.session}
      anchor={RECT}
      onClose={() => {
        setOpen(false);
        props.onClose?.();
      }}
    />
  );
}

beforeEach(() => {
  resetCockpit();
  __resetSessionVerbs();
  // Task 18's overlay stack is module state and survives restoreMocks, exactly like the
  // pending registry. A leaked entry makes the NEXT test's Escape close the wrong overlay.
  __resetOverlays();
  killMock.mockReset();
  renameMock.mockReset();
  useCockpit.getState().setHarnesses([aHarness({ id: "claude-code", name: "Claude Code" })]);
  useCockpit.getState().setWorktrees(IDS.branch, [aWorktree()]);
  useCockpit.getState().upsertSession(aSession({ status: "running" }));
});

afterEach(() => {
  // A Kill leaves a live 6 s deferred timer behind; clearing it here keeps it from firing into
  // the next file rather than relying on the next beforeEach to catch it.
  __resetSessionVerbs();
  // task 14's setup.ts already calls RTL's cleanup() in a global afterEach; this clears the
  // elements the anchorRectFor tests append by hand, which cleanup() does not own.
  document.body.innerHTML = "";
});

describe("anchorRectFor", () => {
  it("prefers the pane's ⋯ button", () => {
    const button = document.createElement("button");
    button.setAttribute("data-session-menu-anchor", IDS.session);
    button.getBoundingClientRect = () => new DOMRect(10, 20, 24, 24);
    document.body.append(button);
    expect(anchorRectFor(IDS.session).x).toBe(10);
  });

  it("falls back to the rail row when no pane is on screen", () => {
    const row = document.createElement("div");
    row.setAttribute("data-kind", "session");
    row.setAttribute("data-id", IDS.session);
    row.getBoundingClientRect = () => new DOMRect(200, 300, 180, 24);
    document.body.append(row);
    expect(anchorRectFor(IDS.session).x).toBe(200);
  });

  it("falls back to a centred point when neither exists, rather than returning null", () => {
    const rect = anchorRectFor(IDS.session);
    expect(rect.width).toBe(0);
    expect(rect.x).toBe(Math.round(window.innerWidth / 2));
  });
});

describe("SessionMenu", () => {
  it("renders the lifecycle verbs and closes on a verb", () => {
    const onClose = vi.fn();
    render(<SessionMenu sessionId={IDS.session} anchor={RECT} onClose={onClose} />);

    // jest-dom is NOT installed (shared-shape §3C, and task 14 says so): read the DOM property.
    const kill = screen.getByRole("menuitem", { name: "Kill" }) as HTMLButtonElement;
    const forget = screen.getByRole("menuitem", { name: "Forget" }) as HTMLButtonElement;
    expect(kill.disabled).toBe(false);
    expect(forget.disabled).toBe(true);

    fireEvent.click(kill);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  // Rendered through Host, not with a bare spy: this is the exact test that stayed green while
  // Rename was a dead verb in the real application (fix round 1, I5).
  it("Rename… swaps the list for an input seeded with the current title, and submits it", async () => {
    renameMock.mockResolvedValue(aSession({ title: "retention rules" }));
    render(<Host />);

    fireEvent.click(screen.getByRole("menuitem", { name: "Rename…" }));
    // The assertion that the inert spy could not make: the menu is STILL MOUNTED. If Rename ever
    // goes back through the close-then-run wrapper, Host unmounts it and this line fails.
    const input = screen.getByRole("textbox", { name: "Session title" }) as HTMLInputElement;
    expect(input.value).toBe("Evidence upload");
    expect(screen.queryByRole("menuitem", { name: "Kill" })).toBeNull();

    fireEvent.change(input, { target: { value: "retention rules" } });
    fireEvent.submit(input.closest("form") as HTMLFormElement);
    expect(renameMock).toHaveBeenCalledWith(IDS.session, "retention rules");
    // Flush the immediate commit's microtask, so the server's answer lands inside act() rather
    // than after the test has finished — an unactioned store write is a warning, not a failure,
    // and warnings are how a real ordering bug hides.
    await act(async () => {
      await Promise.resolve();
    });
    expect(useCockpit.getState().sessions[IDS.session]?.title).toBe("retention rules");
  });

  // Submitting DOES close the menu, and nothing asserted it before: the mutant that deletes
  // `onClose()` from `submit` survived round 1. With a spy there is nothing to observe; with
  // Host the surface actually goes away.
  it("submitting the rename closes the menu", async () => {
    renameMock.mockResolvedValue(aSession({ title: "retention rules" }));
    const onClose = vi.fn();
    render(<Host onClose={onClose} />);

    fireEvent.click(screen.getByRole("menuitem", { name: "Rename…" }));
    const input = screen.getByRole("textbox", { name: "Session title" }) as HTMLInputElement;
    fireEvent.change(input, { target: { value: "retention rules" } });
    fireEvent.submit(input.closest("form") as HTMLFormElement);
    await act(async () => {
      await Promise.resolve();
    });

    expect(onClose).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole("textbox", { name: "Session title" })).toBeNull();
    expect(screen.queryByRole("menuitem", { name: "Kill" })).toBeNull();
  });

  // The other half of the same rule: a verb that leaves DOES take the menu with it. Without this
  // a Host-rendered suite could pass with nothing ever closing.
  it("a verb that is not Rename closes the menu on its way out", () => {
    const onClose = vi.fn();
    render(<Host onClose={onClose} />);

    fireEvent.click(screen.getByRole("menuitem", { name: "Copy session id" }));

    expect(onClose).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole("menuitem", { name: "Kill" })).toBeNull();
  });

  it("Escape in the rename input returns to the verb list and closes nothing else", () => {
    const onClose = vi.fn();
    render(<Host onClose={onClose} />);
    fireEvent.click(screen.getByRole("menuitem", { name: "Rename…" }));
    fireEvent.keyDown(screen.getByRole("textbox", { name: "Session title" }), { key: "Escape" });

    expect(screen.queryByRole("menuitem", { name: "Kill" })).not.toBeNull();
    expect(renameMock).not.toHaveBeenCalled();
    // The assertion that actually pins the behaviour: the menu is still open. Without the
    // rename's own useDismissable entry sitting ABOVE the Popover's on task 18's stack, this
    // Escape closes the whole menu — useDismissable listens on document in the CAPTURE phase
    // and stops propagation itself, so a React onKeyDown on the input would never run.
    expect(onClose).not.toHaveBeenCalled();
  });

  it("renders nothing when the anchor is null", () => {
    const { container } = render(
      <SessionMenu sessionId={IDS.session} anchor={null} onClose={vi.fn()} />,
    );
    expect(container.childNodes.length).toBe(0);
  });
});
