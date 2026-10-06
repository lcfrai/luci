import { afterEach, expect, it, vi } from "vitest";
import { setAuthToken } from "./authSession";
import { createProductClient, productApi } from "./product";
import { platformApi } from "./platform";

const { webcrypto } = await vi.importActual<{ webcrypto: Crypto }>("node:crypto");

function dump(contents = "PGDMP\x00\xff-private-data") {
  const bytes = new TextEncoder().encode(contents);
  const file = new File([bytes], "saved.dump", { type: "application/octet-stream" });
  Object.defineProperty(file, "arrayBuffer", { value: vi.fn(async () => bytes.buffer) });
  return { file, bytes, digest: "ca7de58ef4d6a7b8ad1379bcd064b2415418b0b86cd02d27a5e74f95999df0f8" };
}
afterEach(() => { setAuthToken(null); vi.restoreAllMocks(); vi.unstubAllGlobals(); });

it("uploads unchanged private bytes with authentication, deliberate trust and the locally computed checksum", async () => {
  vi.stubGlobal("crypto", webcrypto);
  setAuthToken("fixture-owner");
  const { file, bytes, digest } = dump();
  const receipt = { id: "backup-imported", imported: true, bytes: bytes.length, sha256: digest, createdAt: "now" };
  const fetcher = vi.fn<typeof fetch>().mockResolvedValue(new Response(JSON.stringify(receipt), { status: 201 }));
  vi.spyOn(productApi, "requestBlob").mockImplementation(createProductClient("https://workspace.example/luci", fetcher).requestBlob);
  expect(await platformApi.importDatabaseBackup("project /1", file, true)).toEqual(receipt);
  expect(fetcher).toHaveBeenCalledExactlyOnceWith("https://workspace.example/luci/v1/platform/projects/project%20%2F1/backups/import", expect.objectContaining({
    method: "POST", body: file,
    headers: expect.objectContaining({ Authorization: "Bearer fixture-owner", "Content-Type": "application/octet-stream", "X-Luci-Trust-Backup": "true", "X-Luci-Backup-SHA256": digest }),
  }));
});

it("refuses missing trust, oversized files, unsupported archives and abandoned account scopes before upload", async () => {
  vi.stubGlobal("crypto", webcrypto);
  const transfer = vi.spyOn(productApi, "requestBlob");
  const { file } = dump();
  await expect(platformApi.importDatabaseBackup("p", file, false)).rejects.toThrow("trust");
  const oversized = dump().file; Object.defineProperty(oversized, "size", { value: 64 * 1024 * 1024 + 1 });
  await expect(platformApi.importDatabaseBackup("p", oversized, true)).rejects.toThrow("64 MiB");
  await expect(platformApi.importDatabaseBackup("p", dump("SELECT 1;").file, true)).rejects.toThrow("custom-format");
  const selected = new AbortController(); selected.abort(new Error("Project changed"));
  await expect(platformApi.importDatabaseBackup("p", file, true, selected.signal)).rejects.toThrow("Project changed");
  setAuthToken("first-owner");
  const changed = dump().file;
  vi.mocked(changed.arrayBuffer).mockImplementation(async () => { setAuthToken("other-owner"); return new TextEncoder().encode("PGDMP-owned").buffer; });
  await expect(platformApi.importDatabaseBackup("p", changed, true)).rejects.toThrow("account or workspace changed");
  expect(transfer).not.toHaveBeenCalled();
});

it("rejects a mismatched import receipt instead of presenting the wrong stored backup", async () => {
  vi.stubGlobal("crypto", webcrypto);
  const { file } = dump();
  vi.spyOn(productApi, "requestBlob").mockImplementation(async () => new Response(JSON.stringify({ id: "wrong", bytes: file.size, sha256: "0".repeat(64) })).blob());
  await expect(platformApi.importDatabaseBackup("p", file, true)).rejects.toThrow("receipt does not match");
});
