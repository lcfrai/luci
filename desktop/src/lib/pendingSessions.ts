// desktop/src/lib/pendingSessions.ts
//
// The ids of sessions whose DEFERRED mutation is still inside its Undo window — the store has
// been patched, and the request has not been sent yet. Written by task 22's Kill and Forget,
// read by the rail (for the pending affordance) and by `dataSlice.replaceSessions` (to stop the
// 2 s poller undoing the patch before the window closes).
//
// WHY THIS IS ITS OWN MODULE, and it is a dependency question rather than a taste one. The
// registry began inside `components/develop/sessionVerbs.ts`, which imports the store. The
// poller suppression has to read it from `store/slices/dataSlice.ts`, and a store slice
// importing a component module that imports the store is a runtime import cycle — the class of
// break that surfaces in a bundler, later, naming neither file. A leaf with NO imports of its
// own cannot participate in a cycle no matter who reads it.
//
// It is deliberately not store state. This is a fact that lives for six seconds, never
// persists, and would otherwise put a transient into `partialize`'s blast radius.

import type { SessionId } from "../store/types";

const pending = new Set<SessionId>();
const listeners = new Set<() => void>();

export function isSessionPending(id: SessionId): boolean {
  return pending.has(id);
}

/** A snapshot, not the live Set: a caller that mutated it would silently break every reader. */
export function pendingSessionIds(): SessionId[] {
  return [...pending];
}

export function setSessionPending(id: SessionId, value: boolean): void {
  // Only notify on an actual change. `runDeferred` clears twice on the failure path — once
  // before the rollback, because the rollback's own store write must not be suppressed, and
  // again in its `finally` — and a listener fired for a no-op re-renders every rail row that
  // subscribes, on a path that changed nothing.
  const had = pending.has(id);
  if (had === value) return;
  if (value) pending.add(id);
  else pending.delete(id);
  for (const listener of listeners) listener();
}

/** For `useSyncExternalStore`. The hook itself stays in task 22's module so this file imports
 *  no React and stays a leaf. */
export function subscribePending(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

/** Test seam. vitest's restoreMocks does not clear module state, and a leaked pending id makes
 *  a later test fail in a file that did nothing wrong. */
export function __resetPendingSessions(): void {
  pending.clear();
  for (const listener of listeners) listener();
}
