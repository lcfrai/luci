import { productApi } from "./product";
import { componentLibraryErrors, type DesignComponentLibrary } from './designComponents';
import { designSceneErrors, type EditableDesignScene, type DesignRasterPin } from './designScene';
export interface DesignScreen { id: string; name: string; route: string; purpose: string; states: string[]; widths: number[]; requirementIds: string[]; accessibility: string }
export interface DesignPlan {
  controlLibraryRevision?:number;controlLibraryDigest?:string;
  currentControlLibraryRevision?:number;currentControlLibraryDigest?:string;
  projectId: string; revision: number; briefRevision: number; specificationRevision: number; sketchRevision: number;
  scene?: EditableDesignScene;
  componentLibrary?: DesignComponentLibrary;
  screens: DesignScreen[]; tokens: { name: string; value: string; purpose: string }[];
  assets: { id: string; path: string; purpose: string; licence: string; content?: DesignRasterPin }[];
  implementationNotes: string; prototypeReleaseId: string; prototypeCommit: string;
  status: "draft" | "approved" | "outdated"; digest: string; updatedAt: string;
}
export interface DesignHistoryResponse { revisions: DesignPlan[]; archiveHead?: string }
function validDesignHistory(value: unknown, projectId: string): value is DesignHistoryResponse {
  const record = (item: unknown): item is Record<string, unknown> => !!item && typeof item === 'object' && !Array.isArray(item);
  const text = (item: unknown, max: number): item is string => typeof item === 'string' && item.length <= max;
  const revision = (item: unknown): item is number => Number.isSafeInteger(item) && (item as number) >= 0;
  if (!record(value) || !Array.isArray(value.revisions) || value.revisions.length > 100 ||
      value.archiveHead !== undefined && !text(value.archiveHead, 256) || new TextEncoder().encode(JSON.stringify(value)).length > 8 * 1024 * 1024) return false;
  const seen = new Set<number>();
  return value.revisions.every(row => {
    if (!record(row) || row.projectId !== projectId || !revision(row.revision) || row.revision === 0 || seen.has(row.revision) ||
        !revision(row.briefRevision) || !revision(row.specificationRevision) || !revision(row.sketchRevision) ||
        !text(row.status, 20) || !['draft', 'approved', 'outdated'].includes(row.status) ||
        !text(row.digest, 64) || row.digest !== '' && !/^[a-f0-9]{64}$/.test(row.digest) || !text(row.updatedAt, 64) || !Number.isFinite(Date.parse(row.updatedAt)) ||
        !Array.isArray(row.screens) || row.screens.length > 100 || !Array.isArray(row.tokens) || row.tokens.length > 200 || !Array.isArray(row.assets) || row.assets.length > 100) return false;
    if (!row.screens.every(screen => record(screen) && text(screen.id, 80) && text(screen.name, 200) && Array.isArray(screen.states) && screen.states.length <= 30 && screen.states.every(state => text(state, 2000)) && Array.isArray(screen.widths) && screen.widths.length <= 12 && screen.widths.every(width => revision(width) && width >= 320 && width <= 3840))) return false;
    if (row.scene !== undefined && designSceneErrors(row.scene).length) return false;
    if (componentLibraryErrors(row.componentLibrary, row.tokens as DesignPlan['tokens']).length) return false;
    seen.add(row.revision); return true;
  });
}
const path = (id: string) => `/v1/product/projects/${encodeURIComponent(id)}/design-plan`;
export const designPlanApi = {
  get: (id: string) => productApi.request<DesignPlan>(path(id)),
  save: (id: string, plan: DesignPlan, expectedRevision: number, expectedBriefRevision: number, expectedSpecificationRevision: number) => productApi.request<DesignPlan>(path(id), { plan, expectedRevision, expectedBriefRevision, expectedSpecificationRevision, expectedControlLibraryRevision: plan.currentControlLibraryRevision ?? 0, expectedControlLibraryDigest: plan.currentControlLibraryDigest ?? "" }),
  approve: (id: string, revision: number) => productApi.request<DesignPlan>(`${path(id)}/approve`, { revision }),
  history: async (id: string, signal?: AbortSignal): Promise<DesignHistoryResponse> => {
    const result: unknown = await productApi.request(`${path(id)}/history`, undefined, 'GET', 30_000, signal);
    if (!validDesignHistory(result, id)) throw new Error('Saved design history could not be confirmed for this project. Your current draft is unchanged.');
    return result;
  },
};
