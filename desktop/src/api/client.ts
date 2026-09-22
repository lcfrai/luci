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

  const makeSocket = opts?.webSocket ?? ((url: string) => new WebSocket(url));
  const sessionPath = (id: SessionId): string => `/v1/sessions/${encodeURIComponent(id)}`;
  // A local const rather than a property read, so resumeSession and forkSession call it
  // without `this` — the returned object is a literal and has no stable receiver.
  const createSession = async (req: CreateSessionRequest): Promise<Session> =>
    readJson<Session>(await send("POST", "/v1/sessions", req));

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
  };
}
