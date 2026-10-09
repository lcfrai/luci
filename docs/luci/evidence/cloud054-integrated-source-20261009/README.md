# CLOUD-054 integrated dependency and wire regression

Root integrated published coder/websocket v1.8.15 in both modules and regenerated sandboxd vendor with normal Go tooling. The portable raw-wire regression is byte-identical to the external before/after candidate. It requires one server Close, peer acknowledgement, then transport EOF for exit-zero, exit-nonzero and overload; private upstream close reasons remain hidden.

The actual repository proxy package passes all 15 named race outcomes (top-level plus subtests; overlapping the earlier selected external suite). Both control-plane and sandboxd compile all packages with `go test -run ^$ ./...`; this is compilation, not a full test pass. All 928 captured repository Go/module/vendor inputs remain unchanged across this batch. The compressed complete repository input manifests omit external compiler/generated inputs and do not claim whole toolchain provenance.

Raw commands, exit codes and logs remain here. Source changes include an upstream licence-preserving vendor regeneration, not a maintained dependency fork. The external capsule preserves identical before/fixed wire regressions and fresh checksum-verified upstream source. Matching installed Chromium/runtime-image and native Windows verification remain open. No new package identity or transport success is inferred from these source checks.
