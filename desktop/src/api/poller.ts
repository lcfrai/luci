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

export function startPolling(client: CockpitClient, store: typeof useCockpit): () => void {
  let stopped = false;
  let inFlight = false;

  const tick = async (): Promise<void> => {
    // Paused while hidden: a laptop lid closed for eight hours must not wake to 14 000 queued
    // requests. In-flight guard: a slow tick delays the next one, it never stacks another.
    if (stopped || inFlight || document.visibilityState === "hidden") return;
    inFlight = true;
    try {
      const sessions = await client.listSessions();
      if (stopped) return;
      const state = store.getState();
      const covered = coveredWorktreeIds(Object.keys(state.sessionIdsByWorktree), sessions);
      state.replaceSessions(sessions, covered);
      // Only written when it actually changes: a store write every 2 s is a re-render every
      // 2 s, which is the layout shift AGENTIC-UX §10 rule 7 bans.
      if (store.getState().lastError !== null) store.getState().setError(null);
    } catch (error) {
      if (stopped) return;
      store.getState().setError(
        error instanceof ApiError
          ? { code: error.code, message: error.message }
          : { code: "internal", message: String(error) },
      );
    } finally {
      inFlight = false;
    }
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
