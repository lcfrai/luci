# CLOUD-031: Go fuzz progress duration parsing

The installed sustained attempt `sustained-4df94ab9-helper-5d306b6d-attempt3` failed at `active-command-restart` after its first Go fuzz command exited successfully. The retained output advances from `57s` to `1m0s` and ends at `2m30s`, with 6,993,018 executions. The original helper matched only durations ending in seconds without a minute component, reporting 57 seconds and 2,560,985 executions, then failing the unchanged 150-second assertion.

This capsule preserves that original failure and demonstrates the parser correction against its exact output. The original installed run remains failed. These bounded tests do not establish a task commit, review, restart, amendment, or successful four-project sustained run. They execute no Docker operations, browsers, Go builds, fuzz commands, or model calls.

The product source is `4df94ab9dad0e9fedd28a873bbadc56720e1402e`; the failed run's helper source is `5d306b6dae3cc9d0a4d9cbf95f8142c37958964f`. `manifest.json` records source hashes, commands, counts, and limitations. `input-binding.json` identifies the original receipt and accepted helper. `before/task-command.mjs` is the exact accepted helper; `after/` contains the fixed helper and its bounded test dependencies.

The correction parses whole-second Go duration components, including minutes and hours, in execution-progress records. It preserves maximum elapsed time and execution count, rejects malformed or unsafe values, and keeps the existing output bound. The generated worker embeds the same parser exercised by the tests. Command success, the wall-time minimum, positive executions, at least 150 seconds of progress for each initial worker, and the aggregate 600-second requirement remain unchanged. Wall time, baseline coverage, and package summaries do not establish active fuzz time.

The before regression executes only the exact original generated post-fuzz assertions in a bounded VM with the retained output and recorded command timing. It fails with the original 57-second value. The corrected tests exercise both the exported parser and the actual generated post-fuzz assertion block. Coverage includes seconds, minutes, hours, the final line without a newline, CRLF, maximum values, malformed components, numeric and output bounds, and refusal to substitute wall or package duration.

Recorded results with Node `v24.19.0`:

| Command | Result | Raw output |
| --- | --- | --- |
| `node --test /workspace/.luci-tools/cloud031-fuzz-duration/before-regression.mjs` | Expected exit 1; 0 pass, 1 fail | `before.log` |
| `node --test /workspace/.luci-tools/cloud031-fuzz-duration/after/task-command.test.mjs /workspace/.luci-tools/cloud031-fuzz-duration/after/helper.test.mjs` | Exit 0; 9 pass, 0 fail | `after.log` |
| `node --test scripts/acceptance/sustained-four-project/task-command.test.mjs scripts/acceptance/sustained-four-project/helper.test.mjs` from repository root | Exit 0; 9 pass, 0 fail | `repository-tests.log` |

For a local reproduction from this directory, use `node --test before-regression.mjs` (expected failure), then `node --test after/task-command.test.mjs after/helper.test.mjs`. These equivalent relative commands were not rerun during publication. Verify capsule bytes with `sha256sum -c SHA256SUMS`.

`original-private-failure.json` is the unchanged retained receipt; its name denotes its origin in the fixture's private failure diagnostic. Its fields contain the synthetic task, command and event records, 79 generated synthetic corpus entries, and six bounded command logs. Every encoded file was decoded and checked before publication; no account, provider, or credential records are included. `original-fuzz.log` and `after/fixtures/go-fuzz-progress.txt` contain identical 4,316-byte decoded output with SHA-256 `506a10848d01b2bb297279b64634aef1bf641c3dcef52080f5683c8cf99161aa`. The worker recorded Go `go1.26.8 linux/amd64`.

The diagnostic output and generated corpus come from this repository's synthetic fixture; no external source code or new dependency was copied. The repository's existing license applies. Directory metadata preserves raw log bytes and includes them in ordinary Git adds and checkouts. `SHA256SUMS` covers every capsule file except itself, including that metadata.
