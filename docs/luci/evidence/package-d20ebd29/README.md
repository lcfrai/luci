# Matching clean Linux package — d20ebd29

**Clean packaging verification passed for `d20ebd292f3f1de04d6726bab0998c210987bcb5`; the complete local product remains unaccepted.** Exact source is retrievable on `codex/luci-production-runs`. Archive bytes remain private cloud artifacts, not downloadable Windows or GitHub release assets. This is not `LOCAL TEST HANDOFF READY`.

| Artifact | Bytes | SHA256 |
| --- | ---: | --- |
| Portable `luci-linux-amd64.tar.gz` | 69,055,452 | `5e4e0a2a380268f0d17cbbf2cf7cfb41f5aa64abc8f454864fdb7eb0bdb6400c` |
| Embedded/separate `luci-cloud-linux-amd64.tar.gz` | 34,622,222 | `f61e023a464aeb992a7b09455de73065475b69b22f8f6690bf818305f4dc8545` |

The [original provenance](receipts/package-provenance.json) binds binary `f322d9091d91b78ff6947c33588dd29d6714bcf71638101c9b7d16e977383ff8`, web index `79991009aadf92f33af333bb9c0da1eb94fd44daf4bd54f8060243f94f31b765` and extracted package identity. The additive package identity is `fe8f365bbe5bc41f37fb583ca45f7e785ac0ffcb959711679ca86ca663d495bc`. [Release manifest](release/release-manifest.json) SHA256 `baee95caa310743fe556c6fcfb7421a9c52312c33bdf1d0901401f25b57d3867` records the same source with `sourceDirty=false` and `nativeRequested=false`.

## Build and retained preflight failure

[Final build](receipts/build-result.json): exit 0 in **71.28 seconds**, clean before/after, unchanged HEAD, `GOMAXPROCS=2`, `GOFLAGS=-mod=readonly -p=1`; the 600-second timeout did not fire. [Raw build output](receipts/build.log) retains warnings. The [invocation](receipts/build-invocation.json) retains its historical `running` status; the final result supersedes it.

The [initial preflight refusal](receipts/build-cloud052-preflight-attempt1.json) belongs to this exact source. A narrow fetch updated `FETCH_HEAD` without updating an official remote-tracking ref containing the commit. The unchanged wrapper correctly refused before build/output creation or package mutation. An actual fetch to the official tracking ref corrected that prerequisite; no synthetic ref or weakened clean-source requirement was used. Its recorded wrapper SHA matches the [frozen wrapper](helpers/build-cloud052-candidate.py). This retained failure is separate from the successful build.

## Verification scope

[Original member verification](receipts/package-integrity.json) reports **956 extracted files**, **347 licensed components**, no missing licences and matching archive members. [Additive verification](receipts/compact-package-verification.json) checks **956 portable / 955 cloud files**, **104 runtime-source files**, **47 staged guide/index files**, and **251 new hosted URLs** against exact source objects/types. The staging implementation separately reports **250 rewritten links**; these are different counts. Archive/extraction equality including modes, embedded-cloud equality, licence bytes and original-provenance preservation are checked by that recorded verifier.

Guide regeneration uses the same committed staging implementation; it is not independent Markdown rewriting proof. Inventory/hash/object checks retain their stated scope. Offline URL/object binding does not prove live URL or heading-anchor availability. This publication independently binds the retained receipts and selected source, **not the large private archive bytes**; it did not rerun either verifier or build. Build duration is not app responsiveness, throughput or RAM evidence.

## Preservation and resource tradeoff

[Previous-release preservation](receipts/previous-release-preservation.json) records a byte-preserving rename of the old **67a5b7c7** release directory before this build. Old package/extraction/provenance and failed fixtures remain separate according to that run-owner receipt.

The [new-archive preservation receipt](receipts/archive-preservation-d20ebd29.json) records verified copies of the two newly generated archives to retained `/tmp` paths, with compatibility links at their original release paths and unchanged hashes/lengths. The original manifests, receipts and failed fixtures were not altered. `/tmp` is memory-backed here: moving archives changes disk-versus-memory pressure; it is **not a total-resource gain or performance result**. This publication copied the receipt only and did not move/delete archives or inspect private failed fixtures.

## What is and is not ready

This product includes **CLOUD-052**: canceled overview waiters can leave the bounded cache-fill gate, and scheduler display reads retain their request context through the bounded reservation observation. Source exclusion, fresh authority, cache/file limits, capacity charges and worker/mutation lifecycles remain protected. [Final combined source validation](../cloud052-final-validation-20261009/README.md) records 31 top-level + 18 subtest race outcomes; the separate all-package compile command executed zero tests. Neither is a full-suite pass or installed cancellation/Windows result.

The previous [67a5 installed idle diagnostic](../cloud032-idle-67a5b7c7-20261009/README.md) failed both primary p95 targets: switch 519.762309 ms against 300 ms, input 114.250714 ms against 100 ms. Its 400 successful actions mean capture completion, not performance acceptance. This new package has no matching installed responsiveness or sustained result at publication. Earlier five scripted task finishes/integrations belong to product 99007404; that sustained run's phone/latency failure and cleanup limits remain preserved separately. No live-model quality, Windows native, whole-computer responsiveness or remaining recovery acceptance is inferred.

There are **no new configuration or secret names**. [Reproduction](REPRODUCE.md) and the [source checkpoint](../../TEST-CHECKPOINT-D20EBD29.md) use fresh isolated state/projects and the existing Windows resource gates. No local machine process was started by publication.

[Original bindings](original-bindings.json) preserve copied receipts/helpers and five selected packaging sources. This is selected-source/package evidence, not a complete compiler-input capture. `publication.json` binds all members and the companion/checkpoint; `SHA256SUMS` additionally binds that manifest. Standard task paths are retained. No private credentials or application-state files were read.
