# Local acceptance evidence

These checked-in receipts travel with the source. They distinguish source tests,
controlled workloads, installed browser/runtime checks and native Windows
acceptance. Each receipt identifies its exact source revision and limitations;
passing one scope does not establish another or complete the local product.

- [e81b5860 foundation](e81b5860/README.md): full source checks, controlled
  ten-minute scheduler run, package integrity, portable startup/persistence,
  source export and retained failures. Includes source-hash manifests and a
  verifier for historical Git blobs.
- [CLOUD-001 source correction](9a84e740/README.md): queue availability during
  source locking, per-item revision checks and current permission enforcement.
- [Partial installed resource observations](2026-10-08-e81b5860-partial-owned-resources.md):
  real browser/service/container measurements from a failed e81 run. Two task
  outputs and manual stops are not complete four-project lifecycle acceptance.
- [Installed Home controls](../../../scripts/acceptance/four-project-queue/README.md):
  real saved queue mutations, partial permission failure and exact-run navigation
  at e81, with reproducible helpers. This witness does not start agent workers.

The private raw logs, credentials, profiles and fixture workspaces stay outside
Git. Original-artifact hashes record lineage; they are not signed attestations.
Use fresh isolated profiles and retain failed attempts when reproducing checks.

The [Windows acceptance procedure](../LOCAL-WINDOWS-ACCEPTANCE.md),
[capacity procedure](../SWARM-CAPACITY-ACCEPTANCE.md) and
[rolling correction loop](../LOCAL-TEST-LOOP.md) define the remaining native and
live-provider gates. Do not treat this index as a handoff-ready notification.
