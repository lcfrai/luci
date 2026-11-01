import { afterEach, expect, it, vi } from 'vitest';
import { productApi } from './product';
import { commercialSignalsApi, stripeChargeAmount, validCommercialContext, validCommercialView, type CommercialSnapshot, type CommercialView } from './commercialSignals';
const digest = 'a'.repeat(64);
function snapshot(): CommercialSnapshot { return { id: 'commercial-' + digest, digest, sourceKey: 'stripe:acct_one:test:account', provider: 'stripe', label: 'Account receipts', accountId: 'acct_one', live: false, periodStart: '2026-10-01', periodEndExclusive: '2026-11-01', startedAt: '2026-11-01T00:01:00Z', observedAt: '2026-11-01T00:02:00Z', records: 2, matchedCharges: 2, stripeFilter: null, attribution: 'account-unallocated', currencies: [{ currency: 'USD', capturedPayments: 2, capturedMinor: 1999, refundedMinor: 100, disputedPayments: 0, paymentsWithoutCustomer: 1 }], payingCustomerReferences: 1, assetDownloads: 0, reactions: null, semantics: 'Native minor units; account-wide unallocated, not accounting revenue.' }; }
function saved(): CommercialView { const row = snapshot(); return { projectId: 'p-one', configuration: { revision: 2, stripe: { id: 'stripe-one', name: row.label, accountId: 'acct_one', live: false, revision: 1 }, stripeEnabled: true, stripeFilter: null, windowDays: 31, releases: [], automaticReads: false }, snapshots: [row], status: [{ sourceKey: row.sourceKey, latestId: row.id, lastAttemptAt: row.startedAt, lastSuccessAt: row.observedAt, state: 'ready', records: 2 }], prunedSnapshots: 0, refreshMinutes: 15, collectionRecordLimit: 100000 }; }
afterEach(() => vi.restoreAllMocks());
it('rejects cross-project, private credential leakage, false zero and enum-array responses', () => {
 expect(validCommercialView(saved(), 'p-one')).toBe(true); expect(validCommercialView(saved(), 'other')).toBe(false);
 expect(validCommercialView({ ...saved(), stripeKey: 'rk_test_private' }, 'p-one')).toBe(false);
 const wrong = saved(); wrong.snapshots[0].currencies[0].refundedMinor = 2000; expect(validCommercialView(wrong, 'p-one')).toBe(false);
 expect(validCommercialView({ ...saved(), status: [{ ...saved().status[0], state: ['ready'] }] }, 'p-one')).toBe(false);
 expect(validCommercialView({ ...saved(), snapshots: [{ ...snapshot(), reactions: Number.MAX_SAFE_INTEGER + 1 }] }, 'p-one')).toBe(false);
});
it('keeps account-wide figures out of captured project findings and accepts legacy omitted context', () => {
 expect(validCommercialContext(undefined, 'p-one')).toBe(true);
 expect(validCommercialContext({ projectId: 'p-one', evidenceOnly: true, unavailable: false, pendingSources: 0, sourcesOmitted: 0, unallocatedOmitted: 1, records: [] }, 'p-one')).toBe(true);
 expect(validCommercialContext({ projectId: 'p-one', evidenceOnly: true, unavailable: false, pendingSources: 0, sourcesOmitted: 0, unallocatedOmitted: 0, records: [{ snapshot: snapshot(), currenciesOmitted: 0, projectionDigest: digest }] }, 'p-one')).toBe(false);
});
it('passes exact consent/CAS and abort signal and rejects a different saved window', async () => {
 const signal = new AbortController().signal, request = vi.spyOn(productApi, 'request').mockResolvedValue(saved());
 await commercialSignalsApi.get('p-one', signal); expect(request).toHaveBeenCalledWith('/v1/product/projects/p-one/distribute/commercial-signals', undefined, 'GET', 15000, signal);
 const input = { expectedRevision: 1, stripeEnabled: true, stripeFilter: null, windowDays: 7, publicationIds: [], automaticReads: false, acknowledgeSharing: true };
 await expect(commercialSignalsApi.sources('p-one', input, signal)).rejects.toThrow(/could not be confirmed/);
 expect(request).toHaveBeenLastCalledWith('/v1/product/projects/p-one/distribute/commercial-signals/sources', input, 'PUT', 15000, signal);
});

it('formats Stripe charge currencies exactly without floating point or payout-rule conversion', () => {
 expect(stripeChargeAmount(12345, 'USD')).toBe('USD 123.45');
 expect(stripeChargeAmount(12345, 'JPY')).toBe('JPY 12345');
 expect(stripeChargeAmount(500, 'UGX')).toBe('UGX 5.00');
 expect(stripeChargeAmount(5120, 'KWD')).toBe('KWD 5.120');
 expect(stripeChargeAmount(Number.MAX_SAFE_INTEGER, 'USD')).toBe('USD 90071992547409.91');
 expect(stripeChargeAmount(500, 'ISK')).toBe('ISK 5.00');
 expect(stripeChargeAmount(123, 'HUF')).toBe('HUF 1.23');
 expect(stripeChargeAmount(123, 'TWD')).toBe('TWD 1.23');
});
