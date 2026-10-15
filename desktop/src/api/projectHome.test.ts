import { beforeEach, expect, it, vi } from 'vitest';
import { projectHomeApi } from './projectHome';
import { productApi } from './product';

beforeEach(() => vi.restoreAllMocks());
const row = (id: string) => ({ id, name: id, working: false, needsInput: false, queued: false, unknown: false, stops: [] });
it('pages all accessible projects without per-project calls or duplicate rail identities', async () => {
  const request = vi.spyOn(productApi, 'request').mockResolvedValueOnce({version:1,revision:2,total:2,nextOffset:1,pinnedProjectIds:['p-b'],projects:[row('p-a')]})
    .mockResolvedValueOnce({version:1,revision:2,total:2,nextOffset:null,pinnedProjectIds:['p-b'],projects:[row('p-b')]});
  const result = await projectHomeApi.getAll();
  expect(result.projects.map(project => project.id)).toEqual(['p-a','p-b']);
  expect(result.pinnedProjectIds).toEqual(['p-b']);
  expect(request.mock.calls.map(call => call[0])).toEqual(['/v1/product/home?offset=0','/v1/product/home?offset=1']);
});
it('refuses a mixed page revision and duplicate project rather than returning stale access', async () => {
  vi.spyOn(productApi, 'request').mockResolvedValueOnce({version:1,revision:2,total:2,nextOffset:1,pinnedProjectIds:[],projects:[row('p-a')]})
    .mockResolvedValueOnce({version:1,revision:3,total:2,nextOffset:null,pinnedProjectIds:[],projects:[row('p-a')]});
  await expect(projectHomeApi.getAll()).rejects.toThrow('changed');
});
