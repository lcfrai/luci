import { productApi, type Brief, type Design } from "./product";
import type { ProjectSpecification } from "./definition";
import type { DesignPlan } from "./designPlan";
export interface ProjectContext {
  contextId: string; projectId: string; imported: boolean; expectedDigest: string;
  exports: { path: string; status: "current" | "missing" | "changed" | "unavailable" }[];
  archiveHead: string; archiveChunks: number; archiveBytes: number; archiveRevisions: number;
  unusedArchiveChunks: number; unusedArchiveBytes: number; archiveError?: string; archiveBytesLimit: number; archiveChunksLimit: number; recentRevisions: number;
}
export interface ContextArchivePage {
  digest: string; nextCursor: string;
  chunk: null | { version: 1; contextId: string; projectId: string; previous?: string;
    briefRevisions?: Brief[]; designRevisions?: Design[];
    specificationRevisions?: ProjectSpecification[]; designPlanRevisions?: DesignPlan[] };
}
const route = (projectId: string) => `/v1/product/projects/${encodeURIComponent(projectId)}/context`;
export const projectContextApi = {
  get: (projectId: string) => productApi.request<ProjectContext>(route(projectId)),
  repair: (projectId: string, expectedDigest: string) => productApi.request<ProjectContext>(`${route(projectId)}/exports/repair`, { expectedDigest }),
  archive: (projectId: string, cursor: string) => productApi.request<ContextArchivePage>(`${route(projectId)}/archive?cursor=${encodeURIComponent(cursor)}`),
  prune: (projectId: string, expectedDigest: string) => productApi.request<ProjectContext>(`${route(projectId)}/archive/prune`, { expectedDigest, acknowledgeArchivedHistoryRemoval: true }),
};
