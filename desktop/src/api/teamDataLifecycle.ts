import { productApi } from "./product";
import type { Team } from "./teams";

export interface TeamDataLifecycle {
  revision: number;
  status: "none" | "requested" | "cancelled";
  requestId?: string;
  requestedBy?: string;
  requestedAt?: string;
  updatedAt?: string;
  cancelledBy?: string;
  cancelledAt?: string;
}
export interface TeamDataLifecycleResponse { team: Team; dataLifecycle: TeamDataLifecycle }
const path = (id: string) => `/v1/teams/${encodeURIComponent(id)}/data-lifecycle`;
function verify(value: TeamDataLifecycleResponse, teamId: string): TeamDataLifecycleResponse {
  const row = value?.dataLifecycle;
  const invalid = () => { throw new Error("This data request could not be verified for the selected team. Refresh its status before continuing."); };
  if (!value?.team || value.team.id !== teamId || !Number.isSafeInteger(value.team.revision) || value.team.revision < 1 ||
      !row || !Number.isSafeInteger(row.revision) || row.revision < 0) return invalid();
  if (row.status === "none") {
    if (row.revision !== 0 || row.requestId || row.requestedBy || row.requestedAt || row.updatedAt || row.cancelledBy || row.cancelledAt) return invalid();
  } else {
    if (!["requested", "cancelled"].includes(row.status) || row.revision < 1 || !/^delete-[A-Za-z0-9_-]{22}$/.test(row.requestId ?? "") ||
        typeof row.requestedBy !== "string" || !row.requestedBy || !Number.isFinite(Date.parse(row.requestedAt ?? "")) ||
        !Number.isFinite(Date.parse(row.updatedAt ?? "")) || Date.parse(row.updatedAt!) < Date.parse(row.requestedAt!)) return invalid();
    if (row.status === "requested" && (row.cancelledBy || row.cancelledAt || row.updatedAt !== row.requestedAt)) return invalid();
    if (row.status === "cancelled" && (!row.cancelledBy || !Number.isFinite(Date.parse(row.cancelledAt ?? "")) || row.cancelledAt !== row.updatedAt)) return invalid();
  }
  return value;
}
export const teamDataLifecycleApi = {
  async get(teamId: string): Promise<TeamDataLifecycleResponse> {
    return verify(await productApi.request<TeamDataLifecycleResponse>(path(teamId)), teamId);
  },
  async request(teamId: string, expectedRevision: number): Promise<TeamDataLifecycleResponse> {
    const result = verify(await productApi.request<TeamDataLifecycleResponse>(`${path(teamId)}/request`, { expectedRevision, acknowledgeDeletion: true }), teamId);
    if (result.dataLifecycle.status !== "requested") throw new Error("The deletion request was not confirmed. Refresh its status before trying again.");
    return result;
  },
  async cancel(teamId: string, expectedRevision: number, requestId: string): Promise<TeamDataLifecycleResponse> {
    const result = verify(await productApi.request<TeamDataLifecycleResponse>(`${path(teamId)}/cancel`, { expectedRevision, requestId }), teamId);
    if (result.dataLifecycle.status !== "cancelled" || result.dataLifecycle.requestId !== requestId) throw new Error("Cancellation was not confirmed for this request. Refresh its status before continuing.");
    return result;
  },
};
