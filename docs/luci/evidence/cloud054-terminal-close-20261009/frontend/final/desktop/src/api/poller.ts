// The one interval for the whole app (§4.7). Started when AppShell mounts (task 24), stopped
// when it unmounts. There is no event stream in the Go contract — §10.5 asks for one — so
// Phase 2 polls, from exactly one place, with exactly one request per tick.

import { ApiError } from "./client";
import type { CockpitClient } from "./client";
import type { useCockpit } from "../store/store";
import type { Session, WorktreeId } from "../store/types";

export const POLL_INTERVAL_MS = 2_000;

/** The worktrees this unfiltered fetch is authoritative for: every one the response mentions,
 *  plus every one the store already indexes. The second half is what makes a worktree whose
 *  last session was purged go EMPTY rather than keep a stale row. */
export function coveredWorktreeIds(
  indexed: readonly WorktreeId[],
  sessions: readonly Session[],
): WorktreeId[] {
  const seen = new Set<WorktreeId>(indexed);
  for (const session of sessions) seen.add(session.worktreeId);
  return [...seen];
}

/**
 * `onGoodTick` — ADDED BY TASK 24, a declared departure into this file — runs after every tick
 * whose fetch succeeded, and never after a failed, skipped or stopped one. A good tick is the
 * one proof that the control plane answers, so it is what retries a catalogue list that failed
 * or never arrived (App.tsx's `startCatalogue`), without a second timer: §4.7 allows one interval.
 */
export function startPolling(
  client: CockpitClient,
  store: typeof useCockpit,
  onGoodTick?: () => void,
): () => void {
  const epoch = store.getState().projectCatalogueEpoch;
  let stopped = false;
  let inFlight = false;
  /** The error object this poller last wrote. setError stores the object it is handed, so
   *  identity says whose report `lastError` currently is. */
  let raised: { code: string; message: string } | null = null;

  const tick = async (): Promise<void> => {
    // Paused while hidden: a laptop lid closed for eight hours must not wake to 14 000 queued
    // requests. In-flight guard: a slow tick delays the next one, it never stacks another.
    if (stopped || store.getState().projectCatalogueEpoch !== epoch || inFlight || document.visibilityState === "hidden") return;
    inFlight = true;
    let good = false;
    // Store rows are immutable. A completed mutation/removal since this request
    // began wins over its delayed response; opaque generation tokens are not clocks.
    const requestedSessions = store.getState().sessions;
    try {
      const sessions = await client.listSessions();
      if (stopped || store.getState().projectCatalogueEpoch !== epoch) return;
      good = true;
      const state = store.getState();
      const changed = new Set([...Object.keys(requestedSessions), ...Object.keys(state.sessions)]
        .filter(id => requestedSessions[id] !== state.sessions[id]));
      const accepted = sessions.filter(session => !changed.has(session.id));
      for (const id of changed) {
        const current = state.sessions[id];
        if (current) accepted.push(current);
      }
      // Fresh subsequent requests still remove records no longer visible. Scope
      // resets above and records removed during this request can never be revived.
      const covered = coveredWorktreeIds(Object.keys(state.sessionIdsByWorktree), accepted);
      state.replaceSessions(accepted, covered);
      // AMENDED BY TASK 24: a good tick clears the error THIS POLLER raised, and nothing else.
      // It used to clear any non-null lastError, so a catalogue list that failed while sessions
      // answered was reported and then wiped by the next tick, 2 s later, leaving an empty rail
      // and a clean footer. An error is cleared by the success of the operation that raised it;
      // the catalogue clears its own. Still written only when it changes: a store write every
      // 2 s is a re-render every 2 s, which is the layout shift AGENTIC-UX §10 rule 7 bans. That
      // is what `raised !== null` is for — with no error raised yet, null === null would write.
      // `raised` is not reset after a clear: the old object cannot match again, because setError
      // is handed each one exactly once, at the failure that built it.
      if (raised !== null && store.getState().lastError === raised) store.getState().setError(null);
    } catch (error) {
      if (stopped || store.getState().projectCatalogueEpoch !== epoch) return;
      raised =
        error instanceof ApiError
          ? { code: error.code, message: error.message }
          : { code: "internal", message: String(error) };
      store.getState().setError(raised);
    } finally {
      inFlight = false;
    }
    // Outside the try, so a throw from the hook is never recorded as the poller's own failure,
    // and after `finally`, so a retry the hook starts never sees this tick still in flight.
    // `good` already excludes a stopped tick: it is set only after the post-await `stopped`
    // check, and a stop during the fetch returns from inside the try, so control never reaches
    // this line. Between that check and here the code is synchronous, and a store write
    // schedules React's render rather than running it, so nothing in between can unmount App.
    // Were that ever to change, the hook App passes is a stopped catalogue's `retry`, which
    // starts nothing (App.tsx's `run`, pinned by defects.test.tsx).
    if (good) onGoodTick?.();
  };

  const onVisibility = (): void => {
    if (document.visibilityState === "visible") void tick();
  };

  void tick();   // the first render does not wait POLL_INTERVAL_MS for its data
  const handle = setInterval(() => void tick(), POLL_INTERVAL_MS);
  document.addEventListener("visibilitychange", onVisibility);

  return () => {
    stopped = true;
    clearInterval(handle);
    document.removeEventListener("visibilitychange", onVisibility);
  };
}
