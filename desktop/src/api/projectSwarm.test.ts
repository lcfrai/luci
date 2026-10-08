import { afterEach, expect, it, vi } from 'vitest';
import { productApi } from './product';
import { runSessionRequest, type ProjectRun, type RunOperation } from './projectRuns';
import { projectSwarmApi } from './projectSwarm';

afterEach(() => vi.restoreAllMocks());
const operation: RunOperation = {id: 'op-a', kind: 'session', taskId: 'task-a', childId: 'child-a', worktreeId: 'isolated-a', status: 'reserved', allocation: 'ready', attempt: 1, requestDigest: 'digest', createdAt: '', updatedAt: '', implementation: {projectId: 'p-1', worktreeId: 'isolated-a', taskIds: ['task-a'], taskTitles: ['Only A'], briefRevision: 1, planRevision: 2, specificationRevision: 3, designPlanRevision: 4, requirementIds: [], briefDigest: 'b', planDigest: 'p', specificationDigest: 's', designPlanDigest: 'd', sourceCommit: 'a'.repeat(40), sourceDigest: 'exact', sourceDirty: false}};
const receipt = {id: 'run-1', projectId: 'p-1', worktreeId: 'main', input: {implementation: {projectId: 'p-1', taskIds: ['task-a', 'task-b']}, route: {harness: 'codex', model: 'recorded-model', permissionMode: 'ask', connection: {id: 'original', revision: 2}}}, swarm: {state: 'ready', maxParallel: 2, tasks: [{id: 'task-a', operationId: operation.id}]}, operations: [operation]} as ProjectRun;

it('sends exact revision and reviewed integration digest without manufacturing evidence', async () => {
  const request = vi.spyOn(productApi, 'request').mockResolvedValue({run: receipt});
  await projectSwarmApi.prepare('p-1', 'run-1', 2, 3);
  await projectSwarmApi.integrate('p-1', 'run-1', 'task-a', 8, 'exact-review');
  await projectSwarmApi.abandon('p-1', 'run-1', 'task-a', 9);
  expect(request.mock.calls).toEqual([
    ['/v1/product/projects/p-1/runs/run-1/swarm', {expectedRevision: 2, maxParallel: 3}],
    ['/v1/product/projects/p-1/runs/run-1/swarm/tasks/task-a/integrate', {expectedRevision: 8, reviewDigest: 'exact-review'}],
    ['/v1/product/projects/p-1/runs/run-1/swarm/tasks/task-a/abandon', {expectedRevision: 9}],
  ]);
});

it('rejects foreign task/workspace reservations and returns only the operation bound inside the saved run', async () => {
  const request = vi.spyOn(productApi, 'request').mockResolvedValue({run: receipt, operation: {...operation, worktreeId: 'other'}});
  await expect(projectSwarmApi.reserve('p-1', 'run-1', 'task-a', 1, 'idempotent')).rejects.toThrow('could not be verified');
  request.mockResolvedValue({run: receipt, operation});
  expect((await projectSwarmApi.reserve('p-1', 'run-1', 'task-a', 1, 'idempotent')).operation).toEqual(operation);
  request.mockResolvedValue({run: {...receipt, projectId: 'p-other'}});
  await expect(projectSwarmApi.control('p-1', 'run-1', 'pause', 1)).rejects.toThrow('does not belong');
});

it('starts one captured task on the recorded route without consulting current defaults or the full run task selection', () => {
  const request = runSessionRequest(receipt, operation);
  expect(request.worktreeId).toBe('isolated-a');
  expect(request.implementationContext?.taskIds).toEqual(['task-a']);
  expect(request.model).toBe('recorded-model'); expect(request.agentConnectionId).toBe('original');
  expect(request.openingPrompt).toContain('read-only reviewed context');
  expect(request.openingPrompt).toContain('commit the task changes');
  expect(() => runSessionRequest(receipt, {...operation, implementation: {...operation.implementation!, projectId: 'p-other'}})).toThrow('no matching captured context');
  expect(() => runSessionRequest(receipt, {...operation, implementation: {...operation.implementation!, taskIds: ['task-b']}})).toThrow('no matching captured context');
});

it('rejects mismatched integration reviews instead of accepting an unrelated diff', async () => {
  vi.spyOn(productApi, 'request').mockResolvedValue({review: {taskId: 'task-a', operationId: 'old-attempt', paths: [], patch: '', reviewDigest: 'old', sourceCommit: '', targetCommit: '', canIntegrate: true, reason: ''}});
  await expect(projectSwarmApi.review('p-1', receipt, 'task-a')).rejects.toThrow('does not match');
});
