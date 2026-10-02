/**
 * desktop/src/test/fixtures.ts
 *
 * Deterministic shared test data. Created by task 14, completed by task 15.
 *
 * Group D shared shape §3: the six typed builders below are declared ONCE, here, by task 15,
 * and every later task imports them. A task needing a VARIANT passes an override object to the
 * existing builder — it never adds a near-duplicate and never declares a local builder.
 */

import type {
  Branch,
  Harness,
  Machine,
  Project,
  Session,
  Worktree,
} from "../store/types";

/**
 * One fixed instant, so every elapsed-time assertion is deterministic.
 * 2026-09-18T09:00:00.000Z. Freeze with `vi.useFakeTimers()` then
 * `vi.setSystemTime(FIXED_NOW)`; this module imports nothing so it can be read from anywhere.
 */
export const FIXED_NOW = 1789722000000;
export const FIXED_NOW_ISO = "2026-09-18T09:00:00.000Z";

/**
 * Stable ids, shared so that a failure message in one suite reads the same as in another.
 * The shapes match the control plane's prefixed ids; task 15's `makeId` generates them for
 * real, and nothing here depends on that.
 */
export const IDS = {
  machine: "m-9c1d40a2",
  project: "p-1f4b8e37",
  branch: "b-7a2c5d91",
  worktree: "w-3e8f16b4",
  session: "s-3f2a91c0",
  sessionB: "s-8d4e72a5",
} as const;

// ── The typed fixture builders. Filled by task 15; reserved by task 14 ──────────────
//
// Group D shared shape §3.2: these SIX are the complete inventory and no other module
// declares a domain builder. A task needing a VARIANT passes an override; a task needing a
// SEVENTH surface adds it to this block and says so in its plan text.
//
// Every builder returns a fully-populated, valid object. Timestamps derive from FIXED_NOW
// and ids from IDS, so nothing here reads the wall clock.

const MINUTE_MS = 60_000;

/** An ISO timestamp `msBefore` milliseconds before the frozen clock. */
const before = (msBefore: number): string => new Date(FIXED_NOW - msBefore).toISOString();

export function aMachine(over: Partial<Machine> = {}): Machine {
  return {
    id: IDS.machine,
    name: "This computer",
    kind: "local",
    state: "running",
    backend: "docker",
    isolation: "container",
    default: true,
    limitations: ["Containers share the host kernel. Cloud machines add microVM isolation."],
    // `message` is deliberately absent: §2 says it is present only when state is "error",
    // so a populated one here would be an invalid machine.
    ...over,
  };
}

export function aProject(over: Partial<Project> = {}): Project {
  return {
    id: IDS.project,
    name: "luci",
    path: "C:\\work\\luci",
    ...over,
  };
}

export function aBranch(over: Partial<Branch> = {}): Branch {
  return {
    id: IDS.branch,
    projectId: IDS.project,
    name: "evidence-upload",
    isHead: false,
    ...over,
  };
}

export function aWorktree(over: Partial<Worktree> = {}): Worktree {
  return {
    id: IDS.worktree,
    projectId: IDS.project,
    branchId: IDS.branch,
    name: "evidence-upload",
    path: "C:\\work\\luci\\.worktrees\\evidence-upload",
    isPrimary: false,
    sessionCount: 1,
    // Always on the wire per §2's amendment: worktreeJSON's `Ready bool` has no omitempty.
    ready: false,
    ...over,
  };
}

export function aSession(over: Partial<Session> = {}): Session {
  return {
    id: IDS.session,
    title: "Evidence upload",
    machineId: IDS.machine,
    projectId: IDS.project,
    branchId: IDS.branch,
    worktreeId: IDS.worktree,
    harness: "claude-code",
    model: "claude-sonnet-4-5",
    effort: "",
    permissionMode: "ask",
    openingPrompt: "",
    status: "running",
    exitReason: "",
    backend: "docker",
    isolation: "container",
    createdAt: before(41 * MINUTE_MS),
    updatedAt: before(2 * MINUTE_MS),
    lastOutputAt: before(2 * MINUTE_MS),
    attachCount: 1,
    attachPath: `/v1/sessions/${IDS.session}/attach`,
    contextRef: "",
    ...over,
  };
}

/**
 * The SHELL descriptor, field for field as `harness.go`'s catalogue declares it, because it is
 * the one catalogue entry whose three arrays are legitimately empty — which is the shape shared
 * shape §3.2 pins: the Go side allocates unconditionally, so they are `[]` and never `null`.
 *
 * `note` is empty for the same reason it is `""` on the wire. `available` is
 * `installed && flagsVerified`, computed server-side, and no catalogue entry pairs an available
 * harness with a note: the two available entries (Claude Code, Shell) leave `Note` unset, and
 * only the three unavailable ones carry `unverifiedNote`. A default of available-with-a-note
 * would be a combination the control plane cannot emit.
 *
 * A test that needs the picker populated passes
 * `aHarness({ id: "claude-code", models: [...], efforts: [...], permissionModes: [...] })`; a test
 * that needs a note passes `aHarness({ installed: false, flagsVerified: false, available: false,
 * note: "..." })`, which is the shape the three unavailable entries actually have.
 */
export function aHarness(over: Partial<Harness> = {}): Harness {
  return {
    id: "shell",
    name: "Shell",
    command: "/bin/bash",
    installed: true,
    flagsVerified: true,
    available: true,
    models: [],
    defaultModel: "",
    efforts: [],
    permissionModes: [],
    supportsOpeningPrompt: false,
    supportsResume: false,
    note: "",
    ...over,
  };
}
