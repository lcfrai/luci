import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
// @ts-expect-error type error without @types/node package
import process from "node:process";
const host = process.env.TAURI_DEV_HOST;

// https://vite.dev/config/
export default defineConfig(() => ({
  plugins: [
    react(),
    {
      name: "luci-local-design-fonts",
      enforce: "pre",
      transform(code, id) {
        if (!id.replaceAll("\\", "/").includes("/node_modules/@excalidraw/excalidraw/")) return;
        // Excalidraw 0.18.1 unconditionally adds a CDN font fallback. Keep its
        // lookup local as well as the configured first choice, including SVG
        // export font subsetting. Do not broaden the desktop CSP for a CDN.
        const fallback = /new URL\(([\w$]+),\s*[\w$]+\.ASSETS_FALLBACK_URL\)/g;
        if (!fallback.test(code)) return;
        return {
          code: code.replace(fallback, 'new URL($1, new URL("/design-assets/", window.location.origin))'),
          map: null,
        };
      },
    },
  ],
  optimizeDeps: { exclude: ["@excalidraw/excalidraw"] },

  // Vite options tailored for Tauri development and only applied in `tauri dev` or `tauri build`
  //
  // 1. prevent Vite from obscuring rust errors
  clearScreen: false,
  // 2. tauri expects a fixed port, fail if that port is not available
  server: {
    port: 1420,
    strictPort: true,
    host: host || false,
    hmr: host
      ? {
          protocol: "ws",
          host,
          port: 1421,
        }
      : undefined,
    watch: {
      // 3. tell Vite to ignore watching `src-tauri`
      ignored: ["**/src-tauri/**"],
    },
  },
}));
