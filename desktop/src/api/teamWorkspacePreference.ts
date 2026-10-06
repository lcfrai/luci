export function teamWorkspaceStorageKey(baseUrl: string, userId: string) {
  return JSON.stringify(["luci.workspace", baseUrl.replace(/\/v1\/teams\/[^/]+\/workspace$/, ""), userId]);
}

// This is a navigation preference, never proof of membership. The workspace
// provider resolves current access before displaying any selected team.
export function rememberInvitedTeam(baseUrl: string, userId: string, teamId: string) {
  if (!userId || !/^team-[A-Za-z0-9_-]{22}$/.test(teamId)) return;
  try { localStorage.setItem(teamWorkspaceStorageKey(baseUrl, userId), teamId); }
  catch { /* Joining succeeded; the team remains available in the workspace selector. */ }
}
