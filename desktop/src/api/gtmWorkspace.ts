import { productApi } from './product';
import type { ArtifactSource } from './distributeArtifacts';

export interface GTMSegment { id: string; name: string; problem: string; value: string; requirementIds: string[] }
export interface GTMCompetitor { id: string; name: string; alternative: string; difference: string }
export interface GTMChannel { id: string; name: string; hypothesis: string; segmentIds: string[] }
export interface GTMMilestone { id: string; title: string; outcome: string; targetDate: string; dependsOn: string[] }
export interface GTMExperiment { id: string; title: string; hypothesis: string; method: string; successCriterion: string; segmentId: string; channelId: string; status: 'planned' | 'in-progress' | 'recorded' | 'retired'; result: string }
export interface GTMContent {
  commercialModel?: CommercialModelInput;
  positioning: string; offer: string; pricingAssumptions: string[];
  segments: GTMSegment[]; competitors: GTMCompetitor[]; channels: GTMChannel[];
  milestones: GTMMilestone[]; experiments: GTMExperiment[];
  artifactLinks: { id: string; revision: number; digest: string }[];
}
export interface GTMWorkspace { projectId: string; revision: number; source: ArtifactSource; sourceDigest: string; content: GTMContent; digest: string; actor: { id: string; displayName: string }; at: string }
export interface CommercialModelInput { currency: string; minorUnitDigits: number; interval: 'month' | 'year' | 'one_time'; pricePerUnitMinor: number; variableCostPerUnitMinor: number; fixedCostMinor: number; units: number; assumptions: string[] }
export interface CommercialModelProjection { version: number; input: CommercialModelInput; digest: string; revenueMinor: number; variableCostMinor: number; totalCostMinor: number; operatingProfitMinor: number; contributionPerUnitMinor: number; contributionMarginBasisPoints: number | null; breakEvenUnits: number | null; breakEvenRevenueMinor: number | null; decisions: string[] }
export interface GTMView { projectId: string; workspace: GTMWorkspace | null; source: ArtifactSource; sourceDigest: string; freshness: 'none' | 'current' | 'outdated' | 'readback' | 'unavailable'; history: { revision: number; digest: string; at: string }[]; requirements: { id: string; title: string }[]; commercialModel?: CommercialModelProjection }
export const emptyGTM = (): GTMContent => ({ positioning: '', offer: '', pricingAssumptions: [], segments: [], competitors: [], channels: [], milestones: [], experiments: [], artifactLinks: [] });
const hash = (x: unknown): x is string => typeof x === 'string' && /^[a-f0-9]{64}$/.test(x);
const text = (x: unknown): x is string => typeof x === 'string' && x.length <= 4000 && !/[\x00-\x08\x0b\x0c\x0e-\x1f]/.test(x);
const recordId = (x: unknown): x is string => typeof x === 'string' && /^[a-zA-Z0-9][a-zA-Z0-9._:-]{0,127}$/.test(x);
const list = (x: unknown): x is unknown[] => Array.isArray(x) && x.length <= 50;
const ids = (x: unknown): x is string[] => list(x) && x.every(recordId) && new Set(x).size === x.length;
export function commercialInputValid(value: unknown): value is CommercialModelInput {
  if (!value || typeof value !== 'object') return false;
  const c = value as CommercialModelInput;
  return /^[A-Z]{3}$/.test(c.currency) && Number.isInteger(c.minorUnitDigits) && c.minorUnitDigits >= 0 && c.minorUnitDigits <= 6 &&
    ['month', 'year', 'one_time'].includes(c.interval) && [c.pricePerUnitMinor, c.variableCostPerUnitMinor, c.fixedCostMinor].every(n => Number.isSafeInteger(n) && n >= 0 && n <= 1e9) &&
    Number.isSafeInteger(c.units) && c.units >= 0 && c.units <= 1e6 && list(c.assumptions) && c.assumptions.length <= 30 && c.assumptions.every(x => text(x) && !!x.trim() && new TextEncoder().encode(x).length <= 1000);
}
// Structural recovery validation deliberately retains dangling links for repair.
// Only the server validates current requirement/artifact identities and authority.
export function gtmDraftValid(value: unknown): value is GTMContent {
  if (!value || typeof value !== 'object') return false;
  const c = value as GTMContent;
  if (c.commercialModel !== undefined && !commercialInputValid(c.commercialModel)) return false;
  if (!text(c.positioning) || !text(c.offer) || !list(c.pricingAssumptions) || !c.pricingAssumptions.every(text)) return false;
  if (![c.segments, c.competitors, c.channels, c.milestones, c.experiments, c.artifactLinks].every(list)) return false;
  const all = [...c.segments, ...c.competitors, ...c.channels, ...c.milestones, ...c.experiments];
  if (all.some(r => !r || !recordId(r.id)) || new Set(all.map(r => r.id)).size !== all.length) return false;
  return c.segments.every(r => text(r.name) && text(r.problem) && text(r.value) && ids(r.requirementIds)) &&
    c.competitors.every(r => text(r.name) && text(r.alternative) && text(r.difference)) &&
    c.channels.every(r => text(r.name) && text(r.hypothesis) && ids(r.segmentIds)) &&
    c.milestones.every(r => text(r.title) && text(r.outcome) && text(r.targetDate) && ids(r.dependsOn)) &&
    c.experiments.every(r => text(r.title) && text(r.hypothesis) && text(r.method) && text(r.successCriterion) && text(r.result) &&
      (r.segmentId === '' || recordId(r.segmentId)) && (r.channelId === '' || recordId(r.channelId)) && ['planned', 'in-progress', 'recorded', 'retired'].includes(r.status)) &&
    c.artifactLinks.every(r => r && recordId(r.id) && Number.isSafeInteger(r.revision) && r.revision > 0 && hash(r.digest)) &&
    new Set(c.artifactLinks.map(r => r.id)).size === c.artifactLinks.length && new TextEncoder().encode(JSON.stringify(c)).length <= 128 * 1024;
}
function checked(projectId: string, view: GTMView): GTMView {
  if (!view || view.projectId !== projectId || !(hash(view.sourceDigest) || view.freshness === 'unavailable' && view.sourceDigest === '' && !!view.workspace) || !view.source ||
      !['none', 'current', 'outdated', 'readback', 'unavailable'].includes(view.freshness) || !Array.isArray(view.history) || view.history.length > 50 ||
      !Array.isArray(view.requirements) || view.requirements.some(r => !r || !recordId(r.id) || typeof r.title !== 'string') || new Set(view.requirements.map(r => r.id)).size !== view.requirements.length ||
      view.history.some(r => !r || !Number.isSafeInteger(r.revision) || r.revision < 1 || !hash(r.digest) || typeof r.at !== 'string') || new Set(view.history.map(r => r.revision)).size !== view.history.length ||
      (view.workspace !== null && (!view.workspace || view.workspace.projectId !== projectId || !Number.isSafeInteger(view.workspace.revision) || view.workspace.revision < 1 || !hash(view.workspace.digest) || !hash(view.workspace.sourceDigest) || !gtmDraftValid(view.workspace.content))) ||
      (view.workspace === null && view.freshness !== 'none')) throw new Error('The GTM workspace could not be verified. Your edits were not saved.');
  if (view.workspace && (!view.history.some(r => r.revision === view.workspace!.revision && r.digest === view.workspace!.digest) ||
      view.freshness === 'current' && view.workspace.sourceDigest !== view.sourceDigest)) throw new Error('The saved GTM revision does not match its history or source. Refresh before continuing.');
  if (view.commercialModel) {
    const p = view.commercialModel;
    const authored = view.workspace?.content.commercialModel;
    if (!authored || p.version !== 1 || !hash(p.digest) || !commercialInputValid(p.input) ||
        (['currency', 'minorUnitDigits', 'interval', 'pricePerUnitMinor', 'variableCostPerUnitMinor', 'fixedCostMinor', 'units'] as const).some(key => p.input[key] !== authored[key]) ||
        JSON.stringify(p.input.assumptions) !== JSON.stringify(authored.assumptions) ||
        ![p.revenueMinor, p.variableCostMinor, p.totalCostMinor, p.operatingProfitMinor, p.contributionPerUnitMinor].every(Number.isSafeInteger) ||
        ![p.contributionMarginBasisPoints, p.breakEvenUnits, p.breakEvenRevenueMinor].every(n => n === null || Number.isSafeInteger(n)) ||
        !Array.isArray(p.decisions) || p.decisions.some(x => typeof x !== 'string')) throw new Error('The saved commercial calculation could not be verified.');
  }
  if (view.workspace?.content.commercialModel && !view.commercialModel) throw new Error('The saved commercial calculation is missing. Refresh before using its figures.');
  return view;
}
const root = (projectId: string) => `/v1/product/projects/${encodeURIComponent(projectId)}/distribute/gtm`;
export const gtmWorkspaceApi = {
  async get(projectId: string, signal?: AbortSignal): Promise<GTMView> { return checked(projectId, await productApi.request<GTMView>(root(projectId), undefined, 'GET', 30_000, signal)); },
  async save(projectId: string, baseline: GTMView, content: GTMContent, idempotencyKey: string): Promise<GTMView> {
    return checked(projectId, await productApi.request<GTMView>(root(projectId), { expectedRevision: baseline.workspace?.revision ?? 0, expectedDigest: baseline.workspace?.digest ?? '', expectedSourceDigest: baseline.sourceDigest, content, idempotencyKey }, 'POST'));
  },
};
