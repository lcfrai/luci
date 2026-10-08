# Installed runtime workflow

This Linux fixture uses a clean installed package, a verified existing sandbox
image, a real browser, real Docker sandboxes and the native Codex CLI. A private
provider deliberately scripts the task commands. It tests execution mechanics;
it does not establish model reasoning quality, production application quality,
native Windows behavior or deployment readiness.

The fixture creates a private account/team and four separate reviewed contexts
through labeled API setup actions. It deliberately saves **Bypass** permission
for autonomous Codex tasks. A separate saved **Ask** run must refuse preparation
and cannot enqueue an unprepared task; its route must remain unchanged and no
worker may be allocated. This is not silent permission conversion. Legacy
prepared Ask/Plan runs have additional backend regression coverage.

Native browser controls prepare the teams, authorize five named queue entries,
check the four projects' rendered goals/counts, cancel an extra waiting task,
and review/integrate each exact result. Global scheduling configuration and
pause/resume are labeled API fixture actions. Bulk Home controls have their own
[separate witness](HOME-CONTROLS.md).

Each real CLI command reads its own immutable context, proves that mount is
read-only, writes only its assigned result, commits it, and returns a receipt.
The scripted provider temporarily holds its final answer so the fixture can
inspect the live worker and restart Luci's service. Restart must retain the
same child/container/context without another allocation or tool execution.
At most three retries of an identical verified final-output HTTP request are
allowed and reported; retrying a final response is distinct from running the
task command twice.

After the provider releases the final answer, the native CLI must exit cleanly
without fixture keystrokes or a success-path session DELETE. The queue must
release capacity. Each following project must receive the single worker slot
**before** the preceding task's human review/integration. Process completion
does not count as integration, verified requirements or a complete application.

The Docker proxy requires zero existing containers/volumes, at least **6 GiB**
free before every create, one worker at a time, at most four Create/Start pairs,
and a **1 GiB** running floor. Transport observations are bounded to 20,000
rows; any omitted observation fails acceptance. The fixture never pulls images,
creates cloud resources or prunes existing Docker resources. Cleanup only
removes exact owned labeled resources. Any forced cleanup after claimed natural
completion invalidates the run.

Install the repository's desktop dependencies first. Supply an existing Chromium
executable with `--chromium` if `/usr/bin/chromium` is unavailable. Both package
archive/member provenance and the successful sandbox export receipt are required.
The expected commit must identify the installed package and matching sandbox
source. The wrapper starts the independent observer immediately and automatically
provides its exact owned instance-file path when available:

```sh
python3 scripts/acceptance/four-project-queue/run-with-observer.py \
  --observation-dir /absolute/fresh/resource-report \
  --fixture-base /absolute/short/roomy/local/directory -- \
  --binary /absolute/installed/luci \
  --web /absolute/installed/web \
  --provenance /absolute/package-provenance.json \
  --expected-source FULL_COMMIT_SHA \
  --image luci/sandbox:verified-candidate-TAG \
  --expected-image-id sha256:FULL_IMAGE_ID \
  --image-provenance /absolute/sandbox-candidate/receipt.json \
  --report /absolute/fresh/workflow-report/result.json
```

Choose a short fixture base because Unix sockets have a path-length limit; on
the configured cloud host `/workspace` is suitable. The workflow deadline is
twenty minutes. The wrapper permits up to three minutes for driver cleanup;
forced termination is reported as a failure with unknown Docker cleanup, never
as successful cleanup. See [resource measurement](RESOURCE-MEASUREMENT.md) for
measurement scope, missing metrics and native Windows procedures.

Use a fresh report directory for every attempt. Preserve failed reports and
their accepted helper snapshots. A failed private fixture may contain credentials
and must not be copied into Git. Publish only reviewed, sanitized receipts and
the reproducible helper sources. Historical interactive-worker attempts used
manual fixture stops and cannot establish natural one-shot completion.
