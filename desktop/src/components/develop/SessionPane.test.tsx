// Contract §6.5, §6.8, and known defect 3. Every behaviour here is one a later refactor would
// otherwise quietly delete: an input surface for every status, focus intent split three ways,
// an exited pane that keeps its transcript, and a footer asserted by what it does NOT contain.
//
// No jest-dom (shared-shape §3C): plain DOM property assertions only.
import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SessionPane, startSessionOnWorktree } from "./SessionPane";
import { ApiError } from "../../api/client";
import { attachLabel } from "./SessionPaneFooter";
import { statusLabel } from "./SessionPaneHeader";
import { resetCockpit, useCockpit } from "../../store/store";
import { aHarness, aSession, aWorktree } from "../../test/fixtures";
import type { Session, SessionStatus } from "../../store/types";
import type { AttachState } from "../../api/attachRegistry";

// §1.5: a component test never renders a glyph. The stand-in records the two props
// SessionPane passes, and carries xterm's own focus target — handing DOM focus to that
// element is the behaviour under test (§6.8).
vi.mock("../Terminal", () => ({
  Terminal: ({ sessionId, readOnly }: { sessionId: string; readOnly: boolean }) => (
    <div data-testid="terminal" data-session={sessionId} data-readonly={String(readOnly)}>
      <textarea className="xterm-helper-textarea" data-testid="xterm" readOnly />
    </div>
  ),
}));

// vi.hoisted, not a bare const: vi.mock is hoisted above the imports, and the factory runs
// while ./SessionPane's import graph is evaluated — before a top-level `const` has been
// initialised. A bare const here throws "Cannot access 'createSessionMock' before
// initialization" and looks like a mocking bug rather than a TDZ one.
const { createSessionMock } = vi.hoisted(() => ({ createSessionMock: vi.fn() }));
vi.mock("../../api", () => ({ api: { createSession: createSessionMock } }));

// Only `useAttachState` is replaced; everything else in the registry is the real module. The
// registry publishes a non-empty `reason` only for a socket that actually failed, which no
// component test can reach without opening one — and the footer's obligation for that state
// (the reason goes in `title`, never in the body) is exactly the part worth pinning.
const { attachState } = vi.hoisted(() => ({
  attachState: {
    current: { phase: "detached", attempts: 0, ladderStartedAt: 0, reason: "" } as AttachState,
  },
}));
vi.mock("../../api/attachRegistry", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../api/attachRegistry")>()),
  useAttachState: () => attachState.current,
}));

const HARNESS = aHarness({
  id: "claude-code",
  name: "Claude Code",
  command: "claude",
  available: true,
  models: [{ id: "opus", name: "Opus" }],
  defaultModel: "opus",
  efforts: ["", "high"],
  permissionModes: ["ask", "plan"],
});

const WORKTREE = aWorktree({ id: "w-1", projectId: "p-1", branchId: "b-1", name: "evidence-upload" });

/** 41 minutes and 30 seconds old, so formatElapsed prints "41m" and the second boundary
 *  cannot flip it mid-run. */
function seed(over: Partial<Session> = {}): Session {
  const session = aSession({
    id: "s-1",
    title: "Claude Code · evidence-upload",
    projectId: "p-1",
    branchId: "b-1",
    worktreeId: "w-1",
    harness: "claude-code",
    model: "opus",
    effort: "high",
    permissionMode: "ask",
    status: "running",
    exitReason: "",
    createdAt: new Date(Date.now() - 41 * 60_000 - 30_000).toISOString(),
    ...over,
  });
  useCockpit.setState({
    harnesses: [HARNESS],
    worktreesByBranch: { "b-1": [WORKTREE] },
    sessions: { [session.id]: session },
    sessionIdsByWorktree: { "w-1": [session.id] },
  });
  return session;
}

describe("SessionPane", () => {
  beforeEach(() => {
    resetCockpit();
    createSessionMock.mockReset();
    attachState.current = { phase: "detached", attempts: 0, ladderStartedAt: 0, reason: "" };
  });

  it("renders a terminal for starting, running, idle and exited", () => {
    const statuses: SessionStatus[] = ["starting", "running", "idle", "exited"];
    for (const status of statuses) {
      resetCockpit();
      const session = seed({ status });
      const view = render(<SessionPane sessionId={session.id} variant="column" />);
      expect(screen.getByTestId("terminal").getAttribute("data-session")).toBe("s-1");
      view.unmount();
    }
  });

  it("wires stdin for every status except exited", () => {
    const cases: [SessionStatus, string][] = [
      ["starting", "false"],
      ["running", "false"],
      ["idle", "false"],
      ["exited", "true"],
    ];
    for (const [status, readOnly] of cases) {
      resetCockpit();
      const session = seed({ status });
      const view = render(<SessionPane sessionId={session.id} variant="column" />);
      expect(screen.getByTestId("terminal").getAttribute("data-readonly")).toBe(readOnly);
      view.unmount();
    }
  });

  it("a pointer on the pane chrome focuses the session and hands DOM focus to the terminal", async () => {
    const session = seed();
    render(<SessionPane sessionId={session.id} variant="column" />);

    await userEvent.click(screen.getByRole("region", { name: /evidence-upload/ }));

    expect(useCockpit.getState().selection.sessionId).toBe("s-1");
    expect(document.activeElement).toBe(screen.getByTestId("xterm"));
  });

  // Exception 2 (§6.8). xterm owns DOM focus AND the selection drag inside its own subtree, and
  // it has already focused itself by the time this event finishes bubbling; calling focus()
  // here fights a drag in progress. Distinguishable from the chrome case only because the two
  // land on different elements, which is why this is a separate test rather than a comment.
  it("a pointer inside the terminal sets the selection and leaves DOM focus to xterm", async () => {
    const session = seed();
    render(<SessionPane sessionId={session.id} variant="column" />);

    await userEvent.click(screen.getByTestId("terminal"));

    expect(useCockpit.getState().selection.sessionId).toBe("s-1");
    expect(document.activeElement).not.toBe(screen.getByTestId("xterm"));
  });

  // Exception 3. A purge that raced a render is rare and recoverable; a "Cannot read properties
  // of undefined" in a pane is not.
  it("renders a named frame rather than throwing when the session is gone", () => {
    seed();
    const { container } = render(<SessionPane sessionId="s-purged" variant="column" />);

    const gone = container.querySelector(".pane--gone");
    expect(gone).not.toBeNull();
    expect(gone?.textContent).toBe("This session is no longer listed.");
    expect(screen.queryByTestId("terminal")).toBeNull();
  });

  it("the ⋯ button focuses the session, opens its menu, and does NOT steal DOM focus", async () => {
    const session = seed();
    render(<SessionPane sessionId={session.id} variant="column" />);

    const more = screen.getByRole("button", { name: /session menu/i });
    await userEvent.click(more);

    expect(useCockpit.getState().selection.sessionId).toBe("s-1");
    expect(useCockpit.getState().sessionMenuFor).toBe("s-1");
    expect(document.activeElement).not.toBe(screen.getByTestId("xterm"));
    // Task 22 resolves the menu's DOMRect from this attribute rather than editing this file.
    expect(more.getAttribute("data-session-menu-anchor")).toBe("s-1");
  });

  // useDismissable's contract is TWO attributes, not one: the trigger is exempted from the
  // capture-phase pointerdown only when it carries data-overlay-trigger AND reports
  // aria-expanded="true". Without both, the pointerdown closes task 22's menu and the click
  // that follows re-opens it, giving a button that can never close what it opened.
  it("the ⋯ button is a real overlay trigger and can close what it opened", async () => {
    const session = seed();
    render(<SessionPane sessionId={session.id} variant="column" />);

    const more = screen.getByRole("button", { name: /session menu/i });
    expect(more.hasAttribute("data-overlay-trigger")).toBe(true);
    expect(more.getAttribute("aria-haspopup")).toBe("menu");
    expect(more.getAttribute("aria-expanded")).toBe("false");

    await userEvent.click(more);
    expect(more.getAttribute("aria-expanded")).toBe("true");

    await userEvent.click(more);
    expect(useCockpit.getState().sessionMenuFor).toBeNull();
    expect(more.getAttribute("aria-expanded")).toBe("false");
  });

  it("an exited pane keeps its scrollback, prints exitReason verbatim and offers Restart", async () => {
    const session = seed({ status: "exited", exitReason: "idle timeout" });
    createSessionMock.mockResolvedValue(
      aSession({ id: "s-2", worktreeId: "w-1", branchId: "b-1", status: "starting" }),
    );
    render(<SessionPane sessionId={session.id} variant="column" />);

    expect(screen.getByTestId("terminal").getAttribute("data-readonly")).toBe("true");
    expect(screen.getByText("idle timeout")).toBeDefined();

    await userEvent.click(screen.getByRole("button", { name: /restart/i }));

    // Restart reuses the dead session's own settings, not the resolved defaults: the user
    // chose them once and a restart is not a chance to silently change them.
    expect(createSessionMock).toHaveBeenCalledWith({
      worktreeId: "w-1",
      harness: "claude-code",
      model: "opus",
      effort: "high",
      permissionMode: "ask",
    });
    await waitFor(() => expect(useCockpit.getState().sessions["s-2"]).toBeDefined());
    expect(useCockpit.getState().selection.sessionId).toBe("s-2");
    // The dead session stays listed: the transcript is the artifact.
    expect(useCockpit.getState().sessions["s-1"]).toBeDefined();
  });

  it("the footer names four facts and invents no fifth", () => {
    const session = seed();
    const { container } = render(<SessionPane sessionId={session.id} variant="column" />);

    const foot = container.querySelector(".pane-foot");
    expect(foot).not.toBeNull();
    const text = (foot?.textContent ?? "").toLowerCase();

    // Exact, not four `toContain`s. A contains-assertion on "claude" is satisfied by
    // session.harness ("claude-code") just as well as by harness.command ("claude"), so it
    // cannot tell the fact the footer claims from the id standing in for it while the
    // catalogue loads — and the point of this test is that each of the four is the named
    // field. Equality also fails if a fifth fact is appended, which no negative list can.
    expect(text).toBe("claude · opus · 41m · detached");
    //                 ^ harness.command
    //                          ^ session.model
    //                                 ^ formatElapsed(session.createdAt, useHeartbeat())
    //                                       ^ AttachState.phase, nothing attached here

    // docs/AGENTIC-UX.md §2 property 7: no element whose justifying fact cannot be named.
    // A PTY carries no spend, no token accounting and no context window.
    for (const invented of ["$", "token", "context", "%", "cost"]) {
      expect(text).not.toContain(invented);
    }
  });

  // Exported so a test can name the sentence rather than match prose. "exited" alone does not
  // tell a user whether they killed it or it fell over, which is why the reason rides along.
  it("names the dot's status, and carries exitReason when there is one", () => {
    expect(statusLabel(aSession({ status: "running" }))).toBe("running");
    expect(statusLabel(aSession({ status: "exited", exitReason: "" }))).toBe("exited");
    expect(statusLabel(aSession({ status: "exited", exitReason: "killed" }))).toBe(
      "exited — killed",
    );
  });

  // Contract §6.6 and §7.7: the registry publishes the RAW ladder stamp and owns no clock, so
  // the pane is what turns it into a number. `attempts` cannot supply an elapsed, and §7.7 asks
  // for "the elapsed retry time, not a spinner".
  it("prints the retry elapsed off the one clock, not a retry count", () => {
    const now = 1_700_000_000_000;
    expect(
      attachLabel({ phase: "reconnecting", attempts: 3, ladderStartedAt: now - 12_000, reason: "" }, now),
    ).toBe("reconnecting 12s");
    expect(
      attachLabel({ phase: "connecting", attempts: 1, ladderStartedAt: now - 3_000, reason: "" }, now),
    ).toBe("connecting 3s");
    // No ladder running, so there is no elapsed to print and none is invented.
    expect(attachLabel({ phase: "attached", attempts: 0, ladderStartedAt: 0, reason: "" }, now)).toBe(
      "attached",
    );
    // "failed" reads as "not attached": the socket failed, the agent did not.
    expect(
      attachLabel({ phase: "failed", attempts: 9, ladderStartedAt: 0, reason: "gave up" }, now),
    ).toBe("not attached");
    expect(attachLabel({ phase: "ended", attempts: 0, ladderStartedAt: 0, reason: "" }, now)).toBe(
      "ended",
    );
    // `connect()` stamps the ladder before it publishes either retrying phase, so a zero stamp
    // on one is unreachable today — the guard is what keeps an unstamped state from printing
    // "reconnecting 1763918400s", which is what the subtraction gives when the stamp is 0.
    expect(
      attachLabel({ phase: "reconnecting", attempts: 1, ladderStartedAt: 0, reason: "" }, now),
    ).toBe("reconnecting");
  });

  // AGENTIC-UX §10 rule 7: the footer must not change width when a socket drops. The reason is a
  // whole sentence written for a human, so putting it in the bar would push the four facts
  // around every time the transport hiccups — and putting it nowhere would make a failed attach
  // unexplainable. A `title` is the third option and it costs no pixels.
  it("puts a failed attach's reason in the title and never in the bar", () => {
    attachState.current = {
      phase: "failed",
      attempts: 9,
      ladderStartedAt: 0,
      reason: "The sandbox is not running, so there is nothing to attach to.",
    };
    const session = seed();
    const { container } = render(<SessionPane sessionId={session.id} variant="column" />);

    const foot = container.querySelector(".pane-foot");
    expect(foot?.textContent).toBe("claude · opus · 41m · not attached");
    expect(foot?.getAttribute("title")).toBe(
      "The sandbox is not running, so there is nothing to attach to.",
    );
  });

  // And with nothing to explain there is no tooltip at all, rather than an empty one.
  it("carries no title when there is no reason", () => {
    const session = seed();
    const { container } = render(<SessionPane sessionId={session.id} variant="column" />);

    expect(container.querySelector(".pane-foot")?.hasAttribute("title")).toBe(false);
  });

  // A shell session has no model. An empty fact would render as a hanging separator — "claude ·
  //  · 41m" — which is the footer claiming a field it does not have.
  it("omits the model when the harness has none, rather than printing an empty fact", () => {
    const session = seed({ model: "" });
    const { container } = render(<SessionPane sessionId={session.id} variant="column" />);

    expect(container.querySelector(".pane-foot")?.textContent).toBe("claude · 41m · detached");
  });

  // §6.5's table says "harness command"; this header shows harness.name and the footer shows the
  // command, which is the one declared departure. Pinned so the departure is a decision on the
  // record rather than a thing a later edit can quietly undo in either direction.
  it("the header names the harness and the footer prints its command", () => {
    const session = seed();
    const { container } = render(<SessionPane sessionId={session.id} variant="column" />);

    expect(container.querySelector(".pane-sub")?.textContent).toBe("Claude Code");
    expect(container.querySelector(".pane-foot")?.textContent).toContain("claude ·");
  });

  // The focused pane must be distinguishable, and §7.5's ring is the only thing that does it.
  it("marks only the focused pane", async () => {
    const session = seed();
    const { container } = render(<SessionPane sessionId={session.id} variant="column" />);

    expect(container.querySelector(".pane")?.className).not.toContain("pane--focused");

    await userEvent.click(screen.getByRole("region", { name: /evidence-upload/ }));

    expect(container.querySelector(".pane")?.className).toContain("pane--focused");
  });
});

// The one create path shared by Restart and EmptyDevelop's two actions. Its failure branch is
// the part a green happy path hides: every client method rejects with ApiError and nothing
// else (§4), and two of the sixteen codes are different SENTENCES that must not be collapsed.
describe("startSessionOnWorktree", () => {
  const DEFAULTS = {
    harness: "claude-code",
    model: "opus",
    effort: "high",
    permissionMode: "ask",
  } as const;

  beforeEach(() => {
    resetCockpit();
    createSessionMock.mockReset();
  });

  it("says the control plane is down, and does not say Docker is", async () => {
    seed();
    createSessionMock.mockRejectedValue(new ApiError("network_unavailable", "fetch failed", 0));

    await startSessionOnWorktree("w-1", { ...DEFAULTS });

    const messages = useCockpit.getState().toasts.map((t) => t.message);
    expect(messages).toEqual(["The control plane is not running. Start it, then try again."]);
  });

  it("says Docker is down, and does not say the control plane is", async () => {
    seed();
    createSessionMock.mockRejectedValue(
      new ApiError("backend_unavailable", "docker daemon unreachable", 503),
    );

    await startSessionOnWorktree("w-1", { ...DEFAULTS });

    const messages = useCockpit.getState().toasts.map((t) => t.message);
    expect(messages).toEqual(["Docker is not running, so a sandbox cannot be started."]);
  });

  it("carries the server's own sentence for every other code, the sixteenth included", async () => {
    seed();
    createSessionMock.mockRejectedValue(
      new ApiError("non_loopback_publish", "The sandbox published a port off loopback.", 502),
    );

    await startSessionOnWorktree("w-1", { ...DEFAULTS });

    const message = useCockpit.getState().toasts[0].message;
    expect(message).toBe("The sandbox published a port off loopback.");
    // That sandbox is RUNNING. Collapsing this into "not running" would be false, which is
    // the whole reason Group C's ruling 7 made it its own code.
    expect(message).not.toContain("not running");
  });

  it("names a harness the catalogue does not have, and creates nothing", async () => {
    seed();

    await startSessionOnWorktree("w-1", { ...DEFAULTS, harness: "codex" });

    expect(createSessionMock).not.toHaveBeenCalled();
    expect(useCockpit.getState().toasts[0].message).toBe(
      "The codex harness is not in the catalogue.",
    );
  });
});
