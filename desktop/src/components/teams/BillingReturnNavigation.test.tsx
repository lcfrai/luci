import { act, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { TeamWorkspaceControl } from './TeamWorkspaceControl';
import { teamsApi, emptyGrants, personalPermissions, type TeamDetail, type TeamSummary } from '../../api/teams';
import { teamBillingApi, type TeamBillingState } from '../../api/teamBilling';
import { resetCockpit, useCockpit } from '../../store/store';

const workspace = vi.hoisted(() => ({ selected: null as TeamSummary | null, teams: [] as TeamSummary[], refresh: vi.fn(), select: vi.fn(), revision: 0, teamId: '', managedTeamId: '' }));
vi.mock('./TeamWorkspaceProvider', () => ({ useTeamWorkspace: () => workspace }));
vi.mock('./TeamDataLifecycle', () => ({ TeamDataLifecycle: () => null }));
vi.mock('./TeamAudit', () => ({ TeamAudit: () => null }));
vi.mock('./TeamInvitations', () => ({ TeamInvitations: () => null, JoinTeamInvitation: () => null }));
vi.mock('../auth/TeamServiceStatus', async () => {
  const { TeamBilling } = await import('../auth/TeamBilling');
  return { TeamServiceStatus: ({ teamId }: { teamId: string }) => <TeamBilling teamId={teamId} /> };
});

const teamA = 'team-' + 'a'.repeat(22), teamB = 'team-' + 'b'.repeat(22);
const member = { userId: 'alice', role: 'owner' as const, grants: { ...emptyGrants, admin: true }, revision: 1, user: { id: 'alice', username: 'alice', displayName: 'Alice', role: 'member' as const, disabled: false } };
const detail = (id = teamA, name = 'Garden'): TeamDetail => ({ team: { id, name, revision: 1, createdAt: '', updatedAt: '' }, membership: member, permissions: personalPermissions, members: [member], executionBoundary: 'Separate workspace' });
const billing: TeamBillingState = { enabled: true, owner: true, executionAllowed: false, separateCosts: 'Separate costs apply.', status: 'unsubscribed', revision: 1 };
function returnURL(kind: string, team = teamA) { window.history.replaceState(null, '', `/?billing=${kind}&billingTeam=${team}`); }
function noBillingMutation() {
  for (const action of ['quote', 'checkout', 'portal', 'refresh', 'reconcile'] as const) expect(teamBillingApi[action]).not.toHaveBeenCalled();
}
beforeEach(() => {
  resetCockpit(); window.history.replaceState(null, '', '/');
  workspace.selected = detail(); workspace.teams = [detail()]; workspace.teamId = teamA; workspace.managedTeamId = teamA;
  workspace.select.mockReset(); workspace.refresh.mockReset().mockResolvedValue(undefined);
  vi.spyOn(teamsApi, 'detail').mockImplementation(async id => detail(id, id === teamA ? 'Garden' : 'Orchard'));
  vi.spyOn(teamBillingApi, 'read').mockResolvedValue(billing);
  vi.spyOn(teamBillingApi, 'quote').mockResolvedValue(billing);
  vi.spyOn(teamBillingApi, 'checkout').mockResolvedValue({ url: 'https://checkout.stripe.com/c/pay/fixture' });
  vi.spyOn(teamBillingApi, 'portal').mockResolvedValue({ url: 'https://billing.stripe.com/p/fixture' });
  vi.spyOn(teamBillingApi, 'refresh').mockResolvedValue({ ...billing, status: 'active', executionAllowed: true });
  vi.spyOn(teamBillingApi, 'reconcile').mockResolvedValue(billing);
});
afterEach(() => { vi.restoreAllMocks(); window.history.replaceState(null, '', '/'); });

it.each([
  ['return', 'Back from checkout'], ['cancel', 'Checkout closed'], ['portal', 'Back from subscription management'],
])('opens the matched %s return on Service and data with server-owned status only', async (kind, heading) => {
  returnURL(kind); render(<TeamWorkspaceControl />);
  await screen.findByRole('heading', { name: heading });
  expect(screen.getByRole('tab', { name: 'Service and data' }).getAttribute('aria-selected')).toBe('true');
  await screen.findByText(/New team execution is unavailable/);
  expect(teamBillingApi.read).toHaveBeenCalledExactlyOnceWith(teamA);
  expect(workspace.select).not.toHaveBeenCalled(); noBillingMutation();
  expect(window.location.search).toBe('');
  expect(screen.getByText(/This return link does not confirm payment or activate access/)).not.toBeNull();
});

it('requires a deliberate verification action after return; a URL cannot grant execution', async () => {
  returnURL('return'); window.history.replaceState(null, '', window.location.search + '&status=active&session_id=forged');
  render(<TeamWorkspaceControl />);
  const verify = await screen.findByRole('button', { name: 'Verify subscription with Stripe' });
  expect(screen.getByText(/New team execution is unavailable/)).not.toBeNull(); noBillingMutation();
  await userEvent.click(verify);
  await screen.findByText(/Team execution is available/);
  expect(teamBillingApi.refresh).toHaveBeenCalledExactlyOnceWith(teamA);
  expect(teamBillingApi.checkout).not.toHaveBeenCalled();
});

it('offers the existing reserved checkout after cancellation without creating or resuming it automatically', async () => {
  vi.mocked(teamBillingApi.read).mockResolvedValue({ ...billing, checkoutState: 'open', checkoutRecoverable: true, quote: {
    id: 'retained-quote', expiresAt: 2000000000, termsUrl: 'https://app.example/terms', privacyUrl: 'https://app.example/privacy',
    price: { priceId: 'price-retained', unitAmount: 2400, currency: 'usd', interval: 'month', intervalCount: 1, taxBehavior: 'exclusive' },
  } });
  returnURL('cancel'); render(<TeamWorkspaceControl />);
  const resume = await screen.findByRole('button', { name: 'Resume reserved checkout' });
  expect((resume as HTMLButtonElement).disabled).toBe(true);
  expect(screen.getByText(/Closing checkout does not cancel an existing subscription/)).not.toBeNull(); noBillingMutation();
});

it('resolves old links only from authenticated managed-team binding', async () => {
  window.history.replaceState(null, '', '/?billing=return'); render(<TeamWorkspaceControl />);
  await screen.findByRole('heading', { name: 'Back from checkout' });
  expect(teamBillingApi.read).toHaveBeenCalledExactlyOnceWith(teamA); noBillingMutation();
});

it.each([
  '?billing=return', '?billing=return&billingTeam=unknown', '?billing=return&billingTeam=' + teamB,
  '?billing=return&billing=cancel&billingTeam=' + teamA,
])('explains unavailable or ambiguous %s links without choosing a team or reading billing', async query => {
  workspace.managedTeamId = '';
  window.history.replaceState(null, '', '/' + query); render(<TeamWorkspaceControl />);
  await screen.findByRole('dialog', { name: 'Billing return' });
  expect(screen.getByText(/could not be matched to a team available/)).not.toBeNull();
  expect(workspace.select).not.toHaveBeenCalled(); expect(teamsApi.detail).not.toHaveBeenCalled(); expect(teamBillingApi.read).not.toHaveBeenCalled(); noBillingMutation();
  await userEvent.click(screen.getByRole('button', { name: 'Back to my workspace' }));
  expect(window.location.search).toBe('');
  expect(screen.queryByRole('dialog')).toBeNull();
});

it('does not follow a team hint outside the managed service binding even for a listed member', async () => {
  workspace.teams = [detail(), detail(teamB, 'Orchard')]; returnURL('return', teamB);
  render(<TeamWorkspaceControl />); await screen.findByRole('dialog', { name: 'Billing return' });
  expect(screen.queryByRole('button', { name: 'Open Orchard billing' })).toBeNull();
  expect(workspace.select).not.toHaveBeenCalled(); expect(teamsApi.detail).not.toHaveBeenCalled(); noBillingMutation();
});

it('requires an explicit validated team choice and preserves the existing draft-switch guard', async () => {
  workspace.managedTeamId = ''; workspace.selected = detail(teamB, 'Orchard'); workspace.teamId = teamB; workspace.teams = [detail(), detail(teamB, 'Orchard')];
  const retainedDraft = { projectId: 'p-1', lens: 'discover' as const, label: 'Definition', retained: true, discard: vi.fn() };
  useCockpit.setState({ unsavedProjectDrafts: { 'private-draft': retainedDraft } });
  returnURL('return'); const mounted = render(<TeamWorkspaceControl />);
  await screen.findByRole('button', { name: 'Open Garden billing' });
  expect(workspace.select).not.toHaveBeenCalled(); expect(teamBillingApi.read).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole('button', { name: 'Open Garden billing' }));
  await waitFor(() => expect(workspace.select).toHaveBeenCalledExactlyOnceWith(teamA));
  expect(teamsApi.detail).toHaveBeenCalledExactlyOnceWith(teamA);
  expect(useCockpit.getState().unsavedProjectDrafts['private-draft']).toBe(retainedDraft);
  expect(retainedDraft.discard).not.toHaveBeenCalled();
  // The actual provider can keep the old selection while its draft dialog is
  // pending/cancelled. No billing read or URL consumption occurs before a switch.
  expect(await screen.findByRole('button', { name: 'Continue billing return' })).not.toBeNull();
  expect(window.location.search).toContain('billingTeam='); expect(teamBillingApi.read).not.toHaveBeenCalled(); noBillingMutation();
  // A confirmed provider switch remounts the scoped application subtree.
  mounted.unmount(); workspace.selected = detail(); workspace.teamId = teamA;
  render(<TeamWorkspaceControl />); await screen.findByRole('heading', { name: 'Back from checkout' });
  await waitFor(() => expect(teamBillingApi.read).toHaveBeenCalledExactlyOnceWith(teamA));
  expect(window.location.search).toBe('');
});

it('keeps the current workspace when membership was revoked after the team list loaded', async () => {
  workspace.managedTeamId = ''; workspace.selected = detail(teamB, 'Orchard'); workspace.teamId = teamB; workspace.teams = [detail(), detail(teamB, 'Orchard')];
  vi.mocked(teamsApi.detail).mockRejectedValue(new Error('Team access is no longer available.'));
  returnURL('portal'); render(<TeamWorkspaceControl />);
  await userEvent.click(await screen.findByRole('button', { name: 'Open Garden billing' }));
  await screen.findByRole('alert'); expect(workspace.select).not.toHaveBeenCalled(); expect(teamBillingApi.read).not.toHaveBeenCalled(); noBillingMutation();
});

it('ignores a late membership response after the current account/workspace identity changes', async () => {
  workspace.managedTeamId = ''; workspace.selected = detail(teamB, 'Orchard'); workspace.teamId = teamB; workspace.teams = [detail(), detail(teamB, 'Orchard')];
  let finish!: (value: TeamDetail) => void;
  vi.mocked(teamsApi.detail).mockReturnValue(new Promise(resolve => { finish = resolve; }));
  returnURL('return'); const mounted = render(<TeamWorkspaceControl />);
  await userEvent.click(await screen.findByRole('button', { name: 'Open Garden billing' }));
  workspace.selected = { ...detail(teamB, 'Orchard'), membership: { ...member, userId: 'bob' } };
  mounted.rerender(<TeamWorkspaceControl />);
  await act(async () => { finish(detail()); });
  expect(workspace.select).not.toHaveBeenCalled(); expect(teamBillingApi.read).not.toHaveBeenCalled(); noBillingMutation();
});
