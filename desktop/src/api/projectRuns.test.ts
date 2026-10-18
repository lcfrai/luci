import { afterEach, expect, it, vi } from 'vitest';
import { productApi } from './product';
import { projectRunsApi, swarmIntegrationDiagnostic, validNativeDelegationRoute, type ProjectRun } from './projectRuns';

afterEach(() => vi.restoreAllMocks());
const receipt = { id: 'run-1', projectId: 'p-1', input: { implementation: { projectId: 'p-1' } }, operations: [{ id: 'op-1', childId: 's-1', kind: 'session' }] } as ProjectRun;
it.each(['sha256:' + 'a'.repeat(64), 'registry.example:5000/luci/sandbox@sha256:' + 'b'.repeat(64)])('native helpers retain exact immutable run image %s without readiness', async image => {
  const route = {harness: 'codex', model: 'opaque', permissionMode: 'bypass' as const, nativeDelegation: 'codex-native-helpers-2-v1' as const, nativeDelegationImage: image, connection: {kind: 'codex-subscription', revision: 1}};
  const value = {...receipt, input: {...receipt.input, route}};
  vi.spyOn(productApi, 'request').mockResolvedValue({run: value});
  expect((await projectRunsApi.get('p-1', 'run-1')).input.route).toBe(route);
  expect(route).not.toHaveProperty('ready');
});
it.each(['', 'sandbox:dev', 'https://host/repo@sha256:' + 'a'.repeat(64), 'UPPER/repo@sha256:' + 'a'.repeat(64), 'sha256:' + 'A'.repeat(64), 'repo@@sha256:' + 'a'.repeat(64)])('native helpers refuse invalid immutable image identity %s', nativeDelegationImage => {
  expect(validNativeDelegationRoute({harness: 'codex', permissionMode: 'bypass', nativeDelegation: 'codex-native-helpers-2-v1', nativeDelegationImage})).toBe(false);
});
it('native helpers preserve omitted legacy route bytes and refuse orphaned run image', async () => {
  const legacy = {harness: 'codex', model: 'm', permissionMode: 'ask' as const, connection: {kind: 'legacy', revision: 0}};
  const bytes = JSON.stringify(legacy);
  expect(validNativeDelegationRoute(legacy)).toBe(true); expect(JSON.stringify(legacy)).toBe(bytes);
  const value = {...receipt, input: {...receipt.input, route: {...legacy, nativeDelegationImage: 'sha256:' + 'a'.repeat(64)}}};
  vi.spyOn(productApi, 'request').mockResolvedValue({run: value});
  await expect(projectRunsApi.get('p-1', 'run-1')).rejects.toThrow(/native delegation route/);
});
it('native helpers keep exact fresh run request intent and refuse a dropped saved grant without replay', async () => {
  const input = {idempotencyKey: 'native-request-one', worktreeId: 'w-1', harness: 'codex', model: 'm', permissionMode: 'bypass' as const, nativeDelegation: 'codex-native-helpers-2-v1' as const, outcome: 'preview' as const,
    implementationContext: {projectId: 'p-1', briefRevision: 1, planRevision: 1, specificationRevision: 1, designPlanRevision: 1, taskIds: ['task-1']}};
  const route = {harness: 'codex', model: 'm', permissionMode: 'bypass' as const, nativeDelegation: input.nativeDelegation, nativeDelegationImage: 'sha256:' + 'a'.repeat(64), connection: {kind: 'legacy', revision: 0}};
  const request = vi.spyOn(productApi, 'request').mockResolvedValueOnce({run: {...receipt, input: {...receipt.input, route}}}).mockResolvedValueOnce({run: receipt});
  expect((await projectRunsApi.create('p-1', input)).input.route.nativeDelegationImage).toBe(route.nativeDelegationImage);
  expect(request.mock.calls[0][1]).toBe(input); expect(input).not.toHaveProperty('nativeDelegationImage');
  await expect(projectRunsApi.create('p-1', input)).rejects.toThrow(/differs/);
  expect(request).toHaveBeenCalledTimes(2);
});

it('keeps owned unknown integration readable and renders only closed diagnostic labels', async () => {
  const integration = {state: 'unknown', sourceCommit: 'a', targetCommit: 'b', reviewDigest: 'd', stage: 'target-check', failureCode: 'target-changed'};
  const value = {...receipt, swarm: {state: 'ready', maxParallel: 1, tasks: [{id: 'task', integration}]}} as ProjectRun;
  const request = vi.spyOn(productApi, 'request').mockResolvedValue({run: value});
  const known = await projectRunsApi.get('p-1', 'run-1');
  expect(swarmIntegrationDiagnostic(known.swarm!.tasks[0].integration)).toContain('Target revalidation: The target no longer matched its reviewed state.');
  for (const fields of [{stage: 'PRIVATE_PATH', failureCode: 'command-failed'}, {stage: '__proto__', failureCode: 'command-failed'}, {stage: 'candidate-import'}, {stage: 'candidate-import', failureCode: 'target-changed'}]) {
    const unsafe = {...integration, failureCode: undefined, ...fields};
    const source = {...value, swarm: {...value.swarm!, tasks: [{...value.swarm!.tasks[0], integration: unsafe}]}} as unknown as ProjectRun;
    request.mockResolvedValue({run: source});
    const safe = await projectRunsApi.get('p-1', 'run-1');
    expect(safe.swarm!.tasks[0].integration?.state).toBe('unknown');
    expect(safe.swarm!.tasks[0].integration?.stage).toBeUndefined();
    expect(swarmIntegrationDiagnostic(safe.swarm!.tasks[0].integration)).toBeUndefined();
    expect(source.swarm!.tasks[0].integration?.stage).toBe(fields.stage);
  }
});

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

it('passes a selected-view cancellation signal only to its read request',async()=>{
  const controller=new AbortController(),request=vi.spyOn(productApi,'request').mockResolvedValue({run:receipt});
  expect(await projectRunsApi.get('p-1','run-1',controller.signal)).toBe(receipt);
  expect(request).toHaveBeenCalledExactlyOnceWith('/v1/product/projects/p-1/runs/run-1',undefined,'GET',30_000,controller.signal);
});

it('validates the child reservation and propagates stale-revision failure without replay', async () => {
  const request = vi.spyOn(productApi, 'request').mockResolvedValue({ run: receipt, operation: { id: 'op-1', childId: 's-foreign' } });
  await expect(projectRunsApi.reserve('p-1', 'run-1', 3, 'idempotency-key-1')).rejects.toThrow('reservation could not be verified');
  request.mockRejectedValue(new Error('The run changed. Reload before continuing.'));
  await expect(projectRunsApi.joinCheck('p-1', 'run-1', 2, 'check-1')).rejects.toThrow('The run changed');
  expect(request).toHaveBeenCalledTimes(2);
});


it.each(['provider-reservation','run-revalidation','admission','lifecycle-lock','launch-journal'])('retains a validated %s pre-launch diagnostic from the owned run',async stage=>{
  const value={...receipt,operations:[{...receipt.operations[0],status:'failed',startFailure:{stage,reasonCode:'start_refused',message:'The task could not start.'}}]};
  const request=vi.spyOn(productApi,'request').mockResolvedValue({run:value});
  expect((await projectRunsApi.get('p-1','run-1')).operations[0].startFailure).toEqual(value.operations[0].startFailure);expect(request).toHaveBeenCalledOnce();
});

it.each([{stage:'future-stage',reasonCode:'refused',message:'Refused'}, {stage:'admission',reasonCode:'',message:'Refused'}, {stage:'admission',reasonCode:'refused',message:''}, null])('refuses an unknown or malformed start diagnostic without replay',async startFailure=>{
  const request=vi.spyOn(productApi,'request').mockResolvedValue({run:{...receipt,operations:[{...receipt.operations[0],startFailure}]}});
  await expect(projectRunsApi.get('p-1','run-1')).rejects.toThrow('task start diagnostic could not be verified');expect(request).toHaveBeenCalledOnce();
});
