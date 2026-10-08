import { productApi } from './product';
export type QueuePriority = 'low' | 'normal' | 'high';
export type QueueState = 'queued' | 'paused' | 'reserved' | 'running' | 'blocked' | 'unknown' | 'completed' | 'failed' | 'cancelled';
export interface SwarmQueueItem {
  id: string; revision: number; projectId: string; runId: string; taskId: string; taskTitle: string;
  priority: QueuePriority; state: QueueState; queuedAt: string; updatedAt: string;
  reasonCode?: string; reason?: string; operationId?: string; childId?: string; memoryMiB?: number; cpuMillis?: number;
}
export interface ProjectSwarmQueue {
  revision: number; observedAt: string; dispatchPaused: boolean;
  counts: Record<QueueState, number>; items: SwarmQueueItem[]; hasMore: boolean;
}
const states: QueueState[] = ['queued', 'paused', 'reserved', 'running', 'blocked', 'unknown', 'completed', 'failed', 'cancelled'];
const path = (project: string) => `/v1/product/projects/${encodeURIComponent(project)}`;
function ownedItem(project: string, item: SwarmQueueItem): SwarmQueueItem {
  if (!item || item.projectId !== project || typeof item.id !== 'string' || !item.id ||
      !Number.isInteger(item.revision) || item.revision < 1 || typeof item.runId !== 'string' || !item.runId || typeof item.taskId !== 'string' || !item.taskId ||
      typeof item.taskTitle !== 'string' || !states.includes(item.state) || !['low', 'normal', 'high'].includes(item.priority) ||
      typeof item.queuedAt !== 'string' || typeof item.updatedAt !== 'string') {
    throw new Error('The queued task does not match this project. Refresh its queue before continuing.');
  }
  return item;
}
function owned(project: string, value: ProjectSwarmQueue) {
  if (!value || !Array.isArray(value.items) || typeof value.dispatchPaused !== 'boolean' || typeof value.hasMore !== 'boolean' ||
      !Number.isInteger(value.revision) || typeof value.observedAt !== 'string' ||
      !value.counts || states.some(state => !Number.isInteger(value.counts[state]) || value.counts[state] < 0)) {
    throw new Error('The project queue could not be read. Refresh before continuing.');
  }
  value.items.forEach(item => ownedItem(project, item));
  return value;
}
export const swarmQueueApi = {
  async get(project: string, signal?: AbortSignal) {
    // Product transport owns workspace generation and account authentication.
    return owned(project, await productApi.request<ProjectSwarmQueue>(`${path(project)}/swarm-queue`, undefined, "GET", 30_000, signal));
  },
  async enqueue(project: string, run: string, task: string, expectedRevision: number, idempotencyKey: string, priority: QueuePriority = 'normal') {
    const value = await productApi.request<{ item: SwarmQueueItem; queue: ProjectSwarmQueue }>(`${path(project)}/runs/${encodeURIComponent(run)}/swarm/tasks/${encodeURIComponent(task)}/queue`, { expectedRevision, idempotencyKey, priority });
    const item = ownedItem(project, value.item);
    if (item.runId !== run || item.taskId !== task) throw new Error('The queue receipt does not match the requested task. Inspect the project queue before trying again.');
    return { item, queue: owned(project, value.queue) };
  },
  async enqueueMany(project: string, run: string, taskIds: string[], expectedRevision: number, idempotencyKey: string, priority: QueuePriority = 'normal') {
    if (!taskIds.length || new Set(taskIds).size !== taskIds.length) throw new Error('Choose distinct reviewed tasks to queue.');
    const value = await productApi.request<{ items: SwarmQueueItem[]; queue: ProjectSwarmQueue }>(`${path(project)}/runs/${encodeURIComponent(run)}/swarm/queue`, { expectedRevision, idempotencyKey, priority, taskIds });
    if (!Array.isArray(value.items) || value.items.length !== taskIds.length || new Set(value.items.map(item => item.taskId)).size !== taskIds.length) throw new Error('The batch queue receipt is incomplete. Refresh the queue before retrying.');
    value.items.forEach(item => { ownedItem(project, item); if (item.runId !== run || !taskIds.includes(item.taskId)) throw new Error('The queued batch does not match the reviewed tasks. Refresh before continuing.'); });
    return { items: value.items, queue: owned(project, value.queue) };
  },
  async control(project: string, item: SwarmQueueItem, action: 'pause' | 'resume' | 'cancel' | 'priority', priority?: QueuePriority) {
    ownedItem(project, item);
    if (item.state === 'failed') throw new Error('This queue entry failed. Inspect its saved run before reviewing a new attempt; failed work cannot be resumed or reprioritized.');
    const value = await productApi.request<{ item: SwarmQueueItem; queue: ProjectSwarmQueue }>(`${path(project)}/swarm-queue/${encodeURIComponent(item.id)}/${action}`, { expectedRevision: item.revision, ...(action === 'priority' ? { priority } : {}) });
    const receipt = ownedItem(project, value.item);
    if (receipt.id !== item.id || receipt.runId !== item.runId || receipt.taskId !== item.taskId) throw new Error('The queue action receipt does not match this task. Refresh the queue before continuing.');
    return owned(project, value.queue);
  },
};
