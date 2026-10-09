# Frozen preparation publication inventory

Staged 245 exact files (2,928,663 bytes) in the external capsule: 176 under `preparation-v2/` and 69 under `prior-v1/`. `capsule-allowlist.txt` is the explicit member allowlist; `capsule-SHA256SUMS` binds the staged bytes. `inventory-privacy-receipt.json` includes individual sizes/hashes, nested manifest verification, exclusions, and review limits.

The old v1 proof, failed pre-body proof, raw logs, draft manager, and blocker remain exact. The v2 preparation includes all frozen variants and validation attempts. No source was rewritten or redacted. Controlled loopback proof results are included; private installed results and runtime artifacts are excluded and owned by the parent publication projection. Package paths and expected image digest bindings are source/package metadata, not private fixture or image-inspect metadata.

`stage.py` performed inventory, manifest/hash checks, and exclusive copying only. `command.json` records its actual invocation; `stage.stdout.log` and `stage.stderr.log` are separately captured original process streams. No tests or runtime were started. The existing independent closure record was hash-checked unchanged.

The top-level capsule README, analysis, sanitized result projection, original-artifact metadata, and final publication manifest remain owned by the parent. This directory is a preparation receipt outside the capsule; its own manifest does not alter the frozen nested manifests.
