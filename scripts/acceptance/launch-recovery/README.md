# Installed uncertain-launch recovery acceptance

This Linux-only harness exercises the installed product, its authenticated browser recovery controls, a real Docker engine and the native Codex task process. It is complementary to the four-project acceptance workflow. **The harness being present or its transport tests passing does not establish that an installed scenario passed.** Record each real attempt separately against a clean packaged revision.

The provider is deliberately scripted. A successful run establishes lifecycle, isolation, recovery and review mechanics for the selected scenario, not model reasoning quality, Windows acceptance, broad throughput or the finished local product.

## Two separate bounded scenarios

`delayed-create` queues a task and holds its uploaded Docker Create before forwarding it. Recovery while the allocator is alive must refuse promptly without recording consent or changing Docker. The harness kills and reaps the exact personal-owner controller, restarts the same private state, uses the recovery dialog and explicit checkbox, then verifies the acknowledged inert fence and durable release. The original Create is forwarded to the real daemon and must receive **409**. A second project then runs one actual native task, exits with status zero and integrates its exact changes only after browser review.

`delayed-start` starts the original task manually. Its actual container is created, while the uploaded immutable-ID Start waits before forwarding. After killing and restarting the controller, the first real fence Create succeeds but its response is deliberately lost. The product must retain capacity and report pending/failed recovery. A reviewed retry must replace the unacknowledged fence with a **different acknowledged ID under the same intent**. The harness holds a real finalizer Inspect reply only after private product proof and retirement are durably recorded, kills the controller again, and verifies that the next reviewed retry resumes the same acknowledged fence. The original Start must subsequently receive real **404**. A queued task in a second project must still complete and integrate.

The proxy delays requests or loses replies; it does not manufacture successful Docker responses, absence proofs, project records or API results. Controller deaths are actual SIGKILL followed by reaping, not graceful worker deletion. API calls seed two explicitly approved synthetic projects and configure the one-worker policy; browser actions authorize task work, recovery and integration. Public recovery DTOs must omit private proof fields.

## Admission and cleanup

An operator must reserve an exclusive Docker window and explicitly authorize these destructive **fixture-only** process-death tests. Use a fresh short fixture parent on a roomy local volume. Never target installed personal data or an existing application state directory.

The driver refuses root execution, existing containers or volumes, an unverified image, dirty package provenance, mismatched archive members or changed sandbox source inputs. Before **every** actual Create, including an inert scratch marker, at least **6 GiB** must remain free on the Docker backing filesystem mounted at `/` in this environment. A one-second monitor enforces a **1 GiB** running reserve. This filesystem assumption must be checked before use on another host; the script is not a generic Docker Desktop capacity detector.

Each invocation permits only two full-image Create requests: the interrupted original and the independent successor. `delayed-create` additionally allows one scratch Create (three total) and one Start; `delayed-start` allows two scratch Creates (four total) and two Starts. There is one expensive worker at a time, with explicit 1 GiB memory, one CPU and 256 PID limits checked before Create. No image pulls, builds, cloud resources or release containers are permitted. The original unforwarded request has a 600-second maximum hold, real upstream requests have 120 seconds, and the scenario has 15 minutes. Any timeout is a failed attempt, never recovery proof.

The proxy bounds concurrent requests, buffered bodies, reply sizes, sockets and observations. Unknown mutations and upgrades fail closed; Start/Stop/Delete must address an immutable ID returned by this proxy, and fence Start/Stop is refused. No failed scenario is automatically retried.

Production recovery markers are permanent. The harness has a separate, explicitly labelled **test-only teardown**: only after every owned controller is dead and every captured request has drained or was cancelled before forwarding may it inspect and remove a captured marker by exact immutable ID, without force. It rechecks the instance, child, name, intent, binding and inert configuration. An uncertain forwarded mutation prevents marker cleanup and preserves private state. Unexpected leftover workers fail acceptance; cleanup may stop that exact owned worker and remove it without force after inspection. Unrelated images, volumes, processes and files are never pruned.

## Reproducible invocation

Prerequisites: Linux `/proc`, Node supported by the repository, Python 3, Git, Docker CLI access to a local Unix socket, Chromium, installed `desktop/node_modules/playwright`, a clean installed Linux package and its retained provenance, plus a byte-verified image receipt. Install dependencies using the repository's normal setup instructions. Do not build or pull images during the test.

Transport and process-cleanup checks below use only fake HTTP/process fixtures; they do not contact Docker:

```sh
node --test scripts/acceptance/launch-recovery/fenced-docker-proxy.test.mjs scripts/acceptance/launch-recovery/worker-create-guard.test.mjs
python3 scripts/acceptance/launch-recovery/run_with_observer_test.py
node scripts/acceptance/launch-recovery/recovery.mjs --help
```

After source/package freeze and explicit runtime admission, run one scenario with fresh report and observer directories. Replace each placeholder with the pinned artifact; the image receipt must identify the same image and bind its source inputs to the package commit.

```sh
python3 scripts/acceptance/launch-recovery/run-with-observer.py \
  --fixture-base /workspace \
  --observation-dir artifacts/quality-swarm/recovery-create-attempt1-resources \
  -- \
  --scenario delayed-create \
  --binary '<installed-package>/luci' \
  --web '<installed-package>/web' \
  --provenance '<package-provenance.json>' \
  --expected-source '<exact-40-character-source-commit>' \
  --image '<existing-verified-image-tag>' \
  --expected-image-id 'sha256:<exact-image-digest>' \
  --image-provenance '<byte-verified-image-receipt.json>' \
  --report artifacts/quality-swarm/recovery-create-attempt1/result.json
```

Repeat separately with `--scenario delayed-start` and new report/observer locations. A failure preserves its evidence and stopped private fixture; correct the defect and use a new attempt directory. A revised harness is a separately identified input even when product bytes are unchanged.

## Evidence and limits

The driver preserves its own accepted source, proxy, wrapper, reused terminal probe and resource observer with SHA-256 values; verifies installed package/archive members; records package revision, actual binary/web hashes and image identity; and retains allowlisted Docker request chronology. It records actual controller identities/crashes, recovery checkpoints, late daemon status, exact fence replacement/reuse, retained context hashes, clean native exit, verified task output and reviewed integration. Raw fixture state contains fixture credentials and private paths: keep it private and publish only reviewed, allowlisted receipts and synthetic screenshots.

The wrapper starts the frozen Linux resource observer beside the driver and publishes only the exact owned instance-file path. It tracks controller, browser and worker ownership by PID/creation identity and Docker instance/image. Its Docker container measurement covers full-image workers, **not inert scratch markers**; marker shape/count and root disk samples are recorded separately. The observer and harness impose overhead; concurrent compilation or other host activity must be disclosed. Failed observation or forced wrapper termination does not establish clean Docker teardown. Read the [observer scope](../four-project-queue/RESOURCE-MEASUREMENT.md) before interpreting CPU/RAM data.

Do not publish raw environment, credentials, provider payloads, personal paths or unbounded logs. Actual installed results, final package revision and remaining Windows/live-provider gates belong in the release handoff after execution. This helper does not clear those gates by itself.
