import { expect, it } from 'vitest';
import type { RunOperation } from '../../api/projectRuns';
import { swarmQueueFixture, swarmQueueItem, swarmRunFixture } from '../../test/swarmFixtures';
import { dependencyFleetCounts, dependencyTasks, matchesDependencyFilter } from './swarmDependencyModel';

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
it('reports an empty saved fleet without inventing work or verification', () => {
  const run = swarmRunFixture(); run.swarm!.tasks = [];
  expect(dependencyFleetCounts(dependencyTasks(run, swarmQueueFixture([swarmQueueItem()])))).toEqual({
    all: 0, active: 0, queued: 0, waiting: 0, review: 0, attention: 0, integrated: 0, unknown: 0,
  });
});
it('counts mixed saved task facts once per category and keeps work, input and attention overlapping', () => {
  const run = swarmRunFixture(), template = run.swarm!.tasks[0];
  run.swarm!.tasks[0].operationId = 'op-a';
  run.swarm!.tasks[1].operationId = 'op-b';
  run.swarm!.tasks[2].integration = {state: 'integrated', sourceCommit: 'a', targetCommit: 'b', reviewDigest: 'review'};
  run.swarm!.tasks.push({...template, id: 'd', operationId: undefined, dependsOn: ['a']},
    {...template, id: 'e', operationId: 'op-e'}, {...template, id: 'f', operationId: undefined}, {...template, id: 'g', operationId: undefined});
  run.operations = [operation({handoff: {digest: 'input', status: 'captured', summary: '', questions: 1, unanswered: 1, capturedAt: 'then'}}),
    operation({id: 'op-b', taskId: 'b', status: 'review-required'}), operation({id: 'op-e', taskId: 'e', status: 'future-status'})];
  const model = dependencyTasks(run, swarmQueueFixture([swarmQueueItem({taskId: 'f', state: 'paused'}),
    swarmQueueItem({id: 'g-1', taskId: 'g'}), swarmQueueItem({id: 'g-2', taskId: 'g'})]));
  expect(dependencyFleetCounts(model)).toEqual({all: 7, active: 1, queued: 1, waiting: 3, review: 1, attention: 3, integrated: 1, unknown: 1});
  expect(matchesDependencyFilter(model[0], 'active')).toBe(true);
  expect(matchesDependencyFilter(model[0], 'waiting')).toBe(true);
  expect(matchesDependencyFilter(model[0], 'attention')).toBe(true);
  expect(model[2]).not.toHaveProperty('verified');
});
it('ignores foreign and old-attempt queue identities while retaining stale-context saved facts', () => {
  const run = swarmRunFixture(); run.swarm!.tasks[0].operationId = 'op-a'; run.operations = [operation()];
  run.contextFreshness = {status: 'changed', reason: 'Design changed'};
  const model = dependencyTasks(run, swarmQueueFixture([swarmQueueItem({operationId: 'old', state: 'queued'}),
    swarmQueueItem({id: 'old-paused', operationId: 'old', state: 'paused'}), swarmQueueItem({projectId: 'other', state: 'blocked'}),
    swarmQueueItem({runId: 'other', state: 'queued'}), swarmQueueItem({taskId: 'removed', state: 'queued'})]));
  const counts = dependencyFleetCounts(model);
  expect(counts.active).toBe(1); expect(counts.queued).toBe(0); expect(counts.waiting).toBe(1);
  expect(model[0].waiting).toBe(false); expect(model[0].attention).toEqual([]);
});
it('counts the entire large fleet without counting duplicate queue observations as extra tasks', () => {
  const run = swarmRunFixture(), template = run.swarm!.tasks[0];
  run.swarm!.tasks = Array.from({length: 201}, (_, i) => ({...template, id: `task-${i}`}));
  const queue = swarmQueueFixture(run.swarm!.tasks.flatMap(task => [swarmQueueItem({id: `${task.id}-1`, taskId: task.id}), swarmQueueItem({id: `${task.id}-2`, taskId: task.id})]));
  expect(dependencyFleetCounts(dependencyTasks(run, queue))).toEqual({all: 201, active: 0, queued: 201, waiting: 0, review: 0, attention: 0, integrated: 0, unknown: 0});
});
it('does not infer pending review or verification from legacy success, stopped attempts or integration', () => {
  const run = swarmRunFixture(); run.swarm!.tasks[0].operationId = 'op-a';
  run.operations = [operation({status: 'succeeded'})];
  let model = dependencyTasks(run);
  expect(model[0].review).toBe(false); expect(model[0].unknown).toBe(true);
  expect(model[0].attention).toContain('Review disposition unconfirmed');
  run.operations[0].status = 'cancelled'; model = dependencyTasks(run);
  expect(model[0].review).toBe(false);
  run.operations[0].status = 'review-required';
  run.swarm!.tasks[0].integration = {state: 'integrated', sourceCommit: 'a', targetCommit: 'b', reviewDigest: 'review'};
  model = dependencyTasks(run);
  expect(model[0].review).toBe(false); expect(model[0].integrated).toBe(true);
  expect(model[0]).not.toHaveProperty('verified');
});
