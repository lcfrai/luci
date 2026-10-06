import { afterEach, expect, it, vi } from 'vitest';
import { productApi } from './product';
import { projectRunsApi, type ProjectRun } from './projectRuns';

afterEach(() => vi.restoreAllMocks());
const receipt = { id: 'run-1', projectId: 'p-1', input: { implementation: { projectId: 'p-1' } }, operations: [{ id: 'op-1', childId: 's-1', kind: 'session' }] } as ProjectRun;

it('uses read-only list/get and exact typed evidence/acceptance payloads', async () => {
  const request = vi.spyOn(productApi, 'request').mockResolvedValue({ run: receipt, runs: [receipt] });
  await projectRunsApi.list('p-1');
  await projectRunsApi.get('p-1', 'run-1');
  expect(request.mock.calls[0]).toEqual(['/v1/product/projects/p-1/runs']);
  expect(request.mock.calls[1]).toEqual(['/v1/product/projects/p-1/runs/run-1']);
  await projectRunsApi.joinRelease('p-1', 'run-1', 3, 'release-1', 'pages');
  expect(request.mock.lastCall).toEqual(['/v1/product/projects/p-1/runs/run-1/releases', { expectedRevision: 3, releaseId: 'release-1', target: 'pages' }]);
  await projectRunsApi.accept('p-1', 'run-1', 4);
  expect(request.mock.lastCall).toEqual(['/v1/product/projects/p-1/runs/run-1/accept', { expectedRevision: 4 }]);
});

it('rejects foreign project/run results instead of allowing their actions', async () => {
  const request = vi.spyOn(productApi, 'request').mockResolvedValue({ runs: [{ ...receipt, projectId: 'p-2' }] });
  await expect(projectRunsApi.list('p-1')).rejects.toThrow('another project');
  request.mockResolvedValue({ run: { ...receipt, id: 'run-other' } });
  await expect(projectRunsApi.cancel('p-1', 'run-1', 1)).rejects.toThrow('another project');
});

it('validates the child reservation and propagates stale-revision failure without replay', async () => {
  const request = vi.spyOn(productApi, 'request').mockResolvedValue({ run: receipt, operation: { id: 'op-1', childId: 's-foreign' } });
  await expect(projectRunsApi.reserve('p-1', 'run-1', 3, 'idempotency-key-1')).rejects.toThrow('reservation could not be verified');
  request.mockRejectedValue(new Error('The run changed. Reload before continuing.'));
  await expect(projectRunsApi.joinCheck('p-1', 'run-1', 2, 'check-1')).rejects.toThrow('The run changed');
  expect(request).toHaveBeenCalledTimes(2);
});
