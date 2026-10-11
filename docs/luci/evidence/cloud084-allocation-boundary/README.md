# CLOUD-084/088 allocation and transfer source checkpoint

This capsule covers the unwired `internal/backend/e2bbe` package. The selected E2B Embed runtime remains pinned to `cf536c284434fdd2f09bbab35ec807a5dacc87e4`; original official-source/licence evidence remains unchanged in `../cloud084-reuse/`.

**22 top-level tests and43 subtests passed**, zero failures/skips, with Go1.26.3 Linux race detection and bounded compilation. Windows amd64 cross-compilation also passed; its temporary owned test binary was hashed then removed. No Windows test executed, no dependency was added, and no cloud provider/VM/model, guest command, product route or real off-host storage was used. Earlier successful development batches overlap these results.

From the `control-plane` directory with the documented task Go toolchain active:

```sh
GOMAXPROCS=2 go test -json -race -p 1 -count=1 -timeout=90s ./internal/backend/e2bbe
```

`go-test.jsonl`, stderr and exit preserve the final raw results. `source-before.json` and `source-after.json` bind all six new package files, statelease source and module/workspace files:18 selected inputs, unchanged. This is selected-source provenance, not a complete compiler, Go standard-library or transitive dependency inventory. `summary.json` includes command/scope/counts, the compile-only Windows identity and earlier invocation mistakes (wrong test directory; missing activation before gofmt), neither a product assertion failure.

The HTTPS fixture enforces the pinned create/body/metadata/API-key shape. Coverage includes12 distinct allocation records across4 projects/4 charged slots, a separate configured6-concurrent test, capacity and cumulative estimated-spend refusal, exclusive state ownership, lost Create acknowledgement and controller-close/reopen without replay, concurrent cancellation, foreign identity refusal, uncertain deletion retaining charges, current authority/stale revision/context checks, mandatory off-host hash acknowledgement and replacement-host uncertainty. These are simulated provider records; configured limits are not measurements of actual AWS instance capacity or billing. All fixture servers and controller resources close through test cleanup; this is not a whole-machine process inventory.

Transfer checks cover exact project/attempt/source/context/direction, deterministic archive/hash/regular executable modes, existing AGENTS.md/CLAUDE.md, size/count limits, traversal/Windows paths/case/parent collisions, links/PAX, changed/extra/short bytes and returned data. The code does not extract, execute, merge, stage remote files or grant authority from a manifest.

CLOUD-084/088 remain open. Required follow-up: real off-host journal/artifact store and restore; actual provider/KVM/TLS/private5008/cost/quota preflight; authenticated guest staging and upstream CLI/sandboxd execution; shared API/scheduler integration; durable output and reviewed delivery; live12-task fan-out and failure/host-loss/cost evidence. No cloud capability should be exposed as supported from this checkpoint alone.
