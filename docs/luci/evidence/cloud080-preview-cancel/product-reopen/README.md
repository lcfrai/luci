# CLOUD-080/082 interrupted cancellation before allocation — narrow follow-up

A cancellation intent can be saved before any runtime allocation identity or held-resource flag exists. If the controller exits before final publication, the old observer skipped this inactive `cancelling` row and kept returning `cancelling`; the UI could keep polling indefinitely. The same gap affected a finished builder before the producer published its final outcome.

The observer now includes inactive `cancelling` in its read-only interruption projection. It returns explicit `unknown` with no live cancellation handle, says the retained outcome/any reservation is unconfirmed, and does not infer settlement, release capacity, replay a build or rewrite the original receipt. Legitimately active cancellation remains `cancelling`; confirmed `cancelled` remains terminal.

The same four-case race regression runs before and after the narrow fix. Before: the before-allocation and finished-builder cases fail; active-cancellation and confirmed-cancelled controls pass (parent test fails). After: all four cases plus their parent pass, exit0. Each run captures800 unchanged Go/module inputs, exact invocation and complete underlying output. Every history read is repeated and checked against the unchanged durable receipt; runtime Start/Stop are forbidden by the fixture.

Go1.26.3, GOMAXPROCS2, -p1, -race, default vet, readonly modules,100-second package timeout and150-second wrapper bound. Only `TestPreviewCancellationReopenDoesNotLeaveInactiveNoDemandCancelling` was selected. No broad suite, browser or Docker workload was run. The preceding [product87-outcome run](../product/README.md), its original failures and the separate API integration receipt remain unchanged; counts are separate coverage across runs, not a new combined suite pass.

Exact original/fixed implementation and identical test bytes accompany each run. Input inventories are losslessly gzip-compressed; `compressed-input-bindings.json` records their original byte lengths and SHA256. The patch contains only the observation correction and its dedicated regression. Installed/native crash/reopen acceptance remains open.
