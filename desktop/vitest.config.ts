import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";

// A separate config from vite.config.ts on purpose: vite.config.ts pins port 1420 and
// strictPort for Tauri, and a test run must never fail because the dev server is up.
export default defineConfig({
  plugins: [react()],
  // Phase 5 task 03, declared amendment: @panacea-app/canvas-editor is linked from
  // ../canvas/editor, which has its own node_modules. Without this its "react" is a second copy
  // and its hooks throw "Invalid hook call" (src/design/editorPackage.test.tsx).
  resolve: { dedupe: ["react", "react-dom"] },
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
    // fails two of the eight assertions in src/styles/tokens.test.ts. Measured; see the plan.
    // Nothing is injected into jsdom by this: `?raw` never reaches the style pipeline, and a
    // real component-CSS import never matches.
    css: { include: [/\.css\?raw$/] },
  },
});
