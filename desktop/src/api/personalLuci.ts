import { productApi } from './product';
import type { PortfolioGoal } from './portfolioGoals';

export interface PersonalPreferences {
  computeProjectId: string; harness: string; model: string; connectionId: string; projectIds: string[]; goalIds: string[];
}
export interface PersonalContext {
  version: number; preferences: PersonalPreferences; digest: string; goals: PortfolioGoal[];
  projects: { project: { id: string; name: string; status: string }; attention: unknown[] }[];
  unknownProjects: number; omittedAttention: number;
}
export type PersonalHandoffPurpose = "work-request";
export interface PersonalHandoff {
  purpose?: PersonalHandoffPurpose;
  id: string; projectId: string; message: string; status: 'dispatching' | 'forwarded' | 'unknown'; projectTurnId?: string; projectJobId?: string;
}
export interface PersonalTurn {
  id: string; sequence: number; message: string; context: PersonalContext; status: 'reserved' | 'succeeded' | 'failed' | 'cancelled'; createdAt: string; withheld?: boolean;
  answer?: { summary: string; question: string; options: string[]; discussion: { projectId: string; message: string } | null };
  handoff?: PersonalHandoff;
}
export interface PersonalView { revision: number; conversationId: string; archived: boolean; archives: { id: string; lastMessageAt: string; turns: number }[]; preferences: PersonalPreferences; context: PersonalContext; turns: PersonalTurn[]; hasEarlier: boolean; beforeSequence: number }
export interface PersonalRequest { expectedRevision: number; expectedContextDigest: string; idempotencyKey: string; message: string }
export interface HandoffRequest { purpose?: PersonalHandoffPurpose; expectedRevision: number; projectId: string; message: string; projectRevision: number; projectContextDigest: string; idempotencyKey: string }
const path = '/v1/product/personal-luci';
const text = (v: unknown, max: number) => typeof v === 'string' && v.length <= max;
const ids = (v: unknown, max: number): v is string[] => Array.isArray(v) && v.length <= max && v.every(id => text(id, 200) && /^[a-zA-Z0-9_-]+$/.test(id)) && new Set(v).size === v.length;
export function validPersonalPreferences(value: PersonalPreferences, empty = false): boolean {
  return !!value && text(value.computeProjectId, 200) && text(value.harness, 30) && text(value.model, 512) && text(value.connectionId, 200) &&
    ids(value.projectIds, 15) && ids(value.goalIds, 16) && (empty && !value.computeProjectId || !!value.computeProjectId && ['codex', 'claude-code', 'opencode'].includes(value.harness) && !!value.model.trim() && !!value.connectionId);
}
function validContext(value: PersonalContext): boolean {
  return !!value && value.version === 1 && /^[a-f0-9]{64}$/.test(value.digest) && validPersonalPreferences(value.preferences) && Array.isArray(value.goals) && value.goals.length <= 16 && Array.isArray(value.projects) && value.projects.length <= 16 &&
    value.projects.every(row => row?.project && value.preferences.projectIds.includes(row.project.id) && text(row.project.name, 200) && Array.isArray(row.attention) && row.attention.length <= 8) &&
    Number.isSafeInteger(value.unknownProjects) && value.unknownProjects >= 0 && Number.isSafeInteger(value.omittedAttention) && value.omittedAttention >= 0;
}
export function checkedPersonalView(value: PersonalView): PersonalView {
	if (!value || !text(value.conversationId, 200) || !/^[a-zA-Z0-9_-]+$/.test(value.conversationId) || typeof value.archived !== 'boolean' || !Array.isArray(value.archives) || value.archives.length > 128 || new Set(value.archives.map(row => row.id)).size !== value.archives.length || value.archives.some(row => !row || !text(row.id, 200) || !/^[a-zA-Z0-9_-]+$/.test(row.id) || !Number.isFinite(Date.parse(row.lastMessageAt)) || !Number.isSafeInteger(row.turns) || row.turns < 1 || row.turns > 64)) throw new Error('Your saved conversations could not be verified.');
  if (!value || !Number.isSafeInteger(value.revision) || value.revision < 0 || !validPersonalPreferences(value.preferences, true) || value.preferences.computeProjectId && !validContext(value.context) ||
    !Array.isArray(value.turns) || value.turns.length > 64 || typeof value.hasEarlier !== 'boolean' || !Number.isSafeInteger(value.beforeSequence) || value.beforeSequence < 0) throw new Error('Your private conversation could not be verified.');
  const seen = new Set<string>();
  for (const turn of value.turns) {
    if (!turn || !text(turn.id, 200) || !turn.id || seen.has(turn.id) || !Number.isSafeInteger(turn.sequence) || turn.sequence < 1 || !['reserved', 'succeeded', 'failed', 'cancelled'].includes(turn.status) || !Number.isFinite(Date.parse(turn.createdAt))) throw new Error('A retained private turn could not be verified.');
    seen.add(turn.id);
    if (turn.withheld) { if (turn.message || turn.answer || turn.handoff || turn.context?.preferences?.computeProjectId) throw new Error('An unavailable project turn was not withheld.'); continue; }
    if (!text(turn.message, 16000) || !turn.message || !validContext(turn.context)) throw new Error('A private turn lost its included context.');
    const answer = turn.answer;
    if (answer && (turn.status !== 'succeeded' || !text(answer.summary, 12000) || !answer.summary || !text(answer.question, 2000) || !Array.isArray(answer.options) || answer.options.length > 6 || answer.options.some(option => !text(option, 500)) ||
      answer.discussion && (!turn.context.preferences.projectIds.includes(answer.discussion.projectId) || !text(answer.discussion.message, 16000)))) throw new Error('The private response could not be verified.');
    if (turn.handoff && (turn.handoff.purpose !== undefined && turn.handoff.purpose !== "work-request" || !turn.context.preferences.projectIds.includes(turn.handoff.projectId) || !['dispatching', 'forwarded', 'unknown'].includes(turn.handoff.status) || !text(turn.handoff.message, 16000))) throw new Error('The project message receipt could not be verified.');
  }
  return value;
}
const turnPath = (id: string) => `${path}/turns/${encodeURIComponent(id)}`;
export const personalLuciApi = {
  async get(signal?: AbortSignal, before?: number, archiveId?: string) { const query = new URLSearchParams(); if (before) query.set('before', String(before)); if (archiveId) query.set('archiveId', archiveId); return checkedPersonalView(await productApi.request<PersonalView>(`${path}${query.toString() ? `?${query}` : ''}`, undefined, 'GET', 30_000, signal)); },
  async continue(expectedRevision: number, conversationId: string) { return checkedPersonalView(await productApi.request<PersonalView>(`${path}/continue`, { expectedRevision, conversationId })); },
  async preferences(expectedRevision: number, preferences: PersonalPreferences) { return checkedPersonalView(await productApi.request<PersonalView>(`${path}/preferences`, { expectedRevision, preferences }, 'PUT')); },
  send: (request: PersonalRequest) => productApi.request<{ turnId: string; status: string }>(`${path}/turns`, request),
  resolve: (request: PersonalRequest) => productApi.request<{ status: 'accepted' | 'not_sent'; turnId?: string }>(`${path}/resolve`, request),
  status: (id: string, signal?: AbortSignal) => productApi.request<{ turnId: string; status: string; message: string }>(turnPath(id), undefined, 'GET', 30_000, signal),
  check: (id: string) => productApi.request(`${turnPath(id)}/resolve`, {}),
  cancel: (id: string) => productApi.request(`${turnPath(id)}/cancel`, {}),
  handoff: (id: string, request: HandoffRequest) => productApi.request<{ handoff: PersonalHandoff }>(`${turnPath(id)}/handoff`, request),
  resolveHandoff: (id: string) => productApi.request<{ handoff: PersonalHandoff }>(`${turnPath(id)}/handoff/resolve`, {}),
  export: (archiveId?: string) => productApi.request<PersonalView>(`${path}/export${archiveId ? `?archiveId=${encodeURIComponent(archiveId)}` : ''}`, undefined, 'GET', 30_000),
};
