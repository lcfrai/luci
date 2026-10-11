# Installed preview cancellation evidence — CLOUD-083 open

Both installed attempts failed before sending HTTP Cancel. Each Start remained unacknowledged, with its unknown build reservation retained in its isolated fixture ledger at **1024 MiB / 1000 mCPU**. These receipts do not establish installed product cancellation behavior or release either charge.

| Attempt | Observed helper failure | Closure |
| --- | --- | --- |
| [Original C080 attempt](../cloud080-preview-cancel/installed-original/README.md) | Classic-builder logging driver `none` prevented stage-proof retrieval. The helper attempted escalation after 10 seconds; actual controller exit was code 0, signal null. | [Original read-only closure](../cloud080-preview-cancel/installed-original/independent-closure-001/closure.sanitized.json) |
| [C083 attempt001](attempt001/README.md) | Strict RUN guard encountered a never-started LABEL metadata intermediate being removed. No archive GET occurred. Controller exited code 0, signal null, without escalation. | [Combined read-only closure](attempt001/independent-closure-001/closure.json) |

The combined closure found the exact observed controller and containers absent, zero containers/volumes on the selected daemon, and 11 retained images. Both build reservations remain in `building`; absence does not prove build-stream settlement. All 58 private files in each fixture were unchanged, including comparison with the original closure manifest. This is bounded identity-specific evidence, not a whole-host closure claim.

The capsules preserve frozen source, bounds, guarded-test outcomes, failed runtime receipts and sanitized provenance byte for byte. Private authentication, raw environment/state, errors/stacks, controller logs, full ledgers and private-file manifests are excluded. Controlled checks do not establish real Docker archive compatibility, which remains unproved. CLOUD-083 stays open; stage classification is unimplemented, and no further runtime retry is included.
