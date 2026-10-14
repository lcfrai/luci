# Dedicated producer placement check

This helper is release-producer tooling. Ordinary installed runtime setup never
starts a registry, compiles an image or performs producer cache assistance.

The placement command first binds the original positive builder terminal,
original clean source checkout, exact image label/ID and current engine. It
starts one 512 MiB / one CPU / 64 PID helper from that existing image, using an
internal bridge, a literal 127.0.0.1 published port, read-only tool/root mounts,
no Docker socket, no privileges and no external network egress. Node has a
128 MiB old-space bound; its pinned Distribution child has a 192 MiB Go limit.
The cgroup is the actual shared helper hard cap. Daemon compression is separate.

For the official Distribution 3.0.0 Linux amd64 release, download the archive,
its published `.sha256` and `.provenance.json` to a private E: tool directory.
Extract only its ordinary `registry`, `LICENSE` and `README.md` files after
checking the published archive SHA-256. `distribution-v3.0.0.json` pins all
required bytes, including the original Apache grant. No network download occurs
inside the helper or during verification.

```powershell
$revision = git rev-parse HEAD
python scripts/runtime-oci/registry_producer.py placement --revision $revision --build-repo E:/private/original-clean-source --original E:/private/original-producer-output --distribution-dir E:/private/distribution-v3.0.0-linux-amd64 --output E:/private/placement-unique
if ($LASTEXITCODE -ne 0) { throw 'Retain generation identities, state and logs; no blind retry' }
```

Placement makes no tag or push. It proves both Windows-host and exact Docker
engine loopback reachability, the actual pinned registry version, and positive
gate/child/container retirement. SIGTERM, client exit or container absence
alone cannot pass. An uncertain generation retains its resource IDs and data.
The original image is preserved. A placement pass is not export, ImageLoad,
ordinary installed setup, model inference or cloud acceptance.

The later push estimate uses the exact largest layer from Docker history with
`--human=false`; the sizes must sum to inspected image.Size. It charges that
layer (at least 1 GiB), the separate 512 MiB helper, unchanged 1 GiB host reserve
and 512 MiB margin. Serial upload creates precede layer compression, but page
cache can persist: this remains an estimate, not a daemon hard cap. Native
pressure cancellation remains necessary. No DockerSave fallback is implicit.
