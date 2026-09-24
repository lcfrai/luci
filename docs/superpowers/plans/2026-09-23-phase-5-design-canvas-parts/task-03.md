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
  "exports": { ".": "./src/index.ts", "./editor.css": "./src/editor.css" },
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
      versionSpec: '20.x'
  - script: cd canvas/editor && npm ci && npx tsc --noEmit && npx vitest run
    displayName: Canvas editor
```

`npm ci` needs `canvas/editor/package-lock.json`, which step 2 creates and commits. Node 20 matches
the sandbox image's base (`images/sandbox/Dockerfile:10`).

## Tests first

| Test | Asserts | Kills the mutant |
|---|---|---|
| `canvas/editor/src/index.test.tsx` › *renders alone* | `render(<PackageProbe/>)` shows `1` | a package that only works inside the desktop |
| `desktop/src/design/editorPackage.test.tsx` › *one React across the boundary* | `render(<PackageProbe><b>x</b></PackageProbe>)` from the desktop shows `1x` and logs no error | the `dedupe` line removed: the hook throws *Invalid hook call* |
| same file › *one `@types/react`* | the file passes a desktop `ReactNode` into `PackageProbe`; `npx tsc --noEmit` in `desktop` is clean | the `paths` block removed: TS2322 on the child |

## Steps

1. Write the package's seven files as above.
2. `cd canvas/editor && npm install` — creates `node_modules` and `package-lock.json`. Run its test.
3. Edit the four desktop files; `cd desktop && npm install` — creates the junction (see the plan's
   global constraints: **never `rm -rf desktop/node_modules` through it**).
4. Write the desktop test; run it; run `npx tsc --noEmit`.
5. Measure the dev server: start `npx vite --port 1499 --strictPort` in the desktop in the
   background, request `http://localhost:1499/@fs/<absolute path>/canvas/editor/src/index.ts`, and
   expect 200 (without the `fs.allow` line, 403). Stop it by PID.
6. `cd desktop && npx vite build` — expect success.
7. Edit `azure-pipelines.yml`; commit every path above plus `canvas/editor/package-lock.json` and
   `desktop/package-lock.json`, by pathspec.

## Acceptance

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/phase-5/canvas/editor && npx vitest run && npx tsc --noEmit && echo editor-ok
```
Expected: `1 passed`, then `editor-ok`.

```bash
cd ../../desktop && npx vitest run src/design/editorPackage.test.tsx && npx tsc --noEmit && echo desktop-ok
```
Expected: `1 passed` (or 2), then `desktop-ok`.

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
- The pipeline step is not run here (never push).
