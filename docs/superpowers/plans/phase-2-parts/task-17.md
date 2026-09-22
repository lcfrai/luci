### Task 17: The store: slices, persistence, inheritance

This is the last task in Group D's foundation that nothing on screen can be built without, and it is the one with the widest blast radius: tasks 18 through 24 all read it, and **shared-shape §4 makes its `selectors.ts` the only file in Group D where a derived read may be declared** — task 18 adds four selectors of its own, and adds them there, at the point step 10 reserves. A component that computes a derivation inline is not merely untidy — an inline `.filter()` inside `useCockpit(...)` returns a new array on every store write and therefore re-renders on every 2 s poll, which is precisely the layout shift `docs/AGENTIC-UX.md` §10 rule 7 bans. So the rule is enforced here by making the derived reads exist, named, in one file, and by giving the five whose absent case is an empty array three shared frozen constants, so that case is referentially free.

Some of this task looks smaller than it is. **Flatness.** Contract §3.1 makes all three slices contribute keys at the root of one object, and that is a correctness requirement rather than a style: zustand's `persist` merges the rehydrated snapshot into the store with a **shallow** merge at the root, so a nested `{ prefs: { … } }` shape silently drops every key added after the snapshot was written. A user who upgrades the app and loses their Canvas tiles would be looking at exactly that bug. Step 4 pins it with a test that rehydrates a snapshot missing a key and asserts the code default survives. **`migrate`.** A stored snapshot from an earlier shape is the only thing between a user and a wiped cockpit, and it is read at module-evaluation time — before React mounts, before any error boundary exists — so a `migrate` that throws on a malformed value is a white screen at launch, not a caught error. It is written to be total over `unknown`. **Purity.** §3.6 specifies `resolveDefaults` as a pure function, which is what lets seventeen inheritance and clamping cases be tested with no store, no React and no fake timers; `defaults.ts` therefore imports no store and holds no state, and the one import it does have is `import type`, erased at build.

The clamp inside `resolveDefaults` earns its keep against a real failure. A branch default recorded six weeks ago can name a harness that has since been uninstalled or a model that harness no longer offers, and the control plane answers `400 unknown_harness` or `400 unsupported_option`. Clamping client-side turns a create that would have failed into one that succeeds with the nearest legal choice, and `toCreateRequest` then strips every field the chosen harness does not accept — a 400 the client could have avoided is a defect in the client.

Two facts from Groups B and C override the contract here, and both land in this task. `GET /v1/harnesses` returns `[]` and never `null` for a harness with no models, efforts or permission modes — task 8's fix round made the Go side allocate unconditionally and pinned it with a marshalled-bytes test — so `clampModel` reads `h.models.length` with no `?? []` and the fields are not described as nullable. And the error-code enum has **sixteen** members, not fifteen, because Group C's ruling 7 added `non_loopback_publish`; this task needs no change for it, and that is the point — `lastError.code` is typed `string`, not a code union, so the sixteenth code needs no store edit and no exhaustive switch lives here.

**Files:**
- `desktop/src/store/slices/dataSlice.ts` — contract §3.2's `DataSlice`, `LoadKey`, `initialDataState`, and the identity-preserving helpers `sameSession` and `sameIds` (new)
- `desktop/src/store/slices/uiSlice.ts` — contract §3.3's `UiSlice`, `initialUiState`, the toast constants, and the reserved insertion point for task 18's `machinesOpen` (new)
- `desktop/src/store/slices/prefsSlice.ts` — contract §3.4's `PrefsSlice`, `initialPrefsState`, `CANVAS_MAX_TILES`, `APP_DEFAULTS` (new)
- `desktop/src/store/store.ts` — `CockpitState`, `createCockpitStore`, the `useCockpit` singleton, `resetCockpit`, `PERSIST_KEY`, `PERSIST_VERSION`, `partialize`, `migrate`, and the `onRehydrateStorage` error log contract §12 open question 3 assigns to this task (new)
- `desktop/src/store/defaults.ts` — `resolveDefaults` and `toCreateRequest`, pure, no store import (new)
- `desktop/src/store/selectors.ts` — contract §3.7's ten selectors, plus the reserved insertion point for the selectors task 18 adds — `selectProjectById`, `selectBranchById`, `selectWorktreeById` and `selectBranchAttentionSessionId` (new)
- `desktop/src/store/defaults.test.ts` — the merge, the four clamps and `toCreateRequest`'s ten-row inclusion table (new)
- `desktop/src/store/store.test.ts` — all three slices, the persist partition, `migrate` and `resetCockpit` (new)
- `desktop/src/store/selectors.test.ts` — every selector, and the referential stability the three shared empty constants exist to provide (new)
- `desktop/package.json` and `desktop/package-lock.json` — re-assert `zustand@5.0.15` with `--save-exact`; normally a no-op because task 14 installs it, in which case neither file changes and step 12 stages neither (modify, conditional)

**Interfaces:**
- Produces:
  - `store/store` — `CockpitState`, `createCockpitStore()`, `useCockpit`, `resetCockpit()`, `PERSIST_KEY`, `PERSIST_VERSION`, `partialize`, `migrate`. The `onRehydrateStorage` log contract §12's open question 3 assigns to this task is an inline option of `persist`, not an export.
  - `store/slices/dataSlice` — `DataSlice`, `LoadKey`, `createDataSlice`, `initialDataState`, `sameSession`, `sameIds`
  - `store/slices/uiSlice` — `UiSlice`, `createUiSlice`, `initialUiState`, `TOAST_TTL_UNDO`, `TOAST_TTL_PLAIN`, `MAX_TOASTS`
  - `store/slices/prefsSlice` — `PrefsSlice`, `createPrefsSlice`, `initialPrefsState`, `CANVAS_MAX_TILES`, `APP_DEFAULTS`
  - `store/defaults` — `resolveDefaults`, `toCreateRequest`, `clampToCatalogue`
  - `store/selectors` — `selectDefaultMachine`, `selectBranches`, `selectWorktrees`, `selectSessionIdsForWorktree`, `selectSession`, `selectPaneSessionIds`, `selectCanvasSessionIds`, `selectHarness`, `selectResolvedDefaults`, `selectFocusedSessionId`
- Consumes: task 14's `zustand@5.0.15`, `vitest.config.ts` and `setup.ts`; task 15's `store/types.ts`, `lib/ids.ts` (`makeId`) and the six typed fixture builders task 15 adds to task 14's `desktop/src/test/fixtures.ts` — `aMachine`, `aProject`, `aBranch`, `aWorktree`, `aSession`, `aHarness`; task 16's `CreateSessionRequest` **type only**. Nothing from tasks 18 to 24.
- Required by: task 18 (`useCockpit`, `resetCockpit`, every §3.7 selector, `resolveDefaults`, `toCreateRequest`, `CANVAS_MAX_TILES`; it also adds `machinesOpen` to `uiSlice.ts` and `selectProjectById`, `selectBranchById`, `selectWorktreeById` and `selectBranchAttentionSessionId` to `selectors.ts`, at the insertion points this task reserves), task 19 (`useCockpit`, `selectSession`), tasks 20 to 24 (the selectors and the ui actions).
- Amends contract §9: task 17 additionally depends on **task 16**. §3.6 fixes `toCreateRequest`'s return type as `CreateSessionRequest` and §4.3 declares that interface in `api/client.ts`; §9's dependency column lists only task 15. The import is `import type`, so it is erased at build and `defaults.ts` stays pure with no runtime edge to the client. Step 1 checks for it and stops rather than declaring a second copy — a structural twin of a shape another file already owns is the Group C defect class.
- Amends contract §3.7: `selectResolvedDefaults` is consumed with `useShallow`, not bare. §3.7 names `useShallow` only for the two array selectors, but `resolveDefaults` returns a freshly built `SessionDefaults` object on every call, so under zustand 5's default `Object.is` it re-renders its consumer on every 2 s poll exactly as an unshelled array would. Argued in step 10.
- Amends contract §3.2: `lastError` is cleared only by an explicit `setError(null)`, never as a side effect of a successful `set*`. Argued in step 5.
- Adds symbols §3.3 does not list — `TOAST_TTL_UNDO`, `TOAST_TTL_PLAIN` and `MAX_TOASTS` — so task 18's `Toasts` reads the TTLs rather than re-deriving them from `types.ts`'s doc comment. Additive; changes no signature §3.3 fixes.
- **Declares no fixture builder.** Shared-shape §3.1 puts all six in task 15, because task 16 runs before this task and imports them; a builder declared here would not exist when task 16 needs it. §3.2 sanctions six names and no seventh, so the three-harness catalogue these suites reason about is composed **inside each test file** from `aHarness(...)` overrides. Every harness field an assertion depends on is written out at that composition site, so no assertion here rests on a default task 15 chose.

- [ ] **Step 1: Preflight — confirm the substrate, and stop rather than patch another task's file**

Tasks 14, 15 and 16 own everything this task builds on. If any check below fails, **stop and report it to the orchestrator.** Adding the missing symbol locally is how Group C produced nine Criticals: two authors wrote the same thing differently and both suites went green.

Every fenced block in this plan starts at the **repository root**; the blocks that work inside the app open with `cd desktop`.

```bash
cd desktop
ls src/store/types.ts src/api/client.ts src/lib/ids.ts src/test/fixtures.ts vitest.config.ts
grep -c '"zustand": "5.0.15"' package.json
grep -cE '^export interface CreateSessionRequest' src/api/client.ts
grep -cE '^export function makeId' src/lib/ids.ts
grep -cE '^export .*FIXED_NOW_ISO' src/test/fixtures.ts
grep -cE '^export function (aMachine|aProject|aBranch|aWorktree|aSession|aHarness)\(' src/test/fixtures.ts
```

`ls` must print every path it was given and no `No such file or directory`. Every `grep` line but the last must print a number ≥ 1; **the last must print exactly `6`** — the six typed builders shared-shape §3.2 sanctions, which task 15 adds to task 14's file.

Every grep over a TypeScript file above is anchored at the first character of the line, per shared-shape §3A. That is not decoration here: task 14's reservation comment in `fixtures.ts` names builders inside a `//` comment, so an unanchored count is satisfied by prose and tells you nothing about the code. The `package.json` line is the one that is not anchored and does not need to be — JSON carries no comments, so the only thing `"zustand": "5.0.15"` can match is the dependency entry itself.

A `0` from the `CreateSessionRequest` line means task 16 has not landed; that is the one this task cannot work around, because the alternative is a second declaration of a shape `api/client.ts` owns. Anything other than `6` from the last line means task 15 has not landed the typed builders — `aMachine`, `aProject`, `aBranch`, `aWorktree`, `aSession`, `aHarness` — and **this task stops rather than declaring its own.** Shared-shape §3.1 is explicit about why: task 16 runs before this task and imports them, so a builder created here does not exist when task 16 needs it. That one omission produced six of Group D's round-one Criticals.

Then re-assert the dependency. Task 14 installs `zustand@5.0.15`, and contract §9 also marks `package.json` modified in this task's row, which is a genuine conflict in the contract; re-asserting is the safe direction, because it is idempotent when task 14 did its job and recovers the task if it did not:

```bash
cd desktop
npm install --save-exact zustand@5.0.15
```

`--save-exact` is load-bearing. npm's default save-prefix is `^`, so a bare `npm install zustand@5.0.15` rewrites task 14's exact `"zustand": "5.0.15"` to `"^5.0.15"` — which falsifies this step's own `grep -c '"zustand": "5.0.15"'` guard and would stage an un-pinned range.

Expect either `up to date` or `changed 1 package`, and **no `npm warn ERESOLVE`**. If this line actually installs something, `package.json` and `package-lock.json` are modified and step 12 stages both; if it prints `up to date`, they are not modified and step 12 stages neither.

- [ ] **Step 2: Write the failing tests for inheritance and the create request**

`resolveDefaults` is pure, so this suite needs no store, no React and no timers — that is the whole reason §3.6 specifies it as a function rather than a store action, and it is why this is the first thing built.

Create `desktop/src/store/defaults.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { aHarness } from "../test/fixtures";
import { resolveDefaults, toCreateRequest } from "./defaults";
import type { Harness, SessionDefaults } from "./types";

/* The catalogue this suite reasons about, composed HERE from task 15's `aHarness` rather than
 * added to `fixtures.ts`: shared-shape §3.2 sanctions six builders and no seventh, and a
 * catalogue is a composition of one of them rather than a new one. Every field an assertion
 * below depends on is written out here, so nothing rests on a default task 15 chose.
 *
 * A full harness, a bare one whose option arrays are all empty, and an installed-but-unverified
 * one that is NOT available. The empty arrays are `[]` and never `null` — the Go side allocates
 * unconditionally (task 8's fix round), so nothing here needs a `?? []`. */
const claude = aHarness({
  id: "claude-code",
  name: "Claude Code",
  command: "claude",
  installed: true,
  flagsVerified: true,
  available: true,
  models: [
    { id: "opus-4.8", name: "Opus 4.8" },
    { id: "sonnet-4.8", name: "Sonnet 4.8" },
  ],
  defaultModel: "sonnet-4.8",
  efforts: ["", "low", "medium", "high"],
  permissionModes: ["ask", "bypass", "plan"],
  supportsOpeningPrompt: true,
  supportsResume: true,
  note: "",
});

const shell = aHarness({
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
  note: "A plain shell in the sandbox.",
});

const codex = aHarness({
  id: "codex",
  name: "Codex",
  command: "codex",
  installed: true,
  flagsVerified: false,
  available: false,
  models: [{ id: "gpt-5-codex", name: "GPT-5 Codex" }],
  defaultModel: "gpt-5-codex",
  efforts: ["low", "high"],
  permissionModes: ["ask"],
  supportsOpeningPrompt: true,
  supportsResume: true,
  note: "Installed, but its flags have not been verified on this machine.",
});

const catalogue: Harness[] = [claude, shell, codex];

/** The literal value of `APP_DEFAULTS` (contract §3.4), written out rather than imported
 *  from `./slices/prefsSlice`. This suite is the proof that §3.6's function is pure, so it
 *  imports exactly one module of production code; step 4's store suite asserts the real
 *  constant equals this literal, which is what keeps the two from drifting. */
const APP: SessionDefaults = {
  harness: "claude-code",
  model: "",
  effort: "",
  permissionMode: "ask",
};

const full: SessionDefaults = {
  harness: "claude-code",
  model: "opus-4.8",
  effort: "high",
  permissionMode: "plan",
};

describe("resolveDefaults — the merge", () => {
  it("lets a branch override beat a project override beat the app default", () => {
    const r = resolveDefaults(full, { model: "sonnet-4.8" }, { effort: "low" }, catalogue);
    expect(r).toEqual({
      harness: "claude-code",
      model: "sonnet-4.8",
      effort: "low",
      permissionMode: "plan",
    });
  });

  it("inherits an absent key rather than resetting it", () => {
    const r = resolveDefaults(full, {}, {}, catalogue);
    expect(r).toEqual(full);
  });

  it("treats a key present with an undefined value as absent, not as a reset", () => {
    const r = resolveDefaults(full, { effort: undefined }, undefined, catalogue);
    expect(r.effort).toBe("high");
  });

  it('honours an explicit "" effort, because "" means send no flag', () => {
    const r = resolveDefaults(full, undefined, { effort: "" }, catalogue);
    expect(r.effort).toBe("");
  });
});
```

Append the clamp block to the same file:

```ts
describe("resolveDefaults — the clamp, in §3.6's order", () => {
  it("falls back to claude-code when the named harness is not in the catalogue", () => {
    const r = resolveDefaults({ ...full, harness: "gone" }, undefined, undefined, catalogue);
    expect(r.harness).toBe("claude-code");
  });

  it("falls back to claude-code when the named harness is present but unavailable", () => {
    const r = resolveDefaults({ ...full, harness: "codex" }, undefined, undefined, catalogue);
    expect(r.harness).toBe("claude-code");
  });

  it("falls back to the first available harness when claude-code is unavailable", () => {
    const degraded: Harness[] = [
      aHarness({ id: "claude-code", available: false }),
      aHarness({ id: "opencode", name: "OpenCode", available: true, defaultModel: "sonnet-4.8" }),
    ];
    const r = resolveDefaults(full, undefined, undefined, degraded);
    expect(r.harness).toBe("opencode");
  });

  it("falls back to shell when nothing in the catalogue is available", () => {
    const none = catalogue.map((h) => ({ ...h, available: false }));
    const r = resolveDefaults(full, undefined, undefined, none);
    expect(r.harness).toBe("shell");
  });

  it("falls back to shell and sends nothing optional when the catalogue is empty", () => {
    const r = resolveDefaults(full, undefined, undefined, []);
    expect(r).toEqual({ harness: "shell", model: "", effort: "", permissionMode: "ask" });
  });

  it("replaces a model the harness no longer offers with its defaultModel", () => {
    const r = resolveDefaults({ ...full, model: "opus-3" }, undefined, undefined, catalogue);
    expect(r.model).toBe(claude.defaultModel);
  });

  it("resolves an empty model to the harness defaultModel, as APP_DEFAULTS relies on", () => {
    const r = resolveDefaults(APP, undefined, undefined, catalogue);
    expect(r.model).toBe("sonnet-4.8");
  });

  it("empties the model when the harness has no models", () => {
    const r = resolveDefaults({ ...full, harness: "shell" }, undefined, undefined, catalogue);
    expect(r.model).toBe("");
  });

  it("empties an effort the harness does not list", () => {
    const only = [aHarness({ id: "claude-code", available: true, efforts: ["low", "medium"] })];
    const r = resolveDefaults(full, undefined, undefined, only);
    expect(r.effort).toBe("");
  });

  it("empties the effort when the harness lists none", () => {
    const r = resolveDefaults({ ...full, harness: "shell" }, undefined, undefined, catalogue);
    expect(r.effort).toBe("");
  });

  it("replaces an unlisted permission mode with the first the harness lists", () => {
    const only = [
      aHarness({ id: "claude-code", available: true, permissionModes: ["bypass", "ask"] }),
    ];
    const r = resolveDefaults(full, undefined, undefined, only);
    expect(r.permissionMode).toBe("bypass");
  });

  it('falls back to "ask" when the harness lists no permission modes', () => {
    const r = resolveDefaults({ ...full, harness: "shell" }, undefined, undefined, catalogue);
    expect(r.permissionMode).toBe("ask");
  });

  it("returns a fresh object and never mutates its inputs", () => {
    const app = { ...APP };
    const project = { model: "opus-4.8" };
    const r = resolveDefaults(app, project, undefined, catalogue);
    expect(r).not.toBe(app);
    expect(app).toEqual(APP);
    expect(project).toEqual({ model: "opus-4.8" });
  });
});
```

And the `toCreateRequest` block, which walks §3.6's inclusion table row by row:

```ts
describe("toCreateRequest — §3.6's inclusion table", () => {
  const resolved = resolveDefaults(full, undefined, undefined, catalogue);

  it("always carries worktreeId and harness, and never machineId", () => {
    const req = toCreateRequest("w-1", resolved, claude);
    expect(req.worktreeId).toBe("w-1");
    expect(req.harness).toBe("claude-code");
    expect("machineId" in req).toBe(false);
  });

  it("omits model and effort for a harness that offers neither", () => {
    const d = resolveDefaults({ ...full, harness: "shell" }, undefined, undefined, catalogue);
    const req = toCreateRequest("w-1", d, shell);
    expect("model" in req).toBe(false);
    expect("effort" in req).toBe(false);
    expect("permissionMode" in req).toBe(false);
  });

  it('omits effort when the value is "" even though the harness lists efforts', () => {
    const req = toCreateRequest("w-1", { ...resolved, effort: "" }, claude);
    expect("effort" in req).toBe(false);
  });

  it("includes permissionMode whenever the harness lists any", () => {
    const req = toCreateRequest("w-1", resolved, claude);
    expect(req.permissionMode).toBe("plan");
  });

  it("trims an opening prompt and omits one that is only whitespace", () => {
    expect(toCreateRequest("w-1", resolved, claude, { openingPrompt: "  fix it  " }).openingPrompt)
      .toBe("fix it");
    expect("openingPrompt" in toCreateRequest("w-1", resolved, claude, { openingPrompt: "   " }))
      .toBe(false);
  });

  it("omits the opening prompt for a harness that does not support one", () => {
    const d = resolveDefaults({ ...full, harness: "shell" }, undefined, undefined, catalogue);
    const req = toCreateRequest("w-1", d, shell, { openingPrompt: "hello" });
    expect("openingPrompt" in req).toBe(false);
  });

  it("omits title unless the user typed one, so the server derives it", () => {
    expect("title" in toCreateRequest("w-1", resolved, claude)).toBe(false);
    expect("title" in toCreateRequest("w-1", resolved, claude, { title: "  " })).toBe(false);
    expect(toCreateRequest("w-1", resolved, claude, { title: " Audit " }).title).toBe("Audit");
  });

  it("carries resume only for a harness that supports it", () => {
    expect(toCreateRequest("w-1", resolved, claude, { resume: "abc" }).resume).toBe("abc");
    const d = resolveDefaults({ ...full, harness: "shell" }, undefined, undefined, catalogue);
    expect("resume" in toCreateRequest("w-1", d, shell, { resume: "abc" })).toBe(false);
  });

  it("carries contextRef only when the caller passes one", () => {
    expect("contextRef" in toCreateRequest("w-1", resolved, claude)).toBe(false);
    expect(toCreateRequest("w-1", resolved, claude, { contextRef: "ev-9" }).contextRef).toBe("ev-9");
  });

  it("sends the descriptor's id, so a mismatched descriptor cannot be sent", () => {
    const req = toCreateRequest("w-1", { ...resolved, harness: "codex" }, claude);
    expect(req.harness).toBe("claude-code");
  });
});
```

Run it and watch it fail:

```bash
cd desktop
npm test -- src/store/defaults.test.ts
```

**What you must see:** the run fails at import time, naming `src/store/defaults.test.ts` and the unresolved import `./defaults`, with a message of the form `Failed to load url ./defaults … Does the file exist?`, and **zero tests collected**. Vitest 5 is not installed in this tree at the time of writing, so the reporter chrome around that line is approximate and nothing here was transcribed from a run; what is not negotiable is that the failure is the missing module and not a passing suite. `./defaults` is the only production module this suite imports at runtime — `./types` is an `import type`, erased before the resolver ever sees it — so it is the only name that can appear in that message; `../test/fixtures` appearing in it instead means task 15's builders are not where step 1 checked for them, and the answer is to stop, not to declare one here.

- [ ] **Step 3: Implement `defaults.ts` — the merge, the four clamps, the create request**

Decisions the contract does not make, and that a later reader would otherwise get wrong.

**(a) The merge is field-by-field, never a spread.** `{ ...base, ...patch }` copies a key that is *present with an `undefined` value* and clobbers the inherited value with `undefined`, which then fails `strict` at the assignment or, worse, ships an `undefined` harness to the server. JSON cannot carry `undefined`, so a rehydrated override is safe — but `setProjectDefaults(id, { model: undefined })` is the documented way to clear an override (step 7), and that object reaches this function in memory. `??` per field is total over both cases and costs four lines.

**(b) When the clamped harness has no descriptor in the catalogue, every optional field is emptied.** That happens only when nothing is available and the fallback lands on the literal `"shell"` with no catalogue row to read. Sending a model or an effort we cannot check against anything is how a create earns `400 unsupported_option`, so the request carries the harness and nothing else. `shell` is always available — the sandbox image has bash — so this is a degraded catalogue, not a degraded machine.

Create `desktop/src/store/defaults.ts`:

```ts
// desktop/src/store/defaults.ts
//
// Contract §3.6. Pure: no store import, no state, no clock. The one import is `import type`,
// erased at build, so this module has no runtime edge to the API client.

import type { CreateSessionRequest } from "../api/client";
import type {
  Effort,
  Harness,
  HarnessId,
  ModelId,
  PermissionMode,
  SessionDefaults,
  SessionDefaultsOverride,
  WorktreeId,
} from "./types";

/** Preferred when the recorded harness is gone or unavailable. */
const PREFERRED_HARNESS: HarnessId = "claude-code";
/** The one harness that is always there: the sandbox image has bash. */
const LAST_RESORT_HARNESS: HarnessId = "shell";

/** Later wins, per field. An ABSENT key inherits and never resets, and a key present with an
 *  `undefined` value counts as absent — see decision (a). `""` is a value, not "unset", so it
 *  overrides: for `effort`, `""` means "send no flag". */
function applyOverride(
  base: SessionDefaults,
  patch: SessionDefaultsOverride | undefined,
): SessionDefaults {
  if (patch === undefined) return base;
  return {
    harness: patch.harness ?? base.harness,
    model: patch.model ?? base.model,
    effort: patch.effort ?? base.effort,
    permissionMode: patch.permissionMode ?? base.permissionMode,
  };
}

function clampHarness(id: HarnessId, harnesses: readonly Harness[]): HarnessId {
  const chosen = harnesses.find((h) => h.id === id);
  if (chosen !== undefined && chosen.available) return chosen.id;
  const preferred = harnesses.find((h) => h.id === PREFERRED_HARNESS && h.available);
  if (preferred !== undefined) return preferred.id;
  const firstAvailable = harnesses.find((h) => h.available);
  if (firstAvailable !== undefined) return firstAvailable.id;
  return LAST_RESORT_HARNESS;
}

/** `models` is `[]` and never `null` — task 8's fix round made the Go side allocate
 *  unconditionally — so `.length` is always safe and no `?? []` belongs here. */
function clampModel(model: ModelId, h: Harness): ModelId {
  if (h.models.length === 0) return "";
  if (h.models.some((m) => m.id === model)) return model;
  return h.defaultModel;
}

function clampEffort(effort: Effort, h: Harness): Effort {
  if (h.efforts.length === 0) return "";
  if (h.efforts.includes(effort)) return effort;
  return "";
}

function clampPermissionMode(mode: PermissionMode, h: Harness): PermissionMode {
  if (h.permissionModes.length === 0) return "ask";
  if (h.permissionModes.includes(mode)) return mode;
  return h.permissionModes[0];
}
```

Append the exported functions to the same file:

```ts
/** Clamps a merged set against the live catalogue, in §3.6's order: harness, then model,
 *  then effort, then permissionMode. The order matters — every field after the first is
 *  checked against the harness the first step chose, not the one that was asked for. */
export function clampToCatalogue(
  d: SessionDefaults,
  harnesses: readonly Harness[],
): SessionDefaults {
  const harness = clampHarness(d.harness, harnesses);
  const h = harnesses.find((x) => x.id === harness);
  if (h === undefined) return { harness, model: "", effort: "", permissionMode: "ask" };
  return {
    harness,
    model: clampModel(d.model, h),
    effort: clampEffort(d.effort, h),
    permissionMode: clampPermissionMode(d.permissionMode, h),
  };
}

/** §3.6. project → branch → session, then clamped. Pure, and it mutates none of its inputs. */
export function resolveDefaults(
  app: SessionDefaults,
  project: SessionDefaultsOverride | undefined,
  branch: SessionDefaultsOverride | undefined,
  harnesses: readonly Harness[],
): SessionDefaults {
  return clampToCatalogue(applyOverride(applyOverride(app, project), branch), harnesses);
}

/** §3.6's inclusion table, exhaustively. `harness` is taken from the DESCRIPTOR rather than
 *  from `d`, so a caller that passes a descriptor for a different harness cannot send a body
 *  whose flags and harness disagree — the one mismatch that would produce
 *  400 unsupported_option with a body that looks correct. `machineId` is never sent: the Go
 *  contract defaults it to "local" and Phase 2 has one machine. */
export function toCreateRequest(
  worktreeId: WorktreeId,
  d: SessionDefaults,
  harness: Harness,
  opts: {
    openingPrompt?: string;
    title?: string;
    resume?: string;
    contextRef?: string;
  } = {},
): CreateSessionRequest {
  const req: CreateSessionRequest = { worktreeId, harness: harness.id };
  if (harness.models.length > 0 && d.model !== "") req.model = d.model;
  if (harness.efforts.length > 0 && d.effort !== "") req.effort = d.effort;
  if (harness.permissionModes.length > 0) req.permissionMode = d.permissionMode;

  const prompt = (opts.openingPrompt ?? "").trim();
  if (harness.supportsOpeningPrompt && prompt !== "") req.openingPrompt = prompt;

  const title = (opts.title ?? "").trim();
  if (title !== "") req.title = title;

  if (harness.supportsResume && opts.resume !== undefined && opts.resume !== "") {
    req.resume = opts.resume;
  }
  if (opts.contextRef !== undefined && opts.contextRef !== "") req.contextRef = opts.contextRef;
  return req;
}
```

Run it:

```bash
cd desktop
npm test -- src/store/defaults.test.ts
npm run typecheck
```

**What you must see:** all twenty-seven tests in the three `describe` blocks pass, and `tsc` exits 0. The failures worth predicting are the ones a transcription error produces. If *treats a key present with an undefined value as absent* fails with `AssertionError: expected undefined to be 'high'`, `applyOverride` was written as a spread instead of four `??` lines. If *falls back to shell and sends nothing optional when the catalogue is empty* fails on `model` rather than on `harness`, the `h === undefined` guard in `clampToCatalogue` is missing and the code is reading a descriptor that does not exist.

- [ ] **Step 4: Write the failing store suite**

One file covers all three slices, the persist partition and `migrate`, because they are one object and the interesting behaviour is at the seams between them: `pinSession` pushes a toast, `removeSession` clears a tile. Create `desktop/src/store/store.test.ts`:

```ts
import { beforeEach, describe, expect, it, vi } from "vitest";
import { aBranch, aHarness, aProject, aSession } from "../test/fixtures";
import {
  createCockpitStore,
  migrate,
  partialize,
  PERSIST_KEY,
  PERSIST_VERSION,
  resetCockpit,
  useCockpit,
} from "./store";
import { APP_DEFAULTS, CANVAS_MAX_TILES } from "./slices/prefsSlice";
import { MAX_TOASTS, TOAST_TTL_PLAIN, TOAST_TTL_UNDO } from "./slices/uiSlice";
import type { Session } from "./types";

/* Three harnesses, so `setHarnesses` has something with a length to assert. Composed at the
 * call site from `aHarness` rather than added to `fixtures.ts` — shared-shape §3.2 sanctions
 * six builders and no seventh. Nothing in this suite reads a field of them. */
const CATALOGUE = [aHarness({ id: "claude-code" }), aHarness({ id: "shell" }), aHarness({ id: "codex" })];

/* Every session this suite builds pins its own `worktreeId`: `aSession()`'s default comes from
 * task 14's `IDS`, not from "w-1", and the assertions below index the worktree map by hand. */
const sessionsOn = (worktreeId: string, ids: string[]): Session[] =>
  ids.map((id) => aSession({ id, worktreeId, attachPath: `/v1/sessions/${id}/attach` }));

beforeEach(() => {
  localStorage.clear();
  resetCockpit();
});
```

Append the data-slice block:

```ts
describe("dataSlice", () => {
  it("keeps catalogue setters independent of one another", () => {
    const s = useCockpit.getState();
    s.setProjects([aProject({ id: "p-1", name: "panacea", path: "C:\\work\\panacea" })]);
    s.setBranches("p-1", [aBranch({ id: "b-1", projectId: "p-1", name: "main", isHead: true })]);
    s.setHarnesses(CATALOGUE);
    expect(useCockpit.getState().projects).toHaveLength(1);
    expect(useCockpit.getState().branchesByProject["p-1"]).toHaveLength(1);
    expect(useCockpit.getState().harnesses).toHaveLength(3);
    expect(useCockpit.getState().machines).toEqual([]);
  });

  it("rebuilds the index in server order and empties a worktree that lost its sessions", () => {
    const s = useCockpit.getState();
    s.replaceSessions([...sessionsOn("w-1", ["s-1", "s-2"]), ...sessionsOn("w-2", ["s-3"])],
      ["w-1", "w-2"]);
    expect(useCockpit.getState().sessionIdsByWorktree["w-1"]).toEqual(["s-1", "s-2"]);
    s.replaceSessions(sessionsOn("w-2", ["s-3"]), ["w-1", "w-2"]);
    expect(useCockpit.getState().sessionIdsByWorktree["w-1"]).toEqual([]);
    expect(useCockpit.getState().sessions["s-1"]).toBeUndefined();
    expect(useCockpit.getState().sessions["s-3"]).toBeDefined();
  });

  it("leaves a worktree the fetch did not cover completely alone", () => {
    const s = useCockpit.getState();
    s.replaceSessions(sessionsOn("w-9", ["s-9"]), ["w-9"]);
    s.replaceSessions(sessionsOn("w-1", ["s-1"]), ["w-1"]);
    expect(useCockpit.getState().sessionIdsByWorktree["w-9"]).toEqual(["s-9"]);
    expect(useCockpit.getState().sessions["s-9"]).toBeDefined();
  });

  it("keeps the id array identical when a poll changed nothing", () => {
    const s = useCockpit.getState();
    s.replaceSessions(sessionsOn("w-1", ["s-1", "s-2"]), ["w-1"]);
    const before = useCockpit.getState().sessionIdsByWorktree["w-1"];
    s.replaceSessions(sessionsOn("w-1", ["s-1", "s-2"]), ["w-1"]);
    expect(useCockpit.getState().sessionIdsByWorktree["w-1"]).toBe(before);
  });

  it("keeps the session object identical when every field matches", () => {
    const s = useCockpit.getState();
    s.replaceSessions(sessionsOn("w-1", ["s-1"]), ["w-1"]);
    const before = useCockpit.getState().sessions["s-1"];
    s.replaceSessions(sessionsOn("w-1", ["s-1"]), ["w-1"]);
    expect(useCockpit.getState().sessions["s-1"]).toBe(before);
  });

  it("replaces the session object when a field actually changed", () => {
    const s = useCockpit.getState();
    s.replaceSessions(sessionsOn("w-1", ["s-1"]), ["w-1"]);
    const before = useCockpit.getState().sessions["s-1"];
    s.replaceSessions(
      [aSession({ id: "s-1", worktreeId: "w-1", attachPath: "/v1/sessions/s-1/attach", status: "idle" })],
      ["w-1"],
    );
    expect(useCockpit.getState().sessions["s-1"]).not.toBe(before);
    expect(useCockpit.getState().sessions["s-1"].status).toBe("idle");
  });

  it("upserts a known id without moving it — a poll moves zero pixels", () => {
    const s = useCockpit.getState();
    s.replaceSessions(sessionsOn("w-1", ["s-1", "s-2", "s-3"]), ["w-1"]);
    s.upsertSession(
      aSession({ id: "s-3", worktreeId: "w-1", status: "exited", exitReason: "killed" }),
    );
    expect(useCockpit.getState().sessionIdsByWorktree["w-1"]).toEqual(["s-1", "s-2", "s-3"]);
    expect(useCockpit.getState().sessions["s-3"].exitReason).toBe("killed");
  });

  it("prepends an unknown id, because server order is createdAt descending", () => {
    const s = useCockpit.getState();
    s.replaceSessions(sessionsOn("w-1", ["s-1"]), ["w-1"]);
    s.upsertSession(aSession({ id: "s-new", worktreeId: "w-1" }));
    expect(useCockpit.getState().sessionIdsByWorktree["w-1"]).toEqual(["s-new", "s-1"]);
  });

  it("notifies no subscriber when an upsert changed nothing", () => {
    const s = useCockpit.getState();
    s.replaceSessions(sessionsOn("w-1", ["s-1"]), ["w-1"]);
    const seen = vi.fn();
    const unsubscribe = useCockpit.subscribe(seen);
    s.upsertSession(aSession({ id: "s-1", worktreeId: "w-1", attachPath: "/v1/sessions/s-1/attach" }));
    unsubscribe();
    expect(seen).not.toHaveBeenCalled();
  });
});
```

Append the purge and error block, then the ui block:

```ts
describe("removeSession and lastError", () => {
  it("purges the record, the index entry, the tile, the pin and the focus", () => {
    const s = useCockpit.getState();
    s.replaceSessions(sessionsOn("w-1", ["s-1", "s-2"]), ["w-1"]);
    s.setTile("s-1", { x: 10, y: 20, w: 400, h: 300 });
    s.pinSession("s-1");
    s.focusSession("s-1");
    s.removeSession("s-1");
    const after = useCockpit.getState();
    expect(after.sessions["s-1"]).toBeUndefined();
    expect(after.sessionIdsByWorktree["w-1"]).toEqual(["s-2"]);
    expect(after.tiles["s-1"]).toBeUndefined();
    expect(after.pinned).toEqual([]);
    expect(after.selection.sessionId).toBeNull();
  });

  it("leaves a focus that pointed at a different session alone", () => {
    const s = useCockpit.getState();
    s.replaceSessions(sessionsOn("w-1", ["s-1", "s-2"]), ["w-1"]);
    s.focusSession("s-2");
    s.removeSession("s-1");
    expect(useCockpit.getState().selection.sessionId).toBe("s-2");
  });

  it("is clearable only by setError(null), never by a later successful fetch", () => {
    const s = useCockpit.getState();
    s.setError({ code: "unsupported_option", message: "effort is not accepted by shell" });
    s.setMachines([]);
    s.replaceSessions([], []);
    expect(useCockpit.getState().lastError?.code).toBe("unsupported_option");
    s.setError(null);
    expect(useCockpit.getState().lastError).toBeNull();
  });

  it("carries the sixteenth error code without a store change", () => {
    useCockpit.getState().setError({ code: "non_loopback_publish", message: "published off loopback" });
    expect(useCockpit.getState().lastError?.code).toBe("non_loopback_publish");
  });

  it("tracks a per-entity load key", () => {
    const s = useCockpit.getState();
    s.setLoading("worktrees:b-1", true);
    expect(useCockpit.getState().loading["worktrees:b-1"]).toBe(true);
    s.setLoading("worktrees:b-1", false);
    expect(useCockpit.getState().loading["worktrees:b-1"]).toBe(false);
  });
});

describe("uiSlice", () => {
  it("sends selectBranch to develop and clears the deeper selection", () => {
    const s = useCockpit.getState();
    s.setLens("merge");
    s.selectWorktree("w-1");
    s.focusSession("s-1");
    s.selectBranch("p-1", "b-2");
    const after = useCockpit.getState();
    expect(after.lens).toBe("develop");
    expect(after.selection).toEqual({
      projectId: "p-1",
      branchId: "b-2",
      worktreeId: null,
      sessionId: null,
    });
  });

  it("clears the branch, the worktree and the session when a project is selected", () => {
    const s = useCockpit.getState();
    s.selectBranch("p-1", "b-1");
    s.selectWorktree("w-1");
    s.focusSession("s-1");
    s.selectProject("p-2");
    expect(useCockpit.getState().selection).toEqual({
      projectId: "p-2",
      branchId: null,
      worktreeId: null,
      sessionId: null,
    });
  });

  it("keeps the focused session when a worktree is selected — focus is not navigation", () => {
    const s = useCockpit.getState();
    s.selectBranch("p-1", "b-1");
    s.focusSession("s-1");
    s.selectWorktree("w-2");
    const after = useCockpit.getState();
    expect(after.selection.worktreeId).toBe("w-2");
    expect(after.selection.sessionId).toBe("s-1");
  });

  it("focuses a session without changing the branch", () => {
    const s = useCockpit.getState();
    s.selectBranch("p-1", "b-1");
    s.focusSession("s-7");
    expect(useCockpit.getState().selection.branchId).toBe("b-1");
    expect(useCockpit.getState().selection.sessionId).toBe("s-7");
  });

  it("expands an absent key on the first toggle and collapses on the second", () => {
    const s = useCockpit.getState();
    expect(useCockpit.getState().expandedProjects["p-1"]).toBeUndefined();
    s.toggleProject("p-1");
    expect(useCockpit.getState().expandedProjects["p-1"]).toBe(true);
    s.toggleProject("p-1");
    expect(useCockpit.getState().expandedProjects["p-1"]).toBe(false);
  });

  it("closes one overlay without closing the others", () => {
    const s = useCockpit.getState();
    s.openPalette();
    s.openPicker();
    s.openContextMenu({ kind: "session", id: "s-1", x: 40, y: 80 });
    s.openSessionMenu("s-1");
    s.closePicker();
    const after = useCockpit.getState();
    expect(after.paletteOpen).toBe(true);
    expect(after.pickerOpen).toBe(false);
    expect(after.contextMenu?.id).toBe("s-1");
    expect(after.sessionMenuFor).toBe("s-1");
  });
});
```

Append the toast and prefs blocks:

```ts
describe("toasts", () => {
  it("gives an undoable toast 6000 ms and a plain one 3200 ms", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-09-18T09:00:00.000Z"));
    const s = useCockpit.getState();
    s.pushToast("Session killed", () => {});
    s.pushToast("Defaults saved");
    const [undoable, plain] = useCockpit.getState().toasts;
    expect(undoable.ttl).toBe(TOAST_TTL_UNDO);
    expect(undoable.createdAt).toBe(Date.parse("2026-09-18T09:00:00.000Z"));
    expect(plain.ttl).toBe(TOAST_TTL_PLAIN);
    expect(plain.undo).toBeUndefined();
    vi.useRealTimers();
  });

  it("dismisses by id and caps the column at MAX_TOASTS, dropping the oldest", () => {
    const s = useCockpit.getState();
    for (let i = 0; i < MAX_TOASTS + 2; i += 1) s.pushToast(`toast ${i}`);
    const toasts = useCockpit.getState().toasts;
    expect(toasts).toHaveLength(MAX_TOASTS);
    expect(toasts[0].message).toBe("toast 2");
    s.dismissToast(toasts[0].id);
    expect(useCockpit.getState().toasts).toHaveLength(MAX_TOASTS - 1);
  });
});

describe("prefsSlice", () => {
  it("starts at the documented APP_DEFAULTS", () => {
    expect(APP_DEFAULTS).toEqual({
      harness: "claude-code",
      model: "",
      effort: "",
      permissionMode: "ask",
    });
    expect(useCockpit.getState().defaults).toEqual(APP_DEFAULTS);
  });

  it("patches app defaults field by field and leaves the rest standing", () => {
    useCockpit.getState().setDefaults({ effort: "high" });
    expect(useCockpit.getState().defaults).toEqual({ ...APP_DEFAULTS, effort: "high" });
  });

  it("deletes an override key set to undefined, so the level above inherits again", () => {
    const s = useCockpit.getState();
    s.setProjectDefaults("p-1", { harness: "codex", model: "gpt-5-codex" });
    s.setProjectDefaults("p-1", { model: undefined });
    expect(useCockpit.getState().projectDefaults["p-1"]).toEqual({ harness: "codex" });
  });

  it("records and clears a tile", () => {
    const s = useCockpit.getState();
    s.setTile("s-1", { x: 1, y: 2, w: 3, h: 4 });
    expect(useCockpit.getState().tiles["s-1"]).toEqual({ x: 1, y: 2, w: 3, h: 4 });
    s.clearTile("s-1");
    expect(useCockpit.getState().tiles["s-1"]).toBeUndefined();
  });

  it("pins in the order the user pinned, and is idempotent", () => {
    const s = useCockpit.getState();
    s.pinSession("s-2");
    s.pinSession("s-1");
    s.pinSession("s-2");
    expect(useCockpit.getState().pinned).toEqual(["s-2", "s-1"]);
    expect(useCockpit.getState().toasts).toHaveLength(0);
  });

  it("no-ops past the cap and says so in a toast naming the number", () => {
    const s = useCockpit.getState();
    for (let i = 0; i < CANVAS_MAX_TILES; i += 1) s.pinSession(`s-${i}`);
    s.pinSession("s-over");
    const after = useCockpit.getState();
    expect(after.pinned).toHaveLength(CANVAS_MAX_TILES);
    expect(after.pinned).not.toContain("s-over");
    expect(after.toasts).toHaveLength(1);
    expect(after.toasts[0].message).toContain(String(CANVAS_MAX_TILES));
  });

  it("unpins without disturbing the order of the rest", () => {
    const s = useCockpit.getState();
    s.pinSession("s-1");
    s.pinSession("s-2");
    s.pinSession("s-3");
    s.unpinSession("s-2");
    expect(useCockpit.getState().pinned).toEqual(["s-1", "s-3"]);
  });
});
```

Append the persistence block, which is the half of this suite that protects a user's work:

```ts
describe("persistence", () => {
  const PERSISTED_KEYS = [
    "defaults", "projectDefaults", "branchDefaults", "tiles", "pinned",
    "layout", "railCollapsed",
    "expandedProjects", "expandedBranches", "expandedWorktrees",
    "selection", "lens",
  ];

  it("partializes exactly the twelve keys §3.5 names, and nothing else", () => {
    const s = useCockpit.getState();
    s.setHarnesses(CATALOGUE);
    s.replaceSessions(sessionsOn("w-1", ["s-1"]), ["w-1"]);
    s.pushToast("hello", () => {});
    s.openPalette();
    s.setError({ code: "bad_request", message: "no" });
    expect(Object.keys(partialize(useCockpit.getState())).sort()).toEqual([...PERSISTED_KEYS].sort());
  });

  it("writes the snapshot under the documented key and version", () => {
    useCockpit.getState().setLayout("canvas");
    const raw = localStorage.getItem(PERSIST_KEY);
    expect(raw).not.toBeNull();
    const parsed = JSON.parse(raw as string);
    expect(parsed.version).toBe(PERSIST_VERSION);
    expect(parsed.state.layout).toBe("canvas");
    expect("toasts" in parsed.state).toBe(false);
    expect("sessions" in parsed.state).toBe(false);
  });

  it("drops a version 0 snapshot rather than translating a shape nobody documented", () => {
    expect(migrate({ layout: "canvas", pinned: ["s-1"] }, 0)).toEqual({});
  });

  it("passes a version 1 snapshot through untouched", () => {
    const snapshot = { layout: "canvas" as const, pinned: ["s-1"] };
    expect(migrate(snapshot, 1)).toBe(snapshot);
  });

  it("is total over garbage, because it runs before any error boundary exists", () => {
    expect(migrate(null, 1)).toEqual({});
    expect(migrate("nonsense", 1)).toEqual({});
    expect(migrate(42, 1)).toEqual({});
    expect(migrate(undefined, 1)).toEqual({});
  });

  it("launches clean over a version 0 snapshot", () => {
    localStorage.setItem(
      PERSIST_KEY,
      JSON.stringify({ state: { layout: "canvas", pinned: ["s-gone"] }, version: 0 }),
    );
    const store = createCockpitStore();
    expect(store.getState().layout).toBe("columns");
    expect(store.getState().pinned).toEqual([]);
    expect(store.getState().defaults).toEqual(APP_DEFAULTS);
  });

  it("rehydrates a version 1 snapshot", () => {
    localStorage.setItem(
      PERSIST_KEY,
      JSON.stringify({
        state: {
          layout: "canvas",
          railCollapsed: true,
          tiles: { "s-1": { x: 8, y: 9, w: 480, h: 320 } },
          pinned: ["s-1"],
        },
        version: 1,
      }),
    );
    const store = createCockpitStore();
    expect(store.getState().layout).toBe("canvas");
    expect(store.getState().railCollapsed).toBe(true);
    expect(store.getState().tiles["s-1"]).toEqual({ x: 8, y: 9, w: 480, h: 320 });
  });

  it("keeps the code default for a key the snapshot predates — the flat-shape proof", () => {
    localStorage.setItem(
      PERSIST_KEY,
      JSON.stringify({ state: { layout: "canvas" }, version: 1 }),
    );
    const store = createCockpitStore();
    expect(store.getState().layout).toBe("canvas");
    expect(store.getState().tiles).toEqual({});
    expect(store.getState().pinned).toEqual([]);
    expect(store.getState().expandedProjects).toEqual({});
    expect(typeof store.getState().pinSession).toBe("function");
  });

  it("logs rather than going silent when the snapshot cannot be read", () => {
    localStorage.setItem(PERSIST_KEY, "{not json");
    const logged = vi.spyOn(console, "error").mockImplementation(() => {});
    const store = createCockpitStore();
    expect(store.getState().layout).toBe("columns");
    expect(logged).toHaveBeenCalled();
    logged.mockRestore();
  });
});

describe("resetCockpit", () => {
  it("restores every field and leaves the actions callable", () => {
    const s = useCockpit.getState();
    s.setHarnesses(CATALOGUE);
    s.replaceSessions(sessionsOn("w-1", ["s-1"]), ["w-1"]);
    s.setLayout("canvas");
    s.pinSession("s-1");
    s.pushToast("hello");
    resetCockpit();
    const after = useCockpit.getState();
    expect(after.harnesses).toEqual([]);
    expect(after.sessions).toEqual({});
    expect(after.sessionIdsByWorktree).toEqual({});
    expect(after.layout).toBe("columns");
    expect(after.pinned).toEqual([]);
    expect(after.toasts).toEqual([]);
    expect(localStorage.getItem(PERSIST_KEY)).toBeNull();
    after.setLayout("canvas");
    expect(useCockpit.getState().layout).toBe("canvas");
  });
});
```

Honest caveats before you run it. **First,** the four `createCockpitStore()` tests assume `persist` hydrates synchronously — it does when the storage's `getItem` returns a value rather than a Promise, which `createJSONStorage(() => localStorage)` does, so the snapshot is already applied when `create` returns. I have not been able to execute this: vitest 5 is not installed in this tree. If the first of those four reads pre-hydration state, the fix is `await store.persist.rehydrate()` before the assertion and an `async` test body — **not** a change to `store.ts`. The fourth is the one that proves the `onRehydrateStorage` log fires; it seeds unparseable JSON, so `createJSONStorage`'s `JSON.parse` throws inside `getItem`, `persist` catches it and calls the rehydration callback with the error. Its `toHaveBeenCalled()` is deliberately not a `toHaveBeenCalledTimes(1)`: I could not run it, and one is a count I would be inventing. **Second,** `localStorage` in a Tauri webview is per-origin: `http://localhost:1420` in dev and `http://tauri.localhost` in a packaged Windows build. The two do not share storage, so a developer's dev-mode layout does not follow them into the packaged app. That is correct behaviour and it will still surprise someone; it is written here so the first person to report it finds the answer.

```bash
cd desktop
npm test -- src/store/store.test.ts
```

**What you must see:** the run fails at import time, naming `src/store/store.test.ts` and the unresolved import `./store`, with a message of the form `Failed to load url ./store … Does the file exist?`, and **zero tests collected**. Reporter chrome is approximate, as above. Steps 5 to 8 write the four modules this import pulls in; the suite is not run again until step 8, because none of them compiles alone.

- [ ] **Step 5: Write `slices/dataSlice.ts`**

Decisions the contract does not make, each with its reason.

**(a) `lastError` is cleared only by `setError(null)`.** §3.2's comment says "Cleared on success", and the obvious reading — every successful setter clears it — is wrong in the one case that matters. The poller fires every 2 s (§4.7); a `400 unsupported_option` raised by a create would be wiped from the rail footer before the user finished reading it. So clearing is the caller's act, performed when the user has been told, and `setError` is the only thing that does it.

**(b) `replaceSessions` and `upsertSession` preserve object identity when nothing changed.** A poll returns freshly parsed JSON, so every session object is a new reference even when every byte matches, and a component reading `selectSession(id)` would re-render thirty times a minute over identical data. `sameSession` is a shallow field comparison and it is exact: every field of `Session` is a primitive — there is no nested object or array in it — so shallow equality *is* deep equality here. The same argument gives `sameIds` for the index arrays. This is the store's half of AGENTIC-UX §10 rule 7, "a poll moves zero pixels"; task 19's poller is the other half.

**(c) `removeSession` reaches into the prefs and ui fields.** Forgetting a session must also drop its tile, its pin and its focus, or a persisted `tiles` map grows without bound across a user's whole history and a dangling `selection.sessionId` points at nothing. The store is flat (§3.1), so one `set` covers all three; this is the first place that flatness pays for itself beyond persistence.

```ts
// desktop/src/store/slices/dataSlice.ts
//
// Contract §3.2. Server-derived entities and their setters.

import type { StateCreator } from "zustand";
import type {
  Branch,
  BranchId,
  Harness,
  Machine,
  Project,
  ProjectId,
  Session,
  SessionId,
  Worktree,
  WorktreeId,
} from "../types";
import type { CockpitState } from "../store";

export type LoadKey =
  | "machines"
  | "projects"
  | "harnesses"
  | "sessions"
  | `branches:${string}`
  | `worktrees:${string}`;

export interface DataSlice {
  machines: Machine[];
  projects: Project[];
  branchesByProject: Record<ProjectId, Branch[]>;
  worktreesByBranch: Record<BranchId, Worktree[]>;
  sessions: Record<SessionId, Session>;
  /** Server order (createdAt descending) preserved. Never re-sorted client-side. */
  sessionIdsByWorktree: Record<WorktreeId, SessionId[]>;
  harnesses: Harness[];
  loading: Partial<Record<LoadKey, boolean>>;
  /** The last API failure, for the one status line in the rail footer. Cleared by an
   *  explicit setError(null) and by nothing else — see decision (a). */
  lastError: { code: string; message: string } | null;

  setMachines(machines: Machine[]): void;
  setProjects(projects: Project[]): void;
  setBranches(projectId: ProjectId, branches: Branch[]): void;
  setWorktrees(branchId: BranchId, worktrees: Worktree[]): void;
  setHarnesses(harnesses: Harness[]): void;
  replaceSessions(sessions: Session[], worktreeIds: WorktreeId[]): void;
  upsertSession(session: Session): void;
  removeSession(id: SessionId): void;
  setLoading(key: LoadKey, value: boolean): void;
  setError(error: { code: string; message: string } | null): void;
}

type DataState = Pick<
  DataSlice,
  | "machines" | "projects" | "branchesByProject" | "worktreesByBranch"
  | "sessions" | "sessionIdsByWorktree" | "harnesses" | "loading" | "lastError"
>;

export const initialDataState: DataState = {
  machines: [],
  projects: [],
  branchesByProject: {},
  worktreesByBranch: {},
  sessions: {},
  sessionIdsByWorktree: {},
  harnesses: [],
  loading: {},
  lastError: null,
};
```

Append the helpers and the creator to the same file:

```ts
/** Shallow field comparison, which is exact for `Session`: every field is a primitive, so
 *  there is no nested value a shallow pass could miss. Used to keep a poll from replacing an
 *  object that did not change. */
export function sameSession(a: Session, b: Session): boolean {
  const keys = Object.keys(a) as (keyof Session)[];
  if (keys.length !== Object.keys(b).length) return false;
  for (const k of keys) {
    if (a[k] !== b[k]) return false;
  }
  return true;
}

export function sameIds(a: readonly SessionId[], b: readonly SessionId[]): boolean {
  if (a.length !== b.length) return false;
  for (let i = 0; i < a.length; i += 1) {
    if (a[i] !== b[i]) return false;
  }
  return true;
}

export const createDataSlice: StateCreator<
  CockpitState,
  [["zustand/persist", unknown]],
  [],
  DataSlice
> = (set, get) => ({
  ...initialDataState,

  setMachines: (machines) => set({ machines }),
  setProjects: (projects) => set({ projects }),
  setBranches: (projectId, branches) =>
    set((s) => ({ branchesByProject: { ...s.branchesByProject, [projectId]: branches } })),
  setWorktrees: (branchId, worktrees) =>
    set((s) => ({ worktreesByBranch: { ...s.worktreesByBranch, [branchId]: worktrees } })),
  setHarnesses: (harnesses) => set({ harnesses }),

  replaceSessions: (incoming, worktreeIds) =>
    set((s) => {
      // The fetch covered these worktrees, plus any the incoming rows mention. A covered
      // worktree with no incoming rows empties; an uncovered one is untouched.
      const covered = new Set<WorktreeId>(worktreeIds);
      for (const row of incoming) covered.add(row.worktreeId);

      const sessions: Record<SessionId, Session> = {};
      for (const [id, row] of Object.entries(s.sessions)) {
        if (!covered.has(row.worktreeId)) sessions[id] = row;
      }
      for (const row of incoming) {
        const prev = s.sessions[row.id];
        sessions[row.id] = prev !== undefined && sameSession(prev, row) ? prev : row;
      }

      const built: Record<WorktreeId, SessionId[]> = {};
      for (const id of covered) built[id] = [];
      for (const row of incoming) built[row.worktreeId].push(row.id);

      const sessionIdsByWorktree = { ...s.sessionIdsByWorktree };
      for (const [id, ids] of Object.entries(built)) {
        const prev = sessionIdsByWorktree[id];
        sessionIdsByWorktree[id] = prev !== undefined && sameIds(prev, ids) ? prev : ids;
      }
      return { sessions, sessionIdsByWorktree };
    }),

  upsertSession: (session) => {
    const prev = get().sessions[session.id];
    // Not `set({})`: returning an empty partial still builds a new state object and runs
    // every subscriber. The only way to write nothing is not to call set.
    if (prev !== undefined && sameSession(prev, session)) return;
    set((s) => {
      const sessions = { ...s.sessions, [session.id]: session };
      // A known id keeps its position — AGENTIC-UX §10 rule 7.
      if (prev !== undefined) return { sessions };
      const list = s.sessionIdsByWorktree[session.worktreeId] ?? [];
      return {
        sessions,
        sessionIdsByWorktree: {
          ...s.sessionIdsByWorktree,
          [session.worktreeId]: [session.id, ...list],
        },
      };
    });
  },

  removeSession: (id) => {
    const target = get().sessions[id];
    if (target === undefined) return;
    set((s) => {
      const sessions = { ...s.sessions };
      delete sessions[id];
      const tiles = { ...s.tiles };
      delete tiles[id];
      const list = s.sessionIdsByWorktree[target.worktreeId];
      return {
        sessions,
        tiles,
        sessionIdsByWorktree:
          list === undefined
            ? s.sessionIdsByWorktree
            : { ...s.sessionIdsByWorktree, [target.worktreeId]: list.filter((x) => x !== id) },
        pinned: s.pinned.includes(id) ? s.pinned.filter((x) => x !== id) : s.pinned,
        selection:
          s.selection.sessionId === id ? { ...s.selection, sessionId: null } : s.selection,
      };
    });
  },

  setLoading: (key, value) => set((s) => ({ loading: { ...s.loading, [key]: value } })),
  setError: (lastError) => set({ lastError }),
});
```

- [ ] **Step 6: Write `slices/uiSlice.ts`, with task 18's insertion points reserved**

Shared-shape §1 rule 1 makes the creating task reserve the insertion point in a comment naming the consumer. Task 18 adds `machinesOpen`, `openMachines()` and `closeMachines()` to this file — `⌘⇧M` opens the machines popover from a global key handler, and a popover held in `MachineChip`'s own `useState` is unreachable from there — so the comments below say exactly where each part goes. They are comments, not code: §3A's anchored grep is what stops a comment satisfying a guard that wanted the real line. The `type UiState = Pick<UiSlice, …>` below carries **no** such comment, and that is task 18's call rather than an oversight here: its step 11 covers that point with its own quoted-name guard and says in as many words that the `Pick` is the one this task does not mark, which is exactly why it gets missed.

Decisions the contract does not make. **(a) `selectProject` clears the branch, worktree and session.** §3.3 spells the clearing out for `selectBranch` and is silent for `selectProject`; a project selection that left a branch from a different project standing would put the rail and the Columns view in disagreement, which is the one thing the product spine forbids. **(b) `selectWorktree` does *not* clear the session focus.** Here the contract's silence reads the other way, because §3.3 says of `focusSession` that it is "focus, not navigation", and it documents clearing for `selectBranch` deliberately enough that its absence here is a choice rather than an omission. **(c) Toasts are capped at `MAX_TOASTS = 4`, oldest dropped.** A poll that fails every 2 s would otherwise stack an unbounded column over the cockpit; the dropped one is always the closest to expiry.

```ts
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
```

Append the state and the creator to the same file:

```ts
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
```

`Toast.undo` is a function, which is why `toasts` is not in the persisted partition and must never be added to it: a rehydrated toast would render an Undo button whose handler is `undefined` and whose click throws. §3.5 already excludes it; this is the reason.

- [ ] **Step 7: Write `slices/prefsSlice.ts`**

One decision the contract does not make: **a key present with an `undefined` value deletes the override**, and that is how a user clears one. §3.6 makes an absent key inherit, so "clear this override" and "delete this key" are the same operation; if `setProjectDefaults(id, { model: undefined })` merely stored `undefined`, `resolveDefaults` would read it as absent anyway but `partialize` would persist a key whose only content is a hole. Deleting is the honest write.

`clearTile` and `unpinSession` return **without calling `set`** when there is nothing to do, rather than returning an empty partial. That is the rule step 5's `upsertSession` comment states and it applies here for the same reason: `set({})` still builds a new state object and runs every subscriber, so an unpin of an id that is not pinned would re-render the Canvas.

`pinSession` calls `pushToast`, which belongs to `uiSlice`. That is legal and intended: the store is flat, every slice creator is typed against `CockpitState` rather than against its own slice, and `get()` therefore sees the whole object. It is also the reason the slices cannot be typed `StateCreator<PrefsSlice, …>` — that shape compiles until the first cross-slice call and then does not.

```ts
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
```

Append the creator to the same file:

```ts
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
```

- [ ] **Step 8: Write `store.ts` — the composition, the persist wiring and `migrate`**

`CockpitState` is declared here and the three slice files import it as a type. That is a module cycle on paper and not one in fact: `import type` is erased at build, so nothing at runtime points from a slice back to this file. It is also where the type belongs — `types.ts` is byte-identical to contract §2 and task 15 diffs it against the contract, so one more declaration added there would fail that diff.

`resetCockpit` merges the three initial-state objects rather than replacing the store, so the action closures survive. It also clears the persisted snapshot: without that, a test that seeds `localStorage` and builds a fresh store would read the previous test's write.

`onRehydrateStorage` closes contract §12's open question 3, which assigns it to this task by name. `localStorage` in a packaged Tauri build with `csp: null` is assumed from the webview being a normal secure-context browser — near-certain and untested. If that assumption is wrong, zustand swallows the failure and the app silently stops remembering the user's tiles, pins and defaults, with nothing anywhere saying so. There is no UI for it in Phase 2 and there should not be one; one line to the console is the whole fix, and it turns "mysterious" into "searchable".

```ts
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

export const PERSIST_KEY = "panacea-app.cockpit";
export const PERSIST_VERSION = 1;

/** What survives a restart. Everything else is re-fetched, because the control plane is the
 *  only truth about machines, projects, branches, worktrees and sessions. */
type Persisted = Pick<
  CockpitState,
  | "defaults" | "projectDefaults" | "branchDefaults" | "tiles" | "pinned"
  | "layout" | "railCollapsed"
  | "expandedProjects" | "expandedBranches" | "expandedWorktrees"
  | "selection" | "lens"
>;

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
});
```

Append the rest of the file:

```ts
/** Version 0 is any snapshot written before this contract existed — a pre-release dev build.
 *  It is dropped rather than translated: nothing shipped, so nothing is owed, and a wrong
 *  translation of a shape nobody documented is worse than a clean start. Future versions
 *  translate; they never drop, because by then a user's tile layout is their work.
 *
 *  This runs at module evaluation, before React mounts and before any error boundary exists,
 *  so it is TOTAL over `unknown`. A throw here is a white screen at launch, not a caught
 *  error, and the snapshot it chokes on is the one the user cannot delete without a devtools
 *  console they do not have in a packaged Tauri build. */
export function migrate(persisted: unknown, from: number): Partial<Persisted> {
  if (from < 1 || persisted === null || typeof persisted !== "object") return {};
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
```

Run the suite:

```bash
cd desktop
npm test -- src/store/store.test.ts
npm run typecheck
```

**What you must see:** all thirty-nine tests across the seven `describe` blocks pass, and `tsc` exits 0. The failures worth predicting each name a specific transcription slip. *keeps the id array identical when a poll changed nothing* failing on `expected [ 's-1', 's-2' ] to be [ 's-1', 's-2' ]` — note `toBe`, not `toEqual` — means `replaceSessions` assigned the freshly built array unconditionally and `sameIds` is not being consulted. *notifies no subscriber when an upsert changed nothing* failing with `expected "spy" to not be called at all, but actually been called 1 times` means `upsertSession` calls `set` before the `sameSession` guard, or returns `{}` from inside `set` instead of returning early. *keeps the code default for a key the snapshot predates* failing with `expected undefined to deeply equal {}` means the persisted shape was nested rather than flat. And *restores every field and leaves the actions callable* failing on `expected 'canvas' to be 'columns'` means `resetCockpit` spread only two of the three initial-state objects. Those strings are vitest's own assertion text, reasoned from its matchers rather than transcribed — vitest is not installed in this tree yet, so no line of this plan has been executed. Match on the test name and the shape of the claim, not byte-for-byte on the chrome.

- [ ] **Step 9: Write the failing tests for the selectors**

Some of these tests are not about values at all. `selectBranches`, `selectWorktrees`, `selectSessionIdsForWorktree`, `selectPaneSessionIds` and `selectCanvasSessionIds` all return a **stable empty-array constant** in the absent case, sharing three frozen constants between them, and `selectPaneSessionIds` builds a fresh array in the present case. The first is what keeps an unselected rail from re-rendering on every poll; the second is why §3.7 has its consumers wrap it in `useShallow`. The referential identity is pinned with `toBe` — once on the branches constant through `selectBranches`, once on the ids constant through `selectPaneSessionIds`.

Create `desktop/src/store/selectors.test.ts`:

```ts
import { beforeEach, describe, expect, it } from "vitest";
import { aBranch, aHarness, aMachine, aSession, aWorktree } from "../test/fixtures";
import {
  selectBranches,
  selectCanvasSessionIds,
  selectDefaultMachine,
  selectFocusedSessionId,
  selectHarness,
  selectPaneSessionIds,
  selectResolvedDefaults,
  selectSession,
  selectSessionIdsForWorktree,
  selectWorktrees,
} from "./selectors";
import { resetCockpit, useCockpit } from "./store";

/* Composed here from task 15's `aHarness`, not added to `fixtures.ts`: shared-shape §3.2
 * sanctions six builders and no seventh. Every field the assertions below read is written out
 * here — `shell`'s name, `codex`'s unavailability, and `claude-code`'s model list and
 * defaultModel, which the clamping test depends on. */
const CATALOGUE = [
  aHarness({
    id: "claude-code",
    name: "Claude Code",
    available: true,
    models: [
      { id: "opus-4.8", name: "Opus 4.8" },
      { id: "sonnet-4.8", name: "Sonnet 4.8" },
    ],
    defaultModel: "sonnet-4.8",
    efforts: ["", "low", "medium", "high"],
    permissionModes: ["ask", "bypass", "plan"],
  }),
  aHarness({
    id: "shell",
    name: "Shell",
    available: true,
    models: [],
    defaultModel: "",
    efforts: [],
    permissionModes: [],
  }),
  aHarness({
    id: "codex",
    name: "Codex",
    available: false,
    models: [{ id: "gpt-5-codex", name: "GPT-5 Codex" }],
    defaultModel: "gpt-5-codex",
  }),
];

/** One branch, two worktrees in server order, two sessions on the first and one on the
 *  second. Everything below reads this shape. Every id is pinned by an override, because
 *  `aSession()` and `aWorktree()` default theirs from task 14's `IDS`. */
function seed(): void {
  const s = useCockpit.getState();
  s.setHarnesses(CATALOGUE);
  s.setBranches("p-1", [aBranch({ id: "b-1", projectId: "p-1", name: "main", isHead: true })]);
  s.setWorktrees("b-1", [
    aWorktree({ id: "w-1", projectId: "p-1", branchId: "b-1", name: "main", isPrimary: true }),
    aWorktree({ id: "w-2", projectId: "p-1", branchId: "b-1", name: "evidence-upload" }),
  ]);
  s.replaceSessions(
    [
      aSession({ id: "s-1", worktreeId: "w-1" }),
      aSession({ id: "s-2", worktreeId: "w-1" }),
      aSession({ id: "s-3", worktreeId: "w-2" }),
    ],
    ["w-1", "w-2"],
  );
  s.selectBranch("p-1", "b-1");
}

beforeEach(() => {
  localStorage.clear();
  resetCockpit();
});
```

Append the assertions:

```ts
describe("selectors", () => {
  it("prefers the machine flagged default, then falls back to the first", () => {
    useCockpit.getState().setMachines([
      aMachine({ id: "cloud-1", name: "cloud-1", kind: "cloud", default: false }),
      aMachine({ id: "local", name: "local", kind: "local", default: true }),
    ]);
    expect(selectDefaultMachine(useCockpit.getState())?.id).toBe("local");
    useCockpit.getState().setMachines([
      aMachine({ id: "cloud-1", name: "cloud-1", kind: "cloud", default: false }),
      aMachine({ id: "cloud-2", name: "cloud-2", kind: "cloud", default: false }),
    ]);
    expect(selectDefaultMachine(useCockpit.getState())?.id).toBe("cloud-1");
    useCockpit.getState().setMachines([]);
    expect(selectDefaultMachine(useCockpit.getState())).toBeUndefined();
  });

  it("returns the very same empty array for an unknown or null parent", () => {
    const s = useCockpit.getState();
    const a = selectBranches("p-unknown")(s);
    const b = selectBranches(null)(s);
    const c = selectWorktrees("b-unknown")(s);
    expect(a).toEqual([]);
    expect(a).toBe(b);
    expect(selectSessionIdsForWorktree("w-unknown")(s)).toEqual([]);
    expect(c).toEqual([]);
  });

  it("reads branches, worktrees, session ids, a session and a harness", () => {
    seed();
    const s = useCockpit.getState();
    expect(selectBranches("p-1")(s).map((b) => b.id)).toEqual(["b-1"]);
    expect(selectWorktrees("b-1")(s).map((w) => w.id)).toEqual(["w-1", "w-2"]);
    expect(selectSessionIdsForWorktree("w-1")(s)).toEqual(["s-1", "s-2"]);
    expect(selectSession("s-3")(s)?.worktreeId).toBe("w-2");
    expect(selectSession("s-nope")(s)).toBeUndefined();
    expect(selectHarness("shell")(s)?.name).toBe("Shell");
    expect(selectHarness("nope")(s)).toBeUndefined();
  });

  it("orders the panes by worktree, then by session, both in server order", () => {
    seed();
    expect(selectPaneSessionIds(useCockpit.getState())).toEqual(["s-1", "s-2", "s-3"]);
  });

  it("keeps an exited session in the pane order, because the transcript is the artifact", () => {
    seed();
    useCockpit.getState().upsertSession(
      aSession({ id: "s-2", worktreeId: "w-1", status: "exited", exitReason: "killed" }),
    );
    expect(selectPaneSessionIds(useCockpit.getState())).toEqual(["s-1", "s-2", "s-3"]);
  });

  it("returns the stable empty constant when no branch is selected", () => {
    const first = selectPaneSessionIds(useCockpit.getState());
    useCockpit.getState().setLayout("canvas");
    expect(first).toBe(selectPaneSessionIds(useCockpit.getState()));
    expect(first).toEqual([]);
  });

  it("filters dead pins out of the canvas and keeps pin order", () => {
    seed();
    const s = useCockpit.getState();
    s.pinSession("s-3");
    s.pinSession("s-1");
    s.pinSession("s-gone");
    expect(selectCanvasSessionIds(useCockpit.getState())).toEqual(["s-3", "s-1"]);
  });

  it("threads the selection through resolveDefaults", () => {
    seed();
    const s = useCockpit.getState();
    s.setProjectDefaults("p-1", { effort: "high" });
    s.setBranchDefaults("b-1", { model: "opus-4.8" });
    expect(selectResolvedDefaults(useCockpit.getState())).toEqual({
      harness: "claude-code",
      model: "opus-4.8",
      effort: "high",
      permissionMode: "ask",
    });
  });

  it("clamps a stale branch default against the live catalogue", () => {
    seed();
    useCockpit.getState().setBranchDefaults("b-1", { harness: "codex", model: "gpt-5-codex" });
    const r = selectResolvedDefaults(useCockpit.getState());
    expect(r.harness).toBe("claude-code");
    expect(r.model).toBe("sonnet-4.8");
  });

  it("reports the focused session id", () => {
    expect(selectFocusedSessionId(useCockpit.getState())).toBeNull();
    useCockpit.getState().focusSession("s-2");
    expect(selectFocusedSessionId(useCockpit.getState())).toBe("s-2");
  });
});
```

```bash
cd desktop
npm test -- src/store/selectors.test.ts
```

**What you must see:** a failure at import time naming `./selectors`, with a message of the form `Failed to load url ./selectors … Does the file exist?`, and **zero tests collected**. Reporter chrome approximate, as above.

- [ ] **Step 10: Implement `selectors.ts`, with task 18's insertion point reserved**

Shared-shape §4 makes this the only file in Group D that declares a derived read, and §1 rule 1 makes this task reserve the point where task 18 adds its own four — `selectProjectById`, `selectBranchById`, `selectWorktreeById` and `selectBranchAttentionSessionId`. The curried selectors take `| null` as well as an id, because the thing a component almost always has is `selection.branchId`, and pushing the null check into the selector is one place instead of one per call site.

**The `useShallow` note, and the amendment.** §3.7 tells consumers to wrap `selectPaneSessionIds` and `selectCanvasSessionIds` in `useShallow` because they build a fresh array on every call, and without it zustand 5's default `Object.is` re-renders Columns thirty times a minute. The same is true of **`selectResolvedDefaults`**, which §3.7 does not say: `resolveDefaults` returns a freshly built `SessionDefaults` object every time it is called, so a bare `useCockpit(selectResolvedDefaults)` in `NewSessionButton` or `SessionPicker` re-renders on every poll for the same reason. It is written into the doc comment below so a consumer reads it at the point of use.

```ts
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
```

Append the built selectors and the reservation:

```ts
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
```

Run it:

```bash
cd desktop
npm test -- src/store/selectors.test.ts
npm run typecheck
```

**What you must see:** all ten tests pass and `tsc` exits 0. The predictable failures are these. *returns the very same empty array for an unknown or null parent* failing on `expected [] to be []` means a selector returns a fresh `[]` literal instead of the module constant — the assertion is `toBe`, and a fresh literal passes `toEqual` while causing the re-render the constant exists to prevent. *orders the panes by worktree, then by session* failing with `expected [ 's-3', 's-1', 's-2' ] to deeply equal [ 's-1', 's-2', 's-3' ]` means the loop iterates `sessionIdsByWorktree` rather than the worktree list, and so takes insertion order instead of server order.

- [ ] **Step 11: Run the whole suite and the type checker**

```bash
cd desktop
npm test
npm run typecheck
```

**What you must see:** every suite in the repository passes, including tasks 14, 15 and 16's, and `tsc` prints nothing beyond npm's banner. The three files this task adds contribute seventy-six tests — twenty-seven in `defaults.test.ts`, thirty-nine in `store.test.ts`, ten in `selectors.test.ts`. The totals for the earlier tasks depend on what has landed, so do not treat a particular grand total as the check; the check is that nothing that was green went red.

Guards before you commit. The first two prove this task left task 18's insertion points as comments and did not write task 18's code; the third proves it declared no fixture builder of its own:

```bash
cd desktop
grep -cE '^[[:space:]]*machinesOpen' src/store/slices/uiSlice.ts   # 0
grep -cE '^export const select(ProjectById|BranchById|WorktreeById|BranchAttentionSessionId)' src/store/selectors.ts   # 0
grep -cE '^export function (aMachine|aProject|aBranch|aWorktree|aSession|aHarness)\(' src/test/fixtures.ts
```

The first two are anchored at the first non-space character for the reason shared-shape §3A gives: the reserving comments spell out every one of those names, so an unanchored `grep -c machinesOpen` prints 2 and an unanchored count of the selector names prints 4, and neither tells you whether a line of code landed. The alternation in the second is the whole of task 18's addition to this file and there is no fifth; `selectBranches` and `selectWorktrees` are this task's own and do not match it, because the branch is `BranchById`, not `Branch`. The third must print the same `6` step 1 saw — this task adds no builder and removes none, and a `7` means someone declared the seventh §3.2 forbids.

- [ ] **Step 12: Commit**

Stage by name — shared-shape §6 forbids `git add -A`, `-u` and `.`, and other agents are working in this tree. Run this from the repository root, like every other block in this plan, and stage `package.json` / `package-lock.json` **only if they appear in its output**; step 1's `npm install --save-exact zustand@5.0.15` is normally a no-op because task 14 already pinned it:

```bash
git status --short desktop/package.json desktop/package-lock.json
```

**If that printed nothing, skip the next block.** If it printed a line for either file, run it — `git add` on a tracked file that did not change stages nothing, so naming both is safe once either has moved, and it is still staging by name:

```bash
git add desktop/package.json desktop/package-lock.json
```

Then the files this task writes:

```bash
git add desktop/src/store/slices/dataSlice.ts \
        desktop/src/store/slices/uiSlice.ts \
        desktop/src/store/slices/prefsSlice.ts \
        desktop/src/store/store.ts \
        desktop/src/store/defaults.ts \
        desktop/src/store/selectors.ts \
        desktop/src/store/defaults.test.ts \
        desktop/src/store/store.test.ts \
        desktop/src/store/selectors.test.ts
```

The commit message runs to several lines, so write it to a file first rather than passing a heredoc through the shell:

```bash
printf '%s\n' \
'feat(cockpit): the store — three flat slices, persistence and inheritance' \
'' \
'Contract §3. One flat zustand 5 store: dataSlice (server entities),' \
'uiSlice (selection, lens, layout, overlays, toasts) and prefsSlice' \
'(defaults, overrides, tiles, pins), all contributing keys at the root.' \
'Flat is a correctness requirement: persist merges the rehydrated' \
'snapshot shallowly, so a nested shape would silently drop every key' \
'added after the snapshot was written.' \
'' \
'persist is pinned to panacea-app.cockpit at version 1, partialized to' \
'the twelve keys §3.5 names — no entity map, no overlay, no toast. The' \
'migrate function drops a version 0 snapshot and is total over unknown,' \
'because it runs before React mounts and a throw there is a white screen' \
'at launch rather than a caught error. A read that fails is logged once' \
'through onRehydrateStorage instead of being swallowed (contract §12,' \
'open question 3).' \
'' \
'resolveDefaults is pure: project -> branch -> session merged field by' \
'field, then clamped against the live catalogue so a six-week-old branch' \
'default naming an uninstalled harness produces a create that succeeds' \
'instead of a 400. toCreateRequest strips every field the chosen harness' \
'does not accept.' \
'' \
'replaceSessions and upsertSession preserve object and array identity' \
'when a poll changed nothing, which is the store half of "a poll moves' \
'zero pixels" (docs/AGENTIC-UX.md §10 rule 7).' \
'' \
'Reserves the insertion points task 18 fills: machinesOpen in uiSlice,' \
'and selectProjectById, selectBranchById, selectWorktreeById and' \
'selectBranchAttentionSessionId in selectors.' \
'' \
'Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>' \
> /tmp/task17-commit.txt
git commit -F /tmp/task17-commit.txt
```

Confirm the trailer survived, exactly:

```bash
git log -1 --pretty=%B | tail -1
```

```
Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
```

