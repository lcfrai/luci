# CLOUD-076 frontend publication capsule

This is the exact external frontend candidate and controlled evidence for an optional attach preparation hint. Root owns integration. No installed app, browser, container or provider was executed by these checks. The recorded installed WebSocket HTTP 409 had no captured server reason; this change does not establish its cause or resolve a backend Create/Restart that remains pending.

Use **`frontend-v2/CLOUD-076-frontend.patch`**. Its SHA-256 is `4acd91725ae235ac63ea2d5a2354971ad1bde453f44b0c7878862e8a9557b88b`; the final frozen manifest is `a9530a09ff25dbb3efa4b61c0dd3942048e562e988f43ce05ca0b396c7f61fd4`. It contains three production files and one dedicated 27-case test. Original bytes are based on functional commit `786652704fa8bebd1ba1af41cfa1d414d7def100`; later runs record docs-only HEAD `078d0e99f9d1553a2233dfa51b368b415964766c`. Captured before/after hashes, rather than HEAD alone, bind the executed inputs.

## Approved backend-v2 contract

The optional `attachReady` hint is advisory, not health or authority. Backend-v2 evidence is separate from this capsule.

| Hint | Meaning |
| --- | --- |
| `true` | Known generation, starting/running/idle, recorded endpoint present. Starting remains attachable before health completes. |
| `false` | Known generation, starting/running/idle, absent endpoint, and the exact preparation belongs to this API instance. |
| omitted | Unknown/legacy identity; an untracked missing endpoint, including a new API after controller loss; or terminal status. |

The API-owned preparation marker is armed before publishing the new allocation and removed by exact, idempotent deferred cleanup. A late cleanup cannot remove a newer claim. No persisted migration or backend poll is added. A new controller must retain the existing Backend.Get recovery path when endpoint persistence was interrupted. **The first pure-record endpoint predicate is superseded and must not ship.**

The frontend pauses only explicit false, displays **preparing**, and starts no socket, ticket or retry clock during that wait. True or omitted resumes an already eligible wait through the existing generation, scope, permission, terminal and overload gates. A cold cleaned session still needs the actual Restart acknowledgement. An established same-generation socket is preserved. An unmounted preparation wait remains deferred even when a ready replacement generation arrives; reacquiring preserves the terminal and opens the current generation once. This adds no global refs guard to initial acquisition or ordinary cached attachments.

## Executed evidence

All Vitest runs used one worker and captured unchanged source/configuration inputs before and after. Counts are tests, not assertions.

| Receipt | Outcome | Scope |
| --- | --- | --- |
| `frontend/run-before-1` | 8 pass, 17 fail | New 25-case regression against original production |
| `frontend/run-fixed-1` | 338 pass, 0 fail/skip | First candidate plus affected registry, poller, SessionPane/Restart, Terminal and store tests |
| `frontend/typecheck-1` | 1 diagnostic | Test helper used `number` instead of wire `KindValue`; retained original test and diagnostic |
| `frontend/run-before-final` | 8 pass, 17 fail | Exact final first-candidate test bytes against original production |
| `frontend/run-fixed-final` | 25 pass, 0 fail/skip | Same final test bytes against first candidate |
| `frontend/typecheck-2` | 0 diagnostics | Full desktop TypeScript 6.0.3 overlay; 745 unchanged inputs |
| `frontend-v2/run-before-unmount` | 26 pass, 1 fail | Same final 27-case test against first candidate; ready B opens once while unmounted |
| `frontend-v2/run-fixed-unmount` | **340 pass, 0 fail/skip** | Final successor plus affected tests; 447 unchanged selected inputs |

The type annotation correction has identical emitted JavaScript, bound in `frontend/typecheck-1/type-only-correction.json`. The first-candidate final pair was still repeated to bind exact source bytes. The final successor changes one boolean guard and adds two boolean-parameterized cases; its full TypeScript check was **not** repeated. The predecessor's successful full check is not represented as an exact final-source type check.

Coverage includes false/true/omitted readiness, no retry clock beyond three retry budgets, warm replacement with retained output, cold ACK before/after polls, unacknowledged cold polls, all five scope-field mismatches, typed Exit, permission revocation/refresh, unmount/reacquire/grace expiry/reset, pending retries, late async sockets, existing open streams, overload, and readiness-only store updates versus stale polls.

## Verify and reproduce

`SHA256SUMS` covers every file except itself. `publication.json` maps each retained file to its original path, original SHA-256 and stored SHA-256. Large raw logs and input/result JSON are losslessly gzip-compressed with deterministic headers; original byte hashes remain explicit. Run `sha256sum -c SHA256SUMS` and `python3 verify-capsule.py` from this directory for read-only integrity checks.

Exact runner/configuration files and source variants are retained. Their absolute paths identify the original controlled environment; do not overwrite those historical fixtures. For a replay, restore into a fresh external review directory, use the original repository at the functional baseline, explicitly map the captured paths to that directory, and generate fresh source bindings and result directories. The retained commands in each `command.json` describe the actual execution. The original-stage runner accepts `before` or `fixed`; the successor accepts `pre-unmount` or `fixed`. The shared test file for the earliest pair is retained at `frontend/typecheck-1/attachRegistry.readiness.test.ts.before`; later first-candidate and final tests are in their respective `tests/` directories.

The source-bound `package.json`, lockfile, Vitest and TypeScript configs are copied under `dependencies/desktop/` and verified unchanged from the first execution. npm's lockfile contains pinned dependency versions and integrity hashes. Publication-time package metadata and a hash of the Node executable are also provided; these are clearly labelled supplemental captures, not retroactive execution traces. The exact TypeScript-read inputs are retained in its input manifests. Dependencies and the Node binary are not bundled, and no install or new test was run during packaging.

Only source, controlled fixture outputs, configuration, hashes and tool metadata are included. No private installed stderr, browser/CDP record, runtime ledger, credentials, account data or process environment dump is included. Backend-v2 controlled evidence will be published separately.
