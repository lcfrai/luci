import { afterEach, expect, it, vi } from 'vitest';
import { productApi } from './product';
import { commercialInputValid, emptyGTM, gtmDraftValid, gtmWorkspaceApi, type GTMView } from './gtmWorkspace';
const source = { briefRevision: 1, briefDigest: 'a'.repeat(64), specificationRevision: 2, specificationDigest: 'b'.repeat(64), designRevision: 1, designDigest: 'c'.repeat(64), planRevision: 3, planDigest: 'd'.repeat(64), checks: 'not-selected', release: 'not-selected' };
const view = (): GTMView => ({ projectId: 'p-one', source, sourceDigest: 'e'.repeat(64), workspace: null, freshness: 'none', history: [], requirements: [] });
afterEach(() => vi.restoreAllMocks());

it('recovers structured drafts with dangling links for explicit repair but refuses malformed records', () => {
  const content = emptyGTM(); content.channels.push({ id: 'channel-one', name: 'Pilot', hypothesis: 'Untested', segmentIds: ['removed-segment'] });
  expect(gtmDraftValid(content)).toBe(true);
  expect(gtmDraftValid({ ...content, channels: null })).toBe(false);
  expect(gtmDraftValid({ ...content, experiments: [{ id: 'experiment', status: 'published' }] })).toBe(false);
  expect(gtmDraftValid({ ...content, artifactLinks: [{ id: 'artifact-one', revision: 0, digest: 'invalid' }] })).toBe(false);
});

it('rejects fractional or missing commercial inputs without turning them into zero', () => {
  const model = { currency: 'USD', minorUnitDigits: 2, interval: 'month', pricePerUnitMinor: 1000, variableCostPerUnitMinor: 200, fixedCostMinor: 100, units: 5, assumptions: ['Demand remains a hypothesis'] };
  expect(commercialInputValid(model)).toBe(true);
  expect(gtmDraftValid({ ...emptyGTM(), commercialModel: { ...model, units: NaN } })).toBe(false);
  expect(commercialInputValid({ ...model, pricePerUnitMinor: 1.5 })).toBe(false);
});

it('uses the exact source and saved revision for explicit save and rejects a foreign response', async () => {
  const request = vi.spyOn(productApi, 'request').mockResolvedValue(view());
  const baseline = await gtmWorkspaceApi.get('p-one');
  await gtmWorkspaceApi.save('p-one', baseline, emptyGTM(), 'request-one');
  expect(request).toHaveBeenLastCalledWith('/v1/product/projects/p-one/distribute/gtm', { expectedRevision: 0, expectedDigest: '', expectedSourceDigest: baseline.sourceDigest, content: emptyGTM(), idempotencyKey: 'request-one' }, 'POST');
  request.mockResolvedValue({ ...view(), projectId: 'p-other' });
  await expect(gtmWorkspaceApi.get('p-one')).rejects.toThrow('could not be verified');
});

it('keeps retained strategy readable when current context is unavailable', async () => {
  const retained = view(); retained.workspace = { projectId: 'p-one', revision: 1, source, sourceDigest: retained.sourceDigest, content: emptyGTM(), digest: 'f'.repeat(64), actor: { id: 'u-one', displayName: 'Owner' }, at: '2026-10-30T00:00:00Z' };
  retained.history = [{ revision: 1, digest: retained.workspace.digest, at: retained.workspace.at }];
  retained.sourceDigest = ''; retained.freshness = 'unavailable';
  vi.spyOn(productApi, 'request').mockResolvedValue(retained);
  expect((await gtmWorkspaceApi.get('p-one')).workspace?.revision).toBe(1);
});

it('accepts unresolved commercial nulls but rejects calculations for different authored inputs', async () => {
  const input = { currency: 'USD', minorUnitDigits: 2, interval: 'month' as const, pricePerUnitMinor: 0, variableCostPerUnitMinor: 100, fixedCostMinor: 100, units: 0, assumptions: ['Demand has not been measured'] };
  const v = view();
  v.workspace = { projectId: 'p-one', revision: 1, source, sourceDigest: v.sourceDigest, content: { ...emptyGTM(), commercialModel: input }, digest: 'f'.repeat(64), actor: { id: 'u-one', displayName: 'Owner' }, at: 'now' };
  v.history = [{ revision: 1, digest: v.workspace.digest, at: 'now' }];
  v.freshness = 'current';
  v.commercialModel = { version: 1, input, digest: '1'.repeat(64), revenueMinor: 0, variableCostMinor: 0, totalCostMinor: 100, operatingProfitMinor: -100, contributionPerUnitMinor: -100, contributionMarginBasisPoints: null, breakEvenUnits: null, breakEvenRevenueMinor: null, decisions: ['Set a price'] };
  const request = vi.spyOn(productApi, 'request').mockResolvedValue(v);
  expect((await gtmWorkspaceApi.get('p-one')).commercialModel?.breakEvenUnits).toBeNull();
  request.mockResolvedValue({ ...v, commercialModel: { ...v.commercialModel, input: { ...input, units: 4 } } });
  await expect(gtmWorkspaceApi.get('p-one')).rejects.toThrow('calculation could not be verified');
});
