import { describe, expect, it } from "vitest";
import ts from "typescript";
import { build, isFileLoadingAllowed, resolveConfig, type Plugin } from "vite";

// Task 03's three other declared amendments, each measured through the tool that reads it.
// editorPackage.test.tsx covers vitest.config.ts's dedupe; `npx tsc --noEmit` cannot see the
// paths block while both lockfiles pin the same @types/react (TypeScript merges packages with
// one name@version), and vitest never reads vite.config.ts. So these ask each tool directly.

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

  // vite.config.ts resolve.dedupe. In dev the optimiser serves every bare "react" from its one
  // pre-bundle whatever this line says, so the build is where two Reacts would ship.
  // The probe imports only "react", so a second module is placed INSIDE the package's directory
  // (in memory, never on disk) importing react and react-dom as tasks 24-26's modules will; its
  // bare imports resolve from canvas/editor, where the package's own copies sit.
  it("one React in a production bundle that mounts the package", async () => {
    const entryId = "\0p5-t03-entry";
    const inPackage = `${desktopDir.replace(/\/desktop$/, "")}/canvas/editor/src/__p5_t03_mount__.js`;
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
        'import { PackageProbe } from "./index.ts";',
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
      configFile,
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
    expect(isFileLoadingAllowed(config, `${root}/src/main.tsx`)).toBe(true);
    expect(isFileLoadingAllowed(config, `${repo}/go.work`)).toBe(false);
    expect(isFileLoadingAllowed(config, `${repo}/canvas/doc/doc.go`)).toBe(false);
  }, 60_000);
});
