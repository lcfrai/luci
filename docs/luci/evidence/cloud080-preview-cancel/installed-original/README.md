# Original failed installed cancellation capsule

This capsule preserves the original failed run `cancel-1791727278123-5ac80fb0-871c-44fe-a402-d5533c4e0edd` independently of any C083 retry. Both `passed` and `scenarioPassed` remain **false**. No HTTP Cancel was sent. Start request `017` remained unacknowledged and is retained as a pending mutation.

The first failure occurred at `build-stage`: the classic builder configured the exact RUN container with `LogConfig.Type = "none"`, so the helper could not retrieve its stage proof using `docker logs`. The approved error literal is:

> Error response from daemon: configured logging driver does not support reading

The helper exceeded its 10-second shutdown grace and attempted escalation. Its historical `controllerKillRequired: true` flag records that helper decision. The actual observed controller exit was **code 0, signal null**. Neither SIGKILL termination nor a product shutdown failure is established by that flag or the helper's `controller-shutdown` error stage.

The independent read-only closure later established absence of the exact controller PID/start identity and exact RUN container. It retained the same unknown build reservation in ledger state `building`, charged at **1024 MiB / 1000 mCPU**. Its 58 private files were unchanged during that closure. Container absence is not authoritative build settlement and does not release the reservation. Closure scope is the exact identities and selected daemon, not whole-machine process absence.

`original/` contains unchanged original helper, wrapper, configuration, fixture, preflight, prepared manifest and preparation evidence. `original/README.md` is the preserved pre-run plan, including its expected outcomes and run command; those are historical intentions, not achieved results or instructions to retry. Original inert oracle evidence includes the initial 23-pass/1-fail harness result and corrected 24/24 result, neither of which proves the installed scenario. `run/result.json`, `wrapper-001/` and selected run bindings are unchanged originals. Files named `*.sanitized.json` are allowlisted projections with source hashes. `outcome.json` states the observed scope explicitly.

Raw authentication, HTTP bodies, private errors/stacks, controller and wrapper logs, full container inspection, full ledgers, actor data from the retained lease, private state and private file manifests are excluded. Only the explicitly approved Docker logs error literal is reproduced. Original source paths and hashes in `MANIFEST.json` identify evidence; the source artifacts remain outside the capsule and were not modified. Package verification `ok: true` and historical preflight `passed: true` describe their own checks and do not override this run's failed result.

Preparation of this capsule performed only filesystem reads, copying, hashing and derived-document writes. No helper, application, test, compiler, Docker or HTTP command was executed. `MANIFEST.json` records every payload hash and provenance; `SHA256SUMS` additionally hashes the manifest. The capsule is frozen as read-only files. Publication or any retry remains a separate action.
