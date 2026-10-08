# CLOUD-026 alpha source-read cohort

Installed product `d6eaa540` attempt 6 returned HTTP 409 with `Project source is busy. Wait for the current source operation before integrating.` from its single stale-review POST. Its unchanged effects, primary HEAD/status and public/private revision were retained. The prior holder of source admission was not recorded. This does not establish continuous polling starvation or identify attempt 5's unknown reason. The [failed installed capsule](../cloud026-installed-d6eaa540-failed/README.md) remains separate; [targeted facts and response-header timing](original-failure-summary.json) retain their original hashes.

This is a helper-only correction. After alpha integration, the actual app dispatches `PROJECT_CONTEXT_CHANGED` from `ProjectRuns.onUpdate`. Develop mounts two listeners with six source reads:

| Mounted listener | Exact GET paths below `/v1/product/projects/{id}` |
| --- | --- |
| `ProjectWork` | `/brief`, `/design`, `/specification`, `/design-plan`, `/git` |
| Always-mounted `OrchestratorPanel` | `/orchestrator`, queued until its own mutation finishes |

The third listener, `DeploymentOverview`, belongs to Deploy and is not mounted in this Develop flow. The listener, API mapping and mounting files match installed `d6eaa540`; their hashes are in the [source manifest](source-manifest.json).

The driver now arms a passive observer after alpha's review opens and before its existing integration click. It tracks the exact mutation and subsequent matching GET request objects, requiring all six HTTP 200 responses, completed `requestfinished`/`response.finished()` and full bodies. It records method, exact path, start/end timestamps, status, byte count and body hash, without retaining response bodies. Preexisting or wrong-project requests cannot fill the cohort. Missing, duplicate, failed, oversized or wrong-project responses fail within the existing workflow budget. Git and orchestrator source must both identify alpha's clean integrated commit.

The helper reveals the existing project-facts and project-understanding panels. It then requires enabled refresh controls, alpha's visible commit, an enabled current Next step, and no pending, stale-context or read-error state. Only this observed cohort and UI boundary are marked ready. The boundary is recorded in `alpha-source-read-cohort.json` and the main receipt before the unchanged single strict stale-review POST. A later ordinary read may still contend; a busy refusal still fails. There is no arbitrary sleep, network-idle heuristic, polling suppression, repeated mutation or source-lock change.

Cancellation removes listeners and fences late body/UI completions from updating the receipt or progressing to the probe. Outstanding read promises have rejection handlers; already-issued UI disclosure clicks may finish but cannot authorize a later mutation. Existing CLOUD-025 response diagnostics and LOCAL-013 startup/cleanup logic remain intact.

**Validation:** 44 tests passed, zero failures/skips/todos: 29 new controlled request/body/readiness/source-seam tests and all 15 existing CLOUD-025 tests. DAG and new helper syntax checks passed. [Raw test output](checks/diagnostic1-2.stdout) and [exact command/source receipt](checks/diagnostic1.json) preserve the checked bytes. Tests cover delayed/missing/failed sixth reads, headers versus body completion, wrong project, busy/denied status, duplicate requests, stale source, visible pending/error state, timeout, cancellation, late rejection and late UI completion. These are synthetic event and minimal DOM-interface checks, not rendered UI acceptance. No browser, Docker, application service, network or provider was run for this correction.

Reproduce from repository root:

```sh
node --check scripts/acceptance/task-dependencies/dag.mjs
node --check scripts/acceptance/task-dependencies/source-read-cohort.mjs
node --test --test-concurrency=1 \
  scripts/acceptance/task-dependencies/source-read-cohort.test.mjs \
  scripts/acceptance/task-dependencies/stale-review-probe.test.mjs
```

To check the frozen evidence bytes, substitute `docs/luci/evidence/cloud026-read-cohort/source` for `scripts/acceptance/task-dependencies`. [The narrow source patch](source-change.patch) contains only the driver and new helper/test. The installed product package remains bound to `d6eaa540`; a subsequent installed attempt must separately record its explicitly selected helper revision. This evidence does not claim that subsequent installed acceptance passed. [Publication manifest](publication-manifest.json) hashes every evidence file except itself.
