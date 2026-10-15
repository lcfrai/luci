import { useSyncExternalStore } from 'react';
export interface DesignWorkStatus { jobId: string; phase: 'working' | 'waiting' | 'unknown' | 'response-ready' | 'failed' | 'stopped'; updatedAt: string; observedAt: number }
const observations = new Map<string, {owner: symbol; value: DesignWorkStatus | null}>();
const listeners = new Set<() => void>();
const notify = () => listeners.forEach(listener => listener());
export const designWorkScope = (scope: string | null, actorId: string, teamId: string, projectId: string) => scope && actorId ? JSON.stringify([scope, actorId, teamId, projectId]) : null;
/** Only the existing conversation job observer publishes. No new reads or timers. */
export function ownDesignWorkStatus(key: string) {
  const owner = Symbol(); observations.set(key, {owner, value: null}); notify();
  return {
    publish(value: DesignWorkStatus | null) { if (observations.get(key)?.owner === owner) { observations.set(key, {owner, value}); notify(); } },
    close() { if (observations.get(key)?.owner === owner) { observations.delete(key); notify(); } },
  };
}
export function readDesignWorkStatus(key: string | null): DesignWorkStatus | null { return key ? observations.get(key)?.value ?? null : null; }
export function useDesignWorkStatus(key: string | null) {
  return useSyncExternalStore(listener => { listeners.add(listener); return () => { listeners.delete(listener); }; }, () => readDesignWorkStatus(key), () => null);
}
export function observedDesignWorkPhase(status: string): DesignWorkStatus['phase'] {
  if (status === 'running') return 'working';
  if (status === 'queued' || status === 'reserved') return 'waiting';
  if (status === 'succeeded') return 'response-ready';
  if (status === 'failed') return 'failed';
  if (status === 'cancelled') return 'stopped';
  return 'unknown';
}
