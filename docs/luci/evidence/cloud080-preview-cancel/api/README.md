# CLOUD-080 / CLOUD-082 API cancellation bridge

The focused registered HTTP/API regression passed with the Go race detector on Linux: **1 pass, 0 failures, 0 skips**, exit 0. Its body took 1.53 seconds; the bounded invocation, including compilation, took approximately 29 seconds. No test was repeated to create this publication.

The fixture uses `API.New` to register Product and wire its preview runtime to the actual API lifecycle. Two reviewed, committed temporary projects share one Product/API, durable registry and execution ledger. The fake executor holds the selected `BuildPreview` return after observing cancellation. Current deployment authority is a controlled lookup behind the actual trusted `WorkerHandler` identity boundary; its explicit two-build policy permits the independent peer. This is controlled integration evidence, not a real management authorization-service exercise.

The test establishes:

- Current exact-release `expectedRevision` cancellation returns HTTP 202 before the held backend returns and reaches the real API build context despite its release lifecycle lock.
- An independent release keeps its live context and unchanged cancellable receipt.
- An unknown build retains its exact lease identity, phase and 1024 MiB / 1000 mCPU charge. The independent settled build retires only its own charge.
- A newly opened Product/API and registry retain the unknown receipt and charge. Refreshes and repeated HTTP 409 cancellation refusals do not replay a build or issue Create, Start, Stop or Remove.

Only `control-plane/internal/api/preview_cancellation_http_test.go` was added by this work. The production fix and its original/fixed product regressions belong to the adjacent evidence. This additional API test was not executed against original production source.

The invocation used Go 1.26.3, `GOMAXPROCS=2`, `-race -p 1 -count=1 -timeout=90s`, a 300-second outer bound, and storage floors of 1.25 GiB on the root filesystem and 256 MiB on `/tmp`. Controlled publisher barriers have 10-second waits and bounded cleanup joins; HTTP request contexts have 30-second bounds. Exact command, timestamps, minimum headroom, JSONL and empty stderr are retained.

The working tree was dirty at HEAD `871ddaa9ea0158c800a20b52be588b8e5df66387`; before/after status is retained. All **994** files in the conservative repository Go/native/module/embed inventory and `go.work` remained unchanged during the run. The inventory includes both active modules, untracked additions, and files outside the selected compiled dependency closure. It is not an exact compiler dependency list. External dependencies are bound through module manifests, not individual cache-file hashes. The process watcher did not retain an executed test-binary hash; no binary-identity claim is made.

`evidence/*-sources.json.gz` losslessly compress the original manifests. `evidence/selected-source.tar.gz` contains the 14 selected source snapshots, with individual hashes in `evidence/selected-source.json`. `compression-receipt.json` records compressed and uncompressed hashes; both decompression and archive member bytes were verified. The exact test fixture and produced patch are included separately. `SHA256SUMS` covers every publication file except itself.

No real Docker allocation or cleanup, browser, installed application, preview serving health, native Windows execution, or performance claim is established. The test runs native Git only in its owned temporary project fixtures. Publication performed no runtime action, test repeat, staging or commit.
