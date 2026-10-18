import { productApi } from './product';
import type { ProjectRun, RunOperation } from './projectRuns';
import { MAX_SWARM_PARALLEL_TASKS } from './swarmLimits';

export const managedCredentialGuidance = 'These independent Luci jobs share managed ChatGPT credentials and run one at a time to protect credential refresh. For bounded parallel jobs, explicitly select and review a supported API model connection. Current access, provider and resource limits still apply.';
export type SwarmParallelPolicy = { mode: 'managed-credential-serial' | 'bounded-api-route' | 'unknown'; maximumSlots?: number; message: string };
/** Saved route support only. A receipt does not prove current readiness or authority. */
export function swarmParallelPolicy(route?: ProjectRun['input']['route']): SwarmParallelPolicy {
  const c = route?.connection;
  if (c?.kind === 'codex-subscription' || route?.harness === 'codex' && c?.authMode === 'chatgpt') return {mode: 'managed-credential-serial', maximumSlots: 1, message: managedCredentialGuidance};
  const protocol = route?.harness === 'codex' ? 'responses' : route?.harness === 'claude-code' ? 'anthropic-messages' : route?.harness === 'opencode' ? 'chat-completions' : undefined;
  if (c && ['litellm', 'compatible-api', 'direct-api'].includes(c.kind) && typeof c.id === 'string' && c.id.length > 0 && Number.isSafeInteger(c.revision) && c.revision > 0 && c.authMode === 'api-key' && protocol && c.protocol === protocol && typeof c.endpoint === 'string' && c.endpoint.length > 0 && typeof c.model === 'string' && c.model.length > 0 && c.model === route?.model) {
    return {mode: 'bounded-api-route', maximumSlots: MAX_SWARM_PARALLEL_TASKS, message: 'This saved API route supports bounded parallel jobs in Luci. Current connection, access, provider and resource checks still determine whether any task can start.'};
  }
  return {mode: 'unknown', message: 'Parallel support is not established for this saved route. Select and review a supported API model connection before requesting parallel jobs. Current readiness has not been established.'};
}
export function swarmParallelLimitIssue(route: ProjectRun['input']['route'] | undefined, limit: number): string {
  const policy = swarmParallelPolicy(route);
  if (!Number.isInteger(limit) || limit < 1 || limit > MAX_SWARM_PARALLEL_TASKS) return 'Choose a concurrency limit within the supported task count.';
  if (limit > (policy.maximumSlots ?? 1)) return policy.message;
  return '';
}

export interface SwarmReview {
  taskId: string; operationId: string; sourceCommit: string; targetCommit: string;
  reviewDigest: string; paths: string[]; patch: string; canIntegrate: boolean; reason: string;
	independentReviewState?: 'legacy-unassessed' | 'pending' | 'passed';
}
/** Both actions identify the saved attempt; destructive consent is never inferred. */
export type SwarmAttemptRecovery = {operationId: string; childId: string} & (
  | {recoverMissingChild: true; stopUncertainLaunch?: never}
  | {stopUncertainLaunch: true; recoverMissingChild?: never}
);
export interface SwarmRecoveryAvailability {
  expectedRevision: number; operationId: string; childId: string;
  availability: {supported: boolean; reasonCode: string; message: string};
}
const path = (project: string, run: string, suffix = '') => `/v1/product/projects/${encodeURIComponent(project)}/runs/${encodeURIComponent(run)}/swarm${suffix}`;
function owned(project: string, run: string, value: ProjectRun) {
  if (!value || value.id !== run || value.projectId !== project || value.input?.implementation?.projectId !== project || !value.swarm) {
    throw new Error('The agent team response does not belong to this build. Refresh history before continuing.');
  }
  return value;
}
async function change(project: string, run: string, suffix: string, body: unknown) {
  const result = await productApi.request<{run: ProjectRun}>(path(project, run, suffix), body);
  return owned(project, run, result.run);
}
export const projectSwarmApi = {
  async recovery(project: string, run: ProjectRun, operation: RunOperation, signal?: AbortSignal): Promise<SwarmRecoveryAvailability> {
    const value = await productApi.request<SwarmRecoveryAvailability>(path(project, run.id, `/operations/${encodeURIComponent(operation.id)}/recovery`), undefined, 'GET', 30_000, signal);
    if (!value || value.operationId !== operation.id || value.childId !== operation.childId ||
        !Number.isInteger(value.expectedRevision) || value.expectedRevision !== run.revision ||
        typeof value.availability?.supported !== 'boolean' || typeof value.availability?.reasonCode !== 'string' || typeof value.availability?.message !== 'string') {
      throw new Error('Recovery availability does not match this recorded attempt. Refresh team status and review it again.');
    }
    return value;
  },
  prepare: (project: string, run: string, expectedRevision: number, maxParallel: number) => change(project, run, '', {expectedRevision, maxParallel}),
  control: (project: string, run: string, action: 'pause' | 'resume' | 'cancel' | 'reconcile', expectedRevision: number) => change(project, run, `/${action}`, {expectedRevision}),
  async reserve(project: string, run: string, task: string, expectedRevision: number, idempotencyKey: string) {
    const result = await productApi.request<{run: ProjectRun; operation: RunOperation}>(path(project, run, `/tasks/${encodeURIComponent(task)}/reserve`), {expectedRevision, idempotencyKey});
    owned(project, run, result.run);
    const operation = result.run.operations.find(item => item.id === result.operation?.id);
    const slot = result.run.swarm!.tasks.find(item => item.id === task);
    if (!operation || slot?.operationId !== operation.id || operation.taskId !== task || operation.kind !== 'session' || !operation.worktreeId || operation.worktreeId !== result.operation.worktreeId || operation.childId !== result.operation.childId) {
      throw new Error('The isolated task reservation could not be verified. Check team status before continuing.');
    }
    return {run: result.run, operation};
  },
  async review(project: string, run: ProjectRun, task: string) {
    const result = await productApi.request<{review: SwarmReview}>(path(project, run.id, `/tasks/${encodeURIComponent(task)}/review`));
    const slot = run.swarm?.tasks.find(item => item.id === task), review = result.review;
    if (!slot || !review || review.taskId !== task || !slot.operationId || review.operationId !== slot.operationId || !Array.isArray(review.paths) || review.paths.some(item => typeof item !== 'string') || typeof review.patch !== 'string' || typeof review.reviewDigest !== 'string' || typeof review.canIntegrate !== 'boolean' || typeof review.sourceCommit !== 'string' || typeof review.targetCommit !== 'string' || typeof review.reason !== 'string') {
      throw new Error('The change review does not match this task attempt. Refresh history and review again.');
    }
    return review;
  },
  integrate: (project: string, run: string, task: string, expectedRevision: number, reviewDigest: string) => change(project, run, `/tasks/${encodeURIComponent(task)}/integrate`, {expectedRevision, reviewDigest}),
  abandon: (project: string, run: string, task: string, expectedRevision: number, recovery?: SwarmAttemptRecovery) => change(project, run, `/tasks/${encodeURIComponent(task)}/abandon`, {expectedRevision, ...recovery}),
};
