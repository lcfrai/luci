import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { ProjectDeploy } from "../lens/DeployLens";
import { productApi, type Release } from "../../api/product";
import { ApiError } from "../../api/client";
import { resetCockpit } from "../../store/store";
const access = vi.hoisted(() => ({ scope: "workspace", read: true, edit: true, deploy: true, loading: false, error: "" }));
vi.mock("../../hooks/useProjectDraft", () => ({ useProjectDraftScope: (id: string) => `${access.scope}/${id}` }));
vi.mock("../../hooks/useProjectAccess", () => ({ useProjectAccess: () => ({ loading: access.loading, error: access.error, message: "Current project access", canRead: access.read, canEdit: access.read && access.edit && !access.loading && !access.error, canExecute: access.read && access.edit && !access.loading && !access.error, permissions: { read: access.read, edit: access.edit, execute: true, deploy: access.deploy } }) }));
vi.mock("./ProjectConfiguration", () => ({ ProjectConfiguration: () => null }));
const active = { id: "preview-a", projectId: "p-a", briefRevision: 1, revision: 4, canCancel: true, cancelRequested: false, canStop: false, commit: "a".repeat(40), target: "local", status: "building", url: "", message: "Building reviewed source", createdAt: "2026-10-11T13:30:00Z" } as Release & { revision: number; canCancel: boolean; cancelRequested: boolean };
const requested: Release = { ...active, revision: 5, canCancel: false, cancelRequested: true, status: "cancelling", message: "Cancellation recorded; awaiting build settlement" };
const ready = { ...active, status: "ready", url: "http://127.0.0.1:12345", canCancel: false, canStop: true, message: "Ready" };
function deferred<T>() { let resolve!: (v: T) => void, reject!: (e: unknown) => void; const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no; }); return { promise, resolve, reject }; }
beforeEach(() => {
  resetCockpit(); Object.assign(access, { scope: "workspace", read: true, edit: true, deploy: true, loading: false, error: "" });
  vi.spyOn(productApi, "releases").mockResolvedValue({ releases: [] });
  vi.spyOn(productApi, "release").mockResolvedValue(ready);
  vi.spyOn(productApi, "cancelRelease").mockResolvedValue(requested);
  vi.spyOn(productApi, "stopRelease").mockResolvedValue({ ...ready, status: "stopped" });
});
afterEach(() => { vi.restoreAllMocks(); vi.useRealTimers(); });
it("offers exact build cancellation while the original preview creation response is held", async () => {
  vi.useFakeTimers({ toFake: ["setInterval", "clearInterval"] });
  const held = deferred<Release>();
  vi.mocked(productApi.release).mockReturnValue(held.promise);
  render(<ProjectDeploy projectId="p-a" />);
  await act(async () => {});
  fireEvent.click(screen.getByRole("button", { name: "Build local preview" }));
  expect(productApi.release).toHaveBeenCalledExactlyOnceWith("p-a", 3000, "Dockerfile");
  vi.mocked(productApi.releases).mockResolvedValue({ releases: [active] });
  await act(async () => { await vi.advanceTimersByTimeAsync(3000); });
  const cancel = screen.getByRole("button", { name: "Cancel build" });
  expect((cancel as HTMLButtonElement).disabled).toBe(false);
  expect(productApi.stopRelease).not.toHaveBeenCalled();
  await act(async () => held.resolve(ready));
});

async function visibleBuild() {
  vi.mocked(productApi.releases).mockResolvedValue({ releases: [active] });
  const view = render(<ProjectDeploy projectId="p-a" />);
  await screen.findByRole("button", { name: "Cancel build" });
  return view;
}
it("dispatches one exact cancellation independently of held creation and fences an earlier history response", async () => {
  vi.useFakeTimers({ toFake: ["setInterval", "clearInterval"] });
  const start = deferred<Release>(), cancel = deferred<Release>(), oldHistory = deferred<{ releases: Release[] }>();
  vi.mocked(productApi.release).mockReturnValue(start.promise);
  vi.mocked(productApi.cancelRelease).mockReturnValue(cancel.promise);
  render(<ProjectDeploy projectId="p-a" />); await act(async () => {});
  fireEvent.click(screen.getByRole("button", { name: "Build local preview" }));
  vi.mocked(productApi.releases).mockResolvedValue({ releases: [active] });
  await act(async () => { await vi.advanceTimersByTimeAsync(3000); });
  const button = screen.getByRole("button", { name: "Cancel build" });
  fireEvent.click(button); fireEvent.click(button);
  expect(productApi.cancelRelease).toHaveBeenCalledExactlyOnceWith("p-a", "preview-a", 4);
  expect(screen.getByText(/This does not yet confirm that the build stopped/)).toBeTruthy();
  expect(productApi.stopRelease).not.toHaveBeenCalled();
  vi.mocked(productApi.releases).mockReturnValueOnce(oldHistory.promise);
  await act(async () => { await vi.advanceTimersByTimeAsync(3000); });
  await act(async () => cancel.resolve(requested));
  expect(screen.getByText(/Waiting for the build outcome and confirmed cleanup/)).toBeTruthy();
  await act(async () => oldHistory.resolve({ releases: [{ ...active, message: "STALE HISTORY" }] }));
  expect(screen.queryByText("STALE HISTORY")).toBeNull();
  expect(screen.queryByRole("button", { name: "Cancel build" })).toBeNull();
  vi.mocked(productApi.releases).mockResolvedValue({ releases: [requested] });
  await act(async () => start.resolve(requested));
  expect(productApi.release).toHaveBeenCalledTimes(1);
});
it.each(["edit", "deploy", "loading", "read"] as const)("checks current %s authority at cancellation time", async permission => {
  const view = await visibleBuild();
  const oldButton = screen.getByRole("button", { name: "Cancel build" });
  access[permission] = permission === "loading";
  view.rerender(<ProjectDeploy projectId="p-a" />);
  fireEvent.click(oldButton);
  expect(productApi.cancelRelease).not.toHaveBeenCalled();
  if (permission === "read") expect(screen.queryByText("Building reviewed source")).toBeNull();
  else expect((screen.getByRole("button", { name: "Cancel build" }) as HTMLButtonElement).disabled).toBe(true);
});
it.each(["read", "deploy"] as const)("withholds late cancellation contents across %s permission churn and requires a later explicit read", async permission => {
  const held = deferred<Release>(); vi.mocked(productApi.cancelRelease).mockReturnValue(held.promise);
  const view = await visibleBuild(); fireEvent.click(screen.getByRole("button", { name: "Cancel build" }));
  access[permission] = false; view.rerender(<ProjectDeploy projectId="p-a" />);
  access[permission] = true; view.rerender(<ProjectDeploy projectId="p-a" />);
  await act(async () => {});
  await act(async () => held.resolve({ ...requested, message: "LATE PRIVATE CONTENT" }));
  expect(screen.queryByText("LATE PRIVATE CONTENT")).toBeNull();
  expect(screen.getByRole("alert").textContent).toContain("Cancellation could not be confirmed");
  expect((screen.getByRole("button", { name: "Cancel build" }) as HTMLButtonElement).disabled).toBe(true);
  vi.mocked(productApi.releases).mockResolvedValue({ releases: [requested] });
  fireEvent.click(screen.getByRole("button", { name: "Refresh" }));
  await waitFor(() => expect(screen.queryByRole("alert")).toBeNull());
  expect(screen.queryByRole("button", { name: "Cancel build" })).toBeNull();
  expect(productApi.cancelRelease).toHaveBeenCalledTimes(1);
});
it("keeps another project clear of late cancellation responses and earlier creation completion", async () => {
  const cancel = deferred<Release>(), start = deferred<Release>();
  vi.mocked(productApi.cancelRelease).mockReturnValue(cancel.promise); vi.mocked(productApi.release).mockReturnValue(start.promise);
  const view = await visibleBuild(); fireEvent.click(screen.getByRole("button", { name: "Build local preview" }));
  fireEvent.click(screen.getByRole("button", { name: "Cancel build" }));
  vi.mocked(productApi.releases).mockImplementation(async id => ({ releases: id === "p-b" ? [] : [requested] }));
  view.rerender(<ProjectDeploy projectId="p-b" />); await act(async () => {});
  await act(async () => { cancel.resolve({ ...requested, message: "OLD PROJECT RESULT" }); start.resolve(ready); });
  expect(screen.queryByText("OLD PROJECT RESULT")).toBeNull(); expect(screen.queryByText(/Cancellation requested/)).toBeNull();
  expect(screen.getByText("No releases yet. Your first preview will appear here.")).toBeTruthy();
  expect(productApi.cancelRelease).toHaveBeenCalledExactlyOnceWith("p-a", "preview-a", 4);
  expect(productApi.stopRelease).not.toHaveBeenCalled();
});
it("retains uncertain transport outcomes despite passive polls until an explicit later Refresh", async () => {
  vi.useFakeTimers({ toFake: ["setInterval", "clearInterval"] });
  const held = deferred<Release>(), start = deferred<Release>(), older = deferred<{ releases: Release[] }>();
  vi.mocked(productApi.cancelRelease).mockReturnValue(held.promise); vi.mocked(productApi.release).mockReturnValue(start.promise);
  await visibleBuild(); fireEvent.click(screen.getByRole("button", { name: "Build local preview" }));
  fireEvent.click(screen.getByRole("button", { name: "Cancel build" }));
  vi.mocked(productApi.releases).mockReturnValueOnce(older.promise);
  await act(async () => { await vi.advanceTimersByTimeAsync(3000); });
  await act(async () => held.reject(new ApiError("timeout", "Cancellation response timed out", 0)));
  await act(async () => older.resolve({ releases: [active] }));
  await act(async () => { await vi.advanceTimersByTimeAsync(3000); });
  expect((screen.getByRole("button", { name: "Cancel build" }) as HTMLButtonElement).disabled).toBe(true);
  expect(screen.getByRole("alert").textContent).toContain("Cancellation could not be confirmed");
  expect(productApi.cancelRelease).toHaveBeenCalledTimes(1);
  fireEvent.click(screen.getByRole("button", { name: "Refresh" })); await act(async () => {});
  expect((screen.getByRole("button", { name: "Cancel build" }) as HTMLButtonElement).disabled).toBe(false);
  expect(productApi.cancelRelease).toHaveBeenCalledTimes(1);
  await act(async () => start.resolve(ready));
});
it("keeps unknown cancellation charged-outcome guidance and stops passive polling after the active operation settles", async () => {
  vi.useFakeTimers({ toFake: ["setInterval", "clearInterval"] });
  await visibleBuild(); fireEvent.click(screen.getByRole("button", { name: "Cancel build" })); await act(async () => {});
  const unknown = { ...requested, revision: 6, status: "unknown", message: "Build acknowledgement is unknown; capacity remains reserved", canStop: false };
  vi.mocked(productApi.releases).mockResolvedValue({ releases: [unknown] });
  await act(async () => { await vi.advanceTimersByTimeAsync(3000); });
  expect(screen.getByText(/build outcome or cleanup is unconfirmed/)).toBeTruthy();
  expect(screen.queryByText(/cancellation completed/)).toBeNull();
  expect(screen.queryByRole("button", { name: "Stop preview" })).toBeNull();
  const count = vi.mocked(productApi.releases).mock.calls.length;
  await act(async () => { await vi.advanceTimersByTimeAsync(15000); });
  expect(productApi.releases).toHaveBeenCalledTimes(count);
  expect(productApi.cancelRelease).toHaveBeenCalledTimes(1);
});
it("announces completed cancellation only after the authoritative cancelled outcome", async () => {
  await visibleBuild(); fireEvent.click(screen.getByRole("button", { name: "Cancel build" })); await act(async () => {});
  expect(screen.queryByText(/cancellation completed/)).toBeNull();
  vi.mocked(productApi.releases).mockResolvedValue({ releases: [{ ...requested, revision: 6, status: "cancelled", message: "Exact allocations settled; capacity released", canStop: false }] });
  fireEvent.click(screen.getByRole("button", { name: "Refresh" }));
  await screen.findByText("Build cancellation completed. The release history is retained.");
  expect(screen.queryByRole("button", { name: "Cancel build" })).toBeNull();
  expect(productApi.stopRelease).not.toHaveBeenCalled();
});
it.each([409, 403])("retains a refused %s cancellation without stop or request replay", async status => {
  vi.mocked(productApi.cancelRelease).mockRejectedValue(new ApiError("conflict", "Current revision or authority refused", status));
  await visibleBuild(); fireEvent.click(screen.getByRole("button", { name: "Cancel build" }));
  await screen.findByRole("alert");
  expect(productApi.cancelRelease).toHaveBeenCalledTimes(1); expect(productApi.stopRelease).not.toHaveBeenCalled();
  expect((screen.getByRole("button", { name: "Cancel build" }) as HTMLButtonElement).disabled).toBe(true);
});
it("does not accept a mismatched cancellation response as acknowledgment", async () => {
  vi.mocked(productApi.cancelRelease).mockResolvedValue({ ...requested, projectId: "p-other" });
  await visibleBuild(); fireEvent.click(screen.getByRole("button", { name: "Cancel build" }));
  expect((await screen.findByRole("alert")).textContent).toContain("could not be confirmed");
  expect(screen.queryByText(/Waiting for the build outcome and confirmed cleanup/)).toBeNull();
});
it("retains the normal exact Stop behavior for a ready preview", async () => {
  vi.mocked(productApi.releases).mockResolvedValue({ releases: [ready] });
  render(<ProjectDeploy projectId="p-a" />);
  fireEvent.click(await screen.findByRole("button", { name: "Stop preview" }));
  await waitFor(() => expect(productApi.stopRelease).toHaveBeenCalledExactlyOnceWith("p-a", "preview-a"));
  expect(productApi.cancelRelease).not.toHaveBeenCalled();
});
it("does not offer cancellation for a retained nonlive operation or dispatch without a current revision", async () => {
  vi.mocked(productApi.releases).mockResolvedValue({ releases: [{ ...active, canCancel: false }] });
  render(<ProjectDeploy projectId="p-a" />); await act(async () => {});
  expect(screen.queryByRole("button", { name: "Cancel build" })).toBeNull();
  vi.mocked(productApi.releases).mockResolvedValue({ releases: [{ ...active, revision: undefined }] });
  fireEvent.click(screen.getByRole("button", { name: "Refresh" }));
  const button = await screen.findByRole("button", { name: "Cancel build" });
  expect((button as HTMLButtonElement).disabled).toBe(true); fireEvent.click(button);
  expect(productApi.cancelRelease).not.toHaveBeenCalled();
});

it("preserves a newer authoritative cancelled history when the earlier cancel acknowledgment arrives", async () => {
  const held = deferred<Release>(); vi.mocked(productApi.cancelRelease).mockReturnValue(held.promise);
  await visibleBuild(); fireEvent.click(screen.getByRole("button", { name: "Cancel build" }));
  vi.mocked(productApi.releases).mockResolvedValue({ releases: [{ ...requested, revision: 6, status: "cancelled", message: "Current cancellation has settled" }] });
  fireEvent.click(screen.getByRole("button", { name: "Refresh" })); await act(async () => {});
  await act(async () => held.resolve(requested));
  expect(screen.getByText("Current cancellation has settled")).toBeTruthy();
  expect(screen.getByText("Build cancellation completed. The release history is retained.")).toBeTruthy();
  expect(screen.queryByText(/Waiting for the build outcome/)).toBeNull();
});
it("does not call a natural ready outcome successful cancellation", async () => {
  vi.mocked(productApi.releases).mockResolvedValue({ releases: [{ ...ready, revision: 6, cancelRequested: true }] });
  render(<ProjectDeploy projectId="p-a" />);
  await screen.findByText(/this release became ready/);
  expect(screen.queryByText(/cancellation completed/)).toBeNull();
  expect(screen.queryByText(/Waiting for the build outcome/)).toBeNull();
  expect((screen.getByRole("button", { name: "Stop preview" }) as HTMLButtonElement).disabled).toBe(false);
});

it("keeps an unresolved request visible if later history no longer includes its row", async () => {
  vi.mocked(productApi.cancelRelease).mockRejectedValue(new ApiError("timeout", "Unknown cancellation transport", 0));
  await visibleBuild(); fireEvent.click(screen.getByRole("button", { name: "Cancel build" }));
  await screen.findByRole("alert");
  vi.mocked(productApi.releases).mockResolvedValue({ releases: [] });
  fireEvent.click(screen.getByRole("button", { name: "Refresh" })); await act(async () => {});
  expect(screen.getByRole("alert").textContent).toContain("Cancellation could not be confirmed");
  expect((screen.getByRole("button", { name: "Build local preview" }) as HTMLButtonElement).disabled).toBe(true);
  expect(productApi.release).not.toHaveBeenCalled(); expect(productApi.cancelRelease).toHaveBeenCalledTimes(1);
});
