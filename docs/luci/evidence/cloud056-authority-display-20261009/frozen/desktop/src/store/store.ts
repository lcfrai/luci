// desktop/src/store/store.ts
//
// Contract §3.1 and §3.5. The store is FLAT: all three slices contribute keys at the root of
// one object. That is a correctness requirement, not a style. zustand's persist merges the
// rehydrated snapshot into the store with a SHALLOW merge at the root, so a nested
// { prefs: {...} } shape would silently drop every key added after the snapshot was written.

import { create } from "zustand";
import { createJSONStorage, persist } from "zustand/middleware";
import { createDataSlice, initialDataState, type DataSlice } from "./slices/dataSlice";
import { createUiSlice, initialUiState, type UiSlice } from "./slices/uiSlice";
import { createPrefsSlice, initialPrefsState, type PrefsSlice } from "./slices/prefsSlice";

export type CockpitState = DataSlice & UiSlice & PrefsSlice;

export const PERSIST_KEY = "luci.cockpit";
export const PERSIST_VERSION = 2;

/** What survives a restart. Everything else is re-fetched, because the control plane is the
 *  only truth about machines, projects, branches, worktrees and sessions. */
type Persisted = Pick<
  CockpitState,
  | "defaults" | "projectDefaults" | "branchDefaults" | "tiles" | "pinned"
  | "layout" | "railCollapsed"
  | "expandedProjects" | "expandedBranches" | "expandedWorktrees"
  | "selection" | "lens"
  | "sendChordsToTerminal"
>;

/** OD-9 added `sendChordsToTerminal` at schema version 1, with no bump and no migration. The
 *  root-level shallow merge above is why that is safe: a snapshot written before the key
 *  existed rehydrates with the key's initial value, false, which is the behaviour it was
 *  written under. store.test.ts pins that case. */
export const partialize = (s: CockpitState): Persisted => ({
  defaults: s.defaults,
  projectDefaults: s.projectDefaults,
  branchDefaults: s.branchDefaults,
  tiles: s.tiles,
  pinned: s.pinned,
  layout: s.layout,
  railCollapsed: s.railCollapsed,
  expandedProjects: s.expandedProjects,
  expandedBranches: s.expandedBranches,
  expandedWorktrees: s.expandedWorktrees,
  selection: s.selection,
  lens: s.lens,
  sendChordsToTerminal: s.sendChordsToTerminal,
});

/** Version 0 is any snapshot written before this contract existed — a pre-release dev build.
 *  It is dropped rather than translated: nothing shipped, so nothing is owed, and a wrong
 *  translation of a shape nobody documented is worse than a clean start. Version 2 adds the
 *  optional agentConnectionId inside defaults and overrides. Version 1 translates without
 *  changing values: missing IDs still inherit, empty IDs still select the personal route,
 *  and exact saved IDs remain exact. All other preferences survive unchanged.
 *
 *  This runs at module evaluation, before React mounts and before any error boundary exists,
 *  so it is TOTAL over `unknown`. A throw here is a white screen at launch, not a caught
 *  error, and the snapshot it chokes on is the one the user cannot delete without a devtools
 *  console they do not have in a packaged Tauri build. */
export function migrate(persisted: unknown, from: number): Partial<Persisted> {
  if (from < 1 || persisted === null || typeof persisted !== "object") return {};
  if (from === 1) return { ...persisted } as Partial<Persisted>;
  return persisted as Partial<Persisted>;
}

/** A fresh, independently persisted store. `useCockpit` is the app's one instance; the
 *  factory exists so a test can build a store over a seeded localStorage without disturbing
 *  the singleton every other suite shares. */
export const createCockpitStore = () =>
  create<CockpitState>()(
    persist(
      (...a) => ({
        ...createDataSlice(...a),
        ...createUiSlice(...a),
        ...createPrefsSlice(...a),
      }),
      {
        name: PERSIST_KEY,
        version: PERSIST_VERSION,
        storage: createJSONStorage(() => localStorage),
        partialize,
        migrate,
        // Contract §12, open question 3. The only report a swallowed storage failure gets.
        onRehydrateStorage: () => (_state: CockpitState | undefined, error: unknown) => {
          if (error !== undefined) {
            console.error(`[cockpit] failed to rehydrate ${PERSIST_KEY}:`, error);
          }
        },
      },
    ),
  );

export const useCockpit = createCockpitStore();

/** Restores every field to its initial value, leaving the action closures in place — the
 *  reason this merges rather than replacing. Clearing the snapshot is second, because the
 *  setState above writes one. Used by every component suite's beforeEach. */
export function resetCockpit(): void {
  useCockpit.setState({ ...initialDataState, ...initialUiState, ...initialPrefsState });
  useCockpit.persist.clearStorage();
}
