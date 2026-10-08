import { productApi } from './product';
import type { ProjectRun, RunOperation } from './projectRuns';

export interface SwarmReview {
  taskId: string; operationId: string; sourceCommit: string; targetCommit: string;
  reviewDigest: string; paths: string[]; patch: string; canIntegrate: boolean; reason: string;
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
