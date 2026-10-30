import { afterEach, expect, it, vi } from 'vitest';
import { coordinatorIntentsApi, type CoordinatorIntent } from './coordinatorIntents';
import { coordinatorTargets } from './coordinatorPolicy';
import { productApi } from './product';
afterEach(() => vi.restoreAllMocks());

it('reads an explicit-review GTM intent while keeping unknown and automatic targets closed', async () => {
  const hash = 'a'.repeat(64);
  const intent: CoordinatorIntent = { id: 'intent-gtm', turnId: 'turn-gtm', actor: { id: 'alice', displayName: 'Alice' }, luciId: `luci-${hash}`, baseContextDigest: hash, proposalDigest: hash, target: 'gtm', requestedChange: 'Review a pilot strategy', changedPaths: ['offer'], requirementIds: [], taskIds: [], affectsAllTasks: false, dependsOn: [], status: 'pending', createdAt: '2026-10-30T00:00:00Z' };
  const value = { projectId: 'p-1', revision: 1, contextDigest: hash, intents: [intent], events: [], conflicts: [], allowedActions: ['propose-own-turn'] };
  const request = vi.spyOn(productApi, 'request').mockResolvedValue(value);
  expect((await coordinatorIntentsApi.get('p-1')).intents[0].target).toBe('gtm');
  expect(coordinatorTargets).not.toContain('gtm');
  request.mockResolvedValue({ ...value, intents: [{ ...intent, target: 'publish-campaign' }] });
  await expect(coordinatorIntentsApi.get('p-1')).rejects.toThrow();
});
