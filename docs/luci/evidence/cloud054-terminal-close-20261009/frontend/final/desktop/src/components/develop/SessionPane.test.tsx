// Contract §6.5, §6.8, and known defect 3. Every behaviour here is one a later refactor would
// otherwise quietly delete: an input surface for every status, focus intent split three ways,
// an exited pane that keeps its transcript, and a footer asserted by what it does NOT contain.
//
// No jest-dom (shared-shape §3C): plain DOM property assertions only.
import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SessionPane } from "./SessionPane";
import { startPolling } from "../../api/poller";
import type { CockpitClient } from "../../api/client";
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
  Terminal: ({
    sessionId,
    readOnly,
    focused,
  }: {
    sessionId: string;
    readOnly: boolean;
    focused: boolean;
  }) => (
    <div
      data-testid="terminal"
      data-session={sessionId}
      data-readonly={String(readOnly)}
      data-focused={String(focused)}
    >
      <textarea className="xterm-helper-textarea" data-testid="xterm" readOnly />
    </div>
  ),
}));

// vi.hoisted, not a bare const: vi.mock is hoisted above the imports, and the factory runs
// while ./SessionPane's import graph is evaluated — before a top-level `const` has been
// initialised. A bare const here throws "Cannot access 'createSessionMock' before
// initialization" and looks like a mocking bug rather than a TDZ one.
const { createSessionMock, restartSessionMock, reviewedContextMock } = vi.hoisted(() => ({
  createSessionMock: vi.fn(),
  restartSessionMock: vi.fn(),
  reviewedContextMock: vi.fn(),
}));
vi.mock("../../api", () => ({
  api: { createSession: createSessionMock, restartSession: restartSessionMock, getSessionReviewedContext: reviewedContextMock },
}));
vi.mock("../../hooks/useProjectDraft", () => ({ useProjectDraftScope: () => "account:workspace" }));

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
    restartSessionMock.mockReset();
    reviewedContextMock.mockReset();
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

  it("distinguishes captured inputs from older live context without replacing the terminal", () => {
    const implementationContext = { projectId: "p-1", briefRevision: 2, planRevision: 3, specificationRevision: 6, designPlanRevision: 8, taskIds: ["task-1"], briefDigest: "brief-2", planDigest: "plan-3", specificationDigest: "spec-6", designPlanDigest: "design-8", worktreeId: "w-1", sourceCommit: "a".repeat(40), sourceDigest: "source-a", sourceDirty: false, taskTitles: ["Build editor"], requirementIds: ["R-EDIT"] };
    const session = seed({ implementationContext });
    render(<SessionPane sessionId={session.id} variant="column" />);
    const terminal = screen.getByTestId("terminal");
    expect(screen.getByRole("note", { name: "Agent context" }).textContent).toContain("A new reviewed run is needed to use captured inputs");
    act(() => useCockpit.getState().upsertSession({ ...session, reviewedContextDelivery: { status: "captured", version: 1, digest: "a".repeat(64) } }));
    expect(screen.getByRole("note", { name: "Agent context" }).textContent).toContain("Reviewed inputs were captured at start");
    expect(screen.getByTestId("terminal")).toBe(terminal);
    act(() => useCockpit.getState().upsertSession({ ...session, status: "exited", reviewedContextDelivery: { status: "legacy-live" } }));
    expect(screen.getByRole("note", { name: "Agent context" }).textContent).toContain("A new reviewed run is needed to use captured inputs");
    expect(screen.getByRole("note", { name: "Agent context" }).textContent).not.toContain("Stop it");
    expect(screen.getByTestId("terminal")).toBe(terminal);
    act(() => useCockpit.getState().upsertSession({ ...session, reviewedContextDelivery: { status: "captured", version: 1, digest: "" } }));
    expect(screen.getByRole("note", { name: "Agent context" }).textContent).toContain("Context delivery is unconfirmed");
  });

  it("does not label a manual session as a reviewed-context delivery", () => {
    const session = seed();
    render(<SessionPane sessionId={session.id} variant="column" />);
    expect(screen.queryByRole("note", { name: "Agent context" })).toBeNull();
  });

  it("keeps its terminal mounted and does not redirect dialog pointer or keyboard focus into the terminal", async () => {
    const implementationContext = { projectId: "p-1", briefRevision: 2, planRevision: 3, specificationRevision: 6, designPlanRevision: 8, taskIds: ["task-1"], briefDigest: "brief-2", planDigest: "plan-3", specificationDigest: "spec-6", designPlanDigest: "design-8", worktreeId: "w-1", sourceCommit: "", sourceDigest: "source-a", sourceDirty: false, taskTitles: ["Build editor"], requirementIds: [] };
    const session = seed({ implementationContext, reviewedContextDelivery: { status: "captured", version: 1, digest: "a".repeat(64) } });
    reviewedContextMock.mockResolvedValue({ sessionId: session.id, projectId: session.projectId, worktreeId: session.worktreeId, version: 1, digest: "a".repeat(64), implementationContext, document: { kind: "definition", sha256: "b".repeat(64), bytes: 40, value: { purpose: "Original editor context" } } });
    render(<SessionPane sessionId={session.id} variant="column" />);
    const terminal = screen.getByTestId("terminal"), input = screen.getByTestId("xterm");
    const trigger = screen.getByRole("button", { name: "View reviewed context" }); trigger.focus();
    await userEvent.keyboard("{Enter}");
    const dialog = await screen.findByRole("dialog", { name: "Reviewed context" });
    await userEvent.click(await within(dialog).findByText("Original editor context"));
    expect(document.activeElement).not.toBe(input);
    expect(within(dialog).getByLabelText("Captured document")).not.toBeNull();
    expect(screen.getByTestId("terminal")).toBe(terminal);
    await userEvent.keyboard("{Escape}");
    await waitFor(() => expect(document.activeElement).toBe(trigger));
    expect(screen.getByTestId("terminal")).toBe(terminal);
    expect(createSessionMock).not.toHaveBeenCalled(); expect(restartSessionMock).not.toHaveBeenCalled();
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

  it("a deferred old poll cannot undo the actual exit-bar Restart response", async () => {
    const old = seed({ status: "exited", exitReason: "exit status 0", executionGeneration: "a".repeat(64) });
    let release!: (sessions: Session[]) => void;
    const listSessions = vi.fn(() => new Promise<Session[]>(resolve => { release = resolve; }));
    Object.defineProperty(document, "visibilityState", { configurable: true, value: "visible" });
    const stop = startPolling({ listSessions } as unknown as CockpitClient, useCockpit);
    const replacement = { ...old, status: "running" as const, exitReason: "", executionGeneration: "b".repeat(64) };
    restartSessionMock.mockResolvedValue(replacement);
    try {
      render(<SessionPane sessionId={old.id} variant="column" />);
      await userEvent.click(screen.getByRole("button", { name: /restart/i }));
      expect(restartSessionMock).toHaveBeenCalledWith(old.id);
      expect(useCockpit.getState().sessions[old.id]).toBe(replacement);
      await act(async () => { release([old]); await Promise.resolve(); });
      expect(useCockpit.getState().sessions[old.id]).toBe(replacement);
      expect(screen.queryByRole("button", { name: /restart/i })).toBeNull();
    } finally { stop(); }
  });

  it("an exited pane keeps its scrollback, prints exitReason verbatim and offers Restart", async () => {
    const session = seed({ status: "exited", exitReason: "idle timeout" });
    restartSessionMock.mockResolvedValue(
      aSession({ id: "s-1", worktreeId: "w-1", branchId: "b-1", status: "starting" }),
    );
    render(<SessionPane sessionId={session.id} variant="column" />);

    expect(screen.getByTestId("terminal").getAttribute("data-readonly")).toBe("true");
    expect(screen.getByText("idle timeout")).toBeDefined();

    await userEvent.click(screen.getByRole("button", { name: /restart/i }));

    // `POST /v1/sessions/{id}/restart`, not a create. sessions.go:723: restart is "same
    // config, clean context" and KEEPS THE SESSION ID, and resume is a different verb
    // "because conflating the two makes restart lossy". A create carrying the dead session's
    // settings would orphan it beside an unrelated session that merely looks related.
    expect(restartSessionMock).toHaveBeenCalledWith("s-1");
    expect(createSessionMock).not.toHaveBeenCalled();

    await waitFor(() => expect(useCockpit.getState().sessions["s-1"].status).toBe("starting"));
    // One session, restarted in place. No second id appeared.
    expect(Object.keys(useCockpit.getState().sessions)).toEqual(["s-1"]);
  });

  // Both are 409 CodeConflict and both must arrive as the server wrote them. Collapsing them
  // would tell a user whose worktree has vanished that a restart is already running.
  it("preserves restart refusals and the reviewed-task recovery path as the server's own sentence", async () => {
    for (const sentence of [
      "A restart is already in flight for that session.",
      "The worktree this session was created in no longer resolves.",
      "Reserve a new coding operation in the project run before retrying. The original attempt remains in the run history.",
    ]) {
      resetCockpit();
      attachState.current = { phase: "detached", attempts: 0, ladderStartedAt: 0, reason: "" };
      const session = seed({ status: "exited", exitReason: "killed" });
      restartSessionMock.mockReset();
      restartSessionMock.mockRejectedValue(new ApiError("conflict", sentence, 409));
      const view = render(<SessionPane sessionId={session.id} variant="column" />);

      await userEvent.click(screen.getByRole("button", { name: /restart/i }));

      await waitFor(() =>
        expect(useCockpit.getState().toasts.map((t) => t.message)).toEqual([sentence]),
      );
      view.unmount();
    }
  });

  it("does not blame Docker when the control plane is the thing that is down", async () => {
    const session = seed({ status: "exited", exitReason: "killed" });
    restartSessionMock.mockRejectedValue(new ApiError("network_unavailable", "fetch failed", 0));
    render(<SessionPane sessionId={session.id} variant="column" />);

    await userEvent.click(screen.getByRole("button", { name: /restart/i }));

    await waitFor(() =>
      expect(useCockpit.getState().toasts[0].message).toBe(
        "The control plane is not running, so the session was not restarted.",
      ),
    );
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

  // The Math.max(0, …) clamp is load-bearing, not defensive padding. useHeartbeat() returns a
  // value FLOORED to the second (heartbeat.ts:28) while ladderStartedAt is a raw Date.now()
  // (attachRegistry.ts:326), so at a ladder's start the tick trails the stamp by up to 999ms
  // and Math.floor(negative / 1000) is -1. Without the clamp the footer reads "connecting -1s"
  // at the start of essentially every attach. formatElapsed's identical clamp has its own test
  // at time.test.ts:37; this one now does too.
  it("never prints a negative elapsed when the tick trails the stamp", () => {
    const now = 1_700_000_000_000;
    expect(
      attachLabel({ phase: "connecting", attempts: 1, ladderStartedAt: now + 500, reason: "" }, now),
    ).toBe("connecting 0s");
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

  // §7.7's connecting line. The gate is the ATTACH PHASE, never the session status — a socket
  // dropping on an hour-old `running` session climbs the same ladder as a cold start, and a
  // status gate would reproduce the black rectangle the row exists to prevent for the commoner
  // case. The overlay is a SIBLING of the terminal in every state: unmounting the terminal to
  // explain the terminal destroys what the explanation is about.
  it("shows nothing over the body while the socket is up", () => {
    attachState.current = { phase: "attached", attempts: 0, ladderStartedAt: 0, reason: "" };
    const session = seed();
    const { container } = render(<SessionPane sessionId={session.id} variant="column" />);

    expect(container.querySelector(".pane-attach")).toBeNull();
    expect(screen.getByTestId("terminal")).toBeDefined();
  });

  it("centres the elapsed on a first connect, where the body is provably empty", () => {
    const now = Date.now();
    attachState.current = {
      phase: "connecting",
      attempts: 1,
      ladderStartedAt: now - 3_000,
      reason: "",
    };
    const session = seed({ status: "starting" });
    const { container } = render(<SessionPane sessionId={session.id} variant="column" />);

    const overlay = container.querySelector(".pane-attach");
    expect(overlay).not.toBeNull();
    expect(overlay?.className).toContain("pane-attach--centre");
    expect(overlay?.textContent).toMatch(/^connecting \d+s$/);
    // Sibling, not a replacement.
    expect(screen.getByTestId("terminal")).toBeDefined();
  });

  it("never covers a transcript: a reconnect gets the corner strip, not the centre", () => {
    attachState.current = {
      phase: "reconnecting",
      attempts: 4,
      ladderStartedAt: Date.now() - 12_000,
      reason: "",
    };
    const session = seed();
    const { container } = render(<SessionPane sessionId={session.id} variant="column" />);

    const overlay = container.querySelector(".pane-attach");
    expect(overlay?.className).toContain("pane-attach--corner");
    expect(overlay?.className).not.toContain("pane-attach--centre");
    expect(screen.getByTestId("terminal")).toBeDefined();
  });

  // `giveUp` sets ladderStartedAt back to 0, so the stamp alone can never cover this phase.
  // Before this, all three of its sentences reached the user only as the footer's `title`.
  it("renders a given-up attach's reason verbatim, in the strip and not the centre", () => {
    attachState.current = {
      phase: "failed",
      attempts: 9,
      ladderStartedAt: 0,
      reason: "the sandbox did not accept a connection within 60 seconds",
    };
    const session = seed();
    const { container } = render(<SessionPane sessionId={session.id} variant="column" />);

    const overlay = container.querySelector(".pane-attach");
    expect(overlay?.textContent).toBe("the sandbox did not accept a connection within 60 seconds");
    expect(overlay?.className).toContain("pane-attach--corner");
    // Announced once, because it is final. The retrying variants tick every second and a
    // polite live region would read the elapsed aloud for the whole ladder.
    expect(overlay?.getAttribute("role")).toBe("status");
    expect(screen.getByTestId("terminal")).toBeDefined();
  });

  // BOTH ticking phases, not just `connecting`. The live region is keyed on `failed` precisely
  // because `reconnecting` ticks too — widening the condition to `phase !== "connecting"` reads
  // plausible and puts a polite live region on a string that changes every second, so a screen
  // reader re-hears the elapsed once a second for the whole ladder. Only the negative case on
  // `reconnecting` pins that boundary; the `connecting` case alone leaves it open.
  it("does not announce either of the ticking variants", () => {
    for (const phase of ["connecting", "reconnecting"] as const) {
      resetCockpit();
      attachState.current = { phase, attempts: 1, ladderStartedAt: Date.now(), reason: "" };
      const session = seed({ status: "starting" });
      const { container, unmount } = render(
        <SessionPane sessionId={session.id} variant="column" />,
      );

      const overlay = container.querySelector(".pane-attach");
      expect(overlay).not.toBeNull();
      expect(overlay?.hasAttribute("role")).toBe(false);
      unmount();
    }
  });

  // The other half of the same boundary. `failed` is what selects the reason over the label, and
  // widening it to include `reconnecting` empties the strip, because a retrying ladder carries
  // no reason — the registry publishes `reason: ""` until it gives up.
  it("a reconnecting strip shows the elapsed, not an empty reason", () => {
    attachState.current = {
      phase: "reconnecting",
      attempts: 4,
      ladderStartedAt: Date.now() - 7_000,
      reason: "",
    };
    const session = seed();
    const { container } = render(<SessionPane sessionId={session.id} variant="column" />);

    const overlay = container.querySelector(".pane-attach");
    expect(overlay?.textContent).toMatch(/^reconnecting \d+s$/);
    expect(overlay?.textContent).not.toBe("");
  });

  // The pane is what tells the terminal whether it is the focused one — `Terminal` moves DOM
  // focus, `uiSlice.focusSession` holds which pane deserves it, and this prop is the seam
  // between them. Unthreaded, every terminal would take the caret on mount again and the last
  // one to flush would keep it.
  it("tells the terminal whether this pane is the focused one", async () => {
    const session = seed();
    render(<SessionPane sessionId={session.id} variant="column" />);

    expect(screen.getByTestId("terminal").getAttribute("data-focused")).toBe("false");

    await userEvent.click(screen.getByRole("region", { name: /evidence-upload/ }));

    expect(screen.getByTestId("terminal").getAttribute("data-focused")).toBe("true");
  });

  // Task 21 renders the pane as a tile and binds its drag handler to the ATTRIBUTE, not the
  // class — a class is a styling decision a redesign may rename; a handle is a contract.
  // Every other test here uses variant="column", so without this the contract is unasserted.
  it("carries task 21's drag handle and tile classes in the tile variant", () => {
    const session = seed();
    const { container } = render(<SessionPane sessionId={session.id} variant="tile" />);

    const head = container.querySelector(".pane-head");
    expect(head?.getAttribute("data-pane-drag-handle")).toBe("true");
    expect(head?.className).toContain("pane-head--tile");
    expect(container.querySelector(".pane")?.className).toContain("pane--tile");
    expect(container.querySelector(".pane")?.getAttribute("data-variant")).toBe("tile");
  });

  it("leaves the drag handle off the column variant", () => {
    const session = seed();
    const { container } = render(<SessionPane sessionId={session.id} variant="column" />);

    const head = container.querySelector(".pane-head");
    expect(head?.hasAttribute("data-pane-drag-handle")).toBe(false);
    expect(head?.className).not.toContain("pane-head--tile");
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
