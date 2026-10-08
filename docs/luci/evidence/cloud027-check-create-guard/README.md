# CLOUD-027 check Create owner contract and diagnostics

The installed attempt with product `d6eaa540` and helper `fdd4c0e8` reached four workers, three integrations, retained conflict recovery and the independent final-output check. Its SDK whole-build check Create was refused by the helper before forwarding. The proxy retained `request-guard-failed`; it did not retain the original request body or identify-guard assertion stack. The final application check was `unknown`, so this attempt did not pass.

[The offline owner regression](original-owner-regression.json) preserves the exact former assertion and trusted fixture owner ID. Management generates `user-` plus 22 URL-safe Base64 characters. The former helper required 24 lowercase hex characters, so that exact assertion rejects the observed owner. This establishes a source-contract mismatch, not a recovered runtime assertion trace. Synthetic diagnostic-stack tests must not be treated as historical evidence. Original fixture proof and cleanup/recovery remain the coordinating agent's separate evidence.

The helper now accepts the actual management ID shape. Authorization remains the unchanged exact comparison between the private active check lease's actor and `{userId: fixtureOwnerID, projectId: owner.owner.id}`. A different valid-looking owner, project, actor fields, identity, inactive lease or extra lease still fails. Container labels, command digest, resource limits, source binding and private lease checks remain strict.

Both identity phases now write an exclusive, synchronous, fsynced checkpoint before returning to the proxy: initial `identify-create` and immediate `before-create` revalidation. Each `identify-create-NN.json` contains only stage, phase, request sequence/ordinal, body byte count/SHA256, guard outcome and bounded error code/source-basename locations. It contains no request body, environment, headers, credentials, free-form error message or private path. An `identified` outcome records guard completion with `forwarded:false`; it never claims Docker creation. Failure to persist a checkpoint refuses forwarding. The existing five-Create budget admits at most ten such guard checkpoints.

**Validation:** 106 focused tests passed, zero failures/skips/todos: 15 new owner-contract/diagnostic/exact-driver-seam tests, 47 unchanged project-check guard tests, and all 44 CLOUD-025/026 tests. DAG and new helper syntax checks passed. [Raw output](checks/diagnostic1-2.stdout) and [source/command receipt](checks/diagnostic1.json) retain the exact checked bytes. The new tests cover the former assertion, current URL-safe owner, foreign actors/projects/leases, durable failure before simulated forwarding, success versus creation, both phases, failed writes, malformed body and secret-text exclusion. No Docker, browser, product server, API or network was run for these checks.

An independent coordinating-agent SDK wire check is separate evidence: it exercised production `PrepareCheck`/`CreateCheck`, real SDK API 1.51 serialization and helper validation using retained fixture inputs with Create-time status modeled as running. It found matching Go/JavaScript command digests and no further wire mismatch. That modeled request is not the original unretained body; this capsule does not claim an installed rerun passed.

Reproduce from repository root:

```sh
node --check scripts/acceptance/task-dependencies/dag.mjs
node --check scripts/acceptance/task-dependencies/identify-create-guard.mjs
node --test --test-concurrency=1 \
  scripts/acceptance/task-dependencies/identify-create-guard.test.mjs \
  scripts/acceptance/task-dependencies/project-check-guard.test.mjs \
  scripts/acceptance/task-dependencies/source-read-cohort.test.mjs \
  scripts/acceptance/task-dependencies/stale-review-probe.test.mjs
```

For frozen bytes, substitute `docs/luci/evidence/cloud027-check-create-guard/source/task-dependencies` for `scripts/acceptance/task-dependencies`. Required unchanged neighboring helpers are included. [Source manifest](source-manifest.json), [narrow patch](source-change.patch) and [publication manifest](publication-manifest.json) bind the original helper, correction and evidence. Product source and prior CLOUD-025, CLOUD-026 and LOCAL-013 helper modules remain unchanged.
