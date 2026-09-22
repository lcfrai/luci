import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SessionPicker } from "./SessionPicker";
import { resetCockpit, useCockpit } from "../../store/store";
import { aBranch, aHarness, aProject, aSession, aWorktree } from "../../test/fixtures";

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
const shell = aHarness({
  id: "shell",
  name: "Shell",
  command: "/bin/bash",
  installed: true,
  flagsVerified: true,
  available: true,
  models: [],
  defaultModel: "",
  efforts: [],
  permissionModes: [],
  supportsOpeningPrompt: false,
  supportsResume: false,
  note: "Always available: the sandbox image has bash.",
});
const codex = aHarness({
  id: "codex",
  name: "Codex",
  command: "codex",
  installed: false,
  flagsVerified: false,
  available: false,
  models: [],
  defaultModel: "",
  efforts: [],
  permissionModes: [],
  supportsOpeningPrompt: true,
  supportsResume: false,
  note: "Not installed on this machine.",
});

describe("SessionPicker", () => {
  beforeEach(() => {
    createSession.mockReset();
    createSession.mockResolvedValue(aSession({ id: "s-new", worktreeId: "w-1" }));
    resetCockpit();
    useCockpit.setState({
      harnesses: [claude, shell, codex],
      projects: [aProject({ id: "p-1" })],
      branchesByProject: { "p-1": [aBranch({ id: "b-1", projectId: "p-1" })] },
      worktreesByBranch: { "b-1": [aWorktree({ id: "w-1", branchId: "b-1", name: "wt-one" })] },
      selection: { projectId: "p-1", branchId: "b-1", worktreeId: "w-1", sessionId: null },
    });
  });

  it("lists an unavailable harness with its note, rather than hiding it", () => {
    render(<SessionPicker open worktreeId="w-1" onClose={() => {}} />);
    expect(screen.getByRole("option", { name: /Not installed on this machine\./ })).toBeDefined();
  });

  it("hides model, effort and permission mode for a harness that declares none", async () => {
    render(<SessionPicker open worktreeId="w-1" onClose={() => {}} />);
    expect(screen.getByLabelText("Model")).toBeDefined();
    await userEvent.selectOptions(screen.getByLabelText("Harness"), "shell");
    expect(screen.queryByLabelText("Model")).toBeNull();
    expect(screen.queryByLabelText("Effort")).toBeNull();
    expect(screen.queryByLabelText("Permission mode")).toBeNull();
    expect(screen.queryByLabelText("Opening prompt")).toBeNull();
  });

  it("sends only the fields the chosen harness accepts", async () => {
    render(<SessionPicker open worktreeId="w-1" onClose={() => {}} />);
    await userEvent.selectOptions(screen.getByLabelText("Harness"), "shell");
    await userEvent.click(screen.getByRole("button", { name: "Create session" }));
    const body = createSession.mock.calls[0][0] as Record<string, unknown>;
    expect(body.harness).toBe("shell");
    expect(body).not.toHaveProperty("model");
    expect(body).not.toHaveProperty("effort");
    expect(body).not.toHaveProperty("permissionMode");
  });

  it("saves the draft as the branch default and creates nothing", async () => {
    const onClose = vi.fn();
    render(<SessionPicker open worktreeId="w-1" onClose={onClose} />);
    await userEvent.selectOptions(screen.getByLabelText("Harness"), "shell");
    await userEvent.click(screen.getByRole("button", { name: "Save as branch default" }));
    expect(useCockpit.getState().branchDefaults["b-1"]).toMatchObject({ harness: "shell" });
    expect(createSession).not.toHaveBeenCalled();
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("renders nothing when closed", () => {
    render(<SessionPicker open={false} worktreeId="w-1" onClose={() => {}} />);
    expect(screen.queryByLabelText("Harness")).toBeNull();
  });
});
