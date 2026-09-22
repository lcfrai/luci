// desktop/src/store/slices/dataSlice.ts
//
// Contract §3.2. Server-derived entities and their setters.

import type { StateCreator } from "zustand";
import type {
  Branch,
  BranchId,
  Harness,
  Machine,
  Project,
  ProjectId,
  Session,
  SessionId,
  Worktree,
  WorktreeId,
} from "../types";
import type { CockpitState } from "../store";

export type LoadKey =
  | "machines"
  | "projects"
  | "harnesses"
  | "sessions"
  | `branches:${string}`
  | `worktrees:${string}`;

export interface DataSlice {
  machines: Machine[];
  projects: Project[];
  branchesByProject: Record<ProjectId, Branch[]>;
  worktreesByBranch: Record<BranchId, Worktree[]>;
  sessions: Record<SessionId, Session>;
  /** Server order (createdAt descending) preserved. Never re-sorted client-side. */
  sessionIdsByWorktree: Record<WorktreeId, SessionId[]>;
  harnesses: Harness[];
  loading: Partial<Record<LoadKey, boolean>>;
  /** The last API failure, for the one status line in the rail footer. Cleared by an
   *  explicit setError(null) and by nothing else — see decision (a). */
  lastError: { code: string; message: string } | null;

  setMachines(machines: Machine[]): void;
  setProjects(projects: Project[]): void;
  setBranches(projectId: ProjectId, branches: Branch[]): void;
  setWorktrees(branchId: BranchId, worktrees: Worktree[]): void;
  setHarnesses(harnesses: Harness[]): void;
  replaceSessions(sessions: Session[], worktreeIds: WorktreeId[]): void;
  upsertSession(session: Session): void;
  removeSession(id: SessionId): void;
  setLoading(key: LoadKey, value: boolean): void;
  setError(error: { code: string; message: string } | null): void;
}

type DataState = Pick<
  DataSlice,
  | "machines" | "projects" | "branchesByProject" | "worktreesByBranch"
  | "sessions" | "sessionIdsByWorktree" | "harnesses" | "loading" | "lastError"
>;

export const initialDataState: DataState = {
  machines: [],
  projects: [],
  branchesByProject: {},
  worktreesByBranch: {},
  sessions: {},
  sessionIdsByWorktree: {},
  harnesses: [],
  loading: {},
  lastError: null,
};

/** Shallow field comparison, which is exact for `Session`: every field is a primitive, so
 *  there is no nested value a shallow pass could miss. Used to keep a poll from replacing an
 *  object that did not change. */
export function sameSession(a: Session, b: Session): boolean {
  const keys = Object.keys(a) as (keyof Session)[];
  if (keys.length !== Object.keys(b).length) return false;
  for (const k of keys) {
    if (a[k] !== b[k]) return false;
  }
  return true;
}

export function sameIds(a: readonly SessionId[], b: readonly SessionId[]): boolean {
  if (a.length !== b.length) return false;
  for (let i = 0; i < a.length; i += 1) {
    if (a[i] !== b[i]) return false;
  }
  return true;
}

export const createDataSlice: StateCreator<
  CockpitState,
  [["zustand/persist", unknown]],
  [],
  DataSlice
> = (set, get) => ({
  ...initialDataState,

  setMachines: (machines) => set({ machines }),
  setProjects: (projects) => set({ projects }),
  setBranches: (projectId, branches) =>
    set((s) => ({ branchesByProject: { ...s.branchesByProject, [projectId]: branches } })),
  setWorktrees: (branchId, worktrees) =>
    set((s) => ({ worktreesByBranch: { ...s.worktreesByBranch, [branchId]: worktrees } })),
  setHarnesses: (harnesses) => set({ harnesses }),

  replaceSessions: (incoming, worktreeIds) =>
    set((s) => {
      // The fetch covered these worktrees, plus any the incoming rows mention. A covered
      // worktree with no incoming rows empties; an uncovered one is untouched.
      const covered = new Set<WorktreeId>(worktreeIds);
      for (const row of incoming) covered.add(row.worktreeId);

      const sessions: Record<SessionId, Session> = {};
      for (const [id, row] of Object.entries(s.sessions)) {
        if (!covered.has(row.worktreeId)) sessions[id] = row;
      }
      for (const row of incoming) {
        const prev = s.sessions[row.id];
        sessions[row.id] = prev !== undefined && sameSession(prev, row) ? prev : row;
      }

      const built: Record<WorktreeId, SessionId[]> = {};
      for (const id of covered) built[id] = [];
      for (const row of incoming) built[row.worktreeId].push(row.id);

      const sessionIdsByWorktree = { ...s.sessionIdsByWorktree };
      for (const [id, ids] of Object.entries(built)) {
        const prev = sessionIdsByWorktree[id];
        sessionIdsByWorktree[id] = prev !== undefined && sameIds(prev, ids) ? prev : ids;
      }
      return { sessions, sessionIdsByWorktree };
    }),

  upsertSession: (session) => {
    const prev = get().sessions[session.id];
    // Not `set({})`: returning an empty partial still builds a new state object and runs
    // every subscriber. The only way to write nothing is not to call set.
    if (prev !== undefined && sameSession(prev, session)) return;
    set((s) => {
      const sessions = { ...s.sessions, [session.id]: session };
      // A known id keeps its position — AGENTIC-UX §10 rule 7.
      if (prev !== undefined) return { sessions };
      const list = s.sessionIdsByWorktree[session.worktreeId] ?? [];
      return {
        sessions,
        sessionIdsByWorktree: {
          ...s.sessionIdsByWorktree,
          [session.worktreeId]: [session.id, ...list],
        },
      };
    });
  },

  removeSession: (id) => {
    const target = get().sessions[id];
    if (target === undefined) return;
    set((s) => {
      const sessions = { ...s.sessions };
      delete sessions[id];
      const tiles = { ...s.tiles };
      delete tiles[id];
      const list = s.sessionIdsByWorktree[target.worktreeId];
      return {
        sessions,
        tiles,
        sessionIdsByWorktree:
          list === undefined
            ? s.sessionIdsByWorktree
            : { ...s.sessionIdsByWorktree, [target.worktreeId]: list.filter((x) => x !== id) },
        pinned: s.pinned.includes(id) ? s.pinned.filter((x) => x !== id) : s.pinned,
        selection:
          s.selection.sessionId === id ? { ...s.selection, sessionId: null } : s.selection,
      };
    });
  },

  setLoading: (key, value) => set((s) => ({ loading: { ...s.loading, [key]: value } })),
  setError: (lastError) => set({ lastError }),
});
