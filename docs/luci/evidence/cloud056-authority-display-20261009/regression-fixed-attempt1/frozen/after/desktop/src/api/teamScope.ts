// Workspace selection changes request paths, never authentication identity.
// The server resolves membership again for every request and socket upgrade.
export const TEAM_WORKSPACE_CHANGED = 'luci-team-workspace-changed';
export const TEAM_WORKSPACE_ACCESS_DENIED = 'luci-team-workspace-access-denied';
export interface TeamAccessDenial { teamId: string; generation: number }
/** A confirmed catalogue membership denial, never a generic403 or a new grant. */
export function reportTeamAccessDenied(requestGeneration: number): void {
  if (!teamId || requestGeneration !== generation || typeof window === 'undefined') return;
  window.dispatchEvent(new CustomEvent<TeamAccessDenial>(TEAM_WORKSPACE_ACCESS_DENIED, { detail: { teamId, generation } }));
}
let teamId = '';
let generation = 0;
export function selectedTeamId() { return teamId; }
export function teamScopeGeneration() { return generation; }
export function selectTeamScope(value: string) {
  if (value && !/^team-[A-Za-z0-9_-]{22}$/.test(value)) throw new Error('Invalid team workspace.');
  teamId = value; generation++;
  if (typeof window !== 'undefined') window.dispatchEvent(new Event(TEAM_WORKSPACE_CHANGED));
}
export function teamScopePrefix() { return teamId ? `/v1/teams/${teamId}/workspace` : ''; }
export function workspacePath(path: string) {
  if (!teamId || !path.startsWith('/v1/') || /^\/v1\/(auth|teams|admin|control-plane)(\/|$)/.test(path) || path.startsWith('/v1/setup/dependencies')) return path;
  return teamScopePrefix() + path;
}
