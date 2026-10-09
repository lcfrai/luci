# Scoped independent source review

Review scope: the actual Home/Define/Rail DOM contract, the external autonomous-readiness observer, `prepare.py`, `integration.patch`, and `observer/readiness-ledger.mjs`. This reviewer performed bounded source reads only. No product/source changes, tests, browser, installed app, benchmark, or runtime were run by this reviewer. This note is the only review artifact written.

The witness is deliberately limited to rendered selected-project ID plus a fresh editor with the expected saved value and native enabled state. It does not establish server provenance, current execution authority, full actionability, paint, or first possible readiness. The reviewed integration preserves the original interaction helper, action sequence, two-frame endpoint and 300 ms full-action threshold; no observer interval is subtracted.

## Findings and resolutions

The initial review identified overflow cleanup retaining listeners/private comparison data; A→B→A reuse of a historic editor; missing editor/Definition-section containment and heading checks; observed root/editor disconnection or replacement being tolerated; final-round cancellation being disabled too early; an unconditional timer after synchronous sampling had already finished; and click timestamps captured after observer work. The follow-up source read confirmed the corresponding cleanup, WeakSet, containment/heading, observed-disconnection/identity, final-round, timer and timestamp corrections were present.

The final narrow integration review identified four remaining failure paths:

1. A missing observer snapshot was silently accepted, including during close.
2. Overflow advanced the sequence for a discarded row, causing the ledger to reject and lose the overflow receipt.
3. Overflow during sampling could close the observer while the caller continued arming a new stage.
4. A timed-out installation could have installed the observer while leaving the driver’s started flag false, skipping explicit readiness cleanup.

The implementation owner subsequently reported all four corrected: missing snapshots are explicit and throw; sequence advances only for retained rows; arming checks closed state before and after finishing; and installation is marked attempted before awaiting setup, with bounded cleanup and an explicit uncertain-installation outcome. The owner also reported clearing private configuration references and recording the terminal sampling blind gap. These final changes were reported to this reviewer, not independently re-read or executed here; no further review was requested.

## Validation attribution and limits

The implementation owner reported final controlled browser batches of 12 and 13 checks passing, plus six ledger checks passing. Those are owner-reported results, not tests performed by this reviewer. Earlier attempts, including the initial browser socket-path failure, remain separate evidence and must not be relabelled as cumulative unique passing checks.

No source-only blocker remains from the findings above given the reported final resolutions. This scoped review does not certify an installed diagnostic result, production performance improvement, or zero observer overhead. The candidate’s exact source bindings, retained raw validation receipts and final publication manifest remain the implementation owner’s evidence.
