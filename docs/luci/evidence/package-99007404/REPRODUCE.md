# Retrieve and reproduce the 990 source build

Exact source: [`99007404a5ddd8592dee376ea00d99c6180d26fe`](https://github.com/lcfrai/luci/commit/99007404a5ddd8592dee376ea00d99c6180d26fe), published on `codex/luci-production-runs`. Use a new isolated checkout. Do not reset an existing user checkout or reuse installed application state. The commands below are instructions, not additional executed checks.

```sh
git clone https://github.com/lcfrai/luci.git luci-99007404
cd luci-99007404
git fetch origin codex/luci-production-runs
git switch --detach 99007404a5ddd8592dee376ea00d99c6180d26fe
git status --short
```

Keep the normal official remote-tracking refs from the actual fetch. Do not synthesize refs to qualify for compact docs. The source revision must be clean; unknown/dirty/unpublished source conservatively retains full documentation.

## Linux build and verifier scope

Use the repository-supported Node 24.19.0, Go 1.26+ and Python environment. Use Python 3.12+ for the frozen verifiers' `hashlib.file_digest` and tar extraction filter APIs; this instruction does not substitute for a toolchain-version receipt. From the clean checkout:

```sh
(cd desktop && npm ci)
GOMAXPROCS=2 GOFLAGS='-mod=readonly -p=1' bash scripts/build-release.sh 0.5.0 linux-amd64
```

The build emits release archives, their checksum file and a source-bound manifest under `artifacts/releases/0.5.0`. Use a fresh checkout/output location so previous candidates and evidence remain intact. A source rebuild is not guaranteed byte-identical to the original archives; inspect its own manifest and checksum instead of rewriting expected values.

The frozen `helpers/verify-local-package.py` is the original cloud-workspace verifier, with explicit `/workspace/luci` and `/tmp/luci-retained-tools/package-0.5.0-<short-revision>` paths. It refuses an existing extraction directory. Reproduce that exact helper only in a **fresh disposable Linux workspace** at those paths, with a new evidence output directory; it is not a general Windows launcher and must not be rerun over preserved receipts or an active package.

Its original invocation shape is:

```sh
python3 /path/to/this/capsule/helpers/verify-local-package.py 99007404a5ddd8592dee376ea00d99c6180d26fe artifacts/quality-swarm/fresh-990-verification/package
```

Once that fresh extraction/provenance exists, the additive helper accepts explicit paths:

```sh
python3 /path/to/this/capsule/helpers/verify-compact-local-package.py \
  --repo /workspace/luci \
  --release /workspace/luci/artifacts/releases/0.5.0 \
  --package /tmp/luci-retained-tools/package-0.5.0-99007404 \
  --commit 99007404a5ddd8592dee376ea00d99c6180d26fe \
  --provenance /workspace/luci/artifacts/quality-swarm/fresh-990-verification/package/package-provenance.json \
  --report /workspace/luci/artifacts/quality-swarm/fresh-990-verification/package/compact-package-integrity.json
```

It reads package/archive contents, performs a temporary guide-stage reproduction and removes only its own temporary stage. It does not start Docker or the application. Its report path must be new. It is Linux-specific (`/tmp` and UID-based cleanup), so it does not establish Windows package verification.

## Windows source build, isolated from installed data

This exact revision has **not** passed native Windows packaging or runtime acceptance. The originating local tester owns that check. Wait for its resource/headroom gate rather than starting a heavy build while normal applications leave insufficient RAM.

Prerequisites are documented in the [pinned release guide](https://github.com/lcfrai/luci/blob/99007404a5ddd8592dee376ea00d99c6180d26fe/docs/luci/RELEASE.md): PowerShell 7, Git, Node 24.19.0/npm, Go 1.26+, Python, and for `-Native` stable Rust/MSVC, Windows SDK and WebView2. Keep checkout, build caches and temporary data on a spacious drive; do not move/prune existing Docker storage or change machine-wide settings as part of this package check.

Use a **new** directory such as `E:\Luci-local-acceptance\checkouts\99007404`. If it exists, select another new directory; do not overwrite it. The local tester can reuse its task-local tool activation and cache setup.

```powershell
git clone https://github.com/lcfrai/luci.git E:\Luci-local-acceptance\checkouts\99007404
Set-Location E:\Luci-local-acceptance\checkouts\99007404
git fetch origin codex/luci-production-runs
git switch --detach 99007404a5ddd8592dee376ea00d99c6180d26fe
git status --short
Push-Location desktop
npm ci --cache E:\Luci-local-acceptance\cache\npm
Pop-Location
pwsh -NoProfile -File .\scripts\build-release.ps1 -Version 0.5.0 -Native
```

This builds packages and an unsigned native installer; it does not instruct installation or application launch. Verify the new manifest/checksums and preserve source/evidence. Later runtime acceptance must use fresh `LUCI_STATE_DIR`, `LUCI_PROJECTS_DIR` and a free `LUCI_ADDR`/launcher port, following the [pinned safe Windows acceptance procedure](https://github.com/lcfrai/luci/blob/99007404a5ddd8592dee376ea00d99c6180d26fe/docs/luci/LOCAL-WINDOWS-ACCEPTANCE.md). Do not bypass the disposable-runner guard in `scripts/test-installed-windows.ps1`, reuse ordinary state, or start providers/workers merely to reproduce this packaging receipt.

Matching idle/sustained results are separate evidence and remain open at package publication. This capsule establishes package verification, not a four-project or local-release pass. The prior failed workloads remain preserved.

The exact build wrapper is also frozen as `helpers/build-cloud046047-candidate.py`. It has task-specific preservation/preflight paths and requires a fresh disposable environment; do not execute it against an active package or a personal checkout. Use the supported release command above for ordinary reproduction.
