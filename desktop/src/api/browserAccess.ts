import { productApi } from './product';
import { isGatewayHostname, isPublicGatewayHostname } from './platform';
import { selectedTeamId, teamScopeGeneration } from './teamScope';

// Team app origins are protected by a host-only browser cookie. Never retain
// the one-use handoff URL in project state or substitute a Luci login token.
export async function prepareBrowserAccess(projectId: string, address: string): Promise<string> {
  const teamId = selectedTeamId();
  if (!teamId) return address;
  const generation = teamScopeGeneration();
  const target = new URL(address);
  if (target.username || target.password) throw new Error('Use the verified browser address shown by Luci.');
  // Intentionally public releases have their own application authentication.
  if (target.protocol === 'https:' && !isGatewayHostname(target.hostname) && !isPublicGatewayHostname(target.hostname)) return address;
  if (!((target.protocol === 'http:' && isGatewayHostname(target.hostname)) || (target.protocol === 'https:' && !target.port && isPublicGatewayHostname(target.hostname))))
    throw new Error('This team preview needs Luci’s isolated browser gateway. Refresh the release after starting the gateway.');
  const response = await productApi.request<{url: string}>(`/v1/teams/${encodeURIComponent(teamId)}/projects/${encodeURIComponent(projectId)}/browser-access`, {url: address});
  if (generation !== teamScopeGeneration() || teamId !== selectedTeamId())
    throw new Error('The workspace changed before the browser was ready. Open the preview again.');
  const handoff = new URL(response.url);
  if (handoff.origin !== target.origin || handoff.pathname !== '/.well-known/luci-team-access' || handoff.username || handoff.password || handoff.hash || [...handoff.searchParams.keys()].join() !== 'ticket' || !/^[A-Za-z0-9_-]{40,128}$/.test(handoff.searchParams.get('ticket') || ''))
    throw new Error('Luci could not verify this project’s browser invitation. Refresh and try again.');
  return handoff.href;
}
