# Matching Linux package at 577f7a0f

Exact product source: `577f7a0fdb039b7f875901a6a567d62fdfee88c8`, clean before/after both builds. The second supported `scripts/build-release.sh 0.5.0 linux-amd64` run passes in 64.268 seconds with GOMAXPROCS=2 and readonly single-package Go compilation. It stages 56 compact guides, 347 licensed components and no missing licences. The initial 105.409-second build also passes but retains the full documentation archive because the local remote-tracking ref was stale after push. Fetching the exact published branch resolved that association; no source or packaging guard changed. Its full-doc archives were renamed and hashes reverified before the repeat. Times are build observations, not a controlled speed comparison.

Actual final archive/extraction verification reads all 967 portable files, recomputes the binary+web package identity and matches all 106 packaged runtime source files to the immutable Git revision. Selected packaging/dependency files are separately bound; this is not a complete compiler/environment inventory. The earlier image still needs its separate runtime provenance checks. No Windows/native, real image build, live provider, preview or latency result follows from archive verification.

- Portable Linux: 69,530,944 bytes, SHA256 `98d8bea09b1476a09a290a6b53567f9ecdca8ced32099dbdcbf3b735a88b07ba`.
- Cloud Linux payload: 34,858,383 bytes, SHA256 `28c61142c1e3c630604d8837b1926a50b38916ce3a930e5efef1e18ddabebffa`.
- Package identity: `6d69e5cebb2af7cbcf5f0c1ca12063b86f71d8f968cbffa007a40447a147592e`.

Original logs/receipts, archive-member hashes, runtime source bindings and verification helpers are published here. Large executable archives and extracted bytes remain in this cloud workspace, not Git; another machine can retrieve the exact source and use its documented native/portable builder. The source handoff is [runtime/preview checkpoint](../../TEST-CHECKPOINT-RUNTIME-PREVIEW.md).

The first invocation used an inline bounded wrapper recorded in command/result metadata; no reusable source file for that wrapper is claimed. The second wrapper and extraction verifier are included verbatim. Existing prior 972d package metadata was retained before the first build; that invocation reused the conventional artifact output directory and did not archive its old output files. Historical installed fixtures and their state/ledgers were not touched. Neither build launches application or Docker processes.
