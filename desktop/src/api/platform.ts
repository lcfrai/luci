import { productApi } from "./product";
import { DEFAULT_BASE_URL } from "./client";
import { authHeaders, expireAuth } from "./authSession";

export interface PlatformDeployment {
  id: string;
  projectId: string;
  provider: "docker" | "panacea";
  commit: string;
  configurationRevision: number;
  status: string;
  url?: string;
  browserConnectionRequired?: boolean;
  message: string;
  image?: string;
  createdAt: string;
  updatedAt: string;
  pipelineRun?: string;
}
export interface PlatformDatabase {
  status: string;
  engine: string;
  host?: string;
  port?: number;
  database?: string;
  username?: string;
  message: string;
}
export interface PlatformBackup {
  id: string;
  createdAt: string;
  bytes: number;
  sha256: string;
  database?: string;
}
export interface PlatformDatabaseOperation {
  id: string;
  action: string;
  status: "running" | "succeeded" | "failed" | "interrupted";
  stage: string;
  message: string;
  createdAt: string;
  updatedAt: string;
  backupId?: string;
  restoreId?: string;
  previousDatabase?: string;
  candidateDatabase?: string;
}
export interface PlatformProject {
  projectId: string;
  deployments: PlatformDeployment[];
  database: PlatformDatabase;
  backups: PlatformBackup[];
  databaseOperation?: PlatformDatabaseOperation;
  databaseRestoreHistory?: PlatformDatabaseOperation[];
  panaceaProject?: string;
}
export interface PlatformStatus {
  dockerReady: boolean;
  scope: string;
  limits: { memoryMiB: number; cpuMillis: number };
  permissions: { connectPanacea: boolean; deployPanacea: boolean };
  supabase: { status: string; message: string; upstream: string };
}
export interface PanaceaConnection {
  configured: boolean;
  baseUrl?: string;
  checkedAt?: string;
  canManage: boolean;
}
export interface PanaceaProject {
  name: string;
  displayName: string;
  repoUrl: string;
  defaultBranch: string;
  status: string;
  url: string;
}
const base = "/v1/platform";
const project = (id: string) => `${base}/projects/${encodeURIComponent(id)}`;
export interface PlatformBrowserConnection {
  url: string;
  projectId: string;
  kind: "application" | "data";
  resourceId: string;
}

export function isGatewayHostname(hostname: string): boolean {
  return /^[a-f0-9]{32}\.[a-f0-9]{32}\.localhost$/.test(hostname);
}

export function isolatedBrowserURL(raw: string): string | undefined {
  try {
    const url = new URL(raw);
    const controlOrigin = new URL(window.__LUCI_API__ || DEFAULT_BASE_URL).origin;
    if (url.username || url.password || url.origin === window.location.origin || url.origin === controlOrigin) return undefined;
    return url.protocol === "http:" && !!url.port &&
      (url.hostname === "127.0.0.1" || isGatewayHostname(url.hostname))
      ? url.href : undefined;
  } catch { return undefined; }
}

export const platformApi = {
  prepareBrowser: (projectId: string, kind: "application" | "data", resourceId?: string) =>
    productApi.request<PlatformBrowserConnection>("/v1/setup/workspace/platform/browser", {
      projectId, kind, ...(resourceId ? { resourceId } : {}),
    }),
  status: () => productApi.request<PlatformStatus>(`${base}/status`),
  project: (id: string) =>
    productApi.request<{ project: PlatformProject; busy: boolean }>(
      project(id),
    ),
  deploy: (id: string, commit: string, port: number, healthPath: string) =>
    productApi.request<PlatformDeployment>(`${project(id)}/deployments`, {
      commit,
      port,
      healthPath,
    }),
  action: (
    id: string,
    deploymentId: string,
    action: "stop" | "restart" | "refresh",
  ) =>
    productApi.request<PlatformDeployment>(
      `${project(id)}/deployments/${encodeURIComponent(deploymentId)}/${action}`,
      {},
    ),
  logs: (id: string, deploymentId: string) =>
    productApi.request<{ text: string; notice: string }>(
      `${project(id)}/deployments/${encodeURIComponent(deploymentId)}/logs`,
    ),
  database: (id: string, action: "start" | "stop" | "backup") =>
    productApi.request<PlatformDatabase | PlatformBackup>(
      `${project(id)}/database`,
      { action },
      "POST",
      200_000,
    ),
  restoreDatabase: (id: string, backupId: string) =>
    productApi.request<PlatformDatabaseOperation>(`${project(id)}/database`, {
      action: "restore",
      backupId,
      confirmRestore: true,
      acknowledgeResources: true,
    }),
  recoverPreviousDatabase: (id: string, restoreId: string) =>
    productApi.request<PlatformDatabaseOperation>(`${project(id)}/database`, {
      action: "rollback-restore",
      restoreId,
      confirmRestore: true,
      acknowledgeResources: true,
    }),
  connection: () => productApi.request<PanaceaConnection>(`${base}/panacea`),
  connect: (baseUrl: string, apiKey: string) =>
    productApi.request<PanaceaConnection>(
      `${base}/panacea`,
      { baseUrl, apiKey },
      "PUT",
    ),
  disconnect: () =>
    productApi.request<PanaceaConnection>(
      `${base}/panacea`,
      undefined,
      "DELETE",
    ),
  projects: () =>
    productApi.request<{ projects: PanaceaProject[] }>(
      `${base}/panacea/projects`,
    ),
  mapProject: (id: string, projectName: string) =>
    productApi.request<{ project: PlatformProject; busy: boolean }>(
      `${project(id)}/panacea`,
      { projectName },
      "PUT",
    ),
  deployPanacea: (id: string, commit: string, branch: string) =>
    productApi.request<PlatformDeployment>(`${project(id)}/panacea/deploy`, {
      commit,
      branch,
    }),
  refreshPanacea: (id: string, deploymentId: string) =>
    productApi.request<PlatformDeployment>(
      `${project(id)}/panacea/deployments/${encodeURIComponent(deploymentId)}/refresh`,
      {},
    ),
  async downloadBackup(id: string, backup: PlatformBackup) {
    const response = await fetch(
      `${DEFAULT_BASE_URL}${project(id)}/backups/${encodeURIComponent(backup.id)}`,
      { headers: authHeaders(), signal: AbortSignal.timeout(60_000) },
    );
    if (response.status === 401) expireAuth();
    if (!response.ok)
      throw new Error("The verified database backup could not be downloaded.");
    const blob = await response.blob();
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = `${backup.id}.dump`;
    link.click();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  },
};
