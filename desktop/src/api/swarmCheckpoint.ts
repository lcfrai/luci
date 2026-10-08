import { productApi } from './product';
import type { ProjectRun, RunOperation } from './projectRuns';

export interface SwarmCheckpointFile {
  path: string; status: string; before: string; after: string; beforeMode: string; afterMode: string;
}
export interface SwarmCheckpointReview {
  projectId: string; runId: string;
  operationId: string; childId: string; taskId: string; expectedRevision: number;
  sourceCommit: string; targetCommit: string; reviewDigest: string; canContinue: boolean;
  reason: string; files: SwarmCheckpointFile[]; totalBytes: number;
}
export interface SwarmCheckpointProvenance {
  sourceOperationId: string; sourceChildId: string; sourceCommit: string; sourceBaseCommit: string;
  targetCommit: string; seedCommit?: string; reviewDigest: string; contextDigest: string; createdAt: string;
}
const sha = (value: unknown) => typeof value === 'string' && /^[a-f0-9]{40}$/.test(value);
const digest = (value: unknown) => typeof value === 'string' && /^[a-f0-9]{64}$/.test(value);
const route = (run: ProjectRun, operation: RunOperation) => `/v1/product/projects/${encodeURIComponent(run.projectId)}/runs/${encodeURIComponent(run.id)}/swarm/operations/${encodeURIComponent(operation.id)}/checkpoint`;
const mismatch = () => new Error('The retained-change review does not match this task attempt. Refresh team status and review it again.');

export const swarmCheckpointApi = {
  async review(run: ProjectRun, operation: RunOperation, signal?: AbortSignal): Promise<SwarmCheckpointReview> {
    const {review} = await productApi.request<{review: SwarmCheckpointReview}>(route(run, operation), undefined, 'GET', 30_000, signal);
    if (!review || review.projectId !== run.projectId || review.runId !== run.id || review.operationId !== operation.id || review.childId !== operation.childId || review.taskId !== operation.taskId ||
      review.expectedRevision !== run.revision || typeof review.canContinue !== 'boolean' || typeof review.reason !== 'string' ||
      typeof review.sourceCommit !== 'string' || typeof review.targetCommit !== 'string' || typeof review.reviewDigest !== 'string' ||
      !Array.isArray(review.files) || review.files.length > 128 || !Number.isSafeInteger(review.totalBytes) || review.totalBytes < 0 || review.totalBytes > 512 * 1024) throw mismatch();
    if (review.canContinue && (!sha(review.sourceCommit) || !sha(review.targetCommit) || !digest(review.reviewDigest))) throw mismatch();
    const paths = new Set<string>(); let bytes = 0;
    for (const file of review.files) {
      if (!file || typeof file.path !== 'string' || !file.path || /[\\:\x00-\x1f]/.test(file.path) || file.path.startsWith('/') ||
        file.path.split('/').some(part => !part || part === '.' || part === '..') || paths.has(file.path.toLowerCase()) ||
        typeof file.status !== 'string' || typeof file.before !== 'string' || typeof file.after !== 'string' ||
        !['', '100644', '100755'].includes(file.beforeMode) || !['', '100644', '100755'].includes(file.afterMode)) throw mismatch();
      paths.add(file.path.toLowerCase());
      const before = new TextEncoder().encode(file.before).length, after = new TextEncoder().encode(file.after).length;
      if (before > 128 * 1024 || after > 128 * 1024) throw mismatch();
      bytes += before + after;
    }
    if (bytes !== review.totalBytes || bytes > 512 * 1024) throw mismatch();
    return review;
  },
  async reserve(run: ProjectRun, operation: RunOperation, review: SwarmCheckpointReview, idempotencyKey: string) {
    if (!review.canContinue || review.projectId !== run.projectId || review.runId !== run.id || review.expectedRevision !== run.revision || review.operationId !== operation.id ||
      review.childId !== operation.childId || review.taskId !== operation.taskId || !idempotencyKey) throw mismatch();
    const value = await productApi.request<{run: ProjectRun; operation: RunOperation}>(route(run, operation), {
      expectedRevision: review.expectedRevision, idempotencyKey, reviewDigest: review.reviewDigest, childId: review.childId,
    });
    const saved = value.run, child = saved?.operations?.find(item => item.id === value.operation?.id), checkpoint = child?.checkpoint;
    if (!saved || saved.id !== run.id || saved.projectId !== run.projectId || saved.input?.implementation?.projectId !== run.projectId ||
      !child || child.id === operation.id || child.childId === operation.childId || child.taskId !== operation.taskId || child.kind !== 'session' ||
      !child.worktreeId || child.worktreeId === operation.worktreeId || child.childId !== value.operation.childId || child.worktreeId !== value.operation.worktreeId ||
      saved.swarm?.tasks.find(task => task.id === operation.taskId)?.operationId !== child.id || !checkpoint ||
      checkpoint.sourceOperationId !== operation.id || checkpoint.sourceChildId !== operation.childId || checkpoint.sourceCommit !== review.sourceCommit ||
      checkpoint.targetCommit !== review.targetCommit || checkpoint.reviewDigest !== review.reviewDigest || child.baseCommit !== review.targetCommit ||
      (child.allocation === 'ready' && (!sha(checkpoint.seedCommit) || child.implementation?.sourceCommit !== checkpoint.seedCommit))) {
      throw new Error('The continued task reservation could not be verified. Check team status before taking another action.');
    }
    return {run: saved, operation: child};
  },
};
