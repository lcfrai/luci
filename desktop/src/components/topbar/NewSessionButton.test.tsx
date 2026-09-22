import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { NewSessionButton } from "./NewSessionButton";
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
});
