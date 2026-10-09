import { teamsApi, type ProjectAccess } from '../../api/teams';
import { productApi } from '../../api/product';
import { teamScopeGeneration } from '../../api/teamScope';

interface PendingRead {
  controller: AbortController;
  promise: Promise<ProjectAccess>;
  subscribers: number;
  generation: number;
}

/** Share only overlapping display reads within one provider. Settled grants are
 * never cached; each hook still owns its previous read projection and write gate. */
export function createProjectPermissionReads() {
  const pending = new Map<string, PendingRead>();
  let generation = 0;
  return {
    invalidate() {
      generation++;
      for (const read of pending.values()) read.controller.abort();
      pending.clear();
    },
    acquire(teamId: string, projectId: string, revision: number) {
      const scope = teamScopeGeneration();
      const key = JSON.stringify([productApi.baseUrl, scope, teamId, projectId, revision, generation]);
      let read = pending.get(key);
      if (!read) {
        const controller = new AbortController();
        const promise = teamsApi.projectAccess(teamId, projectId, controller.signal);
        read = { controller, promise, subscribers: 0, generation };
        const created = read;
        pending.set(key, created);
        // Register both settlement paths without leaving a rejected cleanup promise.
        const settled = () => { if (pending.get(key) === created) pending.delete(key); };
        void promise.then(settled, settled);
      }
      const subscribed = read;
      subscribed.subscribers++;
      let released = false;
      return {
        promise: subscribed.promise,
        current: () => !released && !subscribed.controller.signal.aborted && subscribed.generation === generation && scope === teamScopeGeneration(),
        release() {
          if (released) return;
          released = true;
          if (--subscribed.subscribers === 0) {
            if (pending.get(key) === subscribed) pending.delete(key);
            subscribed.controller.abort();
          }
        },
      };
    },
  };
}
