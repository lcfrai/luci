# LOCAL-013 controlled acceptance-helper evidence

The prepared helpers now bound failure shutdown, persist sanitized lifecycle
evidence before cleanup, admit the exact protocol-1 SDK check, and optionally
observe separately labelled owned checks. These are controlled helper results.
The corrected installed C013 workflow has not passed, and this record supplies no
actual workload peaks or native Windows acceptance result.

The [machine-readable record](controlled-evidence.json) contains current source
hashes, original base/result hashes for the applied patches, test scopes and
explicit limits. Earlier frozen helper copies and failed attempts were preserved.

| Validation | Result | Scope |
| --- | --- | --- |
| Combined candidate Node helpers | 173 passed, 0 failed/skipped | Real candidate helper checkout; controlled fixtures and synthetic transports; all 77 hashed inputs unchanged during the run |
| Shared lifecycle/privacy follow-up | 7 passed, 0 failed/skipped | Exact `sustained.mjs` and `acceptance-lifecycle.mjs` diagnostic basename additions; shared API unchanged |
| Linux sampler | 20 passed | Fake Docker/proc observations, current PID/cgroup and stale identity rejection, one-tick artifact/provenance coverage |
| C013 wrapper | 4 passed | Mocked subprocesses, bounded shutdown/settlement, explicit check-observer option |
| Windows sampler | 17 passed | PowerShell 7.5.5 on Linux, controlled OS/Docker fixtures and the unchanged native HTTP decoder |

These suites overlap; their counts must not be summed. The combined run predates
the diagnostic basename follow-up. Its original [command and source manifest](combined-command.json),
[result](combined-result.json) and [output](combined-node.tap) remain intact. The
affected follow-up has its own [source-bound result](diagnostic-followup-result.json)
and [output](diagnostic-followup-node.tap). The sampler and wrapper suites ran from
external staging; all applied helper bytes were verified against the tested
hashes. Python AST checks, DAG syntax and patch whitespace checks also passed.
No Docker, actual browser or installed workflow was run by this helper task.

## Bounded lifecycle and durable evidence

The old C013 source could signal Luci at its fifteen-minute deadline without
rejecting every workflow await. Its `finally` cleared that deadline and awaited
`browser.close()` before controller/proxy cleanup and the sole final report. The
new deadline rejects stuck waits and fences subsequent API/browser/Create
mutations. A durable `passed: false` checkpoint records stage, deadlines, owned
process identities and unresolved Docker drain before cleanup begins.

C013 retains its 900-second scenario. The wrapper has a 1200-second total driver
budget: 120 seconds preparation allowance, 900 workflow, and 180 cleanup. After
that total, it permits five seconds for TERM and five seconds to reap its own
forced exit. Cleanup independently attempts controllers (30 seconds), proxy (125),
browser (12) and provider (3), within the overall 180 seconds. Browser close gets
five seconds; exact profile/PID-start fallback remains reachable when launch
never returned a browser handle. The observer has a 1215-second maximum and
separate bounded settlement waits; these do not extend the driver budget.

Controlled never-resolving browser and transport promises prove that a failed
checkpoint is durable first, later independent cleanup attempts execute, and a
late workflow continuation cannot mutate. Checkpoint failure does not skip those
attempts. Timed-out operations retain their unsettled status.

The journal is mode 0600, capped at 20,000 events, 4 MiB and 8192 bytes per line.
Meaningful lifecycle/mutation transitions are fsynced. It stores approved scalar
fields, exact accepted filenames and line/column, never raw error messages,
private paths, headers, bodies, environments or credentials. Journal failure
refuses subsequent mutations. SIGKILL and supervisor loss may still prevent a
final in-process record; this instrumentation does not invent their cause.

## SDK protocol and ownership

The DAG explicitly selects `sdk-ledger-v1`. Its one separately authorized check
binds all twelve labels to the current engine/instance, check/allocation, project,
scope, command/source digests and image. The creating lease, authenticated actor,
clean integrated source, capabilities and current resource observation are
independently cross-checked before forwarding Create. Check demand is counted once
against the shared capacity reservation.

Only a fully drained real Create `201` establishes the immutable-ID ACK. SDK mode
then permits exact-ID inspect, non-following logs, one Start, and exact-ID removal
with `v=1` (optional `force=1` for product cancellation). Named check lifecycle,
attach/wait and upgrades are rejected. The prepared request pins 1 GiB memory and
combined RAM+swap, one CPU, 256 PIDs, the verified image and `AutoRemove=false`.
Failure cleanup retains the original stopped-controller, conclusive drain, ACK
and exact-ownership requirements. Intent is not ACK, response-end is not proof of
client receipt, and uncertainty does not authorize ambiguous removal.

## Resource observation limits

Linux `--include-checks` and Windows `-IncludeChecks` (or optional version-1
manifest `includeChecks: true`) add explicit owned-check attribution. The default
worker-only scope and output remain unchanged. The C013 wrapper opts in. Foreign
checks are rejected; malformed own candidates produce explicit measurement
issues. Rows identify worker/check and deduplicate immutable IDs.

Linux requires matching full-ID inspect/stats, fresh stats and a current PID,
creation tick and cgroup identity before attributing running check usage. It
records configured memory and combined RAM+swap caps separately from usage.
Its summary distinguishes `cgroup-usage-observed`, `identity-only` and
`not-observed`; unavailable measurements are not fabricated zeros. Process RSS
and cgroup memory are never added together.

The unchanged one-second cadence can miss the brief final Node check. Enabling
the option does not establish a measured check peak; the next installed witness
needs fresh coverage evidence. Admission and hard-limit proof are separate from
sampled usage. No workload delay was added to force observation.

Windows uses the current engine, exact labels/name/image, immutable-ID fresh
statistics and the existing bounded native read surface. Its configured
memory/swap caps and guest PID/start/cgroup fields remain null with explicit
unavailable status; its statistics can supply the cgroup memory limit. Actual
Windows process probes and Docker named-pipe operation of this option remain
unrun. Controlled PowerShell checks cannot establish those native behaviors.

## Historical failure scope

Attempt 4 on `4e7d5fe4` retained four native worker exits and all current task
integrations. Its legacy final check recorded Create EOF/exit 125, and neither a
complete driver nor observer receipt survived. The driver disappearance remains
unclassified. Neither the source-confirmed LOCAL-013 gap nor daemon-normalized
MemorySwap is an established cause; no retained evidence proves timeout,
supervisor reset or uncaught error. Later exact-ID recovery did not supply the
missing original ACK or proxy-drain proof. A fresh installed witness is required.
