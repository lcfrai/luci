import { ApiError, DEFAULT_BASE_URL, type ProblemCode } from "./client";

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
export interface SetupStatus {
  product: string;
  version: string;
  platform: string;
  configured: boolean;
  workspace?: Workspace;
  config: SetupConfig;
  checks: SetupCheck[];
  cloudProviders: {
    id: "aws" | "azure";
    name: string;
    strategy: string;
    description: string;
  }[];
}
export interface Workspace {
  mode: "local" | "cloud";
  connectionId: string;
  host: string;
  ready: boolean;
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
  size: "small" | "standard";
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
  branch: string;
  clean: boolean;
  changes: { path: string; status: string }[];
  log: { hash: string; subject: string }[];
  diff: string;
}
export interface Release {
  id: string;
  projectId: string;
  briefRevision: number;
  commit: string;
  target: string;
  status: string;
  url: string;
  message: string;
  createdAt: string;
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
export interface SSHRequest {
  host: string;
  port: number;
  user: string;
  privateKeyPath: string;
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
  targetId: string;
  repository: string;
  branch: string;
  commit: string;
  domain: string;
  port: number;
  healthPath: string;
  dockerfile: string;
}
export interface RuntimeJob {
  id: string;
  status: string;
  image: string;
  message: string;
  createdAt: string;
  updatedAt: string;
}
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
  provider: "aws" | "azure";
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
  async function request<T>(
    path: string,
    body?: unknown,
    method = body === undefined ? "GET" : "POST",
    budgetMs = 120_000,
  ): Promise<T> {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), budgetMs);
    try {
      const response = await fetcher(`${baseUrl.replace(/\/$/, "")}${path}`, {
        method,
        signal: controller.signal,
        headers:
          body === undefined
            ? undefined
            : { "Content-Type": "application/json" },
        body: body === undefined ? undefined : JSON.stringify(body),
      });
      const raw = await response.text();
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
          | { error?: string; message?: string }
          | undefined;
        throw new ProductApiError(
          (problem?.error || "internal") as ProblemCode,
          problem?.message || `Request failed (${response.status}).`,
          response.status,
          value,
        );
      }
      return value as T;
    } catch (error) {
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
    }
  }
  const project = (id: string) =>
    `/v1/product/projects/${encodeURIComponent(id)}`;
  return {
    request,
    status: () => request<SetupStatus>("/v1/setup/status"),
    check: () => request<SetupStatus>("/v1/setup/check", {}),
    configure: (config: SetupConfig) =>
      request<SetupStatus>("/v1/setup/config", config),
    cloudPlan: (input: CloudPlanRequest) =>
      request<CloudPlan>("/v1/setup/cloud/plan", input),
    gitIdentity: () =>
      request<{ name: string; email: string }>("/v1/setup/git/identity"),
    agents: () =>
      request<{
        providers: { id: string; name: string; configured: boolean }[];
      }>("/v1/setup/agents"),
    saveAnthropicKey: (apiKey: string) =>
      request<{
        providers: { id: string; name: string; configured: boolean }[];
      }>("/v1/setup/agents/anthropic", { apiKey }),
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
    runtime: () => request<RuntimeJob>("/v1/setup/runtime/install"),
    installRuntime: () => request<RuntimeJob>("/v1/setup/runtime/install", {}),
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
    createProject: (name: string, remoteUrl?: string) =>
      request<{ id: string; name: string; path: string }>(
        "/v1/product/projects",
        { name, ...(remoteUrl ? { remoteUrl } : {}) },
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
      `${baseUrl.replace(/\/$/, "")}${project(id)}/design/export`,
    files: (id: string, path = "") =>
      request<FileResult>(
        `${project(id)}/files?path=${encodeURIComponent(path)}`,
      ),
    git: (id: string) => request<GitState>(`${project(id)}/git`),
    commit: (id: string, message: string) =>
      request<unknown>(`${project(id)}/git/commit`, { message }),
    releases: (id: string) =>
      request<{ releases: Release[] }>(`${project(id)}/releases`),
    release: (id: string, port: number, dockerfile: string) =>
      request<Release>(
        `${project(id)}/releases`,
        { target: "local", port, dockerfile },
        "POST",
        180_000,
      ),
    stopRelease: (id: string, releaseId: string) =>
      request<Release>(
        `${project(id)}/releases/${encodeURIComponent(releaseId)}/stop`,
        {},
      ),
    cloudRelease: (id: string, input: CloudReleaseRequest) =>
      request<Release>(`${project(id)}/releases/cloud`, input, "POST", 180_000),
    refreshRelease: (id: string, releaseId: string) =>
      request<Release>(
        `${project(id)}/releases/${encodeURIComponent(releaseId)}/refresh`,
        {},
      ),
  };
}
export const productApi = createProductClient();
export type ProductClient = ReturnType<typeof createProductClient>;
export const errorText = (error: unknown): string =>
  error instanceof Error ? error.message : "Something went wrong. Try again.";
