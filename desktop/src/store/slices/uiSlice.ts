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
} from "../types";
import type { CockpitState } from "../store";

/** types.ts fixes both numbers in a doc comment; they are constants here so task 18's
 *  <Toasts/> reads them rather than re-deriving them. */
export const TOAST_TTL_UNDO = 6000;
export const TOAST_TTL_PLAIN = 3200;
/** Decision (c). A repeating poll failure must not build an unbounded column. */
export const MAX_TOASTS = 4;

const NO_SELECTION: Selection = {
  projectId: null,
  branchId: null,
  worktreeId: null,
  sessionId: null,
};

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
  // TASK 18 adds `machinesOpen: boolean;` here. See its step 11.

  selectProject(id: ProjectId | null): void;
  /** Sets lens to "develop" and clears worktree + session selection. */
  selectBranch(projectId: ProjectId, id: BranchId): void;
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
  // TASK 18 adds `openMachines(): void;` and `closeMachines(): void;` here.

  pushToast(message: string, undo?: () => void): void;
  dismissToast(id: string): void;
}

type UiState = Pick<
  UiSlice,
  | "selection" | "lens" | "layout" | "railCollapsed"
  | "expandedProjects" | "expandedBranches" | "expandedWorktrees"
  | "paletteOpen" | "pickerOpen" | "contextMenu" | "sessionMenuFor" | "toasts"
>;

export const initialUiState: UiState = {
  selection: NO_SELECTION,
  lens: "develop",
  layout: "columns",
  railCollapsed: false,
  expandedProjects: {},
  expandedBranches: {},
  expandedWorktrees: {},
  paletteOpen: false,
  pickerOpen: false,
  contextMenu: null,
  sessionMenuFor: null,
  toasts: [],
  // TASK 18 adds `machinesOpen: false,` here.
};

export const createUiSlice: StateCreator<
  CockpitState,
  [["zustand/persist", unknown]],
  [],
  UiSlice
> = (set) => ({
  ...initialUiState,

  // Decision (a): a project selection clears everything below it, so the rail and the
  // Columns view can never disagree about which branch is in view.
  selectProject: (id) => set({ selection: { ...NO_SELECTION, projectId: id } }),
  selectBranch: (projectId, id) =>
    set({
      lens: "develop",
      selection: { projectId, branchId: id, worktreeId: null, sessionId: null },
    }),
  // Decision (b): the focused session survives, because focus is not navigation.
  selectWorktree: (id) => set((s) => ({ selection: { ...s.selection, worktreeId: id } })),
  focusSession: (id) => set((s) => ({ selection: { ...s.selection, sessionId: id } })),

  setLens: (lens) => set({ lens }),
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
  // TASK 18 adds `openMachines` and `closeMachines` here.

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
