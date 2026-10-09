# Matching clean Linux package — 99007404

**Packaging verification passed for `99007404a5ddd8592dee376ea00d99c6180d26fe`; integrated local-product acceptance remains open.** This capsule binds the build and package verification only. It does not credit separate idle/sustained outcomes, Windows native behavior, live-provider quality or the remaining recovery cases. Source is retrievable on `codex/luci-production-runs`; these binaries remain private cloud artifacts, not downloadable GitHub releases.

## Exact artifact identity

| Artifact | Bytes | SHA256 |
| --- | ---: | --- |
| Portable `luci-linux-amd64.tar.gz` | 69,027,325 | `9bf32d8ebe0c7c7d3297c81e91f0451f0035357cb77eb7abf06099920dd9f164` |
| Embedded/separate cloud archive | 34,608,456 | `620246cf1f48a7a9b4f8df27a026dbd042d7cd84e4501b9afefa46e63e276e05` |

[Original provenance](receipts/package-provenance.json): binary `cf538cb3bd81d7c8186606b08285e4e25490bf788e7e1e1cda027f2361fee6c8`, web index `ae01cf71f0d045afb7d4520a698834662e20cdcb5e77a57e364a02f24a4e05bb`. Portable package identity `fbdfa478b58287375b712cd4607fb85e3ea4abf0d81f0a8a09c6794efb4b2f01`. [Manifest](release/release-manifest.json) SHA256 `496daa3ce135dbf630281592fef3ccb6dd65167f08e5a9575a0cdf357ac9286f` records clean source. No binaries or full staged guides were copied into this capsule.

## Original failure and final verification

The [first preflight](receipts/cloud046047-build-preflight-failure.json) failed because local official remote-tracking refs did not contain the newly pushed revision. **No build or release-directory mutation started.** That failure remains unchanged. A real fetch corrected tracking state; [fresh post-fix remote comparison](receipts/cloud046047-build-remote-verification.json) directly records current HEAD, remote branch and cached ancestry matching 990. It is a later read-only comparison, **not the original fetch transport log**. No tracking refs were synthesized.

The final [build](receipts/build-result.json) records exit zero, 73.21 seconds, clean before/after, unchanged HEAD, GOMAXPROCS=2 and `GOFLAGS=-mod=readonly -p=1`, with a 600-second timeout that did not fire. [Raw build log](receipts/build.log) includes its warnings. [Initial invocation](receipts/build-invocation.json) retains its historical running status; the final result supersedes it.

[Original member verification](receipts/package-integrity.json) matches **954 extracted files**, **347 licensed components**, and zero missing licences. [Additive verification](receipts/compact-package-integrity.json) checks **954 portable/953 cloud files**, **104 runtime sources**, **45 guides**, and **217 new hosted URLs** against exact source objects/types. Licence bytes, complete archive/extraction equality including modes, embedded cloud equality and original-provenance preservation are checked.

Guide regeneration uses the **same committed staging helper**, so it is not independent Markdown rewriting proof. The separate inventory/hash/URL checks are recorded explicitly. Offline official-source association is not a live URL availability or heading-anchor check. The original source-candidate/parser evidence and the prior 0dc package remain distinct.

## Preservation and limits

[Previous-release preservation](receipts/previous-release-preservation.json) records byte-preserving relocation of the old 0dc release directory before this build; its target archive bytes, original provenance and failed fixtures remain unchanged. Separately, [historical archive relocation](receipts/historical-release-relocation-20261009T0736Z.json) preserves six older archive hashes and compatibility links, totaling **352,946,135 bytes**. The run owner performed those operations; this publication task did not relocate/delete anything. **tmpfs storage consumes host memory and is not free capacity or demonstrated headroom.** No size comparison here establishes RAM, latency or throughput improvement.

[Original bindings](original-bindings.json) preserve every copied receipt/log/helper and selected exact Git source. [SHA256SUMS](SHA256SUMS) includes every capsule file except itself; the companion report is bound in publication.json. Standard task paths remain for precise provenance; no application state, private failure fixture or credential file was read for publication. The publication task did not rehash archives, run the verifiers, build, start Luci or execute acceptance workloads.

[Reproduction](REPRODUCE.md) identifies the exact source and supported Linux/Windows build routes. This is selected-source/package evidence, not a complete compiler-input capture or a `LOCAL TEST HANDOFF READY` declaration.
