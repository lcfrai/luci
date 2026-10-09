# Exact BuildKit cache selector and removal review

Read-only source/help review, 2026-10-09. No prune, Docker mutation, build, container, archive read, or daemon filesystem change was performed by this reviewer. This is not an execution authorization or a guarantee of bytes reclaimed.

The supported selector is `--filter 'id=^jxury1xmsdvxr24fm76wfizm5$'`. Buildx rewrites the `id` key to BuildKit's regular-expression operator `id~=`; `id==...` therefore becomes the invalid `id~==...`. Anchoring the full ID avoids prefix/substring matching. The parent's retained `exact-cache-du.stdout` contains exactly this one immutable, regular, unshared, reclaimable record, with parent `a0z9tfoq25ub9hkicmah62g4c`. That is read-only selection evidence, not a transactional prune dry run.

## Version and source scope

The inspected client reports buildx v0.31.0, commit `44945d71ff077ce7fc142fbdee6acec8d9acb630`. Docker daemon reports 28.4.0/build249d679. Embedded Go module metadata identifies BuildKit v0.24.0 in the daemon, versus v0.27.0 in the buildx client; the v0.24.0 server implementation is relevant to removal. On this installation, `docker builder prune --help` resolves to the buildx prune command. Both the exact buildx commit and Moby v28.4.0's legacy builder conversion implement the same ID-to-regex mapping. This review does not prove the complete installed binaries reproducibly equal upstream source.

Exact upstream source copies, URLs, lengths and SHA256 hashes are retained in `source-review/fetch-bindings.json` and `source-review/followup-bindings.json`. The two unsuccessful exploratory source paths remain recorded as HTTP errors. No source from cached Moby v28.3.3 is used as final version evidence.

## What the filter constrains

- `buildx-prune.go:227–247` maps the selector; `buildkit-manager.go:1634–1643` evaluates `id` against the complete record ID.
- `buildkit-control.go:210–248` passes the request's filters and options to each worker. `buildkit-manager.go:1036–1089` retains the same parsed filter on every repeated `pruneOnce` pass. There is no appended unfiltered/default-policy cache-selection pass in this manual-prune path.
- `buildkit-manager.go:1125–1194` skips dead/in-use records; with the default `All=false`, internal/frontend/shared records are excluded. Only matching eligible records enter the explicit deletion list.
- Buildx iterates every node in the selected builder (`buildx-prune.go:108–144`), and the server iterates its workers. Pin the selected builder/endpoint/node scope; an ID filter is not an endpoint pin. Do not add `--all`, age selectors, free-space targets or storage-budget options.

## Associated effects: not a strict singleton guarantee

The exact filter constrains explicit cache-record selection, but does not promise that only one internal object changes:

1. `buildkit-manager.go:1295–1304` can remove the selected record's equivalent immutable record before removing the selected record.
2. `buildkit-refs.go:454–488` removes the record metadata, its lease and compression-variant lease, then releases parent references. Ordinary immutable-parent release (`1447–1477`) drops the reference and possibly a view lease; it does **not** itself recursively clear a retained ordinary immutable parent's cache record. Newly eligible ordinary parent records still must match the same exact filter to be selected in the repeated pass.
3. Associated nonretained mutable/equivalent records have separate release behavior (`buildkit-refs.go:1636–1667`) that may remove those associated records. The public DU record does not expose every internal equal-reference relationship.
4. After pruning, the cache manager invokes its configured `GarbageCollect` callback (`buildkit-manager.go:1015–1032`). Moby wires this to `mdb.GarbageCollect` (`moby-controller.go:308–316`), i.e. lower-level storage garbage collection, not a call to the default BuildKit cache-selection policy. Unreferenced storage can consequently be reclaimed beyond the one logical record.
5. The controller can cancel image-cache leases when no build is active and, after an actual prune, release unreferenced solver cache metadata (`buildkit-control.go:210–237`). These ancillary effects are also outside a literal “one metadata row only” claim.

Automatic GC is a separate controller method applying worker GC policies (`buildkit-control.go:627–672`); manual `Prune` does not call that method or append its policies. This review does not establish that independent background GC cannot run concurrently. It also does not prove the actual lower-level reference graph or exact eventual storage effects.

## Decision boundary for the proposed operation

An anchored-ID manual prune is substantially narrower than global pruning and does not intentionally select the protected base/new-image cache records. Source review nevertheless cannot certify a universal one-record-only outcome. Keep the plan's explicit protected new image `sha256:76043c5d0f30207c05343e1dbc0f450658eb52726590c8c47c714c67371acec8`, trusted base `sha256:9e0fd9f731408c4405c4b9e97308bf807c8088cf521a138c0b1921667a831351`, all tags and unrelated cache records in before/after comparisons. Pin endpoint and selected builder, preserve the intended filter, capture reported removed IDs and retain failure/timeout as uncertain without replay.

The old-cache/image association remains the parent's timestamp, operation, parent and exact size-delta correlation; DU does not reveal a direct internal cache-ID-to-diffID mapping. No logical-size-to-physical-reclamation claim is made. The retained image archive cannot restore this exact BuildKit cache ID/history. If the required precondition is a proof that absolutely no associated metadata, lease or unreferenced storage can change, this API does not provide it and that requirement must remain unmet.
