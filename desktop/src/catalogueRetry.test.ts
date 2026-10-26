import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { startCatalogue, type Catalogue } from './App';
import { ApiError, type CockpitClient } from './api/client';
import { selectTeamScope } from './api/teamScope';
import { resetCockpit, useCockpit } from './store/store';
import { aBranch, aProject } from './test/fixtures';

let catalogue: Catalogue | undefined;
beforeEach(() => { resetCockpit(); selectTeamScope(''); });
afterEach(() => { catalogue?.stop(); catalogue = undefined; });
// Resolve only the catalogue's controlled promise chain, without an app poller.
async function settle() { for (let i = 0; i < 8; i++) await Promise.resolve(); }
function fixture() {
  const client = {
    baseUrl: 'http://127.0.0.1:62000',
    listMachines: vi.fn().mockResolvedValue([]),
    listProjects: vi.fn().mockResolvedValue([aProject({ id: 'p' })]),
    listHarnesses: vi.fn().mockResolvedValue([]),
    listBranches: vi.fn().mockRejectedValue(new ApiError('timeout', 'Original branch timeout', 0)),
    listWorktrees: vi.fn().mockResolvedValue([]),
  };
  useCockpit.setState({ projects: [aProject({ id: 'p' })], expandedProjects: { p: true } });
  catalogue = startCatalogue(client as unknown as CockpitClient, useCockpit);
  return client;
}

it('retries only the exact failed branch read and clears its own report on success', async () => {
  const client = fixture(); await settle();
  const original = useCockpit.getState().lastError!;
  expect(original.branchLoad?.projectId).toBe('p');
  client.listBranches.mockResolvedValue([aBranch({ id: 'b', projectId: 'p' })]);
  catalogue!.retryBranches(original); catalogue!.retryBranches(original);
  await settle();
  expect(client.listBranches).toHaveBeenCalledTimes(2);
  expect(client.listBranches).toHaveBeenLastCalledWith('p');
  expect(client.listProjects).toHaveBeenCalledTimes(1);
  expect(client.listMachines).toHaveBeenCalledTimes(1); expect(client.listHarnesses).toHaveBeenCalledTimes(1);
  expect(useCockpit.getState().branchesByProject.p[0].id).toBe('b');
  expect(useCockpit.getState().lastError).toBeNull();
});

it('refreshes a failed harness root once without invalidating healthy roots or clearing its error before recovery', async () => {
  let resolveHarnesses!: (value: unknown[]) => void;
  const harnesses = new Promise<unknown[]>(resolve => { resolveHarnesses = resolve; });
  const client = {
    baseUrl: 'http://127.0.0.1:62000',
    listMachines: vi.fn().mockResolvedValue([]),
    listProjects: vi.fn().mockResolvedValue([aProject({ id: 'p' })]),
    listHarnesses: vi.fn().mockRejectedValueOnce(new ApiError('timeout', 'Original harness timeout', 0)).mockReturnValue(harnesses),
    listBranches: vi.fn().mockResolvedValue([]),
    listWorktrees: vi.fn().mockResolvedValue([]),
  };
  catalogue = startCatalogue(client as unknown as CockpitClient, useCockpit);
  await settle();
  const original = useCockpit.getState().lastError;
  expect(original?.message).toBe('Original harness timeout');
  expect(useCockpit.getState().projects.map(project => project.id)).toEqual(['p']);
  catalogue.refreshProjects(); catalogue.refreshProjects();
  await settle();
  expect(client.listHarnesses).toHaveBeenCalledTimes(2);
  expect(client.listMachines).toHaveBeenCalledTimes(1);
  expect(useCockpit.getState().lastError).toBe(original);
  resolveHarnesses([]); await settle();
  expect(client.listHarnesses).toHaveBeenCalledTimes(2);
  expect(useCockpit.getState().lastError).toBeNull();
});

it.each(['account catalogue', 'workspace', 'service', 'error identity', 'project removal', 'stopped loader'] as const)('refuses a retained retry after %s changes', async change => {
  const client = fixture(); await settle(); const original = useCockpit.getState().lastError!;
  if (change === 'account catalogue') useCockpit.setState(state => ({ projectCatalogueEpoch: state.projectCatalogueEpoch + 1 }));
  if (change === 'workspace') selectTeamScope('');
  if (change === 'service') client.baseUrl = 'http://127.0.0.1:62001';
  if (change === 'error identity') useCockpit.getState().setError({ code: 'internal', message: 'A later independent failure' });
  if (change === 'project removal') useCockpit.setState({ projects: [] });
  if (change === 'stopped loader') catalogue!.stop();
  catalogue!.retryBranches(original); await settle();
  expect(client.listBranches).toHaveBeenCalledTimes(1);
});

it('discards an in-flight retry after workspace departure and does not clear a newer unrelated error', async () => {
  const client = fixture(); await settle(); const original = useCockpit.getState().lastError!;
  let resolve!: (branches: unknown[]) => void;
  client.listBranches.mockImplementation(() => new Promise(done => { resolve = done; }));
  catalogue!.retryBranches(original);
  const later = { code: 'internal', message: 'A different operation failed' };
  selectTeamScope(''); useCockpit.getState().setError(later);
  resolve([aBranch({ id: 'b', projectId: 'p' })]); await settle();
  expect(useCockpit.getState().branchesByProject.p).toBeUndefined();
  expect(useCockpit.getState().lastError).toBe(later);
});
