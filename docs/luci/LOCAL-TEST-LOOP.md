# Windows testing and correction loop

The owner explicitly authorizes continuous two-way coordination between this
cloud implementation task and local thread
`01a1199f-7500-7843-b02b-081081e536fa` on host `local`. The local thread tests
Windows; the cloud thread implements and validates corrections. Neither waits
unnecessarily for the other to finish a whole suite.

## Handoff and pinned revisions

Send the initial **LOCAL TEST HANDOFF READY** only when an integrated candidate
is retrievable and available checks have passed or have precise limitations.
Include branch/full commit or artifact, retrieval commands, Windows prerequisites,
startup commands, configuration names without secret values, evidence, remaining
cases and the four-project functional/resource sequence. Use an available thread
messaging tool; if unavailable, post the labelled update in the cloud thread for
the local thread's periodic check. Do not ask the owner to relay messages.

The local tester uses an isolated checkout and state/projects directories. The
existing repository at `C:\Users\DylanMackie\Documents\ChatGPT\Luci\luci`, its
outer workspace and application data are preserved. Its reported starting point
is `802be34d7fa8a7e44fc922583f90a98847a52df8` on
`codex/luci-subscription-e2e`. C: has about 4.8 GiB free; E: and F: have about
1 TiB each. Put acceptance source, caches and outputs on a spacious volume and
verify the Docker engine's actual data volume as well. These are preparation
observations to recheck, not current free-space guarantees. Prepare a supported
Node runtime: the reported PATH version is22.18.0, while the tested version is
24.19.0. Never bypass the disposable-runner restriction in
`scripts/test-installed-windows.ps1`; follow the safe local development route in
[LOCAL-WINDOWS-ACCEPTANCE.md](LOCAL-WINDOWS-ACCEPTANCE.md).

Change revisions only between test batches. Record the full revision with every
result; never replace code underneath a running test. Keep failed and successful
evidence and remaining coverage intact.

### Prepared Windows baseline — 8 October 2026

The local tester has retrieved and independently verified
`e81b586077f8af97e5e59472e34eafd161f16b8a` into
`E:\Luci-local-acceptance\checkouts\e81b5860`. `npm ci` passed with task-local
Node 24.19.0 and npm 10.9.3; Go 1.26.3 is available. Caches and temporary files
are on E:, and the existing checkout, system Node and application data are
untouched. Docker Desktop had not yet been started, and no application or
acceptance workload had been launched. This is preparation, not a Windows pass
or the final handoff candidate. Identify every replacement revision explicitly.

The handoff must include reproducible helpers and sanitized completed reports
in the retrievable source or an accessible evidence artifact. Ignored
`artifacts/` paths alone do not meet that requirement. Record source/package
identity and distinguish completed controlled, installed and native checks.
Cloud-originated findings use `CLOUD-` IDs to avoid collision with the local
tester's `LOCAL-` IDs.

### First native correction result

The local tester subsequently selected exact correction
`9a84e740ad85e4a03540f2dfb082f98cb60604c5` for a separate test batch and reported
`TestSwarmQueueRemainsAvailableWhileSourceAdmissionIsExclusive` passing in
11.09 seconds on Windows amd64 with Go 1.26.3 and real Git. The production HTTP
registrar test verified queue GET/HEAD and controls under a held source lock,
stale-revision refusal, continued source-route protection, and no allocation.
Its local evidence is
`E:\Luci-local-acceptance\evidence\9a84e740\cloud-001-windows-go.jsonl`.
This is a reported focused native Go result, not a cloud rerun or an installed
Docker/browser pass. The same isolated native batch subsequently completed all
five tests with zero failures or skips: the held-source-lock regression, fresh
project Edit revocation, shared primary-branch admission, delayed observation
after pause/cancellation, and independent per-item controls. The source checkout
remained clean. CLOUD-001 is ready for its installed slow-allocation/UI retest.
Carry forward those results; rerun a completed focused check only when a later
revision affects its behavior. Full local acceptance is still pending.

## Findings and acknowledgement

Use stable IDs such as `LOCAL-001`. Each finding records:

- Priority, owner, state (`reported`, `fixing`, `ready to retest`, `verified`).
- Affected full revision, platform/configuration and exact reproduction steps.
- Expected and actual behavior, with relevant screenshots, logs or resource
  measurements; omit secret values.
- Fix commit and verification result. Reopen the same ID if the fix fails.

Send blockers immediately. Group related minor usability/performance findings.
The implementation thread acknowledges each received ID to avoid duplicate work;
the local tester continues independent areas while corrections are underway.

Each **LOCAL FIX READY TO RETEST** update names the issue IDs, exact retrievable
revision/artifact, checks performed and setup changes. The local thread verifies
the original reproduction and affected regression, resource and recovery paths,
then reports the result. A published patch or one passing scenario does not close
the full local objective.

Continue until the agreed local outcomes are demonstrated: interactive shadcn/ui,
useful actions across four or more projects, isolated durable context, dependable
coordination and recovery, checked output quality, and bounded resource use with
comfortable whole-computer responsiveness. Native findings remain separate from
controlled Linux/provider fixtures.

## LOCAL-ENV-001: Docker backing volume prerequisite

The local tester reported this P1 environment prerequisite while testing
`9a84e740`: Docker context `desktop-linux`, no running Desktop/backend/WSL VM,
and no engine pipe. The existing data image is
`C:\Users\DylanMackie\AppData\Local\Docker\wsl\disk\docker_data.vhdx`, with a
file length of about 70.96 GiB; C: had only 4.22 GiB free. File length is not a
measurement of allocated physical blocks. E: and F: each had over 1 TiB free.
Putting source and caches on E: does not relocate Docker's backing storage.

Owner: local environment. State: **fixing**. The owner explicitly authorized
moving Docker to a spacious drive, and the local tester is using Docker Desktop's
supported relocation to E:, first preserving a cold VHD and settings backup.
The cold VHD copy passed full SHA256 verification. Stale zero-byte Windows
socket directories were repaired with Docker stopped; Desktop 4.48.0.207573
and Linux engine 28.5.1 now run. Pre-move inventory is recorded: 107 containers,
196 images and 155 volumes; four existing containers auto-started. Relocation
has not occurred because the Windows desktop is locked. The local tester has
requested an unlock for the supported Dashboard move. C: was last reported at
3.49 GiB free. The DataFolder setting, virtual disk size and Get-DiskImage
Attached flag do not prove this layout's active backing path. No prune, reset, unregister or deletion was authorized by this update.
Before that authorization, no heavy pulls/builds were started and existing
Docker data, settings and global WSL resources were preserved. A large free-space
result inside the Linux VM cannot establish room on its Windows backing drive.
Record both before workload admission. Do not silently move/delete the VHD or
change existing WSL registrations to make the test pass. The completed five-test
native Go batch remains valid; cloud implementation and non-Docker checks continue.

## Native lifecycle and sampler batch at 9898cf5d

LOCAL-001 remains **fixing**: 11 of 15 top-level lifecycle tests passed, four
failed; 27 subtests passed and none were skipped. Unexpected allocation errors
are still being diagnosed. A separate held-Create fixture uses an unrealistically
short derived timeout; changes to that scenario do not alter production deadlines.

LOCAL-002 remains **fixing**: an unchecked failed worktree lookup passed an empty
path to a Git fixture, which committed `alpha.txt` into the isolated test checkout.
The batch started at `9898cf5d4a5302e431c8e42f5b3612c0a95e7061` but HEAD changed
at 08:04:12Z to `8f4deb8a4d7300a875b4e18ff44cf5e222fca9b1`. Go source blobs are
unchanged; later serial/short-TEMP diagnostics used this changed HEAD. Preserve
that provenance and the contaminated checkout. The original user checkout was
untouched. A fresh exact checkout is being used for diagnostic logging overlays;
those overlays are diagnosis, not acceptance of unchanged production code.

Separately, the local tester reports all nine controlled Windows observer checks
passed, followed by a real 8.017-second capture of a newly owned PowerShell root
and exact descendant. All eight samples had physical/commit/working-set/disk
metrics; CPU and I/O deltas followed the initial baseline. Output ACL was restricted
to the current user and both observed identities exited. Evidence is under
`E:\Luci-local-acceptance\evidence\9898cf5d\sampler`. This validates native
sampler APIs and ACL behavior, not Docker named-pipe compatibility or Luci's
workload memory, responsiveness and throughput acceptance.

### Diagnostic confirmation and separate timeout issue

A fresh exact `9898cf5d` logging-only overlay confirms LOCAL-001's portable-path
violation: `swarmPath` constructs backslash-separated internal relative paths on
Windows, while the traversal guard correctly requires forward slashes. The fix
must preserve that guard. Diagnostic base and final HEAD both stayed at
`9898cf5d`; overlay SHA256 is
`82D53E86F94E1613A6FC084864565CADE6880D20959686EB9CB6A8AB0639BD42`.
The local receipt is `diagnostic/allocator-logger-v2.jsonl` under the same
revision evidence directory. The overlay redirects fixture logs only; it is not
a production-source acceptance result. The held-Create 100 ms fixture budget is
now separately tracked as **LOCAL-003**, P1 validation, owner cloud, fixing.

### Correction handoff at 2819ce9e

The cloud thread pushed `2819ce9eacf62b2d73a1522b2db25301a7b393d4` and sent
**LOCAL FIX READY TO RETEST** for LOCAL-001/002/003. The local tester acknowledged
and is preparing a fresh isolated checkout for the four original failures and
both new path/isolation regressions, with actual HEAD and working-tree snapshots
before and after. Preserve the contaminated earlier checkout and diagnostics.
This correction handoff is separate from the complete local acceptance handoff.

The reported Docker server platform label is exactly
`Docker Desktop 4.48.0 (207573)`. Backing storage still resides on C:, last reported
at 3.63 GiB free; the supported move awaits Windows unlock. The verified cold
backup remains on E:. No heavy local Docker workloads have been started.

### Native correction verification

The local tester reports **LOCAL-001/002/003 verified for their original native
reproductions** at exact `2819ce9e`: all nine selected top-level tests passed
between 08:36:39Z and 08:37:37Z using Go 1.26.3, normal E: task TEMP,
GOMAXPROCS=4 and package parallelism 2. No overlay or short-path workaround was
used. HEAD remained unchanged; all 1,694 tracked-file hashes match, and the
working tree stayed clean. CLOUD-006's public HTTP catalog projection also
passed; installed dispatch/UI remains a separate gate.

The [reported receipt](evidence/2026-10-08-2819ce9e-native-fixes-reported.json)
identifies the retained local reports and attribution limits. Original failed
and mutated `9898cf5d` evidence is preserved. These results do not close the
installed Windows, four-project, live-provider or workstation resource gates.

### Native source-browser progress at the same pinned revision

The local thread continued independent testing at `2819ce9e` and reports nine
passing source-browser checks, including four distinct project contexts, scoped
drafts and service/browser restart persistence. Home accessibility had zero axe
violations, and the compact 800 px layout did not overflow. All tracked hashes,
HEAD and working-tree state stayed unchanged. Owned processes were stopped and
ports freed; no Docker allocation occurred. See the
[reported receipt](evidence/2026-10-08-2819ce9e-native-browser-reported.json).
The tester selected Continue to projects before the older local setup auto-install
route. Native worker scheduling, installed distribution, model quality and
sustained workstation resource evidence remain open.

### Latest packaging and provider preparation

The local tester reports Rust/cargo stable 1.99 with the Windows MSVC target,
VS Build Tools 2019 16.11.53/VC 14.29.30133, Windows SDK 10.0.19041.0,
WebView2 154.0.4258.62 and Python 3.13.5 (also 3.11.15). The task-local E:
activation includes Cargo and supported Node. Native64 SDK registry discovery
points to an incomplete directory while WOW6432Node points to the actual SDK;
this is an unproven packaging discovery concern, not a diagnosed defect or
reason to reinstall. Resolve supported discovery only if packaging fails.

The tester process has no provider-key or local-endpoint variables. Installed
Codex/Claude tools do not establish Luci provider access; no credentials were
inspected or imported. Real provider connection and model-quality acceptance
remain open. Docker data is still on C: (last reported 3.49 GiB free), with the
verified cold backup on E: and supported relocation awaiting Windows unlock.
No heavy local container workload has been started.

### Next focused correction handoff

CLOUD-005 and CLOUD-008 are source-verified together: 2,835 Go race outcomes,
27 explicit skips, unchanged repository bytes and complete Windows API
cross-compilation. Exact retrieval identity is supplied with the pushed handoff.
Carry forward earlier native results and run affected storage/cleanup cases on
a new pinned checkout; do not change source underneath a running batch.

CLOUD-009 is a separate required recovery correction discovered by source
review: interrupted backend Create/Start without durable settlement can retain
capacity indefinitely. It remains fixing and prevents full local completion.
Focused correction handoff is distinct from LOCAL TEST HANDOFF READY.

### Focused native correction results at 8f78804f

The local tester reports the CLOUD-005/CLOUD-008 focused batch passed at exact
`8f78804ff318ab744226f139df5998cbd6e0d0ff`: 76 top-level Go tests and 97
subtests, zero failures/skips, with Windows Go 1.26.3, GOMAXPROCS 4 and `-p2`
without the race detector. Four additional Git-fixture top-level tests and three
subtests passed, including native symlink and inherited Git redirection cases.
The five changed storage frontend files passed 53 tests with two workers and
no failures, skips or pending cases. Source/provenance completion is recorded
locally; raw files have not been independently retrieved by this cloud thread.

Separately, `TestWindowsLocalStorageReadOnlyProbe` passed at 10:07:24 UTC on
actual Windows: four mounted local volumes, C: limiting, 3,722,629,120 available
bytes. It made no Docker call, confirmed no storage mapping and started no
workload. Local receipts are under
`E:/Luci-local-acceptance/evidence/8f78804f`: `controlled-native-summary.json`,
`native-storage-probe.json`, `native-gitfixture-guards-summary.json` and
`frontend/summary.json`, with commands/logs alongside. These outcomes do not
establish installed turnover, real WSL process-start deduplication, the full
confirmation UI, provider quality or the complete local milestone.

CLOUD-009 remains the next required implementation handoff. Its source changes
will receive a new immutable revision; neither side changes the running local
batch. Docker relocation remains a local environment task awaiting desktop
unlock. Cloud implementation and the matching installed Linux comparison
continue independently.

### Supported Docker relocation started

The local tester reports Windows is now unlocked and Docker Desktop's supported
Resources > Advanced disk-image move has been confirmed, from the existing
C: WSL directory to `E:/DockerDesktop/DockerDesktopWSL`. The engine is stopping
and the move is in progress, not yet verified. Fresh pre-move inventory matches
107 containers, 196 image entries (178 distinct IDs), 155 volumes and the same
four running IDs. The cold verified backup remains on E:. The tester will
verify the live path, engine, unchanged inventory and both guest/physical host
space before allocation. Focused Windows testing stays pinned to `8f78804f`;
no cloud operation modifies that machine. CLOUD-009 development continues.

### LOCAL-ENV-001 verified; LOCAL-004 remains a separate native gate

The local tester verified the supported disk-image move. Active E: VHD, Docker
UI and CustomWslDistroDir agree; the old C: VHD is absent. Engine ID/version/
context and every container/image/volume ID and container state are unchanged:
107 containers, 196 image rows (178 distinct IDs), 155 volumes and four running
IDs. Reported free space: C: 74.20 GiB, E: 903.27 GiB, guest 886 GiB. A new
read-only probe at `8f78804f` reports four mounted volumes and approximately
74.2 GiB minimum (C:). The verified cold recovery backup is retained. Available
host RAM is only 4.38 GiB, so the next fresh source-app batch tests actual host
operator confirmation without runtime/model allocation; it uses a unique
nonexistent sandbox tag and preserves the user's existing image. No four-worker
performance claim follows from relocation.

LOCAL-004 is acknowledged separately: `8f78804f` native own-sidecar startup
ignores inherited explicit state and selects normal app/legacy state. The
tester did not launch against user data. Cloud correction must honor a validated
explicit state path before default discovery, preserve bootstrap/shutdown/default
migration and supply a new exact revision. Isolate the native WebView profile
as well. External `LUCI_API` validates a different lifecycle and does not close
this gate.

### LOCAL-004 source correction ready for native retest

This correction honors explicit `LUCI_STATE_DIR` before resolving normal or
legacy locations, and rejects invalid overrides without fallback. Nine Linux
Rust state-selection tests pass, including preserved existing/default/legacy
state and root/intermediate-link refusal. The isolated module uses the product's
exact serde_json/uuid versions; it does not compile the full Tauri caller.
[Source receipt](evidence/2026-10-08-local004-state-selection.json) records this
limit and hashes. The [native procedure](LOCAL-WINDOWS-ACCEPTANCE.md#isolated-native-webview-acceptance--local-004-correction)
also isolates the process-local WebView profile, preserves environment values
and existing state roots, and checks actual own-sidecar startup and shutdown.
Use the exact published LOCAL FIX READY TO RETEST revision between batches.
CLOUD-009 implementation is not part of this narrowly published correction.

### LOCAL-005 acknowledged; actual storage confirmation passed separately

At pinned `8f78804f`, the native tester reports Refresh Docker storage status
shows Checking storage readiness and disables Close settings, Back to project,
Choose another setup option and form controls. Source confirms cached GET calls
the changed callback, whose SetupPanel handler calls full `client.check`,
including Git, Docker, AWS and Azure checks. The process-start subscription
failed during an external tool reset, so no process/packet trace is claimed.
Local evidence: `storage-ui-20261008T1030Z/storage-refresh-ui.txt`. LOCAL-005 is
P2, owner cloud UI, fixing; cached refresh and explicit readiness must be separate.

The independent native confirmation checks passed: actual host Super Admin
review/confirmation, guest886 GiB and physical74.2 GiB across four volumes,
member GET/POST403 without private path leakage, and confirmation revision1/hash
persistence after API restart and fresh browser sign-in. Docker recovered after
an external tool interruption; post-recovery inventory verification is still
pending. This is actual native source-application evidence, not live-provider
execution, full native own-sidecar or sustained resource acceptance.

### Completed native storage confirmation batch

The tester completed the `8f78804f` source-app storage batch with all 1,795
tracked-file hashes and HEAD unchanged; the original C: checkout remained clean.
Browser, owned launcher/API/frontend were stopped, ports18731/14201 freed and
no owned containers remained. After an external tool interruption and repair of
the known stale socket issue, the relocated E: engine and all107 containers,
196 image rows/178 IDs,155 volumes and four original running IDs were reverified
at10:43 UTC. Data and backup are preserved. Available host RAM is approximately
2.91 GiB, so no sustained swarm capacity claim follows. The relayed
[public receipt](evidence/2026-10-08-native-storage-confirmation-reported.json)
records the completed scope and unavailable process-start trace.

LOCAL-004 correction is published at
`656a1ae1052814a5a6789910bb99b406cbc652ae`; native own-sidecar retest remains
open. LOCAL-005 is actively fixing. Keep exact revisions distinct between test
batches; CLOUD-009 is not present in either published checkpoint yet.

### LOCAL-004 reopened and parent-directory correction

The local tester ran the unmodified state-selection module from exact
`656a1ae1052814a5a6789910bb99b406cbc652ae` using native Rust1.99, pinned
serde_json1.0.151/uuid1.26.1, E:-only temp/cache/target and single-thread tests.
Eight tests passed; `invalid_explicit_state_fails_without_default_fallback`
failed because an ordinary file was accepted as an explicit directory. Windows
reported missing descendants without the Linux NotADirectory behavior. No native
app ran and no user state changed. Original failure is retained locally under
`656a1ae1/local004-module/` with the Cargo manifest/lock and logs.

The corrected selector explicitly rejects every existing non-directory parent,
including a file above a missing nested state folder. The nine Linux module
cases pass against captured unchanged source; this does not replace the required
native retest. [New receipt](evidence/2026-10-08-local004-windows-parent-correction.json)
retains the failed native scope and corrected source hashes. The new correction
commit also publishes the formerly missing8f788 resource companion; its link
must be checked in the source artifact before claiming the handoff complete.
