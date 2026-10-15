# Distribute and contained Discover source join — 16 October 2026

The branch joins the reviewed Distribute source through `1447a6c6` with
`main` at `798aaba3` (merge `eb40b516`, PDF plan successor `e3ae1d6f`).
No Docker, OCR image, model, provider, renderer dependency or local desktop
runtime was started for this receipt.

Working directory: `control-plane`. Command (one serial Go process):

```sh
set -o pipefail
GOWORK=off GOMAXPROCS=1 GOMEMLIMIT=512MiB GOGC=20 \
  GOCACHE=/dev/shm/luci-go-cache GOMODCACHE=/dev/shm/luci-gomodcache \
  TMPDIR=/dev/shm GOTMPDIR=/dev/shm \
  /dev/shm/luci-toolchain/go/bin/go test -p 1 ./internal/api ./internal/product \
  -run '^(TestDocumentSharedLedgerRetainsExactOwnershipAndJointBound|TestContainedDiscoveryAcceptsOnlyExactResultAfterPositiveRetirement|TestDistributeArtifactPrepareSaveReviewAndSourceCAS|TestDistributeArtifactLegacyXMLScalarReadReviseAndExportRefusal|TestDistributeArtifactSaveRefusesXMLIncompatibleSavedSource)$' \
  -count=1 -timeout=120s -v | tee /dev/shm/distribute-extractor-join-go.log
```

Exit 0. The API ownership/joint-bound case passed; the product positive
retirement case and its eight refusal subcases passed; all three selected
artifact save/review/XML cases passed. Complete bounded output is
[`joined-go.log`](joined-go.log), SHA-256
`da10c54a607b0726c1ca2f3cd395219401516d8531c798182c67df3681aa5267`.
The unchanged broader extractor and artifact suites were not rerun for this
join. This is source-level evidence only; installed extraction, export fidelity,
host memory and real user workflows remain open.
