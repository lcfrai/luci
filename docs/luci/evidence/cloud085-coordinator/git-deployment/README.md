# CLOUD-085 exact Git publication and delegated deployment

Controlled Linux source evidence for the existing coordinator delivery path. The exact reviewed repository, branch and commit are durably reserved before a nonforce Git push. A lost acknowledgement or reserved-before-crash receipt is observed without replay. A moved or absent remote branch remains unconfirmed. Only current exact publication can admit a newly authorized provider reservation.

The original controlled regression reproduced a recovery defect: readback of an already-issued push requested new allocation authority and stopped before observing the remote. The correction uses fresh current Deploy authority for retained observations, including background recovery after reopening, while every new push and provider effect still requires allocating authority. Current actor, Edit/Deploy, public-publishing policy, source, target, configuration and approved context checks remain mandatory. The locked continuation predicate reads trusted current context; authorization callbacks run outside the server mutex.

A second source review found that a new manual rollback copied the coordinator origin digest from its historical release. The narrow correction clears that digest on the new manual release while preserving the original historical receipt. Its identical existing fixture is retained before and after the fix.

## Observed results

| Invocation | Result |
| --- | --- |
| `before-readback` | Expected failure of the identical retained-readback fixture. One actual local bare-remote push succeeded before simulated ACK loss; recovery attempted new-allocation authorization, made zero observations and did not replay. Exit 1. |
| `product-fixed-1` | Race-enabled product selection: 20 top-level and 35 subtest passes; no failures or skips; exit 0. Covers actual bare Git push/readback, restart/unknown/reserved ownership, exact destination and no-force flags, authority/source drift, selected deployment/health guards, background recovery, policy/plan and pure read audit. |
| `rollback-before` | Same existing manual rollback fixture failed against the pre-fix production overlay: the new manual release inherited the original coordinator origin digest. Exit 1; 874 effective repository inputs stable. |
| `integration-fixed` | API 1 top-level plus management 1 top-level and 4 subtests passed, including package pass events. The overall invocation stopped at the unchanged root disk floor during product compilation (exit -15); fixed rollback did not execute. 874 inputs stable. |
| `rollback-fixed` | Fixed manual rollback: 1 top-level race-enabled test passed; exit 0; 874 inputs stable. The new rollback omits coordinator origin and the original historical receipt retains it. |

Counts are hierarchical outcomes, not independent workload totals. No test count establishes live application delivery.

## Provenance and bounds

Every invocation retains command, dirty checkout identity, exact status, JSONL/stderr, resource observations and before/after repository source manifests. Manifests include repository control-plane Go/module files and root go.work/go.work.sum; external modules, Go SDK and exact executed test binaries are excluded. Large manifests and the dependency listing are losslessly gzip-compressed; original byte counts and hashes are recorded in `lossless-copy-provenance.json`.

The initial recovery witness captured 874 stable repository inputs. The product fixed run captured 874 inputs with two explicit changes in sibling e2bbe/capture_types.go and capture_types_test.go; all product/API/management inputs remained unchanged. The retained post-run `go list -deps -test` declaration reports neither changed file in the 491 declared repository sources of the product test graph. This is a scoped dependency declaration, not an exact compiler or binary attestation, and does not turn the full input set into a stable checkout claim. The final integration invocations record their own source stability. The interrupted invocation remains incomplete despite its two passing package results.

`root-draft`, `before-recovery` and `rollback-original` retain original source. `same-fixture-binding.json` proves the entire Git test file is identical between the failing recovery invocation and the fixed product invocation. `source` and `final-selected-source.json` capture the joint final selected source; `owned-source.patch` contains the two new Git files. Sibling deployment and shared-wiring notes link this capsule rather than duplicate the raw runs. The rollback correction landed after the successful product selection; unchanged product tests were not repeated.

The executed runners are retained with their original absolute paths. Reproduction requires mapping those paths to the matching source/helper tree, including the original rollback overlay. Go was serialized with GOMAXPROCS=2 and -p1, package100s/outer180s limits. Following unrelated earlier disk exhaustion, root explicitly authorized a memory-backed test cache, root512MiB and tmp256MiB floors, at least4GiB effective cgroup memory headroom and a2GiB cache ceiling. This adjustment applies only to controlled source tests. Product capacity/admission guards were unchanged. After the integration disk-floor stop, root retained a full hash inventory and removed exactly two unowned disposable Go compiler temporary directories, increasing root headroom to867143680 bytes; failed receipts, source, private fixtures and caches were preserved. The compressed cleanup receipt is retained. Only the unexecuted rollback check was then run again.

## Limits

Git effects were real operations against test-owned local bare repositories; an injected transport mapping replaced only the validated HTTPS fixture destination. There was no real GitHub publication, account/credential inspection, provider allocation, Docker/browser/installed runtime or Windows check. Provider HTTP/health and execution seams were controlled fixtures. The original lost acknowledgement was simulated after the actual local Git mutation. Cloud provider readiness, real upstream model quality, live policy-permitted deployment and full application behavior acceptance remain separate unverified gates. Unknown receipts are not proof of absent effects or released charges.
