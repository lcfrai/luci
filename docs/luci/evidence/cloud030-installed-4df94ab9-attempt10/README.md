# Dependency attempt 10 — helper disk preflight refusal

Exact clean product source: **`4df94ab9dad0e9fedd28a873bbadc56720e1402e`**. Exact clean helper source: **`d6949ac18e4f72fc2dc286627dc5bddc4067e36d`**. The installed Linux Docker/native Codex run used a **scripted provider**, with eight local provider requests and **zero external model calls**. It failed before submitting the actual Luci product check; no product-check Create or Start occurred.

Retained provenance identifies Linux archive SHA-256 `0ac05247a194dd3cd8851c2111b1eda5906b399dbb892438484ff2b508c80d1d`, executed binary SHA-256 `6d6b11dd4382db11161384b23a55f1596e948f68c5a049a7e0aa7e409c724a9c`, 2,021 matched package members and sandbox image `sha256:333effcf40151e2411fa036b4581113be2af7f8d441bf9566f1cce6e7f208e98`. These are retained publisher assertions from this host, not a new build or independent remote retrieval. The 18 archived helper files bind byte-for-byte to the helper revision.

## Completed scoped workflow

Four workers—Alpha, original Beta, replacement Beta and join—exited naturally. Each recorded a clean native exit frame, exact container absence, completed queue state and exact task lease disappearance, without pre-exit integration or coding-command replay. Three reviewed integrations succeeded: Alpha, replacement Beta and join. Original Beta remains `unknown` with its conflicting checkout, source/output hashes and conflict bytes retained. The visible guidance correctly says integration did not complete and dependent tasks still require reviewed integration.

Six HTTP 409 dependency probes retained unchanged source/revision/effects and unallocated join identity: before starts, both original workers exited, only Alpha integrated, original Beta conflict retained, conflict retired but not replaced, and replacement Beta exited but not integrated. The join became eligible only after its current two dependencies were integrated.

All six Alpha source reads completed before the stale review request. That request returned the review-specific HTTP 409 message, “The reviewed source, target or task status changed. Refresh the complete integration review.” Revision 17, primary commit/status, 2/2/2 Create/Start/Destroy counts and absent integration checkout were preserved. The raw diagnostic's generic `unclassified-response` label remains unchanged; its body is the retained evidence of the specific refusal.

An **independent fixture command**, `node verify.mjs final`, passed on exact combined commit `262a1286b4166fdfbe9eccf1be0cc390ca6763c2`, with `DAG_CHECK_OK final`. The product run remained `awaiting-checks`. This does not substitute for running the actual Luci whole-build check.

## Why the attempt failed

At `22:06:44.341Z`, stage `product-check-verification`, the acceptance helper's `assertCheckCapacity` rejected:

```text
assert(positive(rootFreeBytes)>=6*1024**3)
```

This is the helper's **6 GiB pre-allocation disk guard**, not an observed production `memory_pressure` refusal. The driver had opened the real check UI and prepared its command/consent, but had not submitted a check. The API journal contains a check-history GET and no check-start POST. Docker transport records four worker Creates and Starts, **zero check Creates/Starts**, and eight identified worker Create guard diagnostics. The failure screenshot shows “No verification runs recorded yet.”

**The exact failing `rootFreeBytes` argument and scheduler response body were not retained.** `checkAdmission` asserts before appending its successful measurement to `checkResourceAdmissions`, which is absent in this failed result. The raw failure identifies the condition; neighboring periodic readings are corroborating observations, not that exact argument:

| Observation | UTC timestamp | Root free bytes |
|---|---|---:|
| Last periodic driver reading before failure | `22:06:43.393Z` | 6,438,748,160 |
| First periodic driver reading after failure | `22:06:44.393Z` | 6,438,727,680 |
| Unchanged required threshold | — | 6,442,450,944 |

These samples lie 948 ms before and 52 ms after the caught failure. Do not relabel them as a production admission snapshot or infer a RAM cause. No threshold was changed during the run. Wrapper exit was **1**, after **273.862 seconds**. The actual product check, joined check evidence, unknown-Create recovery and full local acceptance remain unverified by this attempt. Earlier failed attempts retain their original outcomes.

## Cgroup/resource observations and limits

The observer recorded **271 samples over 273.663 seconds**. **248** contain an observed control-plane PID/start identity and visible cgroup hierarchy; **23** explicitly do not. The full-run minimum product-comparable effective available memory was **10,390,929,408 bytes**, while minimum physical available memory was **21,456,162,816 bytes**. Those minima need not occur at the same instant. Effective availability clamps physical availability against visible cgroup limit-minus-current; physical and effective scopes are not interchangeable.

During the recorded check-preflight interval, `22:06:41.980Z`–`22:06:44.341Z`, two cgroup observations were available. Their effective available range was **15,366,201,344–15,366,643,712 bytes**. These sampled values do not establish the exact check preflight input, infer the prior attempts' memory refusals, or establish that any unrun product check would have passed admission.

[Resource observations](analysis/resource-observations.json) preserve precise source hashes, sample counts, event brackets for each Create/Start, the failure-time bracket and both physical/effective scopes. They are sequential approximately 1 Hz observations, not atomic admission measurements. Missing post-controller-exit cgroup observations remain explicit. Shared cgroup memory includes unrelated workload, retained RAM-backed files and observation work; it is not attributed solely to Luci. Hidden ancestors are not inferred.

Peak sampled summed owned-process RSS was **1,172,922,368 bytes**, including driver/browser and double-counted shared pages. Separately, maximum single-worker cgroup memory was **48,615,424 bytes**; these overlapping scopes must not simply be added. Check coverage is `not-observed` with zero check identities/cgroup samples, consistent with the missing submission; it is not measurement of an executed check. Four early samples lacked exact instance identity. Minimum sampled root free space over the entire workload was 1,704,026,112 bytes; that is a sample, not continuous proof of all disk states.

Observer interpreter self-cost was **53.668 CPU seconds over 273.753 wall seconds** (about 0.196 core on average), peak RSS **19,591,168 bytes**. Spawned Docker CLI/helper and daemon costs are excluded. There is no matched no-observer control, no pre-run idle baseline, UI-input latency measurement, sustained four-project mixed workload or whole-computer responsiveness claim here.

## Cleanup and publication

All controller/proxy/browser/provider cleanup phases and five startup acquisitions closed. Driver cleanup errors are empty. The proxy drained with no pending handlers/requests/body bytes or uncertain mutations. The final driver records zero owned service/browser processes, containers and volumes, with package/image/helper identities unchanged.

The [independent read-only post-wrapper inventory](independent-closure.json), at `22:08:53.407849Z`, found zero Docker containers and volumes and the exact version-1 empty execution ledger, SHA-256 `c0022f0995797c072d8097729ea90e74487bcefb911576c91e4af0665403a49e`. This receipt covers current Docker inventory and ledger; it does not independently enumerate process identities. No original fixture ledger repair/removal was performed. These observations do not repair or upgrade the retained charged states of earlier attempts.

`publication.json` hashes every other capsule member and binds copied originals byte-for-byte. Samples, journal and observer lifecycle use deterministic gzip with original decoded hashes. Complete final/incomplete receipts, 81-row journal, source-read cohort, six dependency probes, eight Create diagnostics, conflict/patch, three screenshots, sampler/helper copies and provenance are retained. The independent closure embeds only its explicitly scoped empty ledger metadata; no private fixture home, credentials, source/state files, account/session databases or conversations were exported. Pattern scanning and bounded artifact review found no credential values in the published allowlist.

Reproduction requires the exact pinned package/image/helpers, a fresh isolated fixture/output directory, exclusive safe Docker inventory, and the unchanged command/bounds in `invocation.json` and the pinned `scripts/acceptance/task-dependencies/README.md`. Keep the 6 GiB pre-allocation threshold, 1 GiB working reserve, and 1 GiB/1 CPU/256 PID worker limits. Original host paths identify retained evidence rather than downloadable artifacts. No browser, Docker command, compiler, workload or test suite was rerun to prepare this publication; source bindings and calculations are read-only.
