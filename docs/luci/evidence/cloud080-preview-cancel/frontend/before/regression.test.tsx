import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { ProjectDeploy } from "../lens/DeployLens";
import { productApi, type Release } from "../../api/product";
import { resetCockpit } from "../../store/store";
const access = vi.hoisted(() => ({ scope: "workspace", read: true, edit: true, deploy: true, loading: false, error: "" }));
vi.mock("../../hooks/useProjectDraft", () => ({ useProjectDraftScope: (id: string) => `${access.scope}/${id}` }));
vi.mock("../../hooks/useProjectAccess", () => ({ useProjectAccess: () => ({ loading: access.loading, error: access.error, message: "Current project access", canRead: access.read, canEdit: access.read && access.edit && !access.loading && !access.error, canExecute: access.read && access.edit && !access.loading && !access.error, permissions: { read: access.read, edit: access.edit, execute: true, deploy: access.deploy } }) }));
vi.mock("./ProjectConfiguration", () => ({ ProjectConfiguration: () => null }));
const active = { id: "preview-a", projectId: "p-a", briefRevision: 1, revision: 4, canCancel: true, cancelRequested: false, canStop: false, commit: "a".repeat(40), target: "local", status: "building", url: "", message: "Building reviewed source", createdAt: "2026-10-11T13:30:00Z" } as Release & { revision: number; canCancel: boolean; cancelRequested: boolean };
const ready = { ...active, status: "ready", url: "http://127.0.0.1:12345", canCancel: false, canStop: true, message: "Ready" };
function deferred<T>() { let resolve!: (v: T) => void, reject!: (e: unknown) => void; const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no; }); return { promise, resolve, reject }; }
beforeEach(() => {
  resetCockpit(); Object.assign(access, { scope: "workspace", read: true, edit: true, deploy: true, loading: false, error: "" });
  vi.spyOn(productApi, "releases").mockResolvedValue({ releases: [] });
  vi.spyOn(productApi, "release").mockResolvedValue(ready);
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
