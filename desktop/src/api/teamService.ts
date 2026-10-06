import { productApi } from "./product";

export interface TeamService {
  billingEnabled?: boolean;
  mode: "self-hosted" | "managed";
  status: "active" | "suspended";
  revision: number;
  reason?: string;
  updatedAt?: string;
  updatedBy?: string;
}
export interface TeamServiceRow {
  teamId: string;
  name: string;
  service: TeamService;
}
export const teamServiceApi = {
  list: () => productApi.request<{ teams: TeamServiceRow[]; billingConnected: boolean; executionIsolationVerified: false }>("/v1/admin/team-services"),
  read: (teamId: string) => productApi.request<TeamService>(`/v1/teams/${encodeURIComponent(teamId)}/service`),
  save: (teamId: string, input: { expectedRevision: number; mode: TeamService["mode"]; status: TeamService["status"]; reason: string }) =>
    productApi.request<TeamService>(`/v1/admin/team-services/${encodeURIComponent(teamId)}`, input, "PUT"),
};
