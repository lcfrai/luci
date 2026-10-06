import { productApi } from "./product";

export interface ContextChangeEvent {
  id: string;
  projectId: string;
  kind: "brief" | "specification" | "sketch" | "design" | "plan";
  action: "save" | "approve" | "apply-analysis";
  revision: number;
  digest: string;
  at: string;
  actor: { id: string; displayName: string };
  status: "applied" | "unconfirmed";
}
export interface ContextHistoryResponse { events: ContextChangeEvent[]; pruned: boolean }

function record(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
function text(value: unknown, limit: number): value is string {
  return typeof value === "string" && value.trim().length > 0 && value.length <= limit && !/[\x00\r\n]/.test(value);
}
function event(value: unknown, projectId: string): value is ContextChangeEvent {
  return record(value) && value.projectId === projectId && typeof value.id === "string" && /^context-[a-f0-9]{24}$/.test(value.id) &&
    ["brief", "specification", "sketch", "design", "plan"].includes(String(value.kind)) &&
    ["save", "approve", "apply-analysis"].includes(String(value.action)) &&
    Number.isSafeInteger(value.revision) && Number(value.revision) > 0 &&
    typeof value.digest === "string" && /^[a-f0-9]{64}$/.test(value.digest) &&
    typeof value.at === "string" && Number.isFinite(Date.parse(value.at)) &&
    record(value.actor) && text(value.actor.id, 128) && text(value.actor.displayName, 512) &&
    (value.status === "applied" || value.status === "unconfirmed");
}

export const contextHistoryApi = {
  async list(projectId: string): Promise<ContextHistoryResponse> {
    const result = await productApi.request<unknown>(`/v1/product/projects/${encodeURIComponent(projectId)}/context-history`);
    if (!record(result) || typeof result.pruned !== "boolean" || !Array.isArray(result.events) || result.events.length > 2000 ||
        !result.events.every(value => event(value, projectId)) || new Set(result.events.map(value => value.id)).size !== result.events.length) {
      throw new Error("Context activity could not be verified for this project. Refresh before continuing.");
    }
    return { events: result.events, pruned: result.pruned };
  },
};
