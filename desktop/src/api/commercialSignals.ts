import { productApi } from './product';

export interface StripeFilter { key: string; value: string }
export interface StripeConnection { id: string; name: string; accountId: string; live: boolean; revision: number }
export interface ReleaseSource { publicationId: string; repository: string; repositoryId: number; releaseId: number; authorId: number; tag: string; commit: string; artifactId: string; artifactDigest: string; label: string }
export interface CommercialConfiguration { revision: number; stripe: StripeConnection | null; stripeEnabled: boolean; stripeFilter: StripeFilter | null; windowDays: number; releases: ReleaseSource[]; automaticReads: boolean }
export interface CommercialCurrency { currency: string; capturedPayments: number; capturedMinor: number; refundedMinor: number; disputedPayments: number; paymentsWithoutCustomer: number }
export interface CommercialSnapshot {
 id: string; digest: string; sourceKey: string; provider: 'stripe' | 'github-release'; label: string; accountId?: string; live: boolean;
 repository?: string; releaseId?: number; publicationId?: string; artifactId?: string; artifactDigest?: string; tag?: string;
 periodStart?: string; periodEndExclusive?: string; startedAt: string; observedAt: string; records: number; matchedCharges: number;
 stripeFilter: StripeFilter | null; attribution: 'account-unallocated' | 'charge-metadata-filter' | 'original-release'; currencies: CommercialCurrency[];
 payingCustomerReferences: number; assetDownloads: number; reactions: number | null; semantics: string;
}
export interface CommercialStatus { sourceKey: string; latestId: string; lastAttemptAt: string; lastSuccessAt: string; state: 'collecting' | 'ready' | 'read-failed' | 'window-limit' | 'access-changed'; records: number }
export interface CommercialView { projectId: string; configuration: CommercialConfiguration; status: CommercialStatus[]; snapshots: CommercialSnapshot[]; prunedSnapshots: number; refreshMinutes: number; collectionRecordLimit: number }
export interface ReleaseChoice { publicationId: string; repository: string; tag: string; label: string }
export interface CommercialContext { projectId: string; records: { snapshot: CommercialSnapshot; currenciesOmitted: number; projectionDigest: string }[]; sourcesOmitted: number; unallocatedOmitted: number; pendingSources: number; unavailable: boolean; evidenceOnly: true }
export interface CommercialSourcesInput { expectedRevision: number; stripeEnabled: boolean; stripeFilter: StripeFilter | null; windowDays: number; publicationIds: string[]; automaticReads: boolean; acknowledgeSharing: boolean }
export interface CommercialStripeInput { expectedRevision: number; name: string; apiKey: string; remove: boolean; acknowledgePrivateStorage: boolean }
const object = (v: unknown): v is Record<string, unknown> => !!v && typeof v === 'object' && !Array.isArray(v);
const text = (v: unknown, max: number): v is string => typeof v === 'string' && new TextEncoder().encode(v).length <= max && !/[\u0000-\u0008\u000b\u000c\u000e-\u001f]/.test(v);
const integer = (v: unknown, max = Number.MAX_SAFE_INTEGER): v is number => typeof v === 'number' && Number.isSafeInteger(v) && v >= 0 && v <= max;
const positive = (v: unknown): v is number => integer(v) && v > 0;
const hash = (v: unknown): v is string => typeof v === 'string' && /^[a-f0-9]{64}$/.test(v);
const date = (v: unknown): v is string => text(v, 64) && Number.isFinite(Date.parse(v));
const repository = (v: unknown): v is string => typeof v === 'string' && /^[a-z0-9][a-z0-9-]{0,38}\/[a-z0-9_][a-z0-9_.-]{0,99}$/.test(v) && !v.endsWith('/.') && !v.endsWith('/..');
const opaque = (v: unknown, prefix: string) => text(v, 160) && v.startsWith(prefix) && /^[a-zA-Z0-9_-]+$/.test(v);
export function validStripeFilter(v: unknown): v is StripeFilter | null { return v === null || object(v) && typeof v.key === 'string' && /^[A-Za-z0-9_.-]{1,40}$/.test(v.key) && text(v.value, 200) && !!v.value && v.value.trim() === v.value && !/[\r\n\t]/.test(v.value); }
function connection(v: unknown): v is StripeConnection { return object(v) && opaque(v.id, 'stripe-') && text(v.name, 120) && !!v.name && text(v.accountId, 255) && /^acct_[A-Za-z0-9_-]+$/.test(v.accountId) && typeof v.live === 'boolean' && integer(v.revision, 1e9) && v.revision > 0; }
function release(v: unknown): v is ReleaseSource { return object(v) && opaque(v.publicationId, 'gh-release-') && repository(v.repository) && positive(v.repositoryId) && positive(v.releaseId) && positive(v.authorId) && text(v.tag, 128) && !!v.tag && typeof v.commit === 'string' && /^[a-f0-9]{40}$/.test(v.commit) && opaque(v.artifactId, '') && hash(v.artifactDigest) && text(v.label, 200) && !!v.label; }
function currency(v: unknown): v is CommercialCurrency { return object(v) && typeof v.currency === 'string' && /^[A-Z]{3}$/.test(v.currency) && integer(v.capturedPayments, 100000) && integer(v.capturedMinor) && integer(v.refundedMinor) && v.refundedMinor <= v.capturedMinor && integer(v.disputedPayments, v.capturedPayments) && integer(v.paymentsWithoutCustomer, v.capturedPayments); }
export function validCommercialSnapshot(v: unknown, projected = false): v is CommercialSnapshot {
 if (!object(v) || !hash(v.digest) || v.id !== 'commercial-' + v.digest || !text(v.sourceKey, 300) || !v.sourceKey || !text(v.label, 200) || !v.label || typeof v.live !== 'boolean' || !date(v.startedAt) || !date(v.observedAt) || !integer(v.records, 100000) || !integer(v.matchedCharges, v.records) || !validStripeFilter(v.stripeFilter) || !Array.isArray(v.currencies) || v.currencies.length > (projected ? 8 : 180) || !v.currencies.every(currency) || !integer(v.payingCustomerReferences, v.matchedCharges) || !integer(v.assetDownloads) || !(v.reactions === null || integer(v.reactions)) || !text(v.semantics, 2400) || !v.semantics) return false;
 const currencies = v.currencies as CommercialCurrency[];
 if (currencies.some((row, i) => i > 0 && row.currency <= currencies[i - 1].currency) || currencies.reduce((sum, row) => sum + row.capturedPayments, 0) > v.matchedCharges) return false;
 if (v.provider === 'stripe') {
  if (!text(v.accountId, 255) || !/^acct_[A-Za-z0-9_-]+$/.test(v.accountId) || !date(v.periodStart) || !date(v.periodEndExclusive) || !/^\d{4}-\d{2}-\d{2}$/.test(v.periodStart) || !/^\d{4}-\d{2}-\d{2}$/.test(v.periodEndExclusive)) return false;
  const interval = Date.parse(v.periodEndExclusive) - Date.parse(v.periodStart);
  return interval > 0 && interval <= 90 * 86400000 && v.sourceKey.startsWith('stripe:' + v.accountId + ':' + (v.live ? 'live:' : 'test:')) && (v.stripeFilter === null ? v.attribution === 'account-unallocated' : v.attribution === 'charge-metadata-filter') && v.assetDownloads === 0 && v.reactions === null && !v.repository && !v.releaseId;
 }
 return v.provider === 'github-release' && v.attribution === 'original-release' && repository(v.repository) && positive(v.releaseId) && v.sourceKey === `github:${v.repository}:${v.releaseId}` && opaque(v.publicationId, 'gh-release-') && opaque(v.artifactId, '') && hash(v.artifactDigest) && text(v.tag, 128) && !!v.tag && !v.accountId && !v.periodStart && !v.periodEndExclusive && v.stripeFilter === null && currencies.length === 0 && v.matchedCharges === 0 && v.payingCustomerReferences === 0;
}
export function validCommercialView(v: unknown, projectId: string): v is CommercialView {
 if (!object(v) || v.projectId !== projectId || !object(v.configuration) || !Array.isArray(v.status) || v.status.length > 1025 || !Array.isArray(v.snapshots) || v.snapshots.length > Math.max(100, v.status.length) || !v.snapshots.every(row => validCommercialSnapshot(row)) || !integer(v.prunedSnapshots) || v.refreshMinutes !== 15 || v.collectionRecordLimit !== 100000) return false;
 const c = v.configuration;
 if (!integer(c.revision, 1e9) || !(c.stripe === null || connection(c.stripe)) || typeof c.stripeEnabled !== 'boolean' || c.stripeEnabled && c.stripe === null || !validStripeFilter(c.stripeFilter) || !integer(c.windowDays, 90) || c.windowDays < 1 || !Array.isArray(c.releases) || c.releases.length > 1024 || !c.releases.every(release) || typeof c.automaticReads !== 'boolean' || 'apiKey' in c || 'stripeKey' in v || 'releaseCli' in v || 'collectionGenerations' in v) return false;
 const releases = c.releases as ReleaseSource[], snapshots = v.snapshots as CommercialSnapshot[];
 if (new Set(releases.map(row => row.repository + ':' + row.releaseId)).size !== releases.length || new Set(snapshots.map(row => row.id)).size !== snapshots.length) return false;
 return new Set(v.status.map(row => object(row) ? row.sourceKey : null)).size === v.status.length && v.status.every(row => object(row) && text(row.sourceKey, 300) && !!row.sourceKey && typeof row.state === 'string' && ['collecting', 'ready', 'read-failed', 'window-limit', 'access-changed'].includes(row.state) && integer(row.records, 100000) && (row.latestId === '' || typeof row.latestId === 'string' && snapshots.some(snapshot => snapshot.id === row.latestId && snapshot.sourceKey === row.sourceKey)) && (row.lastAttemptAt === '' || date(row.lastAttemptAt)) && (row.lastSuccessAt === '' || date(row.lastSuccessAt)));
}
export function validCommercialContext(v: unknown, projectId: string): v is CommercialContext | undefined {
 if (v === undefined) return true;
 if (!object(v) || v.projectId !== projectId || v.evidenceOnly !== true || !Array.isArray(v.records) || v.records.length > 8 || !integer(v.sourcesOmitted, 1025) || !integer(v.unallocatedOmitted, 1) || !integer(v.pendingSources, 1025) || typeof v.unavailable !== 'boolean') return false;
 if (v.unavailable && (v.records.length || v.sourcesOmitted || v.unallocatedOmitted || v.pendingSources)) return false;
 return new Set(v.records.map(row => object(row) && object(row.snapshot) ? row.snapshot.id : null)).size === v.records.length && v.records.every(row => object(row) && validCommercialSnapshot(row.snapshot, true) && row.snapshot.attribution !== 'account-unallocated' && integer(row.currenciesOmitted, 180 - row.snapshot.currencies.length) && hash(row.projectionDigest));
}
const route = (id: string) => `/v1/product/projects/${encodeURIComponent(id)}/distribute/commercial-signals`;
const invalid = () => new Error('Commercial results could not be confirmed for this project. Read saved results before another change.');
async function view(id: string, suffix: string, body: unknown, method: 'GET' | 'PUT' | 'POST', signal?: AbortSignal): Promise<CommercialView> {
 const value: unknown = await productApi.request(route(id) + suffix, body, method, 15000, signal);
 if (!validCommercialView(value, id)) throw invalid(); return value;
}
export const commercialSignalsApi = {
 get: (id: string, signal?: AbortSignal) => view(id, '', undefined, 'GET', signal),
 async releases(id: string, signal?: AbortSignal): Promise<ReleaseChoice[]> {
  const value: unknown = await productApi.request(route(id) + '/release-choices', undefined, 'GET', 15000, signal);
  if (!object(value) || value.projectId !== id || value.retainedRecentOnly !== true || !Array.isArray(value.releases) || value.releases.length > 64 || !value.releases.every(row => object(row) && opaque(row.publicationId, 'gh-release-') && repository(row.repository) && text(row.tag, 128) && !!row.tag && text(row.label, 200) && !!row.label) || new Set(value.releases.map(row => (row as ReleaseChoice).publicationId)).size !== value.releases.length) throw invalid();
  return value.releases as ReleaseChoice[];
 },
 async stripe(id: string, input: CommercialStripeInput, signal?: AbortSignal) {
  const result = await view(id, '/stripe', input, 'PUT', signal);
  if (result.configuration.revision !== input.expectedRevision + 1 || result.configuration.stripeEnabled || (input.remove ? result.configuration.stripe !== null : !result.configuration.stripe || result.configuration.stripe.name !== input.name.trim() || result.configuration.stripe.live !== input.apiKey.startsWith('rk_live_'))) throw invalid(); return result;
 },
 async sources(id: string, input: CommercialSourcesInput, signal?: AbortSignal) {
  const result = await view(id, '/sources', input, 'PUT', signal), c = result.configuration;
  if (c.revision !== input.expectedRevision + 1 || c.stripeEnabled !== input.stripeEnabled || JSON.stringify(c.stripeFilter) !== JSON.stringify(input.stripeFilter) || c.windowDays !== input.windowDays || c.automaticReads !== (input.automaticReads && (input.stripeEnabled || input.publicationIds.length > 0)) || JSON.stringify(c.releases.map(row => row.publicationId).sort()) !== JSON.stringify([...input.publicationIds].sort())) throw invalid(); return result;
 },
 refresh: (id: string, sourceKey = '', resetCollection = false, signal?: AbortSignal) => view(id, '/refresh', { sourceKey, resetCollection }, 'POST', signal),
};

// Stripe Charges units, not payout rules. ISK/UGX are encoded with two
// decimal places despite ISO zero-decimal presentation; HUF/TWD Charges use two.
// Source: docs.stripe.com/currencies and Stripe's three-decimal currency support.
const stripeZeroDecimal = new Set(['BIF','CLP','DJF','GNF','JPY','KMF','KRW','MGA','PYG','RWF','VND','VUV','XAF','XOF','XPF']);
const stripeThreeDecimal = new Set(['BHD','JOD','KWD','OMR','TND']);
export function stripeChargeAmount(minor: number, currency: string): string {
 if (!Number.isSafeInteger(minor) || minor < 0 || !/^[A-Z]{3}$/.test(currency)) throw new Error('The recorded Stripe amount is invalid.');
 const places = stripeZeroDecimal.has(currency) ? 0 : stripeThreeDecimal.has(currency) ? 3 : 2;
 const scale = BigInt(10) ** BigInt(places), amount = BigInt(minor);
 return `${currency} ${(amount / scale).toString()}${places ? '.' + (amount % scale).toString().padStart(places, '0') : ''}`;
}
