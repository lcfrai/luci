### Task 13: Desktop data layer — types, client, slice, cursor-driven fetch, `upstream-down`

**Half:** 7a. **Hours:** 3. **Depends on:** 08. **Elsewhere:** Hub C2's poller cursor hook (ask
HA7-4); phase 4's key-source loader (ruling R6); Hub C3's local key in `client.ts`.

Everything the Deploy lens reads, and nothing it draws. It adds **no `setInterval`, no per-row
`useEffect` fetch, and nothing to `partialize`** (plan §5 rules 7 and 8; 7P-13). A fetch happens on
exactly three edges: the deploy cursor changed on a poll tick, the selected project changed, or the
Deploy lens was opened. It is phase 2's poller that ticks; this code only listens.

**Files:**
- `desktop/src/deploy/types.ts` (new) — the wire types below, complete
- `desktop/src/deploy/api.ts` (new) — `DeployApi`, `createDeployApi`
- `desktop/src/deploy/deploySlice.ts` (new) — the slice
- `desktop/src/deploy/loader.ts` (new) — `onDeployCursor`, `loadDeployView`
- `desktop/src/deploy/*.test.ts` (new)
- `desktop/src/api/client.ts` (modified) — `Method` gains `"PUT"`; `send` gains `headers`; `CockpitClient.request`; the new `ProblemCode`s
- `desktop/src/api/problems.ts` (modified) — `upstream-down`, **only if phase 5 has not added it** (ruling R4)
- the four `classify` call sites (modified, same condition) — `components/develop/EmptyDevelop.tsx:57`, `components/develop/SessionPane.tsx:48`, `components/develop/sessionVerbs.ts:85`, `hooks/useCreateSession.ts:38`
- `desktop/src/api/poller.ts` (modified, only if Hub C2 has not built HA7-4's `onCursor`)
- `desktop/src/store/store.ts` (modified) — one import and one spread for the slice; **`partialize` untouched**
- `desktop/src/store/selectors.ts` (modified) — `selectDeployView`, `selectReleasePinKeys`

#### Wire types — complete (`types.ts`)

```ts
// Every type mirrors a control-plane route of phase 7 (parts 05, 06, 08-12), field for field.
// Nothing here names Panacea: the control plane translated everything (plan 7P-14).

export type ConnectionState =
  | "not_connected" | "connected" | "awaiting_approval" | "token_rejected" | "unreachable";

export interface ConnectionStatus {
  state: ConnectionState;
  origin: string | null;
  originLocked: boolean;
  apiVersion: string | null;
  account: { username: string; email: string; admin: boolean } | null;
  message: string;
  checkedAt: string | null;
}

export type BindingState =
  | "bound" | "no_origin" | "unknown_host" | "no_match" | "ambiguous" | "no_repo_info"
  | "not_connected" | "unavailable";

export interface DeployProjectChoice { ref: string; displayName: string; repoUrl: string }

export interface Binding {
  projectId: string;
  repoKey: string | null;
  hubProjectId: string | null;
  panaceaProject: string | null;
  state: BindingState;
  chosenBy: "auto" | "local" | null;
  candidates: DeployProjectChoice[];
  message: string;
}

export type BuildState = "succeeded" | "failed" | "running" | "pending" | "historical" | "unknown";

export interface Gate { verdict: string; criticals: number | null; highs: number | null; findings: number | null }
export interface ProvisioningNote { component: string; status: string; detail: string }
export interface ScanNote { scan: string; reason: string }

export interface EnvironmentView {
  name: string;
  /** Panacea's row LABEL (== name for non-base rows). Display only; never compared with a git branch (review b02). */
  branch: string;
  /** Panacea's run labels, compared only with each other. */
  branchesSeen: string[];
  collision: boolean;
  primary: boolean;
  host: string | null;
  status: string;
  replicas: { ready: number; desired: number } | null;
  build: { state: BuildState | "none"; deploymentId: string | null };
  serving: { measured: false } | { measured: true; ready: number };
  scans: {
    state: "reported" | "not_run" | "not_read";
    overall: "FAIL" | "INCOMPLETE" | "WARN" | "PASS" | "SKIPPED" | null;
    blocking: ScanNote[];
    incomplete: ScanNote[];
    detailsSource: "live" | "snapshot" | "mixed" | "unverified" | "none" | null;
  };
  current: { source: "deployment" | "rollback" | "none"; deploymentId: string | null; since: string | null };
}

export type PinReason =
  | "ship_head" | "inherited" | "no_brief" | "no_complete_revision" | "source_unpinned" | "source_unknown";

export interface ReleasePin {
  releaseId: string;
  state: "reserved" | "confirmed" | "abandoned" | "expired";
  pin: {
    status: "pinned" | "none";
    reason: PinReason;
    /** OPAQUE. Passed to phase 4's loader; never split, parsed or displayed raw (P5). */
    briefKey: string | null;
    briefDigest: string | null;
    headRevisionAtShip: number | null;
  };
  designKey: string | null;
  reservedBy: { id: string; displayName: string };
  reservedAt: string;
}

export interface ReleaseRow {
  kind: "run" | "rollback" | "pending" | "refused";
  deploymentId: string | null;
  environment: string | null;
  /** The name of the environments[] row the view joined this row to; null = unmatched. Look rows up by this (review I8). */
  environmentRow: string | null;
  /** Run rows: the Hub recorded the release against `recorded`, Panacea deployed it to `deployed` (task 08 rule 5a; b02 round 2, R1). */
  environmentMismatch: { recorded: string; deployed: string } | null;
  branch: string | null;
  commit: string | null;
  version: string | null;
  build: BuildState | null;
  buildRaw: string | null;
  failedStep: string;
  errorText: string;
  at: string;
  deployedBy: string;
  trigger: string;
  reconstructed: boolean;
  gates: Record<string, Gate> | null;
  provisioning: ProvisioningNote[];
  isCurrent: boolean;
  rollbackOf: { sourceDeploymentId: string; sourceVersion: string | null } | null;
  refusal: { message: string } | null;
  release: ReleasePin | null;
}

export interface RunView {
  run: string;
  build: BuildState;
  startedAt: string | null;
  currentStep: string;
  completedSteps: string[];
  failedStep: string;
  errorText: string;
  ignoredFailures: string[];
}

export interface DeployView {
  projectId: string;
  connection: ConnectionStatus;
  binding: Binding;
  fetchedAt: string | null;
  stale: boolean;
  staleReason: { code: string; message: string } | null;
  ownerControls: "yes" | "no" | "unknown";
  releasesState: "ok" | "no_hub" | "no_hub_project" | "signed_out" | "unavailable";
  /** The selected branch's environment, joined by the control plane (task 08 rule 1a). null = no ?branch= sent. */
  selected: {
    branch: string;
    target: string | null;
    environment: string | null;
    /** Panacea refused to name this branch's environment (task 08 rule 1a; b02 round 2, R2). A branch has an error or a target, never both. */
    error: { code: "not_found" | "rejected" | "forbidden"; message: string } | null;
  } | null;
  environments: EnvironmentView[];
  unmatched: EnvironmentView[];
  history: ReleaseRow[];
  runs: Record<string, RunView>;
}

export interface ShipResult { release: ReleasePin; run: string; confirmed: boolean; message: string }
export interface RollbackResult {
  release: ReleasePin; sourceDeploymentId: string; image: string | null;
  warmth: "warm" | "cold" | null; coldStartSeconds: number; confirmed: boolean;
}
export interface CancelResult { outcome: "cancelled" | "already_finished"; message: string }

export interface AutoDeploy { branch: string; explicit: boolean | null; projectDefault: boolean; effective: boolean; productionBranch: boolean }
export type PostureMode = "panacea-sso" | "kpmg-sso" | "password" | "bypass-token" | "none";
export interface Posture {
  branch: string; environment: string; mode: PostureMode; modeKnown: boolean; modeRaw: string;
  passwordSet: boolean; bypassTokenCount: number; ipAllowlist: string[];
}
export interface PostureResult { posture: Posture; enforced: boolean; message: string; bypassTokens?: string[] }
export type AccessRole = "viewer" | "developer" | "owner" | "unknown";
export interface AccessGrant { grantee: string; group: string; role: AccessRole; roleRaw: string }
export interface UrlPrefix { branch: string; prefix: string; host: string }

export interface ScanReportView {
  deploymentId: string;
  overall: string;
  blocking: ScanNote[];
  incomplete: ScanNote[];
  scans: Array<{
    name: string; status: string; mode: string; counts: Record<string, number>; summary: string;
    skipReason: string; errorReason: string; blocking: boolean; lastScanAt: string; findingsSource: string;
  }>;
  detailsRetained: boolean | null;
  detailsSource: string | null;
  updatedAt: string;
}
export interface Pod { name: string; phase: string; ready: boolean; restarts: number; reason: string; message: string }
export interface PodEvent { type: string; reason: string; object: string; message: string; lastSeen: string | null; count: number }
export interface Diagnosis {
  health: string; summary: string; source: "rules" | "rules+ai" | string;
  findings: Array<{ problem: string; likelyCause: string; suggestedFix: string; evidence: string[]; severity: string; owner: string }>;
}
```

#### The client — complete (`api.ts`)

```ts
export interface DeployApi {
  connection(): Promise<ConnectionStatus>;
  connect(input: { origin?: string; token: string }): Promise<ConnectionStatus>; // PUT; the token is never kept after the call
  recheck(): Promise<ConnectionStatus>;
  disconnect(): Promise<void>;
  /** branch: the lens's selected git branch name, verbatim; sent as ?branch=<encodeURIComponent>, omitted when null. */
  view(projectId: string, branch: string | null): Promise<DeployView>;
  bind(projectId: string, panaceaProject: string): Promise<Binding>;
  ship(projectId: string, req: { branchId: string; commit: string; designKey: string | null; noCache: boolean | null }, idempotencyKey: string): Promise<ShipResult>;
  rollback(projectId: string, deploymentId: string, idempotencyKey: string): Promise<RollbackResult>;
  rebuild(projectId: string, deploymentId: string, noCache: boolean | null, idempotencyKey: string): Promise<ShipResult>;
  cancel(projectId: string, run: string): Promise<CancelResult>;
  run(projectId: string, run: string): Promise<RunView>;
  scans(projectId: string, deploymentId: string): Promise<ScanReportView>;
  pods(projectId: string, env: string): Promise<Pod[]>;
  events(projectId: string, env: string): Promise<PodEvent[]>;
  diagnosis(projectId: string, env: string): Promise<Diagnosis>;
  autoDeploy(projectId: string, branch: string): Promise<AutoDeploy>;
  setAutoDeploy(projectId: string, branch: string, enabled: boolean): Promise<AutoDeploy>;
  posture(projectId: string, branch: string): Promise<Posture>;
  setPosture(projectId: string, branch: string, change: { mode: PostureMode; password?: string; rotateBypassToken?: boolean }): Promise<PostureResult>;
  access(projectId: string): Promise<AccessGrant[]>;
  grant(projectId: string, grantee: string, role: Exclude<AccessRole, "unknown">): Promise<AccessGrant[]>;
  revoke(projectId: string, grantee: string): Promise<AccessGrant[]>;
  setUrlPrefix(projectId: string, branch: string, prefix: string): Promise<UrlPrefix>;
  clearUrlPrefix(projectId: string, branch: string): Promise<void>;
  /** The one streaming call. The caller aborts with `signal`; frames arrive already parsed. */
  stream(projectId: string, what: { kind: "run-logs"; run: string } | { kind: "readiness"; env: string },
         onFrame: (f: StreamFrame) => void, signal: AbortSignal): Promise<void>;
}

export type StreamFrame =
  | { event: "log"; step: string; line: string }
  | { event: "readiness"; measured: boolean; ready: number | null; crashReason?: string; restarts?: number; oomKilled?: boolean; exitCode?: number }
  | { event: "problem"; message: string }
  | { event: "end" };

/** Built over phase 2's client so the base URL, the timeout, the problem decoding and (from Hub
 *  task C3) the local caller key are the same for every request. */
export function createDeployApi(client: CockpitClient): DeployApi;
```

`client.ts` amendments, each a declared departure into phase 2's file: `type Method` gains `"PUT"`;
`send(method, route, body?, headers?)`; `CockpitClient` gains
`request<T>(method: Method, route: string, body?: unknown, headers?: Record<string, string>): Promise<T>`
and `requestStream(route: string, signal: AbortSignal): Promise<ReadableStream<Uint8Array>>` (no
timeout — a stream is long; the abort is the caller's). Branch values go in `?branch=` through
`encodeURIComponent`; ids in paths through `encodeURIComponent` too.

`ProblemCode` gains, in one block with a comment naming this plan: `"upstream_unavailable"`,
`"deploy_not_connected"`, `"deploy_token_rejected"`, `"deploy_forbidden"`, `"deploy_not_found"`,
`"deploy_conflict"`, `"deploy_rejected"`, `"deploy_not_bound"`, `"deploy_environment_invalid"`,
`"deploy_environment_unreadable"`, `"deploy_precondition"`.

#### `upstream-down` — ruling R4, if phase 5 has not landed it

```ts
export type ProblemKind =
  | "control-plane-down"
  | "backend-down"
  /** A remote service the control plane depends on is unreachable: details.upstream names it —
   *  "panacea", "hub" or "canvas". Its own kind, not "server-said", because the Deploy lens must
   *  turn Ship and rollback OFF on it, and a sentence cannot be switched on (ruling R4). */
  | "upstream-down"
  | "server-said";

export function classify(error: ApiError): ProblemKind {
  if (error.code === "network_unavailable") return "control-plane-down";
  if (error.code === "backend_unavailable") return "backend-down";
  if (error.code === "upstream_unavailable") return "upstream-down";
  return "server-said";
}

/** The name a sentence uses for details.upstream. Unknown → "a service the cockpit depends on". */
export function upstreamName(error: ApiError): string; // "Panacea" | "the Hub" | "the design canvas" | …
```

Each of the four call sites gains one `case "upstream-down":` with its own tail, e.g.
`EmptyDevelop.tsx`: *"`${upstreamName(error)}` could not be reached, so the session was not
created."* — TS2366 at each site is the enforcement working (R4).

#### The slice and the loader

```ts
export interface DeploySlice {
  /** Server truth, never persisted (7P-13). Keyed by local project id. */
  deployViews: Record<string, DeployView>;
  deployLoad: Record<string, { state: "idle" | "loading" | "error"; error: { code: string; message: string } | null }>;
  /** The last Panacea-Deploy-Cursor value seen; "" before the first. */
  deployCursor: string;
  setDeployView(projectId: string, view: DeployView): void;
  setDeployLoad(projectId: string, state: DeploySlice["deployLoad"][string]): void;
  setDeployCursor(value: string): void;
}

/** Wired as the poller's onCursor listener for name "Deploy" (HA7-4). A changed value loads the
 *  selected project's view — once, even if the value changes again while that load is in flight
 *  (the in-flight load's answer is newer than the cursor that started it). It loads THROUGH
 *  loadDeployView, so a cursor-driven load sends the selected branch too; it never calls
 *  view(projectId, null) itself, which would return selected: null on every tick (b02 round 2, R6). */
export function onDeployCursor(value: string, deps: { api: DeployApi; store: typeof useCockpit }): void;

/** Called by the lens on mount and by a selection change while the lens is open. Sends the
 *  selected branch's name (the same selection the lens renders), so the control plane can
 *  answer `view.selected` (task 08 rule 1a); the desktop never maps a branch to an environment. */
export function loadDeployView(projectId: string, deps: { api: DeployApi; store: typeof useCockpit }): Promise<void>;
```

```ts
// store/selectors.ts
export const selectDeployView = (projectId: string) => (s: CockpitState): DeployView | undefined;
/** R6's key source for Deploy: the pinned brief keys of the selected project's history, in
 *  history order, de-duplicated. Phase 4's loader unions it with the other lenses' sources and
 *  issues one batched resolve on the existing tick (orchestrator-rulings.md R6). */
export const selectReleasePinKeys = (s: CockpitState): string[];
```

#### Tests to write first

| Test | Assertion | Mutant it kills |
|---|---|---|
| `classify maps upstream_unavailable` | → `"upstream-down"`; `details.upstream: "hub"` → `upstreamName` *"the Hub"* | routing Panacea's outage through `backend-down`, which says "Docker" (brief §4.1) |
| `tsc fails without a case` | a fixture call site with no `upstream-down` case fails `tsc --noEmit` (the existing `designsystem`/guard pattern, grep of the three callers) | a `default:` added to quiet the compiler |
| `cursor change loads once` | with branch `feat/x` selected, three `onDeployCursor` calls with new values while one load is in flight → one `view()` call, then one more after it settles, and both requests carry `?branch=feat%2Fx` (b02 round 2, R6) | a burst of fetches per tick, or a cursor path that drops the branch so the card vanishes on every tick |
| `same cursor loads nothing` | the same value twice → zero `view()` calls | a fetch per tick in the steady state |
| `view sends the branch verbatim` | `view("p-1", "feat/x")` requests `/v1/projects/p-1/deploy?branch=feat%2Fx`; `view("p-1", null)` sends no query | a desktop that sanitises the name itself, or drops it so `selected` is always null (review b02) |
| `no setInterval added` | `grep -rn setInterval desktop/src --include=*.ts --include=*.tsx` in the test's own guard finds only `poller.ts` and `hooks/heartbeat.ts` | a third clock |
| `partialize excludes deploy` | `partialize(state)` has no key starting `deploy` | persisting server truth, which outlives an erase (ADR-H1 §5.4) |
| `pin keys are opaque` | `selectReleasePinKeys` returns the keys byte-for-byte; the source file contains no `split(`, `indexOf("@")` or regex on a key | parsing a brief key in the desktop (P5) |
| `pin keys de-duplicated` | two rows with one key → one key | two resolver entries for one key |
| `branch goes in the query` | `autoDeploy("p", "feat/x")` requests `/v1/projects/p/deploy/auto-deploy?branch=feat%2Fx` | a path-segment branch |
| `ship sends the idempotency key` | `ship(…, "k1")` sets `Idempotency-Key: k1` | a retried Ship reserving twice |
| `connect drops the token` | after `connect`, no store field, closure or module variable holds the token (the test searches the store's JSON and the module's exports) | a token kept in renderer memory, the most exposed process (ADR-H1 §6.5) |
| `stream parses our frames` | a `ReadableStream` of `event: log\ndata: {…}\n\n` chunks split mid-frame → one `log` frame per event | a parser that assumes one frame per chunk |

#### Steps

- [ ] Check first: `grep -n "upstream-down" desktop/src/api/problems.ts` (phase 5 may have it) and
  `grep -n "onCursor" desktop/src/api/poller.ts` (Hub C2 may have it). Build only what is absent,
  and name what was reused in the commit message.
- [ ] Tests first (`vitest`), then `types.ts`, `api.ts`, the `client.ts` amendments, the slice, the
  loader, the selectors, `problems.ts` and its four call sites.
- [ ] `npx tsc --noEmit` and `npx vitest run` green; commit by pathspec.

#### Acceptance

```
cd desktop && npx tsc --noEmit        → no output
cd desktop && npx vitest run          → all suites pass, including phase 2's
```

#### What this task does NOT claim

- Anything on screen. Tasks 14–18.
- That phase 4's loader exists. If it has not merged, `selectReleasePinKeys` is exported and
  unused, and the pin cell (task 15) renders `resolving` for every key — never a head (P1).
