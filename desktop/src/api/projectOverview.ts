import { productApi } from './product';

export interface ProjectOverview {
  projectId: string; observedAt: string; status: 'ready' | 'partial' | 'unavailable';
  goal: { text: string; revision: number; updatedAt: string; truncated: boolean } | null;
  phase: { id: string; label: string };
  context: { briefRevision: number; specificationRevision: number; definitionStatus: string; designRevision: number; designStatus: string; planRevision: number; tasks: { total: number; recordedDone: number } } | null;
  questions: { status: 'recorded' | 'partial' | 'unavailable'; count: number | null; items: { id: string; source: 'definition' | 'orchestrator'; text: string; truncated: boolean }[]; omitted: number | null };
  execution: { scope: 'swarm-queue'; status: 'recorded' | 'unavailable'; observedAt: string; queued: number | null; paused: number | null; reserved: number | null; running: number | null; blocked: number | null; unknown: number | null; dispatchPaused: boolean | null };
  nextAction: { id: string; label: string; lens: string; runId?: string; reason?: string };
  latestRun?: { status: 'recorded' | 'none' | 'unavailable'; id?: string; state?: string; updatedAt?: string; contextStatus?: 'current' | 'changed' | 'unavailable'; contextReason?: string };
  limitations: string[];
}
const count = (value: unknown) => value === null || (Number.isSafeInteger(value) && (value as number) >= 0);
export const projectOverviewApi = {
  async get(projectId: string, signal?: AbortSignal): Promise<ProjectOverview> {
    const value = await productApi.request<ProjectOverview>(`/v1/product/projects/${encodeURIComponent(projectId)}/overview`, undefined, 'GET', 30_000, signal);
    if (!value || value.projectId !== projectId || !['ready', 'partial', 'unavailable'].includes(value.status) ||
      typeof value.observedAt !== 'string' || typeof value.phase?.label !== 'string' || typeof value.nextAction?.label !== 'string' ||
      (value.goal !== null && (typeof value.goal?.text !== 'string' || typeof value.goal.truncated !== 'boolean')) ||
      (value.context !== null && (!Number.isSafeInteger(value.context?.tasks?.total) || !Number.isSafeInteger(value.context?.tasks?.recordedDone))) ||
      !value.questions || !count(value.questions.count) || !count(value.questions.omitted) || !Array.isArray(value.questions.items) ||
      value.questions.items.some(item => typeof item.text !== 'string' || typeof item.id !== 'string') ||
      !value.execution || value.execution.scope !== 'swarm-queue' || !['recorded', 'unavailable'].includes(value.execution.status) ||
      ['queued', 'paused', 'reserved', 'running', 'blocked', 'unknown'].some(key => !count(value.execution[key as 'queued'])) ||
      !Array.isArray(value.limitations) || value.limitations.some(item => typeof item !== 'string')) {
      throw new Error('This project summary could not be read. Refresh it before continuing.');
    }
    return value;
  },
};
