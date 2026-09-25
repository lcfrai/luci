# Licence gate — first run on the repository

**Task:** phase 5, task 04 (`docs/superpowers/plans/2026-09-23-phase-5-design-canvas-parts/task-04.md`,
step 4). **Rule:** `OD-11` rule 3 — dependencies are MIT, Apache-2.0, BSD or ISC only; tldraw is
excluded. **Run:** 2026-09-26, Windows 11, go1.26.2, node v24.15.0, npm 11.12.1, cargo 1.98.1, in
the phase-5 worktree, whose `desktop/node_modules` and `canvas/editor/node_modules` are real
installs (a junctioned `node_modules` is refused, exit 2). Command:
`go run ./tools/licencegate -root .` at commit `5829820` (the output below, verbatim).

## Result

**Exit 0.** Every enforced dependency passes. No exception was needed, so
`tools/licencegate/exceptions.json` stays empty. **No real violation** was found among the enforced
dependencies.

| Ecosystem | Enforced | Pass | Fail | Reported (would fail, not enforced) |
|---|---|---|---|---|
| Go (`canvas`, `control-plane/cmd`, `sandboxd/cmd`) | 57 modules | 57 | 0 | 0 |
| npm `desktop` | 10 (production closure) | 10 | 0 | 6 (dev tooling) |
| npm `canvas/editor` | 1 (`react`, imported by `src/probe.tsx`) | 1 | 0 | 6 (dev tooling) |
| Cargo `desktop/src-tauri` | 0 (`CargoEnforced` false, OQ-P5-2) | — | — | 26 of 262 crates |

The control plane's Kubernetes and Docker trees, which predate `OD-11`, all classify as allowed
licences from their licence files.

## What is reported, and why it is not enforced

**npm (the same six in each project, all dev tooling that no shipped file imports):**

- `lightningcss 1.33.0` and `lightningcss-win32-x64-msvc 1.33.0`, `MPL-2.0`: `vite`'s dependency,
  the case the part predicted (`task-04.md` "What ships means here"). Vite uses it at build time; it
  is not bundled.
- `@csstools/color-helpers 6.1.1` and `@csstools/css-syntax-patches-for-csstree 1.1.14`, `MIT-0`.
- `lru-cache 11.5.3`, `BlueOak-1.0.0`.
- `mdn-data 2.27.1`, `CC0-1.0`.

`MIT-0`, `BlueOak-1.0.0` and `CC0-1.0` are permissive but are not among `OD-11`'s four families;
if any of them is ever imported by shipped source it becomes enforced and fails, and the owner
decides then (a reviewed exception cannot help: its `resolved` must itself be allowed).

## Cargo (reported only: the owner's ruling, OQ-P5-2)

`cargo metadata --offline --locked --format-version 1 --filter-platform x86_64-pc-windows-msvc`
ran in `desktop/src-tauri` from the local registry cache: 263 packages, one workspace member
(`desktop 0.1.0`), 262 crates scanned, **26 would fail** if `CargoEnforced` were true.

The five `MPL-2.0` crates the plan knew of, re-read — **no difference** from the part's table
(`Cargo.lock` lines re-read with grep; chains as the gate prints them):

| Crate | `Cargo.lock` | Reached (normal edges), as printed | Links into the binary? |
|---|---|---|---|
| `cssparser 0.36.0` | `:587` | `tauri 2.11.5 → tauri-utils 2.9.3 → dom_query 0.27.0` | a `lib`: *unverified* |
| `cssparser-macros 0.6.1` | `:600` | `tauri 2.11.5 → tauri-utils 2.9.3 → dom_query 0.27.0 → cssparser 0.36.0 (proc-macro)` | a proc-macro: compile time only |
| `selectors 0.36.1` | `:2923` | `tauri 2.11.5 → tauri-utils 2.9.3 → dom_query 0.27.0` | a `lib` (with a build script): *unverified* |
| `dtoa-short 0.3.5` | `:850` | `tauri 2.11.5 → tauri-utils 2.9.3 → dom_query 0.27.0 → cssparser 0.36.0` | a `lib`: *unverified* |
| `option-ext 0.2.0` | `:2382` | `tauri 2.11.5 → dirs 6.0.0 → dirs-sys 0.5.0` | a `lib`: *unverified* |

The other 21: 18 crates under `Unicode-3.0` alone (`icu_*`, `zerovec`, `yoke`, `tinystr`,
`litemap`, `potential_utf`, `writeable`, `zerofrom`, `zerotrie`, and the `*-derive` proc-macros),
`unicode-ident 1.0.25` as `(MIT OR Apache-2.0) AND Unicode-3.0`, and `foldhash 0.2.0` and
`zlib-rs 0.6.8` as `Zlib` — the same counts the part recorded on 2026-09-24. `same-file 1.0.6` and
`walkdir 2.5.0` (`Unlicense/MIT`), `fnv 1.0.7` (`Apache-2.0 / MIT`) and `brotli-decompressor 5.0.3`
(`BSD-3-Clause/MIT`) pass, because Cargo's `/` reads as OR.

## The gate's output, verbatim

```text
PASS go github.com/coder/websocket v1.8.12 ISC — ISC is allowed (production, from control-plane, sandboxd)
PASS go github.com/docker/docker v28.3.3+incompatible Apache-2.0 — Apache-2.0 is allowed (production, from control-plane)
PASS go github.com/docker/go-connections v0.5.0 Apache-2.0 — Apache-2.0 is allowed (production, from control-plane)
PASS go github.com/docker/go-units v0.5.0 Apache-2.0 — Apache-2.0 is allowed (production, from control-plane)
PASS go github.com/opencontainers/go-digest v1.0.0 Apache-2.0 — Apache-2.0 is allowed (production, from control-plane)
PASS go github.com/opencontainers/image-spec v1.1.1 Apache-2.0 — Apache-2.0 is allowed (production, from control-plane)
PASS go github.com/moby/docker-image-spec v1.3.1 Apache-2.0 — Apache-2.0 is allowed (production, from control-plane)
PASS go github.com/gogo/protobuf v1.3.2 BSD-3-Clause — BSD-3-Clause is allowed (production, from control-plane)
PASS go github.com/containerd/errdefs v1.0.0 Apache-2.0 — Apache-2.0 is allowed (production, from control-plane)
PASS go github.com/containerd/errdefs/pkg v0.3.0 Apache-2.0 — Apache-2.0 is allowed (production, from control-plane)
PASS go github.com/distribution/reference v0.6.0 Apache-2.0 — Apache-2.0 is allowed (production, from control-plane)
PASS go github.com/Microsoft/go-winio v0.6.2 MIT — MIT is allowed (production, from control-plane)
PASS go golang.org/x/sys v0.24.0 BSD-3-Clause — BSD-3-Clause is allowed (production, from control-plane)
PASS go github.com/pkg/errors v0.9.1 BSD-2-Clause — BSD-2-Clause is allowed (production, from control-plane)
PASS go github.com/felixge/httpsnoop v1.0.4 MIT — MIT is allowed (production, from control-plane)
PASS go go.opentelemetry.io/otel v1.28.0 Apache-2.0 — Apache-2.0 is allowed (production, from control-plane)
PASS go go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.53.0 Apache-2.0 — Apache-2.0 is allowed (production, from control-plane)
PASS go github.com/go-logr/logr v1.4.2 Apache-2.0 — Apache-2.0 is allowed (production, from control-plane)
PASS go github.com/go-logr/stdr v1.2.2 Apache-2.0 — Apache-2.0 is allowed (production, from control-plane)
PASS go go.opentelemetry.io/otel/metric v1.28.0 Apache-2.0 — Apache-2.0 is allowed (production, from control-plane)
PASS go go.opentelemetry.io/otel/trace v1.28.0 Apache-2.0 — Apache-2.0 is allowed (production, from control-plane)
PASS go gopkg.in/inf.v0 v0.9.1 BSD-3-Clause — BSD-3-Clause is allowed (production, from control-plane)
PASS go github.com/x448/float16 v0.8.4 MIT — MIT is allowed (production, from control-plane)
PASS go github.com/fxamacker/cbor/v2 v2.7.0 MIT — MIT is allowed (production, from control-plane)
PASS go k8s.io/apimachinery v0.31.1 Apache-2.0 — Apache-2.0 is allowed (production, from control-plane)
PASS go github.com/google/gofuzz v1.2.0 Apache-2.0 — Apache-2.0 is allowed (production, from control-plane)
PASS go k8s.io/utils v0.0.0-20240711033017-18e509b52bc8 Apache-2.0 — Apache-2.0 is allowed (production, from control-plane)
PASS go k8s.io/klog/v2 v2.130.1 Apache-2.0 — Apache-2.0 is allowed (production, from control-plane)
PASS go sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd Apache-2.0 AND BSD-3-Clause — Apache-2.0 AND BSD-3-Clause is allowed (production, from control-plane)
PASS go github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd Apache-2.0 — Apache-2.0 is allowed (production, from control-plane)
PASS go github.com/modern-go/reflect2 v1.0.2 Apache-2.0 — Apache-2.0 is allowed (production, from control-plane)
PASS go github.com/json-iterator/go v1.1.12 MIT — MIT is allowed (production, from control-plane)
PASS go gopkg.in/yaml.v2 v2.4.0 Apache-2.0 — Apache-2.0 is allowed (production, from control-plane)
PASS go sigs.k8s.io/structured-merge-diff/v4 v4.4.1 Apache-2.0 — Apache-2.0 is allowed (production, from control-plane)
PASS go golang.org/x/text v0.16.0 BSD-3-Clause — BSD-3-Clause is allowed (production, from control-plane)
PASS go golang.org/x/net v0.26.0 BSD-3-Clause — BSD-3-Clause is allowed (production, from control-plane)
PASS go k8s.io/api v0.31.1 Apache-2.0 — Apache-2.0 is allowed (production, from control-plane)
PASS go google.golang.org/protobuf v1.34.2 BSD-3-Clause — BSD-3-Clause is allowed (production, from control-plane)
PASS go github.com/golang/protobuf v1.5.4 BSD-3-Clause — BSD-3-Clause is allowed (production, from control-plane)
PASS go github.com/google/gnostic-models v0.6.8 Apache-2.0 — Apache-2.0 is allowed (production, from control-plane)
PASS go gopkg.in/yaml.v3 v3.0.1 Apache-2.0 AND MIT — Apache-2.0 AND MIT is allowed (production, from control-plane)
PASS go sigs.k8s.io/yaml v1.4.0 Apache-2.0 AND MIT AND BSD-3-Clause — Apache-2.0 AND MIT AND BSD-3-Clause is allowed (production, from control-plane)
PASS go k8s.io/client-go v0.31.1 Apache-2.0 — Apache-2.0 is allowed (production, from control-plane)
PASS go k8s.io/kube-openapi v0.0.0-20240228011516-70dd3763d340 Apache-2.0 — Apache-2.0 is allowed (production, from control-plane)
PASS go github.com/josharian/intern v1.0.0 MIT — MIT is allowed (production, from control-plane)
PASS go github.com/mailru/easyjson v0.7.7 MIT — MIT is allowed (production, from control-plane)
PASS go github.com/go-openapi/swag v0.22.4 Apache-2.0 — Apache-2.0 is allowed (production, from control-plane)
PASS go github.com/go-openapi/jsonpointer v0.19.6 Apache-2.0 — Apache-2.0 is allowed (production, from control-plane)
PASS go github.com/go-openapi/jsonreference v0.20.2 Apache-2.0 — Apache-2.0 is allowed (production, from control-plane)
PASS go golang.org/x/term v0.21.0 BSD-3-Clause — BSD-3-Clause is allowed (production, from control-plane)
PASS go github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc ISC — ISC is allowed (production, from control-plane)
PASS go golang.org/x/oauth2 v0.21.0 BSD-3-Clause — BSD-3-Clause is allowed (production, from control-plane)
PASS go golang.org/x/time v0.3.0 BSD-3-Clause — BSD-3-Clause is allowed (production, from control-plane)
PASS go github.com/google/uuid v1.6.0 BSD-3-Clause — BSD-3-Clause is allowed (production, from control-plane)
PASS go github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 BSD-3-Clause — BSD-3-Clause is allowed (production, from control-plane)
PASS go github.com/emicklei/go-restful/v3 v3.11.0 MIT — MIT is allowed (production, from control-plane)
PASS go github.com/google/go-cmp v0.6.0 BSD-3-Clause — BSD-3-Clause is allowed (production, from control-plane)
PASS npm @tauri-apps/api 2.11.1 Apache-2.0 OR MIT — Apache-2.0 OR MIT is allowed (production, from desktop)
PASS npm @tauri-apps/plugin-opener 2.5.5 MIT OR Apache-2.0 — MIT OR Apache-2.0 is allowed (production, from desktop)
PASS npm @types/react 19.3.0 MIT — MIT is allowed (production, from desktop)
PASS npm @xterm/addon-fit 0.11.0 MIT — MIT is allowed (production, from desktop)
PASS npm @xterm/xterm 6.0.0 MIT — MIT is allowed (production, from desktop)
PASS npm csstype 3.2.3 MIT — MIT is allowed (production, from desktop)
PASS npm react 19.3.0 MIT — MIT is allowed (production, from desktop)
PASS npm react-dom 19.3.0 MIT — MIT is allowed (production, from desktop)
PASS npm scheduler 0.28.0 MIT — MIT is allowed (production, from desktop)
PASS npm zustand 5.0.15 MIT — MIT is allowed (production, from desktop)
PASS npm react 19.3.0 MIT — MIT is allowed (imported by canvas/editor/src/probe.tsx, from canvas/editor)
REPORT npm @csstools/color-helpers 6.1.1 MIT-0 — MIT-0 is not MIT, Apache-2.0, BSD or ISC (OD-11 rule 3) (from desktop)
REPORT npm @csstools/css-syntax-patches-for-csstree 1.1.14 MIT-0 — MIT-0 is not MIT, Apache-2.0, BSD or ISC (OD-11 rule 3) (from desktop)
REPORT npm lightningcss 1.33.0 MPL-2.0 — MPL-2.0 is not MIT, Apache-2.0, BSD or ISC (OD-11 rule 3) (from desktop)
REPORT npm lightningcss-win32-x64-msvc 1.33.0 MPL-2.0 — MPL-2.0 is not MIT, Apache-2.0, BSD or ISC (OD-11 rule 3) (from desktop)
REPORT npm lru-cache 11.5.3 BlueOak-1.0.0 — BlueOak-1.0.0 is not MIT, Apache-2.0, BSD or ISC (OD-11 rule 3) (from desktop)
REPORT npm mdn-data 2.27.1 CC0-1.0 — CC0-1.0 is not MIT, Apache-2.0, BSD or ISC (OD-11 rule 3) (from desktop)
REPORT npm @csstools/color-helpers 6.1.1 MIT-0 — MIT-0 is not MIT, Apache-2.0, BSD or ISC (OD-11 rule 3) (from canvas/editor)
REPORT npm @csstools/css-syntax-patches-for-csstree 1.1.14 MIT-0 — MIT-0 is not MIT, Apache-2.0, BSD or ISC (OD-11 rule 3) (from canvas/editor)
REPORT npm lightningcss 1.33.0 MPL-2.0 — MPL-2.0 is not MIT, Apache-2.0, BSD or ISC (OD-11 rule 3) (from canvas/editor)
REPORT npm lightningcss-win32-x64-msvc 1.33.0 MPL-2.0 — MPL-2.0 is not MIT, Apache-2.0, BSD or ISC (OD-11 rule 3) (from canvas/editor)
REPORT npm lru-cache 11.5.3 BlueOak-1.0.0 — BlueOak-1.0.0 is not MIT, Apache-2.0, BSD or ISC (OD-11 rule 3) (from canvas/editor)
REPORT npm mdn-data 2.27.1 CC0-1.0 — CC0-1.0 is not MIT, Apache-2.0, BSD or ISC (OD-11 rule 3) (from canvas/editor)
REPORT cargo cssparser 0.36.0 MPL-2.0 — reached by tauri 2.11.5 → tauri-utils 2.9.3 → dom_query 0.27.0 (from desktop/src-tauri)
REPORT cargo cssparser-macros 0.6.1 MPL-2.0 — reached by tauri 2.11.5 → tauri-utils 2.9.3 → dom_query 0.27.0 → cssparser 0.36.0 (proc-macro) (from desktop/src-tauri)
REPORT cargo dtoa-short 0.3.5 MPL-2.0 — reached by tauri 2.11.5 → tauri-utils 2.9.3 → dom_query 0.27.0 → cssparser 0.36.0 (from desktop/src-tauri)
REPORT cargo foldhash 0.2.0 Zlib — reached by tauri 2.11.5 → tauri-utils 2.9.3 → dom_query 0.27.0 (from desktop/src-tauri)
REPORT cargo icu_collections 2.3.0 Unicode-3.0 — reached by tauri 2.11.5 → url 2.5.8 → idna 1.1.0 → idna_adapter 1.2.2 → icu_normalizer 2.3.0 (from desktop/src-tauri)
REPORT cargo icu_locale_core 2.3.0 Unicode-3.0 — reached by tauri 2.11.5 → url 2.5.8 → idna 1.1.0 → idna_adapter 1.2.2 → icu_properties 2.3.0 (from desktop/src-tauri)
REPORT cargo icu_normalizer 2.3.0 Unicode-3.0 — reached by tauri 2.11.5 → url 2.5.8 → idna 1.1.0 → idna_adapter 1.2.2 (from desktop/src-tauri)
REPORT cargo icu_normalizer_data 2.3.0 Unicode-3.0 — reached by tauri 2.11.5 → url 2.5.8 → idna 1.1.0 → idna_adapter 1.2.2 → icu_normalizer 2.3.0 (from desktop/src-tauri)
REPORT cargo icu_properties 2.3.0 Unicode-3.0 — reached by tauri 2.11.5 → url 2.5.8 → idna 1.1.0 → idna_adapter 1.2.2 (from desktop/src-tauri)
REPORT cargo icu_properties_data 2.3.0 Unicode-3.0 — reached by tauri 2.11.5 → url 2.5.8 → idna 1.1.0 → idna_adapter 1.2.2 → icu_properties 2.3.0 (from desktop/src-tauri)
REPORT cargo icu_provider 2.3.1 Unicode-3.0 — reached by tauri 2.11.5 → url 2.5.8 → idna 1.1.0 → idna_adapter 1.2.2 → icu_normalizer 2.3.0 (from desktop/src-tauri)
REPORT cargo litemap 0.8.3 Unicode-3.0 — reached by tauri 2.11.5 → url 2.5.8 → idna 1.1.0 → idna_adapter 1.2.2 → icu_properties 2.3.0 → icu_locale_core 2.3.0 (from desktop/src-tauri)
REPORT cargo option-ext 0.2.0 MPL-2.0 — reached by tauri 2.11.5 → dirs 6.0.0 → dirs-sys 0.5.0 (from desktop/src-tauri)
REPORT cargo potential_utf 0.1.6 Unicode-3.0 — reached by tauri 2.11.5 → url 2.5.8 → idna 1.1.0 → idna_adapter 1.2.2 → icu_normalizer 2.3.0 → icu_collections 2.3.0 (from desktop/src-tauri)
REPORT cargo selectors 0.36.1 MPL-2.0 — reached by tauri 2.11.5 → tauri-utils 2.9.3 → dom_query 0.27.0 (from desktop/src-tauri)
REPORT cargo tinystr 0.8.4 Unicode-3.0 — reached by tauri 2.11.5 → url 2.5.8 → idna 1.1.0 → idna_adapter 1.2.2 → icu_properties 2.3.0 → icu_locale_core 2.3.0 (from desktop/src-tauri)
REPORT cargo unicode-ident 1.0.25 (MIT OR Apache-2.0) AND Unicode-3.0 — reached by serde 1.0.229 → serde_derive 1.0.229 → proc-macro2 1.0.107 (from desktop/src-tauri)
REPORT cargo writeable 0.6.4 Unicode-3.0 — reached by tauri 2.11.5 → url 2.5.8 → idna 1.1.0 → idna_adapter 1.2.2 → icu_normalizer 2.3.0 → icu_provider 2.3.1 (from desktop/src-tauri)
REPORT cargo yoke 0.8.3 Unicode-3.0 — reached by tauri 2.11.5 → url 2.5.8 → idna 1.1.0 → idna_adapter 1.2.2 → icu_normalizer 2.3.0 → icu_collections 2.3.0 (from desktop/src-tauri)
REPORT cargo yoke-derive 0.8.3 Unicode-3.0 — reached by tauri 2.11.5 → url 2.5.8 → idna 1.1.0 → idna_adapter 1.2.2 → icu_normalizer 2.3.0 → icu_collections 2.3.0 → yoke 0.8.3 (proc-macro) (from desktop/src-tauri)
REPORT cargo zerofrom 0.1.8 Unicode-3.0 — reached by tauri 2.11.5 → url 2.5.8 → idna 1.1.0 → idna_adapter 1.2.2 → icu_normalizer 2.3.0 → icu_collections 2.3.0 (from desktop/src-tauri)
REPORT cargo zerofrom-derive 0.1.8 Unicode-3.0 — reached by tauri 2.11.5 → url 2.5.8 → idna 1.1.0 → idna_adapter 1.2.2 → icu_normalizer 2.3.0 → icu_collections 2.3.0 → zerofrom 0.1.8 (proc-macro) (from desktop/src-tauri)
REPORT cargo zerotrie 0.2.5 Unicode-3.0 — reached by tauri 2.11.5 → url 2.5.8 → idna 1.1.0 → idna_adapter 1.2.2 → icu_properties 2.3.0 (from desktop/src-tauri)
REPORT cargo zerovec 0.11.8 Unicode-3.0 — reached by tauri 2.11.5 → url 2.5.8 → idna 1.1.0 → idna_adapter 1.2.2 → icu_normalizer 2.3.0 (from desktop/src-tauri)
REPORT cargo zerovec-derive 0.11.6 Unicode-3.0 — reached by tauri 2.11.5 → url 2.5.8 → idna 1.1.0 → idna_adapter 1.2.2 → icu_normalizer 2.3.0 → zerovec 0.11.8 (proc-macro) (from desktop/src-tauri)
REPORT cargo zlib-rs 0.6.8 Zlib — reached by tauri 2.11.5 → tauri-macros 2.6.3 → tauri-codegen 2.6.3 → png 0.17.16 → flate2 1.1.10 (from desktop/src-tauri)
cargo: 262 crates scanned, reported only (CargoEnforced false, OQ-P5-2); 26 would fail
licencegate: 68 enforced (68 pass, 0 excepted, 0 fail), 38 reported; exit 0
```

## The done-when (spec §9.1 item 13)

`go run ./tools/licencegate -root ./tools/licencegate/testdata/repo-with-mpl` prints
`FAIL npm mpl-fixture 1.0.0 MPL-2.0 — MPL-2.0 is not MIT, Apache-2.0, BSD or ISC (OD-11 rule 3)
(production, from desktop)` and exits 1; `TestMain_repoWithMPL` runs the same with the real runner.

## What the first runs found in the gate itself (fixed in task 04)

1. npm prints an optional peer dependency that is not installed as `{}` (desktop's
   `@vitejs/plugin-react` → `@rolldown/plugin-babel`). The first run exited 2 on it; it is now
   skipped, since nothing is installed to ship.
2. npm redacts UUIDs in what it prints (`@npmcli/redact` 4.0.0: a UUID becomes `***`), so node
   paths under a UUID-named directory (this program's scratchpad) name nothing. The gate now
   restores each `***` element from the one directory entry that redacts to it.
3. `yoke-derive`, `zerofrom-derive` and `zerovec-derive` list `[proc-macro]` plus `[example]` or
   `[test]` targets; the `(proc-macro)` mark now ignores targets that never link.

## Not claimed

- Which Cargo `lib` crates link into the release binary: the chains come from `cargo metadata`,
  which unifies features across build and normal use.
- Whether the hosted `ubuntu-latest` agent has `cargo`, and whether its `cargo fetch --locked`
  gets through; either way the gate prints a Cargo line and exits on the enforced ecosystems.
- Build plugins and fonts are not scanned (the part's "Not claimed").
- The classifier is a fingerprint match, not a legal reading. This is not legal advice.
