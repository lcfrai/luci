# C080 product extra-label contract correction

Separate follow-up to the original frozen product/platform adapter capsule. Remove the caller-supplied `luci.preview` mandatory ownership label from the product runtime request. The backend assigns that label. Retain the release/project extra labels.

The meaningful regression uses the existing committed safe source → admitted release → health and ownership verification journey, with a new assertion on the actual typed request labels. Before: the assertion fails and the release returns HTTP 422 instead of 201 (underlying exit 1). Fixed: the same journey passes (underlying exit 0, one top-level test). Each run captured 971 source/config/module/embed bindings before and after; they were unchanged. Go 1.26.3, GOMAXPROCS=2, package parallelism 1, default vet, offline modules, 120-second test and 180-second wall bounds; no timeout. Complete invocation and raw logs are retained as exact gzip-compressed bytes.

The tested overlay archive is the complete fixed external Go overlay, with member hashes. Repository sources are bound by the receipt manifests and HEAD. The before overlay differs only in releases.go as recorded in original-adapter-binding.json; the new assertion is used in both runs. No source outside the external staging trees was edited by this check.

Apply the original frozen adapters.patch before this two-file patch. This capsule does not replace or modify that earlier evidence. These are controlled tests, not an actual Docker launch or a full-suite run. Base-image provisioning, uncertain-build recovery and the real Docker product journey remain separate integration gates.
