// desktop/src/store/selectors.ts
//
// Contract §3.7. EVERY derived read lives here — shared-shape §4. A component that
// inlines a `.filter()` inside useCockpit(...) returns a new array on every store write and
// re-renders on every 2 s poll, which is the layout shift docs/AGENTIC-UX.md §10 rule 7 bans.

import { resolveDefaults } from "./defaults";
import type {
  Branch,
  BranchId,
  Harness,
  HarnessId,
  Machine,
  ProjectId,
  Session,
  SessionDefaults,
  SessionId,
  Worktree,
  WorktreeId,
} from "./types";
import type { CockpitState } from "./store";

/** Frozen module-level constants, so the absent case is referentially stable and costs no
 *  re-render. Frozen because a caller that mutated one would poison every other absent read. */
const NO_BRANCHES = Object.freeze([]) as unknown as Branch[];
const NO_WORKTREES = Object.freeze([]) as unknown as Worktree[];
const NO_IDS = Object.freeze([]) as unknown as SessionId[];

/** The machine flagged `default`, else the first. Undefined only before the first fetch. */
export const selectDefaultMachine = (s: CockpitState): Machine | undefined =>
  s.machines.find((m) => m.default) ?? s.machines[0];

export const selectBranches =
  (projectId: ProjectId | null) =>
  (s: CockpitState): Branch[] =>
    (projectId === null ? undefined : s.branchesByProject[projectId]) ?? NO_BRANCHES;

export const selectWorktrees =
  (branchId: BranchId | null) =>
  (s: CockpitState): Worktree[] =>
    (branchId === null ? undefined : s.worktreesByBranch[branchId]) ?? NO_WORKTREES;

/** Ids, not objects: an id array is referentially stable across a poll that changed nothing,
 *  because dataSlice.replaceSessions keeps the previous array when the ids match. */
export const selectSessionIdsForWorktree =
  (worktreeId: WorktreeId | null) =>
  (s: CockpitState): SessionId[] =>
    (worktreeId === null ? undefined : s.sessionIdsByWorktree[worktreeId]) ?? NO_IDS;

export const selectSession =
  (id: SessionId | null) =>
  (s: CockpitState): Session | undefined =>
    id === null ? undefined : s.sessions[id];

export const selectHarness =
  (id: HarnessId | null) =>
  (s: CockpitState): Harness | undefined =>
    id === null ? undefined : s.harnesses.find((h) => h.id === id);

export const selectFocusedSessionId = (s: CockpitState): SessionId | null =>
  s.selection.sessionId;

/** THE Columns order, and the one true ordering: the worktrees of the selected branch in
 *  server order, and within each, its sessions in server order. ⌘1-9, the palette and
 *  ColumnsLayout all index this and nothing else. An exited session stays in it — a killed
 *  session keeps its record and stays listed, because the transcript is the artifact.
 *
 *  Builds a fresh array, so consume it with useShallow:
 *      const ids = useCockpit(useShallow(selectPaneSessionIds)); */
export const selectPaneSessionIds = (s: CockpitState): SessionId[] => {
  const branchId = s.selection.branchId;
  if (branchId === null) return NO_IDS;
  const worktrees = s.worktreesByBranch[branchId];
  if (worktrees === undefined || worktrees.length === 0) return NO_IDS;
  const out: SessionId[] = [];
  for (const w of worktrees) {
    const ids = s.sessionIdsByWorktree[w.id];
    if (ids !== undefined) out.push(...ids);
  }
  return out.length === 0 ? NO_IDS : out;
};

/** `pinned` filtered to sessions that still exist, in pin order. A pin whose session has been
 *  forgotten is dropped here rather than in prefsSlice, so an in-flight poll cannot blank a
 *  tile the user is looking at. Consume with useShallow. */
export const selectCanvasSessionIds = (s: CockpitState): SessionId[] => {
  const out = s.pinned.filter((id) => s.sessions[id] !== undefined);
  return out.length === 0 ? NO_IDS : out;
};

/** The inheritance chain, resolved against the current selection and the live catalogue.
 *  Returns a FRESHLY BUILT object, so consume it with useShallow — an addition to §3.7,
 *  which names useShallow only for the two array selectors:
 *      const d = useCockpit(useShallow(selectResolvedDefaults)); */
export const selectResolvedDefaults = (s: CockpitState): SessionDefaults =>
  resolveDefaults(
    s.defaults,
    s.selection.projectId === null ? undefined : s.projectDefaults[s.selection.projectId],
    s.selection.branchId === null ? undefined : s.branchDefaults[s.selection.branchId],
    s.harnesses,
  );

// TASK 18 adds its selectors here, and there is no fifth. See its step 10.
//
//   selectBranchAttentionSessionId(branchId) => (s) => SessionId | null
//     the highest-attention status among a branch's sessions, and the id of the first
//     session carrying it, so the branch row's dot and its click target come from one read
//   selectProjectById(id)  => (s) => Project | undefined
//   selectBranchById(id)   => (s) => Branch | undefined
//   selectWorktreeById(id) => (s) => Worktree | undefined
//     by-id reads §3.7 has for a session and a harness but not for the three catalogue
//     levels, which ContextMenu needs because openContextMenu hands it only an id and a kind
