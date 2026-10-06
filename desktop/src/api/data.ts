import { productApi } from "./product";

export interface DataBackup {
  id: string; createdAt: string; bytes: number; sha256: string; includesStorage: boolean;
}
export interface DataDomainStatus {
  projectId: string; status: string; message: string; revision: number;
  publicUrl?: string; applicationOrigin?: string; targetId?: string;
  domain?: string; checkedAt?: string; emailDelivery: "unconfigured" | "configured" | "test-accepted" | "attention";
  operation?: DataStatus["operation"];
}
export interface DataDomainRequest {
  action: "publish" | "refresh" | "stop";
  targetId?: string; domain?: string; applicationOrigin?: string;
  acknowledgePublic?: boolean; expectedRevision: number;
}
export interface DataStatus {
  projectId: string; provider: "supabase"; version: string; revision: number;
  status: string; message: string; url?: string; publishableKey?: string;
  endpointScope?: "selected-docker-host-loopback" | "browser-gateway" | "workspace-browser-pending";
  services: { name: string; status: string; health: string; image: string }[];
  operation?: { id: string; action: string; status: string; stage: string; message: string; createdAt: string; updatedAt: string };
  backups: DataBackup[]; capabilities: Record<string, boolean>;
  requirements: { memoryMiB: number; cpuMillis: number; diskGiB: number };
  checkedAt?: string; priorStackRetained: boolean;
  initializationState?: "pending" | "complete" | "unknown";
  canRecoverInstall?: boolean;
  publicDomain?: DataDomainStatus;
  publicUrl?: string;
}
export type DataType = "text" | "uuid" | "boolean" | "integer" | "bigint" | "numeric" | "timestamptz" | "jsonb";
export interface DataColumn { name: string; type: string; nullable: boolean }
export interface DataTable { name: string; rlsEnabled: boolean; columns: DataColumn[] }
export interface DataUser { id: string; email: string; created_at: string; email_confirmed_at?: string }
export interface DataBucket { id: string; name: string; public: boolean }
export interface DataObject { id?: string; name: string; metadata?: { size?: number; mimetype?: string } }
export interface DataRls { enabled: boolean; policies: { name: string; operation: string; roles: string[]; using?: string; check?: string }[] }
export interface DataPolicy { name: string; operation: "select" | "insert" | "update" | "delete" | "all"; role: "authenticated"; using: "authenticated" | "owner"; ownerColumn?: string }
const base = (id: string) => `/v1/platform/projects/${encodeURIComponent(id)}/data`;
const table = (id: string, name: string) => `${base(id)}/tables/${encodeURIComponent(name)}`;
const bucket = (id: string, name: string) => `${base(id)}/buckets/${encodeURIComponent(name)}`;
const object = (id: string, name: string, key: string) => `${bucket(id, name)}/objects/${key.split("/").map(encodeURIComponent).join("/")}`;

async function download(route: string, name: string, current: () => boolean = () => true) {
  const blob = await productApi.requestBlob(route);
  if (!current()) return;
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a"); link.href = url; link.download = name; link.click();
  window.setTimeout(() => URL.revokeObjectURL(url), 1000);
}
export const dataApi = {
  status: (id: string) => productApi.request<DataStatus>(base(id)),
  domain: (id: string) => productApi.request<DataDomainStatus>(`${base(id)}/domain`),
  configureDomain: (id: string, input: DataDomainRequest) => productApi.request<DataDomainStatus>(`${base(id)}/domain`, input),
  action: (id: string, action: "install" | "start" | "stop" | "refresh" | "backup" | "restore" | "recover-install", options: { acknowledgeResources?: boolean; backupId?: string; confirmRestore?: boolean; confirmFreshInstall?: boolean } = {}) => productApi.request<DataStatus>(base(id), { action, ...options }),
  users: (id: string) => productApi.request<{ users: DataUser[]; page: number; limit: number }>(`${base(id)}/users`),
  createUser: (id: string, email: string, password: string) => productApi.request<DataUser>(`${base(id)}/users`, { email, password, emailConfirm: true }),
  tables: (id: string) => productApi.request<{ tables: DataTable[] }>(`${base(id)}/tables`),
  createTable: (id: string, name: string, columns: DataColumn[]) => productApi.request<DataTable>(`${base(id)}/tables`, { name, columns, enableRls: true }),
  rows: (id: string, name: string) => productApi.request<{ rows: Record<string, unknown>[]; limit: number; access: string }>(`${table(id, name)}/rows`),
  insertRow: (id: string, name: string, row: Record<string, unknown>) => productApi.request<Record<string, unknown>>(`${table(id, name)}/rows`, row),
  rls: (id: string, name: string) => productApi.request<DataRls>(`${table(id, name)}/rls`),
  policy: (id: string, name: string, policy: DataPolicy) => productApi.request<DataRls>(`${table(id, name)}/rls`, { enabled: true, policy }),
  enableRls: (id: string, name: string) => productApi.request<DataRls>(`${table(id, name)}/rls`, { enabled: true }),
  buckets: (id: string) => productApi.request<DataBucket[]>(`${base(id)}/buckets`),
  createBucket: (id: string, name: string, isPublic: boolean) => productApi.request<unknown>(`${base(id)}/buckets`, { name, public: isPublic }),
  objects: (id: string, name: string, prefix = "") => productApi.request<{ objects: DataObject[]; limit: number }>(`${bucket(id, name)}/objects${prefix ? `?prefix=${encodeURIComponent(prefix)}` : ""}`),
  upload: async (id: string, name: string, key: string, file: File) => {
    if (file.size > 8 * 1024 * 1024) throw new Error("Choose a file no larger than 8 MiB.");
    await productApi.requestBlob(object(id, name, key), { method: "POST", body: file, headers: { "Content-Type": file.type || "application/octet-stream" } });
  },
  downloadObject: (id: string, name: string, key: string, current?: () => boolean) => download(object(id, name, key), key.split("/").pop() || "download", current),
  realtime: (id: string) => productApi.request<{ tables: string[]; publication: string; rlsRequired: boolean }>(`${base(id)}/realtime`),
  updateRealtime: (id: string, name: string, enabled: boolean) => productApi.request<{ tables: string[] }>(`${base(id)}/realtime`, { table: name, enabled }),
  downloadBackup: (id: string, backup: DataBackup, current?: () => boolean) => download(`${base(id)}/backups/${encodeURIComponent(backup.id)}`, `${backup.id}.zip`, current),
};
