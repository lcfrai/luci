# Matching clean Linux package — edd0cb12

Clean package verification passed for **`edd0cb121ff6847f681ab6bf41509a497f043d41`** on `codex/luci-production-runs`. This is a retrievable source checkpoint, **not LOCAL TEST HANDOFF READY**. Linux archive bytes remain private cloud artifacts; no new Windows binary or native pass is claimed.

| Artifact | Bytes | SHA256 |
| --- | ---: | --- |
| Portable `luci-linux-amd64.tar.gz` | 69,065,419 | `477d48fb6e6ce58f0acb801fbf6ba9046e8721a1f6fff9c1b5bc20a40b80d2c8` |
| Embedded/separate `luci-cloud-linux-amd64.tar.gz` | 34,628,069 | `9b0bf0d02caa354c663fb3729039aeadbf1ef55e887be224cdcd575d4c1b22ea` |

[Provenance](receipts/package-provenance.json) binds binary `e156c638ba111d28f11e683347def5037862589969e617c029a1f5a687e1f540` and web index `b5d6265287b93b6fa83fe25e26843eaf1bb0b42d6923dfdb26ffa93d41dc10df`. Additive package identity: `999f81b307be32d61d92736f25df4115dc6e974b8c758124df04c613d5d74baa`. [Release manifest](release/release-manifest.json), SHA256 `d4c3122beb8a3892295313d1179b26b7c1018d120fb167e4827f2aade6bd08a6`, records the same source with `sourceDirty=false` and `nativeRequested=false`.

## Completed build and preserved refusal

[Build](receipts/build-result.json) completed exit 0 in **67.72 seconds**, clean before/after and unchanged HEAD, with `GOMAXPROCS=2` and `GOFLAGS=-mod=readonly -p=1`; the 600-second timeout did not fire. [Raw output](receipts/build.log) retains warnings. The historical invocation remains `running`; final result supersedes it.

The [preflight recovery account](receipts/preflight-refusal.json) records a guard refusal separately from the build. This is a new factual account from retained tool outputs, **not reconstructed original stderr**. The official remote-tracking ref was stale; the assertion was before build/output-directory and release mutation. Root authenticated the exact published source with remote lookup and fetched the official ref, without synthesizing refs or weakening clean/public source checks. The unchanged [wrapper](helpers/build-cloud054-candidate.py) subsequently ran the build.

## Verification bounds

[Original verifier](receipts/package-integrity.json): **957 extracted files**, **347 licensed components**, zero missing licences, all members matched. [Additive verifier](receipts/compact-package-verification.json): **957 portable / 956 cloud files**, **104 runtime-source files**, **48 staged guide/index files**, and **263 hosted URLs** bound to exact source objects. Staging separately reports **262 rewritten links**; counts differ by design.

These recorded verifiers compare archives/extraction, modes, embedded-cloud equality, licences and source-bound compact guides. Guide regeneration uses the same committed implementation; offline object checks do not establish live URL/heading availability. Publication verifies copied receipts and selected source only, not independent retrieval of private archive bytes or full compiler dependency closure. Build duration is not app throughput, responsiveness or RAM evidence.

[Previous-release preservation](receipts/previous-release-preservation.json) records the byte-preserving rename of old d20 release outputs. Prior exact archive/provenance validation completed before preservation. No failed fixture or ledger was changed by this publication.

[New-archive preservation](receipts/new-archive-preservation.json) is a separate run-owner receipt. It preserves exact bytes and compatibility paths; this publisher did not move/delete archives. Any memory-backed destination trades disk pressure for memory pressure and is not a total-resource gain.

## Product scope and remaining acceptance

This package includes [CLOUD-054 terminal/attach correction](../cloud054-terminal-close-20261009/README.md). Typed terminal evidence survives delayed polls and close ordering; an optional opaque generation distinguishes same-session replacement without being an authorization token or success result. Output-overload pause and explicit retry remain intact. Source verification: Go race 74 named passes (34 top + 40 sub); full frontend 2,826 pass / 1 canonical-doc mismatch across 161 files; subsequent contract-only 24/24 affected pass; TypeScript 0. These overlap and do not constitute a second full-suite run or installed result.

Prior d20 sustained attempt 8 had five natural completions and reviewed integrations with functionalPassed=true, but failed unchanged responsiveness targets. That scripted-provider fixture does not establish live-model quality or captured handoff reports. Historical failures, charged fixtures, cleanup/zombie limits and all Windows/provider/recovery/performance gates remain separate. This new package has no matching installed result at publication.

No new configuration or secret names are required. Follow [reproduction](REPRODUCE.md) and the [isolated Windows source checkpoint](../../TEST-CHECKPOINT-EDD0CB12.md), only after local resource admission. Publication launches nothing on the user's machine. Generic task/source paths remain in raw receipts; no application state or credentials are included.
