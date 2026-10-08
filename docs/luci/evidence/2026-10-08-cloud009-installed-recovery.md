# Installed launch recovery — Linux, 8 October 2026

Both installed recovery scenarios passed against application revision
`919cffe014d1104d9e65fa19351683d182355f36`. They used actual Docker requests,
actual controller process deaths, browser recovery consent, durable private
state and a native Codex task in another project. The local provider deliberately
scripted the task; this verifies recovery and integration mechanics, not model
reasoning or the complete local product.

The [curated receipt](2026-10-08-cloud009-installed-recovery.json) contains exact
artifact/source hashes, observed chronology, context/output hashes, cleanup,
observer lineage and the two preserved earlier failed helper attempts.

| Scenario | Observed result | Driver time | Controller deaths | Docker request counts |
| --- | --- | ---: | ---: | --- |
| Delayed Create, attempt 3 | Explicit recovery completed; original uploaded Create reached Docker afterward and received **409** | 80.808 s | 1 | 3 Create attempts, 1 Start |
| Delayed Start, attempt 1 | Lost fence acknowledgement retained capacity; reviewed replacement and durable-proof restart succeeded; original immutable-ID Start received **404** | 131.308 s | 2 | 4 Create attempts, 2 Start attempts |

Create counts include scratch recovery markers and the rejected late original
Create. The second scenario created the original full-image container but never
started its agent. Each scenario ran exactly one successor task, observed native
exit status zero, checked its isolated output, and integrated it through browser
diff review. Both produced the same expected output SHA-256,
`f009d9ebd4e4c22374f16f092ef4a8df764866db69c6be9cb977657389390624`.

In the delayed-start scenario, the first real fence Create's response was lost.
Capacity stayed reserved and the action returned HTTP 409. A reviewed retry
replaced that unacknowledged container with a different acknowledged ID under
the same intent. The controller was then killed after proof and retirement were
saved but before capacity release. Restart and another reviewed retry reused
the same acknowledged proof and finalized recovery without another Create.

Both scenarios refused recovery while the original allocator was still alive
without recording recovery intent or changing Docker. They retained the
original checkout, approved source exports and captured context; the successor
received only its own project's context. The original delayed request could
neither launch nor recreate an agent after recovery.

## Exact inputs and scope

- Clean Linux application package: `919cffe014d1104d9e65fa19351683d182355f36`.
  Archive SHA-256:
  `210780c433ff590d2e5ee181065a619d9d5ab0f64b8e143e55084b88f89c1fc5`.
  All 1,123 installed archive members matched.
- Reviewed acceptance helper commit:
  `d4d065f56054b46890262b482eb0129214ad93fe`.
  Driver SHA-256:
  `2a841737588295e89e06157ebaae75a47e949259daab396ad18f3172c47f58d2`.
- Runtime image:
  `sha256:42597a985b20f2aee31a629a320980f02180bde307b6ffdd3ec60b257a82af5b`.
  Its byte-verification receipt and sandbox source bindings were checked by the
  driver against the packaged source commit.
- The observed helper checkouts were `d4d065f…` and `675ed402…`, both with
  unrelated source edits present. Captured helper hashes match the reviewed
  bytes in both runs. The installed application package remained unchanged;
  these results are not a test of those later product edits.

Each run kept the declared 6 GiB admission floor before every actual Create and
the 1 GiB running reserve. The executing worker had 1 GiB RAM, one CPU and 256
PIDs. The frozen observer retained 102 samples over 102.084 s for delayed Create
and 152 samples over 153.266 s for delayed Start. These sample counts do not
establish a sustained resource-efficiency or whole-computer responsiveness pass.

Both wrappers exited zero. All exact owned controllers, browser processes,
containers and volumes were absent; requests drained without uncertain mutation;
protected image/package bytes were unchanged. Production recovery markers are
permanent. The harness's separate test-only teardown removed its exact inspected
marker without force only after every owned controller was dead and all captured
requests drained. Successful private fixtures were removed.

## Preserved failures and limits

The first two delayed-create attempts failed before any Docker Create was
forwarded or intended controller crash occurred. The first lacked a detailed
assertion; source-based reproduction found the missing allowance for the exact
read-only Git administration mount. The second pinpointed Docker SDK capability
normalization. Narrow corrections and 25 controlled checks preceded attempt 3.
Their reports, hashes, resource samples and stopped private fixtures remain
separate; failed attempts are not counted as successful recovery.

Both successor terminal probes observed explicit exit status zero followed by
WebSocket close code 1006 and a socket-error event. A clean WebSocket shutdown is
not claimed. The first scenario's final controller stop recorded an unavailable
exit code while independently proving no remaining owned process and no forced
cleanup. Queue/lease timestamps are observation bounds, not exact atomic release
instants.

This is a Linux fixture with two projects per scenario and one executing agent.
It does not close the four-project sustained mixed-workload, native Windows,
live-provider output quality, installation, complete local acceptance, public
release, billing or cloud gates. No external model call or cloud resource was
used. Observer self-cost and complete Docker-daemon overhead were not measured
by this frozen observer.

Reproduction uses the separately pinned package and image with the commands and
guards in [the recovery acceptance README](../../../scripts/acceptance/launch-recovery/README.md).
Run each scenario separately with fresh report/resource directories after
exclusive Docker admission. The receipt names the source/helper identities;
re-running current product source would be a different test input.
