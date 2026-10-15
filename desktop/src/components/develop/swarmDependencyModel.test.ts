import { expect, it } from 'vitest';
import type { RunOperation } from '../../api/projectRuns';
import { swarmQueueFixture, swarmQueueItem, swarmRunFixture } from '../../test/swarmFixtures';
import { dependencyTasks } from './swarmDependencyModel';

const operation = (patch: Partial<RunOperation> = {}): RunOperation => ({id: 'op-a', kind: 'session', taskId: 'a', childId: 'session-a', attempt: 1,
  status: 'running', allocation: 'ready', requestDigest: 'request', createdAt: 'now', updatedAt: 'now', ...patch});
it('keeps retained questions and queue attention visible alongside recorded work', () => {
  const run = swarmRunFixture(); run.swarm!.tasks[0].operationId = 'op-a';
  run.operations = [operation(), operation({id: 'prior-a', attempt: 0, status: 'review-required', handoff: {digest: 'handoff', status: 'captured', summary: '', questions: 2, unanswered: 1, capturedAt: 'then'}})];
  const model = dependencyTasks(run, swarmQueueFixture([swarmQueueItem({state: 'blocked'})]));
  expect(model[0].active).toBe(true);
  expect(model[0].attention).toContain('1 unanswered question across retained attempts');
  expect(model[0].attention).toContain('Queue needs inspection');
  expect(model[0].integrated).toBe(false);
});
it('unblocks a dependency only on saved integration, never normal exit or reported success', () => {
  const run = swarmRunFixture(); run.swarm!.tasks[0].operationId = 'op-a'; run.operations = [operation({status: 'succeeded'})];
  let model = dependencyTasks(run);
  expect(model[2].waitingOn).toEqual(['a', 'b']); expect(model[0].attention).toContain('Review needed');
  run.swarm!.tasks[0].integration = {state: 'integrated', sourceCommit: 'a', targetCommit: 'b', reviewDigest: 'review'};
  model = dependencyTasks(run); expect(model[2].waitingOn).toEqual(['b']); expect(model[0].integrated).toBe(true);
  expect(model[0].attention).not.toContain('Review needed');
  expect(model[0]).not.toHaveProperty('verified');
});
it('preserves missing dependencies, invalid handoffs, foreign attempt references and future states as unconfirmed', () => {
  const run = swarmRunFixture(); run.swarm!.tasks[0].operationId = 'foreign'; run.swarm!.tasks[0].dependsOn = ['removed'];
  run.swarm!.tasks[1].operationId = 'new-status';
  run.operations = [operation({id: 'foreign', taskId: 'elsewhere'}), operation({id: 'new-status', taskId: 'b', status: 'future-status'}),
    operation({id: 'old', handoff: {digest: '', status: 'invalid', summary: '', questions: 0, unanswered: 0, capturedAt: 'then'}})];
  const model = dependencyTasks(run);
  expect(model[0].active).toBe(false); expect(model[0].unknown).toBe(true); expect(model[0].waitingOn).toEqual(['removed']);
  expect(model[1].unknown).toBe(true); expect(model[1].activity).toBe('Unrecognized attempt state');
});
it('ignores queue entries for other projects or runs and marks missing running attempt evidence', () => {
  const run = swarmRunFixture();
  const model = dependencyTasks(run, swarmQueueFixture([swarmQueueItem({projectId: 'other', state: 'failed'}), swarmQueueItem({runId: 'other', state: 'blocked'}), swarmQueueItem({taskId: 'b', state: 'running'})]));
  expect(model[0].attention).toEqual([]); expect(model[1].unknown).toBe(true); expect(model[1].active).toBe(false);
});
