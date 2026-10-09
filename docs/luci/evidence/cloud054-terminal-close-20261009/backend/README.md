# CLOUD-054 backend close-order correction

External staged correction only; no repository file was changed by this work. Original product source is `d20ebd292f3f1de04d6726bab0998c210987bcb5`. The proxy keeps terminal payloads opaque and forwards an upstream WebSocket normal close before cancellation tears down the opposite pending Read. This uses the existing 1013 forwarding order; abnormal EOF and cancellation remain distinct, and fixed empty normal-close text prevents upstream private reasons crossing the relay.

Files for integration:

- `after/proxy.go` → `control-plane/internal/proxy/proxy.go`.
- `tests/proxy_test.go` → `control-plane/internal/proxy/proxy_test.go`.
- `tests/attach_test.go` → `control-plane/internal/api/attach_test.go`.

The tests use actual loopback WebSocket connections and the production proxy/attach HTTP handler. They assert intact final output and typed Exit frames followed by status1000, including exit-status0, nonzero and killed payloads. The relay does not classify any of those payloads as execution success. A normal transport close without an Exit frame must not synthesize one; abnormal upstream EOF must not turn into a normal close. Cancellation with an unacknowledged downstream handshake and cancellation without Exit both settle under the test's bounded deadline. The route regression keeps session status and sandbox Destroy count unchanged while balancing attach count.

The original production proxy with the new unchanged tests produced **three failed top-level tests and three passing controls**, plus three failed subtests: actual EOF after intact terminal frames and lost normal-close status were preserved in `receipts/before`. The fixed overlay passed the same **six top-level tests plus three subtests** with race detection in `receipts/after`. The affected proxy/attach batch also passed with race detection in `receipts/affected`; its outcomes include the original six tests and are overlapping coverage. Existing 1013 transcript-loss behavior, no destruction on client drop, authorization/origin/upgrade refusal and backend error mapping all passed.

`capture.py` records exact commands, toolchain, stdout/stderr hashes and selected input hashes before/after. These are source-bound controlled HTTP/real-WebSocket tests, not installed Docker/native Windows/provider results or full Go-suite/complete dependency-provenance claims. Both overlay mappings, original source and failed receipts are retained. The original sustained8 five exit-0/1006 records and intentional controller-restart interruption are unchanged; this regression proves the mechanism under controlled conditions, not the historical transport timeline for each installed closure.

No extra goroutine is introduced. coder/websocket v1.8.12 already bounds Close to five seconds for its close-frame write and five seconds waiting for a reply; parent-context cancellation closes the pending Read transport and unblocks that handshake. The new cancellation regression exercises that path with a peer that does not acknowledge the close. No sandbox destruction, task/state transition, resource release or integration is triggered by the relay change.
