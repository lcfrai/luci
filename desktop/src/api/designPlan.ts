import { productApi } from "./product";
import type { EditableDesignScene, DesignRasterPin } from './designScene';
export interface DesignScreen { id: string; name: string; route: string; purpose: string; states: string[]; widths: number[]; requirementIds: string[]; accessibility: string }
export interface DesignPlan {
  projectId: string; revision: number; briefRevision: number; specificationRevision: number; sketchRevision: number;
  scene?: EditableDesignScene;
  screens: DesignScreen[]; tokens: { name: string; value: string; purpose: string }[];
  assets: { id: string; path: string; purpose: string; licence: string; content?: DesignRasterPin }[];
  implementationNotes: string; prototypeReleaseId: string; prototypeCommit: string;
  status: "draft" | "approved" | "outdated"; digest: string; updatedAt: string;
}
const path = (id: string) => `/v1/product/projects/${encodeURIComponent(id)}/design-plan`;
export const designPlanApi = {
  get: (id: string) => productApi.request<DesignPlan>(path(id)),
  save: (id: string, plan: DesignPlan, expectedRevision: number, expectedBriefRevision: number, expectedSpecificationRevision: number) => productApi.request<DesignPlan>(path(id), { plan, expectedRevision, expectedBriefRevision, expectedSpecificationRevision }),
  approve: (id: string, revision: number) => productApi.request<DesignPlan>(`${path(id)}/approve`, { revision }),
  history: (id: string) => productApi.request<{ revisions: DesignPlan[] }>(`${path(id)}/history`),
};
