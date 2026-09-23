// Contract §9 (task 24) and §4.7. This file renders nothing of its own: it mounts
// <AppShell/>, it starts the one poller, and it owns every catalogue fetch.
//
// The catalogue is here because nothing else fetches it. Every other api.* call site in the
// tree is a create (hooks/useCreateSession.ts, and EmptyDevelop.tsx's startSessionOnWorktree)
// or a session verb (the seven in develop/sessionVerbs.ts, and SessionPane.tsx's exit-bar
// Restart), plus the poller, which fetches sessions. Every one of them acts on a row the user
// already has. Nothing else LISTS a machine, a project, a harness, a branch or a worktree, so
// without this file the rail renders an empty store for ever.

import { useEffect } from "react";
import type { JSX } from "react";
import { api } from "./api";
import { ApiError } from "./api/client";
import type { CockpitClient } from "./api/client";
import { startPolling } from "./api/poller";
import { AppShell } from "./components/AppShell";
import { useCockpit } from "./store/store";
import type { CockpitState } from "./store/store";
import type { BranchId, ProjectId } from "./store/types";

type Store = typeof useCockpit;

/** Derived rather than imported: `LoadKey` is declared in store/slices/dataSlice.ts and this
 *  file has no other reason to reach inside a slice. Deriving it also means a change to the
 *  key vocabulary is a compile error here rather than a silently unmatched string. */
type LoadKey = Parameters<CockpitState["setLoading"]>[0];

type Problem = { code: string; message: string };

function asError(error: unknown): Problem {
  return error instanceof ApiError
    ? { code: error.code, message: error.message }
    : { code: "internal", message: String(error) };
}

/** Every project whose branches the user has asked to see: expanded in the rail, or
 *  selected. Both, because selecting a project from the palette does not expand it. */
function openProjects(state: CockpitState): ProjectId[] {
  const ids = Object.keys(state.expandedProjects).filter((id) => state.expandedProjects[id]);
  const selected = state.selection.projectId;
  return selected !== null && !ids.includes(selected) ? [...ids, selected] : ids;
}

function openBranches(state: CockpitState): BranchId[] {
  const ids = Object.keys(state.expandedBranches).filter((id) => state.expandedBranches[id]);
  const selected = state.selection.branchId;
  return selected !== null && !ids.includes(selected) ? [...ids, selected] : ids;
}

/** `listWorktrees` is keyed by project AND branch (§4.4), so a branch cannot be fetched
 *  until its project's branch list has arrived. Null means "not yet"; the store write that
 *  delivers the branch list wakes the subscription and this runs again. */
function projectOfBranch(state: CockpitState, branchId: BranchId): ProjectId | null {
  for (const [projectId, branches] of Object.entries(state.branchesByProject)) {
    if (branches.some((branch) => branch.id === branchId)) return projectId;
  }
  return null;
}

export interface Catalogue {
  /** After this returns, nothing this loader started writes the store — the same contract as
   *  `startPolling`'s stop, and for the same reason: a reply that lands after unmount must not
   *  repopulate anything. */
  stop(): void;
  /** Retries every list that failed or has not arrived, at most one request in flight per
   *  list. Wired to the poller's good tick, which is the one proof the control plane answers,
   *  so there is no second timer and a control plane that is down costs no extra request. */
  retry(): void;
}

/**
 * Loads the catalogue, keeps loading it as the user navigates, and retries what failed when
 * the poller proves the control plane is answering.
 *
 * All of its memory is its own — what is in flight, what failed, what arrived, which error it
 * raised — and none of it is read back from the store. That is what makes StrictMode simple:
 * it mounts, stops and re-mounts this effect, the stopped loader's replies are discarded, and
 * the second loader fetches for itself. It costs one duplicate request per list in a dev build
 * only, which is exactly what the poller already costs, and it means no loader ever acts on
 * another loader's state.
 */
export function startCatalogue(client: CockpitClient, store: Store): Catalogue {
  let stopped = false;
  const inFlight = new Set<LoadKey>();
  /** Keys whose last attempt threw. `sync` runs on EVERY store write — including this loader's
   *  own `setLoading` and `setError` — so without this memo a failure would feed itself. Only
   *  `retry` clears it. */
  const failed = new Set<LoadKey>();
  /** The three lists that belong to no selection, once their reply has landed. Tracked here
   *  rather than read off the store, because an empty list is a real answer. */
  const arrived = new Set<LoadKey>();
  /** The error object each key last reported. setError stores the object it is handed, so
   *  identity says whether `lastError` is still this list's report when the list recovers. */
  const raised = new Map<LoadKey, Problem>();

  const run = async <T,>(
    key: LoadKey,
    fetch: () => Promise<T>,
    commit: (value: T) => void,
  ): Promise<void> => {
    // Every request and every store write this loader makes passes through here, so this is
    // the one `stopped` check that "starts nothing after stop" needs; `sync` and `retry` carry
    // none of their own. App's callers never reach it after stop (its cleanup stops this loader
    // and the poller together, synchronously), so defects.test.tsx pins it directly, by calling
    // `retry` on a stopped loader.
    if (stopped || inFlight.has(key) || failed.has(key)) return;
    inFlight.add(key);
    store.getState().setLoading(key, true);
    try {
      const value = await fetch();
      if (stopped) return;
      commit(value);
      arrived.add(key);
      // An error is cleared by the success of the operation that raised it, and only if it is
      // still the one on screen: a later failure elsewhere keeps its own report. Identity alone
      // decides. A key that never failed has no entry, and undefined never equals `lastError`,
      // which is an object or null. A key that failed and has since recovered keeps its old
      // entry, which cannot match again, because setError is handed each Problem exactly once,
      // at the failure that built it.
      if (store.getState().lastError === raised.get(key)) store.getState().setError(null);
    } catch (error) {
      if (stopped) return;
      failed.add(key);
      const problem = asError(error);
      raised.set(key, problem);
      store.getState().setError(problem);
    } finally {
      inFlight.delete(key);
      if (!stopped) store.getState().setLoading(key, false);
    }
  };

  // Fetched in parallel — they are independent, and serialising them would put three round
  // trips in front of the first paint. Nothing waits for all three: `sync` reads none of them.
  const loadRoots = (): void => {
    if (!arrived.has("machines")) {
      void run("machines", () => client.listMachines(), (v) => store.getState().setMachines(v));
    }
    if (!arrived.has("projects")) {
      void run("projects", () => client.listProjects(), (v) => store.getState().setProjects(v));
    }
    if (!arrived.has("harnesses")) {
      void run("harnesses", () => client.listHarnesses(), (v) => store.getState().setHarnesses(v));
    }
  };

  const sync = (): void => {
    const state = store.getState();

    for (const projectId of openProjects(state)) {
      if (state.branchesByProject[projectId] !== undefined) continue;
      void run(
        `branches:${projectId}`,
        () => client.listBranches(projectId),
        (branches) => store.getState().setBranches(projectId, branches),
      );
    }

    for (const branchId of openBranches(state)) {
      if (state.worktreesByBranch[branchId] !== undefined) continue;
      const projectId = projectOfBranch(state, branchId);
      if (projectId === null) continue;
      void run(
        `worktrees:${branchId}`,
        () => client.listWorktrees(projectId, branchId),
        (worktrees) => store.getState().setWorktrees(branchId, worktrees),
      );
    }
  };

  loadRoots();
  const unsubscribe = store.subscribe(sync);
  // A rehydrated selection is navigation that already happened, so its lists are asked for now,
  // beside the root lists, rather than when the first store write after `subscribe` wakes
  // `sync`. In the assembled app that write comes within one round trip anyway (a root reply,
  // or the poller's first tick), so only defects.test.tsx's held-client test can see this call.
  sync();

  return {
    stop: () => {
      stopped = true;
      unsubscribe();
    },
    retry: () => {
      // Writes nothing when nothing failed and everything arrived, which is every quiet tick:
      // the poller's one-store-write-per-quiet-tick property holds with this hook attached.
      failed.clear();
      loadRoots();
      sync();
    },
  };
}

export default function App(): JSX.Element {
  // §4.7 says the poller starts on mount of AppShell; §9 puts it here. Same lifetime: this
  // component renders AppShell and nothing else, so this effect IS AppShell's lifetime.
  useEffect(() => {
    const catalogue = startCatalogue(api, useCockpit);
    const stopPolling = startPolling(api, useCockpit, catalogue.retry);
    return () => {
      catalogue.stop();
      stopPolling();
    };
  }, []);

  return <AppShell />;
}
