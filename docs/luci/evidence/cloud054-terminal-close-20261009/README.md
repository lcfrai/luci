# CLOUD-054: preserve terminal evidence through attach closure and restart

This is **controlled source verification**, ready for matching installed retesting. The integrated product is the 17 exact files in `integrated-source-binding.json` over `f43542a84bd9cd81c2b263e680c36d8225e10024` (whose product base is `d20ebd292f3f1de04d6726bab0998c210987bcb5`). The commit that contains this capsule supplies the retrievable integrated source. No package, Windows, real-model, latency or complete local-release pass is established here.

The backend relay forwards upstream normal closure before cancelling the opposite pending read. Typed terminal frames remain opaque, and neither normal closure nor typed exit is converted into task success, cleanup, review or integration. The API adds an optional opaque `executionGeneration` equality fingerprint tied to the current allocation. It exposes neither an attach capability nor temporal ordering. The frontend preserves typed terminal evidence through delayed status polls, closure and omitted legacy generation; an actual replacement allocation fences the old socket. Paused output overload remains paused through replacement, and explicit authorized retry attaches the replacement. Legacy rows without trusted generation retain their narrowly scoped explicit-retry path.

`documentation/API.after.md` describes the wire contract. `contract-followup/` retains the canonical UI-contract amendment separately from the full frontend run. `DEFERRED-AUTHORITY-REVIEW.md` preserves the preexisting limitation: a fresh list HTTP 403 records an error but does not itself clear existing private rows. This change prevents stale successful responses from overwriting authoritative removals; it does not claim to fix that separate policy.

## Results and retained failures

Counts below overlap; do not sum them as unique tests. Go named outcomes include top-level and subtests. Vitest assertions are distinguished from configured files.

| Evidence | Original / intermediate | Corrected scope |
|---|---|---|
| Backend relay (`backend/`) | 3 pass / 6 fail in 9 named outcomes, no race | Same 9 pass with race; affected 40 pass (23 top + 17 sub), overlapping |
| Frontend terminal order | 6 pass / 8 fail | Same 14 pass |
| Complete frontend regressions | 10 pass / 18 fail / 31 filtered | 28 pass / 31 filtered |
| Frontend affected run | 290 pass / 45 fail: old mock omitted the new imported function | 336 pass across 8 files after fixture correction; this predates two test-only type annotations |
| External typecheck | Original exit 2 retained in `typecheck-final/` | Corrected exit 0 with 430 stable inputs; 30 affected assertions rerun on the final two test files |
| API generation projection | 9 pass / 1 fail; first fatal prevents later subcases | 17 pass (4 top + 13 sub), race, selected input binding |
| HTTP / team supplement | 22 named pass + 1 authored marker-fixture failure | Corrected management test alone passes; successful HTTP/restart cases are not repeated |
| Overload + replacement supplement | 1 fixture failure: parser reset asserted before the declared flush interval | Same case with the correct existing timer passes; product bytes unchanged |
| **Integrated root Go** | — | **74 pass (34 top + 40 sub), race, zero fail/skip; 1,198 stable captured inputs** |
| **Integrated root full frontend** | **2,826 pass / 1 fail / 0 skip across 161 configured files**; canonical docs omitted new optional field | Application and tests unchanged; after matching the canonical docs, the entire affected seams file passes **24/24**. This is **not a second full-suite pass**. |
| **Integrated root TypeScript** | — | **Exit 0**, 1,199 stable captured inputs shared with the full frontend run |

The Go manifest predates the frontend-only overload test; its actual compiled Go sources are unchanged. The frontend manifest includes that extra test. The external 430-input snapshots include an external Vitest cache-directory overlay; that configuration is not a product change. Existing dependencies were reused without installation or dependency-byte revalidation. Root tool versions are Node **24.19.0**, npm **11.9.0**, Go **1.26.3 linux/amd64**, recorded in `combined/tool-versions.json`. Windows npm 10.9.3 is a separate earlier tester environment.

## Reproduction

Use an isolated checkout of the containing commit and the repository's supported tool setup. Commands below reproduce the final integrated scope; they do not promise identical timing. The complete original commands, selected before/after snapshots, helpers and raw logs are retained next to each result.

From `control-plane/`:

```sh
GOMAXPROCS=2 go test -race -p 1 -count=1 -timeout=5m -json ./internal/proxy ./internal/api ./internal/management -run '^(TestRelay|TestAttach|TestADroppedSocket|TestNewActuallyRoutesTheAttachRoute|TestIsWebSocketUpgrade|TestSessionExecutionGeneration|TestTeamOpaqueExecutionGeneration|TestRestartKeepsTheIDAndMintsANewSandboxAndANewToken|TestSessionJSON)'
```

From `desktop/`:

```sh
npm test -- --maxWorkers=2 --reporter=json --outputFile=/absolute/private/evidence/vitest.json
npm run typecheck
npm test -- src/seams.test.ts --maxWorkers=1
```

Create the separate output directory first. Original external helpers use their captured absolute tooling paths; to replay those historical overlays elsewhere, relocate the source checkout and capsule roots in the helper/overlay paths, preserving the mapped source hashes. They never stand in for a matching installed workflow. The published selected before/final sources, `integrated-original/`, Vitest config overlay and exact helper bytes make those transformations reviewable without publishing an isolated dependency tree.

## Integrity and limitations

All copied artifacts are byte-exact; `original-bindings.json` lists their original hashes and sizes. Generic checkout/tooling locators and inert test markers remain in raw records. No private installed fixture, runtime user state, credentials, node_modules or full isolated tree is included. `publication-input-check.json` verifies the 17 integrated source hashes, subordinate manifests and all captured current repository inputs; it does not attest un-fetched external dependencies. Broad repository snapshots are distinct from complete compiler dependency closures. Preserve every original failed result, even where later follow-ups pass.

`publication-guard/` is a separate read-only verification of immutable **f43542a84bd9cd81c2b263e680c36d8225e10024**: all **183 artifact blobs** in four earlier selected capsules match their checksum declarations (plus four manifest reads). Selected-entry digest `805211b27885c965a8b98babc16a855c1cf8bff07b84a5ceeb55f20059bfd856`. That proves publication completeness for those tracked capsule entries, not their functional conclusions or this new capsule's Git publication. The parent must run the guard on this capsule's actual staged Git blobs and final immutable commit; ignored zero-byte logs must be staged explicitly.

The sustained d20 historical native exits and abnormal WebSocket close observations remain unchanged. This controlled relay mechanism does not retrospectively prove their exact ordering. Matching installed terminal/restart/overload/authority workflows, Windows acceptance, report-capture gaps, live-provider quality, recovery gates and responsiveness remain open. No new timing result or causal performance claim is made. Rust relocation and CLOUD-055 diagnostic coverage are separate artifacts and are not included as C054 validation.
