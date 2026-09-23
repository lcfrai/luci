### Task 18: Desktop substrate — phase 3's wire types, client functions, and the convergence slice on the existing poll tick

The desktop half of phase 3 starts here, and it starts with **no UI**: the types that mirror the Go
JSON of tasks 04–17, the client functions that call those routes, an **unpersisted** slice that
holds what they return, and a loader that runs **on the poller's existing good tick** — never on
an interval of its own (orchestrator ruling R6; `DESIGN-SYSTEM.md`'s one clock) — and only while
the Merge or Files lens is mounted for the selected branch. It codes against the part files'
shapes and a fake `fetch`, not a running server, so it can start once gate G2 holds.

**Nothing phase 3 adds to the store is persisted** (R3: phase 3 claims no persist version). git is
the truth for every git fact (A1), so the desktop re-reads it; `partialize` is unchanged, and a
test says so.

**Files:**
- `desktop/src/api/convergence.ts` — the wire types (new)
- `desktop/src/api/client.ts` — `ProblemCode` gains two codes; `send` takes a per-call timeout; phase 3's methods appended to `CockpitClient` and `createClient` (p2, append-only)
- `desktop/src/api/client.phase3.test.ts` (new)
- `desktop/src/api/poller.ts` — `onGoodTick` listener set, additive (p2)
- `desktop/src/store/slices/convergenceSlice.ts` + test (new)
- `desktop/src/store/store.ts` — compose the slice; `partialize` and `PERSIST_VERSION` untouched (p2)
- `desktop/src/store/selectors.ts` — appended selectors (p2)
- `desktop/src/hooks/useConvergenceLoader.ts` + test (new)
- `desktop/src/test/fixtures.ts` — `aConvergence`, `aLaneView`, `aChanges`, `aHistoryEntry` (p2, appended)
- `docs/superpowers/contracts/2026-09-18-phase-2-ui-contract.md` — §4.2's code list (p2)

#### Interfaces — complete

```ts
// desktop/src/api/convergence.ts — mirrors the Go JSON exactly. Renaming a field here breaks at
// runtime, not compile time (types.ts's own warning), so each block names its Go source.
import type { BranchId, ProjectId, SessionId, WorktreeId } from "../store/types";

/* task 07 — converge.Commit, converge.PathChange, converge.LaneObservation */
export type PathStatus =
  | "added" | "modified" | "deleted" | "renamed" | "typechange" | "untracked" | "unmerged";
export interface PathChange { path: string; status: PathStatus; from?: string }
export interface LaneCommit {
  sha: string; subject: string; authorName: string; at: string;
  /** [] renders "no session recorded" — the same words for outside-the-cockpit and bypassed. */
  sessions: SessionId[];
}
export interface LaneObservation {
  worktreeId: WorktreeId; name: string; synced: boolean; syncedAt: string; tip: string; base: string;
  commits: LaneCommit[]; truncated: boolean; changed: PathChange[]; uncommitted: PathChange[];
  readError: string;
}

/* task 08 — remote.State, converge.Forecast, QueueEntry, LaneView, Convergence */
export interface RemoteState {
  remote: string; upstream: string; ahead: number; behind: number;
  fetchedAt: string; fetchError: string; readError: string;
}
export type ForecastState = "clean" | "conflicts" | "nothing" | "unknown";
export interface Forecast {
  state: ForecastState; after: WorktreeId[]; conflictsWith: WorktreeId[]; paths: string[]; why: string;
}
export interface QueueEntry { worktreeId: WorktreeId; forecast: Forecast; settles: string[] }
export interface LaneSession { id: SessionId; title: string; harness: string; status: string }
export interface LaneView extends LaneObservation { ready: boolean; sessions: LaneSession[] }
export interface Convergence {
  projectId: ProjectId; branchId: BranchId; branch: string; tip: string; checkedOutAt: string;
  remote: RemoteState; queue: QueueEntry[]; lanes: LaneView[]; readAt: string;
}

/* task 09 — converge.LandResult */
export interface LandRequest {
  worktreeId: WorktreeId; expectedTip: string; briefKey?: string; briefDigest?: string;
}
export interface LandResult {
  merge: string; before: string; branch: string; worktreeId: WorktreeId; briefKey: string;
  readyCleared: boolean; refreshed: WorktreeId[]; refreshErrors: Record<WorktreeId, string>;
}

/* task 10 — converge.HistoryEntry, UndoResult, RevertResult */
export type HistoryKind = "land" | "revert" | "direct";
export type UndoState = "exact" | "revert" | "none";
export interface HistoryEntry {
  sha: string; parents: string[]; authorName: string; at: string; subject: string; kind: HistoryKind;
  worktreeId: WorktreeId | ""; worktreeName: string; briefKey: string; briefDigest: string;
  sessions: SessionId[]; reverts: string; pushed: boolean; undo: UndoState; undoWhy: "" | "pushed" | "built_upon";
}
export interface UndoResult {
  tip: string; undone: string; readyRestored: boolean;
  refreshed: WorktreeId[]; refreshErrors: Record<WorktreeId, string>;
}
export interface RevertResult { revert: string; before: string; reverts: string }

/* task 11 — lanes.Removed, converge.RenameResult, DeleteResult, lanes.Rename */
export interface Removed { worktreeId: WorktreeId; archivedAs: string }
export interface RenameResult {
  branch: { id: BranchId; projectId: ProjectId; name: string; isHead: boolean };
  renamedFrom: BranchId; remoteKeepsOldName: boolean;
}
export interface DeleteResult { archivedAs: string; archivedBrief: boolean }
export interface BranchRename {
  projectId: ProjectId; fromBranchId: BranchId; toBranchId: BranchId; from: string; to: string; at: string;
}

/* task 12 — remote.FetchResult, PushResult */
export interface FetchResult { remote: string; fetchedAt: string }
export interface PushResult {
  branch: string; remote: string; upstream: string; setUpstream: boolean; pushed: string; upToDate: boolean;
}

/* task 14 — githost.PullRequest */
export interface OpenPullRequest { target?: string; title?: string; draft?: boolean; briefKey?: string }
export interface PullRequest {
  number: number; webUrl: string; title: string; source: string; target: string; draft: boolean; existing: boolean;
}

/* task 15 — evidence.Record, evidence.State */
export type Verdict = "settled" | "contested";
export interface EvidenceInput {
  briefKey: string; criterionId: string; criterionDigest: string; commit: string; verdict: Verdict; because: string;
}
export interface EvidenceRecord extends EvidenceInput {
  id: string; repo: string; projectId: ProjectId; recordedBy: string; recordedAt: string;
}
export interface CriterionState {
  criterionId: string; criterionDigest: string; state: "open" | "settled" | "contested";
  because: string[]; evidence: string[]; recordedAgainst: string[]; commits: string[];
}
export interface EvidenceStates { commit: string; briefKey: string; states: CriterionState[]; shared: boolean }

/* task 16 + 17 — changes.* and claudecode.Span */
export interface Touch { worktreeId: WorktreeId; committed: PathStatus | ""; uncommitted: PathStatus | "" }
export type PathMark = "conflicts" | "both-edit" | "";
export interface PathRow {
  path: string; dir: string; touches: Touch[]; mark: PathMark; conflictBetween: [WorktreeId, WorktreeId][];
}
export interface Unreadable { worktreeId: WorktreeId; name: string; readError: string }
export interface Changes {
  projectId: ProjectId; branchId: BranchId; branch: string; paths: PathRow[]; unreadable: Unreadable[]; readAt: string;
}
export interface DiffLine { kind: " " | "+" | "-"; text: string }
export interface BackLink {
  tier: 0 | 1 | 2; commit: string; commitAt: string; sessions: SessionId[];
  sessionKinds: Record<SessionId, "agent" | "shell">; reason: string;
  toolUseId: string; toolName: string; spanAt: string;
}
export interface Hunk {
  oldStart: number; oldLines: number; newStart: number; newLines: number; header: string;
  lines: DiffLine[]; back: BackLink;
}
export interface LaneDiff {
  worktreeId: WorktreeId; name: string; committed: Hunk[]; uncommitted: Hunk[];
  binary: boolean; truncated: boolean; readError: string;
}
export interface PathDiff { path: string; lanes: LaneDiff[] }
export interface TranscriptSpan {
  toolUseId: string; tool: string; filePath: string; at: string; oldString: string; newString: string; before: string;
}

/** details.reason on a phase 3 409 — task 04's closed list. The desktop switches on it and
 *  never on the message. */
export type ConflictReason =
  | "name_taken" | "git_too_old" | "branch_checked_out" | "branch_moved" | "merge_conflict"
  | "nothing_to_land" | "not_ready" | "brief_unresolvable" | "no_git_identity" | "undo_not_exact"
  | "live_sessions" | "confirm_required" | "confirm_stale" | "not_a_cockpit_worktree"
  | "branch_has_worktrees" | "branch_exists" | "no_remote" | "push_rejected" | "unknown_host"
  | "no_credential" | "not_pushed" | "transcript_unreadable";

/** The reason of a phase 3 conflict, or undefined for anything else. */
export function conflictReason(error: unknown): ConflictReason | undefined;
```

```ts
// client.ts — appended. ProblemCode gains "git_failed" | "host_failed" (both classify as
// "server-said" through classify's existing catch-all; no ProblemKind changes).
// send(method, route, body?, timeoutMs?) — the fourth argument overrides DEFAULT_TIMEOUT_MS for
// the calls that run git against a remote or clone a repository.
export const GIT_WRITE_TIMEOUT_MS = 150_000;  // create worktree, land, undo, revert, branch writes, evidence
export const GIT_REMOTE_TIMEOUT_MS = 200_000; // fetch, push, PR — above the server's 3-minute gitx.Remote

export interface CockpitClient {
  // … phase 2's methods, unchanged …
  createWorktree(projectId: ProjectId, branchId: BranchId, name?: string): Promise<Worktree>;
  removeWorktree(projectId: ProjectId, worktreeId: WorktreeId, confirm?: string): Promise<Removed>;
  createBranch(projectId: ProjectId, name: string, fromBranchId: BranchId): Promise<Branch>;
  renameBranch(projectId: ProjectId, branchId: BranchId, name: string): Promise<RenameResult>;
  deleteBranch(projectId: ProjectId, branchId: BranchId): Promise<DeleteResult>;
  listRenames(projectId: ProjectId): Promise<BranchRename[]>;
  getConvergence(projectId: ProjectId, branchId: BranchId, briefKey?: string): Promise<Convergence>;
  setQueue(projectId: ProjectId, branchId: BranchId, order: WorktreeId[]): Promise<WorktreeId[]>;
  land(projectId: ProjectId, branchId: BranchId, req: LandRequest): Promise<LandResult>;
  listLands(projectId: ProjectId, branchId: BranchId, limit?: number): Promise<HistoryEntry[]>;
  undoLand(projectId: ProjectId, branchId: BranchId, sha: string, expectedTip: string): Promise<UndoResult>;
  revertLand(projectId: ProjectId, branchId: BranchId, sha: string, expectedTip: string): Promise<RevertResult>;
  fetchProject(projectId: ProjectId): Promise<FetchResult>;
  pushBranch(projectId: ProjectId, branchId: BranchId): Promise<PushResult>;
  openPullRequest(projectId: ProjectId, branchId: BranchId, req: OpenPullRequest): Promise<PullRequest>;
  recordEvidence(projectId: ProjectId, input: EvidenceInput): Promise<EvidenceRecord>;
  evidenceStates(projectId: ProjectId, commit: string, briefKey: string): Promise<EvidenceStates>;
  getChanges(projectId: ProjectId, branchId: BranchId): Promise<Changes>;
  getPathDiff(projectId: ProjectId, branchId: BranchId, path: string): Promise<PathDiff>;
  getTranscriptSpan(sessionId: SessionId, toolUseId: string): Promise<TranscriptSpan>;
}
```

```ts
// poller.ts — additive. The existing onGoodTick parameter (task 24's) is untouched.
/** Subscribe to every good tick of the one poller. The returned function unsubscribes. A
 *  listener that throws is caught and ignored, so one lens cannot stop another's loads. */
export function addGoodTickListener(fn: () => void): () => void;
```

```ts
// store/slices/convergenceSlice.ts — UNPERSISTED. Its keys never appear in partialize.
import type { StateCreator } from "zustand";

export type Load<T> =
  | { status: "loading" }
  | { status: "ready"; value: T; at: number }
  /** A failed reload keeps the last good value, dimmed as stale by the lens (prototype m-down). */
  | { status: "error"; error: { code: string; message: string; reason?: ConflictReason }; last?: T; at: number };

export interface ConvergenceSlice {
  convergenceByBranch: Record<BranchId, Load<Convergence>>;
  historyByBranch: Record<BranchId, Load<HistoryEntry[]>>;
  changesByBranch: Record<BranchId, Load<Changes>>;
  /** The path open in Files, per branch. Client-only, unpersisted. */
  openPathByBranch: Record<BranchId, string | null>;
  pathDiffByBranch: Record<BranchId, Load<PathDiff>>;
  setConvergence(branchId: BranchId, load: Load<Convergence>): void;
  setHistory(branchId: BranchId, load: Load<HistoryEntry[]>): void;
  setChanges(branchId: BranchId, load: Load<Changes>): void;
  openPath(branchId: BranchId, path: string | null): void;
  setPathDiff(branchId: BranchId, load: Load<PathDiff>): void;
}

export const initialConvergenceState: Pick<ConvergenceSlice,
  "convergenceByBranch" | "historyByBranch" | "changesByBranch" | "openPathByBranch" | "pathDiffByBranch">;

export const createConvergenceSlice: StateCreator<CockpitState, [], [], ConvergenceSlice>;
```

```ts
// hooks/useConvergenceLoader.ts
/** Mounted by MergeLens (what: "merge") and FilesLens (what: "files"). While mounted, it loads
 *  the selected branch's data once at mount and then on every good poll tick, never stacking a
 *  request behind one still in flight; it stops at unmount. Merge loads convergence (with the
 *  head brief key when phase 4's selector provides one, else none) and, when the tip it just
 *  read differs from the tip history was last read at, the history. Files loads changes, and
 *  the open path's diff when one is open. It returns `reload()` for a lens to call right after a
 *  mutation, so a land's result is on screen without waiting for the next tick. */
export function useConvergenceLoader(what: "merge" | "files"): { reload: () => void };
```

```ts
// store/selectors.ts — appended.
export const selectConvergence = (branchId: BranchId | null) => (s: CockpitState) => Load<Convergence> | undefined;
export const selectHistory     = (branchId: BranchId | null) => (s: CockpitState) => Load<HistoryEntry[]> | undefined;
export const selectChanges     = (branchId: BranchId | null) => (s: CockpitState) => Load<Changes> | undefined;
/** Phase 4's head brief key for a branch, or "" — returns "" until phase 4's provenance slice
 *  exists, and is the only place phase 3 reads it, so wiring it is a one-line change then. */
export const selectHeadBriefKey = (branchId: BranchId | null) => (s: CockpitState) => string;
```

#### The tests to write first

| Test | Asserts | Mutant it kills |
|---|---|---|
| `every phase 3 method sends the method, path and body its part names` | a fake `fetch` records each call; one row per method, e.g. `land` → `POST /v1/projects/p-1/branches/b-1/lands` with the `LandRequest` JSON | a route typo that only a live run finds |
| `path segments are encoded` | a branch id with `/` and a path with `#` reach the URL encoded | a diff route reading the wrong path |
| `remote and write calls use their longer timeouts` | a fake that never answers: `pushBranch` is not aborted at 10 s and is at 200 s (fake timers) | a push aborted by the client mid-flight |
| `git_failed and host_failed classify as server-said` | `classify(new ApiError("git_failed", …))` → `"server-said"` | a new ProblemKind by accident (R4 rules one for upstream-down only) |
| `conflictReason reads details.reason only` | an ApiError with `details.reason: "branch_moved"` → that; a message mentioning "branch_moved" with no detail → undefined | branching on a sentence |
| `the slice is not persisted` | `partialize(state)` has no key from `ConvergenceSlice`; `PERSIST_VERSION` is still 1 | a git fact persisted (A1), or an unclaimed version bump (R3) |
| `a failed reload keeps the last value` | ready then error → `{status:"error", last: <the ready value>}` | the queue blanking when the control plane blips |
| `the loader runs on good ticks only while mounted` | mount → one load; two good ticks → two more; unmount → a third tick loads nothing | a lens that keeps polling after it closes |
| `the loader never stacks` | a tick while a load is in flight → no second request | requests piling up behind a slow merge-tree |
| `the loader adds no interval` | `vi.spyOn(globalThis, "setInterval")` across mount/unmount → 0 calls | a second clock (R6) |
| `history reloads only when the tip moves` | two ticks with the same tip → one `listLands`; a new tip → a second | re-reading history every 2 s |
| `reload loads immediately` | `reload()` → a request without waiting for a tick | a land that appears two seconds late |
| `a listener that throws does not stop the others` | two listeners, the first throws → the second still runs | one lens breaking another |
| `the fixture builders produce the wire shapes` | `aConvergence()` type-checks as `Convergence` and round-trips through JSON | fixtures drifting from the types |

#### Steps

- [ ] **Step 1: Preflight.** G2 holds (`ls desktop/src/components/lens/MergeLens.tsx desktop/src/components/lens/FilesLens.tsx`).
  Parts 04 and 08–17 are the source of every shape; if an implemented Go task changed a field,
  its part file was updated in the same commit (the rule for this plan) and this task follows the
  part file.
- [ ] **Step 2:** tests; `convergence.ts`; the client additions; `addGoodTickListener`; the slice;
  the store composition; selectors; the hook; fixtures.
- [ ] **Step 3:** UI contract §4.2 gains the two codes and a sentence: "phase 3 conflicts carry
  `details.reason` from `api/convergence.ts`'s `ConflictReason`".
- [ ] **Step 4:** `npx tsc --noEmit`, `npx vitest run`; commit by pathspec.

#### Acceptance

```
cd desktop && npx tsc --noEmit   → no output
cd desktop && npx vitest run     → PASS
git grep -n "setInterval" -- desktop/src | grep -v "\.test\." → exactly the two phase 2 sites (poller.ts, heartbeat.ts)
```

#### What this task does not claim

- Nothing is on screen; tasks 19–22 draw it.
- The shapes are the part files' until each Go task lands; a live run against the control plane is
  task 23's.
