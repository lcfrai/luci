# CLOUD-087: retain the actual runtime-preparation refusal

This adds diagnosis for the next refusal. It does **not** identify the cause of
the original Windows job at577, whose failing sample was not retained. The
separate Created-container correction is CLOUD-089.

The admission path copies a bounded projection of the exact snapshot and time
already used by the original predicate. It records sample freshness, required
metric availability, inventory completeness and engine match. If the existing
CPU/memory predicate supplied operands, their measured values and threshold are
retained too; an observed zero is explicit, while unavailable operands stay
absent. It adds no probe, admission exception, automatic retry or lease release.
Daemon identifiers, paths, commands, workload lists and sampler error strings
are excluded from this projection.

Both initial and final pre-build refusal phases persist the observation in the
existing runtime job receipt. Setup offers a read-only “Why preparation paused”
disclosure to backing managers. Older receipts gain no invented explanation.
The original job JSON stays unchanged through status reads and restart.

## Validation and limits

- Initial API run:4top-level +13subtests passed. These overlap the affected run.
- Affected API/product race run:22top-level +23subtests passed, nofail/skip.
  Includes existing installer authority, interruption and retained-allocation
  cases plus missing/stale/future/incomplete samples, actual CPU/memory refusal,
  second admission, zero build side effects, and receipt durability.
- Actual setup components with controlled clients:13assertions in2files passed.
  Opening the explanation adds no check/install/cancel/reconcile call, backing
  viewers do not see it, and workspace switching removes the old observation.

Raw JSONL/logs and explicit command exits are retained. The first API command
was `GOMAXPROCS=2 go test -p1 -count=1 -timeout=180s -json
./control-plane/internal/api -run
'^TestRuntimeInstaller(RetainsActualUnavailableAdmissionWithoutBuild|MissingSamplerIsDistinctFromMissingMetric|RefusalRetainsCapacityOperandsAndSecondAdmission|SuccessDoesNotInventRefusalEvidence)$'`.
The affected command was `GOMAXPROCS=2 go test -race -p1 -count=1 -timeout=180s
-json ./control-plane/internal/api ./control-plane/internal/product -run
'^TestRuntimeInstall(er|Admission)'`. Run from the repository with the documented
Go workspace/toolchain. Frontend:from `desktop`, `npm test -- --maxWorkers=1
src/components/setup/RuntimeAdmission.test.tsx
src/components/setup/RuntimeCancellation.test.tsx`.

`selected-api-inputs.json` and `selected-after.json` bind unchanged selected
source inputs during the API/race checks. The earlier frontend manifest predates
the explicit-zero pointer representation; frontend bytes are unchanged between
these manifests. Other coordinator/team work was present in the shared checkout,
so this is not a clean committed whole-graph compilation claim. Counts are
scoped and overlapping, not a full product suite. No installed image, Windows,
real-provider, responsiveness or whole-product acceptance follows.

## Windows retest

Use a fresh isolated source/package revision containing this change. Keep the
original577job/evidence unchanged. No configuration or runtime-image source
change is required. Preserve `LUCI_STATE_DIR`, `LUCI_PROJECTS_DIR`, a free
`LUCI_ADDR`, the shared Docker inventory and confirmed physical backing storage.

1. Run the named native Go cases and affected setup components, then build using
   the normal release script. Do not use the disposable-runner installed test.
2. Inspect real Created-container inventory for CLOUD-089. Confirm owned pending
   reservations still count; do not start or remove unrelated containers.
3. When ordinary host headroom permits, request installation once with the
   exact pinned source and an absent unique runtime image tag. Retain the202job
   ID and complete terminal JSON. If refused, compare the saved admission
   phase/time/operands and UI text; do not replace it with a later sample.
4. Open/close the disclosure, switch workspace, restart the isolated service and
   reread the same job. Confirm no hidden retry and no rewriting of its evidence.
   A legitimate resource refusal remains a refusal; do not lower the guards.
5. Actual bounded image installation, cancellation/recovery and authenticated
   upstream coding remain separate live acceptance work.
