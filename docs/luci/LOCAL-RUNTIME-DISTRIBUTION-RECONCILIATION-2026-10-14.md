# Local runtime distribution reconciliation

This implements Dylan's direction that Luci work on computers with limited memory. It preserves LC-01–16, the existing upstream harnesses inside Docker, authority, isolation, resource reserves and uncertain-effect recovery. It changes installation and release preparation, not the coding engine or cloud provider contract. Root has reviewed this decision; implementation starts after the named workers acknowledge this committed revision. No cloud operation is authorized.

## Measured problem and selected route

At Windows revision `79b5277bc81e283478116c003e627b7ef7604ee1`, the warm runtime build began with 3,791,605,760 bytes available. Actual RUN containers had 1 GiB memory/swap and one CPU limits. Go and apt completed, but npm installation crossed the 1 GiB host reserve. About 2.148 GB of guest buffer/page/slab cache accumulated while guest anonymous memory grew about 12 MiB. The API stayed near 59 MB and no model was loaded. Exact cancellation settled without a retained allocation. Keep both failed-build receipts; do not claim a RUN-limit failure or OOM from the monitor's explicit kill.

Ordinary installed setup will import a verified, source-bound prebuilt Linux runtime, then create only the small allocation-bound image metadata needed for ownership. It will not compile Go or install OS packages and three harnesses on the user's PC. Keep the capped local source build as an explicitly selected advanced fallback, with its existing authority and cancellation protections. Never silently fall back to that expensive route when a release asset is missing.

The release producer still builds the existing Dockerfile and exact upstream versions. Prefer an adequately provisioned existing Linux builder. If that is unavailable, a dedicated, positively owned local producer session may use measured, bounded transient guest-cache management with no unrelated running workloads, retaining normal host reserve and cancellation. Such producer assistance is recorded separately and cannot count as ordinary installed-path acceptance. No global WSL configuration, broad prune, lowered reserve or unrelated process shutdown is permitted.

## Artifact and execution boundary

Use one versioned manifest contract shared by producer and importer. Before coding, the two owners agree its exact schema and committed fixtures. It binds the full clean source revision, runtime-context input digest, target OS/architecture, archive SHA-256 and byte length, immutable image configuration ID and layer inventory, expected runtime configuration, pinned harness versions and licence/provenance inventory. Validate all bounded metadata before opening an effect. The application only accepts its matching packaged manifest; an arbitrary caller-supplied manifest cannot authorize another image.

Stream and verify the regular-file archive without loading it all into RAM. Reject linked, oversized, malformed or source-mismatched inputs and unsafe archive paths. Import must not contain repository tags that could overwrite unrelated images. Verify exact imported image/configuration identity and expected runtime configuration before publication. Record the small allocation-bound derivative separately from the reusable immutable payload; never remove a shared or unrelated image as owned cleanup.

Preserve the existing current actor authority, engine identity, allocation lineage and durable effect journal. Track import, derivative creation and publication separately. Lost responses, cancellation and restart retain uncertainty until exact readback or positive execution retirement establishes the outcome. Missing containers or tags alone do not establish completion. Measure conservative import demand separately from RUN memory; do not infer host capacity from free space inside the Docker guest.

The matching payload must be available through the normal portable/native package flow without a source checkout or secret-bearing download URL. Measure archive/package size and prevent recursive duplication through the embedded Linux cloud package. If a separate authenticated download is necessary, bring its concrete integrity, authority and user-flow contract back to root before switching distribution mode. Do not silently omit a required asset. Existing licences and all upstream notices remain included.

## Ownership and acceptance map

| Requirement | Owner and implementation | Required evidence |
| --- | --- | --- |
| LOCAL-033; LC-06/12/14 local runtime installation | Cloud Sol: Docker backend import/metadata publication, authenticated API and durable leases, setup status/client join | Manifest/archive/config/engine mismatch refusal; unrelated image preservation; duplicate/lost response/cancel/restart; bounded streaming; no heavy local RUN on the ordinary route |
| Matching distribution; DELIVERY-01 | Local Sol `/root/386944_windows_acceptance`: producer/export verification, release/package manifests and scripts, exact artifact receipts | Clean source/input binding, deterministic inventory, licences, package-size check and no recursive payload; record producer-only assistance separately |
| Actual constrained-PC behavior | Same Windows Sol after source integration; root owns final assessment | Fresh isolated install/import, measured CPU/RAM/host reserve and responsiveness throughout, actual upstream harness/model work, exact cleanup and cancellation/restart |
| Contract, central ledger, main and GitHub handoff | Root only | Exact worker acknowledgments, scoped review, integrated revision and durable issue/evidence states; no acceptance from fixtures or source alone |

Keep Azure source preparation progressing independently under the existing private management reconciliation. Its workers must not edit these runtime/import/package paths without an explicit root ownership transfer. Live Azure remains deferred.
