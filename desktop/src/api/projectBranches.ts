import { productApi } from "./product";
import type { Branch, Worktree } from "../store/types";

export interface BranchState extends Branch { commit: string; checkedOut: boolean; canSwitch: boolean; message: string }
export interface BranchOperation {
  id: string; projectId: string; action: "create" | "switch";
  status: "running" | "succeeded" | "failed" | "uncertain";
  branchId: string; branch: string; sourceBranchId: string; sourceCommit: string;
  targetCommit: string; commit?: string; message: string; createdAt: string; updatedAt: string;
}
export interface BranchCatalogue {
  projectId: string; currentBranchId: string; currentBranch: string; commit: string;
  clean: boolean; canCreate: boolean; canSwitch: boolean; message: string;
  branches: BranchState[]; operation?: BranchOperation;
}
export interface BranchResult { operation: BranchOperation; branch?: Branch; worktree?: Worktree; commit?: string }
export interface BranchMutation { expectedBranchId: string; expectedCommit: string; operationId: string }
const base = (id: string) => `/v1/product/projects/${encodeURIComponent(id)}/git`;
export const projectBranchesApi = {
  catalogue: (id: string) => productApi.request<BranchCatalogue>(`${base(id)}/branches`),
  create: (id: string, input: BranchMutation & { name: string }) => productApi.request<BranchResult>(`${base(id)}/branches`, input),
  switch: (id: string, input: BranchMutation & { branchId: string; expectedTargetCommit: string }) => productApi.request<BranchResult>(`${base(id)}/switch`, input),
  operation: (id: string, operationId: string) => productApi.request<BranchResult>(`${base(id)}/branch-operations/${encodeURIComponent(operationId)}`),
};
