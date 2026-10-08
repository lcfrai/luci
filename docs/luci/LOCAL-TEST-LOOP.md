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
