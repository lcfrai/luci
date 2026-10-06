import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DeploymentWorkspace, ProjectDeploy } from "../lens/DeployLens";
import { productApi, type Release } from "../../api/product";
import { projectRunsApi, type ProjectRun } from "../../api/projectRuns";
import { resetCockpit, useCockpit } from "../../store/store";
import type { ProjectRunDeploymentIntent } from "../../store/slices/uiSlice";

const identity = vi.hoisted(() => ({ scope: "account-team-workspace-project", edit: true, execute: true, deploy: true, secrets: true }));
vi.mock("../../hooks/useProjectDraft", () => ({ useProjectDraftScope: () => identity.scope }));
vi.mock("../../hooks/useProjectAccess", () => ({ useProjectAccess: () => ({
  loading: false, error: null, message: "", canRead: true, canEdit: identity.edit, canExecute: identity.edit && identity.execute,
  permissions: { read: true, edit: identity.edit, execute: identity.execute, deploy: identity.deploy, secrets: identity.secrets },
}) }));
vi.mock("./DeploymentOverview", () => ({ DeploymentOverview: () => null }));
vi.mock("../platform/PlatformWorkspace", () => ({ PlatformWorkspace: () => <div>Application platform controls</div> }));
vi.mock("./ProjectConfiguration", () => ({ ProjectConfiguration: () => <div>Runtime configuration controls</div> }));
const intent: ProjectRunDeploymentIntent = { scope: "account-team-workspace-project", projectId: "p-1", runId: "run-exact", checkId: "check-exact", commit: "a".repeat(40), sourceDigest: "b".repeat(64), outcome: "preview" };
const saved = { id: intent.runId, projectId: intent.projectId, revision: 4, state: "awaiting-release", input: { outcome: "preview" }, checks: [{ checkId: intent.checkId, projectId: intent.projectId, commit: intent.commit, sourceDigest: intent.sourceDigest }], allowedActions: ["join-release"] } as ProjectRun;
const release: Release = { id: "rel-exact", projectId: "p-1", briefRevision: 1, commit: intent.commit, target: "local", status: "ready", url: "http://127.0.0.1:45678/", message: "Preview health passed", createdAt: "2026-10-06T12:00:00Z" };
beforeEach(() => {
  resetCockpit(); Object.assign(identity, { scope: intent.scope, edit: true, execute: true, deploy: true, secrets: true });
  useCockpit.setState({ selection: { projectId: "p-1", branchId: null, worktreeId: null, sessionId: null }, lens: "deploy" });
  vi.spyOn(projectRunsApi, "get").mockResolvedValue(saved);
  vi.spyOn(projectRunsApi, "joinRelease").mockResolvedValue({ ...saved, revision: 5, allowedActions: ["accept"] });
  vi.spyOn(projectRunsApi, "accept").mockResolvedValue(saved);
  vi.spyOn(productApi, "gitDelivery").mockResolvedValue({ head: intent.commit, branch: "main" });
  vi.spyOn(productApi, "git").mockResolvedValue({ branch: "main", clean: true, changes: [], diff: "", log: [] });
  vi.spyOn(productApi, "releases").mockResolvedValue({ releases: [] });
  vi.spyOn(productApi, "release").mockResolvedValue(release);
  vi.spyOn(productApi, "stopRelease").mockResolvedValue({ ...release, status: "stopped" });
  vi.spyOn(productApi, "refreshRelease").mockResolvedValue(release);
  vi.spyOn(productApi, "rollbackRelease").mockResolvedValue(release);
});
afterEach(() => { vi.restoreAllMocks(); vi.useRealTimers(); });

it("opens the checked preview destination without starting work and fences the explicit build to its commit", async () => {
  useCockpit.setState({ projectRunDeployment: { ...intent } });
  render(<DeploymentWorkspace projectId="p-1" />);
  expect(screen.getByRole("tab", { name: "Previews and publishing" }).getAttribute("aria-selected")).toBe("true");
  expect(screen.queryByText("Application platform controls")).toBeNull();
  expect(screen.getByRole("region", { name: "Checked build deployment" }).textContent).toContain(intent.commit);
  await waitFor(() => expect((screen.getByRole("button", { name: "Build local preview" }) as HTMLButtonElement).disabled).toBe(false));
  expect(projectRunsApi.get).toHaveBeenCalledExactlyOnceWith("p-1", "run-exact");
  expect(productApi.release).not.toHaveBeenCalled(); expect(projectRunsApi.joinRelease).not.toHaveBeenCalled();
  expect(useCockpit.getState().projectRunDeployment).toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "Build local preview" }));
  await waitFor(() => expect(productApi.release).toHaveBeenCalledExactlyOnceWith("p-1", 3000, "Dockerfile", intent.commit));
  expect(projectRunsApi.get).toHaveBeenCalledTimes(2);
  expect(projectRunsApi.accept).not.toHaveBeenCalled();
});

it("explicitly attaches a matching ready release and returns to the exact refreshed run without accepting it", async () => {
  useCockpit.setState({ projectRunDeployment: { ...intent } });
  vi.mocked(productApi.releases).mockResolvedValue({ releases: [release] });
  render(<DeploymentWorkspace projectId="p-1" />);
  const attach = await screen.findByRole("button", { name: "Attach release to build results" });
  await waitFor(() => expect((attach as HTMLButtonElement).disabled).toBe(false));
  expect(projectRunsApi.joinRelease).not.toHaveBeenCalled();
  await userEvent.click(attach);
  await waitFor(() => expect(useCockpit.getState().lens).toBe("develop"));
  expect(projectRunsApi.joinRelease).toHaveBeenCalledExactlyOnceWith("p-1", "run-exact", 4, "rel-exact", "local");
  expect(useCockpit.getState().projectRunInspection).toEqual({ scope: intent.scope, projectId: "p-1", runId: "run-exact", refresh: true });
  expect(projectRunsApi.accept).not.toHaveBeenCalled();
});

it("refuses changed source at explicit build time and preserves an exact read-only return", async () => {
  useCockpit.setState({ projectRunDeployment: { ...intent } });
  vi.mocked(productApi.gitDelivery).mockResolvedValueOnce({ head: intent.commit, branch: "main" }).mockResolvedValue({ head: "c".repeat(40), branch: "main" });
  render(<DeploymentWorkspace projectId="p-1" />);
  await waitFor(() => expect((screen.getByRole("button", { name: "Build local preview" }) as HTMLButtonElement).disabled).toBe(false));
  await userEvent.click(screen.getByRole("button", { name: "Build local preview" }));
  await waitFor(() => expect(screen.getAllByRole("alert").some(node => node.textContent?.includes("source changed"))).toBe(true));
  expect(productApi.release).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole("button", { name: "Return to build results" }));
  expect(useCockpit.getState().projectRunInspection?.runId).toBe("run-exact");
  expect(projectRunsApi.joinRelease).not.toHaveBeenCalled();
});

it.each([{ scope: "other-account" }, { projectId: "other-project" }])("ignores foreign deployment intent %j", async mismatch => {
  useCockpit.setState({ projectRunDeployment: { ...intent, ...mismatch } });
  render(<DeploymentWorkspace projectId="p-1" />);
  expect(screen.getByText("Application platform controls")).not.toBeNull();
  expect(screen.queryByRole("region", { name: "Checked build deployment" })).toBeNull();
  await act(async () => {});
  expect(projectRunsApi.get).not.toHaveBeenCalled(); expect(productApi.release).not.toHaveBeenCalled();
});

it("fences a delayed checked-source response after changing account or workspace", async () => {
  let resolve!: (value: ProjectRun) => void;
  vi.mocked(projectRunsApi.get).mockImplementationOnce(() => new Promise(done => { resolve = done; }));
  useCockpit.setState({ projectRunDeployment: { ...intent } });
  const view = render(<DeploymentWorkspace projectId="p-1" />);
  identity.scope = "different-workspace";
  view.rerender(<DeploymentWorkspace projectId="p-1" />);
  await act(async () => resolve(saved));
  expect(screen.queryByRole("region", { name: "Checked build deployment" })).toBeNull();
  expect(screen.getByText("Application platform controls")).not.toBeNull();
  expect(productApi.release).not.toHaveBeenCalled();
});

it("does not navigate another workspace when an earlier release attachment finishes", async () => {
  let resolve!: (value: ProjectRun) => void;
  vi.mocked(projectRunsApi.joinRelease).mockImplementationOnce(() => new Promise(done => { resolve = done; }));
  vi.mocked(productApi.releases).mockResolvedValue({ releases: [release] });
  useCockpit.setState({ projectRunDeployment: { ...intent } });
  // This test controls the attachment response, not initial loading. Flush the
  // resolved source/history fixture before starting that race so a busy suite
  // does not spend its role-query timeout waiting for the initial microtasks.
  let view!: ReturnType<typeof render>;
  await act(async () => { view = render(<DeploymentWorkspace projectId="p-1" />); });
  expect(screen.getByRole("region", { name: "Checked build deployment" }).textContent).toContain(intent.commit);
  expect(screen.getByText("Preview health passed")).not.toBeNull();
  const attach = screen.getByRole("button", { name: "Attach release to build results" });
  expect((attach as HTMLButtonElement).disabled).toBe(false);
  await act(async () => { await userEvent.click(attach); });
  expect(projectRunsApi.joinRelease).toHaveBeenCalledExactlyOnceWith("p-1", "run-exact", 4, "rel-exact", "local");
  identity.scope = "other-workspace";
  view.rerender(<DeploymentWorkspace projectId="p-1" />);
  await act(async () => resolve({ ...saved, revision: 5 }));
  expect(useCockpit.getState().lens).toBe("deploy");
  expect(useCockpit.getState().projectRunInspection).toBeNull();
  expect(screen.queryByRole("region", { name: "Checked build deployment" })).toBeNull();
});

it.each([{ edit: false, deploy: false }, { edit: true, deploy: false }])("keeps release reads available but explains denied deployment mutations %j", async grants => {
  Object.assign(identity, grants);
  vi.mocked(productApi.releases).mockResolvedValue({ releases: [release, { ...release, id: "cloud", target: "cloud" }] });
  render(<ProjectDeploy projectId="p-1" />);
  await screen.findByText("Cloud deployment");
  for (const name of ["Build local preview", "Stop preview", "Check deployment health", "Stop cloud application", "Roll back to this release"]) {
    const button = screen.getByRole("button", { name }) as HTMLButtonElement;
    expect(button.disabled).toBe(true); fireEvent.click(button);
  }
  expect(screen.getByText(/Ask a team owner for editing and deployment access/)).not.toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "Refresh" }));
  expect(productApi.releases).toHaveBeenCalledTimes(2);
  expect(productApi.release).not.toHaveBeenCalled(); expect(productApi.stopRelease).not.toHaveBeenCalled(); expect(productApi.refreshRelease).not.toHaveBeenCalled(); expect(productApi.rollbackRelease).not.toHaveBeenCalled();
});

it("keeps deploy permission distinct from private browser execution", async () => {
  identity.execute = false;
  vi.mocked(productApi.releases).mockResolvedValue({ releases: [release] });
  render(<ProjectDeploy projectId="p-1" />);
  const preview = await screen.findByRole("button", { name: "Open preview" });
  expect((preview as HTMLButtonElement).disabled).toBe(true);
  expect((screen.getByRole("button", { name: "Build local preview" }) as HTMLButtonElement).disabled).toBe(false);
  expect((screen.getByRole("button", { name: "Stop preview" }) as HTMLButtonElement).disabled).toBe(false);
  expect(screen.getByText(/execution access to open/)).not.toBeNull();
});

it("never performs background health mutations for a viewer", async () => {
  identity.edit = false; identity.deploy = false;
  vi.useFakeTimers({ toFake: ["setInterval", "clearInterval"] });
  vi.mocked(productApi.releases).mockResolvedValue({ releases: [{ ...release, target: "cloud", status: "building" }] });
  render(<ProjectDeploy projectId="p-1" />);
  await act(async () => { await Promise.resolve(); });
  await act(async () => { await vi.advanceTimersByTimeAsync(15000); });
  expect(productApi.refreshRelease).not.toHaveBeenCalled();
});
