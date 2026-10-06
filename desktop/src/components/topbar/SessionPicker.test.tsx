import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SessionPicker } from "./SessionPicker";
import { resetCockpit, useCockpit } from "../../store/store";
import {
  aBranch,
  aHarness,
  aProject,
  aSession,
  aWorktree,
} from "../../test/fixtures";
import { ApiError } from "../../api/client";
import { productApi } from "../../api/product";

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
    vi.spyOn(productApi, "agentConnections").mockResolvedValue({ revision: 0, activeConnectionId: "", connections: [] });
    vi.spyOn(productApi, "agents").mockResolvedValue({ providers: [] });
    createSession.mockReset();
    createSession.mockResolvedValue(
      aSession({ id: "s-new", worktreeId: "w-1" }),
    );
    resetCockpit();
    useCockpit.setState({
      harnesses: [claude, shell, codex],
      projects: [aProject({ id: "p-1" })],
      branchesByProject: { "p-1": [aBranch({ id: "b-1", projectId: "p-1" })] },
      worktreesByBranch: {
        "b-1": [aWorktree({ id: "w-1", branchId: "b-1", name: "wt-one" })],
      },
      selection: {
        projectId: "p-1",
        branchId: "b-1",
        worktreeId: "w-1",
        sessionId: null,
      },
    });
  });

  it("uses an exact saved gateway model and connection when starting Develop", async () => {
    const enabledCodex = { ...codex, available: true, installed: true, flagsVerified: true, models: [{ id: "default", name: "Agent default" }], defaultModel: "default", supportsCustomModel: true };
    useCockpit.setState({ harnesses: [enabledCodex], defaults: { harness: "codex", model: "default", effort: "", permissionMode: "ask" } });
    vi.mocked(productApi.agentConnections).mockResolvedValue({ revision: 1, activeConnectionId: "", connections: [{ id: "saved-gateway", name: "Company models", kind: "litellm", revision: 1, enabled: true, configured: true, keyPresent: true, endpoints: { responses: "https://gateway.example/v1" }, models: { codex: "company-coder" } }] });
    render(<SessionPicker open worktreeId="w-1" onClose={vi.fn()} />);
    await screen.findByRole("option", { name: "Company models" });
    await userEvent.selectOptions(screen.getByLabelText("Agent connection"), "saved-gateway");
    expect((screen.getByLabelText("Model") as HTMLInputElement).value).toBe("company-coder");
    expect((screen.getByLabelText("Model") as HTMLInputElement).readOnly).toBe(true);
    await userEvent.click(screen.getByRole("button", { name: "Create session" }));
    await waitFor(() => expect(createSession).toHaveBeenCalledWith(expect.objectContaining({ model: "company-coder", agentConnectionId: "saved-gateway", harness: "codex" })));
  });

  it("preserves a custom Develop model rather than replacing it with the catalogue default", async () => {
    useCockpit.setState({ harnesses: [{ ...claude, supportsCustomModel: true }], defaults: { harness: "claude-code", model: "my-explicit-model", effort: "", permissionMode: "ask" } });
    render(<SessionPicker open worktreeId="w-1" onClose={vi.fn()} />);
    expect((screen.getByLabelText("Model") as HTMLInputElement).value).toBe("my-explicit-model");
    await userEvent.click(screen.getByRole("button", { name: "Create session" }));
    await waitFor(() => expect(createSession).toHaveBeenCalledWith(expect.objectContaining({ model: "my-explicit-model" })));
  });

  it("retains the draft after a failed start and closes only after a successful retry", async () => {
    const close = vi.fn();
    createSession.mockRejectedValueOnce(
      new ApiError(
        "backend_unavailable",
        "Start Docker before trying again.",
        503,
      ),
    );
    render(
      <SessionPicker
        open
        worktreeId="w-1"
        onClose={close}
        initialPrompt="Implement the saved plan"
      />,
    );
    await userEvent.click(
      screen.getByRole("button", { name: "Create session" }),
    );
    expect((await screen.findByRole("alert")).textContent).toContain("Docker");
    expect(
      (
        screen.getByRole("textbox", {
          name: "Opening prompt",
        }) as HTMLTextAreaElement
      ).value,
    ).toBe("Implement the saved plan");
    expect(close).not.toHaveBeenCalled();
    await userEvent.click(
      screen.getByRole("button", { name: "Create session" }),
    );
    await waitFor(() => expect(close).toHaveBeenCalledOnce());
    expect(createSession).toHaveBeenCalledTimes(2);
  });

  it("lists an unavailable harness with its note, rather than hiding it", () => {
    render(<SessionPicker open worktreeId="w-1" onClose={() => {}} />);
    expect(
      screen.getByRole("option", { name: /Not installed on this machine\./ }),
    ).toBeDefined();
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
    await userEvent.click(
      screen.getByRole("button", { name: "Create session" }),
    );
    const body = createSession.mock.calls[0][0] as Record<string, unknown>;
    expect(body.harness).toBe("shell");
    expect(body).not.toHaveProperty("model");
    expect(body).not.toHaveProperty("effort");
    expect(body).not.toHaveProperty("permissionMode");
  });

  it("creates Shell without a provider route while retaining the nested coding default for switching back", async () => {
    const close = vi.fn();
    useCockpit.getState().setProjectDefaults("p-1", { agentConnectionId: "project-gateway" });
    useCockpit.getState().setBranchDefaults("b-1", { agentConnectionId: "saved-gateway" });
    vi.mocked(productApi.agentConnections).mockResolvedValue({ revision: 2, activeConnectionId: "active-gateway", connections: [
      { id: "active-gateway", name: "Active gateway", kind: "litellm", revision: 1, enabled: true, configured: true, keyPresent: true, endpoints: { anthropicMessages: "https://active.example" }, models: { claudeCode: "active-claude" } },
      { id: "saved-gateway", name: "Branch gateway", kind: "litellm", revision: 1, enabled: true, configured: true, keyPresent: true, endpoints: { anthropicMessages: "https://branch.example" }, models: { claudeCode: "branch-claude" } },
    ] });
    createSession.mockResolvedValue(aSession({ id: "s-shell", worktreeId: "w-1", harness: "shell" }));
    render(<SessionPicker open worktreeId="w-1" onClose={close} />);
    await screen.findByRole("option", { name: "Branch gateway" });
    expect((screen.getByLabelText("Agent connection") as HTMLSelectElement).value).toBe("saved-gateway");
    await userEvent.selectOptions(screen.getByLabelText("Harness"), "shell");
    expect(screen.queryByLabelText("Agent connection")).toBeNull();
    expect(screen.queryByLabelText("Model")).toBeNull();
    expect(screen.queryByRole("alert")).toBeNull();
    expect(screen.getByText("Tools installed in this terminal’s private home survive Restart. Forget removes that home and its tools.")).not.toBeNull();
    expect((screen.getByRole("button", { name: "Create session" }) as HTMLButtonElement).disabled).toBe(false);
    await userEvent.selectOptions(screen.getByLabelText("Harness"), "claude-code");
    expect((screen.getByLabelText("Agent connection") as HTMLSelectElement).value).toBe("saved-gateway");
    await userEvent.selectOptions(screen.getByLabelText("Harness"), "shell");
    await userEvent.click(screen.getByRole("button", { name: "Create session" }));
    await waitFor(() => expect(close).toHaveBeenCalledOnce());
    expect(createSession).toHaveBeenCalledWith({ worktreeId: "w-1", harness: "shell" });
    expect(useCockpit.getState().sessions["s-shell"].harness).toBe("shell");
    expect(useCockpit.getState().branchDefaults["b-1"].agentConnectionId).toBe("saved-gateway");
  });

  it("retains an explicitly selected coding connection through a Shell switch", async () => {
    vi.mocked(productApi.agentConnections).mockResolvedValue({ revision: 1, activeConnectionId: "saved-gateway", connections: [
      { id: "saved-gateway", name: "Company gateway", kind: "litellm", revision: 1, enabled: true, configured: true, keyPresent: true, endpoints: { anthropicMessages: "https://gateway.example" }, models: { claudeCode: "company-claude" } },
    ] });
    render(<SessionPicker open worktreeId="w-1" onClose={vi.fn()} />);
    await screen.findByRole("option", { name: "Company gateway" });
    await userEvent.selectOptions(screen.getByLabelText("Agent connection"), "saved-gateway");
    await userEvent.selectOptions(screen.getByLabelText("Harness"), "shell");
    expect((screen.getByRole("button", { name: "Create session" }) as HTMLButtonElement).disabled).toBe(false);
    await userEvent.selectOptions(screen.getByLabelText("Harness"), "claude-code");
    expect((screen.getByLabelText("Agent connection") as HTMLSelectElement).value).toBe("saved-gateway");
  });

  it("saves the draft as the branch default and creates nothing", async () => {
    const onClose = vi.fn();
    render(<SessionPicker open worktreeId="w-1" onClose={onClose} />);
    await userEvent.selectOptions(screen.getByLabelText("Harness"), "shell");
    await userEvent.click(
      screen.getByRole("button", { name: "Save as branch default" }),
    );
    expect(useCockpit.getState().branchDefaults["b-1"]).toMatchObject({
      harness: "shell",
    });
    expect(createSession).not.toHaveBeenCalled();
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("renders nothing when closed", () => {
    render(<SessionPicker open={false} worktreeId="w-1" onClose={() => {}} />);
    expect(screen.queryByLabelText("Harness")).toBeNull();
  });
});
