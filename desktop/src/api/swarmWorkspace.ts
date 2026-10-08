import { productApi } from './product';
import type { RunOperation } from './projectRuns';

export interface SwarmWorkspace {
  operationId: string; taskId: string; worktreeId: string; exists: boolean;
  files: {path: string; status: string}[]; filesOmitted: number; filesTruncated: boolean;
  patch: string; patchTruncated: boolean; reason: string;
}
export interface SwarmWorkspaceFile {
  operationId: string; taskId: string; worktreeId: string; path: string;
  bytes: number; content: string; binary: boolean; omitted: boolean; reason: string;
}
const route = (project: string, run: string, operation: RunOperation) => `/v1/product/projects/${encodeURIComponent(project)}/runs/${encodeURIComponent(run)}/swarm/operations/${encodeURIComponent(operation.id)}/workspace`;
const plainRecord = (value: unknown): value is Record<string, unknown> => !!value && typeof value === 'object' && !Array.isArray(value);
const text = (value: unknown, limit: number): value is string => typeof value === 'string' && value.length <= limit;
const count = (value: unknown): value is number => typeof value === 'number' && Number.isSafeInteger(value) && value >= 0;
const relativePath = (value: unknown): value is string => text(value, 4096) && !!value && !/[\\:\x00-\x1f\x7f]/.test(value) && !value.split('/').some(part => part === '' || part === '.' || part === '..');
function matches(value: unknown, operation: RunOperation): value is Record<string, unknown> {
  return plainRecord(value) && !!operation.taskId && !!operation.worktreeId && value.operationId === operation.id && value.taskId === operation.taskId && value.worktreeId === operation.worktreeId;
}
const mismatch = () => new Error('The workspace response does not match this task attempt or its supported preview format. Close this view and refresh the build before inspecting it again.');

/** GET-only inspection of the retained attempt. These reads never authorize integration. */
export const swarmWorkspaceApi = {
  async inspect(project: string, run: string, operation: RunOperation, signal?: AbortSignal): Promise<SwarmWorkspace> {
    const result = await productApi.request<{workspace: unknown}>(route(project, run, operation), undefined, 'GET', 30_000, signal);
    const value = result?.workspace;
    if (!matches(value, operation) || typeof value.exists !== 'boolean' || !Array.isArray(value.files) || value.files.length > 2000 ||
      value.files.some(file => !plainRecord(file) || !relativePath(file.path) || !text(file.status, 64)) ||
      new Set(value.files.map(file => file.path)).size !== value.files.length || !count(value.filesOmitted) || typeof value.filesTruncated !== 'boolean' ||
      !text(value.patch, 128 * 1024) || typeof value.patchTruncated !== 'boolean' || !text(value.reason, 4096)) throw mismatch();
    return value as unknown as SwarmWorkspace;
  },
  async file(project: string, run: string, operation: RunOperation, path: string, signal?: AbortSignal): Promise<SwarmWorkspaceFile> {
    if (!relativePath(path)) throw new Error('Choose a project-relative file from this task workspace.');
    const result = await productApi.request<{file: unknown}>(`${route(project, run, operation)}/file?path=${encodeURIComponent(path)}`, undefined, 'GET', 30_000, signal);
    const value = result?.file;
    if (!matches(value, operation) || value.path !== path || !count(value.bytes) || !text(value.content, 128 * 1024) || typeof value.binary !== 'boolean' || typeof value.omitted !== 'boolean' || !text(value.reason, 4096) || (value.binary && !!value.content)) throw mismatch();
    return value as unknown as SwarmWorkspaceFile;
  },
};
