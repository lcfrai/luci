import { act, cleanup, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { GTMWorkspace, parseMoneyMinor } from './GTMWorkspace';
import { emptyGTM, gtmWorkspaceApi, type GTMView } from '../../api/gtmWorkspace';
import { resetCockpit } from '../../store/store';

const environment = vi.hoisted(() => ({ scope: 'gtm-owner/workspace/p-one' }));
vi.mock('../../hooks/useProjectDraft', async importOriginal => ({ ...await importOriginal<typeof import('../../hooks/useProjectDraft')>(), useProjectDraftScope: () => environment.scope }));
vi.mock('../teams/TeamWorkspaceProvider', () => ({ useTeamWorkspace: () => null }));
const source = { briefRevision: 1, briefDigest: 'a'.repeat(64), specificationRevision: 2, specificationDigest: 'b'.repeat(64), designRevision: 1, designDigest: 'c'.repeat(64), planRevision: 3, planDigest: 'd'.repeat(64), checks: 'not-selected', release: 'not-selected' };
function fixture(): GTMView { return { projectId: 'p-one', source, sourceDigest: 'e'.repeat(64), workspace: null, freshness: 'none', history: [], requirements: [{ id: 'AC-save', title: 'Retain saved work' }] }; }
beforeEach(() => {
  vi.restoreAllMocks(); localStorage.clear(); resetCockpit(); environment.scope = 'gtm-owner/workspace/p-one';
  vi.spyOn(gtmWorkspaceApi, 'get').mockResolvedValue(fixture());
  vi.spyOn(gtmWorkspaceApi, 'save').mockImplementation(async (projectId, baseline, content) => {
    const revision = (baseline.workspace?.revision ?? 0) + 1;
    return { ...baseline, workspace: { projectId, revision, source, sourceDigest: baseline.sourceDigest, content, digest: 'f'.repeat(64), actor: { id: 'u-one', displayName: 'Owner' }, at: '2026-10-30T00:00:00Z' }, freshness: 'current', history: [...baseline.history, { revision, digest: 'f'.repeat(64), at: '2026-10-30T00:00:00Z' }] };
  });
});
afterEach(cleanup);

it('edits structured customer value and saves only on explicit action with source provenance', async () => {
  const user = userEvent.setup(); render(<GTMWorkspace projectId="p-one" canRead canEdit />);
  await user.type(await screen.findByLabelText('Positioning'), 'A focused workspace for local teams');
  await user.type(screen.getByLabelText('Pricing assumptions, one per line'), 'Price is untested{Enter}Demand is untested');
  expect((screen.getByLabelText('Pricing assumptions, one per line') as HTMLTextAreaElement).value).toBe('Price is untested\nDemand is untested');
  await user.click(screen.getByRole('button', { name: 'Add customer segment' }));
  await user.type(screen.getByLabelText('Segment name'), 'Local teams');
  await user.type(screen.getByLabelText('Customer problem'), 'Repeated handoffs');
  await user.type(screen.getByLabelText('Value for this segment'), 'Keep context together');
  expect(gtmWorkspaceApi.save).not.toHaveBeenCalled();
  await user.click(screen.getByRole('button', { name: 'Save GTM revision' }));
  await screen.findByText(/GTM revision saved/);
  expect(gtmWorkspaceApi.save).toHaveBeenCalledWith('p-one', expect.objectContaining({ sourceDigest: 'e'.repeat(64) }), expect.objectContaining({ positioning: 'A focused workspace for local teams', pricingAssumptions: ['Price is untested', 'Demand is untested'], segments: [expect.objectContaining({ name: 'Local teams', problem: 'Repeated handoffs', value: 'Keep context together' })] }), expect.stringMatching(/^gtm-/));
  expect(screen.queryByRole('button', { name: /Publish|Send campaign/ })).toBeNull();
});

it('keeps a read-only strategy visible without authoring or save authority', async () => {
  render(<GTMWorkspace projectId="p-one" canRead canEdit={false} />);
  const positioning = await screen.findByLabelText('Positioning') as HTMLTextAreaElement;
  // Native fieldset disabled state is inherited; the child's own disabled
  // property stays false even though user input is correctly refused.
  expect(positioning.matches(':disabled')).toBe(true);
  await userEvent.setup().type(positioning, 'Must not author');
  expect(positioning.value).toBe('');
  expect((screen.getByRole('button', { name: 'Add customer segment' }) as HTMLButtonElement).disabled).toBe(true);
  expect((screen.getByRole('button', { name: 'Save GTM revision' }) as HTMLButtonElement).disabled).toBe(true);
  expect(gtmWorkspaceApi.save).not.toHaveBeenCalled();
});

it('requires explicit review before replacing a newer baseline with a recovered draft', async () => {
  const retained = { ...emptyGTM(), positioning: 'Recovered older positioning' };
  localStorage.setItem('luci.project-draft.v1:' + JSON.stringify([environment.scope, 'go-to-market']), JSON.stringify({ version: 1, base: 'older-revision', value: retained, updatedAt: Date.now() }));
  const user = userEvent.setup(); render(<GTMWorkspace projectId="p-one" canRead canEdit />);
  await user.click(await screen.findByRole('button', { name: 'Restore GTM draft' }));
  expect((screen.getByLabelText('Positioning') as HTMLTextAreaElement).value).toBe('Recovered older positioning');
  expect((screen.getByRole('button', { name: 'Save GTM revision' }) as HTMLButtonElement).disabled).toBe(true);
  await user.click(screen.getByRole('checkbox', { name: /I reviewed the current saved revision/ }));
  expect((screen.getByRole('button', { name: 'Save GTM revision' }) as HTMLButtonElement).disabled).toBe(false);
  expect(gtmWorkspaceApi.save).not.toHaveBeenCalled();
});

it('withdraws an old actor response on scope change and never saves it', async () => {
  let resolve!: (value: GTMView) => void;
  vi.mocked(gtmWorkspaceApi.get).mockImplementationOnce(() => new Promise(done => { resolve = done; }));
  const view = render(<GTMWorkspace projectId="p-one" canRead canEdit />);
  environment.scope = 'another-owner/workspace/p-one'; view.rerender(<GTMWorkspace projectId="p-one" canRead canEdit />);
  await screen.findByLabelText('Positioning');
  const old = fixture(); old.workspace = { projectId: 'p-one', revision: 1, source, sourceDigest: old.sourceDigest, content: { ...emptyGTM(), positioning: 'Another actor private draft' }, digest: 'f'.repeat(64), actor: { id: 'u-old', displayName: 'Old' }, at: 'now' };
  await act(async () => { resolve(old); });
  await waitFor(() => expect((screen.getByLabelText('Positioning') as HTMLTextAreaElement).value).toBe(''));
  expect(gtmWorkspaceApi.save).not.toHaveBeenCalled();
});

it('retains edits after a rejected save and explicitly reviews the new baseline before retrying', async () => {
  const user = userEvent.setup();
  vi.mocked(gtmWorkspaceApi.save).mockRejectedValueOnce(new Error('The saved GTM workspace changed.'));
  render(<GTMWorkspace projectId="p-one" canRead canEdit />);
  await user.type(await screen.findByLabelText('Positioning'), 'Keep my positioning');
  await user.click(screen.getByRole('button', { name: 'Save GTM revision' }));
  await screen.findByText('The saved GTM workspace changed.');
  const newer = fixture(); newer.sourceDigest = '9'.repeat(64);
  newer.workspace = { projectId: 'p-one', revision: 2, source, sourceDigest: newer.sourceDigest, content: { ...emptyGTM(), positioning: 'Someone else changed the baseline' }, digest: '2'.repeat(64), actor: { id: 'u-other', displayName: 'Teammate' }, at: '2026-10-30T00:00:00Z' };
  newer.history = [{ revision: 2, digest: newer.workspace.digest, at: newer.workspace.at }];
  newer.freshness = 'current';
  vi.mocked(gtmWorkspaceApi.get).mockResolvedValueOnce(newer);
  await user.click(screen.getByRole('button', { name: 'Review current context without replacing edits' }));
  await screen.findByText(/Current revision loaded/);
  expect((screen.getByLabelText('Positioning') as HTMLTextAreaElement).value).toBe('Keep my positioning');
  expect((screen.getByRole('button', { name: 'Save GTM revision' }) as HTMLButtonElement).disabled).toBe(true);
  await user.click(screen.getByRole('checkbox', { name: /I reviewed the current saved revision/ }));
  await user.click(screen.getByRole('button', { name: 'Save GTM revision' }));
  await screen.findByText('GTM revision saved.');
  expect(gtmWorkspaceApi.save).toHaveBeenLastCalledWith('p-one', expect.objectContaining({ sourceDigest: '9'.repeat(64), workspace: expect.objectContaining({ revision: 2 }) }), expect.objectContaining({ positioning: 'Keep my positioning' }), expect.any(String));
});

it('parses decimal money exactly and displays the saved scenario as currency amounts', async () => {
  expect(parseMoneyMinor('29.50', 2)).toBe(2950);
  expect(parseMoneyMinor('29.501', 2)).toBeNull();
  expect(parseMoneyMinor('', 2)).toBeNull();
  expect(parseMoneyMinor('10000000.01', 2)).toBeNull();
  vi.mocked(gtmWorkspaceApi.save).mockImplementation(async (projectId, baseline, content) => ({ ...baseline, workspace: { projectId, revision: 1, source, sourceDigest: baseline.sourceDigest, content, digest: 'f'.repeat(64), actor: { id: 'u-one', displayName: 'Owner' }, at: 'now' }, freshness: 'current', history: [{ revision: 1, digest: 'f'.repeat(64), at: 'now' }], commercialModel: { version: 1, input: content.commercialModel!, digest: '1'.repeat(64), revenueMinor: 8850, variableCostMinor: 0, totalCostMinor: 0, operatingProfitMinor: 8850, contributionPerUnitMinor: 2950, contributionMarginBasisPoints: 10000, breakEvenUnits: 0, breakEvenRevenueMinor: 0, decisions: [] } }));
  const user = userEvent.setup(); render(<GTMWorkspace projectId="p-one" canRead canEdit />);
  await screen.findByLabelText('Positioning');
  await user.click(screen.getByText('Commercial scenario', { selector: 'summary' }));
  await user.click(screen.getByRole('checkbox', { name: 'Include a commercial scenario' }));
  await user.clear(screen.getByLabelText('Scenario currency (3-letter code)'));
  await user.type(screen.getByLabelText('Scenario currency (3-letter code)'), 'aud');
  const price = screen.getByLabelText('Price per customer');
  await user.clear(price);
  // Clearing deliberately renders inline invalid-value guidance inside the
  // label. Keep the same real input while typing a valid replacement amount.
  await user.type(price, '29.50');
  await user.clear(screen.getByLabelText('Customers or units per period'));
  await user.type(screen.getByLabelText('Customers or units per period'), '3');
  await user.click(screen.getByRole('button', { name: 'Save GTM revision' }));
  await screen.findByText(/Revenue: AUD 88.50/);
  expect(gtmWorkspaceApi.save).toHaveBeenCalledWith('p-one', expect.anything(), expect.objectContaining({ commercialModel: expect.objectContaining({ currency: 'AUD', pricePerUnitMinor: 2950, units: 3 }) }), expect.any(String));
});
