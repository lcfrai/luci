# Local acceptance evidence

These checked-in receipts travel with the source. They distinguish source tests,
controlled workloads, installed browser/runtime checks and native Windows
acceptance. Each receipt identifies its exact source revision and limitations;
passing one scope does not establish another or complete the local product.

- [e81b5860 foundation](e81b5860/README.md): full source checks, controlled
  ten-minute scheduler run, package integrity, portable startup/persistence,
  source export and retained failures. Includes source-hash manifests and a
  verifier for historical Git blobs.
- [9898cf5d task-worker source evidence](9898cf5d/README.md): exact package,
  frontend and scoped Go provenance, including limitations and the subsequent
  LOCAL-001/LOCAL-002 native findings. Do not rerun its unsafe fixture helpers.
- [9898cf5d installed Home controls](2026-10-08-9898cf5d-home-controls.md):
  actual bulk mutations and permission failures, plus explicitly controlled
  source-lock response handling; no agent execution.
- [CLOUD-001 source correction](9a84e740/README.md): queue availability during
  source locking, per-item revision checks and current permission enforcement.
- [Partial installed resource observations](2026-10-08-e81b5860-partial-owned-resources.md):
  real browser/service/container measurements from a failed e81 run. Two task
  outputs and manual stops are not complete four-project lifecycle acceptance.
- [Installed Home controls](../../../scripts/acceptance/four-project-queue/README.md):
  real saved queue mutations, partial permission failure and exact-run navigation
  at e81, with reproducible helpers. This witness does not start agent workers.

- [Reported native Windows observer checks](2026-10-08-windows-owned-observer-native-reported.md):
  native process/memory/disk metrics and private output ACL checks reported by
  the local tester; no Docker or Luci workload performance claim.

- [Reported 2819ce9e native corrections](2026-10-08-2819ce9e-native-fixes-reported.json):
  nine Windows tests passed, original three local findings verified at their
  reproduction scope, and all tracked source bytes plus HEAD preserved.

- [Reported native four-project source-browser smoke](2026-10-08-2819ce9e-native-browser-reported.json):
  startup, distinct saved purposes, switching, scoped draft recovery, restart
  persistence and accessibility; no agents, model output or resource workload.

- [2819ce9e package and source](2819ce9e/README.md): clean portable package,
  extracted-file checks, licences and source archive identities.
- [Installed four-project execution](2026-10-08-2819ce9e-installed-swarm.md):
  four native Docker/Codex tasks, natural exits, automatic capacity handoffs,
  retained worker across controller restart, and explicit browser integration;
  provider output is scripted.
- [Installed owned-resource observations](2026-10-08-2819ce9e-installed-swarm-resources.md):
  445 samples with exact process/container scope, gaps and host-overlap limits.
- [Windows storage frontend](2026-10-08-windows-storage-ui.md): 2,394 passing
  tests, exact-source TypeScript, desktop/phone component interaction and axe
  evidence, plus portable fixture helpers. Native storage remains unverified.
- [Controlled Windows storage backend](2026-10-08-windows-storage-controlled.json):
  identity-bound confirmation, shared private observations and Windows
  cross-compilation, with precise source hashes and native limits.
- [Shared storage across isolated processes](2026-10-08-windows-storage-multiprocess.json):
  18 actual controlled reader processes, 360 reads and zero worker probe
  attempts, with expiry and identity invalidation. Native Windows probe counts
  remain a separate gate.
- [Additional Git fixture isolation](git-fixture-helper-isolation.json):
  temporary-directory ownership and unchanged parent-repository regressions.

- [Combined storage/cleanup source verification](storage-cleanup/README.md):
  2,835 Go race outcomes, complete Windows API compilation, unchanged source
  and exact skip/limitation records. CLOUD-009 remains open.
- [Reported 8f78804f native correction checks](2026-10-08-native-8f78804f.md):
  173 focused Go outcomes, seven Git-fixture outcomes, 53 storage frontend tests
  and an actual read-only Windows storage probe. This does not establish
  installed confirmation, real WSL probe counts or whole-computer capacity.

The private raw logs, credentials, profiles and fixture workspaces stay outside
Git. Original-artifact hashes record lineage; they are not signed attestations.
Use fresh isolated profiles and retain failed attempts when reproducing checks.

The [Windows acceptance procedure](../LOCAL-WINDOWS-ACCEPTANCE.md),
[capacity procedure](../SWARM-CAPACITY-ACCEPTANCE.md) and
[rolling correction loop](../LOCAL-TEST-LOOP.md) define the remaining native and
live-provider gates. Do not treat this index as a handoff-ready notification.

- [LOCAL-004 state-selection source checks](2026-10-08-local004-state-selection.json):
  nine Linux Rust module tests; full Tauri compilation and native own-sidecar
  startup, bootstrap and shutdown still require Windows acceptance.
- [8f78804f installed four-project comparison](2026-10-08-8f78804f-installed-swarm.md):
  four natural completions and reviewed integrations, matching output hashes,
  measured cleanup intervals and retained baseline limits. Scripted provider.

- [LOCAL-004 Windows parent-path correction](2026-10-08-local004-windows-parent-correction.json):
  preserves the native8/1 failure at656a1ae1 and corrected Linux9/0 module
  receipt; native retest remains open.
- [8f78804f installed resource companion](2026-10-08-8f78804f-installed-swarm-resources.md):
  294 observations with exact scope, gaps and remaining idle/latency/soak gates.
- [Reported native storage confirmation](2026-10-08-native-storage-confirmation-reported.json):
  actual Windows host confirmation, member denial, restart persistence and
  unchanged source; LOCAL-005 and workload acceptance remain open.

- [Reported native 2dc own-sidecar lifecycle](2026-10-08-native-lifecycle-2dcf245d-reported.md):
  actual isolated release startup, owner/four definitions, controlled reopen and
  two owned shutdowns. All 1,808 tracked hashes/HEAD and original checkouts remain
  unchanged. LOCAL-004 metadata preservation and LOCAL-007 rapid-click selection
  remain unresolved; this is relayed evidence, not a cloud native rerun.
