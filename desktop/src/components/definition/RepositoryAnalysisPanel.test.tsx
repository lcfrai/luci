import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { StrictMode } from "react";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { RepositoryAnalysisPanel } from "./RepositoryAnalysisPanel";
import { definitionApi, type DefinitionAssistance, type ProjectSpecification } from "../../api/definition";
import { productApi } from "../../api/product";
import { resetCockpit, useCockpit } from "../../store/store";
import { ApiError } from "../../api/client";
import { readRepositoryAnalysisRecovery, retainRepositoryAnalysisRequest, saveRepositoryAnalysisDraft, type PendingRepositoryAnalysisRequest } from "./repositoryAnalysisRecovery";
import { AGENT_CONNECTIONS_CHANGED } from "../../hooks/useAgentConnections";

vi.mock("../../api/definition", () => ({ definitionApi: { listAssistance: vi.fn(), assistance: vi.fn(), analyseRepository: vi.fn(), repositoryRequest: vi.fn(), applyAnalysis: vi.fn(), cancelAssistance: vi.fn() } }));
vi.mock("../../api/product", async original => ({ ...await original<typeof import("../../api/product")>(), productApi: { agents: vi.fn(), agentConnections: vi.fn() } }));
const identity = vi.hoisted(() => ({ actor: "actor-owner", scope: "workspace-one", assist: true, read: true }));
vi.mock("../auth/AuthProvider", () => ({ useAuth: () => ({ profile: { user: { id: identity.actor } } }) }));
vi.mock("../../hooks/useProjectDraft", () => ({ useProjectDraftScope: (id: string) => `${identity.scope}:${identity.actor}:${id}` }));
vi.mock("../../hooks/useProjectAccess", () => ({ useProjectAccess: () => ({ canRead: identity.read, canEdit: identity.assist, canExecute: identity.assist, canAssist: identity.assist, error: null, loading: false }) }));
const originalLocks = Object.getOwnPropertyDescriptor(navigator, "locks");
const scope = () => `${identity.scope}:${identity.actor}:owned`;
const specification = (): ProjectSpecification => ({ projectId: "owned", version: 1, revision: 2, briefRevision: 3, startingPoint: "existing", purpose: "Understand this app", audience: "Its users", constraints: "Keep data", outOfScope: [], requirements: [], journeys: [], roles: [], entities: [], integrations: [], glossary: [], decisions: [], questions: [], status: "draft", digest: "", updatedAt: "" });
const result = (): DefinitionAssistance => ({
  id: "analysis-one", projectId: "owned", requestedBy: "actor-owner", ownRequest: true, kind: "repository", status: "succeeded", harness: "codex", model: "configured-model", authMode: "chatgpt", message: "Analysis saved", createdAt: "2026-10-05T00:00:00Z", updatedAt: "2026-10-05T00:00:05Z",
  source: { mode: "worktree", digest: "source-fingerprint", fileCount: 1, includedBytes: 100, skippedFiles: 7, partial: true, limitation: "Partial source selection", files: [{ path: "src/app.ts", digest: "file-fingerprint", bytes: 100 }] },
  result: { question: "", options: [], rationale: "The route is present; persistence needs clarification.", findings: [{ kind: "observed", summary: "The app renders a start route.", paths: ["src/app.ts"] }, { kind: "gap", summary: "Persistence was not observed in the selected files.", paths: [] }], proposal: { ...specification(), purpose: "A proposed understanding" } },
});
beforeEach(() => {
  vi.resetAllMocks(); resetCockpit();
  localStorage.clear(); Object.assign(identity, { actor: "actor-owner", scope: "workspace-one", assist: true, read: true });
  let tail = Promise.resolve();
  Object.defineProperty(navigator, "locks", { configurable: true, value: { request: (_name: string, _options: unknown, action: () => unknown) => {
    const next = tail.then(action); tail = next.then(() => undefined, () => undefined); return next;
  } } });
  vi.mocked(definitionApi.listAssistance).mockResolvedValue({ jobs: [] });
  vi.mocked(productApi.agents).mockResolvedValue({ providers: [] });
  vi.mocked(productApi.agentConnections).mockResolvedValue({ revision: 0, activeConnectionId: "", connections: [] });
  useCockpit.getState().setDefaults({ harness: "codex", model: "configured-model" });
  vi.mocked(definitionApi.analyseRepository).mockImplementation(async (_project, input) => ({ ...result(), status: "queued", result: undefined, idempotencyKey: input.idempotencyKey, replyToAnalysisId: input.replyToAnalysisId, requestedBy: identity.actor }));
});
afterEach(() => {
  if (originalLocks) Object.defineProperty(navigator, "locks", originalLocks);
  else Reflect.deleteProperty(navigator, "locks");
});
const mount = (dirty = false, onApplied = vi.fn()) => render(<RepositoryAnalysisPanel projectId="owned" specification={specification()} dirty={dirty} onApplied={onApplied} />);
const question = (): DefinitionAssistance => ({ ...result(), result: { ...result().result!, question: "Should existing customer accounts be kept?", options: ["Keep existing accounts", "Start with a fresh account store"], proposal: null } });
const originalRequest = (): PendingRepositoryAnalysisRequest => ({ expectedRevision: 2, expectedBriefRevision: 3, sourceMode: "worktree", harness: "codex", model: "original-model", agentConnectionId: "", acknowledgeSourceRead: true, replyToAnalysisId: "analysis-one", message: "Keep existing accounts", idempotencyKey: "analysis-original-request-key" });
function savedQuestion() {
  vi.mocked(definitionApi.listAssistance).mockResolvedValue({ jobs: [question()] });
  vi.mocked(definitionApi.assistance).mockResolvedValue(question());
}

it("recovers saved findings with read-only requests and does not start inference or apply them", async () => {
  vi.mocked(definitionApi.listAssistance).mockResolvedValue({ jobs: [{ ...result(), result: undefined }] });
  vi.mocked(definitionApi.assistance).mockResolvedValue(result());
  mount();
  await screen.findByText("The app renders a start route.");
  expect(definitionApi.assistance).toHaveBeenCalledExactlyOnceWith("owned", "analysis-one");
  expect(definitionApi.analyseRepository).not.toHaveBeenCalled();
  expect(definitionApi.applyAnalysis).not.toHaveBeenCalled();
  expect(screen.getByText("Observed in source")).not.toBeNull();
  expect(screen.getByText("Partial source selection")).not.toBeNull();
  expect((screen.getByRole("button", { name: "Apply as definition draft" }) as HTMLButtonElement).disabled).toBe(true);
});

it("requires a real model and source acknowledgement before an explicit subscription analysis", async () => {
  useCockpit.getState().setDefaults({ harness: "codex", model: "default" });
  vi.mocked(definitionApi.analyseRepository).mockResolvedValue({ ...result(), status: "queued", result: undefined });
  mount(); await screen.findByRole("button", { name: "Analyse repository" });
  const model = screen.getByLabelText("Analysis model");
  expect((model as HTMLInputElement).value).toBe("");
  fireEvent.click(screen.getByLabelText("Send the selected source and definition to this provider for analysis."));
  expect((screen.getByRole("button", { name: "Analyse repository" }) as HTMLButtonElement).disabled).toBe(true);
  fireEvent.change(model, { target: { value: " chosen-account-model " } });
  expect((screen.getByLabelText("Send the selected source and definition to this provider for analysis.") as HTMLInputElement).checked).toBe(false);
  fireEvent.click(screen.getByLabelText("Send the selected source and definition to this provider for analysis."));
  fireEvent.click(screen.getByRole("button", { name: "Analyse repository" }));
  await waitFor(() => expect(definitionApi.analyseRepository).toHaveBeenCalledExactlyOnceWith("owned", { expectedRevision: 2, expectedBriefRevision: 3, sourceMode: "worktree", harness: "codex", model: "chosen-account-model", agentConnectionId: "", acknowledgeSourceRead: true, idempotencyKey: expect.stringMatching(/^analysis-[a-f0-9-]+$/) }));
  expect(definitionApi.applyAnalysis).not.toHaveBeenCalled();
});

it("uses the exact saved local connection for OpenCode and cannot edit its model in this form", async () => {
  vi.mocked(productApi.agents).mockResolvedValue({ providers: [{ id: "local", name: "Local model", configured: true, model: "installed-model" }] });
  vi.mocked(definitionApi.analyseRepository).mockResolvedValue({ ...result(), status: "queued", result: undefined });
  mount();
  await waitFor(() => expect(productApi.agents).toHaveBeenCalledOnce());
  fireEvent.change(screen.getByLabelText("Analysis provider"), { target: { value: "opencode" } });
  await waitFor(() => expect((screen.getByLabelText("Analysis model") as HTMLInputElement).value).toBe("luci-local/installed-model"));
  expect((screen.getByLabelText("Analysis model") as HTMLInputElement).readOnly).toBe(true);
  fireEvent.click(screen.getByLabelText("Send the selected source and definition to this provider for analysis."));
  fireEvent.click(screen.getByRole("button", { name: "Analyse repository" }));
  await waitFor(() => expect(definitionApi.analyseRepository).toHaveBeenCalledWith("owned", expect.objectContaining({ harness: "opencode", model: "luci-local/installed-model" })));
});

it("uses the saved API connection for an explicitly selected Claude model", async () => {
  vi.mocked(definitionApi.analyseRepository).mockResolvedValue({ ...result(), status: "queued", result: undefined });
  mount(); await screen.findByRole("button", { name: "Analyse repository" });
  fireEvent.change(screen.getByLabelText("Analysis provider"), { target: { value: "claude-code" } });
  fireEvent.change(screen.getByLabelText("Analysis model"), { target: { value: "sonnet" } });
  fireEvent.click(screen.getByLabelText("Send the selected source and definition to this provider for analysis."));
  fireEvent.click(screen.getByRole("button", { name: "Analyse repository" }));
  await waitFor(() => expect(definitionApi.analyseRepository).toHaveBeenCalledWith("owned", expect.objectContaining({ harness: "claude-code", model: "sonnet" })));
});

it("binds an explicitly selected gateway model and refuses a missing protocol without fallback", async () => {
  vi.mocked(productApi.agentConnections).mockResolvedValue({ revision: 3, activeConnectionId: "", connections: [{ id: "chosen-gateway", revision: 2, name: "Company gateway", kind: "litellm", enabled: true, configured: true, keyPresent: true, endpoints: { responses: "https://gateway.example/v1" }, models: { codex: "gateway-coder" } }] });
  vi.mocked(definitionApi.analyseRepository).mockResolvedValue({ ...result(), status: "queued", result: undefined });
  mount();
  await screen.findByRole("option", { name: "Company gateway" });
  fireEvent.change(screen.getByLabelText("Analysis connection"), { target: { value: "chosen-gateway" } });
  expect((screen.getByLabelText("Analysis model") as HTMLInputElement).value).toBe("gateway-coder");
  fireEvent.change(screen.getByLabelText("Analysis provider"), { target: { value: "claude-code" } });
  fireEvent.click(screen.getByLabelText("Send the selected source and definition to this provider for analysis."));
  expect((screen.getByRole("button", { name: "Analyse repository" }) as HTMLButtonElement).disabled).toBe(true);
  expect(definitionApi.analyseRepository).not.toHaveBeenCalled();
  fireEvent.change(screen.getByLabelText("Analysis provider"), { target: { value: "codex" } });
  fireEvent.click(screen.getByLabelText("Send the selected source and definition to this provider for analysis."));
  fireEvent.click(screen.getByRole("button", { name: "Analyse repository" }));
  await waitFor(() => expect(definitionApi.analyseRepository).toHaveBeenCalledWith("owned", expect.objectContaining({ harness: "codex", model: "gateway-coder", agentConnectionId: "chosen-gateway", expectedAgentConnectionRevision: 2 })));
  expect(vi.mocked(definitionApi.analyseRepository).mock.calls[0][1]).not.toHaveProperty("authMode");
});

it("applies reviewed partial findings only as a guarded draft and prevents a second application", async () => {
  vi.mocked(definitionApi.listAssistance).mockResolvedValue({ jobs: [result()] });
  vi.mocked(definitionApi.assistance).mockResolvedValue(result());
  vi.mocked(definitionApi.applyAnalysis).mockResolvedValue({ ...specification(), revision: 3, briefRevision: 4, purpose: "A proposed understanding" });
  const onApplied = vi.fn(); mount(false, onApplied);
  await screen.findByText("The app renders a start route.");
  fireEvent.click(screen.getByLabelText("I reviewed the proposed definition and its partial source coverage."));
  fireEvent.click(screen.getByRole("button", { name: "Apply as definition draft" }));
  await screen.findByText("Applied as draft revision 3.");
  expect(definitionApi.applyAnalysis).toHaveBeenCalledExactlyOnceWith("owned", "analysis-one", { expectedRevision: 2, expectedBriefRevision: 3, acknowledgePartial: true });
  expect(onApplied).toHaveBeenCalledExactlyOnceWith(expect.objectContaining({ status: "draft", revision: 3 }));
  expect(screen.queryByRole("button", { name: "Apply as definition draft" })).toBeNull();
});

it("keeps a stale-source refusal actionable and preserves the saved definition", async () => {
  vi.mocked(definitionApi.listAssistance).mockResolvedValue({ jobs: [result()] });
  vi.mocked(definitionApi.assistance).mockResolvedValue(result());
  vi.mocked(definitionApi.applyAnalysis).mockRejectedValue(new Error("Source has changed. Run a new repository analysis."));
  const onApplied = vi.fn(); mount(false, onApplied);
  await screen.findByText("The app renders a start route.");
  fireEvent.click(screen.getByLabelText("I reviewed the proposed definition and its partial source coverage."));
  fireEvent.click(screen.getByRole("button", { name: "Apply as definition draft" }));
  expect((await screen.findByRole("alert")).textContent).toContain("Source has changed");
  expect(onApplied).not.toHaveBeenCalled();
  expect(screen.queryByText(/Applied as draft/)).toBeNull();
});

it("protects unsaved definition edits from analysis and replacement", async () => {
  vi.mocked(definitionApi.listAssistance).mockResolvedValue({ jobs: [result()] });
  vi.mocked(definitionApi.assistance).mockResolvedValue(result());
  mount(true);
  await screen.findByText("The app renders a start route.");
  expect((screen.getByRole("button", { name: "Analyse repository" }) as HTMLButtonElement).disabled).toBe(true);
  expect((screen.getByRole("button", { name: "Apply as definition draft" }) as HTMLButtonElement).disabled).toBe(true);
  expect(screen.getByText("Save your definition changes before analysing or applying a proposal.")).not.toBeNull();
  expect(definitionApi.analyseRepository).not.toHaveBeenCalled();
  expect(definitionApi.applyAnalysis).not.toHaveBeenCalled();
});

it("ignores a late source analysis from a project the user has left", async () => {
  let finish!: (value: DefinitionAssistance) => void;
  vi.mocked(definitionApi.analyseRepository).mockImplementation(() => new Promise(resolve => { finish = resolve; }));
  const onApplied = vi.fn(); const view = mount(false, onApplied);
  await screen.findByRole("button", { name: "Analyse repository" });
  await waitFor(() => expect((screen.getByLabelText("Analysis connection") as HTMLSelectElement).value).toBe(""));
  fireEvent.click(screen.getByLabelText("Send the selected source and definition to this provider for analysis."));
  fireEvent.click(screen.getByRole("button", { name: "Analyse repository" }));
  await waitFor(() => expect(definitionApi.analyseRepository).toHaveBeenCalledOnce());
  view.rerender(<RepositoryAnalysisPanel projectId="other" specification={{ ...specification(), projectId: "other" }} dirty={false} onApplied={onApplied} />);
  await act(async () => { finish(result()); });
  expect(screen.queryByText("The app renders a start route.")).toBeNull();
  expect(onApplied).not.toHaveBeenCalled();
});

it("lets the user retry failed history loading without invoking a provider", async () => {
  vi.mocked(definitionApi.listAssistance).mockRejectedValueOnce(new Error("Reconnect the workspace.")).mockResolvedValueOnce({ jobs: [result()] });
  vi.mocked(definitionApi.assistance).mockResolvedValue(result());
  mount(); await screen.findByRole("alert");
  fireEvent.click(screen.getByRole("button", { name: "Refresh saved analyses" }));
  await screen.findByText("The app renders a start route.");
  expect(definitionApi.analyseRepository).not.toHaveBeenCalled();
});

it("submits once and keeps a lost response blocked behind its exact saved request", async () => {
  let fail!: (cause: Error) => void;
  vi.mocked(definitionApi.analyseRepository).mockImplementationOnce(() => new Promise((_resolve, reject) => { fail = reject; }));
  mount();
  const action = await screen.findByRole("button", { name: "Analyse repository" });
  await waitFor(() => expect((screen.getByLabelText("Analysis connection") as HTMLSelectElement).value).toBe(""));
  expect(action.getAttribute("type")).toBe("submit");
  fireEvent.click(screen.getByLabelText("Send the selected source and definition to this provider for analysis."));
  fireEvent.click(action);
  expect(action.getAttribute("aria-busy")).toBe("true");
  expect((action as HTMLButtonElement).disabled).toBe(true);
  fireEvent.click(action);
  await waitFor(() => expect(definitionApi.analyseRepository).toHaveBeenCalledOnce());
  await act(async () => fail(new Error("Reconnect the selected provider before trying again.")));
  expect((await screen.findByRole("alert")).textContent).toContain("Reconnect the selected provider");
  expect((screen.getByLabelText("Send the selected source and definition to this provider for analysis.") as HTMLInputElement).checked).toBe(false);
  expect((screen.getByLabelText("Analysis model") as HTMLInputElement).value).toBe("configured-model");
  expect((action as HTMLButtonElement).disabled).toBe(true);
  expect(screen.getByRole("button", { name: "Check original request" })).not.toBeNull();
  expect(readRepositoryAnalysisRecovery(scope()).pending?.idempotencyKey).toBe(vi.mocked(definitionApi.analyseRepository).mock.calls[0][1].idempotencyKey);
  expect(definitionApi.applyAnalysis).not.toHaveBeenCalled();
});

it("renders the exact saved question and options, retains an unsent answer and sends it only with explicit consent", async () => {
  savedQuestion();
  const first = mount();
  expect(await screen.findByText("Should existing customer accounts be kept?")).not.toBeNull();
  fireEvent.click(screen.getByRole("button", { name: "Use answer: Keep existing accounts" }));
  expect((screen.getByLabelText("Your answer") as HTMLTextAreaElement).value).toBe("Keep existing accounts");
  await waitFor(() => expect(readRepositoryAnalysisRecovery(scope()).draft?.replyToAnalysisId).toBe("analysis-one"));
  expect(definitionApi.analyseRepository).not.toHaveBeenCalled();
  first.unmount(); mount();
  await screen.findByText("Should existing customer accounts be kept?");
  expect((screen.getByLabelText("Your answer") as HTMLTextAreaElement).value).toBe("Keep existing accounts");
  expect((screen.getByRole("button", { name: "Send answer" }) as HTMLButtonElement).disabled).toBe(true);
  fireEvent.click(screen.getByLabelText("Send the selected source and definition to this provider for analysis."));
  fireEvent.click(screen.getByRole("button", { name: "Send answer" }));
  await waitFor(() => expect(definitionApi.analyseRepository).toHaveBeenCalledExactlyOnceWith("owned", expect.objectContaining({ replyToAnalysisId: "analysis-one", message: "Keep existing accounts", harness: "codex", model: "configured-model", sourceMode: "worktree", acknowledgeSourceRead: true, idempotencyKey: expect.any(String) })));
  await waitFor(() => expect(readRepositoryAnalysisRecovery(scope()).pending?.idempotencyKey).toBe(vi.mocked(definitionApi.analyseRepository).mock.calls[0][1].idempotencyKey));
  expect(definitionApi.applyAnalysis).not.toHaveBeenCalled();
});

it("keeps later answer edits separate from the original pending payload and retains them after acceptance", async () => {
  savedQuestion();
  let finish!: (value: DefinitionAssistance) => void;
  vi.mocked(definitionApi.analyseRepository).mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  mount(); await screen.findByText("Should existing customer accounts be kept?");
  fireEvent.click(screen.getByRole("button", { name: "Use answer: Keep existing accounts" }));
  fireEvent.click(screen.getByLabelText("Send the selected source and definition to this provider for analysis."));
  fireEvent.click(screen.getByRole("button", { name: "Send answer" }));
  await waitFor(() => expect(definitionApi.analyseRepository).toHaveBeenCalledOnce());
  const request = vi.mocked(definitionApi.analyseRepository).mock.calls[0][1];
  fireEvent.change(screen.getByLabelText("Your answer"), { target: { value: "Later independent notes for another round" } });
  fireEvent.change(screen.getByLabelText("Source to analyse"), { target: { value: "commit" } });
  await waitFor(() => expect(readRepositoryAnalysisRecovery(scope()).draft?.sourceMode).toBe("commit"));
  expect(readRepositoryAnalysisRecovery(scope()).pending?.message).toBe("Keep existing accounts");
  expect(readRepositoryAnalysisRecovery(scope()).pending?.sourceMode).toBe("worktree");
  const editingAnswer = screen.getByLabelText("Your answer"); editingAnswer.focus();
  await act(async () => finish({ ...result(), id: "reply-two", idempotencyKey: request.idempotencyKey, replyToAnalysisId: request.replyToAnalysisId, userMessage: request.message, replySourceChanged: true }));
  await screen.findByText(/source changed since the earlier question/);
  expect(document.activeElement).toBe(editingAnswer);
  expect((screen.getByLabelText("Your answer") as HTMLTextAreaElement).value).toBe("Later independent notes for another round");
  expect(readRepositoryAnalysisRecovery(scope()).draft?.message).toBe("Later independent notes for another round");
  expect(readRepositoryAnalysisRecovery(scope()).pending).toBeNull();
  expect(definitionApi.analyseRepository).toHaveBeenCalledTimes(1); expect(definitionApi.applyAnalysis).not.toHaveBeenCalled();
});

it.each([undefined, 7])("uses read-only recovery and resends only the exact original request with route revision %s", async expectedAgentConnectionRevision => {
  const original = { ...originalRequest(), ...(expectedAgentConnectionRevision === undefined ? {} : { agentConnectionId: "saved-route", expectedAgentConnectionRevision }) };
  await retainRepositoryAnalysisRequest(scope(), original);
  await saveRepositoryAnalysisDraft(scope(), null, { message: "A newer unsent draft", replyToAnalysisId: "analysis-one", harness: "claude-code", model: "new-draft-model", agentConnectionId: "", sourceMode: "commit" });
  savedQuestion();
  vi.mocked(definitionApi.repositoryRequest).mockRejectedValue(new ApiError("project_not_found", "No receipt", 404));
  render(<RepositoryAnalysisPanel projectId="owned" specification={{ ...specification(), revision: 99 }} dirty={false} onApplied={vi.fn()} />);
  const retry = await screen.findByRole("button", { name: "Retry original request" });
  expect(definitionApi.repositoryRequest).toHaveBeenCalledExactlyOnceWith("owned", original.idempotencyKey);
  expect(definitionApi.analyseRepository).not.toHaveBeenCalled();
  expect((retry as HTMLButtonElement).disabled).toBe(true);
  fireEvent.click(screen.getByLabelText("Retry only this saved request with its original answer, source consent, provider and model."));
  fireEvent.click(retry);
  await waitFor(() => expect(definitionApi.analyseRepository).toHaveBeenCalledExactlyOnceWith("owned", original));
  await waitFor(() => expect(readRepositoryAnalysisRecovery(scope()).pending).toEqual(original));
  expect((screen.getByLabelText("Your answer") as HTMLTextAreaElement).value).toBe("A newer unsent draft");
  expect(readRepositoryAnalysisRecovery(scope()).draft?.model).toBe("new-draft-model");
});

it.each(["unavailable", "wrong-key", "not-owned", "ownership-absent"])("keeps %s receipt recovery uncertain without forgetting or invoking the provider", async kind => {
  const original = originalRequest(); await retainRepositoryAnalysisRequest(scope(), original);
  if (kind === "unavailable") vi.mocked(definitionApi.repositoryRequest).mockRejectedValue(new ApiError("internal", "Unavailable", 503));
  else vi.mocked(definitionApi.repositoryRequest).mockResolvedValue({ ...result(), idempotencyKey: kind === "wrong-key" ? "different-analysis-key" : original.idempotencyKey, ownRequest: kind === "ownership-absent" ? undefined : kind !== "not-owned", replyToAnalysisId: original.replyToAnalysisId });
  mount();
  expect((await screen.findByRole("alert")).textContent).toContain("still unconfirmed");
  expect(readRepositoryAnalysisRecovery(scope()).pending).toEqual(original);
  expect(screen.queryByRole("button", { name: "Retry original request" })).toBeNull();
  expect(screen.queryByText("Forget saved request")).toBeNull();
  expect(definitionApi.analyseRepository).not.toHaveBeenCalled();
});

it("joins a reserved own receipt without dispatch and blocks another send while admission is pending", async () => {
  const original = originalRequest(); await retainRepositoryAnalysisRequest(scope(), original);
  vi.mocked(definitionApi.repositoryRequest).mockResolvedValue({ ...result(), idempotencyKey: original.idempotencyKey, replyToAnalysisId: original.replyToAnalysisId, status: "reserved", message: "Preparing the saved request", result: undefined });
  mount();
  await screen.findByText("Preparing the saved request");
  expect((screen.getByRole("button", { name: "Analyse repository" }) as HTMLButtonElement).disabled).toBe(true);
  expect(definitionApi.analyseRepository).not.toHaveBeenCalled();
});

it.each([false, undefined])("treats ownRequest %s as history without inferring ownership from the local account ID", async ownRequest => {
  vi.mocked(definitionApi.listAssistance).mockResolvedValue({ jobs: [question()] });
  vi.mocked(definitionApi.assistance).mockResolvedValue({ ...question(), ownRequest });
  mount(); await screen.findByText("Should existing customer accounts be kept?");
  expect((screen.getByRole("button", { name: "Use answer: Keep existing accounts" }) as HTMLButtonElement).disabled).toBe(true);
  expect(screen.queryByRole("button", { name: "Answer this question" })).toBeNull();
  expect(screen.getByText(/another person or an older unattributed analysis/)).not.toBeNull();
  expect(definitionApi.analyseRepository).not.toHaveBeenCalled();
});

it("fences late analysis acknowledgement across accounts without clearing the original pending request", async () => {
  let finish!: (value: DefinitionAssistance) => void;
  vi.mocked(definitionApi.analyseRepository).mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  const view = mount();
  await waitFor(() => expect((screen.getByLabelText("Analysis connection") as HTMLSelectElement).value).toBe(""));
  fireEvent.click(screen.getByLabelText("Send the selected source and definition to this provider for analysis."));
  fireEvent.click(screen.getByRole("button", { name: "Analyse repository" }));
  await waitFor(() => expect(definitionApi.analyseRepository).toHaveBeenCalledOnce());
  const originalScope = scope(), input = vi.mocked(definitionApi.analyseRepository).mock.calls[0][1];
  identity.actor = "new-actor"; view.rerender(<RepositoryAnalysisPanel projectId="owned" specification={specification()} dirty={false} onApplied={vi.fn()} />);
  await act(async () => finish({ ...result(), idempotencyKey: input.idempotencyKey }));
  expect(readRepositoryAnalysisRecovery(originalScope).pending?.idempotencyKey).toBe(input.idempotencyKey);
  expect(readRepositoryAnalysisRecovery(scope()).pending).toBeNull();
  expect(screen.queryByText("The app renders a start route.")).toBeNull();
});

it("allows a demoted reader to check their own submitted request without provider access", async () => {
  const original = originalRequest(); await retainRepositoryAnalysisRequest(scope(), original);
  identity.assist = false;
  vi.mocked(definitionApi.repositoryRequest).mockResolvedValue({ ...result(), idempotencyKey: original.idempotencyKey, replyToAnalysisId: original.replyToAnalysisId });
  mount(); await screen.findByText("The app renders a start route.");
  expect(definitionApi.repositoryRequest).toHaveBeenCalledExactlyOnceWith("owned", original.idempotencyKey);
  expect((screen.getByRole("button", { name: "Analyse repository" }) as HTMLButtonElement).disabled).toBe(true);
  expect(definitionApi.analyseRepository).not.toHaveBeenCalled();
});

it("refuses to send when the browser cannot durably coordinate request recovery", async () => {
  Reflect.deleteProperty(navigator, "locks");
  mount();
  await screen.findByText(/browser with Web Locks/);
  fireEvent.click(screen.getByLabelText("Send the selected source and definition to this provider for analysis."));
  fireEvent.click(screen.getByRole("button", { name: "Analyse repository" }));
  expect(definitionApi.analyseRepository).not.toHaveBeenCalled();
});

it("recovers an original receipt through Strict Mode remounting without leaving actions stuck", async () => {
  const original = originalRequest(); await retainRepositoryAnalysisRequest(scope(), original);
  let obsolete!: (value: DefinitionAssistance) => void;
  vi.mocked(definitionApi.repositoryRequest).mockImplementationOnce(() => new Promise(resolve => { obsolete = resolve; }))
    .mockResolvedValue({ ...result(), idempotencyKey: original.idempotencyKey, replyToAnalysisId: original.replyToAnalysisId });
  render(<StrictMode><RepositoryAnalysisPanel projectId="owned" specification={specification()} dirty={false} onApplied={vi.fn()} /></StrictMode>);
  await screen.findByText("The app renders a start route.");
  expect(definitionApi.repositoryRequest).toHaveBeenCalledTimes(2);
  expect((screen.getByRole("button", { name: "Refresh saved analyses" }) as HTMLButtonElement).disabled).toBe(false);
  expect(readRepositoryAnalysisRecovery(scope()).pending).toBeNull();
  await act(async () => obsolete({ ...result(), id: "obsolete", idempotencyKey: original.idempotencyKey, replyToAnalysisId: original.replyToAnalysisId, message: "Obsolete receipt" }));
  expect(screen.queryByText("Obsolete receipt")).toBeNull();
  expect(definitionApi.analyseRepository).not.toHaveBeenCalled();
});

it("requires renewed source consent if the selected route changes revision under the same ID and model", async () => {
  const gateway = { id: "chosen-gateway", revision: 2, name: "Company gateway", kind: "litellm" as const, enabled: true, configured: true, keyPresent: true, endpoints: { responses: "https://gateway.example/v1" }, models: { codex: "gateway-coder" } };
  vi.mocked(productApi.agentConnections).mockResolvedValue({ revision: 3, activeConnectionId: "chosen-gateway", connections: [gateway] });
  mount();
  await waitFor(() => expect((screen.getByLabelText("Analysis connection") as HTMLSelectElement).value).toBe("chosen-gateway"));
  fireEvent.click(screen.getByLabelText("Send the selected source and definition to this provider for analysis."));
  expect((screen.getByRole("button", { name: "Analyse repository" }) as HTMLButtonElement).disabled).toBe(false);
  vi.mocked(productApi.agentConnections).mockResolvedValue({ revision: 4, activeConnectionId: "chosen-gateway", connections: [{ ...gateway, revision: 3, endpoints: { responses: "https://different.example/v1" } }] });
  act(() => window.dispatchEvent(new Event(AGENT_CONNECTIONS_CHANGED)));
  await waitFor(() => expect((screen.getByLabelText("Send the selected source and definition to this provider for analysis.") as HTMLInputElement).checked).toBe(false));
  expect((screen.getByLabelText("Analysis model") as HTMLInputElement).value).toBe("gateway-coder");
  expect((screen.getByRole("button", { name: "Analyse repository" }) as HTMLButtonElement).disabled).toBe(true);
  expect(definitionApi.analyseRepository).not.toHaveBeenCalled();
  fireEvent.click(screen.getByLabelText("Send the selected source and definition to this provider for analysis."));
  fireEvent.click(screen.getByRole("button", { name: "Analyse repository" }));
  await waitFor(() => expect(definitionApi.analyseRepository).toHaveBeenCalledExactlyOnceWith("owned", expect.objectContaining({ agentConnectionId: "chosen-gateway", model: "gateway-coder", expectedAgentConnectionRevision: 3 })));
});

it.each([undefined, 0, 1.5, Number.MAX_SAFE_INTEGER + 1])("blocks source dispatch when named connection revision %s is unverifiable", async revision => {
  vi.mocked(productApi.agentConnections).mockResolvedValue({ revision: 3, activeConnectionId: "chosen-gateway", connections: [{ id: "chosen-gateway", revision: revision as number, name: "Company gateway", kind: "litellm", enabled: true, configured: true, keyPresent: true, endpoints: { responses: "https://gateway.example/v1" }, models: { codex: "gateway-coder" } }] });
  mount();
  await screen.findByText(/connection's revision is unavailable/);
  fireEvent.click(screen.getByLabelText("Send the selected source and definition to this provider for analysis."));
  expect((screen.getByRole("button", { name: "Analyse repository" }) as HTMLButtonElement).disabled).toBe(true);
  expect(definitionApi.analyseRepository).not.toHaveBeenCalled();
  expect(readRepositoryAnalysisRecovery(scope()).pending).toBeNull();
});

it("uses server-projected ownership for a selected remote workspace whose actor differs from the local account", async () => {
  identity.actor = "local-alice";
  const remoteQuestion = { ...question(), requestedBy: "remote-bob", ownRequest: true };
  vi.mocked(definitionApi.listAssistance).mockResolvedValue({ jobs: [remoteQuestion] });
  vi.mocked(definitionApi.assistance).mockResolvedValue(remoteQuestion);
  vi.mocked(definitionApi.analyseRepository).mockImplementationOnce(async (_project, input) => ({ ...result(), id: "remote-reply", requestedBy: "remote-bob", ownRequest: true, idempotencyKey: input.idempotencyKey, replyToAnalysisId: input.replyToAnalysisId }));
  mount(); await screen.findByText("Should existing customer accounts be kept?");
  fireEvent.click(screen.getByRole("button", { name: "Use answer: Keep existing accounts" }));
  fireEvent.click(screen.getByLabelText("Send the selected source and definition to this provider for analysis."));
  fireEvent.click(screen.getByRole("button", { name: "Send answer" }));
  await waitFor(() => expect(definitionApi.analyseRepository).toHaveBeenCalledExactlyOnceWith("owned", expect.objectContaining({ replyToAnalysisId: "analysis-one", message: "Keep existing accounts" })));
  await screen.findByText("The app renders a start route.");
  expect(readRepositoryAnalysisRecovery(scope()).pending).toBeNull();
  expect(screen.queryByText(/could not be verified/)).toBeNull();
});

it("retains an accepted request across reload and hidden older history until exact polling observes a terminal receipt", async () => {
  const first = mount();
  await waitFor(() => expect((screen.getByLabelText("Analysis connection") as HTMLSelectElement).value).toBe(""));
  fireEvent.click(screen.getByLabelText("Send the selected source and definition to this provider for analysis."));
  fireEvent.click(screen.getByRole("button", { name: "Analyse repository" }));
  await waitFor(() => expect(definitionApi.analyseRepository).toHaveBeenCalledOnce());
  await screen.findByRole("button", { name: "Cancel repository analysis" });
  const original = readRepositoryAnalysisRecovery(scope()).pending!;
  expect(original.idempotencyKey).toBe(vi.mocked(definitionApi.analyseRepository).mock.calls[0][1].idempotencyKey);
  first.unmount();
  vi.mocked(definitionApi.listAssistance).mockResolvedValue({ jobs: Array.from({ length: 20 }, (_, i) => ({ ...result(), id: `newer-${i}`, status: "failed" as const, result: undefined })) });
  vi.mocked(definitionApi.assistance).mockResolvedValue({ ...result(), id: "newer-0", status: "failed", result: undefined });
  vi.mocked(definitionApi.repositoryRequest).mockResolvedValue({ ...result(), status: "running", result: undefined, idempotencyKey: original.idempotencyKey, message: "Original analysis is still running" });
  mount();
  await screen.findByText("Original analysis is still running");
  expect(definitionApi.repositoryRequest).toHaveBeenCalledWith("owned", original.idempotencyKey);
  expect(readRepositoryAnalysisRecovery(scope()).pending).toEqual(original);
  vi.mocked(definitionApi.repositoryRequest).mockResolvedValue({ ...result(), idempotencyKey: original.idempotencyKey, message: "Original analysis completed" });
  await waitFor(() => expect(screen.queryByText("Original analysis completed")).not.toBeNull(), { timeout: 4000 });
  expect(readRepositoryAnalysisRecovery(scope()).pending).toBeNull();
  expect(definitionApi.analyseRepository).toHaveBeenCalledTimes(1);
});

it("discards an unsent draft to free its storage record while retaining an unresolved original request", async () => {
  const original = originalRequest(); await retainRepositoryAnalysisRequest(scope(), original);
  await saveRepositoryAnalysisDraft(scope(), null, { message: "Unsent new answer", replyToAnalysisId: "analysis-one", harness: "codex", model: "configured-model", agentConnectionId: "", sourceMode: "worktree" });
  vi.mocked(definitionApi.repositoryRequest).mockRejectedValue(new ApiError("internal", "Unavailable", 503));
  mount(); await screen.findByText(/still unconfirmed/);
  fireEvent.click(screen.getByText("Other draft actions"));
  fireEvent.click(screen.getByRole("button", { name: "Discard saved analysis draft" }));
  await waitFor(() => expect(readRepositoryAnalysisRecovery(scope()).draft).toBeNull());
  expect((screen.getByLabelText("What should the analysis focus on?") as HTMLTextAreaElement).value).toBe("");
  expect(readRepositoryAnalysisRecovery(scope()).pending).toEqual(original);
  expect(definitionApi.analyseRepository).not.toHaveBeenCalled();
});

it("places the question before answer and provider review, and keyboard option selection focuses the editor without sending", async () => {
  const user = userEvent.setup(); savedQuestion(); mount();
  const questionText = await screen.findByText("Should existing customer accounts be kept?");
  const input = screen.getByLabelText("What should the analysis focus on?");
  const provider = screen.getByLabelText("Analysis provider");
  const evidence = screen.getByText("Review findings and source evidence");
  expect(questionText.compareDocumentPosition(input) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  expect(input.compareDocumentPosition(provider) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  expect(provider.compareDocumentPosition(evidence) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  expect((evidence.closest("details") as HTMLDetailsElement).open).toBe(false);
  screen.getByRole("button", { name: "Use answer: Keep existing accounts" }).focus();
  await user.keyboard("{Enter}");
  const answer = screen.getByLabelText("Your answer") as HTMLTextAreaElement;
  expect(document.activeElement).toBe(answer);
  expect(answer.value).toBe("Keep existing accounts");
  expect((screen.getByRole("button", { name: "Send answer" }) as HTMLButtonElement).disabled).toBe(true);
  expect((screen.getByLabelText("Send the selected source and definition to this provider for analysis.") as HTMLInputElement).checked).toBe(false);
  await waitFor(() => expect(readRepositoryAnalysisRecovery(scope()).draft?.message).toBe("Keep existing accounts"));
  expect(definitionApi.analyseRepository).not.toHaveBeenCalled();
  expect(definitionApi.applyAnalysis).not.toHaveBeenCalled();
});

it("restores an answered draft without taking focus and lets keyboard Answer resume the same text", async () => {
  const user = userEvent.setup();
  await saveRepositoryAnalysisDraft(scope(), null, { message: "Keep accounts and their existing notes", replyToAnalysisId: "analysis-one", harness: "codex", model: "configured-model", agentConnectionId: "", sourceMode: "commit" });
  let complete!: (value: DefinitionAssistance) => void;
  vi.mocked(definitionApi.listAssistance).mockResolvedValue({ jobs: [question()] });
  vi.mocked(definitionApi.assistance).mockImplementationOnce(() => new Promise(resolve => { complete = resolve; }));
  render(<><input aria-label="Other project notes" /><RepositoryAnalysisPanel projectId="owned" specification={specification()} dirty={false} onApplied={vi.fn()} /></>);
  const otherEditor = screen.getByLabelText("Other project notes"); otherEditor.focus();
  await waitFor(() => expect(definitionApi.assistance).toHaveBeenCalled());
  await act(async () => complete(question()));
  await screen.findByText("Should existing customer accounts be kept?");
  expect(document.activeElement).toBe(otherEditor);
  const answer = screen.getByLabelText("Your answer") as HTMLTextAreaElement;
  expect(answer.value).toBe("Keep accounts and their existing notes");
  expect(screen.getByText("Should existing customer accounts be kept?").compareDocumentPosition(answer) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  screen.getByRole("button", { name: "Answer this question" }).focus();
  await user.keyboard(" ");
  expect(document.activeElement).toBe(answer);
  expect(answer.value).toBe("Keep accounts and their existing notes");
  expect((screen.getByLabelText("Source to analyse") as HTMLSelectElement).value).toBe("commit");
  expect(definitionApi.analyseRepository).not.toHaveBeenCalled();
  expect(definitionApi.applyAnalysis).not.toHaveBeenCalled();
});
