import { describe, expect, it, vi } from "vitest";
import {
  ApiError,
  createClient,
  DEFAULT_BASE_URL,
  type ProblemCode,
} from "./client";
import { FakeWebSocket } from "../test/fakes";
import { aHarness, aSession } from "../test/fixtures";
import type { Harness, Session, SessionStatus } from "../store/types";
import { attachUrl } from "./client";

/** One recorded call to the injected fetch. */
interface Call {
  url: string;
  init: RequestInit | undefined;
}

/** A fetch the test controls: replies are returned in queue order, calls are recorded, and a
 *  queued Error is thrown rather than returned (that is what a rejected fetch looks like). */
function fetchStub(replies: Array<Response | Error>): {
  fn: typeof globalThis.fetch;
  calls: Call[];
} {
  const calls: Call[] = [];
  const fn = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    calls.push({ url: String(input), init });
    const reply = replies.shift();
    if (reply === undefined) {
      throw new Error(`fetch stub: no reply queued for ${String(input)}`);
    }
    if (reply instanceof Error) throw reply;
    return reply;
  });
  return { fn: fn as unknown as typeof globalThis.fetch, calls };
}

/** The bytes the control plane actually writes: httpx.WriteJSON encodes with a trailing
 *  newline (task 9, step 1), so these bodies carry one. */
const jsonReply = (status: number, body: string): Response =>
  new Response(body + "\n", {
    status,
    headers: { "Content-Type": "application/json; charset=utf-8" },
  });

const problemReply = (status: number, code: string, message: string, details?: unknown): Response =>
  jsonReply(
    status,
    JSON.stringify(details === undefined ? { error: code, message } : { error: code, message, details }),
  );

/** A fetch that never answers. The client's own timer is the only thing that can end it. */
const hangingFetch = ((_input: RequestInfo | URL, init?: RequestInit) =>
  new Promise<Response>((_resolve, reject) => {
    init?.signal?.addEventListener("abort", () =>
      reject(new DOMException("The operation was aborted.", "AbortError")),
    );
  })) as unknown as typeof globalThis.fetch;

/** A fetch whose HEADERS arrive at once and whose BODY never does — a server that answered and
 *  then stalled, or a connection that died after the status line. The abort wiring stands in for
 *  the platform's own: the Fetch spec has an abort error the response body stream, and undici
 *  does, so a hand-built Response must do it by hand to behave like a real one. */
const stallingBodyFetch = ((_input: RequestInfo | URL, init?: RequestInit) => {
  const body = new ReadableStream<Uint8Array>({
    start(controller) {
      init?.signal?.addEventListener("abort", () =>
        controller.error(new DOMException("The operation was aborted.", "AbortError")),
      );
    },
  });
  return Promise.resolve(
    new Response(body, { status: 200, headers: { "Content-Type": "application/json; charset=utf-8" } }),
  );
}) as unknown as typeof globalThis.fetch;

/** Every member of the union, written out. The runtime assertion is thin; the real check is
 *  that this array compiles — a missing member is TS2322 under `npm run typecheck`. */
const ALL_CODES: ProblemCode[] = [
  "invalid_json", "bad_request", "unknown_harness", "unsupported_option",
  "forbidden_origin", "forbidden_host", "project_not_found", "worktree_not_found",
  "session_not_found", "conflict", "sandbox_not_running", "harness_unavailable",
  "upgrade_required", "internal", "non_loopback_publish", "backend_unavailable",
  "network_unavailable", "timeout",
];

describe("createClient — transport", () => {
  it("defaults to the loopback base URL and sends no Authorization header", async () => {
    const { fn, calls } = fetchStub([jsonReply(200, '{"machines":[]}')]);

    await createClient({ fetch: fn }).listMachines();

    expect(calls[0].url).toBe(`${DEFAULT_BASE_URL}/v1/machines`);
    expect(calls[0].init?.method).toBe("GET");
    // There is no token on this hop and there is not meant to be one before Phase 8
    // (contract §4.5). A GET therefore carries no headers at all.
    expect(calls[0].init?.headers).toBeUndefined();
  });

  it("prefers an explicit baseUrl and never doubles the slash", async () => {
    const { fn, calls } = fetchStub([jsonReply(200, '{"machines":[]}')]);

    await createClient({ fetch: fn, baseUrl: "http://127.0.0.1:9999/" }).listMachines();

    expect(calls[0].url).toBe("http://127.0.0.1:9999/v1/machines");
  });

  it("reads backendReady from the body, not from the status code", async () => {
    // The exact bytes task 10's handler writes when Docker is not running. HTTP is still 200.
    const { fn } = fetchStub([
      jsonReply(200, '{"status":"ok","version":"0.2.0","backend":"docker","backendReady":false}'),
    ]);

    const health = await createClient({ fetch: fn }).health();

    expect(health.backendReady).toBe(false);
    expect(health.version).toBe("0.2.0");
  });

  it("raises network_unavailable when fetch rejects", async () => {
    const { fn } = fetchStub([new TypeError("Failed to fetch")]);

    const err = await createClient({ fetch: fn }).listMachines().catch((e: unknown) => e as ApiError);

    expect(err).toBeInstanceOf(ApiError);
    expect((err as ApiError).code).toBe("network_unavailable");
    expect((err as ApiError).status).toBe(0);
  });

  it("raises timeout when the request outlives timeoutMs", async () => {
    const err = await createClient({ fetch: hangingFetch, timeoutMs: 5 })
      .listMachines()
      .catch((e: unknown) => e as ApiError);

    expect((err as ApiError).code).toBe("timeout");
    expect((err as ApiError).status).toBe(0);
  });

  it("raises timeout when the body stalls AFTER the headers arrive", async () => {
    // The cockpit polls this client every two seconds. A server that sends 200 and then stalls
    // mid-body must still end in a `timeout` the pane can render: a request left hanging with
    // nothing to cancel it is a surface that silently stops updating, which tells the user less
    // than a fabricated status would. So the timer has to outlive the headers.
    const err = await createClient({ fetch: stallingBodyFetch, timeoutMs: 5 })
      .listMachines()
      .catch((e: unknown) => e as ApiError);

    expect(err).toBeInstanceOf(ApiError);
    expect((err as ApiError).code).toBe("timeout");
    expect((err as ApiError).status).toBe(0);
  });

  it("sets no timer at all when timeoutMs is 0", async () => {
    const client = createClient({ fetch: hangingFetch, timeoutMs: 0 });

    const outcome = await Promise.race([
      client.listMachines().then(() => "resolved", (e: ApiError) => `rejected: ${e.code}`),
      new Promise<string>((resolve) => setTimeout(() => resolve("still waiting"), 30)),
    ]);

    expect(outcome).toBe("still waiting");
  });
});

describe("createClient — the error shape", () => {
  it("turns a Problem body into an ApiError carrying its code, status and details", async () => {
    const { fn } = fetchStub([
      problemReply(404, "session_not_found", "No session with id s-3f2a91c0.", { id: "s-3f2a91c0" }),
    ]);

    const err = await createClient({ fetch: fn }).listMachines().catch((e: unknown) => e as ApiError);

    expect((err as ApiError).code).toBe("session_not_found");
    expect((err as ApiError).status).toBe(404);
    expect((err as ApiError).message).toBe("No session with id s-3f2a91c0.");
    expect((err as ApiError).details).toEqual({ id: "s-3f2a91c0" });
  });

  it("turns a body that is not JSON into an internal ApiError carrying what arrived", async () => {
    const { fn } = fetchStub([new Response("<html>502 Bad Gateway</html>", { status: 502 })]);

    const err = await createClient({ fetch: fn }).listMachines().catch((e: unknown) => e as ApiError);

    expect((err as ApiError).code).toBe("internal");
    expect((err as ApiError).status).toBe(502);
    expect((err as ApiError).message).toBe("<html>502 Bad Gateway</html>");
  });

  it("never renders a blank sentence: an empty body with no reason phrase still names the status", async () => {
    // HTTP/2 carries no reason phrase at all, so statusText is "" on the wire as well as in a
    // hand-built Response. Without a final fallback this is ApiError("internal", "", 502), and
    // task 20's "did not start" pane renders an empty string — the cockpit staying silent about
    // a failure, which is the one outcome this whole file exists to prevent.
    const { fn } = fetchStub([new Response("", { status: 502, statusText: "" })]);

    const err = await createClient({ fetch: fn }).listMachines().catch((e: unknown) => e as ApiError);

    expect((err as ApiError).code).toBe("internal");
    expect((err as ApiError).status).toBe(502);
    expect((err as ApiError).message).toBe("HTTP 502");
  });

  it("turns JSON that is not a Problem into an internal ApiError", async () => {
    const { fn } = fetchStub([jsonReply(500, '{"error":123}')]);

    const err = await createClient({ fetch: fn }).listMachines().catch((e: unknown) => e as ApiError);

    expect((err as ApiError).code).toBe("internal");
    expect((err as ApiError).status).toBe(500);
  });

  it("is an Error subclass, so task 19's `instanceof ApiError` holds", () => {
    const err = new ApiError("backend_unavailable", "Docker is not running.", 503);

    expect(err).toBeInstanceOf(Error);
    expect(err).toBeInstanceOf(ApiError);
    expect(err.name).toBe("ApiError");
    expect(err.details).toEqual({});
  });

  it("carries all eighteen codes, including the sixteenth server code", () => {
    expect(new Set(ALL_CODES).size).toBe(18);
    expect(ALL_CODES).toContain("non_loopback_publish");
  });
});

describe("createClient — the catalogue reads", () => {
  it("unwraps the machines envelope so no caller writes .machines twice", async () => {
    const { fn } = fetchStub([
      jsonReply(
        200,
        '{"machines":[{"id":"local","name":"This machine","kind":"local","state":"running",' +
          '"backend":"docker","isolation":"container","default":true,"limitations":[]}]}',
      ),
    ]);

    const machines = await createClient({ fetch: fn }).listMachines();

    expect(machines).toHaveLength(1);
    expect(machines[0].id).toBe("local");
    expect(machines[0].default).toBe(true);
  });

  it("returns an empty array for a machine with no projects — a state, not an error", async () => {
    const { fn } = fetchStub([jsonReply(200, '{"projects":[]}')]);

    await expect(createClient({ fetch: fn }).listProjects()).resolves.toEqual([]);
  });

  it("unwraps the branches envelope and puts the project id in the path", async () => {
    const { fn, calls } = fetchStub([
      jsonReply(
        200,
        '{"branches":[{"id":"b-8c1d0e42-evidence-upload","projectId":"p-8c1d0e42",' +
          '"name":"evidence-upload","isHead":true}]}',
      ),
    ]);

    const branches = await createClient({ fetch: fn }).listBranches("p-8c1d0e42");

    expect(calls[0].url).toBe(`${DEFAULT_BASE_URL}/v1/projects/p-8c1d0e42/branches`);
    expect(calls[0].init?.method).toBe("GET");
    expect(branches).toHaveLength(1);
    expect(branches[0].name).toBe("evidence-upload");
    // Server-computed. Task 18's rail reads it to mark the checked-out branch and never
    // re-derives it.
    expect(branches[0].isHead).toBe(true);
  });

  it("returns an empty array for a project with no branches", async () => {
    const { fn } = fetchStub([jsonReply(200, '{"branches":[]}')]);

    await expect(createClient({ fetch: fn }).listBranches("p-8c1d0e42")).resolves.toEqual([]);
  });

  it("leaves the harness arrays exactly as the server sent them", async () => {
    // Task 8's fix round made the Go side allocate unconditionally, so these are [] and never
    // null. The client adds no `?? []`: a defensive default here would hide a server regression
    // that `[].map` would otherwise surface immediately.
    const { fn } = fetchStub([
      jsonReply(
        200,
        '{"harnesses":[{"id":"shell","name":"Shell","command":"/bin/bash","installed":true,' +
          '"flagsVerified":true,"available":true,"models":[],"defaultModel":"","efforts":[],' +
          '"permissionModes":[],"supportsOpeningPrompt":false,"supportsResume":false,' +
          '"note":"A plain shell in the sandbox."}]}',
      ),
    ]);

    const [shell] = await createClient({ fetch: fn }).listHarnesses();

    expect(shell.models).toEqual([]);
    expect(shell.efforts).toEqual([]);
    expect(shell.permissionModes).toEqual([]);
  });

  it("filters worktrees by branch only when a branchId is given", async () => {
    const { fn, calls } = fetchStub([
      jsonReply(200, '{"worktrees":[]}'),
      jsonReply(200, '{"worktrees":[]}'),
    ]);
    const client = createClient({ fetch: fn });

    await client.listWorktrees("p-8c1d0e42");
    await client.listWorktrees("p-8c1d0e42", "b-8c1d0e42-evidence-upload");

    expect(calls[0].url).toBe(`${DEFAULT_BASE_URL}/v1/projects/p-8c1d0e42/worktrees`);
    expect(calls[1].url).toBe(
      `${DEFAULT_BASE_URL}/v1/projects/p-8c1d0e42/worktrees?branchId=b-8c1d0e42-evidence-upload`,
    );
  });

  it("sets the worktree ready flag and reads back the bare worktree object", async () => {
    const { fn, calls } = fetchStub([
      jsonReply(
        200,
        '{"id":"w-4a77b1c9","projectId":"p-8c1d0e42","branchId":"b-1","name":"evidence-upload",' +
          '"path":"C:\\\\work\\\\wt","isPrimary":false,"sessionCount":0,"ready":true}',
      ),
    ]);

    const worktree = await createClient({ fetch: fn }).setWorktreeReady("p-8c1d0e42", "w-4a77b1c9", true);

    expect(calls[0].url).toBe(`${DEFAULT_BASE_URL}/v1/projects/p-8c1d0e42/worktrees/w-4a77b1c9`);
    expect(calls[0].init?.method).toBe("PATCH");
    expect(calls[0].init?.body).toBe('{"ready":true}');
    // Adjudicated decision 2: the worktree itself, not an envelope.
    expect(worktree.ready).toBe(true);
  });
});

/** Overrides on task 15's shared builder, not a local literal of the same shape: shared shape
 *  §3.2 says a task needing a variant passes an override object and declares no builder of its
 *  own. Every field an assertion below reads is named here, so neither row depends on what
 *  `aHarness()` picks for its defaults; whatever a row does not name — `installed` and `note`
 *  among them — is the builder's, and nothing here reads it. */
const claudeCode: Harness = aHarness({
  id: "claude-code",
  name: "Claude Code",
  command: "claude",
  models: [{ id: "opus", name: "Opus" }],
  defaultModel: "opus",
  efforts: ["", "low", "medium", "high"],
  permissionModes: ["ask", "bypass", "plan"],
  supportsOpeningPrompt: true,
  supportsResume: true,
});

const shell: Harness = aHarness({
  id: "shell",
  name: "Shell",
  command: "/bin/bash",
  supportsOpeningPrompt: true,
  supportsResume: false,
});

const source: Session = aSession({
  id: "s-1",
  worktreeId: "w-1",
  machineId: "local",
  harness: "claude-code",
  model: "opus",
  effort: "high",
  permissionMode: "ask",
  openingPrompt: "add tests, then refactor",
  title: "Claude Code · evidence-upload",
  contextRef: "ctx-9",
  attachPath: "/v1/sessions/s-1/attach",
});

const sessionBody = (id: string, status: SessionStatus): string =>
  JSON.stringify({ ...source, id, status, attachPath: `/v1/sessions/${id}/attach` });

describe("createClient — sessions", () => {
  it("accepts 201 from createSession and returns the body, not the Location header", async () => {
    const created = new Response(sessionBody("s-2", "starting") + "\n", {
      status: 201,
      headers: { "Content-Type": "application/json; charset=utf-8", Location: "/v1/sessions/s-2" },
    });
    const { fn, calls } = fetchStub([created]);

    const session = await createClient({ fetch: fn }).createSession({ worktreeId: "w-1" });

    expect(calls[0].url).toBe(`${DEFAULT_BASE_URL}/v1/sessions`);
    expect(calls[0].init?.method).toBe("POST");
    expect(session.id).toBe("s-2");
    expect(session.status).toBe("starting");
  });

  it("omits the fields the caller left undefined", async () => {
    const { fn, calls } = fetchStub([jsonReply(201, sessionBody("s-2", "starting"))]);

    await createClient({ fetch: fn }).createSession({
      worktreeId: "w-1",
      harness: "claude-code",
      title: undefined,
    });

    // JSON.stringify drops undefined. It matters: the Go side decodes `"title": null` into a
    // string field and answers 400, so an explicit null would break create for no reason.
    expect(calls[0].init?.body).toBe('{"worktreeId":"w-1","harness":"claude-code"}');
    expect(calls[0].init?.headers).toEqual({ "Content-Type": "application/json" });
  });

  it("sends no query for an unfiltered list and repeats ?status= per value", async () => {
    const { fn, calls } = fetchStub([jsonReply(200, '{"sessions":[]}'), jsonReply(200, '{"sessions":[]}')]);
    const client = createClient({ fetch: fn });

    await client.listSessions();
    await client.listSessions({ worktreeId: "w-1", status: ["running", "idle"] });

    expect(calls[0].url).toBe(`${DEFAULT_BASE_URL}/v1/sessions`);
    expect(calls[1].url).toBe(`${DEFAULT_BASE_URL}/v1/sessions?worktreeId=w-1&status=running&status=idle`);
  });

  it("maps a 404 from getSession to session_not_found", async () => {
    const { fn } = fetchStub([problemReply(404, "session_not_found", "No session with id s-9.")]);

    const err = await createClient({ fetch: fn }).getSession("s-9").catch((e: unknown) => e as ApiError);

    expect((err as ApiError).code).toBe("session_not_found");
    expect((err as ApiError).status).toBe(404);
  });
});

describe("createClient — the lifecycle verbs", () => {
  it("trims a rename and patches the trimmed title", async () => {
    const { fn, calls } = fetchStub([jsonReply(200, sessionBody("s-1", "running"))]);

    await createClient({ fetch: fn }).renameSession("s-1", "  evidence upload  ");

    expect(calls[0].url).toBe(`${DEFAULT_BASE_URL}/v1/sessions/s-1`);
    expect(calls[0].init?.method).toBe("PATCH");
    expect(calls[0].init?.body).toBe('{"title":"evidence upload"}');
  });

  it("refuses an all-whitespace rename without spending a request", async () => {
    const { fn, calls } = fetchStub([]);

    const err = await createClient({ fetch: fn })
      .renameSession("s-1", "   ")
      .catch((e: unknown) => e as ApiError);

    expect(calls).toHaveLength(0);
    expect((err as ApiError).code).toBe("bad_request");
    expect((err as ApiError).status).toBe(0);
  });

  it("posts no body on restart and maps 409 to conflict", async () => {
    const { fn, calls } = fetchStub([
      jsonReply(200, sessionBody("s-1", "starting")),
      problemReply(409, "conflict", "A restart is already in flight for s-1."),
    ]);
    const client = createClient({ fetch: fn });

    await client.restartSession("s-1");
    const err = await client.restartSession("s-1").catch((e: unknown) => e as ApiError);

    expect(calls[0].url).toBe(`${DEFAULT_BASE_URL}/v1/sessions/s-1/restart`);
    expect(calls[0].init?.method).toBe("POST");
    expect(calls[0].init?.body).toBeUndefined();
    expect((err as ApiError).code).toBe("conflict");
  });

  it("is idempotent about killing: an already-exited session is still a 200", async () => {
    const { fn } = fetchStub([
      jsonReply(200, sessionBody("s-1", "exited")),
      jsonReply(200, sessionBody("s-1", "exited")),
    ]);
    const client = createClient({ fetch: fn });

    expect((await client.killSession("s-1")).status).toBe("exited");
    expect((await client.killSession("s-1")).status).toBe("exited");
  });

  it("accepts the 204 from purgeSession and never reads a body", async () => {
    // new Response(body, { status: 204 }) throws; 204 is a null-body status.
    const { fn, calls } = fetchStub([new Response(null, { status: 204 })]);

    await expect(createClient({ fetch: fn }).purgeSession("s-1")).resolves.toBeUndefined();

    expect(calls[0].url).toBe(`${DEFAULT_BASE_URL}/v1/sessions/s-1?purge=true`);
    expect(calls[0].init?.method).toBe("DELETE");
  });
});

describe("createClient — resume and fork are createSession in disguise", () => {
  it("resumes into a NEW session that carries the source id and no opening prompt", async () => {
    const { fn, calls } = fetchStub([jsonReply(201, sessionBody("s-2", "starting"))]);

    await createClient({ fetch: fn }).resumeSession(source, claudeCode);

    const sent = JSON.parse(String(calls[0].init?.body)) as Record<string, unknown>;
    expect(calls[0].url).toBe(`${DEFAULT_BASE_URL}/v1/sessions`);
    expect(sent.resume).toBe("s-1");
    expect(sent.worktreeId).toBe("w-1");
    expect(sent.title).toBe("Claude Code · evidence-upload");
    expect(sent.openingPrompt).toBeUndefined();
  });

  it("refuses to resume a harness that cannot, without spending a request", async () => {
    const { fn, calls } = fetchStub([]);

    const err = await createClient({ fetch: fn })
      .resumeSession(source, shell)
      .catch((e: unknown) => e as ApiError);

    expect(calls).toHaveLength(0);
    expect((err as ApiError).code).toBe("unsupported_option");
  });

  it("forks the conversation into another worktree, as Resume does, and lets the server derive the title", async () => {
    const { fn, calls } = fetchStub([jsonReply(201, sessionBody("s-3", "starting"))]);

    await createClient({ fetch: fn }).forkSession(source, "w-2", claudeCode);

    const sent = JSON.parse(String(calls[0].init?.body)) as Record<string, unknown>;
    expect(calls[0].url).toBe(`${DEFAULT_BASE_URL}/v1/sessions`);
    expect(sent.worktreeId).toBe("w-2");
    // The source, named the way Resume names it: the control plane starts the CLI on a fork of
    // that session's conversation (Go contract §3.9). Without it the fork is a fresh session.
    expect(sent.resume).toBe("s-1");
    expect(sent.title).toBeUndefined();
    // Carried inside the conversation already; sent again it would be asked twice.
    expect(sent.openingPrompt).toBeUndefined();
    expect(sent.model).toBe(source.model);
    expect(sent.permissionMode).toBe(source.permissionMode);
  });

  it("forks a shell exactly as before: the configuration and the opening prompt, no conversation", async () => {
    const { fn, calls } = fetchStub([jsonReply(201, sessionBody("s-4", "starting"))]);
    const shellSource: Session = { ...source, harness: "shell" };

    await createClient({ fetch: fn }).forkSession(shellSource, "w-2", shell);

    const sent = JSON.parse(String(calls[0].init?.body)) as Record<string, unknown>;
    expect(sent.worktreeId).toBe("w-2");
    expect(sent.harness).toBe("shell");
    expect(sent.resume).toBeUndefined();
    expect(sent.openingPrompt).toBe("add tests, then refactor");
  });

  it("refuses a fork into the source's own worktree", async () => {
    const { fn, calls } = fetchStub([]);

    const err = await createClient({ fetch: fn })
      .forkSession(source, "w-1", claudeCode)
      .catch((e: unknown) => e as ApiError);

    expect(calls).toHaveLength(0);
    expect((err as ApiError).code).toBe("bad_request");
  });
});

describe("createClient — the attach socket", () => {
  it("builds ws:// from http:// and takes the path from the session", () => {
    expect(attachUrl("http://127.0.0.1:8730", source)).toBe("ws://127.0.0.1:8730/v1/sessions/s-1/attach");
  });

  it("builds wss:// from https:// and discards a path carried by the base", () => {
    expect(attachUrl("https://desk.example/console", source)).toBe("wss://desk.example/v1/sessions/s-1/attach");
  });

  it("opens through the injected factory and sets binaryType before returning", () => {
    FakeWebSocket.reset();
    const client = createClient({ webSocket: (url) => new FakeWebSocket(url) as unknown as WebSocket });

    const socket = client.openAttach(source);

    expect(FakeWebSocket.instances).toHaveLength(1);
    expect(FakeWebSocket.instances[0].url).toBe("ws://127.0.0.1:8730/v1/sessions/s-1/attach");
    expect(socket.binaryType).toBe("arraybuffer");
  });
});
