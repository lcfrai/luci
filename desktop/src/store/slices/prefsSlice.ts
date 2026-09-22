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

export const APP_DEFAULTS: SessionDefaults = {
  harness: "claude-code",
  model: "", // "" resolves to the harness's own defaultModel — store/defaults.ts
  effort: "",
  permissionMode: "ask",
};

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
  /** No-ops past CANVAS_MAX_TILES and pushes a toast naming the cap. */
  pinSession(id: SessionId): void;
  unpinSession(id: SessionId): void;
}

type PrefsState = Pick<
  PrefsSlice,
  "defaults" | "projectDefaults" | "branchDefaults" | "tiles" | "pinned"
>;

export const initialPrefsState: PrefsState = {
  defaults: APP_DEFAULTS,
  projectDefaults: {},
  branchDefaults: {},
  tiles: {},
  pinned: [],
};

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

  pinSession: (id) => {
    const { pinned, pushToast } = get();
    if (pinned.includes(id)) return;
    if (pinned.length >= CANVAS_MAX_TILES) {
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
