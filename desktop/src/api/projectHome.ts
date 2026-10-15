import { productApi } from './product';

export interface HomeStop { id: string; at: string; kind: 'stopped' | 'succeeded' | 'failed' | 'cancelled' | 'completed' }
export interface HomeProject {
  id: string; name: string; working: boolean; needsInput: boolean; queued: boolean; unknown: boolean;
  stops: HomeStop[]; newestObservedStop?: string;
}
export interface ProjectHome {
  version: 1; revision: number; pinnedProjectIds: string[]; projects: HomeProject[];
}
interface HomePage extends ProjectHome { observedAt: string; total: number; nextOffset: number | null }
const validID = (id: unknown): id is string => typeof id === 'string' && /^p-[a-zA-Z0-9_-]{1,128}$/.test(id);
const validStop = (value: HomeStop): boolean => !!value && /^stop-[a-f0-9]{64}$/.test(value.id) && Number.isFinite(Date.parse(value.at)) &&
  ['stopped', 'succeeded', 'failed', 'cancelled', 'completed'].includes(value.kind);
function validPage(page: HomePage, offset: number): boolean {
  if (!page || page.version !== 1 || !Number.isSafeInteger(page.revision) || page.revision < 1 ||
      !Number.isSafeInteger(page.total) || page.total < 0 || page.total > 4096 ||
      (page.nextOffset !== null && (!Number.isSafeInteger(page.nextOffset) || page.nextOffset <= offset || page.nextOffset > 4096)) ||
      !Array.isArray(page.projects) || page.projects.length > 64 || !Array.isArray(page.pinnedProjectIds) || page.pinnedProjectIds.length > 4096 ||
      !page.pinnedProjectIds.every(validID)) return false;
  const ids = new Set<string>();
  for (const project of page.projects) {
    if (!validID(project.id) || ids.has(project.id) || typeof project.name !== 'string' || project.name.length > 256 ||
      ['working', 'needsInput', 'queued', 'unknown'].some(key => typeof project[key as 'working'] !== 'boolean') ||
      !Array.isArray(project.stops) || project.stops.length > 32 || !project.stops.every(validStop) ||
      (project.newestObservedStop !== undefined && !project.stops.some(stop => stop.id === project.newestObservedStop))) return false;
    ids.add(project.id);
  }
  return true;
}
export const projectHomeApi = {
  async getAll(signal?: AbortSignal): Promise<ProjectHome> {
    const projects: HomeProject[] = [], seen = new Set<string>();
    let offset = 0, revision = 0, pins: string[] = [];
    for (let pageIndex = 0; pageIndex < 64; pageIndex++) {
      const page = await productApi.request<HomePage>(`/v1/product/home?offset=${offset}`, undefined, 'GET', 15_000, signal);
      if (!validPage(page, offset) || (revision && (revision !== page.revision || JSON.stringify(pins) !== JSON.stringify(page.pinnedProjectIds))))
        throw new Error('Project activity changed while loading. Refresh Home.');
      revision = page.revision; pins = page.pinnedProjectIds;
      for (const project of page.projects) {
        if (seen.has(project.id)) throw new Error('Project activity changed while loading. Refresh Home.');
        seen.add(project.id); projects.push(project);
      }
      if (page.nextOffset === null) return { version: 1, revision, pinnedProjectIds: pins, projects };
      offset = page.nextOffset;
    }
    throw new Error('Project activity exceeds the supported page limit.');
  },
  pin(projectId: string, pinned: boolean, expectedRevision: number) {
    return productApi.request<{revision: number; pinnedProjectIds: string[]}>(
      '/v1/product/home/pins', { projectId, pinned, expectedRevision }, 'PUT', 15_000);
  },
  acknowledge(projectId: string, observedStopIds: string[], expectedRevision: number) {
    return productApi.request<{revision: number; acknowledgedStopIds: string[]}>(
      '/v1/product/home/stops/ack', { projectId, observedStopIds, expectedRevision }, 'POST', 15_000);
  },
};
