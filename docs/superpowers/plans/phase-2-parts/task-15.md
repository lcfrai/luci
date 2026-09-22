### Task 15: Domain model, ids and the immutable helpers

This is the file every later Group D task types against. `desktop/src/store/types.ts` is one compiling block with no imports, and contract §2 is explicit about why the names in it are binding: every server-derived field is **exactly** the JSON name the control plane emits, because a rename here fails at runtime against a real control plane and never at compile time. TypeScript will happily bless `s.lastOutput` as a `string` when the wire says `lastOutputAt` — the property is simply `undefined`, `Date.parse(undefined as unknown as string)` is `NaN`, and the pane footer renders a dash forever while the type checker reports nothing. That is the single defect class this task exists to prevent, so step 1 transcribes §2's block verbatim and step 2 proves it verbatim with a `diff` against the contract rather than by eye. Nobody's eye catches `isPrimary` written `primary` on line 71 of 215.

**§2.1 is the paragraph to read twice, and what matters about it is not the shape — it is what the shape stops a later task doing.** Many sessions per worktree, held by task 17 as `sessionIdsByWorktree: Record<WorktreeId, SessionId[]>` and never as a `sessions` array on `Worktree`. `prototype/app.html` carries the other cardinality, and three habits fall out of it that tasks 18, 20 and 22 will each reach for: the rail collapsing worktree and session into one row when a worktree holds exactly one session; `SessionPane`'s header showing the session title alone, which is ambiguous the moment two panes carry the same worktree; and `A.sessAct` case `kill` splicing the worktree out of the tree when its last session dies. The third is the expensive one, and it is the reason this is a type-level decision rather than a component-level one. A worktree is a checkout on disk. It outlives every session that ever ran in it, the control plane enumerates it from git whether or not a session exists, and deleting the row on kill means the user's branch appears to have vanished and their next `＋` has nowhere to land. A `Worktree` with no `sessions` field has nothing to splice, so the mistake is not available to make.

The other three files are small and each one kills a named defect from the contract's list. `lib/ids.ts` kills defect 2, `uid()` colliding on fast successive calls, and is the only place in the cockpit that mints an id at all — every id the user can see comes from the server's `crypto/rand`, so the client needs ids for toasts and React keys and nothing else. `lib/immutable.ts` kills defects 7 and 10, which contract §3.9 correctly calls the same mistake in different clothes: an index computed against one array and applied to another. `lib/time.ts` is the pure formatter that makes `useHeartbeat()`'s one clock legible; it takes `nowMs` as an argument precisely so that every elapsed number on screen in a single render tick is the same number.

The facts from Groups B and C that the UI contract predates are carried here because they are the ones that touch types. **Harness arrays are never `null`** — task 8's fix round made the Go side allocate unconditionally and pinned it with a marshalled-bytes test, so `models`, `efforts` and `permissionModes` are non-optional `T[]` in §2 and stay that way; no `?` and no defensive `?? []` anywhere downstream. **`SessionStatus` has exactly four members** — `starting`, `running`, `idle`, `exited` — and nothing outside that set is ever rendered, because the cockpit cannot derive a status the server cannot send. `docs/AGENTIC-UX.md` §4 rule 8 names a different vocabulary of seven (PLANNING, EXECUTING, BLOCKED-ON-YOU, BLOCKED-ON-MACHINE, PAUSED, DRIFTING, DONE), of which only EXECUTING and DONE have mechanical analogues here; the remaining **five** need the agent-runtime signal that document itself calls a hard runtime dependency, and deriving them means reading PTY bytes, which this phase forbids. Contract §0 says five and is right. §2's own inline comment says three and is not — step 1 says what to do about that, and the answer is not to fix it here. **A dropped WebSocket destroys nothing** — destroy is explicit kill, idle TTL or orphan reap — which is why `Session` has `exitReason` as free text rendered verbatim and why an exited session is still a listed, readable row. **The sixteen-member error-code enum is not declared here.** §3.2's codes, including Group C's sixteenth, `non_loopback_publish`, belong to task 16's `ApiError` in `api/client.ts`; §2 contains none of them and this task does not pre-empt it. Task 16's author should not wait on this file for them.

**Group D's shared shape §3 gives this task one deliverable contract §9 does not, and the reason is a dependency order rather than a preference.** The six typed fixture builders — `aMachine`, `aProject`, `aBranch`, `aWorktree`, `aSession`, `aHarness` — go into task 14's reserved insertion point in `desktop/src/test/fixtures.ts`, and they go in **here**. Task 14 cannot type them, because the types are this file's; task 16 runs before task 17 and imports them for its client tests, so a builder created in task 17 does not exist when task 16 needs it. Task 15 is the first task that can type them and the last that can ship them before they are needed. Two consequences follow and both are honoured below. `fixtures.ts` is the only file this task **modifies** rather than creates, so shared shape §1's three rules bind it and step 14 carries the anchored pre-commit guard rule 3 requires. And §3.2's inventory is closed: there is no seventh builder, and no later task declares a local one — a variant is an override object passed to one of these six.

One naming collision to settle before it reaches the file. Shared shape §3.2 writes the last signature `aHarness(over?: Partial<HarnessDescriptor>): HarnessDescriptor`, but `HarnessDescriptor` is not a type anything declares: contract §2 calls it `Harness`, and `types.ts` is transcribed from §2 verbatim under a diff gate, so it cannot be renamed here. The **builder name** is `aHarness`, exactly as §3.2 fixes it; the type it takes and returns is `Harness`, which is what §2 actually declares.

Every step runs from `desktop/`, except step 2's extraction and diff, which run from the repository root. This task runs `npm test` and `npm run typecheck` and nothing else — no Group D task runs `go test` or edits a `.go` file.

**Files:**
- `desktop/src/store/types.ts` — contract §2 in full: ids, machines, projects, branches, worktrees, harnesses, sessions, and the client-only types (new)
- `desktop/src/lib/ids.ts` — `makeId`, `__resetIdCounter`; contract §3.8 (new)
- `desktop/src/lib/ids.test.ts` — prefix shape, base-36 counter, monotonicity from a reset, 2000-call uniqueness (new)
- `desktop/src/lib/immutable.ts` — `replaceAt`, `replaceById`, `moveWithinVisible`, `idAtVisiblePosition`; contract §3.9 (new)
- `desktop/src/lib/immutable.test.ts` — defect 7's dropped tail, defect 10's filter-boundary swap, and every out-of-range path (new)
- `desktop/src/lib/time.ts` — `formatElapsed`, `formatWaiting`; contract §7.4 (new)
- `desktop/src/lib/time.test.ts` — the contract's worked examples, both unit boundaries, clock skew, unparseable input, and purity (new)
- `desktop/src/test/fixtures.ts` — task 14's file. The six typed builders of shared shape §3.2 fill its reserved insertion point, and the `import type` clause they need goes under its header (modified)

**Interfaces:**
- Produces:
  - Every type in contract §2, from `desktop/src/store/types.ts`: `MachineId`/`ProjectId`/`BranchId`/`WorktreeId`/`SessionId`; `MachineKind`, `MachineState`, `Isolation`, `Machine`; `Project`, `Branch`, `Worktree`; `KnownHarnessId`, `HarnessId`, `ModelId`, `Effort`, `PermissionMode`, `HarnessModel`, `Harness`; `SessionStatus`, `Session`; and the client-only `Layout`, `Lens`, `TilePosition`, `SessionDefaults`, `SessionDefaultsOverride`, `Selection`, `ContextMenuTarget`, `Toast`
  - `makeId(prefix: string): string` and `__resetIdCounter(): void` from `lib/ids.ts`
  - `replaceAt`, `replaceById`, `moveWithinVisible`, `idAtVisiblePosition` from `lib/immutable.ts`, at the exact signatures contract §3.9 fixes
  - `formatElapsed(fromIso: string, nowMs: number): string` and `formatWaiting(sinceIso: string, nowMs: number): string` from `lib/time.ts`
  - The six typed fixture builders of shared shape §3.2, added to `desktop/src/test/fixtures.ts`: `aMachine(over?: Partial<Machine>): Machine`, `aProject(over?: Partial<Project>): Project`, `aBranch(over?: Partial<Branch>): Branch`, `aWorktree(over?: Partial<Worktree>): Worktree`, `aSession(over?: Partial<Session>): Session`, `aHarness(over?: Partial<Harness>): Harness`
- Consumes: task 14's toolchain — `vitest@5.0.1` with `globals: false`, the jsdom environment and `src/test/setup.ts` — and, for the builders only, task 14's `FIXED_NOW` and `IDS` from `src/test/fixtures.ts` plus the insertion point that file reserves. This task's own three test files still use **no** domain builder: their `Row` is two fields, `id` and `label`, which is a test-local shape and not a domain fixture, so shared shape §3's "add it to `fixtures.ts`" rule does not reach it.
- Required by: task 16 (types every response against §2, declares the error codes this file does not, and imports `aSession`/`aHarness` for its client tests), task 17 (three flat slices are `Record<Id, T>` over these types; `replaceById` in the reducers; the builders seed every store test), task 18 (`StatusDot` switches on `SessionStatus`; `Selection` and `ContextMenuTarget` drive the rail; `makeId` mints every `Toast.id`; `aProject`/`aBranch`/`aWorktree`/`aSession` build the rail's rows), task 19 (`Session.attachPath`, `SessionStatus`), task 20 (`formatElapsed` in the pane footer), task 21 (`TilePosition`), task 22 (`replaceById` plus `makeId` for the Undo toast), task 23 (`idAtVisiblePosition` for every numbered keyboard and palette jump).
- Amends nothing in the contract's signatures. It adds the behaviours §3.9 and §7.4 leave undefined — (a) out-of-range `replaceAt`, (b) unmatched `replaceById`, (c) clamped `moveWithinVisible`, (d) the non-finite-delta no-op, (e) the always-a-fresh-array rule, (f) unpadded leading unit and zero-padded trailing unit, (g) the days-and-hours arm of `formatElapsed`, and (h) the unparseable-timestamp sentinel — lettered here exactly as steps 7 and 11 letter them, each stated as a decision at the step that implements it.

- [ ] **Step 1: Write the domain model**

This is a transcription, not a design. Contract §2's fenced block is the deliverable, byte for byte, comments included — the comments are load-bearing (`default: boolean` explains why a reserved word is legal as a property; `sessionCount` explains why it must never be rendered as a number), and step 2 diffs this file against the contract, so a reflowed comment fails the step.

Two properties of the file that are easy to lose and that the diff exists to hold. It has **no imports and no runtime exports** — every declaration is a `type` or an `interface`, which is legal under `isolatedModules: true` without an `export {}` because every one of them is exported. And the ids are **plain aliases, not branded strings**; §2 gives the reason, which is that branding buys a `resp.id as SessionId` cast at every JSON boundary in exchange for catching a defect the store's shape already makes hard, since a `WorktreeId` and a `SessionId` never appear in the same signature.

**One comment in the block below is wrong, and you transcribe it anyway.** The `SessionStatus` comment reads "the other three need the agent-runtime signal". It is five, not three: `docs/AGENTIC-UX.md` §4 rule 8's seven are PLANNING, EXECUTING, BLOCKED-ON-YOU, BLOCKED-ON-MACHINE, PAUSED, DRIFTING and DONE, only EXECUTING and DONE have mechanical analogues in this phase's four, and contract §0 says five. The *type* is right either way — four members, and that is the part any later task depends on — so the error is confined to a count inside a comment. Do not correct it in this file: step 2 diffs `types.ts` against the contract, so a corrected comment fails the step and the implementer spends twenty minutes deciding whether the plan or the contract is lying. The correction belongs in the contract, as a one-line amendment raised separately, and this plan does not make it because a plan that edits the contract it is being verified against has removed its own check.

Create `desktop/src/store/types.ts`:

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
  /** REQUESTED, not yet in the Go contract — see §10.2. Undefined until it lands; the
   *  "Mark ready" verb is not rendered while it is undefined. */
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

/** Exactly four. docs/AGENTIC-UX.md §4 rule 8 specifies seven; the other three need the
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

One note for task 17, which persists state: `Toast.undo` is a function, so `Toast[]` is **not** persistable. The contract already puts toasts in `uiSlice` and persists only `prefsSlice`, so nothing is wrong here; it is written down because a later task adding toasts to the persisted partition would produce a rehydrated toast whose Undo button is `undefined` and whose click throws. The type is correct as specified and does not gain a serialisable variant.

- [ ] **Step 2: Prove the transcription is verbatim, and that it compiles**

`types.ts` has no test file, and deliberately so: it declares no runtime value, so there is nothing for vitest to call. Its two proofs are a diff against the source of truth and the type checker. The diff is the one that catches the defect that matters, because `tsc` is perfectly happy with a misspelt field name — it is only wrong relative to a JSON document `tsc` has never seen.

From the **repository root**, extract §2's fenced block and compare it. The extraction is anchored on the heading rather than on line numbers, so it survives the contract being edited above it:

This block is fenced with **four** backticks on purpose — the awk program contains three-backtick literals, and a three-backtick fence would close on them:

````bash
awk '/^## 2\. The domain model/{f=1} f&&/^```ts$/{c=1;next} c&&/^```$/{exit} c' \
  docs/superpowers/contracts/2026-09-18-phase-2-ui-contract.md > /tmp/types.contract.ts
wc -l /tmp/types.contract.ts
diff -u --strip-trailing-cr /tmp/types.contract.ts desktop/src/store/types.ts && echo IDENTICAL
````

I ran this against the contract while writing this plan: the extraction is **215 lines**, first line `// desktop/src/store/types.ts`, last line `}`. So `wc -l` printing anything other than 215 means the extraction misfired, not that your file is wrong — check the fence markers before you edit anything. A clean diff prints only:

```
IDENTICAL
```

`diff -u` prints the unified hunks and returns 1 on any difference, which short-circuits the `&&`, so **absence of `IDENTICAL` is the failure signal** even if you skim past the hunk.

**`--strip-trailing-cr` is load-bearing on this checkout and is not defensive padding.** The line endings here are mixed, and I measured it rather than assuming: `tr -cd '\r' | wc -c` counts **0** CR bytes in the contract markdown (2197 lines) and in every plan file, but **116 of 116** lines in `desktop/src/App.css`, **9 of 9** in `desktop/src/main.tsx`, **25 of 25** in `desktop/tsconfig.json` and **1 of 1** in `desktop/src/vite-env.d.ts` are CRLF. There is no `.gitattributes`, and `core.autocrlf = true` is set in the system config (`C:\Program Files\Git\etc\gitconfig`), so a fresh checkout converts text files on the way out. The awk extraction therefore comes out LF, while an editor following the convention of the directory it is writing into produces a CRLF `types.ts` — and a plain `diff -u` then reports a whole-file `@@ -1,215 +1,215 @@` hunk on a byte-identical transcription. I measured both halves with GNU diffutils 3.12: plain `diff -u` on an LF/CRLF pair prints the full-file hunk and exits 1; with `--strip-trailing-cr` the same pair prints `IDENTICAL`. The flag masks exactly one difference — the trailing CR — which is the one git normalises on commit anyway, and nothing else.

Then, from `desktop/`:

```bash
npm run typecheck
```

`tsc --noEmit` prints nothing on success, so the whole of a passing run is npm's own banner:

```

> desktop@0.1.0 typecheck
> tsc --noEmit

```

**The point of this step is what `tsc` does *not* catch, so here is what transcription mutations actually produce.** I mutated `types.ts` and ran each through this repository's own `node_modules/typescript/lib/tsc.js` — version **6.0.3** — against a copy of `desktop/tsconfig.json`, in a tree holding `types.ts` and nothing else, which is the state this step is run in. These are the runs, not predictions:

| Mutation | tsc |
|---|---|
| `isPrimary` transcribed as `primary` | **exit 0, no output** |
| `export` dropped from `MachineId`, which the file itself references | **exit 0, no output** |
| all six `/* ── */` section headers deleted | **exit 0, no output** |
| `export` dropped from `Layout` | `src/store/types.ts(163,6): error TS6196: 'Layout' is declared but never used.` |
| `export` dropped from `interface Machine` | `src/store/types.ts(32,11): error TS6196: 'Machine' is declared but never used.` |

The first three are silent, and the first of those is the exact defect class this whole task exists to prevent. **That is why the diff is the gate and the type checker is only the second opinion.** An earlier draft of this step predicted `TS1205` for a dropped `export` and `TS2300` for a dropped section header. Both were invented and both are wrong: `TS1205` is about re-exporting a type and this file re-exports nothing, and deleting a comment changes no declaration, so there is nothing to duplicate. The one `export`-related diagnostic you can really see is `TS6196`, and only under the `noUnusedLocals` rule — so it fires for a declaration **nothing else inside `types.ts` mentions** (`Machine`, `Project`, `Branch`, `Worktree`, `Harness`, `Session`, `Layout`, `Lens`, `TilePosition`, `SessionDefaultsOverride`, `Selection`, `ContextMenuTarget` and `Toast` — thirteen of the file's twenty-nine exported declarations) and stays silent for every id alias and every union the interfaces below it consume.

The one genuinely loud failure is a truncated paste — a fenced block copied short. Deleting everything after line 200 and running `tsc --pretty` gives this, measured and with ANSI stripped — the two lines after the first blank are the empty source line 201 and its squiggle gutter, and both carry trailing spaces your editor may strip on save, so match on the code and the coordinate rather than on the whitespace:

```
src/store/types.ts:201:1 - error TS1005: '}' expected.

201 
    


Found 1 error in src/store/types.ts:201

```

- [ ] **Step 3: Write the failing tests for client-side ids**

Contract §3.8 fixes the implementation of `makeId` completely, so these tests are not discovering a design — they are pinning the two properties the implementation exists for. **Known defect 2 is `uid()` colliding on fast successive calls**, and the fix is a counter *and* a random suffix rather than either alone: the counter alone collides across two windows sharing `localStorage`, and `randomUUID` alone is fine but puts 36 characters into every React key. Both halves get a test, because an implementer who drops the suffix passes the monotonicity test and an implementer who drops the counter passes the uniqueness one.

`__resetIdCounter` exists because module state is not test state. Contract §1.3 sets `restoreMocks` and `clearMocks`, and neither resets a module-level `let` — a fact the contract calls out explicitly at §1.4. So every test that asserts on a counter value calls the reset first, and a test that forgets is order-dependent rather than wrong-looking.

Create `desktop/src/lib/ids.test.ts`:

```ts
import { describe, expect, it } from "vitest";

import { __resetIdCounter, makeId } from "./ids";

describe("makeId", () => {
  it("joins the caller's prefix, a base-36 counter and eight hex characters", () => {
    __resetIdCounter();
    expect(makeId("toast")).toMatch(/^toast-1-[0-9a-f]{8}$/);
  });

  it("counts in base 36, so the counter stays one character until 36", () => {
    __resetIdCounter();
    let last = "";
    for (let i = 0; i < 35; i += 1) last = makeId("t");
    expect(last.split("-")[1]).toBe("z");
    expect(makeId("t").split("-")[1]).toBe("10");
  });

  it("is monotonic from a known point, which is the whole of what __resetIdCounter buys", () => {
    __resetIdCounter();
    const counters = [makeId("t"), makeId("t"), makeId("t")].map((id) => id.split("-")[1]);
    expect(counters).toEqual(["1", "2", "3"]);
  });

  it("does not collide on fast successive calls — known defect 2", () => {
    __resetIdCounter();
    const ids = new Set<string>();
    for (let i = 0; i < 2000; i += 1) ids.add(makeId("row"));
    expect(ids.size).toBe(2000);
  });

  it("stays unique across a counter reset, because the suffix is random and the counter is not", () => {
    __resetIdCounter();
    const first = makeId("t");
    __resetIdCounter();
    const second = makeId("t");
    expect(second.split("-")[1]).toBe("1");
    expect(second).not.toBe(first);
  });

  it("leaves a prefix containing a hyphen intact, because nothing ever parses the prefix back out", () => {
    __resetIdCounter();
    expect(makeId("ctx-menu").startsWith("ctx-menu-1-")).toBe(true);
  });
});
```

The fifth test is the only probabilistic one in this task: two eight-hex-character suffixes collide with probability 2^-32, about one run in four billion. That is worth saying out loud rather than leaving for someone to discover during a flake hunt — if it ever fails, it is not this test, it is a `makeId` that stopped calling `crypto.randomUUID`.

- [ ] **Step 4: Run the ids tests to verify they fail**

From `desktop/`, run both checks. The type checker first, because its diagnostic is the precise one:

```bash
npm run typecheck
npm test -- src/lib/ids.test.ts
```

`tsc` resolves `./ids` against a file that does not exist and says so once, at the import rather than at each use site — unlike Go, which reports every use of an undefined symbol:

```

> desktop@0.1.0 typecheck
> tsc --noEmit

src/lib/ids.test.ts:3:42 - error TS2307: Cannot find module './ids' or its corresponding type declarations.

3 import { __resetIdCounter, makeId } from "./ids";
                                           ~~~~~~~


Found 1 error in src/lib/ids.test.ts:3

```

The code `TS2307` and the module specifier are the assertion; the line and column move the moment a comment reflows, and npm appends its own `npm error Lifecycle script ... failed` block on the non-zero exit whose exact wording is npm's, not this repository's.

**A caveat on the vitest transcript below, stated rather than papered over.** The toolchain is installed by task 14 and was not present when this plan was written, so I could not execute `npm test` to capture its output. What is certain is the *shape*: vitest cannot collect a file whose import does not resolve, so this is a **collection error**, not a failing assertion — zero tests run, and the summary reports one failed test file with no test count. The resolution message itself comes from Vite's import analysis and is one of two wordings depending on where resolution gives up:

```
 FAIL  src/lib/ids.test.ts [ src/lib/ids.test.ts ]
Error: Failed to resolve import "./ids" from "src/lib/ids.test.ts". Does the file exist?
```

The other is `Failed to load url /src/lib/ids.ts (resolved id: ...). Does the file exist?`. Either is the correct red. What would be **wrong** is a run reporting `6 passed` — that means `ids.ts` already exists — or `Test Files  no tests` with no error, which means the file was created outside `src/` and `include: ["src/**/*.test.{ts,tsx}"]` never matched it.

- [ ] **Step 5: Write the client-side ids**

Contract §3.8's implementation, unchanged, with a file header saying why the module exists at all. Nothing is decided here.

`crypto.randomUUID` is available in the Tauri webview (WebView2 and WKWebView are both secure contexts) and in jsdom 30 under Node 24. It is absent on an insecure `http://` origin in some browsers, which does not apply: the only origins this app ever runs on are `tauri.localhost` and `localhost`, and browsers treat both as secure contexts. If a test ever throws `crypto.randomUUID is not a function`, that is a task 14 environment regression and belongs in `src/test/setup.ts`, not in a fallback here — a fallback would silently weaken every id in production to cover a test-only gap.

Create `desktop/src/lib/ids.ts`:

```ts
// desktop/src/lib/ids.ts
//
// Ids the CLIENT owns. Every id the user can see comes from the server
// (control-plane/internal/ids, crypto/rand), so this file exists for the two
// things the server never names: toasts, and React keys for transient rows.
// Contract §3.8.

let counter = 0;

/** Monotonic within a window, unique across windows. The counter alone collides across two
 *  windows sharing localStorage; randomUUID alone is fine but 36 chars in every React key. */
export function makeId(prefix: string): string {
  counter += 1;
  return `${prefix}-${counter.toString(36)}-${crypto.randomUUID().slice(0, 8)}`;
}

/** Test seam: resets the counter so a test can assert monotonicity from a known point.
 *  vitest's restoreMocks and clearMocks do not reset module state, so this is the only
 *  way a test gets a known starting point. */
export function __resetIdCounter(): void {
  counter = 0;
}
```

- [ ] **Step 6: Run the ids tests to verify they pass**

```bash
npm test -- src/lib/ids.test.ts
npm run typecheck
```

Six passing tests in one file. Vitest's default reporter prints a `✓` line per file and a summary block; the per-test `✓` lines appear only under `--reporter=verbose`, so a green run of one file is short:

```
 ✓ src/lib/ids.test.ts (6 tests) 9ms

 Test Files  1 passed (1)
      Tests  6 passed (6)
```

The counts are the assertion. Durations, the `Start at` and `Duration` lines and the transform/collect/setup breakdown vary per machine and per run, and the same caveat as step 4 applies to the exact reporter layout: I could not run vitest 5 while writing this. `6 passed (6)` with no `FAIL` line is what you are looking for.

The near-miss worth naming is incrementing the counter **after** the template literal rather than before, so the first id is `-0-` and every later one is off by one. That mutation is not a one-test failure. I executed all six test bodies against both implementations under Node, and the post-increment variant reports **`1 passed | 5 failed`** — only the 2000-id uniqueness test survives, because the random suffix carries it:

```
1. joins the caller's prefix, a base-36 counter and eight hex characters   FAIL  (id is "toast-0-…", the regex wants "toast-1-")
2. counts in base 36, so the counter stays one character until 36          FAIL  (35 calls end at "y", not "z")
3. is monotonic from a known point                                         FAIL  (["0","1","2"], not ["1","2","3"])
4. does not collide on fast successive calls                               pass
5. stays unique across a counter reset                                     FAIL  (counter reads "0", not "1")
6. leaves a prefix containing a hyphen intact                              FAIL  ("ctx-menu-0-…")
```

So `5 failed` is the signature to read as "off-by-one counter", and a run that fails the base-36 test **alone** is a different bug — a radix, not an increment.

- [ ] **Step 7: Write the failing tests for the immutable helpers**

Contract §3.9 gives four signatures and the two defects behind them, and it is worth restating defect 10 concretely because the test that catches it looks arbitrary otherwise. The prototype's `reorder` finds a worktree's index in the **filtered ready list** and applies that index to the **unfiltered worktrees array**. With rows `a b c d` where only `a c d` are ready, moving `a` down one place finds index 0 in the filtered list, adds one, and swaps positions 0 and 1 of the full array — so the user asked to move `a` past `c` and instead moved it past `b`, a row they cannot even see. The fix is not a bounds check. It is a signature that names the two lists separately and treats the hidden rows as **fixed anchors**: the visible rows permute among the absolute slots they already occupy, and everything else stays exactly where it is. `["a","b","c","d"]` with `b` hidden becomes `["c","b","a","d"]`, not `["b","a","c","d"]`, and the test asserts both — the right answer and, by name, the wrong one.

The behaviours the contract leaves undefined are decided here and pinned by tests. **(a) An out-of-range index in `replaceAt` is a no-op copy**, not a throw and not an append. These run inside reducers driven by render events, where a throw is a blank window; an append is worse, because it is defect 7 wearing a different hat — a list that silently grows. **(b) An unmatched `replaceById` is a no-op copy and never appends**, because upsert is a different operation with a different name, and a hidden append here would let a stale poll response resurrect a session the user just forgot. **(c) `moveWithinVisible` clamps at both ends rather than wrapping**: the arrow keys repeat, and wrapping would teleport the top row to the bottom on one extra keypress. **(d) A non-finite or non-integer delta is a no-op**, because `Array.prototype.splice` coerces `NaN` to 0 and would silently move the row to the front. **(e) Every helper that returns an array returns a fresh one, even when nothing changed.** Returning the input would require casting away the `readonly` on the parameter and would hand the caller a mutable alias to state it does not own; the cost is one array identity change on a keypress that did nothing, on lists of tens of rows.

Create `desktop/src/lib/immutable.test.ts`:

```ts
import { describe, expect, it } from "vitest";

import type { SessionId } from "../store/types";
import {
  idAtVisiblePosition,
  moveWithinVisible,
  replaceAt,
  replaceById,
} from "./immutable";

interface Row {
  id: SessionId;
  label: string;
}

const row = (id: string): Row => ({ id, label: id.toUpperCase() });
const ids = (rows: readonly Row[]): string[] => rows.map((r) => r.id);

describe("replaceAt", () => {
  it("replaces one element and keeps every later one — known defect 7", () => {
    const items = [row("a"), row("b"), row("c"), row("d")];
    const out = replaceAt(items, 1, { id: "b", label: "EDITED" });
    expect(ids(out)).toEqual(["a", "b", "c", "d"]);
    expect(out[1].label).toBe("EDITED");
  });

  it("returns a new array and leaves the input untouched", () => {
    const items = [row("a"), row("b")];
    const out = replaceAt(items, 0, { id: "a", label: "EDITED" });
    expect(out).not.toBe(items);
    expect(items[0].label).toBe("A");
  });

  it("makes an out-of-range index a no-op copy, never a throw and never an append", () => {
    const items = [row("a"), row("b")];
    for (const index of [-1, 2, 99, 1.5, NaN]) {
      const out = replaceAt(items, index, { id: "z", label: "Z" });
      expect(ids(out)).toEqual(["a", "b"]);
      expect(out).not.toBe(items);
    }
  });
});

describe("replaceById", () => {
  it("replaces by identity rather than by position", () => {
    const items = [row("a"), row("b"), row("c")];
    const out = replaceById(items, { id: "c", label: "EDITED" });
    expect(ids(out)).toEqual(["a", "b", "c"]);
    expect(out[2].label).toBe("EDITED");
  });

  it("never appends an unmatched element, because upsert is a different operation", () => {
    const items = [row("a"), row("b")];
    const out = replaceById(items, { id: "ghost", label: "GHOST" });
    expect(ids(out)).toEqual(["a", "b"]);
    expect(out).not.toBe(items);
  });
});

describe("moveWithinVisible", () => {
  // "b" is filtered out. What happens to it is the whole of known defect 10.
  const all = [row("a"), row("b"), row("c"), row("d")];
  const visible = ["a", "c", "d"];

  it("moves within the filtered subsequence and anchors the hidden row — known defect 10", () => {
    const out = moveWithinVisible(all, visible, "a", 1);
    expect(ids(out)).toEqual(["c", "b", "a", "d"]);
    // The prototype's bug produced this instead: an index from the filtered list
    // applied to the unfiltered array, swapping "a" with a row the user cannot see.
    expect(ids(out)).not.toEqual(["b", "a", "c", "d"]);
  });

  it("moves upwards through a hidden row without disturbing it", () => {
    const out = moveWithinVisible(all, visible, "d", -2);
    expect(ids(out)).toEqual(["d", "b", "a", "c"]);
  });

  it("keeps two hidden rows anchored at their own absolute positions", () => {
    const wide = [row("h1"), row("a"), row("h2"), row("b"), row("c")];
    const out = moveWithinVisible(wide, ["a", "b", "c"], "c", -2);
    expect(ids(out)).toEqual(["h1", "c", "h2", "a", "b"]);
  });

  it("reorders correctly when nothing is filtered out", () => {
    const out = moveWithinVisible(all, ["a", "b", "c", "d"], "c", -2);
    expect(ids(out)).toEqual(["c", "a", "b", "d"]);
  });

  it("clamps at the top rather than wrapping, because arrow keys repeat", () => {
    expect(ids(moveWithinVisible(all, visible, "a", -1))).toEqual(["a", "b", "c", "d"]);
    expect(ids(moveWithinVisible(all, visible, "a", -99))).toEqual(["a", "b", "c", "d"]);
  });

  it("clamps at the bottom rather than wrapping", () => {
    expect(ids(moveWithinVisible(all, visible, "d", 5))).toEqual(["a", "b", "c", "d"]);
  });

  it("is a no-op copy for a zero, fractional or non-finite delta", () => {
    for (const delta of [0, 0.4, NaN, Infinity]) {
      const out = moveWithinVisible(all, visible, "a", delta);
      expect(ids(out)).toEqual(["a", "b", "c", "d"]);
      expect(out).not.toBe(all);
    }
  });

  it("is a no-op when the id is filtered out, so a keypress on a hidden row does nothing", () => {
    expect(ids(moveWithinVisible(all, visible, "b", 1))).toEqual(["a", "b", "c", "d"]);
  });

  it("is a no-op when a visible id names a row that is not in the list at all", () => {
    const out = moveWithinVisible(all, [...visible, "ghost"], "ghost", -1);
    expect(ids(out)).toEqual(["a", "b", "c", "d"]);
  });
});

describe("idAtVisiblePosition", () => {
  const visible = ["c", "a", "d"];

  it("maps a 1-based position back to an id across a filter boundary", () => {
    expect(idAtVisiblePosition(visible, 1)).toBe("c");
    expect(idAtVisiblePosition(visible, 3)).toBe("d");
  });

  it("returns null outside the range rather than undefined", () => {
    for (const n of [0, -1, 4, 99]) expect(idAtVisiblePosition(visible, n)).toBeNull();
  });

  it("returns null for an empty list, so a keypress on nothing selects nothing", () => {
    expect(idAtVisiblePosition([], 1)).toBeNull();
  });

  it("returns null for a fractional or non-finite position", () => {
    for (const n of [1.5, NaN, Infinity]) expect(idAtVisiblePosition(visible, n)).toBeNull();
  });
});
```

Eighteen tests. Note the `import type { SessionId }` at the top: it is the only import of `store/types.ts` in this task's three test files — step 14 adds the other one, to `fixtures.ts` — and it is there on purpose, so that a `types.ts` which fails to resolve breaks a test run and not only `tsc`. `Row` is defined locally rather than pulled from `src/test/fixtures.ts` — a domain fixture here would invert the dependency, since `fixtures.ts` is itself typed against `types.ts`.

- [ ] **Step 8: Run the immutable tests to verify they fail**

```bash
npm run typecheck
npm test -- src/lib/immutable.test.ts
```

`tsc` prints one `TS2307` for `./immutable`. It is **not** laid out like step 4's, and the difference is worth having in front of you, because the import here is a four-name clause spread over six lines: the diagnostic anchors on the **closing brace line**, at `9:8`, and the source line it echoes is `} from "./immutable";`. I deleted `immutable.ts` and ran `tsc --pretty` to capture it:

```
src/lib/immutable.test.ts:9:8 - error TS2307: Cannot find module './immutable' or its corresponding type declarations.

9 } from "./immutable";
         ~~~~~~~~~~~~~


Found 1 error in src/lib/immutable.test.ts:9
```

Nine spaces of squiggle indent and thirteen tildes under `"./immutable"`, and two blank lines before the `Found 1 error` summary. As in step 4, the code and the module specifier are the assertion; `9:8` moves the moment a line is added above the import. It does **not** print a second error for `../store/types`, which already exists — if it does, step 1 put the file in the wrong directory.

vitest fails to collect, zero tests run, for the same reason and with the same two possible wordings as step 4. The thing to check is that the failure names `./immutable` and nothing else: an error mentioning `../store/types` means the relative depth is wrong, `src/lib/x.ts` reaching `src/store/types.ts` being exactly two segments.

- [ ] **Step 9: Write the immutable helpers**

The four functions, with the header comment carrying the rule they exist to encode. `moveWithinVisible` is the only one with any structure to it, and it reads in three movements: find the absolute slots the visible rows occupy, compute the new order of the visible ids, then write the reordered rows back into those same slots. Hidden rows are never touched because their indices are never in `slots`.

Create `desktop/src/lib/immutable.ts`:

```ts
// desktop/src/lib/immutable.ts
//
// Contract §3.9. Known defects 7 and 10 are the same mistake in different
// clothes: an index computed against one array and applied to another. These
// helpers make "never doing that" the path of least resistance.
//
// The rule they encode, and it belongs in every task that touches a list:
// NEVER pass an array index across a filter boundary; pass the id.
//
// Every helper that returns an array returns a fresh one, even when nothing
// changed. Returning the input would mean casting away the readonly on the
// parameter and handing the caller a mutable alias to state it does not own.

/** Replaces exactly one element. Known defect 7: the prototype's editAnswer splices with
 *  `splice(i, 1)` and then pushes, which drops every later element. An index outside the
 *  array is a no-op copy — these run inside reducers driven by render events, where a
 *  throw is a blank window and an append is defect 7 in a different hat. */
export function replaceAt<T>(items: readonly T[], index: number, next: T): T[] {
  const out = items.slice();
  if (!Number.isInteger(index) || index < 0 || index >= out.length) return out;
  out[index] = next;
  return out;
}

/** Replaces the element whose key matches, by identity rather than position. Preferred over
 *  replaceAt wherever the element has an id. An unmatched id is a no-op copy and never an
 *  append: upsert is a different operation, and a hidden append here would let a stale poll
 *  response resurrect a session the user just forgot. */
export function replaceById<T extends { id: string }>(items: readonly T[], next: T): T[] {
  const index = items.findIndex((item) => item.id === next.id);
  if (index === -1) return items.slice();
  return replaceAt(items, index, next);
}

/** Moves `id` by `delta` places WITHIN the subsequence `visibleIds`, and returns the full
 *  list reordered to match. Known defect 10: the prototype's `reorder` finds the index in the
 *  FILTERED ready list and applies it to the UNFILTERED worktrees array, so reordering two
 *  ready worktrees swaps two unrelated ones whenever an unready worktree sits between them.
 *  The signature makes the filtered/unfiltered distinction explicit and impossible to
 *  conflate.
 *
 *  Rows that are not visible are fixed anchors: the visible rows permute among the absolute
 *  slots they already occupy. Moves past either end clamp rather than wrap, because the
 *  arrow keys repeat. */
export function moveWithinVisible<T extends { id: string }>(
  all: readonly T[],
  visibleIds: readonly string[],
  id: string,
  delta: number,
): T[] {
  const out = all.slice();

  // The absolute positions the visible rows occupy, ascending, and the rows themselves.
  const visible = new Set(visibleIds);
  const slots: number[] = [];
  const byId = new Map<string, T>();
  for (let i = 0; i < all.length; i += 1) {
    const item = all[i];
    if (visible.has(item.id)) {
      slots.push(i);
      byId.set(item.id, item);
    }
  }

  // A non-integer delta must not reach splice, which coerces NaN to 0 and would
  // silently move the row to the front.
  const step = Math.trunc(delta);
  if (!Number.isFinite(step) || step === 0) return out;

  const from = visibleIds.indexOf(id);
  if (from === -1 || !byId.has(id)) return out;

  const to = Math.min(Math.max(from + step, 0), visibleIds.length - 1);
  if (to === from) return out;

  const order = visibleIds.slice();
  order.splice(from, 1);
  order.splice(to, 0, id);

  let slot = 0;
  for (const orderedId of order) {
    if (slot >= slots.length) break;
    const item = byId.get(orderedId);
    if (item === undefined) continue;
    out[slots[slot]] = item;
    slot += 1;
  }
  return out;
}

/** Maps a 1-based position in a FILTERED list back to an entity id, or null. Every keyboard
 *  and palette path uses this instead of indexing an array. */
export function idAtVisiblePosition(
  visibleIds: readonly string[],
  oneBased: number,
): string | null {
  if (!Number.isInteger(oneBased) || oneBased < 1 || oneBased > visibleIds.length) return null;
  return visibleIds[oneBased - 1];
}
```

`Math.trunc(NaN)` is `NaN` and `Math.trunc(Infinity)` is `Infinity`, which is why the guard tests `Number.isFinite(step)` rather than `step !== 0` alone — `NaN === 0` is false, so a bare zero check lets both through. The `slot >= slots.length` break costs nothing and covers a `visibleIds` containing a duplicate, where `order` would otherwise be longer than `slots` and the last write would land on `out[undefined]`.

- [ ] **Step 10: Run the immutable tests to verify they pass**

```bash
npm test -- src/lib/immutable.test.ts
npm run typecheck
```

Eighteen passing tests in one file, and `tsc` silent:

```
 ✓ src/lib/immutable.test.ts (18 tests) 12ms

 Test Files  1 passed (1)
      Tests  18 passed (18)
```

`18 passed (18)` with no `FAIL` is the assertion; durations vary and the reporter-layout caveat from step 4 still applies. Particular failures map to particular drifts. The defect-10 test failing with `["b","a","c","d"]` means the implementation reordered `all` directly instead of permuting within `slots`. The "two hidden rows" test failing while the first one passes means `slots` was built from `visibleIds` order rather than from a walk of `all`, which is the same bug that only shows up once the hidden rows are not contiguous. The fractional-delta test failing means `Math.trunc` was applied but `Number.isFinite` was not.

- [ ] **Step 11: Write the failing tests for time formatting**

Contract §7.4 fixes the signatures and gives its worked examples — `"41m"`, `"1h 06m"`, `"2d"` and `"waiting 5h 51m"` — and the reason both functions take `nowMs` instead of calling `Date.now()`: they are pure, a test needs no fake timers, and every caller inside one render tick prints a consistent number from `useHeartbeat()`. That last one is not a nicety. Two panes rendering `42m` and `41m` for sessions started in the same second is the kind of detail that makes an interface read as unreliable.

What those examples do not pin, decided here. **(f) The leading unit is unpadded and the trailing unit is zero-padded to two.** That is the rule that generates every one of them exactly — `41m`, `1h 06m`, `2d`, `waiting 5h 51m` — and it keeps the string width stable within a magnitude, which is what `font-variant-numeric: tabular-nums` (`docs/DESIGN-SYSTEM.md`, "every live number") needs to line up a column. **(g) Past 24 hours it prints days and hours, `1d 03h`, collapsing to `2d` only when the hour remainder is zero.** §7.4's example is `2d`, which this produces exactly; the hours arm is an addition, and the justification is §7.4's own sentence that "aging must be legible or wasted idle time is invisible" — a bare `3d` cannot distinguish a session idle since this morning from one idle since three days ago plus twenty-three hours. **(h) An absent or unparseable timestamp renders an em dash and never a duration.** `Date.parse("")` is `NaN`, and the alternatives are worse: `"0m"` would claim the session started now, `"NaN m"` ships the bug to the user, and throwing turns a missing field into a blank pane. Everything floors rather than rounds, so a duration is never displayed before it has elapsed, and a negative interval from clock skew clamps to zero rather than printing a minus sign.

Create `desktop/src/lib/time.test.ts`:

```ts
import { describe, expect, it } from "vitest";

import { formatElapsed, formatWaiting } from "./time";

const MIN = 60_000;
const HOUR = 60 * MIN;
const DAY = 24 * HOUR;

const DASH = "—"; // em dash: what an unparseable timestamp renders as

const BASE = "2026-09-18T09:00:00.000Z";
const now = Date.parse(BASE);
const ago = (ms: number): string => new Date(now - ms).toISOString();

describe("formatElapsed", () => {
  it("floors to whole minutes below an hour, so it never shows time that has not elapsed", () => {
    expect(formatElapsed(ago(0), now)).toBe("0m");
    expect(formatElapsed(ago(59_000), now)).toBe("0m");
    expect(formatElapsed(ago(41 * MIN), now)).toBe("41m");
    expect(formatElapsed(ago(41 * MIN + 59_000), now)).toBe("41m");
    expect(formatElapsed(ago(59 * MIN), now)).toBe("59m");
  });

  it("zero-pads the minutes once there is an hour, and leaves the hour unpadded", () => {
    expect(formatElapsed(ago(60 * MIN), now)).toBe("1h 00m");
    expect(formatElapsed(ago(HOUR + 6 * MIN), now)).toBe("1h 06m");
    expect(formatElapsed(ago(23 * HOUR + 59 * MIN), now)).toBe("23h 59m");
  });

  it("switches to days at 24 hours and keeps the hour when it is not zero", () => {
    expect(formatElapsed(ago(DAY), now)).toBe("1d");
    expect(formatElapsed(ago(2 * DAY), now)).toBe("2d");
    expect(formatElapsed(ago(DAY + 3 * HOUR + 40 * MIN), now)).toBe("1d 03h");
    expect(formatElapsed(ago(9 * DAY + 23 * HOUR), now)).toBe("9d 23h");
  });

  it("clamps clock skew to zero rather than printing a negative duration", () => {
    expect(formatElapsed(new Date(now + 5_000).toISOString(), now)).toBe("0m");
  });

  it("renders an em dash for a timestamp it cannot parse", () => {
    expect(formatElapsed("", now)).toBe(DASH);
    expect(formatElapsed("not a date", now)).toBe(DASH);
  });

  it("renders an em dash when nowMs is not a finite number", () => {
    expect(formatElapsed(BASE, Number.NaN)).toBe(DASH);
    expect(formatElapsed(BASE, Number.POSITIVE_INFINITY)).toBe(DASH);
  });

  it("is pure: it never reads the wall clock, so one render tick prints one number", () => {
    const realNow = Date.now;
    Date.now = () => {
      throw new Error("formatElapsed must not read the wall clock");
    };
    try {
      expect(formatElapsed(ago(41 * MIN), now)).toBe("41m");
    } finally {
      Date.now = realNow;
    }
  });
});

describe("formatWaiting", () => {
  it("prefixes the elapsed form — contract §7.4's worked example", () => {
    expect(formatWaiting(ago(5 * HOUR + 51 * MIN), now)).toBe("waiting 5h 51m");
  });

  it("says waiting 0m rather than nothing at all in the first minute", () => {
    expect(formatWaiting(BASE, now)).toBe("waiting 0m");
  });

  it("does not prefix a value it could not compute", () => {
    expect(formatWaiting("", now)).toBe(DASH);
  });
});
```

Ten tests. `Date.now` is reassigned raw rather than with `vi.spyOn`, and restored in a `finally`, because vitest's `restoreMocks` only restores spies it created — a raw assignment that escapes would poison every later file in the run.

- [ ] **Step 12: Run the time tests to verify they fail**

```bash
npm run typecheck
npm test -- src/lib/time.test.ts
```

One `TS2307` for `./time`, and a vitest collection error naming the same module, in the shapes step 4 describes. Zero tests run.

- [ ] **Step 13: Write the time formatters**

Create `desktop/src/lib/time.ts`:

```ts
// desktop/src/lib/time.ts
//
// Contract §7.4, which quotes docs/AGENTIC-UX.md §10 rule 9: "No 'N ago' as a
// primary timestamp. Primary is 'since you last looked' or 'waiting Xh Ym'."
//
// Both functions take nowMs rather than calling Date.now() — so they are pure,
// so a test needs no fake timers, and so every caller in one render tick prints
// a consistent number. nowMs comes from useHeartbeat().
//
// Formatting rule, which generates every example in §7.4: the leading unit is
// unpadded and the trailing unit is zero-padded to two. Everything floors, so a
// duration is never shown before it has elapsed.

const MINUTE_MS = 60_000;
const HOUR_MS = 60 * MINUTE_MS;
const DAY_MS = 24 * HOUR_MS;

/** Rendered when a timestamp is absent or unparseable. "0m" would claim the session
 *  started now, "NaN m" ships the bug to the user, and throwing blanks the pane. */
const UNKNOWN = "—"; // em dash

function pad2(n: number): string {
  return n < 10 ? `0${n}` : String(n);
}

/** "41m", "1h 06m", "1d 03h", "2d". For how long a session has been alive. */
export function formatElapsed(fromIso: string, nowMs: number): string {
  const fromMs = Date.parse(fromIso);
  if (!Number.isFinite(fromMs) || !Number.isFinite(nowMs)) return UNKNOWN;

  // Clock skew between the machine and the control plane must not print a minus sign.
  const elapsed = Math.max(0, nowMs - fromMs);

  if (elapsed < HOUR_MS) {
    return `${Math.floor(elapsed / MINUTE_MS)}m`;
  }
  if (elapsed < DAY_MS) {
    const hours = Math.floor(elapsed / HOUR_MS);
    const minutes = Math.floor((elapsed % HOUR_MS) / MINUTE_MS);
    return `${hours}h ${pad2(minutes)}m`;
  }
  const days = Math.floor(elapsed / DAY_MS);
  const hours = Math.floor((elapsed % DAY_MS) / HOUR_MS);
  return hours === 0 ? `${days}d` : `${days}d ${pad2(hours)}h`;
}

/** "waiting 5h 51m". For how long something has been idle without the user. Aging must be
 *  legible or wasted idle time is invisible. */
export function formatWaiting(sinceIso: string, nowMs: number): string {
  const elapsed = formatElapsed(sinceIso, nowMs);
  return elapsed === UNKNOWN ? UNKNOWN : `waiting ${elapsed}`;
}
```

`formatWaiting` returns the bare dash rather than `"waiting —"` when it could not compute, because "waiting" followed by nothing reads as a broken render and the dash alone reads as "no value", which is what it is.

- [ ] **Step 14: Fill task 14's reserved insertion point with the six typed fixture builders**

This is the one file this task modifies rather than creates, and shared shape §3 puts the builders here rather than in task 14 or task 17 for a reason that is an ordering bug, not a style preference: task 14 cannot type a builder because the types are this task's, and task 16 — which runs **before** task 17 — imports them, so a builder created in task 17 does not exist when task 16 needs it. `store/types.ts` exists as of step 1, so this is the first point in Group D where the builders can be written at all.

Two edits to `desktop/src/test/fixtures.ts`, both in this step, because shared shape §1 rule 2 forbids adding a reference in one step and defining what it refers to in another.

**Edit 1 — the import clause**, immediately under the file's header doc comment and above `FIXED_NOW`. Only six of §2's types are needed; `import type` is erased entirely under `isolatedModules`, so this adds nothing to the bundle:

```ts
import type {
  Branch,
  Harness,
  Machine,
  Project,
  Session,
  Worktree,
} from "../store/types";
```

**Edit 2 — the builders**, replacing task 14's reserved comment block at the end of the file. That block is a reservation, not code: it names this task, states why the builders land here rather than in task 14 or 17, and carries the six signatures in comment form, already at shared shape §3.2's `aThing` spelling and already typed `Harness`. The `HarnessDescriptor` spelling survives only in §3.2 itself, and task 14's block settles it the same way line 13 of this plan does. The reservation goes away with the comment and this block takes its place:

```ts
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
    name: "panacea",
    path: "C:\\work\\panacea",
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
    path: "C:\\work\\panacea\\.worktrees\\evidence-upload",
    isPrimary: false,
    sessionCount: 1,
    // `ready` is deliberately absent: §2 says it is undefined until the Go contract carries
    // it, and the "Mark ready" verb is not rendered while it is undefined.
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
 * The SHELL descriptor, because it is the one catalogue entry whose three arrays are
 * legitimately empty — which is the shape shared shape §3.2 pins: the Go side allocates
 * unconditionally, so they are `[]` and never `null`. A test that needs the picker populated
 * passes `aHarness({ id: "claude-code", models: [...], efforts: [...], permissionModes: [...] })`.
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
    note: "A plain shell. No models, no effort levels and no permission modes.",
    ...over,
  };
}
```

The decisions in that block are worth a sentence each, because a later task will otherwise wonder whether they were oversights.

**`aHarness()` is the shell, not `claude-code`, and that is what makes its three empty arrays honest.** Shared shape §3.2 requires `models`, `efforts` and `permissionModes` to be `[]`, and §5.1 is the Go-side guarantee behind it — never `null`, never `undefined` — because Group B pinned the Go side to allocate unconditionally and a fixture that disagreed would hide the regression. Handing that shape to a `claude-code` descriptor would be a lie about the catalogue; handing it to the shell descriptor is simply true, since the shell has no models, no effort levels and no permission modes. A test that needs a populated picker overrides, which is §3.2's rule anyway.

**`aSession()` is `claude-code` because that is `APP_DEFAULTS.harness` in contract §3.4** — the common case, and the harness a session is actually created with. It does not agree with `aHarness()`, deliberately: they are independent builders of independent surfaces, and a test that needs the two to line up passes the override.

**`message` and `ready` are the only two fields left out, and both are conditional in §2.** A `Machine.message` on a `running` machine would be an invalid object, and a defined `Worktree.ready` would render a "Mark ready" verb the Go contract does not yet support. Everything else is populated.

**The timestamps make the contract's own worked example fall out.** `createdAt` is 41 minutes before `FIXED_NOW`, so `formatElapsed(aSession().createdAt, FIXED_NOW)` is `"41m"` — contract §7.4's first example — and `lastOutputAt` is two minutes before, so `formatWaiting(aSession().lastOutputAt, FIXED_NOW)` is `"waiting 2m"`. I ran both, under Node 24 against step 13's `time.ts`, and those are the returned strings.

Then verify, from `desktop/`:

```bash
npm run typecheck
grep -cE '^export function a[A-Z]' src/test/fixtures.ts
grep -rlE '^export function a(Machine|Project|Branch|Worktree|Session|Harness)\(' src
```

Expected, and every number here was read off a run rather than reasoned about — I built the seven files of this task plus this `fixtures.ts` into a scratch tree with a copy of `desktop/tsconfig.json` and ran the repository's own `tsc.js` 6.0.3 over it:

```
6
src/test/fixtures.ts
```

with `tsc` silent and exit 0.

**Both greps are anchored at column zero, per shared shape §3A, and the contrast is measured rather than asserted.** The naive form `grep -c 'aHarness' src/test/fixtures.ts` returns **2** against the correct file, because the doc comment above `aHarness` names the builder it documents — a guard written that way fails on correct code and sends the implementer hunting a duplicate that does not exist. `^export function a[A-Z]` cannot match a `//` or `*` comment line, so it returns **6** and nothing else. The second grep is the one that enforces §3.2's closed inventory: it must print `src/test/fixtures.ts` and nothing else, now and in every later task.

- [ ] **Step 15: Run the whole suite and the type checker**

```bash
npm test
npm run typecheck
```

This task contributes **three test files and 34 tests** — 6, 18 and 10. Step 14's builders add none: `fixtures.ts` is test data rather than product code, its correctness is a type question and `npm run typecheck` is the check that answers it, and a `fixtures.test.ts` would be a file contract §9 assigns to nobody. Task 14's `src/styles/tokens.test.ts` runs in the same pass; its count belongs to task 14 and is not asserted here. So the acceptance criterion is *no `FAIL` line anywhere*, plus these three lines, whose file order in the reporter follows completion time rather than the alphabet and so may differ from run to run:

```
 ✓ src/lib/ids.test.ts (6 tests) 9ms
 ✓ src/lib/immutable.test.ts (18 tests) 12ms
 ✓ src/lib/time.test.ts (10 tests) 8ms
```

`npm run typecheck` prints nothing but npm's banner. That is worth one sentence of evidence rather than an assumption: I ran `node_modules/.bin/tsc --noEmit` against `desktop/` as it stands before this task and it exited 0 with no output, on TypeScript **6.0.3**. So the baseline is clean and any diagnostic you see here is yours.

**What was and was not verified while writing this plan.** The four source files in steps 1, 5, 9 and 13, the three test files, and step 14's `fixtures.ts` were compiled together with that same `tsc` under this repository's exact `compilerOptions` — `strict`, `noUnusedLocals`, `noUnusedParameters`, `isolatedModules`, `target: ES2020`, `lib: ["ES2020","DOM"]` — and exited 0 with no output. Every expected value in the three test files was then executed under Node 24 outside vitest, all of them passing, including the base-36 rollover at 35, the 2000-id uniqueness run and all eighteen `formatElapsed` assertions; the six ids tests were also run against a post-increment `makeId` to capture the `1 passed | 5 failed` signature in step 6, and the six builders were called and their returned objects inspected field by field. The step-2 diagnostics table and step 8's `TS2307` block are captured `tsc` output, not predictions. What was **not** run is vitest itself, because task 14 installs it — so treat every `✓`/`Test Files`/`Tests` block in this plan as the reporter's documented shape and the counts as the assertion, not as a captured transcript.

- [ ] **Step 16: Commit**

Staged by name per shared shape §6 — the seven files this task creates, plus task 14's `fixtures.ts`, which step 14 modifies. Never `git add -A`, `-u` or `.`:

```bash
git add desktop/src/store/types.ts \
        desktop/src/lib/ids.ts \
        desktop/src/lib/ids.test.ts \
        desktop/src/lib/immutable.ts \
        desktop/src/lib/immutable.test.ts \
        desktop/src/lib/time.ts \
        desktop/src/lib/time.test.ts \
        desktop/src/test/fixtures.ts
git commit -m "feat(cockpit): the domain model, client ids and the immutable helpers

store/types.ts is transcribed verbatim from the UI contract's section 2 and
verified with a diff rather than by eye, because every server-derived field
name is the JSON name the control plane emits and a rename here is not a
compile error anywhere. It is a runtime undefined the type system blesses:
lastOutput instead of lastOutputAt types as string, arrives undefined, and
renders a dash forever.

The cardinality is the part that constrains later tasks. Many sessions per
worktree, so Worktree has no sessions array to splice. A worktree is a
checkout on disk that outlives every session that ran in it, and the
prototype deleting the row when the last session is killed would make the
user's branch appear to vanish.

The three helper modules each close a known defect. makeId pairs a counter
with a random suffix because the counter alone collides across two windows
and randomUUID alone puts 36 characters in every React key. replaceAt keeps
the tail the prototype's splice-and-push dropped. moveWithinVisible treats
hidden rows as fixed anchors and permutes the visible ones among the slots
they already occupy, so reordering two ready worktrees can no longer swap an
unrelated row sitting between them. formatElapsed and formatWaiting take
nowMs rather than reading the clock, so every elapsed number in one render
tick is the same number.

It also fills task 14's reserved point in src/test/fixtures.ts with the six
typed builders aMachine, aProject, aBranch, aWorktree, aSession and aHarness.
They land here rather than later because task 14 cannot type them and task 16
runs before task 17, so a builder written in 17 does not exist when 16 imports
it. The inventory is closed: a variant is an override object, not a seventh
builder.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```
