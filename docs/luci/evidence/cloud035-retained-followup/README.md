# CLOUD-035 followup: retain the shared canonical export contract

The first CLOUD-035 correction removed `canonicalNames` during seeding but missed its use in `retained()`. The fresh installed worker attempt with product `d348f0127a920aab05b54a4192baa8ed3800fe54` and helper `c322398ce5645236866cfc7b1bad13cd94bdcc99` consequently failed with `ReferenceError: canonicalNames is not defined` while recording output. This was introduced by the helper correction. The [actual failure excerpt](actual-failure-excerpt.json) retains its precise source, failure and reported cleanup; the original result and private fixture were not changed by this followup.

The adapter now imports `projectExportPaths` from its existing context helper and uses those seven persisted export paths in `retained()`. It does not create a checkout `.luci/brief.json`, which remains exclusive to the reviewed capture. The change adds no runtime module or product behavior.

The same regression executes the actual `retained()`, `bundleSnapshot()` and `transcriptFiles()` functions against two real temporary Git repositories and actual controlled export, capture, output and native transcript files. It failed with the original `ReferenceError`, then passed after the adapter fix. It checks exact canonical/capture hashes, primary and task commit/status, output/observation hashes and transcript prefixes. Further cases show changed content and commits in the snapshot, allow appended transcript events while retaining the original prefix, and refuse changed prefixes or missing captured files. Neither writable repository contains a brief JSON file.

The initial test preparation hit a host/VM array-prototype mismatch before the target failure. Its unchanged test and log remain as `initial-realm-test.mjs` and `preparation-realm-failure.log`. Normalizing plain array values at that assertion boundary fixed the test harness; the corrected test bytes are identical in both before/after regression runs.

Evidence:

- [Before regression](before-regression.log): one test, one `ReferenceError`, exit 1.
- [After regression](after-regression.log): the same test, one pass, exit 0.
- [Targeted batch](targeted-tests.log): 14 passes, including retained state, seeding/capture contracts, inert adapter and complete runtime source binding.
- [Before undefined-name check](before-undefined-names.log) and [after check](after-undefined-names.log): existing TypeScript 6.0.3 resolves the full adapter's names without emitting files. The historical source reports only `canonicalNames` unresolved; current source reports none. This is explicitly not a complete JavaScript type check.
- [Validation](validation.json), [original hashes](before.json), `before/`, `tested-sources/` and [checksums](SHA256SUMS) preserve exact commands, source bytes, bindings and limits.

No installed server, Docker worker, browser, model call or build ran in this correction batch. It does not establish ordinary Stop, controller restart or exact capacity release. LOCAL-016 still requires a fresh installed attempt with the newly committed helper pin.
