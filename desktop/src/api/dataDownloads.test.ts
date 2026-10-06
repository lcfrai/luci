import { afterEach, expect, it, vi } from "vitest";
import { productApi } from "./product";
import { dataApi } from "./data";
import { platformApi } from "./platform";

afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals(); });
const backup = { id: "backup-owned", createdAt: "now", bytes: 10, sha256: "a".repeat(64), includesStorage: true };
it.each(["database", "data", "object"] as const)("does not save a delayed %s download after the caller's access scope changes", async kind => {
  let finish!: (blob: Blob) => void, current = true;
  vi.spyOn(productApi, "requestBlob").mockImplementation(() => new Promise(resolve => { finish = resolve; }));
  const click = vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => {});
  const create = vi.fn(() => "blob:fixture"); vi.stubGlobal("URL", class extends URL { static createObjectURL = create; static revokeObjectURL = vi.fn(); });
  const request = kind === "database" ? platformApi.downloadBackup("owned", backup, () => current)
    : kind === "data" ? dataApi.downloadBackup("owned", backup, () => current)
      : dataApi.downloadObject("owned", "private", "personal.txt", () => current);
  current = false; finish(new Blob(["private bytes"])); await request;
  expect(create).not.toHaveBeenCalled(); expect(click).not.toHaveBeenCalled();
});
it("saves an authorized data backup only after its actual bytes arrive", async () => {
  const blob = new Blob(["actual archive"]); vi.spyOn(productApi, "requestBlob").mockResolvedValue(blob);
  const click = vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => {});
  const create = vi.fn(() => "blob:fixture"); vi.stubGlobal("URL", class extends URL { static createObjectURL = create; static revokeObjectURL = vi.fn(); });
  await dataApi.downloadBackup("owned", backup, () => true);
  expect(create).toHaveBeenCalledExactlyOnceWith(blob); expect(click).toHaveBeenCalledOnce();
});
