# Preparing a public-source review candidate

`scripts/export-public-source.py` creates an independently inspectable source folder and ZIP from a clean, exact Git commit. It does not publish anything or change repository visibility. Keep the private repository and its history private while preparing a filtered candidate.

Use Python 3.11 or newer and Git. Commit the complete reviewed implementation first, then supply its full commit ID and matching version. The checkout must have that exact HEAD and no tracked or untracked changes; ignored runtime/build outputs are never read. Choose a new output name for each candidate. Existing folders, ZIPs and checksum files are never overwritten or removed.

```powershell
$sourceRevision = git rev-parse --verify HEAD
python scripts/export-public-source.py --revision $sourceRevision --version 0.5.0 --output artifacts/public-source-candidate/luci-0.5.0-source --plan
python scripts/export-public-source.py --revision $sourceRevision --version 0.5.0 --output artifacts/public-source-candidate/luci-0.5.0-source
```

The same arguments work in a Unix shell after setting `sourceRevision=$(git rev-parse --verify HEAD)`. `--repo` can select another clean checkout. `--plan` performs source/version/licence checks without writing. Exporting from a Git-less candidate is intentionally unsupported; building the application from that candidate is supported.

## What is retained

The script's explicit allowlist retains active source, tests, release scripts, workflow, dependency lockfiles, the vendored sandbox Go dependencies, root MIT licence and attribution, original third-party grants, and current Luci documentation. It reads original Git blobs and preserves line endings and executable modes. The retained third-party licence manifest's file hashes must match; it never substitutes newly generated licence text. Dependency installation and package licence collection are separate build steps.

The shadcn/ui subset retains its original MIT grant and five exact upstream
component snapshots alongside the adapted Luci components. Both licence
collection and source export check the pinned upstream bytes, grant, and adapted
source hashes. Export includes only the named provenance files, not arbitrary
adjacent files. Existing Fluid attribution remains for the retained adaptations.

Private Git metadata/history, corporate archives and previous programmes, infrastructure prototypes, local runtime/provider state, environment files, installed dependencies, generated resources and prior artifacts are excluded. New top-level source trees default to excluded and require a deliberate allowlist update. Source comments and current documents can retain truthful historical references; links into excluded archives will not resolve in the candidate.

One test is explicitly omitted: `desktop/src/seams.test.ts` loads excluded historical contracts, plans and verification reports. All other selected tests remain. This is a documented difference from the private repository suite, not an assertion that the historical audit passed on the exported tree. No application source is rewritten to accommodate the exclusion.

The exporter refuses selected symlinks, submodules, unusual modes, unsafe/colliding paths, unreviewed local dependency references, changed grant hashes and a small set of credential signatures. It records the existing synthetic opening-header-only private-key fixture explicitly. This scan cannot establish that every secret or proprietary element is absent; ownership and source review remain release gates.

## Evidence and acceptance

The folder includes the exact commit and version in `PROVENANCE.md`, the selection rules in `EXPORT-ALLOWLIST.json`, structural checks in `EXPORT-CHECKS.json`, and original Git blob IDs, SHA256 hashes, sizes and modes in `SOURCE-INVENTORY.json`. `SOURCE-SHA256SUMS` covers original files; `EXPORT-SHA256SUMS` additionally covers generated metadata except itself. The adjacent ZIP and `.zip.sha256` cover the complete candidate. Hashes detect changed bytes; they are not signatures or publication approval. Failed writes leave their partial output for inspection and require a new output name on retry.

After export, install locked dependencies and run the retained Go and frontend suites from the separate candidate directory. Build its web assets and portable package without borrowing the original checkout's `node_modules`, generated resources or runtime state. Record commands, outcomes, version, exact source commit and ZIP hash in separate verification evidence. A Git-less binary package correctly has null `sourceCommit`/`sourceDirty`; the candidate's provenance supplies its original revision. Earlier source exports and private-checkout tests do not prove this candidate builds.

Follow [RELEASE.md](RELEASE.md) for the remaining runtime, clean-machine, live-cloud, signing and distribution gates. Public publication and any repository visibility change remain separate authorized actions. This script neither clears historic ownership nor makes a public download available.

The exporter has an isolated fixture suite: `python scripts/test_export_public_source.py`. It exercises exact CRLF grants, executable modes, exclusions/checksums, no-overwrite, dirty/wrong-head refusal, version/hash/credential failures and symlink rejection without exporting the real working tree.
