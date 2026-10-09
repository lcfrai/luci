# Reproduce the pinned attempt

This is a **failed historical acceptance run**, preserved to reproduce its checks and failure. CLOUD-040's later proxy correction is a different helper revision and needs a fresh run. Do not change a captured assertion, drop the six refusal rows or rerun over the retained fixture.

Use an isolated Linux checkout at `da8a6ff29d5f4b15b674087076855404456f2d9d`, Node 24.19.0/Python 3, installed Playwright Chromium dependencies, and a dedicated initially empty Docker daemon. Preserve the helper's 25%/minimum 2 GiB headroom, 6 GiB Create disk floor and 1 GiB reserve. There are 600 seconds for workflow, 180 for cleanup and 120 for preparation. VFS image copies can consume significant disk and cgroup memory. This is not the Windows acceptance route.

Build/verify the exact Linux product using the repository's `scripts/build-release.sh 0.5.0 linux-amd64` route, then extract it to a fresh directory. Supply the package provenance naming that exact clean source, archive/binary/web hashes and member verification. The historical image is exactly `sha256:333effcf40151e2411fa036b4581113be2af7f8d441bf9566f1cce6e7f208e98`; its full source/build receipt is in this capsule. No Docker image/archive is uploaded here: those artifacts must be independently obtained/verified, or a newly built image must be recorded as a distinct candidate with appropriately updated reviewed harness provenance. Never substitute an arbitrary tag for the exact image.

The following variables are operator-supplied paths to existing verified artifacts and a spacious fixture volume. `LUCI_ACCEPT_OUT` must be new; `LUCI_ACCEPT_FIXTURE_BASE` is an existing spacious directory, never an old fixture. The source-binding generator only reads source/Git and writes its output. The subsequent wrapper launches the actual test and must run only inside the explicitly reserved runtime window.

```sh
git rev-parse HEAD
# Must print da8a6ff29d5f4b15b674087076855404456f2d9d.

LUCI_ACCEPT_PIN=da8a6ff29d5f4b15b674087076855404456f2d9d
LUCI_ACCEPT_OUT=/spacious-volume/new-check-recovery-attempt
LUCI_ACCEPT_FIXTURE_BASE=/spacious-volume
LUCI_ACCEPT_PACKAGE=/spacious-volume/verified-da8-package
LUCI_ACCEPT_PROVENANCE=/spacious-volume/verified-da8-package-provenance.json
LUCI_ACCEPT_IMAGE_PROVENANCE=/spacious-volume/verified-image-provenance.json

mkdir -m 700 "$LUCI_ACCEPT_OUT"
python3 scripts/acceptance/check-create-recovery/freeze-source-binding.py \
  --repo "$PWD" --helper-commit "$LUCI_ACCEPT_PIN" \
  --output "$LUCI_ACCEPT_OUT/source-binding.json"

python3 scripts/acceptance/check-create-recovery/run-with-observer.py \
  --observation-dir "$LUCI_ACCEPT_OUT/observation" \
  --fixture-base "$LUCI_ACCEPT_FIXTURE_BASE" \
  -- --window-granted --repo "$PWD" \
  --helper-commit "$LUCI_ACCEPT_PIN" --expected-source "$LUCI_ACCEPT_PIN" \
  --source-binding "$LUCI_ACCEPT_OUT/source-binding.json" \
  --binary "$LUCI_ACCEPT_PACKAGE/luci" --web "$LUCI_ACCEPT_PACKAGE/web" \
  --provenance "$LUCI_ACCEPT_PROVENANCE" \
  --image-provenance "$LUCI_ACCEPT_IMAGE_PROVENANCE" \
  --expected-image-id sha256:333effcf40151e2411fa036b4581113be2af7f8d441bf9566f1cce6e7f208e98 \
  --require-public-reservations \
  --report "$LUCI_ACCEPT_OUT/result/report.json" \
  >"$LUCI_ACCEPT_OUT/wrapper.stdout.log" 2>"$LUCI_ACCEPT_OUT/wrapper.stderr.log"
```

The exact historical invocation, including actual artifact paths and timestamps, is retained in [invocation.json](invocation.json). Source-binding copies in [result/accepted-harness](result/accepted-harness) flatten path separators solely for archival names; run the original layout from the pinned repository. The 11 executing source blobs match `da8a6ff2`. The generator and setup instructions are available at that same immutable commit.

Preserve exit codes, all failed JSON/journal/observer rows, original screenshots, the instance pointer and private fixture. Never publish auth values or raw fixture contents. A retained image-free fence is intentional; do not delete it to satisfy cleanup. A separate terminal retirement procedure must prove transport/controller settlement and prevent future fixture use before removing an exact marker. No such retirement occurred in this receipt.

For artifact-only verification, without launching any application:

```sh
cd docs/luci/evidence/check-recovery-da8-attempt6
sha256sum -c SHA256SUMS
python3 recompute-resources.py
```

Decoded gzip hashes and original byte bindings are in [publication.json](publication.json). The public resource calculation uses the pinned sampler's independent minima of physical total/available and all finite visible ancestor limit/current pairs; zero formula mismatches do not establish atomic admission or sustained performance.
