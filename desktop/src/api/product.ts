import { checkRunResult, checkRecoveryResult, checkCanRecover, type CheckRecoveryReview } from './projectCheckContracts';
import { workspacePath, teamScopePrefix, teamScopeGeneration } from './teamScope';
import { ApiError, DEFAULT_BASE_URL, type ProblemCode } from "./client";
import { authHeaders, expireAuth } from "./authSession";
import type { Session } from "../store/types";

export interface SetupConfig {
  projectsDir: string;
  mode: "local" | "cloud";
  cloudProvider: "" | "aws" | "azure";
  region: string;
  domain: string;
  monthlyBudgetUsd: number;
}
export interface SetupCheck {
  id: string;
  label: string;
  status: "unknown" | "ready" | "missing" | "error";
  message: string;
  action: string;
}
export interface AgentProvider {
  id: string;
  name: string;
  configured: boolean;
  authMode?: string;
  model?: string;
  baseUrl?: string;
  signInCommand?: string;
}
export interface AgentConnections {
  providers: AgentProvider[];
}
export type AgentConnectionKind = "litellm" | "compatible-api" | "codex-subscription" | "direct-api";
export interface AgentConnection {
  id: string; name: string; kind: AgentConnectionKind; revision: number; enabled: boolean;
  endpoints: { responses?: string; anthropicMessages?: string; chatCompletions?: string };
  models: { codex?: string; claudeCode?: string; opencode?: string };
  configured: boolean; keyPresent: boolean; blockedReason?: string;
}
export interface AgentConnectionCatalogue {
  revision: number; activeConnectionId: string; connections: AgentConnection[];
}
export type AgentConnectionMutation =
  | { action: "save"; expectedRevision: number; id?: string; name: string; kind: AgentConnectionKind; enabled: boolean; endpoints: AgentConnection["endpoints"]; models: AgentConnection["models"]; apiKey?: string }
  | { action: "select" | "delete"; expectedRevision: number; id: string };
export interface CodexAccountConnection {
  configured: boolean;
  authMode: "chatgpt";
  sourceSessionId?: string;
  activeSessionId?: string;
  message: string;
}
export interface GitHubAccount {
  available: boolean;
  signedIn: boolean;
  login?: string;
  message: string;
  actionUrl?: string;
}
export interface GitPublication {
  repository: string;
  visibility?: "private" | "public";
  status: string;
  branch?: string;
  commit?: string;
  verified: boolean;
  updatedAt: string;
}
export interface GitDelivery {
  origin?: string;
  branch: string;
  head: string;
  publication?: GitPublication;
  message?: string;
}
export interface SetupStatus {
	setupChoice?: SetupChoice;
  product: string;
  version: string;
  platform: string;
  configured: boolean;
  workspace?: Workspace;
  access?: {
    user: { username: string; role: "member" | "admin" | "super-admin" };
    manageBacking: boolean;
    projectsDirManaged: boolean;
  };
  config: SetupConfig;
  checks: SetupCheck[];
  cloudProviders: {
    id: "aws" | "azure";
    name: string;
    strategy: string;
    description: string;
  }[];
}
export type SetupMode = 'local' | 'bring-your-own-cloud' | 'luci-cloud';
export interface SetupChoice { revision: number; mode: SetupMode | ''; updatedAt?: string }
export interface Workspace {
  mode: "local" | "cloud";
  connectionId: string;
  host: string;
  ready: boolean;
}
export interface WorkspaceAuth {
  authenticated: boolean;
  user?: { id: string; username: string; displayName: string; role: string };
  bootstrapRequired: boolean;
  bootstrapTokenAvailable?: boolean;
  message: string;
}
export interface WorkspaceInstall {
  id: string;
  connectionId: string;
  status: string;
  message: string;
  createdAt: string;
  updatedAt: string;
}
export interface CloudPlanRequest {
  provider: "aws" | "azure";
  name: string;
  region: string;
  domain: string;
  size: "small" | "standard" | "full";
  monthlyBudgetUsd: number;
  sshPublicKey: string;
}
export interface CloudPlan {
  id: string;
  provider: string;
  name: string;
  region: string;
  domain: string;
  resources: string[];
  estimate: {
    currency: string;
    monthly: number | null;
    verified: boolean;
    explanation: string;
  };
  steps: { id: string; title: string; detail: string }[];
  files: { name: string; content: string; mediaType: string }[];
  commands: { label: string; executable: string; args: string[] }[];
  warnings: string[];
  createdAt: string;
}
export interface Brief {
  projectId: string;
  revision: number;
  purpose: string;
  audience: string;
  constraints: string;
  criteria: string[];
  updatedAt: string;
}
export interface DesignElement {
  id: string;
  kind: "text" | "rectangle" | "button";
  x: number;
  y: number;
  width: number;
  height: number;
  text: string;
  color: string;
}
export interface DesignFrame {
  id: string;
  name: string;
  width: number;
  height: number;
  background: string;
  elements: DesignElement[];
}
export interface DesignScene {
  elements: unknown[];
  appState: Record<string, unknown>;
  files: Record<string, unknown>;
}
export interface Design {
  projectId: string;
  revision: number;
  briefRevision: number;
  frames: DesignFrame[];
  scene?: DesignScene;
  updatedAt: string;
}
export interface FileEntry {
  name: string;
  path: string;
  kind: string;
  size: number;
}
export type FileResult =
  | { path: string; entries: FileEntry[] }
  | { path: string; content: string; truncated: boolean };
export interface GitState {
  /** Opaque identity of the source, index, branch and HEAD displayed for review. */
  reviewDigest?: string;
  branch: string;
  clean: boolean;
  changes: { path: string; status: string }[];
  log: { hash: string; subject: string }[];
  diff: string;
}
export interface ImplementationTask {
  id: string;
  title: string;
  status: "todo" | "doing" | "done";
  requirementIds?: string[];
  dependsOn?: string[];
  writePaths?: string[];
	role?: 'coding' | 'reviewer';
	reviewOfTaskId?: string;
  acceptance?: string;
}
/** Planning advice derived from saved tasks; human-recorded Done is not a verified result. */
export interface ImplementationTaskGraph {
  waves: string[][];
  readyTaskIds: string[];
  blocked: { taskId: string; dependsOn: string[] }[];
  conflicts: { taskIds: [string, string]; paths: string[] }[];
  conflictsOmitted: number;
  unscopedTaskIds: string[];
  recordedDoneTaskIds: string[];
}
export interface ImplementationPlan {
  projectId: string;
  revision: number;
  briefRevision: number;
  specificationRevision?: number;
  specificationDigest?: string;
  tasks: ImplementationTask[];
  graph?: ImplementationTaskGraph;
  updatedAt: string;
}
export interface CheckCommand {
  label: string;
  executable: string;
  args: string[];
}
export interface CheckResult extends CheckCommand {
	/** Runner attests this command was not started after a settled failure. */
	notStartedAfterFailure?: boolean;
  allocationId?: string;
  stage?: string;
  reasonCode?: string;
  message?: string;
  cleanupConfirmed?: boolean;
  status: string;
  exitCode?: number;
  output: string;
  truncated: boolean;
}
export interface CheckRun {
  revision?: number;
  resourcesHeld?: boolean | null;
  stage?: string;
  reasonCode?: string;
  id: string;
  projectId: string;
  commit: string;
  dirty: boolean;
  execution?: "docker" | "host";
  sourceDigest?: string;
  sourceChanged?: boolean;
  requirementIds?: string[];
  requirementSnapshots?: { id: string; title: string; acceptance: string }[];
  specificationRevision?: number;
  specificationDigest?: string;
  designPlanRevision?: number;
  designPlanDigest?: string;
  evidenceStatus?: RequirementEvidenceStatus;
  staleReasons?: string[];
  status: string;
  commands: CheckResult[];
  createdAt: string;
  updatedAt: string;
  message: string;
}
export type RequirementEvidenceStatus = "linked-test-evidence" | "stale" | "unverified";
export interface RequirementCoverage {
  projectId: string;
  specification: { revision: number; status: string; digest: string };
  design: { revision: number; status: string; digest: string };
  source: { commit: string; sourceDigest: string; dirty: boolean; available: boolean };
  rows: {
    requirementId: string;
    title: string;
    acceptance: string;
    taskIds: string[];
    doneTaskIds: string[];
    evidenceRunIds: string[];
    evidenceStatus: RequirementEvidenceStatus;
  }[];
}
export interface RequirementCheckBinding {
  requirementIds: string[];
  expectedSpecificationRevision: number;
  expectedDesignPlanRevision: number;
}
export interface Release {
  kind?: 'reviewed-source';
  canStop?: boolean;
  revision?: number;
  canCancel?: boolean;
  cancelRequested?: boolean;
  id: string;
  projectId: string;
  briefRevision: number;
  configurationRevision?: number;
  commit: string;
  target: string;
  status: string;
  url: string;
  browserConnectionRequired?: boolean;
  message: string;
  createdAt: string;
}
export interface PagesReleaseRequest {
  commit: string;
  branch: string;
  acknowledgePublic: boolean;
}
export interface CloudJob {
  id: string;
  planId: string;
  provider: string;
  name: string;
  region: string;
  domain: string;
  status: string;
  message: string;
  publicIp: string;
  resourceId: string;
  createdAt: string;
  updatedAt: string;
}
export interface DeploymentTarget {
  id: string;
  name: string;
  baseUrl: string;
  serverUuid: string;
}
export interface AzureConnectionPin {
  sourceCommit: string;
  scope: string;
  generation: string;
  controllerResourceID: string;
  dataDiskID: string;
  bootID: string;
  fingerprint: string;
}
export interface SSHRequest {
  host: string;
  port: number;
  user: string;
  privateKeyPath: string;
  azureIdentity?: AzureConnectionPin;
}
export interface SSHScan {
  id: string;
  host: string;
  fingerprints: string[];
  createdAt: string;
}
export interface Tunnel {
  id: string;
  host: string;
  localUrl: string;
  status: string;
  message: string;
}
export interface DNSResult {
  domain: string;
  expectedIp: string;
  addresses: string[];
  ready: boolean;
  message: string;
}
export interface CloudReleaseRequest {
  dataClientFramework?: "none" | "vite" | "nextjs";
  source?: "public" | "private";
  privateKeyUuid?: string;
  targetId: string;
  repository: string;
  branch: string;
  commit: string;
  domain: string;
  port: number;
  healthPath: string;
  dockerfile: string;
}
export interface RuntimeAdmissionObservation {
  stage: "initial" | "before-build";
  reasonCode: string;
  decidedAt: string;
  samplerAvailable: boolean;
  hostObservedAt?: string;
  executionObservedAt?: string;
  hostFresh: boolean;
  executionFresh: boolean;
  hostMemoryKnown: boolean;
  hostCPUKnown: boolean;
  executionMemoryKnown: boolean;
  executionCPUKnown: boolean;
  inventoryComplete: boolean;
  engineMatched: boolean;
  metric?: string;
  metricObservedAt?: string;
  observedBytes?: number;
  requiredBytes?: number;
  observedPercent?: number;
  maximumPercent?: number;
}
export interface RuntimeJob {
  execution?: { jobId: string; allocationId?: string; engineId?: string; instanceId?: string; sourceDigest?: string; imageId?: string; imageTag?: string; phase: string; resourcesHeld: boolean; cleanupConfirmed: boolean; reasonCode?: string; message: string; admission?: RuntimeAdmissionObservation };
  cancelRequested?: boolean;
  id: string;
  status: string;
  image: string;
  message: string;
  createdAt: string;
  updatedAt: string;
}
export interface ProjectConfiguration {
  revision: number;
  variables: { name: string }[];
  updatedAt: string;
}
export type ConfigurationUpdate =
  | { name: string; value: string; remove?: never }
  | { name: string; remove: true; value?: never };
export class ProductApiError extends ApiError {
  constructor(
    code: ProblemCode,
    message: string,
    status: number,
    readonly body: unknown,
  ) {
    super(code, message, status);
    Object.setPrototypeOf(this, ProductApiError.prototype);
  }
}
export interface ProviderLoginJob {
  id: string;
  provider: "aws" | "azure" | "github";
  status: string;
  message: string;
  verificationUrl?: string;
  userCode?: string;
  actionUrl?: string;
  createdAt: string;
}

export function createProductClient(
  baseUrl = (typeof window === "undefined" ? undefined : window.__LUCI_API__) ||
    import.meta.env.VITE_LUCI_API ||
    DEFAULT_BASE_URL,
  fetcher: typeof fetch = (...args) => globalThis.fetch(...args),
) {
  async function transfer<T>(
    path: string,
    body: Blob | undefined,
    read: (response: Response) => Promise<T>,
    options: {
      method?: "GET" | "POST";
      headers?: Record<string, string>;
      budgetMs?: number;
      signal?: AbortSignal;
      kind?: "project" | "file";
    } = {},
  ): Promise<T> {
    const scopeGeneration = teamScopeGeneration();
    const controller = new AbortController();
    const abort = () => controller.abort(options.signal?.reason);
    options.signal?.addEventListener("abort", abort, { once: true });
    if (options.signal?.aborted) abort();
    const timer = setTimeout(() => controller.abort(), options.budgetMs ?? 120_000);
    const method = options.method ?? "POST";
    try {
      controller.signal.throwIfAborted();
      const response = await fetcher(`${baseUrl.replace(/\/$/, "")}${workspacePath(path)}`, {
        method,
        signal: controller.signal,
        headers: authHeaders(method === "GET" ? options.headers : {
          "Content-Type": body ? "application/zip" : "application/json",
          ...options.headers,
        }),
        body: method === "GET" ? undefined : body || "{}",
      });
      if (scopeGeneration !== teamScopeGeneration()) throw new ApiError("conflict", "The workspace changed before this response arrived. Open its history to inspect the result.", 409);
      if (response.status === 401) expireAuth();
      if (!response.ok) {
        const problem = (await response.json().catch(() => ({
          message: `Request failed (${response.status}).`,
        }))) as { error?: string; message?: string };
        throw new ProductApiError(
          (problem.error || "internal") as ProblemCode,
          problem.message || (options.kind === "file" ? "File transfer failed." : "Project transfer failed."),
          response.status,
          problem,
        );
      }
      const value = await read(response);
      if (scopeGeneration !== teamScopeGeneration()) throw new ApiError("conflict", "The workspace changed while this transfer was loading.", 409);
      return value;
    } catch (cause) {
      if (options.signal?.aborted) throw options.signal.reason;
      if (cause instanceof ApiError) throw cause;
      throw new ApiError(
        controller.signal.aborted ? "timeout" : "network_unavailable",
        controller.signal.aborted
          ? options.kind === "file"
            ? "The file transfer timed out. Check its result before trying again."
            : "Project transfer timed out. Check the project list before retrying a restore."
          : "The workspace service is unavailable. Retry when it reconnects.",
        0,
      );
    } finally {
      clearTimeout(timer);
      options.signal?.removeEventListener("abort", abort);
    }
  }
  async function request<T>(
    path: string,
    body?: unknown,
    method = body === undefined ? "GET" : "POST",
    budgetMs = 120_000,
    signal?: AbortSignal,
  ): Promise<T> {
    const scopeGeneration = teamScopeGeneration();
    const controller = new AbortController();
    const abort = () => controller.abort(signal?.reason);
    signal?.addEventListener("abort", abort, { once: true });
    if (signal?.aborted) abort();
    const timer = setTimeout(() => controller.abort(), budgetMs);
    try {
      controller.signal.throwIfAborted();
      const response = await fetcher(`${baseUrl.replace(/\/$/, "")}${workspacePath(path)}`, {
        method,
        signal: controller.signal,
        headers: authHeaders(
          body === undefined
            ? undefined
            : { "Content-Type": "application/json" },
        ),
        body: body === undefined ? undefined : JSON.stringify(body),
      });
      if (scopeGeneration !== teamScopeGeneration()) throw new ApiError("conflict", "The workspace changed before this response arrived. Open its history to inspect the result.", 409);
      if (response.status === 401) expireAuth();
      const raw = await response.text();
      if (scopeGeneration !== teamScopeGeneration()) throw new ApiError("conflict", "The workspace changed while this response was loading.", 409);
      let value: unknown;
      try {
        value = raw === "" ? undefined : JSON.parse(raw);
      } catch {
        throw new ApiError(
          "internal",
          `Luci received an unreadable response (${response.status}).`,
          response.status,
        );
      }
      if (!response.ok) {
        const problem = value as
          { error?: string; message?: string } | undefined;
        throw new ProductApiError(
          (problem?.error || "internal") as ProblemCode,
          problem?.message || `Request failed (${response.status}).`,
          response.status,
          value,
        );
      }
      return value as T;
    } catch (error) {
      if (signal?.aborted) throw signal.reason;
      if (error instanceof ApiError) throw error;
      throw new ApiError(
        controller.signal.aborted ? "timeout" : "network_unavailable",
        controller.signal.aborted
          ? "Luci is taking too long to answer. Try again."
          : "Luci’s local service is not available. Start the service and retry.",
        0,
      );
    } finally {
      clearTimeout(timer);
      signal?.removeEventListener("abort", abort);
    }
  }
  const project = (id: string) =>
    `/v1/product/projects/${encodeURIComponent(id)}`;
  return {
    get baseUrl() { return baseUrl.replace(/\/$/, "") + teamScopePrefix(); },
    request,
    uploadJSON: <T>(path: string, body: Blob, signal?: AbortSignal) => transfer<T>(path, body, response => response.json() as Promise<T>, {
      method: "POST", headers: { "Content-Type": body.type || "application/octet-stream" }, budgetMs: 30_000, signal, kind: "file",
    }),
    requestBlob: (
      path: string,
      options: { method?: "GET" | "POST"; body?: Blob; headers?: Record<string, string> } = {},
      budgetMs = 60_000,
      signal?: AbortSignal,
    ) => transfer(path, options.body, response => response.blob(), {
      method: options.method ?? "GET", headers: options.headers, budgetMs, signal, kind: "file",
    }),
    status: () => request<SetupStatus>("/v1/setup/status"),
    chooseSetup: async (expectedRevision: number, mode: SetupMode) => {
      const choice = await request<SetupChoice>('/v1/setup/choice', {expectedRevision, mode}, 'PUT');
      if (!choice || !Number.isSafeInteger(choice.revision) || choice.revision <= expectedRevision || choice.mode !== mode) throw new Error('The saved setup choice could not be confirmed. Refresh before trying again.');
      return choice;
    },
    check: (signal?: AbortSignal) =>
      request<SetupStatus>("/v1/setup/check", {}, "POST", 120_000, signal),
    configure: (config: SetupConfig) =>
      request<SetupStatus>("/v1/setup/config", config),
    cloudPlan: (input: CloudPlanRequest) =>
      request<CloudPlan>("/v1/setup/cloud/plan", input),
    gitIdentity: () =>
      request<{ name: string; email: string }>("/v1/setup/git/identity"),
    agents: () => request<AgentConnections>("/v1/setup/agents"),
    agentConnections: () => request<AgentConnectionCatalogue>("/v1/setup/agent-connections"),
    updateAgentConnection: (input: AgentConnectionMutation) => request<AgentConnectionCatalogue>("/v1/setup/agent-connections", input),
    codexAccount: () =>
      request<CodexAccountConnection>("/v1/setup/agents/codex/account"),
    startCodexSignIn: () =>
      request<Session>("/v1/setup/agents/codex/sign-in", {}),
    connectCodexAccount: (sessionId: string) =>
      request<CodexAccountConnection>("/v1/setup/agents/codex/account", {
        sessionId,
      }),
    disconnectCodexAccount: () =>
      request<CodexAccountConnection>(
        "/v1/setup/agents/codex/account",
        undefined,
        "DELETE",
      ),
    saveAnthropicKey: (apiKey: string) =>
      request<AgentConnections>("/v1/setup/agents/anthropic", { apiKey }),
    saveOpenAIKey: (apiKey: string) =>
      request<AgentConnections>("/v1/setup/agents/openai", { apiKey }),
    saveLocalAgent: (baseUrl: string, model: string, apiKey?: string) =>
      request<AgentConnections>("/v1/setup/agents/local", {
        baseUrl,
        model,
        ...(apiKey ? { apiKey } : {}),
      }),
    saveGitIdentity: (name: string, email: string) =>
      request<{ name: string; email: string }>("/v1/setup/git/identity", {
        name,
        email,
      }),
    createSSHKey: () =>
      request<{
        publicKey: string;
        privateKeyPath: string;
        fingerprint: string;
      }>("/v1/setup/ssh/key", {}),
    runtime: (signal?: AbortSignal) =>
      request<RuntimeJob>("/v1/setup/runtime/install", undefined, "GET", 120_000, signal),
    installRuntime: (mode?: 'document-packaged') => request<RuntimeJob>("/v1/setup/runtime/install", mode ? {mode} : {}),
    cancelRuntime: (jobId: string) => request<RuntimeJob>("/v1/setup/runtime/install/cancel", { jobId }),
    reconcileRuntime: (jobId: string) => request<RuntimeJob>("/v1/setup/runtime/install/reconcile", { jobId }),
    cloudJobs: () => request<{ jobs: CloudJob[] }>("/v1/setup/cloud/jobs"),
    workspaceInstalls: () =>
      request<{ jobs: WorkspaceInstall[] }>("/v1/setup/workspace/install"),
    installWorkspace: (connectionId: string, acknowledgeInstall: boolean) =>
      request<WorkspaceInstall>(
        `/v1/setup/ssh/connections/${encodeURIComponent(connectionId)}/install-luci`,
        { acknowledgeInstall },
      ),
    activateWorkspace: (connectionId: string) =>
      request<unknown>("/v1/setup/workspace/activate", { connectionId }),
    workspaceAuth: (connectionId: string) =>
      request<WorkspaceAuth>(
        `/v1/setup/workspace/auth?connectionId=${encodeURIComponent(connectionId)}`,
      ),
    signInWorkspace: (input: {
      connectionId: string;
      username: string;
      password: string;
      bootstrapToken?: string;
      inviteCode?: string;
      displayName?: string;
    }) => request<WorkspaceAuth>("/v1/setup/workspace/auth", input),
    disconnectWorkspaceAuth: (connectionId: string) =>
      request<WorkspaceAuth>(
        `/v1/setup/workspace/auth?connectionId=${encodeURIComponent(connectionId)}`,
        undefined,
        "DELETE",
      ),
    localWorkspace: () => request<unknown>("/v1/setup/workspace/local", {}),
    prepareWorkspacePreview: (projectId: string, releaseId: string) =>
      request<{ url: string }>("/v1/setup/workspace/preview", {
        projectId,
        releaseId,
      }),
    providerLogins: () =>
      request<{ jobs: ProviderLoginJob[] }>("/v1/setup/cloud/login"),
    providerLogin: (provider: "aws" | "azure") =>
      request<ProviderLoginJob>("/v1/setup/cloud/login", { provider }),
    createCloud: (
      planId: string,
      acknowledgeCosts: boolean,
      acknowledgeUnverifiedPrice: boolean,
    ) =>
      request<CloudJob>("/v1/setup/cloud/create", {
        planId,
        acknowledgeCosts,
        acknowledgeUnverifiedPrice,
      }),
    refreshCloud: (id: string) =>
      request<CloudJob>(
        `/v1/setup/cloud/jobs/${encodeURIComponent(id)}/refresh`,
        {},
      ),
    deleteCloud: (id: string, confirmName: string) =>
      request<CloudJob>(
        `/v1/setup/cloud/jobs/${encodeURIComponent(id)}/delete`,
        { confirmName },
      ),
    cloudDNS: (id: string) =>
      request<DNSResult>(
        `/v1/setup/cloud/jobs/${encodeURIComponent(id)}/dns`,
        {},
      ),
    targets: () =>
      request<{ targets: DeploymentTarget[] }>("/v1/setup/targets"),
    connectTarget: (input: {
      id?: string;
      name: string;
      baseUrl: string;
      token: string;
      serverUuid?: string;
    }) => request<DeploymentTarget>("/v1/setup/targets", input),
    inspectSSH: (input: SSHRequest) =>
      request<SSHScan>("/v1/setup/ssh/inspect", input),
    connectSSH: (scanId: string, acceptedFingerprint: string) =>
      request<Tunnel>("/v1/setup/ssh/connect", { scanId, acceptedFingerprint }),
    tunnels: () =>
      request<{ connections: Tunnel[] }>("/v1/setup/ssh/connections"),
    stopTunnel: (id: string) =>
      request<{ status: string }>(
        `/v1/setup/ssh/connections/${encodeURIComponent(id)}/stop`,
        {},
      ),
    createProject: (name: string, remoteUrl?: string, importAttemptId?: string) =>
      request<{ id: string; name: string; path: string }>(
        "/v1/product/projects",
        { name, ...(remoteUrl ? { remoteUrl } : {}), ...(importAttemptId ? { importAttemptId } : {}) },
      ),
    brief: (id: string) => request<Brief>(`${project(id)}/brief`),
    saveBrief: (
      id: string,
      brief: Pick<
        Brief,
        "revision" | "purpose" | "audience" | "constraints" | "criteria"
      >,
    ) => request<Brief>(`${project(id)}/brief`, brief),
    design: (id: string) => request<Design>(`${project(id)}/design`),
    saveDesign: (
      id: string,
      design: Pick<Design, "revision"> &
        Partial<Pick<Design, "frames" | "scene">>,
    ) => request<Design>(`${project(id)}/design`, design),
    designExportUrl: (id: string) =>
      `${baseUrl.replace(/\/$/, "")}${workspacePath(project(id) + "/design/export")}`,
    files: (id: string, path = "") =>
      request<FileResult>(
        `${project(id)}/files?path=${encodeURIComponent(path)}`,
      ),
    git: (id: string) => request<GitState>(`${project(id)}/git`),
    commit: (id: string, message: string, expectedReviewDigest: string) =>
      request<{ commit: string; message: string; sourceChanged: boolean }>(`${project(id)}/git/commit`, { message, expectedReviewDigest }),
    gitDelivery: (id: string) =>
      request<GitDelivery>(`${project(id)}/git/delivery`),
    gitRemote: (id: string, url: string) =>
      request<GitPublication>(`${project(id)}/git/remote`, { url }),
    githubCreate: (
      id: string,
      input: {
        owner?: string;
        name: string;
        visibility: "private" | "public";
        acknowledgePublic: boolean;
      },
    ) => request<GitPublication>(`${project(id)}/git/github`, input),
    gitPush: (id: string, commit: string, branch: string) =>
      request<GitPublication>(`${project(id)}/git/push`, { commit, branch }),
    githubCheck: () => request<GitHubAccount>("/v1/setup/github/check", {}),
    githubLogin: () => request<ProviderLoginJob>("/v1/setup/github/login", {}),
    githubLogins: () =>
      request<{ jobs: ProviderLoginJob[] }>("/v1/setup/github/login"),
    backupProject: (id: string) =>
      transfer(`${project(id)}/backups`, undefined, (response) =>
        response.blob(),
      ),
    restoreProject: (name: string, archive: Blob) =>
      transfer<{ id: string; name: string; path: string }>(
        `/v1/product/projects/restore?name=${encodeURIComponent(name)}`,
        archive,
        (response) => response.json(),
      ),
    plan: (id: string) => request<ImplementationPlan>(`${project(id)}/plan`),
    savePlan: (
      id: string,
      expectedRevision: number,
      tasks: ImplementationTask[],
      expectedSpecificationRevision?: number,
    ) =>
      request<ImplementationPlan>(`${project(id)}/plan`, {
        expectedRevision,
        tasks,
        ...(expectedSpecificationRevision === undefined ? {} : { expectedSpecificationRevision }),
      }),
    checks: async (id: string, signal?: AbortSignal) => {
      const value = await request<{ runs: CheckRun[]; suggested: CheckCommand[]; coverage?: RequirementCoverage }>(`${project(id)}/checks`, undefined, 'GET', 30_000, signal);
      if (!value || !Array.isArray(value.runs)) throw new Error('Check history could not be verified. Refresh this project.');
      return {...value, runs: value.runs.map(run => checkRunResult(id, run))};
    },
    runChecks: async (
      id: string,
      commands: CheckCommand[],
      acknowledgeHostExecution: boolean,
      execution: "docker" | "host" = "docker",
      binding?: RequirementCheckBinding,
    ) => checkRunResult(id, await request<CheckRun>(`${project(id)}/checks`, {
      commands, acknowledgeHostExecution, execution, ...binding,
    })),
    checkRun: async (id: string, runId: string, signal?: AbortSignal) =>
      checkRunResult(id, await request<CheckRun>(`${project(id)}/checks/${encodeURIComponent(runId)}`, undefined, 'GET', 30_000, signal), runId),
    cancelChecks: async (id: string, runId: string) =>
      checkRunResult(id, await request<CheckRun>(`${project(id)}/checks/${encodeURIComponent(runId)}/cancel`, {}), runId),
    checkRecovery: async (id: string, runId: string, signal?: AbortSignal) =>
      checkRecoveryResult(id, runId, await request(`${project(id)}/checks/${encodeURIComponent(runId)}/recovery`, undefined, 'GET', 30_000, signal)),
    recoverCheck: async (id: string, runId: string, review: CheckRecoveryReview) => {
      if (review.projectId !== id || review.checkId !== runId || !checkCanRecover(review)) throw new Error('Refresh this exact check recovery before continuing.');
      const selected = checkRecoveryResult(id, runId, review);
      return checkRunResult(id, await request(`${project(id)}/checks/${encodeURIComponent(runId)}/recovery`, {
        expectedRevision: selected.revision, recoveryDigest: selected.recoveryDigest, acknowledgeStop: true,
      }), runId);
    },
    releases: (id: string) =>
      request<{ releases: Release[] }>(`${project(id)}/releases`),
    prototypeReleases: (id: string) =>
      request<{ releases: Release[] }>(`${project(id)}/prototype-releases`),
    configuration: (id: string) =>
      request<ProjectConfiguration>(`${project(id)}/configuration`),
    saveConfiguration: (
      id: string,
      expectedRevision: number,
      updates: ConfigurationUpdate[],
    ) =>
      request<ProjectConfiguration>(`${project(id)}/configuration`, {
        expectedRevision,
        updates,
      }),
    release: async (id: string, port: number, dockerfile: string, expectedCommit?: string, kind?: 'reviewed-source') => {
      if (kind && (kind !== 'reviewed-source' || dockerfile !== '' || !expectedCommit || !/^[a-f0-9]{40}([a-f0-9]{24})?$/.test(expectedCommit))) throw new Error('Reviewed-source preview requires an empty Dockerfile and the full exact checked commit.');
      const value = await request<Release>(
        `${project(id)}/releases`,
        { ...(kind ? { kind } : {}), target: "local", port, dockerfile, ...(expectedCommit ? { expectedCommit } : {}) },
        "POST",
        180_000,
      );
      if (kind && (value.kind !== kind || value.projectId !== id || value.commit !== expectedCommit || value.target !== 'local')) throw new Error('The reviewed-source release response did not match the exact request. Refresh release history before any further action.');
      return value;
    },
    cancelRelease: async (id: string, releaseId: string, expectedRevision: number) => {
      if (!Number.isSafeInteger(expectedRevision) || expectedRevision < 1) throw new Error("Refresh this exact build before requesting cancellation.");
      const value = await request<Release>(`${project(id)}/releases/${encodeURIComponent(releaseId)}/cancel`, { expectedRevision });
      if (!value || value.projectId !== id || value.id !== releaseId || value.target !== "local" || value.cancelRequested !== true || !Number.isSafeInteger(value.revision) || value.revision! <= expectedRevision) {
        throw new Error("The cancellation response could not be confirmed for this build. Refresh release history before taking another action.");
      }
      return value;
    },
    stopRelease: (id: string, releaseId: string) =>
      request<Release>(
        `${project(id)}/releases/${encodeURIComponent(releaseId)}/stop`,
        {},
      ),
    cloudRelease: (id: string, input: CloudReleaseRequest) =>
      request<Release>(`${project(id)}/releases/cloud`, input, "POST", 180_000),
    pagesRelease: (id: string, input: PagesReleaseRequest) =>
      request<Release>(`${project(id)}/releases/pages`, input, "POST", 180_000),
    refreshRelease: (id: string, releaseId: string) =>
      request<Release>(
        `${project(id)}/releases/${encodeURIComponent(releaseId)}/refresh`,
        {},
      ),
    rollbackRelease: (id: string, releaseId: string) =>
      request<Release>(
        `${project(id)}/releases/${encodeURIComponent(releaseId)}/rollback`,
        {},
        "POST",
        180_000,
      ),
    prepareDeployKey: (id: string, targetId: string, repository: string) =>
      request<{
        status: string;
        privateKeyUuid?: string;
        message: string;
        publicKey?: string;
      }>(
        `${project(id)}/releases/deploy-key`,
        { targetId, repository },
        "POST",
        180_000,
      ),
  };
}
export const productApi = createProductClient();
export type ProductClient = ReturnType<typeof createProductClient>;
export const errorText = (error: unknown): string =>
  error instanceof Error ? error.message : "Something went wrong. Try again.";
