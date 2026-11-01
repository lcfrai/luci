import { act, cleanup, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { CommercialSignals } from './CommercialSignals';
import { commercialSignalsApi, type CommercialView } from '../../api/commercialSignals';
const scope = vi.hoisted(() => ({ current: 'alice/workspace/project' }));
const access = vi.hoisted(() => ({ canRead: true, canEdit: true, canAssist: true, loading: false, error: '', permissions: { secrets: true } }));
const team = vi.hoisted(() => ({ teamId: 'team-one', revision: 1, accessBlocked: false }));
const draftProbe = vi.hoisted(() => ({ dirty: false }));
vi.mock('../../hooks/useProjectDraft', () => ({ useProjectDraftScope: () => scope.current }));
vi.mock('../../hooks/useProjectAccess', () => ({ useProjectAccess: () => access }));
vi.mock('../../hooks/useUnsavedProjectDraft', () => ({ useUnsavedProjectDraft: (_id: string, dirty: boolean) => { draftProbe.dirty = dirty; } }));
vi.mock('../teams/TeamWorkspaceProvider', () => ({ useTeamWorkspace: () => team }));
vi.mock('../orchestrator/ObservationRouting', () => ({ ObservationRouting: () => <p>Existing bounded observation controls</p> }));
vi.mock('../../api/commercialSignals', async importOriginal => { const actual = await importOriginal<typeof import('../../api/commercialSignals')>(); return { ...actual, commercialSignalsApi: { get: vi.fn(), stripe: vi.fn(), sources: vi.fn(), releases: vi.fn(), refresh: vi.fn() } }; });
function saved(): CommercialView { return { projectId: 'p-one', configuration: { revision: 2, stripe: { id: 'stripe-one', name: 'Product receipts', accountId: 'acct_one', live: false, revision: 1 }, stripeEnabled: false, stripeFilter: null, windowDays: 30, releases: [], automaticReads: false }, snapshots: [], status: [], prunedSnapshots: 0, refreshMinutes: 15, collectionRecordLimit: 100000 }; }
beforeEach(() => { vi.clearAllMocks(); scope.current = 'alice/workspace/project'; access.permissions.secrets = true; draftProbe.dirty = false; team.revision = 1; vi.mocked(commercialSignalsApi.get).mockResolvedValue(saved()); vi.mocked(commercialSignalsApi.releases).mockResolvedValue([]); });
afterEach(() => cleanup());
it('reads only saved data on entry and explicitly enables an attributed source and existing automatic route', async () => {
 const user = userEvent.setup(); vi.mocked(commercialSignalsApi.sources).mockImplementation(async (_id, input) => ({ ...saved(), configuration: { ...saved().configuration, revision: 3, stripeEnabled: input.stripeEnabled, stripeFilter: input.stripeFilter, automaticReads: input.automaticReads } }));
 render(<CommercialSignals projectId="p-one"/>); await waitFor(() => expect(commercialSignalsApi.get).toHaveBeenCalledTimes(1));
 await waitFor(() => expect(commercialSignalsApi.get).toHaveBeenCalledTimes(1)); expect(commercialSignalsApi.refresh).not.toHaveBeenCalled();
 await user.click(screen.getByText('Connect and choose sources'));
 await user.click(screen.getByLabelText('Enable captured-charge aggregates from the saved Stripe account'));
 await user.click(screen.getByLabelText('Attribute this project using an exact CHARGE metadata match'));
 await user.type(screen.getByLabelText('Exact metadata value'), 'product-one');
 await user.click(screen.getByLabelText(/Allow automatic bounded read-only refreshes/));
 await user.click(screen.getByLabelText(/Share complete source-labelled aggregates/)); await user.click(screen.getByRole('button', { name: 'Save source consent' }));
 await waitFor(() => expect(commercialSignalsApi.sources).toHaveBeenCalledWith('p-one', expect.objectContaining({ expectedRevision: 2, stripeEnabled: true, stripeFilter: { key: 'luci_project', value: 'product-one' }, automaticReads: true, acknowledgeSharing: true }), expect.any(AbortSignal)));
 await user.click(screen.getByRole('button', { name: 'Configure bounded findings' })); expect(screen.getByText('Existing bounded observation controls')).toBeTruthy(); expect(commercialSignalsApi.refresh).not.toHaveBeenCalled();
});
it('preserves a dirty window on saved readback and requires a fresh baseline after another writer changes settings', async () => {
 const user = userEvent.setup(); render(<CommercialSignals projectId="p-one"/>); await waitFor(() => expect(commercialSignalsApi.get).toHaveBeenCalledTimes(1));
 await user.click(screen.getByText('Connect and choose sources')); const days = screen.getByLabelText('Previous complete UTC days'); await user.clear(days); await user.type(days, '7');
 vi.mocked(commercialSignalsApi.get).mockResolvedValue({ ...saved(), configuration: { ...saved().configuration, revision: 3, windowDays: 14 } });
 await user.click(screen.getByRole('button', { name: 'Read saved commercial results' })); await screen.findByText(/Saved settings changed while your draft was open/);
 expect((days as HTMLInputElement).value).toBe('7'); expect(draftProbe.dirty).toBe(true);
 await user.click(screen.getByLabelText(/Share complete source-labelled aggregates/)); expect(screen.getByRole('button', { name: 'Save source consent' }).matches(':disabled')).toBe(true); expect(commercialSignalsApi.sources).not.toHaveBeenCalled();
});
it('withdraws credentials and late private results when Secrets or workspace scope changes', async () => {
 const user = userEvent.setup(), mounted = render(<CommercialSignals projectId="p-one"/>); await waitFor(() => expect(commercialSignalsApi.get).toHaveBeenCalledTimes(1)); await user.click(screen.getByText('Connect and choose sources'));
 const key = screen.getByLabelText('Restricted Stripe key'); await user.type(key, 'rk_test_privatevalue1234567890');
 let resolve!: (value: CommercialView) => void; vi.mocked(commercialSignalsApi.get).mockImplementationOnce(() => new Promise(done => { resolve = done; }));
 await user.click(screen.getByRole('button', { name: 'Read saved commercial results' }));
 access.permissions.secrets = false; scope.current = 'bob/another-workspace/project'; mounted.rerender(<CommercialSignals projectId="p-one"/>);
 await act(async () => resolve(saved())); await user.click(screen.getByText('Connect and choose sources'));
 expect((screen.getByLabelText('Restricted Stripe key') as HTMLInputElement).value).toBe(''); expect(screen.getByRole('button', { name: 'Verify and save restricted key' }).matches(':disabled')).toBe(true); expect(commercialSignalsApi.stripe).not.toHaveBeenCalled();
});
it('labels completed UTC unallocated figures and previous snapshots without inferring project revenue', async () => {
 const value = saved(), hash = 'a'.repeat(64); value.snapshots = [{ id: 'commercial-' + hash, digest: hash, sourceKey: 'stripe:acct_one:test:account', provider: 'stripe', label: 'Account receipts', accountId: 'acct_one', live: false, periodStart: '2026-10-01', periodEndExclusive: '2026-11-01', startedAt: '2026-11-01T01:00:00Z', observedAt: '2026-11-01T01:01:00Z', records: 1, matchedCharges: 1, stripeFilter: null, attribution: 'account-unallocated', currencies: [{ currency: 'JPY', capturedPayments: 1, capturedMinor: 1000, refundedMinor: 0, disputedPayments: 0, paymentsWithoutCustomer: 0 }], payingCustomerReferences: 1, assetDownloads: 0, reactions: null, semantics: 'Integer native units, not accounting revenue.' }];
 vi.mocked(commercialSignalsApi.get).mockResolvedValue(value); const user = userEvent.setup(); render(<CommercialSignals projectId="p-one"/>); await waitFor(() => expect(commercialSignalsApi.get).toHaveBeenCalledTimes(1));
 await user.click(screen.getByLabelText('Show previous snapshots, including changed or paused sources'));
 const results = screen.getByRole('region', { name: 'Complete commercial snapshots' }); expect(within(results).getByText(/Previous snapshot/)).toBeTruthy(); expect(within(results).getByText(/ACCOUNT-WIDE · UNALLOCATED/)).toBeTruthy(); expect(within(results).getByRole('columnheader', { name: 'Captured amount' })).toBeTruthy();
 expect(screen.getByRole('button', { name: 'Review commercial findings with Luci' }).matches(':disabled')).toBe(true); expect(commercialSignalsApi.refresh).not.toHaveBeenCalled();
});

it('keeps source/key drafts across unchanged team membership refresh and pauses using saved settings even with an invalid draft', async () => {
 const user = userEvent.setup(), initial = saved(); initial.configuration.stripeEnabled = true;
 vi.mocked(commercialSignalsApi.get).mockResolvedValue(initial);
 vi.mocked(commercialSignalsApi.sources).mockResolvedValue({ ...initial, configuration: { ...initial.configuration, revision: 3, stripeEnabled: false } });
 const mounted = render(<CommercialSignals projectId="p-one"/>); await waitFor(() => expect(commercialSignalsApi.get).toHaveBeenCalledTimes(1));
 await user.click(screen.getByText('Connect and choose sources')); await user.type(screen.getByLabelText('Restricted Stripe key'), 'rk_test_privatevalue1234567890');
 await user.clear(screen.getByLabelText('Previous complete UTC days'));
 await user.click(screen.getByLabelText('Attribute this project using an exact CHARGE metadata match'));
 team.revision++; mounted.rerender(<CommercialSignals projectId="p-one"/>);
 expect((screen.getByLabelText('Restricted Stripe key') as HTMLInputElement).value).toBe('rk_test_privatevalue1234567890'); expect(commercialSignalsApi.get).toHaveBeenCalledTimes(1);
 await user.click(screen.getByRole('button', { name: 'Pause all provider reads' }));
 await waitFor(() => expect(commercialSignalsApi.sources).toHaveBeenCalledWith('p-one', { expectedRevision: 2, stripeEnabled: false, stripeFilter: null, windowDays: 30, publicationIds: [], automaticReads: false, acknowledgeSharing: true }, expect.any(AbortSignal)));
 expect((screen.getByLabelText('Previous complete UTC days') as HTMLInputElement).value).toBe('0'); expect(draftProbe.dirty).toBe(true);
});
