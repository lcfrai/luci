# CLOUD-045: terminal required queue targets

The original sustained attempt remains failed. Its exact product and helper were `0dcafb14da7d1f7e2eedaf70366f1e3cf4f4303f`; the later documentation checkpoint `7aeb3373f9cf21c1c8d39711f9ee6ec294226633` contains identical helper bytes. This capsule changes only `scripts/acceptance/sustained-four-project/sustained.mjs` and `queue-selection.test.mjs`. No product admission, scheduler, retry or progress behavior is changed.

The original B queue, operation and exited session retain `cpu_pressure` at final admission, before Docker Create. B was already terminal while A, C and D completed naturally. Failed queue entries are intentionally ineligible for automatic dispatch; this was a resource refusal, not a queue backoff deadlock. The helper ignored that required terminal target and eventually reported a bounded allocation timeout under the previous D stage. `original-diagnosis.json` binds the unchanged original result, accepted driver, scheduler, run and sessions files and contains allowlisted facts and exact source references. The original CPU percentage is unavailable. Its configured refusal predicate was measured host CPU utilization greater than `100 - 25 = 75` percent; neighboring observations cannot reconstruct the actual operand.

The correction observes all required candidates, applies the existing project, run, enqueue and one-worker guards, then stops on an exact required failed or cancelled entry even when another candidate is running. A prematurely completed required target stops with a distinct `required-queued-task-completed-before-capture` code; it does not become an accepted result. The deliberate extra `cancel-a` target remains separate. Blocked, unknown and other nonterminal states keep the previous conservative behavior.

The initial `required-queue-refusal.json` is written through the existing synchronous file-and-parent-fsync helper before any diagnostic await. Up to three parallel GETs have a 3-second timeout each; their allowlisted run, operation, session, scheduler and resource projections go to an additive `required-queue-refusal-details.json`. Failed or mismatched reads remain explicit. No arbitrary messages, prompts, context, paths, commands, environments or tokens are copied. Resource snapshots are labelled later and non-atomic with the queue, run, session and actual decision. This helper does not claim to retain the original admission operands; CLOUD-046 is separate.

The final same-test comparison uses the identical frozen test bytes against original and revised helper sources:

- Original: 46 cases, 39 passed and 7 expected failures, exit 1.
- Revised: all 46 passed, zero skipped or cancelled, exit 0.
- Revised helper syntax and two-file `git diff --check` passed. Bound source, test, retained fixture and lifecycle helper bytes were unchanged during validation.

The final test batch includes all 24 project orders, retained C043 identity/context/cancellation/priority/one-worker checks, failed/cancelled/completed required-target handling, unavailable or foreign diagnostic identities, and refusal to continue if the first durable write fails. These are inert GET, context and loop fixtures. No service, browser, Docker, network, provider or native workload was started. They do not establish an installed retest, native-platform gate, performance improvement or full sustained acceptance. Durability uses the existing `writeFailureCheckpoint` implementation; the selection tests mock its call boundary rather than claim a new filesystem fault-injection test.

`validation-final.json` retains exact commands, source/test hashes, input bindings before and after, versions, exits, counts and raw log hashes. `*-final.stdout.log` and `*-final.stderr.log` are raw final process outputs. Earlier `*-attempt1.*` files and `validation-attempt1.json` remain separate: that earlier 45-case batch preceded the additional completed-target regression. `candidate.patch` contains only the two authorized source files; `candidate.json` binds the exact before and after bytes. Root owns integration and publication.

To reproduce the final inert comparison from this directory (the original run is expected to exit 1):

```sh
LUCI_SUSTAINED_TEST_SOURCE="$PWD/before/sustained.mjs" node --test --test-reporter=tap after/queue-selection.test.mjs
LUCI_SUSTAINED_TEST_SOURCE="$PWD/after/sustained.mjs" node --test --test-reporter=tap after/queue-selection.test.mjs
node --check after/sustained.mjs
```
