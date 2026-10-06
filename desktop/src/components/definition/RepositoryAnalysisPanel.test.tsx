import { beforeEach, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { RepositoryAnalysisPanel } from "./RepositoryAnalysisPanel";
import { definitionApi, type DefinitionAssistance, type ProjectSpecification } from "../../api/definition";
import { productApi } from "../../api/product";
import { resetCockpit, useCockpit } from "../../store/store";

vi.mock("../../api/definition", () => ({ definitionApi: { listAssistance: vi.fn(), assistance: vi.fn(), analyseRepository: vi.fn(), applyAnalysis: vi.fn(), cancelAssistance: vi.fn() } }));
vi.mock("../../api/product", async original => ({ ...await original<typeof import("../../api/product")>(), productApi: { agents: vi.fn(), agentConnections: vi.fn() } }));
const specification = (): ProjectSpecification => ({ projectId: "owned", version: 1, revision: 2, briefRevision: 3, startingPoint: "existing", purpose: "Understand this app", audience: "Its users", constraints: "Keep data", outOfScope: [], requirements: [], journeys: [], roles: [], entities: [], integrations: [], glossary: [], decisions: [], questions: [], status: "draft", digest: "", updatedAt: "" });
const result = (): DefinitionAssistance => ({
  id: "analysis-one", projectId: "owned", kind: "repository", status: "succeeded", harness: "codex", model: "configured-model", authMode: "chatgpt", message: "Analysis saved", createdAt: "2026-10-05T00:00:00Z", updatedAt: "2026-10-05T00:00:05Z",
  source: { mode: "worktree", digest: "source-fingerprint", fileCount: 1, includedBytes: 100, skippedFiles: 7, partial: true, limitation: "Partial source selection", files: [{ path: "src/app.ts", digest: "file-fingerprint", bytes: 100 }] },
  result: { question: "", options: [], rationale: "The route is present; persistence needs clarification.", findings: [{ kind: "observed", summary: "The app renders a start route.", paths: ["src/app.ts"] }, { kind: "gap", summary: "Persistence was not observed in the selected files.", paths: [] }], proposal: { ...specification(), purpose: "A proposed understanding" } },
});
beforeEach(() => {
  vi.resetAllMocks(); resetCockpit();
  vi.mocked(definitionApi.listAssistance).mockResolvedValue({ jobs: [] });
  vi.mocked(productApi.agents).mockResolvedValue({ providers: [] });
  vi.mocked(productApi.agentConnections).mockResolvedValue({ revision: 0, activeConnectionId: "", connections: [] });
  useCockpit.getState().setDefaults({ harness: "codex", model: "configured-model" });
});
const mount = (dirty = false, onApplied = vi.fn()) => render(<RepositoryAnalysisPanel projectId="owned" specification={specification()} dirty={dirty} onApplied={onApplied} />);

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
  await waitFor(() => expect(definitionApi.analyseRepository).toHaveBeenCalledExactlyOnceWith("owned", { expectedRevision: 2, expectedBriefRevision: 3, sourceMode: "worktree", harness: "codex", model: "chosen-account-model", acknowledgeSourceRead: true }));
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
  await waitFor(() => expect(definitionApi.analyseRepository).toHaveBeenCalledWith("owned", expect.objectContaining({ harness: "codex", model: "gateway-coder", agentConnectionId: "chosen-gateway" })));
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
  fireEvent.click(screen.getByLabelText("Send the selected source and definition to this provider for analysis."));
  fireEvent.click(screen.getByRole("button", { name: "Analyse repository" }));
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

it("submits once while pending and retains explicit source consent for a failed-start retry", async () => {
  let fail!: (cause: Error) => void;
  vi.mocked(definitionApi.analyseRepository).mockImplementationOnce(() => new Promise((_resolve, reject) => { fail = reject; }));
  mount();
  const action = await screen.findByRole("button", { name: "Analyse repository" });
  expect(action.getAttribute("type")).toBe("submit");
  fireEvent.click(screen.getByLabelText("Send the selected source and definition to this provider for analysis."));
  fireEvent.click(action);
  expect(action.getAttribute("aria-busy")).toBe("true");
  expect((action as HTMLButtonElement).disabled).toBe(true);
  fireEvent.click(action);
  expect(definitionApi.analyseRepository).toHaveBeenCalledOnce();
  await act(async () => fail(new Error("Reconnect the selected provider before trying again.")));
  expect((await screen.findByRole("alert")).textContent).toContain("Reconnect the selected provider");
  expect((screen.getByLabelText("Send the selected source and definition to this provider for analysis.") as HTMLInputElement).checked).toBe(true);
  expect((screen.getByLabelText("Analysis model") as HTMLInputElement).value).toBe("configured-model");
  expect((action as HTMLButtonElement).disabled).toBe(false);
  expect(definitionApi.applyAnalysis).not.toHaveBeenCalled();
});
