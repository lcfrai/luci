import { productApi } from "./product";

export type CoordinatorTarget = "specification" | "design-plan" | "plan";
export interface CoordinatorPolicy {
  projectId: string; revision: number; mode: "review" | "auto-drafts";
  reviewMode: "manual" | "approved-amendments";
  allowedTargets: CoordinatorTarget[];
  executionMode: "manual" | "reviewed-tasks"; integrationMode?: "manual" | "reviewed-scopes"; deploymentMode: "manual" | "existing-target"; previewMode?: "manual" | "checked-source" | "reviewed-source";
  updatedAt?: string; actor?: { id: string; displayName: string };
}
export const coordinatorTargets: CoordinatorTarget[] = ["specification", "design-plan", "plan"];
const route = (id: string) => `/v1/product/projects/${encodeURIComponent(id)}/orchestrator/policy`;
function verified(value: CoordinatorPolicy, projectId: string) {
  if (value?.projectId !== projectId || !Number.isSafeInteger(value.revision) || value.revision < 0 ||
      !["review", "auto-drafts"].includes(value.mode) || !["manual", "approved-amendments"].includes(value.reviewMode) || (value.reviewMode === "approved-amendments" && value.mode !== "auto-drafts") || !Array.isArray(value.allowedTargets) ||
      !["manual", "reviewed-tasks"].includes(value.executionMode) || (value.integrationMode !== undefined && !["manual", "reviewed-scopes"].includes(value.integrationMode)) || !["manual", "existing-target"].includes(value.deploymentMode) || (value.previewMode !== undefined && !["manual", "checked-source", "reviewed-source"].includes(value.previewMode)) ||
      value.allowedTargets.some(target => !coordinatorTargets.includes(target)) ||
      new Set(value.allowedTargets).size !== value.allowedTargets.length ||
      (value.mode === "auto-drafts" && (!value.revision || !value.allowedTargets.length)) ||
      (value.revision > 0 && (!value.actor?.id || !value.updatedAt || !Number.isFinite(Date.parse(value.updatedAt))))) {
    throw new Error("The coordinator policy could not be verified for this project. Refresh before changing it.");
  }
  return value;
}
export const coordinatorPolicyApi = {
  async get(projectId: string, signal?: AbortSignal) {
    return verified(await productApi.request<CoordinatorPolicy>(route(projectId), undefined, "GET", 120_000, signal), projectId);
  },
  async save(projectId: string, input: { expectedRevision: number; mode: CoordinatorPolicy["mode"]; reviewMode?: CoordinatorPolicy["reviewMode"]; allowedTargets: CoordinatorTarget[]; executionMode?: CoordinatorPolicy["executionMode"]; integrationMode?: CoordinatorPolicy["integrationMode"]; previewMode?: CoordinatorPolicy["previewMode"]; deploymentMode?: CoordinatorPolicy["deploymentMode"] }) {
    const value = verified(await productApi.request<CoordinatorPolicy>(route(projectId), input), projectId);
    if (value.revision !== input.expectedRevision + 1 || value.mode !== input.mode ||
        value.allowedTargets.join(",") !== input.allowedTargets.join(",") || (input.reviewMode !== undefined && value.reviewMode !== input.reviewMode) || (input.executionMode !== undefined && value.executionMode !== input.executionMode) || (input.integrationMode !== undefined && value.integrationMode !== input.integrationMode) || (input.previewMode !== undefined && value.previewMode !== input.previewMode) || (input.deploymentMode !== undefined && value.deploymentMode !== input.deploymentMode)) {
      throw new Error("The saved policy response did not match this change. Refresh to observe its outcome before changing it again.");
    }
    return value;
  },
};
