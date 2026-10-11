# Linux package at 80462898

The official `scripts/build-release.sh 0.5.0 linux-amd64` completed in an isolated, clean detached checkout at `80462898735a191cceaa4cb33e486b42b0a35f3c`.

Use `installed/luci` with the immutable provenance receipt `verification-compact-final/verification.json`. Package identity: `4dccaf0d141ee12da9523bdb00affc3f2517dcb8e45652625e3502a8bfb2cf0c`. The intended compact archives are in `source/artifacts/releases/0.5.0`; their exact hashes and sizes are in `package-result.json`.

Static verification passed archive integrity, safe membership and modes, package identity, matching binaries, nested cloud payload, compact guide mode, and 347 license components with no missing entries. All 12,677 tracked source input hashes stayed unchanged. The original 577 archives under the root checkout remain unchanged. No service, Docker operation, or product test was run by this packaging task.

The first official build used stale cached branch tracking, so the official documentation rule selected the full guide/evidence payload. All four first-build outputs and their receipts are preserved unchanged in `full-doc/releases/0.5.0` and `receipts`. After an authorized fetch refreshed the actual published official branch association, the official script ran again against the same pinned source to produce the intended compact distribution. Both builds have the same binary and runtime package identity. These build durations are not product performance evidence.

The official Go binary has no embedded VCS revision fields. Source attribution is supported by the exact official invocation, clean pinned source bindings, release manifest, and artifact hashes. The first external verification attempt incorrectly required optional VCS fields; its original helper and failure receipts remain in place. The corrected helper records their absence and rejects any conflicting fields if present. Both package variants then passed static verification; no rebuild was performed for that correction.

`evidence-manifest.json` explicitly lists retained receipt/helper bindings and output bindings. The selected evidence omits duplicate verifier binary copies, caches, the installed tree, and the full source tree; those runtime/output inputs are separately bound by archive inventories and tracked-source manifests. `SHA256SUMS.evidence` covers the selected receipt/helper files plus this manifest. Generated task-owned caches were removed; their cleanup receipt is an environment record, not a product result.
