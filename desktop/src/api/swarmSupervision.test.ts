import { afterEach, expect, it, vi } from 'vitest';
import { productApi } from './product';
import { checkedSupervision, mergeSwarmMessages, swarmSupervisionApi, type PostSwarmMessage } from './swarmSupervision';
import { swarmMessageFixture, swarmSupervisionFixture } from '../test/swarmSupervisionFixtures';
afterEach(() => vi.restoreAllMocks());
it('reads only the exact project/run cursor with cancellation; preserves unknown fields and observed zero', async () => {
  const value = swarmSupervisionFixture({futurePolicy: {state: 'unknown'}, activityPhase: 'future-phase', metrics: {routineDecisions: 0, latency: null, quality: null, throughput: null, cost: null}});
  const request = vi.spyOn(productApi, 'request').mockResolvedValue(value), controller = new AbortController();
  expect(await swarmSupervisionApi.get('p/1', 'run-1', 0, controller.signal)).toBe(value);
  expect(request).toHaveBeenCalledWith('/v1/product/projects/p%2F1/runs/run-1/swarm/supervision?after=0&limit=50', undefined, 'GET', 30_000, controller.signal);
  expect(value.metrics.routineDecisions).toBe(0);
});
it('rejects foreign scope, nonadvancing cursors, oversized pages and identity collisions', () => {
  expect(() => checkedSupervision('p-1', 'run-1', 0, swarmSupervisionFixture({projectId: 'foreign'}))).toThrow();
  expect(() => checkedSupervision('p-1', 'run-2', 0, swarmSupervisionFixture())).toThrow();
  expect(() => checkedSupervision('p-1', 'run-1', 1, swarmSupervisionFixture({page: {after: 1, nextCursor: 1, hasMore: true, omitted: 0}}))).toThrow();
  expect(() => checkedSupervision('p-1', 'run-1', 0, swarmSupervisionFixture({messages: Array.from({length: 51}, () => swarmMessageFixture())}))).toThrow();
  expect(() => checkedSupervision('p-1', 'run-1', 0, swarmSupervisionFixture({messages: [swarmMessageFixture(), swarmMessageFixture({id: 'msg-2'})]}))).toThrow();
});
it('orders/deduplicates pages, updates delivery only and refuses changing immutable claims or overflowing a window', () => {
  const first = swarmMessageFixture(), second = swarmMessageFixture({id: 'msg-2', sequence: 2});
  const ack = {...first, ack: {operationId: 'op-b', childId: 'session-b', actorId: 'actor', at: 'now'}};
  expect(mergeSwarmMessages([second, first], [ack]).map(message => message.id)).toEqual(['msg-1', 'msg-2']);
  expect(mergeSwarmMessages([first], [ack])[0].ack).toEqual(ack.ack);
  expect(() => mergeSwarmMessages([first], [{...first, summary: 'changed'}])).toThrow();
  expect(() => mergeSwarmMessages([ack], [first])).toThrow();
  expect(() => mergeSwarmMessages([], Array.from({length: 251}, (_, index) => swarmMessageFixture({id: `msg-${index}`, sequence: index + 1})))).toThrow();
});
it('posts the unchanged stable request and verifies returned sender/payload; acks are exact receipt requests', async () => {
  const body: PostSwarmMessage = {expectedRevision: 1, idempotencyKey: 'stable-message-request-key', recipientTaskId: 'b', kind: 'question', causalMessageId: '', evidence: {kind: 'none', id: '', digest: ''}, summary: swarmMessageFixture().summary};
  const request = vi.spyOn(productApi, 'request').mockResolvedValue({message: swarmMessageFixture(), runRevision: 2});
  await swarmSupervisionApi.post('p-1', 'run-1', 'op-a', body);
  expect(request.mock.calls[0][1]).toBe(body);
  request.mockResolvedValueOnce({message: swarmMessageFixture({recipientTaskId: 'foreign'}), runRevision: 2});
  await expect(swarmSupervisionApi.post('p-1', 'run-1', 'op-a', body)).rejects.toThrow();
  const ack = {expectedRevision: 2, recipientOperationId: 'op-b'};
  request.mockResolvedValueOnce({message: swarmMessageFixture({ack: {operationId: 'op-b', childId: 'session-b', actorId: 'actor', at: 'now'}}), runRevision: 3});
  await swarmSupervisionApi.ack('p-1', 'run-1', 'msg-1', ack);
  expect(request.mock.calls[2][1]).toBe(ack);
});
