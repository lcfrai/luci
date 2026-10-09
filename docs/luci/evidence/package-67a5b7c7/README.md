# Matching clean Linux package — 67a5b7c7

**Packaging verification passed for `67a5b7c75ca6354d43085f5bb81ec53ee4291f34`; integrated local-product acceptance remains open.** This capsule binds the clean build and package verification. It does not establish matching installed workload, responsiveness, Windows native, live-provider or remaining recovery acceptance. Exact source is retrievable on `codex/luci-production-runs`; the binaries are retained private cloud artifacts, not downloadable GitHub releases.

## Exact artifact identity

| Artifact | Bytes | SHA256 |
| --- | ---: | --- |
| Portable `luci-linux-amd64.tar.gz` | 69,040,919 | `8d389ccc4a87d33e04dd7e3e96123bb105759b45535bd9077cccc3e3b6904157` |
| Embedded/separate `luci-cloud-linux-amd64.tar.gz` | 34,616,061 | `180641ba23eda51b9e316186c1cad47ea42d8bf2c717b80e218a86f30a89a5f1` |

[Original provenance](receipts/package-provenance.json) records binary `87da14ce4415ecc15f6cf214bbcf1a103a358720cf838e7a93b723283aa26c6f`, web index `79991009aadf92f33af333bb9c0da1eb94fd44daf4bd54f8060243f94f31b765`, and extracted package location. Portable package identity is `b6caa8a0c29598f86fd280cc6399943d975919b5c1b0c7882d1d77a63408a868`. [Release manifest](release/release-manifest.json) SHA256 `a2e18d0e1fcf2c51df7ae474eb3883dfb447cc7b90a90b807173753cca20aad7` records the same clean source. No binaries or complete staged guides were copied into this capsule.

## Build and verification

The [final build](receipts/build-result.json) records exit zero, **70.01 seconds**, clean source before and after, unchanged HEAD, `GOMAXPROCS=2`, and `GOFLAGS=-mod=readonly -p=1`. The 600-second timeout did not fire. [Raw build output](receipts/build.log) retains warnings. [Original invocation](receipts/build-invocation.json) retains its historical `running` status; the final result supersedes that status. The frozen build wrapper requires an existing official remote-tracking ref containing the exact source. This run contains no failed-preflight receipt; the earlier 990 tracking failure belongs to its separate capsule and is not attributed to this build.

[Original member verification](receipts/package-integrity.json) records **955 extracted files**, **347 licensed components**, and zero missing licences. [Additive verification](receipts/compact-package-integrity.json) checks **955 portable / 954 cloud files**, **104 runtime source files**, **46 staged guide/index files**, and **230 new hosted URLs** against exact source objects and types. Complete archive/extraction equality including modes, embedded cloud equality, licence bytes and original-provenance preservation are checked.

Guide regeneration uses the **same committed staging implementation**; it is not independent Markdown rewriting proof. Separate inventory/hash/URL checks retain their stated scope. Offline source-object association is not live URL availability or heading-anchor verification. The build elapsed time is not application responsiveness, throughput or RAM evidence.

## Preservation and acceptance limits

[Previous-release preservation](receipts/previous-release-preservation.json) records the exact old **99007404** release artifacts and byte-preserving directory rename before this build. The old candidate, archive targets, original provenance and failed fixtures remain separate and unchanged according to that run-owner receipt. This publication task did not rename, delete or mutate release directories, failed fixtures or ledgers.

The new package includes the CLOUD-048 retained-retry guidance, CLOUD-050 deferred editor rendering and CLOUD-051 draft-dialog focus correction. The retrievable source also contains the CLOUD-049 phone acceptance-helper correction; that acceptance helper is not bundled in the portable runtime. The package receipt does not execute those UI workflows. The prior sustained run used **99007404 product / b81d6454 helper**, not this package: five scripted natural finishes and reviewed integrations remain scoped evidence in [its preserved capsule](../sustained-99007404-attempt7/README.md), whose overall result failed on the phone helper and responsiveness targets. Its two exited Chromium processes adopted by PID 1 remain explicit; no current-package runtime or whole-machine cleanup result is inferred.

[Original bindings](original-bindings.json) preserve all copied receipts/helpers and five selected exact packaging sources. This is **selected-source/package evidence, not a complete compiler-input capture**. [SHA256SUMS](SHA256SUMS) binds every capsule file except itself; the companion report is bound by publication.json. Standard task paths are retained for precise provenance. Publication read no private application-state, credential or failed-fixture files, and did not rebuild, run the verifiers, start Luci or run acceptance workloads.

[Reproduction](REPRODUCE.md) supplies exact retrieval and supported Linux/Windows build routes. This is a useful retrievable-source checkpoint, **not `LOCAL TEST HANDOFF READY` or completion of the local product**. Windows testing stays held while the tester's machine is below its declared memory gates.
