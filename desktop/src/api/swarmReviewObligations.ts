import { productApi } from './product';
import { checkedReviewObligation, type SwarmReviewObligation } from './swarmSupervision';

export interface CreateReviewObligation {
  expectedRevision: number; idempotencyKey: string; candidateOperationId: string; reviewerOperationId: string; reviewDigest: string;
}
export interface ResolveReviewObligation {
  expectedRevision: number; outcome: 'passed' | 'repair-required'; reviewerHandoffDigest: string; findingMessageId: string;
}
export interface ReviewObligationResult {obligation: SwarmReviewObligation; runRevision: number}
export interface ReviewObligationReadback extends ReviewObligationResult {projectId: string; runId: string}
export interface ReviewObligationDisposition {projectId: string; runId: string; taskId: string; state: 'recorded' | 'rejected' | 'unconfirmed' | 'key-conflict'; runRevision: number; obligation: SwarmReviewObligation | null}
export interface ReviewResolutionRetirement {projectId: string; runId: string; obligationId: string; state: 'rejected'; runRevision: number}
const base = (project: string, run: string) => `/v1/product/projects/${encodeURIComponent(project)}/runs/${encodeURIComponent(run)}/swarm`;
const revision = (value: unknown): value is number => Number.isSafeInteger(value) && (value as number) >= 1;
export const swarmReviewObligationsApi = {
  async create(project: string, run: string, task: string, request: CreateReviewObligation, signal?: AbortSignal) {
    const result = await productApi.request<ReviewObligationResult>(`${base(project, run)}/tasks/${encodeURIComponent(task)}/review-obligations`, request, 'POST', 30_000, signal);
    checkedReviewObligation(result?.obligation);
    if (!revision(result.runRevision) || result.runRevision < request.expectedRevision || result.obligation.taskId !== task ||
        result.obligation.candidateOperationId !== request.candidateOperationId || result.obligation.reviewerOperationId !== request.reviewerOperationId ||
        result.obligation.reviewDigest !== request.reviewDigest) throw new Error('The independent review reservation does not match its exact request. Inspect saved history.');
    return result;
  },
  async resolve(project: string, run: string, id: string, request: ResolveReviewObligation, signal?: AbortSignal) {
    const result = await productApi.request<ReviewObligationResult>(`${base(project, run)}/review-obligations/${encodeURIComponent(id)}/resolve`, request, 'POST', 30_000, signal);
    checkedReviewObligation(result?.obligation);
    if (!revision(result.runRevision) || result.runRevision < request.expectedRevision || result.obligation.id !== id ||
        result.obligation.state !== request.outcome || result.obligation.reviewerHandoffDigest !== request.reviewerHandoffDigest ||
        result.obligation.findingMessageId !== request.findingMessageId) throw new Error('The independent review outcome does not match its exact request. Inspect saved history.');
    return result;
  },
  async get(project: string, run: string, id: string, signal?: AbortSignal) {
    const value = await productApi.request<ReviewObligationReadback>(`${base(project, run)}/review-obligations/${encodeURIComponent(id)}`, undefined, 'GET', 30_000, signal);
    checkedReviewObligation(value?.obligation);
    if (value.projectId !== project || value.runId !== run || value.obligation.id !== id || !revision(value.runRevision)) throw new Error('The independent review readback is outside this project and run.');
    return value;
  },
  async disposition(project: string, run: string, task: string, request: CreateReviewObligation, signal?: AbortSignal) {
    const value = await productApi.request<ReviewObligationDisposition>(`${base(project, run)}/tasks/${encodeURIComponent(task)}/review-obligations/disposition`, request, 'POST', 30_000, signal);
    if (value?.projectId !== project || value.runId !== run || value.taskId !== task || !revision(value.runRevision) ||
        value.runRevision < request.expectedRevision || !['recorded', 'rejected', 'unconfirmed', 'key-conflict'].includes(value.state) ||
        (value.state === 'recorded') !== !!value.obligation) throw new Error('The exact review request disposition is unconfirmed.');
    if (value.obligation) {
      checkedReviewObligation(value.obligation);
      if (value.obligation.taskId !== task || value.obligation.candidateOperationId !== request.candidateOperationId ||
          value.obligation.reviewerOperationId !== request.reviewerOperationId || value.obligation.reviewDigest !== request.reviewDigest) throw new Error('The saved review differs from its retained request.');
    }
    return value;
  },
  async retire(project: string, run: string, task: string, request: CreateReviewObligation, signal?: AbortSignal) {
    const value = await productApi.request<ReviewObligationDisposition>(`${base(project, run)}/tasks/${encodeURIComponent(task)}/review-obligations/retire`, request, 'POST', 30_000, signal);
    if (value?.projectId !== project || value.runId !== run || value.taskId !== task || value.state !== 'rejected' || value.obligation !== null || !revision(value.runRevision) || value.runRevision <= request.expectedRevision) throw new Error('The exact review retirement was not confirmed. Keep its retained request.');
    return value;
  },
  async retireResolution(project: string, run: string, id: string, request: ResolveReviewObligation, signal?: AbortSignal) {
    const value = await productApi.request<ReviewResolutionRetirement>(`${base(project, run)}/review-obligations/${encodeURIComponent(id)}/resolve/retire`, request, 'POST', 30_000, signal);
    if (value?.projectId !== project || value.runId !== run || value.obligationId !== id || value.state !== 'rejected' || !revision(value.runRevision) || value.runRevision <= request.expectedRevision) throw new Error('The exact reviewer outcome retirement was not confirmed. Keep its retained request.');
    return value;
  },
};
