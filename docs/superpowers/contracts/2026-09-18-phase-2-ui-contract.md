# Phase 2 — the TypeScript/UI contract for Group D (the cockpit)

> **Status: binding.** Every name, type, field, prop, CSS variable, file path and npm
> version below is fixed. Task authors for tasks 14–24 restate the parts their task
> touches inside that task's `Interfaces:` block — a fresh subagent sees only one task,
> so nothing here may be referenced by pointer ("see the contract"); it must be copied
> into the task.
>
> Where this document contradicts the **File structure** table at line 66 of
> `docs/superpowers/plans/2026-09-18-phase-2-backend-and-cockpit.md`, this document wins.
> Where it contradicts `scratchpad/phase2-parts/contract-go.md` on anything server-side,
> **the Go contract wins** and §10 records what I would have changed and why.

**What was verified, and how.** Everything asserted about a third-party API below was read
from the npm registry or from `desktop/node_modules` on this machine. Nothing here is recalled.

| Claim | Checked against |
|---|---|
| `vitest@5.0.1`, peers `vite ^6.4.0 \|\| ^7.0.0 \|\| ^8.0.0` | `npm view vitest version` / `npm view vitest peerDependencies`, 2026-09-18 |
| `jsdom@30.1.0` | `npm view jsdom version` |
| `@testing-library/react@16.3.3`, peers React `^18 \|\| ^19` **and `@testing-library/dom ^10.0.0`** (must be installed explicitly) | `npm view @testing-library/react peerDependencies` |
| `@testing-library/dom@10.4.2`, `@testing-library/user-event@14.6.7` | `npm view … version` |
| `zustand@5.0.15`, export map has a `"./*"` wildcard so `zustand/middleware` and `zustand/react/shallow` resolve | `npm view zustand version` / `npm view zustand exports --json` |
| `@xterm/xterm@6.0.0` typings expose `write(data: string \| Uint8Array, cb?)` (1253), `scrollToBottom()` (1227), `IBuffer.viewportY` (1525), `IBuffer.baseY` (1531), `attachCustomKeyEventHandler` (1072) — assigned to `makeTerm` by task 23's fix round 1 (§6.6, "The chord filter") | `desktop/node_modules/@xterm/xterm/typings/xterm.d.ts` |
| Node on this machine is **v24.15.0**; `vitest@5` `engines.node` is `^22.12.0 \|\| ^24.0.0 \|\| >=26.0.0` | `node --version` |
| `desktop/tsconfig.json` is `strict`, `noUnusedLocals`, `noUnusedParameters`, `lib: ["ES2020","DOM"]`, `include: ["src"]`; `tsconfig.node.json` includes only `vite.config.ts`; there is **no `@types/node`** (`vite.config.ts` carries a `@ts-expect-error` for `node:process`) | the files |
| `main.tsx` renders inside `<React.StrictMode>` — effects double-invoke in dev | `desktop/src/main.tsx` |
| OKLCH values in §5 | computed with the Björn Ottosson sRGB↔Oklab matrices; every value round-trips to its source hex exactly, working shown in §5.2 |

**Not verified, and marked as such:** how `@xterm/xterm@6` behaves under jsdom. §1.5 does not
guess — it removes the question by never running real xterm in a unit test.

---

## 0. Scope boundary, and the three documents that fight

Three specs describe this UI and they do not agree. The resolution is fixed here so no task
author re-litigates it.

1. **`docs/FUNCTIONAL-SPEC.md` is the Phase 2 scope.** It says so in its first line:
   "Supersedes assumptions baked into `prototype/app.html`". Rail → sub-rail → top bar →
   Columns/Canvas → session lifecycle → palette → keyboard map. That is Group D.
2. **`prototype/app.html` is the approved *look and layout*, not the approved *model*.**
   Where the spec contradicts it, the spec wins, and there are four such places, listed in §0.1.
3. **`docs/AGENTIC-UX.md` describes the Desk, the ranked queue, the handover and seven
   statuses. Almost none of that is buildable in Phase 2**, because five of the seven statuses
   and every element of the queue need the agent-runtime signal that document itself calls
   "a hard runtime dependency, not a UI task". Phase 2 therefore ships the four mechanically
   derived statuses (`starting`, `running`, `idle`, `exited`) and the AGENTIC-UX rules that
   *are* enforceable without that signal. §7 lists exactly which, and §0.2 lists what is
   deliberately not built and where it goes.

### 0.1 Where `docs/FUNCTIONAL-SPEC.md` supersedes `prototype/app.html`

| Prototype does | Spec requires | Consequence for Group D |
|---|---|---|
| One agent per worktree (`w.sessions` exists but the rail, merge and lock logic assume one) | **Many sessions per worktree** | `sessionIdsByWorktree: Record<WorktreeId, SessionId[]>` in the store; the rail nests sessions under a worktree; the worktree row is never a session row |
| `＋ session` always opens the dialog (`A.newSessionDialog`) | **One click for the common case**, caret opens the picker | `NewSessionButton` is a split button: the main half calls `createSession` with resolved defaults and no dialog; the caret half opens `SessionPicker` |
| Synthetic transcript objects (`T.user`, `T.perm`, `T.todos` …) rendered by `renderTty` | A session is a **real PTY**; agent CLIs are never parsed | There is no transcript model in Phase 2. `renderTty`, `T.*`, `answerPerm`, `cycleMode` and `sendTo` have **no successor**. The pane body is `<Terminal/>` |
| Session status vocabulary `run · need · idle · drift · done · shell` | `starting · running · idle · exited` (Phase 2 subset) | `SessionStatus` has exactly four members. `need` and `drift` cannot be derived without reading PTY bytes and are not invented |

### 0.2 What Group D deliberately does not build

The Desk, the ranked queue, the read cursor and watermark, the handover, the authority grid,
convergence, capacity, Discover, Design, Files, Merge and Deploy surfaces. The sub-rail renders
all six lens rows — AGENTIC-UX rule 12 requires that entry into any lens is legal from any state,
so **no lens row is ever disabled and none carries a completion marker** — but five of them route
to `LensPlaceholder`, which names the phase that brings the surface (§6.4). Discover arrives in
Phase 4 under `desktop/src/discover/` and must not be pre-empted here; §11 reconciles the two
plans file by file.

---

## 1. Test infrastructure

`desktop/package.json` today has **no test tooling at all**: no vitest, no jsdom, no testing
library, no `test` script. Task 14 is the first task in Group D because tasks 15–24 are written
TDD and cannot start without a runner.

### 1.1 The exact devDependency block

Five packages, no more. `@testing-library/dom` is listed explicitly because RTL 16 made it a
peer dependency rather than a transitive one, and npm will not install it for you. No
`@vitest/coverage-v8` (nothing in Phase 2 gates on a coverage number, and a number nobody gates
on is a number that rots). No `@types/node` — `vitest.config.ts` below touches no Node API, which
is the only reason the repo can stay without it.

```json
  "devDependencies": {
    "@tauri-apps/cli": "^2",
    "@testing-library/dom": "10.4.2",
    "@testing-library/react": "16.3.3",
    "@testing-library/user-event": "14.6.7",
    "@types/react": "^19.1.8",
    "@types/react-dom": "^19.1.6",
    "@vitejs/plugin-react": "^6.0.2",
    "jsdom": "30.1.0",
    "typescript": "~6.0.3",
    "vite": "^8.0.16",
    "vitest": "5.0.1"
  }
```

Test-tool versions are **pinned exactly** (no `^`) and product dependencies keep their existing
ranges. A caret on a test runner means a green suite today and a red suite on a colleague's
`npm install` tomorrow, for a change nobody made.

One runtime dependency is added, to `"dependencies"`:

```json
    "zustand": "5.0.15",
```

Install command, in **PowerShell** from `desktop/`:

```powershell
npm install --save-dev vitest@5.0.1 jsdom@30.1.0 `
  @testing-library/react@16.3.3 @testing-library/dom@10.4.2 @testing-library/user-event@14.6.7
npm install zustand@5.0.15
```

Expected: `added N packages` twice, and **no `npm warn ERESOLVE`**. A peer warning here means the
React 19 / RTL 16 pairing resolved wrongly and the next 10 tasks will produce confusing render
errors; stop and read it rather than continuing.

### 1.2 The npm scripts

```json
  "scripts": {
    "dev": "vite",
    "build": "tsc && vite build",
    "preview": "vite preview",
    "tauri": "tauri",
    "test": "vitest run",
    "test:watch": "vitest",
    "typecheck": "tsc --noEmit"
  }
```

`npm test` is the command every task in Group D runs. It is non-watching, so an agent does not
hang on it.

### 1.3 `desktop/vitest.config.ts`

```ts
import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";

// A separate config from vite.config.ts on purpose: vite.config.ts pins port 1420 and
// strictPort for Tauri, and a test run must never fail because the dev server is up.
export default defineConfig({
  plugins: [react()],
  test: {
    environment: "jsdom",
    include: ["src/**/*.test.{ts,tsx}"],
    setupFiles: ["./src/test/setup.ts"],
    // No globals: every test imports describe/it/expect explicitly, so tsconfig needs no
    // extra "types" entry and `npm run typecheck` stays clean.
    globals: false,
    restoreMocks: true,
    clearMocks: true,
  },
});
```

`vitest.config.ts` must be added to `tsconfig.node.json`'s `include`, or `npm run typecheck`
ignores it:

```json
{
  "compilerOptions": {
    "composite": true,
    "skipLibCheck": true,
    "module": "ESNext",
    "moduleResolution": "bundler",
    "allowSyntheticDefaultImports": true
  },
  "include": ["vite.config.ts", "vitest.config.ts"]
}
```

### 1.4 `desktop/src/test/setup.ts`

jsdom implements neither `matchMedia` nor `ResizeObserver` nor `scrollIntoView`. All three are
used by real cockpit code (`prefers-reduced-motion`, the fit addon's container observation, and
defect 4's scroll-into-view), so all three are stubbed once, here, rather than in 20 test files.

```ts
import { afterEach, vi } from "vitest";
import { cleanup } from "@testing-library/react";

afterEach(() => {
  cleanup();
});

// jsdom has no matchMedia. Reduced motion is reported as OFF so that motion code paths are
// the ones under test; a test that wants the reduced path overrides window.matchMedia itself.
if (!window.matchMedia) {
  window.matchMedia = ((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addEventListener: () => {},
    removeEventListener: () => {},
    addListener: () => {},
    removeListener: () => {},
    dispatchEvent: () => false,
  })) as unknown as typeof window.matchMedia;
}

// jsdom has no ResizeObserver. The fit addon and the Canvas tiles both construct one.
if (!("ResizeObserver" in window)) {
  (window as unknown as { ResizeObserver: unknown }).ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  };
}

// jsdom implements Element but not scrollIntoView. Defect 4's fix calls it, and a test must be
// able to assert THAT it was called, so it is a spy rather than a no-op.
if (!Element.prototype.scrollIntoView) {
  Element.prototype.scrollIntoView = vi.fn();
}
```

### 1.5 xterm.js under jsdom — the decision, not a guess

`@xterm/xterm@6` measures character cells with a canvas 2D context and renders with WebGL or a
DOM renderer. jsdom implements neither `CanvasRenderingContext2D` (without the optional `canvas`
native package, which needs a C++ toolchain on Windows) nor WebGL. **Phase 2 does not unit-test
the xterm rendering layer, and does not install `canvas`.**

The layer is instead made small enough that not testing it is cheap. `Terminal.tsx` is split in
three, and the two testable thirds carry all the logic:

| File | Tested how |
|---|---|
| `desktop/src/lib/wire.ts` (exists) | Pure functions over `Uint8Array`. Plain unit tests, no DOM. |
| `desktop/src/api/attachRegistry.ts` | The socket lifecycle, the 10 Hz output batching, the pinned-scroll rule and the reconnect retry ladder (§4.6: `starting`, `running` and `idle`). Unit tested against a **fake terminal** and a **fake WebSocket**, both injected — no jsdom globals are patched. |
| `desktop/src/components/Terminal.tsx` | ~40 lines that construct a real `XTerm`, install the keyboard map's chord filter on it (§6.6), call `open()` and hand it to the registry. Tested with `vi.mock("@xterm/xterm")` (below), which asserts the wiring and never renders a glyph. |

The mock, copied verbatim into any test that renders `<Terminal/>`:

```ts
// Records what the component does to the terminal without loading the real one.
export class FakeTerm {
  cols = 80;
  rows = 24;
  written: string[] = [];
  disposed = false;
  scrolledToBottom = 0;
  buffer = { active: { viewportY: 0, baseY: 0 } };
  private dataHandlers: ((d: string) => void)[] = [];
  loadAddon() {}
  attachCustomKeyEventHandler() {}   // makeTerm calls it (§6.6); without it construction throws
  open() {}
  focus() {}
  write(data: string | Uint8Array) {
    this.written.push(typeof data === "string" ? data : new TextDecoder().decode(data));
  }
  scrollToBottom() { this.scrolledToBottom += 1; }
  onData(cb: (d: string) => void) { this.dataHandlers.push(cb); return { dispose: () => {} }; }
  onResize() { return { dispose: () => {} }; }
  dispose() { this.disposed = true; }
  /** test helper: simulate the user typing */
  type(s: string) { this.dataHandlers.forEach((h) => h(s)); }
}

vi.mock("@xterm/xterm", () => ({ Terminal: FakeTerm }));
vi.mock("@xterm/addon-fit", () => ({ FitAddon: class { fit() {} activate() {} dispose() {} } }));
vi.mock("@xterm/xterm/css/xterm.css", () => ({}));
```

Real xterm rendering — ANSI fidelity, resize, the permission prompt drawing correctly — was
already proven in Phase 1 against a real Claude Code session
(`docs/verification/2026-09-17-phase-1-results.md`, 9/9) and is re-proven by hand in Group E.
It is not a thing a jsdom assertion can tell the truth about.

**One declared departure, task 23's fix round 1: `src/hooks/useKeyboardMap.xterm.test.tsx` runs
the real xterm.** Not its rendering — its *key path*, which is plain DOM event handling and which
jsdom does run. The defect it pins was invisible to every other test because they all dispatched
on `window` and none on the focused element: xterm handles keydown on its own helper textarea
first and cancels what it maps to data, so a chord pressed in a focused terminal went to the agent
as control bytes (§6.6, "The chord filter"). The file mounts the real `<Terminal/>` and asserts the
end state — the map acts and the agent receives nothing, for every chord pressed with Ctrl and
again with Meta (the Meta rows added by fix round 2); Escape, shift+Tab and Ctrl+C still reach
the agent. It was allowed on one condition, that it is stable (five consecutive identical runs
before commit); **if it ever flakes it is deleted**, and `Terminal.test.tsx`'s handler-level tests
stay. Nothing in it draws or measures a glyph, so the rule above is unchanged for rendering.

### 1.6 How a test renders a store-backed component

The store is a zustand store, so it is a module singleton. Three rules make that safe under
`vitest --run` with `restoreMocks`/`clearMocks` (which do **not** reset module state):

1. `desktop/src/store/store.ts` exports `createCockpitStore(initial?: Partial<CockpitState>)`
   **and** the singleton `useCockpit = createCockpitStore()`. Tests that need isolation build
   their own.
2. The singleton exports `resetCockpit()`, which calls `useCockpit.setState(initialState(), true)`
   (the `true` replaces rather than merges) and clears `localStorage`. Every store test calls it
   in `beforeEach`.
3. Components never import the store's *actions* through props. They call
   `useCockpit(selector)` directly, so a test controls them by calling `useCockpit.setState(...)`
   before render — no provider, no wrapper, no context.

The canonical component test, which task 18 copies:

```tsx
import { beforeEach, describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Rail } from "./Rail";
import { resetCockpit, useCockpit } from "../../store/store";

describe("Rail", () => {
  beforeEach(() => {
    resetCockpit();
    useCockpit.setState({
      projects: [{ id: "p-1", name: "engagement-portal", path: "C:\\work\\engagement-portal" }],
      expandedProjects: { "p-1": true },
      branchesByProject: {
        "p-1": [{ id: "b-1", projectId: "p-1", name: "main", isHead: true }],
      },
    });
  });

  it("lists projects and their branches", async () => {
    render(<Rail />);
    expect(screen.getByRole("treeitem", { name: /engagement-portal/ })).toBeDefined();
    expect(screen.getByRole("treeitem", { name: /main/ })).toBeDefined();
  });

  it("selects a branch on click", async () => {
    render(<Rail />);
    await userEvent.click(screen.getByRole("treeitem", { name: /main/ }));
    expect(useCockpit.getState().selection.branchId).toBe("b-1");
  });
});
```

`@testing-library/jest-dom` is **not** installed, so there is no `toBeInTheDocument`. Use
`expect(...).toBeDefined()` on a `getBy*` (which already throws when absent) and
`expect(screen.queryBy*(...)).toBeNull()` for absence. That is one fewer dependency and one
fewer `tsconfig.types` entry, and it is the same assertion strength.

### 1.7 The one thing that will bite: `React.StrictMode`

`desktop/src/main.tsx` renders inside `<React.StrictMode>`, so in dev **every effect mounts,
unmounts and mounts again**. A naive `useEffect(() => { const ws = new WebSocket(url); … })`
opens two sockets per session, and the control plane's `attachCount` will read 2 with one pane on
screen. `@testing-library/react` does **not** enable StrictMode by default, so a unit test will
not catch this.

The fix is structural and is why `attachRegistry` exists: the socket and the xterm instance live
in a module-level, **ref-counted** registry keyed by `sessionId`, not in a React effect. Mount
increments, unmount decrements, and the socket closes only when the count reaches zero **and**
a 5 s grace timer expires. Under StrictMode the count goes 1 → 0 → 1 inside one tick, and the
grace timer means the socket is never actually torn down. The same mechanism is what lets a pane
move from Columns to Canvas without dropping the PTY.

Task 19 has an explicit step that wraps the render in `<StrictMode>` and asserts exactly one
`FakeWebSocket` was constructed.
---

## 2. The domain model — `desktop/src/store/types.ts`

One file, one compiling block, no imports. Every server-derived field name is **exactly** the
JSON name from `contract-go.md` §3, because a rename here is a bug that only shows up at runtime
against a real control plane. Client-only types are grouped at the bottom and marked.

Id types are plain aliases, not branded strings. Branding would force a cast at every JSON
boundary — `resp.id as SessionId` on every response field — in exchange for catching a defect
class (passing a `WorktreeId` where a `SessionId` belongs) that the store's shape already makes
hard, because the two never appear in the same signature.

```ts
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
  | "provisioning"
  | "running"
  | "idle"
  | "sleeping"
  | "stopped"
  | "error";

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
export type KnownHarnessId = "claude-code" | "codex" | "opencode" | "gemini-cli" | "shell";
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
  /** Why it is unavailable, or what it is. Rendered verbatim in the picker. */
  note: string;
}

/* ── sessions ──────────────────────────────────────────────────────────────────────── */

/** Exactly four. docs/AGENTIC-UX.md §4 rule 8 specifies seven; the other five need the
 *  agent-runtime signal that document calls a hard runtime dependency, and deriving them
 *  would require reading PTY bytes, which docs/PROGRAM.md forbids absolutely. */
export type SessionStatus = "starting" | "running" | "idle" | "exited";

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
export type Lens = "discover" | "design" | "develop" | "files" | "merge" | "deploy";

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
```

### 2.1 Cardinality, and the one that matters

`docs/FUNCTIONAL-SPEC.md`: **many sessions per worktree**. The store therefore holds
`sessionIdsByWorktree: Record<WorktreeId, SessionId[]>` and never a `sessions` array on a
`Worktree`. Three consequences a task author must not undo:

- The rail nests **project → branch → worktree → session**, four levels, and a worktree row is
  never a session row even when it holds exactly one session.
- `SessionPane`'s header shows the **worktree name** and the **harness command**, because two
  panes on screen can legitimately carry the same worktree name (`prototype/app.html` line 1105
  already does this and is right).
- Killing the last session on a worktree **does not remove the worktree**. The prototype does
  (`A.sessAct` case `kill` splices the worktree when `w.sessions` empties) and that is wrong
  under the new cardinality: a worktree is a checkout on disk, it outlives its sessions, and the
  control plane enumerates it from git either way.

---

## 3. The store — zustand 5

### 3.1 Files

| Path | Owns |
|---|---|
| `desktop/src/store/types.ts` | §2, nothing else. No imports, no runtime code. |
| `desktop/src/store/store.ts` | `createCockpitStore`, the `useCockpit` singleton, `resetCockpit`, the `persist` wiring, `PERSIST_KEY`, `PERSIST_VERSION`, `migrate`. |
| `desktop/src/store/slices/dataSlice.ts` | Server-derived entities and their setters. |
| `desktop/src/store/slices/uiSlice.ts` | Selection, lens, layout, rail expansion, overlays, toasts. |
| `desktop/src/store/slices/prefsSlice.ts` | Defaults, inheritance overrides, tiles, pins. |
| `desktop/src/store/selectors.ts` | Every derived read. No component computes a derivation inline. |
| `desktop/src/store/defaults.ts` | `resolveDefaults`, `toCreateRequest` — pure, no store import. |

The store is **flat**: all three slices contribute keys at the root of one object. This is not
cosmetic — zustand's `persist` middleware merges the rehydrated object into the store with a
**shallow** merge at the root, so a nested `{ prefs: {...} }` shape silently drops any key added
after the persisted snapshot was written. Flat + shallow is correct; nested + shallow is a
lurking data-loss bug.

### 3.2 `dataSlice`

```ts
export type LoadKey = "machines" | "projects" | "harnesses" | "sessions" | `branches:${string}` | `worktrees:${string}`;

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
  /** The last API failure, for the one status line in the rail footer. Cleared only by
   *  setError(null) (task 17, decision (a)), which the operation that RAISED it calls when it
   *  next succeeds, and only while this is still its report; another operation's success
   *  leaves it alone. The operations are: the poller; each catalogue list; the session verbs,
   *  which are one operation because they share one reporter; and the create path. While a
   *  catalogue list the user can see is still failing, the loader also puts that list's report
   *  back whenever this is null. Amended 2026-09-23 by task 24 (ruling B, §4.7), and
   *  2026-09-24 by its fix round 1. */
  lastError: { code: string; message: string } | null;

  setMachines(machines: Machine[]): void;
  setProjects(projects: Project[]): void;
  setBranches(projectId: ProjectId, branches: Branch[]): void;
  setWorktrees(branchId: BranchId, worktrees: Worktree[]): void;
  setHarnesses(harnesses: Harness[]): void;

  /** Replaces the full session set for the given worktree ids and rebuilds their indexes.
   *  `worktreeIds` is the set the fetch covered, so a worktree that now has zero sessions
   *  empties rather than keeping stale rows.
   *
   *  ONE EXCEPTION, added 2026-09-23 by task 22: ids held in `lib/pendingSessions` are skipped,
   *  and the store's own view of them wins — present with its optimistic status, or absent.
   *  A deferred verb (Kill, Forget) patches the store now and HOLDS its request for the 6 s
   *  Undo window, so the server still reports the old truth; without this the 2 s poll takes
   *  the row back about two seconds in, with the Undo toast still on screen. The suppression
   *  lives here rather than in the poller because this is the one place holding both the
   *  incoming rows and the pending ids, so every future deferred verb gets it for free. It
   *  lasts exactly as long as the window: once the id clears, the server is authoritative
   *  again. */
  replaceSessions(sessions: Session[], worktreeIds: WorktreeId[]): void;
  /** Merges one session in place. If the id is known, the position in
   *  sessionIdsByWorktree is UNCHANGED — docs/AGENTIC-UX.md §10 rule 7: a poll moves
   *  zero pixels. If it is new, it is prepended (server order is createdAt descending). */
  upsertSession(session: Session): void;
  /** Purge only. A killed session keeps its record and stays listed — the transcript is the
   *  artifact (docs/FUNCTIONAL-SPEC.md "Session lifecycle"). */
  removeSession(id: SessionId): void;

  setLoading(key: LoadKey, value: boolean): void;
  setError(error: { code: string; message: string } | null): void;
}
```

### 3.3 `uiSlice`

```ts
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

  selectProject(id: ProjectId | null): void;
  /** Selecting a branch sets lens to "develop" and clears worktree+session selection. */
  selectBranch(projectId: ProjectId, id: BranchId): void;
  selectWorktree(id: WorktreeId | null): void;
  /** Focus, not navigation: it does not change the branch. Callers that need both call
   *  selectBranch first. */
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

  pushToast(message: string, undo?: () => void): void;
  dismissToast(id: string): void;
}
```

**There is no `drafts` map, and that is deliberate.** The prototype keeps `s.draft` because its
composer is a React-rendered `<input>` that a re-render destroys — known defect 1. Phase 2 has no
composer: the user types into the agent's own TUI, inside the PTY, and those keystrokes never
touch React state. Defect 1 is therefore fixed *structurally* (the xterm instance is never
unmounted by a re-render, §1.7 and §7.2) rather than by copying text back and forth. The only
draft that exists in Phase 2 is the picker's opening prompt, which lives in `SessionPicker`'s own
`useState` and dies with the popover.

### 3.4 `prefsSlice`

```ts
export interface PrefsSlice {
  /** App-level defaults. The bottom of the inheritance chain. */
  defaults: SessionDefaults;
  projectDefaults: Record<ProjectId, SessionDefaultsOverride>;
  branchDefaults: Record<BranchId, SessionDefaultsOverride>;
  /** Hand-placed Canvas tiles, keyed by session. Known defect 6. */
  tiles: Record<SessionId, TilePosition>;
  /** The Canvas working set. Capped at CANVAS_MAX_TILES = 10 (docs/AGENTIC-UX.md §10 rule 5).
   *  Order is the order the user pinned in, and is never re-sorted. */
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

export const CANVAS_MAX_TILES = 10;

export const APP_DEFAULTS: SessionDefaults = {
  harness: "claude-code",
  model: "",          // "" resolves to the harness's own defaultModel, see §3.6
  effort: "",
  permissionMode: "ask",
};
```

### 3.5 Persistence, the key, the version and the migration

```ts
import { create } from "zustand";
import { createJSONStorage, persist } from "zustand/middleware";

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

/** Version 0 is any snapshot written before this contract existed — a pre-release dev build.
 *  It is dropped rather than translated: nothing shipped, so nothing is owed, and a wrong
 *  translation of a shape nobody documented is worse than a clean start. Future versions
 *  translate; they never drop, because by then a user's tile layout is their work. */
export function migrate(persisted: unknown, from: number): Partial<Persisted> {
  if (from < 1 || persisted === null || typeof persisted !== "object") return {};
  return persisted as Partial<Persisted>;
}
```

Wired as:

```ts
export const useCockpit = create<CockpitState>()(
  persist(
    (...a) => ({ ...createDataSlice(...a), ...createUiSlice(...a), ...createPrefsSlice(...a) }),
    {
      name: PERSIST_KEY,
      version: PERSIST_VERSION,
      storage: createJSONStorage(() => localStorage),
      partialize,
      migrate,
    },
  ),
);
```

**Not persisted, on purpose:** every entity map (the server is truth, and a stale worktree list
rendered before the first fetch is a lie the user has to un-learn), `loading`, `lastError`,
`toasts`, `paletteOpen`, `pickerOpen`, `contextMenu`, `sessionMenuFor`. An overlay that survives
a restart is a bug report.

`localStorage` inside a Tauri webview is per-origin and persists across app restarts —
`http://tauri.localhost` in a packaged Windows build, `http://localhost:1420` in dev. **The two
do not share storage**, so a developer's dev-mode layout does not follow them into the packaged
app. That is correct but surprising; say it in the task.

### 3.6 Inheritance — `resolveDefaults`, as a pure function

`docs/FUNCTIONAL-SPEC.md`: "**Defaults inherit** project → branch → session, so a team
standardises once and nobody re-chooses."

```ts
// desktop/src/store/defaults.ts

/** Later wins, per field. An ABSENT key inherits; it never resets. This is why the override
 *  type is Partial<SessionDefaults> and why "" is not used as "unset" — for `effort`, ""
 *  is a meaningful value meaning "send no flag".
 *
 *  After merging, the result is CLAMPED against the live catalogue, because a branch default
 *  recorded six weeks ago can name a harness that is no longer installed or a model that
 *  harness no longer offers, and the control plane would answer 400 unknown_harness or
 *  400 unsupported_option. Clamping client-side turns a failed create into a correct one.
 *
 *  Clamp order, and it matters:
 *    1. harness       — if absent from the catalogue or not `available`, fall back to
 *                       "claude-code" when it is available, else the first available harness,
 *                       else "shell". `shell` is always available: the sandbox image has bash.
 *    2. model         — if the chosen harness has no models, "". If the model is not among
 *                       them, the harness's defaultModel.
 *    3. effort        — if the harness's `efforts` is empty, "". If the value is not in it, "".
 *    4. permissionMode— if the harness's `permissionModes` is empty, "ask" (the server ignores
 *                       it for such a harness; see toCreateRequest, which strips it). If the
 *                       value is not in the list, the first entry.
 */
export function resolveDefaults(
  app: SessionDefaults,
  project: SessionDefaultsOverride | undefined,
  branch: SessionDefaultsOverride | undefined,
  harnesses: readonly Harness[],
): SessionDefaults;

/** Builds the POST /v1/sessions body, stripping every field the chosen harness does not
 *  accept. Sending `effort` to a shell is 400 unsupported_option (contract-go.md §3.2), and a
 *  400 the client could have avoided is a defect in the client. */
export function toCreateRequest(
  worktreeId: WorktreeId,
  d: SessionDefaults,
  harness: Harness,
  opts?: { openingPrompt?: string; title?: string; resume?: string; contextRef?: string },
): CreateSessionRequest;
```

`toCreateRequest` rules, exhaustively:

| Field | Included when |
|---|---|
| `worktreeId` | always |
| `machineId` | never — the Go contract defaults it to `"local"` and Phase 2 has one machine. Sending it would make a second machine a client change as well as a server one. |
| `harness` | always |
| `model` | `harness.models.length > 0` and the value is non-empty |
| `effort` | `harness.efforts.length > 0` and the value is non-empty |
| `permissionMode` | `harness.permissionModes.length > 0` |
| `openingPrompt` | `harness.supportsOpeningPrompt` and the trimmed value is non-empty |
| `title` | only when the user typed one; otherwise omitted so the server derives `"<harness name> · <worktree name>"` |
| `resume` | only for the Resume verb, and only when `harness.supportsResume` |
| `contextRef` | only when the caller passes one (Phase 4) |

### 3.7 Selectors

Every one is a named export from `selectors.ts`. Components never inline a derivation, because
an inline `.filter()` inside `useCockpit(...)` returns a new array on every store write and
re-renders the component on every 2 s poll — which is exactly the "layout shift from polling"
AGENTIC-UX §10 rule 7 bans.

| Selector | Signature | Notes |
|---|---|---|
| `selectDefaultMachine` | `(s) => Machine \| undefined` | The one with `default: true`, else `machines[0]`. |
| `selectBranches` | `(projectId) => (s) => Branch[]` | Curried; stable empty array constant when absent. |
| `selectWorktrees` | `(branchId) => (s) => Worktree[]` | Same. |
| `selectSessionIdsForWorktree` | `(worktreeId) => (s) => SessionId[]` | Ids, not objects — an id array is referentially stable across a poll that changed nothing. |
| `selectSession` | `(id) => (s) => Session \| undefined` | |
| `selectPaneSessionIds` | `(s) => SessionId[]` | **The Columns order.** Worktrees of the selected branch in server order; within each, its sessions in server order. This is the one true ordering, and `⌘1-9`, the palette and Columns all index it. |
| `selectCanvasSessionIds` | `(s) => SessionId[]` | `pinned` filtered to sessions that still exist, in pin order. |
| `selectHarness` | `(id) => (s) => Harness \| undefined` | |
| `selectResolvedDefaults` | `(s) => SessionDefaults` | `resolveDefaults(defaults, projectDefaults[sel.projectId], branchDefaults[sel.branchId], harnesses)`. |
| `selectFocusedSessionId` | `(s) => SessionId \| null` | |

Selectors returning a freshly built array (`selectPaneSessionIds`, `selectCanvasSessionIds`) are
consumed with `useShallow` from `zustand/react/shallow`:

```ts
import { useShallow } from "zustand/react/shallow";
const ids = useCockpit(useShallow(selectPaneSessionIds));
```

Without `useShallow`, zustand 5's default `Object.is` comparison sees a new array every poll and
re-renders Columns 30 times a minute. That is the single most likely performance defect in
Group D and it is invisible until there are ten panes open.

### 3.8 Client-side ids — `desktop/src/lib/ids.ts`

Known defect 2 is `uid()` colliding on fast successive calls. Every id the user can see comes
from the **server** (`internal/ids`, `crypto/rand`), so the client needs ids for exactly two
things: toasts and React keys for transient rows.

```ts
let counter = 0;
/** Monotonic within a window, unique across windows. The counter alone collides across two
 *  windows sharing localStorage; randomUUID alone is fine but 36 chars in every React key. */
export function makeId(prefix: string): string {
  counter += 1;
  return `${prefix}-${counter.toString(36)}-${crypto.randomUUID().slice(0, 8)}`;
}
/** Test seam: resets the counter so a test can assert monotonicity from a known point. */
export function __resetIdCounter(): void { counter = 0; }
```

`crypto.randomUUID` is available in the Tauri webview (WebView2 / WKWebView, both secure
contexts) and in jsdom 30 under Node 24. It is **not** available on an insecure `http://` origin
in some browsers — which does not apply here, because the only origins are `tauri.localhost` and
`localhost`, both of which browsers treat as secure contexts.

### 3.9 Immutable helpers — `desktop/src/lib/immutable.ts`

Two known defects (7 and 10) are the same mistake in different clothes: an index computed against
one array and applied to another. Both are fixed by never doing that, and the helpers make
"never doing that" the path of least resistance.

```ts
/** Replaces exactly one element. Known defect 7: the prototype's editAnswer splices with
 *  `splice(i, 1)` and then pushes, which drops every later element. */
export function replaceAt<T>(items: readonly T[], index: number, next: T): T[];

/** Replaces the element whose key matches, by identity rather than position. Preferred over
 *  replaceAt wherever the element has an id. */
export function replaceById<T extends { id: string }>(items: readonly T[], next: T): T[];

/** Moves `id` by `delta` places WITHIN the subsequence `visibleIds`, and returns the full
 *  list reordered to match. Known defect 10: the prototype's `reorder` finds the index in the
 *  FILTERED ready list and applies it to the UNFILTERED worktrees array, so reordering two
 *  ready worktrees swaps two unrelated ones whenever an unready worktree sits between them.
 *  The signature makes the filtered/unfiltered distinction explicit and impossible to
 *  conflate. */
export function moveWithinVisible<T extends { id: string }>(
  all: readonly T[],
  visibleIds: readonly string[],
  id: string,
  delta: number,
): T[];

/** Maps a 1-based position in a FILTERED list back to an entity id, or null. Every keyboard
 *  and palette path uses this instead of indexing an array. */
export function idAtVisiblePosition(visibleIds: readonly string[], oneBased: number): string | null;
```

The rule these encode, which belongs in every task that touches a list: **never pass an array
index across a filter boundary; pass the id.**
---

## 4. The typed API client — `desktop/src/api/client.ts`

One exported function per Go endpoint. Paths, field names and status codes are copied from
`contract-go.md` §3 and are not re-derived. Where I believe the Go surface needs a change, §10
records it; nothing here silently diverges.

### 4.1 Base URL, and how the client is constructed

```ts
export const DEFAULT_BASE_URL = "http://127.0.0.1:8730";

export interface ClientOptions {
  /** No trailing slash. Overridden in dev by import.meta.env.VITE_PANACEA_APP_API. */
  baseUrl?: string;
  /** Injected so a unit test never touches the network and never patches a global. jsdom
   *  does not implement fetch; vitest's jsdom environment leaves Node's global fetch in
   *  place, which works but is an implementation detail this client refuses to depend on. */
  fetch?: typeof globalThis.fetch;
  /** Injected for the same reason. jsdom's WebSocket opens a real socket. */
  webSocket?: (url: string) => WebSocket;
  /** Per-request timeout. 0 disables. Defaults to 10_000; /attach is not a fetch and is
   *  unaffected. */
  timeoutMs?: number;
}

export interface CockpitClient { /* every method of §4.4 */ }

export function createClient(opts?: ClientOptions): CockpitClient;

/** The app-wide instance, constructed once in desktop/src/api/index.ts. Components never
 *  construct their own. */
export const api: CockpitClient;
```

`baseUrl` resolution order: `opts.baseUrl` → `import.meta.env.VITE_PANACEA_APP_API` →
`DEFAULT_BASE_URL`. `import.meta.env` is Vite's, is statically replaced at build time, and is
`undefined` under vitest unless a test sets it — which is why `opts.baseUrl` is first.

### 4.2 The error shape

```ts
/** The complete set from contract-go.md §3.2 — including non_loopback_publish, which Group C's
 *  ruling 7 added — and the two the client raises itself.
 *
 *  non_loopback_publish is NOT a "not running" state: the sandbox IS running and published its
 *  port off loopback, so any exhaustive switch over this union must give it its own sentence. */
export type ProblemCode =
  | "invalid_json" | "bad_request" | "unknown_harness" | "unsupported_option"
  | "worktree_not_found" | "project_not_found" | "session_not_found"
  | "forbidden_origin" | "forbidden_host" | "conflict" | "sandbox_not_running"
  | "harness_unavailable" | "upgrade_required" | "backend_unavailable" | "internal"
  | "non_loopback_publish"
  /* client-raised */
  | "network_unavailable"   // fetch rejected: the control plane is not running
  | "timeout";              // the request exceeded timeoutMs

export interface Problem {
  error: ProblemCode;
  message: string;
  details?: Record<string, string>;
}

export class ApiError extends Error {
  readonly code: ProblemCode;
  /** 0 for network_unavailable and timeout. */
  readonly status: number;
  readonly details: Record<string, string>;
  constructor(code: ProblemCode, message: string, status: number, details?: Record<string, string>);
}
```

Every method rejects with `ApiError` and nothing else. One error branch in every caller:

```ts
try { await api.createSession(req); }
catch (e) {
  const err = e as ApiError;
  if (err.code === "backend_unavailable") { /* Docker Desktop is closed — say so */ }
  else if (err.code === "network_unavailable") { /* the control plane is not running */ }
  else { pushToast(err.message); }
}
```

`network_unavailable` and `backend_unavailable` are different sentences to the user and must not
be collapsed: the first means *our* process is not up, the second means Docker is not. Telling a
user to start Docker when the control plane is dead costs them ten minutes.

**Sixteen codes, not fifteen.** `non_loopback_publish` was added by Group C's ruling 7 and is
emitted by the attach handler when the daemon reports a sandbox published off loopback. It carries
the same trap as the pair above and a worse one: **the sandbox is running**, so the plausible
guess — "the sandbox is not running" — is the one sentence it must never produce. Anything
rendering a failure reason from this union owes it its own branch.

Non-2xx handling: read the body, `JSON.parse` it, and if it has a string `error` and a string
`message`, throw that `ApiError`. If the body is not a Problem (a proxy interposed, or the server
panicked before writing JSON), throw `new ApiError("internal", <first 200 chars of body> || res.statusText, res.status)`.
Never throw the raw `Response`.

### 4.3 Request and response envelopes

The Go surface wraps every list in a named object. The client unwraps, so no caller writes
`.machines` twice.

```ts
interface MachinesResponse { machines: Machine[] }
interface ProjectsResponse { projects: Project[] }
interface BranchesResponse { branches: Branch[] }
interface WorktreesResponse { worktrees: Worktree[] }
interface HarnessesResponse { harnesses: Harness[] }
interface SessionsResponse { sessions: Session[] }

export interface Health {
  status: "ok";
  version: string;
  backend: string;
  /** false means the control plane is up but Docker is not. HTTP is still 200 —
   *  contract-go.md §3.3 — so the client must read this field, not the status code. */
  backendReady: boolean;
}

export interface CreateSessionRequest {
  worktreeId: WorktreeId;
  machineId?: MachineId;
  harness?: HarnessId;
  model?: ModelId;
  effort?: Effort;
  permissionMode?: PermissionMode;
  openingPrompt?: string;
  title?: string;
  resume?: string;
  contextRef?: string;
}

export interface ListSessionsFilter {
  worktreeId?: WorktreeId;
  /** Repeated as ?status=running&status=idle, per contract-go.md §3.11. */
  status?: SessionStatus[];
}
```

### 4.4 One function per endpoint

| Method | Go endpoint | Success | Returns |
|---|---|---|---|
| `health(): Promise<Health>` | `GET /v1/healthz` | 200 | `Health` |
| `listMachines(): Promise<Machine[]>` | `GET /v1/machines` | 200 | unwrapped |
| `listProjects(): Promise<Project[]>` | `GET /v1/projects` | 200 | unwrapped; `[]` is a state, not an error |
| `listBranches(projectId): Promise<Branch[]>` | `GET /v1/projects/{projectId}/branches` | 200 | 404 `project_not_found` |
| `listWorktrees(projectId, branchId?): Promise<Worktree[]>` | `GET /v1/projects/{projectId}/worktrees[?branchId=]` | 200 | |
| `listHarnesses(): Promise<Harness[]>` | `GET /v1/harnesses` | 200 | |
| `createSession(req): Promise<Session>` | `POST /v1/sessions` | **201** | body is the session; `Location` header ignored (the body already carries the id) |
| `listSessions(filter?): Promise<Session[]>` | `GET /v1/sessions[?worktreeId=&status=]` | 200 | server-sorted `createdAt` descending; the client never re-sorts |
| `getSession(id): Promise<Session>` | `GET /v1/sessions/{id}` | 200 | |
| `renameSession(id, title): Promise<Session>` | `PATCH /v1/sessions/{id}` | 200 | body `{ "title": string }`; empty/whitespace title is 400 `bad_request`, so the client trims and refuses to send an empty one |
| `restartSession(id): Promise<Session>` | `POST /v1/sessions/{id}/restart` | 200 | no body sent; 409 `conflict` when a restart is already in flight |
| `killSession(id): Promise<Session>` | `DELETE /v1/sessions/{id}` | 200 | idempotent; killing an exited session is 200 |
| `purgeSession(id): Promise<void>` | `DELETE /v1/sessions/{id}?purge=true` | **204** | no body — the method must not attempt `res.json()` |
| `attachUrl(session): string` | — | — | §4.6 |
| `openAttach(session): WebSocket` | `GET /v1/sessions/{id}/attach` | 101 | §4.6 |

Two shapes that catch people: `createSession` is **201, not 200**, and `purgeSession` is
**204 with no body**. A generic `if (res.status !== 200) throw` breaks both.

Resume and Fork are **not endpoints**. Per `contract-go.md` §3.14, resume is
`createSession({ ..., resume })` — a new session continuing an old transcript — and fork is
`createSession` against a different worktree. The client exposes them as named wrappers so the
UI reads correctly and the distinction stays visible:

```ts
/** Resume: a NEW session that continues an old one's context (claude --resume). The old
 *  session keeps its record and its transcript. Only offered when harness.supportsResume. */
resumeSession(source: Session, harness: Harness): Promise<Session>;
/** Fork: the same configuration in a DIFFERENT worktree. Phase 2 cannot create a worktree
 *  (no endpoint — see §10.3), so Fork is offered only when the branch already has another
 *  worktree, and the menu names the target. */
forkSession(source: Session, targetWorktreeId: WorktreeId, harness: Harness): Promise<Session>;
```

### 4.5 There is no bearer token on this hop

`contract-go.md` §3.16 lists the attach handler's five pre-upgrade checks: origin allowlist,
loopback `Host`, session exists, sandbox running, WebSocket headers. **No token.** The
per-sandbox `attachToken` is minted by the control plane and injected into the sandbox
environment; it authenticates the *control plane → sandbox* hop and is explicitly never
serialised to the desktop (§3.10, "Never serialised: attachToken, endpoint, …").

So the client sends no `Authorization` header anywhere, and a task author who adds one is
guessing. The security boundary on the desktop hop is: loopback bind + `Host` check +
`Origin` allowlist. §10.4 records what that does and does not buy, because it is worth a
reviewer's attention rather than a silent assumption.

### 4.6 Building the attach WebSocket URL

```ts
/** ws(s):// + the session's own attachPath. The path is taken from the session object and
 *  NOT reassembled from the id, so a server-side route change is one server change. */
export function attachUrl(baseUrl: string, session: Session): string {
  const base = new URL(baseUrl);
  base.protocol = base.protocol === "https:" ? "wss:" : "ws:";
  // attachPath is absolute and already begins with "/v1/". Resolving it against the base
  // discards the base's own path, which is what we want and what string concatenation
  // would get wrong for a baseUrl carrying a path.
  return new URL(session.attachPath, base).toString();
}
```

For the default base that yields `ws://127.0.0.1:8730/v1/sessions/s-3f2a91c0/attach`.

`openAttach(session)` calls `opts.webSocket(attachUrl(...))` — default
`(url) => new WebSocket(url)` — and sets `binaryType = "arraybuffer"` before returning. The
browser sends `Origin` on the handshake automatically; nothing is added by hand.

**The retry rule, and it is required, not optional.** `POST /v1/sessions` returns `201` as soon
as the record exists and the sandbox has been *created* — status `starting`, not ready
(`contract-go.md` §3.9). Attaching immediately gets `409 sandbox_not_running` or a socket that
closes at once. The registry therefore retries:

- Delays: 250 ms, 500 ms, 1 s, 2 s, 2 s, 2 s … capped at 2 s.
- Give up after **60 s** total and render the pane's "did not start" state with a Restart
  action. 60 s is the outer bound on a cold Docker pull on this machine; a shorter budget turns
  a slow first run into a false failure.
- **AMENDED by task 19 — the give-up reason is read, not guessed.** This bullet used to say
  "surface `sandbox_not_running`", and that is a sentence the cockpit is not entitled to. A
  refused WebSocket upgrade exposes **neither the HTTP status nor the body** to JavaScript: the
  handshake arrives as `onerror` then `onclose` with code 1006, and task 12's perfectly good
  `409 sandbox_not_running` / `500 non_loopback_publish` JSON body is thrown away by the
  browser. `sandbox_not_running` is merely the likeliest-looking code, and guessing it produces
  exactly the sentence Group C's ruling 7 forbids, because `non_loopback_publish` means the
  sandbox **is running** and published its port off loopback. So on give-up the registry makes
  one diagnostic `GET /v1/sessions/{id}` (§4.4, no new client surface) and phrases the reason
  from the record: `exited` renders `exitReason` verbatim; anything else says the sandbox *is*
  in that status and refused the connection. `reasonForCode` keeps a `non_loopback_publish`
  branch with the correct wording, ready for the phase that lets the client read a refusal
  directly.
- **AMENDED by task 19 — `idle` is retried too.** This bullet used to read "retry only while
  the session's status is `starting` or `running`", which strands `idle`: an idle session is a
  live sandbox that has simply been quiet, its status is derived from a `lastOutput` timestamp,
  and refusing to reconnect leaves a healthy agent unreachable for ever — nothing can move it
  back to `running`, because the only thing that would is the output the cockpit is no longer
  attached to receive. The rule's intent is its own complement, stated in the same paragraph.
  So: **the ladder runs for `starting`, `running` and `idle`, and stops on `exited`.** A session
  that reads `exited` is not retried — the sandbox is gone and the correct UI is the exit reason.
- A socket that opened and then closed while the session is still live **is** retried, on a
  ladder that restarts from the bottom rung with a fresh 60 s budget: a successful handshake
  retires both the attempt count and the budget clock, or a session attached for an hour would
  give up instantly the first time its socket dropped. That is a control-plane restart, and the
  user should see the pane reconnect, not a dead terminal.

### 4.7 Polling

There is no event stream in the Go contract (§10.5 requests one). Phase 2 polls, from exactly one
place: `desktop/src/api/poller.ts`.

```ts
/** One interval for the whole app. Starts on mount of AppShell, stops on unmount, pauses
 *  while document.visibilityState === "hidden" (a laptop lid closed for eight hours must not
 *  wake to 14,000 queued requests). `onGoodTick` runs after every tick whose fetch succeeded,
 *  and never after a failed, skipped or stopped one. */
export function startPolling(
  client: CockpitClient,
  store: typeof useCockpit,
  onGoodTick?: () => void,
): () => void;

export const POLL_INTERVAL_MS = 2000;
```

Each tick issues **one** request: `GET /v1/sessions?worktreeId=<each worktree of the selected
branch>` is wrong (N requests); it issues `GET /v1/sessions` unfiltered and calls
`replaceSessions(all, worktreeIdsSeen)`. Machines, projects, branches, worktrees and harnesses
are fetched on navigation and on an explicit refresh, not on the poll — they change when the user
acts, not when an agent does.

**Two amendments, added 2026-09-23 by task 24 (ledger, "Task 24 rulings"), both declared
departures into task 19's `poller.ts`.** (A) A catalogue list that **failed, or never arrived**,
is retried on the poll after all: App passes the catalogue loader's `retry` as `onGoodTick`, so
every good tick retries such a list, with at most one request in flight per list, and a list that
has arrived is never fetched again. A good tick is the one proof the control plane answers, so a
daemon that is down costs no extra request, and there is still one interval. (B) A good tick
clears `lastError` **only if the poller raised it** and it is still on screen. It used to clear
any error, so a catalogue failure was reported and then wiped 2 s later, leaving an empty rail and
a clean footer. Each operation clears its own report when it next succeeds, and only while that
report is still on screen. §3.2's `lastError` lists the operations.

**Task 24's fix round 1 (2026-09-24) finished (B).** Two call sites still cleared any report on
success: the session verbs (task 22's `sessionVerbs.ts`) and the create path (task 18's
`useCreateSession.ts`). Both now keep the report they last wrote and clear only that. Both are
declared departures. The verbs have five success paths, because every path that calls the one
reporter is the same operation: the immediate commit (Rename, Restart), the deferred commit (Kill,
Forget), Mark ready, Resume and Fork. The last four now clear their own report too; before, none
of them cleared anything. Round 1 wired the deferred commit and Mark ready, and **fix round 2**
(re-review I1) wired Resume and Fork, whose "Claude Code is not installed." otherwise stayed on
the footer after a Resume that worked. The loader also **puts a report back**. The footer has one slot,
so a catalogue failure that the poller's own failure replaced disappeared when the poller
recovered, even though the list was still missing. Now, whenever the slot is empty and a list the
user can see is still failing, the loader puts that list's latest report back. A list in a
collapsed project or branch is not put back. It is not retried while closed, so its report would
sit on the footer without being refreshed.

The poll must move zero pixels. Three things make that true, and all three are testable:
`replaceSessions` preserves index position for known ids (§3.2), the server sorts and the client
never re-sorts, and Columns keys panes by `session.id`.

**And one thing makes the poll not move a row it must not move at all.** A deferred verb holds its
request for the 6 s Undo window while the poll runs every 2 s, so for those ids the server is
briefly *wrong* rather than merely stale. `replaceSessions` skips ids held in
`lib/pendingSessions` (§3.2) — **the poller itself has no knowledge of this and needs none**,
which is the point: a check here would leak store state into a fetch loop, and every deferred verb
added later would have to remember it. *Added 2026-09-23 by task 22.*
---

## 5. The token layer — `desktop/src/styles/tokens.css`

This is a hard gate. `docs/DESIGN-SYSTEM.md` exists because the first prototype failed by looking
machine-generated, and the token layer does more anti-templating work than any component library.
Every component CSS file starts with `@import "../styles/tokens.css";` and contains **no literal
hex**, which is the one rule a reviewer can check mechanically (§8, task 14's test).

### 5.1 Why OKLCH, and what it costs

`docs/DESIGN-SYSTEM.md`: equal numeric steps in OKLCH read as equal *perceptual* steps, and a
dark tool interface lives in the bottom quarter of the lightness range, where hand-picked hex
ramps go muddy or banded. The ramp below is therefore **derived**, not sampled: lightness steps
of 2.0–2.5 % with chroma rising in step, at one warm hue.

Cost: `oklch()` needs Chromium 111+ / Safari 15.4+. The Tauri 2 webview is WebView2 (evergreen
Chromium) on Windows and WKWebView on macOS, both well past that, so no fallback ladder is
shipped. Support is asserted once, in a comment, rather than doubled into a `@supports` block
that nobody will ever delete.

### 5.2 The arithmetic, so a reviewer can check it

The accent is specified in `docs/DESIGN-SYSTEM.md` as **deep teal `#2fb3bd`**. Conversion, using
Björn Ottosson's sRGB→Oklab matrices:

1. **sRGB → linear**, per channel, `c ≤ 0.04045 ? c/12.92 : ((c+0.055)/1.055)^2.4`:
   `0x2f/255 = 0.184314 → 0.028426`; `0xb3/255 = 0.701961 → 0.450786`; `0xbd/255 = 0.741176 → 0.508881`.
2. **linear sRGB → LMS**, then cube roots:
   `l = 0.4122214708·R + 0.5363325363·G + 0.0514459929·B`, and the two sibling rows.
3. **LMS′ → Oklab**: `L = 0.702977`, `a = −0.100765`, `b = −0.042096`.
4. **Oklab → OKLCH**: `C = √(a² + b²) = √(0.010154 + 0.001772) = 0.109205`;
   `h = atan2(b, a) = atan2(−0.042096, −0.100765) = 202.67°` (third quadrant, so +180°).

**`--accent: oklch(70.30% 0.1092 202.67)`**, which round-trips back to `#2fb3bd` exactly.

Two checks that matter more than the digits:

- **Distance from running-green.** `--run` is `#43b95e` = `oklch(69.82% 0.1671 147.96)`.
  `202.67 − 147.96 = 54.71°`. `docs/DESIGN-SYSTEM.md` asks for "~50° off running-green so the two
  never read as the same signal". Satisfied, with margin, and — the part that actually matters for
  colourblind users — the two also differ in chroma (0.109 vs 0.167), so they separate even when
  hue does not.
- **Distance from the banned tell.** The indigo/violet slop pair is `#6366f1`/`#8b5cf6`, hues
  ≈ 276°/303° in OKLCH. The accent is 73–100° away. Nothing in this palette lands in that arc;
  `--drift` at 316.7° is the closest, is a *status* colour, and is not emitted by Phase 2 at all
  (it needs the drift signal Phase 2 cannot derive). It is defined so Phase 3 does not invent a
  second purple.

Every other value in §5.3 was computed the same way and **round-trips to its source hex exactly**;
all 18 are in the sRGB gamut (verified by the inverse transform, no channel outside
[−0.0005, 1.0005]).

WCAG contrast, computed against `--page` `#0a0908`:

| Token | On page | Verdict |
|---|---|---|
| `--ink` | 17.09 : 1 | body text |
| `--ink-2` | 7.63 : 1 | secondary text, passes AA at any size |
| `--ink-3` | 3.44 : 1 | **large text and non-text only.** This is the floor for anything a user must read |
| `--ink-4` | 1.92 : 1 | **never text.** Hairline glyphs, the rail's thread lines, disabled glyphs only |
| `--accent` | 7.87 : 1 | |
| `--run` / `--need` / `--fail` | 7.90 / 9.04 / 6.07 : 1 | |

`prototype/app.html` uses `--ink-4` for 9 px section headers (`.sech`, `.mhd`, `.cf`). At 1.92 : 1
that is unreadable and it is one of the four places the prototype is not carried forward verbatim.
Section headers move to `--ink-3`. Recorded as a deliberate change so a reviewer comparing against
the prototype does not file it as drift.

On-accent ink: `--accent-ink: oklch(18% 0.030 202.7)` = `#001517`, **7.39 : 1** against the accent
fill. The prototype's `#04201f` is close but was eyeballed; this one is derived from the same hue
and states its ratio.

### 5.3 The file, in full

```css
/* desktop/src/styles/tokens.css
 *
 * The Panacea App token layer. docs/DESIGN-SYSTEM.md governs every value here.
 *
 * Colour space is OKLCH so light and dark are DERIVED rather than inverted, and so equal
 * numeric steps read as equal perceptual steps in the bottom quarter of the range, which is
 * where this interface lives. Requires Chromium 111+ / Safari 15.4+; the Tauri 2 webview is
 * WebView2 and WKWebView, both well past that, so there is no fallback ladder.
 *
 * Rules a reviewer can check:
 *   1. No component CSS contains a literal hex. Every colour is a var() from this file.
 *   2. The accent appears ONLY on interactive and current state. Never on a card, an icon,
 *      a border or a fill that is not carrying "you can act here" or "this is selected".
 *   3. Elevation is a 1px ring PLUS a layered shadow. Never a flat border.
 *   4. Radius is tied to element size: chip 6 / control 8 / card 10 / window 14.
 *   5. Status colour is a separate vocabulary from the accent and always ships with a shape.
 */

:root {
  color-scheme: dark;

  /* ── Surfaces, back to front ──────────────────────────────────────────────────────
   * page → canvas → surface → inset → hover → hover-2 → field.
   * One warm hue (67°, a warm graphite) with chroma rising alongside lightness, so the
   * ramp stays warm instead of desaturating to neutral grey as it lightens. Nearly every
   * generated dark UI is cold-tinted; warmth reads as considered on sight.
   * Lightness steps: 14.0 → 16.0 → 18.5 → 21.0 → 23.5 → 26.0. */
  --page:    oklch(14.0% 0.0040 67);   /* #0a0908 — the window ground */
  --canvas:  oklch(16.0% 0.0045 67);   /* #0f0d0b — rail, sub-rail, the Canvas backdrop */
  --surface: oklch(18.5% 0.0050 67);   /* #141210 — cards, popovers, menus, the top bar's controls */
  --inset:   oklch(21.0% 0.0060 67);   /* #1a1815 — a selected row, a chip, a segmented control's thumb */
  --hover:   oklch(23.5% 0.0065 67);   /* #201e1b — hover on a row */
  --hover-2: oklch(26.0% 0.0070 67);   /* #262320 — hover on something already at --inset */
  /* A form field is a WELL, not a raised plane: it reads as something you put text into.
   * This deliberately breaks the monotone ramp; DESIGN-SYSTEM lists the seven surfaces as a
   * naming order, not a lightness order. If a reviewer disagrees, this is the one line to
   * change. */
  --field:   oklch(12.5% 0.0035 67);   /* #070605 */

  /* ── Ink ramp ─────────────────────────────────────────────────────────────────────
   * ink-3 is the FLOOR for anything a user must read (3.44:1 on --page).
   * ink-4 is 1.92:1 and is never text: thread lines, hairline glyphs, disabled marks. */
  --ink:   oklch(94.8% 0.0097 73);     /* #f2ede7 — 17.09:1 */
  --ink-2: oklch(70.7% 0.0148 68);     /* #a79f97 —  7.63:1 */
  --ink-3: oklch(50.9% 0.0149 64);     /* #6c645d —  3.44:1 */
  --ink-4: oklch(37.2% 0.0130 67);     /* #453f39 —  1.92:1, non-text only */

  /* ── Hairlines ────────────────────────────────────────────────────────────────────
   * Hairlines carry STRUCTURE; shadows carry ELEVATION. Kept as alpha over the warm ink
   * so they read correctly on every surface in the ramp without a per-surface variant. */
  --line-soft:   oklch(94.8% 0.0097 73 / 0.04);
  --line:        oklch(94.8% 0.0097 73 / 0.075);
  --line-strong: oklch(94.8% 0.0097 73 / 0.15);
  /* The rail's parent→child connector. Structurally a hairline, semantically a thread. */
  --thread:      oklch(94.8% 0.0097 73 / 0.16);

  /* ── Accent — interactive and current state ONLY ──────────────────────────────────
   * Deep teal. Maximally distant from the indigo/violet tell (73–100° of hue away) and
   * 54.7° off running-green so the two never read as the same signal.
   * If the accent starts appearing decoratively, the hierarchy is broken. */
  --accent:     oklch(70.3% 0.1092 202.7);          /* #2fb3bd */
  --accent-hi:  oklch(82.7% 0.0950 202.7);          /* #74d9e1 — hover on an accent fill, and accent text */
  --accent-lo:  oklch(56.0% 0.0900 202.7);          /* #18848c — pressed */
  --accent-ink: oklch(18.0% 0.0300 202.7);          /* #001517 — text ON an accent fill, 7.39:1 */
  --accent-tint: oklch(70.3% 0.1092 202.7 / 0.11);  /* selected-row wash */
  --accent-ring: oklch(70.3% 0.1092 202.7 / 0.28);  /* the ring on a selected row */

  /* ── Status — a SEPARATE vocabulary that never collides with the accent ───────────
   * Each also carries a SHAPE in the component layer (§7.5): colour alone fails for
   * colourblind users. Phase 2 emits running / idle / exited only; needs-you and drift
   * are defined so Phase 3 does not invent a second amber and a second purple. */
  --run:   oklch(69.8% 0.1671 148);    /* #43b95e — running */
  --need:  oklch(75.8% 0.1370 75.8);   /* #e2a33c — needs you (Phase 3+) */
  --fail:  oklch(66.7% 0.1830 32.3);   /* #ef5f45 — failed */
  --drift: oklch(72.7% 0.1378 316.7);  /* #c98ae0 — drifting (Phase 3+) */
  --run-tint:  oklch(69.8% 0.1671 148  / 0.12);
  --need-tint: oklch(75.8% 0.1370 75.8 / 0.12);
  --fail-tint: oklch(66.7% 0.1830 32.3 / 0.12);
  /* "idle" and "exited" are NEUTRAL on purpose. A colour for every state is a colour for
   * no state. */
  --idle:   var(--ink-3);   /* amended: --ink-4 is 1.92:1, below the 3:1 floor this file asserts */
  --exited: var(--ink-3);

  /* ── Radius: tied to element size, not one value everywhere ───────────────────────*/
  --r-chip: 6px;    /* status pills, kbd hints, the tiny mono badges */
  --r-ctl: 8px;     /* buttons, inputs, segmented controls, rail rows */
  --r-card: 10px;   /* cards, panes, tiles, popovers */
  --r-window: 14px; /* the palette, modals — anything that reads as its own window */

  /* ── Elevation: a 1px ring PLUS a layered shadow. Never a flat border. ─────────────
   * Five steps. The ring is what makes the edge legible on a dark ground; the shadow is
   * what places it in depth. Either alone looks wrong, which is the whole point. */
  --e-hair:    0 0 0 1px var(--line);
  --e-btn:     0 0 0 1px var(--line), 0 1px 1px oklch(0% 0 0 / 0.35);
  --e-card:    0 0 0 1px var(--line), 0 1px 2px oklch(0% 0 0 / 0.40), 0 4px 12px oklch(0% 0 0 / 0.22);
  --e-raised:  0 0 0 1px var(--line-strong), 0 2px 4px oklch(0% 0 0 / 0.45), 0 14px 36px oklch(0% 0 0 / 0.40);
  --e-overlay: 0 0 0 1px var(--line-strong), 0 4px 8px oklch(0% 0 0 / 0.50), 0 30px 70px oklch(0% 0 0 / 0.60);

  /* ── Motion ───────────────────────────────────────────────────────────────────────
   * One house curve on nearly every transition. Durations under 300ms.
   * transform/opacity ONLY. `transition: all` is banned — it animates layout properties
   * by accident and forces a paint on every unrelated style change. */
  --ease: cubic-bezier(0.23, 1, 0.32, 1);
  --dur-fast: 120ms;
  --dur: 180ms;
  --dur-slow: 260ms;
  /* One global heartbeat. Every live indicator shares this period AND its phase; see
   * §7.1 — the phase is aligned by a negative animation-delay from a shared epoch, not by
   * hoping ten elements mounted on the same frame. */
  --beat: 2400ms;

  /* ── Type ─────────────────────────────────────────────────────────────────────────
   * Geist Sans and a licensed mono are the production pairing in docs/DESIGN-SYSTEM.md.
   * Phase 2 ships neither webfont: the app is offline-first on a corporate laptop, and a
   * font that fails to load is a layout shift. These stacks are the local fallbacks, and
   * swapping the first entry for Geist/Berkeley Mono when the licence lands is one line.
   * The `--mono` stack is the one xterm.js is configured with, so it must stay a real
   * monospace on every platform. */
  --sans: "Segoe UI Variable Text", "Segoe UI", -apple-system, system-ui, sans-serif;
  --mono: "Cascadia Mono", ui-monospace, "SF Mono", Menlo, Consolas, "DejaVu Sans Mono", monospace;

  /* Live numbers — tokens, cost, elapsed, attach count — get tabular figures or the digits
   * jitter and the interface reads cheap. Applied on <body> and inherited; §7.3. */
  --numeric: tabular-nums;
}

/* Grain: 1.5–3% film grain over dark surfaces. It kills banding on the dark ramp and adds a
 * made-object quality. Applied ONCE, on the app shell, as a fixed pseudo-element — not per
 * component, which would cost a compositor layer each. pointer-events:none so it never eats
 * a click. Correctly tuned you would only notice it if it were removed. */
.grain::after {
  content: "";
  position: fixed;
  inset: 0;
  z-index: 9999;
  pointer-events: none;
  opacity: 0.022;
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='120' height='120'%3E%3Cfilter id='n'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.8' numOctaves='3'/%3E%3C/filter%3E%3Crect width='120' height='120' filter='url(%23n)'/%3E%3C/svg%3E");
}

/* The floor. prefers-reduced-motion variants on everything, per docs/DESIGN-SYSTEM.md. */
@media (prefers-reduced-motion: reduce) {
  *,
  *::before,
  *::after {
    animation-duration: 0.01ms !important;
    animation-iteration-count: 1 !important;
    transition-duration: 0.01ms !important;
    scroll-behavior: auto !important;
  }
}

/* Focus rings are a designed element, not an accessibility afterthought: keyboard is a
 * primary input here. :focus-visible only, so a mouse click never draws one. */
:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 1px;
  border-radius: var(--r-chip);
}
```

### 5.4 Compatibility with Phase 4

`docs/superpowers/plans/2026-09-18-phase-4-discover.md` task 10 step 4 creates its own
`tokens.css` **only if Phase 2 has not shipped one**, and its comment says: "Phase 2 may
re-derive these in OKLCH; when it does, **keep the variable NAMES** so no component changes."

Every name Phase 4 lists is present above with the same meaning: `--page`, `--canvas`,
`--surface`, `--inset`, `--hover`, `--ink`, `--ink-2`, `--ink-3`, `--ink-4`, `--line-soft`,
`--line`, `--line-strong`, `--accent`, `--accent-hi`, `--accent-tint`, `--run`, `--need`,
`--fail`, `--drift`, `--r-chip`, `--r-ctl`, `--r-card`, `--r-window`, `--e-hair`, `--e-card`,
`--ease`, `--mono`, `--sans`. Task 14 has a test asserting all 28 exist, so a later edit cannot
silently break Phase 4.

Names Phase 2 adds: `--hover-2`, `--field`, `--thread`, `--accent-lo`, `--accent-ink`,
`--accent-ring`, `--run-tint`, `--need-tint`, `--fail-tint`, `--idle`, `--exited`, `--e-btn`,
`--e-raised`, `--e-overlay`, `--dur-fast`, `--dur`, `--dur-slow`, `--beat`, `--numeric`.

### 5.5 What is NOT in the stack, and why

`docs/DESIGN-SYSTEM.md` names Tailwind v4 + shadcn/ui, Motion v13 and Kibo UI. Phase 2 installs
**none of them**, and this is a decision a reviewer should either accept or overturn now rather
than at task 20:

- **Tailwind v4 + shadcn/ui.** Adding them is a build-pipeline change plus a component-generator
  surface, and the Phase 4 plan — already written — ships plain CSS (`discover.css`) against this
  same token layer. Two styling systems in one app is worse than either. Phase 2 ships plain
  per-component CSS files with a component prefix (`rail-`, `sub-`, `top-`, `pane-`, `cv-`,
  `pal-`, `menu-`, `dot-`), each beginning `@import "../styles/tokens.css";`, matching Phase 4's
  convention exactly. Revisit at Phase 5, when the Deploy surface brings tables and forms that
  shadcn actually earns.
- **Motion v13.** The only motion Phase 2 needs is a popover fade, a menu fade, and the heartbeat
  — all CSS, all `transform`/`opacity`, all cheaper than a JS animation loop. Canvas tile drag
  uses Pointer Events writing `transform: translate3d(...)`, as `prototype/app.html` already does.
  DESIGN-SYSTEM's own preference order is **CSS > WAAPI > JS library**; Phase 2 never leaves the
  first rung. Motion earns its place when FLIP reorder arrives with Phase 3's merge queue.
- **Kibo UI.** Its composites (code block, table, kanban, dropzone) have no consumer in Phase 2.
---

## 6. The component tree

Every file, its props, and the one thing it owns. A component "owns" a concern when it is the
only place that concern may be changed — two owners of focus is the bug that produces a cursor
that jumps. **One owner does not mean one call site**: §6.8's focus row has two movers under one
owner, and what makes that safe is that their triggers are disjoint and their target is the same.
The thing to forbid is two parties that can *disagree*, not two lines that both call `.focus()`.

```
desktop/src/
├─ main.tsx                      unchanged
├─ App.tsx                       REWRITTEN — mounts AppShell, starts the poller, nothing else
├─ api/
│  ├─ client.ts                  §4
│  ├─ index.ts                   the `api` singleton
│  ├─ attachRegistry.ts          sockets + xterm instances, ref-counted, module-level
│  └─ poller.ts                  the one interval
├─ store/                        §3
├─ lib/
│  ├─ wire.ts                    exists, unchanged
│  ├─ ids.ts                     §3.8
│  ├─ immutable.ts               §3.9
│  └─ time.ts                    formatElapsed / formatWaiting — §7.4
├─ hooks/
│  ├─ heartbeat.ts               §7.1 — the ONE clock
│  ├─ useKeyboardMap.ts          §6.8 — the ONE keydown listener
│  └─ useDismissable.ts          outside-click + Escape for popovers and menus
├─ styles/
│  ├─ tokens.css                 §5
│  └─ app.css                    resets, scrollbars, body type; imports tokens.css
├─ test/
│  ├─ setup.ts                   §1.4
│  ├─ fakes.ts                   FakeTerm, FakeWebSocket, fixture builders
│  └─ fixtures.ts                aMachine(), aProject(), aBranch(), aWorktree(), aSession(), aHarness()
└─ components/  …§6.1–6.9
```

### 6.1 Shell

| File | Props | Owns |
|---|---|---|
| `components/AppShell.tsx` | `{}` | The three-column grid (rail / sub-rail / main), the `.grain` layer, the single `useKeyboardMap()` mount, and `<Toasts/>`. Owns **no** focus and **no** scroll. |

```tsx
export function AppShell(): JSX.Element;
```

Layout is CSS grid, `grid-template-columns: <rail> <sub-rail> 1fr`, with the rail collapsing to
`0` via `grid-template-columns` transition on `--dur`. The sub-rail is present only when a branch
is selected, matching `prototype/app.html`'s `.app.deep .sub` (width 0 → 182px). Transitioning
`grid-template-columns` animates layout; the prototype transitions `width`/`flex-basis` for the
same reason. Both are layout properties and both are the one exception this plan allows, because
a rail that appears instantaneously reads as a glitch — it is bounded to two elements, runs once
per toggle, and is disabled under `prefers-reduced-motion`.

### 6.2 Rail — `components/rail/`

`docs/FUNCTIONAL-SPEC.md`: "machine chip at top, then projects → branches → worktrees → sessions.
Right-click any row for its verbs."

| File | Props | Owns |
|---|---|---|
| `Rail.tsx` | `{}` | The scroll container, the `role="tree"` wrapper, keyboard traversal within the tree. |
| `MachineChip.tsx` | `{ machine: Machine \| undefined }` | The machine name, its state dot, and the `unsandboxed` badge when `isolation !== "microvm"`. Click opens the machines popover. |
| `ProjectRow.tsx` | `{ project: Project; selected: boolean; expanded: boolean }` | One project row + its chevron. Renders `<BranchRow/>` children when expanded. |
| `BranchRow.tsx` | `{ branch: Branch; selected: boolean; expanded: boolean }` | One branch row. Click selects the branch **and** sets lens `develop`. |
| `WorktreeRow.tsx` | `{ worktree: Worktree; expanded: boolean }` | One worktree row + the `＋ session` affordance that appears on hover/focus. Chevron rendered only when `sessionCount > 0`. |
| `SessionRow.tsx` | `{ sessionId: SessionId; selected: boolean }` | One session row: status dot + harness command + title. Click focuses the session and scrolls its pane into view. |
| `RailFooter.tsx` | `{}` | The user chip, the `⌘K` hint, and the single API-error line (`lastError`). |
| `rail.css` | — | `.rail-*` |

```tsx
export function Rail(): JSX.Element;
export function MachineChip(props: { machine: Machine | undefined }): JSX.Element;
export function ProjectRow(props: { project: Project; selected: boolean; expanded: boolean }): JSX.Element;
export function BranchRow(props: { branch: Branch; selected: boolean; expanded: boolean }): JSX.Element;
export function WorktreeRow(props: { worktree: Worktree; expanded: boolean }): JSX.Element;
export function SessionRow(props: { sessionId: SessionId; selected: boolean }): JSX.Element;
export function RailFooter(): JSX.Element;
```

**Right-click verbs**, per row kind. Every one is dispatched through `openContextMenu` and
rendered by the single `<ContextMenu/>`; no row renders its own menu.

| Row | Verbs |
|---|---|
| project | New branch *(disabled in Phase 2 — no endpoint, §10.3, with the reason in the tooltip)*, Copy path, Reveal in Explorer *(Tauri opener plugin, already a dependency)* |
| branch | New session here, Copy branch name, Set branch defaults… |
| worktree | New session here, Open a shell here, Copy path, Mark ready *(always rendered: §10.2 landed, so `ready` is always on the wire)* |
| session | Resume, Fork into…, Restart, Rename…, Kill, Forget *(purge)* |

The rail carries **names and one status dot per row, and no numbers** — `docs/AGENTIC-UX.md` §10
rule 10. `worktree.sessionCount` decides whether a chevron is drawn and is never printed. A
branch row's dot is the **highest-attention status among its sessions**, resolved by the fixed
precedence `running > starting > idle > exited`; there is no aggregate count behind it and
clicking the dot navigates to the first session carrying that status, satisfying rule 7
(every aggregate names the fact that produced it).

### 6.3 Sub-rail — `components/subrail/`

| File | Props | Owns |
|---|---|---|
| `SubRail.tsx` | `{}` | The six lens rows, the back-link to the project, and the branch name. |
| `subrail.css` | — | `.sub-*` |

```tsx
export const LENSES: ReadonlyArray<{ id: Lens; label: string; glyph: string }> = [
  { id: "discover", label: "Discover", glyph: "◇" },
  { id: "design",   label: "Design",   glyph: "◈" },
  { id: "develop",  label: "Develop",  glyph: "◆" },
  { id: "files",    label: "Files",    glyph: "▤" },
  { id: "merge",    label: "Merge",    glyph: "⑂" },
  { id: "deploy",   label: "Deploy",   glyph: "▲" },
];
export function SubRail(): JSX.Element;
```

No row is ever disabled, numbered, ordered with arrows, or marked complete —
`docs/AGENTIC-UX.md` §4 rule 12 and §9 item 1. Five of the six render `LensPlaceholder`.

| File | Props | Owns |
|---|---|---|
| `components/lens/LensPlaceholder.tsx` | `{ lens: Lens; phase: number; what: string }` | The honest empty state: what the lens will do, and which phase brings it. No spinner, no "coming soon" badge. |

### 6.4 Top bar — `components/topbar/`

`docs/FUNCTIONAL-SPEC.md`: "layout toggle, Authority, Run locally, and the split New-session
button."

| File | Props | Owns |
|---|---|---|
| `TopBar.tsx` | `{}` | The bar. Renders the layout toggle, Authority, Run locally and the split button. |
| `LayoutToggle.tsx` | `{ layout: Layout; onChange(l: Layout): void }` | The Columns/Canvas segmented control. |
| `NewSessionButton.tsx` | `{ worktreeId: WorktreeId \| null }` | The split button. Left half = create with resolved defaults, no dialog. Right half (caret) = open the picker. |
| `SessionPicker.tsx` | `{ open: boolean; worktreeId: WorktreeId \| null; onClose(): void }` | The popover: harness, model, effort, permission mode, worktree, opening prompt. Owns its own draft state. |
| `topbar.css` | — | `.top-*` |

```tsx
export function TopBar(): JSX.Element;
export function LayoutToggle(props: { layout: Layout; onChange: (l: Layout) => void }): JSX.Element;
export function NewSessionButton(props: { worktreeId: WorktreeId | null }): JSX.Element;
export function SessionPicker(props: { open: boolean; worktreeId: WorktreeId | null; onClose: () => void }): JSX.Element;
```

**Authority** and **Run locally** are rendered and **disabled**, each with a tooltip naming the
phase that makes it real (Authority needs the runtime signal, `docs/AGENTIC-UX.md` §7; Run locally
needs Phase 5's deploy connector). They are in `docs/FUNCTIONAL-SPEC.md`'s top bar, so deleting
them would be a silent scope cut; rendering them live would be a lie. A disabled control with a
reason is the honest third option.

`SessionPicker` rules, all from `docs/FUNCTIONAL-SPEC.md` "Starting a session":

- The model list is **filtered to what that harness supports** (`harness.models`), and the control
  is hidden — not disabled — when `models` is empty.
- Effort is offered only when `harness.efforts.length > 0` ("Claude Code only" today, but the
  gate is the descriptor, never a hard-coded `harness === "claude-code"`).
- Permission mode is offered only when `harness.permissionModes.length > 0`.
- Opening prompt is offered only when `harness.supportsOpeningPrompt`. "blank just opens the
  session" — the field is never required.
- An unavailable harness is listed, not hidden, and shows `harness.note` verbatim. Hiding Codex
  because it is not installed makes "why can't I use Codex?" unanswerable from the UI.
- Submitting calls `toCreateRequest` (§3.6), so a field the harness does not accept is never sent.

### 6.5 Develop — `components/develop/`

| File | Props | Owns |
|---|---|---|
| `DevelopLens.tsx` | `{}` | Chooses Columns vs Canvas vs `EmptyDevelop`. Owns nothing else. |
| `ColumnsLayout.tsx` | `{ sessionIds: SessionId[] }` | **The horizontal scroll container**, and scroll-into-view of a newly created or newly focused pane (defect 4). |
| `CanvasLayout.tsx` | `{ sessionIds: SessionId[] }` | The 2400×1600 scrollable plane, tile drag, tile resize, z-order, and writing `TilePosition` to the store (defect 6). |
| `SessionPane.tsx` | `{ sessionId: SessionId; variant: "column" \| "tile" }` | One session: header, `<Terminal/>`, the attach overlay (§7.7), `<LinkedWorktreeNote/>` after the body *(sandbox git fix, 2026-09-23)*, footer. **Owns focus intent** — clicking anywhere in the pane focuses the session and hands DOM focus to the terminal — and tells `<Terminal/>` whether this pane is the focused one, so nine mounting panes do not each take the caret. Restart is `api.restartSession`, which keeps the session id. |
| `SessionPaneHeader.tsx` | `{ session: Session; worktree: Worktree \| undefined; variant: "column" \| "tile" }` | Status dot, worktree name, harness command, the `⋯` button. In `tile` variant it is also the drag handle. |
| `SessionPaneFooter.tsx` | `{ session: Session }` | harness command · model · elapsed · attach state. All numbers `tabular-nums`. Attach state comes from `useAttachState(session.id)` (§6.6), never from the store; the retry elapsed is `useHeartbeat() - state.ladderStartedAt`, computed here because the registry owns no clock. |
| `SessionMenu.tsx` | `{ sessionId: SessionId; anchor: DOMRect \| null; onClose(): void }` | The `⋯` menu: Resume, Fork into…, **Restart**, Rename…, Open a shell here, Kill, Forget, Copy session id, and **Retry attach** *(only while `useAttachState(id).phase === "failed"` — §6.6)*. Mounted by `ContextMenu.tsx` (§6.7). The rows are built by `develop/sessionVerbs.ts`, which the rail's menu shares. |
| `EmptyDevelop.tsx` | `{ branch: Branch \| undefined }` | The zero state. Named actions, no spinner, and `<LinkedWorktreeNote/>` before them *(sandbox git fix)*. Also exports `startSessionOnWorktree(worktreeId, defaults)`, the create path its two actions share — an addition to this table, sited here because they are its only callers. Distinct from task 18's `useCreateSession`, which re-resolves against the live store. |
| `LinkedWorktreeNote.tsx` | `{ worktree: Worktree \| undefined }` | ADDED by the sandbox git fix (2026-09-23). One sentence, `LINKED_WORKTREE_GIT_NOTE`, for `worktree.isPrimary === false`: git is unavailable in a linked worktree's sessions, because its `.git` names a host path the sandbox cannot see, and that arrives in Phase 3 (`3-D1`). Nothing for the primary worktree, and nothing for an unloaded one. No wire field, no §2 change. AMENDED by the sandbox git fix, round 1 (2026-09-24): the premise "the primary worktree's git works" was false on a host with Git for Windows' system `core.autocrlf=true`, where the sandbox's git saw every file modified and `commit -a` rewrote line endings. The sandbox's git now takes the host's `core.autocrlf` and `core.eol` for the worktree, and the primary's `.git/worktrees` is read-only in the sandbox (Go contract §5.1 step 5), so the premise holds and "not primary" is again exactly the set without working git. |
| `develop.css` | — | `.pane-*`, `.cols-*`, `.cv-*` |

```tsx
export function DevelopLens(): JSX.Element;
export function ColumnsLayout(props: { sessionIds: SessionId[] }): JSX.Element;
export function CanvasLayout(props: { sessionIds: SessionId[] }): JSX.Element;
export function SessionPane(props: { sessionId: SessionId; variant: "column" | "tile" }): JSX.Element;
export function SessionPaneHeader(props: { session: Session; worktree: Worktree | undefined; variant: "column" | "tile" }): JSX.Element;
export function SessionPaneFooter(props: { session: Session }): JSX.Element;
export function SessionMenu(props: { sessionId: SessionId; anchor: DOMRect | null; onClose: () => void }): JSX.Element;
export function EmptyDevelop(props: { branch: Branch | undefined }): JSX.Element;
export function LinkedWorktreeNote(props: { worktree: Worktree | undefined }): JSX.Element | null;
```

The footer shows what Phase 2 can honestly know: `claude · opus · 41m · attached`. It does **not**
show cost, token count or a context meter — the prototype's `ctxdots`, `cost` and `acct` are all
synthetic, none is derivable from a PTY, and capacity arrives in Phase 3. `docs/AGENTIC-UX.md` §2
property 7: if you cannot name the fact that justifies the element, the element must not exist.

### 6.6 Terminal — `components/Terminal.tsx` (modified)

| File | Props | Owns |
|---|---|---|
| `Terminal.tsx` | `{ sessionId: SessionId; readOnly: boolean; focused: boolean }` | Constructing the `XTerm`, installing **the chord filter** on it (below), calling `open()`, handing it to `attachRegistry`, and **vertical scrollback inside the pane**. Takes DOM focus **only when `focused`** — on mount and on a false→true transition. Unconditional focus-on-mount meant nine mounting panes each took the caret and the last one flushed kept it. |

```tsx
export function Terminal(props: {
  sessionId: SessionId;
  readOnly: boolean;
  focused: boolean;
}): JSX.Element;
```

The prop is a **session id, not a URL**. The old signature `{ url: string }` makes the socket a
render-time concern; the registry keyed by session id is what makes "disconnect does not destroy"
and "move a pane between layouts" both work.

`focused` is the third and it is not cosmetic: without it this component focused on every mount,
and React flushes passive effects children-before-parents in mount order, so Columns with nine
panes left the **ninth** terminal holding the caret while the focus ring follows
`selection.sessionId` — which is `null` on a cold open. `SessionPane` passes it; §6.8's focus row
records why the two movers cannot disagree.

`readOnly` is `session.status === "exited"`: the scrollback stays readable (the transcript is the
artifact) but `onData` is not wired, so a keystroke into a dead session does nothing rather than
silently vanishing into a closed socket.

**The chord filter — ADDED by task 23's fix round 1, a declared departure into task 19's closed
file.** It amends this section's "Owns" cell and §1.5's `Terminal.tsx` row, which both described
`makeTerm` as construct-open-hand-over and nothing else, and §1.5's `FakeTerm` block, which now
carries the method. xterm handles keydown on its own focused helper textarea, and for every key
it maps to data it sends the bytes to the agent and cancels the event, `stopPropagation`
included. So with a terminal focused — the state the cockpit spends its time in — six of §6.9's
chords never reached `useKeyboardMap`: `⌘3` went down the PTY as ESC and interrupted the agent,
`⌘⏎` as CR and submitted its half-typed prompt, and `⌘K`, `⌘W`, `⌘B` and `⌘\` as C0 bytes.
`makeTerm` therefore calls

```ts
term.attachCustomKeyEventHandler((event) => !isCockpitChord(event));
```

`isCockpitChord` is exported by `hooks/useKeyboardMap.ts` and is also that hook's own early
return, so the chord list exists **once** and the terminal and the map cannot disagree about it.
Returning `false` makes xterm skip the event **without cancelling it**, so it bubbles to the map.
Everything outside the predicate — Escape, shift+Tab, `/`, AltGr (Ctrl+Alt), Ctrl+C and every
bare key — is still xterm's to send, which is what §6.9's two spelled-out entries require.

**ADDED by task 19 — the `createTerm` seam, because neither §1.5 nor §1.7 describes it.** §1.5
puts the xterm import in this component; §1.7 puts the *instance* in the registry. A factory
satisfies both: `Terminal` passes `acquireAttach(session, { createTerm, readOnly })` and the
registry calls it **at most once per session id**, so `attachRegistry.ts` imports no xterm at all
and `attachRegistry.test.ts` needs no `vi.mock`. Two consequences the component owns and the
registry cannot: the registry hands back a **registry-owned `<div data-session-id>`** which the
component appends and later removes — re-parenting one element is what preserves rendered
scrollback when a pane moves between Columns and Canvas, because xterm does not support
re-`open()`ing across parents — and `handle.mount()` is separate from `acquireAttach`, called
**after** the append and idempotent, because xterm measures its cell size from a rendered element
and `open()` on a detached node yields a zero-column terminal.

**The published attach state, which no section described before.** §6.5's footer and §7.7's
"connecting" row both ask the pane for facts about the attach, and neither named the interface
that supplies them — which is how §7.7 came to ask for an elapsed that `AttachState` could not
produce. The registry publishes this, and the footer reads it through `useAttachState(sessionId)`,
**not** through the store: attach state is transport state, and putting it in the store would turn
every socket event into a store write and every store write into a re-render of the rail.

```ts
export type AttachPhase =
  | "detached" | "connecting" | "attached" | "reconnecting" | "ended" | "failed";

export interface AttachState {
  phase: AttachPhase;
  /** Handshakes tried in the current ladder; back to 0 once one succeeds. */
  attempts: number;
  /** `Date.now()` when the current ladder started, and 0 whenever none is running — so
   *  `ladderStartedAt !== 0` is exactly "this pane is retrying". */
  ladderStartedAt: number;
  /** A sentence written for a user. Rendered verbatim; never a code, never a stack. */
  reason: string;
}

export function useAttachState(sessionId: SessionId): AttachState;

/** Start a FRESH ladder for a pane whose previous one is spent. False, having done nothing,
 *  when there is no entry, it is destroyed, the session has exited, or a socket is already
 *  open or in flight. Added 2026-09-23 by task 22. */
export function retryAttach(sessionId: SessionId): boolean;
```

**`failed` is terminal, and `retryAttach` is the only way out of it.** Nothing in the registry
re-attempts once the 60 s budget is spent, and the pane's exit bar cannot offer the way out
because it is absent *by definition* — the socket died while the session is **not** exited, so
the Restart that answers `exited` never appears. Release-and-reacquire is not a retry either:
after the grace it destroys the terminal and loses the transcript, and inside the grace it hands
back the same exhausted entry. The verb that calls this is a row in task 22's `⋯` menu, rendered
only while `phase === "failed"`, so no new surface was needed — `sessionVerbs` reads a snapshot
at open time and `getAttachState` is a synchronous snapshot read.

**It resets the counters and calls `connect()` rather than stamping the clock itself**, and that
follows directly from the `ladderStartedAt` contract three paragraphs above: `!== 0` means "this
pane is retrying", and `connect()` already owns that write with its own conditional stamp. Two
writers of one invariant is the defect; leaving the zero in place would render a retrying pane
with no elapsed.

**The elapsed is computed by the pane, not by the registry.** The registry publishes the raw
stamp and owns no clock: the pane subtracts `ladderStartedAt` from `useHeartbeat()`'s 1 Hz tick
(§7.1) at render time and prints the difference in `var(--numeric)`. That is what keeps §7.1's
one-clock rule intact — a registry that published a ticking elapsed would need a second display
clock — and it is why §7.7's row asks for a number rather than a spinner and can now get one.

### 6.7 Overlays

| File | Props | Owns |
|---|---|---|
| `palette/CommandPalette.tsx` | `{}` | The `⌘K` surface: the input, the filtered list, arrow/Enter selection. Owns focus **while open**, and returns it on close. |
| `palette/commands.ts` | — | `buildCommands(state): Command[]` — the command set, pure, testable without rendering. |
| `menu/ContextMenu.tsx` | `{}` | The single right-click menu, positioned from `store.contextMenu`, clamped to the viewport. **Also mounts `<SessionMenu/>`** from `store.sessionMenuFor`, so it returns a fragment whose two halves open independently — the `⋯` menu must be able to open while no right-click menu is up, which is the normal case. |
| `common/StatusDot.tsx` | `{ status: SessionStatus \| MachineState; live: boolean; label: string }` | The status dot: **colour plus shape**, and the shared heartbeat phase when `live`. |
| `common/Toasts.tsx` | `{}` | The optimistic-mutation Undo window. Plain text only. |
| `common/Popover.tsx` | `{ anchor: DOMRect \| null; onClose(): void; children }` | Positioning, outside-click, Escape, and focus return. Used by the picker, the session menu and the machine chip. |

**`SessionMenu` takes no row of its own here**, and the omission is deliberate rather than
stale. This table lists overlays by the file that *owns* the surface; `SessionMenu` is a
presentational component with three props that renders **inside** `Popover`, and the row above
records who mounts it. Its inline rename input is likewise not a fourth overlay: it is a second
`useDismissable` entry within the popover that is already open, which is what makes §6.9's
"close the topmost overlay" step back to the verb list rather than closing the menu. It adds a
row to `useDismissable`'s runtime stack and none to this table, so §6.8's focus rule needs no
new case — focus returns through the popover it never left. *Amended 2026-09-23 by task 22.*

```ts
export interface Command {
  id: string;
  /** Rendered verbatim; matched case-insensitively against the query. */
  label: string;
  /** Right-aligned context, e.g. "engagement-portal / evidence-upload". */
  meta?: string;
  glyph: string;
  /** Shown inline so the palette teaches its own shortcuts. */
  hint?: string;
  run(): void;
}
export function buildCommands(state: CockpitState): Command[];
```

Palette ranking is **stable**: `buildCommands` returns a fixed order (verbs, then sessions, then
worktrees, then branches, then projects) and filtering only *removes* rows; it never re-ranks.
`docs/DESIGN-SYSTEM.md`: "Stable ranking; rows never reflow while filtering."

The palette selects by **id**, never by array index — `idAtVisiblePosition` from §3.9. This is
known defect 10's class and the palette is where Phase 2 actually contains it.

### 6.8 Focus and scroll — the ownership table

| Concern | Owner | Everyone else |
|---|---|---|
| Which session is *focused* (app state) | `uiSlice.focusSession` | Rail rows, Columns, the palette and `⌘1-9` all call it; none keeps its own copy |
| DOM focus inside a pane | **Two movers, disjoint by trigger** — `Terminal.tsx` (`term.focus()`) and `SessionPane.focusTerminalIn` | Nothing else moves focus into a pane. The triggers cannot both fire for one act, and they target the same element when they do — see the note below |
| DOM focus while an overlay is open | `CommandPalette` / `Popover` | They capture on open and **restore to the previously focused element** on close. A palette that drops focus to `<body>` makes `⌘K`, Escape, keep-typing impossible |
| Horizontal scroll of Columns | `ColumnsLayout` | `SessionRow` and the palette request it via `focusSession`; they never call `scrollIntoView` themselves |
| Vertical scrollback inside a pane | xterm, via `attachRegistry`'s pinned rule (§7.2) | Nothing else calls `scrollToBottom()` |
| Canvas pan/zoom | `CanvasLayout` | — |
| The keydown listener | `useKeyboardMap`, mounted once in `AppShell` | No component adds a `window` keydown listener. Panes use React's own `onKeyDown` where they need one. xterm's own textarea handler is not a second owner of the table: it defers every chord to `useKeyboardMap`'s `isCockpitChord` (§6.6, "The chord filter") |

**Why two movers is not a violation, and what the rule actually is.** Earlier wording named
`Terminal.tsx` as the only thing that calls `.focus()` inside a pane. **That was never true** —
`SessionPane.focusTerminalIn` has always moved focus on the pointer-down path, and the pane's own
suite pins it. The rule this section exists to protect is not *"exactly one function calls
`.focus()"*; it is **"no two movers can disagree about where the caret belongs"**, which is the
condition that produces the jumping cursor. Two movers satisfy it when their triggers are disjoint
and their target is the same, and these are:

| mover | fires when | cannot collide because |
|---|---|---|
| `Terminal.tsx`'s focus effect | `focused` goes **false→true**, including a mount that is already focused | A pane that is already focused produces no transition, so this never runs on the chrome-click path |
| `SessionPane.focusTerminalIn` | a **pointer-down on the pane chrome**, after the two early returns (an interactive control, or inside the terminal itself) | Reached only when the pane is already selected *or* is being selected by this same event — and in the second case both movers resolve to the same element, so the second call is idempotent |

The element is `textarea.xterm-helper-textarea` either way: `term.focus()` is xterm focusing its
own helper textarea, and `focusTerminalIn` looks that element up by selector. **So the worst case
is two calls to `.focus()` on one element in one tick, which is a no-op, not a fight.**

The case the second mover exists for: the `⋯` button takes the caret (deliberately — a menu must
be keyboard-drivable), and the user then clicks the pane chrome to get back to typing. The pane
was already focused throughout, so no transition fires and the first mover stays silent. Delete
`focusTerminalIn` and the caret stays on the button.

### 6.9 The keyboard map

`docs/FUNCTIONAL-SPEC.md` "Keyboard", implemented in `hooks/useKeyboardMap.ts` as one listener.
`⌘` is `metaKey || ctrlKey` — the app runs on Windows, where the physical key is Ctrl.

| Chord | Effect | Guard |
|---|---|---|
| `⌘K` | Toggle the palette | Always, including while a terminal has focus |
| `⌘⏎` | New default session in the focused worktree | Requires a selected branch; falls back to the branch's first worktree |
| `⌘⇧M` | Machines popover | Always |
| `⌘1`–`⌘9` | Focus the *n*th session in `selectPaneSessionIds` | Via `idAtVisiblePosition`; no-op past the end |
| `⌘\` | Focus the next session, wrapping | Same list |
| `⌘W` | Kill the focused session (optimistic + Undo toast) | Requires a focused session. **Does not close the window** — `preventDefault` is mandatory or the webview closes |
| `⌘B` | Toggle the rail | Always |
| `Escape` | Close the topmost overlay; if none, **do nothing** | See below |
| `shift+tab` | **Not intercepted.** Passed to the terminal | See below |
| `/` | Not intercepted | Slash commands belong to the agent CLI |

**"Including while a terminal has focus" needs the terminal's cooperation, and has it.** A
focused xterm sees a keystroke before any `window` listener and cancels what it maps to data.
The rows above marked `⌘` reach this map from inside a terminal only because `makeTerm` hands
xterm the map's own predicate, `isCockpitChord`, and xterm skips what it claims (§6.6, "The chord
filter"). So the predicate is the table: a key added to it is a key the agent stops receiving,
and the table's last three rows — Escape, `shift+tab`, `/` — are kept out of it on purpose.

Two entries need the reason spelled out, because both look like omissions:

- **`esc` interrupt.** `docs/FUNCTIONAL-SPEC.md` lists `esc` as "interrupt". In Phase 2 the
  interrupt is a real keystroke the agent CLI already handles: when a terminal has focus, xterm
  sends `\x1b` down the PTY and Claude Code interrupts itself. The app must therefore **not**
  intercept Escape while a terminal has focus, and must not synthesise an "interrupted" state —
  which is also why Escape is outside `isCockpitChord`: claimed there, xterm would skip it and the
  agent would never receive `\x1b`. When an overlay is open, the overlay consumes Escape first.
  That ordering is the whole rule.
- **`shift+tab` cycle permission mode.** Claude Code cycles its own permission mode on shift+tab,
  inside its own TUI. The cockpit intercepting it, mutating `session.permissionMode` and
  *not* telling the agent would put the two out of sync permanently — which is known defect 8
  wearing a different hat. The cockpit passes the key through and reads nothing back. The
  regression test asserts that `useKeyboardMap` does not `preventDefault` shift+tab and that
  `permissionMode` in the store is unchanged after it.

---

## 7. Cross-cutting rules, mapped to owners

### 7.1 One global heartbeat clock — `desktop/src/hooks/heartbeat.ts`

`docs/DESIGN-SYSTEM.md`: "Every live indicator shares a phase. Ten agents breathing on one
timeline reads as a system under control; ten independently-phased spinners read as noise."
And: "**Never a spinner per row.**"

Two exports, one file, one owner.

```ts
/** The shared epoch. Module load time, once per window. */
const EPOCH = performance.now();

/** Returns the style that aligns a CSS animation's PHASE to the shared epoch.
 *
 *  WHY a negative animation-delay rather than a JS-driven opacity: a CSS animation runs on the
 *  compositor and costs nothing per frame, but it starts its cycle when the ELEMENT mounts —
 *  so ten rows appearing at ten moments breathe out of phase, which is exactly the failure
 *  DESIGN-SYSTEM names. A negative delay of (now - EPOCH) mod --beat starts the animation
 *  already part-way through its cycle, so every element, whenever it mounted, is at the same
 *  point of the same breath. One line, zero JS per frame, perfect phase. */
export function beatDelayStyle(): { animationDelay: string };

/** A 1 Hz tick for LIVE NUMBERS ONLY (elapsed, waiting). One setInterval for the whole app,
 *  exposed through useSyncExternalStore so React subscribes rather than each component owning
 *  a timer. Returns the current epoch-ms, rounded to the second, so two components rendering
 *  in the same tick print the same number. */
export function useHeartbeat(): number;
```

`StatusDot` is the only component that calls `beatDelayStyle()`. `SessionPaneFooter` and any
"waiting" label are the only callers of `useHeartbeat()`. A component that needs neither must
subscribe to neither — `useHeartbeat` re-renders its caller once a second, and a rail row that
does that for 40 rows is the second most likely performance defect in Group D.

The CSS, in `common.css`:

```css
@keyframes beat { 0%, 100% { opacity: 0.45 } 50% { opacity: 1 } }
.dot-live { animation: beat var(--beat) var(--ease) infinite; }
```

`opacity` only, so it stays on the compositor, and it is inert under `prefers-reduced-motion` via
the `@media` block in `tokens.css`. **The breath stopping is not the whole story**: `running` and
`idle` are both `dot-disc`, so with the animation inert they would differ by colour alone — which is
what the shape vocabulary exists to prevent. §7.5 requires a compensating **static** ring on
`.dot-live` inside that same media query; read the two together.

### 7.2 Log panes: ~10 Hz, and never a scroll-jump — `desktop/src/api/attachRegistry.ts`

Two rules from `docs/DESIGN-SYSTEM.md`, one owner. Both live in the registry's message handler,
not in a component, because a component that re-renders on every frame of agent output is the
defect they exist to prevent.

```ts
/** Batching. Incoming stdout payloads accumulate in an array and flush on a 100 ms timer
 *  (~10 Hz), concatenated into ONE term.write per flush. A build log at 4 000 lines/second
 *  produces 10 writes a second, not 4 000. The timer is only armed when the buffer is
 *  non-empty, so an idle session costs nothing. */
const FLUSH_INTERVAL_MS = 100;

/** Pinned-scroll. Known defect 5: the prototype auto-scrolls every pane on every tick, which
 *  rips the scrollback away from a user who has scrolled up to read.
 *
 *  Measured BEFORE the write, because the write itself moves baseY:
 *      const pinned = term.buffer.active.viewportY >= term.buffer.active.baseY;
 *      term.write(chunk, () => { if (pinned) term.scrollToBottom(); });
 *
 *  A pane scrolled up stays exactly where it is, forever, until the user scrolls back down.
 *  xterm's own `scrollback` (default 1 000 lines) is raised to 10 000 so scrolling up is
 *  worth doing. */
```

The write callback is used rather than an immediate `scrollToBottom()` because `term.write` is
asynchronous — xterm parses on its own schedule, and scrolling before the parse lands leaves the
viewport one chunk short.

### 7.3 `tabular-nums` on every live number

`--numeric: tabular-nums` is declared in `tokens.css` and applied once, in `app.css`:

```css
body { font-variant-numeric: var(--numeric); }
```

Inherited everywhere, so no component opts in and none can forget. The one place it must be
overridden back to proportional is prose; Phase 2 has none dense enough to matter. The rule is
enforced by inspection in review, on: elapsed, waiting, attach count, tile coordinates, version
strings and any timestamp.

### 7.4 Time formatting — `desktop/src/lib/time.ts`

`docs/AGENTIC-UX.md` §10 rule 9: "No 'N ago' as a primary timestamp. Primary is 'since you last
looked' or 'waiting Xh Ym'."

```ts
/** "41m", "1h 06m", "2d". For how long a session has been alive. */
export function formatElapsed(fromIso: string, nowMs: number): string;
/** "waiting 5h 51m". For how long something has been idle without the user. Aging must be
 *  legible or wasted idle time is invisible. */
export function formatWaiting(sinceIso: string, nowMs: number): string;
```

Both take `nowMs` as a parameter rather than calling `Date.now()` internally — so they are pure,
so a test does not need fake timers, and so every caller in one render tick prints a consistent
number. `nowMs` comes from `useHeartbeat()`.

### 7.5 Motion, and the shapes that carry status

- `transform` and `opacity` only. **`transition: all` is banned**; every transition names its
  properties. The two layout-property exceptions are named in §6.1 (the rail collapse and the
  sub-rail reveal) and nowhere else.
- One house curve, `var(--ease)`; durations from `--dur-fast` / `--dur` / `--dur-slow`, all under
  300 ms.
- Status is **colour plus shape**, because colour alone fails for colourblind users:
  `running` filled circle (breathing), `starting` hollow ring, `idle` filled circle at
  `--idle` (no animation), `exited` a small square at `--exited`. `StatusDot` also renders a
  `title` and `aria-label` from its `label` prop, so the state is available to a screen reader
  and to a hover.
- State changes flash and decay over ~600 ms on `opacity`/`background` only. One class,
  `.dot-flash`, applied for 600 ms when `status` changes, removed by a timer the component owns.
- **Under `prefers-reduced-motion`, a live status carries a STATIC differentiator, because the
  breath is not available to carry it.** `running` and `idle` are both filled circles, told
  apart in normal motion by colour *and* by the breath; `tokens.css` zeroes `animation-duration`
  globally for that preference, which would leave colour alone — the failure this whole bullet
  list exists to prevent, and a WCAG 1.4.1 problem for a sighted user who is also colour-blind,
  whom the `aria-label` does not reach. So `common.css` gives `.dot-live` a non-inset
  `box-shadow` ring inside a `prefers-reduced-motion: reduce` block, yielding four static forms:
  hollow ring (`starting`), disc (`idle`), disc with a halo (`running`), square (`exited`).
  Shape separates every status in every motion mode and the breath is reinforcement, never the
  sole signal. Added by task 18 fix round 1, ruled by the contract owner.

### 7.6 Counts, badges and the one place a number is allowed

`docs/AGENTIC-UX.md` §10 rule 10: "No badge or count outside the queue. The rail carries names,
not numbers." Phase 2 has no queue, so Phase 2 has **no counts at all** in the rail, the sub-rail
or the top bar. `Worktree.sessionCount` is consumed only as `> 0` (§6.2). Numbers appear in
exactly three places, all of them facts about one object rather than aggregates: the pane footer's
elapsed time, the machine popover's state line, and the Canvas tile's own coordinates while it is
being dragged.

### 7.7 Every state is designed

`docs/DESIGN-SYSTEM.md` floor: "Every state designed: loading, empty, sparse, dense, partial,
stale, offline, unauthorized, error, blocked, success." The ones Phase 2 can actually reach, and
who owns each:

| State | Where it shows | Owner |
|---|---|---|
| control plane not running (`network_unavailable`) | Rail footer line + a full-width bar in the main area naming the command to start it | `RailFooter`, `AppShell` |
| Docker not running (`backendReady: false`, or `backend_unavailable`) | The machine chip's state goes `error` with `machine.message` verbatim | `MachineChip` |
| no projects (`PANACEA_APP_PROJECTS_DIR` unset) | Rail empty state naming the environment variable | `Rail` |
| branch with no worktrees | `EmptyDevelop` with "this branch has no worktrees yet" | `EmptyDevelop` |
| worktree with no sessions | `EmptyDevelop` with the two named actions (`＋ Claude Code`, `Open a shell`) | `EmptyDevelop` |
| linked worktree (`isPrimary: false`) — git does not work in its sandboxes | `LinkedWorktreeNote`'s one sentence, before `EmptyDevelop`'s actions and after every such pane's terminal body. ADDED by the sandbox git fix (2026-09-23) | `LinkedWorktreeNote`, placed by `EmptyDevelop` and `SessionPane` |
| the attach socket is not up — a first connect, a reconnect, or a ladder that gave up | Pane body shows the connecting line with the elapsed retry time, not a spinner — `useHeartbeat() - useAttachState(id).ladderStartedAt`, the interface for which is §6.6. An overlay **sibling** of `<Terminal/>`, never a conditional render of it, and never a scrim once a transcript may be behind it: centred only for `connecting`, a corner strip otherwise, and for `failed` the strip carries `AttachState.reason` verbatim. **`failed` also has a way forward**, which it did not when this row was written: `Retry attach` in the `⋯` menu, calling §6.6's `retryAttach`. The strip explains; the menu acts — this row stays the *explanation*, and a second action inside the overlay would be a control on a surface that is `pointer-events: none` | `SessionPane`, and `SessionMenu` for the action |
| session `exited` | Scrollback stays, read-only, with `exitReason` and a Restart action | `SessionPane` |
| harness unavailable | Listed in the picker with `harness.note` verbatim, not hidden | `SessionPicker` |
| more than 10 pins on Canvas | `pinSession` no-ops and toasts the cap | `prefsSlice` |

Skeletons mirror final geometry exactly, or are not used. Phase 2 uses none: every list either has
data within one request or has a designed empty state, and a skeleton that causes one pixel of
layout shift is worse than no skeleton.
---

## 8. The ten known defects

`docs/FUNCTIONAL-SPEC.md` lists ten defects "being fixed in this pass". Three of them
(7, 9, 10) live in prototype surfaces that Phase 2 does not build — Discover is Phase 4, Merge is
Phase 3. They are not deferred: each is a *defect class*, and the class is fixed here, in the
Phase 2 surface where the same mistake would otherwise be made, with a test that fails if it is.
The middle column says exactly where, so nobody has to take that on faith.

| # | `docs/FUNCTIONAL-SPEC.md` defect | In Phase 2 this is | Fixed by | Regression test |
|---|---|---|---|---|
| **1** | Live tick destroys in-progress typing | A store write (a 2 s poll, a status change, a layout toggle) must never dispose or remount the xterm instance the user is typing into | **19** — the terminal lives in `attachRegistry`, keyed by session id, ref-counted; `<Terminal/>` is `memo`'d on `sessionId` and never re-creates it | `src/api/attachRegistry.test.ts` → *"a store update does not dispose the terminal"*; `src/components/Terminal.test.tsx` → *"two renders with the same sessionId construct exactly one Terminal"* |
| **2** | `uid()` collides on fast successive calls | Client ids for toasts and transient keys | **15** — `makeId()` = monotonic counter **+** 8 chars of `crypto.randomUUID()`; every user-visible id comes from the server's `crypto/rand` | `src/lib/ids.test.ts` → *"10 000 successive makeId calls are unique"* and *"ids are unique across two module instances"* |
| **3** | Denying a permission dead-ends the session — composer must always render | The pane must render an input surface for **every** status, and a permission prompt answered "no" inside the PTY leaves a live, typeable terminal | **20** — `SessionPane` renders `<Terminal/>` unconditionally; `readOnly` is set only for `exited`, and even then the scrollback stays | `src/components/develop/SessionPane.test.tsx` → *"renders a terminal for starting, running, idle and exited"* and *"stdin stays wired for starting, running and idle"* |
| **4** | New session created off-screen with no focus | A `POST /v1/sessions` that lands outside the Columns viewport | **20** — `ColumnsLayout` calls `scrollIntoView({ inline: "center", block: "nearest" })` and `focusSession` on the new id | `src/components/develop/ColumnsLayout.test.tsx` → *"a newly focused session is scrolled into view and takes focus"* |
| **5** | Auto-scroll steals the scrollback | Agent output arriving while the user has scrolled up to read | **19** — pinned measured **before** the write, `scrollToBottom()` only in the write callback and only when pinned (§7.2) | `src/api/attachRegistry.test.ts` → *"output does not scroll a pane the user has scrolled up"* and *"output does scroll a pane pinned to the bottom"* |
| **6** | Canvas tile positions not persisted | Drag/resize a tile, switch to Columns and back, restart the app | **21** — `prefsSlice.tiles` keyed by session id, inside `partialize` | `src/components/develop/CanvasLayout.test.tsx` → *"a dragged tile writes its position to the store"*; `src/store/store.test.ts` → *"tiles survive a rehydrate at the current schema version"* |
| **7** | `editAnswer` splices all later answers | Any edit-one-of-many: renaming session 2 of 3, replacing one worktree in a list | **15** (the helper) consumed by **22** (rename) — `replaceAt` / `replaceById`, which replace exactly one | `src/lib/immutable.test.ts` → *"replaceAt replaces one element and keeps every later one"*; `src/components/develop/sessionVerbs.test.ts` → *"renaming the middle session leaves its neighbours unchanged"* |
| **8** | "Don't ask again for X" flips the whole session to bypass | The cockpit must not own a permission model at all: the agent's own TUI owns it, and shift+tab belongs to the agent | **23** — `useKeyboardMap` does not intercept shift+tab, does not `preventDefault` it, and never writes `permissionMode` after create | `src/hooks/useKeyboardMap.test.ts` → *"shift+tab is not intercepted and session.permissionMode is unchanged"*; *"Escape is not intercepted while a terminal has focus and no overlay is open"* |
| **9** | Nothing ever sets `ready`, so user-created worktrees can never merge | There is no endpoint to set it — §10.2 requested one | **22** — the "Mark ready" verb PATCHes the worktree and the flag survives a refetch. **No longer contingent:** §10.2 landed in task 10, so the verb is always rendered and task 22 does not check which world it is in | `src/components/develop/sessionVerbs.test.ts` → *"Mark ready PATCHes the worktree and the flag survives the next list"*. The second test this row used to name — *"the verb is absent when the server does not report `ready`"* — is **deleted**, not moved: the server always reports it, so that test would assert against a shape the control plane cannot produce |
| **10** | Merge reorder indexes the unfiltered array | Any index taken against a filtered list and applied to the full one: the palette while a query is typed, `⌘1-9` across a filtered pane list | **15** (the helper) consumed by **23** — `moveWithinVisible` / `idAtVisiblePosition`; the rule is *never pass an array index across a filter boundary, pass the id* | `src/lib/immutable.test.ts` → *"moveWithinVisible reorders only within the visible subsequence"*; `src/components/palette/CommandPalette.test.tsx` → *"Enter on the second visible row runs the second VISIBLE command, not commands[1]"* |

---

## 9. The task list — tasks 14 to 24

Eleven tasks. **D1 writes 14–19** (the foundation: nothing on screen until 18, everything on
screen depends on 14–17). **D2 writes 20–24** (the surfaces, the verbs and the proof). The line is
drawn so that D1 hands D2 a compiling, tested substrate with no UI decisions left in it, and D2
never edits a file D1 owns except by declared departure, recorded at the site and in the section
it amends. *Amended 2026-09-24 by task 23's fix round 2.* The sentence used to say "except
`App.tsx` in task 24", which was the one departure planned. D2's tasks and their review rounds
have since made others, into `Terminal.tsx`, `fakes.ts`, `poller.ts` and `useCreateSession.ts`,
each recorded where it happened.

Legend: **C** create, **M** modify, **T** test.

**Two entries in task 19's row are departures from this table as first written, and both are
argued in that task's plan rather than assumed.** `App.tsx` is §9-assigned to task 24, but task 19
changes `<Terminal/>`'s signature and `App.tsx` was its only caller, so the tree would not
typecheck; task 19 reduces it to a placeholder that reserves task 24's mount point and retires the
Phase 1 `ws://` harness with it. `test/fakes.ts` is task 14's, and Group D's shared shape §3.3
names task 19 as the one task that **grows** its `FakeTerm` rather than replacing it. Read the row
as the record of who actually touched what, which is the job §9 does for anyone reconstructing
this phase from the table alone.

### D1 — foundation (tasks 14–19)

| # | Title | Scope in one line | Files | Depends on |
|---|---|---|---|---|
| **14** | Test infrastructure and the token layer | vitest 5 + jsdom 30 + RTL 16 pinned, `vitest.config.ts`, `src/test/setup.ts`, the npm scripts, and `tokens.css` in full OKLCH | M `desktop/package.json`, `desktop/tsconfig.node.json`; C `desktop/vitest.config.ts`, `desktop/src/test/setup.ts`, `desktop/src/test/fakes.ts`, `desktop/src/test/fixtures.ts`, `desktop/src/styles/tokens.css`, `desktop/src/styles/app.css`; T `desktop/src/styles/tokens.test.ts` | — |
| **15** | Domain model, ids and the immutable helpers | `store/types.ts` as one compiling block; `makeId`; `replaceAt` / `replaceById` / `moveWithinVisible` / `idAtVisiblePosition`; `formatElapsed` / `formatWaiting` | C `desktop/src/store/types.ts`, `desktop/src/lib/ids.ts`, `desktop/src/lib/immutable.ts`, `desktop/src/lib/time.ts`; T `ids.test.ts`, `immutable.test.ts`, `time.test.ts` | 14 |
| **16** | The typed control-plane client | `ApiError`, `createClient`, one function per endpoint, `attachUrl`, the injected `fetch`/`WebSocket` seams | C `desktop/src/api/client.ts`, `desktop/src/api/index.ts`; T `desktop/src/api/client.test.ts` | 14, 15 |
| **17** | The store: slices, persistence, inheritance | three flat slices, `persist` with `PERSIST_KEY`/version 1/`migrate`, `selectors.ts`, `resolveDefaults` and `toCreateRequest` | C `desktop/src/store/store.ts`, `store/slices/dataSlice.ts`, `store/slices/uiSlice.ts`, `store/slices/prefsSlice.ts`, `store/selectors.ts`, `store/defaults.ts`; M `desktop/package.json` (zustand); T `store/store.test.ts`, `store/defaults.test.ts`, `store/selectors.test.ts` | 15 |
| **18** | Shell, rail, sub-rail and top bar | `AppShell`, the four-level rail with right-click verbs, the six-lens sub-rail, the top bar with the layout toggle and the split New-session button + picker, `StatusDot`, `heartbeat`, `Popover`, `ContextMenu`, `Toasts` | C `components/AppShell.tsx`, `components/rail/*` (7 files + css), `components/subrail/SubRail.tsx` + css, `components/lens/LensPlaceholder.tsx`, `components/topbar/*` (4 files + css), `components/common/{StatusDot,Toasts,Popover}.tsx` + css, `components/menu/ContextMenu.tsx` + css, `hooks/heartbeat.ts`, `hooks/useDismissable.ts`; T `rail/Rail.test.tsx`, `topbar/NewSessionButton.test.tsx`, `topbar/SessionPicker.test.tsx`, `common/StatusDot.test.tsx`, `hooks/heartbeat.test.ts` | 16, 17 |
| **19** | The terminal binding and the attach registry | `attachRegistry` (ref-counted sockets + xterm instances, the §4.6 retry ladder, 10 Hz batching, pinned scroll), `Terminal.tsx` rewritten to take a `sessionId`, `poller.ts` | C `desktop/src/api/attachRegistry.ts`, `desktop/src/api/poller.ts`; M `desktop/src/components/Terminal.tsx`, **`desktop/src/App.tsx`**, **`desktop/src/test/fakes.ts`**; T `api/attachRegistry.test.ts`, `api/poller.test.ts`, `components/Terminal.test.tsx` | 16, 17 |

**Note on task 18.** It is the widest task in Group D — four surfaces and six shared primitives —
and it is the only one that could reasonably be two. Keep it whole if it can be: the rail, the
sub-rail and the top bar share `Popover`, `ContextMenu` and `StatusDot`, and splitting them
produces an 18a whose only consumer is 18b. If it runs long, the natural cut is **18b = the top
bar and the session picker** (`components/topbar/*` and its two tests), leaving 14–17 and 19–24
untouched.

### D2 — surfaces, verbs and proof (tasks 20–24)

| # | Title | Scope in one line | Files | Depends on |
|---|---|---|---|---|
| **20** | Columns layout and the session pane | `DevelopLens`, `ColumnsLayout` (horizontal scroll, scroll-into-view, focus), `SessionPane` + header + footer, `EmptyDevelop` | C `components/develop/{DevelopLens,ColumnsLayout,SessionPane,SessionPaneHeader,SessionPaneFooter,EmptyDevelop}.tsx`, `components/develop/develop.css`; T `develop/ColumnsLayout.test.tsx`, `develop/SessionPane.test.tsx` | 18, 19 |
| **21** | Canvas layout and persisted tiles | The 2400×1600 plane, pointer-driven drag and resize on `transform`, z-order, the 10-tile pin cap, `TilePosition` written through to `prefsSlice` | C `components/develop/CanvasLayout.tsx`; M `components/develop/develop.css`, `components/develop/DevelopLens.tsx`; T `develop/CanvasLayout.test.tsx` | 20 |
| **22** | Session lifecycle verbs | `SessionMenu` and the rail's context verbs wired to the client: Resume, Fork into…, Restart, Rename…, Kill, Forget, Open a shell here, Copy session id, Mark ready; optimistic mutation with an Undo window, in **two** commit shapes — deferred (Kill, Forget: the request is held for the window and Undo cancels it) and immediate (Rename, Restart: sent at once, snapshot restored on refusal). A deferred verb's ids are held in `lib/pendingSessions` and `replaceSessions` suppresses the poller for them, or the 2 s tick undoes the patch inside the 6 s window | C `components/develop/SessionMenu.tsx`, `components/develop/sessionVerbs.ts`, `lib/pendingSessions.ts`; M `components/menu/ContextMenu.tsx`, `components/menu/menu.css`, `components/rail/*Row.tsx`, `store/slices/dataSlice.ts` *(declared departure — task 17's)*, `api/attachRegistry.ts` *(declared departure — task 19's; the `retryAttach` export of §6.6, which answers the terminal `failed` state)*; T `develop/sessionVerbs.test.ts`, `develop/SessionMenu.test.tsx`, and edits to `menu/ContextMenu.test.tsx` (whose verb-list assertions this task necessarily breaks) and `api/attachRegistry.test.ts` | 20 |
| **23** | The command palette and the keyboard map | `buildCommands`, `CommandPalette` with stable ranking and id-based selection, `useKeyboardMap` with the full chord table and the two deliberate non-interceptions | C `components/palette/{CommandPalette.tsx,commands.ts,palette.css}`, `hooks/useKeyboardMap.ts`; M `components/AppShell.tsx`; T `palette/CommandPalette.test.tsx`, `palette/commands.test.ts`, `hooks/useKeyboardMap.test.ts` | 20, 22 |
| **24** | Wire the cockpit to the live control plane, and prove the ten | `App.tsx` rewritten to mount `AppShell` + start the poller + load the catalogue; the defect regression suite as one file; the design-system audit (no literal hex, no named colour, no `transition: all`, no banned hue, and no transition off shared shape §3B's list in a stylesheet or in any inline style) | M `desktop/src/App.tsx`, `desktop/src/main.tsx` (imports `app.css`), `desktop/src/api/poller.ts` *(declared departure, task 19's file: §4.7's `onGoodTick` and ruling B)*, and, in fix round 1, `desktop/src/hooks/useCreateSession.ts` *(declared departure, task 18's file)* and `desktop/src/components/develop/sessionVerbs.ts` *(declared departure, task 22's file)*, both for §3.2's own-report rule; D `desktop/src/App.css`; C `desktop/src/defects.test.tsx`, `desktop/src/styles/designsystem.test.ts`; T the two above, which also pin `poller.ts`'s departure, plus fix round 1's edits to `develop/sessionVerbs.test.ts` and `topbar/NewSessionButton.test.tsx` | 21, 22, 23 |

Every task runs `npm test` from `desktop/` and `npm run typecheck`, and every task's commit step
ends with:

```
Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
```

No task in Group D runs `go test`, and no task in Group D edits a `.go` file. Group D's only
dependency on Groups B and C is the JSON shape in §4, which is why task 16's tests run entirely
against an injected `fetch` and need no control plane.

---

## 10. Requested changes to the Go contract

`contract-go.md` is authoritative and nothing above silently diverges from it. Seven items, in
descending order of how much they cost if ignored. The first is blocking.

### 10.1 BLOCKING — the control plane must answer CORS, or the cockpit cannot call it at all

`contract-go.md` §2.12 specifies an `OriginPolicy` that **rejects** a bad origin. It does not
specify that the server *grants* a good one, and those are different mechanisms. The webview is a
browser: a `fetch` from the cockpit's origin to `http://127.0.0.1:8730` is **cross-origin** in
both builds —

| Build | Webview origin | Control plane | Cross-origin? |
|---|---|---|---|
| dev (`build.devUrl`) | `http://localhost:1420` | `http://127.0.0.1:8730` | yes — different host *and* port |
| packaged, Windows | `http://tauri.localhost` | `http://127.0.0.1:8730` | yes |
| packaged, macOS/Linux | `tauri://localhost` | `http://127.0.0.1:8730` | yes — different scheme |

— so the browser enforces CORS. `GET /v1/machines` fails at the response with "No
'Access-Control-Allow-Origin' header is present"; `POST`/`PATCH`/`DELETE` with
`Content-Type: application/json` fail one step earlier, at a preflight `OPTIONS` the mux does not
route. **WebSockets are exempt** (no preflight, no ACAO requirement), which is exactly why
Phase 1 worked and why this will not be noticed until the first REST call in task 24.

Requested, as ~15 lines inside the existing `OriginPolicy.Middleware` (task 9 or 13):

```
On every request whose Origin is present and allowed:
  Access-Control-Allow-Origin: <the request's Origin, echoed verbatim — never "*">
  Vary: Origin
On OPTIONS, when Origin is present and allowed, answer 204 BEFORE the mux and add:
  Access-Control-Allow-Methods: GET, POST, PATCH, DELETE, OPTIONS
  Access-Control-Allow-Headers: Content-Type
  Access-Control-Max-Age: 600
Credentials are NOT allowed (no cookies are used), so no Access-Control-Allow-Credentials.
```

Echoing the origin rather than `*` keeps the allowlist as the single decision point, and `Vary:
Origin` stops any interposed cache serving one origin's response to another.

**The fallback, if this is refused**: the desktop drops `fetch` and uses `@tauri-apps/plugin-http`,
whose `fetch` is proxied through Rust and is not subject to CORS. That costs a new npm dependency,
a new Rust crate, a `tauri.conf.json` plugin entry, and this in
`desktop/src-tauri/capabilities/default.json`:

```json
{
  "identifier": "http:default",
  "allow": [{ "url": "http://127.0.0.1:8730/*" }]
}
```

It also makes §1's client untestable with a plain injected `fetch`, because the plugin's `fetch`
only exists inside a Tauri window. That is a real cost for no security gain — the Rust side would
have to allowlist the same origin anyway. **Recommend 10.1 as specified; design tasks 16 and 24
against it.**

### 10.2 REQUIRED for defect 9 — a worktree needs a settable `ready` flag

`docs/FUNCTIONAL-SPEC.md` known defect 9: "Nothing ever sets `ready`, so user-created worktrees
can never merge." The Go contract's worktree object (§3.7) has no `ready` field and there is no
worktree mutation endpoint, so Phase 2 cannot fix the defect — it can only delete the verb, which
moves the defect rather than fixing it.

Requested:

```
GET /v1/projects/{projectId}/worktrees   →  each worktree gains:  "ready": false
PATCH /v1/projects/{projectId}/worktrees/{worktreeId}
  body:    { "ready": true }
  200:     the worktree object
  400 bad_request       — body has any field other than "ready"
  404 worktree_not_found
```

Storage: one more small map in the existing registry document
(`"worktreeFlags": { "w-4a77b1c9": { "ready": true } }`), written through the same
`Store.Update` + atomic replace. It is a user-set fact about a checkout, it must survive a
restart, and it is the only new persisted state Phase 2 needs. Natural home: **task 7** (the
registry) and **task 10** (the read surface), both already in Group B/C scope.

**LANDED, so this section is history rather than a request.** Task 7 added the `worktreeFlags` map
and `WorktreeFlagStore`; task 10 shipped `worktreeJSON.Ready bool` with no `omitempty` at
`catalog.go:105`, the zero-value fill at `:319`, the route at `:33` and `handlePatchWorktree` at
`:384`. So `ready` is **always** on the wire as true or false, defect 9 is fixed in Phase 2 rather
than deferred to Phase 3, and **task 22 does not check which world it is in** — it renders the verb
unconditionally.

The paragraph this replaced described the refusal branch: `Worktree.ready` staying `undefined`, the
verb not rendered, defect 9 moving to Phase 3. None of that happened, and leaving it written would
have had task 22 either re-deriving the answer or writing the test it named for a shape the server
cannot produce.

### 10.3 Scope, not a defect — Phase 2 can create sessions and nothing else

There is no endpoint to create a project, a branch or a worktree. `prototype/app.html` offers all
three (`A.newProject`, `A.newBranch`, `A.newSession` creating a worktree implicitly), and
`docs/PROGRAM.md` puts worktree lifecycle in Phase 3 and project import in Phase 4, so this is
correct — but it means three prototype affordances have no backing. Group D renders them disabled
with a tooltip naming the phase, rather than deleting them (a deleted control is a silent scope
cut) or faking them client-side (a fake control is the worse of the two). No change requested;
recorded so a reviewer does not read it as an omission.

Consequence worth naming: `POST /v1/sessions` requires an existing `worktreeId`, so in Phase 2
**the `＋` on a worktree row is the only way to start a session**, and a branch with zero
worktrees can hold zero sessions. `EmptyDevelop` must say that in words rather than offering a
button that 404s.

### 10.4 Security note — the desktop→control-plane attach hop carries no secret

Per §4.5 the only checks on `/attach` are the origin allowlist and the loopback `Host` guard.
Neither stops a **non-browser** process: any local program can `curl --include -H "Connection:
Upgrade"` its way to `ws://127.0.0.1:8730/v1/sessions/{id}/attach` with no `Origin` header at all
(which `OriginPolicy.Allow` permits by design, correctly, since curl and the Go test client send
none) and get a live shell inside the sandbox.

This is the same exposure the Group A task 3 bearer token was added to close on the
sandbox→control-plane hop, one layer up. It is defensible for a single-user laptop — any process
running as that user can already read `~/.claude` — and it should be a conscious decision rather
than an accident.

Requested for **Phase 8, not Phase 2**: `GET /v1/sessions/{id}` gains a short-lived
`attachTicket` (60 s, single-use, minted per request), which the desktop appends as
`?ticket=`. No change requested now; recorded so it is on the record before the app is
distributed to a fleet.

### 10.5 A session event stream would remove the poll

Phase 2 polls `GET /v1/sessions` every 2 s (§4.7) because there is no push. That is fine at ten
sessions on a laptop and it is the right Phase 2 answer. It costs one HTTP round trip per 2 s per
window and it means a status change is visible in 0–2 s rather than immediately.

Requested for Phase 3: `GET /v1/events` as SSE, emitting `session.created`,
`session.status_changed` and `session.removed` with the full session object, with the poll kept as
the fallback when the stream drops. Not requested for Phase 2 — a stream added now would be a
second code path with one consumer.

### 10.6 Cosmetic — `machines[].default`

`default` is a JS reserved word. It is legal as a property name and `m.default` parses fine, so
this costs nothing; `isDefault` would simply read better and avoid a reflex double-take in review.
**Not worth a change** now that the field is specified. Recorded so nobody "fixes" it on one side
only.

### 10.7 Clarification — `harness_unavailable` is 409

`contract-go.md` §3.2 maps `harness_unavailable` to **409 Conflict**; §3.9's error list repeats it.
The cockpit treats it as a user-fixable condition ("Codex is not in the sandbox image") and shows
`harness.note`, so the status code does not change any client behaviour. Confirming rather than
requesting: the client switches on `error`, never on the status code, precisely so a re-map
between 400 and 409 is not a client change.

---

## 11. Reconciliation with the Phase 4 (Discover) plan

`docs/superpowers/plans/2026-09-18-phase-4-discover.md` is already written and touches four files
Group D also touches. Resolved here so neither plan clobbers the other.

| File | Phase 4 says | Resolution |
|---|---|---|
| `desktop/src/styles/tokens.css` | Task 10 step 4: "**Check first**: `ls desktop/src/styles/tokens.css`. If Phase 2 has already shipped it, **skip this step**" and keep the variable names | Phase 2 ships it (§5). All 28 names Phase 4 uses are present with the same meaning, and task 14's test asserts they exist. **No action needed in Phase 4 beyond the check it already does.** |
| `desktop/vitest.config.ts` | Task 10 step 3 creates it, with no `setupFiles` | **Conflict.** Phase 2's config has `setupFiles: ["./src/test/setup.ts"]`, without which jsdom has no `matchMedia`, no `ResizeObserver` and no `scrollIntoView`. Phase 4's task 10 must gain the same "check first, and if it exists confirm the settings below are present" wording it already uses for `tokens.css`. **Flagged for whoever finalises the Phase 4 plan**; Group D does not edit it. |
| `desktop/package.json` | Task 10 step 1 installs vitest/jsdom/RTL with the same versions | Identical version set, so whichever lands first satisfies the other. Phase 2 additionally adds `zustand@5.0.15` and the `typecheck` script. |
| `desktop/src/lib/discoverApi.ts`, `src/discover/*`, `src/provenance/*` | Phase 4 creates them | Group D creates none of these and must not pre-empt them. The one seam Phase 2 owes Phase 4 is `Session.contextRef`, which is carried through `toCreateRequest` and stored, never interpreted (§2, §3.6). The sub-rail's `discover` row routes to `LensPlaceholder` until Phase 4 replaces that one line. |

---

## 12. Open questions — things a task author must not silently resolve

1. **`@xterm/xterm@6` under jsdom was never observed**, only avoided. §1.5 mocks the module, so
   no test loads it, and the decision does not depend on the answer. If a future task genuinely
   needs a real xterm in a test, the answer is a Playwright run in Group E, not the `canvas`
   native package — which needs a C++ toolchain this Windows machine is not guaranteed to have.
2. **The CORS behaviour in §10.1 was reasoned from the browser's model, not observed against the
   Group C server**, because the server does not exist yet. Task 16's verification step must run
   one real `GET /v1/machines` from the dev webview and read the network panel before task 24
   builds on it. If the request succeeds without §10.1, my reasoning is wrong and the section
   should be struck — say so rather than quietly deleting it.
3. **`localStorage` availability in a packaged Tauri build with `csp: null`** is assumed from the
   webview being a normal secure-context browser. It is near-certain and it is untested here. If
   `persist` throws on write, zustand swallows it and the app silently stops remembering
   anything; task 17 must add the one-line `onRehydrateStorage` error log so that failure is
   visible rather than mysterious.
4. **Which `permissionMode` the picker should default to.** `contract-go.md` §8.1 records that
   Claude Code's `--permission-mode` has no `default` value and that the server emits no flag at
   all for `ask`. `APP_DEFAULTS.permissionMode = "ask"` therefore means "the CLI's own default",
   which is right today but will read as a lie the moment someone verifies what `manual` does.
   Whoever closes Go open question 1 should also decide whether the picker's label stays "ask".
5. **Whether the rail's branch dot should aggregate at all.** §6.2 gives it the highest-attention
   status among its sessions with a click-through to the session that produced it, which satisfies
   `docs/AGENTIC-UX.md` rule 7. But rule 3 of §9 in that document explicitly deletes "the dot
   clusters on every rail level" as a flood generator. I have kept **one** dot per row and no
   counts, which I believe is the narrow reading, but a reviewer who reads it as "no dots above a
   session row" should say so before task 18 — it is a five-line change then and a rail rewrite
   later.
6. **`docs/DESIGN-SYSTEM.md`'s surface list puts `field` last, after `hover-2`.** §5.3 makes it a
   well (lighter-to-darker break in an otherwise monotone ramp) because an input that is lighter
   than a hover state glows. If the doc's order was meant literally as a lightness order, one line
   changes. Flagged rather than silently reinterpreted.
7. **Typography.** `docs/DESIGN-SYSTEM.md` specifies Geist Sans plus one licensed mono (Berkeley
   Mono ~$100, or Commit Mono, OFL). Phase 2 ships neither webfont and uses local stacks (§5.3),
   because a font that fails to load on a corporate laptop is a layout shift on every launch.
   That is a visible difference from the approved direction and it is the owner's call, not a
   task author's: the licence needs buying and the file needs vendoring before it can ship.
