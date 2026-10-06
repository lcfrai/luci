import { productApi } from "./product";
export interface TeamAuditEvent { id: string; at: string; actorId: string; action: string; target: string; outcome: string; actor: { id: string; displayName: string } }
export interface TeamAuditResponse { teamId: string; events: TeamAuditEvent[]; limit: number; installationRetentionLimit: number }
export const teamAuditApi = {
  async get(teamId: string): Promise<TeamAuditResponse> {
    const result = await productApi.request<TeamAuditResponse>(`/v1/teams/${encodeURIComponent(teamId)}/audit`);
    if (!result || result.teamId !== teamId || !Array.isArray(result.events) || result.events.length > 200 ||
      result.limit !== 200 || result.installationRetentionLimit !== 1000 || !result.events.every(event =>
        !!event && event.target === teamId && typeof event.action === "string" && event.action.startsWith("team.") &&
        typeof event.id === "string" && typeof event.outcome === "string" && Number.isFinite(Date.parse(event.at)) &&
        !!event.actor && typeof event.actorId === "string" && event.actor.id === event.actorId && typeof event.actor.displayName === "string" && !!event.actor.displayName.trim()) ||
        new Set(result.events.map(event => event.id)).size !== result.events.length) {
      throw new Error("This activity could not be verified for the selected team. Refresh before continuing.");
    }
    return result;
  },
};
