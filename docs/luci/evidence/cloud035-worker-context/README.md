# CLOUD-035: worker recovery helper reads the actual context contract

The first installed LOCAL-016 attempt failed during seeding, before any worker Create or Start. Its helper read `.luci/brief.json` from the writable checkout, although that file exists only in the immutable reviewed session capture. The original installed result remains at `artifacts/quality-swarm/worker-recovery-d348f012-attempt1`; this correction did not access its private fixture.

The helper now obtains the public `GET /v1/product/projects/{id}/context` export inventory and `GET /v1/product/projects/{id}/brief`. It requires the selected project and brief revision, exactly seven expected current checkout exports, and the actual bytes of every listed file. It never creates a missing export. When the worker capture becomes available, it checks all six unchanged document exports, the canonical `LUCI.md` prefix and the full captured brief against those recorded inputs. Existing version-2 manifest, dependency packet, project identity and immutable-byte checks remain in place. The new helper is included in the complete runtime source binding.

The product contracts were checked against the exact installed product source `d348f0127a920aab05b54a4192baa8ed3800fe54`: `journeyContextExports` publishes the seven checkout files; `captureReviewedContext` adds `.luci/brief.json` from saved state. Public `/brief` returns that saved brief. Their source hashes are in [validation.json](validation.json).

The same controlled regression ran before and after the adapter correction. It executes the actual `seedProjects` function with controlled API responses and two real temporary checkout directories containing seven persisted exports and no brief JSON file. Before the fix, it failed with the original `ENOENT`; afterward it seeded both projects successfully, requested both public context documents and left both checkout brief paths absent. The new helper existed during the failing run but was not yet used by the original adapter. No installed service, Docker worker, browser, compiler or model ran in this correction batch.

Evidence:

- [Before regression](before-regression.log): one test, one failure, exit 1.
- [After regression](after-regression.log): the same test bytes, one pass, exit 0.
- [Targeted batch](targeted-tests.log): 11 passing context, inert-adapter and source-binding tests, exit 0. Negative cases cover foreign identity, stale brief revision, missing/changed/duplicate/unsafe export declarations, missing actual files, and altered captured brief, exports or project prefix.
- [Original source hashes](before.json), [commands and tested source hashes](validation.json), and [capsule checksums](SHA256SUMS). `before/` retains original adapter/source-binding bytes and the prepared regression/helper. `tested-sources/` retains the exact six implementation/test files used by the passing batch.

CLOUD-035 is a corrected acceptance-helper defect. These controlled results do not establish installed worker Stop/restart/release behavior and do not close LOCAL-016. The root must select the new clean helper commit separately from the existing product package before another fresh installed attempt. Original failed fixtures and runtime evidence remain unchanged.
