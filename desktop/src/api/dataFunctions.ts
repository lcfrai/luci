import { productApi } from "./product";

export type FunctionAuth = "authenticated" | "project-key";
export interface FunctionFile { path: string; content: string }
export interface FunctionSource {
  name: string; path: string; sourceDigest: string; authMode: FunctionAuth;
  files: FunctionFile[]; commit?: string; committedDigest?: string; workingChanged: boolean;
}
export interface FunctionRelease {
  releaseId: string; commit: string; sourceDigest: string; authMode: FunctionAuth;
  publishedAt: string; status: "active" | "superseded" | "failed" | "stopped" | "candidate"; message: string;
}
export interface FunctionSummary {
  name: string; path: string; sourceDigest: string; authMode: FunctionAuth;
  workingChanged: boolean; builtin: boolean; active?: FunctionRelease; history: FunctionRelease[];
}
export interface FunctionCatalog {
  projectId: string; revision: number; canManage: boolean; customPublishing: boolean;
  functions: FunctionSummary[]; message: string;
  operation?: { id: string; action: "publish" | "rollback" | "remove" | "refresh"; name: string;
    status: "running" | "succeeded" | "failed"; stage: string; message: string; updatedAt: string };
}
export interface FunctionAction {
  action: "publish" | "rollback" | "remove" | "refresh"; expectedRevision: number;
  commit?: string; releaseId?: string; acknowledgeExecute?: boolean; acknowledgeRemove?: boolean;
}
export interface FunctionResponse {
  function: string; releaseId: string; statusCode: number; result: unknown; durationMs: number; contentType?: string;
}
const base = (id: string) => `/v1/platform/projects/${encodeURIComponent(id)}/data/functions`;
const path = (id: string, name: string) => `${base(id)}/${encodeURIComponent(name)}`;
export const dataFunctionsApi = {
  catalog: (id: string) => productApi.request<FunctionCatalog>(base(id)),
  source: (id: string, name: string) => productApi.request<FunctionSource>(path(id, name)),
  save: (id: string, name: string, input: { expectedDigest: string; authMode: FunctionAuth; files: FunctionFile[] }) =>
    productApi.request<FunctionSource>(`${path(id, name)}/source`, input),
  action: (id: string, name: string, input: FunctionAction) => productApi.request<FunctionCatalog>(`${path(id, name)}/action`, input),
  invoke: (id: string, name: string, input: Record<string, unknown>, applicationAccessToken?: string) =>
    productApi.request<FunctionResponse>(`${path(id, name)}/invoke`, { input, ...(applicationAccessToken ? { applicationAccessToken } : {}) }),
};
