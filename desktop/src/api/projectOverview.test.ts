import { afterEach, expect, it, vi } from 'vitest';
import { projectOverviewApi } from './projectOverview';
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
