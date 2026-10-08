import { productApi } from './product';
export interface SchedulerPolicy { maxActive: number; maxPerProject: number; headroomPercent: number; minHeadroomMiB: number; diskReserveMiB: number }
export type SchedulerReservations = { status: 'ready'; observedAt: string; count: number; memoryMiB: number; cpuMillis: number } | { status: 'unavailable' };
export interface SchedulerResources {
  status?: string;
  host?: { observedAt: string; status: string; platform: string; scope: string;
    memory: { effectiveTotalBytes: number | null; effectiveAvailableBytes: number | null; limitSource: string };
    cpu: { logicalProcessors: number | null; effectiveMillis: number | null; utilizationPercent: number | null };
    disks: { role: string; totalBytes: number | null; availableBytes: number | null }[];
    unavailable: { metric: string; reason: string }[];
  };
  execution?: { observedAt: string; status: string; scope: string; totalMemoryBytes: number | null; cpuMillis: number | null; unavailable: { metric: string; reason: string }[] };
  reservations?: SchedulerReservations | null;
}
export interface SwarmScheduler { revision: number; observedAt: string; dispatchPaused: boolean; policy: SchedulerPolicy; resources: SchedulerResources | null }
function checkedReservations(value: unknown): SchedulerReservations {
  const unavailable: SchedulerReservations = { status: 'unavailable' };
  if (value == null) return unavailable;
  if (typeof value !== 'object' || Array.isArray(value)) throw new Error('Workspace reservations could not be read.');
  const row = value as Record<string, unknown>;
  // Older servers returned an instance-local cache without a freshness status.
  if (!Object.prototype.hasOwnProperty.call(row, 'status')) return unavailable;
  if (row.status === 'unavailable' && !['observedAt', 'count', 'memoryMiB', 'cpuMillis'].some(key => Object.prototype.hasOwnProperty.call(row, key))) return unavailable;
  if (row.status !== 'ready' || typeof row.observedAt !== 'string' ||
    !/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?(?:Z|[+-]\d{2}:\d{2})$/.test(row.observedAt) || !Number.isFinite(Date.parse(row.observedAt)) ||
    !['count', 'memoryMiB', 'cpuMillis'].every(key => typeof row[key] === 'number' && Number.isSafeInteger(row[key]) && (row[key] as number) >= 0)) {
    throw new Error('Workspace reservations could not be read.');
  }
  return { status: 'ready', observedAt: row.observedAt, count: row.count as number, memoryMiB: row.memoryMiB as number, cpuMillis: row.cpuMillis as number };
}
function checked(value: SwarmScheduler): SwarmScheduler {
  if (!value || !Number.isInteger(value.revision) || value.revision < 0 || typeof value.dispatchPaused !== 'boolean' || !value.policy ||
    !['maxActive', 'maxPerProject', 'headroomPercent', 'minHeadroomMiB', 'diskReserveMiB'].every(key => Number.isSafeInteger(value.policy[key as keyof SchedulerPolicy]))) {
    throw new Error('Workspace capacity could not be read. Refresh before changing it.');
  }
  return value.resources ? { ...value, resources: { ...value.resources, reservations: checkedReservations(value.resources.reservations) } } : value;
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
