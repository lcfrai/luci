import { productApi } from './product';

export interface FeedbackSource { repository: string; contextDigest: string }
export interface GitHubIssue { number: number; title: string; body: string; bodyTruncated: boolean; state: 'open' | 'closed'; updatedAt: string }
export interface FeedbackObservation { id: string; digest: string; repository: string; issue: GitHubIssue; observedAt: string; importedAt: string; actor: { id: string; displayName: string }; sourceContextDigest: string }
export interface ProjectFeedbackView { projectId: string; revision: number; source: FeedbackSource | null; observations: FeedbackObservation[]; selectedIds: string[]; prunedObservations: number }
export interface FeedbackPreview { id: string; projectId: string; source: FeedbackSource; createdAt: string; expiresAt: string; fetched: number; issues: GitHubIssue[]; cliAuthScope: 'workspace-cli-not-personal-identity' }
export interface FeedbackImportRequest { previewId: string; expectedRevision: number; issueNumbers: number[]; acknowledgeSharing: boolean; idempotencyKey: string }
export interface OrchestratorFeedbackContext {
  projectId: string; evidenceOnly: true; omittedBodyBytes: number;
  items: { id: string; digest: string; repository: string; number: number; titleExcerpt: string; titleBytesOmitted: number; bodyExcerpt: string; bodyBytesOmitted: number; providerBodyTruncated: boolean; state: 'open' | 'closed'; updatedAt: string; observedAt: string; sourceContextDigest: string }[];
}
const object = (value: unknown): value is Record<string, unknown> => !!value && typeof value === 'object' && !Array.isArray(value);
const text = (value: unknown, max: number): value is string => typeof value === 'string' && new TextEncoder().encode(value).length <= max && !/[\u0000-\u0008\u000b\u000c\u000e-\u001f]/.test(value);
const count = (value: unknown): value is number => Number.isSafeInteger(value) && (value as number) >= 0 && (value as number) <= 1_000_000_000;
const hash = (value: unknown): value is string => typeof value === 'string' && /^[a-f0-9]{64}$/.test(value);
const repository = (value: unknown): value is string => typeof value === 'string' && /^[a-z0-9][a-z0-9-]{0,38}\/[a-z0-9_][a-z0-9_.-]{0,99}$/.test(value) && !value.endsWith('/.') && !value.endsWith('/..');
const date = (value: unknown): value is string => text(value, 64) && Number.isFinite(Date.parse(value));
const source = (value: unknown): value is FeedbackSource => object(value) && repository(value.repository) && hash(value.contextDigest);
export const sameFeedbackSource = (left: FeedbackSource, right: FeedbackSource) => left.repository === right.repository && left.contextDigest === right.contextDigest;
export const issueEqual = (left: GitHubIssue, right: GitHubIssue) => left.number === right.number && left.title === right.title && left.body === right.body && left.bodyTruncated === right.bodyTruncated && left.state === right.state && left.updatedAt === right.updatedAt;
function issue(value: unknown): value is GitHubIssue { return object(value) && count(value.number) && value.number > 0 && text(value.title, 2048) && !!value.title.trim() && text(value.body, 8192) && typeof value.bodyTruncated === 'boolean' && (value.state === 'open' || value.state === 'closed') && date(value.updatedAt); }
function observation(value: unknown): value is FeedbackObservation { return object(value) && hash(value.digest) && value.id === 'feedback-' + value.digest && repository(value.repository) && issue(value.issue) && date(value.observedAt) && date(value.importedAt) && object(value.actor) && text(value.actor.id, 128) && !!value.actor.id && text(value.actor.displayName, 512) && !!value.actor.displayName && hash(value.sourceContextDigest); }
export function validProjectFeedback(value: unknown, projectId: string): value is ProjectFeedbackView {
  return object(value) && value.projectId === projectId && count(value.revision) && (value.source === null || source(value.source)) && Array.isArray(value.observations) && value.observations.length <= 200 && value.observations.every(observation) && new Set(value.observations.map(row => row.id)).size === value.observations.length && Array.isArray(value.selectedIds) && value.selectedIds.length <= 10 && value.selectedIds.every(id => typeof id === 'string' && (value.observations as FeedbackObservation[]).some(row => row.id === id)) && new Set(value.selectedIds).size === value.selectedIds.length && count(value.prunedObservations);
}
export function validFeedbackPreview(value: unknown, projectId: string): value is FeedbackPreview {
  return object(value) && value.projectId === projectId && text(value.id, 100) && /^feedback-preview-[a-f0-9]{24}$/.test(value.id) && source(value.source) && date(value.createdAt) && date(value.expiresAt) && Date.parse(value.expiresAt) - Date.parse(value.createdAt) === 600_000 && count(value.fetched) && value.fetched <= 100 && Array.isArray(value.issues) && value.issues.length <= value.fetched && value.issues.every(issue) && new Set(value.issues.map(row => row.number)).size === value.issues.length && value.cliAuthScope === 'workspace-cli-not-personal-identity' && !('cli' in value);
}
export function validCapturedFeedbackContext(value: OrchestratorFeedbackContext | undefined, projectId: string): boolean {
  if (value === undefined) return true;
  if (!value || value.projectId !== projectId || value.evidenceOnly !== true || !count(value.omittedBodyBytes) || !Array.isArray(value.items) || value.items.length === 0 || value.items.length > 10) return false;
  return value.items.every(row => row && hash(row.digest) && row.id === 'feedback-' + row.digest && repository(row.repository) && count(row.number) && row.number > 0 && text(row.titleExcerpt, 128) && !!row.titleExcerpt && text(row.bodyExcerpt, 256) && count(row.titleBytesOmitted) && count(row.bodyBytesOmitted) && typeof row.providerBodyTruncated === 'boolean' && (row.state === 'open' || row.state === 'closed') && date(row.updatedAt) && date(row.observedAt) && hash(row.sourceContextDigest)) && new Set(value.items.map(row => row.id)).size === value.items.length && value.items.reduce((sum, row) => sum + row.bodyBytesOmitted, 0) === value.omittedBodyBytes;
}

const route = (id: string) => `/v1/product/projects/${encodeURIComponent(id)}/feedback`;
const invalid = () => new Error('This feedback response could not be confirmed for the selected project. Read saved feedback before retrying a change.');
export const projectFeedbackApi = {
  async get(id: string, signal?: AbortSignal): Promise<ProjectFeedbackView> {
    const value: unknown = await productApi.request(route(id), undefined, 'GET', 30_000, signal);
    if (!validProjectFeedback(value, id)) throw invalid(); return value;
  },
  async preview(id: string, expected: FeedbackSource, signal?: AbortSignal): Promise<FeedbackPreview> {
    const value: unknown = await productApi.request(route(id) + '/preview', { source: expected }, 'POST', 30_000, signal);
    if (!validFeedbackPreview(value, id) || !sameFeedbackSource(value.source, expected)) throw invalid(); return value;
  },
  async import(id: string, preview: FeedbackPreview, request: FeedbackImportRequest, signal?: AbortSignal): Promise<{ view: ProjectFeedbackView; importedIds: string[]; replayed: boolean }> {
    if (preview.projectId !== id || request.previewId !== preview.id || !request.acknowledgeSharing || request.issueNumbers.length === 0 || request.issueNumbers.length > 10 || new Set(request.issueNumbers).size !== request.issueNumbers.length || request.issueNumbers.some(number => !preview.issues.some(issue => issue.number === number))) throw invalid();
    const value: unknown = await productApi.request(route(id) + '/import', request, 'POST', 30_000, signal);
    if (!object(value) || !validProjectFeedback(value.view, id) || !Array.isArray(value.importedIds) || value.importedIds.length !== request.issueNumbers.length || new Set(value.importedIds).size !== value.importedIds.length || typeof value.replayed !== 'boolean') throw invalid();
    const view = value.view, ids = value.importedIds;
    if (!request.issueNumbers.every(number => { const selected = preview.issues.find(row => row.number === number); return !!selected && view.observations.some(row => ids.includes(row.id) && row.repository === preview.source.repository && issueEqual(row.issue, selected)); })) throw invalid();
    return { view, importedIds: ids as string[], replayed: value.replayed };
  },
  async select(id: string, expectedRevision: number, selected: string[], signal?: AbortSignal): Promise<ProjectFeedbackView> {
    const ids = [...selected].sort();
    const value: unknown = await productApi.request(route(id) + '/selection', { expectedRevision, ids }, 'POST', 30_000, signal);
    if (!validProjectFeedback(value, id) || JSON.stringify([...value.selectedIds].sort()) !== JSON.stringify(ids) || ![expectedRevision, expectedRevision + 1].includes(value.revision)) throw invalid(); return value;
  },
};
