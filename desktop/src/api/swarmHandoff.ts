import { productApi } from './product';
import type { ProjectRun, RunOperation, SwarmTask } from './projectRuns';
import { safeSwarmIntegration } from './projectRuns';

export type TaskHandoffStatus = 'not-recorded' | 'captured' | 'missing' | 'invalid' | 'unavailable';
export interface TaskHandoffReport {
  version: 1; summary: string; changedFiles: string[];
  decisions: {decision: string; rationale: string}[];
  reportedChecks: {command: string; outcome: 'passed' | 'failed' | 'not-run' | 'unknown'; detail: string}[];
  questions: {question: string; impact: string; blocking: boolean}[];
  limitations: string[];
}
export interface TaskHandoffReference {
  digest: string; status: Exclude<TaskHandoffStatus, 'not-recorded'>; summary: string;
  questions: number; unanswered: number; answersDigest?: string; capturedAt: string;
	coordinationOmitted?: boolean;
}
export interface TaskHandoffReceipt {
  version: 1; projectId: string; runId: string; taskId: string; operationId: string; childId: string; worktreeId: string;
  requestDigest: string; implementationDigest: string; contextDigest?: string;
  sourceCommit: string; sourceDigest: string; sourceDirty: boolean;
  status: Exclude<TaskHandoffStatus, 'not-recorded'>; reasonCode?: string; capturedAt: string;
  reported: TaskHandoffReport | null;
}
export interface TaskHandoffView {
  projectId: string; runId: string; taskId: string; operationId: string; status: TaskHandoffStatus;
  digest?: string; receipt: TaskHandoffReceipt | null;
  answers: {questionId: string; answer: string; answeredAt: string; attribution: string}[];
  questionIds: string[]; recordedChecks: ProjectRun['checks']; integration: SwarmTask['integration'] | null;
}
export interface TaskHandoffMutation {run: ProjectRun; handoff: TaskHandoffView}

const record = (value: unknown): value is Record<string, unknown> => !!value && typeof value === 'object' && !Array.isArray(value);
const text = (value: unknown, maximum: number): value is string => typeof value === 'string' && !value.includes('\0') && new TextEncoder().encode(value).length <= maximum;
const digest = (value: unknown): value is string => typeof value === 'string' && /^[a-f0-9]{64}$/.test(value);
const timestamp = (value: unknown): value is string => text(value, 128) && !!value && Number.isFinite(Date.parse(value));
const list = (value: unknown, maximum: number): value is unknown[] => Array.isArray(value) && value.length <= maximum;
const relativePath = (value: unknown): value is string => text(value, 512) && !!value && !/[\\:\x00-\x1f\x7f]/.test(value) && !value.split('/').some(part => !part || part === '.' || part === '..');
const mismatch = () => new Error('The task result could not be verified for this exact attempt. Refresh team status before reviewing it again.');
const route = (projectId: string, runId: string, operationId: string) => `/v1/product/projects/${encodeURIComponent(projectId)}/runs/${encodeURIComponent(runId)}/swarm/operations/${encodeURIComponent(operationId)}/handoff`;

function validReport(value: unknown): value is TaskHandoffReport {
  return record(value) && value.version === 1 && text(value.summary, 4000) &&
    list(value.changedFiles, 128) && value.changedFiles.every(relativePath) &&
    list(value.decisions, 8) && value.decisions.every(item => record(item) && text(item.decision, 2000) && text(item.rationale, 2000)) &&
    list(value.reportedChecks, 16) && value.reportedChecks.every(item => record(item) && text(item.command, 2000) && text(item.detail, 2000) && ['passed', 'failed', 'not-run', 'unknown'].includes(item.outcome as string)) &&
    list(value.questions, 8) && value.questions.every(item => record(item) && text(item.question, 2000) && !!item.question.trim() && text(item.impact, 2000) && typeof item.blocking === 'boolean') &&
    list(value.limitations, 16) && value.limitations.every(item => text(item, 2000));
}
function verified(value: unknown, projectId: string, runId: string, operation: RunOperation): TaskHandoffView {
  if (!record(value) || value.projectId !== projectId || value.runId !== runId || value.operationId !== operation.id || value.taskId !== operation.taskId || !operation.taskId ||
    !['not-recorded', 'captured', 'missing', 'invalid', 'unavailable'].includes(value.status as string) ||
    !list(value.questionIds, 8) || !value.questionIds.every(digest) || new Set(value.questionIds).size !== value.questionIds.length ||
    !list(value.answers, 8) || !value.answers.every(answer => record(answer) && value.questionIds instanceof Array && value.questionIds.includes(answer.questionId) && text(answer.answer, 4000) && !!answer.answer.trim() && timestamp(answer.answeredAt) && text(answer.attribution, 1000)) ||
    new Set(value.answers.map(answer => (answer as Record<string, unknown>).questionId)).size !== value.answers.length ||
    !list(value.recordedChecks, 256) || !value.recordedChecks.every(check => record(check) && check.projectId === projectId && text(check.checkId, 512) && !!check.checkId && text(check.commit, 128) && text(check.sourceDigest, 128) && text(check.execution, 512) && text(check.joinedAt, 128) && typeof check.originalAvailable === 'boolean' &&
      list(check.commands, 128) && check.commands.every(command => record(command) && text(command.label, 4000) && text(command.executable, 4000) && (command.args === null || list(command.args, 256) && command.args.every(arg => text(arg, 64 * 1024))) && text(command.status, 128) && Number.isSafeInteger(command.exitCode)))) throw mismatch();
  if (value.integration !== null && (!record(value.integration) || !['integrating', 'integrated', 'unknown'].includes(value.integration.state as string) || !text(value.integration.sourceCommit, 128) || !text(value.integration.targetCommit, 128) || !text(value.integration.reviewDigest, 128) || (value.integration.commit !== undefined && !text(value.integration.commit, 128)))) throw mismatch();
  if (value.status === 'not-recorded') {
    if (value.receipt !== null || value.digest !== undefined || value.questionIds.length || value.answers.length) throw mismatch();
  } else {
    const receipt = value.receipt;
    if (!digest(value.digest) || !record(receipt) || receipt.version !== 1 || receipt.projectId !== projectId || receipt.runId !== runId || receipt.taskId !== operation.taskId || receipt.operationId !== operation.id || receipt.childId !== operation.childId || receipt.worktreeId !== operation.worktreeId || receipt.requestDigest !== operation.requestDigest ||
      !digest(receipt.implementationDigest) || (receipt.contextDigest !== undefined && !digest(receipt.contextDigest)) || !text(receipt.sourceCommit, 128) || !text(receipt.sourceDigest, 128) || typeof receipt.sourceDirty !== 'boolean' || receipt.status !== value.status || !timestamp(receipt.capturedAt) || (receipt.reasonCode !== undefined && !text(receipt.reasonCode, 512))) throw mismatch();
    if (value.status === 'captured') {
      if (!validReport(receipt.reported) || receipt.reported.questions.length !== value.questionIds.length) throw mismatch();
    } else if (receipt.reported !== null || value.questionIds.length || value.answers.length) throw mismatch();
  }
  const result = value as unknown as TaskHandoffView;
  return {...result, integration: result.integration ? safeSwarmIntegration(result.integration) : result.integration, recordedChecks: result.recordedChecks.map(check => ({...check, commands: check.commands.map(command => ({...command, args: command.args ?? []}))}))};
}
function mutation(value: TaskHandoffMutation, original: ProjectRun, operation: RunOperation): TaskHandoffMutation {
  const run = value?.run, saved = run?.operations?.find(item => item.id === operation.id);
  if (!run || run.id !== original.id || run.projectId !== original.projectId || run.input?.implementation?.projectId !== original.projectId || !Number.isSafeInteger(run.revision) || run.revision < original.revision || !saved || saved.taskId !== operation.taskId || saved.childId !== operation.childId || saved.worktreeId !== operation.worktreeId || saved.requestDigest !== operation.requestDigest) throw mismatch();
  return {run, handoff: verified(value.handoff, original.projectId, original.id, operation)};
}

export const swarmHandoffApi = {
  async get(projectId: string, runId: string, operation: RunOperation, signal?: AbortSignal): Promise<TaskHandoffView> {
    const value = await productApi.request<{handoff: unknown}>(route(projectId, runId, operation.id), undefined, 'GET', 30_000, signal);
    return verified(value?.handoff, projectId, runId, operation);
  },
  async collect(run: ProjectRun, operation: RunOperation): Promise<TaskHandoffMutation> {
    const value = await productApi.request<TaskHandoffMutation>(`${route(run.projectId, run.id, operation.id)}/collect`, {expectedRevision: run.revision});
    return mutation(value, run, operation);
  },
  async answer(run: ProjectRun, operation: RunOperation, handoffDigest: string, questionId: string, answer: string): Promise<TaskHandoffMutation> {
    if (!digest(handoffDigest) || !digest(questionId) || !text(answer, 4000) || !answer.trim()) throw new Error('Answer this recorded question using at most 4,000 UTF-8 bytes.');
    const value = await productApi.request<TaskHandoffMutation>(`${route(run.projectId, run.id, operation.id)}/questions/${encodeURIComponent(questionId)}/answer`, {expectedRevision: run.revision, handoffDigest, answer});
    return mutation(value, run, operation);
  },
};
