import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, expect, it, vi } from "vitest";
import { orchestratorApi, type OrchestratorJob, type OrchestratorView } from "../../api/orchestrator";
import { productApi, type AgentConnectionCatalogue } from "../../api/product";
import { ApiError } from "../../api/client";
import { selectTeamScope } from "../../api/teamScope";
import { AGENT_CONNECTIONS_CHANGED } from "../../hooks/useAgentConnections";
import { useCockpit, resetCockpit } from "../../store/store";
import { ORCHESTRATOR_OPEN, OrchestratorPanel } from "./OrchestratorPanel";
import { notifyProjectContextChanged, takeProjectContextEdit } from "../../api/projectContextEvents";
import { DefinitionWorkspace } from "../definition/DefinitionWorkspace";
import { definitionApi } from "../../api/definition";
import { context, job, proposalTurn, turn, view } from "./fixtures";

vi.mock("../journey/ContextStorage", () => ({ ContextStorage: () => null }));
const identity = vi.hoisted(() => ({ actor: "owner", connection: "", root: "/projects/owner", read: true, edit: true, assist: true }));
vi.mock("../auth/AuthProvider", () => ({ useAuth: () => identity.actor ? { profile: { user: { id: identity.actor } } } : null }));
vi.mock("../setup/SetupProvider", () => ({ useSetup: () => ({ openAgents: vi.fn(), workspace: { mode: identity.connection ? "cloud" : "local", connectionId: identity.connection, host: identity.connection }, status: { access: { user: { username: identity.actor } }, config: { projectsDir: identity.root } } }) }));
vi.mock("../../hooks/useProjectAccess", () => ({ useProjectAccess: () => ({ canRead: identity.read, canEdit: identity.edit, canExecute: identity.assist, canAssist: identity.assist, message: identity.edit ? "" : "Read-only access." }) }));

const catalogue = (activeConnectionId = "gateway"): AgentConnectionCatalogue => ({ revision: 1, activeConnectionId, connections: [
  { id: "gateway", name: "Team provider", revision: 1, kind: "compatible-api", enabled: true, configured: true, keyPresent: true, models: { codex: "model-team", opencode: "model-open" }, endpoints: { responses: "https://provider.test/v1/responses", chatCompletions: "https://provider.test/v1/chat/completions" } },
  { id: "alternative", name: "Alternative provider", revision: 1, kind: "compatible-api", enabled: true, configured: true, keyPresent: true, models: { codex: "model-alternative" }, endpoints: { responses: "https://other.test/v1/responses" } },
] });
beforeEach(() => {
  vi.restoreAllMocks(); localStorage.clear(); resetCockpit(); selectTeamScope("");
  useCockpit.setState(state => ({ defaults: { ...state.defaults, harness: "codex", model: "" } }));
  Object.assign(identity, { actor: "owner", connection: "", root: "/projects/owner", read: true, edit: true, assist: true });
  vi.spyOn(productApi, "agentConnections").mockResolvedValue(catalogue());
  vi.spyOn(orchestratorApi, "get").mockImplementation(async id => view({ projectId: id, context: context(id) }));
  vi.spyOn(orchestratorApi, "send").mockResolvedValue(job());
  vi.spyOn(orchestratorApi, "job").mockResolvedValue(job());
  vi.spyOn(orchestratorApi, "resolveRequest").mockResolvedValue({ projectId: "p-1", status: "accepted", job: job(), message: "Original receipt found" });
  vi.spyOn(orchestratorApi, "cancel").mockResolvedValue(job({ status: "cancelled" }));
  vi.spyOn(orchestratorApi, "reconcile").mockResolvedValue(job({ status: "succeeded" }));
  vi.spyOn(orchestratorApi, "apply").mockResolvedValue(view({ revision: 4 }));
});
async function ready() {
  await screen.findByRole("button", { name: "Review design" });
  const disclosure = screen.getByText("Discuss or request a change");
  if (!(disclosure.parentElement as HTMLDetailsElement).open) await userEvent.click(disclosure);
  await screen.findByText(/Using Team provider/);
}

function responsiveOverview(initialCompact: boolean) {
  const original = window.matchMedia;
  const listeners = new Set<() => void>();
  const media = {
    matches: initialCompact,
    addEventListener: (_event: string, listener: () => void) => { listeners.add(listener); },
    removeEventListener: (_event: string, listener: () => void) => { listeners.delete(listener); },
  };
  vi.spyOn(window, "matchMedia").mockImplementation(query => query === "(max-width: 1100px)" ? media as unknown as MediaQueryList : original(query));
  return {
    resize(compact: boolean) { act(() => { media.matches = compact; listeners.forEach(listener => listener()); }); },
    listeners,
  };
}

it("starts compact screens with a labelled phase/context disclosure and opens only on request", async () => {
  const media = responsiveOverview(true);
  const mounted = render(<OrchestratorPanel projectId="p-1" />);
  await screen.findByText("Design · Saved context");
  const disclosure = screen.getByRole("button", { name: "Show project overview" });
  expect(disclosure.getAttribute("aria-expanded")).toBe("false");
  expect(screen.queryByRole("region", { name: "Current project understanding" })).toBeNull();
  expect(document.getElementById(disclosure.getAttribute("aria-controls")!)?.querySelector(".project-understanding")).not.toBeNull();
  expect(orchestratorApi.send).not.toHaveBeenCalled();
  await userEvent.click(disclosure);
  expect(screen.getByRole("button", { name: "Review design" })).not.toBeNull();
  expect(screen.getByRole("button", { name: "Hide project overview" }).getAttribute("aria-expanded")).toBe("true");
  mounted.unmount();
  expect(media.listeners.size).toBe(0);
});

it("adapts an untouched overview on resize without moving task focus, while preserving explicit open and close choices", async () => {
  const media = responsiveOverview(false);
  render(<><input aria-label="Main task draft" /><OrchestratorPanel projectId="p-1" /></>);
  await screen.findByRole("button", { name: "Review design" });
  const task = screen.getByRole("textbox", { name: "Main task draft" });
  task.focus(); media.resize(true);
  expect(screen.getByRole("button", { name: "Show project overview" })).not.toBeNull();
  expect(document.activeElement).toBe(task);
  media.resize(false);
  expect(screen.getByRole("button", { name: "Hide project overview" })).not.toBeNull();
  expect(document.activeElement).toBe(task);
  await userEvent.click(screen.getByRole("button", { name: "Hide project overview" }));
  media.resize(true); media.resize(false);
  expect(screen.getByRole("button", { name: "Show project overview" })).not.toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "Show project overview" }));
  media.resize(true);
  expect(screen.getByRole("button", { name: "Hide project overview" })).not.toBeNull();
  expect(orchestratorApi.send).not.toHaveBeenCalled();
});

it("retains the exact mounted conversation and unsent composer through compact disclosure changes", async () => {
  const media = responsiveOverview(true);
  vi.mocked(orchestratorApi.get).mockResolvedValue(view({ turns: [turn()] }));
  render(<OrchestratorPanel projectId="p-1" />);
  await screen.findByText("Design · Saved context");
  await userEvent.click(screen.getByRole("button", { name: "Show project overview" }));
  await ready();
  const composer = screen.getByRole("textbox", { name: "Message Luci" }) as HTMLTextAreaElement;
  await userEvent.type(composer, "Keep this unsent clarification while I work in the project.");
  const conversation = screen.getByRole("list", { name: "Conversation history" });
  await userEvent.click(screen.getByRole("button", { name: "Hide project overview" }));
  expect(screen.queryByRole("textbox", { name: "Message Luci" })).toBeNull();
  expect(composer.isConnected).toBe(true); expect(conversation.isConnected).toBe(true);
  media.resize(false); media.resize(true);
  await userEvent.click(screen.getByRole("button", { name: "Show project overview" }));
  expect(screen.getByRole("textbox", { name: "Message Luci" })).toBe(composer);
  expect(composer.value).toBe("Keep this unsent clarification while I work in the project.");
  expect(screen.getByRole("list", { name: "Conversation history" })).toBe(conversation);
  expect(screen.getByText("Support guest checkout")).not.toBeNull();
  expect(orchestratorApi.send).not.toHaveBeenCalled();
});

it("does not hide a focused composer on resize and keeps explicit project-open events working after a manual close", async () => {
  const media = responsiveOverview(false);
  render(<OrchestratorPanel projectId="p-1" />); await ready();
  const composer = screen.getByRole("textbox", { name: "Message Luci" }) as HTMLTextAreaElement;
  await userEvent.type(composer, "Preserve my active thought");
  media.resize(true);
  expect(screen.getByRole("button", { name: "Hide project overview" })).not.toBeNull();
  expect(document.activeElement).toBe(composer);
  await userEvent.click(screen.getByRole("button", { name: "Hide project overview" }));
  act(() => window.dispatchEvent(new CustomEvent(ORCHESTRATOR_OPEN, { detail: { projectId: "other" } })));
  expect(screen.getByRole("button", { name: "Show project overview" })).not.toBeNull();
  act(() => window.dispatchEvent(new CustomEvent(ORCHESTRATOR_OPEN, { detail: { projectId: "p-1" } })));
  await waitFor(() => expect(document.activeElement).toBe(composer));
  expect(composer.value).toBe("Preserve my active thought");
  media.resize(false); media.resize(true);
  expect(screen.getByRole("button", { name: "Hide project overview" })).not.toBeNull();
  expect(orchestratorApi.send).not.toHaveBeenCalled();
});

it("restores shared history without inference and keeps the same composer across phase changes", async () => {
  vi.mocked(orchestratorApi.get).mockResolvedValue(view({ turns: [turn()] }));
  render(<OrchestratorPanel projectId="p-1" />); await ready();
  expect(screen.getByText("Alice")).not.toBeNull();
  expect(screen.getByText("Support guest checkout")).not.toBeNull();
  const composer = screen.getByLabelText("Message Luci");
  await userEvent.type(composer, "Keep this thought while I review the design");
  await userEvent.click(screen.getByRole("button", { name: "Review design" }));
  expect(useCockpit.getState().lens).toBe("design");
  expect(screen.getByLabelText("Message Luci")).toBe(composer);
  expect((composer as HTMLTextAreaElement).value).toBe("Keep this thought while I review the design");
  expect(orchestratorApi.send).not.toHaveBeenCalled();
  expect(orchestratorApi.apply).not.toHaveBeenCalled();
});

it("opens and focuses only this project's composer without sending or overwriting an unsent question", async () => {
  render(<OrchestratorPanel projectId="p-1" />); await ready();
  await userEvent.click(screen.getByRole("button", { name: "Hide project overview" }));
  act(() => window.dispatchEvent(new CustomEvent(ORCHESTRATOR_OPEN, { detail: { projectId: "other", message: "Foreign draft" } })));
  expect(screen.getByRole("button", { name: "Show project overview" })).not.toBeNull();
  act(() => window.dispatchEvent(new CustomEvent(ORCHESTRATOR_OPEN, { detail: { projectId: "p-1", message: "My idea" } })));
  const composer = screen.getByLabelText("Message Luci") as HTMLTextAreaElement;
  await waitFor(() => expect(document.activeElement).toBe(composer));
  expect(composer.value).toBe("My idea");
  act(() => window.dispatchEvent(new CustomEvent(ORCHESTRATOR_OPEN, { detail: { projectId: "p-1", message: "Replace it" } })));
  expect(composer.value).toBe("My idea");
  expect(orchestratorApi.send).not.toHaveBeenCalled();
});

it("honors the chosen saved connection/model on explicit Send and preserves it across settings refresh and reload", async () => {
  const mounted = render(<OrchestratorPanel projectId="p-1" />); await ready();
  await userEvent.click(screen.getByText("Agent and model"));
  await userEvent.selectOptions(screen.getByLabelText("Luci connection"), "alternative");
  vi.mocked(productApi.agentConnections).mockResolvedValue(catalogue("gateway"));
  act(() => window.dispatchEvent(new Event(AGENT_CONNECTIONS_CHANGED)));
  await waitFor(() => expect(productApi.agentConnections).toHaveBeenCalledTimes(2));
  expect((screen.getByLabelText("Luci connection") as HTMLSelectElement).value).toBe("alternative");
  expect(orchestratorApi.send).not.toHaveBeenCalled();
  await userEvent.type(screen.getByLabelText("Message Luci"), "What should we clarify?");
  await userEvent.click(screen.getByRole("button", { name: "Send to Luci" }));
  expect(orchestratorApi.send).toHaveBeenCalledWith("p-1", expect.objectContaining({ agentConnectionId: "alternative", model: "model-alternative", harness: "codex", message: "What should we clarify?", expectedRevision: 3, expectedContextDigest: context().digest, idempotencyKey: expect.any(String) }));
  mounted.unmount(); render(<OrchestratorPanel projectId="p-1" />);
  await userEvent.click(screen.getByText("Discuss or request a change"));
  await screen.findByText(/Using Alternative provider/);
  expect((screen.getByLabelText("Luci connection") as HTMLSelectElement).value).toBe("alternative");
  expect(orchestratorApi.send).toHaveBeenCalledTimes(1);
});

it("explains material ambiguity and pre-fills an attributed reply only until Send", async () => {
  vi.mocked(orchestratorApi.get).mockResolvedValue(view({ turns: [turn()], blockingQuestions: [{ turnId: "turn-1", question: "Can guests book without an account?", rationale: "Privacy changes", options: ["Allow guest booking"] }] }));
  render(<OrchestratorPanel projectId="p-1" />); await ready();
  expect(screen.getByText("Definition and data access")).not.toBeNull();
  expect(screen.getByRole("region", { name: "Unresolved decisions" })).not.toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "Allow guest booking" }));
  expect((screen.getByLabelText("Message Luci") as HTMLTextAreaElement).value).toBe("Allow guest booking");
  expect(orchestratorApi.send).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole("button", { name: "Send to Luci" }));
  expect(orchestratorApi.send).toHaveBeenCalledWith("p-1", expect.objectContaining({ replyToTurnId: "turn-1", message: "Allow guest booking" }));
});

it.each(["Answer the open question", "Work through the decisions"])("binds the actual unresolved question when a person types an answer from %s", async label => {
  vi.mocked(orchestratorApi.get).mockResolvedValue(view({
    turns: [turn({ id: "older-question", questionResolvedBy: "already-answered" })],
    blockingQuestions: [{ turnId: "current-privacy-decision", question: "Can guests see other people's bookings?", rationale: "This changes the privacy boundary.", options: [] }],
    nextAction: { id: "answer-question", label: "Answer the open question", reason: "Decide who can see a booking.", lens: "discover" },
  }));
  render(<OrchestratorPanel projectId="p-1" />);
  await screen.findByRole("button", { name: label });
  expect(screen.queryByRole("textbox", { name: "Message Luci" })).toBeNull();
  await userEvent.click(screen.getByRole("button", { name: label }));
  const composer = screen.getByRole("textbox", { name: "Message Luci" });
  await waitFor(() => expect(document.activeElement).toBe(composer));
  expect(screen.getByText("Replying to: Can guests see other people's bookings?")).not.toBeNull();
  expect(orchestratorApi.send).not.toHaveBeenCalled();
  await userEvent.type(composer, "No. Each guest can see only their own booking.");
  await userEvent.click(screen.getByRole("button", { name: "Send to Luci" }));
  expect(orchestratorApi.send).toHaveBeenCalledWith("p-1", expect.objectContaining({ replyToTurnId: "current-privacy-decision", message: "No. Each guest can see only their own booking." }));
});

it("keeps Change the plan generic after a decision reply was selected", async () => {
  vi.mocked(orchestratorApi.get).mockResolvedValue(view({ blockingQuestions: [{ turnId: "privacy-decision", question: "Private bookings?", rationale: "Privacy", options: [] }] }));
  render(<OrchestratorPanel projectId="p-1" />);
  await screen.findByRole("button", { name: "Work through the decisions" });
  await userEvent.click(screen.getByRole("button", { name: "Work through the decisions" }));
  await userEvent.click(screen.getByRole("button", { name: "Change the plan" }));
  await userEvent.click(screen.getByRole("button", { name: "Discuss a change with Luci" }));
  expect(screen.queryByRole("button", { name: "Clear reply" })).toBeNull();
  await userEvent.type(screen.getByRole("textbox", { name: "Message Luci" }), "Add a calendar export");
  await userEvent.click(screen.getByRole("button", { name: "Send to Luci" }));
  expect(orchestratorApi.send).toHaveBeenCalledTimes(1);
  expect(vi.mocked(orchestratorApi.send).mock.calls[0][1].replyToTurnId).toBeUndefined();
});

it("reviews actual changes and saves only after explicit acknowledgement and exact CAS Apply", async () => {
  vi.mocked(orchestratorApi.get).mockResolvedValue(view({ turns: [proposalTurn()] }));
  render(<OrchestratorPanel projectId="p-1" />); await ready();
  await userEvent.click(screen.getByRole("button", { name: "Review amendment" }));
  const dialog = screen.getByRole("dialog", { name: "Review definition amendment" });
  expect(within(dialog).getByText("A booking service")).not.toBeNull();
  expect(within(dialog).getByText("Booking with guest checkout")).not.toBeNull();
  expect(within(dialog).getByText(/A guest receives a private recovery link/)).not.toBeNull();
  expect(orchestratorApi.apply).not.toHaveBeenCalled();
  expect((within(dialog).getByRole("button", { name: "Apply reviewed amendment" }) as HTMLButtonElement).disabled).toBe(true);
  await userEvent.click(within(dialog).getByRole("checkbox"));
  await userEvent.click(within(dialog).getByRole("button", { name: "Apply reviewed amendment" }));
  expect(orchestratorApi.apply).toHaveBeenCalledWith("p-1", "turn-1", { expectedRevision: 3, expectedContextDigest: context().digest });
  await screen.findByText(/Amendment saved as a draft/);
  expect(useCockpit.getState().projectSourceRevisions["p-1"]).toBe(1);
  expect(orchestratorApi.send).not.toHaveBeenCalled();
});

it("lets a person decline a proposal and blocks stale proposals without overwriting current context", async () => {
  vi.mocked(orchestratorApi.get).mockResolvedValue(view({ turns: [proposalTurn()], context: { ...context(), digest: "newer-context" } }));
  render(<OrchestratorPanel projectId="p-1" />); await ready();
  await userEvent.click(screen.getByRole("button", { name: "Review amendment" }));
  await userEvent.click(screen.getByRole("checkbox"));
  expect(screen.getByText(/saved context changed after this proposal/)).not.toBeNull();
  expect((screen.getByRole("button", { name: "Apply reviewed amendment" }) as HTMLButtonElement).disabled).toBe(true);
  await userEvent.click(screen.getByRole("button", { name: "Keep current context" }));
  expect(screen.queryByRole("dialog")).toBeNull();
  expect(orchestratorApi.apply).not.toHaveBeenCalled();
});

it("preserves an amendment on a save conflict and leaves edited source untouched", async () => {
  vi.mocked(orchestratorApi.get).mockResolvedValue(view({ turns: [proposalTurn()] }));
  vi.mocked(orchestratorApi.apply).mockRejectedValue(new Error("The context changed (409)."));
  render(<OrchestratorPanel projectId="p-1" />); await ready();
  await userEvent.click(screen.getByRole("button", { name: "Review amendment" }));
  await userEvent.click(screen.getByRole("checkbox"));
  await userEvent.click(screen.getByRole("button", { name: "Apply reviewed amendment" }));
  await waitFor(() => expect(screen.getByRole("dialog").textContent).toContain("context changed (409)"));
  expect(screen.getByRole("dialog").textContent).toContain("Booking with guest checkout");
  expect(useCockpit.getState().projectSourceRevisions["p-1"]).toBeUndefined();
});

it("blocks sending/applying over unsaved edits and also preserves edits begun while Apply is in flight", async () => {
  vi.mocked(orchestratorApi.get).mockResolvedValue(view({ turns: [proposalTurn()] }));
  render(<OrchestratorPanel projectId="p-1" />); await ready();
  act(() => useCockpit.getState().setUnsavedProjectDraft("editor", { projectId: "p-1", label: "Definition", lens: "discover" }));
  await userEvent.type(screen.getByLabelText("Message Luci"), "Amend this");
  expect((screen.getByRole("button", { name: "Send to Luci" }) as HTMLButtonElement).disabled).toBe(true);
  await userEvent.click(screen.getByRole("button", { name: "Review amendment" }));
  await userEvent.click(screen.getByRole("checkbox"));
  expect((screen.getByRole("button", { name: "Apply reviewed amendment" }) as HTMLButtonElement).disabled).toBe(true);
  act(() => useCockpit.getState().setUnsavedProjectDraft("editor", null));
  let finish!: (value: OrchestratorView) => void;
  vi.mocked(orchestratorApi.apply).mockReturnValue(new Promise(resolve => { finish = resolve; }));
  await userEvent.click(screen.getByRole("button", { name: "Apply reviewed amendment" }));
  act(() => useCockpit.getState().setUnsavedProjectDraft("editor", { projectId: "p-1", label: "Definition", lens: "discover" }));
  await act(async () => finish(view({ revision: 4 })));
  expect(useCockpit.getState().projectSourceRevisions["p-1"]).toBeUndefined();
  expect(screen.getByText(/Your newer unsaved edits remain/)).not.toBeNull();
  act(() => useCockpit.getState().setUnsavedProjectDraft("editor", null));
  await waitFor(() => expect(useCockpit.getState().projectSourceRevisions["p-1"]).toBe(1));
});

it.each(["actor", "connection", "root", "project", "team"] as const)("fences late private conversation replies when the selected %s changes", async changed => {
  let finish!: (value: OrchestratorView) => void;
  vi.mocked(orchestratorApi.get).mockReturnValueOnce(new Promise(resolve => { finish = resolve; }));
  const mounted = render(<OrchestratorPanel projectId="p-1" />);
  if (changed === "team") selectTeamScope("team-abcdefghijklmnopqrstuv");
  else if (changed !== "project") identity[changed] = "other";
  mounted.rerender(<OrchestratorPanel projectId={changed === "project" ? "p-2" : "p-1"} />);
  await ready();
  await act(async () => finish(view({ turns: [turn()] })));
  expect(screen.queryByText("Support guest checkout")).toBeNull();
  expect(orchestratorApi.send).not.toHaveBeenCalled();
});

it("keeps project facts visible by default and opens discussion only for an explicit change request", async () => {
  render(<OrchestratorPanel projectId="p-1" />);
  await screen.findByRole("button", { name: "Review design" });
  expect(screen.getByRole("complementary", { name: "Project understanding" })).not.toBeNull();
  expect(screen.queryByRole("textbox", { name: "Message Luci" })).toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "Change the plan" }));
  expect(screen.queryByRole("textbox", { name: "Message Luci" })).toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "Discuss a change with Luci" }));
  await waitFor(() => expect(document.activeElement).toBe(screen.getByRole("textbox", { name: "Message Luci" })));
  expect(orchestratorApi.send).not.toHaveBeenCalled();
});

it("restores an active request without replay and cancels only on an explicit click", async () => {
  vi.mocked(orchestratorApi.get).mockResolvedValue(view({ turns: [turn({ status: "running", answer: undefined })] }));
  render(<OrchestratorPanel projectId="p-1" />); await ready();
  expect(screen.getByText("Luci is working")).not.toBeNull();
  expect(orchestratorApi.send).not.toHaveBeenCalled();
  expect(orchestratorApi.cancel).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole("button", { name: "Cancel Luci request" }));
  expect(orchestratorApi.cancel).toHaveBeenCalledWith("p-1", "job-1");
  expect(orchestratorApi.send).not.toHaveBeenCalled();
});

it.each(["terminal", "missing"] as const)("recovers a %s job only with an explicit recording action, without inference", async kind => {
  vi.useFakeTimers();
  try {
    vi.mocked(orchestratorApi.get).mockResolvedValue(view({ turns: [turn({ status: "reserved", answer: undefined })] }));
    if (kind === "missing") vi.mocked(orchestratorApi.job).mockRejectedValue(new ApiError("project_not_found", "The agent job could not be found", 404));
    else vi.mocked(orchestratorApi.job).mockResolvedValue(job({ id: "job-1", status: "succeeded" }));
    render(<OrchestratorPanel projectId="p-1" />);
    await act(async () => {});
    act(() => window.dispatchEvent(new CustomEvent(ORCHESTRATOR_OPEN, { detail: { projectId: "p-1" } })));
    await act(async () => { await vi.advanceTimersByTimeAsync(2500); });
    expect(screen.getByRole("button", { name: "Recover recorded response" })).not.toBeNull();
    expect(orchestratorApi.reconcile).not.toHaveBeenCalled();
    vi.mocked(orchestratorApi.get).mockResolvedValue(view({ revision: 4, turns: [kind === "terminal" ? turn() : turn({ status: "failed", answer: undefined })] }));
    await act(async () => fireEvent.click(screen.getByRole("button", { name: "Recover recorded response" })));
    expect(orchestratorApi.reconcile).toHaveBeenCalledWith("p-1", "job-1");
    expect(screen.getByText(kind === "terminal" ? "We need to decide how guests recover their booking." : "Request failed")).not.toBeNull();
    expect(orchestratorApi.send).not.toHaveBeenCalled();
  } finally { vi.useRealTimers(); }
});

it("ignores a late send response after access is revoked and restored", async () => {
  let finish!: (value: OrchestratorJob) => void;
  vi.mocked(orchestratorApi.send).mockReturnValue(new Promise(resolve => { finish = resolve; }));
  const mounted = render(<OrchestratorPanel projectId="p-1" />); await ready();
  await userEvent.type(screen.getByLabelText("Message Luci"), "My pending question");
  await userEvent.click(screen.getByRole("button", { name: "Send to Luci" }));
  identity.read = false; mounted.rerender(<OrchestratorPanel projectId="p-1" />);
  expect(screen.queryByRole("textbox", { name: "Message Luci" })).toBeNull();
  identity.read = true; mounted.rerender(<OrchestratorPanel projectId="p-1" />); await ready();
  await act(async () => finish(job()));
  expect((screen.getByLabelText("Message Luci") as HTMLTextAreaElement).value).toBe("My pending question");
  expect(screen.queryByRole("button", { name: "Cancel Luci request" })).toBeNull();
});

it("clears private conversation on logout and reads without inviting execution for a viewer", async () => {
  identity.edit = false; identity.assist = false;
  vi.mocked(orchestratorApi.get).mockResolvedValue(view({ turns: [turn()] }));
  const mounted = render(<OrchestratorPanel projectId="p-1" />); await ready();
  expect(screen.getByText("Alice")).not.toBeNull();
  expect((screen.getByLabelText("Message Luci") as HTMLTextAreaElement).disabled).toBe(true);
  expect((screen.getByRole("button", { name: "Send to Luci" }) as HTMLButtonElement).disabled).toBe(true);
  identity.actor = ""; mounted.rerender(<OrchestratorPanel projectId="p-1" />);
  expect(screen.queryByRole("complementary")).toBeNull();
  expect(orchestratorApi.send).not.toHaveBeenCalled();
});

it("does not replace newer current turns or context with a late historical page", async () => {
  const current = view({ revision: 8, turns: [turn({ sequence: 21, id: "latest", message: "Current decision" })], hasEarlier: true, beforeSequence: 21 });
  vi.mocked(orchestratorApi.get).mockResolvedValue(current);
  render(<OrchestratorPanel projectId="p-1" />); await ready();
  vi.mocked(orchestratorApi.get).mockResolvedValue(view({ revision: 2, turns: [turn({ id: "older", message: "Earlier idea" })], context: { ...context(), digest: "obsolete" }, nextAction: { id: "obsolete", label: "Obsolete next action", reason: "", lens: "discover" } }));
  await userEvent.click(screen.getByRole("button", { name: "Show earlier conversation" }));
  await screen.findByText("Earlier idea");
  expect(screen.getByText("Current decision")).not.toBeNull();
  expect(screen.queryByRole("button", { name: "Obsolete next action" })).toBeNull();
  await userEvent.type(screen.getByLabelText("Message Luci"), "Continue from current context");
  await userEvent.click(screen.getByRole("button", { name: "Send to Luci" }));
  expect(orchestratorApi.send).toHaveBeenCalledWith("p-1", expect.objectContaining({ expectedRevision: 8, expectedContextDigest: context().digest }));
});

it("reports missing provider and uncertain send honestly, keeping text without automatic retries", async () => {
  vi.mocked(productApi.agentConnections).mockResolvedValue({ revision: 1, activeConnectionId: "", connections: [] });
  render(<OrchestratorPanel projectId="p-1" />);
  await userEvent.click(screen.getByText("Discuss or request a change"));
  await screen.findByText(/Choose a saved connection, or enter the exact model/);
  await userEvent.type(screen.getByLabelText("Message Luci"), "My unsent idea");
  expect((screen.getByRole("button", { name: "Send to Luci" }) as HTMLButtonElement).disabled).toBe(true);
  await userEvent.click(screen.getByText("Agent and model"));
  await userEvent.type(screen.getByLabelText("Luci model"), "own-model");
  vi.mocked(orchestratorApi.send).mockRejectedValue(new Error("Provider credentials are unavailable."));
  await userEvent.click(screen.getByRole("button", { name: "Send to Luci" }));
  await screen.findByText(/Provider credentials are unavailable/);
  expect((screen.getByLabelText("Message Luci") as HTMLTextAreaElement).value).toBe("My unsent idea");
  expect((screen.getByRole("button", { name: "Send to Luci" }) as HTMLButtonElement).disabled).toBe(true);
  expect(orchestratorApi.send).toHaveBeenCalledTimes(1);
});

const currentScope = (projectId = "p-1") => JSON.stringify([productApi.baseUrl, identity.actor, identity.connection ? "cloud" : "local", identity.connection, identity.connection, identity.actor, identity.root, projectId]);
it("refreshes immediately after a real definition save and approval without remounting the editor or losing a newer draft", async () => {
  let saved = context().specification;
  const getDefinition = vi.spyOn(definitionApi, "get").mockImplementation(async () => saved);
  vi.spyOn(definitionApi, "save").mockImplementation(async (_id, value) => { saved = { ...value, revision: saved.revision + 1 }; return saved; });
  vi.spyOn(definitionApi, "approve").mockImplementation(async () => { saved = { ...saved, revision: saved.revision + 1, status: "approved", digest: "b".repeat(64) }; return saved; });
  vi.mocked(orchestratorApi.get).mockImplementation(async () => view({ context: { ...context(), specification: saved } }));
  render(<><DefinitionWorkspace projectId="p-1" /><OrchestratorPanel projectId="p-1" /></>);
  const idea = await screen.findByRole("textbox", { name: "The idea" });
  await userEvent.clear(idea); await userEvent.type(idea, "A changed booking application");
  const calls = vi.mocked(orchestratorApi.get).mock.calls.length;
  await userEvent.click(screen.getByRole("button", { name: "Save definition" }));
  const overview = screen.getByRole("region", { name: "Current project understanding" });
  await waitFor(() => expect(within(overview).getByText("A changed booking application")).not.toBeNull());
  expect(orchestratorApi.get).toHaveBeenCalledTimes(calls + 1);
  await userEvent.click(screen.getByRole("button", { name: "Approve definition" }));
  await waitFor(() => expect(within(overview).getByText(`Reviewed definition · Revision ${saved.revision}`)).not.toBeNull());
  expect(getDefinition).toHaveBeenCalledOnce();
  expect(screen.getByRole("textbox", { name: "The idea" })).toBe(idea);
  await userEvent.type(idea, " with a newer unsaved decision");
  act(() => notifyProjectContextChanged(currentScope(), "p-1"));
  await waitFor(() => expect(orchestratorApi.get).toHaveBeenCalledTimes(calls + 3));
  expect((idea as HTMLTextAreaElement).value).toContain("newer unsaved decision");
  expect(getDefinition).toHaveBeenCalledOnce();
  expect(useCockpit.getState().projectSourceRevisions["p-1"]).toBeUndefined();
});
it("fences an older overview read after a saved-context notification and ignores foreign account/project notifications", async () => {
  let finish!: (value: OrchestratorView) => void;
  vi.mocked(orchestratorApi.get).mockReturnValueOnce(new Promise(resolve => { finish = resolve; }));
  render(<OrchestratorPanel projectId="p-1" />);
  const changed = context(); changed.specification.purpose = "Current saved work";
  vi.mocked(orchestratorApi.get).mockResolvedValue(view({ context: changed }));
  act(() => notifyProjectContextChanged("foreign-account", "p-1"));
  act(() => notifyProjectContextChanged(currentScope("p-2"), "p-2"));
  expect(orchestratorApi.get).toHaveBeenCalledOnce();
  act(() => notifyProjectContextChanged(currentScope(), "p-1"));
  await screen.findByText("Current saved work");
  await act(async () => finish(view()));
  expect(screen.getByText("Current saved work")).not.toBeNull();
  expect(screen.queryByText(context().specification.purpose)).toBeNull();
});
it("queues context refresh behind an in-flight Send without orphaning its response or replaying inference", async () => {
  render(<OrchestratorPanel projectId="p-1" />); await ready();
  let finish!: (value: OrchestratorJob) => void;
  vi.mocked(orchestratorApi.send).mockReturnValue(new Promise(resolve => { finish = resolve; }));
  await userEvent.type(screen.getByLabelText("Message Luci"), "Review the saved change");
  await userEvent.click(screen.getByRole("button", { name: "Send to Luci" }));
  act(() => notifyProjectContextChanged(currentScope(), "p-1"));
  expect(orchestratorApi.get).toHaveBeenCalledOnce();
  await act(async () => finish(job({ status: "succeeded" })));
  await waitFor(() => expect(orchestratorApi.get).toHaveBeenCalledTimes(3));
  expect(orchestratorApi.send).toHaveBeenCalledOnce();
  expect(screen.queryByText("Sending…")).toBeNull();
  expect((screen.getByLabelText("Message Luci") as HTMLTextAreaElement).value).toBe("");
});
it("offers manual editing without a provider and delivers its exact target after the editor mounts", async () => {
  vi.mocked(productApi.agentConnections).mockResolvedValue({ revision: 1, activeConnectionId: "", connections: [] });
  let finish!: (value: ReturnType<typeof context>["specification"]) => void;
  vi.spyOn(definitionApi, "get").mockReturnValue(new Promise(resolve => { finish = resolve; }));
  vi.spyOn(definitionApi, "history").mockResolvedValue({ revisions: [] });
  useCockpit.setState({ lens: "design" });
  function Workspace() { const lens = useCockpit(state => state.lens); return <><OrchestratorPanel projectId="p-1" />{lens === "discover" && <DefinitionWorkspace projectId="p-1" />}</>; }
  render(<Workspace />);
  await screen.findByRole("button", { name: "Review design" });
  await userEvent.click(screen.getByRole("button", { name: "Change the plan" }));
  expect(screen.queryByRole("textbox", { name: "Message Luci" })).toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "Edit outcomes" }));
  await act(async () => finish(context().specification));
  const idea = await screen.findByRole("textbox", { name: "The idea" });
  await userEvent.type(idea, " retained draft");
  await userEvent.click(screen.getByRole("tab", { name: "Revision history" }));
  await userEvent.click(screen.getByRole("button", { name: "Change the plan" }));
  await userEvent.click(screen.getByRole("button", { name: "Edit outcomes" }));
  expect((screen.getByRole("textbox", { name: "The idea" }) as HTMLTextAreaElement).value).toContain("retained draft");
  expect(definitionApi.get).toHaveBeenCalledOnce();
  expect(orchestratorApi.send).not.toHaveBeenCalled(); expect(orchestratorApi.apply).not.toHaveBeenCalled();
});
it("discards a pending manual edit on account change instead of replaying it when the old account returns", async () => {
  const mounted = render(<OrchestratorPanel projectId="p-1" />); await ready();
  const oldScope = currentScope();
  await userEvent.click(screen.getByRole("button", { name: "Change the plan" }));
  await userEvent.click(screen.getByRole("button", { name: "Edit outcomes" }));
  identity.actor = "other"; mounted.rerender(<OrchestratorPanel projectId="p-1" />); await ready();
  expect(takeProjectContextEdit(oldScope, "p-1", "definition")).toBe(false);
  expect(orchestratorApi.send).not.toHaveBeenCalled();
});

it("retains an unsent scoped message and reply target after the conversation remounts", async () => {
  vi.mocked(orchestratorApi.get).mockResolvedValue(view({ turns: [turn()], blockingQuestions: [{ turnId: "turn-1", question: "Can guests book without an account?", rationale: "Privacy", options: [] }] }));
  const mounted = render(<OrchestratorPanel projectId="p-1" />); await ready();
  await userEvent.click(screen.getByRole("button", { name: "Answer this decision" }));
  await userEvent.type(screen.getByRole("textbox", { name: "Message Luci" }), "Keep this private answer while I review the plan");
  mounted.unmount(); render(<OrchestratorPanel projectId="p-1" />); await ready();
  expect((screen.getByRole("textbox", { name: "Message Luci" }) as HTMLTextAreaElement).value).toBe("");
  await userEvent.click(screen.getByRole("button", { name: "Restore unsent message" }));
  expect((screen.getByRole("textbox", { name: "Message Luci" }) as HTMLTextAreaElement).value).toBe("Keep this private answer while I review the plan");
  expect(screen.getByText("Replying to: Can guests book without an account?")).not.toBeNull();
  expect(orchestratorApi.send).not.toHaveBeenCalled();
});

it("retains an uncertain request through remount and refresh, and retries its exact original identity", async () => {
  vi.mocked(orchestratorApi.send).mockRejectedValueOnce(new Error("Response lost"));
  const mounted = render(<OrchestratorPanel projectId="p-1" />); await ready();
  await userEvent.type(screen.getByLabelText("Message Luci"), "Discuss a private change");
  await userEvent.click(screen.getByRole("button", { name: "Send to Luci" }));
  await screen.findByText(/Response lost/);
  const original = vi.mocked(orchestratorApi.send).mock.calls[0][1];
  mounted.unmount();
  vi.mocked(orchestratorApi.get).mockResolvedValue(view({ revision: 90, context: { ...context(), digest: "b".repeat(64) } }));
  render(<OrchestratorPanel projectId="p-1" />); await ready();
  expect((screen.getByLabelText("Message Luci") as HTMLTextAreaElement).value).toBe("");
  expect(orchestratorApi.send).toHaveBeenCalledTimes(1);
  await userEvent.click(screen.getByRole("button", { name: "Refresh conversation" }));
  expect((screen.getByRole("button", { name: "Send to Luci" }) as HTMLButtonElement).disabled).toBe(true);
  await userEvent.click(screen.getByRole("button", { name: "Review original request" }));
  expect((screen.getByLabelText("Message Luci") as HTMLTextAreaElement).value).toBe("Discuss a private change");
  await userEvent.click(screen.getByRole("button", { name: "Retry original request" }));
  expect(orchestratorApi.send).toHaveBeenCalledTimes(2);
  expect(vi.mocked(orchestratorApi.send).mock.calls[1][1]).toEqual(original);
  expect(original.expectedRevision).toBe(3);
});

it("uses a durable not-sent receipt before restoring an editable message against new context", async () => {
  vi.mocked(orchestratorApi.send).mockRejectedValueOnce(new Error("Connection interrupted"));
  vi.mocked(orchestratorApi.resolveRequest).mockResolvedValue({ projectId: "p-1", status: "not-sent", message: "The original key is fenced" });
  render(<OrchestratorPanel projectId="p-1" />); await ready();
  await userEvent.type(screen.getByLabelText("Message Luci"), "Retain my question");
  await userEvent.click(screen.getByRole("button", { name: "Send to Luci" }));
  await screen.findByText(/Connection interrupted/);
  const original = vi.mocked(orchestratorApi.send).mock.calls[0][1];
  vi.mocked(orchestratorApi.get).mockResolvedValue(view({ revision: 8 }));
  await userEvent.click(screen.getByRole("button", { name: "Check original request" }));
  await screen.findByText(/was not sent and can no longer start/);
  expect(orchestratorApi.resolveRequest).toHaveBeenCalledWith("p-1", original);
  expect(orchestratorApi.send).toHaveBeenCalledTimes(1);
  await userEvent.click(screen.getByRole("button", { name: "Restore unsent message" }));
  await userEvent.click(screen.getByRole("button", { name: "Send to Luci" }));
  const next = vi.mocked(orchestratorApi.send).mock.calls[1][1];
  expect(next.message).toBe(original.message); expect(next.expectedRevision).toBe(8);
  expect(next.idempotencyKey).not.toBe(original.idempotencyKey);
});

it("keeps unknown outcomes blocked, and an accepted receipt clears the draft without resending", async () => {
  vi.mocked(orchestratorApi.send).mockRejectedValueOnce(new Error("Timeout"));
  vi.mocked(orchestratorApi.resolveRequest).mockResolvedValueOnce({ projectId: "p-1", status: "unknown", message: "Original job unavailable; retained for recovery" });
  const mounted = render(<OrchestratorPanel projectId="p-1" />); await ready();
  await userEvent.type(screen.getByLabelText("Message Luci"), "Original question");
  await userEvent.click(screen.getByRole("button", { name: "Send to Luci" })); await screen.findByText(/Timeout/);
  await userEvent.click(screen.getByRole("button", { name: "Check original request" }));
  await screen.findByText(/Original job unavailable/);
  expect((screen.getByRole("button", { name: "Send to Luci" }) as HTMLButtonElement).disabled).toBe(true);
  expect(screen.queryByRole("button", { name: "Discard unsent message" })).toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "Check original request" }));
  await screen.findByText(/original request was accepted/);
  expect(orchestratorApi.send).toHaveBeenCalledTimes(1);
  mounted.unmount(); render(<OrchestratorPanel projectId="p-1" />); await ready();
  expect(screen.queryByRole("button", { name: "Restore unsent message" })).toBeNull();
  expect(screen.queryByRole("button", { name: "Check original request" })).toBeNull();
  expect((screen.getByLabelText("Message Luci") as HTMLTextAreaElement).value).toBe("");
});

it.each(["actor", "connection", "root", "project", "team"])("keeps an unsent message private across %s scope navigation", async changed => {
  const mounted = render(<OrchestratorPanel projectId="p-1" />); await ready();
  await userEvent.type(screen.getByLabelText("Message Luci"), "Only in my original scope");
  if (changed === "actor") identity.actor = "second-user";
  if (changed === "connection") identity.connection = "another-host";
  if (changed === "root") identity.root = "/different/root";
  if (changed === "team") selectTeamScope("team-AAAAAAAAAAAAAAAAAAAAAA");
  mounted.rerender(<OrchestratorPanel projectId={changed === "project" ? "p-2" : "p-1"} />); await ready();
  expect((screen.getByLabelText("Message Luci") as HTMLTextAreaElement).value).toBe("");
  expect(screen.queryByRole("button", { name: "Restore unsent message" })).toBeNull();
  Object.assign(identity, { actor: "owner", connection: "", root: "/projects/owner" }); selectTeamScope("");
  mounted.rerender(<OrchestratorPanel projectId="p-1" />); await ready();
  await userEvent.click(screen.getByRole("button", { name: "Restore unsent message" }));
  expect((screen.getByLabelText("Message Luci") as HTMLTextAreaElement).value).toBe("Only in my original scope");
  expect(orchestratorApi.send).not.toHaveBeenCalled();
});

it("hides recovery and private composer content when permission is revoked", async () => {
  const mounted = render(<OrchestratorPanel projectId="p-1" />); await ready();
  await userEvent.type(screen.getByLabelText("Message Luci"), "Private unsent text");
  identity.assist = false; mounted.rerender(<OrchestratorPanel projectId="p-1" />);
  expect((screen.getByLabelText("Message Luci") as HTMLTextAreaElement).value).toBe("");
  expect(screen.queryByRole("button", { name: "Restore unsent message" })).toBeNull();
  identity.read = false; mounted.rerender(<OrchestratorPanel projectId="p-1" />);
  expect(screen.queryByRole("textbox", { name: "Message Luci" })).toBeNull();
  expect(orchestratorApi.send).not.toHaveBeenCalled();
});

it("blocks paid dispatch if its recovery identity cannot be stored", async () => {
  render(<OrchestratorPanel projectId="p-1" />); await ready();
  await userEvent.type(screen.getByLabelText("Message Luci"), "Keep a copy before sending");
  vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => { throw new Error("Storage denied"); });
  await userEvent.click(screen.getByRole("button", { name: "Send to Luci" }));
  await screen.findByText(/enable browser storage before sending/);
  expect(orchestratorApi.send).not.toHaveBeenCalled();
  expect((screen.getByLabelText("Message Luci") as HTMLTextAreaElement).value).toBe("Keep a copy before sending");
});

it("lets a demoted reader resolve their original request without allowing another paid send", async () => {
  vi.mocked(orchestratorApi.send).mockRejectedValueOnce(new Error("Lost response"));
  const mounted = render(<OrchestratorPanel projectId="p-1" />); await ready();
  await userEvent.type(screen.getByLabelText("Message Luci"), "Private original prompt");
  await userEvent.click(screen.getByRole("button", { name: "Send to Luci" })); await screen.findByText(/Lost response/);
  identity.assist = false; identity.edit = false; mounted.rerender(<OrchestratorPanel projectId="p-1" />);
  expect(screen.queryByRole("button", { name: "Retry original request" })).toBeNull();
  expect((screen.getByLabelText("Message Luci") as HTMLTextAreaElement).value).toBe("");
  await userEvent.click(screen.getByRole("button", { name: "Check original request" }));
  await screen.findByText(/original request was accepted/);
  expect(orchestratorApi.send).toHaveBeenCalledTimes(1);
  expect((screen.getByRole("button", { name: "Send to Luci" }) as HTMLButtonElement).disabled).toBe(true);
});

it("does not resend stale text after another tab clears the saved draft", async () => {
  render(<OrchestratorPanel projectId="p-1" />); await ready();
  await userEvent.type(screen.getByLabelText("Message Luci"), "Text accepted in another tab");
  const key = Object.keys(localStorage).find(key => key.startsWith("luci.orchestrator-composer") && key.endsWith(":draft"))!;
  const oldValue = localStorage.getItem(key); localStorage.removeItem(key);
  act(() => window.dispatchEvent(new StorageEvent("storage", { key, oldValue, newValue: null })));
  expect((screen.getByRole("button", { name: "Send to Luci" }) as HTMLButtonElement).disabled).toBe(true);
  await userEvent.click(screen.getByRole("button", { name: "Reload saved message" }));
  expect((screen.getByLabelText("Message Luci") as HTMLTextAreaElement).value).toBe("");
  expect(orchestratorApi.send).not.toHaveBeenCalled();
});
