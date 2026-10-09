# Retrieve and reproduce the 67a5 source build

Exact source: [`67a5b7c75ca6354d43085f5bb81ec53ee4291f34`](https://github.com/lcfrai/luci/commit/67a5b7c75ca6354d43085f5bb81ec53ee4291f34), published on `codex/luci-production-runs`. Use a new isolated checkout. Do not reset an existing user checkout or reuse installed application state. The commands below are instructions, not additional executed checks.

```sh
git clone https://github.com/lcfrai/luci.git luci-67a5b7c7
cd luci-67a5b7c7
git fetch origin codex/luci-production-runs
git switch --detach 67a5b7c75ca6354d43085f5bb81ec53ee4291f34
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
python3 /path/to/this/capsule/helpers/verify-local-package.py 67a5b7c75ca6354d43085f5bb81ec53ee4291f34 artifacts/quality-swarm/fresh-67a5-verification/package
```

Once that fresh extraction/provenance exists, the additive helper accepts explicit paths:

```sh
python3 /path/to/this/capsule/helpers/verify-compact-local-package.py \
  --repo /workspace/luci \
  --release /workspace/luci/artifacts/releases/0.5.0 \
  --package /tmp/luci-retained-tools/package-0.5.0-67a5b7c7 \
  --commit 67a5b7c75ca6354d43085f5bb81ec53ee4291f34 \
  --provenance /workspace/luci/artifacts/quality-swarm/fresh-67a5-verification/package/package-provenance.json \
  --report /workspace/luci/artifacts/quality-swarm/fresh-67a5-verification/package/compact-package-integrity.json
```

It reads package/archive contents, performs a temporary guide-stage reproduction and removes only its own temporary stage. It does not start Docker or the application. Its report path must be new. It is Linux-specific (`/tmp` and UID-based cleanup), so it does not establish Windows package verification.

## Windows source build, isolated from installed data

This exact revision has **not** passed native Windows packaging or runtime acceptance. The originating local tester owns that check. Wait for its resource/headroom gate rather than starting a heavy build while normal applications leave insufficient RAM.

Prerequisites are documented in the [pinned release guide](https://github.com/lcfrai/luci/blob/67a5b7c75ca6354d43085f5bb81ec53ee4291f34/docs/luci/RELEASE.md): PowerShell 7, Git, Node 24.19.0/npm, Go 1.26+, Python, and for `-Native` stable Rust/MSVC, Windows SDK and WebView2. Keep checkout, build caches and temporary data on a spacious drive; do not move/prune existing Docker storage or change machine-wide settings as part of this package check.

Use a **new** directory such as `E:\Luci-local-acceptance\checkouts\67a5b7c7`. If it exists, select another new directory; do not overwrite it. The local tester can reuse its task-local tool activation and cache setup.

```powershell
git clone https://github.com/lcfrai/luci.git E:\Luci-local-acceptance\checkouts\67a5b7c7
Set-Location E:\Luci-local-acceptance\checkouts\67a5b7c7
git fetch origin codex/luci-production-runs
git switch --detach 67a5b7c75ca6354d43085f5bb81ec53ee4291f34
git status --short
Push-Location desktop
npm ci --cache E:\Luci-local-acceptance\cache\npm
Pop-Location
pwsh -NoProfile -File .\scripts\build-release.ps1 -Version 0.5.0 -Native
```

This builds packages and an unsigned native installer; it does not instruct installation or application launch. Verify the new manifest/checksums and preserve source/evidence. Later runtime acceptance must use fresh `LUCI_STATE_DIR`, `LUCI_PROJECTS_DIR` and a free `LUCI_ADDR`/launcher port, following the [pinned safe Windows acceptance procedure](https://github.com/lcfrai/luci/blob/67a5b7c75ca6354d43085f5bb81ec53ee4291f34/docs/luci/LOCAL-WINDOWS-ACCEPTANCE.md). Do not bypass the disposable-runner guard in `scripts/test-installed-windows.ps1`, reuse ordinary state, or start providers/workers merely to reproduce this packaging receipt.

Matching 67a5 idle/sustained results remain open at package publication. The previous installed sustained attempt used product 99007404 and helper b81d6454, so it is not this package's runtime verification. This capsule establishes package verification, not a four-project or local-release pass. All prior failed workloads and their cleanup limits remain preserved. No setup/configuration change is required by CLOUD-048/049/050/051; Windows acceptance must still use fresh isolated directories and wait for its established resource gates.

The exact build wrapper is also frozen as `helpers/build-cloud048051-candidate.py`. It has task-specific preservation paths and clean-source/remote-ref preconditions and requires a fresh disposable environment; do not execute it against an active package or a personal checkout. Use the supported release command above for ordinary reproduction.
