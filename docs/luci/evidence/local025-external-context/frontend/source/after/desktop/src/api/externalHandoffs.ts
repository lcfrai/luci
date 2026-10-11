import { productApi } from './product';
import type { ImplementationContext, ImplementationContextRequest } from '../store/types';

export interface ExternalSource { commit: string; sourceDigest: string; dirty: boolean }
export interface ExternalChangedFile { path: string; status: string; beforeSha256?: string; afterSha256?: string }
export interface ExternalReport { projectId: string; handoffId: string; handoffDigest: string; summary: string; checks: { command: string; outcome: string; output: string }[]; notes: string }
export interface ExternalReturn {
  id: string; handoffId: string; handoffDigest: string; createdAt: string; idempotencyKey: string;
  source: ExternalSource; changedFiles: ExternalChangedFile[]; report: ExternalReport; evidenceStatus: 'unverified';
}
export interface ExternalHandoffSummary {
  id: string; projectId: string; createdAt: string; digest: string;
  /** Summary responses have a count; full handoffs carry their returns array. */
  implementation: ImplementationContext; returnCount?: number;
}
export interface ExternalHandoff extends ExternalHandoffSummary {
  files: { path: string; sha256: string; bytes: number }[]; instructions: string; returns: ExternalReturn[];
}
export interface ExternalReturnReview {
  handoffId: string; handoffDigest: string; contextCurrent: boolean; contextMessage: string;
  source: ExternalSource; changedFiles: ExternalChangedFile[]; reviewDigest: string;
}
const path = (project: string, id?: string) => `/v1/product/projects/${encodeURIComponent(project)}/external-handoffs${id ? `/${encodeURIComponent(id)}` : ''}`;
const digest = (value: unknown) => typeof value === 'string' && /^[a-f0-9]{64}$/.test(value);
function owned<T extends ExternalHandoffSummary>(project: string, value: T, id?: string): T {
  if (!value || value.projectId !== project || value.implementation?.projectId !== project || !value.id || id !== undefined && value.id !== id || !digest(value.digest)) {
    throw new Error('This handoff could not be matched to the selected project. Refresh before continuing.');
  }
  return value;
}
function returned(value: ExternalReturn, handoff: ExternalHandoffSummary) {
  if (!value || !value.id || typeof value.idempotencyKey !== 'string' || !value.idempotencyKey || value.handoffId !== handoff.id || value.handoffDigest !== handoff.digest || value.evidenceStatus !== 'unverified') {
    throw new Error('This report could not be matched to the selected handoff. Refresh before continuing.');
  }
  return value;
}
function full(project: string, value: ExternalHandoff, id?: string) {
  owned(project, value, id);
  if (!Array.isArray(value.files) || !Array.isArray(value.returns) || typeof value.instructions !== 'string') throw new Error('The handoff response is incomplete. Refresh before continuing.');
  value.returns.forEach(report => returned(report, value));
  return value;
}
export const externalHandoffsApi = {
  async list(project: string) {
    const value = await productApi.request<{ handoffs: ExternalHandoffSummary[] }>(path(project));
    if (!Array.isArray(value.handoffs)) throw new Error('The handoff list could not be read. Refresh before continuing.');
    return value.handoffs.map(row => owned(project, row));
  },
  async create(project: string, selection: ImplementationContextRequest, idempotencyKey: string) {
    return full(project, await productApi.request<ExternalHandoff>(path(project), { selection, idempotencyKey }));
  },
  async get(project: string, id: string) { return full(project, await productApi.request<ExternalHandoff>(path(project, id)), id); },
  bundle(project: string, id: string) { return productApi.requestBlob(`${path(project, id)}/bundle`); },
  async review(project: string, handoff: ExternalHandoffSummary) {
    const value = await productApi.request<ExternalReturnReview>(`${path(project, handoff.id)}/review`);
    if (!value || value.handoffId !== handoff.id || value.handoffDigest !== handoff.digest || !digest(value.reviewDigest) || typeof value.contextCurrent !== 'boolean' || !Array.isArray(value.changedFiles)) {
      throw new Error('This source review could not be matched to the selected handoff. Refresh before continuing.');
    }
    return value;
  },
  async record(project: string, handoff: ExternalHandoffSummary, expectedReviewDigest: string, report: ExternalReport, idempotencyKey: string) {
    const value = returned(await productApi.request<ExternalReturn>(`${path(project, handoff.id)}/returns`, { expectedReviewDigest, report, idempotencyKey }), handoff);
    if (value.idempotencyKey !== idempotencyKey) throw new Error('This report does not match the submitted request. Refresh saved handoffs before continuing.');
    return value;
  },
};

/** External output is a bounded report, never executable instructions or verification. */
export function parseExternalReport(text: string, handoff: ExternalHandoffSummary): ExternalReport {
  if (new TextEncoder().encode(text).byteLength > 128 * 1024) throw new Error('Choose a report smaller than 128 KiB.');
  let value: unknown;
  try { value = JSON.parse(text); } catch { throw new Error('The report must be valid JSON with summary, checks and notes.'); }
  const row = value as Partial<ExternalReport> | null;
  if (!row || row.projectId !== handoff.projectId || row.handoffId !== handoff.id || row.handoffDigest !== handoff.digest) {
    throw new Error('This report does not match the selected project and captured handoff. Use the identities from its manifest.');
  }
  const bounded = (value: unknown, bytes: number) => typeof value === 'string' && !value.includes('\0') && new TextEncoder().encode(value).byteLength <= bytes;
  if (!row || typeof row !== 'object' || typeof row.summary !== 'string' || !row.summary.trim() || !bounded(row.summary, 16 * 1024) ||
      !bounded(row.notes, 32 * 1024) || !Array.isArray(row.checks) || row.checks.length > 50 || row.checks.some(check =>
        !check || typeof check.command !== 'string' || !check.command.trim() || !bounded(check.command, 4 * 1024) || typeof check.outcome !== 'string' || !check.outcome.trim() || !bounded(check.outcome, 4 * 1024) || !bounded(check.output, 32 * 1024))) {
    throw new Error('Use a nonempty summary, notes text and up to 50 checks with command, outcome and output text.');
  }
  return { projectId: row.projectId, handoffId: row.handoffId, handoffDigest: row.handoffDigest, summary: row.summary, notes: row.notes as string, checks: row.checks.map(({ command, outcome, output }) => ({ command, outcome, output })) };
}
