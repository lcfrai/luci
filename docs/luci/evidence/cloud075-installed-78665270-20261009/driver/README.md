# Prepared CLOUD-075 installed Linux check

Status: prepared and syntax-parsed; five controlled in-process route-boundary tests passed. The application, browser, fixture APIs and package verifier have not been run by this preparation. Execution requires the parent's explicit serial runtime grant and `--window-granted`.

Bound package: `786652704fa8bebd1ba1af41cfa1d414d7def100`, clean packaged Linux binary and web assets at `/tmp/luci-retained-tools/package-0.5.0-78665270`. The driver checks the captured provenance hash, source revision and every installed archive member before starting the application. It validates frozen local helper bytes and selected Playwright/lockfile metadata before startup and after cleanup. This is installed web/API verification, not a native desktop window or latency acceptance.

The check creates a fresh isolated owner, ordinary editor member, one team and four projects through real management/product APIs. Each project has a distinct saved purpose and a real per-project grant with execute/secrets/deploy/admin all false. The member signs in through the actual UI, continues past initial personal setup if shown and selects the team through the Workspace select. No auth or application storage is seeded. Normal harmless read-only setup/check and dependency/check POST probes are permitted and recorded; runtime installation and all agent/session/check/task mutations are refused. Measurement begins after the initial team Home settles.

Four project opens each require exactly one overlapping current-project permission GET and the matching saved purpose, with other projects' purposes absent. A later full browser reload on the same selected project requires a fresh permission GET. To expose overlapping consumers deterministically, these real permission responses are fetched unchanged and their delivery delayed by 300ms. This is an explicitly instrumented request-count witness; it provides no timing or latency comparison.

A separate boundary check holds a real successful permission response for a newly opened project, removes the member's team access using the owner's current real revision, and waits for actual app workspace suspension. The held response is then released/cancelled. Membership is restored without project grants; the visible UI must show none of the four private purposes and the real member specification GET must return403. A current project grant is subsequently restored, then actual UI reload/open must display the correct purpose through a fresh read. This coarse transport race is not claimed as the exact settled-promise microtask race proved by the carried component tests.

Management gateway and owned user/team workspace service subprocesses are required by the real product and explicitly accounted. They are not agent/shell/task/check execution. No provider is configured. Both Docker endpoint variables point to a fresh, absent private Unix socket; the global Docker daemon/configuration is not read or changed. Actual session catalogues must be empty before and after. Browser and operator mutation allowlists prohibit execution/create routes. Scoped process scans use exact fixture markers or exact managed authority/state identities; PID+start-time checks fence cleanup. No existing/shared process is signalled. A fresh inherited-env allowlist preserves only PATH/LANG/LC_ALL/TZ before fixture-specific settings are applied.

Workflow budget is180s plus bounded independent browser/controller/final-observation cleanup (40s nominal,45s reserved). HTTP reads are bounded10s, manual response hold15s and package verification45s. Every request/mutation boundary checks workflow health. Startup handles remain owned if an async factory completes late. Cleanup writes a durable checkpoint and records final identities. State, tokens, private failure details and screenshots remain in the private run root; they must not be blindly copied to publication. Journal entries contain only paths/status/counts/hashes and reject known token/password values.

After a runtime grant, create a fresh mode0700 run-root directly under a private mode0700 parent. Example command (the run root must already exist and be empty):

```sh
source /workspace/.luci-tools/activate.sh
node /workspace/.luci-tools/cloud075-installed-preparation/launch.mjs \
  --repo /workspace/luci \
  --artifacts /tmp/luci-retained-tools/package-0.5.0-78665270 \
  --provenance /workspace/luci/artifacts/quality-swarm/candidate-78665270/package/package-provenance.json \
  --product 786652704fa8bebd1ba1af41cfa1d414d7def100 \
  --chromium /usr/bin/chromium \
  --parent /workspace/.luci-tools/cloud075-private-runs \
  --run-root /workspace/.luci-tools/cloud075-private-runs/attempt-001 \
  --window-granted
```

A failed or unsupported setup records its bounded failure and closes only owned resources. It must not be called a successful actual-management check. Source/protocol tests already passed are carried separately in the CLOUD-075 source capsule; this driver does not repeat them or alter their result.

## Teardown source review and carried validation

The exact original prepared launch and its binding are preserved under `pre-teardown-correction/`. Before any installed run, the launch was corrected so declared teardown cancels every already-permitted same-origin GET/HEAD/OPTIONS without forwarding; foreign requests and mutations still fail. Accepted route promises are tracked before yielding. Forwarded POST readiness/login requests remain pending until actual requestfinished/requestfailed events; they are never cleared merely because cleanup begins. Route admission is sealed and accepted callbacks drain before the final receipt. Timeouts and unconfirmed outcomes remain failures. The first failure writes a bounded private record and safe public metadata directly from `fail()`, including failures arising after the main catch.

A manual held-response fulfillment error is not automatically called cancellation: it requires an independently observed `net::ERR_ABORTED` on that exact request; otherwise it remains unconfirmed and fails. Independent read-only review by `/root/check_backend` found no further concrete blocker at launch SHA-256 `c21fabfeeef93563772eecf2e77eb033f03815574f8e37686a3005d58ecc12e6`, scenario `ee118dc131f470638dfd84be245b4f37ee5bb9ac99754d2ff9732ecbe2734e03`, and route-boundary `00104ba5f89a8606535e3853e5df54da5ce3dead9a7db283ce0cdbdededffa4a`. The reviewer ran no tests or runtime.

The parent granted one source-only test window. `node --test route-boundary.test.mjs` joined with5passing/0failing tests, exit0; raw output and exit file are retained. Tests cover normal mutation terminal settlement, tracked delayed read-abort completion, late abort rejection, forbidden foreign/mutation admission after seal, and tracked specific-scenario route rejection. No application, browser, Docker or managed service starts occur in those tests. This supplements the carried CLOUD-075 product tests without repeating them.
