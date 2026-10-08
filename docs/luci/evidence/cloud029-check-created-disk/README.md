# CLOUD-029: check Start must not reserve another image copy

After Docker acknowledges a check's exact Create, its Start revalidation still
reserved storage for another container. On VFS this charged another pair of
image copies. A controlled test through the real HTTP, product and private
execution-ledger path reproduced `execution_disk_pressure` with 1.5 GiB free
after Create and a 1 GiB working reserve. The same workflow passes with the fix.

The correction marks a Create as acknowledged only after matching the durable
lease's identity, actor, resources and immutable reference in `created` state.
The shared evaluator then excludes only that completed Create's image-copy
allowance. Fresh host and execution observations, memory headroom, CPU limits,
complete workload inventory, host and execution working reserves, Windows
backing-volume verification, and all other pending allocations remain required.
Initial Create still reserves both VFS copies. An unacknowledged Create still
retains capacity and cannot Start.

## Triggering installed attempt and limits of the diagnosis

Attempt8 used product source `d6eaa540960fb11c6d425104f3da136bf9217712` and
helper source `81f2ba5b194a81a4f2ad167e466793f59162b387`. Its four workers and
join completed; the separately consented check Create received HTTP 201.
The actual recorded refusal before Start was **`memory_pressure`**, followed
by confirmed cleanup and `resourcesHeld:false`. CLOUD-029 is a separate defect
found while tracing that admission path; this fix does not change the recorded
memory refusal into a disk refusal or establish installed success.

Before forwarding Create, effective host available memory was 10,035,089,408
bytes. The unchanged 25% headroom on 32 GiB plus the check's 1 GiB requires
9,663,676,416 bytes, leaving only **371,412,992 bytes** of margin. Production
excludes the own check from both its pending ledger demand and the observed
container list, then adds its requested 1 GiB once. No self-memory double count
was found.

Effective memory is constrained by physical `MemAvailable` and each visible
cgroup's `memory.max - memory.current`. The independent observer retained
physical memory but did **not** retain outer cgroup current/max/stat samples.
The exact post-Create admission snapshot was not retained. Physical available
memory above 9 GiB therefore does not disprove the recorded refusal or identify
its exact cgroup cause. No memory policy, cache accounting or headroom was
relaxed.

For the configured image, VFS creation demand was 4,589,258,154 bytes. The old
pre-Start path therefore required another 5,662,999,978 bytes including reserve.
The last independent root-disk sample before the Create ACK was 1,730,125,824
bytes at 21:11:59.475274 UTC. Create was acknowledged at 21:11:59.610 and cleanup
Delete forwarded at 21:11:59.650. No disk sample falls inside that 40 ms window;
the near-ACK sample is supporting context, not the exact refused snapshot.

[attempt8-diagnosis.json](attempt8-diagnosis.json) preserves selected admissions,
physical-memory/disk samples, check transport timings and source-file hashes.
It excludes credentials and provider/account records. The private fixture and
its ledger were not mutated by this diagnosis.

## Controlled proof

- [before.log](before.log): the new HTTP regression fails against unchanged
  installed product source with `execution_disk_pressure`.
- [validation.log](validation.log): 34 top-level tests and 86 subtests pass;
  no failures or skips. This includes 15 new phase/negative cases and the
  independent second-account pending-copy scenario.
- [after.log](after.log): initial focused pass before adding the final two
  Windows backing-volume cases; the final validation includes both additions.
- [before/](before/), [after/](after/), and [manifest.json](manifest.json): exact
  source/test snapshots, hashes, executed commands and tool version.
- [SHA256SUMS](SHA256SUMS): checksums for all capsule members.

The controlled backend changes resource observations only after returning its
Create acknowledgment. Success is checked through the actual product receipt
and exact Create/Start/Remove counts. Negative cases preserve initial copy
reservation, post-Create working reserve, host/guest memory, host/execution/disk
freshness, backing-volume confirmation, complete inventory, engine identity,
other workload limits and unacknowledged-Create retention. A second real HTTP
check holds a different account's pending copy while the first is revalidated;
its capacity remains charged and its durable claim remains unchanged.

These tests do not use Docker, launch check processes, open network connections
or call a model. They establish production coordination and resource-accounting
behavior with controlled backend observations, not installed execution quality.

## Reproduction

The exact executed commands are in [manifest.json](manifest.json). From
`/workspace/luci/control-plane`, with the recorded toolchain activated, the
published overlays select the captured before/after sources without editing
the checkout:

```sh
source /workspace/.luci-tools/activate.sh
cd /workspace/luci/control-plane
# Expected exit 1, execution_disk_pressure; original before run used these same source bytes.
GOMAXPROCS=2 go test -p 1 -overlay /workspace/luci/docs/luci/evidence/cloud029-check-created-disk/before-overlay.json ./internal/api -run '^TestProjectCheckHTTPDiskAdmissionAcrossCreate/acknowledged-create-needs-no-second-copy$' -count=1 -timeout=60s -v
# Expected exit 0; original final run used these same source bytes.
GOMAXPROCS=2 go test -p 1 -overlay /workspace/luci/docs/luci/evidence/cloud029-check-created-disk/after-overlay.json ./internal/api -run '^(TestProjectCheck|TestExecutionAdmission|TestExecution.*Disk|TestExecutionWindowsHostStorage)' -count=1 -timeout=120s -v
```

The overlays were generated for reproducibility after the original tests; the
publication step did not rerun them. Rebuild the product with this correction
before an installed retest. Preserve the configured memory headroom and disk
working reserve, and retain actual effective-memory/cgroup observations across
Create and Start to resolve any further memory refusal.
