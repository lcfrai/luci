import { productApi } from "./product";
import { authToken } from "./authSession";

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
  imported?: boolean;
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

// Only classifies the server-returned presentation. Browser admission still
// resolves the exact configured domain and private team/project registration.
export function isPublicGatewayHostname(hostname: string): boolean {
  return /^p-[a-z2-7]{51}[aq]\.(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/.test(hostname) && hostname.length <= 253 && !/[.](?:localhost|local|internal)$/.test(hostname);
}

export function isolatedBrowserURL(raw: string): string | undefined {
  try {
    const url = new URL(raw);
    const controlOrigin = new URL(productApi.baseUrl).origin;
    if (url.username || url.password || url.origin === window.location.origin || url.origin === controlOrigin) return undefined;
    return ((url.protocol === "http:" && !!url.port && (url.hostname === "127.0.0.1" || isGatewayHostname(url.hostname))) || (url.protocol === "https:" && !url.port && isPublicGatewayHostname(url.hostname))) ? url.href : undefined;
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
  async importDatabaseBackup(id: string, file: File, trusted: boolean, signal?: AbortSignal, current: () => boolean = () => true): Promise<PlatformBackup> {
    if (!trusted) throw new Error("Confirm that you trust this dump before importing it.");
    if (file.size < 5 || file.size > 64 * 1024 * 1024) throw new Error("Choose a PostgreSQL custom-format dump up to 64 MiB.");
    signal?.throwIfAborted();
    const workspace = productApi.baseUrl, session = authToken();
    const bytes = await file.arrayBuffer();
    if (String.fromCharCode(...new Uint8Array(bytes, 0, 5)) !== "PGDMP") throw new Error("Choose a PostgreSQL custom-format dump, not SQL text or a ZIP archive.");
    const hash = await crypto.subtle.digest("SHA-256", bytes);
    const digest = Array.from(new Uint8Array(hash), byte => byte.toString(16).padStart(2, "0")).join("");
    signal?.throwIfAborted();
    if (!current() || workspace !== productApi.baseUrl || session !== authToken()) throw new Error("The account or workspace changed. Select the backup again in the intended project.");
    const response = await productApi.requestBlob(`${project(id)}/backups/import`, {
      method: "POST", body: file, headers: { "Content-Type": "application/octet-stream", "X-Luci-Backup-SHA256": digest, "X-Luci-Trust-Backup": "true" },
    }, 120_000, signal);
    const backup = JSON.parse(await response.text()) as PlatformBackup;
    if (!backup.id || backup.sha256 !== digest || backup.bytes !== file.size) throw new Error("The imported backup receipt does not match the selected file. Refresh its history before retrying.");
    return backup;
  },
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
  async downloadBackup(id: string, backup: PlatformBackup, current: () => boolean = () => true) {
    const blob = await productApi.requestBlob(
      `${project(id)}/backups/${encodeURIComponent(backup.id)}`,
    );
    if (!current()) return;
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = `${backup.id}.dump`;
    link.click();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  },
};
