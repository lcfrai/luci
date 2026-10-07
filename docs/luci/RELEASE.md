# Building and distributing Luci

For current source/CI status use [the 7 October handoff](HANDOFF-2026-10-07.md).
The latest completed 0.4.18 CI run failed Go tests before downstream acceptance;
the new driver changes have syntax checks, not completed installed/runtime runs.
Installed and package observations below retain their dated source identities.
This push does not publish a release or replace an installation.

The primary installed workspace remains **0.4.16**, from clean source
`eda1d061d679e4051e720b7ff98f67a760c270dc`. The secondary workspace admitted
**0.4.17**, from clean source `e0e51f640efc6aed54b1ce84d742b56d83b68215`,
with its matching 917-file bundle and three passing PostgreSQL retention checks.
Accounts, existing projects, history and volumes were preserved. These are
retention outcomes, not new model or database execution outcomes. Docker's VM
subsequently recorded disk I/O errors and a read-only filesystem after C: ran
out of space. Current runtime health and the interrupted sandbox-image rebuild
are unknown; the earlier ready observations are historical.

The matching 0.4.17 Linux, Docker and source artifacts were built in a fresh
independent F: checkout with locked dependencies and isolated caches.
Materialization verifies 906 files, and the source audit verifies 987 selected
Git blobs and all 993 exported files. The Linux package retains 344 licence
components with zero
missing grants. Earlier packages, manifests and failed receipts retain their
original source bindings.

The preceding 0.4.13 task-draft, native design-prototype and readiness journeys
pass, with separate durable before/after preservation confirmation. Earlier
drawing, workspace, archive, email, data, Functions, branch and recovery results
retain their own source bindings. See the
[full product checkpoint](FULL-PRODUCT.md) for those outcomes and their limits.

The 0.4.17 source correction includes shared model connections across guided Define,
repository analysis and Develop, a configurable optional LiteLLM gateway, and
Codex's Docker coding permissions. The requested GPT-6 Astra audit proved that
the default Codex inner workspace sandbox cannot create its namespace under the
existing restricted Docker boundary. A controlled provider fixture proves the
supported Docker-only correction writes actual application files and serves
HTTP 200; this is not live subscription or gateway acceptance. The correction
retains Codex approval policy and the outer non-root Docker restrictions.
Native click-driver correction also has focused tests and a browser scroll
fixture; its actual installed WebView2 outcome awaits a fresh candidate run.

The source batch passes 1,311 frontend checks in 81 files, with a later focused
40-check Shell picker/defaults run and TypeScript validation. The schema-2
preference migration preserves existing layouts, settings and connection
inheritance. Five packaging checks pass. Local Go failures and their focused
corrections remain recorded. Matching 0.4.17 CI run
[`37398697512`](https://github.com/lcfrai/luci/actions/runs/37398697512) passes
whole Go, race, frontend, TypeScript, web build and connected browser checks.
Its runtime tests pass, but runtime evidence upload fails on GitHub artifact
storage quota; that job remains failed and native/portable jobs are skipped.

The current source correction is **0.4.18**. It adds reviewed implementation
context, private account-scoped review authority, actual preview ownership and
health observation, remote connection forwarding, and independent optional
prototype loading. Its focused source results do not establish installed
acceptance. A matching clean build, full CI and installed continuation remain
required. Successful Windows builds now remove only their verified generated
stage; failed stages remain available for diagnosis.

This is not a public production release. The required real Codex-generated
application and public deployment, live AWS/Azure and domain acceptance,
external email delivery, supported native installer lifecycle and public
download acceptance remain open. Candidate CI
[`37389074732`](https://github.com/lcfrai/luci/actions/runs/37389074732)
passes whole Go, expanded Linux race coverage, 1,278 frontend tests in 80 files,
TypeScript, the web build, connected browser acceptance, actual Docker/SSH runtime
acceptance and Linux/macOS portable startup and persistence. Windows portable and
unsigned NSIS builds, compiled-service acceptance and state/upgrade checks pass.
The actual installed native journey fails after six checks, before its first
saved brief. Later design, Git, restart, reinstall and uninstall checkpoints are
unverified. Metadata and the remote merge tree bind the candidate revision; the
frozen observer did not capture each job's actual checkout log. This limit is
retained separately from build success. The earlier 0.4.13 Windows native and
0.4.15 browser failures remain retained.
Earlier runner
assignment failures remain separately preserved. [lcfr.ai/luci](https://lcfr.ai/luci) does not yet
offer public source or package downloads.

The 0.4.3 development candidate adds a retained per-user Codex subscription connection, public static GitHub Pages deployment and storage-capable isolated previews. Its stricter acceptance goal is a real Docker/Codex subscription journey through Define, Design, Develop and Deploy ending in an independently checked public site. `scripts/subscription-journey-eval.mjs` retains staged evidence and cannot mark that journey passed while authentication, Docker or public deployment is missing. The previous 0.4.2 fixture/native acceptance does not satisfy this new live-model gate, and the retained 0.4.2 download is not rebuilt or relabelled by these source changes.

Luci 0.4.0 packages integrate the solution workflow and managed backing: local account bootstrap, roles and private user workspaces, saved plans, multiple model providers, command checks, Git delivery, project backups, private runtime configuration and deployment maintenance. The Admin workspace can manage the selected cloud backing with that backing's own authenticated account and permissions. CI produces portable downloads and the native Windows installer. A successful build proves the checks it ran; live AWS/Azure rollout still needs an account and a domain. Publication URLs and signing are separate distribution configuration.

Luci's own code uses MIT. Include the root `LICENSE` and `NOTICE` with distributions; upstream components and bundled fonts retain their original grants. In particular, Luci's MIT choice does not replace dependencies' Apache-2.0 or other licence files.

## Candidate packages

| Package | Interface | Launch |
| --- | --- | --- |
| `luci-windows-amd64.zip` | Local browser | Extract completely, then run `Start-Luci.cmd` |
| `luci-linux-amd64.tar.gz` | Local browser | Extract completely, then run `./start-luci.sh` |
| `luci-darwin-arm64.tar.gz` | Local browser on Apple silicon | Extract completely, then run `./start-luci.sh` |
| Windows `*-setup.exe` | Native Tauri desktop | Install, then launch Luci |

Portable packages contain the compiled local service, built web assets including local design fonts, launch scripts, sandbox Docker build sources and dependency licence evidence. `cloud/luci-cloud-linux-amd64.tar.gz` carries the matching Linux service, web assets, runtime sources and licences for the explicitly requested cloud workspace install. It is created before insertion into desktop packages, so it does not contain a recursive copy of itself. Packages do not require Go or Node to run. Git is needed for projects; Docker is needed for agent sessions and container previews. Setup reports missing dependencies. Provider CLIs and SSH are needed only for the optional cloud journey. The sandbox build downloads its base images, OS packages and agent CLI; those external components are not an offline, bundled runtime.

The local service listens on loopback port 8730. Close the portable launch terminal to stop it. Keep the complete extracted folder together. Project files and Luci state remain outside the application package; replacing an extracted application folder is not a project backup. Back up both before testing upgrades.

## Downloading a CI candidate

When this workflow has run in the repository that will distribute Luci, open **Actions → Luci build and release candidates**, select the desired successful commit and download its artifact. Artifact access follows the repository's permissions and requires GitHub sign-in; artifacts expire after 14 days. There is no public lcfr.ai download implied by this document.

Unpack GitHub's artifact wrapper first. Verify the inner archive or installer against its accompanying `SHA256SUMS`. For example, use `sha256sum -c SHA256SUMS` on Linux, `shasum -a 256 -c SHA256SUMS` on macOS, or compare `(Get-FileHash -Algorithm SHA256 -LiteralPath <download>).Hash` on Windows. Checksums detect changed bytes; they do not establish publisher identity. Current Windows candidates are unsigned and are intended for review.

The accompanying `release-manifest.json` records the version, artifact hashes and sizes, licence component counts, and source state captured before building. `sourceCommit` is the full Git commit and `sourceDirty` reports tracked or untracked changes. Both are `null` when building an exported source folder without its own Git metadata. A dirty build contains local changes beyond the named commit; use a clean checkout for a candidate intended to identify an exact reviewed revision.

## Rebuilding locally

Use Go 1.26.3, Node 24.19.0, Python 3.11 or newer, and the committed lockfiles. Install desktop dependencies with `npm ci` from `desktop/`. The scripts build the desktop assets themselves.

On Windows:

```powershell
./scripts/build-release.ps1 -Version 0.4.18
# Also build the native Windows NSIS installer:
./scripts/build-release.ps1 -Version 0.4.18 -Native
```

Native Windows builds also need stable Rust with the MSVC target and the [Tauri Windows prerequisites](https://v2.tauri.app/start/prerequisites/). Tauri receives the Cargo lock flag after its argument separator: `npm run tauri -- build --bundles nsis -- --locked`. The [NSIS distribution documentation](https://v2.tauri.app/distribute/windows-installer/) describes the installer and signing options.

On Linux or macOS:

```bash
bash scripts/build-release.sh 0.4.18 linux-amd64
bash scripts/build-release.sh 0.4.18 darwin-arm64
```

Archives and their checksums are written under `artifacts/releases/0.4.18/`. The native installer is under `desktop/src-tauri/target/release/bundle/nsis/`. Cross-compiling an archive does not execute it; CI separately starts Linux and macOS packages on their respective runners. Native macOS and Linux installers are not provided by this workflow.

`scripts/collect-licenses.py` inventories installed npm runtime packages, each bundled Excalidraw font family, the Go modules compiled into the service and sandbox, and the Go standard library. Native builds also inventory reachable Windows Cargo packages. It retains original files and source snapshot references with SHA-256 hashes in `licenses/manifest.json`; `MISSING-LICENSES.txt` lists missing grants. The default command fails when a component has no retained licence file. `--allow-missing` is diagnostic only and must not be used to make a distributable package pass. Updating a dependency can require a new version-specific snapshot in `third-party/licenses/manifest.json`.

The inventory is deliberately broader than the final JavaScript bundle. Native candidates retain the Rust library notices and unmodified source archives for the included MPL-2.0 Rust crates, identified in the manifest. It is evidence for review, not a legal conclusion. Docker, provider CLIs, browsers/WebView, base images and a separately installed Coolify server have their own distribution and service terms; the client package does not grant rights to them. The root `LICENSE`, `NOTICE` and original dependency grants must accompany redistributed packages.

## Local application outcomes

The 0.4 integration batch passed 959 desktop tests and TypeScript validation. The production browser eval completed thirteen outcomes with no JavaScript or CSP errors: authenticated setup, owner policy/member creation, saved brief/design/plan, an explicitly approved host diagnostic, a Git commit, a real ZIP backup and restore, deployment controls, and member project isolation. Private application configuration is stored outside project files and backups; API responses and release history contain variable names and configuration revisions, not values. Preview and cloud deployment receive private values at runtime. Current cloud data integration permits explicitly selected public Vite/Next.js client aliases at build time; it does not embed private configuration or administrative keys. The compiled-service eval separately proved seven management/project outcomes, including actual child supervision, password/account revocation and malicious Git worktree configuration confinement. These evals use private disposable state and do not make cloud or paid-model calls.

## Checks and remaining acceptance gates

The workflow runs both Go module test suites and desktop tests, type checking and asset builds. Its Linux runtime job builds the actual sandbox image and runs the service as host UID 10123, different from the image's default UID 1001. A real shell WebSocket reads the brief, writes project files, and commits through Git. A separate conversation-bearing session starts without credentials or a prompt; a local probe running as the mapped user verifies writes to its API-created transcript mount without model inference. The job checks private directory modes, host file ownership, zero effective and bounding capabilities of the running sandbox, and `NoNewPrivs`. It then builds a preview from the committed Dockerfile, verifies HTTP health and provenance, and stops the owned container. The retained `luci-real-runtime-evidence-<commit>` artifact contains the result and process/shell logs.

The verify job also runs the race detector on identity, project and check/deployment paths. The runtime job exercises the compiled multi-user control plane before Docker. Real Codex/OpenCode CLIs use a local deterministic API with container networking disabled; completion, tool execution and durable fork outcomes are recorded separately from real-model quality. After the runtime job passes, the workflow creates portable archives and exercises authenticated startup, persistence and isolated member workspaces on Linux/macOS. Windows CI builds the native NSIS candidate and runs the portable and multi-user acceptance scripts. These checks do not perform cloud account login, create billable resources, use model credits or certify a clean-machine installer journey. A workflow definition is not an execution result; inspect the successful run and its evidence at the candidate's exact commit.

Before calling a candidate a public production release, retain evidence for these gates at the exact release commit:

1. Install and uninstall the Windows candidate on a clean supported machine. Confirm service startup, clean shutdown, first-run dependency remedies and project/state survival through an upgrade. Exercise Linux and Apple-silicon portable launchers on clean hosts too.
2. Create a project, save its brief and design, restart Luci and recover both. Run a real Docker-backed agent session with the user's chosen model access, inspect its Git changes, and create a local preview that passes a real health check.
3. In explicitly authorized test accounts, finish the complete AWS Lightsail and Azure VM journeys separately: provider login, cost acknowledgement, creation, trusted SSH fingerprint, tunnel, bundled cloud runtime installation, activation, remote project/session/preview journey, restart/reconnect, Coolify setup/token, DNS, reachable Git commit deployment, HTTPS health and subsequent deployment. Verify that remote previews use a separate browser origin and support root-relative assets and WebSocket traffic. Also prove failed provisioning/deployment recovery and cleanup. Verify resources are actually removed in the provider console. Local fixtures are not evidence of these live outcomes.
4. Verify a domain/TLS failure never becomes a ready release; a release remains pinned to its brief revision and exact Git commit. Check that exported artifacts and logs contain no credentials. Test backups and restoration for any application data that the intended release supports.
5. Review licence inventory completeness, source attribution, retained-code ownership, dependency advisories and distribution permissions. The historical private repository and its history require review before any public-source publication.
6. Set up publisher signing and the intended public source/download location. Native macOS distribution additionally needs its own build, signing and notarization path. Automatic updates are not configured; publish upgrade and rollback instructions for the release actually shipped.

Recorded CI evidence: [run 36953123907](https://github.com/lcfrai/luci/actions/runs/36953123907) passed at source commit `4c117f6a088189f640ad946459f36c9072d344c1`. This includes the real Linux Docker journey, Linux and macOS package startup checks, strict licence inventories and the Windows native installer build. The Linux/macOS manifests identify that exact commit with `sourceDirty: false`; their inventories contain 317 and 318 components respectively, with zero missing licence grants. These results describe that source snapshot, not later package builds or a signed public release.

Current verification limits: Docker Desktop is available on the development host, and the Docker candidate has real installation/runtime receipts. Installed data, Functions and native confirmation/recovery outcomes have scoped evidence in the full product checkpoint; they do not prove external email deliverability or a subscription-generated application. The required implementation/public deployment remains pending. Earlier CI evidence belongs to its recorded source revision. The provider CLI fixture can prove protocol, real tool execution and state isolation without paid inference. Actual model quality, subscription login and provider acceptance still require the user's chosen account/model. A built Windows installer has not by itself passed a clean-host install/uninstall evaluation. No live AWS/Azure account, billable resource, real domain/TLS rollout or complete remote installation is implied by local provider/process adapters and HTTP/WebSocket fixtures.

## Publishing after those gates

Choose the reviewed release commit, align the version in the desktop package/lockfile, Tauri config, Cargo package/lockfile and workflow, and rebuild from that commit. Record the tested platforms, known limitations, source revision, hashes, dependency notices and installation instructions in the release notes. Any public source export must preserve required notices while excluding private history and secrets according to the ownership review.

Create a release in the approved public repository or approved download host only after publication is authorized. Upload the exact reviewed and signed assets and their final checksums, along with the corresponding source and notices. Then configure the lcfr.ai download surface to point to those real versioned assets, test the links as a signed-out visitor and test installation from those downloads. This workflow has no publication step and does not supply a placeholder download URL.
