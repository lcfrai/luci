# Dedicated producer placement check

This helper is release-producer tooling. Ordinary installed runtime setup never
starts a registry, compiles an image or performs producer cache assistance.

The placement command first binds the original positive builder terminal,
original clean source checkout, exact image label/ID and current engine. It
starts one 512 MiB / one CPU / 64 PID helper from that existing image, using an
dedicated ordinary user-defined bridge, a literal 127.0.0.1 published port,
read-only tool/root mounts, no Docker socket and no privileges. This bridge
permits Docker Desktop port forwarding; `--internal` disabled the actual
mapping in the measured Windows placement. It does not enforce an egress ban.
Only the trusted pinned registry/gate runs here, with no credentials or workload;
its own connections are container loopback. No daemon trust, global networking
or default bridge changes are made. Node has a
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
$env:GOMAXPROCS = '1'
$env:GOMEMLIMIT = '512MiB'
go build -p 1 -trimpath -ldflags "-X main.sourceCommit=$revision" -o E:/private/runtime-registry-probe.exe ./control-plane/cmd/runtime-registry-probe
if ($LASTEXITCODE -ne 0) { throw 'Daemon probe compilation failed' }
python scripts/runtime-oci/registry_producer.py placement --revision $revision --build-repo E:/private/original-clean-source --original E:/private/original-producer-output --distribution-dir E:/private/distribution-v3.0.0-linux-amd64 --daemon-probe E:/private/runtime-registry-probe.exe --output E:/private/placement-unique
if ($LASTEXITCODE -ne 0) { throw 'Retain generation identities, state and logs; no blind retry' }
```

Placement makes no tag or push. It proves both Windows-host and exact Docker
engine loopback reachability, the actual pinned registry version, and positive
gate/child/container retirement. SIGTERM, client exit or container absence
alone cannot pass. An uncertain generation retains its resource IDs and data.
The original image is preserved. A placement pass is not export, ImageLoad,
ordinary installed setup, model inference or cloud acceptance.

The running container's `NetworkSettings.Ports` must show exactly the intended
loopback binding; HostConfig intent alone is insufficient. Host checks read
only `/v2/` and `/luci/status`. The clean-source-bound probe calls the selected
local Engine's `DistributionInspect` for the fresh absent `:payload` manifest,
with no auth, tag, pull or push. Both unchanged engine identity and a structured
not-found result must match the exact fresh manifest 404 in bounded original
registry logs. Missing request evidence refuses placement. A Windows HTTP
request, `docker manifest` command or WSL wget cannot substitute for this proof.

The later push estimate uses the exact largest layer from Docker history with
`--human=false`; the sizes must sum to inspected image.Size. It charges that
layer (at least 1 GiB), the separate 512 MiB helper, unchanged 1 GiB host reserve
and 512 MiB margin. Serial upload creates precede layer compression, but page
cache can persist: this remains an estimate, not a daemon hard cap. Native
pressure cancellation remains necessary. No DockerSave fallback is implicit.
