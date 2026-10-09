# CLOUD-054 integrated dependency source audit

This is a read-only audit of the root agent's uncommitted dependency update at base `a61ac242da77edb6a5a1dead793487ca7a8d646f`. No tests, application, browser, Docker workload, or repository edits were performed in this audit. The captured 36 selected repository inputs were unchanged before/after.

Both control-plane and sandboxd pin github.com/coder/websocket v1.8.15 and contain each expected upstream module/go.mod checksum exactly once. Retaining older go.sum entries is not a pin. All 30 vendored coder files exactly match the freshly sum.golang.org-verified upstream module. Offline `go mod vendor` from a separate sandboxd source copy yields exactly the same file set and bytes, including added errors.go, hijack.go, Makefile, .gitignore, AGENTS.md and the changed licence. No missing, additional or different coder files were found; vendor/modules.txt is also byte-exact. Upstream AGENTS.md is captured third-party data and was not adopted as local instructions.

The licence is exactly upstream LICENSE.txt (720 bytes; SHA256 cc0975a5f6305145bdd7b41ce9479632fdac3870e6ac4281f28017f18c767c4e). The integrated close_ack_test.go matches the independently tested candidate exactly (6146 bytes; SHA256 2cf7e44be58a0b95ae3239059ded4c069a0c9dbca35f73f4707107c74d61e05f).

## Reviewable worklist claim

CLOUD-054 remains the same issue. The installed v8 failure retained a fixed Chromium protocol diagnostic classified as duplicate-close-received, after typed Exit 0. The old v1.8.12 dependency replies to a close ACK with a second Close frame. An independent raw RFC6455 client reproduced the exact duplicate post-ACK normal frame 88 02 03 e8 through the real PTY/API/owner route. Upstream v1.8.15 tracks close state and prevents that duplicate. Both module pins and sandbox vendor now match the verified upstream source; no local fork is introduced.

The same strengthened raw PTY regression fails under old dependency (one top-level/two subcases) and passes under v1.8.15. The portable tracked production-relay CI regression likewise fails under old dependency (one top-level/three subcases) and passes unchanged under v1.8.15, covering normal exit 0, nonzero exit and overload. Thirty-one named affected race outcomes passed separately. These counts describe their respective scoped batches, not a full-suite result or an installed run. Root's subsequent integrated tests and compilation require their own receipts.

A new matching packaged control plane and rebuilt sandboxd runtime/browser run is still required. Earlier Go-aware clients and raw tests stopping immediately after ACK did not cover the duplicate boundary. Earlier controller status 1000 does not establish downstream handshake success. Native Windows, provider quality, performance and wider recovery acceptance remain open. Original failed fixtures/receipts are unchanged.

## Evidence

- audit.json: exact selected source/upstream/vendor regeneration bindings and results.
- before.json and source-snapshot/: immutable selected point-in-time inputs.
- vendor-regeneration-command.json and logs: external offline vendor generation only, exit 0.
- Existing frozen before/after source-fix capsule: /workspace/.luci-tools/cloud054-duplicate-close-edd0/publication-capsule (manifest SHA256 285d1574315f8f060cdac83c1aa0573cef7aed4425169d6cae0dd4303b0b4b20).

Private arbitrary transport text, tokens and URLs are excluded. The earlier installed raw protocol-error receipt remains private; the public claim is the narrow fixed diagnostic classification and controlled source reproduction.
