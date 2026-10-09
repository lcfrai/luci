# CLOUD-041: bounded native command receipt

This is a helper correction and controlled validation capsule. It does not pass the interrupted sustained workload, native session completion, Windows acceptance, live-provider quality or responsiveness gates.

The original installed attempt used product `2da1a821b8456c20a858fb86ef87649ea917d9de` and helper `abd2d8c9314d6b73d0c202c82de72f3c3acc0d9a`. Its complete native command stdout contained an 8,180-byte JSON observation encoded as a 10,908-character line. The actual provider input retained 10,211 UTF-8 bytes, reported truncation and command exit 0, and omitted the receipt prefix. The old provider callback consequently refused it. Exact original tool output, full stdout, complete observation and initial/running outputs are tracked in `scripts/acceptance/sustained-four-project/fixtures/native-*`; `triage.json` and the input manifests bind them. The complete private native rollout is not published.

The original command requested **16,000 output tokens**. Its effective truncation configuration remains unidentified. The two retained output forms share exactly the final **5,000 characters**; this is measured evidence from that output, not a general SDK truncation guarantee. The full original stdout and observation remain unchanged. Original accepted task results remain **zero**, and no natural native-session exit was observed. The underlying command did exit 0 with 150 active fuzz seconds and 6,846,700 executions; that does not retroactively make the failed acceptance pass.

The correction emits the same complete observation as deflate-raw + SHA-256 + canonical base64. The actual generated emitter produces a **2,651-byte wire line / 2,652 bytes with newline** for the retained 8,180-byte observation, well inside the observed 5,000-character suffix. The generic transport refuses wire lines over 4 KiB, decoded data over 64 KiB, malformed/multiple/unsupported markers, noncanonical encoding, trailing compressed bytes, decompression bombs, digest mismatch and wrong task/operation. Receipt bounds fail closed if a future observation does not fit. Workload artifacts and full command logs are unchanged; the generated scratch cleanup policy is unchanged. Diagnostic output, 150-second fuzz workloads, 16,000-token requests and 48 task-tool calls are unchanged.

An early receipt remains pending until the **same native command** reports exit 0. The driver records tool/call/command identity and never re-executes to recover a missing receipt. Receipt replay under a different task, command or consumed call is refused; the existing identical verified final-response retry remains bounded. Full task/context/permission/output validation remains in the actual provider callback. Command-status authority comes only from the pinned native textual header before `Output:`; body text that quotes an error or exit status cannot supply or override status. The exact initial exec, running `write_stdin` and final formatted output contracts are bound to retained fixtures. Legacy loose JSON status forms are deliberately unsupported and refuse conservatively. No native-session completion is inferred from this command header.

Validation on Node 24.19.0:

- Final affected run: **51 passed, 0 failed, 0 skipped** across receipt transport (13), generated task/fuzz parser, existing helper, actual lifecycle wiring and source bindings. Eighteen selected local source/fixture inputs were identical before/after; the additional unchanged C031 fuzz fixture has a separately captured publication binding. This is selected helper scope, not a full application suite or environment-wide input inventory.
- Pure old/new provider-callback replay reproduces the original retained failure verbatim. The corrected callback still refuses the original truncated output. Replacing only the old emitted marker with the new generated envelope and applying the measured retained-edge envelope yields one accepted **controlled callback result**, one dispatch, exact command 39517 and exit 0. It starts no native process or workload. Its safe settlement metadata is included in the replay receipt.
- Actual generated emitter code is executed in a VM against the complete retained observation. Pending receipt, correct later exit, nonzero exit, changed identity, changed context, revoked read-only assertion, duplicate final response and benign error text are exercised through the actual extracted provider callback using inert HTTP objects.
- Syntax checks for the driver, command generator and transport pass. No Docker, browser, provider/model or application runtime was started for this correction.

The final 51 outcomes include the earlier 12 transport outcomes; do not sum them. `runs/transport-attempt1.*` preserves a test syntax failure. Attempt 2 preserves six passes and one overly specific trailing-stream assertion (the original larger stream correctly hit the wire bound first); the corrected test uses small concatenated streams. The original callback replay's first attempt hit a VM cross-realm empty-array assertion after the replay; its script and error are preserved separately. These are test/diagnostic defects, not new installed product failures.

Reproduce from the repository root:

```bash
node --test --test-reporter=tap \
  scripts/acceptance/sustained-four-project/receipt-transport.test.mjs \
  scripts/acceptance/sustained-four-project/task-command.test.mjs \
  scripts/acceptance/sustained-four-project/helper.test.mjs \
  scripts/acceptance/sustained-four-project/lifecycle.test.mjs \
  scripts/acceptance/sustained-four-project/source-binding.test.mjs
node docs/luci/evidence/cloud041-bounded-command-receipt-20261009/reproduce-original.mjs
```

`before/` contains exact original source; `after/` freezes the corrected four sources. `SHA256SUMS` binds every other capsule file. `publication-bindings.json` binds all five exact retained fixtures and the existing fuzz fixture without duplicating them here. A fresh matching installed workload remains required.
