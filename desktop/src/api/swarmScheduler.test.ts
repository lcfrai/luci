import { afterEach, expect, it, vi } from 'vitest';
import { productApi } from './product';
import { swarmSchedulerApi, type SwarmScheduler } from './swarmScheduler';

const policy = { maxActive: 4, maxPerProject: 2, headroomPercent: 25, minHeadroomMiB: 2048, diskReserveMiB: 2048 };
const ready = { status: 'ready', observedAt: '2026-10-08T22:00:00.123456789Z', count: 2, memoryMiB: 3072, cpuMillis: 1500 };
const response = (reservations: unknown) => ({ revision: 7, observedAt: '', dispatchPaused: false, policy, resources: { reservations } });
afterEach(() => vi.restoreAllMocks());

it('reads explicit ready allocations with cancellation and preserves the reviewed configure request', async () => {
  const request = vi.spyOn(productApi, 'request').mockResolvedValue(response(ready));
  const signal = new AbortController().signal;
  const current = await swarmSchedulerApi.get(signal);
  expect(current.resources?.reservations).toEqual(ready);
  expect(request).toHaveBeenLastCalledWith('/v1/product/swarm-scheduler', undefined, 'GET', 30_000, signal);
  request.mockResolvedValue({ ...response({ ...ready, count: 0, memoryMiB: 0, cpuMillis: 0 }), revision: 8, dispatchPaused: true });
  const updated = await swarmSchedulerApi.configure(current, policy, true);
  expect(updated.resources?.reservations).toEqual({ ...ready, count: 0, memoryMiB: 0, cpuMillis: 0 });
  expect(request).toHaveBeenLastCalledWith('/v1/product/swarm-scheduler', { expectedRevision: 7, policy, dispatchPaused: true }, 'PUT');
});

it.each([undefined, null, {}, { observedAt: ready.observedAt, count: 0, memoryMiB: 0, cpuMillis: 0 }, { status: 'unavailable' }])(
  'keeps missing, legacy or explicitly unavailable observations untrusted: %j', async reservations => {
    vi.spyOn(productApi, 'request').mockResolvedValue(response(reservations));
    expect((await swarmSchedulerApi.get()).resources?.reservations).toEqual({ status: 'unavailable' });
  },
);

it.each([
  ['unknown status', { ...ready, status: 'cached' }],
  ['explicit missing status', { ...ready, status: undefined }],
  ['unavailable with counters', { status: 'unavailable', count: 0 }],
  ['unavailable with timestamp', { status: 'unavailable', observedAt: ready.observedAt }],
  ['missing count', { ...ready, count: undefined }],
  ['negative count', { ...ready, count: -1 }],
  ['fractional count', { ...ready, count: 0.5 }],
  ['string count', { ...ready, count: '2' }],
  ['unsafe count', { ...ready, count: Number.MAX_SAFE_INTEGER + 1 }],
  ['missing memory', { ...ready, memoryMiB: undefined }],
  ['negative memory', { ...ready, memoryMiB: -1 }],
  ['nonfinite CPU', { ...ready, cpuMillis: Infinity }],
  ['fractional CPU', { ...ready, cpuMillis: 0.1 }],
  ['missing timestamp', { ...ready, observedAt: undefined }],
  ['invalid timestamp', { ...ready, observedAt: 'invalid' }],
  ['date without time', { ...ready, observedAt: '2026-10-08' }],
  ['array', []],
  ['scalar', 0],
])('rejects malformed reservation responses: %s', async (_label, reservations) => {
  vi.spyOn(productApi, 'request').mockResolvedValue(response(reservations));
  await expect(swarmSchedulerApi.get()).rejects.toThrow('Workspace reservations could not be read');
});

it('also refuses malformed mutation receipts without replaying the write', async () => {
  const request = vi.spyOn(productApi, 'request').mockResolvedValue(response({ ...ready, count: null }));
  const current: SwarmScheduler = { revision: 7, observedAt: '', dispatchPaused: false, policy, resources: null };
  await expect(swarmSchedulerApi.configure(current, policy, true)).rejects.toThrow('Workspace reservations could not be read');
  expect(request).toHaveBeenCalledTimes(1);
});
