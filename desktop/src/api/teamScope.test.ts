import { afterEach, expect, it, vi } from "vitest";
import { createClient } from "./client";
import { createProductClient } from "./product";
import { authToken, setAuthToken } from "./authSession";
import { selectedTeamId, selectTeamScope, teamScopeGeneration, TEAM_WORKSPACE_CHANGED, workspacePath } from "./teamScope";
import { aSession } from "../test/fixtures";
import { FakeWebSocket } from "../test/fakes";

const first = `team-${"a".repeat(22)}`;
const second = `team-${"b".repeat(22)}`;
const prefix = (id: string) => `/v1/teams/${id}/workspace`;

afterEach(() => {
  setAuthToken(null);
  selectTeamScope("");
  FakeWebSocket.reset();
  vi.restoreAllMocks();
});

it("validates workspace identities and leaves authentication and management outside worker routing", () => {
  selectTeamScope(first);
  const generation = teamScopeGeneration();
  for (const value of ["team-short", `${first}/../${second}`, "https://elsewhere.example", "user-account"]) {
    expect(() => selectTeamScope(value)).toThrow("Invalid team workspace");
  }
  expect(selectedTeamId()).toBe(first);
  expect(teamScopeGeneration()).toBe(generation);
  expect(workspacePath("/v1/projects")).toBe(`${prefix(first)}/v1/projects`);
  expect(workspacePath("/v1/product/projects/p-1/checks?latest=1")).toBe(`${prefix(first)}/v1/product/projects/p-1/checks?latest=1`);
  for (const path of ["/v1/auth/me", "/v1/auth/socket-ticket", "/v1/teams", `${prefix(second)}/v1/projects`, "/v1/admin/users", "/v1/control-plane/status", "/v1/setup/dependencies"]) {
    expect(workspacePath(path)).toBe(path);
  }
  selectTeamScope("");
  expect(workspacePath("/v1/projects")).toBe("/v1/projects");
});

it("keeps the real bearer identity while routing JSON and raw transfers into the selected team", async () => {
  setAuthToken("real-member-token");
  selectTeamScope(first);
  const fetcher = vi.fn<typeof fetch>().mockImplementation(async input => {
    const path = String(input);
    if (path.endsWith("/v1/projects")) return new Response('{"projects":[]}');
    if (path.includes("/restore?")) return new Response('{"id":"p-imported","name":"Imported","path":"private"}', { status: 201 });
    if (path.includes("/backups/")) return new Response(new Uint8Array([0, 255, 10]));
    return new Response('{"user":{"id":"real-member"}}');
  });
  const cockpit = createClient({ baseUrl: "https://luci.example", fetch: fetcher });
  const product = createProductClient("https://luci.example", fetcher);
  await cockpit.listProjects();
  await product.request("/v1/auth/me");
  const archive = new Blob(["source archive"], { type: "application/zip" });
  await product.restoreProject("Imported", archive);
  const backup = await product.requestBlob("/v1/platform/projects/p-1/backups/b-1");
  expect(new Uint8Array(await backup.arrayBuffer())).toEqual(new Uint8Array([0, 255, 10]));
  expect(fetcher.mock.calls.map(call => call[0])).toEqual([
    `https://luci.example${prefix(first)}/v1/projects`,
    "https://luci.example/v1/auth/me",
    `https://luci.example${prefix(first)}/v1/product/projects/restore?name=Imported`,
    `https://luci.example${prefix(first)}/v1/platform/projects/p-1/backups/b-1`,
  ]);
  for (const call of fetcher.mock.calls) expect(new Headers(call[1]?.headers).get("Authorization")).toBe("Bearer real-member-token");
  expect(fetcher.mock.calls[2][1]?.body).toBe(archive);
  expect(cockpit.baseUrl).toBe(`https://luci.example${prefix(first)}`);
  expect(product.baseUrl).toBe(`https://luci.example${prefix(first)}`);
  selectTeamScope(second);
  await cockpit.listProjects();
  expect(fetcher.mock.calls[fetcher.mock.calls.length - 1]?.[0]).toBe(`https://luci.example${prefix(second)}/v1/projects`);
});

it.each(["cockpit", "product"])("discards a late %s response after a team switch", async kind => {
  selectTeamScope(first);
  let reply!: (value: Response) => void;
  const fetcher = vi.fn<typeof fetch>(() => new Promise(resolve => { reply = resolve; }));
  const pending = kind === "cockpit" ? createClient({ fetch: fetcher }).listProjects() : createProductClient("http://localhost", fetcher).request("/v1/projects");
  const observed = pending.catch((error: unknown) => error);
  selectTeamScope(second);
  reply(new Response('{"projects":[{"id":"from-previous-team"}]}'));
  expect(await observed).toMatchObject({ code: "conflict", status: 409 });
});

it.each(["cockpit", "product", "download", "restore"])("discards a %s body finishing after the workspace changed", async kind => {
  selectTeamScope(first);
  let body!: ReadableStreamDefaultController<Uint8Array>;
  const stream = new ReadableStream<Uint8Array>({ start(controller) { body = controller; } });
  const fetcher = vi.fn<typeof fetch>().mockResolvedValue(new Response(stream));
  const product = createProductClient("http://localhost", fetcher);
  const pending = kind === "cockpit" ? createClient({ fetch: fetcher }).listProjects()
    : kind === "product" ? product.request("/v1/projects")
      : kind === "download" ? product.requestBlob("/v1/platform/projects/p-1/backups/b-1")
        : product.restoreProject("Restored", new Blob(["archive"]));
  const observed = pending.catch((error: unknown) => error);
  await vi.waitFor(() => expect(stream.locked).toBe(true));
  selectTeamScope(second);
  body.enqueue(new TextEncoder().encode('{"projects":[],"id":"previous-team-result"}'));
  body.close();
  expect(await observed).toMatchObject({ code: "conflict", status: 409 });
});

it("does not expire the new account when a previous account request returns unauthorized", async () => {
  setAuthToken("previous-account");
  selectTeamScope(first);
  let reply!: (value: Response) => void;
  const fetcher = vi.fn<typeof fetch>(() => new Promise(resolve => { reply = resolve; }));
  const pending = createClient({ fetch: fetcher }).listProjects().catch((error: unknown) => error);
  setAuthToken("current-account");
  expect(selectedTeamId()).toBe("");
  reply(new Response('{"error":"unauthorized","message":"Old account expired"}', { status: 401 }));
  expect(await pending).toMatchObject({ code: "conflict" });
  expect(authToken()).toBe("current-account");
});

it("requests a global one-use ticket for the exact team attach path and never sends bearer credentials in its URL", async () => {
  setAuthToken("real-member-token");
  selectTeamScope(first);
  const fetcher = vi.fn<typeof fetch>().mockResolvedValue(new Response('{"ticket":"single-use-team-ticket"}'));
  const client = createClient({ baseUrl: "https://luci.example", fetch: fetcher, webSocket: url => new FakeWebSocket(url) as unknown as WebSocket });
  const path = `${prefix(first)}/v1/sessions/s-1/attach`;
  await client.openAttach(aSession({ attachPath: path }));
  expect(fetcher.mock.calls[0][0]).toBe("https://luci.example/v1/auth/socket-ticket");
  expect(JSON.parse(String(fetcher.mock.calls[0][1]?.body))).toEqual({ path });
  expect(FakeWebSocket.last()?.url).toBe(`wss://luci.example${path}?ticket=single-use-team-ticket`);
  expect(FakeWebSocket.last()?.url).not.toContain("real-member-token");
});

it("refuses delayed attachment callbacks from another team or the previous personal workspace", async () => {
  setAuthToken("real-member-token");
  selectTeamScope(second);
  const fetcher = vi.fn<typeof fetch>();
  const client = createClient({ fetch: fetcher, webSocket: url => new FakeWebSocket(url) as unknown as WebSocket });
  for (const attachPath of [`${prefix(first)}/v1/sessions/s-1/attach`, "/v1/sessions/s-1/attach"]) {
    expect(() => client.openAttach(aSession({ attachPath }))).toThrow("another workspace");
  }
  selectTeamScope("");
  expect(() => client.openAttach(aSession({ attachPath: `${prefix(second)}/v1/sessions/s-1/attach` }))).toThrow("another workspace");
  expect(fetcher).not.toHaveBeenCalled();
  expect(FakeWebSocket.instances).toHaveLength(0);
});

it("does not open a terminal when team selection changes while its ticket request is pending", async () => {
  setAuthToken("real-member-token");
  selectTeamScope(first);
  let reply!: (value: Response) => void;
  const fetcher = vi.fn<typeof fetch>(() => new Promise(resolve => { reply = resolve; }));
  const client = createClient({ fetch: fetcher, webSocket: url => new FakeWebSocket(url) as unknown as WebSocket });
  const pending = Promise.resolve(client.openAttach(aSession({ attachPath: `${prefix(first)}/v1/sessions/s-1/attach` }))).catch((error: unknown) => error);
  selectTeamScope(second);
  reply(new Response('{"ticket":"abandoned-ticket"}'));
  expect(await pending).toMatchObject({ code: "conflict" });
  expect(FakeWebSocket.instances).toHaveLength(0);
});

it("notifies workspace consumers and clears selection on an actual account change", () => {
  setAuthToken("first-account");
  selectTeamScope(first);
  const changed = vi.fn();
  window.addEventListener(TEAM_WORKSPACE_CHANGED, changed);
  try {
    setAuthToken("first-account");
    expect(selectedTeamId()).toBe(first);
    expect(changed).not.toHaveBeenCalled();
    setAuthToken("second-account");
    expect(selectedTeamId()).toBe("");
    expect(changed).toHaveBeenCalledOnce();
  } finally { window.removeEventListener(TEAM_WORKSPACE_CHANGED, changed); }
});
