// desktop/src/store/types.ts
//
// The cockpit's domain model. Server-derived types mirror the control plane's JSON exactly
// (control-plane/internal/api, contract-go.md §3). Renaming a field here breaks at runtime,
// not at compile time, so the names are binding.

/* ── ids ───────────────────────────────────────────────────────────────────────────── */

export type MachineId = string; // "local"
export type ProjectId = string; // "p-8c1d0e42"
export type BranchId = string; // "b-8c1d0e42-evidence-upload"
export type WorktreeId = string; // "w-4a77b1c9"
export type SessionId = string; // "s-3f2a91c0"

/* ── machines ──────────────────────────────────────────────────────────────────────── */

export type MachineKind = "local" | "cloud";

/** The full enum from docs/FUNCTIONAL-SPEC.md "Machines". Phase 2's control plane emits only
 *  "running" and "error"; the other four need cloud machines (Phase 8). The cockpit renders
 *  all six so nothing changes here later. */
export type MachineState =
  "provisioning" | "running" | "idle" | "sleeping" | "stopped" | "error";

export type Isolation = "microvm" | "container" | "none";

export interface Machine {
  id: MachineId;
  name: string;
  kind: MachineKind;
  state: MachineState;
  /** "docker" | "kata" — informational; the cockpit never branches on it. */
  backend: string;
  isolation: Isolation;
  /** JSON field is literally "default". Legal as a TS property and as `m.default`. */
  default: boolean;
  /** Honest sentences, rendered verbatim. Never summarised into a badge. */
  limitations: string[];
  /** Present only when state === "error". Names the cause, e.g. Docker not running. */
  message?: string;
  /** Effective per-session limits after the operator ceiling and user policy. */
  sessionMemoryMiB?: number;
  sessionCPUMillis?: number;
}

/* ── projects, branches, worktrees ─────────────────────────────────────────────────── */

export interface Project {
  id: ProjectId;
  name: string;
  /** Absolute path on the machine. Windows paths arrive with backslashes. */
  path: string;
}

export interface Branch {
  id: BranchId;
  projectId: ProjectId;
  name: string;
  /** True for the branch the primary worktree currently has checked out. */
  isHead: boolean;
}

export interface Worktree {
  id: WorktreeId;
  projectId: ProjectId;
  branchId: BranchId;
  name: string;
  path: string;
  isPrimary: boolean;
  /** Sessions on this worktree whose status is not "exited".
   *  NEVER rendered as a number — docs/AGENTIC-UX.md §10 rule 10 bans counts outside the
   *  queue, and Phase 2 has no queue. It decides one thing: whether the worktree row draws
   *  an expand chevron. See §7.6. */
  sessionCount: number;
  /** LANDED. §10.2 was requested and shipped: `catalog.go`'s `worktreeJSON` carries
   *  `Ready bool` with no `omitempty`, so the field is ALWAYS on the wire as true or
   *  false — never absent, never null — and `PATCH …/worktrees/{id}` with
   *  `{"ready": bool}` persists it through `registry.WorktreeFlagStore`. So task 22's
   *  contingency is resolved: the "Mark ready" verb IS rendered, and task 22 does not
   *  need to check which world it is in. The type stays optional because that accepts
   *  an always-present boolean and is the safe direction; it is not a signal that the
   *  field might be absent. Amended 2026-09-23 after task 15 measured the Go side. */
  ready?: boolean;
}

/* ── harnesses ─────────────────────────────────────────────────────────────────────── */

/** The five ids the catalogue ships today. The `(string & {})` arm keeps the union open so a
 *  sixth harness added server-side does not fail to parse, while preserving autocomplete. */
export type KnownHarnessId =
  "claude-code" | "codex" | "opencode" | "gemini-cli" | "shell";
export type HarnessId = KnownHarnessId | (string & {});

/** Server-driven. Never a hard-coded list in the cockpit. */
export type ModelId = string;

/** "" means "send no flag", which is the harness's own default. It is a legal value, not
 *  "unset" — which is why the inheritance functions in §3.6 use `undefined` for unset. */
export type Effort = "" | "low" | "medium" | "high" | "xhigh";

export type PermissionMode = "ask" | "bypass" | "plan";

export interface HarnessModel {
  id: ModelId;
  name: string;
}

export interface Harness {
  id: HarnessId;
  name: string;
  /** e.g. "claude", "/bin/bash". Rendered in the pane footer; never used to build argv — the
   *  control plane owns argv construction (contract-go.md §2.9). */
  command: string;
  installed: boolean;
  flagsVerified: boolean;
  /** installed && flagsVerified, computed server-side. The cockpit never re-derives it. */
  available: boolean;
  models: HarnessModel[];
  defaultModel: ModelId;
  efforts: Effort[];
  permissionModes: PermissionMode[];
  supportsOpeningPrompt: boolean;
  supportsResume: boolean;
  supportsCustomModel?: boolean;
  dockerAskNotice?: string;
  /** Why it is unavailable, or what it is. Rendered verbatim in the picker. */
  note: string;
}

/* ── sessions ──────────────────────────────────────────────────────────────────────── */

/** Exactly four. docs/AGENTIC-UX.md §4 rule 8 specifies seven; the other five need the
 *  agent-runtime signal that document calls a hard runtime dependency, and deriving them
 *  would require reading PTY bytes, which docs/PROGRAM.md forbids absolutely. */
export type SessionStatus = "starting" | "running" | "idle" | "exited";

/** Exact saved revisions and stable task IDs requested for an implementation session. */
export interface ImplementationContextRequest {
  projectId: ProjectId;
  briefRevision: number;
  planRevision: number;
  specificationRevision: number;
  designPlanRevision: number;
  taskIds: string[];
}

/** Immutable server snapshot. Session activity never establishes task completion or verification. */
export interface ImplementationContext extends ImplementationContextRequest {
  briefDigest: string;
  planDigest: string;
  specificationDigest: string;
  designPlanDigest: string;
  worktreeId: WorktreeId;
  sourceCommit: string;
  sourceDigest: string;
  sourceDirty: boolean;
  taskTitles: string[];
  requirementIds: string[];
}

export interface Session {
  id: SessionId;
  title: string;
  machineId: MachineId;
  projectId: ProjectId;
  branchId: BranchId;
  worktreeId: WorktreeId;
  harness: HarnessId;
  model: ModelId;
  effort: Effort;
  permissionMode: PermissionMode;
  openingPrompt: string;
  status: SessionStatus;
  /** Official account sign-in only; contains no implementation prompt. */
  authOnly?: boolean;
  /** Recorded subscription use cannot fall back to an API key on restart. */
  authMode?: "chatgpt";
  agentConnectionId?: string;
  agentConnectionRevision?: number;
  agentConnectionKind?: string;
  agentConnectionProtocol?: string;
  implementationContext?: ImplementationContext;
  projectRunId?: string;
  projectRunOperationId?: string;
  /** Free text for the user: "killed", "process exited", "idle timeout",
   *  "sandbox disappeared while the app was not running". Rendered verbatim. */
  exitReason: string;
  backend: string;
  isolation: Isolation;
  /** RFC 3339 with a Z offset. Kept as strings; parsed only at the point of display. */
  createdAt: string;
  updatedAt: string;
  lastOutputAt: string;
  attachCount: number;
  /** "/v1/sessions/s-3f2a91c0/attach". The client appends it to the ws base; it never
   *  reassembles the path from the id. See §4.6. */
  attachPath: string;
  /** Opaque provenance handle from Phase 4. Stored, carried, never interpreted. */
  contextRef: string;
}

/* ── client-only types ─────────────────────────────────────────────────────────────── */

/** The Develop lens has two layouts. docs/FUNCTIONAL-SPEC.md "Navigation". */
export type Layout = "columns" | "canvas";

/** The six lens rows of the sub-rail. No order, no numbering, no completion state —
 *  docs/AGENTIC-UX.md §4 rule 12. Only "develop" has a surface in Phase 2. */
export type Lens =
  "discover" | "design" | "develop" | "files" | "merge" | "deploy";

/** A hand-placed Canvas tile. Persisted per session — known defect 6. */
export interface TilePosition {
  x: number;
  y: number;
  w: number;
  h: number;
}

/** What the `＋` button starts without asking, and what the picker opens pre-filled.
 *  `effort: ""` means "send no flag"; see Effort above. */
export interface SessionDefaults {
  harness: HarnessId;
  model: ModelId;
  effort: Effort;
  permissionMode: PermissionMode;
  agentConnectionId?: string;
}

/** A partial override recorded against a project or a branch. Every field optional: an
 *  absent key inherits, it does not reset. */
export type SessionDefaultsOverride = Partial<SessionDefaults>;

export interface Selection {
  projectId: ProjectId | null;
  branchId: BranchId | null;
  worktreeId: WorktreeId | null;
  sessionId: SessionId | null;
}

/** Where a right-click landed, and on what. One menu component serves every row. */
export interface ContextMenuTarget {
  kind: "project" | "branch" | "worktree" | "session";
  id: string;
  x: number;
  y: number;
}

/** An optimistic mutation the user can still take back. docs/DESIGN-SYSTEM.md requires an
 *  Undo window rather than a confirm dialog for agent actions. */
export interface Toast {
  id: string;
  /** Plain text. No HTML — the prototype's `innerHTML` toasts are not carried forward. */
  message: string;
  undo?: () => void;
  /** ms. 6000 with an undo, 3200 without. */
  ttl: number;
  createdAt: number;
}
