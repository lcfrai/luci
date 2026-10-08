import { afterEach, expect, it, vi } from 'vitest';
import { projectOverviewApi, type ProjectOverview } from './projectOverview';
import { productApi } from './product';
afterEach(() => vi.restoreAllMocks());
it('rejects summaries from a different project before rendering private context', async () => {
  vi.spyOn(productApi, 'request').mockResolvedValue({ projectId: 'foreign' });
  await expect(projectOverviewApi.get('requested')).rejects.toThrow('summary could not be read');
});
it('uses a read-only project route and passes cancellation through the product transport', async () => {
  const request = vi.spyOn(productApi, 'request').mockRejectedValue(new Error('offline'));
  const controller = new AbortController();
  await expect(projectOverviewApi.get('project/one', controller.signal)).rejects.toThrow('offline');
  expect(request).toHaveBeenCalledWith('/v1/product/projects/project%2Fone/overview', undefined, 'GET', 30_000, controller.signal);
});

const attention = { id: 'earlier-build', state: 'awaiting-checks', kind: 'task-review' as const, contextStatus: 'changed' as const };
function summary(): ProjectOverview {
  return { projectId: 'p-1', observedAt: '2026-10-08T12:00:00Z', status: 'partial', goal: null, context: null, phase: { id: 'develop', label: 'Develop' }, nextAction: { id: 'inspect-run', label: 'Review earlier task changes', lens: 'develop', runId: attention.id },
    questions: { status: 'unavailable', count: null, omitted: null, items: [] }, execution: { scope: 'swarm-queue', status: 'recorded', observedAt: '', running: 0, reserved: 0, queued: 0, paused: 0, blocked: 0, unknown: 0, failed: 0, dispatchPaused: false },
    latestRun: { status: 'recorded', id: 'new-ready', state: 'ready', contextStatus: 'current' }, attention: { status: 'recorded', count: 4, items: [attention], omitted: 3 }, limitations: [] };
}
it('retains older review identities separately from an empty queue and newer prepared run', async () => {
  const request = vi.spyOn(productApi, 'request').mockResolvedValue(summary());
  const result = await projectOverviewApi.get('p-1');
  expect(result.attention).toEqual(summary().attention);
  expect(result.execution.running).toBe(0);
  expect(result.latestRun?.id).toBe('new-ready');
  expect(request).toHaveBeenCalledTimes(1);
});
it.each(['complete', 'cancelled'])('keeps reported task questions visible for a %s parent without inventing activity', async state => {
  const value = summary(); value.attention = { status: 'recorded', count: 1, omitted: 0, items: [{ ...attention, state, kind: 'task-question' }] };
  vi.spyOn(productApi, 'request').mockResolvedValue(value);
  const result = await projectOverviewApi.get('p-1');
  expect(result.attention).toEqual(value.attention); expect(result.execution.running).toBe(0);
});
it.each([undefined, { status: 'unavailable', count: null, omitted: null, items: [] }])('preserves unknown review history instead of making up zero counts', async attention => {
  vi.spyOn(productApi, 'request').mockResolvedValue({ ...summary(), attention });
  expect((await projectOverviewApi.get('p-1')).attention).toEqual(attention);
});
it.each([
  null,
  { status: 'unavailable', count: 0, omitted: 0, items: [] },
  { status: 'unavailable', count: null, omitted: null, items: [attention] },
  { status: 'recorded', count: null, omitted: 0, items: [] },
  { status: 'recorded', count: 2, omitted: 0, items: [attention] },
  { status: 'recorded', count: 2, omitted: 0, items: [attention, attention] },
  { status: 'recorded', count: 1, omitted: 0, items: [{ ...attention, contextStatus: 'verified' }] },
  { status: 'recorded', count: 4, omitted: 0, items: [0, 1, 2, 3].map(id => ({ ...attention, id: String(id) })) },
])('rejects malformed or misleading review counts and identities', async attention => {
  vi.spyOn(productApi, 'request').mockResolvedValue({ ...summary(), attention });
  await expect(projectOverviewApi.get('p-1')).rejects.toThrow('summary could not be read');
});

it('keeps recorded failures separate from pending activity and unavailable failure counts', async () => {
  const value = summary(); value.execution.failed = 2;
  const request = vi.spyOn(productApi, 'request').mockResolvedValue(value);
  const result = await projectOverviewApi.get('p-1');
  expect(result.execution.failed).toBe(2);
  expect(result.execution.running).toBe(0);
  expect(result.execution.queued).toBe(0);
  request.mockResolvedValue({ ...value, execution: { ...value.execution, status: 'unavailable', failed: null } });
  expect((await projectOverviewApi.get('p-1')).execution.failed).toBeNull();
});

it.each([
  { status: 'recorded', failed: null },
  { status: 'recorded', failed: -1 },
  { status: 'recorded', failed: '1' },
  { status: 'recorded', failed: 1.5 },
  { status: 'unavailable', failed: 0 },
  { status: 'future-observation', failed: 0 },
])('rejects misleading failure observation %s', async execution => {
  const value = summary();
  vi.spyOn(productApi, 'request').mockResolvedValue({ ...value, execution: { ...value.execution, ...execution } });
  await expect(projectOverviewApi.get('p-1')).rejects.toThrow('summary could not be read');
});

it('normalizes a legacy absent failure count to unknown rather than zero', async () => {
  const value = summary();
  vi.spyOn(productApi, 'request').mockResolvedValue({ ...value, execution: { ...value.execution, failed: undefined } });
  expect((await projectOverviewApi.get('p-1')).execution.failed).toBeNull();
});
