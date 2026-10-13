import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, expect, it, vi } from "vitest";
import { coordinatorPolicyApi, type CoordinatorPolicy as Policy } from "../../api/coordinatorPolicy";
import { orchestratorApi, type OrchestratorAdvanceRequest, type OrchestratorView } from "../../api/orchestrator";
import { ApiError } from "../../api/client";
import { ProductApiError } from "../../api/product";
import { CoordinatorPolicy } from "./CoordinatorPolicy";
import { context, proposalTurn, view } from "./fixtures";

const policy = (patch: Partial<Policy> = {}): Policy => ({ projectId: "p-1", revision: 1, mode: "review", reviewMode: "manual", allowedTargets: ["specification", "design-plan", "plan"], executionMode: "manual", deploymentMode: "manual", actor: { id: "alice", displayName: "Alice" }, updatedAt: "2026-10-12T00:00:00Z", ...patch });
const pendingView = () => view({ turns: [proposalTurn()] });
const savedView = () => view({ revision: 4, context: { ...context(), digest: "new-context" }, turns: [{ ...proposalTurn(), appliedRevision: 3 }] });
const props = () => {
  const onObserved = vi.fn();
  return { projectId: "p-1", scope: "workspace/alice/p-1", actorId: "alice", view: pendingView(), canRead: true, canEdit: true, blocked: "", onAdvance: vi.fn<(id: string, input: OrchestratorAdvanceRequest) => Promise<OrchestratorView>>().mockResolvedValue(savedView()), onObserved, beginObservation: () => onObserved };
};
function deferred<T>() { let resolve!: (value: T) => void, reject!: (cause: unknown) => void; const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no; }); return { promise, resolve, reject }; }
const records = () => Object.keys(localStorage).filter(key => key.startsWith("luci.coordinator-request.v1:"));
async function open() { await userEvent.click(screen.getByText(/Context policy ·/)); }
beforeEach(() => {
  vi.restoreAllMocks(); localStorage.clear();
  vi.spyOn(coordinatorPolicyApi, "get").mockResolvedValue(policy());
  vi.spyOn(coordinatorPolicyApi, "save").mockImplementation(async (_id, input) => policy({ revision: input.expectedRevision + 1, mode: input.mode, reviewMode: input.reviewMode ?? "manual", allowedTargets: input.allowedTargets }));
  vi.spyOn(orchestratorApi, "get").mockResolvedValue(pendingView());
});
it("defaults to review and requires explicit consent for selected document targets, without execution/deploy grant", async () => {
  const input = props(); render(<CoordinatorPolicy {...input} />); await open();
  await userEvent.selectOptions(screen.getByLabelText("Draft policy"), "auto-drafts");
  await userEvent.click(screen.getByRole("button", { name: "Save draft policy" }));
  expect(coordinatorPolicyApi.save).not.toHaveBeenCalled(); expect(input.onAdvance).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole("checkbox", { name: /Apply this context policy/ }));
  await userEvent.click(screen.getByRole("checkbox", { name: "Design" }));
  expect((screen.getByRole("checkbox", { name: /Apply this context policy/ }) as HTMLInputElement).checked).toBe(false);
  await userEvent.click(screen.getByRole("checkbox", { name: /Apply this context policy/ }));
  await userEvent.click(screen.getByRole("button", { name: "Save draft policy" }));
  await waitFor(() => expect(coordinatorPolicyApi.save).toHaveBeenCalledExactlyOnceWith("p-1", { expectedRevision: 1, mode: "auto-drafts", reviewMode: "manual", allowedTargets: ["specification", "plan"] }));
  expect(screen.getByText(/Neither option starts a build/)).not.toBeNull();
});
it("requires a separate opt-in to renew reviewed definition or design amendments", async () => {
  render(<CoordinatorPolicy {...props()} />); await open();
  await userEvent.selectOptions(screen.getByLabelText("Draft policy"), "auto-drafts");
  await userEvent.selectOptions(screen.getByLabelText("Routine amendment review"), "approved-amendments");
  await userEvent.click(screen.getByRole("checkbox", { name: /Apply this context policy/ }));
  await userEvent.click(screen.getByRole("button", { name: "Save draft policy" }));
  await waitFor(() => expect(coordinatorPolicyApi.save).toHaveBeenCalledExactlyOnceWith("p-1", { expectedRevision: 1, mode: "auto-drafts", reviewMode: "approved-amendments", allowedTargets: ["specification", "design-plan", "plan"] }));
  expect(screen.getByText(/An initial definition\/design review and unresolved human questions still need me/)).not.toBeNull();
});
it("saves only the current person's current nonblocking proposal exactly once, even while the parent view lags", async () => {
  vi.mocked(coordinatorPolicyApi.get).mockResolvedValue(policy({ mode: "auto-drafts" }));
  const input = props(), ui = render(<CoordinatorPolicy {...input} />);
  await waitFor(() => expect(input.onAdvance).toHaveBeenCalledExactlyOnceWith("turn-1", { expectedRevision: 3, expectedContextDigest: context().digest, expectedPolicyRevision: 1 }));
  await waitFor(() => expect(records()).toHaveLength(0));
  ui.rerender(<CoordinatorPolicy {...input} view={pendingView()} />);
  await act(async () => {});
  expect(input.onAdvance).toHaveBeenCalledTimes(1);
});
it.each(["other-actor", "material-question", "blocking-answer", "unselected-target", "drafts", "read-only"])("refuses automatic draft saving for %s", async reason => {
  const input = props(), p = policy({ mode: "auto-drafts" });
  if (reason === "other-actor") input.actorId = "bob";
  if (reason === "material-question") input.view.blockingQuestions = [{ turnId: "decision", question: "Who can access this?", rationale: "Privacy", options: [] }];
  if (reason === "blocking-answer") input.view.turns[0].answer!.blocking = true;
  if (reason === "unselected-target") p.allowedTargets = ["plan"];
  if (reason === "drafts") input.blocked = "Unsaved design edits.";
  if (reason === "read-only") input.canEdit = false;
  vi.mocked(coordinatorPolicyApi.get).mockResolvedValue(p);
  render(<CoordinatorPolicy {...input} />); await open();
  expect(input.onAdvance).not.toHaveBeenCalled(); expect(records()).toHaveLength(0);
});
it("retains an unconfirmed advance across remount until a subsequent explicit exact readback, without replay", async () => {
  vi.mocked(coordinatorPolicyApi.get).mockResolvedValue(policy({ mode: "auto-drafts" }));
  const input = props(); input.onAdvance.mockRejectedValue(new Error("Reply lost"));
  const first = render(<CoordinatorPolicy {...input} />); await open();
  await screen.findByText(/Reply lost/); expect(records()).toHaveLength(1); first.unmount();
  render(<CoordinatorPolicy {...input} />); await open();
  const held = deferred<OrchestratorView>(); vi.mocked(orchestratorApi.get).mockReturnValue(held.promise);
  await userEvent.click(screen.getByRole("button", { name: "Refresh policy and conversation" }));
  expect(records()).toHaveLength(1); expect(input.onAdvance).toHaveBeenCalledTimes(1);
  await act(async () => held.resolve(savedView()));
  await screen.findByText(/retained amendment was saved/);
  expect(input.onObserved).toHaveBeenCalledWith(savedView()); expect(records()).toHaveLength(0);
  // The caller must adopt the readback before automatic work can proceed. A
  // stale parent view must not re-run the same already-confirmed request.
  expect(input.onAdvance).toHaveBeenCalledTimes(1);
});
it("does not accept an unrelated turn's applied revision as resolution", async () => {
  vi.mocked(coordinatorPolicyApi.get).mockResolvedValue(policy({ mode: "auto-drafts" }));
  const input = props(); input.onAdvance.mockRejectedValue(new Error("Reply lost")); render(<CoordinatorPolicy {...input} />); await open(); await screen.findByText(/Reply lost/);
  vi.mocked(orchestratorApi.get).mockResolvedValue(view({ turns: [{ ...proposalTurn(), id: "different", appliedRevision: 8 }] }));
  await userEvent.click(screen.getByRole("button", { name: "Refresh policy and conversation" }));
  await screen.findByText(/Current policy and conversation read/);
  expect(records()).toHaveLength(1); expect(input.onAdvance).toHaveBeenCalledTimes(1);
});
it("preserves an uncertain policy save through edit revocation/restoration until the later revision fences it", async () => {
  const input = props(); input.view = view(); const held = deferred<Policy>();
  vi.mocked(coordinatorPolicyApi.save).mockReturnValue(held.promise);
  const ui = render(<CoordinatorPolicy {...input} />); await open();
  await userEvent.selectOptions(screen.getByLabelText("Draft policy"), "auto-drafts");
  await userEvent.click(screen.getByRole("checkbox", { name: /Apply this context policy/ }));
  await userEvent.click(screen.getByRole("button", { name: "Save draft policy" }));
  ui.rerender(<CoordinatorPolicy {...input} canEdit={false} />); await act(async () => {});
  ui.rerender(<CoordinatorPolicy {...input} />); await act(async () => {});
  await act(async () => held.resolve(policy({ revision: 2, mode: "auto-drafts" })));
  expect(records()).toHaveLength(1);
  expect(screen.getByRole("button", { name: "Save draft policy" }).matches(":disabled")).toBe(true);
  vi.mocked(coordinatorPolicyApi.get).mockResolvedValue(policy({ revision: 2, mode: "auto-drafts" }));
  await userEvent.click(screen.getByRole("button", { name: "Refresh policy and conversation" }));
  await waitFor(() => expect(records()).toHaveLength(0));
  expect(coordinatorPolicyApi.save).toHaveBeenCalledTimes(1);
});
it("does not clear unconfirmed policy work from a pre-write revision or unblock before both reads finish", async () => {
  const input = props(); input.view = view(); vi.mocked(coordinatorPolicyApi.save).mockRejectedValue(new Error("Reply lost"));
  render(<CoordinatorPolicy {...input} />); await open(); await userEvent.click(screen.getByRole("checkbox", { name: /Apply this context policy/ }));
  await userEvent.click(screen.getByRole("button", { name: "Save draft policy" })); await screen.findByText(/Reply lost/);
  await userEvent.click(screen.getByRole("button", { name: "Refresh policy and conversation" })); await screen.findByText(/Current policy and conversation read/);
  expect(records()).toHaveLength(1); expect(coordinatorPolicyApi.save).toHaveBeenCalledTimes(1);
});
it("suppresses late private responses after scope unmount and preserves the old scope's request", async () => {
  vi.mocked(coordinatorPolicyApi.get).mockResolvedValue(policy({ mode: "auto-drafts" }));
  const first = props(), held = deferred<OrchestratorView>(); first.onAdvance.mockReturnValue(held.promise);
  const ui = render(<CoordinatorPolicy key={first.scope} {...first} />);
  await waitFor(() => expect(first.onAdvance).toHaveBeenCalledTimes(1));
  vi.mocked(coordinatorPolicyApi.get).mockResolvedValue(policy({ projectId: "p-2" }));
  const second = { ...props(), projectId: "p-2", scope: "workspace/alice/p-2", view: view({ projectId: "p-2", context: context("p-2") }) };
  ui.rerender(<CoordinatorPolicy key={second.scope} {...second} />);
  await act(async () => held.resolve(savedView()));
  expect(first.onObserved).not.toHaveBeenCalled(); expect(second.onObserved).not.toHaveBeenCalled();
  expect(second.onAdvance).not.toHaveBeenCalled(); expect(records()).toEqual([`luci.coordinator-request.v1:${first.scope}`]);
});
it("refuses automatic work when retained request storage cannot be written", async () => {
  vi.mocked(coordinatorPolicyApi.get).mockResolvedValue(policy({ mode: "auto-drafts" }));
  vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => { throw new Error("Storage unavailable"); });
  const input = props(); render(<CoordinatorPolicy {...input} />); await open();
  await screen.findByText("Storage unavailable"); expect(input.onAdvance).not.toHaveBeenCalled();
});
it("requires a subsequent fresh read after definite policy refusal, then permits a newly consented change", async () => {
  const input = props(); input.view = view();
  vi.mocked(coordinatorPolicyApi.save).mockRejectedValueOnce(new ProductApiError("conflict", "Editing access revoked", 403, { error: "coordinator_forbidden" }));
  render(<CoordinatorPolicy {...input} />); await open();
  await userEvent.click(screen.getByRole("checkbox", { name: /Apply this context policy/ }));
  await userEvent.click(screen.getByRole("button", { name: "Save draft policy" }));
  await screen.findByText(/Editing access revoked/);
  expect(records()).toHaveLength(1);
  await userEvent.click(screen.getByRole("button", { name: "Refresh policy and conversation" }));
  await waitFor(() => expect(records()).toHaveLength(0));
  expect((screen.getByRole("checkbox", { name: /Apply this context policy/ }) as HTMLInputElement).checked).toBe(false);
  expect(coordinatorPolicyApi.save).toHaveBeenCalledTimes(1);
});
it("settles a definite automatic refusal by explicit read without retrying the unchanged proposal or claiming it was saved", async () => {
  vi.mocked(coordinatorPolicyApi.get).mockResolvedValue(policy({ mode: "auto-drafts" }));
  const input = props(); input.onAdvance.mockRejectedValue(new ProductApiError("conflict", "Question now unresolved", 409, { error: "orchestrator_question" }));
  render(<CoordinatorPolicy {...input} />); await open(); await screen.findByText(/Question now unresolved/);
  await userEvent.click(screen.getByRole("button", { name: "Refresh policy and conversation" }));
  await waitFor(() => expect(records()).toHaveLength(0));
  expect(input.onAdvance).toHaveBeenCalledTimes(1);
  expect(screen.queryByText(/retained amendment was saved/)).toBeNull();
});
it("removes cached policy presentation after an explicit read refusal and retains the pending request", async () => {
  vi.mocked(coordinatorPolicyApi.get).mockResolvedValue(policy({ mode: "auto-drafts" }));
  const input = props(); input.onAdvance.mockRejectedValue(new Error("Reply lost"));
  render(<CoordinatorPolicy {...input} />); await open(); await screen.findByText(/Reply lost/);
  vi.mocked(coordinatorPolicyApi.get).mockRejectedValue(new ApiError("conflict", "Read revoked", 403));
  await userEvent.click(screen.getByRole("button", { name: "Refresh policy and conversation" }));
  await screen.findByRole("region", { name: "Coordinator policy unavailable" });
  expect(screen.queryByLabelText("Draft policy")).toBeNull(); expect(records()).toHaveLength(1);
  expect(input.onAdvance).toHaveBeenCalledTimes(1);
});
it("retains uncertainty when a client scope fence discards the response with HTTP-like 409", async () => {
  vi.mocked(coordinatorPolicyApi.get).mockResolvedValue(policy({ mode: "auto-drafts" }));
  const input = props(); input.onAdvance.mockRejectedValue(new ApiError("conflict", "Workspace changed before this response arrived", 409));
  render(<CoordinatorPolicy {...input} />); await open(); await screen.findByText(/Workspace changed before this response arrived/);
  await userEvent.click(screen.getByRole("button", { name: "Refresh policy and conversation" })); await screen.findByText(/Current policy and conversation read/);
  expect(records()).toHaveLength(1); expect(input.onAdvance).toHaveBeenCalledTimes(1);
});
it("does not publish a held conversation read over a newer parent context at the same conversation revision", async () => {
  const input = props(); input.view = view(); const held = deferred<OrchestratorView>();
  const ui = render(<CoordinatorPolicy {...input} />); await open();
  vi.mocked(orchestratorApi.get).mockReturnValue(held.promise);
  await userEvent.click(screen.getByRole("button", { name: "Refresh policy and conversation" }));
  await waitFor(() => expect(orchestratorApi.get).toHaveBeenCalledTimes(1));
  const newer = view({ context: { ...context(), digest: "newer-approved-context" } });
  ui.rerender(<CoordinatorPolicy {...input} view={newer} />);
  await act(async () => held.resolve(view()));
  expect(input.onObserved).not.toHaveBeenCalled();
});

it("settles an exact uncertain amendment without publishing its old read over newer parent context", async () => {
  vi.mocked(coordinatorPolicyApi.get).mockResolvedValue(policy({ mode: "auto-drafts" }));
  const input = props(); input.onAdvance.mockRejectedValue(new Error("Reply lost"));
  const ui = render(<CoordinatorPolicy {...input} />); await open(); await screen.findByText(/Reply lost/);
  const held = deferred<OrchestratorView>(); vi.mocked(orchestratorApi.get).mockReturnValue(held.promise);
  await userEvent.click(screen.getByRole("button", { name: "Refresh policy and conversation" }));
  await waitFor(() => expect(orchestratorApi.get).toHaveBeenCalledTimes(1));
  ui.rerender(<CoordinatorPolicy {...input} view={view({ context: { ...context(), digest: "newer-context" } })} />);
  await act(async () => held.resolve(savedView()));
  await screen.findByText(/retained amendment was saved/);
  expect(records()).toHaveLength(0); expect(input.onObserved).not.toHaveBeenCalled();
  expect(input.onAdvance).toHaveBeenCalledTimes(1);
});
it("asks the server to rebase the latest own proposal with the current context CAS despite a later peer turn", async () => {
  vi.mocked(coordinatorPolicyApi.get).mockResolvedValue(policy({ mode: "auto-drafts" }));
  const input = props();
  const own = proposalTurn(), peer = { ...proposalTurn(), id: "peer-turn", sequence: 2, actor: { id: "bob", displayName: "Bob" } };
  input.view = view({ revision: 7, context: { ...context(), digest: "current-team-context" }, turns: [own, peer] });
  render(<CoordinatorPolicy {...input} />);
  await waitFor(() => expect(input.onAdvance).toHaveBeenCalledExactlyOnceWith(own.id, { expectedRevision: 7, expectedContextDigest: "current-team-context", expectedPolicyRevision: 1 }));
  await waitFor(() => expect(records()).toHaveLength(0));
});
it("retains a conflicting old-base proposal without repeated automatic attempts or invented compatibility", async () => {
  vi.mocked(coordinatorPolicyApi.get).mockResolvedValue(policy({ mode: "auto-drafts" }));
  const input = props(); input.view = view({ revision: 7, context: { ...context(), digest: "changed-team-context" }, turns: [proposalTurn()] });
  input.onAdvance.mockRejectedValue(new ProductApiError("conflict", "Contradictory team intent", 409, { error: "coordination_conflict" }));
  render(<CoordinatorPolicy {...input} />); await open(); await screen.findByText(/Contradictory team intent/);
  expect(records()).toHaveLength(1);
  vi.mocked(orchestratorApi.get).mockResolvedValue(input.view);
  await userEvent.click(screen.getByRole("button", { name: "Refresh policy and conversation" }));
  await waitFor(() => expect(records()).toHaveLength(0));
  expect(input.onAdvance).toHaveBeenCalledTimes(1); expect(screen.queryByText(/retained amendment was saved/)).toBeNull();
});
