# Luci desktop

The local-first Luci application from [LCFR AI](https://lcfr.ai), built with Tauri, React and TypeScript.

Run `npm ci`, then `npm run dev` for the browser development client. It connects to a local Luci API on `http://127.0.0.1:8730`; set `VITE_LUCI_API` for another address. Use a supported Node release (22.22.2+ on the 22 line, or 24.15+).

The packaged native desktop starts and stops its bundled API on an available loopback port. Set `LUCI_API` to connect to an already-running loopback service instead. Native development (`npm run tauri dev`) requires staging the Go sidecar plus the runtime and licence resources with the repository's release script first.

Run `npm run typecheck`, `npm test` and `npm run build` to verify the frontend. Native packaging additionally requires the Tauri platform prerequisites and Rust toolchain.

`node scripts/verify-journey.mjs` exercises the real local app through headless Chromium. It requires an API configured to a disposable `artifacts/eval-projects` folder; it creates a fixture project and writes results and screenshots under `artifacts/journey-ui`. It never signs in or creates cloud resources.

Set `LUCI_EVAL_CSP=1` for the same journey with the production desktop content security policy. That mode also verifies a bundled design font loads, refuses console policy violations and records evidence under `artifacts/csp-journey`.

Design uses the MIT-licensed Excalidraw editor, loaded only on demand. Its fonts are bundled locally by `prepare-design-assets.mjs`; a documented Vite transform keeps the upstream font fallback on the same local origin. Dependency overrides retain Excalidraw 0.18.1 while applying patched lodash-es and nanoid releases. Run `npm audit --omit=dev` after dependency updates.
