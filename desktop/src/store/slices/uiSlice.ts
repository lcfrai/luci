// desktop/src/store/slices/uiSlice.ts
//
// Contract §3.3. Selection, lens, layout, rail expansion, overlays, toasts.

import type { StateCreator } from "zustand";
import { makeId } from "../../lib/ids";
import type {
  BranchId,
  ContextMenuTarget,
  Layout,
  Lens,
  ProjectId,
  Selection,
  SessionId,
  Toast,
  WorktreeId,
  Branch,
  Worktree,
} from "../types";
import type { CockpitState } from "../store";

/** types.ts fixes both numbers in a doc comment; they are constants here so task 18's
 *  <Toasts/> reads them rather than re-deriving them. */
export const TOAST_TTL_UNDO = 6000;
export const TOAST_TTL_PLAIN = 3200;
/** Decision (c). A repeating poll failure must not build an unbounded column. */
export const MAX_TOASTS = 4;

const NO_SELECTION: Selection = Object.freeze({
  projectId: null,
  branchId: null,
  worktreeId: null,
  sessionId: null,
});

export interface ProjectRunDeploymentIntent { scope: string; projectId: ProjectId; runId: string; checkId: string; commit: string; sourceDigest: string; outcome: "preview" | "publish" }
/** Unspecified purpose is unsaved shared context. Local check setup still needs navigation/source protection. */
export interface UnsavedProjectDraft { projectId: ProjectId; label: string; lens: Lens; purpose?: "verification-setup"; retained?: boolean; discard?(): void }
export type PendingNavigation =
  | { kind: "project"; projectId: ProjectId | null; lens?: Lens }
  | { kind: "branch"; projectId: ProjectId; branchId: BranchId; worktreeId?: WorktreeId }
  | { kind: "lens"; lens: Lens };
export interface BranchManagerTarget { projectId: ProjectId; action: "create" | "switch"; branchId?: BranchId }
const navigationState = (navigation: PendingNavigation, selection: Selection): Partial<UiSlice> => {
  if (navigation.kind === "project") return { selection: { ...NO_SELECTION, projectId: navigation.projectId }, ...(navigation.lens ? { lens: navigation.lens } : {}) };
  if (navigation.kind === "branch") return { lens: "develop", selection: { projectId: navigation.projectId, branchId: navigation.branchId, worktreeId: navigation.worktreeId ?? null, sessionId: null } };
  return { lens: navigation.lens, selection };
};

function hasNavigationWorktree(state: CockpitState, projectId: ProjectId, branchId: BranchId, worktreeId: WorktreeId) {
  return state.projects.some(project => project.id === projectId) && state.branchesByProject[projectId]?.some(branch => branch.id === branchId && branch.projectId === projectId) &&
    state.worktreesByBranch[branchId]?.some(worktree => worktree.id === worktreeId && worktree.projectId === projectId && worktree.branchId === branchId);
}

export interface UiSlice {
  selection: Selection;
  lens: Lens;
  layout: Layout;
  railCollapsed: boolean;
  expandedProjects: Record<ProjectId, boolean>;
  expandedBranches: Record<BranchId, boolean>;
  expandedWorktrees: Record<WorktreeId, boolean>;

  paletteOpen: boolean;
  pickerOpen: boolean;
  contextMenu: ContextMenuTarget | null;
  /** The session whose ⋯ menu is open, or null. Separate from contextMenu because the ⋯
   *  menu anchors to a button and the context menu anchors to a pointer. */
  sessionMenuFor: SessionId | null;
  toasts: Toast[];
  /** ADDED BY TASK 18 - not in contract §3.3. The machine chip's popover. It lives here and
   *  not in MachineChip's own state because §6.9's ⌘⇧M must open it from useKeyboardMap,
   *  which is mounted in AppShell and cannot reach a component's local state. */
  machinesOpen: boolean;
  /** Runtime-only editor guards and source revisions; never saved as preferences. */
  unsavedProjectDrafts: Record<string, UnsavedProjectDraft>;
  pendingNavigation: PendingNavigation | null;
  branchManager: BranchManagerTarget | null;
  projectSourceRevisions: Record<ProjectId, number>;
  /** Scoped one-shot inspection request. Survives lens mounting, never persisted. */
  projectRunInspection: { scope: string; projectId: ProjectId; runId: string; refresh?: boolean } | null;
  projectRunVerification: { scope: string; projectId: ProjectId; runId: string } | null;
  projectRunDeployment: ProjectRunDeploymentIntent | null;
  setUnsavedProjectDraft(id: string, draft: UnsavedProjectDraft | null): void;
  cancelNavigation(): void;
  discardDraftsAndNavigate(keepDrafts?: boolean): void;
  openBranchManager(target: BranchManagerTarget): void;
  closeBranchManager(): void;
  acceptBranchChange(projectId: ProjectId, branches: Branch[], worktree: Worktree): void;

  selectProject(id: ProjectId | null, lens?: Lens): void;
  /** Sets lens to "develop", clears the session, and selects an explicit checkout or clears the worktree. */
  selectBranch(projectId: ProjectId, id: BranchId, worktreeId?: WorktreeId): void;
  selectWorktree(id: WorktreeId | null): void;
  /** Focus, not navigation: it does not change the branch. */
  focusSession(id: SessionId | null): void;
  setLens(lens: Lens): void;
  setLayout(layout: Layout): void;
  toggleRail(): void;
  toggleProject(id: ProjectId): void;
  toggleBranch(id: BranchId): void;
  toggleWorktree(id: WorktreeId): void;

  openPalette(): void;
  closePalette(): void;
  openPicker(): void;
  closePicker(): void;
  openContextMenu(target: ContextMenuTarget): void;
  closeContextMenu(): void;
  openSessionMenu(id: SessionId): void;
  closeSessionMenu(): void;
  openMachines(): void;
  closeMachines(): void;

  pushToast(message: string, undo?: () => void): void;
  dismissToast(id: string): void;
}

type UiState = Pick<
  UiSlice,
  | "selection" | "lens" | "layout" | "railCollapsed"
  | "expandedProjects" | "expandedBranches" | "expandedWorktrees"
  | "paletteOpen" | "pickerOpen" | "contextMenu" | "sessionMenuFor" | "toasts"
  | "machinesOpen" | "unsavedProjectDrafts" | "pendingNavigation" | "branchManager" | "projectSourceRevisions" | "projectRunInspection" | "projectRunVerification" | "projectRunDeployment"
>;

/** FROZEN, containers and all — see the note on `initialDataState`. */
export const initialUiState: UiState = Object.freeze({
  selection: NO_SELECTION,
  lens: "develop",
  layout: "columns",
  railCollapsed: false,
  expandedProjects: Object.freeze({}) as Record<ProjectId, boolean>,
  expandedBranches: Object.freeze({}) as Record<BranchId, boolean>,
  expandedWorktrees: Object.freeze({}) as Record<WorktreeId, boolean>,
  paletteOpen: false,
  pickerOpen: false,
  contextMenu: null,
  sessionMenuFor: null,
  toasts: Object.freeze([]) as unknown as Toast[],
  machinesOpen: false,
  unsavedProjectDrafts: Object.freeze({}),
  pendingNavigation: null,
  branchManager: null,
  projectSourceRevisions: Object.freeze({}),
  projectRunInspection: null,
  projectRunVerification: null,
  projectRunDeployment: null,
});

export const createUiSlice: StateCreator<
  CockpitState,
  [["zustand/persist", unknown]],
  [],
  UiSlice
> = (set, get) => ({
  ...initialUiState,

  // Decision (a): a project selection clears everything below it, so the rail and the
  // Columns view can never disagree about which branch is in view.
  selectProject: (id, lens) => {
    const state = get();
    if (lens && id !== null && !state.projects.some(project => project.id === id)) {
      state.pushToast("This project is no longer available. Refresh your projects before opening its files.");
      return;
    }
    if (id === state.selection.projectId && state.selection.branchId === null && (!lens || lens === state.lens)) return;
    const navigation: PendingNavigation = { kind: "project", projectId: id, ...(lens ? { lens } : {}) };
    if (Object.values(state.unsavedProjectDrafts).some(draft => draft.projectId === state.selection.projectId)) set({ pendingNavigation: navigation });
    else set(navigationState(navigation, state.selection));
  },
  selectBranch: (projectId, id, worktreeId) =>
    {
      const state = get();
      if (worktreeId !== undefined && !hasNavigationWorktree(state, projectId, id, worktreeId)) {
        state.pushToast("That checkout is no longer available. Choose a current build location.");
        return;
      }
      const navigation: PendingNavigation = { kind: "branch", projectId, branchId: id, ...(worktreeId !== undefined ? { worktreeId } : {}) };
      if ((state.selection.projectId !== projectId || state.selection.branchId !== id || state.lens !== "develop" || (worktreeId !== undefined && state.selection.worktreeId !== worktreeId)) && Object.values(state.unsavedProjectDrafts).some(draft => draft.projectId === state.selection.projectId)) set({ pendingNavigation: navigation });
      else set(navigationState(navigation, state.selection));
    },
  // Decision (b): the focused session survives, because focus is not navigation.
  selectWorktree: (id) => set((s) => ({ selection: { ...s.selection, worktreeId: id } })),
  focusSession: (id) => set((s) => ({ selection: { ...s.selection, sessionId: id } })),

  setLens: (lens) => {
    const state = get();
    if (state.lens === lens) return;
    if (Object.values(state.unsavedProjectDrafts).some(draft => draft.projectId === state.selection.projectId)) set({ pendingNavigation: { kind: "lens", lens } });
    else set({ lens });
  },
  setUnsavedProjectDraft: (id, draft) => set(state => {
    const drafts = { ...state.unsavedProjectDrafts };
    if (draft) drafts[id] = draft; else delete drafts[id];
    return { unsavedProjectDrafts: drafts };
  }),
  cancelNavigation: () => set({ pendingNavigation: null, projectRunInspection: null, projectRunVerification: null, projectRunDeployment: null }),
  discardDraftsAndNavigate: (keepDrafts = false) => {
    const current = get();
    const destination = current.pendingNavigation;
    if (destination?.kind === "project" && destination.lens && destination.projectId !== null &&
        !current.projects.some(project => project.id === destination.projectId)) {
      set({ pendingNavigation: null });
      current.pushToast("This project is no longer available. Your edited draft is still here.");
      return;
    }
    if (destination?.kind === "branch" && destination.worktreeId !== undefined &&
        !hasNavigationWorktree(current, destination.projectId, destination.branchId, destination.worktreeId)) {
      set({ pendingNavigation: null });
      current.pushToast("That checkout is no longer available. Choose a current build location. Your edited draft is still here.");
      return;
    }
    const leaving = Object.values(current.unsavedProjectDrafts).filter(draft => draft.projectId === current.selection.projectId);
    if (!destination || (keepDrafts && leaving.some(draft => !draft.retained))) return;
    if (!keepDrafts) leaving.forEach(draft => draft.discard?.());
    set(state => {
      if (!state.pendingNavigation) return state;
      const drafts = Object.fromEntries(Object.entries(state.unsavedProjectDrafts).filter(([, draft]) => draft.projectId !== state.selection.projectId));
      return { ...navigationState(state.pendingNavigation, state.selection), unsavedProjectDrafts: drafts, pendingNavigation: null };
    });
  },
  openBranchManager: target => set({ branchManager: target }),
  closeBranchManager: () => set({ branchManager: null }),
  acceptBranchChange: (projectId, branches, worktree) => set(state => {
    const worktreesByBranch = Object.fromEntries(Object.entries(state.worktreesByBranch).map(([id, worktrees]) => [id, worktrees.filter(item => item.id !== worktree.id)]));
    worktreesByBranch[worktree.branchId] = [...(worktreesByBranch[worktree.branchId] ?? []), worktree];
    return {
      branchesByProject: { ...state.branchesByProject, [projectId]: branches }, worktreesByBranch,
      projectSourceRevisions: { ...state.projectSourceRevisions, [projectId]: (state.projectSourceRevisions[projectId] ?? 0) + 1 },
      ...(state.selection.projectId === projectId ? { selection: { projectId, branchId: worktree.branchId, worktreeId: worktree.id, sessionId: null }, lens: "develop" as const } : {}),
      branchManager: null,
    };
  }),
  setLayout: (layout) => set({ layout }),
  toggleRail: () => set((s) => ({ railCollapsed: !s.railCollapsed })),
  // An absent key means collapsed, so the first toggle expands.
  toggleProject: (id) =>
    set((s) => ({ expandedProjects: { ...s.expandedProjects, [id]: !s.expandedProjects[id] } })),
  toggleBranch: (id) =>
    set((s) => ({ expandedBranches: { ...s.expandedBranches, [id]: !s.expandedBranches[id] } })),
  toggleWorktree: (id) =>
    set((s) => ({ expandedWorktrees: { ...s.expandedWorktrees, [id]: !s.expandedWorktrees[id] } })),

  openPalette: () => set({ paletteOpen: true }),
  closePalette: () => set({ paletteOpen: false }),
  openPicker: () => set({ pickerOpen: true }),
  closePicker: () => set({ pickerOpen: false }),
  openContextMenu: (target) => set({ contextMenu: target }),
  closeContextMenu: () => set({ contextMenu: null }),
  openSessionMenu: (id) => set({ sessionMenuFor: id }),
  closeSessionMenu: () => set({ sessionMenuFor: null }),
  openMachines: () => set({ machinesOpen: true }),
  closeMachines: () => set({ machinesOpen: false }),

  pushToast: (message, undo) =>
    set((s) => {
      const toast: Toast = {
        id: makeId("toast"),
        message,
        undo,
        ttl: undo === undefined ? TOAST_TTL_PLAIN : TOAST_TTL_UNDO,
        createdAt: Date.now(),
      };
      const next = [...s.toasts, toast];
      return { toasts: next.length > MAX_TOASTS ? next.slice(next.length - MAX_TOASTS) : next };
    }),
  dismissToast: (id) => set((s) => ({ toasts: s.toasts.filter((t) => t.id !== id) })),
});
