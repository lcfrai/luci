# Verification and reproduction limits

Run `sha256sum -c SHA256SUMS` inside this capsule to verify its public members. `original-bindings.json` records the exact small input bytes. The binary archives and extracted payloads are excluded, so public receipts alone cannot independently repeat their private-byte verification.

The original package command was `bash scripts/build-release.sh 0.5.0 linux-amd64`. The retained wrapper adds clean/retrievable source checks, exact prior-release preservation, supported task-local tools, a bounded subprocess and the original package verifier. A fresh execution has side effects and requires the exact previous 29931afc release, b94ad37e on a remote-tracking ref, distinct new destinations and enough space. Do not replay against the already-created release. Private historical tool paths are provenance, not guaranteed fresh-clone locations.

The wrapper's `manifest.json` describes its original preparation-only state. Its unchanged historical controls remain evidence for that wrapper; the new build result separately proves the actual successful b94ad37e invocation. The environment supplement does not rewrite the wrapper's own recorded fields. No new tests or builds were run to prepare this publication.

Reassembly with `assemble.py` requires the retained original small files at their historical paths and an unused output directory. It never builds, loads Docker images or reads the large package/archive payloads. The five selected tracked scripts must still match the exact immutable source.

Use the repository's supported isolated Windows instructions for local testing. This Linux packaging helper is not a safe substitute for those instructions on Dylan's shared Windows host. No credential values, private fixture state, raw controller logs, unredacted private wrapper arguments or whole-machine cleanup claims are included.
