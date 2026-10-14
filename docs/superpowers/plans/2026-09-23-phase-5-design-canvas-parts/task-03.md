# Task 03 — The `canvas/editor` npm package, and how the desktop consumes it

**Plan:** `../2026-09-23-phase-5-design-canvas.md` · **Depends on:** — · **Hours:** 1.5

## Why

`OD-11` rule 4 on the TypeScript side: the renderer and editor are a package of their own, tested
alone, which the desktop mounts. Two things go wrong by default when a React app consumes a sibling
React package through `file:`: **two Reacts at runtime** (hooks throw *"Invalid hook call"*), and
**two `@types/react` at compile time** (a `ReactNode` from one is not assignable to the other). This
task wires the package so neither happens, and proves it with one test in each direction.

## Files

- `canvas/editor/package.json`, `tsconfig.json`, `vitest.config.ts`, `src/index.ts`,
  `src/test/setup.ts`, `src/index.test.tsx` (all new)
- Not `canvas/editor/go.mod`: task 02 owns it, the module boundary that keeps `./canvas/...` out
  of this package's `node_modules` (review M6). Do not create, edit or delete it; if task 02 has
  not merged, the directory simply lacks it until then.
- `desktop/package.json` — one dependency. **Declared amendment.**
- `desktop/vite.config.ts` — `resolve.dedupe`, `server.fs.allow`. **Declared amendment.**
- `desktop/vitest.config.ts` — `resolve.dedupe`. **Declared amendment** (phase 4's change C12 is
  that plan's rule for its own tasks; this is one key, and task 30 records it in the UI contract).
- `desktop/tsconfig.json` — `compilerOptions.paths` for `react` and `react-dom`. **Declared amendment.**
- `desktop/src/design/editorPackage.test.tsx` (new) — the one-React test from the desktop side.
- `desktop/src/design/editorPackage.config.test.ts` (new, added by the implementer on
  2026-09-25) — the other three amendments, each measured through the tool that reads it
  (TypeScript's resolver, a Vite production build, Vite's `isFileLoadingAllowed`). Without it the
  `paths` block and `vite.config.ts`'s `dedupe` survive every command below; see *Tests first*.
- `azure-pipelines.yml` — a Node step for the package's tests (after task 02's edit).

## Contracts

```jsonc
// canvas/editor/package.json
{
  "name": "@panacea-app/canvas-editor",
  "private": true,
  "version": "0.1.0",
  "type": "module",
  "license": "UNLICENSED",
  "main": "src/index.ts",
  "types": "src/index.ts",
  "exports": { ".": "./src/index.ts", "./editor.css": "./src/editor/editor.css" },
  "scripts": { "test": "vitest run", "typecheck": "tsc --noEmit" },
  "peerDependencies": { "react": "^19.1.0", "react-dom": "^19.1.0" },
  "dependencies": {},
  "devDependencies": {
    "@testing-library/dom": "10.4.2",
    "@testing-library/react": "16.3.3",
    "@testing-library/user-event": "14.6.7",
    "@types/react": "^19.1.8",
    "@types/react-dom": "^19.1.6",
    "@vitejs/plugin-react": "^6.0.2",
    "jsdom": "30.1.0",
    "react": "^19.1.0",
    "react-dom": "^19.1.0",
    "typescript": "~6.0.3",
    "vitest": "5.0.1"
  }
}
```

Every dev version is the desktop's own (`desktop/package.json`), so one lockfile resolution serves
both. The package ships **source** (`main` is a `.ts` file): Vite treats a linked package as source
and transpiles it, so there is no build step and no `dist/` to go stale. `dependencies` is empty
here; tasks 24–26 add the few runtime libraries they use, each through the gate (task 04).
The `./editor.css` export names `src/editor/editor.css`, where task 25 creates the file
(`task-25.md:17-18`, plan file map `src/editor/`); it read `./src/editor.css` until 2026-09-25,
a path no task creates.

```jsonc
// canvas/editor/tsconfig.json — compilerOptions copied from desktop/tsconfig.json, so a file that
// type-checks here type-checks when the desktop's program compiles it
{
  "compilerOptions": {
    "target": "ES2020", "useDefineForClassFields": true, "lib": ["ES2020", "DOM"],
    "module": "ESNext", "skipLibCheck": true, "moduleResolution": "bundler",
    "allowImportingTsExtensions": true, "resolveJsonModule": true, "isolatedModules": true,
    "noEmit": true, "jsx": "react-jsx", "strict": true, "noUnusedLocals": true,
    "noUnusedParameters": true, "noFallthroughCasesInSwitch": true
  },
  "include": ["src"],
  "exclude": ["spike"]
}
```

```ts
// canvas/editor/vitest.config.ts
import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
export default defineConfig({
  plugins: [react()],
  test: {
    environment: "jsdom",
    include: ["src/**/*.test.{ts,tsx}"],
    setupFiles: ["./src/test/setup.ts"],
    globals: false,
    restoreMocks: true,
    clearMocks: true,
  },
});
```

```ts
// canvas/editor/src/index.ts — the package's public surface; tasks 23–26 and 29 add exports here
// and nowhere else. This task exports exactly one thing, the probe the two tests render.
export { PackageProbe } from "./probe";
```

```tsx
// canvas/editor/src/probe.tsx — deleted by task 23, which replaces it with real exports
import { useState, type ReactNode } from "react";
export function PackageProbe(props: { children?: ReactNode }): ReactNode {
  const [n] = useState(1); // a hook: fails with "Invalid hook call" under two Reacts
  return <span data-testid="probe">{n}{props.children}</span>;
}
```

The desktop amendments, each exactly:

```jsonc
// desktop/package.json — "dependencies" gains one entry
"@panacea-app/canvas-editor": "file:../canvas/editor"
```

```ts
// desktop/vite.config.ts — inside the returned object
resolve: { dedupe: ["react", "react-dom"] },
// and inside server:
fs: { allow: [".", "../canvas/editor"] },
```

```ts
// desktop/vitest.config.ts — top level of defineConfig's object
resolve: { dedupe: ["react", "react-dom"] },
```

```jsonc
// desktop/tsconfig.json — compilerOptions gains
"paths": {
  "react": ["./node_modules/@types/react/index.d.ts"],
  "react/*": ["./node_modules/@types/react/*"],
  "react-dom": ["./node_modules/@types/react-dom/index.d.ts"],
  "react-dom/*": ["./node_modules/@types/react-dom/*"]
}
```

```yaml
# azure-pipelines.yml — appended after the Vet step
  - task: NodeTool@0
    inputs:
      versionSpec: '24.x'
  - script: cd canvas/editor && npm ci && npx tsc --noEmit && npx vitest run
    displayName: Canvas editor
```

`npm ci` needs `canvas/editor/package-lock.json`, which step 2 creates and commits. **Node 24, not
20** (corrected 2026-09-25): this read `'20.x'`, "matching the sandbox image's base"
(`images/sandbox/Dockerfile:10` is `FROM node:20-bookworm-slim`), but the step runs the package's
tests, not the image, and the locked dev dependencies refuse Node 20: `vitest` 5.0.1 declares
`engines.node` `^22.12.0 || ^24.0.0 || >=26.0.0`, `jsdom` 30.1.0 `^22.22.2 || ^24.15.0 || >=26.0.0`
(read from `canvas/editor/node_modules/*/package.json`). The package is tested here on 24.15.0.

## Tests first

| Test | Asserts | Kills the mutant |
|---|---|---|
| `canvas/editor/src/index.test.tsx` › *renders alone* | `render(<PackageProbe/>)` shows `1` | a package that only works inside the desktop |
| `desktop/src/design/editorPackage.test.tsx` › *one React across the boundary* | `render(<PackageProbe><b>x</b></PackageProbe>)` from the desktop shows `1x` and logs no error | the `dedupe` line removed: the hook throws *Invalid hook call* |
| same file › *one `@types/react`* | the file passes a desktop `ReactNode` into `PackageProbe`; `npx tsc --noEmit` in `desktop` is clean | ~~the `paths` block removed: TS2322 on the child~~ **does not kill it** (measured 2026-09-25): both lockfiles pin `@types/react` 19.3.0, and TypeScript merges two packages with one name@version, so `tsc` stays clean. It stays as the compile-time smoke test |
| `desktop/src/design/editorPackage.config.test.ts` › *one @types/react: the package's react types are the desktop's own* | TypeScript's own resolver, with `desktop/tsconfig.json`'s options, resolves `react`, `react/jsx-runtime`, `react-dom`, `react-dom/client` from `canvas/editor/src/index.ts` to the same file as from `desktop/src/main.tsx`, under `desktop/node_modules/@types/` | the `paths` block removed, or its `react` or `react-dom` pair |
| same file › *one React in a bundle built with %s*, once per `vite.config.ts` and `vitest.config.ts` | a Vite build (`write: false`) of an entry importing the package, plus a module held in memory whose id is `canvas/editor/src/index.ts?p5-t03-mount` importing `react` and `react-dom/client`, bundles no module under `canvas/editor/node_modules/`. The id must clean to a file on disk: Vite resolves a bare import from the importer's directory only then, else from the root (`tryNodeResolve`, measured on 8.3.1) | either config's `dedupe` removed, or cut to `["react"]` (the part's own test cannot see the cut: the probe never imports `react-dom`). The dev server cannot show the Vite line: its optimiser serves every bare `react` from one pre-bundle (measured), so the build is where two Reacts would ship |
| same file › *the dev server may load the package's source and nothing else outside desktop/* | Vite's `isFileLoadingAllowed` on the resolved config: `canvas/editor/src/*.ts(x)`, a file under `canvas/editor/node_modules/` (a runtime dependency's asset, tasks 24–26) and `desktop/src/main.tsx` yes; `go.work` and `canvas/doc/doc.go` no | `fs.allow` removed; `"."` dropped (the real dev server then answers 403 to `/src/styles/tokens.css?raw`, measured); widened to `../canvas`; narrowed to `../canvas/editor/src` |

## Steps

1. Write the package's seven files as above.
2. `cd canvas/editor && npm install` — creates `node_modules` and `package-lock.json`. Run its test.
3. Edit the four desktop files; `cd desktop && npm install` — creates the junction (see the plan's
   global constraints: **never `rm -rf desktop/node_modules` through it**). In an agent worktree
   `desktop/node_modules` must be **real** for this step, never a junction to the main tree's:
   the install would rewrite the main tree's modules. First cold `vitest` run on a fresh install
   may fail "Timeout waiting for worker to respond" at 60 s; the rerun passes (measured twice).
4. Write the desktop tests (both files); run them; run `npx tsc --noEmit`.
5. Measure the dev server: start `npx vite --port 1499 --strictPort` (an agent: a port from its
   assigned range) in the desktop in the background, request
   `http://localhost:1499/@fs/<absolute path>/canvas/editor/src/index.ts`, and expect 200 (without
   the `fs.allow` line, 403). Use the path's **long** form: reached by an 8.3 name (`AU-DEV~1`)
   the same file answers 403 whatever the config says (measured). Stop it by PID.
6. `cd desktop && npx vite build` — expect success.
7. Edit `azure-pipelines.yml`; commit every path above plus `canvas/editor/package-lock.json` and
   `desktop/package-lock.json`, by pathspec.

## Acceptance

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/phase-5/canvas/editor && npx vitest run && npx tsc --noEmit && echo editor-ok
```
Expected: `1 passed`, then `editor-ok`.

```bash
cd ../../desktop && npx vitest run src/design/ && npx tsc --noEmit && echo desktop-ok
```
Expected: `2 passed (2)` files, `6 passed (6)` tests, then `desktop-ok`. (It named only
`editorPackage.test.tsx`, "1 passed (or 2)", until 2026-09-25; that file holds 2.)

```bash
cmd //c dir node_modules\\@panacea-app
```
Expected: `canvas-editor` listed as `<JUNCTION>`.

```bash
npx vite build >/dev/null && echo build-ok
```
Expected: `build-ok`.

## Not claimed

- `tauri dev` is not started (the owner's hands-on list, task 30); step 5 measures Vite's dev
  server, which is what `tauri dev` serves.
- The package has no product code yet. `PackageProbe` is deleted by task 23.
- The pipeline step is not run here (never push); Node 24.x on the hosted agent is unobserved.
- `vitest.config.ts`'s `react-dom` half is measured through a Vite build over that config (the
  same resolver and options), not inside a vitest run: no package module imports `react-dom`
  until task 24.
