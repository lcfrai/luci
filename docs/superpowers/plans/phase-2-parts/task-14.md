### Task 14: Test infrastructure and the token layer

This is the first task in Group D and it has no dependencies, because every other task in the group is written TDD against a runner that does not exist yet: `desktop/package.json` today has no vitest, no jsdom, no testing library and no `test` script. Tasks 15 to 24 all end with `npm test` and `npm run typecheck` from `desktop/`, so if this task's substrate is wrong every later task inherits the wrongness ten times over. It ships two things that look unrelated and are not. The first is **the harness** — the pinned devDependency set, `vitest.config.ts`, `src/test/setup.ts` and the two shared test modules `fakes.ts` and `fixtures.ts`. The second is **the token layer**, `desktop/src/styles/tokens.css`, which contract §5 calls a hard gate: `docs/DESIGN-SYSTEM.md` exists because the first prototype failed by looking machine-generated, and the token layer does more anti-templating work than any component library would. They belong in one task because the rule that makes the token layer enforceable — *no component CSS contains a literal hex* — is a test, and the test needs the harness.

**The xterm-under-jsdom decision is a decision, not a guess, and this task records it rather than re-deriving it.** Contract §1.5: `@xterm/xterm@6` measures character cells with a canvas 2D context and renders with WebGL or a DOM renderer, and jsdom implements neither `CanvasRenderingContext2D` (which would need the optional `canvas` native package and a C++ toolchain on Windows) nor WebGL. **Phase 2 does not unit-test the xterm rendering layer and does not install `canvas`.** What makes that cheap rather than negligent is that the layer is deliberately small: `lib/wire.ts` is pure functions over `Uint8Array` and is unit tested with no DOM at all; `api/attachRegistry.ts` holds the socket lifecycle, the 10 Hz output batching, the pinned-scroll rule and the reconnect-while-`starting` retry, and is tested against an injected fake terminal and fake WebSocket with no jsdom global patched; and `components/Terminal.tsx` is the ~40 lines that construct a real `XTerm`, call `open()` and hand it to the registry, tested through `vi.mock("@xterm/xterm")` so the wiring is asserted and no glyph is ever rendered. Real xterm fidelity — ANSI, resize, a permission prompt drawing correctly — was proven in Phase 1 against a live Claude Code session (`docs/verification/2026-09-17-phase-1-results.md`, 9/9) and is re-proven by hand in Group E. It is not a thing a jsdom assertion can tell the truth about. `FakeTerm` and `FakeWebSocket` are created here, in `src/test/fakes.ts`, and per the Group D shared shape §3 they are **not redefined anywhere else**; task 19 is their heavy user and grows *this* copy if either needs a capability.

**The token file is checkable arithmetic.** Contract §5.2 exists so a reviewer can verify the OKLCH values by hand, and I re-derived all nineteen with Björn Ottosson's sRGB↔Oklab matrices before writing this plan: **every one round-trips to its source hex exactly and every one is in the sRGB gamut** (no channel outside [−0.0005, 1.0005]). Three published numbers differ in the second decimal from my recomputation — `--ink-3` is 3.43 : 1 not 3.44, `--fail` is 6.06 : 1 not 6.07, and `--accent-ink` on the accent fill is 7.42 : 1 not 7.39 — and §5.2's "hues ≈ 276°/303°" for the indigo/violet slop pair computes to 277.12° and 292.72°. None of that changes a decision: `--ink-3` is still above the 3 : 1 non-text floor, `--fail` still passes AA, and the accent at 202.67° is still 74.4° to 90.1° from the banned arc and 54.72° from running-green. Two further corrections of record: §5.2 says "all 18 are in the sRGB gamut" but §5.3 carries **nineteen** hex-documented colours (7 surfaces + 4 ink + 4 accent + 4 status), and the step-6 test pins the set of nineteen so the count is no longer something a reader has to trust.

**The one thing that makes the hex rule mechanical is the strip-comments-first step.** `tokens.css` is *full* of hex — nineteen of them, one per derived colour, in trailing comments, which is precisely what §5.2's arithmetic is for. A naive `grep '#[0-9a-f]'` over it therefore finds nineteen hits and the audit is useless. The two helpers that make it work — `stripCssComments` and `findHexLiterals` — live in `src/test/fakes.ts` and are exported so task 24's `designsystem.test.ts` imports them rather than writing a second regex that disagrees. A third, `declaredCustomProperties`, ships alongside them for the same reason one file down: step 6's name checks must assert on a declaration and never on a `var()` reference. Three exported CSS-audit helpers in all, and task 24 imports the first two. That placement is a decision: §9's row for this task is authoritative for the file list and adds no `cssAudit.ts`, and the shared shape §3 reserves `fakes.ts` for test-only modules, so they go there under their own section header rather than in a file of their own.

**Two traps that this task cannot leave to a later author.** First, `React.StrictMode` (§1.7): `main.tsx` renders inside it, so in dev every effect mounts, unmounts and mounts again, a naive `useEffect(() => new WebSocket(url))` opens two sockets per session, and `@testing-library/react` does **not** enable StrictMode by default, so no unit test catches it. The structural fix is task 19's ref-counted `attachRegistry`, and the reason `FakeWebSocket` exposes a static `instances` array is that task 19 has an explicit step wrapping the render in `<StrictMode>` and asserting exactly one socket was constructed. Second, `@testing-library/jest-dom` is **not** installed, so there is no `toBeInTheDocument`: use `expect(screen.getBy*(...)).toBeDefined()` for presence — `getBy*` already throws when absent — and `expect(screen.queryBy*(...)).toBeNull()` for absence. Same assertion strength, one fewer dependency, one fewer `tsconfig` types entry. Shared shape §3C makes that binding on every Group D task and fixes the four substitutions that actually come up: `toBeInTheDocument()` → `expect(el).not.toBeNull()`, `toBeDisabled()` → `expect(el.disabled).toBe(true)`, `toHaveValue("x")` → `expect(input.value).toBe("x")`, `toBeEmptyDOMElement()` → `expect(el.childNodes.length).toBe(0)`.

Group D's shared shape binds this task in the places named below, and every one of them is honoured. **§3** fixes `fakes.ts` and `fixtures.ts` as this task's property and settles the builder question the first authoring round got wrong: the six typed builders are `aMachine`, `aProject`, `aBranch`, `aWorktree`, `aSession` and `aHarness`, and **task 15** adds all six — not this task, which cannot type them, and not task 17, which runs after task 16 already needs them. This task ships the untyped half (`FIXED_NOW`, `FIXED_NOW_ISO`, `IDS`) and reserves the point, per **§1 rule 1**, in a comment naming task 15. **§3A** requires every pre-commit grep guard to be anchored; this task has none, and correctly so — it adds no call site into a file another Group D task created, so §1 rule 3 does not bite and there is nothing to anchor. **§3B** makes task 24's CSS audit binding on whoever writes the stylesheets, which is why `app.css` declares no colour of its own and why `--scrim` is added to `tokens.css` here rather than left for task 23 to write as a raw `oklch()` in a component file. **§3C** forbids jest-dom matchers, which this task states and does not install. And **§6** fixes the commit trailer and forbids `git add -A`. No step here runs `go test` or touches a `.go` file.

**Files:**
- `desktop/package.json` — the five pinned test devDependencies, `zustand`, and the `test` / `test:watch` / `typecheck` scripts (modify)
- `desktop/package-lock.json` — regenerated by the two installs; staged, because pinned versions without a committed lock are not pinned (modify)
- `desktop/tsconfig.node.json` — `vitest.config.ts` added to `include` (modify)
- `desktop/vitest.config.ts` — jsdom environment, the setup file, `globals: false`, and the one `css` key contract §1.3 does not have (new)
- `desktop/src/test/setup.ts` — `cleanup()` plus the three jsdom gaps: `matchMedia`, `ResizeObserver`, `scrollIntoView` (new)
- `desktop/src/test/fakes.ts` — `FakeTerm`, `FakeWebSocket`, and the three CSS-audit helpers (new)
- `desktop/src/test/fixtures.ts` — the stable ids and frozen clock every later suite shares, and the insertion point reserved for **task 15**'s six typed builders (new)
- `desktop/src/styles/tokens.css` — the token layer in full OKLCH (new)
- `desktop/src/styles/app.css` — the document-level ground: the token import, the reset, the page colours, the tabular figures (new)
- `desktop/src/styles/tokens.test.ts` — the eight assertions that make §5 enforceable (new)

**Interfaces:**
- Produces:
  - `npm test` (`vitest run`, non-watching), `npm run test:watch`, `npm run typecheck` (`tsc --noEmit`) — the three commands every task 15–24 step calls
  - `desktop/vitest.config.ts` — `environment: "jsdom"`, `include: ["src/**/*.test.{ts,tsx}"]`, `setupFiles: ["./src/test/setup.ts"]`, `globals: false`, `restoreMocks`, `clearMocks`, `css: { include: [/\.css\?raw$/] }`
  - `FakeTerm` — contract §1.5's recorder, verbatim, plus its `type()` helper; consumed by tasks 19 and 20
  - `FakeWebSocket` — constructor-recording socket stand-in with `instances`, `reset()`, `acceptOpen()`, `emitBinary()`, `emitText()`, `emitClose()`, `emitError()` and the four `readyState` constants; consumed by tasks 16 and 19
  - `stripCssComments(css)`, `findHexLiterals(css)` — the hex-audit pair; consumed by task 24's `designsystem.test.ts`, which imports exactly these two
  - `declaredCustomProperties(css)` — the third CSS-audit helper, shipped in the same file and section; consumed by this task's own `tokens.test.ts` for the §5.4 name checks
  - `FIXED_NOW`, `FIXED_NOW_ISO`, `IDS` — the frozen clock and the stable id set every later fixture and assertion uses
  - `desktop/src/styles/tokens.css` — all 47 custom properties of contract §5.3 plus `--scrim` (48 declarations), the grain layer, the reduced-motion floor and the focus ring
  - `desktop/src/styles/app.css` — imported by `main.tsx` in task 24, which also deletes `App.css`
- Consumes: nothing. This task imports no application module, which is what lets it be first.
- Required by: every task 15 to 24. Task 15 onward for the runner and `tsconfig`; **task 15** in particular for `fixtures.ts`'s reserved insertion point, which it fills with the six typed builders, and for `FIXED_NOW` / `IDS`, which those builders read; task 16 for `FakeWebSocket` and `IDS`; task 17 for `zustand` and `FIXED_NOW`; task 18 for the RTL pairing, `setup.ts`'s `matchMedia` and every token its CSS references; task 19 for `FakeTerm`, `FakeWebSocket` and `ResizeObserver`; task 20 for `scrollIntoView`; task 24 for `stripCssComments` / `findHexLiterals` and for `app.css`.
- Amends contract §1.3: adds `css: { include: [/\.css\?raw$/] }`, without which the token test silently reads an empty string. Step 2 measures both the mechanism and why the pattern is scoped to the `?raw` query rather than named after one file.
- Amends contract §5.3 and §5.4: adds one token, `--scrim: oklch(0% 0 0 / 0.42)`, so the command palette's backdrop is a `var()` instead of a raw colour function in a component stylesheet. Group D shared shape §3B routes a missing token into this file; task 23's `.pal-scrim` is the case that needs it. `PHASE_2_NAMES` therefore holds twenty names, not nineteen, and `tokens.css` declares 48 custom properties, not 47. Step 7 argues it.
- Amends contract §9's row 14: adds `desktop/package-lock.json` to the file list, and installs `zustand` here as §1.1 directs even though row 17 also marks `package.json` modified for it. Both are argued in step 1.

- [ ] **Step 1: Install the runner and pin it**

Contract §1.1 fixes five test packages and no more. `@testing-library/dom` is listed explicitly because RTL 16 made it a peer dependency rather than a transitive one and npm will not install it for you. There is no `@vitest/coverage-v8`, because nothing in Phase 2 gates on a coverage number and a number nobody gates on is a number that rots. There is no `@types/node`: `vitest.config.ts` below touches no Node API, `tsconfig.json` already sets `skipLibCheck: true` so vitest's own declaration files are never checked, and that is the only reason the repo can stay without it. Test-tool versions are pinned **exactly**, with no caret, while product dependencies keep their existing ranges — a caret on a test runner means a green suite today and a red suite on a colleague's `npm install` tomorrow, for a change nobody made.

`zustand` is installed here too. Contract §1.1 puts it in this task and §9's row 17 also marks `package.json` modified for it, which is a genuine conflict in the contract. Installing it here is the safe direction: task 17's `npm install zustand@5.0.15` is then an idempotent re-assert that changes nothing, whereas the reverse — task 17 discovering mid-TDD that the dependency it imports in step 1 was never installed — costs a round trip. `package-lock.json` is staged with it, because an exact version in `package.json` without a committed lock is not a pin.

Run both installs from `desktop/`, in PowerShell:

```powershell
npm install --save-dev --save-exact vitest@5.0.1 jsdom@30.1.0 `
  @testing-library/react@16.3.3 @testing-library/dom@10.4.2 @testing-library/user-event@14.6.7
npm install --save-exact zustand@5.0.15
```

Expect `added N packages` from each, and **no `npm warn ERESOLVE`**. A peer warning here means the React 19 / RTL 16 pairing resolved wrongly, and the next ten tasks will produce confusing render errors that look like component bugs; stop and read it rather than continuing. The package counts differ by npm version and platform, so do not treat a particular `N` as the check — the check is the absence of the warning.

Then make `desktop/package.json` read exactly this. `--save-exact` is load-bearing: npm's default save-prefix is `^`, so without it the two commands above write `"vitest": "^5.0.1"` and `"zustand": "^5.0.15"` and nothing below is a pin. With it, the install has already written both dependency blocks verbatim and the `scripts` block is the one hand edit — task 17 step 1 documents the same trap where it re-asserts `zustand`:

```json
{
  "name": "desktop",
  "private": true,
  "version": "0.1.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc && vite build",
    "preview": "vite preview",
    "tauri": "tauri",
    "test": "vitest run",
    "test:watch": "vitest",
    "typecheck": "tsc --noEmit"
  },
  "dependencies": {
    "@tauri-apps/api": "^2",
    "@tauri-apps/plugin-opener": "^2",
    "@xterm/addon-fit": "^0.11.0",
    "@xterm/xterm": "^6.0.0",
    "react": "^19.1.0",
    "react-dom": "^19.1.0",
    "zustand": "5.0.15"
  },
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
}
```

`npm test` is the command every Group D task runs and it is deliberately non-watching, so an agent never hangs on it. `test:watch` is the human's.

Verify the pins landed rather than trusting the edit:

```powershell
npm ls vitest jsdom @testing-library/react @testing-library/dom @testing-library/user-event zustand --depth=0
```

Expect a tree listing each at the exact version above and **no `UNMET DEPENDENCY`** and no `invalid` marker. `npm ls` exits non-zero if anything is unmet, so the exit code is the assertion; the printed tree is for reading which one broke.

- [ ] **Step 2: Write `vitest.config.ts`, and the one line contract §1.3 is missing**

Contract §1.3's config is reproduced below with one addition, `css: { include: [/\.css\?raw$/] }`, and both the reason and the exact pattern are measured rather than assumed. The token test in step 6 reads **both** stylesheets as text with Vite's `?raw` query. Vitest ships a plugin pair, `vitest:css-disable` and `vitest:css-empty-post`, whose job is to stop real stylesheets being processed in a unit run. I read the compiled source of the installed copy I had to hand (vitest 4.1.7, `dist/chunks/cli-api.*.js`) and the predicate is:

```js
const cssLangs = "\\.(?:css|less|sass|scss|styl|stylus|pcss|postcss)(?:$|\\?)";
const cssLangRE = new RegExp(cssLangs);
function isCSS(id) { return cssLangRE.test(id); }
// ... enforce: "pre"
transform(code, id) {
  if (!isCSS(id)) return;
  if (!shouldProcessCSS(id)) return { code: "" };
}
```

The id of a raw import is `.../styles/tokens.css?raw`, which matches `\.css(?:$|\?)`, so with the default `css: false` the **pre** transform blanks the module that Vite's asset loader has just filled with the file's text, and the **post** transform replaces it with `export default ""`. Vite's own `vite:css` plugin does not have this problem because it guards on `SPECIAL_QUERY_RE`, which excludes `?raw`, `?url` and `?worker`; vitest's copy of the predicate does not carry that guard. Setting `css.include` to a pattern that matches the raw id makes `shouldProcessCSS` return true, both vitest plugins return `undefined`, and Vite's raw loader result survives untouched.

**The pattern must be scoped to the query, not named after one file, and that is measured.** The obvious form, `[/tokens\.css/]`, opts in exactly the one file it names and leaves `app.css?raw` blanked, so two of step 6's eight assertions fail against an empty string. I extracted this plan's `vitest.config.ts`, `setup.ts`, `fakes.ts`, `tokens.test.ts`, `tokens.css` and `app.css` by line range into a scratch project outside the repository and ran them on the nearest installed version set to step 1's pins — vitest 4.1.9, jsdom 29.1.1, vite 8.1.3, RTL 16.3.2. With `[/tokens\.css/]`:

```
 ❯ src/styles/tokens.test.ts (8 tests | 2 failed) 22ms
     × reads the stylesheets as raw text 9ms
     × has app.css import the token layer first and own no colour of its own 4ms

AssertionError: expected 0 to be greater than 200                     ← the appCss length guard
AssertionError: expected undefined to be '@import "./tokens.css";'    ← the app.css first-line check

 Test Files  1 failed (1)
      Tests  2 failed | 6 passed (8)
```

The two arrows name the **assertion** rather than a line number on purpose. Which line either one lands on depends on how `tokens.test.ts` was transcribed into this plan, and both have already moved once in this plan's history; the message and the assertion it belongs to do not move.

With `[/\.css\?raw$/]` and nothing else changed, the same run is `Test Files  1 passed (1)` / `Tests  8 passed (8)`. `[/tokens\.css/, /app\.css/]` passes too, but the query-scoped form is the better line: it covers task 24's raw reads as well, and it never engages for a real component-CSS import, so nothing is ever injected into jsdom.

I could not re-measure on vitest 5.0.1 — it is not installed on this machine and the corporate network blocks the registry from here — so treat the version as the uncertain part: if vitest 5 has adopted the `SPECIAL_QUERY_RE` guard, this line is a harmless no-op, and step 6's first assertion is there to tell you which world you are in rather than letting the suite pass vacuously.

Create `desktop/vitest.config.ts`:

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
    // Not in contract §1.3, and required. Vitest's `vitest:css-disable` plugin matches any id
    // whose path ends `.css` OR is followed by a query, so `tokens.css?raw` is caught by it and
    // rewritten to `export default ""` under the default `css: false`. Opting the raw query in
    // makes the plugin stand aside so Vite's raw loader result reaches the test.
    // Scoped to `?raw` rather than to one file name ON PURPOSE: tokens.css AND app.css are both
    // read this way, and a per-file pattern silently blanks the one it does not name, which
    // fails two of the eight assertions in src/styles/tokens.test.ts. Measured; see above.
    // Nothing is injected into jsdom by this: `?raw` never reaches the style pipeline, and a
    // real component-CSS import never matches.
    css: { include: [/\.css\?raw$/] },
  },
});
```

Then add the config to `desktop/tsconfig.node.json`'s `include`, which becomes:

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

Be honest about what that line buys. Contract §1.3 says `npm run typecheck` ignores `vitest.config.ts` without it, and that is half right: `tsc --noEmit` without `-b` does not build referenced projects at all, so the config file is not covered by `npm run typecheck` either way. What the `include` does cover is the editor's language service and any later `tsc -b`, and leaving it out would mean a typo in the config is invisible in both. Keep it; do not expect `npm run typecheck` to catch a mistake in it.

- [ ] **Step 3: Write `src/test/setup.ts` — the three jsdom gaps, stubbed once**

jsdom implements neither `matchMedia` nor `ResizeObserver` nor `Element.prototype.scrollIntoView`, and all three are used by real cockpit code: `prefers-reduced-motion` in the shell, the fit addon's container observation and the Canvas tiles in task 21, and defect 4's scroll-into-view in task 20's `ColumnsLayout`. Stubbing them here rather than in twenty test files is the whole point of a setup file. Two of the three details below are load-bearing. Reduced motion is reported **off**, so the motion code paths are the ones under test and a test that wants the reduced path overrides `window.matchMedia` itself. And `scrollIntoView` is a `vi.fn()` spy rather than a no-op, because task 20 has to assert **that it was called**. What is strictly load-bearing is that the property exists here at all: jsdom defines none, and `vi.spyOn(Element.prototype, "scrollIntoView")` — the form task 20 actually uses — throws on a property that is not there. Shipping it as a `vi.fn()` rather than a no-op is what lets a test assert on it directly, without wrapping it first.

Each stub is guarded by a feature check rather than assigned unconditionally, so if a future jsdom grows a real implementation the real one wins and the suite starts testing the truth.

Create `desktop/src/test/setup.ts`:

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

One interaction to know about before task 20 relies on it: `vitest.config.ts` sets `restoreMocks: true`, which restores spies created with `vi.spyOn` after each test but does **not** delete a `vi.fn()` assigned onto a prototype, and `clearMocks: true`, which does reset that function's recorded calls between tests. So `scrollIntoView` survives the whole run as the same spy with its call list cleared per test, which is exactly what an assertion on "was it called in *this* test" needs. Neither option resets module state, which is why the store gets the separate discipline contract §1.6 describes and task 17 implements.

- [ ] **Step 4: Write `src/test/fakes.ts` — the two injectable fakes**

Shared shape §3: `FakeTerm` and `FakeWebSocket` are this task's and are **not redefined anywhere else**. Task 19 is the heavy user of both, and if either needs a capability it grows this copy and says so in its plan. The file has no imports at all — not even `vitest` — which matters, because it is imported from inside `vi.mock` factories and a module with side effects there is a hoisting hazard.

`FakeTerm` is contract §1.5 verbatim. It records what the component does to the terminal and loads none of the real one. Note what it does *not* do: no measurement, no renderer, no `element`. That is the xterm-under-jsdom decision expressed as code.

Create `desktop/src/test/fakes.ts`, first section:

```ts
/**
 * desktop/src/test/fakes.ts
 *
 * Injectable stand-ins shared by every Group D suite. Created by task 14; per the Group D
 * shared shape §3.3 these are NOT redefined in another file — a task that needs a capability
 * GROWS this copy and says so in both plans. TASK 19 is the known one that does: it adds
 * `scrollUp`, `resize`, `openedIn` and `focused` to FakeTerm, here. Replacing the file rather
 * than growing it orphans the non-exported helpers below and breaks this file's own typecheck
 * under `noUnusedLocals`.
 *
 * This module imports nothing on purpose. It is loaded from inside `vi.mock` factories, and
 * the safe form of that is a dynamic import inside the factory, because `vi.mock` is hoisted
 * above the test file's own imports:
 *
 *     vi.mock("@xterm/xterm", async () => {
 *       const { FakeTerm } = await import("../test/fakes");
 *       return { Terminal: FakeTerm };
 *     });
 *
 * A factory that closes over a top-level `import { FakeTerm }` binding can throw "Cannot
 * access '__vi_import_0__' before initialization" depending on which module is pulled first.
 */

/** Records what the component does to the terminal without loading the real one. */
export class FakeTerm {
  cols = 80;
  rows = 24;
  written: string[] = [];
  disposed = false;
  scrolledToBottom = 0;
  buffer = { active: { viewportY: 0, baseY: 0 } };
  private dataHandlers: ((d: string) => void)[] = [];
  loadAddon() {}
  open() {}
  focus() {}
  write(data: string | Uint8Array) {
    this.written.push(typeof data === "string" ? data : new TextDecoder().decode(data));
  }
  scrollToBottom() {
    this.scrolledToBottom += 1;
  }
  onData(cb: (d: string) => void) {
    this.dataHandlers.push(cb);
    return { dispose: () => {} };
  }
  onResize() {
    return { dispose: () => {} };
  }
  dispose() {
    this.disposed = true;
  }
  /** test helper: simulate the user typing */
  type(s: string) {
    this.dataHandlers.forEach((h) => h(s));
  }
}
```

The three `vi.mock` lines that go with it belong in the test file rather than here, because `vi.mock` is hoisted per module and has no effect from a helper. Copy them into any test that renders `<Terminal/>`:

```ts
vi.mock("@xterm/xterm", async () => ({ Terminal: (await import("../test/fakes")).FakeTerm }));
vi.mock("@xterm/addon-fit", () => ({ FitAddon: class { fit() {} activate() {} dispose() {} } }));
vi.mock("@xterm/xterm/css/xterm.css", () => ({}));
```

`FakeWebSocket` is not in the contract as code — §1.5 names it and §1.7 requires it — so its shape is decided here. Four properties are load-bearing. It is **constructor-recording**: `FakeWebSocket.instances` is what task 19's StrictMode step counts to prove the registry opened one socket, not two. It carries the four `readyState` constants as **both** statics and instance fields, because the existing `Terminal.tsx` compares against `WebSocket.OPEN` off the constructor and a registry may compare off the instance. Its `close()` **records and does not fire `onclose`**: a real browser dispatches that event asynchronously, and a fake that fires synchronously would re-enter a registry's own close handler during React cleanup and invent a failure that cannot happen in production — a test that wants the event calls `emitClose()` itself. And `send()` **throws** when the socket is not open, because silently swallowing a write to a closed socket is the exact bug the ref-counted registry exists to prevent.

Append to `desktop/src/test/fakes.ts`:

```ts
export interface FakeMessageEvent {
  data: string | ArrayBuffer;
}

export interface FakeCloseEvent {
  code: number;
  reason: string;
  wasClean: boolean;
}

type Listener = (ev: unknown) => void;

function toArrayBuffer(bytes: Uint8Array): ArrayBuffer {
  const out = new ArrayBuffer(bytes.byteLength);
  new Uint8Array(out).set(bytes);
  return out;
}

/**
 * A structural stand-in for the browser WebSocket, injected wherever the code takes a socket
 * constructor. Inject it as `FakeWebSocket as unknown as typeof WebSocket`: it implements what
 * the cockpit uses, not the whole DOM interface.
 *
 * `instances` is module state. `restoreMocks` and `clearMocks` do NOT clear it, so every suite
 * that touches it calls `FakeWebSocket.reset()` in `beforeEach`.
 */
export class FakeWebSocket {
  static readonly CONNECTING = 0;
  static readonly OPEN = 1;
  static readonly CLOSING = 2;
  static readonly CLOSED = 3;

  /** Every socket constructed since the last reset(), in construction order. */
  static instances: FakeWebSocket[] = [];

  static reset(): void {
    FakeWebSocket.instances = [];
  }

  /** The most recently constructed socket, or undefined if none. */
  static last(): FakeWebSocket | undefined {
    return FakeWebSocket.instances[FakeWebSocket.instances.length - 1];
  }

  readonly CONNECTING = 0;
  readonly OPEN = 1;
  readonly CLOSING = 2;
  readonly CLOSED = 3;

  readonly url: string;
  readonly protocol: string;
  binaryType: "blob" | "arraybuffer" = "blob";
  readyState: number = FakeWebSocket.CONNECTING;

  /** Everything send() was handed, in order. */
  sent: (string | ArrayBufferLike | ArrayBufferView)[] = [];
  /** Set by close(); null while the socket has never been closed by the client. */
  closedWith: { code?: number; reason?: string } | null = null;

  onopen: ((ev: unknown) => void) | null = null;
  onmessage: ((ev: FakeMessageEvent) => void) | null = null;
  onclose: ((ev: FakeCloseEvent) => void) | null = null;
  onerror: ((ev: unknown) => void) | null = null;

  private listeners: Record<string, Listener[]> = {};

  constructor(url: string | URL, protocols?: string | string[]) {
    this.url = String(url);
    this.protocol = Array.isArray(protocols) ? protocols[0] ?? "" : protocols ?? "";
    FakeWebSocket.instances.push(this);
  }

  addEventListener(type: string, cb: Listener): void {
    const list = this.listeners[type];
    if (list) list.push(cb);
    else this.listeners[type] = [cb];
  }

  removeEventListener(type: string, cb: Listener): void {
    this.listeners[type] = (this.listeners[type] || []).filter((l) => l !== cb);
  }

  send(data: string | ArrayBufferLike | ArrayBufferView): void {
    if (this.readyState !== FakeWebSocket.OPEN) {
      throw new Error(`FakeWebSocket: send() while readyState=${this.readyState}, expected OPEN`);
    }
    this.sent.push(data);
  }

  /** Client-side close. Records; does NOT dispatch — the browser's is asynchronous. */
  close(code?: number, reason?: string): void {
    if (this.readyState === FakeWebSocket.CLOSED) return;
    this.readyState = FakeWebSocket.CLOSED;
    this.closedWith = { code, reason };
  }

  // ── test drivers ──────────────────────────────────────────────────────────────────
  acceptOpen(): void {
    this.readyState = FakeWebSocket.OPEN;
    if (this.onopen) this.onopen({});
    this.fire("open", {});
  }

  emitText(data: string): void {
    const ev: FakeMessageEvent = { data };
    if (this.onmessage) this.onmessage(ev);
    this.fire("message", ev);
  }

  emitBinary(bytes: Uint8Array): void {
    const ev: FakeMessageEvent = { data: toArrayBuffer(bytes) };
    if (this.onmessage) this.onmessage(ev);
    this.fire("message", ev);
  }

  emitClose(code = 1000, reason = ""): void {
    this.readyState = FakeWebSocket.CLOSED;
    const ev: FakeCloseEvent = { code, reason, wasClean: code === 1000 };
    if (this.onclose) this.onclose(ev);
    this.fire("close", ev);
  }

  emitError(): void {
    if (this.onerror) this.onerror({});
    this.fire("error", {});
  }

  private fire(type: string, ev: unknown): void {
    for (const l of this.listeners[type] || []) l(ev);
  }
}
```

- [ ] **Step 5: Append the CSS-audit helpers, then write `src/test/fixtures.ts`**

The hex rule is the one design-system rule a reviewer can check mechanically, and it is checkable only if the checker strips comments first — `tokens.css` documents nineteen source hexes in trailing comments deliberately, because that documentation *is* §5.2's arithmetic. These two helpers are the single implementation of that rule; task 24's `designsystem.test.ts` imports them rather than writing a second regex that drifts. `declaredCustomProperties` goes with them so step 6's name checks assert on declarations and never on a `var()` reference.

Append to `desktop/src/test/fakes.ts`:

```ts
// ── CSS audit helpers ───────────────────────────────────────────────────────────────
// Not fakes, and they live here because contract §9's row for task 14 names nine files and no
// cssAudit.ts, and that row is authoritative for the file list. The
// alternative — a second hex regex inside task 24's designsystem.test.ts — is exactly how two
// audits of the same rule come to disagree. Task 24 imports these.

const CSS_BLOCK_COMMENT = /\/\*[\s\S]*?\*\//g;
const HEX_LITERAL = /#(?:[0-9a-fA-F]{8}|[0-9a-fA-F]{6}|[0-9a-fA-F]{3,4})\b/g;
const CUSTOM_PROPERTY_DECL = /^[ \t]*(--[a-z0-9-]+)[ \t]*:/gm;

/**
 * Remove CSS block comments. tokens.css documents its nineteen source hexes in comments on
 * purpose, so a hex audit that does not strip first finds its own documentation.
 */
export function stripCssComments(css: string): string {
  return css.replace(CSS_BLOCK_COMMENT, "");
}

/** Every literal hex colour in the text, in source order, lowercased. */
export function findHexLiterals(css: string): string[] {
  return (css.match(HEX_LITERAL) || []).map((h) => h.toLowerCase());
}

/** Every custom property DECLARED — not merely referenced through var() — in the text. */
export function declaredCustomProperties(css: string): string[] {
  const out: string[] = [];
  CUSTOM_PROPERTY_DECL.lastIndex = 0;
  let m: RegExpExecArray | null = CUSTOM_PROPERTY_DECL.exec(css);
  while (m !== null) {
    out.push(m[1]);
    m = CUSTOM_PROPERTY_DECL.exec(css);
  }
  return out;
}
```

`lastIndex` is reset explicitly because the regex is a module-level `/g` literal and `exec` leaves state on it; without the reset the second call in a run starts mid-file and silently returns a short list. That is a quiet failure, so it is pinned rather than trusted.

Now `desktop/src/test/fixtures.ts`. Shared shape §3.1 makes this file task 14's property and **task 15's to fill**. It ships no typed builder, and that is forced rather than chosen: the domain types a builder would be typed against live in `store/types.ts`, which task 15 creates. It is equally forced that task 15 and not task 17 fills it — task 16 runs first and imports typed fixtures for its client tests, so a builder declared in task 17 would not exist when task 16 reached for it. That ordering bug is what the first authoring round shipped, and §3.1 is the correction. So this file carries the half that needs no types — the stable ids and the frozen clock — plus a reserved insertion point per shared shape §1 rule 1, naming **task 15** and listing the six builders under their exact names so no later author invents a seventh or a second spelling.

```ts
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
```

`FIXED_NOW` is checkable: `new Date(1789722000000).toISOString()` is `"2026-09-18T09:00:00.000Z"`. I computed it rather than copied it.

- [ ] **Step 6: Write the failing token test**

This one file is both the design-system gate and the harness's own smoke test: if it runs at all, vitest, jsdom, the setup file and the raw-import path are all working. Contract §9 gives this task exactly one test file, so it carries every assertion §5 needs enforcing.

Three things in this file are doing work a reader might not expect. The first, the `reads the stylesheets as raw text` case, exists because of step 2's measurement: if the raw import is blanked the other seven pass over an empty string and the gate is decorative. The `filter(...).toEqual([])` form on the name lists is chosen over `toContain` in a loop so a failure prints **every** missing name at once instead of the first; the paired `toHaveLength` stops the list itself being edited down to match a file that lost a token. And the pair `no literal hex in any declaration` / `documents the source hex of all nineteen` is the whole mechanism: hex must appear only as documentation of §5.2's arithmetic, never as a value.

Create `desktop/src/styles/tokens.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import tokensCss from "./tokens.css?raw";
import appCss from "./app.css?raw";
import { declaredCustomProperties, findHexLiterals, stripCssComments } from "../test/fakes";

/** Contract §5.4: the names the already-written Phase 4 Discover plan depends on. Renaming
 *  one of these breaks `desktop/src/discover/` before it is written. */
const PHASE_4_NAMES = [
  "--page", "--canvas", "--surface", "--inset", "--hover",
  "--ink", "--ink-2", "--ink-3", "--ink-4",
  "--line-soft", "--line", "--line-strong",
  "--accent", "--accent-hi", "--accent-tint",
  "--run", "--need", "--fail", "--drift",
  "--r-chip", "--r-ctl", "--r-card", "--r-window",
  "--e-hair", "--e-card",
  "--ease", "--mono", "--sans",
];

/** Contract §5.4's nineteen, plus `--scrim`. The twentieth is this task's addition under Group D
 *  shared shape §3B, which routes a token a component needs into the token layer rather than
 *  letting the component write a raw colour function; task 23's `.pal-scrim` is the case. */
const PHASE_2_NAMES = [
  "--hover-2", "--field", "--thread",
  "--accent-lo", "--accent-ink", "--accent-ring",
  "--run-tint", "--need-tint", "--fail-tint", "--idle", "--exited",
  "--e-btn", "--e-raised", "--e-overlay",
  "--dur-fast", "--dur", "--dur-slow", "--beat", "--numeric",
  "--scrim",
];

/** Contract §5.2/§5.3: every derived colour documents the sRGB hex it round-trips to. All
 *  nineteen were re-derived through the inverse Oklab transform while this plan was written:
 *  each is exact, and no channel falls outside [-0.0005, 1.0005]. */
const DOCUMENTED_HEXES = [
  "#0a0908", "#0f0d0b", "#141210", "#1a1815", "#201e1b", "#262320", "#070605",
  "#f2ede7", "#a79f97", "#6c645d", "#453f39",
  "#2fb3bd", "#74d9e1", "#18848c", "#001517",
  "#43b95e", "#e2a33c", "#ef5f45", "#c98ae0",
];

describe("the token layer", () => {
  it("reads the stylesheets as raw text", () => {
    // Guards a vacuous pass. Vitest's css-disable plugin matches BOTH `tokens.css?raw` and
    // `app.css?raw` and rewrites them to `export default ""` unless vitest.config.ts opts the
    // `?raw` query in; every assertion below would then pass over an empty string and prove
    // nothing. A per-file `css.include` blanks whichever of the two it does not name.
    expect(tokensCss.length).toBeGreaterThan(2000);
    expect(appCss.length).toBeGreaterThan(200);
    expect(tokensCss).toContain(":root");
  });

  it("declares every custom property Phase 4 already depends on", () => {
    const declared = new Set(declaredCustomProperties(tokensCss));
    expect(PHASE_4_NAMES.filter((n) => !declared.has(n))).toEqual([]);
    expect(PHASE_4_NAMES).toHaveLength(28);
  });

  it("declares every custom property Phase 2 adds", () => {
    const declared = new Set(declaredCustomProperties(tokensCss));
    expect(PHASE_2_NAMES.filter((n) => !declared.has(n))).toEqual([]);
    expect(PHASE_2_NAMES).toHaveLength(20);
  });

  it("uses no literal hex in any declaration", () => {
    expect(findHexLiterals(stripCssComments(tokensCss))).toEqual([]);
  });

  it("documents the source hex of all nineteen derived colours, in comments only", () => {
    expect(findHexLiterals(tokensCss).sort()).toEqual([...DOCUMENTED_HEXES].sort());
  });

  it("never uses `transition: all`", () => {
    expect(stripCssComments(tokensCss)).not.toMatch(/transition\s*:\s*all\b/);
    expect(stripCssComments(appCss)).not.toMatch(/transition\s*:\s*all\b/);
  });

  it("ships the reduced-motion floor, the focus ring and the dark colour-scheme", () => {
    const css = stripCssComments(tokensCss);
    expect(css).toContain("color-scheme: dark");
    expect(css).toContain("@media (prefers-reduced-motion: reduce)");
    expect(css).toMatch(/:focus-visible\s*\{/);
    expect(css).toContain("outline: 2px solid var(--accent)");
  });

  it("has app.css import the token layer first and own no colour of its own", () => {
    const lines = stripCssComments(appCss).split("\n").map((l) => l.trim()).filter(Boolean);
    expect(lines[0]).toBe('@import "./tokens.css";');
    expect(findHexLiterals(appCss)).toEqual([]);
    expect(stripCssComments(appCss)).not.toMatch(/\b(?:rgb|rgba|hsl|hsla|oklch)\(/);
  });
});
```

Run `npm test` from `desktop/` and watch it fail on the import, not on an assertion — neither stylesheet exists yet:

```
 RUN  v4.1.9 C:/Users/au-dev-dmackie1/Projects/PanaceaApp/desktop

 ❯ src/styles/tokens.test.ts (0 test)

⎯⎯⎯⎯⎯⎯ Failed Suites 1 ⎯⎯⎯⎯⎯⎯⎯

 FAIL  src/styles/tokens.test.ts [ src/styles/tokens.test.ts ]
Error: Failed to resolve import "./tokens.css?raw" from "src/styles/tokens.test.ts". Does the file exist?
  Plugin: vite:import-analysis
  File: C:/Users/au-dev-dmackie1/Projects/PanaceaApp/desktop/src/styles/tokens.test.ts:2:22
  1  |  import { describe, expect, it } from "vitest";
  2  |  import tokensCss from "./tokens.css?raw";
     |                        ^
  3  |  import appCss from "./app.css?raw";

 Test Files  1 failed (1)
      Tests  no tests
```

That block is measured, not predicted: it is the scratch reproduction described in step 2, on vitest 4.1.9, with only the project root rewritten to the `desktop/` you will be standing in. Your banner will read `v5.0.1` once step 1's pin is installed.

**Do not match on the `Error:` line.** Its wording is the part that moves — vite reports a missing `?raw` import either as the resolve failure above or as `Error: ENOENT: no such file or directory, open 'C:\...	okens.css'`, depending on the minor, and an earlier draft of this plan predicted a third wording that neither version emits. What is stable across both is the `Failed Suites 1` banner, the `FAIL  src/styles/tokens.test.ts [ src/styles/tokens.test.ts ]` line, the string `tokens.css` inside the error, and the two summary lines. Match on those.

What must **not** happen is a green run: a pass here means the test file was not collected, and the first thing to check is `include: ["src/**/*.test.{ts,tsx}"]` against the file's actual path.

- [ ] **Step 7: Write `src/styles/tokens.css` in full**

This is contract §5.3 transcribed, not re-derived. Do not adjust a number: the ramp is arithmetic, with lightness steps of 2.0–2.5 % and chroma rising in step at one warm hue (67°, a warm graphite), and changing one value by eye breaks the perceptual evenness that is the entire reason the file is in OKLCH rather than hex. `oklch()` needs Chromium 111+ / Safari 15.4+; the Tauri 2 webview is WebView2 on Windows and WKWebView on macOS, both well past that, so support is asserted once in a comment rather than doubled into a `@supports` block nobody will ever delete.

Two things in here are deliberate and will look like mistakes to a reviewer skimming. `--field` is **darker** than `--page`, breaking the monotone surface ramp, because a form field is a well you put text into rather than a raised plane — the comment says so and names itself as the one line to change if that is overturned. And `--idle` and `--exited` are both `var(--ink-3)` — aliases onto the ink ramp rather than colours of their own, and onto the *same* rung of it, because a colour for every state is a colour for no state and because `--ink-3` is the lowest rung on the ink ramp that still clears WCAG 1.4.11's 3:1 floor for a graphic you must perceive — `--ink-4`, the rung below it, is 1.92:1. The two states are told apart by shape, disc against square, which is where a status difference belongs.

Create `desktop/src/styles/tokens.css`:

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
   * Lightness rises 14.0 → 16.0 → 18.5 → 21.0 → 23.5 → 26.0 across the six that rise. --field
   * is the seventh name and does NOT rise: it sits below --page at 12.5, and the comment
   * beside it says why. */
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
   * ink-3 is the FLOOR for anything a user must read (3.43:1 on --page).
   * ink-4 is 1.92:1 and is never text: thread lines, hairline glyphs, disabled marks. */
  --ink:   oklch(94.8% 0.0097 73);     /* #f2ede7 — 17.09:1 */
  --ink-2: oklch(70.7% 0.0148 68);     /* #a79f97 —  7.63:1 */
  --ink-3: oklch(50.9% 0.0149 64);     /* #6c645d —  3.43:1 */
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
   * Deep teal. Maximally distant from the indigo/violet tell (74-90 degrees of hue away)
   * and 54.7 degrees off running-green so the two never read as the same signal.
   * If the accent starts appearing decoratively, the hierarchy is broken. */
  --accent:     oklch(70.3% 0.1092 202.7);          /* #2fb3bd — 7.87:1 on --page */
  --accent-hi:  oklch(82.7% 0.0950 202.7);          /* #74d9e1 — hover on an accent fill, and accent text */
  --accent-lo:  oklch(56.0% 0.0900 202.7);          /* #18848c — pressed */
  --accent-ink: oklch(18.0% 0.0300 202.7);          /* #001517 — text ON an accent fill, 7.42:1 */
  --accent-tint: oklch(70.3% 0.1092 202.7 / 0.11);  /* selected-row wash */
  --accent-ring: oklch(70.3% 0.1092 202.7 / 0.28);  /* the ring on a selected row */

  /* ── Status — a SEPARATE vocabulary that never collides with the accent ───────────
   * Each also carries a SHAPE in the component layer: colour alone fails for colourblind
   * users. Phase 2 emits running / idle / exited only; needs-you and drift are defined so
   * Phase 3 does not invent a second amber and a second purple. */
  --run:   oklch(69.8% 0.1671 148);    /* #43b95e — running, 7.90:1 */
  --need:  oklch(75.8% 0.1370 75.8);   /* #e2a33c — needs you (Phase 3+), 9.04:1 */
  --fail:  oklch(66.7% 0.1830 32.3);   /* #ef5f45 — failed, 6.06:1 */
  --drift: oklch(72.7% 0.1378 316.7);  /* #c98ae0 — drifting (Phase 3+) */
  --run-tint:  oklch(69.8% 0.1671 148  / 0.12);
  --need-tint: oklch(75.8% 0.1370 75.8 / 0.12);
  --fail-tint: oklch(66.7% 0.1830 32.3 / 0.12);
  /* "idle" and "exited" are NEUTRAL on purpose. A colour for every state is a colour for
   * no state.
   * Both resolve to --ink-3 (3.43:1 on --page), and that is forced rather than lazy. A status
   * dot is a graphic you must perceive in order to read the interface, so WCAG 1.4.11 puts a
   * 3:1 floor under it, and --ink-4 is 1.92:1 — an earlier draft pointed --idle there and
   * shipped a dot below the floor the ink-ramp comment above asserts. --ink-3 is the
   * lowest rung on this ramp that still clears the floor, so both states land on it
   * and the difference is carried by SHAPE rather than colour: the component layer draws idle
   * as a disc and exited as a square. That is rule 5 at the top of this file, and it is what a
   * colourblind user reads anyway. Do not "fix" the duplication by moving one back to --ink-4. */
  --idle:   var(--ink-3);
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
  /* The backdrop behind a window-class overlay — the command palette, and any modal that
   * follows it. Not in contract §5.3: it is here because Group D's shared shape §3B says a
   * colour a component needs becomes a token rather than a raw colour function in that
   * component's stylesheet, and task 23's `.pal-scrim` is the one that needs it. Pure black at
   * 0.42 alpha, so it darkens the warm ramp without tinting it. Hue 0, chroma 0, so task 24's
   * banned-arc audit over this file is unaffected. */
  --scrim: oklch(0% 0 0 / 0.42);

  /* ── Motion ───────────────────────────────────────────────────────────────────────
   * One house curve on nearly every transition. Durations under 300ms.
   * transform/opacity ONLY. Animating every property at once is banned — it animates
   * layout properties by accident and forces a paint on every unrelated style change. */
  --ease: cubic-bezier(0.23, 1, 0.32, 1);
  --dur-fast: 120ms;
  --dur: 180ms;
  --dur-slow: 260ms;
  /* One global heartbeat (contract §7.1). Every live indicator shares this period AND its
   * phase: the phase is aligned by a negative animation-delay from a shared epoch, not by
   * hoping ten elements mounted on the same frame. */
  --beat: 2400ms;

  /* ── Type ─────────────────────────────────────────────────────────────────────────
   * Geist Sans and a licensed mono are the production pairing in docs/DESIGN-SYSTEM.md.
   * Phase 2 ships neither webfont: the app is offline-first on a corporate laptop, and a
   * font that fails to load is a layout shift. These stacks are the local fallbacks, and
   * swapping the first entry for Geist/Berkeley Mono when the licence lands is one line.
   * The --mono stack is the one xterm.js is configured with, so it must stay a real
   * monospace on every platform. */
  --sans: "Segoe UI Variable Text", "Segoe UI", -apple-system, system-ui, sans-serif;
  --mono: "Cascadia Mono", ui-monospace, "SF Mono", Menlo, Consolas, "DejaVu Sans Mono", monospace;

  /* Live numbers — tokens, cost, elapsed, attach count — get tabular figures or the digits
   * jitter and the interface reads cheap. Applied on <body> and inherited. */
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

**One value is added to §5.3 and one is changed.** The addition is `--scrim`, and it is not a taste call: Group D's shared shape §3B forbids a colour function in a component stylesheet and says a colour a component needs becomes a token here. Task 23's palette backdrop is currently written `.pal-scrim { background: oklch(0% 0 0 / 0.42) }`, which task 24's mechanical audit fails. Putting the value here makes that line `background: var(--scrim)` and the audit passes. It is the last `PHASE_2_NAMES` entry, taking that list from nineteen names to twenty, and it takes the file from 47 declarations to 48. Step 6 asserts the twenty directly; it pins the 48 only as the sum of its two length assertions, 28 + 20, and nothing there forbids a forty-ninth declaration.

The departures from the literal *text* of §5.3, named rather than counted, because one of them is a value and the rest are comment wording. **The Motion block's comment** is reworded where it spelled the banned `transition` shorthand out — the test greps for that string, and a comment containing it would be a false positive the moment anyone loosened the strip. **The published contrast figures** are written as I recomputed them (`--ink-3` 3.43, `--fail` 6.06, `--accent-ink` 7.42) rather than as §5.2 prints them, and the hue-distance sentence says 74–90° rather than 73–100°, because the indigo/violet pair computes to 277.12° and 292.72°, not 276° and 303°. **And one value: `--idle` is `var(--ink-3)`, not `var(--ink-4)`.** §5.3 points it at `--ink-4`, which is 1.92:1 on `--page` and below the 3:1 floor WCAG 1.4.11 sets for a graphic you must perceive — a floor the ink-ramp comment in this same file states out loud, which makes the published value a file that asserts a floor and then ships under it. `--ink-3` at 3.43:1 is the lowest rung that clears it, so `--idle` and `--exited` share it and the disc/square shape vocabulary carries the difference. That is the one place this task overrides a §5.3 value rather than the arithmetic beside it, and it is the one to raise if it is wrong. It is deliberately not an instruction to task 18: task 18's shape vocabulary already pins `idle`→disc and `exited`→square, and reshaping it to rescue a colour value would be the wrong direction.

- [ ] **Step 8: Write `src/styles/app.css`, and watch the suite go green**

Run `npm test` now, before writing anything, and confirm the failure has **moved**: the same import error, naming `./app.css?raw` instead of `./tokens.css?raw`. Measured, with step 7's `tokens.css` on disk and `app.css` still absent:

```
 FAIL  src/styles/tokens.test.ts [ src/styles/tokens.test.ts ]
Error: Failed to resolve import "./app.css?raw" from "src/styles/tokens.test.ts". Does the file exist?
  Plugin: vite:import-analysis
  File: C:/Users/au-dev-dmackie1/Projects/PanaceaApp/desktop/src/styles/tokens.test.ts:3:19
```

That is the check that step 7's file is being read rather than skipped; if the error still names `tokens.css?raw`, the file is in the wrong directory. Note what this run **cannot** tell you: it cannot detect step 2's blanking failure. A missing `app.css` fails at *load*, before a single test is collected, and the css-disable plugin is a `transform` that never gets to run — so a wrong `css.include` is invisible here. It shows up in the opposite direction one run later: once `app.css` exists it is blanked rather than missing, and the suite goes **red** on the raw-text guard's `expect(appCss.length).toBeGreaterThan(200)`, reported as `expected 0 to be greater than 200`. That is the symptom to recognise, and recognising it is the whole reason the raw-text guard is the first `it` in the file.

`app.css` is the document-level ground only: the token import, the reset, the page colours and the typographic defaults everything inherits. It stays thin on purpose. Contract §5.5 ships plain per-component CSS with a component prefix (`rail-`, `sub-`, `top-`, `pane-`, `cv-`, `pal-`, `menu-`, `dot-`), each file beginning with its own `@import "../styles/tokens.css";`, so anything that belongs to one surface belongs in that surface's file and not here. It holds no colour of its own — the last assertion in step 6 enforces that, and it is the rule that keeps task 24's audit meaningful.

Create `desktop/src/styles/app.css`:

```css
/* desktop/src/styles/app.css
 *
 * Document-level ground. The token import, the reset, and the defaults every surface
 * inherits. Deliberately thin: per contract §5.5 each component ships its own prefixed CSS
 * file, so nothing that belongs to one surface belongs here.
 *
 * Task 24 imports this from main.tsx and deletes the Vite starter's App.css. Every value
 * below is a var() from tokens.css; this file declares no colour of its own.
 */
@import "./tokens.css";

*,
*::before,
*::after {
  box-sizing: border-box;
}

html,
body,
#root {
  height: 100%;
}

body {
  margin: 0;
  background: var(--page);
  color: var(--ink);
  font-family: var(--sans);
  font-size: 13px;
  line-height: 1.45;
  /* Live numbers must not jitter. Set once here and inherited. */
  font-variant-numeric: var(--numeric);
  -webkit-font-smoothing: antialiased;
  text-rendering: optimizeLegibility;
  /* The shell scrolls its own regions; the document itself never does. */
  overflow: hidden;
}

code,
kbd,
pre,
samp {
  font-family: var(--mono);
}

:where(button, input, select, textarea) {
  font: inherit;
  color: inherit;
}

::selection {
  background: var(--accent-tint);
  color: var(--ink);
}

/* Scrollbars are part of the surface, not the browser's. The standard properties first, then
 * the WebKit pseudo-elements the Tauri webview actually honours. */
* {
  scrollbar-width: thin;
  scrollbar-color: var(--ink-4) transparent;
}

::-webkit-scrollbar {
  width: 10px;
  height: 10px;
}

::-webkit-scrollbar-track {
  background: transparent;
}

::-webkit-scrollbar-thumb {
  background: var(--ink-4);
  border: 2px solid transparent;
  background-clip: padding-box;
  border-radius: var(--r-chip);
}

::-webkit-scrollbar-thumb:hover {
  background: var(--ink-3);
  background-clip: padding-box;
}
```

Run `npm test` again:

```
 RUN  v4.1.9 C:/Users/au-dev-dmackie1/Projects/PanaceaApp/desktop


 Test Files  1 passed (1)
      Tests  8 passed (8)
   Start at  17:29:01
   Duration  4.55s (transform 153ms, setup 818ms, import 117ms, tests 14ms, environment 2.95s)
```

Measured, from the scratch reproduction of step 2, with every file in steps 2 to 8 extracted from this plan by line range — only the project root is rewritten to the `desktop/` you will be standing in. The banner version, the timestamp and every duration will differ, and your terminal may draw a per-file line above the summary that a piped capture like this one does not; `Test Files  1 passed (1)` and `Tests  8 passed (8)` are the assertion. **Eight, not seven** — a seven means one `it` was dropped, and the one most likely to be dropped is the raw-text guard, which is the one holding the other seven honest.

The duration breakdown is worth reading once rather than skipping: vitest 4 and 5 print `(transform, setup, import, tests, environment)`. If yours prints `collect` and `prepare` instead, you are on a vitest 2 left over from a stale lockfile and step 1's pin did not take.

- [ ] **Step 9: Typecheck, and be honest about what cannot be proven yet**

From `desktop/`:

```powershell
npm run typecheck
```

`tsc --noEmit` prints **nothing** on success and exits 0. There is no summary line to read; silence is the pass.

Three failures are worth predicting, because each means a specific thing drifted rather than the build being broken generally. `Cannot find module './tokens.css?raw' or its corresponding type declarations` means `src/vite-env.d.ts` lost its `/// <reference types="vite/client" />` line — that reference is what supplies `declare module '*?raw'`, and without it the raw imports typecheck nowhere even though vitest runs them fine. Errors reported *inside* `node_modules/vitest/**/*.d.ts` mean `skipLibCheck` was turned off in `tsconfig.json`; leave it on, because it is the reason this repo needs no `@types/node`. And `Module '"../test/fakes"' has no exported member 'declaredCustomProperties'` — three of them, TS2305, all on line 4 of `tokens.test.ts` — means step 5's append never landed. `fakes.ts` is written in two sittings, step 4 and step 5, and the second is the one that gets skipped. Measured, by truncating `fakes.ts` at the end of step 4 and running this repo's own `tsc` (6.0.3) with `desktop/tsconfig.json`'s compiler options:

```
src/styles/tokens.test.ts(4,10): error TS2305: Module '"../test/fakes"' has no exported member 'declaredCustomProperties'.
src/styles/tokens.test.ts(4,36): error TS2305: Module '"../test/fakes"' has no exported member 'findHexLiterals'.
src/styles/tokens.test.ts(4,53): error TS2305: Module '"../test/fakes"' has no exported member 'stripCssComments'.
src/styles/tokens.test.ts(NN,61): error TS7006: Parameter 'l' implicitly has an 'any' type.
```

The fourth coordinate is written `NN` because it is the one here that floats. It points at the `.map((l) => …)` inside the last `it`, which moves whenever anything above it in the test file is edited — and it has moved once already in this plan's history. Column 61 and `Parameter 'l'` are what identify it. The `(4,…)` three do not float: line 4 is the `../test/fakes` import line, which is what those three errors are about.

That fourth line is a consequence of the first three rather than a separate defect: with the import unresolved, `stripCssComments` is `any` and `.split("
").map((l) => …)` has nothing to infer `l` from. With the append in place the same command prints nothing and exits 0 — I ran both.

One prediction is **not** worth making, and an earlier draft of this plan made it. Rewriting step 5's two-call `exec` loop as `while ((m = CUSTOM_PROPERTY_DECL.exec(css)) !== null)` does **not** produce `'m' is possibly 'null'`. TypeScript narrows an assignment-in-condition correctly, and that form compiles clean under this repo's exact options — tsc 6.0.3, `strict`, `noUnusedLocals`, exit 0, measured. Keep the two-call form anyway, because it is the form the explicit `lastIndex` reset reads against, but do not go hunting a type error there if someone changes it.

There is no end-to-end check in this task and it is worth saying so rather than implying one. Nothing renders: there is no component, no store and no client yet, `main.tsx` imports no CSS at all — the Vite starter's `App.css` is still in the tree and nothing imports it — and `app.css` stays unreferenced until task 24 **adds** `import "./styles/app.css"` to `main.tsx` and deletes `App.css`. `npm run dev` would show Phase 1's unchanged `ws://` harness page and prove nothing. The eight assertions in `tokens.test.ts` are the whole of this task's evidence, and the first time the token layer is seen by a human eye is task 18.

No `go test`, no `.go` file: this is Group D.

- [ ] **Step 10: Commit**

```bash
git add desktop/package.json \
        desktop/package-lock.json \
        desktop/tsconfig.node.json \
        desktop/vitest.config.ts \
        desktop/src/test/setup.ts \
        desktop/src/test/fakes.ts \
        desktop/src/test/fixtures.ts \
        desktop/src/styles/tokens.css \
        desktop/src/styles/app.css \
        desktop/src/styles/tokens.test.ts
git commit -m "feat(desktop): the test harness and the OKLCH token layer

Tasks 15 to 24 are written TDD against a runner the desktop did not have.
This adds it: vitest 5, jsdom 30 and RTL 16, all pinned exactly rather than
carated, because a caret on a test runner means a green suite today and a red
one on a colleague's install tomorrow for a change nobody made. The setup file
stubs the three things jsdom lacks and real cockpit code uses - matchMedia,
ResizeObserver and scrollIntoView - once, rather than in twenty test files,
and scrollIntoView is a spy rather than a no-op because task 20 has to assert
that it was called.

xterm is not unit tested and canvas is not installed. That is a decision, not
an omission: xterm measures cells with a 2D context and renders with WebGL,
jsdom has neither, and the native canvas package wants a C++ toolchain on
Windows. The layer is instead kept small enough that not testing it is cheap -
wire.ts is pure, attachRegistry is tested against an injected FakeTerm and
FakeWebSocket, and Terminal.tsx is the forty lines in between. Real terminal
fidelity was proven against a live session in Phase 1 and is re-proven by hand
in Group E; it is not a thing a jsdom assertion can tell the truth about.

The token layer is arithmetic rather than taste. All nineteen colours were
re-derived through the inverse Oklab transform: each round-trips to its source
hex exactly and each is in gamut. The accent sits 54.7 degrees off running
green and 74 to 90 degrees off the indigo/violet tell that makes generated
interfaces recognisable on sight. Three published contrast figures are written
as recomputed rather than as the contract prints them, and the count of
documented colours is nineteen, not the eighteen the contract claims.

One published value is overridden. --idle points at --ink-3 rather than
--ink-4: a status dot is a graphic you must perceive, WCAG 1.4.11 puts a 3:1
floor under it, and --ink-4 is 1.92:1 - which the file's own ink-ramp comment
states out loud. --ink-3 at 3.43:1 is the lowest rung that still clears
the floor, so idle and exited share it and the disc/square shape
vocabulary carries the difference.

One token is added beyond contract 5.3: --scrim, the backdrop behind the
command palette. Group D's shared shape routes a colour a component needs into
the token layer rather than letting the component write a raw oklch(), and
task 23's .pal-scrim is the case that needs it. That makes twenty Phase 2
names, which the suite asserts, and 48 declarations in all.

Hex appears in this file nineteen times and only ever in a comment, which is
what makes the rule checkable: the test strips comments first and then asserts
no literal hex survives in a declaration. Those two helpers live in
src/test/fakes.ts so task 24's design-system audit imports them instead of
writing a second regex that drifts from this one.

One line is not in the contract and is required. Vitest's css-disable plugin
matches any id ending .css or followed by a query, so tokens.css?raw and
app.css?raw are both rewritten to an empty string under the default
css: false, and every assertion in this suite would then pass over nothing.
css.include opts the ?raw query back in - scoped to the query rather than
named after one file, because a per-file pattern silently blanks whichever
sheet it does not name - and the first assertion checks the text is non-empty
so a regression is a failure rather than a silent vacuum.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

