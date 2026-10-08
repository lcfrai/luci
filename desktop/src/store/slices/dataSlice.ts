// desktop/src/store/slices/dataSlice.ts
//
// Contract §3.2. Server-derived entities and their setters.

import type { StateCreator } from "zustand";
import { pendingSessionIds } from "../../lib/pendingSessions";
import type {
  Branch,
  BranchId,
  Harness,
  ImplementationContext,
  Machine,
  Project,
  ProjectId,
  Session,
  SessionId,
  Worktree,
  WorktreeId,
} from "../types";
import type { CockpitState } from "../store";
import { pruneProjectPositions, writeProjectNavigation } from "../../preferences/projectNavigation";

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
  projectsLoaded: boolean;
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
  | "machines" | "projects" | "projectsLoaded" | "branchesByProject" | "worktreesByBranch"
  | "sessions" | "sessionIdsByWorktree" | "harnesses" | "loading" | "lastError"
>;

/** FROZEN, containers and all. Every store in the process spreads this same object —
 *  `createCockpitStore()` on each call and `resetCockpit()` on each reset — so its nested
 *  containers are shared across all of them. Nothing mutates one today; freezing makes the
 *  day something does a loud TypeError rather than silent cross-store corruption. */
export const initialDataState: DataState = Object.freeze({
  machines: Object.freeze([]) as unknown as Machine[],
  projects: Object.freeze([]) as unknown as Project[],
  projectsLoaded: false,
  branchesByProject: Object.freeze({}) as Record<ProjectId, Branch[]>,
  worktreesByBranch: Object.freeze({}) as Record<BranchId, Worktree[]>,
  sessions: Object.freeze({}) as Record<SessionId, Session>,
  sessionIdsByWorktree: Object.freeze({}) as Record<WorktreeId, SessionId[]>,
  harnesses: Object.freeze([]) as unknown as Harness[],
  loading: Object.freeze({}) as Partial<Record<LoadKey, boolean>>,
  lastError: null,
});

/** The immutable implementation snapshot has primitive fields and three ordered string arrays.
 *  Compare their values so an unchanged poll preserves the session object and its history row. */
function sameImplementationContext(a: ImplementationContext | undefined, b: ImplementationContext | undefined): boolean {
  if (a === b) return true;
  if (!a || !b) return false;
  const keys = Object.keys(a) as (keyof ImplementationContext)[];
  if (keys.length !== Object.keys(b).length) return false;
  return keys.every(key => {
    const left = a[key], right = b[key];
    return Array.isArray(left) && Array.isArray(right) ? sameIds(left, right) : left === right;
  });
}

/** Primitive session fields plus the exact immutable implementation snapshot. */
export function sameSession(a: Session, b: Session): boolean {
  const keys = Object.keys(a) as (keyof Session)[];
  if (keys.length !== Object.keys(b).length) return false;
  for (const k of keys) {
    if (k === "implementationContext") {
      if (!sameImplementationContext(a.implementationContext, b.implementationContext)) return false;
    } else if (k === "reviewedContextDelivery") {
      const left = a.reviewedContextDelivery, right = b.reviewedContextDelivery;
      if (left !== right && (!left || !right || left.status !== right.status || left.version !== right.version || left.digest !== right.digest)) return false;
    } else if (a[k] !== b[k]) return false;
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

/** Preserve unchanged catalogue identities so a projects refresh does not rerender every branch. */
function retainRecord<T>(record: Record<string, T>, keep: (key: string, value: T) => boolean): Record<string, T> {
  const entries = Object.entries(record), retained = entries.filter(([key, value]) => keep(key, value));
  return retained.length === entries.length ? record : Object.fromEntries(retained);
}
function retainItems<T>(items: T[], keep: (value: T) => boolean): T[] {
  const retained = items.filter(keep);
  return retained.length === items.length ? items : retained;
}

export const createDataSlice: StateCreator<
  CockpitState,
  [["zustand/persist", unknown]],
  [],
  DataSlice
> = (set, get) => ({
  ...initialDataState,

  setMachines: (machines) => set({ machines }),
  setProjects: (projects) => set((state) => {
    // Only a successful catalogue response can invalidate a retained selection.
    // Failed or pending requests leave the user's current project in place.
    const missing = state.selection.projectId !== null &&
      !projects.some(project => project.id === state.selection.projectId);
    const ids = new Set(projects.map(project => project.id));
    const unchanged = new Set(projects.filter(project => !state.projects.some(old => old.id === project.id && old.path !== project.path)).map(project => project.id));
    const branchesByProject = retainRecord(state.branchesByProject, id => unchanged.has(id));
    const branchIds = new Set(Object.values(branchesByProject).flat().map(branch => branch.id));
    const worktreesByBranch = retainRecord(state.worktreesByBranch, id => branchIds.has(id));
    const sessions = retainRecord(state.sessions, (_, session) => unchanged.has(session.projectId));
    const removedBranchIds = new Set(Object.entries(state.branchesByProject).filter(([id]) => !unchanged.has(id)).flatMap(([, branches]) => branches.map(branch => branch.id)));
    const removedWorktreeIds = new Set(Object.entries(state.worktreesByBranch).filter(([id]) => removedBranchIds.has(id)).flatMap(([, worktrees]) => worktrees.map(worktree => worktree.id)));
    const removedSessionIds = new Set(Object.values(state.sessions).filter(session => !unchanged.has(session.projectId)).map(session => session.id));
    const sourceChanged = state.selection.projectId !== null && !unchanged.has(state.selection.projectId);
    const selection = missing ? { projectId: null, branchId: null, worktreeId: null, sessionId: null } : sourceChanged ? { ...state.selection, branchId: null, worktreeId: null, sessionId: null } : state.selection;
    const projectPositions = pruneProjectPositions(state.projectPositions, projects);
    const saved = !state.projectNavigationScope || writeProjectNavigation(state.projectNavigationScope, { projects: projectPositions, selected: selection.projectId });
    return {
      projects, projectsLoaded: true, selection, projectPositions, projectNavigationError: !saved,
      ...(sourceChanged && !missing ? { lens: "discover" as const } : {}),
      branchesByProject, worktreesByBranch, sessions,
      sessionIdsByWorktree: !removedWorktreeIds.size && !removedSessionIds.size ? state.sessionIdsByWorktree : Object.fromEntries(Object.entries(state.sessionIdsByWorktree).filter(([id]) => !removedWorktreeIds.has(id)).map(([id, list]) => [id, list.filter(sessionId => !removedSessionIds.has(sessionId))])),
      expandedProjects: retainRecord(state.expandedProjects, id => ids.has(id)),
      expandedBranches: retainRecord(state.expandedBranches, id => !removedBranchIds.has(id)),
      expandedWorktrees: retainRecord(state.expandedWorktrees, id => !removedWorktreeIds.has(id)),
      projectSourceRevisions: retainRecord(state.projectSourceRevisions, id => unchanged.has(id)),
      pinned: retainItems(state.pinned, id => !removedSessionIds.has(id)),
      tiles: retainRecord(state.tiles, id => !removedSessionIds.has(id)),
    };
  }),
  setBranches: (projectId, branches) => set(s => {
    if (s.projectsLoaded && !s.projects.some(project => project.id === projectId)) return s;
    const removed = new Set((s.branchesByProject[projectId] ?? []).filter(branch => !branches.some(next => next.id === branch.id)).map(branch => branch.id));
    return { branchesByProject: { ...s.branchesByProject, [projectId]: branches },
      worktreesByBranch: Object.fromEntries(Object.entries(s.worktreesByBranch).filter(([id]) => !removed.has(id))) };
  }),
  setWorktrees: (branchId, worktrees) => set(s => {
    if (s.projectsLoaded && !Object.values(s.branchesByProject).some(branches => branches.some(branch => branch.id === branchId))) return s;
    return { worktreesByBranch: { ...s.worktreesByBranch, [branchId]: worktrees } };
  }),
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

      // ── deferred-verb suppression. ADDED BY TASK 22's FIX ROUND 1, a declared departure
      // into task 17's file, and it belongs here rather than in the poller or in the verbs.
      //
      // Kill and Forget patch the store NOW and hold the request for the 6 s Undo window, so the
      // server still reports the old truth. The poller runs every 2 s. Without this, a killed row
      // comes back as `running`, and a forgotten row reappears, about two seconds into the window
      // — WITH THE UNDO TOAST STILL ON SCREEN.
      //
      // The store is the one place holding both the incoming rows and the pending ids, so every
      // future deferred verb gets this for free and no caller has to remember. A poller-side
      // check would leak store knowledge into a fetch loop; a re-patch after each tick is a race.
      //
      // The rule is simply: while a deferred verb is in flight, the store's own view of that id
      // wins — present with its optimistic status (Kill), or absent (Forget).
      for (const id of pendingSessionIds()) {
        const held = s.sessions[id];
        if (held === undefined) delete sessions[id];
        else sessions[id] = held;
      }

      const built: Record<WorktreeId, SessionId[]> = {};
      for (const id of covered) built[id] = [];
      // `sessions[row.id]` is the test, not `incoming`: a row a deferred Forget has removed must
      // not be re-indexed, or the rail renders an id the session map no longer holds.
      for (const row of incoming) {
        if (sessions[row.id] !== undefined) built[row.worktreeId].push(row.id);
      }
      // A held row the server has stopped listing keeps its old place. Present in `sessions` but
      // missing from the index renders nowhere, and Undo cannot repair it: `upsertSession` leaves
      // the position of an id it already holds unchanged, so there would be no position to leave.
      for (const id of pendingSessionIds()) {
        const held = sessions[id];
        if (held === undefined) continue;
        const list = built[held.worktreeId];
        if (list === undefined || list.includes(id)) continue;
        const at = (s.sessionIdsByWorktree[held.worktreeId] ?? []).indexOf(id);
        list.splice(at === -1 ? list.length : at, 0, id);
      }

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
        // An open menu pointing at a session that no longer exists is a surface reading a
        // dangling id: task 18 owns the ⋯ menu and task 22 owns its verbs, so clearing here
        // is one place instead of two later tasks each remembering to. The context menu
        // serves every row kind, so the kind is checked too — a branch whose id happens to
        // match must keep its menu open.
        sessionMenuFor: s.sessionMenuFor === id ? null : s.sessionMenuFor,
        contextMenu:
          s.contextMenu !== null && s.contextMenu.kind === "session" && s.contextMenu.id === id
            ? null
            : s.contextMenu,
      };
    });
  },

  setLoading: (key, value) => set((s) => ({ loading: { ...s.loading, [key]: value } })),
  setError: (lastError) => set({ lastError }),
});
