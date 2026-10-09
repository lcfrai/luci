# Independent C075 postrun closure inspection

Prepared only. The frozen installed driver is unchanged. Run after the parent joins the C075 driver and its declared cleanup, using the exact run root; this helper starts no service, browser or subprocess and performs no cleanup action.

```sh
python3 /workspace/.luci-tools/cloud075-installed-closure/inspect.py \
  --parent /workspace/.luci-tools/cloud075-private-runs \
  --run-root /workspace/.luci-tools/cloud075-private-runs/attempt-001 \
  --output /workspace/.luci-tools/cloud075-installed-closure/attempt-001.json
```

The output is created once at mode0600 outside the fixture. Its parent and the run root must be owned mode0700 directories. A zero exit means strict closure confirmation; exit2 means a completed inspection did not establish strict closure. Assertions refuse unsafe paths or malformed ownership rather than guessing.

Ownership proof: read only the fixture's bounded `driver-start.json`, `driver-owned-identities.json`, and exact `driver-managed-scopes.json`, plus sanitized driver result/journal files. Inspect only each already-recorded PID's `/proc/<pid>/stat`, comparing PID and start ticks. Include the recorded driver identity as well as controller/browser identities. Match the final driver registry against its own last successful cleanup observation. A PID reused by another process means the old identity is absent; do not inspect its environment, command line or files. A same-identity zombie is quiescent but still present, explicitly reported and insufficient for strict originals-absent confirmation.

Management workspace services are covered by the driver's recorded controller-family identities and validated exact user/team runtime scopes. The helper does not discover additional processes, scan `/proc`, inspect any process environment, enumerate unrelated users, or claim whole-machine closure. Missing ownership manifests/receipts are reported as incomplete proof; exact start/identity records are mandatory.

Selected own state hashes include only main/manifest-scoped `sessions/sessions.json`, the execution lease ledger, and worker-address metadata. Session registries are hashed rather than parsed or copied because their format can hold attach tokens. Identity/bootstrap-token, browser storage, provider config, credential files, arbitrary project files, raw/private failures and screenshots are not read. Selected state and safe receipt files are hashed before/after the inspection and must remain unchanged. These are inspection bookends, not a fabricated before/after workflow history. No state file is modified.

The fresh private nonexistent Docker socket is checked with filesystem metadata only before/after inspection. There is no Docker command, daemon inventory, socket connection, HTTP request, signal or network access. The helper carries the actual workflow result separately; successful resource closure cannot turn a failed workflow into a pass. It also projects the driver's final sealed/drained route state without claiming a second runtime observation of browser callbacks.
