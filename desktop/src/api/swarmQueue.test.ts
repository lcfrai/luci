import { afterEach, expect, it, vi } from 'vitest';
import { productApi } from './product';
import { swarmQueueApi } from './swarmQueue';
import { swarmQueueFixture, swarmQueueItem } from '../test/swarmFixtures';
afterEach(() => vi.restoreAllMocks());

it('reads with cancellation and queues the exact reviewed batch in one request', async () => {
  const signal = new AbortController().signal, a = swarmQueueItem(), b = swarmQueueItem({id: 'q-b', taskId: 'b'});
  const request = vi.spyOn(productApi, 'request').mockResolvedValue(swarmQueueFixture());
  await swarmQueueApi.get('p-1', signal);
  expect(request.mock.lastCall).toEqual(['/v1/product/projects/p-1/swarm-queue', undefined, 'GET', 30000, signal]);
  request.mockResolvedValue({items: [a,b], queue: swarmQueueFixture([a,b])});
  await swarmQueueApi.enqueueMany('p-1', 'run-1', ['a','b'], 8, 'stable-key', 'high');
  expect(request.mock.lastCall).toEqual(['/v1/product/projects/p-1/runs/run-1/swarm/queue', {expectedRevision: 8, idempotencyKey: 'stable-key', priority: 'high', taskIds: ['a','b']}]);
  expect(request).toHaveBeenCalledTimes(2);
});

it('uses the item revision for controls despite unrelated project activity', async () => {
  const item = swarmQueueItem({revision: 3});
  const request = vi.spyOn(productApi, 'request').mockResolvedValue({item: {...item, revision: 4, state: 'paused'}, queue: swarmQueueFixture([item])});
  await swarmQueueApi.control('p-1', item, 'pause');
  expect(request.mock.lastCall).toEqual(['/v1/product/projects/p-1/swarm-queue/q-a/pause', {expectedRevision: 3}]);
  await expect(swarmQueueApi.control('foreign', item, 'cancel')).rejects.toThrow('does not match this project');
  expect(request).toHaveBeenCalledTimes(1);
  request.mockResolvedValue({item: {...item, id: 'other-queue-item'}, queue: swarmQueueFixture([item])});
  await expect(swarmQueueApi.control('p-1', item, 'pause')).rejects.toThrow('receipt does not match');
});

it('refuses foreign, incomplete and duplicate receipts and never automatically replays mutations', async () => {
  const a = swarmQueueItem(), request = vi.spyOn(productApi, 'request').mockResolvedValue(swarmQueueFixture([{...a, projectId: 'foreign'}]));
  await expect(swarmQueueApi.get('p-1')).rejects.toThrow('does not match');
  for (const items of [[a], [a,a], [a, {...a, taskId: 'foreign'}]]) {
    request.mockResolvedValue({items, queue: swarmQueueFixture()});
    await expect(swarmQueueApi.enqueueMany('p-1', 'run-1', ['a','b'], 1, 'key')).rejects.toThrow(/receipt|reviewed tasks/);
  }
  request.mockRejectedValue(new Error('response interrupted'));
  await expect(swarmQueueApi.enqueueMany('p-1', 'run-1', ['a','b'], 1, 'key')).rejects.toThrow('interrupted');
  expect(request).toHaveBeenCalledTimes(5);
});
