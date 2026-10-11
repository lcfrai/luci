import { productApi } from './product';
import type { CreateProjectRun } from './projectRuns';
import type { OrchestratorView } from './orchestrator';

export type CoordinatorActionKind = 'create-run' | 'prepare-tasks' | 'queue-tasks';
export interface CoordinatorContinuation {
  id: string; projectId: string; actor: { id: string; displayName: string }; idempotencyKey: string;
  originTurnId: string; contextDigest: string; policyRevision: number; revision: number;
  status: 'ready' | 'continuing' | 'queued' | 'checking' | 'checks-verified' | 'unknown' | 'refused' | 'blocked'; message: string;
  runId?: string; queueIds: string[];
  actions: { id: string; kind: CoordinatorActionKind; status: 'pending' | 'confirmed' | 'unknown' | 'refused'; referenceId?: string; message?: string; startedAt: string; finishedAt?: string }[];
  delivery?: { actionId: string; checkId: string; status: 'pending' | 'running' | 'verified' | 'unknown' | 'blocked'; commit: string; sourceDigest: string; integrationDigest: string; message: string; startedAt: string; updatedAt: string };
  createdAt: string; updatedAt: string; nextAction?: OrchestratorView['nextAction'];
}
export interface CoordinatorContinuationRequest {
  idempotencyKey: string; originTurnId: string; expectedContextDigest: string; expectedPolicyRevision: number;
  run: Omit<CreateProjectRun, 'idempotencyKey'>; maxParallel: number;
}
const base = (project: string) => `/v1/product/projects/${encodeURIComponent(project)}/orchestrator/continuations`;
const kinds: CoordinatorActionKind[] = ['create-run', 'prepare-tasks', 'queue-tasks'];
const text = (value: unknown): value is string => typeof value === 'string' && value.length > 0;
function verified(value: CoordinatorContinuation, project: string, actor: string, id?: string) {
  if (!value || value.projectId !== project || value.actor?.id !== actor || !text(value.id) || (id && value.id !== id) ||
      !text(value.idempotencyKey) || !text(value.originTurnId) || !text(value.contextDigest) ||
      !Number.isSafeInteger(value.revision) || value.revision < 1 || !Number.isSafeInteger(value.policyRevision) || value.policyRevision < 1 ||
      !['ready', 'continuing', 'queued', 'checking', 'checks-verified', 'unknown', 'refused', 'blocked'].includes(value.status) || typeof value.message !== 'string' ||
      !Array.isArray(value.queueIds) || value.queueIds.some(item => !text(item)) || new Set(value.queueIds).size !== value.queueIds.length ||
      !Array.isArray(value.actions) || value.actions.length > kinds.length || value.actions.some((action, index) =>
        action.kind !== kinds[index] || action.id !== `${value.id}-${action.kind}` || !['pending', 'confirmed', 'unknown', 'refused'].includes(action.status) || !text(action.startedAt)) ||
      value.status === 'checks-verified' && value.delivery?.status !== 'verified' ||
      value.delivery && (value.delivery.actionId !== `${value.id}-checks` || !text(value.delivery.checkId) || !['pending', 'running', 'verified', 'unknown', 'blocked'].includes(value.delivery.status) || !/^[0-9a-f]{40,64}$/.test(value.delivery.commit) || !/^[0-9a-f]{64}$/.test(value.delivery.sourceDigest) || !/^[0-9a-f]{64}$/.test(value.delivery.integrationDigest) || typeof value.delivery.message !== 'string') ||
      !Number.isFinite(Date.parse(value.createdAt)) || !Number.isFinite(Date.parse(value.updatedAt)) ||
      value.nextAction && (!['discover', 'design', 'develop', 'deploy'].includes(value.nextAction.lens) || !text(value.nextAction.label) || typeof value.nextAction.reason !== 'string')) {
    throw new Error('The continuation response could not be verified for this person and project. Refresh its exact outcome before continuing.');
  }
  return value;
}
export const coordinatorContinuationsApi = {
  async list(project: string, actor: string, signal?: AbortSignal) {
    const result = await productApi.request<{ continuations: CoordinatorContinuation[] }>(base(project), undefined, 'GET', 30_000, signal);
    if (!Array.isArray(result?.continuations) || result.continuations.length > 40 || new Set(result.continuations.map(row => row.id)).size !== result.continuations.length || new Set(result.continuations.map(row => row.idempotencyKey)).size !== result.continuations.length) throw new Error('The continuation history could not be verified. Refresh before continuing.');
    return result.continuations.map(row => verified(row, project, actor));
  },
  async get(project: string, actor: string, id: string, signal?: AbortSignal) {
    return verified(await productApi.request<CoordinatorContinuation>(`${base(project)}/${encodeURIComponent(id)}`, undefined, 'GET', 30_000, signal), project, actor, id);
  },
  async create(project: string, actor: string, input: CoordinatorContinuationRequest) {
    const value = verified(await productApi.request<CoordinatorContinuation>(base(project), input), project, actor);
    if (value.idempotencyKey !== input.idempotencyKey || value.originTurnId !== input.originTurnId || value.contextDigest !== input.expectedContextDigest || value.policyRevision !== input.expectedPolicyRevision) throw new Error('The returned continuation did not match this exact request. Read its retained outcome before another start.');
    return value;
  },
  async advance(project: string, actor: string, id: string, expectedRevision: number) {
    const value = verified(await productApi.request<CoordinatorContinuation>(`${base(project)}/${encodeURIComponent(id)}/advance`, { expectedRevision }), project, actor, id);
    if (value.revision < expectedRevision) throw new Error('The continuation returned an older revision. Refresh before continuing.');
    return value;
  },
};
