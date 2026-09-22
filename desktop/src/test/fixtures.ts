/**
 * desktop/src/test/fixtures.ts
 *
 * Deterministic shared test data. Created by task 14, completed by task 15.
 *
 * Group D shared shape §3: the six typed builders below are declared ONCE, here, by task 15,
 * and every later task imports them. A task needing a VARIANT passes an override object to the
 * existing builder — it never adds a near-duplicate and never declares a local builder.
 */

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

// ── RESERVED FOR TASK 15: the six typed fixture builders ─────────────────────
// Group D shared shape §3.1. TASK 15 fills this point and no other task does. Task 15 is the
// first task that CAN type a builder — it declares "../store/types" — and the last one that can
// before they are needed, because task 16 runs before task 17 and imports them for its client
// tests. A builder declared in task 17 does not exist when task 16 reaches for it.
//
// Task 15 adds exactly these six, each taking an optional partial override, importing its types
// from "../store/types":
//
//   export function aMachine(over?: Partial<Machine>): Machine { ... }
//   export function aProject(over?: Partial<Project>): Project { ... }
//   export function aBranch(over?: Partial<Branch>): Branch { ... }
//   export function aWorktree(over?: Partial<Worktree>): Worktree { ... }
//   export function aSession(over?: Partial<Session>): Session { ... }
//   export function aHarness(over?: Partial<Harness>): Harness { ... }
//
// The last one's type is `Harness`. Shared shape §3.2 writes that signature against a
// `HarnessDescriptor`, and nothing declares such a type: contract §2 calls the interface
// `Harness`, and task 15 transcribes §2 verbatim under a diff gate so it cannot rename it. The
// BUILDER name is `aHarness`, exactly as §3.2 fixes it; the type it takes and returns is
// `Harness`. Task 15 settles this the same way in its own step text.
//
// Each returns a FULLY POPULATED, valid object: timestamps from FIXED_NOW so no test depends on
// the wall clock, ids from IDS. aHarness() returns `models`, `efforts` and `permissionModes` as
// EMPTY ARRAYS, never null or undefined — shared shape §5.1: the Go side allocates them
// unconditionally and pins it with a marshalled-bytes test, so a fixture that disagrees would
// hide a real regression instead of exposing one.
//
// There is no seventh. A task that believes it needs one adds it HERE, in task 15, and says so
// in its plan text. No local builders and no second spelling: `aThing`, not `makeThingFixture`.
