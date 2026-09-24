import { describe, expect, it } from "vitest";
import ts from "typescript";
import { build, isFileLoadingAllowed, resolveConfig, type Plugin } from "vite";

// Task 03's declared amendments, each measured through the tool that reads it (phase 5 plan,
// task-03.md "Tests first"). `npx tsc --noEmit` cannot see the paths block while both lockfiles
// pin the same @types/react (TypeScript merges packages with one name@version); vitest never
// reads vite.config.ts; editorPackage.test.tsx's probe imports react only. So these ask
// TypeScript's resolver, a Vite build and Vite's file-serving check directly.

const slash = (p: string) => p.replace(/\\/g, "/");
// desktop/, from this file's own path. Not import.meta.url: under jsdom Vitest serves it as an
// /@fs/ URL. The desktop has no @types/node, so no node:path either.
const testPath = slash(expect.getState().testPath ?? "");
const desktopDir = testPath.replace(/\/src\/design\/editorPackage\.config\.test\.ts$/, "");
const configFile = `${desktopDir}/vite.config.ts`;
if (desktopDir === testPath) throw new Error(`cannot locate desktop/ from ${testPath}`);

function tsOptions(): ts.CompilerOptions {
  const host: ts.ParseConfigFileHost = {
    ...ts.sys,
    onUnRecoverableConfigFileDiagnostic: (d) => {
      throw new Error(ts.flattenDiagnosticMessageText(d.messageText, "\n"));
    },
  };
  const parsed = ts.getParsedCommandLineOfConfigFile(`${desktopDir}/tsconfig.json`, {}, host);
  if (!parsed) throw new Error("desktop/tsconfig.json did not parse");
  return parsed.options;
}

function resolveTypes(spec: string, from: string, options: ts.CompilerOptions): string {
  const r = ts.resolveModuleName(spec, from, options, ts.sys).resolvedModule;
  if (!r) throw new Error(`TypeScript cannot resolve ${spec} from ${from}`);
  return slash(r.resolvedFileName);
}

describe("canvas editor package: the desktop's config amendments", () => {
  // tsconfig.json compilerOptions.paths
  it("one @types/react: the package's react types are the desktop's own", () => {
    const options = tsOptions();
    const index = resolveTypes("@panacea-app/canvas-editor", `${desktopDir}/src/main.tsx`, options);
    expect(index).toMatch(/\/canvas\/editor\/src\/index\.ts$/);
    for (const spec of ["react", "react/jsx-runtime", "react-dom", "react-dom/client"]) {
      const own = resolveTypes(spec, `${desktopDir}/src/main.tsx`, options);
      expect(own).toMatch(/\/desktop\/node_modules\/@types\/react(-dom)?\//);
      expect(resolveTypes(spec, index, options)).toBe(own);
    }
  });

  // resolve.dedupe in vite.config.ts. In dev the optimiser serves every bare "react" from its one
  // pre-bundle whatever this line says, so the build is where two Reacts would ship. The same
  // build over vitest.config.ts runs Vite's resolver with vitest's resolve options, which
  // editorPackage.test.tsx cannot fully reach: its probe imports react only, never react-dom.
  // So a second module, held in memory, imports react and react-dom from INSIDE the package as
  // tasks 24-26's modules will. Vite resolves a bare import from the importer's directory only
  // when the importer's id, query stripped, is a file on disk (else from the root, where dedupe
  // is moot; vite node.js, tryNodeResolve), so its id is the package's index.ts plus a query.
  it.each(["vite.config.ts", "vitest.config.ts"])("one React in a bundle built with %s", async (file) => {
    const entryId = "\0p5-t03-entry";
    const inPackage = `${desktopDir.replace(/\/desktop$/, "")}/canvas/editor/src/index.ts?p5-t03-mount`;
    const code: Record<string, string> = {
      [entryId]: [
        'import { createElement } from "react";',
        'import { createRoot } from "react-dom/client";',
        'import { PackageProbe } from "@panacea-app/canvas-editor";',
        `import { mount } from ${JSON.stringify(inPackage)};`,
        "createRoot(document.body).render(createElement(PackageProbe));",
        "mount(document.createElement('div'));",
      ].join("\n"),
      [inPackage]: [
        'import { createElement } from "react";',
        'import { createRoot } from "react-dom/client";',
        'import { PackageProbe } from "./probe.tsx";',
        "export function mount(el) { createRoot(el).render(createElement(PackageProbe)); }",
      ].join("\n"),
    };
    const entry: Plugin = {
      name: "p5-t03-entry",
      enforce: "pre",
      resolveId: (id) => (id === "p5-t03-entry" ? entryId : id === inPackage ? inPackage : undefined),
      load: (id) => code[id],
    };
    const out = await build({
      configFile: `${desktopDir}/${file}`,
      root: desktopDir,
      logLevel: "silent",
      plugins: [entry],
      build: { write: false, minify: false, rolldownOptions: { input: "p5-t03-entry" } },
    });
    const outputs = Array.isArray(out) ? out : [out];
    const ids = outputs.flatMap((o) =>
      "output" in o ? o.output.flatMap((c) => (c.type === "chunk" ? c.moduleIds : [])) : [],
    ).map(slash);
    expect(ids.some((id) => id.endsWith("/canvas/editor/src/probe.tsx"))).toBe(true);
    expect(ids).toContain(inPackage);
    expect(ids.filter((id) => /\/desktop\/node_modules\/react\//.test(id))).not.toEqual([]);
    expect(ids.filter((id) => /\/desktop\/node_modules\/react-dom\//.test(id))).not.toEqual([]);
    expect(ids.filter((id) => id.includes("/canvas/editor/node_modules/"))).toEqual([]);
  }, 120_000);

  // vite.config.ts server.fs.allow: the package's source is served, the desktop's own files
  // still are, and the rest of the repository still is not.
  it("the dev server may load the package's source and nothing else outside desktop/", async () => {
    const config = await resolveConfig({ configFile, root: desktopDir, logLevel: "silent" }, "serve");
    const root = slash(config.root);
    const repo = root.replace(/\/desktop$/, "");
    expect(repo).not.toBe(root);
    expect(isFileLoadingAllowed(config, `${repo}/canvas/editor/src/index.ts`)).toBe(true);
    expect(isFileLoadingAllowed(config, `${repo}/canvas/editor/src/probe.tsx`)).toBe(true);
    // The whole package directory, not only src/: a runtime dependency tasks 24-26 add lands in
    // canvas/editor/node_modules (npm installs a linked package's dependencies in its own
    // directory; unverified here, the package has none yet), and an asset it ships, such as a
    // font file, is served from there rather than from the optimiser's pre-bundle.
    expect(isFileLoadingAllowed(config, `${repo}/canvas/editor/node_modules/dep/font.woff2`)).toBe(true);
    expect(isFileLoadingAllowed(config, `${root}/src/main.tsx`)).toBe(true);
    expect(isFileLoadingAllowed(config, `${repo}/go.work`)).toBe(false);
    expect(isFileLoadingAllowed(config, `${repo}/canvas/doc/doc.go`)).toBe(false);
  }, 60_000);
});
