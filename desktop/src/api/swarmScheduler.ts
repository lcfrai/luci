import { productApi } from './product';
export interface SchedulerPolicy { maxActive: number; maxPerProject: number; headroomPercent: number; minHeadroomMiB: number; diskReserveMiB: number }
export interface SchedulerResources {
  status?: string;
  host?: { observedAt: string; status: string; platform: string; scope: string;
    memory: { effectiveTotalBytes: number | null; effectiveAvailableBytes: number | null; limitSource: string };
    cpu: { logicalProcessors: number | null; effectiveMillis: number | null; utilizationPercent: number | null };
    disks: { role: string; totalBytes: number | null; availableBytes: number | null }[];
    unavailable: { metric: string; reason: string }[];
  };
  execution?: { observedAt: string; status: string; scope: string; totalMemoryBytes: number | null; cpuMillis: number | null; unavailable: { metric: string; reason: string }[] };
  reservations?: { observedAt: string; count: number; memoryMiB: number; cpuMillis: number } | null;
}
export interface SwarmScheduler { revision: number; observedAt: string; dispatchPaused: boolean; policy: SchedulerPolicy; resources: SchedulerResources | null }
function checked(value: SwarmScheduler): SwarmScheduler {
  if (!value || !Number.isInteger(value.revision) || value.revision < 0 || typeof value.dispatchPaused !== 'boolean' || !value.policy ||
    !['maxActive', 'maxPerProject', 'headroomPercent', 'minHeadroomMiB', 'diskReserveMiB'].every(key => Number.isSafeInteger(value.policy[key as keyof SchedulerPolicy]))) {
    throw new Error('Workspace capacity could not be read. Refresh before changing it.');
  }
  return value;
}
export const swarmSchedulerApi = {
  async get(signal?: AbortSignal) {
    const value = await productApi.request<SwarmScheduler>('/v1/product/swarm-scheduler', undefined, 'GET', 30_000, signal);
    return checked(value);
  },
  async configure(snapshot: SwarmScheduler, policy: SchedulerPolicy, dispatchPaused: boolean) {
    return checked(await productApi.request<SwarmScheduler>('/v1/product/swarm-scheduler', {
      expectedRevision: snapshot.revision, policy, dispatchPaused,
    }, 'PUT'));
  },
};
