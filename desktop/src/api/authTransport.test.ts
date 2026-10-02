import { afterEach, expect, it, vi } from "vitest";
import { createClient } from "./client";
import { createProductClient } from "./product";
import { authToken, setAuthToken } from "./authSession";
import { aSession } from "../test/fixtures";
import { FakeWebSocket } from "../test/fakes";
afterEach(() => {
  setAuthToken(null);
  FakeWebSocket.reset();
});

it("uses a Bearer for the ticket request and only its single-use ticket on the socket URL", async () => {
  setAuthToken("private-session-token");
  const fetcher = vi
    .fn<typeof fetch>()
    .mockResolvedValue(
      new Response(
        JSON.stringify({ ticket: "one-use-ticket", expiresAt: "soon" }),
      ),
    );
  const client = createClient({
    fetch: fetcher,
    webSocket: (url) => new FakeWebSocket(url) as unknown as WebSocket,
  });
  await client.openAttach(aSession({ attachPath: "/v1/sessions/s-1/attach" }));
  expect(fetcher.mock.calls[0][0]).toBe(
    "http://127.0.0.1:8730/v1/auth/socket-ticket",
  );
  expect(fetcher.mock.calls[0][1]?.headers).toMatchObject({
    Authorization: "Bearer private-session-token",
  });
  expect(FakeWebSocket.last()!.url).toBe(
    "ws://127.0.0.1:8730/v1/sessions/s-1/attach?ticket=one-use-ticket",
  );
  expect(FakeWebSocket.last()!.url).not.toContain("private-session-token");
});

it("authenticates raw ZIP uploads and invalidates rejected sessions without treating them as successful restores", async () => {
  setAuthToken("private-session-token");
  const fetcher = vi
    .fn<typeof fetch>()
    .mockResolvedValue(
      new Response(
        JSON.stringify({ error: "unauthorized", message: "Sign in again" }),
        { status: 401 },
      ),
    );
  const archive = new Blob(["zip fixture"], { type: "application/zip" });
  await expect(
    createProductClient("http://localhost", fetcher).restoreProject(
      "restored",
      archive,
    ),
  ).rejects.toThrow("Sign in again");
  expect(fetcher.mock.calls[0][1]).toMatchObject({
    body: archive,
    headers: {
      "Content-Type": "application/zip",
      Authorization: "Bearer private-session-token",
    },
  });
  expect(authToken()).toBeNull();
});
