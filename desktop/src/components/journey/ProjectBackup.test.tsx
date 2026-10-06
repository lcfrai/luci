import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { productApi } from "../../api/product";
import { ProjectBackup, ProjectRestore } from "./ProjectBackup";

const identity = vi.hoisted(() => ({ actor: "alice", workspace: "local" }));
const access = vi.hoisted(() => ({ loading: false, error: "", permissions: { read: true, edit: true, execute: false, secrets: false, deploy: false, admin: false } }));
vi.mock("../auth/AuthProvider", () => ({ useAuth: () => ({ profile: { user: { id: identity.actor } } }) }));
vi.mock("../setup/SetupProvider", () => ({ useSetup: () => ({ workspace: { mode: "local", connectionId: identity.workspace, host: "" }, status: { config: { projectsDir: "/projects" }, access: { user: { username: identity.actor } } } }) }));
vi.mock("../teams/TeamWorkspaceProvider", () => ({ useTeamProjectPermissions: () => access, useTeamWorkspace: () => null }));
let download: ReturnType<typeof vi.spyOn>;
beforeEach(() => {
  identity.actor = "alice"; identity.workspace = "local"; access.loading = false; access.error = "";
  Object.assign(access.permissions, { read: true, edit: true, execute: false, secrets: false, deploy: false, admin: false });
  vi.spyOn(productApi, "backupProject").mockResolvedValue(new Blob(["private source"], { type: "application/zip" }));
  vi.stubGlobal("URL", class extends URL { static createObjectURL = vi.fn(() => "blob:fixture"); static revokeObjectURL = vi.fn(); });
  download = vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => {});
});
afterEach(() => { cleanup(); vi.restoreAllMocks(); vi.unstubAllGlobals(); });
const start = () => userEvent.click(screen.getByRole("button", { name: "Download project backup" }));

it("exports source with editing permission, without requiring execution, deployment or secrets grants", async () => {
  render(<ProjectBackup projectId="p-1" />); await start();
  await waitFor(() => expect(download).toHaveBeenCalledOnce()); expect(productApi.backupProject).toHaveBeenCalledExactlyOnceWith("p-1");
});
it("refuses viewer source export even if execution and secrets grants are set", async () => {
  access.permissions.edit = false; access.permissions.execute = true; access.permissions.secrets = true;
  render(<ProjectBackup projectId="p-1" />); await start();
  expect((screen.getByRole("button", { name: "Download project backup" }) as HTMLButtonElement).disabled).toBe(true);
  expect(productApi.backupProject).not.toHaveBeenCalled(); expect(download).not.toHaveBeenCalled();
});
it.each(["edit", "read"] as const)("does not revive a private download after %s access is revoked and restored", async permission => {
  let finish!: (value: Blob) => void;
  vi.mocked(productApi.backupProject).mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  const view = render(<ProjectBackup projectId="p-1" />); await start();
  access.permissions[permission] = false; view.rerender(<ProjectBackup projectId="p-1" />);
  access.permissions[permission] = true; view.rerender(<ProjectBackup projectId="p-1" />);
  await act(async () => finish(new Blob(["old private source"])));
  expect(download).not.toHaveBeenCalled(); expect(URL.createObjectURL).not.toHaveBeenCalled();
});
it.each(["unmount", "project", "account", "workspace"])("drops a delayed private archive after %s changes", async change => {
  let finish!: (value: Blob) => void;
  vi.mocked(productApi.backupProject).mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  const view = render(<ProjectBackup projectId="p-1" />); await start();
  if (change === "unmount") view.unmount();
  else { if (change === "account") identity.actor = "bob"; if (change === "workspace") identity.workspace = "other"; view.rerender(<ProjectBackup projectId={change === "project" ? "p-2" : "p-1"} />); }
  await act(async () => finish(new Blob(["old private source"])));
  expect(download).not.toHaveBeenCalled(); expect(URL.createObjectURL).not.toHaveBeenCalled();
});
it("ignores a stale failure and finally while a newer authorized download is pending", async () => {
  let rejectOld!: (cause: Error) => void, finishNew!: (value: Blob) => void;
  vi.mocked(productApi.backupProject).mockImplementationOnce(() => new Promise((_resolve, reject) => { rejectOld = reject; })).mockImplementationOnce(() => new Promise(resolve => { finishNew = resolve; }));
  const view = render(<ProjectBackup projectId="p-1" />); await start();
  access.permissions.edit = false; view.rerender(<ProjectBackup projectId="p-1" />);
  access.permissions.edit = true; view.rerender(<ProjectBackup projectId="p-1" />); await start();
  expect(productApi.backupProject).toHaveBeenCalledTimes(2);
  await act(async () => rejectOld(new Error("Private old request failed")));
  expect(screen.queryByRole("alert")).toBeNull(); expect((screen.getByRole("button", { name: "Preparing backup…" }) as HTMLButtonElement).disabled).toBe(true);
  await act(async () => finishNew(new Blob(["current private source"]))); expect(download).toHaveBeenCalledOnce();
});

it("retains an admitted backup when a permission refresh confirms the same grants before completion", async () => {
  let finish!: (value: Blob) => void;
  vi.mocked(productApi.backupProject).mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  const view = render(<ProjectBackup projectId="p-1" />); await start();
  access.loading = true; view.rerender(<ProjectBackup projectId="p-1" />);
  expect((screen.getByRole("button", { name: "Preparing backup…" }) as HTMLButtonElement).disabled).toBe(true);
  access.loading = false; view.rerender(<ProjectBackup projectId="p-1" />);
  await act(async () => finish(new Blob(["current source"])));
  expect(download).toHaveBeenCalledOnce(); expect(productApi.backupProject).toHaveBeenCalledOnce();
});

it.each(["loading", "error"] as const)("offers an explicit retry without delivering when backup finishes during permission %s", async state => {
  let finish!: (value: Blob) => void;
  vi.mocked(productApi.backupProject).mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  const view = render(<ProjectBackup projectId="p-1" />); await start();
  if (state === "loading") access.loading = true; else access.error = "Refresh unavailable";
  view.rerender(<ProjectBackup projectId="p-1" />);
  await act(async () => finish(new Blob(["private source"])));
  expect(screen.getByText(/Editing access could not be confirmed when this backup finished/)).not.toBeNull();
  expect(download).not.toHaveBeenCalled(); expect(URL.createObjectURL).not.toHaveBeenCalled();
  expect((screen.getByRole("button", { name: "Download project backup" }) as HTMLButtonElement).disabled).toBe(true);
  access.loading = false; access.error = ""; view.rerender(<ProjectBackup projectId="p-1" />);
  expect(download).not.toHaveBeenCalled(); expect((screen.getByRole("button", { name: "Download project backup" }) as HTMLButtonElement).disabled).toBe(false);
  await start(); await waitFor(() => expect(download).toHaveBeenCalledOnce());
  expect(productApi.backupProject).toHaveBeenCalledTimes(2);
});

async function submitRestore() {
  await userEvent.upload(screen.getByLabelText("Luci backup ZIP"), new File(["private archive"], "backup.zip", { type: "application/zip" }));
  await userEvent.type(screen.getByLabelText("Restored project name"), "restored-project");
  await userEvent.click(screen.getByRole("checkbox", { name: "I trust the source of this project archive." }));
  // happy-dom cannot reconcile FileList with native required-file validity.
  fireEvent.submit(screen.getByRole("button", { name: "Restore as new project" }).closest("form")!);
}

it.each(["unmount", "account", "workspace", "client"])("does not navigate after a restore completes following %s change, and releases its parent's busy state once", async change => {
  let finish!: (value: { id: string; name: string; path: string }) => void;
  const restore = vi.spyOn(productApi, "restoreProject").mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  const restored = vi.fn(), busy = vi.fn();
  const view = render(<ProjectRestore client={productApi} onRestored={restored} onBusy={busy} />); await submitRestore();
  expect(restore).toHaveBeenCalledOnce(); expect(busy.mock.calls).toEqual([[true]]);
  if (change === "unmount") view.unmount();
  else {
    if (change === "account") identity.actor = "bob";
    if (change === "workspace") identity.workspace = "another-workspace";
    view.rerender(<ProjectRestore client={change === "client" ? { ...productApi } : productApi} onRestored={restored} onBusy={busy} />);
  }
  expect(busy.mock.calls).toEqual([[true], [false]]);
  await act(async () => finish({ id: "restored", name: "restored-project", path: "/projects/restored-project" }));
  expect(restored).not.toHaveBeenCalled(); expect(busy.mock.calls).toEqual([[true], [false]]);
  // This fences UI callbacks; the admitted server restore was not cancelled or repeated.
  expect(restore).toHaveBeenCalledOnce();
});

it("ignores a detached restore failure and finally while a replacement client's restore is running", async () => {
  let rejectOld!: (cause: Error) => void, finishNew!: (value: { id: string; name: string; path: string }) => void;
  vi.spyOn(productApi, "restoreProject").mockImplementationOnce(() => new Promise((_resolve, reject) => { rejectOld = reject; }));
  const next = { ...productApi, baseUrl: "/another-workspace", restoreProject: vi.fn(() => new Promise<{ id: string; name: string; path: string }>(resolve => { finishNew = resolve; })) };
  const restored = vi.fn(), busy = vi.fn();
  const view = render(<ProjectRestore client={productApi} onRestored={restored} onBusy={busy} />); await submitRestore();
  view.rerender(<ProjectRestore client={next} onRestored={restored} onBusy={busy} />); await submitRestore();
  expect(busy.mock.calls).toEqual([[true], [false], [true]]);
  await act(async () => rejectOld(new Error("Private old restore failure")));
  expect(screen.queryByRole("alert")).toBeNull(); expect((screen.getByRole("button", { name: "Restoring project…" }) as HTMLButtonElement).disabled).toBe(true);
  expect(busy.mock.calls).toEqual([[true], [false], [true]]);
  const project = { id: "new", name: "restored-project", path: "/other/restored-project" };
  await act(async () => finishNew(project));
  expect(restored).toHaveBeenCalledExactlyOnceWith(project); expect(busy.mock.calls).toEqual([[true], [false], [true], [false]]);
});
