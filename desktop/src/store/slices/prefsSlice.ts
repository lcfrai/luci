// desktop/src/store/slices/prefsSlice.ts
//
// Contract §3.4. Defaults, inheritance overrides, tiles, pins.

import type { StateCreator } from "zustand";
import type {
  BranchId,
  ProjectId,
  SessionDefaults,
  SessionDefaultsOverride,
  SessionId,
  TilePosition,
} from "../types";
import type { CockpitState } from "../store";

/** docs/AGENTIC-UX.md §10 rule 5. */
export const CANVAS_MAX_TILES = 10;

export const APP_DEFAULTS: SessionDefaults = Object.freeze({
  harness: "claude-code",
  model: "", // "" resolves to the harness's own defaultModel — store/defaults.ts
  effort: "",
  permissionMode: "ask",
});

export interface PrefsSlice {
  /** App-level defaults. The bottom of the inheritance chain. */
  defaults: SessionDefaults;
  projectDefaults: Record<ProjectId, SessionDefaultsOverride>;
  branchDefaults: Record<BranchId, SessionDefaultsOverride>;
  /** Hand-placed Canvas tiles, keyed by session. Known defect 6. */
  tiles: Record<SessionId, TilePosition>;
  /** The Canvas working set, capped at CANVAS_MAX_TILES. Pin order, never re-sorted. */
  pinned: SessionId[];

  setDefaults(patch: SessionDefaultsOverride): void;
  setProjectDefaults(id: ProjectId, patch: SessionDefaultsOverride): void;
  setBranchDefaults(id: BranchId, patch: SessionDefaultsOverride): void;
  setTile(id: SessionId, tile: TilePosition): void;
  clearTile(id: SessionId): void;
  /** No-ops past CANVAS_MAX_TILES **within `scopeIds`** and pushes a toast naming the cap.
   *
   *  EDITED BY TASK 21's FIX ROUND 2, as a declared departure — this is task 17's file. The
   *  cap used to count `pinned` globally while Canvas renders one branch, so two branches of
   *  six sessions each left the second unable to seed, with a toast pointing at tiles that
   *  were not on screen. `scopeIds` is the set Canvas is currently showing from — the selected
   *  branch's pane order — and the cap counts `pinned ∩ scopeIds`.
   *
   *  A second parameter rather than re-keying `pinned` by branch: this changes a signature,
   *  not the persisted shape, so there is no migration at schema version 1 and a stored blob
   *  written by the previous build still rehydrates. It is REQUIRED rather than optional on
   *  purpose — an optional scope would silently restore the global cap for any caller that
   *  forgot it, which is the defect itself wearing a default value. */
  pinSession(id: SessionId, scopeIds: SessionId[]): void;
  unpinSession(id: SessionId): void;
}

type PrefsState = Pick<
  PrefsSlice,
  "defaults" | "projectDefaults" | "branchDefaults" | "tiles" | "pinned"
>;

/** FROZEN, containers and all — see the note on `initialDataState`. */
export const initialPrefsState: PrefsState = Object.freeze({
  defaults: APP_DEFAULTS,
  projectDefaults: Object.freeze({}) as Record<ProjectId, SessionDefaultsOverride>,
  branchDefaults: Object.freeze({}) as Record<BranchId, SessionDefaultsOverride>,
  tiles: Object.freeze({}) as Record<SessionId, TilePosition>,
  pinned: Object.freeze([]) as unknown as SessionId[],
});

/** Merges a patch into an override record. A key whose value is `undefined` is DELETED, not
 *  stored — clearing an override and inheriting again are the same act. */
function mergeOverride(
  base: SessionDefaultsOverride | undefined,
  patch: SessionDefaultsOverride,
): SessionDefaultsOverride {
  const next: SessionDefaultsOverride = { ...base };
  for (const key of Object.keys(patch) as (keyof SessionDefaultsOverride)[]) {
    if (patch[key] === undefined) delete next[key];
    else Object.assign(next, { [key]: patch[key] });
  }
  return next;
}

export const createPrefsSlice: StateCreator<
  CockpitState,
  [["zustand/persist", unknown]],
  [],
  PrefsSlice
> = (set, get) => ({
  ...initialPrefsState,

  // `defaults` stays a COMPLETE SessionDefaults, so an undefined in the patch is ignored
  // rather than punching a hole in the bottom of the inheritance chain.
  setDefaults: (patch) =>
    set((s) => ({
      defaults: {
        harness: patch.harness ?? s.defaults.harness,
        model: patch.model ?? s.defaults.model,
        effort: patch.effort ?? s.defaults.effort,
        permissionMode: patch.permissionMode ?? s.defaults.permissionMode,
      },
    })),

  setProjectDefaults: (id, patch) =>
    set((s) => ({
      projectDefaults: { ...s.projectDefaults, [id]: mergeOverride(s.projectDefaults[id], patch) },
    })),
  setBranchDefaults: (id, patch) =>
    set((s) => ({
      branchDefaults: { ...s.branchDefaults, [id]: mergeOverride(s.branchDefaults[id], patch) },
    })),

  setTile: (id, tile) => set((s) => ({ tiles: { ...s.tiles, [id]: tile } })),
  // Not `set({})` when there is nothing to clear: an empty partial still builds a new state
  // object and runs every subscriber. The only way to write nothing is not to call set.
  clearTile: (id) => {
    if (get().tiles[id] === undefined) return;
    set((s) => {
      const tiles = { ...s.tiles };
      delete tiles[id];
      return { tiles };
    });
  },

  pinSession: (id, scopeIds) => {
    const { pinned, pushToast } = get();
    if (pinned.includes(id)) return;
    // The cap is over what Canvas SHOWS, not over what the store holds. `pinned` stays a flat
    // global list and is allowed to hold a branch's worth of pins for every branch visited.
    const inScope = pinned.filter((p) => scopeIds.includes(p));
    if (inScope.length >= CANVAS_MAX_TILES) {
      pushToast(`Canvas holds ${CANVAS_MAX_TILES} sessions. Unpin one to add another.`);
      return;
    }
    set({ pinned: [...pinned, id] });
  },
  unpinSession: (id) => {
    if (!get().pinned.includes(id)) return;
    set((s) => ({ pinned: s.pinned.filter((x) => x !== id) }));
  },
});
