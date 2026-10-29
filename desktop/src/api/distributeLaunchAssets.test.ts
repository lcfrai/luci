import { afterEach, expect, it, vi } from 'vitest';
import { distributeArtifactsApi } from './distributeArtifacts';
import { productApi } from './product';
afterEach(() => vi.restoreAllMocks());

it('uses the existing encoded artifact prepare and explicit save routes for launch kinds', async () => {
  const request = vi.spyOn(productApi, 'request').mockResolvedValue({});
  await distributeArtifactsApi.prepare('project/space', 'channel-copy');
  expect(request).toHaveBeenLastCalledWith('/v1/product/projects/project%2Fspace/distribute/artifacts/prepare', { kind: 'channel-copy', runId: '' }, 'POST');
  expect(request).toHaveBeenCalledTimes(1);
  await distributeArtifactsApi.save('project/space', 'channel-copy', '', 'a'.repeat(64), 'explicit-launch-save');
  expect(request).toHaveBeenLastCalledWith('/v1/product/projects/project%2Fspace/distribute/artifacts', { kind: 'channel-copy', runId: '', prepareDigest: 'a'.repeat(64), idempotencyKey: 'explicit-launch-save' }, 'POST');
});
