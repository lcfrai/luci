import { afterEach, expect, it, vi } from "vitest";
import { AUTH_EXPIRED, authToken, setAuthToken } from "./authSession";
import { createProductClient, productApi } from "./product";
import { isolatedBrowserURL, platformApi } from "./platform";
import { dataApi } from "./data";

const objectURL = Object.getOwnPropertyDescriptor(URL, "createObjectURL");
const revokeURL = Object.getOwnPropertyDescriptor(URL, "revokeObjectURL");
const runtimeBase = Object.getOwnPropertyDescriptor(window, "__LUCI_API__");

function restore(target: object, key: string, descriptor?: PropertyDescriptor) {
  if (descriptor) Object.defineProperty(target, key, descriptor);
  else Reflect.deleteProperty(target, key);
}

afterEach(() => {
  setAuthToken(null);
  vi.restoreAllMocks();
  vi.useRealTimers();
  restore(URL, "createObjectURL", objectURL);
  restore(URL, "revokeObjectURL", revokeURL);
  restore(window, "__LUCI_API__", runtimeBase);
});

it("downloads unmodified backup bytes from the selected workspace, with authentication only in headers", async () => {
  Object.defineProperty(window, "__LUCI_API__", { configurable: true, value: "http://127.0.0.1:8750/" });
  setAuthToken("transfer-test-session");
  const bytes = new Uint8Array([80, 71, 68, 77, 80, 0, 255, 128, 13, 10]);
  const fetcher = vi.fn<typeof fetch>().mockResolvedValue(new Response(bytes));
  const selected = new AbortController();
  const remove = vi.spyOn(selected.signal, "removeEventListener");
  const client = createProductClient(undefined, fetcher);
  const blob = await client.requestBlob("/v1/platform/projects/p-1/backups/b-1", {}, 60_000, selected.signal);
  expect(client.baseUrl).toBe("http://127.0.0.1:8750");
  expect(new Uint8Array(await blob.arrayBuffer())).toEqual(bytes);
  expect(fetcher.mock.calls[0][0]).toBe("http://127.0.0.1:8750/v1/platform/projects/p-1/backups/b-1");
  expect(fetcher.mock.calls[0][1]).toMatchObject({ method: "GET", headers: { Authorization: "Bearer transfer-test-session" } });
  expect(fetcher.mock.calls[0][1]?.body).toBeUndefined();
  expect(remove).toHaveBeenCalledOnce();
  expect(selected.signal.aborted).toBe(false);
});

it("uses the same selected transport for database backups, data backups and nested storage objects", async () => {
  vi.useFakeTimers();
  setAuthToken("transfer-test-session");
  const fetcher = vi.fn<typeof fetch>().mockImplementation(async () => new Response(new Uint8Array([1, 2, 3])));
  const client = createProductClient("https://workspace.example/luci/", fetcher);
  vi.spyOn(productApi, "requestBlob").mockImplementation(client.requestBlob);
  const created = vi.fn(() => "blob:download-test");
  const revoked = vi.fn();
  Object.defineProperty(URL, "createObjectURL", { configurable: true, value: created });
  Object.defineProperty(URL, "revokeObjectURL", { configurable: true, value: revoked });
  const downloads: { href: string; name: string }[] = [];
  vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(function (this: HTMLAnchorElement) {
    downloads.push({ href: this.href, name: this.download });
  });
  const backup = { id: "backup #1", createdAt: "now", bytes: 3, sha256: "fixture" };
  await platformApi.downloadBackup("project /1", backup);
  await dataApi.downloadBackup("project /1", { ...backup, includesStorage: true });
  await dataApi.downloadObject("project /1", "public assets", "nested folder/photo #1.png");
  const file = new File(["file contents"], "photo.png", { type: "image/png" });
  await dataApi.upload("project /1", "public assets", "nested folder/photo #1.png", file);
  const root = "https://workspace.example/luci/v1/platform/projects/project%20%2F1";
  const object = `${root}/data/buckets/public%20assets/objects/nested%20folder/photo%20%231.png`;
  expect(fetcher.mock.calls.map(call => call[0])).toEqual([
    `${root}/backups/backup%20%231`, `${root}/data/backups/backup%20%231`, object, object,
  ]);
  expect(fetcher.mock.calls.every(call => new Headers(call[1]?.headers).get("Authorization") === "Bearer transfer-test-session")).toBe(true);
  expect(fetcher.mock.calls[3][1]).toMatchObject({ method: "POST", body: file, headers: { "Content-Type": "image/png" } });
  expect(downloads).toEqual([
    { href: "blob:download-test", name: "backup #1.dump" },
    { href: "blob:download-test", name: "backup #1.zip" },
    { href: "blob:download-test", name: "photo #1.png" },
  ]);
  expect(created).toHaveBeenCalledTimes(3);
  expect(vi.getTimerCount()).toBe(3);
  await vi.advanceTimersByTimeAsync(1000);
  expect(revoked).toHaveBeenCalledTimes(3);
  expect(vi.getTimerCount()).toBe(0);
  const oversized = new File([new Uint8Array(8 * 1024 * 1024 + 1)], "too-large.bin");
  await expect(dataApi.upload("project /1", "public assets", "large.bin", oversized)).rejects.toThrow("8 MiB");
  expect(fetcher).toHaveBeenCalledTimes(4);
});

it("rejects an expired download without creating a misleading file and clears the current session", async () => {
  setAuthToken("expired-transfer-test-session");
  const expired = vi.fn();
  window.addEventListener(AUTH_EXPIRED, expired);
  const fetcher = vi.fn<typeof fetch>().mockResolvedValue(new Response(
    JSON.stringify({ error: "unauthorized", message: "Sign in again" }), { status: 401 },
  ));
  vi.spyOn(productApi, "requestBlob").mockImplementation(createProductClient("http://127.0.0.1:8750", fetcher).requestBlob);
  const created = vi.fn();
  Object.defineProperty(URL, "createObjectURL", { configurable: true, value: created });
  try {
    await expect(platformApi.downloadBackup("p-1", { id: "b-1", createdAt: "now", bytes: 3, sha256: "fixture" }))
      .rejects.toMatchObject({ code: "unauthorized", status: 401, message: "Sign in again" });
    expect(authToken()).toBeNull();
    expect(expired).toHaveBeenCalledOnce();
    expect(created).not.toHaveBeenCalled();
    expect(fetcher).toHaveBeenCalledOnce();
  } finally {
    window.removeEventListener(AUTH_EXPIRED, expired);
  }
});

it("does not dispatch a binary transfer for an already abandoned workspace", async () => {
  vi.useFakeTimers();
  const selected = new AbortController();
  selected.abort(new Error("Workspace changed"));
  const remove = vi.spyOn(selected.signal, "removeEventListener");
  const fetcher = vi.fn<typeof fetch>();
  await expect(createProductClient("http://127.0.0.1:8750", fetcher).requestBlob("/backup", {}, 60_000, selected.signal))
    .rejects.toBe(selected.signal.reason);
  expect(fetcher).not.toHaveBeenCalled();
  expect(remove).toHaveBeenCalledOnce();
  expect(vi.getTimerCount()).toBe(0);
});

it.each(["caller", "timeout"])("cancels a stalled binary response body on %s abort and releases its resources", async kind => {
  vi.useFakeTimers();
  const selected = new AbortController();
  const remove = vi.spyOn(selected.signal, "removeEventListener");
  let observed!: AbortSignal;
  let body!: ReadableStream<Uint8Array>;
  const fetcher = vi.fn<typeof fetch>().mockImplementation(async (_url, init) => {
    observed = init!.signal!;
    body = new ReadableStream<Uint8Array>({ start(stream) {
      observed.addEventListener("abort", () => stream.error(observed.reason), { once: true });
    } });
    return new Response(body);
  });
  const result = createProductClient("http://127.0.0.1:8750", fetcher).requestBlob("/backup", {}, 60_000, selected.signal);
  const outcome = result.catch((cause: unknown) => cause);
  await Promise.resolve();
  expect(body.locked).toBe(true);
  if (kind === "caller") selected.abort(new Error("Workspace changed"));
  else await vi.advanceTimersByTimeAsync(60_000);
  const cause = await outcome;
  if (kind === "caller") expect(cause).toBe(selected.signal.reason);
  else expect(cause).toMatchObject({ code: "timeout", message: expect.stringContaining("file transfer timed out") });
  expect(observed.aborted).toBe(true);
  expect(remove).toHaveBeenCalledOnce();
  expect(vi.getTimerCount()).toBe(0);
});

it("excludes the captured workspace control origin from browser previews on a nondefault port", () => {
  vi.spyOn(productApi, "baseUrl", "get").mockReturnValue("http://127.0.0.1:8750/luci");
  expect(isolatedBrowserURL("http://127.0.0.1:8750/anything")).toBeUndefined();
  expect(isolatedBrowserURL("http://127.0.0.1:8751/")).toBe("http://127.0.0.1:8751/");
});
