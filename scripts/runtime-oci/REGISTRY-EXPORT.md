# Owned serial registry export helper

`registry_gate.mjs` is an offline source component of LOCAL-038. Its tests use a
fake registry; an actual Docker Desktop daemon, helper container, image push,
memory envelope, and retirement remain unverified.

The producer starts one capped helper container from the **exact inspected
runtime image**, with no Docker socket or privilege, on its own ordinary
user-defined bridge. Only literal host loopback is published; this permits
Docker Desktop forwarding but is not an enforced egress ban. The trusted
registry/gate has no credentials/workload and connects only to container
loopback. Mount the
official Distribution 3.0.0 Linux amd64 `registry` executable and `LICENSE`
read-only at `/tools/registry` and `/tools/LICENSE`, and an owned E: directory
read/write at `/registry-data`. The official archive SHA-256 is
`61c9a2c0d5981a78482025b6b69728521fbc78506d68b223d4a2eb825de5ca3d`;
the gate verifies the extracted binary and licence hashes before starting it.
The producer must independently bind the archive/source and the helper image.

Run `node /tools/registry_gate.mjs` with:

| Variable | Value |
| --- | --- |
| `LUCI_REGISTRY_SESSION` | Fresh 32-character lowercase hex owner nonce |
| `LUCI_REGISTRY_DATA` | `/registry-data` |
| `LUCI_REGISTRY_BINARY` | `/tools/registry` |
| `LUCI_REGISTRY_LICENSE` | `/tools/LICENSE` |
| `LUCI_REGISTRY_BACKEND_PORT` | `5001` |
| `LUCI_REGISTRY_FRONT_PORT` | `5000` |
| `LUCI_REGISTRY_PUBLIC_PORT` | Exact host port published **only** at `127.0.0.1` |

The only repository is `luci/runtime-export-<session>` and the only tag is
`payload`. Distribution binds container loopback port 5001; Node binds the
container front port 5000. The producer must first prove **both host and
daemon** loopback reaches this exact helper. Host checks use `/v2/`; the
daemon proof is an Engine `DistributionInspect` of the fresh absent own
`:payload` manifest with matching exact request/404 in registry logs.
Windows host loopback reachability alone is insufficient. No daemon-wide
insecure-registry configuration or `host.docker.internal` substitution is
allowed. If the Docker Desktop port mapping is not daemon-reachable, stop here.

`GET /luci/status` gives session, repository, active upload, completed count,
and manifest state. A fresh POST gets a durable slot **before** upstream
creation; only its exact PATCH and final PUT may use that slot. Other POSTs
wait. A positive final digest/Location readback or positive upload DELETE
releases the slot. Lost acknowledgments, backend errors, timeouts, and
process restarts retain uncertainty in
`/registry-data/luci-serial-upload-v1.json`. No retry may create a second
upload around that state. `SIGTERM` writes a terminal gate receipt only after
the child exits, listener sockets close, and no upload or manifest write is
pending. It explicitly does **not** prove export, engine retirement, or charge
settlement. The producer owns exact container and registry-data reconciliation.

After positive push, the producer supplies the immutable registry manifest
digest, original image config SHA and inspected diffIDs to
`registry_export.convert`. That read-only helper fetches blobs one at a time,
checks each compressed digest and uncompressed diffID, and creates the
tagless normalized schema-763 tar on E:. It requires 26 GiB free scratch
space and limits the archive to 8 GiB. Its receipt deliberately says
`pushAndRegistryOwnershipVerified: false` until the producer binds separate
positive push and helper ownership evidence. No `docker save` fallback is
implicit.

The gate serializes layer compression starts according to Moby 28.5.1's
fresh-layer `Create` → `Open`/gzip order. It does not put a hard cap on
Docker Desktop's aggregate page cache. The producer keeps its measured host
reserve watch, exact engine/image ownership, and owned cancellation.
