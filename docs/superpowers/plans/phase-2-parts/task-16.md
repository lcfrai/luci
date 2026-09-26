### Task 16: The typed control-plane client

This is the only file in the cockpit that knows a URL, a status code or a wire field name. Everything above it — the store, the rail, the panes, the verbs, the poller — calls a method and gets a domain object from `store/types.ts` or an `ApiError`, and that is the whole point: contract §4 says "one exported function per Go endpoint" so that no component ever writes `.machines` twice and no component ever invents a path. The cost of getting it wrong is not a compile error. A client that reads `res.json()` on the 204 that `purgeSession` receives throws `SyntaxError: Unexpected end of JSON input` inside a `catch` block that was written for `ApiError`, and the user sees nothing at all; a client that treats `createSession`'s **201** as a failure makes the product's central verb look broken while the session it created runs perfectly well in the background.

**The injected `fetch` and `WebSocket` are the seam that makes the other ten Group D tasks testable, and they are not a style preference.** Contract §4.1 gives the reason for the injection — jsdom implements neither, and a test that patches `globalThis.fetch` leaks that patch into whatever runs next — and contract §10.1 gives the reason the injection is possible at all: Group C's task 9 shipped the CORS grant, so the desktop keeps the platform `fetch` instead of falling back to `@tauri-apps/plugin-http`, whose `fetch` exists only inside a Tauri window and therefore cannot be injected by a unit test at all. That fallback is now dead and this plan is written against the live one. The consequence for every later task is direct: task 19's registry, task 22's verbs and task 24's wiring all get their control plane from an argument, so **no Group D test ever needs a control plane running**, and the one thing that genuinely cannot be proved that way — that a real browser accepts the real server's CORS headers — is contract §12's open question 2, assigned by name to this task and carried out in step 10.

Facts from the shipped Go side override the UI contract where they disagree, and each is load-bearing here. **The error-code enum has sixteen server members, not fifteen**: Group C's ruling 7 added `non_loopback_publish`, and task 19's `reasonForCode` carries a `case "non_loopback_publish"` (its switch is deliberately not exhaustive — it falls through to the server's own message), so a fifteen-member union in this file is a `tsc` failure in a task that may not edit this one — its step 1 greps for the string and stops if it is missing. **`GET /v1/harnesses` returns `[]` and never `null`**: task 8's fix round made the Go side allocate unconditionally and pinned it with a marshalled-bytes test, so `models`, `efforts` and `permissionModes` arrive as arrays and this client writes no `?? []` over them. And **`SessionStatus` is four values**, so `ListSessionsFilter.status` is typed `SessionStatus[]` and a cockpit cannot send the server a `?status=` value it will answer `400 bad_request` to.

What follows shapes the code rather than the types. **There is no bearer token on this hop** — Go contract §3.16 lists five pre-upgrade checks on attach and none of them is a token — so this file sets no `Authorization` header anywhere, and step 2 pins that with an assertion rather than leaving it as an absence nobody checks. And **a dropped WebSocket destroys nothing**: destroy is an explicit kill, the idle TTL or an orphan reap, which is why `openAttach` is a plain constructor with no lifecycle of its own. The ref-counting, the retry ladder and the reconnect are task 19's registry, deliberately not here.

What this task does **not** build: `poller.ts`, which contract §4.7 describes in this section but §9 assigns to task 19, and every part of the attach lifecycle beyond handing back a socket with `binaryType` already set. The things the contract leaves to the client are decided here, each at the step that decides it: the client-raised `bad_request` for an empty rename — §4.4 states the refusal but not who makes it, and making it here is what keeps task 22's optimistic rename instant — the `unsupported_option` and `bad_request` refusals that `resumeSession` and `forkSession` respectively raise before spending a request, what a resume and a fork each carry forward from their source session, and the addition of `setWorktreeReady` for the `PATCH /v1/projects/{projectId}/worktrees/{worktreeId}` route that contract §10.2 requested and Group C's task 10 shipped — mounted at `control-plane/internal/api/catalog.go`, verified present.

Steps 1 to 9 run from `desktop/`: step 1 opens with `cd desktop` and every `npm` command after it assumes that directory. Steps 10 and 11 do not — step 10's greps and its `go run` are from the repo root, as its comments say, and its second terminal is back in `desktop/` for `npm run dev`, while step 11 stages repo-root-relative paths. This task runs `npm test` and `npm run typecheck` and nothing else — no Group D task runs `go test` or edits a `.go` file.

**Files:**
- `desktop/src/api/client.ts` — `DEFAULT_BASE_URL`, `ProblemCode`, `Problem`, `ApiError`, `ClientOptions`, `CockpitClient`, `createClient`, `attachUrl`, and one function per endpoint (new)
- `desktop/src/api/index.ts` — the app-wide `api` singleton and the package's re-exports (new)
- `desktop/src/api/client.test.ts` — the transport, every error path, every endpoint's URL, method, body and success status, and both seams (new)

**Interfaces:**
- Produces:
  - `DEFAULT_BASE_URL`, `DEFAULT_TIMEOUT_MS`, and the `ProblemCode` union — the Go contract §3.2's fifteen codes, Group C's sixteenth `non_loopback_publish`, and the two the client raises itself, `network_unavailable` and `timeout`
  - `ApiError` (a class, so `instanceof` works — task 19's poller branches on it), `Problem`, `Health`, `CreateSessionRequest`, `ListSessionsFilter`, `ClientOptions`, `CockpitClient`
  - `createClient(opts?: ClientOptions): CockpitClient` and `attachUrl(baseUrl: string, session: Session): string`
  - the singleton `api`, from `desktop/src/api/index.ts`, which also `export *`s the module above it so a consumer writes one import
- Consumes: task 15's `store/types.ts` — `Machine`, `Project`, `Branch`, `Worktree`, `Harness`, `Session`, `SessionStatus`, `Effort`, `PermissionMode`, and the id aliases — as **type-only** imports, so this module has no runtime edge to the store at all; task 14's runner (`vitest@5.0.1`, `globals: false`, jsdom) and its `src/test/fakes.ts`; and `src/test/fixtures.ts`, whose file is task 14's but whose six typed builders are **task 15's** — shared shape §3.1 puts them there because task 15 is the first task that can type them and the last that can add them before this task needs them. This task uses `aSession` and `aHarness`, passes overrides, and declares no builder of its own.
- Required by: task 17 (`toCreateRequest` produces this file's `CreateSessionRequest`), task 18 (the split New-session button calls `createSession`; the picker reads `listHarnesses`), task 19 (`openAttach`, `getSession`, `listSessions`, `ApiError`, `ProblemCode`, and the `api` singleton), task 20 (the pane's "did not start" state renders an `ApiError`), task 22 (every lifecycle verb), task 23 (the palette's commands call the same verbs), task 24 (mounts the poller and loads the catalogue through `api`).
- Amends contract §4.4 and §4.1. §4.4's table gains `setWorktreeReady(projectId, worktreeId, ready)` for the route §10.2 asked for and task 10 shipped as its adjudicated decision 2; §4.1's `CockpitClient` gains a readonly `baseUrl` so a diagnostic can print what the client is actually talking to. Both are additive; neither changes a signature §4 fixes.

- [ ] **Step 1: Preflight — confirm the two files this task types against**

Tasks 14 and 15 own everything this task imports, and a silent local patch to either is how Group C produced nine Criticals. The checks below are cheap. **If either fails, stop and report it to the orchestrator rather than creating the missing file here.**

```bash
cd desktop
# (a) Task 15's domain model must exist, with the four-member SessionStatus this client
#     types its filter against.
grep -cE '^export type SessionStatus = "starting" \| "running" \| "idle" \| "exited";' src/store/types.ts   # expect 1

# (b) Task 15's two typed builders must be in task 14's fixtures.ts. Step 6 calls each with an
#     override object — shared shape §3.1's "every later task imports them and declares none".
#     The ^ anchor is shared shape §3A's form: task 15 fills task 14's reserved point by
#     replacing the comment that named these builders, but an anchor costs nothing.
grep -cE '^export function (aSession|aHarness)' src/test/fixtures.ts   # expect 2

# (c) This task creates src/api/. Nothing may be there yet. `ls src/api 2>/dev/null` cannot do
#     this job: the "No such file or directory" it is meant to show is on the stderr the
#     redirect discards, so the run prints nothing at all and the check reads as a pass.
test -d src/api && echo 'PRESENT' || echo 'absent'   # expect: absent
```

`grep -c` prints a bare count on its own line, so (a) prints `1` and (b) prints `2`; (c) prints `absent`. All three were measured against a scratch copy of task 15's `types.ts` and of `fixtures.ts` in its post-task-15 state, not reasoned about — and unanchored, (b) reads `2` against that same file, because task 15 fills task 14's reserved point by replacing the comment that named these builders rather than leaving it above them. The `^` is kept anyway: shared shape §3A's rule is that a guard anchors, and this one costs nothing. A `0` from (a) means task 15 transcribed §2's block with a different status list, and every later switch in Group D is wrong, not just this one. Anything but `2` from (b) means task 15 did not fill task 14's reserved insertion point, or filled it under other names: **stop and report it to the orchestrator.** Shared shape §3.1 assigns those six builders to task 15 by name and §3.2 forbids a consumer declaring a seventh, so writing a local `aSession` here would collide with task 15's the moment both land — which is the defect this check exists to catch, not one to work around.

- [ ] **Step 2: Write the failing tests for the transport and the catalogue reads**

This file tests the client the way the control plane will be used and not the way it is implemented: every assertion is about a URL, a method, a body, a status code or a thrown `ApiError`. The fetch double is local to this file on purpose. Task 14's `fakes.ts` is a shared file and shared-shape rule 3 keeps it that way for the fakes many tasks need — `FakeTerm` and `FakeWebSocket` — but only this task tests transport, and a `fakeFetch` in the shared file would be a fake every later task carries and none of them uses. `FakeWebSocket` is a different matter: the contract's §6 file tree puts it in `fakes.ts` beside `FakeTerm`, and §4.1 is what makes it shared — it is handed to the client as the `webSocket` option — so step 6 imports it rather than writing a second one.

Two details of the double that will otherwise cost an hour. `new Response(body, { status: 204 })` **throws**, because 204 is a null-body status. The wording is runtime-specific and neither is worth memorising: on this repo's Node — v24.15.0, measured — it is `TypeError: Response constructor: Invalid response status code 204`, while a Chromium webview words the same refusal `Response with null body status cannot have body`. Either way, 204 replies must be built as `new Response(null, { status: 204 })`, which is exactly the shape `purgeSession` has to survive. And the client is written to touch only `res.ok`, `res.status`, `res.statusText` and `res.text()`, so if a future environment turns out not to expose a global `Response`, a hand-rolled object literal cast to `Response` is a drop-in for `jsonReply` — contract §4.1 states that vitest's jsdom environment leaves Node's fetch stack in place, and that is what these tests rely on.

Create `desktop/src/api/client.test.ts`:

```ts
import { describe, expect, it, vi } from "vitest";
import {
  ApiError,
  createClient,
  DEFAULT_BASE_URL,
  type ProblemCode,
} from "./client";

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
```

- [ ] **Step 3: Run the tests and watch them fail to collect**

```bash
npm test -- src/api/client.test.ts
```

**A caveat, stated rather than papered over.** The toolchain is installed by task 14 and was not present when this plan was written, so the transcript below was not captured from a run. What is certain is the *shape*: vitest cannot collect a file whose import does not resolve, so this is a **collection error** and not a failing assertion — zero tests run, and the summary reports one failed test file with no test count. The resolution message comes from Vite's import analysis and is one of two wordings depending on where resolution gives up:

```
 FAIL  src/api/client.test.ts [ src/api/client.test.ts ]
Error: Failed to resolve import "./client" from "src/api/client.test.ts". Does the file exist?
```

The other is `Failed to load url /src/api/client.ts (resolved id: ...). Does the file exist?`. Either is the correct red. What would be **wrong** is `Test Files  no tests`, which means the file landed outside `src/` and `include: ["src/**/*.test.{ts,tsx}"]` never matched it, or a green run, which means `client.ts` already exists and this task is not starting from zero.

- [ ] **Step 4: Write the transport and the catalogue reads**

What follows is where this file decides something rather than transcribing it, and each one is a defect avoided rather than a preference.

**The default `fetch` is wrapped in an arrow, not passed by reference.** `const f = globalThis.fetch; f(url)` throws `TypeError: Illegal invocation` in a Chromium webview, because the platform `fetch` checks its receiver. `(input, init) => globalThis.fetch(input, init)` costs one closure and removes a failure that would only ever appear in the packaged app, never in a test.

**The timeout is an `AbortController` plus `setTimeout`, not `AbortSignal.timeout`.** The static is in the DOM lib TypeScript sees, but it is not in every runtime this code meets, and — the real reason — a client that aborts through it cannot tell its own timeout from a caller's cancellation. The `timedOut` flag is what makes `timeout` and `network_unavailable` two different sentences to the user, which contract §4.2 keeps as two separate codes and which a shared `AbortError` cannot distinguish.

**A non-2xx is read as text first and parsed second.** `res.json()` on a proxy's HTML error page throws `SyntaxError` out of the `catch` that was written for `ApiError`, and the caller sees an exception with no `code` at all. Reading text always succeeds, and the parse is then allowed to fail into the `internal` branch contract §4.2 specifies.

**`Health.status` is typed `"ok"` and is not checked at runtime.** Contract §4.3 fixes the literal; the field is decoration on a 200 and `backendReady` is the field that carries the information, so validating it would add a failure mode that tells the user nothing.

Create `desktop/src/api/client.ts`:

```ts
// desktop/src/api/client.ts
//
// The typed control-plane client — contract §4. One exported function per Go endpoint;
// paths, field names and status codes are copied from the Go contract §3 and never re-derived.
//
// There is no Authorization header in this file and there is not meant to be one before
// Phase 8 (contract §4.5, Go contract §3.16). The boundary on the desktop hop is a loopback
// bind, a Host guard and an Origin allowlist, and all three live server-side.

import type {
  Branch,
  BranchId,
  Harness,
  Machine,
  Project,
  ProjectId,
  Worktree,
  WorktreeId,
} from "../store/types";

/** No trailing slash. Overridden in dev by import.meta.env.VITE_PANACEA_APP_API. */
export const DEFAULT_BASE_URL = "http://127.0.0.1:8730";

/** Per-request. /attach is not a fetch and is unaffected. */
export const DEFAULT_TIMEOUT_MS = 10_000;

/** The sixteen codes the control plane emits — Go contract §3.2's fifteen plus the
 *  non_loopback_publish that Group C's ruling 7 added — and the two the client raises itself.
 *
 *  non_loopback_publish is NOT a "not running" state: the sandbox is running and published its
 *  port off loopback, so any exhaustive switch over this union must give it its own sentence. */
export type ProblemCode =
  | "invalid_json" | "bad_request" | "unknown_harness" | "unsupported_option"
  | "worktree_not_found" | "project_not_found" | "session_not_found"
  | "forbidden_origin" | "forbidden_host" | "conflict" | "sandbox_not_running"
  | "harness_unavailable" | "upgrade_required" | "backend_unavailable" | "internal"
  | "non_loopback_publish"
  /* client-raised */
  | "network_unavailable"   // fetch rejected: the control plane is not running
  | "timeout";              // the request exceeded timeoutMs

export interface Problem {
  error: ProblemCode;
  message: string;
  details?: Record<string, string>;
}

/** Every method of this client rejects with this and nothing else, so a caller writes one
 *  error branch. Task 19's poller and attach registry both narrow with `instanceof`. */
export class ApiError extends Error {
  readonly code: ProblemCode;
  /** 0 for network_unavailable, timeout, and the refusals the client raises before sending. */
  readonly status: number;
  readonly details: Record<string, string>;

  constructor(code: ProblemCode, message: string, status: number, details?: Record<string, string>) {
    super(message);
    this.name = "ApiError";
    this.code = code;
    this.status = status;
    this.details = details ?? {};
    // Cheap insurance: if any toolchain in the pipeline ever downlevels this class,
    // `instanceof` stops working without it, silently, in task 19's poller.
    Object.setPrototypeOf(this, ApiError.prototype);
  }
}

export interface Health {
  status: "ok";
  version: string;
  backend: string;
  /** false means the control plane is up but Docker is not. HTTP is still 200 (Go contract
   *  §3.3), so this field is the signal and the status code is not. */
  backendReady: boolean;
}

export interface ClientOptions {
  /** No trailing slash needed; one is trimmed if present. */
  baseUrl?: string;
  /** Injected so a unit test never touches the network and never patches a global. */
  fetch?: typeof globalThis.fetch;
  /** Injected for the same reason: jsdom's WebSocket opens a real socket. */
  webSocket?: (url: string) => WebSocket;
  /** 0 disables. Defaults to DEFAULT_TIMEOUT_MS. */
  timeoutMs?: number;
}

export interface CockpitClient {
  /** What this client is actually talking to. An addition to §4.1, so a diagnostic and the
   *  attach URL builder both read it from one place. */
  readonly baseUrl: string;

  health(): Promise<Health>;
  listMachines(): Promise<Machine[]>;
  listProjects(): Promise<Project[]>;
  listBranches(projectId: ProjectId): Promise<Branch[]>;
  listWorktrees(projectId: ProjectId, branchId?: BranchId): Promise<Worktree[]>;
  /** contract §10.2, shipped by task 10 as its adjudicated decision 2. Answers with the
   *  worktree object itself, not an envelope. */
  setWorktreeReady(projectId: ProjectId, worktreeId: WorktreeId, ready: boolean): Promise<Worktree>;
  listHarnesses(): Promise<Harness[]>;

  // ── step 8 of this task adds the session surface here: createSession, listSessions,
  //    getSession, renameSession, restartSession, killSession, purgeSession, resumeSession,
  //    forkSession, attachUrl, openAttach. Nothing outside task 16 inserts into this
  //    interface — a later task that needs a derived read adds a selector (shared-shape §4).
}

type Method = "GET" | "POST" | "PATCH" | "DELETE";

/** opts.baseUrl → import.meta.env.VITE_PANACEA_APP_API → DEFAULT_BASE_URL, per §4.1. The env
 *  var is statically replaced by Vite at build time and is undefined under vitest, which is
 *  exactly why the explicit option is first. */
function resolveBaseUrl(explicit?: string): string {
  const fromEnv: unknown = import.meta.env?.VITE_PANACEA_APP_API;
  const raw = explicit ?? (typeof fromEnv === "string" && fromEnv !== "" ? fromEnv : DEFAULT_BASE_URL);
  return raw.endsWith("/") ? raw.slice(0, -1) : raw;
}

/** Non-2xx → ApiError. Read as text first: res.json() on a proxy's HTML page throws a
 *  SyntaxError the caller's `catch (e) { const err = e as ApiError }` cannot make sense of. */
async function readProblem(res: Response): Promise<ApiError> {
  const text = await res.text().catch(() => "");
  try {
    const parsed: unknown = JSON.parse(text);
    if (
      typeof parsed === "object" &&
      parsed !== null &&
      typeof (parsed as Problem).error === "string" &&
      typeof (parsed as Problem).message === "string"
    ) {
      const problem = parsed as Problem;
      return new ApiError(problem.error, problem.message, res.status, problem.details);
    }
  } catch {
    // Not JSON: a proxy interposed, or the server died before it wrote a body.
  }
  return new ApiError("internal", text.slice(0, 200) || res.statusText, res.status);
}

async function readJson<T>(res: Response): Promise<T> {
  const text = await res.text();
  try {
    return JSON.parse(text) as T;
  } catch {
    throw new ApiError(
      "internal",
      `The control plane answered ${res.status} with a body that is not JSON: ${text.slice(0, 200)}`,
      res.status,
    );
  }
}

export function createClient(opts?: ClientOptions): CockpitClient {
  const baseUrl = resolveBaseUrl(opts?.baseUrl);
  const timeoutMs = opts?.timeoutMs ?? DEFAULT_TIMEOUT_MS;
  // Wrapped, not passed by reference: an extracted platform fetch throws
  // "TypeError: Illegal invocation" in a Chromium webview, and never in a test.
  const doFetch =
    opts?.fetch ??
    ((input: RequestInfo | URL, init?: RequestInit): Promise<Response> => globalThis.fetch(input, init));

  async function send(method: Method, route: string, body?: unknown): Promise<Response> {
    const url = baseUrl + route;
    const controller = new AbortController();
    // Our own flag rather than reading the abort reason: it is the only way to tell this
    // client's timeout from any other abort, and §4.2 keeps them two separate codes.
    let timedOut = false;
    const timer =
      timeoutMs > 0
        ? setTimeout(() => {
            timedOut = true;
            controller.abort();
          }, timeoutMs)
        : undefined;
    try {
      const res = await doFetch(url, {
        method,
        signal: controller.signal,
        headers: body === undefined ? undefined : { "Content-Type": "application/json" },
        body: body === undefined ? undefined : JSON.stringify(body),
      });
      if (!res.ok) throw await readProblem(res);
      return res;
    } catch (cause) {
      if (cause instanceof ApiError) throw cause;
      if (timedOut) {
        throw new ApiError("timeout", `The control plane did not answer ${method} ${url} within ${timeoutMs} ms.`, 0);
      }
      throw new ApiError("network_unavailable", `The control plane could not be reached at ${url}.`, 0, {
        cause: String(cause),
      });
    } finally {
      if (timer !== undefined) clearTimeout(timer);
    }
  }

  const get = async <T,>(route: string): Promise<T> => readJson<T>(await send("GET", route));
  const project = (id: ProjectId): string => `/v1/projects/${encodeURIComponent(id)}`;

  return {
    baseUrl,
    health: () => get<Health>("/v1/healthz"),
    listMachines: async () => (await get<{ machines: Machine[] }>("/v1/machines")).machines,
    listProjects: async () => (await get<{ projects: Project[] }>("/v1/projects")).projects,
    listBranches: async (projectId) =>
      (await get<{ branches: Branch[] }>(`${project(projectId)}/branches`)).branches,
    listWorktrees: async (projectId, branchId) => {
      const query = branchId === undefined ? "" : `?branchId=${encodeURIComponent(branchId)}`;
      return (await get<{ worktrees: Worktree[] }>(`${project(projectId)}/worktrees${query}`)).worktrees;
    },
    setWorktreeReady: async (projectId, worktreeId, ready) =>
      readJson<Worktree>(
        await send("PATCH", `${project(projectId)}/worktrees/${encodeURIComponent(worktreeId)}`, { ready }),
      ),
    listHarnesses: async () => (await get<{ harnesses: Harness[] }>("/v1/harnesses")).harnesses,
    // ── step 8 adds the session methods here.
  };
}
```

The trailing comma in `<T,>` is not a typo: in a `.ts` file it is unnecessary, but it is the form that survives a later rename to `.tsx`, where `<T>` parses as JSX. Leave it.

- [ ] **Step 5: Run the transport tests to verify they pass**

```bash
npm test -- src/api/client.test.ts
npm run typecheck
```

Sixteen passing tests in one file, and `tsc` silent. Vitest's default reporter prints a `✓` line per file and a summary block; the per-test lines appear only under `--reporter=verbose`, so a green run of one file is short:

```
 ✓ src/api/client.test.ts (16 tests) 31ms

 Test Files  1 passed (1)
      Tests  16 passed (16)
```

The counts are the assertion — durations, the `Start at` and `Duration` lines and the transform/collect breakdown vary per machine, and the same caveat as step 3 applies to the exact reporter layout. The failures worth naming in advance each map to a specific drift. `timeout` arriving as `network_unavailable` means the `timedOut` flag was read after `clearTimeout` reset it, or the `instanceof ApiError` re-throw was omitted so the `ApiError` thrown by `readProblem` fell into the network branch. An `internal` where `session_not_found` was expected means `readProblem` checked `res.headers` for a JSON content type instead of just parsing — the control plane always sends `application/json; charset=utf-8`, but a proxy in front of it does not, and the body is the evidence, not the header.

- [ ] **Step 6: Write the failing tests for the session surface**

Three of these tests pin a refusal the client makes **without spending a request**, and the assertion in each is `calls` staying empty. That is the part worth writing first: a rename that posts `{"title":"   "}` gets a truthful `400 bad_request` from the server, so the feature works either way — but it works by burning a round trip on a state the client already knew was invalid, and the Undo window in task 22 is built on the optimistic mutation happening immediately.

Add to the import block at the top of `desktop/src/api/client.test.ts`:

```ts
import { FakeWebSocket } from "../test/fakes";
import { aHarness, aSession } from "../test/fixtures";
import type { Harness, Session, SessionStatus } from "../store/types";
import { attachUrl } from "./client";
```

Then append to the same file:

```ts
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

  it("forks into another worktree and lets the server derive the new title", async () => {
    const { fn, calls } = fetchStub([jsonReply(201, sessionBody("s-3", "starting"))]);

    await createClient({ fetch: fn }).forkSession(source, "w-2", claudeCode);

    const sent = JSON.parse(String(calls[0].init?.body)) as Record<string, unknown>;
    expect(sent.worktreeId).toBe("w-2");
    expect(sent.title).toBeUndefined();
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
```

- [ ] **Step 7: Run the new tests and watch them fail for the right reason**

```bash
npm test -- src/api/client.test.ts
npm run typecheck
```

**Read this before running it, because the red here is not the usual red.** Vitest transpiles TypeScript with esbuild and does not type-check, so a call to a method that does not exist yet is a *runtime* failure, not a compile one. Two outcomes are both correct and which one you get depends on whether the module runner validates named imports: either the file fails to collect with `SyntaxError: [vite] The requested module '/src/api/client.ts' does not provide an export named 'attachUrl'`, or it collects and the sixteen new tests fail individually with `TypeError: client.createSession is not a function` and its siblings while the original sixteen still pass. I could not run vitest 5 to settle which — do not treat the wording as the assertion.

What **is** certain, and is the assertion, is `npm run typecheck`. It fails with **eighteen diagnostics** — one `TS2305` for `attachUrl`, and seventeen `TS2339`s naming the ten members the tests call, each once per call site. Measured: this exact state was compiled with the repo's own `node_modules/.bin/tsc` (TypeScript 6.0.3) under `desktop/tsconfig.json`'s `compilerOptions`. The first and last of the eighteen:

```
src/api/client.test.ts(NN,10): error TS2305: Module '"./client"' has no exported member 'attachUrl'.
src/api/client.test.ts(NN,55): error TS2339: Property 'createSession' does not exist on type 'CockpitClient'.
...
src/api/client.test.ts(NN,27): error TS2339: Property 'openAttach' does not exist on type 'CockpitClient'.
```

The ten named by `TS2339` are `createSession`, `listSessions`, `getSession`, `renameSession`, `restartSession`, `killSession`, `purgeSession`, `resumeSession`, `forkSession` and `openAttach` — the eleventh member, `CockpitClient.attachUrl`, is not called by any test (the tests call the module-level `attachUrl`, which is the `TS2305`), so step 8 must add it from the interface block and not from this list. The codes and the member names are the assertion. The **line** depends on how this file was transcribed and is shown as `NN`; the **column** does not, because it points into a line this plan fixes verbatim — 10 at `attachUrl` in the import, 55 at `.createSession(`, 27 at `.openAttach(`. npm prints its diagnostics in `tsc`'s pretty form with a ` - ` between location and code when stdout is a TTY, and npm appends its own `npm error Lifecycle script ... failed` block on the non-zero exit. A run in which `typecheck` passes means `client.ts` already carries the session surface and this step was reached out of order.

- [ ] **Step 8: Write the session surface, the resume and fork wrappers, and the attach URL**

Every decision in this step is a place contract §4.4 names a function without saying what it carries.

**Resume keeps the title and drops the opening prompt.** A resumed session continues the same work in the same worktree, so the title still reads true; the opening prompt was consumed by the session being resumed and re-sending it would make the harness re-run the instruction the user already gave. **Fork drops the title and keeps the opening prompt.** The title the server derives is `harness name · worktree name`, and carrying the old one names the wrong worktree in the rail — while the prompt is the thing the user wants tried against the other branch, which is what fork is for.

**`resume` is sent as the source session's control-plane id.** That is the only handle the cockpit has, and it is worth being explicit that this is not yet proved end to end: Go contract §2.9's argv table maps a non-empty `resume` to `--resume <value>`, and whether the control plane translates its own `s-…` id into the harness's transcript id, or passes it through verbatim to a CLI that will not recognise it, is not stated anywhere in the Go contract. **Raise it rather than resolve it** — the client cannot invent a second identifier, and task 22 must render the failure honestly if it comes back as an error. This plan flags it; it does not paper over it.

**The two pre-flight refusals cost nothing and buy an immediate answer.** `resumeSession` on a harness whose `supportsResume` is false raises `unsupported_option`, which is the same code the server would answer with; `forkSession` into the source's own worktree raises `bad_request`, which the server would not catch at all — it would happily create a second session in the same worktree and the user would wonder what fork did.

First, extend the type-only import at the top of `desktop/src/api/client.ts` with the names the session surface needs — `Effort`, `HarnessId`, `MachineId`, `ModelId`, `PermissionMode`, `Session`, `SessionId` and `SessionStatus` — keeping the list alphabetical:

```ts
import type {
  Branch,
  BranchId,
  Effort,
  Harness,
  HarnessId,
  Machine,
  MachineId,
  ModelId,
  PermissionMode,
  Project,
  ProjectId,
  Session,
  SessionId,
  SessionStatus,
  Worktree,
  WorktreeId,
} from "../store/types";
```

Then add the two request shapes, immediately after the `Health` interface:

```ts
export interface CreateSessionRequest {
  worktreeId: WorktreeId;
  machineId?: MachineId;
  harness?: HarnessId;
  model?: ModelId;
  effort?: Effort;
  permissionMode?: PermissionMode;
  openingPrompt?: string;
  title?: string;
  /** The session whose transcript this one continues. See the decision in the plan. */
  resume?: string;
  contextRef?: string;
}

export interface ListSessionsFilter {
  worktreeId?: WorktreeId;
  /** Repeated as ?status=running&status=idle, per Go contract §3.11. Four values and only
   *  four: a fifth is 400 bad_request server-side, which is why this is typed. */
  status?: SessionStatus[];
}

/** ws(s):// + the session's own attachPath. The path is taken from the session object and
 *  NOT reassembled from the id, so a server-side route change is one server change. */
export function attachUrl(baseUrl: string, session: Session): string {
  const base = new URL(baseUrl);
  base.protocol = base.protocol === "https:" ? "wss:" : "ws:";
  // attachPath is absolute and already begins with "/v1/". Resolving it against the base
  // discards the base's own path, which is what we want and what string concatenation
  // would get wrong for a baseUrl carrying a path.
  return new URL(session.attachPath, base).toString();
}
```

Replace the reserved comment inside `CockpitClient` with the members it named:

```ts
  createSession(req: CreateSessionRequest): Promise<Session>;
  /** Server-sorted createdAt descending. The client never re-sorts — the poll must move
   *  zero pixels (§4.7). */
  listSessions(filter?: ListSessionsFilter): Promise<Session[]>;
  getSession(id: SessionId): Promise<Session>;
  renameSession(id: SessionId, title: string): Promise<Session>;
  restartSession(id: SessionId): Promise<Session>;
  /** Idempotent: killing an exited session is a 200, not an error. */
  killSession(id: SessionId): Promise<Session>;
  /** 204, no body. */
  purgeSession(id: SessionId): Promise<void>;
  /** Resume: a NEW session continuing an old one's context. The old session keeps its record
   *  and its transcript. Only offered when harness.supportsResume. */
  resumeSession(source: Session, harness: Harness): Promise<Session>;
  /** Fork: the same configuration in a DIFFERENT worktree. Phase 2 cannot create a worktree
   *  (§10.3), so it is offered only when the branch already has another one. */
  forkSession(source: Session, targetWorktreeId: WorktreeId, harness: Harness): Promise<Session>;
  attachUrl(session: Session): string;
  openAttach(session: Session): WebSocket;
```

> **AMENDED by the fork-carries fix (2026-09-24; UI contract §4.4, Go contract §3.9).** Fork as
> planned here started a fresh session and did not carry the conversation. For a harness with
> `supportsResume`, `forkSession` now also sends `resume: from.id` and no `openingPrompt`; a shell's
> fork is as below. `client.ts` is the source of truth.

Add these locals immediately before the `return {` in `createClient`:

```ts
  const makeSocket = opts?.webSocket ?? ((url: string) => new WebSocket(url));
  const sessionPath = (id: SessionId): string => `/v1/sessions/${encodeURIComponent(id)}`;
  // A local const rather than a property read, so resumeSession and forkSession call it
  // without `this` — the returned object is a literal and has no stable receiver.
  const createSession = async (req: CreateSessionRequest): Promise<Session> =>
    readJson<Session>(await send("POST", "/v1/sessions", req));
```

And replace the reserved comment inside the returned object with the methods themselves:

```ts
    createSession,
    listSessions: async (filter) => {
      const params = new URLSearchParams();
      if (filter?.worktreeId !== undefined) params.append("worktreeId", filter.worktreeId);
      for (const status of filter?.status ?? []) params.append("status", status);
      const query = params.toString();
      return (await get<{ sessions: Session[] }>(`/v1/sessions${query === "" ? "" : `?${query}`}`)).sessions;
    },
    getSession: (id) => get<Session>(sessionPath(id)),
    renameSession: async (id, title) => {
      const trimmed = title.trim();
      if (trimmed === "") {
        // The server answers 400 for this too. Raising it here keeps task 22's optimistic
        // rename instant and costs the user no round trip to be told what we already knew.
        throw new ApiError("bad_request", "A session title cannot be empty.", 0, { id });
      }
      return readJson<Session>(await send("PATCH", sessionPath(id), { title: trimmed }));
    },
    restartSession: async (id) => readJson<Session>(await send("POST", `${sessionPath(id)}/restart`)),
    killSession: async (id) => readJson<Session>(await send("DELETE", sessionPath(id))),
    purgeSession: async (id) => {
      // 204 and no body. A res.json() here throws SyntaxError out of a catch written for
      // ApiError, and the caller sees an exception with no code at all.
      await send("DELETE", `${sessionPath(id)}?purge=true`);
    },
    resumeSession: async (from, harness) => {
      if (!harness.supportsResume) {
        throw new ApiError("unsupported_option", `${harness.name} cannot resume a session.`, 0, {
          harness: harness.id,
        });
      }
      return createSession({
        worktreeId: from.worktreeId,
        machineId: from.machineId,
        harness: from.harness,
        model: from.model,
        effort: from.effort,
        permissionMode: from.permissionMode,
        // Same work, same worktree: the title still reads true. The opening prompt does not
        // travel — the session being resumed already consumed it.
        title: from.title,
        resume: from.id,
        contextRef: from.contextRef,
      });
    },
    forkSession: async (from, targetWorktreeId, harness) => {
      if (targetWorktreeId === from.worktreeId) {
        throw new ApiError(
          "bad_request",
          "A fork needs a different worktree from the one the session is already in.",
          0,
          { worktreeId: targetWorktreeId },
        );
      }
      return createSession({
        worktreeId: targetWorktreeId,
        machineId: from.machineId,
        harness: from.harness,
        model: from.model,
        effort: from.effort,
        permissionMode: from.permissionMode,
        // No title: the server derives "harness name · worktree name", and the source's title
        // names the worktree the fork is NOT in.
        openingPrompt: harness.supportsOpeningPrompt ? from.openingPrompt : undefined,
        contextRef: from.contextRef,
      });
    },
    attachUrl: (session) => attachUrl(baseUrl, session),
    openAttach: (session) => {
      const socket = makeSocket(attachUrl(baseUrl, session));
      // Before returning, so no caller can read a frame as a Blob. The browser sends Origin
      // on the handshake by itself; nothing is added by hand, and there is no token.
      socket.binaryType = "arraybuffer";
      return socket;
    },
```

Finally, create `desktop/src/api/index.ts`:

```ts
// desktop/src/api/index.ts
//
// The app-wide client, constructed once (contract §4.1). Components and the store import
// `api` from here and never call createClient themselves: a second client is a second base
// URL and a second timeout, and in dev one of them is always the wrong one. Tests build
// their own with createClient({ fetch, webSocket }) and never touch this singleton.

export * from "./client";

import { createClient } from "./client";

export const api = createClient();
```

- [ ] **Step 9: Run the whole suite and the type checker**

```bash
npm test
npm run typecheck
```

This task contributes **three files and 32 tests** — sixteen from step 2 and sixteen from step 6, all in one file. Tasks 14 and 15 run in the same pass and their counts are theirs, so the acceptance criterion is *no `FAIL` line anywhere* plus this line:

```
 ✓ src/api/client.test.ts (32 tests) 48ms
```

`npm run typecheck` prints nothing but npm's banner.

**What was and was not verified while writing this plan.** Both source files were compiled with this repository's own `node_modules/.bin/tsc` — TypeScript **6.0.3** — under `desktop/tsconfig.json`'s exact `compilerOptions` (`strict`, `noUnusedLocals`, `noUnusedParameters`, `isolatedModules`, `useDefineForClassFields`, `target: ES2020`, `lib: ["ES2020","DOM"]`), against a verbatim copy of contract §2's `types.ts`, and exited 0. The test file was compiled the same way, with `vitest` and task 14's `FakeWebSocket` as declaration-only stand-ins and task 15's `aSession` and `aHarness` as real builders over §2's types, and also exited 0 — so every key of the `aSession` override and of the two `aHarness` overrides in step 6 is a real field of §2's type, correctly spelt and correctly typed, and an excess-property check would have caught any that were not. Every **expected value** in both test blocks was then executed against the real `client.ts` under Node v24.15.0 outside vitest: **all passing**, including the `ws://`/`wss://` conversion, the exact `create` body with `title: undefined` dropped, the repeated `?status=` query, the `PATCH .../worktrees/{id}` body and read-back, the 204 that returns `undefined`, the `timeout`-versus-`network_unavailable` split, the `session_not_found` and HTML-body error shapes, `openAttach` setting `binaryType` before it returns, and all three no-request refusals — including the `Shell cannot resume a session.` message, which reads `harness.name` off the `aHarness` override rather than off a local literal. Both intermediate states were compiled as well: the step-7 state (step-4 `client.ts`, whole test file) produced exactly the eighteen diagnostics that step quotes, and the step-8 state produced none. What was **not** run is vitest itself, because task 14 installs it — so treat every `✓`/`Test Files`/`Tests` block here as the reporter's documented shape and the counts as the assertion, not as a captured transcript.

- [ ] **Step 10: Close contract §12's open question 2, or record honestly that you could not**

Contract §12 assigns this to task 16 by name: the CORS behaviour in §10.1 was reasoned from the browser's model and never observed against a running server, and **task 24 builds on the answer**. The two halves the observation needs are in different states, so **measure before running anything**:

```bash
# From the repo root. Both are reads: no `go test`, no `.go` file edited.
grep -cE '^[[:space:]]*w\.Header\(\)\.Set\("Access-Control-Allow-Origin"' \
     control-plane/internal/httpx/origin.go     # expect 1 — task 9's grant
grep -cE '^[[:space:]]*(ln, err := net\.Listen|go func\(\) \{ serveErr <- srv\.Serve)' \
     control-plane/cmd/api/main.go                 # expect 2 — task 13's listener
```

Both are anchored on purpose: the unanchored `grep -c 'Access-Control-Allow-Origin'` reads **3** against that same correct file, because the other two hits are comments (measured; shared shape §3A).

**This step was rewritten after task 16's implementer measured it.** It used to grep for `ListenAndServe` and expect `0`, reading that `0` as proof that "`main.go` is a seven-line stub, nothing serves it, stop here". The count is genuinely `0` — and the conclusion is false. Task 13 shipped a fully wired control plane that binds with `net.Listen` and serves with `srv.Serve(ln)` (`main.go:139` and `:154`), deliberately, so it can log the address it actually bound. A guard whose **passing** value means the opposite of what its comment claims is worse than no guard: this one would have declared contract §12's open question 2 unanswerable against a server that runs, and handed a closed question to task 24 as an open one.

The replacement greps for what the listener really is. If it reads `2`, the binary listens and you can answer §12 Q2 here rather than deferring it.

Task 9 shipped the CORS grant and task 10 mounted the routes; task 13 wired the `main` that serves them. So the chain is complete and the question is measurable: start the binary, and check both halves — the **grant** on a simple request, and the **preflight**, which is the half a browser reaches first for any `POST`/`PATCH`/`DELETE` carrying `Content-Type: application/json`. Record what you measured, not what you expect. What remains after that is one real Chromium window, which is task 24's and the owner's eyes.

**If it reads `1` or more**, a later Group C task has shipped the server and the observation is one request:

```bash
# Terminal 1 — from the repo root. This runs the Go binary; it is not `go test` and it
# edits no .go file, which is what Group D is barred from.
cd control-plane && go run ./cmd/api

# Terminal 2 — from desktop/
npm run dev
```

In the dev webview at `http://localhost:1420`, open the devtools console and run `await (await fetch("http://127.0.0.1:8730/v1/machines")).json()`. Then read the **network** panel, not the console: the response must carry `Access-Control-Allow-Origin: http://localhost:1420` echoed verbatim — not `*` — and `Vary: Origin`.

Each outcome has a different action. It succeeds with those headers: §10.1 is confirmed, say so in the commit message. It fails with `No 'Access-Control-Allow-Origin' header is present on the requested resource`: task 9's middleware is not reaching this route, and that is a **Group C** defect to report, not something to work around by adding `@tauri-apps/plugin-http` here. It succeeds *without* the headers: §12's open question 2 says explicitly that my reasoning was wrong and the section should be struck — say so rather than quietly deleting it.

- [ ] **Step 11: Commit**

```bash
git add desktop/src/api/client.ts \
        desktop/src/api/index.ts \
        desktop/src/api/client.test.ts
git commit -m "feat(cockpit): the typed control-plane client

One exported function per Go endpoint, so no component knows a URL, a status
code or a wire field name. The two shapes that break a generic client are
covered by name: createSession is 201 and not 200, and purgeSession is 204
with no body, where a res.json() would throw a SyntaxError out of a catch
written for ApiError and the user would see nothing at all.

fetch and WebSocket are injected. That is what lets every later Group D task
test against this client with no control plane running, and it is only
possible because Group C shipped the CORS grant the UI contract asked for in
section 10.1 - the @tauri-apps/plugin-http fallback would have moved fetch
inside a Tauri window, where no unit test can reach it.

ProblemCode carries eighteen members: the fifteen of the Go contract
section 3.2, the non_loopback_publish that Group C's ruling 7 added, and
the two the client raises itself. network_unavailable and timeout stay
separate because they are different sentences to the user, and
non_loopback_publish is not a 'not running' state - that sandbox is
running and published off loopback.

Harness arrays arrive as [] and never null, pinned Go-side by task 8, so
there is no defensive ?? [] anywhere in this file.

resume sends the source session's control-plane id, which is the only handle
the cockpit has. Whether the control plane translates it into the harness's
own transcript id is not stated in the Go contract; raised for task 22
rather than resolved here.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```
