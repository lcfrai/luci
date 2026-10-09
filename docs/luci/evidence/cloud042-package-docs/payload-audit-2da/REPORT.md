# CLOUD-042 — runtime distribution footprint (proposed P3)

This is a metadata/source audit, not a runtime, performance, build or acceptance result. No repository files, test fixtures, Docker resources or package payloads were changed. No archives were decoded and no tree payload hashes were calculated during the sustained workload.

## Observed candidate

Product/source: `2da1a821b8456c20a858fb86ef87649ea917d9de`.
Extracted package: `<extracted-2da-candidate>`.
The separate already-published package identity/provenance remains authoritative; this audit does not independently reverify its archive bytes.

One `os.walk`/`lstat` metadata pass observed **3,587 ordinary files, 1,010 directories, 223,470,105 logical bytes** and no symlinks. `metadata.json` records the complete category summary and largest members.

| Category | Files | Logical bytes |
| --- | ---: | ---: |
| `docs/luci/evidence` | 2,635 | 67,844,220 |
| Other `docs/luci` guides | 37 | 1,014,031 |
| UAT and runbook | 2 | 22,529 |
| Embedded cloud archive | 1 | 67,254,189 |
| Root files, principally 62,759,074-byte API binary | 7 | 62,779,198 |
| Web assets | 424 | 23,014,574 |
| Licences | 377 | 1,294,233 |
| Runtime build context | 104 | 247,131 |

The evidence directory is **64.7 MiB / 30.4% of extracted logical bytes, 73.5% of files**. It includes 1,006 JSON files (29,690,305 bytes), 128 PNGs (17,976,152 bytes), 394 `.mjs` files (6,429,460 bytes), and compressed logs/frozen test sources. It is distributed again inside the cloud archive by construction; this audit did not decode that archive or estimate its compressed evidence contribution. A build is required to report actual archive/installer savings.

## Why it ships and when it is read

- `scripts/build-release.sh:52` recursively copies all `docs/luci` into the cloud package and portable package. Lines 59–77 make the cloud archive before embedding it in the desktop package.
- `scripts/build-release.ps1:110` does the same for portable targets. Its `Copy-Tree` safety checks do not exclude validation evidence.
- `desktop/src-tauri/tauri.conf.json:42` directly maps the entire `../../docs/luci/` tree into native resources; native also includes the cloud archive. Changing only portable scripts would leave native affected.
- Portable launchers select distinct state, web, runtime and cloud paths. `launch-unix.sh:37` does not traverse docs. `package-identity` is read as a small fixed-length identity; `scripts/package.py` computes it from API/web at packaging time, not from docs at startup.
- Native `lib.rs:235` constructs only runtime/cloud paths for the owned sidecar in the inspected startup block.
- Product `server.go:141` defaults projects to `<state>/projects` and reads the retained configuration; `main.go:515` constructs the project scanner from that configured project root. The packaged documentation tree is not the default project/index root.
- Runtime installation `setup_tools.go:324–393` validates four named runtime files, then uses the separately allowlisted `runtime/` as Docker build context. Evidence is outside that context.
- Cloud workspace installation `workspace.go:343–412` reads/hashes/transfers the cloud archive only after connection and explicit install acknowledgement. It does not establish a startup scan.
- A targeted production-source search (excluding tests/vendor) found no `docs/luci`, `docs/UAT`, `docs/RUNBOOK` or `DESIGN.md` references in the API, UI source, native source or sandboxd.

The listed packaging/startup source files are unchanged between current inspected source and exact 2da according to a path-limited Git diff. This is source evidence of the default path, not tracing of every possible access or a promise that users cannot deliberately import Luci source/docs as a project.

**Do not attribute RAM, UI latency or worker disk-layer improvements to this finding.** The evidence tree is not in the agent image's runtime build context. This correction primarily reduces installation/download/storage file count and prevents further acceptance-publication growth in release artifacts. Current sustained acceptance should continue on its pinned original package.

## Safest proposed implementation after measurement

1. Add one shared documentation staging helper used by both release scripts. Keep all existing root documents, UAT/runbook and the 37 top-level Luci guides. Preserve source documentation and all validation capsules in Git; omit only the staged `docs/luci/evidence/**` payload from clean, exactly source-bound runtime distributions.
2. Stage native docs through the same helper and point Tauri at generated resources, rather than directly at `../../docs/luci`. Include this generated path in the existing tightly restricted generated-tree ownership checks. Preserve existing runtime, web, licences, bootstrap resources and the offline cloud bundle.
3. Rewrite Markdown destinations whose repository files are intentionally absent from the staged distribution to `https://github.com/lcfrai/luci/blob/<full sourceCommit>/<repository path>` (directory links may use `tree`). Retain local destinations for every shipped guide and preserve fragments. Do not rewrite upstream URLs, code blocks or plain commands. Verify the referenced source object exists at the exact commit; never substitute `main`, a branch name or a guessed latest revision.
4. Do not merely remove evidence and leave links broken. `link-candidates.json` records a deliberately simple inline-link inventory: **163 evidence links and 24 already-absent helper/source destinations across 10 guides**. This is a useful implementation inventory, not a complete Markdown-parser validation. The already-absent links include context-quality and Windows acceptance helper procedures. Resolve these in the same staging pass while keeping source-clone commands explicit as source commands.
5. Write a small generated validation/provenance index explaining that complete validation artifacts are in the exact source revision and that packaged evidence is not a completion certificate. Preserve all failure and unverified-scope statements in the retained guides.
6. For dirty or source-export inputs with no proven exact source revision, conservatively preserve current evidence-inclusive behavior and disclose unavailable exact hosted provenance; do not link dirty/new evidence to the clean base commit. This avoids breaking existing source-export packaging while limiting the optimization initially to reviewable clean release builds. If exports later receive a trusted source manifest, support it explicitly.

No patch was applied: link rewriting and native staging must be implemented together; a blind exclusion is unsafe for the packaged guides. The outside-checkout artifacts supply concrete scope for that small tracked implementation.

## Deliberately retain

- Root MIT licence/NOTICE and complete third-party grant files/inventory.
- Runtime Dockerfile/entrypoint, vendored sandbox dependencies, go.mod/go.sum and used source. The ten sandboxd `_test.go` files total only 48,028 bytes; pruning them is unnecessary churn for this batch.
- Web lazy chunks/fonts/design assets; the audit did not establish that any are unused.
- The embedded cloud bundle: it supports an existing explicit offline transfer flow. Making it a separate download is a feature/design change, not a safe exclusion here.
- Reproducible Windows, recovery and operator procedures. Source-only scripts need exact-revision retrieval links, not silent removal or a claim they are installed launchers.

## Focused validation after implementation (not executed here)

- Exercise the staging helper with clean exact source and dirty/unknown-source cases: no false source binding; upstream/fragment links preserved; every remaining local guide destination exists; excluded capsules get exact source URLs; malformed/escaping destinations fail safely rather than copying unexpected files.
- Check all three targets' staged manifests plus native resource configuration: evidence payload absent only in optimized mode; guides/licences/runtime bootstrap unchanged in inclusion. Verify normal runtime setup still finds its four required assets.
- Build one matching Linux artifact and measure actual compressed/uncompressed change and member count; preserve source/package identity, licence completeness and startup checks. Do not retest unrelated product suites solely for this packaging change.
- Native Windows packaging/resource layout remains a separate check when a candidate is ready and local capacity permits. No native verification is claimed by this audit.
