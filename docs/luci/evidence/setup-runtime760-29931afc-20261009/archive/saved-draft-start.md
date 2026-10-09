Use the existing isolated checkout at /workspace/luci. Read AGENTS.md and preserve current user changes. Read docs/luci/GOAL.md, PRODUCTION-ROADMAP.md and the production handoff before continuing. Inspect `git status --short` and `git log -1` against the repository declaration saved in environment settings; do not reset to the old cockpit branch. A cloud task already has an isolated checkout: do not create a worktree unless requested.

The prepared workflow is the Go control plane/sandbox libraries, React browser client, system Chromium and an inspected Docker runtime. Native Tauri packaging, real provider inference, public publishing, SSH acceptance and cloud provisioning remain separate workflows. Local development and deterministic acceptance need no model/cloud credentials. Reuse the platform's HTTPS Git authentication; never print credential values or fixture tokens.

Tool activation and recovery
----------------------------

For every new shell:

```bash
source /workspace/.luci-tools/activate.sh
cd /workspace/luci
```

Activation selects verified Go 1.26.3, GOTOOLCHAIN=local, readonly module resolution, GOMAXPROCS=4, and an equivalent Go workspace file outside the checkout. The toolchain itself is a real retained directory at `/workspace/.luci-tools/go1.26.3`. Its verified download archive was reclaimed as disposable cache; the tested installer reacquires it with the pinned official checksum if the installed toolchain is missing. Required runtime recovery data is a real regular file at `/workspace/.luci-tools/luci-sandbox-dev.tar.gz`. These essentials must not be symlinks into /tmp when publishing the snapshot.

Activation also sets TMPDIR and GOTMPDIR to the private external `/workspace/.t` directory. This keeps native Unix socket fixture paths below the platform length limit; do not substitute deep temporary paths.

Build, module and npm caches are disposable. Activation recreates `/tmp/luci-go-build-cache`, `/tmp/luci-retained-tools/go-mod`, `/tmp/luci-gopath`, `/tmp/luci-npm-cache-recovery` and `/tmp/luci-playwright-cache`. The historical `retained-tools/go-mod` name is only a warm-cache location; no required toolchain or recovery archive depends on it. Losing /tmp requires module downloads/builds again, not a new toolchain trust decision. Historical binary/archive outputs relocated to /tmp are optional diagnostics, not startup dependencies. A missing known generated `artifacts/dev` link is repaired by activation so development can rebuild it.

Node 24.19.0, Python, Git, GCC, Docker CLI and system Chromium were present and verified in this environment. If the pinned toolchain or helper initialization is missing, run:

```bash
bash /workspace/.luci-tools/install.sh --tools-only
source /workspace/.luci-tools/activate.sh
```

This narrow mode installs/reuses Go, refreshes external helpers/local workspace, and creates cache directories. It does not compile application code, reinstall npm dependencies or restore Docker images. Missing Go is downloaded over verified HTTPS from dl.google.com, checked against the pinned official SHA-256, staged and version-checked before replacement. A mismatched managed installation is retained for diagnosis. No checksum, signature or TLS verification is disabled.

At a stable source revision, run the saved full installer to install locked dependencies and refresh generated outputs:

```bash
bash /workspace/.luci-tools/install.sh --full
```

The full mode runs Go builds, creates `artifacts/onboarding/luci`, runs `npm ci --no-fund --no-audit` and the desktop build, checks a real headless browser page, and verifies/restores the pinned runtime image. It keeps tracked source, manifests and lockfiles unchanged. Do not run this while another agent is editing dependencies or compiling the same outputs. The runtime-only mode below is available independently.

Docker service and retained image
---------------------------------

The privileged daemon is supplied and managed by this cloud environment; this agent has no supported unprivileged command to recreate a root daemon. Its current socket is `/var/run/docker.sock`. Live processes and daemon/container state must not be assumed to survive snapshot restore. Probe the supplied service before running workloads:

```bash
source /workspace/.luci-tools/activate.sh
docker info
bash /workspace/.luci-tools/install.sh --restore-runtime
```

If Docker is unavailable, restore/start the environment-provided service before retrying. Do not claim Docker readiness from an installed CLI alone. The helper fails clearly if the daemon or retained archive is missing; it does not silently choose a different engine or runtime. It checks the archive SHA-256 before `docker load`, and checks the resulting/existing image ID. An intentionally rebuilt image with a different identity needs explicit provenance and updated pins, not a weakened identity check.

Pinned recovery identities:

- Go download archive (reacquired when needed) `go1.26.3.linux-amd64.tar.gz`: SHA-256 `2b2cfc7148493da5e73981bffbf3353af381d5f93e789c82c79aff64962eb556`.
- Sandbox archive `luci-sandbox-dev.tar.gz`: SHA-256 `7caf7aab012cd4a5bf2ace5c8a2e12e617b4afbaef20f6b3db64586a24e9fcf3`.
- `luci/sandbox:dev`: image ID `sha256:333effcf40151e2411fa036b4581113be2af7f8d441bf9566f1cce6e7f208e98`.
- `/workspace/.luci-tools/sandbox-image-provenance.json` records the trusted base, entrypoint/sandboxd checksums, original build history, export/load verification and acceptance scope. It describes the retained image, not every subsequent app source revision.

If both runtime image and recovery archive are lost, the old pin cannot be recovered by inventing a replacement checksum. Rebuild from the reviewed repository Dockerfile through the normal setup flow, with adequate disk and network access, then verify the result and update provenance before retaining a new image. If sandbox sources or its Dockerfile changed, explicitly review whether that rebuild is necessary. The current immutable image is sufficient only for unchanged runtime sources.

The external Docker adapter passes non-build commands through unchanged. For builds it uses existing platform proxies and resolves their DNS dynamically. For the sandbox's npm build step it creates an external temporary Dockerfile mounting the platform CA as a BuildKit secret. It does not change tracked source, copy credentials/CA contents into images, or disable TLS/package verification. Public image pulls need no additional registry credentials here.

Storage and concurrency
-----------------------

The supplied daemon uses vfs. Its initialization and writable copies can require about 4.6 GiB for one 2.29 GiB runtime image. Recheck actual free space immediately before each allocation: reserve at least 6 GiB before the first full sandbox, then budget 4.6 GiB transient usage plus at least 1 GiB remaining free for every later full-sandbox admission. The running hard floor is 1 GiB remaining, not the pre-allocation budget. Run only one full sandbox/check at a time; include any retained deployment in the next admission calculation. A small existing Node preview needs a separate budget (initial estimate512MiB; actual peak remains to be measured). Check both filesystems before costly work:

```bash
df -h /workspace /tmp
docker system df -v
docker buildx du --verbose
```

Docker's logical build-cache size can substantially understate physical vfs usage. During onboarding, known old cache reporting only 2.374 KiB occupied approximately 4.5 GiB physically. Do not use broad cache/resource pruning. Any further reclamation must identify an exact task-owned, immutable, unshared disposable entry and verify that no build depends on it. Preserve unrelated resources, pinned images and all volumes. The installed Go toolchain and required runtime archive stay in /workspace; temporary Go/npm caches are disposable host data. This setup does not prune caches or resources automatically. Interrupted daemon requests can complete after a service exits: drain known pending requests before exact owned-resource cleanup and repeat verification; an instantaneous empty list is not sufficient.

Development startup
-------------------

Start development in a managed terminal session; processes must restart after restore:

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

Keep private state outside projects. The script compiles the current API, sets LUCI_RUNTIME_DIR to the checkout, writes API logs under artifacts/dev, and starts Vite on port 1420. It installs no Docker image unless passed `--install-runtime`. Preserve existing accounts/projects and check for an already-owned process before starting another; stop only task-owned processes. Never print bootstrap tokens.

From another shell, verify local requests: `/v1/control-plane/status` at 127.0.0.1:8730 must return 200 and product Luci; localhost:1420 must return the Vite page; localhost:1420/src/main.tsx must return transformed source containing createRoot. A fresh private state requires bootstrap; existing accounts need not. Keep the API loopback-bound. These are internal checks; onboarding does not support user-facing localhost preview links. Publication, new-task restoration and live authentication are separate from current-instance checks.

Relevant validation
-------------------

Run only the checks needed by the current source change, and preserve real failures and exit status:

```bash
source /workspace/.luci-tools/activate.sh
cd /workspace/luci
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

The external preload uses Playwright's supported executablePath option with the installed Chromium and preserves all assertions. System Chromium 151.0.7922.173 was verified; exact Playwright 1.63.0 Chromium 153.0.8010.12 compatibility was not. The pinned download from cdn.playwright.dev was blocked by network policy. If exact-browser testing is required, add the necessary official download domains in environment settings, run `npx playwright install chromium` in desktop, and omit the preload. Do not replace the known system-browser result with an untested pinned-browser claim.

The service/browser acceptance runners create fresh private fixtures and stop only owned services/resources. Their logs/state may contain generated test credentials; keep them private. See docs/luci/TEAM-ACCEPTANCE.md for actual two-account team, browser handoff and reviewed-run Docker acceptance. `scripts/team-run-smoke.mjs` passed the full real CLI/context/check/release/revocation journey using its local deterministic provider, pinned runtime and a digest-pinned small Node release image. This verifies orchestration/protocol/execution, not model reasoning or paid model access. Existing-image runtime smoke explicitly reports that image installation was not exercised. No cloud resources were provisioned by these checks.


Local-first swarm workflow and latest checkpoint
------------------------------------------------

Read docs/luci/LOCAL-TEST-LOOP.md, QUALITY-WORKLIST.md and LOCAL-WINDOWS-ACCEPTANCE.md before choosing a revision. The user requires a polished interactive local app, four or more concurrent projects, isolated durable context, reviewed integration/recovery and whole-computer responsiveness. The full objective in GOAL.md remains unfinished. The last confirmed platform goal was blocked; no goal control is available in this session. Do not equate an active thread or a source test count with completion.

Latest published implementation checkpoint: abd2d8c9314d6b73d0c202c82de72f3c3acc0d9a on codex/luci-production-runs; inspect current HEAD and preserve later edits. This update does not replace the saved repository membership. Historical complete discovery receipt: /workspace/.luci-tools/onboarding-next/discovery-current-20261008T230338903177Z.json (272 Git roots: one upstream, 271 explicitly classified generated roots, 17 live directory aliases, zero unknowns/errors). This task uses the existing checkout; do not reset, create a worktree, alter origin or include generated private acceptance repositories as selected upstream projects.

Keep product, helper, package, runtime image and provider identities separate. Build only a clean selected source through scripts/build-release.sh, retain prior outputs first, and validate every archive/extracted member against that exact source. The latest packaged revision and exact provenance belong in the retained candidate receipt, not an assumed moving branch. /tmp package extractions and historical archive compatibility links are optional and can vanish on restoration; rebuild from the chosen commit rather than substituting another package or changing trusted image pins. Required Go tools and active/rollback sandbox archives remain regular /workspace files. Runtime archive SHA256 and image ID are listed above. Existing-image checks/export/layer verification passed; importing into a fresh daemon and fresh-task restoration remain unverified.

Installed dependency attempt 11 at product 4df94ab9dad0e9fedd28a873bbadc56720e1402e / helper d6949ac18e4f72fc2dc286627dc5bddc4067e36d passed four real Docker/native Codex workers with scripted responses, six dependency refusals, a retained conflict, replacement/dependent join, three reviewed integrations and an actual whole-build check. Exact cleanup/charge release and explicit evidence joining passed. The build awaits release, not deployment or semantic acceptance. Read docs/luci/evidence/cloud029-installed-4df94ab9-attempt11/README.md. Earlier failed attempts, absent measurements and two sampled Chromium zombies remain separate evidence.

CLOUD-032 responsiveness is still open. The original sustained attempt failed its duration parser and did not finish four-project throughput. Its correction preserves the workload and thresholds. The latest complete standalone idle diagnostic on product/helper 2da1a821 misses the unchanged 300 ms switch p95 target (369.714279 ms); input p95 is 65.910818 ms. The later partial sustained attempt on product 2da/helper abd2d8c9 fails after SDK tool-output truncation, with zero accepted task results and a retained charge. Its loaded switch p95 is 550.770057 ms from only 74 switches. CLOUD-041 tracks that helper transport correction separately from CLOUD-032 responsiveness. No static-control or observer cost is subtracted. New source optimizations require matching post-change and loaded checks. CLOUD-033 corrects confirmed-cleanup text while preserving historical receipts. CLOUD-034 addresses stale cross-process reservation visibility independently of admission. Read each exact source/test receipt rather than inferring installed or Windows behavior.

Recovery helpers: scripts/acceptance/launch-recovery/README.md; task-continuation/README.md; task-dependencies/README.md; check-create-recovery/README.md; worker-recovery/README.md. Sustained workflow: scripts/acceptance/sustained-four-project/README.md. Follow their explicit product/helper/image pins and fresh private outputs. Published scoped Linux witnesses now cover ordinary acknowledged-worker Stop/restart (product d348/helper d52) and complete unknown-Create review/consent/fence recovery (product/helper 2da1a821). The latter target is cancelled/unverified without Start/replay; only its exact charge releases. Original review-before-first-observation ordering, concurrent charged-witness isolation, Windows and live-provider quality remain open. Check the exact published receipts, not a blanket recovery-pass claim. Preserve original charged fixtures and their bytes: empty Docker inventory is not capacity-release proof. Never hand-edit ledgers or reuse a failed authenticated fixture to manufacture a pass.

Use an exclusive runtime/performance window: no source compilers, competing browser suites or second full VFS allocation. Check both physical/cgroup-effective memory and backing disk immediately before Create/Start; sampled physical RAM alone is insufficient. Keep production headroom/admission and unchanged helper six-GiB pre-Create/one-GiB running floors. RAM-backed /tmp caches and archives count toward this environment's memory pressure. Do not reclaim arbitrary resources or stop user applications. The bounded Linux observer samples actual owned processes/cgroup ancestry; its sequential estimates are not atomic admission readings. Preserve observer costs and coverage gaps alongside throughput, fairness, latency and output quality.

The Windows tester owns the user's machine. It pulls immutable revisions into isolated checkouts with private LUCI_STATE_DIR/LUCI_PROJECTS_DIR and a free LUCI_ADDR, preserving existing work and Docker data. Do not launch processes there. Relayed native passes retain their tested revisions and scope; recent heavy local testing is RAM-gated. Report fix handoffs by exact retrievable commit, evidence and remaining checks. Do not declare LOCAL TEST HANDOFF READY while required integrated gates remain open. No provider API key/local endpoint is present in this cloud process; scripted provider mechanics do not establish live-model quality. Existing platform Git auth remains usable without extracting credentials.

An additional verified Rust 1.99.0 compiler, standard library and Cargo are retained under /workspace/.luci-tools/rust-1.99.0 for isolated state-selector module checks. This is optional for the normal Go/browser workflow and is not a Linux Tauri desktop preparation claim. The official manifest is /workspace/.luci-tools/rust-stable-manifest.toml; the external module harness /workspace/.luci-tools/rust-state-check/Cargo.toml reads the actual repository state_dir.rs. If present, activate its bin directory only for that task, use CARGO_HOME=/workspace/.luci-tools/rust-cargo and a disposable CARGO_TARGET_DIR, and run locked/offline with one job after dependencies are available. Native Windows module, complete packaging and owned-sidecar evidence are separately relayed from the local tester. Do not infer native success from Linux Rust tests.

Saving this draft does not run setup, publish a snapshot, apply runtime changes, restore Docker or verify a new task. Preserve the existing network policy and credential requirements unless actual setup needs a supported change.

Terminal fixture and generated-artifact retention notes (9 October 2026)
---------------------------------------------------------------------
Successful recovery attempt 8 and the earlier failed-but-recovered attempt 6 were separately archived, moved to forensic-only locations and protected by read-only tombstones at their original fixture paths before exact inert marker removal. Original acceptance outcomes and raw ledger bytes remain unchanged. See the separate terminal-retirement evidence in docs/luci/evidence. Never resume those archived fixtures or treat marker removal as product automatic cleanup. Other historical failed fixtures with retained charges remain untouched.

Obsolete generated aadc/da8 extracted package copies and standalone acceptance binaries have been byte-verified against retained archives and removed to regain headroom. They are not startup dependencies; do not reference them as executable paths. The current 2da extraction and release archive links under /tmp are optional current-instance artifacts, not guaranteed snapshot-restored tools. Normal startup still builds current source using scripts/dev.sh or the full installer. /tmp memory-backed contents consume cgroup headroom. The current package, idle, recovery and failed sustained receipts describe the existing instance only; saving these instructions is not publication or a fresh-task restoration test.

