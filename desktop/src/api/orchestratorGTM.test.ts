import { afterEach, expect, it, vi } from 'vitest';
import { productApi } from './product';
import { orchestratorApi } from './orchestrator';
import { emptyGTM } from './gtmWorkspace';
import { context, view } from '../components/orchestrator/fixtures';

afterEach(() => vi.restoreAllMocks());
it('accepts legacy context and an exact optional GTM snapshot without a new endpoint', async () => {
  const request = vi.spyOn(productApi, 'request').mockResolvedValue(view());
  await expect(orchestratorApi.get('p-1')).resolves.toHaveProperty('projectId', 'p-1');
  const value = view({ context: { ...context(), gtm: { revision: 0, digest: '', sourceDigest: 'c'.repeat(64), content: emptyGTM(), artifactLinks: [], artifactsOmitted: 0 } } });
  request.mockResolvedValue(value);
  await expect(orchestratorApi.get('p-1')).resolves.toEqual(value);
  expect(request).toHaveBeenLastCalledWith('/v1/product/projects/p-1/orchestrator');
});
it('refuses an invalid optional GTM snapshot instead of displaying it as saved context', async () => {
  vi.spyOn(productApi, 'request').mockResolvedValue(view({ context: { ...context(), gtm: { revision: 2, digest: '', sourceDigest: 'c'.repeat(64), content: emptyGTM(), artifactLinks: [], artifactsOmitted: 0 } } }));
  await expect(orchestratorApi.get('p-1')).rejects.toThrow(/could not be verified/);
});
