import { beforeEach, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { ApiError } from "../../api/client";
import { projectBranchesApi, type BranchCatalogue, type BranchResult } from "../../api/projectBranches";
import { resetCockpit, useCockpit } from "../../store/store";
import { aBranch, aWorktree } from "../../test/fixtures";
import { ProjectBranchManager } from "./BranchManager";

vi.mock("../../api/projectBranches", () => ({ projectBranchesApi: { catalogue: vi.fn(), create: vi.fn(), switch: vi.fn(), operation: vi.fn() } }));
const commit = "a".repeat(40), targetCommit = "b".repeat(40);
const main = aBranch({ id: "b-main", projectId: "owned", name: "main", isHead: true });
const feature = aBranch({ id: "b-feature", projectId: "owned", name: "feature/work", isHead: false });
const catalog: BranchCatalogue = { projectId: "owned", currentBranchId: main.id, currentBranch: "main", commit, clean: true, canCreate: true, canSwitch: true, message: "Ready to change branches.", branches: [{ ...main, commit, checkedOut: true, canSwitch: false, message: "Already open." }, { ...feature, commit: targetCommit, checkedOut: false, canSwitch: true, message: "" }] };
const target = { projectId: "owned", action: "create" as const };
beforeEach(() => { resetCockpit(); vi.resetAllMocks(); vi.mocked(projectBranchesApi.catalogue).mockResolvedValue(structuredClone(catalog)); useCockpit.setState({ selection: { projectId: "owned", branchId: main.id, worktreeId: "primary", sessionId: null } }); });
function result(operationId: string, action: "create" | "switch", status: "succeeded" | "failed" | "uncertain" = "succeeded"): BranchResult {
  return { operation: { id: operationId, projectId: "owned", action, status, branchId: feature.id, branch: feature.name, sourceBranchId: main.id, sourceCommit: commit, targetCommit: action === "create" ? commit : targetCommit, commit: action === "create" ? commit : targetCommit, message: status === "failed" ? "The branch did not change." : "Checked out.", createdAt: "now", updatedAt: "now" }, branch: { ...feature, isHead: true }, worktree: aWorktree({ id: "primary", projectId: "owned", branchId: feature.id, isPrimary: true, sessionCount: 0, ready: false }), commit: action === "create" ? commit : targetCommit };
}
function opened(currentCommit = commit) { return { ...catalog, currentBranchId: feature.id, currentBranch: feature.name, commit: currentCommit, branches: [{ ...catalog.branches[0], isHead: false, checkedOut: false, canSwitch: true }, { ...catalog.branches[1], commit: currentCommit, isHead: true, checkedOut: true, canSwitch: false }] }; }

it("creates once from the inspected source and refreshes the branch-scoped workspace", async () => {
  let finish!: (value: BranchResult) => void;
  vi.mocked(projectBranchesApi.create).mockImplementation(() => new Promise(resolve => { finish = resolve; }));
  render(<ProjectBranchManager target={target} />);
  await screen.findByText(/Ready to change/);
  fireEvent.change(screen.getByLabelText(/Branch name/), { target: { value: feature.name } });
  // The existing feature would be rejected: this scenario starts with only main.
  vi.mocked(projectBranchesApi.catalogue).mockResolvedValueOnce({ ...catalog, branches: [catalog.branches[0]] });
  fireEvent.click(screen.getByRole("button", { name: "Refresh branches" }));
  await waitFor(() => expect((screen.getByRole("button", { name: "Create and open branch" }) as HTMLButtonElement).disabled).toBe(false));
  fireEvent.click(screen.getByRole("button", { name: "Create and open branch" })); fireEvent.click(screen.getByRole("button", { name: "Create and open branch" }));
  expect(projectBranchesApi.create).toHaveBeenCalledTimes(1);
  const input = vi.mocked(projectBranchesApi.create).mock.calls[0][1];
  expect(input).toMatchObject({ name: feature.name, expectedBranchId: main.id, expectedCommit: commit });
  expect(input.operationId).toMatch(/^branch-/);
  expect((screen.getByRole("button", { name: "Cancel" }) as HTMLButtonElement).disabled).toBe(true);
  vi.mocked(projectBranchesApi.catalogue).mockResolvedValueOnce(opened());
  await act(async () => finish(result(input.operationId, "create")));
  expect(useCockpit.getState().selection).toMatchObject({ branchId: feature.id, worktreeId: "primary", sessionId: null });
  expect(useCockpit.getState().projectSourceRevisions.owned).toBe(1);
});
it("switches using the inspected target commit and verifies the new checkout", async () => {
  vi.mocked(projectBranchesApi.switch).mockImplementation(async (_id, input) => { vi.mocked(projectBranchesApi.catalogue).mockResolvedValueOnce(opened(targetCommit)); return result(input.operationId, "switch"); });
  render(<ProjectBranchManager target={{ ...target, action: "switch", branchId: feature.id }} />);
  await screen.findByText(/Ready to change/); fireEvent.click(screen.getByRole("button", { name: "Switch branch" }));
  await waitFor(() => expect(useCockpit.getState().selection.branchId).toBe(feature.id));
  expect(vi.mocked(projectBranchesApi.switch).mock.calls[0][1]).toMatchObject({ branchId: feature.id, expectedTargetCommit: targetCommit, expectedBranchId: main.id, expectedCommit: commit });
});
it("blocks editor drafts and dirty repositories with a path back to the work", async () => {
  useCockpit.getState().setUnsavedProjectDraft("editor", { projectId: "owned", label: "Definition", lens: "discover" });
  vi.mocked(projectBranchesApi.catalogue).mockResolvedValue({ ...catalog, clean: false, canCreate: false, canSwitch: false, message: "Commit your changes first." });
  render(<ProjectBranchManager target={target} />); await screen.findByText(/Commit your changes first/);
  fireEvent.change(screen.getByLabelText(/Branch name/), { target: { value: "new-work" } });
  fireEvent.click(screen.getByRole("button", { name: "Create and open branch" }));
  expect(projectBranchesApi.create).not.toHaveBeenCalled(); expect(screen.getByText(/Save your Definition edits/)).not.toBeNull();
  expect((screen.getByRole("button", { name: "Open Git to commit changes" }) as HTMLButtonElement).disabled).toBe(true);
});
it.each([true, false])("keeps branch changes protected by local verification setup with an actionable retention remedy (retained=%s)", async retained => {
  const discard = vi.fn();
  useCockpit.getState().setUnsavedProjectDraft("checks", { projectId: "owned", label: "Verification setup", lens: "develop", purpose: "verification-setup", retained, discard });
  render(<ProjectBranchManager target={target} />); await screen.findByText(/Ready to change/);
  fireEvent.change(screen.getByLabelText(/Branch name/), { target: { value: "new-work" } });
  expect((screen.getByRole("button", { name: "Create and open branch" }) as HTMLButtonElement).disabled).toBe(true);
  expect(screen.getByText(retained ? /Close this dialog, change phase and choose Keep drafts and continue/ : /Keep editing to copy the command text you need/)).not.toBeNull();
  expect(screen.queryByText(/Save your Verification setup/)).toBeNull();
  fireEvent.click(screen.getByRole("button", { name: "Keep editing" }));
  expect(projectBranchesApi.create).not.toHaveBeenCalled(); expect(discard).not.toHaveBeenCalled();
  expect(useCockpit.getState().unsavedProjectDrafts.checks).toBeDefined();
});
it("retains the name after a stale-source refusal and permits an explicit refresh", async () => {
  vi.mocked(projectBranchesApi.create).mockRejectedValue(new ApiError("conflict", "Source changed. Refresh Git.", 409));
  render(<ProjectBranchManager target={target} />); await screen.findByText(/Ready to change/);
  fireEvent.change(screen.getByLabelText(/Branch name/), { target: { value: "new-work" } }); fireEvent.click(screen.getByRole("button", { name: "Create and open branch" }));
  expect((await screen.findByRole("alert")).textContent).toContain("Source changed");
  expect((screen.getByLabelText(/Branch name/) as HTMLInputElement).value).toBe("new-work");
  expect((screen.getByLabelText(/Branch name/) as HTMLInputElement).disabled).toBe(false); expect(projectBranchesApi.create).toHaveBeenCalledTimes(1);
});
it("reconciles a lost reply by reading the exact operation without repeating checkout", async () => {
  vi.mocked(projectBranchesApi.create).mockRejectedValue(new ApiError("timeout", "The reply was lost.", 0));
  render(<ProjectBranchManager target={target} />); await screen.findByText(/Ready to change/);
  fireEvent.change(screen.getByLabelText(/Branch name/), { target: { value: feature.name + "-new" } }); fireEvent.click(screen.getByRole("button", { name: "Create and open branch" }));
  await screen.findByRole("alert");
  const input = vi.mocked(projectBranchesApi.create).mock.calls[0][1];
  const recovered = result(input.operationId, "create"); recovered.branch!.name = input.name; recovered.operation.branch = input.name;
  vi.mocked(projectBranchesApi.operation).mockResolvedValue(recovered); const observed = opened(); observed.branches[1].name = input.name; vi.mocked(projectBranchesApi.catalogue).mockResolvedValueOnce(observed);
  fireEvent.click(screen.getByRole("button", { name: "Check branch state" }));
  await waitFor(() => expect(useCockpit.getState().selection.branchId).toBe(feature.id));
  expect(projectBranchesApi.operation).toHaveBeenCalledExactlyOnceWith("owned", input.operationId); expect(projectBranchesApi.create).toHaveBeenCalledTimes(1);
});
it("rejects mismatched provenance before adopting source", async () => {
  vi.mocked(projectBranchesApi.switch).mockImplementation(async (_id, input) => { const wrong = result(input.operationId, "switch"); wrong.operation.sourceCommit = "c".repeat(40); return wrong; });
  render(<ProjectBranchManager target={{ ...target, action: "switch", branchId: feature.id }} />); await screen.findByText(/Ready to change/);
  fireEvent.click(screen.getByRole("button", { name: "Switch branch" }));
  expect((await screen.findByRole("alert")).textContent).toContain("does not match"); expect(useCockpit.getState().selection.branchId).toBe(main.id);
});
it("rejects a checkout changed before the final catalogue readback", async () => {
  vi.mocked(projectBranchesApi.switch).mockImplementation(async (_id, input) => result(input.operationId, "switch"));
  render(<ProjectBranchManager target={{ ...target, action: "switch", branchId: feature.id }} />); await screen.findByText(/Ready to change/);
  fireEvent.click(screen.getByRole("button", { name: "Switch branch" }));
  expect((await screen.findByRole("alert")).textContent).toContain("changed after checkout"); expect(useCockpit.getState().selection.branchId).toBe(main.id);
  expect(projectBranchesApi.switch).toHaveBeenCalledTimes(1);
});
it("ignores a catalogue delivered after this project was unmounted", async () => {
  let finish!: (value: BranchCatalogue) => void;
  vi.mocked(projectBranchesApi.catalogue).mockImplementation(() => new Promise(resolve => { finish = resolve; }));
  const view = render(<ProjectBranchManager target={target} />); view.unmount();
  await act(async () => finish({ ...catalog, projectId: "another" }));
  expect(projectBranchesApi.create).not.toHaveBeenCalled(); expect(useCockpit.getState().projectSourceRevisions.owned).toBeUndefined();
});
