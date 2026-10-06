import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DeploymentWorkspace, ProjectDeploy } from "../lens/DeployLens";
import { productApi, type Release } from "../../api/product";
import { platformApi } from "../../api/platform";
import { resetCockpit } from "../../store/store";
import { notifyProjectContextChanged } from "../../api/projectContextEvents";

const identity = vi.hoisted(() => ({ scope: "account-team-workspace-project", edit: true }));
vi.mock("../../hooks/useProjectDraft", () => ({ useProjectDraftScope: () => identity.scope }));
vi.mock("../../hooks/useProjectAccess", () => ({ useProjectAccess: () => ({ loading: false, error: null, message: "", canRead: true, canEdit: identity.edit, canExecute: identity.edit, permissions: { read: true, edit: identity.edit, execute: identity.edit, deploy: identity.edit, secrets: false } }) }));
vi.mock("../platform/PlatformWorkspace", () => ({ PlatformWorkspace: ({ inspectDeploymentId }: { inspectDeploymentId?: string }) => <div>Persistent application controls {inspectDeploymentId}</div> }));
vi.mock("./ProjectConfiguration", () => ({ ProjectConfiguration: () => null }));
const release: Release = { id: "retained-preview", projectId: "p-1", briefRevision: 1, commit: "a".repeat(40), target: "local", status: "stopped", canStop: false, url: "", message: "Earlier preview retained.", createdAt: "2026-10-06T12:00:00Z" };
const nextCommit = "b".repeat(40);
beforeEach(() => {
  resetCockpit(); identity.scope = "account-team-workspace-project"; identity.edit = true;
  vi.spyOn(productApi, "releases").mockResolvedValue({ releases: [release] });
  vi.spyOn(productApi, "gitDelivery").mockResolvedValue({ head: nextCommit, branch: "main" });
  vi.spyOn(productApi, "git").mockResolvedValue({ branch: "main", clean: true, changes: [], diff: "", log: [] });
  vi.spyOn(productApi, "release").mockResolvedValue({ ...release, id: "new-preview", commit: nextCommit, status: "building" });
  vi.spyOn(productApi, "stopRelease").mockResolvedValue({ ...release, status: "stopped" });
  vi.spyOn(productApi, "refreshRelease").mockResolvedValue(release);
  vi.spyOn(platformApi, "project").mockResolvedValue({ project: { projectId: "p-1", deployments: [], backups: [], database: { status: "not-created", engine: "postgres", message: "" } }, busy: false });
});
afterEach(() => vi.restoreAllMocks());

it("finds a retained preview on returning to Deploy and opens its exact history row without starting or verifying work", async () => {
  render(<DeploymentWorkspace projectId="p-1" />);
  expect(screen.getByText(/Persistent application controls/)).not.toBeNull();
  await userEvent.click(await screen.findByRole("button", { name: "Inspect workspace preview" }));
  expect(screen.getByRole("tab", { name: "Previews and publishing" }).getAttribute("aria-selected")).toBe("true");
  const row = await screen.findByRole("listitem", { name: "Release retained-preview" });
  await waitFor(() => expect(document.activeElement).toBe(row));
  expect(within(row).getByText("Earlier preview retained.")).not.toBeNull();
  expect(productApi.release).not.toHaveBeenCalled(); expect(productApi.stopRelease).not.toHaveBeenCalled(); expect(productApi.refreshRelease).not.toHaveBeenCalled();
});

it("keeps available history usable when the other service is unavailable and scopes event refresh", async () => {
  vi.mocked(platformApi.project).mockRejectedValue(new Error("Workspace platform is temporarily unavailable"));
  render(<DeploymentWorkspace projectId="p-1" />);
  expect(await screen.findByRole("button", { name: "Inspect workspace preview" })).not.toBeNull();
  expect(screen.getByRole("alert").textContent).toContain("Persistent application history:");
  const count = vi.mocked(productApi.releases).mock.calls.length;
  act(() => notifyProjectContextChanged("other-workspace", "p-1"));
  expect(productApi.releases).toHaveBeenCalledTimes(count);
  act(() => notifyProjectContextChanged(identity.scope, "p-1"));
  await waitFor(() => expect(productApi.releases).toHaveBeenCalledTimes(count + 1));
});

it("discards delayed retained releases from another workspace", async () => {
  let finish!: (value: { releases: Release[] }) => void;
  vi.mocked(productApi.releases).mockImplementationOnce(() => new Promise(resolve => { finish = resolve; })).mockResolvedValue({ releases: [] });
  const view = render(<DeploymentWorkspace projectId="p-1" />);
  identity.scope = "new-workspace"; view.rerender(<DeploymentWorkspace projectId="p-1" />);
  await screen.findByText(/No application has been released/);
  await act(async () => finish({ releases: [release] }));
  expect(screen.queryByRole("button", { name: "Inspect workspace preview" })).toBeNull();
});

it("reports an unavailable selected release instead of silently choosing another", async () => {
  render(<ProjectDeploy projectId="p-1" inspectReleaseId="missing-preview" />);
  expect((await screen.findByRole("alert")).textContent).toContain("selected release is no longer");
  expect(document.activeElement?.getAttribute("aria-label")).not.toBe("Release retained-preview");
  expect(productApi.release).not.toHaveBeenCalled();
});

it("requires source review and acknowledgement before building a replacement at the captured commit", async () => {
  render(<ProjectDeploy projectId="p-1" />);
  await userEvent.click(await screen.findByRole("button", { name: "Prepare replacement preview" }));
  const dialog = await screen.findByRole("dialog", { name: "Review replacement preview" });
  expect(dialog.textContent).toContain(release.commit); expect(dialog.textContent).toContain(nextCommit);
  expect(dialog.textContent).toContain("checked and reviewed again");
  const build = within(dialog).getByRole("button", { name: "Build replacement preview" });
  expect((build as HTMLButtonElement).disabled).toBe(true); expect(productApi.release).not.toHaveBeenCalled();
  await userEvent.click(within(dialog).getByRole("checkbox"));
  await userEvent.click(build);
  await waitFor(() => expect(productApi.release).toHaveBeenCalledExactlyOnceWith("p-1", 3000, "Dockerfile", nextCommit));
  expect(productApi.stopRelease).not.toHaveBeenCalled();
  expect(await screen.findByText("Earlier preview retained.")).not.toBeNull();
});

it("preserves replacement review when the server rejects changed source and never retries implicitly", async () => {
  vi.mocked(productApi.release).mockRejectedValue(new Error("The checked source changed. Review and verify the new commit."));
  render(<ProjectDeploy projectId="p-1" />);
  await userEvent.click(await screen.findByRole("button", { name: "Prepare replacement preview" }));
  const dialog = await screen.findByRole("dialog", { name: "Review replacement preview" });
  fireEvent.click(within(dialog).getByRole("checkbox"));
  fireEvent.click(within(dialog).getByRole("button", { name: "Build replacement preview" }));
  expect((await within(dialog).findByRole("alert")).textContent).toContain("source changed");
  expect(productApi.release).toHaveBeenCalledTimes(1); expect(productApi.stopRelease).not.toHaveBeenCalled();
});

it("refuses uncommitted replacement source and exposes only server-authorized failed container cleanup", async () => {
  vi.mocked(productApi.releases).mockResolvedValue({ releases: [{ ...release, status: "failed", canStop: true }] });
  vi.mocked(productApi.git).mockResolvedValue({ branch: "main", clean: false, changes: [{ path: "app.js", status: "M" }], diff: "changed", log: [] });
  render(<ProjectDeploy projectId="p-1" />);
  const prepare = await screen.findByRole("button", { name: "Prepare replacement preview" });
  expect((prepare as HTMLButtonElement).disabled).toBe(true);
  await userEvent.click(screen.getByRole("button", { name: "Stop preview" }));
  expect(productApi.stopRelease).toHaveBeenCalledExactlyOnceWith("p-1", release.id);
  vi.mocked(productApi.releases).mockResolvedValue({ releases: [release] });
  await userEvent.click(screen.getByRole("button", { name: "Refresh" }));
  await userEvent.click(screen.getByRole("button", { name: "Prepare replacement preview" }));
  expect((await screen.findByRole("alert")).textContent).toContain("Review and commit");
  expect(screen.queryByRole("dialog")).toBeNull(); expect(productApi.release).not.toHaveBeenCalled();
});

it("lets viewers inspect retained history while keeping recovery disabled", async () => {
  identity.edit = false;
  render(<DeploymentWorkspace projectId="p-1" />);
  await userEvent.click(await screen.findByRole("button", { name: "Inspect workspace preview" }));
  const prepare = await screen.findByRole("button", { name: "Prepare replacement preview" });
  expect((prepare as HTMLButtonElement).disabled).toBe(true);
  expect(screen.getByRole("button", { name: "Refresh" }).hasAttribute("disabled")).toBe(false);
  expect(productApi.release).not.toHaveBeenCalled(); expect(productApi.stopRelease).not.toHaveBeenCalled();
});

it("does not infer cleanup eligibility when the server explicitly reports no owned retained handle", async () => {
  vi.mocked(productApi.releases).mockResolvedValue({ releases: [{ ...release, status: "unknown", canStop: false }] });
  render(<ProjectDeploy projectId="p-1" />);
  await screen.findByText("Earlier preview retained.");
  expect(screen.queryByRole("button", { name: "Stop preview" })).toBeNull();
  expect(productApi.stopRelease).not.toHaveBeenCalled();
});
