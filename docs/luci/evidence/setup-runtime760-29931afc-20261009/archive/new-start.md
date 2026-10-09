Use the existing isolated checkout `/workspace/luci`, remote `github.com/lcfrai/luci`. Read `AGENTS.md`, `docs/luci/GOAL.md`, `PRODUCTION-ROADMAP.md`, `LOCAL-TEST-LOOP.md`, `QUALITY-WORKLIST.md` and the current production/local handoffs. Check `git status --short` and `git log -1` against saved repository membership before work; preserve later/user edits. Do not reset to a historical branch, alter origin, select private generated fixture repositories, or create a worktree unless the user asks. Prepared source at this update: 29931afc5b31ae9b9bd7b8973a7e3fdb840609a3; the current compiled package remains exact29931afc5b31ae9b9bd7b8973a7e3fdb840609a3. Product, helper, package, runtime and provider identities remain separate.

Required tools and activation
-----------------------------

For each shell:

```bash
source /workspace/.luci-tools/activate.sh
cd /workspace/luci
```

Activation selects Go1.26.3, GOTOOLCHAIN=local, readonly module resolution, GOMAXPROCS=4 and an equivalent Go workspace/checksum file outside Git. Go is a real retained directory at `/workspace/.luci-tools/go1.26.3`. Its optional download archive can be reacquired by the installer from the official HTTPS URL and pinned SHA256`2b2cfc7148493da5e73981bffbf3353af381d5f93e789c82c79aff64962eb556`; no signature/checksum/TLS checks are disabled. A mismatched installation is preserved before replacement.

`TMPDIR` and `GOTMPDIR` use private external `/workspace/.t` to respect native Unix socket limits; do not substitute a deep temporary prefix. Go/module/npm/browser caches are disposable and recreated under `/tmp` by activation. `/tmp/luci-retained-tools/go-mod` is only a cache despite its name; required Go/runtime bytes do not depend on TMP. Historical package/archive compatibility links and optional Rust1.99/PowerShell tools may target TMP and need restoration/reinstallation if absent. Never treat them as required snapshot contents. A known missing generated `artifacts/dev` link is repaired by activation so development can rebuild.

Node24.19, Python, Git, GCC, Docker CLI and system Chromium were available in the current instance; recheck capabilities after restoration. Reuse existing platform HTTPS Git authentication. Inspect only credential names/presence, never values, logs/tokens or private environment contents. No new model/cloud credential is required for deterministic source/browser fixtures; real-provider quality requires an explicitly connected provider and remains separate.

Installer modes:

```bash
bash /workspace/.luci-tools/install.sh --tools-only
source /workspace/.luci-tools/activate.sh
# At stable source, without another compiler/output writer:
bash /workspace/.luci-tools/install.sh --full
# Independently restore/verify runtime, once the supplied daemon is healthy:
bash /workspace/.luci-tools/install.sh --restore-runtime
```

Tools-only refreshes pinned Go, external helpers/workspace and cache directories without compiling, npm installation or Docker restoration. Full mode builds Go packages/API, runs locked `npm ci --no-fund --no-audit`, desktop build and a real headless browser smoke, then verifies/restores runtime. Keep source/manifests/lockfiles unchanged. Preserve actual failures and exit status; avoid repeated unchanged suites. The saved installer does not prune resources automatically.

Docker/runtime recovery
-----------------------

The privileged Docker daemon is supplied by the environment. The local endpoint is `unix:///var/run/docker.sock`; live daemon/auth/process state is not a snapshot guarantee. Confirm the intended daemon before workloads. Do not silently use an inherited Docker context/TLS endpoint or recreate/change daemon storage. If unavailable, restore/start the supplied service before retrying; CLI presence alone is not readiness.

The required runtime archive is the real retained regular file `/workspace/.luci-tools/luci-sandbox-dev.tar.gz`, SHA256`637d1d02c701be5f4327bb76dcb58335e1665b27945e3f1cb08bb825c5fd96fd`. It restores `luci/sandbox:dev` with exact image`sha256:76043c5d0f30207c05343e1dbc0f450658eb52726590c8c47c714c67371acec8`. `/workspace/.luci-tools/sandbox-image-provenance.json` binds its verified config/layers/runtime files, websocket1.8.15, trusted base, candidate build and original scoped runtime receipts. Archive verification does not establish Docker load into a fresh daemon, snapshot restoration, full product acceptance or live-model quality. Check the provenance's actual flags and retained operation receipt before making those claims.

The installer checks the archive checksum before `docker load` and the existing/restored image ID. It refuses a mismatched existing tag; update an intentionally changed image only with exact source/config/layer evidence and revised pins. Never invent a replacement checksum. If both image and archive are lost, rebuild the reviewed Dockerfile with adequate resources/network, verify the result, and update provenance before retaining a new archive. Exact760 is reusable only while all106 runtime inputs match its source-bound inventory; new runtime bytes require a matching rebuild.

The external build adapter uses the environment's existing proxy and public CA through an ephemeral BuildKit secret for npm, without copying credentials into images or changing tracked Dockerfiles/TLS verification. Non-build CLI behavior follows the configured local daemon. Public images normally need no new registry credentials; preserve existing network/credential settings and add only demonstrated missing requirements. The pinned Playwright browser download was previously blocked; current system-browser compatibility is separate from exact pinned-browser acceptance.

Storage, ownership and concurrency
---------------------------------

Use one exclusive full-container or performance window: no competing source compilers, browser suites or second full vfs allocation. Check root and TMP immediately before costly work. The exact760 full-sandbox pre-Create disk floor is6,736,759,858 bytes, and all production memory/CPU/authority/accounting gates still apply. The earlier general six-GiB helper floor and one-GiB running floor do not lower a stronger current exact allocation requirement. Full vfs copies can consume several GiB. A later container/check or retained deployment needs its own complete admission calculation.

Archive export is distinct: both filesystems require3,905,250,841 bytes before export because daemon temporary placement is unknown, plus separately bounded compressed-output space on its destination. Reclaim only exact task-owned disposable caches or byte-verified generated duplicates in a reviewed operation, preserving provenance. Docker logical cache sizes can understate physical vfs usage; do not use broad prune/reset/unregister/delete operations, drop volumes, stop unrelated applications or hand-edit ledgers. Required Go and active760 archive stay in workspace. Historical333/425 archives may be moved to TMP with preserved bytes/aliases/receipts; that does not guarantee their snapshot retention and never deletes a failed fixture.

```bash
df -h /workspace /tmp
docker system df -v
docker buildx du --verbose
```

An instantaneous empty Docker list is not cleanup or capacity-release proof. Interrupted requests can complete late: drain/fence exact owned operations, preserve unknown outcomes, verify exact allocation identity, confirm reservation release through supported product recovery, and recheck inventories. Preserve every failed authenticated fixture, original bytes/ledger/receipts and retained charge. Never reuse it or manually change accounting to manufacture a pass. Historical forensic archives/tombstones and operator marker retirement are separate from product automatic cleanup.

Development startup and validation
----------------------------------

Start only owned processes in a managed terminal; services must restart after restore:

```bash
source /workspace/.luci-tools/activate.sh
cd /workspace/luci
umask 077
mkdir -p /workspace/.luci-dev/state /workspace/.luci-dev/projects
export LUCI_ADDR=127.0.0.1:8730
export LUCI_STATE_DIR=/workspace/.luci-dev/state
export LUCI_PROJECTS_DIR=/workspace/.luci-dev/projects
export LUCI_BACKEND=docker
bash scripts/dev.sh
```

Keep private state outside projects; inspect existing owned listeners/account state first. The script builds the API, sets LUCI_RUNTIME_DIR, records private logs under artifacts/dev and starts Vite1420. It installs no image unless explicitly passed `--install-runtime`. Preserve existing projects/accounts, never print bootstrap tokens, and stop only processes/resources you own. Internally verify API `/v1/control-plane/status` returns200/productLuci, Vite1420 serves the page, and `/src/main.tsx` serves transformed createRoot source. Fresh state needs bootstrap; existing state may not. Do not publish localhost preview links.

At a stable revision, choose affected checks:

```bash
go test ./control-plane/... ./sandboxd/...
go test -race ./control-plane/internal/management/... ./control-plane/internal/product/... ./control-plane/internal/project/...
cd desktop
npm run typecheck
npm test
npm run build
cd ..
mkdir -p artifacts/onboarding
CGO_ENABLED=0 go build -trimpath -o artifacts/onboarding/luci ./control-plane/cmd/api
node scripts/control-plane-smoke.mjs --binary artifacts/onboarding/luci --output artifacts/onboarding/control-plane-smoke.json
NODE_OPTIONS="${NODE_OPTIONS:+$NODE_OPTIONS }--import=/workspace/.luci-tools/use-system-chromium.mjs" node scripts/run-ui-uat.mjs artifacts/onboarding/luci desktop/dist .
```

The preload uses supported Playwright executablePath with system Chromium and preserves assertions. Historical systemChromium151/Playwright1.63 results do not certify current exact-browser parity. If exact downloaded-browser acceptance is needed, use approved official domains and `npx playwright install chromium`, then omit the preload. Tauri/nativeWindows, real provider, public publishing, SSH and cloud provisioning are separate workflows.

Current acceptance and Windows loop
----------------------------------

The goal remains a dependable local app with four or more concurrent projects, isolated durable context, reviewed integration/recovery and whole-computer responsiveness. Read current `QUALITY-WORKLIST.md`, `LOCAL-TEST-LOOP.md`, `LOCAL-WINDOWS-ACCEPTANCE.md` and exact receipts in `docs/luci/evidence`; old source/test counts and historical checkpoints are not current acceptance. CLOUD-032 stays open while unchanged switchp95≤300ms/inputp95≤100ms and full loaded/settled quotas fail or remain incomplete. Keep every timing row, resource coverage gap, observer cost and failed attempt; do not subtract instrumentation to claim a pass or treat zero vertical displacement as latency evidence.

Current package299 and runtime760 support separately scoped terminal/native-close improvements; overall warm/cold/restart and retained-charge outcomes must follow their actual final receipts, not this setup text. Linux scripted Codex/Docker dependency/recovery witnesses establish protocol/orchestration scope only. Historical c440,4df,d348,2da and sustained results remain at their original products/helpers/images. Review-before-first-observation, concurrent charged witnesses, refusal branches, Windows, real-provider quality and sustained performance cannot be inferred from unrelated source/unit/archive checks.

The Windows tester alone owns the user's machine: immutable revision changes only between batches, isolated checkout, fresh LUCI_STATE_DIR/LUCI_PROJECTS_DIR/freeLUCI_ADDR, preserved user checkout/state and unrelated Docker workloads. Heavy checks are gated by actual physical/commit headroom; no cloud agent should launch there. Relayed native passes keep their original revision/scope. Publish fix handoffs promptly with exact retrievable source, matching package/sidecar/runtime identity, evidence, startup/config deltas and remaining checks. Do not announce full LOCAL TEST HANDOFF READY while integrated required gates are open.

Acceptance helpers under `scripts/acceptance/{launch-recovery,task-continuation,task-dependencies,check-create-recovery,worker-recovery,sustained-four-project}` require their own exact pins/ownership contracts; use fresh private outputs and preserve secrets. Optional Rust state-selector checks require their verified compiler/manifest, locked module harness, task Cargo cache and disposable target with one job; they are not LinuxTauri or Windows-native proof.

Saving fields does not execute setup, publish a snapshot, apply runtime changes or verify another task. The platform owns publication/restoration; preserve existing network/environment/secret settings. Claims about service readiness, model quality, performance, cleanup and new-task recovery require their own actual evidence.
