# CLOUD-065: restart an initially cleaned terminal

Base source: `5f4a966f6d70264c92ee2176dde912e1b43fc3c7`. Externally staged; repository unchanged.

A current exited session legitimately omits its erased allocation fingerprint. Opening its pane and using Restart changed the header/store to running but never opened a terminal socket. Original production with the final real SessionPane + Terminal regression: **5 failures,11 passing controls**.

The correction captures the exact cold-ended registry entry before either UI Restart request and accepts only a successful same-scope live response with a valid fingerprint. Pending requests, errors, missing fingerprints and late live polls alone cannot attach. Matching acknowledged polls can be consumed directly while mounted because the store deduplicates identical responses. Existing authority/typed-exit/scope/teardown guards still apply; old/repeated acknowledgements cannot replace a later marker.

**Verification:**251 scoped tests across7 files passed, followed by42 final tests across2 affected files (40 overlap;253 distinct latest outcomes). Final TypeScript check passed. Original final regression bytes are identical between original/fixed runs. Full439 input bindings are supplied; broad251 binding is explicitly reconstructed from preserved intermediate exact files, with the changed guard and final tests verified by the final42 report. Counts are not additive.

Preserved intermediate evidence includes the new-fingerprint poll race, repeated-ack race, missing mock export, corrected shutdown transport fixture and invalid test-option TypeScript errors. See report.json for exact scope/counts.

Reproduce in desktop after applying cloud065.patch: `npm exec vitest -- run src/api/attachRegistry.exit.test.ts src/components/develop/SessionPane.restartAttach.test.tsx --maxWorkers=1 --no-file-parallelism`, then `npm run typecheck`. Dependencies are the exact repository package-lock; Node24.19.0. No browser, app, Docker or live model was used. These controlled results do not pass installed/native Windows acceptance. The marker ordinal is not a general competing-HTTP-response store fence.
