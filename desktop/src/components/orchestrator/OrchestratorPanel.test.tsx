import { aProject } from "../../test/fixtures";
import { StrictMode } from "react";
import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, expect, it, vi } from "vitest";
import { orchestratorApi, type OrchestratorJob, type OrchestratorView } from "../../api/orchestrator";
import { productApi, type AgentConnectionCatalogue } from "../../api/product";
import { ApiError } from "../../api/client";
import { coordinatorContinuationsApi } from "../../api/coordinatorContinuations";
import { coordinatorPolicyApi } from "../../api/coordinatorPolicy";
import { selectTeamScope } from "../../api/teamScope";
import { AGENT_CONNECTIONS_CHANGED } from "../../hooks/useAgentConnections";
import { useCockpit, resetCockpit } from "../../store/store";
import { ORCHESTRATOR_OPEN, OrchestratorPanel } from "./OrchestratorPanel";
import { notifyProjectContextChanged, takeProjectContextEdit } from "../../api/projectContextEvents";
import { DefinitionWorkspace } from "../definition/DefinitionWorkspace";
import { ProjectChecks } from "../journey/ProjectChecks";
import { DraftNavigationDialog } from "../common/DraftNavigationDialog";
import { definitionApi } from "../../api/definition";
import { context, job, proposalTurn, turn, view } from "./fixtures";
import { readComposerRecovery, retainComposerRequest, saveComposerDraft } from "./composerRecovery";

vi.mock("../journey/ContextStorage", () => ({ ContextStorage: () => null }));
// The team component has its own actual-card/API tests. Here exercise only the
// parent history/focus seam without introducing additional service reads.
vi.mock("./CoordinatorTeamIntents", () => ({ CoordinatorTeamIntents: ({ onDiscuss }: { onDiscuss(id: string): void }) => <button onClick={() => onDiscuss("turn-1")}>Open retained team conversation</button> }));
const identity = vi.hoisted(() => ({ actor: "owner", connection: "", root: "/projects/owner", read: true, edit: true, assist: true }));
vi.mock("../auth/AuthProvider", () => ({ useAuth: () => identity.actor ? { profile: { user: { id: identity.actor } } } : null }));
vi.mock("../setup/SetupProvider", () => ({ useSetup: () => ({ openAgents: vi.fn(), workspace: { mode: identity.connection ? "cloud" : "local", connectionId: identity.connection, host: identity.connection }, status: { access: { user: { username: identity.actor } }, config: { projectsDir: identity.root } } }) }));
vi.mock("../../hooks/useProjectAccess", () => ({ useProjectAccess: () => ({ canRead: identity.read, canEdit: identity.edit, canExecute: identity.assist, permissions: { deploy: identity.assist }, canAssist: identity.assist, message: identity.edit ? "" : "Read-only access." }) }));

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
  vi.spyOn(coordinatorContinuationsApi, "list").mockResolvedValue([]);
  vi.spyOn(coordinatorPolicyApi, "get").mockImplementation(async projectId => ({ projectId, revision: 0, mode: "review", reviewMode: "manual", allowedTargets: ["specification", "design-plan", "plan"], executionMode: "manual", deploymentMode: "manual" }));
});
async function ready() {
  await screen.findByRole("button", { name: "Review design" });
  const disclosure = screen.getByText("Discuss or request a change");
  if (!(disclosure.parentElement as HTMLDetailsElement).open) await userEvent.click(disclosure);
  await screen.findByText(/Using Team provider/);
}

it("shows accepted peer changes, actor-owned work and decisions in the primary conversation without replay", async () => {
  const accepted = { eventId: "event-1", sequence: 4, intentId: "intent-1", originTurnId: "turn-1",
    actor: { id: "bob", displayName: "Bob" }, target: "design-plan", appliedTargetDigest: "c".repeat(64), acceptedContextDigest: "d".repeat(64),
    taskIds: ["task-1"], affectsAllTasks: false };
  vi.mocked(orchestratorApi.get).mockImplementation(async id => view({ projectId: id, context: context(id),
    blockingQuestions: id === "p-1" ? [{ turnId: "question-1", question: "Which layout?", rationale: "The designs conflict.", options: ["Compact", "Wide"] }] : [],
    activity: id === "p-1" ? { accepted: [accepted], work: [{ id: "continue-1", originTurnId: "turn-1", contextDigest: accepted.acceptedContextDigest,
      acceptedEvents: [{ eventId: accepted.eventId, sequence: accepted.sequence, originIntentId: accepted.intentId }],
      status: "checks-verified", message: "The exact check is linked to reviewed source.", runId: "run-1", updatedAt: "2026-10-13T00:00:00Z",
      finding: { status: "verified", checkId: "check-1", commit: "a1b2c3", previewStatus: "ready", previewReleaseId: "preview-1" } }], earlierAvailable: false }
      : { accepted: [], work: [], earlierAvailable: false } }));
  const mounted = render(<OrchestratorPanel projectId="p-1" primary />);
  const progress = await screen.findByRole("region", { name: "Shared changes and affected work" });
  expect(within(progress).getByText(/Bob’s design-plan change was accepted/)).toBeTruthy();
  expect(within(progress).getAllByText(accepted.acceptedContextDigest)).toHaveLength(2);
  expect(within(progress).getByText(accepted.appliedTargetDigest)).toBeTruthy();
  expect(within(progress).getByText("check-1")).toBeTruthy();
  expect(within(progress).getByText("preview-1")).toBeTruthy();
  expect(within(progress).getByText(/1 decision needs an answer/)).toBeTruthy();
  expect(screen.getByRole("button", { name: "Answer this decision" })).toBeTruthy();
  expect(orchestratorApi.send).not.toHaveBeenCalled();
  mounted.rerender(<OrchestratorPanel projectId="p-2" primary />);
  await waitFor(() => expect(screen.queryByText(/Bob’s design-plan change was accepted/)).toBeNull());
  expect(orchestratorApi.send).not.toHaveBeenCalled();
});

it("uses the existing conversation mutation boundary for a policy-authorized draft and leaves Build/Deploy untouched", async () => {
  const proposal = { ...proposalTurn(), actor: { id: "owner", displayName: "Owner" } };
  const initial = view({ turns: [proposal] });
  vi.mocked(orchestratorApi.get).mockResolvedValue(initial);
  vi.mocked(coordinatorPolicyApi.get).mockResolvedValue({ projectId: "p-1", revision: 2, mode: "auto-drafts", reviewMode: "manual", allowedTargets: ["specification"], executionMode: "manual", deploymentMode: "manual", actor: proposal.actor, updatedAt: "2026-10-12T00:00:00Z" });
  const advance = vi.spyOn(orchestratorApi, "advance").mockResolvedValue(view({ revision: 4, context: { ...context(), digest: "saved-draft" }, turns: [{ ...proposal, appliedRevision: 3 }] }));
  render(<OrchestratorPanel projectId="p-1" />);
  await waitFor(() => expect(advance).toHaveBeenCalledExactlyOnceWith("p-1", proposal.id, { expectedRevision: 3, expectedContextDigest: context().digest, expectedPolicyRevision: 2 }));
  await userEvent.click(screen.getByRole("button", { name: "Talk to Luci" }));
  await screen.findByRole("button", { name: "Open current definition" });
  expect(orchestratorApi.send).not.toHaveBeenCalled(); expect(orchestratorApi.apply).not.toHaveBeenCalled();
});

it("does not automatically apply while this project has unsaved context", async () => {
  const proposal = { ...proposalTurn(), actor: { id: "owner", displayName: "Owner" } };
  vi.mocked(orchestratorApi.get).mockResolvedValue(view({ turns: [proposal] }));
  vi.mocked(coordinatorPolicyApi.get).mockResolvedValue({ projectId: "p-1", revision: 2, mode: "auto-drafts", reviewMode: "manual", allowedTargets: ["specification"], executionMode: "manual", deploymentMode: "manual", actor: proposal.actor, updatedAt: "2026-10-12T00:00:00Z" });
  const advance = vi.spyOn(orchestratorApi, "advance");
  useCockpit.setState({ unsavedProjectDrafts: { definition: { projectId: "p-1", label: "Definition", lens: "discover", retained: true, discard: vi.fn() } } });
  render(<OrchestratorPanel projectId="p-1" />); await ready();
  expect(advance).not.toHaveBeenCalled();
});

it.each(["resolved", "pending"])("keeps the parent context ahead of an older child observation when the newer read is %s", async state => {
  const oldView = view({ context: { ...context(), specification: { ...context().specification, purpose: "Old booking purpose" } } });
  const newer = view({ context: { ...context(), digest: "new-current-context", specification: { ...context().specification, purpose: "New booking purpose" } } });
  vi.mocked(orchestratorApi.get).mockResolvedValue(oldView);
  render(<OrchestratorPanel projectId="p-1" />); await ready();
  await userEvent.click(screen.getByText(/Context policy ·/));
  let oldResolve!: (value: OrchestratorView) => void, newResolve!: (value: OrchestratorView) => void;
  const oldRead = new Promise<OrchestratorView>(resolve => { oldResolve = resolve; });
  const newRead = new Promise<OrchestratorView>(resolve => { newResolve = resolve; });
  vi.mocked(orchestratorApi.get).mockReturnValueOnce(oldRead).mockReturnValueOnce(newRead);
  const reads = vi.mocked(orchestratorApi.get).mock.calls.length;
  await userEvent.click(screen.getByRole("button", { name: "Refresh policy and conversation" }));
  await waitFor(() => expect(orchestratorApi.get).toHaveBeenCalledTimes(reads + 1));
  await userEvent.click(screen.getByRole("button", { name: "Refresh project understanding" }));
  await waitFor(() => expect(orchestratorApi.get).toHaveBeenCalledTimes(reads + 2));
  if (state === "resolved") await act(async () => newResolve(newer));
  await act(async () => oldResolve(oldView));
  if (state === "pending") {
    expect(screen.getByText("Updating saved project understanding…")).not.toBeNull();
    expect(screen.getByRole("button", { name: "Review design" }).matches(":disabled")).toBe(true);
    await act(async () => newResolve(newer));
  }
  expect(screen.getByText("New booking purpose")).not.toBeNull();
  expect(screen.queryByText("Old booking purpose")).toBeNull();
  expect(orchestratorApi.apply).not.toHaveBeenCalled(); expect(orchestratorApi.send).not.toHaveBeenCalled();
});

it("opens the exact team conversation on compact screens without changing its retained draft or sending work", async () => {
  responsiveOverview(true);
  const current = proposalTurn();
  vi.mocked(orchestratorApi.get).mockResolvedValue(view({ turns: [current] }));
  render(<OrchestratorPanel projectId="p-1" />);
  await userEvent.click(await screen.findByRole("button", { name: "Talk to Luci" }));
  const composer = await screen.findByLabelText("Message Luci");
  await userEvent.type(composer, "Keep this separate unsent amendment.");
  const before = browserRecords();
  await userEvent.click(screen.getByRole("button", { name: "Open retained team conversation" }));
  const retained = within(screen.getByRole("list", { name: "Conversation history" })).getByText(current.message).closest("li");
  await waitFor(() => expect(document.activeElement).toBe(retained));
  expect((composer as HTMLTextAreaElement).value).toBe("Keep this separate unsent amendment.");
  expect(browserRecords()).toEqual(before); expect(orchestratorApi.send).not.toHaveBeenCalled();
  expect(orchestratorApi.apply).not.toHaveBeenCalled();
});

it("explains an older team conversation without fabricating a reply or fetching/mutating it automatically", async () => {
  vi.mocked(orchestratorApi.get).mockResolvedValue(view({ turns: [turn({ id: "other-turn" })], hasEarlier: true, beforeSequence: 2 }));
  render(<OrchestratorPanel projectId="p-1" />); await ready();
  const before = vi.mocked(orchestratorApi.get).mock.calls.length;
  await userEvent.click(screen.getByRole("button", { name: "Open retained team conversation" }));
  await screen.findByText(/This request is in earlier conversation history/);
  expect(orchestratorApi.get).toHaveBeenCalledTimes(before); expect(orchestratorApi.send).not.toHaveBeenCalled();
});

const decisionScope = () => JSON.stringify([productApi.baseUrl, identity.actor, identity.connection ? "cloud" : "local", identity.connection, identity.connection, identity.actor, identity.root, "p-1"]);
const decisions = [
  { turnId: "decision-other", question: "What is the initial launch region?", rationale: "Release planning", options: [] },
  { turnId: "decision-access", question: "Are notes private or shared?", rationale: "Implementation needs an access boundary", options: [] },
];
function openDecision(turnId = "decision-access", overrides: Record<string, unknown> = {}) {
  act(() => window.dispatchEvent(new CustomEvent(ORCHESTRATOR_OPEN, { detail: { projectId: "p-1", scope: decisionScope(), destination: "decisions", turnId, ...overrides } })));
}
const browserRecords = () => Object.fromEntries(Object.keys(localStorage).sort().map(key => [key, localStorage.getItem(key)]));

it.each(["recovered", "pending"])("defers unopened discussion presentation while preserving current understanding and the %s request record", async mode => {
  const scope = decisionScope();
  saveComposerDraft(scope, null, "Retain my original question", "original-reply");
  if (mode === "pending") retainComposerRequest(scope, { expectedRevision: 3, expectedContextDigest: context().digest, idempotencyKey: "original-request-lazy-mount", message: "Retain my original question", replyToTurnId: "original-reply", harness: "codex", model: "model-team", agentConnectionId: "gateway" });
  const before = readComposerRecovery(scope);
  vi.mocked(orchestratorApi.get).mockResolvedValue(view({ turns: [turn()] }));
  render(<OrchestratorPanel projectId="p-1" />);
  await screen.findByRole("button", { name: "Review design" });
  expect(orchestratorApi.get).toHaveBeenCalledWith("p-1", undefined, expect.any(AbortSignal));
  expect(screen.getByRole("region", { name: "Current project understanding" })).not.toBeNull();
  // A hidden subtree is still allocated. The untouched discussion must have no
  // composer, history or provider presentation, while its parent keeps context.
  expect(screen.queryByLabelText("Message Luci")).toBeNull();
  expect(document.querySelector(".orchestrator-turns")).toBeNull();
  expect(screen.queryByLabelText("Luci connection")).toBeNull();
  expect(readComposerRecovery(scope)).toEqual(before);
  await userEvent.click(screen.getByText("Discuss or request a change"));
  await screen.findByLabelText("Message Luci");
  expect(screen.getByRole("button", { name: mode === "pending" ? "Check original request" : "Restore unsent message" })).not.toBeNull();
  expect(screen.getByText("Support guest checkout")).not.toBeNull();
  expect(readComposerRecovery(scope)).toEqual(before);
  expect(orchestratorApi.send).not.toHaveBeenCalled();
  expect(orchestratorApi.resolveRequest).not.toHaveBeenCalled();
});

it("keeps active-job observation running before discussion presentation is opened", async () => {
  vi.useFakeTimers();
  try {
    vi.mocked(orchestratorApi.get).mockResolvedValue(view({ turns: [turn({ status: "running", answer: undefined })] }));
    vi.mocked(orchestratorApi.job).mockResolvedValue(job({ id: "job-1", status: "succeeded" }));
    render(<OrchestratorPanel projectId="p-1" />);
    await act(async () => {});
    expect(document.querySelector(".orchestrator-composer")).toBeNull();
    await act(async () => { await vi.advanceTimersByTimeAsync(2500); });
    expect(orchestratorApi.job).toHaveBeenCalledWith("p-1", "job-1");
    expect(document.querySelector(".orchestrator-composer")).toBeNull();
    act(() => window.dispatchEvent(new CustomEvent(ORCHESTRATOR_OPEN, { detail: { projectId: "p-1" } })));
    await act(async () => { await vi.advanceTimersByTimeAsync(20); });
    expect(screen.getByRole("button", { name: "Recover recorded response" })).not.toBeNull();
    expect(document.activeElement).toBe(screen.getByLabelText("Message Luci"));
    expect(orchestratorApi.send).not.toHaveBeenCalled();
    expect(orchestratorApi.reconcile).not.toHaveBeenCalled();
  } finally { vi.useRealTimers(); }
});

it("keeps an opened discussion node and draft across closing but resets its presentation for another project", async () => {
  const mounted = render(<OrchestratorPanel projectId="p-1" />);
  await screen.findByRole("button", { name: "Review design" });
  act(() => window.dispatchEvent(new CustomEvent(ORCHESTRATOR_OPEN, { detail: { projectId: "p-1", message: "Only project one" } })));
  const composer = screen.getByLabelText("Message Luci") as HTMLTextAreaElement;
  await waitFor(() => expect(document.activeElement).toBe(composer));
  await userEvent.click(screen.getByText("Discuss or request a change"));
  await waitFor(() => expect(screen.queryByRole("textbox", { name: "Message Luci" })).toBeNull());
  expect(composer.isConnected).toBe(true);
  await userEvent.click(screen.getByText("Discuss or request a change"));
  expect(screen.getByRole("textbox", { name: "Message Luci" })).toBe(composer);
  expect(composer.value).toBe("Only project one");
  const retained = readComposerRecovery(decisionScope());
  mounted.rerender(<OrchestratorPanel projectId="p-2" />);
  await waitFor(() => expect(orchestratorApi.get).toHaveBeenCalledWith("p-2", undefined, expect.any(AbortSignal)));
  expect(composer.isConnected).toBe(false);
  expect(screen.queryByLabelText("Message Luci")).toBeNull();
  await userEvent.click(screen.getByText("Discuss or request a change"));
  expect((await screen.findByLabelText("Message Luci") as HTMLTextAreaElement).value).toBe("");
  expect(readComposerRecovery(decisionScope())).toEqual(retained);
  expect(orchestratorApi.send).not.toHaveBeenCalled();
});

it("reveals the exact current blocking question without replacing an unsent message or its reply target", async () => {
  vi.mocked(orchestratorApi.get).mockResolvedValue(view({ blockingQuestions: decisions }));
  render(<OrchestratorPanel projectId="p-1" />); await ready();
  const other = screen.getByRole("group", { name: decisions[0].question });
  await userEvent.click(within(other).getByRole("button", { name: "Answer this decision" }));
  await userEvent.type(screen.getByLabelText("Message Luci"), "Keep my launch-region answer intact");
  const before = browserRecords(), reads = vi.mocked(orchestratorApi.get).mock.calls.length;
  openDecision();
  const target = within(screen.getByRole("group", { name: decisions[1].question })).getByRole("button", { name: "Answer this decision" });
  await waitFor(() => expect(document.activeElement).toBe(target));
  expect(orchestratorApi.get).toHaveBeenCalledTimes(reads + 1);
  expect((screen.getByLabelText("Message Luci") as HTMLTextAreaElement).value).toBe("Keep my launch-region answer intact");
  expect(screen.getByText(`Replying to: ${decisions[0].question}`)).not.toBeNull();
  expect(browserRecords()).toEqual(before);
  expect(orchestratorApi.send).not.toHaveBeenCalled(); expect(orchestratorApi.resolveRequest).not.toHaveBeenCalled();
  // Focus is one-shot; normal typing after arrival keeps its caret.
  await userEvent.click(screen.getByLabelText("Message Luci"));
  await userEvent.type(screen.getByLabelText("Message Luci"), ".");
  expect(document.activeElement).toBe(screen.getByLabelText("Message Luci"));
});

it.each(["recovered", "pending"])("reveals a decision while preserving the exact %s composer record", async mode => {
  const scope = decisionScope();
  saveComposerDraft(scope, null, "Original message must survive", "original-reply");
  if (mode === "pending") retainComposerRequest(scope, { expectedRevision: 3, expectedContextDigest: context().digest, idempotencyKey: "original-paid-request-123456", message: "Original message must survive", replyToTurnId: "original-reply", harness: "codex", model: "original-model", agentConnectionId: "original-connection" });
  vi.mocked(orchestratorApi.get).mockResolvedValue(view({ blockingQuestions: decisions }));
  render(<OrchestratorPanel projectId="p-1" />); await ready();
  const before = browserRecords();
  openDecision();
  const answer = within(screen.getByRole("group", { name: decisions[1].question })).getByRole("button", { name: "Answer this decision" });
  await waitFor(() => expect(document.activeElement).toBe(answer));
  expect(browserRecords()).toEqual(before);
  expect((screen.getByLabelText("Message Luci") as HTMLTextAreaElement).value).toBe("");
  expect(screen.getByRole("button", { name: mode === "pending" ? "Check original request" : "Restore unsent message" })).not.toBeNull();
  await userEvent.click(answer);
  expect(screen.getByText("Restore or resolve your saved message before starting another one.")).not.toBeNull();
  expect(browserRecords()).toEqual(before);
  expect(orchestratorApi.send).not.toHaveBeenCalled(); expect(orchestratorApi.resolveRequest).not.toHaveBeenCalled();
});

it("reports an already resolved or missing target instead of selecting another unresolved decision", async () => {
  vi.mocked(orchestratorApi.get).mockResolvedValueOnce(view({ blockingQuestions: decisions })).mockResolvedValue(view({ revision: 4, blockingQuestions: [decisions[0]] }));
  render(<OrchestratorPanel projectId="p-1" />); await ready();
  openDecision();
  const notice = await screen.findByText(/This decision is no longer listed as unresolved/);
  await waitFor(() => expect(document.activeElement).toBe(notice));
  expect(screen.queryByText(/Replying to:/)).toBeNull();
  expect(orchestratorApi.send).not.toHaveBeenCalled();
});

it("keeps an unreadable decision lookup actionable without touching the composer", async () => {
  render(<OrchestratorPanel projectId="p-1" />); await ready();
  await userEvent.type(screen.getByLabelText("Message Luci"), "Keep this unsent text");
  const before = browserRecords();
  vi.mocked(orchestratorApi.get).mockRejectedValueOnce(new Error("Conversation storage is unavailable"));
  openDecision("", { turnId: undefined });
  const notice = await screen.findByText(/Project decisions could not be refreshed: Conversation storage is unavailable/);
  await waitFor(() => expect(document.activeElement).toBe(notice));
  expect(browserRecords()).toEqual(before);
  expect(screen.getByRole("button", { name: "Refresh conversation" })).not.toBeNull();
  expect(orchestratorApi.send).not.toHaveBeenCalled();
});

it("ignores foreign project and workspace decision destinations", async () => {
  render(<OrchestratorPanel projectId="p-1" />); await ready();
  const reads = vi.mocked(orchestratorApi.get).mock.calls.length;
  openDecision("decision-access", { scope: "another-account-workspace" });
  openDecision("decision-access", { projectId: "p-other" });
  expect(orchestratorApi.get).toHaveBeenCalledTimes(reads);
  expect(screen.queryByText("Reading current project decisions…")).toBeNull();
});

it.each(["navigation", "new destination", "read access", "account"])("cancels a slow decision destination after %s changes", async change => {
  vi.mocked(orchestratorApi.get).mockResolvedValue(view({ blockingQuestions: decisions }));
  const mounted = render(<><button>Outside conversation</button><OrchestratorPanel projectId="p-1" /></>); await ready();
  let resolve!: (value: OrchestratorView) => void;
  vi.mocked(orchestratorApi.get).mockReturnValueOnce(new Promise(done => { resolve = done; }));
  openDecision();
  if (change === "navigation") act(() => useCockpit.setState({ lens: "deploy" }));
  if (change === "new destination") {
    openDecision("decision-other");
    await waitFor(() => expect(document.activeElement).toBe(within(screen.getByRole("group", { name: decisions[0].question })).getByRole("button", { name: "Answer this decision" })));
  }
  if (change === "read access") identity.read = false;
  if (change === "account") identity.actor = "another-person";
  if (change === "read access" || change === "account") mounted.rerender(<><button>Outside conversation</button><OrchestratorPanel projectId="p-1" /></>);
  screen.getByRole("button", { name: "Outside conversation" }).focus();
  await act(async () => resolve(view({ revision: 6, blockingQuestions: decisions })));
  expect(document.activeElement).toBe(screen.getByRole("button", { name: "Outside conversation" }));
  expect(screen.queryByText(/Replying to:/)).toBeNull();
  expect(orchestratorApi.send).not.toHaveBeenCalled();
});

it("focuses the readable question itself when the member cannot use its answer control", async () => {
  identity.assist = false;
  vi.mocked(orchestratorApi.get).mockResolvedValue(view({ blockingQuestions: decisions }));
  render(<OrchestratorPanel projectId="p-1" />);
  await screen.findByRole("button", { name: "Review design" });
  openDecision();
  const group = await screen.findByRole("group", { name: decisions[1].question });
  await waitFor(() => expect(document.activeElement).toBe(group));
  expect((within(group).getByRole("button", { name: "Answer this decision" }) as HTMLButtonElement).disabled).toBe(true);
  expect(orchestratorApi.send).not.toHaveBeenCalled();
});

it("does not replace a newer saved context at the same conversation revision with a delayed decision lookup", async () => {
  const original = view({ blockingQuestions: decisions });
  vi.mocked(orchestratorApi.get).mockResolvedValue(original);
  render(<OrchestratorPanel projectId="p-1" />); await ready();
  let resolve!: (value: OrchestratorView) => void;
  vi.mocked(orchestratorApi.get).mockReturnValueOnce(new Promise(done => { resolve = done; }));
  openDecision();
  const newer = view({ revision: original.revision, blockingQuestions: decisions, context: { ...context(), specification: { ...context().specification, purpose: "The newly saved purpose must remain visible" } } });
  vi.mocked(orchestratorApi.get).mockResolvedValue(newer);
  act(() => notifyProjectContextChanged(decisionScope(), "p-1"));
  await screen.findByText("The newly saved purpose must remain visible");
  screen.getByLabelText("Message Luci").focus();
  await act(async () => resolve(original));
  expect(screen.getByText("The newly saved purpose must remain visible")).not.toBeNull();
  expect(screen.getByText(/The project changed while its decisions were loading/)).not.toBeNull();
  expect(document.activeElement).toBe(screen.getByLabelText("Message Luci"));
  expect(orchestratorApi.send).not.toHaveBeenCalled();
});

it("does not let a refresh started before a decision lookup overwrite that lookup's newer same-revision context", async () => {
  const original = view({ blockingQuestions: decisions });
  vi.mocked(orchestratorApi.get).mockResolvedValue(original);
  render(<OrchestratorPanel projectId="p-1" />); await ready();
  let oldRefresh!: (value: OrchestratorView) => void, decision!: (value: OrchestratorView) => void;
  vi.mocked(orchestratorApi.get).mockReturnValueOnce(new Promise(done => { oldRefresh = done; })).mockReturnValueOnce(new Promise(done => { decision = done; }));
  await userEvent.click(screen.getByRole("button", { name: "Refresh conversation" }));
  openDecision();
  const newer = view({ revision: original.revision, blockingQuestions: decisions, context: { ...context(), specification: { ...context().specification, purpose: "New context from the decision lookup" } } });
  await act(async () => decision(newer));
  await screen.findByText("New context from the decision lookup");
  const answer = within(screen.getByRole("group", { name: decisions[1].question })).getByRole("button", { name: "Answer this decision" });
  await waitFor(() => expect(document.activeElement).toBe(answer));
  await act(async () => oldRefresh(original));
  expect(screen.getByText("New context from the decision lookup")).not.toBeNull();
  expect(document.activeElement).toBe(answer);
  expect(orchestratorApi.send).not.toHaveBeenCalled();
});

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

it("opens the same project conversation from compact Develop and preserves its amendment through all activities", async () => {
  const media = responsiveOverview(true);
  useCockpit.setState(state => ({ lens: "develop", selection: { ...state.selection, projectId: "p-1" } }));
  render(<OrchestratorPanel projectId="p-1" />);
  await screen.findByText("Design · Saved context");
  expect(screen.queryByLabelText("Message Luci")).toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "Talk to Luci" }));
  const composer = await screen.findByLabelText("Message Luci") as HTMLTextAreaElement;
  await waitFor(() => expect(document.activeElement).toBe(composer));
  await userEvent.type(composer, "Add keyboard shortcuts to the calculator while preserving the current build.");
  for (const lens of ["discover", "design", "develop", "deploy"] as const) {
    act(() => useCockpit.getState().setLens(lens));
    await waitFor(() => expect(screen.queryByText("Updating saved project understanding…")).toBeNull());
    expect(screen.getByLabelText("Message Luci")).toBe(composer);
    expect(composer.value).toContain("keyboard shortcuts");
  }
  media.resize(false); media.resize(true);
  expect(screen.getByLabelText("Message Luci")).toBe(composer);
  expect(within(screen.getByRole("region", { name: "Conversation context" })).getByText(/Definition 2 · Design 1 · Plan 1/)).not.toBeNull();
  expect(screen.getByText(/A running agent keeps its original captured scope/)).not.toBeNull();
  expect(readComposerRecovery(decisionScope()).draft?.message).toBe(composer.value);
  expect(orchestratorApi.send).not.toHaveBeenCalled(); expect(orchestratorApi.apply).not.toHaveBeenCalled();
});

it.each(["recovered", "pending"])("Talk to Luci focuses the %s recovery action without replacing or sending its saved message", async mode => {
  responsiveOverview(true);
  saveComposerDraft(decisionScope(), null, "Keep my unsent project amendment", "");
  if (mode === "pending") retainComposerRequest(decisionScope(), { expectedRevision: 3, expectedContextDigest: context().digest, idempotencyKey: "pending-amendment-header-entry", message: "Keep my unsent project amendment", harness: "codex" });
  const original = readComposerRecovery(decisionScope());
  render(<OrchestratorPanel projectId="p-1" />);
  await screen.findByText("Design · Saved context");
  await userEvent.click(screen.getByRole("button", { name: "Talk to Luci" }));
  const action = screen.getByRole("button", { name: mode === "pending" ? "Check original request" : "Restore unsent message" });
  await waitFor(() => expect(document.activeElement).toBe(action));
  expect(readComposerRecovery(decisionScope())).toEqual(original);
  expect(orchestratorApi.send).not.toHaveBeenCalled(); expect(orchestratorApi.resolveRequest).not.toHaveBeenCalled();
});

it.each([
  ["specification", "definition", "definition", "discover"],
  ["design-plan", "design", "design", "design"],
  ["plan", "implementation plan", "plan", "develop"],
] as const)("continues an applied %s amendment at its current editor without approving or applying again", async (target, label, editor, lens) => {
  const proposal = proposalTurn();
  proposal.answer!.proposal!.target = target;
  proposal.appliedRevision = 1;
  vi.mocked(orchestratorApi.get).mockResolvedValue(view({ turns: [proposal] }));
  useCockpit.setState(state => ({ lens: "develop", selection: { ...state.selection, projectId: "p-1" } }));
  render(<OrchestratorPanel projectId="p-1" />); await ready();
  const continuation = screen.getByRole("region", { name: "Continue amendment review" });
  expect(continuation.textContent).toContain(`saved ${label} draft revision 1`);
  expect(continuation.textContent).toContain("Existing builds retain the context they started with");
  await userEvent.click(within(continuation).getByRole("button", { name: `Open current ${label}` }));
  expect(useCockpit.getState().lens).toBe(lens);
  expect(takeProjectContextEdit(decisionScope(), "p-1", editor)).toEqual({ editor });
  expect(orchestratorApi.apply).not.toHaveBeenCalled(); expect(orchestratorApi.send).not.toHaveBeenCalled();
});

it("keeps a newer dirty task draft and cancels the applied amendment navigation when the user stays", async () => {
  const proposal = { ...proposalTurn(), appliedRevision: 3 };
  vi.mocked(orchestratorApi.get).mockResolvedValue(view({ turns: [proposal] }));
  useCockpit.setState(state => ({ lens: "develop", selection: { ...state.selection, projectId: "p-1" } }));
  useCockpit.getState().setUnsavedProjectDraft("plan", { projectId: "p-1", label: "Implementation plan", lens: "develop" });
  render(<><OrchestratorPanel projectId="p-1" /><DraftNavigationDialog /></>); await ready();
  await userEvent.click(screen.getByRole("button", { name: "Open current definition" }));
  const dialog = screen.getByRole("dialog", { name: "Keep your changes?" });
  await userEvent.click(within(dialog).getByRole("button", { name: "Keep editing" }));
  expect(useCockpit.getState().lens).toBe("develop");
  expect(takeProjectContextEdit(decisionScope(), "p-1", "definition")).toBe(false);
  expect(Object.values(useCockpit.getState().unsavedProjectDrafts)).toHaveLength(1);
  expect(orchestratorApi.apply).not.toHaveBeenCalled();
});

it("does not steal focus after a newer action while the coordinator opens", async () => {
  render(<><button>Other project work</button><OrchestratorPanel projectId="p-1" /></>);
  await screen.findByRole("button", { name: "Review design" });
  vi.useFakeTimers({ toFake: ["requestAnimationFrame", "cancelAnimationFrame"] });
  try {
    fireEvent.click(screen.getByRole("button", { name: "Talk to Luci" }));
    const other = screen.getByRole("button", { name: "Other project work" }); other.focus();
    await act(async () => { await vi.advanceTimersByTimeAsync(32); });
    expect(document.activeElement).toBe(other);
    expect(orchestratorApi.send).not.toHaveBeenCalled();
  } finally { vi.useRealTimers(); }
});

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

it("keeps an occupied composer and its recovery copy until a suggested answer is explicitly chosen", async () => {
  vi.mocked(orchestratorApi.get).mockResolvedValue(view({ turns: [turn()], blockingQuestions: [decisions[0]] }));
  const mounted = render(<OrchestratorPanel projectId="p-1" />); await ready();
  await userEvent.click(screen.getByRole("button", { name: "Answer this decision" }));
  const amendment = "Please preserve existing bookings when we add calendar exports.";
  await userEvent.type(screen.getByLabelText("Message Luci"), amendment);
  const before = browserRecords();
  const option = screen.getByRole("button", { name: "Allow guest booking" });
  await userEvent.click(option);
  expect((screen.getByLabelText("Message Luci") as HTMLTextAreaElement).value).toBe(amendment);
  expect(browserRecords()).toEqual(before);
  const dialog = screen.getByRole("dialog", { name: "Replace your unsent message?" });
  expect(within(dialog).getByText(decisions[0].question)).not.toBeNull();
  expect(within(dialog).getByText(/also switches your reply to the new question/)).not.toBeNull();
  expect(within(dialog).getByText("Can guests book without an account?")).not.toBeNull();
  expect(within(dialog).getByText("Allow guest booking")).not.toBeNull();
  await userEvent.click(within(dialog).getByRole("button", { name: "Keep current message" }));
  await waitFor(() => expect(document.activeElement).toBe(option));
  expect(browserRecords()).toEqual(before);
  mounted.unmount(); render(<OrchestratorPanel projectId="p-1" />); await ready();
  await userEvent.click(screen.getByRole("button", { name: "Restore unsent message" }));
  expect((screen.getByLabelText("Message Luci") as HTMLTextAreaElement).value).toBe(amendment);
  expect(screen.getByText(`Replying to: ${decisions[0].question}`)).not.toBeNull();
  expect(orchestratorApi.send).not.toHaveBeenCalled();
});

it("replaces an occupied message only explicitly and retains the selected question until deliberate Send", async () => {
  vi.mocked(orchestratorApi.get).mockResolvedValue(view({ turns: [turn()], blockingQuestions: [decisions[0]] }));
  render(<OrchestratorPanel projectId="p-1" />); await ready();
  await userEvent.click(screen.getByRole("button", { name: "Answer this decision" }));
  await userEvent.type(screen.getByLabelText("Message Luci"), "Add a calendar export after the first release.");
  await userEvent.click(screen.getByRole("button", { name: "Allow guest booking" }));
  expect(readComposerRecovery(decisionScope()).draft?.replyTo).toBe(decisions[0].turnId);
  await userEvent.click(screen.getByRole("button", { name: "Replace with suggested answer" }));
  expect(screen.queryByRole("dialog")).toBeNull();
  const composer = screen.getByLabelText("Message Luci");
  await waitFor(() => expect(document.activeElement).toBe(composer));
  expect((composer as HTMLTextAreaElement).value).toBe("Allow guest booking");
  expect(screen.getByText("Replying to: Can guests book without an account?")).not.toBeNull();
  expect(readComposerRecovery(decisionScope()).draft).toMatchObject({ message: "Allow guest booking", replyTo: "turn-1" });
  expect(orchestratorApi.send).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole("button", { name: "Allow guest booking" }));
  expect(screen.queryByRole("dialog")).toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "Send to Luci" }));
  expect(orchestratorApi.send).toHaveBeenCalledExactlyOnceWith("p-1", expect.objectContaining({ message: "Allow guest booking", replyToTurnId: "turn-1" }));
});

it.each(["newer focus", "message", "read", "assist", "scope"] as const)("does not complete a pending replacement focus handoff after a change to %s", async change => {
  vi.mocked(orchestratorApi.get).mockResolvedValue(view({ turns: [turn()] }));
  const ui = () => <><button>Other project work</button><OrchestratorPanel projectId="p-1" /></>;
  const mounted = render(ui()); await ready();
  await userEvent.type(screen.getByLabelText("Message Luci"), "Retain the amendment until I choose.");
  await userEvent.click(screen.getByRole("button", { name: "Allow guest booking" }));
  const composer = screen.getByLabelText("Message Luci");
  const focus = vi.spyOn(composer, "focus");
  // Radix defers close autofocus until trap teardown. Intervene in that actual
  // interval without replacing the dialog or calling its callback ourselves.
  vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout"] });
  try {
    fireEvent.click(screen.getByRole("button", { name: "Replace with suggested answer" }));
    expect(focus).not.toHaveBeenCalled();
    if (change === "newer focus") screen.getByRole("button", { name: "Other project work" }).focus();
    else if (change === "message") fireEvent.change(composer, { target: { value: "A newer amendment after closing." } });
    else {
      if (change === "scope") identity.actor = "another-member";
      else identity[change] = false;
      mounted.rerender(ui());
    }
    await act(async () => { await vi.runOnlyPendingTimersAsync(); });
    expect(focus).not.toHaveBeenCalled();
    if (change === "newer focus") expect(document.activeElement).toBe(screen.getByRole("button", { name: "Other project work" }));
    if (change === "message") expect((composer as HTMLTextAreaElement).value).toBe("A newer amendment after closing.");
    expect(orchestratorApi.send).not.toHaveBeenCalled();
  } finally { vi.useRealTimers(); }
});

it.each(["assist", "read"] as const)("retires a pending suggested replacement across %s loss and regrant without changing the original draft", async grant => {
  vi.mocked(orchestratorApi.get).mockResolvedValue(view({ turns: [turn()] }));
  const mounted = render(<OrchestratorPanel projectId="p-1" />); await ready();
  await userEvent.type(screen.getByLabelText("Message Luci"), "Keep the original amendment.");
  const before = browserRecords();
  await userEvent.click(screen.getByRole("button", { name: "Allow guest booking" }));
  expect(screen.getByRole("dialog")).not.toBeNull();
  identity[grant] = false; mounted.rerender(<OrchestratorPanel projectId="p-1" />);
  expect(screen.queryByRole("dialog")).toBeNull();
  identity[grant] = true; mounted.rerender(<OrchestratorPanel projectId="p-1" />); await ready();
  expect(screen.queryByRole("dialog")).toBeNull();
  expect((screen.getByLabelText("Message Luci") as HTMLTextAreaElement).value).toBe("Keep the original amendment.");
  expect(browserRecords()).toEqual(before);
  expect(orchestratorApi.send).not.toHaveBeenCalled();
});

it("does not carry a suggested replacement into another account or replace its retained draft", async () => {
  vi.mocked(orchestratorApi.get).mockResolvedValue(view({ turns: [turn()] }));
  const mounted = render(<OrchestratorPanel projectId="p-1" />); await ready();
  await userEvent.type(screen.getByLabelText("Message Luci"), "First account amendment.");
  const originalScope = decisionScope(), before = readComposerRecovery(originalScope);
  await userEvent.click(screen.getByRole("button", { name: "Allow guest booking" }));
  identity.actor = "second-user";
  mounted.rerender(<OrchestratorPanel projectId="p-1" />); await ready();
  expect(screen.queryByRole("dialog")).toBeNull();
  expect((screen.getByLabelText("Message Luci") as HTMLTextAreaElement).value).toBe("");
  expect(readComposerRecovery(originalScope)).toEqual(before);
  expect(orchestratorApi.send).not.toHaveBeenCalled();
});

it("preserves another tab's newer message if storage changes during the replacement choice", async () => {
  vi.mocked(orchestratorApi.get).mockResolvedValue(view({ turns: [turn()] }));
  render(<OrchestratorPanel projectId="p-1" />); await ready();
  await userEvent.type(screen.getByLabelText("Message Luci"), "Original browser draft.");
  await userEvent.click(screen.getByRole("button", { name: "Allow guest booking" }));
  const previous = readComposerRecovery(decisionScope()).draft;
  saveComposerDraft(decisionScope(), previous, "Newer amendment from another tab.", "");
  act(() => window.dispatchEvent(new StorageEvent("storage", { key: previous!.slot })));
  expect(screen.queryByRole("dialog")).toBeNull();
  expect(readComposerRecovery(decisionScope()).draft?.message).toBe("Newer amendment from another tab.");
  await userEvent.click(screen.getByRole("button", { name: "Reload saved message" }));
  await userEvent.click(screen.getByRole("button", { name: "Restore unsent message" }));
  expect((screen.getByLabelText("Message Luci") as HTMLTextAreaElement).value).toBe("Newer amendment from another tab.");
  expect(orchestratorApi.send).not.toHaveBeenCalled();
});

it.each(["Replace with suggested answer", "Keep current message", "Escape"])("keeps an amendment and returns to its editor after %s when the question was resolved during the choice", async action => {
  vi.mocked(orchestratorApi.get).mockResolvedValue(view({ turns: [turn()] }));
  render(<OrchestratorPanel projectId="p-1" />); await ready();
  await userEvent.type(screen.getByLabelText("Message Luci"), "Keep this amendment after a teammate answers.");
  const before = browserRecords(), reads = vi.mocked(orchestratorApi.get).mock.calls.length;
  await userEvent.click(screen.getByRole("button", { name: "Allow guest booking" }));
  vi.mocked(orchestratorApi.get).mockResolvedValue(view({ revision: 4, turns: [turn({ questionResolvedBy: "teammate-reply" })] }));
  act(() => notifyProjectContextChanged(decisionScope(), "p-1"));
  await waitFor(() => expect(orchestratorApi.get).toHaveBeenCalledTimes(reads + 1));
  if (action === "Escape") await userEvent.keyboard("{Escape}");
  else await userEvent.click(screen.getByRole("button", { name: action }));
  expect((screen.getByLabelText("Message Luci") as HTMLTextAreaElement).value).toBe("Keep this amendment after a teammate answers.");
  if (action === "Replace with suggested answer") expect(screen.getByText(/This question changed while this choice was open/)).not.toBeNull();
  await waitFor(() => expect(document.activeElement).toBe(screen.getByLabelText("Message Luci")));
  expect(browserRecords()).toEqual(before);
  expect(orchestratorApi.send).not.toHaveBeenCalled();
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

it.each([false, true])("keeps an occupied answer and its question through generic discussion until intentional Send, with explicit detachment %s", async clearReply => {
  vi.mocked(orchestratorApi.get).mockResolvedValue(view({ blockingQuestions: [{ turnId: "privacy-decision", question: "Private bookings?", rationale: "Privacy", options: [] }] }));
  const mounted = render(<OrchestratorPanel projectId="p-1" />); await ready();
  await userEvent.click(screen.getByRole("button", { name: "Work through the decisions" }));
  const answer = "No. Each guest may see only their own bookings.";
  await userEvent.type(screen.getByRole("textbox", { name: "Message Luci" }), answer);
  const before = browserRecords();
  expect(readComposerRecovery(decisionScope()).draft).toMatchObject({ message: answer, replyTo: "privacy-decision" });
  await userEvent.click(screen.getByRole("button", { name: "Change the plan" }));
  await userEvent.click(screen.getByRole("button", { name: "Discuss a change with Luci" }));
  const composer = screen.getByRole("textbox", { name: "Message Luci" });
  await waitFor(() => expect(document.activeElement).toBe(composer));
  expect((composer as HTMLTextAreaElement).value).toBe(answer);
  expect(browserRecords()).toEqual(before);
  expect(screen.getByText("Replying to: Private bookings?")).not.toBeNull();
  expect(orchestratorApi.send).not.toHaveBeenCalled();

  mounted.unmount(); render(<OrchestratorPanel projectId="p-1" />); await ready();
  await userEvent.click(screen.getByRole("button", { name: "Restore unsent message" }));
  expect((screen.getByRole("textbox", { name: "Message Luci" }) as HTMLTextAreaElement).value).toBe(answer);
  expect(screen.getByText("Replying to: Private bookings?")).not.toBeNull();
  if (clearReply) {
    await userEvent.click(screen.getByRole("button", { name: "Clear reply" }));
    expect(readComposerRecovery(decisionScope()).draft).toMatchObject({ message: answer, replyTo: "" });
    expect(screen.queryByText("Replying to: Private bookings?")).toBeNull();
  }
  await userEvent.click(screen.getByRole("button", { name: "Send to Luci" }));
  expect(orchestratorApi.send).toHaveBeenCalledOnce();
  expect(vi.mocked(orchestratorApi.send).mock.calls[0][1].message).toBe(answer);
  expect(vi.mocked(orchestratorApi.send).mock.calls[0][1].replyToTurnId).toBe(clearReply ? undefined : "privacy-decision");
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

function checkDraftFixture() {
  vi.spyOn(productApi, "checks").mockResolvedValue({ runs: [], suggested: [{ label: "Acceptance tests", executable: "npm", args: ["test"] }] });
  vi.spyOn(productApi, "runChecks").mockResolvedValue({} as never);
  useCockpit.setState(state => ({ selection: { ...state.selection, projectId: "p-1" }, lens: "develop" }));
}
async function authorLocalChecks() {
  await screen.findByRole("button", { name: "Remove Acceptance tests" });
  fireEvent.change(screen.getByRole("textbox", { name: "Executable" }), { target: { value: "node" } });
  fireEvent.change(screen.getByRole("textbox", { name: "Arguments · JSON array" }), { target: { value: '["private-draft-only.mjs", ' } });
}

it("lets a person ask for help beside an actual unfinished check draft without attaching it or weakening navigation protection", async () => {
  checkDraftFixture();
  render(<><OrchestratorPanel projectId="p-1" /><ProjectChecks projectId="p-1" /><DraftNavigationDialog /></>);
  await ready(); await authorLocalChecks();
  expect(Object.values(useCockpit.getState().unsavedProjectDrafts)).toEqual([expect.objectContaining({ purpose: "verification-setup", retained: true })]);
  expect(screen.getByText(/verification setup is a browser draft and is not included automatically/)).not.toBeNull();
  const input = screen.getByRole("textbox", { name: "Arguments · JSON array" }) as HTMLInputElement;
  act(() => useCockpit.getState().setLens("design"));
  const navigation = screen.getByRole("dialog", { name: "Keep your changes?" });
  expect(navigation.textContent).toContain("Stay here to continue editing");
  expect(within(navigation).getByRole("button", { name: "Keep drafts and continue" })).not.toBeNull();
  await userEvent.click(within(navigation).getByRole("button", { name: "Keep editing" }));
  const message = "Which acceptance scenarios should these tests cover?";
  await userEvent.type(screen.getByLabelText("Message Luci"), message);
  await userEvent.click(screen.getByRole("button", { name: "Send to Luci" }));
  expect(orchestratorApi.send).toHaveBeenCalledExactlyOnceWith("p-1", expect.objectContaining({ message, expectedContextDigest: context().digest, agentConnectionId: "gateway" }));
  expect(JSON.stringify(vi.mocked(orchestratorApi.send).mock.calls[0])).not.toContain("private-draft-only");
  expect(input.isConnected).toBe(true); expect(input.value).toBe('["private-draft-only.mjs", ');
  expect(Object.values(useCockpit.getState().unsavedProjectDrafts)).toHaveLength(1);
  expect(productApi.runChecks).not.toHaveBeenCalled();
});

it("keeps amendments blocked by a local check draft and shared context still blocks sending when both exist", async () => {
  checkDraftFixture(); vi.mocked(orchestratorApi.get).mockResolvedValue(view({ turns: [proposalTurn()] }));
  render(<><OrchestratorPanel projectId="p-1" /><ProjectChecks projectId="p-1" /></>);
  await ready(); await authorLocalChecks();
  await userEvent.click(screen.getByRole("button", { name: "Review amendment" }));
  const dialog = screen.getByRole("dialog", { name: "Review definition amendment" });
  expect(within(dialog).getByText(/Keep your verification setup for later by changing phase/)).not.toBeNull();
  await userEvent.click(within(dialog).getByRole("checkbox"));
  expect((within(dialog).getByRole("button", { name: "Apply reviewed amendment" }) as HTMLButtonElement).disabled).toBe(true);
  expect(orchestratorApi.apply).not.toHaveBeenCalled();
  await userEvent.click(within(dialog).getByRole("button", { name: "Keep current context" }));
  act(() => useCockpit.getState().setUnsavedProjectDraft("definition", { projectId: "p-1", label: "Definition", lens: "discover" }));
  await userEvent.type(screen.getByLabelText("Message Luci"), "Help me finish this check");
  expect(within(screen.getByLabelText("Message Luci").closest("form")!).getByText(/Definition has unsaved edits. Save or discard/)).not.toBeNull();
  expect((screen.getByRole("button", { name: "Send to Luci" }) as HTMLButtonElement).disabled).toBe(true);
  expect(orchestratorApi.send).not.toHaveBeenCalled(); expect(productApi.runChecks).not.toHaveBeenCalled();
});

it("allows only the exact original request retry beside local setup, while shared context retains its retry guard", async () => {
  const original = { expectedRevision: 3, expectedContextDigest: context().digest, idempotencyKey: "original-request-with-check-draft", message: "Original explicit question", harness: "codex", model: "model-team", agentConnectionId: "gateway" } as const;
  retainComposerRequest(decisionScope(), original);
  useCockpit.getState().setUnsavedProjectDraft("checks", { projectId: "p-1", label: "Verification setup", lens: "develop", purpose: "verification-setup", retained: true });
  render(<OrchestratorPanel projectId="p-1" />); await ready();
  await userEvent.click(screen.getByRole("button", { name: "Review original request" }));
  expect((screen.getByRole("button", { name: "Retry original request" }) as HTMLButtonElement).disabled).toBe(false);
  act(() => useCockpit.getState().setUnsavedProjectDraft("design", { projectId: "p-1", label: "Design", lens: "design" }));
  expect((screen.getByRole("button", { name: "Retry original request" }) as HTMLButtonElement).disabled).toBe(true);
  act(() => useCockpit.getState().setUnsavedProjectDraft("design", null));
  await userEvent.click(screen.getByRole("button", { name: "Retry original request" }));
  expect(orchestratorApi.send).toHaveBeenCalledExactlyOnceWith("p-1", original);
  expect(useCockpit.getState().unsavedProjectDrafts.checks).toBeDefined();
});

it("defers editor remount for a new local setup created while an amendment is saving", async () => {
  checkDraftFixture(); vi.mocked(orchestratorApi.get).mockResolvedValue(view({ turns: [proposalTurn()] }));
  let finish!: (value: OrchestratorView) => void;
  vi.mocked(orchestratorApi.apply).mockReturnValue(new Promise(resolve => { finish = resolve; }));
  render(<><OrchestratorPanel projectId="p-1" /><ProjectChecks projectId="p-1" /></>);
  await ready();
  await userEvent.click(screen.getByRole("button", { name: "Review amendment" }));
  const dialog = screen.getByRole("dialog", { name: "Review definition amendment" });
  await userEvent.click(within(dialog).getByRole("checkbox"));
  await userEvent.click(within(dialog).getByRole("button", { name: "Apply reviewed amendment" }));
  // An independently mounted editor can change while the network request is pending.
  const args = screen.getByLabelText("Arguments · JSON array") as HTMLInputElement;
  fireEvent.change(args, { target: { value: '["new-draft' } });
  await act(async () => finish(view({ revision: 4 })));
  expect(useCockpit.getState().projectSourceRevisions["p-1"]).toBeUndefined();
  expect(screen.getByText(/Your verification setup remains in the editor. Change phase/)).not.toBeNull();
  expect(args.value).toBe('["new-draft');
  await userEvent.click(screen.getByRole("button", { name: "Discard verification setup" }));
  await waitFor(() => expect(useCockpit.getState().projectSourceRevisions["p-1"]).toBe(1));
  expect(productApi.runChecks).not.toHaveBeenCalled();
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

it.each([401, 403])("clears current and earlier private reads on confirmed HTTP %i while preserving the unsent draft", async status => {
  const snapshot = context();
  snapshot.needsAttention = [{ id: "private-review-build", state: "awaiting-checks", kind: "task-review", contextStatus: "current" }];
  vi.mocked(orchestratorApi.get).mockResolvedValue(view({ context: snapshot, turns: [turn({ message: "Current private discussion" })], hasEarlier: true, beforeSequence: 5 }));
  render(<OrchestratorPanel projectId="p-1" />); await ready();
  await userEvent.type(screen.getByLabelText("Message Luci"), "My unsent amendment");
  const draft = readComposerRecovery(decisionScope()).draft;
  vi.mocked(orchestratorApi.get).mockResolvedValueOnce(view({ turns: [turn({ id: "earlier-private", sequence: 1, message: "Older private discussion" })], hasEarlier: false }));
  await userEvent.click(screen.getByRole("button", { name: "Show earlier conversation" }));
  await screen.findByText("Older private discussion");
  vi.mocked(orchestratorApi.get).mockRejectedValueOnce(new ApiError("internal", "Project read access revoked", status));
  await userEvent.click(screen.getByRole("button", { name: "Refresh project understanding" }));
  await screen.findByText("Project read access revoked");
  expect(screen.queryByText("private-review-build")).toBeNull();
  expect(screen.queryByText("Current private discussion")).toBeNull();
  expect(screen.queryByText("Older private discussion")).toBeNull();
  expect((screen.getByLabelText("Message Luci") as HTMLTextAreaElement).value).toBe("My unsent amendment");
  expect(readComposerRecovery(decisionScope()).draft).toEqual(draft);
  expect(screen.getByRole("button", { name: "Send to Luci" })).toHaveProperty("disabled", true);
  expect(orchestratorApi.send).not.toHaveBeenCalled();
});

it("labels review context as stale and disables its derived actions until a failed refresh succeeds", async () => {
  const snapshot = context();
  snapshot.needsAttention = [{ id: "recorded-review", state: "awaiting-checks", kind: "verification", contextStatus: "current" }];
  vi.mocked(orchestratorApi.get).mockResolvedValue(view({ context: snapshot }));
  render(<OrchestratorPanel projectId="p-1" />); await ready();
  let fail!: (cause: Error) => void;
  vi.mocked(orchestratorApi.get).mockReturnValueOnce(new Promise((_resolve, reject) => { fail = reject; }));
  await userEvent.click(screen.getByRole("button", { name: "Refresh project understanding" }));
  expect(screen.getByRole("button", { name: "Review design" })).toHaveProperty("disabled", true);
  expect(screen.getByRole("region", { name: "Last recorded review work" })).toBeTruthy();
  expect(screen.queryByRole("button", { name: "Inspect recorded-review: Verification to complete" })).toBeNull();
  await act(async () => fail(new Error("Read interrupted")));
  await screen.findByText("Read interrupted");
  expect(screen.getByRole("button", { name: "Review design" })).toHaveProperty("disabled", true);
  await userEvent.click(screen.getByRole("button", { name: "Refresh project understanding" }));
  await screen.findByRole("region", { name: "Recorded review work" });
  expect(screen.getByRole("button", { name: "Review design" })).toHaveProperty("disabled", false);
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

function manualQuestionView() {
  const snapshot = context();
  snapshot.specification.questions = [{ id: "membership", question: "How do members join?", answer: "" }];
  return view({ context: snapshot });
}
function QuestionWorkspace() {
  const lens = useCockpit(state => state.lens);
  return <><OrchestratorPanel projectId="p-1" />{lens === "discover" && <DefinitionWorkspace projectId="p-1" />}<DraftNavigationDialog /></>;
}
function guardDesignDraft(discard = vi.fn()) {
  useCockpit.setState(state => ({ selection: { ...state.selection, projectId: "p-1" }, lens: "design", unsavedProjectDrafts: { sketch: { projectId: "p-1", label: "Sketch", lens: "design", retained: true, discard } } }));
  return discard;
}
function planReviewView() {
  return view({ nextAction: { id: "plan", label: "Review implementation tasks", reason: "Bind a current task plan to the reviewed project definition.", lens: "develop" } });
}
it.each(["question", "plan"] as const)("collapses compact overview only after an explicit successful %s handoff", async target => {
  responsiveOverview(true);
  vi.mocked(orchestratorApi.get).mockResolvedValue(target === "question" ? manualQuestionView() : planReviewView());
  useCockpit.setState({ lens: "develop" });
  render(<OrchestratorPanel projectId="p-1" />);
  await userEvent.click(await screen.findByRole("button", { name: "Show project overview" }));
  await userEvent.click(await screen.findByRole("button", { name: target === "question" ? "Answer in definition" : "Review implementation tasks" }));
  expect(screen.getByRole("button", { name: "Show project overview" }).getAttribute("aria-expanded")).toBe("false");
  expect(useCockpit.getState().lens).toBe(target === "question" ? "discover" : "develop");
  expect(takeProjectContextEdit(currentScope(), "p-1", target === "question" ? "definition" : "plan")).toEqual(target === "question" ? { editor: "definition", questionId: "membership" } : { editor: "plan" });
  await userEvent.click(screen.getByRole("button", { name: "Show project overview" }));
  expect(screen.getByRole("button", { name: "Hide project overview" })).not.toBeNull();
  expect(orchestratorApi.send).not.toHaveBeenCalled(); expect(orchestratorApi.apply).not.toHaveBeenCalled();
});
it.each(["definition", "design"] as const)("keeps compact overview open for a generic %s handoff without a destination-focus contract", async editor => {
  responsiveOverview(true);
  useCockpit.setState({ lens: "develop" });
  render(<OrchestratorPanel projectId="p-1" />);
  await userEvent.click(await screen.findByRole("button", { name: "Show project overview" }));
  await userEvent.click(await screen.findByRole("button", { name: "Change the plan" }));
  await userEvent.click(screen.getByRole("button", { name: editor === "definition" ? "Edit outcomes" : "Edit experience" }));
  expect(takeProjectContextEdit(currentScope(), "p-1", editor)).toEqual({ editor });
  expect(screen.getByRole("button", { name: "Hide project overview" }).getAttribute("aria-expanded")).toBe("true");
});
it("keeps the desktop overview open after a successful same-stage plan review", async () => {
  responsiveOverview(false);
  vi.mocked(orchestratorApi.get).mockResolvedValue(planReviewView());
  useCockpit.setState({ lens: "develop" });
  render(<OrchestratorPanel projectId="p-1" />);
  await userEvent.click(await screen.findByRole("button", { name: "Review implementation tasks" }));
  expect(takeProjectContextEdit(currentScope(), "p-1", "plan")).toEqual({ editor: "plan" });
  expect(screen.getByRole("button", { name: "Hide project overview" }).getAttribute("aria-expanded")).toBe("true");
});
it.each(["Keep drafts and continue", "Discard edits and continue"])("keeps compact overview and drafts on guard cancellation, then collapses after %s", async choice => {
  responsiveOverview(true);
  vi.mocked(orchestratorApi.get).mockResolvedValue(planReviewView());
  const discard = guardDesignDraft();
  render(<><OrchestratorPanel projectId="p-1" /><DraftNavigationDialog /></>);
  await userEvent.click(await screen.findByRole("button", { name: "Show project overview" }));
  await userEvent.click(await screen.findByRole("button", { name: "Review implementation tasks" }));
  await userEvent.click(screen.getByRole("button", { name: "Keep editing" }));
  expect(screen.getByRole("button", { name: "Hide project overview" }).getAttribute("aria-expanded")).toBe("true");
  expect(useCockpit.getState().unsavedProjectDrafts.sketch).toBeDefined();
  expect(discard).not.toHaveBeenCalled();
  expect(takeProjectContextEdit(currentScope(), "p-1", "plan")).toBe(false);
  await userEvent.click(screen.getByRole("button", { name: "Review implementation tasks" }));
  await userEvent.click(screen.getByRole("button", { name: choice }));
  expect(screen.getByRole("button", { name: "Show project overview" }).getAttribute("aria-expanded")).toBe("false");
  expect(takeProjectContextEdit(currentScope(), "p-1", "plan")).toEqual({ editor: "plan" });
  expect(discard).toHaveBeenCalledTimes(choice === "Discard edits and continue" ? 1 : 0);
});
it.each([true, false])("uses the viewport at guarded success, with compact=%s, instead of the activation width", async compactAtSuccess => {
  const media = responsiveOverview(!compactAtSuccess);
  vi.mocked(orchestratorApi.get).mockResolvedValue(planReviewView());
  guardDesignDraft();
  render(<><OrchestratorPanel projectId="p-1" /><DraftNavigationDialog /></>);
  if (compactAtSuccess) await userEvent.click(await screen.findByRole("button", { name: "Hide project overview" }));
  // Keep the user's explicit open choice while the dialog is active.
  await userEvent.click(await screen.findByRole("button", { name: "Show project overview" }));
  await userEvent.click(await screen.findByRole("button", { name: "Review implementation tasks" }));
  media.resize(compactAtSuccess);
  await userEvent.click(screen.getByRole("button", { name: "Keep drafts and continue" }));
  expect(takeProjectContextEdit(currentScope(), "p-1", "plan")).toEqual({ editor: "plan" });
  expect(screen.getByRole("button", { name: compactAtSuccess ? "Show project overview" : "Hide project overview" }).getAttribute("aria-expanded")).toBe(compactAtSuccess ? "false" : "true");
});
it("does not collapse compact overview when permission is lost before guarded editor navigation succeeds", async () => {
  responsiveOverview(true);
  vi.mocked(orchestratorApi.get).mockResolvedValue(manualQuestionView());
  guardDesignDraft();
  const mounted = render(<><OrchestratorPanel projectId="p-1" /><DraftNavigationDialog /></>);
  await userEvent.click(await screen.findByRole("button", { name: "Show project overview" }));
  await userEvent.click(await screen.findByRole("button", { name: "Answer in definition" }));
  identity.edit = false;
  mounted.rerender(<><OrchestratorPanel projectId="p-1" /><DraftNavigationDialog /></>);
  await userEvent.click(screen.getByRole("button", { name: "Keep drafts and continue" }));
  expect(takeProjectContextEdit(currentScope(), "p-1", "definition")).toBe(false);
  expect(screen.getByRole("button", { name: "Hide project overview" }).getAttribute("aria-expanded")).toBe("true");
});
it("preserves an explicitly open compact overview through programmatic navigation and asynchronous context refresh", async () => {
  responsiveOverview(true);
  vi.mocked(orchestratorApi.get).mockResolvedValue(planReviewView());
  useCockpit.setState({ lens: "develop" });
  render(<OrchestratorPanel projectId="p-1" />);
  await userEvent.click(await screen.findByRole("button", { name: "Show project overview" }));
  await screen.findByRole("button", { name: "Review implementation tasks" });
  const before = vi.mocked(orchestratorApi.get).mock.calls.length;
  act(() => useCockpit.getState().setLens("design"));
  await waitFor(() => expect(orchestratorApi.get).toHaveBeenCalledTimes(before + 1));
  act(() => notifyProjectContextChanged(currentScope(), "p-1"));
  await waitFor(() => expect(orchestratorApi.get).toHaveBeenCalledTimes(before + 2));
  expect(screen.getByRole("button", { name: "Hide project overview" }).getAttribute("aria-expanded")).toBe("true");
  expect(takeProjectContextEdit(currentScope(), "p-1", "plan")).toBe(false);
});
it("publishes repeatable exact plan review requests in Develop for a read-only reader without sending or changing context", async () => {
  identity.edit = false; identity.assist = false;
  vi.mocked(orchestratorApi.get).mockResolvedValue(planReviewView());
  useCockpit.setState({ lens: "develop" });
  render(<OrchestratorPanel projectId="p-1" />);
  const review = await screen.findByRole("button", { name: "Review implementation tasks" });
  await userEvent.click(review);
  expect(useCockpit.getState().lens).toBe("develop");
  expect(takeProjectContextEdit(currentScope(), "p-1", "plan")).toEqual({ editor: "plan" });
  await userEvent.click(review);
  expect(takeProjectContextEdit(currentScope(), "p-1", "plan")).toEqual({ editor: "plan" });
  expect(orchestratorApi.send).not.toHaveBeenCalled(); expect(orchestratorApi.apply).not.toHaveBeenCalled();
});
it("keeps a guarded plan review unpublished when navigation is cancelled, then delivers it after explicit continuation", async () => {
  vi.mocked(orchestratorApi.get).mockResolvedValue(planReviewView());
  const discard = guardDesignDraft();
  render(<><OrchestratorPanel projectId="p-1" /><DraftNavigationDialog /></>);
  await userEvent.click(await screen.findByRole("button", { name: "Review implementation tasks" }));
  expect(takeProjectContextEdit(currentScope(), "p-1", "plan")).toBe(false);
  await userEvent.click(screen.getByRole("button", { name: "Keep editing" }));
  expect(useCockpit.getState().lens).toBe("design");
  expect(useCockpit.getState().unsavedProjectDrafts.sketch).toBeDefined();
  expect(takeProjectContextEdit(currentScope(), "p-1", "plan")).toBe(false);
  await userEvent.click(screen.getByRole("button", { name: "Review implementation tasks" }));
  await userEvent.click(screen.getByRole("button", { name: "Keep drafts and continue" }));
  expect(useCockpit.getState().lens).toBe("develop");
  expect(takeProjectContextEdit(currentScope(), "p-1", "plan")).toEqual({ editor: "plan" });
  expect(discard).not.toHaveBeenCalled();
  expect(orchestratorApi.send).not.toHaveBeenCalled(); expect(orchestratorApi.apply).not.toHaveBeenCalled();
});
it("keeps pending plan review after edit access is removed but cancels it when read access is removed", async () => {
  vi.mocked(orchestratorApi.get).mockResolvedValue(planReviewView());
  useCockpit.setState({ lens: "develop" });
  const mounted = render(<OrchestratorPanel projectId="p-1" />);
  await userEvent.click(await screen.findByRole("button", { name: "Review implementation tasks" }));
  identity.edit = false; mounted.rerender(<OrchestratorPanel projectId="p-1" />);
  expect(takeProjectContextEdit(currentScope(), "p-1", "plan")).toEqual({ editor: "plan" });
  await userEvent.click(screen.getByRole("button", { name: "Review implementation tasks" }));
  identity.read = false; mounted.rerender(<OrchestratorPanel projectId="p-1" />);
  expect(takeProjectContextEdit(currentScope(), "p-1", "plan")).toBe(false);
  identity.read = true; mounted.rerender(<OrchestratorPanel projectId="p-1" />);
  await screen.findByRole("button", { name: "Review implementation tasks" });
  expect(takeProjectContextEdit(currentScope(), "p-1", "plan")).toBe(false);
});
it.each(["navigation", "account", "project"])("abandons an unconsumed lazy plan request after %s changes", async change => {
  vi.mocked(orchestratorApi.get).mockImplementation(async id => ({ ...planReviewView(), projectId: id, context: context(id) }));
  useCockpit.setState({ lens: "develop" });
  const mounted = render(<OrchestratorPanel projectId="p-1" />);
  await userEvent.click(await screen.findByRole("button", { name: "Review implementation tasks" }));
  const oldScope = currentScope();
  if (change === "navigation") act(() => useCockpit.getState().setLens("design"));
  else if (change === "account") { identity.actor = "other"; mounted.rerender(<OrchestratorPanel projectId="p-1" />); }
  else mounted.rerender(<OrchestratorPanel projectId="p-2" />);
  expect(takeProjectContextEdit(oldScope, "p-1", "plan")).toBe(false);
  expect(orchestratorApi.send).not.toHaveBeenCalled();
});
it("cancels question navigation with the existing draft guard and leaves no answer target for a later visit", async () => {
  vi.mocked(orchestratorApi.get).mockResolvedValue(manualQuestionView());
  const getDefinition = vi.spyOn(definitionApi, "get").mockResolvedValue(manualQuestionView().context.specification);
  const discard = guardDesignDraft();
  render(<QuestionWorkspace />);
  await userEvent.click(await screen.findByRole("button", { name: "Answer in definition" }));
  await userEvent.click(screen.getByRole("button", { name: "Keep editing" }));
  expect(useCockpit.getState().lens).toBe("design");
  expect(useCockpit.getState().unsavedProjectDrafts.sketch).toBeDefined();
  expect(takeProjectContextEdit(currentScope(), "p-1", "definition")).toBe(false);
  expect(discard).not.toHaveBeenCalled(); expect(getDefinition).not.toHaveBeenCalled();
  expect(orchestratorApi.send).not.toHaveBeenCalled(); expect(orchestratorApi.apply).not.toHaveBeenCalled();
});
it.each(["Keep drafts and continue", "Discard edits and continue"])("focuses the requested answer after %s and after the real draft dialog restores focus", async choice => {
  vi.mocked(orchestratorApi.get).mockResolvedValue(manualQuestionView());
  vi.spyOn(definitionApi, "get").mockResolvedValue(manualQuestionView().context.specification);
  const save = vi.spyOn(definitionApi, "save"), discard = guardDesignDraft();
  render(<QuestionWorkspace />);
  await userEvent.click(await screen.findByRole("button", { name: "Answer in definition" }));
  await userEvent.click(screen.getByRole("button", { name: choice }));
  const answer = await screen.findByRole("textbox", { name: "Answer" });
  await waitFor(() => expect(document.activeElement).toBe(answer));
  expect(screen.queryByRole("dialog", { name: "Keep your changes?" })).toBeNull();
  await userEvent.type(answer, "Coordinators invite members.");
  expect(document.activeElement).toBe(answer);
  expect((answer as HTMLTextAreaElement).value).toBe("Coordinators invite members.");
  expect(discard).toHaveBeenCalledTimes(choice === "Discard edits and continue" ? 1 : 0);
  expect(save).not.toHaveBeenCalled(); expect(orchestratorApi.send).not.toHaveBeenCalled(); expect(orchestratorApi.apply).not.toHaveBeenCalled();
});
it("collapses the compact overview and focuses the requested answer after the real draft dialog closes", async () => {
  responsiveOverview(true);
  vi.mocked(orchestratorApi.get).mockResolvedValue(manualQuestionView());
  vi.spyOn(definitionApi, "get").mockResolvedValue(manualQuestionView().context.specification);
  const save = vi.spyOn(definitionApi, "save"), discard = guardDesignDraft();
  render(<QuestionWorkspace />);
  await userEvent.click(await screen.findByRole("button", { name: "Show project overview" }));
  await userEvent.click(await screen.findByRole("button", { name: "Answer in definition" }));
  await userEvent.click(screen.getByRole("button", { name: "Keep drafts and continue" }));
  expect(screen.getByRole("button", { name: "Show project overview" }).getAttribute("aria-expanded")).toBe("false");
  const answer = await screen.findByRole("textbox", { name: "Answer" });
  await waitFor(() => expect(document.activeElement).toBe(answer));
  expect(screen.queryByRole("dialog", { name: "Keep your changes?" })).toBeNull();
  await userEvent.type(answer, "Coordinators invite members.");
  expect(document.activeElement).toBe(answer);
  expect((answer as HTMLTextAreaElement).value).toBe("Coordinators invite members.");
  expect(discard).not.toHaveBeenCalled(); expect(save).not.toHaveBeenCalled();
  expect(orchestratorApi.send).not.toHaveBeenCalled(); expect(orchestratorApi.apply).not.toHaveBeenCalled();
});
it("abandons an unconsumed question target when navigation changes before its lazy editor loads", async () => {
  vi.mocked(orchestratorApi.get).mockResolvedValue(manualQuestionView());
  let finish!: (value: ReturnType<typeof context>["specification"]) => void;
  vi.spyOn(definitionApi, "get").mockReturnValue(new Promise(resolve => { finish = resolve; }));
  useCockpit.setState({ lens: "design" });
  render(<QuestionWorkspace />);
  await userEvent.click(await screen.findByRole("button", { name: "Answer in definition" }));
  act(() => useCockpit.getState().setLens("develop"));
  expect(takeProjectContextEdit(currentScope(), "p-1", "definition")).toBe(false);
  await act(async () => finish(manualQuestionView().context.specification));
  expect(useCockpit.getState().lens).toBe("develop");
  expect(orchestratorApi.send).not.toHaveBeenCalled();
});
it("cancels a question target when editing access is lost while the definition is loading", async () => {
  vi.mocked(orchestratorApi.get).mockResolvedValue(manualQuestionView());
  let finish!: (value: ReturnType<typeof context>["specification"]) => void;
  vi.spyOn(definitionApi, "get").mockReturnValue(new Promise(resolve => { finish = resolve; }));
  useCockpit.setState({ lens: "design" });
  const mounted = render(<QuestionWorkspace />);
  await userEvent.click(await screen.findByRole("button", { name: "Answer in definition" }));
  identity.edit = false; mounted.rerender(<QuestionWorkspace />);
  await act(async () => finish(manualQuestionView().context.specification));
  expect(takeProjectContextEdit(currentScope(), "p-1", "definition")).toBe(false);
  expect(document.activeElement?.tagName).not.toBe("TEXTAREA");
  identity.edit = true; mounted.rerender(<QuestionWorkspace />);
  expect(document.activeElement?.tagName).not.toBe("TEXTAREA");
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


it("aborts only the abandoned project's display refresh and fences its late private result", async () => {
  const reads: { id: string; signal: AbortSignal; finish(value: OrchestratorView): void }[] = [];
  vi.mocked(orchestratorApi.get).mockImplementation((id, before, signal) => new Promise(finish => {
    expect(before).toBeUndefined(); expect(signal).toBeInstanceOf(AbortSignal); reads.push({ id, signal: signal!, finish });
  }));
  const mounted = render(<OrchestratorPanel projectId="p-1" />);
  mounted.rerender(<OrchestratorPanel projectId="p-2" />);
  expect(reads).toHaveLength(2); expect(reads[0].signal.aborted).toBe(true); expect(reads[1].signal.aborted).toBe(false);
  await act(async () => reads[0].finish(view({ turns: [turn()] })));
  expect(screen.queryByText("Support guest checkout")).toBeNull();
  const current = context("p-2"); current.specification.purpose = "Project B current purpose";
  await act(async () => reads[1].finish(view({ projectId: "p-2", context: current })));
  expect(screen.getByText("Project B current purpose")).not.toBeNull();
  mounted.rerender(<OrchestratorPanel projectId="p-1" />);
  expect(reads).toHaveLength(3); expect(reads[2].signal.aborted).toBe(false);
  mounted.unmount(); expect(reads[2].signal.aborted).toBe(true);
  expect(orchestratorApi.cancel).not.toHaveBeenCalled(); expect(orchestratorApi.send).not.toHaveBeenCalled();
});

it("aborts a superseded display refresh without letting its finally clear the newer request", async () => {
  const reads: { signal: AbortSignal; fail(cause: unknown): void }[] = [];
  vi.mocked(orchestratorApi.get).mockImplementation((_id, _before, signal) => new Promise((_resolve, fail) => reads.push({ signal: signal!, fail })));
  const mounted = render(<OrchestratorPanel projectId="p-1" />);
  act(() => notifyProjectContextChanged(decisionScope(), "p-1"));
  expect(reads).toHaveLength(2); expect(reads[0].signal.aborted).toBe(true); expect(reads[1].signal.aborted).toBe(false);
  await act(async () => reads[0].fail(new Error("Late canceled read failure")));
  expect(screen.queryByText("Late canceled read failure")).toBeNull();
  mounted.unmount(); expect(reads[1].signal.aborted).toBe(true);
  await act(async () => reads[1].fail(reads[1].signal.reason));
});

it("uses a fresh display controller after StrictMode cleanup without accepting canceled context", async () => {
  const reads: { signal: AbortSignal; finish(value: OrchestratorView): void }[] = [];
  vi.mocked(orchestratorApi.get).mockImplementation((_id, _before, signal) => new Promise(finish => reads.push({ signal: signal!, finish })));
  const mounted = render(<StrictMode><OrchestratorPanel projectId="p-1" /></StrictMode>);
  expect(reads).toHaveLength(2); expect(reads[0].signal.aborted).toBe(true); expect(reads[1].signal.aborted).toBe(false);
  await act(async () => reads[0].finish(view({ turns: [turn()] })));
  expect(screen.queryByText("Support guest checkout")).toBeNull();
  await act(async () => reads[1].finish(view()));
  await screen.findByRole("button", { name: "Review design" }); mounted.unmount();
});

it("preserves unsent recovery and does not abort display reads for Edit-only churn but does for Read loss", async () => {
  const reads: { signal: AbortSignal; finish(value: OrchestratorView): void }[] = [];
  saveComposerDraft(decisionScope(), null, "Keep this draft", ""); const before = readComposerRecovery(decisionScope());
  vi.mocked(orchestratorApi.get).mockImplementation((_id, _sequence, signal) => new Promise(finish => reads.push({ signal: signal!, finish })));
  const mounted = render(<OrchestratorPanel projectId="p-1" />);
  identity.edit = false; mounted.rerender(<OrchestratorPanel projectId="p-1" />);
  expect(reads).toHaveLength(1); expect(reads[0].signal.aborted).toBe(false);
  identity.read = false; mounted.rerender(<OrchestratorPanel projectId="p-1" />);
  expect(reads[0].signal.aborted).toBe(true); expect(readComposerRecovery(decisionScope())).toEqual(before);
  identity.read = true; mounted.rerender(<OrchestratorPanel projectId="p-1" />);
  expect(reads).toHaveLength(2); expect(reads[1].signal.aborted).toBe(false);
  await act(async () => reads[0].finish(view({ turns: [turn()] })));
  expect(screen.queryByText("Support guest checkout")).toBeNull();
  await act(async () => reads[1].finish(view()));
  expect(readComposerRecovery(decisionScope())).toEqual(before);
});

it("display cleanup does not cancel or replay an acknowledged-late send or clear its durable identity early", async () => {
  let finish!: (value: OrchestratorJob) => void;
  vi.mocked(orchestratorApi.send).mockReturnValue(new Promise(resolve => { finish = resolve; }));
  const mounted = render(<OrchestratorPanel projectId="p-1" />); await ready();
  await userEvent.type(screen.getByLabelText("Message Luci"), "Retain the exact pending request");
  await userEvent.click(screen.getByRole("button", { name: "Send to Luci" }));
  const pending = readComposerRecovery(decisionScope()); expect(pending.pending).toHaveLength(1);
  expect(vi.mocked(orchestratorApi.send).mock.calls[0]).toHaveLength(2);
  mounted.unmount(); expect(readComposerRecovery(decisionScope())).toEqual(pending);
  await act(async () => finish(job()));
  expect(readComposerRecovery(decisionScope()).pending).toHaveLength(0);
  expect(orchestratorApi.send).toHaveBeenCalledOnce(); expect(orchestratorApi.cancel).not.toHaveBeenCalled();
});

it("keeps post-send readback outside display cancellation ownership", async () => {
  const mounted = render(<OrchestratorPanel projectId="p-1" />); await ready();
  let finish!: (value: OrchestratorView) => void;
  vi.mocked(orchestratorApi.get).mockReturnValueOnce(new Promise(resolve => { finish = resolve; }));
  await userEvent.type(screen.getByLabelText("Message Luci"), "Accepted request with a delayed readback");
  await userEvent.click(screen.getByRole("button", { name: "Send to Luci" }));
  const calls = vi.mocked(orchestratorApi.get).mock.calls;
  expect(calls[calls.length - 1]).toEqual(["p-1"]);
  expect(readComposerRecovery(decisionScope()).pending).toHaveLength(0);
  mounted.unmount(); await act(async () => finish(view()));
  expect(orchestratorApi.send).toHaveBeenCalledOnce(); expect(orchestratorApi.cancel).not.toHaveBeenCalled();
});


it("opens the existing conversation as the primary pane without sending or granting permission", async () => {
  useCockpit.setState({ projects: [aProject({ id: "p-1", name: "Atlas" })] });
  render(<OrchestratorPanel projectId="p-1" primary />); await ready();
  const pane = screen.getByRole("region", { name: "Luci conversation" });
  expect(pane.classList.contains("orchestrator-primary")).toBe(true);
  expect(screen.getByText(/Working in Atlas/)).not.toBeNull();
  expect(screen.getByLabelText("Message Luci")).not.toBeNull();
  expect(orchestratorApi.send).not.toHaveBeenCalled(); expect(orchestratorApi.apply).not.toHaveBeenCalled();
  expect(screen.getByRole("region", { name: "Saved project understanding" })).not.toBeNull();
  screen.getByRole("button", { name: "Talk to Luci" }).focus(); await userEvent.keyboard("{Enter}");
  await waitFor(() => expect(document.activeElement).toBe(screen.getByLabelText("Message Luci")));
});
it("preserves the current unsent message through primary and phase-tool presentations", async () => {
  const ui = render(<OrchestratorPanel projectId="p-1" primary />); await ready();
  await userEvent.type(screen.getByLabelText("Message Luci"), "Add booking reminders");
  ui.rerender(<OrchestratorPanel projectId="p-1" />); expect(screen.getByLabelText("Message Luci")).toHaveProperty("value", "Add booking reminders");
  ui.rerender(<OrchestratorPanel projectId="p-1" primary />);
  expect(screen.getByLabelText("Message Luci")).toHaveProperty("value", "Add booking reminders"); expect(orchestratorApi.send).not.toHaveBeenCalled();
});
it("keeps primary conversations isolated and recovers the original project draft explicitly", async () => {
  const ui = render(<OrchestratorPanel projectId="p-1" primary />); await ready(); await userEvent.type(screen.getByLabelText("Message Luci"), "Atlas only");
  ui.rerender(<OrchestratorPanel projectId="p-2" primary />); await ready(); expect(screen.getByLabelText("Message Luci")).toHaveProperty("value", "");
  expect(screen.queryByText("Atlas only")).toBeNull(); ui.rerender(<OrchestratorPanel projectId="p-1" primary />);
  await userEvent.click(await screen.findByRole("button", { name: "Restore unsent message" }));
  expect(screen.getByLabelText("Message Luci")).toHaveProperty("value", "Atlas only"); expect(orchestratorApi.send).not.toHaveBeenCalled();
});
it("shows the main conversation on compact screens and hides its private contents when Read is revoked", async () => {
  const media = vi.spyOn(window, "matchMedia").mockImplementation(query => ({ matches: true, media: query, onchange: null, addEventListener: vi.fn(), removeEventListener: vi.fn(), addListener: vi.fn(), removeListener: vi.fn(), dispatchEvent: () => false }));
  try { const ui = render(<OrchestratorPanel projectId="p-1" primary />); await ready();
    await userEvent.type(screen.getByLabelText("Message Luci"), "Private amendment");
    expect(screen.getByRole("region", { name: "Luci conversation" }).classList.contains("orchestrator-open")).toBe(true);
    identity.read = false; identity.assist = false; ui.rerender(<OrchestratorPanel projectId="p-1" primary />);
    expect(screen.queryByLabelText("Message Luci")).toBeNull(); expect(screen.queryByText("Private amendment")).toBeNull(); expect(orchestratorApi.send).not.toHaveBeenCalled();
  } finally { media.mockRestore(); }
});
