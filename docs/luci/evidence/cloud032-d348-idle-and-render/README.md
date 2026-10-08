# CLOUD-032: d348 idle result and subsequent render reduction

The fresh installed **d348f012** diagnostic completed all400 primary actions (100 switches,200 inputs,100 separate draft resets), but **both original latency targets failed**: switch p95 **400.460745ms** against300ms; input p95 **132.942794ms** against100ms. `actual/result.json.gz` preserves the exact original result bytes. Its `passed:true` means diagnostic completion, not target success or full sustained acceptance.

The later source change avoids three unused definition-panel child constructions per render and prevents team-context broadcasts when only draft registration changes. It retains all four accessible tab panels, current draft reads at navigation time, fresh specification reads, and revision/permission revalidation. **Its installed-app latency improvement is unmeasured.** The d348 package does not contain this new source change.

Validation of the final source: DefinitionWorkspace **14/14** tests passed; TeamWorkspaceProvider **28/28** passed, including two new consumer-render/current-draft/authority-refresh regressions; TypeScript and owned-file diff checks passed. `product/product-validation.json` records commands and source hashes. `product/before/`, `product/after/`, source-before/source-after manifests, and the patch make the exact change reviewable. The baseline commit is6e91c319901ee912dc02b562e3c3e4e449e9ce4e; subsequent evidence commits do not turn these files into a measured package. Test logs may contain harness diagnostics; test outcomes are taken from completed summaries.

## Measured stages and limits

The largest607.255860ms switch included366.185463ms in the All projects automation click. Its specification resource completed61.3ms before the expected DOM; orchestrator completed39.3ms before it. These endpoints do not identify a backend or React function as the bottleneck. The separate eight-round trace had40 clicks,40 attempts,zero retries; its raw archive remains private and is excluded here.

The existing EventTiming collector yields unique input-handler matches for198/200 primary inputs. The100 x handlers have p9518.0ms; the98 matched Backspace handlers have p955.9ms. The largest240.098659ms input includes a202.971803ms Node action, a7.6ms input handler, and21.1ms browser dispatch-to-expected-DOM interval. These intervals have different endpoints. Quantiles are not additive; browser time is never subtracted from Node time; missing events are not zero. No longtask or React profile was collected, so draft storage is not established as the cause of the large tail.

The earlier4df diagnostic remains a separate sequential observation: actual switch/input p95367.203417/83.219029ms; static control157.082192/52.542128ms. No causal subtraction, matched-load assumption, or absent-sampler explanation is claimed.

## Files and binding

- `actual/` contains the compressed original result, original invocation and wrapper result, package/image bindings, external source binding, and independent daemon closure. The invocation's preparation status is historical; the wrapper and original completed result establish execution.
- `analysis/` contains allowlisted numerical derivation, original reference summaries, extraction code, and the pre-change source review. Its source-review timing was before the later authorized product edits/tests; those later checks are in `product/`.
- `product/` contains exact before/after source, the patch, final source hashes, component-test results, and typecheck output. All tests finished before the subsequent root-owned exclusive runtime window.
- `manifest.json` and `SHA256SUMS` cover every included file. No raw browser trace archive is present.

The executed driver SHA-256 is `d7fb7586e5b4a2639da9abb6c2e1c6a1296d2161ea7cf2ec393a8f1d3ab1b016`; interaction helper is `ec0bc9d06fda6636cbba6975e05b9c66979ca189b59aa38c373b37a318b79502`. Their exact bytes, and latency helper bytes, equal the retrievable files in `../cloud032-switch-latency/actual/accepted-harness/`. Original thresholds and400-action journey are unchanged. Product/binary/package/result hashes remain distinct. The independent closure records zero containers/volumes; process-specific closure is supported by the original driver's cleanup receipt.
