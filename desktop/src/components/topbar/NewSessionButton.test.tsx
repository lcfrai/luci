import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { NewSessionButton } from "./NewSessionButton";
import { __resetCreateSession } from "../../hooks/useCreateSession";
import { resetCockpit, useCockpit } from "../../store/store";
import { aHarness, aSession, aWorktree } from "../../test/fixtures";

// vi.hoisted, not a bare const: vi.mock is hoisted above the module body, so a factory that
// closed over a plain const would read it in the temporal dead zone.
const { createSession } = vi.hoisted(() => ({ createSession: vi.fn() }));
vi.mock("../../api", () => ({ api: { createSession } }));

const claude = aHarness({
  id: "claude-code",
  name: "Claude Code",
  command: "claude",
  installed: true,
  flagsVerified: true,
  available: true,
  models: [{ id: "opus", name: "Opus" }],
  defaultModel: "opus",
  efforts: ["", "high"],
  permissionModes: ["ask", "plan"],
  supportsOpeningPrompt: true,
  supportsResume: true,
  note: "",
});

describe("NewSessionButton", () => {
  beforeEach(() => {
    createSession.mockReset();
    createSession.mockResolvedValue(aSession({ id: "s-new", worktreeId: "w-1" }));
    resetCockpit();
    __resetCreateSession();
    useCockpit.setState({
      harnesses: [claude],
      worktreesByBranch: { "b-1": [aWorktree({ id: "w-1", branchId: "b-1" })] },
    });
  });

  it("creates with resolved defaults on one click, and opens no dialog", async () => {
    render(<NewSessionButton worktreeId="w-1" />);
    await userEvent.click(screen.getByRole("button", { name: /new session/i }));
    await waitFor(() => expect(createSession).toHaveBeenCalledTimes(1));
    expect(createSession.mock.calls[0][0]).toMatchObject({
      worktreeId: "w-1",
      harness: "claude-code",
      model: "opus",
    });
    expect(useCockpit.getState().pickerOpen).toBe(false);
  });

  it("opens the picker from the caret and creates nothing", async () => {
    render(<NewSessionButton worktreeId="w-1" />);
    await userEvent.click(screen.getByRole("button", { name: "Session options" }));
    expect(useCockpit.getState().pickerOpen).toBe(true);
    expect(createSession).not.toHaveBeenCalled();
  });

  it("says why it cannot create when no worktree is selected", () => {
    render(<NewSessionButton worktreeId={null} />);
    const button = screen.getByRole("button", { name: /new session/i });
    expect(button.hasAttribute("disabled")).toBe(true);
    expect(button.getAttribute("title") ?? "").toMatch(/worktree/i);
  });

  it("focuses the created session, which is what makes defect 4's scroll fire", async () => {
    render(<NewSessionButton worktreeId="w-1" />);
    await userEvent.click(screen.getByRole("button", { name: /new session/i }));
    await waitFor(() => expect(useCockpit.getState().selection.sessionId).toBe("s-new"));
  });

  // Contract §3.2's `lastError` and §4.7: an operation clears only the report it raised itself.
  // ADDED BY TASK 24's FIX ROUND 1 (review I1), a declared departure into task 18's test file.
  // useCreateSession used to clear ANY report on success, so starting a session wiped a
  // catalogue list's failure off the footer while that list was still missing from the rail.
  describe("and the footer line", () => {
    const click = async (): Promise<void> => {
      await userEvent.click(screen.getByRole("button", { name: /new session/i }));
    };

    it("leaves another operation's report there when the create succeeds", async () => {
      const foreign = { code: "internal", message: "Error: projects answered 500" };
      useCockpit.getState().setError(foreign);
      render(<NewSessionButton worktreeId="w-1" />);
      await click();
      // The clear, when there is one, comes before the selection in useCreateSession, so this
      // wait is also the wait for it.
      await waitFor(() => expect(useCockpit.getState().selection.sessionId).toBe("s-new"));
      expect(useCockpit.getState().lastError).toBe(foreign);
    });

    it("clears the report its own failure left, when the next create succeeds", async () => {
      createSession.mockRejectedValueOnce({ code: "conflict", message: "Already starting." });
      render(<NewSessionButton worktreeId="w-1" />);
      await click();
      await waitFor(() => expect(useCockpit.getState().lastError?.code).toBe("conflict"));
      await click();
      await waitFor(() => expect(useCockpit.getState().selection.sessionId).toBe("s-new"));
      expect(useCockpit.getState().lastError).toBeNull();
    });

    it("writes nothing to the footer when there is nothing of its own to clear", async () => {
      // The act (shared shape §3A.1 clause 7): with no report raised and an empty slot, the
      // identity test is null === null, and without `raised !== null` a success would write
      // setError(null) over nothing. The same mutant survived at the verb site as V5.
      const setError = vi.spyOn(useCockpit.getState(), "setError");
      render(<NewSessionButton worktreeId="w-1" />);
      await click();
      await waitFor(() => expect(useCockpit.getState().selection.sessionId).toBe("s-new"));
      expect(setError).not.toHaveBeenCalled();
    });

    it("clears nothing when another report replaced its own while the request was out", async () => {
      // useCreateSession's `state` is read before the await, and the slot can change hands
      // during it. Here the create's own failure is on the footer when the click lands, and a
      // catalogue list's failure replaces it while the request is out. Read off the stale
      // snapshot, the create's report still looks like the one on screen, and the success
      // wipes the catalogue's.
      createSession.mockRejectedValueOnce({ code: "conflict", message: "Already starting." });
      render(<NewSessionButton worktreeId="w-1" />);
      await click();
      await waitFor(() => expect(useCockpit.getState().lastError?.code).toBe("conflict"));

      let answer: (session: unknown) => void = () => undefined;
      createSession.mockReturnValueOnce(
        new Promise((resolve) => {
          answer = resolve;
        }),
      );
      await click();
      await waitFor(() => expect(createSession).toHaveBeenCalledTimes(2));
      const foreign = { code: "internal", message: "Error: projects answered 500" };
      useCockpit.getState().setError(foreign);
      answer(aSession({ id: "s-new", worktreeId: "w-1" }));
      await waitFor(() => expect(useCockpit.getState().selection.sessionId).toBe("s-new"));
      expect(useCockpit.getState().lastError).toBe(foreign);
    });
  });

  // useCreateSession's failure path. Every test above resolves, so before these landed the
  // catch block, sentenceFor's three arms and the no-harness early return were executed by
  // nothing. §4.2's rule is that network_unavailable and backend_unavailable must NOT be
  // collapsed, and its stated cost is ten minutes of a user's time — telling someone to start
  // Docker when the control plane is dead. That rule was asserted by nobody.
  //
  // NESTED, not a sibling describe: these need the beforeEach above. As a sibling they got no
  // resetCockpit() between cases and the toasts accumulated 1, 2, 3, 4 — which is how the
  // length assertions caught it.
  describe("when the create fails", () => {
    const click = async (): Promise<void> => {
      render(<NewSessionButton worktreeId="w-1" />);
      await userEvent.click(screen.getByRole("button", { name: /new session/i }));
    };

    it("says the control plane is down for network_unavailable", async () => {
      createSession.mockRejectedValue({
        code: "network_unavailable",
        message: "fetch failed",
      });
      await click();
      await waitFor(() => expect(useCockpit.getState().toasts).toHaveLength(1));
      const toast = useCockpit.getState().toasts[0].message;
      expect(toast).toContain("control plane is not running");
      expect(toast).not.toContain("Docker");
      expect(useCockpit.getState().lastError).toEqual({
        code: "network_unavailable",
        message: "fetch failed",
      });
    });

    it("says Docker is down for backend_unavailable, which is a different sentence", async () => {
      createSession.mockRejectedValue({
        code: "backend_unavailable",
        message: "docker daemon unreachable",
      });
      await click();
      await waitFor(() => expect(useCockpit.getState().toasts).toHaveLength(1));
      const toast = useCockpit.getState().toasts[0].message;
      expect(toast).toContain("Docker is not running");
      expect(toast).not.toContain("control plane");
    });

    // The sixteenth code, added by Group C's ruling 7. Its sandbox IS running, so any locally
    // invented "not running" phrasing would be a lie — the message must pass through verbatim.
    // This also pins the default arm, which exists because task 16's client lets an UNKNOWN
    // code through intact rather than validating it.
    it("renders an unrecognised code's server message verbatim", async () => {
      createSession.mockRejectedValue({
        code: "non_loopback_publish",
        message: "sandbox published on 0.0.0.0, refusing to attach",
      });
      await click();
      await waitFor(() => expect(useCockpit.getState().toasts).toHaveLength(1));
      expect(useCockpit.getState().toasts[0].message).toBe(
        "sandbox published on 0.0.0.0, refusing to attach",
      );
    });

    it("refuses to create when the machine has no harness, and calls the API not at all", async () => {
      useCockpit.setState({ harnesses: [] });
      await click();
      await waitFor(() => expect(useCockpit.getState().toasts).toHaveLength(1));
      expect(useCockpit.getState().toasts[0].message).toContain("No harness is available");
      expect(createSession).not.toHaveBeenCalled();
    });
  });
});
