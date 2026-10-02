import { afterEach, expect, it, vi } from "vitest";
import { portableOriginAllowed, portableRequest } from "./portableAccess";

afterEach(() => {
  vi.unstubAllGlobals();
  Reflect.deleteProperty(window, "__LUCI_API__");
});
it("accepts only canonical loopback pages bound to the same API and excludes native windows", () => {
  for (const origin of [
    "http://localhost:8730",
    "http://127.0.0.1:8730",
    "http://[::1]:8730",
  ])
    expect(portableOriginAllowed(origin, false, origin)).toBe(true);
  for (const origin of [
    "https://team.example",
    "http://127.0.0.1.example",
    "http://192.168.1.1",
    "file:///tmp/luci",
    "bad-url",
  ])
    expect(portableOriginAllowed(origin, false)).toBe(false);
  expect(portableOriginAllowed("http://localhost:8730", true)).toBe(false);
  expect(
    portableOriginAllowed(
      "http://localhost:8730",
      false,
      "http://localhost:9000",
    ),
  ).toBe(false);
  expect(
    portableOriginAllowed(
      "http://localhost:8730",
      false,
      "https://team.example",
    ),
  ).toBe(false);
});
it("sends owner proof only in the local request header, forbids redirects and never uses browser session credentials", async () => {
  const fetcher = vi
    .fn()
    .mockResolvedValue(new Response(JSON.stringify({ owner: null })));
  vi.stubGlobal("fetch", fetcher);
  await portableRequest("o".repeat(43), "/v1/auth/local-owner");
  expect(fetcher).toHaveBeenCalledWith(
    `${window.location.origin}/v1/auth/local-owner`,
    expect.objectContaining({
      method: "GET",
      headers: { "X-Luci-Local-Access-Token": "o".repeat(43) },
      redirect: "error",
      credentials: "omit",
      cache: "no-store",
    }),
  );
  expect(String(fetcher.mock.calls[0][0])).not.toContain("o".repeat(43));
});
it("refuses invalid proof or a different API origin before any request", async () => {
  const fetcher = vi.fn();
  vi.stubGlobal("fetch", fetcher);
  await expect(
    portableRequest("invalid", "/v1/auth/local-owner"),
  ).rejects.toMatchObject({ status: 403 });
  window.__LUCI_API__ = "https://team.example";
  await expect(
    portableRequest("o".repeat(43), "/v1/auth/local-owner"),
  ).rejects.toMatchObject({ status: 403 });
  expect(fetcher).not.toHaveBeenCalled();
});
