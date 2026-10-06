export type BillingReturnKind = 'return' | 'cancel' | 'portal';
export type BillingReturnHint = { valid: true; kind: BillingReturnKind; teamId: string | null } | { valid: false };
const teamID = /^team-[A-Za-z0-9_-]{22}$/;

// This is a navigation hint, never a payment receipt or account/workspace
// authority. Keep only the bounded fields; do not persist the incoming URL.
export function readBillingReturn(href = window.location.href): BillingReturnHint | null {
  const query = new URL(href).searchParams;
  if (!query.has('billing') && !query.has('billingTeam')) return null;
  const kinds = query.getAll('billing'), teams = query.getAll('billingTeam');
  if (kinds.length !== 1 || !['return', 'cancel', 'portal'].includes(kinds[0]) || teams.length > 1 || (teams.length === 1 && !teamID.test(teams[0]))) return { valid: false };
  return { valid: true, kind: kinds[0] as BillingReturnKind, teamId: teams[0] ?? null };
}

export function consumeBillingReturn(): void {
  const url = new URL(window.location.href);
  url.searchParams.delete('billing'); url.searchParams.delete('billingTeam');
  window.history.replaceState(window.history.state, '', url.pathname + url.search + url.hash);
}
