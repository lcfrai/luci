# Building and distributing Luci

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

Use Go 1.26.3, Node 22.18.0, Python 3.11 or newer, and the committed lockfiles. Install desktop dependencies with `npm ci` from `desktop/`. The scripts build the desktop assets themselves.

On Windows:

```powershell
./scripts/build-release.ps1 -Version 0.4.0
# Also build the native Windows NSIS installer:
./scripts/build-release.ps1 -Version 0.4.0 -Native
```

Native Windows builds also need stable Rust with the MSVC target and the [Tauri Windows prerequisites](https://v2.tauri.app/start/prerequisites/). Tauri receives the Cargo lock flag after its argument separator: `npm run tauri -- build --bundles nsis -- --locked`. The [NSIS distribution documentation](https://v2.tauri.app/distribute/windows-installer/) describes the installer and signing options.

On Linux or macOS:

```bash
bash scripts/build-release.sh 0.4.0 linux-amd64
bash scripts/build-release.sh 0.4.0 darwin-arm64
```

Archives and their checksums are written under `artifacts/releases/0.4.0/`. The native installer is under `desktop/src-tauri/target/release/bundle/nsis/`. Cross-compiling an archive does not execute it; CI separately starts Linux and macOS packages on their respective runners. Native macOS and Linux installers are not provided by this workflow.

`scripts/collect-licenses.py` inventories installed npm runtime packages, each bundled Excalidraw font family, the Go modules compiled into the service and sandbox, and the Go standard library. Native builds also inventory reachable Windows Cargo packages. It retains original files and source snapshot references with SHA-256 hashes in `licenses/manifest.json`; `MISSING-LICENSES.txt` lists missing grants. The default command fails when a component has no retained licence file. `--allow-missing` is diagnostic only and must not be used to make a distributable package pass. Updating a dependency can require a new version-specific snapshot in `third-party/licenses/manifest.json`.

The inventory is deliberately broader than the final JavaScript bundle. Native candidates retain the Rust library notices and unmodified source archives for the included MPL-2.0 Rust crates, identified in the manifest. It is evidence for review, not a legal conclusion. Docker, provider CLIs, browsers/WebView, base images and a separately installed Coolify server have their own distribution and service terms; the client package does not grant rights to them. The root `LICENSE`, `NOTICE` and original dependency grants must accompany redistributed packages.

## Local application outcomes

The 0.4 integration batch passed 959 desktop tests and TypeScript validation. The production browser eval completed thirteen outcomes with no JavaScript or CSP errors: authenticated setup, owner policy/member creation, saved brief/design/plan, an explicitly approved host diagnostic, a Git commit, a real ZIP backup and restore, deployment controls, and member project isolation. Private application configuration is stored outside project files and backups; API responses and release history contain variable names and configuration revisions, not values. Preview and cloud deployment receive values at runtime, with cloud build-time injection explicitly disabled. The compiled-service eval separately proved seven management/project outcomes, including actual child supervision, password/account revocation and malicious Git worktree configuration confinement. These evals use private disposable state and do not make cloud or paid-model calls.

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

Current verification limits: the development host's Docker engine remains unavailable; the real Docker evidence came from CI. The provider CLI fixture can prove protocol, real tool execution and state isolation without paid inference. Actual model quality, subscription login and provider acceptance still require the user’s chosen account/model. A built Windows installer has not by itself passed a clean-host install/uninstall evaluation. No live AWS/Azure account, billable resource, real domain/TLS rollout or complete remote installation is implied by local provider/process adapters and HTTP/WebSocket fixtures.

## Publishing after those gates

Choose the reviewed release commit, align the version in the desktop package/lockfile, Tauri config, Cargo package/lockfile and workflow, and rebuild from that commit. Record the tested platforms, known limitations, source revision, hashes, dependency notices and installation instructions in the release notes. Any public source export must preserve required notices while excluding private history and secrets according to the ownership review.

Create a release in the approved public repository or approved download host only after publication is authorized. Upload the exact reviewed and signed assets and their final checksums, along with the corresponding source and notices. Then configure the lcfr.ai download surface to point to those real versioned assets, test the links as a signed-out visitor and test installation from those downloads. This workflow has no publication step and does not supply a placeholder download URL.
