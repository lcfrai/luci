# CLOUD-084/088 guest operation ownership correction

The original guest transport returned from `Controller.Guest` without registering subsequent uploads/downloads with controller shutdown. A deterministic regression against the exact retained source reproduced three failures: `Controller.Close` reported success with an old upload still held; a replacement controller acquired the state directory; releasing the held transport forwarded one old upload after that ownership release. This was a controlled pre-POST transport barrier followed by an actual in-process TLS fixture, not an E2B deployment or observed production incident.

The correction registers guest upload/download/readback/Start operations and attach handshakes with both the guest transport and its owning controller before effects. Caller, guest-close and controller-close cancellation propagate to the operation. Closing the guest prevents new registration and waits for accepted operations; the controller retains its journal/checkpoint lease until its accepted client work returns. Body verification/storage after HTTP headers is included in that lifetime. Returned WebSockets retain their documented caller ownership after the handshake.

**Cancellation is not remote settlement.** A failed/lost upload or Start remains uncertain, no command is replayed, and VM capacity remains held. A handler that already accepted a request may still finish remotely; this fix does not certify its absence or zero side effects. Input hashes and the trusted bootstrap's materialized-file checks remain necessary. Arbitrary blocking readers cannot be preempted by a context: shutdown may remain pending with state ownership held instead of claiming a successful drain.

## Evidence

- `original.jsonl`: the original top-level regression fails with the three concrete assertions above; Go exit1. All40 declared selected repository inputs stayed stable.
- `fixed.jsonl`: complete `e2bbe` race suite, **54 top-level and73 subtests passed**, no failure/skip, exit0. All42 selected repository inputs stayed stable. Counts overlap the earlier guest transport checkpoint and are not additional unique coverage.
- The exact original regression is byte-identical in the fixed run. Additional tests cover response-body drain after headers, already-cancelled requests starting no I/O, and guest close during pending Start retaining the one-shot fence and exact VM charge.
- Original source/fixture copies, changed final sources and original/fixed command/environment/input receipts are included. The selected manifest covers e2bbe, direct backend/statelease and module/workspace files, **not every external compiler dependency**.

Reproduce using supported Go1.26.3 from `control-plane`: `GOMAXPROCS=2 go test -race -count=1 -p1 -timeout 2m ./internal/backend/e2bbe`. For the original case, use an isolated checkout/overlay with the retained `before-guest.go`, `before-grant.go`, `before-controller.go` and regression, against the parent transport checkpoint. The additional corrected-only files must follow the original source baseline when reproducing it; do not mix the fixed operation helper with old struct fields. Preserve existing work instead of overwriting a running checkout.

No provider credential, cloud VM, model, Docker runtime, Windows native test or latency workload ran. The original transport checkpoint remains immutable historical evidence. Live provider/template/kernel protection, output production/quiescence and backend/API/scheduler integration remain open.
