import { afterEach, expect, it, vi } from 'vitest';
import { productApi } from './product';
import type { RunOperation } from './projectRuns';
import { swarmWorkspaceApi, type SwarmWorkspace, type SwarmWorkspaceFile } from './swarmWorkspace';

const operation: RunOperation = {id: 'op-a', taskId: 'task-a', worktreeId: 'isolated-a', childId: 'never-started', kind: 'session', status: 'cancelled', allocation: 'abandoned', attempt: 1, requestDigest: '', createdAt: '', updatedAt: ''};
const workspace: SwarmWorkspace = {operationId: 'op-a', taskId: 'task-a', worktreeId: 'isolated-a', exists: true, files: [{path: 'src/new file.ts', status: '??'}], filesOmitted: 0, filesTruncated: false, patch: '', patchTruncated: false, reason: ''};
const file: SwarmWorkspaceFile = {operationId: 'op-a', taskId: 'task-a', worktreeId: 'isolated-a', path: 'src/new file.ts', bytes: 20, content: 'export const value=1', binary: false, omitted: false, reason: ''};
afterEach(() => vi.restoreAllMocks());

it('reads an abandoned attempt without a child through GET-only bounded requests and preserves cancellation', async () => {
  const request = vi.spyOn(productApi, 'request').mockResolvedValueOnce({workspace}).mockResolvedValueOnce({file});
  const controller = new AbortController();
  expect(await swarmWorkspaceApi.inspect('project A', 'run A', operation, controller.signal)).toEqual(workspace);
  expect(await swarmWorkspaceApi.file('project A', 'run A', operation, file.path, controller.signal)).toEqual(file);
  expect(request.mock.calls).toEqual([
    ['/v1/product/projects/project%20A/runs/run%20A/swarm/operations/op-a/workspace', undefined, 'GET', 30_000, controller.signal],
    ['/v1/product/projects/project%20A/runs/run%20A/swarm/operations/op-a/workspace/file?path=src%2Fnew%20file.ts', undefined, 'GET', 30_000, controller.signal],
  ]);
});

it.each([
  {operationId: 'foreign'}, {taskId: 'foreign'}, {worktreeId: 'foreign'},
  {files: [{path: '/private/host/path', status: '??'}]}, {files: [{path: '../secret', status: '??'}]},
  {files: [{path: 'src/a.ts', status: 'M'}, {path: 'src/a.ts', status: 'M'}]},
  {filesOmitted: -1}, {patch: 'x'.repeat(128 * 1024 + 1)},
])('refuses unrelated or unsupported workspace inventory %j', async invalid => {
  vi.spyOn(productApi, 'request').mockResolvedValue({workspace: {...workspace, ...invalid}});
  await expect(swarmWorkspaceApi.inspect('p-1', 'run-1', operation)).rejects.toThrow('does not match');
});

it('allows explicitly bounded partial text and binary metadata, and refuses wrong-file or binary-content substitutions', async () => {
  const request = vi.spyOn(productApi, 'request');
  request.mockResolvedValue({file: {...file, bytes: 500_000, omitted: true, content: 'Partial prefix'}});
  expect((await swarmWorkspaceApi.file('p-1', 'run-1', operation, file.path)).content).toBe('Partial prefix');
  request.mockResolvedValue({file: {...file, binary: true, omitted: true, content: ''}});
  expect((await swarmWorkspaceApi.file('p-1', 'run-1', operation, file.path)).binary).toBe(true);
  for (const invalid of [{path: 'other.ts'}, {worktreeId: 'foreign'}, {binary: true}, {content: 'x'.repeat(128 * 1024 + 1)}]) {
    request.mockResolvedValue({file: {...file, ...invalid}});
    await expect(swarmWorkspaceApi.file('p-1', 'run-1', operation, file.path)).rejects.toThrow('does not match');
  }
  request.mockClear();
  await expect(swarmWorkspaceApi.file('p-1', 'run-1', operation, '/etc/passwd')).rejects.toThrow('project-relative');
  expect(request).not.toHaveBeenCalled();
});
