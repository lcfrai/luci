# Runtime pull management-core companion, version 2

The Linux management core contains `runtime/runtime-payload-companion.json` as
an ordinary regular member of at most 4096 bytes. The core does **not** carry
the v2 manifest, receipts, registry store, bootstrap archive, runtime-image
licences, or runtime-bootstrap licences. The outer installed package carries
those files once. The fixed marker has exactly these ten required fields:

| Field | Required value |
| --- | --- |
| `version` | integer `2` |
| `kind` | `luci-runtime-pull-companion-required` |
| `sourceCommit` | exact `runtime-oci-pull-manifest-v2.json.sourceCommit` |
| `manifestSHA256` | SHA-256 of the installed v2 manifest's exact bytes |
| `registryStoreSHA256`, `registryStoreBytes` | exact hash and byte length from the verified v2 manifest's fixed-name `registryStore` |
| `bootstrapSHA256`, `bootstrapBytes` | exact hash and byte length from the verified v2 manifest's fixed-name `bootstrap` |
| `ordinaryLocalInstall` | `requires-verified-companion` |
| `automaticSourceBuildFallback` | literal JSON `false` |

The Go consumer rejects duplicate/unknown/null or mixed v1/v2 fields and
binds the core's source receipt and every runtime source file to the outer
verified v2 package. It sends metadata receipts and runtime/bootstrap grants
as a separate bounded tar, then the held store and bootstrap archives as
separate transfers. All four remote transfer hashes and the core marker hash
are checked before service installation. Failed or unacknowledged SSH effects
remain `unknown` and are not replayed. A v1 Docker-save transfer requires the
explicit `runtimeTransport: "v1-diagnostic"` request; omitted transport means
v2 and never falls back to v1 or a source build.
