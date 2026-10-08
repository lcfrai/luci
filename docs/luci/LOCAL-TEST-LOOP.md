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
3.85 GiB free. The DataFolder setting, virtual disk size and Get-DiskImage
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
