# Combined frontend checkpoint: cleanup display and draft registration

9 October 2026 (Brisbane). This combined source run has **2,713 passed tests, zero failures/skips**, across 156 actual Vitest file results. Nested runner suite counts are not file counts. All 434 captured frontend/configuration inputs were unchanged during the run. Base checkout: `9121b5bab09df5827efa291026ed842253256842` with the explicit integrated CLOUD-033 and draft-registration changes bound in source-before.json.

The subsequent TypeScript check **failed** with TS2550: the new `Object.hasOwn` call requires a newer standard library than the configured target. This is a retained implementation compatibility error, not a passing typecheck. A separate exact follow-up uses the supported equivalent call; do not replace this original receipt or infer its outcome here. These results precede the CLOUD-034 reservation-visibility frontend change.

Commands, elapsed time, original stdout/stderr, exact source hashes and test report are retained. vitest.json.gz decodes to the unchanged original JSON, with both hashes in capture-bindings.json. The canvas-not-implemented fixture warning remains in stderr and did not fail the run. No browser, installed package, Windows, provider or performance acceptance is credited.

The capture wrapper itself returned zero after writing both command outcomes; it did not aggregate the failing typecheck into its own exit code. This wrapper limitation is retained with its exact source. The underlying `tsc` exit code 2 is explicit in commands.json/result.json and is not a passing validation result. Future orchestration must propagate aggregate failure.

LOCAL-017 publication correction: four original stdout/stderr log files were initially omitted from Git by its log ignore rule. They are now explicitly tracked, with unchanged declared hashes and byte counts. No test rerun or reclassification of the original typecheck failure occurred.
