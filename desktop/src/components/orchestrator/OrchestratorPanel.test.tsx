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
import { context, job, proposalTurn, turn, view } from "./fixtures";

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
