import { productApi, type ProductClient, type SetupStatus } from './product';

export interface ExecutionStorage {
  platform: string;
  scope: 'local-docker-desktop' | 'unsupported' | 'not-required';
  required: boolean;
  state: 'not-required' | 'confirmation-required' | 'pending-observation' | 'confirmed' | 'invalidated' | 'unavailable';
  revision: number;
  reasonCode: string;
  message: string;
  basis: string;
  observedAt?: string;
  confirmedAt?: string;
  vhdPath?: string;
  desktopVersion?: string;
  mountedVolumeCount?: number;
  limitingVolume?: string;
  hostAvailableBytes?: number;
  guestAvailableBytes?: number;
}
function checked(value: ExecutionStorage): ExecutionStorage {
  const optionalText = ['observedAt', 'confirmedAt', 'vhdPath', 'desktopVersion', 'limitingVolume'] as const;
  const optionalCount = ['mountedVolumeCount', 'hostAvailableBytes', 'guestAvailableBytes'] as const;
  if (!value || typeof value.platform !== 'string' || typeof value.required !== 'boolean' ||
    !['local-docker-desktop', 'unsupported', 'not-required'].includes(value.scope) ||
    !['not-required', 'confirmation-required', 'pending-observation', 'confirmed', 'invalidated', 'unavailable'].includes(value.state) ||
    !Number.isSafeInteger(value.revision) || value.revision < 0 ||
    [value.reasonCode, value.message, value.basis].some(field => typeof field !== 'string') ||
    optionalText.some(key => value[key] !== undefined && typeof value[key] !== 'string') ||
    optionalCount.some(key => value[key] !== undefined && (!Number.isSafeInteger(value[key]) || value[key]! < 0))) {
    throw new Error('Docker storage status could not be read. Refresh it before confirming a location.');
  }
  return value;
}
/** This management endpoint always addresses the authenticated control-plane host. */
export function createExecutionStorageClient(client: Pick<ProductClient, 'request'> = productApi) {
  const path = '/v1/admin/execution-storage';
  return {
    async get(signal?: AbortSignal) {
      return checked(await client.request<ExecutionStorage>(path, undefined, 'GET', 30_000, signal));
    },
    async confirm(snapshot: ExecutionStorage, vhdPath: string) {
      return checked(await client.request<ExecutionStorage>(path, {
        expectedRevision: snapshot.revision, vhdPath, confirmActiveAndPendingLocal: true,
      }, 'POST'));
    },
  };
}
export type ExecutionStorageClient = ReturnType<typeof createExecutionStorageClient>;

/** Readiness advice mirrors the backend; it never authorizes a Docker mutation. */
export function storagePreparationBlock(status: Pick<SetupStatus, 'checks' | 'platform' | 'workspace'> | null | undefined): string {
  const check = status?.checks.find(value => value.id === 'docker-backing-storage');
  if (check) return check.status === 'ready' ? '' : check.message || 'Review Docker storage in Settings before preparing the runtime.';
  return status?.platform === 'windows' && status.workspace?.mode !== 'cloud' ? 'Check Docker storage readiness before preparing the runtime. A host Super Admin can review it in Settings.' : '';
}
